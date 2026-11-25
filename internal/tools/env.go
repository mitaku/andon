package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/delegate"
	"andon/internal/ledger"
)

const EnvUsage = "usage: andon tools env\n" +
	"役の起動の行（package/templates/ の command・resume）が読む。自分の端末（HC_TERMINAL_ID）のラベル project・issue から\n" +
	"課題の kanban の worktree を引き、その根に mise.toml があれば " + EnvKey + "=<根>/mise.toml の 1 行を出す（template が env に渡す）。\n" +
	"mise はこの設定をいちばん弱い既定として読むので、役が repo の外（一時の置き場など）で打つ道具も worktree の mise.toml のバージョンを引く。\n" +
	"課題が引けない・worktree が空のときは、打った所の git の根を見る。根に mise.toml が無ければ何も出さずに終了コード 0。\n" +
	"根のパスに空白の文字があるときは、行を出さずに警告を 1 行出す（template は $(…) を語で割る）。mise は呼ばず、待たない。\n"

var (
	// ラベルの値をパスに組む前の形の確かめ（andon otel env と同じ広さ。.. や / を通さない）
	slugRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	issueRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

type envDeps struct {
	terminal string                                       // 自分の端末の address（HC_TERMINAL_ID）
	labels   func(addr string) (map[string]string, error) // 端末のラベル
	worktree func(slug, id string) (string, error)        // 課題の kanban の worktree（~ を開いたもの）
	gitRoot  func() (string, error)                       // 打った所の git の根
}

func EnvMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, EnvUsage)
		return 2
	}
	d := envDeps{
		terminal: os.Getenv("HC_TERMINAL_ID"),
		labels:   hcLabels,
		worktree: func(slug, id string) (string, error) {
			t, err := ledger.At(filepath.Join(delegate.Workspace(), "projects", slug)).Show(id)
			if err != nil {
				return "", err
			}
			return t.Path("worktree"), nil
		},
		gitRoot: func() (string, error) {
			out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
			return strings.TrimSpace(string(out)), err
		},
	}
	line, warn := d.env()
	if warn != "" {
		fmt.Fprintf(stderr, "andon tools env: %s\n", warn)
	}
	if line != "" {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// env は出す 1 行（出さなければ空）と、stderr に出す警告（無ければ空）を返す。
func (d envDeps) env() (line, warn string) {
	root := d.root()
	if !HasManifest(root) {
		return "", ""
	}
	p := filepath.Join(root, Manifest)
	if strings.ContainsAny(p, " \t\n\r\v\f") {
		return "", fmt.Sprintf("%s のパスに空白の文字があるので %s を渡さない（template が語で割る）", p, EnvKey)
	}
	return EnvKey + "=" + p, ""
}

// root は端末の課題の worktree の根。課題が引けない・worktree が空なら、打った所の git の根（引けなければ空）。
func (d envDeps) root() string {
	if wt := d.issueWorktree(); wt != "" {
		return wt
	}
	if r, err := d.gitRoot(); err == nil {
		return r
	}
	return ""
}

func (d envDeps) issueWorktree() string {
	if d.terminal == "" {
		return ""
	}
	labels, err := d.labels(d.terminal)
	if err != nil {
		return ""
	}
	slug, id := labels["project"], labels["issue"]
	if !slugRe.MatchString(slug) || !issueRe.MatchString(id) {
		return ""
	}
	wt, err := d.worktree(slug, id)
	if err != nil {
		return ""
	}
	return wt
}

func hcLabels(addr string) (map[string]string, error) {
	cmd := exec.Command("hc", "get", addr, "--json")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("hc get %s に失敗: %v %s", addr, err, strings.TrimSpace(errb.String()))
	}
	var t struct {
		Labels map[string]string `json:"labels"`
	}
	if err := json.Unmarshal(out, &t); err != nil {
		return nil, fmt.Errorf("hc get %s の返事が読めない", addr)
	}
	return t.Labels, nil
}
