package otel

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/delegate"
)

// nameServer は GET / に body を返す HTTP のサーバ（andon の受け口か、ほかのプロセスの代わり）。
func nameServer(t *testing.T, body string) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, body) }))
	t.Cleanup(s.Close)
	return strings.TrimPrefix(s.URL, "http://")
}

// freeAddr は誰も listen していない 127.0.0.1 の所。
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func projectJSON(otel string) func(string) (*delegate.Project, error) {
	return func(slug string) (*delegate.Project, error) {
		j := `{"version": 1, "project": {"title": "Sample site", "prefix": "S"}, "repos": []` + otel + `}`
		return delegate.ParseProject([]byte(j), slug)
	}
}

// deps は担当の端末（ラベル project・issue）の偽物。受け口は andon の受け口として listen している。
func deps(t *testing.T, labels map[string]string, profile string) envDeps {
	return envDeps{
		terminal: "t1",
		labels:   func(string) (map[string]string, error) { return labels, nil },
		profile: func(slug, id string) (string, error) {
			if slug != "sample-site" || id != "S-0012" {
				return "", fmt.Errorf("知らない課題 %s/%s", slug, id)
			}
			return profile, nil
		},
		project: projectJSON(""),
		addr:    nameServer(t, Name),
		start:   func() error { return errors.New("起動しない（もう listen している）") },
		wait:    time.Second,
	}
}

var worker = map[string]string{"project": "sample-site", "issue": "S-0012", "title": "返事の下書きボタン", "session": "56401e3a-30ce-4815-a5da-9acdc5bcf49f"}

// 決めたこと 5 の細部の並び（属性のほか）。
func wantLines(addr, attrs string) []string {
	return []string{
		"CLAUDE_CODE_ENABLE_TELEMETRY=1",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1",
		"OTEL_TRACES_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
		"OTEL_METRICS_EXPORTER=none",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/json",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://" + addr,
		"OTEL_RESOURCE_ATTRIBUTES=" + attrs,
		"OTEL_LOG_TOOL_DETAILS=1",
	}
}

// ADR-0060 受け入れ条件 7（前半）: ラベルが project と issue だけの担当の端末は andon.role=worker、プロファイルがレビュアーなら
// reviewer、ラベル role=chief-of-staff なら chief-of-staff で andon.issue が無い。本文を載せる設定は出さない。
func TestEnvRoles(t *testing.T) {
	cos := map[string]string{"role": "chief-of-staff", "project": "sample-site", "herdr.space": "andon-sample-site"}
	ce := map[string]string{"role": "chief-engineer", "project": "sample-site", "issue": "S-0012", "title": "x"}
	for _, c := range []struct {
		name    string
		labels  map[string]string
		profile string
		attrs   string
	}{
		{"担当", worker, "claude", "andon.project=sample-site,andon.issue=S-0012,andon.role=worker"},
		{"担当（effort medium）", worker, "claude-medium", "andon.project=sample-site,andon.issue=S-0012,andon.role=worker"},
		{"レビュアー", worker, "codex-review", "andon.project=sample-site,andon.issue=S-0012,andon.role=reviewer"},
		{"Chief of Staff", cos, "", "andon.project=sample-site,andon.role=chief-of-staff"},
		{"Chief of Staff（ラベル issue があっても載せない）", map[string]string{"role": "chief-of-staff", "project": "sample-site", "issue": "S-0012"}, "", "andon.project=sample-site,andon.role=chief-of-staff"},
		{"Chief Engineer", ce, "", "andon.project=sample-site,andon.issue=S-0012,andon.role=chief-engineer"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := deps(t, c.labels, c.profile)
			got, why := d.env(claudeCode)
			if why != "" {
				t.Fatalf("送らない: %s", why)
			}
			if want := wantLines(d.addr, c.attrs); !slices.Equal(got, want) {
				t.Errorf("並びが違う:\n got %q\nwant %q", got, want)
			}
			for _, l := range got {
				for _, body := range []string{"OTEL_LOG_USER_PROMPTS", "OTEL_LOG_ASSISTANT_RESPONSES", "OTEL_LOG_TOOL_CONTENT"} {
					if strings.HasPrefix(l, body) {
						t.Errorf("本文を載せる設定を出した: %s", l)
					}
				}
			}
		})
	}
}

// ADR-0060 受け入れ条件 7（後半）: 送らない設定・project.json が読めない・端末の id が無い・ほかのプロセスがポートを取っている
// ときは、何も出さない（EnvMain は終了コード 0）。
func TestEnvSendsNothing(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(*envDeps)
		quiet  bool // 理由も出さない（送らない設定のとおり）
	}{
		{"otel.enabled が false", func(d *envDeps) { d.project = projectJSON(`, "otel": {"enabled": false}`) }, true},
		{"project.json が schema を通らない", func(d *envDeps) { d.project = projectJSON(`, "otel": {"enabled": "no"}`) }, false},
		{"project.json が無い", func(d *envDeps) {
			d.project = func(string) (*delegate.Project, error) { return nil, delegate.ErrAbsent }
		}, false},
		{"端末の id が無い", func(d *envDeps) { d.terminal = "" }, false},
		{"ラベルが読めない", func(d *envDeps) {
			d.labels = func(string) (map[string]string, error) { return nil, errors.New("hc get t1 に失敗") }
		}, false},
		{"ラベル project が無い", func(d *envDeps) {
			d.labels = func(string) (map[string]string, error) { return map[string]string{"issue": "S-0012"}, nil }
		}, false},
		{"ラベル issue の形が違う", func(d *envDeps) {
			d.labels = func(string) (map[string]string, error) {
				return map[string]string{"role": "worker", "project": "sample-site", "issue": "../x"}, nil
			}
		}, false},
		{"課題のプロファイルが読めない", func(d *envDeps) {
			d.labels = func(string) (map[string]string, error) {
				return map[string]string{"project": "sample-site", "issue": "S-0099"}, nil
			}
		}, false},
		{"ほかのプロセスがポートを取っている", func(d *envDeps) { d.addr = nameServer(t, "portless") }, false},
		{"受け口を起動できない", func(d *envDeps) { d.addr = freeAddr(t) }, false},
	} {
		for _, h := range []harness{claudeCode, codexArgs, codexEnv} {
			t.Run(fmt.Sprintf("%s/%d", c.name, h), func(t *testing.T) {
				d := deps(t, worker, "claude")
				c.change(&d)
				got, why := d.env(h)
				if len(got) != 0 {
					t.Errorf("送る設定を出した: %q", got)
				}
				if (why == "") != c.quiet {
					t.Errorf("理由 %q（理由を出さないのは送らない設定のときだけ）", why)
				}
			})
		}
	}
}

// ADR-0060 決めたこと 19: --codex は Codex の -c の並び（ログとトレースを受け口の道へ JSON で、span_attributes に
// 決めたこと 6 の 3 つ、ツールの出力の写しは 0 バイト）を出し、metrics_exporter は書かない。--codex-env は
// resource 属性の環境変数を 1 つだけ出す（Codex の SDK が resource に読み、ログも課題に結ぶ）。
func TestEnvCodex(t *testing.T) {
	cos := map[string]string{"role": "chief-of-staff", "project": "sample-site"}
	for _, c := range []struct {
		name    string
		labels  map[string]string
		profile string
		span    string
		res     string
	}{
		{"レビュアー", worker, "codex-review", `"andon.project"="sample-site","andon.issue"="S-0012","andon.role"="reviewer"`,
			"andon.project=sample-site,andon.issue=S-0012,andon.role=reviewer"},
		{"担当", worker, "codex", `"andon.project"="sample-site","andon.issue"="S-0012","andon.role"="worker"`,
			"andon.project=sample-site,andon.issue=S-0012,andon.role=worker"},
		{"Chief of Staff", cos, "", `"andon.project"="sample-site","andon.role"="chief-of-staff"`,
			"andon.project=sample-site,andon.role=chief-of-staff"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := deps(t, c.labels, c.profile)
			got, why := d.env(codexArgs)
			if why != "" {
				t.Fatalf("送らない: %s", why)
			}
			want := []string{
				"-c", `otel.exporter={otlp-http={endpoint="http://` + d.addr + `/v1/logs",protocol="json"}}`,
				"-c", `otel.trace_exporter={otlp-http={endpoint="http://` + d.addr + `/v1/traces",protocol="json"}}`,
				"-c", "otel.span_attributes={" + c.span + "}",
				"-c", "otel.tool_result.max_bytes=0",
			}
			if !slices.Equal(got, want) {
				t.Errorf("並びが違う:\n got %q\nwant %q", got, want)
			}
			for _, w := range got {
				// template は $(…) を引用符なしで展開するので、1 行が 1 語のまま渡るには空白も glob の文字も持てない
				if strings.ContainsAny(w, " \t\n*?[") {
					t.Errorf("1 語にならない行: %q", w)
				}
				if strings.Contains(w, "metrics_exporter") || strings.Contains(w, "log_user_prompt") || strings.Contains(w, "log_agent_responses") {
					t.Errorf("出さないはずの設定: %q", w)
				}
			}
			got, why = d.env(codexEnv)
			if why != "" || !slices.Equal(got, []string{"OTEL_RESOURCE_ATTRIBUTES=" + c.res}) {
				t.Errorf("--codex-env が %q（理由 %q）", got, why)
			}
		})
	}
}

// 決めたこと 4: 受け口が listen していなければ起動して、listen を待ってから出す。
func TestEnvStartsReceiver(t *testing.T) {
	d := deps(t, worker, "claude")
	d.addr = freeAddr(t)
	started := 0
	d.start = func() error {
		started++
		ln, err := net.Listen("tcp", d.addr)
		if err != nil {
			return err
		}
		s := &httptest.Server{Listener: ln, Config: &http.Server{Handler: &Receiver{Root: t.TempDir()}}}
		s.Start()
		t.Cleanup(s.Close)
		return nil
	}
	got, why := d.env(claudeCode)
	if why != "" || started != 1 {
		t.Fatalf("起動 %d 回・理由 %q", started, why)
	}
	if want := wantLines(d.addr, "andon.project=sample-site,andon.issue=S-0012,andon.role=worker"); !slices.Equal(got, want) {
		t.Errorf("並びが違う: %q", got)
	}
	// もう listen しているので、2 回目は起動しない
	if _, why := d.env(claudeCode); why != "" || started != 1 {
		t.Errorf("2 回目: 起動 %d 回・理由 %q", started, why)
	}
}

// 受け入れ条件 7: 端末の id が無いと、andon otel env は何も出さずに終了コード 0。
func TestEnvMainWithoutTerminal(t *testing.T) {
	t.Setenv("HC_TERMINAL_ID", "")
	var out, errb bytes.Buffer
	if rc := EnvMain(nil, nil, &out, &errb); rc != 0 || out.Len() != 0 {
		t.Errorf("終了コード %d・出力 %q", rc, out.String())
	}
	for _, args := range [][]string{{"--codex"}, {"--codex-env"}} {
		if rc := EnvMain(args, nil, &out, &errb); rc != 0 || out.Len() != 0 {
			t.Errorf("%v: 終了コード %d・出力 %q", args, rc, out.String())
		}
	}
	for _, args := range [][]string{{"x"}, {"--codex", "--codex-env"}, {"--codex=1"}} {
		if rc := EnvMain(args, nil, &out, &errb); rc != 2 {
			t.Errorf("%v: 知らない引数で終了コード 2 のはず: %d", args, rc)
		}
	}
}

// claudeRe は claude を起動する語（前が空白か / の claude のあとに空白）。/usr/bin/claude のような書き方も Claude Code の行と見る。
var claudeRe = regexp.MustCompile(`(^|[ /])claude `)

// codexRe は codex を起動する語（claudeRe と同じ見方）。
var codexRe = regexp.MustCompile(`(^|[ /])codex `)

// 決めたこと 5・19: Claude Code を起動する template の行（command・resume）は、どれも andon otel env を env に渡す。
// Codex を起動する行は、resource 属性を env に、-c の並びを codex の引数に渡す。
func TestTemplatesPassOtelEnv(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "package", "templates", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("template が無い: %v", err)
	}
	n, nCodex := 0, 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if !strings.HasPrefix(l, "command:") && !strings.HasPrefix(l, "resume:") {
				continue
			}
			claude := claudeRe.MatchString(l)
			passes := strings.Contains(l, "env $(andon otel env) CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude ")
			// 渡す設定は claude を起動する env の中だけで効く（別の command の env や注釈に書いても効かない）
			if claude != passes {
				t.Errorf("%s: Claude Code の行が andon otel env を渡さないか、Claude Code でない行が渡す: %s", filepath.Base(f), l)
			}
			if claude {
				n++
			}
			codex := codexRe.MatchString(l)
			passesCodex := strings.Contains(l, "env $(andon otel env --codex-env) codex $(andon otel env --codex) ")
			if codex != passesCodex {
				t.Errorf("%s: Codex の行が andon otel env --codex-env・--codex を渡さないか、Codex でない行が渡す: %s", filepath.Base(f), l)
			}
			if codex {
				nCodex++
			}
		}
	}
	if n == 0 || nCodex == 0 {
		t.Errorf("Claude Code を起動する行 %d・Codex を起動する行 %d（どちらかが 0 だと検査が何も見ていない）", n, nCodex)
	}
}

// go test のバイナリに otel serve を打たせない（検査を走らせ直し、また起動して process が際限なく増える）。
func TestDetachOnlyAndon(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if IsAndon(self) {
		t.Fatalf("検査のバイナリ %s を andon と見た", self)
	}
	if err := Detach(self, filepath.Join(t.TempDir(), "log")); err == nil {
		t.Fatal("検査のバイナリで受け口を起動した")
	}
	for _, bin := range []string{"/opt/andon/bin/andon", "andon"} {
		if !IsAndon(bin) {
			t.Errorf("%s を andon と見ない", bin)
		}
	}
}
