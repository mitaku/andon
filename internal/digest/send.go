package digest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/publish"
)

// Sent は library/digest/sent.json（schema/digest-sent.schema.json。docs/contract.md §28）。
type Sent struct {
	Issue        string        `json:"issue"`
	Since        string        `json:"since"`
	Until        string        `json:"until"`
	Destinations []Destination `json:"destinations"`
}

// Destination は送り先1つの結果。
type Destination struct {
	Kind  string `json:"kind"`
	At    string `json:"at"`
	OK    bool   `json:"ok"`
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

// done は送り先が全部済んでいるかと、最後に済んだ時刻。
func (s *Sent) done() (string, bool) {
	at := ""
	for _, d := range s.Destinations {
		if !d.OK {
			return "", false
		}
		if d.At > at {
			at = d.At
		}
	}
	return at, len(s.Destinations) > 0
}

func readSent(path string) (*Sent, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := contract.SchemaDigestSent.Check(b); err != nil {
		return nil, err
	}
	var s Sent
	if err := contract.Decode(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// filesEnv は files/ に出すときの publish の値。日付は期間の終わりの日（セルの時刻）で、組み立てるときと送るときで同じ名前になる。
func filesEnv(env Env, in *Input) publish.Env {
	until, _ := time.Parse(timeLayout, in.Until)
	return publish.Env{Workspace: env.Workspace, Hostname: env.Hostname, Now: until.In(time.Local)}
}

// filesSlug は files/ に出す名前の slug（決めたこと 15。同じ日に臨時の 1 枚を作っても上書きしないよう課題の番号を入れる）。
func filesSlug(in *Input) string {
	return "digest-" + in.Project + "-" + in.Issue
}

// filesURL は digest.html を files/ に出したときの URL。
func filesURL(env Env, in *Input) string {
	pe := filesEnv(env, in)
	return publish.URL(pe, publish.Name(pe, filesSlug(in), ".html"))
}

// Send は課題 id の digest.html を送り先へ出し、files/ の URL を返す。送り先は今は files/ だけ（決めたこと 15）。
// 済んだ送り先には送らない。失敗した送り先には送り直す（決めたこと 16）。
func Send(env Env, id string) (string, error) {
	in, err := readInput(env.Root, id)
	if err != nil {
		return "", err
	}
	d := dir(env.Root, id)
	html := filepath.Join(d, htmlFile)
	if _, err := os.Stat(html); err != nil {
		return "", fmt.Errorf("%s が無い（先に andon digest build %s）", html, id)
	}
	path := filepath.Join(d, sentFile)
	sent := &Sent{Issue: id, Since: in.Since, Until: in.Until}
	switch old, err := readSent(path); {
	case err == nil && old.Since == in.Since && old.Until == in.Until:
		sent = old
	case err == nil, errors.Is(err, fs.ErrNotExist):
		// 集め直した（期間が違う）ときは、前の結果を捨てて送り直す
	default:
		return "", fmt.Errorf("%s が読めない（直すか消す）: %v", path, err)
	}
	for _, x := range sent.Destinations {
		if x.Kind == "files" && x.OK {
			return x.URL, nil
		}
	}
	now := env.Now.UTC().Format(timeLayout)
	res := Destination{Kind: "files", At: now}
	url, perr := publish.Publish(filesEnv(env, in), html, filesSlug(in))
	if perr != nil {
		res.Error = oneLine(perr.Error())
	} else {
		res.OK, res.URL = true, url
	}
	kept := sent.Destinations[:0]
	for _, x := range sent.Destinations {
		if x.Kind != "files" {
			kept = append(kept, x)
		}
	}
	sent.Destinations = append(kept, res)
	if err := contract.Write(path, contract.SchemaDigestSent, sent); err != nil {
		return "", fmt.Errorf("sent.json が schema/digest-sent.schema.json に合わない（書かない）: %v", err)
	}
	if perr != nil {
		return "", fmt.Errorf("files/ に出せない: %v", perr)
	}
	return url, nil
}

func oneLine(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
	if s == "" {
		return "理由が分からない"
	}
	return s
}
