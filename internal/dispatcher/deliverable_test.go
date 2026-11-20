package dispatcher

import (
	"os"
	"path/filepath"
	"testing"

	"andon/internal/ledger"
)

// gate を回す（レビュアーを起動する）のは案件 repo の担当の PR の課題だけ。
// deliverable: candidates は飛ばす（ADR-0006 受け入れ条件 3）。最後に書いたのが今の Chief of Staff でなくても飛ばす
// （Chief of Staff が起き直してアドレスが変わり、書き直す前。ADR-0006 決めたこと 17）。
func TestGated(t *testing.T) {
	l := ledger.At(t.TempDir())
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	p := &project{l: l, rows: []ledger.Row{
		{ID: "D-0001", By: "terminal:t5", Change: map[string]any{"deliverable": "candidates"}},
		{ID: "D-0002", By: "terminal:t7", Change: map[string]any{"deliverable": "candidates"}},
	}}
	const repo = "github.com/example/app"
	for _, c := range []struct {
		name string
		task ledger.Issue
		want bool
	}{
		{"直す課題", ledger.Issue{"id": "D-0003", "repo": repo, "profile": "claude"}, true},
		{"調べる課題（Chief of Staff が書いた）", ledger.Issue{"id": "D-0001", "repo": repo, "profile": "claude", "deliverable": "candidates"}, false},
		{"調べる課題（照合が外れた。前の Chief of Staff が書いた）", ledger.Issue{"id": "D-0002", "repo": repo, "profile": "claude", "deliverable": "candidates"}, false},
		{"知らない deliverable は pr", ledger.Issue{"id": "D-0006", "repo": repo, "profile": "claude", "deliverable": "survey"}, true},
		{"repo の無い課題", ledger.Issue{"id": "D-0004", "profile": "claude"}, false},
		{"Chief Engineer", ledger.Issue{"id": "D-0005", "repo": repo, "profile": "chief-engineer"}, false},
		{"レビュアー", ledger.Issue{"id": "D-0003-r1", "repo": repo, "profile": "codex-review", "parent": "D-0003"}, false},
	} {
		if got := p.gated(c.task); got != c.want {
			t.Errorf("%s: gated = %v, want %v", c.name, got, c.want)
		}
	}
}
