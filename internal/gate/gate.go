// Package gate は andon gate（docs/contract.md §7。仮設の gate lib/yard/gate を Go に移したもの）。
// Markdown（依頼書・報告）は読まない。マージはしない（dispatcher が記録を読んでする。P6a）。
//
// 終了コード: approve 0・fix 1・使い方 2・道具や kanban の誤り 3・needs_human 4・wait 5（記録しない）・behind 6（積んだ課題の前の PR 待ち）。
package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/flock"
	"andon/internal/ledger"
	"andon/internal/spawn"
	"andon/internal/tools"
)

const Usage = "usage: andon gate <id> [--dry-run]\n" +
	"kanban の課題の PR（report.json の pr）を、issue.yaml の予約キー・findings.json・条件表・委譲で照合し、記録を JSON で出す。\n" +
	"記録は issues/<id>/library/gate/<head>.json に書き、PR に commit status（andon/gate）とコメントを付ける（--dry-run は書かない）。\n" +
	"GitHub の checks がまだ揃っていなければ wait（記録も PR への書き込みもしない。回し直す）。\n" +
	"マージはしない。終了コード: approve 0・fix 1・needs_human 4・wait 5・behind 6（積んだ課題の前の PR 待ち）・誤り 3（deliverable: candidates の課題と、PR の無い deliverable: report の課題も。回さない）。\n"

const (
	exitFix   = 1
	exitUsage = 2
	exitError = 3
	exitHuman = 4
	exitWait  = 5
	// exitBehind は前の PR 待ち（behind。ADR-0067 決めたこと 8）
	exitBehind = 6
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Record は gate の記録（schema/gate.schema.json の record）。
type Record struct {
	At           string          `json:"at"`
	ID           string          `json:"id"`
	PR           string          `json:"pr"`
	Head         string          `json:"head"`
	Tier         string          `json:"tier"`
	TierReasons  []string        `json:"tier_reasons"`
	ChangedLines int             `json:"changed_lines"`
	Checks       []Check         `json:"checks"`
	GitHubChecks GitHubChecks    `json:"github_checks"`
	Review       ledger.Review   `json:"review"`
	Acceptance   AcceptanceCount `json:"acceptance"`
	Delegation   struct {
		Merge string `json:"merge"`
		OK    bool   `json:"ok"`
		Grant string `json:"grant,omitempty"` // レベルを通した名指しの grant の id（ADR-0041 決めたこと 9）
	} `json:"delegation"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
	// Restack は載せ替えを頼む直し（fix）の印（ADR-0067 決めたこと 8・12）。計測と振り返りは差し戻しに数えない
	Restack bool `json:"restack,omitempty"`
}

// Counted は、記録を計測と振り返りの差し戻しの率に数えるか（ADR-0067 決めたこと 8）: 前の PR 待ち（behind）の記録と、
// 載せ替えを頼む直し（restack）の記録は、分母にも分子にも数えない（積んだ課題の数だけ率が動いて見えないように）。
func Counted(decision string, restack bool) bool {
	return decision != "behind" && (decision != "fix" || !restack)
}

// Delegated は課題 id の判定に使う委譲の merge の値と、それを広げた名指しの grant の id（ADR-0041 決めたこと 6・7）。
// 担当が戻せない（one_way）と申告していれば名指しを読まず、プロジェクト全体の merge で判定する。
func Delegated(d delegate.Result, id string, rep *contract.Report) (merge, grant string) {
	return d.MergeFor(id, OneWay(rep))
}

func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 || !idRe.MatchString(args[0]) || (len(args) == 2 && args[1] != "--dry-run") {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	dry := len(args) == 2
	rec, early, err := run(ledger.CanonID(args[0]), dry, stderr)
	var h holding
	if errors.As(err, &h) {
		if err := writeShort(stdout, "wait", string(h)); err != nil {
			fmt.Fprintf(stderr, "andon gate: %v\n", err)
			return exitError
		}
		return exitWait
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon gate: %v\n", err)
		return exitError
	}
	if early != "" {
		if err := writeShort(stdout, "fix", early); err != nil {
			fmt.Fprintf(stderr, "andon gate: %v\n", err)
			return exitError
		}
		return exitFix
	}
	b, err := contract.Encode(contract.SchemaGate, rec)
	if err != nil {
		fmt.Fprintf(stderr, "andon gate: 記録を出せない: %v\n", err)
		return exitError
	}
	stdout.Write(b)
	return ExitCode(rec.Decision)
}

// withWhy は理由の文 s に、添える句 why（空なら添えない）を「。」でつなぐ。
func withWhy(s, why string) string {
	if why == "" {
		return s
	}
	return s + "。" + why
}

// holding は、Chief of Staff が止めている課題で gate を回さなかった印（run が誤りの形で返し、Main が wait を出す）。値は理由の1文。
type holding string

func (h holding) Error() string { return string(h) }

// writeShort は記録を持たない判定（信じられる PR が無いときの fix・止めているときの wait。decision と reasons だけ。
// internal/issue の gateResult が読む形）を w に1行で出す。
// 理由は issue.yaml の値を含み、制御文字があっても JSON の文字列になるよう encoding/json で組む。
func writeShort(w io.Writer, decision, reason string) error {
	b, err := json.Marshal(struct {
		Decision string   `json:"decision"`
		Reasons  []string `json:"reasons"`
	}{decision, []string{reason}})
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func ExitCode(decision string) int {
	switch decision {
	case "approve":
		return 0
	case "fix":
		return exitFix
	case "wait":
		return exitWait
	case "behind":
		return exitBehind
	}
	return exitHuman
}

const recordFact = "gate の記録 "

// oldRecordFact は、2026-11-20 までの語で書いた recordFact の頭。動いているセルの kanban に開いたまま残る gate の問いを読むため、
// 読む側だけが受ける（ADR-0040 決めたこと 7）。書く側は recordFact だけを使う。
const oldRecordFact = "ゲートの記録 " // 古い語を読む（ADR-0040 決めたこと 7）

// RecordFact の行から、問いがどの head について出たかを QuestionHead で読む。
func RecordFact(record string) string { return recordFact + record }

// RecordFact の行が無い・head が読めなければ空（どの head の問いか分からない。呼び手は今の head の問いとして扱う）。
func QuestionHead(facts []string) string {
	for _, f := range facts {
		p, ok := strings.CutPrefix(f, recordFact)
		if !ok {
			p, ok = strings.CutPrefix(f, oldRecordFact)
		}
		if !ok || filepath.Base(filepath.Dir(p)) != "gate" {
			continue
		}
		if h := strings.TrimSuffix(filepath.Base(p), ".json"); contract.IsSHA(h) {
			return h
		}
	}
	return ""
}

func gh(args ...string) ([]byte, error) {
	var errb bytes.Buffer
	c := exec.Command("gh", args...)
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %v %s", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(errb.String()))
	}
	return out, nil
}

type prView struct {
	State             string `json:"state"`
	IsDraft           bool   `json:"isDraft"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	MergeStateStatus  string `json:"mergeStateStatus"`
	Mergeable         string `json:"mergeable"`
	ChangedFiles      int    `json:"changedFiles"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	Author            struct {
		Login string `json:"login"`
	} `json:"author"`
	Body     string `json:"body"`
	Comments []struct {
		Body string `json:"body"`
	} `json:"comments"`
}

func (pv prView) texts() []string {
	out := []string{pv.Body}
	for _, c := range pv.Comments {
		out = append(out, c.Body)
	}
	return out
}

// LoadTable は条件表を読む: パッケージの gate.json に、セルの gate.local.json のキーを1つずつ置き換えて重ねる。
// gate.local.json はセルの repo の HEAD のもので、最後に変えたのが owner のときだけ重ねる（委譲と同じ読み方。
// 信じられなければ重ねず、理由を warn に返す。repos が無くなるので gate は needs_human に倒れる）。
func LoadTable(pkg, ws string) (*Table, string, error) {
	merged := map[string]json.RawMessage{}
	add := func(b []byte, p string) error {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("条件表が JSON でない: %s", p)
		}
		for k, v := range m {
			merged[k] = v
		}
		return nil
	}
	b, err := os.ReadFile(filepath.Join(pkg, "gate.json"))
	if err != nil {
		return nil, "", fmt.Errorf("条件表が読めない: %v", err)
	}
	if err := add(b, "gate.json"); err != nil {
		return nil, "", err
	}
	warn := ""
	switch local, err := delegate.OwnerFile(ws, "gate.local.json"); {
	case errors.Is(err, delegate.ErrAbsent):
	case err != nil:
		warn = "gate.local.json を重ねない: " + err.Error()
	default:
		if err := add(local, "gate.local.json"); err != nil {
			return nil, "", err
		}
	}
	b, _ = json.Marshal(merged)
	var t Table
	if err := contract.Decode(b, &t); err != nil {
		return nil, "", fmt.Errorf("条件表の形が違う: %v", err)
	}
	for r, rt := range t.Repos {
		if rt.Merge != "" && rt.Merge != "merge" && rt.Merge != "squash" && rt.Merge != "rebase" {
			return nil, "", fmt.Errorf("条件表の repos.%s.merge が知らない値: %q", r, rt.Merge)
		}
		for g, v := range rt.TierOverrides {
			if v != "A" && v != "B" && v != "C" {
				return nil, "", fmt.Errorf("条件表の repos.%s.tier_overrides[%s] が A/B/C でない", r, g)
			}
		}
		if rt.GitHubChecks != "" && rt.GitHubChecks != "required" && rt.GitHubChecks != "optional" {
			return nil, "", fmt.Errorf("条件表の repos.%s.github_checks が知らない値: %q", r, rt.GitHubChecks)
		}
	}
	if t.SizeHumanLines == nil || t.SizeReviewLines == nil || t.GitHubChecksWait == nil {
		return nil, "", errors.New("条件表に size_review_lines・size_human_lines・github_checks_wait_minutes が無い")
	}
	return &t, warn, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func run(id string, dry bool, stderr io.Writer) (*Record, string, error) {
	began := time.Now()
	if err := ledger.Check(); err != nil {
		return nil, "", err
	}
	for _, c := range []string{"gh", "git"} {
		if _, err := exec.LookPath(c); err != nil {
			return nil, "", fmt.Errorf("%s が無い", c)
		}
	}
	l, err := ledger.Open()
	if err != nil {
		return nil, "", err
	}
	t, err := l.Show(id)
	if err != nil {
		return nil, "", err
	}
	if ledger.SettingsOf(t).Deliverable == "candidates" {
		// 調べる課題の成果は PR ではなく候補の一覧で、Chief of Staff が読んで切る（ADR-0006 決めたこと 7）。記録しない。
		// 照合が外れていても PR を求めない（Chief of Staff が起き直して書き直す前。決めたこと 17）
		if !l.Candidates(l.Log(), t) {
			return nil, "", errors.New("deliverable: candidates の課題に gate は無い（deliverable を最後に書いたのが Chief of Staff でない。PR の課題なら Chief of Staff が deliverable=pr を書く）")
		}
		return nil, "", errors.New("deliverable: candidates の課題に gate は無い（候補の一覧は Chief of Staff が読む）")
	}
	// 報告の課題（deliverable: report を書いた時点の Chief of Staff が書いた課題）は PR が無くてよい。PR が無ければ回さず、
	// 記録も書かない（ADR-0066 決めたこと 8）。PR があれば PR の課題と同じに回す。照合の外れた report は PR の課題として読み、
	// PR を求める理由の文に外れた理由（notReport）を添える
	form, notReport := l.Deliverable(t)
	if form == "report" {
		if rep, err := contract.ReadReport(filepath.Join(l.Library(id), "report.json")); errors.Is(err, os.ErrNotExist) || (err == nil && rep.PR == "") {
			return nil, "", errors.New("deliverable: report の課題で PR が無い。gate を回すものが無い")
		}
	}
	if !dry {
		// 記録を書いて PR に写す run は課題ごとに1つずつ（G2 を塞いだ。dispatcher.md §7.2 #4）。
		// 待った間にほかの run が同じ head の記録を書いていれば、それを返して書かない・写さない（下の written）。
		// 止め（予約キー hold）は鍵を取ってから読む: andon issue hold は同じ鍵の中で hold を書く（ADR-0056 決めたこと 8）
		unlock, err := flock.Lock(filepath.Join(l.Library(id), ".gate.lock"))
		if err != nil {
			return nil, "", err
		}
		defer unlock()
		if t, err = l.Show(id); err != nil {
			return nil, "", err
		}
	}
	// 止めている課題は GitHub を読まずに待つ（記録も印も書かない。--dry-run も同じ。ADR-0056 決めたこと 9）。
	// 待ちが長いとオーナー待ちに倒す決まり（github_checks_wait_minutes）は当てない。止めを読めなければ止めているものとして待つ
	rows, logErr := l.ReadLog()
	if reason, held, err := l.Hold(rows, logErr, t); err != nil || held {
		if err != nil {
			return nil, "", holding("止めを読めない（止めているものとして待つ）: " + err.Error())
		}
		return nil, "", holding(fmt.Sprintf("Chief of Staff が PR を止めている（理由の課題 %s）。andon issue unhold で外すまで gate は回らない", reason))
	}
	ws := delegate.Workspace()
	table, warn, err := LoadTable(getenv("ANDON_HOME", "/opt/andon"), ws)
	if err != nil {
		return nil, "", err
	}
	if warn != "" {
		fmt.Fprintln(stderr, "andon gate:", warn)
	}

	// L1 kanban: PR の場所は report.json の pr だけ（Markdown も issue.yaml の pr も読まない）
	rep, err := contract.ReadReport(filepath.Join(l.Library(id), "report.json"))
	if err != nil {
		return nil, withWhy("report.json が無い・形が違う（andon issue report）", notReport), nil
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		return nil, withWhy("report.json に pr が無い（andon issue report --pr）", notReport), nil
	}
	repo := t.Str("repo")
	switch {
	case repo != "github.com/"+owner+"/"+name:
		return nil, fmt.Sprintf("issue.yaml の repo（%s）と PR の repo（%s/%s）が違う", repo, owner, name), nil
	case !slices.Contains(l.Repos(), repo):
		return nil, "kanban の config.yaml の repos にこの repo が無い", nil
	case t.Str("branch") == "":
		return nil, "issue.yaml に branch が無い", nil
	}
	slug := owner + "/" + name

	var pv prView
	b, err := gh("pr", "view", number, "--repo", slug, "--json", "state,isDraft,headRefName,headRefOid,baseRefName,mergeStateStatus,mergeable,author,changedFiles,isCrossRepository,body,comments")
	if err != nil {
		return nil, "", err
	}
	if json.Unmarshal(b, &pv) != nil || !contract.IsSHA(pv.HeadRefOid) {
		return nil, "", errors.New("PR が読めない")
	}
	// この課題の PR でない（ほかの課題の PR か、同じ名前の branch を持つ fork の PR を report.json に書いた）。
	// 記録も PR の印とコメントも書かない（ほかの PR に書き込まない）。担当は課題の branch を origin に push するので、fork の PR は課題の PR でない
	switch {
	case pv.HeadRefName != t.Str("branch"):
		return nil, fmt.Sprintf("PR の branch（%s）が issue.yaml の branch（%s）と違う。この課題の PR でないので gate は PR に書かない（andon issue report --pr にこの課題の PR を書く）", pv.HeadRefName, t.Str("branch")), nil
	case pv.IsCrossRepository:
		return nil, "PR が fork から出ている。この課題の PR でないので gate は PR に書かない（andon issue report --pr にこの課題の PR を書く）", nil
	}
	head := pv.HeadRefOid
	rec := &Record{ID: id, PR: rep.PR, Head: head, Checks: []Check{}, TierReasons: []string{}}
	facts := Facts{PROpen: pv.State == "OPEN" && !pv.IsDraft}
	// 積んだ課題（信じられる stacked_on がある）: draft でもよく、PR の事情のうち base・衝突・分かれ目から数えたファイルの件数と古い base を見ない
	// （ADR-0067 決めたこと 8。Decide が approve と needs_human を behind に置き換える）
	// 前の課題の branch が読めなければ判定しない（誤り。向き先が違うと取り違えて担当へ返さない。dispatcher が前の PR 待ちに置く）
	prevBranch, prevDone, from := "", false, ""
	if on, base, ok := ledger.Stacked(rows, t, l.ChiefOfStaff()); ok {
		facts.StackedOn, facts.PROpen, facts.StackedDraft = on, pv.State == "OPEN", pv.IsDraft
		p, err := l.Show(on)
		prevDone = err == nil && p.Str("status") == "done"
		if err != nil {
			return nil, "", fmt.Errorf("積んだ課題の前の課題 %s が kanban で読めない: %v", on, err)
		}
		if prevBranch = p.Str("branch"); prevBranch == "" {
			return nil, "", fmt.Errorf("積んだ課題の前の課題 %s の branch が kanban に無い", on)
		}
		if pv.BaseRefName == prevBranch {
			from = base
		}
	}
	stacked := facts.StackedOn != ""
	if t.Str("status") != "review" {
		facts.LedgerReasons = append(facts.LedgerReasons, fmt.Sprintf("kanban の課題が review でない（%s）", t.Str("status")))
	}
	if rep.Commit != head {
		facts.LedgerReasons = append(facts.LedgerReasons, "report.json の commit が PR の head と違う（andon issue ready からやり直す）")
	}

	// 積んだ課題の差分を分かれ目から数えるのは、PR が前の課題の branch に向いている間だけ（from。stackedFrom と同じ決まりを、上で読んだ
	// 前の課題の branch 1 つで決める）。既定の branch などほかへ向いた PR は、マージで入る中身が分かれ目からの差分より多いことがある
	// （前の課題の変更ごと入る）ので、GitHub の PR の差分で数える
	files, diff, err := prChanges(from, t, slug, number, head)
	if err != nil {
		return nil, "", err
	}
	if from == "" && len(files) != pv.ChangedFiles {
		// 積んだ課題のファイルは分かれ目から数えるので、GitHub の数（PR の base との数）とは比べない（ADR-0067 決めたこと 8・9）。
		// 積んだ課題の PR が既定の branch に向いていれば PR の差分で数えるので照らし、合わなければオーナーへ回す理由として behind の記録に写す
		facts.Blockers = append(facts.Blockers, "files の件数が changedFiles と合わない（一覧の上限・取得中の変更）")
	}
	// 衝突は mergeable でも読む: 必須の check が足りない PR は mergeStateStatus が BLOCKED になり、
	// 衝突と重なったときに DIRTY が返る順の決まりは GitHub の文書に無い（ADR-0056 決めたこと 13）
	if !stacked && (pv.MergeStateStatus == "DIRTY" || pv.Mergeable == "CONFLICTING") {
		facts.Blockers = append(facts.Blockers, "conflict")
	}

	// 予約キーは Chief of Staff が書いたものだけを信じる（log.jsonl の by）
	s := ledger.SettingsOf(t)
	switch {
	case s.Review == "" || s.MustFix == "":
		facts.SettingsReason = "issue.yaml の review・must_fix が無い・知らない値（Chief of Staff が kanban set する）"
	case !l.Trusted(rows, t, "review") || !l.Trusted(rows, t, "must_fix"):
		facts.SettingsReason = "issue.yaml の review・must_fix を最後に書いたのが Chief of Staff（config.yaml の chief_of_staff）でない・記録と値が違う"
	}

	rt, known := table.Repos[repo]
	facts.RepoKnown = known
	if screen := table.ScreenFiles(rt, files); len(screen) > 0 && !HasAttachment(pv.texts()) {
		facts.ScreenMissing = screen
	}
	cl, err := table.Classify(rt, files, string(diff), s.Review == "on")
	if err != nil {
		return nil, "", err
	}
	cl, rec.Acceptance = FromReport(cl, rep)
	// 受け入れ条件の行が記録を指すときだけ中身を見る。古い commit の記録は戻さず、記録に出す（ADR-0035 決めたこと 2・11）
	facts.RunProblems, rec.Acceptance.StaleRuns = contract.CheckRuns(l.Library(id), rep.Acceptance, head)
	tier := cl.Tier
	facts.Tier, facts.AcceptanceFail = tier, rec.Acceptance.Fail
	rec.Tier, rec.TierReasons, rec.ChangedLines = tier, cl.TierReasons, cl.ChangedLines

	d := delegate.Read(ws, l.Slug, time.Now().UTC().Format("2006-01-02"))
	facts.MergeLevel, rec.Delegation.Grant = Delegated(d, id, rep)
	rec.Delegation.Merge, rec.Delegation.OK = facts.MergeLevel, Within(tier, facts.MergeLevel)
	if !rec.Delegation.OK || Within(tier, d.Merge()) {
		rec.Delegation.Grant = "" // 名指しがレベルを通したときだけ残す（プロジェクト全体の値で通るなら名指しは要らなかった）
	}

	// files・diff は head と別に取っている。取り直して動いていれば判定しない。
	// GitHub の checks は head と同じ1回で読む（判定する head の checks だけを見る）
	// 既定ブランチとセルの gh アカウントも同じ GraphQL で読む（変わらない値に REST の上限を使わない）
	cr, err := readChecks(number, slug)
	if err != nil {
		return nil, "", err
	}
	after, checks := cr.Head, cr.Checks
	if after != head {
		return nil, fmt.Sprintf("判定の途中で PR の head が動いた（%s → %s）。取り直す", head, after), nil
	}
	switch {
	case stacked:
		facts.StackedBaseOK = pv.BaseRefName == cr.DefaultBranch || pv.BaseRefName == prevBranch
	case pv.BaseRefName != cr.DefaultBranch:
		facts.Blockers = append(facts.Blockers, "base が既定ブランチでない")
	}
	if prevDone {
		// 前の課題が done: その PR が既定の branch にマージされていれば載せ替えを頼み、そうでなければマージされずに終わったと読む
		// （既定でない base へのマージは main に入ったと読まない。ADR-0067 決めたこと 12・14・15）
		in, err := prevInMain(l, facts.StackedOn, cr.DefaultBranch)
		if err != nil {
			return nil, "", err
		}
		facts.PrevMerged, facts.PrevGone = in, !in
	}
	if pv.Author.Login != cr.Viewer {
		facts.Blockers = append(facts.Blockers, "作者がセルの gh アカウントでない")
	}
	// 記録の at は GitHub の checks を読んだ時刻（判定の事実のうち最後に外から読んだもの）。
	// gate を始めた時刻にすると、その後に通った checks を前もって通ったように見せる（P7・v0.0.3）
	now := time.Now().UTC()
	rec.At = now.Format("2006-01-02T15:04:05Z")
	rec.GitHubChecks = checks
	facts.GitHubChecks, facts.ChecksOptional = checks, rt.GitHubChecks == "optional"
	facts.ChecksWait = *table.GitHubChecksWait
	facts.ChecksOverdue = ChecksOverdue(rows, id, now, facts.ChecksWait)

	facts.ReviewRequired = ReviewRequired(tier, s.Review)
	rec.Review = ledger.Review{State: "off", Waived: len(rep.Waived)}
	// 委譲の外（オーナーがマージする変更）でも条件表の検査を回す。委譲は決定式の最後に見る（ADR-0056 決めたこと 4）
	gated := len(facts.LedgerReasons) == 0 && facts.PROpen && len(facts.Blockers) == 0 && facts.SettingsReason == "" && known
	if stacked {
		// 積んだ課題では、オーナーへ回す理由があっても判定を止めない（Decide が写す）。担当へ返す理由を拾うため、検査も回す
		// 載せ替えを頼むときは、載せ替えたあとの head で見るので検査を回さない（ADR-0067 決めたこと 12）
		gated = len(facts.LedgerReasons) == 0 && facts.PROpen && facts.StackedBaseOK && facts.StackedDraft && known && !Restacking(facts)
	}
	if facts.ReviewRequired {
		tasks, err := l.List()
		if err != nil {
			return nil, "", err
		}
		rec.Review = l.CheckReview(rows, tasks, t, head, s.MustFix, rep.Waived, AddedLines(string(diff)))
	}
	facts.Review = rec.Review
	if gated && (!facts.ReviewRequired || rec.Review.State == "current") && checks.Final(facts.ChecksOptional, facts.ChecksOverdue) {
		rec.Checks, err = runChecks(repo, head, rt.Check)
		if err != nil {
			return nil, "", err
		}
	}
	facts.Checks = rec.Checks
	if gated && !stacked {
		// 古い base: PR の分かれ目より後に既定ブランチで検査の置き場が変わっていれば、この PR の checks は今の検査を通っていない。
		// 条件表の検査（分単位）のあとに読み、判定からマージまでの間に既定ブランチが動く窓を狭める
		moved, ahead, unread, err := baseFiles(slug, head, pv.BaseRefName)
		if err != nil {
			return nil, "", err
		}
		facts.StaleBase, facts.BaseUnread = CheckPlaceFiles(rt, moved), unread
		facts.OwnChecks, facts.BaseAhead = CheckPlaceFiles(rt, files), ahead
	}
	path := filepath.Join(l.Library(id), "gate", head+".json")
	if prev, err := os.ReadFile(path); err == nil {
		var p Record
		if json.Unmarshal(prev, &p) == nil {
			if !dry && written(path, began) {
				// 同時に打たれたほかの run が、この run が lock を待つ間に同じ head を記録した（dispatcher の巡と手の andon gate）。
				// 同じ事実から決めた記録なので、それを返す（「同じ head で2度目の Must fix」に数えない・PR に二重に付けない）
				return &p, "", nil
			}
			if p.Decision == "fix" && p.Review.MustFixOpen > 0 {
				facts.PreviousMustFix = true
			}
		}
	}
	rec.Decision, rec.Reasons = Decide(facts)
	rec.Restack = rec.Decision == "fix" && Restacking(facts)
	if dry || rec.Decision == "wait" {
		return rec, "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", err
	}
	if err := contract.Write(path, contract.SchemaGate, rec); err != nil {
		return nil, "", err
	}
	postGitHub(slug, number, rec, cr.Mark, cr.Link, stderr)
	return rec, "", nil
}

func written(path string, began time.Time) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.ModTime().Before(began)
}

// CheckPlaces は検査の置き場の glob（条件表の glob と同じ書き方）。ここが変わると、それより前の base で通した checks は今の検査を通っていない。
// gate は既定ブランチでここが変わったあとの PR を、rebase して checks を回し直すまで通さない（StaleBaseReason）。
// 検査の置き場の定義はここだけに置く（ほかの command も CheckPlaceFiles・IsCheckPlace で読む）。
// Go の検査（*_test.go）も入れる: 既定ブランチに足した Go の検査は、それより前の base の PR が変えたコードを見ていない。
// tray の検査（job tray が回す Workers の検査・ブラウザの検査・端から端の検査と、どれを回すかを決める設定）も入れる。
// package.json は job tray が打つ検査の command（scripts）を持つので入れる。tray のコードと bun.lock は検査の置き場でない。
var CheckPlaces = []string{"tests/**", ".github/workflows/**", ".claude/skills/verify-andon/**", "**/*_test.go",
	"tray/test/**", "tray/browser/**", "tray/scripts/e2e.mjs", "tray/vitest.config.ts", "tray/playwright.config.ts", "tray/package.json"}

func IsCheckPlace(path string) bool { return matchAny(CheckPlaces, path) }

// CheckPlaceFiles は files のうち検査の置き場に当たるパス（純粋関数。strip_prefix を外し、rename の元も見る。files の順で重ねない）。
func CheckPlaceFiles(repo RepoTable, files []File) []string {
	hit := []string{}
	for _, f := range files {
		ps := []string{strings.TrimPrefix(f.Path, repo.StripPrefix)}
		if f.PreviousPath != "" && f.PreviousPath != f.Path {
			ps = append(ps, strings.TrimPrefix(f.PreviousPath, repo.StripPrefix))
		}
		for _, p := range ps {
			if IsCheckPlace(p) && !slices.Contains(hit, p) {
				hit = append(hit, p)
			}
		}
	}
	return hit
}

// compareFilesLimit は GitHub の compare が返すファイルの上限。これだけ返れば、一覧が切れているかもしれない。
const compareFilesLimit = 300

// compareJq は compare の応答から読む欄だけを残す（差分の本文を運ばない）。
const compareJq = `{ahead_by, files: (if .files == null then null else [.files[] | {filename, previous_filename}] end)}`

// baseFiles は、PR の head と base の分かれ目（merge-base）から base の先頭までに、base の側で変わったファイル（GitHub の compare）。
// unread は一覧が上限で切れたかもしれない（読み切れない）とき。
func baseFiles(slug, head, base string) ([]File, int, bool, error) {
	b, err := gh("api", "repos/"+slug+"/compare/"+head+"..."+base, "--jq", compareJq)
	if err != nil {
		return nil, 0, false, err
	}
	return ParseCompare(b)
}

// ParseCompare の ahead は分かれ目より後に既定ブランチにだけある commit の数。
func ParseCompare(b []byte) ([]File, int, bool, error) {
	var v struct {
		AheadBy *int `json:"ahead_by"`
		Files   *[]struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
		} `json:"files"`
	}
	if json.Unmarshal(b, &v) != nil || v.AheadBy == nil || *v.AheadBy < 0 {
		return nil, 0, false, errors.New("既定ブランチとの比べ（compare）が読めない")
	}
	files := []File{}
	if *v.AheadBy == 0 {
		return files, 0, false, nil
	}
	if v.Files == nil {
		// 既定ブランチが進んだのにファイルの一覧が無い: 変わっていないと読まない（読めないとして判定しない）
		return nil, 0, false, errors.New("既定ブランチとの比べ（compare）にファイルの一覧が無い")
	}
	for _, f := range *v.Files {
		if f.Filename == "" {
			return nil, 0, false, errors.New("既定ブランチとの比べ（compare）のファイルに名前が無い")
		}
		files = append(files, File{Path: f.Filename, PreviousPath: f.PreviousFilename})
	}
	return files, *v.AheadBy, len(*v.Files) >= compareFilesLimit, nil
}

// prevInMain は、積んだ課題の前の課題 prev（done）の PR が既定の branch def にマージされたか（ADR-0067 決めたこと 12・15）。
// PR の場所は前の課題の report.json の pr だけ。PR が無い・読めない report.json は、マージされずに終わったと読む。
// GitHub に聞けなければ誤り（判定しない。dispatcher が前の PR 待ちのまま置き、次の巡で回し直す）。
func prevInMain(l *ledger.Ledger, prev, def string) (bool, error) {
	rep, err := contract.ReadReport(filepath.Join(l.Library(prev), "report.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // PR を出さずに終わった
	}
	if err != nil {
		// 在るのに読めない報告を、マージされずに終わったと読まない（オーナーへやめるかを聞かせてしまう）
		return false, fmt.Errorf("積んだ課題の前の課題 %s の report.json が読めない: %v", prev, err)
	}
	if rep.PR == "" {
		return false, nil
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		return false, fmt.Errorf("積んだ課題の前の課題 %s の report.json の pr が PR の URL でない: %q", prev, rep.PR)
	}
	b, err := gh("pr", "view", number, "--repo", owner+"/"+name, "--json", "state,baseRefName")
	if err != nil {
		return false, fmt.Errorf("積んだ課題の前の課題 %s の PR が読めない: %v", prev, err)
	}
	var v struct {
		State       string `json:"state"`
		BaseRefName string `json:"baseRefName"`
	}
	if json.Unmarshal(b, &v) != nil || v.State == "" {
		return false, fmt.Errorf("積んだ課題の前の課題 %s の PR の状態が読めない", prev)
	}
	return v.State == "MERGED" && v.BaseRefName == def, nil
}

// stackedFrom は、積んだ課題の差分を数える起点（信じられる stacked_base）。PR の向き先 prBase が前の課題（stacked_on）の branch で
// なければ空（PR の差分で数える）。gate とレビューが要るかの判定（NeedsReview）は、この1つで決める（ADR-0067 決めたこと 9）。
func stackedFrom(l *ledger.Ledger, rows []ledger.Row, t ledger.Issue, prBase string) string {
	on, base, ok := ledger.Stacked(rows, t, l.ChiefOfStaff())
	if !ok {
		return ""
	}
	if p, err := l.Show(on); err == nil && p.Str("branch") != "" && p.Str("branch") == prBase {
		return base
	}
	return ""
}

// prChanges は PR の変更したファイルと差分。ふつうは GitHub の PR の files と `gh pr diff` で取り、from（積んだ課題の分かれ目。
// 信じられる stacked_base。ledger.Stacked）が空でなければ、from から head までで取る（StackedDiff。ADR-0067 決めたこと 9）。
// gate とレビューが要るかの判定（NeedsReview）は、この1つで差分を取る。
func prChanges(from string, t ledger.Issue, slug, number, head string) ([]File, []byte, error) {
	if from != "" {
		return StackedDiff(t.Str("repo"), from, head)
	}
	files, err := PRFiles(slug, number)
	if err != nil {
		return nil, nil, err
	}
	diff, err := gh("pr", "diff", number, "--repo", slug)
	if err != nil {
		return nil, nil, err
	}
	return files, diff, nil
}

// PRFiles は PR の変更されたファイル（REST を全ページ。GraphQL の files は100件で切れ、rename 元も無い）。
func PRFiles(slug, number string) ([]File, error) {
	b, err := gh("api", "--paginate", "--slurp", "repos/"+slug+"/pulls/"+number+"/files")
	if err != nil {
		return nil, err
	}
	var pages [][]struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
		Additions        int    `json:"additions"`
		Deletions        int    `json:"deletions"`
	}
	if err := json.Unmarshal(b, &pages); err != nil {
		return nil, errors.New("PR の files が読めない")
	}
	files := []File{}
	for _, p := range pages {
		for _, f := range p {
			files = append(files, File{f.Filename, f.PreviousFilename, f.Additions, f.Deletions})
		}
	}
	return files, nil
}

// runChecks は条件表の check を PR の head の一時 worktree で順に回す（最初の失敗で止める）。
// clone は GHQ_ROOT（無ければ ghq root）の下の repo。
func runChecks(repo, head string, cmds []string) ([]Check, error) {
	checks := []Check{}
	if len(cmds) == 0 {
		return checks, nil
	}
	dir, err := spawn.RepoPath(repo)
	if err != nil {
		return nil, err
	}
	scratch, err := os.MkdirTemp("", "andon-gate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch) //nolint:errcheck // 一時の置き場の片付けは試すだけ
	wt := filepath.Join(scratch, "worktree")
	if out, err := exec.Command("git", "-C", dir, "fetch", "-q", "https://"+repo, head).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("PR の head の fetch に失敗: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("git", "-C", dir, "worktree", "add", "-q", "--detach", wt, head).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("一時 worktree を作れない: %s", strings.TrimSpace(string(out)))
	}
	defer exec.Command("git", "-C", dir, "worktree", "remove", "--force", wt).Run() //nolint:errcheck // 一時 worktree の片付けは試すだけ
	tmp := filepath.Join(scratch, "tmp")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return nil, err
	}
	// 宣言を満たす: 検査の command の前に、一時の worktree の mise.toml の道具を入れる（ADR-0063 決めたこと 8）。
	// 満たせなければ検査を打たずに、PR の head の fetch に失敗したときと同じく判定せずに返す（道具が無いことを PR の欠陥として返さない。
	// 返った失敗を dispatcher は、ほかの判定できない回と同じく gate の問いにする）
	if _, err := tools.Install(wt); err != nil {
		return nil, fmt.Errorf("検査の道具を入れられない（%s の mise install。判定しない）: %v", tools.Manifest, err)
	}
	env := append(append(os.Environ(), "TMPDIR="+tmp), tools.EnvFor(wt)...)
	for _, cmd := range cmds {
		c := exec.Command("bash", "-e", "-o", "pipefail", "-c", cmd)
		c.Dir, c.Env = wt, env
		ok := c.Run() == nil
		checks = append(checks, Check{Cmd: cmd, OK: ok})
		if !ok {
			break
		}
	}
	return checks, nil
}

// checksQuery は PR の head の checks と一緒に、gh のアカウント（viewer）と repo の既定ブランチも読む（gate が作者と base を見る）。
const checksQuery = `query($owner: String!, $name: String!, $number: Int!, $after: String) {
  viewer { login }
  repository(owner: $owner, name: $name) { defaultBranchRef { name } pullRequest(number: $number) { headRefOid
    commits(last: 1) { nodes { commit { oid statusCheckRollup { contexts(first: 100, after: $after) {
      pageInfo { hasNextPage endCursor }
      nodes { __typename
        ... on CheckRun { name status conclusion databaseId checkSuite { app { slug } workflowRun { workflow { id } } } }
        ... on StatusContext { context state createdAt description targetUrl } } } } } } } } } }`

// checksPages は読むページの上限（100件ずつ。超えたら読めないとして止める）。
const checksPages = 50

// ReadChecks は PR の head と、その head の GitHub の checks を全ページ読む（dispatcher がマージの直前と checks 待ちに使う）。
// ページの間に head が動いたら、動いた先の head を返す（呼び手は自分の head と比べて止める）。
func ReadChecks(number, slug string) (string, GitHubChecks, error) {
	cr, err := readChecks(number, slug)
	return cr.Head, cr.Checks, err
}

type checksRead struct {
	Head          string
	Checks        GitHubChecks
	Mark          Mark
	Link          string
	DefaultBranch string
	Viewer        string
}

func readChecks(number, slug string) (checksRead, error) {
	var cr checksRead
	owner, name, _ := strings.Cut(slug, "/")
	head, after := "", ""
	var items []CheckItem
	for page := 0; ; page++ {
		if page == checksPages {
			return checksRead{}, fmt.Errorf("GitHub の checks が %d 件を超えて読みきれない", checksPages*100)
		}
		args := []string{"api", "graphql", "-f", "query=" + checksQuery, "-f", "owner=" + owner, "-f", "name=" + name, "-F", "number=" + number}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		b, err := gh(args...)
		if err != nil {
			return checksRead{}, err
		}
		var v struct {
			Data struct {
				Viewer struct {
					Login string `json:"login"`
				} `json:"viewer"`
				Repository struct {
					DefaultBranchRef *struct {
						Name string `json:"name"`
					} `json:"defaultBranchRef"`
					PullRequest struct {
						HeadRefOid string `json:"headRefOid"`
						Commits    struct {
							Nodes []struct {
								Commit struct {
									Oid               string `json:"oid"`
									StatusCheckRollup *struct {
										Contexts struct {
											PageInfo struct {
												HasNextPage bool   `json:"hasNextPage"`
												EndCursor   string `json:"endCursor"`
											} `json:"pageInfo"`
											Nodes []struct {
												CheckItem
												CheckSuite struct {
													App         struct{ Slug string } `json:"app"`
													WorkflowRun *struct {
														Workflow struct{ ID string } `json:"workflow"`
													} `json:"workflowRun"`
												} `json:"checkSuite"`
											} `json:"nodes"`
										} `json:"contexts"`
									} `json:"statusCheckRollup"`
								} `json:"commit"`
							} `json:"nodes"`
						} `json:"commits"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}
		pr := &v.Data.Repository.PullRequest
		if json.Unmarshal(b, &v) != nil || !contract.IsSHA(pr.HeadRefOid) || len(pr.Commits.Nodes) != 1 || pr.Commits.Nodes[0].Commit.Oid != pr.HeadRefOid {
			return checksRead{}, errors.New("PR の head と checks が読めない")
		}
		if v.Data.Viewer.Login == "" || v.Data.Repository.DefaultBranchRef == nil || v.Data.Repository.DefaultBranchRef.Name == "" {
			return checksRead{}, errors.New("gh のアカウントと repo の既定ブランチが読めない")
		}
		cr.Viewer, cr.DefaultBranch = v.Data.Viewer.Login, v.Data.Repository.DefaultBranchRef.Name
		if head != "" && pr.HeadRefOid != head {
			cr.Head = pr.HeadRefOid
			return cr, nil // 読んでいる間に head が動いた
		}
		head = pr.HeadRefOid
		roll := pr.Commits.Nodes[0].Commit.StatusCheckRollup
		if roll == nil {
			break // checks が1つも無い
		}
		for _, n := range roll.Contexts.Nodes {
			it := n.CheckItem
			it.App = n.CheckSuite.App.Slug
			if n.CheckSuite.WorkflowRun != nil {
				it.Workflow = n.CheckSuite.WorkflowRun.Workflow.ID
			}
			items = append(items, it)
		}
		if !roll.Contexts.PageInfo.HasNextPage {
			break
		}
		if after = roll.Contexts.PageInfo.EndCursor; after == "" {
			return checksRead{}, errors.New("GitHub の checks の続きのページが読めない")
		}
	}
	cr.Head, cr.Checks, cr.Mark, cr.Link = head, SummarizeChecks(items), GateMark(items), GateLink(items)
	return cr, nil
}

// postGitHub は記録を PR のコメントと commit status（andon/gate）に写す。失敗しても判定は変えない。
// コメントを先に書き、gh pr comment が出したコメントの URL を印のリンク（target_url）にする（ADR-0054 決めたこと 6。
// コメントを書けなかったときはリンクの無い印）。印は通したときとオーナー待ちは緑、差し戻しは赤（ADR-0056 決めたこと 3）。
// head に同じ状態と説明文の印が既にあれば（have。判定し直しても同じ印）、印は書き直さない。ただし今の印に
// リンクが無く（haveLink が空。前のバージョンの印か、コメントを書けなかった回の印）、今回コメントの URL を得たときは、リンクを付けて書き直す。
// 写す文は外の人が読む文（comment.go・mark.go）で、記録の文をそのまま写さない（ADR-0026 決めたこと 9）。
func postGitHub(slug, number string, rec *Record, have Mark, haveLink string, stderr io.Writer) {
	url := ""
	if out, err := gh("pr", "comment", number, "--repo", slug, "--body", CommentBody(rec)); err != nil {
		fmt.Fprintln(stderr, "andon gate: コメントの記録に失敗:", err)
	} else if u := strings.TrimSpace(string(out)); strings.HasPrefix(u, "https://") && !strings.ContainsAny(u, " \n") {
		url = u
	}
	mark, ok := DecisionMark(rec.Decision, rec.Reasons)
	if !ok || (Same(mark, have, false) && (url == "" || haveLink != "")) {
		return
	}
	args := []string{"api", "-X", "POST", "repos/" + slug + "/statuses/" + rec.Head, "-f", "context=" + GateContext, "-f", "state=" + mark.State, "-f", "description=" + mark.Description}
	if url != "" {
		args = append(args, "-f", "target_url="+url)
	}
	if _, err := gh(args...); err != nil {
		fmt.Fprintln(stderr, "andon gate: commit status の記録に失敗:", err)
	}
}
