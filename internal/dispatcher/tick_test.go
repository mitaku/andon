package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
[ "$1" = get ] && [ -e "$FAKE_KANBAN/hcfail" ] && { echo '{"error":{"kind":"internal","message":"x"}}' >&2; exit 1; }
echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2
exit 3
`

// `kanban tick` は 1巡の 3（kanban を読む）と 4（ready の差）の間に1回打つ。Chief of Staff の留守（端末が無い）でも打ち、
// chief_of_staff が設定に無い kanban では打たない（ADR-0017 決めたこと 8）。
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
		t.Errorf("chief_of_staff が設定に無い kanban で kanban の CLI を打った: %q", got)
	}
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, round := range []string{"初めて見る kanban", "2巡目"} {
		if round == "2巡目" {
			logb.Reset()
			if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		c.Pass("")
		want := "list tick ready"
		if round == "初めて見る kanban" {
			want = "list ready tick ready"
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

const routineKanban = `#!/bin/sh
d=$FAKE_KANBAN
case $1 in
  tick) [ -e "$d/due" ] && { rm "$d/due"; n=$(($(cat "$d/n" 2>/dev/null || echo 0) + 1)); echo $n > "$d/n"; }; echo ok ;;
  list) echo '[]' ;;
  ready)
    printf '[{"id":"P-0001","status":"todo","title":"前からある"}'
    i=1; while [ "$i" -le "$(cat "$d/n" 2>/dev/null || echo 0)" ]; do printf ',{"id":"R-000%s","status":"todo","title":"週1の見直し","routine":"weekly"}' "$i"; i=$((i + 1)); done
    echo ']' ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

func TestPassPicksTickedRoutine(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	os.MkdirAll(bin, 0o755)
	for name, body := range map[string]string{"kanban": routineKanban, "hc": tickHc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ws := filepath.Join(d, "ws")
	led := filepath.Join(ws, "projects", "sample-site", "kanban")
	os.MkdirAll(led, 0o755)
	os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t6\n"), 0o644)
	os.WriteFile(filepath.Join(d, "cos"), nil, 0o644)
	c := &Dispatcher{WS: ws, Log: &bytes.Buffer{}}
	readies := func() []string {
		t.Helper()
		st, _, err := LoadState("sample-site")
		if err != nil {
			t.Fatal(err)
		}
		var v []string
		for _, n := range st.Pending {
			if n.Kind == "ready" {
				v = append(v, n.Issue+" "+asJSON(t, n.Detail))
			}
		}
		st.Pending = []Notification{}
		if err := st.Save("sample-site"); err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, round := range []string{"初めて見る kanban", "2巡目"} {
		os.WriteFile(filepath.Join(d, "due"), nil, 0o644)
		c.Pass("")
		id := map[string]string{"初めて見る kanban": "R-0001", "2巡目": "R-0002"}[round]
		want := id + ` {"routine":"weekly","title":"週1の見直し"}`
		if got := readies(); len(got) != 1 || got[0] != want {
			t.Errorf("%s: tick が足した課題だけを同じ巡で routine つきで拾わない: %q（%q のはず）", round, got, want)
		}
	}
	c.Pass("")
	if got := readies(); len(got) != 0 {
		t.Errorf("tick が足した課題を二度拾った: %q", got)
	}
}
