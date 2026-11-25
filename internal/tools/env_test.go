package tools

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/tools/toolstest"
)

// fakeEnv は端末のラベルと課題の worktree を表で返す envDeps。gitRoot は打った所の git の根。
func fakeEnv(labels map[string]string, worktrees map[string]string, gitRoot string) envDeps {
	return envDeps{
		terminal: "t7",
		labels: func(addr string) (map[string]string, error) {
			if labels == nil {
				return nil, errors.New("hc get t7 に失敗")
			}
			return labels, nil
		},
		worktree: func(slug, id string) (string, error) {
			wt, ok := worktrees[slug+"/"+id]
			if !ok {
				return "", errors.New("課題が無い")
			}
			return wt, nil
		},
		gitRoot: func() (string, error) {
			if gitRoot == "" {
				return "", errors.New("git の外")
			}
			return gitRoot, nil
		},
	}
}

func TestToolsEnv(t *testing.T) {
	// mise を呼ばないこと: 偽の mise を置き、どの場合も呼ばれていないことを最後に見る
	f := toolstest.New(t, "")
	wt, kanban, other, bare := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	toolstest.Manifest(t, wt)
	toolstest.Manifest(t, other)
	want := "MISE_SYSTEM_CONFIG_FILE=" + filepath.Join(wt, "mise.toml")
	reviewer := map[string]string{"project": "sample-site", "issue": "S-0007-r1"}
	cases := []struct {
		name string
		d    envDeps
		line string
	}{
		// レビュアー: 打った所（cwd）は kanban の課題のフォルダで、git の根はセルの repo。出るのは課題の worktree の根
		{"レビュアーの cwd でも課題の worktree", fakeEnv(reviewer, map[string]string{"sample-site/S-0007-r1": wt}, kanban), want},
		{"担当", fakeEnv(map[string]string{"project": "sample-site", "issue": "S-0007"}, map[string]string{"sample-site/S-0007": wt}, other), want},
		// 課題が引けない・worktree が空なら、打った所の git の根
		{"ラベルが読めない", fakeEnv(nil, nil, other), "MISE_SYSTEM_CONFIG_FILE=" + filepath.Join(other, "mise.toml")},
		{"課題が無い", fakeEnv(reviewer, map[string]string{}, other), "MISE_SYSTEM_CONFIG_FILE=" + filepath.Join(other, "mise.toml")},
		{"worktree が空", fakeEnv(reviewer, map[string]string{"sample-site/S-0007-r1": ""}, other), "MISE_SYSTEM_CONFIG_FILE=" + filepath.Join(other, "mise.toml")},
		{"Chief of Staff（issue が無い）", fakeEnv(map[string]string{"project": "sample-site", "role": "chief-of-staff"}, nil, bare), ""},
		{"形の違うラベルはパスに組まない", fakeEnv(map[string]string{"project": "../x", "issue": "S-0007"}, map[string]string{"../x/S-0007": wt}, bare), ""},
		// 根に mise.toml が無ければ何も出さない
		{"worktree に mise.toml が無い", fakeEnv(reviewer, map[string]string{"sample-site/S-0007-r1": bare}, other), ""},
		{"git の外", fakeEnv(nil, nil, ""), ""},
	}
	for _, c := range cases {
		line, warn := c.d.env()
		if line != c.line || warn != "" {
			t.Errorf("%s: line=%q warn=%q want %q", c.name, line, warn, c.line)
		}
	}
	if got := f.Calls(); len(got) != 0 {
		t.Errorf("andon tools env が mise を呼んだ: %q", got)
	}
}

func TestToolsEnvSpaceInPath(t *testing.T) {
	wt := filepath.Join(t.TempDir(), "a b")
	os.Mkdir(wt, 0o755)
	toolstest.Manifest(t, wt)
	line, warn := fakeEnv(nil, nil, wt).env()
	if line != "" || !strings.Contains(warn, "空白") {
		t.Errorf("line=%q warn=%q", line, warn)
	}
}

// TestToolsEnvMain は葉を本物の配線で打つ: 偽の hc のラベルと、kanban の CLI の代わりの偽の kanban が返す worktree から 1 行出す。
// stdout は 1 行だけで、終了コードは 0。課題の無い端末は打った所の git の根を見る。
func TestToolsEnvMain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	f := toolstest.New(t, "")
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	wt := filepath.Join(dir, "wt")
	issueDir := filepath.Join(dir, "projects", "sample-site", "kanban", "issues", "S-0007-r1")
	for _, d := range []string{bin, wt, issueDir} {
		os.MkdirAll(d, 0o755)
	}
	toolstest.Manifest(t, wt)
	os.WriteFile(filepath.Join(bin, "hc"), []byte(`#!/bin/sh
[ "$1 $2 $3" = "get t7 --json" ] || exit 9
echo '{"labels":{"project":"sample-site","issue":"S-0007-r1"}}'
`), 0o755)
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(`#!/bin/sh
[ "$1 $2 $3" = "show S-0007-r1 --json" ] || exit 9
printf '{"issue":{"id":"S-0007-r1","worktree":"%s"}}\n' "$FAKE_WT"
`), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANDON_WORKSPACE", dir)
	t.Setenv("FAKE_WT", wt)
	t.Setenv("HC_TERMINAL_ID", "t7")
	t.Chdir(issueDir)
	var out, errb bytes.Buffer
	if rc := EnvMain(nil, nil, &out, &errb); rc != 0 || out.String() != "MISE_SYSTEM_CONFIG_FILE="+filepath.Join(wt, "mise.toml")+"\n" || errb.Len() != 0 {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	// 課題の無い端末（HC_TERMINAL_ID が無い）: 打った所の git の根。mise.toml の無い根なら空で 0
	t.Setenv("HC_TERMINAL_ID", "")
	repo := filepath.Join(dir, "repo")
	os.MkdirAll(filepath.Join(repo, "sub"), 0o755)
	if err := exec.Command("git", "init", "-q", repo).Run(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(repo, "sub"))
	out.Reset()
	if rc := EnvMain(nil, nil, &out, &errb); rc != 0 || out.Len() != 0 || errb.Len() != 0 {
		t.Errorf("mise.toml の無い根: rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	toolstest.Manifest(t, repo)
	root, _ := filepath.EvalSymlinks(repo)
	if rc := EnvMain(nil, nil, &out, &errb); rc != 0 || out.String() != "MISE_SYSTEM_CONFIG_FILE="+filepath.Join(root, "mise.toml")+"\n" {
		t.Errorf("git の根: rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	if rc := EnvMain([]string{"--x"}, nil, &out, &errb); rc != 2 {
		t.Errorf("引数を取った: rc=%d", rc)
	}
	if got := f.Calls(); len(got) != 0 {
		t.Errorf("andon tools env が mise を呼んだ: %q", got)
	}
}
