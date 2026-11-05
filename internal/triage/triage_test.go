package triage

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReportLines(t *testing.T) {
	body := "## Report\n" +
		"見送り（minor）:\n" +
		"- a の修正\n" +
		"* b の修正\n" +
		"本文\n" +
		"- 外の箇条\n" +
		"\n" +
		"## Next\n" +
		"- 次\n" +
		"# 見出しは拾わない\n" +
		"## Remember\n" +
		"- 学び\r\n" +
		"For AGENTS.md: 規則\n" +
		"## 人に聞くこと\n" +
		"- 問い"
	want := []reportLine{
		{"waived", "見送り（minor）:"},
		{"waived", "- a の修正"},
		{"waived", "* b の修正"},
		{"next", "- 次"},
		{"remember", "- 学び\r"},
		{"agents", "For AGENTS.md: 規則"},
		{"question", "- 問い"},
	}
	if got := reportLines([]byte(body)); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if hasHeading([]byte("見出しなし\n#  二つ目\n")) || !hasHeading([]byte("x\n## a\n")) {
		t.Error("hasHeading")
	}
}

func TestItemKeyMatchesShellSha1(t *testing.T) {
	// printf '%s' 'abc' | sha1sum
	if got := itemKey("demo", "T-1", "next", "abc"); got != "demo:T-1:next:a9993e364706816aba3e25717850c26c9cd0d89d" {
		t.Error(got)
	}
}

func TestAgeDays(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	cases := map[string]int64{
		"2026-01-01T12:00:00Z":     9,
		"2026-01-01T12:00:01Z":     8,
		"2026-02-01T00:00:00Z":     0,
		"":                         0,
		"2026-01-01T12:00:00.5Z":   0,
		"2026-01-01T12:00:00+0900": 0,
	}
	for in, want := range cases {
		if got := ageDays(in, now); got != want {
			t.Errorf("%q: %d, want %d", in, got, want)
		}
	}
}

const k1 = "demo:T-1:next:0000000000000000000000000000000000000001"
const k2 = "demo:T-2:next:0000000000000000000000000000000000000002"

func fix(key, title string) map[string]any {
	return map[string]any{"key": key, "decision": "fix", "reason": "直せる",
		"task": map[string]any{"title": title, "repo": "example.invalid/me/app", "profile": "claude", "review": "on", "brief": "やること"}}
}

func decisions(items ...map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"goal": "g", "collected": "c", "items": items})
	return b
}

func TestParseDecisionsRejects(t *testing.T) {
	ok := func(items ...map[string]any) bool { _, _, ok := parseDecisions(decisions(items...), "demo"); return ok }
	if !ok(fix(k1, "t")) {
		t.Fatal("正しいものが通らない")
	}
	with := func(m map[string]any, k string, v any) map[string]any {
		c := map[string]any{}
		for kk, vv := range m {
			c[kk] = vv
		}
		c[k] = v
		return c
	}
	withTask := func(k string, v any) map[string]any {
		f := fix(k1, "t")
		f["task"].(map[string]any)[k] = v
		return f
	}
	bad := map[string][]map[string]any{
		"空白の理由":         {with(fix(k1, "t"), "reason", " ")},
		"改行の理由":         {with(fix(k1, "t"), "reason", "a\nb")},
		"題が空":           {withTask("title", "")},
		"題が - で始まる":     {withTask("title", "-x")},
		"repo の形":       {withTask("repo", "me/app")},
		"profile の形":    {withTask("profile", "-x")},
		"review の値":     {withTask("review", "yes")},
		"ほかの台帳の key":    {with(fix(k1, "t"), "key", "other:T-1:next:0000000000000000000000000000000000000001")},
		"hash の無い key":  {with(fix(k1, "t"), "key", "demo:T-1:task")},
		"知らない kind":     {with(fix(k1, "t"), "key", "demo:T-1:bogus:0000000000000000000000000000000000000001")},
		"知らない decision": {with(fix(k1, "t"), "decision", "drop")},
		"同じ key":        {fix(k1, "a"), fix(k1, "b")},
		"bundle の名前":    {with(fix(k1, "t"), "decision", "bundle")},
		"repo が混ざる bundle": {
			with(with(fix(k1, "t"), "decision", "bundle"), "bundle", "b"),
			func() map[string]any {
				f := with(with(fix(k2, "t"), "decision", "bundle"), "bundle", "b")
				f["task"] = map[string]any{"title": "t", "repo": "example.invalid/other/repo", "profile": "claude", "review": "on", "brief": "x"}
				return f
			}(),
		},
		"問いが空":         {{"key": k1, "decision": "person", "reason": "r", "question": ""}},
		"kind に推しが無い":  {{"key": k1, "decision": "person", "reason": "r", "question": "q", "kind": "promise"}},
		"知らない kind の人": {{"key": k1, "decision": "person", "reason": "r", "question": "q", "kind": "other", "pick": "p"}},
	}
	for name, items := range bad {
		if ok(items...) {
			t.Errorf("%s が通った", name)
		}
	}
	for _, raw := range []string{`[]`, `{"goal":"","collected":"c","items":[]}`, `{"goal":"g","collected":"c"}`, `{"goal":"g","collected":"c","items":[]} {}`} {
		if _, _, ok := parseDecisions([]byte(raw), "demo"); ok {
			t.Errorf("%s が通った", raw)
		}
	}
	if !ok(map[string]any{"key": k1, "decision": "person", "reason": "r", "question": "q", "kind": nil}) {
		t.Error("kind が null の person が通らない")
	}
}

func TestCapTasksCountsBundleOnce(t *testing.T) {
	b := func(key string) item {
		f := item(fix(key, "t"))
		f["decision"], f["bundle"] = "bundle", "docs"
		return f
	}
	k3 := "demo:T-3:next:0000000000000000000000000000000000000003"
	k4 := "demo:T-4:next:0000000000000000000000000000000000000004"
	in := []item{b(k1), item(fix(k2, "a")), b(k3), item(fix(k4, "b")), {"key": "x", "decision": "close", "reason": "r"}}
	got := capTasks(in, 2)
	var decisions []string
	for _, it := range got {
		decisions = append(decisions, it.decision())
	}
	if want := []string{"bundle", "fix", "bundle", "keep", "close"}; !reflect.DeepEqual(decisions, want) {
		t.Errorf("got %v want %v", decisions, want)
	}
	if got[3].str("reason") != "次回（起こす上限）: 直せる" || in[3].decision() != "fix" {
		t.Errorf("keep の理由・元を書き換えない: %v / %v", got[3], in[3])
	}
	groups := applyGroups(got)
	if len(groups) != 4 || len(groups[0]) != 2 || groups[1][0].key() != k2 {
		t.Errorf("groups = %v", groups)
	}
}

func TestSummary(t *testing.T) {
	items := []item{
		{"key": k1, "decision": "close", "reason": "a|b", "text": "行\n本文"},
		{"key": k2, "decision": "fix", "reason": "r", "created_task": "T-9", "task": map[string]any{"title": "題", "review": "on"}},
		{"key": "demo:T-3:unreadable:0000000000000000000000000000000000000003", "decision": "keep", "reason": "読む"},
	}
	for i := range 4 {
		items = append(items, item{"key": "demo:T-5:question:" + strings.Repeat("0", 39) + string(rune('a'+i)), "decision": "person", "reason": "r", "question": "q", "created_task": "T-1" + string(rune('0'+i))})
	}
	got := summary("demo", "2026-01-02", "ゴール", items)
	want := "線引きを見直す\n\n# トリアージ demo 2026-01-02（ゴール: ゴール）\n" +
		"閉じた 1 ・ 直す 1（うち束ね 0） ・ 人へ 4 ・ 次回 1 ・ 読めない 1\n" +
		"\n## 閉じた\n| 出どころ | 項目 | 理由 |\n|---|---|---|\n" +
		"| " + k1 + " | 行 本文 | a&#124;b |\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("got:\n%s", got)
	}
	for _, line := range []string{
		"| T-9 | 題 | " + k2 + " | on |\n",
		"| T-10 | q | 判断がつかない | なし |\n",
		"| demo:T-3:unreadable:0000000000000000000000000000000000000003 | demo:T-3:unreadable:0000000000000000000000000000000000000003 | 読む |\n",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("%q が無い:\n%s", line, got)
		}
	}
	if s := summary("demo", "d", "g", nil); !strings.HasPrefix(s, "\n# トリアージ") {
		t.Errorf("人の kind 無しが3以下なら先頭は空行: %q", s)
	}
}

func TestMainUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"apply"}, {"collect", "--limit"}, {"collect", "--limit", "x"},
		{"collect", "--limit", "1234567"}, {"collect", "--max-tasks", "1"}, {"apply", "T-1", "--limit", "1"}, {"collect", "--dry-run"}} {
		var out, errb bytes.Buffer
		if rc := Main(args, nil, &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon triage") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if rc := Main([]string{"apply", "../x"}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "id が不正") {
		t.Errorf("bad id: rc=%d err=%q", rc, errb.String())
	}
}
