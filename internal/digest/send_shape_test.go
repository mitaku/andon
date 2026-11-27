package digest

import (
	"strings"
	"testing"

	"andon/internal/contract"
)

// sent.json の kind tray の行の形（ADR-0064 決めたこと 11）。tests/schema.sh は tray を名指せないので、ここで見る。
func TestSentTrayRowShape(t *testing.T) {
	const head = `{"issue": "S-0100", "since": "2026-11-20T10:00:00Z", "until": "2026-11-21T10:00:00Z", "destinations": [` +
		`{"kind": "files", "at": "2026-11-21T10:01:00Z", "ok": true, "url": "http://tray.cell.localhost:1355/r/01"}, `
	for _, c := range []struct {
		row  string
		pass bool
		why  string
	}{
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": true, "url": "https://tray.example.com/r/01"}`, true, "Cloudflare の tray に届いた"},
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": false, "error": "HTTP 503"}`, true, "送りそこねた"},
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": true}`, false, "届いたのに url が無い"},
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": true, "url": "https://tray.example.com/r/01", "path": "files/a.html"}`, false, "path を持つ"},
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": true, "url": "https://tray.example.com/r/01", "channel": "C01"}`, false, "channel を持つ"},
		{`{"kind": "tray", "at": "2026-11-21T10:01:00Z", "ok": false, "error": "x", "url": "https://tray.example.com/r/01"}`, false, "送りそこねたのに url"},
	} {
		err := contract.SchemaDigestSent.Check([]byte(head + c.row + `]}`))
		if (err == nil) != c.pass {
			t.Errorf("%s: err=%v（%s）", c.why, err, strings.TrimSpace(c.row))
		}
	}
}
