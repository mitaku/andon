package boot

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Andon の外の skill（ADR-0018 決めたこと 5・前提 4）。
//
//   - 同梱の skill: イメージの備品（/opt/andon-tools）に同梱の skill を、boot が
//     <claude>/skills/<name> と ~/.agents/skills/<name> に symlink で張る（genBundledSkills。生成の一部）。
//     /opt/andon-tools の下を指す symlink は boot のもの（張り替え・外す）。実ディレクトリや別の先を指すものには触らない。
//   - 外の skill: <pkg>/gh-skills.txt（既定は空）と <ws>/local/gh-skills.txt の一覧を、gh のログインがあれば
//     gh skill install（latest）で入れ、gh skill update --all で上げる（installGhSkills。起動の一部。ネットワークが要る）。

// DefaultTools はイメージの備品（npm のグローバル）の置き場。
const DefaultTools = "/opt/andon-tools"

// bundledSkills は同梱の skill の名前と、備品の置き場からの相対パス。
var bundledSkills = map[string]string{
	"playwright-cli": "lib/node_modules/playwright/node_modules/playwright-core/lib/tools/skills/playwright-cli",
}

func (e Env) tools() string {
	if e.Tools != "" {
		return e.Tools
	}
	return DefaultTools
}

// genBundledSkills は同梱の skill の symlink を dir に張る。備品に無い skill のリンクと、
// 一覧から消えた名前の boot のリンク（備品の置き場の下を指す symlink）は外す。
func (e Env) genBundledSkills(dir string) error {
	want := map[string]string{}
	for name, rel := range bundledSkills {
		target := filepath.Join(e.tools(), rel)
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			want[name] = target
		}
	}
	have, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var errs []error
	for _, ent := range have {
		p := filepath.Join(dir, ent.Name())
		if _, ok := want[ent.Name()]; ok || !e.ownedToolLink(p) {
			continue
		}
		if err := os.Remove(p); err != nil {
			errs = append(errs, err)
			continue
		}
		e.logf("外した: %s", p)
	}
	for _, name := range sortedKeys(want) {
		link := filepath.Join(dir, name)
		if _, err := os.Lstat(link); err == nil && !e.ownedToolLink(link) {
			e.logf("触らない: %s（boot が張ったリンクでない）", link)
			continue
		}
		errs = append(errs, e.symlink(want[name], link))
	}
	return errors.Join(errs...)
}

// ownedToolLink は link が備品の置き場（今の ANDON_TOOLS と既定の /opt/andon-tools）の下を指す symlink か。
func (e Env) ownedToolLink(link string) bool {
	to, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(to) {
		to = filepath.Join(filepath.Dir(link), to)
	}
	to = filepath.Clean(to)
	for _, t := range []string{e.tools(), DefaultTools} {
		if strings.HasPrefix(to, filepath.Clean(t)+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ghSkill は外の skill の一覧の1行（<owner>/<repo> <skill>）。
type ghSkill struct{ repo, skill string }

// dirName は gh skill install が置くディレクトリの名前（skill の名前・author/skill・パスの最後の要素）。
func (s ghSkill) dirName() string {
	n, _, _ := strings.Cut(s.skill, "@")
	return path.Base(strings.TrimSuffix(n, "/SKILL.md"))
}

// GhSkillsFile は外の skill の一覧のファイルの名前（パッケージの直下とセルの local/）。
const GhSkillsFile = "gh-skills.txt"

// ghSkillList はパッケージとセルの一覧を合わせて読む（同じ行は1つに）。1行は「<owner>/<repo> <skill>」。
// 空行と # からの行は飛ばす。形の違う行はログに1行出して飛ばす。
func (e Env) ghSkillList() ([]ghSkill, error) {
	var list []ghSkill
	seen := map[ghSkill]bool{}
	for _, p := range []string{filepath.Join(e.Package, GhSkillsFile), filepath.Join(e.Workspace, "local", GhSkillsFile)} {
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		for n := 1; sc.Scan(); n++ {
			ln := strings.TrimSpace(sc.Text())
			if ln == "" || strings.HasPrefix(ln, "#") {
				continue
			}
			f := strings.Fields(ln)
			if len(f) != 2 || strings.Count(f[0], "/") != 1 || strings.HasPrefix(f[0], "-") || strings.HasPrefix(f[1], "-") {
				e.logf("飛ばす: %s:%d は「<owner>/<repo> <skill>」の形でない", p, n)
				continue
			}
			s := ghSkill{f[0], f[1]}
			if !seen[s] {
				seen[s] = true
				list = append(list, s)
			}
		}
	}
	return list, nil
}

// ghTimeout は gh skill の1回の打ち込みの上限（ネットワークが詰まっても起動を止めきらない）。
var ghTimeout = 2 * time.Minute

// installGhSkills は外の skill の一覧を gh skill install で入れ、gh skill update --all で上げる。
// 一覧が空なら何も打たない。gh が無い・ログインしていなければ飛ばして1行残す（gh auth setup-git と同じ扱い）。
// 入れるのは置き場（Claude Code は <claude>/skills、Codex は ~/.agents/skills）に同じ名前が無いときだけ
// （利用者の skill を上書きしない。入っているものは update が上げる）。gh skill install の --agent は1つしか効かないので
// エージェントごとに打つ。gh の出力はログに出さない（失敗のときだけ最後の1行）。
func installGhSkills(e Env) error {
	list, err := e.ghSkillList()
	if err != nil || len(list) == 0 {
		return err
	}
	gh, ok := e.has("gh")
	if !ok {
		e.logf("gh が無い。外の skill（%d 件）は入れない", len(list))
		return nil
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, gh, args...)
		cmd.Dir = e.Home // project scope（cwd の git repo）に触れない
		cmd.Env = append(os.Environ(), "HOME="+e.Home, "CLAUDE_CONFIG_DIR="+e.claudeDir(), "GH_PROMPT_DISABLED=1")
		return cmd.CombinedOutput()
	}
	if _, err := run("auth", "status"); err != nil {
		e.logf("gh はログインしていない。外の skill（%d 件）は入れない（gh auth login のあと、再起動）", len(list))
		return nil
	}
	var errs []error
	for _, s := range list {
		for _, ag := range []struct{ agent, dir string }{
			{"claude-code", filepath.Join(e.claudeDir(), "skills")},
			{"codex", filepath.Join(e.Home, ".agents", "skills")},
		} {
			if _, err := os.Lstat(filepath.Join(ag.dir, s.dirName())); err == nil {
				continue
			}
			if out, err := run("skill", "install", s.repo, s.skill, "--agent", ag.agent, "--scope", "user"); err != nil {
				errs = append(errs, fmt.Errorf("gh skill install %s %s（%s）: %v: %s", s.repo, s.skill, ag.agent, err, lastLine(out)))
				continue
			}
			e.logf("入れた: %s %s（%s）", s.repo, s.skill, ag.agent)
		}
	}
	if out, err := run("skill", "update", "--all"); err != nil {
		errs = append(errs, fmt.Errorf("gh skill update --all: %v: %s", err, lastLine(out)))
	}
	return errors.Join(errs...)
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
