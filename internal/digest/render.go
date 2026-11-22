package digest

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"time"
)

// Loc は時刻をオーナーに見せるときのタイムゾーン（kanban の timezone）。
type Loc struct {
	loc  *time.Location
	name string
}

func newLoc(tz string) *Loc {
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			return &Loc{loc: l, name: tz}
		}
	}
	return &Loc{loc: time.UTC, name: "UTC"}
}

func (l *Loc) parse(s string) (time.Time, bool) {
	t, err := time.Parse(timeLayout, s)
	return t.In(l.loc), err == nil
}

// full は 2026-11-22 07:37 の形。
func (l *Loc) full(s string) string {
	if t, ok := l.parse(s); ok {
		return t.Format("2006-01-02 15:04")
	}
	return s
}

// short は 11/22 07:37 の形。
func (l *Loc) short(s string) string {
	if t, ok := l.parse(s); ok {
		return t.Format("1/2 15:04")
	}
	return s
}

// 色のクラス（決めたこと 10）。テンプレートはこの名前だけを書き、色の値をスタイルに渡さない。
const (
	amber = "c-amber" // マージの依頼と判断（オーナーが呼ばれているもの）
	red   = "c-red"   // 失敗して止まっているもの
)

type view struct {
	Title, Project, Period, Zone, PrevSent string
	Truncated                              bool
	Headline                               []string
	Big                                    []bigNum
	Days                                   []dayBar
	DaysMax                                int
	Waiting                                []waitRow
	Themes                                 []themeCard
	Rest                                   []ref
	Auto                                   []autoRow
	Tiers                                  []tierBar
	Concerns                               []concernRow
	Notes                                  []noteRow
	Metrics                                []metricRow
	MetricWeeks                            [2]string
	Ready                                  int
	Observations                           []Observation
	Routines                               []routineRow
	Redacted                               int
	Skipped                                []Skip
}

type bigNum struct {
	Label, Value, Class string
}

type dayBar struct {
	Label          string
	X, Mid         int
	YAuto, HAut    int
	YPerson, HPers int
	Merged, Auto   int
}

type waitRow struct {
	Kind, ID, Title, Since, Wait, PR, Class string
	Width, Holds                            int
}

type ref struct {
	ID, Title, PR, Mark string // Mark: auto（dispatcher）・person（オーナー）・空（マージしていない）
}

type themeCard struct {
	Title       string
	Body        []string
	Issues      []ref
	Width       int
	Count       int
	Figure      *Figure
	FocusIndex  int
	Before      []string
	After       []string
	Steps       []step
	HasBefore   bool
	HasSteps    bool
	StepsCount  int
	ThemeNumber int
}

type step struct {
	Text  string
	Focus bool
	Last  bool
}

type autoRow struct {
	ID, Title, PR, Tier, Theme, Grant string
	Retro                             bool
}

type tierBar struct {
	Tier         string
	Count, Width int
}

type concernRow struct {
	Kind, Title, At, Issue, URL, Class string
}

type noteRow struct {
	Text   string
	Issues []ref
}

type metricRow struct {
	Label, Cur, Prev string
	WCur, WPrev      int
}

type routineRow struct {
	Name, Title, Next string
}

var concernName = map[string]string{
	"stalled": "止まった担当", "gate_fix": "gate の差し戻し", "review_over": "レビューの上限を超えた",
	"ci_failed": "main の CI の失敗", "revert": "revert", "estimate": "見込みとのズレ",
}

// 棒の幅（SVG の座標）。
const (
	barW = 160 // 横棒の長さ
	dayW = 36  // 日ごとの棒の間隔
	dayH = 80  // 日ごとの棒の高さ
)

func scale(v, max float64, w int) int {
	if max <= 0 || v <= 0 {
		return 0
	}
	x := int(math.Round(v / max * float64(w)))
	if x < 2 {
		x = 2 // 0 でない値は見える幅にする
	}
	return x
}

// Render は担当の JSON と入力から HTML を組み立てる（数・一覧・図は入力から描く。決めたこと 8）。
func Render(t *Text, in *Input, loc *Loc) ([]byte, error) {
	v := view{Title: in.Title, Project: in.Project, Zone: loc.name, Truncated: in.Truncated, Headline: t.Headline,
		Period: loc.full(in.Since) + " 〜 " + loc.full(in.Until), Redacted: in.Redacted, Skipped: in.Skipped,
		Ready: in.Next.Ready, Observations: in.Next.Observations}
	if in.Previous != nil {
		v.PrevSent = loc.full(in.Previous.SentAt)
	}
	byID := map[string]Issue{}
	for _, x := range in.Issues {
		byID[x.ID] = x
	}
	refOf := func(id string) ref {
		x := byID[id]
		r := ref{ID: id, Title: x.Title}
		if x.PR != nil {
			r.PR = *x.PR
		}
		if x.MergedBy != nil {
			r.Mark = "person"
			if *x.MergedBy == "dispatcher" {
				r.Mark = "auto"
			}
		}
		return r
	}

	// 1. 冒頭の大きな数
	merged, auto, closed := 0, 0, 0
	for _, x := range in.Issues {
		if x.MergedBy != nil {
			merged++
			if *x.MergedBy == "dispatcher" {
				auto++
			}
		}
		if x.DoneAt != nil {
			closed++
		}
	}
	called := 0
	for _, it := range in.Waiting.Items {
		if it.Kind == "merge" || it.Kind == "decision" {
			called++
		}
	}
	calledClass := ""
	if called > 0 {
		calledClass = amber
	}
	v.Big = []bigNum{{"マージした PR", strconv.Itoa(merged), ""}, {"そのうち自動", strconv.Itoa(auto), ""},
		{"片付いた課題", strconv.Itoa(closed), ""}, {"オーナーが呼ばれている", strconv.Itoa(called), calledClass}}
	for _, d := range in.Numbers.Days {
		v.DaysMax = max(v.DaysMax, d.Merged)
	}
	for i, d := range in.Numbers.Days {
		ha := scale(float64(d.Auto), float64(v.DaysMax), dayH)
		hp := scale(float64(d.Merged-d.Auto), float64(v.DaysMax), dayH)
		label := d.Date
		if tm, err := time.Parse("2006-01-02", d.Date); err == nil {
			label = tm.Format("1/2")
		}
		v.Days = append(v.Days, dayBar{Label: label, X: i*dayW + 6, Mid: i*dayW + 18, YAuto: dayH - ha, HAut: ha, YPerson: dayH - ha - hp, HPers: hp,
			Merged: d.Merged, Auto: d.Auto})
	}

	// 2. オーナーを待っているもの
	until, _ := time.Parse(timeLayout, in.Until)
	maxH := 0.0
	hours := make([]float64, len(in.Waiting.Items))
	for i, it := range in.Waiting.Items {
		if s, err := time.Parse(timeLayout, it.Since); err == nil {
			hours[i] = math.Max(0, until.Sub(s).Hours())
		}
		maxH = math.Max(maxH, hours[i])
	}
	for i, it := range in.Waiting.Items {
		cls := ""
		if it.Kind == "merge" || it.Kind == "decision" {
			cls = amber
		}
		v.Waiting = append(v.Waiting, waitRow{Kind: kindName[it.Kind], ID: it.ID, Title: it.Title, Since: loc.short(it.Since),
			Wait: duration(hours[i]), PR: it.PR, Class: cls, Width: scale(hours[i], maxH, barW), Holds: len(it.Holds)})
	}

	// 3. 進んだこと
	inTheme := map[string]string{}
	maxN := 0
	for _, th := range t.Themes {
		maxN = max(maxN, len(th.Issues))
	}
	for i, th := range t.Themes {
		c := themeCard{Title: th.Title, Body: th.Body, Count: len(th.Issues), Width: scale(float64(len(th.Issues)), float64(maxN), barW), ThemeNumber: i + 1}
		for _, id := range th.Issues {
			c.Issues = append(c.Issues, refOf(id))
			if _, ok := inTheme[id]; !ok {
				inTheme[id] = th.Title
			}
		}
		if f := th.Figure; f != nil {
			switch f.Type {
			case "before_after":
				c.HasBefore, c.Before, c.After = true, f.Before, f.After
			case "steps":
				c.HasSteps = true
				for j, s := range f.Steps {
					c.Steps = append(c.Steps, step{Text: s, Focus: j+1 == f.Focus, Last: j == len(f.Steps)-1})
				}
			}
		}
		v.Themes = append(v.Themes, c)
	}
	// どのテーマにも入らなかったマージ済みの課題は「そのほか」に題で並べる（黙って落とさない）
	for _, x := range in.Issues {
		if _, ok := inTheme[x.ID]; !ok && x.MergedBy != nil {
			v.Rest = append(v.Rest, refOf(x.ID))
		}
	}

	// 4. オーナーを通らずに入った変更
	tiers := map[string]int{}
	for _, x := range in.Issues {
		if x.MergedBy == nil || *x.MergedBy != "dispatcher" {
			continue
		}
		r := autoRow{ID: x.ID, Title: x.Title, Theme: inTheme[x.ID], Retro: x.Retro, Tier: "—"}
		if r.Theme == "" {
			r.Theme = "そのほか"
		}
		if x.PR != nil {
			r.PR = *x.PR
		}
		if x.Tier != nil {
			r.Tier = *x.Tier
			tiers[*x.Tier]++
		}
		if x.Grant != nil {
			r.Grant = *x.Grant
		}
		v.Auto = append(v.Auto, r)
	}
	maxT := 0
	for _, n := range tiers {
		maxT = max(maxT, n)
	}
	for _, k := range []string{"A", "B", "C"} {
		v.Tiers = append(v.Tiers, tierBar{Tier: k, Count: tiers[k], Width: scale(float64(tiers[k]), float64(maxT), barW)})
	}

	// 6. 気になること
	for _, c := range in.Concerns {
		r := concernRow{Kind: concernName[c.Kind], Title: c.Title, At: loc.short(c.At)}
		if c.Issue != nil {
			r.Issue = *c.Issue
		}
		if c.URL != nil {
			r.URL = *c.URL
		}
		if c.failed() {
			r.Class = red
		}
		v.Concerns = append(v.Concerns, r)
	}
	for _, c := range t.Concerns {
		n := noteRow{Text: c.Text}
		for _, id := range c.Issues {
			n.Issues = append(n.Issues, refOf(id))
		}
		v.Notes = append(v.Notes, n)
	}

	// 7. 数字
	cw, pw := in.Numbers.Current, in.Numbers.Previous
	v.MetricWeeks = [2]string{cw.Since + "〜" + cw.Until, pw.Since + "〜" + pw.Until}
	rate := func(w Week) *float64 {
		if w.Merged == 0 {
			return nil
		}
		r := float64(w.Auto) / float64(w.Merged) * 100
		return &r
	}
	intp := func(n int) *float64 { f := float64(n); return &f }
	tok := func(p *int64) *float64 {
		if p == nil {
			return nil
		}
		f := float64(*p)
		return &f
	}
	for _, m := range []struct {
		label     string
		cur, prev *float64
		unit      func(float64) string
	}{
		{"マージした PR", intp(cw.Merged), intp(pw.Merged), func(f float64) string { return fmt.Sprintf("%.0f 本", f) }},
		{"自動でマージした割合", rate(cw), rate(pw), func(f float64) string { return fmt.Sprintf("%.0f%%", f) }},
		{"片付いた課題", intp(cw.Closed), intp(pw.Closed), func(f float64) string { return fmt.Sprintf("%.0f 件", f) }},
		{"課題が流れる時間（中央値）", cw.LeadTime, pw.LeadTime, func(f float64) string { return fmt.Sprintf("%.1f 時間", f) }},
		{"問いが開いていた時間（平均）", cw.QuestionOpen, pw.QuestionOpen, func(f float64) string { return fmt.Sprintf("%.1f 時間", f) }},
		{"トークン", tok(cw.Tokens), tok(pw.Tokens), func(f float64) string { return fmt.Sprintf("%.0f 万", f/1e4) }},
	} {
		r := metricRow{Label: m.label, Cur: "—", Prev: "—"}
		hi := 0.0
		if m.cur != nil {
			r.Cur, hi = m.unit(*m.cur), *m.cur
		}
		if m.prev != nil {
			r.Prev, hi = m.unit(*m.prev), math.Max(hi, *m.prev)
		}
		if m.cur != nil {
			r.WCur = scale(*m.cur, hi, barW)
		}
		if m.prev != nil {
			r.WPrev = scale(*m.prev, hi, barW)
		}
		v.Metrics = append(v.Metrics, r)
	}

	// 8. このあと
	for _, r := range in.Next.Routines {
		row := routineRow{Name: r.Name, Title: r.Title, Next: "止めてある"}
		if r.Next != nil {
			row.Next = loc.full(*r.Next)
		}
		v.Routines = append(v.Routines, row)
	}

	var b bytes.Buffer
	if err := page.Execute(&b, v); err != nil {
		return nil, fmt.Errorf("HTML を組み立てられない: %v", err)
	}
	return b.Bytes(), nil
}

// duration は待っている時間の人が読む形。
func duration(h float64) string {
	switch {
	case h < 1:
		return "1 時間未満"
	case h < 48:
		return fmt.Sprintf("%.0f 時間", math.Floor(h))
	default:
		return fmt.Sprintf("%.0f 日", math.Floor(h/24))
	}
}
