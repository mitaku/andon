// Package create は andon create [slug]: 建屋の repo に projects/<slug>/ を立てる（ADR-0004 決めたこと 1 の予約名）。
//
//	projects/<slug>/kanban/      台帳（kanban init。prefix と repos を渡す）
//	projects/<slug>/project.json 下書き（version・project〈題・接頭字〉・repos。delegation は無し＝無委譲）。commit しない
//	projects/<slug>/AGENTS.md    薄い地図の見出しだけ
//	repos の clone               ghq get（GHQ_ROOT の下に無いものだけ）
//
// 引数で足りないもの（slug・repos・prefix・題）は、端末なら既定値を示して順に聞く。端末でなければ聞かない:
// slug が無ければ止まり、ほかは既定（repos は無し・prefix は slug の頭文字・題は slug）。
// projects/<slug> が既にあれば何も変えずに止まる。役の端末（hc のラベル role= か task=）では打てない。
package create

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/spawn"
)

// Usage は andon create --help の中身。
const Usage = "usage: andon create [slug] [--repo <host>/<owner>/<repo>]… [--prefix X] [--title <題>]\n" +
	"人が建屋の repo（ANDON_WORKSPACE。既定 /workspace）で打つ。projects/<slug>/ に台帳（kanban/）・project.json の下書き・\n" +
	"AGENTS.md の雛形を作り、repo を ghq get で迎える（無いものだけ）。project.json と AGENTS.md は commit しない（人が見て commit する）。\n" +
	"足りない引数は、端末なら既定値を示して順に聞く（repos は 1 行に 1 つ、空行で終わり）。端末でなければ聞かない:\n" +
	"slug が無ければ止まり、ほかは既定（repos 無し・prefix は slug の頭文字・題は slug）。\n" +
	"slug は英小文字・数字・-。projects/<slug> が既にあれば何も変えずに止まる。役の端末（hc のラベル role= か task=）では打てない。\n"

const (
	exitFail  = 1
	exitUsage = 2
)

var (
	slugRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	prefixRe = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
)

// 差し替えられる外部とのつなぎ（テスト用）。
// /dev/null も文字デバイスなので除く（スクリプトの </dev/null を端末とみなさない）。
var isTerminal = func(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, null)
}

// input は引数で渡されたもの（空は渡されていない。repos は hasRepo で見分ける）。
type input struct {
	slug    string
	repos   []string
	prefix  string
	title   string
	hasRepo bool
}

func parse(args []string) (*input, error) {
	in := &input{}
	for rest := args; len(rest) > 0; {
		switch a := rest[0]; a {
		case "--repo", "--prefix", "--title":
			if len(rest) < 2 {
				return nil, fmt.Errorf("%s に値が無い", a)
			}
			v := rest[1]
			switch a {
			case "--repo":
				in.repos, in.hasRepo = append(in.repos, v), true
			case "--prefix":
				in.prefix = v
			case "--title":
				in.title = v
			}
			rest = rest[2:]
		default:
			if strings.HasPrefix(a, "-") || in.slug != "" {
				return nil, fmt.Errorf("知らない引数: %s", a)
			}
			in.slug, rest = a, rest[1:]
		}
	}
	return in, nil
}

// DefaultPrefix は slug の - で区切った頭文字を英大文字にしたもの（英字で始まるまで頭を落とす。何も残らなければ P）。
func DefaultPrefix(slug string) string {
	var b strings.Builder
	for _, part := range strings.Split(slug, "-") {
		if part != "" {
			b.WriteRune(unicode.ToUpper(rune(part[0])))
		}
	}
	p := strings.TrimLeftFunc(b.String(), func(r rune) bool { return r < 'A' || r > 'Z' })
	if p == "" {
		return "P"
	}
	return p
}

func checkSlug(s string) error {
	if !slugRe.MatchString(s) {
		return fmt.Errorf("slug は英小文字・数字・-（英小文字か数字で始まる）: %q", s)
	}
	return nil
}

func checkRepos(rs []string) error {
	for _, r := range rs {
		if !spawn.RepoRe.MatchString(r) {
			return fmt.Errorf("repo は <host>/<owner>/<repo>: %q", r)
		}
	}
	s := slices.Clone(rs)
	slices.Sort(s)
	if len(slices.Compact(s)) != len(rs) {
		return errors.New("同じ repo が2つある")
	}
	return nil
}

// asker は端末で1問ずつ聞く。
type asker struct {
	r *bufio.Reader
	w io.Writer
}

func (a *asker) line(prompt string) (string, error) {
	fmt.Fprint(a.w, prompt)
	s, err := a.r.ReadString('\n')
	if err != nil && (err != io.EOF || s == "") {
		return "", errors.New("入力が途中で終わった")
	}
	return strings.TrimSpace(s), nil
}

// ask は既定値 def を示して聞き、check を通るまで聞き直す。空の答えは def。
func (a *asker) ask(label, def string, check func(string) error) (string, error) {
	for {
		prompt := label + ": "
		if def != "" {
			prompt = fmt.Sprintf("%s [%s]: ", label, def)
		}
		s, err := a.line(prompt)
		if err != nil {
			return "", err
		}
		if s == "" {
			s = def
		}
		if err := check(s); err != nil {
			fmt.Fprintf(a.w, "  %v\n", err)
			continue
		}
		return s, nil
	}
}

func (a *asker) repos() ([]string, error) {
	fmt.Fprintln(a.w, "書いてよい repo（<host>/<owner>/<repo>。1 行に 1 つ、空行で終わり。無ければそのまま空行）:")
	var out []string
	for {
		s, err := a.line("  repo: ")
		if err != nil {
			return nil, err
		}
		if s == "" {
			return out, nil
		}
		if err := checkRepos(append(slices.Clone(out), s)); err != nil {
			fmt.Fprintf(a.w, "  %v\n", err)
			continue
		}
		out = append(out, s)
	}
}

// fill は足りないものを聞く（端末のとき）か既定で埋める（端末でないとき）。聞いたら最後に確かめる。
func fill(in *input, a *asker) error {
	checkPrefix := func(s string) error {
		if !prefixRe.MatchString(s) {
			return fmt.Errorf("prefix は英大文字で始まる英大文字・数字: %q", s)
		}
		return nil
	}
	checkTitle := func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("題が空")
		}
		return nil
	}
	asked := false
	var err error
	if in.slug == "" {
		if a == nil {
			return errors.New("slug が無い（端末でないので聞かない）。andon create <slug> と打つ")
		}
		if in.slug, err = a.ask("slug（英小文字・数字・-。projects/<slug> になる）", "", checkSlug); err != nil {
			return err
		}
		asked = true
	}
	if err := checkSlug(in.slug); err != nil {
		return err
	}
	if !in.hasRepo && a != nil {
		if in.repos, err = a.repos(); err != nil {
			return err
		}
		asked = true
	}
	if err := checkRepos(in.repos); err != nil {
		return err
	}
	if in.prefix == "" {
		in.prefix = DefaultPrefix(in.slug)
		if a != nil {
			if in.prefix, err = a.ask("タスクの id の接頭字（prefix）", in.prefix, checkPrefix); err != nil {
				return err
			}
			asked = true
		}
	}
	if err := checkPrefix(in.prefix); err != nil {
		return err
	}
	if in.title == "" {
		in.title = in.slug
		if a != nil {
			if in.title, err = a.ask("題（人に見せる名前）", in.title, checkTitle); err != nil {
				return err
			}
			asked = true
		}
	}
	if err := checkTitle(in.title); err != nil {
		return err
	}
	if !asked {
		return nil
	}
	repos := "無し"
	if len(in.repos) > 0 {
		repos = strings.Join(in.repos, " ")
	}
	fmt.Fprintf(a.w, "\nprojects/%s/ を作る: 題 %s・prefix %s・repos %s\n", in.slug, in.title, in.prefix, repos)
	ok, err := a.ask("作りますか（y/n）", "y", func(s string) error {
		if s != "y" && s != "n" {
			return errors.New("y か n")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if ok != "y" {
		return errCanceled
	}
	return nil
}

var errCanceled = errors.New("取り消した。何も作っていない")

// Draft は project.json の下書き（delegation は無し＝無委譲）。Andon が読むのと同じ検査を通したものだけを返す。
func Draft(slug, title, prefix string, repos []string) ([]byte, error) {
	if repos == nil {
		repos = []string{}
	}
	b := contract.Marshal(delegate.Project{Version: 1, Project: delegate.ProjectInfo{Title: title, Prefix: prefix}, Repos: repos})
	if _, err := delegate.ParseProject(b, slug); err != nil {
		return nil, fmt.Errorf("project.json の下書きが Andon の検査に通らない: %v", err)
	}
	return b, nil
}

// AgentsMD は AGENTS.md の雛形（薄い地図の見出しだけ。ADR-0004 決めたこと 1）。
func AgentsMD(title string) []byte {
	return []byte("# " + title + "\n\n" +
		"## 何のプロジェクトか\n\n" +
		"## どこに何があるか\n\n" +
		"## 毎回要る決まり\n\n" +
		"## Chief of Staff\n")
}

// ghqRoot は GHQ_ROOT（無ければ ghq root）。
func ghqRoot() (string, error) {
	if r := os.Getenv("GHQ_ROOT"); r != "" {
		return r, nil
	}
	out, err := exec.Command("ghq", "root").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "", errors.New("GHQ_ROOT が無く ghq root も読めない")
	}
	return strings.TrimSpace(string(out)), nil
}

// fetch は repos のうち clone の無いものを ghq get で迎える。迎えた repo を返す。
func fetch(repos []string, stderr io.Writer) ([]string, error) {
	if len(repos) == 0 {
		return nil, nil
	}
	root, err := ghqRoot()
	if err != nil {
		return nil, err
	}
	var got []string
	for _, r := range repos {
		if st, err := os.Stat(filepath.Join(root, r)); err == nil && st.IsDir() {
			continue
		}
		c := exec.Command("ghq", "get", r)
		c.Stdout, c.Stderr = stderr, stderr
		if err := c.Run(); err != nil {
			return got, fmt.Errorf("ghq get %s が落ちた: %v", r, err)
		}
		got = append(got, r)
	}
	return got, nil
}

// Main は andon create。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	in, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "andon create: %v\n%s", err, Usage)
		return exitUsage
	}
	if err := delegate.AgentTerminal("project.json を書くのは人だけ。人の端末で打つ"); err != nil {
		fmt.Fprintf(stderr, "andon create: %v\n", err)
		return exitFail
	}
	ws := delegate.Workspace()
	projects := filepath.Join(ws, "projects")
	if st, err := os.Stat(projects); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "andon create: %s が無い（建屋の repo で打つ。ANDON_WORKSPACE）\n", projects)
		return exitFail
	}
	var a *asker
	if f, ok := stdin.(*os.File); ok && isTerminal(f) {
		a = &asker{r: bufio.NewReader(stdin), w: stderr}
	}
	exists := func() bool {
		_, err := os.Lstat(filepath.Join(projects, in.slug))
		return err == nil
	}
	// 聞く前に分かるなら聞かずに止める
	if in.slug != "" && checkSlug(in.slug) == nil && exists() {
		fmt.Fprintf(stderr, "andon create: projects/%s が既にある。何も変えていない（そのまま使うなら andon open %s）\n", in.slug, in.slug)
		return exitFail
	}
	if err := fill(in, a); errors.Is(err, errCanceled) {
		fmt.Fprintf(stderr, "andon create: %v\n", err)
		return exitFail
	} else if err != nil {
		fmt.Fprintf(stderr, "andon create: %v\n", err)
		return exitUsage
	}
	if exists() {
		fmt.Fprintf(stderr, "andon create: projects/%s が既にある。何も変えていない（そのまま使うなら andon open %s）\n", in.slug, in.slug)
		return exitFail
	}
	draft, err := Draft(in.slug, in.title, in.prefix, in.repos)
	if err != nil {
		fmt.Fprintf(stderr, "andon create: %v\n", err)
		return exitFail
	}
	if err := create(ws, in, draft, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "andon create: %v\n", err)
		return exitFail
	}
	return 0
}

func create(ws string, in *input, draft []byte, stdout, stderr io.Writer) error {
	// repo を先に迎える（落ちたら projects/<slug> は作らない）
	got, err := fetch(in.repos, stderr)
	for _, r := range got {
		fmt.Fprintf(stdout, "迎えた: %s（ghq get）\n", r)
	}
	if err != nil {
		return fmt.Errorf("%v。projects/%s は作っていない", err, in.slug)
	}
	rel := filepath.Join("projects", in.slug)
	dir := filepath.Join(ws, rel)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	// ここから先で落ちたら、作った projects/<slug> ごと退ける（作る前は無かった）
	undo := func(err error) error {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("%v。projects/%s は退けた", err, in.slug)
	}
	kargs := []string{"init", "--root", dir, "--prefix", in.prefix}
	for _, r := range in.repos {
		kargs = append(kargs, "--repo", r)
	}
	if _, err := spawn.OutputIn(dir, nil, "", "kanban", kargs...); err != nil {
		return undo(fmt.Errorf("kanban init が落ちた: %v", err))
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), draft, 0o644); err != nil {
		return undo(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), AgentsMD(in.title), 0o644); err != nil {
		return undo(err)
	}
	fmt.Fprintf(stdout, "作った: %s/（kanban/・project.json・AGENTS.md）\n", rel)
	fmt.Fprintf(stdout, "project.json と AGENTS.md は人のファイルで、commit していない。中身を見て、andon.toml の owner の email で commit する:\n")
	fmt.Fprintf(stdout, "  cd %s && git add %s/project.json %s/AGENTS.md && git commit -m '%s'\n", ws, rel, rel, rel)
	fmt.Fprintf(stdout, "確かめる: andon delegate show --project %s（無委譲と出る）\n", in.slug)
	fmt.Fprintf(stdout, "Chief of Staff を起こす: andon open %s\n", in.slug)
	return nil
}
