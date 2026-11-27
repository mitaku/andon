// Package tray は、セルの側から tray（セルが出した報告を集めて読む場所。ADR-0064）へ報告を送る口を持つ。
//
// 送る先は2つある。ローカルの tray（セルの中。andon tray serve が状態の置き場に書いた控えから宛先を引く）と、
// プロジェクトが選んだときだけの Cloudflare の tray（決めたこと 5）である。約束は tray/contract/upload-v1.json（決めたこと 15）。
// 送れなくても呼ぶ側は止めない（決めたこと 8・12）。理由は Delivery に残し、呼ぶ側が stderr に出す。
package tray

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"andon/internal/delegate"
	"andon/internal/redact"
)

// OffEnv は送るのを切る環境変数（決めたこと 15）。0 でない値があればどこへも送らない。tests/lib/isolate.sh が切る
// （切らないと、検査が動いている本物のローカルの tray に偽の報告を上げる）。Go の検査の中では、値が無くても切る
// （どのパッケージの検査も、報告を送る葉を通るとき切り忘れない）。偽の口へ送る検査は traytest.Local で 0 にして開ける。
const OffEnv = "ANDON_TRAY_OFF"

// 報告の種類（X-Tray-Kind）。
const (
	KindPublish = "publish"
	KindDigest  = "digest"
	KindReport  = "report"
)

// NoProject は、プロジェクトが引けない報告のプロジェクトの名前（決めたこと 8・15）。
const NoProject = "-"

// MaxBytes は送れる本文の上限（tray の upload-api.ts の MAX_BYTES と同じ）。
const MaxBytes = 20 << 20

// maxName は名前・プロジェクトの UTF-8 のバイト数の上限（tray の names.ts と同じ）。
const maxName = 200

// Report は送る1件。
type Report struct {
	Project     string // プロジェクトの slug。無ければ NoProject
	Name        string // 同じ報告を見分ける名前（決めたこと 9）
	Title       string
	Kind        string
	ContentType string
	Body        []byte
}

// Answer はアップロードの口の答え（約束の response.body）。欄は足すだけで変わるので、知らない欄は読み捨てる。
type Answer struct {
	URL     string `json:"url"`
	Version int    `json:"version"`
	Bumped  bool   `json:"bumped"`
}

// Remote は Cloudflare の tray のアップロードの口と、セルの API キー。
type Remote struct {
	Upload string // https://<アップロードの口のホスト名>
	Key    string
}

// RemoteFor は project が Cloudflare の tray へ送ると選んでいれば、その宛先を返す。選んでいなければ nil。
// 選んだかは project.json の tray のキーで決まる（決めたこと 5・14）。読むのは、ダイジェストの送り先と同じく
// delegate.ReadProject を通した値（セルの repo の HEAD で、最後の commit の author がオーナーのもの）だけ。
// 信じられない・形が違う・読めないときは誤りを返す（呼ぶ側はローカルの tray にだけ送る。安全側）。検査は差し替えてよい。
var RemoteFor = remoteFor

func remoteFor(workspace, project string) (*Remote, error) {
	if project == "" || project == NoProject {
		return nil, nil
	}
	p, err := delegate.ReadProject(workspace, project)
	if errors.Is(err, delegate.ErrAbsent) {
		// git の失敗も ErrAbsent に入るので、無いと言い切れるときだけ「選んでいない」にする
		if err := delegate.ConfirmProjectAbsent(workspace, project); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if p.Tray == nil {
		return nil, nil
	}
	return &Remote{Upload: strings.TrimRight(p.Tray.Upload, "/"), Key: ReadKey()}, nil
}

// Delivery は1件を送った結果。Remote と RemoteErr がどちらも nil なら、プロジェクトは Cloudflare を選んでいない。
type Delivery struct {
	Local     *Answer
	LocalErr  error
	Remote    *Answer
	RemoteErr error
}

// URL はオーナーに渡す URL を1つ返す（決めたこと 8）。Cloudflare に届けばその URL、届かなければローカルの URL、どちらも無ければ空。
func (d Delivery) URL() string {
	if d.Remote != nil {
		return d.Remote.URL
	}
	if d.Local != nil {
		return d.Local.URL
	}
	return ""
}

// Warnings は送れなかった理由の行（呼ぶ側が stderr に出す）。
func (d Delivery) Warnings() []string {
	var out []string
	if d.LocalErr != nil {
		out = append(out, "ローカルの tray に送れない: "+oneLine(d.LocalErr.Error()))
	}
	if d.RemoteErr != nil {
		out = append(out, "Cloudflare の tray に送れない: "+oneLine(d.RemoteErr.Error()))
	}
	return out
}

// ErrOff は OffEnv で切ってあるときの理由。
var ErrOff = errors.New(OffEnv + " で切ってある")

// Off は送るのを切ってあるか。
func Off() bool {
	v := os.Getenv(OffEnv)
	if v == "" {
		return testing.Testing()
	}
	return v != "0"
}

// Send は r をローカルの tray に送り、remote があれば Cloudflare の tray にも送る（決めたこと 8 の順）。
// Cloudflare へ送る写しは秘密の形を伏せる（決めたこと 10）。ローカルの写しは伏せない。
func Send(r Report, remote *Remote) Delivery {
	var d Delivery
	if Off() {
		d.LocalErr = ErrOff
		if remote != nil {
			d.RemoteErr = ErrOff
		}
		return d
	}
	if c, err := ReadControl(); err != nil {
		d.LocalErr = err
	} else {
		d.Local, d.LocalErr = Put(c.Upload, "", r)
	}
	if remote != nil {
		if remote.Key == "" {
			d.RemoteErr = fmt.Errorf("API キーが無い（andon tray auth で %s に置く）", KeyPath())
		} else {
			d.Remote, d.RemoteErr = Put(remote.Upload, remote.Key, ForRemote(r))
		}
	}
	return d
}

// Client はアップロードの口へ打つ HTTP の口（検査が差し替える）。
var Client = &http.Client{Timeout: 60 * time.Second}

// Put は base（アップロードの口のオリジン）へ r を PUT する（約束 tray/contract/upload-v1.json）。key が空なら鍵を付けない（ローカルの tray）。
func Put(base, key string, r Report) (*Answer, error) {
	req, err := NewRequest(base, key, r)
	if err != nil {
		return nil, err
	}
	res, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close() //nolint:errcheck // 読み終えた答えの本文で、閉じる失敗は読んだ中身に効かない
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return ParseAnswer(res.StatusCode, b)
}

// NewRequest は r を約束の要求にする（Go の検査が見本と突き合わせる）。
func NewRequest(base, key string, r Report) (*http.Request, error) {
	if r.Project == "" {
		r.Project = NoProject
	}
	if err := checkName("プロジェクト", r.Project); err != nil {
		return nil, err
	}
	if err := checkName("名前", r.Name); err != nil {
		return nil, err
	}
	switch r.Kind {
	case KindPublish, KindDigest, KindReport:
	default:
		return nil, fmt.Errorf("種類 %q は publish・digest・report のどれでもない", r.Kind)
	}
	if r.ContentType == "" {
		return nil, errors.New("Content-Type が無い")
	}
	if len(r.Body) > MaxBytes {
		return nil, fmt.Errorf("大きすぎる（%d バイト、上限 %d）", len(r.Body), MaxBytes)
	}
	base = strings.TrimRight(base, "/")
	if u, err := url.Parse(base); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("アップロードの口 %q が http か https の URL でない", base)
	}
	// 名前とプロジェクトは段ごとにパーセントで符号化する（UTF-8 のまま持つ。決めたこと 9）
	target := base + "/v1/reports/" + url.PathEscape(r.Project) + "/" + url.PathEscape(r.Name)
	req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(r.Body))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(r.Body)
	req.Header.Set("Content-Type", r.ContentType)
	req.Header.Set("X-Tray-Kind", r.Kind)
	req.Header.Set("X-Tray-Title", url.PathEscape(cleanTitle(r.Title, r.Name)))
	req.Header.Set("X-Tray-Sha256", hex.EncodeToString(sum[:]))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return req, nil
}

// ParseAnswer は答えを読む。200 でなければ答えの error の欄（無ければ本文の1行）を理由にする。
func ParseAnswer(status int, b []byte) (*Answer, error) {
	if status != http.StatusOK {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		why := strings.TrimSpace(string(b))
		if json.Unmarshal(b, &e) == nil && (e.Error != "" || e.Message != "") {
			why = strings.TrimSpace(e.Error + " " + e.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", status, oneLine(why))
	}
	var a Answer
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("答えが読めない: %v", err)
	}
	if a.URL == "" || a.Version < 1 {
		return nil, fmt.Errorf("答えに url か version が無い: %s", oneLine(string(b)))
	}
	return &a, nil
}

// checkName は tray が受ける名前か（names.ts の decodeName と同じ線）を見る。
func checkName(what, s string) error {
	switch {
	case s == "":
		return fmt.Errorf("%sが空", what)
	case strings.Contains(s, "/"):
		return fmt.Errorf("%s %q に / は使えない", what, s)
	case !utf8.ValidString(s):
		return fmt.Errorf("%s %q が UTF-8 でない", what, s)
	case len(s) > maxName:
		return fmt.Errorf("%s %q が %d バイトを超える", what, s, maxName)
	}
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return fmt.Errorf("%s %q に制御文字がある", what, s)
		}
	}
	return nil
}

// cleanTitle は制御文字を空白に替え、空なら名前にする。
func cleanTitle(title, name string) string {
	t := strings.Map(func(r rune) rune {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return ' '
		}
		return r
	}, title)
	if t = strings.TrimSpace(t); t == "" {
		return name
	}
	return t
}

// ForRemote は Cloudflare の tray へ送る r の写しを返す。本文と、一覧のメタデータに残るタイトルの秘密の形を伏せる（決めたこと 10）。
func ForRemote(r Report) Report {
	out := r
	out.Body = Redact(r.ContentType, r.Body)
	if t, n := redact.JSON([]byte(r.Title)); n > 0 {
		out.Title = string(t)
	}
	return out
}

// Redact は Cloudflare へ送る写しの秘密の形を伏せる（決めたこと 10）。HTML は文の部分と属性の値だけ、
// Markdown とテキストは全体に当てる（redact.JSON の形はどれも " と \ をまたがないので、JSON でない文にも当てられる）。ほかの形式（画像・PDF など）はそのまま。
func Redact(contentType string, body []byte) []byte {
	switch mediaType(contentType) {
	case "text/html":
		return RedactHTML(body)
	case "text/markdown", "text/plain":
		out, _ := redact.JSON(body)
		return out
	}
	return body
}

func mediaType(ct string) string {
	t, _, _ := strings.Cut(ct, ";")
	return strings.ToLower(strings.TrimSpace(t))
}

func oneLine(s string) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	if s == "" {
		return "理由が分からない"
	}
	return s
}

// StateDir はローカルの tray の状態の置き場（~/.local/state/andon/tray。volume state の下。決めたこと 26）。
func StateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "tray")
}

// ControlPath は andon tray serve が書く控え（宛先）。
func ControlPath() string { return filepath.Join(StateDir(), "serve.json") }

// Control は andon tray serve が動き始めたときに書く控え。serve.mjs が出す1行の JSON と、serve の pid。
type Control struct {
	List       string `json:"list"`        // 一覧の面と報告の面が listen する口（127.0.0.1）
	Upload     string `json:"upload"`      // アップロードの口（127.0.0.1）
	ListOrigin string `json:"list_origin"` // オーナーが開く一覧の面のオリジン（portless の名前）
	ViewOrigin string `json:"view_origin"`
	PID        int    `json:"pid"`
}

// ReadControl は控えを読む。無ければ「動いていない」を理由にする。
func ReadControl() (*Control, error) {
	b, err := os.ReadFile(ControlPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ローカルの tray が動いていない（%s が無い。andon boot が起動する）", ControlPath())
	}
	if err != nil {
		return nil, err
	}
	var c Control
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", ControlPath(), err)
	}
	if c.Upload == "" || c.List == "" {
		return nil, fmt.Errorf("%s に宛先が無い", ControlPath())
	}
	return &c, nil
}

// WriteControl は控えを一時ファイル＋rename で書く。
func WriteControl(c *Control) error {
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := ControlPath() + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ControlPath())
}

// KeyDir は andon tray auth が Cloudflare の tray の API キーを置く dir（compose の volume tray。決めたこと 19）。
func KeyDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "andon", "tray")
}

// KeyName は鍵のファイルの名前（signal.PlaceToken が <name>.token にする）。
const KeyName = "cloudflare"

func KeyPath() string { return filepath.Join(KeyDir(), KeyName+".token") }

// ReadKey は置いた API キーを読む。無ければ空。
func ReadKey() string {
	b, err := os.ReadFile(KeyPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
