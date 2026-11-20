package retro

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"andon/internal/launch"
	"andon/internal/spawn"
)

// Typed はオーナーが端末で打った文1つ（ADR-0028 決めたこと 4・14 の 1）。根拠として指す id は h-<セッションログの行の uuid>。
// 同じセッションの文は根拠として1件に数える（決めたこと 14 の 2）ので、session を一緒に出す。
type Typed struct {
	ID        string  `json:"id"`
	Session   string  `json:"session"`
	At        string  `json:"at"`
	Role      string  `json:"role"`  // chief-of-staff・chief-engineer・worker・reviewer
	Issue     *string `json:"issue"` // Chief of Staff は null
	Text      string  `json:"text"`
	Truncated bool    `json:"truncated"`
}

// maxTyped は1つの文の長さの上限（文字）。超えた分は切って truncated にする（貼った長い文書が入力を埋めないように）。
const maxTyped = 2000

// owner はセッションを持つもの（このプロジェクトの役と課題）。
type owner struct {
	role  string
	issue string
}

// wrapped は機械が送った文の包み（hc send・dispatcher の通知・ハーネスの通知）。貼った文の包みの内側も見る。
var (
	pastedOpen = regexp.MustCompile(`^<pasted_content[^>]*>\s*`)
	wrappers   = []string{"<hc from=", "<news ", "<news>", "<task-notification>"}
)

func wrappedText(s string) bool {
	s = strings.TrimSpace(s)
	s = pastedOpen.ReplaceAllString(s, "")
	for _, w := range wrappers {
		if strings.HasPrefix(s, w) {
			return true
		}
	}
	return false
}

// machineText は Andon の command が端末の外から hc send した文か（hc の包みが付かず、オーナーの印が付く）。
// dispatcher とゲートは、担当・レビュアーへ送る文の先頭に送り手の印（[dispatcher]・[gate]。spawn.Mark）を置く。
// その印で照らし、文の意味は読まない。貼った文の包みの内側も見る。起動の最初のメッセージは位置で除く。
func machineText(s string) bool {
	return spawn.Marked(pastedOpen.ReplaceAllString(strings.TrimSpace(s), ""))
}

// roleOf は課題のプロファイルから役を決める（起動の記録の role と同じ決め方）。
func roleOf(t *task) string {
	switch {
	case t.profile == "chief-engineer":
		return launch.RoleChiefEngineer
	case t.reviewer():
		return launch.RoleReviewer
	}
	return launch.RoleWorker
}

// typed は期間にオーナーが打った文を、このプロジェクトの Claude Code のセッションログから抜く（Codex の記録にはオーナーの印が無いので抜かない）。
//
// セッションを持つものは、起動の記録の session（課題と Chief of Staff）か、ログの cwd（プロジェクトの根は Chief of Staff、
// 台帳の issues/<id>/ と課題の worktree はその課題）で決める。どちらにも当たらないセッションは読まない。
// 抜く文: user の行で、本線（isSidechain でない）・isMeta でない・要約でない・オーナーの印（origin.kind が human）があり、
// セッションの最初のオーナーの印の文（起動の最初のメッセージ）でなく、包み（hc send・通知）が無く、Andon の command が端末の外から送った
// 定型の文（machineText）でもなく、時刻が期間の中のもの。文の意味は読まない。
func (r *reader) typed(tasks []*task, since, until time.Time) []Typed {
	out := []Typed{}
	bySession := map[string]owner{}
	byCwd := map[string]owner{}
	root := r.o.Root
	if a, err := filepath.Abs(root); err == nil {
		root = a
	}
	byCwd[root] = owner{role: launch.RoleChiefOfStaff}
	for _, t := range tasks {
		o := owner{role: roleOf(t), issue: t.id}
		byCwd[filepath.Join(root, "kanban", "issues", t.id)] = o
		if t.worktree != "" {
			byCwd[filepath.Clean(t.worktree)] = o
		}
		for _, l := range r.launchesQuiet(filepath.Join(r.dir, "issues", t.id, "library", launch.File)) {
			if s, ok := l["session"].(string); ok && s != "" {
				bySession[s] = o
			}
		}
	}
	for _, l := range r.launchesQuiet(launch.ChiefOfStaffPath(r.dir)) {
		if s, ok := l["session"].(string); ok && s != "" {
			bySession[s] = owner{role: launch.RoleChiefOfStaff}
		}
	}
	files, _ := filepath.Glob(filepath.Join(r.o.ClaudeDir, "projects", "*", "*.jsonl"))
	sort.Strings(files)
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil || st.ModTime().Before(since) {
			continue // 期間より前に書き終わったログに期間の文は無い
		}
		out = append(out, r.typedFile(f, bySession, byCwd, since, until)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// launchesQuiet は起動の記録を読む（読めない行は黙って飛ばす。束ねる段が skipped に書く）。
func (r *reader) launchesQuiet(path string) []map[string]any {
	saved := r.skipped
	l := r.launches(path)
	r.skipped = saved
	return l
}

// userLine はセッションログの1行のうち読む所。
type userLine struct {
	Type             string `json:"type"`
	UUID             string `json:"uuid"`
	Timestamp        string `json:"timestamp"`
	Cwd              string `json:"cwd"`
	IsSidechain      bool   `json:"isSidechain"`
	IsMeta           bool   `json:"isMeta"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Origin           *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// text は user の行の文。文字列か text の block を改行でつないだもの。道具の結果の block があれば ok は false。
func (u *userLine) text() (string, bool) {
	var s string
	if json.Unmarshal(u.Message.Content, &s) == nil {
		return s, true
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(u.Message.Content, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "tool_result":
			return "", false
		}
	}
	return strings.Join(parts, "\n"), len(parts) > 0
}

func (r *reader) typedFile(f string, bySession, byCwd map[string]owner, since, until time.Time) []Typed {
	session := strings.TrimSuffix(filepath.Base(f), ".jsonl")
	o, known := bySession[session]
	fh, err := os.Open(f)
	if err != nil {
		return nil
	}
	defer fh.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	var out []Typed
	first := true
	br := bufio.NewReader(fh)
	for n := 1; ; n++ {
		b, rerr := br.ReadBytes('\n')
		if len(b) > 0 && (!known || strings.Contains(string(b), `"human"`)) {
			var u userLine
			if json.Unmarshal(b, &u) == nil {
				if !known && u.Cwd != "" {
					o, known = byCwd[filepath.Clean(u.Cwd)]
					if !known {
						return nil // このプロジェクトのセッションでない
					}
				}
				if t, ok := r.typedLine(&u, session, n, o, &first, since, until); ok && known {
					out = append(out, t)
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	if !known {
		return nil
	}
	return out
}

// typedLine は1行が抜く文なら返す。first は「まだオーナーの印の文を見ていない」（見たら false にする）。
func (r *reader) typedLine(u *userLine, session string, n int, o owner, first *bool, since, until time.Time) (Typed, bool) {
	if u.Type != "user" || u.IsSidechain || u.IsMeta || u.IsCompactSummary || u.Origin == nil || u.Origin.Kind != "human" {
		return Typed{}, false
	}
	text, ok := u.text()
	if !ok {
		return Typed{}, false
	}
	if *first {
		*first = false
		return Typed{}, false // 起動の最初のメッセージ
	}
	at, err := time.Parse(time.RFC3339, u.Timestamp)
	if err != nil || at.Before(since) || at.After(until) || wrappedText(text) || machineText(text) {
		return Typed{}, false
	}
	id := "h-" + u.UUID
	if u.UUID == "" {
		id = "h-" + session + "-" + strconv.Itoa(n)
	}
	t := Typed{ID: id, Session: session, At: at.UTC().Format(time.RFC3339), Role: o.role, Issue: strPtr(o.issue), Text: text}
	if rs := []rune(text); len(rs) > maxTyped {
		t.Text, t.Truncated = string(rs[:maxTyped]), true
	}
	return t, true
}
