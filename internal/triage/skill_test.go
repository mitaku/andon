package triage

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The triage skill names the person kinds that decisions.json takes, and no other: a kind the
// skill sends that apply refuses stops the whole apply.
func TestSkillPersonKindsAreTheOnesApplyTakes(t *testing.T) {
	b, err := os.ReadFile("../../package/skills/triage/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	var named []string
	// `kind: a|b|c` (in the table, | is written \|) and `kind` は `a`／`b`
	for _, m := range regexp.MustCompile("`kind: ([a-z|\\\\]+)`").FindAllStringSubmatch(string(b), -1) {
		named = append(named, strings.Split(strings.ReplaceAll(m[1], `\`, ""), "|")...)
	}
	for _, m := range regexp.MustCompile("`kind` は (`[a-z]+`(?:／`[a-z]+`)+)").FindAllStringSubmatch(string(b), -1) {
		for _, k := range strings.Split(m[1], "／") {
			named = append(named, strings.Trim(k, "`"))
		}
	}
	if len(named) == 0 {
		t.Fatal("the skill names no person kind")
	}
	for _, k := range named {
		if !slices.Contains(personKinds, k) {
			t.Errorf("the skill sends kind %q, which decisions.json doesn't take (%v)", k, personKinds)
		}
	}
	for _, k := range personKinds {
		if !slices.Contains(named, k) {
			t.Errorf("the skill doesn't name kind %q", k)
		}
	}
}
