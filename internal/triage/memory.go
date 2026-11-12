package triage

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// memoryIndex は memory の目録。項目にしない（ADR-0020）。
const memoryIndex = "MEMORY.md"

// MemoryDir は auto memory の置き場: <セルの repo>/memory（ANDON_WORKSPACE。既定 /workspace。ADR-0020 決めたこと 2）。
func MemoryDir() string {
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
	entries, err := MemoryFiles(l.memory)
	if err != nil {
		return nil, err
	}
	var got []Item
	for _, e := range entries {
		name := e.Name()
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

// MemoryFiles は置き場の直下の memory のファイル（.md の普通のファイル。目録を除く）。置き場が無ければ空。
// トリアージが拾うものと dispatcher が数えるもの（ADR-0020 決めたこと 4 の量の線）は、この1つで決まる。
func MemoryFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fail("memory の置き場を読めない")
	}
	got := entries[:0]
	for _, e := range entries {
		if name := e.Name(); e.Type().IsRegular() && strings.HasSuffix(name, ".md") && name != memoryIndex {
			got = append(got, e)
		}
	}
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
// 消す前に今の本文を収集のときの key（本文の sha1）と照らし、違えば（収集のあと書き換わった）ファイルも目録の行も残して false を返す（A-0155）。
// ファイルがもう無ければ消えたものとして目録の行を消す。目録の行は同時書き込みで欠けうるので、無くても落ちない（A-0144）。
func removeMemory(path, key string) (bool, error) {
	body, err := os.ReadFile(path)
	switch {
	case err == nil:
		sum := sha1.Sum(body)
		if !strings.HasSuffix(key, ":"+hex.EncodeToString(sum[:])) {
			return false, nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, fail("memory を消せない: %s", filepath.Base(path))
		}
	case !os.IsNotExist(err):
		return false, fail("memory を読めない: %s", filepath.Base(path))
	}
	index := filepath.Join(filepath.Dir(path), memoryIndex)
	b, err := os.ReadFile(index)
	if err != nil {
		return true, nil
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
		return true, nil
	}
	tmp := index + ".triage.tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(kept, "")), 0o644); err != nil {
		return true, fail("目録を書けない")
	}
	if err := os.Rename(tmp, index); err != nil {
		os.Remove(tmp)
		return true, fail("目録を書けない")
	}
	return true, nil
}
