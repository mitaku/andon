package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
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

// TestFromReport は担当の report.json の申告がレベルを上げる向きにだけ効くことを固定する（risk.one_way・acceptance）。
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
	// 申告で C になった変更は、委譲の merge が C でなければ人へ回る
	rep, _ := contract.ParseReport([]byte(base + `, "risk": {"one_way": true}}`))
	cl, _ := FromReport(Classified{Tier: "A", TierReasons: []string{}}, rep)
	f := Facts{PROpen: true, RepoKnown: true, Tier: cl.Tier, MergeLevel: "B", GitHubChecks: GitHubChecks{State: "success"}}
	if d, r := Decide(f); d != "needs_human" || !reflect.DeepEqual(r, []string{"delegation: merge=B < tier C"}) {
		t.Errorf("one_way の変更が人へ回らない: %s %v", d, r)
	}
	// acceptance に fail がある report はゲートが fix を返す（数えて Decide に渡す）
	rep, _ = contract.ParseReport([]byte(base + `, "acceptance": [{"id": "A1", "result": "pass", "evidence": "go test ./..."}, {"id": "A2", "result": "fail", "evidence": "bash tests/x.sh"}]}`))
	cl, n := FromReport(Classified{Tier: "A", TierReasons: []string{}}, rep)
	if n != (AcceptanceCount{Pass: 1, Fail: 1}) || cl.Tier != "A" {
		t.Errorf("acceptance の数え: %+v %+v", n, cl)
	}
	f = Facts{PROpen: true, RepoKnown: true, Tier: cl.Tier, MergeLevel: "B", GitHubChecks: GitHubChecks{State: "success"}, AcceptanceFail: n.Fail}
	if d, r := Decide(f); d != "fix" || !reflect.DeepEqual(r, []string{"受け入れ条件の fail が 1 件"}) {
		t.Errorf("acceptance の fail で fix にならない: %s %v", d, r)
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
		{"台帳と PR が合わない", func(f *Facts) { f.LedgerReasons = []string{"x"} }, "fix"},
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
		{"走っていても委譲の外は人へ", func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}; f.Tier = "C" }, "needs_human"},
		{"走っていても head が台帳と違えば止める", func(f *Facts) {
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

// TestChecksToExit は GitHub の checks の読みからゲートの終了コードまでを通す（P7・v0.0.3: 0 件を success と読まない）。
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
		{"ゲート自身の status だけ（0 件と同じ）は待つ", []CheckItem{{Typename: "StatusContext", Context: GateContext, State: "SUCCESS"}}, false, "wait", 5},
		{"30 分を過ぎた 0 件は人へ", nil, true, "needs_human", 4},
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
		{"ゲート自身の status だけは無いと同じ", []CheckItem{ctx(GateContext, "FAILURE")}, GitHubChecks{"none", []string{}}},
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
	} {
		ws := ownerPlant(t, local)
		if _, warn, err := LoadTable(pkg, ws); err == nil {
			t.Errorf("%s: 通ってしまう（warn %q）", name, warn)
		}
	}
	ws := ownerPlant(t, `{"repos": {"github.com/example/app": {"check": []}}}`)
	if tb, warn, err := LoadTable(pkg, ws); err != nil || warn != "" || tb.Repos["github.com/example/app"].Check == nil {
		t.Errorf("空の check（人が決めた「検査なし」）は通る: %v %q", err, warn)
	}
}

// ownerPlant は owner が gate.local.json を commit したセルの repo を作る。
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
	got := AddedLines(diff)
	want := map[string]map[int]bool{"a.go": {2: true, 4: true}, "n.md": {1: true, 2: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AddedLines = %v, want %v", got, want)
	}
}
