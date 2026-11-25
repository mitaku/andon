package measure

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"andon"
	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/trace"
)

// 内訳の節とプロファイルごとの節（ADR-0060 決めたこと 16・18。docs/contract.md §13）。
// 課題の分は、dispatcher が片付けのときに書いた outcome.json の breakdown を読む（生の記録は読まない。決めたこと 15）。
// Chief of Staff には outcome.json が無いので、その分だけは役ごとの生の記録（roles/chief-of-staff/<日付>.traces.jsonl）から数える。

// BreakdownSection は内訳の節。
type BreakdownSection struct {
	Issues       int             `json:"issues"` // 期間に片付いた課題（オーナーの課題を除く）
	None         int             `json:"none"`   // そのうち outcome.json の breakdown が無いか null のもの
	Waits        WorkerWaits     `json:"waits"`
	Roles        []RoleUse       `json:"roles"`
	Tools        []trace.ToolUse `json:"tools"`
	ChiefOfStaff ChiefOfStaffUse `json:"chief_of_staff"`
}

// WorkerWaits は役が担当（worker）の内訳の待ちの合計（ミリ秒）。
type WorkerWaits struct {
	Issues     int   `json:"issues"`
	QuestionMs int64 `json:"question_ms"`
	ReviewMs   int64 `json:"review_ms"`
	CIMs       int64 `json:"ci_ms"`
	GoneMs     int64 `json:"gone_ms"`
}

// RoleUse は役ごとに合わせた内訳。
type RoleUse struct {
	Role          *string              `json:"role"` // 内訳の role（null の内訳は null の行に）
	Issues        int                  `json:"issues"`
	TokenIssues   int                  `json:"token_issues"` // そのうちモデルの呼び出しの記録があり、トークンを数えた内訳
	Tokens        trace.Tokens         `json:"tokens"`
	CacheReadRate *float64             `json:"cache_read_rate"` // キャッシュ読み ÷（入力＋キャッシュ読み＋キャッシュ書き）
	Errors        trace.BreakdownError `json:"errors"`
}

// ChiefOfStaffUse は期間の Chief of Staff のトークン（役ごとの生の記録から。残っている日の分だけ）。
type ChiefOfStaffUse struct {
	Days   int           `json:"days"`   // 記録の読めた日数
	Tokens *trace.Tokens `json:"tokens"` // 記録が無ければ null
	Share  *float64      `json:"share"`  // 全体（Chief of Staff ＋ 役ごとの内訳）に占める割合
}

// Profile はプロファイルごとの節の 1 行。
type Profile struct {
	Profile      *string   `json:"profile"` // issue.yaml の profile（無ければ null）
	Issues       int       `json:"issues"`
	Worked       Median    `json:"worked_minutes"`
	Tokens       Median    `json:"tokens"`
	Gate         GateStats `json:"gate"`
	ReviewRounds Median    `json:"review_rounds"`
}

// Median は件数と中央値（件数が 0 なら null）。
type Median struct {
	Count  int      `json:"count"`
	Median *float64 `json:"median"`
}

// roleOrder は役の並び（ほかは名前の順、null は最後）。
var roleOrder = map[string]int{"worker": 0, "reviewer": 1, "chief-engineer": 2}

// outcomeBreakdowns は、期間に最後の列へ動いた課題（オーナーの課題を除く）の outcome.json の breakdown。
// 値が nil の課題は内訳が無い（outcome.json が無い・breakdown が無いか null・読めない）。
func (r *reader) outcomeBreakdowns(ids []string, byID map[string]*issue, doneAt map[string]time.Time, in func(time.Time) bool) map[string]*trace.Breakdown {
	out := map[string]*trace.Breakdown{}
	for _, id := range ids {
		t := byID[id]
		if d, ok := doneAt[id]; !ok || !in(d) || t.raw.Str("who") == "person" {
			continue
		}
		path := filepath.Join(r.dir, "issues", id, "library", "outcome.json")
		out[id] = nil
		b, ok := r.read(path)
		if !ok {
			continue
		}
		var v struct {
			Breakdown json.RawMessage `json:"breakdown"`
		}
		if json.Unmarshal(b, &v) != nil {
			r.skip(r.rel(path), 0, "JSON の object でない。この課題は内訳の無い課題に数えた")
			continue
		}
		if len(v.Breakdown) == 0 || string(v.Breakdown) == "null" {
			continue
		}
		// dispatcher は schema を通して書くが、手で直した記録や前のバージョンの記録を、黙って 0 に数えない（止まりもしない）
		if err := checkBreakdown(v.Breakdown); err != nil {
			r.skip(r.rel(path), 0, "breakdown が schema/outcome.schema.json の形でない（%v）。この課題は内訳の無い課題に数えた", oneLine(err))
			continue
		}
		var bd trace.Breakdown
		if json.Unmarshal(v.Breakdown, &bd) != nil {
			r.skip(r.rel(path), 0, "breakdown が読めない。この課題は内訳の無い課題に数えた")
			continue
		}
		out[id] = &bd
	}
	return out
}

// breakdownChecker は schema/outcome.schema.json の breakdown だけを確かめる検査器（埋め込んだ schema から 1 度だけ組む）。
var breakdownChecker = sync.OnceValues(func() (*contract.Checker, error) {
	b, err := andon.Schemas.ReadFile(contract.SchemaOutcome.File())
	if err != nil {
		return nil, err
	}
	var o struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       json.RawMessage            `json:"$defs"`
	}
	if err := json.Unmarshal(b, &o); err != nil || o.Properties["breakdown"] == nil {
		return nil, fmt.Errorf("%s に breakdown が無い", contract.SchemaOutcome.File())
	}
	sub := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": ["breakdown"],` +
		`"properties": {"breakdown": ` + string(o.Properties["breakdown"]) + `}, "$defs": ` + string(o.Defs) + `}`
	return contract.Compile(contract.SchemaOutcome.File()+" の breakdown", []byte(sub))
})

// checkBreakdown は outcome.json の breakdown の値 raw が schema の形かを確かめる。
func checkBreakdown(raw json.RawMessage) error {
	c, err := breakdownChecker()
	if err != nil {
		return err
	}
	return c.Check([]byte(`{"breakdown": ` + string(raw) + `}`))
}

// oneLine は誤りを 1 行にする（飛ばした記録の理由は 1 行）。
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

// hasModelCalls は内訳がモデルの呼び出しの記録を持つか。受け口は claude_code. の span だけをトークンに数えるので、
// Codex の課題の内訳はトークンも呼び出しの時間も 0 になる。それを「トークン 0」と数えない。
func hasModelCalls(b *trace.Breakdown) bool {
	return total(b.Tokens) > 0 || b.LLMMs > 0
}

// breakdownSection は内訳の節を組む。cos は Chief of Staff の生の記録の数え（無ければ nil と 0）。
func breakdownSection(bs map[string]*trace.Breakdown, cos *trace.Tokens, cosDays int) BreakdownSection {
	s := BreakdownSection{Roles: []RoleUse{}, Tools: []trace.ToolUse{}, ChiefOfStaff: ChiefOfStaffUse{Days: cosDays, Tokens: cos}}
	ids := make([]string, 0, len(bs))
	for id := range bs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	roles := map[string]*RoleUse{}
	type toolKey struct{ name, argv0 string }
	tools := map[toolKey]*trace.ToolUse{}
	var all int64
	for _, id := range ids {
		b := bs[id]
		s.Issues++
		if b == nil {
			s.None++
			continue
		}
		key := "\x00" // role が null の内訳
		if b.Role != nil {
			key = *b.Role
		}
		ru := roles[key]
		if ru == nil {
			ru = &RoleUse{Role: b.Role}
			roles[key] = ru
		}
		ru.Issues++
		if hasModelCalls(b) {
			ru.TokenIssues++
			addTokens(&ru.Tokens, b.Tokens)
		}
		ru.Errors.LLM += b.Errors.LLM
		ru.Errors.Tool += b.Errors.Tool
		if key == "worker" {
			s.Waits.Issues++
			s.Waits.QuestionMs = plus(s.Waits.QuestionMs, b.Waits.QuestionMs)
			s.Waits.ReviewMs = plus(s.Waits.ReviewMs, b.Waits.ReviewMs)
			s.Waits.CIMs = plus(s.Waits.CIMs, b.Waits.CIMs)
			s.Waits.GoneMs = plus(s.Waits.GoneMs, b.Waits.GoneMs)
		}
		for _, u := range b.Tools {
			k := toolKey{name: u.Name}
			if u.Argv0 != nil {
				k.argv0 = "\x00" + *u.Argv0
			}
			x := tools[k]
			if x == nil {
				x = &trace.ToolUse{Name: u.Name, Argv0: u.Argv0}
				tools[k] = x
			}
			x.Count += u.Count
			x.Ms = plus(x.Ms, u.Ms)
			x.Errors += u.Errors
		}
	}
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return roleLess(keys[i], keys[j]) })
	for _, k := range keys {
		ru := roles[k]
		ru.CacheReadRate = ratio64(ru.Tokens.CacheRead, ru.Tokens.Input+ru.Tokens.CacheRead+ru.Tokens.CacheWrite)
		all += total(ru.Tokens)
		s.Roles = append(s.Roles, *ru)
	}
	for _, x := range tools {
		s.Tools = append(s.Tools, *x)
	}
	sort.Slice(s.Tools, func(i, j int) bool {
		x, y := s.Tools[i], s.Tools[j]
		if x.Ms != y.Ms {
			return x.Ms > y.Ms
		}
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		return argv0Key(x) < argv0Key(y)
	})
	if len(s.Tools) > 10 {
		s.Tools = s.Tools[:10]
	}
	if cos != nil {
		s.ChiefOfStaff.Share = ratio64(total(*cos), total(*cos)+all)
	}
	return s
}

// roleLess は役の並び: worker・reviewer・chief-engineer、ほかは名前の順、null（"\x00"）は最後。
func roleLess(a, b string) bool {
	if a == "\x00" || b == "\x00" {
		return b == "\x00" && a != "\x00"
	}
	oa, ka := roleOrder[a]
	ob, kb := roleOrder[b]
	switch {
	case ka && kb:
		return oa < ob
	case ka != kb:
		return ka
	}
	return a < b
}

func argv0Key(t trace.ToolUse) string {
	if t.Argv0 == nil {
		return ""
	}
	return "\x00" + *t.Argv0
}

// profiles はプロファイルごとの節: 期間に閉じた担当の課題（issues[] と同じ顔ぶれ）を issue.yaml の profile で分ける。
// 実働は issues[] の worked_minutes、トークンは内訳の tokens の 4 つの和（内訳の無い課題と、モデルの呼び出しの記録の無い内訳は数えない）、gate の差し戻しの率は
// その課題の gate の記録の全部、レビューの周回はその課題を親に持つレビュアーの課題の数（0 も数える）。
func profiles(xs []Issue, byID map[string]*issue, ids []string, bs map[string]*trace.Breakdown) []Profile {
	rounds := map[string]int{}
	for _, id := range ids {
		t := byID[id]
		if !t.reviewer {
			continue
		}
		p := t.parent
		if base, _, ok := ledger.SplitRound(id); ok && p == "" {
			p = base
		}
		rounds[p]++
	}
	type acc struct {
		p                      Profile
		worked, tokens, rounds []float64
	}
	groups := map[string]*acc{}
	for _, x := range xs {
		t := byID[x.Issue]
		key := t.profile
		g := groups[key]
		if g == nil {
			g = &acc{}
			if key != "" {
				k := key
				g.p.Profile = &k
			}
			groups[key] = g
		}
		g.p.Issues++
		g.worked = append(g.worked, float64(x.WorkedMinutes))
		if b := bs[x.Issue]; b != nil && hasModelCalls(b) {
			g.tokens = append(g.tokens, float64(total(b.Tokens)))
		}
		g.rounds = append(g.rounds, float64(rounds[x.Issue]))
		for _, gr := range t.gates {
			g.p.Gate.Records++
			switch gr.decision {
			case "fix":
				g.p.Gate.Fix++
			case "needs_human":
				g.p.Gate.NeedsHuman++
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { // 名前の順。profile の無い課題は最後
		if keys[i] == "" || keys[j] == "" {
			return keys[j] == "" && keys[i] != ""
		}
		return keys[i] < keys[j]
	})
	out := []Profile{}
	for _, k := range keys {
		g := groups[k]
		g.p.Worked = Median{Count: len(g.worked), Median: median(g.worked)}
		g.p.Tokens = Median{Count: len(g.tokens), Median: median(g.tokens)}
		g.p.ReviewRounds = Median{Count: len(g.rounds), Median: median(g.rounds)}
		g.p.Gate.Rate = ratio(g.p.Gate.Fix+g.p.Gate.NeedsHuman, g.p.Gate.Records)
		out = append(out, g.p)
	}
	return out
}

func addTokens(sum *trace.Tokens, t trace.Tokens) {
	sum.Input = plus(sum.Input, t.Input)
	sum.Output = plus(sum.Output, t.Output)
	sum.CacheRead = plus(sum.CacheRead, t.CacheRead)
	sum.CacheWrite = plus(sum.CacheWrite, t.CacheWrite)
}

func total(t trace.Tokens) int64 {
	return plus(plus(t.Input, t.Output), plus(t.CacheRead, t.CacheWrite))
}

// plus は 0 以上の数の和を int64 の上限で止める（内訳は上限で止めた数を持ちうる。負の数は足さない）。
func plus(a, b int64) int64 {
	if b <= 0 {
		return a
	}
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

func ratio64(n, d int64) *float64 {
	if d <= 0 {
		return nil
	}
	v := round(float64(n) / float64(d))
	return &v
}

// breakdownTable は内訳の節（役ごとの合計と、担当の待ち、時間を使ったツール、Chief of Staff）。
func breakdownTable(w io.Writer, s BreakdownSection) {
	fmt.Fprintln(w, "\n内訳（期間に片付いた課題の outcome.json の breakdown を役ごとに合わせた。トークンは入力・出力・キャッシュ読み・キャッシュ書き）")
	fmt.Fprintf(w, "  片付いた課題                   %d件・内訳の無い課題 %d件\n", s.Issues, s.None)
	if s.Waits.Issues > 0 {
		fmt.Fprintf(w, "  担当の待ち（合計。%d件）        問い %s・レビュー %s・CI %s・端末が死んでいた間 %s\n", s.Waits.Issues,
			msHours(s.Waits.QuestionMs), msHours(s.Waits.ReviewMs), msHours(s.Waits.CIMs), msHours(s.Waits.GoneMs))
	} else {
		fmt.Fprintf(w, "  担当の待ち                     %s\n", none)
	}
	for _, ru := range s.Roles {
		name := "役なし"
		if ru.Role != nil {
			name = *ru.Role
		}
		cache := none
		if ru.CacheReadRate != nil {
			cache = fmt.Sprintf("%.0f%%", *ru.CacheReadRate*100)
		}
		fmt.Fprintf(w, "  %s %d件（トークンの記録 %d件）%s（キャッシュ読み %s）・失敗 モデル %d・ツール %d\n", pad(name, 14), ru.Issues, ru.TokenIssues,
			tokenLine(ru.Tokens), cache, ru.Errors.LLM, ru.Errors.Tool)
	}
	c := s.ChiefOfStaff
	if c.Tokens == nil {
		fmt.Fprintf(w, "  %-14s %s\n", "chief-of-staff", none)
	} else {
		share := none
		if c.Share != nil {
			share = fmt.Sprintf("%.0f%%", *c.Share*100)
		}
		fmt.Fprintf(w, "  %-14s %d日分 %s（全体の %s）\n", "chief-of-staff", c.Days, tokenLine(*c.Tokens), share)
	}
	fmt.Fprintln(w, "  時間を使ったツール（上から 10 個。課題ごとの上から 10 個を合わせた）")
	if len(s.Tools) == 0 {
		fmt.Fprintf(w, "    %s\n", none)
	}
	for _, u := range s.Tools {
		name := u.Name
		if u.Argv0 != nil {
			name += " " + *u.Argv0
		}
		fmt.Fprintf(w, "    %-20s %d回・%s・失敗 %d\n", name, u.Count, msHours(u.Ms), u.Errors)
	}
}

// profilesTable はプロファイルごとの節。
func profilesTable(w io.Writer, ps []Profile) {
	fmt.Fprintln(w, "\nプロファイル（期間に閉じた担当の課題。実働は分、トークンは内訳の 4 つの和。どちらも中央値）")
	if len(ps) == 0 {
		fmt.Fprintf(w, "  %s\n", none)
	}
	for _, p := range ps {
		name := "プロファイル無し"
		if p.Profile != nil {
			name = *p.Profile
		}
		parts := []string{fmt.Sprintf("%d件", p.Issues), "実働 " + medianText(p.Worked), "トークン " + medianText(p.Tokens),
			"gate の差し戻し " + pct(p.Gate.Rate, p.Gate.Fix+p.Gate.NeedsHuman, p.Gate.Records), "レビューの周回 " + medianText(p.ReviewRounds)}
		fmt.Fprintf(w, "  %s %s\n", pad(name, 14), strings.Join(parts, "・"))
	}
}

// pad は表の欄を幅 n にそろえる（ASCII でない文字は 2 桁に数える）。
func pad(s string, n int) string {
	w := 0
	for _, c := range s {
		if c < 0x80 {
			w++
		} else {
			w += 2
		}
	}
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func medianText(m Median) string {
	if m.Median == nil {
		return none
	}
	return fmt.Sprintf("%g（%d件）", *m.Median, m.Count)
}

func msHours(ms int64) string {
	return fmt.Sprintf("%.1f 時間", float64(ms)/float64(time.Hour/time.Millisecond))
}

func tokenLine(t trace.Tokens) string {
	return fmt.Sprintf("入力 %d・出力 %d・キャッシュ読み %d・キャッシュ書き %d", t.Input, t.Output, t.CacheRead, t.CacheWrite)
}
