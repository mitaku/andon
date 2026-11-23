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

// fakeBin は name の名前で、kanban の根に dir を答える偽の CLI を置いた PATH を返す。
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

// kanban が PATH に無ければ、入れ方を案内して止める。
func TestOpenRefusesWithoutKanban(t *testing.T) {
	t.Setenv("PATH", fakeBin(t, t.TempDir()))
	_, err := Open()
	if err == nil || !strings.Contains(err.Error(), "kanban が無い") || !strings.Contains(err.Error(), "go install ./cmd/kanban") {
		t.Fatalf("Open() without kanban: err = %v; want kanban が無い and how to install it", err)
	}
}

// kanban があればそれを打つ。
func TestOpenUsesKanban(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", fakeBin(t, root, "kanban"))
	l, err := Open()
	if err != nil || l.Root != root {
		t.Fatalf("Open() = %+v, %v; want root %s", l, err, root)
	}
}

// オーナーが短く打った id と、移す前の log.jsonl の id は 4 桁に揃えて読む。
func TestCanonID(t *testing.T) {
	for in, want := range map[string]string{"A-5": "A-0005", "A-05": "A-0005", "A-0005": "A-0005", "A-12345": "A-12345", "Y-T-7": "Y-T-0007", "A-0": "A-0", "A-x": "A-x", "abc": "abc", "-5": "-5", "A-": "A-",
		"A-5-r1": "A-0005-r1", "A-0005-r01": "A-0005-r1", "A-0005-r0": "A-0005-r0", "A-0005-r": "A-0005-r", "A-0005-rx": "A-0005-rx", "A-12345-r3": "A-12345-r3"} {
		if got := CanonID(in); got != want {
			t.Errorf("CanonID(%q) = %q, want %q", in, got, want)
		}
	}
}

// レビュアーの課題は親のすぐあとに回の順で並ぶ（番号が 4 桁を超えても）。
func TestIdLess(t *testing.T) {
	ids := []string{"A-10000", "A-0006", "A-0005-r10", "A-0005-r2", "A-0005", "A-0005-r1"}
	sort.SliceStable(ids, func(i, j int) bool { return IDLess(ids[i], ids[j]) })
	if got := strings.Join(ids, " "); got != "A-0005 A-0005-r1 A-0005-r2 A-0005-r10 A-0006 A-10000" {
		t.Fatalf("got %s", got)
	}
}

// 移す前の行（id: A-5・by: Chief of Staff）は、移したあとの課題 A-0005 の照合に数える。
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

// Log は kanban が書く長さ（16 MiB まで）の行を読み、その先の行も読む。長い行で読むのを止めると、
// 後の行（Chief of Staff が書いた予約キー）が無いことになる。
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
		row("A-0002", "terminal:t7", "candidates"),                                             // 担当が書いた
		row("A-0003", "terminal:t5", "candidates"), row("A-0003", "terminal:t7", "candidates"), // 最後が担当
		row("A-0004", "terminal:t5", "pr"),
	}
	for _, c := range []struct {
		task Issue
		want bool
	}{
		{Issue{"id": "A-0001", "deliverable": "candidates"}, true},
		{Issue{"id": "A-0002", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0003", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0004", "deliverable": "candidates"}, false}, // 履歴の値と違う
		{Issue{"id": "A-0004", "deliverable": "pr"}, false},
		{Issue{"id": "A-0005"}, false}, // 無い＝pr
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
		row("A-0002", "terminal:t7", "large"),                                        // 担当が書いた
		row("A-0003", "terminal:t5", "small"), row("A-0003", "terminal:t7", "large"), // 最後が担当
		row("A-0004", "terminal:t5", "中"), // 段の名は値でない
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
		{Issue{"id": "A-0005", "estimate": "large"}, "large", ""}, // 履歴の値と違う
		{Issue{"id": "A-0006"}, "", ""},                           // 無い＝見込み無し（既定で埋めない）
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
		v        any // nil ならキーが無い
		ids      string
		ok       bool
		settings string
	}{
		{nil, "", true, ""},
		{"A4", "A4", true, "A4"},
		{"A4,A13,A4", "A4,A13", true, "A4,A13,A4"}, // 重なりは1つに（settings は書いたまま）
		{"4,13", "4,13", true, "4,13"},             // Chief Engineer の文書の番号の振り方（数字だけ）
		{"4,A4,13", "4,13", true, "4,A4,13"},       // 4 と A4 は同じ番号
		{"A01,A1", "A01,A1", true, "A01,A1"},       // 数字の綴りは変えない
		{"A1, A4", "", false, ""},                  // 空白
		{"A1,,A4", "", false, ""},                  // 空の番号
		{"", "", false, ""},
		{[]any{"A1"}, "", false, ""}, // 文字列でない
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

// 受け入れ条件の番号は A を付けた形（A4）と数字だけの形（4）を同じ番号と読む。A の後が数字だけでなければそのまま。
func TestAcceptanceKey(t *testing.T) {
	for id, want := range map[string]string{"4": "4", "A4": "4", "A13": "13", "A01": "01", "A": "A", "A4a": "A4a", "a4": "a4", "B4": "B4", "AA4": "AA4"} {
		if got := AcceptanceKey(id); got != want {
			t.Errorf("AcceptanceKey(%q) = %q, want %q", id, got, want)
		}
	}
}

// config.yaml の repos は block の並びも flow の並び（repos: [x]）も読む。chief_of_staff・timezone は引用符と行末のコメントを除く。
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

// Unsettled は既定のうち、無いキーと、既定と同じ値なのに照合が外れたキーを返す（command が足した課題の打ち直しで書き直す）。
// Chief of Staff が書いた値と、既定と違う値（誰かが選んだもの）は返さない。
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

// 依頼書の ## Settings の写しは予約キー（正）に合わせて書き直す。写しの後ろの地の文と追記の節は触らない。
func TestRewriteSettings(t *testing.T) {
	const tail = "\n頼まれていない稀なケースのために設計しない。\n\n## 追記（2026-11-23）\n- Review: off のままにしない\n"
	for _, c := range []struct {
		name, brief, want string
		issue             Issue
	}{
		{"あとから書き直した予約キー",
			"# 題\n\n## Settings\n- Review: off\n- Must fix: major\n- Merge: person\n- Estimate: 中\n" + tail,
			"# 題\n\n## Settings\n- Review: on\n- Must fix: major\n- Merge: auto\n- Estimate: 中\n- Reviewer: codex-review\n- Review rounds: 2\n" + tail,
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
			"## Settings\n- Review: off", "## Settings\n- Review: on\n- Merge: auto\n",
			Issue{"review": "on", "merge": "auto"}},
		{"見出しで終わる依頼書", "## Settings", "## Settings\n- Review: on\n", Issue{"review": "on"}},
		{"CRLF", "## Settings\r\n- Review: off\r\n", "## Settings\r\n- Review: on\r\n", Issue{"review": "on"}},
		{"写しが無ければ書き足さない", "# 題\n完了条件: 押せる\n", "", Issue{"review": "on"}},
		{"見出しが2つあれば両方",
			"## Settings\n- Review: off\n\n## 追記\n## Settings\n- Review: off\n- Merge: person\n",
			"## Settings\n- Review: on\n- Merge: auto\n\n## 追記\n## Settings\n- Review: on\n- Merge: auto\n",
			Issue{"review": "on", "merge": "auto"}},
		{"コード例の中の見出しは触らない",
			"例:\n```\n## Settings\n- Review: off\n```\n\n## Settings\n- Review: off\n",
			"例:\n```\n## Settings\n- Review: off\n```\n\n## Settings\n- Review: on\n",
			Issue{"review": "on"}},
		{"4つのバッククォートの囲みの中の3つの囲みと見出しは触らない",
			"````\n```\n## Settings\n- Review: off\n```\n## Settings\n- Review: off\n````\n\n## Settings\n- Review: off\n",
			"````\n```\n## Settings\n- Review: off\n```\n## Settings\n- Review: off\n````\n\n## Settings\n- Review: on\n",
			Issue{"review": "on"}},
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
			got, changed := RewriteSettings(c.brief, c.issue)
			if got != want || changed != (want != c.brief) {
				t.Errorf("changed=%v\n got %q\nwant %q", changed, got, want)
			}
			// 書き直したものをもう一度書き直しても変わらない（打ち直しても写しが動かない）
			if again, ch := RewriteSettings(got, c.issue); ch || again != got {
				t.Errorf("2回目で変わった: %q", again)
			}
		})
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
	// 担当が Chief of Staff の端末と同じ（Chief of Staff が自分で引き取った課題）なら、消したのは Chief of Staff として外す
	rows := append(append([]Row{}, base...), row("terminal:cos", "A-0001", map[string]any{"agent": "cos"}), row("terminal:cos", "A-0001", map[string]any{"hold": nil}))
	if _, held := HoldOf(rows, tasks, "cos", Issue{"id": "A-0001"}); held {
		t.Error("Chief of Staff が消した止めを、担当が消したと読んだ")
	}
}
