package issue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger/ledgertest"
)

const fakeDeck = `#!/bin/sh
case "$1" in
  root) echo "$FAKE_ROOT" ;;
  show) cat "$FAKE_ROOT/show-$2.json" ;;
  list) cat "$FAKE_ROOT/list.json" ;;
  questions) cat "$FAKE_ROOT/questions.json" 2>/dev/null || true ;;
  review) echo "$@" > "$FAKE_ROOT/reviewed-$2"; echo "$2 updated" ;;
  report) cat > "$FAKE_ROOT/reported-$2"; echo "$2 reported" ;;
  *) exit 9 ;;
esac
`

const fakeGH = `#!/bin/sh
case "$1 $2" in
  "api --paginate") cat "$FAKE_ROOT/prfiles"; exit 0 ;;
  "pr diff") cat "$FAKE_ROOT/prdiff" 2>/dev/null; exit 0 ;;
esac
echo x >> "$FAKE_ROOT/gh-calls"
n=$(cat "$FAKE_ROOT/prhead-lag" 2>/dev/null || echo 0)
if [ "$n" -gt 0 ]; then
  echo $((n - 1)) > "$FAKE_ROOT/prhead-lag"
  cat "$FAKE_ROOT/prhead-old"
  exit 0
fi
cat "$FAKE_ROOT/prhead"
`

type fixture struct {
	t         *testing.T
	root      string
	wt        string
	head      string
	task      map[string]any
	reviewer  map[string]any
	by        string
	report    string
	findings  string
	prHead    string
	prFile    string
	questions string
	state     bool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	for _, c := range []string{"git", "sh"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skipf("%s が無い", c)
		}
	}
	dir := t.TempDir()
	f := &fixture{t: t, root: filepath.Join(dir, "proj"), wt: filepath.Join(dir, "wt"), by: "terminal:t5"}
	bin := filepath.Join(dir, "bin")
	for _, d := range []string{bin, filepath.Join(f.root, "kanban"), f.wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(fakeDeck), 0o755)
	os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755)
	os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", f.wt}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "x")
	f.head = git("rev-parse", "HEAD")
	f.prHead = f.head
	f.task = map[string]any{"id": "T-0001", "status": "doing", "profile": "claude", "review": "on", "must_fix": "major", "worktree": f.wt}
	f.reviewer = map[string]any{"id": "T-0002", "parent": "T-0001", "status": "review", "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
	f.report = `{"commit": "` + f.head + `", "pr": "https://github.com/example/app/pull/7"}`
	f.findings = f.head
	f.prFile = "docs/a.md"
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, file, _, _ := runtime.Caller(0)
	t.Setenv("ANDON_HOME", filepath.Join(filepath.Dir(file), "..", "..", "package"))
	t.Setenv("ANDON_WORKSPACE", filepath.Join(dir, "workspace"))
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	wait := prHeadWait
	prHeadWait = time.Millisecond
	t.Cleanup(func() { prHeadWait = wait })
	return f
}

func (f *fixture) lagPRHead(n int, old string) {
	f.write("prhead-lag", fmt.Sprintf("%d\n", n))
	f.write("prhead-old", old+"\n")
}

func (f *fixture) ghCalls() int {
	b, _ := os.ReadFile(filepath.Join(f.root, "gh-calls"))
	return strings.Count(string(b), "\n")
}

func (f *fixture) write(rel, s string) {
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func js(v any) string { b, _ := json.Marshal(v); return string(b) }

func (f *fixture) ready() (int, string, bool) {
	f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
	list := []any{f.task}
	log := ""
	if f.reviewer != nil {
		list = append(list, f.reviewer)
		log = ledgertest.Log(f.t, js(map[string]any{"at": "2026-11-20T00:00:00Z", "by": f.by, "id": "T-0002", "change": map[string]any{"add": "review"}}))
		if f.findings != "" {
			f.write("kanban/issues/T-0002/library/findings.json", `{"commit": "`+f.findings+`", "findings": []}`)
		}
	}
	f.write("list.json", js(list))
	f.write("kanban/log.jsonl", log)
	if f.report != "" {
		f.write("kanban/issues/T-0001/library/report.json", f.report)
	}
	f.write("prhead", f.prHead+"\n")
	f.writePRFiles()
	if f.questions != "" {
		f.write("questions.json", f.questions)
	}
	var out, errb bytes.Buffer
	rc := Main([]string{"ready", "T-0001"}, strings.NewReader(""), &out, &errb)
	_, err := os.Stat(filepath.Join(f.root, "reviewed-T-0001"))
	return rc, errb.String(), err == nil
}

const other = "0000000000000000000000000000000000000001"

func (f *fixture) writePRFiles() {
	f.write("prfiles", js([][]any{{map[string]any{"filename": f.prFile, "additions": 3, "deletions": 0}}}))
}

func (f *fixture) gateQuestion(from, head string) {
	facts := []string{"delegation: merge=B < tier C", "PR https://github.com/example/app/pull/7"}
	if head != "" {
		facts = append(facts, gate.RecordFact(filepath.Join(f.root, "kanban/issues/T-0001/library/gate", head+".json")))
	}
	f.write("kanban/issues/T-0001/library/questions/1.json", js(map[string]any{"kind": "line", "question": "マージするか", "facts": facts,
		"options": []any{map[string]any{"label": "オーナーが PR を読んでマージする", "pick": true}, map[string]any{"label": "担当に直させる"}}, "from": from}))
	f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}]`
}

func (f *fixture) runRow(n int, expect string, code int, commit string, dirty bool) {
	row := map[string]any{"id": "A1", "result": "pass", "evidence": "go test ./internal/issue/"}
	if n > 0 {
		row["run"] = n
	}
	f.report = strings.TrimSuffix(f.report, "}") + `, "acceptance": [` + js(row) + `]}`
	if expect == "" {
		return
	}
	lib := "kanban/issues/T-0001/library/runs/"
	f.write(lib+fmt.Sprintf("%d.json", n), js(map[string]any{"id": n, "command": []string{"go", "test", "./..."}, "expect": expect, "exit_code": code,
		"commit": commit, "dirty": dirty, "started_at": "2026-11-20T09:00:00Z", "finished_at": "2026-11-20T09:00:10Z",
		"output": fmt.Sprintf("runs/%d.log", n), "output_bytes": 3, "truncated": false}))
	f.write(lib+fmt.Sprintf("%d.log", n), "ok\n")
}

func TestReady(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*fixture)
		want string
	}{
		{"揃えば review にする", func(*fixture) {}, ""},
		{"report.json が無い", func(f *fixture) { f.report = "" }, "report.json が無い"},
		{"report.json に知らないキー", func(f *fixture) { f.report = strings.Replace(f.report, `"pr"`, `"x": 1, "pr"`, 1) }, "形が違う"},
		{"report.json の pr が null", func(f *fixture) { f.report = `{"commit": "` + f.head + `", "pr": null}` }, "形が違う"},
		{"report.json に pr が無い", func(f *fixture) { f.report = `{"commit": "` + f.head + `"}` }, "pr が無い"},
		{"HEAD 不一致", func(f *fixture) { f.report = strings.Replace(f.report, f.head, other, 1) }, "HEAD"},
		{"PR の head が違う", func(f *fixture) { f.prHead = other }, "PR の head"},
		{"レビューが古い commit（stale）", func(f *fixture) { f.findings = other }, "stale"},
		{"レビューが無い（missing）", func(f *fixture) { f.reviewer = nil }, "missing"},
		{"findings.json が無い", func(f *fixture) { f.findings = "" }, "missing"},
		{"Chief of Staff が足していないレビュー", func(f *fixture) { f.by = "terminal:t7" }, "数えない"},
		{"同じブランドのレビュアー", func(f *fixture) { f.reviewer["profile"] = "claude-review" }, "missing"},
		{"レビュー中（review でも done でもない）", func(f *fixture) { f.reviewer["status"] = "doing" }, "missing"},
		{"review: off で差分がレベル A ならレビューを見ない", func(f *fixture) { f.task["review"] = "off"; f.reviewer = nil }, ""},
		{"review: off でも差分がレベル B ならレビューを求める", func(f *fixture) {
			f.task["review"] = "off"
			f.reviewer = nil
			f.prFile = "internal/issue/start_test.go"
		}, "review は off だが、差分がレベル B（実行面 internal/issue/start_test.go・段 A 許可リスト外 internal/issue/start_test.go）なので gate はレビューを求める。andon issue review-request で頼む"},
		{"review: off で差分がレベル C ならレビューを求める", func(f *fixture) {
			f.task["review"] = "off"
			f.reviewer = nil
			f.prFile = "model/issue-map.txt"
		}, "差分がレベル C"},
		{"review: off で差分がレベル B でも、今の head のレビューがあれば通る", func(f *fixture) {
			f.task["review"] = "off"
			f.prFile = "internal/issue/start_test.go"
		}, ""},
		{"review が無ければ止まる", func(f *fixture) { delete(f.task, "review") }, "on/off でない"},
		{"開いた問いがあれば止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}]` }, "T-0001#1（open）"},
		{"答えを受けていない問いがあれば止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "n": 2, "status": "answered"}]` }, "T-0001#2（answered）"},
		{"questions.yaml が読めなければ止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "broken": "x"}]` }, "questions.yaml が読めない"},
		{"受け取った問いと、ほかの課題の問いは見ない", func(f *fixture) {
			f.questions = `[{"id": "T-0001", "n": 1, "status": "received"}, {"id": "T-0009", "n": 1, "status": "open"}]`
		}, ""},
		{"前の head についての gate の問いは数えない", func(f *fixture) { f.gateQuestion("gate", other) }, ""},
		{"前の head の gate の問いに答えがあっても数えない", func(f *fixture) {
			f.gateQuestion("gate", other)
			f.questions = `[{"id": "T-0001", "n": 1, "status": "answered"}]`
		}, ""},
		{"今の head についての gate の問いは止まる", func(f *fixture) { f.gateQuestion("gate", f.head) }, "T-0001#1（open）"},
		{"どの head か読めない gate の問いは止まる", func(f *fixture) { f.gateQuestion("gate", "") }, "T-0001#1（open）"},
		{"gate の外の問いは記録の行があっても止まる", func(f *fixture) { f.gateQuestion("issue", other) }, "T-0001#1（open）"},
		{"前の head の gate の問いのほかに開いた問いがあれば止まる", func(f *fixture) {
			f.gateQuestion("gate", other)
			f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}, {"id": "T-0001", "n": 2, "status": "open"}]`
		}, "T-0001#2（open）"},
		// ADR-0035 受け入れ条件 3: 記録を指す行だけ中身を見る
		{"記録を指さない行だけなら今までどおり通る", func(f *fixture) { f.runRow(0, "", 0, "", false) }, ""},
		{"指す記録が pass で通っていれば通る", func(f *fixture) { f.runRow(1, "pass", 0, f.head, false) }, ""},
		{"fail を期待する記録は変更のある木でよい", func(f *fixture) { f.runRow(1, "fail", 1, f.head, true) }, ""},
		{"記録の commit が HEAD より古くても断らない", func(f *fixture) { f.runRow(1, "pass", 0, other, false) }, ""},
		{"指す記録が無い", func(f *fixture) { f.runRow(3, "", 0, "", false) }, "受け入れ条件 A1 の記録 runs/3.json が無い"},
		{"記録の終了コードが期待と違う", func(f *fixture) { f.runRow(1, "pass", 2, f.head, false) }, "終了コード 2 が期待（pass）と違う"},
		{"pass を期待する記録が commit していない変更のある木", func(f *fixture) { f.runRow(1, "pass", 0, f.head, true) }, "commit していない変更のある木で撮った"},
		// ADR-0035 受け入れ条件 4: 課題が受け入れ条件の番号を持てば、その番号の行が報告に無いと断り、足りない番号を出す
		{"番号の行が報告にあれば通る", func(f *fixture) { f.task["acceptance"] = "A1"; f.runRow(0, "", 0, "", false) }, ""},
		{"番号の行が manual でも行があれば通る", func(f *fixture) {
			f.task["acceptance"] = "A1"
			f.report = strings.TrimSuffix(f.report, "}") + `, "acceptance": [{"id": "A1", "result": "manual", "evidence": "オーナーが見る"}]}`
		}, ""},
		{"報告に受け入れ条件の欄が無い", func(f *fixture) { f.task["acceptance"] = "A1,A4" }, "acceptance に無いので review にしない: A1・A4"},
		{"足りない番号だけを出す", func(f *fixture) { f.task["acceptance"] = "A1,A4,A13"; f.runRow(0, "", 0, "", false) }, "acceptance に無いので review にしない: A4・A13。"},
		{"数字だけの番号に A を付けた行で通る", func(f *fixture) { f.task["acceptance"] = "1"; f.runRow(0, "", 0, "", false) }, ""},
		{"A を付けた番号に数字だけの行で通る", func(f *fixture) {
			f.task["acceptance"] = "A4,13"
			f.report = strings.TrimSuffix(f.report, "}") + `, "acceptance": [{"id": "4", "result": "pass", "evidence": "go test"}, {"id": "A13", "result": "manual", "evidence": "オーナーが見る"}]}`
		}, ""},
		{"足りない番号は予約キーの綴りで出す", func(f *fixture) { f.task["acceptance"] = "1,4,13"; f.runRow(0, "", 0, "", false) }, "acceptance に無いので review にしない: 4・13。"},
		{"番号の綴りが違う行は数えない", func(f *fixture) { f.task["acceptance"] = "A01"; f.runRow(0, "", 0, "", false) }, ": A01。"},
		{"番号の形が違えば断る", func(f *fixture) { f.task["acceptance"] = "A1, A4" }, "acceptance の形が違う"},
		{"番号が文字列でなければ断る", func(f *fixture) { f.task["acceptance"] = []any{"A1"} }, "acceptance の形が違う"},
		{"調べる課題でも番号の行を求める", func(f *fixture) {
			f.task["deliverable"] = "candidates"
			f.task["review"] = "off"
			f.task["acceptance"] = "A2"
		}, "acceptance に無いので review にしない: A2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			c.mod(f)
			rc, errs, reviewed := f.ready()
			if c.want == "" {
				if rc != 0 || !reviewed {
					t.Fatalf("rc=%d reviewed=%v err=%q", rc, reviewed, errs)
				}
				return
			}
			if rc != 1 || reviewed || !strings.Contains(errs, c.want) {
				t.Fatalf("want rc 1 with %q, got rc=%d reviewed=%v err=%q", c.want, rc, reviewed, errs)
			}
		})
	}
}

func TestReadyNamesOnlyStaleGateQuestions(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*fixture)
		want string
	}{
		{"問いが無ければ名指さない", func(*fixture) {}, "review T-0001"},
		{"前の head の gate の問いだけを名指す", func(f *fixture) { f.gateQuestion("gate", other) }, "review T-0001 --past T-0001#1"},
		{"前の head の gate の問いがいくつあっても名指すのはそれだけ", func(f *fixture) {
			f.gateQuestion("gate", other)
			f.write("kanban/issues/T-0001/library/questions/3.json", js(map[string]any{"kind": "line", "question": "マージするか", "from": "gate",
				"facts":   []string{gate.RecordFact(filepath.Join(f.root, "kanban/issues/T-0001/library/gate", other+".json"))},
				"options": []any{map[string]any{"label": "オーナーが PR を読んでマージする", "pick": true}, map[string]any{"label": "担当に直させる"}}}))
			f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}, {"id": "T-0001", "n": 2, "status": "received"}, {"id": "T-0001", "n": 3, "status": "answered"}]`
		}, "review T-0001 --past T-0001#1,T-0001#3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			c.mod(f)
			if rc, errs, reviewed := f.ready(); rc != 0 || !reviewed {
				t.Fatalf("rc=%d reviewed=%v err=%q", rc, reviewed, errs)
			}
			b, _ := os.ReadFile(filepath.Join(f.root, "reviewed-T-0001"))
			if got := strings.TrimSpace(string(b)); got != c.want {
				t.Fatalf("kanban に %q、want %q", got, c.want)
			}
		})
	}
}

// review-request の守り（ADR-0032 決めたこと 8）: その課題のレビュアーの課題が動いている（todo・doing）間は、
// 見ている commit に関わらず頼み直さない。断ったときは report.json も報告も書かない。
func TestReviewRequestWhileReviewerRuns(t *testing.T) {
	cases := []struct {
		name   string
		status string
		brief  string
		want   string
	}{
		{"古い commit のレビュアーが doing", "doing", other, "レビュアー T-0001-r1 がまだ動いている（doing。commit " + other[:12] + " を見ている）"},
		{"古い commit のレビュアーが todo", "todo", other, "レビュアー T-0001-r1 がまだ動いている（todo。"},
		{"依頼書の読めないレビュアーが doing", "doing", "", "見ている commit は依頼書から読めない"},
		{"HEAD のレビュアーが doing", "doing", "HEAD", "のレビューはもう頼んである（T-0001-r1 が doing）"},
		{"古い commit のレビュアーが返した（結果なし）", "review", other, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			rev := map[string]any{"id": "T-0001-r1", "parent": "T-0001", "status": c.status, "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task, rev}))
			f.write("kanban/log.jsonl", "")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("prhead", f.prHead+"\n")
			if c.brief != "" {
				sha := c.brief
				if sha == "HEAD" {
					sha = f.head
				}
				f.write("kanban/issues/T-0001-r1/brief.md", "対象: worktree `x`（detached）。commit "+sha+"。差分は gh pr diff。\n")
			}
			var out, errb bytes.Buffer
			rc := Main([]string{"review-request", "T-0001"}, strings.NewReader("## Report\n"), &out, &errb)
			report, _ := os.ReadFile(filepath.Join(f.root, "kanban/issues/T-0001/library/report.json"))
			_, reported := os.Stat(filepath.Join(f.root, "reported-T-0001"))
			if c.want == "" {
				if rc != 0 || reported != nil || !strings.Contains(string(report), `"round": 2`) {
					t.Fatalf("rc=%d reported=%v err=%q report=%s", rc, reported == nil, errb.String(), report)
				}
				return
			}
			if rc != 1 || !strings.Contains(errb.String(), c.want) {
				t.Fatalf("want rc 1 with %q, got rc=%d err=%q", c.want, rc, errb.String())
			}
			if reported == nil || string(report) != f.report {
				t.Fatalf("断ったのに書いた: reported=%v report=%s", reported == nil, report)
			}
		})
	}
}

func TestReviewRequestFollowsGateTier(t *testing.T) {
	cases := []struct {
		name   string
		review any
		file   string
		want   string
	}{
		{"review: off でも差分がレベル B なら頼める", "off", "internal/issue/start_test.go", ""},
		{"review: off でも差分がレベル C なら頼める", "off", "model/issue-map.txt", ""},
		{"review: off で差分がレベル A なら断る", "off", "docs/a.md", "issue.yaml の review が off で、差分はレベル A。gate はレビューを求めないので頼めない"},
		{"review: on なら差分がレベル A でも頼める", "on", "docs/a.md", ""},
		{"review が無ければ断る", nil, "internal/issue/start_test.go", "review が on/off でない"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			if c.review == nil {
				delete(f.task, "review")
			} else {
				f.task["review"] = c.review
			}
			f.prFile = c.file
			f.writePRFiles()
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task}))
			f.write("kanban/log.jsonl", "")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("prhead", f.prHead+"\n")
			var out, errb bytes.Buffer
			rc := Main([]string{"review-request", "T-0001"}, strings.NewReader("## Report\n"), &out, &errb)
			report, _ := os.ReadFile(filepath.Join(f.root, "kanban/issues/T-0001/library/report.json"))
			if c.want == "" {
				if rc != 0 || !strings.Contains(string(report), `"round": 1`) {
					t.Fatalf("rc=%d err=%q report=%s", rc, errb.String(), report)
				}
				md, _ := os.ReadFile(filepath.Join(f.root, "reported-T-0001"))
				if c.review == "off" && !strings.Contains(string(md), "review は off だが、差分がレベル ") {
					t.Fatalf("レビュー依頼の行に review: off で頼んだ理由が無い: %s", md)
				}
				return
			}
			if rc != 1 || !strings.Contains(errb.String(), c.want) || string(report) != f.report {
				t.Fatalf("want rc 1 with %q, got rc=%d err=%q report=%s", c.want, rc, errb.String(), report)
			}
		})
	}
}

// report の守り（ADR-0034 決めたこと 11・受け入れ条件 9）: 課題の repo と違う repo の PR は、--pr・--data の pr・
// 前の報告からの引き継ぎのどれで渡しても断り、report.json も報告も書かない。
func TestReportRefusesOtherRepoPR(t *testing.T) {
	const mine, theirs = "https://github.com/example/app/pull/7", "https://github.com/example/other/pull/7"
	cases := []struct {
		name string
		repo string
		prev string
		args []string
		want string
	}{
		{"--pr で違う repo", "github.com/example/app", mine, []string{"--pr", theirs}, "pr の repo（github.com/example/other）が課題の repo（github.com/example/app）と違う"},
		{"--data の pr で違う repo", "github.com/example/app", mine, []string{"--data", `{"pr": "` + theirs + `"}`}, "pr の repo（github.com/example/other）が課題の repo（github.com/example/app）と違う"},
		{"前の報告から違う repo を引き継ぐ", "github.com/example/app", theirs, nil, "pr の repo（github.com/example/other）が課題の repo（github.com/example/app）と違う"},
		{"owner が違う", "github.com/example/app", mine, []string{"--pr", "https://github.com/other/app/pull/7"}, "と違う"},
		{"repo の無い課題に PR", "", "", []string{"--pr", mine}, "repo の無い課題に PR は書かない"},
		{"--pr で同じ repo", "github.com/example/app", "", []string{"--pr", mine}, ""},
		{"--data の pr で同じ repo", "github.com/example/app", "", []string{"--data", `{"pr": "` + mine + `"}`}, ""},
		{"前の報告から同じ repo を引き継ぐ", "github.com/example/app", mine, nil, ""},
		{"repo の無い課題で PR なし", "", "", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			if c.repo != "" {
				f.task["repo"] = c.repo
			}
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			prev := `{"commit": "` + f.head + `"}`
			if c.prev != "" {
				prev = `{"commit": "` + f.head + `", "pr": "` + c.prev + `"}`
			}
			if c.repo == "" {
				prev = `{}`
			}
			rp := filepath.Join(f.root, "kanban/issues/T-0001/library/report.json")
			f.write("kanban/issues/T-0001/library/report.json", prev)
			var out, errb bytes.Buffer
			rc := Main(append([]string{"report", "T-0001"}, c.args...), strings.NewReader("## Report\n"), &out, &errb)
			got, _ := os.ReadFile(rp)
			_, reported := os.Stat(filepath.Join(f.root, "reported-T-0001"))
			if c.want == "" {
				if rc != 0 || reported != nil || (c.repo != "" && !strings.Contains(string(got), mine)) {
					t.Fatalf("rc=%d reported=%v err=%q report=%s", rc, reported == nil, errb.String(), got)
				}
				return
			}
			if rc != 1 || !strings.Contains(errb.String(), c.want) || strings.Count(strings.TrimSpace(errb.String()), "\n") != 0 {
				t.Fatalf("want rc 1 with one line %q, got rc=%d err=%q", c.want, rc, errb.String())
			}
			if reported == nil || string(got) != prev {
				t.Fatalf("断ったのに書いた: reported=%v report=%s", reported == nil, got)
			}
		})
	}
}

// report は人が読む報告（Markdown）に見出し「## Report」の行が無ければ断り、report.json も kanban の報告も
// 書かない。前の報告を sed で切り出して書き直し、本文が消えて PR の行だけが残った報告で置き換えたことがあった。
func TestReportRefusesMarkdownWithoutReportHeading(t *testing.T) {
	const pr = "PR: https://github.com/example/app/pull/7\n"
	for _, c := range []struct {
		name string
		md   string
		ok   bool
	}{
		{"本文が消えて PR の行だけが残った", pr, false},
		{"見出しの無い本文", pr + "直した。\n## Next\n- x\n", false},
		{"見出しが違う（### Report）", "### Report\n直した。\n", false},
		{"見出しが違う（## Reports）", "## Reports\n直した。\n", false},
		{"見出しが行の途中", "直した。## Report\n", false},
		{"見出しがコードフェンスの中だけ", pr + "```\n## Report\n```\n", false},
		{"見出しが ~~~ のフェンスの中だけ", pr + "~~~md\n## Report\n~~~\n", false},
		{"見出しが閉じないフェンスの中", pr + "```\n## Report\n直した。\n", false},
		{"短い閉じではフェンスが閉じない", pr + "````\n```\n## Report\n````\n", false},
		{"フェンスのあとに見出し", pr + "```\nx\n```\n## Report\n直した。\n", true},
		{"違う文字ではフェンスが閉じない", pr + "```\n~~~\n```\n## Report\n", true},
		{"PR の行のあとに見出し", pr + "## Report\n直した。\n## Next\n- x\n", true},
		{"見出しだけ", "## Report\n", true},
		{"CRLF の見出し", "## Report\r\n直した。\r\n", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.task["repo"] = "github.com/example/app"
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			rp := filepath.Join(f.root, "kanban/issues/T-0001/library/report.json")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			var out, errb bytes.Buffer
			rc := Main([]string{"report", "T-0001"}, strings.NewReader(c.md), &out, &errb)
			got, _ := os.ReadFile(rp)
			md, mdErr := os.ReadFile(filepath.Join(f.root, "reported-T-0001"))
			if c.ok {
				if rc != 0 || mdErr != nil || string(md) != c.md {
					t.Fatalf("rc=%d err=%q reported=%q", rc, errb.String(), md)
				}
				return
			}
			if rc != 1 || !strings.Contains(errb.String(), "見出し「## Report」の行が無い") || strings.Count(strings.TrimSpace(errb.String()), "\n") != 0 {
				t.Fatalf("want rc 1 with one line, got rc=%d err=%q", rc, errb.String())
			}
			if mdErr == nil || string(got) != f.report {
				t.Fatalf("断ったのに書いた: reported=%v report=%s", mdErr == nil, got)
			}
		})
	}
}

// report の引き継ぎ（ADR-0061 決めたこと 2・受け入れ条件 2）: 知識の欠けの申告 gaps は、--data に無ければ前の
// report.json から引き継ぐ（途中の報告に書いた申告が、最後の書き直しで消えない）。空の配列を渡すと取り下げになり、
// 次の打ち直しでも空のまま。--data に gaps があれば丸ごと置き換える。
func TestReportCarriesGaps(t *testing.T) {
	f := newFixture(t)
	f.task["repo"] = "github.com/example/app"
	f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
	rp := filepath.Join(f.root, "kanban/issues/T-0001/library/report.json")
	report := func(args ...string) *contract.Report {
		t.Helper()
		var out, errb bytes.Buffer
		if rc := Main(append([]string{"report", "T-0001"}, args...), strings.NewReader("## Report\n"), &out, &errb); rc != 0 {
			t.Fatalf("report %v: rc=%d err=%q", args, rc, errb.String())
		}
		r, err := contract.ReadReport(rp)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	two := `{"gaps": [{"kind": "wrong", "where": "AGENTS.md", "text": "一覧が 1 つ違う", "proposal": "合わせる"},` +
		` {"kind": "missing", "where": "docs/operate.md", "text": "順が書いていない"}]}`
	if r := report("--data", two); len(r.Gaps) != 2 {
		t.Fatalf("gaps を書けない: %+v", r.Gaps)
	}
	r := report("--data", `{"remember": ["書き直した"]}`)
	if len(r.Gaps) != 2 || r.Gaps[0].Kind != "wrong" || r.Gaps[0].Proposal == nil || *r.Gaps[0].Proposal != "合わせる" || r.Gaps[1].Where != "docs/operate.md" {
		t.Fatalf("--data に gaps が無いのに前の gaps が残らない: %+v", r.Gaps)
	}
	if r := report(); len(r.Gaps) != 2 {
		t.Fatalf("--data 無しの打ち直しで前の gaps が残らない: %+v", r.Gaps)
	}
	if r := report("--data", `{"gaps": [{"kind": "hard_to_find", "where": "wiki/topics/tool-quirks.md", "text": "遅れて見つけた"}]}`); len(r.Gaps) != 1 || r.Gaps[0].Kind != "hard_to_find" {
		t.Fatalf("--data の gaps で置き換わらない: %+v", r.Gaps)
	}
	if r := report("--data", `{"gaps": []}`); len(r.Gaps) != 0 {
		t.Fatalf("空の配列で取り下げられない: %+v", r.Gaps)
	}
	if b, _ := os.ReadFile(rp); strings.Contains(string(b), "gaps") {
		t.Fatalf("取り下げたのに report.json に gaps が残る: %s", b)
	}
	if r := report(); len(r.Gaps) != 0 {
		t.Fatalf("取り下げたあとの打ち直しで gaps が戻る: %+v", r.Gaps)
	}
	var out, errb bytes.Buffer
	if rc := Main([]string{"report", "T-0001", "--data", `{"gaps": [{"kind": "stale", "where": "AGENTS.md", "text": "x"}]}`}, strings.NewReader("## Report\n"), &out, &errb); rc != 1 || !strings.Contains(errb.String(), "gaps") {
		t.Fatalf("知らない kind を断らない: rc=%d err=%q", rc, errb.String())
	}
	// 前の report.json が壊れていて gaps を引き継げないときは、pr・estimate・plan を渡しても黙って越えない
	// （越えると途中の報告に書いた申告が消える）。gaps も渡せば書ける。
	f.write("kanban/issues/T-0001/library/report.json", "{")
	all := `"pr": "https://github.com/example/app/pull/7", "estimate": {"size": "large", "reason": "x"}, "plan": [{"id": "p1", "text": "x", "done": false}]`
	out.Reset()
	errb.Reset()
	if rc := Main([]string{"report", "T-0001", "--data", "{" + all + "}"}, strings.NewReader("## Report\n"), &out, &errb); rc != 1 {
		t.Fatalf("壊れた前の報告を gaps 無しで越えた: rc=%d err=%q", rc, errb.String())
	}
	if r := report("--data", "{"+all+`, "gaps": []}`); len(r.Gaps) != 0 || r.PR == "" {
		t.Fatalf("gaps も渡したのに書けない: %+v", r)
	}
}

func (f *fixture) rebaseWorktree(conflict bool) string {
	f.t.Helper()
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", f.wt}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			f.t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(name, body string) {
		if err := os.WriteFile(filepath.Join(f.wt, name), []byte(body), 0o644); err != nil {
			f.t.Fatal(err)
		}
		git("add", name)
	}
	git("branch", "-M", "main")
	put("a.txt", "a\n")
	put("c.txt", "c\n")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "topic")
	put("a.txt", "a\nA\n")
	git("commit", "-q", "-m", "topic")
	x := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	if conflict {
		put("a.txt", "a\nM\n")
	} else {
		put("c.txt", "c\nC\n")
	}
	git("commit", "-q", "-m", "main moves")
	git("update-ref", "refs/remotes/origin/main", "main")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	git("checkout", "-q", "topic")
	if conflict {
		git("reset", "-q", "--hard", "origin/main")
		put("a.txt", "a\nM\nA2\n")
		git("commit", "-q", "-m", "topic")
	} else {
		git("rebase", "-q", "origin/main")
	}
	y := git("rev-parse", "HEAD")
	f.head, f.prHead, f.findings = y, y, x
	f.report = `{"commit": "` + y + `", "pr": "https://github.com/example/app/pull/7"}`
	return x
}

func TestReadyCarriesReviewOverSamePatchRebase(t *testing.T) {
	for _, c := range []struct {
		name     string
		conflict bool
		want     string
	}{
		{"衝突の無い rebase", false, ""},
		{"衝突を解いて差分が変わった rebase", true, "stale"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.rebaseWorktree(c.conflict)
			rc, errs, reviewed := f.ready()
			if c.want == "" {
				if rc != 0 || !reviewed {
					t.Fatalf("rc=%d reviewed=%v err=%q", rc, reviewed, errs)
				}
				return
			}
			if rc != 1 || reviewed || !strings.Contains(errs, c.want) {
				t.Fatalf("want rc 1 with %q, got rc=%d reviewed=%v err=%q", c.want, rc, reviewed, errs)
			}
		})
	}
}

func TestReviewRequestAfterSamePatchRebase(t *testing.T) {
	for _, c := range []struct {
		name     string
		conflict bool
		want     string
	}{
		{"衝突の無い rebase", false, "そのレビューを引き継ぐので頼まない"},
		{"衝突を解いて差分が変わった rebase", true, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			x := f.rebaseWorktree(c.conflict)
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task, f.reviewer}))
			f.write("kanban/log.jsonl", ledgertest.Log(t, js(map[string]any{"at": "2026-11-20T00:00:00Z", "by": f.by, "id": "T-0002", "change": map[string]any{"add": "review"}})))
			f.write("kanban/issues/T-0002/library/findings.json", `{"commit": "`+x+`", "findings": []}`)
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("prhead", f.prHead+"\n")
			var out, errb bytes.Buffer
			rc := Main([]string{"review-request", "T-0001"}, strings.NewReader("## Report\n"), &out, &errb)
			report, _ := os.ReadFile(filepath.Join(f.root, "kanban/issues/T-0001/library/report.json"))
			_, reported := os.Stat(filepath.Join(f.root, "reported-T-0001"))
			if c.want == "" {
				if rc != 0 || reported != nil || !strings.Contains(string(report), `"round": 2`) {
					t.Fatalf("rc=%d reported=%v err=%q report=%s", rc, reported == nil, errb.String(), report)
				}
				return
			}
			if rc != 1 || !strings.Contains(errb.String(), c.want) {
				t.Fatalf("want rc 1 with %q, got rc=%d err=%q", c.want, rc, errb.String())
			}
			if reported == nil || string(report) != f.report {
				t.Fatalf("断ったのに書いた: reported=%v report=%s", reported == nil, report)
			}
		})
	}
}

func TestReviewRequestKeepsPreviousReport(t *testing.T) {
	const body = "PR: https://github.com/example/app/pull/7\n## Report\n直した。\n## Next\n- マージ後に andon issue finish T-0001\n## Remember\n- 前の報告の学び\n"
	const oldHeader = "## レビュー依頼\n1 回目／上限 2。commit " + other + "。PR https://github.com/example/app/pull/7。Reviewer codex-review\n\n"
	for _, c := range []struct {
		name  string
		prev  string
		stdin string
		want  []string
		not   []string
		fail  string
	}{
		{"stdin が空なら前の報告を残す", body, "", []string{"## レビュー依頼\n1 回目", body}, nil, ""},
		{"前のレビュー依頼の段は重ねない", oldHeader + body, "", []string{body}, []string{other}, ""},
		{"stdin があれば置き換える", body, "## Report\n新しい報告\n", []string{"## Report\n新しい報告\n"}, []string{"## Next"}, ""},
		{"stdin も前の報告も空なら断る", "", " \n", nil, nil, "前の報告（report.md）も空"},
		{"stdin に見出し「## Report」の行が無ければ断る", body, "PR: https://github.com/example/app/pull/7\n", nil, nil, "見出し「## Report」の行が無い"},
		{"前の報告に見出し「## Report」の行が無ければ断る", "PR: https://github.com/example/app/pull/7\n", "", nil, nil, "見出し「## Report」の行が無い"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task}))
			f.write("kanban/log.jsonl", "")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("prhead", f.prHead+"\n")
			if c.prev != "" {
				f.write("kanban/issues/T-0001/report.md", c.prev)
			}
			var out, errb bytes.Buffer
			rc := Main([]string{"review-request", "T-0001"}, strings.NewReader(c.stdin), &out, &errb)
			report, _ := os.ReadFile(filepath.Join(f.root, "kanban/issues/T-0001/library/report.json"))
			md, mdErr := os.ReadFile(filepath.Join(f.root, "reported-T-0001"))
			if c.fail != "" {
				if rc != 1 || !strings.Contains(errb.String(), c.fail) || mdErr == nil || string(report) != f.report {
					t.Fatalf("want rc 1 with %q, got rc=%d err=%q reported=%v report=%s", c.fail, rc, errb.String(), mdErr == nil, report)
				}
				return
			}
			if rc != 0 || !strings.Contains(string(report), `"round": 1`) {
				t.Fatalf("rc=%d err=%q report=%s", rc, errb.String(), report)
			}
			if !strings.HasPrefix(string(md), "## レビュー依頼\n1 回目／上限 ") || strings.Count(string(md), "## レビュー依頼") != 1 {
				t.Fatalf("レビュー依頼の段が先頭に1つでない: %s", md)
			}
			for _, w := range c.want {
				if !strings.Contains(string(md), w) {
					t.Fatalf("報告に %q が無い: %s", w, md)
				}
			}
			for _, w := range c.not {
				if strings.Contains(string(md), w) {
					t.Fatalf("報告に %q が残る: %s", w, md)
				}
			}
		})
	}
}

func TestCarriedReviewRequest(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	rr := &contract.ReviewRequest{Round: 1, Limit: 2, Commit: a, Reviewer: "codex-review"}
	for _, tc := range []struct {
		name string
		prev *contract.Report
		head string
		want bool
	}{
		{"HEAD が同じ", &contract.Report{Commit: a, ReviewRequest: rr}, a, true},
		{"HEAD が進んだ", &contract.Report{Commit: a, ReviewRequest: rr}, b, false},
		{"前の報告が依頼と違う commit", &contract.Report{Commit: b, ReviewRequest: rr}, b, false},
		{"依頼が無い", &contract.Report{Commit: a}, a, false},
		{"repo の無い課題", &contract.Report{ReviewRequest: rr}, "", false},
	} {
		if got := carriedReviewRequest(tc.prev, tc.head); (got != nil) != tc.want {
			t.Errorf("%s: 引き継いだ %v、want %v", tc.name, got != nil, tc.want)
		}
	}
}

func TestWaitPRHead(t *testing.T) {
	upstream := func(f *fixture, pushed bool) {
		bare := filepath.Join(filepath.Dir(f.wt), "remote.git")
		git := func(dir string, args ...string) string {
			c := exec.Command("git", append([]string{"-C", dir}, args...)...)
			c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
				"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
			out, err := c.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		git(filepath.Dir(f.wt), "init", "-q", "--bare", bare)
		git(f.wt, "remote", "add", "origin", bare)
		git(f.wt, "push", "-q", "-u", "origin", "HEAD:refs/heads/b")
		if !pushed {
			git(f.wt, "commit", "-q", "--allow-empty", "-m", "y")
			old := f.head
			f.head = git(f.wt, "rev-parse", "HEAD")
			f.report = strings.Replace(f.report, old, f.head, 1)
		}
	}
	cases := []struct {
		name    string
		mod     func(*fixture)
		request bool
		want    string
		calls   int
	}{
		{"ready は追いつくのを待って通る", func(f *fixture) { f.lagPRHead(2, other) }, false, "", 3},
		{"review-request は追いつくのを待って頼む", func(f *fixture) { f.lagPRHead(2, other) }, true, "", 3},
		{"upstream が HEAD と同じなら待つ", func(f *fixture) { upstream(f, true); f.lagPRHead(2, other) }, false, "", 3},
		{"上限まで読んでも違えば断る", func(f *fixture) { f.prHead = other }, false, "PR の head", prHeadTries},
		{"review-request も上限まで読んでも違えば断る", func(f *fixture) { f.prHead = other }, true, "PR の head", prHeadTries},
		{"push していなければ待たずに断る", func(f *fixture) { old := f.head; upstream(f, false); f.prHead = old }, false, "PR の head", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.task["review"] = "off"
			f.reviewer = nil
			c.mod(f)
			var rc int
			var errs string
			if c.request {
				f.task["review"] = "on"
				f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
				f.write("list.json", js([]any{f.task}))
				f.write("kanban/log.jsonl", "")
				f.write("kanban/issues/T-0001/library/report.json", f.report)
				f.write("prhead", f.prHead+"\n")
				var out, errb bytes.Buffer
				rc = Main([]string{"review-request", "T-0001"}, strings.NewReader("## Report\n"), &out, &errb)
				errs = errb.String()
				if c.want == "" {
					report, _ := os.ReadFile(filepath.Join(f.root, "kanban/issues/T-0001/library/report.json"))
					if !strings.Contains(string(report), `"round": 1`) {
						t.Fatalf("頼んでいない: %s", report)
					}
				}
			} else {
				rc, errs, _ = f.ready()
			}
			if c.want == "" && rc != 0 {
				t.Fatalf("rc=%d err=%q", rc, errs)
			}
			if c.want != "" && (rc != 1 || !strings.Contains(errs, c.want)) {
				t.Fatalf("want rc 1 with %q, got rc=%d err=%q", c.want, rc, errs)
			}
			if got := f.ghCalls(); got != c.calls {
				t.Fatalf("gh を %d 回呼んだ、want %d", got, c.calls)
			}
		})
	}
}
