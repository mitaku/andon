package dispatcher

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"andon/internal/ledger"
)

// 偽の hc に kill・rm・run を足し、偽の台帳の CLI（kanban）を置く。どちらも $FAKE_HC/log に1行ずつ書く。
const fallbackHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/$2.json" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/$2.json" ;;
  read) cat "$d/$2.screen" 2>/dev/null ;;
  send) printf 'send %s\n' "$2" >>"$d/log" ;;
  label) printf 'label %s\n' "${*:2}" >>"$d/log" ;;
  kill|rm) printf '%s %s\n' "$1" "$2" >>"$d/log" ;;
  run) printf 'run %s\n' "$*" >>"$d/log"; echo '{"address":"t60","first_message":{"sent":true,"started":true}}' ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

const fakeKanban = `#!/usr/bin/env bash
printf 'kanban %s\n' "$*" >>"$FAKE_HC/log"
case "$*" in *agent=*) [ -e "$FAKE_HC/set-agent-fails" ] && { echo 'lock busy' >&2; exit 1; } ;; esac
exit 0
`

// 実測の Claude Code の使用量の上限の文（pickup_test の ProviderError と同じ）。
const allowanceMsg = "You've hit your usage limit · resets 3pm"

func newFallback(t *testing.T, impl string) *retryFixture {
	t.Helper()
	f := newRetry(t)
	bin := filepath.Join(f.d, "bin")
	for name, body := range map[string]string{"hc": fallbackHc, "kanban": fakeKanban} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.p.tasks = []ledger.Issue{
		{"id": "S-7", "status": "review", "agent": "t42", "profile": impl},
		{"id": "S-7-r1", "parent": "S-7", "status": "doing", "agent": "t50", "profile": "codex-review", "title": "S-7 のレビュー（1 回目）"},
	}
	f.term("t50", "error", allowanceMsg, 0, false)
	return f
}

func TestFallbackRule(t *testing.T) {
	for _, c := range []struct {
		impl, reviewer, want string
		ok                   bool
	}{
		{"claude", "codex-review", "", false}, // claude-review は担当と同じブランド
		{"sonnet", "codex-review", "", false},
		{"codex", "codex-review", "claude-review", true},
		{"codex", "claude-review", "", false},
		{"claude", "claude-review", "codex-review", true},
		{"codex", "reviewer", "", false}, // 知らないプロファイルは起こし直さない
	} {
		if got, ok := Fallback(c.impl, c.reviewer); got != c.want || ok != c.ok {
			t.Errorf("Fallback(%q, %q) = %q, %v; want %q, %v", c.impl, c.reviewer, got, ok, c.want, c.ok)
		}
	}
	if v := Judge(Seen{Status: "error", Msg: allowanceMsg, StatusMs: 5}, nil, Resend{}, 5); !v.Allowance || v.Kind != "error" || v.Retry {
		t.Errorf("使用量の上限の error: %+v", v)
	}
	if v := Judge(Seen{Status: "error", Msg: "Error: tool crashed", StatusMs: 5}, nil, Resend{}, 5); v.Allowance {
		t.Errorf("ほかの error を使用量の上限とした: %+v", v)
	}
}

// 担当が claude 系で codex-review が上限: claude-review へは逃げず、今どおり Chief of Staff へ error を知らせる。
func TestFallbackKeepsBrand(t *testing.T) {
	f := newFallback(t, "claude")
	if got := f.at(0, ""); got != nil {
		t.Fatalf("起こし直そうとした: %q", got)
	}
	if k := f.kinds(); !slices.Equal(k, []string{"S-7-r1:error"}) {
		t.Fatalf("error を知らせていない: %q", k)
	}
	if !strings.Contains(f.log.String(), "別ブランドのプロファイルが残らない") {
		t.Fatalf("ログに理由が無い: %s", f.log.String())
	}
}

// 担当が codex 系で codex-review が上限: 端末を片付け、profile と逃げた理由を台帳に書き、claude-review で起こし直す。知らせない。
func TestFallbackRestarts(t *testing.T) {
	f := newFallback(t, "codex")
	got := f.at(0, "")
	want := []string{"kill t50", "rm t50", "kanban set S-7-r1 profile=claude-review fallback=codex-review → claude-review（使用量の上限で止まった。端末 t50）"}
	if len(got) < 5 || !slices.Equal(got[:3], want) {
		t.Fatalf("起こし直しの手順が違う: %q", got)
	}
	if !strings.HasPrefix(got[3], "run run ") || !strings.Contains(got[3], "--name sample-site-s-7-r1") || !strings.Contains(got[3], "--label issue=S-7-r1") ||
		!strings.Contains(got[3], " claude-review 課題 `sample-site/S-7-r1`。") {
		t.Fatalf("hc run が違う: %q", got[3])
	}
	// Claude 系のレビュアーはセッションの id をラベル session で渡し、台帳の session に書く（andon issue resume が続きから起こす。ADR-0012 決めたこと 8）
	m := regexp.MustCompile(`--label session=([0-9a-f-]{36}) --cwd `).FindStringSubmatch(got[3])
	if m == nil {
		t.Fatalf("hc run に session のラベルが無い: %q", got[3])
	}
	if last := got[len(got)-1]; last != "kanban set S-7-r1 agent=t60 session="+m[1] { // 最初のメッセージは複数行なので run の行は割れる
		t.Fatalf("agent と session を替えていない: %q", last)
	}
	if len(f.p.st.Pending) != 0 {
		t.Fatalf("起こし直したのに知らせた: %q", f.kinds())
	}
}

// 起こし直したあと台帳の agent を書けなければ、起こした端末を片付けて起こし直さなかったことにする（今の見回りは error を知らせる）。
func TestFallbackAgentNotRecorded(t *testing.T) {
	f := newFallback(t, "codex")
	if err := os.WriteFile(filepath.Join(f.d, "set-agent-fails"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := f.at(0, "")
	if n := len(got); n < 2 || got[n-2] != "kill t60" || got[n-1] != "rm t60" {
		t.Fatalf("起こした端末を片付けていない: %q", got)
	}
	if k := f.kinds(); !slices.Equal(k, []string{"S-7-r1:error"}) {
		t.Fatalf("起こし直せなかったのに知らせない: %q", k)
	}
}
