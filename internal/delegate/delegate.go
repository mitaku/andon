// Package delegate は委譲（docs/contract.md §11・docs/design.md §1.10）。セルの repo の projects/<slug>/project.json の
// delegation を読み（Read。gate・dispatcher・command が使う）、オーナーが書く手順を助ける（andon delegation grant/narrow/revoke/show）。
//
// 読み方: セルの repo の HEAD のファイルだけを読み、それを最後に変えた commit の author が andon.toml の owner と
// 一致するときだけ信じる。無い・読めない・形が違う・一致しない は無委譲（全部オーナー）。
package delegate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/spawn"
)

var Scopes = []string{"merge", "deploy", "publish", "irreversible", "external-requests", "profile", "package-update", "line-change"}

// levels は順序のある範囲の値（狭い順）。
var levels = map[string][]string{
	"merge":       {"none", "A", "B", "C"},
	"deploy":      {"none", "preview", "production"},
	"publish":     {"none", "internal", "public"},
	"line-change": {"none", "project", "plant"},
}

var irreversibleOps = []string{"delete-branch", "force-push", "rewrite-history"}

// Presets は既定の型（design.md §1.10 の表）。
var Presets = map[string]map[string]any{
	"hobby": {"merge": "C", "deploy": "production", "publish": "public", "irreversible": []string{"delete-branch", "force-push"},
		"external-requests": true, "profile": true, "package-update": true, "line-change": "plant"},
	"work": {"merge": "B", "deploy": "none", "publish": "none", "irreversible": []string{},
		"external-requests": false, "profile": true, "package-update": true, "line-change": "project"},
	"andon": {"merge": "B", "deploy": "none", "publish": "none", "irreversible": []string{},
		"external-requests": false, "profile": true, "package-update": true, "line-change": "none"},
}

// None は無委譲（全部オーナー）の値。
var None = map[string]any{"merge": "none", "deploy": "none", "publish": "none", "irreversible": []string{},
	"external-requests": false, "profile": false, "package-update": false, "line-change": "none"}

type Revoked struct {
	At     string `json:"at"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}

type Grant struct {
	ID       string          `json:"id"`
	Scope    string          `json:"scope"`
	Value    json.RawMessage `json:"value"`
	By       string          `json:"by"`
	At       string          `json:"at"`
	Until    *string         `json:"until" andon:"nullable"`
	Evidence string          `json:"evidence"`
	Revoked  *Revoked        `json:"revoked" andon:"nullable"`
	// Issues は名指しした課題の id（ADR-0041 決めたこと 6）。書けるのは scope が merge で値が A・B・C の grant だけ。
	// 名指しの grant はプロジェクト全体の値（Effective）に重ねず、名指しした課題の判定（MergeFor）にだけ効く。
	Issues []string `json:"issues,omitempty"`
}

// File は委譲（project.json の delegation。移す前の delegation.json と同じ形）。
type File struct {
	Version int     `json:"version"`
	Project string  `json:"project"`
	Preset  string  `json:"preset,omitempty"`
	Grants  []Grant `json:"grants"`
}

var (
	slugRe  = spawn.SlugRe // schema の project と同じ
	idRe    = regexp.MustCompile(`^d[1-9][0-9]*$`)
	issueRe = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[0-9]+$`) // schema の grants[].issues[] と同じ
)

// Date は暦にある YYYY-MM-DD か。
func Date(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Format("2006-01-02") == s
}

// ParseValue は scope の値を JSON の値にする（コマンドの引数から）。真偽値は true/false、irreversible は , 区切り（空は空の一覧）。
func ParseValue(scope, s string) (json.RawMessage, error) {
	var v any
	switch scope {
	case "irreversible":
		ops := []string{}
		for _, op := range strings.Split(s, ",") {
			if op = strings.TrimSpace(op); op != "" && !slices.Contains(ops, op) {
				ops = append(ops, op)
			}
		}
		v = ops
	case "external-requests", "profile", "package-update":
		switch s {
		case "true":
			v = true
		case "false":
			v = false
		default:
			return nil, fmt.Errorf("%s の値は true か false", scope)
		}
	default:
		v = s
	}
	b, _ := json.Marshal(v)
	if err := checkValue(scope, b); err != nil {
		return nil, err
	}
	return b, nil
}

func checkValue(scope string, raw json.RawMessage) error {
	if lv, ok := levels[scope]; ok {
		var s string
		if json.Unmarshal(raw, &s) != nil || !slices.Contains(lv, s) {
			return fmt.Errorf("%s の値は %s のどれか", scope, strings.Join(lv, "/"))
		}
		return nil
	}
	switch scope {
	case "irreversible":
		var ops []string
		if json.Unmarshal(raw, &ops) != nil || ops == nil {
			return errors.New("irreversible の値は操作の一覧（delete-branch / force-push / rewrite-history。delete-remote は委譲できない）")
		}
		for _, op := range ops {
			if !slices.Contains(irreversibleOps, op) {
				return fmt.Errorf("irreversible に %q は書けない（delete-branch / force-push / rewrite-history。delete-remote は委譲できない）", op)
			}
		}
	case "external-requests", "profile", "package-update":
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return fmt.Errorf("%s の値は true か false", scope)
		}
	default:
		return fmt.Errorf("知らない scope: %q（%s）", scope, strings.Join(Scopes, " / "))
	}
	return nil
}

// checkIssues は名指しの課題の決まり: scope が merge で値が A・B・C、id が1件以上で重ならない（ADR-0041 決めたこと 6）。
func checkIssues(scope string, value json.RawMessage, issues []string) error {
	var v string
	_ = json.Unmarshal(value, &v)
	switch {
	case scope != "merge" || !slices.Contains([]string{"A", "B", "C"}, v):
		return errors.New("issues（名指し）を書けるのは scope が merge で値が A・B・C の grant だけ")
	case len(issues) == 0:
		return errors.New("issues（名指し）が空")
	}
	seen := map[string]bool{}
	for _, id := range issues {
		switch {
		case !issueRe.MatchString(id):
			return fmt.Errorf("issues に課題の id でないものがある: %q", id)
		case seen[id]:
			return fmt.Errorf("issues に %s が重なる", id)
		}
		seen[id] = true
	}
	return nil
}

// Parse は委譲（project.json の delegation）を読み、schema と command の決まり（id が一意・project が置き場と同じ）で確かめる。
func Parse(b []byte, slug string) (*File, error) {
	var f File
	if err := contract.Decode(b, &f); err != nil {
		return nil, err
	}
	var raw struct {
		Grants []map[string]json.RawMessage `json:"grants"`
	}
	_ = json.Unmarshal(b, &raw)
	switch {
	case f.Version != 1:
		return nil, fmt.Errorf("version が 1 でない: %d", f.Version)
	case !slugRe.MatchString(f.Project) || f.Project != slug:
		return nil, fmt.Errorf("project %q が置き場 %q と違う", f.Project, slug)
	case f.Preset != "" && Presets[f.Preset] == nil:
		return nil, fmt.Errorf("知らない preset: %q", f.Preset)
	}
	seen := map[string]bool{}
	for i, g := range f.Grants {
		for _, k := range []string{"id", "scope", "value", "by", "at", "evidence"} {
			if _, ok := raw.Grants[i][k]; !ok {
				return nil, fmt.Errorf("grant %d に %s が無い", i+1, k)
			}
		}
		switch {
		case !idRe.MatchString(g.ID) || seen[g.ID]:
			return nil, fmt.Errorf("grant の id が d<n> でないか重なる: %q", g.ID)
		case g.By != "owner":
			return nil, fmt.Errorf("grant %s の by が owner でない", g.ID)
		case !Date(g.At) || (g.Until != nil && !Date(*g.Until)):
			return nil, fmt.Errorf("grant %s の at・until が暦にある YYYY-MM-DD でない", g.ID)
		case g.Evidence == "":
			return nil, fmt.Errorf("grant %s の evidence が空", g.ID)
		case g.Revoked != nil && (!Date(g.Revoked.At) || g.Revoked.By != "owner" || g.Revoked.Reason == ""):
			return nil, fmt.Errorf("grant %s の revoked は {at, by: owner, reason}", g.ID)
		}
		if err := checkValue(g.Scope, g.Value); err != nil {
			return nil, fmt.Errorf("grant %s: %v", g.ID, err)
		}
		if _, ok := raw.Grants[i]["issues"]; ok {
			if err := checkIssues(g.Scope, g.Value, g.Issues); err != nil {
				return nil, fmt.Errorf("grant %s: %v", g.ID, err)
			}
		}
		seen[g.ID] = true
	}
	return &f, nil
}

// Active は today に効いている grant か（取り消されておらず、期限が過ぎていない）。
func (g Grant) Active(today string) bool {
	return g.Revoked == nil && (g.Until == nil || *g.Until >= today)
}

// Effective は型に、効いている grant を at の順（同じ日なら書いた順）に重ねた値。名指しの grant（issues のあるもの）は重ねない
// （重ねると名指しの値がプロジェクト全体に漏れる。ADR-0041 決めたこと 6）。
func (f *File) Effective(today string) map[string]any {
	preset := f.Preset
	if preset == "" {
		preset = "work"
	}
	out := map[string]any{}
	for k, v := range Presets[preset] {
		out[k] = v
	}
	grants := slices.Clone(f.Grants)
	slices.SortStableFunc(grants, func(a, b Grant) int { return strings.Compare(a.At, b.At) })
	for _, g := range grants {
		if !g.Active(today) || len(g.Issues) > 0 {
			continue
		}
		var v any
		_ = json.Unmarshal(g.Value, &v)
		out[g.Scope] = v
	}
	return out
}

type Result struct {
	Project   string         `json:"project"`
	Trusted   bool           `json:"trusted"`
	Reason    string         `json:"reason,omitempty"` // 無委譲のときの理由（1行）
	Preset    string         `json:"preset,omitempty"`
	Effective map[string]any `json:"effective"`
	Grants    []Grant        `json:"grants"` // 効いている grant
}

// Merge は merge の値（無委譲は none）。
func (r Result) Merge() string {
	s, _ := r.Effective["merge"].(string)
	if s == "" {
		return "none"
	}
	return s
}

// MergeFor は課題 id の判定に使う merge の値: プロジェクト全体の値（Merge）と、id を名指しした効いている grant の値のうち、
// いちばん広いもの。名指しの grant がプロジェクト全体より広げたときは、その grant の id も返す（広げたのでなければ空）。
// oneWay（担当が戻せないと申告した）なら名指しを読まず、プロジェクト全体の値だけを返す（ADR-0041 決めたこと 6・7）。
// 名指しは広げる向きにだけ効く。読むのは gate の判定と、マージの直前の読み直しの2か所だけ。
func (r Result) MergeFor(id string, oneWay bool) (string, string) {
	merge, grant := r.Merge(), ""
	if oneWay || !r.Trusted {
		return merge, ""
	}
	lv := levels["merge"]
	for _, g := range r.Grants {
		var v string
		if g.Scope != "merge" || !slices.Contains(g.Issues, id) || json.Unmarshal(g.Value, &v) != nil {
			continue
		}
		if slices.Index(lv, v) > slices.Index(lv, merge) {
			merge, grant = v, g.ID
		}
	}
	return merge, grant
}

// Workspace はセルの repo（ANDON_WORKSPACE。既定 /workspace）。
func Workspace() string {
	if w := os.Getenv("ANDON_WORKSPACE"); w != "" {
		return w
	}
	return "/workspace"
}

// Dotenv はセルの repo ws の .env の key の値（最後の空でない行。前後の空白と引用符を除く）。無ければ def。
func Dotenv(ws, key, def string) string {
	f, err := os.Open(filepath.Join(ws, ".env"))
	if err != nil {
		return def
	}
	defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	v := def
	s := bufio.NewScanner(f)
	for s.Scan() {
		if k, val, ok := strings.Cut(s.Text(), "="); ok && strings.TrimSpace(k) == key {
			if val = strings.TrimSpace(strings.Trim(strings.TrimSpace(val), `"'`)); val != "" {
				v = val
			}
		}
	}
	return v
}

// Owner はセルの andon.toml の owner（git の author の email。最上段の owner = "…" の行だけを読む）。無ければ空。
func Owner(ws string) string {
	b, err := os.ReadFile(filepath.Join(ws, "andon.toml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			return ""
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != "owner" {
			continue
		}
		v = strings.TrimSpace(v)
		if i := strings.Index(v, "#"); i >= 0 && !strings.HasPrefix(v, `"`) {
			v = strings.TrimSpace(v[:i])
		}
		if len(v) >= 2 && v[0] == '"' {
			if j := strings.IndexByte(v[1:], '"'); j >= 0 {
				return v[1 : j+1]
			}
		}
		return ""
	}
	return ""
}

func git(ws string, args ...string) (string, error) { return gitEnv(ws, nil, args...) }

func gitEnv(ws string, env []string, args ...string) (string, error) {
	var errb bytes.Buffer
	c := exec.Command("git", append([]string{"-C", ws}, args...)...)
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return string(out), nil
}

// ErrAbsent はセルの repo の HEAD にそのファイルが無いこと。
var ErrAbsent = errors.New("セルの repo の HEAD に無い")

// OwnerFile はセルの repo の HEAD の rel を読み、それを最後に変えた commit の author の email が andon.toml の owner のときだけ返す
// （project.json・gate.local.json。オーナーだけが書くファイル。作業ツリーと未 commit の変更は読まない）。
func OwnerFile(ws, rel string) ([]byte, error) {
	b, err := git(ws, "show", "HEAD:"+filepath.ToSlash(rel))
	if err != nil {
		return nil, ErrAbsent
	}
	owner := Owner(ws)
	if owner == "" {
		return nil, errors.New("andon.toml に owner が無い")
	}
	author, err := git(ws, "log", "-1", "--format=%an%x00%ae", "HEAD", "--", rel)
	name, email, _ := strings.Cut(strings.TrimSpace(author), "\x00")
	if err != nil || email != owner {
		return nil, fmt.Errorf("%s を最後に変えた commit の author（%s <%s>）が owner でない", rel, name, email)
	}
	return []byte(b), nil
}

// Read はセルの repo の HEAD の委譲（project.json の delegation）を読む（読み手は全部これ）。信じられなければ無委譲。
// 移す前の delegation.json は読まない（無委譲。理由にそう書く）。
func Read(ws, slug, today string) Result {
	none := func(reason string) Result {
		return Result{Project: slug, Reason: reason, Effective: maps(None), Grants: []Grant{}}
	}
	p, err := ReadProject(ws, slug)
	if errors.Is(err, ErrAbsent) {
		if _, lerr := git(ws, "cat-file", "-e", "HEAD:"+filepath.ToSlash(LegacyRel(slug))); lerr == nil {
			return none("委譲が無い（" + LegacyRel(slug) + " は読まない。" + ProjectRel(slug) + " の delegation へ移すまで無委譲）")
		}
		return none("委譲が無い（セルの repo の HEAD に " + ProjectRel(slug) + " が無い）")
	}
	if err != nil {
		return none(err.Error())
	}
	if p.Delegation == nil {
		return none("委譲が無い（" + ProjectRel(slug) + " に delegation が無い）")
	}
	f, err := Parse(p.Delegation, slug)
	if err != nil {
		return none("委譲の形が違う: " + err.Error())
	}
	return FromFile(f, today)
}

// FromFile は信じられる委譲 f を、読み手が受け取る形にする（Read の後半。検査は f の形を Parse で作って使う）。
func FromFile(f *File, today string) Result {
	r := Result{Project: f.Project, Trusted: true, Preset: f.Preset, Effective: f.Effective(today), Grants: []Grant{}}
	if r.Preset == "" {
		r.Preset = "work"
	}
	for _, g := range f.Grants {
		if g.Active(today) {
			r.Grants = append(r.Grants, g)
		}
	}
	return r
}

func maps(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Narrower は a が b より狭いか同じか（narrow が確かめる）。
func Narrower(scope string, a, b any) bool {
	if lv, ok := levels[scope]; ok {
		as, _ := a.(string)
		bs, _ := b.(string)
		return slices.Index(lv, as) <= slices.Index(lv, bs)
	}
	switch scope {
	case "irreversible":
		bl := toStrings(b)
		for _, op := range toStrings(a) {
			if !slices.Contains(bl, op) {
				return false
			}
		}
		return true
	default:
		ab, _ := a.(bool)
		bb, _ := b.(bool)
		return !ab || bb
	}
}

func toStrings(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		var out []string
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func show(v any) string {
	switch x := v.(type) {
	case []string:
		if len(x) == 0 {
			return "（空）"
		}
		return strings.Join(x, " / ")
	case []any:
		if len(x) == 0 {
			return "（空）"
		}
		return strings.Join(toStrings(x), " / ")
	}
	return fmt.Sprint(v)
}
