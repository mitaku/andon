package contract

import (
	"strings"
	"testing"
)

const sha = "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"

func TestParseReport(t *testing.T) {
	good := `{"commit":"` + sha + `","pr":"https://github.com/o/r/pull/1",
	 "review_request":{"round":1,"limit":2,"commit":"` + sha + `","previous":null,"reviewer":"codex-review","extra":false},
	 "waived":[{"severity":"minor","file":"a/b","line":3,"reason":"x"}]}`
	if _, err := ParseReport([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"知らないキー":                strings.Replace(good, `"pr"`, `"x":1,"pr"`, 1),
		"previous が無い":          strings.Replace(good, `"previous":null,`, ``, 1),
		"commit が短い":            strings.Replace(good, `"commit":"`+sha+`","pr"`, `"commit":"9c4e1b0","pr"`, 1),
		"pr の owner に空白":        strings.Replace(good, "github.com/o/", "github.com/o o/", 1),
		"waived の file が .. の段": strings.Replace(good, `"a/b"`, `"a/../../b"`, 1),
		"reviewer が知らない":        strings.Replace(good, "codex-review", "claude", 1),
		"後ろに余り":                 good + "{}",
	} {
		if _, err := ParseReport([]byte(bad)); err == nil {
			t.Errorf("%s: 通ってしまう", name)
		}
	}
}

func TestParseQuestion(t *testing.T) {
	good := `{"kind":"line","question":"q","options":[{"label":"a","pick":true},{"label":"b","pick":false}],"remaining":0}`
	q, err := ParseQuestion([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if q.LedgerKind() != "" || !strings.Contains(q.Text(), "推奨案: a") || strings.Contains(q.Text(), "あと") {
		t.Errorf("text・kind: %q %q", q.Text(), q.LedgerKind())
	}
	for name, bad := range map[string]string{
		"推奨案が2つ":    strings.Replace(good, `"pick":false`, `"pick":true`, 1),
		"先頭が推奨案でない": strings.Replace(good, `"pick":true`, `"pick":false`, 1),
		"2行":        strings.Replace(good, `"q"`, `"a\nb"`, 1),
		"reason が空": strings.Replace(good, `"remaining"`, `"reason":"","remaining"`, 1),
		"from が人":   strings.Replace(good, `"remaining":0`, `"from":"person"`, 1),
	} {
		if _, err := ParseQuestion([]byte(bad)); err == nil {
			t.Errorf("%s: 通ってしまう", name)
		}
	}
}

func TestRelPath(t *testing.T) {
	for p, want := range map[string]bool{"a/b": true, "..hidden/x": true, "": false, "/a": false, "a/../b": false, "a\nb": false, " /../x": false} {
		if RelPath(p) != want {
			t.Errorf("RelPath(%q) want %v", p, want)
		}
	}
}

// 入口は存在するキーの null・型違いを拒む（schema と同じ）。null を許すのは review_request.previous だけ
func TestDecodeRejectsNullAndWrongTypes(t *testing.T) {
	q := `{"kind":"users","question":"q","options":[{"label":"a","pick":true},{"label":"b"}]}`
	for name, bad := range map[string]string{
		"options[1].pick が null": strings.Replace(q, `{"label":"b"}`, `{"label":"b","pick":null}`, 1),
		"remaining が null":       strings.Replace(q, `"kind"`, `"remaining":null,"kind"`, 1),
		"facts が null":           strings.Replace(q, `"kind"`, `"facts":null,"kind"`, 1),
		"from が null":            strings.Replace(q, `"kind"`, `"from":null,"kind"`, 1),
		"facts の要素が null":        strings.Replace(q, `"kind"`, `"facts":[null],"kind"`, 1),
		"remaining が文字列":         strings.Replace(q, `"kind"`, `"remaining":"1","kind"`, 1),
		"全体が null":               `null`,
	} {
		if _, err := ParseQuestion([]byte(bad)); err == nil {
			t.Errorf("問い: %s: 通ってしまう", name)
		}
	}
	r := `{"commit":"` + sha + `"}`
	for name, bad := range map[string]string{
		"pr が null":                      strings.Replace(r, `}`, `,"pr":null}`, 1),
		"waived が null":                  strings.Replace(r, `}`, `,"waived":null}`, 1),
		"next の要素が null":                 strings.Replace(r, `}`, `,"next":[null]}`, 1),
		"review_request が null":          strings.Replace(r, `}`, `,"review_request":null}`, 1),
		"review_request.reviewer が null": strings.Replace(r, `}`, `,"review_request":{"round":1,"limit":2,"commit":"`+sha+`","previous":null,"reviewer":null,"extra":false}}`, 1),
	} {
		if _, err := ParseReport([]byte(bad)); err == nil {
			t.Errorf("report.json: %s: 通ってしまう", name)
		}
	}
	if err := Decode([]byte(`{"commit":"`+sha+`","findings":null}`), &Findings{}); err == nil {
		t.Error("findings.json: findings が null で通ってしまう")
	}
}
