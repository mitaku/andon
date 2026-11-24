package measure

import (
	"andon/internal/ledger/ledgertest"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// touchesLedger はオーナーの介在（ADR-0049 決めたこと 3・7・8・9・11）を手で数える偽の kanban。期間は 2026-11-01〜04。
//
//	S-0001 PR #1 を dispatcher がマージ。owner の答えは期間の前（PR の行には 1、判断には 0）。端末でない by の行がある。
//	S-0002 PR #2 は GitHub でマージ（merged の印が別の PR の記録にだけある）。owner・chief-of-staff・machine の答えが1つずつ。
//	       PR のコメントを持つ手直しの記録は数えない。PR の行は マージ 1 ＋ 答え 1。
//	S-0003 PR #3 は gate を通らずにマージ。owner の答えは期間の直後（PR の行には 1、判断には 0）。
//	S-0004 PR #4 は期間の前にマージ。owner の答えは期間の始まりちょうど（判断 1）。
//	S-0005 問いを持って片付いたオーナーの課題。owner の答え（判断 1。課題は数えない）。
//	S-0006 問いを持たずに片付いたオーナーの課題（1）。
//	S-0007 期間の直後に片付いたオーナーの課題（0）。
//	S-0008 答えの無い問いを持って片付いたオーナーの課題（0）。
//	S-0009 印の無い答えと、3つの値のどれでもない印の答え 4 つ（unmarked 5）。
//	S-0010 期間に2度片付いたオーナーの課題（1）。
//	S-0011 questions.yaml が空の並びのオーナーの課題（1）。
//	S-0012 片付いていないオーナーの課題（0）。
//	S-0013 問いを持たずに片付いた担当の課題（0）。
//	S-0002-r1 レビュアーの課題。owner の答えも数えない。
//
//	マージ 2 ＋ 判断 3 ＋ オーナーの課題 3 = 8。8 / 3 本 = 2.67。PR の行は 1・2・2（和は 8 と一致しない）。
func touchesLedger(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: S\n")
	issue := func(id, extra string) {
		put(t, filepath.Join(k, "issues", id, "issue.yaml"),
			"id: "+id+"\ntitle: t\nstatus: done\ncreated: \"2026-10-30T00:00:00Z\"\n"+extra)
	}
	withPR := func(id string, n int) {
		issue(id, "")
		put(t, filepath.Join(k, "issues", id, "library/report.json"), fmt.Sprintf(`{"pr": "%s%d"}`, repoURL, n))
	}
	answer := func(n int, answered, mark string) string {
		q := fmt.Sprintf("- n: %d\n  text: x\n  asked: \"2026-10-30T01:00:00Z\"\n", n)
		if answered == "" {
			return q + "  status: open\n"
		}
		return q + "  status: received\n  answered: \"" + answered + "\"\n" + mark
	}
	questions := func(id string, qs ...string) {
		put(t, filepath.Join(k, "issues", id, "questions.yaml"), strings.Join(qs, ""))
	}
	const owner = "  decidedBy: owner\n"

	withPR("S-0001", 1)
	put(t, filepath.Join(k, "issues/S-0001/library/gate/aaaa.json"),
		`{"at": "2026-11-01T06:00:00Z", "pr": "`+repoURL+`1", "tier": "A", "decision": "approve", "merged": true}`)
	questions("S-0001", answer(1, "2026-10-31T23:59:59Z", owner))

	withPR("S-0002", 2)
	put(t, filepath.Join(k, "issues/S-0002/library/gate/bbbb.json"),
		`{"at": "2026-11-01T07:00:00Z", "pr": "`+repoURL+`20", "tier": "B", "decision": "approve", "merged": true}`)
	put(t, filepath.Join(k, "issues/S-0002/library/gate/cccc.json"),
		`{"at": "2026-11-02T11:00:00Z", "pr": "`+repoURL+`2", "tier": "C", "decision": "needs_human", "merged": false}`)
	put(t, filepath.Join(k, "issues/S-0002/library/corrections.json"),
		`{"at": "2026-11-02T13:00:00Z", "corrections": [{"kind": "pr", "comments": [{"author": "someone", "at": "2026-11-02T11:30:00Z", "body": "x"}]}]}`)
	questions("S-0002",
		answer(1, "2026-11-02T05:00:00Z", owner),
		answer(2, "2026-11-02T06:00:00Z", "  decidedBy: chief-of-staff\n"),
		answer(3, "2026-11-02T12:00:00Z", "  decidedBy: machine\n"))

	withPR("S-0003", 3)
	questions("S-0003", answer(1, "2026-11-05T00:00:00Z", owner))

	withPR("S-0004", 4)
	questions("S-0004", answer(1, "2026-11-01T00:00:00Z", owner))

	for _, id := range []string{"S-0005", "S-0006", "S-0007", "S-0008", "S-0010", "S-0011", "S-0012"} {
		issue(id, "who: person\n")
	}
	questions("S-0005", answer(1, "2026-11-04T23:59:59Z", owner))
	questions("S-0008", answer(1, "", ""))
	questions("S-0011", "[]\n")

	issue("S-0009", "")
	questions("S-0009",
		answer(1, "2026-11-03T01:00:00Z", ""),
		answer(2, "2026-11-03T02:00:00Z", "  decidedBy: person\n"),
		answer(3, "2026-11-03T03:00:00Z", "  decidedBy: Owner\n"),
		answer(4, "2026-11-03T04:00:00Z", "  decidedBy:\n"),
		answer(5, "2026-11-03T05:00:00Z", "  decidedBy: [owner]\n")) // 文字列でない印でもファイルを丸ごと落とさない

	issue("S-0013", "")
	issue("S-0002-r1", "parent: S-0002\nprofile: codex-review\nwho: person\n")
	questions("S-0002-r1", answer(1, "2026-11-02T06:30:00Z", owner))

	line := func(at, by, id, status string) string {
		return `{"at":"` + at + `","by":"` + by + `","id":"` + id + `","change":{"status":"` + status + `"}}`
	}
	done := func(at, id string) string { return line(at, "terminal:t1", id, "done") }
	put(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t,
		`{"at":"2026-11-01T02:00:00Z","by":"someone","id":"S-0001","change":{"merge":"auto"}}`, // by が端末でない行は数えない
		`{"at":"2026-11-02T12:00:00Z","by":"someone","id":"S-0002","change":{"merge":"person"}}`,
		done("2026-11-01T06:00:00Z", "S-0001"),
		line("2026-11-01T08:00:00Z", "terminal:t1", "S-0012", "review"),
		done("2026-11-02T01:00:00Z", "S-0005"),
		done("2026-11-02T02:00:00Z", "S-0006"),
		done("2026-11-02T03:00:00Z", "S-0008"),
		done("2026-11-02T04:00:00Z", "S-0010"),
		line("2026-11-02T05:00:00Z", "terminal:t1", "S-0010", "todo"),
		done("2026-11-02T06:00:00Z", "S-0010"),
		done("2026-11-02T06:30:00Z", "S-0002-r1"),
		done("2026-11-02T07:00:00Z", "S-0011"),
		done("2026-11-02T08:00:00Z", "S-0013"),
		done("2026-11-05T00:00:00Z", "S-0007"),
	))
	return root
}

func touchesGH(string) ([]byte, error) {
	var list []string
	for i, at := range []string{"2026-11-01T06:00:00Z", "2026-11-02T12:00:00Z", "2026-11-03T09:00:00Z", "2026-10-31T09:00:00Z"} {
		list = append(list, fmt.Sprintf(`{"number": %d, "url": "%s%d", "title": "t", "body": "", "mergedAt": "%s"}`, i+1, repoURL, i+1, at))
	}
	return []byte("[" + strings.Join(list, ",") + "]"), nil
}

// touchesJSON は efficiency のうち介在と答えの数を、--json のキーのまま返す。
func touchesJSON(t *testing.T, res *Result) (touches, answers string) {
	t.Helper()
	b, err := json.Marshal(res.Efficiency)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return string(v["human_touches"]), string(v["answers"])
}

func TestTouchesMatchHandCount(t *testing.T) {
	o := opts(touchesLedger(t))
	o.GH = touchesGH
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	touches, answers := touchesJSON(t, res)
	if want := `{"total":8,"per_pr":2.67,"merge":2,"decision":3,"task":3}`; touches != want {
		t.Errorf("human_touches: got %s, want %s", touches, want)
	}
	if want := `{"owner":3,"chief_of_staff":1,"machine":1,"unmarked":5}`; answers != want {
		t.Errorf("answers: got %s, want %s", answers, want)
	}
	var rows []string
	for _, x := range res.PRs {
		rows = append(rows, fmt.Sprintf("%s auto=%t touches=%d", x.Issue, x.Auto, x.HumanTouches))
	}
	if got, want := strings.Join(rows, " "), "S-0001 auto=true touches=1 S-0002 auto=false touches=2 S-0003 auto=false touches=2"; got != want {
		t.Errorf("prs: got %s, want %s", got, want)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("skipped: %+v", res.Skipped)
	}

	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	for _, want := range []string{
		"2.67（8回 / 3本。マージ 2・判断 3・オーナーの課題 3）",
		"オーナー 3・Chief of Staff 1・command 1・印なし 5",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("表に %q が無い:\n%s", want, b.String())
		}
	}
}

// マージされた PR が無くても、判断とオーナーの課題は数える（per_pr は null）。
func TestTouchesWithoutMergedPR(t *testing.T) {
	o := opts(touchesLedger(t))
	o.GH = func(string) ([]byte, error) { return nil, errors.New("not logged in") }
	res, err := Measure(o)
	if err != nil {
		t.Fatal(err)
	}
	touches, _ := touchesJSON(t, res)
	if want := `{"total":6,"per_pr":null,"merge":0,"decision":3,"task":3}`; touches != want {
		t.Errorf("human_touches: got %s, want %s", touches, want)
	}
	var b bytes.Buffer
	Table(&b, res, columnsOrder(res))
	if want := "記録なし（6回 / 0本。マージ 0・判断 3・オーナーの課題 3）"; !strings.Contains(b.String(), want) {
		t.Errorf("表に %q が無い:\n%s", want, b.String())
	}
}

// 問いを持つか分からない・問いの時刻が読めないオーナーの課題は、問いを持たない課題として数えず、飛ばした記録に出す。
func TestBrokenQuestionsAreNotCountedAsTask(t *testing.T) {
	for name, c := range map[string]struct {
		body string
		mode os.FileMode
	}{
		"YAML でない":    {"- n: [\n", 0o644},
		"並びでない":       {"n: 1\n", 0o644},
		"読む権限が無い":     {"[]\n", 0},
		"asked が読めない": {"- n: 1\n  asked: \"yesterday\"\n  answered: \"2026-11-02T01:00:00Z\"\n  decidedBy: owner\n", 0o644},
	} {
		t.Run(name, func(t *testing.T) {
			if c.mode == 0 && os.Geteuid() == 0 {
				t.Skip("root は読み取り権限を無視する")
			}
			root := touchesLedger(t)
			p := filepath.Join(root, "kanban/issues/S-0006/questions.yaml")
			put(t, p, c.body)
			if err := os.Chmod(p, c.mode); err != nil {
				t.Fatal(err)
			}
			o := opts(root)
			o.GH = touchesGH
			res, err := Measure(o)
			if err != nil {
				t.Fatal(err)
			}
			touches, _ := touchesJSON(t, res)
			if want := `{"total":7,"per_pr":2.33,"merge":2,"decision":3,"task":2}`; touches != want {
				t.Errorf("human_touches: got %s, want %s", touches, want)
			}
			if _, ok := skippedSources(res)["kanban/issues/S-0006/questions.yaml"]; !ok {
				t.Errorf("飛ばした記録に無い: %+v", res.Skipped)
			}
		})
	}
}

// もう片付いている課題に done を打ち直した行は、最後の列へ動いたうちに入らない。戻してから片付ければ入る。
func TestTaskCountsOnlyMovesToLastColumn(t *testing.T) {
	for name, c := range map[string]struct {
		between string
		total   int
		task    int
	}{
		"打ち直し":     {"", 8, 3},
		"戻してから片付け": {"todo", 9, 4},
	} {
		t.Run(name, func(t *testing.T) {
			root := touchesLedger(t)
			k := filepath.Join(root, "kanban")
			put(t, filepath.Join(k, "issues/S-0014/issue.yaml"), "id: S-0014\ntitle: t\nstatus: done\nwho: person\n")
			rows := []string{`{"at":"2026-10-20T00:00:00Z","by":"terminal:t1","id":"S-0014","change":{"status":"done"}}`}
			if c.between != "" {
				rows = append(rows, `{"at":"2026-10-25T00:00:00Z","by":"terminal:t1","id":"S-0014","change":{"status":"`+c.between+`"}}`)
			}
			rows = append(rows, `{"at":"2026-11-02T00:00:00Z","by":"terminal:t1","id":"S-0014","change":{"status":"done"}}`)
			log, err := os.ReadFile(filepath.Join(k, "log.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(k, "log.jsonl"), string(log)+ledgertest.Log(t, rows...))
			o := opts(root)
			o.GH = touchesGH
			res, err := Measure(o)
			if err != nil {
				t.Fatal(err)
			}
			if h := res.Efficiency.HumanTouches; h.Total != c.total || h.Task != c.task {
				t.Errorf("human_touches: got %+v, want total %d task %d", h, c.total, c.task)
			}
		})
	}
}

// 答え済みの問いに answered が無ければ、開いたままに数えず、飛ばした記録に出す。
func TestAnswerWithoutTimeIsSkipped(t *testing.T) {
	for _, status := range []string{"answered", "received"} {
		t.Run(status, func(t *testing.T) {
			root := touchesLedger(t)
			put(t, filepath.Join(root, "kanban/issues/S-0005/questions.yaml"),
				"- n: 1\n  text: x\n  asked: \"2026-11-01T01:00:00Z\"\n  status: "+status+"\n  answer: y\n  decidedBy: owner\n")
			o := opts(root)
			o.GH = touchesGH
			res, err := Measure(o)
			if err != nil {
				t.Fatal(err)
			}
			if h := res.Efficiency.HumanTouches; h.Decision != 2 || h.Task != 3 {
				t.Errorf("human_touches: %+v", h)
			}
			if why := skippedSources(res)["kanban/issues/S-0005/questions.yaml"]; !strings.Contains(why, "answered") {
				t.Errorf("飛ばした記録に無い: %+v", res.Skipped)
			}
		})
	}
}

// もう片付いている課題に done を打ち直した行は、期間に片付いた課題（flow.closed）にも、期間に最後の列へ動いた課題の
// 見込みと実際（issues）にも入らない。期間の前に片付いた課題を期間の中で打ち直しても、期間ごとに数え直さない。
func TestClosedSkipsRedoneIssue(t *testing.T) {
	base := func(t *testing.T) int {
		res, err := Measure(withGH(opts(touchesLedger(t))))
		if err != nil {
			t.Fatal(err)
		}
		return res.Flow.Closed.Count
	}(t)
	for name, c := range map[string]struct {
		between string
		closed  int
		listed  bool
	}{
		"打ち直し":     {"", base, false},
		"戻してから片付け": {"todo", base + 1, true},
	} {
		t.Run(name, func(t *testing.T) {
			root := touchesLedger(t)
			k := filepath.Join(root, "kanban")
			put(t, filepath.Join(k, "issues/S-0015/issue.yaml"), "id: S-0015\ntitle: t\nstatus: done\ncreated: \"2026-10-10T00:00:00Z\"\n")
			rows := []string{`{"at":"2026-10-20T00:00:00Z","by":"terminal:t1","id":"S-0015","change":{"status":"done"}}`}
			if c.between != "" {
				rows = append(rows, `{"at":"2026-10-25T00:00:00Z","by":"terminal:t1","id":"S-0015","change":{"status":"`+c.between+`"}}`)
			}
			rows = append(rows, `{"at":"2026-11-02T00:00:00Z","by":"terminal:t1","id":"S-0015","change":{"status":"done"}}`)
			log, err := os.ReadFile(filepath.Join(k, "log.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(k, "log.jsonl"), string(log)+ledgertest.Log(t, rows...))
			res, err := Measure(withGH(opts(root)))
			if err != nil {
				t.Fatal(err)
			}
			if got := res.Flow.Closed.Count; got != c.closed {
				t.Errorf("flow.closed: got %d, want %d", got, c.closed)
			}
			listed := false
			for _, x := range res.Issues {
				listed = listed || x.Issue == "S-0015"
			}
			if listed != c.listed {
				t.Errorf("issues に S-0015: got %v, want %v", listed, c.listed)
			}
		})
	}
}

func withGH(o Options) Options {
	o.GH = touchesGH
	return o
}
