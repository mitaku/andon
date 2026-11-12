package measure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	if !strings.HasPrefix(lines[0], "価値: 仮説を確かめた数 記録なし") {
		t.Errorf("先頭が価値の行でない: %q", lines[0])
	}
	if strings.Contains(out, "未計測") {
		t.Errorf("「未計測」が残っている:\n%s", out)
	}
	if i, j, k := strings.Index(out, "\n価値\n"), strings.Index(out, "\n流れ\n"), strings.Index(out, "\n効率\n"); i < 0 || j < 0 || k < 0 || i > j || j > k {
		t.Errorf("価値 → 流れ → 効率 の順でない:\n%s", out)
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
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // dispatcher の置き場をセルの本物から外す
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

// valueLedger は価値の行の偽の台帳（ADR-0011 決めたこと 18）。期間 2026-11-01〜2026-11-10 で手で数えた値:
//
//	E-0001 Chief Engineer の課題。11-02 00:00 に done。観察 E-0002（parent）が 11-04 12:00 に review・hit → 2.5 日。
//	E-0003 Chief Engineer の課題。11-03 00:00 に done。観察 E-0004 が 11-08 00:00 に review・unknown → 5 日。
//	E-0005 Chief Engineer の課題。11-05 00:00 に done。観察の課題が無い（価値の仮説「無し」）。
//	E-0006 Chief Engineer の課題。11-12 に done（期間の外）。観察の課題が無い → 数えない。
//	E-0007 観察の課題だが parent が Chief Engineer でない（E-0008）。11-06 に review・miss → 数えるが日数には入れない。
//
//	(a) 確かめた 3 件（hit 1・miss 1・unknown 1）  (b) 日数 (2.5, 5) の中央値 3.75、2 件  (c) 仮説「無し」 1 件
func valueLedger(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: E\n")
	issue := func(id, profile, parent, obs string) {
		y := "id: " + id + "\ntitle: t\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\nprofile: " + profile + "\n"
		if parent != "" {
			y += "parent: " + parent + "\n"
		}
		put(t, filepath.Join(k, "issues", id, "issue.yaml"), y)
		if obs != "" {
			put(t, filepath.Join(k, "issues", id, "library/report.json"),
				`{"observation": {"hypothesis": "h", "result": "`+obs+`", "evidence": []}}`)
		}
	}
	issue("E-0001", "chief-engineer", "", "")
	issue("E-0002", "claude", "E-0001", "hit")
	issue("E-0003", "chief-engineer", "", "")
	issue("E-0004", "claude", "E-0003", "unknown")
	issue("E-0005", "chief-engineer", "", "")
	issue("E-0006", "chief-engineer", "", "")
	issue("E-0007", "claude", "E-0008", "miss")
	issue("E-0008", "claude", "", "")
	// 通常の epic の子課題（観察の課題ではない）は、E-0005 を「無し」から外さない
	issue("E-0009", "claude", "E-0005", "")
	put(t, filepath.Join(k, "issues/E-0009/issue.yaml"), "id: E-0009\ntitle: t\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\nprofile: claude\nparent: E-0005\nnot_before: \"2026-11-12\"\n")
	// 不正な result は黙って落とさず skipped に出す
	issue("E-0010", "claude", "E-0001", "")
	put(t, filepath.Join(k, "issues/E-0010/library/report.json"), `{"observation": {"result": "later"}}`)
	st := func(at, id, status string) string {
		return `{"at":"` + at + `","by":"terminal:t1","id":"` + id + `","change":{"status":"` + status + `"}}`
	}
	log := strings.Join([]string{
		st("2026-11-02T00:00:00Z", "E-0001", "done"), st("2026-11-04T12:00:00Z", "E-0002", "review"),
		st("2026-11-03T00:00:00Z", "E-0003", "done"), st("2026-11-08T00:00:00Z", "E-0004", "review"),
		st("2026-11-05T00:00:00Z", "E-0005", "done"), st("2026-11-12T00:00:00Z", "E-0006", "done"),
		st("2026-11-06T00:00:00Z", "E-0007", "review"), ``,
	}, "\n")
	put(t, filepath.Join(k, "log.jsonl"), log)
	return root
}

func TestValueMatchesHandCount(t *testing.T) {
	o := opts(valueLedger(t))
	o.Until, o.Now = day("2026-11-10"), day("2026-11-20")
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	v := res.Value
	checks := []struct {
		name      string
		got, want any
	}{
		{"(a) observed", v.Observed.Count, 3},
		{"(a) hit", v.Observed.Hit, 1},
		{"(a) miss", v.Observed.Miss, 1},
		{"(a) unknown", v.Observed.Unknown, 1},
		{"(b) days count", v.DaysToObserve.Count, 2},
		{"(b) days median", val(v.DaysToObserve.Days), 3.75},
		{"(c) no hypothesis", v.NoHypothesis, 1},
		{"skipped（不正な result）", len(res.Skipped), 1},
	}
	for _, x := range checks {
		if x.got != x.want {
			t.Errorf("%s: got %v, want %v", x.name, x.got, x.want)
		}
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	for _, want := range []string{"価値: 仮説を確かめた数 3件（hit 1・miss 1・unknown 1）", "3.75日（中央値。2件）", "価値の仮説「無し」の題         1"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("表に %q が無い:\n%s", want, b.String())
		}
	}
	// 期間を狭めると、期間の外の報告・done は数えない
	o.Since, o.Until = day("2026-11-07"), day("2026-11-10")
	if res, err = Measure(o); err != nil {
		t.Fatal(err)
	}
	if v := res.Value; v.Observed.Count != 1 || v.Observed.Unknown != 1 || v.DaysToObserve.Count != 1 || val(v.DaysToObserve.Days) != 5.0 || v.NoHypothesis != 0 {
		t.Errorf("狭めた期間: %+v", v)
	}
}

// fakeNews は偽の dispatcher の記録（届けた通知の記録と記録だけの通知）を置き、Options に足す。
//
// 期間 2026-11-01〜2026-11-04 で手で数えた値:
//
//	届けたターン（期間の中）: ①report・ready（ack）②report（ack）③report・asked（ack なし）④report（ack）⑤ready（ack なし）⑥report（ack）
//	  期間の外: 10-31 の report（ack）・11-05 の asked（ack）は数えない。壊れた行 1・kind の無い通知 1 は飛ばす。
//	  report: 届けた 5・素通り 4（①②④⑥）= 0.8 → 候補（8割ちょうど）
//	  ready:  届けた 2・素通り 1 = 0.5
//	  asked:  届けた 1・素通り 0 = 0
//	記録だけ（期間の中）: merged 3・review-requested 1。期間の外の merged 1 は数えない。壊れた行 1 は飛ばす。
//	  merged: 届けた 0・rate null・記録だけ 3 → 候補にしない（届けていない）
func fakeNews(t *testing.T, o Options) Options {
	dir := filepath.Join(t.TempDir(), "andon", "dispatcher")
	o.Delivered = filepath.Join(dir, "sample-site.delivered.jsonl")
	o.Recorded = filepath.Join(dir, "sample-site.recorded.jsonl")
	put(t, o.Delivered, strings.Join([]string{
		`{"at":"2026-10-31T23:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0001","kind":"report"}]}`,
		`{"at":"2026-11-01T01:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0001","kind":"report"},{"issue":"S-0002","kind":"ready"}]}`,
		`{"at":"2026-11-01T02:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0001","kind":"report"}]}`,
		`{"at":"2026-11-02T01:00:00Z","project":"sample-site","acked":false,"news":[{"issue":"S-0002","kind":"report"},{"issue":"S-0002","kind":"asked"}]}`,
		`broken`,
		`{"at":"2026-11-02T03:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0002","kind":"report"},{"issue":"S-0003"}]}`,
		`{"at":"2026-11-03T01:00:00Z","project":"sample-site","acked":false,"news":[{"issue":"S-0003","kind":"ready"}]}`,
		`{"at":"2026-11-04T23:59:59Z","project":"sample-site","acked":true,"news":[{"kind":"report"}]}`,
		`{"at":"2026-11-05T00:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0003","kind":"asked"}]}`,
	}, "\n")+"\n")
	put(t, o.Recorded, strings.Join([]string{
		`{"at":"2026-10-31T00:00:00Z","project":"sample-site","issue":"S-0001","kind":"merged","detail":{}}`,
		`{"at":"2026-11-01T06:00:00Z","project":"sample-site","issue":"S-0001","kind":"merged","detail":{}}`,
		`{"at":"2026-11-02T05:00:00Z","project":"sample-site","issue":"S-0002","kind":"review-requested","detail":{}}`,
		`{"at":"2026-11-02T12:00:00Z","project":"sample-site","issue":"S-0002","kind":"merged","detail":{}}`,
		`{"at":"nope","project":"sample-site","kind":"merged","detail":{}}`,
		`{"at":"2026-11-04T00:00:00Z","project":"sample-site","issue":"S-0003","kind":"merged","detail":{}}`,
	}, "\n")+"\n")
	return o
}

func TestNewsMatchesHandCount(t *testing.T) {
	res, err := Measure(fakeNews(t, opts(fakeLedger(t))))
	if err != nil {
		t.Fatal(err)
	}
	f := func(x float64) *float64 { return &x }
	want := []NewsKind{
		{Kind: "asked", Delivered: 1, Acked: 0, Rate: f(0)},
		{Kind: "merged", Recorded: 3},
		{Kind: "ready", Delivered: 2, Acked: 1, Rate: f(0.5)},
		{Kind: "report", Delivered: 5, Acked: 4, Rate: f(0.8)},
		{Kind: "review-requested", Recorded: 1},
	}
	got := res.News.Kinds
	if len(got) != len(want) {
		t.Fatalf("kind の数が違う: %+v", got)
	}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.Kind || g.Delivered != w.Delivered || g.Acked != w.Acked || g.Recorded != w.Recorded || val(g.Rate) != val(w.Rate) {
			t.Errorf("%s: got {届けた %d 素通り %d 率 %v 記録だけ %d} want {%d %d %v %d}",
				w.Kind, g.Delivered, g.Acked, val(g.Rate), g.Recorded, w.Delivered, w.Acked, val(w.Rate), w.Recorded)
		}
	}
	if res.News.Threshold != 0.8 {
		t.Errorf("閾値が 0.8 でない: %v", res.News.Threshold)
	}
	// 候補は閾値（8割）以上の kind だけ: report（0.8 ちょうど）は出し、ready（0.5）・asked（0）・届けていない merged は出さない
	if c := res.News.Candidates; len(c) != 1 || c[0] != "report" {
		t.Errorf("候補が report だけでない: %v", c)
	}
	var skipped []string
	for _, s := range res.Skipped {
		if strings.HasPrefix(s.Source, "dispatcher/") {
			skipped = append(skipped, fmt.Sprintf("%s:%d", s.Source, s.Line))
		}
	}
	if strings.Join(skipped, " ") != "dispatcher/sample-site.delivered.jsonl:5 dispatcher/sample-site.delivered.jsonl:6 dispatcher/sample-site.recorded.jsonl:5" {
		t.Errorf("飛ばした記録が違う: %v", skipped)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	out := b.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	cand := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "  記録だけへ移す候補") {
			cand = i
		}
	}
	if cand < 0 || !strings.HasSuffix(lines[cand], "  report") || strings.Contains(lines[cand], "ready") {
		t.Fatalf("候補の行が report だけでない:\n%s", out)
	}
	// 候補の行は表の末尾（後ろは飛ばした記録の一覧だけ）
	if cand+1 < len(lines) && (lines[cand+1] != "" || !strings.HasPrefix(lines[cand+2], "飛ばした記録")) {
		t.Errorf("候補の行が表の末尾でない:\n%s", out)
	}
	if !strings.Contains(out, "report             届けた 5・素通り 80%（4 / 5）・記録だけ 0") {
		t.Errorf("report の行が違う:\n%s", out)
	}
}

// 閾値を下回れば候補の行は「なし」。記録が無ければ通知の節は「記録なし」。
func TestNewsNoCandidate(t *testing.T) {
	o := opts(fakeLedger(t))
	dir := t.TempDir()
	o.Delivered = filepath.Join(dir, "x.delivered.jsonl")
	o.Recorded = filepath.Join(dir, "x.recorded.jsonl")
	put(t, o.Delivered, `{"at":"2026-11-01T01:00:00Z","project":"x","acked":true,"news":[{"kind":"report"},{"kind":"error"}]}`+"\n"+
		`{"at":"2026-11-01T02:00:00Z","project":"x","acked":false,"news":[{"kind":"report"},{"kind":"error"}]}`+"\n")
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.News.Candidates) != 0 {
		t.Errorf("5割の kind が候補に出た: %v", res.News.Candidates)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	if !strings.Contains(b.String(), "記録だけへ移す候補（80%以上が素通り。移すかは人が決める）  なし") {
		t.Errorf("候補なしの行が無い:\n%s", b.String())
	}
	res, _ = Measure(opts(fakeLedger(t))) // 記録の置き場を渡さない（まだ無い記録）
	b.Reset()
	Table(&b, res, columnsOrder(res))
	if !strings.Contains(b.String(), "通知（Chief of Staff へ届けた数と、andon news ack で終わった素通り）\n  記録なし\n") {
		t.Errorf("記録が無いのに「記録なし」が出ない:\n%s", b.String())
	}
}

// fakeTokens は偽の Claude Code・Codex の記録。期間 2026-11-01〜04 に log.jsonl の行がある課題は S-0001・S-0002・S-0002-r1・S-0003。
//
//	S-0001（claude。session aaaa…）: 要求 req_1 が content block ごとに3行（同じ usage）→ 1回だけ。req_2 が1行。
//	       requestId の無い行は message.id（msg_3）で2行 → 1回。synthetic の行（id も無い）は数えない。サブエージェントの req_4 が1行。
//	       入力 1+2+3+4=10・出力 10+20+30+40=100・キャッシュ読み 100+200+300+400=1000・キャッシュ書き 5+6+7+8=26
//	S-0002（claude。session bbbb…）: 記録が無い → usage null（0 と分ける）
//	S-0002-r1（codex。cwd は台帳の issues/S-0002-r1）: rollout 2本（起き直し）。それぞれ最後の token_count を足す。
//	       1本目 input 500（cached 300・write 50）・output 40、2本目 input 200（cached 100）・output 10
//	       → 入力 (500-300-50)+(200-100)=250・出力 50・キャッシュ読み 400・キャッシュ書き 50
//	S-0003（claude。session 無し）→ usage null
func fakeTokens(t *testing.T, o Options) Options {
	home := t.TempDir()
	o.ClaudeDir = filepath.Join(home, "claude")
	o.CodexSessions = filepath.Join(home, "codex", "sessions")
	k := filepath.Join(o.Root, "kanban")
	a := "aaaaaaaa-0000-4000-8000-000000000001"
	put(t, filepath.Join(k, "issues/S-0001/issue.yaml"), "id: S-0001\ntitle: one\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\nsession: "+a+"\n")
	put(t, filepath.Join(k, "issues/S-0002/issue.yaml"),
		"id: S-0002\ntitle: two\nstatus: done\ncreated: \"2026-11-02T00:00:00Z\"\nsession: bbbbbbbb-0000-4000-8000-000000000002\n")
	usage := func(req, id string, in, out, cr, cw int) string {
		r := ""
		if req != "" {
			r = `"requestId":"` + req + `",`
		}
		return fmt.Sprintf(`{"type":"assistant",%s"message":{"id":"%s","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`,
			r, id, in, out, cr, cw)
	}
	main := filepath.Join(o.ClaudeDir, "projects", "-sample-worktree", a+".jsonl")
	put(t, main, strings.Join([]string{
		`{"type":"user","message":{"content":"x"}}`,
		usage("req_1", "msg_1", 1, 10, 100, 5),
		usage("req_1", "msg_1", 1, 10, 100, 5),
		usage("req_1", "msg_1", 1, 10, 100, 5),
		usage("req_2", "msg_2", 2, 20, 200, 6),
		usage("", "msg_3", 3, 30, 300, 7),
		usage("", "msg_3", 3, 30, 300, 7),
		usage("", "", 9, 9, 9, 9),
	}, "\n")+"\n")
	put(t, filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-x.jsonl"), usage("req_4", "msg_4", 4, 40, 400, 8)+"\n"+usage("req_1", "msg_1", 1, 10, 100, 5)+"\n")
	cwd, _ := filepath.Abs(filepath.Join(k, "issues", "S-0002-r1"))
	meta := `{"type":"session_meta","payload":{"id":"x","cwd":"` + cwd + `"}}`
	count := func(in, cr, cw, out int) string {
		return fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":%d,"output_tokens":%d}}}}`, in, cr, cw, out)
	}
	day := filepath.Join(o.CodexSessions, "2026", "11", "02")
	put(t, filepath.Join(day, "rollout-1.jsonl"), meta+"\n"+count(100, 50, 0, 5)+"\n"+`{"type":"event_msg","payload":{"type":"token_count","info":null}}`+"\n"+count(500, 300, 50, 40)+"\n")
	put(t, filepath.Join(day, "rollout-2.jsonl"), meta+"\n"+count(200, 100, 0, 10)+"\n")
	put(t, filepath.Join(day, "rollout-3.jsonl"), `{"type":"session_meta","payload":{"id":"y","cwd":"/elsewhere"}}`+"\n"+count(9999, 0, 0, 9999)+"\n")
	return o
}

func TestTokensCountEachRequestOnce(t *testing.T) {
	o := fakeTokens(t, opts(fakeLedger(t)))
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Tokens{}
	var order []string
	for _, x := range res.Tokens {
		got[x.Issue] = x
		order = append(order, x.Issue)
	}
	if strings.Join(order, " ") != "S-0001 S-0002 S-0002-r1 S-0003" {
		t.Errorf("課題の並び: %v", order)
	}
	want := map[string]*TokenUsage{
		"S-0001":    {Input: 10, Output: 100, CacheRead: 1000, CacheWrite: 26},
		"S-0002":    nil,
		"S-0002-r1": {Input: 250, Output: 50, CacheRead: 400, CacheWrite: 50},
		"S-0003":    nil,
	}
	for id, w := range want {
		g := got[id]
		switch {
		case w == nil && g.Usage != nil:
			t.Errorf("%s: 記録が無いのに %+v", id, *g.Usage)
		case w != nil && (g.Usage == nil || *g.Usage != *w):
			t.Errorf("%s: %+v、want %+v", id, g.Usage, *w)
		}
	}
	if got["S-0002-r1"].Source != "codex" || got["S-0001"].Source != "claude" {
		t.Errorf("source: %+v", res.Tokens)
	}
	js, _ := json.Marshal(got["S-0002"])
	if string(js) != `{"issue":"S-0002","source":"claude","usage":null}` {
		t.Errorf("記録の無い課題の形: %s", js)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	if !strings.Contains(b.String(), "S-0001       claude 入力 10・出力 100・キャッシュ読み 1000・キャッシュ書き 26") ||
		!strings.Contains(b.String(), "S-0003       claude 記録なし") {
		t.Errorf("トークンの節が違う:\n%s", b.String())
	}
}

func TestTokensBrokenLineIsSkipped(t *testing.T) {
	o := fakeTokens(t, opts(fakeLedger(t)))
	main, _ := filepath.Glob(filepath.Join(o.ClaudeDir, "projects", "*", "aaaaaaaa-0000-4000-8000-000000000001.jsonl"))
	b, _ := os.ReadFile(main[0])
	put(t, main[0], string(b)+`{"type":"assistant","requestId":"req_9","message":{"usage":{"input_tokens":`+"\n")
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	src := skippedSources(res)
	if _, ok := src["claude/aaaaaaaa-0000-4000-8000-000000000001.jsonl"]; !ok {
		t.Errorf("壊れた行が飛ばした記録に無い: %+v", res.Skipped)
	}
	for _, s := range res.Skipped {
		if filepath.IsAbs(s.Source) || strings.Contains(s.Reason, o.ClaudeDir) {
			t.Errorf("飛ばした記録に絶対パス: %+v", s)
		}
	}
}

// 見込みと実際（ADR-0014 決めたこと 9・contract.md §13）。期間 2026-11-01（1日）で手で数えた値:
//
//	S-0011 見込み medium（Chief of Staff）・担当の見込み large。doing 01:00〜02:30 の 90 分から、自分の問い 10 分・
//	       レビュー待ち（S-0011-r1 の add 02:00〜review 02:20）20 分・死んでいた区間（gone 02:25〜agent 02:28）3 分を引いて 57 分。比 0.95（収まった）
//	S-0012 見込み small を書いたのが Chief of Staff でない → 見込み無し。実働 30 分
//	S-0013 見込み small（Chief of Staff）。実働 45 分。比 1.5（収まらない）。期間のあとに起動し直した doing（11-02 02:00〜）は数えない
//	S-0014 Chief Engineer の課題・S-0011-r1 レビュアーの課題・S-0015 期間の前に閉じた課題は載らない
func TestMeasureEstimate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: S\nchief_of_staff: t0\n")
	issue := func(id, extra string) {
		put(t, filepath.Join(k, "issues", id, "issue.yaml"), "id: "+id+"\ntitle: x\nstatus: done\ncreated: \"2026-10-31T00:00:00Z\"\n"+extra)
	}
	issue("S-0011", "estimate: medium\n")
	put(t, filepath.Join(k, "issues/S-0011/library/report.json"), `{"estimate": {"size": "large", "reason": "x"}}`)
	issue("S-0011-r1", "parent: S-0011\nprofile: codex-review\n")
	issue("S-0012", "estimate: small\n")
	issue("S-0013", "estimate: small\n")
	issue("S-0014", "profile: chief-engineer\n")
	issue("S-0015", "")
	row := func(at, by, id, change string) string {
		return `{"at":"2026-11-01T` + at + `:00Z","by":"` + by + `","id":"` + id + `","change":` + change + `}`
	}
	put(t, filepath.Join(k, "log.jsonl"), strings.Join([]string{
		`{"at":"2026-10-31T01:00:00Z","by":"terminal:t0","id":"S-0015","change":{"status":"doing"}}`,
		`{"at":"2026-10-31T02:00:00Z","by":"terminal:t0","id":"S-0015","change":{"status":"done"}}`,
		row("00:00", "terminal:t0", "S-0011", `{"estimate":"medium"}`),
		row("00:00", "terminal:t9", "S-0012", `{"estimate":"small"}`),
		row("00:00", "terminal:t0", "S-0013", `{"estimate":"small"}`),
		row("01:00", "terminal:t1", "S-0011", `{"status":"doing","agent":"t1"}`),
		row("01:30", "terminal:t1", "S-0011", `{"question":1,"status":"open"}`),
		row("01:40", "terminal:t0", "S-0011", `{"question":1,"status":"answered"}`),
		row("02:00", "dispatcher", "S-0011-r1", `{"add":"review"}`),
		row("02:20", "terminal:t5", "S-0011-r1", `{"status":"review"}`),
		row("02:28", "terminal:t0", "S-0011", `{"agent":"t2"}`),
		row("02:30", "terminal:t2", "S-0011", `{"status":"review"}`),
		row("03:00", "dispatcher", "S-0011", `{"status":"done"}`),
		row("04:00", "terminal:t3", "S-0012", `{"status":"doing"}`),
		row("04:30", "terminal:t3", "S-0012", `{"status":"done"}`),
		row("05:00", "terminal:t4", "S-0013", `{"status":"doing"}`),
		row("05:45", "terminal:t4", "S-0013", `{"status":"done"}`),
		// 期間のあと（11-02）に起動し直して doing のまま: 期間に閉じたときまでの実働 45 分だけを数える
		`{"at":"2026-11-02T01:00:00Z","by":"terminal:t0","id":"S-0013","change":{"status":"todo"}}`,
		`{"at":"2026-11-02T02:00:00Z","by":"terminal:t4","id":"S-0013","change":{"status":"doing"}}`,
		row("06:00", "terminal:t6", "S-0014", `{"status":"doing"}`),
		row("09:00", "terminal:t6", "S-0014", `{"status":"done"}`),
	}, "\n")+"\n")
	gone := time.Date(2026, 11, 1, 2, 25, 0, 0, time.UTC).UnixMilli()
	state := filepath.Join(t.TempDir(), "sample-site.json")
	put(t, state, fmt.Sprintf(`{"gone": {"S-0011": [{"agent": "t1", "at": %d}]}}`, gone))
	res, err := Measure(Options{Root: root, Since: day("2026-11-01"), Until: day("2026-11-01"), Now: day("2026-11-03"),
		GH: func(string) ([]byte, error) { return []byte("[]"), nil }, State: state})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(map[string]any{"estimate": res.Estimate, "issues": res.Issues})
	want := `{"estimate":{"count":2,"within":1,"rate":0.5,"none":1},"issues":[` +
		`{"issue":"S-0011","estimate_minutes":60,"agent_estimate_minutes":120,"worked_minutes":57,"ratio":0.95},` +
		`{"issue":"S-0012","estimate_minutes":null,"agent_estimate_minutes":null,"worked_minutes":30,"ratio":null},` +
		`{"issue":"S-0013","estimate_minutes":30,"agent_estimate_minutes":null,"worked_minutes":45,"ratio":1.5}]}`
	if string(got) != want {
		t.Fatalf("見込みと実際:\n got %s\nwant %s", got, want)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	for _, s := range []string{"見込みの中に収まった割合       50%（1 / 2）・見込み無し 1件", "S-0011       見込み 60（担当の見込み 120）・実働 57・比 0.95", "S-0012       見込み無し・実働 30"} {
		if !strings.Contains(b.String(), s) {
			t.Errorf("表に %q が無い:\n%s", s, b.String())
		}
	}
}
