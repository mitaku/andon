// Package signal は外の出所の信号を読む（andon signal。ADR-0011 決めたこと 3・5・8・9・10、docs/contract.md §15）。
//
// collect は project.json の signals に書かれた出所を読み、信号の記録 <slug>.signals.json だけを書く。出所へは書かない
// （GitHub は gh の資格情報が書ける token でも、打つのは読む API だけ）。最初の版で読むのは GitHub の main の CI の失敗と
// open の issue。Sentry・Linear・Cloudflare は auth でトークンを置く器だけで、読む command は使う案件が出たときに足す。
package signal

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
)

// Usage は andon signal の使い方。
const Usage = "usage: andon signal collect [--project <slug>]   project.json の signals の出所を読み、信号の記録を書く\n" +
	"       andon signal auth <source>             出所のトークンをセルの中で聞いて置く（sentry・linear・cloudflare）\n" +
	"collect は projects/<slug>/ で打つか --project。記録は ~/.local/state/andon/dispatcher/<slug>.signals.json\n" +
	"（XDG_STATE_HOME があればその下。docs/contract.md §15）。signals が無ければ何も読まず、何も書かない。\n" +
	"GitHub（github-ci・github-issues）は gh の資格情報を使う。読めなかった出所はその出所の記録を変えずに飛ばし、終了コード 1。\n" +
	"auth は標準入力の1行をトークンとして ~/.config/andon/signals/<source>.token（mode 600）に置く。トークンは読み取り専用の scope で作る。\n"

// Record は信号の記録の1件（schema/signal.schema.json）。
type Record struct {
	Source           string `json:"source"`
	Project          string `json:"project"`
	Key              string `json:"key"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	Weight           string `json:"weight"`
	Fingerprint      string `json:"fingerprint"`
	FirstSeen        string `json:"first_seen"`
	LastSeen         string `json:"last_seen"`
	Count            int    `json:"count"`
	Active           bool   `json:"active"`
	Episodes         int    `json:"episodes"`
	DeliveredEpisode int    `json:"delivered_episode"`
	Release          string `json:"release,omitempty"`
}

// File は <slug>.signals.json。
type File struct {
	Signals []Record `json:"signals"`
}

// Seen は出所を1回読んで見えた信号1つ。
type Seen struct {
	Key, URL, Title, Release string
}

// GH は gh を引数で打ち、標準出力を返す。
type GH func(args ...string) ([]byte, error)

// Options は collect の入力。
type Options struct {
	Slug     string
	Project  *delegate.Project
	Path     string // 記録の置き場
	TokenDir string // auth が置くトークンの dir
	Now      time.Time
	GH       GH
}

// Path は記録の置き場（dispatcher の控えと同じ dir）。
func Path(slug string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "dispatcher", slug+".signals.json")
}

// TokenDir は auth がトークンを置く dir（ADR-0011 決めたこと 10。compose の volume）。
func TokenDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "andon", "signals")
}

// Fingerprint は source と改行と key の SHA-1（contract.md §15）。
func Fingerprint(source, key string) string {
	h := sha1.Sum([]byte(source + "\n" + key))
	return hex.EncodeToString(h[:])
}

var (
	ctrlRe = regexp.MustCompile(`[\x00-\x1f\x7f]+`)
	urlRe  = regexp.MustCompile(`^https://[!-~]+$`)
	shaRe  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// line は出所の題を1行にする（制御文字を空白に。中身は信用しないまま写す）。
func line(s string) string { return strings.TrimSpace(ctrlRe.ReplaceAllString(s, " ")) }

// Load は記録を読む。無ければ空。
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{Signals: []Record{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err := contract.Decode(b, &f); err != nil {
		return nil, fmt.Errorf("信号の記録が読めない（%s）: %v", path, err)
	}
	if f.Signals == nil {
		f.Signals = []Record{}
	}
	return &f, nil
}

// Collect は出所を読み、記録を書く。読めなかった出所の数を返す（記録が壊れているなど続けられないときは誤り）。
func Collect(o Options, w io.Writer) (int, error) {
	if o.Project == nil || len(o.Project.Signals) == 0 {
		fmt.Fprintf(w, "%s: project.json に signals が無い。何も読まない\n", o.Slug)
		return 0, nil
	}
	f, err := Load(o.Path)
	if err != nil {
		return 0, err
	}
	now := o.Now.UTC().Format("2006-01-02T15:04:05Z")
	failed, read := 0, 0
	done := map[string]bool{}
	for _, s := range o.Project.Signals {
		switch s.Source {
		case "github-ci", "github-issues":
		default:
			if _, err := os.Stat(filepath.Join(o.TokenDir, s.Source+".token")); err != nil {
				fmt.Fprintf(w, "%s: トークンが無いので飛ばす（andon signal auth %s）\n", s.Source, s.Source)
			} else {
				fmt.Fprintf(w, "%s: 読む command がまだ無いので飛ばす（ADR-0011 決めたこと 3）\n", s.Source)
			}
			continue
		}
		repos := o.Project.Repos
		if s.Repo != nil {
			repos = []string{*s.Repo}
		}
		for _, repo := range repos {
			unit := s.Source + " " + repo
			if done[unit] {
				continue
			}
			done[unit] = true
			var seen []Seen
			var judged func(key string) bool
			var prefix string
			if s.Source == "github-ci" {
				seen, judged, err = readCI(o.GH, repo)
				prefix = repo + "/"
			} else {
				seen, judged, err = readIssues(o.GH, repo, s.Filter)
				prefix = repo + "#"
			}
			if err != nil {
				failed++
				fmt.Fprintf(w, "%s: 読めないので記録を変えずに飛ばす（次に打ち直す）: %v\n", unit, err)
				continue
			}
			read++
			n, again, gone := merge(f, o.Slug, s.Source, s.Weight, prefix, seen, judged, now)
			fmt.Fprintf(w, "%s: 見えている %d 件（新しい %d・また出た %d・消えた %d）\n", unit, len(seen), n, again, gone)
		}
	}
	if read == 0 {
		return failed, nil
	}
	if err := os.MkdirAll(filepath.Dir(o.Path), 0o755); err != nil {
		return failed, err
	}
	return failed, contract.Write(o.Path, f)
}

// merge は1つの出所（source と repo）を1回読んだ結果を記録に当てる。見えたものは count を1増やし、
// 消えていたもの（active: false）がまた見えたら episodes を1増やす。この出所の記録で見えなかったもののうち、
// 読んだ範囲で結果が分かったもの（judged）は active: false。分からなかったもの（読める件数の外）は変えない。delivered_episode は dispatcher が書くので触らない（ADR-0011 決めたこと 8）。
func merge(f *File, slug, source, weight, prefix string, seen []Seen, judged func(string) bool, now string) (added, again, gone int) {
	at := map[string]int{}
	for i, r := range f.Signals {
		at[r.Fingerprint] = i
	}
	hit := map[string]bool{}
	for _, s := range seen {
		fp := Fingerprint(source, s.Key)
		if hit[fp] {
			continue
		}
		hit[fp] = true
		i, ok := at[fp]
		if !ok {
			f.Signals = append(f.Signals, Record{Source: source, Project: slug, Key: s.Key, URL: s.URL, Title: s.Title,
				Weight: weight, Fingerprint: fp, FirstSeen: now, LastSeen: now, Count: 1, Active: true, Episodes: 1, Release: s.Release})
			added++
			continue
		}
		r := &f.Signals[i]
		if !r.Active {
			r.Active = true
			r.Episodes++
			again++
		}
		r.Count++
		r.LastSeen, r.URL, r.Title, r.Weight, r.Release = now, s.URL, s.Title, weight, s.Release
	}
	for i := range f.Signals {
		r := &f.Signals[i]
		if r.Source == source && r.Project == slug && strings.HasPrefix(r.Key, prefix) && r.Active && !hit[r.Fingerprint] && judged(r.Key) {
			r.Active = false
			gone++
		}
	}
	return
}

// split は <host>/<owner>/<repo> を host と owner/repo に分ける。
func split(repo string) (string, string) {
	host, rest, _ := strings.Cut(repo, "/")
	return host, rest
}

// readCI は repo の main の CI を読み、workflow ごとに最新の結果が失敗のものを返す（key は <repo>/<workflow のファイル名>@main）。
// 見るのは結果の出た run（成功・失敗・時間切れ・起動の失敗）だけで、取り消し・飛ばしは workflow の状態を決めない。
// 読むのは新しい 100 件で、その中に結果の出た run の無い workflow は judged が false（消えたことにしない）。
func readCI(gh GH, repo string) ([]Seen, func(string) bool, error) {
	host, name := split(repo)
	b, err := gh("api", "--hostname", host, "repos/"+name+"/actions/runs?branch=main&status=completed&per_page=100")
	if err != nil {
		return nil, nil, err
	}
	var res struct {
		WorkflowRuns []struct {
			Path         string `json:"path"`
			Name         string `json:"name"`
			Event        string `json:"event"`
			Conclusion   string `json:"conclusion"`
			HTMLURL      string `json:"html_url"`
			HeadSHA      string `json:"head_sha"`
			DisplayTitle string `json:"display_title"`
		} `json:"workflow_runs"`
	}
	if err := jsonDecode(b, &res); err != nil {
		return nil, nil, fmt.Errorf("gh api の応答が読めない: %v", err)
	}
	decided := map[string]bool{}
	var out []Seen
	for _, r := range res.WorkflowRuns { // 新しい順
		file := filepath.Base(r.Path)
		if r.Path == "" || decided[file] || strings.HasPrefix(r.Event, "pull_request") ||
			!slices.Contains([]string{"success", "failure", "timed_out", "startup_failure"}, r.Conclusion) {
			continue
		}
		decided[file] = true
		if r.Conclusion == "success" || !urlRe.MatchString(r.HTMLURL) {
			continue
		}
		s := Seen{Key: repo + "/" + file + "@main", URL: r.HTMLURL, Title: line(r.Name + ": " + r.DisplayTitle)}
		if shaRe.MatchString(r.HeadSHA) {
			s.Release = r.HeadSHA
		}
		out = append(out, s)
	}
	return out, func(key string) bool { return decided[strings.TrimSuffix(strings.TrimPrefix(key, repo+"/"), "@main")] }, nil
}

// readIssues は repo の open の issue を読む（key は <repo>#<n>）。filter は GitHub の検索の語としてそのまま渡す（決めたこと 9）。
// 上限（issueLimit 件）まで返ったら、その外があるかもしれないので judged は false（どれも消えたことにしない）。
func readIssues(gh GH, repo string, filter *string) ([]Seen, func(string) bool, error) {
	args := []string{"issue", "list", "--repo", repo, "--state", "open", "--limit", strconv.Itoa(issueLimit), "--json", "number,title,url"}
	if filter != nil {
		args = append(args, "--search", *filter)
	}
	b, err := gh(args...)
	if err != nil {
		return nil, nil, err
	}
	var res []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}
	if err := jsonDecode(b, &res); err != nil {
		return nil, nil, fmt.Errorf("gh issue list の応答が読めない: %v", err)
	}
	var out []Seen
	for _, r := range res {
		if r.Number < 1 || !urlRe.MatchString(r.URL) {
			continue
		}
		out = append(out, Seen{Key: fmt.Sprintf("%s#%d", repo, r.Number), URL: r.URL, Title: line(r.Title)})
	}
	complete := len(res) < issueLimit
	return out, func(string) bool { return complete }, nil
}

// issueLimit は1回に読む issue の上限。
var issueLimit = 1000

// RunGH は本物の gh。誤りには stderr の最後の行を添える。
func RunGH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return nil, fmt.Errorf("gh %s: %v %s", args[0], err, msg)
	}
	return out, nil
}
