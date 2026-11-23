package retro

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/flock"
	"andon/internal/ledger"
)

// fakeKanbanApply は list・add・set・ask・show だけを受ける偽の kanban の CLI を PATH に置く。課題は FAKE_STATE に1行1件
// （id・題・parent・who・review・問いの数を | で区切る。空の欄を潰さないため）で持つ。add は行を足して新しい id を出し、set は who と
// review を覚え、ask は問いの数を1つ増やして <id>#<n> を出す（FAKE_ASK_FAIL があれば落ちる）。打った add・set・ask は FAKE_LOG に1行ずつ。
const fakeKanbanApply = `#!/usr/bin/env bash
st=$FAKE_STATE
put() { awk -F'|' -v OFS='|' -v id="$1" -v f="$2" -v v="$3" '$1==id{$f=v}1' "$st" >"$st.new" && mv "$st.new" "$st"; }
row() { awk -F'|' -v id="$1" '$1==id' "$st"; }
case $1 in
list) sep=; printf '['
  while IFS='|' read -r id title parent who review nq; do
    printf '%s{"id":"%s","title":"%s","parent":"%s","who":"%s","review":"%s","status":"todo"}' "$sep" "$id" "$title" "$parent" "$who" "$review"; sep=,
  done <"$st"; printf ']\n' ;;
add) cat >/dev/null; shift; title= parent=
  while [ $# -gt 0 ]; do case $1 in
    --brief|--repo|--after) shift 2 ;; --parent) parent=$2; shift 2 ;; --) title=$2; shift 2 ;; *) title=$1; shift ;; esac; done
  id="S-00$(( $(wc -l <"$st") + 20 ))"
  printf '%s|%s|%s|||0\n' "$id" "$title" "$parent" >>"$st"; echo "add $title" >>"$FAKE_LOG"; echo "$id" ;;
set) echo "$*" >>"$FAKE_LOG"
  for kv in "${@:3}"; do case $kv in who=*) put "$2" 4 "${kv#who=}" ;; review=*) put "$2" 5 "${kv#review=}" ;; esac; done ;;
ask) cat >/dev/null; echo "ask $2 $4 $5" >>"$FAKE_LOG"
  [ -n "$FAKE_ASK_FAIL" ] && exit 9
  n=$(( $(row "$2" | cut -d'|' -f6) + 1 )); put "$2" 6 "$n"; echo "$2#$n" ;;
show) n=$(row "$2" | cut -d'|' -f6); qs=; for ((i = 0; i < n; i++)); do qs+=${qs:+,}'{}'; done
  printf '{"issue":{"id":"%s"},"questions":[%s]}\n' "$2" "$qs" ;;
*) exit 9 ;;
esac
`

// applyFixture は偽の kanban の CLI を置き、課題の行（state）から始める。課題の置き場と、打った CLI の記録を読む関数を返す。
func applyFixture(t *testing.T, state string) (*ledger.Ledger, func() []string) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	if err := os.WriteFile(filepath.Join(bin, "kanban"), []byte(fakeKanbanApply), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	st, logf := filepath.Join(dir, "state"), filepath.Join(dir, "log")
	t.Setenv("FAKE_STATE", st)
	t.Setenv("FAKE_LOG", logf)
	t.Setenv("FAKE_ASK_FAIL", "")
	os.WriteFile(st, []byte(state), 0o644)
	os.WriteFile(logf, nil, 0o644)
	return ledger.At(filepath.Join(dir, "sample-site")), func() []string {
		b, _ := os.ReadFile(logf)
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
}

// andonPlans は、担当の課題になるパターン p1 と、Andon の repo を持たないプロジェクトでオーナーの課題になるパターン p2 の計画。
func andonPlans() []plan {
	in := &Input{}
	p1 := planOf("S-0020", Pattern{ID: "p1", Summary: "直す A", Target: Target{Kind: "project", Repo: "github.com/example/app", Paths: []string{"AGENTS.md"}}}, in, []string{"github.com/example/app"})
	p2 := planOf("S-0020", Pattern{ID: "p2", Summary: "B", Target: Target{Kind: "andon", Paths: []string{"package/skills/worker/SKILL.md"}}}, in, []string{"github.com/example/app"})
	return []plan{p1, p2}
}

// checkOwnerQuestion は、オーナーの課題 id に置いた問いの控え（library/questions/1.json）が「Andon へ伝えるか」の問いであることを見る。
func checkOwnerQuestion(t *testing.T, l *ledger.Ledger, id string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(l.Library(id), "questions", "1.json"))
	if err != nil {
		t.Fatalf("%s に問いの控えが無い: %v", id, err)
	}
	q, err := contract.ParseQuestion(b)
	if err != nil {
		t.Fatalf("問いの控えの形が違う: %v", err)
	}
	if q.Kind != "promise" || q.From != "retro" || q.Question != "Andon へ伝えるか: B" || len(q.Options) != 2 ||
		q.Options[0].Label != tellLabel || q.Options[1].Label != dropLabel || len(q.Facts) == 0 || q.Facts[0] != "S-0020/library/retro.json のパターン p2" {
		t.Errorf("問い: %+v", q)
	}
}

// kanban add のあと記録の保存で落ちると、記録には足した課題が無い。打ち直しは、kanban にこの振り返りを parent に持つ同じ題の
// 課題を見つけて記録へ写し、同じ課題を重ねて足さない。題が同じでも parent が別の課題は写さない（問いを持つオーナーの課題でも断らない）。
// Andon の repo を持たないプロジェクトのオーナーの課題は、種類 promise・選択肢 2 つ・from が retro の問いと一緒に足す（ADR-0057 受け入れ条件 14）。
func TestApplyRerunAfterSaveFailureDoesNotAddTwice(t *testing.T) {
	// 別の振り返りが足した同じ題の課題（担当の課題と、問いを持つオーナーの課題）。parent が違うので、取り違えも断りもしない
	l, logged := applyFixture(t, "S-0010|直す A|S-0001|||0\nS-0011|Andon へ伝えるか: B|S-0001|person||1\n")
	plans := andonPlans()
	if plans[0].Who != "worker" || plans[0].Question != nil || plans[1].Who != "person" || plans[1].Title != "Andon へ伝えるか: B" || plans[1].Question == nil {
		t.Fatalf("計画: %+v", plans)
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
	// 担当の課題には予約キーを書く（off は reviewer を書かない。profile は後で決まる）。偽の kanban は log.jsonl を書かないので、
	// 打ち直しは前の回の値を照合の外れた既定と見て、同じ値で書き直す（書き直さない場合は ledger の TestUnsettled が見る）
	keys := "set S-0022 review=off must_fix=major merge=person estimate=medium"
	want := []string{"add 直す A", keys, keys, "add Andon へ伝えるか: B", "set S-0023 who=person", "ask S-0023 --kind promise"}
	if got := logged(); !slices.Equal(got, want) {
		t.Errorf("打った kanban の CLI: %q（want %q）", got, want)
	}
	wantRec := []AppliedIssue{
		{Pattern: "p1", Issue: "S-0022", Who: "worker", Title: "直す A"},
		{Pattern: "p2", Issue: "S-0023", Who: "person", Title: "Andon へ伝えるか: B"},
	}
	if !slices.Equal(again.Issues, wantRec) {
		t.Errorf("記録: %+v（want %+v）", again.Issues, wantRec)
	}
	checkOwnerQuestion(t, l, "S-0023")
}

// オーナーの課題を足したあと問いを置く所で落ちた回は、課題を足して記録に残さずに終わる。打ち直すと、その課題を親と題で
// 見つけ、課題を足さずに問いを 1 つだけ置く（ADR-0057 決めたこと 11・受け入れ条件 14）。
func TestApplyRerunAfterAskFailurePutsOnlyTheQuestion(t *testing.T) {
	l, logged := applyFixture(t, "S-0010|ほかの課題||||0\n")
	plans := andonPlans()[1:]
	t.Setenv("FAKE_ASK_FAIL", "1")
	var first Applied
	err := addPlans(l, "S-0020", plans, &first, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "S-0021") {
		t.Fatalf("問いを置けなかったのに、足した課題を名指して落ちない: %v", err)
	}
	if len(first.Issues) != 0 {
		t.Errorf("問いを置けなかった課題を記録に残した: %+v", first.Issues)
	}
	t.Setenv("FAKE_ASK_FAIL", "")
	var again Applied
	if err := addPlans(l, "S-0020", plans, &again, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	want := []string{"add Andon へ伝えるか: B", "set S-0021 who=person", "ask S-0021 --kind promise", "ask S-0021 --kind promise"}
	if got := logged(); !slices.Equal(got, want) {
		t.Errorf("打った kanban の CLI: %q（want %q）", got, want)
	}
	if !slices.Equal(again.Issues, []AppliedIssue{{Pattern: "p2", Issue: "S-0021", Who: "person", Title: "Andon へ伝えるか: B"}}) {
		t.Errorf("記録: %+v", again.Issues)
	}
	if n, err := l.Questions("S-0021"); err != nil || n != 1 {
		t.Errorf("問いの数: %d（%v）。1 つだけ置くはず", n, err)
	}
	checkOwnerQuestion(t, l, "S-0021")
	// もう一度打ち直しても（記録を失った場合）、問いを持つ課題には問いを足さない
	var third Applied
	if err := addPlans(l, "S-0020", plans, &third, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if n, _ := l.Questions("S-0021"); n != 1 || len(logged()) != len(want) {
		t.Errorf("問いを持つ課題に問いを足した: 問い %d、打った CLI %q", n, logged())
	}
}

// 同じ振り返りを当てる apply は1本ずつ。ほかの apply が lock を持っている間は、kanban を照らす前に断る（2本が同時に
// 「まだ無い」と読んで同じ課題を足さない）。--dry-run は何も書かないので lock を取らない。lock が外れれば先へ進む。
func TestApplyRefusesWhileAnotherApplyRuns(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	logf := filepath.Join(dir, "log")
	// kanban の CLI が打たれたら残す（断ったときは打たれない）
	if err := os.WriteFile(filepath.Join(bin, "kanban"), []byte("#!/usr/bin/env bash\necho \"$*\" >>\"$FAKE_LOG\"\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", logf)
	root := filepath.Join(dir, "sample-site")
	l := ledger.At(root)
	unlock, err := flock.TryLock(filepath.Join(l.Library("S-0020"), appliedLock))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runApply(root, "S-0020", false, &out); err == nil || !strings.Contains(err.Error(), "ほかの andon retro apply が当てている") {
		t.Fatalf("ほかの apply が lock を持っているのに断らない: %v", err)
	}
	if b, _ := os.ReadFile(logf); len(b) != 0 {
		t.Errorf("断ったのに kanban の CLI を打った: %s", b)
	}
	if err := runApply(root, "S-0020", true, &out); err == nil || strings.Contains(err.Error(), "ほかの andon retro apply") {
		t.Errorf("--dry-run が lock で断られた（retro.json が無いことで断られるはず）: %v", err)
	}
	unlock()
	if err := runApply(root, "S-0020", false, &out); err == nil || strings.Contains(err.Error(), "ほかの andon retro apply") {
		t.Errorf("lock が外れたのに断られた（retro.json が無いことで断られるはず）: %v", err)
	}
}
