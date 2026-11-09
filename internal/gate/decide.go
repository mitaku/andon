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

// Table はゲートの条件表（package/gate.json ＋ セルの gate.local.json。キーごとの置き換え）。
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
	for _, pat := range t.TierCAddedLinePatterns {
		rx, err := regexp.Compile(pat)
		if err != nil {
			return Classified{}, fmt.Errorf("tier_c_added_line_patterns が読めない: %q", pat)
		}
		for _, line := range strings.Split(diff, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && rx.MatchString(line[1:]) {
				c = append(c, "追加行 "+pat)
				break
			}
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

// AcceptanceCount は report.json の acceptance[] の件数（ゲートの記録の acceptance）。
type AcceptanceCount struct {
	Pass   int `json:"pass"`
	Fail   int `json:"fail"`
	Manual int `json:"manual"`
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
	if rep.Risk != nil && rep.Risk.OneWay != nil && *rep.Risk.OneWay {
		cl.Tier = "C"
		reasons = append(reasons, OneWayReason)
	}
	cl.TierReasons = reasons
	return cl, n
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

// GateContext はゲート自身が PR の head に付ける commit status の context（GitHub の checks の事実から除く）。
const GateContext = "andon/gate"

// CheckItem は head の commit の statusCheckRollup.contexts の1件（CheckRun か StatusContext）。
type CheckItem struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`       // CheckRun
	Status     string `json:"status"`     // CheckRun
	Conclusion string `json:"conclusion"` // CheckRun
	DatabaseID int64  `json:"databaseId"` // CheckRun（再実行ほど大きい）
	App        string `json:"app"`        // CheckRun の check suite の app（slug）
	Workflow   string `json:"workflow"`   // CheckRun の workflow の id（GraphQL の Workflow.id。表示名でなく。Actions でなければ空）
	Context    string `json:"context"`    // StatusContext
	State      string `json:"state"`      // StatusContext
	CreatedAt  string `json:"createdAt"`  // StatusContext
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

var checkRank = map[string]int{"none": 0, "success": 1, "pending": 2, "unknown": 3, "failure": 4}

// SummarizeChecks は statusCheckRollup をまとめる（純粋関数）。ゲート自身の status（andon/gate）は数えない。
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
	LedgerReasons   []string // 台帳と PR が合わない（fix）
	PROpen          bool     // open で draft でない
	Blockers        []string // 人が見る PR の事情（conflict・base・作者・fork・files の件数）
	SettingsReason  string   // 予約キーを信じられない（空なら信じる）
	RepoKnown       bool     // 条件表に repo がある
	Tier            string
	MergeLevel      string // 委譲の merge
	Checks          []Check
	ReviewRequired  bool
	Review          ledger.Review
	PreviousMustFix bool // 同じ head で Must fix が残って fix を返したことがある
	AcceptanceFail  int
	GitHubChecks    GitHubChecks // PR の head の GitHub の checks
	ChecksOptional  bool         // 条件表の repos.<repo>.github_checks が optional（1件も無くてよい）
	ChecksOverdue   bool         // review に入ってから github_checks_wait_minutes を過ぎた
	ChecksWait      int          // github_checks_wait_minutes（理由の文に入れる）
}

// Decide は決定式（純粋関数）。decision と reasons を返す。approve の reasons は空。
// wait は GitHub の checks を待つ（記録も PR への書き込みもしない。dispatcher が次の見回りで回し直す）。
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
	case !Within(f.Tier, f.MergeLevel):
		return "needs_human", []string{fmt.Sprintf("delegation: merge=%s < tier %s", f.MergeLevel, f.Tier)}
	}
	for _, c := range f.Checks {
		if !c.OK {
			return "fix", []string{"check: " + c.Cmd}
		}
	}
	switch g := f.GitHubChecks; g.State {
	case "failure":
		return "fix", []string{"GitHub の checks が落ちた: " + strings.Join(g.Names, ", ")}
	case "success", "pending", "none":
	default: // unknown・読めていない（空）
		return "needs_human", []string{"GitHub の checks に通すか決められない状態がある: " + strings.Join(g.Names, ", ")}
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
	// 待ち（wait）は、ほかに止める理由が無く GitHub の checks だけが揃っていないとき
	switch g := f.GitHubChecks; {
	case g.State == "pending":
		return "wait", []string{"GitHub の checks がまだ走っている: " + strings.Join(g.Names, ", ")}
	case g.State == "none" && !f.ChecksOptional && f.ChecksOverdue:
		return "needs_human", []string{fmt.Sprintf("GitHub の checks が1つも無いまま %d 分過ぎた（条件表の github_checks: optional でない）", f.ChecksWait)}
	case g.State == "none" && !f.ChecksOptional:
		return "wait", []string{"GitHub の checks がまだ1つも無い"}
	}
	return "approve", []string{}
}

// AddedLines は PR の差分（gh pr diff の unified diff）から、足した行を file（新しい側のパス）→ 行番号で返す（純粋関数）。
// レビュアーの preexisting の指摘がこの行に当たれば、課題の前からあったはずがないのでゲートは数える（ledger.CheckReview）。
func AddedLines(diff string) map[string]map[int]bool {
	added := map[string]map[int]bool{}
	file, line, hunk := "", 0, false
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			file, hunk = "", false
		case !hunk && strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
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
