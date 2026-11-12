// Package contract は機械向けの約束（docs/contract.md・schema/）のうち、command が読み書きする JSON の形と検証。
// 正は schema/ で、ここはその写し（知らないキーを拒む・必須・値の形）。tests/schema.sh が command の出力を schema に通して突き合わせる。
package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
)

var (
	shaRe  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	prRe   = regexp.MustCompile(`^https://github\.com/([!-.0-~]+)/([!-.0-~]+)/pull/([1-9][0-9]*)$`)
	ctrlRe = regexp.MustCompile("[\x00-\x1f\x7f]")
)

// IsSHA は40桁・小文字の commit か。
func IsSHA(s string) bool { return shaRe.MatchString(s) }

// ParsePR は PR の URL を owner・repo・番号に分ける。形が違えば ok が false。
func ParsePR(s string) (owner, repo, number string, ok bool) {
	m := prRe.FindStringSubmatch(s)
	if m == nil {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}

// RelPath は repo からの相対パスか（空でない・制御文字が無い・/ で始まらない・.. の段が無い）。
// schema の pattern は U+2028 などの行区切りを段の区切りとして扱わないので、ここも / だけで段を切る。
func RelPath(s string) bool {
	if s == "" || ctrlRe.MatchString(s) || strings.HasPrefix(s, "/") {
		return false
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// Decode は JSON を v に読む（約束の JSON の入口は全部これ）。schema と同じく厳しく読む:
// 知らないキー・型違い・後ろの余りは誤り。存在するキーの null も誤り（Go は null を nil・ゼロ値として黙って受けるので、
// 先に生の JSON を型に沿って歩いて拒む）。キーは struct の json の名前と完全に一致すること（大小文字違いは知らないキー）。null を許すのは struct の tag に `andon:"nullable"` を書いたキーだけ。
func Decode(b []byte, v any) error {
	var raw any
	rd := json.NewDecoder(bytes.NewReader(b))
	rd.UseNumber()
	if err := rd.Decode(&raw); err != nil {
		return err
	}
	if err := noNull(raw, reflect.TypeOf(v), "$"); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("JSON の後ろに余りがある")
	}
	return nil
}

var rawMessage = reflect.TypeOf(json.RawMessage{})

// noNull は raw を型 t に沿って歩き、許されていない null を探す。
func noNull(raw any, t reflect.Type, at string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if raw == nil {
		return fmt.Errorf("%s が null", at)
	}
	if t == rawMessage || t.Kind() == reflect.Interface {
		return nil // 中身は呼び手が確かめる
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := raw.(map[string]any)
		if !ok {
			return nil // 型違いは json の Decode が拒む
		}
		// encoding/json はキーを大文字小文字を区別せずに当てるので、名前が完全に一致しないキーは知らないキーとして先に拒む
		known := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
				known[name] = true
			}
		}
		for k := range m {
			if !known[k] {
				return fmt.Errorf("%s に知らないキー %q", at, k)
			}
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			x, present := m[name]
			if !present {
				continue
			}
			if x == nil && f.Tag.Get("andon") == "nullable" {
				continue
			}
			if err := noNull(x, f.Type, at+"."+name); err != nil {
				return err
			}
		}
	case reflect.Map:
		if m, ok := raw.(map[string]any); ok {
			for k, x := range m {
				if err := noNull(x, t.Elem(), at+"."+k); err != nil {
					return err
				}
			}
		}
	case reflect.Slice:
		if a, ok := raw.([]any); ok {
			for i, x := range a {
				if err := noNull(x, t.Elem(), fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Write は v を2字下げの JSON で書く（HTML の文字を逃がさない）。一時ファイルに書いてから置き換える。
func Write(path string, v any) error {
	b := Marshal(v)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Marshal は v を2字下げの JSON にする（末尾に改行）。
func Marshal(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return b.Bytes()
}

// Finding は findings.json の1件（contract.md §3）。
type Finding struct {
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        *int   `json:"line"`
	Preexisting bool   `json:"preexisting,omitempty"` // 課題の前からあった問題（ゲートは must_fix に数えない。差分で足した行なら数える）
}

// Findings は findings.json（レビュアーが書く）。
type Findings struct {
	Commit   string    `json:"commit"`
	Findings []Finding `json:"findings"`
}

func severityOK(s string) bool {
	switch s {
	case "critical", "major", "minor", "trivial":
		return true
	}
	return false
}

func (f Finding) check() error {
	switch {
	case !severityOK(f.Severity):
		return fmt.Errorf("severity が critical/major/minor/trivial でない: %q", f.Severity)
	case !RelPath(f.File):
		return fmt.Errorf("file が repo からの相対パスでない: %q", f.File)
	case f.Line == nil || *f.Line < 1:
		return errors.New("line が 1 以上の整数でない")
	}
	return nil
}

// ReadFindings は findings.json を読んで検査する。
func ReadFindings(path string) (*Findings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f Findings
	if err := Decode(b, &f); err != nil {
		return nil, err
	}
	if !IsSHA(f.Commit) {
		return nil, errors.New("commit が40桁・小文字でない")
	}
	if f.Findings == nil {
		return nil, errors.New("findings が無い")
	}
	for _, x := range f.Findings {
		if err := x.check(); err != nil {
			return nil, err
		}
	}
	return &f, nil
}

// Candidate は candidates.json の1件（contract.md §12）。
type Candidate struct {
	ID      string  `json:"id"`
	Text    string  `json:"text"`
	File    *string `json:"file,omitempty"`
	Line    *int    `json:"line,omitempty"`
	InScope *bool   `json:"in_scope"`
}

// Candidates は candidates.json（担当が andon issue candidates で書く）。commit は repo の無い課題では書かない。
type Candidates struct {
	Commit     string      `json:"commit,omitempty"`
	Candidates []Candidate `json:"candidates"`
}

var candidateIDRe = regexp.MustCompile(`^c[1-9][0-9]*$`)

// Check は候補の一覧の形を確かめる。id の一意は schema では拡張キーワード（uniqueKeys）で、ほかの検査器は飛ばすのでここでも確かめる。
func (c *Candidates) Check() error {
	if c.Commit != "" && !IsSHA(c.Commit) {
		return errors.New("commit が40桁・小文字でない")
	}
	if c.Candidates == nil {
		return errors.New("candidates が無い")
	}
	seen := map[string]bool{}
	for i, x := range c.Candidates {
		at := fmt.Sprintf("candidates[%d]", i)
		switch {
		case !candidateIDRe.MatchString(x.ID):
			return fmt.Errorf("%s の id が c1, c2, … の形でない: %q", at, x.ID)
		case seen[x.ID]:
			return fmt.Errorf("%s の id %s が重なっている", at, x.ID)
		case x.Text == "" || strings.Contains(x.Text, "\n"):
			return fmt.Errorf("%s（%s）の text が空か2行以上", at, x.ID)
		case x.File != nil && !RelPath(*x.File):
			return fmt.Errorf("%s（%s）の file が repo からの相対パスでない: %q", at, x.ID, *x.File)
		case x.Line != nil && *x.Line < 1:
			return fmt.Errorf("%s（%s）の line が 1 以上の整数でない", at, x.ID)
		case x.InScope == nil:
			return fmt.Errorf("%s（%s）に in_scope が無い", at, x.ID)
		}
		seen[x.ID] = true
	}
	return nil
}

// ParseCandidates は候補の一覧の JSON を読んで検査する。
func ParseCandidates(b []byte) (*Candidates, error) {
	var c Candidates
	if err := Decode(b, &c); err != nil {
		return nil, err
	}
	if err := c.Check(); err != nil {
		return nil, err
	}
	return &c, nil
}

// ReadCandidates は candidates.json を読んで検査する。
func ReadCandidates(path string) (*Candidates, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCandidates(b)
}

// ReviewRequest は report.json の review_request（contract.md §4）。
type ReviewRequest struct {
	Round    int     `json:"round"`
	Limit    int     `json:"limit"`
	Commit   string  `json:"commit"`
	Previous *string `json:"previous" andon:"nullable"`
	Reviewer string  `json:"reviewer"`
	Extra    bool    `json:"extra"`
}

// Waived は見送った指摘。
type Waived struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     *int   `json:"line"`
	Reason   string `json:"reason"`
}

// Acceptance は受け入れ条件の結果。
type Acceptance struct {
	ID       string `json:"id"`
	Result   string `json:"result"`
	Evidence string `json:"evidence"`
}

// Risk は担当の一方通行の申告（変更そのものが戻せないか）。機械が読むのは one_way だけで、impact は人が読む欄。
type Risk struct {
	OneWay *bool    `json:"one_way"`
	Impact []string `json:"impact,omitempty"`
}

// Next は宛先つきの次の一手。
type Next struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// Observation は観察の課題の結果（ADR-0011 決めたこと 7）。
type Observation struct {
	Hypothesis string   `json:"hypothesis"`
	Result     string   `json:"result"`
	Evidence   []string `json:"evidence"`
}

// Report は report.json（担当が書く。command が書く）。commit は repo のある課題では必須で、確かめるのは andon issue report・ready。
type Report struct {
	Commit        string         `json:"commit,omitempty"` // repo の無い課題では書かない
	PR            string         `json:"pr,omitempty"`
	ReviewRequest *ReviewRequest `json:"review_request,omitempty"`
	Estimate      *Estimate      `json:"estimate,omitempty"` // 担当の見込み。andon issue report が前の報告から引き継ぐ
	Waived        []Waived       `json:"waived,omitempty"`
	Acceptance    []Acceptance   `json:"acceptance,omitempty"`
	Risk          *Risk          `json:"risk,omitempty"`
	Next          []Next         `json:"next,omitempty"`
	Observation   *Observation   `json:"observation,omitempty"`
	Remember      []string       `json:"remember,omitempty"`
	AgentsMD      []string       `json:"agents_md,omitempty"`
}

// Estimate は担当の見込み（依頼書の見込みと違うと見たときだけ。ADR-0014 決めたこと 4）。課題の見込み（issue.yaml の estimate）は変えない。
type Estimate struct {
	Size   string `json:"size"`
	Reason string `json:"reason"`
}

// Check は report.json の形を schema/report.schema.json と同じ決まりで確かめる。
func (r *Report) Check() error {
	if r.Commit != "" && !IsSHA(r.Commit) {
		return errors.New("commit が40桁・小文字でない")
	}
	if r.PR != "" {
		if _, _, _, ok := ParsePR(r.PR); !ok {
			return fmt.Errorf("pr が https://github.com/<owner>/<repo>/pull/<n> でない: %q", r.PR)
		}
	}
	if q := r.ReviewRequest; q != nil {
		switch {
		case q.Round < 1 || q.Limit < 1:
			return errors.New("review_request の round・limit が 1 以上でない")
		case !IsSHA(q.Commit):
			return errors.New("review_request.commit が40桁・小文字でない")
		case q.Previous != nil && !IsSHA(*q.Previous):
			return errors.New("review_request.previous が sha でも null でもない")
		case q.Reviewer != "codex-review" && q.Reviewer != "claude-review":
			return errors.New("review_request.reviewer が codex-review/claude-review でない")
		}
	}
	if e := r.Estimate; e != nil {
		if (e.Size != "small" && e.Size != "medium" && e.Size != "large") || e.Reason == "" {
			return errors.New("estimate は {size: small|medium|large, reason}")
		}
	}
	for _, w := range r.Waived {
		if err := (Finding{Severity: w.Severity, File: w.File, Line: w.Line}).check(); err != nil {
			return fmt.Errorf("waived: %v", err)
		}
		if w.Reason == "" {
			return errors.New("waived の reason が空")
		}
	}
	for _, a := range r.Acceptance {
		if a.ID == "" || a.Evidence == "" || (a.Result != "pass" && a.Result != "fail" && a.Result != "manual") {
			return errors.New("acceptance は {id, result: pass|fail|manual, evidence}")
		}
	}
	if k := r.Risk; k != nil {
		if k.OneWay == nil {
			return errors.New("risk に one_way（true/false）が無い")
		}
		for _, s := range k.Impact {
			if s == "" {
				return errors.New("risk.impact に空の行")
			}
		}
	}
	for _, n := range r.Next {
		if n.Text == "" || (n.To != "person" && n.To != "chief-of-staff" && n.To != "issue") {
			return errors.New("next は {to: person|chief-of-staff|issue, text}")
		}
	}
	if o := r.Observation; o != nil {
		if o.Hypothesis == "" || (o.Result != "hit" && o.Result != "miss" && o.Result != "unknown") {
			return errors.New("observation は {hypothesis, result: hit|miss|unknown, evidence[]}")
		}
		for _, s := range o.Evidence {
			if s == "" {
				return errors.New("observation.evidence に空の行")
			}
		}
	}
	for _, s := range append(append([]string{}, r.Remember...), r.AgentsMD...) {
		if s == "" {
			return errors.New("remember・agents_md に空の行")
		}
	}
	return nil
}

// ParseReport は report.json の中身を読んで検査する。
func ParseReport(b []byte) (*Report, error) {
	var r Report
	if err := Decode(b, &r); err != nil {
		return nil, err
	}
	if r.ReviewRequest != nil {
		// previous は必須（null 可）。Decode では null と無いの区別がつかないので、生で確かめる。
		var raw struct {
			RR map[string]json.RawMessage `json:"review_request"`
		}
		_ = json.Unmarshal(b, &raw)
		for _, k := range []string{"round", "limit", "commit", "previous", "reviewer", "extra"} {
			if _, ok := raw.RR[k]; !ok {
				return nil, fmt.Errorf("review_request に %s が無い", k)
			}
		}
	}
	if r.Observation != nil {
		// evidence は必須（空の配列は可）。無いと null になるので、生で確かめる。
		var raw struct {
			O map[string]json.RawMessage `json:"observation"`
		}
		_ = json.Unmarshal(b, &raw)
		if v, ok := raw.O["evidence"]; !ok || string(v) == "null" {
			return nil, errors.New("observation に evidence が無い")
		}
	}
	if err := r.Check(); err != nil {
		return nil, err
	}
	return &r, nil
}

// ReadReport は report.json を読む。
func ReadReport(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseReport(b)
}

// Option は問いの選択肢。
type Option struct {
	Label string `json:"label"`
	Pick  *bool  `json:"pick,omitempty"`
}

// Question は問いの JSON（contract.md §5）。
type Question struct {
	Kind      string   `json:"kind"`
	Question  string   `json:"question"`
	Facts     []string `json:"facts,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Options   []Option `json:"options"`
	Remaining *int     `json:"remaining,omitempty"`
	From      string   `json:"from,omitempty"`
}

// ParseQuestion は問いの JSON を読んで検査する。
func ParseQuestion(b []byte) (*Question, error) {
	var q Question
	if err := Decode(b, &q); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	if _, ok := raw["reason"]; ok && q.Reason == "" {
		return nil, errors.New("reason が空")
	}
	switch q.Kind {
	case "business", "users", "promise", "irreversible", "line", "scope":
	default:
		return nil, fmt.Errorf("kind が business/users/promise/irreversible/line/scope でない: %q", q.Kind)
	}
	if q.Question == "" || strings.Contains(q.Question, "\n") {
		return nil, errors.New("question が空か2行以上")
	}
	for _, f := range q.Facts {
		if f == "" {
			return nil, errors.New("facts に空の行")
		}
	}
	if len(q.Options) < 2 {
		return nil, errors.New("options が2つ未満")
	}
	for i, o := range q.Options {
		if o.Label == "" {
			return nil, errors.New("options の label が空")
		}
		if i == 0 && (o.Pick == nil || !*o.Pick) {
			return nil, errors.New("options の先頭が推奨案（pick: true）でない")
		}
		if i > 0 && o.Pick != nil && *o.Pick {
			return nil, errors.New("推奨案（pick: true）は先頭の1つだけ")
		}
	}
	if q.Remaining != nil && *q.Remaining < 0 {
		return nil, errors.New("remaining が負")
	}
	switch q.From {
	case "", "issue", "gate", "retro", "chief-of-staff", "chief-engineer":
	default:
		return nil, fmt.Errorf("from が知らない書き手: %q", q.From)
	}
	return &q, nil
}

// Text は問いから人向けの text を描く（台帳の questions.yaml に入る。機械は読まない）。
func (q *Question) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s（%s）", q.Question, q.Kind)
	if len(q.Facts) > 0 {
		fmt.Fprintf(&b, "\n事実: %s", strings.Join(q.Facts, " / "))
	}
	if q.Reason != "" {
		fmt.Fprintf(&b, "\n理由: %s", q.Reason)
	}
	fmt.Fprintf(&b, "\n推奨案: %s", q.Options[0].Label)
	var others []string
	for _, o := range q.Options[1:] {
		others = append(others, o.Label)
	}
	fmt.Fprintf(&b, "\nほかの案: %s", strings.Join(others, " / "))
	if q.Remaining != nil && *q.Remaining > 0 {
		fmt.Fprintf(&b, "\nあと %d 問", *q.Remaining)
	}
	return b.String()
}
