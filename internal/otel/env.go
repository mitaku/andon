package otel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
	"andon/internal/launch"
	"andon/internal/ledger"
)

// EnvUsage は andon otel env の使い方。
const EnvUsage = "usage: andon otel env\n" +
	"役の起動の行（package/templates/ の command・resume）が読む。自分の端末（HC_TERMINAL_ID）のラベル project・issue・role から、\n" +
	"Claude Code が受け口 " + Addr + " へトレースとログを送る設定を KEY=VALUE の並びで出す（1 行に 1 つ。template が env に渡す）。\n" +
	"役はラベル role。無ければラベル issue の課題のプロファイルから、起動の記録と同じ決め方で担当かレビュアーかを決める。\n" +
	"受け口が listen していなければ起動する。送らない設定（project.json の otel.enabled が false）・project.json が無い・読めない・\n" +
	"ラベルが読めない・受け口を起動できない・ポートをほかのプロセスが取っているときは、何も出さずに終了コード 0（起動は止めない）。\n"

// envDeps は andon otel env が外に触る所（検査が偽物に替える）。
type envDeps struct {
	terminal string                                       // 自分の端末の address（HC_TERMINAL_ID）
	labels   func(addr string) (map[string]string, error) // 端末のラベル
	profile  func(slug, id string) (string, error)        // 課題のプロファイル
	project  func(slug string) (*delegate.Project, error) // セルの repo の HEAD の project.json（author 照合つき）
	addr     string                                       // 受け口の所
	start    func() error                                 // 受け口を起動して手放す
	wait     time.Duration                                // 起動してから listen を待つ長さ
}

// EnvMain は andon otel env。
func EnvMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, EnvUsage)
		return 2
	}
	d := envDeps{
		terminal: os.Getenv("HC_TERMINAL_ID"),
		labels:   hcLabels,
		profile: func(slug, id string) (string, error) {
			t, err := ledger.At(filepath.Join(delegate.Workspace(), "projects", slug)).Show(id)
			if err != nil {
				return "", err
			}
			return t.Str("profile"), nil
		},
		project: func(slug string) (*delegate.Project, error) { return delegate.ReadProject(delegate.Workspace(), slug) },
		addr:    Addr,
		start:   startServe,
		wait:    3 * time.Second,
	}
	lines, why := d.env()
	if why != "" {
		fmt.Fprintf(stderr, "andon otel env: 送らない（%s）\n", why)
		return 0
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

// env は送る設定の並びを返す。送らないときは空の並びと理由（stderr に 1 行。起動は止めない）。
// 送らない設定のときは理由も空（決めたことのとおりで、知らせることが無い）。
func (d envDeps) env() ([]string, string) {
	if d.terminal == "" {
		return nil, "HC_TERMINAL_ID が無い（hc の端末の外）"
	}
	labels, err := d.labels(d.terminal)
	if err != nil {
		return nil, err.Error()
	}
	project, issue, role := labels["project"], labels["issue"], labels["role"]
	if !projectRe.MatchString(project) {
		return nil, fmt.Sprintf("端末のラベル project が無いか形が違う（%q）", short(project))
	}
	switch {
	case role == launch.RoleChiefOfStaff:
		issue = "" // Chief of Staff の記録は課題に結ばない（決めたこと 6）
	case role != "":
		if !roleRe.MatchString(role) {
			return nil, fmt.Sprintf("端末のラベル role の形が違う（%q）", short(role))
		}
	case issue == "":
		return nil, "端末のラベルに role も issue も無い"
	default:
		profile, err := d.profile(project, issue)
		if err != nil {
			return nil, fmt.Sprintf("課題 %s のプロファイルが読めない: %v", issue, err)
		}
		role = launch.RoleOf(profile)
	}
	if issue != "" && !issueRe.MatchString(issue) {
		return nil, fmt.Sprintf("端末のラベル issue の形が違う（%q）", short(issue))
	}
	p, err := d.project(project)
	if err != nil {
		return nil, fmt.Sprintf("projects/%s/project.json が読めない: %v", project, err)
	}
	if p.Otel != nil && p.Otel.Enabled != nil && !*p.Otel.Enabled {
		return nil, ""
	}
	if why := d.ensure(); why != "" {
		return nil, why
	}
	attrs := "andon.project=" + project
	if issue != "" {
		attrs += ",andon.issue=" + issue
	}
	attrs += ",andon.role=" + role
	return []string{
		"CLAUDE_CODE_ENABLE_TELEMETRY=1",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1",
		"OTEL_TRACES_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_METRICS_EXPORTER=none",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/json",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://" + d.addr,
		"OTEL_RESOURCE_ATTRIBUTES=" + attrs,
		"OTEL_LOG_TOOL_DETAILS=1", // ツールの詳細だけ載せ、プロンプトと応答とツールの中身は載せない（決めたこと 7）
	}, ""
}

// listener は受け口の所を確かめた結果。
type listener int

const (
	absent listener = iota // 誰も listen していない
	ours                   // andon の受け口が listen している
	other                  // ほかのプロセスが listen している（名前が違う・HTTP で返らない）
)

// probe は addr に listen している相手を確かめる。名前は GET / が返す Name で見る（決めたこと 4）。
func probe(addr string) listener {
	c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return absent
	}
	c.Close() //nolint:errcheck // 届くかを見るだけの接続
	cl := http.Client{Timeout: time.Second}
	res, err := cl.Get("http://" + addr + "/")
	if err != nil {
		return other
	}
	defer res.Body.Close() //nolint:errcheck // 読み終えた本文を閉じるだけ
	b, _ := io.ReadAll(io.LimitReader(res.Body, 256))
	if res.StatusCode == http.StatusOK && strings.TrimSpace(string(b)) == Name {
		return ours
	}
	return other
}

// ensure は受け口が listen していることを確かめ、していなければ起動して待つ。送れないときは理由を返す。
// 二重の起動は bind の失敗で防ぐ（同時に 2 つ起動しても、ポートを取れるのは 1 つだけ）。
func (d envDeps) ensure() string {
	switch probe(d.addr) {
	case ours:
		return ""
	case other:
		return d.addr + " をほかのプロセスが取っている"
	}
	if err := d.start(); err != nil {
		return "受け口を起動できない: " + err.Error()
	}
	for end := time.Now().Add(d.wait); ; time.Sleep(100 * time.Millisecond) {
		switch probe(d.addr) {
		case ours:
			return ""
		case other:
			return d.addr + " をほかのプロセスが取っている"
		}
		if time.Now().After(end) {
			return "受け口が listen しない（" + LogPath() + "）"
		}
	}
}

// Ensure は andon の受け口が listen していることを確かめ、していなければ bin の otel serve を起動して待つ（boot が使う）。
// 起動できなかった・ほかのプロセスがポートを取っているときは理由を返す。
func Ensure(bin string) string {
	return envDeps{addr: Addr, start: func() error { return Detach(bin, LogPath()) }, wait: 3 * time.Second}.ensure()
}

// LogPath は env と boot が起動した受け口の出力の置き場。
func LogPath() string { return filepath.Join(os.TempDir(), "andon-otel.log") }

// startServe は andon otel serve を自分のバイナリで起動して手放す。新しいセッションにするので、
// 起動した端末を片付けても受け口は残る（受け入れ条件 15）。
func startServe() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return Detach(self, LogPath())
}

// IsAndon は bin が andon のバイナリ（ファイルの名前が andon）かを見る。go test のバイナリ（<package>.test）に otel serve を
// 打たせると、引数を無視して検査を全部走らせ直し、その検査がまた受け口を起動して、process が際限なく増える。
func IsAndon(bin string) bool { return filepath.Base(bin) == "andon" }

// Detach は bin の otel serve を新しいセッションで起動して手放す（boot と andon otel env が使う）。出力は log に足す。
// bin が andon のバイナリでなければ起動しない（IsAndon）。
func Detach(bin, log string) error {
	if !IsAndon(bin) {
		return fmt.Errorf("%s は andon のバイナリでないので受け口を起動しない", bin)
	}
	out, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck // 子は自分の写しで書くので、親の写しを閉じる失敗は出力に効かない
	cmd := exec.Command(bin, "otel", "serve")
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// hcLabels は端末 addr のラベル（hc get <addr> --json の labels）。
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
		return nil, errors.New("hc get " + addr + " の返事が読めない")
	}
	return t.Labels, nil
}
