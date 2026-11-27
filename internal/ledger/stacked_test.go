package ledger

import (
	"strings"
	"testing"
)

func TestStacked(t *testing.T) {
	sha := strings.Repeat("a", 40)
	cos := func(v any) Row { return Row{By: "andon", Config: map[string]any{"chief_of_staff": v}} }
	set := func(by string, kv ...any) Row {
		ch := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			ch[kv[i].(string)] = kv[i+1]
		}
		return Row{By: "terminal:" + by, ID: "S-0008", Change: ch}
	}
	both := func(by string) Row { return set(by, "stacked_on", "S-0007", "stacked_base", sha) }
	for _, c := range []struct {
		name     string
		rows     []Row
		on, base string
		want     bool
	}{
		{"Chief of Staff の端末が 2 つを書いた", []Row{cos("t5"), both("t5")}, "S-0007", sha, true},
		{"書いたあとで Chief of Staff が起き直した", []Row{cos("t5"), both("t5"), cos("t6")}, "S-0007", sha, true},
		{"番号の短い形で書いても同じ課題", []Row{cos("t5"), set("t5", "stacked_on", "S-7", "stacked_base", sha)}, "S-7", sha, true},
		{"担当の端末が 2 つを書いた", []Row{cos("t5"), both("t9")}, "S-0007", sha, false},
		{"Chief of Staff が書いたあと、担当が分かれ目だけを書き換えた", []Row{cos("t5"), both("t5"), set("t9", "stacked_base", strings.Repeat("b", 40))}, "S-0007", strings.Repeat("b", 40), false},
		{"Chief of Staff が書いたあと、担当が前の課題だけを書き換えた", []Row{cos("t5"), both("t5"), set("t9", "stacked_on", "S-0001")}, "S-0001", sha, false},
		{"今の値が書いた値と違う", []Row{cos("t5"), both("t5")}, "S-0007", strings.Repeat("c", 40), false},
		{"前の課題だけで分かれ目が無い", []Row{cos("t5"), set("t5", "stacked_on", "S-0007")}, "S-0007", "", false},
		{"分かれ目が SHA でない", []Row{cos("t5"), set("t5", "stacked_on", "S-0007", "stacked_base", "main")}, "S-0007", "main", false},
		{"自分の上に積んだ", []Row{cos("t5"), set("t5", "stacked_on", "S-0008", "stacked_base", sha)}, "S-0008", sha, false},
		{"行が無い", nil, "S-0007", sha, false},
	} {
		issue := Issue{"id": "S-0008"}
		if c.on != "" {
			issue["stacked_on"] = c.on
		}
		if c.base != "" {
			issue["stacked_base"] = c.base
		}
		on, base, ok := Stacked(c.rows, issue, "t5")
		if ok != c.want || (ok && (on != "S-0007" || base != c.base)) || (!ok && (on != "" || base != "")) {
			t.Errorf("%s: %q %q %v（want %v）", c.name, on, base, ok, c.want)
		}
	}
}

func TestReviewBase(t *testing.T) {
	sha := strings.Repeat("a", 40)
	set := func(by string, v any) Row {
		return Row{By: by, ID: "S-0008-r1", Change: map[string]any{"review_base": v}}
	}
	for _, c := range []struct {
		name  string
		rows  []Row
		value string
		want  string
	}{
		{"dispatcher が書いた", []Row{set(DispatcherBy, sha)}, sha, sha},
		{"担当の端末が書いた", []Row{set("terminal:t9", sha)}, sha, ""},
		{"Chief of Staff の端末が書いた", []Row{set("terminal:t5", sha)}, sha, ""},
		{"dispatcher が書いたあと、担当が書き換えた", []Row{set(DispatcherBy, sha), set("terminal:t9", strings.Repeat("b", 40))}, strings.Repeat("b", 40), ""},
		{"今の値が書いた値と違う", []Row{set(DispatcherBy, sha)}, strings.Repeat("b", 40), ""},
		{"SHA でない", []Row{set(DispatcherBy, "main")}, "main", ""},
		{"行が無い", nil, sha, ""},
	} {
		rev := Issue{"id": "S-0008-r1"}
		if c.value != "" {
			rev["review_base"] = c.value
		}
		if got := ReviewBase(c.rows, rev); got != c.want {
			t.Errorf("%s: %q（want %q）", c.name, got, c.want)
		}
	}
}
