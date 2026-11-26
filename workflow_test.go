package andon

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const tagOnly = "github.event_name == 'push' && github.ref_type == 'tag'"

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

// モデルの検査（model/check。外から取る npm の検査器 tla-ts を走らせる）は .github/workflows/model.yml の job model だけで打つ
// （ADR-0025 決めたこと 7）。その job は PR と main の push で動き、秘密も token も参照せず、権限は contents: read だけ。
// ほかの job（tests・release・go）と tests/*.sh は、検査器を入れない・打たない（秘密と書き込みの権限がある所で外のコードを走らせない）。
func TestWorkflowModelCheckOnlyInModelJob(t *testing.T) {
	type step struct {
		Run  string         `yaml:"run"`
		Uses string         `yaml:"uses"`
		With map[string]any `yaml:"with"`
	}
	type job struct {
		Permissions any    `yaml:"permissions"`
		Steps       []step `yaml:"steps"`
	}
	readOnly := func(perm any) bool {
		m, ok := perm.(map[string]any)
		return ok && len(m) == 1 && m["contents"] == "read"
	}
	checker := func(run string) bool {
		for _, w := range []string{"model/check", "tla-ts", "npm ", "npx "} {
			if strings.Contains(run, w) {
				return true
			}
		}
		return false
	}
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			On          map[string]any `yaml:"on"`
			Permissions any            `yaml:"permissions"`
			Jobs        map[string]job `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, j := range wf.Jobs {
			runs := false
			for _, s := range j.Steps {
				runs = runs || checker(s.Run)
			}
			if !runs {
				continue
			}
			if filepath.Base(f) != "model.yml" || name != "model" {
				t.Errorf("%s の job %s がモデルの検査器を入れる・打つ（model.yml の job model だけのはず）", f, name)
				continue
			}
			found++
			if strings.Contains(string(b), "secrets.") || strings.Contains(string(b), "github.token") {
				t.Errorf("%s が秘密か token を参照する", f)
			}
			if !readOnly(wf.Permissions) || !readOnly(j.Permissions) {
				t.Errorf("%s の job %s の権限が contents: read だけでない（top %v・job %v）", f, name, wf.Permissions, j.Permissions)
			}
			pr := wf.On["pull_request"]
			push, _ := wf.On["push"].(map[string]any)
			branches, _ := push["branches"].([]any)
			if _, ok := wf.On["pull_request"]; !ok || pr != nil || len(branches) != 1 || branches[0] != "main" {
				t.Errorf("%s が PR と main の push の両方で動かない（on: %v）", f, wf.On)
			}
			for _, s := range j.Steps {
				if strings.HasPrefix(s.Uses, "actions/checkout@") && s.With["persist-credentials"] != false {
					t.Errorf("%s の job %s の checkout が persist-credentials: false でない", f, name)
				}
			}
		}
	}
	// model.yml の job model の1つ（この検査が何も見ていなくても通るのを防ぐ）
	if found != 1 {
		t.Errorf("モデルの検査器を打つ job が %d 個（model.yml の job model の1つだけのはず）", found)
	}
	scripts, _ := filepath.Glob("tests/*.sh")
	libs, _ := filepath.Glob("tests/lib/*.sh")
	if len(scripts) == 0 {
		t.Fatal("tests/*.sh が無い")
	}
	for _, f := range append(append(scripts, libs...), "tests/run") {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "model/check") || strings.Contains(string(b), "tla-ts") {
			t.Errorf("%s がモデルの検査器を打つ（tests と release の job で回ってしまう）", f)
		}
	}
}

func TestWorkflowAptMirrorOnlyInTestsJob(t *testing.T) {
	b, err := os.ReadFile(".github/workflows/tests.yml")
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Run string            `yaml:"run"`
		Env map[string]string `yaml:"env"`
	}
	var wf struct {
		Env  map[string]string `yaml:"env"`
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []step            `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	if _, ok := wf.Env["ANDON_APT_MIRROR"]; ok {
		t.Error("tests.yml の workflow の env に ANDON_APT_MIRROR がある（release の job にも効く）")
	}
	set := map[string]int{}
	for name, j := range wf.Jobs {
		if _, ok := j.Env["ANDON_APT_MIRROR"]; ok {
			t.Errorf("tests.yml の job %s の env に ANDON_APT_MIRROR がある（step の env に置く）", name)
		}
		for _, s := range j.Steps {
			if strings.Contains(s.Run, "ANDON_APT_MIRROR") {
				t.Errorf("tests.yml の job %s の run が ANDON_APT_MIRROR に触る", name)
			}
			if v, ok := s.Env["ANDON_APT_MIRROR"]; ok {
				set[name]++
				if !strings.Contains(s.Run, "tests/lib/run-tests.sh") {
					t.Errorf("tests.yml の job %s の、tests/*.sh を回さない step に ANDON_APT_MIRROR がある", name)
				}
				if v != "http://azure.archive.ubuntu.com/ubuntu/" {
					t.Errorf("tests.yml の job %s の ANDON_APT_MIRROR が %q（runner と同じ Azure の mirror のはず）", name, v)
				}
			}
		}
	}
	if _, ok := wf.Jobs["release"]; !ok {
		t.Fatal("tests.yml に job release が無い")
	}
	if set["release"] != 0 {
		t.Error("tests.yml の job release が ANDON_APT_MIRROR を置く（release の取得元は既定のまま）")
	}
	if set["tests"] != 1 {
		t.Errorf("tests.yml の job tests で ANDON_APT_MIRROR を置く step が %d 個（tests/*.sh を回す1つのはず）", set["tests"])
	}
}

// golangci-lint（ADR-0034 決めたこと 4・5）は .github/workflows/go.yml の job go の1か所で回す。バージョンは mise.toml の golangci-lint の
// 1か所にだけ書き、action の version には mise.toml を読む step の出力を渡す。workflow にバージョンを書くと mise.toml とずれるので書かない。
func TestWorkflowGolangciLintVersionFromMise(t *testing.T) {
	mise, err := os.ReadFile("mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	vers := regexp.MustCompile(`(?m)^golangci-lint = "([0-9]+\.[0-9]+\.[0-9]+)"$`).FindAllSubmatch(mise, -1)
	if len(vers) != 1 {
		t.Fatalf("mise.toml に golangci-lint のバージョンが %d 行（1行のはず）", len(vers))
	}
	ver := string(vers[0][1])
	output := regexp.MustCompile(`^\$\{\{ steps\.([A-Za-z0-9_-]+)\.outputs\.version \}\}$`)
	type step struct {
		ID   string         `yaml:"id"`
		Run  string         `yaml:"run"`
		Uses string         `yaml:"uses"`
		With map[string]any `yaml:"with"`
	}
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), ver) {
			t.Errorf("%s に golangci-lint のバージョン %s が書いてある（mise.toml から読む）", f, ver)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []step `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, j := range wf.Jobs {
			for i, s := range j.Steps {
				if !strings.HasPrefix(s.Uses, "golangci/golangci-lint-action@") {
					continue
				}
				found++
				if filepath.Base(f) != "go.yml" || name != "go" {
					t.Errorf("%s の job %s が golangci-lint を回す（go.yml の job go の1か所のはず）", f, name)
				}
				for _, k := range []string{"version-file", "install-mode"} {
					if _, ok := s.With[k]; ok {
						t.Errorf("%s の job %s の golangci-lint に %s がある（バージョンは mise.toml のバージョンで、既定の binary で入れる）", f, name, k)
					}
				}
				v, _ := s.With["version"].(string)
				m := output.FindStringSubmatch(v)
				if m == nil {
					t.Errorf("%s の job %s の golangci-lint の version が %q（mise.toml を読む step の出力 ${{ steps.<id>.outputs.version }} のはず）", f, name, v)
					continue
				}
				reads := false
				for _, p := range j.Steps[:i] {
					if p.ID == m[1] && strings.Contains(p.Run, "mise.toml") && strings.Contains(p.Run, "golangci-lint") {
						reads = true
					}
				}
				if !reads {
					t.Errorf("%s の job %s で、golangci-lint より前の step %s が mise.toml の golangci-lint を読まない", f, name, m[1])
				}
			}
		}
	}
	// go.yml の job go の1つ（この検査が何も見ていなくても通るのを防ぐ）
	if found != 1 {
		t.Errorf("golangci-lint を回す step が %d 個（go.yml の job go の1つのはず）", found)
	}
}

// tray の検査（tray/。外から取る npm の依存を Bun で入れて走らせる。ADR-0064 決めたこと 28・受け入れ条件 18）は
// .github/workflows/tray.yml の job tray だけで回す。その job は PR と main の push で動き、秘密も token も参照せず、
// 権限は contents: read だけ。依存は lock から、install のスクリプトを走らせずに入れる。ほかの job（tests・release・go・model）と
// tests/*.sh は、Bun も tray の依存も走らせない（秘密と書き込みの権限がある所で外のコードを走らせない）。
func TestWorkflowTrayOnlyInTrayJob(t *testing.T) {
	type step struct {
		Run  string         `yaml:"run"`
		Uses string         `yaml:"uses"`
		With map[string]any `yaml:"with"`
	}
	type job struct {
		Permissions any    `yaml:"permissions"`
		Steps       []step `yaml:"steps"`
	}
	readOnly := func(perm any) bool {
		m, ok := perm.(map[string]any)
		return ok && len(m) == 1 && m["contents"] == "read"
	}
	runsTray := func(s step) bool {
		if strings.HasPrefix(s.Uses, "oven-sh/setup-bun@") {
			return true
		}
		for _, w := range []string{"bun ", "bunx ", "playwright ", "tray/"} {
			if strings.Contains(s.Run, w) {
				return true
			}
		}
		return false
	}
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			On          map[string]any `yaml:"on"`
			Permissions any            `yaml:"permissions"`
			Jobs        map[string]job `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, j := range wf.Jobs {
			runs := false
			for _, s := range j.Steps {
				runs = runs || runsTray(s)
			}
			if !runs {
				continue
			}
			if filepath.Base(f) != "tray.yml" || name != "tray" {
				t.Errorf("%s の job %s が Bun か tray の依存を走らせる（tray.yml の job tray だけのはず）", f, name)
				continue
			}
			found++
			if strings.Contains(string(b), "secrets.") || strings.Contains(string(b), "github.token") {
				t.Errorf("%s が秘密か token を参照する", f)
			}
			if !readOnly(wf.Permissions) || !readOnly(j.Permissions) {
				t.Errorf("%s の job %s の権限が contents: read だけでない（top %v・job %v）", f, name, wf.Permissions, j.Permissions)
			}
			pr := wf.On["pull_request"]
			push, _ := wf.On["push"].(map[string]any)
			branches, _ := push["branches"].([]any)
			if _, ok := wf.On["pull_request"]; !ok || pr != nil || len(branches) != 1 || branches[0] != "main" {
				t.Errorf("%s が PR と main の push の両方で動かない（on: %v）", f, wf.On)
			}
			installs := 0
			for _, s := range j.Steps {
				if strings.HasPrefix(s.Uses, "actions/checkout@") && s.With["persist-credentials"] != false {
					t.Errorf("%s の job %s の checkout が persist-credentials: false でない", f, name)
				}
				if strings.Contains(s.Run, "bun install") || strings.Contains(s.Run, "bun add") || strings.Contains(s.Run, "bun i ") {
					installs++
					if !strings.Contains(s.Run, "bun install --frozen-lockfile --ignore-scripts") {
						t.Errorf("%s の job %s が依存を lock のとおりに入れない、か install のスクリプトを走らせる: %q", f, name, s.Run)
					}
				}
			}
			if installs != 1 {
				t.Errorf("%s の job %s で依存を入れる step が %d 個（bun install --frozen-lockfile --ignore-scripts の1つのはず）", f, name, installs)
			}
		}
	}
	// tray.yml の job tray の1つ（この検査が何も見ていなくても通るのを防ぐ）
	if found != 1 {
		t.Errorf("Bun か tray の依存を走らせる job が %d 個（tray.yml の job tray の1つだけのはず）", found)
	}
	scripts, _ := filepath.Glob("tests/*.sh")
	libs, _ := filepath.Glob("tests/lib/*.sh")
	if len(scripts) == 0 {
		t.Fatal("tests/*.sh が無い")
	}
	for _, f := range append(append(scripts, libs...), "tests/run") {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range []string{"bun ", "bunx ", "tray/scripts", "tray/node_modules"} {
			if strings.Contains(string(b), w) {
				t.Errorf("%s が %q を含む（tray の依存は tests と release の job で走らせない）", f, w)
			}
		}
	}
}
