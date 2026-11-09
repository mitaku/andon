package measure

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

const repoURL = "https://github.com/example/sample-site/pull/"

// fakeLedger は偽の台帳: 課題3件（＋レビュアーの課題1件）・ゲートの記録3件・問い1件・revert の PR 1本・壊れた行1行。
//
// 期間 2026-11-01〜2026-11-04（4日）で手で数えた値:
//
//	S-0001 作成 11-01 00:00。todo 1h → doing 4h → review 5h → done 10:00。PR #1 を 06:00 に dispatcher がマージ（レベル A）。人の行 1。
//	S-0002 作成 11-02 00:00。todo 2h → doing 4h → review 2h（fix）→ doing 2h → review 14h → done 11-03 00:00。
//	       PR #2 を 12:00 に人がマージ（最後の記録は needs_human・レベル B）。人の行 2。問い 03:00〜05:00（2h）。corrections 1件。
//	S-0003 作成 11-03 00:00。todo 1h → doing のまま（11-05 00:00 まで 47h）。links に PR #1（同じ箇所の課題）。
//	PR #4 "Revert ..." が 11-04 にマージされ、PR #1 を revert。
//
//	(1) 人の介在 3 回 / 2 本 = 1.5   (2) 自動マージ 1 / 2 = 0.5   (3) 差し戻し (fix 1 + needs_human 1) / 3 = 0.67
//	(4) 依頼書からマージ 6h・12h → 中央値 9h   (5) todo (1+2+1)/3 = 1.33・doing (4+6+47)/3 = 19・review (5+16)/2 = 10.5・問い 2h
//	(6) 閉じた 2 件 / 4 日 = 0.5   (7) WIP 1   (8) revert 1・同じ箇所の課題 1・開け直した A 1（revert）・B 1（corrections）
func fakeLedger(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: S\n")
	put(t, filepath.Join(k, "issues/S-0001/issue.yaml"), "id: S-0001\ntitle: one\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\n")
	put(t, filepath.Join(k, "issues/S-0001/library/report.json"), `{"pr": "`+repoURL+`1"}`)
	put(t, filepath.Join(k, "issues/S-0001/library/gate/aaaa.json"),
		`{"at": "2026-11-01T06:00:00Z", "id": "S-0001", "pr": "`+repoURL+`1", "tier": "A", "decision": "approve", "merged": true}`)
	put(t, filepath.Join(k, "issues/S-0002/issue.yaml"), "id: S-0002\ntitle: two\nstatus: done\ncreated: \"2026-11-02T00:00:00Z\"\n")
	put(t, filepath.Join(k, "issues/S-0002/library/report.json"), `{"pr": "`+repoURL+`2"}`)
	put(t, filepath.Join(k, "issues/S-0002/library/gate/bbbb.json"),
		`{"at": "2026-11-02T07:00:00Z", "id": "S-0002", "pr": "`+repoURL+`2", "tier": "B", "decision": "fix", "merged": false}`)
	put(t, filepath.Join(k, "issues/S-0002/library/gate/cccc.json"),
		`{"at": "2026-11-02T11:00:00Z", "id": "S-0002", "pr": "`+repoURL+`2", "tier": "B", "decision": "needs_human", "merged": false}`)
	put(t, filepath.Join(k, "issues/S-0002/library/corrections.json"), `{"at": "2026-11-04T00:00:00Z", "corrections": [{"kind": "answer"}]}`)
	put(t, filepath.Join(k, "issues/S-0002/questions.yaml"),
		"- n: 1\n  text: x\n  asked: \"2026-11-02T03:00:00Z\"\n  status: received\n  answered: \"2026-11-02T05:00:00Z\"\n")
	put(t, filepath.Join(k, "issues/S-0002-r1/issue.yaml"),
		"id: S-0002-r1\ntitle: review\nstatus: done\nparent: S-0002\nprofile: codex-review\ncreated: \"2026-11-02T06:00:00Z\"\n")
	put(t, filepath.Join(k, "issues/S-0003/issue.yaml"),
		"id: S-0003\ntitle: three\nstatus: doing\ncreated: \"2026-11-03T00:00:00Z\"\nlinks:\n  - "+repoURL+"1\n")
	log := strings.Join([]string{
		`{"at":"2026-11-01T00:00:00Z","by":"terminal:t1","id":"S-0001","change":{"add":"one"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"doing","agent":"t2"}}`,
		`{"at":"2026-11-01T02:00:00Z","by":"someone","id":"S-0001","change":{"merge":"auto"}}`,
		`{"at":"2026-11-01T05:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"review"}}`,
		`{"at":"2026-11-01T10:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"done"}}`,
		`{"at":"2026-11-02T00:00:00Z","by":"terminal:t1","id":"S-0002","change":{"add":"two"}}`,
		`{"at":"2026-11-02T02:00:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"doing","agent":"t3"}}`,
		`not json`,
		`{"at":"2026-11-02T03:00:00Z","by":"terminal:t3","id":"S-0002","change":{"question":1,"status":"open"}}`,
		`{"at":"2026-11-02T05:00:00Z","by":"someone","id":"S-0002","change":{"question":1,"status":"answered"}}`,
		`{"at":"2026-11-02T06:00:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"review"}}`,
		`{"at":"2026-11-02T06:00:00Z","by":"terminal:dispatcher","id":"S-0002-r1","change":{"add":"review"}}`,
		`{"at":"2026-11-02T06:30:00Z","by":"terminal:dispatcher","id":"S-0002-r1","change":{"status":"done"}}`,
		`{"at":"2026-11-02T08:00:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"doing"}}`,
		`{"at":"2026-11-02T10:00:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"review"}}`,
		`{"at":"2026-11-02T12:00:00Z","by":"someone","id":"S-0002","change":{"merge":"person"}}`,
		`{"at":"2026-11-03T00:00:00Z","by":"terminal:t1","id":"S-0002","change":{"status":"done"}}`,
		`{"at":"2026-11-03T00:00:00Z","by":"terminal:t1","id":"S-0003","change":{"add":"three"}}`,
		`{"at":"2026-11-03T01:00:00Z","by":"terminal:t4","id":"S-0003","change":{"status":"doing","agent":"t4"}}`,
		`{"at":"2026-11-03T02:00:00Z","by":"routine:triage","id":"S-0003","change":{"report":"write"}}`,
		``,
	}, "\n")
	put(t, filepath.Join(k, "log.jsonl"), log)
	return root
}

func fakeGH(repo string) ([]byte, error) {
	if repo != "example/sample-site" {
		return nil, errors.New("unexpected repo " + repo)
	}
	return []byte(`[
 {"number": 1, "url": "` + repoURL + `1", "title": "one", "body": "", "mergedAt": "2026-11-01T06:00:00Z"},
 {"number": 2, "url": "` + repoURL + `2", "title": "two", "body": "", "mergedAt": "2026-11-02T12:00:00Z"},
 {"number": 4, "url": "` + repoURL + `4", "title": "Revert \"one\"", "body": "Reverts example/sample-site#1", "mergedAt": "2026-11-04T00:00:00Z"},
 {"number": 9, "url": "` + repoURL + `9", "title": "Revert \"other\"", "body": "Reverts example/sample-site#10", "mergedAt": "2026-11-04T00:00:00Z"}
]`), nil
}

func opts(root string) Options {
	return Options{Root: root, Since: day("2026-11-01"), Until: day("2026-11-04"), Now: day("2026-11-09"), GH: fakeGH}
}

func val(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestMeasureMatchesHandCount(t *testing.T) {
	res, err := Measure(opts(fakeLedger(t)))
	if err != nil {
		t.Fatal(err)
	}
	f, e, c := res.Flow, res.Efficiency, res.Counter
	checks := []struct {
		name      string
		got, want any
	}{
		{"days", res.Days, 4},
		{"(1) merged PRs", e.MergedPRs, 2},
		{"(1) human touches", e.HumanTouches.Total, 3},
		{"(1) per PR", val(e.HumanTouches.PerPR), 1.5},
		{"(2) auto merged", e.AutoMerge.Count, 1},
		{"(2) auto merge rate", val(e.AutoMerge.Rate), 0.5},
		{"(3) gate records", e.Gate.Records, 3},
		{"(3) fix", e.Gate.Fix, 1},
		{"(3) needs_human", e.Gate.NeedsHuman, 1},
		{"(3) rate", val(e.Gate.Rate), 0.67},
		{"(4) lead time count", f.LeadTime.Count, 2},
		{"(4) lead time median", val(f.LeadTime.Value), 9.0},
		{"(5) todo", val(f.Wait["todo"].Value), 1.33},
		{"(5) todo issues", f.Wait["todo"].Count, 3},
		{"(5) doing", val(f.Wait["doing"].Value), 19.0},
		{"(5) review", val(f.Wait["review"].Value), 10.5},
		{"(5) review issues", f.Wait["review"].Count, 2},
		{"(5) queued", val(f.Wait["queued"].Value), nil},
		{"(5) question open", val(f.QuestionOpen.Value), 2.0},
		{"(5) questions", f.QuestionOpen.Count, 1},
		{"(6) closed", f.Closed.Count, 2},
		{"(6) per day", f.Closed.PerDay, 0.5},
		{"(7) WIP", f.WIPEnd, 1},
		{"(8) reverted", c.Reverted, 1},
		{"(8) follow-ups", c.FollowUps, 1},
		{"(8) corrections", c.Reopened.Corrections, "recorded"},
		{"(8) reopened A", c.Reopened.ByTier["A"], 1},
		{"(8) reopened B", c.Reopened.ByTier["B"], 1},
		{"(8) reopened C", c.Reopened.ByTier["C"], 0},
		{"(8) corrected A（revert は数えない）", c.Corrected.ByTier["A"], 0},
		{"(8) corrected B", c.Corrected.ByTier["B"], 1},
		{"(8) corrected corrections", c.Corrected.Corrections, "recorded"},
		{"skipped", len(res.Skipped), 1},
	}
	for _, x := range checks {
		if x.got != x.want {
			t.Errorf("%s: got %v, want %v", x.name, x.got, x.want)
		}
	}
	if s := res.Skipped[0]; s.Source != "kanban/log.jsonl" || s.Line != 8 || s.Reason == "" {
		t.Errorf("skipped: %+v", s)
	}
}

func TestTableOrderAndCounterNextToAutoMerge(t *testing.T) {
	res, err := Measure(opts(fakeLedger(t)))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	out := b.String()
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[0], "価値: 未計測") {
		t.Errorf("先頭が「価値: 未計測」でない: %q", lines[0])
	}
	if i, j := strings.Index(out, "\n流れ\n"), strings.Index(out, "\n効率\n"); i < 0 || j < 0 || i > j {
		t.Errorf("流れ → 効率 の順でない:\n%s", out)
	}
	found := false
	for _, ln := range lines {
		if strings.Contains(ln, "自動マージ率") {
			found = true
			for _, want := range []string{"50%（1 / 2）", "対抗:", "revert 1本", "同じ箇所の課題 1件", "人が開け直した A 1・B 1・C 0"} {
				if !strings.Contains(ln, want) {
					t.Errorf("自動マージ率の行に %q が無い: %q", want, ln)
				}
			}
		}
	}
	if !found {
		t.Errorf("自動マージ率の行が無い:\n%s", out)
	}
	if !strings.Contains(out, "飛ばした記録: 1件") || !strings.Contains(out, "kanban/log.jsonl:8") {
		t.Errorf("飛ばした記録が出ない:\n%s", out)
	}
}

func TestEmptyProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "empty")
	put(t, filepath.Join(root, "kanban/config.yaml"), "version: 3\n")
	called := false
	o := opts(root)
	o.GH = func(string) ([]byte, error) { called = true; return nil, errors.New("x") }
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("PR の無い台帳で gh を打った")
	}
	e, f := res.Efficiency, res.Flow
	if e.MergedPRs != 0 || e.AutoMerge.Rate != nil || e.Gate.Rate != nil || e.HumanTouches.PerPR != nil ||
		f.LeadTime.Value != nil || f.Closed.Count != 0 || f.WIPEnd != 0 || res.Counter.Reverted != 0 ||
		res.Counter.Reopened.Corrections != "none" || len(res.Skipped) != 0 {
		t.Errorf("空の台帳が 0・null・記録なしでない: %+v", res)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	if n := strings.Count(b.String(), "記録なし"); n < 5 {
		t.Errorf("記録なしが足りない（%d）:\n%s", n, b.String())
	}
	js, _ := json.Marshal(res)
	if !strings.Contains(string(js), `"prs":[]`) || !strings.Contains(string(js), `"skipped":[]`) {
		t.Errorf("空の配列が null になる: %s", js)
	}
}

func TestMainEmptyProjectExitsZero(t *testing.T) {
	root := filepath.Join(t.TempDir(), "empty")
	put(t, filepath.Join(root, "kanban/config.yaml"), "version: 3\n")
	t.Setenv("KANBAN_ROOT", root)
	for _, args := range [][]string{nil, {"--json"}} {
		var out, errb bytes.Buffer
		if rc := Main(args, nil, &out, &errb); rc != 0 || errb.Len() != 0 {
			t.Errorf("%v: rc=%d err=%q", args, rc, errb.String())
		}
	}
}

func TestGitHubUnreadableIsSkippedNotZero(t *testing.T) {
	o := opts(fakeLedger(t))
	o.GH = func(string) ([]byte, error) { return nil, errors.New("not logged in") }
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range res.Skipped {
		if s.Source == "github.com/example/sample-site" && strings.Contains(s.Reason, "not logged in") {
			found = true
		}
	}
	if !found {
		t.Errorf("GitHub を読めないことが飛ばした記録に無い: %+v", res.Skipped)
	}
}

func TestUnknownLedgerVersionStops(t *testing.T) {
	root := filepath.Join(t.TempDir(), "old")
	put(t, filepath.Join(root, "kanban/config.yaml"), "version: 2\n")
	if _, err := Measure(opts(root)); err == nil {
		t.Error("版 2 の台帳を読んだ")
	}
}

func TestMeasureWritesNothing(t *testing.T) {
	root := fakeLedger(t)
	before := snapshot(t, root)
	if _, err := Measure(opts(root)); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, root); after != before {
		t.Errorf("台帳が変わった:\n%s\n---\n%s", before, after)
	}
}

func snapshot(t *testing.T, root string) string {
	var b strings.Builder
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		body := ""
		if !info.IsDir() {
			x, _ := os.ReadFile(p)
			body = string(x)
		}
		b.WriteString(p + "\x00" + body + "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestLineWithoutByIsSkippedNotHuman(t *testing.T) {
	root := fakeLedger(t)
	f, err := os.OpenFile(filepath.Join(root, "kanban/log.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 有効な JSON だが by が無い（log.jsonl は20行目まで。この行は21行目）
	if _, err := f.WriteString(`{"at":"2026-11-01T03:00:00Z","id":"S-0001","change":{"report":"write"}}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	res, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Efficiency.HumanTouches.Total; got != 3 {
		t.Errorf("by の無い行を人の介在に数えた: %d", got)
	}
	found := false
	for _, s := range res.Skipped {
		if s.Source == "kanban/log.jsonl" && s.Line == 21 && strings.Contains(s.Reason, "by") {
			found = true
		}
	}
	if !found {
		t.Errorf("by の無い行が飛ばした記録に無い: %+v", res.Skipped)
	}
}

func TestUnreadableRecordsAreSkippedWithoutAbsolutePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root は読み取り権限を無視する")
	}
	root := fakeLedger(t)
	k := filepath.Join(root, "kanban")
	files := []string{
		"log.jsonl",
		"issues/S-0002/questions.yaml",
		"issues/S-0002/library/report.json",
		"issues/S-0002/library/corrections.json",
		"issues/S-0002/library/gate/bbbb.json",
	}
	for _, f := range files {
		p := filepath.Join(k, f)
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(p, 0o644) })
	}
	res, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range res.Skipped {
		got[s.Source] = s.Reason
	}
	for _, f := range files {
		why, ok := got["kanban/"+f]
		if !ok {
			t.Errorf("読めない %s が飛ばした記録に無い: %+v", f, res.Skipped)
			continue
		}
		if strings.Contains(why, root) || strings.Contains(why, "/") {
			t.Errorf("%s の理由にパスが入る: %q", f, why)
		}
	}
	js, _ := json.Marshal(res)
	if strings.Contains(string(js), root) {
		t.Errorf("--json に台帳の絶対パスが出る: %s", js)
	}
}

func skippedSources(res *Result) map[string]string {
	got := map[string]string{}
	for _, s := range res.Skipped {
		got[s.Source] = s.Reason
	}
	return got
}

func TestUnreadableDirectoriesAreSkippedNotDropped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root は読み取り権限を無視する")
	}
	root := fakeLedger(t)
	k := filepath.Join(root, "kanban")
	dirs := []string{
		"issues/S-0003",              // 辿れない課題のディレクトリ（skip の出どころは中の issue.yaml）
		"issues/S-0002/library/gate", // 読めないゲートのディレクトリ
	}
	for _, d := range dirs {
		p := filepath.Join(k, d)
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(p, 0o755) })
	}
	res, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	got := skippedSources(res)
	for _, d := range dirs {
		src := "kanban/" + d
		if d == "issues/S-0003" {
			src += "/issue.yaml"
		}
		why, ok := got[src]
		if !ok {
			t.Errorf("読めない %s が飛ばした記録に無い: %+v", d, res.Skipped)
			continue
		}
		if strings.Contains(why, root) || strings.Contains(why, "/") {
			t.Errorf("%s の理由にパスが入る: %q", d, why)
		}
	}
	js, _ := json.Marshal(res)
	if strings.Contains(string(js), root) || !strings.Contains(string(js), "kanban/issues/S-0003") {
		t.Errorf("--json の飛ばした記録が合わない: %s", js)
	}
}

func TestUnreadableAnsweredIsSkippedNotUnanswered(t *testing.T) {
	root := fakeLedger(t)
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "issues/S-0002/questions.yaml"),
		"- n: 1\n  text: x\n  asked: \"2026-11-02T03:00:00Z\"\n  status: received\n  answered: \"yesterday\"\n")
	res, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	why, ok := skippedSources(res)["kanban/issues/S-0002/questions.yaml"]
	if !ok || !strings.Contains(why, "answered") {
		t.Errorf("読めない answered が飛ばした記録に無い: %+v", res.Skipped)
	}
}
