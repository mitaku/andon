package tray

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// deployIn は検査の間だけ、パッケージの置き場を一時の場所にして deploy.mjs を置き、node と起動の口を差し替える。
// 返すのは起動した引数の控え。
func deployIn(t *testing.T) *[][]string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("ANDON_HOME", home)
	if Dir() != filepath.Join(home, "tray") {
		t.Skipf("パッケージの置き場を差し替えられない（Dir() = %s）", Dir())
	}
	if err := os.MkdirAll(filepath.Join(Dir(), "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(), "scripts", "deploy.mjs"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	prevCmd, prevNode := deployCmd, lookNode
	deployCmd = func(node string, args []string, _, _ io.Writer) error {
		calls = append(calls, append([]string{node}, args...))
		return nil
	}
	lookNode = func() (string, error) { return "/usr/bin/node", nil }
	t.Cleanup(func() { deployCmd, lookNode = prevCmd, prevNode })
	return &calls
}

// 決めたこと 27: トークンはこの1回の実行の環境変数にだけ渡す。無ければ打たない（--dry-run は Cloudflare に触れないので要らない）。
func TestDeployNeedsToken(t *testing.T) {
	calls := deployIn(t)
	cfg := filepath.Join(t.TempDir(), "tray-deploy.json")
	if err := os.WriteFile(cfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	var out, errb bytes.Buffer
	if rc := DeployMain([]string{"--config", cfg}, nil, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "CLOUDFLARE_API_TOKEN が無い") || len(*calls) != 0 {
		t.Fatalf("トークンが無いのに打った: rc=%d err=%q calls=%v", rc, errb.String(), *calls)
	}
	if rc := DeployMain([]string{"--config", cfg, "--dry-run"}, nil, &out, &errb); rc != 0 || len(*calls) != 1 {
		t.Fatalf("--dry-run: rc=%d err=%q", rc, errb.String())
	}
	want := []string{"/usr/bin/node", filepath.Join(Dir(), "scripts", "deploy.mjs"), "--config", cfg, "--dry-run"}
	if !slices.Equal((*calls)[0], want) {
		t.Errorf("起動した引数 = %v（%v のはず）", (*calls)[0], want)
	}
	t.Setenv("CLOUDFLARE_API_TOKEN", "x")
	if rc := DeployMain([]string{"--config", cfg}, nil, &out, &errb); rc != 0 || len(*calls) != 2 || slices.Contains((*calls)[1], "--dry-run") {
		t.Errorf("トークンのある本番: rc=%d calls=%v", rc, *calls)
	}
}

func TestDeployArgs(t *testing.T) {
	calls := deployIn(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "x")
	for _, args := range [][]string{nil, {"--config"}, {"--config", ""}, {"--dry-run"}, {"--config", "a.json", "extra"}, {"--config", filepath.Join(t.TempDir(), "none.json")}} {
		var out, errb bytes.Buffer
		if rc := DeployMain(args, nil, &out, &errb); rc != 2 {
			t.Errorf("%v: rc=%d（2 のはず）err=%q", args, rc, errb.String())
		}
	}
	if len(*calls) != 0 {
		t.Errorf("断るはずの引数で打った: %v", *calls)
	}
}
