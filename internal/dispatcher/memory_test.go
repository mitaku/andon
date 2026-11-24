package dispatcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ADR-0031 受け入れ条件 B4: プロジェクトが2つある偽のセルで、memory の量の通知が1回に1通だけ届く。
// 記録はセルに1つ（.memory.json）なので、先に回った kanban が積めば、ほかの kanban は積まない（dispatcher.md §2.11）。
// 線を超えたまま回っても積み直さず、線の下に戻ってまた超えたら1通だけ積み直す。
// 積んだ kanban の控えを保存できなかった巡は、記録を書かず、同じ巡のほかの kanban が積む（通知は落ちない）。
// セルの記録だけを書けない巡も1通で、書けない間は積み直さず、書けるようになった巡で書き直す。
func TestMemoryOncePerCell(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ws := filepath.Join(d, "ws")
	t.Setenv("ANDON_WORKSPACE", ws)
	slugs := []string{"sample-a", "sample-b"}
	for i, slug := range slugs {
		led := filepath.Join(ws, "projects", slug, "kanban")
		if err := os.MkdirAll(led, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte(fmt.Sprintf("chief_of_staff: t%d\n", i+1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mem := filepath.Join(ws, "memory")
	if err := os.MkdirAll(mem, 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(i int) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(mem, fmt.Sprintf("m%d.md", i)), []byte("m\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= memoryLine; i++ {
		put(i)
	}
	if err := os.WriteFile(filepath.Join(mem, "MEMORY.md"), []byte("- [m1](m1.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mem, "note.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logb bytes.Buffer
	c := &Dispatcher{WS: ws, Log: &logb}
	memories := func() map[string]int {
		t.Helper()
		got := map[string]int{}
		for _, slug := range slugs {
			st, _, err := LoadState(slug)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range st.Pending {
				if n.Kind == "memory" {
					got[slug]++
					if n.Issue != "" || fmt.Sprint(n.Detail["line"]) != fmt.Sprint(memoryLine) || fmt.Sprint(n.Detail["count"]) != fmt.Sprint(memoryLine+1) {
						t.Errorf("memory の形が違う: %+v", n)
					}
				}
			}
			st.Pending = []Notification{}
			if err := st.Save(slug); err != nil {
				t.Fatal(err)
			}
		}
		return got
	}
	total := func(m map[string]int) int {
		n := 0
		for _, v := range m {
			n += v
		}
		return n
	}

	c.Pass("")
	if got := memories(); total(got) != 0 {
		t.Errorf("線ちょうど（%d 件と目録・.md でないもの）で memory を積んだ: %v", memoryLine, got)
	}

	put(memoryLine + 1)
	c.Pass("")
	if got := memories(); total(got) != 1 || got["sample-a"] != 1 {
		t.Errorf("線を超えた巡の memory が 1 通でない（先に回った sample-a に 1 通のはず）: %v", got)
	}
	if m, err := LoadMemory(); err != nil || !m.Over {
		t.Errorf("積んだのにセルの記録が over でない: %+v %v", m, err)
	}

	put(memoryLine + 2)
	c.Pass("")
	c.Pass("")
	if got := memories(); total(got) != 0 {
		t.Errorf("線を超えたまま memory をもう一度積んだ: %v", got)
	}

	for _, i := range []int{memoryLine + 1, memoryLine + 2} {
		if err := os.Remove(filepath.Join(mem, fmt.Sprintf("m%d.md", i))); err != nil {
			t.Fatal(err)
		}
	}
	c.Pass("")
	if m, err := LoadMemory(); err != nil || m.Over {
		t.Errorf("線の下に戻ったのにセルの記録が over のまま: %+v %v", m, err)
	}
	put(memoryLine + 3)
	c.Pass("")
	if got := memories(); total(got) != 1 {
		t.Errorf("線の下に戻ってまた超えた巡の memory が 1 通でない: %v", got)
	}

	if err := os.Remove(filepath.Join(mem, fmt.Sprintf("m%d.md", memoryLine+3))); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	put(memoryLine + 4)
	tmp := StatePath("sample-a") + ".tmp"
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	logb.Reset()
	c.Pass("")
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	if got := memories(); total(got) != 1 || got["sample-b"] != 1 {
		t.Errorf("sample-a の控えを保存できない巡で、memory が sample-b に 1 通届かない: %v（ログ %q）", got, logb.String())
	}
	c.Pass("")
	if got := memories(); total(got) != 0 {
		t.Errorf("sample-b が積んだあとで memory をもう一度積んだ: %v", got)
	}

	if err := os.Remove(filepath.Join(mem, fmt.Sprintf("m%d.md", memoryLine+4))); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	put(memoryLine + 5)
	mtmp := MemoryPath() + ".tmp"
	if err := os.MkdirAll(mtmp, 0o755); err != nil {
		t.Fatal(err)
	}
	logb.Reset()
	for range 3 {
		c.Pass("")
	}
	if got := memories(); total(got) != 1 {
		t.Errorf("セルの記録を書けない間に memory が 1 通でない: %v（ログ %q）", got, logb.String())
	}
	if m, err := LoadMemory(); err != nil || m.Over || !bytes.Contains(logb.Bytes(), []byte("記録を書けない")) {
		t.Errorf("書けない記録が書けた・ログに出ていない: %+v %v %q", m, err, logb.String())
	}
	if err := os.Remove(mtmp); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	if m, err := LoadMemory(); err != nil || !m.Over {
		t.Errorf("書けるようになった巡で記録を書き直していない: %+v %v", m, err)
	}
	(&Dispatcher{WS: ws, Log: &logb}).Pass("")
	if got := memories(); total(got) != 0 {
		t.Errorf("記録を書き直したあとで memory をもう一度積んだ: %v", got)
	}
}
