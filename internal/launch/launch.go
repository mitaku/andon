// Package launch は起動の記録（docs/contract.md §18。ADR-0028 決めたこと 3・7・14 の 5）を集めて足す。
//
// エージェントを起動する command（andon issue start・andon issue resume・dispatcher のレビュアーの起動・
// andon chief-engineer・andon chief-of-staff）が、hc run の直前に Gather で集め、起動できたら端末の address を足して Append する。
// 1回の起動に1行（JSON Lines）。追記だけで、前の行は書き換えない。置き場:
//
//	担当・レビュアー・Chief Engineer  <台帳>/issues/<id>/library/launches.jsonl
//	Chief of Staff                    <台帳>/chief-of-staff/launches.jsonl（課題を持たないので台帳の直下）
//
// 取れない値は null にし、エラーにしない（記録が取れないことで起動を止めない）。
package launch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andon/internal/contract"

	"go.yaml.in/yaml/v3"

	"andon/internal/pkgversion"
	"andon/internal/spawn"
)

// File は記録のファイルの名前。
const File = "launches.jsonl"

// 役（role）と起動のしかた（launch）の値。
const (
	RoleWorker         = "worker"
	RoleReviewer       = "reviewer"
	RoleChiefEngineer  = "chief-engineer"
	RoleChiefOfStaff   = "chief-of-staff"
	LaunchStart        = "start"    // 新しく起動した
	LaunchResume       = "resume"   // 止まった端末をセッションの続きから起動し直した
	LaunchFresh        = "fresh"    // 止まった端末を新しいセッションで起動し直した（andon issue resume --fresh）
	LaunchFallback     = "fallback" // 使用量の上限で止まったレビュアーを別のプロファイルで起動し直した
	externalSkill      = "skill"
	externalPlugin     = "plugin"
	harnessTimeout     = 10 * time.Second
	memoryIndexName    = "MEMORY.md"
	installedPlugins   = "installed_plugins.json"
	skillFile          = "SKILL.md"
	frontmatterMarker  = "---"
	defaultWorkspace   = "/workspace"
	memoryDirName      = "memory"
	chiefOfStaffFolder = "chief-of-staff"
)

// Record は起動の記録の1行（schema/launch.schema.json）。
type Record struct {
	At           string          `json:"at"`
	Role         string          `json:"role"`
	Launch       string          `json:"launch"`
	Issue        *string         `json:"issue"`
	Profile      string          `json:"profile"`
	Template     string          `json:"template"`
	Command      *string         `json:"command"`
	Terminal     string          `json:"terminal"`
	Session      *string         `json:"session"`
	Harness      Harness         `json:"harness"`
	Package      pkgversion.Info `json:"package"`
	Workspace    Checkout        `json:"workspace"`
	Repo         *Repo           `json:"repo"`
	External     *[]External     `json:"external"`
	Drift        *bool           `json:"drift"`
	DriftReasons []string        `json:"drift_reasons"`
	MemoryIndex  *string         `json:"memory_index"`
}

// Harness はエージェントを動かすもの（claude か codex）と、その版（<name> --version の1行目）。
type Harness struct {
	Name    string  `json:"name"`
	Version *string `json:"version"`
}

// Checkout は git の checkout の commit と、commit していない変更があるか。
type Checkout struct {
	Path   string  `json:"path"`
	Commit *string `json:"commit"`
	Dirty  *bool   `json:"dirty"`
}

// Repo は案件の repo（担当・レビュアー・Chief Engineer の worktree）。
type Repo struct {
	Name string `json:"name"`
	Checkout
}

// External は外から入る skill（gh skill install）と Claude Code の plugin の1つ。
type External struct {
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Source   *string `json:"source"`
	Ref      *string `json:"ref"`
	Version  *string `json:"version"`
	Revision *string `json:"revision"`
}

// Spec は呼び手が決めて渡すもの。
type Spec struct {
	Role     string
	Launch   string
	Issue    string // 空なら null（Chief of Staff）
	Profile  string
	Template string // hc の template の名前（起動し直しは <profile>-resume）
	// Command は template の command（起動の前の確かめで hc templates から読んだもの）。nil なら Gather が hc templates で引く
	Command  *string
	Session  string // 空なら null（Codex の新しい起動・Chief of Staff）
	Repo     string // 案件の repo（<host>/<owner>/<repo>）。空なら repo は null
	RepoPath string // その worktree
	Brand    string // claude か codex（ledger.Brand）
}

// Gather は起動の直前に記録を集める。Terminal は起動のあとに呼び手が埋める。
func Gather(s Spec) Record {
	r := Record{
		At:       time.Now().UTC().Format(time.RFC3339),
		Role:     s.Role,
		Launch:   s.Launch,
		Issue:    strPtr(s.Issue),
		Profile:  s.Profile,
		Template: s.Template,
		Session:  strPtr(s.Session),
		Harness:  Harness{Name: s.Brand, Version: harnessVersion(s.Brand)},
		Package:  pkgversion.Read(pkgversion.Home()),
	}
	if s.Command != nil {
		r.Command = strPtr(*s.Command)
	} else if cmd, found, err := spawn.TemplateCommand(s.Template); err == nil && found {
		r.Command = strPtr(cmd)
	}
	r.Workspace = checkout(Workspace())
	if s.Repo != "" {
		r.Repo = &Repo{Name: s.Repo, Checkout: checkout(s.RepoPath)}
	}
	if ext, err := externals(s.Brand); err == nil {
		r.External = &ext
	}
	r.Drift, r.DriftReasons = drift(r.Package, r.Workspace)
	if s.Brand == "claude" {
		if b, err := os.ReadFile(filepath.Join(Workspace(), memoryDirName, memoryIndexName)); err == nil {
			t := string(b)
			r.MemoryIndex = &t
		}
	}
	return r
}

// IssuePath は課題 id の記録の置き場（dir は台帳。issues/ の親）。
func IssuePath(dir, id string) string {
	return filepath.Join(dir, "issues", id, "library", File)
}

// ChiefOfStaffPath は Chief of Staff の記録の置き場（dir は台帳）。
func ChiefOfStaffPath(dir string) string {
	return filepath.Join(dir, chiefOfStaffFolder, File)
}

// Append は path に r を1行足す（置き場が無ければ作る）。前の行には触らない。
func Append(path string, r Record) error {
	line, err := contract.EncodeLine(contract.SchemaLaunch, r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Workspace はセルの repo（ANDON_WORKSPACE。既定 /workspace）。
func Workspace() string {
	if ws := os.Getenv("ANDON_WORKSPACE"); ws != "" {
		return ws
	}
	return defaultWorkspace
}

// drift は、当時の文を git から復元できないかを決める。パッケージの置き場の食い違い（andon version の drift）と、
// セルの repo の commit していない変更を見る。案件の repo の変更は担当の作業そのもの（起動し直しでは毎回ある）なので数えない。
func drift(pkg pkgversion.Info, ws Checkout) (*bool, []string) {
	reasons := []string{}
	d, unknown := false, false
	switch {
	case pkg.Drift == nil:
		unknown = true
		for _, r := range pkg.Reasons {
			reasons = append(reasons, "パッケージ: "+r)
		}
		if len(pkg.Reasons) == 0 {
			reasons = append(reasons, "パッケージ: 食い違いを確かめられない")
		}
	case *pkg.Drift:
		d = true
		for _, r := range pkg.Reasons {
			reasons = append(reasons, "パッケージ: "+r)
		}
	}
	switch {
	case ws.Commit == nil || ws.Dirty == nil:
		unknown = true
		reasons = append(reasons, "セルの repo の commit を読めない")
	case *ws.Dirty:
		d = true
		reasons = append(reasons, "セルの repo に commit していない変更がある")
	}
	if !d && unknown {
		return nil, reasons
	}
	return &d, reasons
}

// checkout は dir の HEAD の commit と、commit していない変更（git status --porcelain。無視するものを除く）があるか。
func checkout(dir string) Checkout {
	c := Checkout{Path: dir}
	if dir == "" {
		return c
	}
	head, err := spawn.Output(nil, "", "git", "-C", dir, "rev-parse", "--verify", "-q", "HEAD^{commit}")
	if err != nil || len(head) != 40 {
		return c
	}
	c.Commit = &head
	st, err := spawn.Output(nil, "", "git", "-C", dir, "status", "--porcelain")
	if err == nil {
		dirty := st != ""
		c.Dirty = &dirty
	}
	return c
}

// harnessVersion は <brand> --version の1行目。読めなければ null。
func harnessVersion(brand string) *string {
	if brand == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), harnessTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, brand, "--version").Output()
	if err != nil {
		return nil
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strPtr(strings.TrimSpace(line))
}

// claudeDir は Claude Code の置き場（CLAUDE_CONFIG_DIR。無ければ ~/.claude）。
func claudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".claude")
}

// externals は外から入る skill（skill の置き場の実ディレクトリ。symlink は Andon のパッケージか image の備品なので数えない）と、
// Claude Code の plugin（installed_plugins.json）。Claude 系は <claude>/skills と ~/.agents/skills、Codex 系は ~/.agents/skills を見る。
// 置き場が無いのは「無い」（空）。読めなければエラー（記録は null）。
func externals(brand string) ([]External, error) {
	dirs := []string{filepath.Join(os.Getenv("HOME"), ".agents", "skills")}
	if brand == "claude" {
		dirs = append([]string{filepath.Join(claudeDir(), "skills")}, dirs...)
	}
	out := []External{}
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		for _, e := range ents {
			if !e.IsDir() || e.Type()&fs.ModeSymlink != 0 {
				continue
			}
			p := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(filepath.Join(p, skillFile))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, err
			}
			out = append(out, skillOf(e.Name(), p, b))
		}
	}
	if brand == "claude" {
		ps, err := plugins(filepath.Join(claudeDir(), "plugins", installedPlugins))
		if err != nil {
			return nil, err
		}
		out = append(out, ps...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return out, nil
}

// skillOf は SKILL.md の frontmatter の metadata（gh skill install が書く github-repo・github-ref・github-tree-sha・version）を読む。
// 読めない frontmatter は版の無い skill にする。
func skillOf(dirName, path string, b []byte) External {
	x := External{Kind: externalSkill, Name: dirName, Path: path}
	var fm struct {
		Name     string         `yaml:"name"`
		Metadata map[string]any `yaml:"metadata"`
	}
	if y := frontmatter(b); y != nil && yaml.Unmarshal(y, &fm) == nil {
		if fm.Name != "" {
			x.Name = fm.Name
		}
		str := func(k string) *string {
			if v, ok := fm.Metadata[k].(string); ok {
				return strPtr(v)
			}
			return nil
		}
		x.Source, x.Ref, x.Version, x.Revision = str("github-repo"), str("github-ref"), str("version"), str("github-tree-sha")
	}
	return x
}

// frontmatter は先頭の --- と次の --- の間（無ければ nil）。
func frontmatter(b []byte) []byte {
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != frontmatterMarker {
		return nil
	}
	var y bytes.Buffer
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == frontmatterMarker {
			return y.Bytes()
		}
		y.WriteString(sc.Text())
		y.WriteByte('\n')
	}
	return nil
}

// plugins は Claude Code の installed_plugins.json（version 2: plugins.<名前@marketplace>[] に version・gitCommitSha・installPath）。
// ファイルが無ければ空。
func plugins(path string) ([]External, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var f struct {
		Plugins map[string][]struct {
			InstallPath  string `json:"installPath"`
			Version      string `json:"version"`
			GitCommitSha string `json:"gitCommitSha"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	var out []External
	for name, ins := range f.Plugins {
		for _, in := range ins {
			out = append(out, External{Kind: externalPlugin, Name: name, Path: in.InstallPath,
				Version: strPtr(in.Version), Revision: strPtr(in.GitCommitSha)})
		}
	}
	return out, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
