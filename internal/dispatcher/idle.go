package dispatcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/delegate"
	"andon/internal/ledger"
	"andon/internal/retro"
	"andon/internal/triage"
	"andon/internal/waiting"
)

// idleEvery は空きを見る間の既定（分。project.json の chief_of_staff.idle_minutes。0 は見ない。ADR-0047 決めたこと 4・16）。
const idleEvery = 15

// triageRoutine はトリアージのルーチンの名前（kanban/routines/triage.yaml。issue.yaml の routine）。
const triageRoutine = "triage"

// Idle は空きを見た前の回の結果と、知らせたもの（控えの idle。ADR-0047 決めたこと 4・11・12）。
type Idle struct {
	At    string   `json:"at,omitempty"`    // 前の回に見た時刻（RFC3339。空なら前の回の結果が無い）
	Free  bool     `json:"free,omitempty"`  // 前の回に枠が1つ以上空いていた
	Ready []string `json:"ready,omitempty"` // 前の回に起動できる課題の一覧に出ていた id（オーナーの課題・レビュアーの子課題を除く）
	Told  []string `json:"told,omitempty"`  // 通知 idle で知らせた id（一覧から消えたら外す。起動し直しと留守では消さない）
	Lint  string   `json:"lint,omitempty"`  // 知らせた kanban lint の problem の並びの印（problem が 0 件に戻ったら消す）
}

// 検査が差し替える口（偽の kanban では枠もトリアージの未処理も数えられないため）。
var (
	// slotsOf は空いた枠の数（andon waiting と同じ式。ADR-0042 決めたこと 9）
	slotsOf = func(root string, now time.Time) (int, error) {
		res, _, err := waiting.Read(root, now)
		return res.Slots.Free, err
	}
	// unsortedOf は仕分けていない項目の数（ADR-0047 決めたこと 7）
	unsortedOf = triage.Unsorted
)

// clock は今の時刻（検査は Dispatcher.Now で決める）。
func (c *Dispatcher) clock() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

// idle は空きを見る段（1巡の 3a。振り返りの件数のきっかけのあと、kanban tick の前。ADR-0047 決めたこと 3〜12・17。
// dispatcher.md §2.13）。idle_minutes ごとに枠を数え、前の回も今の回も枠が1つ以上空いていたら、撃てるルーチン（トリアージ・
// 振り返り）を空いた枠の数まで撃ち、前の回にも今の回にも起動できる課題の一覧に出ている課題と kanban lint の結果を通知 idle で
// Chief of Staff に知らせる。初めて見る kanban・dispatcher が起動し直した最初の回・前の回から間が2倍を超えて空いた回は、
// 控えるだけで何もしない。idle_hours を持つルーチン（決めたこと 9）も撃つ。retroFired は同じ巡で件数のきっかけが振り返りを撃ったか。失敗はログに1行で、巡は止めない。
func (p *project) idle(retroFired bool) {
	if p.away {
		p.forgetIdle() // 留守の間は足した課題を起動する者がいない（決めたこと 17）
		return
	}
	every, err := idleMinutesOf(p.c.WS, p.l.Slug)
	if err != nil {
		p.idleOnce("set", fmt.Sprintf("project.json の chief_of_staff.idle_minutes を読めない（既定の %d 分で見る）: %v", idleEvery, err))
	} else {
		p.idleOnce("set", "")
	}
	if every == 0 {
		p.forgetIdle()
		return
	}
	now := p.c.clock()
	var prev Idle
	if p.st.Idle != nil {
		prev = *p.st.Idle
	}
	seen := p.c.Once || p.c.idleSeen[p.l.Slug]
	prevAt, atErr := time.Parse(time.RFC3339, prev.At)
	known := prev.At != "" && atErr == nil
	interval := time.Duration(every) * time.Minute
	if known && seen && now.Sub(prevAt) < interval {
		return // まだ見る時刻でない
	}
	free, err := slotsOf(p.l.Root, now)
	if err != nil {
		p.idleOnce("slots", fmt.Sprintf("枠を数えられない（この回は空きを見ない）: %v", err))
		p.forgetIdle()
		return
	}
	p.idleOnce("slots", "")
	if p.c.idleSeen == nil {
		p.c.idleSeen = map[string]bool{}
	}
	p.c.idleSeen[p.l.Slug] = true
	ready := p.readyNow()
	cur := &Idle{At: now.Format(time.RFC3339), Free: free > 0, Ready: ready, Told: keepIn(prev.Told, ready), Lint: prev.Lint}
	p.st.Idle = cur
	if !known || !seen || p.first || now.Sub(prevAt) > 2*interval || !prev.Free || free == 0 {
		return // 控えるだけ（決めたこと 4 の細部）
	}
	fired := p.fireIdle(free, retroFired, now)
	var unstarted []string
	for _, id := range ready {
		if slices.Contains(prev.Ready, id) && !slices.Contains(cur.Told, id) {
			unstarted = append(unstarted, id)
		}
	}
	cur.Told = append(cur.Told, unstarted...)
	lint := p.idleLint(cur)
	if len(unstarted) == 0 && len(fired) == 0 && lint == 0 {
		return
	}
	p.pushIdle(free, unstarted, fired, lint)
}

// forgetIdle は前の回の結果を消す（知らせた id と lint の印は残す。決めたこと 4・11）。
func (p *project) forgetIdle() {
	if p.st.Idle == nil {
		return
	}
	i := *p.st.Idle
	i.At, i.Free, i.Ready = "", false, nil
	if len(i.Told) == 0 && i.Lint == "" {
		p.st.Idle = nil
		return
	}
	p.st.Idle = &i
}

// readyNow は今の起動できる課題の id。前の巡が控えた kanban ready（オーナーの課題とレビュアーの子課題を除いたもの。readReady）を、
// この巡に読んだ一覧で、もう起動された（doing・review・done）か無くなった課題を除いて読む。
func (p *project) readyNow() []string {
	if p.st.Ready == nil {
		return nil
	}
	status := map[string]string{}
	for _, t := range p.tasks {
		status[t.Str("id")] = t.Str("status")
	}
	var out []string
	for _, id := range *p.st.Ready {
		switch status[id] {
		case "", "doing", "review", "done":
			continue
		}
		out = append(out, id)
	}
	return out
}

// keepIn は ids のうち in にあるものを、ids の順で返す。
func keepIn(ids, in []string) []string {
	var out []string
	for _, id := range ids {
		if slices.Contains(in, id) {
			out = append(out, id)
		}
	}
	return out
}

// fireIdle は空いたときのルーチンを、トリアージ・振り返り・idle_hours を持つルーチン（名前の順）の順に、空いた枠の数まで撃つ
// （決めたこと 5・7〜10）。
// ルーチンが無い・読めない・paused のとき、そのルーチンの課題が片付かずに残っているときは、数えも撃ちもしない。
// 起動できる課題の数は引かない（決めたこと 10）。同じ巡で件数のきっかけが振り返りを撃っていれば、その 1 つも
// 空いた枠を使うので、撃てる数から引く。撃ったルーチンの名前を返す。
func (p *project) fireIdle(free int, retroFired bool, now time.Time) []string {
	var fired []string
	budget := free
	if retroFired {
		budget--
	}
	fire := func(name, why string) {
		out, err := p.l.Run("", "routine", "run", name)
		if err != nil {
			p.idleOnce("run-"+name, fmt.Sprintf("kanban routine run %s が通らない（巡は続ける）: %v", name, err))
			return
		}
		p.idleOnce("run-"+name, "")
		p.logf("枠が %d つ空いたままなので、%s のルーチンを撃った（%s）: %s", free, name, why, strings.TrimSpace(out))
		fired = append(fired, name)
	}
	if budget > 0 && routineOn(p.l.Dir, triageRoutine) && !p.triageOpen() {
		switch n, err := unsortedOf(p.l.Root, now); {
		case err != nil:
			p.idleOnce("triage", fmt.Sprintf("仕分けていない項目を数えられない（空きのトリアージを撃たない）: %v", err))
		default:
			p.idleOnce("triage", "")
			if n >= triage.DefaultLimit {
				fire(triageRoutine, fmt.Sprintf("仕分けていない項目 %d 件。線 %d 件", n, triage.DefaultLimit))
			}
		}
	}
	if len(fired) < budget && !retroFired && routineOn(p.l.Dir, retroRoutine) && !p.routineOpen(retroRoutine) {
		every, err := retroEveryOf(p.c.WS, p.l.Slug)
		if err != nil {
			p.retroOnce("set", fmt.Sprintf("project.json の chief_of_staff.retro_every を読めない（既定の %d 件で数える）: %v", retroEvery, err))
		}
		line := (every + 1) / 2
		if n := retro.FreshOwn(p.l.Root, p.tasks, p.rows, now); n >= line {
			fire(retroRoutine, fmt.Sprintf("ルーチンの課題を除いた片付いた担当の課題 %d 件。線 %d 件", n, line))
		}
	}
	// idle_hours を持つルーチン（名前の順。決めたこと 9）。トリアージと振り返りは上の線で撃つので、ここでは見ない
	for _, name := range routineNames(p.l.Dir) {
		if name == triageRoutine || name == retroRoutine {
			continue
		}
		hours, ok := p.idleHoursOf(name)
		if !ok || len(fired) >= budget || !routineOn(p.l.Dir, name) || p.routineOpen(name) {
			continue
		}
		last, found := lastFired(p.rows, name)
		switch {
		case !found:
			fire(name, fmt.Sprintf("まだ課題を足していない。線 %d 時間", hours))
		case now.Sub(last).Hours() >= float64(hours): // time.Duration に直すと大きい値であふれるので、時間で比べる
			fire(name, fmt.Sprintf("最後に課題を足してから %d 時間。線 %d 時間", int(now.Sub(last).Hours()), hours))
		}
	}
	return fired
}

// routineNames は kanban のルーチンの名前（routines/*.yaml。名前の順）。読めなければ無い。
func routineNames(dir string) []string {
	ents, err := os.ReadDir(filepath.Join(dir, "routines"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if name, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && !e.IsDir() {
			out = append(out, name)
		}
	}
	return out
}

// idleHoursOf はルーチン name の idle_hours（Andon が読む欄。kanban の CLI は読まない。決めたこと 9）。欄が無い・ファイルを
// 読めないときは false。値が 1 以上の整数でなければ欄が無いものとして false を返し、ログに 1 行を書く（同じ理由が続く間は 1 行）。
func (p *project) idleHoursOf(name string) (int, bool) {
	b, err := os.ReadFile(filepath.Join(p.l.Dir, "routines", name+".yaml"))
	if err != nil {
		return 0, false
	}
	var r map[string]any
	if yaml.Unmarshal(b, &r) != nil {
		return 0, false
	}
	v, ok := r["idle_hours"]
	if !ok {
		p.idleOnce("hours-"+name, "")
		return 0, false
	}
	if n, isInt := v.(int); isInt && n >= 1 {
		p.idleOnce("hours-"+name, "")
		return n, true
	}
	p.idleOnce("hours-"+name, fmt.Sprintf("ルーチン %s の idle_hours が 1 以上の整数でない（%v）。空きでは撃たない", name, v))
	return 0, false
}

// lastFired はルーチン name が最後に課題を足した時刻（log.jsonl の fired の行。撃ち損ねの行〈skipped〉は数えない。
// kanban の state/routines.json は撃ち損ねと初めて見たときにも書かれるので読まない。決めたこと 9）。
func lastFired(rows []ledger.Row, name string) (time.Time, bool) {
	var last time.Time
	found := false
	for _, r := range rows {
		if r.Routine != name || r.Fired == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, r.At)
		if err != nil {
			continue
		}
		if !found || at.After(last) {
			last, found = at, true
		}
	}
	return last, found
}

// routineOpen はルーチン name が足した課題が片付かずに残っているか。
func (p *project) routineOpen(name string) bool {
	for _, t := range p.tasks {
		if t.Str("routine") == name && t.Str("status") != "done" {
			return true
		}
	}
	return false
}

// triageOpen は片付いていないトリアージの課題があるか（同時に1つ。ADR-0047 決めたこと 5 の細部）。ルーチンが足したものと、
// Chief of Staff が臨時に足したもの（タイトルが「トリアージ」で始まり、repo を持たない）を数える。
func (p *project) triageOpen() bool {
	if p.routineOpen(triageRoutine) {
		return true
	}
	for _, t := range p.tasks {
		if t.Str("status") != "done" && t.Str("repo") == "" && strings.HasPrefix(t.Str("title"), "トリアージ") {
			return true
		}
	}
	return false
}

// idleLint は kanban lint --json を打ち、problem が1件以上あって、前に知らせた結果と違えば件数を返す（決めたこと 12）。
// 知らせた結果の印を cur に書き、problem が 0 件なら印を消す。打てない・読めないときはログに1行で 0。
func (p *project) idleLint(cur *Idle) int {
	out, _, err := run(p.l.Root, append([]string{"KANBAN_ROOT=" + p.l.Root}, p.l.Env...), "", ledger.CLI, "lint", "--json")
	var v struct {
		OK       *bool    `json:"ok"`
		Problems []string `json:"problems"`
	}
	if jerr := json.Unmarshal([]byte(out), &v); jerr != nil || v.OK == nil {
		p.idleOnce("lint", fmt.Sprintf("kanban lint の結果を読めない（巡は続ける）: %v", errors.Join(err, jerr)))
		return 0
	}
	p.idleOnce("lint", "")
	if len(v.Problems) == 0 {
		cur.Lint = ""
		return 0
	}
	sum := sha256.Sum256([]byte(strings.Join(v.Problems, "\n")))
	mark := hex.EncodeToString(sum[:])
	if mark == cur.Lint {
		return 0
	}
	cur.Lint = mark
	return len(v.Problems)
}

// pushIdle は通知 idle を積む。まだ届いていない idle があれば、その unstarted と fired を新しいほうに足し、
// この回に lint の件数が無ければ前の件数を残す（idle は issue を持たないので、同じ出来事として置き換わる。
// lint の印はもう「知らせた」と控えてあるので、ここで落とすと problem の並びが変わるまで知らせない。決めたこと 11・12 の細部）。
func (p *project) pushIdle(free int, unstarted, fired []string, lint int) {
	for _, n := range p.st.Pending {
		if n.Kind != "idle" {
			continue
		}
		unstarted = mergeIDs(idList(n.Detail["unstarted"]), unstarted)
		fired = mergeIDs(idList(n.Detail["fired"]), fired)
		if lint == 0 {
			lint = countOf(n.Detail["lint"])
		}
	}
	p.notify("", "idle", map[string]any{"free": free, "unstarted": nonNil(unstarted), "fired": nonNil(fired), "lint": lint})
}

// countOf は detail の数（積んだプロセスの中では int、控えから読んだあとは float64）。数でなければ 0。
func countOf(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

// mergeIDs は old のあとに、old に無い add を足す。
func mergeIDs(old, add []string) []string {
	out := append([]string{}, old...)
	for _, id := range add {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// idleOnce は空きの段の失敗をログに1行だけ書く（kanban と種類ごとに、同じ理由が続く間は書かない）。reason が空なら覚えを消す。
func (p *project) idleOnce(kind, reason string) {
	key := p.l.Slug + "\x00" + kind
	if p.c.idleLogged[key] == reason {
		return
	}
	if p.c.idleLogged == nil {
		p.c.idleLogged = map[string]string{}
	}
	p.c.idleLogged[key] = reason
	if reason != "" {
		p.logf("%s", reason)
	}
}

// routineOn は kanban にルーチン name があり、止める印（paused: true）が無いか。読めない・paused が真偽でないファイルは、
// kanban の CLI も撃たない（kanban lint の problem）ので無いと同じに扱う。
func routineOn(dir, name string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "routines", name+".yaml"))
	if err != nil {
		return false
	}
	var r map[string]any
	if yaml.Unmarshal(b, &r) != nil || r == nil {
		return false
	}
	v, ok := r["paused"]
	if !ok {
		return true
	}
	paused, isBool := v.(bool)
	return isBool && !paused
}

// idleMinutesOf は空きを見る間を、セルの repo の HEAD の project.json の chief_of_staff.idle_minutes から読む（author 照合は
// delegate.ReadProject）。project.json か値が無ければ既定。信じられない・形が違うときは既定と、その理由を返す。
func idleMinutesOf(ws, slug string) (int, error) {
	pj, err := delegate.ReadProject(ws, slug)
	switch {
	case errors.Is(err, delegate.ErrAbsent):
		return idleEvery, nil
	case err != nil:
		return idleEvery, err
	case pj.ChiefOfStaff == nil || pj.ChiefOfStaff.IdleMinutes == nil:
		return idleEvery, nil
	}
	return *pj.ChiefOfStaff.IdleMinutes, nil
}
