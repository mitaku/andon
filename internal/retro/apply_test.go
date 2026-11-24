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

const fakeKanbanApply = `#!/usr/bin/env bash
st=$FAKE_STATE
put() { awk -F'|' -v OFS='|' -v id="$1" -v f="$2" -v v="$3" '$1==id{$f=v}1' "$st" >"$st.new" && mv "$st.new" "$st"; }
row() { awk -F'|' -v id="$1" '$1==id' "$st"; }
case $1 in
list) sep=; printf '['
  while IFS='|' read -r id title parent who review nq later; do
    printf '%s{"id":"%s","title":"%s","parent":"%s","who":"%s","review":"%s","later":"%s","status":"todo"}' "$sep" "$id" "$title" "$parent" "$who" "$review" "$later"; sep=,
  done <"$st"; printf ']\n' ;;
add) cat >/dev/null; shift; title= parent=
  while [ $# -gt 0 ]; do case $1 in
    --brief|--repo|--after) shift 2 ;; --parent) parent=$2; shift 2 ;; --) title=$2; shift 2 ;; *) title=$1; shift ;; esac; done
  id="S-00$(( $(wc -l <"$st") + 20 ))"
  printf '%s|%s|%s|||0\n' "$id" "$title" "$parent" >>"$st"; echo "add $title" >>"$FAKE_LOG"; echo "$id" ;;
set) echo "$*" >>"$FAKE_LOG"
  for kv in "${@:3}"; do case $kv in who=*) put "$2" 4 "${kv#who=}" ;; later=*) put "$2" 7 "${kv#later=}" ;; review=*) put "$2" 5 "${kv#review=}" ;; esac; done ;;
ask) text=$(cat); echo "ask $2 $4 $5" >>"$FAKE_LOG"
  [ -n "$FAKE_ASK_FAIL" ] && exit 9
  n=$(( $(row "$2" | cut -d'|' -f6) + 1 )); put "$2" 6 "$n"
  [ "$n" = 1 ] && printf '%s' "$text" >"$st.q.$2"
  echo "$2#$n" ;;
show) n=$(row "$2" | cut -d'|' -f6); qs=
  for ((i = 1; i <= n; i++)); do
    q='{}'; [ "$i" = 1 ] && q="{\"n\":1,\"text\":$(jq -Rs . <"$st.q.$2")}"
    qs+=${qs:+,}$q
  done
  printf '{"issue":{"id":"%s"},"questions":[%s]}\n' "$2" "$qs" ;;
*) exit 9 ;;
esac
`

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

func andonPlans() []plan {
	in := &Input{}
	p1 := planOf("S-0020", Pattern{ID: "p1", Summary: "直す A", Target: Target{Kind: "project", Repo: "github.com/example/app", Paths: []string{"AGENTS.md"}}}, in, []string{"github.com/example/app"}, "")
	p2 := planOf("S-0020", Pattern{ID: "p2", Summary: "B", Target: Target{Kind: "andon", Paths: []string{"package/skills/worker/SKILL.md"}}}, in, []string{"github.com/example/app"}, "")
	return []plan{p1, p2}
}

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

// kanban add のあと記録の保存で落ちると、記録には足した課題が無い。打ち直しは、kanban にこの振り返りを parent に持つ同じタイトルの
// 課題を見つけて記録へ写し、同じ課題を重ねて足さない。タイトルが同じでも parent が別の課題は写さない（問いを持つオーナーの課題でも断らない）。
// Andon の repo を持たないプロジェクトのオーナーの課題は、種類 promise・選択肢 2 つ・from が retro の問いと一緒に足す（ADR-0057 受け入れ条件 14）。
func TestApplyRerunAfterSaveFailureDoesNotAddTwice(t *testing.T) {
	l, logged := applyFixture(t, "S-0010|直す A|S-0001|||0\nS-0011|Andon へ伝えるか: B|S-0001|person||1\n")
	plans := andonPlans()
	if plans[0].Who != "worker" || plans[0].Question != nil || plans[1].Who != "person" || plans[1].Title != "Andon へ伝えるか: B" || plans[1].Question == nil {
		t.Fatalf("計画: %+v", plans)
	}

	first := Applied{Retro: "S-0020"}
	if err := addPlans(l, "S-0020", plans, &first, func() error { return errors.New("書けない") }); err == nil {
		t.Fatal("保存で落ちたのにエラーにならない")
	}
	var again Applied
	if err := addPlans(l, "S-0020", plans, &again, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	keys := "set S-0022 review=off must_fix=major merge=person estimate=medium"
	want := []string{"add 直す A", keys, keys, "add Andon へ伝えるか: B", "set S-0023 who=person later=" + laterReason, "ask S-0023 --kind promise"}
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

// オーナーの課題を足したあと問いを置く所で落ちた回は、課題を足して記録に残さずに終わる。打ち直すと、その課題を親とタイトルで
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
	want := []string{"add Andon へ伝えるか: B", "set S-0021 who=person later=" + laterReason, "ask S-0021 --kind promise", "ask S-0021 --kind promise"}
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
	var third Applied
	if err := addPlans(l, "S-0020", plans, &third, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if n, _ := l.Questions("S-0021"); n != 1 || len(logged()) != len(want) {
		t.Errorf("問いを持つ課題に問いを足した: 問い %d、打った CLI %q", n, logged())
	}
}

func TestApplyRefusesWhileAnotherApplyRuns(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	logf := filepath.Join(dir, "log")
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

func TestApplyRerunRestoresMissingQuestionJSON(t *testing.T) {
	l, logged := applyFixture(t, "S-0010|ほかの課題||||0\n")
	plans := andonPlans()[1:]
	block := filepath.Join(l.Library("S-0021"), "questions")
	os.MkdirAll(filepath.Dir(block), 0o755)
	os.WriteFile(block, nil, 0o644)
	var first Applied
	if err := addPlans(l, "S-0020", plans, &first, func() error { return nil }); err == nil {
		t.Fatal("問いの控えを書けなかったのにエラーにならない")
	}
	os.Remove(block)
	var again Applied
	if err := addPlans(l, "S-0020", plans, &again, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if n, _ := l.Questions("S-0021"); n != 1 {
		t.Errorf("問いの数: %d（1 つのまま）。打った CLI %q", n, logged())
	}
	checkOwnerQuestion(t, l, "S-0021")
}

func TestPlanEvidencePaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kanban")
	issues := filepath.Join(dir, "issues")
	files := map[string]string{
		"A-0283/report.md":                 "担当の報告の本文",
		"A-0283/library/journal.md":        "日報の本文",
		"A-0283-r1/report.md":              "レビュアーの報告の本文",
		"A-0283-r1/library/findings.json":  `{"commit":"x","findings":[]}`,
		"A-0283-r10/library/findings.json": `{"commit":"y","findings":[]}`,
		"A-0283-r2/report.md":              "2 回目の報告",
		"A-02830-r1/report.md":             "番号の違う課題のレビュアー",
		"A-0283-rx/report.md":              "回の形でない",
		"A-0219/report.md":                 "担当の報告",
		"A-0219-r1/report.md":              "レビュアーの報告",
		"A-0219-r1/library/findings.json":  `{"commit":"z","findings":[]}`,
	}
	for rel, body := range files {
		p := filepath.Join(issues, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr := "https://github.com/example/app/pull/468"
	in := &Input{
		Summaries: []Summary{{ID: "A-0283", Title: "mod の検査で直し直した", DoneAt: "2026-11-21T23:48:16Z", PR: &pr}},
		Issues:    []Bundle{{ID: "A-0219", Title: "検査の穴が素通りした", DoneAt: "2026-11-21T08:51:53Z", Reviews: []Review{{ID: "A-0219-r1"}}}},
	}
	p := Pattern{ID: "p6", Summary: "根拠に中身へ辿る道が無い", Evidence: []string{"A-0283", "A-0219"},
		Target: Target{Kind: "project", Repo: "github.com/example/app", Paths: []string{"internal/retro/apply.go"}}}
	pl := planOf("A-0372", p, in, []string{"github.com/example/app"}, dir)

	lines := strings.Split(pl.Brief, "\n")
	pathsAfter := func(head string) string {
		for i, l := range lines {
			if strings.HasPrefix(l, head) && i+1 < len(lines) {
				return lines[i+1]
			}
		}
		t.Fatalf("依頼書に %q の行が無い:\n%s", head, pl.Brief)
		return ""
	}
	at := func(rel ...string) string {
		var out []string
		for _, r := range rel {
			out = append(out, filepath.Join(issues, r))
		}
		return "  中身の置き場（本文は写さない）: " + strings.Join(out, " ")
	}
	if got, want := pathsAfter("- A-0283: "), at("A-0283/report.md", "A-0283/library/journal.md", "A-0283-r1/report.md",
		"A-0283-r1/library/findings.json", "A-0283-r2/report.md", "A-0283-r10/library/findings.json"); got != want {
		t.Errorf("要約の束の根拠の次の行:\n got %q\nwant %q", got, want)
	}
	if got, want := pathsAfter("- A-0219: "), at("A-0219/report.md", "A-0219-r1/report.md", "A-0219-r1/library/findings.json"); got != want {
		t.Errorf("全文の束の根拠の次の行（日報は無いので載せない。レビュアーの子は1回だけ）:\n got %q\nwant %q", got, want)
	}
	for _, body := range files {
		if strings.Contains(pl.Brief, body) {
			t.Errorf("依頼書に記録の本文 %q を写した", body)
		}
	}
	if pl := planOf("A-0372", p, in, []string{"github.com/example/app"}, ""); strings.Contains(pl.Brief, "中身の置き場") {
		t.Errorf("置き場が空なのにパスの行を書いた:\n%s", pl.Brief)
	}
}

// 課題を足したあと who と印を書く前に落ちた回は、打ち直しが同じ親とタイトルの課題を見つけ、課題を足さずに who: person と
// 置いておく印 later を同じ kanban set で書いてから問いを置く（ADR-0062 決めたこと 10）。
// 印を書かない前のバージョンが who: person だけ書いて、問いを置く前に落ちた課題も、打ち直しで印を書く（ADR-0062 受け入れ条件 13）
func TestApplyRerunWritesLaterOnPersonWithoutMark(t *testing.T) {
	l, logged := applyFixture(t, "S-0021|Andon へ伝えるか: B|S-0020|person||0\n")
	var rec Applied
	if err := addPlans(l, "S-0020", andonPlans()[1:], &rec, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	want := []string{"set S-0021 who=person later=" + laterReason, "ask S-0021 --kind promise"}
	if got := logged(); !slices.Equal(got, want) {
		t.Errorf("打った kanban の CLI: %q（want %q）", got, want)
	}
	// 印がもうあれば書き直さない
	l, logged = applyFixture(t, "S-0021|Andon へ伝えるか: B|S-0020|person||0|"+laterReason+"\n")
	rec = Applied{}
	if err := addPlans(l, "S-0020", andonPlans()[1:], &rec, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := logged(); !slices.Equal(got, []string{"ask S-0021 --kind promise"}) {
		t.Errorf("印のある課題で打った kanban の CLI: %q", got)
	}
}

func TestApplyRerunWritesWhoAndLaterTogether(t *testing.T) {
	l, logged := applyFixture(t, "S-0021|Andon へ伝えるか: B|S-0020|||0\n")
	var rec Applied
	if err := addPlans(l, "S-0020", andonPlans()[1:], &rec, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	want := []string{"set S-0021 who=person later=" + laterReason, "ask S-0021 --kind promise"}
	if got := logged(); !slices.Equal(got, want) {
		t.Errorf("打った kanban の CLI: %q（want %q）", got, want)
	}
}
