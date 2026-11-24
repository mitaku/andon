package retro

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/ledger"
)

// Question は期間に聞かれた問い1つ（ADR-0061 決めたこと 10）。振り返りの担当が、同じことを聞いた問いが別々に2件あるかを
// 読み、あれば自分の報告の gaps[] に missing として書く。パターンの根拠には指せない（根拠の単位は変えない。決めたこと 9）。
type Question struct {
	Issue    string    `json:"issue"`
	N        int       `json:"n"`
	From     *string   `json:"from"`
	Kind     string    `json:"kind"`
	Question string    `json:"question"`
	Options  []QOption `json:"options"`
	Answer   *string   `json:"answer"`
	AskedAt  string    `json:"asked_at"`
}

type QOption struct {
	Label string `json:"label"`
	Pick  bool   `json:"pick"`
}

// askedQuestion は問いの JSON（contract.md §5。library/questions/<n>.json）のうち、ここで読む所。
type askedQuestion struct {
	Kind     string `json:"kind"`
	Question string `json:"question"`
	Options  []struct {
		Label string `json:"label"`
		Pick  bool   `json:"pick"`
	} `json:"options"`
	From string `json:"from"`
}

// questionKinds と questionFroms は questions[] に入れる問いの kind と from（schema/retro.schema.json の question）。
// from の空は、問いの JSON に from が無い問い（null で出す）。
var (
	questionKinds = map[string]bool{"business": true, "users": true, "promise": true, "irreversible": true, "line": true}
	questionFroms = map[string]bool{"": true, "issue": true, "retro": true, "chief-of-staff": true, "chief-engineer": true}
)

func (r *reader) questions(tasks []*task, since, until time.Time) []Question {
	out := []Question{}
	for _, t := range tasks {
		path := filepath.Join(r.dir, "issues", t.id, "questions.yaml")
		b, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var list []struct {
			N      int    `yaml:"n"`
			Asked  string `yaml:"asked"`
			Answer string `yaml:"answer"`
		}
		if err != nil || yaml.Unmarshal(b, &list) != nil {
			r.skip(r.rel(path), "読めない（問いを入れていない）")
			continue
		}
		for _, q := range list {
			at, err := time.Parse(time.RFC3339, q.Asked)
			if err != nil || q.N < 1 {
				r.skip(r.rel(path), "問いの番号か聞いた時刻が読めない（入れていない）")
				continue
			}
			if !at.After(since) || at.After(until) {
				continue
			}
			jp := filepath.Join(r.dir, "issues", t.id, "library", "questions", strconv.Itoa(q.N)+".json")
			var a askedQuestion
			jb, err := os.ReadFile(jp)
			if err != nil || json.Unmarshal(jb, &a) != nil || a.Kind == "" || a.Question == "" {
				r.skip(r.rel(jp), "問いの JSON が無いか読めない（gate の問いかを決められないので入れていない）")
				continue
			}
			if a.From == "gate" || a.Kind == "scope" {
				continue
			}
			if !questionKinds[a.Kind] || !questionFroms[a.From] {
				// 前のバージョンの値（from: task など）か知らない値。載せると出力が schema に落ち、collect 全体が出ない
				r.skip(r.rel(jp), "問いの kind か from が知らない値（入れていない）")
				continue
			}
			x := Question{Issue: t.id, N: q.N, From: strPtr(a.From), Kind: a.Kind, Question: a.Question,
				Options: []QOption{}, Answer: strPtr(q.Answer), AskedAt: at.UTC().Format(time.RFC3339)}
			for _, o := range a.Options {
				x.Options = append(x.Options, QOption{Label: o.Label, Pick: o.Pick})
			}
			out = append(out, x)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AskedAt != out[j].AskedAt {
			return out[i].AskedAt < out[j].AskedAt
		}
		if out[i].Issue != out[j].Issue {
			return ledger.IDLess(out[i].Issue, out[j].Issue)
		}
		return out[i].N < out[j].N
	})
	return out
}
