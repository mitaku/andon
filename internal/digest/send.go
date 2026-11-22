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
)

// Sent は library/digest/sent.json（schema/digest-sent.schema.json。docs/contract.md §28）。
type Sent struct {
	Issue        string        `json:"issue"`
	Since        string        `json:"since"`
	Until        string        `json:"until"`
	Destinations []Destination `json:"destinations"`
}

// 送り先の kind。
const (
	kindFiles = "files"
	kindSlack = "slack"
)

// notSent は、送る前に書く理由。送っている途中で落ちても、済んだ送り先に数えない（決めたこと 16）。
const notSent = "送り終えていない（打ち直すと送る）"

// Destination は送り先1つの結果。files は url、slack は channel とファイルの id（file）を持つ。
type Destination struct {
	Kind    string `json:"kind"`
	At      string `json:"at"`
	OK      bool   `json:"ok"`
	URL     string `json:"url,omitempty"`
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

// filesEnv は files/ に出すときの publish の値。日付は期間の終わりの日（セルの時刻）で、組み立てるときと送るときで同じ名前になる。
func filesEnv(env Env, in *Input) publish.Env {
	until, _ := time.Parse(timeLayout, in.Until)
	return publish.Env{Workspace: env.Workspace, Hostname: env.Hostname, Now: until.In(time.Local)}
}

// filesSlug は files/ に出す名前の slug（決めたこと 15。同じ日に臨時の 1 枚を作っても上書きしないよう課題の番号を入れる）。
func filesSlug(in *Input) string {
	return "digest-" + in.Project + "-" + in.Issue
}

// filesURL は digest.html を files/ に出したときの URL。
func filesURL(env Env, in *Input) string {
	pe := filesEnv(env, in)
	return publish.URL(pe, publish.Name(pe, filesSlug(in), ".html"))
}

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
	now := env.Now.UTC().Format(timeLayout)
	sent := &Sent{Issue: id, Since: in.Since, Until: in.Until, Destinations: plan(old, to, unread != nil, now)}
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
	// まだの Slack へ送る回は、files/ にも同じ 1 枚を置き直す（組み立て直していると、短い文の URL が古い 1 枚を指す）
	toSend := unread == nil && slices.ContainsFunc(sent.Destinations[1:], func(x Destination) bool { return !x.OK })
	if !files.OK || toSend {
		u, perr := publishPage(filesEnv(env, in), page, filesSlug(in))
		files.At = now
		if perr != nil {
			files.OK, files.URL, files.Error = false, "", oneLine(perr.Error())
		} else {
			files.OK, files.URL, files.Error = true, u, ""
		}
		if err := save(); err != nil {
			return "", err
		}
		if perr != nil {
			return "", fmt.Errorf("files/ に出せない: %v", perr)
		}
	}

	var failed []string
	for i := range sent.Destinations[1:] {
		x := &sent.Destinations[1+i]
		if x.OK {
			continue
		}
		if unread != nil {
			failed = append(failed, "project.json の送り先を読めないので、前に送りそこねた "+x.Channel+" へ送り直せない")
			continue
		}
		loc := location(env.Root)
		file, err := sendSlack(env, x.Channel, slackName(env, in), Message(text, in, loc, files.URL), page)
		if err != nil {
			x.Error = oneLine(err.Error())
			failed = append(failed, fmt.Sprintf("Slack の %s へ送れない: %s", x.Channel, x.Error))
		} else {
			x.OK, x.File, x.Error = true, file, ""
		}
		if err := save(); err != nil {
			return files.URL, err
		}
	}
	if len(failed) > 0 {
		return files.URL, errors.New(strings.Join(failed, "、"))
	}
	return files.URL, nil
}

// publishPage は照らした中身そのものを files/ に出す（digest.html を読み直さない。照らしたあとに変わっていても出さない）。
func publishPage(env publish.Env, page []byte, slug string) (string, error) {
	tmp, err := os.CreateTemp("", "andon-digest-*.html")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // 一時ファイルの片付けで、失敗しても出した中身に効かない
	_, err = tmp.Write(page)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	return publish.Publish(env, tmp.Name(), slug)
}

// plan は今回の送り先の行。先頭は files で、済んだ行は残す（同じ kind へ2度送らない）。
// 済んでいない行は今の設定で作り直す。設定から消えた送り先は落とす。設定を読めないときは、送りそこねた行を落とさずに残す
// （落とすと、Slack に届いていない期間が送ったことになる）。
func plan(old []Destination, to []delegate.DigestDestination, unread bool, now string) []Destination {
	out := []Destination{{Kind: kindFiles, At: now, Error: notSent}}
	done := map[string]bool{}
	for _, x := range old {
		switch {
		case x.Kind == kindFiles:
			if x.OK {
				out[0] = x
			}
		case x.OK || unread:
			out = append(out, x)
			done[x.Kind] = true
		}
	}
	for _, t := range to {
		if !done[t.Kind] {
			out = append(out, Destination{Kind: t.Kind, At: now, Channel: t.Channel, Error: notSent})
		}
	}
	return out
}

// slackName は Slack に上げるファイルの名前（決めたこと 13。日付は files/ と同じく期間の終わりの日）。
func slackName(env Env, in *Input) string {
	return fmt.Sprintf("digest-%s-%s.html", in.Project, filesEnv(env, in).Now.Format("2006-01-02"))
}

func sendSlack(env Env, channel, name, comment string, body []byte) (string, error) {
	s, err := newSlack(env)
	if err != nil {
		return "", err
	}
	return s.upload(channel, name, comment, body)
}

// CheckDestinations は andon digest check。project.json の送り先へ、確かめの小さなテキストのファイルを send と同じ口で送る（決めたこと 14）。
func CheckDestinations(env Env, w io.Writer) error {
	to, err := destinations(env)
	if err != nil {
		return fmt.Errorf("project.json の送り先を読めないので何も送らない: %s", oneLine(err.Error()))
	}
	if len(to) == 0 {
		return errors.New("project.json に digest の送り先の設定が無いので何も送らない")
	}
	project := filepath.Base(env.Root)
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
