package issue

import (
	"reflect"
	"testing"
)

func TestBriefRefs(t *testing.T) {
	brief := "入力（正）: docs/dispatcher.md の §7 と ADR-0006。skill は chief-of-staff/SKILL.md"
	files := []string{"docs/dispatcher.md", "package/skills/chief-of-staff/SKILL.md", "package/skills/worker/SKILL.md", "internal/issue/issue.go"}
	// worker/SKILL.md は名前（SKILL.md）が依頼書に出るので拾う（手がかりで、読んで決めるのは Chief of Staff）
	want := []string{"docs/dispatcher.md", "package/skills/chief-of-staff/SKILL.md", "package/skills/worker/SKILL.md"}
	if got := BriefRefs(brief, files); !reflect.DeepEqual(got, want) {
		t.Errorf("BriefRefs = %v, want %v", got, want)
	}
	if got := BriefRefs("", files); len(got) != 0 || got == nil {
		t.Errorf("空の依頼書で %v", got)
	}
}

func TestIntersect(t *testing.T) {
	got := Intersect([]string{"b", "a", "b", "c"}, []string{"c", "b", "x"})
	if !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Errorf("Intersect = %v", got)
	}
}
