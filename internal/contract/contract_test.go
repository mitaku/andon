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
	for _, risk := range []string{`{"one_way":true,"impact":["本番の表の列を消す"]}`, `{"one_way":false}`} {
		if _, err := ParseReport([]byte(strings.Replace(good, `"pr"`, `"risk":`+risk+`,"pr"`, 1))); err != nil {
			t.Errorf("risk %s: %v", risk, err)
		}
	}
	if _, err := ParseReport([]byte(strings.Replace(good, `"pr"`, `"estimate":{"size":"large","reason":"3つの package にまたがる"},"pr"`, 1))); err != nil {
		t.Errorf("estimate: %v", err)
	}
	for name, bad := range map[string]string{
		"知らないキー":                strings.Replace(good, `"pr"`, `"x":1,"pr"`, 1),
		"previous が無い":          strings.Replace(good, `"previous":null,`, ``, 1),
		"commit が短い":            strings.Replace(good, `"commit":"`+sha+`","pr"`, `"commit":"9c4e1b0","pr"`, 1),
		"pr の owner に空白":        strings.Replace(good, "github.com/o/", "github.com/o o/", 1),
		"waived の file が .. の段": strings.Replace(good, `"a/b"`, `"a/../../b"`, 1),
		"reviewer が知らない":        strings.Replace(good, "codex-review", "claude", 1),
		"後ろに余り":                 good + "{}",
		"risk に one_way が無い":    strings.Replace(good, `"pr"`, `"risk":{"impact":["x"]},"pr"`, 1),
		"risk.one_way が null":   strings.Replace(good, `"pr"`, `"risk":{"one_way":null},"pr"`, 1),
		"risk.one_way が文字列":     strings.Replace(good, `"pr"`, `"risk":{"one_way":"true"},"pr"`, 1),
		"risk.impact に空の行":      strings.Replace(good, `"pr"`, `"risk":{"one_way":true,"impact":[""]},"pr"`, 1),
		"risk に知らないキー":          strings.Replace(good, `"pr"`, `"risk":{"one_way":true,"level":"A"},"pr"`, 1),
		"estimate の段が段の名":       strings.Replace(good, `"pr"`, `"estimate":{"size":"大","reason":"x"},"pr"`, 1),
		"estimate に reason が無い": strings.Replace(good, `"pr"`, `"estimate":{"size":"large"},"pr"`, 1),
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
	if !strings.Contains(q.Text(), "推奨案: a") || strings.Contains(q.Text(), "あと") {
		t.Errorf("text: %q", q.Text())
	}
	if _, err := ParseQuestion([]byte(strings.Replace(good, `"line"`, `"scope"`, 1))); err != nil {
		t.Errorf("kind scope が通らない: %v", err)
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

// 候補の一覧（contract.md §12・ADR-0006 決めたこと 2）。schema の uniqueKeys（id の一意）もここで確かめる
func TestParseCandidates(t *testing.T) {
	good := `{"commit":"` + sha + `","candidates":[
	 {"id":"c1","text":"docs/a.md:4 誤字","file":"docs/a.md","line":4,"in_scope":true},
	 {"id":"c2","text":"README の段を書き直す","in_scope":false}]}`
	c, err := ParseCandidates([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Candidates) != 2 || !*c.Candidates[0].InScope || *c.Candidates[1].InScope || c.Candidates[1].File != nil {
		t.Errorf("読んだ形: %+v", c.Candidates)
	}
	for name, ok := range map[string]string{
		"commit 無し（repo の無い課題）": `{"candidates":[{"id":"c1","text":"x","in_scope":false}]}`,
		"候補 0": `{"candidates":[]}`,
	} {
		if _, err := ParseCandidates([]byte(ok)); err != nil {
			t.Errorf("%s: 通らない: %v", name, err)
		}
	}
	for name, bad := range map[string]string{
		"id が重なる":         strings.Replace(good, `"id":"c2"`, `"id":"c1"`, 1),
		"id の形":           strings.Replace(good, `"id":"c2"`, `"id":"x2"`, 1),
		"id が c0":         strings.Replace(good, `"id":"c2"`, `"id":"c0"`, 1),
		"in_scope 無し":     strings.Replace(good, `,"in_scope":false`, ``, 1),
		"in_scope が null": strings.Replace(good, `"in_scope":false`, `"in_scope":null`, 1),
		"知らないキー":          strings.Replace(good, `"id":"c2"`, `"tier":"A","id":"c2"`, 1),
		"上の知らないキー":        strings.Replace(good, `"candidates"`, `"x":1,"candidates"`, 1),
		"text が空":         strings.Replace(good, `"README の段を書き直す"`, `""`, 1),
		"text が2行":        strings.Replace(good, `"README の段を書き直す"`, `"a\nb"`, 1),
		"file が絶対":        strings.Replace(good, `"file":"docs/a.md"`, `"file":"/etc/x"`, 1),
		"file が空":         strings.Replace(good, `"file":"docs/a.md"`, `"file":""`, 1),
		"line が 0":        strings.Replace(good, `"line":4`, `"line":0`, 1),
		"commit が短い":      strings.Replace(good, sha, "9c4e1b0", 1),
		"candidates 無し":   `{"commit":"` + sha + `"}`,
	} {
		if _, err := ParseCandidates([]byte(bad)); err == nil {
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
		"observation が null":             strings.Replace(r, `}`, `,"observation":null}`, 1),
		"observation.result が enum の外":   strings.Replace(r, `}`, `,"observation":{"hypothesis":"h","result":"maybe","evidence":[]}}`, 1),
		"observation.hypothesis が空":      strings.Replace(r, `}`, `,"observation":{"hypothesis":"","result":"hit","evidence":[]}}`, 1),
		"observation.evidence が無い":       strings.Replace(r, `}`, `,"observation":{"hypothesis":"h","result":"hit"}}`, 1),
		"observation.evidence に空の行":      strings.Replace(r, `}`, `,"observation":{"hypothesis":"h","result":"hit","evidence":[""]}}`, 1),
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

// encoding/json はキーを大文字小文字を区別せず当てる。入口は名前の完全一致だけを受ける
func TestDecodeRejectsCaseVariantKeys(t *testing.T) {
	r := `{"commit":"` + sha + `"}`
	for name, bad := range map[string]string{
		"Commit":               strings.Replace(r, `"commit"`, `"Commit"`, 1),
		"PR が null":            strings.Replace(r, `}`, `,"PR":null}`, 1),
		"Pr が型違い":              strings.Replace(r, `}`, `,"Pr":12}`, 1),
		"review_request.Extra": strings.Replace(r, `}`, `,"review_request":{"round":1,"limit":2,"commit":"`+sha+`","previous":null,"reviewer":"codex-review","Extra":false}}`, 1),
	} {
		if _, err := ParseReport([]byte(bad)); err == nil {
			t.Errorf("report.json: %s: 通ってしまう", name)
		}
	}
	if _, err := ParseQuestion([]byte(`{"kind":"users","question":"q","options":[{"label":"a","pick":true},{"Label":"b"}]}`)); err == nil {
		t.Error("問い: options[1].Label が通ってしまう")
	}
	if err := Decode([]byte(`{"commit":"`+sha+`","findings":[],"Findings":null}`), &Findings{}); err == nil {
		t.Error("findings.json: Findings: null が通ってしまう")
	}
}

// ADR-0008 決めたこと 2・7: next[].to と問いの from の値は issue。古い task は読む互換を持たず落ちる。
func TestIssueNotTask(t *testing.T) {
	r := `{"commit":"` + sha + `","next":[{"to":"issue","text":"x"}]}`
	if _, err := ParseReport([]byte(r)); err != nil {
		t.Errorf("next[].to: issue が通らない: %v", err)
	}
	if _, err := ParseReport([]byte(strings.Replace(r, `"issue"`, `"task"`, 1))); err == nil {
		t.Error("next[].to: task が通ってしまう")
	}
	q := `{"kind":"scope","question":"q","options":[{"label":"a","pick":true},{"label":"b"}],"from":"issue"}`
	if _, err := ParseQuestion([]byte(q)); err != nil {
		t.Errorf("from: issue が通らない: %v", err)
	}
	if _, err := ParseQuestion([]byte(strings.Replace(q, `"issue"`, `"task"`, 1))); err == nil {
		t.Error("from: task が通ってしまう")
	}
}

// ADR-0011 決めたこと 7: observation は3つとも必須で、result は hit/miss/unknown だけ。unknown は証拠が空でも通る。
func TestReportObservation(t *testing.T) {
	for _, res := range []string{"hit", "miss", "unknown"} {
		ok := `{"observation":{"hypothesis":"h","result":"` + res + `","evidence":["signal:abc"]}}`
		if _, err := ParseReport([]byte(ok)); err != nil {
			t.Errorf("observation %s が通らない: %v", res, err)
		}
	}
	if _, err := ParseReport([]byte(`{"observation":{"hypothesis":"h","result":"unknown","evidence":[]}}`)); err != nil {
		t.Errorf("証拠が空の unknown が通らない: %v", err)
	}
}
