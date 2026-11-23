package retro

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

// andon retro apply は、kanban に書く所（担当の課題の add・予約キーの set、オーナーの課題の add・set・ask の、前・
// ファイルと log.jsonl の間・後）のどこで止まっても、同じ command を打ち直せば、止まらなかった回と同じ課題・予約キー
// （履歴の照合まで）・問い・問いの控え・記録（retro-applied.json）に揃う。
func TestApplyConvergesAfterAnyStop(t *testing.T) {
	f := ledgertest.NewFault(t)
	const retro = "T-0001"
	var l *ledger.Ledger
	var plans []plan
	setup := func(t *testing.T) {
		t.Setenv("HC_TERMINAL_ID", "t5")
		t.Setenv("DECK_NOW", "2026-11-23T10:00:00Z")
		root := filepath.Join(t.TempDir(), "sample-site")
		env := []string{"HC_TERMINAL_ID=t5"}
		f.Kanban(root, "", env, "init", "--root", root, "--prefix", "T", "--repo", "github.com/example/app")
		f.Kanban(root, "", env, "config", "set", "chief_of_staff=t5")
		f.Kanban(root, "", env, "add", "振り返り")
		l = ledger.At(root)
		if err := os.MkdirAll(l.Library(retro), 0o755); err != nil {
			t.Fatal(err)
		}
		in, repos := &Input{}, []string{"github.com/example/app"}
		plans = []plan{
			planOf(retro, Pattern{ID: "p1", Summary: "直す A", Target: Target{Kind: "project", Repo: "github.com/example/app", Paths: []string{"AGENTS.md"}}}, in, repos),
			planOf(retro, Pattern{ID: "p2", Summary: "B", Target: Target{Kind: "andon", Paths: []string{"package/skills/worker/SKILL.md"}}}, in, repos),
		}
	}
	run := func() error {
		rec, err := loadApplied(l, retro)
		if err != nil {
			return err
		}
		return applyPlans(l, retro, plans, []Dropped{{Pattern: "p3", Reason: "一度きり"}}, rec, io.Discard)
	}
	state := func(t *testing.T) string {
		rows := l.Log()
		s := f.Snapshot(l.Root, func(id, key string) bool {
			is, err := l.Show(id)
			return err == nil && l.Trusted(rows, is, key)
		})
		b, err := os.ReadFile(filepath.Join(l.Library(retro), appliedFile))
		if err != nil {
			t.Fatal(err)
		}
		var rec Applied
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
		rec.AppliedAt = ""
		got, _ := json.Marshal(rec)
		return s + "record " + string(got) + "\n"
	}
	f.Converge(t, ledgertest.Case{Setup: setup, Run: run, Tries: 2, State: state})
}
