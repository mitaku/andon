package spawn

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// 作り直し（herdr-core が自分の状態から端末を戻す）や core の起き直しのあと、端末は名前とラベルをそのままに、
// 中身が素のシェル（argv ["/bin/bash"] など）に替わって live で戻ってくる。ラベルと live だけでは
// 「動いている」と言えないので、前に出ているもの（hc get の foreground.argv）で見る。
// エージェントは sh -c '… claude …' で起きるので foreground.name が sh のこともある。名前ではなく
// 「引数の無いシェル」かで見分ける。

// Shells は、前に出ていれば「中身がシェル」とみなすもの。
var Shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ash": true, "fish": true, "nu": true}

// BareShell は argv が引数の無いシェル（戻った端末の中身・人が抜けたあと）か。sh -c '…' のような起動の途中は含めない。
func BareShell(argv []string) bool {
	return len(argv) == 1 && Shells[strings.TrimPrefix(filepath.Base(argv[0]), "-")]
}

// Term は hc get --json のうち、役の端末の見分けに読むもの。
type Term struct {
	Address    string            `json:"address"`
	Name       string            `json:"name"`
	Cwd        string            `json:"cwd"`
	State      string            `json:"state"`
	Labels     map[string]string `json:"labels"`
	Foreground *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} `json:"foreground"`
}

// Shell は前に出ているものが引数の無いシェルか（作り直しで戻った・中身が抜けた端末）。
func (t *Term) Shell() bool { return t.Foreground != nil && BareShell(t.Foreground.Argv) }

// SettleWait は前に出ているものが取れるまで読み直す上限。settlePoll はその間隔。テストで差し替える。
var (
	SettleWait = 5 * time.Second
	settlePoll = 500 * time.Millisecond
)

// Look は hc get addr。前に出ているものが取れない（戻った直後・替わる途中）あいだは SettleWait まで読み直す。
// 上限を過ぎても取れなければ Foreground が nil のまま返す（呼び手は「確かめられない」として扱う）。
func Look(addr string) (*Term, error) {
	end := time.Now().Add(SettleWait)
	for {
		out, err := Output(nil, "", "hc", "get", addr, "--json")
		if err != nil {
			return nil, fmt.Errorf("hc get %s に失敗: %v", addr, err)
		}
		var t Term
		if err := json.Unmarshal([]byte(out), &t); err != nil {
			return nil, fmt.Errorf("hc get %s の返事が読めない: %v", addr, err)
		}
		if t.Foreground != nil || !time.Now().Before(end) {
			return &t, nil
		}
		time.Sleep(settlePoll)
	}
}

// Discard は端末を片付ける（keep を外して kill・rm。address で打つ）。中身がシェルに替わった役の端末に使う。
// シェルには打ち込まない（入力欄に人の打ちかけがあれば、それに足して実行してしまう）。
func Discard(addr string) error {
	for _, args := range [][]string{{"keep", addr, "--off"}, {"kill", addr}, {"rm", addr}} {
		if _, err := Output(nil, "", "hc", args...); err != nil {
			return fmt.Errorf("hc %s に失敗: %v", strings.Join(args, " "), err)
		}
	}
	return nil
}
