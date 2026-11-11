package triage

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// memoryIndex は memory の目録。項目にしない（ADR-0020）。
const memoryIndex = "MEMORY.md"

// memoryDir は auto memory の置き場: <セルの repo>/memory（ANDON_WORKSPACE。既定 /workspace。ADR-0020 決めたこと 2）。
func memoryDir() string {
	ws := os.Getenv("ANDON_WORKSPACE")
	if ws == "" {
		ws = "/workspace"
	}
	return filepath.Join(ws, "memory")
}

// memoryItems は置き場の直下の .md（目録を除く）を1件1項目（kind: memory）にする。
// 置き場が無ければ項目は無い。出どころ（from）はファイル名から作り、key がファイルごとに分かれる。
func (l *ledger) memoryItems() ([]Item, error) {
	if l.memory == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(l.memory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fail("memory の置き場を読めない")
	}
	var got []Item
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".md") || name == memoryIndex {
			continue
		}
		path := filepath.Join(l.memory, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fail("memory を読めない: %s", name)
		}
		var age int64
		if st, err := e.Info(); err == nil {
			age = max(int64(l.now.Sub(st.ModTime())/(24*time.Hour)), 0)
		}
		from := memoryFrom(name)
		text := string(body)
		got = append(got, Item{Key: itemKey(l.slug, from, "memory", text), Kind: "memory", From: from, Text: text, AgeDays: age, path: path})
	}
	sort.Slice(got, func(i, j int) bool { return got[i].path < got[j].path })
	return got, nil
}

// memoryFrom はファイル名を出どころ（key の2つ目。idRe に合う形）にする: .md を落とし、使えない文字を - にする。
func memoryFrom(name string) string {
	stem := strings.TrimSuffix(name, ".md")
	b := []byte(stem)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			b[i] = '-'
		}
	}
	from := string(b)
	if from == "" || !idRe.MatchString(from) {
		from = "m" + from
	}
	return from
}

// removeMemory は仕分けの済んだ memory のファイルと、目録（MEMORY.md）でそれを指す行を消す。
// 目録の行は同時書き込みで欠けうるので、無くても落ちない（A-0144）。
func removeMemory(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fail("memory を消せない: %s", filepath.Base(path))
	}
	index := filepath.Join(filepath.Dir(path), memoryIndex)
	b, err := os.ReadFile(index)
	if err != nil {
		return nil
	}
	link := "(" + filepath.Base(path) + ")"
	lines := strings.SplitAfter(string(b), "\n")
	kept := lines[:0]
	for _, ln := range lines {
		if !strings.Contains(ln, link) {
			kept = append(kept, ln)
		}
	}
	if len(kept) == len(lines) {
		return nil
	}
	tmp := index + ".triage.tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(kept, "")), 0o644); err != nil {
		return fail("目録を書けない")
	}
	if err := os.Rename(tmp, index); err != nil {
		os.Remove(tmp)
		return fail("目録を書けない")
	}
	return nil
}
