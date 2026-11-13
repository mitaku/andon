// Package measure は台帳と Andon の記録からラインの流れを数える（andon measure。
// docs/decisions/2026-11-09-measure-flow-metrics.md・docs/contract.md §13）。
//
// 読むだけ。台帳にも library/ にも書かず、集計を保存しない。読むもの: log.jsonl・issues/<id>/issue.yaml・
// questions.yaml・library/report.json・library/gate/<sha>.json・library/corrections.json と、
// GitHub のマージされた PR の一覧（gh pr list。repo ごとに1回）と、dispatcher の記録だけの通知
// （<slug>.recorded.jsonl）・届けた通知の記録（<slug>.delivered.jsonl）と、担当とレビュアーのセッションの記録
// （Claude Code・Codex。トークン数だけ。tokens.go）。Markdown は読まない。
// 読めない記録は止まらずに飛ばし、飛ばした件数と理由を出す（黙って 0 に数えない）。
package measure

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/dispatcher"
	"andon/internal/ledger"
)

// Usage は andon measure の使い方。
const Usage = "usage: andon measure [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]\n" +
	"台帳（KANBAN_ROOT か projects/<slug>/）の記録から、期間の流れ（待ち時間・流量・WIP）と効率（介在回数・自動マージ率・\n" +
	"ゲートの差し戻し率）と対抗の指標（revert・同じ箇所の課題・人が開け直した数）と、通知の kind ごとの素通りの数と、\n" +
	"期間に動いた課題ごとのトークン数（Claude Code・Codex のセッションの記録から）を数える。\n" +
	"読むだけで、何も書かない。\n" +
	"期間は UTC の日付で両端を含む。既定は --until が今日、--since がその6日前（7日間）。--json は docs/contract.md §13 の形。\n"

// Options は数える入力。
type Options struct {
	Root  string    // プロジェクトの根（kanban/ の親）
	Since time.Time // 期間の始まり（UTC の 0 時）
	Until time.Time // 期間の終わりの日（含む。UTC の 0 時）
	Now   time.Time
	// GH は repo（<owner>/<repo>）のマージされた PR の一覧を gh pr list --json number,url,title,body,mergedAt の形で返す。
	GH func(repo string) ([]byte, error)
	// State は dispatcher の控え（<slug>.json）のパス。gone（端末が死んでいた時刻）だけ読む。空なら読まない。
	State string
	// Recorded・Delivered は dispatcher の記録だけの通知と届けた通知の記録（contract.md §6・§17）のパス。空なら読まない。
	Recorded, Delivered string
	// ClaudeDir は Claude Code の設定の置き場（projects/ の親）、CodexSessions は Codex の記録の置き場（sessions）。空なら読まない。
	ClaudeDir, CodexSessions string
}

// Stat は件数と平均か中央値（件数が 0 なら null）。
type Stat struct {
	Count int      `json:"count"`
	Value *float64 `json:"hours"`
}

// Rate は件数と率（分母が 0 なら null）。
type Rate struct {
	Count int      `json:"count"`
	Rate  *float64 `json:"rate"`
}

// Result は andon measure --json の出力（schema/measure.schema.json）。
type Result struct {
	Project    string     `json:"project"`
	Since      string     `json:"since"`
	Until      string     `json:"until"`
	Days       int        `json:"days"`
	Value      Value      `json:"value"`
	Flow       Flow       `json:"flow"`
	Efficiency Efficiency `json:"efficiency"`
	Counter    Counter    `json:"counter"`
	News       News       `json:"news"`
	PRs        []PR       `json:"prs"`
	Estimate   Estimate   `json:"estimate"`
	Issues     []Issue    `json:"issues"`
	Tokens     []Tokens   `json:"tokens"`
	Skipped    []Skip     `json:"skipped"`
}

// Value は価値（ADR-0011 決めたこと 18）。観察の課題の報告 `observation` と Chief Engineer の課題から導く。
type Value struct {
	Observed      Observed `json:"observed"`        // (a) 期間に報告された観察の結果の内訳
	DaysToObserve DaysStat `json:"days_to_observe"` // (b) Chief Engineer の課題の done から観察の報告まで（中央値）
	NoHypothesis  int      `json:"no_hypothesis"`   // (c) 期間に done になった Chief Engineer の課題のうち、観察の課題が無いもの
}

// Observed は仮説を確かめた数。
type Observed struct {
	Count   int `json:"count"`
	Hit     int `json:"hit"`
	Miss    int `json:"miss"`
	Unknown int `json:"unknown"`
}

// DaysStat は件数と日数の中央値（無ければ null）。
type DaysStat struct {
	Count int      `json:"count"`
	Days  *float64 `json:"days"`
}

// Flow は流れ（Little の法則の3つと、依頼書からマージまで）。
type Flow struct {
	LeadTime     Stat            `json:"lead_time"`     // 依頼書からマージまで（中央値）
	Wait         map[string]Stat `json:"wait"`          // 状態ごとの1件あたりの待ち時間（平均）
	QuestionOpen Stat            `json:"question_open"` // 問いが open の間（1問あたりの平均）
	Closed       Closed          `json:"closed"`
	WIPEnd       int             `json:"wip_end"`
}

// Closed は期間に閉じた課題の数と1日あたり。
type Closed struct {
	Count  int     `json:"count"`
	PerDay float64 `json:"per_day"`
}

// Efficiency は効率。
type Efficiency struct {
	MergedPRs    int       `json:"merged_prs"`
	HumanTouches PerPR     `json:"human_touches"`
	AutoMerge    Rate      `json:"auto_merge"`
	Gate         GateStats `json:"gate"`
}

// PerPR は合計と PR あたり。
type PerPR struct {
	Total int      `json:"total"`
	PerPR *float64 `json:"per_pr"`
}

// GateStats はゲートの記録の数と差し戻し率（(fix + needs_human) ÷ 記録）。
type GateStats struct {
	Records    int      `json:"records"`
	Fix        int      `json:"fix"`
	NeedsHuman int      `json:"needs_human"`
	Rate       *float64 `json:"rate"`
}

// Counter は対抗の指標。
type Counter struct {
	Reverted  int      `json:"reverted"`
	FollowUps int      `json:"follow_ups"`
	Reopened  Reopened `json:"reopened"`
	Corrected Reopened `json:"corrected"` // 人の手直しがあった課題の数（revert を含めない。ゲートのレベルごと）
}

// Reopened は人が開け直した数（ゲートのレベルごと）。
type Reopened struct {
	Corrections string         `json:"corrections"` // recorded: corrections.json が1つでもある。none: 1つも無い（記録なし）
	ByTier      map[string]int `json:"by_tier"`
}

// News は Chief of Staff へ届けた通知の素通り（kind ごと。docs/decisions/2026-11-12-measure-news-pass-through.md）。
type News struct {
	Threshold  float64    `json:"threshold"`  // 素通りの割合がこれ以上の kind を候補にする
	Kinds      []NewsKind `json:"kinds"`      // kind の名前順
	Candidates []string   `json:"candidates"` // 記録だけへ移す候補（移すかは人が決める）
}

// NewsKind は kind 1つの数。
type NewsKind struct {
	Kind      string   `json:"kind"`
	Delivered int      `json:"delivered"` // 届けて、渡したターンの終わりを見た数
	Acked     int      `json:"acked"`     // そのうち andon news ack で終わったターンの数（素通り）
	Rate      *float64 `json:"rate"`      // acked ÷ delivered（delivered が 0 なら null）
	Recorded  int      `json:"recorded"`  // 記録だけにして届けなかった数
}

// passThrough は記録だけへ移す候補の閾値（8割以上が素通り）。
const passThrough = 0.8

// PR は期間にマージされた台帳の PR 1本。
type PR struct {
	Issue        string   `json:"issue"`
	PR           string   `json:"pr"`
	MergedAt     string   `json:"merged_at"`
	Tier         string   `json:"tier"`
	Auto         bool     `json:"auto"`
	HumanTouches int      `json:"human_touches"`
	LeadTime     *float64 `json:"lead_time_hours"`
	Reverted     bool     `json:"reverted"`
	Corrections  bool     `json:"corrections"`
}

// Estimate は見込みとのズレの全体（ADR-0014 決めたこと 9）。数えるのは issues[] の課題。
type Estimate struct {
	Count  int      `json:"count"`  // 見込みのあった課題
	Within int      `json:"within"` // そのうち実働が見込みの中に収まった（比が 1 以下）
	Rate   *float64 `json:"rate"`   // within ÷ count（count が 0 なら null）
	None   int      `json:"none"`   // 見込みの無い課題
}

// Issue は期間に最後の列へ動いた担当の課題1件の見込みと実際（ADR-0014 決めたこと 9・11 の 6）。
// 見込みが無ければ分と比は null（0 と分ける）。
type Issue struct {
	Issue                string   `json:"issue"`
	EstimateMinutes      *int     `json:"estimate_minutes"`       // 予約キー estimate の分（Chief of Staff が書いたときだけ）
	AgentEstimateMinutes *int     `json:"agent_estimate_minutes"` // report.json の estimate.size の分（担当の見込み）
	WorkedMinutes        int      `json:"worked_minutes"`         // 実働（待ちを除く。ledger.Worked）
	Ratio                *float64 `json:"ratio"`                  // 実働 ÷ 見込み
}

// Skip は飛ばした記録1件。
type Skip struct {
	Source string `json:"source"`
	Line   int    `json:"line,omitempty"`
	Reason string `json:"reason"`
}

const stamp = "2006-01-02T15:04:05Z"

var prRe = regexp.MustCompile(`^https://github\.com/([!-.0-~]+)/([!-.0-~]+)/pull/([1-9][0-9]*)$`)

type logRow struct {
	at     time.Time
	by     string
	id     string
	status string
	add    bool
}

type issue struct {
	id       string
	created  time.Time
	added    time.Time
	after    bool
	links    []string
	reviewer bool
	pr       string
	gates    []gateRec
	corr     bool
	asked    []span
	profile  string
	parent   string
	obs      string // report.json の observation.result（観察の報告が無ければ空）
	session  string // issue.yaml の session（Claude Code のセッション）
	worktree string // issue.yaml の worktree
	agentEst string // report.json の estimate.size（担当の見込み）
	raw      ledger.Issue
}

type span struct{ from, to time.Time } // to がゼロなら開いたまま

type gateRec struct {
	at       time.Time
	pr       string
	tier     string
	decision string
	merged   bool
}

type ghPR struct {
	Number   int    `json:"number"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	MergedAt string `json:"mergedAt"`
}

type reader struct {
	root    string
	dir     string
	skipped []Skip
}

func (r *reader) skip(src string, line int, format string, a ...any) {
	r.skipped = append(r.skipped, Skip{Source: src, Line: line, Reason: fmt.Sprintf(format, a...)})
}

func (r *reader) rel(p string) string {
	if s, err := filepath.Rel(r.root, p); err == nil {
		return filepath.ToSlash(s)
	}
	return p
}

func parseAt(s string) (time.Time, bool) {
	t, err := time.Parse(stamp, s)
	return t, err == nil
}

// Measure は数える。台帳の config.yaml が読めない・版を知らないときだけ誤りを返す。
func Measure(o Options) (*Result, error) {
	r := &reader{root: o.Root, dir: filepath.Join(o.Root, "kanban")}
	columns, err := r.config()
	if err != nil {
		return nil, err
	}
	first, last := columns[0], columns[len(columns)-1]
	start := o.Since
	periodEnd := o.Until.AddDate(0, 0, 1)
	end := periodEnd
	if o.Now.Before(end) {
		end = o.Now
	}
	in := func(t time.Time) bool { return !t.Before(start) && t.Before(periodEnd) }
	overlap := func(a, b time.Time) time.Duration {
		if a.Before(start) {
			a = start
		}
		if b.After(end) {
			b = end
		}
		if b.After(a) {
			return b.Sub(a)
		}
		return 0
	}

	rows := r.log()
	issues := r.issues()
	byID := map[string]*issue{}
	ids := make([]string, 0, len(issues))
	anyCorrections := false
	for _, t := range issues {
		byID[t.id] = t
		ids = append(ids, t.id)
		anyCorrections = anyCorrections || r.hasCorrections(t)
	}
	sort.Strings(ids)
	for _, row := range rows {
		if t := byID[row.id]; t != nil && row.add && t.added.IsZero() {
			t.added = row.at // issue.yaml に created が無いときの入った時刻
		}
	}

	// GitHub のマージされた PR（台帳の PR がある repo だけ）
	merged := map[string]ghPR{}
	var reverts []ghPR
	repos := map[string]bool{}
	for _, id := range ids {
		if m := prRe.FindStringSubmatch(byID[id].pr); m != nil {
			repos[m[1]+"/"+m[2]] = true
		}
	}
	repoNames := make([]string, 0, len(repos))
	for k := range repos {
		repoNames = append(repoNames, k)
	}
	sort.Strings(repoNames)
	for _, repo := range repoNames {
		list, err := r.github(o.GH, repo)
		if err != nil {
			r.skip("github.com/"+repo, 0, "マージされた PR の一覧を読めない（%v）。この repo のマージ・revert・同じ箇所の課題は数えていない", err)
			continue
		}
		for _, p := range list {
			merged[p.URL] = p
			if strings.HasPrefix(p.Title, "Revert ") {
				reverts = append(reverts, p)
			}
		}
	}

	res := &Result{
		Project: filepath.Base(o.Root),
		Since:   start.Format("2006-01-02"),
		Until:   o.Until.Format("2006-01-02"),
		Days:    int(periodEnd.Sub(start).Hours()/24 + 0.5),
		PRs:     []PR{},
	}

	// revert された台帳の PR（revert の PR が期間にマージされたもの）
	revertedPR := map[string]bool{}
	for _, rv := range reverts {
		at, ok := parseAt(rv.MergedAt)
		if !ok || !in(at) {
			continue
		}
		for _, id := range ids {
			t := byID[id]
			m := prRe.FindStringSubmatch(t.pr)
			if m == nil || t.pr == rv.URL {
				continue
			}
			if _, done := merged[t.pr]; !done {
				continue
			}
			ref := m[1] + "/" + m[2] + "#" + m[3]
			if containsRef(rv.Body, ref) || containsRef(rv.Body, t.pr) {
				revertedPR[t.pr] = true
			}
		}
	}

	// 期間にマージされた台帳の PR
	var leads []float64
	for _, id := range ids {
		t := byID[id]
		p, ok := merged[t.pr]
		if t.reviewer || !ok {
			continue
		}
		at, ok := parseAt(p.MergedAt)
		if !ok || !in(at) {
			continue
		}
		x := PR{Issue: id, PR: t.pr, MergedAt: p.MergedAt, Tier: "none", Reverted: revertedPR[t.pr], Corrections: t.corr}
		var latest time.Time
		for _, g := range t.gates {
			if g.merged && g.pr == t.pr {
				x.Auto, x.Tier = true, g.tier
				break
			}
			if !g.at.Before(latest) {
				latest, x.Tier = g.at, g.tier
			}
		}
		for _, row := range rows {
			if row.id == id && human(row.by) {
				x.HumanTouches++
			}
		}
		if born := t.born(); !born.IsZero() {
			h := round(at.Sub(born).Hours())
			x.LeadTime = &h
			leads = append(leads, h)
		}
		res.PRs = append(res.PRs, x)
	}

	// 効率
	e := &res.Efficiency
	e.MergedPRs = len(res.PRs)
	for _, x := range res.PRs {
		e.HumanTouches.Total += x.HumanTouches
		if x.Auto {
			e.AutoMerge.Count++
		}
	}
	e.HumanTouches.PerPR = ratio(e.HumanTouches.Total, e.MergedPRs)
	e.AutoMerge.Rate = ratio(e.AutoMerge.Count, e.MergedPRs)
	for _, id := range ids {
		for _, g := range byID[id].gates {
			if !in(g.at) {
				continue
			}
			e.Gate.Records++
			switch g.decision {
			case "fix":
				e.Gate.Fix++
			case "needs_human":
				e.Gate.NeedsHuman++
			}
		}
	}
	e.Gate.Rate = ratio(e.Gate.Fix+e.Gate.NeedsHuman, e.Gate.Records)

	// 流れ
	f := &res.Flow
	f.LeadTime = Stat{Count: len(leads), Value: median(leads)}
	waitSum := map[string]time.Duration{}
	waitN := map[string]int{}
	var qSum time.Duration
	closed := 0
	for _, id := range ids {
		t := byID[id]
		if t.reviewer {
			continue
		}
		cur := first
		if t.after {
			cur = "queued"
		}
		since := t.born()
		seen := map[string]bool{}
		wasClosed := false
		account := func(to time.Time) {
			if since.IsZero() || cur == last {
				return
			}
			if d := overlap(since, to); d > 0 {
				waitSum[cur] += d
				if !seen[cur] {
					seen[cur] = true
					waitN[cur]++
				}
			}
		}
		for _, row := range rows {
			if row.id != id || row.status == "" {
				continue
			}
			if row.at.Before(periodEnd) {
				account(row.at)
				cur, since = row.status, row.at
			}
			if row.status == last && in(row.at) {
				wasClosed = true
			}
		}
		account(end)
		if wasClosed {
			closed++
		}
		if !since.IsZero() && cur != first && cur != "queued" && cur != last {
			f.WIPEnd++
		}
		for _, q := range t.asked {
			to := q.to
			if to.IsZero() {
				to = end
			}
			if d := overlap(q.from, to); d > 0 {
				qSum += d
				f.QuestionOpen.Count++
			}
		}
	}
	f.Wait = map[string]Stat{}
	for _, s := range columns[:len(columns)-1] {
		st := Stat{Count: waitN[s]}
		if waitN[s] > 0 {
			h := round(waitSum[s].Hours() / float64(waitN[s]))
			st.Value = &h
		}
		f.Wait[s] = st
	}
	if f.QuestionOpen.Count > 0 {
		h := round(qSum.Hours() / float64(f.QuestionOpen.Count))
		f.QuestionOpen.Value = &h
	}
	f.Closed = Closed{Count: closed, PerDay: round(float64(closed) / float64(res.Days))}

	// 価値（ADR-0011 決めたこと 18）。観察の課題は parent が Chief Engineer の課題。
	v := &res.Value
	var doneAt, reportedAt = map[string]time.Time{}, map[string]time.Time{}
	for _, row := range rows {
		if row.status == "" {
			continue
		}
		if row.status == last {
			doneAt[row.id] = row.at // 最後に done へ動いた時刻
		}
		if _, seen := reportedAt[row.id]; !seen && (row.status == "review" || row.status == last) {
			reportedAt[row.id] = row.at // 報告を返した（review か done に最初に動いた）時刻
		}
	}
	hasChild := map[string]bool{}
	var days []float64
	for _, id := range ids {
		t := byID[id]
		if t.reviewer || t.parent == "" {
			continue
		}
		if t.obs != "" {
			hasChild[t.parent] = true // 観察の報告を持つ子だけ（通常の epic の子課題や not_before だけの課題は数えない）
		}
		if t.obs == "" {
			continue
		}
		at, ok := reportedAt[id]
		if !ok || !in(at) {
			continue
		}
		v.Observed.Count++
		switch t.obs {
		case "hit":
			v.Observed.Hit++
		case "miss":
			v.Observed.Miss++
		default:
			v.Observed.Unknown++
		}
		if d, ok := doneAt[t.parent]; ok && byID[t.parent] != nil && byID[t.parent].profile == "chief-engineer" && !at.Before(d) {
			days = append(days, at.Sub(d).Hours()/24)
		}
	}
	v.DaysToObserve = DaysStat{Count: len(days), Days: median(days)}
	for _, id := range ids {
		t := byID[id]
		if d, ok := doneAt[id]; ok && t.profile == "chief-engineer" && !t.reviewer && in(d) && !hasChild[id] {
			v.NoHypothesis++
		}
	}

	// 対抗の指標
	c := &res.Counter
	c.Reverted = len(revertedPR)
	c.Reopened = Reopened{Corrections: "none", ByTier: map[string]int{"A": 0, "B": 0, "C": 0, "none": 0}}
	if anyCorrections {
		c.Reopened.Corrections = "recorded"
	}
	c.Corrected = Reopened{Corrections: c.Reopened.Corrections, ByTier: map[string]int{"A": 0, "B": 0, "C": 0, "none": 0}}
	for _, x := range res.PRs {
		if x.Corrections {
			c.Reopened.ByTier[x.Tier]++
			c.Corrected.ByTier[x.Tier]++
		}
	}
	// 自動マージした PR の revert（期間の外でマージされたものも、revert が期間の中なら数える）
	for _, id := range ids {
		t := byID[id]
		if !revertedPR[t.pr] || t.corr && t.mergedIn(merged, in) {
			continue // corrections の側で数えた PR は二度数えない
		}
		for _, g := range t.gates {
			if g.merged && g.pr == t.pr {
				c.Reopened.ByTier[g.tier]++
				break
			}
		}
	}
	mergedLedger := map[string]string{} // マージされた台帳の PR の URL → 課題
	for _, id := range ids {
		if _, ok := merged[byID[id].pr]; ok && !byID[id].reviewer {
			mergedLedger[byID[id].pr] = id
		}
	}
	for _, id := range ids {
		t := byID[id]
		if t.reviewer || t.created.IsZero() || !in(t.created) {
			continue
		}
		refs := append([]string{}, t.links...)
		if p, ok := merged[t.pr]; ok {
			refs = append(refs, p.Body)
		}
	followUp:
		for url, owner := range mergedLedger {
			if owner == id {
				continue
			}
			for _, s := range refs {
				if containsRef(s, url) {
					c.FollowUps++
					break followUp
				}
			}
		}
	}

	res.Estimate, res.Issues = r.estimates(o, ids, byID, doneAt, in)
	res.News = r.news(o, in)
	res.Tokens = r.tokens(o, ids, byID, active(rows, in))

	sort.SliceStable(r.skipped, func(i, j int) bool {
		if r.skipped[i].Source != r.skipped[j].Source {
			return r.skipped[i].Source < r.skipped[j].Source
		}
		return r.skipped[i].Line < r.skipped[j].Line
	})
	res.Skipped = append([]Skip{}, r.skipped...)
	return res, nil
}

func (t *issue) born() time.Time {
	if !t.created.IsZero() {
		return t.created
	}
	return t.added
}

func (t *issue) mergedIn(merged map[string]ghPR, in func(time.Time) bool) bool {
	p, ok := merged[t.pr]
	if !ok {
		return false
	}
	at, ok := parseAt(p.MergedAt)
	return ok && in(at)
}

// human は log.jsonl の by が人か（terminal:・routine: でない）。
func human(by string) bool {
	return !strings.HasPrefix(by, "terminal:") && !strings.HasPrefix(by, "routine:")
}

// containsRef は本文に ref（owner/repo#n か PR の URL）が、続く数字なしで現れるか。
func containsRef(body, ref string) bool {
	for i := 0; ; {
		j := strings.Index(body[i:], ref)
		if j < 0 {
			return false
		}
		k := i + j + len(ref)
		if k == len(body) || body[k] < '0' || body[k] > '9' {
			return true
		}
		i = k
	}
}

func round(x float64) float64 { return math.Round(x*100) / 100 }

func ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := round(float64(n) / float64(d))
	return &v
}

func median(xs []float64) *float64 {
	if len(xs) == 0 {
		return nil
	}
	s := append([]float64{}, xs...)
	sort.Float64s(s)
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + m) / 2
	}
	m = round(m)
	return &m
}

// config は config.yaml の columns（無ければ既定）。版が 3 でなければ止まる（台帳の形式の規則）。
func (r *reader) config() ([]string, error) {
	b, err := os.ReadFile(filepath.Join(r.dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("台帳の config.yaml を読めない（%s）", r.rel(filepath.Join(r.dir, "config.yaml")))
	}
	var c struct {
		Version any      `yaml:"version"`
		Columns []string `yaml:"columns"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, errors.New("台帳の config.yaml が YAML でない")
	}
	if v, ok := c.Version.(int); !ok || v != 3 {
		return nil, fmt.Errorf("台帳の版が 3 でない（%v）。知らない版は読まない", c.Version)
	}
	if len(c.Columns) == 0 {
		c.Columns = []string{"todo", "queued", "doing", "review", "done"}
	}
	return c.Columns, nil
}

// read は記録のファイルを読む。無ければ ok が false（まだ無い記録）。ほかの理由で読めなければ、
// 飛ばした記録に相対パスと理由（絶対パスを含まない）を出して ok が false。
func (r *reader) read(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, true
	}
	if !errors.Is(err, os.ErrNotExist) {
		why := "読み取りの誤り"
		var pe *fs.PathError
		if errors.As(err, &pe) {
			why = pe.Err.Error()
		}
		r.skip(r.rel(path), 0, "読めない（%s）", why)
	}
	return nil, false
}

// names はディレクトリの中の項目（名前順）。ディレクトリが無ければ空（まだ無い記録）。
// ほかの理由で読めなければ、飛ばした記録に相対パスと理由（絶対パスを含まない）を出して空。
func (r *reader) names(dir string) []fs.DirEntry {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			why := "読み取りの誤り"
			var pe *fs.PathError
			if errors.As(err, &pe) {
				why = pe.Err.Error()
			}
			r.skip(r.rel(dir), 0, "ディレクトリを読めない（%s）", why)
		}
		return nil
	}
	return ents
}

func (r *reader) subdirs(dir string) []string {
	var out []string
	for _, e := range r.names(dir) {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

func (r *reader) files(dir, ext string) []string {
	var out []string
	for _, e := range r.names(dir) {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func (r *reader) log() []logRow {
	path := filepath.Join(r.dir, "log.jsonl")
	b, ok := r.read(path)
	if !ok {
		return nil
	}
	var rows []logRow
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var v struct {
			At     string                     `json:"at"`
			By     string                     `json:"by"`
			ID     string                     `json:"id"`
			Change map[string]json.RawMessage `json:"change"`
		}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			r.skip(r.rel(path), i+1, "JSON の object でない")
			continue
		}
		at, ok := parseAt(v.At)
		if !ok {
			r.skip(r.rel(path), i+1, "at が UTC の時刻（YYYY-MM-DDTHH:MM:SSZ）でない")
			continue
		}
		if v.By == "" {
			r.skip(r.rel(path), i+1, "by が無い（誰の行か分からないので人の介在に数えない）")
			continue
		}
		row := logRow{at: at, by: v.By, id: ledger.CanonID(v.ID), add: v.Change["add"] != nil}
		// 問いの行（{"question": n, "status": …}）の status は問いの状態で、課題の状態ではない
		if s, ok := v.Change["status"]; ok && v.Change["question"] == nil {
			if err := json.Unmarshal(s, &row.status); err != nil {
				r.skip(r.rel(path), i+1, "change.status が文字列でない")
				continue
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func (r *reader) issues() []*issue {
	var out []*issue
	for _, id := range r.subdirs(filepath.Join(r.dir, "issues")) {
		path := filepath.Join(r.dir, "issues", id, "issue.yaml")
		b, ok := r.read(path)
		if !ok {
			continue
		}
		var raw ledger.Issue
		if yaml.Unmarshal(b, &raw) != nil {
			r.skip(r.rel(path), 0, "issue.yaml が YAML でない")
			continue
		}
		raw["id"] = id
		var v struct {
			Created  string   `yaml:"created"`
			After    []string `yaml:"after"`
			Links    []string `yaml:"links"`
			Profile  string   `yaml:"profile"`
			Parent   string   `yaml:"parent"`
			Session  string   `yaml:"session"`
			Worktree string   `yaml:"worktree"`
		}
		if yaml.Unmarshal(b, &v) != nil {
			r.skip(r.rel(path), 0, "issue.yaml が YAML でない")
			continue
		}
		t := &issue{id: id, after: len(v.After) > 0, links: v.Links, profile: v.Profile, parent: ledger.CanonID(v.Parent),
			session: v.Session, worktree: v.Worktree, raw: raw}
		t.created, _ = parseAt(v.Created)
		_, _, isRound := ledger.SplitRound(id)
		t.reviewer = isRound || strings.HasSuffix(v.Profile, "-review")
		lib := filepath.Join(r.dir, "issues", id, "library")
		t.pr, t.agentEst = r.report(lib)
		t.gates = r.gates(lib)
		if t.pr == "" {
			var latest time.Time
			for _, g := range t.gates {
				if !g.at.Before(latest) {
					latest, t.pr = g.at, g.pr
				}
			}
		}
		t.obs = r.observation(lib)
		t.corr = r.corrections(lib)
		t.asked = r.questions(filepath.Join(r.dir, "issues", id, "questions.yaml"))
		out = append(out, t)
	}
	return out
}

// report は report.json の pr と、担当の見込み estimate.size。
func (r *reader) report(lib string) (pr, est string) {
	path := filepath.Join(lib, "report.json")
	b, ok := r.read(path)
	if !ok {
		return "", ""
	}
	var v struct {
		PR       string `json:"pr"`
		Estimate struct {
			Size string `json:"size"`
		} `json:"estimate"`
	}
	if json.Unmarshal(b, &v) != nil {
		r.skip(r.rel(path), 0, "JSON の object でない")
		return "", ""
	}
	return v.PR, v.Estimate.Size
}

// observation は観察の報告の result（hit・miss・unknown）。無ければ空。
func (r *reader) observation(lib string) string {
	path := filepath.Join(lib, "report.json")
	b, ok := r.read(path)
	if !ok {
		return ""
	}
	var v struct {
		Observation *struct {
			Result any `json:"result"`
		} `json:"observation"`
	}
	if json.Unmarshal(b, &v) != nil || v.Observation == nil {
		return ""
	}
	if res, _ := v.Observation.Result.(string); res == "hit" || res == "miss" || res == "unknown" {
		return res
	}
	r.skip(r.rel(path), 0, "observation.result が hit・miss・unknown のどれでもない。この観察は数えていない")
	return ""
}

func (r *reader) gates(lib string) []gateRec {
	var out []gateRec
	for _, path := range r.files(filepath.Join(lib, "gate"), ".json") {
		b, ok := r.read(path)
		if !ok {
			continue
		}
		var v struct {
			At       string `json:"at"`
			PR       string `json:"pr"`
			Tier     string `json:"tier"`
			Decision string `json:"decision"`
			Merged   bool   `json:"merged"`
		}
		if json.Unmarshal(b, &v) != nil {
			r.skip(r.rel(path), 0, "ゲートの記録が JSON の object でない")
			continue
		}
		at, ok := parseAt(v.At)
		if !ok || v.Decision == "" {
			r.skip(r.rel(path), 0, "ゲートの記録に at か decision が無い")
			continue
		}
		switch v.Tier {
		case "A", "B", "C":
		default:
			v.Tier = "none"
		}
		out = append(out, gateRec{at: at, pr: v.PR, tier: v.Tier, decision: v.Decision, merged: v.Merged})
	}
	return out
}

// corrections は corrections.json（contract.md §14）の corrections に1件以上あるか。形が違えば飛ばした記録にする。
func (r *reader) corrections(lib string) bool {
	path := filepath.Join(lib, "corrections.json")
	b, ok := r.read(path)
	if !ok {
		return false
	}
	var v struct {
		Corrections *[]json.RawMessage `json:"corrections"`
	}
	if json.Unmarshal(b, &v) != nil || v.Corrections == nil {
		r.skip(r.rel(path), 0, "corrections の配列を持つ JSON でない")
		return false
	}
	return len(*v.Corrections) > 0
}

func (r *reader) hasCorrections(t *issue) bool {
	_, err := os.Stat(filepath.Join(r.dir, "issues", t.id, "library", "corrections.json"))
	return err == nil
}

func (r *reader) questions(path string) []span {
	b, ok := r.read(path)
	if !ok {
		return nil
	}
	var v []struct {
		N        int    `yaml:"n"`
		Asked    string `yaml:"asked"`
		Answered string `yaml:"answered"`
	}
	if yaml.Unmarshal(b, &v) != nil {
		r.skip(r.rel(path), 0, "questions.yaml を読めない")
		return nil
	}
	var out []span
	for _, q := range v {
		from, ok := parseAt(q.Asked)
		if !ok {
			r.skip(r.rel(path), 0, "問い %d の asked が UTC の時刻でない", q.N)
			continue
		}
		var to time.Time
		if q.Answered != "" {
			to, ok = parseAt(q.Answered)
			if !ok {
				r.skip(r.rel(path), 0, "問い %d の answered が UTC の時刻でない", q.N)
				continue
			}
		}
		out = append(out, span{from, to})
	}
	return out
}

func (r *reader) github(gh func(string) ([]byte, error), repo string) ([]ghPR, error) {
	b, err := gh(repo)
	if err != nil {
		return nil, err
	}
	var list []ghPR
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, errors.New("gh の出力が PR の配列でない")
	}
	return list, nil
}

// news は記録だけの通知と届けた通知の記録から、期間の中（行の at）の kind ごとの数を数える。
func (r *reader) news(o Options, in func(time.Time) bool) News {
	by := map[string]*NewsKind{}
	get := func(kind string) *NewsKind {
		if by[kind] == nil {
			by[kind] = &NewsKind{Kind: kind}
		}
		return by[kind]
	}
	r.jsonl(o.Recorded, func(src string, line int, b []byte) {
		var v struct {
			At   string `json:"at"`
			Kind string `json:"kind"`
		}
		if json.Unmarshal(b, &v) != nil || v.Kind == "" {
			r.skip(src, line, "kind を持つ JSON の object でない")
			return
		}
		at, ok := parseAt(v.At)
		if !ok {
			r.skip(src, line, "at が UTC の時刻（YYYY-MM-DDTHH:MM:SSZ）でない")
			return
		}
		if in(at) {
			get(v.Kind).Recorded++
		}
	})
	r.jsonl(o.Delivered, func(src string, line int, b []byte) {
		var v struct {
			At    string `json:"at"`
			Acked *bool  `json:"acked"`
			News  []struct {
				Kind string `json:"kind"`
			} `json:"news"`
		}
		if json.Unmarshal(b, &v) != nil || v.Acked == nil {
			r.skip(src, line, "acked を持つ JSON の object でない")
			return
		}
		at, ok := parseAt(v.At)
		if !ok {
			r.skip(src, line, "at が UTC の時刻（YYYY-MM-DDTHH:MM:SSZ）でない")
			return
		}
		if !in(at) {
			return
		}
		for _, n := range v.News {
			if n.Kind == "" {
				r.skip(src, line, "news に kind の無い通知がある。その1件は数えていない")
				continue
			}
			k := get(n.Kind)
			k.Delivered++
			if *v.Acked {
				k.Acked++
			}
		}
	})
	out := News{Threshold: passThrough, Kinds: []NewsKind{}, Candidates: []string{}}
	names := make([]string, 0, len(by))
	for k := range by {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		k := *by[name]
		k.Rate = ratio(k.Acked, k.Delivered)
		out.Kinds = append(out.Kinds, k)
		if k.Delivered > 0 && float64(k.Acked) >= passThrough*float64(k.Delivered) {
			out.Candidates = append(out.Candidates, name)
		}
	}
	return out
}

// jsonl は JSON Lines のファイルを1行ずつ f に渡す（空の行は飛ばす）。パスが空・ファイルが無ければ何もしない（まだ無い記録）。
// 飛ばした記録の source は dispatcher の置き場からの名前（絶対パスを書かない）。
func (r *reader) jsonl(path string, f func(src string, line int, b []byte)) {
	if path == "" {
		return
	}
	src := "dispatcher/" + filepath.Base(path)
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			why := "読み取りの誤り"
			var pe *fs.PathError
			if errors.As(err, &pe) {
				why = pe.Err.Error()
			}
			r.skip(src, 0, "読めない（%s）", why)
		}
		return
	}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			f(src, i+1, []byte(line))
		}
	}
}

// GH は gh pr list でマージされた PR を読む。
func GH(repo string) ([]byte, error) {
	out, err := exec.Command("gh", "pr", "list", "--repo", repo, "--state", "merged", "--limit", "1000",
		"--json", "number,url,title,body,mergedAt").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := strings.TrimSpace(string(ee.Stderr))
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				msg = msg[:i]
			}
			return nil, fmt.Errorf("gh pr list: %s", msg)
		}
		return nil, fmt.Errorf("gh pr list: %v", err)
	}
	return out, nil
}

// Main は andon measure。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return 2 }
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var since, until time.Time
	asJSON := false
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			asJSON = true
			args = args[1:]
		case "--since", "--until":
			if len(args) < 2 {
				return usage()
			}
			d, err := time.Parse("2006-01-02", args[1])
			if err != nil {
				fmt.Fprintf(stderr, "andon measure: %s は YYYY-MM-DD の日付（%q）\n", args[0], args[1])
				return 2
			}
			if args[0] == "--since" {
				since = d
			} else {
				until = d
			}
			args = args[2:]
		default:
			return usage()
		}
	}
	if until.IsZero() {
		until = today
	}
	if since.IsZero() {
		since = until.AddDate(0, 0, -6)
	}
	if since.After(until) {
		fmt.Fprintln(stderr, "andon measure: --since が --until より後")
		return 2
	}
	root := os.Getenv("KANBAN_ROOT")
	if root == "" {
		l, err := ledger.Open()
		if err != nil {
			fmt.Fprintf(stderr, "andon measure: %v\n", err)
			return 1
		}
		root = l.Root
	}
	slug := filepath.Base(root)
	res, err := Measure(Options{Root: root, Since: since, Until: until, Now: now, GH: GH,
		State: dispatcher.StatePath(slug), Recorded: dispatcher.RecordedPath(slug), Delivered: dispatcher.DeliveredPath(slug),
		ClaudeDir: ClaudeDir(), CodexSessions: CodexSessions()})
	if err != nil {
		fmt.Fprintf(stderr, "andon measure: %v\n", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return 1
		}
		return 0
	}
	Table(stdout, res, columnsOrder(res))
	return 0
}

func columnsOrder(res *Result) []string {
	order := []string{"todo", "queued", "doing", "review"}
	var out []string
	for _, s := range order {
		if _, ok := res.Flow.Wait[s]; ok {
			out = append(out, s)
		}
	}
	var rest []string
	for s := range res.Flow.Wait {
		if !slices.Contains(order, s) {
			rest = append(rest, s)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

const none = "記録なし"

func hours(s Stat) string {
	if s.Value == nil {
		return none
	}
	return fmt.Sprintf("%.1f 時間（%d件）", *s.Value, s.Count)
}

func pct(r *float64, n, d int) string {
	if r == nil {
		return none
	}
	return fmt.Sprintf("%.0f%%（%d / %d）", *r*100, n, d)
}

// Table は人向けの表。価値 → 流れ → 効率 → トークン → 通知の順で、自動マージ率の行に対抗の指標を並べる。
func Table(w io.Writer, res *Result, states []string) {
	val := res.Value
	obs := none
	if val.Observed.Count > 0 {
		obs = fmt.Sprintf("%d件（hit %d・miss %d・unknown %d）", val.Observed.Count, val.Observed.Hit, val.Observed.Miss, val.Observed.Unknown)
	}
	fmt.Fprintf(w, "価値: 仮説を確かめた数 %s\n", obs)
	fmt.Fprintf(w, "プロジェクト %s・期間 %s〜%s（%d日。UTC）\n", res.Project, res.Since, res.Until, res.Days)
	fmt.Fprintln(w, "\n価値")
	seen := none
	if val.DaysToObserve.Days != nil {
		seen = fmt.Sprintf("%.2f日（中央値。%d件）", *val.DaysToObserve.Days, val.DaysToObserve.Count)
	}
	fmt.Fprintf(w, "  題から結果を見るまで           %s\n", seen)
	fmt.Fprintf(w, "  価値の仮説「無し」の題         %d\n", val.NoHypothesis)
	f, e, c := res.Flow, res.Efficiency, res.Counter
	fmt.Fprintln(w, "\n流れ")
	fmt.Fprintf(w, "  依頼書からマージまで（中央値）  %s\n", hours(f.LeadTime))
	var waits []string
	for _, s := range states {
		waits = append(waits, s+" "+hours(f.Wait[s]))
	}
	fmt.Fprintf(w, "  待ち時間（1件あたり平均）      %s\n", strings.Join(waits, "・"))
	fmt.Fprintf(w, "  問いが open の間（1問あたり）  %s\n", hours(f.QuestionOpen))
	closed := none
	if f.Closed.Count > 0 {
		closed = fmt.Sprintf("%.2f（%d件 / %d日）", f.Closed.PerDay, f.Closed.Count, res.Days)
	}
	fmt.Fprintf(w, "  1日に閉じた数                  %s\n", closed)
	fmt.Fprintf(w, "  期間の終わりの WIP             %d\n", f.WIPEnd)
	fmt.Fprintln(w, "\n効率")
	touches := none
	if e.HumanTouches.PerPR != nil {
		touches = fmt.Sprintf("%.2f（%d回 / %d本）", *e.HumanTouches.PerPR, e.HumanTouches.Total, e.MergedPRs)
	}
	fmt.Fprintf(w, "  PR あたりの人の介在回数        %s\n", touches)
	corr := "corrections あり"
	if c.Reopened.Corrections == "none" {
		corr = "corrections " + none
	}
	t := c.Reopened.ByTier
	fmt.Fprintf(w, "  自動マージ率                   %s ｜ 対抗: revert %d本・同じ箇所の課題 %d件・人が開け直した A %d・B %d・C %d・レベル無し %d（%s）\n",
		pct(e.AutoMerge.Rate, e.AutoMerge.Count, e.MergedPRs), c.Reverted, c.FollowUps, t["A"], t["B"], t["C"], t["none"], corr)
	k := c.Corrected.ByTier
	fmt.Fprintf(w, "  人の手直しがあった課題         A %d・B %d・C %d・レベル無し %d（%s）\n", k["A"], k["B"], k["C"], k["none"], corr)
	gate := none
	if e.Gate.Rate != nil {
		gate = fmt.Sprintf("%.0f%%（fix %d・needs_human %d / %d）", *e.Gate.Rate*100, e.Gate.Fix, e.Gate.NeedsHuman, e.Gate.Records)
	}
	fmt.Fprintf(w, "  ゲートの差し戻し率             %s\n", gate)
	estimateTable(w, res.Estimate, res.Issues)
	tokensTable(w, res.Tokens)
	newsTable(w, res.News)
	if len(res.Skipped) == 0 {
		return
	}
	fmt.Fprintf(w, "\n飛ばした記録: %d件（数えていない）\n", len(res.Skipped))
	for _, s := range res.Skipped {
		loc := s.Source
		if s.Line > 0 {
			loc = fmt.Sprintf("%s:%d", s.Source, s.Line)
		}
		fmt.Fprintf(w, "  %s  %s\n", loc, s.Reason)
	}
}

// newsTable は通知の節（kind ごとの素通りと、末尾に記録だけへ移す候補）。
func newsTable(w io.Writer, n News) {
	fmt.Fprintln(w, "\n通知（Chief of Staff へ届けた数と、andon news ack で終わった素通り）")
	if len(n.Kinds) == 0 {
		fmt.Fprintf(w, "  %s\n", none)
	}
	for _, k := range n.Kinds {
		fmt.Fprintf(w, "  %-18s 届けた %d・素通り %s・記録だけ %d\n", k.Kind, k.Delivered, pct(k.Rate, k.Acked, k.Delivered), k.Recorded)
	}
	cand := "なし"
	if len(n.Candidates) > 0 {
		cand = strings.Join(n.Candidates, "・")
	}
	fmt.Fprintf(w, "  記録だけへ移す候補（%.0f%%以上が素通り。移すかは人が決める）  %s\n", n.Threshold*100, cand)
}

// tokensTable はトークンの節（期間に動いた課題ごと。記録が無い課題は「記録なし」で、0 と分ける）。
func tokensTable(w io.Writer, ts []Tokens) {
	fmt.Fprintln(w, "\nトークン（期間に動いた課題ごと。課題の記録の全体。入力・出力・キャッシュ読み・キャッシュ書き）")
	if len(ts) == 0 {
		fmt.Fprintf(w, "  %s\n", none)
	}
	for _, t := range ts {
		u := none
		if t.Usage != nil {
			u = fmt.Sprintf("入力 %d・出力 %d・キャッシュ読み %d・キャッシュ書き %d", t.Usage.Input, t.Usage.Output, t.Usage.CacheRead, t.Usage.CacheWrite)
		}
		fmt.Fprintf(w, "  %-12s %-6s %s\n", t.Issue, t.Source, u)
	}
}
