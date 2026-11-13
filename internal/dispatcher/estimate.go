package dispatcher

import (
	"math"
	"strconv"
	"strings"
	"time"

	"andon/internal/ledger"
)

// 見込みとのズレ（docs/dispatcher.md §2.10。ADR-0014 決めたこと 5〜8・11 の 4）: doing の担当の課題の実働
// （ledger.Worked。待ちを除いた時間）が見込み（予約キー estimate の分）の 1・2・4 倍を超えたら、段ごとに1回だけ
// estimate を積む。届けるのは Chief of Staff だけで、担当には何も送らない（止めない・打たない）。

// estimateStages は知らせる段（見込みの何倍を超えたか）。
var estimateStages = []int{1, 2, 4}

// GoneAt は担当の端末が Gone だと見た時刻（Unix ミリ秒）。実働から「死んでいた区間」を引くのに使う。
type GoneAt struct {
	Agent string `json:"agent"`
	At    int64  `json:"at"`
}

// sawGone は課題 id の端末 agent が Gone だと見た最初の時刻を控える（同じ端末の死は1回だけ）。
func (p *project) sawGone(id, agent string, now time.Time) {
	for _, g := range p.st.Gone[id] {
		if g.Agent == agent {
			return
		}
	}
	p.st.Gone[id] = append(p.st.Gone[id], GoneAt{Agent: agent, At: now.UnixMilli()})
}

// estimated は、課題がズレを数える担当の課題か（レビュアー・Chief Engineer・人・Chief of Staff 自身の課題は数えない）。
func estimated(t ledger.Issue, cos string) bool {
	return !isReviewer(t) && t.Str("profile") != "chief-engineer" && t.Str("who") != "person" && t.Str("agent") != cos
}

// estimate は doing の担当の課題ごとに実働を数え、超えた段のうちまだ知らせていないものがあれば、一番上の段を1件積む
// （下の段も知らせたことにする）。控えのキーは <issue>|estimate|<見込みの値>|<段> で、最初の列に戻る
// （todo・queued・done・台帳から消えた）まで持ち越す。見込みの値が替われば古い値のキーは捨てて数え直す。
// 初めて見る台帳では今の段を控えるだけで積まない。
func (p *project) estimate(now time.Time, cos string) {
	keep := map[string]string{} // 控えを残す課題 → 今の見込みの値
	for _, t := range p.tasks {
		id := t.Str("id")
		switch t.Str("status") {
		case "todo", "queued", "done", "":
			continue
		}
		v := p.l.Estimate(p.rows, t)
		keep[id] = v
		if t.Str("status") != "doing" || v == "" || !estimated(t, cos) {
			continue
		}
		est := ledger.EstimateMinutes(v)
		var gone []time.Time
		for _, g := range p.st.Gone[id] {
			gone = append(gone, time.UnixMilli(g.At))
		}
		worked := ledger.Worked(p.rows, p.tasks, id, cos, gone, now)
		top := 0
		for _, s := range estimateStages {
			k := estimateKey(id, v, s)
			if worked <= time.Duration(est*s)*time.Minute || p.st.Estimate[k] {
				continue
			}
			p.st.Estimate[k] = true
			top = s
		}
		if top == 0 || p.first {
			continue
		}
		ratio := worked.Minutes() / float64(est)
		detail := map[string]any{
			"estimate":         v,
			"estimate_minutes": est,
			"worked_minutes":   int(worked.Minutes()),
			"ratio":            math.Round(ratio*100) / 100,
			"stage":            top,
		}
		if s := signs(t.Str("worktree"), est, now); len(s) > 0 {
			detail["sign"] = s
		}
		p.notify(id, "estimate", detail)
	}
	for k := range p.st.Estimate {
		id, rest, _ := strings.Cut(k, "|estimate|")
		v, _, _ := strings.Cut(rest, "|")
		if cur, ok := keep[id]; !ok || cur != v {
			delete(p.st.Estimate, k)
		}
	}
	// Gone は done のあとも残す（andon measure が閉じた課題の実働から死んでいた区間を引く）。最初の列に戻るか台帳から消えたら捨てる。
	live := map[string]bool{}
	for _, t := range p.tasks {
		if s := t.Str("status"); s != "todo" && s != "queued" && s != "" {
			live[t.Str("id")] = true
		}
	}
	for id := range p.st.Gone {
		if !live[id] {
			delete(p.st.Gone, id)
		}
	}
}

func estimateKey(id, v string, stage int) string {
	return id + "|estimate|" + v + "|" + strconv.Itoa(stage)
}

// diffLimit は large-diff の線（テストを除いた変更の行数。横展開の既定の上限と同じ）。
const diffLimit = 500

// signs は段の estimate に乗せる詰まりの兆し（ADR-0017 決めたこと 11）を、課題の worktree の git で数える。
// no-commit: worktree の HEAD の commit から見込みの分より長く経った。large-diff: origin/HEAD との merge-base から
// 作業ツリーまでの変更（追跡しているファイル。テストを除く）が diffLimit 行を超えた。worktree が無い・git で読めない兆しは数えない。
func signs(wt string, est int, now time.Time) []string {
	if wt == "" {
		return nil
	}
	git := func(args ...string) (string, bool) {
		out, _, err := run("", nil, "", "git", append([]string{"-C", wt}, args...)...)
		return strings.TrimSpace(out), err == nil
	}
	var s []string
	if out, ok := git("log", "-1", "--format=%ct"); ok {
		if ct, err := strconv.ParseInt(out, 10, 64); err == nil && now.Sub(time.Unix(ct, 0)) > time.Duration(est)*time.Minute {
			s = append(s, "no-commit")
		}
	}
	if base, ok := git("merge-base", "HEAD", "origin/HEAD"); ok {
		if out, ok := git("diff", "--numstat", "--no-renames", base); ok && changedLines(out) > diffLimit {
			s = append(s, "large-diff")
		}
	}
	return s
}

// changedLines は git diff --numstat の足した行と消した行の和。テスト（tests という名のディレクトリの下と、
// 名前が *_test.* のファイル）と、行で数えないファイル（バイナリ）は数えない。
func changedLines(numstat string) int {
	n := 0
	for _, ln := range strings.Split(numstat, "\n") {
		f := strings.SplitN(ln, "\t", 3)
		if len(f) != 3 || isTestPath(f[2]) {
			continue
		}
		a, err1 := strconv.Atoi(f[0])
		d, err2 := strconv.Atoi(f[1])
		if err1 == nil && err2 == nil {
			n += a + d
		}
	}
	return n
}

func isTestPath(p string) bool {
	dirs := strings.Split(p, "/")
	for _, d := range dirs[:len(dirs)-1] {
		if d == "tests" {
			return true
		}
	}
	return strings.Contains(dirs[len(dirs)-1], "_test.")
}
