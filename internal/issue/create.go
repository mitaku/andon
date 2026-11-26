package issue

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"andon/internal/contract"
	"andon/internal/ledger"
)

const CreateUsage = "usage: andon issue create \"<タイトル>\" [kanban add の旗…] [<key>=<value>…] [--reason <key>=<理由>]… [--dry-run] [--json]\n" +
	"担当の課題を足し、予約キーを書き、依頼書の ## Settings 欄を予約キーから書く（Chief of Staff が打つ）。\n" +
	"kanban add の旗: --repo <r>・--profile <p>・--parent <id>・--round <k>・--after <id,…>・--phase <id>・--status <s>・--brief <file|->。\n" +
	"予約キー: review・must_fix・reviewer・review_rounds・merge・deliverable・estimate・acceptance（値は contract.md §2）。\n" +
	"予約キーでないキーか知らない値があれば、何も足さずに終了コード 1。\n" +
	"--reason は既定から変えたキーの理由で、Settings 欄の行の ★ に続けて書く。\n" +
	"--dry-run は何も書かずに、足す課題・書くキー・依頼書を出す。--json は出すものを JSON にする。\n" +
	"足したら id と、次に打つ andon issue start <id> を出す。\n"

// createFlags は kanban add に渡す、値を取る旗（kanban add の使い方のとおり）。
var createFlags = []string{"--repo", "--profile", "--parent", "--round", "--after", "--phase", "--status", "--brief"}

// keyWordRe は key=value の形の語（タイトルの書き忘れを見分ける）。
var keyWordRe = regexp.MustCompile(`^[a-z][a-z_]*=`)

// createKeys は andon issue create が受ける予約キー: 依頼書の ## Settings の行になるキー（playbooks/brief.md の表）。
// machine（まだ使えない）・hold・later などの印は受けない（ADR-0053 決めたこと 18。オーナーの課題の印は andon issue ask --new）。
var createKeys = []string{"review", "reviewer", "must_fix", "review_rounds", "merge", "deliverable", "acceptance", "estimate"}

type createPlan struct {
	DryRun    bool     `json:"dry_run,omitempty"`
	ID        string   `json:"id,omitempty"`
	Title     string   `json:"title"`
	Add       []string `json:"add"`  // kanban add に渡す引数
	Keys      []string `json:"keys"` // kanban set に渡す key=value
	BriefPath string   `json:"brief_path,omitempty"`
	Brief     string   `json:"brief"` // Settings 欄を書いたあとの依頼書（依頼書が無ければ空）
	Missing   []string `json:"missing"`
	Next      string   `json:"next"`
	slug      string   // 次の1手の --project に書くプロジェクト
}

func Create(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	refuse := func(rc int, format string, a ...any) int {
		fmt.Fprintf(stderr, "andon issue create: "+format+"\n", a...)
		return rc
	}
	var title string
	var add, keys []string
	var dry, asJSON, haveTitle, words bool
	vals, reasons := map[string]string{}, map[string]string{}
	brief := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case words || !strings.HasPrefix(a, "-") || a == "-":
			if !haveTitle {
				// タイトルを書き忘れると最初の key=value がタイトルに吸われて課題が足される。= を含むタイトルは -- のあとに置く
				if !words && keyWordRe.MatchString(a) {
					return refuse(exitFail, "タイトルが無い（最初の語 %s が key=value の形）。= を含むタイトルなら -- のあとに置く。何も足していない", a)
				}
				title, haveTitle = a, true
				continue
			}
			k, v, ok := strings.Cut(a, "=")
			if !ok || k == "" {
				fmt.Fprint(stderr, CreateUsage)
				return exitUsage
			}
			if !slices.Contains(createKeys, k) {
				return refuse(exitFail, "%s は andon issue create が受ける予約キーでない（受けるのは %s）。何も足していない", k, strings.Join(createKeys, "・"))
			}
			if _, dup := vals[k]; dup {
				return refuse(exitFail, "%s が 2 つある。何も足していない", k)
			}
			vals[k] = v
			keys = append(keys, k+"="+v)
		case a == "--":
			words = true
		case a == "--dry-run" && !dry:
			dry = true
		case a == "--json" && !asJSON:
			asJSON = true
		case a == "--reason" && i+1 < len(args):
			k, why, ok := strings.Cut(args[i+1], "=")
			if !ok || strings.TrimSpace(why) == "" || strings.ContainsAny(why, "\r\n") {
				return refuse(exitUsage, "--reason は <key>=<理由> で、理由は空でない 1 行にする: %q", args[i+1])
			}
			if _, dup := reasons[k]; dup {
				return refuse(exitUsage, "--reason %s が 2 つある", k)
			}
			reasons[k] = why
			i++
		case slices.Contains(createFlags, a) && i+1 < len(args):
			if a == "--brief" {
				brief = args[i+1]
			}
			add = append(add, a, args[i+1])
			i++
		default:
			fmt.Fprint(stderr, CreateUsage)
			return exitUsage
		}
	}
	if !haveTitle || strings.TrimSpace(title) == "" {
		fmt.Fprint(stderr, CreateUsage)
		return exitUsage
	}
	// 予約キーは足す前に schema（issue-keys）で確かめる。知らない値があれば何も足さない（決めたこと 18）
	for _, kv := range keys {
		k, v, _ := strings.Cut(kv, "=")
		if _, err := contract.Encode(contract.SchemaIssueKeys, map[string]string{k: v}); err != nil {
			return refuse(exitFail, "%s は知らない値（%v）。何も足していない", kv, err)
		}
	}
	for k := range reasons {
		v, ok := vals[k]
		switch {
		case !ok:
			return refuse(exitFail, "--reason %s のキーを渡していない。何も足していない", k)
		case !ledger.Marked(ledger.Issue{"profile": profileArg(add)}, k, v):
			return refuse(exitFail, "--reason %s: %s=%s は既定どおりなので、Settings 欄に印も理由も付かない。何も足していない", k, k, v)
		}
	}
	p := &createPlan{DryRun: dry, Title: title, Add: append(append([]string{"add"}, add...), "--", title), Keys: keys}
	if err := p.run(stdin, stderr, brief, reasons); err != nil {
		return refuse(exitFail, "%v", err)
	}
	if asJSON {
		stdout.Write(contract.Marshal(p))
		return 0
	}
	p.print(stdout)
	return 0
}

// profileArg は kanban add に渡す --profile の値（無ければ空）。
func profileArg(add []string) string {
	for i := 0; i+1 < len(add) && add[i] != "--"; i++ {
		if add[i] == "--profile" {
			return add[i+1]
		}
	}
	return ""
}

// run は課題を足し（kanban add）、予約キーを書き（kanban set。書き手は打った端末）、依頼書の Settings 欄を書く。
// --dry-run なら何も書かず、足すはずの課題と、Settings 欄を書いたあとの依頼書を作るだけ。
// kanban add は旗の並べ方をそのまま受けるので、旗の確かめは kanban の CLI に任せる（--dry-run は確かめない）。
func (p *createPlan) run(stdin io.Reader, stderr io.Writer, brief string, reasons map[string]string) error {
	var text string
	if brief != "" {
		b, err := readInput(stdin, brief)
		if err != nil {
			return err
		}
		text = string(b)
	}
	t := ledger.Issue{"profile": profileArg(p.Add)}
	for i := 0; i+1 < len(p.Add) && p.Add[i] != "--"; i++ {
		if p.Add[i] == "--repo" {
			t["repo"] = p.Add[i+1]
		}
	}
	for _, kv := range p.Keys {
		k, v, _ := strings.Cut(kv, "=")
		t[k] = v
	}
	if brief != "" {
		// 空の依頼書に欄だけを書くと、start の「依頼書が無い」の断りを素通りして中身の無い依頼書で起動してしまう
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("依頼書（--brief %s）が空。< brief.md の付け忘れでないか。何も足していない", brief)
		}
		// 欄にだけ書いた値（渡し忘れた予約キー）を、欄の書き直しで黙って捨てない
		if d := ledger.SettingsDropped(text, t); len(d) > 0 && !settingsKept(t.Str("profile")) {
			return fmt.Errorf("依頼書の ## Settings 欄に、渡していない予約キーの値がある（%s）。key=value で渡すか、欄から消す。何も足していない", strings.Join(d, "・"))
		}
	}
	l, err := ledger.Open()
	if err != nil {
		return err
	}
	p.slug = l.Slug
	if p.DryRun {
		if brief != "" && !settingsKept(t.Str("profile")) {
			text, _ = ledger.RewriteSettings(text, t, reasons)
		}
		p.Brief, p.ID = text, "<id>"
		p.next(t, brief != "")
		p.ID = ""
		return nil
	}
	id, err := l.Run(text, p.Add...)
	if err != nil {
		return err
	}
	p.ID = ledger.CanonID(strings.TrimSpace(id))
	if len(p.Keys) > 0 {
		if _, err := l.Run("", append([]string{"set", p.ID}, p.Keys...)...); err != nil {
			return fmt.Errorf("%s を足したが予約キーを書けない: %v。create は打ち直さず（課題が2件になる）、Chief of Staff の端末で kanban set %s %s を打つ", p.ID, err, p.ID, strings.Join(p.Keys, " "))
		}
	}
	t, err = l.Show(p.ID)
	if err != nil {
		return fmt.Errorf("%s を足して予約キーを書いたが、課題を読み直せない: %v。create は打ち直さず（課題が2件になる）、andon issue start %s を打つ", p.ID, err, p.ID)
	}
	p.BriefPath = filepath.Join(l.Dir, "issues", p.ID, "brief.md")
	_, statErr := os.Stat(p.BriefPath)
	hasBrief := statErr == nil
	if hasBrief && !settingsKept(t.Str("profile")) {
		// 欄を書けなくても課題と予約キーは揃っている（start が起動の前に書く）。打ち直して課題を2件にしないよう、断らずに言うだけ
		if p.Brief, err = writeSettings(l, p.BriefPath, t, reasons); err != nil {
			fmt.Fprintf(stderr, "andon issue create: 依頼書 %s の ## Settings 欄を書けない（続ける。andon issue start が起動の前に書く）: %v\n", p.BriefPath, err)
		}
	} else if hasBrief {
		b, _ := os.ReadFile(p.BriefPath)
		p.Brief = string(b)
	}
	p.next(t, hasBrief)
	return nil
}

// next は次に打つ1手（決めたこと 14）: andon issue start が求める予約キーが無ければそれを書く kanban set の行、
// 依頼書が無ければ書く場所、揃っていれば andon issue start <id>。
func (p *createPlan) next(t ledger.Issue, hasBrief bool) {
	keys, optional := requiredSettings(t)
	var args []string
	p.Missing = []string{}
	if t.Str("profile") == "" {
		// profile が無いと andon issue start が断る
		p.Missing = append(p.Missing, "profile")
		args = append(args, "'profile=<名前>'")
	}
	for _, k := range keys {
		if v := t.Str(k); v == "" && !optional[k] {
			p.Missing = append(p.Missing, k)
			args = append(args, "'"+k+"="+settingChoices[k]+"'")
		}
	}
	start := "andon issue start " + p.ID
	if p.slug != "" {
		start += " --project " + p.slug
	}
	switch {
	case len(args) > 0:
		p.Next = fmt.Sprintf("kanban set %s %s を打ってから %s", p.ID, strings.Join(args, " "), start)
	case !hasBrief:
		p.Next = fmt.Sprintf("依頼書 %s を書いてから %s", or(p.BriefPath, "brief.md"), start)
	default:
		p.Next = start
	}
}

func (p *createPlan) print(w io.Writer) {
	if p.DryRun {
		fmt.Fprintf(w, "andon issue create: 足すもの（--dry-run。何も書いていない）: kanban %s\n", strings.Join(p.Add, " "))
		if len(p.Keys) > 0 {
			fmt.Fprintf(w, "andon issue create: 書く予約キー: %s\n", strings.Join(p.Keys, " "))
		}
		if p.Brief != "" {
			fmt.Fprintf(w, "--- 依頼書（Settings 欄を書いたあと）\n%s", p.Brief)
			if !strings.HasSuffix(p.Brief, "\n") {
				fmt.Fprintln(w)
			}
		}
		fmt.Fprintf(w, "andon issue create: 次: %s\n", strings.ReplaceAll(p.Next, "<id>", "<足した id>"))
		return
	}
	fmt.Fprintf(w, "andon issue create: %s（%s）を足した", p.ID, p.Title)
	if len(p.Keys) > 0 {
		fmt.Fprintf(w, "。予約キー %s を書いた", strings.Join(p.Keys, " "))
	}
	if p.BriefPath != "" && p.Brief != "" {
		fmt.Fprintf(w, "。依頼書 %s の ## Settings 欄を予約キーから書いた", p.BriefPath)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "andon issue create: 次: %s\n", p.Next)
}
