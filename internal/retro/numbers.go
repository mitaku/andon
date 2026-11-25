package retro

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andon/internal/ledger"
	"andon/internal/trace"
)

// 振り返りの入力の内訳と、直しのマージの前後の数字（ADR-0060 決めたこと 17。docs/contract.md §20）。
// 課題の内訳は、dispatcher が片付けのときに書いた outcome.json の breakdown を読む（生の記録は読まない。決めたこと 15）。
// Chief of Staff には outcome.json が無いので、その分だけは役ごとの生の記録から数える（andon measure と同じ数え方）。

const (
	sideMax = 30 // 前と後のそれぞれで、マージの時刻に近いほうから数える課題の数
	sideMin = 10 // 前か後がこれに満たなければ、数字を出さない
)

// BeforeAfter は直しのマージの前と後の数字。Short が nil でないときは、前か後が sideMin 件に満たず、どちらの数字も出さない。
type BeforeAfter struct {
	Before Side    `json:"before"`
	After  Side    `json:"after"`
	Short  *string `json:"short"`
}

// Side はマージの前か後に片付いた担当の課題（近いほうから sideMax 件まで）の数と、その数字（出さないときは nil）。
type Side struct {
	Issues  int      `json:"issues"`
	Numbers *Numbers `json:"numbers"`
}

// Numbers は andon measure のプロファイルごとの節と同じ数え方の数字。
type Numbers struct {
	WorkedMinutes Median    `json:"worked_minutes"` // outcome.json の worked_minutes（無い課題は数えない）
	Tokens        Median    `json:"tokens"`         // 内訳の tokens の 4 つの和（内訳の無い課題と、モデルの呼び出しの記録の無い内訳は数えない）
	Gate          GateStats `json:"gate"`           // その課題の gate の記録の全部
	ReviewRounds  Median    `json:"review_rounds"`  // その課題を親に持つレビュアーの課題の数（0 も数える）
}

// Median は件数と中央値（小数 2 桁。件数が 0 なら null）。
type Median struct {
	Count  int      `json:"count"`
	Median *float64 `json:"median"`
}

// GateStats は gate の記録の数と差し戻しの率（(fix + needs_human) ÷ 記録。記録が無ければ null）。
type GateStats struct {
	Records    int      `json:"records"`
	Fix        int      `json:"fix"`
	NeedsHuman int      `json:"needs_human"`
	Rate       *float64 `json:"rate"`
}

// ChiefOfStaff は振り返りの期間の Chief of Staff のトークン（役ごとの生の記録から。残っている日の分だけ）。
type ChiefOfStaff struct {
	Days   int           `json:"days"`   // 記録の読めた日付のファイルの数
	Tokens *trace.Tokens `json:"tokens"` // 記録が無ければ null（0 と分ける）
}

// breakdown は課題のフォルダ lib の outcome.json の breakdown。無い・null・読めないなら nil。
// schema の形でなければ飛ばした記録に 1 行書いて nil（1 件の壊れた記録で collect の全体を落とさない）。
// 読めない・JSON でない outcome.json は object() が飛ばした記録に書くので、ここでは書かない。
// 数は上限で止めた int64 を持ちうるので、map[string]any を通さず（float64 に落ちる）、生の JSON から読む。
func (r *reader) breakdown(lib string) *trace.Breakdown {
	path := filepath.Join(lib, "outcome.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var v struct {
		Breakdown json.RawMessage `json:"breakdown"`
	}
	if json.Unmarshal(b, &v) != nil || len(v.Breakdown) == 0 || string(v.Breakdown) == "null" {
		return nil
	}
	if err := trace.CheckBreakdown(v.Breakdown); err != nil {
		r.skip(r.rel(path), "breakdown が schema/outcome.schema.json の形でない（%s）。内訳は null にした", strings.Join(strings.Fields(err.Error()), " "))
		return nil
	}
	var bd trace.Breakdown
	if json.Unmarshal(v.Breakdown, &bd) != nil {
		r.skip(r.rel(path), "breakdown が読めない。内訳は null にした")
		return nil
	}
	return &bd
}

// issueNums は前後の数字に数える課題 1 件の値。
type issueNums struct {
	worked *float64
	tokens *float64
	gates  []Gate
	rounds int
}

// nums は前後の数字に使う、課題ごとの値を読む（同じ課題は 1 度だけ読む）。
type nums struct {
	r      *reader
	rounds map[string]int
	cache  map[string]*issueNums
}

func (r *reader) nums(tasks []*task) *nums {
	n := &nums{r: r, rounds: map[string]int{}, cache: map[string]*issueNums{}}
	for _, t := range tasks {
		if !t.reviewer() {
			continue
		}
		p := t.parent
		if base, _, ok := ledger.SplitRound(t.id); ok && p == "" {
			p = base
		}
		n.rounds[p]++
	}
	return n
}

func (n *nums) of(id string) *issueNums {
	if x := n.cache[id]; x != nil {
		return x
	}
	lib := filepath.Join(n.r.dir, "issues", id, "library")
	x := &issueNums{gates: n.r.gates(filepath.Join(lib, "gate")), rounds: n.rounds[id]}
	if o := n.r.object(filepath.Join(lib, "outcome.json")); o != nil {
		if w := outcomeView(o).WorkedMinutes; w != nil {
			v := float64(*w)
			x.worked = &v
		}
		if b := n.r.breakdown(lib); b != nil && (tokenSum(b.Tokens) > 0 || b.LLMMs > 0) {
			// モデルの呼び出しの記録の無い内訳（Codex の課題）を「トークン 0」と数えない（andon measure と同じ）
			v := float64(tokenSum(b.Tokens))
			x.tokens = &v
		}
	}
	n.cache[id] = x
	return x
}

// beforeAfter は、マージの時刻 merged より前と後に片付いた担当の課題 done（片付いた時刻の順）から、前後の数字を組む。
// 前はマージに近い（遅い）ほうから、後はマージに近い（早い）ほうから sideMax 件まで。マージと同じ時刻の課題はどちらにも入れない。
func (n *nums) beforeAfter(done []doneTask, merged time.Time) BeforeAfter {
	var before, after []string
	for _, d := range done {
		switch {
		case d.at.Before(merged):
			before = append(before, d.id)
		case d.at.After(merged):
			after = append(after, d.id)
		}
	}
	if len(before) > sideMax {
		before = before[len(before)-sideMax:]
	}
	if len(after) > sideMax {
		after = after[:sideMax]
	}
	ba := BeforeAfter{Before: Side{Issues: len(before)}, After: Side{Issues: len(after)}}
	var short []string
	if len(before) < sideMin {
		short = append(short, fmt.Sprintf("前が %d 件", len(before)))
	}
	if len(after) < sideMin {
		short = append(short, fmt.Sprintf("後が %d 件", len(after)))
	}
	if short != nil {
		s := fmt.Sprintf("マージの前と後に片付いた担当の課題のうち、%sで、%d 件に満たない。数字は出さない", strings.Join(short, "・"), sideMin)
		ba.Short = &s
		return ba
	}
	ba.Before.Numbers = n.numbers(before)
	ba.After.Numbers = n.numbers(after)
	return ba
}

func (n *nums) numbers(ids []string) *Numbers {
	var worked, tokens, rounds []float64
	out := &Numbers{}
	for _, id := range ids {
		x := n.of(id)
		if x.worked != nil {
			worked = append(worked, *x.worked)
		}
		if x.tokens != nil {
			tokens = append(tokens, *x.tokens)
		}
		rounds = append(rounds, float64(x.rounds))
		for _, g := range x.gates {
			out.Gate.Records++
			switch g.Decision {
			case "fix":
				out.Gate.Fix++
			case "needs_human":
				out.Gate.NeedsHuman++
			}
		}
	}
	out.WorkedMinutes = Median{Count: len(worked), Median: median(worked)}
	out.Tokens = Median{Count: len(tokens), Median: median(tokens)}
	out.ReviewRounds = Median{Count: len(rounds), Median: median(rounds)}
	if out.Gate.Records > 0 {
		v := round2(float64(out.Gate.Fix+out.Gate.NeedsHuman) / float64(out.Gate.Records))
		out.Gate.Rate = &v
	}
	return out
}

// doneTask は片付いた担当の課題と、その片付いた時刻。
type doneTask struct {
	id string
	at time.Time
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
	m = round2(m)
	return &m
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// tokenSum は 4 つのトークンの和（int64 の上限で止める）。
func tokenSum(t trace.Tokens) int64 {
	var s int64
	for _, v := range []int64{t.Input, t.Output, t.CacheRead, t.CacheWrite} {
		if v <= 0 {
			continue
		}
		if s > math.MaxInt64-v {
			return math.MaxInt64
		}
		s += v
	}
	return s
}
