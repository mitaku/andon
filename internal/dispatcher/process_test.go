package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"andon/internal/process"

	"go.yaml.in/yaml/v3"
)

// recordFields は process/states/*.yaml の states[].record と positions[] のうち、記録 rec の項目を
// 「項目 → 出た場所（<ファイル>:<state の id か positions>）」で返す。
func recordFields(t *testing.T, dir, rec string) map[string][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("%s にステートマシンが無い", dir)
	}
	seen := map[string][]string{}
	add := func(field, where string) {
		r, name, ok := strings.Cut(field, ":")
		if ok && r == rec {
			seen[name] = append(seen[name], where)
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var sm process.StateMachine
		d := yaml.NewDecoder(bytes.NewReader(b))
		d.KnownFields(true)
		if err := d.Decode(&sm); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		base := filepath.Base(f)
		for _, s := range sm.States {
			for _, r := range s.Record {
				add(r, base+":"+s.ID)
			}
		}
		for _, p := range sm.Positions {
			add(p, base+":positions")
		}
	}
	return seen
}

// topFields は構造体の一番上の項目の名前。
func topFields(v any) []string {
	typ := reflect.TypeOf(v)
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		out = append(out, typ.Field(i).Name)
	}
	return out
}

// mismatches は fields（構造体の一番上の項目）と seen（定義に出た項目）を照らし、食い違いを 1 行ずつ返す。
func mismatches(rec string, fields []string, seen map[string][]string) []string {
	var out []string
	have := map[string]bool{}
	for _, f := range fields {
		have[f] = true
		switch w := seen[f]; len(w) {
		case 0:
			out = append(out, "項目 "+f+" が process/states/ のどの states[].record にも positions[] にも無い（"+rec+":"+f+" を足す）")
		case 1:
		default:
			out = append(out, "項目 "+f+" が process/states/ に 1 回より多く出る（1 回だけにする）: "+strings.Join(w, "・"))
		}
	}
	var extra []string
	for f := range seen {
		if !have[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(extra)
	for _, f := range extra {
		out = append(out, "process/states/ の "+rec+":"+f+" が構造体に無い（"+strings.Join(seen[f], "・")+"）")
	}
	return out
}

// TestStateFieldsInProcess は dispatcher の控え（State）の一番上の項目が、どれか 1 本のステートマシンの
// states[].record か positions[] に 1 回ずつ出て、定義に State に無い項目が無いことを見る（ADR-0017 受け入れ条件 3。
// 項目を足す・消すときは process/states/ の対応も直す）。
func TestStateFieldsInProcess(t *testing.T) {
	seen := recordFields(t, filepath.Join("..", "..", "process", "states"), "dispatcher")
	for _, m := range mismatches("dispatcher", topFields(State{}), seen) {
		t.Error("State: " + m)
	}
}

// TestMismatches は照合が抜けと重なりと余りを見分けることを見本で確かめる。
func TestMismatches(t *testing.T) {
	dir := t.TempDir()
	def := "name: sample\ntitle: 見本\nsource: 見本\n" +
		"states:\n  - { id: a, title: A, record: [x:One, x:Two] }\n  - { id: b, title: B, record: x:Two }\n" +
		"positions: [x:Gone, y:Three]\n"
	if err := os.WriteFile(filepath.Join(dir, "sample.yaml"), []byte(def), 0o644); err != nil {
		t.Fatal(err)
	}
	type sample struct{ One, Two, Three int }
	got := mismatches("x", topFields(sample{}), recordFields(t, dir, "x"))
	want := []string{
		"項目 Two が process/states/ に 1 回より多く出る（1 回だけにする）: sample.yaml:a・sample.yaml:b",
		"項目 Three が process/states/ のどの states[].record にも positions[] にも無い（x:Three を足す）", // y:Three は別の記録
		"process/states/ の x:Gone が構造体に無い（sample.yaml:positions）",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mismatches:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	ok := mismatches("x", []string{"One"}, map[string][]string{"One": {"sample.yaml:a"}})
	if len(ok) != 0 {
		t.Errorf("揃っているのに食い違い: %v", ok)
	}
}
