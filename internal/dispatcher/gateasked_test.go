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

// dispatcher 自身が出したゲートの問い（andon issue merge の asked）は、出した巡で積む1通だけが届く。
// 次の巡の readLog が by terminal:dispatcher の問いの行からもう一度積まない。担当の問いの行は今までどおり積む。
func TestGateAskedDeliveredOnce(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	t.Setenv("PATH", t.TempDir()) // hc も andon も無い（drain のフックの外の確かめは落ちて通す）
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

	// 巡 1: readLog が位置を控える。andon issue merge がゲートの問いを出し（by terminal:dispatcher の行と問いの JSON）、
	// 返した asked を merge の末尾と同じ put で積む
	p := load()
	p.readLog(cos)
	addRow(t, logPath, `{"at":"2026-11-19T00:01:00Z","by":"terminal:dispatcher","id":"S-0007","change":{"question":1,"status":"open"}}`)
	writeFile(t, filepath.Join(qdir, "1.json"), `{"kind":"line","question":"ゲートがオーナーに回した","facts":["PR https://github.com/example/sample-site/pull/1"],"reason":"ゲートの決定式が needs_human を返した","options":[{"label":"マージする","pick":true},{"label":"直させる"}],"from":"gate"}`)
	gateAsks(p, slug)
	if err := p.st.Save(slug); err != nil {
		t.Fatal(err)
	}
	// 巡の間: Stop の drain が渡し、続きのターンの Stop で確かめる
	first := stopTwice(t, slug, "p1")

	// 担当が自分の問いを出す（by は担当の端末）。この行は readLog が積む
	addRow(t, logPath, `{"at":"2026-11-19T00:02:00Z","by":"terminal:t42","id":"S-0007","change":{"question":2,"status":"open"}}`)
	writeFile(t, filepath.Join(qdir, "2.json"), `{"kind":"users","question":"担当が聞く","options":[{"label":"はい","pick":true},{"label":"いいえ"}],"from":"issue"}`)

	// 巡 2: readLog が問い 1 と問い 2 の行を読む
	p = load()
	p.readLog(cos)
	if err := p.st.Save(slug); err != nil {
		t.Fatal(err)
	}
	if len(p.st.Asked) != 0 {
		t.Errorf("問いの行を読んだのに印が残った: %v", p.st.Asked)
	}
	second := stopTwice(t, slug, "p2")

	// 巡 2 のあとのターンが渡すのは担当の問い 2 だけ。ゲートの問い 1 は巡 1 のあとのターンで1回だけ渡した
	if !strings.Contains(first, "問い 1") {
		t.Fatalf("巡 1 のあとのターンがゲートの問い 1 を渡さない: %q", first)
	}
	if !strings.Contains(second, "問い 2") {
		t.Errorf("巡 2 のあとのターンが担当の問い 2 を渡さない: %q", second)
	}
	if strings.Contains(second, "問い 1") {
		t.Errorf("ゲートの問い 1 が2度届いた: %q", second)
	}
	// 届けた記録（delivered.jsonl）でも asked は2ターンに1件ずつ
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

// merge が保存の前に落ちたとき: 積んだ asked も印も読んだ位置も控えに無いので、次の巡が問いの行から1回だけ積み直す。
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
			writeFile(t, filepath.Join(qdir, "1.json"), `{"kind":"line","question":"ゲートがオーナーに回した","options":[{"label":"マージする","pick":true},{"label":"直させる"}],"from":"gate"}`)
			gateAsks(p, slug)
		}
		if crash {
			return // 巡の終わりの保存の前に落ちた
		}
		if err := p.st.Save(slug); err != nil {
			t.Fatal(err)
		}
	}
	pass(false, false) // 位置を控える
	pass(true, true)   // ゲートの問いを出して積んだが、保存の前に落ちた
	pass(false, false) // 起き直した巡: 問いの行から積み直す
	if news := stopTwice(t, slug, "p1"); !strings.Contains(news, "問い 1") {
		t.Fatalf("保存の前に落ちたゲートの問いが届かない: %q", news)
	}
	pass(false, false) // もう積まない
	if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p2"}); err != nil || out != nil {
		t.Fatalf("ゲートの問いをもう一度渡した: %v %v", out, err)
	}
}

// gateAsks は merge の末尾と同じく、andon issue merge が返したゲートの問い 1 の asked を積んで印を付ける。
func gateAsks(p *project, slug string) {
	p.put(Notification{At: now(), Project: slug, Issue: "S-0007", Kind: "asked", Detail: map[string]any{"n": float64(1), "kind": "line", "question": "ゲートがオーナーに回した"}}, p.facts("S-0007"))
	p.markAsked("S-0007", float64(1))
}

// stopTwice は Stop で待ちを渡し、続きのターンの Stop で確かめる（drain の①）。渡した <news> の文を返す。
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
