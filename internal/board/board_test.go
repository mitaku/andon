package board

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"

	"go.yaml.in/yaml/v3"
)

// 偽の kanban（ファイルを直に置く）に本物の読む関数と出す関数を通す（ADR-0059 受け入れ条件 1〜4）。

var (
	now   = time.Date(2026, 11, 23, 13, 0, 0, 0, time.UTC)
	since = now.Add(-24 * time.Hour)
	fresh = now.Add(-2 * time.Hour).Format(timeLayout)  // 線の内側
	stale = now.Add(-72 * time.Hour).Format(timeLayout) // 線の外側
)

type fake struct {
	t    *testing.T
	root string
}

func newFake(t *testing.T) *fake {
	t.Helper()
	f := &fake{t: t, root: filepath.Join(t.TempDir(), "sample-site")}
	f.write("kanban/config.yaml", "version: 3\nprefix: S\nchief_of_staff: t1\n")
	return f
}

func (f *fake) write(rel, body string) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// issue は課題を置く（keys は issue.yaml に足す行）。
func (f *fake) issue(id, status, updated string, keys ...string) {
	body := fmt.Sprintf("id: %s\ntitle: タイトル %s\nstatus: %s\nupdated: \"%s\"\n", id, id, status, updated)
	for _, k := range keys {
		body += k + "\n"
	}
	f.write("kanban/issues/"+id+"/issue.yaml", body)
}

func ids(r Result) []string {
	out := []string{}
	for _, it := range r.Issues {
		out = append(out, it["id"].(string))
	}
	return out
}

// rangeFixture は受け入れ条件 1 の見本の kanban と、範囲に入るべき課題（番号の順）・older。
func rangeFixture(t *testing.T) (*fake, []string, int) {
	f := newFake(t)
	// ア: 片付いていない課題と読めない課題
	f.issue("S-0001", "doing", fresh, "parent: S-0010")      // 片付いていない子を持つ古い片付いた親 S-0010
	f.issue("S-0003", "todo", fresh, "after: [S-0030]")      // after の相手が古い片付いた課題
	f.write("kanban/issues/S-0050/issue.yaml", "title: [\n") // 読めない課題
	// イ: 線の内側の片付いた課題
	f.issue("S-0002", "done", fresh, "parent: S-0020", "after: [S-0021]")
	// ウ: アとイの親をたどった先（2 段）
	f.issue("S-0010", "done", stale, "parent: S-0012")
	f.issue("S-0012", "done", stale)
	f.issue("S-0020", "done", stale)
	// エ: アの親をたどった先を親に持つ課題（古い片付いた子）
	f.issue("S-0011", "done", stale, "parent: S-0010")
	f.issue("S-0013", "done", stale, "parent: S-0012")
	// オ: アとイの after の相手
	f.issue("S-0030", "done", stale)
	f.issue("S-0021", "done", stale)
	// どれにも関わらない古い片付いた課題。イの親の子（エはアの親だけ）も入らない
	f.issue("S-0040", "done", stale)
	f.issue("S-0022", "done", stale, "parent: S-0020")
	want := []string{"S-0001", "S-0002", "S-0003", "S-0010", "S-0011", "S-0012", "S-0013", "S-0020", "S-0021", "S-0030", "S-0050"}
	// older: updated が since より前の片付いた課題（範囲に入ったものも数える）
	return f, want, 9
}

// dropped は環境の ANDON_BOARD_DROP（ア〜オの記号 a・i・u・e・o を並べたもの）が抜いた範囲の組（誤りを戻した記録を撮るため）。
func dropped() []Rule {
	var rules []Rule
	drop := os.Getenv("ANDON_BOARD_DROP")
	for _, r := range AllRules {
		if !strings.Contains(drop, string(r)) {
			rules = append(rules, r)
		}
	}
	return rules
}

// 受け入れ条件 1: 範囲はア〜オと同じ顔ぶれで、どれにも関わらない課題を含まない。older は線より前の片付いた課題の数
func TestBoardRange(t *testing.T) {
	f, want, older := rangeFixture(t)
	res, err := board(f.root, since, now, dropped())
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(res); !slices.Equal(got, want) {
		t.Fatalf("issues = %v, want %v", got, want)
	}
	if res.Older == nil || *res.Older != older {
		t.Fatalf("older = %v, want %d", res.Older, older)
	}
	if res.Since != "2026-11-22T13:00:00Z" || res.Done != "done" || res.Project != "sample-site" || res.Version != 1 || res.More != nil {
		t.Fatalf("head = %+v", res)
	}
	if b, ok := res.Issues[len(res.Issues)-1]["broken"].(string); !ok || b == "" {
		t.Fatalf("読めない課題に broken が無い: %v", res.Issues[len(res.Issues)-1])
	}
	if _, err := contract.Encode(contract.SchemaIssueBoard, res); err != nil {
		t.Fatalf("出力が schema に合わない: %v", err)
	}
}

// 受け入れ条件 1 の検査が何かを見ていること: 範囲の組のどれを抜いても、見本の顔ぶれが変わる（エを抜くと落ちる、を含む）
func TestBoardRangeEachRuleMatters(t *testing.T) {
	f, want, _ := rangeFixture(t)
	for _, drop := range AllRules {
		var rules []Rule
		for _, r := range AllRules {
			if r != drop {
				rules = append(rules, r)
			}
		}
		res, err := board(f.root, since, now, rules)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Equal(ids(res), want) {
			t.Errorf("組 %s を抜いても顔ぶれが同じ（検査が組 %s を見ていない）", drop, drop)
		}
	}
}

// 親の循環でも止まり、線の内側の境目（since ちょうど）は内側に入る。updated の読めない片付いた課題は内側に入れる
func TestBoardCycleAndEdges(t *testing.T) {
	f := newFake(t)
	f.issue("S-0001", "doing", fresh, "parent: S-0002")
	f.issue("S-0002", "done", stale, "parent: S-0003")
	f.issue("S-0003", "done", stale, "parent: S-0002")
	f.issue("S-0004", "done", since.Format(timeLayout))
	f.write("kanban/issues/S-0005/issue.yaml", "title: x\nstatus: done\n")
	f.issue("S-0006", "done", stale)
	res, err := Board(f.root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(res), []string{"S-0001", "S-0002", "S-0003", "S-0004", "S-0005"}; !slices.Equal(got, want) {
		t.Fatalf("issues = %v, want %v", got, want)
	}
	if *res.Older != 3 {
		t.Fatalf("older = %d, want 3", *res.Older)
	}
}

func historyFixture(t *testing.T) *fake {
	f := newFake(t)
	f.issue("S-0001", "done", now.Add(-3*time.Hour).Format(timeLayout), "repo: github.com/example/Alpha")
	f.issue("S-0002", "done", now.Add(-1*time.Hour).Format(timeLayout), "profile: Codex-Review")
	f.issue("S-0003", "done", now.Add(-2*time.Hour).Format(timeLayout), "agent: t9QZ")
	f.issue("S-0004", "doing", now.Format(timeLayout), "repo: github.com/example/alpha") // 片付いていない
	f.issue("S-0005", "done", now.Add(-5*time.Hour).Format(timeLayout))
	f.write("kanban/issues/S-0005/issue.yaml", "id: S-0005\ntitle: 返事の下書きボタン\nstatus: done\nupdated: \""+now.Add(-5*time.Hour).Format(timeLayout)+"\"\n")
	return f
}

// 受け入れ条件 2（1/3）: 片付いた課題を updated の新しい順に limit 件、more は残りの数。片付いていない課題は出ない
func TestHistoryLimit(t *testing.T) {
	res, err := History(historyFixture(t).root, 2, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(res), []string{"S-0002", "S-0003"}; !slices.Equal(got, want) {
		t.Fatalf("issues = %v, want %v", got, want)
	}
	if res.More == nil || *res.More != 2 || res.Older != nil || res.Since != "" {
		t.Fatalf("more/older/since = %v %v %q", res.More, res.Older, res.Since)
	}
	if _, err := contract.Encode(contract.SchemaIssueBoard, res); err != nil {
		t.Fatalf("出力が schema に合わない: %v", err)
	}
}

// 受け入れ条件 2（2/3）: --match は番号・タイトル・repo・プロファイル・端末の名前の部分一致で、大文字と小文字を分けない
func TestHistoryMatch(t *testing.T) {
	f := historyFixture(t)
	for _, c := range []struct{ match, want string }{
		{"s-0005", "S-0005"},  // 番号
		{"下書き", "S-0005"},     // タイトル
		{"ALPHA", "S-0001"},   // repo（片付いていない S-0004 は出ない）
		{"codex-r", "S-0002"}, // プロファイル
		{"T9qz", "S-0003"},    // 端末の名前
		{"nothing-here", ""},  // どれにも当たらない
	} {
		res, err := History(f.root, DefaultLimit, c.match, now)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(ids(res), ","); got != c.want || *res.More != 0 {
			t.Errorf("--match %q = %q (more %d), want %q", c.match, got, *res.More, c.want)
		}
	}
}

// 受け入れ条件 2（3/3）: 片付いていない課題・読めない課題は、--match に当たっても出ない。limit が全部より多ければ more は 0
func TestHistoryOnlyDone(t *testing.T) {
	f := historyFixture(t)
	f.write("kanban/issues/S-0009/issue.yaml", "title: [\n")
	res, err := History(f.root, DefaultLimit, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(res), []string{"S-0002", "S-0003", "S-0001", "S-0005"}; !slices.Equal(got, want) || *res.More != 0 {
		t.Fatalf("issues = %v more %d, want %v", got, *res.More, want)
	}
}

const marker = "MARKER-0b6f2c-SECRET"

// naiveRead は守りを外した読み方: symlink をたどり、大きさを見ず、alias を開く（kanban の CLI の読み方と同じ）。
func naiveRead(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return b, nil // 読めなくても中身はここまで来ている
	}
	return json.Marshal(naiveValue(&root))
}

func naiveValue(n *yaml.Node) any {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	switch n.Kind {
	case yaml.DocumentNode:
		return naiveValue(n.Content[0])
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = naiveValue(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		out := []any{}
		for _, c := range n.Content {
			out = append(out, naiveValue(c))
		}
		return out
	}
	return n.Value
}

// aliasBomb は alias を depth 段重ねた issue.yaml（1 段ごとに開いた大きさが倍になる）。
func aliasBomb(depth int) string {
	var b strings.Builder
	b.WriteString("title: x\nstatus: doing\na0: &a0 [" + marker + "]\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "a%d: &a%d [*a%d, *a%d]\n", i, i, i-1, i-1)
	}
	return b.String()
}

func boardJSON(t *testing.T, root string) []byte {
	t.Helper()
	res, err := Board(root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := contract.Encode(contract.SchemaIssueBoard, res)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func brokenOf(t *testing.T, root, id string) string {
	t.Helper()
	res, err := Board(root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Issues {
		if it["id"] == id {
			if len(it) != 2 {
				t.Fatalf("%s の読めない課題に id と broken のほかのキー: %v", id, it)
			}
			b, _ := it["broken"].(string)
			return b
		}
	}
	t.Fatalf("%s が出ない", id)
	return ""
}

// 受け入れ条件 3（1/4）: issue.yaml が symlink の課題は broken で出て、先の目印は出ない（守りを外すと出る）
func TestGuardFileSymlink(t *testing.T) {
	f := newFake(t)
	f.write("outside/issue.yaml", "title: "+marker+"\nstatus: doing\n")
	if err := os.MkdirAll(filepath.Join(f.root, "kanban/issues/S-0001"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.root, "kanban/issues/S-0001/issue.yaml")
	if err := os.Symlink(filepath.Join(f.root, "outside/issue.yaml"), link); err != nil {
		t.Fatal(err)
	}
	if b := brokenOf(t, f.root, "S-0001"); b != "symbolic link" {
		t.Fatalf("broken = %q", b)
	}
	if bytes.Contains(boardJSON(t, f.root), []byte(marker)) {
		t.Fatal("symlink の先の目印が出力に出た")
	}
	if naive, _ := naiveRead(link); !bytes.Contains(naive, []byte(marker)) {
		t.Fatal("守りを外した読み方でも目印が出ない（検査が何も見ていない）")
	}
}

// 受け入れ条件 3（2/4）: フォルダが symlink の課題は broken で出て、先の目印は出ない
func TestGuardFolderSymlink(t *testing.T) {
	f := newFake(t)
	f.write("outside/S-0002/issue.yaml", "title: "+marker+"\nstatus: doing\n")
	if err := os.MkdirAll(filepath.Join(f.root, "kanban/issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.root, "kanban/issues/S-0002")
	if err := os.Symlink(filepath.Join(f.root, "outside/S-0002"), link); err != nil {
		t.Fatal(err)
	}
	if b := brokenOf(t, f.root, "S-0002"); b != "symbolic link" {
		t.Fatalf("broken = %q", b)
	}
	if bytes.Contains(boardJSON(t, f.root), []byte(marker)) {
		t.Fatal("symlink のフォルダの先の目印が出力に出た")
	}
	if naive, _ := naiveRead(filepath.Join(link, "issue.yaml")); !bytes.Contains(naive, []byte(marker)) {
		t.Fatal("守りを外した読み方でも目印が出ない（検査が何も見ていない）")
	}
}

// 受け入れ条件 3（3/4）: 256 KiB を超える issue.yaml の課題は broken で出て、中身は出ない
func TestGuardLarge(t *testing.T) {
	f := newFake(t)
	body := "title: " + marker + "\nstatus: doing\nnote: " + strings.Repeat("x", MaxFile) + "\n"
	f.write("kanban/issues/S-0003/issue.yaml", body)
	if b := brokenOf(t, f.root, "S-0003"); b != "larger than 262144 bytes" {
		t.Fatalf("broken = %q", b)
	}
	if bytes.Contains(boardJSON(t, f.root), []byte(marker)) {
		t.Fatal("大きすぎるファイルの中身が出力に出た")
	}
	if naive, _ := naiveRead(filepath.Join(f.root, "kanban/issues/S-0003/issue.yaml")); !bytes.Contains(naive, []byte(marker)) {
		t.Fatal("守りを外した読み方でも目印が出ない（検査が何も見ていない）")
	}
	// ちょうど上限の大きさは読む
	f.write("kanban/issues/S-0004/issue.yaml", "title: ok\nstatus: doing\nnote: "+strings.Repeat("y", MaxFile-len("title: ok\nstatus: doing\nnote: \n"))+"\n")
	res, err := Board(f.root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Issues {
		if it["id"] == "S-0004" && it["broken"] != nil {
			t.Fatalf("ちょうど上限のファイルを読まない: %v", it["broken"])
		}
	}
}

// 受け入れ条件 3（4/4）: alias を重ねた issue.yaml の課題は broken で出て、置く前とほぼ同じ大きさ（守りを外すと膨らむ）
func TestGuardAlias(t *testing.T) {
	f := newFake(t)
	f.issue("S-0001", "doing", fresh)
	before := len(boardJSON(t, f.root))
	f.write("kanban/issues/S-0005/issue.yaml", aliasBomb(12))
	if b := brokenOf(t, f.root, "S-0005"); b != "has a YAML alias" {
		t.Fatalf("broken = %q", b)
	}
	after := boardJSON(t, f.root)
	if len(after) > before+200 || bytes.Contains(after, []byte(marker)) {
		t.Fatalf("alias の課題で出力が膨らんだ: %d → %d バイト", before, len(after))
	}
	naive, err := naiveRead(filepath.Join(f.root, "kanban/issues/S-0005/issue.yaml"))
	if err != nil || len(naive) < 100*len(aliasBomb(12)) {
		t.Fatalf("守りを外した読み方でも膨らまない（%d バイト。検査が何も見ていない）", len(naive))
	}
}

// snapshot は root の下の全部のファイルの中身（パス → sha256）。
func snapshot(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	m := map[string][32]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			m[p] = sha256.Sum256(b)
		} else {
			m[p+"/"] = [32]byte{}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func run(t *testing.T, main func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := main(args, &out, &errb)
	return rc, out.String(), errb.String()
}

func boardMain(args []string, out, errb *bytes.Buffer) int { return BoardMain(args, nil, out, errb) }
func historyMain(args []string, out, errb *bytes.Buffer) int {
	return HistoryMain(args, nil, out, errb)
}

// 受け入れ条件 4（1/3）: 2 つの command を打つ前とあとで、kanban のファイルの中身と数が変わらず、履歴の行も増えない
func TestReadOnly(t *testing.T) {
	f, _, _ := rangeFixture(t)
	f.write("kanban/log.jsonl", `{"at":"2026-11-23T10:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"doing"}}`+"\n")
	f.write("kanban/issues/S-0001/questions.yaml", "- n: 1\n  text: 問い\n  status: open\n")
	t.Setenv("KANBAN_ROOT", f.root)
	before := snapshot(t, f.root)
	for _, args := range [][]string{
		{"--json", "--since", since.Format(time.RFC3339)},
		{"--since", since.Format(time.RFC3339)},
	} {
		if rc, _, e := run(t, boardMain, args...); rc != 0 {
			t.Fatalf("board %v: rc %d %s", args, rc, e)
		}
	}
	for _, args := range [][]string{{"--json"}, {"--json", "--limit", "1", "--match", "s-00"}, {}} {
		if rc, _, e := run(t, historyMain, args...); rc != 0 {
			t.Fatalf("history %v: rc %d %s", args, rc, e)
		}
	}
	after := snapshot(t, f.root)
	if len(before) != len(after) {
		t.Fatalf("ファイルの数が変わった: %d → %d", len(before), len(after))
	}
	for p, h := range before {
		if after[p] != h {
			t.Fatalf("%s が変わった・消えた", p)
		}
	}
}

// 受け入れ条件 4（2/3）: --since の無い andon issue board は終了コードが 0 でない
func TestBoardNeedsSince(t *testing.T) {
	f, _, _ := rangeFixture(t)
	t.Setenv("KANBAN_ROOT", f.root)
	for _, args := range [][]string{{"--json"}, {}, {"--since"}, {"--since", "2026-11-22"}, {"--since", "yesterday", "--json"}} {
		if rc, out, _ := run(t, boardMain, args...); rc == 0 || out != "" {
			t.Errorf("board %v: rc %d、出力 %q", args, rc, out)
		}
	}
}

// 受け入れ条件 4（3/3）: 知らないフラグを付けた呼び出しは終了コードが 0 でない（黙って読み飛ばさない）
func TestUnknownFlags(t *testing.T) {
	f, _, _ := rangeFixture(t)
	t.Setenv("KANBAN_ROOT", f.root)
	s := since.Format(time.RFC3339)
	for _, args := range [][]string{{"--json", "--since", s, "--all"}, {"--since", s, "--limit", "5"}, {"--since", s, "extra"}} {
		if rc, out, _ := run(t, boardMain, args...); rc == 0 || out != "" {
			t.Errorf("board %v: rc %d、出力 %q", args, rc, out)
		}
	}
	for _, args := range [][]string{{"--json", "--since", s}, {"--all"}, {"--limit", "0"}, {"--limit", "2001"}, {"--limit", "x"}, {"--match"}} {
		if rc, out, _ := run(t, historyMain, args...); rc == 0 || out != "" {
			t.Errorf("history %v: rc %d、出力 %q", args, rc, out)
		}
	}
	if rc, _, e := run(t, historyMain, "--json", "--limit", "2000"); rc != 0 {
		t.Errorf("history --limit 2000 (上限): rc %d %s", rc, e)
	}
}

// --since は小数の秒と時差つきの時刻も受け、出力の since は UTC にする（mod は toISOString の形で渡す）
func TestSinceForms(t *testing.T) {
	f, _, _ := rangeFixture(t)
	t.Setenv("KANBAN_ROOT", f.root)
	for _, c := range []struct{ arg, want string }{
		{"2026-11-22T13:00:00.000Z", "2026-11-22T13:00:00Z"},
		{"2026-11-22T22:00:00+09:00", "2026-11-22T13:00:00Z"},
		{"2026-11-22T13:00:00.5Z", "2026-11-22T13:00:00.5Z"},
	} {
		rc, out, e := run(t, boardMain, "--json", "--since", c.arg)
		if rc != 0 {
			t.Fatalf("--since %s: rc %d %s", c.arg, rc, e)
		}
		var r Result
		if err := json.Unmarshal([]byte(out), &r); err != nil || r.Since != c.want {
			t.Errorf("--since %s: since %q, want %q (%v)", c.arg, r.Since, c.want, err)
		}
	}
}

// kanban の CLI と同じ中身: 値の型（数・真偽・null・時刻の字面）、BOM、空のファイル、mapping でないファイル、issue.yaml の無いフォルダ
func TestValuesLikeKanban(t *testing.T) {
	f := newFake(t)
	f.write("kanban/issues/S-0001/issue.yaml", "\xef\xbb\xbfid: S-0001\ntitle: \"123\"\nstatus: doing\nn: 3\nf: 1.5\nb: true\nz: null\nt: 2026-11-23T10:00:00Z\nlinks: [a, b]\nm: {k: v}\ninf: .inf\n")
	f.write("kanban/issues/S-0002/issue.yaml", "")
	f.write("kanban/issues/S-0003/issue.yaml", "- a\n- b\n")
	if err := os.MkdirAll(filepath.Join(f.root, "kanban/issues/S-0004"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.write("kanban/issues/README", "not an issue")
	res, err := Board(f.root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := contract.Encode(contract.SchemaIssueBoard, res)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := `[{"b":true,"f":1.5,"id":"S-0001","inf":".inf","links":["a","b"],"m":{"k":"v"},"n":3,"status":"doing","t":"2026-11-23T10:00:00Z","title":"123","z":null},` +
		`{"id":"S-0002"},{"broken":"S-0003/issue.yaml is not a mapping","id":"S-0003"},{"broken":"no issue S-0004","id":"S-0004"}]`
	if g, _ := json.Marshal(got.Issues); string(g) != want {
		t.Fatalf("issues =\n%s\nwant\n%s", g, want)
	}
}

// kanban/issues が symlink なら、先を読まずに断る
func TestIssuesDirSymlink(t *testing.T) {
	f := newFake(t)
	f.write("outside/S-0001/issue.yaml", "title: "+marker+"\nstatus: doing\n")
	if err := os.Symlink(filepath.Join(f.root, "outside"), filepath.Join(f.root, "kanban/issues")); err != nil {
		t.Fatal(err)
	}
	if _, err := Board(f.root, since, now); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err = %v", err)
	}
}

// 人が読む出力は 1 課題 1 行と older
func TestText(t *testing.T) {
	f, want, _ := rangeFixture(t)
	res, err := Board(f.root, since, now)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(Text(res), "\n"), "\n")
	if len(lines) != len(want)+1 || !strings.HasPrefix(lines[0], "S-0001 doing タイトル S-0001") || !strings.Contains(lines[len(lines)-1], "9 件") {
		t.Fatalf("text = %q", lines)
	}
}
