package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon"
	"andon/internal/process"

	"go.yaml.in/yaml/v3"
)

// 定義に無い移り方は、台帳の CLI を打つ前に終了コード 1 と理由1行で断る（ADR-0017 受け入れ条件 11）。
func TestRefusesUndefinedMove(t *testing.T) {
	cases := []struct {
		status string
		args   []string
		want   string
	}{
		{"todo", []string{"ready", "T-0001"}, "T-0001 は todo（andon issue ready は doing の課題にだけ打てる。process/states/issue.yaml）"},
		{"done", []string{"review-request", "T-0001"}, "T-0001 は done（andon issue review-request は doing の課題にだけ打てる。process/states/issue.yaml）"},
		{"review", []string{"report", "T-0001"}, "andon issue report は doing の課題にだけ打てる"},
		{"queued", []string{"candidates", "T-0001", "-"}, "andon issue candidates は doing の課題にだけ打てる"},
		{"done", []string{"ask", "T-0001", "-"}, "andon issue ask は todo・doing・review の課題にだけ打てる"},
		{"done", []string{"finish", "T-0001"}, "andon issue finish は review・queued・todo・doing の課題にだけ打てる"},
	}
	for _, c := range cases {
		t.Run(c.status+" "+c.args[0], func(t *testing.T) {
			f := newFixture(t)
			f.task["status"] = c.status
			f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
			f.write("list.json", js([]any{f.task}))
			f.write("kanban/log.jsonl", "")
			f.write("kanban/issues/T-0001/library/report.json", f.report)
			f.write("prhead", f.prHead+"\n")
			var out, errb bytes.Buffer
			rc := Main(c.args, strings.NewReader("## Report\nx\n"), &out, &errb)
			msg := errb.String()
			if rc != 1 || !strings.Contains(msg, c.want) || strings.Count(msg, "\n") != 1 {
				t.Fatalf("rc=%d err=%q want rc 1 with %q in one line", rc, msg, c.want)
			}
			if _, err := os.Stat(filepath.Join(f.root, "reviewed-T-0001")); err == nil {
				t.Fatal("断ったのに kanban review を打った")
			}
		})
	}
}

// 定義に在る移り方は通す（ready は doing から review にする。TestReady が残りを見る）。
func TestAllowsDefinedMove(t *testing.T) {
	for _, c := range []struct{ sub, status string }{
		{"ready", "doing"}, {"ask", "review"}, {"ask", "todo"}, {"finish", "doing"}, {"finish", "review"}, {"start", "todo"},
	} {
		if err := allowed(c.sub, "T-0001", c.status); err != nil {
			t.Errorf("%s を %s で断った: %v", c.sub, c.status, err)
		}
	}
	for _, sub := range []string{"show", "impact", "resume"} {
		if err := allowed(sub, "T-0001", "done"); err != nil {
			t.Errorf("移り方でない %s を断った: %v", sub, err)
		}
	}
}

// 定義の andon issue の command は moves と同じ集まりで、by: deck の移り方は kanban のコマンドだけ（断らない）。
func TestDefinitionMatchesMoves(t *testing.T) {
	f, err := froms(andon.IssueStates)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range moves {
		if len(f[sub]) == 0 {
			t.Errorf("andon issue %s の移り方が process/states/issue.yaml に無い", sub)
		}
	}
	for sub := range f {
		if !slices.Contains(moves, sub) {
			t.Errorf("process/states/issue.yaml の andon issue %s を moves が知らない", sub)
		}
	}
	var sm process.StateMachine
	if err := yaml.Unmarshal(andon.IssueStates, &sm); err != nil {
		t.Fatal(err)
	}
	for _, tr := range sm.Transitions {
		if tr.By == "deck" && tr.Command != "" && !strings.HasPrefix(tr.Command, "kanban ") {
			t.Errorf("by: deck の移り方（%s → %s）の command が kanban でない: %s", tr.From, tr.To, tr.Command)
		}
		if tr.By != "deck" && strings.HasPrefix(tr.Command, "kanban ") {
			t.Errorf("kanban を直に打つ移り方（%s → %s）が by: deck でない: %s", tr.From, tr.To, tr.By)
		}
	}
}
