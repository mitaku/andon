package dispatcher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

// 端末の外から書かれた答え（オーナーのペイン）は通知 answered を1回積み、Chief of Staff・dispatcher・ほかの端末が書いた答えは
// 積まない。同じ問いを2度読んでも2度は積まない（ADR-0042 決めたこと 18・19。受け入れ条件 15）。
func TestAnsweredFromPane(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	l := ledger.At(filepath.Join(d, "projects", "sample-site"))
	logPath := filepath.Join(l.Dir, "log.jsonl")
	qpath := filepath.Join(l.Dir, "issues", "S-0030", "questions.yaml")
	writeFile(t, logPath, ledgertest.Log(t, `{"at":"2026-11-21T00:00:00Z","by":"terminal:t51","id":"S-0030","change":{"status":"doing"}}`))
	writeFile(t, filepath.Join(l.Library("S-0030"), "questions", "1.json"),
		`{"kind":"users","question":"書き出しは CSV だけにするか","options":[{"label":"CSV だけ","pick":true},{"label":"Excel も"}],"from":"issue"}`)
	questions := func(st1, st2, st3 string) {
		writeFile(t, qpath, "- n: 1\n  text: 書き出しは CSV だけにするか\n  kind: users\n  status: "+st1+"\n  answer: CSV だけ\n"+
			"- n: 2\n  text: 範囲\n  kind: scope\n  status: "+st2+"\n  answer: c1 だけ\n"+
			"- n: 3\n  text: 進めるか\n  kind: line\n  status: "+st3+"\n  answer: 進める\n")
	}
	slug, cos := l.Slug, "t51"
	tasks := []ledger.Issue{{"id": "S-0030", "status": "doing", "agent": "t42", "profile": "claude", "repo": "github.com/example/sample-site"}}
	pass := func(rows ...string) []Notification {
		t.Helper()
		st, _, _ := LoadState(slug)
		p := &project{c: &Dispatcher{Log: os.Stderr}, l: l, st: st, tasks: tasks}
		for _, r := range rows {
			addRow(t, logPath, r)
		}
		p.readLog(cos)
		if err := p.st.Save(slug); err != nil {
			t.Fatal(err)
		}
		var out []Notification
		for _, n := range p.st.Pending {
			if n.Kind == "answered" {
				out = append(out, n)
			}
		}
		return out
	}
	pass() // 位置を控える

	// 積む: 書き手が端末でない答えの行（andon waiting answer が端末の外で kanban answer を打った）
	questions("answered", "open", "open")
	got := pass(`{"at":"2026-11-21T00:01:00Z","by":"owner","id":"S-0030","change":{"question":1,"status":"answered"}}`)
	if len(got) != 1 || got[0].Issue != "S-0030" || fmt.Sprint(got[0].Detail["n"]) != "1" || got[0].Detail["kind"] != "users" || got[0].Detail["answer"] != "CSV だけ" {
		t.Fatalf("ペインの答えの通知 = %+v（S-0030 の問い 1・users・「CSV だけ」の1件のはず）", got)
	}
	if _, err := contract.Encode(contract.SchemaNotification, got[0]); err != nil {
		t.Fatalf("通知 answered が schema に合わない: %v", err)
	}
	if s := line(got[0]); s != "S-0030 answered: 問い 1（users）にオーナーがペインで「CSV だけ」と答えた" {
		t.Errorf("<news> の行 = %q", s)
	}

	// 積まない: Chief of Staff の端末・dispatcher・ほかの端末が書いた答えの行
	questions("answered", "answered", "answered")
	got = pass(
		`{"at":"2026-11-21T00:02:00Z","by":"terminal:t51","id":"S-0030","change":{"question":2,"status":"answered"}}`,
		`{"at":"2026-11-21T00:02:01Z","by":"terminal:dispatcher","id":"S-0030","change":{"question":3,"status":"answered"}}`,
		`{"at":"2026-11-21T00:02:02Z","by":"terminal:t7","id":"S-0030","change":{"question":3,"status":"answered"}}`)
	if len(got) != 1 || fmt.Sprint(got[0].Detail["n"]) != "1" {
		t.Fatalf("端末の中で書いた答えも通知にした: %+v（問い 1 の1件のまま）", got)
	}

	// 2度目は積まない: 担当が受け取ったあと、log.jsonl を先頭から読み直しても（先頭の行が替わった）積み直さない
	questions("received", "received", "received")
	st, _, _ := LoadState(slug)
	st.Pending = nil
	if err := st.Save(slug); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(logPath)
	writeFile(t, logPath, ledgertest.Log(t, `{"at":"2026-11-20T00:00:00Z","by":"terminal:t51","id":"S-0030","change":{"status":"todo"}}`)+string(b))
	if got = pass(); len(got) != 0 {
		t.Fatalf("受け取った問いの答えを、読み直しでまた積んだ: %+v", got)
	}
	// 受け取る前に同じ問いの行を2度読んでも、通知は問いごとに1件（eventKey）
	questions("answered", "open", "open")
	row := `{"at":"2026-11-21T00:03:00Z","by":"owner","id":"S-0030","change":{"question":1,"status":"answered"}}`
	if got = pass(row, row); len(got) != 1 {
		t.Fatalf("同じ問いの答えを2件積んだ: %+v", got)
	}
	// 同じ課題の別の問いの答えは、前の問いの通知を消さない（eventKey は問いの n ごと）
	questions("answered", "open", "answered")
	if got = pass(`{"at":"2026-11-21T00:04:00Z","by":"owner","id":"S-0030","change":{"question":3,"status":"answered"}}`); len(got) != 2 {
		t.Fatalf("問い 3 の答えが問い 1 の通知を消した・積まれない: %+v", got)
	}
}
