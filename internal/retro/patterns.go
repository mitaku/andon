package retro

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// パターンの一覧（ADR-0028 決めたこと 8 の「判定を書く」。docs/contract.md §21・schema/patterns.schema.json）。
// 振り返りの担当が andon retro patterns で書き、Chief of Staff が andon retro apply で当てる。
// 書くときも当てるときも、同じ検査（checkPatterns）を集めた入力に当てる。command が断るのは形と、根拠の数・実在・新しさ、
// 上の段で止められない理由、振り返り自身を直さないこと、判定と fix の上限だけ。パターンが本当に起きたかは確かめない。

// MaxFix は1回の振り返りが直す課題に切ってよい数（ADR-0028 決めたこと 13）。
const MaxFix = 2

// 段（決めたこと 9）。上から効き目の確かな順。
var levels = []string{"design", "test", "check", "skill", "text"}

// levelName は段の日本語の名前（依頼書に書く）。
var levelName = map[string]string{
	"design": "設計・型", "test": "テスト・lint", "check": "決まった手順の確かめ", "skill": "skill・指針", "text": "文面",
}

var (
	actions   = []string{"add", "change", "remove"}
	kinds     = []string{"andon", "project"}
	decisions = []string{"fix", "drop"}
	// actionName は足すか変えるか削るかの日本語。
	actionName = map[string]string{"add": "足す", "change": "変える", "remove": "削る"}
)

// selfPaths は振り返り自身（skill retro と andon retro の command）。直す先にできない（決めたこと 8・やらないこと）。
var selfPaths = []string{"package/skills/retro", "internal/retro"}

var (
	patternIDRe = regexp.MustCompile(`^p[1-9][0-9]*$`)
	repoRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// PatternsInput は担当が andon retro patterns に渡す JSON（command が足す欄は書かない）。
type PatternsInput struct {
	Patterns []Pattern `json:"patterns"`
	Fixes    []FixView `json:"fixes"`
}

// Patterns は library/retro.json（PatternsInput に、command が集めた入力の窓を足したもの）。
type Patterns struct {
	Retro         string     `json:"retro"`
	At            string     `json:"at"`
	Since         string     `json:"since"`
	Until         string     `json:"until"`
	Days          int        `json:"days"`
	PreviousRetro *PrevRetro `json:"previous_retro" andon:"nullable"`
	Patterns      []Pattern  `json:"patterns"`
	Fixes         []FixView  `json:"fixes"`
}

// Pattern はパターン1つ。
type Pattern struct {
	ID          string   `json:"id"`
	Summary     string   `json:"summary"`
	Evidence    []string `json:"evidence"`
	Level       string   `json:"level"`
	Action      string   `json:"action"`
	Target      Target   `json:"target"`
	NotUpstream string   `json:"not_upstream,omitempty"`
	Effect      string   `json:"effect"`
	Risk        string   `json:"risk"`
	Decision    string   `json:"decision"`
	Reason      string   `json:"reason"`
}

// Target は直す先。kind は andon（Andon のパッケージ）か project（案件の repo のテスト・AGENTS.md・REVIEW.md など）。
type Target struct {
	Kind  string   `json:"kind"`
	Repo  string   `json:"repo,omitempty"`
	Paths []string `json:"paths"`
}

// FixView は前の振り返りが切った直し1件の再発の数と見立て（決めたこと 6）。
type FixView struct {
	Issue    string `json:"issue"`
	Recurred *int   `json:"recurred"`
	View     string `json:"view"`
}

// line は空白でない字を持ち、改行を含まない文字列か。
func line(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0 && !strings.ContainsAny(s, "\r\n")
}

// checkShape は入力の形を確かめる（集めた入力が要らない所）。
func checkShape(ps []Pattern, fixes []FixView) error {
	ids := map[string]bool{}
	for i, p := range ps {
		at := fmt.Sprintf("patterns[%d]", i)
		if !patternIDRe.MatchString(p.ID) || ids[p.ID] {
			return fmt.Errorf("%s の id %q が p1・p2 … の形でないか、重なっている", at, p.ID)
		}
		ids[p.ID] = true
		at = "パターン " + p.ID
		for _, f := range []struct{ name, v string }{{"summary", p.Summary}, {"effect", p.Effect}, {"risk", p.Risk}, {"reason", p.Reason}} {
			if !line(f.v) {
				return fmt.Errorf("%s の %s が空か、改行を含む", at, f.name)
			}
		}
		if strings.HasPrefix(p.Summary, "-") {
			return fmt.Errorf("%s の summary が - で始まる（直す課題の題になる）", at)
		}
		if !slices.Contains(levels, p.Level) {
			return fmt.Errorf("%s の level %q は %s のどれでもない", at, p.Level, strings.Join(levels, "・"))
		}
		if !slices.Contains(actions, p.Action) {
			return fmt.Errorf("%s の action %q は %s のどれでもない", at, p.Action, strings.Join(actions, "・"))
		}
		if !slices.Contains(decisions, p.Decision) {
			return fmt.Errorf("%s に判定が無い（decision は fix か drop）", at)
		}
		if (p.Level == "skill" || p.Level == "text") && !line(p.NotUpstream) {
			return fmt.Errorf("%s は直す先が %s なのに、上の段で止められない理由（not_upstream）が無い", at, levelName[p.Level])
		}
		if p.NotUpstream != "" && !line(p.NotUpstream) {
			return fmt.Errorf("%s の not_upstream が改行を含む", at)
		}
		if !slices.Contains(kinds, p.Target.Kind) {
			return fmt.Errorf("%s の target.kind %q は andon か project でない", at, p.Target.Kind)
		}
		if p.Target.Repo != "" && !repoRe.MatchString(p.Target.Repo) {
			return fmt.Errorf("%s の target.repo %q が <host>/<owner>/<repo> の形でない", at, p.Target.Repo)
		}
		if p.Target.Kind == "project" && p.Target.Repo == "" {
			return fmt.Errorf("%s は target.kind が project なのに target.repo が無い", at)
		}
		if len(p.Target.Paths) == 0 {
			return fmt.Errorf("%s の target.paths が空", at)
		}
		for _, x := range p.Target.Paths {
			c := path.Clean(x)
			if x == "" || path.IsAbs(x) || c == ".." || strings.HasPrefix(c, "../") || strings.ContainsAny(x, "\r\n") {
				return fmt.Errorf("%s の target.paths の %q が repo の中の相対パスでない", at, x)
			}
			for _, s := range selfPaths {
				if c == s || strings.HasPrefix(c, s+"/") {
					return fmt.Errorf("%s の直す先 %s は振り返り自身（skill retro と andon retro）。振り返りは自分を直さない", at, x)
				}
			}
		}
		if len(p.Evidence) == 0 {
			return fmt.Errorf("%s に根拠が無い", at)
		}
	}
	n := 0
	for _, p := range ps {
		if p.Decision == "fix" {
			n++
		}
	}
	if n > MaxFix {
		return fmt.Errorf("fix が %d 件ある。1回の振り返りで直す課題に切るのは %d 件まで（残りは drop にして理由を書く）", n, MaxFix)
	}
	seen := map[string]bool{}
	for i, f := range fixes {
		if f.Issue == "" || seen[f.Issue] {
			return fmt.Errorf("fixes[%d] の issue が空か、重なっている", i)
		}
		seen[f.Issue] = true
		if f.Recurred == nil || *f.Recurred < 0 {
			return fmt.Errorf("前の直し %s の recurred（再発の数）が無いか負", f.Issue)
		}
		if !line(f.View) {
			return fmt.Errorf("前の直し %s の view（見立て）が空か、改行を含む", f.Issue)
		}
	}
	return nil
}

// evidenceUnit は根拠1件を数える単位（決めたこと 14 の 2）。課題は課題ごと（レビュアーの子は親）、人が打った文は
// セッションのオーナーの課題ごと（Chief of Staff は課題を持たないのでセッションごと）。同じ単位の根拠は1件と数える。
func evidenceUnit(id string, in *Input) (unit string, at time.Time, ok bool) {
	for _, b := range in.Issues {
		if b.ID == id {
			return "issue:" + b.ID, mustTime(b.DoneAt), true
		}
	}
	for _, t := range in.Typed {
		if t.ID != id {
			continue
		}
		if t.Issue != nil && *t.Issue != "" {
			owner := *t.Issue
			if parent, _, round := ledger.SplitRound(owner); round {
				owner = parent
			}
			return "issue:" + owner, mustTime(t.At), true
		}
		return "session:" + t.Session, mustTime(t.At), true
	}
	return "", time.Time{}, false
}

// checkEvidence は根拠を集めた入力に照らす: 実在・別々の2件以上・前の振り返りより後のものが1件以上。
func checkEvidence(ps []Pattern, in *Input) error {
	var prev time.Time
	if in.PreviousRetro != nil {
		prev = mustTime(in.PreviousRetro.DoneAt)
	}
	for _, p := range ps {
		units := map[string]bool{}
		newer := false
		for _, id := range p.Evidence {
			unit, at, ok := evidenceUnit(id, in)
			if !ok {
				return fmt.Errorf("パターン %s の根拠 %q が集めた入力（期間 %s〜%s の課題と人の文）に無い", p.ID, id, in.Since, in.Until)
			}
			units[unit] = true
			if in.PreviousRetro == nil || at.After(prev) {
				newer = true
			}
		}
		if len(units) < 2 {
			return fmt.Errorf("パターン %s の根拠が別々の2件に満たない（%d 件。同じ課題の中・同じセッションの中の2回は1件と数える）。1回だけの出来事から直さない", p.ID, len(units))
		}
		if !newer {
			return fmt.Errorf("パターン %s の根拠がどれも前の振り返り %s（%s に片付いた）より前。新しい出来事が1件以上要る", p.ID, in.PreviousRetro.Issue, in.PreviousRetro.DoneAt)
		}
	}
	return nil
}

// checkFixes は前の直しの見立てが、集めた入力の前の直しと過不足なく合うかを確かめる。
func checkFixes(fixes []FixView, in *Input) error {
	want := map[string]bool{}
	for _, f := range in.Fixes {
		want[f.Issue] = true
	}
	for _, f := range fixes {
		if !want[f.Issue] {
			return fmt.Errorf("前の直し %s が集めた入力の fixes に無い", f.Issue)
		}
		delete(want, f.Issue)
	}
	if len(want) > 0 {
		var miss []string
		for id := range want {
			miss = append(miss, id)
		}
		slices.Sort(miss)
		return fmt.Errorf("前の直し %s の再発の数と見立て（fixes）が無い", strings.Join(miss, "・"))
	}
	return nil
}

// checkRepos は直す先の repo を台帳の repos に照らす。project の repo は台帳の repos の中でなければならない。
func checkRepos(ps []Pattern, repos []string) error {
	for _, p := range ps {
		if p.Target.Kind == "project" && !slices.Contains(repos, p.Target.Repo) {
			return fmt.Errorf("パターン %s の target.repo %s がこのプロジェクトの repos に無い", p.ID, p.Target.Repo)
		}
	}
	return nil
}

// ParsePatternsInput は担当の入力を読んで形を確かめる。
func ParsePatternsInput(b []byte) (*PatternsInput, error) {
	var p PatternsInput
	if err := contract.Decode(b, &p); err != nil {
		return nil, err
	}
	if p.Patterns == nil || p.Fixes == nil {
		return nil, errors.New("patterns と fixes は必須（無ければ空の配列）")
	}
	if err := checkShape(p.Patterns, p.Fixes); err != nil {
		return nil, err
	}
	return &p, nil
}

// ParsePatterns は library/retro.json を読んで形を確かめる（集めた入力には照らさない）。
func ParsePatterns(b []byte) (*Patterns, error) {
	var p Patterns
	if err := contract.Decode(b, &p); err != nil {
		return nil, err
	}
	if p.Patterns == nil || p.Fixes == nil {
		return nil, errors.New("patterns と fixes は必須（無ければ空の配列）")
	}
	if p.Retro == "" {
		return nil, errors.New("retro が無い")
	}
	since, err1 := time.Parse(time.RFC3339, p.Since)
	until, err2 := time.Parse(time.RFC3339, p.Until)
	if _, err := time.Parse(time.RFC3339, p.At); err != nil || err1 != nil || err2 != nil {
		return nil, errors.New("at・since・until が時刻でない")
	}
	if p.Days < 1 || p.Days > maxDays || !until.Add(-time.Duration(p.Days)*24*time.Hour).Equal(since) {
		return nil, errors.New("days が 1〜90 でないか、since と until の間と合わない")
	}
	if err := checkShape(p.Patterns, p.Fixes); err != nil {
		return nil, err
	}
	return &p, nil
}

// CheckPatterns は集めた入力 in に照らして、パターンの一覧を全部確かめる（書くときと当てるとき）。
// withFixes が真なら、前の直しの見立ても入力の前の直しに照らす（当てるときは GitHub を読まないので照らさない）。
func CheckPatterns(ps []Pattern, fixes []FixView, in *Input, repos []string, withFixes bool) error {
	if err := checkShape(ps, fixes); err != nil {
		return err
	}
	if err := checkRepos(ps, repos); err != nil {
		return err
	}
	if err := checkEvidence(ps, in); err != nil {
		return err
	}
	if withFixes {
		return checkFixes(fixes, in)
	}
	return nil
}
