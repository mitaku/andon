// Package dispatcher は andon dispatcher: セルに1つ常駐し、台帳を周期で回ってインナーループの決まった次の工程を起動する
// （docs/dispatcher.md §5 の P6a）。判断はしない。判断はゲート（andon gate）・Chief of Staff・人が持つ。
//
//	report.json の review_request   → レビュアーを起動する（kanban add --parent ＋ detached worktree ＋ hc run）
//	{"status": "review"} に入った    → andon gate を回す（deliverable: candidates の課題は回さない。通知だけ）
//	approve ＋ merge: auto ＋ 委譲    → マージ → andon issue finish（レビュアーの課題も片付ける）
//	fix                             → 理由を担当へ hc send
//	needs_human                     → 問い（from: gate）を出す
//	review の PR を人がマージした      → ゲートの問いを閉じて andon issue finish（1分に1回だけ gh pr view。humanmerge.go）
//	{"status": "done"} の行            → library/corrections.json を1回だけ書く（人の手直し。corrections.go）
//	担当の端末に first_message.sent=false → 入力を受けられれば最初のメッセージを送り直す（pickup.go の sendFirst）
//	担当・Chief of Staff がプロバイダに切られた → 1・2・4・8 分あとに「続けて」、5 回目に切られたら知らせる（pickup.go。§2.4）
//	project.json の signals           → andon signal collect --due（組ごとに every 分に1回）。届けていない now の信号を
//	                                     通知 signal で積み、andon signal delivered で届けたと書く（signals.go。§2.2.1）
//
// 通知の元（pickup.go。dispatcher.md §2.2・§2.3）: log.jsonl の続き・ready の差・doing の担当の端末の状態を拾って積む。
// 通知（contract.md §6）は控えに JSON で積む。Chief of Staff に届けるのはフック（drain.go。経路①②）だけで、
// 離席中は Stop フックの待ち役（wait.go。経路③）が合図で Chief of Staff を起動する。dispatcher は入力欄へ <news> を打たない。dispatcher.md §3。届けない kind は記録だけ（route.go の表。§3.2）。
// 台帳に書くのは台帳の CLI と andon issue … だけで、by は terminal:dispatcher（ledger.DispatcherBy）。
// 例外は dispatcher が書き手の library/ のファイル（corrections.json。contract.md §1）。ゲートとマージは andon issue merge（loop.go の merge）。
package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // 実行環境に zoneinfo が無くても台帳の timezone を引けるようにする

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/flock"
	"andon/internal/ledger"
)

// Usage は andon dispatcher --help の中身。
const Usage = "usage: andon dispatcher [--project <slug>]          回り続ける（hc の端末 dispatcher の中で動く）\n" +
	"       andon dispatcher --once [--project <slug>]   1回だけ回して終わる\n" +
	"       andon dispatcher drain [--context] [--project <slug>]   待ちの通知を取り出す（Chief of Staff のフックが打つ）\n" +
	"       andon dispatcher wait [--timeout <秒>] [--project <slug>]   待ち役（Chief of Staff の Stop フックが asyncRewake で起動する）\n" +
	"       andon dispatcher rewake [--project <slug>]   プロバイダに切られた Chief of Staff を起動する（StopFailure フックが asyncRewake で起動する）\n" +
	"セル（ANDON_WORKSPACE。無ければ /workspace）の projects/*/kanban を回り、レビュアーを起動する・ゲートを回す・\n" +
	"merge: auto をマージして片付ける・fix を担当へ返す・needs_human を問いにする。\n" +
	"待ちの通知は Chief of Staff のフック（drain）が届ける。離席中は待ち役（wait）が合図で Chief of Staff を起動する（入力欄へは打たない）。\n" +
	"Chief of Staff（config.yaml の chief_of_staff）の端末が生きていない間（留守）も回し、配る段だけ止める（戻った最初の <news> に留守の1行）。\n" +
	"chief_of_staff が設定に無い台帳は飛ばす。初めて見る台帳は通知の元を控えるだけ（今の review_request は拾う）。\n" +
	"控えは ~/.local/state/andon/dispatcher/<slug>.json（XDG_STATE_HOME があればその下）。届けない通知は横の <slug>.recorded.jsonl に1行ずつ。\n"

// Name は dispatcher の端末の名前で、台帳の CLI を打つときの HC_TERMINAL_ID（by は terminal:dispatcher）。
const Name = "dispatcher"

const (
	exitFail  = 1
	exitUsage = 2
)

// Main は andon dispatcher。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	once, only := false, ""
	if len(args) > 0 && args[0] == "drain" {
		return Drain(args[1:], stdin, stdout, stderr)
	}
	if len(args) > 0 && args[0] == "wait" {
		return WaitMain(args[1:], stdin, stderr)
	}
	if len(args) > 0 && args[0] == "rewake" {
		return RewakeMain(args[1:], stdin, stderr)
	}
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--once":
			once = true
		case args[i] == "--project" && i+1 < len(args):
			only = args[i+1]
			i++
		default:
			fmt.Fprint(stderr, Usage)
			return exitUsage
		}
	}
	if err := ledger.Check(); err != nil {
		fmt.Fprintf(stderr, "andon dispatcher: %v\n", err)
		return exitFail
	}
	for _, c := range []string{"hc", "gh", "git"} {
		if _, err := exec.LookPath(c); err != nil {
			fmt.Fprintf(stderr, "andon dispatcher: %s が無い\n", c)
			return exitFail
		}
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "andon dispatcher: 自分のパスが分からない: %v\n", err)
		return exitFail
	}
	// 1つのセルに dispatcher は1本（G1 を塞いだ。dispatcher.md §7.2 #1）。lock はプロセスが終わるまで持つ
	unlock, err := flock.TryLock(SingletonLock())
	if errors.Is(err, flock.ErrBusy) {
		fmt.Fprintln(stderr, "andon dispatcher: ほかの andon dispatcher が動いているので終わる（"+SingletonLock()+"）")
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon dispatcher: %v\n", err)
		return exitFail
	}
	defer unlock()
	c := &Dispatcher{WS: delegate.Workspace(), Andon: self, Log: stderr}
	for {
		// 見回りの間は PassLock を持つ（Ensure が置き直すのは合間だけ）
		unpass, err := flock.Lock(PassLock())
		if err != nil {
			fmt.Fprintf(stderr, "andon dispatcher: %v\n", err)
			return exitFail
		}
		busy := c.Pass(only)
		unpass()
		if once {
			return 0
		}
		// 待ちが無ければ1分ごと、待ちがあれば5秒ごと（dispatcher.md §2.1）
		if busy {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(time.Minute)
		}
	}
}

// SingletonLock は andon dispatcher（巡を回す本体）が動いている間持つ lock のパス（控えの置き場の .dispatcher.lock）。
func SingletonLock() string {
	return filepath.Join(filepath.Dir(StatePath("x")), ".dispatcher.lock")
}

// PassLock は andon dispatcher が1回の見回り（Pass）の間だけ持つ lock のパス。Ensure は取れたときだけ置き直す。
func PassLock() string {
	return filepath.Join(filepath.Dir(StatePath("x")), ".pass.lock")
}

// Dispatcher は1つのセルの dispatcher。
type Dispatcher struct {
	WS    string           // セルの repo
	Andon string           // andon のバイナリ（gate・issue を打つ）
	Log   io.Writer        // ログ（1行ずつ。頭に時刻）
	Now   func() time.Time // ログの時刻（無ければ time.Now。検査用）

	unwaited map[string]string // 待ち役がいないとログに書いた待ち（台帳ごと。同じ待ちを二度書かない）
	skip     map[string]string // 台帳を飛ばしている・留守だとログに書いた理由（台帳ごと。同じ理由を二度書かない）
}

// cosDown は Chief of Staff の端末が無い（留守）理由。hc が端末を無い（not_found）・live でないと答えたときだけ返す。
// hc get が一時に落ちた（ほかのエラー・形が読めない）ときは unsure に理由を返し、留守にしない（取り違えて留守の記録を作らない）。
func cosDown(cos string) (reason, unsure string) {
	cmd := exec.Command("hc", "get", cos, "--json")
	cmd.Env = withoutTerminal()
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		var e struct {
			Error struct {
				Kind string `json:"kind"`
			} `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(errb.Bytes()), &e) == nil && e.Error.Kind == "not_found" {
			return "端末 " + cos + " が無い", ""
		}
		return "", fmt.Sprintf("hc get %s: %v %s", cos, err, strings.TrimSpace(errb.String()))
	}
	var t struct {
		State string `json:"state"`
	}
	if json.Unmarshal(out, &t) != nil || t.State == "" {
		return "", "hc get " + cos + " の形が読めない"
	}
	if t.State != "live" {
		return "端末 " + cos + " が live でない（" + t.State + "）", ""
	}
	return "", ""
}

// skipped は台帳を丸ごと飛ばし始めたとき（chief_of_staff が設定に無い）ログに1行と、人に気づける通知を1つ出す。
// 同じ理由が続く間は出さない。
func (p *project) skipped(reason string) {
	if p.c.skip[p.l.Slug] == reason {
		return
	}
	p.c.mark(p.l.Slug, reason)
	p.logf("台帳を飛ばす（%s。レビュアーも人のマージも拾わない）", reason)
	cmd := exec.Command("herdr", "notification", "show", "andon dispatcher: "+p.l.Slug+" を飛ばしている", "--body", reason)
	cmd.Env = withoutTerminal()
	_ = cmd.Run() // herdr が無い・失敗でも巡は続ける（ログの1行は出ている）
}

// absent は Chief of Staff の留守に入ったとき（理由が変わったときも）ログに1行だけ出す（ADR-0017 決めたこと 8）。
func (p *project) absent(cos, reason string) {
	if p.c.skip[p.l.Slug] == reason {
		return
	}
	p.c.mark(p.l.Slug, reason)
	p.logf("Chief of Staff の留守（宛先 %q: %s）。配る段だけ止め、拾う・コマンドを打つ段は回す。届ける通知は待ちに積む", cos, reason)
}

func (c *Dispatcher) mark(slug, reason string) {
	if c.skip == nil {
		c.skip = map[string]string{}
	}
	c.skip[slug] = reason
}

func (c *Dispatcher) logf(format string, a ...any) { c.logIn(time.UTC, format, a...) }

// logIn は行の頭に秒までの時刻（RFC 3339。loc の時刻）を付けて書く。機械が読む記録（控え・log.jsonl）は変えない。
func (c *Dispatcher) logIn(loc *time.Location, format string, a ...any) {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	fmt.Fprintf(c.Log, now().In(loc).Format(time.RFC3339)+" andon dispatcher: "+format+"\n", a...)
}

// location は台帳の config.yaml の timezone（無い・読めなければ UTC）。
func (l *project) location() *time.Location {
	if tz := l.l.Timezone(); tz != "" && tz != "Local" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

// Pass はセルの台帳を1回ずつ回す。動いている課題（doing・review）があれば busy。
func (c *Dispatcher) Pass(only string) bool {
	cfgs, _ := filepath.Glob(filepath.Join(c.WS, "projects", "*", "kanban", "config.yaml"))
	sort.Strings(cfgs)
	busy := false
	for _, cfg := range cfgs {
		root := filepath.Dir(filepath.Dir(cfg))
		if only != "" && filepath.Base(root) != only {
			continue
		}
		p := &project{c: c, l: ledger.At(root)}
		p.l.Env = []string{"HC_TERMINAL_ID=" + Name}
		b, err := p.pass()
		if err != nil {
			p.logf("%v", err)
		}
		busy = busy || b
	}
	return busy
}

// project は1つの台帳を1回回す間のもの。
type project struct {
	c     *Dispatcher
	l     *ledger.Ledger
	st    *State
	first bool
	away  bool // Chief of Staff の端末が無い（留守）。配る段だけ飛ばす（ADR-0017 決めたこと 8）
	rows  []ledger.Row
	tasks []ledger.Issue
}

func (p *project) logf(format string, a ...any) {
	p.c.logIn(p.location(), p.l.Slug+": "+format, a...)
}

// pass の段の順は docs/dispatcher.md §2.1.1 の表で固定している。変えるときは表も直す。
func (p *project) pass() (bool, error) {
	cos := p.l.ChiefOfStaff()
	if cos == "" {
		p.skipped("config.yaml の chief_of_staff が空") // 留守と区別する（ADR-0017 決めたこと 8 の (a)）
		return false, nil
	}
	switch reason, unsure := cosDown(cos); {
	case reason != "":
		p.away = true
		p.absent(cos, reason) // 配る段だけ飛ばす（dispatcher.md §2.1）
	case unsure != "":
		// 留守かを確かめられない（hc get の一時の失敗）: 留守と取り違えないよう、この巡はこの台帳を回さない（次の巡で確かめ直す）。ログは同じ理由に1回
		if p.c.skip[p.l.Slug] != unsure {
			p.c.mark(p.l.Slug, unsure)
			p.logf("Chief of Staff の端末を確かめられない（%s）。この巡はこの台帳を回さない", unsure)
		}
		return false, nil
	default:
		delete(p.c.skip, p.l.Slug)
	}
	unlock, err := lockState(p.l.Slug, 0) // drain（フック）と控えを取り合わない
	if err != nil {
		return false, err
	}
	defer unlock()
	st, first, err := LoadState(p.l.Slug)
	if err != nil {
		return false, err
	}
	p.st, p.first = st, first
	p.markAway(now())
	if p.tasks, err = p.l.List(); err != nil {
		return false, err
	}
	p.rows = p.l.Log()
	p.tick()       // ルーチンの schedule（dispatcher.md §2.1。足した課題は collect の ready の差で拾う）
	p.collect(cos) // 通知の元（dispatcher.md §2.2・§2.3）
	busy := false
	for _, t := range p.tasks {
		status := t.Str("status")
		busy = busy || status == "doing" || status == "review"
		if !p.gated(t) {
			continue
		}
		rep, err := contract.ReadReport(filepath.Join(p.l.Library(t.Str("id")), "report.json"))
		if err != nil {
			continue
		}
		switch status {
		case "doing":
			if rep.ReviewRequest != nil {
				p.reviewRequest(t, rep)
			}
		case "review":
			p.merge(t)           // ゲートとマージと片付け（andon issue merge。dispatcher.md §5 の P6a）
			p.humanMerge(t, rep) // 人が GitHub でマージした（andon issue settle。dispatcher.md §2.7）
		}
	}
	p.impacts()         // マージのあとの影響（dispatcher.md §2.5）
	p.corrections()     // 人の手直し（dispatcher.md §2.6・contract.md §14）
	p.stuck(time.Now()) // 記録だけにした approve のまま止まった（dispatcher.md §3.2）
	if !p.first && !p.away {
		p.unwaited(cos) // 経路③は待ち役。dispatcher は打たず、待ち役がいなければログに1行（dispatcher.md §3・§4）
	}
	return busy || len(p.st.Pending) > 0, p.st.Save(p.l.Slug)
}

// markAway は控えの留守の記録を巡の始めに合わせる（ADR-0017 決めたこと 8 の (b)）。留守に入ったら始まりを書き、
// 戻ったら終わりを書く（消すのは戻った最初の <news> を描いた drain）。留守の間に何も積まなかったら、戻ったときに消す。
// 終わりを書いたあと drain の前にまた留守になったら、同じ留守の続きとして終わりを消す。
func (p *project) markAway(at string) {
	a := p.st.Away
	switch {
	case p.away && a == nil:
		p.st.Away = &Away{Since: at}
	case p.away:
		a.Until = ""
	case a == nil:
	case a.Count == 0:
		p.st.Away = nil
	case a.Until == "":
		a.Until = at
	}
}

// tick は kanban tick を1回打つ（ルーチンの schedule が来たものを課題にする。dispatcher.md §2.1・§7.2 #8）。
// 打つのは dispatcher だけ（cron は置かない）。同じ時刻を二度撃たない守りは台帳の側（kanban/.lock と state/routines.json）。
// 失敗はログに1行で、巡は止めない。前回の ready が無い（初めて見る台帳）ときは、tick の前の ready を先に控える。
// 過去の ready は流さず、tick が足した課題だけを collect の ready の差で拾う（次の巡で届く）。
func (p *project) tick() {
	if p.st.Ready == nil {
		p.readReady()
	}
	if _, err := p.l.Run("", "tick"); err != nil {
		p.logf("kanban tick が通らない（巡は続ける）: %v", err)
	}
}

// gated は、課題がインナーループ（レビュアーを起動する・ゲートを回す）を通るか。通るのは案件 repo の担当の
// PR の課題だけ。レビュアー・Chief Engineer・repo の無い課題・調べる課題（deliverable: candidates。
// Chief of Staff が書いたものだけ信じる）は Chief of Staff が見る（通知は collect が積む）。
func (p *project) gated(t ledger.Issue) bool {
	return !isReviewer(t) && t.Str("profile") != "chief-engineer" && t.Str("repo") != "" && !p.l.Candidates(p.rows, t)
}

func isReviewer(t ledger.Issue) bool {
	return t.Str("parent") != "" && strings.HasSuffix(t.Str("profile"), "-review")
}

// trusted は予約キー key が無い（既定で読む）か、Chief of Staff が最後に書いたものか（contract.md §2 の履歴の照合）。
func (p *project) trusted(t ledger.Issue, key string) bool {
	return t.Str(key) == "" || p.l.Trusted(p.rows, t, key)
}

// notify は通知を扱いの表（route.go）のとおりに、控えに積む（同じ課題の同じ出来事は新しい1件に置き換える）か記録だけにする。
func (p *project) notify(task, kind string, detail map[string]any) {
	p.put(Notification{At: now(), Project: p.l.Slug, Issue: task, Kind: kind, Detail: detail}, p.facts(task))
}

func (p *project) reportPath(id string) string {
	return filepath.Join(p.l.Dir, "issues", id, "report.md")
}

func now() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

// run はコマンドを打ち、stdout と終了コードを返す。err は打てなかったときと 0 以外のとき（stderr の1行目つき）。
func run(dir string, env []string, stdin string, name string, args ...string) (string, int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return string(out), code, &runError{fmt.Sprintf("%s %s: %v %s", name, strings.Join(args[:min(2, len(args))], " "), err, msg), errb.String()}
	}
	return string(out), 0, nil
}

// runError は run の err。Error は stderr の1行目まで、Stderr は stderr の全部。
type runError struct {
	msg    string
	Stderr string
}

func (e *runError) Error() string { return e.msg }

// andon は自分（andon）をこの台帳の根で打つ（by は terminal:dispatcher）。
func (p *project) andon(stdin string, args ...string) (string, int, error) {
	return run(p.l.Root, []string{"KANBAN_ROOT=" + p.l.Root, "HC_TERMINAL_ID=" + Name}, stdin, p.c.Andon, args...)
}

// prHead は PR の head（gh pr view）。
func prHead(pr string) (string, error) {
	out, _, err := run("", nil, "", "gh", "pr", "view", pr, "--json", "headRefOid", "-q", ".headRefOid")
	sha := strings.TrimSpace(out)
	if err != nil || !contract.IsSHA(sha) {
		return "", fmt.Errorf("PR の head が読めない: %s", pr)
	}
	return sha, nil
}

// repoDir は $GHQ_ROOT（無ければ ghq root）/<repo>。
func repoDir(repo string) (string, error) {
	root := os.Getenv("GHQ_ROOT")
	if root == "" {
		out, _, err := run("", nil, "", "ghq", "root")
		if err != nil {
			return "", errors.New("GHQ_ROOT が無い")
		}
		root = strings.TrimSpace(out)
	}
	return filepath.Join(root, repo), nil
}

// ask は問い（contract.md §5）を andon issue ask で出し、通知 asked を積む。
func (p *project) ask(id string, q contract.Question) {
	b, _ := json.Marshal(q)
	out, _, err := p.andon(string(b), "issue", "ask", id, "-")
	if err != nil {
		p.logf("%s の問いを出せない: %v", id, err)
		return
	}
	n := 0
	if m := askedRe.FindStringSubmatch(strings.TrimSpace(out)); m != nil {
		fmt.Sscan(m[1], &n)
	}
	if n < 1 {
		p.logf("%s の問いの番号が読めない: %q", id, out)
		return
	}
	p.notify(id, "asked", map[string]any{"n": n, "kind": q.Kind, "question": q.Question})
}
