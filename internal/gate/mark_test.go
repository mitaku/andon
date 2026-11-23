package gate

import (
	"strings"
	"testing"

	"andon/internal/ledger"
)

// TestWantMark は ADR-0056 受け入れ条件 A2: 判定ごとの印が決めたこと 3 の表のとおりになる。
// approve と needs_human は success、fix は failure、wait は書かない、止めていて渡しているときは pending、
// 止めていて渡していないときは書かない、doing で前の印が success のときは failure、同じ状態と説明文の印が既にあるときは書かない。
func TestWantMark(t *testing.T) {
	human := []string{"delegation: merge=B < tier C"}
	fix := []string{"review missing: findings.json が無い"}
	green := Mark{"success", "andon gate: メンテナーの判断が要る変更です（自動でマージしてよい範囲を超える変更）"}
	for _, c := range []struct {
		name  string
		f     MarkFacts
		write bool
		state string
		desc  string // 説明文（空なら見ない）
	}{
		{"approve は success", MarkFacts{Status: "review", Decision: "approve"}, true, "success", "andon gate: 確かめが通った変更です"},
		{"needs_human は success（理由を括弧で持つ）", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human}, true, "success", green.Description},
		{"needs_human は前の赤を success に直す", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human, Have: Mark{"failure", "andon gate: メンテナーの判断が要る変更です（メンテナーが見る変更）"}}, true, "success", green.Description},
		{"fix は failure", MarkFacts{Status: "review", Decision: "fix", Reasons: fix}, true, "failure", "andon gate: 作った側が直してから出し直す変更です（レビューがまだ無い）"},
		{"wait は書かない", MarkFacts{Status: "review", Decision: "wait"}, false, "", ""},
		{"判定の前は書かない", MarkFacts{Status: "review"}, false, "", ""},
		{"止めていて渡している（判定が通した）ときは pending", MarkFacts{Status: "review", Decision: "approve", Held: true}, true, "pending", "andon gate: メンテナーの確認を待っています"},
		{"止めていて渡している（今の印が緑）ときは pending", MarkFacts{Status: "review", Held: true, Have: green}, true, "pending", ""},
		{"止めていて渡していないときは書かない", MarkFacts{Status: "review", Decision: "fix", Reasons: fix, Held: true}, false, "", ""},
		{"止めていて判定の前は書かない", MarkFacts{Status: "review", Held: true}, false, "", ""},
		{"doing で前の印が success なら failure", MarkFacts{Status: "doing", Decision: "needs_human", Have: green}, true, "failure", "andon gate: 作った側が直しています"},
		{"doing で前の印が failure なら書かない", MarkFacts{Status: "doing", Have: Mark{"failure", "x"}}, false, "", ""},
		{"doing で印が無ければ書かない", MarkFacts{Status: "doing"}, false, "", ""},
		{"doing で止めていて緑なら pending", MarkFacts{Status: "doing", Held: true, Have: green}, true, "pending", ""},
		{"同じ状態と説明文の印が既にあれば書かない", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human, Have: green}, false, "", ""},
		{"説明文が違えば書き直す", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human, Have: Mark{"success", "andon gate: 確かめが通った変更です"}}, true, "success", green.Description},
		{"説明文を読めていなければ状態だけで比べる", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human, Have: Mark{State: "success"}, HaveDescUnknown: true}, false, "", ""},
		{"説明文を読めていなくても状態が違えば書く", MarkFacts{Status: "review", Decision: "needs_human", Reasons: human, Have: Mark{State: "failure"}, HaveDescUnknown: true}, true, "success", ""},
		{"CE の文書の PR が review なら success", MarkFacts{Status: "review", Engineer: true}, true, "success", "andon gate: 仕様・設計の文書です。メンテナーがマージします"},
		{"CE の文書の PR が doing で緑なら failure", MarkFacts{Status: "doing", Engineer: true, Have: Mark{"success", documentDescription}}, true, "failure", ""},
		{"CE の文書の PR を止めたら pending", MarkFacts{Status: "review", Engineer: true, Held: true}, true, "pending", ""},
		{"review でも doing でもなければ書かない", MarkFacts{Status: "done", Decision: "approve"}, false, "", ""},
	} {
		m, w := WantMark(c.f)
		if w != c.write || m.State != c.state || (c.desc != "" && m.Description != c.desc) {
			t.Errorf("%s: %v %+v want %v %s %q", c.name, w, m, c.write, c.state, c.desc)
		}
	}
}

// TestDescribe は ADR-0054 受け入れ条件 A2（ADR-0056 決めたこと 3 の細部）: 説明文は見出しと最初の理由。
func TestDescribe(t *testing.T) {
	if got, want := describe("needs_human", []string{"delegation: merge=B < tier C"}), "andon gate: メンテナーの判断が要る変更です（自動でマージしてよい範囲を超える変更）"; got != want {
		t.Errorf("理由が 1 件: %q want %q", got, want)
	}
	if got, want := describe("fix", []string{"GitHub の checks が落ちた: tests", "review missing: x", "受け入れ条件の fail が 1 件"}), "andon gate: 作った側が直してから出し直す変更です（CI が落ちた: tests、ほか 2 件）"; got != want {
		t.Errorf("理由が 3 件: %q want %q", got, want)
	}
	if got, want := describe("approve", nil), "andon gate: 確かめが通った変更です"; got != want {
		t.Errorf("approve は今と同じ: %q want %q", got, want)
	}
	long := []string{
		"PR の branch（" + strings.Repeat("feature/very-long-branch-name-", 10) + "）が issue.yaml の branch と違う",
		"check: " + strings.Repeat("bash tests/run --changed && ", 12),
	}
	for _, rs := range [][]string{long, long[1:], append(long, "conflict")} {
		for _, d := range []string{"fix", "needs_human"} {
			s := describe(d, rs)
			if n := len([]rune(s)); n > descriptionLimit {
				t.Errorf("%s: 140 字を超える（%d）: %q", d, n, s)
			}
			if !strings.HasSuffix(s, "）") || !strings.Contains(s, "…") {
				t.Errorf("%s: 理由を「…」で切っていない: %q", d, s)
			}
			if len(rs) > 1 && !strings.Contains(s, "ほか "+string(rune('0'+len(rs)-1))+" 件") {
				t.Errorf("%s: 切っても「ほか N 件」が残らない: %q", d, s)
			}
		}
	}
	// 印の文はどれも外の人が読む文（内輪の語・役の語が無い）
	for _, s := range []string{heldDescription, reworkDescription, documentDescription, describe("needs_human", []string{"conflict"})} {
		if !strings.HasPrefix(s, CommentPrefix) {
			t.Errorf("頭が andon gate: でない: %q", s)
		}
		if m := insideWords.FindString(s); m != "" {
			t.Errorf("内輪の語 %q: %q", m, s)
		}
		if m := roleWords.FindString(s); m != "" {
			t.Errorf("役の語 %q: %q", m, s)
		}
	}
	if insideWords.FindString("andon gate: 担当が直しています") == "" {
		t.Fatal("内輪の語の検査が「担当」に当たらない（上の検査が何も見ていない）")
	}
}

// TestGateMark は statusCheckRollup から gate 自身の最新の印を読む（ほかの status と CheckRun は読まない）。
func TestGateMark(t *testing.T) {
	items := []CheckItem{
		{Typename: "CheckRun", Name: "andon/gate", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Typename: "StatusContext", Context: "ci/x", State: "SUCCESS", Description: "ci"},
		{Typename: "StatusContext", Context: GateContext, State: "FAILURE", Description: "old", CreatedAt: "2026-11-23T00:00:00Z"},
		{Typename: "StatusContext", Context: GateContext, State: "SUCCESS", Description: "new", CreatedAt: "2026-11-23T00:01:00Z"},
	}
	if got := GateMark(items); got != (Mark{"success", "new"}) {
		t.Errorf("GateMark = %+v", got)
	}
	if got := GateMark(items[:2]); got != (Mark{}) {
		t.Errorf("gate の印が無いのに読んだ: %+v", got)
	}
}

// Final は、checks で待たないときだけ条件表の検査を回させる（ADR-0056 決めたこと 4。反論の段で見つけた穴）
func TestChecksFinal(t *testing.T) {
	for _, c := range []struct {
		state             string
		optional, overdue bool
		want              bool
	}{
		{"success", false, false, true},
		{"unknown", false, false, true},
		{"", false, false, true},
		{"none", false, true, true},
		{"none", true, false, true},
		{"none", false, false, false},
		{"pending", false, true, false},
		{"failure", false, false, false},
	} {
		if got := (GitHubChecks{State: c.state}).Final(c.optional, c.overdue); got != c.want {
			t.Errorf("Final(%q, optional=%v, overdue=%v) = %v, want %v", c.state, c.optional, c.overdue, got, c.want)
		}
	}
}

// JudgedThisRound は、同じ head のまま review に入り直した回の前の判定を使わせない（反論の段で見つけた穴）
func TestJudgedThisRound(t *testing.T) {
	row := func(id, at, status string) ledger.Row {
		return ledger.Row{ID: id, At: at, Change: map[string]any{"status": status}}
	}
	rows := []ledger.Row{
		row("A-0001", "2026-11-20T00:00:00Z", "review"),
		row("A-0001", "2026-11-20T01:00:00Z", "doing"),
		row("A-0002", "2026-11-20T03:00:00Z", "review"),
		row("A-0001", "2026-11-20T02:00:00Z", "review"),
	}
	for _, c := range []struct {
		id, at string
		want   bool
	}{
		{"A-0001", "2026-11-20T00:30:00Z", false}, // 前の回の判定
		{"A-0001", "2026-11-20T02:00:00Z", false}, // 入った秒と同じ（前の回の記録と見分けられない）
		{"A-0001", "2026-11-20T02:10:00Z", true},
		{"A-0001", "壊れた時刻", false},
		{"A-0009", "2026-11-20T00:00:00Z", true}, // review の行が無い
	} {
		if got := JudgedThisRound(rows, c.id, c.at); got != c.want {
			t.Errorf("JudgedThisRound(%s, %s) = %v, want %v", c.id, c.at, got, c.want)
		}
	}
}

// GateLink は gate 自身の印のリンクを読む（前のバージョンのリンクの無い印を書き直すため。ADR-0054 決めたこと 6）
func TestGateLink(t *testing.T) {
	items := []CheckItem{
		{Typename: "StatusContext", Context: "ci/other", State: "SUCCESS", TargetURL: "https://example.com/other"},
		{Typename: "StatusContext", Context: GateContext, State: "SUCCESS", CreatedAt: "2026-11-20T00:00:00Z", TargetURL: "https://example.com/old"},
		{Typename: "StatusContext", Context: GateContext, State: "SUCCESS", CreatedAt: "2026-11-20T01:00:00Z"},
	}
	if got := GateLink(items); got != "" {
		t.Errorf("最新の印にリンクが無いのに %q を返した", got)
	}
	if got := GateLink(items[:2]); got != "https://example.com/old" {
		t.Errorf("GateLink = %q", got)
	}
}
