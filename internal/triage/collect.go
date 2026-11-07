package triage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Item は集めた1項目。key は <slug>:<出どころの id>:<kind>:<本文の sha1>。
type Item struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	From    string `json:"from"`
	Repo    string `json:"repo"`
	Text    string `json:"text"`
	AgeDays int64  `json:"age_days"`
}

// task は台帳の CLI の list --json の1件のうち、ここが読むキー。
type task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Repo     string `json:"repo"`
	Created  string `json:"created"`
	Who      string `json:"who"`
	Agent    string `json:"agent"`
	Worktree string `json:"worktree"`
}

// personSuffix は人の判断待ちの課題の題の末尾。
const personSuffix = "（人の判断待ち）"

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
	return writeJSON(stdout, items)
}

// collect は台帳から未処理の項目を集める。前回までの記録で keep 以外に仕分けた key は出さない。
// 読むだけで、台帳にもリポジトリにも書かない。
func (l *ledger) collect() ([]Item, error) {
	out, err := l.kanban("", "list", "--all", "--json")
	if err != nil {
		return nil, fail("台帳を読めない")
	}
	var tasks []task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fail("台帳を読めない")
	}
	records, err := l.records()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range records {
		for _, it := range r.items {
			// keep は次回に回しただけ。次も出す。
			if it.decision() != "keep" {
				seen[it.key()] = true
			}
		}
	}
	terms, err := l.terminalTasks()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Created != tasks[j].Created {
			return tasks[i].Created < tasks[j].Created
		}
		return tasks[i].ID < tasks[j].ID
	})

	byKey := map[string]Item{}
	for _, t := range tasks {
		if !idRe.MatchString(t.ID) {
			return nil, fail("台帳の id が不正")
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
		if t.Status == "todo" || t.Status == "queued" {
			if t.Who == "person" || strings.HasSuffix(t.Title, personSuffix) {
				emit("person", t.Title)
			} else if t.Agent == "" {
				emit("task", t.Title)
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
			emit("unreadable", "報告が無い・読めない、または見出しが無い")
			continue
		}
		for _, ln := range reportLines(body) {
			emit(ln.kind, ln.text)
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

// terminalTasks は hc の端末のうち、このプロジェクトのものの task ラベル。hc が無ければ空（起こさない）。
func (l *ledger) terminalTasks() (map[string]bool, error) {
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
func (l *ledger) leftover(t task, terms map[string]bool) bool {
	wt := expandHome(t.Worktree)
	if wt != "" {
		if _, err := os.Stat(wt); err == nil {
			return true
		}
		if t.Repo != "" {
			root := os.Getenv("GHQ_ROOT")
			if root == "" {
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

var bulletRe = regexp.MustCompile(`^\s*[-*]`)

// reportLines は報告から項目になる行を拾う。見出し（## ）で節を切り、
// For AGENTS.md の行・「見送」を含む行とその下の箇条書き・Next／Remember／人に聞くこと の節の行。
func reportLines(body []byte) []reportLine {
	var got []reportLine
	section, waived := "", false
	for _, ln := range splitLines(body) {
		if rest, ok := strings.CutPrefix(ln, "## "); ok {
			section, waived = rest, false
		}
		if strings.TrimFunc(ln, unicode.IsSpace) == "" {
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
			case "人に聞くこと":
				kind = "question"
			}
		}
		if kind != "" {
			got = append(got, reportLine{kind, ln})
		}
	}
	return got
}
