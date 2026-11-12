package dispatcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RewakeUsage は andon dispatcher rewake の使い方。
const RewakeUsage = "usage: andon dispatcher rewake [--project <slug>] < フックの入力の JSON\n" +
	"Chief of Staff の StopFailure フックが asyncRewake で起こす（package/hooks/stop-failure。contract.md §9。ADR-0019 決めたこと 10）。\n" +
	"入力の error が server_error・rate_limit なら、続けて切られた 1〜4 回目はそれぞれ 1・2・4・8 分あとに合図の1行を stderr に書いて終了コード 2\n" +
	"（Claude Code が入力欄を通らずに新しいターンを始め、切られたターンの続きをする）。5 回目からは起こさず、ログに1行書いて 0。\n" +
	"回数は控えの横の <slug>.json.rewake に持ち、Chief of Staff の Stop（drain）が消す（ターンが通ったら数え直す）。\n" +
	"待ったあと、端末が error のままで回数も変わっていない（人が打ったターンが通っていない・また切られていない）ときだけ合図する。\n" +
	"Chief of Staff の端末（ラベル role=chief-of-staff）でない・ほかの error・落ちたときは、待たずに終了コード 0。\n"

// RewakeSignal は合図の文（stderr。Claude Code が system reminder でモデルに見せる）。起こしたターンには切られたユーザー文がそのまま入る（A-0135 の実測）。
const RewakeSignal = "andon: 提供元に切られて止まったターンを起こした。これは合図で、人の依頼ではない。止まる前の続きを、今の許可の範囲で続ける。"

// rewakeMax は合図で起こす回数（1・2・4・8 分）。rewakeUnit は待つ間の1単位（テストで縮める）。
var (
	rewakeMax  = 4
	rewakeUnit = time.Minute
)

// rewakeErrors は起こす error（StopFailure の入力の error。提供元の混雑・rate limit。待てば通るもの）。
var rewakeErrors = map[string]bool{"server_error": true, "rate_limit": true}

// rewakeCount は回数の控え（<slug>.json.rewake）。
type rewakeCount struct {
	Count int `json:"count"`
}

func rewakePath(slug string) string { return StatePath(slug) + ".rewake" }

// ResetRewake は回数の控えを消す（Chief of Staff のターンが Stop で通った。drain が呼ぶ）。
func ResetRewake(slug string) { os.Remove(rewakePath(slug)) }

// RewakeMain は andon dispatcher rewake。終了コードは 0（通す・上限）か 2（合図）だけ。
func RewakeMain(args []string, stdin io.Reader, stderr io.Writer) int {
	slug := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--project" && i+1 < len(args):
			slug = args[i+1]
			i++
		default:
			fmt.Fprint(stderr, RewakeUsage) // 0 で通す（終了コード 2 は合図の意味なので、使い方の誤りに使わない）
			return 0
		}
	}
	var in struct {
		Error string `json:"error"`
	}
	if stdin != nil {
		if b, _ := io.ReadAll(stdin); len(bytes.TrimSpace(b)) > 0 {
			json.Unmarshal(b, &in)
		}
	}
	if !rewakeErrors[in.Error] {
		return 0 // 待っても直らない error（認証・使用量の上限など）か、読めない入力
	}
	addr := os.Getenv("HC_TERMINAL_ID")
	if addr == "" {
		return 0
	}
	self, err := hcGet(addr)
	if err != nil || self.Labels["role"] != "chief-of-staff" || self.Labels["project"] == "" {
		return 0 // Chief of Staff の端末ではない（担当・レビュアーへの「続けて」は dispatcher が打つ。dispatcher.md §2.4）
	}
	if slug == "" {
		slug = self.Labels["project"]
	}
	if !slugRe.MatchString(slug) {
		return 0
	}
	path := rewakePath(slug)
	var c rewakeCount
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &c)
	}
	c.Count++
	b, _ := json.Marshal(c)
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path+".tmp", b, 0o644) != nil || os.Rename(path+".tmp", path) != nil {
		return 0 // 回数を持てなければ起こさない（上限を超えて起こし続けない）
	}
	if c.Count > rewakeMax {
		rewakeLog(slug, fmt.Sprintf("Chief of Staff %s は提供元に切られて %d 回目（%s）。起こさない（人が Herdr App の error で気づく）", addr, c.Count, in.Error))
		return 0
	}
	time.Sleep(time.Duration(1<<(c.Count-1)) * rewakeUnit)
	// 起こすのは、切られたターンがまだ止まったままのときだけ: 端末が error のままで、回数が自分の書いた値のまま
	// （待つ間に人が打ったターンが通れば Stop の drain が消し、また切られれば次のフックが進める）。読めなければ起こさない。
	var now rewakeCount
	b, err = os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &now) != nil || now.Count != c.Count {
		return 0
	}
	if t, err := hcGet(addr); err != nil || t.Status.State != "error" {
		return 0
	}
	fmt.Fprintln(stderr, RewakeSignal)
	return 2
}

// rewakeLog は控えの横の <slug>.json.rewake.log に1行足す（StopFailure の出力は Claude Code が捨てるので、ここに残す）。
func rewakeLog(slug, msg string) {
	f, err := os.OpenFile(rewakePath(slug)+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s andon dispatcher: %s: %s\n", time.Now().UTC().Format(time.RFC3339), slug, msg)
}
