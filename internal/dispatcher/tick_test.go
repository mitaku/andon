package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 偽の kanban: 打たれたものを $FAKE_KANBAN/log に1行ずつ。tick は $FAKE_KANBAN/tick-rc があればその終了コードで落ちる。
// 一覧と ready は空。偽の hc: get は $FAKE_KANBAN/cos があれば生きている。
const tickKanban = `#!/bin/sh
d=$FAKE_KANBAN
echo "$*" >>"$d/log"
case $1 in
  tick) [ -e "$d/tick-rc" ] && { echo "kanban: routines not read: bad (kanban lint)" >&2; exit "$(cat "$d/tick-rc")"; }; echo "(nothing due)" ;;
  list|ready) echo '[]' ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const tickHc = `#!/bin/sh
[ "$1" = get ] && [ -e "$FAKE_KANBAN/cos" ] && { echo '{"state":"live","status":{"state":"idle"},"labels":{}}'; exit 0; }
exit 3
`

// kanban tick は 1巡の 3（台帳を読む）と 4（ready の差）の間に1回打つ。Chief of Staff の留守（端末が無い）でも打ち、
// chief_of_staff が設定に無い台帳では打たない（ADR-0017 決めたこと 8）。
// 失敗はログに1行で、巡は止めない（dispatcher.md §2.1.1・§7.2 #8。ADR-0007 の切る案 R1）。
func TestPassTicks(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kanban": tickKanban, "hc": tickHc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ws := filepath.Join(d, "ws")
	led := filepath.Join(ws, "projects", "sample-site", "kanban")
	if err := os.MkdirAll(led, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("timezone: UTC\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logb bytes.Buffer
	c := &Dispatcher{WS: ws, Log: &logb}
	calls := func() []string {
		t.Helper()
		b, _ := os.ReadFile(filepath.Join(d, "log"))
		os.Remove(filepath.Join(d, "log"))
		var v []string
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if l != "" {
				v = append(v, strings.Fields(l)[0])
			}
		}
		return v
	}

	c.Pass("")
	if got := calls(); len(got) != 0 {
		t.Errorf("chief_of_staff が設定に無い台帳で kanban を打った: %q", got)
	}
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, round := range []string{"初めて見る台帳", "2巡目"} {
		if round == "2巡目" {
			// 飛ばした 1 行・留守の 1 行は skip_test.go で見る。ここで留守から戻る
			logb.Reset()
			if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		c.Pass("") // 初めて見る台帳は留守のまま（留守でも tick を打つ）
		want := "list tick ready"
		if round == "初めて見る台帳" {
			want = "list ready tick ready" // tick の前の ready を基準に控える
		}
		if got := strings.Join(calls(), " "); got != want {
			t.Errorf("%s: 打った順 %q（%s のはず）", round, got, want)
		}
	}
	if logb.Len() != 0 {
		t.Errorf("通った tick でログを書いた: %q", logb.String())
	}

	if err := os.WriteFile(filepath.Join(d, "tick-rc"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	if got := strings.Join(calls(), " "); got != "list tick ready" {
		t.Errorf("tick が落ちた巡で後ろの段を止めた: %q", got)
	}
	lines := strings.Split(strings.TrimSpace(logb.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "sample-site: kanban tick が通らない") || !strings.Contains(lines[0], "routines not read") {
		t.Errorf("tick の失敗のログが1行でない: %q", logb.String())
	}
}
