package ledger

import (
	"path/filepath"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
)

func faultLedger(f *ledgertest.Fault) (setup func(t *testing.T), cur func() *Ledger, state func(t *testing.T) string) {
	var l *Ledger
	setup = func(t *testing.T) {
		t.Setenv("HC_TERMINAL_ID", "t5")
		t.Setenv("DECK_NOW", "2026-11-23T10:00:00Z")
		root := filepath.Join(t.TempDir(), "sample-site")
		f.Kanban(root, "", nil, "init", "--root", root, "--prefix", "T")
		f.Kanban(root, "", nil, "config", "set", "chief_of_staff=t5")
		l = At(root)
	}
	state = func(t *testing.T) string {
		rows := l.Log()
		return f.Snapshot(l.Root, func(id, key string) bool {
			t, err := l.Show(id)
			return err == nil && l.Trusted(rows, t, key)
		})
	}
	return setup, func() *Ledger { return l }, state
}

func TestNewOwnerQuestionConvergesAfterAnyStop(t *testing.T) {
	f := ledgertest.NewFault(t)
	setup, cur, state := faultLedger(f)
	q, err := contract.ParseQuestion([]byte(`{"kind":"users","question":"どちらの画面にするか","options":[{"label":"一覧","pick":true},{"label":"表"}],"facts":["docs/a.md"],"from":"chief-of-staff"}`))
	if err != nil {
		t.Fatal(err)
	}
	f.Converge(t, ledgertest.Case{
		Setup: setup,
		Run: func() error {
			_, _, err := cur().NewOwnerQuestion(q, "", nil, Mark{})
			return err
		},
		Tries: 2,
		State: state,
	})
}

// 置いておく印も、どこで止まっても打ち直しで同じ形に揃う（印は who と同じ kanban set で、問いを置く前に書く）。
func TestNewOwnerQuestionWithMarkConvergesAfterAnyStop(t *testing.T) {
	f := ledgertest.NewFault(t)
	setup, cur, state := faultLedger(f)
	q, err := contract.ParseQuestion([]byte(`{"kind":"users","question":"どちらの画面にするか","options":[{"label":"一覧","pick":true},{"label":"表"}],"facts":["docs/a.md"],"from":"chief-of-staff"}`))
	if err != nil {
		t.Fatal(err)
	}
	f.Converge(t, ledgertest.Case{
		Setup: setup,
		Run: func() error {
			_, _, err := cur().NewOwnerQuestion(q, "", nil, Mark{Later: "急がない", Until: "2026-12-01"})
			return err
		},
		Tries: 2,
		State: state,
	})
}
