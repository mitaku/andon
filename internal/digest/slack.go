package digest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// slackAPI は Slack の Web API の根。検査は Env.SlackAPI を偽のサーバに向ける（環境変数では変えられない。送る先は設定の送り先だけ）。
const slackAPI = "https://slack.com/api/"

const slackTimeout = 60 * time.Second

var (
	// 誤りの名前はこの形のものだけを写す（応答の中身を、そのまま出力や sent.json に流さない）
	slackCodeRe = regexp.MustCompile(`^[a-z_]{1,80}$`)
	slackFileRe = regexp.MustCompile(`^[A-Z0-9]+$`)
	tokenRe     = regexp.MustCompile(`^[!-~]+$`)
)

// TokenDir は andon digest auth がトークンを置く dir（ADR-0044 決めたこと 14。compose の volume）。
// 外の信号の置き場（読み取り専用のトークン）とは分ける。
func TokenDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "andon", "digest")
}

// slack は Slack へファイルを1つ送る口（Go の標準の HTTP。決めたこと 20）。トークンは誤りの文に入れない。
type slack struct {
	api, token string
	http       *http.Client
}

func newSlack(env Env) (*slack, error) {
	if env.SlackAPI == "" || env.Tokens == "" {
		return nil, errors.New("送り口（Slack）が決まっていない")
	}
	path := filepath.Join(env.Tokens, "slack.token")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("トークン（Slack）が無い（andon digest auth slack で置いてから打ち直す）")
	}
	if err != nil {
		return nil, fmt.Errorf("トークン（Slack）が読めない: %v", err)
	}
	token := strings.TrimSpace(string(b))
	if !tokenRe.MatchString(token) {
		return nil, fmt.Errorf("%s の形が違う（andon digest auth slack で置き直す）", path)
	}
	return &slack{api: env.SlackAPI, token: token, http: &http.Client{
		Timeout: slackTimeout,
		// Slack のこの3つの呼び出しは転送しない。追うと、POST が GET に化ける
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// upload は body を name のファイルとして channel へ、comment を添えて1回の投稿で送り、ファイルの id を返す（決めたこと 13）。
// 呼び出しは 3 回: 置き場をもらう（名前とバイト数）・中身を置く・チャンネルへ出す（チャンネルと短い文）。
func (s *slack) upload(channel, name, comment string, body []byte) (string, error) {
	var slot struct {
		UploadURL string `json:"upload_url"`
		FileID    string `json:"file_id"`
	}
	const first, second, third = "files.getUploadURLExternal", "中身のアップロード", "files.completeUploadExternal"
	if err := s.call(first, url.Values{"filename": {name}, "length": {strconv.Itoa(len(body))}}, &slot); err != nil {
		return "", err
	}
	if slot.UploadURL == "" || !slackFileRe.MatchString(slot.FileID) {
		return "", fmt.Errorf("%s の応答に upload_url か file_id が無い", first)
	}
	// 置き場の URL はそれ自体が書き込みの鍵なので誤りに出さない。トークンも付けない
	req, err := http.NewRequest(http.MethodPost, slot.UploadURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%s の置き場の URL が読めない", second)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if _, err := s.do(second, req); err != nil {
		return "", err
	}
	files, err := json.Marshal([]map[string]string{{"id": slot.FileID, "title": name}})
	if err != nil {
		return "", err
	}
	form := url.Values{"files": {string(files)}, "channel_id": {channel}, "initial_comment": {comment}}
	if err := s.call(third, form, nil); err != nil {
		var se slackError
		// private のチャンネルに bot がいないときの名前は文書に無いので、channel_not_found でも招く手順を出す
		if errors.As(err, &se) && (se.code == "not_in_channel" || se.code == "channel_not_found") {
			return "", fmt.Errorf("%w。bot をチャンネル %s に招いてから打ち直す（Slack のそのチャンネルで /invite @<bot の名前>。招いてあればチャンネルの ID を確かめる）", err, channel)
		}
		return "", err
	}
	return slot.FileID, nil
}

// slackError は Slack が ok: false で返した誤り。
type slackError struct{ method, code string }

func (e slackError) Error() string { return e.method + " が " + e.code + " を返した" }

// call は API を1つ呼び、ok: true の応答を out に読む。
func (s *slack) call(method string, form url.Values, out any) error {
	req, err := http.NewRequest(http.MethodPost, s.api+method, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%s を呼べない: %v", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+s.token)
	b, err := s.do(method, req)
	if err != nil {
		return err
	}
	var res struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &res) != nil {
		return fmt.Errorf("%s の応答が読めない", method)
	}
	if !res.OK {
		if !slackCodeRe.MatchString(res.Error) {
			return fmt.Errorf("%s が失敗した（理由の形が読めない）", method)
		}
		return slackError{method, res.Error}
	}
	if out != nil && json.Unmarshal(b, out) != nil {
		return fmt.Errorf("%s の応答の形が違う", method)
	}
	return nil
}

// do は req を送り、HTTP 200 の本文を返す。
func (s *slack) do(step string, req *http.Request) ([]byte, error) {
	res, err := s.http.Do(req)
	if err != nil {
		var ue *url.Error // URL（置き場の鍵）を落として理由だけにする
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("%s が届かない: %v", step, err)
	}
	defer res.Body.Close() //nolint:errcheck // 読むだけの本文で、閉じる失敗は読んだ中身に効かない
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s が HTTP %d を返した", step, res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%s の応答を読めない: %v", step, err)
	}
	return b, nil
}
