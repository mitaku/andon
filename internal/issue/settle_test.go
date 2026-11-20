package issue

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
)

// 片付けを止めた理由の文は、記録に残した前の文の古い語を新しい語に読み替えてから比べる（ADR-0040 決めたこと 7）。
func TestReadOld(t *testing.T) {
	for _, c := range []struct{ old, want string }{
		{"「ゲート」の外の問いが開いている: A-1#1", "「gate」の外の問いが開いている: A-1#1"},
		{"ゲートの外の問いが開いている: A-1#1・A-1#2", "gate の外の問いが開いている: A-1#1・A-1#2"},
		{"ゲートの問い A-1#1 を閉じられない: answer: exit status 1", "gate の問い A-1#1 を閉じられない: answer: exit status 1"},
		{"kanban done が断った（台帳の問いが開いている）", "kanban done が断った（kanban の問いが開いている）"},
		{"前の版の控え", "前のバージョンの控え"},
		{"最新版・初版・Go版は読み替えない", "最新版・初版・Go版は読み替えない"},
		{"gate の外の問いが開いている: A-1#1", "gate の外の問いが開いている: A-1#1"}, // 新しい語の文はそのまま
		{"", ""},
	} {
		if got := readOld(c.old); got != c.want {
			t.Errorf("readOld(%q) = %q, want %q", c.old, got, c.want)
		}
	}
}

// 記録に残した理由が古い語で、今の理由が同じ中身の新しい語なら、通知 merged（held）を出し直さない。中身が違えば出す。
func TestSettleHeldOldWords(t *testing.T) {
	for _, c := range []struct {
		name, held string
		notify     bool
	}{
		{"古い語の同じ理由", "ゲートの外の問いが開いている: T-0001#1", false},
		{"新しい語の同じ理由", "gate の外の問いが開いている: T-0001#1", false},
		{"違う理由", "ゲートの外の問いが開いている: T-0001#2", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.task["status"] = "review"
			f.task["repo"] = "github.com/example/app"
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task}))
			f.write("kanban/log.jsonl", "")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("questions.json", `[{"id": "T-0001", "n": 1, "status": "open"}]`) // 問いの JSON が無い: gate の外の問い
			pr := "https://github.com/example/app/pull/7"
			path := settlePath(filepath.Base(f.root))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			rec := map[string]contract.Settled{"T-0001": {PR: pr, Commit: f.head, State: "MERGED", Head: f.head, Sha: f.head, Held: c.held}}
			if err := contract.WriteState(path, rec); err != nil {
				t.Fatal(err)
			}
			var out, errb bytes.Buffer
			if rc := Main([]string{"settle", "T-0001"}, strings.NewReader(""), &out, &errb); rc != 0 {
				t.Fatalf("settle の終了コード %d: %s", rc, errb.String())
			}
			var res contract.Settle
			if err := json.Unmarshal(out.Bytes(), &res); err != nil {
				t.Fatalf("settle の出力が読めない: %v: %s", err, out.String())
			}
			if res.State != "held" || (len(res.Notify) > 0) != c.notify {
				t.Fatalf("state %q・通知 %v（通知を出すのは %v のとき）", res.State, res.Notify, c.notify)
			}
			if b, _ := os.ReadFile(path); c.notify && !strings.Contains(string(b), "gate の外の問いが開いている: T-0001#1") {
				t.Fatalf("知らせたときは今の理由を記録に残す: %s", b)
			}
		})
	}
}
