package issue

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
)

// recordRun は andon issue record T-0001 <args…> を打ち、終了コード・stdout・stderr を返す。
func (f *fixture) recordRun(args ...string) (int, string, string) {
	f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
	var out, errb bytes.Buffer
	rc := Main(append([]string{"record", "T-0001"}, args...), strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

func (f *fixture) runsDir() string { return filepath.Join(f.root, "kanban/issues/T-0001/library/runs") }

// readRecord は記録 n を読み、埋め込んだ schema/run.schema.json に通して返す。
func (f *fixture) readRecord(n string) *contract.Run {
	f.t.Helper()
	p := filepath.Join(f.runsDir(), n+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		f.t.Fatalf("記録 %s が無い: %v", n, err)
	}
	if err := contract.SchemaRun.Check(b); err != nil {
		f.t.Fatalf("記録 %s が schema に合わない: %v", n, err)
	}
	r, err := contract.ReadRun(p)
	if err != nil {
		f.t.Fatalf("記録 %s が読めない: %v", n, err)
	}
	return r
}

// TestRecordWritesRun は受け入れ条件 1（ADR-0035 決めたこと 2・11）: 成功する command を渡すと、課題のフォルダに記録
// （終了コード・commit・commit していない変更の有無・時刻・出力のパス）が書かれ、schema を通る。失敗する command では、
// 終了コードが 0 でない記録になる。
func TestRecordWritesRun(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	rc, out, errs := f.recordRun("--", "sh", "-c", "echo 通った; echo 標準エラー >&2")
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	r := f.readRecord("1")
	if r.ID != 1 || r.Expect != "pass" || r.ExitCode != 0 || r.Commit != f.head || r.Dirty == nil || *r.Dirty ||
		r.Output != "runs/1.log" || r.Truncated || r.StartedAt == "" || r.FinishedAt < r.StartedAt ||
		strings.Join(r.Command, " ") != "sh -c echo 通った; echo 標準エラー >&2" {
		t.Fatalf("記録が違う: %+v", r)
	}
	log, _ := os.ReadFile(filepath.Join(f.runsDir(), "1.log"))
	if string(log) != "通った\n標準エラー\n" || r.OutputBytes != int64(len(log)) {
		t.Fatalf("出力が違う: %q（%d バイト）", log, r.OutputBytes)
	}
	if !strings.Contains(out, "通った") || !strings.Contains(out, "T-0001/library/runs/1.json written（期待 pass・終了コード 0・commit "+f.head[:12]+"）") {
		t.Fatalf("端末に流さない・書いた行が無い: %q", out)
	}

	rc, _, errs = f.recordRun("--", "sh", "-c", "echo 落ちた; exit 3")
	if rc != 1 || !strings.Contains(errs, "pass を期待したが終了コード 3（記録 runs/2.json は書いた）") {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("2"); r.ID != 2 || r.ExitCode != 3 || r.Expect != "pass" || r.Output != "runs/2.log" {
		t.Fatalf("落ちた記録が違う: %+v", r)
	}
}

// TestRecordExpectFail は受け入れ条件 2（ADR-0035 決めたこと 8・11）: 落ちることを期待する形で、落ちる command は
// 期待どおりの記録になる。通る command を渡すと、記録の command が 0 でない終了コードで終わる。
// 落ちることを期待する記録は、誤りを戻した木（commit していない変更がある木）で撮ってよい。
func TestRecordExpectFail(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	if err := os.WriteFile(filepath.Join(f.wt, "mistake.txt"), []byte("戻した誤り\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs := f.recordRun("--expect", "fail", "--", "sh", "-c", "exit 1")
	if rc != 0 {
		t.Fatalf("落ちる command で rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("1"); r.Expect != "fail" || r.ExitCode != 1 || r.Dirty == nil || !*r.Dirty || !r.Met() {
		t.Fatalf("記録が違う: %+v", r)
	}
	if strings.Contains(out, "証拠に数えられない") {
		t.Fatalf("fail を期待する記録に、変更のある木の注意を出した: %q", out)
	}

	rc, _, errs = f.recordRun("--expect", "fail", "--", "true")
	if rc != 1 || !strings.Contains(errs, "fail を期待したが終了コード 0（記録 runs/2.json は書いた）") {
		t.Fatalf("通る command で rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("2"); r.Expect != "fail" || r.ExitCode != 0 || r.Met() {
		t.Fatalf("記録が違う: %+v", r)
	}
}

// pass を期待する記録を、commit していない変更のある木で撮ると、書いたうえで証拠に数えられないと言う（ADR-0035 決めたこと 11）。
func TestRecordDirtyPass(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	os.WriteFile(filepath.Join(f.wt, "new.txt"), []byte("x\n"), 0o644)
	rc, out, errs := f.recordRun("--", "true")
	if rc != 0 || !strings.Contains(out, "commit していない変更あり") || !strings.Contains(out, "pass を期待する記録は証拠に数えられない") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if r := f.readRecord("1"); r.Dirty == nil || !*r.Dirty {
		t.Fatalf("dirty が true でない: %+v", r)
	}
}

// repo の無い課題の記録は commit と dirty を持たない。
func TestRecordNoRepo(t *testing.T) {
	f := newFixture(t)
	rc, _, errs := f.recordRun("--", "true")
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("1"); r.Commit != "" || r.Dirty != nil {
		t.Fatalf("repo の無い課題に commit・dirty を書いた: %+v", r)
	}
}

// 起動できない command（打ち間違い）は、落ちた記録にしない。記録も出力も残さず、次の番号も空けない。
func TestRecordCannotStart(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	rc, _, errs := f.recordRun("--expect", "fail", "--", "no-such-command-andon")
	if rc != 1 || !strings.Contains(errs, "command を起動できない（記録は書かない）") {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if left, _ := os.ReadDir(f.runsDir()); len(left) != 0 {
		t.Fatalf("起動できないのに残した: %v", left)
	}
	if rc, _, _ := f.recordRun("--", "true"); rc != 0 || f.readRecord("1").ID != 1 {
		t.Fatalf("次の記録が 1 でない")
	}
}

// 番号は今ある記録と出力の最大 ＋ 1（前の記録を書き直さない）。
func TestRecordNumbering(t *testing.T) {
	f := newFixture(t)
	f.write("kanban/issues/T-0001/library/runs/4.log", "前の出力\n")
	f.write("kanban/issues/T-0001/library/runs/notes.txt", "番号でない\n")
	if rc, _, errs := f.recordRun("--", "true"); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("5"); r.Output != "runs/5.log" {
		t.Fatalf("記録が違う: %+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(f.runsDir(), "4.log")); string(b) != "前の出力\n" {
		t.Fatalf("前の出力を書き換えた: %q", b)
	}
}

// doing でない課題には打てない。使い方の誤りは終了コード 2 で、何も書かない。
func TestRecordRefuses(t *testing.T) {
	cases := []struct {
		name   string
		status string
		args   []string
		rc     int
		want   string
	}{
		{"review の課題", "review", []string{"--", "true"}, 1, "andon issue record は doing の課題にだけ打てる"},
		{"-- が無い", "doing", []string{"true"}, 2, "usage"},
		{"command が無い", "doing", []string{"--"}, 2, "usage"},
		{"知らない期待", "doing", []string{"--expect", "maybe", "--", "true"}, 2, "usage"},
		{"期待の値が無い", "doing", []string{"--expect"}, 2, "usage"},
		{"command が空", "doing", []string{"--", ""}, 2, "usage"},
		{"引数に空がある", "doing", []string{"--", "true", "a", ""}, 2, "usage"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.task["status"] = c.status
			rc, _, errs := f.recordRun(c.args...)
			if rc != c.rc || !strings.Contains(errs, c.want) {
				t.Fatalf("want rc %d with %q, got rc=%d err=%q", c.rc, c.want, rc, errs)
			}
			if _, err := os.Stat(f.runsDir()); err == nil {
				t.Fatal("断ったのに runs/ を作った")
			}
		})
	}
}

// 空の引数は走らせる前に断る。走らせてから記録の形の確かめで断ると、出力だけ残って記録が書かれない。
func TestRecordEmptyArgNotRun(t *testing.T) {
	f := newFixture(t)
	mark := filepath.Join(t.TempDir(), "ran")
	rc, out, errs := f.recordRun("--", "sh", "-c", `echo ran > "$0"`, mark, "")
	if rc != 2 || !strings.Contains(errs, "usage") {
		t.Fatalf("want rc 2 with usage, got rc=%d out=%q err=%q", rc, out, errs)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("空の引数があるのに command を走らせた")
	}
	if _, err := os.Stat(f.runsDir()); err == nil {
		t.Fatal("断ったのに runs/ を作った")
	}
}

// 記録は worktree で走らせる（cwd ではない）。
func TestRecordRunsInWorktree(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	t.Chdir(t.TempDir())
	if rc, _, errs := f.recordRun("--", "pwd"); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	log, _ := os.ReadFile(filepath.Join(f.runsDir(), "1.log"))
	want, _ := filepath.EvalSymlinks(f.wt)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(log)))
	if got != want {
		t.Fatalf("worktree で走らせていない: %q（want %q）", got, want)
	}
}

// 出力は上限を超えたら終わりの側を残し、先頭に切った量の1行を置く。文字の途中から始めない。
func TestTailBuffer(t *testing.T) {
	tb := &tailBuffer{limit: 8}
	tb.Write([]byte("abc"))
	if string(tb.bytes()) != "abc" || tb.cut() != 0 || tb.total != 3 {
		t.Fatalf("上限の中で切った: %q", tb.bytes())
	}
	for _, s := range []string{"0123", "4567", "89", "あいう"} { // 「あいう」は 9 バイト
		tb.Write([]byte(s))
	}
	got := string(tb.bytes())
	if tb.total != 22 || tb.cut() != 14 || !strings.HasSuffix(got, "\n"+"いう") ||
		!strings.HasPrefix(got, "（出力が 22 バイトで上限 8 バイトを超えたので、先頭の 16 バイトを切った）\n") {
		t.Fatalf("切り方が違う: total=%d cut=%d %q", tb.total, tb.cut(), got)
	}
}

// 大きい出力でも記録は上限で切って書き、truncated と全体のバイト数を残す。
func TestRecordTruncates(t *testing.T) {
	if _, err := exec.LookPath("head"); err != nil {
		t.Skip("head が無い")
	}
	f := newFixture(t)
	rc, _, errs := f.recordRun("--", "sh", "-c", "head -c 1500000 /dev/zero | tr '\\0' x; echo; echo 終わり")
	if rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	r := f.readRecord("1")
	log, _ := os.ReadFile(filepath.Join(f.runsDir(), "1.log"))
	if !r.Truncated || r.OutputBytes != int64(1500000+1+len("終わり\n")) || !strings.HasSuffix(string(log), "x\n終わり\n") || len(log) > recordLimit+200 {
		t.Fatalf("切り方が違う: %+v（log %d バイト）", r, len(log))
	}
}

// linkedWorktree は fixture の repo から linked worktree を切り、課題の worktree をそこへ向ける（案件の repo の worktree と同じ形）。
// repo には .gitignore が無い。利用者の git の設定（core.excludesFile）が結果を変えないよう、設定を読ませない。
func (f *fixture) linkedWorktree() string {
	f.t.Helper()
	f.t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	f.t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	linked := filepath.Join(filepath.Dir(f.wt), "linked")
	if out, err := exec.Command("git", "-C", f.wt, "worktree", "add", "-q", "-b", "case", linked).CombinedOutput(); err != nil {
		f.t.Fatalf("git worktree add: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(linked, ".gitignore")); err == nil {
		f.t.Fatal("worktree に .gitignore がある（案件の repo の形でない）")
	}
	f.task["worktree"] = linked
	return linked
}

// yomiyasu の状態フォルダ（git の最上位の .yomiyasu/）は、yomiyasu が中に置く .gitignore（中身は *）で git から外れる。
// 案件の repo の worktree（.gitignore が無い）でも、記録はそれを commit していない変更に数えない。
// 検査が当たる場合があること: 中の .gitignore が無いフォルダは、commit していない変更に数える。
func TestRecordIgnoresYomiyasuState(t *testing.T) {
	for _, c := range []struct {
		name      string
		gitignore bool
		dirty     bool
	}{{"中に .gitignore がある", true, false}, {"中に .gitignore が無い", false, true}} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.task["repo"] = "github.com/example/app"
			state := filepath.Join(f.linkedWorktree(), ".yomiyasu")
			files := map[string]string{"state.json": `{"format": "yomiyasu-state", "format_version": 1}` + "\n",
				"inputs/a.md": "原文の写し\n", "runs/r1/record.json": "{}\n"}
			if c.gitignore {
				files[".gitignore"] = "# yomiyasu の検査記録。git に入れない。\n*\n"
			}
			for name, body := range files {
				p := filepath.Join(state, name)
				if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if rc, _, errs := f.recordRun("--", "true"); rc != 0 {
				t.Fatalf("rc=%d err=%q", rc, errs)
			}
			if r := f.readRecord("1"); r.Dirty == nil || *r.Dirty != c.dirty {
				t.Fatalf("dirty が %v でない: %+v", c.dirty, r)
			}
		})
	}
}

// 本物の yomiyasu を案件の repo の worktree で走らせても、記録は commit していない変更に数えない。
// yomiyasu はパッケージに入らない外の skill なので、ANDON_TEST_YOMIYASU_CHECK に yomiyasu_check.py のパスを渡したときだけ走る。
func TestRecordIgnoresRealYomiyasu(t *testing.T) {
	check := os.Getenv("ANDON_TEST_YOMIYASU_CHECK")
	if check == "" {
		t.Skip("ANDON_TEST_YOMIYASU_CHECK が無い（yomiyasu_check.py のパスを渡すと走る）")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 が無い")
	}
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	linked := f.linkedWorktree()
	t.Setenv("YOMIYASU_STATE_DIR", "") // 既定の置き場（git の最上位の .yomiyasu/）に書かせる
	for _, name := range []string{"source.md", "final.md"} {
		if err := os.WriteFile(filepath.Join(linked, name), []byte("# 見出し\n\n推敲の文です。\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "source.md", "final.md"}, {"commit", "-q", "-m", "原稿"}} {
		c := exec.Command("git", append([]string{"-C", linked}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run := exec.Command(python, check, "run", "--final", "final.md", "--source", "source.md")
	run.Dir = linked
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("yomiyasu_check.py run: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(linked, ".yomiyasu", "state.json")); err != nil {
		t.Fatalf("yomiyasu が worktree の最上位に .yomiyasu/ を作っていない: %v", err)
	}
	if rc, _, errs := f.recordRun("--", "true"); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if r := f.readRecord("1"); r.Dirty == nil || *r.Dirty {
		t.Fatalf("yomiyasu の記録を commit していない変更に数えた: %+v", r)
	}
}
