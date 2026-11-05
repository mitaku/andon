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

// Reviewer は起こしたレビュアー（控えの「task → reviewer_task・commit」）。状態は Started・Skipped・Failed のどれか。
type Reviewer struct {
	Task         string `json:"task"`
	Commit       string `json:"commit"`
	Round        int    `json:"round,omitempty"`
	Repo         string `json:"repo,omitempty"`
	Worktree     string `json:"worktree,omitempty"`      // 作った detached worktree（andon task finish が消す）
	ReviewerTask string `json:"reviewer_task,omitempty"` // 足したレビュアーのタスク
	Address      string `json:"address,omitempty"`       // 起こした端末
	Started      bool   `json:"started,omitempty"`       // 最初のメッセージまで届いた（もう起こさない）
	Skipped      string `json:"skipped,omitempty"`       // 起こさない理由（初めて見た・設定・上限・head 違い）
	OverLimit    int    `json:"over_limit,omitempty"`    // 上限を超えて起こさなかったときの上限（それより上がれば起こし直す）
	Failed       string `json:"failed,omitempty"`        // 起こしそこねた理由（次の見回りで続きから）
	Attempts     int    `json:"attempts,omitempty"`      // 起こしそこねた回数
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
	Reviewers map[string]Reviewer `json:"reviewers"`           // <task>@<commit>
	Gates     map[string]Gate     `json:"gates"`               // <task>#<log.jsonl の行>
	Pending   []Notification      `json:"pending"`             // 待ちの通知（drain.go のフックと deliver.go の離席時の hc send が届ける）
	Coord     string              `json:"coord,omitempty"`     // 離席時に送った Chief of Staff の端末（替わったら NewsTurn を捨てる）
	NewsTurn  *int                `json:"news_turn,omitempty"` // 離席時に <news> を送った turn（courier.md §4.1 の c）
	Hook      string              `json:"hook,omitempty"`      // フックで <news> を渡した端末（次の Stop で <no-reply> を見る）
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
