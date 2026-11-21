package waiting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
)

// 偽の kanban（ファイルを直に置く）と gate の記録を置いて、本物の Read を動かす（ADR-0042 受け入れ条件 1〜4・6）。

const (
	head = "1111111111111111111111111111111111111111" // 今の head
	old  = "2222222222222222222222222222222222222222" // 前の head
	pr   = "https://github.com/example/sample-site/pull/41"
)

var now = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

type fake struct {
	t    *testing.T
	root string
	log  []string
}

func newFake(t *testing.T, parallel int) *fake {
	t.Helper()
	root := filepath.Join(t.TempDir(), "sample-site")
	f := &fake{t: t, root: root}
	f.write("kanban/config.yaml", "version: 3\nprefix: S\nchief_of_staff: t1\n")
	if parallel > 0 {
		f.write("project.json", fmt.Sprintf(`{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": [], "chief_of_staff": {"parallel": %d}}`, parallel))
	}
	return f
}

func (f *fake) write(rel, body string) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// issue は課題を置く（keys は issue.yaml に足す行）。
func (f *fake) issue(id, status string, keys ...string) {
	body := fmt.Sprintf("id: %s\ntitle: 題 %s\nstatus: %s\ncreated: \"2026-11-21T01:00:00Z\"\n", id, id, status)
	for _, k := range keys {
		body += k + "\n"
	}
	f.write("kanban/issues/"+id+"/issue.yaml", body)
}

// question は questions.yaml に問いを置き、ask が空でなければ library/questions/<n>.json も置く。
func (f *fake) questions(id string, qs ...string) {
	f.write("kanban/issues/"+id+"/questions.yaml", strings.Join(qs, ""))
}

func q(n int, kind, status, asked string) string {
	return fmt.Sprintf("- n: %d\n  text: 問い\n  kind: %s\n  status: %s\n  asked: \"%s\"\n", n, kind, status, asked)
}

func (f *fake) ask(id string, n int, body string) {
	f.write(fmt.Sprintf("kanban/issues/%s/library/questions/%d.json", id, n), body)
}

// gateAsk は gate の問いの JSON（事実に gate の記録の行）。
func (f *fake) gateAsk(id string, n int, sha string) {
	record := filepath.Join(f.root, "kanban/issues", id, "library/gate", sha+".json")
	f.ask(id, n, fmt.Sprintf(`{"kind": "line", "question": "マージするか", "facts": ["delegation: merge=B < tier C", "gate の記録 %s"], "options": [{"label": "オーナーがマージする", "pick": true}, {"label": "直させる"}], "from": "gate"}`, record))
}

func (f *fake) report(id, sha string) {
	f.write("kanban/issues/"+id+"/library/report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, sha, pr))
}

func (f *fake) gate(id, sha, at, decision string, merged bool) {
	f.write(fmt.Sprintf("kanban/issues/%s/library/gate/%s.json", id, sha),
		fmt.Sprintf(`{"at": "%s", "id": "%s", "pr": "%s", "head": "%s", "tier": "C", "decision": "%s", "reasons": ["delegation: merge=B < tier C"], "merged": %t}`, at, id, pr, sha, decision, merged))
}

// row は log.jsonl に行を足す（kanban の CLI の書く形に通す）。
func (f *fake) row(at, by, id, change string) {
	f.log = append(f.log, fmt.Sprintf(`{"at":"%s","by":"%s","id":"%s","change":%s}`, at, by, id, change))
}

func (f *fake) read() Result {
	f.t.Helper()
	f.write("kanban/log.jsonl", ledgertest.Log(f.t, f.log...))
	res, _, err := Read(f.root, now)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := contract.Encode(contract.SchemaWaiting, res); err != nil {
		f.t.Fatalf("出力が schema に合わない: %v\n%s", err, contract.Marshal(res))
	}
	return res
}

// kinds は id ごとの行の種類（問いのある行は <id>#<n>=<kind>）。
func kinds(r Result) []string {
	var out []string
	for _, it := range r.Items {
		name := it.ID
		if it.Question != "" {
			name = it.Question
		}
		out = append(out, name+"="+it.Kind)
	}
	slices.Sort(out)
	return out
}

func want(t *testing.T, r Result, exp ...string) {
	t.Helper()
	slices.Sort(exp)
	if got := kinds(r); !slices.Equal(got, exp) {
		t.Fatalf("items = %v, want %v", got, exp)
	}
}

// 受け入れ条件 1: 3 種の分け方と、入らないもの（7 つの場合）。
func TestKinds(t *testing.T) {
	f := newFake(t, 4)
	// 1. review にいて、今の head について gate の問いが開いた課題 → merge
	f.issue("S-0001", "review", "profile: claude")
	f.report("S-0001", head)
	f.questions("S-0001", q(1, "line", "open", "2026-11-21T04:00:00Z"))
	f.gateAsk("S-0001", 1, head)
	f.gate("S-0001", head, "2026-11-21T03:59:00Z", "needs_human", false)
	// 2. 灯りの種類の問いが開いた課題 → decision
	f.issue("S-0002", "doing", "profile: claude")
	f.questions("S-0002", q(1, "users", "open", "2026-11-21T05:00:00Z"))
	f.ask("S-0002", 1, `{"kind": "users", "question": "CSV だけにするか", "options": [{"label": "CSV だけ", "pick": true}, {"label": "Excel も"}], "from": "issue"}`)
	// 3. 起動できて問いの無いオーナーの課題 → task
	f.issue("S-0003", "todo", "who: person")
	// 4. answered の問い → 入らない
	f.issue("S-0004", "doing", "profile: claude")
	f.questions("S-0004", q(1, "business", "answered", "2026-11-21T05:00:00Z"))
	// 5. scope の問い → 入らない
	f.issue("S-0005", "doing", "profile: claude")
	f.questions("S-0005", q(1, "scope", "open", "2026-11-21T05:00:00Z"))
	// 6. answered の問いを持つオーナーの課題 → 入らない
	f.issue("S-0006", "todo", "who: person")
	f.questions("S-0006", q(1, "business", "answered", "2026-11-21T05:00:00Z"))
	// 7. 前の課題を待つオーナーの課題 → 入らない（日付を待つものも）
	f.issue("S-0007", "todo", "who: person", "after: [S-0002]")
	f.issue("S-0008", "todo", "who: person", "not_before: \"2026-12-01\"")
	r := f.read()
	want(t, r, "S-0001#1=merge", "S-0002#1=decision", "S-0003=task")
	for _, it := range r.Items {
		switch it.Kind {
		case "merge":
			if it.PR != pr || it.Reason != "delegation: merge=B < tier C" || it.Since != "2026-11-21T04:00:00Z" {
				t.Errorf("merge の行 = %+v", it)
			}
		case "decision":
			if it.Ask == nil || it.Ask.Question != "CSV だけにするか" || it.Answerable == nil || !*it.Answerable {
				t.Errorf("decision の行 = %+v", it)
			}
		}
	}
}

// 受け入れ条件 2: マージの依頼は今の head について決め終えたものだけ（7 つの場合）。
func TestMerge(t *testing.T) {
	f := newFake(t, 4)
	at := func(id string) { f.row("2026-11-21T03:00:00Z", "terminal:t9", id, `{"status":"review"}`) }
	// 1. 今の head の approve で merge: person のまま review → merge
	f.issue("S-0001", "review", "profile: claude", "merge: person")
	f.report("S-0001", head)
	f.gate("S-0001", head, "2026-11-21T03:10:00Z", "approve", false)
	at("S-0001")
	// 2. merge: auto を Chief of Staff が書いた → 入らない（dispatcher が自動でマージする途中）
	f.issue("S-0002", "review", "profile: claude", "merge: auto")
	f.report("S-0002", head)
	f.gate("S-0002", head, "2026-11-21T03:10:00Z", "approve", false)
	f.row("2026-11-21T01:00:00Z", "terminal:t1", "S-0002", `{"merge":"auto"}`)
	at("S-0002")
	// 2'. merge: auto を Chief of Staff でない端末が書いた → merge
	f.issue("S-0003", "review", "profile: claude", "merge: auto")
	f.report("S-0003", head)
	f.gate("S-0003", head, "2026-11-21T03:10:00Z", "approve", false)
	f.row("2026-11-21T01:00:00Z", "terminal:t7", "S-0003", `{"merge":"auto"}`)
	at("S-0003")
	// 3. doing に戻った課題は gate の問いが開いていても入らない
	f.issue("S-0004", "doing", "profile: claude")
	f.report("S-0004", head)
	f.questions("S-0004", q(1, "line", "open", "2026-11-21T04:00:00Z"))
	f.gateAsk("S-0004", 1, head)
	// 4. 前の head の gate の問いしか無い review の課題 → 入らない
	f.issue("S-0005", "review", "profile: claude")
	f.report("S-0005", head)
	f.questions("S-0005", q(1, "line", "open", "2026-11-21T04:00:00Z"))
	f.gateAsk("S-0005", 1, old)
	at("S-0005")
	// 5. 前の head の approve の記録しか無い review の課題 → 入らない
	f.issue("S-0006", "review", "profile: claude", "merge: person")
	f.report("S-0006", head)
	f.gate("S-0006", old, "2026-11-21T03:10:00Z", "approve", false)
	at("S-0006")
	// 5'. 今の head の approve でも、今の review に入る前の記録 → 入らない
	f.issue("S-0007", "review", "profile: claude", "merge: person")
	f.report("S-0007", head)
	f.gate("S-0007", head, "2026-11-21T02:00:00Z", "approve", false)
	at("S-0007")
	// 6. gate の記録にマージ済みとある課題は、片付けに失敗した gate の問いがあっても入らない
	f.issue("S-0008", "review", "profile: claude", "merge: auto")
	f.report("S-0008", head)
	f.gate("S-0008", head, "2026-11-21T03:10:00Z", "approve", true)
	f.questions("S-0008", q(1, "irreversible", "open", "2026-11-21T04:00:00Z"))
	f.gateAsk("S-0008", 1, head)
	at("S-0008")
	// 7. review にいて報告に PR がある Chief Engineer の課題 → merge（枠には数えない）
	f.issue("S-0009", "review", "profile: chief-engineer")
	f.report("S-0009", head)
	at("S-0009")
	r := f.read()
	want(t, r, "S-0001=merge", "S-0003=merge", "S-0009=merge")
	if r.Slots.Waiting != 2 || r.Slots.Working != 6 {
		t.Errorf("slots = %+v, want waiting 2（S-0001・S-0003）, working 6", r.Slots)
	}
}

// 受け入れ条件 3: holds は直接と後ろで待つ済んでいない課題。並びは holds の多い順、同じなら古い順。
func TestHoldsAndOrder(t *testing.T) {
	f := newFake(t, 4)
	f.issue("S-0001", "todo", "who: person")
	f.issue("S-0002", "todo", "after: [S-0001]")
	f.issue("S-0003", "todo", "after: [S-0002]")
	f.issue("S-0004", "done", "after: [S-0001]") // 済んだものは入れない
	f.issue("S-0005", "todo", "after: [S-0003, S-0001]")
	f.write("kanban/issues/S-0006/issue.yaml", "id: S-0006\ntitle: 古い\nstatus: todo\nwho: person\ncreated: \"2026-11-20T01:00:00Z\"\n")
	f.write("kanban/issues/S-0007/issue.yaml", "id: S-0007\ntitle: 新しい\nstatus: todo\nwho: person\ncreated: \"2026-11-21T09:00:00Z\"\n")
	f.issue("S-0008", "todo", "after: [S-0007]")
	f.write("kanban/issues/S-0009/issue.yaml", "id: S-0009\ntitle: 後ろ無しの古い\nstatus: todo\nwho: person\ncreated: \"2026-11-19T01:00:00Z\"\n")
	r := f.read()
	var order []string
	for _, it := range r.Items {
		order = append(order, it.ID)
	}
	if !slices.Equal(order, []string{"S-0001", "S-0007", "S-0009", "S-0006"}) {
		t.Fatalf("並び = %v, want S-0001（3件）, S-0007（1件）, S-0009（0件・古い）, S-0006（0件）", order)
	}
	if h := r.Items[0].Holds; !slices.Equal(h, []string{"S-0002", "S-0003", "S-0005"}) {
		t.Errorf("S-0001 の holds = %v, want [S-0002 S-0003 S-0005]", h)
	}
	if h := r.Items[3].Holds; h == nil || len(h) != 0 {
		t.Errorf("後ろの無い行の holds = %#v, want 空の並び", h)
	}
}

// 受け入れ条件 4: 枠の数え方（6 つの場合）。
func TestSlots(t *testing.T) {
	waitingOn := func(f *fake, id string) {
		f.issue(id, "review", "profile: claude", "merge: person")
		f.report(id, head)
		f.gate(id, head, "2026-11-21T03:10:00Z", "approve", false)
		f.row("2026-11-21T03:00:00Z", "terminal:t9", id, `{"status":"review"}`)
	}
	t.Run("数えない課題とオーナー待ちの分け方", func(t *testing.T) {
		f := newFake(t, 4)
		f.issue("S-0001", "doing", "profile: claude")
		f.issue("S-0001-r1", "doing", "profile: codex-review", "parent: S-0001") // レビュアーの子課題
		f.issue("S-0002", "review", "profile: claude-review", "parent: S-0001")  // レビュアーの子課題（番号の形でない）
		f.issue("S-0003", "doing", "profile: chief-engineer")                    // Chief Engineer の課題
		f.issue("S-0004", "doing", "who: person")                                // オーナーの課題
		waitingOn(f, "S-0005")
		// gate の問いが開いたまま doing に戻った課題は working
		f.issue("S-0006", "doing", "profile: claude")
		f.report("S-0006", head)
		f.questions("S-0006", q(1, "line", "open", "2026-11-21T04:00:00Z"))
		f.gateAsk("S-0006", 1, head)
		r := f.read()
		if r.Slots != (Slots{Limit: 4, Working: 2, Waiting: 1, Free: 2}) {
			t.Fatalf("slots = %+v, want limit 4, working 2（S-0001・S-0006）, waiting 1（S-0005）, free 2", r.Slots)
		}
	})
	t.Run("判断の問いを出した担当は waiting", func(t *testing.T) {
		f := newFake(t, 3)
		f.issue("S-0001", "doing", "profile: claude")
		f.questions("S-0001", q(1, "promise", "open", "2026-11-21T04:00:00Z"))
		r := f.read()
		if r.Slots != (Slots{Limit: 3, Working: 0, Waiting: 1, Free: 3}) {
			t.Fatalf("slots = %+v", r.Slots)
		}
	})
	t.Run("waiting が limit 以上なら waiting_full で free は 0", func(t *testing.T) {
		f := newFake(t, 2)
		waitingOn(f, "S-0001")
		waitingOn(f, "S-0002")
		r := f.read()
		if r.Slots != (Slots{Limit: 2, Working: 0, Waiting: 2, Free: 0, WaitingFull: true}) {
			t.Fatalf("slots = %+v", r.Slots)
		}
	})
	t.Run("working が limit を超えても free は 0 より小さくならない", func(t *testing.T) {
		f := newFake(t, 2)
		for _, id := range []string{"S-0001", "S-0002", "S-0003"} {
			f.issue(id, "doing", "profile: claude")
		}
		r := f.read()
		if r.Slots != (Slots{Limit: 2, Working: 3, Waiting: 0, Free: 0}) {
			t.Fatalf("slots = %+v", r.Slots)
		}
	})
	t.Run("parallel が無ければ limit は 2", func(t *testing.T) {
		f := newFake(t, 0)
		f.issue("S-0001", "doing", "profile: claude")
		r := f.read()
		if r.Slots != (Slots{Limit: 2, Working: 1, Waiting: 0, Free: 1}) {
			t.Fatalf("slots = %+v", r.Slots)
		}
	})
	t.Run("project.json に chief_of_staff が無くても limit は 2", func(t *testing.T) {
		f := newFake(t, 0)
		f.write("project.json", `{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": []}`)
		r := f.read()
		if r.Slots.Limit != 2 {
			t.Fatalf("slots = %+v", r.Slots)
		}
	})
}

// 受け入れ条件 6: 課題 500 件の kanban で 1 秒以内に返る。GitHub も git も呼ばない（偽の gh・git を PATH に置き、呼ばれたら印を残す）。
func TestFastAndOffline(t *testing.T) {
	f := newFake(t, 4)
	for i := 1; i <= 500; i++ {
		id := fmt.Sprintf("S-%04d", i)
		switch i % 5 {
		case 0:
			f.issue(id, "done")
		case 1:
			f.issue(id, "review", "profile: claude", "merge: person", fmt.Sprintf("after: [S-%04d]", max(i-1, 1)))
			f.report(id, head)
			f.gate(id, head, "2026-11-21T03:10:00Z", "approve", false)
			f.row("2026-11-21T03:00:00Z", "terminal:t9", id, `{"status":"review"}`)
		case 2:
			f.issue(id, "doing", "profile: claude")
			f.questions(id, q(1, "users", "open", "2026-11-21T04:00:00Z"))
		case 3:
			f.issue(id, "todo", "who: person")
		default:
			f.issue(id, "todo", fmt.Sprintf("after: [S-%04d]", i-1))
		}
	}
	f.write("kanban/log.jsonl", ledgertest.Log(t, f.log...))
	bin := t.TempDir()
	mark := filepath.Join(t.TempDir(), "called")
	for _, name := range []string{"gh", "git"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho \"$0 $*\" >>"+mark+"\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KANBAN_ROOT", f.root)
	var out, errb bytes.Buffer
	start := time.Now()
	code := Main([]string{"--json"}, nil, &out, &errb)
	took := time.Since(start)
	if code != 0 {
		t.Fatalf("andon waiting --json: 終了コード %d %s", code, errb.String())
	}
	if took > time.Second {
		t.Errorf("課題 500 件で %v かかった（1 秒以内）", took)
	}
	if b, err := os.ReadFile(mark); err == nil {
		t.Errorf("gh か git を呼んだ:\n%s", b)
	}
	var r Result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || len(r.Items) != 300 {
		t.Fatalf("items = %d（%v）, want 300（merge 100・decision 100・task 100）", len(r.Items), err)
	}
	t.Logf("課題 500 件を %v で数えた", took)
}

// 人が読む出力は枠の行と、行ごとの種類・課題・後ろで待つ数を出す。
func TestText(t *testing.T) {
	f := newFake(t, 4)
	f.issue("S-0001", "todo", "who: person")
	f.issue("S-0002", "todo", "after: [S-0001]")
	out := Text(f.read())
	for _, s := range []string{"sample-site の枠: 上限 4、動いている 0、オーナー待ち 0、空き 4", "手作業 S-0001（題 S-0001）", "後ろで 1 件が待つ（S-0002）"} {
		if !strings.Contains(out, s) {
			t.Errorf("出力に %q が無い:\n%s", s, out)
		}
	}
}
