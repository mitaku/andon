package signal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
)

const repo = "github.com/example/sample-site"

// fake は偽の gh。ci と issues が応答で、err があれば落ちる。打たれた引数を calls に残す。
type fake struct {
	ci, issues string
	err        error
	calls      [][]string
}

func (f *fake) gh(args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		return nil, f.err
	}
	if args[0] == "api" {
		return []byte(f.ci), nil
	}
	return []byte(f.issues), nil
}

const (
	ciRed = `{"workflow_runs": [
 {"path": ".github/workflows/ci.yml", "name": "ci", "event": "push", "conclusion": "failure", "html_url": "https://github.com/example/sample-site/actions/runs/3", "head_sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "display_title": "Fix\nexport"},
 {"path": ".github/workflows/ci.yml", "name": "ci", "event": "push", "conclusion": "success", "html_url": "https://github.com/example/sample-site/actions/runs/2"},
 {"path": ".github/workflows/lint.yml", "name": "lint", "event": "pull_request", "conclusion": "failure", "html_url": "https://github.com/example/sample-site/actions/runs/4"},
 {"path": ".github/workflows/lint.yml", "name": "lint", "event": "push", "conclusion": "cancelled", "html_url": "https://github.com/example/sample-site/actions/runs/5"},
 {"path": ".github/workflows/lint.yml", "name": "lint", "event": "push", "conclusion": "success", "html_url": "https://github.com/example/sample-site/actions/runs/1"}]}`
	ciGreen = `{"workflow_runs": [{"path": ".github/workflows/ci.yml", "name": "ci", "event": "push", "conclusion": "success", "html_url": "https://github.com/example/sample-site/actions/runs/6"}]}`
	issues2 = `[{"number": 12, "title": "Export breaks <b>", "url": "https://github.com/example/sample-site/issues/12"},
 {"number": 13, "title": "Typo", "url": "https://github.com/example/sample-site/issues/13"}]`
)

func project(filter *string) *delegate.Project {
	r := repo
	return &delegate.Project{Repos: []string{repo}, Signals: []delegate.Signal{
		{Source: "github-ci", Repo: &r, Weight: "now"},
		{Source: "github-issues", Weight: "later", Filter: filter}}}
}

func collect(t *testing.T, o Options) *File {
	t.Helper()
	var out strings.Builder
	failed, err := Collect(o, &out)
	if err != nil || failed != 0 {
		t.Fatalf("collect: failed=%d err=%v\n%s", failed, err, out.String())
	}
	f, err := Load(o.Path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func find(f *File, key string) Record {
	for _, r := range f.Signals {
		if r.Key == key {
			return r
		}
	}
	return Record{}
}

// ADR-0011 受け入れ条件 2: main の CI の失敗1件と open の issue 2件を書き、2回目で count が増えて件数は増えない。
// CI が通ったあとは active: false、また失敗すると episodes が 1 増える。delivered_episode は触らない。
func TestCollectGitHub(t *testing.T) {
	dir := t.TempDir()
	g := &fake{ci: ciRed, issues: issues2}
	t0 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	o := Options{Slug: "sample-site", Project: project(nil), Path: filepath.Join(dir, "s", "sample-site.signals.json"), TokenDir: dir, Now: t0, GH: g.gh}
	f := collect(t, o)
	if len(f.Signals) != 3 {
		t.Fatalf("3件のはず: %+v", f.Signals)
	}
	ci := find(f, repo+"/ci.yml@main")
	want := Record{Source: "github-ci", Project: "sample-site", Key: repo + "/ci.yml@main", URL: "https://github.com/example/sample-site/actions/runs/3",
		Title: "ci: Fix export", Weight: "now", Fingerprint: Fingerprint("github-ci", repo+"/ci.yml@main"),
		FirstSeen: "2026-11-10T09:00:00Z", LastSeen: "2026-11-10T09:00:00Z", Count: 1, Active: true, Episodes: 1,
		Release: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}
	if !reflect.DeepEqual(ci, want) {
		t.Errorf("CI の記録:\n got %+v\nwant %+v", ci, want)
	}
	if is := find(f, repo+"#12"); is.Weight != "later" || is.Title != "Export breaks <b>" || is.URL != "https://github.com/example/sample-site/issues/12" {
		t.Errorf("issue の記録: %+v", is)
	}
	if got := Fingerprint("github-ci", repo+"/ci.yml@main"); got != "af3c0a047c19a9c56867fe43afe6794dd408eb8f" {
		t.Errorf("fingerprint が contract.md §15 の例と違う: %s", got)
	}

	// dispatcher が届けた（S3）とする。collect はこれを触らない
	f.Signals[0].DeliveredEpisode = 1
	writeFile(t, o.Path, f)
	o.Now = t0.Add(10 * time.Minute)
	f = collect(t, o)
	ci = find(f, repo+"/ci.yml@main")
	if len(f.Signals) != 3 || ci.Count != 2 || ci.Episodes != 1 || !ci.Active || ci.LastSeen != "2026-11-10T09:10:00Z" || ci.FirstSeen != "2026-11-10T09:00:00Z" || ci.DeliveredEpisode != 1 {
		t.Errorf("2回目: 件数 %d・%+v", len(f.Signals), ci)
	}

	g.ci, g.issues = ciGreen, issues2[:strings.Index(issues2, ",\n")]+"]"
	o.Now = t0.Add(20 * time.Minute)
	f = collect(t, o)
	if ci = find(f, repo+"/ci.yml@main"); ci.Active || ci.Count != 2 || ci.Episodes != 1 {
		t.Errorf("CI が通ったあと: %+v", ci)
	}
	if is := find(f, repo+"#13"); is.Active {
		t.Errorf("閉じた issue が active: %+v", is)
	}

	g.ci = ciRed
	o.Now = t0.Add(30 * time.Minute)
	f = collect(t, o)
	if ci = find(f, repo+"/ci.yml@main"); !ci.Active || ci.Count != 3 || ci.Episodes != 2 || ci.DeliveredEpisode != 1 {
		t.Errorf("また失敗: %+v", ci)
	}

	// 読んだ 100 件に ci.yml の結果の出た run が無い（取り消しだけ・ページの外）なら、失敗中の記録を消えたことにしない。
	// issue が上限まで返ったら、その外があるかもしれないので、見えなかった issue を消えたことにしない（レビュー 1 回目）
	g.ci = `{"workflow_runs": [{"path": ".github/workflows/ci.yml", "name": "ci", "event": "push", "conclusion": "cancelled", "html_url": "https://github.com/example/sample-site/actions/runs/8"},
 {"path": ".github/workflows/lint.yml", "name": "lint", "event": "push", "conclusion": "success", "html_url": "https://github.com/example/sample-site/actions/runs/9"}]}`
	g.issues = `[{"number": 14, "title": "New", "url": "https://github.com/example/sample-site/issues/14"}]`
	defer func(n int) { issueLimit = n }(issueLimit)
	issueLimit = 1
	o.Now = t0.Add(40 * time.Minute)
	f = collect(t, o)
	if ci = find(f, repo+"/ci.yml@main"); !ci.Active || ci.Count != 3 || ci.Episodes != 2 {
		t.Errorf("ページの外の CI が消えた: %+v", ci)
	}
	if is := find(f, repo+"#12"); !is.Active {
		t.Errorf("上限まで返ったのに issue を消えたことにした: %+v", is)
	}
	if is := find(f, repo+"#14"); !is.Active || is.Count != 1 {
		t.Errorf("上限の中の issue: %+v", is)
	}
}

func writeFile(t *testing.T, path string, f *File) {
	t.Helper()
	if err := os.WriteFile(path, []byte(mustMarshal(f)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ADR-0011 受け入れ条件 5: signals が無ければ何も読まず書かない。トークンが無い出所は飛ばして1行。API が落ちたら書かない。
func TestCollectSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.signals.json")
	g := &fake{ci: ciRed, issues: issues2}
	for name, p := range map[string]*delegate.Project{"project.json が無い": nil, "signals が無い": {Repos: []string{repo}}} {
		var out strings.Builder
		if failed, err := Collect(Options{Slug: "x", Project: p, Path: path, TokenDir: dir, GH: g.gh}, &out); err != nil || failed != 0 {
			t.Errorf("%s: %d %v", name, failed, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) || len(g.calls) != 0 {
			t.Errorf("%s: 書いた・読んだ（%v）", name, g.calls)
		}
	}

	var out strings.Builder
	p := &delegate.Project{Signals: []delegate.Signal{{Source: "sentry", Weight: "later"}}}
	if failed, err := Collect(Options{Slug: "x", Project: p, Path: path, TokenDir: dir, GH: g.gh}, &out); err != nil || failed != 0 {
		t.Fatalf("%d %v", failed, err)
	}
	if !strings.Contains(out.String(), "sentry: トークンが無いので飛ばす（andon signal auth sentry）") {
		t.Errorf("1行が無い: %q", out.String())
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("読んでいないのに書いた")
	}

	o := Options{Slug: "x", Project: project(nil), Path: path, TokenDir: dir, Now: time.Now(), GH: g.gh}
	before := collect(t, o)
	b0, _ := os.ReadFile(path)
	g.err = errors.New("HTTP 502")
	out.Reset()
	failed, err := Collect(o, &out)
	if err != nil || failed != 2 || !strings.Contains(out.String(), "読めないので信号の記録は変えずに飛ばす") {
		t.Errorf("落ちた API: %d %v %q", failed, err, out.String())
	}
	if b1, _ := os.ReadFile(path); string(b0) != string(b1) || len(before.Signals) != 3 {
		t.Error("落ちた API で記録が変わった")
	}

	// 片方だけ落ちたら、落ちた出所の記録は active のまま
	g.err = nil
	calls := 0
	o.GH = func(args ...string) ([]byte, error) {
		calls++
		if args[0] == "api" {
			return nil, errors.New("HTTP 502")
		}
		return []byte("[]"), nil
	}
	out.Reset()
	if failed, _ := Collect(o, &out); failed != 1 {
		t.Errorf("片方が落ちた: %d", failed)
	}
	f, _ := Load(path)
	if !find(f, repo+"/ci.yml@main").Active || find(f, repo+"#12").Active {
		t.Errorf("落ちた出所だけ変えないはず: %+v", f.Signals)
	}
}

// ADR-0011 受け入れ条件 12: filter が GitHub の検索にそのまま渡る。repo の無い出所は repos を読む。
func TestFilterPassedAsIs(t *testing.T) {
	filter := "label:bug -author:<owner>"
	g := &fake{ci: `{"workflow_runs": []}`, issues: "[]"}
	collect(t, Options{Slug: "x", Project: project(&filter), Path: filepath.Join(t.TempDir(), "x.json"), Now: time.Now(), GH: g.gh})
	want := [][]string{
		{"api", "--hostname", "github.com", "repos/example/sample-site/actions/runs?branch=main&status=completed&per_page=100"},
		{"issue", "list", "--repo", repo, "--state", "open", "--limit", "1000", "--json", "number,title,url", "--search", filter},
	}
	if !reflect.DeepEqual(g.calls, want) {
		t.Errorf("gh の引数:\n got %q\nwant %q", g.calls, want)
	}
}

// ADR-0011 受け入れ条件 12: auth が置いたファイルが mode 600（dir は 700）。GitHub は置かない。
func TestAuth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "signals")
	var out strings.Builder
	if err := Auth("sentry", dir, strings.NewReader("tok-123\n"), &out); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, "sentry.token"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", st, err)
	}
	if d, _ := os.Stat(dir); d.Mode().Perm() != 0o700 {
		t.Errorf("dir の mode: %v", d.Mode())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "sentry.token")); string(b) != "tok-123\n" {
		t.Errorf("中身: %q", b)
	}
	for _, c := range []struct{ src, in string }{{"github-ci", "x\n"}, {"datadog", "x\n"}, {"linear", "\n"}, {"linear", "a b\n"}, {"linear", ""}} {
		if err := Auth(c.src, dir, strings.NewReader(c.in), &out); err == nil {
			t.Errorf("%s %q: 通った", c.src, c.in)
		}
	}
	// トークンがあれば、読む command がまだ無いと言って飛ばす
	out.Reset()
	p := &delegate.Project{Signals: []delegate.Signal{{Source: "sentry", Weight: "later"}}}
	if _, err := Collect(Options{Slug: "x", Project: p, Path: filepath.Join(dir, "x.json"), TokenDir: dir}, &out); err != nil || !strings.Contains(out.String(), "読む command がまだ無い") {
		t.Errorf("%v %q", err, out.String())
	}
}

func mustMarshal(f *File) string {
	var b strings.Builder
	b.Write(contract.Marshal(f))
	return b.String()
}

// ADR-0011 受け入れ条件 9（every）: --due は出所と repo の組ごとに、前に読みに行った時刻から every 分（既定 10）たつまで読まない。
// 読めなかった組も時刻を書く（gh が落ちている間も every より短く読みに行かない）。--due でなければいつも読む。
func TestCollectDue(t *testing.T) {
	every := 30
	r := repo
	p := &delegate.Project{Repos: []string{repo}, Signals: []delegate.Signal{
		{Source: "github-ci", Repo: &r, Weight: "now"},
		{Source: "github-issues", Weight: "later", Every: &every}}}
	g := &fake{ci: ciRed, issues: issues2}
	t0 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	o := Options{Slug: "x", Project: p, Path: filepath.Join(t.TempDir(), "x.signals.json"), Now: t0, GH: g.gh, Due: true}
	reads := func(at time.Duration) (ci, issues int) {
		t.Helper()
		g.calls = nil
		o.Now = t0.Add(at)
		var out strings.Builder
		Collect(o, &out)
		for _, c := range g.calls {
			if c[0] == "api" {
				ci++
			} else {
				issues++
			}
		}
		return
	}
	for _, c := range []struct {
		at         time.Duration
		ci, issues int
	}{{0, 1, 1}, {time.Minute, 0, 0}, {9*time.Minute + 59*time.Second, 0, 0}, {10 * time.Minute, 1, 0}, {19 * time.Minute, 0, 0}, {20 * time.Minute, 1, 0}, {29 * time.Minute, 0, 0}, {30 * time.Minute, 1, 1}} {
		if ci, issues := reads(c.at); ci != c.ci || issues != c.issues {
			t.Errorf("%v: CI %d 回・issue %d 回（want %d・%d）", c.at, ci, issues, c.ci, c.issues)
		}
	}
	f, _ := Load(o.Path)
	if f.ReadAt["github-ci "+repo] != "2026-11-10T09:30:00Z" || f.ReadAt["github-issues "+repo] != "2026-11-10T09:30:00Z" {
		t.Errorf("read_at: %v", f.ReadAt)
	}

	// 読めなかった組も、次に読むのは every のあと（記録は変えない）
	g.err = errors.New("HTTP 403 API rate limit exceeded")
	if ci, _ := reads(40 * time.Minute); ci != 1 {
		t.Fatalf("40 分: CI を読みに行かない")
	}
	g.err = nil
	if ci, _ := reads(41 * time.Minute); ci != 0 {
		t.Error("読めなかった組を every より前に読み直した")
	}
	if after, _ := Load(o.Path); !reflect.DeepEqual(after.Signals, f.Signals) {
		t.Error("読めなかったのに記録が変わった")
	}

	// --due でなければ、間隔によらず読む（人が手で打つ）
	o.Due = false
	if ci, issues := reads(42 * time.Minute); ci != 1 || issues != 1 {
		t.Errorf("--due でないのに読まない: %d %d", ci, issues)
	}
}

// ADR-0011 決めたこと 8・ADR-0017 決めたこと 5 の (c): delivered は届けた出来事を delivered_episode に書く。
// 同じか前の出来事では書かない（1回だけ）。記録に無い信号・まだ無い出来事は誤り。Undelivered は届ける now の信号だけを返す。
func TestDelivered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.signals.json")
	rec := func(key, weight string, active bool, episodes, delivered int) Record {
		return Record{Source: "github-ci", Project: "x", Key: key, URL: "https://example.com/" + key, Title: key, Weight: weight,
			Fingerprint: Fingerprint("github-ci", key), FirstSeen: "2026-11-10T09:00:00Z", LastSeen: "2026-11-10T09:00:00Z",
			Count: 1, Active: active, Episodes: episodes, DeliveredEpisode: delivered}
	}
	f := &File{Signals: []Record{rec("a", "now", true, 1, 0), rec("b", "now", true, 2, 1), rec("c", "now", true, 1, 1),
		rec("d", "now", false, 1, 0), rec("e", "later", true, 1, 0)}}
	f.Signals = append(f.Signals, rec("f", "now", true, 1, 0))
	f.Signals[5].Project = "other"
	writeFile(t, path, f)
	var got []string
	for _, r := range Undelivered(f, "x") {
		got = append(got, r.Key)
	}
	if strings.Join(got, ",") != "a,b" {
		t.Errorf("Undelivered: %v（a: 初めて・b: また出た。c は届けた・d は消えた・e は later・f は別のプロジェクト）", got)
	}

	fp := Fingerprint("github-ci", "b")
	if wrote, err := Delivered(path, fp, 2); err != nil || !wrote {
		t.Fatalf("b の 2: %v %v", wrote, err)
	}
	if wrote, err := Delivered(path, fp, 2); err != nil || wrote {
		t.Errorf("同じ出来事を2回書いた: %v %v", wrote, err)
	}
	if wrote, err := Delivered(path, fp, 1); err != nil || wrote {
		t.Errorf("前の出来事で戻した: %v %v", wrote, err)
	}
	for _, c := range []struct {
		fp string
		e  int
	}{{fp, 3}, {fp, 0}, {Fingerprint("github-ci", "zz"), 1}} {
		if _, err := Delivered(path, c.fp, c.e); err == nil {
			t.Errorf("%s %d: 通った", c.fp, c.e)
		}
	}
	after, _ := Load(path)
	if b := find(after, "b"); b.DeliveredEpisode != 2 || b.Episodes != 2 || b.Count != 1 {
		t.Errorf("b: %+v", b)
	}
	if len(Undelivered(after, "x")) != 1 {
		t.Errorf("b を届けたあとも残る: %+v", Undelivered(after, "x"))
	}
}
