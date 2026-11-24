package triage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"andon/internal/signal"
	"andon/internal/spawn"
)

// Item は集めた1項目。key は <slug>:<出どころの id>:<kind>:<本文の sha1>。
type Item struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
	From string `json:"from"`
	Repo string `json:"repo"`
	// Commit は repo の最新の main（collect が取ってきたもの。Collected.Commits の写し）。担当はこれを読んで仕分け、
	// 判定に写す（ADR-0030 決めたこと 2）。repo の無い項目と、memory・signal の項目は持たない。
	Commit  string `json:"commit,omitempty"`
	Text    string `json:"text"`
	AgeDays int64  `json:"age_days"`
	// kind: signal の項目だけ: オーナーと担当が読む所と、text（出所のタイトル）が外の人の文であることの印。
	URL       string `json:"url,omitempty"`
	Untrusted bool   `json:"untrusted,omitempty"`
	path      string // kind: memory の項目のファイル（apply が消す）
}

// ledgerIssue は kanban の CLI の list --json の1件のうち、ここが読むキー。
type ledgerIssue struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Repo      string   `json:"repo"`
	Created   string   `json:"created"`
	Who       string   `json:"who"`
	Agent     string   `json:"agent"`
	Worktree  string   `json:"worktree"`
	Links     []string `json:"links"`
	NotBefore string   `json:"not_before"`
}

// personSuffix は、オーナーの判断待ちの課題のタイトルの古い末尾（バージョンを上げる前の kanban に残る）。
const personSuffix = "（人の判断待ち）"

// ownerSuffix は、skill が書くようになった新しい末尾（ADR-0036）。読む側は古い末尾と両方を受ける（決めたこと 6）。
const ownerSuffix = "（オーナーの判断待ち）"

// personTitle はタイトルの末尾が、オーナーの判断待ちの印（古い形か新しい形）か。
func personTitle(title string) bool {
	return strings.HasSuffix(title, personSuffix) || strings.HasSuffix(title, ownerSuffix)
}

func itemKey(slug, from, kind, text string) string {
	sum := sha1.Sum([]byte(text))
	return slug + ":" + from + ":" + kind + ":" + hex.EncodeToString(sum[:])
}

func runCollect(l *ledger, limit int, stdout, _ io.Writer) error {
	items, err := l.collect()
	if err != nil {
		return err
	}
	if len(items) > limit {
		return limitError{limit}
	}
	repos, err := l.repos()
	if err != nil {
		return err
	}
	commits, err := latestMains(repos)
	if err != nil {
		return err
	}
	for i, it := range items {
		if carriesCommit(it.Kind) {
			items[i].Commit = commits[it.Repo]
		}
	}
	return writeJSON(stdout, Collected{Commits: commits, Items: items})
}

// Collected は collect の出力。担当はこれをそのまま library/items.json に置き、apply が collect の記録として読む。
// commits は kanban の repo ごとの最新の main（項目の無い repo も。前回までの記録の key を見直すときに照らす）。
type Collected struct {
	Commits map[string]string `json:"commits"`
	Items   []Item            `json:"items"`
}

// repos は kanban の課題の repo（空でないもの）を重ねずに並べる。
func (l *ledger) repos() ([]string, error) {
	out, err := l.kanban("", "list", "--all", "--json")
	if err != nil {
		return nil, fail("kanban を読めない")
	}
	var issues []ledgerIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return nil, fail("kanban を読めない")
	}
	var got []string
	for _, t := range issues {
		if t.Repo != "" {
			got = append(got, t.Repo)
		}
	}
	sort.Strings(got)
	return slices.Compact(got), nil
}

// latestMains は repo ごとに最新の main を取ってきて、その commit を返す。
// 1つでも取れなければ止まる（古い木で仕分けさせない。ADR-0030 決めたこと 5 の 6）。
// apply が中で集め直すときは打たない（取り直すと、担当が読んだ commit から動く）。
func latestMains(repos []string) (map[string]string, error) {
	commits := map[string]string{}
	for _, repo := range repos {
		c, err := latestMain(repo)
		if err != nil {
			return nil, fail("%s の最新の main を取れない（%v）。古い木で仕分けないよう止まる", repo, err)
		}
		commits[repo] = c
	}
	return commits, nil
}

// carriesCommit は、その kind の項目が repo の commit を持つか。memory と外の信号は持たない。
func carriesCommit(kind string) bool { return kind != "memory" && kind != "signal" }

// latestMain は repo（host/owner/repo）の checkout で origin の既定の branch を取り直し、その commit を返す。
// 作業ツリーと手元の branch は動かさない（取り直すのは refs/remotes/origin/<既定の branch> だけ）。
func latestMain(repo string) (string, error) {
	dir, err := spawn.RepoPath(repo)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("checkout が無い: %s", dir)
	}
	sym, err := gitIn(dir, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", err
	}
	def := ""
	for _, ln := range strings.Split(sym, "\n") {
		if f := strings.Fields(ln); len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD" && strings.HasPrefix(f[1], "refs/heads/") {
			def = strings.TrimPrefix(f[1], "refs/heads/")
		}
	}
	if def == "" {
		return "", errors.New("origin の既定の branch が分からない")
	}
	ref := "refs/remotes/origin/" + def
	if _, err := gitIn(dir, "fetch", "-q", "origin", "+refs/heads/"+def+":"+ref); err != nil {
		return "", err
	}
	return gitIn(dir, "rev-parse", "--verify", "-q", ref+"^{commit}")
}

// gitIn は dir で git を打ち、stdout を返す（前後の空白を除く）。資格情報を尋ねて止まらないようにする。
// SSH 鍵はセルに無いので、github.com の SSH の URL は HTTPS で読む（internal/issue の finish と同じ）。
func gitIn(dir string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-C", dir, "-c", "url.https://github.com/.insteadOf=git@github.com:"}, args...)...)
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var errb strings.Builder
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return "", fmt.Errorf("git %s: %v %s", args[0], err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// collect は担当に出す項目を集める。前回までの記録で keep 以外に仕分けた key と、keep にして keepHold たたない key は出さない。
// 読むだけで、kanban にもリポジトリにも書かない（main を取り直すのは runCollect の latestMains）。
func (l *ledger) collect() ([]Item, error) {
	records, err := l.records()
	if err != nil {
		return nil, err
	}
	signals, err := l.loadSignals()
	if err != nil {
		return nil, err
	}
	return l.listFrom(records, signals)
}

// listFrom は読んだ記録と信号で、担当に出す項目を出す順に返す。
// 並びと上限は出す項目の数で決めるので、伏せる分を先に除いてから並べる。
func (l *ledger) listFrom(records []record, signals []signal.Record) ([]Item, error) {
	items, err := l.collectFrom(records, signals)
	if err != nil {
		return nil, err
	}
	return prioritize(withoutHeld(items, records, l.now)), nil
}

// withoutHeld は、集めた時刻から keepHold たたない記録が keep にした項目を除く（ADR-0047 決めたこと 19）。
// どれか1つの記録が当たれば除くので、いちばん新しい記録から数えるのと同じになる。
// 時刻を読めない記録と、now より先の時刻の記録では除かない（書き誤りで項目がいつまでも消えないように）。
func withoutHeld(items []Item, records []record, now time.Time) []Item {
	held := map[string]bool{}
	for _, r := range records {
		if since := now.Sub(r.collected); since < 0 || since >= keepHold {
			continue
		}
		for _, it := range r.items {
			if it.decision() == "keep" {
				held[it.key()] = true
			}
		}
	}
	return slices.DeleteFunc(items, func(it Item) bool { return held[it.Key] })
}

// collectFrom は読んだ記録（records）と信号（signals）で、未処理の項目の全部を古い順に返す。
// 20 時間たたない keep も入る: apply はこれで key を照らすので、伏せた項目も key の名指しで見直して閉じられる。
func (l *ledger) collectFrom(records []record, signals []signal.Record) ([]Item, error) {
	out, err := l.kanban("", "list", "--all", "--json")
	if err != nil {
		return nil, fail("kanban を読めない")
	}
	var issues []ledgerIssue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return nil, fail("kanban を読めない")
	}
	seen := map[string]bool{}
	for _, r := range records {
		for _, it := range r.items {
			// keep は次回に回しただけで、仕分けは済んでいない。
			if it.decision() != "keep" {
				seen[it.key()] = true
			}
		}
	}
	terms, err := l.terminalIssues()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Created != issues[j].Created {
			return issues[i].Created < issues[j].Created
		}
		return issues[i].ID < issues[j].ID
	})

	byKey := map[string]Item{}
	for _, t := range issues {
		if !idRe.MatchString(t.ID) {
			return nil, fail("kanban の id が不正")
		}
		age := ageDays(t.Created, l.now)
		emit := func(kind, text string) {
			k := itemKey(l.slug, t.ID, kind, text)
			if seen[k] {
				return
			}
			if _, dup := byKey[k]; !dup {
				byKey[k] = Item{Key: k, Kind: kind, From: t.ID, Repo: t.Repo, Text: text, AgeDays: age}
			}
		}
		person := t.Who == "person" || personTitle(t.Title)
		// not_before が未来の課題は、見る日まで出さない（閉じられないように。ADR-0011 決めたこと 7）。
		if (t.Status == "todo" || t.Status == "queued") && !notYet(t.NotBefore, l.tz, l.now) {
			if person {
				emit("person", t.Title)
			} else if t.Agent == "" {
				emit("issue", t.Title)
			}
		}
		if t.Status == "done" && l.leftover(t, terms) {
			text := "片付け: " + t.ID
			if wt := expandHome(t.Worktree); wt != "" {
				text += " worktree=" + wt
			}
			emit("leftover", text)
		}
		if t.Status != "done" && t.Status != "review" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(l.dir, "issues", t.ID, "report.md"))
		if err != nil || !hasHeading(body) {
			// オーナーの課題は報告を書かないのがふつう。読む中身が無いので項目にしない（ADR-0030 決めたこと 5 の 2）。
			if !person {
				emit("unreadable", "報告が無い・読めない、または見出しが無い")
			}
			continue
		}
		for _, ln := range reportLines(body) {
			emit(ln.kind, ln.text)
		}
	}
	mem, err := l.memoryItems()
	if err != nil {
		return nil, err
	}
	sig := l.signalItems(issues, signals)
	for _, it := range append(mem, sig...) {
		if _, dup := byKey[it.Key]; !seen[it.Key] && !dup {
			byKey[it.Key] = it
		}
	}
	items := make([]Item, 0, len(byKey))
	for _, it := range byKey {
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AgeDays != items[j].AgeDays {
			return items[i].AgeDays > items[j].AgeDays
		}
		return items[i].Key < items[j].Key
	})
	return items, nil
}

// prioritize は、上限の既定を超える日に memory と signal を先頭に寄せる（その中と残りは古い順のまま）。
// 担当は先頭から N 件だけ仕分けるので、古い順のままだと後ろに来る memory と signal がいつまでも仕分けられない。
func prioritize(items []Item) []Item {
	if len(items) > DefaultLimit {
		sort.SliceStable(items, func(i, j int) bool { return firstOverLimit(items[i].Kind) && !firstOverLimit(items[j].Kind) })
	}
	return items
}

// firstOverLimit は、項目が上限の既定を超える日に先頭に寄せる kind か。memory は仕分けるまで全部の役に載り続け、signal は外の不具合。
func firstOverLimit(kind string) bool { return kind == "memory" || kind == "signal" }

// ageDays は created（2006-01-02T15:04:05Z）から now までの日数（切り捨て・0 以上）。読めなければ 0。
func ageDays(created string, now time.Time) int64 {
	const layout = "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, created)
	if err != nil || len(created) != len(layout) { // 小数の秒は読まない（仮設の jq と同じ）
		return 0
	}
	return max(int64(now.Sub(t)/(24*time.Hour)), 0)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~") {
		return os.Getenv("HOME") + p[1:]
	}
	return p
}

// terminalIssues は hc の端末のうち、このプロジェクトのものの issue ラベル。hc が無ければ空（起動しない）。
func (l *ledger) terminalIssues() (map[string]bool, error) {
	got := map[string]bool{}
	if _, err := exec.LookPath("hc"); err != nil {
		return got, nil
	}
	out, err := exec.Command("hc", "ls", "--all", "--json", "--label", "project="+l.slug).Output()
	if err != nil {
		return nil, fail("端末一覧を読めない")
	}
	var v struct {
		Terminals []struct {
			Labels map[string]any `json:"labels"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fail("端末一覧を読めない")
	}
	for _, t := range v.Terminals {
		if s, ok := t.Labels["issue"].(string); ok {
			got[s] = true
		}
	}
	return got, nil
}

// leftover は done の課題に worktree か端末が残っているか。
func (l *ledger) leftover(t ledgerIssue, terms map[string]bool) bool {
	wt := expandHome(t.Worktree)
	if wt != "" {
		if _, err := os.Stat(wt); err == nil {
			return true
		}
		if t.Repo != "" {
			root, err := spawn.GhqRoot()
			if err != nil {
				root = filepath.Join(os.Getenv("HOME"), ".ghq")
			}
			repo := filepath.Join(root, t.Repo)
			if st, err := os.Stat(repo); err == nil && st.IsDir() {
				out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
				if err == nil {
					for _, ln := range strings.Split(string(out), "\n") {
						if ln == "worktree "+wt {
							return true
						}
					}
				}
			}
		}
	}
	return terms[t.ID]
}

func hasHeading(body []byte) bool {
	for _, ln := range splitLines(body) {
		if strings.HasPrefix(ln, "## ") {
			return true
		}
	}
	return false
}

// splitLines は改行（\n）で切る。\r は行に残す（本文の sha1 が仮設と同じになるように）。
func splitLines(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

type reportLine struct{ kind, text string }

var (
	bulletRe = regexp.MustCompile(`^\s*[-*]`)
	// planDiffRe は Remember の節の「計画との差:」の行（worker skill の報告の書式で担当が必ず書く行。箇条書きでも）。
	planDiffRe = regexp.MustCompile(`^\s*(?:[-*]\s*)?計画との差:`)
)

// reportLines は報告から項目になる行を拾う。見出し（## ）で節を切り、
// For AGENTS.md の行・「見送」を含む行とその下の箇条書き・Next／Remember／オーナーに聞くこと の節の行。
// Remember の節の「計画との差:」の行は拾わない（ADR-0030 決めたこと 5 の 2。振り返りと measure は別に読む）。
func reportLines(body []byte) []reportLine {
	var got []reportLine
	section, waived := "", false
	for _, ln := range splitLines(body) {
		if rest, ok := strings.CutPrefix(ln, "## "); ok {
			section, waived = rest, false
		}
		if strings.TrimFunc(ln, unicode.IsSpace) == "" || (section == "Remember" && planDiffRe.MatchString(ln)) {
			waived = false
			continue
		}
		kind := ""
		switch {
		case strings.Contains(ln, "For AGENTS.md:"):
			kind = "agents"
		case strings.Contains(ln, "見送"):
			kind, waived = "waived", true
		case waived && bulletRe.MatchString(ln):
			kind = "waived"
		case !strings.HasPrefix(ln, "#"):
			waived = false
			switch section {
			case "Next":
				kind = "next"
			case "Remember":
				kind = "remember"
			case "人に聞くこと", "オーナーに聞くこと": // 古い見出しも読む（ADR-0036 決めたこと 6）
				kind = "question"
			}
		}
		if kind != "" {
			got = append(got, reportLine{kind, ln})
		}
	}
	return got
}
