package tray

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// sample はアップロードの約束の見本（tray/contract/upload-v1.json）。Workers の検査も同じファイルを読む（受け入れ条件 14）。
type sample struct {
	Request struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
		Body    *string           `json:"body"`
	} `json:"request"`
	Response struct {
		Status int            `json:"status"`
		Body   map[string]any `json:"body"`
	} `json:"response"`
}

func readSample(t *testing.T) (sample, map[string]any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "tray", "contract", "upload-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s sample
	var raw map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return s, raw
}

// contractProblems は、送る口が見本の要求と同じものを作り、見本の答えを読めるかを見る。食い違いを返す。
func contractProblems(raw map[string]any) []string {
	b, _ := json.Marshal(raw)
	var s sample
	if err := json.Unmarshal(b, &s); err != nil {
		return []string{"見本が読めない: " + err.Error()}
	}
	var out []string
	bad := func(f string, a ...any) { out = append(out, fmt.Sprintf(f, a...)) }
	if s.Request.Body == nil {
		return []string{"見本の request に body が無い"}
	}
	r := Report{
		Project: "andon", Name: "2026-11-27-週の報告", Title: "週の報告", Kind: KindPublish,
		ContentType: "text/markdown; charset=utf-8", Body: []byte(*s.Request.Body),
	}
	req, err := NewRequest("https://tray-up.example.com", "tray_samplekey0000000", r)
	if err != nil {
		return []string{"要求を作れない: " + err.Error()}
	}
	if req.Method != s.Request.Method {
		bad("method が %q（見本は %q）", req.Method, s.Request.Method)
	}
	if got := req.URL.EscapedPath(); got != s.Request.Path {
		bad("path が %q（見本は %q）", got, s.Request.Path)
	}
	var sent []string
	for k := range req.Header {
		sent = append(sent, http.CanonicalHeaderKey(k))
	}
	var want []string
	for k := range s.Request.Headers {
		want = append(want, http.CanonicalHeaderKey(k))
	}
	slices.Sort(sent)
	slices.Sort(want)
	if !slices.Equal(sent, want) {
		bad("送るヘッダが %v（見本は %v）", sent, want)
	}
	for k, v := range s.Request.Headers {
		got := req.Header.Get(k)
		if k == "Authorization" {
			if !strings.HasPrefix(got, "Bearer ") || !strings.HasPrefix(v, "Bearer ") {
				bad("Authorization が %q（見本は %q）", got, v)
			}
			continue
		}
		if got != v {
			bad("ヘッダ %s が %q（見本は %q）", k, got, v)
		}
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != *s.Request.Body {
		bad("本文が見本と違う")
	}
	// 答え: 見本の欄は、送る口が読む欄と同じで、読めること
	fields := []string{"bumped", "url", "version"}
	if got := slices.Sorted(maps.Keys(s.Response.Body)); !slices.Equal(got, fields) {
		bad("答えの欄が %v（送る口が読むのは %v）", got, fields)
	}
	rb, _ := json.Marshal(s.Response.Body)
	a, err := ParseAnswer(s.Response.Status, rb)
	if err != nil {
		bad("見本の答えを読めない: %v", err)
	} else if a.URL != s.Response.Body["url"] || float64(a.Version) != s.Response.Body["version"] || a.Bumped != s.Response.Body["bumped"] {
		bad("見本の答えの読みが違う: %+v", a)
	}
	return out
}

func TestContractSample(t *testing.T) {
	_, raw := readSample(t)
	if p := contractProblems(raw); len(p) > 0 {
		t.Fatalf("送る口が約束の見本と食い違う:\n%s", strings.Join(p, "\n"))
	}
	// 検査が当たる場合があること: 見本の欄を1つ消すと落ちる（Workers の検査と同じく）
	cut := func(path ...string) map[string]any {
		b, _ := json.Marshal(raw)
		var c map[string]any
		_ = json.Unmarshal(b, &c)
		m := c
		for _, p := range path[:len(path)-1] {
			m = m[p].(map[string]any)
		}
		delete(m, path[len(path)-1])
		return c
	}
	cases := [][]string{
		{"request", "method"}, {"request", "path"}, {"request", "body"},
		{"response", "status"}, {"response", "body", "url"}, {"response", "body", "version"}, {"response", "body", "bumped"},
	}
	s, _ := readSample(t)
	for h := range s.Request.Headers {
		cases = append(cases, []string{"request", "headers", h})
	}
	for _, c := range cases {
		if p := contractProblems(cut(c...)); len(p) == 0 {
			t.Errorf("見本の %s を消しても通った", strings.Join(c, "."))
		}
	}
}

// fakeTray は偽のアップロードの口。受けた要求を残し、名前ごとにバージョンを数える。
type fakeTray struct {
	mu   sync.Mutex
	got  []got
	ver  map[string]int
	sum  map[string]string
	fail int // 0 でなければこの状態で断る
	srv  *httptest.Server
}

type got struct {
	Path, Kind, Title, Type, Auth string
	Body                          []byte
}

func newFake(t *testing.T) *fakeTray {
	t.Helper()
	f := &fakeTray{ver: map[string]int{}, sum: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		f.got = append(f.got, got{Path: r.URL.EscapedPath(), Kind: r.Header.Get("X-Tray-Kind"), Title: r.Header.Get("X-Tray-Title"),
			Type: r.Header.Get("Content-Type"), Auth: r.Header.Get("Authorization"), Body: b})
		if f.fail != 0 {
			w.WriteHeader(f.fail)
			fmt.Fprint(w, `{"error":"down","message":"落ちている"}`)
			return
		}
		bumped := f.sum[r.URL.Path] != r.Header.Get("X-Tray-Sha256")
		if bumped {
			f.ver[r.URL.Path]++
			f.sum[r.URL.Path] = r.Header.Get("X-Tray-Sha256")
		}
		fmt.Fprintf(w, `{"url":"%s/r/%x","version":%d,"bumped":%v,"later":"足した欄"}`, f.srv.URL, len(r.URL.Path), f.ver[r.URL.Path], bumped)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTray) received() []got {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got)
}

// Local は偽のローカルの tray の控えを一時の HOME に書く。
func local(t *testing.T, f *fakeTray) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv(OffEnv, "0")
	if f == nil {
		return
	}
	if err := WriteControl(&Control{List: f.srv.URL, Upload: f.srv.URL, ListOrigin: "http://tray.sample.localhost:1355"}); err != nil {
		t.Fatal(err)
	}
}

func TestSendLocalAndRemote(t *testing.T) {
	loc, cf := newFake(t), newFake(t)
	local(t, loc)
	r := Report{Project: "sample-site", Name: "2026-01-02-週の報告", Title: "週の報告", Kind: KindPublish, ContentType: "text/markdown; charset=utf-8", Body: []byte("# 週\n")}
	d := Send(r, nil)
	if d.LocalErr != nil || d.Local == nil || d.Local.Version != 1 || !d.Local.Bumped || d.Remote != nil || d.RemoteErr != nil {
		t.Fatalf("ローカルだけ: %+v", d)
	}
	if d.URL() != d.Local.URL {
		t.Errorf("URL = %q", d.URL())
	}
	g := loc.received()[0]
	if g.Path != "/v1/reports/sample-site/2026-01-02-%E9%80%B1%E3%81%AE%E5%A0%B1%E5%91%8A" || g.Auth != "" || g.Kind != "publish" {
		t.Errorf("ローカルへの要求: %+v", g)
	}
	// 同じ中身はバージョンを上げない
	if d := Send(r, nil); d.Local == nil || d.Local.Bumped || d.Local.Version != 1 {
		t.Errorf("同じ中身: %+v", d)
	}
	d = Send(r, &Remote{Upload: cf.srv.URL, Key: "tray_key000000000000"})
	if d.Remote == nil || d.URL() != d.Remote.URL || cf.received()[0].Auth != "Bearer tray_key000000000000" {
		t.Errorf("Cloudflare: %+v %+v", d, cf.received())
	}
	// Cloudflare が落ちていれば、ローカルの URL に倒れる
	cf.fail = 503
	d = Send(r, &Remote{Upload: cf.srv.URL, Key: "tray_key000000000000"})
	if d.Remote != nil || d.RemoteErr == nil || d.URL() != d.Local.URL || !strings.Contains(strings.Join(d.Warnings(), "\n"), "落ちている") {
		t.Errorf("Cloudflare が落ちた: %+v %v", d, d.Warnings())
	}
	// 鍵が無ければ送らない
	if d := Send(r, &Remote{Upload: cf.srv.URL}); d.RemoteErr == nil || !strings.Contains(d.RemoteErr.Error(), "andon tray auth") {
		t.Errorf("鍵なし: %+v", d)
	}
}

func TestSendWithoutLocal(t *testing.T) {
	local(t, nil)
	d := Send(Report{Name: "a", Title: "a", Kind: KindReport, ContentType: "text/plain", Body: []byte("x")}, nil)
	if d.Local != nil || d.LocalErr == nil || !strings.Contains(d.LocalErr.Error(), "動いていない") || d.URL() != "" {
		t.Errorf("控えが無い: %+v", d)
	}
}

func TestSendOff(t *testing.T) {
	loc, cf := newFake(t), newFake(t)
	local(t, loc)
	t.Setenv(OffEnv, "1")
	d := Send(Report{Name: "a", Title: "a", Kind: KindReport, ContentType: "text/plain", Body: []byte("x")}, &Remote{Upload: cf.srv.URL, Key: "k"})
	if d.LocalErr != ErrOff || d.RemoteErr != ErrOff || len(loc.received())+len(cf.received()) != 0 {
		t.Errorf("切ってあるのに送った: %+v", d)
	}
}

func TestNewRequestRejectsNames(t *testing.T) {
	for _, n := range []string{"", "a/b", "a\x01", strings.Repeat("あ", 67), string([]byte{0xff})} {
		if _, err := NewRequest("http://127.0.0.1:1", "", Report{Name: n, Kind: KindPublish, ContentType: "text/plain"}); err == nil {
			t.Errorf("名前 %q を受けた", n)
		}
	}
	ok := strings.Repeat("あ", 66) + "ab" // 200 バイトちょうど
	if _, err := NewRequest("http://127.0.0.1:1", "", Report{Name: ok, Kind: KindPublish, ContentType: "text/plain"}); err != nil {
		t.Errorf("200 バイトの名前: %v", err)
	}
	if _, err := NewRequest("ftp://x", "", Report{Name: "a", Kind: KindPublish, ContentType: "text/plain"}); err == nil {
		t.Error("ftp の宛先を受けた")
	}
}

// fakeSecret は秘密の形の文字列（そのまま書くと tests/public.sh に当たるので組み立てる）。
var fakeSecret = "gh" + "p_" + strings.Repeat("A1b2", 9)

// 受け入れ条件 8: Cloudflare へ送る HTML は文の部分だけ伏せ、スタイルとスクリプトは元のまま。ローカルの写しは変わらない。
func TestSendRedactsRemoteHTMLOnly(t *testing.T) {
	loc, cf := newFake(t), newFake(t)
	local(t, loc)
	style := `<style>.secret:hover{color:red}</style>`
	script := `<script>const token = new Date(); const x = "` + fakeSecret + `";</script>`
	page := `<!doctype html><html><head><title>t</title>` + style + `</head><body><p>鍵は ` + fakeSecret + ` です</p>` +
		`<a title="` + fakeSecret + `" href="#a">a</a>` + script + `</body></html>`
	r := Report{Name: "p", Title: "t", Kind: KindPublish, ContentType: "text/html; charset=utf-8", Body: []byte(page)}
	d := Send(r, &Remote{Upload: cf.srv.URL, Key: "tray_key000000000000"})
	if d.LocalErr != nil || d.RemoteErr != nil {
		t.Fatal(d.Warnings())
	}
	if string(loc.received()[0].Body) != page {
		t.Errorf("ローカルの写しが変わった: %s", loc.received()[0].Body)
	}
	remote := string(cf.received()[0].Body)
	if strings.Contains(strings.SplitN(remote, "<script>", 2)[0], fakeSecret) {
		t.Errorf("文の部分か属性の値に秘密が残った: %s", remote)
	}
	if !strings.Contains(remote, "鍵は [REDACTED:github] です") || !strings.Contains(remote, `title="[REDACTED:github]"`) {
		t.Errorf("文の部分と属性の値が伏せていない: %s", remote)
	}
	if !strings.Contains(remote, style) || !strings.Contains(remote, script) {
		t.Errorf("スタイルかスクリプトが変わった: %s", remote)
	}
	// Markdown は全体に当てる
	md := Report{Name: "m", Title: "m", Kind: KindReport, ContentType: "text/markdown; charset=utf-8", Body: []byte("token: " + fakeSecret + "\n")}
	Send(md, &Remote{Upload: cf.srv.URL, Key: "tray_key000000000000"})
	if b := string(cf.received()[1].Body); strings.Contains(b, fakeSecret) {
		t.Errorf("Markdown に秘密が残った: %s", b)
	}
}

func TestRedactHTMLKeepsBytesWhenNothingMatches(t *testing.T) {
	page := "<!DOCTYPE html>\n<html><HEAD><title>a &amp; b</title></HEAD><body class=x data-y='1'>\n<p>本文 &lt;x&gt;</p></body></html>\n"
	if got := string(RedactHTML([]byte(page))); got != page {
		t.Errorf("当たらないのに書き換えた:\n%s", got)
	}
}
