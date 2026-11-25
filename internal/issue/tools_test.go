package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/tools"
	"andon/internal/tools/toolstest"
)

// manifest は案件の repo の main に mise.toml を足して push する（start が切る worktree に入る）。
func (f *startFixture) manifest() {
	f.t.Helper()
	toolstest.Manifest(f.t, f.repoDir)
	gitT(f.t, f.repoDir, "add", "mise.toml")
	gitT(f.t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "mise")
	gitT(f.t, f.repoDir, "push", "-q", "origin", "HEAD:main")
}

// order は呼び出しの記録のうち、偽の mise と hc run の行だけを順に並べる。
func (f *startFixture) order() []string {
	var got []string
	for _, c := range f.calls() {
		switch {
		case strings.HasPrefix(c, "mise "):
			got = append(got, c)
		case strings.HasPrefix(c, "hc run "):
			got = append(got, "hc run")
		}
	}
	return got
}

// worktree の根に mise.toml があれば、hc run の前に偽の mise を worktree の根で install で 1 回呼ぶ（ADR-0063 受け入れ条件 1）。
func TestStartSatisfiesDeclaration(t *testing.T) {
	f := newStart(t)
	f.manifest()
	toolstest.New(t, "")
	rc, _, errs := f.run()
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	if got, want := f.order(), []string{"mise install | cwd=" + f.wt(), "hc run"}; !slices.Equal(got, want) {
		t.Errorf("偽の mise と hc run の順:\n got %q\nwant %q", got, want)
	}
}

func TestStartWithoutManifestCallsNoMise(t *testing.T) {
	f := newStart(t)
	m := toolstest.New(t, "")
	if rc, _, errs := f.run(); rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if got := m.Calls(); len(got) != 0 {
		t.Errorf("mise.toml が無いのに呼んだ: %q", got)
	}
}

// 偽の mise が失敗しても、上限を過ぎても、警告を出して hc run を打つ。過ぎた偽の mise は止められずに残り、あとで自分で終わる（受け入れ条件 2）。
func TestStartWarnsAndLaunches(t *testing.T) {
	for _, c := range []struct{ mode, warn string }{
		{"fail", " の道具を入れられない（起動は続ける）: mise install が失敗（exit status 1）: mise ERROR boom"},
		{"slow", " の道具が 200ms で入りきらない（mise install は止めずに続けさせ、起動へ進む）"},
	} {
		t.Run(c.mode, func(t *testing.T) {
			f := newStart(t)
			f.manifest()
			m := toolstest.New(t, c.mode)
			toolstest.Short(t, &tools.Limit, 200*time.Millisecond)
			rc, _, errs := f.run()
			if rc != 0 || !strings.Contains(errs, "andon issue: "+f.wt()+" の mise.toml"+c.warn) || !strings.Contains(errs, "cd "+f.wt()+" && mise install") {
				t.Fatalf("rc=%d err=%q", rc, errs)
			}
			if got, want := f.order(), []string{"mise install | cwd=" + f.wt(), "hc run"}; !slices.Equal(got, want) {
				t.Errorf("順:\n got %q\nwant %q", got, want)
			}
			if c.mode == "slow" {
				if m.Done() {
					t.Fatal("hc run の前に偽の mise が終わっている（上限を見ていない）")
				}
				m.WaitDone(t)
			}
		})
	}
}

// resume も hc run の前に worktree の根で宣言を満たす（受け入れ条件 1・2）。
func TestResumeSatisfiesDeclaration(t *testing.T) {
	for _, mode := range []string{"", "fail", "slow"} {
		t.Run("mode="+mode, func(t *testing.T) {
			f := newStart(t)
			f.manifest()
			if rc, _, errs := f.run(); rc != 0 {
				t.Fatalf("start: rc=%d err=%q", rc, errs)
			}
			m := toolstest.New(t, mode)
			toolstest.Short(t, &tools.Limit, 200*time.Millisecond)
			os.Truncate(f.log, 0)
			sid := "11111111-2222-4333-8444-555555555555"
			cfg := filepath.Join(f.home, ".claude")
			t.Setenv("CLAUDE_CONFIG_DIR", cfg)
			os.MkdirAll(filepath.Join(cfg, "projects", "x"), 0o755)
			os.WriteFile(filepath.Join(cfg, "projects", "x", sid+".jsonl"), nil, 0o644)
			hc := strings.Replace(startHc, "case $1 in\n", "case $1 in\n  get) echo '{\"error\":{\"kind\":\"not_found\"}}' >&2; exit 3 ;;\n", 1)
			os.WriteFile(filepath.Join(filepath.Dir(f.log), "bin", "hc"), []byte(hc), 0o755)
			t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"claude-resume","command":"claude --resume \"$s\""}]}`)
			t.Setenv("FAKE_RUN", `{"address":"t43","first_message":{"sent":true}}`)
			f.task = map[string]any{"id": "S-0007", "title": "Add the button", "status": "doing", "profile": "claude", "agent": "t42", "session": sid,
				"repo": "github.com/example/sample-site", "worktree": f.wt(), "branch": "sample-site/S-0007-add-the-button"}
			os.WriteFile(filepath.Join(f.root, "show-S-0007.json"), []byte(js(map[string]any{"issue": f.task})), 0o644)
			var out, errb bytes.Buffer
			if rc := Main([]string{"resume", "S-0007"}, strings.NewReader(""), &out, &errb); rc != 0 {
				t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out.String(), errb.String(), f.calls())
			}
			if got, want := f.order(), []string{"mise install | cwd=" + f.wt(), "hc run"}; !slices.Equal(got, want) {
				t.Errorf("順:\n got %q\nwant %q", got, want)
			}
			if mode != "" && !strings.Contains(errb.String(), "andon issue: "+f.wt()+" の mise.toml の道具") {
				t.Errorf("警告が無い: %q", errb.String())
			}
			if mode == "slow" {
				m.WaitDone(t)
			}
		})
	}
}
