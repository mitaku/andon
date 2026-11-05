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
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     *int   `json:"line"`
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

// Next は宛先つきの次の一手。
type Next struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// Report は report.json（担当が書く。command が書く）。
type Report struct {
	Commit        string         `json:"commit"`
	PR            string         `json:"pr,omitempty"`
	ReviewRequest *ReviewRequest `json:"review_request,omitempty"`
	Waived        []Waived       `json:"waived,omitempty"`
	Acceptance    []Acceptance   `json:"acceptance,omitempty"`
	Next          []Next         `json:"next,omitempty"`
	Remember      []string       `json:"remember,omitempty"`
	AgentsMD      []string       `json:"agents_md,omitempty"`
}

// Check は report.json の形を schema/report.schema.json と同じ決まりで確かめる。
func (r *Report) Check() error {
	if !IsSHA(r.Commit) {
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
	for _, w := range r.Waived {
		if err := (Finding{w.Severity, w.File, w.Line}).check(); err != nil {
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
	for _, n := range r.Next {
		if n.Text == "" || (n.To != "person" && n.To != "chief-of-staff" && n.To != "task") {
			return errors.New("next は {to: person|chief-of-staff|task, text}")
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
	case "business", "users", "promise", "irreversible", "line":
	default:
		return nil, fmt.Errorf("kind が business/users/promise/irreversible/line でない: %q", q.Kind)
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
	case "", "task", "gate", "retro", "chief-of-staff", "chief-engineer":
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

// LedgerKind は台帳の kind（4種）。line は台帳が受けないので空（相談 F3）。
func (q *Question) LedgerKind() string {
	if q.Kind == "line" {
		return ""
	}
	return q.Kind
}
