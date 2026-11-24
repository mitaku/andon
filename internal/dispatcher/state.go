package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"andon/internal/contract"
)

// Notification は通知（contract.md §6。schema/notification.schema.json）。
type Notification struct {
	At      string         `json:"at"`
	Project string         `json:"project"`
	Issue   string         `json:"issue,omitempty"`
	Kind    string         `json:"kind"`
	Detail  map[string]any `json:"detail"`
}

type Reviewer struct {
	Issue         string `json:"issue"`
	Commit        string `json:"commit"`
	Round         int    `json:"round,omitempty"`
	Repo          string `json:"repo,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	ReviewerIssue string `json:"reviewer_issue,omitempty"`
	Address       string `json:"address,omitempty"`
	Started       bool   `json:"started,omitempty"`
	Skipped       string `json:"skipped,omitempty"`
	OverLimit     int    `json:"over_limit,omitempty"`
	Untrusted     bool   `json:"untrusted,omitempty"`
	Failed        string `json:"failed,omitempty"`
	Attempts      int    `json:"attempts,omitempty"`
	Row           int    `json:"row,omitempty"`
}

type State struct {
	Reviewers   map[string]Reviewer `json:"reviewers"`
	Pending     []Notification      `json:"pending"`
	Hook        *HookNews           `json:"hook,omitempty"`
	LogPos      *int64              `json:"log_pos,omitempty"`
	LogHead     string              `json:"log_head,omitempty"`
	LogHold     *int64              `json:"log_hold,omitempty"`
	Ready       *[]string           `json:"ready,omitempty"`
	Sent        map[string]bool     `json:"sent,omitempty"`
	Retry       map[string]Resend   `json:"retry,omitempty"`
	First       map[string]bool     `json:"first,omitempty"`
	Moved       map[string]Moved    `json:"moved,omitempty"`
	Impact      Tries               `json:"impact,omitempty"`
	Corrections Tries               `json:"corrections,omitempty"`
	Outcomes    Tries               `json:"outcomes,omitempty"`
	Stuck       map[string]string   `json:"stuck,omitempty"`
	Unrecorded  []Notification      `json:"unrecorded,omitempty"`
	// news.replied を書いた数を、何で書いたかごとに（ack だけ。前のバージョンの控えには no-reply が残りうる。ADR-0010 決めたこと 6。ack.go）
	Replied map[string]int `json:"replied,omitempty"`
	// dispatcher が andon issue resume を打った課題 → Gone を見た端末と起動した端末（resume.go。ADR-0012 決めたこと 4・13 の 3）
	Resumed map[string]Resumed `json:"resumed,omitempty"`
	GHReset string             `json:"gh_reset,omitempty"`
	GHWait  int                `json:"gh_wait,omitempty"`
	// 見込みを超えた段を知らせた課題（<issue>|estimate|<見込みの値>|<段>。estimate.go。ADR-0014 決めたこと 6）
	Estimate map[string]bool `json:"estimate,omitempty"`
	// 担当の端末が Gone だと見た時刻（課題 → 端末と時刻。estimate.go が実働から死んでいた区間を引く。ADR-0014 決めたこと 5）
	Gone map[string][]GoneAt `json:"gone,omitempty"`
	// Chief of Staff の留守（端末が無い間。配る段だけ止めて、積んだ数を数える。ADR-0017 決めたこと 8。drain が戻った最初の <news> に1行付けて消す）
	Away  *Away           `json:"away,omitempty"`
	Asked map[string]bool `json:"asked,omitempty"`
	// 空きを見た前の回の結果と、通知 idle で知らせたもの（idle.go。ADR-0047 決めたこと 4・11・12）
	Idle *Idle `json:"idle,omitempty"`
}

// Tries は done の行ごとに1回だけする仕事（影響の集め・オーナーの手直しの集め・結果の記録）の、まだ済んでいない課題 →
// 失敗した回数。失敗を数えて上限で諦める控えを1つの型にまとめる（ADR-0034 決めたこと 9）。控えの JSON の形は課題 → 数のまま。
type Tries map[string]int

func (t *Tries) Add(id string) {
	if *t == nil {
		*t = Tries{}
	}
	if _, ok := (*t)[id]; !ok {
		(*t)[id] = 0
	}
}

func (t *Tries) Each(do func(id string) bool) {
	ids := make([]string, 0, len(*t))
	for id := range *t {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if do(id) {
			delete(*t, id)
		}
	}
	if len(*t) == 0 {
		*t = nil
	}
}

func (t Tries) Fail(id string, limit int) bool {
	t[id]++
	return t[id] >= limit
}

// Away は Chief of Staff の留守の記録（ADR-0017 決めたこと 8 の (b)）。
type Away struct {
	Since string `json:"since"`
	Until string `json:"until,omitempty"`
	Count int    `json:"count"` // 留守の間に待ちに積んだ通知の数（置き換えの前の数。決めたこと 8 の (d)）
}

// HookNews はフックで渡した <news>（dispatcher.md §3）。渡した turn の Stop で確かめ済みにする。
// 確かめないまま別の turn が来たら、News だけを1回届け直す（Again は届け直した分で、もう届け直さない）。
type HookNews struct {
	Terminal  string         `json:"terminal"`
	Via       string         `json:"via"`
	SessionID string         `json:"session_id,omitempty"`
	PromptID  string         `json:"prompt_id,omitempty"`
	News      []Notification `json:"news,omitempty"`
	Again     []Notification `json:"again,omitempty"`
	Turn      *int           `json:"turn,omitempty"` // 渡したときの端末の turn（ラベル news.ack と照らす。ADR-0010 決めたこと 5）
	AckBefore string         `json:"ack_before,omitempty"`
}

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

// Save は控えを書く（書いてから置き換える）。控えそのものは約束でないが、中の通知（待ち・記録の書き残し・フックで渡した分）は
// 約束の JSON なので、書く前に schema/notification.schema.json と照らし、合わない通知があれば書かずに誤りを返す。
func (s *State) Save(slug string) error {
	if err := s.checkNotifications(); err != nil {
		return err
	}
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

func (s *State) checkNotifications() error {
	all := append(append([]Notification{}, s.Pending...), s.Unrecorded...)
	if s.Hook != nil {
		all = append(append(all, s.Hook.News...), s.Hook.Again...)
	}
	for _, n := range all {
		if _, err := contract.EncodeLine(contract.SchemaNotification, n); err != nil {
			return fmt.Errorf("通知（%s %s）: %v", n.Issue, n.Kind, err)
		}
	}
	return nil
}

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
// （同じ課題の2つ目の問いが1つ目を消さない。ADR-0017 決めたこと 8 の (d)）。answered も問いの n ごと。
func eventKey(n Notification) string {
	k := n.Issue + "\x00" + n.Kind
	switch n.Kind {
	case "signal":
		k += "\x00" + fmt.Sprint(n.Detail["source"]) + "\x00" + fmt.Sprint(n.Detail["key"])
	case "asked", "answered":
		k += "\x00" + fmt.Sprint(n.Detail["n"])
	}
	return k
}
