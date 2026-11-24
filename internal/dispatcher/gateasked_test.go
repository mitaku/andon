package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

func TestGateAskedDeliveredOnce(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	root := filepath.Join(d, "projects", "sample-site")
	l := ledger.At(root)
	qdir := filepath.Join(l.Library("S-0007"), "questions")
	if err := os.MkdirAll(qdir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(l.Dir, "log.jsonl")
	writeFile(t, logPath, ledgertest.Log(t, `{"at":"2026-11-19T00:00:00Z","by":"terminal:t51","id":"S-0007","change":{"status":"doing"}}`))
	slug, cos := l.Slug, "t51"
	tasks := []ledger.Issue{{"id": "S-0007", "status": "review", "agent": "t42", "profile": "claude", "repo": "github.com/example/sample-site"}}
	load := func() *project {
		st, _, _ := LoadState(slug)
		return &project{c: &Dispatcher{Log: os.Stderr}, l: l, st: st, tasks: tasks}
	}

	p := load()
	p.readLog(cos)
	addRow(t, logPath, `{"at":"2026-11-19T00:01:00Z","by":"terminal:dispatcher","id":"S-0007","change":{"question":1,"status":"open"}}`)
	writeFile(t, filepath.Join(qdir, "1.json"), `{"kind":"line","question":"gate がオーナーに回した","facts":["PR https://github.com/example/sample-site/pull/1"],"reason":"gate の決定式が needs_human を返した","options":[{"label":"マージする","pick":true},{"label":"直させる"}],"from":"gate"}`)
	gateAsks(p, slug)
	if err := p.st.Save(slug); err != nil {
		t.Fatal(err)
	}
	first := stopTwice(t, slug, "p1")

	addRow(t, logPath, `{"at":"2026-11-19T00:02:00Z","by":"terminal:t42","id":"S-0007","change":{"question":2,"status":"open"}}`)
	writeFile(t, filepath.Join(qdir, "2.json"), `{"kind":"users","question":"担当が聞く","options":[{"label":"はい","pick":true},{"label":"いいえ"}],"from":"issue"}`)

	p = load()
	p.readLog(cos)
	if err := p.st.Save(slug); err != nil {
		t.Fatal(err)
	}
	if len(p.st.Asked) != 0 {
		t.Errorf("問いの行を読んだのに印が残った: %v", p.st.Asked)
	}
	second := stopTwice(t, slug, "p2")

	if !strings.Contains(first, "問い 1") {
		t.Fatalf("巡 1 のあとのターンが gate の問い 1 を渡さない: %q", first)
	}
	if !strings.Contains(second, "問い 2") {
		t.Errorf("巡 2 のあとのターンが担当の問い 2 を渡さない: %q", second)
	}
	if strings.Contains(second, "問い 1") {
		t.Errorf("gate の問い 1 が2度届いた: %q", second)
	}
	b, _ := os.ReadFile(DeliveredPath(slug))
	var asked []int
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var dl Delivered
		if json.Unmarshal([]byte(ln), &dl) != nil {
			t.Fatalf("届けた記録が読めない: %q", ln)
		}
		n := 0
		for _, x := range dl.News {
			if x.Issue == "S-0007" && x.Kind == "asked" {
				n++
			}
		}
		asked = append(asked, n)
	}
	if len(asked) != 2 || asked[0] != 1 || asked[1] != 1 {
		t.Errorf("届けた asked の数がターンごとに %v（[1 1] のはず）。届けた記録:\n%s", asked, b)
	}
}

func TestGateAskedRecoveredAfterCrash(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	l := ledger.At(filepath.Join(d, "projects", "sample-site"))
	qdir := filepath.Join(l.Library("S-0007"), "questions")
	logPath := filepath.Join(l.Dir, "log.jsonl")
	writeFile(t, logPath, ledgertest.Log(t, `{"at":"2026-11-19T00:00:00Z","by":"terminal:t51","id":"S-0007","change":{"status":"review"}}`))
	slug, cos := l.Slug, "t51"
	tasks := []ledger.Issue{{"id": "S-0007", "status": "review", "agent": "t42", "profile": "claude", "repo": "github.com/example/sample-site"}}
	load := func() *project {
		st, _, _ := LoadState(slug)
		return &project{c: &Dispatcher{Log: os.Stderr}, l: l, st: st, tasks: tasks}
	}
	pass := func(crash bool, ask bool) {
		p := load()
		p.readLog(cos)
		if ask {
			addRow(t, logPath, `{"at":"2026-11-19T00:01:00Z","by":"terminal:dispatcher","id":"S-0007","change":{"question":1,"status":"open"}}`)
			writeFile(t, filepath.Join(qdir, "1.json"), `{"kind":"line","question":"gate がオーナーに回した","options":[{"label":"マージする","pick":true},{"label":"直させる"}],"from":"gate"}`)
			gateAsks(p, slug)
		}
		if crash {
			return
		}
		if err := p.st.Save(slug); err != nil {
			t.Fatal(err)
		}
	}
	pass(false, false)
	pass(true, true)
	pass(false, false)
	if news := stopTwice(t, slug, "p1"); !strings.Contains(news, "問い 1") {
		t.Fatalf("保存の前に落ちた gate の問いが届かない: %q", news)
	}
	pass(false, false)
	if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p2"}); err != nil || out != nil {
		t.Fatalf("gate の問いをもう一度渡した: %v %v", out, err)
	}
}

func gateAsks(p *project, slug string) {
	p.put(Notification{At: now(), Project: slug, Issue: "S-0007", Kind: "asked", Detail: map[string]any{"n": float64(1), "kind": "line", "question": "gate がオーナーに回した"}}, p.facts("S-0007"))
	p.markAsked("S-0007", float64(1))
}

func stopTwice(t *testing.T, slug, prompt string) string {
	t.Helper()
	out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: prompt})
	if err != nil || out == nil {
		t.Fatalf("Stop で渡さない: %v %v", out, err)
	}
	if _, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: prompt, StopHookActive: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := out.(map[string]any)
	h, _ := m["hookSpecificOutput"].(map[string]any)
	news, _ := h["additionalContext"].(string)
	return news
}

func addRow(t *testing.T, path, row string) {
	t.Helper()
	if err := appendLine(path, []byte(strings.TrimSuffix(ledgertest.Log(t, row), "\n"))); err != nil {
		t.Fatal(err)
	}
}
