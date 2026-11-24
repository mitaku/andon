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

	"andon/internal/contract"
	ledgercli "andon/internal/ledger"
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
	if err := checkPersons(items); err != nil {
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
	repos, past, err := l.preflight(id, items)
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
	// 判定の要る close の全部に Chief of Staff の「閉じてよい」が無ければ、kanban にも記録の置き場にも手を入れずに断る。
	if err := checkVerdicts(items, verdicts, writer(l)); err != nil {
		return err
	}

	return l.applyItems(id, date, base, data, items, past, memoryPath, stdout)
}

// applyItems は確かめ終えた決定（items）を1つずつ kanban に当て、当てるたびに記録（base の .json）を書き直し、終われば要約（.md）を書く。
// date は当てた日、base は記録の置き場（library/triage/<日付>-<id>。拡張子なし）、data は decisions.json の残りの欄、past は閉じる課題ごとに越える問い、
// memoryPath は仕分けの済んだ memory の置き場。
func (l *ledger) applyItems(id, date, base string, data obj, items []item, past map[string][]int, memoryPath map[string]string, stdout io.Writer) error {
	// 当てた決定を1つずつ記録する。途中で kanban の CLI が落ちても、どこまで当たったかが残り、黙って当て直されない。
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
		// 課題を足す決定（fix・bundle）は、足す前に「足している」印（adding）と印を残した時刻（adding_at）を記録に残す。kanban add が
		// 返ったあと記録の前で止まっても、andon issue start が印（タイトルと、Chief of Staff の端末が足した課題か。足した行が無ければ、
		// 課題が印より後に作られたか）からトリアージの課題と分かる（ledger.TriageMade）
		adding := g[0].decision() == "fix" || g[0].decision() == "bundle"
		if adding {
			at := time.Now().UTC().Format(time.RFC3339)
			for _, it := range g {
				it["adding"], it["adding_at"] = true, at
			}
			done = append(done, g...)
			if err := save(); err != nil {
				return err
			}
		}
		newID, keys, err := l.applyGroup(id, date, g, past)
		if err != nil {
			return err
		}
		for _, it := range g {
			delete(it, "adding")
			delete(it, "adding_at")
			if newID != "" {
				it["created_issue"] = newID
			}
			if !adding {
				done = append(done, it)
			}
		}
		if err := save(); err != nil {
			return err
		}
		// 予約キーは、足した課題を記録に残してから書く。ここで止まっても、andon issue start が記録から
		// トリアージの足した課題と分かり、無いキーを既定で書いて起動する（打ち直しは記録済みで断るので要らない）
		if len(keys) > 0 {
			if _, err := l.kanban("", append([]string{"set", newID}, keys...)...); err != nil {
				return err
			}
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
	_, err := io.WriteString(stdout, md)
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
// repo のある課題から出た項目（今回の項目も、前回までの記録の key の見直しも）は、その課題の repo（kanban の値）について
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

// preflight は出どころの課題が kanban にあるかと、閉じる課題が未担当の todo／queued かを確かめ、出どころの課題の repo と、
// 閉じる課題ごとに越える問い（開いている問いの番号。kanban done --past に渡す）を返す。
// memory と signal の項目は出どころが課題ではないので、kanban show で照らさない。
// 閉じる課題に、答えられてまだ受け取られていない問いがあれば断る。閉じるとオーナーの答えを捨てることになる（ADR-0057 決めたこと 9）。
// 越えるのはここで読んだ番号だけで、読んだあとに置かれた問いは kanban done が断る。
func (l *ledger) preflight(id string, items []item) (map[string]string, map[string][]int, error) {
	var sources []string
	for _, it := range items {
		if k := it.keyPart(2); k == "memory" || k == "signal" { // 出どころは課題でなく memory のファイル・信号の記録
			continue
		}
		sources = append(sources, it.keyPart(1))
	}
	slices.Sort(sources)
	repos := map[string]string{}
	past := map[string][]int{}
	for _, src := range slices.Compact(sources) {
		if src == id {
			return nil, nil, fail("自分自身は仕分けできない")
		}
		out, err := l.kanban("", "show", src, "--json")
		if err != nil {
			return nil, nil, fail("出どころが無い: %s", src)
		}
		var show struct {
			Issue     ledgerIssue `json:"issue"`
			Questions []struct {
				N      int    `json:"n"`
				Status string `json:"status"`
			} `json:"questions"`
			Broken string `json:"questionsBroken"`
		}
		if err := json.Unmarshal([]byte(out), &show); err != nil {
			return nil, nil, fail("出どころを読めない: %s", src)
		}
		repos[src] = show.Issue.Repo
		closes := slices.ContainsFunc(items, func(it item) bool {
			return it.keyPart(1) == src && it.closesIssue()
		})
		if !closes {
			continue
		}
		if (show.Issue.Status != "todo" && show.Issue.Status != "queued") || show.Issue.Agent != "" {
			return nil, nil, fail("close の対象が未担当の todo/queued でない: %s", src)
		}
		if show.Broken != "" { // kanban done も読めずに断るので、当てる途中で止まらないよう先に断る
			return nil, nil, fail("close の対象 %s の問いを読めない: %s", src, show.Broken)
		}
		for _, q := range show.Questions {
			switch q.Status {
			case "answered":
				return nil, nil, fail("close の対象 %s に、答えられてまだ受け取られていない問い %s#%d がある（閉じると答えを捨てる。Chief of Staff が受け取ってから仕分け直す）", src, src, q.N)
			case "open":
				past[src] = append(past[src], q.N)
			}
		}
	}
	if err := l.checkOwnerTitles(items); err != nil {
		return nil, nil, err
	}
	return repos, past, nil
}

// checkOwnerTitles は、判断の person が足すオーナーの課題と同じタイトルの片付いていない課題が、もう問いを持っていないかを確かめる。
// 持っていれば、andon issue ask --new と同じ関数は同じ問いを 2 つ足さないよう断るので、当てる途中で止まらないよう先に断る。
// 問いを持たない同じタイトルの課題は、その関数が足し直さずに問いを置くので通す。
func (l *ledger) checkOwnerTitles(items []item) error {
	titles := map[string]string{}
	for _, it := range items {
		if it.decision() == "person" && !it.handWork() {
			titles[it.str("question")+ownerSuffix] = it.key()
		}
	}
	if len(titles) == 0 {
		return nil
	}
	out, err := l.kanban("", "list", "--all", "--json")
	if err != nil {
		return fail("kanban を読めない")
	}
	var issues []ledgerIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return fail("kanban を読めない")
	}
	for _, t := range issues {
		key, ok := titles[t.Title]
		if !ok || t.Status == "done" {
			continue
		}
		out, err := l.kanban("", "show", t.ID, "--json")
		if err != nil {
			return fail("課題を読めない: %s", t.ID)
		}
		var show struct {
			Questions []json.RawMessage `json:"questions"`
		}
		if err := json.Unmarshal([]byte(out), &show); err != nil {
			return fail("課題を読めない: %s", t.ID)
		}
		if len(show.Questions) > 0 {
			return fail("同じ問いのオーナーの課題 %s がもう問いを持っている（その課題を仕分けるか、question を変える）: %s", t.ID, key)
		}
	}
	return nil
}

// applyGroup は1つの決定（bundle は束）を kanban に当て、足した課題の id と、記録のあとに書く予約キーを返す。
// past は閉じる課題ごとに越える問いの番号（preflight が読んだもの）。
func (l *ledger) applyGroup(id, date string, g []item, past map[string][]int) (string, []string, error) {
	first := g[0]
	switch first.decision() {
	case "close":
		if first.closesIssue() {
			src := first.keyPart(1)
			note := fmt.Sprintf("不要（%s・%s）: %s\n", id, date, first.str("reason"))
			if _, err := l.kanban(note, "report", src, "--append"); err != nil {
				return "", nil, err
			}
			args := []string{"done", src}
			if ns := past[src]; len(ns) > 0 {
				refs := make([]string, len(ns))
				for i, n := range ns {
					refs[i] = fmt.Sprintf("%s#%d", src, n)
				}
				args = append(args, "--past", strings.Join(refs, ","))
			}
			if _, err := l.kanban("", args...); err != nil {
				return "", nil, err
			}
		}
	case "fix", "bundle":
		// 予約キーは呼び手が記録のあとに書く（打つのは Chief of Staff の端末なので、andon issue start と gate の履歴の照合が通る）。
		// 値は束の review と既定（ledger.DefaultSettings）。依頼書の Settings はその写し
		profile := first.issueStr("profile")
		keys := ledgercli.DefaultSettings(profile, groupReview(g))
		newID, err := l.kanban(groupBrief(g, keys), "add", first.issueStr("title"), "--repo", first.issueStr("repo"), "--profile", profile, "--brief", "-")
		if err != nil {
			return "", nil, err
		}
		return newID, keys, nil
	case "person":
		var newID string
		var err error
		if first.handWork() {
			newID, err = l.addHandWork(first)
		} else {
			newID, err = l.addDecision(first)
		}
		return newID, nil, err
	}
	return "", nil, nil
}

// groupReview は束の review（どれか1つが on なら on）。
func groupReview(g []item) string {
	for _, it := range g {
		if it.issueStr("review") == "on" {
			return "on"
		}
	}
	return "off"
}

// personQuestion は判断の person から、オーナーの課題に置く問い（contract.md §5）を作る。出し手は apply を打つ Chief of Staff、
// 事実の先頭は出どころの key。種類を決めきれなかった項目は、そのことと理由を事実に 1 行足す（ADR-0057 決めたこと 9・10）。
func personQuestion(it item) *contract.Question {
	q := &contract.Question{Kind: it.str("kind"), Question: it.str("question"), Facts: []string{it.key()}, Reason: it.str("reason"), From: "chief-of-staff"}
	if it.unsure() {
		q.Facts = append(q.Facts, unsureFact+it.str("reason"))
	}
	for i, label := range it.options() {
		o := contract.Option{Label: label}
		if i == 0 {
			pick := true
			o.Pick = &pick
		}
		q.Options = append(q.Options, o)
	}
	return q
}

// unsureFact は、種類を決めきれなかった判断の問いに足す事実の行の頭（続けて何が分からないか〈項目の reason〉）。
const unsureFact = "トリアージは種類を決めきれなかった: "

// addDecision は判断の person を、andon issue ask --new と同じ関数でオーナーの課題と問いにして置く（ADR-0057 決めたこと 7・9）。
func (l *ledger) addDecision(it item) (string, error) {
	newID, _, err := ledgercli.At(l.root).NewOwnerQuestion(personQuestion(it), "", nil)
	if err != nil && newID != "" {
		// トリアージの apply は打ち直せない（記録が先に置かれる）。残った課題は問いも印も無いので andon waiting が判断に数えて灯す。
		return "", fail("%v（トリアージの apply は打ち直さず、andon issue ask %s - で問いを置く）", err, newID)
	}
	return newID, err
}

// addHandWork は手作業の person を、印 only_owner を持つオーナーの課題にする（ADR-0057 決めたこと 8・9）。
// タイトルは question のままで「（オーナーの判断待ち）」を付けない。
func (l *ledger) addHandWork(it item) (string, error) {
	brief := "出どころ: " + it.key() +
		"\nオーナーにしかできない理由: " + it.str("only_owner") +
		"\nしてほしいこと: " + it.str("question") +
		"\n理由: " + it.str("reason") + "\n"
	newID, err := l.kanban(brief, "add", it.str("question"), "--brief", "-")
	if err != nil {
		return "", err
	}
	if _, err := l.kanban("", "set", newID, "who=person", "only_owner="+it.str("only_owner")); err != nil {
		return "", err
	}
	return newID, nil
}

// groupBrief は fix と bundle の依頼書を組む。束ねた項目は同じ brief を写していることが多いので、同じ brief は1回だけ書く。
// Settings は issue.yaml に書く予約キー（keys。key=value の並び）の人が読む写し。
func groupBrief(g []item, keys []string) string {
	var briefs, refs []string
	for _, it := range g {
		if b := it.issueStr("brief"); !slices.Contains(briefs, b) {
			briefs = append(briefs, b)
		}
		refs = append(refs, it.key())
	}
	return "解決すべき問題:\n" + strings.Join(briefs, "\n\n") +
		"\nなぜ: トリアージ " + strings.Join(refs, ", ") +
		"\n完了条件: 上記の依頼を満たす。\n守ること: 依頼の範囲内で作業する。\n\n" + ledgercli.SettingsCopy(keys)
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
	person := is("person")
	hand := func(it item) bool { return person(it) && it.handWork() }
	unsure := func(it item) bool { return person(it) && it.unsure() }
	// 種類を決めきれなかった判断が 3 件を超えたら、オーナーに回す線を見直す知らせを先頭に出す（ADR-0057 決めたこと 10）。
	if count(unsure) > 3 {
		b.WriteString("線引きを見直す\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "# トリアージ %s %s（ゴール: %s）\n", slug, date, cell(goal))
	fmt.Fprintf(&b, "閉じた %d ・ 直す %d（うち束ね %d） ・ オーナーへ %d（判断 %d・手作業 %d・unsure %d） ・ 次回 %d ・ 読めない %d\n",
		count(is("close")), created("fix", "bundle"), created("bundle"),
		count(person), count(person)-count(hand), count(hand), count(unsure), count(is("keep")),
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
	b.WriteString("\n## 直す（起動した課題）\n| 課題 | タイトル | 出どころ | Review |\n|---|---|---|---|\n")
	for _, it := range items {
		if d := it.decision(); d == "fix" || d == "bundle" {
			b.WriteString(row(it["created_issue"], it.issue()["title"], it.key(), it.issue()["review"]))
		}
	}
	// 判断は種類と推奨案（options の先頭）、手作業は「手作業」と印の理由を出す。
	b.WriteString("\n## オーナーへ（オーナーの課題）\n| 課題 | 問い | 種類 | 推奨案（手作業は理由） |\n|---|---|---|---|\n")
	for _, it := range items {
		switch {
		case hand(it):
			b.WriteString(row(it["created_issue"], it["question"], "手作業", it["only_owner"]))
		case person(it):
			kind, pick := it.str("kind"), ""
			if it.unsure() {
				kind += "（unsure）"
			}
			if o := it.options(); len(o) > 0 {
				pick = o[0]
			}
			b.WriteString(row(it["created_issue"], it["question"], kind, pick))
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
