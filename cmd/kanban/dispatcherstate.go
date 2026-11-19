package main

// The shape of Andon's dispatcher state (internal/dispatcher, State), so that kanban migrate
// can check a state file reads as the dispatcher reads it before it moves anything. A copy,
// not an import: internal/dispatcher doesn't build for every target kanban is built for.
// TestDispatcherStateShape keeps the two the same.

type dsNotification struct {
	At      string         `json:"at"`
	Project string         `json:"project"`
	Issue   string         `json:"issue,omitempty"`
	Kind    string         `json:"kind"`
	Detail  map[string]any `json:"detail"`
}

type dsReviewer struct {
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
	Failed        string `json:"failed,omitempty"`
	Attempts      int    `json:"attempts,omitempty"`
	Row           int    `json:"row,omitempty"`
}

type dsHookNews struct {
	Terminal  string           `json:"terminal"`
	Via       string           `json:"via"`
	SessionID string           `json:"session_id,omitempty"`
	PromptID  string           `json:"prompt_id,omitempty"`
	News      []dsNotification `json:"news,omitempty"`
	Again     []dsNotification `json:"again,omitempty"`
	Turn      *int             `json:"turn,omitempty"`
	AckBefore string           `json:"ack_before,omitempty"`
}

type dsResend struct {
	N     int   `json:"n"`
	At    int64 `json:"at"`
	Cut   int64 `json:"cut,omitempty"`
	Since int64 `json:"since,omitempty"`
	Sent  int64 `json:"sent,omitempty"`
}

type dsMoved struct {
	Msg    string `json:"msg"`
	Screen string `json:"screen,omitempty"`
	At     int64  `json:"at"`
}

type dispatcherState struct {
	Reviewers map[string]dsReviewer `json:"reviewers"`
	Pending   []dsNotification      `json:"pending"`
	Hook      *dsHookNews           `json:"hook,omitempty"`
	LogPos    *int64                `json:"log_pos,omitempty"`
	LogHead   string                `json:"log_head,omitempty"`
	LogHold   *int64                `json:"log_hold,omitempty"`
	Ready     *[]string             `json:"ready,omitempty"`
	Sent      map[string]bool       `json:"sent,omitempty"`
	Retry     map[string]dsResend   `json:"retry,omitempty"`
	First     map[string]bool       `json:"first,omitempty"`
	Moved     map[string]dsMoved    `json:"moved,omitempty"`
	Impact    map[string]int        `json:"impact,omitempty"`
	// Issues done whose corrections.json is not collected yet → failed tries
	Corrections map[string]int `json:"corrections,omitempty"`
	// Issues whose approve stayed undecided 10 minutes and was delivered → the merge record's approved
	Stuck map[string]string `json:"stuck,omitempty"`
	// Record-only notifications not yet appended to <slug>.recorded.jsonl
	Unrecorded []dsNotification `json:"unrecorded,omitempty"`
	// news.replied written, by what (ack; no-reply only in state from older versions)
	Replied map[string]int `json:"replied,omitempty"`
	// Issues the dispatcher ran andon issue resume for → the gone terminal and the one it started
	Resumed map[string]dsResumed `json:"resumed,omitempty"`
	GHReset string               `json:"gh_reset,omitempty"`
	GHWait  int                  `json:"gh_wait,omitempty"`
	// Estimate stages already notified (<issue>|estimate|<value>|<stage>)
	Estimate map[string]bool `json:"estimate,omitempty"`
	// When the dispatcher saw an issue's terminal gone (issue → terminal and Unix ms)
	Gone map[string][]dsGoneAt `json:"gone,omitempty"`
	// The memory count went over the line and memory was notified (false again below the line)
	MemoryOver bool `json:"memory_over,omitempty"`
	// The Chief of Staff's absence: when it began and ended, and how many notifications were queued
	Away *dsAway `json:"away,omitempty"`
}

type dsAway struct {
	Since string `json:"since"`
	Until string `json:"until,omitempty"`
	Count int    `json:"count"`
}

type dsGoneAt struct {
	Agent string `json:"agent"`
	At    int64  `json:"at"`
}

type dsResumed struct {
	From  string `json:"from"`
	Agent string `json:"agent,omitempty"`
}
