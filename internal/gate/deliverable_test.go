package gate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// deliverable: candidates のタスクに andon gate を打つと、回さずに理由1行と終了コード 3。記録を書かない
// （ADR-0006 受け入れ条件 4）。担当が書いた deliverable は信じず、今までどおり PR を見に行く。
func TestGateRefusesCandidates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "kanban")
	os.MkdirAll(filepath.Join(dir, "tasks", "D-0001", "library"), 0o755)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("version: 2\nchief_of_staff: t5\nrepos: [github.com/example/app]\n"), 0o644)
	bin := t.TempDir()
	// 偽の kanban（root と show だけ）・gh・git（呼ばれたら印を残す）
	kanban := "#!/bin/sh\ncase \"$1\" in\n  root) echo " + root + " ;;\n" +
		"  show) echo '{\"task\": {\"id\": \"D-0001\", \"status\": \"review\", \"repo\": \"github.com/example/app\", \"branch\": \"b\", \"deliverable\": \"candidates\"}}' ;;\n" +
		"esac\n"
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(kanban), 0o755)
	for _, n := range []string{"gh", "git"} {
		os.WriteFile(filepath.Join(bin, n), []byte("#!/bin/sh\necho called >> "+filepath.Join(bin, "called")+"\nexit 1\n"), 0o755)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANDON_WORKSPACE", t.TempDir())
	_, file, _, _ := runtime.Caller(0)
	t.Setenv("ANDON_HOME", filepath.Join(filepath.Dir(file), "..", "..", "package"))
	gate := func() (int, string, string) {
		var out, errb bytes.Buffer
		code := Main([]string{"D-0001"}, nil, &out, &errb)
		return code, out.String(), errb.String()
	}

	log := filepath.Join(dir, "log.jsonl")
	os.WriteFile(log, []byte(`{"at":"x","by":"terminal:t5","id":"D-0001","change":{"deliverable":"candidates"}}`+"\n"), 0o644)
	code, out, errs := gate()
	if code != 3 || out != "" || strings.Count(errs, "\n") != 1 || !strings.Contains(errs, "deliverable: candidates") {
		t.Fatalf("candidates: code=%d stdout=%q stderr=%q（終了コード 3 と理由1行のはず）", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(bin, "called")); err == nil {
		t.Error("candidates なのに gh・git を打った")
	}
	if _, err := os.Stat(filepath.Join(dir, "tasks", "D-0001", "library", "gate")); err == nil {
		t.Error("candidates なのに記録を書いた")
	}

	// 最後に書いたのが担当（t7）なら pr として読む: report.json が無いので fix（今までどおり）
	os.WriteFile(log, []byte(`{"at":"x","by":"terminal:t7","id":"D-0001","change":{"deliverable":"candidates"}}`+"\n"), 0o644)
	if code, out, errs := gate(); code != exitFix || !strings.Contains(out, "report.json") {
		t.Fatalf("担当が書いた candidates: code=%d stdout=%q stderr=%q（pr として読んで fix のはず）", code, out, errs)
	}
}
