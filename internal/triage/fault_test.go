package triage

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ledgercli "andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

// トリアージの apply は記録を先に置くので打ち直せない（「記録済み」で断る）。止まったあとの続きは andon issue start が受け持つ:
// 足した担当の課題を記録から辿れれば（ledger.TriageMade）、無い予約キーと照合の外れた既定を書いてから起動する
// （書く所で止まっても揃うことは internal/issue の TestStartFillConvergesAfterAnyStop が見る）。
// ここでは、kanban に書く所（担当の課題の add・予約キーの set、判断のオーナーの課題の add・set・ask の、前・ファイルと log.jsonl の
// 間・後）のどこで止まっても、kanban にある担当の課題を記録から辿れることと、記録の作った課題が kanban にあることを見る。
func TestApplyLeavesAddedIssuesTraceableAfterAnyStop(t *testing.T) {
	const repo = "github.com/example/sample-site"
	const title = "ボタンを足す"
	f := ledgertest.NewFault(t)
	var l *ledger
	base := ""
	items := func() []item {
		zeros := strings.Repeat("0", 40)
		return []item{
			{"key": "sample-site:T-0001:next:" + zeros, "decision": "fix", "reason": "直す",
				"issue": map[string]any{"title": title, "repo": repo, "profile": "claude", "review": "on", "brief": "ボタンが無い"}},
			{"key": "sample-site:T-0001:question:" + zeros, "decision": "person", "kind": "users", "question": "どちらの画面にするか",
				"options": []any{"一覧", "表"}, "reason": "使い勝手の好み"},
		}
	}
	setup := func(t *testing.T) {
		t.Setenv("HC_TERMINAL_ID", "t5")
		root := filepath.Join(t.TempDir(), "sample-site")
		f.Kanban(root, "", nil, "init", "--root", root, "--prefix", "T", "--repo", repo)
		f.Kanban(root, "", nil, "config", "set", "chief_of_staff=t5")
		f.Kanban(root, "", nil, "add", "トリアージ")
		l = &ledger{cli: "kanban", root: root, slug: "sample-site", dir: filepath.Join(root, "kanban"), now: time.Now()}
		base = filepath.Join(l.dir, "library", "triage", "2026-11-23-T-0001")
	}
	run := func() error {
		data := obj{"goal": "未担当を減らす"}
		return l.applyItems("T-0001", "2026-11-23", base, data, items(), nil, nil, io.Discard)
	}
	after := func(t *testing.T, n int) {
		b, err := os.ReadFile(base + ".json")
		if err != nil {
			t.Fatalf("止まったあと記録が無い: %v", err)
		}
		var rec struct {
			Items []struct {
				Decision string `json:"decision"`
				Created  string `json:"created_issue"`
			} `json:"items"`
		}
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
		cli := ledgercli.At(l.root)
		issues, err := cli.List()
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		rows := cli.Log()
		for _, is := range issues {
			ids[is.Str("id")] = true
			if is.Str("title") != title {
				continue
			}
			if name, _ := cli.TriageMade(rows, is); name != "T-0001" {
				t.Errorf("%d 番目の所で止めたあと、kanban にある担当の課題 %s を記録から辿れない（andon issue start が予約キーを埋めずに断る）:\n%s", n, is.Str("id"), b)
			}
		}
		for _, it := range rec.Items {
			if it.Created != "" && !ids[it.Created] {
				t.Errorf("%d 番目の所で止めたあと、記録の作った課題 %s が kanban に無い", n, it.Created)
			}
		}
	}
	f.Converge(t, ledgertest.Case{Setup: setup, Run: run, After: after})
}
