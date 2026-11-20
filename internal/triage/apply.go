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

	"andon/internal/signal"
)

func runApply(l *ledger, id string, maxIssues int, dry bool, now time.Time, stdout io.Writer) error {
	b, err := os.ReadFile(filepath.Join(l.dir, "issues", id, "library", "decisions.json"))
	if err != nil {
		return fail("decisions.json が無い: %s", id)
	}
	data, items, ok := parseDecisions(b, l.slug)
	if !ok {
		return fail("decisions.json が不正（key・理由・commit・issue・question を確認）")
	}
	if err := checkClosings(items); err != nil {
		return err
	}
	verdicts, err := readVerdicts(verdictPath(l, id))
	if err != nil {
		return err
	}
	collected, err := readCollected(filepath.Join(l.dir, "issues", id, "library", "items.json"))
	if err != nil {
		return err
	}
	repos, err := l.preflight(id, items)
	if err != nil {
		return err
	}
	// 受けるのは、今回集めた key か、記録にある key（Chief of Staff が見直すとき）だけ。
	// 課題を閉じるのは今の未担当の候補だけ（古い記録に載っていたことを理由に閉じない）。
	// 記録と信号は1回だけ読み、集めるのと照らすのに同じものを使う。
	records, err := l.records()
	if err != nil {
		return fail("出どころを読めない")
	}
	signals, err := l.loadSignals()
	if err != nil {
		return fail("出どころを読めない")
	}
	inventory, err := l.collectFrom(records, signals)
	if err != nil {
		return fail("出どころを読めない")
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
	// 信号の項目は、collect のあと active でなくなった（CI が通った・issue が閉じた）ものも記録の信号と照らして受ける。
	for _, it := range items {
		_, inCurrent := current[it.key()]
		known := inCurrent || slices.ContainsFunc(historical, func(h item) bool { return h.key() == it.key() })
		if it.keyPart(2) == "signal" && !known {
			_, known = knownSignal(it.key(), signals, l.slug)
		}
		if !known || (it.closesIssue() && !inCurrent) {
			return fail("key が収集項目・記録に無い、または close 対象が現在の未担当項目でない")
		}
	}
	if err := checkCommits(items, collected, repos); err != nil {
		return err
	}
	// オーナーに見せる本文は出どころのもの。decisions.json の text は信じない。
	for _, it := range items {
		it["text"] = sourceText(it.key(), inventory, historical)
		if r, ok := knownSignal(it.key(), signals, l.slug); ok && it.keyPart(2) == "signal" {
			it["text"], it["url"], it["untrusted"] = r.Title, r.URL, true
		}
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
		// Chief of Staff が読む判定（判断の閉じと、課題を閉じる判定）を別の表で出す。判定はまだ要らない。
		out := obj{}
		for k, v := range data {
			out[k] = v
		}
		out["review"] = reviewTable(items, verdicts, writer(l))
		return writeJSON(stdout, out)
	}
	// 判定の要る close の全部に Chief of Staff の「閉じてよい」が無ければ、台帳にも記録の置き場にも手を入れずに断る。
	if err := checkVerdicts(items, verdicts, writer(l)); err != nil {
		return err
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
		// 収集のあと書き換わったものは消さずに残し、理由を記録に書く（新しい中身は次のトリアージで拾う。A-0155）。
		kept := false
		for _, it := range g {
			if p := memoryPath[it.key()]; p != "" && it.decision() != "keep" {
				removed, err := removeMemory(p, it.key())
				if err != nil {
					return err
				}
				if !removed {
					it["memory_kept"] = memoryChanged
					kept = true
				}
			}
		}
		if kept {
			if err := save(); err != nil {
				return err
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

// memoryChanged は消さずに残した memory の理由（記録の memory_kept）。
const memoryChanged = "収集のあと書き換わったので消さずに残した（次のトリアージで拾い直す）"

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

// readCollected は collect の記録（担当が collect の出力を置いた library/items.json）の commits を読む。
// 前の形（項目の配列）と、commits の無いもの・形の合わない commit は断る（照らせない判定を通さない）。
func readCollected(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fail("library/items.json が無い（andon triage collect > library/items.json で collect の出力を置く）")
	}
	var got Collected
	if err := json.Unmarshal(b, &got); err != nil || got.Commits == nil {
		return nil, fail("library/items.json を読めない（今の andon triage collect の出力のまま置く）")
	}
	for repo, c := range got.Commits {
		if repo == "" || !commitRe.MatchString(c) {
			return nil, fail("library/items.json の commits が不正: %s", repo)
		}
	}
	return got.Commits, nil
}

// checkCommits は判定の commit（担当が見た commit）を、collect の出したものと照らす（ADR-0030 決めたこと 2）。
// repo のある課題から出た項目（今回の項目も、前回までの記録の key の見直しも）は、その課題の repo（台帳の値）について
// collect が出した commit と同じでなければならない。commit を持たない項目（repo の無い課題・memory・signal）には書かない。
func checkCommits(items []item, commits, repos map[string]string) error {
	for _, it := range items {
		want := ""
		if repo := repos[it.keyPart(1)]; carriesCommit(it.keyPart(2)) && repo != "" {
			if want = commits[repo]; want == "" {
				return fail("見た commit を照らせない: %s（collect の記録に %s の main が無い。collect からやり直す）", it.key(), repo)
			}
		}
		if it.str("commit") != want {
			return fail("見た commit が collect の出したもの（library/items.json の commits）と違うか無い: %s", it.key())
		}
	}
	return nil
}

// preflight は出どころの課題が台帳にあるかと、閉じる課題が未担当の todo／queued かを確かめ、出どころの課題の repo を返す。
// memory と signal の項目は出どころが課題ではないので、kanban show で照らさない。
func (l *ledger) preflight(id string, items []item) (map[string]string, error) {
	var sources []string
	for _, it := range items {
		if k := it.keyPart(2); k == "memory" || k == "signal" { // 出どころは課題でなく memory のファイル・信号の記録
			continue
		}
		sources = append(sources, it.keyPart(1))
	}
	slices.Sort(sources)
	repos := map[string]string{}
	for _, src := range slices.Compact(sources) {
		if src == id {
			return nil, fail("自分自身は仕分けできない")
		}
		out, err := l.kanban("", "show", src, "--json")
		if err != nil {
			return nil, fail("出どころが無い: %s", src)
		}
		var show struct {
			Issue ledgerIssue `json:"issue"`
		}
		if err := json.Unmarshal([]byte(out), &show); err != nil {
			return nil, fail("出どころを読めない: %s", src)
		}
		repos[src] = show.Issue.Repo
		closes := slices.ContainsFunc(items, func(it item) bool {
			k := it.keyPart(2)
			return it.keyPart(1) == src && (k == "issue" || k == "person") && it.decision() == "close"
		})
		if !closes {
			continue
		}
		if (show.Issue.Status != "todo" && show.Issue.Status != "queued") || show.Issue.Agent != "" {
			return nil, fail("close の対象が未担当の todo/queued でない: %s", src)
		}
	}
	return repos, nil
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
			"\n推奨案: " + orDefault(first["pick"], "なし") +
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
			if s, ok := t.(string); ok && it["untrusted"] == true {
				return signal.Untrusted(s)
			}
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
	fmt.Fprintf(&b, "閉じた %d ・ 直す %d（うち束ね %d） ・ オーナーへ %d ・ 次回 %d ・ 読めない %d\n",
		count(is("close")), created("fix", "bundle"), created("bundle"), count(is("person")), count(is("keep")),
		count(func(it item) bool { return it.keyPart(2) == "unreadable" }))
	// 閉じたものは、Chief of Staff が読んだもの（判断の閉じと、課題を閉じる判定）を先に、根拠の閉じを後に出す（ADR-0030 決めたこと 5 の 5）。
	b.WriteString("\n## 閉じた（判断の閉じと課題の閉じ。Chief of Staff が読んだ）\n| 出どころ | 項目 | 閉じ方 | 理由 |\n|---|---|---|---|\n")
	for _, it := range items {
		if it.needsVerdict() {
			b.WriteString(row(it.key(), text(it), it["closing"], it["reason"]))
		}
	}
	b.WriteString("\n## 閉じた（根拠の閉じ）\n| 出どころ | 項目 | 閉じ方 | 理由 |\n|---|---|---|---|\n")
	for _, it := range items {
		if it.decision() == "close" && !it.needsVerdict() {
			b.WriteString(row(it.key(), text(it), orDefault(it["closing"], "なし"), it["reason"]))
		}
	}
	b.WriteString("\n## 直す（起動した課題）\n| 課題 | 題 | 出どころ | Review |\n|---|---|---|---|\n")
	for _, it := range items {
		if d := it.decision(); d == "fix" || d == "bundle" {
			b.WriteString(row(it["created_issue"], it.issue()["title"], it.key(), it.issue()["review"]))
		}
	}
	b.WriteString("\n## オーナーへ（オーナーの課題）\n| 課題 | 問い | kind | 推奨案 |\n|---|---|---|---|\n")
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
	var overturned []item
	for _, it := range items {
		if it["overturned"] != nil {
			overturned = append(overturned, it)
		}
	}
	if len(overturned) > 0 {
		b.WriteString("\n## Chief of Staff が覆した close（担当が行き先を書き直した）\n| 出どころ | 項目 | 覆した close（閉じ方: 理由） | 書き直した行き先 |\n|---|---|---|---|\n")
		for _, it := range overturned {
			o, _ := it["overturned"].(obj)
			b.WriteString(row(it.key(), text(it), cell(o["closing"])+": "+cell(o["reason"]), it.decision()))
		}
	}
	var kept []string
	for _, it := range items {
		if it["memory_kept"] != nil {
			kept = append(kept, it.key())
		}
	}
	if len(kept) > 0 {
		b.WriteString("\n## 消さずに残した memory\n" + memoryChanged + ": " + strings.Join(kept, ", ") + "\n")
	}
	return b.String()
}
