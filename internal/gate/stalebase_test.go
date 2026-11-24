package gate

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"andon/internal/ledger"
)

// 根拠の出来事: 既定ブランチに、検証の機能の一覧が internal/ の下のファイルを全部載せているかを見る検査
// （tests/verify-andon.sh と .claude/skills/verify-andon/ の一覧）が入ったあと、それより前の base で CI を通した PR
// （internal/dispatcher/otel.go を足す）がマージされ、既定ブランチの tests が落ちた。
// その PR を gate が通さないこと（rebase して CI を回し直すまで fix）を見る。

// mainAfterBase は、PR の分かれ目より後に既定ブランチで変わったファイル（compare の応答。compareJq を通した形）。
const mainAfterBase = `{"ahead_by": 2, "files": [
  {"filename": "tests/verify-andon.sh", "previous_filename": null},
  {"filename": ".claude/skills/verify-andon/features/dispatcher-news.md", "previous_filename": null},
  {"filename": "internal/waiting/open.go", "previous_filename": null}]}`

func TestStaleBaseFromIncident(t *testing.T) {
	files, unread, err := ParseCompare([]byte(mainAfterBase))
	if err != nil || unread {
		t.Fatalf("compare を読めない: %v %v", err, unread)
	}
	moved := CheckPlaceFiles(RepoTable{}, files)
	want := []string{"tests/verify-andon.sh", ".claude/skills/verify-andon/features/dispatcher-news.md"}
	if !slices.Equal(moved, want) {
		t.Fatalf("検査の置き場 = %v, want %v", moved, want)
	}
	// PR 自身は CI もレビューも通り、委譲の中（ほかに戻す理由が無い）
	f := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"go test ./...", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{"go", "tests"}}, StaleBase: moved}
	d, r := Decide(f)
	if d != "fix" || len(r) != 1 || !strings.HasPrefix(r[0], "base が古い: ") || !strings.Contains(r[0], "tests/verify-andon.sh") {
		t.Fatalf("古い base の PR を戻さない: %s %v", d, r)
	}
	// 外の人（メンテナー）にも、CI が今の検査を通っていないことが分かる
	if o := outsideReason(r[0]); !strings.Contains(o, "今の検査を通っていない") || !strings.Contains(o, "tests/verify-andon.sh") {
		t.Fatalf("外向けの文が古い base を言わない: %s", o)
	}
	// rebase したあと（分かれ目が既定ブランチの先頭）は通す
	f.StaleBase = nil
	if d, r := Decide(f); d != "approve" {
		t.Fatalf("rebase したあとも通さない: %s %v", d, r)
	}
}

func TestStaleBaseFirst(t *testing.T) {
	base := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Review: ledger.Review{State: "current"}, GitHubChecks: GitHubChecks{State: "success"}, StaleBase: []string{"tests/a.sh"}}
	// CI が走っている・CI が落ちた・レビューが無い・検査が落ちたときも、古い base を先に返す（その結果は今の検査のものでない）
	for _, m := range []func(*Facts){
		func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"tests"}} },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{"failure", []string{"tests"}} },
		func(f *Facts) { f.Review = ledger.Review{State: "missing", Reason: "x"} },
		func(f *Facts) { f.Checks = []Check{{"go test ./...", false}} },
		func(f *Facts) { f.MergeLevel = "A" },
	} {
		f := base
		m(&f)
		if d, r := Decide(f); d != "fix" || !strings.HasPrefix(r[0], "base が古い: ") {
			t.Fatalf("古い base より先に別の判定: %s %v（%+v）", d, r, f)
		}
	}
	// PR の事情（衝突など）はオーナー待ちのまま（gate は古い base を読む前に止める）
	f := base
	f.Blockers = []string{"conflict"}
	if d, _ := Decide(f); d != "needs_human" {
		t.Fatalf("PR の事情より古い base を先にした: %s", d)
	}
	// 読み切れないときも戻す
	f = base
	f.StaleBase, f.BaseUnread = nil, true
	if d, r := Decide(f); d != "fix" || !strings.Contains(r[0], "読み切れない") {
		t.Fatalf("読み切れない base を通した: %s %v", d, r)
	}
}

func TestCheckPlaceFiles(t *testing.T) {
	files := []File{
		{Path: "tests/run"}, {Path: ".github/workflows/go.yml"}, {Path: ".claude/skills/verify-andon/SKILL.md"},
		{Path: "docs/tests/a.md"}, {Path: "internal/gate/gate.go"}, {Path: ".github/CODEOWNERS"}, {Path: "internal/x_test.go"},
		{Path: "lib/new.sh", PreviousPath: "tests/lib/old.sh"}, // 検査の置き場から出した rename も変わったうちに数える
		{Path: "tests/run"},
	}
	want := []string{"tests/run", ".github/workflows/go.yml", ".claude/skills/verify-andon/SKILL.md", "tests/lib/old.sh"}
	if got := CheckPlaceFiles(RepoTable{}, files); !slices.Equal(got, want) {
		t.Fatalf("CheckPlaceFiles = %v, want %v", got, want)
	}
	// strip_prefix を外して見る
	if got := CheckPlaceFiles(RepoTable{StripPrefix: "app/"}, []File{{Path: "app/tests/x.sh"}, {Path: "tests/y.sh"}}); !slices.Equal(got, []string{"tests/x.sh", "tests/y.sh"}) {
		t.Fatalf("strip_prefix を外さない: %v", got)
	}
}

func TestParseCompare(t *testing.T) {
	// 既定ブランチが分かれ目から進んでいない（rebase 済み）
	if files, unread, err := ParseCompare([]byte(`{"ahead_by": 0, "files": [{"filename": "tests/a.sh"}]}`)); err != nil || unread || len(files) != 0 {
		t.Fatalf("進んでいない既定ブランチ: %v %v %v", files, unread, err)
	}
	// 上限の数だけ返れば、一覧が切れているかもしれない
	var b strings.Builder
	b.WriteString(`{"ahead_by": 400, "files": [`)
	for i := range compareFilesLimit {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"filename": "docs/%d.md"}`, i)
	}
	b.WriteString("]}")
	if files, unread, err := ParseCompare([]byte(b.String())); err != nil || !unread || len(files) != compareFilesLimit {
		t.Fatalf("上限で切れた一覧を読み切れたとした: %d %v %v", len(files), unread, err)
	}
	// 読めない応答は誤り（通さない）
	for _, s := range []string{``, `{}`, `{"ahead_by": -1}`, `{"ahead_by": 1, "files": [{"filename": ""}]}`, `[]`} {
		if _, _, err := ParseCompare([]byte(s)); err == nil {
			t.Fatalf("読めない応答を読めたとした: %q", s)
		}
	}
}
