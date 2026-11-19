package andon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// tagOnly は vX.Y.Z の tag の push でだけ動く job の条件（.github/workflows/tests.yml の release）。
const tagOnly = "github.event_name == 'push' && github.ref_type == 'tag'"

// writes は permissions（write-all・read-all の文字列か、scope → 権限の map）が、
// GHCR か repo に書ける scope（packages・contents）を持つか。
func writes(perm any) []string {
	switch p := perm.(type) {
	case string:
		if p == "write-all" {
			return []string{"write-all"}
		}
	case map[string]any:
		var w []string
		for _, k := range []string{"packages", "contents"} {
			if p[k] == "write" {
				w = append(w, k+": write")
			}
		}
		return w
	}
	return nil
}

// .github/workflows/ の全部の workflow で、PR と main の push で動く job（tag の条件でない job）の token に
// packages・contents の書き込みを持たせない（step の if は token の権限を狭めない）。job に permissions が無ければ
// workflow の top のものを継ぐので、それも見る。GHCR に書くのは tests.yml の release（tag でだけ動く）の1つだけ。
func TestWorkflowWritePermissionsOnlyOnTagJob(t *testing.T) {
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	if len(files) == 0 {
		t.Fatal(".github/workflows/ に workflow が無い")
	}
	pushers := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Permissions any `yaml:"permissions"`
			Jobs        map[string]struct {
				If          string `yaml:"if"`
				Permissions any    `yaml:"permissions"`
				Steps       []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if wf.Permissions == nil {
			t.Errorf("%s: workflow の top に permissions が無い（既定の token の権限に頼らない）", f)
		}
		for name, j := range wf.Jobs {
			perm := j.Permissions
			if perm == nil {
				perm = wf.Permissions
			}
			w := writes(perm)
			pushes := false
			for _, s := range j.Steps {
				if strings.Contains(s.Run, "docker push") || strings.Contains(s.Run, "docker login ghcr.io") {
					pushes = true
				}
			}
			tag := strings.TrimSpace(j.If) == tagOnly
			if (len(w) > 0 || pushes) && !tag {
				t.Errorf("%s の job %s は tag の push 以外でも動くのに %v（GHCR への push: %v）を持つ（if: %q）", f, name, w, pushes, j.If)
			}
			if pushes {
				pushers++
			}
		}
	}
	if pushers != 1 {
		t.Errorf("GHCR へ push する job が %d 個（tests.yml の release の1つだけのはず）", pushers)
	}
}

// tests/*.sh を回す job は .github/workflows/tests.yml の中だけにし（go.yml と二重に回さない）、どれも先に焼いた
// andon・kanban を ANDON_BIN・KANBAN_BIN で渡す（渡さないと tests/*.sh が1本ごとに go build し、release の job では
// runner の go で焼く）。
func TestWorkflowShellTestsRunOnceWithPrebuiltBinaries(t *testing.T) {
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	runners := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, j := range wf.Jobs {
			built := false
			for _, s := range j.Steps {
				if strings.Contains(s.Run, "ANDON_BIN=") && strings.Contains(s.Run, "KANBAN_BIN=") && strings.Contains(s.Run, "GITHUB_ENV") {
					built = true
				}
				if !strings.Contains(s.Run, "tests/lib/run-tests.sh") && !strings.Contains(s.Run, "tests/andon") && !strings.Contains(s.Run, "tests/kanban.sh") {
					continue
				}
				runners++
				if filepath.Base(f) != "tests.yml" {
					t.Errorf("%s の job %s が tests/*.sh を回す（tests.yml と二重になる）", f, name)
				}
				if !built {
					t.Errorf("%s の job %s が andon・kanban を焼いて ANDON_BIN・KANBAN_BIN に渡す前に tests/*.sh を回す", f, name)
				}
			}
		}
	}
	// tests.yml の tests と release の2つ（この検査が何も見ていなくても通るのを防ぐ）
	if runners != 2 {
		t.Errorf("tests/*.sh を回す step が %d 個（tests.yml の tests と release の2つのはず）", runners)
	}
}
