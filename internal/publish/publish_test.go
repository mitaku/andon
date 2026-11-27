package publish

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/tray"
	"andon/internal/tray/traytest"
)

var now = time.Date(2026, 1, 2, 23, 4, 5, 600_000_000, time.UTC)

func env(t *testing.T) (Env, *bytes.Buffer) {
	t.Helper()
	var w bytes.Buffer
	// セルの repo（project.json の無い git の repo）。Cloudflare の tray を選んだかを、本物の読み口で「選んでいない」と読む
	return Env{Workspace: traytest.CellRepo(t, traytest.Owner, nil), Project: "sample-site", Now: now, Warn: &w}, &w
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func filesIn(t *testing.T, ws string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(ws, "files", "*"))
	return m
}

// 受け入れ条件 1 の Go の側: ローカルの tray に送り、URL を1つ返し、files/ には写さない。
func TestPublishSendsToLocalTray(t *testing.T) {
	f := traytest.New(t)
	traytest.Local(t, f)
	e, warn := env(t)
	src := writeTemp(t, "review.md", "# 週の報告\n\n本文\n")
	url, err := Publish(e, src, "weekly", "")
	if err != nil {
		t.Fatal(err)
	}
	got := f.Received()
	if len(got) != 1 || got[0].Project != "sample-site" || got[0].Name != "2026-01-02-weekly" || got[0].Title != "週の報告" ||
		got[0].Kind != "publish" || got[0].Type != "text/markdown; charset=utf-8" || got[0].Auth != "" {
		t.Fatalf("送った物: %+v", got)
	}
	if url != f.URL+"/r/sample-site%2F2026-01-02-weekly" {
		t.Errorf("url = %q", url)
	}
	if m := filesIn(t, e.Workspace); len(m) != 0 {
		t.Errorf("tray に届いたのに files/ に写した: %v", m)
	}
	if warn.Len() != 0 {
		t.Errorf("警告: %s", warn)
	}
}

// 決めたこと 9: slug を省くと名前に時刻を足し、同じファイル名でも別の報告になる。タイトルの決め方。
func TestPublishNamesAndTitles(t *testing.T) {
	f := traytest.New(t)
	traytest.Local(t, f)
	e, _ := env(t)
	src := writeTemp(t, "a.html", "<html><head><title>HTML のタイトル</title></head><body>x</body></html>")
	if _, err := Publish(e, src, "", ""); err != nil {
		t.Fatal(err)
	}
	e.Now = now.Add(time.Second)
	if _, err := Publish(e, src, "", "--title のタイトル"); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(e, writeTemp(t, "plain.txt", "# 見出しに見えても text"), "x", ""); err != nil {
		t.Fatal(err)
	}
	got := f.Received()
	if len(got) != 3 || got[0].Name != "2026-01-02-a-230405.600" || got[1].Name != "2026-01-02-a-230406.600" {
		t.Fatalf("名前: %+v", got)
	}
	if got[0].Title != "HTML のタイトル" || got[1].Title != "--title のタイトル" || got[2].Title != "plain.txt" {
		t.Errorf("タイトル: %q %q %q", got[0].Title, got[1].Title, got[2].Title)
	}
	if !strings.Contains(string(got[0].Body), `<base target="_blank"/>`) {
		t.Errorf("HTML の写しを整えていない: %s", got[0].Body)
	}
}

// 決めたこと 10: 相対で引いた画像を埋め込み、外の参照の数を stderr に出す。
func TestPublishEmbedsReferences(t *testing.T) {
	f := traytest.New(t)
	traytest.Local(t, f)
	e, warn := env(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "r.html")
	if err := os.WriteFile(src, []byte(`<img src="a.png"><img src="https://example.com/b.png">`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(e, src, "r", ""); err != nil {
		t.Fatal(err)
	}
	if b := string(f.Received()[0].Body); !strings.Contains(b, `src="data:image/png;base64,UE5H"`) || !strings.Contains(b, `src="https://example.com/b.png"`) {
		t.Errorf("埋め込み: %s", b)
	}
	if !strings.Contains(warn.String(), "http・https の参照 1 個は埋めずに残した") {
		t.Errorf("警告: %q", warn)
	}
}

// 受け入れ条件 2: ローカルの tray が止まっているときは files/ に写し、そのパスを出し、終了コードは 0。
func TestMainFallsBackToFilesWhenTrayIsDown(t *testing.T) {
	traytest.Local(t, nil)
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	t.Setenv("KANBAN_ROOT", filepath.Join(ws, "projects", "sample-site"))
	src := writeTemp(t, "report.html", "<p>x</p>")
	var out, errb bytes.Buffer
	if rc := Main([]string{src, "weekly"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	made, _ := filepath.Glob(filepath.Join(ws, "files", "*-weekly.html"))
	if len(made) != 1 {
		t.Fatalf("files/ に1つ書くはず: %v", made)
	}
	if strings.TrimSpace(out.String()) != made[0] {
		t.Errorf("stdout = %q（files/ のパス %s のはず）", out.String(), made[0])
	}
	if b, _ := os.ReadFile(made[0]); !strings.Contains(string(b), "<p>x</p>") {
		t.Errorf("写した中身: %q", b)
	}
	if !strings.Contains(errb.String(), "ローカルの tray に送れない") || !strings.Contains(errb.String(), "files/ に写した") {
		t.Errorf("stderr に理由が無い: %q", errb.String())
	}
}

// tray が答えで断っても（落ちている）、files/ に倒れて 0。
func TestPublishFallsBackWhenTrayRefuses(t *testing.T) {
	f := traytest.New(t)
	traytest.Local(t, f)
	f.Fail(503)
	e, warn := env(t)
	out, err := Publish(e, writeTemp(t, "a.md", "x"), "a", "")
	if err != nil || out != filepath.Join(e.Workspace, "files", "2026-01-02-a.md") || !strings.Contains(warn.String(), "落ちている") {
		t.Errorf("out=%q err=%v warn=%q", out, err, warn)
	}
}

// 決めたこと 8: Cloudflare の tray に届けばその URL を出す。届かなければローカルの URL。
func TestPublishPrefersRemote(t *testing.T) {
	loc, cf := traytest.New(t), traytest.New(t)
	traytest.Local(t, loc)
	traytest.Remote(t, cf, "sample-site", "tray_key000000000000")
	e, _ := env(t)
	src := writeTemp(t, "a.md", "# a\n")
	url, err := Publish(e, src, "a", "")
	if err != nil || !strings.HasPrefix(url, cf.URL) || len(loc.Received()) != 1 || cf.Received()[0].Auth != "Bearer tray_key000000000000" {
		t.Fatalf("url=%q err=%v", url, err)
	}
	cf.Fail(500)
	if url, _ := Publish(e, src, "a", ""); !strings.HasPrefix(url, loc.URL) {
		t.Errorf("Cloudflare が落ちたときの url = %q", url)
	}
	// 選んでいないプロジェクトは Cloudflare へ送らない
	e.Project = "other"
	n := len(cf.Received())
	if _, err := Publish(e, src, "b", ""); err != nil || len(cf.Received()) != n {
		t.Errorf("選んでいないプロジェクトを送った: %v", cf.Received())
	}
}

func TestPublishRejects(t *testing.T) {
	traytest.Local(t, nil)
	e, _ := env(t)
	for _, p := range []string{filepath.Join(e.Workspace, "none.html"), e.Workspace} {
		if _, err := Publish(e, p, "", ""); err == nil || !strings.Contains(err.Error(), "not a file") {
			t.Errorf("%s: err = %v", p, err)
		}
	}
	if _, err := Publish(e, writeTemp(t, "a.md", "x"), "a/b", ""); err == nil {
		t.Error("/ の slug を受けた")
	}
	big := filepath.Join(t.TempDir(), "big.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Publish(e, big, "", ""); err == nil || !strings.Contains(err.Error(), "大きすぎる") {
		t.Fatalf("上限超えを拒まなかった: %v", err)
	}
	if m := filesIn(t, e.Workspace); len(m) != 0 {
		t.Error("拒んだのに files/ を作った")
	}
}

func TestMainUsage(t *testing.T) {
	traytest.Local(t, nil)
	for _, args := range [][]string{nil, {"a", "b", "c"}, {"a", "--title"}, {"a", "--x"}} {
		var out, errb bytes.Buffer
		if rc := Main(args, nil, &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon publish") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errb.String())
		}
	}
	var out, errb bytes.Buffer
	if rc := Main([]string{filepath.Join(t.TempDir(), "none")}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "not a file") {
		t.Errorf("rc=%d err=%q", rc, errb.String())
	}
}

func TestProject(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "/w/projects/sample-site")
	if got := Project(); got != "sample-site" {
		t.Errorf("KANBAN_ROOT から: %q", got)
	}
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("PATH", t.TempDir()) // kanban の CLI が無い
	if got := Project(); got != tray.NoProject {
		t.Errorf("引けないとき: %q", got)
	}
}

// 受け入れ条件 7: project.json に tray を書いたプロジェクトの報告だけが、偽の Cloudflare の tray に届く。書いていないプロジェクト、
// プロジェクトを引けない報告、オーナーでない commit の tray の報告は届かない（どれもローカルの tray には届く）。
// RemoteFor は差し替えず、セルの repo の HEAD の project.json を本物の読み口で読む。
func TestPublishSendsToCloudflareOnlyWhenProjectChose(t *testing.T) {
	loc, cf := traytest.New(t), traytest.New(t)
	traytest.Local(t, loc)
	traytest.Route(t, "tray-up.example.com", cf)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(tray.KeyDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tray.KeyPath(), []byte("tray_key000000000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := traytest.CellRepo(t, traytest.Owner, map[string]string{
		"chosen": traytest.ProjectJSON("https://tray-up.example.com"),
		"plain":  traytest.ProjectJSON(""),
	})
	src := writeTemp(t, "a.md", "# a\n")
	for _, p := range []string{"chosen", "plain", tray.NoProject} {
		var w bytes.Buffer
		url, err := Publish(Env{Workspace: ws, Project: p, Now: now, Warn: &w}, src, "a", "")
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		want := loc.URL
		if p == "chosen" {
			want = cf.URL
		}
		if !strings.HasPrefix(url, want) {
			t.Errorf("%s: url = %q（%s のはず）warn=%q", p, url, want, w.String())
		}
	}
	got := cf.Received()
	if len(got) != 1 || got[0].Project != "chosen" || got[0].Auth != "Bearer tray_key000000000000" {
		t.Fatalf("Cloudflare の tray に届いたもの: %+v", got)
	}
	if n := len(loc.Received()); n != 3 {
		t.Errorf("ローカルの tray に届いた数 = %d（3 のはず）", n)
	}

	// オーナーでない commit の tray は読まず、ローカルの tray にだけ送って理由を出す
	untrusted := traytest.CellRepo(t, "someone@example.com", map[string]string{"chosen": traytest.ProjectJSON("https://tray-up.example.com")})
	var w bytes.Buffer
	url, err := Publish(Env{Workspace: untrusted, Project: "chosen", Now: now, Warn: &w}, src, "b", "")
	if err != nil || !strings.HasPrefix(url, loc.URL) || len(cf.Received()) != 1 || !strings.Contains(w.String(), "Cloudflare の tray の設定を読めない") {
		t.Errorf("オーナーでない commit: url=%q err=%v 届いた=%d warn=%q", url, err, len(cf.Received()), w.String())
	}
}
