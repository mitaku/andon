package courier

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var askedRe = regexp.MustCompile(`#([1-9][0-9]*)$`)

// Notification は通知（contract.md §6。schema/notification.schema.json）。
type Notification struct {
	At      string         `json:"at"`
	Project string         `json:"project"`
	Task    string         `json:"task,omitempty"`
	Kind    string         `json:"kind"`
	Detail  map[string]any `json:"detail"`
}

// Reviewer は起こしたレビュアー（控えの「task → reviewer_task・commit」）。
type Reviewer struct {
	Task         string `json:"task"`
	ReviewerTask string `json:"reviewer_task,omitempty"` // 空なら起こしていない（初めて見た・起こさなかった）
	Commit       string `json:"commit"`
	Round        int    `json:"round,omitempty"`
	Repo         string `json:"repo,omitempty"`
	Worktree     string `json:"worktree,omitempty"`   // detached worktree（andon task finish が消す）
	Skipped      string `json:"skipped,omitempty"`    // 起こさなかった理由
	OverLimit    int    `json:"over_limit,omitempty"` // 上限を超えて起こさなかったときの上限（それより上がれば起こし直す）
}

// Gate は回したゲート（控えの「task → sha」）。review に入った log.jsonl の行ごとに1回。
type Gate struct {
	Task     string `json:"task"`
	Sha      string `json:"sha,omitempty"`
	Decision string `json:"decision,omitempty"` // 空なら回していない（初めて見た）
	Merged   string `json:"merged,omitempty"`   // マージの commit
}

// State はクーリエの控え（~/.local/state/andon/courier/<slug>.json。台帳の外）。
type State struct {
	Reviewers map[string]Reviewer `json:"reviewers"` // <task>@<commit>
	Gates     map[string]Gate     `json:"gates"`     // <task>#<log.jsonl の行>
	Pending   []Notification      `json:"pending"`   // 待ちの通知（届けるのは P3b・P6b）
}

// StatePath は控えの置き場。
func StatePath(slug string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "courier", slug+".json")
}

// LoadState は控えを読む。無ければ空の控えと first=true（初めて見る台帳）。
func LoadState(slug string) (*State, bool, error) {
	st := &State{Reviewers: map[string]Reviewer{}, Gates: map[string]Gate{}, Pending: []Notification{}}
	b, err := os.ReadFile(StatePath(slug))
	if errors.Is(err, os.ErrNotExist) {
		return st, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, false, errors.New("控えが読めない: " + StatePath(slug))
	}
	if st.Reviewers == nil {
		st.Reviewers = map[string]Reviewer{}
	}
	if st.Gates == nil {
		st.Gates = map[string]Gate{}
	}
	if st.Pending == nil {
		st.Pending = []Notification{}
	}
	return st, false, nil
}

// Save は控えを書く（書いてから置き換える）。
func (s *State) Save(slug string) error {
	path := StatePath(slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Push は通知を積む。同じタスクの同じ出来事（task と kind）は新しい1件に置き換える。
func (s *State) Push(n Notification) {
	out := s.Pending[:0]
	for _, x := range s.Pending {
		if x.Task != n.Task || x.Kind != n.Kind {
			out = append(out, x)
		}
	}
	s.Pending = append(out, n)
}
