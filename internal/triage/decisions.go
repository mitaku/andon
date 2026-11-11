package triage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// obj は decisions.json と記録の JSON の object。知らないキーもそのまま記録へ運ぶ。
type obj = map[string]any

func decode(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if d.More() {
		return nil, fail("JSON の後ろに余りがある")
	}
	return v, nil
}

// item は decisions.json・記録の items の1件。
type item obj

func (i item) str(k string) string { s, _ := i[k].(string); return s }
func (i item) key() string         { return i.str("key") }
func (i item) decision() string    { return i.str("decision") }

// keyPart は key を : で切った n 番目（1=出どころの id、2=kind）。
func (i item) keyPart(n int) string {
	p := strings.Split(i.key(), ":")
	if n < len(p) {
		return p[n]
	}
	return ""
}

func (i item) issue() obj { t, _ := i["issue"].(obj); return t }

func (i item) issueStr(k string) string { s, _ := i.issue()[k].(string); return s }

type record struct {
	path  string
	items []item
}

// records は library/triage/*.json（前回までの記録）を名前の順に読む。
func (l *ledger) records() ([]record, error) {
	paths, _ := filepath.Glob(filepath.Join(l.dir, "library", "triage", "*.json"))
	var got []record
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fail("記録を読めない: %s", p)
		}
		v, err := decode(b)
		o, ok := v.(obj)
		if err != nil || !ok {
			return nil, fail("記録を読めない: %s", p)
		}
		raw, ok := o["items"].([]any)
		if !ok {
			return nil, fail("記録を読めない: %s", p)
		}
		r := record{path: p}
		for _, x := range raw {
			it, ok := x.(obj)
			if !ok {
				return nil, fail("記録を読めない: %s", p)
			}
			r.items = append(r.items, item(it))
		}
		got = append(got, r)
	}
	return got, nil
}

var (
	keyRe   = regexp.MustCompile(`^[a-z0-9-]+:[A-Za-z0-9][A-Za-z0-9_-]*:(issue|person|next|waived|remember|agents|question|leftover|unreadable|memory):[a-f0-9]{40}$`)
	repoRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

var (
	decisionNames = []string{"close", "fix", "bundle", "person", "keep"}
	personKinds   = []string{"business", "users", "promise", "irreversible"}
)

// nonempty は空白でない文字を1つ以上持つ文字列か。
func nonempty(v any) bool {
	s, ok := v.(string)
	return ok && strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0
}

// parseDecisions は decisions.json を読み、台帳に手を入れる前に全件を検査する。
// key は台帳の slug で始まらなければならない（ほかの台帳を指させない）。
func parseDecisions(b []byte, slug string) (obj, []item, bool) {
	v, err := decode(b)
	if err != nil {
		return nil, nil, false
	}
	data, ok := v.(obj)
	if !ok || !nonempty(data["goal"]) || !nonempty(data["collected"]) {
		return nil, nil, false
	}
	raw, ok := data["items"].([]any)
	if !ok {
		return nil, nil, false
	}
	items := make([]item, 0, len(raw))
	keys := map[string]bool{}
	bundleRepo := map[string]string{}
	for _, x := range raw {
		o, ok := x.(obj)
		if !ok {
			return nil, nil, false
		}
		it := item(o)
		k, ok := it["key"].(string)
		if !ok || keys[k] || !strings.HasPrefix(k, slug+":") || !keyRe.MatchString(k) {
			return nil, nil, false
		}
		keys[k] = true
		if !nonempty(it["reason"]) || strings.Contains(it.str("reason"), "\n") {
			return nil, nil, false
		}
		d, ok := it["decision"].(string)
		if !ok || !slices.Contains(decisionNames, d) {
			return nil, nil, false
		}
		switch d {
		case "fix", "bundle":
			t, ok := it["issue"].(obj)
			if !ok {
				return nil, nil, false
			}
			title, _ := t["title"].(string)
			repo, _ := t["repo"].(string)
			profile, _ := t["profile"].(string)
			review, _ := t["review"].(string)
			if !nonempty(t["title"]) || strings.HasPrefix(title, "-") || !repoRe.MatchString(repo) ||
				!tokenRe.MatchString(profile) || !nonempty(t["brief"]) || (review != "on" && review != "off") {
				return nil, nil, false
			}
			if d == "bundle" {
				if !nonempty(it["bundle"]) {
					return nil, nil, false
				}
				b := it.str("bundle")
				if r, seen := bundleRepo[b]; seen && r != repo {
					return nil, nil, false // 束ねるのは同じ repo だけ
				}
				bundleRepo[b] = repo
			}
		case "person":
			if !nonempty(it["question"]) || strings.HasPrefix(it.str("question"), "-") {
				return nil, nil, false
			}
			if kind, present := it["kind"]; present && kind != nil {
				ks, ok := kind.(string)
				if !ok || !slices.Contains(personKinds, ks) || !nonempty(it["pick"]) {
					return nil, nil, false
				}
			}
		}
		items = append(items, it)
	}
	return data, items, true
}

// capIssues は起こす課題（fix と、bundle の束）を入力の順に max まで数え、超えたものを keep にする。
func capIssues(items []item, max int) []item {
	got := make([]item, 0, len(items))
	var groups []string
	for _, it := range items {
		d := it.decision()
		if d != "fix" && d != "bundle" {
			got = append(got, it)
			continue
		}
		g := "fix:" + it.key()
		if d == "bundle" {
			g = "bundle:" + it.str("bundle")
		}
		switch {
		case slices.Contains(groups, g):
			got = append(got, it)
		case len(groups) < max:
			groups = append(groups, g)
			got = append(got, it)
		default:
			c := item{}
			for k, v := range it {
				c[k] = v
			}
			c["decision"] = "keep"
			c["reason"] = "次回（起こす上限）: " + it.str("reason")
			got = append(got, c)
		}
	}
	return got
}

// applyGroups は台帳に当てる単位。bundle は名前ごとに1つ、ほかは1件ずつ。最初の1件の順に並べる。
func applyGroups(items []item) [][]item {
	var groups [][]item
	index := map[string]int{}
	for _, it := range items {
		if it.decision() == "bundle" {
			name := it.str("bundle")
			if i, ok := index[name]; ok {
				groups[i] = append(groups[i], it)
				continue
			}
			index[name] = len(groups)
		}
		groups = append(groups, []item{it})
	}
	return groups
}
