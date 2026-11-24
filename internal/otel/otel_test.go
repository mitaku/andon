package otel

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 見本は、Claude Code v2.1.331 が OTEL_EXPORTER_OTLP_PROTOCOL=http/json で実際に送った /v1/traces と /v1/logs の本文を写し、
// 個人の値（user.email など）を架空の値に、andon の属性を sample-site・S-0001 に置き換えたもの。
func sample(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func encode(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func setResourceAttr(m map[string]any, listKey, key string, val any) {
	res := m[listKey].([]any)[0].(map[string]any)["resource"].(map[string]any)
	var out []any
	for _, a := range res["attributes"].([]any) {
		if a.(map[string]any)["key"] != key {
			out = append(out, a)
		}
	}
	if val != nil {
		out = append(out, map[string]any{"key": key, "value": map[string]any{"stringValue": val}})
	}
	res["attributes"] = out
}

type cell struct {
	r    *Receiver
	srv  *httptest.Server
	root string // 置き場の根
	top  string // 根を含む一時のフォルダ（根の外にファイルができないかを見る）
	logb *bytes.Buffer
}

func newCell(t *testing.T) *cell {
	t.Helper()
	top := t.TempDir()
	c := &cell{root: filepath.Join(top, "otel"), top: top, logb: &bytes.Buffer{}}
	c.r = &Receiver{Root: c.root, Log: c.logb, Now: func() time.Time { return time.Date(2026, 11, 24, 23, 59, 0, 0, time.UTC) }}
	c.srv = httptest.NewServer(c.r)
	t.Cleanup(c.srv.Close)
	return c
}

func (c *cell) post(t *testing.T, path, ctype string, body []byte) int {
	t.Helper()
	resp, err := http.Post(c.srv.URL+path, ctype, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func (c *cell) files(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(c.top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(c.top, p)
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func readLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("%s の行が JSON として読めない: %v", path, err)
		}
		out = append(out, m)
	}
	return out
}

func count(m map[string]any, listKey, scopeKey, itemKey string) int {
	n := 0
	for _, r := range m[listKey].([]any) {
		for _, s := range r.(map[string]any)[scopeKey].([]any) {
			n += len(s.(map[string]any)[itemKey].([]any))
		}
	}
	return n
}

func keys(v any, out map[string]int) {
	switch x := v.(type) {
	case map[string]any:
		if k, ok := x["key"].(string); ok {
			out[k]++
		}
		for _, c := range x {
			keys(c, out)
		}
	case []any:
		for _, c := range x {
			keys(c, out)
		}
	}
}

var personalKeys = []string{"user.email", "user.account_uuid", "user.account_id", "user.id", "organization.id"}

// 受け入れ条件 1・2: 見本を POST すると 200 で、その課題のファイルに 1 行が足され、span（log record）の数が見本と同じで、
// 個人の 5 つの属性が無い。見本にはその 5 つが在る。
func TestReceiveSample(t *testing.T) {
	for _, tc := range []struct {
		path, file, listKey, scopeKey, itemKey string
	}{
		{"/v1/traces", "traces.json", "resourceSpans", "scopeSpans", "spans"},
		{"/v1/logs", "logs.json", "resourceLogs", "scopeLogs", "logRecords"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			c := newCell(t)
			in := sample(t, tc.file)
			before := map[string]int{}
			keys(in, before)
			for _, k := range personalKeys {
				if before[k] == 0 {
					t.Fatalf("見本に %s が無い（落とすことを確かめられない）", k)
				}
			}
			want := count(in, tc.listKey, tc.scopeKey, tc.itemKey)
			if want == 0 {
				t.Fatal("見本の span・log record が 0")
			}
			if got := c.post(t, tc.path, "application/json", encode(t, in)); got != http.StatusOK {
				t.Fatalf("POST %s が %d（200 のはず）", tc.path, got)
			}
			name := strings.TrimSuffix(tc.path[len("/v1/"):], "/") + ".jsonl"
			lines := readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0001", name))
			if len(lines) != 1 {
				t.Fatalf("足された行が %d（1 のはず）", len(lines))
			}
			if got := count(lines[0], tc.listKey, tc.scopeKey, tc.itemKey); got != want {
				t.Errorf("行の数が %d（見本は %d）", got, want)
			}
			after := map[string]int{}
			keys(lines[0], after)
			for _, k := range personalKeys {
				if after[k] != 0 {
					t.Errorf("書いた行に %s が %d 個残っている", k, after[k])
				}
			}
			for _, k := range []string{"andon.project", "andon.issue", "andon.role", "session.id"} {
				if after[k] != before[k] {
					t.Errorf("落とさない属性 %s の数が変わった: %d → %d", k, before[k], after[k])
				}
			}
			if strings.Contains(string(encode(t, lines[0])), "example.com") {
				t.Error("書いた行にメールアドレスが残っている")
			}
			if c.r.Dropped() != 0 {
				t.Errorf("捨てた数が %d", c.r.Dropped())
			}
		})
	}
}

func TestReceiveSplitsResources(t *testing.T) {
	c := newCell(t)
	a := sample(t, "traces.json")
	b := sample(t, "traces.json")
	setResourceAttr(b, "resourceSpans", "andon.issue", nil)
	setResourceAttr(b, "resourceSpans", "andon.role", "chief-of-staff")
	in := map[string]any{"resourceSpans": append(a["resourceSpans"].([]any), b["resourceSpans"].([]any)...)}
	if got := c.post(t, "/v1/traces", "application/json; charset=utf-8", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	if n := len(readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0001", "traces.jsonl"))); n != 1 {
		t.Errorf("課題のファイルの行が %d", n)
	}
	if n := len(readLines(t, filepath.Join(c.root, "sample-site", "roles", "chief-of-staff", "2026-11-24.traces.jsonl"))); n != 1 {
		t.Errorf("役のファイルの行が %d", n)
	}
	if got := c.post(t, "/v1/traces", "application/json", encode(t, a)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	if n := len(readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0001", "traces.jsonl"))); n != 2 {
		t.Errorf("2 度目のあとの課題のファイルの行が %d（2 のはず）", n)
	}
}

// 受け入れ条件 3: 属性の値に GitHub のトークンの形を入れると、書いた行ではその値が伏せた印に替わる。
func TestReceiveRedactsSecrets(t *testing.T) {
	c := newCell(t)
	token := "gh" + "p_" + strings.Repeat("Ab1", 12) // そのまま書くと tests/public.sh に当たるので組み立てる
	in := sample(t, "traces.json")
	span := in["resourceSpans"].([]any)[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)["spans"].([]any)[0].(map[string]any)
	span["attributes"] = append(span["attributes"].([]any), map[string]any{"key": "full_command", "value": map[string]any{"stringValue": "gh auth login --with-token " + token}})
	if !strings.Contains(string(encode(t, in)), token) {
		t.Fatal("見本にトークンを入れられていない")
	}
	if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	b, err := os.ReadFile(filepath.Join(c.root, "sample-site", "issues", "S-0001", "traces.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(token)) {
		t.Error("書いた行にトークンが残っている")
	}
	if !bytes.Contains(b, []byte("gh auth login --with-token [REDACTED:github]")) {
		t.Errorf("トークンが伏せた印に替わっていない")
	}
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(b), &m); err != nil {
		t.Errorf("伏せたあとの行が JSON として読めない: %v", err)
	}
}

func TestReceiveRedactsInsideStrings(t *testing.T) {
	c := newCell(t)
	token := "gh" + "p_" + strings.Repeat("Ab1", 12)
	secret := "hunter2xyz"
	in := sample(t, "traces.json")
	span := in["resourceSpans"].([]any)[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)["spans"].([]any)[0].(map[string]any)
	params := string(encode(t, map[string]string{"bash_command": `export API_KEY="` + secret + `"`}))
	span["attributes"] = append(span["attributes"].([]any),
		map[string]any{"key": "full_command", "value": map[string]any{"stringValue": "gh auth login --with-token <<EOF\n" + token + "\nEOF"}},
		map[string]any{"key": "tool_parameters", "value": map[string]any{"stringValue": params}},
	)
	if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	b, err := os.ReadFile(filepath.Join(c.root, "sample-site", "issues", "S-0001", "traces.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(token)) {
		t.Error("改行の直後のトークンが残っている")
	}
	if bytes.Contains(b, []byte(secret)) {
		t.Error("JSON の文字列の中の引用符つきの値が残っている")
	}
}

func TestReceiveRedactsSecretNamedAttributes(t *testing.T) {
	c := newCell(t)
	in := sample(t, "traces.json")
	span := in["resourceSpans"].([]any)[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)["spans"].([]any)[0].(map[string]any)
	secrets := map[string]string{"password": "hunter2xyz", "http.request.header.x-api-key": "k3y", "access_token": "a1", "DB_PASSWORD": "pw9"}
	for k, v := range secrets {
		span["attributes"] = append(span["attributes"].([]any), map[string]any{"key": k, "value": map[string]any{"stringValue": v}})
	}
	span["attributes"] = append(span["attributes"].([]any),
		map[string]any{"key": "input_tokens", "value": map[string]any{"stringValue": "1200"}},
		map[string]any{"key": "token_note", "value": map[string]any{"intValue": "7"}})
	if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	b, err := os.ReadFile(filepath.Join(c.root, "sample-site", "issues", "S-0001", "traces.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range secrets {
		if bytes.Contains(b, []byte(`"`+v+`"`)) {
			t.Errorf("属性 %s の値 %q が残っている", k, v)
		}
	}
	if !bytes.Contains(b, []byte(`"key":"input_tokens","value":{"stringValue":"1200"}`)) {
		t.Error("数を運ぶ属性 input_tokens まで伏せた")
	}
}

// 受け入れ条件 4: andon.issue が ../x のものと andon.project の無いものは、200 が返り、何も書かれず、捨てた数が 1 ずつ増える。
// 置き場の外にファイルができない。
func TestReceiveDropsBadAttributes(t *testing.T) {
	c := newCell(t)
	steps := []struct {
		name string
		edit func(m map[string]any)
	}{
		{"andon.issue が ../x", func(m map[string]any) { setResourceAttr(m, "resourceSpans", "andon.issue", "../x") }},
		{"andon.project が無い", func(m map[string]any) { setResourceAttr(m, "resourceSpans", "andon.project", nil) }},
		{"andon.project が ..", func(m map[string]any) { setResourceAttr(m, "resourceSpans", "andon.project", "..") }},
		{"andon.role が無い", func(m map[string]any) { setResourceAttr(m, "resourceSpans", "andon.role", nil) }},
		{"andon.role が ../../x", func(m map[string]any) {
			setResourceAttr(m, "resourceSpans", "andon.issue", nil)
			setResourceAttr(m, "resourceSpans", "andon.role", "../../x")
		}},
	}
	for i, s := range steps {
		in := sample(t, "traces.json")
		s.edit(in)
		if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
			t.Errorf("%s: POST が %d（200 のはず）", s.name, got)
		}
		if got := c.r.Dropped(); got != i+1 {
			t.Errorf("%s: 捨てた数が %d（%d のはず）", s.name, got, i+1)
		}
	}
	if fs := c.files(t); len(fs) != 0 {
		t.Errorf("捨てるはずの記録でファイルができた: %q", fs)
	}
	if !strings.Contains(c.logb.String(), "書かずに捨てた（andon.issue の形が違う") || !strings.Contains(c.logb.String(), "捨てた数 2") {
		t.Errorf("捨てた理由と数をログに残さない:\n%s", c.logb.String())
	}
}

// 受け入れ条件 5: /v1/metrics への POST と、Content-Type が JSON でない POST は 4xx で、何も書かれない。
func TestReceiveRejects(t *testing.T) {
	c := newCell(t)
	body := encode(t, sample(t, "traces.json"))
	for _, tc := range []struct{ path, ctype string }{
		{"/v1/metrics", "application/json"},
		{"/v1/traces", "application/x-protobuf"},
		{"/v1/logs", "text/plain"},
		{"/v1/traces", ""},
	} {
		if got := c.post(t, tc.path, tc.ctype, body); got < 400 || got > 499 {
			t.Errorf("POST %s（%q）が %d（4xx のはず）", tc.path, tc.ctype, got)
		}
	}
	if got := c.post(t, "/v1/traces", "application/json", []byte("{not json")); got != http.StatusBadRequest {
		t.Errorf("JSON として読めない本文が %d（400 のはず）", got)
	}
	resp, err := http.Get(c.srv.URL + "/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /v1/traces が %d（405 のはず）", resp.StatusCode)
	}
	if fs := c.files(t); len(fs) != 0 {
		t.Errorf("断った POST でファイルができた: %q", fs)
	}
}

func TestReceiveGzip(t *testing.T) {
	c := newCell(t)
	var zb bytes.Buffer
	zw := gzip.NewWriter(&zb)
	if _, err := zw.Write(encode(t, sample(t, "logs.json"))); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	send := func(enc string) int {
		req, _ := http.NewRequest(http.MethodPost, c.srv.URL+"/v1/logs", bytes.NewReader(zb.Bytes()))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", enc)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := send("gzip"); got != http.StatusOK {
		t.Fatalf("gzip の POST が %d", got)
	}
	if got := send("br"); got != http.StatusUnsupportedMediaType {
		t.Errorf("知らない圧縮が %d（415 のはず）", got)
	}
	if n := len(readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0001", "logs.jsonl"))); n != 1 {
		t.Errorf("行が %d", n)
	}
}

// GET / は受け口の名前を返す（ポートを取っている相手が andon の受け口かを確かめる。決めたこと 4）。
func TestName(t *testing.T) {
	c := newCell(t)
	resp, err := http.Get(c.srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(b)) != Name {
		t.Errorf("GET / が %d %q", resp.StatusCode, b)
	}
}

func TestReceiveDoesNotFollowSymlink(t *testing.T) {
	c := newCell(t)
	outside := filepath.Join(c.top, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.root, "sample-site"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(c.root, "sample-site", "issues")); err != nil {
		t.Fatal(err)
	}
	if got := c.post(t, "/v1/traces", "application/json", encode(t, sample(t, "traces.json"))); got != http.StatusInternalServerError {
		t.Errorf("根の外を指す symlink の先へ書こうとして %d（500 のはず）", got)
	}
	if ents, _ := os.ReadDir(outside); len(ents) != 0 {
		t.Errorf("根の外に書いた: %v", ents)
	}
}

func TestServeMainPortTaken(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ln, err := net.Listen("tcp", Addr)
	if err == nil {
		defer ln.Close()
	} // 取れなければ、もう誰かが取っている（どちらでも serve は取れない）
	var errb bytes.Buffer
	if got := ServeMain(nil, nil, io.Discard, &errb); got != 1 {
		t.Errorf("ポートが取られているのに終了コード %d", got)
	}
	if !strings.Contains(errb.String(), "を取れない") {
		t.Errorf("取れない理由を出さない: %q", errb.String())
	}
	if got := ServeMain([]string{"x"}, nil, io.Discard, io.Discard); got != 2 {
		t.Errorf("余分な引数で終了コード %d（2 のはず）", got)
	}
}

func TestServe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "otel")
	done := make(chan error, 1)
	go func() { done <- serve(ln, &Receiver{Root: root, Log: io.Discard}) }()
	var resp *http.Response
	for i := 0; i < 50; i++ {
		if resp, err = http.Get("http://" + ln.Addr().String() + "/"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Errorf("置き場の根を作らない: %v", err)
	}
	ln.Close()
	if err := <-done; err == nil || !errors.Is(err, net.ErrClosed) {
		t.Errorf("listener を閉じたあとの serve の返り値: %v", err)
	}
}
