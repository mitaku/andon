package ledger

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"andon/internal/ledger/ledgertest"
)

func fakeBin(t *testing.T, dir string, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, n := range names {
		body := "#!/bin/sh\necho " + dir + "\n"
		if err := os.WriteFile(filepath.Join(bin, n), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

func TestOpenRefusesWithoutKanban(t *testing.T) {
	t.Setenv("PATH", fakeBin(t, t.TempDir()))
	_, err := Open()
	if err == nil || !strings.Contains(err.Error(), "kanban が無い") || !strings.Contains(err.Error(), "go install ./cmd/kanban") {
		t.Fatalf("Open() without kanban: err = %v; want kanban が無い and how to install it", err)
	}
}

func TestOpenUsesKanban(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", fakeBin(t, root, "kanban"))
	l, err := Open()
	if err != nil || l.Root != root {
		t.Fatalf("Open() = %+v, %v; want root %s", l, err, root)
	}
}

func TestCanonID(t *testing.T) {
	for in, want := range map[string]string{"A-5": "A-0005", "A-05": "A-0005", "A-0005": "A-0005", "A-12345": "A-12345", "Y-T-7": "Y-T-0007", "A-0": "A-0", "A-x": "A-x", "abc": "abc", "-5": "-5", "A-": "A-",
		"A-5-r1": "A-0005-r1", "A-0005-r01": "A-0005-r1", "A-0005-r0": "A-0005-r0", "A-0005-r": "A-0005-r", "A-0005-rx": "A-0005-rx", "A-12345-r3": "A-12345-r3"} {
		if got := CanonID(in); got != want {
			t.Errorf("CanonID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIdLess(t *testing.T) {
	ids := []string{"A-10000", "A-0006", "A-0005-r10", "A-0005-r2", "A-0005", "A-0005-r1"}
	sort.SliceStable(ids, func(i, j int) bool { return IDLess(ids[i], ids[j]) })
	if got := strings.Join(ids, " "); got != "A-0005 A-0005-r1 A-0005-r2 A-0005-r10 A-0006 A-10000" {
		t.Fatalf("got %s", got)
	}
}

func TestTrustedReadsOldIds(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5 # the lead\n"), 0o644)
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(ledgertest.Log(t, `{"at":"2026-11-01T00:00:00Z","by":"terminal:t5","id":"A-5","change":{"review":"on"}}`)), 0o644)
	if l.ChiefOfStaff() != "t5" {
		t.Fatalf("ChiefOfStaff() = %q", l.ChiefOfStaff())
	}
	if !l.Trusted(l.Log(), Issue{"id": "A-0005", "review": "on"}, "review") {
		t.Fatal("the row written as A-5 does not count for A-0005")
	}
}

func TestLogReadsLongLinesAndWhatFollows(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	long := `{"at":"2026-11-01T00:00:00Z","by":"terminal:t9","id":"A-0001","change":{"add":"` + strings.Repeat("a", 8<<20) + `"}}`
	last := `{"at":"2026-11-01T00:01:00Z","by":"terminal:t5","id":"A-0001","change":{"review":"on"}}`
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(ledgertest.Log(t, long, last)), 0o644)
	rows := l.Log()
	if len(rows) != 2 || rows[1].By != "terminal:t5" {
		t.Fatalf("rows: %d", len(rows))
	}
}

// deliverable: candidates は Chief of Staff が最後に書いたときだけ信じる（ADR-0006 決めたこと 7）。
func TestCandidates(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	row := func(id, by, v string) Row { return Row{ID: id, By: by, Change: map[string]any{"deliverable": v}} }
	rows := []Row{
		row("A-0001", "terminal:t5", "candidates"),
		row("A-0002", "terminal:t7", "candidates"),
		row("A-0003", "terminal:t5", "candidates"), row("A-0003", "terminal:t7", "candidates"),
		row("A-0004", "terminal:t5", "pr"),
	}
	for _, c := range []struct {
		task Issue
		want bool
	}{
		{Issue{"id": "A-0001", "deliverable": "candidates"}, true},
		{Issue{"id": "A-0002", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0003", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0004", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0004", "deliverable": "pr"}, false},
		{Issue{"id": "A-0005"}, false},
	} {
		if got := l.Candidates(rows, c.task); got != c.want {
			t.Errorf("Candidates(%v) = %v, want %v", c.task, got, c.want)
		}
	}
}

// estimate は small・medium・large だけを読み、Chief of Staff が最後に書いたときだけ信じる（ADR-0014 受け入れ条件 1）。
func TestEstimate(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	row := func(id, by, v string) Row { return Row{ID: id, By: by, Change: map[string]any{"estimate": v}} }
	rows := []Row{
		row("A-0001", "terminal:t5", "medium"),
		row("A-0002", "terminal:t7", "large"),
		row("A-0003", "terminal:t5", "small"), row("A-0003", "terminal:t7", "large"),
		row("A-0004", "terminal:t5", "中"),
		row("A-0005", "terminal:t5", "small"),
	}
	for _, c := range []struct {
		task     Issue
		settings string
		estimate string
	}{
		{Issue{"id": "A-0001", "estimate": "medium"}, "medium", "medium"},
		{Issue{"id": "A-0002", "estimate": "large"}, "large", ""},
		{Issue{"id": "A-0003", "estimate": "large"}, "large", ""},
		{Issue{"id": "A-0004", "estimate": "中"}, "", ""},
		{Issue{"id": "A-0005", "estimate": "large"}, "large", ""},
		{Issue{"id": "A-0006"}, "", ""},
	} {
		if got := SettingsOf(c.task).Estimate; got != c.settings {
			t.Errorf("SettingsOf(%v).Estimate = %q, want %q", c.task, got, c.settings)
		}
		if got := l.Estimate(rows, c.task); got != c.estimate {
			t.Errorf("Estimate(%v) = %q, want %q", c.task, got, c.estimate)
		}
	}
}

// 予約キー acceptance は受け入れ条件の番号のコンマの並び（ADR-0035 決めたこと 3）。無ければ番号を持たない課題、形が違えば ok が false。
func TestAcceptanceOf(t *testing.T) {
	for _, c := range []struct {
		v        any
		ids      string
		ok       bool
		settings string
	}{
		{nil, "", true, ""},
		{"A4", "A4", true, "A4"},
		{"A4,A13,A4", "A4,A13", true, "A4,A13,A4"},
		{"4,13", "4,13", true, "4,13"},
		{"4,A4,13", "4,13", true, "4,A4,13"},
		{"A01,A1", "A01,A1", true, "A01,A1"},
		{"A1, A4", "", false, ""},
		{"A1,,A4", "", false, ""},
		{"", "", false, ""},
		{[]any{"A1"}, "", false, ""},
	} {
		task := Issue{"id": "A-0001"}
		if c.v != nil {
			task["acceptance"] = c.v
		}
		ids, ok := AcceptanceOf(task)
		if strings.Join(ids, ",") != c.ids || ok != c.ok {
			t.Errorf("AcceptanceOf(%v) = %v, %v; want %q, %v", c.v, ids, ok, c.ids, c.ok)
		}
		if got := SettingsOf(task).Acceptance; got != c.settings {
			t.Errorf("SettingsOf(%v).Acceptance = %q, want %q", c.v, got, c.settings)
		}
	}
}

func TestAcceptanceKey(t *testing.T) {
	for id, want := range map[string]string{"4": "4", "A4": "4", "A13": "13", "A01": "01", "A": "A", "A4a": "A4a", "a4": "a4", "B4": "B4", "AA4": "AA4"} {
		if got := AcceptanceKey(id); got != want {
			t.Errorf("AcceptanceKey(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestConfigReadsReposAnyForm(t *testing.T) {
	for body, want := range map[string]string{
		"version: 3\nrepos:\n  - github.com/example/a\n  - \"github.com/example/b\"\n": "github.com/example/a github.com/example/b",
		"version: 3\nrepos: [github.com/example/a, 'github.com/example/b']\n":          "github.com/example/a github.com/example/b",
		"version: 3\nrepos: []\n": "",
		"version: 3\n":            "",
	} {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "kanban"), 0o755)
		os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte(body+"chief_of_staff: \"t1\" # the CoS\ntimezone: Asia/Tokyo\n"), 0o644)
		l := At(root)
		if got := strings.Join(l.Repos(), " "); got != want {
			t.Errorf("%q: Repos = %q, want %q", body, got, want)
		}
		if l.ChiefOfStaff() != "t1" || l.Timezone() != "Asia/Tokyo" {
			t.Errorf("%q: chief_of_staff %q timezone %q", body, l.ChiefOfStaff(), l.Timezone())
		}
	}
}

func TestTakenByChiefOfStaff(t *testing.T) {
	cos := func(v any) Row { return Row{By: "andon", Config: map[string]any{"chief_of_staff": v}} }
	agent := func(id, a string) Row {
		return Row{By: "terminal:" + a, ID: id, Change: map[string]any{"status": "doing", "agent": a}}
	}
	for _, c := range []struct {
		name string
		rows []Row
		want bool
	}{
		{"Chief of Staff が受けて、あとで起き直した", []Row{cos("t5"), agent("S-7", "t5"), cos("t6")}, true},
		{"担当が受けた", []Row{cos("t5"), agent("S-7", "t9"), cos("t6")}, false},
		{"Chief of Staff だったアドレスを、あとで担当が使った", []Row{cos("t5"), cos("t6"), agent("S-7", "t5")}, false},
		{"Chief of Staff が受けたあと、担当が受け直した", []Row{cos("t5"), agent("S-7", "t5"), agent("S-7", "t9")}, false},
		{"chief_of_staff を消したあとに受けた", []Row{cos("t5"), cos(nil), agent("S-7", "t5")}, false},
		{"config の行が無い", []Row{agent("S-7", "t5")}, false},
		{"前の名前（coordinator）の行で Chief of Staff だったときに受けた", []Row{{By: "andon", Config: map[string]any{"coordinator": "t5"}}, agent("S-7", "t5"), cos("t6")}, true},
		{"ほかの課題の行", []Row{cos("t5"), agent("S-8", "t5")}, false},
	} {
		last := ""
		for _, r := range c.rows {
			if r.ID == "S-7" {
				last = r.Change["agent"].(string)
			}
		}
		if last == "" {
			last = "t5"
		}
		if got := TakenByChiefOfStaff(c.rows, "S-7", last); got != c.want {
			t.Errorf("%s: %v（want %v）", c.name, got, c.want)
		}
	}
}

func TestUnsettled(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(ledgertest.Log(t,
		`{"at":"2026-11-01T00:00:00Z","by":"terminal:t5","id":"A-0001","change":{"review":"off","must_fix":"major"}}`,
		`{"at":"2026-11-01T00:00:00Z","by":"terminal:t9","id":"A-0001","change":{"merge":"auto"}}`)), 0o644)
	issue := Issue{"id": "A-0001", "review": "off", "must_fix": "major", "merge": "auto", "estimate": "medium"}
	got := l.Unsettled(l.Log(), issue, DefaultSettings("claude", "off"))
	if want := []string{"estimate=medium"}; !slices.Equal(got, want) {
		t.Errorf("Unsettled = %q, want %q", got, want)
	}
	delete(issue, "estimate")
	if got := l.Unsettled(l.Log(), issue, DefaultSettings("claude", "off")); !slices.Equal(got, []string{"estimate=medium"}) {
		t.Errorf("無いキー: %q", got)
	}
}

func TestRewriteSettings(t *testing.T) {
	const tail = "\n頼まれていない稀なケースのために設計しない。\n\n## 追記（2026-11-23）\n- Review: off のままにしない\n"
	for _, c := range []struct {
		name, brief, want string
		issue             Issue
	}{
		{"あとから書き直した予約キー",
			"# タイトル\n\n## Settings\n- Review: off\n- Must fix: major\n- Merge: person\n- Estimate: 中\n" + tail,
			"# タイトル\n\n## Settings\n- Review: on ★\n- Must fix: major\n- Merge: auto ★\n- Estimate: 中\n- Reviewer: codex-review\n- Review rounds: 2\n" + tail,
			Issue{"review": "on", "must_fix": "major", "reviewer": "codex-review", "review_rounds": "2", "merge": "auto", "estimate": "medium"}},
		{"合っていれば変えない",
			"## Settings\n- Review: on\n- Merge: person\n- Estimate: 小\n" + tail, "",
			Issue{"review": "on", "merge": "person", "estimate": "small"}},
		{"値の頭が同じ行は注釈ごと残す",
			"## Settings\n- Deliverable: candidates ★調べる\n- Review: on", "",
			Issue{"deliverable": "candidates", "review": "on"}},
		{"知らない行は残し、正に値の無い行と重なった行は消す",
			"## Settings\n- Review: on\n- 横展開: 指された箇所だけ\n- Acceptance: 4\n- review: off\n",
			"## Settings\n- Review: off\n- 横展開: 指された箇所だけ\n",
			Issue{"review": "off"}},
		{"無いキーは SettingsOf の既定",
			"## Settings\n- Reviewer: codex-review\n- Merge: auto\n",
			"## Settings\n- Reviewer: claude-review\n- Merge: person\n",
			Issue{"profile": "codex"}},
		{"最後の行に改行が無くても行がつながらない",
			"## Settings\n- Review: off", "## Settings\n- Review: on ★\n- Merge: auto ★\n",
			Issue{"review": "on", "merge": "auto"}},
		{"見出しで終わる依頼書", "## Settings", "## Settings\n- Review: on ★\n", Issue{"review": "on"}},
		{"CRLF", "## Settings\r\n- Review: off\r\n", "## Settings\r\n- Review: on ★\r\n", Issue{"review": "on"}},
		{"写しが無ければ末尾に空行を挟んで足す（ADR-0053 決めたこと 19）", "# タイトル\n完了条件: 押せる\n",
			"# タイトル\n完了条件: 押せる\n\n## Settings\n- Review: on ★\n- Merge: person\n- Estimate: 中\n", Issue{"review": "on", "merge": "person", "estimate": "medium"}},
		{"改行で終わらない依頼書にも空行を挟んで足す", "# タイトル", "# タイトル\n\n## Settings\n- Review: off\n", Issue{"review": "off"}},
		{"空行で終わる依頼書には空行を足さない", "# タイトル\n\n", "# タイトル\n\n## Settings\n- Review: off\n", Issue{"review": "off"}},
		{"CRLF の依頼書には CRLF で足す", "# タイトル\r\n", "# タイトル\r\n\r\n## Settings\r\n- Review: off\r\n", Issue{"review": "off"}},
		{"予約キーが無ければ写しを足さない", "# タイトル\n", "", Issue{"profile": "claude"}},
		{"コード例の中にしか見出しが無ければ足す", "```\n## Settings\n```\n", "```\n## Settings\n```\n\n## Settings\n- Review: off\n", Issue{"review": "off"}},
		{"見出しが2つあれば両方",
			"## Settings\n- Review: off\n\n## 追記\n## Settings\n- Review: off\n- Merge: person\n",
			"## Settings\n- Review: on ★\n- Merge: auto ★\n\n## 追記\n## Settings\n- Review: on ★\n- Merge: auto ★\n",
			Issue{"review": "on", "merge": "auto"}},
		{"コード例の中の見出しは触らない",
			"例:\n```\n## Settings\n- Review: off\n```\n\n## Settings\n- Review: off\n",
			"例:\n```\n## Settings\n- Review: off\n```\n\n## Settings\n- Review: on ★\n",
			Issue{"review": "on"}},
		{"4つのバッククォートの囲みの中の3つの囲みと見出しは触らない",
			"````\n```\n## Settings\n- Review: off\n```\n## Settings\n- Review: off\n````\n\n## Settings\n- Review: off\n",
			"````\n```\n## Settings\n- Review: off\n```\n## Settings\n- Review: off\n````\n\n## Settings\n- Review: on ★\n",
			Issue{"review": "on"}},
		{"成果が報告の課題は Deliverable: report と写す（ADR-0066 受け入れ条件 2）",
			"## Settings\n- Review: off\n- Deliverable: pr\n",
			"## Settings\n- Review: off\n- Deliverable: report ★\n",
			Issue{"review": "off", "deliverable": "report"}},
		{"知らない値は SettingsOf が読む値で写す",
			"## Settings\n- Deliverable: survey\n- Merge: never\n- Review: maybe\n",
			"## Settings\n- Deliverable: pr\n- Merge: person\n",
			Issue{"deliverable": "survey", "merge": "never", "review": "maybe"}},
		{"行の無いキーも知らない値は足さない", "## Settings\n- Review: on\n", "## Settings\n- Review: on\n- Merge: person\n",
			Issue{"review": "on", "merge": "never", "estimate": "huge"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			want := c.want
			if want == "" {
				want = c.brief
			}
			got, changed := RewriteSettings(c.brief, c.issue, nil)
			if got != want || changed != (want != c.brief) {
				t.Errorf("changed=%v\n got %q\nwant %q", changed, got, want)
			}
			if again, ch := RewriteSettings(got, c.issue, nil); ch || again != got {
				t.Errorf("2回目で変わった: %q", again)
			}
		})
	}
}

// 既定から変えた行には ★ を付け、理由があれば ★ に続けて添える。既定どおりの行・見込み・受け入れ条件には付けない。
// 理由のあるキーは、値の頭が同じ行も理由つきに書き直す（ADR-0053 決めたこと 19）。
func TestRewriteSettingsMarks(t *testing.T) {
	issue := Issue{"profile": "claude", "review": "on", "reviewer": "codex-review", "must_fix": "minor", "review_rounds": "2",
		"merge": "person", "estimate": "large", "acceptance": "9,10"}
	reasons := map[string]string{"review": "新しい module", "must_fix": "細かい誤りも直す"}
	for _, c := range []struct{ name, brief, want string }{
		{"足す", "# x\n", "# x\n\n## Settings\n- Review: on ★新しい module\n- Reviewer: codex-review\n- Must fix: minor ★細かい誤りも直す\n" +
			"- Review rounds: 2\n- Merge: person\n- Acceptance: 9,10\n- Estimate: 大\n"},
		{"値の同じ行も理由つきに書き直す", "## Settings\n- Review: on\n- Must fix: minor ★前の理由\n- Merge: person\n- Estimate: 大\n",
			"## Settings\n- Review: on ★新しい module\n- Must fix: minor ★細かい誤りも直す\n- Merge: person\n- Estimate: 大\n- Reviewer: codex-review\n- Review rounds: 2\n- Acceptance: 9,10\n"},
	} {
		got, _ := RewriteSettings(c.brief, issue, reasons)
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if again, ch := RewriteSettings(got, issue, reasons); ch {
			t.Errorf("%s: 2回目で変わった: %q", c.name, again)
		}
	}
	// Codex の担当の既定のレビュアーは claude-review なので印を付けず、codex-review に印を付ける
	if got, _ := RewriteSettings("", Issue{"profile": "codex", "reviewer": "claude-review"}, nil); got != "## Settings\n- Reviewer: claude-review\n" {
		t.Errorf("codex の既定のレビュアー: %q", got)
	}
	if got, _ := RewriteSettings("", Issue{"profile": "codex", "reviewer": "codex-review"}, nil); got != "## Settings\n- Reviewer: codex-review ★\n" {
		t.Errorf("codex と同じブランドのレビュアー: %q", got)
	}
}

// 止め（予約キー hold）は、値があれば誰が書いても止めている。空でも、最後に消したのがその課題か子の課題の担当の端末なら
// 止めているものとして読む。Chief of Staff とオーナー（端末の外）が消したら外れる（ADR-0056 決めたこと 10）。
func TestHoldOf(t *testing.T) {
	row := func(by, id string, change map[string]any) Row { return Row{By: by, ID: id, Change: change} }
	base := []Row{
		row("terminal:cos", "A-0001", map[string]any{"add": "x"}),
		row("terminal:w1", "A-0001", map[string]any{"status": "doing", "agent": "w1"}),
		row("terminal:r1", "A-0001-r1", map[string]any{"status": "doing", "agent": "r1"}),
		row("terminal:cos", "A-0001", map[string]any{"hold": "A-0009"}),
	}
	tasks := []Issue{{"id": "A-0001", "agent": "w2"}, {"id": "A-0001-r1", "parent": "A-0001", "agent": "r1"}, {"id": "A-0002", "agent": "x9"}}
	cases := []struct {
		name   string
		hold   string
		extra  []Row
		reason string
		held   bool
	}{
		{"値がある", "A-0009", nil, "A-0009", true},
		{"担当が書いた値でも止まる", "A-0008", []Row{row("terminal:w1", "A-0001", map[string]any{"hold": "A-0008"})}, "A-0008", true},
		{"Chief of Staff が消した", "", []Row{row("terminal:cos", "A-0001", map[string]any{"hold": nil})}, "", false},
		{"オーナーが端末の外で消した", "", []Row{row("owner", "A-0001", map[string]any{"hold": nil})}, "", false},
		{"担当の端末が消した", "", []Row{row("terminal:w1", "A-0001", map[string]any{"hold": nil})}, "A-0009", true},
		{"今の担当（issue.yaml の agent）が消した", "", []Row{row("terminal:w2", "A-0001", map[string]any{"hold": nil})}, "A-0009", true},
		{"子の課題の担当が消した", "", []Row{row("terminal:r1", "A-0001", map[string]any{"hold": nil})}, "A-0009", true},
		{"ほかの課題の担当が消した", "", []Row{row("terminal:x9", "A-0001", map[string]any{"hold": nil})}, "", false},
		{"外したあとに担当が空を消し直した", "", []Row{row("terminal:cos", "A-0001", map[string]any{"hold": nil}), row("terminal:w1", "A-0001", map[string]any{"hold": nil})}, "", false},
		{"理由を書き換えたあと担当が消した", "", []Row{row("terminal:cos", "A-0001", map[string]any{"hold": "A-0007"}), row("terminal:w1", "A-0001", map[string]any{"hold": nil})}, "A-0007", true},
	}
	for _, c := range cases {
		rows := append(append([]Row{}, base...), c.extra...)
		reason, held := HoldOf(rows, tasks, "cos", Issue{"id": "A-0001", "hold": c.hold, "agent": "w2"})
		if reason != c.reason || held != c.held {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, reason, held, c.reason, c.held)
		}
	}
	if _, held := HoldOf(nil, nil, "cos", Issue{"id": "A-0001"}); held {
		t.Error("hold を一度も書いていない課題を止めていると読んだ")
	}
	rows := append(append([]Row{}, base...), row("terminal:cos", "A-0001", map[string]any{"agent": "cos"}), row("terminal:cos", "A-0001", map[string]any{"hold": nil}))
	if _, held := HoldOf(rows, tasks, "cos", Issue{"id": "A-0001"}); held {
		t.Error("Chief of Staff が消した止めを、担当が消したと読んだ")
	}
}

// log.jsonl を読めないとき、hold が空の課題の止めは分からない。Ledger.Hold は誤りを返し、呼び手は止めているものとして扱う
// （担当が消した止めを、読めないまま外れたと読まない。ADR-0056 決めたこと 10）。
func TestHoldUnreadableLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("version: 3\nprefix: A\nchief_of_staff: cos\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "log.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	l := &Ledger{Dir: dir, Slug: "demo"}
	rows, logErr := l.ReadLog()
	if _, _, err := l.Hold(rows, logErr, Issue{"id": "A-0001"}); err == nil {
		t.Error("log.jsonl を読めないのに、hold が空の課題を止めていないと読んだ")
	}
	if r, held, err := l.Hold(rows, logErr, Issue{"id": "A-0001", "hold": "A-0009"}); err != nil || !held || r != "A-0009" {
		t.Errorf("hold に値があるのに log を読もうとした: %q %v %v", r, held, err)
	}
	if err := os.Remove(filepath.Join(dir, "log.jsonl")); err != nil {
		t.Fatal(err)
	}
	rows, logErr = l.ReadLog()
	if _, held, err := l.Hold(rows, logErr, Issue{"id": "A-0001"}); err != nil || held {
		t.Errorf("log.jsonl がまだ無い kanban で止めていると読んだか誤りにした: %v %v", held, err)
	}
}

func TestLastHoldSet(t *testing.T) {
	rows := []Row{
		{At: "2026-01-01T00:00:01Z", ID: "A-0001", Change: map[string]any{"hold": "A-0007"}},
		{At: "2026-01-01T00:00:02Z", ID: "A-0001", Change: map[string]any{"status": "review"}},
		{At: "2026-01-01T00:00:03Z", ID: "A-0002", Change: map[string]any{"hold": "A-0008"}},
		{At: "2026-01-01T00:00:04Z", ID: "A-0001", Change: map[string]any{"hold": "A-0009"}},
		{At: "2026-01-01T00:00:05Z", ID: "A-0001", Change: map[string]any{"hold": nil}},
	}
	if r, at, ok := LastHoldSet(rows, "A-0001", 2); !ok || r != "A-0009" || at != "2026-01-01T00:00:04Z" {
		t.Errorf("review に入ったあとに止めた行を読まない: %q %q %v", r, at, ok)
	}
	if _, _, ok := LastHoldSet(rows, "A-0001", 4); ok {
		t.Error("外した行（値が空）を止めた行と読んだ")
	}
	if _, _, ok := LastHoldSet(rows[:3], "A-0001", 2); ok {
		t.Error("review に入る前の止めやほかの課題の止めを読んだ")
	}
}

// DeliverableOf は成果の形を照合を済ませて返す。report は書いた時点の Chief of Staff が書いたときだけ信じ、
// candidates は値だけで読み、値の無い課題と知らない値は pr（ADR-0066 決めたこと 10〜12・受け入れ条件 2）。
func TestDeliverableOf(t *testing.T) {
	cos := func(v string) Row { return Row{By: "andon", Config: map[string]any{"chief_of_staff": v}} }
	set := func(by, v string) Row {
		return Row{By: "terminal:" + by, ID: "S-0007", Change: map[string]any{"deliverable": v}}
	}
	for _, c := range []struct {
		name, value, now, want string
		rows                   []Row
	}{
		{"Chief of Staff が書いた report", "report", "t5", "report", []Row{cos("t5"), set("t5", "report")}},
		{"Chief of Staff が書いたあと起き直して端末が替わった", "report", "t6", "report", []Row{cos("t5"), set("t5", "report"), cos("t6")}},
		{"担当の端末が書いた report", "report", "t5", "pr", []Row{cos("t5"), set("t9", "report")}},
		{"Chief of Staff が書いたあと担当が書き直した", "report", "t5", "pr", []Row{cos("t5"), set("t5", "report"), set("t9", "report")}},
		{"行の無い report", "report", "t5", "pr", nil},
		{"Chief of Staff が書いた candidates", "candidates", "t5", "candidates", []Row{cos("t5"), set("t5", "candidates")}},
		{"担当が書いた candidates も値だけで読む", "candidates", "t5", "candidates", []Row{cos("t5"), set("t9", "candidates")}},
		{"値の無い課題", "", "t5", "pr", []Row{cos("t5")}},
		{"Chief of Staff が書いた pr", "pr", "t5", "pr", []Row{cos("t5"), set("t5", "pr")}},
		{"知らない値", "reports", "t5", "pr", []Row{cos("t5"), set("t5", "reports")}},
	} {
		issue := Issue{"id": "S-0007"}
		if c.value != "" {
			issue["deliverable"] = c.value
		}
		if got := DeliverableOf(c.rows, issue, c.now); got != c.want {
			t.Errorf("%s: %q（want %q）", c.name, got, c.want)
		}
	}
}

// Ledger.Deliverable は kanban の履歴を読んで DeliverableOf を呼び、report を pr と読んだときは理由の句を返す。
// 履歴に読めない行があれば（Chief of Staff の交代の行が抜けうる）report を pr と読む（ADR-0066 決めたこと 7・8・10）。
func TestLedgerDeliverable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("version: 3\nprefix: S\nchief_of_staff: t6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Ledger{Dir: dir, Slug: "demo"}
	write := func(lines ...string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "log.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report := Issue{"id": "S-0007", "deliverable": "report"}
	for _, c := range []struct {
		name      string
		lines     []string
		issue     Issue
		form, why string
	}{
		{"Chief of Staff が書いて起き直した", []string{`{"by":"andon","config":{"chief_of_staff":"t5"}}`,
			`{"by":"terminal:t5","id":"S-0007","change":{"deliverable":"report"}}`, `{"by":"andon","config":{"chief_of_staff":"t6"}}`},
			report, "report", ""},
		{"担当が書いた", []string{`{"by":"andon","config":{"chief_of_staff":"t6"}}`, `{"by":"terminal:t9","id":"S-0007","change":{"deliverable":"report"}}`},
			report, "pr", "Chief of Staff でない"},
		{"読めない行がある", []string{`{"by":"andon","config":{"chief_of_staff":"t5"}}`,
			`{"by":"terminal:t5","id":"S-0007","change":{"deliverable":"report"}}`, `{broken`},
			report, "pr", "確かめられない"},
		{"candidates は履歴を見ない", []string{`{broken`}, Issue{"id": "S-0007", "deliverable": "candidates"}, "candidates", ""},
		{"値の無い課題", []string{`{broken`}, Issue{"id": "S-0007"}, "pr", ""},
	} {
		write(c.lines...)
		form, why := l.Deliverable(c.issue)
		if form != c.form || (c.why == "") != (why == "") || !strings.Contains(why, c.why) {
			t.Errorf("%s: %q %q（want %q と %q を含む理由）", c.name, form, why, c.form, c.why)
		}
	}
}

// TrustedThen は、キーを最後に変えた行の書き手を、その行の時点の Chief of Staff と比べる（ADR-0062 決めたこと 3）。
func TestTrustedThen(t *testing.T) {
	cos := func(v any) Row { return Row{By: "andon", Config: map[string]any{"chief_of_staff": v}} }
	set := func(by string, v any) Row {
		return Row{By: "terminal:" + by, ID: "S-0007", Change: map[string]any{"later": v}}
	}
	for _, c := range []struct {
		name  string
		rows  []Row
		value string
		now   string
		want  bool
	}{
		{"書いたあとで Chief of Staff が起き直した", []Row{cos("t5"), set("t5", "急がない"), cos("t6")}, "急がない", "t6", true},
		{"担当が書いた", []Row{cos("t5"), set("t9", "急がない")}, "急がない", "t5", false},
		{"Chief of Staff だったアドレスを、あとで担当が使って書いた", []Row{cos("t5"), cos("t6"), set("t5", "急がない")}, "急がない", "t6", false},
		{"Chief of Staff が書いたあと、担当が書き換えた", []Row{cos("t5"), set("t5", "急がない"), set("t9", "あとで")}, "あとで", "t5", false},
		{"今の値が書いた値と違う", []Row{cos("t5"), set("t5", "急がない")}, "あとで", "t5", false},
		{"消した（空）", []Row{cos("t5"), set("t5", nil)}, "", "t5", false},
		{"config の行が無く、今の Chief of Staff が書いた", []Row{set("t5", "急がない")}, "急がない", "t5", true},
		{"config の行が無く、ほかの端末が書いた", []Row{set("t9", "急がない")}, "急がない", "t5", false},
		{"書いたあとで初めて config の行が来た", []Row{set("t5", "急がない"), cos("t5")}, "急がない", "t5", false},
		{"前の名前（coordinator）の行で Chief of Staff だった", []Row{{By: "andon", Config: map[string]any{"coordinator": "t5"}}, set("t5", "急がない")}, "急がない", "t5", true},
		{"行が無い", nil, "急がない", "t5", false},
	} {
		issue := Issue{"id": "S-0007"}
		if c.value != "" {
			issue["later"] = c.value
		}
		if got := TrustedThen(c.rows, issue, "later", c.now); got != c.want {
			t.Errorf("%s: %v（want %v）", c.name, got, c.want)
		}
	}
}
