// Package delegate は委譲（docs/contract.md §11・docs/design.md §1.10）。建屋の repo の projects/<slug>/delegation.json を
// 読み（Read。ゲート・クーリエ・command が使う）、人が書く手順を助ける（andon delegate grant/narrow/revoke/show）。
//
// 読み方: 建屋の repo の HEAD のファイルだけを読み、それを最後に変えた commit の author が andon.toml の owner と
// 一致するときだけ信じる。無い・読めない・形が違う・一致しない は無委譲（全部人）。
package delegate

import (
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
)

// Scopes は範囲（最初の版の8つ）。
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

// None は無委譲（全部人）の値。
var None = map[string]any{"merge": "none", "deploy": "none", "publish": "none", "irreversible": []string{},
	"external-requests": false, "profile": false, "package-update": false, "line-change": "none"}

// AndonProject は Andon 自身のプロジェクト（型 andon に固定。merge は B まで・line-change は none）。
const AndonProject = "andon"

// Revoked は取り消し。
type Revoked struct {
	At     string `json:"at"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}

// Grant は1つの付与。
type Grant struct {
	ID       string          `json:"id"`
	Scope    string          `json:"scope"`
	Value    json.RawMessage `json:"value"`
	By       string          `json:"by"`
	At       string          `json:"at"`
	Until    *string         `json:"until"`
	Evidence string          `json:"evidence"`
	Revoked  *Revoked        `json:"revoked"`
}

// File は delegation.json。
type File struct {
	Version int     `json:"version"`
	Project string  `json:"project"`
	Preset  string  `json:"preset,omitempty"`
	Grants  []Grant `json:"grants"`
}

var (
	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	idRe   = regexp.MustCompile(`^d[1-9][0-9]*$`)
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

// Parse は delegation.json を読み、schema と command の決まり（id が一意・project が置き場と同じ・andon は型 andon）で確かめる。
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
	case slug == AndonProject && f.Preset != AndonProject:
		return nil, errors.New("andon プロジェクトの preset は andon だけ")
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
		if slug == AndonProject {
			if err := andonCap(g.Scope, g.Value); err != nil {
				return nil, fmt.Errorf("grant %s: %v", g.ID, err)
			}
		}
		seen[g.ID] = true
	}
	return &f, nil
}

// andonCap は Andon 自身のプロジェクトの上限（merge は B まで・line-change は none）。
func andonCap(scope string, raw json.RawMessage) error {
	var s string
	_ = json.Unmarshal(raw, &s)
	if scope == "merge" && s == "C" {
		return errors.New("andon プロジェクトの merge は B まで")
	}
	if scope == "line-change" && s != "none" {
		return errors.New("andon プロジェクトの line-change は none だけ")
	}
	return nil
}

// Active は today に効いている grant か（取り消されておらず、期限が過ぎていない）。
func (g Grant) Active(today string) bool {
	return g.Revoked == nil && (g.Until == nil || *g.Until >= today)
}

// Effective は型に、効いている grant を at の順（同じ日なら書いた順）に重ねた値。
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
		if !g.Active(today) {
			continue
		}
		var v any
		_ = json.Unmarshal(g.Value, &v)
		out[g.Scope] = v
	}
	return out
}

// Result は読み手が受け取る委譲。
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

// Rel は建屋の repo の中の置き場。
func Rel(slug string) string { return filepath.Join("projects", slug, "delegation.json") }

// Workspace は建屋の repo（ANDON_WORKSPACE。既定 /workspace）。
func Workspace() string {
	if w := os.Getenv("ANDON_WORKSPACE"); w != "" {
		return w
	}
	return "/workspace"
}

// Owner は建屋の andon.toml の owner（git の author の email。最上段の owner = "…" の行だけを読む）。無ければ空。
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

func git(ws string, args ...string) (string, error) {
	var errb bytes.Buffer
	c := exec.Command("git", append([]string{"-C", ws}, args...)...)
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return string(out), nil
}

// ErrAbsent は建屋の repo の HEAD にそのファイルが無いこと。
var ErrAbsent = errors.New("建屋の repo の HEAD に無い")

// OwnerFile は建屋の repo の HEAD の rel を読み、それを最後に変えた commit の author の email が andon.toml の owner のときだけ返す
// （委譲・gate.local.json。人だけが書くファイル。作業ツリーと未 commit の変更は読まない）。
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

// Read は建屋の repo の HEAD の委譲を読む（読み手は全部これ）。信じられなければ無委譲。
func Read(ws, slug, today string) Result {
	none := func(reason string) Result {
		return Result{Project: slug, Reason: reason, Effective: maps(None), Grants: []Grant{}}
	}
	b, err := OwnerFile(ws, Rel(slug))
	if errors.Is(err, ErrAbsent) {
		return none("委譲が無い（建屋の repo の HEAD に " + Rel(slug) + " が無い）")
	}
	if err != nil {
		return none(err.Error())
	}
	f, err := Parse(b, slug)
	if err != nil {
		return none("委譲の形が違う: " + err.Error())
	}
	r := Result{Project: slug, Trusted: true, Preset: f.Preset, Effective: f.Effective(today), Grants: []Grant{}}
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

// Markdown は人用の一覧 delegation.md（command が生成する。機械は読まない）。
func Markdown(f *File, today string) string {
	preset := f.Preset
	if preset == "" {
		preset = "work（既定）"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# 委譲: %s\n\n", f.Project)
	fmt.Fprintf(&b, "andon delegate が delegation.json から生成する（手で直さない。直すのは andon delegate grant / narrow / revoke）。\n\n")
	fmt.Fprintf(&b, "型: %s。%s 時点で効いている値:\n\n", preset, today)
	b.WriteString("| 範囲 | 値 | 元 |\n|---|---|---|\n")
	eff := f.Effective(today)
	from := map[string]string{}
	grants := slices.Clone(f.Grants)
	slices.SortStableFunc(grants, func(a, b Grant) int { return strings.Compare(a.At, b.At) })
	for _, g := range grants {
		if g.Active(today) {
			from[g.Scope] = g.ID
		}
	}
	for _, s := range Scopes {
		src := "型"
		if id := from[s]; id != "" {
			src = id
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s, show(eff[s]), src)
	}
	if len(f.Grants) > 0 {
		b.WriteString("\n## 付与と取り消し（履歴。行は消さない）\n\n| id | 範囲 | 値 | 日付 | 期限 | 根拠 | 取り消し |\n|---|---|---|---|---|---|---|\n")
		for _, g := range f.Grants {
			var v any
			_ = json.Unmarshal(g.Value, &v)
			until, revoked := "無期限", ""
			if g.Until != nil {
				until = *g.Until
			}
			if g.Revoked != nil {
				revoked = g.Revoked.At + " " + cell(g.Revoked.Reason)
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s | %s | %s |\n", g.ID, g.Scope, show(v), g.At, until, cell(g.Evidence), revoked)
		}
	}
	return b.String()
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

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}
