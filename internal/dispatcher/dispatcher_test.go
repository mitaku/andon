package dispatcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func TestRetry(t *testing.T) {
	for _, c := range []struct {
		name    string
		prev    Reviewer
		seen    bool
		limit   int
		row     int
		trusted bool
		redo    bool
		want    bool
	}{
		{"初めて", Reviewer{}, false, 2, 5, true, false, true},
		{"起動した（二重に起動しない）", Reviewer{Started: true, Row: 5}, true, 2, 5, true, false, false},
		{"起動した回を同じ commit で頼み直した（二重に起動しない）", Reviewer{Started: true, Row: 5}, true, 2, 9, true, false, false},
		{"起動しそこねた（続きから）", Reviewer{Failed: "fetch", ReviewerIssue: "D-2", Row: 5}, true, 2, 5, true, false, true},
		{"設定で起動しない", Reviewer{Skipped: "review が on でない", Row: 5}, true, 5, 5, true, false, false},
		{"起動しなかった回を同じ commit で頼み直した", Reviewer{Skipped: "review が on でない", Row: 5}, true, 5, 9, true, false, true},
		{"前のバージョンの控え（初めて見た kanban で控えただけ）", Reviewer{Skipped: "初めて見た kanban"}, true, 2, 5, true, false, true},
		{"上限を超えた・上限そのまま", Reviewer{Skipped: "追加の回", OverLimit: 1, Row: 5}, true, 1, 5, true, false, false},
		{"上限を超えた・上限が上がった", Reviewer{Skipped: "追加の回", OverLimit: 1, Row: 5}, true, 2, 5, true, false, true},
		{"照合で見送った・照合が外れたまま", Reviewer{Skipped: "最後に書いたのが Chief of Staff でない", Untrusted: true, Row: 5}, true, 2, 5, false, false, false},
		{"照合で見送った・Chief of Staff が書き直して照合が戻った", Reviewer{Skipped: "最後に書いたのが Chief of Staff でない", Untrusted: true, Row: 5}, true, 2, 5, true, false, true},
		{"照合の外の理由で見送った（照合が合っていても拾い直さない）", Reviewer{Skipped: "report.json に pr が無い", Row: 5}, true, 2, 5, true, false, false},
		{"起動した回は照合が戻っても二重に起動しない", Reviewer{Started: true, Untrusted: true, Row: 5}, true, 2, 5, true, false, false},
		{"起動した回が結果を残さずに返った・まだ頼み直していない", Reviewer{Started: true, ReviewerIssue: "D-1-r1", Row: 5}, true, 2, 5, true, true, false},
		{"起動した回が結果を残さずに返った・同じ commit で頼み直した（起動し直す）", Reviewer{Started: true, ReviewerIssue: "D-1-r1", Row: 5}, true, 2, 9, true, true, true},
	} {
		if got := Retry(c.prev, c.seen, c.limit, c.row, c.trusted, c.redo); got != c.want {
			t.Errorf("%s: Retry = %v, want %v", c.name, got, c.want)
		}
	}
}

// 控えに無い review_request（初めて見た kanban）は、kanban にその commit のレビュアーの子課題が無ければ起動しに行き、
// あれば引き継ぐ（todo でなければもう受けている。todo なら端末を照らし、live なら address を引き継ぐ）。
func TestAdopt(t *testing.T) {
	const c1, c2 = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	briefs := map[string]string{
		"D-2": "対象: worktree `x`（detached）。commit " + c1 + "。差分は",
		"D-3": "対象: worktree `y`（detached）。commit " + c2 + "。差分は",
	}
	brief := func(rev string) string { return briefs[rev] }
	tasks := []ledger.Issue{
		{"id": "D-1", "status": "doing", "profile": "claude"},
		{"id": "D-2", "status": "done", "profile": "codex-review", "parent": "D-1", "agent": "t9"},
		{"id": "D-3", "status": "todo", "profile": "codex-review", "parent": "D-1"},
		{"id": "D-4", "status": "doing", "profile": "claude", "parent": "D-1"}, // レビュアーでない子
	}
	none := func(string) (string, int, bool) { return "", 0, false }
	kept := func(string) bool { return false } // どの子課題もその commit の結果を残している
	if _, ok := Adopt(tasks, "D-1", "3333333333333333333333333333333333333333", 1, brief, none, kept); ok {
		t.Error("子課題の無い commit を引き継いだ（起動しに行かない）")
	}
	if r, ok := Adopt(tasks, "D-1", c1, 1, brief, none, kept); !ok || !r.Started || r.ReviewerIssue != "D-2" || r.Address != "t9" {
		t.Errorf("受けた子課題: %+v %v（もう受けているはず）", r, ok)
	}
	if r, ok := Adopt(tasks, "D-1", c2, 1, brief, none, kept); !ok || r.Started || r.ReviewerIssue != "D-3" || r.Address != "" {
		t.Errorf("todo で端末も無い子課題: %+v %v（起動するところから続けるはず）", r, ok)
	}
	// todo だが端末が live（hc run は通ったが kanban start の前に落ちた）: address を引き継ぎ、hc run し直さない
	asked := ""
	fresh := func(rev string) (string, int, bool) { asked = rev; return "t12", 0, true }
	if r, ok := Adopt(tasks, "D-1", c2, 1, brief, fresh, kept); !ok || r.Started || r.Address != "t12" || asked != "D-3" {
		t.Errorf("todo で端末が live（まだ何も受けていない）: %+v %v %q（address を引き継ぎ、最初のメッセージから）", r, ok, asked)
	}
	busy := func(string) (string, int, bool) { return "t12", 2, true }
	if r, ok := Adopt(tasks, "D-1", c2, 1, brief, busy, kept); !ok || !r.Started || r.Address != "t12" {
		t.Errorf("todo で端末が live（もう受けている）: %+v %v（二重に送らないはず）", r, ok)
	}
	if _, ok := Adopt(tasks, "D-9", c1, 1, brief, none, kept); ok {
		t.Error("ほかの課題の子を引き継いだ")
	}
}

// 結果（その commit の findings.json）を残さずに返ったレビュアーの子課題は、担当が頼み直した回（round がそれより後）では
// 引き継がない（起動し直す）。同じ回の review_request や、まだ見ている子課題は今までどおり引き継ぐ。
func TestAdoptSkipsReturnedEmpty(t *testing.T) {
	const c1 = "1111111111111111111111111111111111111111"
	brief := func(string) string { return "対象: worktree `x`（detached）。commit " + c1 + "。差分は" }
	none := func(string) (string, int, bool) { return "", 0, false }
	empty := func(rev string) bool { return rev == "D-1-r1" }
	tasks := []ledger.Issue{
		{"id": "D-1", "status": "doing", "profile": "claude"},
		{"id": "D-1-r1", "status": "done", "profile": "codex-review", "parent": "D-1", "agent": "t9"},
	}
	if r, ok := Adopt(tasks, "D-1", c1, 2, brief, none, empty); ok {
		t.Errorf("結果を残さずに返った前の回の子課題を引き継いだ（起動し直さない）: %+v", r)
	}
	if r, ok := Adopt(tasks, "D-1", c1, 1, brief, none, empty); !ok || !r.Started || r.ReviewerIssue != "D-1-r1" {
		t.Errorf("同じ回の review_request で子課題を引き継がない: %+v %v", r, ok)
	}
	tasks = append(tasks, ledger.Issue{"id": "D-1-r2", "status": "todo", "profile": "codex-review", "parent": "D-1"})
	if r, ok := Adopt(tasks, "D-1", c1, 2, brief, none, empty); !ok || r.Started || r.ReviewerIssue != "D-1-r2" {
		t.Errorf("起動し直しかけた回の子課題（todo）を引き継がない: %+v %v", r, ok)
	}
}

func TestLastReport(t *testing.T) {
	rows := []ledger.Row{
		{ID: "D-1", Change: map[string]any{"report": "write"}},
		{ID: "D-2", Change: map[string]any{"report": "write"}},
		{ID: "D-1", Change: map[string]any{"status": "doing"}},
		{ID: "D-1", Change: map[string]any{"report": "write"}},
	}
	if got := lastReport(rows, "D-1"); got != 4 {
		t.Errorf("D-1: %d", got)
	}
	if got := lastReport(rows, "D-9"); got != 0 {
		t.Errorf("D-9: %d", got)
	}
}

func TestStatePushReplacesSameEvent(t *testing.T) {
	s := &State{Pending: []Notification{}}
	s.Push(Notification{Issue: "D-1", Kind: "gate", Detail: map[string]any{"decision": "fix"}})
	s.Push(Notification{Issue: "D-2", Kind: "gate"})
	s.Push(Notification{Issue: "D-1", Kind: "review-requested"})
	s.Push(Notification{Issue: "D-1", Kind: "gate", Detail: map[string]any{"decision": "approve"}})
	if len(s.Pending) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(s.Pending), s.Pending)
	}
	last := s.Pending[2]
	if last.Issue != "D-1" || last.Kind != "gate" || last.Detail["decision"] != "approve" {
		t.Errorf("同じ task と kind が新しい1件に置き換わっていない: %+v", s.Pending)
	}
}

func TestStateLoadSave(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st, first, err := LoadState("demo")
	if err != nil || !first || st.Reviewers == nil || st.Pending == nil {
		t.Fatalf("初めて: first=%v err=%v st=%+v", first, err, st)
	}
	st.Reviewers["D-1@abc"] = Reviewer{Issue: "D-1", Commit: "abc", Failed: "fetch", Attempts: 1, ReviewerIssue: "D-2", Address: "t9"}
	if err := st.Save("demo"); err != nil {
		t.Fatal(err)
	}
	back, first, err := LoadState("demo")
	if err != nil || first {
		t.Fatalf("2回目: first=%v err=%v", first, err)
	}
	if r := back.Reviewers["D-1@abc"]; r.Failed != "fetch" || r.Attempts != 1 || r.Address != "t9" || r.Started {
		t.Errorf("控えが戻らない: %+v", r)
	}
	if _, err := os.Stat(StatePath("demo") + ".tmp"); err == nil {
		t.Error("書きかけの .tmp が残った")
	}
	os.WriteFile(StatePath("demo"), []byte("{"), 0o644)
	if _, _, err := LoadState("demo"); err == nil {
		t.Error("壊れた控えを読めたことにした")
	}
}

func TestReviewerBriefAddressesTheWorker(t *testing.T) {
	l := ledger.At(filepath.Join(t.TempDir(), "demo"))
	prev := strings.Repeat("a", 40)
	rep := &contract.Report{PR: "https://github.com/example/app/pull/7",
		ReviewRequest: &contract.ReviewRequest{Round: 2, Limit: 2, Commit: strings.Repeat("b", 40), Previous: &prev, Reviewer: "codex-review"}}
	task := ledger.Issue{"id": "D-1", "agent": "t7"}
	b := reviewerBrief(l, task, rep, Reviewer{Issue: "D-1", Round: 2, Worktree: "/w/demo-D-1-r2"}, span{}, "example/app", "7")
	for _, want := range []string{
		"hc send t7 ", "Chief of Staff・dispatcher へは送らない", "宛先はこの1つだけ",
		"`" + filepath.Join(l.Dir, "issues", "D-1-r2", "library", "findings.json") + "`", // kanban の中のレビュアー自身の課題（A-0216）
		// 正しさは worker skill §2 の3つの観点を名指しする
		"2. 正しさ: 並行性・失敗の波及・境界・前のバージョンの形・落ちたあとの起き直し\n",
		"screen_paths", "画像の添付が無い", "commit " + strings.Repeat("b", 40), "git diff " + prev + ".." + strings.Repeat("b", 40), "/w/demo-D-1-r2",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("依頼書に %q が無い:\n%s", want, b)
		}
	}
	if b2 := reviewerBrief(l, ledger.Issue{"id": "D-1"}, rep, Reviewer{Issue: "D-1", Round: 2}, span{}, "example/app", "7"); strings.Contains(b2, "hc send") || !strings.Contains(b2, "（まだ無い）") {
		t.Errorf("担当がいないのに宛先を書いた:\n%s", b2)
	}
}

// 1回目の依頼書は、手元の git で読む差分の起点を merge-base にさせ、指摘の file を PR の差分と照らさせる。
// 根拠の出来事: レビュアーが gh pr diff のあと baseRefOid（main の先頭）からの2点の差分を読み、
// main に後から入った変更を「PR が削除した」と読んで major を付けた。レビュアー向けの2つの skill も同じことを言う。
func TestFirstReviewBriefStartsAtMergeBase(t *testing.T) {
	pr, head := "https://github.com/example/app/pull/7", strings.Repeat("b", 40)
	rep := &contract.Report{PR: pr, ReviewRequest: &contract.ReviewRequest{Round: 1, Limit: 2, Commit: head, Reviewer: "codex-review"}}
	l := ledger.At(filepath.Join(t.TempDir(), "demo"))
	brief := func() string {
		return reviewerBrief(l, ledger.Issue{"id": "D-1", "agent": "t7"}, rep, Reviewer{Issue: "D-1", Round: 1, Worktree: "/w/demo-D-1-r1"}, span{}, "example/app", "7")
	}
	has := func(name, text, commit, pr string) {
		t.Helper()
		for _, want := range []string{
			"起点は merge-base: `git diff <baseRefOid>..." + commit + "`（点3つ",
			"main の先頭（baseRefOid）との2点の差分 `git diff <baseRefOid>.." + commit + "` は使わない",
			"指摘する file が PR の差分にあるかを `gh pr diff " + pr + " --name-only` で確かめる",
			// --name-only は rename の新しいパスしか出さない。元のパスの削除は PR のもの
			"`gh pr diff` の `rename from` のパスも PR の差分に数える",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s に %q が無い:\n%s", name, want, text)
			}
		}
	}
	first := brief()
	has("1回目の依頼書", first, head, pr)
	if !strings.Contains(first, "`gh pr view "+pr+" --json baseRefOid -q .baseRefOid`") {
		t.Errorf("1回目の依頼書に baseRefOid の取り方が無い:\n%s", first)
	}
	// 見るのはその節だけ（レビュアーは worker skill の §6 だけに従う）
	for _, s := range []struct{ skill, from, to string }{
		{"independent-review", "\n## 4. ", "\n## 5. "},
		{"worker", "\n## 6. ", "\n## 7. "},
	} {
		body, err := os.ReadFile(filepath.Join("..", "..", "package", "skills", s.skill, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		_, section, found := strings.Cut(string(body), s.from)
		section, _, closed := strings.Cut(section, s.to)
		if !found || !closed {
			t.Fatalf("skill %s に節 %q〜%q が無い", s.skill, s.from, s.to)
		}
		has("skill "+s.skill, section, "<commit>", "<PR>")
	}

	// 再レビューが見る範囲は reReviewRange だけが決める
	prev := strings.Repeat("a", 40)
	rep.ReviewRequest.Round, rep.ReviewRequest.Previous = 2, &prev
	if b := brief(); strings.Contains(b, "<baseRefOid>") {
		t.Errorf("再レビューの依頼書に1回目の範囲を書いた:\n%s", b)
	}
}

// rebase をまたいだ再レビューは、前後の commit の素の差分（base に入ったほかの変更まで含む）でなく、PR 自身の差分どうしの違いを見せる。
// 根拠の出来事: 前の回のあと main が進み、担当が rebase して直した。`git diff 前..今` は PR の外の main の変更まで見せていた
// （PR の 36 ファイルに対し 140 ファイル）。
func TestReviewSpanAcrossRebase(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app")
	git := func(args ...string) string {
		t.Helper()
		out, _, err := run("", []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@example.com", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@example.com"}, "", "git", append([]string{"-C", dir}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git("init", "-q", "-b", "main")
	write("pr.txt", "one\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	oldBase := git("rev-parse", "HEAD")
	git("switch", "-q", "-c", "work")
	write("pr.txt", "one\ntwo\n")
	git("commit", "-q", "-am", "pr")
	prev := git("rev-parse", "HEAD") // 前の回に見た commit
	git("switch", "-q", "main")
	for i := range 5 { // 前の回のあと main に入ったほかの変更
		write(fmt.Sprintf("main%d.txt", i), "m\n")
	}
	git("add", ".")
	git("commit", "-q", "-m", "main moves")
	newBase := git("rev-parse", "HEAD")
	git("switch", "-q", "work")
	git("rebase", "-q", "main")
	write("pr.txt", "one\ntwo!\n") // 前の回の指摘の直し
	git("commit", "-q", "-am", "fix")
	head := git("rev-parse", "HEAD")
	if files := git("diff", "--name-only", prev+".."+head); !strings.Contains(files, "main0.txt") {
		t.Fatalf("入力が根拠の出来事を写していない（素の差分に main の変更が無い）: %s", files)
	}

	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	gh := "#!/bin/sh\necho \"$*\" >> " + calls + "\n[ -f " + filepath.Join(bin, "ghfail") + " ] && exit 1\n" +
		"[ \"$*\" = 'pr view https://github.com/example/app/pull/7 --json baseRefOid -q .baseRefOid' ] || exit 9\necho " + newBase + "\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	pr, url := "https://github.com/example/app/pull/7", filepath.Join(root, "nowhere")
	l := ledger.At(filepath.Join(t.TempDir(), "demo"))
	brief := func(p, c string, sp span) string {
		rep := &contract.Report{PR: pr, ReviewRequest: &contract.ReviewRequest{Round: 2, Limit: 2, Commit: c, Previous: &p, Reviewer: "codex-review"}}
		return reviewerBrief(l, ledger.Issue{"id": "D-1", "agent": "t7"}, rep, Reviewer{Issue: "D-1", Round: 2, Worktree: "/w/demo-D-1-r2"}, sp, "example/app", "7")
	}

	// rebase をまたいだ: それぞれの merge-base から数えた PR の commit を突き合わせる
	sp := reviewSpan(dir, url, pr, prev, head)
	if sp != (span{Rebased: true, Old: oldBase, New: newBase}) {
		t.Fatalf("rebase をまたいだ範囲 = %+v（want Old %s New %s）", sp, oldBase, newBase)
	}
	b := brief(prev, head, sp)
	rd := "git range-diff " + oldBase + ".." + prev + " " + newBase + ".." + head
	if !strings.Contains(b, rd) || strings.Contains(b, "見るのは `git diff "+prev+".."+head+"`") {
		t.Errorf("rebase をまたいだ再レビューで PR 自身の差分どうしの違いを見せない（素の差分を見せた）:\n%s", b)
	}
	if !strings.Contains(b, "gh pr diff "+pr) {
		t.Errorf("直しの中身を前後ごと読む手順が無い:\n%s", b)
	}
	// 依頼書の手順どおりに読む: range-diff は main の commit を出さず、足した commit（`>`）は git show で読む
	out := git(strings.Fields(rd)[1:]...)
	added := regexp.MustCompile(`(?m)^-:\s+-+ > \d+:\s+([0-9a-f]+) `).FindAllStringSubmatch(out, -1)
	if strings.Contains(out, "main moves") || len(added) != 1 || !strings.Contains(b, "`git show <その commit>`") {
		t.Fatalf("依頼書の range-diff が main の commit を見せる・足した commit の読み方が無い:\n%s", out)
	}
	if fix := git("show", added[0][1]); strings.Contains(fix, "main0.txt") || !strings.Contains(fix, "+two!") {
		t.Errorf("足した commit が直しを見せない:\n%s", fix)
	}

	// rebase をまたがない（前の回の commit が祖先）: 今までどおり素の差分。gh には聞かない
	write("pr.txt", "one\ntwo!\nthree\n")
	git("commit", "-q", "-am", "fix2")
	next := git("rev-parse", "HEAD")
	os.Remove(calls)
	if sp := reviewSpan(dir, url, pr, head, next); sp != (span{}) {
		t.Errorf("祖先の再レビューの範囲 = %+v", sp)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("祖先の再レビューで gh に PR の base を聞いた")
	}
	if b := brief(head, next, span{}); !strings.Contains(b, "見るのは `git diff "+head+".."+next+"` だけ") || strings.Contains(b, "range-diff") {
		t.Errorf("祖先の再レビューの依頼書:\n%s", b)
	}

	// PR の base が読めない: 素の差分は使わせず、PR 自身の差分を見せる
	if err := os.WriteFile(filepath.Join(bin, "ghfail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sp = reviewSpan(dir, url, pr, prev, head)
	if sp != (span{Rebased: true}) {
		t.Errorf("base が読めないときの範囲 = %+v", sp)
	}
	if b := brief(prev, head, sp); strings.Contains(b, "見るのは `git diff") || !strings.Contains(b, "`gh pr diff "+pr+"`") {
		t.Errorf("base が読めないときの依頼書:\n%s", b)
	}

	// 前の回の commit が手元に無く fetch もできない: 祖先かをレビュアーに確かめさせる
	gone := strings.Repeat("c", 40)
	sp = reviewSpan(dir, url, pr, gone, head)
	if sp != (span{Unknown: true}) {
		t.Errorf("前の回の commit が無いときの範囲 = %+v", sp)
	}
	if b := brief(gone, head, sp); !strings.Contains(b, "git merge-base --is-ancestor "+gone+" "+head) || !strings.Contains(b, "`gh pr diff "+pr+"`") {
		t.Errorf("確かめられないときの依頼書:\n%s", b)
	}
}

// 担当の端末のラベル herdr.space・herdr.tab をレビュアーに写す（ADR-0002 決めたこと 12・受け入れ条件 3。ADR-0003 で Space も課題ごと）。読めなければ付けない。
func TestWorkerPlace(t *testing.T) {
	bin := t.TempDir()
	hc := "#!/bin/sh\n[ \"$1 $3\" = 'get --json' ] || exit 9\ncase $2 in\n" +
		"  t7) echo '{\"address\":\"t7\",\"labels\":{\"task\":\"D-1\",\"herdr.space\":\"andon-demo-D-1\",\"herdr.tab\":\"andon-demo-D-1\",\"where\":\"x / y\"}}' ;;\n" +
		"  t8) echo '{\"address\":\"t8\",\"labels\":{\"task\":\"D-2\"}}' ;;\n" +
		"  t9) echo '{\"address\":\"t9\"' ;;\n" +
		"  *) echo '{\"error\":{\"kind\":\"not_found\"}}' >&2; exit 3 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(hc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, c := range []struct {
		agent string
		want  []string
	}{
		{"t7", []string{"herdr.space=andon-demo-D-1", "herdr.tab=andon-demo-D-1"}}, // where は写さない
		{"t8", nil},  // ラベルが無い
		{"t9", nil},  // 返事が読めない
		{"t10", nil}, // 端末が無い
		{"", nil},    // agent が無い
	} {
		if got := workerPlace(c.agent); !slices.Equal(got, c.want) {
			t.Errorf("%q: got %q want %q", c.agent, got, c.want)
		}
	}
}

// 引き継ぐ detached worktree は、その repo の worktree list に detached で載り HEAD が commit のものだけ（置き場にあるだけでは信じない）。
func TestDetachedAt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GHQ_ROOT", root)
	dir := filepath.Join(root, "github.com", "example", "app")
	git := func(args ...string) string {
		t.Helper()
		out, _, err := run("", []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@example.com", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@example.com"}, "", "git", args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	git("init", "-q", dir)
	git("-C", dir, "commit", "-q", "--allow-empty", "-m", "a")
	c1 := git("-C", dir, "rev-parse", "HEAD")
	git("-C", dir, "commit", "-q", "--allow-empty", "-m", "b")
	wt := filepath.Join(root, "wt", "sample-D-1-r1")
	git("-C", dir, "worktree", "add", "-q", "--detach", wt, c1)
	if !detachedAt("github.com/example/app", wt, c1) {
		t.Error("その commit の detached worktree を引き継がない")
	}
	if detachedAt("github.com/example/app", wt, git("-C", dir, "rev-parse", "HEAD")) {
		t.Error("HEAD が違う worktree を引き継いだ")
	}
	plain := filepath.Join(root, "wt", "sample-D-1-r2")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if detachedAt("github.com/example/app", plain, c1) {
		t.Error("worktree でない置き場を引き継いだ")
	}
	link := filepath.Join(root, "wt", "sample-D-1-r3")
	if err := os.Symlink(wt, link); err != nil {
		t.Fatal(err)
	}
	if detachedAt("github.com/example/app", link, c1) {
		t.Error("symlink を引き継いだ")
	}
}
