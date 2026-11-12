package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const watch = `name: watch
title: 見守り（Watch）
purpose: 進みを見る
triggers:
  - { name: estimate, kind: notification, notification: estimate, start: read }
steps:
  - id: read
    title: 読む
    by: ai
    skill: chief-of-staff#5
    next: [{ id: act, when: 手を打つ }, { id: done, when: 見るだけ }]
  - { id: act, title: 打つ, by: machine, command: andon x, once: true, next: [{ id: done }] }
  - { id: done, title: 終わる, by: person }
`

func TestRenderWritesChecksAndPrunes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "process/workstreams/watch.yaml", watch)
	write(t, root, "process/states/item.yaml", "name: item\ntitle: もの\nsource: ADR-0017\n"+
		"states: [{ id: a, title: A, record: d:Pending }]\ntransitions: [{ from: \"[*]\", to: a, on: 積む, by: kanban }]\n")
	write(t, root, "docs/process/README.md", "人が書く\n")
	write(t, root, "docs/process/old.md", "定義の無い生成物\n")

	changed, err := Render(root, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "docs/process/item.md docs/process/old.md docs/process/watch.md"
	if got := strings.Join(changed, " "); got != want {
		t.Fatalf("check: %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/process/watch.md")); err == nil {
		t.Fatal("--check が書いた")
	}
	if _, err := Render(root, false); err != nil {
		t.Fatal(err)
	}
	if changed, err := Render(root, true); err != nil || len(changed) != 0 {
		t.Fatalf("作り直すと差分: %v %v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/process/old.md")); err == nil {
		t.Fatal("定義の無い生成物が残る")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "docs/process/README.md")); string(b) != "人が書く\n" {
		t.Fatal("README.md を書き換えた")
	}
	ws, _ := os.ReadFile(filepath.Join(root, "docs/process/watch.md"))
	for _, s := range []string{"```mermaid\nflowchart LR", `t_estimate(("estimate")):::event`, `g_read{"分かれ道"}`,
		`g_read -->|"手を打つ"| s_act`, "subgraph lane_person", "s_done --> fin", "| `act` | `andon x` |  | はい |"} {
		if !strings.Contains(string(ws), s) {
			t.Errorf("watch.md に %q が無い:\n%s", s, ws)
		}
	}
	st, _ := os.ReadFile(filepath.Join(root, "docs/process/item.md"))
	for _, s := range []string{"```mermaid\nstateDiagram-v2", `state "A" as s_a`, "[*] --> s_a: 積む", "| `a` | A | `d:Pending` |"} {
		if !strings.Contains(string(st), s) {
			t.Errorf("item.md に %q が無い:\n%s", s, st)
		}
	}
}

func TestRenderRejects(t *testing.T) {
	for name, c := range map[string]struct{ rel, body, want string }{
		"知らないキー":    {"process/workstreams/w.yaml", "name: w\ntitle: W\npurpose: P\nreads: [x]\n", "reads"},
		"名前とファイル":   {"process/workstreams/w.yaml", "name: v\ntitle: W\npurpose: P\n", "ファイルの名前"},
		"状態の知らないキー": {"process/states/s.yaml", "name: s\ntitle: S\nsource: X\nstates: [{ id: a, title: A, color: red }]\n", "color"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, c.rel, c.body)
			_, err := Render(root, true)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err=%v, want %q", err, c.want)
			}
		})
	}
	root := t.TempDir()
	write(t, root, "process/workstreams/x.yaml", "name: x\ntitle: X\npurpose: P\n")
	write(t, root, "process/states/x.yaml", "name: x\ntitle: X\nsource: S\n")
	if _, err := Render(root, true); err == nil || !strings.Contains(err.Error(), "重なる") {
		t.Fatalf("名前の重なり: err=%v", err)
	}
}

func TestToJSONKeepsScalarTypes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.yaml", "every: 60\nonce: true\nname: x\n")
	b, err := ToJSON(filepath.Join(root, "a.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "{\"every\":60,\"name\":\"x\",\"once\":true}\n" {
		t.Fatalf("got %s", got)
	}
}

func TestRenderRefusesSymlinks(t *testing.T) {
	for name, link := range map[string]string{"docs": "docs", "docs/process": "docs/process", "生成物": "docs/process/watch.md", "消す生成物": "docs/process/old.md"} {
		t.Run(name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			write(t, root, "process/workstreams/watch.yaml", watch)
			write(t, outside, "stale.md", "外のファイル\n")
			target := outside
			if strings.HasSuffix(link, ".md") {
				target = filepath.Join(outside, "stale.md")
				if err := os.MkdirAll(filepath.Join(root, "docs/process"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
				t.Fatal(err)
			}
			for _, check := range []bool{true, false} {
				if _, err := Render(root, check); err == nil || !strings.Contains(err.Error(), "symlink") {
					t.Fatalf("check=%v: err=%v", check, err)
				}
			}
			ents, _ := os.ReadDir(outside)
			if len(ents) != 1 || ents[0].Name() != "stale.md" {
				t.Fatalf("外が変わった: %v", ents)
			}
			if b, _ := os.ReadFile(filepath.Join(outside, "stale.md")); string(b) != "外のファイル\n" {
				t.Fatalf("外のファイルが書き換わった: %q", b)
			}
		})
	}
}

// 確かめたあと書く・消す前に docs/process を外への symlink に差し替えても、外は変わらない（os.Root が止める）。
func TestRenderSwapAfterCheckStaysInside(t *testing.T) {
	for name, stale := range map[string]bool{"書く": false, "消す": true} {
		t.Run(name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			write(t, root, "process/workstreams/watch.yaml", watch)
			write(t, root, "docs/process/README.md", "人が書く\n")
			write(t, outside, "old.md", "外のファイル\n")
			if stale {
				write(t, root, "docs/process/old.md", "定義の無い生成物\n")
			}
			dir := filepath.Join(root, "docs/process")
			afterCheck = func() {
				if err := os.Rename(dir, dir+".moved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, dir); err != nil {
					t.Fatal(err)
				}
				afterCheck = func() {}
			}
			t.Cleanup(func() { afterCheck = func() {} })
			if _, err := Render(root, false); err == nil {
				t.Fatal("差し替えた先へ書いた・消した")
			}
			ents, _ := os.ReadDir(outside)
			if len(ents) != 1 || ents[0].Name() != "old.md" {
				t.Fatalf("外が変わった: %v", ents)
			}
			if b, _ := os.ReadFile(filepath.Join(outside, "old.md")); string(b) != "外のファイル\n" {
				t.Fatalf("外のファイルが書き換わった: %q", b)
			}
		})
	}
}
