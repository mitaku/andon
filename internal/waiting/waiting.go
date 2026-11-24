// Package waiting は andon waiting: オーナー待ち（マージの依頼・判断・手作業）と並行の枠を数える（ADR-0042 決めたこと 8・9）。
// 読むだけで、何も書かない。kanban のファイル（config.yaml・issues/<id>/issue.yaml・questions.yaml・log.jsonl）と、
// 課題の library/（report.json・gate/<head>.json・questions/<n>.json）と、project.json の chief_of_staff.parallel を直に読む。
// GitHub は読まず、git も呼ばない。kanban の CLI は、数えるのには使わない（置き場が分からないときに入口が kanban root で根を聞くだけ）。
// 数える式はこの package だけが持つ（決めたこと 14）。
package waiting

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/gate"
	"andon/internal/ledger"

	"go.yaml.in/yaml/v3"
)

// Version は出力の version（docs/contract.md §25・schema/waiting.schema.json）。
const Version = 1

// DefaultParallel は project.json に chief_of_staff.parallel が無いときの並行の上限（contract.md §11）。
const DefaultParallel = 2

// Result は andon waiting --json の出力。
type Result struct {
	Version int    `json:"version"`
	At      string `json:"at"`
	Project string `json:"project"`
	Slots   Slots  `json:"slots"`
	Items   []Item `json:"items"`
}

// Slots は並行の枠（決めたこと 9・10）。
type Slots struct {
	Limit       int  `json:"limit"`
	Working     int  `json:"working"`
	Waiting     int  `json:"waiting"`
	Free        int  `json:"free"`
	WaitingFull bool `json:"waiting_full"`
}

// Item はオーナー待ちの1行。
type Item struct {
	Kind       string             `json:"kind"` // merge・decision・task
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Since      string             `json:"since"`
	Question   string             `json:"question,omitempty"` // <id>#<n>
	PR         string             `json:"pr,omitempty"`
	Reason     string             `json:"reason,omitempty"`
	Ask        *contract.Question `json:"ask,omitempty"`        // 判断の行の問いの JSON の写し
	Answerable *bool              `json:"answerable,omitempty"` // 判断の行だけ
	Steps      string             `json:"steps,omitempty"`      // 手作業の行だけ: 依頼書（brief.md）の本文
	Links      []string           `json:"links,omitempty"`      // 手作業の行だけ: 依頼書に出てくる http(s) の URL
	Holds      []string           `json:"holds"`
	// Call は行燈を灯すか（呼んでいる）。どの行にも出す（ADR-0062 決めたこと 4・6）
	Call bool `json:"call"`
	// Later は信じる印 later の理由の 1 行。置いておける課題の行（マージの依頼を除く）にだけ出す
	Later string `json:"later,omitempty"`
	// LaterUntil は予約キー later_until の値（Later を出す行で、あれば）
	LaterUntil string `json:"later_until,omitempty"`
}

// lightKinds は判断に数える問いの種類（灯りの決断の種類。scope は Chief of Staff が答えるので入れない）。
var lightKinds = []string{"business", "users", "promise", "irreversible", "line"}

const timeLayout = "2006-01-02T15:04:05Z"

// issue は数えるのに読む課題1件。
type issue struct {
	raw       ledger.Issue
	dir       string // issues/<id>/ の置き場（依頼書を読む）
	id        string
	title     string
	status    string
	profile   string
	who       string
	after     []string
	notBefore string
	created   string
	updated   string
	onlyOwner string // 手作業の印（予約キー only_owner の 1 行目。ADR-0057 決めたこと 3）
	later     string // 置いておく印（予約キー later の 1 行目。ADR-0062 決めたこと 3）
	until     string // 予約キー later_until（YYYY-MM-DD）
	questions []question
}

// question は questions.yaml の1件。
type question struct {
	N      int    `yaml:"n"`
	Kind   string `yaml:"kind"`
	Status string `yaml:"status"`
	Asked  string `yaml:"asked"`
}

func (t *issue) reviewer() bool {
	_, _, round := ledger.SplitRound(t.id)
	return round || strings.HasSuffix(t.profile, "-review")
}

// worker は並行の枠に数える担当の課題か（レビュアーの子課題・Chief Engineer の課題・オーナーの課題でない）。
func (t *issue) worker() bool {
	return !t.reviewer() && t.profile != "chief-engineer" && t.who != "person"
}

// Read は kanban の根 root（projects/<slug>/。kanban/ の親）を読み、now の時点のオーナー待ちと枠を返す。
// warn は読めなかったが数えるのを止めないもの（project.json が読めないなど）。
func Read(root string, now time.Time) (res Result, warn []string, err error) {
	dir := filepath.Join(root, "kanban")
	cfg, err := ledger.ReadConfig(dir)
	if err != nil {
		return Result{}, nil, fmt.Errorf("kanban が読めない（%s）: %v", filepath.Join(dir, "config.yaml"), err)
	}
	limit, w := parallel(root)
	if w != "" {
		warn = append(warn, w)
	}
	issues, w2 := readIssues(dir)
	warn = append(warn, w2...)
	l := ledger.At(root)
	rows, err := l.ReadLog()
	// 履歴が無い・読めないと、課題が review に入った時刻と merge の書き手が分からず、approve で止まった PR を動いている仕事に
	// 数えてしまう。課題が1つも無い kanban の、まだ無い log.jsonl だけは空の履歴として読む
	if err != nil && (!errors.Is(err, fs.ErrNotExist) || len(issues) > 0) {
		return Result{}, warn, fmt.Errorf("kanban の履歴が読めない（%s）。枠を数えられない: %v", filepath.Join(dir, "log.jsonl"), err)
	}
	return count(l, cfg, issues, rows, limit, now), warn, nil
}

// parallel は project.json の chief_of_staff.parallel。無ければ DefaultParallel。読めない・形が違うときも DefaultParallel と理由
// （安全側。上限を広げない）。git を呼ばないため、作者の照合（delegate.ReadProject）は通さずファイルを読む。
func parallel(root string) (int, string) {
	path := filepath.Join(root, "project.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultParallel, ""
	}
	if err != nil {
		return DefaultParallel, fmt.Sprintf("%s が読めない（上限は %d で数える）: %v", path, DefaultParallel, err)
	}
	p, err := delegate.ParseProject(b, filepath.Base(root))
	if err != nil {
		return DefaultParallel, fmt.Sprintf("%s の形が違う（上限は %d で数える）: %v", path, DefaultParallel, err)
	}
	if p.ChiefOfStaff == nil || p.ChiefOfStaff.Parallel == nil {
		return DefaultParallel, ""
	}
	return *p.ChiefOfStaff.Parallel, ""
}

// readIssues は issues/*/issue.yaml と questions.yaml を読む。読めない課題は飛ばして理由を返す。
func readIssues(dir string) ([]*issue, []string) {
	ents, err := os.ReadDir(filepath.Join(dir, "issues"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("%s が読めない: %v", filepath.Join(dir, "issues"), err)}
	}
	var out []*issue
	var warn []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		id := ledger.CanonID(e.Name())
		b, err := os.ReadFile(filepath.Join(dir, "issues", e.Name(), "issue.yaml"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var raw ledger.Issue
		if err != nil || yaml.Unmarshal(b, &raw) != nil || raw == nil {
			warn = append(warn, fmt.Sprintf("%s の issue.yaml が読めない（数えない）", id))
			continue
		}
		raw["id"] = id
		t := &issue{raw: raw, dir: filepath.Join(dir, "issues", e.Name()), id: id, title: raw.Str("title"), status: raw.Str("status"), profile: raw.Str("profile"),
			who: raw.Str("who"), notBefore: raw.Str("not_before"), created: raw.Str("created"), updated: raw.Str("updated"),
			onlyOwner: firstLine(raw.Str("only_owner")), later: firstLine(raw.Str("later")), until: strings.TrimSpace(raw.Str("later_until"))}
		if list, ok := raw["after"].([]any); ok {
			for _, a := range list {
				if s, ok := a.(string); ok && s != "" {
					t.after = append(t.after, ledger.CanonID(s))
				}
			}
		}
		if qb, err := os.ReadFile(filepath.Join(dir, "issues", e.Name(), "questions.yaml")); err == nil {
			if yaml.Unmarshal(qb, &t.questions) != nil {
				warn = append(warn, fmt.Sprintf("%s の questions.yaml が読めない（問いを数えない）", id))
				t.questions = nil
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i].id, out[j].id) })
	return out, warn
}

// count は読んだ kanban から出力を作る。
func count(l *ledger.Ledger, cfg ledger.Config, issues []*issue, rows []ledger.Row, limit int, now time.Time) Result {
	done := "done"
	if len(cfg.Columns) > 0 {
		done = cfg.Columns[len(cfg.Columns)-1]
	}
	byID := map[string]*issue{}
	for _, t := range issues {
		byID[t.id] = t
	}
	// 課題が最後に review に入った行の時刻（gate の記録はこのあとのものだけを見る。internal/issue の gatedSince と同じ引き方）
	reviewAt := map[string]string{}
	for _, r := range rows {
		if r.Change != nil && r.Change["status"] == "review" {
			reviewAt[r.ID] = r.At
		}
	}
	loc := time.UTC
	if cfg.Timezone != "" {
		if z, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = z
		}
	}
	// Chief of Staff が止めた課題（予約キー hold。ADR-0056 決めたこと 11）: マージの依頼に出さず、理由の課題の行の holds に数え、
	// 枠ではオーナー待ちに数える。理由の課題がもう行を持たない（オーナーが答えた）あとは、外すまでどの行にも出ない（ADR-0057 事実 19）
	raws := make([]ledger.Issue, len(issues))
	for i, t := range issues {
		raws[i] = t.raw
	}
	heldOn := map[string][]string{}
	items := []Item{}
	waiting := map[string]bool{}
	for _, t := range issues {
		if t.status == done {
			continue
		}
		lib := l.Library(t.id)
		if reason, held := ledger.HoldOf(rows, raws, cfg.ChiefOfStaff, t.raw); held {
			heldOn[ledger.CanonID(reason)] = append(heldOn[ledger.CanonID(reason)], t.id)
			waiting[t.id] = true
		} else if it, ok := mergeItem(l, t, rows, reviewAt[t.id], lib); ok {
			items = append(items, it)
			waiting[t.id] = true
		}
		// 前の課題を待つオーナーの課題の判断は、まだオーナーの番でない（ADR-0057 決めたこと 5）。手作業の行は今のまま
		if t.who != "person" || !waitsOnPrev(t, byID, done) {
			for _, it := range decisionItems(t, lib) {
				items = append(items, it)
				waiting[t.id] = true
			}
			if it, ok := unaskedItem(t, lib); ok {
				items = append(items, it)
			}
		}
		if it, ok := taskItem(t, byID, done, now.In(loc)); ok {
			items = append(items, it)
		}
	}
	holds := holders(issues, heldOn, done)
	placeable := map[string]bool{}
	for _, t := range issues {
		placeable[t.id] = t.status != done && t.placeable(rows, cfg.ChiefOfStaff)
	}
	today := now.In(loc).Format("2006-01-02")
	for i := range items {
		it := &items[i]
		it.Holds = holds(it.ID)
		t := byID[it.ID]
		it.Call = call(*it, t, placeable, rows, cfg.ChiefOfStaff, today)
		if it.Kind != "merge" && placeable[it.ID] {
			it.Later, it.LaterUntil = t.later, t.until
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		// 呼んでいる行が先で、後ろで待つ課題の多い順、古い順。置いてある行は後で、古い順（ADR-0062 決めたこと 6）
		if a.Call != b.Call {
			return a.Call
		}
		if a.Call && len(a.Holds) != len(b.Holds) {
			return len(a.Holds) > len(b.Holds)
		}
		if a.Since != b.Since {
			return a.Since < b.Since
		}
		if a.ID != b.ID {
			return ledger.IDLess(a.ID, b.ID)
		}
		return a.Question < b.Question
	})
	s := Slots{Limit: limit}
	for _, t := range issues {
		if (t.status != "doing" && t.status != "review") || !t.worker() {
			continue
		}
		if waiting[t.id] {
			s.Waiting++
		} else {
			s.Working++
		}
	}
	s.WaitingFull = s.Waiting >= s.Limit
	if !s.WaitingFull {
		s.Free = max(s.Limit-s.Working, 0)
	}
	return Result{Version: Version, At: now.UTC().Format(timeLayout), Project: l.Slug, Slots: s, Items: items}
}

// placeable は置いておける課題か（ADR-0062 決めたこと 3）: オーナーの課題か、状態が todo か queued の課題で、
// 信じる印 later（その行の時点の Chief of Staff の端末が最後に書いた、空でない値）を持つ。状態が doing か review の、
// オーナーの課題でない課題（担当と Chief Engineer が動かしている課題と、マージの依頼）は、印があっても置いておけない。
func (t *issue) placeable(rows []ledger.Row, cos string) bool {
	if t.who != "person" && t.status != "todo" && t.status != "queued" {
		return false
	}
	return t.later != "" && ledger.TrustedThen(rows, t.raw, "later", cos)
}

// call は行が呼んでいるか（ADR-0062 決めたこと 4 の細部の 1〜6 を上から順に）。holds の相手は、相手の印と状態だけを読み、
// 相手の行の call を読まない。today は config.yaml の timezone（無ければ UTC）の今日。
func call(it Item, t *issue, placeable map[string]bool, rows []ledger.Row, cos, today string) bool {
	switch {
	case it.Kind == "merge":
		return true
	case it.Kind == "decision" && it.Question == "": // 問いも印 only_owner も無いオーナーの課題の行
		return true
	case t == nil || !placeable[it.ID]:
		return true
	}
	if t.until != "" {
		day, err := time.Parse("2006-01-02", t.until)
		// 形の違う日付と、その時点の Chief of Staff が書いていない日付は、日が来たものとして読む
		if err != nil || !ledger.TrustedThen(rows, t.raw, "later_until", cos) || day.Format("2006-01-02") <= today {
			return true
		}
	}
	for _, h := range it.Holds {
		if !placeable[h] {
			return true
		}
	}
	return false
}

// mergeItem はマージの依頼（決めたこと 8）。課題が review にいて、次のどれか: 今の head について gate が出した問いが開いている／
// 今の head の gate の判定が approve で、merge: auto でないか、merge: auto を最後に書いたのが Chief of Staff でない／
// Chief Engineer の課題で、報告に PR がある。gate の記録にマージ済みとある課題は数えない。
func mergeItem(l *ledger.Ledger, t *issue, rows []ledger.Row, since, lib string) (Item, bool) {
	if t.status != "review" {
		return Item{}, false
	}
	rep, _ := contract.ReadReport(filepath.Join(lib, "report.json"))
	head, pr := "", ""
	if rep != nil {
		head, pr = rep.Commit, rep.PR
	}
	rec := gateRecord(lib, head)
	if rec != nil && rec.Merged {
		return Item{}, false
	}
	it := Item{Kind: "merge", ID: t.id, Title: t.title, PR: pr}
	for _, q := range t.questions {
		if q.Status != "open" {
			continue
		}
		ask := readAsk(lib, q.N)
		if ask == nil || ask.From != "gate" {
			continue
		}
		// どの head の問いか読めない問いは今の head の問いとして数える（andon issue ready の staleGateQuestions と同じ）
		if h := gate.QuestionHead(ask.Facts); h != "" && h != head {
			continue
		}
		it.Question, it.Since = fmt.Sprintf("%s#%d", t.id, q.N), firstOf(q.Asked, t.updated, t.created)
		it.Reason = ask.Question
		if rec != nil && len(rec.Reasons) > 0 {
			it.Reason = strings.Join(rec.Reasons, "／")
		}
		return it, true
	}
	if rec != nil && since != "" && rec.At >= since && rec.Decision == "approve" {
		s := ledger.SettingsOf(t.raw)
		switch {
		case s.Merge != "auto":
			it.Reason = "gate は approve。merge が auto でない（オーナーがマージする）"
		case !l.Trusted(rows, t.raw, "merge"):
			it.Reason = "gate は approve。merge: auto を最後に書いたのが Chief of Staff でない"
		default:
			return Item{}, false // dispatcher が自動でマージする途中
		}
		it.Since = firstOf(rec.At, t.updated, t.created)
		return it, true
	}
	if t.profile == "chief-engineer" && pr != "" {
		it.Reason = "Chief Engineer の文書の PR"
		it.Since = firstOf(since, t.updated, t.created)
		return it, true
	}
	return Item{}, false
}

// decisionItems は判断（ADR-0042 決めたこと 8）: 灯りの種類の問いが開いていて、出したのが gate でない。問い1件に1行。
// 前の課題を待つオーナーの課題は、呼ぶ前に外す（ADR-0057 決めたこと 5）。
func decisionItems(t *issue, lib string) []Item {
	var out []Item
	for _, q := range t.questions {
		if q.Status != "open" {
			continue
		}
		ask := readAsk(lib, q.N)
		if ask != nil && ask.From == "gate" {
			continue
		}
		kind := q.Kind
		if ask != nil {
			kind = ask.Kind // 写す問いの JSON の種類で決める（questions.yaml と食い違っても ask と行の種類を揃える）
		}
		if !slices.Contains(lightKinds, kind) {
			continue
		}
		it := Item{Kind: "decision", ID: t.id, Title: t.title, Since: firstOf(q.Asked, t.updated, t.created),
			Question: fmt.Sprintf("%s#%d", t.id, q.N), Ask: ask}
		answerable := ask != nil && len(ask.Options) > 0 && ask.Kind != "irreversible"
		it.Answerable = &answerable
		out = append(out, it)
	}
	return out
}

// questionKind は問いの種類。問いの JSON があればその種類で決める（questions.yaml と食い違っても行の種類と揃える）。
func questionKind(lib string, q question) string {
	if ask := readAsk(lib, q.N); ask != nil {
		return ask.Kind
	}
	return q.Kind
}

// owed はオーナーの課題が起動できるか動いているか（手作業と、問いの置かれていない判断を数える状態）。
func (t *issue) owed() bool {
	return t.who == "person" && (t.status == "todo" || t.status == "queued" || t.status == "doing")
}

// waitsOnPrev は after の相手に済んでいない課題（か kanban に無い課題）があるか。
func waitsOnPrev(t *issue, byID map[string]*issue, done string) bool {
	for _, a := range t.after {
		if o := byID[a]; o == nil || o.status != done {
			return true
		}
	}
	return false
}

// unaskedReason は問いの置かれていない判断の行の理由（直す 1 手。ADR-0057 決めたこと 4）。
const unaskedReason = "オーナーに聞く問いが置かれていない。判断なら問いを置き、オーナーにしかできない手作業なら印 only_owner を書く（Chief of Staff が直す）"

// unaskedItem は問いの置かれていない判断（ADR-0057 決めたこと 4）: オーナーの課題で、起動できるか動いていて、
// 灯りの種類の問いを1つも持たず（種類の無い問いと scope の問いはオーナーに届かないので数えない）、印 only_owner も無い。
// 問いの番号を持たず、ペインからは答えられない。日付（not_before）では隠さない（決めたこと 5）。
func unaskedItem(t *issue, lib string) (Item, bool) {
	if !t.owed() || t.onlyOwner != "" {
		return Item{}, false
	}
	for _, q := range t.questions {
		if slices.Contains(lightKinds, questionKind(lib, q)) {
			return Item{}, false
		}
	}
	answerable := false
	return Item{Kind: "decision", ID: t.id, Title: t.title, Since: firstOf(t.created, t.updated), Reason: unaskedReason,
		Answerable: &answerable}, true
}

// taskItem は手作業（ADR-0042 決めたこと 8・ADR-0057 決めたこと 2・3）: 印 only_owner を持つオーナーの課題で、起動できるか
// 動いていて、受け取られていない問いを持たない。起動できる前（todo・queued で前の課題か日付を待っている間）は出さない。理由は印の値。
func taskItem(t *issue, byID map[string]*issue, done string, now time.Time) (Item, bool) {
	if !t.owed() || t.onlyOwner == "" {
		return Item{}, false
	}
	for _, q := range t.questions {
		if q.Status == "open" || q.Status == "answered" {
			return Item{}, false
		}
	}
	if t.status != "doing" {
		if waitsOnPrev(t, byID, done) {
			return Item{}, false // 前の課題を待っている（まだオーナーの番でない）
		}
		if day, err := time.Parse("2006-01-02", t.notBefore); err == nil && now.Format("2006-01-02") < day.Format("2006-01-02") {
			return Item{}, false // 日付を待っている
		}
	}
	steps, links := brief(t.dir)
	return Item{Kind: "task", ID: t.id, Title: t.title, Since: firstOf(t.created, t.updated), Reason: t.onlyOwner,
		Steps: steps, Links: links}, true
}

// maxSteps は手作業の行に運ぶ依頼書の本文の上限（文字数）。超えた分は落として末尾に印を付ける（URL は全文から拾う）。
const maxSteps = 6000

// stepsCut は本文を上限で切ったときの末尾の印。
const stepsCut = "\n…（続きは依頼書）"

// urlPattern は依頼書の中の http(s) の URL。印字できる ASCII の続きで、空白・全角の文字（日本語の文にくっついた URL）で切れる。
var urlPattern = regexp.MustCompile(`https?://[!-~]+`)

// brief は手作業の行に運ぶ依頼書（issues/<id>/brief.md）の本文と、その中の http(s) の URL（出てくる順、重ねない）。
// 依頼書が無い・読めない・空なら両方とも空（行はそのまま出す）。手順の部分を切り出さず、本文を運ぶ（依頼書の形は課題ごとに違う）。
func brief(dir string) (string, []string) {
	b, err := readBrief(filepath.Join(dir, "brief.md"))
	if err != nil || !utf8.Valid(b) {
		return "", nil
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if text == "" {
		return "", nil
	}
	var links []string
	for _, u := range urlPattern.FindAllString(text, -1) {
		// 山括弧・引用符・バッククォートで切り、文の終わりの句読点と、Markdown のリンクや括弧の閉じを落とす
		if i := strings.IndexAny(u, "<>\"'`"); i >= 0 {
			u = u[:i]
		}
		if i := strings.Index(u, "]("); i >= 0 {
			u = u[:i] // [URL](URL) の前の URL
		}
		u = strings.TrimRight(u, ".,;:!?()[]{}*_")
		if _, host, _ := strings.Cut(u, "://"); host != "" && !slices.Contains(links, u) {
			links = append(links, u)
		}
	}
	if r := []rune(text); len(r) > maxSteps {
		text = strings.TrimSpace(string(r[:maxSteps])) + stepsCut
	}
	return text, links
}

// maxBriefBytes は依頼書を読む上限。超えた分は読まない（手作業の行に運ぶ本文は maxSteps 文字まで）。
const maxBriefBytes = 1 << 20

// readBrief は依頼書を読む。普通のファイルでなければ（FIFO・装置を指す symlink）読まない: 読むと止まるか際限なく読み、
// andon waiting と、それを呼ぶ dispatcher が止まる。上限を超えた分は読まず、末尾で切れた文字を落とす。
func readBrief(path string) ([]byte, error) {
	fh, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close() //nolint:errcheck // 読むだけのファイル
	b, err := io.ReadAll(io.LimitReader(fh, maxBriefBytes))
	if err != nil {
		return nil, err
	}
	if len(b) == maxBriefBytes {
		for i := 0; i < utf8.UTFMax && len(b) > 0 && !utf8.Valid(b); i++ {
			b = b[:len(b)-1]
		}
	}
	return b, nil
}

// openRegular は path を開き、開いたものが普通のファイルのときだけ返す。開く前に見ると、見てから開くまでに FIFO へ
// 差し替えられたとき開くところで止まるので、待たずに開いてから、開いたものを見る。
func openRegular(path string) (*os.File, error) {
	fh, err := openNoWait(path)
	if err != nil {
		return nil, err
	}
	st, err := fh.Stat()
	if err == nil && !st.Mode().IsRegular() {
		err = fmt.Errorf("%s は普通のファイルでない", path)
	}
	if err != nil {
		fh.Close() //nolint:errcheck // 読まずに閉じる
		return nil, err
	}
	return fh, nil
}

// firstLine は値の 1 行目（前後の空白を除く）。
func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return strings.TrimSpace(s)
}

// holders は、課題の id から、それを after で待つ済んでいない課題と、それを理由に止めた課題 heldOn（直接と、その後ろで待つもの全部）を
// id の順で返す関数を作る。
func holders(issues []*issue, heldOn map[string][]string, done string) func(string) []string {
	waitsOn := map[string][]string{}
	for r, ids := range heldOn {
		waitsOn[r] = append(waitsOn[r], ids...)
	}
	for _, t := range issues {
		if t.status == done {
			continue
		}
		for _, a := range t.after {
			waitsOn[a] = append(waitsOn[a], t.id)
		}
	}
	return func(id string) []string {
		seen := map[string]bool{id: true}
		out := []string{}
		next := []string{id}
		for len(next) > 0 {
			cur := next[0]
			next = next[1:]
			for _, d := range waitsOn[cur] {
				if !seen[d] {
					seen[d] = true
					out = append(out, d)
					next = append(next, d)
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i], out[j]) })
		return out
	}
}

// gateRecord は今の head の gate の記録（library/gate/<head>.json）。無い・読めなければ nil。
func gateRecord(lib, head string) *gate.Record {
	if !contract.IsSHA(head) {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(lib, "gate", head+".json"))
	var g gate.Record
	if err != nil || json.Unmarshal(b, &g) != nil || g.Decision == "" {
		return nil
	}
	return &g
}

// readAsk は問いの JSON（library/questions/<n>.json）。無い・形が違えば nil（問いの JSON を持たない問い）。
func readAsk(lib string, n int) *contract.Question {
	b, err := os.ReadFile(filepath.Join(lib, "questions", strconv.Itoa(n)+".json"))
	if err != nil {
		return nil
	}
	q, err := contract.ParseQuestion(b)
	if err != nil {
		return nil
	}
	return q
}

// firstOf は最初の空でない時刻（UTC の YYYY-MM-DDTHH:MM:SSZ）。どれも形が違えば 1970 年の頭。
func firstOf(vs ...string) string {
	for _, v := range vs {
		if _, err := time.Parse(timeLayout, v); err == nil {
			return v
		}
	}
	return "1970-01-01T00:00:00Z"
}
