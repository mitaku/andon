package ghpr

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// fake は n 本のマージされた PR を新しい順に持ち、--limit で切って返す偽の gh。打った --limit を控える。
func fake(n int, limits *[]int) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		limit := 0
		for i, a := range args {
			if a == "--limit" {
				limit, _ = strconv.Atoi(args[i+1])
			}
		}
		*limits = append(*limits, limit)
		var items []string
		for i := n; i >= 1 && len(items) < limit; i-- {
			items = append(items, fmt.Sprintf(`{"number":%d}`, i))
		}
		return []byte("[" + strings.Join(items, ",") + "]"), nil
	}
}

func TestMergedReadsPastLimit(t *testing.T) {
	for _, c := range []struct {
		n      int
		limits string
	}{
		{3, "[1000]"},
		{1000, "[1000 2000]"},
		{1001, "[1000 2000]"},
		{2500, "[1000 2000 4000]"},
	} {
		var limits []int
		b, err := Merged(fake(c.n, &limits), "example/sample-site", "number")
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(b), `"number"`); got != c.n {
			t.Errorf("%d 本のうち %d 本しか読まない", c.n, got)
		}
		if !strings.Contains(string(b), `{"number":1}`) {
			t.Errorf("%d 本のとき一番古い PR が無い", c.n)
		}
		if got := fmt.Sprint(limits); got != c.limits {
			t.Errorf("%d 本のときの --limit = %s（want %s）", c.n, got, c.limits)
		}
	}
}

func TestMergedPassesThroughErrorsAndUnreadable(t *testing.T) {
	if _, err := Merged(func(...string) ([]byte, error) { return nil, fmt.Errorf("gh pr list: boom") }, "r/r", "number"); err == nil || err.Error() != "gh pr list: boom" {
		t.Errorf("gh の誤り = %v", err)
	}
	calls := 0
	b, err := Merged(func(...string) ([]byte, error) { calls++; return []byte("not json"), nil }, "r/r", "number")
	if err != nil || string(b) != "not json" || calls != 1 {
		t.Errorf("読めない出力 = %q %v（%d 回打った）", b, err, calls)
	}
}
