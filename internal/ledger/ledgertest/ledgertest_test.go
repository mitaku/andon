package ledgertest

import (
	"fmt"
	"strings"
	"testing"
)

type fakeT struct {
	testing.TB
	fatal string
}

func (f *fakeT) Helper()                        {}
func (f *fakeT) Fatalf(format string, a ...any) { f.fatal = fmt.Sprintf(format, a...) }
func (f *fakeT) Fatal(a ...any)                 { f.fatal = fmt.Sprint(a...) }

const at = `"at":"2026-11-02T10:03:00Z"`

func TestLogLineShapes(t *testing.T) {
	for _, c := range []struct {
		line string
		ok   bool
		why  string
	}{
		{`{` + at + `,"by":"terminal:t3","id":"FD-0012","change":{"status":"doing","agent":"t3"}}`, true, "ledger.md の例"},
		{`{` + at + `,"by":"mitaku","id":"W-0001","change":{"add":"First"}}`, true, "kanban add"},
		{`{` + at + `,"by":"unknown","id":"W-0001","change":{"report":"append"}}`, true, "kanban report --append"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"links":["https://a.example","docs/x.md"],"phase":null}}`, true, "kanban set の並びと消したキー"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"after":null,"status":"todo"}}`, true, "after を空にして todo に出た"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"question":2,"status":"answered"}}`, true, "問い"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"status":"done","forced":[1,3]}}`, true, "done --force"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0002","change":{"status":"todo","freedBy":"W-0001"}}`, true, "done で queued から出た"},
		{`{` + at + `,"by":"terminal:t3","id":"A-5","change":{"review":"on"}}`, true, "4 桁に揃える前の id"},
		{`{` + at + `,"by":"terminal:t2","config":{"chief_of_staff":"t2"}}`, true, "kanban config set"},
		{`{` + at + `,"by":"terminal:t2","config":{"coordinator":null}}`, true, "2026-11-06 より前の config の行"},
		{`{` + at + `,"by":"routine:after","routine":"after","fired":"W-0004"}`, true, "ルーチンが課題を足した"},
		{`{` + at + `,"by":"routine:after","routine":"after","skipped":"paused"}`, true, "ルーチンが止めてある"},
		{`{` + at + `,"by":"terminal:t3","migrate":{"from":".flightdeck","ids":{"A-5":"A-0005"}}}`, true, "kanban migrate"},

		{`{"by":"terminal:t3","id":"W-0001","change":{"add":"x"}}`, false, "at が無い"},
		{`{"at":"x","by":"terminal:t3","id":"W-0001","change":{"add":"x"}}`, false, "at が時刻でない"},
		{`{"at":"2026-11-02","by":"terminal:t3","id":"W-0001","change":{"add":"x"}}`, false, "at が日付だけ"},
		{`{` + at + `,"id":"W-0001","change":{"add":"x"}}`, false, "by が無い"},
		{`{` + at + `,"by":"","id":"W-0001","change":{"add":"x"}}`, false, "by が空"},
		{`{` + at + `,"by":"terminal:t3","change":{"add":"x"}}`, false, "id が無い"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001"}`, false, "change が無い"},
		{`{` + at + `,"by":"terminal:t3","id":"../W-0001","change":{"add":"x"}}`, false, "id がフォルダの外を指す"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"add":"x"},"x_unknown":1}`, false, "知らないキー"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"add":"x"},"config":{}}`, false, "課題の行と config の行が混ざる"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":"doing"}`, false, "change が object でない"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"round":1}}`, false, "change の値が数"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"report":"replace"}}`, false, "report が write・append でない"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"question":"2","status":"open"}}`, false, "question が文字列"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"question":0,"status":"open"}}`, false, "question が 0"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"question":2}}`, false, "question に status が無い"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"question":2,"status":"closed"}}`, false, "問いの status が知らない値"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"forced":[1]}}`, false, "forced に status が無い"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"status":"done","forced":[]}}`, false, "forced が空"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0002","change":{"freedBy":"W-0001"}}`, false, "freedBy に status が無い"},
		{`{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"after":"W-0002"}}`, false, "after が並びでない"},
		{`{` + at + `,"by":"terminal:t2","config":{"chief_of_staff":2}}`, false, "config の値が数"},
		{`{` + at + `,"by":"terminal:t2","id":"W-0001","config":{"chief_of_staff":"t2"}}`, false, "config の行に id"},
		{`{` + at + `,"by":"routine:after","routine":"after"}`, false, "ルーチンの行に fired も skipped も無い"},
		{`{` + at + `,"by":"routine:after","routine":"after","fired":"W-0004","skipped":"open"}`, false, "fired と skipped の両方"},
		{`{` + at + `,"by":"routine:after","routine":"after","skipped":"later"}`, false, "skipped が open・paused でない"},
		{`{` + at + `,"by":"terminal:t3","routine":"after","fired":"W-0004"}`, false, "ルーチンの行の by が routine: でない"},
		{`{` + at + `,"by":"terminal:t3","migrate":{"from":".flightdeck"}}`, false, "migrate に ids が無い"},
		{`not json`, false, "JSON でない"},
	} {
		err := Check(LogSchema, []byte(c.line))
		if (err == nil) != c.ok {
			t.Errorf("%s: ok=%v、got %v\n%s", c.why, c.ok, err, c.line)
		}
	}
}

func TestLogFailsTheTestOnABrokenLine(t *testing.T) {
	good := `{` + at + `,"by":"terminal:t3","id":"W-0001","change":{"add":"x"}}`
	f := &fakeT{TB: t}
	if got := Log(f, good, good); got != good+"\n"+good+"\n" || f.fatal != "" {
		t.Errorf("通る行: %q %q", got, f.fatal)
	}
	if got := Log(f); got != "" || f.fatal != "" {
		t.Errorf("行が無い: %q %q", got, f.fatal)
	}
	Log(f, good, `{"by":"terminal:t3","id":"W-0002","change":{"add":"review"}}`)
	if !strings.Contains(f.fatal, "log.jsonl:2:") || strings.Contains(f.fatal, "log.jsonl:1:") {
		t.Errorf("2行目だけを言っていない: %s", f.fatal)
	}
}

func TestReadyFailsTheTestOnABrokenIssue(t *testing.T) {
	f := &fakeT{TB: t}
	if got := Ready(f); got != "[]" || f.fatal != "" {
		t.Errorf("空: %q %q", got, f.fatal)
	}
	ok := map[string]any{"id": "W-0001", "title": "x", "status": "todo", "after": []string{"W-0000"}, "mine": 1}
	if got := Ready(f, ok, map[string]any{"id": "W-0002", "status": "queued"}); f.fatal != "" || !strings.HasPrefix(got, `[{"after"`) {
		t.Errorf("通る並び: %q %q", got, f.fatal)
	}
	for _, c := range []struct {
		issue map[string]any
		why   string
	}{
		{map[string]any{"title": "x", "status": "todo"}, "id が無い"},
		{map[string]any{"id": "W-0001"}, "status が無い"},
		{map[string]any{"id": "W-0001", "status": "doing"}, "status が todo・queued でない"},
		{map[string]any{"id": "W-0001", "status": "todo", "after": "W-0000"}, "after が並びでない"},
		{map[string]any{"id": "W-0001", "status": "todo", "who": "agent"}, "who が person でない"},
		{map[string]any{"id": "W-0001", "status": "todo", "broken": "can't read"}, "読めない課題"},
	} {
		f := &fakeT{TB: t}
		Ready(f, c.issue)
		if f.fatal == "" {
			t.Errorf("%s で落ちない", c.why)
		}
	}
}
