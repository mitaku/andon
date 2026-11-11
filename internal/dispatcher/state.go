package dispatcher

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
	Issue   string         `json:"issue,omitempty"`
	Kind    string         `json:"kind"`
	Detail  map[string]any `json:"detail"`
}

// Reviewer は起こしたレビュアー（控えの「issue → reviewer_issue・commit」）。状態は Started・Skipped・Failed のどれか。
type Reviewer struct {
	Issue         string `json:"issue"`
	Commit        string `json:"commit"`
	Round         int    `json:"round,omitempty"`
	Repo          string `json:"repo,omitempty"`
	Worktree      string `json:"worktree,omitempty"`       // 作った detached worktree（andon issue finish が消す）
	ReviewerIssue string `json:"reviewer_issue,omitempty"` // 足したレビュアーの課題
	Address       string `json:"address,omitempty"`        // 起こした端末
	Started       bool   `json:"started,omitempty"`        // 最初のメッセージまで届いた（もう起こさない）
	Skipped       string `json:"skipped,omitempty"`        // 起こさない理由（設定・上限・head 違い）
	OverLimit     int    `json:"over_limit,omitempty"`     // 上限を超えて起こさなかったときの上限（それより上がれば起こし直す）
	Failed        string `json:"failed,omitempty"`         // 起こしそこねた理由（次の見回りで続きから）
	Attempts      int    `json:"attempts,omitempty"`       // 起こしそこねた回数
	Row           int    `json:"row,omitempty"`            // 決めたときの課題の報告の最後の行（増えれば頼み直し。Skipped を拾い直す）
}

// Gate は回したゲート（控えの「issue → sha」）。review に入った log.jsonl の行ごとに1回。
type Gate struct {
	Issue    string `json:"issue"`
	Sha      string `json:"sha,omitempty"`
	Decision string `json:"decision,omitempty"` // 空なら回していない（初めて見た）
	Merged   string `json:"merged,omitempty"`   // マージの commit
}

// State は dispatcher の控え（~/.local/state/andon/dispatcher/<slug>.json。台帳の外）。
type State struct {
	Reviewers map[string]Reviewer `json:"reviewers"`           // <issue>@<commit>
	Gates     map[string]Gate     `json:"gates"`               // <issue>#<log.jsonl の行>
	Pending   []Notification      `json:"pending"`             // 待ちの通知（drain.go のフックと deliver.go の離席時の hc send が届ける）
	Coord     string              `json:"coord,omitempty"`     // 離席時に送った Chief of Staff の端末（替わったら NewsTurn を捨てる）
	NewsTurn  *int                `json:"news_turn,omitempty"` // 離席時に <news> を送った turn（dispatcher.md §4.1 の c）
	Hook      *HookNews           `json:"hook,omitempty"`      // フックで渡して、その turn の終わりをまだ見ていない <news>
	LogPos    *int64              `json:"log_pos,omitempty"`   // log.jsonl の読んだ位置（バイト。無ければ初めて）
	LogHead   string              `json:"log_head,omitempty"`  // log.jsonl の最初の行の sha256（替われば先頭から読み直す）
	LogHold   *int64              `json:"log_hold,omitempty"`  // 問いの JSON を待って止まった行の頭（1回だけ待つ）
	Ready     *[]string           `json:"ready,omitempty"`     // 前回の ready の id（人の課題を除く。無ければ初めて）
	Sent      map[string]bool     `json:"sent,omitempty"`      // 知らせた担当の状態（<issue>|<agent>|<状態>|<status_ms>）
	Retry     map[string]Resend   `json:"retry,omitempty"`     // 提供元に切られて「続けて」を打った回数（<issue>|<agent>。Chief of Staff は :cos|<address>）
	First     map[string]bool     `json:"first,omitempty"`     // 最初のメッセージを送ったがラベルを外しそこねた担当（<issue>|<agent>。二度と送らない）
	Moved     map[string]Moved    `json:"moved,omitempty"`     // working の担当の status.msg と画面の要約と、それが最後に動いた時刻（<issue>|<agent>。止まったかの見分け）
	Impact    map[string]int      `json:"impact,omitempty"`    // done になった、影響（andon issue impact）をまだ見ていない課題 → 失敗した回数
	// done になった、人の手直し（corrections.json）をまだ集めていない課題 → 失敗した回数
	Corrections map[string]int `json:"corrections,omitempty"`
	// review の課題の PR を人がマージしたかの控え（humanmerge.go。dispatcher.md §2.7）
	Watch map[string]Watch `json:"watch,omitempty"`
	// 記録だけにした gate の approve（merge: auto）で、merged も asked もまだ無いもの（route.go の stuck。ADR-0010 決めたこと 1）
	Approved map[string]Notification `json:"approved,omitempty"`
	// 記録だけの置き場に書けなかった通知（route.go の flush が次の見回りで先に書く）
	Unrecorded []Notification `json:"unrecorded,omitempty"`
	// news.replied を書いた数を、何で書いたかごとに（ack だけ。前の版の控えには no-reply が残りうる。ADR-0010 決めたこと 6。ack.go）
	Replied map[string]int `json:"replied,omitempty"`
}

// HookNews はフックで渡した <news>（dispatcher.md §3）。渡した turn の Stop で確かめ済みにする。
// 確かめないまま別の turn が来たら、News だけを1回届け直す（Again は届け直した分で、もう届け直さない）。
type HookNews struct {
	Terminal  string         `json:"terminal"`
	Via       string         `json:"via"` // stop（block の reason）・prompt（additionalContext）
	SessionID string         `json:"session_id,omitempty"`
	PromptID  string         `json:"prompt_id,omitempty"`
	News      []Notification `json:"news,omitempty"`
	Again     []Notification `json:"again,omitempty"`
	Turn      *int           `json:"turn,omitempty"`       // 渡したときの端末の turn（ラベル news.ack と照らす。ADR-0010 決めたこと 5）
	AckBefore string         `json:"ack_before,omitempty"` // 渡したときのラベル news.ack の値（同じ turn の前の ack を数えない）
}

// StatePath は控えの置き場。
func StatePath(slug string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "dispatcher", slug+".json")
}

// LoadState は控えを読む。無ければ空の控えと first=true（初めて見る台帳）。
func LoadState(slug string) (*State, bool, error) {
	st := &State{Reviewers: map[string]Reviewer{}, Gates: map[string]Gate{}, Pending: []Notification{}}
	st.ensure()
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
	st.ensure()
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

// Push は通知を積む。同じ課題の同じ出来事（issue と kind）は新しい1件に置き換える。
func (s *State) Push(n Notification) {
	out := s.Pending[:0]
	for _, x := range s.Pending {
		if x.Issue != n.Issue || x.Kind != n.Kind {
			out = append(out, x)
		}
	}
	s.Pending = append(out, n)
}
