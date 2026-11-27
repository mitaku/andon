package gate

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"andon/internal/spawn"
)

// StackedDiff は、積んだ課題の PR の変更したファイルと差分を、分かれ目 base（予約キー stacked_base）から head までで取る
// （ADR-0067 決めたこと 9）。GitHub の PR の差分は PR の base の今の先との merge-base から数えるので、前の課題の PR が
// rebase で書き直されると、前の課題の古い commit が混ざる。分かれ目から head までなら、積んだ課題の自分の変更だけになる。
// repo の clone（GHQ_ROOT の下。条件表の検査と同じ clone）に head を fetch して、git で数える。分かれ目は head の祖先なので
// head と一緒に手元に来る。分かれ目が head の祖先でなければ（キーと PR が食い違う）、数えずに失敗を返す（判定しない）。
// rename は GitHub の PR の files と同じく見つけて、元のパスを PreviousPath に置く。
func StackedDiff(repo, base, head string) ([]File, []byte, error) {
	dir, err := spawn.RepoPath(repo)
	if err != nil {
		return nil, nil, err
	}
	git := func(args ...string) ([]byte, error) {
		var errb bytes.Buffer
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Stderr = &errb
		out, err := c.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s: %v %s", args[0], err, strings.TrimSpace(errb.String()))
		}
		return out, nil
	}
	if _, err := git("fetch", "-q", "https://"+repo, head); err != nil {
		return nil, nil, fmt.Errorf("PR の head の fetch に失敗（積んだ課題の差分を分かれ目から数えられない）: %v", err)
	}
	if _, err := git("merge-base", "--is-ancestor", base+"^{commit}", head+"^{commit}"); err != nil {
		return nil, nil, fmt.Errorf("分かれ目 %s が PR の head %s の祖先でない（stacked_base と PR が食い違う。判定しない）", short(base), short(head))
	}
	opts := []string{"--no-ext-diff", "--no-textconv", "--no-color", "--find-renames"}
	num, err := git(append(append([]string{"diff", "--numstat", "-z"}, opts...), base, head)...)
	if err != nil {
		return nil, nil, err
	}
	files, err := ParseNumstat(num)
	if err != nil {
		return nil, nil, err
	}
	diff, err := git(append(append([]string{"diff"}, opts...), base, head)...)
	if err != nil {
		return nil, nil, err
	}
	return files, diff, nil
}

// ParseNumstat は `git diff --numstat -z` の出力をファイルの並びに直す（純粋関数）。1 件は「足した数<TAB>消した数<TAB>パス<NUL>」で、
// rename は「足した数<TAB>消した数<TAB><NUL>元のパス<NUL>新しいパス<NUL>」。バイナリの数は「-」で、GitHub の PR の files と同じく 0 と読む。
func ParseNumstat(b []byte) ([]File, error) {
	files := []File{}
	parts := strings.Split(string(b), "\x00")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		f := strings.SplitN(parts[i], "\t", 3)
		if len(f) != 3 {
			return nil, errors.New("分かれ目からの差分（git diff --numstat）が読めない")
		}
		n := func(s string) (int, error) {
			if s == "-" {
				return 0, nil
			}
			return strconv.Atoi(s)
		}
		add, aerr := n(f[0])
		del, derr := n(f[1])
		if aerr != nil || derr != nil {
			return nil, errors.New("分かれ目からの差分（git diff --numstat）の行数が読めない")
		}
		file := File{Path: f[2], Additions: add, Deletions: del}
		if f[2] == "" {
			if i+2 >= len(parts) || parts[i+1] == "" || parts[i+2] == "" {
				return nil, errors.New("分かれ目からの差分（git diff --numstat）の rename が読めない")
			}
			file.PreviousPath, file.Path = parts[i+1], parts[i+2]
			i += 2
		}
		files = append(files, file)
	}
	return files, nil
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
