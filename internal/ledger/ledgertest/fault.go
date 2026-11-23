package ledgertest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// faultScript は本物の kanban の CLI を包む偽の kanban の CLI。読むだけの command はそのまま本物に渡す。
// 書く command は1回につき止まる所を3つ数える（ADR-0057 のあとの振り返りで拾った、途中で止まったあとの打ち直しの穴を見る）:
//  1. 打つ前（何も書かずに落ちる）
//  2. 中（本物は課題のファイル〈issue.yaml・brief.md・questions.yaml〉を書いてから log.jsonl に足す。その間で止まった形にする:
//     本物を最後まで走らせてから log.jsonl を打つ前の中身に戻して落ちる）
//  3. 後（本物を最後まで走らせてから落ちる。呼び手が返事を受けて次を書く前に止まった形）
//
// 数えた所の番号は KANBAN_FAULT_COUNT のファイルに残し、KANBAN_FAULT_AT 番目の所で落ちる（0 なら落ちない）。
const faultScript = `#!/usr/bin/env bash
real=$KANBAN_FAULT_REAL
case $1 in
show|list|ready|root|questions|lint|routines|help|--help|version) exec "$real" "$@" ;;
esac
# log.jsonl の置き場が分からなければ、中で止めた形を作れないので打たずに落ちる（黙って止まり方を変えない）
[ -n "$KANBAN_ROOT" ] || { echo "kanban (fault): KANBAN_ROOT が無い" >&2; exit 64; }
count=$KANBAN_FAULT_COUNT at=${KANBAN_FAULT_AT:-0}
n=$(cat "$count" 2>/dev/null); n=${n:-0}
log=$KANBAN_ROOT/kanban/log.jsonl
step() { n=$((n + 1)); echo "$n" >"$count"; [ "$n" = "$at" ]; }
if step; then echo "kanban: stopped before $1 (fault $n)" >&2; exit 75; fi
if step; then
  saved=$(mktemp)
  had=0; if [ -f "$log" ]; then cp "$log" "$saved"; had=1; fi
  "$real" "$@" >/dev/null; rc=$?
  if [ "$had" = 1 ]; then cp "$saved" "$log"; else rm -f "$log"; fi
  rm -f "$saved"
  echo "kanban: stopped between the files and log.jsonl of $1 (fault $n, rc $rc)" >&2; exit 75
fi
if step; then "$real" "$@" >/dev/null; echo "kanban: stopped after $1 (fault $n)" >&2; exit 75; fi
exec "$real" "$@"
`

// Fault は途中で止まる kanban の CLI（faultScript）。PATH の先頭に置き、コードが打つ kanban の CLI をこれにする。
// 本物（Real）は repo の cmd/kanban を build したもので、検査の下ごしらえ（kanban init など）は本物で打つ（数えない）。
type Fault struct {
	t     testing.TB
	Real  string // 本物の kanban の CLI
	count string
}

// NewFault は本物の kanban の CLI を build し、包んだ偽の kanban の CLI を PATH の先頭に置く。go が無ければ飛ばす。
func NewFault(t testing.TB) *Fault {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash が無い")
	}
	real := filepath.Join(Kanban(t), "kanban")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "kanban"), []byte(faultScript), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &Fault{t: t, Real: real, count: filepath.Join(bin, "count")}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KANBAN_FAULT_REAL", real)
	t.Setenv("KANBAN_FAULT_COUNT", f.count)
	t.Setenv("KANBAN_FAULT_AT", "0")
	return f
}

// Arm は数えた所を 0 に戻し、n 番目の所で落ちるようにする（0 なら落ちない）。
func (f *Fault) Arm(n int) {
	f.t.Helper()
	if err := os.WriteFile(f.count, []byte("0\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Setenv("KANBAN_FAULT_AT", strconv.Itoa(n)); err != nil {
		f.t.Fatal(err)
	}
}

// Points は Arm のあと数えた所の数。
func (f *Fault) Points() int {
	f.t.Helper()
	b, err := os.ReadFile(f.count)
	if err != nil {
		f.t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		f.t.Fatal(err)
	}
	return n
}

// Kanban は本物の kanban の CLI を root（kanban/ の親）に向けて打つ（下ごしらえ用。数えない）。env は足す環境。
func (f *Fault) Kanban(root, stdin string, env []string, args ...string) string {
	f.t.Helper()
	c := exec.Command(f.Real, args...)
	c.Env = append(append(os.Environ(), "KANBAN_ROOT="+root), env...)
	c.Stdin = strings.NewReader(stdin)
	out, err := c.CombinedOutput()
	if err != nil {
		f.t.Fatalf("kanban %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// Case は、kanban に順に書く道を、どの所で止めても打ち直せば止めなかった回と同じ記録と課題に揃うかを見る1件。
type Case struct {
	// Setup はまっさらな kanban を用意する（毎回呼ぶ。t.TempDir で新しい置き場を作る）。
	Setup func(t *testing.T)
	// Run は道を1回打つ（止まれば誤りを返す）。
	Run func() error
	// Retry は止まったあとの打ち直し（無ければ Run）。Tries 回まで、誤りが無くなるまで打つ。
	Retry func() error
	Tries int
	// State は記録と課題を比べられる文字列にする（時刻のように回ごとに変わるものは入れない）。
	State func(t *testing.T) string
	// After は Retry の代わりに、止まった直後の形を見る（打ち直せない道。n は止めた所の番号）。
	After func(t *testing.T, n int)
}

// Converge は、まず止めずに打って所の数と揃うべき形を得て、所を1つずつ進めて止め、打ち直したあと同じ形に揃うかを見る。
// 止めた回が誤りを返さない（落ちたのに黙って進んだ）ときも落とす。
func (f *Fault) Converge(t *testing.T, c Case) {
	t.Helper()
	c.Setup(t)
	f.Arm(0)
	if err := c.Run(); err != nil {
		t.Fatalf("止めずに打った回が落ちた: %v", err)
	}
	total := f.Points()
	var want string
	if c.State != nil {
		want = c.State(t)
	}
	if total == 0 {
		t.Fatal("kanban に1回も書いていない（偽の kanban の CLI を通っていない）")
	}
	retry := c.Retry
	if retry == nil {
		retry = c.Run
	}
	tries := max(c.Tries, 1)
	for n := 1; n <= total; n++ {
		t.Run(fmt.Sprintf("stop %d of %d", n, total), func(t *testing.T) {
			c.Setup(t)
			f.Arm(n)
			if err := c.Run(); err == nil {
				t.Fatalf("%d 番目の所で止めたのに、誤りを返さずに進んだ", n)
			}
			f.Arm(0)
			if c.After != nil {
				c.After(t, n)
				return
			}
			var err error
			for range tries {
				if err = retry(); err == nil {
					break
				}
			}
			if err != nil {
				t.Fatalf("%d 番目の所で止めたあと、%d 回打ち直しても通らない: %v", n, tries, err)
			}
			if got := c.State(t); got != want {
				t.Errorf("%d 番目の所で止めて打ち直した形が、止めなかった回と違う:\n%s", n, lineDiff(want, got))
			}
		})
	}
}

// lineDiff は want と got の違う行を -（want だけ）と +（got だけ）で並べる。
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for _, l := range w {
		if !slices.Contains(g, l) {
			b.WriteString("- " + l + "\n")
		}
	}
	for _, l := range g {
		if !slices.Contains(w, l) {
			b.WriteString("+ " + l + "\n")
		}
	}
	if b.Len() == 0 {
		return "（同じ行の並びが違う）\n--- want\n" + want + "\n--- got\n" + got
	}
	return b.String()
}

// Snapshot は kanban の課題と問い、課題の library/questions/ の控えを、比べられる文字列にする（時刻と端末は入れない）。
// trusted は課題 id の予約キー key を最後に書いたのが Chief of Staff の端末か（呼び手が ledger.Trusted で答える。
// この package は ledger を import できない）。
func (f *Fault) Snapshot(root string, trusted func(id, key string) bool) string {
	f.t.Helper()
	var issues []map[string]any
	if err := json.Unmarshal([]byte(f.Kanban(root, "", nil, "list", "--all", "--json")), &issues); err != nil {
		f.t.Fatal(err)
	}
	var b strings.Builder
	for _, is := range issues {
		id, _ := is["id"].(string)
		fmt.Fprintf(&b, "issue %s title=%v status=%v parent=%v who=%v only_owner=%v\n", id, is["title"], is["status"], is["parent"], is["who"], is["only_owner"])
		for _, k := range SettingKeys {
			if v, ok := is[k]; ok {
				fmt.Fprintf(&b, "  %s=%v trusted=%v\n", k, v, trusted(id, k))
			}
		}
		var show struct {
			Questions []map[string]any `json:"questions"`
			Broken    string           `json:"questionsBroken"`
		}
		if err := json.Unmarshal([]byte(f.Kanban(root, "", nil, "show", id, "--json")), &show); err != nil {
			f.t.Fatal(err)
		}
		if show.Broken != "" {
			fmt.Fprintf(&b, "  questions broken: %s\n", show.Broken)
		}
		for _, q := range show.Questions {
			fmt.Fprintf(&b, "  question %v kind=%v status=%v text=%q\n", q["n"], q["kind"], q["status"], q["text"])
		}
		files, _ := filepath.Glob(filepath.Join(root, "kanban", "issues", id, "library", "questions", "*.json"))
		slices.Sort(files)
		for _, p := range files {
			c, err := os.ReadFile(p)
			if err != nil {
				f.t.Fatal(err)
			}
			fmt.Fprintf(&b, "  library/questions/%s %s\n", filepath.Base(p), strings.Join(strings.Fields(string(c)), " "))
		}
	}
	return b.String()
}

// SettingKeys は Snapshot が並べる予約キー（contract.md §2）。
var SettingKeys = []string{"review", "must_fix", "reviewer", "review_rounds", "merge", "estimate"}
