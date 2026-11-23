// Package ledger は kanban を読む（docs/contract.md §1・§2・§3）。kanban を書くのは kanban の CLI だけで、
// ここはそれを打つか、kanban のファイルを読むだけ。Markdown（依頼書・報告）は読まない。
//
// command（andon issue・andon gate）が共有するもの: 課題の項目（issue.yaml の Andon の予約キー）と履歴の照合・
// レビュアーの課題の選び方・findings.json の読み方・library/ の置き場。
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
	"slices"
	"sort"
	"strings"
)

// CLI は kanban の CLI の名前。
const CLI = "kanban"

// Missing は kanban の CLI が PATH に無いときの言い方（入れ方の案内つき）。
const Missing = "kanban が無い（kanban の CLI。Andon の image に入っている。手元なら go install ./cmd/kanban で入れる）"

// Check は kanban の CLI が PATH にあるかを確かめる。無ければ Missing を返す。
func Check() error {
	if _, err := exec.LookPath(CLI); err != nil {
		return errors.New(Missing)
	}
	return nil
}

// DispatcherBy は dispatcher が kanban の CLI を打つときの log.jsonl の by（HC_TERMINAL_ID=dispatcher で打つ）。
// dispatcher が足したレビュアーの課題は、Chief of Staff が足したものと同じに数える（docs/dispatcher.md §5 の P6a）。
const DispatcherBy = "terminal:dispatcher"

// Ledger は1つの kanban。
type Ledger struct {
	Root string // プロジェクトの根（kanban/ の親）
	Slug string
	Dir  string   // <Root>/kanban
	Env  []string // kanban の CLI に足す環境（dispatcher は HC_TERMINAL_ID=dispatcher）
}

// At は kanban の根 root（kanban/ の親）の Ledger。
func At(root string) *Ledger {
	return &Ledger{Root: root, Slug: filepath.Base(root), Dir: filepath.Join(root, "kanban")}
}

// Open は kanban の CLI に kanban の根を聞く。
func Open() (*Ledger, error) {
	if err := Check(); err != nil {
		return nil, err
	}
	out, err := exec.Command(CLI, "root").Output()
	root := strings.TrimSpace(string(out))
	if err != nil || root == "" {
		return nil, errors.New("kanban が見つからない（projects/<slug>/ か担当の worktree で打つ）")
	}
	return At(root), nil
}

// Run は kanban の CLI をこの kanban に向けて打ち、stdout を返す（前後の空白を除く）。
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

// Issue は kanban の課題の項目（`show <id> --json` の .issue）。値は全部文字列（kanban の CLI がそう書く）。
type Issue map[string]any

// Str はキーの値。文字列でなければ空。
func (t Issue) Str(k string) string {
	s, _ := t[k].(string)
	return s
}

// Path はキーの値をパスとして読む。kanban の CLI は HOME の下を `~/…` で書くことがあるので、HOME に開く。
func (t Issue) Path(k string) string {
	s := t.Str(k)
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, rest)
		}
	}
	return s
}

// Show は課題の項目を読む。
func (l *Ledger) Show(id string) (Issue, error) {
	out, err := l.Run("", "show", id, "--json")
	if err != nil {
		return nil, err
	}
	var v struct {
		Issue Issue `json:"issue"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Issue == nil {
		return nil, fmt.Errorf("課題 %s が読めない", id)
	}
	return v.Issue, nil
}

// List は全部の課題（done も）。
func (l *Ledger) List() ([]Issue, error) {
	out, err := l.Run("", "list", "--all", "--json")
	if err != nil {
		return nil, err
	}
	var v []Issue
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, errors.New("課題の一覧が読めない")
	}
	return v, nil
}

// Library は課題の library/（機械向けの置き場）。
func (l *Ledger) Library(id string) string {
	return filepath.Join(l.Dir, "issues", id, "library")
}

// ChiefOfStaff は config.yaml の chief_of_staff（Chief of Staff の端末）。無い・読めなければ空。
func (l *Ledger) ChiefOfStaff() string {
	c, _ := l.Config()
	return c.ChiefOfStaff
}

// Timezone は config.yaml の timezone（ルーチンの schedule と not_before を読む IANA の名前）。無い・読めなければ空（UTC）。
func (l *Ledger) Timezone() string {
	c, _ := l.Config()
	return c.Timezone
}

// Repos は config.yaml の repos。無い・読めなければ空。
func (l *Ledger) Repos() []string {
	c, _ := l.Config()
	return c.Repos
}

// Row は log.jsonl の1行（kanban の CLI が書く）。
type Row struct {
	At     string         `json:"at"`
	By     string         `json:"by"`
	ID     string         `json:"id"`
	Change map[string]any `json:"change"`
	Config map[string]any `json:"config"` // kanban config set の行（id と change は無い）
	// ルーチンが課題を足した行（by は routine:<名前>。id と change は無い）
	Routine string `json:"routine"`
	Fired   string `json:"fired"`
}

// Log は log.jsonl を読む。読めない行は飛ばす（無いキーと同じ）。ファイルが読めなければ読めたところまで（無ければ nil）。
func (l *Ledger) Log() []Row {
	rows, _ := l.ReadLog()
	return rows
}

// ReadLog は Log と同じく log.jsonl を読み、ファイルを開けない・読み切れないときは読めたところまでと err を返す
// （空の kanban と、読めなかった kanban を分けたい呼び手のため）。
func (l *Ledger) ReadLog() ([]Row, error) {
	f, err := os.Open(filepath.Join(l.Dir, "log.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	var rows []Row
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64<<10), 16<<20) // kanban と同じ1行の上限（cmd/kanban の loggedNumber）
	for s.Scan() {
		var r Row
		if json.Unmarshal(s.Bytes(), &r) == nil {
			r.ID = CanonID(r.ID) // 移す前の行は A-5 と書いてある（kanban migrate は log.jsonl を書き換えない）
			rows = append(rows, r)
		}
	}
	return rows, s.Err()
}

// TakenByChiefOfStaff は、課題 id の agent を agent に書いた最後の行が、その時点の Chief of Staff の端末だったときに書かれたか
// （log.jsonl の kanban config set の行で chief_of_staff の移り変わりを辿る。前の名前の coordinator も読む）。Chief of Staff が自分で kanban start した課題は、
// 起き直してアドレスが変わったあとも担当の課題でない。同じアドレスが後で担当に使われても、その時点の値で見分ける。
// 行が無い・config の行が無い kanban では false（今の chief_of_staff との比べは呼び手がする）。
func TakenByChiefOfStaff(rows []Row, id, agent string) bool {
	if agent == "" {
		return false
	}
	cos, taken := "", false
	for _, r := range rows {
		if r.Config != nil {
			for _, k := range []string{"chief_of_staff", "coordinator"} { // 2026-11-06 より前の行は coordinator（docs/ledger.md）
				if v, ok := r.Config[k]; ok {
					cos, _ = v.(string) // null は消した（空）
				}
			}
			continue
		}
		if r.ID != id || r.Change == nil {
			continue
		}
		if v, ok := r.Change["agent"]; ok {
			s, _ := v.(string)
			taken = s == agent && cos == agent
		}
	}
	return taken
}

// Trusted は、課題 id の key を最後に変えた log.jsonl の行の by が terminal:<chief_of_staff> で、
// その値がいまの課題の値と同じか（contract.md §2 の履歴の照合）。chief_of_staff が無ければ信じない。
func (l *Ledger) Trusted(rows []Row, t Issue, key string) bool {
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

// Candidates は、課題の成果が候補の一覧（予約キー deliverable が candidates）で、それを最後に書いたのが
// Chief of Staff か（contract.md §2 の履歴の照合）。無い・知らない値・照合が合わないときは false。
// 読み手は andon gate（終了コード 3 の理由の文を分ける）。dispatcher と andon issue ready は照合を見ずに
// SettingsOf の Deliverable で読む（Chief of Staff の起き直しのあと書き直す前でも PR を求めない。ADR-0006 決めたこと 7・17）。
func (l *Ledger) Candidates(rows []Row, t Issue) bool {
	return t.Str("deliverable") == "candidates" && l.Trusted(rows, t, "deliverable")
}

// Estimate は、課題の見込みの段（予約キー estimate の small・medium・large）を、最後に書いたのが Chief of Staff のときだけ返す
// （contract.md §2 の履歴の照合）。無い・知らない値・照合が合わないときは空（見込み無し）。ADR-0014 決めたこと 2・11 の 5。
func (l *Ledger) Estimate(rows []Row, t Issue) string {
	if v := SettingsOf(t).Estimate; v != "" && l.Trusted(rows, t, "estimate") {
		return v
	}
	return ""
}

// Settings は issue.yaml の Andon の予約キー（contract.md §2）。無い・知らない値は空（読み手が安全側に倒す）。
// 既定があるキー（reviewer・review_rounds・merge）は既定で埋める。
type Settings struct {
	Review       string `json:"review,omitempty"`
	MustFix      string `json:"must_fix,omitempty"`
	Reviewer     string `json:"reviewer"`
	ReviewRounds string `json:"review_rounds"`
	Merge        string `json:"merge"`
	Machine      string `json:"machine,omitempty"`
	Deliverable  string `json:"deliverable"`
	Estimate     string `json:"estimate,omitempty"`
	Acceptance   string `json:"acceptance,omitempty"`
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

// FiredBy は、課題 t を足したのがルーチンで、それが課題の routine の欄と同じルーチンなら、その名前を返す
// （log.jsonl の by が routine:<名前> で fired が課題の id の行。kanban の CLI がルーチンを撃つときだけ書く）。
// 欄だけがあって行が無い（kanban set で routine を書いた）課題は空。
func FiredBy(rows []Row, t Issue) string {
	name, id := t.Str("routine"), CanonID(t.Str("id"))
	if name == "" {
		return ""
	}
	for _, r := range rows {
		if r.By == "routine:"+name && r.Routine == name && CanonID(r.Fired) == id {
			return name
		}
	}
	return ""
}

// DefaultSettings は、command が足す担当の課題に書く予約キーの既定を、kanban set に渡す key=value の並びで返す
// （トリアージと振り返りの apply が足した課題・ルーチンが足した課題。contract.md §2）。値は Chief of Staff の
// playbooks/brief.md の表の既定（review は呼び手が決める）で、estimate は中。
// reviewer と review_rounds は review が on のときだけ書き、reviewer は実装と別ブランドにする。off のときは書かない:
// 書くと足した時点の profile で固定され、あとで Chief of Staff が profile を替えたとき同じブランドのレビュアーが起動される
// （書かなければ SettingsOf が読むときの profile から選ぶ。andon issue start も off なら求めない）。
func DefaultSettings(profile, review string) []string {
	if review != "on" {
		return []string{"review=" + review, "must_fix=major", "merge=person", "estimate=medium"}
	}
	reviewer := "codex-review"
	if Brand(profile) == "codex" {
		reviewer = "claude-review"
	}
	return []string{"review=on", "must_fix=major", "reviewer=" + reviewer, "review_rounds=2", "merge=person", "estimate=medium"}
}

// Unsettled は、既定（DefaultSettings の key=value の並び）のうち、課題 t に書き直すものを返す: キーが無いか、
// 既定と同じ値なのに履歴の照合（Trusted）が外れている（kanban set が issue.yaml を書いたあと log.jsonl に足す前に止まった・
// Chief of Staff が起き直してアドレスが変わった）。既定と違う値は誰かが選んだものなので、照合が外れていても返さない。
func (l *Ledger) Unsettled(rows []Row, t Issue, defaults []string) []string {
	var out []string
	for _, kv := range defaults {
		k, v, _ := strings.Cut(kv, "=")
		if cur := t.Str(k); cur == "" || (cur == v && !l.Trusted(rows, t, k)) {
			out = append(out, kv)
		}
	}
	return out
}

// SettingsCopy は予約キー（DefaultSettings の key=value の並び）を、依頼書の ## Settings の写しにする
// （行の名前と見込みの段の名は playbooks/brief.md の表）。正は issue.yaml で、これは人が読む写し。
func SettingsCopy(keys []string) string {
	names := map[string]string{"review": "Review", "reviewer": "Reviewer", "must_fix": "Must fix", "review_rounds": "Review rounds", "merge": "Merge", "estimate": "Estimate"}
	sizes := map[string]string{"small": "小", "medium": "中", "large": "大"}
	var b strings.Builder
	b.WriteString("## Settings\n")
	for _, kv := range keys {
		k, v, _ := strings.Cut(kv, "=")
		if k == "estimate" {
			v = sizes[v]
		}
		b.WriteString("- " + names[k] + ": " + v + "\n")
	}
	return b.String()
}

// Brand はプロファイルのブランド（codex* → codex、ほかは claude）。
func Brand(profile string) string {
	if strings.HasPrefix(profile, "codex") {
		return "codex"
	}
	return "claude"
}

// SettingsOf は課題の予約キーを読む。
func SettingsOf(t Issue) Settings {
	s := Settings{ReviewRounds: "2", Merge: "person", Deliverable: "pr"}
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
	if v := t.Str("machine"); nameOK(v) {
		s.Machine = v
	}
	if v := t.Str("deliverable"); v == "candidates" {
		s.Deliverable = v
	}
	if v := t.Str("estimate"); EstimateMinutes(v) > 0 {
		s.Estimate = v
	}
	if ids, ok := AcceptanceOf(t); ok && len(ids) > 0 {
		s.Acceptance = t.Str("acceptance")
	}
	return s
}

// AcceptanceOf は予約キー acceptance（課題が満たす受け入れ条件の番号を、報告の acceptance[].id と同じ綴りでコンマで並べた文字列。
// A4,A13）を並びにする（ADR-0035 決めたこと 3）。キーが無ければ nil と true（番号を持たない課題）。
// 文字列でない・空の番号・知らない文字があれば nil と false（読み手の andon issue ready は黙って通さず断る）。
// 履歴の照合（Trusted）は見ない: 担当が書き換えても求める行が増えるか、キーを消して今までどおりになるだけで、
// 照合を求めると Chief of Staff が起き直したあと書き直す前の課題で番号が消える。
func AcceptanceOf(t Issue) ([]string, bool) {
	raw, present := t["acceptance"]
	if !present {
		return nil, true
	}
	v, ok := raw.(string)
	if !ok || v == "" {
		return nil, false
	}
	var ids []string
	for _, id := range strings.Split(v, ",") {
		if !nameOK(id) {
			return nil, false
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// nameOK は英数字で始まり、英数字と . _ - だけの名前か（machine の名前と受け入れ条件の番号）。
func nameOK(v string) bool {
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

// reviewers は、課題 t のレビューに数えるレビュアーの課題を古い順（updated・id の順）に全部と、
// 数えなかった子課題（理由の文に使う）を返す。数えるのは: parent が t の id・status が review か done・
// 作成の行（log.jsonl の最初の add）の by が terminal:<chief_of_staff> か dispatcher（DispatcherBy）・profile が実装と別ブランドの *-review。
func (l *Ledger) reviewers(rows []Row, tasks []Issue, t Issue) ([]string, []string) {
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
	var picked []Issue
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
	sort.SliceStable(picked, func(i, j int) bool {
		a, b := picked[i], picked[j]
		if a.Str("updated") != b.Str("updated") {
			return a.Str("updated") < b.Str("updated")
		}
		return IDLess(a.Str("id"), b.Str("id"))
	})
	ids := make([]string, len(picked))
	for i, c := range picked {
		ids[i] = c.Str("id")
	}
	return ids, others
}
