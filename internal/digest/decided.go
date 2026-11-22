package digest

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// Decided はオーナーの代わりに決められたこと（ADR-0044 決めたこと 6 の節 5。docs/contract.md §26 の decided）。
type Decided struct {
	Decisions []Decision   `json:"decisions"`
	Triage    []Triaged    `json:"triage"`
	Retro     []Retroed    `json:"retro"`
	Answers   []ScopeReply `json:"answers"`
}

// Decision は期間に main に入った、Chief Engineer の印の付いた決定の行 1 つ。
type Decision struct {
	Text   string  `json:"text"`
	Mark   string  `json:"mark"` // 括弧の中。書いてあるとおり
	Repo   string  `json:"repo"`
	Commit string  `json:"commit"`
	At     string  `json:"at"`
	Path   string  `json:"path"`
	Line   int     `json:"line"`
	URL    *string `json:"url" andon:"nullable"`
}

// Triaged は期間に片付いたトリアージ 1 回。
type Triaged struct {
	Issue  string `json:"issue"`
	Title  string `json:"title"`
	At     string `json:"at"`
	Closed int    `json:"closed"`
	Cut    int    `json:"cut"`
	Path   string `json:"path"`
}

// Retroed は期間に当てた振り返り 1 回。
type Retroed struct {
	Issue   string `json:"issue"`
	Title   string `json:"title"`
	At      string `json:"at"`
	Fix     int    `json:"fix"`
	Dropped int    `json:"dropped"`
	Path    string `json:"path"`
}

// ScopeReply は期間に Chief of Staff が答えた範囲の問い。
type ScopeReply struct {
	Issue    string `json:"issue"`
	Title    string `json:"title"`
	N        int    `json:"n"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	At       string `json:"at"`
	Path     string `json:"path"`
}

const (
	decidedHeading = "決めたこと"            // 決定の節の見出しの頭
	decidedMark    = "Chief Engineer が" // 拾う括弧の印（決めたこと 6 の細部）
	docsDir        = "docs/"
	githubHost     = "github.com/"
	dispatcherBy   = "terminal:dispatcher"
	terminalBy     = "terminal:"
)

func (c *collector) decided(rows []ledger.Row, in func(time.Time) bool) *Decided {
	return &Decided{Decisions: c.decisions(in), Triage: c.triaged(rows, in), Retro: c.retroed(in), Answers: c.scopeReplies(in)}
}

// decisions は kanban の課題の repo ごとに、期間に main に入った決定の行を古い順に並べる。
func (c *collector) decisions(in func(time.Time) bool) []Decision {
	repos := map[string]bool{}
	for _, t := range c.byID {
		if t.repo != "" {
			repos[t.repo] = true
		}
	}
	out := []Decision{}
	for _, repo := range sortedKeys(repos) {
		got, err := repoDecisions(repo, in)
		if err != nil {
			c.skip(repo, "期間に main に入った文書を読めない（%v）。代わりに決めたことの決定の行を探していない", err)
			continue
		}
		out = append(out, got...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// repoDecisions は repo の最新の main の first-parent の commit のうち期間のものが足した決定の行を読む。
// 手元の origin/main は古いことがあり、漏らした行は次の期間にも載らないので取り直す。collect は checkout に何も書かないので、
// 取り直す先は object を借りる一時の bare の写しにする。
func repoDecisions(repo string, in func(time.Time) bool) ([]Decision, error) {
	checkout, err := spawn.RepoPath(repo)
	if err != nil {
		return nil, err
	}
	origin, err := git(checkout, "remote", "get-url", "origin")
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "andon-digest-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // 一時の写しで、消しそこねても集めた中身に効かない
	if _, err := git(checkout, "clone", "-q", "--bare", "--shared", checkout, tmp); err != nil {
		return nil, err
	}
	if _, err := git(tmp, "fetch", "-q", "--no-tags", strings.TrimSpace(string(origin)), "HEAD"); err != nil {
		return nil, err
	}
	list, err := git(tmp, "rev-list", "--first-parent", "--timestamp", "--parents", "FETCH_HEAD")
	if err != nil {
		return nil, err
	}
	var out []Decision
	lines := strings.Split(strings.TrimSpace(string(list)), "\n")
	for _, line := range slices.Backward(lines) {
		// <マージの時刻> <commit> [<親>…]
		f := strings.Fields(line)
		if len(f) < 2 {
			return nil, fmt.Errorf("git rev-list の行が読めない: %q", line)
		}
		sec, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("git rev-list の時刻が読めない: %q", line)
		}
		at := time.Unix(sec, 0).UTC()
		if !in(at) {
			continue
		}
		parent := ""
		if len(f) > 2 {
			parent = f[2]
		}
		added, err := addedDecisions(tmp, f[1], parent)
		if err != nil {
			return nil, err
		}
		for _, d := range added {
			d.Repo, d.Commit, d.At = repo, f[1], at.Format(timeLayout)
			d.URL = blobURL(repo, d.Commit, d.Path, d.Line)
			out = append(out, d)
		}
	}
	return out, nil
}

// addedDecisions は commit の docs/ の文書にあって、親の同じ文書に無い決定の行。
// 差分の + の行をそのまま拾わないのは、本文だけを直した行と、名前を変えた文書の行を、新しい決定として出さないため。
func addedDecisions(dir, commit, parent string) ([]Decision, error) {
	args := []string{"diff-tree", "-r", "-z", "-M", "--name-status", "--no-commit-id"}
	if parent == "" {
		args = append(args, "--root", commit)
	} else {
		args = append(args, parent, commit)
	}
	b, err := git(dir, append(args, "--", docsDir)...)
	if err != nil {
		return nil, err
	}
	f := strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
	var out []Decision
	for i := 0; i+1 < len(f); i += 2 {
		status, before, after := f[i], f[i+1], f[i+1]
		if strings.HasPrefix(status, "R") && i+2 < len(f) {
			after = f[i+2]
			i++
		}
		if status == "D" || !strings.HasSuffix(after, ".md") {
			continue
		}
		had := map[markedLine]int{}
		if status != "A" {
			old, err := git(dir, "cat-file", "blob", parent+":"+before)
			if err != nil {
				return nil, err
			}
			for _, d := range markedDecisions(old) {
				had[markedLine{d.Text, d.Mark}]++
			}
		}
		doc, err := git(dir, "cat-file", "blob", commit+":"+after)
		if err != nil {
			return nil, err
		}
		for _, d := range markedDecisions(doc) {
			if k := (markedLine{d.Text, d.Mark}); had[k] > 0 {
				had[k]--
				continue
			}
			d.Path = after
			out = append(out, d)
		}
	}
	return out, nil
}

// markedLine は決定の行の同一（番号と行の位置は見ない）。
type markedLine struct{ text, mark string }

var (
	headingRe  = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)
	numberedRe = regexp.MustCompile(`^ {0,3}[0-9]+\.[ \t]+\*\*(.*)$`)
)

// markedDecisions は文書の「決めたこと」の節の番号つきの行のうち、印の付いたもの。コードの囲みの中は読まない。
func markedDecisions(doc []byte) []Decision {
	var out []Decision
	level := 0 // いまいる決定の節の見出しの深さ（0 なら節の外）
	fence := ""
	for i, line := range strings.Split(string(doc), "\n") {
		line = strings.TrimRight(line, "\r")
		lead := strings.TrimLeft(line, " \t")
		if fence != "" {
			if strings.HasPrefix(lead, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(lead, "```") || strings.HasPrefix(lead, "~~~") {
			fence = lead[:3]
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			n := len(m[1])
			if level > 0 && n <= level {
				level = 0
			}
			if level == 0 && strings.HasPrefix(m[2], decidedHeading) {
				level = n
			}
			continue
		}
		if level == 0 {
			continue
		}
		if m := numberedRe.FindStringSubmatch(line); m != nil {
			if text, mark, ok := splitMark(m[1]); ok {
				out = append(out, Decision{Text: text, Mark: mark, Line: i + 1})
			}
		}
	}
	return out
}

// splitMark は太字の 1 文（s は開きの ** の後ろ）と、その括弧の印を分ける。
// 括弧は太字の中のものと、閉じの ** のすぐ後ろに続くものの両方（ADR-0037 決めたこと 3 は後ろに書いている）。
func splitMark(s string) (text, mark string, ok bool) {
	end, code := -1, false
	for i := range len(s) {
		if s[i] == '`' {
			code = !code
		} else if !code && strings.HasPrefix(s[i:], "**") { // コードの中の ** は閉じでない
			end = i
			break
		}
	}
	if end < 0 {
		return "", "", false
	}
	bold, after := s[:end], strings.TrimLeft(s[end+2:], " ")
	groups := parens(bold)
	for _, g := range slices.Backward(groups) {
		if in := bold[g[0]+len("（") : g[1]-len("）")]; strings.Contains(in, decidedMark) {
			return bold[:g[0]] + bold[g[1]:], in, true
		}
	}
	if g := parens(after); len(g) > 0 && g[0][0] == 0 {
		if in := after[len("（") : g[0][1]-len("）")]; strings.Contains(in, decidedMark) {
			return bold, in, true
		}
	}
	return "", "", false
}

// parens はいちばん外の全角の括弧の範囲。
func parens(s string) [][2]int {
	var out [][2]int
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '（':
			if depth == 0 {
				start = i
			}
			depth++
		case '）':
			if depth > 0 {
				if depth--; depth == 0 {
					out = append(out, [2]int{start, i + len("）")})
				}
			}
		}
	}
	return out
}

// blobURL は GitHub の repo の、その commit の文書の行（?plain=1 が無いと Markdown は行へ飛ばない）。
func blobURL(repo, commit, path string, line int) *string {
	if !strings.HasPrefix(repo, githubHost) {
		return nil
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return ptr(fmt.Sprintf("https://%s/blob/%s/%s?plain=1#L%d", repo, commit, strings.Join(parts, "/"), line))
}

// git は dir で git を打ち、stdout をそのまま返す。資格情報を尋ねて止まらない。SSH 鍵はセルに無いので github.com は HTTPS で読む。
func git(dir string, args ...string) ([]byte, error) {
	c := exec.Command("git", append([]string{"-C", dir, "-c", "url.https://github.com/.insteadOf=git@github.com:"}, args...)...)
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out, nil
}

var triageRecordRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}-(.+)\.json$`)

// triaged は期間に片付いたトリアージの記録。記録に当てた時刻が無いので、当てた直後に打つ片付け（done）の時刻で期間を切る。
func (c *collector) triaged(rows []ledger.Row, in func(time.Time) bool) []Triaged {
	out := []Triaged{}
	dir := filepath.Join(c.l.Dir, "library", "triage")
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out
	}
	if err != nil {
		c.skip(c.rel(dir), "読めない（トリアージで決めたことを数えていない）: %v", err)
		return out
	}
	done := c.doneAt(rows, in)
	for _, e := range ents {
		m := triageRecordRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		at, ok := done[m[1]]
		if !ok {
			continue
		}
		var rec struct {
			Items *[]struct {
				Decision string `json:"decision"`
				Created  string `json:"created_issue"`
			} `json:"items"`
		}
		path := filepath.Join(dir, e.Name())
		if !c.readJSON(path, &rec) {
			continue
		}
		if rec.Items == nil {
			c.skip(c.rel(path), "items が無い（トリアージで決めたことを数えていない）")
			continue
		}
		x := Triaged{Issue: m[1], Title: c.title(m[1]), At: at.Format(timeLayout), Path: c.rel(path)}
		cut := map[string]bool{} // bundle は複数の項目が同じ課題になる
		for _, it := range *rec.Items {
			switch it.Decision {
			case "close":
				x.Closed++
			case "fix", "bundle":
				if it.Created != "" {
					cut[it.Created] = true
				}
			}
		}
		x.Cut = len(cut)
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// retroed は期間に当てた振り返りの記録（retro-applied.json）。
func (c *collector) retroed(in func(time.Time) bool) []Retroed {
	out := []Retroed{}
	for _, id := range c.ids {
		var rec struct {
			AppliedAt string     `json:"applied_at"`
			Complete  bool       `json:"complete"`
			Issues    []struct{} `json:"issues"`
			Dropped   []struct{} `json:"dropped"`
		}
		path := filepath.Join(c.lib(id), "retro-applied.json")
		if !c.readJSON(path, &rec) {
			continue
		}
		at, err := time.Parse(time.RFC3339, rec.AppliedAt)
		switch {
		case err != nil:
			c.skip(c.rel(path), "applied_at が読めない（振り返りで決めたことを数えていない）")
		case !in(at):
		case !rec.Complete:
			c.skip(c.rel(path), "振り返りを当て終えていない（数えていない。当て直すと、その時刻の期間に載る）")
		default:
			out = append(out, Retroed{Issue: id, Title: c.title(id), At: at.UTC().Format(timeLayout),
				Fix: len(rec.Issues), Dropped: len(rec.Dropped), Path: c.rel(path)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// scopeReplies は期間に答えた範囲の問い。端末の外（オーナーの shell）と dispatcher が書いた答えは載せない
// （オーナーが打った文をセルの外へ出さない。決めたこと 6 の細部）。
func (c *collector) scopeReplies(in func(time.Time) bool) []ScopeReply {
	out := []ScopeReply{}
	for _, id := range c.ids {
		path := filepath.Join(c.l.Dir, "issues", id, "questions.yaml")
		b, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var list []struct {
			N          int    `yaml:"n"`
			Kind       string `yaml:"kind"`
			Text       string `yaml:"text"`
			Answer     string `yaml:"answer"`
			Answered   string `yaml:"answered"`
			AnsweredBy string `yaml:"answeredBy"`
		}
		if err != nil || yaml.Unmarshal(b, &list) != nil {
			c.skip(c.rel(path), "読めない（範囲の問いの答えを数えていない）")
			continue
		}
		for _, q := range list {
			if q.Kind != "scope" || q.Answered == "" {
				continue
			}
			at, err := time.Parse(timeLayout, q.Answered)
			if err != nil || q.N < 1 {
				c.skip(c.rel(path), "範囲の問いの番号か答えた時刻が読めない（数えていない）")
				continue
			}
			if !in(at) || !strings.HasPrefix(q.AnsweredBy, terminalBy) || q.AnsweredBy == dispatcherBy {
				continue
			}
			x := ScopeReply{Issue: id, Title: c.title(id), N: q.N, Answer: q.Answer, At: q.Answered, Path: c.rel(path)}
			// 問いの 1 行は問いの JSON。kanban の text は事実と選択肢も含む人向けの描画なので、JSON が無いときだけ頭の行を使う
			var asked contract.Question
			if c.readJSON(filepath.Join(c.lib(id), "questions", strconv.Itoa(q.N)+".json"), &asked) {
				x.Question = asked.Question
			} else {
				x.Question, _, _ = strings.Cut(q.Text, "\n")
			}
			out = append(out, x)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}
