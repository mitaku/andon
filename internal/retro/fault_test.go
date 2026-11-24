package retro

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

type retroFault struct {
	f     *ledgertest.Fault
	l     *ledger.Ledger
	plans []plan
}

const faultRetro = "T-0001"

func (r *retroFault) setup(t *testing.T) {
	t.Setenv("HC_TERMINAL_ID", "t5")
	t.Setenv("DECK_NOW", "2026-11-23T10:00:00Z")
	root := filepath.Join(t.TempDir(), "sample-site")
	env := []string{"HC_TERMINAL_ID=t5"}
	r.f.Kanban(root, "", env, "init", "--root", root, "--prefix", "T", "--repo", "github.com/example/app")
	r.f.Kanban(root, "", env, "config", "set", "chief_of_staff=t5")
	r.f.Kanban(root, "", env, "add", "振り返り")
	r.l = ledger.At(root)
	if err := os.MkdirAll(r.l.Library(faultRetro), 0o755); err != nil {
		t.Fatal(err)
	}
	in, repos := &Input{}, []string{"github.com/example/app"}
	r.plans = []plan{
		planOf(faultRetro, Pattern{ID: "p1", Summary: "直す A", Target: Target{Kind: "project", Repo: "github.com/example/app", Paths: []string{"AGENTS.md"}}}, in, repos, ""),
		planOf(faultRetro, Pattern{ID: "p2", Summary: "B", Target: Target{Kind: "andon", Paths: []string{"package/skills/worker/SKILL.md"}}}, in, repos, ""),
	}
}

func (r *retroFault) run() error {
	rec, err := loadApplied(r.l, faultRetro)
	if err != nil {
		return err
	}
	return applyPlans(r.l, faultRetro, r.plans, []Dropped{{Pattern: "p3", Reason: "一度きり"}}, rec, io.Discard)
}

func (r *retroFault) record(t *testing.T) Applied {
	b, err := os.ReadFile(filepath.Join(r.l.Library(faultRetro), appliedFile))
	if err != nil {
		t.Fatal(err)
	}
	var rec Applied
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func (r *retroFault) state(t *testing.T) string {
	rows := r.l.Log()
	s := r.f.Snapshot(r.l.Root, func(id, key string) bool {
		is, err := r.l.Show(id)
		return err == nil && r.l.Trusted(rows, is, key)
	})
	rec := r.record(t)
	rec.AppliedAt = ""
	got, _ := json.Marshal(rec)
	return s + "record " + string(got) + "\n"
}

func TestApplyConvergesAfterAnyStop(t *testing.T) {
	r := &retroFault{f: ledgertest.NewFault(t)}
	r.f.Converge(t, ledgertest.Case{Setup: r.setup, Run: r.run, Tries: 2, State: r.state})
}

func TestApplyRerunRefusesWhenTheQuestionChanged(t *testing.T) {
	r := &retroFault{f: ledgertest.NewFault(t)}
	r.setup(t)
	r.f.Arm(0)
	if err := r.run(); err != nil {
		t.Fatal(err)
	}
	last := r.f.Points()
	r.setup(t)
	r.f.Arm(last)
	if err := r.run(); err == nil {
		t.Fatal("止めたのに誤りを返さない")
	}
	r.f.Arm(0)
	changed := *r.plans[1].Question
	changed.Reason = "直す先が変わった"
	r.plans[1].Question = &changed
	if err := r.run(); err == nil || !strings.Contains(err.Error(), "書き直せない") {
		t.Fatalf("問いが変わって控えを書き直せないのに止まらない: %v", err)
	}
	if rec := r.record(t); rec.Complete || len(rec.Issues) != 1 {
		t.Errorf("控えの無いオーナーの課題を記録に残したか、当て済みにした: %+v", rec)
	}
	if _, err := os.Stat(filepath.Join(r.l.Library("T-0003"), "questions", "1.json")); err == nil {
		t.Error("違う問いの控えを書いた")
	}
}
