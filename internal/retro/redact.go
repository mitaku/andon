package retro

import (
	"regexp"
)

// 伏せる秘密の形（ADR-0028 決めたこと 12 の「実装が決め」）。書き出した JSON の文字列に当てるので、
// どの形も JSON の文字列の外（"）とエスケープ（\）にまたがらない。伏せた所は [REDACTED:<名前>] に置き換える。
// 多めに伏せる（秘密でない文字列が伏せられても、振り返りの読みに要る所はほとんど無い）。
var secrets = []struct {
	name string
	re   *regexp.Regexp
	keep int // 残す前半の部分一致の番号（0 なら全部伏せる）
}{
	// 秘密鍵の塊（JSON の中では改行が \n になっている）
	{"private-key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[^"]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), 0},
	{"anthropic", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`), 0},
	{"openai", regexp.MustCompile(`\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}`), 0},
	{"github", regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,})`), 0},
	{"aws", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), 0},
	{"slack", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`), 0},
	{"google", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`), 0},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), 0},
	{"bearer", regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9._~+/=-]{20,}`), 1},
	// 鍵の名前に続く値（api_key=…・"password": "…" など。名前は残し、値だけ伏せる）
	{"assignment", regexp.MustCompile(`(?i)((?:\b|_)(?:api[_-]?key|access[_-]?key|secret[_-]?key|client[_-]?secret|secret|token|password|passwd|pwd)(?:\\?["'])?\s*[:=]\s*(?:\\?["'])?)[^\s"'\\,;\[\]]+`), 1}, // 値の長さは問わない（短いパスワードも伏せる）。[ ] は前の形が伏せた印を2度数えないため
}

// redact は b（JSON）の中の秘密の形を伏せ、伏せたあとの JSON と伏せた数を返す。
func redact(b []byte) ([]byte, int) {
	n := 0
	for _, s := range secrets {
		b = s.re.ReplaceAllFunc(b, func(m []byte) []byte {
			n++
			mark := []byte("[REDACTED:" + s.name + "]")
			if s.keep == 0 {
				return mark
			}
			sub := s.re.FindSubmatch(m)
			return append(append([]byte{}, sub[s.keep]...), mark...)
		})
	}
	return b, n
}
