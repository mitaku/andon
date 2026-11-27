package digest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/flock"
	"andon/internal/publish"
	"andon/internal/tray"
)

// Sent は library/digest/sent.json（schema/digest-sent.schema.json。docs/contract.md §28）。
type Sent struct {
	Issue        string        `json:"issue"`
	Since        string        `json:"since"`
	Until        string        `json:"until"`
	Destinations []Destination `json:"destinations"`
}

const (
	kindFiles = "files" // セルの中に置いた（ローカルの tray。届かなければ files/）
	kindTray  = "tray"  // Cloudflare の tray（プロジェクトが選んだときだけ。ADR-0064 決めたこと 11）
	kindSlack = "slack"
)

// notSent は、送る前に書く理由。送っている途中で落ちても、済んだ送り先に数えない（決めたこと 16）。
const notSent = "送り終えていない（打ち直すと送る）"

type Destination struct {
	Kind    string `json:"kind"`
	At      string `json:"at"`
	OK      bool   `json:"ok"`
	URL     string `json:"url,omitempty"`
	Path    string `json:"path,omitempty"` // files: ローカルの tray に届かず files/ に置いたときの、セルの repo からのパス
	Channel string `json:"channel,omitempty"`
	File    string `json:"file,omitempty"`
	Error   string `json:"error,omitempty"`
}

// done は送り先が全部済んでいるかと、最後に済んだ時刻。
func (s *Sent) done() (string, bool) {
	at := ""
	for _, d := range s.Destinations {
		if !d.OK {
			return "", false
		}
		if d.At > at {
			at = d.At
		}
	}
	return at, len(s.Destinations) > 0
}

func readSent(path string) (*Sent, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := contract.SchemaDigestSent.Check(b); err != nil {
		return nil, err
	}
	var s Sent
	if err := contract.Decode(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// pageDate は名前の日付（期間の終わりの日。セルの時刻）。組み立てるときと送るときで同じ名前になる。
func pageDate(in *Input) time.Time {
	until, _ := time.Parse(timeLayout, in.Until)
	return until.In(time.Local)
}

// filesSlug は名前の slug（ADR-0044 決めたこと 15。同じ日に臨時の 1 枚を作っても上書きしないよう課題の番号を入れる）。
func filesSlug(in *Input) string {
	return "digest-" + in.Project + "-" + in.Issue
}

// pageName は tray での名前（ADR-0064 決めたこと 9。今までの files/ の名前から拡張子を除いたもの）。
func pageName(in *Input) string { return publish.Name(pageDate(in), filesSlug(in)) }

// pendingURL は組み立てたときの短い文の控えに書く URL の代わり（送るまで tray の URL は決まらない）。send は送った URL で書き直す。
const pendingURL = "（andon digest send が tray に送った URL）"

// destinations はオーナーが project.json に書いた送り先（決めたこと 12）。セルの repo の HEAD のものを、作者の照合を通して読む。
// project.json か digest が無ければ空。信じられない・形が違えば誤り（どこへも送らない）。
func destinations(env Env) ([]delegate.DigestDestination, error) {
	slug := filepath.Base(env.Root)
	p, err := delegate.ReadProject(env.Workspace, slug)
	if errors.Is(err, delegate.ErrAbsent) {
		// git の失敗を「送り先が無い」に読むと、送りそこねた行を落として期間を進める
		return nil, delegate.ConfirmProjectAbsent(env.Workspace, slug)
	}
	if err != nil {
		return nil, err
	}
	if p.Digest == nil {
		return nil, nil
	}
	return p.Digest.Destinations, nil
}

// composed は課題 id の digest.html を読み、今の input.json と digest.json から組み立てたものと同じことを確かめて返す。
// 送る中身は組み立てたダイジェストだけ（決めたこと 12 の守りの限り）で、短い文も同じ JSON から作る（決めたこと 13）。
func composed(env Env, id string, in *Input) ([]byte, *Text, error) {
	d := dir(env.Root, id)
	html := filepath.Join(d, htmlFile)
	page, err := os.ReadFile(html)
	if err != nil {
		return nil, nil, fmt.Errorf("%s が無い（先に andon digest build %s）", html, id)
	}
	// 組み立てたあとに集め直したら、古い HTML を新しい期間として送らない（送った期間のあとから次が数えるので、間の出来事が落ちる）
	if !strings.Contains(string(page), `<meta name="andon-digest-period" content="`+periodKey(in)+`">`) {
		return nil, nil, fmt.Errorf("%s は今の %s と違う期間から組み立てた（andon digest build %s で組み立て直す）", html, inputFile, id)
	}
	path := filepath.Join(d, digestFile)
	b, err := os.ReadFile(path)
	if err == nil {
		err = contract.SchemaDigest.Check(b)
	}
	var text Text
	if err == nil {
		err = contract.Decode(b, &text)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s が読めない（andon digest build %s で組み立て直す）: %v", path, id, err)
	}
	want, errs := compose(in, &text, location(env.Root))
	if len(errs) > 0 || !bytes.Equal(want, page) {
		return nil, nil, fmt.Errorf("%s が %s と %s から組み立てたものと違う（andon digest build %s で組み立て直す）", html, inputFile, digestFile, id)
	}
	return page, &text, nil
}

// Send は課題 id の digest.html を files/ と project.json の送り先へ出し、files/ の URL を返す（決めたこと 12・13・15）。
// 済んだ送り先には送らない。失敗した送り先には送り直す（決めたこと 16）。files/ に出せていれば、誤りのときも URL を返す。
// files/・Slack・sent.json は、鍵を取ったあとに読んで照らした同じ 1 枚を指す。
func Send(env Env, id string) (string, error) {
	// 鍵を置く前に、課題と入力が在ることだけ確かめる（中身は鍵を取ってから読む）
	if _, err := readInput(env.Root, id); err != nil {
		return "", err
	}
	path := filepath.Join(dir(env.Root, id), sentFile)
	// 読んでから送って書くまでを1人ずつにする（同時に打っても、外へ2度送らない・結果が消えない）
	unlock, err := flock.Lock(path + ".lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	in, err := readInput(env.Root, id)
	if err != nil {
		return "", err
	}
	page, text, err := composed(env, id, in)
	if err != nil {
		return "", err
	}

	var old []Destination
	switch prev, err := readSent(path); {
	case err == nil && prev.Since == in.Since && prev.Until == in.Until:
		old = prev.Destinations
	case err == nil, errors.Is(err, fs.ErrNotExist):
		// 集め直した（期間が違う）ときは、前の結果を捨てて送り直す
	default:
		return "", fmt.Errorf("%s が読めない（直すか消す）: %v", path, err)
	}
	to, unread := destinations(env)
	if unread != nil {
		env.warn("project.json の送り先を読めないので、セルの外へは送らない: %s", oneLine(unread.Error()))
	}
	remote, remoteErr := tray.RemoteFor(env.Workspace, in.Project)
	if remoteErr != nil {
		env.warn("Cloudflare の tray を選んだかを読めないので、Cloudflare の tray へは送らない: %s", oneLine(remoteErr.Error()))
	}
	now := env.Now.UTC().Format(timeLayout)
	sent := &Sent{Issue: id, Since: in.Since, Until: in.Until,
		Destinations: plan(old, to, unread != nil, remote != nil, remoteErr != nil, now)}
	save := func() error {
		if err := contract.Write(path, contract.SchemaDigestSent, sent); err != nil {
			return fmt.Errorf("sent.json を書けない: %w", err)
		}
		return nil
	}
	// 外へ出す前に、まだの送り先を済んでいないものとして書く。途中で落ちても次の期間の始まりに数えない
	if err := save(); err != nil {
		return "", err
	}

	files := &sent.Destinations[0]
	report := tray.Report{Project: in.Project, Name: pageName(in), Title: tray.HTMLTitle(page), Kind: tray.KindDigest,
		ContentType: "text/html; charset=utf-8", Body: page}
	// まだの Slack へ送る回は、tray にも同じ 1 枚を上げ直す（組み立て直していると、短い文の URL が古い 1 枚を指す。中身が同じならバージョンは上がらない）
	toSend := unread == nil && slices.ContainsFunc(sent.Destinations, func(x Destination) bool { return x.Kind == kindSlack && !x.OK })
	if !files.OK || toSend {
		u, p, perr := placeLocal(env, in, report)
		*files = Destination{Kind: kindFiles, At: now, OK: perr == nil, URL: u, Path: p}
		if perr != nil {
			files.Error = oneLine(perr.Error())
		}
		if err := save(); err != nil {
			return "", err
		}
		if perr != nil {
			return "", fmt.Errorf("セルの中に置けない: %v", perr)
		}
	}
	link := files.URL
	if link == "" {
		link = files.Path
	}
	var failed []string
	if i := slices.IndexFunc(sent.Destinations, func(x Destination) bool { return x.Kind == kindTray }); i >= 0 {
		x := &sent.Destinations[i]
		if (!x.OK || toSend) && remoteErr == nil {
			u, err := placeRemote(remote, report)
			x.At = now
			if err != nil {
				if !x.OK {
					x.Error = oneLine(err.Error())
				}
				failed = append(failed, "Cloudflare の tray に送れない: "+oneLine(err.Error()))
			} else {
				x.OK, x.URL, x.Error = true, u, ""
			}
			if err := save(); err != nil {
				return link, err
			}
		}
		if !x.OK && remoteErr != nil {
			failed = append(failed, "Cloudflare の tray を選んだかを読めないので、前に送りそこねた tray へ送り直せない")
		}
		if x.OK {
			// Cloudflare の tray に届いたダイジェストは、短い文にその URL を書く（決めたこと 11）
			link = x.URL
		}
	}

	for i := range sent.Destinations {
		x := &sent.Destinations[i]
		if x.Kind != kindSlack || x.OK {
			continue
		}
		if unread != nil {
			failed = append(failed, "project.json の送り先を読めないので、前に送りそこねた "+x.Channel+" へ送り直せない")
			continue
		}
		loc := location(env.Root)
		file, err := sendSlack(env, x.Channel, slackName(in), Message(text, in, loc, link), page)
		if err != nil {
			x.Error = oneLine(err.Error())
			failed = append(failed, fmt.Sprintf("Slack の %s へ送れない: %s", x.Channel, x.Error))
		} else {
			x.OK, x.File, x.Error = true, file, ""
		}
		if err := save(); err != nil {
			return link, err
		}
	}
	// 短い文の控えを、送った URL で書き直す（組み立てたときは URL が決まっていない）
	if err := writeMessage(env, id, Message(text, in, location(env.Root), link)); err != nil {
		failed = append(failed, err.Error())
	}
	if len(failed) > 0 {
		return link, errors.New(strings.Join(failed, "、"))
	}
	return link, nil
}

// placeLocal は照らした中身そのもの（digest.html を読み直さない）をローカルの tray に送り、その URL を返す。
// ローカルの tray に届かなければ、andon publish と同じく files/ に置き、セルの repo からのパスを返す（ADR-0064 決めたこと 8）。
func placeLocal(env Env, in *Input, r tray.Report) (url, path string, err error) {
	d := tray.Send(r, nil)
	if d.Local != nil {
		return d.Local.URL, "", nil
	}
	env.warn("%s。files/ に置く", oneLine(d.LocalErr.Error()))
	dir := filepath.Join(env.Workspace, "files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	name := r.Name + ".html"
	if err := writeAtomic(filepath.Join(dir, name), r.Body); err != nil {
		return "", "", err
	}
	return "", "files/" + name, nil
}

// placeRemote は Cloudflare の tray へ送る（伏せた写し。決めたこと 10）。
func placeRemote(remote *tray.Remote, r tray.Report) (string, error) {
	switch {
	case remote == nil:
		return "", errors.New("project.json が Cloudflare の tray を選んでいない")
	case tray.Off():
		return "", tray.ErrOff
	case remote.Key == "":
		return "", fmt.Errorf("API キーが無い（andon tray auth で %s に置く）", tray.KeyPath())
	}
	out := r
	out.Body = tray.Redact(r.ContentType, r.Body)
	a, err := tray.Put(remote.Upload, remote.Key, out)
	if err != nil {
		return "", err
	}
	return a.URL, nil
}

// writeMessage は短い文の控え（message.txt）を一時ファイル＋rename で書く。
func writeMessage(env Env, id, msg string) error {
	if err := writeAtomic(filepath.Join(dir(env.Root, id), messageFile), []byte(msg)); err != nil {
		return fmt.Errorf("%s を書けない: %v", messageFile, err)
	}
	return nil
}

func writeAtomic(dst string, b []byte) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// plan は今回の送り先の行。先頭は files で、済んだ行は残す（同じ kind へ2度送らない）。
// 済んでいない行は今の設定で作り直す。設定から消えた送り先は落とす。設定を読めないときは、送りそこねた行を落とさずに残す
// （落とすと、Slack に届いていない期間が送ったことになる）。
// 選んでいれば Cloudflare の tray の行（kind tray）を files の次に置く。選んだかを読めないときも、送りそこねた行は残す。
func plan(old []Destination, to []delegate.DigestDestination, unread, remote, remoteUnread bool, now string) []Destination {
	out := []Destination{{Kind: kindFiles, At: now, Error: notSent}}
	done := map[string]bool{}
	for _, x := range old {
		switch {
		case x.Kind == kindFiles:
			if x.OK {
				out[0] = x
			}
		case x.Kind == kindTray:
			if x.OK || remoteUnread {
				out = append(out, x)
				done[x.Kind] = true
			}
		case x.OK || unread:
			out = append(out, x)
			done[x.Kind] = true
		}
	}
	if remote && !done[kindTray] {
		out = append(out, Destination{Kind: kindTray, At: now, Error: notSent})
	}
	// tray の行は files の次（短い文の URL を決めてから Slack へ送る）
	slices.SortStableFunc(out, func(a, b Destination) int { return rank(a.Kind) - rank(b.Kind) })
	for _, t := range to {
		if !done[t.Kind] {
			out = append(out, Destination{Kind: t.Kind, At: now, Channel: t.Channel, Error: notSent})
		}
	}
	return out
}

func rank(kind string) int {
	switch kind {
	case kindFiles:
		return 0
	case kindTray:
		return 1
	}
	return 2
}

// slackName は Slack に上げるファイルの名前（決めたこと 13。日付は tray の名前と同じく期間の終わりの日）。
func slackName(in *Input) string {
	return fmt.Sprintf("digest-%s-%s.html", in.Project, pageDate(in).Format("2006-01-02"))
}

func sendSlack(env Env, channel, name, comment string, body []byte) (string, error) {
	s, err := newSlack(env)
	if err != nil {
		return "", err
	}
	return s.upload(channel, name, comment, body)
}

// CheckDestinations は andon digest check。project.json の送り先へ、確かめの小さなテキストのファイルを send と同じ口で送る（決めたこと 14）。
// Cloudflare の tray を選んだプロジェクトでは、その API キーが置いてあることも見る（ADR-0064 決めたこと 11）。
func CheckDestinations(env Env, w io.Writer) error {
	to, err := destinations(env)
	if err != nil {
		return fmt.Errorf("project.json の送り先を読めないので何も送らない: %s", oneLine(err.Error()))
	}
	project := filepath.Base(env.Root)
	remote, err := tray.RemoteFor(env.Workspace, project)
	if err != nil {
		return fmt.Errorf("送り先の Cloudflare の tray を選んだかを読めない: %s", oneLine(err.Error()))
	}
	if remote != nil {
		if remote.Key == "" {
			return fmt.Errorf("送り先の Cloudflare の tray の API キーが無い（andon tray auth で %s に置く）", tray.KeyPath())
		}
		fmt.Fprintf(w, "確かめた: Cloudflare の tray（%s）の API キーが置いてある\n", remote.Upload)
	}
	if len(to) == 0 {
		if remote != nil {
			return nil
		}
		return errors.New("project.json に digest の送り先の設定が無いので何も送らない")
	}
	body := fmt.Sprintf("andon digest check: %s のダイジェストの送り先の確かめ（%s）\n", project, env.Now.UTC().Format(timeLayout))
	for _, t := range to {
		file, err := sendSlack(env, t.Channel, "digest-check-"+project+".txt", body, []byte(body))
		if err != nil {
			return fmt.Errorf("送り先 %s（Slack）へ送れない: %s", t.Channel, oneLine(err.Error()))
		}
		fmt.Fprintf(w, "送った: Slack の %s に確かめのファイル %s\n", t.Channel, file)
	}
	return nil
}

func oneLine(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
	if s == "" {
		return "理由が分からない"
	}
	return s
}
