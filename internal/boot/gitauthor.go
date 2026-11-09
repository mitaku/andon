package boot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"andon/internal/delegate"
)

// AgentName・AgentEmail はセルの repo（/workspace）での既定の作者。andon.toml の owner と一致しない値にする
// （委譲の照合〈delegate.OwnerFile〉を、担当や Chief of Staff の commit が素通りしないように。docs/decisions/2026-11-09-git-author-by-boot.md）。
const (
	AgentName  = "Andon agent"
	AgentEmail = "andon-agent@cell.invalid"
)

// noreply は GitHub の noreply アドレス（<id>+<login>@users.noreply.github.com か <login>@users.noreply.github.com）。
var noreply = regexp.MustCompile(`^(?:[0-9]+\+)?([^@+]+)@users\.noreply\.github\.com$`)

// genGitAuthor は git の既定の作者を ~/.gitconfig の includeIf で書く（~ は作り直すと消えるので毎起動）:
//
//	includeIf "gitdir:<ws>/"      → ~/.config/andon/git/cell.gitconfig     （Andon agent。owner でない）
//	includeIf "gitdir:<GHQ_ROOT>/" → ~/.config/andon/git/product.gitconfig （owner。案件の repo と、その worktree）
//
// 後ろの include が勝つので、/workspace/.ghq の下は product になる。worktree は元の repo の gitdir で当たる。
// owner が空なら product もセルの作者にする（commit は通し、owner の作者は名乗らない）。
// ~/.gitconfig に user.name か user.email が直に書いてあれば、人の設定として何もしない。git が無ければ何もしない。
func genGitAuthor(e Env) error {
	git, ok := e.has("git")
	if !ok {
		return nil
	}
	gc := filepath.Join(e.Home, ".gitconfig")
	cfg := func(args ...string) (string, error) {
		var errb bytes.Buffer
		c := exec.Command(git, append([]string{"config", "--file", gc}, args...)...)
		c.Stderr = &errb
		out, err := c.Output()
		var ee *exec.ExitError
		if errors.As(err, &ee) && (ee.ExitCode() == 1 || ee.ExitCode() == 5) && errb.Len() == 0 {
			return "", nil // キーが無い（--get は 1、--unset-all は 5）
		}
		if err != nil {
			return "", fmt.Errorf("git config %s: %v %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
		}
		return string(out), nil
	}
	if _, err := os.Stat(gc); err == nil {
		for _, k := range []string{"user.name", "user.email"} {
			v, err := cfg("--get", k)
			if err != nil {
				return err
			}
			if v != "" {
				e.logf("触らない: %s の %s（人が書いた作者）", gc, k)
				return nil
			}
		}
	}

	dir := filepath.Join(e.Home, ".config", "andon", "git")
	cell := filepath.Join(dir, "cell.gitconfig")
	if err := e.generate(cell, userSection(AgentName, AgentEmail), 0o644); err != nil {
		return err
	}
	want := [][2]string{{"gitdir:" + strings.TrimSuffix(e.Workspace, "/") + "/", cell}}
	if e.GhqRoot != "" {
		product := filepath.Join(dir, "product.gitconfig")
		name, email := AgentName, AgentEmail
		if owner := delegate.Owner(e.Workspace); owner != "" {
			name, email = ownerName(owner), owner
		} else {
			e.logf("andon.toml に owner が無い。案件の repo の作者もセルの作者（%s）にする", AgentEmail)
		}
		if err := e.generate(product, userSection(name, email), 0o644); err != nil {
			return err
		}
		want = append(want, [2]string{"gitdir:" + strings.TrimSuffix(e.GhqRoot, "/") + "/", product})
	}

	// 今の includeIf が want と同じ並び・同じ値なら書かない。違えば boot の分を外して、この順に足し直す。
	// boot の include（値が boot のファイル）だけを見る。want と同じ並び・同じ値なら書かない。
	// 違えば boot の行だけを外して、この順に足し直す（同じ条件の、人が置いた include には触らない）。
	have, err := cfg("--get-regexp", `^includeif\.gitdir:.*\.path$`)
	if err != nil {
		return err
	}
	owned := map[string]bool{}
	for _, w := range want {
		owned[w[1]] = true
	}
	var cur, lines []string
	for _, line := range strings.Split(strings.TrimSpace(have), "\n") {
		if i := strings.LastIndexByte(line, ' '); i >= 0 && owned[line[i+1:]] {
			cur = append(cur, line)
		}
	}
	for _, w := range want {
		lines = append(lines, "includeif."+w[0]+".path "+w[1])
	}
	if slices.Equal(cur, lines) {
		return nil
	}
	for _, line := range cur {
		i := strings.LastIndexByte(line, ' ')
		if _, err := cfg("--unset-all", line[:i], "^"+regexp.QuoteMeta(line[i+1:])+"$"); err != nil {
			return err
		}
	}
	for _, w := range want {
		if _, err := cfg("--add", "includeIf."+w[0]+".path", w[1]); err != nil {
			return err
		}
	}
	e.logf("git の作者: %s に includeIf を書いた", gc)
	return nil
}

// ownerName は owner の email から作者の名前を作る（noreply なら login、ほかは @ の前）。
func ownerName(email string) string {
	if m := noreply.FindStringSubmatch(email); m != nil {
		return m[1]
	}
	name, _, _ := strings.Cut(email, "@")
	return name
}

func userSection(name, email string) []byte {
	return []byte("[user]\n\tname = " + name + "\n\temail = " + email + "\n")
}
