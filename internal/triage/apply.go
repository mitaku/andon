package triage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func runApply(l *ledger, id string, maxIssues int, dry bool, now time.Time, stdout io.Writer) error {
	b, err := os.ReadFile(filepath.Join(l.dir, "issues", id, "library", "decisions.json"))
	if err != nil {
		return fail("decisions.json が無い: %s", id)
	}
	data, items, ok := parseDecisions(b, l.slug)
	if !ok {
		return fail("decisions.json が不正（key・理由・issue・question を確認）")
	}
	if err := l.preflight(id, items); err != nil {
		return err
	}
	// 受けるのは、今回集めた key か、記録にある key（Chief of Staff が見直すとき）だけ。
	// 課題を閉じるのは今の未担当の候補だけ（古い記録に載っていたことを理由に閉じない）。
	inventory, err := l.collect()
	if err != nil {
		return fail("出どころを読めない")
	}
	records, err := l.records()
	if err != nil {
		return err
	}
	current := map[string]string{}
	memoryPath := map[string]string{}
	for _, it := range inventory {
		current[it.Key] = it.Text
		if it.path != "" {
			memoryPath[it.Key] = it.path
		}
	}
	var historical []item
	for _, r := range records {
		historical = append(historical, r.items...)
	}
	for _, it := range items {
		_, inCurrent := current[it.key()]
		known := inCurrent || slices.ContainsFunc(historical, func(h item) bool { return h.key() == it.key() })
		closesIssue := it.decision() == "close" && (it.keyPart(2) == "issue" || it.keyPart(2) == "person")
		if !known || (closesIssue && !inCurrent) {
			return fail("key が収集項目・記録に無い、または close 対象が現在の未担当項目でない")
		}
	}
	// 人に見せる本文は出どころのもの。decisions.json の text は信じない。
	for _, it := range items {
		it["text"] = sourceText(it.key(), inventory, historical)
	}
	items = capIssues(items, maxIssues)
	data["items"] = toAny(items)

	date := now.Format("2006-01-02")
	base := filepath.Join(l.dir, "library", "triage", date+"-"+id)
	for _, ext := range []string{".json", ".md"} {
		if _, err := os.Lstat(base + ext); err == nil {
			return fail("このトリアージは記録済み")
		}
	}
	if dry {
		return writeJSON(stdout, data)
	}

	// 当てた決定を1つずつ記録する。途中で台帳の CLI が落ちても、どこまで当たったかが残り、黙って当て直されない。
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return err
	}
	result := obj{}
	for k, v := range data {
		result[k] = v
	}
	var done []item
	save := func() error {
		result["items"] = toAny(done)
		return os.WriteFile(base+".json", marshalJSON(result), 0o644)
	}
	if err := save(); err != nil {
		return err
	}
	for _, g := range applyGroups(items) {
		newID, err := l.applyGroup(id, date, g)
		if err != nil {
			return err
		}
		for _, it := range g {
			if newID != "" {
				it["created_issue"] = newID
			}
			done = append(done, it)
		}
		if err := save(); err != nil {
			return err
		}
		// 本文と理由を記録に残してから、仕分けの済んだ memory を消す（ADR-0020 決めたこと 5）。keep は残す。
		for _, it := range g {
			if p := memoryPath[it.key()]; p != "" && it.decision() != "keep" {
				if err := removeMemory(p); err != nil {
					return err
				}
			}
		}
	}
	md := summary(l.slug, date, result["goal"], done)
	if err := os.WriteFile(base+".md", []byte(md), 0o644); err != nil {
		return err
	}
	_, err = io.WriteString(stdout, md)
	return err
}

func toAny(items []item) []any {
	got := make([]any, len(items))
	for i, it := range items {
		got[i] = obj(it)
	}
	return got
}

func sourceText(key string, inventory []Item, historical []item) any {
	for _, it := range inventory {
		if it.Key == key {
			return it.Text
		}
	}
	for _, h := range historical {
		if h.key() == key {
			if t, ok := h["text"]; ok && t != nil && t != false {
				return t
			}
			return key
		}
	}
	return key
}

// preflight は出どころの課題が台帳にあるかと、閉じる課題が未担当の todo／queued かを確かめる。
func (l *ledger) preflight(id string, items []item) error {
	var sources []string
	for _, it := range items {
		if it.keyPart(2) == "memory" { // 出どころは課題でなく memory のファイル
			continue
		}
		sources = append(sources, it.keyPart(1))
	}
	slices.Sort(sources)
	for _, src := range slices.Compact(sources) {
		if src == id {
			return fail("自分自身は仕分けできない")
		}
		out, err := l.kanban("", "show", src, "--json")
		if err != nil {
			return fail("出どころが無い: %s", src)
		}
		closes := slices.ContainsFunc(items, func(it item) bool {
			k := it.keyPart(2)
			return it.keyPart(1) == src && (k == "issue" || k == "person") && it.decision() == "close"
		})
		if !closes {
			continue
		}
		var show struct {
			Issue ledgerIssue `json:"issue"`
		}
		if err := json.Unmarshal([]byte(out), &show); err != nil ||
			(show.Issue.Status != "todo" && show.Issue.Status != "queued") || show.Issue.Agent != "" {
			return fail("close の対象が未担当の todo/queued でない: %s", src)
		}
	}
	return nil
}

// applyGroup は1つの決定（bundle は束）を台帳に当て、足した課題の id を返す。
func (l *ledger) applyGroup(id, date string, g []item) (string, error) {
	first := g[0]
	switch first.decision() {
	case "close":
		if k := first.keyPart(2); k == "issue" || k == "person" {
			src := first.keyPart(1)
			note := fmt.Sprintf("不要（%s・%s）: %s\n", id, date, first.str("reason"))
			if _, err := l.kanban(note, "report", src, "--append"); err != nil {
				return "", err
			}
			if _, err := l.kanban("", "done", src); err != nil {
				return "", err
			}
		}
	case "fix", "bundle":
		review := "off"
		var briefs, keys []string
		for _, it := range g {
			if it.issueStr("review") == "on" {
				review = "on"
			}
			briefs = append(briefs, it.issueStr("brief"))
			keys = append(keys, it.key())
		}
		brief := "解決すべき問題:\n" + strings.Join(briefs, "\n\n") +
			"\nなぜ: トリアージ " + strings.Join(keys, ", ") +
			"\n完了条件: 上記の依頼を満たす。\n守ること: 依頼の範囲内で作業する。\n\n## Settings\n- Review: " + review + "\n"
		return l.kanban(brief, "add", first.issueStr("title"), "--repo", first.issueStr("repo"), "--profile", first.issueStr("profile"), "--brief", "-")
	case "person":
		brief := "出どころ: " + first.key() +
			"\nkind: " + orDefault(first["kind"], "判断がつかない") +
			"\n推し: " + orDefault(first["pick"], "なし") +
			"\n理由: " + first.str("reason") + "\n"
		newID, err := l.kanban(brief, "add", first.str("question")+personSuffix, "--brief", "-")
		if err != nil {
			return "", err
		}
		if _, err := l.kanban("", "set", newID, "who=person"); err != nil {
			return "", err
		}
		return newID, nil
	}
	return "", nil
}

// orDefault は jq の `v // d`（null と false なら d）。
func orDefault(v any, d string) string {
	if v == nil || v == false {
		return d
	}
	return cell(v)
}

// cell は表のセル: 文字列はそのまま、ほかは JSON。| と改行を逃がす。
func cell(v any) string {
	s, ok := v.(string)
	if !ok {
		s = strings.TrimSpace(string(marshalJSON(v)))
	}
	s = strings.ReplaceAll(s, "|", "&#124;")
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func row(cells ...any) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = cell(c)
	}
	return "| " + strings.Join(parts, " | ") + " |\n"
}

// summary は人が読む要約（library/triage/<日付>-<id>.md）。
func summary(slug, date string, goal any, items []item) string {
	count := func(f func(item) bool) int {
		n := 0
		for _, it := range items {
			if f(it) {
				n++
			}
		}
		return n
	}
	created := func(decisions ...string) int {
		seen := map[string]bool{}
		for _, it := range items {
			if slices.Contains(decisions, it.decision()) {
				seen[cell(it["created_issue"])] = true
			}
		}
		return len(seen)
	}
	is := func(d string) func(item) bool { return func(it item) bool { return it.decision() == d } }
	text := func(it item) any {
		if t, ok := it["text"]; ok && t != nil && t != false {
			return t
		}
		return it.key()
	}

	var b strings.Builder
	if count(func(it item) bool {
		_, has := it["kind"]
		return it.decision() == "person" && (!has || it["kind"] == nil)
	}) > 3 {
		b.WriteString("線引きを見直す\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "# トリアージ %s %s（ゴール: %s）\n", slug, date, cell(goal))
	fmt.Fprintf(&b, "閉じた %d ・ 直す %d（うち束ね %d） ・ 人へ %d ・ 次回 %d ・ 読めない %d\n",
		count(is("close")), created("fix", "bundle"), created("bundle"), count(is("person")), count(is("keep")),
		count(func(it item) bool { return it.keyPart(2) == "unreadable" }))
	b.WriteString("\n## 閉じた\n| 出どころ | 項目 | 理由 |\n|---|---|---|\n")
	for _, it := range items {
		if it.decision() == "close" {
			b.WriteString(row(it.key(), text(it), it["reason"]))
		}
	}
	b.WriteString("\n## 直す（起こした課題）\n| 課題 | 題 | 出どころ | Review |\n|---|---|---|---|\n")
	for _, it := range items {
		if d := it.decision(); d == "fix" || d == "bundle" {
			b.WriteString(row(it["created_issue"], it.issue()["title"], it.key(), it.issue()["review"]))
		}
	}
	b.WriteString("\n## 人へ（人の課題）\n| 課題 | 問い | kind | 推し |\n|---|---|---|---|\n")
	for _, it := range items {
		if it.decision() == "person" {
			b.WriteString(row(it["created_issue"], it["question"], orDefault(it["kind"], "判断がつかない"), orDefault(it["pick"], "なし")))
		}
	}
	b.WriteString("\n## 次回に回した（keep）\n| 出どころ | 項目 | 待つもの |\n|---|---|---|\n")
	for _, it := range items {
		if it.decision() == "keep" {
			b.WriteString(row(it.key(), text(it), it["reason"]))
		}
	}
	return b.String()
}
