// Package ledger は台帳を読む（docs/contract.md §1・§2・§3）。台帳を書くのは台帳の CLI（kanban）だけで、
// ここはそれを打つか、台帳のファイルを読むだけ。Markdown（依頼書・報告）は読まない。
//
// command（andon task・andon gate）が共有するもの: タスクの項目（task.yaml の Andon の予約キー）と履歴の照合・
// レビュアーのタスクの選び方・findings.json の読み方・library/ の置き場。
package ledger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CLI は台帳の CLI の名前。別名の deck には落ちない（同じ名前の古い deck が版違いの振る舞いで動くのを避ける）。
const CLI = "kanban"

// Missing は台帳の CLI が PATH に無いときの言い方（入れ方の案内つき）。
const Missing = "kanban が無い（台帳の CLI。Andon の image に入っている。手元なら go install ./cmd/kanban で入れる）"

// Check は台帳の CLI が PATH にあるかを確かめる。無ければ Missing を返す。
func Check() error {
	if _, err := exec.LookPath(CLI); err != nil {
		return errors.New(Missing)
	}
	return nil
}

// DispatcherBy は dispatcher が台帳の CLI を打つときの log.jsonl の by（HC_TERMINAL_ID=dispatcher で打つ）。
// dispatcher が足したレビュアーのタスクは、Chief of Staff が足したものと同じに数える（docs/dispatcher.md §5 の P6a）。
const DispatcherBy = "terminal:dispatcher"

// Ledger は1つの台帳。
type Ledger struct {
	Root string // プロジェクトの根（kanban/ の親）
	Slug string
	Dir  string   // <Root>/kanban
	Env  []string // 台帳の CLI に足す環境（dispatcher は HC_TERMINAL_ID=dispatcher）
}

// At は台帳の根 root（kanban/ の親）の Ledger。
func At(root string) *Ledger {
	return &Ledger{Root: root, Slug: filepath.Base(root), Dir: filepath.Join(root, "kanban")}
}

// Open は台帳の CLI に台帳の根を聞く。
func Open() (*Ledger, error) {
	if err := Check(); err != nil {
		return nil, err
	}
	out, err := exec.Command(CLI, "root").Output()
	root := strings.TrimSpace(string(out))
	if err != nil || root == "" {
		return nil, errors.New("台帳が見つからない（projects/<slug>/ か担当の worktree で打つ）")
	}
	return At(root), nil
}

// Run は台帳の CLI をこの台帳に向けて打ち、stdout を返す（前後の空白を除く）。
func (l *Ledger) Run(stdin string, args ...string) (string, error) {
	c := exec.Command(CLI, args...)
	c.Env = append(append(os.Environ(), "KANBAN_ROOT="+l.Root), l.Env...)
	c.Stdin = strings.NewReader(stdin)
	var errb bytes.Buffer
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return "", fmt.Errorf("%s %s: %v %s", CLI, args[0], err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// Task は台帳のタスクの項目（`show <id> --json` の .task）。値は全部文字列（台帳の CLI がそう書く）。
type Task map[string]any

// Str はキーの値。文字列でなければ空。
func (t Task) Str(k string) string {
	s, _ := t[k].(string)
	return s
}

// Path はキーの値をパスとして読む。台帳の CLI は HOME の下を `~/…` で書くことがあるので、HOME に開く。
func (t Task) Path(k string) string {
	s := t.Str(k)
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, rest)
		}
	}
	return s
}

// Show はタスクの項目を読む。
func (l *Ledger) Show(id string) (Task, error) {
	out, err := l.Run("", "show", id, "--json")
	if err != nil {
		return nil, err
	}
	var v struct {
		Task Task `json:"task"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Task == nil {
		return nil, fmt.Errorf("タスク %s が読めない", id)
	}
	return v.Task, nil
}

// List は全部のタスク（done も）。
func (l *Ledger) List() ([]Task, error) {
	out, err := l.Run("", "list", "--all", "--json")
	if err != nil {
		return nil, err
	}
	var v []Task
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, errors.New("タスクの一覧が読めない")
	}
	return v, nil
}

// Library はタスクの library/（機械向けの置き場）。
func (l *Ledger) Library(id string) string {
	return filepath.Join(l.Dir, "tasks", id, "library")
}

// ChiefOfStaff は config.yaml の chief_of_staff（Chief of Staff の端末）。無ければ空。
func (l *Ledger) ChiefOfStaff() string {
	b, err := os.ReadFile(filepath.Join(l.Dir, "config.yaml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "chief_of_staff:"); ok {
			if i := strings.Index(v, " #"); i >= 0 {
				v = v[:i]
			}
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// Repos は config.yaml の repos（block sequence だけを読む。ほかの書き方は空）。
func (l *Ledger) Repos() []string {
	b, err := os.ReadFile(filepath.Join(l.Dir, "config.yaml"))
	if err != nil {
		return nil
	}
	var repos []string
	inside := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.TrimRight(line, " \t") == "repos:":
			inside = true
		case inside && strings.HasPrefix(strings.TrimLeft(line, " \t"), "- ") && line != strings.TrimLeft(line, " \t"):
			repos = append(repos, strings.Trim(strings.TrimSpace(strings.TrimLeft(line, " \t")[2:]), `"'`))
		case inside && line != "" && line == strings.TrimLeft(line, " \t"):
			inside = false
		}
	}
	return repos
}

// Row は log.jsonl の1行（台帳の CLI が書く）。
type Row struct {
	At     string         `json:"at"`
	By     string         `json:"by"`
	ID     string         `json:"id"`
	Change map[string]any `json:"change"`
}

// Log は log.jsonl を読む。読めない行は飛ばす（無いキーと同じ）。
func (l *Ledger) Log() []Row {
	f, err := os.Open(filepath.Join(l.Dir, "log.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var rows []Row
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var r Row
		if json.Unmarshal(s.Bytes(), &r) == nil {
			r.ID = CanonID(r.ID) // 移す前の行は A-5 と書いてある（kanban migrate は log.jsonl を書き換えない）
			rows = append(rows, r)
		}
	}
	return rows
}

// Trusted は、タスク id の key を最後に変えた log.jsonl の行の by が terminal:<chief_of_staff> で、
// その値がいまの task の値と同じか（contract.md §2 の履歴の照合）。chief_of_staff が無ければ信じない。
func (l *Ledger) Trusted(rows []Row, t Task, key string) bool {
	cos := l.ChiefOfStaff()
	if cos == "" {
		return false
	}
	var last *Row
	for i := range rows {
		if rows[i].ID != t.Str("id") || rows[i].Change == nil {
			continue
		}
		if _, ok := rows[i].Change[key]; ok {
			last = &rows[i]
		}
	}
	if last == nil || last.By != "terminal:"+cos {
		return false
	}
	v, ok := last.Change[key].(string)
	return ok && v == t.Str(key)
}

// Settings は task.yaml の Andon の予約キー（contract.md §2）。無い・知らない値は空（読み手が安全側に倒す）。
// 既定があるキー（reviewer・review_rounds・merge）は既定で埋める。
type Settings struct {
	Review       string `json:"review,omitempty"`
	MustFix      string `json:"must_fix,omitempty"`
	Reviewer     string `json:"reviewer"`
	ReviewRounds string `json:"review_rounds"`
	Merge        string `json:"merge"`
	Machine      string `json:"machine,omitempty"`
}

// Severities は重い順。
var Severities = []string{"critical", "major", "minor", "trivial"}

// SeverityRank は重さの順位（0 が一番重い）。知らなければ -1。
func SeverityRank(s string) int {
	for i, v := range Severities {
		if v == s {
			return i
		}
	}
	return -1
}

// Brand はプロファイルのブランド（codex* → codex、ほかは claude）。
func Brand(profile string) string {
	if strings.HasPrefix(profile, "codex") {
		return "codex"
	}
	return "claude"
}

// SettingsOf は task の予約キーを読む。
func SettingsOf(t Task) Settings {
	s := Settings{ReviewRounds: "2", Merge: "person"}
	if v := t.Str("review"); v == "on" || v == "off" {
		s.Review = v
	}
	if v := t.Str("must_fix"); SeverityRank(v) >= 0 {
		s.MustFix = v
	}
	switch v := t.Str("reviewer"); v {
	case "codex-review", "claude-review":
		s.Reviewer = v
	default:
		s.Reviewer = map[string]string{"claude": "codex-review", "codex": "claude-review"}[Brand(t.Str("profile"))]
	}
	if v := t.Str("review_rounds"); v != "" && v[0] != '0' && strings.Trim(v, "0123456789") == "" {
		s.ReviewRounds = v
	}
	if v := t.Str("merge"); v == "auto" || v == "person" {
		s.Merge = v
	}
	if v := t.Str("machine"); machineOK(v) {
		s.Machine = v
	}
	return s
}

func machineOK(v string) bool {
	if v == "" {
		return false
	}
	for i, r := range v {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || (r != '.' && r != '_' && r != '-')) {
			return false
		}
	}
	return true
}

// Reviewer は、タスク id のレビューに数えるレビュアーのタスクのうち最新のもの（無ければ空）と、
// 数えなかった子タスク（理由の文に使う）。数えるのは: parent が id・status が review か done・
// 作成の行（log.jsonl の最初の add）の by が terminal:<chief_of_staff> か dispatcher（DispatcherBy）・profile が実装と別ブランドの *-review。
func (l *Ledger) Reviewer(rows []Row, tasks []Task, t Task) (string, []string) {
	cos := l.ChiefOfStaff()
	creator := map[string]string{}
	for _, r := range rows {
		if _, ok := r.Change["add"]; ok {
			if _, seen := creator[r.ID]; !seen {
				creator[r.ID] = r.By
			}
		}
	}
	brand := Brand(t.Str("profile"))
	var picked []Task
	var others []string
	for _, c := range tasks {
		if c.Str("parent") != t.Str("id") || (c.Str("status") != "review" && c.Str("status") != "done") {
			continue
		}
		p := c.Str("profile")
		by := creator[c.Str("id")]
		if (by == DispatcherBy || cos != "" && by == "terminal:"+cos) && strings.HasSuffix(p, "-review") && Brand(p) != brand {
			picked = append(picked, c)
		} else {
			others = append(others, c.Str("id"))
		}
	}
	if len(picked) == 0 {
		return "", others
	}
	sort.SliceStable(picked, func(i, j int) bool {
		a, b := picked[i], picked[j]
		if a.Str("updated") != b.Str("updated") {
			return a.Str("updated") < b.Str("updated")
		}
		return idLess(a.Str("id"), b.Str("id"))
	})
	return picked[len(picked)-1].Str("id"), others
}

// CanonID は人が打った id を台帳の形に揃える（A-5・A-05 → A-0005。docs/ledger.md の Ids）。
// <prefix>-<数字> でなければそのまま。台帳の CLI（kanban）の canonID と同じ。
func CanonID(id string) string {
	i := strings.LastIndex(id, "-")
	if i <= 0 || i == len(id)-1 || strings.Trim(id[i+1:], "0123456789") != "" {
		return id
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil || n < 1 {
		return id
	}
	return fmt.Sprintf("%s-%04d", id[:i], n)
}

// idLess は台帳の id（<prefix>-<n>）を番号の順に比べる。
func idLess(a, b string) bool {
	pa, na, _ := strings.Cut(a, "-")
	pb, nb, _ := strings.Cut(b, "-")
	if pa != pb || len(na) == len(nb) {
		return a < b
	}
	return len(na) < len(nb)
}
