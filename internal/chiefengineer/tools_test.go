package chiefengineer

import (
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/tools"
	"andon/internal/tools/toolstest"
)

// manifest は案件の repo の main に mise.toml を足して push する（andon chief-engineer が切る worktree に入る）。
func (f *fake) manifest() {
	f.t.Helper()
	toolstest.Manifest(f.t, f.repoDir)
	git(f.t, f.repoDir, "add", "mise.toml")
	git(f.t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "mise")
	git(f.t, f.repoDir, "push", "-q", "origin", "HEAD:main")
}

func (f *fake) order() []string {
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

// worktree の根に mise.toml があれば、hc run の前に偽の mise を worktree の根で install で 1 回呼ぶ。
// 偽の mise が失敗しても、上限を過ぎても、警告を出して hc run を打つ（ADR-0063 受け入れ条件 1・2）。
func TestChiefEngineerSatisfiesDeclaration(t *testing.T) {
	for _, c := range []struct{ mode, warn string }{
		{"", ""},
		{"fail", " の mise.toml の道具を入れられない（起動は続ける）: mise install が失敗（exit status 1）: mise ERROR boom"},
		{"slow", " の mise.toml の道具が 200ms で入りきらない（mise install は止めずに続けさせ、起動へ進む）"},
	} {
		t.Run("mode="+c.mode, func(t *testing.T) {
			f := newFake(t)
			f.manifest()
			m := toolstest.New(t, c.mode)
			toolstest.Short(t, &tools.Limit, 200*time.Millisecond)
			rc, _, errs := runMain("--repo", "github.com/example/sample-site", "Billing の設計")
			if rc != 0 {
				t.Fatalf("rc=%d err=%q", rc, errs)
			}
			if c.warn == "" && errs != "" || c.warn != "" && !strings.Contains(errs, "andon chief-engineer: "+f.wt()+c.warn) {
				t.Errorf("err=%q", errs)
			}
			if got, want := f.order(), []string{"mise install | cwd=" + f.wt(), "hc run"}; !slices.Equal(got, want) {
				t.Errorf("偽の mise と hc run の順:\n got %q\nwant %q", got, want)
			}
			if c.mode == "slow" {
				m.WaitDone(t)
			}
		})
	}
}

func TestChiefEngineerWithoutManifestCallsNoMise(t *testing.T) {
	newFake(t)
	m := toolstest.New(t, "")
	if rc, _, errs := runMain("--repo", "github.com/example/sample-site", "Billing の設計"); rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if got := m.Calls(); len(got) != 0 {
		t.Errorf("mise.toml が無いのに呼んだ: %q", got)
	}
}
