package issue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

func TestStartFillConvergesAfterAnyStop(t *testing.T) {
	const repo = "github.com/example/sample-site"
	record := func(t *testing.T, root string, item map[string]any) {
		dir := filepath.Join(root, "kanban", "library", "triage")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(map[string]any{"items": []map[string]any{item}})
		if err := os.WriteFile(filepath.Join(dir, "2026-11-23-T-0009.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		by   string
		made func(t *testing.T, f *ledgertest.Fault, root string)
	}{
		{"routine", "dispatcher", func(t *testing.T, f *ledgertest.Fault, root string) {
			f.Kanban(root, "", []string{"HC_TERMINAL_ID=dispatcher"}, "set", "T-0001", "routine=nightly")
			row := ledgertest.Log(t, `{"at":"2026-11-23T10:00:00Z","by":"routine:nightly","routine":"nightly","fired":"T-0001"}`)
			lf, err := os.OpenFile(filepath.Join(root, "kanban", "log.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			defer lf.Close()
			if _, err := lf.WriteString(row); err != nil {
				t.Fatal(err)
			}
		}},
		{"triage created_issue", "t5", func(t *testing.T, _ *ledgertest.Fault, root string) {
			record(t, root, map[string]any{"key": "k", "decision": "fix", "created_issue": "T-0001", "issue": map[string]any{"title": "ボタンを足す", "review": "on"}})
		}},
		{"triage adding", "t5", func(t *testing.T, _ *ledgertest.Fault, root string) {
			record(t, root, map[string]any{"key": "k", "decision": "bundle", "adding": true, "issue": map[string]any{"title": "ボタンを足す", "review": "off"}})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := ledgertest.NewFault(t)
			var cur *ctx
			setup := func(t *testing.T) {
				t.Setenv("HC_TERMINAL_ID", "t5")
				t.Setenv("DECK_NOW", "2026-11-23T10:00:00Z")
				root := filepath.Join(t.TempDir(), "sample-site")
				f.Kanban(root, "", nil, "init", "--root", root, "--prefix", "T", "--repo", repo)
				f.Kanban(root, "", nil, "config", "set", "chief_of_staff=t5")
				f.Kanban(root, "", []string{"HC_TERMINAL_ID=" + c.by}, "add", "ボタンを足す", "--repo", repo, "--profile", "claude")
				c.made(t, f, root)
				cur = &ctx{l: ledger.At(root), id: "T-0001"}
			}
			run := func() error {
				is, err := cur.l.Show(cur.id)
				if err != nil {
					return err
				}
				cur.issue = is
				p := &startPlan{Profile: "claude"}
				if err := cur.checkSettings(p); err != nil {
					return err
				}
				return cur.fill(p)
			}
			state := func(t *testing.T) string {
				rows := cur.l.Log()
				return f.Snapshot(cur.l.Root, func(id, key string) bool {
					is, err := cur.l.Show(id)
					return err == nil && cur.l.Trusted(rows, is, key)
				})
			}
			f.Converge(t, ledgertest.Case{Setup: setup, Run: run, State: state})
		})
	}
}
