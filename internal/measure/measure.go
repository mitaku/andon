// Package measure は kanban と Andon の記録からラインの流れを数える（andon metrics show。
// docs/decisions/2026-11-09-measure-flow-metrics.md・docs/contract.md §13）。
//
// 読むだけ。kanban にも library/ にも書かず、集計を保存しない。読むもの: log.jsonl・issues/<id>/issue.yaml・
// questions.yaml・library/report.json・library/gate/<sha>.json・library/corrections.json と、
// GitHub のマージされた PR の一覧（gh pr list。repo ごとに1回）と、dispatcher の記録だけの通知
// （<slug>.recorded.jsonl）・届けた通知の記録（<slug>.delivered.jsonl）と、担当とレビュアーのセッションの記録
// （Claude Code・Codex。トークン数だけ。tokens.go）と、片付いた課題の outcome.json の breakdown と、ハーネスの Chief of Staff の
// 役ごとの生の記録（内訳の節とプロファイルごとの節。breakdown.go）。Markdown は読まない。
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

	"andon/internal/contract"
	"andon/internal/dispatcher"
	"andon/internal/ghpr"
	"andon/internal/ledger"
	"andon/internal/otel"
	"andon/internal/trace"
	"andon/internal/waiting"
)

const Usage = "usage: andon metrics show [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]\n" +
	"kanban（KANBAN_ROOT か projects/<slug>/）の記録から、期間の流れ（待ち時間・流量・WIP）と効率（介在回数・自動マージ率・\n" +
	"gate の差し戻し率）と対抗の指標（revert・同じ箇所の課題・オーナーが開け直した数）と、通知の kind ごとの素通りの数と、\n" +
	"期間に動いた課題ごとのトークン数（Claude Code・Codex のセッションの記録から）と、片付いた課題の内訳（outcome.json の breakdown を\n" +
	"役ごとに合わせたものと、Chief of Staff の分〈ハーネスの生の記録から〉）と、プロファイルごとの数と、知識の節（依頼書に wiki の\n" +
	"ページを載せた課題・ページを読んだ課題・知識の欠けの申告）を数える。\n" +
	"読むだけで、何も書かない。\n" +
	"期間は UTC の日付で両端を含む。既定は --until が今日、--since がその6日前（7日間）。--json は docs/contract.md §13 の形。\n"

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
	// Otel はハーネスの記録のプロジェクトの置き場（~/.local/state/andon/otel/<slug>。docs/contract.md §30）。
	// Chief of Staff の役ごとの生の記録だけを読む。空なら読まない。
	Otel string
	// Wiki はセルの wiki の置き場（trace.WikiDir）。知識の節の、依頼書に載せたページと読まれなかったページに使う。
	// 空なら依頼書は wiki/ から始まる相対パスだけを見て、読まれなかったページは数えない（null）。
	Wiki string
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

// Result は andon metrics show --json の出力（schema/measure.schema.json）。
type Result struct {
	Project    string           `json:"project"`
	Since      string           `json:"since"`
	Until      string           `json:"until"`
	Days       int              `json:"days"`
	Value      Value            `json:"value"`
	Flow       Flow             `json:"flow"`
	Efficiency Efficiency       `json:"efficiency"`
	Counter    Counter          `json:"counter"`
	News       News             `json:"news"`
	PRs        []PR             `json:"prs"`
	Estimate   Estimate         `json:"estimate"`
	Issues     []Issue          `json:"issues"`
	Tokens     []Tokens         `json:"tokens"`
	Breakdown  BreakdownSection `json:"breakdown"`
	Profiles   []Profile        `json:"profiles"`
	Knowledge  KnowledgeSection `json:"knowledge"`
	Skipped    []Skip           `json:"skipped"`
}

// Value は価値（ADR-0011 決めたこと 18）。観察の課題の報告 `observation` と Chief Engineer の課題から導く。
type Value struct {
	Observed      Observed `json:"observed"`        // (a) 期間に報告された観察の結果の内訳
	DaysToObserve DaysStat `json:"days_to_observe"` // (b) Chief Engineer の課題の done から観察の報告まで（中央値）
	NoHypothesis  int      `json:"no_hypothesis"`   // (c) 期間に done になった Chief Engineer の課題のうち、観察の課題が無いもの
}

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

type Closed struct {
	Count  int     `json:"count"`
	PerDay float64 `json:"per_day"`
}

type Efficiency struct {
	MergedPRs    int       `json:"merged_prs"`
	HumanTouches Touches   `json:"human_touches"`
	Answers      Answers   `json:"answers"`
	AutoMerge    Rate      `json:"auto_merge"`
	Gate         GateStats `json:"gate"`
}

// Touches はオーナーの介在回数。期間に起きた操作の数で、Total は3種の和（ADR-0049 決めたこと 2・11）。
type Touches struct {
	Total    int      `json:"total"`
	PerPR    *float64 `json:"per_pr"`
	Merge    int      `json:"merge"`    // gate の記録にマージの印が無い、マージされた PR（決めたこと 3）
	Decision int      `json:"decision"` // 印が owner の答え（決めたこと 7）
	Task     int      `json:"task"`     // 問いを持たずに片付いたオーナーの課題（決めたこと 9）
}

// Answers は期間に書かれた答えを、決めた者（questions.yaml の decidedBy）ごとに数えたもの（ADR-0049 決めたこと 8）。
type Answers struct {
	Owner        int `json:"owner"`
	ChiefOfStaff int `json:"chief_of_staff"`
	Machine      int `json:"machine"`
	Unmarked     int `json:"unmarked"` // 印が無いか3つの値のどれでもない。介在回数に入れない
}

const (
	decidedOwner        = "owner"
	decidedChiefOfStaff = "chief-of-staff"
	decidedMachine      = "machine"
)

// GateStats は gate の記録の数と差し戻し率（(fix + needs_human) ÷ 記録）。
type GateStats struct {
	Records    int      `json:"records"`
	Fix        int      `json:"fix"`
	NeedsHuman int      `json:"needs_human"`
	Rate       *float64 `json:"rate"`
}

type Counter struct {
	Reverted  int      `json:"reverted"`
	FollowUps int      `json:"follow_ups"`
	Reopened  Reopened `json:"reopened"`
	Corrected Reopened `json:"corrected"` // オーナーの手直しがあった課題の数（revert を含めない。gate のレベルごと）
}

type Reopened struct {
	Corrections string         `json:"corrections"` // recorded: corrections.json が1つでもある。none: 1つも無い（記録なし）
	ByTier      map[string]int `json:"by_tier"`
}

// News は Chief of Staff へ届けた通知の素通り（kind ごと。docs/decisions/2026-11-12-measure-news-pass-through.md）。
type News struct {
	Threshold  float64    `json:"threshold"`  // 素通りの割合がこれ以上の kind を候補にする
	Kinds      []NewsKind `json:"kinds"`      // kind の名前順
	Candidates []string   `json:"candidates"` // 記録だけへ移す候補（移すかはオーナーが決める）
}

type NewsKind struct {
	Kind      string   `json:"kind"`
	Delivered int      `json:"delivered"` // 届けて、渡したターンの終わりを見た数
	Acked     int      `json:"acked"`     // そのうち andon news ack で終わったターンの数（素通り）
	Rate      *float64 `json:"rate"`      // acked ÷ delivered（delivered が 0 なら null）
	Recorded  int      `json:"recorded"`  // 記録だけにして届けなかった数
}

const passThrough = 0.8

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

type Skip struct {
	Source string `json:"source"`
	Line   int    `json:"line,omitempty"`
	Reason string `json:"reason"`
}

const stamp = "2006-01-02T15:04:05Z"

var prRe = regexp.MustCompile(`^https://github\.com/([!-.0-~]+)/([!-.0-~]+)/pull/([1-9][0-9]*)$`)

type logRow struct {
	at     time.Time
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
	asked    []question
	hasQs    bool // 問いを持つか、持たないと言えない（questions.yaml を読めない）
	profile  string
	parent   string
	obs      string // report.json の observation.result（観察の報告が無ければ空）
	session  string
	worktree string
	agentEst string // report.json の estimate.size（担当の見込み）
	gaps     []gap  // report.json の gaps[]（種類の壊れた申告は除く。知識の節）
	raw      ledger.Issue
}

type question struct {
	from, to  time.Time // to（答えた時刻）がゼロなら開いたまま
	decidedBy string    // 印が無いか文字列でなければ空
}

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
	cos     string       // config.yaml の chief_of_staff
	ledger  []ledger.Row // log.jsonl の読めた行（印 later を信じるかの照合に使う）
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

// Measure は数える。kanban の config.yaml が読めない・バージョンを知らないときだけ誤りを返す。
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

	// GitHub のマージされた PR（kanban の PR がある repo だけ）
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

	// revert された kanban の PR（revert の PR が期間にマージされたもの）
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
		if !x.Auto {
			x.HumanTouches++
		}
		for _, q := range t.asked {
			if !q.to.IsZero() && q.decidedBy == decidedOwner {
				x.HumanTouches++ // PR の行は答えた時期を問わない（ADR-0049 決めたこと 11）
			}
		}
		if born := t.born(); !born.IsZero() {
			h := round(at.Sub(born).Hours())
			x.LeadTime = &h
			leads = append(leads, h)
		}
		res.PRs = append(res.PRs, x)
	}

	// 期間にほかの列から最後の列へ動いた課題（flow.closed と問いの無いオーナーの課題）。kanban done の打ち直しも
	// 同じ形の行を足すので、前の列が最後の列の行は動いたうちに入れない
	movedIn := map[string]bool{}
	for id, at := range arrivals(rows, last) {
		for _, a := range at {
			if in(a) {
				movedIn[id] = true
			}
		}
	}

	// 効率。介在は操作ごとの時刻で期間に入れる（PR の行の和ではない。ADR-0049 決めたこと 11）
	e := &res.Efficiency
	e.MergedPRs = len(res.PRs)
	touches := &e.HumanTouches
	for _, x := range res.PRs {
		if x.Auto {
			e.AutoMerge.Count++
		} else {
			touches.Merge++
		}
	}
	for _, id := range ids {
		t := byID[id]
		if t.reviewer {
			continue
		}
		if movedIn[id] && !t.hasQs && t.raw.Str("who") == "person" {
			touches.Task++
		}
		for _, q := range t.asked {
			if !q.to.IsZero() && in(q.to) {
				e.Answers.add(q.decidedBy)
			}
		}
	}
	touches.Decision = e.Answers.Owner
	touches.Total = touches.Merge + touches.Decision + touches.Task
	touches.PerPR = ratio(touches.Total, e.MergedPRs)
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
		}
		account(end)
		if movedIn[id] {
			closed++
		}
		if !since.IsZero() && cur != first && cur != "queued" && cur != last {
			f.WIPEnd++
		}
		// 置いておける課題の問いは入れない（ADR-0062 決めたこと 9）。数えるときの印だけを見るので、途中で印を外した問いは置いてあった間も入る
		if waiting.Placeable(t.raw, r.ledger, r.cos) {
			continue
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
	for id, at := range arrivals(rows, last) {
		doneAt[id] = at[len(at)-1] // 最後に done へ動いた時刻（打ち直しの行は動いたうちに入れない）
	}
	for _, row := range rows {
		if row.status == "" {
			continue
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
	mergedLedger := map[string]string{} // マージされた kanban の PR の URL → 課題
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

	res.Estimate, res.Issues = r.estimates(o, last, ids, byID, doneAt, in)
	res.News = r.news(o, in)
	res.Tokens = r.tokens(o, ids, byID, active(rows, in))
	bs := r.outcomeBreakdowns(ids, byID, doneAt, in)
	cos, cosDays := trace.RoleTokens(o.Otel, "chief-of-staff", start, o.Until, func(file string, line int, reason string) {
		r.skip("otel/"+file, line, "%s", reason)
	})
	res.Breakdown = breakdownSection(bs, cos, cosDays)
	res.Profiles = profiles(res.Issues, byID, ids, bs)
	workers := make([]string, 0, len(res.Issues))
	for _, x := range res.Issues {
		workers = append(workers, x.Issue)
	}
	res.Knowledge = r.knowledge(o, workers, byID, bs)

	sort.SliceStable(r.skipped, func(i, j int) bool {
		if r.skipped[i].Source != r.skipped[j].Source {
			return r.skipped[i].Source < r.skipped[j].Source
		}
		return r.skipped[i].Line < r.skipped[j].Line
	})
	res.Skipped = append([]Skip{}, r.skipped...)
	return res, nil
}

// arrivals は課題ごとに、ほかの列から status の列へ動いた時刻（履歴の順）。前の列が同じ列の行（kanban done の打ち直し）は
// 動いたうちに入れない。履歴に前の列が無い課題の最初の行は動いたうちに入れる。
func arrivals(rows []logRow, status string) map[string][]time.Time {
	out := map[string][]time.Time{}
	column := map[string]string{}
	for _, row := range rows {
		if row.status == "" {
			continue
		}
		if row.status == status && column[row.id] != status {
			out[row.id] = append(out[row.id], row.at)
		}
		column[row.id] = row.status
	}
	return out
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

func (a *Answers) add(decidedBy string) {
	switch decidedBy {
	case decidedOwner:
		a.Owner++
	case decidedChiefOfStaff:
		a.ChiefOfStaff++
	case decidedMachine:
		a.Machine++
	default:
		a.Unmarked++
	}
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

// config は config.yaml の columns（無ければ既定）。バージョンが 3 でなければ止まる（kanban の形式の規則）。
func (r *reader) config() ([]string, error) {
	c, err := ledger.ReadConfig(r.dir)
	r.cos = c.ChiefOfStaff
	var pathErr *fs.PathError
	switch {
	case errors.As(err, &pathErr):
		return nil, fmt.Errorf("kanban の config.yaml を読めない（%s）", r.rel(filepath.Join(r.dir, "config.yaml")))
	case err != nil:
		return nil, errors.New("kanban の config.yaml が YAML でない")
	case !c.VersionIs(3):
		var v any
		if c.HasVersion {
			v = c.Version
		}
		return nil, fmt.Errorf("kanban のバージョンが 3 でない（%v）。知らないバージョンは読まない", v)
	}
	if len(c.Columns) == 0 {
		return []string{"todo", "queued", "doing", "review", "done"}, nil
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
			r.skip(r.rel(path), i+1, "by が無い（誰の行か分からない）")
			continue
		}
		row := logRow{at: at, id: ledger.CanonID(v.ID), add: v.Change["add"] != nil}
		// 問いの行（{"question": n, "status": …}）の status は問いの状態で、課題の状態ではない
		if s, ok := v.Change["status"]; ok && v.Change["question"] == nil {
			if err := json.Unmarshal(s, &row.status); err != nil {
				r.skip(r.rel(path), i+1, "change.status が文字列でない")
				continue
			}
		}
		rows = append(rows, row)
		var lr ledger.Row
		if json.Unmarshal([]byte(line), &lr) == nil {
			lr.ID = ledger.CanonID(lr.ID)
			r.ledger = append(r.ledger, lr)
		}
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
		var doc yaml.Node
		var raw ledger.Issue
		if yaml.Unmarshal(b, &doc) != nil || doc.Decode(&raw) != nil {
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
		if doc.Decode(&v) != nil {
			r.skip(r.rel(path), 0, "issue.yaml が YAML でない")
			continue
		}
		t := &issue{id: id, after: len(v.After) > 0, links: v.Links, profile: v.Profile, parent: ledger.CanonID(v.Parent),
			session: v.Session, worktree: v.Worktree, raw: raw}
		t.created, _ = parseAt(v.Created)
		_, _, isRound := ledger.SplitRound(id)
		t.reviewer = isRound || strings.HasSuffix(v.Profile, "-review")
		lib := filepath.Join(r.dir, "issues", id, "library")
		t.pr, t.agentEst, t.obs, t.gaps = r.report(lib)
		t.gates = r.gates(lib)
		if t.pr == "" {
			var latest time.Time
			for _, g := range t.gates {
				if !g.at.Before(latest) {
					latest, t.pr = g.at, g.pr
				}
			}
		}
		t.corr = r.corrections(lib)
		t.asked, t.hasQs = r.questions(filepath.Join(r.dir, "issues", id, "questions.yaml"))
		out = append(out, t)
	}
	return out
}

func (r *reader) report(lib string) (pr, est, obs string, gaps []gap) {
	path := filepath.Join(lib, "report.json")
	b, ok := r.read(path)
	if !ok {
		return "", "", "", nil
	}
	var v struct {
		PR       string `json:"pr"`
		Estimate struct {
			Size string `json:"size"`
		} `json:"estimate"`
	}
	bad := json.Unmarshal(b, &v) != nil
	if bad {
		r.skip(r.rel(path), 0, "JSON の object でない")
	}
	obs = r.observation(path, b)
	if bad {
		return "", "", obs, nil
	}
	return v.PR, v.Estimate.Size, obs, r.gaps(path, b)
}

// observation は report.json（path の中身 b）の観察の報告の result（hit・miss・unknown）。無ければ空。
func (r *reader) observation(path string, b []byte) string {
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
			r.skip(r.rel(path), 0, "gate の記録が JSON の object でない")
			continue
		}
		at, ok := parseAt(v.At)
		if !ok || v.Decision == "" {
			r.skip(r.rel(path), 0, "gate の記録に at か decision が無い")
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

// questions は読めた問いと、課題が問いを持つか。在るのに読めなければ持つ側に倒す（問いの無いオーナーの課題に数えない）。
func (r *reader) questions(path string) (out []question, has bool) {
	b, ok := r.read(path)
	if !ok {
		_, err := os.Stat(path)
		return nil, !errors.Is(err, os.ErrNotExist)
	}
	var v []struct {
		N         int    `yaml:"n"`
		Asked     string `yaml:"asked"`
		Answered  string `yaml:"answered"`
		Status    any    `yaml:"status"`
		DecidedBy any    `yaml:"decidedBy"` // 文字列でない値でもファイルを丸ごと落とさない
	}
	if yaml.Unmarshal(b, &v) != nil {
		r.skip(r.rel(path), 0, "questions.yaml を読めない")
		return nil, true
	}
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
		} else if q.Status == "answered" || q.Status == "received" {
			r.skip(r.rel(path), 0, "問い %d は答え済みなのに answered が無い", q.N)
			continue
		}
		by, _ := q.DecidedBy.(string)
		out = append(out, question{from: from, to: to, decidedBy: by})
	}
	return out, len(v) > 0
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
		if k.Rate != nil && *k.Rate >= passThrough { // 表に出す丸めた rate で決める（contract.md §13）
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

// GH は gh pr list でマージされた PR を全部読む（--limit で切らない。ghpr.Merged）。
func GH(repo string) ([]byte, error) {
	return ghpr.Merged(runGH, repo, "number,url,title,body,mergedAt")
}

// runGH は gh を打つ。誤りには stderr の最初の行を添える。
func runGH(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
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
				fmt.Fprintf(stderr, "andon metrics show: %s は YYYY-MM-DD の日付（%q）\n", args[0], args[1])
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
		fmt.Fprintln(stderr, "andon metrics show: --since が --until より後")
		return 2
	}
	root := os.Getenv("KANBAN_ROOT")
	if root == "" {
		l, err := ledger.Open()
		if err != nil {
			fmt.Fprintf(stderr, "andon metrics show: %v\n", err)
			return 1
		}
		root = l.Root
	}
	slug := filepath.Base(root)
	res, err := Measure(Options{Root: root, Since: since, Until: until, Now: now, GH: GH,
		State: dispatcher.StatePath(slug), Recorded: dispatcher.RecordedPath(slug), Delivered: dispatcher.DeliveredPath(slug),
		ClaudeDir: ClaudeDir(), CodexSessions: CodexSessions(), Otel: filepath.Join(otel.Root(), slug),
		Wiki: trace.WikiDir()})
	if err != nil {
		fmt.Fprintf(stderr, "andon metrics show: %v\n", err)
		return 1
	}
	if asJSON {
		b, err := contract.Encode(contract.SchemaMeasure, res)
		if err != nil {
			fmt.Fprintf(stderr, "andon metrics show: %v\n", err)
			return 1
		}
		stdout.Write(b)
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
	fmt.Fprintf(w, "  テーマから結果を見るまで       %s\n", seen)
	fmt.Fprintf(w, "  価値の仮説「無し」のテーマ     %d\n", val.NoHypothesis)
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
	h, touches := e.HumanTouches, none
	if h.PerPR != nil {
		touches = fmt.Sprintf("%.2f", *h.PerPR)
	}
	if h.PerPR != nil || h.Total > 0 { // マージされた PR が無くても、介在があれば数を出す
		touches += fmt.Sprintf("（%d回 / %d本。マージ %d・判断 %d・オーナーの課題 %d）", h.Total, e.MergedPRs, h.Merge, h.Decision, h.Task)
	}
	fmt.Fprintf(w, "  PR あたりのオーナーの介在回数        %s\n", touches)
	answers := none
	if a := e.Answers; a != (Answers{}) {
		answers = fmt.Sprintf("オーナー %d・Chief of Staff %d・command %d・印なし %d", a.Owner, a.ChiefOfStaff, a.Machine, a.Unmarked)
	}
	fmt.Fprintf(w, "  答えを決めた者                 %s\n", answers)
	corr := "corrections あり"
	if c.Reopened.Corrections == "none" {
		corr = "corrections " + none
	}
	t := c.Reopened.ByTier
	fmt.Fprintf(w, "  自動マージ率                   %s ｜ 対抗: revert %d本・同じ箇所の課題 %d件・オーナーが開け直した A %d・B %d・C %d・レベル無し %d（%s）\n",
		pct(e.AutoMerge.Rate, e.AutoMerge.Count, e.MergedPRs), c.Reverted, c.FollowUps, t["A"], t["B"], t["C"], t["none"], corr)
	k := c.Corrected.ByTier
	fmt.Fprintf(w, "  オーナーの手直しがあった課題         A %d・B %d・C %d・レベル無し %d（%s）\n", k["A"], k["B"], k["C"], k["none"], corr)
	gate := none
	if e.Gate.Rate != nil {
		gate = fmt.Sprintf("%.0f%%（fix %d・needs_human %d / %d）", *e.Gate.Rate*100, e.Gate.Fix, e.Gate.NeedsHuman, e.Gate.Records)
	}
	fmt.Fprintf(w, "  gate の差し戻し率             %s\n", gate)
	estimateTable(w, res.Estimate, res.Issues)
	tokensTable(w, res.Tokens)
	breakdownTable(w, res.Breakdown)
	profilesTable(w, res.Profiles)
	knowledgeTable(w, res.Knowledge)
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
	fmt.Fprintf(w, "  記録だけへ移す候補（%.0f%%以上が素通り。移すかはオーナーが決める）  %s\n", n.Threshold*100, cand)
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
