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
	"regexp"
	"slices"
	"sort"
	"strings"
)

const CLI = "kanban"

const Missing = "kanban が無い（kanban の CLI。Andon の image に入っている。手元なら go install ./cmd/kanban で入れる）"

func Check() error {
	if _, err := exec.LookPath(CLI); err != nil {
		return errors.New(Missing)
	}
	return nil
}

// DispatcherBy は dispatcher が kanban の CLI を打つときの log.jsonl の by（HC_TERMINAL_ID=dispatcher で打つ）。
// dispatcher が足したレビュアーの課題は、Chief of Staff が足したものと同じに数える（docs/dispatcher.md §5 の P6a）。
const DispatcherBy = "terminal:dispatcher"

type Ledger struct {
	Root string // プロジェクトの根（kanban/ の親）
	Slug string
	Dir  string
	Env  []string
}

func At(root string) *Ledger {
	return &Ledger{Root: root, Slug: filepath.Base(root), Dir: filepath.Join(root, "kanban")}
}

func Open() (*Ledger, error) {
	if err := Check(); err != nil {
		return nil, err
	}
	out, err := exec.Command(CLI, "root").Output()
	root := strings.TrimSpace(string(out))
	if err != nil || root == "" {
		return nil, errors.New("kanban が見つからない（--project <slug> で指す）")
	}
	return At(root), nil
}

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

type Issue map[string]any

func (t Issue) Str(k string) string {
	s, _ := t[k].(string)
	return s
}

func (t Issue) Path(k string) string {
	s := t.Str(k)
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, rest)
		}
	}
	return s
}

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

type Row struct {
	At      string         `json:"at"`
	By      string         `json:"by"`
	ID      string         `json:"id"`
	Change  map[string]any `json:"change"`
	Config  map[string]any `json:"config"`
	Routine string         `json:"routine"`
	Fired   string         `json:"fired"`
}

// Log は log.jsonl を読む。読めない行は飛ばす（無いキーと同じ）。ファイルが読めなければ読めたところまで（無ければ nil）。
func (l *Ledger) Log() []Row {
	rows, _ := l.ReadLog()
	return rows
}

// ReadLog は Log と同じく log.jsonl を読み、ファイルを開けない・読み切れないときは読めたところまでと err を返す
// （空の kanban と、読めなかった kanban を分けたい呼び手のため）。
func (l *Ledger) ReadLog() ([]Row, error) {
	rows, _, err := l.readLog()
	return rows, err
}

// ErrBadRow は、log.jsonl に JSON として読めない行があったこと（ReadWholeLog）。
var ErrBadRow = errors.New("log.jsonl に読めない行がある")

// ReadWholeLog は ReadLog と同じ行を返し、JSON として読めない行（空行は除く）があれば ErrBadRow も返す。行を飛ばした履歴で
// 書き手の照合を言い切りたくない呼び手のため（Chief of Staff の交代の行が抜けると、交代のあとの前の端末を Chief of Staff と読む）。
func (l *Ledger) ReadWholeLog() ([]Row, error) {
	rows, bad, err := l.readLog()
	if err == nil && bad > 0 {
		err = fmt.Errorf("%w（%d 行）", ErrBadRow, bad)
	}
	return rows, err
}

func (l *Ledger) readLog() (rows []Row, bad int, err error) {
	f, err := os.Open(filepath.Join(l.Dir, "log.jsonl"))
	if err != nil {
		return nil, 0, err
	}
	defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64<<10), 16<<20)
	for s.Scan() {
		var r Row
		if json.Unmarshal(s.Bytes(), &r) == nil {
			r.ID = CanonID(r.ID)
			rows = append(rows, r)
		} else if len(bytes.TrimSpace(s.Bytes())) > 0 {
			bad++
		}
	}
	return rows, bad, s.Err()
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
					cos, _ = v.(string)
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

// TrustedThen は、課題 t の key を最後に変えた log.jsonl の行の by が、その行の時点の Chief of Staff の端末で、
// その値がいまの課題の値と同じか（純粋関数。ADR-0062 決めたこと 3）。その行の時点の Chief of Staff は、履歴の
// kanban config set の行（chief_of_staff・前の名前の coordinator）を順に辿って決める（TakenByChiefOfStaff と同じ辿り方）。
// Chief of Staff の行が 1 つも無い kanban では、今の config.yaml の値 cos と比べる。
// 印が何日も残る予約キー（later・later_until）のための照合で、Chief of Staff が起き直して端末が替わっても印を信じ続ける。
// 起き直したら打ち直す決まりの予約キー（merge など）は、今の端末と比べる Trusted で読む。deliverable の report はこの照合で読む（DeliverableOf）。
func TrustedThen(rows []Row, t Issue, key, cos string) bool {
	last, byCoS := LastChangeThen(rows, t, key, cos)
	if last == nil || !byCoS {
		return false
	}
	v, ok := last.Change[key].(string)
	return ok && v != "" && v == t.Str(key)
}

// LastChangeThen は、課題 t の key を最後に変えた log.jsonl の行（消した行も含む。無ければ nil）と、その行の by が
// その行の時点の Chief of Staff の端末か（TrustedThen と同じ辿り方。純粋関数）。消した行の書き手を見たい読み手
// （担当が later_until を消したか）のため。行より前に Chief of Staff の行が無く、あとの行で初めて出る kanban では、
// 行の時点の Chief of Staff が分からないので false。
func LastChangeThen(rows []Row, t Issue, key, cos string) (*Row, bool) {
	id := CanonID(t.Str("id"))
	then, seen := "", false
	var last *Row
	lastCoS, lastSeen := "", false
	for i, r := range rows {
		if r.Config != nil {
			for _, k := range []string{"chief_of_staff", "coordinator"} { // 2026-11-06 より前の行は coordinator（docs/ledger.md）
				if v, ok := r.Config[k]; ok {
					then, _ = v.(string)
					seen = true
				}
			}
			continue
		}
		if r.ID != id || r.Change == nil {
			continue
		}
		if _, ok := r.Change[key]; ok {
			last, lastCoS, lastSeen = &rows[i], then, seen
		}
	}
	if last == nil {
		return nil, false
	}
	if !lastSeen {
		if seen {
			return last, false
		}
		lastCoS = cos
	}
	return last, lastCoS != "" && last.By == "terminal:"+lastCoS
}

// HoldOf は課題 t の止め（予約キー hold。Chief of Staff が PR を止めた理由の課題の id。ADR-0056 決めたこと 7・10）を読む
// （純粋関数）。返すのは止めた理由の課題の id と、止めているか。
// hold が空でなければ、誰が書いても止めている（足すほうは誰が書いても止まる）。
// hold が空でも、hold を最後に変えた log.jsonl の行の書き手が、その課題かその子の課題（tasks のうち parent がその課題のもの）の
// 担当の端末（その課題の agent に書かれたことのある端末。Chief of Staff の端末 cos を除く）なら、止めているものとして読む
// （担当が消しても止めは外れない）。そのときの理由は、消す前に最後に書かれていた値。消してよいのは Chief of Staff とオーナーである。
func HoldOf(rows []Row, tasks []Issue, cos string, t Issue) (string, bool) {
	id := t.Str("id")
	if v := t.Str("hold"); v != "" {
		return v, true
	}
	last, prev, cur := -1, "", ""
	for i, r := range rows {
		if r.ID != id || r.Change == nil {
			continue
		}
		if v, ok := r.Change["hold"]; ok {
			last, prev = i, cur
			cur, _ = v.(string)
		}
	}
	if last < 0 || prev == "" {
		return "", false
	}
	by, ok := strings.CutPrefix(rows[last].By, "terminal:")
	if !ok || by == "" || by == cos {
		return "", false
	}
	mine := map[string]bool{id: true}
	for _, o := range tasks {
		if CanonID(o.Str("parent")) == id {
			mine[CanonID(o.Str("id"))] = true
		}
	}
	agents := map[string]bool{}
	for _, o := range tasks {
		if mine[CanonID(o.Str("id"))] && o.Str("agent") != "" {
			agents[o.Str("agent")] = true
		}
	}
	for _, r := range rows {
		if !mine[r.ID] || r.Change == nil {
			continue
		}
		if s, _ := r.Change["agent"].(string); s != "" {
			agents[s] = true
		}
	}
	if agents[by] {
		return prev, true
	}
	return "", false
}

// LastHoldSet は、log.jsonl の after 行目（1 から数える）より後で、課題 id の hold に値を書いた行のうち最後のものの値と時刻
// （純粋関数）。止めてから外すまでの間に dispatcher が一度も見なかったとき、止める前の gate の判定を捨てるために読む
// （ADR-0056 決めたこと 9）。無ければ ok が false。
func LastHoldSet(rows []Row, id string, after int) (reason, at string, ok bool) {
	for i, r := range rows {
		if i < after || r.ID != id || r.Change == nil {
			continue
		}
		if v, _ := r.Change["hold"].(string); v != "" {
			reason, at, ok = v, r.At, true
		}
	}
	return reason, at, ok
}

// Hold は HoldOf を、kanban の今の Chief of Staff で読む。rows と logErr は呼び手が ReadLog で読んだ log.jsonl と、その誤り。
// 子の課題の一覧は、hold が空で、hold を消した行があるときだけ読む（止めていない課題で kanban の一覧を読まない）。
// hold が空で log.jsonl を読み切れない（無いのは除く）か、一覧を読めなければ誤りを返す（呼び手は止めているものとして扱う。
// 担当が消した止めを、読めないまま外れたと読まない）。
func (l *Ledger) Hold(rows []Row, logErr error, t Issue) (string, bool, error) {
	if v := t.Str("hold"); v != "" {
		return v, true, nil
	}
	if logErr != nil && !errors.Is(logErr, os.ErrNotExist) {
		return "", false, fmt.Errorf("log.jsonl が読めない: %w", logErr)
	}
	touched := false
	for _, r := range rows {
		if _, ok := r.Change["hold"]; ok && r.ID == t.Str("id") {
			touched = true
		}
	}
	if !touched {
		return "", false, nil
	}
	tasks, err := l.List()
	if err != nil {
		return "", false, err
	}
	reason, held := HoldOf(rows, tasks, l.ChiefOfStaff(), t)
	return reason, held, nil
}

// Candidates は、課題の成果が候補の一覧（予約キー deliverable が candidates）で、それを最後に書いたのが
// Chief of Staff か（contract.md §2 の履歴の照合）。無い・知らない値・照合が合わないときは false。
// 読み手は andon gate（終了コード 3 の理由の文を分ける）。dispatcher と andon issue ready は照合を見ずに
// SettingsOf の Deliverable で読む（Chief of Staff の起き直しのあと書き直す前でも PR を求めない。ADR-0006 決めたこと 7・17）。
func (l *Ledger) Candidates(rows []Row, t Issue) bool {
	return t.Str("deliverable") == "candidates" && l.Trusted(rows, t, "deliverable")
}

// DeliverableOf は課題 t の成果の形（予約キー deliverable の pr・candidates・report。contract.md §2 の「成果の形の読み方」）を、
// kanban の履歴 rows と今の config.yaml の Chief of Staff cos で照合を済ませて返す（純粋関数。ADR-0066 決めたこと 10〜12）。
// candidates は値だけで読む（ADR-0006 決めたこと 17）。report は、最後に変えた行の書き手がその行の時点の Chief of Staff の
// 端末だったときだけ信じ（TrustedThen。Chief of Staff が起き直しても外れない）、外れたら pr。値の無い課題と知らない値は pr。
// SettingsOf の Deliverable は値だけを読む写し（依頼書の Settings の写しと andon issue show が読む）で、照合を済ませていない。
func DeliverableOf(rows []Row, t Issue, cos string) string {
	v := SettingsOf(t).Deliverable
	if v == "report" && !TrustedThen(rows, t, "deliverable", cos) {
		return "pr"
	}
	return v
}

// Deliverable は課題 t の成果の形を、kanban の履歴を読んで DeliverableOf で返す（PR の無い報告の課題を分ける andon issue merge・
// andon gate・andon issue ready の読み手。ADR-0066 決めたこと 7・8・10）。値が report なのに pr と読んだときは、why にその理由の
// 1 句を返す（読み手が理由の文に添える）。履歴を読み切れない・読めない行があるときは、report を書いたのが Chief of Staff か
// 確かめられない（交代の行が抜けると前の端末を Chief of Staff と読む）ので pr と読む。pr と読むのは今までの動きに戻るだけである。
func (l *Ledger) Deliverable(t Issue) (form, why string) {
	if t.Str("deliverable") != "report" {
		return DeliverableOf(nil, t, ""), ""
	}
	rows, err := l.ReadWholeLog()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "pr", "kanban の履歴を読み切れず、deliverable: report を書いたのが Chief of Staff か確かめられない"
	}
	if form = DeliverableOf(rows, t, l.ChiefOfStaff()); form != "report" {
		return form, "deliverable: report を書いたのが Chief of Staff でない"
	}
	return form, ""
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

var fenceRe = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")

// settingNames は予約キーの、依頼書の ## Settings の行の名前（playbooks/brief.md の表）。settingOrder は写しに足すときの順。
var (
	settingNames = map[string]string{"review": "Review", "reviewer": "Reviewer", "must_fix": "Must fix", "review_rounds": "Review rounds",
		"merge": "Merge", "deliverable": "Deliverable", "acceptance": "Acceptance", "estimate": "Estimate"}
	settingOrder = []string{"review", "reviewer", "must_fix", "review_rounds", "merge", "deliverable", "acceptance", "estimate"}
	settingSizes = map[string]string{"small": "小", "medium": "中", "large": "大"}
)

func copyValue(k, v string) string {
	if k == "estimate" {
		return settingSizes[v]
	}
	return v
}

// SettingsCopy は予約キー（DefaultSettings の key=value の並び）を、依頼書の ## Settings の写しにする
// （行の名前と見込みの段の名は playbooks/brief.md の表）。正は issue.yaml で、これは人が読む写し。
func SettingsCopy(keys []string) string {
	var b strings.Builder
	b.WriteString("## Settings\n")
	for _, kv := range keys {
		k, v, _ := strings.Cut(kv, "=")
		b.WriteString("- " + settingNames[k] + ": " + copyValue(k, v) + "\n")
	}
	return b.String()
}

// RewriteSettings は依頼書 brief の ## Settings の写しを、課題 t の予約キー（正）に合わせて書き直す。書き換えたかを返す。
// 写しは見出しの直後に続く「- 名前: 値」の行の並びで、その後ろ（空行・地の文・追記の節）は触らない。見出しが無ければ、
// 依頼書の末尾に空行を挟んで足す（予約キーが1つも無ければ足さない。ADR-0053 決めたこと 19）。
// 見出しが2つ以上あれば（追記の節に写しを書き直した依頼書）、どれも書き直す。コード例（``` か ~~~ の囲み）の中の見出しは写しでないので触らない。
// 行の値は SettingsOf が読む値（無いキーと知らない値は既定か空。gate・dispatcher が読む値と同じ）で、値の頭が同じ行は
// そのまま残す（「Deliverable: candidates ★調べる」）。値の無い行は消し、行の無いキーで issue.yaml にあるものは並びの末尾に足す。
// 知らない名前の行は残す。書く行のうち既定（playbooks/brief.md の表）から変えた行には行末に ★ を付け、reasons にそのキーの理由が
// あれば ★ に続けて添える（理由のあるキーは、値の頭が同じ行も理由つきに書き直す）。見込み（Estimate）と受け入れ条件（Acceptance）には
// 既定が無いので印を付けない。課題があとから kanban set で予約キーを書き直しても、担当に渡る写しが正と食い違わないように、
// andon issue create・start・resume が打つ。
func RewriteSettings(brief string, t Issue, reasons map[string]string) (string, bool) {
	lines := strings.SplitAfter(brief, "\n")
	var out strings.Builder
	fence, found := "", false
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		trim := strings.TrimRight(ln, " \t\r\n")
		if m := fenceRe.FindStringSubmatch(trim); m != nil {
			switch {
			case fence == "":
				fence = m[1]
			case m[2] == "" && m[1][0] == fence[0] && len(m[1]) >= len(fence):
				fence = "" // 閉じるのは開いたのと同じ文字で同じ数以上の並びだけの行（CommonMark）
			}
		}
		if fence != "" || trim != "## Settings" {
			out.WriteString(ln)
			continue
		}
		found = true
		end := i + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "- ") {
			end++
		}
		eol := "\n"
		if strings.HasSuffix(ln, "\r\n") {
			eol = "\r\n"
		}
		block := settingsBlock(lines[i+1:end], t, eol, reasons)
		if len(block) > 0 && !strings.HasSuffix(ln, "\n") {
			ln += eol
		}
		out.WriteString(ln + strings.Join(block, ""))
		i = end - 1
	}
	if !found {
		eol := "\n"
		if strings.Contains(brief, "\r\n") {
			eol = "\r\n"
		}
		if block := settingsBlock(nil, t, eol, reasons); len(block) > 0 {
			switch {
			case brief == "" || strings.HasSuffix(brief, eol+eol):
			case strings.HasSuffix(brief, eol):
				out.WriteString(eol)
			default:
				out.WriteString(eol + eol)
			}
			out.WriteString("## Settings" + eol + strings.Join(block, ""))
		}
	}
	return out.String(), out.String() != brief
}

// settingDefault は印を付けるかを決める既定（playbooks/brief.md の表の既定の欄）。reviewer は実装と別ブランド。
// 見込みと受け入れ条件は既定が無いので空（印を付けない）。
func settingDefault(t Issue, k string) string {
	switch k {
	case "review":
		return "off"
	case "must_fix":
		return "major"
	case "reviewer":
		return SettingsOf(Issue{"profile": t.Str("profile")}).Reviewer
	case "review_rounds":
		return "2"
	case "merge":
		return "person"
	case "deliverable":
		return "pr"
	}
	return ""
}

// Marked は、予約キー k の値 v が写しで印 ★ の付く値（既定から変えた値）か。
func Marked(t Issue, k, v string) bool {
	d := settingDefault(t, k)
	return d != "" && v != d
}

func settingsBlock(old []string, t Issue, eol string, reasons map[string]string) []string {
	s := SettingsOf(t)
	vals := map[string]string{"review": s.Review, "must_fix": s.MustFix, "reviewer": s.Reviewer, "review_rounds": s.ReviewRounds,
		"merge": s.Merge, "deliverable": s.Deliverable, "acceptance": s.Acceptance, "estimate": copyValue("estimate", s.Estimate)}
	value := func(k string) string { return vals[k] }
	row := func(k, v string) string {
		ln := "- " + settingNames[k] + ": " + v
		if Marked(t, k, v) {
			ln += " ★" + reasons[k]
		}
		return ln + eol
	}
	byName := map[string]string{}
	for k, n := range settingNames {
		byName[strings.ToLower(n)] = k
	}
	var block []string
	add := func(ln string) {
		if n := len(block); n > 0 && !strings.HasSuffix(block[n-1], "\n") {
			block[n-1] += eol
		}
		block = append(block, ln)
	}
	seen := map[string]bool{}
	for _, ln := range old {
		name, cur, ok := strings.Cut(strings.TrimRight(strings.TrimPrefix(ln, "- "), "\r\n"), ":")
		k, known := byName[strings.ToLower(strings.TrimSpace(name))]
		switch {
		case !ok || !known:
			add(ln)
			continue
		case seen[k]:
			continue
		}
		seen[k] = true
		v := value(k)
		switch f := strings.Fields(cur); {
		case v == "":
		case len(f) > 0 && f[0] == v && (reasons[k] == "" || !Marked(t, k, v)):
			add(ln)
		default:
			add(row(k, v))
		}
	}
	for _, k := range settingOrder {
		if v := value(k); !seen[k] && t.Str(k) != "" && v != "" {
			add(row(k, v))
		}
	}
	return block
}

func Brand(profile string) string {
	if strings.HasPrefix(profile, "codex") {
		return "codex"
	}
	return "claude"
}

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
	if v := t.Str("deliverable"); v == "candidates" || v == "report" {
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

// AcceptanceOf は予約キー acceptance（課題が満たす受け入れ条件の番号をコンマで並べた文字列。4,13）を並びにする
// （ADR-0035 決めたこと 3）。キーが無ければ nil と true（番号を持たない課題）。番号は書いた綴りのまま返し、
// 同じ番号（AcceptanceKey が同じもの。4 と A4）の重なりは先のものだけを残す。
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
		if !slices.ContainsFunc(ids, func(x string) bool { return AcceptanceKey(x) == AcceptanceKey(id) }) {
			ids = append(ids, id)
		}
	}
	return ids, true
}

// AcceptanceKey は受け入れ条件の番号を比べるときの形を返す。A の後に数字だけが続く番号（A4）は A を外した形（4）にし、
// ほかはそのまま返す。Chief Engineer の文書は受け入れ条件を数字で振り（4）、担当は A を付けて書くことがある（A4）ので、
// 予約キー acceptance と報告の acceptance[].id はどちらの形でも同じ番号と読む。数字の綴りは変えない（A01 と A1 は別）。
func AcceptanceKey(id string) string {
	if n, ok := strings.CutPrefix(id, "A"); ok && n != "" && strings.Trim(n, "0123456789") == "" {
		return n
	}
	return id
}

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
