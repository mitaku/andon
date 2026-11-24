package retro

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"andon/internal/contract"
	"andon/internal/ghpr"
)

// AfterMerge はマージ後のこと（ADR-0028 決めたこと 7・14 の 4）。記録には書かず、集めるときに GitHub から読む。
type AfterMerge struct {
	MergeCommit string   `json:"merge_commit"`
	MergedAt    string   `json:"merged_at"`
	Checks      *Checks  `json:"checks"`
	RevertedBy  []Revert `json:"reverted_by"`
}

type Checks struct {
	State  string   `json:"state"`
	Failed []string `json:"failed"`
}

type Revert struct {
	PR       string `json:"pr"`
	MergedAt string `json:"merged_at"`
}

// 失敗に数える check run の conclusion（cancelled は数えない。同じ workflow の後の run に止められたものが多い）。
var failedConclusions = map[string]bool{"failure": true, "timed_out": true, "startup_failure": true, "action_required": true}

type ghPR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	MergedAt    string `json:"mergedAt"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
}

type github struct {
	r      *reader
	gh     func(args ...string) ([]byte, error)
	lists  map[string][]ghPR
	warned bool
}

func newGitHub(r *reader, gh func(args ...string) ([]byte, error)) *github {
	return &github{r: r, gh: gh, lists: map[string][]ghPR{}}
}

func (g *github) list(repo string) ([]ghPR, bool) {
	if g.gh == nil {
		if !g.warned {
			g.warned = true
			g.r.skip("github", "GitHub を読まない（マージ後のことは null）")
		}
		return nil, false
	}
	if l, ok := g.lists[repo]; ok {
		return l, l != nil
	}
	b, err := ghpr.Merged(g.gh, repo, "number,url,title,body,mergedAt,mergeCommit")
	var l []ghPR
	if err == nil {
		err = json.Unmarshal(b, &l)
	}
	if err != nil {
		g.lists[repo] = nil
		g.r.skip("github.com/"+repo, "マージされた PR の一覧が読めない（%v）。この repo のマージ後のことは null", err)
		return nil, false
	}
	if l == nil {
		l = []ghPR{}
	}
	g.lists[repo] = l
	return l, true
}

func (g *github) merged(pr string) (ghPR, bool) {
	owner, name, _, ok := contract.ParsePR(pr)
	if !ok {
		return ghPR{}, false
	}
	l, ok := g.list(owner + "/" + name)
	if !ok {
		return ghPR{}, false
	}
	for _, p := range l {
		if p.URL == pr {
			return p, true
		}
	}
	return ghPR{}, false
}

func (g *github) afterMerge(b Bundle) *AfterMerge {
	if b.PR == nil {
		return nil
	}
	owner, name, number, ok := contract.ParsePR(*b.PR)
	if !ok {
		return nil
	}
	p, ok := g.merged(*b.PR)
	if !ok {
		return nil
	}
	a := &AfterMerge{MergedAt: p.MergedAt, RevertedBy: []Revert{}}
	if p.MergeCommit != nil {
		a.MergeCommit = p.MergeCommit.Oid
	}
	if a.MergeCommit == "" {
		a.MergeCommit = b.mergeCommit
	}
	repo := owner + "/" + name
	if a.MergeCommit != "" {
		a.Checks = g.checks(repo, a.MergeCommit)
	}
	ref := repo + "#" + number
	l, _ := g.list(repo)
	for _, rv := range l {
		if rv.URL == *b.PR || !strings.HasPrefix(rv.Title, "Revert ") {
			continue
		}
		if containsRef(rv.Body, ref) || containsRef(rv.Body, *b.PR) {
			a.RevertedBy = append(a.RevertedBy, Revert{PR: rv.URL, MergedAt: rv.MergedAt})
		}
	}
	sort.Slice(a.RevertedBy, func(i, j int) bool { return a.RevertedBy[i].MergedAt < a.RevertedBy[j].MergedAt })
	return a
}

// --paginate でページを最後まで読む（gh はページごとの JSON の object を続けて出す）。
func (g *github) checks(repo, sha string) *Checks {
	b, err := g.gh("api", "--paginate", "repos/"+repo+"/commits/"+sha+"/check-runs?per_page=100")
	type run struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	var v struct{ CheckRuns []run }
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(b))
		for err == nil {
			var page struct {
				CheckRuns []run `json:"check_runs"`
			}
			if err = dec.Decode(&page); err == nil {
				v.CheckRuns = append(v.CheckRuns, page.CheckRuns...)
			}
		}
		if err == io.EOF {
			err = nil
		}
	}
	if err != nil {
		g.r.skip("github.com/"+repo+"@"+sha, "マージ commit の checks が読めない（%v）", err)
		return nil
	}
	c := &Checks{State: "none", Failed: []string{}}
	seen := map[string]bool{}
	pending := false
	for _, cr := range v.CheckRuns {
		switch {
		case cr.Status != "completed":
			pending = true
		case failedConclusions[cr.Conclusion] && !seen[cr.Name]:
			seen[cr.Name] = true
			c.Failed = append(c.Failed, cr.Name)
		}
	}
	sort.Strings(c.Failed)
	switch {
	case len(c.Failed) > 0:
		c.State = "failure"
	case pending:
		c.State = "pending"
	case len(v.CheckRuns) > 0:
		c.State = "success"
	}
	return c
}

func containsRef(body, ref string) bool {
	for i := 0; ; {
		j := strings.Index(body[i:], ref)
		if j < 0 {
			return false
		}
		k := i + j + len(ref)
		if k == len(body) || body[k] < '0' || body[k] > '9' {
			return true
		}
		i = k
	}
}
