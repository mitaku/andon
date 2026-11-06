// Package dispatcher は andon dispatcher: 建屋に1つ常駐し、台帳を周期で回ってインナーループの決まった次の工程を起こす
// （docs/dispatcher.md §5 の P6a）。判断はしない。判断はゲート（andon gate）・Chief of Staff・人が持つ。
//
//	report.json の review_request   → レビュアーを起こす（kanban add --parent ＋ detached worktree ＋ hc run）
//	{"status": "review"} に入った    → andon gate を回す
//	approve ＋ merge: auto ＋ 委譲    → マージ → andon task finish（レビュアーのタスクも片付ける）
//	fix                             → 理由を担当へ hc send
//	needs_human                     → 問い（from: gate）を出す
//	担当の端末に first_message.sent=false → 入力を受けられれば最初のメッセージを送り直す（pickup.go の sendFirst）
//
// 通知の元（pickup.go。dispatcher.md §2.2・§2.3）: log.jsonl の続き・ready の差・doing の担当の端末の状態を拾って積む。
// 通知（contract.md §6）は控えに JSON で積む。Chief of Staff に届けるのはフック（drain.go。経路①②）と、
// 離席時だけの hc send（deliver.go。経路③）。dispatcher.md §3。
// 台帳に書くのは台帳の CLI と andon task … だけで、by は terminal:dispatcher（ledger.DispatcherBy）。
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

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
)

// Usage は andon dispatcher --help の中身。
const Usage = "usage: andon dispatcher [--project <slug>]          回り続ける（hc の端末 dispatcher の中で動く）\n" +
	"       andon dispatcher --once [--project <slug>]   1回だけ回して終わる\n" +
	"       andon dispatcher drain [--context] [--project <slug>]   待ちの通知を取り出す（Chief of Staff のフックが打つ）\n" +
	"建屋（ANDON_WORKSPACE。無ければ /workspace）の projects/*/kanban を回り、レビュアーを起こす・ゲートを回す・\n" +
	"merge: auto をマージして片付ける・fix を担当へ返す・needs_human を問いにする。\n" +
	"待ちの通知は、Chief of Staff がターンを終えて入力を待ち、入力欄が空のときだけ hc send で届ける（ほかはフックの drain）。\n" +
	"Chief of Staff（config.yaml の chief_of_staff）の端末が生きていない台帳は飛ばす。初めて見る台帳は通知の元を控えるだけ（今の review_request は拾う）。\n" +
	"控えは ~/.local/state/andon/dispatcher/<slug>.json（XDG_STATE_HOME があればその下）。\n"

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
	c := &Dispatcher{WS: delegate.Workspace(), Andon: self, Log: stderr}
	for {
		busy := c.Pass(only)
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

// Dispatcher は1つの建屋の dispatcher。
type Dispatcher struct {
	WS    string    // 建屋の repo
	Andon string    // andon のバイナリ（gate・task を打つ）
	Log   io.Writer // ログ（1行ずつ）
}

func (c *Dispatcher) logf(format string, a ...any) {
	fmt.Fprintf(c.Log, "andon dispatcher: "+format+"\n", a...)
}

// Pass は建屋の台帳を1回ずつ回す。動いているタスク（doing・review）があれば busy。
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
			c.logf("%s: %v", p.l.Slug, err)
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
	rows  []ledger.Row
	tasks []ledger.Task
}

func (p *project) logf(format string, a ...any) { p.c.logf(p.l.Slug+": "+format, a...) }

func (p *project) pass() (bool, error) {
	cos := p.l.ChiefOfStaff()
	if cos == "" || exec.Command("hc", "get", cos).Run() != nil {
		return false, nil // Chief of Staff が起きたときに台帳を自分で読む（dispatcher.md §2.1）
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
	if p.tasks, err = p.l.List(); err != nil {
		return false, err
	}
	p.rows = p.l.Log()
	p.collect(cos) // 通知の元（dispatcher.md §2.2・§2.3）
	busy := false
	for _, t := range p.tasks {
		status := t.Str("status")
		busy = busy || status == "doing" || status == "review"
		if isReviewer(t) || t.Str("profile") == "chief-engineer" || t.Str("repo") == "" {
			continue // ゲートを通るのは案件 repo の担当のタスクだけ（レビュアー・Chief Engineer は Chief of Staff が見る）
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
			p.review(t, rep)
		}
	}
	if !p.first {
		p.deliver(cos) // 経路③: 離席時だけ（dispatcher.md §3・§4）
	}
	return busy || len(p.st.Pending) > 0, p.st.Save(p.l.Slug)
}

func isReviewer(t ledger.Task) bool {
	return t.Str("parent") != "" && strings.HasSuffix(t.Str("profile"), "-review")
}

// trusted は予約キー key が無い（既定で読む）か、Chief of Staff が最後に書いたものか（contract.md §2 の履歴の照合）。
func (p *project) trusted(t ledger.Task, key string) bool {
	return t.Str(key) == "" || p.l.Trusted(p.rows, t, key)
}

// notify は通知を控えに積む（同じタスクの同じ出来事は新しい1件に置き換える）。
func (p *project) notify(task, kind string, detail map[string]any) {
	p.st.Push(Notification{At: now(), Project: p.l.Slug, Task: task, Kind: kind, Detail: detail})
}

func (p *project) reportPath(id string) string {
	return filepath.Join(p.l.Dir, "tasks", id, "report.md")
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
		return string(out), code, fmt.Errorf("%s %s: %v %s", name, strings.Join(args[:min(2, len(args))], " "), err, msg)
	}
	return string(out), 0, nil
}

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

// ask は問い（contract.md §5）を andon task ask で出し、通知 asked を積む。
func (p *project) ask(id string, q contract.Question) {
	b, _ := json.Marshal(q)
	out, _, err := p.andon(string(b), "task", "ask", id, "-")
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
