package andon

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
// .github/workflows/tray.yml の job tray だけで回す。job tray は秘密も token も読めず、権限は contents: read だけで、依存を lock から
// install のスクリプトなしで入れる。ほかの workflow（秘密や書き込みの権限を持つ job がある）と tests/ の下の検査は、Bun も tray の
// コードも走らせない。字面の部分一致では書き方を変えた入力が素通りするので、tray.yml は許す形の一覧で照らす（式・キー・uses・run の行）。
// tray.yml を変えたら、この一覧も同じ PR で直す。
func TestWorkflowTrayOnlyInTrayJob(t *testing.T) {
	wfs := map[string]string{}
	files, _ := filepath.Glob(".github/workflows/*.y*ml")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		wfs[f] = string(b)
	}
	tests := map[string]string{}
	if err := filepath.WalkDir("tests", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		tests[p] = string(b)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(tests) == 0 {
		t.Fatal("tests/ の下にファイルが無い")
	}
	if got := trayProblems(wfs, tests); len(got) > 0 {
		t.Errorf("job tray の守りが破れている:\n%s", strings.Join(got, "\n"))
	}

	// 検査が当たる場合があること: 落とすべき書き方ごとに本物のファイルを書き換え、理由が出ることを見る
	const tray, testsYML = ".github/workflows/tray.yml", ".github/workflows/tests.yml"
	extraJob := "\n  extra:\n    runs-on: ubuntu-24.04\n    steps:\n"
	bad := []struct {
		what, file, old, new, want string
	}{
		{"secrets を添字で読む", tray, "        run: bun run test\n", "        run: bun run test\n        env:\n          K: ${{ secrets['K'] }}\n", "式"},
		{"secrets を丸ごと読む", tray, "        run: bun run typecheck\n", "        run: bun run typecheck\n        env:\n          K: ${{ toJSON(secrets) }}\n", "式"},
		{"token を添字で読む", tray, "        run: bun run build\n", "        run: bun run build\n        env:\n          T: ${{ github['token'] }}\n", "式"},
		{"workflow の権限を広げる", tray, "permissions:\n  contents: read\njobs:", "permissions: write-all\njobs:", "権限"},
		{"job の権限を広げる", tray, "    permissions:\n      contents: read\n", "    permissions:\n      contents: write\n", "権限"},
		{"install のあとにスクリプトを許す", tray, "bun install --frozen-lockfile --ignore-scripts", "bun install --frozen-lockfile --ignore-scripts && bun pm trust --all", "run の行"},
		{"別の step で入れ直す", tray, "      - name: Typecheck\n", "      - run: bun i\n      - name: Typecheck\n", "run の行"},
		{"検査の step を止める", tray, "      - name: Typecheck\n", "      - name: Typecheck\n        if: false\n", "キー"},
		{"落ちても通す", tray, "      - name: Workers tests\n", "      - name: Workers tests\n        continue-on-error: true\n", "キー"},
		{"端から端の検査を外す", tray, "run: bun run e2e", "run: echo e2e", "run の行"},
		{"checkout が資格情報を残す", tray, "persist-credentials: false", "persist-credentials: true", "persist-credentials"},
		{"job を container で走らせる", tray, "    timeout-minutes: 20\n", "    timeout-minutes: 20\n    container: node:26\n", "キー"},
		{"知らない action を使う", tray, "      - name: Typecheck\n", "      - uses: ./tray/ci\n      - name: Typecheck\n", "uses"},
		{"main の push で回さない", tray, "  push:\n    branches: [main]\n", "", "on"},
		{"ほかの workflow でローカルの action", testsYML, "", extraJob + "      - uses: ./tray/ci\n", "ほかの workflow"},
		{"ほかの workflow で tray に cd", testsYML, "", extraJob + "      - run: npm ci\n        working-directory: tray\n", "ほかの workflow"},
		{"ほかの workflow で docker の bun", testsYML, "", extraJob + "      - uses: docker://oven/bun:1.4.5\n", "ほかの workflow"},
		{"ほかの workflow で bunx", testsYML, "", extraJob + "      - run: bunx vitest run\n", "ほかの workflow"},
		{"tests の新しい検査が tray を走らせる", "tests/tray.sh", "", "#!/bin/bash\n(cd \"$root/tray\" && npm ci && node scripts/e2e.mjs)\n", "tests"},
		{"tests の置き場が bun を走らせる", "tests/package/99-x.sh", "", "bun run test\n", "tests"},
		{"振り分けの道具の行が tray を走らせる", "tests/ci.sh", "", "\n(cd tray && node scripts/e2e.mjs)\n", "tests"},
	}
	for _, c := range bad {
		w, ts := maps.Clone(wfs), maps.Clone(tests)
		m := w
		if strings.HasPrefix(c.file, "tests/") {
			m = ts
		}
		orig := m[c.file]
		if c.old == "" {
			m[c.file] = orig + c.new
		} else {
			if !strings.Contains(orig, c.old) {
				t.Errorf("%s: 書き換える元 %q が %s に無い（自己確認が何も見ていない）", c.what, c.old, c.file)
				continue
			}
			m[c.file] = strings.Replace(orig, c.old, c.new, 1)
		}
		got := strings.Join(trayProblems(w, ts), "\n")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: 理由に %q が出ない（素通りした）。出た理由: %q", c.what, c.want, got)
		}
		if strings.Contains(got, "読めない") {
			t.Errorf("%s: 書き換えた YAML が読めない（形の誤りで当たっただけ）: %s", c.what, got)
		}
	}
}

// trayProblems: job tray の守りが破れている理由を返す。wfs は workflow のパスと中身、tests は tests/ の下のファイルのパスと中身。
func trayProblems(wfs, tests map[string]string) []string {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	const trayFile = ".github/workflows/tray.yml"
	sha := regexp.MustCompile(`@[0-9a-f]{40}$`)
	exprRe := regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	// ほかの workflow で、Bun・tray のコード・ローカルの action・docker の action を走らせる形
	other := regexp.MustCompile(`\bbunx?\b|\btray\b|playwright|docker://|uses:\s*["']?\./`)
	bunRe := regexp.MustCompile(`\bbunx?(\s|$|")`)
	trayRe := regexp.MustCompile(`\btray\b`)
	runs := regexp.MustCompile(`\bcd\b|\bnode\b|\bnpm\b|\bnpx\b|scripts/|node_modules`)

	src, ok := wfs[trayFile]
	if !ok {
		return []string{trayFile + " が無い"}
	}
	for f, text := range wfs {
		if f == trayFile {
			continue
		}
		for i, l := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "#") {
				continue
			}
			if other.MatchString(l) {
				bad("ほかの workflow %s:%d が Bun か tray を走らせる: %s", f, i+1, strings.TrimSpace(l))
			}
		}
	}
	for f, text := range tests {
		for i, l := range strings.Split(text, "\n") {
			if bunRe.MatchString(l) {
				bad("tests の %s:%d が Bun を走らせる", f, i+1)
			}
			if !trayRe.MatchString(l) {
				continue
			}
			// tray を名指してよいのは、パスを振り分ける道具とその確かめだけ。そこでも走らせる行は書かない
			switch f {
			case "tests/ci.sh", "tests/lib/scope.sh", "tests/lib/image-scope.sh":
				if runs.MatchString(l) {
					bad("tests の %s:%d が tray を走らせる", f, i+1)
				}
			default:
				bad("tests の %s:%d が tray を名指す（振り分けの道具の外）", f, i+1)
			}
		}
	}

	// tray.yml: 式は mise.toml から読んだバージョンだけ
	for _, m := range exprRe.FindAllStringSubmatch(src, -1) {
		switch strings.TrimSpace(m[1]) {
		case "steps.tools.outputs.bun", "steps.tools.outputs.node":
		default:
			bad("tray.yml の式 %q は許していない（秘密や token を読める）", m[0])
		}
	}
	var wf map[string]any
	if err := yaml.Unmarshal([]byte(src), &wf); err != nil {
		return append(out, "tray.yml が読めない: "+err.Error())
	}
	keysIn := func(where string, m map[string]any, allow ...string) {
		for k := range m {
			if !slices.Contains(allow, k) {
				bad("tray.yml の%sのキー %s は許していない", where, k)
			}
		}
	}
	readOnly := func(perm any) bool {
		m, ok := perm.(map[string]any)
		return ok && len(m) == 1 && m["contents"] == "read"
	}
	keysIn(" workflow ", wf, "name", "on", "permissions", "jobs")
	if !readOnly(wf["permissions"]) {
		bad("tray.yml の workflow の権限が contents: read だけでない: %v", wf["permissions"])
	}
	on, _ := wf["on"].(map[string]any)
	push, _ := on["push"].(map[string]any)
	branches, _ := push["branches"].([]any)
	if pr, ok := on["pull_request"]; !ok || pr != nil || len(on) != 2 || len(branches) != 1 || branches[0] != "main" {
		bad("tray.yml の on が PR と main の push の2つでない: %v", wf["on"])
	}
	jobs, _ := wf["jobs"].(map[string]any)
	job, _ := jobs["tray"].(map[string]any)
	if len(jobs) != 1 || job == nil {
		return append(out, fmt.Sprintf("tray.yml の job が tray の1つでない: %d 個", len(jobs)))
	}
	keysIn(" job ", job, "runs-on", "timeout-minutes", "permissions", "defaults", "steps")
	if !readOnly(job["permissions"]) {
		bad("tray.yml の job tray の権限が contents: read だけでない: %v", job["permissions"])
	}
	// run の行は一覧のとおり。検査の行は1回ずつ要る
	allowLines := map[string]bool{
		"set -euo pipefail": true,
		`bun=$(sed -n 's/^bun = "\(.*\)"$/\1/p' ../mise.toml)`:                                             true,
		`node=$(sed -n 's/^node = "\(.*\)"$/\1/p' ../mise.toml)`:                                           true,
		`[ -n "$bun" ] && [ -n "$node" ] || { echo "::error::mise.toml に bun か node のバージョンが無い"; exit 1; }`: true,
		`echo "bun=$bun" >>"$GITHUB_OUTPUT"`:                                                               true,
		`echo "node=$node" >>"$GITHUB_OUTPUT"`:                                                             true,
		"bunx playwright install --with-deps chromium":                                                     true,
	}
	need := map[string]int{
		"bun install --frozen-lockfile --ignore-scripts": 0, "bun run typecheck": 0, "bun run test": 0,
		"bun run build": 0, "bun run e2e": 0, "bun run browser": 0,
	}
	allowUses := map[string][]string{
		"actions/checkout@": {"persist-credentials"}, "oven-sh/setup-bun@": {"bun-version"}, "actions/setup-node@": {"node-version"},
	}
	steps, _ := job["steps"].([]any)
	for i, x := range steps {
		s, _ := x.(map[string]any)
		keysIn(fmt.Sprintf(" step %d の", i+1), s, "name", "id", "uses", "with", "run")
		if u, ok := s["uses"].(string); ok {
			prefix := ""
			for p := range allowUses {
				if strings.HasPrefix(u, p) && sha.MatchString(u) {
					prefix = p
				}
			}
			if prefix == "" {
				bad("tray.yml の step %d の uses %s は許していない（3つの action を commit の SHA で）", i+1, u)
				continue
			}
			with, _ := s["with"].(map[string]any)
			keysIn(fmt.Sprintf(" step %d の with の", i+1), with, allowUses[prefix]...)
			if prefix == "actions/checkout@" && with["persist-credentials"] != false {
				bad("tray.yml の checkout が persist-credentials: false でない")
			}
		}
		run, _ := s["run"].(string)
		for _, l := range strings.Split(run, "\n") {
			l = strings.TrimSpace(l)
			if l == "" || allowLines[l] {
				continue
			}
			if _, ok := need[l]; ok {
				need[l]++
				continue
			}
			bad("tray.yml の step %d の run の行 %q は許していない", i+1, l)
		}
	}
	for l, n := range need {
		if n != 1 {
			bad("tray.yml の run の行 %q が %d 回（1回のはず）", l, n)
		}
	}
	return out
}
