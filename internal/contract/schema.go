package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"andon"

	"github.com/google/jsonschema-go/jsonschema"
)

// Schema は約束の JSON の schema の名前（schema/<名前>.schema.json。docs/contract.md の冒頭の表）。
// 約束の JSON は Write・Encode・EncodeLine で、書く前にこの schema と照らす（ADR-0034 決めたこと 6）。
type Schema string

// schema/ の直下の 17 本。
const (
	SchemaCandidates   Schema = "candidates"
	SchemaCorrections  Schema = "corrections"
	SchemaDelivered    Schema = "delivered"
	SchemaFindings     Schema = "findings"
	SchemaGate         Schema = "gate"
	SchemaIssueKeys    Schema = "issue-keys"
	SchemaLaunch       Schema = "launch"
	SchemaMeasure      Schema = "measure"
	SchemaNotification Schema = "notification"
	SchemaOutcome      Schema = "outcome"
	SchemaPatterns     Schema = "patterns"
	SchemaProject      Schema = "project"
	SchemaQuestion     Schema = "question"
	SchemaReport       Schema = "report"
	SchemaRetro        Schema = "retro"
	SchemaRun          Schema = "run"
	SchemaSignal       Schema = "signal"
)

// Schemas は schema/ の直下の全部（テストが、埋め込んだ顔ぶれと突き合わせる）。
var Schemas = []Schema{SchemaCandidates, SchemaCorrections, SchemaDelivered, SchemaFindings, SchemaGate, SchemaIssueKeys,
	SchemaLaunch, SchemaMeasure, SchemaNotification, SchemaOutcome, SchemaPatterns, SchemaProject, SchemaQuestion,
	SchemaReport, SchemaRetro, SchemaRun, SchemaSignal}

var (
	resolvedMu sync.Mutex
	resolved   = map[Schema]*jsonschema.Resolved{}
)

// File は埋め込んだ schema の置き場（repo からの相対）。
func (s Schema) File() string { return "schema/" + string(s) + ".schema.json" }

// compile は埋め込んだ schema を読んで解く（1度だけ。読めなければ毎回誤りを返す）。
// 標準に無いキーワード uniqueKeys は検査器が読み飛ばすので、一意は呼び手が確かめる（Candidates.Check など）。
func (s Schema) compile() (*jsonschema.Resolved, error) {
	resolvedMu.Lock()
	defer resolvedMu.Unlock()
	if r := resolved[s]; r != nil {
		return r, nil
	}
	b, err := andon.Schemas.ReadFile(s.File())
	if err != nil {
		return nil, fmt.Errorf("%s が埋め込まれていない: %v", s.File(), err)
	}
	var js jsonschema.Schema
	if err := json.Unmarshal(b, &js); err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", s.File(), err)
	}
	r, err := js.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", s.File(), err)
	}
	resolved[s] = r
	return r, nil
}

// Check は JSON の b が schema に合うかを確かめる。合わなければ、どこがどう合わないかの誤り。
func (s Schema) Check(b []byte) error {
	r, err := s.compile()
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("%s と照らせない（JSON でない）: %v", s.File(), err)
	}
	if err := r.Validate(v); err != nil {
		return fmt.Errorf("%s に合わない: %v", s.File(), err)
	}
	return nil
}

// Encode は v を約束の JSON（2字下げ・HTML の文字を逃がさない・末尾に改行）にし、schema と照らす。合わなければ誤りで、中身を返さない。
func Encode(s Schema, v any) ([]byte, error) {
	b := Marshal(v)
	if err := s.Check(b); err != nil {
		return nil, err
	}
	return b, nil
}

// EncodeLine は v を1行の約束の JSON（HTML の文字を逃がさない・末尾の改行は無し）にし、schema と照らす。
// 1行1件で足していく置き場（*.jsonl）に使う。
func EncodeLine(s Schema, v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	line := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	if err := s.Check(line); err != nil {
		return nil, err
	}
	return line, nil
}

// Write は v を約束の JSON にして schema と照らし、合えば path に書く（一時ファイルに書いてから置き換える）。
// 合わなければ書かずに誤りを返す（前の中身は残る）。
func Write(path string, s Schema, v any) error {
	b, err := Encode(s, v)
	if err != nil {
		return err
	}
	return writeFile(path, b)
}

// WriteState は約束でない JSON（schema の無い控え。dispatcher の merge の記録など）を書く。照らさない。
// 呼ぶ所は、ソースを読むテスト（writers_test.go）の表に理由と載せる。
func WriteState(path string, v any) error {
	return writeFile(path, Marshal(v))
}

func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
