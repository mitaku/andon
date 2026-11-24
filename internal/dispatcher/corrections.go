package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger"
)

// オーナーの手直し（contract.md §14。design.md §1.9 の (b) の表の 1〜3）: done になった課題ごとに1回だけ
// library/corrections.json を書く。書き手は dispatcher だけ。手直しが無ければ空の配列で書き（「無かった」）、
// gh が失敗したら書かない（「集めていない」）。打ち直すのは次の見回りの1回だけで、それも駄目なら通知 corrections を積む。

const correctionsTries = 2

// machineGateAnswers は、オーナーでなく機械が gate の問いを閉じたときの答えの形（全体一致）。オーナーの手直しに数えない。
//   - andon issue settle がオーナーのマージを見て閉じた答え（internal/issue/settle.go closeAndFinish）
//   - andon issue merge が、gate の判定のあとに PR の head が動いたのを見て、gate を回し直す前に問いを閉じた答え
//     （internal/issue/merge.go の headMoved から closeGateQuestions）
//   - andon issue merge が、今の head で gate を回し直したときに前の head の問いを閉じた答え
//     （internal/issue/merge.go closeStaleGateQuestions）
//   - andon issue merge が、gate の判定のあとに Chief of Staff が PR を止めたのを見て、判定を捨てて閉じた答え
//     （internal/issue/merge.go holdDecision から closeGateQuestions。ADR-0056 決めたこと 9）
//
// 答えた者（answeredBy）では見分けない: dispatcher が止まっているときは Chief of Staff が settle を打ち、答えた者が Chief of Staff になる。
// 書き方が変わるか、gate の問いを閉じる書き手が増えたら TestMachineGateAnswersMatchWriters が落ちる。
var machineGateAnswers = []*regexp.Regexp{
	regexp.MustCompile(`^オーナーがマージした（[^（）]*）$`),
	regexp.MustCompile(`^PR の head が gate のあとに動いた（[0-9a-f]+ → [0-9a-f]+）。gate が今の head で回し直す$`),
	regexp.MustCompile(`^PR の head が動いた（[0-9a-f]+ → [0-9a-f]+）。gate が今の head で決め直した（記録 [^（）]*）$`),
	regexp.MustCompile(`^Chief of Staff が PR を止めた（理由の課題 [^（）]*）。外したら gate が今の head で回し直す$`),
}

func machineGateAnswer(a string) bool {
	for _, re := range machineGateAnswers {
		if re.MatchString(a) {
			return true
		}
	}
	return false
}

type QA struct {
	N      int    `json:"n"`
	Status string `json:"status"`
	Answer string `json:"answer"`
	Q      *contract.Question
}

type PRView struct {
	State   string `json:"state"`
	Commits []struct {
		Oid             string `json:"oid"`
		MessageHeadline string `json:"messageHeadline"`
	} `json:"commits"`
	Comments []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
	} `json:"comments"`
	Reviews []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body        string `json:"body"`
		SubmittedAt string `json:"submittedAt"`
	} `json:"reviews"`
	Inline []InlineComment `json:"-"`
}

type InlineComment struct {
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	Body         string `json:"body"`
	CreatedAt    string `json:"created_at"`
	Path         string `json:"path"`
	Line         *int   `json:"line"`
	OriginalLine *int   `json:"original_line"`
}

type HumanCommit struct {
	Sha      string   `json:"sha"`
	Headline string   `json:"headline"`
	Files    []string `json:"files"`
}

func HumanCommits(reported string, pr *PRView) []HumanCommit {
	if pr == nil || reported == "" {
		return nil
	}
	at := -1
	for i, c := range pr.Commits {
		if c.Oid == reported {
			at = i
		}
	}
	if at < 0 {
		return nil
	}
	var out []HumanCommit
	for _, c := range pr.Commits[at+1:] {
		out = append(out, HumanCommit{Sha: c.Oid, Headline: c.MessageHeadline, Files: []string{}})
	}
	return out
}

type correctionAnswer struct {
	Kind         string `json:"kind"`
	Question     int    `json:"question"`
	DecisionKind string `json:"decision_kind"`
	Recommended  string `json:"recommended"`
	Picked       string `json:"picked"`
}

type prComment struct {
	Author string `json:"author"`
	At     string `json:"at"`
	Body   string `json:"body"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type correctionPR struct {
	Kind         string        `json:"kind"`
	Comments     []prComment   `json:"comments"`
	HumanCommits []HumanCommit `json:"human_commits"`
}

type finding struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type correctionFinding struct {
	Kind    string  `json:"kind"`
	Finding finding `json:"finding"`
	Reason  string  `json:"reason"`
}

type correctionQuestion struct {
	Kind        string `json:"kind"`
	Question    int    `json:"question"`
	Recommended string `json:"recommended"`
	Answer      string `json:"answer"`
}

// Corrections は1つの課題のオーナーの手直しを並べる（純粋関数。contract.md §14 の規則）。
//   - answer: 答えのある問い（gate の問いと scope を除く）で、答えが推奨案（pick: true）の label と違うもの
//   - override（問い）: 答えのある gate の問い（from: gate。needs_human へのオーナーの答え）。機械が閉じた答え（machineGateAnswer）は除く
//   - pr: PR の上の、gate のものでないコメント（会話欄・レビューの本文・インラインのレビューコメント〈Bot を除く〉）と、担当の最後の報告より後の commit（humans。
//     merge commit と Chief Engineer の課題の commit は collectCorrections が外す）
//   - override（指摘）: マージした PR の waived のうち must_fix 以上のもの（must_fix が無ければ全部。安全側＝多めに拾う）
//
// 返すのは空でも nil でない配列。
func Corrections(qs []QA, rep *contract.Report, mustFix string, pr *PRView, humans []HumanCommit) []any {
	out := []any{}
	for _, qa := range qs {
		if qa.Q == nil || (qa.Status != "answered" && qa.Status != "received") || strings.TrimSpace(qa.Answer) == "" || len(qa.Q.Options) == 0 {
			continue
		}
		rec, ans := strings.TrimSpace(qa.Q.Options[0].Label), strings.TrimSpace(qa.Answer)
		switch {
		case qa.Q.From == "gate" && machineGateAnswer(ans):
		case qa.Q.From == "gate":
			out = append(out, correctionQuestion{Kind: "override", Question: qa.N, Recommended: rec, Answer: ans})
		case qa.Q.Kind == "scope":
		case ans != rec:
			out = append(out, correctionAnswer{Kind: "answer", Question: qa.N, DecisionKind: qa.Q.Kind, Recommended: rec, Picked: ans})
		}
	}
	if pr != nil {
		c := correctionPR{Kind: "pr", Comments: []prComment{}, HumanCommits: []HumanCommit{}}
		for _, m := range pr.Comments {
			if strings.TrimSpace(m.Body) == "" || strings.HasPrefix(m.Body, gate.CommentPrefix) {
				continue
			}
			c.Comments = append(c.Comments, prComment{Author: orUnknown(m.Author.Login), At: orUnknown(m.CreatedAt), Body: m.Body})
		}
		for _, m := range pr.Reviews {
			if strings.TrimSpace(m.Body) == "" || strings.HasPrefix(m.Body, gate.CommentPrefix) {
				continue
			}
			c.Comments = append(c.Comments, prComment{Author: orUnknown(m.Author.Login), At: orUnknown(m.SubmittedAt), Body: m.Body})
		}
		for _, m := range pr.Inline {
			if strings.TrimSpace(m.Body) == "" || strings.HasPrefix(m.Body, gate.CommentPrefix) || m.User.Type == "Bot" {
				continue
			}
			x := prComment{Author: orUnknown(m.User.Login), At: orUnknown(m.CreatedAt), Body: m.Body, File: m.Path}
			if m.Line != nil && *m.Line > 0 {
				x.Line = *m.Line
			} else if m.OriginalLine != nil && *m.OriginalLine > 0 {
				x.Line = *m.OriginalLine
			}
			c.Comments = append(c.Comments, x)
		}
		c.HumanCommits = append(c.HumanCommits, humans...)
		if len(c.Comments) > 0 || len(c.HumanCommits) > 0 {
			out = append(out, c)
		}
	}
	if rep != nil && pr != nil && pr.State == "MERGED" {
		limit := ledger.SeverityRank(mustFix)
		for _, w := range rep.Waived {
			r := ledger.SeverityRank(w.Severity)
			if w.Line == nil || r < 0 || (limit >= 0 && r > limit) {
				continue
			}
			out = append(out, correctionFinding{Kind: "override", Finding: finding{Severity: w.Severity, File: w.File, Line: *w.Line}, Reason: w.Reason})
		}
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

func (p *project) corrections() {
	p.st.Corrections.Each(p.correction)
}

func (p *project) correction(id string) bool {
	path := filepath.Join(p.l.Library(id), "corrections.json")
	if _, err := os.Stat(path); err == nil {
		return true
	}
	t, qs, err := p.show(id)
	if err == nil && isReviewer(t) {
		return true
	}
	var out []any
	if err == nil {
		out, err = p.collectCorrections(id, t, qs)
	}
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			err = contract.Write(path, contract.SchemaCorrections, map[string]any{"at": now(), "corrections": out})
		}
	}
	if err == nil {
		p.logf("%s のオーナーの手直しを %d 件 corrections.json に書いた", id, len(out))
		return true
	}
	if p.st.Corrections.Fail(id, correctionsTries) {
		p.logf("%s のオーナーの手直しを集められない（諦めて Chief of Staff に知らせる）: %v", id, err)
		p.notify(id, "corrections", map[string]any{"error": clean(fmt.Sprintf("%d 回打って失敗: %v", correctionsTries, err), 200)})
		return true
	}
	p.logf("%s のオーナーの手直しを集められない（次の見回りで1回だけ打ち直す）: %v", id, err)
	return false
}

func (p *project) collectCorrections(id string, t ledger.Issue, qs []QA) ([]any, error) {
	rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
	if errors.Is(err, os.ErrNotExist) {
		rep, err = nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("report.json が読めない: %v", err)
	}
	var pr *PRView
	var humans []HumanCommit
	if rep != nil && rep.PR != "" {
		owner, name, number, ok := contract.ParsePR(rep.PR)
		if !ok {
			return nil, fmt.Errorf("report.json の pr が読めない: %s", rep.PR)
		}
		out, _, err := run("", nil, "", "gh", "pr", "view", number, "--repo", owner+"/"+name, "--json", "state,commits,comments,reviews")
		if err != nil {
			return nil, err
		}
		pr = &PRView{}
		if err := json.Unmarshal([]byte(out), pr); err != nil {
			return nil, fmt.Errorf("gh pr view の出力が読めない: %v", err)
		}
		out, _, err = run("", nil, "", "gh", "api", "--paginate", "--slurp", "repos/"+owner+"/"+name+"/pulls/"+number+"/comments")
		if err != nil {
			return nil, err
		}
		var pages [][]InlineComment
		if err := json.Unmarshal([]byte(out), &pages); err != nil {
			return nil, fmt.Errorf("PR のレビューコメントが読めない: %v", err)
		}
		for _, pg := range pages {
			pr.Inline = append(pr.Inline, pg...)
		}
		if !isEngineer(t) { // Chief Engineer は報告のあとの push で報告を書き直さないので、報告より後でもオーナーの commit と言えない（ADR-0058 決めたこと 8）
			humans = HumanCommits(rep.Commit, pr)
		}
		kept := humans[:0]
		for _, h := range humans {
			out, _, err := run("", nil, "", "gh", "api", "repos/"+owner+"/"+name+"/commits/"+h.Sha)
			if err != nil {
				return nil, err
			}
			var c struct {
				Parents json.RawMessage `json:"parents"`
				Files   []struct {
					Filename string `json:"filename"`
				} `json:"files"`
			}
			if err := json.Unmarshal([]byte(out), &c); err != nil {
				return nil, fmt.Errorf("commit %s のファイルが読めない: %v", short12(h.Sha), err)
			}
			var parents []json.RawMessage
			if json.Unmarshal(c.Parents, &parents) == nil && len(parents) >= 2 {
				continue // ほかの branch を取り込んだ merge commit はエージェントの仕事を直した跡でない（ADR-0058 決めたこと 7）。親が読めなければ入れる
			}
			for _, f := range c.Files {
				h.Files = append(h.Files, f.Filename)
			}
			kept = append(kept, h)
		}
		humans = kept
	}
	mustFix := ""
	if p.l.Trusted(p.l.Log(), t, "must_fix") {
		mustFix = ledger.SettingsOf(t).MustFix
	}
	return Corrections(qs, rep, mustFix, pr, humans), nil
}

func (p *project) show(id string) (ledger.Issue, []QA, error) {
	out, err := p.l.Run("", "show", id, "--json")
	if err != nil {
		return nil, nil, err
	}
	var v struct {
		Issue     ledger.Issue `json:"issue"`
		Questions []QA         `json:"questions"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Issue == nil {
		return nil, nil, fmt.Errorf("課題 %s が読めない", id)
	}
	for i := range v.Questions {
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), "questions", strconv.Itoa(v.Questions[i].N)+".json"))
		if err != nil {
			continue
		}
		if q, err := contract.ParseQuestion(b); err == nil {
			v.Questions[i].Q = q
		}
	}
	return v.Issue, v.Questions, nil
}
