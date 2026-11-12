package signal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"andon/internal/delegate"
)

// jsonDecode は出所の応答を読む（知らないキーは無視。出所の形は Andon の約束ではない）。
func jsonDecode(b []byte, v any) error { return json.Unmarshal(b, v) }

// Main は andon signal。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return 2 }
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "collect":
		given, due := "", false
		for rest := args[1:]; len(rest) > 0; {
			switch {
			case rest[0] == "--due" && !due:
				due, rest = true, rest[1:]
			case rest[0] == "--project" && len(rest) > 1 && given == "":
				given, rest = rest[1], rest[2:]
			default:
				return usage()
			}
		}
		ws := delegate.Workspace()
		slug, err := delegate.ProjectSlug(ws, given)
		if err != nil {
			fmt.Fprintf(stderr, "andon signal collect: %v\n", err)
			return 2
		}
		p, err := delegate.ReadProject(ws, slug)
		if errors.Is(err, delegate.ErrAbsent) {
			p, err = nil, nil
		}
		if err != nil {
			fmt.Fprintf(stderr, "andon signal collect: %v\n", err)
			return 1
		}
		failed, err := Collect(Options{Slug: slug, Project: p, Path: Path(slug), TokenDir: TokenDir(), Now: time.Now(), GH: RunGH, Due: due}, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "andon signal collect: %v\n", err)
			return 1
		}
		if failed > 0 {
			return 1
		}
		return 0
	case "delivered":
		rest, given := args[1:], ""
		if len(rest) == 4 && rest[0] == "--project" {
			given, rest = rest[1], rest[2:]
		}
		if len(rest) != 2 {
			return usage()
		}
		episode, err := strconv.Atoi(rest[1])
		if err != nil {
			return usage()
		}
		slug, err := delegate.ProjectSlug(delegate.Workspace(), given)
		if err != nil {
			fmt.Fprintf(stderr, "andon signal delivered: %v\n", err)
			return 2
		}
		if _, err := Delivered(Path(slug), rest[0], episode); err != nil {
			fmt.Fprintf(stderr, "andon signal delivered: %v\n", err)
			return 1
		}
		return 0
	case "auth":
		if len(args) != 2 {
			return usage()
		}
		if err := Auth(args[1], TokenDir(), stdin, stderr); err != nil {
			fmt.Fprintf(stderr, "andon signal auth: %v\n", err)
			return 2
		}
		return 0
	}
	return usage()
}

// Auth はトークンを1行読んで dir/<source>.token に mode 600 で置く（ADR-0011 決めたこと 10）。
// GitHub の出所は gh の資格情報を使うので置かない。
func Auth(source, dir string, stdin io.Reader, w io.Writer) error {
	switch {
	case strings.HasPrefix(source, "github-"):
		return errors.New("GitHub の出所は gh の資格情報をそのまま使う（gh auth login）。置くものは無い")
	case !slices.Contains(delegate.SignalSources, source):
		return fmt.Errorf("知らない出所: %q（sentry・linear・cloudflare）", source)
	}
	fmt.Fprintf(w, "%s のトークン（読み取り専用の scope で作る。linear はコメントが書けるまで）: ", source)
	restore := noEcho(stdin)
	tok, err := bufio.NewReader(stdin).ReadString('\n')
	restore()
	fmt.Fprintln(w)
	if err != nil && !(errors.Is(err, io.EOF) && tok != "") {
		return errors.New("トークンが読めない（標準入力に1行で渡す）")
	}
	tok = strings.TrimSpace(tok)
	if tok == "" || strings.ContainsAny(tok, " \t\r\n") || ctrlRe.MatchString(tok) {
		return errors.New("トークンは空白も制御文字も無い1語")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, source+".token")
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(tok + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Fprintf(w, "置いた: %s（mode 600）\n", path)
	return nil
}
