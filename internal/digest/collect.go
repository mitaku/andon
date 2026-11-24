package digest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/measure"
	"andon/internal/redact"
	"andon/internal/waiting"
)

// Input は library/digest/input.json（schema/digest-input.schema.json。docs/contract.md §26）。
type Input struct {
	Version   int            `json:"version"`
	Project   string         `json:"project"`
	Issue     string         `json:"issue"`
	Title     string         `json:"title"`
	At        string         `json:"at"`
	Since     string         `json:"since"`
	Until     string         `json:"until"`
	Previous  *Previous      `json:"previous" andon:"nullable"`
	Truncated bool           `json:"truncated"`
	Issues    []Issue        `json:"issues"`
	Waiting   waiting.Result `json:"waiting"`
	Concerns  []Concern      `json:"concerns"`
	Numbers   Numbers        `json:"numbers"`
	Next      Next           `json:"next"`
	Decided   *Decided       `json:"decided,omitempty"` // 節 5。前のバージョンの入力には無い（build は入っていないと描く）
	Redacted  int            `json:"redacted"`
	Skipped   []Skip         `json:"skipped"`
}

type Previous struct {
	Issue  string `json:"issue"`
	Until  string `json:"until"`
	SentAt string `json:"sent_at"`
}

type Issue struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	DoneAt   *string `json:"done_at" andon:"nullable"`
	PR       *string `json:"pr" andon:"nullable"`
	MergedAt *string `json:"merged_at" andon:"nullable"`
	MergedBy *string `json:"merged_by" andon:"nullable"`
	Tier     *string `json:"tier" andon:"nullable"`
	Grant    *string `json:"grant" andon:"nullable"`
	Retro    bool    `json:"retro"`
}

type Concern struct {
	Kind  string  `json:"kind"`
	Issue *string `json:"issue" andon:"nullable"`
	Title string  `json:"title"`
	At    string  `json:"at"`
	URL   *string `json:"url" andon:"nullable"`
}

// failed は失敗して止まっているもの（赤で描く。決めたこと 10）。
func (c Concern) failed() bool {
	return c.Kind == "stalled" || c.Kind == "ci_failed" || c.Kind == "revert"
}

type Numbers struct {
	Days     []Day `json:"days"`
	Current  Week  `json:"current"`
	Previous Week  `json:"previous"`
}

// Day は UTC の 1 日。
type Day struct {
	Date   string `json:"date"`
	Merged int    `json:"merged"`
	Auto   int    `json:"auto"`
}

// Week は 7 日の数（andon measure と同じ数え方）。
type Week struct {
	Since        string   `json:"since"`
	Until        string   `json:"until"`
	Merged       int      `json:"merged"`
	Auto         int      `json:"auto"`
	Closed       int      `json:"closed"`
	LeadTime     *float64 `json:"lead_time_hours" andon:"nullable"`
	QuestionOpen *float64 `json:"question_open_hours" andon:"nullable"`
	Tokens       *int64   `json:"tokens" andon:"nullable"`
}

type Next struct {
	Ready        int           `json:"ready"`
	Observations []Observation `json:"observations"`
	Routines     []Routine     `json:"routines"`
}

type Observation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	NotBefore string `json:"not_before"`
}

type Routine struct {
	Name  string  `json:"name"`
	Title string  `json:"title"`
	Next  *string `json:"next" andon:"nullable"`
}

type Skip struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

const (
	defaultSpan = 24 * time.Hour     // 前に送ったものが無いときの期間（決めたこと 5）
	maxSpan     = 7 * 24 * time.Hour // 遡る上限
	retroName   = "retro"            // 振り返りのルーチンの名前（issue.yaml の routine）
	estimateX   = 2                  // 見込みとのズレに数える倍（dispatcher の知らせの 2 段目と同じ）
	soonDays    = 7                  // 見る日が近い観察の課題
)

type task struct {
	id, title, status, profile, who, parent, routine, notBefore, estimate, repo string
	after                                                                       []string
}

func (t *task) reviewer() bool {
	_, _, round := ledger.SplitRound(t.id)
	return round || strings.HasSuffix(t.profile, "-review")
}

type collector struct {
	env     Env
	l       *ledger.Ledger
	last    string // done の列
	byID    map[string]*task
	ids     []string
	skipped []Skip
}

func (c *collector) skip(src, format string, a ...any) {
	c.skipped = append(c.skipped, Skip{Source: src, Reason: fmt.Sprintf(format, a...)})
}

func (c *collector) rel(p string) string {
	if s, err := filepath.Rel(c.env.Root, p); err == nil && !strings.HasPrefix(s, "..") {
		return s
	}
	return filepath.Base(p)
}

// Collect は課題 id のダイジェストの入力を集める。kanban・課題・履歴が読めなければ err。ほかの読めないものは skipped に並べて続ける。
func Collect(env Env, id string) (*Input, error) {
	l := ledger.At(env.Root)
	cfg, err := ledger.ReadConfig(l.Dir)
	if err != nil {
		return nil, fmt.Errorf("kanban が読めない（%s）: %v", filepath.Join(l.Dir, "config.yaml"), err)
	}
	c := &collector{env: env, l: l, last: "done", byID: map[string]*task{}}
	if len(cfg.Columns) > 0 {
		c.last = cfg.Columns[len(cfg.Columns)-1]
	}
	if err := c.readTasks(); err != nil {
		return nil, err
	}
	self := c.byID[id]
	if self == nil {
		return nil, fmt.Errorf("課題 %s が kanban に無い", id)
	}
	rows, err := l.ReadLog()
	if err != nil {
		return nil, fmt.Errorf("kanban の履歴が読めない（%s）: %v", filepath.Join(l.Dir, "log.jsonl"), err)
	}
	now := env.Now.UTC().Truncate(time.Second)
	in := &Input{Version: 1, Project: filepath.Base(env.Root), Issue: id, Title: self.title, At: now.Format(timeLayout),
		Until: now.Format(timeLayout), Issues: []Issue{}, Concerns: []Concern{}}
	if in.Title == "" {
		in.Title = id
	}

	// 期間（決めたこと 5）
	since := now.Add(-defaultSpan)
	if p := c.previous(self); p != nil {
		in.Previous = p
		since, _ = time.Parse(timeLayout, p.Until)
	}
	if since.Before(now.Add(-maxSpan)) {
		since, in.Truncated = now.Add(-maxSpan), true
	}
	if since.After(now) {
		since = now
	}
	in.Since = since.Format(timeLayout)
	inPeriod := func(t time.Time) bool { return !t.Before(since) && !t.After(now) }

	in.Issues = c.issues(self, rows, inPeriod)
	res, warn, err := waiting.Read(env.Root, now)
	if err != nil {
		return nil, err
	}
	for _, w := range warn {
		c.skip("andon waiting", "%s", w)
	}
	in.Waiting = res
	gh := c.cachedGH()
	in.Concerns = c.concerns(in.Issues, rows, inPeriod, gh)
	in.Numbers = c.numbers(now, gh)
	in.Next = c.next(now, cfg.Timezone)
	in.Decided = c.decided(rows, since, now, in.Previous)
	in.Skipped = append([]Skip{}, c.skipped...)

	// 秘密の形を伏せる（決めたこと 17。振り返りと同じ伏せ方）
	b, n := redact.JSON(contract.Marshal(in))
	var out Input
	if err := contract.Decode(b, &out); err != nil {
		return nil, fmt.Errorf("秘密の形を伏せたあとの入力が読めない: %v", err)
	}
	out.Redacted = n
	return &out, nil
}

func (c *collector) readTasks() error {
	dir := filepath.Join(c.l.Dir, "issues")
	ents, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s が読めない: %v", dir, err)
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		id := ledger.CanonID(e.Name())
		b, err := os.ReadFile(filepath.Join(dir, e.Name(), "issue.yaml"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var raw ledger.Issue
		if err != nil || yaml.Unmarshal(b, &raw) != nil || raw == nil {
			c.skip(c.rel(filepath.Join(dir, e.Name(), "issue.yaml")), "読めない（数えない）")
			continue
		}
		t := &task{id: id, title: raw.Str("title"), status: raw.Str("status"), profile: raw.Str("profile"), who: raw.Str("who"),
			parent: ledger.CanonID(raw.Str("parent")), routine: raw.Str("routine"), notBefore: raw.Str("not_before"), estimate: raw.Str("estimate"), repo: raw.Str("repo")}
		if list, ok := raw["after"].([]any); ok {
			for _, a := range list {
				if s, ok := a.(string); ok && s != "" {
					t.after = append(t.after, ledger.CanonID(s))
				}
			}
		}
		c.byID[id] = t
		c.ids = append(c.ids, id)
	}
	sort.Slice(c.ids, func(i, j int) bool { return ledger.IDLess(c.ids[i], c.ids[j]) })
	return nil
}

func (c *collector) lib(id string) string { return c.l.Library(id) }

// sameRoutine は t が self と同じルーチンのダイジェストか（自分を除く）。
func sameRoutine(self, t *task) bool {
	return t.id != self.id && self.routine != "" && t.routine == self.routine
}

// previous は同じルーチンが足したダイジェストのうち、sent.json の送り先が全部済んでいるもので、期間の終わりがいちばん新しいもの。
func (c *collector) previous(self *task) *Previous {
	var best *Previous
	for _, id := range c.ids {
		t := c.byID[id]
		if !sameRoutine(self, t) {
			continue
		}
		path := filepath.Join(c.lib(id), dirName, sentFile)
		s, err := readSent(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			c.skip(c.rel(path), "読めない（前に送ったものに数えない）: %v", err)
			continue
		}
		at, done := s.done()
		if !done {
			continue
		}
		if best == nil || s.Until > best.Until {
			best = &Previous{Issue: id, Until: s.Until, SentAt: at}
		}
	}
	return best
}

type outcome struct {
	Result   string  `json:"result"`
	PR       *string `json:"pr"`
	MergedAt *string `json:"merged_at"`
	Gates    []struct {
		Head     string `json:"head"`
		At       string `json:"at"`
		Tier     string `json:"tier"`
		Decision string `json:"decision"`
		Merged   bool   `json:"merged"`
	} `json:"gates"`
	WorkedMinutes int `json:"worked_minutes"`
}

func (c *collector) readJSON(path string, v any) bool {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err == nil {
		err = json.Unmarshal(b, v)
	}
	if err != nil {
		c.skip(c.rel(path), "読めない: %v", err)
		return false
	}
	return true
}

// doneAt は期間にほかの列から done へ動いた課題と、その時刻。片付いた課題への kanban done の打ち直しは動いたうちに入れない。
func (c *collector) doneAt(rows []ledger.Row, in func(time.Time) bool) map[string]time.Time {
	out := map[string]time.Time{}
	moved := ledger.MovedTo(rows, c.last)
	for i, r := range rows {
		if moved[i] {
			if at, err := time.Parse(timeLayout, r.At); err == nil && in(at) {
				out[r.ID] = at
			}
		}
	}
	return out
}

func (c *collector) title(id string) string {
	if t := c.byID[id]; t != nil {
		return t.title
	}
	return id
}

// issues は期間に done になったか、PR がマージされた課題（レビュアーの子課題と、同じルーチンのダイジェストを除く）。
func (c *collector) issues(self *task, rows []ledger.Row, in func(time.Time) bool) []Issue {
	doneAt := c.doneAt(rows, in)
	out := []Issue{}
	for _, id := range c.ids {
		t := c.byID[id]
		if t.reviewer() || t.id == self.id || sameRoutine(self, t) {
			continue
		}
		x := Issue{ID: id, Title: t.title, Status: t.status}
		if at, ok := doneAt[id]; ok {
			s := at.Format(timeLayout)
			x.DoneAt = &s
		}
		var rep struct {
			PR string `json:"pr"`
		}
		if c.readJSON(filepath.Join(c.lib(id), "report.json"), &rep) && rep.PR != "" {
			x.PR = &rep.PR
		}
		var o outcome
		opath := filepath.Join(c.lib(id), "outcome.json")
		if c.readJSON(opath, &o) {
			if x.PR == nil && o.PR != nil {
				x.PR = o.PR
			}
			// 結果の記録は done のあとに書かれるので、マージが前の期間でも done が今の期間なら、前のダイジェストには載っていない
			if o.Result == "merged" && o.MergedAt != nil {
				if at, err := time.Parse(timeLayout, *o.MergedAt); err == nil && (in(at) || x.DoneAt != nil) {
					x.MergedAt = o.MergedAt
					by := "person"
					for _, g := range o.Gates {
						if g.Merged {
							by = "dispatcher"
							tier := g.Tier
							x.Tier = &tier
							x.Grant = c.grant(id, g.Head)
							break
						}
					}
					x.MergedBy = &by
				}
			}
			if x.Tier == nil && len(o.Gates) > 0 {
				tier := o.Gates[len(o.Gates)-1].Tier
				x.Tier = &tier
			}
		}
		if x.DoneAt == nil && x.MergedAt == nil {
			continue
		}
		if x.DoneAt != nil && x.PR != nil && !fileExists(opath) {
			c.skip(c.rel(opath), "PR のある課題が片付いたのに結果の記録がまだ無い（マージしたかを数えていない。次の 1 巡で dispatcher が書く）")
		}
		if p := c.byID[t.parent]; p != nil && p.routine == retroName {
			x.Retro = true
		}
		out = append(out, x)
	}
	return out
}

// grant はマージした head の gate の記録の delegation.grant（名指しの grant がレベルを通したときだけある）。
func (c *collector) grant(id, head string) *string {
	if !contract.IsSHA(head) {
		return nil
	}
	var rec struct {
		Delegation struct {
			Grant string `json:"grant"`
		} `json:"delegation"`
	}
	if !c.readJSON(filepath.Join(c.lib(id), "gate", head+".json"), &rec) || rec.Delegation.Grant == "" {
		return nil
	}
	g := rec.Delegation.Grant
	return &g
}

// ghPR は gh pr list の1行（measure.GH の形）。
type ghPR struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	MergedAt string `json:"mergedAt"`
}

// cachedGH は repo ごとに1回だけ GitHub を読む（andon measure を2回回し、revert も探すため）。
func (c *collector) cachedGH() func(string) ([]byte, error) {
	type got struct {
		b   []byte
		err error
	}
	cache := map[string]got{}
	return func(repo string) ([]byte, error) {
		if c.env.GH == nil {
			return nil, errors.New("GitHub を読まない")
		}
		if g, ok := cache[repo]; ok {
			return g.b, g.err
		}
		b, err := c.env.GH(repo)
		cache[repo] = got{b, err}
		return b, err
	}
}

func ptr(s string) *string { return &s }

// concerns は期間の気になること（決めたこと 6 の節 6）。時刻の順。
func (c *collector) concerns(issues []Issue, rows []ledger.Row, in func(time.Time) bool, gh func(string) ([]byte, error)) []Concern {
	latest := map[string]Concern{} // kind|issue → いちばん新しいもの
	add := func(x Concern) {
		k := x.Kind + "|"
		if x.Issue != nil {
			k += *x.Issue
		} else {
			k += x.Title
		}
		if old, ok := latest[k]; !ok || x.At > old.At {
			latest[k] = x
		}
	}
	// 止まった担当: 届けた通知の記録のうち error・stalled
	if c.env.Delivered != "" {
		c.jsonl(c.env.Delivered, func(b []byte) {
			var line struct {
				At   string `json:"at"`
				News []struct {
					Issue string `json:"issue"`
					Kind  string `json:"kind"`
				} `json:"news"`
			}
			if json.Unmarshal(b, &line) != nil {
				return
			}
			at, err := time.Parse(timeLayout, line.At)
			if err != nil || !in(at) {
				return
			}
			for _, n := range line.News {
				if (n.Kind == "error" || n.Kind == "stalled") && n.Issue != "" {
					id := ledger.CanonID(n.Issue)
					add(Concern{Kind: "stalled", Issue: ptr(id), Title: c.title(id), At: line.At})
				}
			}
		})
	}
	for _, id := range c.ids {
		t := c.byID[id]
		if t.reviewer() {
			continue
		}
		gdir := filepath.Join(c.lib(id), "gate")
		ents, err := os.ReadDir(gdir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			c.skip(c.rel(gdir), "読めない: %v", err)
		}
		for _, e := range ents {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			var g struct {
				At       string `json:"at"`
				Decision string `json:"decision"`
			}
			if !c.readJSON(filepath.Join(gdir, e.Name()), &g) || g.Decision != "fix" {
				continue
			}
			if at, err := time.Parse(timeLayout, g.At); err == nil && in(at) {
				add(Concern{Kind: "gate_fix", Issue: ptr(id), Title: t.title, At: g.At})
			}
		}
		var rep struct {
			ReviewRequest *struct {
				Extra bool `json:"extra"`
			} `json:"review_request"`
		}
		if t.status != c.last && c.readJSONQuiet(filepath.Join(c.lib(id), "report.json"), &rep) && rep.ReviewRequest != nil && rep.ReviewRequest.Extra {
			if at := lastRow(rows, id, in); at != "" {
				add(Concern{Kind: "review_over", Issue: ptr(id), Title: t.title, At: at})
			}
		}
	}
	// main の CI の失敗（外の信号の記録の github-ci で、今も見えているもの）
	if c.env.Signals != "" {
		var rec struct {
			Signals []struct {
				Source   string `json:"source"`
				URL      string `json:"url"`
				Title    string `json:"title"`
				LastSeen string `json:"last_seen"`
				Active   bool   `json:"active"`
			} `json:"signals"`
		}
		if c.readJSON(c.env.Signals, &rec) {
			for _, s := range rec.Signals {
				if s.Source == "github-ci" && s.Active {
					add(Concern{Kind: "ci_failed", Title: s.Title, At: s.LastSeen, URL: ptr(s.URL)})
				}
			}
		}
	}
	prIssue := map[string]string{}
	repos := map[string]bool{}
	for _, id := range c.ids {
		var rep struct {
			PR string `json:"pr"`
		}
		if c.readJSONQuiet(filepath.Join(c.lib(id), "report.json"), &rep) {
			if owner, repo, _, ok := contract.ParsePR(rep.PR); ok {
				repos[owner+"/"+repo] = true
				prIssue[rep.PR] = id
			}
		}
	}
	for _, repo := range sortedKeys(repos) {
		b, err := gh(repo)
		if err != nil {
			c.skip("github.com/"+repo, "マージされた PR の一覧を読めない（%v）。revert を探していない", err)
			continue
		}
		var list []ghPR
		if json.Unmarshal(b, &list) != nil {
			c.skip("github.com/"+repo, "gh の出力が PR の配列でない。revert を探していない")
			continue
		}
		for _, p := range list {
			at, err := time.Parse(timeLayout, p.MergedAt)
			if err != nil || !in(at) || !strings.HasPrefix(p.Title, "Revert ") {
				continue
			}
			x := Concern{Kind: "revert", Title: p.Title, At: p.MergedAt, URL: ptr(p.URL)}
			if id, ok := prIssue[p.URL]; ok {
				x.Issue = ptr(id)
			}
			add(x)
		}
	}
	// 見込みとのズレ（期間に done になった課題で、実働が見込みの 2 倍を超えた）
	for _, x := range issues {
		t := c.byID[x.ID]
		if x.DoneAt == nil || t.estimate == "" {
			continue
		}
		est := ledger.EstimateMinutes(t.estimate)
		var o outcome
		if est > 0 && c.readJSONQuiet(filepath.Join(c.lib(x.ID), "outcome.json"), &o) && o.WorkedMinutes > est*estimateX {
			add(Concern{Kind: "estimate", Issue: ptr(x.ID), Title: t.title, At: *x.DoneAt})
		}
	}
	out := make([]Concern, 0, len(latest))
	for _, x := range latest {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Kind+out[i].Title < out[j].Kind+out[j].Title
	})
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readJSONQuiet は readJSON と同じで、読めなくても skipped に足さない（同じファイルを先に readJSON で読んだとき）。
func (c *collector) readJSONQuiet(path string, v any) bool {
	b, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(b, v) == nil
}

// lastRow は課題 id の、期間の中のいちばん新しい履歴の時刻（無ければ空）。
func lastRow(rows []ledger.Row, id string, in func(time.Time) bool) string {
	last := ""
	for _, r := range rows {
		if r.ID != id {
			continue
		}
		if at, err := time.Parse(timeLayout, r.At); err == nil && in(at) && r.At > last {
			last = r.At
		}
	}
	return last
}

func (c *collector) jsonl(path string, f func([]byte)) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		c.skip(filepath.Base(path), "読めない: %v", err)
		return
	}
	defer file.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	s := bufio.NewScanner(file)
	s.Buffer(make([]byte, 64<<10), 16<<20)
	for s.Scan() {
		f(s.Bytes())
	}
	if err := s.Err(); err != nil {
		c.skip(filepath.Base(path), "読み切れない: %v", err)
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// numbers は直近 7 日とその前の 7 日を andon measure で数える（数え方を写さない）。
func (c *collector) numbers(now time.Time, gh func(string) ([]byte, error)) Numbers {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	week := func(since, until time.Time) (Week, *measure.Result) {
		w := Week{Since: since.Format("2006-01-02"), Until: until.Format("2006-01-02")}
		res, err := measure.Measure(measure.Options{Root: c.env.Root, Since: since, Until: until, Now: now, GH: gh,
			State: c.env.State, Delivered: c.env.Delivered, ClaudeDir: c.env.ClaudeDir, CodexSessions: c.env.CodexSessions})
		if err != nil {
			c.skip("andon measure", "%s〜%s を数えられない: %v", w.Since, w.Until, err)
			return w, nil
		}
		if n := len(res.Skipped); n > 0 {
			c.skip("andon measure", "%s〜%s で %d 件の記録を読めなかった（andon measure --since %s --until %s で見る）", w.Since, w.Until, n, w.Since, w.Until)
		}
		w.Merged, w.Auto, w.Closed = res.Efficiency.MergedPRs, res.Efficiency.AutoMerge.Count, res.Flow.Closed.Count
		w.LeadTime, w.QuestionOpen = res.Flow.LeadTime.Value, res.Flow.QuestionOpen.Value
		total, any := int64(0), false
		for _, t := range res.Tokens {
			if t.Usage != nil {
				any = true
				total += t.Usage.Input + t.Usage.Output + t.Usage.CacheRead + t.Usage.CacheWrite
			}
		}
		if any {
			w.Tokens = &total
		}
		return w, res
	}
	n := Numbers{Days: []Day{}}
	var cur *measure.Result
	n.Current, cur = week(today.AddDate(0, 0, -6), today)
	n.Previous, _ = week(today.AddDate(0, 0, -13), today.AddDate(0, 0, -7))
	for i := 6; i >= 0; i-- {
		n.Days = append(n.Days, Day{Date: today.AddDate(0, 0, -i).Format("2006-01-02")})
	}
	if cur != nil {
		for _, p := range cur.PRs {
			at, err := time.Parse(timeLayout, p.MergedAt)
			if err != nil {
				continue
			}
			d := at.UTC().Format("2006-01-02")
			for i := range n.Days {
				if n.Days[i].Date == d {
					n.Days[i].Merged++
					if p.Auto {
						n.Days[i].Auto++
					}
				}
			}
		}
	}
	return n
}

// next はこのあと（決めたこと 6 の節 8）。
func (c *collector) next(now time.Time, tz string) Next {
	loc := time.UTC
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	today := now.In(loc).Format("2006-01-02")
	soon := now.In(loc).AddDate(0, 0, soonDays).Format("2006-01-02")
	n := Next{Observations: []Observation{}, Routines: []Routine{}}
	for _, id := range c.ids {
		t := c.byID[id]
		if t.status == c.last {
			continue
		}
		if p := c.byID[t.parent]; p != nil && p.profile == "chief-engineer" && !t.reviewer() && t.notBefore != "" && t.notBefore <= soon {
			n.Observations = append(n.Observations, Observation{ID: id, Title: t.title, NotBefore: t.notBefore})
		}
		if t.status != "todo" || t.reviewer() || t.who == "person" || (t.notBefore != "" && t.notBefore > today) {
			continue
		}
		ready := true
		for _, a := range t.after {
			if d := c.byID[a]; d == nil || d.status != c.last {
				ready = false
			}
		}
		if ready {
			n.Ready++
		}
	}
	dir := filepath.Join(c.l.Dir, "routines")
	files, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	sort.Strings(files)
	for _, f := range files {
		var r struct {
			Title    string `yaml:"title"`
			Schedule string `yaml:"schedule"`
			Paused   bool   `yaml:"paused"`
		}
		b, err := os.ReadFile(f)
		if err != nil || yaml.Unmarshal(b, &r) != nil {
			c.skip(c.rel(f), "読めない")
			continue
		}
		if r.Schedule == "" {
			continue
		}
		x := Routine{Name: strings.TrimSuffix(filepath.Base(f), ".yaml"), Title: r.Title}
		if cr, err := ledger.ParseCron(r.Schedule); err == nil && !r.Paused {
			if t, ok := cr.Next(now, loc); ok {
				x.Next = ptr(t.UTC().Format(timeLayout))
			}
		}
		n.Routines = append(n.Routines, x)
	}
	return n
}
