package digest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
)

// 代わりに決めたこと（ADR-0044 決めたこと 6 の節 5。受け入れ条件 14）。一時の origin と checkout を置いて、本物の git で通す。

const docRepo = "github.com/example/sample-site"

type gitRepo struct {
	t             *testing.T
	origin, local string
}

// newGitRepo は期間の前の commit が 1 つある origin と、その時点の checkout（GHQ_ROOT の下）を作る。
func newGitRepo(t *testing.T, files map[string]string) *gitRepo {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("ANDON_WORKSPACE", filepath.Join(base, "cell"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR", "GIT_COMMITTER"} {
		t.Setenv(k+"_NAME", "t")
		t.Setenv(k+"_EMAIL", "t@example.com")
	}
	t.Setenv("GHQ_ROOT", filepath.Join(base, "ghq"))
	g := &gitRepo{t: t, origin: filepath.Join(base, "origin"), local: filepath.Join(base, "ghq", docRepo)}
	if err := os.MkdirAll(g.origin, 0o755); err != nil {
		t.Fatal(err)
	}
	g.git(g.origin, "init", "-q", "-b", "main")
	g.commit("2026-11-01T00:00:00Z", files)
	g.git(base, "clone", "-q", g.origin, g.local)
	return g
}

func (g *gitRepo) git(dir string, args ...string) string {
	g.t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		g.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit は origin に at の時刻の commit を積む。中身が空の文字列なら消す。
func (g *gitRepo) commit(at string, files map[string]string) string {
	g.t.Helper()
	for rel, body := range files {
		p := filepath.Join(g.origin, rel)
		if body == "" {
			if err := os.Remove(p); err != nil {
				g.t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			g.t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			g.t.Fatal(err)
		}
	}
	g.t.Setenv("GIT_AUTHOR_DATE", at)
	g.t.Setenv("GIT_COMMITTER_DATE", at)
	g.git(g.origin, "add", "-A")
	g.git(g.origin, "commit", "-q", "--allow-empty", "-m", "c")
	return g.git(g.origin, "rev-parse", "HEAD")
}

type wantDecision struct {
	Text, Mark, Path, Commit string
	Line                     int
}

// collectDecisions は input.json の決定の行（書かれた JSON から読む）。
func collectDecisions(t *testing.T, f *fake) []wantDecision {
	t.Helper()
	b := contract.Marshal(f.collect("S-0100"))
	var got struct {
		Decided *struct {
			Decisions []wantDecision `json:"decisions"`
		} `json:"decided"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Decided == nil {
		t.Fatal("入力に decided が無い")
	}
	return got.Decided.Decisions
}

// 受け入れ条件 14: 節へ 3 行を足す commit と、節の外に同じ印の文を足す commit。入力の決定の行は前の 2 行だけ。
func TestDecisions(t *testing.T) {
	const doc = "docs/adr/0001-sample.md"
	head := "# ADR-0001\n\n## 事実\n\n"
	section := "## 決めたこと（番号・日付・誰が決めたか）\n\n"
	g := newGitRepo(t, map[string]string{doc: head + section})
	three := "1. **2026-11-21 一覧を新しい順にする（Chief Engineer が決めた）。** 理由。\n" +
		"2. **2026-11-21 下書きを残す（Chief Engineer が決めた。推奨は別の案だった）。**\n" +
		"3. **2026-11-21 送る前に聞く（オーナーが決めた）。**\n"
	g.commit("2026-11-21T08:00:00Z", map[string]string{doc: head + section + three})
	last := g.commit("2026-11-21T09:00:00Z", map[string]string{doc: head + "1. **2026-11-21 節の外の文（Chief Engineer が決めた）。**\n\n" + section + three +
		"\n## やらないこと\n\n4. **2026-11-21 節のあとの文（Chief Engineer が決めた）。**\n"})

	f := newFake(t)
	f.issue("S-0001", "doing", "repo: "+docRepo)
	got := collectDecisions(t, f)
	want := []wantDecision{
		{Text: "2026-11-21 一覧を新しい順にする。", Mark: "Chief Engineer が決めた", Path: doc, Commit: last, Line: 9},
		{Text: "2026-11-21 下書きを残す。", Mark: "Chief Engineer が決めた。推奨は別の案だった", Path: doc, Commit: last, Line: 10},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("決定の行\n got %+v\nwant %+v", got, want)
	}
}

// 節 14 の細部: 太字のすぐ後ろの括弧（ADR-0037 の書き方）・ほかの文の混ざった括弧・コードの中の **・囲みと小見出し・番号の無い行。
func TestDecisionsLineShapes(t *testing.T) {
	doc := strings.Join([]string{
		"## 決めたこと",
		"### 小見出し",
		"1. **後ろに印。**（Chief Engineer が決めた）",
		"2. **混ざった（形はオーナーが決めた。描き方は Chief Engineer が決めた）。** 本文",
		"3. **別の括弧（S-0001）と `**/AGENTS.md`（Chief Engineer が決めた）。**",
		"4. **入れ子（Chief Engineer が決めた（仮））。**\r",
		"- **番号が無い（Chief Engineer が決めた）。**",
		"5. **本文の印は拾わない。** 理由（Chief Engineer が決めた）",
		"6. **閉じていない（Chief Engineer が決めた）。",
		"   ```md",
		"7. **例（Chief Engineer が決めた）。**",
		"   ```",
		"8. **オーナーの決定。**（オーナーが決めた。Chief Engineer の推奨どおり）",
		"## やらないこと",
		"9. **節の外（Chief Engineer が決めた）。**",
	}, "\n")
	var got []string
	for _, d := range markedDecisions([]byte(doc)) {
		got = append(got, fmt.Sprintf("%d %s|%s", d.Line, d.Text, d.Mark))
	}
	want := []string{
		"3 後ろに印。|Chief Engineer が決めた",
		"4 混ざった。|形はオーナーが決めた。描き方は Chief Engineer が決めた",
		"5 別の括弧（S-0001）と `**/AGENTS.md`。|Chief Engineer が決めた",
		"6 入れ子。|Chief Engineer が決めた（仮）",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("拾った行\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// 期間の中だけ・本文だけの直しと改名は新しい決定でない・マージ commit は入った時刻で数える・古い checkout から読み、checkout に何も書かない。
func TestDecisionsHistory(t *testing.T) {
	line := func(n int, text, body string) string {
		return fmt.Sprintf("%d. **%s（Chief Engineer が決めた）。** %s\n", n, text, body)
	}
	const sec = "## 決めたこと\n\n"
	old := "docs/adr/古い タイトル.md"
	g := newGitRepo(t, map[string]string{"docs/a.md": sec + line(1, "前からある", "理由"), old: sec + line(1, "名前だけ変わる", "長い本文")})
	g.commit("2026-11-20T09:59:59Z", map[string]string{"docs/before.md": sec + line(1, "期間の前", "")})
	g.commit("2026-11-21T01:00:00Z", map[string]string{"docs/a.md": sec + line(2, "前からある", "直した理由"), old: "", "docs/adr/新しい タイトル.md": sec + line(1, "名前だけ変わる", "長い本文")})
	g.commit("2026-11-21T02:00:00Z", map[string]string{"README.md": sec + line(1, "docs の外", ""), "docs/memo.txt": sec + line(1, "文書でない", "")})
	// branch の commit は期間の前、main に入ったのは期間の中
	g.git(g.origin, "checkout", "-q", "-b", "topic")
	g.commit("2026-11-10T00:00:00Z", map[string]string{"docs/adr/日本 語.md": sec + line(1, "マージで入った", "")})
	g.git(g.origin, "checkout", "-q", "main")
	g.t.Setenv("GIT_COMMITTER_DATE", "2026-11-21T03:00:00Z")
	g.git(g.origin, "merge", "-q", "--no-ff", "-m", "merge", "topic")
	merge := g.git(g.origin, "rev-parse", "HEAD")
	g.commit("2026-11-21T10:00:01Z", map[string]string{"docs/after.md": sec + line(1, "集めた時刻のあと", "")})
	refs, status := g.git(g.local, "for-each-ref"), g.git(g.local, "status", "--porcelain")

	f := newFake(t)
	f.issue("S-0001", "doing", "repo: "+docRepo)
	in := f.collect("S-0100")
	if len(in.Decided.Decisions) != 1 {
		t.Fatalf("決定の行: %+v", in.Decided.Decisions)
	}
	d := in.Decided.Decisions[0]
	wantURL := "https://" + docRepo + "/blob/" + merge + "/docs/adr/%E6%97%A5%E6%9C%AC%20%E8%AA%9E.md?plain=1#L3"
	if d.Text != "マージで入った。" || d.Commit != merge || str(d.URL) != wantURL {
		t.Errorf("決定の行: %+v url=%s", d, str(d.URL))
	}
	if g.git(g.local, "for-each-ref") != refs || g.git(g.local, "status", "--porcelain") != status {
		t.Error("checkout の refs か作業ツリーに書いた")
	}
	if _, err := os.Stat(filepath.Join(g.local, ".git", "FETCH_HEAD")); err == nil {
		t.Error("checkout に FETCH_HEAD を書いた")
	}
}

// 期間の中で語を直した行は最後の形の 1 行、足して取り消した行は出ない、括弧の中だけの直しは新しい決定でない。
func TestDecisionsNet(t *testing.T) {
	const sec = "## 決めたこと\n"
	g := newGitRepo(t, map[string]string{"docs/a.md": sec + "1. **前からある。**（Chief Engineer が決め、推奨どおり）\n"})
	kept := "1. **前からある。**（Chief Engineer が決め、オーナーが確かめた）\n"
	g.commit("2026-11-21T01:00:00Z", map[string]string{"docs/a.md": sec + kept + "2. **並びを変える（Chief Engineer が決めた）。**\n3. **あとで取り消す（Chief Engineer が決めた）。**\n",
		"docs/b.md": sec + "1. **文書ごと取り消す（Chief Engineer が決めた）。**\n"})
	g.commit("2026-11-21T02:00:00Z", map[string]string{"docs/a.md": sec + kept + "2. **並び順を変える（Chief Engineer が決めた）。**\n", "docs/b.md": ""})
	f := newFake(t)
	f.issue("S-0001", "doing", "repo: "+docRepo)
	if got := collectDecisions(t, f); len(got) != 1 || got[0].Text != "並び順を変える。" || got[0].Line != 3 {
		t.Errorf("決定の行: %+v", got)
	}
}

// 最初の commit（親が無い）の文書の行も拾う。
func TestDecisionsRootCommit(t *testing.T) {
	g := newGitRepo(t, map[string]string{"docs/a.md": "## 決めたこと\n1. **最初。**（Chief Engineer が決めた）\n"})
	got, _, err := repoDecisions(docRepo, "", time.Time{}, now.AddDate(1, 0, 0))
	if err != nil || len(got) != 1 || got[0].Line != 2 || got[0].Commit != g.git(g.origin, "rev-parse", "HEAD") {
		t.Errorf("%+v %v", got, err)
	}
}

// main を取り直せない・checkout が無いときは、古い木で数えずに skipped へ出し、ほかは集める。
func TestDecisionsUnreadable(t *testing.T) {
	for name, breakIt := range map[string]func(g *gitRepo) error{
		"origin が無い":   func(g *gitRepo) error { return os.RemoveAll(g.origin) },
		"checkout が無い": func(g *gitRepo) error { return os.RemoveAll(g.local) },
	} {
		t.Run(name, func(t *testing.T) {
			g := newGitRepo(t, map[string]string{"docs/a.md": "x\n"})
			if err := breakIt(g); err != nil {
				t.Fatal(err)
			}
			f := newFake(t)
			f.issue("S-0001", "doing", "repo: "+docRepo)
			in := f.collect("S-0100")
			if len(in.Decided.Decisions) != 0 || !hasSkip(in, docRepo, "決定の行を探していない") {
				t.Errorf("decided=%+v skipped=%+v", in.Decided, in.Skipped)
			}
		})
	}
}

func hasSkip(in *Input, source, reason string) bool {
	for _, s := range in.Skipped {
		if strings.Contains(s.Source, source) && strings.Contains(s.Reason, reason) {
			return true
		}
	}
	return false
}

// Chief of Staff が決めたこと: トリアージ・振り返りの記録と、範囲の問いの答え。
func TestDecidedRecords(t *testing.T) {
	f := newFake(t)
	for _, id := range []string{"S-0020", "S-0021", "S-0022"} {
		f.issue(id, "done")
	}
	f.row("2026-11-21T06:00:00Z", "t1", "S-0020", `{"status":"done"}`)
	f.row("2026-11-19T06:00:00Z", "t1", "S-0021", `{"status":"done"}`) // 期間の前
	f.row("2026-11-21T06:30:00Z", "t1", "S-0022", `{"status":"done"}`)
	f.write("kanban/library/triage/2026-11-21-S-0020.json", `{"items": [
		{"decision": "close"}, {"decision": "close"}, {"decision": "keep"},
		{"decision": "fix", "created_issue": "S-0050"},
		{"decision": "bundle", "created_issue": "S-0051"}, {"decision": "bundle", "created_issue": "S-0051"},
		{"decision": "person", "created_issue": "S-0052"}]}`)
	f.write("kanban/library/triage/2026-11-19-S-0021.json", `{"items": [{"decision": "close"}]}`)
	f.write("kanban/library/triage/2026-11-21-S-0022.json", `{"goal": "items が無い"}`)

	f.issue("S-0030", "review")
	f.lib("S-0030", "retro-applied.json", `{"applied_at": "2026-11-21T07:00:00Z", "complete": true, "issues": [{"pattern": "p1"}, {"pattern": "p2"}], "dropped": [{"pattern": "p3"}]}`)
	f.issue("S-0031", "done")
	f.lib("S-0031", "retro-applied.json", `{"applied_at": "2026-11-19T07:00:00Z", "complete": true, "issues": [], "dropped": []}`)
	f.issue("S-0032", "review")
	f.lib("S-0032", "retro-applied.json", `{"applied_at": "2026-11-21T07:30:00Z", "complete": false, "issues": []}`)

	f.issue("S-0040", "doing")
	q := func(n int, kind, at, by string) string {
		s := fmt.Sprintf("- n: %d\n  text: |-\n    問い %d（%s）\n    事実: x\n  kind: %s\n", n, n, kind, kind)
		if at == "" {
			return s + "  status: open\n"
		}
		return s + fmt.Sprintf("  status: received\n  answer: \"答え %d\"\n  answered: \"%s\"\n  answeredBy: %s\n", n, at, by)
	}
	f.write("kanban/issues/S-0040/questions.yaml",
		q(1, "scope", "2026-11-21T08:00:00Z", "terminal:t1")+
			q(2, "scope", "2026-11-21T08:10:00Z", "terminal:t1")+
			q(3, "line", "2026-11-21T08:20:00Z", "terminal:t1")+ // 灯りの判断はオーナーのもの
			q(4, "scope", "2026-11-21T08:30:00Z", "owner")+ // オーナーが打った文は出さない
			q(5, "scope", "2026-11-21T08:40:00Z", "terminal:dispatcher")+
			q(6, "scope", "2026-11-19T08:00:00Z", "terminal:t1")+
			q(7, "scope", "", ""))
	f.lib("S-0040", "questions/1.json", `{"kind": "scope", "question": "どれを直すか", "options": [{"label": "c1", "pick": true}, {"label": "c2"}]}`)
	f.issue("S-0041", "doing")
	f.write("kanban/issues/S-0041/questions.yaml", "- [壊れた\n")

	in := f.collect("S-0100")
	got := contract.Marshal(in.Decided)
	want := contract.Marshal(&Decided{
		Decisions: []Decision{},
		Mains:     []MainRead{},
		Triage:    []Triaged{{Issue: "S-0020", Title: "タイトル S-0020", At: "2026-11-21T06:00:00Z", Closed: 2, Cut: 2, Path: "kanban/library/triage/2026-11-21-S-0020.json"}},
		Retro:     []Retroed{{Issue: "S-0030", Title: "タイトル S-0030", At: "2026-11-21T07:00:00Z", Fix: 2, Dropped: 1, Path: "kanban/issues/S-0030/library/retro-applied.json"}},
		Answers: []ScopeReply{
			{Issue: "S-0040", Title: "タイトル S-0040", N: 1, Question: "どれを直すか", Answer: "答え 1", At: "2026-11-21T08:00:00Z", Path: "kanban/issues/S-0040/questions.yaml"},
			{Issue: "S-0040", Title: "タイトル S-0040", N: 2, Question: "問い 2（scope）", Answer: "答え 2", At: "2026-11-21T08:10:00Z", Path: "kanban/issues/S-0040/questions.yaml"},
		},
	})
	if string(got) != string(want) {
		t.Errorf("decided\n%s\nwant\n%s", got, want)
	}
	for _, id := range []string{"S-0022.json", "S-0032/library/retro-applied.json", "S-0041/questions.yaml"} {
		if !hasSkip(in, id, "") {
			t.Errorf("%s が skipped に無い: %+v", id, in.Skipped)
		}
	}
}

// 節 5 は入力から描く。秘密は伏せ、decided の無い前のバージョンの入力は「入っていない」と描く（「無かった」と読ませない）。
func TestBuildDecided(t *testing.T) {
	g := newGitRepo(t, map[string]string{"docs/a.md": "## 決めたこと\n"})
	secret := "ghp_" + strings.Repeat("a", 36)
	g.commit("2026-11-21T04:00:00Z", map[string]string{"docs/a.md": "## 決めたこと\n1. **<b>並びを変える</b> " + secret + "（Chief Engineer が決めた）。**\n"})
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.issue("S-0002", "doing", "repo: "+docRepo)
	f.write("kanban/issues/S-0002/questions.yaml", "- n: 1\n  text: どれを直すか\n  kind: scope\n  status: received\n  answer: c1 だけ\n  answered: \"2026-11-21T05:00:00Z\"\n  answeredBy: terminal:t1\n")
	f.collect("S-0100")
	html := buildHTML(t, f, validText())
	for _, s := range []string{"文書に入った決定（1 件）", "&lt;b&gt;並びを変える&lt;/b&gt;", "[REDACTED:github]", "（Chief Engineer が決めた）", "docs/a.md:2", "/docs/a.md?plain=1#L2", "c1 だけ", "どれを直すか"} {
		if !strings.Contains(html, s) {
			t.Errorf("節 5 に %q が無い", s)
		}
	}
	if strings.Contains(html, secret) {
		t.Error("秘密の形が残った")
	}

	path := filepath.Join(dir(f.root, "S-0100"), inputFile)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	delete(old, "decided")
	f.write(path, string(contract.Marshal(old)))
	html = buildHTML(t, f, validText())
	if !strings.Contains(html, "代わりに決めたことが入っていない") || strings.Contains(html, "見つからなかった。</p>") {
		t.Error("decided の無い入力を、決めたことが無かったと描いた")
	}
}

// 前からある決定の 1 文の語を直しても、新しい決定にしない（同じ文書・番号・日付）。番号を奪った別の決定と、日付の変わった決め直しは載せる。
func TestDecisionsRewording(t *testing.T) {
	const sec = "## 決めたこと\n"
	g := newGitRepo(t, map[string]string{"docs/a.md": sec +
		"1. **2026-11-01 一覧を新しい順にする（Chief Engineer が決めた）。**\n" +
		"2. **形を揃える。**（2026-11-02。Chief Engineer が決めた）\n" +
		"3. **2026-11-03 下書きを残す（Chief Engineer が決めた）。**\n" +
		"4. **2026-11-04 検索を足す（Chief Engineer が決めた）。**\n"})
	g.commit("2026-11-21T01:00:00Z", map[string]string{"docs/a.md": sec +
		"1. **2026-11-01 一覧を新着順にする（Chief Engineer が決めた）。**\n" + // 語の直し
		"2. **書式を揃える。**（2026-11-02。Chief Engineer が決めた）\n" + // 日付が括弧の中
		"3. **2026-11-21 下書きを残さない（Chief Engineer が決めた）。**\n" + // 決め直し
		"4. **2026-11-04 通知を足す（Chief Engineer が決めた）。**\n" + // 割り込んだ決定
		"5. **2026-11-04 検索を足す（Chief Engineer が決めた）。**\n"})
	f := newFake(t)
	f.issue("S-0001", "doing", "repo: "+docRepo)
	var got []string
	for _, d := range collectDecisions(t, f) {
		got = append(got, d.Text)
	}
	if want := "2026-11-21 下書きを残さない。|2026-11-04 通知を足す。"; strings.Join(got, "|") != want {
		t.Errorf("決定の行 %q, want %q", strings.Join(got, "|"), want)
	}
}

// sentBefore は at に集めて送り終えた前のダイジェスト。
func sentBefore(f *fake, at time.Time) {
	f.t.Helper()
	const id = "S-0090"
	f.issue(id, "doing", "routine: daily-digest")
	today := f.env.Now
	f.env.Now = at
	in := f.collect(id)
	f.env.Now = today
	f.issue(id, "done", "routine: daily-digest")
	f.lib(id, "digest/sent.json", `{"issue": "`+id+`", "since": "`+in.Since+`", "until": "`+in.Until+`", "destinations": [{"kind": "files", "at": "`+in.Until+`", "ok": true, "url": "http://files.cell.localhost:1355/x.html"}]}`)
}

// commit の時刻が期間の前でも、前のダイジェストのあとに main に入った行は載る（fast-forward の push は commit の時刻が古いまま入る）。
func TestDecisionsPushedLate(t *testing.T) {
	const sec = "## 決めたこと\n"
	for name, newer := range map[string]bool{"それだけ": false, "上に新しい commit": true} {
		t.Run(name, func(t *testing.T) {
			g := newGitRepo(t, map[string]string{"docs/a.md": sec})
			f := newFake(t)
			f.issue("S-0001", "doing", "repo: "+docRepo)
			sentBefore(f, now.Add(-2*time.Hour))
			g.commit("2026-11-20T09:00:00Z", map[string]string{"docs/a.md": sec + "1. **遅れて入った（Chief Engineer が決めた）。**\n"})
			if newer {
				g.commit("2026-11-21T09:00:00Z", map[string]string{"README.md": "x\n"})
			}
			if got := collectDecisions(t, f); len(got) != 1 || got[0].Text != "遅れて入った。" {
				t.Errorf("決定の行: %+v", got)
			}
		})
	}
}

// main を読めなかった回の分は、次のダイジェストが同じ所から読む。
func TestDecisionsAfterFailedRead(t *testing.T) {
	g := newGitRepo(t, map[string]string{"docs/a.md": "## 決めたこと\n"})
	f := newFake(t)
	f.issue("S-0001", "doing", "repo: "+docRepo)
	today := f.env.Now
	f.env.Now = today.Add(-3 * time.Hour)
	sentBefore(f, f.env.Now)
	g.commit("2026-11-21T07:30:00Z", map[string]string{"docs/a.md": "## 決めたこと\n1. **読めなかった回の分（Chief Engineer が決めた）。**\n"})
	away := g.origin + ".away"
	if err := os.Rename(g.origin, away); err != nil {
		t.Fatal(err)
	}
	f.env.Now = today.Add(-time.Hour)
	f.issue("S-0091", "doing", "routine: daily-digest")
	in := f.collect("S-0091")
	f.issue("S-0091", "done", "routine: daily-digest")
	f.lib("S-0091", "digest/sent.json", `{"issue": "S-0091", "since": "`+in.Since+`", "until": "`+in.Until+`", "destinations": [{"kind": "files", "at": "`+in.Until+`", "ok": true, "url": "http://files.cell.localhost:1355/y.html"}]}`)
	if err := os.Rename(away, g.origin); err != nil {
		t.Fatal(err)
	}
	f.env.Now = today
	if got := collectDecisions(t, f); len(got) != 1 || got[0].Text != "読めなかった回の分。" {
		t.Errorf("決定の行: %+v", got)
	}
}

// 集めている最中に当て終えていなかった振り返りは、終わった次のダイジェストに 1 度だけ載る（applied_at は当て始めの時刻のまま）。
func TestRetroFinishedAfterCollect(t *testing.T) {
	f := newFake(t)
	f.issue("S-0030", "review")
	rec := func(complete bool) {
		f.lib("S-0030", "retro-applied.json", fmt.Sprintf(`{"applied_at": "2026-11-21T07:59:59Z", "complete": %t, "issues": [{}], "dropped": []}`, complete))
	}
	rec(false)
	sentBefore(f, now.Add(-2*time.Hour))
	rec(true)
	if in := f.collect("S-0100"); len(in.Decided.Retro) != 1 || in.Decided.Retro[0].Issue != "S-0030" {
		t.Errorf("retro=%+v skipped=%+v", in.Decided.Retro, in.Skipped)
	}
}

// 読んだ main を使えない前の入力（前のバージョン・main に無い commit・送ったあとの集め直し・7 日で切った期間）では、時刻で切る。
func TestDecisionsWithoutLastMain(t *testing.T) {
	g := newGitRepo(t, map[string]string{"docs/a.md": "## 決めたこと\n"})
	g.commit("2026-11-21T09:00:00Z", map[string]string{"docs/a.md": "## 決めたこと\n1. **期間の中（Chief Engineer が決めた）。**\n"})
	tip := g.git(g.origin, "rev-parse", "HEAD")
	for name, c := range map[string]struct{ until, body string }{
		"前のバージョン":         {"2026-11-21T08:00:00Z", `{"until": "2026-11-21T08:00:00Z", "skipped": []}`},
		"main に無い commit": {"2026-11-21T08:00:00Z", `{"until": "2026-11-21T08:00:00Z", "decided": {"mains": [{"repo": "` + docRepo + `", "commit": "` + sha1 + `"}]}}`},
		"集め直した":           {"2026-11-21T08:00:00Z", `{"until": "2026-11-21T09:30:00Z", "decided": {"mains": [{"repo": "` + docRepo + `", "commit": "` + tip + `"}]}}`},
		"7 日で切った":         {"2026-11-12T08:00:00Z", `{"until": "2026-11-12T08:00:00Z", "decided": {"mains": [{"repo": "` + docRepo + `", "commit": "` + tip + `"}]}}`},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			f.issue("S-0001", "doing", "repo: "+docRepo)
			f.issue("S-0090", "done", "routine: daily-digest")
			f.lib("S-0090", "digest/input.json", c.body)
			f.lib("S-0090", "digest/sent.json", `{"issue": "S-0090", "since": "2026-11-01T00:00:00Z", "until": "`+c.until+`", "destinations": [{"kind": "files", "at": "`+c.until+`", "ok": true, "url": "http://files.cell.localhost:1355/x.html"}]}`)
			if got := collectDecisions(t, f); len(got) != 1 || got[0].Commit != tip {
				t.Errorf("決定の行: %+v", got)
			}
		})
	}
}
