package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon/internal/ledger"
)

// 待つ相手のうち PR を成果にする課題がマージされずに done になったものを gone に、確かめられなかったものを unsure に返す。
// マージで片付いた待つ相手（結果の記録・gate の記録・gh の MERGED）は返さない。PR を成果にしない待つ相手は見ない。
func TestWaitedGone(t *testing.T) {
	p := routeProject(t)
	bin := t.TempDir()
	// 偽の gh: PR 1 は MERGED、2 は CLOSED、3 は OPEN、4 は落ちる。打った PR を calls に書く
	calls := filepath.Join(bin, "calls")
	gh := "#!/bin/sh\necho \"$3\" >> " + calls + "\ncase \"$3\" in\n" +
		"1) echo '{\"state\":\"MERGED\",\"mergeCommit\":{\"oid\":\"x\"},\"mergedAt\":\"2026-11-20T00:00:00Z\"}' ;;\n" +
		"2) echo '{\"state\":\"CLOSED\",\"mergeCommit\":null,\"mergedAt\":null}' ;;\n" +
		"3) echo '{\"state\":\"OPEN\",\"mergeCommit\":null,\"mergedAt\":null}' ;;\n" +
		"*) echo 'HTTP 502' >&2; exit 1 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := "github.com/example/app"
	write := func(id, file, body string) {
		dir := p.l.Library(id)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report := func(id string, pr int) {
		write(id, "report.json", `{"commit":"9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08","pr":"https://github.com/example/app/pull/`+string(rune('0'+pr))+`"}`)
	}
	done := func(id string, kv ...string) {
		t2 := ledger.Issue{"id": id, "status": "done", "profile": "claude", "repo": repo}
		for i := 0; i+1 < len(kv); i += 2 {
			t2[kv[i]] = kv[i+1]
		}
		p.tasks = append(p.tasks, t2)
	}
	// マージで片付いた待つ相手
	done("E-0001")
	write("E-0001", "outcome.json", `{"result":"merged"}`) // 結果の記録
	done("E-0002")
	report("E-0002", 4) // gh は落ちるが、gate の記録の merged が先に答える
	write("E-0002", "gate/9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08.json", `{"head":"9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08","at":"2026-11-20T00:00:00Z","tier":"B","decision":"approve","reasons":[],"merged":true}`)
	done("E-0003")
	report("E-0003", 1) // gh で MERGED（オーナーがマージした）
	// やめになった待つ相手
	done("E-0011")
	write("E-0011", "outcome.json", `{"result":"closed"}`)
	done("E-0012") // report.json が無い（PR を出さずに片付けた）
	done("E-0013")
	report("E-0013", 2) // CLOSED
	done("E-0014")
	report("E-0014", 3) // OPEN のまま片付けた
	// 確かめられない待つ相手
	done("E-0021")
	report("E-0021", 4)
	// PR を成果にしない待つ相手（見ない）
	p.tasks = append(p.tasks,
		ledger.Issue{"id": "E-0031", "status": "done", "profile": "claude"},                                            // repo の無い課題
		ledger.Issue{"id": "E-0032", "status": "done", "profile": "claude", "repo": repo, "deliverable": "candidates"}, // 調べる課題
		ledger.Issue{"id": "E-0033", "status": "done", "who": "person", "repo": repo},                                  // オーナーの課題
	)

	after := func(ids ...string) ledger.Issue {
		a := make([]any, len(ids))
		for i, s := range ids {
			a[i] = s
		}
		return ledger.Issue{"id": "E-0100", "status": "todo", "after": a}
	}
	cases := []struct {
		name         string
		after        []string
		gone, unsure []string
	}{
		{"マージで片付いた待つ相手は返さない", []string{"E-0001", "E-0002", "E-0003"}, nil, nil},
		{"やめになった待つ相手を返す", []string{"E-0001", "E-0011", "E-0012", "E-0013", "E-0014"}, []string{"E-0011", "E-0012", "E-0013", "E-0014"}, nil},
		{"確かめられない待つ相手は unsure", []string{"E-0021", "E-0013"}, []string{"E-0013"}, []string{"E-0021"}},
		{"PR を成果にしない待つ相手は見ない", []string{"E-0031", "E-0032", "E-0033"}, nil, nil},
		{"待つ相手が無い", nil, nil, nil},
	}
	for _, c := range cases {
		gone, unsure := p.waitedGone(after(c.after...))
		if !slices.Equal(gone, c.gone) || !slices.Equal(unsure, c.unsure) {
			t.Errorf("%s: gone %v unsure %v, want %v %v", c.name, gone, unsure, c.gone, c.unsure)
		}
	}
	// 手元の記録（結果の記録・gate の記録・report.json の無いこと）で決まる待つ相手には gh を打たない。
	// 打つのは E-0003（1）・E-0013（2）・E-0014（3）・E-0021（4）・E-0013（2）の順だけ（E-0002 の PR 4 は打たない）
	b, _ := os.ReadFile(calls)
	if got := strings.Fields(string(b)); !slices.Equal(got, []string{"1", "2", "3", "4", "2"}) {
		t.Fatalf("gh を打った PR が違う: %q", got)
	}

	// gh の上限で止めている間は gh を打たずに確かめられないとする
	os.Remove(calls)
	p.st.GHReset = "2999-01-01T00:00:00Z"
	if gone, unsure := p.waitedGone(after("E-0013", "E-0012")); !slices.Equal(gone, []string{"E-0012"}) || !slices.Equal(unsure, []string{"E-0013"}) {
		t.Fatalf("gh の上限の間: gone %v unsure %v", gone, unsure)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Fatal("gh の上限の間に gh を打った")
	}
}

// ready の行: 待つ相手がやめになったら「待つ相手 <id> がやめになった」、確かめられなければその旨の1句が付く。
// 無ければ今どおりタイトルだけ。控えから読み直した通知（[]any）も同じ文に描く。
func TestReadyLine(t *testing.T) {
	cases := []struct {
		detail map[string]any
		want   string
	}{
		{map[string]any{"title": "続き"}, "E-0100 ready: 「続き」"},
		{map[string]any{"title": "続き", "routine": "nightly"}, "E-0100 ready: 「続き」（ルーチン nightly）"},
		{map[string]any{"title": "続き", "abandoned": []string{"E-0011"}}, "E-0100 ready: 「続き」。待つ相手 E-0011 がやめになった"},
		{map[string]any{"title": "続き", "abandoned": []string{"E-0011", "E-0012"}, "unchecked": []string{"E-0021"}},
			"E-0100 ready: 「続き」。待つ相手 E-0011、E-0012 がやめになった。待つ相手 E-0021 がマージされたか確かめられない"},
		{map[string]any{"title": "続き", "routine": "nightly", "abandoned": []any{"E-0011"}}, "E-0100 ready: 「続き」（ルーチン nightly）。待つ相手 E-0011 がやめになった"},
	}
	for _, c := range cases {
		if got := line(Notification{Issue: "E-0100", Kind: "ready", Detail: c.detail}); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
	// 控えに保存して読み直した通知も同じ文
	n := Notification{Issue: "E-0100", Kind: "ready", Detail: map[string]any{"title": "続き", "abandoned": []string{"E-0011"}}}
	b, _ := json.Marshal(n)
	var back Notification
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if line(back) != line(n) {
		t.Fatalf("読み直した通知の文が違う: %q / %q", line(back), line(n))
	}
}
