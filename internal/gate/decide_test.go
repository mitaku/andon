package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func pkgTable(t *testing.T) *Table {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tb, _, err := LoadTable(filepath.Join(root, "package"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestGlob(t *testing.T) {
	for _, c := range []struct {
		g, p string
		want bool
	}{
		{"docs/**", "docs/a/b.md", true},
		{"**/AGENTS.md", "AGENTS.md", true},
		{"**/AGENTS.md", "a/b/AGENTS.md", true},
		{"README*", "README.md", true},
		{"README*", "docs/README.md", false},
		{"compose*.y*ml", "compose.yaml", true},
		{"projects/*/delegation.json", "projects/x/delegation.json", true},
		{"projects/*/delegation.json", "projects/x/y/delegation.json", false},
		{"a.b", "axb", false},
		{"docs/機密/**", "docs/機密/a.md", true},
		{"docs/機密/**", "docs/機械/a.md", false},
		{"?.md", "機.md", true},
	} {
		if got := Glob(c.g, c.p); got != c.want {
			t.Errorf("Glob(%q, %q) = %v", c.g, c.p, got)
		}
	}
}

func TestClassify(t *testing.T) {
	tb := pkgTable(t)
	for _, c := range []struct {
		name     string
		files    []string
		diff     string
		review   bool
		override map[string]string
		want     string
	}{
		{"docs だけは A", []string{"docs/x.md"}, "", false, nil, "A"},
		{"internal は B", []string{"internal/x.go"}, "", false, nil, "B"},
		{"review: on は B", []string{"docs/x.md"}, "", true, nil, "B"},
		{"skills は C", []string{"package/skills/worker/SKILL.md"}, "", false, nil, "C"},
		{".claude/skills は C の例外（B）", []string{"x/.claude/skills/a.md"}, "", false, nil, "B"},
		{".claude/settings は C", []string{"x/.claude/settings.json"}, "", false, nil, "C"},
		{"追加行 sudo は C", []string{"docs/x.md"}, "+run sudo x\n", false, nil, "C"},
		{"削除行の sudo は数えない", []string{"docs/x.md"}, "-run sudo x\n", false, nil, "A"},
		{"tier_overrides で AGENTS.md を B", []string{"AGENTS.md"}, "", false, map[string]string{"AGENTS.md": "B"}, "B"},
		{"tier_overrides で docs を C", []string{"docs/x.md"}, "", false, map[string]string{"docs/**": "C"}, "C"},
		{"非 ASCII のパスの tier_overrides", []string{"docs/機密/a.md"}, "", false, map[string]string{"docs/機密/**": "C"}, "C"},
	} {
		var files []File
		for _, f := range c.files {
			files = append(files, File{Path: f, Additions: 1})
		}
		got, err := tb.Classify(RepoTable{TierOverrides: c.override}, files, c.diff, c.review)
		if err != nil || got.Tier != c.want {
			t.Errorf("%s: tier %s（%v）want %s", c.name, got.Tier, got.TierReasons, c.want)
		}
	}
	got, _ := tb.Classify(RepoTable{}, []File{{Path: "docs/x.md", Additions: 600}, {Path: "tests/a.sh", Additions: 5000}}, "", false)
	if got.Tier != "B" || got.ChangedLines != 600 {
		t.Errorf("行数は tests を除いて数え、size_review_lines 超は B: %+v", got)
	}
	got, _ = tb.Classify(RepoTable{}, []File{{Path: "docs/x.md", Additions: 1001}}, "", false)
	if got.Tier != "C" {
		t.Errorf("size_human_lines 超が C でない: %+v", got)
	}
}

func TestWithinAndRaise(t *testing.T) {
	if Within("A", "none") || !Within("B", "B") || Within("C", "B") || !Within("C", "C") {
		t.Error("Within")
	}
	if Raise("A") != "B" || Raise("B") != "C" || Raise("C") != "C" {
		t.Error("Raise")
	}
}

func TestFromReport(t *testing.T) {
	const sha = "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"
	base := `{"commit": "` + sha + `", "pr": "https://github.com/owner/andon/pull/1"`
	for _, c := range []struct {
		name, tier, extra, want string
		oneWayReason            bool
	}{
		{"one_way: true で A が C", "A", `, "risk": {"one_way": true, "impact": ["本番の表の列を消す"]}`, "C", true},
		{"one_way: true で B も C", "B", `, "risk": {"one_way": true}`, "C", true},
		{"one_way: true で C は C（理由は残す）", "C", `, "risk": {"one_way": true}`, "C", true},
		{"one_way: false で A は A", "A", `, "risk": {"one_way": false}`, "A", false},
		{"one_way: false で C を下げない", "C", `, "risk": {"one_way": false}`, "C", false},
		{"risk の無い古い report.json は今までどおり", "A", ``, "A", false},
		{"impact の文は読まない", "A", `, "risk": {"one_way": false, "impact": ["one_way: true", "戻せない"]}`, "A", false},
		{"manual は1つ上げる", "A", `, "acceptance": [{"id": "A1", "result": "manual", "evidence": "docs/trace.md"}]`, "B", false},
		{"manual と one_way は C", "A", `, "acceptance": [{"id": "A1", "result": "manual", "evidence": "x"}], "risk": {"one_way": true}`, "C", true},
	} {
		rep, err := contract.ParseReport([]byte(base + c.extra + `}`))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		in := Classified{Tier: c.tier, TierReasons: []string{"path x"}}
		got, _ := FromReport(in, rep)
		has := slices.Contains(got.TierReasons, OneWayReason)
		if got.Tier != c.want || has != c.oneWayReason || got.TierReasons[0] != "path x" {
			t.Errorf("%s: %+v want tier %s（理由に one_way: %v）", c.name, got, c.want, c.oneWayReason)
		}
		if len(in.TierReasons) != 1 {
			t.Errorf("%s: 入力の理由を書き換えた: %v", c.name, in.TierReasons)
		}
	}
	rep, _ := contract.ParseReport([]byte(base + `, "risk": {"one_way": true}}`))
	cl, _ := FromReport(Classified{Tier: "A", TierReasons: []string{}}, rep)
	f := Facts{PROpen: true, RepoKnown: true, Tier: cl.Tier, MergeLevel: "B", GitHubChecks: GitHubChecks{State: "success"}}
	if d, r := Decide(f); d != "needs_human" || !reflect.DeepEqual(r, []string{"delegation: merge=B < tier C"}) {
		t.Errorf("one_way の変更がオーナーへ回らない: %s %v", d, r)
	}
	rep, _ = contract.ParseReport([]byte(base + `, "acceptance": [{"id": "A1", "result": "pass", "evidence": "go test ./..."}, {"id": "A2", "result": "fail", "evidence": "bash tests/x.sh"}]}`))
	cl, n := FromReport(Classified{Tier: "A", TierReasons: []string{}}, rep)
	if !reflect.DeepEqual(n, AcceptanceCount{Pass: 1, Fail: 1}) || cl.Tier != "A" {
		t.Errorf("acceptance の数え: %+v %+v", n, cl)
	}
	f = Facts{PROpen: true, RepoKnown: true, Tier: cl.Tier, MergeLevel: "B", GitHubChecks: GitHubChecks{State: "success"}, AcceptanceFail: n.Fail}
	if d, r := Decide(f); d != "fix" || !reflect.DeepEqual(r, []string{"受け入れ条件の fail が 1 件"}) {
		t.Errorf("acceptance の fail で fix にならない: %s %v", d, r)
	}
	// 受け入れ条件の行が指す記録が証拠にならなければ fix（理由は contract.CheckRuns の文のまま。ADR-0035 決めたこと 2）
	f = Facts{PROpen: true, RepoKnown: true, Tier: "A", MergeLevel: "B", GitHubChecks: GitHubChecks{State: "success"},
		RunProblems: []string{"受け入れ条件 A1 の記録 runs/3.json が無い"}}
	if d, r := Decide(f); d != "fix" || !reflect.DeepEqual(r, []string{"受け入れ条件 A1 の記録 runs/3.json が無い"}) {
		t.Errorf("記録が無い行で fix にならない: %s %v", d, r)
	}
}

func TestDecide(t *testing.T) {
	good := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"bash tests/*.sh", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{}}, ChecksWait: 30}
	cases := []struct {
		name string
		mod  func(*Facts)
		want string
	}{
		{"揃えば approve", func(*Facts) {}, "approve"},
		{"kanban と PR が合わない", func(f *Facts) { f.LedgerReasons = []string{"x"} }, "fix"},
		{"draft", func(f *Facts) { f.PROpen = false }, "fix"},
		{"fork", func(f *Facts) { f.Blockers = []string{"fork の PR"} }, "needs_human"},
		{"予約キーを信じられない", func(f *Facts) { f.SettingsReason = "x" }, "needs_human"},
		{"条件表に repo が無い", func(f *Facts) { f.RepoKnown = false }, "needs_human"},
		{"委譲の外", func(f *Facts) { f.Tier = "C" }, "needs_human"},
		{"無委譲", func(f *Facts) { f.MergeLevel = "none" }, "needs_human"},
		{"検査が落ちた", func(f *Facts) { f.Checks[0].OK = false }, "fix"},
		{"レビュー missing", func(f *Facts) { f.Review.State = "missing" }, "fix"},
		{"レビュー stale", func(f *Facts) { f.Review.State = "stale" }, "fix"},
		{"Must fix が残る", func(f *Facts) { f.Review.MustFixOpen = 1 }, "fix"},
		{"同じ head で2度目", func(f *Facts) { f.Review.MustFixOpen = 1; f.PreviousMustFix = true }, "needs_human"},
		{"Must fix を見送った", func(f *Facts) { f.Review.WaivedHeavy = 1 }, "needs_human"},
		{"受け入れ条件の fail", func(f *Facts) { f.AcceptanceFail = 1 }, "fix"},
		{"screen_paths に当たるのに添付が無い", func(f *Facts) { f.ScreenMissing = []string{"web/a.css"} }, "fix"},
		{"委譲の外でも添付が無ければ先に担当へ", func(f *Facts) { f.ScreenMissing = []string{"web/a.css"}; f.Tier = "C" }, "fix"},
		{"添付が無ければ checks を待たずに担当へ", func(f *Facts) {
			f.ScreenMissing = []string{"web/a.css"}
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
		}, "fix"},
		{"レビュー不要なら missing でも通る", func(f *Facts) { f.ReviewRequired = false; f.Review.State = "off" }, "approve"},
		{"GitHub の checks が落ちた", func(f *Facts) { f.GitHubChecks = GitHubChecks{"failure", []string{"test"}} }, "fix"},
		{"GitHub の checks が走っている", func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}} }, "wait"},
		{"走っていても長く待ったまま needs_human にしない", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
			f.ChecksOverdue = true
		}, "wait"},
		{"GitHub の checks に知らない状態", func(f *Facts) { f.GitHubChecks = GitHubChecks{"unknown", []string{"x"}} }, "needs_human"},
		{"GitHub の checks を読めていない", func(f *Facts) { f.GitHubChecks = GitHubChecks{} }, "needs_human"},
		{"checks が1つも無い（要る）", func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"} }, "wait"},
		{"checks が1つも無いまま上限を過ぎた", func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"}; f.ChecksOverdue = true }, "needs_human"},
		{"checks が要らない repo で1つも無い", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{State: "none"}
			f.ChecksOptional, f.ChecksOverdue = true, true
		}, "approve"},
		{"要らない repo でも落ちた checks は止める", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"failure", []string{"test"}}
			f.ChecksOptional = true
		}, "fix"},
		{"走っていても Must fix は先に担当へ", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
			f.Review.MustFixOpen = 1
		}, "fix"},
		{"委譲の外でも走っていれば待つ", func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}; f.Tier = "C" }, "wait"},
		{"走っていても head が kanban と違えば止める", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
			f.LedgerReasons = []string{"report.json の commit が PR の head と違う"}
		}, "fix"},
	}
	for _, c := range cases {
		f := good
		f.Checks = append([]Check{}, good.Checks...)
		c.mod(&f)
		d, reasons := Decide(f)
		if d != c.want || (d == "approve") != (len(reasons) == 0) {
			t.Errorf("%s: %s %v want %s", c.name, d, reasons, c.want)
		}
	}
	if _, r := Decide(func() Facts { f := good; f.Tier = "C"; return f }()); !reflect.DeepEqual(r, []string{"delegation: merge=B < tier C"}) {
		t.Errorf("委譲の理由の形: %v", r)
	}
}

// TestDecideDelegationLast は ADR-0056 受け入れ条件 A1: 委譲の外（オーナーがマージする変更）かは、検査・CI・レビュー・
// 受け入れ条件・CI の待ちのあとに見る。差し戻す理由があれば先に担当へ、CI が走っていれば待つ。オーナー待ちになるのは、
// 委譲の中なら通していた PR だけで、理由は委譲の 1 件。PR の事情・予約キー・条件表の理由は、今の順のままオーナー待ち。
func TestDecideDelegationLast(t *testing.T) {
	outside := Facts{PROpen: true, RepoKnown: true, Tier: "C", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"bash tests/*.sh", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{"test"}}, ChecksWait: 30}
	for _, c := range []struct {
		name   string
		mod    func(*Facts)
		want   string
		reason string // 理由の 1 件目に含む語
	}{
		{"CI が落ちていれば fix", func(f *Facts) { f.GitHubChecks = GitHubChecks{"failure", []string{"test"}} }, "fix", "GitHub の checks が落ちた"},
		{"条件表の検査が落ちていれば fix", func(f *Facts) { f.Checks[0].OK = false }, "fix", "check: "},
		{"レビューが無ければ fix", func(f *Facts) { f.Review.State = "missing" }, "fix", "review missing"},
		{"レビューが古ければ fix", func(f *Facts) { f.Review.State = "stale" }, "fix", "review stale"},
		{"Must fix が残っていれば fix", func(f *Facts) { f.Review.MustFixOpen = 1 }, "fix", "Must fix 以上が 1 件"},
		{"受け入れ条件の fail があれば fix", func(f *Facts) { f.AcceptanceFail = 1 }, "fix", "受け入れ条件の fail"},
		{"CI が走っていれば wait", func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}} }, "wait", "まだ走っている"},
		{"どれも無ければ needs_human（理由は委譲の 1 件）", func(*Facts) {}, "needs_human", "delegation: merge=B < tier C"},
		{"PR の事情は今の順のまま needs_human", func(f *Facts) {
			f.Blockers = []string{"conflict"}
			f.GitHubChecks = GitHubChecks{"failure", []string{"test"}}
		}, "needs_human", "conflict"},
		{"予約キーを信じられなければ今の順のまま needs_human", func(f *Facts) {
			f.SettingsReason = "issue.yaml の review・must_fix が無い"
			f.Review.State = "missing"
		}, "needs_human", "issue.yaml の review・must_fix"},
		{"条件表に repo が無ければ今の順のまま needs_human", func(f *Facts) {
			f.RepoKnown = false
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
		}, "needs_human", "条件表の repos"},
		{"CI が読めなくてもレビューが無ければ fix", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"unknown", []string{"x"}}
			f.Review.State = "missing"
		}, "fix", "review missing"},
		{"CI が読めなくても Must fix が残れば fix", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{"unknown", []string{"x"}}
			f.Review.MustFixOpen = 1
		}, "fix", "Must fix 以上が 1 件"},
		{"CI が来ないまま待ちを過ぎてもレビューが無ければ fix", func(f *Facts) {
			f.GitHubChecks = GitHubChecks{State: "none"}
			f.ChecksOverdue = true
			f.Review.State = "missing"
		}, "fix", "review missing"},
		{"CI が読めず差し戻す理由も無ければ needs_human", func(f *Facts) { f.GitHubChecks = GitHubChecks{"unknown", []string{"x"}} }, "needs_human", "通すか決められない"},
		{"同じ head で2度目の Must fix は変えない", func(f *Facts) { f.Review.MustFixOpen = 1; f.PreviousMustFix = true }, "needs_human", "2度目"},
		{"レビュアーを覆したときは変えない", func(f *Facts) { f.Review.WaivedHeavy = 1 }, "needs_human", "見送った"},
	} {
		f := outside
		f.Checks = append([]Check{}, outside.Checks...)
		c.mod(&f)
		d, r := Decide(f)
		if d != c.want || len(r) == 0 || !strings.Contains(r[0], c.reason) {
			t.Errorf("%s: %s %v want %s（%s）", c.name, d, r, c.want, c.reason)
		}
		if c.want == "needs_human" && c.reason == "delegation: merge=B < tier C" && len(r) != 1 {
			t.Errorf("%s: 理由が委譲の 1 件でない: %v", c.name, r)
		}
	}
}

// TestScreen は ADR-0018 受け入れ条件 3: screen_paths に当たる差分は PR に GitHub の添付が要る。
func TestScreen(t *testing.T) {
	if got := pkgTable(t).ScreenFiles(RepoTable{}, []File{{Path: "web/a.css"}}); len(got) != 0 {
		t.Errorf("既定の条件表（screen_paths 無し）で当たった: %v", got)
	}
	tb := &Table{ScreenPaths: []string{"web/**", "**/*.css"}}
	files := []File{{Path: "app/web/x.tsx"}, {Path: "app/b.css"}, {Path: "app/go/main.go"}, {Path: "app/web/new.tsx", PreviousPath: "app/old/old.css"}, {Path: "app/b.css"}}
	if got, want := tb.ScreenFiles(RepoTable{StripPrefix: "app/"}, files), []string{"web/x.tsx", "b.css", "web/new.tsx", "old/old.css"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ScreenFiles: %v want %v", got, want)
	}
	if got := tb.ScreenFiles(RepoTable{}, []File{{Path: "internal/gate/gate.go"}}); len(got) != 0 {
		t.Errorf("画面でないファイルに当たった: %v", got)
	}
	for name, c := range map[string]struct {
		texts []string
		want  bool
	}{
		"gh --attach の画像（本文）": {[]string{"## Summary\n\n![login](https://github.com/user-attachments/assets/0a1b2c3d-4e5f-6789-abcd-ef0123456789)\n"}, true},
		"Web で上げた画像（img タグ）":  {[]string{"", `<img width="400" alt="x" src="https://github.com/user-attachments/assets/1f2e3d4c-0000-1111-2222-333344445555" />`}, true},
		"動画（URL だけ）・コメント":     {[]string{"本文", "動画\nhttps://github.com/user-attachments/assets/abcdef01-2345-6789-abcd-ef0123456789"}, true},
		"添付が無い":               {[]string{"## Summary\nテストを直した", "andon gate: **fix**"}, false},
		"書き換えられていない相対パス":      {[]string{"![x](./login.png)"}, false},
		"外の画像の URL":           {[]string{"![x](https://example.com/a.png)"}, false},
		"添付のファイル（画像・動画でない）":   {[]string{"[log.zip](https://github.com/user-attachments/files/123/log.zip)"}, false},
		"assets だけで id が無い":   {[]string{"https://github.com/user-attachments/assets/"}, false},
		"本文もコメントも無い":          {nil, false},
	} {
		if got := HasAttachment(c.texts); got != c.want {
			t.Errorf("%s: %v want %v", name, got, c.want)
		}
	}
	_, r := Decide(Facts{PROpen: true, RepoKnown: true, Tier: "A", MergeLevel: "A", GitHubChecks: GitHubChecks{State: "success"},
		ScreenMissing: []string{"web/a.css", "web/b.tsx"}})
	if want := []string{"screen_paths: web/a.css に当たるが画面の証拠（添付）が無い", "screen_paths: web/b.tsx に当たるが画面の証拠（添付）が無い"}; !reflect.DeepEqual(r, want) {
		t.Errorf("理由の形: %v", r)
	}
}

// TestChecksToExit は GitHub の checks の読みから gate の終了コードまでを通す（P7・v0.0.3: 0 件を success と読まない）。
func TestChecksToExit(t *testing.T) {
	run := func(name, status, conclusion string) CheckItem {
		return CheckItem{Typename: "CheckRun", Name: name, Status: status, Conclusion: conclusion}
	}
	good := Facts{PROpen: true, RepoKnown: true, Tier: "A", MergeLevel: "B", ChecksWait: 30}
	for _, c := range []struct {
		name     string
		items    []CheckItem
		overdue  bool
		decision string
		exit     int
	}{
		{"checks 0 件は待つ", nil, false, "wait", 5},
		{"gate 自身の status だけ（0 件と同じ）は待つ", []CheckItem{{Typename: "StatusContext", Context: GateContext, State: "SUCCESS"}}, false, "wait", 5},
		{"30 分を過ぎた 0 件はオーナーへ", nil, true, "needs_human", 4},
		{"全部 success は approve", []CheckItem{run("go", "COMPLETED", "SUCCESS"), run("tests", "COMPLETED", "SUCCESS"), run("release", "COMPLETED", "SKIPPED")}, false, "approve", 0},
		{"走っているもの 1 つは待つ", []CheckItem{run("go", "COMPLETED", "SUCCESS"), run("tests", "IN_PROGRESS", "")}, true, "wait", 5},
	} {
		f := good
		f.GitHubChecks, f.ChecksOverdue = SummarizeChecks(c.items), c.overdue
		if d, _ := Decide(f); d != c.decision || ExitCode(d) != c.exit {
			t.Errorf("%s: %s（終了コード %d） want %s（%d）", c.name, d, ExitCode(d), c.decision, c.exit)
		}
	}
	if ExitCode("fix") != 1 {
		t.Error("fix の終了コード")
	}
}

func TestSummarizeChecks(t *testing.T) {
	run := func(name, status, conclusion string) CheckItem {
		return CheckItem{Typename: "CheckRun", Name: name, Status: status, Conclusion: conclusion}
	}
	ctx := func(name, state string) CheckItem {
		return CheckItem{Typename: "StatusContext", Context: name, State: state}
	}
	for _, c := range []struct {
		name  string
		items []CheckItem
		want  GitHubChecks
	}{
		{"無い", nil, GitHubChecks{"none", []string{}}},
		{"gate 自身の status だけは無いと同じ", []CheckItem{ctx(GateContext, "FAILURE")}, GitHubChecks{"none", []string{}}},
		{"全部 success（skipped・neutral も通す）", []CheckItem{run("test", "COMPLETED", "SUCCESS"), run("lint", "COMPLETED", "SKIPPED"),
			run("opt", "COMPLETED", "NEUTRAL"), ctx("ci/x", "SUCCESS"), ctx(GateContext, "FAILURE")}, GitHubChecks{"success", []string{"test", "lint", "opt", "ci/x"}}},
		{"走っている", []CheckItem{run("test", "COMPLETED", "SUCCESS"), run("image", "IN_PROGRESS", ""), run("q", "QUEUED", "")},
			GitHubChecks{"pending", []string{"image", "q"}}},
		{"status の pending", []CheckItem{ctx("ci/x", "PENDING")}, GitHubChecks{"pending", []string{"ci/x"}}},
		{"落ちたのが一番重い", []CheckItem{run("image", "IN_PROGRESS", ""), run("test", "COMPLETED", "FAILURE"),
			run("c", "COMPLETED", "CANCELLED"), run("t", "COMPLETED", "TIMED_OUT"), ctx("ci/x", "ERROR")},
			GitHubChecks{"failure", []string{"test", "c", "t", "ci/x"}}},
		{"action_required・知らない値は unknown", []CheckItem{run("test", "COMPLETED", "SUCCESS"), run("deploy", "COMPLETED", "ACTION_REQUIRED"),
			run("x", "SOMETHING", "")}, GitHubChecks{"unknown", []string{"deploy", "x"}}},
	} {
		if got := SummarizeChecks(c.items); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v want %+v", c.name, got, c.want)
		}
	}
	if !(GitHubChecks{State: "success"}).Passed(false) || (GitHubChecks{State: "none"}).Passed(false) ||
		!(GitHubChecks{State: "none"}).Passed(true) || (GitHubChecks{State: "pending"}).Passed(true) {
		t.Error("Passed")
	}
}

func TestSummarizeChecksLatestRun(t *testing.T) {
	run := func(wf, conclusion string, id int64) CheckItem {
		return CheckItem{Typename: "CheckRun", Name: "test", Status: "COMPLETED", Conclusion: conclusion, DatabaseID: id, App: "github-actions", Workflow: wf}
	}
	ctx := func(state, at string) CheckItem {
		return CheckItem{Typename: "StatusContext", Context: "ci/x", State: state, CreatedAt: at}
	}
	queued := CheckItem{Typename: "CheckRun", Name: "test", Status: "QUEUED", DatabaseID: 3, App: "github-actions", Workflow: "tests"}
	for _, c := range []struct {
		name  string
		items []CheckItem
		want  string
	}{
		{"古い failure → 新しい success", []CheckItem{run("tests", "FAILURE", 1), run("tests", "SUCCESS", 2)}, "success"},
		{"新しい success が先に出ても", []CheckItem{run("tests", "SUCCESS", 2), run("tests", "CANCELLED", 1)}, "success"},
		{"古い success → 新しい failure", []CheckItem{run("tests", "SUCCESS", 1), run("tests", "FAILURE", 2)}, "failure"},
		{"新しい failure が先に出ても", []CheckItem{run("tests", "FAILURE", 2), run("tests", "SUCCESS", 1)}, "failure"},
		{"走り直しは pending", []CheckItem{run("tests", "SUCCESS", 2), queued}, "pending"},
		{"別の workflow の同じ名前は別の check", []CheckItem{run("lint", "FAILURE", 1), run("tests", "SUCCESS", 2)}, "failure"},
		// 表示名が同じでも別ファイルの workflow は id が違う: 束ねない
		{"同じ表示名・別 id の workflow（failure が先）", []CheckItem{run("W_ci_a", "FAILURE", 1), run("W_ci_b", "SUCCESS", 2)}, "failure"},
		{"同じ表示名・別 id の workflow（success が先）", []CheckItem{run("W_ci_b", "SUCCESS", 2), run("W_ci_a", "FAILURE", 1)}, "failure"},
		// 新旧を判別できない（同じ createdAt・同じ databaseId）ときは悪い方を残す。順に依らない
		{"status: 同じ時刻の failure と success", []CheckItem{ctx("FAILURE", "2026-11-06T09:00:00Z"), ctx("SUCCESS", "2026-11-06T09:00:00Z")}, "failure"},
		{"status: 同じ時刻の success と failure", []CheckItem{ctx("SUCCESS", "2026-11-06T09:00:00Z"), ctx("FAILURE", "2026-11-06T09:00:00Z")}, "failure"},
		{"status: 同じ時刻の pending と success", []CheckItem{ctx("PENDING", "2026-11-06T09:00:00Z"), ctx("SUCCESS", "2026-11-06T09:00:00Z")}, "pending"},
		{"status: 同じ時刻の success と pending", []CheckItem{ctx("SUCCESS", "2026-11-06T09:00:00Z"), ctx("PENDING", "2026-11-06T09:00:00Z")}, "pending"},
		{"CheckRun: 同じ id の failure と success", []CheckItem{run("tests", "FAILURE", 5), run("tests", "SUCCESS", 5)}, "failure"},
		{"CheckRun: 同じ id の success と failure", []CheckItem{run("tests", "SUCCESS", 5), run("tests", "FAILURE", 5)}, "failure"},
		{"status: 古い error → 新しい success", []CheckItem{ctx("ERROR", "2026-11-06T09:00:00Z"), ctx("SUCCESS", "2026-11-06T09:05:00Z")}, "success"},
		{"status: 新しい failure が先に出ても", []CheckItem{ctx("FAILURE", "2026-11-06T09:05:00Z"), ctx("SUCCESS", "2026-11-06T09:00:00Z")}, "failure"},
	} {
		if got := SummarizeChecks(c.items); got.State != c.want {
			t.Errorf("%s: %+v want %s", c.name, got, c.want)
		}
	}
}

func TestChecksOverdue(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	rows := []ledger.Row{
		{At: "2026-11-06T08:00:00Z", ID: "D-1", Change: map[string]any{"status": "review"}},
		{At: "2026-11-06T09:40:00Z", ID: "D-1", Change: map[string]any{"status": "review"}}, // 入り直した行から数える
		{At: "2026-11-06T09:00:00Z", ID: "D-2", Change: map[string]any{"status": "review"}},
	}
	switch {
	case ChecksOverdue(rows, "D-1", now, 30):
		t.Error("入り直してから 20 分で過ぎたことになった")
	case !ChecksOverdue(rows, "D-1", now, 20):
		t.Error("20 分ちょうどで過ぎていない")
	case !ChecksOverdue(rows, "D-3", now, 30):
		t.Error("review の行が無いのに過ぎていない（安全側）")
	case !ChecksOverdue([]ledger.Row{{At: "x", ID: "D-1", Change: map[string]any{"status": "review"}}}, "D-1", now, 30):
		t.Error("at が読めないのに過ぎていない（安全側）")
	}
}

// 条件表の入口は Decode の厳しい読み方を通る: null・型違い・知らないキーは誤り（null の check で検査を飛ばさない）
func TestLoadTableRejectsNullAndWrongTypes(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	pkg := filepath.Join(filepath.Dir(file), "..", "..", "package")
	for name, local := range map[string]string{
		"check が null":         `{"repos": {"github.com/example/app": {"check": null}}}`,
		"repos の値が null":       `{"repos": {"github.com/example/app": null}}`,
		"tier_c_paths が null":  `{"tier_c_paths": null}`,
		"check が文字列":           `{"repos": {"github.com/example/app": {"check": "bash x"}}}`,
		"check の要素が null":      `{"repos": {"github.com/example/app": {"check": [null]}}}`,
		"知らないキー":               `{"repos": {"github.com/example/app": {"checks": ["x"]}}}`,
		"Check が null（大小文字違い）": `{"repos": {"github.com/example/app": {"Check": null}}}`,
		"Check が文字列（大小文字違い）":   `{"repos": {"github.com/example/app": {"Check": "true"}}}`,
		"Repos（大小文字違い）":        `{"Repos": {}}`,
		"github_checks が知らない値": `{"repos": {"github.com/example/app": {"github_checks": "off"}}}`,
		"screen_paths が文字列":    `{"screen_paths": "web/**"}`,
	} {
		ws := ownerPlant(t, local)
		if _, warn, err := LoadTable(pkg, ws); err == nil {
			t.Errorf("%s: 通ってしまう（warn %q）", name, warn)
		}
	}
	if tb, _, err := LoadTable(pkg, ownerPlant(t, `{"screen_paths": ["web/**"]}`)); err != nil || !reflect.DeepEqual(tb.ScreenPaths, []string{"web/**"}) {
		t.Errorf("gate.local.json の screen_paths が効かない: %v", err)
	}
	ws := ownerPlant(t, `{"repos": {"github.com/example/app": {"check": []}}}`)
	if tb, warn, err := LoadTable(pkg, ws); err != nil || warn != "" || tb.Repos["github.com/example/app"].Check == nil {
		t.Errorf("空の check（オーナーが決めた「検査なし」）は通る: %v %q", err, warn)
	}
}

func ownerPlant(t *testing.T, local string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte(`owner = "owner@example.com"`+"\n"), 0o644)
	os.WriteFile(filepath.Join(ws, "gate.local.json"), []byte(local), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-qm", "plant"}} {
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=o", "GIT_AUTHOR_EMAIL=owner@example.com", "GIT_COMMITTER_NAME=o", "GIT_COMMITTER_EMAIL=owner@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return ws
}

func TestAddedLines(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,4 @@\n ctx\n+add2\n-old\n ctx3\n+add4\n\\ No newline at end of file\n" +
		"diff --git a/gone.go b/gone.go\n--- a/gone.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n" +
		"diff --git a/n.md b/n.md\nnew file mode 100644\n--- /dev/null\n+++ b/n.md\n@@ -0,0 +1,2 @@\n++++ not a header\n+b\n"
	// git の core.quotePath（既定）は非 ASCII・引用符の名前を C 形式で引用する
	diff += "diff --git \"a/\\350\\250\\255\\345\\256\\232.go\" \"b/\\350\\250\\255\\345\\256\\232.go\"\nnew file mode 100644\n--- /dev/null\n" +
		"+++ \"b/\\350\\250\\255\\345\\256\\232.go\"\n@@ -0,0 +1 @@\n+x\n" +
		"diff --git \"a/q\\\"t.go\" \"b/q\\\"t.go\"\n--- /dev/null\n+++ \"b/q\\\"t.go\"\n@@ -0,0 +1 @@\n+y\n"
	got := AddedLines(diff)
	want := map[string]map[int]bool{"a.go": {2: true, 4: true}, "n.md": {1: true, 2: true}, "設定.go": {1: true}, `q"t.go`: {1: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AddedLines = %v, want %v", got, want)
	}
}

// 本物の package/gate.json で、モデルの規則・検査の設定・対応表はレベル C、行動のファイルだけは C にならない（ADR-0025 受け入れ条件 7）。
func TestClassifyModelFiles(t *testing.T) {
	tb := pkgTable(t)
	for _, p := range []string{"model/IssueRules.tla", "model/IssueRules.cfg", "model/expected.txt", "model/issue-map.txt"} {
		got, err := tb.Classify(RepoTable{}, []File{{Path: p, Additions: 1}}, "", false)
		if err != nil || got.Tier != "C" {
			t.Errorf("%s: tier=%q err=%v, want C", p, got.Tier, err)
		}
	}
	for _, p := range []string{"model/IssueLife.tla", "model/broken/BrokenFinish.tla"} {
		got, err := tb.Classify(RepoTable{}, []File{{Path: p, Additions: 1}}, "", false)
		if err != nil || got.Tier == "C" {
			t.Errorf("%s: tier=%q err=%v, want not C", p, got.Tier, err)
		}
	}
}

// hunkDiff は1ファイル・1 hunk の差分（行の頭の +・-・空白はそのまま書く）。
func hunkDiff(path string, oldN, newN int, body ...string) string {
	return fmt.Sprintf("diff --git a/%[1]s b/%[1]s\n--- a/%[1]s\n+++ b/%[1]s\n@@ -1,%[2]d +1,%[3]d @@\n%[4]s\n", path, oldN, newN, strings.Join(body, "\n"))
}

// 受け入れ条件 4（ADR-0041 決めたこと 4）: 追加行のパターンは、当たる箇所がその hunk で増えたときだけ C の理由にする。
func TestClassifyAddedLinePatternGrowth(t *testing.T) {
	tb := pkgTable(t)
	sudo := `\bsudo\b`
	for _, c := range []struct {
		name   string
		path   string
		diff   string
		tier   string
		reason string
	}{
		{"もとから sudo のある行を書き換えただけは B（増えていない）", "internal/x.go",
			hunkDiff("internal/x.go", 2, 2, " ctx", "-// オーナーが違えば sudo chown で渡す", "+// ファイルのオーナーが違えば sudo chown で渡す"),
			"B", "追加行 " + sudo + NotGrown},
		{"sudo の行を新しく足すと C", "internal/x.go",
			hunkDiff("internal/x.go", 1, 2, " ctx", "+run sudo x"),
			"C", "追加行 " + sudo},
		{"同じ hunk で sudo の行を 1 つ消して 1 つ足すと B", "internal/x.go",
			hunkDiff("internal/x.go", 3, 3, "-sudo apt-get update", " ctx", "+sudo chown -R a b"),
			"B", "追加行 " + sudo + NotGrown},
		{"新しいファイルは C", "internal/y.go",
			"diff --git a/internal/y.go b/internal/y.go\nnew file mode 100644\n--- /dev/null\n+++ b/internal/y.go\n@@ -0,0 +1,1 @@\n+run sudo x\n",
			"C", "追加行 " + sudo},
		{"docs だけでも、当たった行を書き換えていれば A にならない", "docs/x.md",
			hunkDiff("docs/x.md", 1, 1, "-利用者・sudo・git を入れる（古い語）", "+利用者・sudo・git を入れる（新しい語）"),
			"B", "追加行 " + sudo + NotGrown},
	} {
		got, err := tb.Classify(RepoTable{}, []File{{Path: c.path, Additions: 1, Deletions: 1}}, c.diff, false)
		if err != nil || got.Tier != c.tier || !slices.Contains(got.TierReasons, c.reason) {
			t.Errorf("%s: tier %s（%v）want %s と理由 %q", c.name, got.Tier, got.TierReasons, c.tier, c.reason)
		}
	}
}

func TestClassifyAddedLinePatternEdges(t *testing.T) {
	tb := pkgTable(t)
	files := []File{{Path: "docs/x.md", Additions: 1, Deletions: 1}}
	for _, c := range []struct {
		name, diff, tier string
	}{
		// 離れた hunk の消した行では打ち消さない（ファイルごとに数えない）
		{"別の hunk で消して足すと C", "diff --git a/docs/x.md b/docs/x.md\n--- a/docs/x.md\n+++ b/docs/x.md\n" +
			"@@ -1,1 +1,0 @@\n-sudo a\n@@ -9,0 +9,1 @@\n+sudo b\n", "C"},
		{"消して別の名前で足すと C", "diff --git a/docs/a.md b/docs/a.md\ndeleted file mode 100644\n--- a/docs/a.md\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-sudo a\n" +
			"diff --git a/docs/b.md b/docs/b.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/b.md\n@@ -0,0 +1,1 @@\n+sudo a\n", "C"},
		{"hunk の中の +++ の行も数える", hunkDiff("docs/x.md", 0, 1, "+++ sudo x"), "C"},
		{"同じ行で当たる箇所が増えると C", hunkDiff("docs/x.md", 1, 1, "-sudo a", "+sudo a && sudo b"), "C"},
		// 当たった文字列ごとに数える（password= を token= に替えると token= が増えた）
		{"別の文字列に替えると C", hunkDiff("docs/x.md", 1, 1, "-password=x", "+token=x"), "C"},
		{"消した行だけが当たるなら理由にしない", hunkDiff("docs/x.md", 1, 0, "-sudo a"), "A"},
	} {
		got, err := tb.Classify(RepoTable{}, files, c.diff, false)
		if err != nil || got.Tier != c.tier {
			t.Errorf("%s: tier %s（%v）want %s", c.name, got.Tier, got.TierReasons, c.tier)
		}
	}
}

func TestHunks(t *testing.T) {
	diff := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n a\n--- old\n++++ new\n b\n\\ No newline at end of file\n" +
		"@@ -9 +9 @@\n-c\n+d\n"
	want := []Hunk{{Added: []string{"+++ new"}, Removed: []string{"-- old"}}, {Added: []string{"d"}, Removed: []string{"c"}}}
	if got := Hunks(diff); !reflect.DeepEqual(got, want) {
		t.Errorf("Hunks = %#v, want %#v", got, want)
	}
	// 見出しの行数が実際より多くても、次のファイルの見出し（--- a/…・+++ b/…）を数えない
	if got := Hunks("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,5 +1,5 @@\n-a\n+b\ndiff --git a/y b/y\n--- a/y\n+++ b/y\n@@ -1 +1 @@\n-c\n+d\n"); !reflect.DeepEqual(got,
		[]Hunk{{Added: []string{"b"}, Removed: []string{"a"}}, {Added: []string{"d"}, Removed: []string{"c"}}}) {
		t.Errorf("行数の多すぎる見出しの Hunks = %#v", got)
	}
	if got := Hunks("+a\n-b\ndiff --git a/y b/y\n+c\n"); len(got) != 2 || got[0].Added[0] != "a" || got[0].Removed[0] != "b" || got[1].Added[0] != "c" {
		t.Errorf("見出しの無い差分の Hunks = %#v", got)
	}
}

// 受け入れ条件 5（ADR-0041 決めたこと 4）: 語を置き換えた PR の差分を今の条件表で判定すると B、新しく sudo を書いた PR は C。
// 見本は PR の差分のうちパターンに当たる hunk だけを縮めたもの（変わらない行と当たらない行を除き、長い行は当たった所の前後で切った。
// 見出しの行数は縮めたあとの数）。ファイルの一覧（パス・足した行・消した行）は PR のまま。
func TestClassifyPRSamples(t *testing.T) {
	tb := pkgTable(t)
	for _, c := range []struct {
		pr     string
		tier   string
		reason string
	}{
		{"383", "B", "追加行 authorized_keys" + NotGrown},
		{"392", "B", "追加行 password=" + NotGrown},
		{"394", "B", `追加行 \bsudo\b` + NotGrown},
		{"318", "C", `追加行 \bsudo\b`},
	} {
		diff, err := os.ReadFile(filepath.Join("testdata", "pr-"+c.pr+".diff"))
		if err != nil {
			t.Fatal(err)
		}
		list, err := os.ReadFile(filepath.Join("testdata", "pr-"+c.pr+".files"))
		if err != nil {
			t.Fatal(err)
		}
		var files []File
		for _, l := range strings.Split(strings.TrimSpace(string(list)), "\n") {
			var f File
			if _, err := fmt.Sscanf(l, "%s +%d -%d", &f.Path, &f.Additions, &f.Deletions); err != nil {
				t.Fatalf("pr-%s.files: %q: %v", c.pr, l, err)
			}
			files = append(files, f)
		}
		got, err := tb.Classify(RepoTable{}, files, string(diff), false)
		if err != nil || got.Tier != c.tier || !slices.Contains(got.TierReasons, c.reason) {
			t.Errorf("#%s: tier %s（%v）want %s と理由 %q", c.pr, got.Tier, got.TierReasons, c.tier, c.reason)
		}
		for _, r := range got.TierReasons {
			if c.tier == "B" && strings.HasPrefix(r, "追加行 ") && !strings.HasSuffix(r, NotGrown) {
				t.Errorf("#%s: 増えていない語の置き換えに C の理由 %q", c.pr, r)
			}
		}
	}
}
