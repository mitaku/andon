package delegate

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"

	"andon/internal/contract"
	"andon/internal/spawn"
)

// Project は projects/<slug>/project.json（docs/contract.md §11・ADR-0004 決めたこと 1）。人だけが書く機械向けの事実。
// 委譲は delegation に今の delegation.json と同じ形で入る。
type Project struct {
	Version      int             `json:"version"`
	Project      ProjectInfo     `json:"project"`
	Repos        []string        `json:"repos"`
	References   []string        `json:"references,omitempty"`
	Delegation   json.RawMessage `json:"delegation,omitempty"`
	ChiefOfStaff *ChiefOfStaff   `json:"chief_of_staff,omitempty"`
}

// ProjectInfo は題と接頭字。
type ProjectInfo struct {
	Title  string `json:"title"`
	Prefix string `json:"prefix"`
}

// ChiefOfStaff は Chief of Staff の設定値（読むのは Chief of Staff。無いキーは skill の既定）。
type ChiefOfStaff struct {
	Parallel       *int     `json:"parallel,omitempty"`
	ReviewWhen     []string `json:"review_when,omitempty"`
	TriageMaxTasks *int     `json:"triage_max_tasks,omitempty"`
}

var (
	prefixRe = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	urlRe    = regexp.MustCompile(`^https://[!-~]+$`)
)

// ProjectRel はセルの repo の中の project.json の置き場。
func ProjectRel(slug string) string { return filepath.Join("projects", slug, "project.json") }

// LegacyRel は移す前の委譲の置き場（読まない。無委譲の理由を言うためだけに見る）。
func LegacyRel(slug string) string { return filepath.Join("projects", slug, "delegation.json") }

// ParseProject は project.json を読み、schema と command の決まりで確かめる（delegation があればその中身も Parse で）。
func ParseProject(b []byte, slug string) (*Project, error) {
	var p Project
	if err := contract.Decode(b, &p); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	for _, k := range []string{"version", "project", "repos"} {
		if _, ok := raw[k]; !ok {
			return nil, fmt.Errorf("%s が無い", k)
		}
	}
	var info map[string]json.RawMessage
	_ = json.Unmarshal(raw["project"], &info)
	for _, k := range []string{"title", "prefix"} {
		if _, ok := info[k]; !ok {
			return nil, fmt.Errorf("project.%s が無い", k)
		}
	}
	switch {
	case p.Version != 1:
		return nil, fmt.Errorf("version が 1 でない: %d", p.Version)
	case p.Project.Title == "":
		return nil, errors.New("project.title が空")
	case !prefixRe.MatchString(p.Project.Prefix):
		return nil, fmt.Errorf("project.prefix が英大文字で始まる英大文字・数字でない: %q", p.Project.Prefix)
	}
	for _, r := range p.Repos {
		if !spawn.RepoRe.MatchString(r) {
			return nil, fmt.Errorf("repos の %q が <host>/<owner>/<repo> でない", r)
		}
	}
	if len(p.Repos) != len(uniq(p.Repos)) {
		return nil, errors.New("repos に同じ repo が2つある")
	}
	for _, r := range p.References {
		if !spawn.RepoRe.MatchString(r) && !urlRe.MatchString(r) {
			return nil, fmt.Errorf("references の %q が <host>/<owner>/<repo> でも https の URL でもない", r)
		}
	}
	if c := p.ChiefOfStaff; c != nil {
		if (c.Parallel != nil && *c.Parallel < 1) || (c.TriageMaxTasks != nil && *c.TriageMaxTasks < 1) {
			return nil, errors.New("chief_of_staff の parallel・triage_max_tasks は 1 以上")
		}
		if slices.Contains(c.ReviewWhen, "") {
			return nil, errors.New("chief_of_staff.review_when に空の条件がある")
		}
	}
	if p.Delegation != nil {
		if _, err := Parse(p.Delegation, slug); err != nil {
			return nil, fmt.Errorf("delegation: %v", err)
		}
	}
	return &p, nil
}

func uniq(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return slices.Compact(out)
}

// ReadProject はセルの repo の HEAD の project.json を、author 照合（OwnerFile）を通して読む。
// 無ければ ErrAbsent。信じられない・形が違うものは誤り（読み手は安全側に倒す: 無委譲・repo 無し）。
func ReadProject(ws, slug string) (*Project, error) {
	b, err := OwnerFile(ws, ProjectRel(slug))
	if err != nil {
		return nil, err
	}
	p, err := ParseProject(b, slug)
	if err != nil {
		return nil, fmt.Errorf("%s の形が違う: %v", ProjectRel(slug), err)
	}
	return p, nil
}
