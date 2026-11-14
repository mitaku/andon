package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/dispatcher"
	"andon/internal/gate"
	"andon/internal/ledger"
)

// merge は andon issue merge: review の課題にゲートを回し（review に入った log.jsonl の行ごとに1回）、approve ＋ merge: auto ＋
// 委譲の範囲内ならマージして片付ける。fix は理由を担当へ送り、needs_human は問い（from: gate）を出す（dispatcher.md §5 の P6a）。
// 積む通知は stdout の JSON で返す（積むのは dispatcher）。2回打っても同じ結果になるよう、行ごとに決めたことを自分の記録に持つ
// （ADR-0017 決めたこと 5・6）。記録は dispatcher も読む（approve のまま 10 分。dispatcher.md §3.2）。
func (c *ctx) merge(stdout, stderr io.Writer, now time.Time) error {
	path := dispatcher.MergePath(c.l.Slug)
	rec := map[string]contract.Merging{}
	fresh := false
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &rec); err != nil {
			return fail("merge の記録が読めない（%s）: %v", path, err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		fresh = true
	} else {
		return err
	}
	if err := forgetOthers(c, rec); err != nil {
		return err
	}
	res := contract.Merge{Issue: c.id, State: "skip", Log: []string{}, Notify: []contract.Notice{}}
	w := c.mergeOne(rec[c.id], fresh, &res, stderr, now)
	if w == nil {
		delete(rec, c.id)
	} else {
		rec[c.id] = *w
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := contract.Write(path, rec); err != nil {
		return fail("merge の記録を書けない: %v", err)
	}
	_, err := stdout.Write(contract.Marshal(res))
	return err
}

// forgetOthers は review にいない課題の記録を捨てる（片付いた・doing に戻った・やめた）。打たれた課題の分は呼び手が決める。
func forgetOthers[T any](c *ctx, rec map[string]T) error {
	others := false
	for id := range rec {
		others = others || id != c.id
	}
	if !others {
		return nil
	}
	tasks, err := c.l.List()
	if err != nil {
		return err
	}
	review := map[string]bool{}
	for _, t := range tasks {
		review[t.Str("id")] = t.Str("status") == "review"
	}
	for id := range rec {
		if id != c.id && !review[id] {
			delete(rec, id)
		}
	}
	return nil
}

// mergeOne は1つの課題を見て res を埋め、残す記録を返す（nil なら記録から外す）。
func (c *ctx) mergeOne(w contract.Merging, fresh bool, res *contract.Merge, stderr io.Writer, now time.Time) *contract.Merging {
	if c.issue.Str("status") != "review" {
		return nil
	}
	rows := c.l.Log()
	row := 0
	for i, r := range rows {
		if r.ID == c.id && r.Change["status"] == "review" {
			row = i + 1
		}
	}
	rep, err := c.previous()
	if row == 0 || err != nil || rep == nil {
		return nil
	}
	if w.Row != row {
		w = contract.Merging{Row: row}
		// 記録が無い（初めて打つ・消えた）ときは、この行のあとに書かれたゲートの記録を引き継ぐ（同じ行に2度目の問いや通知を出さない）
		if fresh {
			if g := c.gatedSince(rows[row-1].At); g != nil {
				w.Sha, w.Decision = g.Head, g.Decision
				return &w
			}
		}
	}
	if w.Decision != "" {
		return &w
	}
	if w.Wait != "" && c.stillWaiting(rows, rep.PR, w.Wait, now) {
		// wait を返した head の checks がまだ揃っていない: ゲートを回さない（GraphQL の checks だけを読み、REST を使わない）
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s は GitHub の checks を待つ（ゲートは回さない）: head %s の checks がまだ揃っていない", c.id, short12(w.Wait)))
		return &w
	}
	w.Wait = ""
	var out, errb bytes.Buffer
	code := gate.Main([]string{c.id}, nil, &out, &errb)
	if code == 3 && strings.Contains(strings.ToLower(errb.String()), "rate limit") {
		// gh の上限で落ちた: 問いにもせず、決めたことも書かない（いつ回し直すかは dispatcher が台帳ごとに決める）
		res.State = "rate-limited"
		return &w
	}
	var gr gateResult
	if (code != 0 && code != 1 && code != 4 && code != 5) || json.Unmarshal(out.Bytes(), &gr) != nil {
		w.Decision, res.State = "needs_human", "needs_human"
		why := fmt.Sprintf("andon gate %s: 終了コード %d %s", c.id, code, firstLine(errb.String()))
		res.Log = append(res.Log, fmt.Sprintf("%s のゲートが判定できない: %s", c.id, why))
		c.askGate(res, contract.Question{
			Kind: "irreversible", Question: fmt.Sprintf("%s（%s）のゲートが判定できなかった。PR を人が見るか", c.id, short(c.issue.Str("title"))),
			Facts: []string{why}, From: "gate",
			Options: []contract.Option{{Label: "人が PR を読んでマージを決める", Pick: ptr(true)}, {Label: "Chief of Staff が原因を直してから回し直す"}}})
		return &w
	}
	if gr.Decision == "wait" {
		// GitHub の checks が揃っていない: 決めたことは書かず、次に打ったときゲートから回し直す（通知もしない）
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s のゲートは GitHub の checks を待つ: %s", c.id, strings.Join(gr.Reasons, "／")))
		if contract.IsSHA(gr.Head) {
			w.Wait = gr.Head
		}
		return &w
	}
	record := ""
	if contract.IsSHA(gr.Head) {
		record = filepath.Join(c.l.Library(c.id), "gate", gr.Head+".json")
		res.Auto = gr.Decision == "approve" && c.autoMerge(rows, gr)
		res.Notify = append(res.Notify, contract.Notice{Kind: "gate",
			Detail: map[string]any{"decision": gr.Decision, "reasons": nonNil(gr.Reasons), "record": record}})
	}
	switch gr.Decision {
	case "approve":
		if res.Auto && w.Approved == "" { // 回し直したゲートの approve では測り直さない
			w.Approved = now.UTC().Format("2006-01-02T15:04:05Z")
		}
		if c.approve(rep, gr, record, &w, res, stderr) {
			w.Sha, res.State = gr.Head, "regate" // 決めたことは書かない: 次に打ったときゲートから回し直す
			return &w
		}
	case "fix":
		c.sendFix(gr, record, res)
	default:
		kind := "irreversible"
		if gr.Tier == "C" {
			kind = "line"
		}
		facts := append([]string{}, gr.Reasons...)
		if rep.PR != "" {
			facts = append(facts, "PR "+rep.PR)
		}
		if record != "" {
			facts = append(facts, "ゲートの記録 "+record)
		}
		c.askGate(res, contract.Question{
			Kind: kind, Question: fmt.Sprintf("%s（%s）の PR はゲートが人へ回した。マージするか", c.id, short(c.issue.Str("title"))),
			Facts: facts, Reason: "ゲートの決定式が needs_human を返した（委譲・レベル・人が見る PR の事情）", From: "gate",
			Options: []contract.Option{{Label: "人が PR を読んでマージする", Pick: ptr(true)}, {Label: "担当に直させる（理由を Chief of Staff が返す）"}}})
	}
	w.Sha, w.Decision, w.Approved = gr.Head, gr.Decision, ""
	res.State = gr.Decision
	if w.Merged != "" {
		res.State = "merged"
	}
	return &w
}

// gatedSince は since 以後に書かれたこの課題のゲートの記録（library/gate/*.json の at）を返す。無ければ nil。
func (c *ctx) gatedSince(since string) *gate.Record {
	files, _ := filepath.Glob(filepath.Join(c.l.Library(c.id), "gate", "*.json"))
	var last *gate.Record
	for _, f := range files {
		b, err := os.ReadFile(f)
		var g gate.Record
		if err != nil || json.Unmarshal(b, &g) != nil || g.At < since || (last != nil && g.At < last.At) {
			continue
		}
		last = &g
	}
	return last
}

// gateResult は andon gate の出力のうち merge が読むもの（記録 contract.md §7。信じられる PR が無いときは decision・reasons だけ）。
type gateResult struct {
	Decision   string   `json:"decision"`
	Reasons    []string `json:"reasons"`
	Head       string   `json:"head"`
	Tier       string   `json:"tier"`
	PR         string   `json:"pr"`
	Delegation struct {
		Merge string `json:"merge"`
		OK    bool   `json:"ok"`
	} `json:"delegation"`
}

// autoMerge は gate の approve のあとマージに進むか（merge: auto を Chief of Staff が書いた・委譲の範囲内）。
func (c *ctx) autoMerge(rows []ledger.Row, gr gateResult) bool {
	return ledger.SettingsOf(c.issue).Merge == "auto" && c.l.Trusted(rows, c.issue, "merge") && gr.Delegation.OK
}

// stillWaiting は、PR の head が wait を返したときの head のままで、ゲートを回しても wait になる checks のままか（GraphQL を1回）。
// checks が走っている（pending）か、1つも無く（none）review に入ってから github_checks_wait_minutes を過ぎていない。
// 読めない・過ぎた・揃ったなら false（ゲートを回す。期限を過ぎた none はゲートが needs_human を返す）。
func (c *ctx) stillWaiting(rows []ledger.Row, pr, waited string, now time.Time) bool {
	owner, name, number, ok := contract.ParsePR(pr)
	if !ok {
		return false
	}
	head, checks, err := gate.ReadChecks(number, owner+"/"+name)
	if err != nil || head != waited {
		return false
	}
	switch checks.State {
	case "pending":
		return true
	case "none":
		table, _, err := gate.LoadTable(pkgHome(), delegate.Workspace())
		return err == nil && !gate.ChecksOverdue(rows, c.id, now.UTC(), *table.GitHubChecksWait)
	}
	return false
}

// sendFix は理由を担当へ返す（担当は人が打つ端末ではないので、離席の守りは要らない）。担当がいなければ通知 report。
func (c *ctx) sendFix(gr gateResult, record string, res *contract.Merge) {
	agent := c.issue.Str("agent")
	if agent == "" {
		res.Log = append(res.Log, fmt.Sprintf("%s のゲートが fix だが担当（agent）がいない", c.id))
		res.Notify = append(res.Notify, contract.Notice{Kind: "report", Detail: map[string]any{"report": filepath.Join(c.l.Dir, "issues", c.id, "report.md")}})
		return
	}
	msg := fmt.Sprintf("%s（%s）: ゲートが fix を返した。理由: %s。", c.id, short(c.issue.Str("title")), strings.Join(gr.Reasons, "／"))
	if record != "" {
		msg += "記録は " + record + "。"
	}
	msg += "kanban start で doing に戻して直し、push して andon issue review-request（レビューが要るとき）か andon issue ready からやり直す。"
	if out, err := exec.Command("hc", "send", agent, msg).CombinedOutput(); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の担当 %s に fix を送れない: %v %s", c.id, agent, err, firstLine(string(out))))
	}
}

// approve は merge: auto ＋ 委譲の範囲内ならマージして片付ける。それ以外は通知 gate のまま止まる（人がマージする）。
// マージの直前に、課題の Settings・履歴・委譲を読み直し、ゲートのときと変わっていればマージせず regate を返す
// （呼び手は決めたことを書かず、次に打ったときゲートから回し直す）。
func (c *ctx) approve(rep *contract.Report, gr gateResult, record string, w *contract.Merging, res *contract.Merge, stderr io.Writer) (regate bool) {
	if !res.Auto {
		return false
	}
	logf := func(format string, a ...any) { res.Log = append(res.Log, c.id+" "+fmt.Sprintf(format, a...)) }
	stop := func(why string) {
		logf("をマージしない: %s", why)
		c.askGate(res, contract.Question{
			Kind: "irreversible", Question: fmt.Sprintf("%s（%s）はゲートが approve したが、dispatcher がマージできなかった。人がマージするか", c.id, short(c.issue.Str("title"))),
			Facts: []string{why, "PR " + rep.PR, "ゲートの記録 " + record}, From: "gate",
			Options: []contract.Option{{Label: "人が PR を読んでマージする", Pick: ptr(true)}, {Label: "Chief of Staff が原因を直してから回し直す"}}})
	}
	table, warn, err := gate.LoadTable(pkgHome(), delegate.Workspace())
	if err != nil {
		stop("条件表が読めない: " + err.Error())
		return false
	}
	if warn != "" {
		res.Log = append(res.Log, warn)
	}
	repo := c.issue.Str("repo")
	method := table.Repos[repo].Merge
	if method == "" {
		stop(fmt.Sprintf("条件表の repos.%s.merge（マージの方式）が無い", repo))
		return false
	}
	// マージするのはゲートが判定した PR と head だけ（判定のあとに report.json が書き換わっていたら打たない）
	owner, name, number, ok := contract.ParsePR(gr.PR)
	switch {
	case !ok || !contract.IsSHA(gr.Head):
		stop("ゲートの記録の pr・head が読めない")
		return false
	case gr.PR != rep.PR:
		stop(fmt.Sprintf("ゲートが判定した PR（%s）と report.json の pr（%s）が違う", gr.PR, rep.PR))
		return false
	case "github.com/"+owner+"/"+name != repo:
		stop(fmt.Sprintf("ゲートが判定した PR の repo が issue.yaml の repo（%s）と違う", repo))
		return false
	}
	slug := owner + "/" + name
	if ok, why := c.stillAuthorized(gr); !ok {
		logf("をいまはマージしない（次に打ったときゲートから回し直す）: %s", why)
		return true
	}
	// PR の head と GitHub の checks も読み直す（ゲートのあとに push された・checks が変わったら打たない）
	nowHead, checks, err := gate.ReadChecks(number, slug)
	if err != nil {
		logf("をいまはマージしない（次に打ったときゲートから回し直す）: %v", err)
		return true
	}
	if ok, why := ChecksStill(gr.Head, nowHead, checks, table.Repos[repo].GitHubChecks == "optional"); !ok {
		logf("をいまはマージしない（次に打ったときゲートから回し直す）: %s", why)
		return true
	}
	if _, err := gh("pr", "merge", number, "--repo", slug, "--"+method, "--match-head-commit", gr.Head); err != nil {
		// 人が先にマージしていた（ゲートが判定した head のまま MERGED）なら誤報の問いを出さず、人がマージしたときの道（§2.7）へ渡す。
		// 同じ巡の andon issue settle が片付ける。ゲートの記録の merged は書かない（dispatcher がマージした印）
		out, verr := gh("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit,headRefOid")
		if sha, ok := MergedBefore(out, gr.Head); verr == nil && ok {
			logf("の PR %s は gh pr merge の前に同じ head でマージされていた（%s）。問いを出さず人のマージとして片付ける", rep.PR, short12(sha))
			return false
		}
		stop("gh pr merge に失敗: " + err.Error())
		return false
	}
	out, err := gh("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit")
	var view struct {
		State       string `json:"state"`
		MergeCommit struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err != nil || json.Unmarshal([]byte(out), &view) != nil || view.State != "MERGED" || !contract.IsSHA(view.MergeCommit.Oid) {
		stop(fmt.Sprintf("マージを打ったが PR が MERGED にならない（%s）", view.State))
		return false
	}
	w.Merged = view.MergeCommit.Oid
	if err := markMerged(record); err != nil {
		logf("のゲートの記録に merged を書けない: %v", err)
	}
	logf("の PR %s をマージした（%s）", rep.PR, short12(view.MergeCommit.Oid))
	if why := c.finishMerged(stderr); why != "" {
		c.askGate(res, contract.Question{
			Kind: "irreversible", Question: fmt.Sprintf("%s（%s）はマージしたが片付けに失敗した。Chief of Staff が片付けるか", c.id, short(c.issue.Str("title"))),
			Facts: []string{why, "PR " + rep.PR}, From: "gate",
			Options: []contract.Option{{Label: "Chief of Staff が理由を読んで andon issue finish する", Pick: ptr(true)}, {Label: "人が片付ける"}}})
	}
	res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": rep.PR, "sha": view.MergeCommit.Oid}})
	return false
}

// finishMerged は finish を同じプロセスで打つ（lock を持ったまま。別プロセスにすると同じ lock を待って止まる）。失敗したら理由を返す。
func (c *ctx) finishMerged(stderr io.Writer) string {
	if err := allowed("finish", c.id, c.issue.Str("status")); err != nil {
		return err.Error()
	}
	if err := c.finish(stderr); err != nil {
		return err.Error()
	}
	c.writeBack(stderr) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
	return ""
}

// askGate は問いを台帳に出し（andon issue ask と同じ道）、通知 asked を res に足す。出せなければログに1行。
func (c *ctx) askGate(res *contract.Merge, q contract.Question) {
	b, _ := json.Marshal(q)
	var out bytes.Buffer
	if err := c.ask(bytes.NewReader(b), &out, "-"); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の問いを出せない: %v", c.id, err))
		return
	}
	m := askedRe.FindStringSubmatch(strings.TrimSpace(out.String()))
	if m == nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の問いの番号が読めない: %q", c.id, out.String()))
		return
	}
	var n int
	fmt.Sscan(m[1], &n)
	res.Notify = append(res.Notify, contract.Notice{Kind: "asked", Detail: map[string]any{"n": n, "kind": q.Kind, "question": q.Question}})
}

// stillAuthorized はマージの直前に課題と履歴と委譲を読み直し、ゲートのときの認可がまだ効いているか。
func (c *ctx) stillAuthorized(gr gateResult) (bool, string) {
	now, err := c.l.Show(c.id)
	if err != nil {
		return false, "課題を読み直せない: " + err.Error()
	}
	rows := c.l.Log()
	trusted := c.l.Trusted(rows, now, "merge") && c.l.Trusted(rows, now, "review") && c.l.Trusted(rows, now, "must_fix")
	merge := delegate.Read(delegate.Workspace(), c.l.Slug, time.Now().UTC().Format("2006-01-02")).Merge()
	return Authorized(ledger.SettingsOf(c.issue), ledger.SettingsOf(now), now.Str("status"), trusted, gr.Delegation.Merge, merge, gr.Tier)
}

// Authorized は、ゲートのときの事実（before・gateMerge）とマージ直前に読み直した事実（now・status・trusted・nowMerge）から、
// dispatcher の道でマージしてよいかを決める（純粋関数）。どれか1つでも変わっていれば、いまはマージしない。
func Authorized(before, now ledger.Settings, status string, trusted bool, gateMerge, nowMerge, tier string) (bool, string) {
	switch {
	case status != "review":
		return false, fmt.Sprintf("課題が review でなくなった（%s）", status)
	case now != before:
		return false, "issue.yaml の予約キーがゲートのあとに変わった"
	case now.Merge != "auto" || !trusted:
		return false, "merge: auto でない、または予約キーを最後に書いたのが Chief of Staff でない"
	case nowMerge != gateMerge:
		return false, fmt.Sprintf("委譲の merge がゲートのあとに変わった（%s → %s）", gateMerge, nowMerge)
	case !gate.Within(tier, nowMerge):
		return false, fmt.Sprintf("delegation: merge=%s < tier %s", nowMerge, tier)
	}
	return true, ""
}

// ChecksStill は、マージの直前に読み直した PR の head と GitHub の checks で、まだマージしてよいか（純粋関数）。
// head がゲートの判定した head と違う（ゲートのあとに push された）か、checks が通す状態でなければマージしない。
func ChecksStill(gateHead, nowHead string, checks gate.GitHubChecks, optional bool) (bool, string) {
	switch {
	case nowHead != gateHead:
		return false, fmt.Sprintf("PR の head がゲートのあとに動いた（%s → %s）", short12(gateHead), short12(nowHead))
	case !checks.Passed(optional):
		return false, fmt.Sprintf("GitHub の checks がゲートのあとに変わった（%s: %s）", checks.State, strings.Join(checks.Names, ", "))
	}
	return true, ""
}

// MergedBefore は、gh pr merge が失敗したあとに読み直した PR（gh pr view --json state,mergeCommit,headRefOid の返事）が、
// ゲートの判定した head のまま、もうマージされていたか（純粋関数。dispatcher.md §7.2 #5）。そうならマージの commit を返す。
// MERGED でない・head が違う・返事が読めないときは false（今までどおり問いにする）。
func MergedBefore(view, gateHead string) (string, bool) {
	var v struct {
		State       string `json:"state"`
		HeadRefOid  string `json:"headRefOid"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if json.Unmarshal([]byte(view), &v) != nil || v.State != "MERGED" || !contract.IsSHA(gateHead) || v.HeadRefOid != gateHead ||
		v.MergeCommit == nil || !contract.IsSHA(v.MergeCommit.Oid) {
		return "", false
	}
	return v.MergeCommit.Oid, true
}

// markMerged はゲートの記録の merged を true にする（dispatcher の道でマージした印。settle が読む。contract.md §7）。
func markMerged(record string) error {
	b, err := os.ReadFile(record)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	m["merged"] = true
	return contract.Write(record, m)
}

// gh は gh を打ち、stdout を返す（失敗は stderr の1行目つき）。
func gh(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return string(out), fmt.Errorf("gh %s: %v %s", args[0]+" "+args[1], err, firstLine(errb.String()))
	}
	return string(out), nil
}

// pkgHome はパッケージの置き場（ANDON_HOME。無ければ /opt/andon）。
func pkgHome() string {
	if h := os.Getenv("ANDON_HOME"); h != "" {
		return h
	}
	return "/opt/andon"
}

func ptr[T any](v T) *T { return &v }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func short12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// short は題を問いの1行に入れる形に切る（改行を除き40字まで）。
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}
