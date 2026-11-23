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
	"andon/internal/pkgversion"
	"andon/internal/sessionlog"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// appliedFile は andon retro apply の記録（振り返りの課題の library/）。
const appliedFile = "retro-applied.json"

// appliedLock は andon retro apply を振り返りごとに1本ずつにする lock（振り返りの課題の library/）。
const appliedLock = appliedFile + ".lock"

// Applied は andon retro apply の記録（docs/contract.md §21）。足すたびに書き直し、全部を足したら complete。
type Applied struct {
	Retro     string         `json:"retro"`
	AppliedAt string         `json:"applied_at"`
	Complete  bool           `json:"complete"`
	Issues    []AppliedIssue `json:"issues"`
	Dropped   []Dropped      `json:"dropped"`
}

// AppliedIssue は fix のパターン1つに足した課題。who は worker（直す課題）か person（オーナーの課題）。
type AppliedIssue struct {
	Pattern string `json:"pattern"`
	Issue   string `json:"issue"`
	Who     string `json:"who"`
	Title   string `json:"title"`
}

// Dropped は drop のパターンと理由。
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

// runPatterns は andon retro patterns: 担当の端末から、自分の doing の課題にだけ書ける。
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
	col, err := Collect(Options{Root: root, Now: time.Now(), Days: days, GH: GH,
		ClaudeDir: sessionlog.ClaudeDir(), Package: pkgversion.Home(), Exclude: id})
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
		Patterns: in.Patterns, Fixes: in.Fixes}
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

// plan は fix のパターン1つに足す課題。
type plan struct {
	Pattern string `json:"pattern"`
	Who     string `json:"who"`
	Title   string `json:"title"`
	Repo    string `json:"repo,omitempty"`
	Brief   string `json:"brief"`
}

// runApply は andon retro apply: library/retro.json を書いたときと同じ窓で照らし直し、fix ごとに課題を1件足す。
// 途中で kanban の CLI が落ちても、足した分は記録に残り、打ち直すと残りから続ける（足した課題をもう一度足さない）。
// 同じ振り返りを当てる apply は1本ずつ（library/ の lock）。2本が同時に kanban を照らして同じ課題を足さない。
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
	rec := Applied{Retro: id, Issues: []AppliedIssue{}, Dropped: []Dropped{}}
	recPath := filepath.Join(lib, appliedFile)
	if rb, err := os.ReadFile(recPath); err == nil {
		if err := json.Unmarshal(rb, &rec); err != nil {
			return fmt.Errorf("%s/library/%s が読めない", id, appliedFile)
		}
		if rec.Complete {
			return fmt.Errorf("%s は当て済み（%s/library/%s）", id, id, appliedFile)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
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
		plans = append(plans, planOf(id, x, col, repos))
	}
	if dry {
		out := map[string]any{"retro": id, "issues": plans, "dropped": dropped}
		if plans == nil {
			out["issues"] = []plan{}
		}
		_, err := stdout.Write(contract.Marshal(out))
		return err
	}
	rec.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	rec.Dropped = dropped
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

// addPlans は fix のパターンごとに課題を足し、足すたびに記録を書き直す。記録に無いパターンでも、kanban にこの振り返りを parent に持つ
// 同じ題の課題があれば足さずに記録へ写す（前の回が kanban add のあと記録の保存で落ちたとき、打ち直しで同じ課題を重ねて足さない）。
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
		newID, who, review := "", "", ""
		for _, t := range all {
			tid := ledger.CanonID(t.Str("id"))
			if ledger.CanonID(t.Str("parent")) == parent && t.Str("title") == pl.Title &&
				!slices.ContainsFunc(rec.Issues, func(a AppliedIssue) bool { return a.Issue == tid }) {
				newID, who, review = tid, t.Str("who"), t.Str("review")
				break
			}
		}
		keys := ledger.DefaultSettings("", "off") // 担当の課題の予約キー（profile は書かないので reviewer は codex-review）
		added := newID == ""
		if added {
			args := []string{"add", pl.Title, "--parent", id}
			if pl.Repo != "" {
				args = append(args, "--repo", pl.Repo)
			}
			brief := pl.Brief
			if pl.Who == "worker" {
				brief += "\n" + ledger.SettingsCopy(keys)
			}
			if newID, err = l.Run(brief, append(args, "--brief", "-")...); err != nil {
				return err
			}
		}
		if pl.Who == "person" && who != "person" { // 前の回が add のあと set の前で落ちていれば、ここで付ける
			if _, err := l.Run("", "set", newID, "who=person"); err != nil {
				return err
			}
		}
		// 担当の課題には予約キーを書く（打つのは Chief of Staff の端末なので、andon issue start と gate の履歴の照合が通る）。
		// 前の回が add のあと set の前で落ちていれば、打ち直しで書く。書いてある課題は Chief of Staff が変えたかもしれないので触らない
		if pl.Who == "worker" && (added || review == "") {
			if _, err := l.Run("", append([]string{"set", newID}, keys...)...); err != nil {
				return err
			}
		}
		rec.Issues = append(rec.Issues, AppliedIssue{Pattern: pl.Pattern, Issue: newID, Who: pl.Who, Title: pl.Title})
		if err := save(); err != nil {
			return err
		}
	}
	return nil
}

func list(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return "（" + strings.Join(ids, "・") + "）"
}

// planOf は fix のパターン1つから足す課題を組む。直す先が Andon のパッケージで、その repo をこのプロジェクトが持たなければ
// オーナーの課題（決めたこと 2）。依頼書には要約・根拠・直す先・期待する効果・リスクを入れ、オーナーが打った文の本文は写さない。
func planOf(retro string, x Pattern, in *Input, repos []string) plan {
	pl := plan{Pattern: x.ID, Who: "worker", Title: x.Summary, Repo: x.Target.Repo}
	if x.Target.Kind == "andon" && (x.Target.Repo == "" || !slices.Contains(repos, x.Target.Repo)) {
		pl.Who, pl.Repo = "person", ""
		pl.Title = "Andon へ伝えるか: " + x.Summary
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
		b.WriteString("守ること: 直す先の段と paths の範囲で直す。上の段で止められるなら下の段で直さない。PR は worker skill §3 の「改善の PR」の決まりで出す" +
			"（ラベル retro。本文に根拠の課題番号の一覧・オーナーのコメントやオーナーが打った文の本文・人の名前を書かない）。\n")
	}
	b.WriteString("\n## 根拠（セルの中の記録。PR に写さない）\n")
	var prs []string
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
