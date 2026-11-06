package spawn

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// 作り直し（herdr-core が自分の状態から端末を戻す）や core の起き直しのあと、端末は名前とラベルをそのままに、
// 中身が素のシェルに替わって live で戻ってくる。戻った端末は top-level の argv ごとシェルに替わる（P7 の実測: argv ["/bin/bash"]）。
// 起こした役の端末の top-level の argv は template の command（sh -c '… claude …'）のまま。
// ラベルと live だけでは「動いている」と言えないので、top-level の argv と、hc の agent の情報（status.app）で見分ける。

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
	Argv       []string          `json:"argv"`
	Labels     map[string]string `json:"labels"`
	Foreground *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} `json:"foreground"`
	Status *struct {
		App string `json:"app"`
	} `json:"status"`
}

// Agent は hc の agent の情報（status.app。エージェントの連携が報告する。報告が無ければ空）。
func (t *Term) Agent() string {
	if t.Status == nil {
		return ""
	}
	return t.Status.App
}

// Kind は役の端末の見分け。
type Kind int

const (
	Unsure   Kind = iota // 動いているとも戻った端末とも言えない（片付けず、動いているとも数えない）
	Running              // Claude Code が動いている
	Restored             // 作り直しで中身が素のシェルに替わって戻った端末（片付けてよい）
)

// AgentApp は役の端末で動いているはずのもの（hc の agent の値）。
const AgentApp = "claude-code"

// Kind は端末を見分ける。戻った端末は top-level の argv も前に出ているものも引数の無いシェル
// （役のラベルの付いた端末は Andon の持ち物で、人がシェルとして使う場所ではない、と決めた。decisions.md §16）
// （top-level がシェルでも前に別のものが出ていれば、人が中で何かしているとみて Unsure）。
// 動いているは、top-level の argv がシェルでなく、hc の agent が Claude Code のときだけ。
// agent の情報が無い・別のもの・前に出ているものが取れないは Unsure。
func (t *Term) Kind() Kind {
	switch {
	case BareShell(t.Argv):
		if t.Shell() {
			return Restored
		}
		return Unsure
	case t.Agent() == AgentApp:
		return Running
	}
	return Unsure
}

// Shell は前に出ているものが引数の無いシェルか。
func (t *Term) Shell() bool { return t.Foreground != nil && BareShell(t.Foreground.Argv) }

// Describe は人に見せる端末の様子（argv・前に出ているもの・agent）。
func (t *Term) Describe() string {
	fg := "取れない"
	if t.Foreground != nil {
		fg = strings.Join(t.Foreground.Argv, " ")
	}
	agent := t.Agent()
	if agent == "" {
		agent = "無し"
	}
	return fmt.Sprintf("argv %s・前に %s・agent %s", strings.Join(t.Argv, " "), fg, agent)
}

// SettleWait は前に出ているものが取れるまで読み直す上限。settlePoll はその間隔。テストで差し替える。
var (
	SettleWait = 5 * time.Second
	settlePoll = 500 * time.Millisecond
)

// Look は hc get addr。見分け（Kind）が Unsure のあいだ（前に出ているものが取れない・起動の直後で agent の情報がまだ無い）は
// SettleWait まで読み直す。上限を過ぎても Unsure なら最後に読んだものを返す（呼び手は「確かめられない」として扱う）。
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
		if t.Kind() != Unsure || !time.Now().Before(end) {
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
