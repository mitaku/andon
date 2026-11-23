package triage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
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
	path      string
	collected time.Time // その回の collect を打った時刻。読めなければゼロ
	items     []item
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
		// collected は担当が decisions.json に書いた文字列。時刻として読めない記録は、項目を伏せる理由にしない。
		if s, ok := o["collected"].(string); ok {
			if at, err := time.Parse(time.RFC3339, s); err == nil {
				r.collected = at
			}
		}
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
	keyRe   = regexp.MustCompile(`^[a-z0-9_-]+:[A-Za-z0-9][A-Za-z0-9_-]*:(issue|person|next|waived|remember|agents|question|leftover|unreadable|memory|signal):[a-f0-9]{40}$`)
	repoRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	tokenRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	// commitRe は判定の commit（担当が見た commit。sha1 か sha256 の 16 進）。
	commitRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
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

// parseDecisions は decisions.json を読み、kanban に手を入れる前に全件を検査する。
// key は kanban の slug で始まらなければならない（ほかの kanban を指させない）。
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
		if c, present := it["commit"]; present && c != nil {
			if cs, ok := c.(string); !ok || !commitRe.MatchString(cs) {
				return nil, nil, false
			}
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

// 閉じ方（decisions.json の close の closing。ADR-0030 決めたこと 5 の 1）。
// 根拠の閉じはどの種類の項目にも使える。判断の閉じは、使える項目の種類が決まっている。
var (
	evidenceClosings = []string{"done", "gone", "wiki", "delegated"}
	judgmentClosings = map[string][]string{
		"wont-happen":    {"waived"},             // 起きない: 見送りの項目だけ
		"one-off":        {"remember", "memory"}, // その場限り: ## Remember の行と auto memory だけ
		"not-applicable": {"memory"},             // 当てはまらない: auto memory だけ
		"read":           {"unreadable"},         // 読んだ: 拾うものなし: 読めない報告だけ
	}
)

func (i item) closing() string { return i.str("closing") }

// closesIssue は kanban の課題そのもの（担当の無い課題・オーナーの判断待ち）を閉じる判定か。
func (i item) closesIssue() bool {
	k := i.keyPart(2)
	return i.decision() == "close" && (k == "issue" || k == "person")
}

// judged は判断の閉じか（担当の判断だけで閉じる）。
func (i item) judged() bool {
	_, ok := judgmentClosings[i.closing()]
	return i.decision() == "close" && ok
}

// needsVerdict は Chief of Staff の判定が無ければ当てない close か（判断の閉じと、課題を閉じる判定。決めたこと 1・6）。
func (i item) needsVerdict() bool { return i.judged() || i.closesIssue() }

// checkClosings は close の閉じ方の欄と、項目の種類との組み合わせを確かめる。
// 欄の無い close・知らない値・組み合わせに合わない close を断る。close でない判定の closing は値だけ確かめる。
func checkClosings(items []item) error {
	for _, it := range items {
		c, present := it["closing"]
		if !present || c == nil {
			if it.decision() == "close" {
				return fail("close に閉じ方（closing）が無い: %s", it.key())
			}
			continue
		}
		cs, _ := c.(string)
		kinds, judged := judgmentClosings[cs]
		if !judged && !slices.Contains(evidenceClosings, cs) {
			return fail("閉じ方（closing）が不正: %s（%v か %v）", it.key(), evidenceClosings, judgmentNames())
		}
		if it.decision() == "close" && judged && !slices.Contains(kinds, it.keyPart(2)) {
			return fail("閉じ方 %s は %s の項目に使えない（使えるのは %s）: %s", cs, it.keyPart(2), strings.Join(kinds, "・"), it.key())
		}
	}
	return nil
}

func judgmentNames() []string {
	var got []string
	for k := range judgmentClosings {
		got = append(got, k)
	}
	slices.Sort(got)
	return got
}

// capIssues は起動する課題（fix と、bundle の束）を入力の順に max まで数え、超えたものを keep にする。
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
			c["reason"] = "次回（起動する上限）: " + it.str("reason")
			got = append(got, c)
		}
	}
	return got
}

// applyGroups は kanban に当てる単位。bundle は名前ごとに1つ、ほかは1件ずつ。最初の1件の順に並べる。
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
