package issue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func TestSendFix(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "hc"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+filepath.Join(d, "sent")+"\n[ ! -e "+filepath.Join(d, "sendfail")+" ] || { echo 'no terminal' >&2; exit 3; }\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	const record = "/k/issues/D-0001/library/gate/x.json"
	gr := gateResult{Decision: "fix", Reasons: []string{"Must fix 以上が 1 件"}}
	fix := func(agent string) (*contract.Merge, string) {
		t.Helper()
		os.Remove(filepath.Join(d, "sent"))
		c := &ctx{l: &ledger.Ledger{Root: d, Slug: "demo", Dir: filepath.Join(d, "kanban")}, id: "D-0001",
			issue: ledger.Issue{"id": "D-0001", "title": "説明を足す", "agent": agent}}
		res := &contract.Merge{Notify: []contract.Notice{{Kind: "gate", Detail: map[string]any{"decision": "fix", "reasons": gr.Reasons, "record": record}}}}
		c.sendFix(gr, record, res)
		b, _ := os.ReadFile(filepath.Join(d, "sent"))
		return res, string(b)
	}
	res, sent := fix("t7")
	if !strings.HasPrefix(sent, "send t7 [gate] D-0001（説明を足す）: gate が fix を返した。理由: Must fix 以上が 1 件。記録は "+record+"。") {
		t.Errorf("fix を担当へ送っていない・文が違う: %q", sent)
	}
	if d := res.Notify[0].Detail; d["unsent"] != nil || d["no_agent"] != nil || len(res.Notify) != 1 {
		t.Errorf("送れた fix に印を書いた: %+v", res.Notify)
	}
	os.WriteFile(filepath.Join(d, "sendfail"), nil, 0o644)
	res, _ = fix("t7")
	if d := res.Notify[0].Detail; d["unsent"] != "t7" || d["record"] != record || len(res.Notify) != 1 {
		t.Errorf("送れなかった fix に unsent を書いていない（理由と記録を残していない）: %+v", res.Notify)
	}
	if len(res.Log) != 1 || !strings.HasPrefix(res.Log[0], "D-0001 の担当 t7 に fix を送れない") {
		t.Errorf("送れなかったことをログに書かない: %q", res.Log)
	}
	res, sent = fix("")
	if d := res.Notify[0].Detail; d["no_agent"] != true || d["unsent"] != nil || d["record"] != record || len(res.Notify) != 1 || sent != "" {
		t.Errorf("担当がいない fix に no_agent を書いていない・誰かに送った: %+v %q", res.Notify, sent)
	}
	if len(res.Log) != 1 || !strings.HasPrefix(res.Log[0], "D-0001 の gate が fix だが担当（agent）がいない") {
		t.Errorf("担当がいないことをログに書かない: %q", res.Log)
	}
}
