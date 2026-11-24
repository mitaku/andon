package retro

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/ledger"
)

// 件数のきっかけの数え方（ADR-0039 決めたこと 8）。数えるのは次の振り返りが全文で読む束で、collect と同じ境目と選び方。

func (f *split) issues() []ledger.Issue {
	f.t.Helper()
	ents, err := os.ReadDir(filepath.Join(f.k(), "issues"))
	if err != nil {
		f.t.Fatal(err)
	}
	var out []ledger.Issue
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(f.k(), "issues", e.Name(), "issue.yaml"))
		if err != nil {
			f.t.Fatal(err)
		}
		var raw ledger.Issue
		if err := yaml.Unmarshal(b, &raw); err != nil {
			f.t.Fatal(err)
		}
		raw["id"] = e.Name()
		out = append(out, raw)
	}
	return out
}

func (f *split) fresh() int {
	f.t.Helper()
	f.collect(Options{})
	return Fresh(f.root, f.issues(), ledger.At(f.root).Log(), now)
}

func TestFreshCountsWhatNextRetroReadsInFull(t *testing.T) {
	f, _ := boundaryFixture(t)
	in := f.collect(Options{})
	if got := f.fresh(); got != 25 || got != len(in.Issues) {
		t.Fatalf("Fresh = %d（want 25・collect の全文 %d）", got, len(in.Issues))
	}
	defer func(n int) { fullLimit = n }(fullLimit)
	fullLimit = 10
	if got := f.fresh(); got != 25 {
		t.Errorf("全文の上限で切った: %d", got)
	}
}

func TestFreshWithoutPreviousRetro(t *testing.T) {
	f := newSplit(t)
	for i := 1; i <= 3; i++ {
		f.issue(fmt.Sprintf("A-%04d", i), now.Add(-time.Duration(i)*time.Hour).Format(time.RFC3339))
	}
	f.issue("A-0009", now.Add(-8*day).Format(time.RFC3339))
	if got := f.fresh(); got != 3 {
		t.Fatalf("前の振り返りが無いとき %d（want 3）", got)
	}
	f.retro("A-0010", now.Add(-90*time.Minute), now.Add(-2*time.Hour), "")
	if got := f.fresh(); got != 1 {
		t.Errorf("前の振り返りが集めたあとの 1 件だけを数えない: %d", got)
	}
}

// 空きの振り返りの線（ADR-0047 決めたこと 8）は、Fresh の束からルーチンが足した課題（routine を持つ）を除いて数える。
// Fresh（件数のきっかけ）の数え方は変えない。
func TestFreshOwnSkipsRoutineIssues(t *testing.T) {
	f := newSplit(t)
	for i := 1; i <= 3; i++ {
		f.issue(fmt.Sprintf("A-%04d", i), now.Add(-time.Duration(i)*time.Hour).Format(time.RFC3339))
	}
	for _, id := range []string{"A-0004", "A-0005"} {
		f.issue(id, now.Add(-30*time.Minute).Format(time.RFC3339))
		write(t, filepath.Join(f.is(id), "issue.yaml"), issueYAML(id, "done", "profile: sonnet\nroutine: triage\n"))
	}
	f.collect(Options{})
	issues, rows := f.issues(), ledger.At(f.root).Log()
	if got := Fresh(f.root, issues, rows, now); got != 5 {
		t.Errorf("Fresh = %d（ルーチンの課題も数えて 5 のはず）", got)
	}
	if got := FreshOwn(f.root, issues, rows, now); got != 3 {
		t.Errorf("FreshOwn = %d（ルーチンの課題を除いて 3 のはず）", got)
	}
}
