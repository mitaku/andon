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
	Role      string  `json:"role"`
	Issue     *string `json:"issue"`
	Text      string  `json:"text"`
	Truncated bool    `json:"truncated"`
}

const maxTyped = 2000

type owner struct {
	role  string
	issue string
}

var (
	pastedOpen  = regexp.MustCompile(`^<pasted_content[^>]*>\s*`)
	pastedClose = regexp.MustCompile(`\s*</pasted_content[^>]*>$`)
	wrappers    = []string{"<hc from=", "<news ", "<news>", "<task-notification>"}
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

// andon issue resume は最初のメッセージの先頭に [resume] を置くが、hc run が template の本文と空行をその前に貼るので、
// 印は最後の段落の頭に来る。文の頭か最後の段落の頭の印で照らし、文の意味は読まない。貼った文の包みの内側も見る。
// 起動の最初のメッセージ（andon issue start）は位置で除く。resume はハーネスのセッションの続きに入るので位置で除けない。
func machineText(s string) bool {
	s = pastedClose.ReplaceAllString(pastedOpen.ReplaceAllString(strings.TrimSpace(s), ""), "")
	if spawn.Marked(s) {
		return true
	}
	if i := strings.LastIndex(s, "\n\n"); i >= 0 {
		return spawn.Marked(s[i+2:])
	}
	return false
}

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
			continue
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

func (r *reader) launchesQuiet(path string) []map[string]any {
	saved := r.skipped
	l := r.launches(path)
	r.skipped = saved
	return l
}

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
						return nil
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
		return Typed{}, false
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
