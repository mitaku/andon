package issue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger/ledgertest"
)

// ready の照合は偽の kanban の CLI と gh・本物の git で見る（振る舞いの全体は tests/andon-issue.sh）。
// 偽の kanban の CLI は $FAKE_ROOT の show-<id>.json・list.json を返し、review は reviewed-<id> に引数を書くだけ。

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
cat "$FAKE_ROOT/prhead"
`

type fixture struct {
	t         *testing.T
	root      string
	wt        string
	head      string
	task      map[string]any
	reviewer  map[string]any // nil ならレビュアーの課題は無い
	by        string         // レビュアーの課題を足した端末
	report    string         // report.json の中身（空なら置かない）
	findings  string         // レビュアーの findings.json の commit（空なら置かない）
	prHead    string
	questions string // kanban questions --json の中身（空なら問いが無い）
	state     bool   // stop-check の控えの置き場を作ったか
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
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("KANBAN_ROOT", "")
	// 控え（dispatcher の置き場と ready の lock）は一時ディレクトリに。本物の HOME の ~/.local/state に書かない
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	return f
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

// ready を打ち、終了コードと stderr と review にしたかを返す。
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
	if f.questions != "" {
		f.write("questions.json", f.questions)
	}
	var out, errb bytes.Buffer
	rc := Main([]string{"ready", "T-0001"}, strings.NewReader(""), &out, &errb)
	_, err := os.Stat(filepath.Join(f.root, "reviewed-T-0001"))
	return rc, errb.String(), err == nil
}

const other = "0000000000000000000000000000000000000001"

// gateQuestion は T-0001 の問い 1 を開いたまま置く。from が出した問いで、head が空でなければ事実に gate の記録
// （library/gate/<head>.json）の行を書く（merge.go の needs_human の問いと同じ形）。
func (f *fixture) gateQuestion(from, head string) {
	facts := []string{"delegation: merge=B < tier C", "PR https://github.com/example/app/pull/7"}
	if head != "" {
		facts = append(facts, gate.RecordFact(filepath.Join(f.root, "kanban/issues/T-0001/library/gate", head+".json")))
	}
	f.write("kanban/issues/T-0001/library/questions/1.json", js(map[string]any{"kind": "line", "question": "マージするか", "facts": facts,
		"options": []any{map[string]any{"label": "オーナーが PR を読んでマージする", "pick": true}, map[string]any{"label": "担当に直させる"}}, "from": from}))
	f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}]`
}

// runRow は report.json に、記録 n を指す受け入れ条件の行 A1 を足す（n が 0 なら記録を指さない行）。
// 記録は andon issue record が書く形で library/runs/<n>.json と <n>.log に置く（expect が空なら置かない）。
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
		want string // 空なら通る
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
		{"review: off ならレビューを見ない", func(f *fixture) { f.task["review"] = "off"; f.reviewer = nil }, ""},
		{"review が無ければ止まる", func(f *fixture) { delete(f.task, "review") }, "on/off でない"},
		{"開いた問いがあれば止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "n": 1, "status": "open"}]` }, "T-0001#1（open）"},
		{"答えを受けていない問いがあれば止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "n": 2, "status": "answered"}]` }, "T-0001#2（answered）"},
		{"questions.yaml が読めなければ止まる", func(f *fixture) { f.questions = `[{"id": "T-0001", "broken": "x"}]` }, "questions.yaml が読めない"},
		{"受け取った問いと、ほかの課題の問いは見ない", func(f *fixture) {
			f.questions = `[{"id": "T-0001", "n": 1, "status": "received"}, {"id": "T-0009", "n": 1, "status": "open"}]`
		}, ""},
		// gate が needs_human で問いを出したあと、担当が doing に戻って rebase した（head が動いた）。その問いは数えない
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

// ready が前の head の gate の問いを越えるときは、その問いだけを `kanban review --past` で名指す。--force は打たない
// （確かめのあとに kanban ask で足された問いまで越えてしまう。名指しの外の問いは kanban が lock の中で断る）
func TestReadyNamesOnlyStaleGateQuestions(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*fixture)
		want string // 偽の kanban の CLI に届いた review の引数
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
		status string // レビュアーの課題の status
		brief  string // レビュアーの依頼書の commit（空なら依頼書を置かない）
		want   string // 空なら頼める
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

// report の守り（ADR-0034 決めたこと 11・受け入れ条件 9）: 課題の repo と違う repo の PR は、--pr・--data の pr・
// 前の報告からの引き継ぎのどれで渡しても断り、report.json も報告も書かない。
func TestReportRefusesOtherRepoPR(t *testing.T) {
	const mine, theirs = "https://github.com/example/app/pull/7", "https://github.com/example/other/pull/7"
	cases := []struct {
		name string
		repo string   // issue.yaml の repo（空なら repo の無い課題）
		prev string   // 前の report.json の pr（空なら pr を書かない）
		args []string // report <id> のあとの引数
		want string   // 空なら通る
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

// rebaseWorktree は f.wt の履歴を、根拠の出来事の形に作り直す: 課題の branch の commit x をレビューが見たあと、
// main が衝突しない変更で進み、担当が fetch して rebase した（head は y）。conflict なら rebase で衝突を解いて差分が変わる。
// f.head・f.prHead・f.report を y に、レビューの findings を x にする。
func (f *fixture) rebaseWorktree(conflict bool) (x, y string) {
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
	x = git("rev-parse", "HEAD")
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
		// 衝突を解いて rebase し直したのと同じ: main の上に、解いた中身の commit を積む
		git("reset", "-q", "--hard", "origin/main")
		put("a.txt", "a\nM\nA\n")
		git("commit", "-q", "-m", "topic")
	} else {
		git("rebase", "-q", "origin/main")
	}
	y = git("rev-parse", "HEAD")
	f.head, f.prHead, f.findings = y, y, x
	f.report = `{"commit": "` + y + `", "pr": "https://github.com/example/app/pull/7"}`
	return x, y
}

// 衝突の無い rebase で head だけが変わり、PR の差分が前のレビューと同じなら、ready はそのレビューを
// head に引き継いで通す。衝突を解いて差分が変わった rebase は、今までどおり stale で止まる。
func TestReadyCarriesReviewOverSamePatchRebase(t *testing.T) {
	for _, c := range []struct {
		name     string
		conflict bool
		want     string // 空なら通る
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

// 衝突の無い rebase のあとの review-request は、前のレビューを引き継ぐので頼まない（レビュアーを起動しない）。
// report.json も報告も書かない。差分が変わった rebase なら、今までどおり次の回を頼む。
func TestReviewRequestAfterSamePatchRebase(t *testing.T) {
	for _, c := range []struct {
		name     string
		conflict bool
		want     string // 空なら頼める
	}{
		{"衝突の無い rebase", false, "そのレビューを引き継ぐので頼まない"},
		{"衝突を解いて差分が変わった rebase", true, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			x, _ := f.rebaseWorktree(c.conflict)
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

// 報告の打ち直しは、HEAD が前の報告の commit とも頼んだ commit とも同じときだけ review_request を引き継ぐ。
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
