package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/dispatcher"
	"andon/internal/ledger/ledgertest"
	"andon/internal/spawn"
)

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }

const day = 24 * time.Hour

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// dummyKey は検査の中で組み立てる偽の鍵（repo に鍵の形の文字列を置かない）。
func dummyKey() string { return "ghp_" + strings.Repeat("Z9", 18) }

const (
	pr1       = "https://github.com/example/sample-site/pull/1"
	pr7       = "https://github.com/example/sample-site/pull/7"
	pr8       = "https://github.com/example/sample-site/pull/8"
	sha1      = "1111111111111111111111111111111111111111"
	cosSess   = "c0c0c0c0-0000-4000-8000-000000000001"
	workSess  = "a1a1a1a1-0000-4000-8000-000000000001"
	otherSess = "ffffffff-0000-4000-8000-000000000001"
)

type fixture struct {
	root, claude, pkg string
	gh                func(args ...string) ([]byte, error)
	calls             []string
}

func issueYAML(id, status string, extra ...string) string {
	return fmt.Sprintf("id: %s\ntitle: %s の題\nstatus: %s\n%s", id, id, status, strings.Join(extra, ""))
}

func launchLine(at, session, commit string) string {
	return fmt.Sprintf(`{"at":%q,"role":"worker","launch":"start","issue":"x","session":%q,"package":{"commit":%q},"memory_index":"- 大きな目録\n"}`+"\n", at, session, commit)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	tmp := t.TempDir()
	f := &fixture{root: filepath.Join(tmp, "sample-site"), claude: filepath.Join(tmp, "claude"), pkg: filepath.Join(tmp, "pkg")}
	k := filepath.Join(f.root, "kanban")
	is := func(id string) string { return filepath.Join(k, "issues", id) }
	write(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: A\n")
	wt := filepath.Join(tmp, "wt", "A-0001")
	// A-0001: 期間の内に片付いた担当の課題。PR はマージされ、revert され、マージ commit の checks が落ちた
	write(t, filepath.Join(is("A-0001"), "issue.yaml"), issueYAML("A-0001", "done", "profile: claude\nrepo: github.com/example/sample-site\nestimate: medium\nworktree: "+wt+"\n"))
	write(t, filepath.Join(is("A-0001"), "library", "launches.jsonl"), launchLine(ago(3*day), workSess, sha1))
	write(t, filepath.Join(is("A-0001"), "library", "report.json"), `{"pr":"`+pr1+`","estimate":{"size":"large","reason":"x"},"waived":[{"severity":"minor","file":"a.go","line":1,"reason":"軽い"}],"remember":["学び"],"agents_md":["作法"]}`)
	write(t, filepath.Join(is("A-0001"), "library", "outcome.json"), `{"result":"merged","pr":"`+pr1+`","merge_commit":"`+sha1+`","merged_at":"`+ago(2*day)+`","worked_minutes":95}`)
	write(t, filepath.Join(is("A-0001"), "library", "gate", sha1+".json"), `{"head":"`+sha1+`","at":"`+ago(2*day)+`","tier":"B","decision":"approve","reasons":[],"merged":true}`)
	write(t, filepath.Join(is("A-0001"), "library", "corrections.json"), `{"at":"x","corrections":[{"kind":"pr","comments":[{"author":"o","at":"x","body":"token = `+dummyKey()+`"}],"human_commits":[]}]}`)
	write(t, filepath.Join(is("A-0001"), "library", "journal.md"), "迷った\n")
	// A-0001-r1: レビュアーの子（親に束ねる）
	write(t, filepath.Join(is("A-0001-r1"), "issue.yaml"), issueYAML("A-0001-r1", "done", "profile: codex-review\nparent: A-0001\n"))
	write(t, filepath.Join(is("A-0001-r1"), "library", "findings.json"), `{"commit":"`+sha1+`","findings":[{"severity":"major","file":"a.go","line":3}]}`)
	// A-0002: 期間の外に片付いた
	write(t, filepath.Join(is("A-0002"), "issue.yaml"), issueYAML("A-0002", "done", "profile: claude\n"))
	// A-0003: 起動の記録のパッケージの commit が2通り（途中で当て直した）
	write(t, filepath.Join(is("A-0003"), "issue.yaml"), issueYAML("A-0003", "done", "profile: claude\n"))
	write(t, filepath.Join(is("A-0003"), "library", "launches.jsonl"), launchLine(ago(4*day), "", sha1)+launchLine(ago(3*day), "", strings.Repeat("2", 40)))
	// A-0004: 当て直しの記録の時刻が起動と片付けの間
	write(t, filepath.Join(is("A-0004"), "issue.yaml"), issueYAML("A-0004", "done", "profile: claude\n"))
	write(t, filepath.Join(is("A-0004"), "library", "launches.jsonl"), launchLine(ago(2*day), "", sha1))
	// A-0005: オーナーの課題（束ねない）・A-0009: Chief Engineer の課題（束ねない）・A-0010: まだ片付いていない
	write(t, filepath.Join(is("A-0005"), "issue.yaml"), issueYAML("A-0005", "done", "who: person\n"))
	write(t, filepath.Join(is("A-0009"), "issue.yaml"), issueYAML("A-0009", "done", "profile: chief-engineer\n"))
	write(t, filepath.Join(is("A-0010"), "issue.yaml"), issueYAML("A-0010", "doing", "profile: claude\n"))
	// A-0006: 前の振り返り（ルーチンが足した）。A-0007・A-0008 はそれが切った直し（マージは3日前と20日前）
	write(t, filepath.Join(is("A-0006"), "issue.yaml"), issueYAML("A-0006", "done", "profile: claude\nroutine: retro\n"))
	write(t, filepath.Join(is("A-0007"), "issue.yaml"), issueYAML("A-0007", "done", "profile: claude\nparent: A-0006\n"))
	write(t, filepath.Join(is("A-0007"), "library", "report.json"), `{"pr":"`+pr7+`"}`)
	write(t, filepath.Join(is("A-0008"), "issue.yaml"), issueYAML("A-0008", "done", "profile: claude\nparent: A-0006\n"))
	write(t, filepath.Join(is("A-0008"), "library", "report.json"), `{"pr":"`+pr8+`"}`)
	var log []string
	for _, x := range []struct {
		id string
		d  time.Duration
	}{{"A-0001", 2 * day}, {"A-0001-r1", 2 * day}, {"A-0002", 9 * day}, {"A-0003", 1 * day}, {"A-0004", 1 * day},
		{"A-0005", 1 * day}, {"A-0009", 1 * day}, {"A-0006", 21 * day}, {"A-0007", 3 * day}, {"A-0008", 20 * day}} {
		log = append(log, fmt.Sprintf(`{"at":%q,"by":"terminal:t1","id":%q,"change":{"status":"done"}}`, ago(x.d), x.id))
	}
	write(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t, log...))
	write(t, filepath.Join(f.pkg, "applied.json"), `{"commit":"`+sha1+`","applied_at":"`+ago(36*time.Hour)+`","image_commit":"x"}`)

	// セッションログ: Chief of Staff（cwd がプロジェクトの根）・担当（起動の記録の session）・ほかのプロジェクト
	hum := func(uuid, at, text string) string {
		b, _ := json.Marshal(map[string]any{"type": "user", "uuid": uuid, "timestamp": at, "cwd": f.root,
			"origin": map[string]any{"kind": "human"}, "promptSource": "typed", "message": map[string]any{"role": "user", "content": text}})
		return string(b) + "\n"
	}
	cos := hum("u0", ago(3*day), "プロジェクト sample-site の Chief of Staff。") + // 起動の最初のメッセージ
		hum("u1", ago(2*day), "PR の題に課題番号を書かないで。鍵は "+dummyKey()) +
		hum("u2", ago(2*day), `<hc from="t9 x">担当から届いた文</hc>`) +
		hum("u3", ago(2*day), "<pasted_content id=\"1\">\n<hc from=\"t9 x\">貼った中の hc</hc>") +
		hum("u4", ago(2*day), "<news project=\"sample-site\">\nA-0001 review\n</news>") +
		hum("u5", ago(10*day), "期間の外の文") +
		`{"type":"user","uuid":"u6","timestamp":"` + ago(2*day) + `","origin":{"kind":"task-notification"},"message":{"content":"<task-notification>x</task-notification>"}}` + "\n" +
		`{"type":"user","uuid":"u7","timestamp":"` + ago(2*day) + `","origin":{"kind":"human"},"message":{"content":[{"type":"tool_result","content":"道具の結果"}]}}` + "\n" +
		`{"type":"user","uuid":"u8","timestamp":"` + ago(2*day) + `","isMeta":true,"origin":{"kind":"human"},"message":{"content":"meta"}}` + "\n" +
		`{"type":"user","uuid":"u9","timestamp":"` + ago(2*day) + `","isSidechain":true,"origin":{"kind":"human"},"message":{"content":"sidechain"}}` + "\n" +
		hum("u11", ago(2*day), "[dispatcher] 一時のエラーで止まった。やり直しに承認は要らない。止まる前の作業を続けて（dispatcher から）") +
		hum("u12", ago(2*day), "[gate] A-0001（題）: ゲートが fix を返した。理由: checks が落ちた。記録は /x。") +
		`{"type":"user","uuid":"u10","timestamp":"` + ago(1*day) + `","origin":{"kind":"human"},"message":{"content":[{"type":"text","text":"block の文"}]}}` + "\n"
	write(t, filepath.Join(f.claude, "projects", "-sample-site", cosSess+".jsonl"), cos)
	work := strings.ReplaceAll(hum("w0", ago(3*day), "<hc from=\"t1 cos\">課題 A-0001。依頼書は …</hc>")+hum("w1", ago(2*day), "そこは直さなくていい"), f.root, wt)
	write(t, filepath.Join(f.claude, "projects", "-wt-A-0001", workSess+".jsonl"), work)
	other := strings.ReplaceAll(hum("o0", ago(2*day), "最初")+hum("o1", ago(2*day), "ほかのプロジェクトの文"), f.root, filepath.Join(tmp, "other"))
	write(t, filepath.Join(f.claude, "projects", "-other", otherSess+".jsonl"), other)

	f.gh = func(args ...string) ([]byte, error) {
		f.calls = append(f.calls, strings.Join(args, " "))
		switch {
		case args[0] == "pr" && args[1] == "list":
			return []byte(`[{"number":1,"url":"` + pr1 + `","title":"one","body":"","mergedAt":"` + ago(2*day) + `","mergeCommit":{"oid":"` + sha1 + `"}},
 {"number":3,"url":"https://github.com/example/sample-site/pull/3","title":"Revert \"one\"","body":"Reverts example/sample-site#1","mergedAt":"` + ago(1*day) + `","mergeCommit":{"oid":"3333333333333333333333333333333333333333"}},
 {"number":4,"url":"https://github.com/example/sample-site/pull/4","title":"Revert \"ten\"","body":"Reverts example/sample-site#10","mergedAt":"` + ago(1*day) + `","mergeCommit":null},
 {"number":7,"url":"` + pr7 + `","title":"fix","body":"","mergedAt":"` + ago(3*day) + `","mergeCommit":{"oid":"7777777777777777777777777777777777777777"}},
 {"number":8,"url":"` + pr8 + `","title":"fix","body":"","mergedAt":"` + ago(20*day) + `","mergeCommit":{"oid":"8888888888888888888888888888888888888888"}}]`), nil
		case args[0] == "api" && args[1] == "--paginate" && args[2] == "repos/example/sample-site/commits/"+sha1+"/check-runs?per_page=100":
			// gh api --paginate はページごとの object を続けて出す。失敗は2ページ目にだけある
			return []byte(`{"total_count":3,"check_runs":[{"name":"lint","status":"completed","conclusion":"success"}]}
{"total_count":3,"check_runs":[{"name":"tests","status":"completed","conclusion":"failure"},{"name":"tests","status":"completed","conclusion":"failure"}]}`), nil
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}
	return f
}

func (f *fixture) collect(t *testing.T) (*Input, []byte) {
	t.Helper()
	in, err := Collect(Options{Root: f.root, Now: now, Days: 7, GH: f.gh, ClaudeDir: f.claude, Package: f.pkg})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Redacted(in)
	if err != nil {
		t.Fatal(err)
	}
	return in, b
}

func ids(in *Input) []string {
	var out []string
	for _, b := range in.Issues {
		out = append(out, b.ID)
	}
	return out
}

func bundle(t *testing.T, in *Input, id string) Bundle {
	t.Helper()
	for _, b := range in.Issues {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("%s が束ねられていない: %v", id, ids(in))
	return Bundle{}
}

// 受け入れ条件 8: 期間に片付いた担当の課題だけ（片付いた順）。レビュアーの子は親の1件に入る。前の振り返りが切った直しも担当の課題として入る。
func TestBundlesIssuesDoneInPeriod(t *testing.T) {
	in, _ := newFixture(t).collect(t)
	if got := strings.Join(ids(in), " "); got != "A-0007 A-0001 A-0003 A-0004" {
		t.Errorf("束ねた課題 = %q（期間の内の担当の課題だけ。子・オーナー・Chief Engineer・振り返り・期間の外・片付いていないものは入らない）", got)
	}
	b := bundle(t, in, "A-0001")
	if len(b.Reviews) != 1 || b.Reviews[0].ID != "A-0001-r1" {
		t.Fatalf("子の指摘が親に入っていない: %+v", b.Reviews)
	}
	fs, _ := b.Reviews[0].Findings["findings"].([]any)
	if len(fs) != 1 {
		t.Errorf("子の指摘 = %v", b.Reviews[0].Findings)
	}
	if b.PR == nil || *b.PR != pr1 || b.WorkedMinutes == nil || *b.WorkedMinutes != 95 || b.Estimate == nil || *b.Estimate != "medium" ||
		b.AgentEstimate == nil || *b.AgentEstimate != "large" || len(b.Gates) != 1 || b.Gates[0].Decision != "approve" ||
		len(b.Waived) != 1 || len(b.Corrections) != 1 || strings.Join(b.Remember, "") != "学び" || strings.Join(b.AgentsMD, "") != "作法" ||
		b.Journal == nil || *b.Journal != "迷った\n" || b.Outcome == nil || len(b.Launches) != 1 {
		t.Errorf("束ねた記録が足りない: %+v", b)
	}
	if l := b.Launches[0]; l.At != ago(3*day) || l.Commit == nil || *l.Commit != sha1 || l.Model != nil {
		t.Errorf("起動の記録は時刻・パッケージの commit・プロファイル・モデルだけ: %+v", l)
	}
	if o := b.Outcome; o.Result != "merged" || o.WorkedMinutes == nil || *o.WorkedMinutes != 95 || o.Cost != nil {
		t.Errorf("結果の記録は結果・実働・コストの合計だけ: %+v", o)
	}
	if b3 := bundle(t, in, "A-0003"); b3.Launches == nil || b3.Outcome != nil || b3.Corrections != nil || b3.Journal != nil || b3.AfterMerge != nil {
		t.Errorf("無い記録は null: %+v", b3)
	}
	if in.PreviousRetro == nil || in.PreviousRetro.Issue != "A-0006" {
		t.Errorf("前の振り返り = %+v", in.PreviousRetro)
	}
}

// 受け入れ条件 10: マージ後のこと（マージ commit の checks の失敗・その PR を revert した PR）が元の課題に付く。
func TestAfterMerge(t *testing.T) {
	in, _ := newFixture(t).collect(t)
	a := bundle(t, in, "A-0001").AfterMerge
	if a == nil || a.MergeCommit != sha1 || a.Checks == nil || a.Checks.State != "failure" || strings.Join(a.Checks.Failed, ",") != "tests" ||
		len(a.RevertedBy) != 1 || a.RevertedBy[0].PR != "https://github.com/example/sample-site/pull/3" {
		t.Fatalf("マージ後のこと = %+v", a)
	}
	// GitHub を読まないときは null で、skipped に1行
	f := newFixture(t)
	f.gh = nil
	in, _ = f.collect(t)
	if bundle(t, in, "A-0001").AfterMerge != nil || len(in.Skipped) != 1 || in.Skipped[0].Source != "github" {
		t.Errorf("GitHub 無し: %+v %+v", bundle(t, in, "A-0001").AfterMerge, in.Skipped)
	}
}

// 受け入れ条件 10: マージ commit の checks の状態の決め方。
func TestChecksState(t *testing.T) {
	for _, c := range []struct{ runs, want string }{
		{`[]`, "none"},
		{`[{"name":"a","status":"completed","conclusion":"success"}]`, "success"},
		{`[{"name":"a","status":"in_progress","conclusion":null}]`, "pending"},
		{`[{"name":"a","status":"completed","conclusion":"cancelled"}]`, "success"},
		{`[{"name":"a","status":"completed","conclusion":"timed_out"},{"name":"b","status":"queued"}]`, "failure"},
		// 2ページ目の失敗も数える（1ページ目は成功だけ）
		{`[{"name":"a","status":"completed","conclusion":"success"}]}` + "\n" + `{"check_runs":[{"name":"b","status":"completed","conclusion":"failure"}]`, "failure"},
	} {
		g := newGitHub(&reader{}, func(...string) ([]byte, error) { return []byte(`{"check_runs":` + c.runs + `}`), nil })
		if got := g.checks("o/r", sha1); got == nil || got.State != c.want {
			t.Errorf("%s → %+v、want %s", c.runs, got, c.want)
		}
	}
}

// 受け入れ条件 9: オーナーが打った文だけが抜かれる。
func TestTypedOnlyHuman(t *testing.T) {
	in, _ := newFixture(t).collect(t)
	var got []string
	for _, x := range in.Typed {
		issue := "-"
		if x.Issue != nil {
			issue = *x.Issue
		}
		got = append(got, x.ID+"/"+x.Role+"/"+issue)
	}
	want := "h-u1/chief-of-staff/- h-w1/worker/A-0001 h-u10/chief-of-staff/-"
	if strings.Join(got, " ") != want {
		t.Errorf("抜いた文 = %v、want %s（起動の最初のメッセージ・hc send・貼った中の hc・通知・端末の外から送った再開とゲートの差し戻し・道具の結果・meta・sidechain・期間の外・ほかのプロジェクトは入らない）", got, want)
	}
	for _, x := range in.Typed {
		if x.Session == "" || x.At == "" {
			t.Errorf("session か at が無い: %+v", x)
		}
	}
}

// 受け入れ条件 7: 秘密が出力に残らない（同じ検索が入力の側では当たることも確かめる）。
func TestRedactsSecrets(t *testing.T) {
	f := newFixture(t)
	in, out := f.collect(t)
	key := dummyKey()
	if !strings.Contains(in.Typed[0].Text, key) {
		t.Fatal("検査の前提: 伏せる前のオーナーの文に鍵がある")
	}
	if strings.Contains(string(out), key) {
		t.Error("出力に鍵が残っている")
	}
	if !strings.Contains(string(out), "[REDACTED:github]") || in.Redacted != 2 {
		t.Errorf("伏せた数 = %d（オーナーの文と手直しの本文の2つ）", in.Redacted)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil || v["redacted"] != float64(2) {
		t.Errorf("出力が JSON でない・redacted が違う: %v %v", err, v["redacted"])
	}
}

// 伏せる形の一覧（どれも検査の中で組み立てた偽の値）。秘密でない文は伏せない。
func TestRedactPatterns(t *testing.T) {
	rep := strings.Repeat
	for _, c := range []struct{ in, want string }{
		{"key " + "sk-ant-" + rep("a1", 15), "key [REDACTED:anthropic]"},
		{"sk-proj-" + rep("b2", 15), "[REDACTED:openai]"},
		{"github_pat_" + rep("C3", 20), "[REDACTED:github]"},
		{"AKIA" + rep("Q", 16), "[REDACTED:aws]"},
		{"xoxb-" + rep("1", 12), "[REDACTED:slack]"},
		{"AIza" + rep("d", 35), "[REDACTED:google]"},
		{"eyJ" + rep("e", 12) + "." + rep("f", 12) + "." + rep("g", 12), "[REDACTED:jwt]"},
		{"Authorization: Bearer " + rep("h", 24), "Authorization: Bearer [REDACTED:bearer]"},
		{"api_key=" + rep("i", 10), "api_key=[REDACTED:assignment]"},
		// 短い値も伏せる（引用あり・なし）
		{"password=" + rep("l", 7), "password=[REDACTED:assignment]"},
		{`pwd: "` + rep("m", 3) + `"`, `pwd: "[REDACTED:assignment]"`},
		{"token=x", "token=[REDACTED:assignment]"},
		{`GITHUB_TOKEN: ` + rep("j", 10), "GITHUB_TOKEN: [REDACTED:assignment]"},
		// 鍵の塊の見出しは組み立てる（repo に鍵の形の文字列を置かない）
		{"-----BEGIN OPENSSH " + "PRIVATE KEY-----\n" + rep("k", 40) + "\n-----END OPENSSH " + "PRIVATE KEY-----", "[REDACTED:private-key]"},
		// 伏せないもの
		{"task-assessment-for-something-long", "task-assessment-for-something-long"},
		{"tokens: 1200 と token の数", "tokens: 1200 と token の数"},
		{"commit " + sha1, "commit " + sha1},
	} {
		b, _ := json.Marshal(map[string]string{"s": c.in})
		got, _ := redact(b)
		var v map[string]string
		if err := json.Unmarshal(got, &v); err != nil {
			t.Errorf("%q: 伏せたあとが JSON でない: %s", c.in, got)
			continue
		}
		if v["s"] != c.want {
			t.Errorf("%q → %q、want %q", c.in, v["s"], c.want)
		}
	}
	// JSON の中の "password": "…" の形（文字列の中のエスケープした引用符）
	b, _ := json.Marshal(map[string]string{"s": `{"password": "` + rep("p", 12) + `"}`})
	got, _ := redact(b)
	if strings.Contains(string(got), rep("p", 12)) || !json.Valid(got) {
		t.Errorf("引用符の中の値: %s", got)
	}
}

// 受け入れ条件 18: 前の直しが、マージの日とそのあとの課題の数つきで出る。14 日を過ぎたものは出ない。
func TestPreviousFixes(t *testing.T) {
	in, _ := newFixture(t).collect(t)
	if len(in.Fixes) != 1 {
		t.Fatalf("前の直し = %+v（3日前のものだけ）", in.Fixes)
	}
	x := in.Fixes[0]
	// マージ（3日前）のあとに片付いた担当の課題: A-0001（2日前）・A-0003・A-0004（1日前）
	if x.Issue != "A-0007" || x.Retro != "A-0006" || x.PR != pr7 || x.MergedAt != ago(3*day) || x.IssuesAfter != 3 {
		t.Errorf("前の直し = %+v", x)
	}
}

// 受け入れ条件 19: 途中でパッケージが当て直された課題に印が付く。
func TestReapplied(t *testing.T) {
	in, _ := newFixture(t).collect(t)
	for id, want := range map[string]bool{"A-0001": false, "A-0003": true, "A-0004": true} {
		b := bundle(t, in, id)
		if b.Reapplied == nil || *b.Reapplied != want {
			t.Errorf("%s の印 = %v（%v）、want %v", id, b.Reapplied, b.ReappliedReasons, want)
		}
		if want != (len(b.ReappliedReasons) > 0) {
			t.Errorf("%s の理由 = %v", id, b.ReappliedReasons)
		}
	}
	if r := bundle(t, in, "A-0003").ReappliedReasons; !strings.Contains(strings.Join(r, ""), "2 通り") {
		t.Errorf("A-0003 の理由 = %v", r)
	}
	if r := bundle(t, in, "A-0004").ReappliedReasons; !strings.Contains(strings.Join(r, ""), "起動と片付けの間") {
		t.Errorf("A-0004 の理由 = %v", r)
	}
	// 起動の記録が無い課題は分からない（null）
	if got, reasons := reapplied(nil, ago(day), nil, nil); got != nil || len(reasons) != 1 {
		t.Errorf("起動の記録が無い: %v %v", got, reasons)
	}
}

// 読むだけ: 台帳とセッションログのファイルを書き換えない。
func TestReadOnly(t *testing.T) {
	f := newFixture(t)
	snap := func() string {
		var b strings.Builder
		for _, d := range []string{f.root, f.claude, f.pkg} {
			_ = filepath.Walk(d, func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					c, _ := os.ReadFile(p)
					fmt.Fprintf(&b, "%s %d %s\n", p, info.ModTime().UnixNano(), c)
				}
				return nil
			})
		}
		return b.String()
	}
	before := snap()
	f.collect(t)
	if snap() != before {
		t.Error("台帳かセッションログが書き換わった")
	}
	for _, c := range f.calls {
		if !strings.HasPrefix(c, "pr list ") && !strings.HasPrefix(c, "api --paginate repos/") {
			t.Errorf("gh で読む以外を打った: %s", c)
		}
	}
}

// 端末の外から送った文は、送り手の印（spawn.Mark）で照らす。印の無い文と、文の中ほどの角括弧はオーナーの文として残す。
func TestMachineText(t *testing.T) {
	for s, want := range map[string]bool{
		spawn.Mark(spawn.FromDispatcher, dispatcher.RetryText):                     true,
		spawn.Mark(spawn.FromGate, "A-0017（PR #262 の衝突を解く）: ゲートが fix を返した。理由: x。"): true,
		"<pasted_content id=\"1\">\n[gate] A-0017（題）: ゲートが fix を返した。":              true,
		"  [dispatcher] 課題 `sample-site/S-7`。":                                     true,
		"[dispatcher]の文はどこから？":                                                     false,
		"[cos] オーナーが書いた角括弧":                                                        false,
		"ゲートの [gate] 印は何？":                                                         false,
		"A-0017 のゲートが fix を返したのはなぜ？":                                               false,
	} {
		if got := machineText(s); got != want {
			t.Errorf("%q → %v", s, got)
		}
	}
}

func TestMainUsage(t *testing.T) {
	var out, errs strings.Builder
	if rc := Main(nil, nil, &out, &errs); rc != 2 || !strings.HasPrefix(errs.String(), "usage: andon retro collect") {
		t.Errorf("rc=%d %q", rc, errs.String())
	}
	errs.Reset()
	if rc := Main([]string{"collect", "--days", "0"}, nil, &out, &errs); rc != 2 {
		t.Errorf("--days 0: rc=%d", rc)
	}
}
