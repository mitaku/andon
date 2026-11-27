package gate

import (
	"reflect"
	"testing"
)

func TestParseNumstat(t *testing.T) {
	in := "3\t1\tdocs/a.md\x00-\t-\timg/b.png\x002\t0\t\x00old/c.go\x00new/c.go\x00"
	want := []File{{Path: "docs/a.md", Additions: 3, Deletions: 1}, {Path: "img/b.png"}, {Path: "new/c.go", PreviousPath: "old/c.go", Additions: 2}}
	got, err := ParseNumstat([]byte(in))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %v %v, want %v", got, err, want)
	}
	if got, err := ParseNumstat(nil); err != nil || len(got) != 0 {
		t.Errorf("空: %v %v", got, err)
	}
	for _, bad := range []string{"x\t1\ta\x00", "1\t1\x00", "1\t1\t\x00old\x00"} {
		if _, err := ParseNumstat([]byte(bad)); err == nil {
			t.Errorf("%q を読めないと言わない", bad)
		}
	}
}
