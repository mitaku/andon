package trace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// RoleTokens は、ハーネスの記録のプロジェクトの置き場 dir の、課題の無い役 role の生の記録
// （roles/<role>/<日付>.traces.jsonl。docs/contract.md §30）のうち、日付が from から to まで（UTC の日付。両端を含む）のものから、
// モデルの呼び出し（claude_code.llm_request）のトークンを合わせる（andon measure の Chief of Staff の分。ADR-0060 決めたこと 16）。
// 数え方は内訳の tokens と同じで、同じ span（traceId と spanId が同じ）は 1 つに数える。
// days は 1 行以上読めた日付のファイルの数（数えられた日数）。0 なら tokens は nil（記録が無いのと 0 を分ける）。
// 在るのに読めないファイルと JSON として読めない行は skip に渡して飛ばす（file は置き場からの相対のパス）。
func RoleTokens(dir, role string, from, to time.Time, skip func(file string, line int, reason string)) (tokens *Tokens, days int) {
	if dir == "" {
		return nil, 0
	}
	var sum Tokens
	seen := map[string]bool{}
	for d := from.UTC().Truncate(24 * time.Hour); !d.After(to); d = d.AddDate(0, 0, 1) {
		rel := filepath.ToSlash(filepath.Join("roles", role, d.Format("2006-01-02")+".traces.jsonl"))
		read := 0
		err := eachLine(filepath.Join(dir, rel), func(n int, b []byte) {
			var line lightLine
			if json.Unmarshal(b, &line) != nil {
				skip(rel, n, "JSON として読めない行")
				return
			}
			read++
			for _, rs := range line.ResourceSpans {
				for _, ss := range rs.ScopeSpans {
					for _, x := range ss.Spans {
						if x.Name != "claude_code.llm_request" || x.TraceID == "" || x.SpanID == "" || seen[x.TraceID+"/"+x.SpanID] {
							continue
						}
						seen[x.TraceID+"/"+x.SpanID] = true
						attrs := attrsLight(x.Attributes)
						add(&sum.Input, intAttr(attrs, "input_tokens"))
						add(&sum.Output, intAttr(attrs, "output_tokens"))
						add(&sum.CacheRead, intAttr(attrs, "cache_read_tokens"))
						add(&sum.CacheWrite, intAttr(attrs, "cache_creation_tokens"))
					}
				}
			}
		})
		if err != nil {
			skip(rel, 0, err.Error())
		}
		if read > 0 {
			days++
		}
	}
	if days == 0 {
		return nil, 0
	}
	return &sum, days
}

// eachLine は JSON Lines のファイルを 1 行ずつ渡す（空の行は飛ばす）。無ければ何もしない。在るのに読めなければ理由を返す。
func eachLine(path string, f func(n int, b []byte)) error {
	fh, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return errors.New("読めない（" + pe.Err.Error() + "）")
		}
		return errors.New("読めない（読み取りの誤り）")
	}
	defer fh.Close() //nolint:errcheck // 読むだけのファイル
	s := bufio.NewScanner(fh)
	s.Buffer(make([]byte, 64<<10), 64<<20)
	n := 0
	for s.Scan() {
		n++
		if len(bytes.TrimSpace(s.Bytes())) == 0 {
			continue
		}
		f(n, s.Bytes())
	}
	if s.Err() != nil {
		return errors.New("読み切れない（" + s.Err().Error() + "）")
	}
	return nil
}
