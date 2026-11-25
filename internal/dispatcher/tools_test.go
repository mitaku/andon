package dispatcher

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/tools"
	"andon/internal/tools/toolstest"
)

// reviewerTools は hc・kanban・gh を 1 つの記録（$LOG）に並べる偽物。hc run と kanban set の順と、偽の mise との順を見る。
const reviewerTools = `#!/usr/bin/env bash
tool=${0##*/}
printf '%s %s\n' "$tool" "$*" >>"$LOG"
case "$tool $1" in
  "gh pr") echo "$FAKE_HEAD" ;;
  "hc get") echo '{"labels":{}}' ;;
  "hc run") echo '{"address":"t9","first_message":{"sent":true,"started":true}}' ;;
  "kanban set"|"kanban start") ;;
  *) echo "unexpected $tool $*" >&2; exit 9 ;;
esac
`

// dispatcher がレビュアーを起動する口: hc run の前に、レビュアーの課題の kanban に worktree を書き（受け入れ条件 5）、
// PR の head の worktree の根（cwd の issues/<rev>/ でない）で偽の mise を install で 1 回呼ぶ（受け入れ条件 1）。
// 偽の mise が失敗しても、上限を過ぎても、ログに 1 行出して hc run を打つ（受け入れ条件 2）。
func TestReviewerLaunchSatisfiesDeclaration(t *testing.T) {
	const c = "1111111111111111111111111111111111111111"
	for _, m := range []struct{ mode, log string }{
		{"", ""},
		{"fail", "の道具を入れられない（起動は続ける）: mise install が失敗（exit status 1）: mise ERROR boom"},
		{"slow", "の道具が 200ms で入りきらない（mise install は止めずに続けさせ、起動へ進む）"},
	} {
		t.Run("mode="+m.mode, func(t *testing.T) {
			bin := t.TempDir()
			for _, n := range []string{"hc", "kanban", "gh"} {
				os.WriteFile(filepath.Join(bin, n), []byte(reviewerTools), 0o755)
			}
			// 起動の記録が codex --version を聞く。本物の codex は CODEX_HOME に書くので、偽物で答える
			os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\necho codex-cli 0.0.0\n"), 0o755)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			logf := filepath.Join(t.TempDir(), "log")
			t.Setenv("LOG", logf)
			t.Setenv("FAKE_HEAD", c)
			t.Setenv("HOME", t.TempDir())
			fake := toolstest.New(t, m.mode)
			toolstest.Short(t, &tools.Limit, 200*time.Millisecond)
			var log strings.Builder
			p := reviewProject(t, &log, ledger.Issue{"id": "D-0001-r1", "status": "todo", "profile": "codex-review", "parent": "D-0001"})
			p.tasks[0]["reviewer"], p.tasks[0]["review_rounds"] = "codex-review", "2"
			p.rows = append(p.rows, ledger.Row{ID: "D-0001", By: "terminal:t5", Change: map[string]any{"reviewer": "codex-review", "review_rounds": "2"}})
			wt := t.TempDir()
			toolstest.Manifest(t, wt)
			r := &Reviewer{Issue: "D-0001", Commit: c, Round: 1, Worktree: wt, ReviewerIssue: "D-0001-r1"}
			rep := &contract.Report{PR: "https://github.com/example/app/pull/7", ReviewRequest: &contract.ReviewRequest{Round: 1, Limit: 2, Commit: c, Reviewer: "codex-review"}}
			if why, final := p.startReviewer(p.tasks[0], rep, r); why != "" {
				t.Fatalf("起動しない: %s（final=%v）", why, final)
			}
			b, _ := os.ReadFile(logf)
			var got []string
			for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
				switch {
				case strings.HasPrefix(l, "kanban set D-0001-r1 worktree="), strings.HasPrefix(l, "mise "):
					got = append(got, l)
				case strings.HasPrefix(l, "hc run "):
					got = append(got, "hc run")
					if !strings.Contains(l, "--cwd "+filepath.Join(p.l.Dir, "issues", "D-0001-r1")+" codex-review ") {
						t.Errorf("レビュアーの cwd が課題のフォルダでない（検査の前提が崩れた）: %q", l)
					}
				}
			}
			want := []string{"kanban set D-0001-r1 worktree=" + wt, "mise install | cwd=" + wt, "hc run"}
			if !slices.Equal(got, want) {
				t.Errorf("順:\n got %q\nwant %q", got, want)
			}
			if m.log != "" && !strings.Contains(log.String(), "D-0001-r1: "+wt+" の mise.toml "+m.log) {
				t.Errorf("ログに警告が無い: %s", log.String())
			}
			if m.log == "" && strings.Contains(log.String(), "mise") {
				t.Errorf("入っているのにログに出した: %s", log.String())
			}
			if m.mode == "slow" {
				fake.WaitDone(t)
			}
		})
	}
}
