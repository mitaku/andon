// Package traytest は、tray へ送る道を検査するための偽のアップロードの口と、偽のローカルの tray の控えを持つ。
package traytest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"andon/internal/tray"
)

// Got は偽の口が受けた1件。
type Got struct {
	Project, Name, Kind, Title, Type, Auth string
	Body                                   []byte
}

// Fake は偽のアップロードの口。名前ごとにバージョンを数え、同じ中身ではバージョンを上げない（tray の upload-api.ts と同じ）。
type Fake struct {
	URL  string
	mu   sync.Mutex
	got  []Got
	ver  map[string]int
	sum  map[string]string
	fail int
}

// New は偽の口を立てる。検査の終わりに閉じる。
func New(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{ver: map[string]int{}, sum: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, _ := io.ReadAll(r.Body)
	var seg []string
	for _, s := range splitPath(r.URL.EscapedPath()) {
		v, _ := url.PathUnescape(s)
		seg = append(seg, v)
	}
	if len(seg) != 4 || seg[0] != "v1" || seg[1] != "reports" || r.Method != http.MethodPut {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not_found","message":"無い"}`)
		return
	}
	title, _ := url.PathUnescape(r.Header.Get("X-Tray-Title"))
	f.got = append(f.got, Got{Project: seg[2], Name: seg[3], Kind: r.Header.Get("X-Tray-Kind"), Title: title,
		Type: r.Header.Get("Content-Type"), Auth: r.Header.Get("Authorization"), Body: b})
	if f.fail != 0 {
		w.WriteHeader(f.fail)
		fmt.Fprint(w, `{"error":"down","message":"落ちている"}`)
		return
	}
	sum := sha256.Sum256(b)
	key := seg[2] + "/" + seg[3]
	bumped := f.sum[key] != hex.EncodeToString(sum[:])
	if bumped {
		f.ver[key]++
		f.sum[key] = hex.EncodeToString(sum[:])
	}
	fmt.Fprintf(w, `{"url":"%s/r/%s","version":%d,"bumped":%v}`, f.URL, url.PathEscape(key), f.ver[key], bumped)
}

func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' })
}

// Received は受けた要求の写し。
func (f *Fake) Received() []Got {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got)
}

// Fail は status（0 なら直す）で断るようにする。
func (f *Fake) Fail(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = status
}

// Local は一時の状態の置き場（XDG_STATE_HOME）に、f を宛先とするローカルの tray の控えを書き、送るのを開ける
// （Go の検査の中では既定で切ってある）。f が nil なら控えを書かない（ローカルの tray が止まっている）。
func Local(t testing.TB, f *Fake) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(tray.OffEnv, "0")
	if f == nil {
		return
	}
	if err := tray.WriteControl(&tray.Control{List: f.URL, Upload: f.URL, ListOrigin: "http://tray.sample.localhost:1355"}); err != nil {
		t.Fatal(err)
	}
}

// Last は最後に受けた本文（無ければ検査を落とす）。
func (f *Fake) Last(t testing.TB) Got {
	t.Helper()
	got := f.Received()
	if len(got) == 0 {
		t.Fatal("tray に何も届いていない")
	}
	return got[len(got)-1]
}

// Remote は検査の間だけ、project が Cloudflare の tray（f）を選んだことにする。
func Remote(t testing.TB, f *Fake, project, key string) {
	t.Helper()
	prev := tray.RemoteFor
	tray.RemoteFor = func(_, p string) (*tray.Remote, error) {
		if p != project {
			return nil, nil
		}
		return &tray.Remote{Upload: f.URL, Key: key}, nil
	}
	t.Cleanup(func() { tray.RemoteFor = prev })
}

// Route は検査の間だけ、https://<host> への送りを f へ回す（project.json の tray.upload は https のホスト名だけを受けるので、
// 本物の RemoteFor を通して偽の Cloudflare の tray へ届けるときに使う）。127.0.0.1 の http（偽のローカルの tray）はそのまま通し、
// ほかの宛先へは出さずに断る。
func Route(t testing.TB, host string, f *Fake) {
	t.Helper()
	target, err := url.Parse(f.URL)
	if err != nil {
		t.Fatal(err)
	}
	prev := tray.Client
	tray.Client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "http" && r.URL.Hostname() == "127.0.0.1" {
			return http.DefaultTransport.RoundTrip(r)
		}
		if r.URL.Scheme != "https" || r.URL.Host != host {
			return nil, fmt.Errorf("検査の外の宛先 %s へは送らない", r.URL.Redacted())
		}
		r2 := r.Clone(r.Context())
		r2.URL.Scheme, r2.URL.Host, r2.Host = target.Scheme, target.Host, target.Host
		return http.DefaultTransport.RoundTrip(r2)
	})}
	t.Cleanup(func() { tray.Client = prev })
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Owner は CellRepo が andon.toml に書くオーナー。
const Owner = "owner@example.com"

// CellRepo は一時のセルの repo を作り、andon.toml（オーナーは Owner）と projects/<slug>/project.json（projects の値）を
// author の名前で1つの commit にする。返すのはセルの repo の置き場（ANDON_WORKSPACE に入れて使う）。git が無ければ検査を飛ばす。
func CellRepo(t testing.TB, author string, projects map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	ws := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(ws, "andon.toml"), "owner = \""+Owner+"\"\n")
	for slug, body := range projects {
		write(filepath.Join(ws, "projects", slug, "project.json"), body)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "projects"}} {
		cmd := exec.Command("git", append([]string{"-C", ws, "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL="+author, "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL="+author,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return ws
}

// ProjectJSON は project.json の中身。tray が空でなければ "tray": {"upload": tray} を足す。
func ProjectJSON(tray string) string {
	extra := ""
	if tray != "" {
		extra = `, "tray": {"upload": "` + tray + `"}`
	}
	return `{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": []` + extra + `}`
}
