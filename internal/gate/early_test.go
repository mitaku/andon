package gate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// 信じられる PR が無いときの fix は、理由に制御文字や壊れた UTF-8 があっても JSON の1行で出る（internal/issue の gateResult が読む形）。
func TestWriteEarly(t *testing.T) {
	for _, reason := range []string{
		"issue.yaml に branch が無い",
		"issue.yaml の repo（github.com/a/b\x01\x1b[31m\t\"x\"\\）と PR の repo（a/b）が違う",
		"壊れた UTF-8 \xff\xfe",
	} {
		var out bytes.Buffer
		if err := writeShort(&out, "fix", reason); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(out.String(), "}\n") || strings.Count(out.String(), "\n") != 1 {
			t.Fatalf("1行でない: %q", out.String())
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("JSON でない: %q %v", out.String(), err)
		}
		reasons, ok := got["reasons"].([]any)
		if len(got) != 2 || got["decision"] != "fix" || !ok || len(reasons) != 1 {
			t.Fatalf("形が違う（decision と reasons だけ）: %q", out.String())
		}
		if r, _ := reasons[0].(string); string([]rune(reason)) != r { // 壊れたバイトは1つずつ U+FFFD になる
			t.Fatalf("理由が違う: got %q want %q", r, reason)
		}
	}
}
