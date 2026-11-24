// Package otel は、ハーネス（Claude Code・Codex）が出す OpenTelemetry のトレースとログの受け口と、
// その生の記録の寿命の片付けを持つ（ADR-0060 決めたこと 2・3・8・9・10）。
//
// 受け口は OTLP/HTTP の JSON の POST（/v1/traces・/v1/logs）だけを受け、個人の属性を落とし、秘密の形を
// internal/redact で伏せてから、課題ごと（resource 属性 andon.issue）か役ごと（andon.role）のファイルに、
// resource 1 つ分の書き出しの要求を 1 行ずつ足す。kanban も project.json も読まない（受けて書くだけ）。
// 置き場と行の形の約束は docs/contract.md §30。
package otel

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"andon/internal/redact"
)

const (
	// Addr は受け口が listen する所（セルの外から届く口は作らない。決めたこと 3）。
	Addr = "127.0.0.1:4318"
	// Name は受け口が GET / に返す名前。ポートを取っている相手が andon の受け口かを確かめる（決めたこと 4）。
	Name = "andon-otel"
	// maxBody は 1 回の POST の本文の上限（展開したあと）。Claude Code の 1 回は数十 KB。
	maxBody = 16 << 20
)

// personal は書く前に落とす個人の属性（決めたこと 8。事実 11 の 5 つ）。
var personal = map[string]bool{
	"user.email":        true,
	"user.account_uuid": true,
	"user.account_id":   true,
	"user.id":           true,
	"organization.id":   true,
}

// contentAttrs は、イベントの名前ごとに、本文を運ぶので落とす属性（決めたこと 7・8）。Codex のログの
// codex.tool_result は、ツールの出力の写しを output に載せる（v0.166.0。既定で 2 KB まで）。
var contentAttrs = map[string]map[string]bool{
	"codex.tool_result": {"output": true},
}

// secretKey は値を伏せる属性の名前（名前を区切った語のどれかが秘密の名前）。値の形に秘密が現れない短い値
// （"password": "hunter2"）も、名前で伏せる。数を運ぶ input_tokens などは語が違うので当たらない。
var secretKey = regexp.MustCompile(`(?i)(?:^|[._-])(?:token|password|passwd|pwd|secret|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|credentials?)(?:$|[._-])`)

// 属性の値の形。送り手が書く値をパスに使うので、書く前に確かめる（決めたこと 9）。
// どれも先頭が英数字なので、"." と ".." と "/" を含む値は通らない。
var (
	projectRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,59}$`)
	issueRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	roleRe    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

type signal struct {
	key  string // 書き出しの要求の中の resource の並びのキー
	file string // 置き場のファイルの名前の部分
}

var (
	traces = signal{key: "resourceSpans", file: "traces"}
	logs   = signal{key: "resourceLogs", file: "logs"}
)

// Root は生の記録の置き場の根（作り直しても残る volume の ~/.local/state/andon/otel。決めたこと 9）。
// プロジェクトごとの置き場は Root()/<project>/。
func Root() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "otel")
}

type Receiver struct {
	Root string           // 置き場の根（Root()）
	Log  io.Writer        // ログ（1 行ずつ。頭に時刻）
	Now  func() time.Time // 役のファイルの日付とログの時刻（無ければ time.Now。検査用）

	mu      sync.Mutex // ファイルへの書き足しと捨てた数
	dropped int
}

// Dropped は受け口が起動してから捨てた記録（resource 1 つ分）の数。
func (r *Receiver) Dropped() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

func (r *Receiver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Receiver) logf(format string, a ...any) {
	if r.Log == nil {
		return
	}
	fmt.Fprintf(r.Log, r.now().UTC().Format(time.RFC3339)+" andon otel: "+format+"\n", a...)
}

// ServeHTTP は道ごとに振り分ける。受けるのは /v1/traces と /v1/logs の POST だけ（/v1/metrics は 404。決めたこと 3）。
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/":
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(w, "GET だけ", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, Name)
	case "/v1/traces":
		r.receive(w, req, traces)
	case "/v1/logs":
		r.receive(w, req, logs)
	default:
		http.Error(w, "受けない道（受けるのは /v1/traces と /v1/logs の JSON だけ）", http.StatusNotFound)
	}
}

// receive は書き出しの要求を 1 つ受け、resource ごとに 1 行ずつ書く。捨てた記録があっても 200 を返す
// （送り手に送り直させても同じ記録はまた捨てる）。書けなかったときだけ 500 を返す。
func (r *Receiver) receive(w http.ResponseWriter, req *http.Request, sig signal) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST だけ", http.StatusMethodNotAllowed)
		return
	}
	if mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		http.Error(w, "JSON だけ受ける（Content-Type: application/json。protobuf は受けない）", http.StatusUnsupportedMediaType)
		return
	}
	body, status, err := readBody(w, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "OTLP の JSON として読めない", http.StatusBadRequest)
		return
	}
	var resources []json.RawMessage
	if raw, ok := msg[sig.key]; ok {
		if err := json.Unmarshal(raw, &resources); err != nil {
			http.Error(w, sig.key+" が並びでない", http.StatusBadRequest)
			return
		}
	}
	var werr error
	for _, raw := range resources {
		if err := r.store(sig, raw); err != nil {
			werr = errors.Join(werr, err)
		}
	}
	if werr != nil {
		r.logf("書けない: %v", werr)
		http.Error(w, "置き場に書けない", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "{}")
}

// readBody は本文を読む（gzip なら展開する）。上限を超えたら 413、知らない圧縮は 415。
func readBody(w http.ResponseWriter, req *http.Request) ([]byte, int, error) {
	in := io.Reader(http.MaxBytesReader(w, req.Body, maxBody))
	switch req.Header.Get("Content-Encoding") {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(in)
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("gzip として読めない")
		}
		defer zr.Close() //nolint:errcheck // 読むだけの gzip。閉じる失敗で読んだ中身は変わらない
		in = zr
	default:
		return nil, http.StatusUnsupportedMediaType, errors.New("知らない圧縮（受けるのは gzip だけ）")
	}
	b, err := io.ReadAll(io.LimitReader(in, maxBody+1))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig) || len(b) > maxBody:
		return nil, http.StatusRequestEntityTooLarge, errors.New("本文が大きすぎる")
	case err != nil:
		return nil, http.StatusBadRequest, errors.New("本文が読めない")
	}
	return b, 0, nil
}

// store は resource 1 つ分を確かめ、個人の属性を落として伏せ、置き場の 1 行にして足す。
// 置き場を決められない記録は書かずに捨て、捨てた数を数えてログに残す（決めたこと 9）。
func (r *Receiver) store(sig signal, raw json.RawMessage) error {
	var entry any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // 数を書き換えない（OTLP の JSON の 64 ビットの整数を丸めない）
	if err := dec.Decode(&entry); err != nil {
		r.drop("resource が読めない")
		return nil
	}
	if sig == traces {
		if why := liftSpanAttrs(entry); why != "" {
			r.drop(why)
			return nil
		}
	}
	rel, reason := r.place(sig, entry)
	if reason != "" {
		r.drop(reason)
		return nil
	}
	strip(entry)
	entry = redactStrings(entry) // 送り手の文字列を 1 つずつ伏せる（JSON に書き直すと改行や引用符が \n・\\\" に化け、形が当たらない）
	var line bytes.Buffer
	enc := json.NewEncoder(&line)
	enc.SetEscapeHTML(false) // 伏せた印 <REDACTED> などを < に書き換えない
	if err := enc.Encode(map[string]any{sig.key: []any{entry}}); err != nil {
		return err
	}
	out, _ := redact.JSON(line.Bytes()) // 書き直した行にも当てる（文字列をまたぐ形の念のため。決めたこと 8。Encode の改行は残る）
	return r.append(rel, out)
}

// place は記録を書くファイル（置き場の根からの相対パス）を決める。決められなければ理由を返す。
// 課題の属性があれば issues/<id>/<種類>.jsonl、無ければ roles/<role>/<UTC の日付>.<種類>.jsonl。
func (r *Receiver) place(sig signal, entry any) (string, string) {
	attrs := resourceAttrs(entry)
	project, ok := attrs["andon.project"]
	if !ok {
		return "", "andon.project が無い"
	}
	if !projectRe.MatchString(project) {
		return "", fmt.Sprintf("andon.project の形が違う（%q）", short(project))
	}
	role, ok := attrs["andon.role"]
	if !ok {
		return "", "andon.role が無い"
	}
	if !roleRe.MatchString(role) {
		return "", fmt.Sprintf("andon.role の形が違う（%q）", short(role))
	}
	if issue, ok := attrs["andon.issue"]; ok {
		if !issueRe.MatchString(issue) {
			return "", fmt.Sprintf("andon.issue の形が違う（%q）", short(issue))
		}
		return filepath.Join(project, "issues", issue, sig.file+".jsonl"), ""
	}
	return filepath.Join(project, "roles", role, r.now().UTC().Format("2006-01-02")+"."+sig.file+".jsonl"), ""
}

func short(s string) string {
	if r := []rune(s); len(r) > 64 {
		return string(r[:64]) + "…"
	}
	return s
}

// andonKeys は振り分けに使う resource 属性（決めたこと 6）。書き足すときもこの順にする。
var andonKeys = []string{"andon.project", "andon.issue", "andon.role"}

// resourceAttrs は resource の属性のうち andon. で始まるものを、文字列の値で返す。文字列でない値は
// 形の違う値として「"" でない別の値」にして返す（呼び手の形の確かめで落ちる）。
func resourceAttrs(entry any) map[string]string {
	m, _ := entry.(map[string]any)
	res, _ := m["resource"].(map[string]any)
	list, _ := res["attributes"].([]any)
	return andonAttrs(list)
}

func andonAttrs(list []any) map[string]string {
	out := map[string]string{}
	for _, a := range list {
		kv, _ := a.(map[string]any)
		key, _ := kv["key"].(string)
		if !slices.Contains(andonKeys, key) {
			continue
		}
		val, _ := kv["value"].(map[string]any)
		if s, ok := val["stringValue"].(string); ok {
			out[key] = s
		} else {
			out[key] = "(文字列でない値)"
		}
	}
	return out
}

// liftSpanAttrs は、resource に andon. の属性が 1 つも無いトレースで、span の属性の andon. の値（Codex の
// span_attributes。ADR-0060 決めたこと 19）を resource の属性に写す。全部の span が同じ値を持つときだけ写し、
// 揃わなければ捨てる理由を返す。どの span にも無ければ何もしない（place が「andon.project が無い」で捨てる）。
func liftSpanAttrs(entry any) string {
	if len(resourceAttrs(entry)) != 0 {
		return ""
	}
	m, _ := entry.(map[string]any)
	if m == nil {
		return ""
	}
	var want map[string]string
	scopes, _ := m["scopeSpans"].([]any)
	for _, sc := range scopes {
		scm, _ := sc.(map[string]any)
		spans, _ := scm["spans"].([]any)
		for _, sp := range spans {
			spm, _ := sp.(map[string]any)
			list, _ := spm["attributes"].([]any)
			got := andonAttrs(list)
			if want == nil {
				want = got
			} else if !maps.Equal(want, got) {
				return "span ごとの andon. の属性が揃わない"
			}
		}
	}
	if len(want) == 0 {
		return ""
	}
	res, _ := m["resource"].(map[string]any)
	if res == nil {
		res = map[string]any{}
		m["resource"] = res
	}
	list, _ := res["attributes"].([]any)
	for _, k := range andonKeys {
		if v, ok := want[k]; ok {
			list = append(list, map[string]any{"key": k, "value": map[string]any{"stringValue": v}})
		}
	}
	res["attributes"] = list
	return ""
}

// strip は、属性の並び（どの深さの "attributes" も。kvlistValue の "values" も）から個人の属性を落とし、
// 名前が秘密の名前の属性の文字列の値を伏せる。
func strip(v any) {
	switch x := v.(type) {
	case map[string]any:
		content := contentOf(x)
		for k, child := range x {
			if list, ok := child.([]any); ok && (k == "attributes" || k == "values") {
				x[k] = dropPersonal(list, content)
			}
			strip(x[k])
		}
	case []any:
		for _, child := range x {
			strip(child)
		}
	}
}

// redactStrings は木の中の文字列の値を、書き直す前の生の文字列のまま internal/redact に通す。
func redactStrings(v any) any {
	switch x := v.(type) {
	case string:
		b, n := redact.JSON([]byte(x))
		if n == 0 {
			return x
		}
		return string(b)
	case map[string]any:
		for k, child := range x {
			x[k] = redactStrings(child)
		}
	case []any:
		for i, child := range x {
			x[i] = redactStrings(child)
		}
	}
	return v
}

// contentOf は記録（ログの 1 件か span のイベント）の名前から、落とす本文の属性を引く。名前は属性 event.name の文字列の値と
// OTLP の eventName の両方を見る。Codex のログの eventName は tracing の呼び出し元の名前（event <ファイル>:<行>）で、
// codex.tool_result の名前は属性 event.name にだけある（opentelemetry-appender-tracing 0.31）。
func contentOf(rec map[string]any) map[string]bool {
	if s, ok := rec["eventName"].(string); ok && contentAttrs[s] != nil {
		return contentAttrs[s]
	}
	list, _ := rec["attributes"].([]any)
	for _, a := range list {
		kv, _ := a.(map[string]any)
		if k, _ := kv["key"].(string); k == "event.name" {
			val, _ := kv["value"].(map[string]any)
			if s, _ := val["stringValue"].(string); contentAttrs[s] != nil {
				return contentAttrs[s]
			}
		}
	}
	return nil
}

func dropPersonal(list []any, content map[string]bool) []any {
	out := list[:0:0]
	for _, a := range list {
		if kv, ok := a.(map[string]any); ok {
			key, _ := kv["key"].(string)
			if personal[key] || content[key] {
				continue
			}
			if val, ok := kv["value"].(map[string]any); ok && secretKey.MatchString(key) {
				if _, ok := val["stringValue"].(string); ok {
					val["stringValue"] = "[REDACTED:attribute]"
				}
			}
		}
		out = append(out, a)
	}
	return out
}

func (r *Receiver) drop(reason string) {
	r.mu.Lock()
	r.dropped++
	n := r.dropped
	r.mu.Unlock()
	r.logf("書かずに捨てた（%s）。捨てた数 %d", reason, n)
}

// append は置き場の根の中のファイルに 1 行足す。開くのは os.Root を通すので、根の外を指す symlink は辿らない。
// 片付けが空のフォルダを消した直後に当たったら、作り直して 1 度だけ開き直す。
func (r *Receiver) append(rel string, line []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(r.Root, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // 根を開いただけ。書いたファイルは f.Close で確かめる
	var f *os.File
	for try := 0; ; try++ {
		if err = root.MkdirAll(filepath.Dir(rel), 0o700); err == nil {
			f, err = root.OpenFile(rel, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
		}
		if err == nil {
			break
		}
		if try > 0 || !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if _, err := f.Write(line); err != nil {
		f.Close() //nolint:errcheck // 書けなかった err を返す
		return err
	}
	return f.Close()
}
