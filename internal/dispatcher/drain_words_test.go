package dispatcher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// wordsEnv は Drain を本物の置き場から切り離す（控えは一時の置き場、hc も andon も無い）。
func wordsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
}

// stopDrain は Stop フックと同じく、入力の JSON を stdin に渡して Drain を打ち、additionalContext を返す（出力が無ければ空）。
func stopDrain(t *testing.T, in map[string]any) string {
	t.Helper()
	in["hook_event_name"] = "Stop"
	b, _ := json.Marshal(in)
	var out, errb bytes.Buffer
	if code := Drain([]string{"--project", "demo"}, bytes.NewReader(b), &out, &errb); code != 0 {
		t.Fatalf("Drain が %d で終わった: %s", code, errb.String())
	}
	if out.Len() == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("Drain の出力が JSON でない: %s", out.String())
	}
	return asContext(m)
}

// 根拠の出来事: CoS がオーナーへの返事に「検証の地図」と書いた。Stop で言い換えを渡して言い直させ、
// 同じ turn の言い直した返事（続きの turn。同じ prompt_id）に語が残っても、もう止めない。次の turn ではまた探す。
func TestDrainOwnerWordsOncePerTurn(t *testing.T) {
	wordsEnv(t)
	msg := "検証の地図に機能を足しました。灯りはついていません。"
	got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p1", "stop_hook_active": false, "last_assistant_message": msg})
	for _, want := range []string{"<owner-words>", "「地図」→「当てたあとに確かめる機能の一覧」", "「灯り」→「オーナーの判断待ち」", "</owner-words>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("言い換えを渡さない（%s が無い）: %q", want, got)
		}
	}
	if got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p1", "stop_hook_active": true, "last_assistant_message": "検証の地図は機能の一覧のことです"}); got != "" {
		t.Fatalf("同じ turn で2回止めた: %q", got)
	}
	if got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p2", "stop_hook_active": false, "last_assistant_message": "レベルC なのでマージを待ちます"}); !strings.Contains(got, "「レベル C」→「オーナーがマージ」") {
		t.Fatalf("次の turn で探さない（空白の無いレベルC）: %q", got)
	}
}

// 語が無い返事と、コードの囲みの中だけにある語では止めない。
func TestDrainOwnerWordsSkipsPlainAndCode(t *testing.T) {
	wordsEnv(t)
	for i, msg := range []string{
		"PR をマージしました。",
		"`.claude/skills/verify-andon/features/地図.md` と `灯り` は識別子です。",
		"```\nレベル C\n```\n片付けました。",
		"アクセシビリティはレベル AA を満たしています。", // 英字が続く語は別の語（WCAG のレベル AA）
	} {
		if got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p" + string(rune('a'+i)), "last_assistant_message": msg}); got != "" {
			t.Errorf("止めてはいけない返事で止めた: %q → %q", msg, got)
		}
	}
}

// prompt_id を渡さない入力では、続きの turn（stop_hook_active）で探さない（言い直しを止め続けない）。
func TestDrainOwnerWordsWithoutPromptID(t *testing.T) {
	wordsEnv(t)
	if got := stopDrain(t, map[string]any{"session_id": "s1", "stop_hook_active": false, "last_assistant_message": "灯りがつきました"}); !strings.Contains(got, "<owner-words>") {
		t.Fatalf("prompt_id の無い turn の終わりで探さない: %q", got)
	}
	if got := stopDrain(t, map[string]any{"session_id": "s1", "stop_hook_active": true, "last_assistant_message": "灯りがつきました"}); got != "" {
		t.Fatalf("prompt_id の無い続きの turn で止めた: %q", got)
	}
}

// turn を控えに書けなければ探さない（書けないまま止めると、言い直すたびに止め続ける）。
func TestDrainOwnerWordsNeedsRecord(t *testing.T) {
	wordsEnv(t)
	f := t.TempDir() + "/file"
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", f) // 置き場がファイルで、控えも turn も書けない
	for i := 0; i < 2; i++ {
		if got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p1", "last_assistant_message": "灯りがつきました"}); got != "" {
			t.Fatalf("turn を書けないのに止めた（%d 回目）: %q", i+1, got)
		}
	}
}

// 控えの lock が取れなくても言い換えは渡す。待ちの通知があれば、同じ additionalContext の <owner-words> の後ろに <news> を添える。
func TestDrainOwnerWordsWithLockAndNews(t *testing.T) {
	wordsEnv(t)
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	unlock, err := lockState("demo", 0)
	if err != nil {
		t.Fatal(err)
	}
	got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p1", "last_assistant_message": "地図に足しました"})
	unlock()
	if !strings.Contains(got, "<owner-words>") {
		t.Fatalf("lock が取れないと言い換えを渡さない: %q", got)
	}
	st, _, _ := LoadState("demo")
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: "demo", Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
	if err := st.Save("demo"); err != nil {
		t.Fatal(err)
	}
	got = stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": "p2", "last_assistant_message": "レベル B でした"})
	w, n := strings.Index(got, "</owner-words>"), strings.Index(got, "<news")
	if w < 0 || n < 0 || w > n || !strings.Contains(got, "A-0001 report") {
		t.Fatalf("<owner-words> の後ろに <news> を添えない: %q", got)
	}
}

// 用語集の「オーナーへはふだんの言葉で言う語」の表の行ごとに、その語を書いた返事で、表の言い方を渡す
// （用語集に足した行を drain.go の写しに足し忘れると落ちる。逆向きは tests/package/08-writing.sh が見る）。
func TestDrainOwnerWordsMatchGlossary(t *testing.T) {
	wordsEnv(t)
	f, err := os.Open("../../package/skills/glossary/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	in, rows := false, 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			in = line == "## オーナーへはふだんの言葉で言う語"
			continue
		}
		if !in || !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| 語 |") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if len(cells) != 2 {
			t.Fatalf("表の行の形が違う: %q", line)
		}
		rows++
		word, say := cells[0], cells[1]
		for i, w := range []string{word, strings.ReplaceAll(word, " ", "")} {
			got := stopDrain(t, map[string]any{"session_id": "s1", "prompt_id": fmt.Sprintf("row-%d-%d", rows, i), "last_assistant_message": "それは" + w + "です。"})
			if !strings.Contains(got, "「"+word+"」→「"+say+"」") {
				t.Errorf("用語集の行 %q を、返事 %q で渡さない: %q", line, w, got)
			}
		}
	}
	if rows == 0 {
		t.Fatal("用語集に「オーナーへはふだんの言葉で言う語」の表の行が無い")
	}
}
