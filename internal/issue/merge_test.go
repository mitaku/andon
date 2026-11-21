package issue

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/gate"
	"andon/internal/ledger"
)

func TestAuthorized(t *testing.T) {
	auto := ledger.Settings{Review: "on", MustFix: "major", Reviewer: "codex-review", ReviewRounds: "2", Merge: "auto"}
	person := auto
	person.Merge = "person"
	minor := auto
	minor.MustFix = "minor"
	for _, c := range []struct {
		name                string
		before, now         ledger.Settings
		status              string
		trusted             bool
		gateMerge, nowMerge string
		tier                string
		want                bool
		why                 string
	}{
		{"変わっていない", auto, auto, "review", true, "B", "B", "B", true, ""},
		{"merge: person に変わった", auto, person, "review", true, "B", "B", "B", false, "予約キー"},
		{"must_fix が変わった", auto, minor, "review", true, "B", "B", "B", false, "予約キー"},
		{"Chief of Staff でない者が書いた", auto, auto, "review", false, "B", "B", "B", false, "Chief of Staff"},
		{"review でなくなった", auto, auto, "doing", true, "B", "B", "B", false, "review でなくなった"},
		{"委譲を取り消した", auto, auto, "review", true, "B", "none", "B", false, "B → none"},
		{"委譲を広げた（判定とずれる）", auto, auto, "review", true, "A", "B", "A", false, "A → B"},
		{"委譲の外", auto, auto, "review", true, "A", "A", "B", false, "merge=A < tier B"},
	} {
		got, why := Authorized(c.before, c.now, c.status, c.trusted, c.gateMerge, c.nowMerge, c.tier)
		if got != c.want || !strings.Contains(why, c.why) {
			t.Errorf("%s: Authorized = %v %q, want %v（%q を含む）", c.name, got, why, c.want, c.why)
		}
	}
}

// ADR-0041 受け入れ条件 8: 名指しの grant で gate が approve したあとに、その grant を revoke すると、マージの直前の読み直しが断る。
// 読み直しは gate と同じ gate.Delegated で課題ごとの値を引く（名指しの値は C、取り消したあとは型 andon の B）。
func TestAuthorizedNamedGrantRevoked(t *testing.T) {
	auto := ledger.Settings{Review: "on", MustFix: "major", Reviewer: "codex-review", ReviewRounds: "2", Merge: "auto"}
	const named = `{"id":"d1","scope":"merge","value":"C","by":"owner","at":"2026-11-21","evidence":"x","issues":["A-0001"]`
	read := func(grant string) delegate.Result {
		f, err := delegate.Parse([]byte(`{"version":1,"project":"andon","preset":"andon","grants":[`+grant+`]}`), "andon")
		if err != nil {
			t.Fatal(err)
		}
		return delegate.FromFile(f, "2026-11-21")
	}
	rep := &contract.Report{}
	gateMerge, grant := gate.Delegated(read(named+`}`), "A-0001", rep)
	if gateMerge != "C" || grant != "d1" {
		t.Fatalf("gate のときの値: %s %q", gateMerge, grant)
	}
	if ok, why := Authorized(auto, auto, "review", true, gateMerge, gateMerge, "C"); !ok {
		t.Fatalf("取り消す前に断った: %s", why)
	}
	nowMerge, _ := gate.Delegated(read(named+`,"revoked":{"at":"2026-11-21","by":"owner","reason":"やめる"}}`), "A-0001", rep)
	if ok, why := Authorized(auto, auto, "review", true, gateMerge, nowMerge, "C"); ok || !strings.Contains(why, "C → B") {
		t.Errorf("取り消したあとの読み直し = %v %q, want 断る（C → B）", ok, why)
	}
	yes := true
	oneWay := &contract.Report{Risk: &contract.Risk{OneWay: &yes}}
	if m, _ := gate.Delegated(read(named+`}`), "A-0001", oneWay); m != "B" {
		t.Errorf("one_way の申告のある課題で名指しを読んだ: %s", m)
	}
}

func TestChecksStill(t *testing.T) {
	h1, h2 := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ok := gate.GitHubChecks{State: "success", Names: []string{}}
	for _, c := range []struct {
		name     string
		now      string
		checks   gate.GitHubChecks
		optional bool
		want     bool
		why      string
	}{
		{"変わっていない", h1, ok, false, true, ""},
		{"gate のあとに push された", h2, ok, false, false, "head が gate のあとに動いた"},
		{"checks が走り直した", h1, gate.GitHubChecks{State: "pending", Names: []string{"test"}}, false, false, "pending: test"},
		{"checks が落ちた", h1, gate.GitHubChecks{State: "failure", Names: []string{"image"}}, false, false, "failure: image"},
		{"checks が消えた（要る）", h1, gate.GitHubChecks{State: "none"}, false, false, "none"},
		{"checks が無い（要らない repo）", h1, gate.GitHubChecks{State: "none"}, true, true, ""},
	} {
		got, why := ChecksStill(h1, c.now, c.checks, c.optional)
		if got != c.want || !strings.Contains(why, c.why) {
			t.Errorf("%s: ChecksStill = %v %q, want %v（%q を含む）", c.name, got, why, c.want, c.why)
		}
	}
}

// gh pr merge が失敗したあと、gate の判定した head のまま MERGED ならオーナーが先にマージしたとして問いにしない。
// head が違う・MERGED でない・返事が読めないときは今までどおり問いにする（dispatcher.md §7.2 #5）。
func TestMergedBefore(t *testing.T) {
	head, other, merge := "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	for _, c := range []struct {
		name, view string
		want       bool
	}{
		{"同じ head で MERGED", `{"state":"MERGED","headRefOid":"` + head + `","mergeCommit":{"oid":"` + merge + `"}}`, true},
		{"違う head で MERGED", `{"state":"MERGED","headRefOid":"` + other + `","mergeCommit":{"oid":"` + merge + `"}}`, false},
		{"OPEN のまま", `{"state":"OPEN","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"CLOSED", `{"state":"CLOSED","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"マージの commit が無い", `{"state":"MERGED","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"head が無い", `{"state":"MERGED","mergeCommit":{"oid":"` + merge + `"}}`, false},
		{"読めない", `HTTP 502`, false},
	} {
		sha, ok := MergedBefore(c.view, head)
		if ok != c.want || (ok && sha != merge) {
			t.Errorf("%s: got (%q, %v), want ok=%v", c.name, sha, ok, c.want)
		}
	}
	if _, ok := MergedBefore(`{"state":"MERGED","headRefOid":"","mergeCommit":{"oid":"`+merge+`"}}`, ""); ok {
		t.Error("gate の head が空でも MERGED とした")
	}
}

// gate が approve か needs_human を決めたあと、open の PR の head が判定した head から動いたときだけ回し直す。
// fix（担当が ready し直す）・dispatcher がマージした・オーナーがマージした・閉じた PR は回し直さない（dispatcher.md §5 の P6a）。
func TestHeadMoved(t *testing.T) {
	h1, h2 := strings.Repeat("a", 40), strings.Repeat("b", 40)
	m := func(decision, merged string) contract.Merging {
		return contract.Merging{Row: 3, Sha: h1, Decision: decision, Merged: merged}
	}
	for _, c := range []struct {
		name        string
		w           contract.Merging
		state, head string
		want        bool
	}{
		{"approve のあとに push された", m("approve", ""), "OPEN", h2, true},
		{"needs_human のあとに push された", m("needs_human", ""), "OPEN", h2, true},
		{"head が同じ", m("approve", ""), "OPEN", h1, false},
		{"fix は担当が ready し直す", m("fix", ""), "OPEN", h2, false},
		{"dispatcher がマージした", m("approve", strings.Repeat("c", 40)), "OPEN", h2, false},
		{"オーナーが別の head でマージした（settle の道）", m("needs_human", ""), "MERGED", h2, false},
		{"閉じた PR", m("approve", ""), "CLOSED", h2, false},
		{"head が読めない", m("approve", ""), "OPEN", "", false},
		{"判定した head が無い（決めていない）", contract.Merging{Row: 3, Decision: "needs_human"}, "OPEN", h2, false},
	} {
		if got := HeadMoved(c.w, c.state, c.head); got != c.want {
			t.Errorf("%s: HeadMoved = %v, want %v", c.name, got, c.want)
		}
	}
}

// gate の問いを閉じるとき、一覧を読んだあとにオーナーか Chief of Staff が答えていれば answer は断られる。オーナーの答えは上書きせず、
// そのまま received に進めて閉じる。受け取れなければ誤りを返す（merge は次に打たれたときに閉じ直す）。
func TestCloseQuestionAfterAnswer(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh が無い")
	}
	dir := t.TempDir()
	// 偽の kanban の CLI: 打たれた引数を calls に書く。answer は answered の印があれば断り、received は fail の印があれば断る
	fake := "#!/bin/sh\necho \"$*\" >> \"$FAKE/calls\"\ncase \"$1\" in\n" +
		"  answer) [ ! -f \"$FAKE/answered\" ] || { echo 'not open' >&2; exit 1; } ;;\n" +
		"  received) [ ! -f \"$FAKE/fail\" ] || { echo 'not answered' >&2; exit 1; } ;;\nesac\n"
	os.WriteFile(filepath.Join(dir, "kanban"), []byte(fake), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE", dir)
	c := &ctx{l: &ledger.Ledger{Root: dir, Slug: "x", Dir: dir}, id: "T-0001"}
	calls := func() string {
		b, _ := os.ReadFile(filepath.Join(dir, "calls"))
		os.Remove(filepath.Join(dir, "calls"))
		return string(b)
	}

	if err := c.closeQuestion("T-0001#1", "open", "head が動いた"); err != nil || calls() != "answer T-0001#1 head が動いた\nreceived T-0001#1\n" {
		t.Fatalf("open の問いを答えて受け取っていない: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "answered"), nil, 0o644)
	if err := c.closeQuestion("T-0001#1", "open", "head が動いた"); err != nil || !strings.HasSuffix(calls(), "received T-0001#1\n") {
		t.Fatalf("一覧のあとに答えられた問いを受け取っていない: %v", err)
	}
	if err := c.closeQuestion("T-0001#2", "answered", "head が動いた"); err != nil || calls() != "received T-0001#2\n" {
		t.Fatalf("答えのある問いに答えを書いた・受け取っていない: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "fail"), nil, 0o644)
	if err := c.closeQuestion("T-0001#1", "open", "head が動いた"); err == nil || !strings.Contains(err.Error(), "answer:") {
		t.Fatalf("受け取れないのに誤りを返していない: %v", err)
	}
}
