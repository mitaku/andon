package ledger

import (
	"path/filepath"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
)

// faultLedger は本物の kanban の CLI を Chief of Staff の端末 t5 で書く置き場を、Setup のたびに作り直す（Fault.Converge の下ごしらえ）。
// 返す関数は、今の置き場の Ledger と、その記録と課題（Fault.Snapshot。予約キーの履歴の照合つき）を返す。
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

// andon issue ask --new（NewOwnerQuestion）は、kanban に書く所（add・set・ask の前・ファイルと log.jsonl の間・後）の
// どこで止まっても、同じ command を打ち直せば、止まらなかった回と同じ課題・who・問い・問いの控えに揃う
// （課題を2つ足さない。問いを2つ置かない。控えの無い問いを残さない）。
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
			_, _, err := cur().NewOwnerQuestion(q, "", nil)
			return err
		},
		Tries: 2,
		State: state,
	})
}
