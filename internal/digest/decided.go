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
	Mains     []MainRead   `json:"mains"`
}

// MainRead は repo の main をどこまで読んだか。次のダイジェストはここから比べる。
type MainRead struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
}

type lastRead struct {
	mains   map[string]string // repo → commit
	skipped map[string]bool   // skipped の source
}

// Decision は期間に main に入った、Chief Engineer の印の付いた決定の行 1 つ。
type Decision struct {
	Text   string  `json:"text"`
	Mark   string  `json:"mark"` // 括弧の中。書いてあるとおり
	Repo   string  `json:"repo"`
	Commit string  `json:"commit"` // 期間の終わりの main（path と line はその木のもの）
	Path   string  `json:"path"`
	Line   int     `json:"line"`
	URL    *string `json:"url" andon:"nullable"`
}

type Triaged struct {
	Issue  string `json:"issue"`
	Title  string `json:"title"`
	At     string `json:"at"`
	Closed int    `json:"closed"`
	Cut    int    `json:"cut"`
	Path   string `json:"path"`
}

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
	emptyTree      = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // git の空の木（期間に最初の commit があるとき、その前の木）
	githubHost     = "github.com/"
	dispatcherBy   = "terminal:dispatcher"
	terminalBy     = "terminal:"
)

func (c *collector) decided(rows []ledger.Row, since, until time.Time, prev *Previous) *Decided {
	in := func(t time.Time) bool { return !t.Before(since) && !t.After(until) }
	last := c.lastRead(prev, since)
	d := &Decided{Triage: c.triaged(rows, in), Retro: c.retroed(in, last.skipped), Answers: c.scopeReplies(in)}
	d.Decisions, d.Mains = c.decisions(since, until, last.mains)
	return d
}

// lastRead は前のダイジェストの input.json を読む。7 日で切った期間と、送ったあとに集め直した入力には使わない（時刻で切る）。
func (c *collector) lastRead(prev *Previous, since time.Time) lastRead {
	out := lastRead{mains: map[string]string{}, skipped: map[string]bool{}}
	if prev == nil || prev.Until != since.Format(timeLayout) {
		return out
	}
	var was struct {
		Until   string `json:"until"`
		Skipped []Skip `json:"skipped"`
		Decided *struct {
			Mains []MainRead `json:"mains"`
		} `json:"decided"`
	}
	if !c.readJSON(filepath.Join(c.lib(prev.Issue), dirName, inputFile), &was) || was.Until != prev.Until || was.Decided == nil {
		return out
	}
	for _, m := range was.Decided.Mains {
		out.mains[m.Repo] = m.Commit
	}
	for _, x := range was.Skipped {
		out.skipped[x.Source] = true
	}
	return out
}

// decisions は kanban の課題の repo ごとに、期間に main に入った決定の行を repo・文書・行の順に並べる。
func (c *collector) decisions(since, until time.Time, seen map[string]string) ([]Decision, []MainRead) {
	repos := map[string]bool{}
	for _, t := range c.byID {
		if t.repo != "" {
			repos[t.repo] = true
		}
	}
	out, mains := []Decision{}, []MainRead{}
	for _, repo := range sortedKeys(repos) {
		got, read, err := repoDecisions(repo, seen[repo], since, until)
		if err != nil {
			c.skip(repo, "期間に main に入った文書を読めない（%v）。代わりに決めたことの決定の行を探していない", err)
			read = seen[repo] // 読めなかった分は次のダイジェストが同じ所から読む
		}
		if read != "" {
			mains = append(mains, MainRead{Repo: repo, Commit: read})
		}
		out = append(out, got...)
	}
	return out, mains
}

// repoDecisions は、期間の前と期間の終わりの main（first-parent）を比べて増えた決定の行と、読んだ main の commit。
// 期間の前の main は seen（前のダイジェストが読んだ main）。fast-forward の push は commit の時刻が古いまま入るので、時刻だけでは切らない。
// seen が無いか main に無ければ、committer の時刻で切る。
// commit ごとに拾わないのは、期間の中で語を直された行が2度並び、取り消された行も残るため。
// 手元の origin/main は古いことがあるので取り直す。collect は checkout に何も書かないので、取り直す先は object を借りる一時の bare の写しにする。
func repoDecisions(repo, seen string, since, until time.Time) ([]Decision, string, error) {
	checkout, err := spawn.RepoPath(repo)
	if err != nil {
		return nil, "", err
	}
	origin, err := git(checkout, "remote", "get-url", "origin")
	if err != nil {
		return nil, "", err
	}
	tmp, err := os.MkdirTemp("", "andon-digest-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // 一時の写しで、消しそこねても集めた中身に効かない
	if _, err := git(checkout, "clone", "-q", "--bare", "--shared", checkout, tmp); err != nil {
		return nil, "", err
	}
	if _, err := git(tmp, "fetch", "-q", "--no-tags", strings.TrimSpace(string(origin)), "HEAD"); err != nil {
		return nil, "", err
	}
	list, err := git(tmp, "rev-list", "--first-parent", "--timestamp", "--parents", "FETCH_HEAD")
	if err != nil {
		return nil, "", err
	}
	var tip, base, end string
	found := false
	for line := range strings.Lines(string(list)) { // 新しい順に <committer の時刻> <commit> [<親>…]
		f := strings.Fields(line)
		if len(f) < 2 {
			return nil, "", fmt.Errorf("git rev-list の行が読めない: %q", line)
		}
		if tip == "" {
			tip = f[1]
		}
		if f[1] == seen {
			found = true
			break
		}
		sec, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			return nil, "", fmt.Errorf("git rev-list の時刻が読めない: %q", line)
		}
		at := time.Unix(sec, 0)
		if at.After(until) {
			continue
		}
		if end == "" {
			end, base = f[1], f[1]
		}
		if !at.Before(since) {
			base = emptyTree
			if len(f) > 2 {
				base = f[2]
			}
		}
	}
	if found {
		base, end = seen, tip
	}
	if base == end {
		return nil, end, nil
	}
	out, err := addedDecisions(tmp, base, end)
	if err != nil {
		return nil, "", err
	}
	for i := range out {
		out[i].Repo, out[i].Commit = repo, end
		out[i].URL = blobURL(repo, end, out[i].Path, out[i].Line)
	}
	return out, end, nil
}

// addedDecisions は docs/ の変わった文書のうち、head にあって base に無い決定の行。base の決定の行と同じとみなすのは、
// 同じ 1 文（どの文書でも）か、残りのうち同じ文書・番号・日付の行。差分の + の行をそのまま拾わないのは、
// 語・本文・括弧の中だけを直した行と、ほかの文書へ移した行を、新しい決定として出さないため。
func addedDecisions(dir, base, head string) ([]Decision, error) {
	b, err := git(dir, "diff-tree", "-r", "-z", "--no-renames", "--name-status", base, head, "--", docsDir)
	if err != nil {
		return nil, err
	}
	var was, now []markedLine
	read := func(commit, path string, to *[]markedLine) error {
		doc, err := git(dir, "cat-file", "blob", commit+":"+path)
		if err != nil {
			return err
		}
		for _, d := range markedDecisions(doc) {
			d.Path = path
			*to = append(*to, d)
		}
		return nil
	}
	f := strings.Split(string(b), "\x00") // <A・M・D・T> <path> の繰り返し
	for i := 0; i+1 < len(f); i += 2 {
		status, path := f[i], f[i+1]
		if !strings.HasSuffix(path, ".md") {
			continue
		}
		if status != "A" {
			if err := read(base, path, &was); err != nil {
				return nil, err
			}
		}
		if status != "D" {
			if err := read(head, path, &now); err != nil {
				return nil, err
			}
		}
	}
	// 同じ 1 文を先に除く（割り込んだ決定に番号を奪われた行を、直した行と読まないため）
	now, was = unmatched(now, was, func(a, b markedLine) bool { return a.Text == b.Text })
	now, _ = unmatched(now, was, func(a, b markedLine) bool {
		return a.date != "" && a.date == b.date && a.number == b.number && a.Path == b.Path
	})
	var out []Decision
	for _, d := range now {
		out = append(out, d.Decision)
	}
	return out, nil
}

// unmatched は now と was の行を same で 1 対 1 に組にし、組にならなかった行を返す。
func unmatched(now, was []markedLine, same func(a, b markedLine) bool) (left, rest []markedLine) {
	rest = slices.Clone(was)
	for _, a := range now {
		if i := slices.IndexFunc(rest, func(b markedLine) bool { return same(a, b) }); i >= 0 {
			rest = slices.Delete(rest, i, i+1)
		} else {
			left = append(left, a)
		}
	}
	return left, rest
}

// markedLine は文書の中の決定の行。number と date（1 文か括弧の中の最初の日付）は、語を直しただけの行を見分けるため。
type markedLine struct {
	Decision
	number, date string
}

var (
	headingRe  = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)
	numberedRe = regexp.MustCompile(`^ {0,3}([0-9]+)\.[ \t]+\*\*(.*)$`)
	dateRe     = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)
)

// markedDecisions は文書の「決めたこと」の節の番号つきの行のうち、印の付いたもの。コードの囲みの中は読まない。
func markedDecisions(doc []byte) []markedLine {
	var out []markedLine
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
			if text, mark, ok := splitMark(m[2]); ok {
				out = append(out, markedLine{Decision{Text: text, Mark: mark, Line: i + 1}, m[1], dateRe.FindString(text + " " + mark)})
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

// retroed は期間に当てた振り返りの記録（retro-applied.json）。applied_at は当て始めの時刻なので、
// 前のダイジェストが読み飛ばした記録（skipped）は、期間の前の時刻でも数える。
func (c *collector) retroed(in func(time.Time) bool, skipped map[string]bool) []Retroed {
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
		case !in(at) && !skipped[c.rel(path)]:
		case !rec.Complete:
			c.skip(c.rel(path), "振り返りを当て終えていない（数えていない。当て終えたあとのダイジェストに載る）")
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
