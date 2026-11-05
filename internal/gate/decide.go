package gate

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"andon/internal/ledger"
)

// Table はゲートの条件表（package/gate.json ＋ 建屋の gate.local.json。キーごとの置き換え）。
type Table struct {
	SizeReviewLines        *int                 `json:"size_review_lines,omitempty"`
	SizeHumanLines         *int                 `json:"size_human_lines,omitempty"`
	TestPaths              []string             `json:"test_paths,omitempty"`
	TierAPaths             []string             `json:"tier_a_paths,omitempty"`
	TierBPaths             []string             `json:"tier_b_paths,omitempty"`
	TierCPaths             []string             `json:"tier_c_paths,omitempty"`
	TierCPathExceptions    map[string][]string  `json:"tier_c_path_exceptions,omitempty"`
	TierCAddedLinePatterns []string             `json:"tier_c_added_line_patterns,omitempty"`
	Repos                  map[string]RepoTable `json:"repos,omitempty"`
}

// RepoTable は repo ごとの条件。
type RepoTable struct {
	Merge         string            `json:"merge,omitempty"`
	StripPrefix   string            `json:"strip_prefix,omitempty"`
	Check         []string          `json:"check,omitempty"`
	TierOverrides map[string]string `json:"tier_overrides,omitempty"`
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

// Classify は変更のレベルを決める（純粋関数）。reviewOn はタスクの review: on。
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
}

// Decide は決定式（純粋関数）。decision と reasons を返す。approve の reasons は空。
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
	return "approve", []string{}
}
