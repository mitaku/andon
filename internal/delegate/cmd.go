package delegate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

const (
	owner = "オーナーが打つ（セルの中のどこで打ってもよい）。セルの repo（ANDON_WORKSPACE。既定 /workspace）の projects/<slug>/project.json の delegation を検査して書き、\n" +
		"project.json を andon.toml の owner を author にして commit する（project.json は先にオーナーが書く）。\n" +
		"Chief of Staff・担当・dispatcher の端末（hc のラベル role= か issue= がある）では打てない。\n"
	scopes = "scope: merge(none/A/B/C)・deploy(none/preview/production)・publish(none/internal/public)・\n" +
		"       irreversible(delete-branch,force-push,rewrite-history の , 区切り)・external-requests・profile・package-update(true/false)・\n" +
		"       line-change(none/project/plant)。\n"
	GrantUsage = "usage: andon delegation grant <scope> <value> --evidence <path|text> [--until YYYY-MM-DD] [--project <slug>]\n" +
		"       andon delegation grant merge <A|B|C> --issues <id,…> --evidence <path|text> [--until YYYY-MM-DD] [--project <slug>]\n" +
		"委譲を足す。\n" + scopes +
		"--issues は名指し: 並べた課題（kanban にあり done でないもの）の判定にだけ merge の値を足す（プロジェクト全体の値は変えない）。\n" +
		"       grant merge だけが取る。担当が戻せない（risk.one_way）と申告した課題には効かない。名指しは gate が回る前に出す。\n" + owner
	NarrowUsage = "usage: andon delegation narrow <scope> <value> --evidence <path|text> [--until YYYY-MM-DD] [--project <slug>]\n" +
		"委譲を狭める。今の値より広げない。\n" + scopes + owner
	RevokeUsage = "usage: andon delegation revoke <id> --reason <text> [--project <slug>]\n" +
		"委譲を1件取り消す。\n" + owner
	ShowUsage = "usage: andon delegation show [--json] [--project <slug>]\n" +
		"HEAD の委譲を読み手と同じに読んで出す（読むだけ）。\n"
)

var usages = map[string]string{"grant": GrantUsage, "narrow": NarrowUsage, "revoke": RevokeUsage, "show": ShowUsage}

const (
	exitFail  = 1
	exitUsage = 2
)

func GrantMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("grant", args, stdout, stderr)
}

func NarrowMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("narrow", args, stdout, stderr)
}

func RevokeMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("revoke", args, stdout, stderr)
}

func ShowMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("show", args, stdout, stderr)
}

func run(sub string, args []string, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, usages[sub]); return exitUsage }
	args = append([]string{sub}, args...)
	var pos []string
	opt := map[string]string{}
	asJSON := false
	for rest := args[1:]; len(rest) > 0; {
		switch a := rest[0]; a {
		case "--json":
			asJSON, rest = true, rest[1:]
		case "--evidence", "--until", "--reason", "--project", "--issues":
			if len(rest) < 2 {
				return usage()
			}
			opt[a], rest = rest[1], rest[2:]
		default:
			if strings.HasPrefix(a, "--") {
				return usage()
			}
			pos, rest = append(pos, a), rest[1:]
		}
	}
	ws := Workspace()
	slug, err := project(ws, opt["--project"])
	if err != nil {
		fmt.Fprintf(stderr, "andon delegation: %v\n", err)
		return exitUsage
	}
	today := time.Now().UTC().Format("2006-01-02")
	_, withProject := opt["--project"]
	_, withIssues := opt["--issues"]
	switch {
	case sub == "show" && len(pos) == 0 && (len(opt) == 0 || len(opt) == 1 && withProject):
		r := Read(ws, slug, today)
		if asJSON {
			stdout.Write(contract.Marshal(r))
			return 0
		}
		if !r.Trusted {
			fmt.Fprintf(stdout, "%s: 無委譲（全部オーナー）。%s\n", slug, r.Reason)
			return 0
		}
		fmt.Fprintf(stdout, "%s: 型 %s\n", slug, r.Preset)
		for _, s := range Scopes {
			fmt.Fprintf(stdout, "  %-17s %s\n", s, show(r.Effective[s]))
		}
		for _, g := range r.Grants {
			if len(g.Issues) > 0 {
				fmt.Fprintf(stdout, "  名指し %s %s %s: %s%s\n", g.ID, g.Scope, show(rawValue(g.Value)), strings.Join(g.Issues, ", "), untilNote(g.Until))
			}
		}
		return 0
	case (sub == "grant" || sub == "narrow") && len(pos) == 2 && opt["--evidence"] != "" && opt["--reason"] == "":
		var issues []string
		if withIssues {
			if issues = splitIssues(opt["--issues"]); len(issues) == 0 {
				fmt.Fprintf(stderr, "andon delegation %s: --issues が空\n", sub)
				return exitFail
			}
		}
		kanbanRoot := filepath.Join(ws, "projects", slug)
		err = change(ws, slug, today, stdout, stderr, func(f *File) (string, error) {
			return add(f, sub, pos[0], pos[1], opt["--evidence"], opt["--until"], today, issues, func(id string) (string, error) {
				return issueStatus(kanbanRoot, id)
			})
		})
	case sub == "revoke" && len(pos) == 1 && opt["--reason"] != "" && opt["--evidence"] == "" && opt["--until"] == "" && !withIssues:
		err = change(ws, slug, today, stdout, stderr, func(f *File) (string, error) {
			return revoke(f, pos[0], opt["--reason"], today)
		})
	default:
		return usage()
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon delegation %s: %v\n", sub, err)
		return exitFail
	}
	return 0
}

// project はプロジェクトの slug（ADR-0053 決めたこと 4）: --project、KANBAN_ROOT、cwd が <ws>/projects/<slug>/ の下ならその slug、
// kanban の CLI が今いるフォルダから見つける kanban（repo の載ったプロジェクト・セルに1つだけ）の順。
func project(ws, given string) (string, error) {
	if given != "" {
		if !slugRe.MatchString(given) {
			return "", fmt.Errorf("slug が不正: %q", given)
		}
		return given, nil
	}
	if root := os.Getenv("KANBAN_ROOT"); root != "" {
		if slug := filepath.Base(root); slugRe.MatchString(slug) {
			return slug, nil
		}
	}
	cwd, _ := os.Getwd()
	base, _ := filepath.EvalSymlinks(filepath.Join(ws, "projects"))
	if real, err := filepath.EvalSymlinks(cwd); err == nil && base != "" {
		if rel, err := filepath.Rel(base, real); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			slug := strings.Split(rel, string(filepath.Separator))[0]
			if slugRe.MatchString(slug) {
				return slug, nil
			}
		}
	}
	if l, err := ledger.Open(); err == nil && slugRe.MatchString(l.Slug) {
		return l.Slug, nil
	}
	return "", errors.New("プロジェクトが決まらない（--project <slug> で指す）")
}

// AgentTerminal は打っているのが Chief of Staff・担当・dispatcher の端末か（hc のラベル role= か issue=）。
// 役の端末なら why を添えた誤り。読めなければ打たせない（安全側）。オーナーのファイルを書く command（delegate・create）が使う。
func AgentTerminal(why string) error {
	id := os.Getenv("HC_TERMINAL_ID")
	if id == "" {
		return nil
	}
	out, err := exec.Command("hc", "get", id, "--json").Output()
	if err != nil {
		return fmt.Errorf("端末 %s のラベルが読めない（hc get）。オーナーの端末で打つ", id)
	}
	var t struct {
		Labels map[string]string `json:"labels"`
	}
	if json.Unmarshal(out, &t) != nil {
		return fmt.Errorf("端末 %s のラベルが読めない。オーナーの端末で打つ", id)
	}
	for _, k := range []string{"role", "issue"} {
		if v, ok := t.Labels[k]; ok {
			return fmt.Errorf("この端末（%s=%s）では打てない。%s", k, v, why)
		}
	}
	return nil
}

// change はセルの repo の HEAD の project.json（owner の commit）を読み、delegation を f で変え、検査して書き、project.json だけを commit する。
// project.json の delegation のほかのキーは HEAD のまま（形だけ2字下げに揃う）。project.json が無ければ書かない（先にオーナーが書く）。
// 作業ツリーや index に commit していない変更があれば打たない（エージェントが書いた変更をオーナーの commit に混ぜない）。
// HEAD を最後に変えたのが owner でなければ打たない（owner でない変更をオーナーの commit で信じられる形にしない）。
// commit の author と committer は andon.toml の owner（名前は HEAD の project.json を最後に変えた owner の commit のもの）。
// 端末の git の user は使わない（セルではエージェントの名前で、読み手が無委譲として読む）。
func change(ws, slug, today string, stdout, stderr io.Writer, f func(*File) (string, error)) error {
	if err := AgentTerminal("委譲を書くのはオーナーだけ"); err != nil {
		return err
	}
	rel := ProjectRel(slug)
	path := filepath.Join(ws, rel)
	if st, err := git(ws, "status", "--porcelain", "--", rel); err != nil {
		return err
	} else if strings.TrimSpace(st) != "" {
		return fmt.Errorf("%s に commit していない変更がある（git diff で中身を確かめ、オーナーの変更なら owner の email で commit してから打つ）", rel)
	}
	b, err := OwnerFile(ws, rel)
	if errors.Is(err, ErrAbsent) {
		return fmt.Errorf("セルの repo の HEAD に %s が無い。先にオーナーが書いて commit する（形は docs/contract.md §11）", rel)
	} else if err != nil {
		return fmt.Errorf("%v（中身を確かめ、owner の email で commit し直してから打つ）", err)
	}
	p, err := ParseProject(b, slug)
	if err != nil {
		return fmt.Errorf("%s の形が違う: %v（直してから打つ）", rel, err)
	}
	owner := Owner(ws) // OwnerFile が確かめた（無ければここに来ない）
	name, err := git(ws, "log", "-1", "--format=%an", "HEAD", "--", rel)
	if name = strings.TrimSpace(name); err != nil || name == "" {
		return fmt.Errorf("%s を最後に変えた owner の commit の名前が読めない", rel)
	}
	file := &File{Version: 1, Project: slug, Grants: []Grant{}}
	if p.Delegation != nil {
		if file, err = Parse(p.Delegation, slug); err != nil {
			return err // ParseProject が通したので来ない
		}
	}
	if file.Grants == nil {
		file.Grants = []Grant{}
	}
	msg, err := f(file)
	if err != nil {
		return err
	}
	p.Delegation = json.RawMessage(bytes.TrimSpace(contract.Marshal(file)))
	out, err := contract.Encode(contract.SchemaProject, p)
	if err != nil {
		return fmt.Errorf("書く前の検査に落ちた: %v", err)
	}
	if _, err := ParseProject(out, slug); err != nil {
		return fmt.Errorf("書く前の検査に落ちた: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	if _, err := git(ws, "add", "--", rel); err != nil {
		return err
	}
	env := []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + owner, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + owner}
	if _, err := gitEnv(ws, env, "commit", "-q", "-m", msg, "--", rel); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s: %s\n", rel, strings.SplitN(msg, "\n", 2)[0])
	return nil
}

func nextID(f *File) string {
	n := 0
	for _, g := range f.Grants {
		var k int
		if _, err := fmt.Sscanf(g.ID, "d%d", &k); err == nil && k > n {
			n = k
		}
	}
	return fmt.Sprintf("d%d", n+1)
}

// splitIssues は --issues の , 区切りを課題の id の並びにする（空白と空の要素を除き、kanban の形に揃える。重なりは残す。検査は add）。
func splitIssues(s string) []string {
	var out []string
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, ledger.CanonID(id))
		}
	}
	return out
}

// issueStatus はプロジェクトの kanban（root は kanban/ の親）の課題 id の status。無ければ誤り。
func issueStatus(root, id string) (string, error) {
	t, err := ledger.At(root).Show(id)
	if err != nil {
		return "", err
	}
	return t.Str("status"), nil
}

func rawValue(raw json.RawMessage) any {
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}

func untilNote(u *string) string {
	if u == nil {
		return ""
	}
	return "（期限 " + *u + "）"
}

// add は grant か narrow の1行を f に足す。issues があれば名指しの grant（ADR-0041 決めたこと 6）: grant merge の A・B・C だけが取り、
// 名指しする課題はそのプロジェクトの kanban にあり done でないもの（status が引く）。
func add(f *File, sub, scope, value, evidence, until, today string, issues []string, status func(id string) (string, error)) (string, error) {
	if !slices.Contains(Scopes, scope) {
		return "", fmt.Errorf("知らない scope: %q（%s）", scope, strings.Join(Scopes, " / "))
	}
	raw, err := ParseValue(scope, value)
	if err != nil {
		return "", err
	}
	if issues != nil {
		if sub != "grant" {
			return "", fmt.Errorf("%s は --issues を取らない（名指しは grant merge だけ。狭めるのは課題の merge: person か revoke）", sub)
		}
		if err := checkIssues(scope, raw, issues); err != nil {
			return "", err
		}
		for _, id := range issues {
			st, err := status(id)
			switch {
			case err != nil:
				return "", fmt.Errorf("--issues の %s が kanban に無い（%v）", id, err)
			case st == "done":
				return "", fmt.Errorf("--issues の %s はもう done（名指しは片付いていない課題だけ）", id)
			}
		}
	}
	var u *string
	if until != "" {
		if !Date(until) || until < today {
			return "", fmt.Errorf("--until は今日以後の YYYY-MM-DD: %q", until)
		}
		u = &until
	}
	if sub == "narrow" {
		var v any
		_ = json.Unmarshal(raw, &v)
		cur := f.Effective(today)[scope]
		if !Narrower(scope, v, cur) {
			return "", fmt.Errorf("narrow は広げない: %s は今 %s（広げるなら grant）", scope, show(cur))
		}
	}
	g := Grant{ID: nextID(f), Scope: scope, Value: raw, By: "owner", At: today, Until: u, Evidence: evidence, Issues: issues}
	f.Grants = append(f.Grants, g)
	if issues != nil {
		named := strings.Join(issues, ",")
		return fmt.Sprintf("delegate %s %s: %s %s --issues %s\n\nscope: %s\nvalue: %s\nissues: %s\nevidence: %s\n",
			sub, f.Project, scope, raw, named, scope, raw, named, evidence), nil
	}
	return fmt.Sprintf("delegate %s %s: %s %s\n\nscope: %s\nvalue: %s\nevidence: %s\n", sub, f.Project, scope, raw, scope, raw, evidence), nil
}

func revoke(f *File, id, reason, today string) (string, error) {
	for i := range f.Grants {
		if f.Grants[i].ID != id {
			continue
		}
		if f.Grants[i].Revoked != nil {
			return "", fmt.Errorf("%s はもう取り消してある", id)
		}
		f.Grants[i].Revoked = &Revoked{At: today, By: "owner", Reason: reason}
		g := f.Grants[i]
		return fmt.Sprintf("delegate revoke %s: %s（%s %s）\n\nscope: %s\nvalue: %s\nreason: %s\n", f.Project, id, g.Scope, g.Value, g.Scope, g.Value, reason), nil
	}
	return "", fmt.Errorf("grant %s が無い", id)
}

// ProjectSlug はプロジェクトの slug（given か、cwd が projects/<slug>/ の下ならその slug）。andon signal も使う。
func ProjectSlug(ws, given string) (string, error) { return project(ws, given) }
