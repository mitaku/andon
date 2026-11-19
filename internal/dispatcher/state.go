package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// Reviewer は起動したレビュアー（控えの「issue → reviewer_issue・commit」）。状態は Started・Skipped・Failed のどれか。
type Reviewer struct {
	Issue         string `json:"issue"`
	Commit        string `json:"commit"`
	Round         int    `json:"round,omitempty"`
	Repo          string `json:"repo,omitempty"`
	Worktree      string `json:"worktree,omitempty"`       // 作った detached worktree（andon issue finish が消す）
	ReviewerIssue string `json:"reviewer_issue,omitempty"` // 足したレビュアーの課題
	Address       string `json:"address,omitempty"`        // 起動した端末
	Started       bool   `json:"started,omitempty"`        // 最初のメッセージまで届いた（もう起動しない）
	Skipped       string `json:"skipped,omitempty"`        // 起動しない理由（設定・上限・head 違い）
	OverLimit     int    `json:"over_limit,omitempty"`     // 上限を超えて起動しなかったときの上限（それより上がれば起動し直す）
	Failed        string `json:"failed,omitempty"`         // 起動しそこねた理由（次の見回りで続きから）
	Attempts      int    `json:"attempts,omitempty"`       // 起動しそこねた回数
	Row           int    `json:"row,omitempty"`            // 決めたときの課題の報告の最後の行（増えれば頼み直し。Skipped を拾い直す）
}

// State は dispatcher の控え（~/.local/state/andon/dispatcher/<slug>.json。台帳の外）。
type State struct {
	Reviewers map[string]Reviewer `json:"reviewers"`          // <issue>@<commit>
	Pending   []Notification      `json:"pending"`            // 待ちの通知（drain.go のフックが届ける。離席中は wait.go の待ち役が合図で起動する）
	Hook      *HookNews           `json:"hook,omitempty"`     // フックで渡して、その turn の終わりをまだ見ていない <news>
	LogPos    *int64              `json:"log_pos,omitempty"`  // log.jsonl の読んだ位置（バイト。無ければ初めて）
	LogHead   string              `json:"log_head,omitempty"` // log.jsonl の最初の行の sha256（替われば先頭から読み直す）
	LogHold   *int64              `json:"log_hold,omitempty"` // 問いの JSON を待って止まった行の頭（1回だけ待つ）
	Ready     *[]string           `json:"ready,omitempty"`    // 前回の ready の id（人の課題を除く。無ければ初めて）
	Sent      map[string]bool     `json:"sent,omitempty"`     // 知らせた担当の状態（<issue>|<agent>|<状態>|<status_ms>）
	Retry     map[string]Resend   `json:"retry,omitempty"`    // プロバイダに切られて「続けて」を打った回数（<issue>|<agent>。Chief of Staff は :cos|<address>）
	First     map[string]bool     `json:"first,omitempty"`    // 最初のメッセージを送ったがラベルを外しそこねた担当（<issue>|<agent>。二度と送らない）
	Moved     map[string]Moved    `json:"moved,omitempty"`    // working の担当の status.msg と画面の要約と、それが最後に動いた時刻（<issue>|<agent>。止まったかの見分け）
	Impact    map[string]int      `json:"impact,omitempty"`   // done になった、影響（andon issue impact）をまだ見ていない課題 → 失敗した回数
	// done になった、人の手直し（corrections.json）をまだ集めていない課題 → 失敗した回数
	Corrections map[string]int `json:"corrections,omitempty"`
	// approve のまま 10 分たって届けた課題 → andon issue merge の記録の approved（route.go の stuck。同じ approve では二度届けない）
	Stuck map[string]string `json:"stuck,omitempty"`
	// 記録だけの置き場に書けなかった通知（route.go の flush が次の見回りで先に書く）
	Unrecorded []Notification `json:"unrecorded,omitempty"`
	// news.replied を書いた数を、何で書いたかごとに（ack だけ。前の版の控えには no-reply が残りうる。ADR-0010 決めたこと 6。ack.go）
	Replied map[string]int `json:"replied,omitempty"`
	// dispatcher が andon issue resume を打った課題 → Gone を見た端末と起動した端末（resume.go。ADR-0012 決めたこと 4・13 の 3）
	Resumed map[string]Resumed `json:"resumed,omitempty"`
	// ゲートが gh の上限（rate limit）で落ちたとき、上限が戻る時刻（RFC3339。loop.go の merge）。それまでこの台帳の andon issue merge を打たない
	GHReset string `json:"gh_reset,omitempty"`
	// ゲートが続けて gh の上限で落ちたときの、前の待ち（分。1・2・4…60。上限で落ちずにゲートが返れば 0）
	GHWait int `json:"gh_wait,omitempty"`
	// 見込みを超えた段を知らせた課題（<issue>|estimate|<見込みの値>|<段>。estimate.go。ADR-0014 決めたこと 6）
	Estimate map[string]bool `json:"estimate,omitempty"`
	// 担当の端末が Gone だと見た時刻（課題 → 端末と時刻。estimate.go が実働から死んでいた区間を引く。ADR-0014 決めたこと 5）
	Gone map[string][]GoneAt `json:"gone,omitempty"`
	// memory の件数が線を超えて memory を積んだ（線の下に戻ると false。memory.go。ADR-0020 決めたこと 4）
	MemoryOver bool `json:"memory_over,omitempty"`
	// Chief of Staff の留守（端末が無い間。配る段だけ止めて、積んだ数を数える。ADR-0017 決めたこと 8。drain が戻った最初の <news> に1行付けて消す）
	Away *Away `json:"away,omitempty"`
}

// Away は Chief of Staff の留守の記録（ADR-0017 決めたこと 8 の (b)）。
type Away struct {
	Since string `json:"since"`           // 留守に気づいた巡の時刻
	Until string `json:"until,omitempty"` // 端末が戻ったと気づいた巡の時刻（空なら留守の最中か、drain が先に気づいた）
	Count int    `json:"count"`           // 留守の間に待ちに積んだ通知の数（置き換えの前の数。決めたこと 8 の (d)）
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

// MergePath は andon issue merge の記録（控えの横。書き手は merge だけ。dispatcher は approve のまま 10 分を読む。ADR-0017 決めたこと 5・6）。
func MergePath(slug string) string {
	return strings.TrimSuffix(StatePath(slug), ".json") + ".merge.json"
}

// LoadState は控えを読む。無ければ空の控えと first=true（初めて見る台帳）。
func LoadState(slug string) (*State, bool, error) {
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
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

// Push は通知を積む。同じ出来事（eventKey が同じ）は新しい1件に置き換える。留守の間は積んだ数を数える（置き換えの前の数）。
func (s *State) Push(n Notification) {
	out := s.Pending[:0]
	for _, x := range s.Pending {
		if eventKey(x) != eventKey(n) {
			out = append(out, x)
		}
	}
	s.Pending = append(out, n)
	if s.Away != nil && s.Away.Until == "" {
		s.Away.Count++
	}
}

// eventKey は通知の出来事の名前。同じ名前の通知は新しい1件に置き換える（課題と kind）。
// signal は issue が無いので、同じ信号（detail の source と key。fingerprint はこの2つから決まる）だけ
// （ADR-0011 決めたこと 8 の改め。別の信号どうしは消し合わない）。asked は問いの n ごと
// （同じ課題の2つ目の問いが1つ目を消さない。ADR-0017 決めたこと 8 の (d)）。
func eventKey(n Notification) string {
	k := n.Issue + "\x00" + n.Kind
	switch n.Kind {
	case "signal":
		k += "\x00" + fmt.Sprint(n.Detail["source"]) + "\x00" + fmt.Sprint(n.Detail["key"])
	case "asked":
		k += "\x00" + fmt.Sprint(n.Detail["n"])
	}
	return k
}
