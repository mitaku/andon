package dispatcher

import (
	"strings"
	"testing"

	"andon/internal/ledger"
)

// ready の差のつなぎ: 待つ相手が done になって todo へ戻った課題（kanban ready に新しく出た課題）の通知 ready に、
// マージされずに done になった待つ相手を abandoned で添える。マージで片付いた待つ相手と、PR を成果にしない待つ相手（repo の無い課題）
// だけを待つ課題は題だけ。初めて見る kanban は控えるだけで、同じ課題を二度知らせない。
// （kanban done が queued の課題を todo へ移すことは cmd/kanban の TestDoneFreeingQueuedFires が見る。）
func TestReadyDiffAddsAbandoned(t *testing.T) {
	sha := "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"
	tools := newScripted(t, "kanban", "gh")
	var log strings.Builder
	p := gluedProject(t, &log)
	repo := "github.com/example/app"
	p.tasks = []ledger.Issue{
		{"id": "W-0001", "status": "done", "profile": "claude", "repo": repo}, // PR を出さずにやめた（report.json が無い）
		{"id": "W-0002", "status": "done", "profile": "claude", "repo": repo}, // PR を閉じてやめた
		{"id": "W-0003", "status": "done", "profile": "claude", "repo": repo}, // マージで片付いた（gate の記録の merged）
		{"id": "W-0004", "status": "done", "profile": "claude"},               // 調べて終えた（repo の無い課題）
	}
	writeLib(t, p, "W-0002", "report.json", `{"commit":"`+sha+`","pr":"https://github.com/example/app/pull/8"}`)
	writeLib(t, p, "W-0003", "gate/"+sha+".json", `{"head":"`+sha+`","at":"2026-11-20T00:00:00Z","tier":"B","decision":"approve","reasons":[],"merged":true}`)
	tools.reply("gh pr view 8 --repo example/app --json state,mergeCommit,mergedAt", `{"state":"CLOSED","mergeCommit":null,"mergedAt":null}`)
	tools.reply("kanban ready --json", `[{"id":"Q-0009","status":"todo","title":"前からある"}]`)
	p.readReady() // 初めて見る kanban: 控えるだけ
	if len(p.st.Pending) != 0 {
		t.Fatalf("初めて見る kanban で知らせた: %+v", p.st.Pending)
	}
	tools.reply("kanban ready --json", `[{"id":"Q-0009","status":"todo","title":"前からある"},`+
		`{"id":"Q-0001","status":"todo","title":"やめた相手を待つ","after":["W-0001","W-0003"]},`+
		`{"id":"Q-0002","status":"todo","title":"閉じた相手を待つ","after":["W-0002"]},`+
		`{"id":"Q-0003","status":"todo","title":"マージした相手を待つ","after":["W-0003","W-0004"]}]`)
	tools.calls()
	p.readReady()
	got := map[string]string{}
	for _, n := range p.st.Pending {
		if n.Kind != "ready" {
			t.Fatalf("ready でない通知を積んだ: %+v", n)
		}
		got[n.Issue] = asJSON(t, n.Detail)
	}
	want := map[string]string{
		"Q-0001": `{"abandoned":["W-0001"],"title":"やめた相手を待つ"}`,
		"Q-0002": `{"abandoned":["W-0002"],"title":"閉じた相手を待つ"}`,
		"Q-0003": `{"title":"マージした相手を待つ"}`,
	}
	if len(got) != len(want) {
		t.Fatalf("知らせた課題が違う（前回もあった ready を拾った？）: %v", got)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s の detail が %s（%s のはず）", id, got[id], w)
		}
	}
	if err := p.st.Save(p.l.Slug); err != nil {
		t.Fatalf("積んだ通知が書く前の照合を通らない: %v", err)
	}
	if !hasCall(tools.calls(), "gh pr view 8 --repo example/app --json state,mergeCommit,mergedAt") {
		t.Error("閉じた PR の状態を gh に聞かない")
	}
	p.st.Pending = nil
	p.readReady()
	if len(p.st.Pending) != 0 {
		t.Errorf("同じ ready を二度知らせた: %+v", p.st.Pending)
	}
}
