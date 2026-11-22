package digest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
	"andon/internal/waiting"
)

// 偽の kanban（ファイルを直に置く）と dispatcher の記録を置いて、本物の Collect・Build・Send を動かす（ADR-0044 受け入れ条件 1〜8）。

const (
	sha1 = "1111111111111111111111111111111111111111"
	sha2 = "2222222222222222222222222222222222222222"
	sha3 = "3333333333333333333333333333333333333333"
)

var now = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

type fake struct {
	t    *testing.T
	root string
	ws   string
	log  []string
	env  Env
}

func newFake(t *testing.T) *fake {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "sample-site")
	f := &fake{t: t, root: root, ws: filepath.Join(base, "workspace")}
	f.write("kanban/config.yaml", "version: 3\nprefix: S\nchief_of_staff: t1\n")
	f.env = Env{Root: root, Workspace: f.ws, Hostname: "cell", Now: now,
		Delivered: filepath.Join(base, "state", "sample-site.delivered.jsonl"), Signals: filepath.Join(base, "state", "sample-site.signals.json")}
	// 期間を決めるダイジェストの課題（ルーチン daily-digest が足したもの）
	f.issue("S-0100", "doing", "routine: daily-digest", "profile: claude")
	return f
}

func (f *fake) write(rel, body string) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if filepath.IsAbs(rel) {
		p = rel
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fake) issue(id, status string, keys ...string) {
	body := fmt.Sprintf("id: %s\ntitle: 題 %s\nstatus: %s\ncreated: \"2026-11-01T01:00:00Z\"\n", id, id, status)
	for _, k := range keys {
		body += k + "\n"
	}
	f.write("kanban/issues/"+id+"/issue.yaml", body)
}

func (f *fake) lib(id, rel, body string) { f.write("kanban/issues/"+id+"/library/"+rel, body) }

func prURL(n int) string { return fmt.Sprintf("https://github.com/example/sample-site/pull/%d", n) }

// merged は PR n をマージした課題を置く（auto なら dispatcher が head でマージした）。
func (f *fake) merged(id string, n int, at, head string, auto bool) {
	f.issue(id, "done")
	f.lib(id, "report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, head, prURL(n)))
	f.lib(id, "outcome.json", fmt.Sprintf(`{"result": "merged", "pr": "%s", "merged_at": "%s", "gates": [{"head": "%s", "at": "%s", "tier": "B", "decision": "approve", "reasons": [], "merged": %t}], "worked_minutes": 30, "sessions": 1}`,
		prURL(n), at, head, at, auto))
	f.row(at, "dispatcher", id, `{"status":"done"}`)
}

// mergeWait は PR n のマージを待っている課題を置く（review にいて、今の head について gate の問いが開いている）。
func (f *fake) mergeWait(id string, n int, head, title string) {
	f.write("kanban/issues/"+id+"/issue.yaml", fmt.Sprintf("id: %s\ntitle: %s\nstatus: review\nprofile: claude\ncreated: \"2026-11-21T01:00:00Z\"\n", id, title))
	f.lib(id, "report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, head, prURL(n)))
	f.write("kanban/issues/"+id+"/questions.yaml", "- n: 1\n  text: 問い\n  kind: line\n  status: open\n  asked: \"2026-11-21T04:00:00Z\"\n")
	record := filepath.Join(f.root, "kanban/issues", id, "library/gate", head+".json")
	f.lib(id, "questions/1.json", fmt.Sprintf(`{"kind": "line", "question": "マージするか", "facts": ["delegation: merge=B < tier C", "gate の記録 %s"], "options": [{"label": "オーナーがマージする", "pick": true}, {"label": "直させる"}], "from": "gate"}`, record))
	f.lib(id, "gate/"+head+".json", fmt.Sprintf(`{"at": "2026-11-21T03:59:00Z", "id": "%s", "pr": "%s", "head": "%s", "tier": "C", "decision": "needs_human", "reasons": ["delegation: merge=B < tier C"], "merged": false}`, id, prURL(n), head))
}

func (f *fake) row(at, by, id, change string) {
	f.log = append(f.log, fmt.Sprintf(`{"at":"%s","by":"%s","id":"%s","change":%s}`, at, by, id, change))
}

func (f *fake) flush() { f.write("kanban/log.jsonl", ledgertest.Log(f.t, f.log...)) }

func (f *fake) collect(id string) *Input {
	f.t.Helper()
	f.flush()
	in, err := Collect(f.env, id)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := writeInput(f.root, id, in); err != nil {
		f.t.Fatal(err)
	}
	return in
}

func (f *fake) byID(in *Input, id string) *Issue {
	for i := range in.Issues {
		if in.Issues[i].ID == id {
			return &in.Issues[i]
		}
	}
	return nil
}

func str(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// 受け入れ条件 1: dispatcher がマージした課題とオーナーがマージした課題で merged_by が分かれ、waiting は同じ時刻の andon waiting と同じ。
func TestCollect(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.lib("S-0001", "gate/"+sha1+".json", `{"at": "2026-11-21T08:00:00Z", "decision": "approve", "delegation": {"grant": "d2"}}`)
	f.merged("S-0002", 2, "2026-11-21T09:00:00Z", sha2, false)
	f.merged("S-0003", 3, "2026-11-19T09:00:00Z", sha3, true) // 期間の外
	f.issue("S-0004", "done", "parent: S-0001", "profile: codex-review")
	f.row("2026-11-21T08:30:00Z", "t1", "S-0004", `{"status":"done"}`) // レビュアーは数えない
	f.issue("S-0005", "done")
	f.row("2026-11-21T07:00:00Z", "t1", "S-0005", `{"status":"done"}`) // PR の無い片付け
	f.issue("S-0006", "done", "routine: retro")
	f.merged("S-0007", 7, "2026-11-21T07:30:00Z", sha3, true)
	f.write("kanban/issues/S-0007/issue.yaml", "id: S-0007\ntitle: 題 S-0007\nstatus: done\nparent: S-0006\n")
	f.issue("S-0008", "review")
	f.lib("S-0008", "report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, sha1, prURL(8)))
	f.lib("S-0008", "gate/"+sha1+".json", `{"at": "2026-11-21T09:30:00Z", "decision": "fix"}`)
	f.write(f.env.Delivered, `{"at":"2026-11-21T06:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0008","kind":"stalled"}]}`+"\n")
	f.mergeWait("S-0009", 9, sha2, "マージを待つ")
	in := f.collect("S-0100")

	if in.Since != "2026-11-20T10:00:00Z" || in.Until != "2026-11-21T10:00:00Z" || in.Previous != nil || in.Truncated {
		t.Errorf("期間 %s〜%s previous=%v truncated=%v", in.Since, in.Until, in.Previous, in.Truncated)
	}
	for id, want := range map[string]string{"S-0001": "dispatcher", "S-0002": "person", "S-0007": "dispatcher"} {
		x := f.byID(in, id)
		if x == nil || str(x.MergedBy) != want {
			t.Errorf("%s の merged_by: %v（want %s）", id, x, want)
		}
	}
	if x := f.byID(in, "S-0001"); x == nil || str(x.Grant) != "d2" || str(x.Tier) != "B" || str(x.PR) != prURL(1) {
		t.Errorf("S-0001 の grant・tier・pr: %+v", x)
	}
	if x := f.byID(in, "S-0002"); x == nil || x.Grant != nil {
		t.Errorf("オーナーがマージした課題に grant がある: %+v", x)
	}
	if x := f.byID(in, "S-0007"); x == nil || !x.Retro {
		t.Errorf("振り返りの課題から切った課題に retro が無い: %+v", x)
	}
	if x := f.byID(in, "S-0005"); x == nil || x.MergedBy != nil || str(x.DoneAt) != "2026-11-21T07:00:00Z" {
		t.Errorf("PR の無い片付け: %+v", x)
	}
	for _, id := range []string{"S-0003", "S-0004", "S-0100"} {
		if f.byID(in, id) != nil {
			t.Errorf("%s を数えた", id)
		}
	}
	kinds := map[string]bool{}
	for _, c := range in.Concerns {
		kinds[c.Kind+"|"+str(c.Issue)] = true
	}
	if !kinds["gate_fix|S-0008"] || !kinds["stalled|S-0008"] {
		t.Errorf("気になること: %+v", in.Concerns)
	}
	want, _, err := waiting.Read(f.root, now)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := contract.Marshal(in.Waiting), contract.Marshal(want); !bytes.Equal(a, b) {
		t.Errorf("waiting が andon waiting と違う:\n%s\n%s", a, b)
	}
	if len(in.Waiting.Items) == 0 {
		t.Errorf("待っているものが無い（S-0009 のマージの依頼があるはず）: %+v", in.Waiting)
	}
	// 読めなかったもの（GitHub を読まない）は skipped に出す
	if len(in.Skipped) == 0 {
		t.Errorf("GitHub を読めないのに skipped が空")
	}
}

// 受け入れ条件 2: 期間は前に送った同じルーチンのダイジェストの until から。無ければ 24 時間前、8 日前なら 7 日で切る。組み立てただけのものは数えない。
func TestPeriod(t *testing.T) {
	sent := func(f *fake, id, until string, ok bool) {
		f.issue(id, "done", "routine: daily-digest")
		dest := `{"kind": "files", "at": "` + until + `", "ok": true, "url": "http://files.cell.localhost:1355/x.html"}`
		if !ok {
			dest = `{"kind": "files", "at": "` + until + `", "ok": false, "error": "書けない"}`
		}
		f.lib(id, "digest/sent.json", `{"issue": "`+id+`", "since": "2026-11-01T00:00:00Z", "until": "`+until+`", "destinations": [`+dest+`]}`)
	}
	t.Run("前がある", func(t *testing.T) {
		f := newFake(t)
		sent(f, "S-0090", "2026-11-21T01:00:00Z", true)
		sent(f, "S-0091", "2026-11-20T01:00:00Z", true)
		in := f.collect("S-0100")
		if in.Since != "2026-11-21T01:00:00Z" || in.Previous == nil || in.Previous.Issue != "S-0090" || in.Truncated {
			t.Errorf("since=%s previous=%+v", in.Since, in.Previous)
		}
	})
	t.Run("組み立てただけ・送りそこねたものは数えない", func(t *testing.T) {
		f := newFake(t)
		f.issue("S-0090", "done", "routine: daily-digest")
		f.lib("S-0090", "digest/digest.html", "<html></html>")
		sent(f, "S-0091", "2026-11-21T05:00:00Z", false)
		sent(f, "S-0092", "2026-11-21T03:00:00Z", true)
		f.issue("S-0093", "done", "routine: other")
		f.lib("S-0093", "digest/sent.json", `{"issue": "S-0093", "since": "2026-11-01T00:00:00Z", "until": "2026-11-21T09:00:00Z", "destinations": [{"kind": "files", "at": "2026-11-21T09:00:00Z", "ok": true, "url": "http://x/y"}]}`)
		in := f.collect("S-0100")
		if in.Since != "2026-11-21T03:00:00Z" || in.Previous == nil || in.Previous.Issue != "S-0092" {
			t.Errorf("since=%s previous=%+v", in.Since, in.Previous)
		}
	})
	t.Run("無ければ 24 時間前", func(t *testing.T) {
		in := newFake(t).collect("S-0100")
		if in.Since != "2026-11-20T10:00:00Z" || in.Previous != nil || in.Truncated {
			t.Errorf("since=%s previous=%+v truncated=%v", in.Since, in.Previous, in.Truncated)
		}
	})
	t.Run("8 日前なら 7 日で切る", func(t *testing.T) {
		f := newFake(t)
		f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
		sent(f, "S-0090", "2026-11-13T10:00:00Z", true)
		in := f.collect("S-0100")
		if in.Since != "2026-11-14T10:00:00Z" || !in.Truncated || in.Previous == nil {
			t.Errorf("since=%s truncated=%v previous=%+v", in.Since, in.Truncated, in.Previous)
		}
		if html := buildHTML(t, f, validText()); !strings.Contains(html, "7 日前で切った") {
			t.Errorf("切ったことを冒頭に出していない")
		}
	})
}

func validText() string {
	return `{"headline": ["ボードの表示が増えた。", "オーナーの待ちは 1 件。"], "themes": [{"title": "表示", "body": ["流れを段で見せた。"], "issues": ["S-0001"], "figure": {"type": "steps", "steps": ["集める", "組む", "出す"], "focus": 2}}], "concerns": []}`
}

func buildHTML(t *testing.T, f *fake, text string) string {
	t.Helper()
	paths, errs := Build(f.env, "S-0100", []byte(text))
	if len(errs) > 0 {
		t.Fatalf("build が断った: %v", errs)
	}
	if len(paths) != 3 {
		t.Fatalf("paths: %v", paths)
	}
	b, err := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), htmlFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readMessage(t *testing.T, f *fake) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), messageFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func onlyInput(t *testing.T, f *fake) {
	t.Helper()
	ents, err := os.ReadDir(dir(f.root, "S-0100"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.Name() != inputFile {
			t.Errorf("断ったのに %s を書いた", e.Name())
		}
	}
}

// 受け入れ条件 3: 断る入力は終了コード 1 で、library/digest/ に何も書かない。
func TestBuildRejects(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	theme := func(n int) string {
		var ts []string
		for i := 0; i < n; i++ {
			ts = append(ts, fmt.Sprintf(`{"title": "テーマ %d", "body": ["文。"], "issues": ["S-0001"]}`, i))
		}
		return strings.Join(ts, ",")
	}
	cases := []struct{ name, text, why string }{
		{"入力に無い課題", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0999"]}], "concerns": []}`, "入力に無い課題 S-0999"},
		{"要旨が 1 文", `{"headline": ["一。"], "themes": [], "concerns": []}`, "schema"},
		{"要旨が 6 文", `{"headline": ["一。", "二。", "三。", "四。", "五。", "六。"], "themes": [], "concerns": []}`, "要旨が 6 文"},
		{"テーマが 9", `{"headline": ["一。", "二。"], "themes": [` + theme(9) + `], "concerns": []}`, "テーマが 9"},
		{"テーマの文が 4", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["一。", "二。", "三。", "四。"], "issues": ["S-0001"]}], "concerns": []}`, "の文が 4 文"},
		{"気になることが 4", `{"headline": ["一。", "二。"], "themes": [], "concerns": [{"text": "a", "issues": ["S-0001"]}, {"text": "b", "issues": ["S-0001"]}, {"text": "c", "issues": ["S-0001"]}, {"text": "d", "issues": ["S-0001"]}]}`, "気になることが 4"},
		{"気になることの根拠", `{"headline": ["一。", "二。"], "themes": [], "concerns": [{"text": "a", "issues": ["S-0998"]}]}`, "入力に無い課題 S-0998"},
		{"知らない図の型", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0001"], "figure": {"type": "pie", "steps": ["a", "b"], "focus": 1}}], "concerns": []}`, "schema"},
		{"steps が 7 段", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0001"], "figure": {"type": "steps", "steps": ["1", "2", "3", "4", "5", "6", "7"], "focus": 1}}], "concerns": []}`, "steps が 7 段"},
		{"focus が段の外", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0001"], "figure": {"type": "steps", "steps": ["1", "2"], "focus": 3}}], "concerns": []}`, "focus 3"},
		{"前後が 4 行", `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0001"], "figure": {"type": "before_after", "before": ["1", "2", "3", "4"], "after": ["a"]}}], "concerns": []}`, "行が多い"},
		{"壊れた JSON", `{"headline": [`, "schema"},
	}
	t.Setenv("KANBAN_ROOT", f.root)
	t.Setenv("ANDON_WORKSPACE", f.ws)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := Main([]string{"build", "S-0100", "-"}, strings.NewReader(c.text), &out, &errb); code != 1 {
				t.Errorf("終了コード %d（want 1）: %s", code, errb.String())
			}
			if !strings.Contains(errb.String(), c.why) || out.Len() != 0 {
				t.Errorf("理由に %q が無い・何か出した: out=%q err=%q", c.why, out.String(), errb.String())
			}
			onlyInput(t, f)
		})
	}
	// 当たる場合があること: 同じ入力で正しい JSON は通る
	if _, errs := Build(f.env, "S-0100", []byte(validText())); len(errs) > 0 {
		t.Errorf("正しい JSON を断った: %v", errs)
	}
}

// 受け入れ条件 4: どのテーマにも入らないマージ済みの課題は「そのほか」に題で出る。
func TestBuildRest(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.merged("S-0002", 2, "2026-11-21T09:00:00Z", sha2, false)
	f.write("kanban/issues/S-0002/issue.yaml", "id: S-0002\ntitle: テーマに入らなかった直し\nstatus: done\n")
	f.collect("S-0100")
	html := buildHTML(t, f, validText())
	i := strings.Index(html, "そのほか")
	if i < 0 {
		t.Fatalf("「そのほか」の節が無い")
	}
	rest := html[i:]
	rest = rest[:strings.Index(rest, "</div>")]
	if !strings.Contains(rest, "テーマに入らなかった直し") || !strings.Contains(rest, prURL(2)) {
		t.Errorf("そのほかに題と PR が無い: %s", rest)
	}
	if strings.Contains(rest, "題 S-0001") {
		t.Errorf("テーマに入った課題をそのほかにも並べた: %s", rest)
	}
}

// 受け入れ条件 5: HTML は依存の無い単一のファイル。壊した HTML で検査が当たることも同じ検査で確かめる。
func TestSingleFile(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	html := buildHTML(t, f, `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0001"], "figure": {"type": "before_after", "before": ["前"], "after": ["後"]}}], "concerns": [{"text": "気になる。", "issues": ["S-0001"]}]}`)
	if probs := CheckHTML([]byte(html)); len(probs) > 0 {
		t.Errorf("組み立てた HTML が通らない: %v", probs)
	}
	for _, want := range []string{"prefers-color-scheme: dark", `name="viewport"`, "<svg", "max-width"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML に %q が無い", want)
		}
	}
	broken := map[string]string{
		"script":             strings.Replace(html, "</main>", "<script>alert(1)</script></main>", 1),
		"外の画像":               strings.Replace(html, "</main>", `<img src="https://example.com/a.png"></main>`, 1),
		"引用符の無い src":         strings.Replace(html, "</main>", `<img src=//example.com/a.png></main>`, 1),
		"SVG の外の参照":          strings.Replace(html, "</svg>", `<image href="https://example.com/a.png"></image></svg>`, 1),
		"url(":               strings.Replace(html, "</style>", "body { background: url(https://example.com/a.png); }</style>", 1),
		"style 属性の url(":     strings.Replace(html, "</main>", `<div style="background: URL (https://example.com/a.png)"></div></main>`, 1),
		"SVG の style の url(": strings.Replace(html, "</svg>", `<style>rect { fill: url(https://example.com/a.svg#g); }</style></svg>`, 1),
		"link":               strings.Replace(html, "</head>", `<link rel="stylesheet" href="a.css"></head>`, 1),
		"ZgotmplZ":           strings.Replace(html, "</main>", `<a href="#ZgotmplZ">x</a></main>`, 1),
		"SVG が無い":            regexp.MustCompile(`(?s)<svg.*?</svg>`).ReplaceAllString(html, ""),
		"1 MiB":              strings.Replace(html, "</main>", strings.Repeat("あ", htmlMax/3+1)+"</main>", 1),
	}
	for name, b := range broken {
		if len(CheckHTML([]byte(b))) == 0 {
			t.Errorf("壊した HTML（%s）が通った", name)
		}
	}
	// a の外向きの href は持ってよい（PR へのリンク）
	if !strings.Contains(html, `<a href="`+prURL(1)+`">`) {
		t.Errorf("PR へのリンクが無い")
	}
}

// 受け入れ条件 6: 琥珀はマージの依頼と判断の行と冒頭の呼ばれている数だけ、赤は失敗して止まっているものだけ。色はクラスで決める。
func TestColors(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.mergeWait("S-0010", 10, sha1, "マージを待つ")
	f.issue("S-0011", "todo", "who: person")
	f.issue("S-0012", "review")
	f.lib("S-0012", "report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, sha2, prURL(12)))
	f.lib("S-0012", "gate/"+sha2+".json", `{"at": "2026-11-21T09:30:00Z", "decision": "fix"}`)
	f.write(f.env.Delivered, `{"at":"2026-11-21T06:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0012","kind":"stalled"}]}`+"\n")
	in := f.collect("S-0100")
	kinds := map[string]int{}
	for _, it := range in.Waiting.Items {
		kinds[it.Kind]++
	}
	if kinds["merge"] != 1 || kinds["task"] != 1 {
		t.Fatalf("検査の前提: 待っているもの %+v", in.Waiting.Items)
	}
	html := buildHTML(t, f, validText())
	body := regexp.MustCompile(`(?s)<style>.*?</style>`).ReplaceAllString(html, "")
	if strings.Contains(body, "style=") {
		t.Errorf("色や大きさを値としてスタイルに渡している")
	}
	rows := regexp.MustCompile(`(?s)<tr( class="([^"]*)")?>(.*?)</tr>`).FindAllStringSubmatch(body, -1)
	amberRows, redRows := 0, 0
	for _, r := range rows {
		switch r[2] {
		case amber:
			amberRows++
			if !strings.Contains(r[3], "マージの依頼") && !strings.Contains(r[3], "判断") {
				t.Errorf("琥珀の行がマージの依頼・判断でない: %s", r[3])
			}
		case red:
			redRows++
			if !strings.Contains(r[3], "止まった担当") {
				t.Errorf("赤の行が止まった担当でない: %s", r[3])
			}
		}
		if strings.Contains(r[3], "手作業") && r[2] != "" {
			t.Errorf("手作業の行に色がある: %s", r[0])
		}
		if strings.Contains(r[3], "gate の差し戻し") && r[2] != "" {
			t.Errorf("gate の差し戻しに色がある: %s", r[0])
		}
	}
	if amberRows != 1 || redRows != 1 {
		t.Errorf("琥珀の行 %d・赤の行 %d（want 1・1）", amberRows, redRows)
	}
	// 琥珀の印が出るのは、行と棒のほかは冒頭の呼ばれている数の1か所だけ
	if n := strings.Count(body, amber); n != 3 {
		t.Errorf("琥珀の印が %d か所（want 3: 行・その棒・呼ばれている数）", n)
	}
	if n := strings.Count(body, red); n != 1 {
		t.Errorf("赤の印が %d か所（want 1）", n)
	}
}

// 受け入れ条件 7: 題に GitHub のトークンの形を置くと、html と message に印が出て、元の文字列が無い。
func TestRedact(t *testing.T) {
	token := "gh" + "p_" + strings.Repeat("A1b2C3d4E5", 4)[:36]
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.write("kanban/issues/S-0001/issue.yaml", "id: S-0001\ntitle: トークン "+token+" を消す\nstatus: done\n")
	f.mergeWait("S-0010", 10, sha1, "待ち "+token)
	in := f.collect("S-0100")
	if in.Redacted == 0 || strings.Contains(string(contract.Marshal(in)), token) {
		t.Errorf("入力で伏せていない: redacted=%d", in.Redacted)
	}
	// 担当の文に置いても伏せる（入力を手で書き戻しても build が伏せる）
	b, _ := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), inputFile))
	f.lib("S-0100", "digest/input.json", regexp.MustCompile(`\[REDACTED:[a-z-]+\]`).ReplaceAllString(string(b), token))
	html := buildHTML(t, f, `{"headline": ["`+token+` を消した。", "二。"], "themes": [], "concerns": []}`)
	msg := readMessage(t, f)
	for name, s := range map[string]string{"html": html, "message": msg} {
		if strings.Contains(s, token) {
			t.Errorf("%s に元の文字列がある", name)
		}
		if !strings.Contains(s, "[REDACTED:") {
			t.Errorf("%s に伏せた印が無い", name)
		}
	}
	if !strings.Contains(msg, "[REDACTED:") || !strings.Contains(msg, "待ち ") {
		t.Errorf("message の待っているものの行に印が無い: %s", msg)
	}
}

// 受け入れ条件 8: 送り先は files/ だけ。project.json に digest が無いときも同じで、URL を出す。済んだ送り先には送り直さない。
func TestSendNoDestination(t *testing.T) {
	for name, project := range map[string]string{
		"project.json が無い":          "",
		"project.json に digest が無い": `{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": []}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			if project != "" {
				f.write("project.json", project)
			}
			f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
			f.collect("S-0100")
			buildHTML(t, f, validText())
			url, err := Send(f.env, "S-0100")
			if err != nil {
				t.Fatal(err)
			}
			name := "2026-11-21-digest-sample-site-S-0100.html"
			if until, _ := time.Parse(timeLayout, "2026-11-21T10:00:00Z"); until.In(time.Local).Format("2006-01-02") != "2026-11-21" {
				name = until.In(time.Local).Format("2006-01-02") + "-digest-sample-site-S-0100.html"
			}
			if !strings.HasSuffix(url, "/"+name) || !strings.HasPrefix(url, "http://files.cell.localhost:") {
				t.Errorf("URL %s", url)
			}
			if !strings.Contains(readMessage(t, f), url) {
				t.Errorf("message に files/ の URL が無い")
			}
			out := filepath.Join(f.ws, "files", name)
			if _, err := os.Stat(out); err != nil {
				t.Fatal(err)
			}
			s, err := readSent(filepath.Join(dir(f.root, "S-0100"), sentFile))
			if err != nil || len(s.Destinations) != 1 || !s.Destinations[0].OK || s.Destinations[0].URL != url {
				t.Fatalf("sent.json: %+v %v", s, err)
			}
			// 済んだ送り先には送らない（files/ から消しても打ち直しで作り直さない）
			if err := os.Remove(out); err != nil {
				t.Fatal(err)
			}
			if again, err := Send(f.env, "S-0100"); err != nil || again != url {
				t.Errorf("打ち直し: %s %v", again, err)
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("済んだ送り先に送り直した")
			}
		})
	}
}

// 送りそこねた送り先は、打ち直しで送り直す（決めたこと 16）。
func TestSendRetry(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	buildHTML(t, f, validText())
	f.write(filepath.Join(f.ws, "files"), "files/ の代わりのファイル") // files/ を作れない
	if _, err := Send(f.env, "S-0100"); err == nil {
		t.Fatal("files/ を作れないのに送れた")
	}
	s, err := readSent(filepath.Join(dir(f.root, "S-0100"), sentFile))
	if err != nil || len(s.Destinations) != 1 || s.Destinations[0].OK || s.Destinations[0].Error == "" {
		t.Fatalf("失敗を記録していない: %+v %v", s, err)
	}
	if err := os.Remove(filepath.Join(f.ws, "files")); err != nil {
		t.Fatal(err)
	}
	if _, err := Send(f.env, "S-0100"); err != nil {
		t.Fatalf("送り直せない: %v", err)
	}
	if s, _ := readSent(filepath.Join(dir(f.root, "S-0100"), sentFile)); len(s.Destinations) != 1 || !s.Destinations[0].OK {
		t.Errorf("送り直しの記録: %+v", s)
	}
}

// 短い文は 3000 字まで。超えたら待っているものの行を下から削り、削った数を出す。
func TestMessageLimit(t *testing.T) {
	in := &Input{Title: "デイリーダイジェスト", Project: "sample-site", Since: "2026-11-20T10:00:00Z", Until: "2026-11-21T10:00:00Z"}
	long := strings.Repeat("長", 700)
	for i := 0; i < 8; i++ {
		in.Waiting.Items = append(in.Waiting.Items, waiting.Item{Kind: "merge", ID: fmt.Sprintf("S-%04d", i), Title: long})
	}
	msg := Message(&Text{Headline: []string{"一。", "二。"}}, in, newLoc(""), "http://files.cell.localhost:1355/x.html")
	if n := len([]rune(msg)); n > messageMax {
		t.Errorf("%d 字", n)
	}
	if !strings.Contains(msg, "ほか 6 件") && !strings.Contains(msg, "ほか 7 件") && !strings.Contains(msg, "ほか 5 件") {
		t.Errorf("削った数が無い: %s", msg[len(msg)-200:])
	}
	if !strings.Contains(msg, "全文: http://files.cell.localhost:1355/x.html") {
		t.Errorf("files/ の URL が残っていない")
	}
	for _, mark := range []string{"**", "<", "`"} {
		if strings.Contains(msg, mark) {
			t.Errorf("装飾の記法 %q がある", mark)
		}
	}
}

// 出力の JSON はどれも schema に合う（受け入れ条件 13 の一部）。
func TestOutputsMatchSchema(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	buildHTML(t, f, validText())
	if _, err := Send(f.env, "S-0100"); err != nil {
		t.Fatal(err)
	}
	for file, s := range map[string]contract.Schema{inputFile: contract.SchemaDigestInput, digestFile: contract.SchemaDigest, sentFile: contract.SchemaDigestSent} {
		b, err := os.ReadFile(filepath.Join(dir(f.root, "S-0100"), file))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Check(b); err != nil {
			t.Errorf("%s: %v", file, err)
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			t.Errorf("%s が JSON でない", file)
		}
	}
}

// 反論の段で見つけた道: マージの時刻が前の期間で、done が今の期間（オーナーのマージの片付けが止まっていた）でも、マージとして数える。
// done なのに結果の記録がまだ無い課題は、黙って落とさず skipped に出す。
func TestMergeAcrossBoundary(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-19T08:00:00Z", sha1, false)
	f.log = nil
	f.row("2026-11-21T08:00:00Z", "dispatcher", "S-0001", `{"status":"done"}`)
	f.issue("S-0002", "done")
	f.lib("S-0002", "report.json", fmt.Sprintf(`{"commit": "%s", "pr": "%s"}`, sha2, prURL(2)))
	f.row("2026-11-21T09:00:00Z", "dispatcher", "S-0002", `{"status":"done"}`)
	in := f.collect("S-0100")
	if x := f.byID(in, "S-0001"); x == nil || str(x.MergedBy) != "person" {
		t.Errorf("前の期間にマージして今の期間に done になった課題をマージに数えていない: %+v", x)
	}
	found := false
	for _, s := range in.Skipped {
		if strings.Contains(s.Source, "S-0002") {
			found = true
		}
	}
	if !found {
		t.Errorf("結果の記録の無い done の課題を skipped に出していない: %+v", in.Skipped)
	}
}

// 反論の段で見つけた道: 組み立てたあとに集め直したら、古い HTML を新しい期間として送らない。
func TestSendStaleBuild(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.collect("S-0100")
	buildHTML(t, f, validText())
	f.env.Now = now.Add(time.Hour)
	f.collect("S-0100")
	if _, err := Send(f.env, "S-0100"); err == nil {
		t.Errorf("集め直した入力で、前に組み立てた HTML を送った")
	}
	if _, err := os.Stat(filepath.Join(f.ws, "files")); err == nil {
		t.Errorf("files/ に出した")
	}
	buildHTML(t, f, validText())
	if _, err := Send(f.env, "S-0100"); err != nil {
		t.Errorf("組み立て直したあとに送れない: %v", err)
	}
}

// 反論の段で見つけた道: 課題の題の「curl (」で url( の検査に当たらない（検査はスタイルの中だけを見る）。
func TestTitleWithParen(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.write("kanban/issues/S-0001/issue.yaml", "id: S-0001\ntitle: gh の代わりに curl (標準の HTTP) で呼ぶ\nstatus: done\n")
	f.collect("S-0100")
	html := buildHTML(t, f, validText())
	if !strings.Contains(html, "curl (標準の HTTP)") {
		t.Errorf("題が出ていない")
	}
}

// 反論の段で見つけた道: 担当が添える気になることは、command が見つけた気になることの課題と、オーナーを待っている課題も指せる。
func TestConcernRefs(t *testing.T) {
	f := newFake(t)
	f.merged("S-0001", 1, "2026-11-21T08:00:00Z", sha1, true)
	f.issue("S-0008", "review")
	f.lib("S-0008", "gate/"+sha1+".json", `{"at": "2026-11-21T09:30:00Z", "decision": "fix"}`)
	f.mergeWait("S-0009", 9, sha2, "マージを待つ")
	f.collect("S-0100")
	text := `{"headline": ["一。", "二。"], "themes": [], "concerns": [{"text": "差し戻しが続いた。", "issues": ["S-0008", "S-0009"]}]}`
	if _, errs := Build(f.env, "S-0100", []byte(text)); len(errs) > 0 {
		t.Errorf("気になることの課題・待っている課題を指せない: %v", errs)
	}
	text = `{"headline": ["一。", "二。"], "themes": [{"title": "t", "body": ["文。"], "issues": ["S-0008"]}], "concerns": []}`
	if _, errs := Build(f.env, "S-0100", []byte(text)); len(errs) == 0 {
		t.Errorf("テーマが期間に片付いていない課題を指せた")
	}
}
