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
//   - 外の skill: <pkg>/gh-skills.txt（全セルに配る既定。ADR-0038）と <ws>/local/gh-skills.txt の一覧を、gh のログインがあれば
//     gh skill install（latest）で入れ、置き場ごとに gh skill update --all --dir <置き場> <名前…> で上げる
//     （installGhSkills。起動の一部。ネットワークが要る。上げるのは出所が一覧の repo と同じ skill だけ）。

const DefaultTools = "/opt/andon-tools"

var bundledSkills = map[string]string{
	"playwright-cli": "lib/node_modules/playwright/node_modules/playwright-core/lib/tools/skills/playwright-cli",
}

func (e Env) tools() string {
	if e.Tools != "" {
		return e.Tools
	}
	return DefaultTools
}

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

type ghSkill struct{ repo, skill string }

// dirName は gh skill install が置くディレクトリの名前（skill の名前・author/skill・パスの最後の要素）。
func (s ghSkill) dirName() string {
	n, _, _ := strings.Cut(s.skill, "@")
	return path.Base(strings.TrimSuffix(n, "/SKILL.md"))
}

const GhSkillsFile = "gh-skills.txt"

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

var ghTimeout = 2 * time.Minute

// gh skill install の --agent は1つしか効かないので、エージェントごとに打つ。
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
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	run := func(args ...string) ([]byte, error) {
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
	timedOut := func() bool {
		if ctx.Err() == nil {
			return false
		}
		errs = append(errs, fmt.Errorf("%s を過ぎたので残りを打たない", ghTimeout))
		return true
	}
	agents := []struct{ agent, dir string }{
		{"claude-code", filepath.Join(e.claudeDir(), "skills")},
		{"codex", filepath.Join(e.Home, ".agents", "skills")},
	}
	for _, s := range list {
		if timedOut() {
			return errors.Join(errs...)
		}
		for _, ag := range agents {
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
	if timedOut() {
		return errors.Join(errs...)
	}
	// 上げるのは、一覧の skill のうち、置き場にあって出所（SKILL.md の metadata.github-repo。gh skill install が書く）が
	// 一覧の repo と同じものだけ。gh skill update は名前だけだと全部の置き場から同じ名前を拾うので、置き場ごとに --dir で
	// 絞って打つ（一覧の外の skill も、別の出所から入れた同じ名前の利用者の skill も上げない）。
	// 渡す名前が無い置き場には打たない（入っていない名前だけを渡すと gh は失敗で返す）。
	for _, ag := range agents {
		var names []string
		seen := map[string]bool{}
		for _, s := range list {
			n := s.dirName()
			if !seen[n] && fromRepo(filepath.Join(ag.dir, n, "SKILL.md"), s.repo) {
				seen[n] = true
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			continue
		}
		if timedOut() {
			return errors.Join(errs...)
		}
		args := append([]string{"skill", "update", "--all", "--dir", ag.dir}, names...)
		if out, err := run(args...); err != nil {
			errs = append(errs, fmt.Errorf("gh %s: %v: %s", strings.Join(args, " "), err, lastLine(out)))
		}
	}
	return errors.Join(errs...)
}

func fromRepo(skillMD, repo string) bool {
	b, err := os.ReadFile(skillMD)
	if err != nil {
		return false
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, ln := range lines[1:] {
		t := strings.TrimSpace(ln)
		if t == "---" {
			return false
		}
		if v, ok := strings.CutPrefix(t, "github-repo:"); ok {
			v = strings.TrimSuffix(strings.Trim(strings.TrimSpace(v), `"'`), ".git")
			return strings.EqualFold(v, "https://github.com/"+repo)
		}
	}
	return false
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
