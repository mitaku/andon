package andon

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// tagOnly は vX.Y.Z の tag の push でだけ動く job の条件（.github/workflows/tests.yml の release）。
const tagOnly = "github.event_name == 'push' && github.ref_type == 'tag'"

// PR と main の push で動く job の token に packages の書き込みを持たせない（step の if は token の権限を狭めない）。
// GHCR に書くのは tag でだけ動く job だけ。
func TestWorkflowPackagesWriteOnlyOnTagJob(t *testing.T) {
	b, err := os.ReadFile(".github/workflows/tests.yml")
	if err != nil {
		t.Fatal(err)
	}
	var wf struct {
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			If          string            `yaml:"if"`
			Permissions map[string]string `yaml:"permissions"`
			Steps       []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	if p := wf.Permissions["packages"]; p == "write" {
		t.Errorf("workflow の permissions に packages: write がある（全部の job に効く）")
	}
	writers := 0
	for name, j := range wf.Jobs {
		if j.Permissions == nil {
			t.Errorf("job %s に permissions が無い（workflow の既定に頼らない）", name)
		}
		pushes := false
		for _, s := range j.Steps {
			if strings.Contains(s.Run, "docker push") || strings.Contains(s.Run, "docker login ghcr.io") {
				pushes = true
			}
		}
		write := j.Permissions["packages"] == "write"
		if (write || pushes) && strings.TrimSpace(j.If) != tagOnly {
			t.Errorf("job %s は tag の push 以外でも動くのに packages: write か GHCR への push を持つ（if: %q）", name, j.If)
		}
		if write {
			writers++
		}
	}
	if writers != 1 {
		t.Errorf("packages: write の job が %d 個（tag の release の1つだけのはず）", writers)
	}
}
