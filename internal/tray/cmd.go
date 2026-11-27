package tray

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"andon/internal/delegate"
	"andon/internal/pkgversion"
	"andon/internal/signal"
)

const AuthUsage = "usage: andon tray auth\n" +
	"Cloudflare の tray の API キー（セルごとに1つ）を標準入力から読み、" + "~/.config/andon/tray/" + KeyName + ".token にモード 600 で置く（ADR-0064 決めたこと 19）。\n" +
	"キーは引数に書かない。置き場は compose の volume tray で、セルを作り直しても残る。\n" +
	"1Password に置いたキーは、ホストで op read の出力を ssh 越しにこの葉へ流す。\n"

const StatusUsage = "usage: andon tray status\n" +
	"ローカルの tray の URL と動いているか、Cloudflare の tray の設定と到達、両方のバージョン、\n" +
	"ローカルの tray に直近 7 日に上がった枚数と開いた枚数を出す（ADR-0064 決めたこと 26。読むだけ）。\n" +
	"Cloudflare の tray を選んだかはプロジェクトごと（project.json）なので、プロジェクトを引けたときだけ見る。\n"

func AuthMain(args []string, stdin io.Reader, _, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprint(stderr, AuthUsage)
		return 2
	}
	if err := signal.PlaceToken(KeyDir(), KeyName, "Cloudflare の tray の API キー（このセルの分）", stdin, stderr); err != nil {
		fmt.Fprintf(stderr, "andon tray auth: %v\n", err)
		return 2
	}
	return 0
}

// StatusEnv は status が読むもの（検査が差し替える）。
type StatusEnv struct {
	Project string // 引けたプロジェクトの slug（無ければ空）
}

func StatusMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprint(stderr, StatusUsage)
		return 2
	}
	project := ""
	if p, ok := currentProject(); ok {
		project = p
	}
	Status(StatusEnv{Project: project}, stdout)
	return 0
}

// currentProject はほかの葉と同じ引き方で引いたプロジェクト（cli が KANBAN_ROOT に入れて渡す）。
var currentProject = func() (string, bool) {
	root := strings.TrimRight(os.Getenv("KANBAN_ROOT"), "/")
	if root == "" {
		return "", false
	}
	return root[strings.LastIndex(root, "/")+1:], true
}

// Status は状態を人が読む行で書く。読めない所は理由を書いて続ける（終了コードは 0）。
func Status(env StatusEnv, w io.Writer) {
	fmt.Fprintf(w, "パッケージ: %s\n", pkgVersion())
	c, running := Running()
	switch {
	case c == nil:
		fmt.Fprintf(w, "ローカルの tray: 動いていない（%s が無い。andon boot が起動する）\n", ControlPath())
	case !running:
		fmt.Fprintf(w, "ローカルの tray: 止まっている（%s を書いた serve の pid %d がもう無い。andon boot で起動し直す）\n", ControlPath(), c.PID)
	default:
		fmt.Fprintf(w, "ローカルの tray: %s （動いている。アップロードの口 %s）\n", c.ListOrigin, c.Upload)
		if v, err := metaVersion(c.Upload, ""); err != nil {
			fmt.Fprintf(w, "ローカルの tray のバージョン: 読めない（%s）\n", oneLine(err.Error()))
		} else {
			fmt.Fprintf(w, "ローカルの tray のバージョン: %s\n", v)
		}
		if s, err := readStats(c.Upload, "", 7); err != nil {
			fmt.Fprintf(w, "直近 7 日: 読めない（%s）\n", oneLine(err.Error()))
		} else {
			fmt.Fprintf(w, "直近 7 日: 上がった %d 枚、開いた %d 枚\n", s.Uploaded, s.Opened)
		}
	}
	if env.Project == "" {
		fmt.Fprintln(w, "Cloudflare の tray: プロジェクトを引けないので見ない（--project <slug> で指す）")
		return
	}
	remote, err := RemoteFor(delegate.Workspace(), env.Project)
	switch {
	case err != nil:
		fmt.Fprintf(w, "Cloudflare の tray: %s の設定を読めない（%s）\n", env.Project, oneLine(err.Error()))
		return
	case remote == nil:
		fmt.Fprintf(w, "Cloudflare の tray: %s は選んでいない（ローカルの tray にだけ送る）\n", env.Project)
		return
	}
	fmt.Fprintf(w, "Cloudflare の tray: %s は %s へ送る\n", env.Project, remote.Upload)
	if remote.Key == "" {
		fmt.Fprintf(w, "API キー: 無い（andon tray auth で %s に置く）\n", KeyPath())
		return
	}
	if v, err := metaVersion(remote.Upload, remote.Key); err != nil {
		fmt.Fprintf(w, "Cloudflare の tray のバージョン: 届かない（%s）\n", oneLine(err.Error()))
	} else {
		fmt.Fprintf(w, "Cloudflare の tray のバージョン: %s（届く）\n", v)
	}
}

func pkgVersion() string {
	i := pkgversion.Read(pkgversion.Home())
	if i.Commit == nil || *i.Commit == "" {
		if len(i.Reasons) > 0 {
			return "焼いた commit が分からない（" + i.Reasons[0] + "）"
		}
		return "焼いた commit が分からない"
	}
	return *i.Commit
}

var statusClient = &http.Client{Timeout: 5 * time.Second}

func get(base, key, path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := statusClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close() //nolint:errcheck // 読み終えた答えの本文
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, oneLine(string(b)))
	}
	return json.Unmarshal(b, v)
}

func metaVersion(base, key string) (string, error) {
	var m struct {
		Version string `json:"version"`
	}
	if err := get(base, key, "/v1/meta", &m); err != nil {
		return "", err
	}
	return m.Version, nil
}

// Stats は GET /v1/stats の答え。
type Stats struct {
	Days     int `json:"days"`
	Uploaded int `json:"uploaded"`
	Opened   int `json:"opened"`
}

func readStats(base, key string, days int) (*Stats, error) {
	var s Stats
	if err := get(base, key, fmt.Sprintf("/v1/stats?days=%d", days), &s); err != nil {
		return nil, err
	}
	return &s, nil
}
