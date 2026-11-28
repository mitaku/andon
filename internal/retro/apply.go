package retro

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/flock"
	"andon/internal/ledger"
	"andon/internal/otel"
	"andon/internal/pkgversion"
	"andon/internal/sessionlog"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

const appliedFile = "retro-applied.json"

const appliedLock = appliedFile + ".lock"

// Applied は andon retro apply の記録（docs/contract.md §21）。足すたびに書き直し、全部を足したら complete。
type Applied struct {
	Retro     string         `json:"retro"`
	AppliedAt string         `json:"applied_at"`
	Complete  bool           `json:"complete"`
	Issues    []AppliedIssue `json:"issues"`
	Dropped   []Dropped      `json:"dropped"`
}

type AppliedIssue struct {
	Pattern string `json:"pattern"`
	Issue   string `json:"issue"`
	Who     string `json:"who"`
	Title   string `json:"title"`
}

type Dropped struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

func readInput(stdin io.Reader, src string) ([]byte, error) {
	if src == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(src)
}

// patternsNow は runPatterns が集める期間の終わりにする時刻。検査は決まった時刻に差し替える（今の時刻で集めると、
// 決まった時刻で作った記録が日が経つと期間の外に出る）
var patternsNow = time.Now

func runPatterns(root, id string, days int, src string, stdin io.Reader, stdout io.Writer) error {
	b, err := readInput(stdin, src)
	if err != nil {
		return fmt.Errorf("入力が読めない: %v", err)
	}
	l := ledger.At(root)
	iss, err := l.Show(id)
	if err != nil {
		return err
	}
	if me, agent := os.Getenv("HC_TERMINAL_ID"), iss.Str("agent"); me == "" || me != agent {
		return fmt.Errorf("%s の担当（agent: %q）の端末でない（HC_TERMINAL_ID: %q）。パターンの一覧は自分の課題にだけ書く", id, agent, me)
	}
	if s := iss.Str("status"); s != "doing" {
		return fmt.Errorf("%s は %s。パターンの一覧は doing の課題にだけ書く", id, s)
	}
	in, err := ParsePatternsInput(b)
	if err != nil {
		return fmt.Errorf("パターンの一覧の形が違う: %v", err)
	}
	col, err := Collect(Options{Root: root, Now: patternsNow(), Days: days, GH: GH,
		ClaudeDir: sessionlog.ClaudeDir(), Package: pkgversion.Home(), Exclude: id, Otel: filepath.Join(otel.Root(), filepath.Base(root))})
	if err != nil {
		return err
	}
	if err := CheckPatterns(in.Patterns, in.Fixes, col, l.Repos(), true); err != nil {
		return err
	}
	var prev *PrevRetro
	if col.PreviousRetro != nil {
		prev = &PrevRetro{Issue: col.PreviousRetro.Issue, DoneAt: col.PreviousRetro.DoneAt}
	}
	p := Patterns{Retro: id, At: col.At, Since: col.Since, Until: col.Until, Days: days, PreviousRetro: prev,
		Patterns: in.Patterns, Fixes: in.Fixes, ChiefOfStaff: &col.ChiefOfStaff}
	if err := os.MkdirAll(l.Library(id), 0o755); err != nil {
		return err
	}
	if err := contract.Write(filepath.Join(l.Library(id), retroFile), contract.SchemaPatterns, p); err != nil {
		return err
	}
	fix := 0
	for _, x := range p.Patterns {
		if x.Decision == "fix" {
			fix++
		}
	}
	fmt.Fprintf(stdout, "%s/library/%s written（パターン %d 件、fix %d 件・drop %d 件、前の直し %d 件）\n",
		id, retroFile, len(p.Patterns), fix, len(p.Patterns)-fix, len(p.Fixes))
	return nil
}

type plan struct {
	Pattern string `json:"pattern"`
	Who     string `json:"who"`
	Title   string `json:"title"`
	Repo    string `json:"repo,omitempty"`
	Brief   string `json:"brief"`
	// Question はオーナーの課題（who: person）に置く問い（contract.md §5）。担当の課題には無い。
	Question *contract.Question `json:"question,omitempty"`
}

func runApply(root, id string, dry bool, stdout io.Writer) error {
	l := ledger.At(root)
	lib := l.Library(id)
	if !dry {
		unlock, err := flock.TryLock(filepath.Join(lib, appliedLock))
		if errors.Is(err, flock.ErrBusy) {
			return fmt.Errorf("%s はほかの andon retro apply が当てている。終わってから打ち直す（当て済みなら断られる）", id)
		}
		if err != nil {
			return err
		}
		defer unlock()
	}
	b, err := os.ReadFile(filepath.Join(lib, retroFile))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s/library/%s が無い（振り返りの担当が andon retro patterns で書く）", id, retroFile)
	}
	if err != nil {
		return err
	}
	p, err := ParsePatterns(b)
	if err != nil {
		return fmt.Errorf("%s/library/%s の形が違う: %v", id, retroFile, err)
	}
	if p.Retro != id {
		return fmt.Errorf("%s/library/%s の retro が %q（別の課題の一覧）", id, retroFile, p.Retro)
	}
	rec, err := loadApplied(l, id)
	if err != nil {
		return err
	}
	until := mustTime(p.Until)
	col, err := Collect(Options{Root: root, Now: until, Days: p.Days, ClaudeDir: sessionlog.ClaudeDir(), Exclude: id})
	if err != nil {
		return err
	}
	repos := l.Repos()
	if err := CheckPatterns(p.Patterns, p.Fixes, col, repos, false); err != nil {
		return err
	}
	var plans []plan
	dropped := []Dropped{}
	for _, x := range p.Patterns {
		if x.Decision == "drop" {
			dropped = append(dropped, Dropped{Pattern: x.ID, Reason: x.Reason})
			continue
		}
		plans = append(plans, planOf(id, x, col, repos, l.Dir))
	}
	if dry {
		out := map[string]any{"retro": id, "issues": plans, "dropped": dropped}
		if plans == nil {
			out["issues"] = []plan{}
		}
		_, err := stdout.Write(contract.Marshal(out))
		return err
	}
	return applyPlans(l, id, plans, dropped, rec, stdout)
}

func loadApplied(l *ledger.Ledger, id string) (Applied, error) {
	rec := Applied{Retro: id, Issues: []AppliedIssue{}, Dropped: []Dropped{}}
	if rb, err := os.ReadFile(filepath.Join(l.Library(id), appliedFile)); err == nil {
		if err := json.Unmarshal(rb, &rec); err != nil {
			return rec, fmt.Errorf("%s/library/%s が読めない", id, appliedFile)
		}
		if rec.Complete {
			return rec, fmt.Errorf("%s は当て済み（%s/library/%s）", id, id, appliedFile)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return rec, err
	}
	return rec, nil
}

func applyPlans(l *ledger.Ledger, id string, plans []plan, dropped []Dropped, rec Applied, stdout io.Writer) error {
	// オーナーの課題には置いておく印を書く。印を信じるのは Chief of Staff の端末が書いたときだけなので、
	// まだ足していないオーナーの課題があれば、打った端末を確かめてから何かを書く（ADR-0062 決めたこと 10）。
	if slices.ContainsFunc(plans, func(pl plan) bool {
		return pl.Who == "person" && !slices.ContainsFunc(rec.Issues, func(a AppliedIssue) bool { return a.Pattern == pl.Pattern })
	}) {
		if cos, me := l.ChiefOfStaff(), os.Getenv("HC_TERMINAL_ID"); cos == "" || me != cos {
			return fmt.Errorf("オーナーの課題に置いておく印を書くのは Chief of Staff の端末だけ（config.yaml の chief_of_staff: %q・HC_TERMINAL_ID: %q）", cos, me)
		}
	}
	rec.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	rec.Dropped = dropped
	recPath := filepath.Join(l.Library(id), appliedFile)
	save := func() error { return contract.WriteState(recPath, rec) } // retro-applied.json は schema の無い記録
	if err := save(); err != nil {
		return err
	}
	if err := addPlans(l, id, plans, &rec, save); err != nil {
		return err
	}
	rec.Complete = true
	if err := save(); err != nil {
		return err
	}
	var fixes, persons []string
	for _, a := range rec.Issues {
		if a.Who == "person" {
			persons = append(persons, a.Issue)
		} else {
			fixes = append(fixes, a.Issue)
		}
	}
	fmt.Fprintf(stdout, "振り返り %s を当てた: 直す課題 %d 件%s・オーナーの課題 %d 件%s・捨てた %d 件。記録は %s/library/%s\n",
		id, len(fixes), list(fixes), len(persons), list(persons), len(rec.Dropped), id, appliedFile)
	return nil
}

func addPlans(l *ledger.Ledger, id string, plans []plan, rec *Applied, save func() error) error {
	var left []plan
	for _, pl := range plans {
		if !slices.ContainsFunc(rec.Issues, func(a AppliedIssue) bool { return a.Pattern == pl.Pattern }) {
			left = append(left, pl)
		}
	}
	if len(left) == 0 {
		return nil
	}
	all, err := l.List()
	if err != nil {
		return err
	}
	parent := ledger.CanonID(id)
	for _, pl := range left {
		newID := ""
		var found ledger.Issue
		for _, t := range all {
			tid := ledger.CanonID(t.Str("id"))
			if ledger.CanonID(t.Str("parent")) == parent && t.Str("title") == pl.Title &&
				!slices.ContainsFunc(rec.Issues, func(a AppliedIssue) bool { return a.Issue == tid }) {
				newID, found = tid, t
				break
			}
		}
		keys := ledger.DefaultSettings("", "off")
		added := newID == ""
		if pl.Who == "person" {
			// オーナーの課題は andon issue ask --new と同じ関数で、課題と問いを置く（ADR-0057 決めたこと 11）。
			if newID, err = addOwner(l, id, found, pl); err != nil {
				return err
			}
		} else if added {
			args := []string{"add", pl.Title, "--parent", id}
			if pl.Repo != "" {
				args = append(args, "--repo", pl.Repo)
			}
			if newID, err = l.Run(pl.Brief+"\n"+ledger.SettingsCopy(keys), append(args, "--brief", "-")...); err != nil {
				return err
			}
		}
		if pl.Who == "worker" {
			if !added {
				keys = l.Unsettled(l.Log(), found, keys)
			}
			if len(keys) > 0 {
				if _, err := l.Run("", append([]string{"set", newID}, keys...)...); err != nil {
					return err
				}
			}
		}
		rec.Issues = append(rec.Issues, AppliedIssue{Pattern: pl.Pattern, Issue: newID, Who: pl.Who, Title: pl.Title})
		if err := save(); err != nil {
			return err
		}
	}
	return nil
}

// laterReason は「Andon へ伝えるか」の課題に書く置いておく印の理由（ADR-0062 決めたこと 10）。
const laterReason = "振り返りの直しの提案で、答えを待って止まる仕事が無い"

// addOwner は「Andon へ伝えるか」の課題を、置いておく印 later を書いて足す。打ち直しで見つけた課題（found）に who: person か
// 印が無ければ（前の回が課題を足したあと、書く前に落ちた。印を書かない前のバージョンが who だけ書いて落ちた）、who と印を同じ kanban set で書く。
func addOwner(l *ledger.Ledger, retro string, found ledger.Issue, pl plan) (string, error) {
	id := ""
	if found != nil {
		id = ledger.CanonID(found.Str("id"))
	}
	if id == "" {
		newID, _, err := l.NewOwnerIssue(pl.Question, pl.Title, pl.Brief, retro, nil, ledger.Mark{Later: laterReason})
		return newID, err
	}
	if found.Str("who") != "person" || found.Str("later") == "" {
		if _, err := l.Run("", "set", id, "who=person", "later="+laterReason); err != nil {
			return "", err
		}
	}
	n, err := l.Questions(id)
	if err != nil {
		return "", err
	}
	if n == 0 {
		if _, err := l.Ask(id, pl.Question); err != nil {
			return "", err
		}
		return id, nil
	}
	if _, err := l.RestoreQuestion(id, pl.Question); err != nil {
		return "", err
	}
	return id, nil
}

const (
	tellLabel = "Andon のメンテナーへ伝える"
	dropLabel = "捨てる"
)

// ownerQuestion は「Andon へ伝えるか」の課題に置く問い（contract.md §5）。外のメンテナーへの頼みごとなので種類は promise。
// パターンは別々の出来事から拾われて command が数を確かめたものなので、推奨は伝える側（ADR-0057 決めたこと 11）。
func ownerQuestion(retro string, x Pattern) *contract.Question {
	pick := true
	return &contract.Question{
		Kind:     "promise",
		Question: "Andon へ伝えるか: " + x.Summary,
		Facts:    []string{fmt.Sprintf("%s/library/%s のパターン %s", retro, retroFile, x.ID)},
		Reason: "振り返りが別々の出来事から拾い、command が根拠の数を確かめたパターン。直す先は Andon のパッケージ（" +
			strings.Join(x.Target.Paths, "・") + "）だが、このプロジェクトは Andon の repo を持たないので直せない",
		Options: []contract.Option{{Label: tellLabel, Pick: &pick}, {Label: dropLabel}},
		From:    "retro",
	}
}

func list(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "（" + strings.Join(ids, "・") + "）"
}

// planOf は fix のパターン1つから足す課題を組む。直す先が Andon のパッケージで、その repo をこのプロジェクトが持たなければ
// オーナーの課題（決めたこと 2）。依頼書には要約・根拠・直す先・期待する効果・リスクを入れ、オーナーが打った文の本文は写さない。
func planOf(retro string, x Pattern, in *Input, repos []string, dir string) plan {
	pl := plan{Pattern: x.ID, Who: "worker", Title: x.Summary, Repo: x.Target.Repo}
	if toOwner(x.Target, repos) {
		pl.Who, pl.Repo = "person", ""
		pl.Title = "Andon へ伝えるか: " + x.Summary
		pl.Question = ownerQuestion(retro, x)
	}
	where := x.Target.Repo
	if x.Target.Kind == "andon" {
		where = "Andon のパッケージ"
		if x.Target.Repo != "" {
			where += "（" + x.Target.Repo + "）"
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", pl.Title)
	if pl.Who == "person" {
		fmt.Fprintf(&b, "解決すべき問題: 振り返り %s が拾ったパターン %s の直す先は Andon のパッケージだが、このプロジェクトは Andon の repo を持たないので直せない。"+
			"Andon のメンテナーへ伝えるか、捨てるかをオーナーが決める。\n", retro, x.ID)
		fmt.Fprintf(&b, "推奨案: %s（ほかの案: %s）。問いの正は library/questions/ にある。\n", tellLabel, dropLabel)
	} else {
		fmt.Fprintf(&b, "解決すべき問題: %s。振り返り %s がパターン %s として、別々の出来事から拾った（根拠は下）。\n", x.Summary, retro, x.ID)
	}
	fmt.Fprintf(&b, "なぜ: 同じことが繰り返し起きている。1回だけの出来事ではない（根拠の数は command が確かめた）。\n")
	fmt.Fprintf(&b, "直す先: %s の段で%s。%s の %s。\n", levelName[x.Level], actionName[x.Action], where, strings.Join(x.Target.Paths, "・"))
	if x.NotUpstream != "" {
		fmt.Fprintf(&b, "上の段で止められない理由: %s\n", x.NotUpstream)
	}
	fmt.Fprintf(&b, "期待する効果: %s\n", x.Effect)
	fmt.Fprintf(&b, "リスク: %s\n", x.Risk)
	if pl.Who == "worker" {
		b.WriteString("完了条件: 期待する効果が、直す先の段で確かめられる形で入る。段がテストか確かめなら、根拠の出来事を写した入力で直す前に落ち、直したあと通る。\n")
		b.WriteString("守ること: 直す先の段と paths の範囲で直す。paths の文を字面で確かめる検査と、paths の文をそのまま写した所（docs など。docs/adr/ と docs/decisions/ の記録は除く）は、" +
			"paths に無くても同じ PR で直してよい。それより外は直さず、報告の ## Next に1行書く。上の段で止められるなら下の段で直さない。PR は worker skill §3 の「改善の PR」の決まりで出す" +
			"（ラベル retro。本文に根拠の課題番号の一覧・オーナーのコメントやオーナーが打った文の本文・人の名前を書かない）。\n")
	}
	b.WriteString("\n## 根拠（セルの中の記録。PR に写さない）\n")
	var prs []string
	rounds := reviewRounds(dir)
	for _, ev := range x.Evidence {
		if i := slices.IndexFunc(in.Issues, func(bd Bundle) bool { return bd.ID == ev }); i >= 0 {
			bd := in.Issues[i]
			pr := "PR 無し"
			if bd.PR != nil {
				pr = "PR " + *bd.PR
				if !slices.Contains(prs, *bd.PR) {
					prs = append(prs, *bd.PR)
				}
			}
			fmt.Fprintf(&b, "- %s: %s（課題。%s に片付いた。%s）\n", bd.ID, bd.Title, bd.DoneAt, pr)
			var known []string
			for _, rv := range bd.Reviews {
				known = append(known, rv.ID)
			}
			writePaths(&b, evidencePaths(dir, bd.ID, append(known, rounds[bd.ID]...)))
			continue
		}
		if i := slices.IndexFunc(in.Summaries, func(sm Summary) bool { return sm.ID == ev }); i >= 0 {
			sm := in.Summaries[i]
			pr := "PR 無し"
			if sm.PR != nil {
				pr = "PR " + *sm.PR
				if !slices.Contains(prs, *sm.PR) {
					prs = append(prs, *sm.PR)
				}
			}
			fmt.Fprintf(&b, "- %s: %s（課題。%s に片付いた。%s）\n", sm.ID, sm.Title, sm.DoneAt, pr)
			writePaths(&b, evidencePaths(dir, sm.ID, rounds[sm.ID]))
			continue
		}
		if i := slices.IndexFunc(in.Typed, func(t Typed) bool { return t.ID == ev }); i >= 0 {
			t := in.Typed[i]
			who := t.Role
			if t.Issue != nil {
				who += "・" + *t.Issue
			}
			fmt.Fprintf(&b, "- %s: オーナーが打った文（%s、%s）。本文は振り返りの入力にだけある\n", t.ID, who, t.At)
		}
	}
	if len(prs) == 0 {
		b.WriteString("PR の本文に書いてよい根拠の PR: 無い\n")
	} else {
		b.WriteString("PR の本文に書いてよい根拠の PR: " + strings.Join(prs, " ") + "\n")
	}
	fmt.Fprintf(&b, "辿り方: 振り返りの課題 %s の library/%s のパターン %s。\n", retro, retroFile, x.ID)
	pl.Brief = b.String()
	return pl
}

func reviewRounds(dir string) map[string][]string {
	out := map[string][]string{}
	if dir == "" {
		return out
	}
	ents, err := os.ReadDir(filepath.Join(dir, "issues"))
	if err != nil {
		return out
	}
	round := map[string]int{}
	for _, e := range ents {
		parent, k, ok := ledger.SplitRound(e.Name())
		if !ok || !e.IsDir() {
			continue
		}
		round[e.Name()] = k
		out[parent] = append(out[parent], e.Name())
	}
	for _, ids := range out {
		slices.SortFunc(ids, func(a, b string) int { return round[a] - round[b] })
	}
	return out
}

func evidencePaths(dir, id string, reviews []string) []string {
	if dir == "" {
		return nil
	}
	if a, err := filepath.Abs(dir); err == nil {
		dir = a
	}
	issues := filepath.Join(dir, "issues")
	want := []string{filepath.Join(issues, id, "report.md"), filepath.Join(issues, id, "library", journalFile)}
	var seen []string
	for _, rid := range reviews {
		if slices.Contains(seen, rid) {
			continue
		}
		seen = append(seen, rid)
		want = append(want, filepath.Join(issues, rid, "report.md"), filepath.Join(issues, rid, "library", "findings.json"))
	}
	var out []string
	for _, p := range want {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

func writePaths(b *strings.Builder, paths []string) {
	if len(paths) > 0 {
		b.WriteString("  中身の置き場（本文は写さない）: " + strings.Join(paths, " ") + "\n")
	}
}
