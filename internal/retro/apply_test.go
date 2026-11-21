package retro

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon/internal/ledger"
)

// fakeKanbanApply は list・add・set だけを受ける偽の kanban の CLI を PATH に置く。課題は FAKE_STATE に1行1件
// （id・題・parent・who をタブで区切る）で持ち、add は行を足して新しい id を出す。打った add と set は FAKE_LOG に1行ずつ。
const fakeKanbanApply = `#!/usr/bin/env bash
st=$FAKE_STATE
case $1 in
list) sep=; printf '['
  while IFS=$'\t' read -r id title parent who; do
    printf '%s{"id":"%s","title":"%s","parent":"%s","who":"%s"}' "$sep" "$id" "$title" "$parent" "$who"; sep=,
  done <"$st"; printf ']\n' ;;
add) cat >/dev/null; id="S-00$(( $(wc -l <"$st") + 20 ))"
  printf '%s\t%s\t%s\t\n' "$id" "$2" "$4" >>"$st"; echo "add $2" >>"$FAKE_LOG"; echo "$id" ;;
set) echo "set $2 $3" >>"$FAKE_LOG" ;;
*) exit 9 ;;
esac
`

// kanban add のあと記録の保存で落ちると、記録には足した課題が無い。打ち直しは、kanban にこの振り返りを parent に持つ同じ題の
// 課題を見つけて記録へ写し、同じ課題を重ねて足さない。題が同じでも parent が別の課題は写さない。
func TestApplyRerunAfterSaveFailureDoesNotAddTwice(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	if err := os.WriteFile(filepath.Join(bin, "kanban"), []byte(fakeKanbanApply), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	state, logf := filepath.Join(dir, "state"), filepath.Join(dir, "log")
	t.Setenv("FAKE_STATE", state)
	t.Setenv("FAKE_LOG", logf)
	os.WriteFile(state, []byte("S-0010\t直す A\tS-0001\t\n"), 0o644) // 別の振り返りが足した同じ題の課題
	os.WriteFile(logf, nil, 0o644)

	l := ledger.At(filepath.Join(dir, "sample-site"))
	plans := []plan{
		{Pattern: "p1", Who: "worker", Title: "直す A", Repo: "github.com/example/app", Brief: "# 直す A\n"},
		{Pattern: "p2", Who: "person", Title: "Andon へ伝えるか: B", Brief: "# B\n"},
	}

	// 1回目: p1 を足したあと、記録の保存で落ちる
	first := Applied{Retro: "S-0020"}
	if err := addPlans(l, "S-0020", plans, &first, func() error { return errors.New("書けない") }); err == nil {
		t.Fatal("保存で落ちたのにエラーにならない")
	}
	// 打ち直し: 記録は前の回の保存前のまま（足した課題が無い）
	var again Applied
	if err := addPlans(l, "S-0020", plans, &again, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(logf)
	got := strings.Split(strings.TrimSpace(string(b)), "\n")
	want := []string{"add 直す A", "add Andon へ伝えるか: B", "set S-0022 who=person"}
	if !slices.Equal(got, want) {
		t.Errorf("打った kanban の CLI: %q（want %q）", got, want)
	}
	wantRec := []AppliedIssue{
		{Pattern: "p1", Issue: "S-0021", Who: "worker", Title: "直す A"},
		{Pattern: "p2", Issue: "S-0022", Who: "person", Title: "Andon へ伝えるか: B"},
	}
	if !slices.Equal(again.Issues, wantRec) {
		t.Errorf("記録: %+v（want %+v）", again.Issues, wantRec)
	}
}
