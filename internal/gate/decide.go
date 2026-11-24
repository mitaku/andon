package gate

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// Table は gate の条件表（package/gate.json ＋ セルの gate.local.json。キーごとの置き換え）。
type Table struct {
	SizeReviewLines        *int                 `json:"size_review_lines,omitempty"`
	SizeHumanLines         *int                 `json:"size_human_lines,omitempty"`
	TestPaths              []string             `json:"test_paths,omitempty"`
	TierAPaths             []string             `json:"tier_a_paths,omitempty"`
	TierBPaths             []string             `json:"tier_b_paths,omitempty"`
	TierCPaths             []string             `json:"tier_c_paths,omitempty"`
	TierCPathExceptions    map[string][]string  `json:"tier_c_path_exceptions,omitempty"`
	TierCAddedLinePatterns []string             `json:"tier_c_added_line_patterns,omitempty"`
	GitHubChecksWait       *int                 `json:"github_checks_wait_minutes,omitempty"`
	ScreenPaths            []string             `json:"screen_paths,omitempty"`
	Repos                  map[string]RepoTable `json:"repos,omitempty"`
}

// RepoTable は repo ごとの条件。
type RepoTable struct {
	Merge         string            `json:"merge,omitempty"`
	StripPrefix   string            `json:"strip_prefix,omitempty"`
	Check         []string          `json:"check,omitempty"`
	TierOverrides map[string]string `json:"tier_overrides,omitempty"`
	GitHubChecks  string            `json:"github_checks,omitempty"` // required（既定）/ optional
}

// File は PR の変更された1ファイル。
type File struct {
	Path         string
	PreviousPath string
	Additions    int
	Deletions    int
}

// Classified はレベルの判定。
type Classified struct {
	Tier         string
	TierReasons  []string
	ChangedLines int
}

var globCache = map[string]*regexp.Regexp{}

// Glob は条件表の glob が path に当たるか（** は深さ0以上、* と ? は1成分の中）。
func Glob(g, path string) bool {
	rx, ok := globCache[g]
	if !ok {
		var b strings.Builder
		b.WriteString("^")
		// リテラルは rune 単位で逃がす（1 byte ずつ逃がすと非 ASCII の字が壊れて当たらない）
		for i := 0; i < len(g); {
			switch {
			case strings.HasPrefix(g[i:], "**/"):
				b.WriteString("(?:.*/)?")
				i += 3
			case strings.HasPrefix(g[i:], "**"):
				b.WriteString(".*")
				i += 2
			case g[i] == '*':
				b.WriteString("[^/]*")
				i++
			case g[i] == '?':
				b.WriteString("[^/]")
				i++
			default:
				_, n := utf8.DecodeRuneInString(g[i:])
				b.WriteString(regexp.QuoteMeta(g[i : i+n]))
				i += n
			}
		}
		b.WriteString("$")
		rx = regexp.MustCompile(b.String())
		globCache[g] = rx
	}
	return rx.MatchString(path)
}

func matchAny(globs []string, path string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return Glob(g, path) })
}

// matchesC は path がレベル C の path か。例外は対象の C 規則だけに効く（skills の例外で別の C 規則を外さない）。
func (t *Table) matchesC(path string) bool {
	for _, g := range t.TierCPaths {
		except := t.TierCPathExceptions[g]
		switch {
		case len(except) == 0:
			if Glob(g, path) {
				return true
			}
		case strings.HasPrefix(g, "**/"):
			parts := strings.Split(path, "/")
			trimmed := make([]string, len(except))
			for i, e := range except {
				trimmed[i] = strings.TrimPrefix(e, "**/")
			}
			for i := range parts {
				suffix := strings.Join(parts[i:], "/")
				if Glob(strings.TrimPrefix(g, "**/"), suffix) && !matchAny(trimmed, suffix) {
					return true
				}
			}
		default:
			if Glob(g, path) && !matchAny(except, path) {
				return true
			}
		}
	}
	return false
}

// Classify は変更のレベルを決める（純粋関数）。reviewOn は課題の review: on。
func (t *Table) Classify(repo RepoTable, files []File, diff string, reviewOn bool) (Classified, error) {
	var c, b []string
	lines := 0
	var paths []string
	for _, f := range files {
		ps := []string{strings.TrimPrefix(f.Path, repo.StripPrefix)}
		if f.PreviousPath != "" && f.PreviousPath != f.Path {
			ps = append(ps, strings.TrimPrefix(f.PreviousPath, repo.StripPrefix))
		}
		test := true
		for _, p := range ps {
			if !matchAny(t.TestPaths, p) {
				test = false
			}
		}
		if !test {
			lines += f.Additions + f.Deletions
		}
		paths = append(paths, ps...)
	}
	for _, p := range paths {
		if tier, ok := override(repo.TierOverrides, p); ok {
			switch tier {
			case "C":
				c = append(c, "path "+p+"（tier_overrides）")
			case "B":
				b = append(b, "path "+p+"（tier_overrides）")
			}
			continue
		}
		if t.matchesC(p) {
			c = append(c, "path "+p)
		}
		if matchAny(t.TierBPaths, p) {
			b = append(b, "実行面 "+p)
		}
		if !matchAny(t.TierAPaths, p) {
			b = append(b, "段 A 許可リスト外 "+p)
		}
	}
	hunks := Hunks(diff)
	for _, pat := range t.TierCAddedLinePatterns {
		rx, err := regexp.Compile(pat)
		if err != nil {
			return Classified{}, fmt.Errorf("tier_c_added_line_patterns が読めない: %q", pat)
		}
		switch hit, grown := patternHits(rx, hunks); {
		case grown:
			c = append(c, "追加行 "+pat)
		case hit:
			b = append(b, "追加行 "+pat+NotGrown)
		}
	}
	if t.SizeHumanLines != nil && lines > *t.SizeHumanLines {
		c = append(c, fmt.Sprintf("%d 行（size_human_lines %d 超）", lines, *t.SizeHumanLines))
	}
	if t.SizeReviewLines != nil && lines > *t.SizeReviewLines {
		b = append(b, fmt.Sprintf("%d 行（size_review_lines %d 超）", lines, *t.SizeReviewLines))
	}
	if reviewOn {
		b = append(b, "review: on")
	}
	switch {
	case len(c) > 0:
		return Classified{"C", c, lines}, nil
	case len(b) > 0:
		return Classified{"B", b, lines}, nil
	}
	return Classified{"A", []string{}, lines}, nil
}

// NotGrown は追加行のパターンに当たったが、どの hunk でも当たる箇所が増えていないときに理由の末尾に付ける文
// （ADR-0041 決めたこと 4。もとからあった語を書き換えた行。レベル B の理由にする）。
const NotGrown = "（増えていない）"

// Hunk は差分の1つの hunk の、足した行と消した行（行の頭の + と - を外した中身）。
type Hunk struct {
	Added   []string
	Removed []string
}

// Hunks は PR の差分（gh pr diff の unified diff）を hunk に分ける（純粋関数）。
// hunk の中は見出し（@@ -a,b +c,d @@）の行数で数えるので、中身が ++ や -- で始まる行（行の頭が +++ や ---）も
// 足した行・消した行に数える。hunk の外の見出し（--- a/x・+++ b/x）は数えない。
// 見出しの無い差分（hunk の外の + と - の行）は、diff --git の行で区切った1つの hunk として数える（読めない差分を空振りにしない）。
func Hunks(diff string) []Hunk {
	var out []Hunk
	loose := -1 // 見出しの無い + と - の行を入れている hunk（out の添字）
	oldN, newN := 0, 0
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "diff --git ") {
			// hunk の中の行は頭に +・-・空白が付くので、この行は次のファイル。見出しの行数が多すぎても次のファイルの見出しを数えない
			oldN, newN = 0, 0
		}
		if oldN > 0 || newN > 0 {
			switch {
			case strings.HasPrefix(l, "+"):
				out[len(out)-1].Added = append(out[len(out)-1].Added, l[1:])
				newN--
			case strings.HasPrefix(l, "-"):
				out[len(out)-1].Removed = append(out[len(out)-1].Removed, l[1:])
				oldN--
			case strings.HasPrefix(l, `\`): // \ No newline at end of file
			default:
				oldN--
				newN--
			}
			continue
		}
		switch {
		case strings.HasPrefix(l, "diff --git "):
			loose = -1
		case strings.HasPrefix(l, "@@ "):
			loose = -1
			if m := hunkCountRe.FindStringSubmatch(l); m != nil {
				oldN, newN = hunkCount(m[1]), hunkCount(m[2])
				out = append(out, Hunk{})
			}
		case strings.HasPrefix(l, "+++ "), strings.HasPrefix(l, "--- "):
		case strings.HasPrefix(l, "+"), strings.HasPrefix(l, "-"):
			if loose < 0 {
				out = append(out, Hunk{})
				loose = len(out) - 1
			}
			if l[0] == '+' {
				out[loose].Added = append(out[loose].Added, l[1:])
			} else {
				out[loose].Removed = append(out[loose].Removed, l[1:])
			}
		}
	}
	return out
}

var hunkCountRe = regexp.MustCompile(`^@@ -[0-9]+(,[0-9]+)? \+[0-9]+(,[0-9]+)? @@`)

// hunkCount は hunk の見出しの「,b」の行数（省けば 1）。
func hunkCount(s string) int {
	if s == "" {
		return 1
	}
	n, _ := strconv.Atoi(s[1:])
	return n
}

// patternHits は、足した行が rx に当たるか（hit）と、当たった文字列の数が足した行のほうで多い hunk が1つでもあるか（grown）を返す
// （ADR-0041 決めたこと 4。数えるのは当たった文字列ごと。消した行の同じ文字列と打ち消す）。
func patternHits(rx *regexp.Regexp, hunks []Hunk) (hit, grown bool) {
	for _, h := range hunks {
		n := map[string]int{}
		for _, l := range h.Added {
			for _, m := range rx.FindAllString(l, -1) {
				n[m]++
				hit = true
			}
		}
		for _, l := range h.Removed {
			for _, m := range rx.FindAllString(l, -1) {
				n[m]--
			}
		}
		for _, v := range n {
			if v > 0 {
				return true, true
			}
		}
	}
	return hit, false
}

// ScreenFiles は差分のうち条件表の screen_paths に当たるパス（純粋関数。strip_prefix を外し、rename の元も見る）。
// 画面に触れる変更で、PR に画面の証拠（添付）が要る（ADR-0018 決めたこと 2）。
func (t *Table) ScreenFiles(repo RepoTable, files []File) []string {
	hit := []string{}
	for _, f := range files {
		ps := []string{strings.TrimPrefix(f.Path, repo.StripPrefix)}
		if f.PreviousPath != "" && f.PreviousPath != f.Path {
			ps = append(ps, strings.TrimPrefix(f.PreviousPath, repo.StripPrefix))
		}
		for _, p := range ps {
			if matchAny(t.ScreenPaths, p) && !slices.Contains(hit, p) {
				hit = append(hit, p)
			}
		}
	}
	return hit
}

// attachmentRe は GitHub の添付の URL（gh pr create／edit／comment --attach と Web の画面で上げた画像・動画）。
// gh は本文に ![alt](https://github.com/user-attachments/assets/<id>) を足し、動画は URL だけを置く。
// 外の画像の URL や書き換えられていない相対パス（![x](./a.png)）は添付でない。
var attachmentRe = regexp.MustCompile(`https://github\.com/user-attachments/assets/[0-9A-Za-z][0-9A-Za-z-]*`)

// HasAttachment は PR の本文かコメントのどれかに GitHub の添付があるか（純粋関数）。
func HasAttachment(texts []string) bool {
	return slices.ContainsFunc(texts, attachmentRe.MatchString)
}

func override(o map[string]string, p string) (string, bool) {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if Glob(k, p) {
			return o[k], true
		}
	}
	return "", false
}

var tiers = []string{"A", "B", "C"}

// Raise はレベルを1つ上げる（acceptance に manual があるとき）。C は C のまま。
func Raise(tier string) string {
	if i := slices.Index(tiers, tier); i >= 0 && i < 2 {
		return tiers[i+1]
	}
	return tier
}

// AcceptanceCount は report.json の acceptance[] の件数（gate の記録の acceptance）。
// StaleRuns は記録を指す行のうち、記録の commit が head と違うもの（ADR-0035 決めたこと 2。戻さず、記録に出すだけ）。
type AcceptanceCount struct {
	Pass      int                 `json:"pass"`
	Fail      int                 `json:"fail"`
	Manual    int                 `json:"manual"`
	StaleRuns []contract.StaleRun `json:"stale_runs,omitempty"`
}

// OneWayReason は担当が risk.one_way: true と申告してレベルを C にしたときの理由。
const OneWayReason = "担当の申告: one_way"

// FromReport は Classify の判定に担当の report.json を足す（純粋関数）。担当の申告はレベルを上げる向きにだけ効く:
// acceptance に manual があれば1つ上げ、risk.one_way が true なら C にする。false・無しでは何もしない（下げない）。
// risk.impact は人が読む欄で、ここでは読まない。
func FromReport(cl Classified, rep *contract.Report) (Classified, AcceptanceCount) {
	var n AcceptanceCount
	for _, a := range rep.Acceptance {
		switch a.Result {
		case "pass":
			n.Pass++
		case "fail":
			n.Fail++
		case "manual":
			n.Manual++
		}
	}
	reasons := append([]string{}, cl.TierReasons...)
	if n.Manual > 0 && cl.Tier != "C" {
		cl.Tier = Raise(cl.Tier)
		reasons = append(reasons, fmt.Sprintf("受け入れ条件の manual が %d 件（1つ上げた）", n.Manual))
	}
	if OneWay(rep) {
		cl.Tier = "C"
		reasons = append(reasons, OneWayReason)
	}
	cl.TierReasons = reasons
	return cl, n
}

// OneWay は担当が risk.one_way: true と申告したか。
func OneWay(rep *contract.Report) bool {
	return rep != nil && rep.Risk != nil && rep.Risk.OneWay != nil && *rep.Risk.OneWay
}

// Within は tier が委譲の merge の範囲の中か（none は何も通さない）。
func Within(tier, merge string) bool {
	m := slices.Index(tiers, merge)
	return m >= 0 && slices.Index(tiers, tier) <= m
}

// Check は検査の1つの結果。
type Check struct {
	Cmd string `json:"cmd"`
	OK  bool   `json:"ok"`
}

// GateContext は gate 自身が PR の head に付ける commit status の context（GitHub の checks の事実から除く）。
const GateContext = "andon/gate"

// CheckItem は head の commit の statusCheckRollup.contexts の1件（CheckRun か StatusContext）。
type CheckItem struct {
	Typename    string `json:"__typename"`
	Name        string `json:"name"`        // CheckRun
	Status      string `json:"status"`      // CheckRun
	Conclusion  string `json:"conclusion"`  // CheckRun
	DatabaseID  int64  `json:"databaseId"`  // CheckRun（再実行ほど大きい）
	App         string `json:"app"`         // CheckRun の check suite の app（slug）
	Workflow    string `json:"workflow"`    // CheckRun の workflow の id（GraphQL の Workflow.id。表示名でなく。Actions でなければ空）
	Context     string `json:"context"`     // StatusContext
	State       string `json:"state"`       // StatusContext
	CreatedAt   string `json:"createdAt"`   // StatusContext
	Description string `json:"description"` // StatusContext（gate 自身の印を書き直すかを決めるのに読む。GateMark）
	TargetURL   string `json:"targetUrl"`   // StatusContext（gate 自身の印にコメントへのリンクがあるか。GateLink）
}

// key は同じ check を見分ける鍵（CheckRun は app・workflow の id・名前、StatusContext は context）。
func (it CheckItem) key() string {
	if it.Typename == "StatusContext" {
		return "status\x00" + it.Context
	}
	return it.Typename + "\x00" + it.App + "\x00" + it.Workflow + "\x00" + it.Name
}

// order は it が old より新しい実行なら 1、古ければ -1、判別できなければ 0
// （CheckRun は databaseId、StatusContext は createdAt。RFC 3339 の UTC は文字列の順が時刻の順）。
func (it CheckItem) order(old CheckItem) int {
	if it.Typename == "StatusContext" {
		return strings.Compare(it.CreatedAt, old.CreatedAt)
	}
	switch {
	case it.DatabaseID > old.DatabaseID:
		return 1
	case it.DatabaseID < old.DatabaseID:
		return -1
	}
	return 0
}

// Latest は同じ check の実行が複数あれば最新の1つだけを残す（再実行で古い failure を数えない）。
// 新旧を判別できない（同じ時刻・同じ id）ときは悪い方を残す。順は最初に出た順。
func Latest(items []CheckItem) []CheckItem {
	idx := map[string]int{}
	out := []CheckItem{}
	for _, it := range items {
		k := it.key()
		if i, ok := idx[k]; ok {
			switch o := it.order(out[i]); {
			case o > 0, o == 0 && checkRank[it.state()] > checkRank[out[i].state()]:
				out[i] = it
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, it)
	}
	return out
}

// state は1件の状態（success・pending・failure・unknown）。
// CheckRun: 終わっていなければ pending。結論 success・skipped・neutral は通す（GitHub のブランチ保護と同じ）、
// failure・cancelled・timed_out・startup_failure は failure、それ以外（action_required・stale・知らない値）は unknown。
// StatusContext: success は通す、pending・expected は pending、failure・error は failure、それ以外は unknown。
func (it CheckItem) state() string {
	switch it.Typename {
	case "CheckRun":
		switch strings.ToUpper(it.Status) {
		case "COMPLETED":
			switch strings.ToUpper(it.Conclusion) {
			case "SUCCESS", "SKIPPED", "NEUTRAL":
				return "success"
			case "FAILURE", "CANCELLED", "TIMED_OUT", "STARTUP_FAILURE":
				return "failure"
			}
		case "QUEUED", "IN_PROGRESS", "WAITING", "PENDING", "REQUESTED":
			return "pending"
		}
	case "StatusContext":
		switch strings.ToUpper(it.State) {
		case "SUCCESS":
			return "success"
		case "PENDING", "EXPECTED":
			return "pending"
		case "FAILURE", "ERROR":
			return "failure"
		}
	}
	return "unknown"
}

// GitHubChecks は PR の head の GitHub の checks を1つの状態にまとめたもの。
// state は failure > unknown > pending > success の順で一番悪いもの。1件も無ければ none。
// names はその状態にした checks の名前（success では通した checks 全部。記録に何を読んだかを残す。none では空）。
type GitHubChecks struct {
	State string   `json:"state"`
	Names []string `json:"names"`
}

// Passed は GitHub の checks がマージを止めないか（success か、要らない repo で1件も無い）。
func (g GitHubChecks) Passed(optional bool) bool {
	return g.State == "success" || (g.State == "none" && optional)
}

// Final は、gate がこの checks で待たない（通るか、オーナー待ちで決まる）か。条件表の検査はこのとき回す。
// 待つ（pending・まだ1つも無い）ときと落ちたときは回さない。決められない・待ちを過ぎたときも回すのは、
// オーナー待ち（緑の印）にする前に差し戻す理由を拾うため（ADR-0056 決めたこと 4）。
func (g GitHubChecks) Final(optional, overdue bool) bool {
	switch g.State {
	case "pending", "failure":
		return false
	case "none":
		return optional || overdue
	}
	return true
}

var checkRank = map[string]int{"none": 0, "success": 1, "pending": 2, "unknown": 3, "failure": 4}

// SummarizeChecks は statusCheckRollup をまとめる（純粋関数）。gate 自身の status（andon/gate）は数えない。
// 同じ check の実行が複数あれば最新の1つだけを数える（Latest）。1件ごとの状態は state。
func SummarizeChecks(items []CheckItem) GitHubChecks {
	g := GitHubChecks{State: "none", Names: []string{}}
	names := map[string][]string{}
	for _, it := range Latest(items) {
		name, state := it.Name, it.state()
		if it.Typename == "StatusContext" {
			if name = it.Context; name == GateContext {
				continue
			}
		}
		if name == "" {
			name = "(" + it.Typename + ")"
		}
		names[state] = append(names[state], name)
		if checkRank[state] > checkRank[g.State] {
			g.State = state
		}
	}
	if g.State != "none" {
		g.Names = names[g.State]
	}
	return g
}

// ChecksOverdue は、課題が最後に review に入った log.jsonl の行の at から limit 分を過ぎたか（純粋関数）。
// その行が無い・at が読めなければ過ぎたとみなす（安全側。needs_human に倒れる）。
func ChecksOverdue(rows []ledger.Row, id string, now time.Time, limit int) bool {
	at := ""
	for _, r := range rows {
		if r.ID == id && r.Change["status"] == "review" {
			at = r.At
		}
	}
	since, err := time.Parse(time.RFC3339, at)
	return err != nil || now.Sub(since) >= time.Duration(limit)*time.Minute
}

// Facts は決定式の入力（集めた事実）。
type Facts struct {
	LedgerReasons   []string // kanban と PR が合わない（fix）
	PROpen          bool     // open で draft でない
	Blockers        []string // オーナーが見る PR の事情（conflict・base・作者・fork・files の件数）
	SettingsReason  string   // 予約キーを信じられない（空なら信じる）
	RepoKnown       bool     // 条件表に repo がある
	Tier            string
	MergeLevel      string // 委譲の merge
	Checks          []Check
	ReviewRequired  bool
	Review          ledger.Review
	PreviousMustFix bool // 同じ head で Must fix が残って fix を返したことがある
	AcceptanceFail  int
	RunProblems     []string     // 受け入れ条件の行が指す記録が証拠にならない（contract.CheckRuns）
	GitHubChecks    GitHubChecks // PR の head の GitHub の checks
	ChecksOptional  bool         // 条件表の repos.<repo>.github_checks が optional（1件も無くてよい）
	ChecksOverdue   bool         // review に入ってから github_checks_wait_minutes を過ぎた
	ChecksWait      int          // github_checks_wait_minutes（理由の文に入れる）
	ScreenMissing   []string     // screen_paths に当たるのに PR に添付が無いファイル
	StaleBase       []string     // PR の分かれ目より後に既定ブランチで変わった検査の置き場のファイル（CheckPlaces）
	BaseUnread      bool         // 分かれ目より後の既定ブランチの変更が多く、検査の置き場が変わったか読み切れない
	OwnChecks       []string     // PR 自身が変えた検査の置き場のファイル（CheckPlaces）
	BaseAhead       int          // PR の分かれ目より後に既定ブランチにだけある commit の数
}

// staleBaseShown は古い base の理由に並べるファイルの数（残りは「ほか N 件」）。
const staleBaseShown = 3

// OwnChecksBehindReason は、PR 自身が検査の置き場を変え、その分かれ目より後に既定ブランチが進んだときの理由の文。
func OwnChecksBehindReason(files []string, ahead int) string {
	return fmt.Sprintf("base が古い: PR が検査の置き場を変えた（%s）のに、PR の分かれ目より後に既定ブランチが %d commit 進んだ。既定ブランチへ rebase して checks を回し直す",
		shownFiles(files), ahead)
}

// shownFiles は理由に並べるファイル（staleBaseShown 件まで。残りは「ほか N 件」）。
func shownFiles(files []string) string {
	shown := strings.Join(files[:min(len(files), staleBaseShown)], ", ")
	if n := len(files) - staleBaseShown; n > 0 {
		shown += fmt.Sprintf(" ほか %d 件", n)
	}
	return shown
}

// StaleBaseReason は古い base の理由の文（記録の reasons。外向けの文は comment.go の表）。
func StaleBaseReason(files []string, unread bool) string {
	if len(files) == 0 && unread {
		return "base が古い: PR の分かれ目より後の既定ブランチの変更が多く、検査の置き場が変わったか読み切れない。既定ブランチへ rebase して checks を回し直す"
	}
	return "base が古い: PR の分かれ目より後に既定ブランチで検査の置き場が変わった（" + shownFiles(files) + "）。既定ブランチへ rebase して checks を回し直す"
}

// Decide は決定式（純粋関数）。decision と reasons を返す。approve の reasons は空。
// wait は GitHub の checks を待つ（記録も PR への書き込みもしない。dispatcher が次の見回りで回し直す）。
// 委譲の範囲の外（オーナーがマージする変更）かは、検査・CI・レビュー・受け入れ条件・画面の証拠・CI の待ちのあと、通す直前に見る
// （ADR-0056 決めたこと 4）。オーナー待ちになるのは、委譲の中なら通していた PR だけ。PR の事情・予約キー・条件表の理由は
// gate が判定できない形なので、先に見てオーナー待ちにする。
func Decide(f Facts) (string, []string) {
	switch {
	case len(f.LedgerReasons) > 0:
		return "fix", f.LedgerReasons
	case !f.PROpen:
		return "fix", []string{"PR が open でない・draft"}
	case len(f.Blockers) > 0:
		return "needs_human", f.Blockers
	case f.SettingsReason != "":
		return "needs_human", []string{f.SettingsReason}
	case !f.RepoKnown:
		return "needs_human", []string{"条件表の repos にこの repo が無い（検査を回せない）"}
	}
	// 古い base の checks は今の検査を通っていない。検査・CI・レビューの結果より先に戻す（マージすると既定ブランチの検査が落ちる）
	if len(f.StaleBase) > 0 || f.BaseUnread {
		return "fix", []string{StaleBaseReason(f.StaleBase, f.BaseUnread)}
	}
	// 逆の向き: PR 自身が検査の置き場を変えたなら、その CI は分かれ目より後に既定ブランチへ入った変更を見ていない
	if len(f.OwnChecks) > 0 && f.BaseAhead > 0 {
		return "fix", []string{OwnChecksBehindReason(f.OwnChecks, f.BaseAhead)}
	}
	for _, c := range f.Checks {
		if !c.OK {
			return "fix", []string{"check: " + c.Cmd}
		}
	}
	if g := f.GitHubChecks; g.State == "failure" {
		return "fix", []string{"GitHub の checks が落ちた: " + strings.Join(g.Names, ", ")}
	}
	if f.ReviewRequired {
		switch r := f.Review; {
		case r.State == "missing":
			return "fix", []string{"review missing: " + r.Reason}
		case r.State == "stale":
			return "fix", []string{"review stale: " + r.Reason}
		case r.MustFixOpen > 0 && f.PreviousMustFix:
			return "needs_human", []string{fmt.Sprintf("Must fix 以上が %d 件、同じ head で2度目（担当は直さないと判断した）", r.MustFixOpen)}
		case r.MustFixOpen > 0:
			return "fix", []string{fmt.Sprintf("Must fix 以上が %d 件", r.MustFixOpen)}
		case r.WaivedHeavy > 0:
			return "needs_human", []string{fmt.Sprintf("Must fix 以上を %d 件見送った（担当がレビュアーを覆した）", r.WaivedHeavy)}
		}
	}
	if f.AcceptanceFail > 0 {
		return "fix", []string{fmt.Sprintf("受け入れ条件の fail が %d 件", f.AcceptanceFail)}
	}
	if len(f.RunProblems) > 0 {
		return "fix", f.RunProblems
	}
	if len(f.ScreenMissing) > 0 {
		reasons := make([]string, len(f.ScreenMissing))
		for i, p := range f.ScreenMissing {
			reasons[i] = "screen_paths: " + p + " に当たるが画面の証拠（添付）が無い"
		}
		return "fix", reasons
	}
	// 待ち（wait）と、GitHub の checks を通すか決められないオーナー待ちは、ほかに差し戻す理由が無いときだけ
	// （オーナー待ちの印は緑なので、差し戻す理由を先に担当へ返す。ADR-0056 決めたこと 4）
	switch g := f.GitHubChecks; {
	case g.State != "success" && g.State != "pending" && g.State != "none": // unknown・読めていない（空）
		return "needs_human", []string{"GitHub の checks に通すか決められない状態がある: " + strings.Join(g.Names, ", ")}
	case g.State == "pending":
		return "wait", []string{"GitHub の checks がまだ走っている: " + strings.Join(g.Names, ", ")}
	case g.State == "none" && !f.ChecksOptional && f.ChecksOverdue:
		return "needs_human", []string{fmt.Sprintf("GitHub の checks が1つも無いまま %d 分過ぎた（条件表の github_checks: optional でない）", f.ChecksWait)}
	case g.State == "none" && !f.ChecksOptional:
		return "wait", []string{"GitHub の checks がまだ1つも無い"}
	}
	if !Within(f.Tier, f.MergeLevel) {
		return "needs_human", []string{fmt.Sprintf("delegation: merge=%s < tier %s", f.MergeLevel, f.Tier)}
	}
	return "approve", []string{}
}

// AddedLines は PR の差分（gh pr diff の unified diff）から、足した行を file（新しい側のパス）→ 行番号で返す（純粋関数）。
// レビュアーの preexisting の指摘がこの行に当たれば、課題の前からあったはずがないので gate は数える（ledger.CheckReview）。
func AddedLines(diff string) map[string]map[int]bool {
	added := map[string]map[int]bool{}
	file, line, hunk := "", 0, false
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			file, hunk = "", false
		case !hunk && strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(l, "+++ ")
			if strings.HasPrefix(file, `"`) {
				// 非 ASCII・引用符を含む名前は C 形式で引用される（"b/\350\250\255.go"）。Go の引用の解き方で同じ byte に戻る
				if u, err := strconv.Unquote(file); err == nil {
					file = u
				} else {
					file = ""
				}
			}
			file = strings.TrimPrefix(file, "b/")
			if file == "/dev/null" {
				file = ""
			}
		case strings.HasPrefix(l, "@@ "):
			hunk = false
			if m := hunkRe.FindStringSubmatch(l); m != nil {
				line, _ = strconv.Atoi(m[1])
				hunk = true
			}
		case !hunk:
		case strings.HasPrefix(l, "+"):
			if file != "" {
				if added[file] == nil {
					added[file] = map[int]bool{}
				}
				added[file][line] = true
			}
			line++
		case strings.HasPrefix(l, " "), l == "":
			line++
		}
	}
	return added
}

var hunkRe = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@`)
