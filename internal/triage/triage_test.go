package triage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
	// skill が書く新しい見出しも、古い見出しと同じに拾う（ADR-0036 決めたこと 6）
	if got := reportLines([]byte("## オーナーに聞くこと\n- 新しい問い\n")); !reflect.DeepEqual(got, []reportLine{{"question", "- 新しい問い"}}) {
		t.Errorf("新しい見出し: got %q", got)
	}
}

func TestPersonTitle(t *testing.T) {
	for title, want := range map[string]bool{
		"端末で確認（人の判断待ち）":    true,
		"端末で確認（オーナーの判断待ち）": true,
		"端末で確認": false,
		"（オーナーの判断待ち）の続き": false,
	} {
		if got := personTitle(title); got != want {
			t.Errorf("personTitle(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestReportLinesSkipsPlanDiff(t *testing.T) {
	body := "## Report\n" +
		"計画との差: Report の節は元から拾わない\n" +
		"## Remember\n" +
		"- 学び\n" +
		"計画との差: 差なし。見送りは無い\n" +
		"- 計画との差: 箇条書きでも\n" +
		"  計画との差: 字下げでも\n" +
		"- 学び 2\n" +
		"## Next\n" +
		"- 計画との差: Next の節では拾う"
	want := []reportLine{
		{"remember", "- 学び"},
		{"remember", "- 学び 2"},
		{"next", "- 計画との差: Next の節では拾う"},
	}
	if got := reportLines([]byte(body)); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
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
		"issue": map[string]any{"title": title, "repo": "example.invalid/me/app", "profile": "claude", "review": "on", "brief": "解決すべき問題"}}
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
	withIssue := func(k string, v any) map[string]any {
		f := fix(k1, "t")
		f["issue"].(map[string]any)[k] = v
		return f
	}
	bad := map[string][]map[string]any{
		"空白の理由":            {with(fix(k1, "t"), "reason", " ")},
		"改行の理由":            {with(fix(k1, "t"), "reason", "a\nb")},
		"タイトルが空":           {withIssue("title", "")},
		"タイトルが - で始まる":     {withIssue("title", "-x")},
		"repo の形":          {withIssue("repo", "me/app")},
		"profile の形":       {withIssue("profile", "-x")},
		"review の値":        {withIssue("review", "yes")},
		"ほかの kanban の key": {with(fix(k1, "t"), "key", "other:T-1:next:0000000000000000000000000000000000000001")},
		"hash の無い key":     {with(fix(k1, "t"), "key", "demo:T-1:issue")},
		"知らない kind":        {with(fix(k1, "t"), "key", "demo:T-1:bogus:0000000000000000000000000000000000000001")},
		"知らない decision":    {with(fix(k1, "t"), "decision", "drop")},
		"同じ key":           {fix(k1, "a"), fix(k1, "b")},
		"bundle の名前":       {with(fix(k1, "t"), "decision", "bundle")},
		"repo が混ざる bundle": {
			with(with(fix(k1, "t"), "decision", "bundle"), "bundle", "b"),
			func() map[string]any {
				f := with(with(fix(k2, "t"), "decision", "bundle"), "bundle", "b")
				f["issue"] = map[string]any{"title": "t", "repo": "example.invalid/other/repo", "profile": "claude", "review": "on", "brief": "x"}
				return f
			}(),
		},
		"commit の形":      {with(fix(k1, "t"), "commit", "HEAD")},
		"commit が短い":     {with(fix(k1, "t"), "commit", "5ccac6a")},
		"commit が文字列でない": {with(fix(k1, "t"), "commit", 1)},
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
	if !ok(with(fix(k1, "t"), "commit", sha1a)) || !ok(with(fix(k1, "t"), "commit", nil)) {
		t.Error("commit が 40 桁か null の判定が通らない")
	}
}

const (
	sha1a = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha1b = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// 判定の commit は collect の出したものと同じでなければ通らない（ADR-0030 受け入れ条件 6）。
func TestCheckCommits(t *testing.T) {
	mem := "demo:note:memory:0000000000000000000000000000000000000003"
	old := "demo:T-9:remember:0000000000000000000000000000000000000009" // 前回までの記録の key（見直し）。今回の項目に同じ repo が無い
	bare := "demo:T-8:next:0000000000000000000000000000000000000008"    // repo の無い課題
	commits := map[string]string{"example.invalid/me/app": sha1a, "example.invalid/me/lib": sha1b}
	repos := map[string]string{"T-1": "example.invalid/me/app", "T-9": "example.invalid/me/lib", "T-7": "example.invalid/me/other", "T-8": ""}
	it := func(key string, commit any) item {
		i := item{"key": key, "decision": "close", "reason": "r"}
		if commit != nil {
			i["commit"] = commit
		}
		return i
	}
	cases := []struct {
		name string
		item item
		ok   bool
	}{
		{"collect の出したもの", it(k1, sha1a), true},
		{"古い commit", it(k1, sha1b), false},
		{"commit が無い", it(k1, nil), false},
		{"memory は持たない", it(mem, nil), true},
		{"memory に commit", it(mem, sha1a), false},
		{"見直しは repo の commit", it(old, sha1b), true},
		{"見直しの古い commit", it(old, sha1a), false},
		{"repo の無い課題", it(bare, nil), true},
		{"collect が取っていない repo", it("demo:T-7:next:0000000000000000000000000000000000000007", sha1a), false},
	}
	for _, c := range cases {
		err := checkCommits([]item{c.item}, commits, repos)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v", c.name, err)
		}
	}
	// collect の記録に commits が無ければ（前の形）、commit の無い判定でも通らない。
	if checkCommits([]item{it(k1, nil)}, map[string]string{}, repos) == nil {
		t.Error("commits の無い記録で、commit の無い判定が通った")
	}
}

// 前の形（項目の配列）・commits の無い・形の合わない collect の記録は読まない。
func TestReadCollected(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"前の形":         `[{"key":"k","kind":"next","from":"T-1","repo":"r","text":"t","age_days":0}]`,
		"commits が無い": `{"items":[]}`,
		"commit の形":   `{"commits":{"example.invalid/me/app":"HEAD"},"items":[]}`,
	} {
		p := dir + "/" + name
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readCollected(p); err == nil {
			t.Errorf("%s が通った", name)
		}
	}
	p := dir + "/ok"
	_ = os.WriteFile(p, []byte(`{"commits":{"example.invalid/me/app":"`+sha1a+`"},"items":[]}`), 0o644)
	if got, err := readCollected(p); err != nil || got["example.invalid/me/app"] != sha1a {
		t.Errorf("今の形が読めない: %v %v", got, err)
	}
}

func TestCapIssuesCountsBundleOnce(t *testing.T) {
	b := func(key string) item {
		f := item(fix(key, "t"))
		f["decision"], f["bundle"] = "bundle", "docs"
		return f
	}
	k3 := "demo:T-3:next:0000000000000000000000000000000000000003"
	k4 := "demo:T-4:next:0000000000000000000000000000000000000004"
	in := []item{b(k1), item(fix(k2, "a")), b(k3), item(fix(k4, "b")), {"key": "x", "decision": "close", "reason": "r"}}
	got := capIssues(in, 2)
	var decisions []string
	for _, it := range got {
		decisions = append(decisions, it.decision())
	}
	if want := []string{"bundle", "fix", "bundle", "keep", "close"}; !reflect.DeepEqual(decisions, want) {
		t.Errorf("got %v want %v", decisions, want)
	}
	if got[3].str("reason") != "次回（起動する上限）: 直せる" || in[3].decision() != "fix" {
		t.Errorf("keep の理由・元を書き換えない: %v / %v", got[3], in[3])
	}
	groups := applyGroups(got)
	if len(groups) != 4 || len(groups[0]) != 2 || groups[1][0].key() != k2 {
		t.Errorf("groups = %v", groups)
	}
}

func TestSummary(t *testing.T) {
	items := []item{
		{"key": k1, "decision": "close", "closing": "done", "reason": "a|b", "text": "行\n本文"},
		{"key": "demo:T-4:remember:0000000000000000000000000000000000000004", "decision": "close", "closing": "one-off", "reason": "その場限り", "text": "学び"},
		{"key": k2, "decision": "fix", "reason": "r", "created_issue": "T-9", "issue": map[string]any{"title": "タイトル", "review": "on"}},
		{"key": "demo:T-3:unreadable:0000000000000000000000000000000000000003", "decision": "keep", "reason": "読む"},
	}
	// 種類を決めきれなかった判断（unsure）が 4 件と、手作業が 1 件（ADR-0057 受け入れ条件 12）。
	for i := range 4 {
		items = append(items, item{"key": "demo:T-5:question:" + strings.Repeat("0", 39) + string(rune('a'+i)), "decision": "person", "reason": "r",
			"question": "q", "kind": "line", "unsure": true, "options": []any{"推す案", "ほかの案"}, "created_issue": "T-1" + string(rune('0'+i))})
	}
	items = append(items, item{"key": "demo:T-6:person:" + strings.Repeat("0", 40), "decision": "person", "reason": "r",
		"question": "実機で確かめる（今週中）", "only_owner": "実機はオーナーの手元にだけある", "created_issue": "T-20"})
	got := summary("demo", "2026-01-02", "ゴール", items)
	want := "線引きを見直す\n\n# トリアージ demo 2026-01-02（ゴール: ゴール）\n" +
		"閉じた 2 ・ 直す 1（うち束ね 0） ・ オーナーへ 5（判断 4・手作業 1・unsure 4） ・ 次回 1 ・ 読めない 1\n" +
		"\n## 閉じた（判断の閉じと課題の閉じ。Chief of Staff が読んだ）\n| 出どころ | 項目 | 閉じ方 | 理由 |\n|---|---|---|---|\n" +
		"| demo:T-4:remember:0000000000000000000000000000000000000004 | 学び | one-off | その場限り |\n" +
		"\n## 閉じた（根拠の閉じ）\n| 出どころ | 項目 | 閉じ方 | 理由 |\n|---|---|---|---|\n" +
		"| " + k1 + " | 行 本文 | done | a&#124;b |\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("got:\n%s", got)
	}
	for _, line := range []string{
		"| T-9 | タイトル | " + k2 + " | on |\n",
		"| T-10 | q | line（unsure） | 推す案 |\n",
		"| T-20 | 実機で確かめる（今週中） | 手作業 | 実機はオーナーの手元にだけある |\n",
		"| demo:T-3:unreadable:0000000000000000000000000000000000000003 | demo:T-3:unreadable:0000000000000000000000000000000000000003 | 読む |\n",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("%q が無い:\n%s", line, got)
		}
	}
	if strings.Contains(got, "覆した") {
		t.Errorf("覆したものが無いのに覆した表がある:\n%s", got)
	}
	items[2]["overturned"] = obj{"closing": "one-off", "reason": "前の理由"}
	if got := summary("demo", "d", "g", items); !strings.Contains(got, "| "+k2+" | "+k2+" | one-off: 前の理由 | fix |\n") {
		t.Errorf("覆した表が無い:\n%s", got)
	}
	if s := summary("demo", "d", "g", nil); !strings.HasPrefix(s, "\n# トリアージ") {
		t.Errorf("unsure が無ければ先頭は空行: %q", s)
	}
	// unsure が 3 件なら「線引きを見直す」を出さない。種類を決めた判断と手作業は数えない。
	items[5]["unsure"] = false
	if s := summary("demo", "d", "g", items); !strings.HasPrefix(s, "\n# トリアージ") || !strings.Contains(s, "オーナーへ 5（判断 4・手作業 1・unsure 3）") {
		t.Errorf("unsure が 3 件なら先頭は空行で、内訳は 3:\n%s", s)
	}
}

func TestMainUsage(t *testing.T) {
	for _, args := range [][]string{{"apply"}, {"collect", "--limit"}, {"collect", "--limit", "x"},
		{"collect", "--limit", "1234567"}, {"collect", "--max-issues", "1"}, {"apply", "T-1", "--limit", "1"}, {"collect", "--dry-run"},
		{"verdict"}, {"verdict", "T-1"}, {"verdict", "T-1", "k"}, {"verdict", "T-1", "k", "ok", "extra"}} {
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

// slug は schema の project と同じ（_ を含めてよい）。項目の key も同じ slug を受ける。
func TestSlugFollowsSchema(t *testing.T) {
	if !slugRe.MatchString("sample_site") || slugRe.MatchString("_x") {
		t.Error("slugRe が schema の project と違う")
	}
	k := itemKey("sample_site", "S-0001", "next", "x")
	if !keyRe.MatchString(k) {
		t.Errorf("key %q を受けない", k)
	}
}

func key(kind string, n int) string {
	return fmt.Sprintf("demo:T-%d:%s:%040d", n, kind, n)
}

// 閉じ方の欄が無い close と、組み合わせに合わない判断の閉じは通らない（ADR-0030 受け入れ条件 1・2）。
func TestCheckClosings(t *testing.T) {
	c := func(kind, closing string) item {
		it := item{"key": key(kind, 1), "decision": "close", "reason": "r"}
		if closing != "" {
			it["closing"] = closing
		}
		return it
	}
	good := []item{
		c("waived", "wont-happen"), c("remember", "one-off"), c("memory", "one-off"), c("memory", "not-applicable"),
		c("unreadable", "read"), c("next", "done"), c("next", "gone"), c("issue", "done"), c("person", "gone"),
		c("signal", "gone"), c("remember", "wiki"), c("remember", "delegated"), c("leftover", "gone"),
		{"key": key("next", 1), "decision": "keep", "reason": "r"},
	}
	for _, it := range good {
		if err := checkClosings([]item{it}); err != nil {
			t.Errorf("%v が通らない: %v", it, err)
		}
	}
	bad := map[string]item{
		"欄が無い":              c("next", ""),
		"null":              {"key": key("next", 1), "decision": "close", "reason": "r", "closing": nil},
		"知らない値":             c("next", "drop"),
		"next を起きない":        c("next", "wont-happen"),
		"remember を起きない":    c("remember", "wont-happen"),
		"issue をその場限り":      c("issue", "one-off"),
		"person を読んだ":       c("person", "read"),
		"next をその場限り":       c("next", "one-off"),
		"question を当てはまらない": c("question", "not-applicable"),
		"agents をその場限り":     c("agents", "one-off"),
		"signal をその場限り":     c("signal", "one-off"),
		"leftover をその場限り":   c("leftover", "one-off"),
		"remember を当てはまらない": c("remember", "not-applicable"),
		"fix に知らない閉じ方":      {"key": key("next", 1), "decision": "fix", "reason": "r", "closing": "drop"},
	}
	for name, it := range bad {
		if err := checkClosings([]item{it}); err == nil {
			t.Errorf("%s が通った", name)
		}
	}
}

// 判定の要る close は、判断の閉じと、課題を閉じる判定（根拠の閉じでも）。
func TestNeedsVerdict(t *testing.T) {
	for _, tc := range []struct {
		it   item
		want bool
	}{
		{item{"key": key("remember", 1), "decision": "close", "closing": "one-off"}, true},
		{item{"key": key("issue", 1), "decision": "close", "closing": "done"}, true},
		{item{"key": key("person", 1), "decision": "close", "closing": "gone"}, true},
		{item{"key": key("next", 1), "decision": "close", "closing": "done"}, false},
		{item{"key": key("memory", 1), "decision": "close", "closing": "wiki"}, false},
		{item{"key": key("issue", 1), "decision": "keep"}, false},
	} {
		if got := tc.it.needsVerdict(); got != tc.want {
			t.Errorf("%v: %v", tc.it, got)
		}
	}
}

// 判定が揃わない・覆した・理由が書き換わったら断り、揃えば判定と覆した記録を項目に書く（受け入れ条件 9〜11）。
func TestCheckVerdicts(t *testing.T) {
	a := item{"key": key("remember", 1), "decision": "close", "closing": "one-off", "reason": "ra"}
	b := item{"key": key("issue", 2), "decision": "close", "closing": "done", "reason": "rb"}
	n := item{"key": key("next", 3), "decision": "close", "closing": "done", "reason": "rn"}
	f := item{"key": key("remember", 4), "decision": "fix", "reason": "rf"}
	ok := func(it item) verdict {
		return verdict{Key: it.key(), Verdict: verdictOK, Closing: it.closing(), Reason: it.str("reason"), By: "terminal:t1", At: "x"}
	}
	vs := func(v ...verdict) map[string]verdict {
		m := map[string]verdict{}
		for _, x := range v {
			m[x.Key] = x
		}
		return m
	}
	items := []item{a, b, n, f}
	if err := checkVerdicts(items, vs(), "terminal:t1"); err == nil || !strings.Contains(err.Error(), "2 件足りない") {
		t.Errorf("判定なし: %v", err)
	}
	if err := checkVerdicts(items, vs(ok(a)), "terminal:t1"); err == nil || !strings.Contains(err.Error(), "1 件足りない") {
		t.Errorf("1 件だけ: %v", err)
	}
	changed := ok(b)
	changed.Reason = "前の理由"
	if err := checkVerdicts(items, vs(ok(a), changed), "terminal:t1"); err == nil {
		t.Error("理由が書き換わった判定が通った")
	}
	over := ok(b)
	over.Verdict = verdictOverturn
	if err := checkVerdicts(items, vs(ok(a), over), "terminal:t1"); err == nil || !strings.Contains(err.Error(), "覆した") {
		t.Errorf("覆した: %v", err)
	}
	st := func(v map[string]verdict, it item) string { return verdictState(v, "terminal:t1", it) }
	if st(vs(ok(a), over), a) != "ok" || st(vs(over), b) != "overturn" || st(vs(changed), b) != "stale" || st(vs(), a) != "none" {
		t.Error("verdictState")
	}
	// 書き手が今の Chief of Staff の端末でない判定（by の無いもの・別の端末）は数えない。chief_of_staff が無ければどれも数えない。
	noBy, other := ok(a), ok(a)
	noBy.By, other.By = "", "terminal:t9"
	for _, v := range []verdict{noBy, other} {
		if st(vs(v), a) != "foreign" {
			t.Errorf("%+v: %s", v, st(vs(v), a))
		}
		if err := checkVerdicts([]item{a}, vs(v), "terminal:t1"); err == nil {
			t.Errorf("%+v が通った", v)
		}
	}
	if verdictState(vs(ok(a)), "", a) != "foreign" || checkVerdicts([]item{a}, vs(ok(a)), "") == nil {
		t.Error("chief_of_staff が無いのに判定が通った")
	}
	fo := verdict{Key: f.key(), Verdict: verdictOverturn, Closing: "one-off", Reason: "前", By: "terminal:t1", At: "x"}
	if err := checkVerdicts(items, vs(ok(a), ok(b), fo), "terminal:t1"); err != nil {
		t.Fatal(err)
	}
	if a["verdict"].(obj)["verdict"] != "ok" || b["verdict"].(obj)["by"] != "terminal:t1" || n["verdict"] != nil {
		t.Errorf("判定が項目に無い: %v %v %v", a, b, n)
	}
	if o, _ := f["overturned"].(obj); o["reason"] != "前" || o["closing"] != "one-off" {
		t.Errorf("覆した記録が無い: %v", f)
	}
}

// 束ねた項目が同じ brief を写していても、依頼書の「解決すべき問題」の段落は1回だけ書く。違う brief は順に残す。
func TestGroupBriefWritesEachBriefOnce(t *testing.T) {
	b := func(key, brief string) item {
		f := item(fix(key, "t"))
		f["decision"], f["bundle"] = "bundle", "docs"
		f["issue"].(map[string]any)["brief"] = brief
		return f
	}
	k3 := "demo:T-3:next:0000000000000000000000000000000000000003"
	got := groupBrief([]item{b(k1, "同じ段落。"), b(k2, "同じ段落。"), b(k3, "同じ段落。")}, nil)
	if n := strings.Count(got, "同じ段落。"); n != 1 {
		t.Errorf("同じ段落が %d 回:\n%s", n, got)
	}
	if !strings.Contains(got, "なぜ: トリアージ "+k1+", "+k2+", "+k3+"\n") {
		t.Errorf("key を全部書かない:\n%s", got)
	}
	got = groupBrief([]item{b(k1, "一つ目。"), b(k2, "二つ目。"), b(k3, "一つ目。")}, nil)
	if !strings.HasPrefix(got, "解決すべき問題:\n一つ目。\n\n二つ目。\nなぜ:") {
		t.Errorf("違う段落を順に残さない:\n%s", got)
	}
}
