package contract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// unchecked は、照らさずに JSON を作る呼び出し（encoding/json の Marshal・MarshalIndent・NewEncoder と、
// この package の Marshal・WriteState）を、ファイルごとに数えた表。どれも約束の JSON（schema/ の直下の 16 本）を書かない。
// 約束の JSON は Write・Encode・EncodeLine で書く（書く前に schema と照らす。ADR-0034 決めたこと 6）。
// 呼び出しを足すとき: 約束の JSON なら照らす口で書く。約束でなければ、ここに数と理由を足す。
var unchecked = map[string]struct {
	calls map[string]int
	why   string
}{
	"internal/boot/boot.go":                    {map[string]int{"json.MarshalIndent": 2}, "boot の当て直しの控えと、Claude Code の settings.json（約束の外のファイル）"},
	"internal/boot/trust.go":                   {map[string]int{"json.NewEncoder": 1}, "Claude Code・Codex の信頼の設定（約束の外のファイル）"},
	"internal/delegate/cmd.go":                 {map[string]int{"contract.Marshal": 2}, "delegate show --json の出力（schema の無い見せる形）と、project.json の中の delegation（全体を Encode で照らす）"},
	"internal/delegate/delegate.go":            {map[string]int{"json.Marshal": 1}, "委譲の値を1つ読み直して確かめる途中の形（書かない）"},
	"internal/dispatcher/drain.go":             {map[string]int{"json.Marshal": 1, "json.NewEncoder": 1}, "フックの出力（Claude Code への hookSpecificOutput）と、Stop の控え（schema の無い控え）"},
	"internal/dispatcher/impact.go":            {map[string]int{"json.Marshal": 1}, "影響の結果を通知の detail の形にそろえる途中（通知は控えに書く前に State.Save が照らす）"},
	"internal/dispatcher/memory.go":            {map[string]int{"json.Marshal": 1}, "memory の件数の控え（schema の無い控え）"},
	"internal/dispatcher/rewake.go":            {map[string]int{"json.Marshal": 1}, "切られたターンを起動した回数の控え（schema の無い控え）"},
	"internal/dispatcher/state.go":             {map[string]int{"json.MarshalIndent": 1}, "dispatcher の控え（schema の無い控え。中の通知は Save が書く前に照らす）"},
	"internal/dispatcher/wait.go":              {map[string]int{"json.Marshal": 1}, "待ち役の控え（schema の無い控え）"},
	"internal/gate/gate.go":                    {map[string]int{"json.Marshal": 2}, "条件表を重ねた途中の形（書かない。Decode で読み直す）と、信じられる PR が無いときの fix の出力（decision と reasons だけ。記録ではないので schema が無い）"},
	"internal/ledger/ledgertest/ledgertest.go": {map[string]int{"json.Marshal": 1}, "テストが偽の kanban の CLI の ready --json の返事を作る形（schema/ledger/ready.v3 と照らしてから返す。テストだけが使う）"},
	"internal/issue/impact.go":                 {map[string]int{"contract.Marshal": 1}, "andon issue impact の出力（schema の無い形。dispatcher が通知の detail にして照らす）"},
	"internal/issue/issue.go":                  {map[string]int{"contract.Marshal": 1}, "andon issue show の出力（読み直した report.json と kanban の項目を見せる形）"},
	"internal/issue/merge.go":                  {map[string]int{"contract.Marshal": 1, "contract.WriteState": 1, "json.Marshal": 1}, "andon issue merge の出力と merge の記録（schema の無い形）と、問いを ask に渡す形（書くのは ask で、そこで照らす）"},
	"internal/issue/resume.go":                 {map[string]int{"contract.Marshal": 1}, "andon issue resume --dry-run --json の起動の計画（schema の無い形）"},
	"internal/issue/settle.go":                 {map[string]int{"contract.Marshal": 1, "contract.WriteState": 1}, "andon issue settle の出力と settle の記録（schema の無い形）"},
	"internal/issue/start.go":                  {map[string]int{"contract.Marshal": 1}, "andon issue start --dry-run --json の起動の計画（schema の無い形）"},
	"internal/issue/stopcheck.go":              {map[string]int{"json.Marshal": 2}, "Stop フックの出力（Claude Code への decision）と、止めた回数の控え（schema の無い控え）"},
	"internal/otel/otel.go":                    {map[string]int{"json.NewEncoder": 1}, "ハーネスの OTLP の JSON の行（約束の外の、OpenTelemetry の形。contract.md §30）"},
	"internal/pkgversion/pkgversion.go":        {map[string]int{"json.MarshalIndent": 2}, "パッケージのバージョンの控えと andon version の出力（schema の無い形）"},
	"internal/digest/build.go":                 {map[string]int{"contract.Marshal": 1}, "秘密を伏せる前の形（伏せたあとを contract.Decode で読み直し、書くときに Encode が照らす）"},
	"internal/digest/collect.go":               {map[string]int{"contract.Marshal": 1}, "秘密を伏せる前の形（伏せたあとの入力を writeInput の contract.Write が照らす）"},
	"internal/digest/slack.go":                 {map[string]int{"json.Marshal": 1}, "Slack へ送る files の欄（約束の外の、Slack の API の形）"},
	"internal/process/process.go":              {map[string]int{"json.Marshal": 1}, "流れと状態の定義の YAML を読むための変換（書かない）"},
	"internal/retro/apply.go":                  {map[string]int{"contract.Marshal": 1, "contract.WriteState": 1}, "andon retro apply --dry-run の出力と retro-applied.json（schema の無い記録）"},
	"internal/retro/retro.go":                  {map[string]int{"contract.Marshal": 2}, "秘密を伏せる前の形（伏せたあとの出力を Redacted が照らす）"},
	"internal/trace/output.go":                 {map[string]int{"json.NewEncoder": 1}, "andon issue trace の --json（schema の無い見せる形）と --otlp（約束の外の、OpenTelemetry の OTLP の JSON）"},
	"internal/triage/triage.go":                {map[string]int{"json.NewEncoder": 1}, "トリアージの出力（schema の無い形）"},
}

var skipped = map[string]string{
	"internal/contract": "照らす口そのもの",
	"cmd/kanban":        "kanban の CLI（別の binary）。書くのは kanban の YAML と log.jsonl で、schema/ の直下の約束ではない",
}

// TestNoUncheckedContractWrites は、照らさずに JSON を作る所が表のとおりかをソースを読んで確かめる
// （ADR-0034 受け入れ条件 6「照合を通らずに約束の JSON を書く所が無い」）。表に無い呼び出しが増えたら落ちる。
func TestNoUncheckedContractWrites(t *testing.T) {
	root := filepath.Join("..", "..")
	got := map[string]map[string]int{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if _, ok := skipped[rel]; ok {
				return filepath.SkipDir
			}
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		names := map[string]string{} // import の名前 → 数える呼び方の頭
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			short := map[string]string{"encoding/json": "json", "andon/internal/contract": "contract"}[p]
			if short == "" {
				continue
			}
			name := short
			if im.Name != nil {
				name = im.Name.Name
			}
			names[name] = short
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok || names[x.Name] == "" {
				return true
			}
			key := names[x.Name] + "." + sel.Sel.Name
			switch key {
			case "json.Marshal", "json.MarshalIndent", "json.NewEncoder", "contract.Marshal", "contract.WriteState":
				if got[rel] == nil {
					got[rel] = map[string]int{}
				}
				got[rel][key]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for file, calls := range got {
		want := unchecked[file].calls
		for k, n := range calls {
			if want[k] != n {
				bad = append(bad, fmt.Sprintf("%s: %s が %d か所（表は %d）", file, k, n, want[k]))
			}
		}
	}
	for file, u := range unchecked {
		for k, n := range u.calls {
			if got[file][k] != n {
				bad = append(bad, fmt.Sprintf("%s: %s が %d か所（表は %d）", file, k, got[file][k], n))
			}
		}
		if u.why == "" {
			bad = append(bad, file+": 表に理由が無い")
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		bad = slices.Compact(bad)
		t.Fatalf("照らさずに JSON を作る所が表と合わない。約束の JSON なら contract.Write・Encode・EncodeLine で書き、"+
			"約束でなければ writers_test.go の表に数と理由を足す:\n  %s", strings.Join(bad, "\n  "))
	}
	n := 0
	for _, u := range unchecked {
		for _, c := range u.calls {
			n += c
		}
	}
	t.Logf("照らさずに JSON を作る所は %d ファイルの %d か所で、どれも表のとおり約束の外", len(unchecked), n)
}
