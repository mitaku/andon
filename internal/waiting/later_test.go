package waiting

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
)

// 置いておく印 later と、行の call（ADR-0062 決めたこと 3〜6・受け入れ条件 1〜7）。
// 偽の kanban の Chief of Staff は t1（config.yaml）。担当の端末は t7。now は 2026-11-21 10:00 UTC。

const why = "急がない。手の空いたときに読めばよい"

// later は Chief of Staff（既定 t1）が印 later を書いた行と、issue.yaml の行を返す。
func (f *fake) later(id, by, value string) string {
	f.row("2026-11-21T05:00:00Z", "terminal:"+by, id, `{"later":"`+value+`"}`)
	return "later: " + value
}

func (f *fake) config(at, cos string) {
	f.log = append(f.log, `{"at":"`+at+`","by":"terminal:`+cos+`","config":{"chief_of_staff":"`+cos+`"}}`)
}

// itemOf は行を1つ返す（id か <id>#<n> で）。
func itemOf(t *testing.T, r Result, name string) Item {
	t.Helper()
	for _, it := range r.Items {
		if it.ID == name || it.Question == name {
			return it
		}
	}
	t.Fatalf("行 %s が無い: %v", name, kinds(r))
	return Item{}
}

func wantCall(t *testing.T, r Result, name string, call bool, later string) {
	t.Helper()
	it := itemOf(t, r, name)
	if it.Call != call || it.Later != later {
		t.Errorf("%s: call = %v, later = %q, want call = %v, later = %q", name, it.Call, it.Later, call, later)
	}
}

func (f *fake) decision(id string, keys ...string) {
	f.issue(id, "todo", append([]string{"who: person"}, keys...)...)
	f.questions(id, q(1, "users", "open", "2026-11-21T02:00:00Z"))
}

// 受け入れ条件 1: Chief of Staff の印のある todo のオーナーの課題の判断は call が false で later に理由が出る。
// 印の無い同じ行は true。どの行にも call がある。
func TestLaterMark(t *testing.T) {
	f := newFake(t, 4)
	f.decision("S-0001", f.later("S-0001", "t1", why))
	f.decision("S-0002")
	f.issue("S-0003", "todo", "who: person", "only_owner: ホストで焼き直す")
	r := f.read()
	wantCall(t, r, "S-0001#1", false, why)
	wantCall(t, r, "S-0002#1", true, "")
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, it := range raw.Items {
		if _, ok := it["call"].(bool); !ok {
			t.Errorf("call の無い行: %v", it)
		}
	}
	if len(raw.Items) != 3 {
		t.Fatalf("行の数 = %d, want 3", len(raw.Items))
	}
	// 当たる場合があること: 印の無い行は later を持たない
	if _, ok := raw.Items[0]["later"]; ok {
		t.Errorf("呼んでいる行（先頭）に later がある: %v", raw.Items[0])
	}
}

// 受け入れ条件 2: 印の書き手の照合（4 つの場合）。
func TestLaterTrust(t *testing.T) {
	t.Run("担当の端末が書いた印は読まない", func(t *testing.T) {
		f := newFake(t, 4)
		f.decision("S-0001", f.later("S-0001", "t7", why))
		wantCall(t, f.read(), "S-0001#1", true, "")
	})
	t.Run("Chief of Staff が替わっても書いた時点の端末で信じる", func(t *testing.T) {
		f := newFake(t, 4)
		f.write("kanban/config.yaml", "version: 3\nprefix: S\nchief_of_staff: t2\n")
		f.config("2026-11-21T00:00:00Z", "t1")
		f.decision("S-0001", f.later("S-0001", "t1", why))
		f.config("2026-11-21T06:00:00Z", "t2")
		wantCall(t, f.read(), "S-0001#1", false, why)
	})
	t.Run("担当の端末が値を書き換えると読まない", func(t *testing.T) {
		f := newFake(t, 4)
		f.config("2026-11-21T00:00:00Z", "t1")
		f.later("S-0001", "t1", why)
		f.row("2026-11-21T06:00:00Z", "terminal:t7", "S-0001", `{"later":"担当が書いた"}`)
		f.decision("S-0001", "later: 担当が書いた")
		wantCall(t, f.read(), "S-0001#1", true, "")
	})
	t.Run("Chief of Staff の行の無い kanban は今の端末と比べる", func(t *testing.T) {
		f := newFake(t, 4)
		f.decision("S-0001", f.later("S-0001", "t1", why))
		f.decision("S-0002", f.later("S-0002", "t2", why))
		r := f.read()
		wantCall(t, r, "S-0001#1", false, why)
		wantCall(t, r, "S-0002#1", true, "")
	})
}

// 受け入れ条件 3: 印のある課題でも、マージの依頼・doing か review の担当の課題の問い・Chief Engineer の課題の問いは呼ぶ。
func TestLaterNotPlaceable(t *testing.T) {
	f := newFake(t, 4)
	f.issue("S-0001", "review", "profile: claude", f.later("S-0001", "t1", why))
	f.report("S-0001", head)
	f.questions("S-0001", q(1, "line", "open", "2026-11-21T04:00:00Z"))
	f.gateAsk("S-0001", 1, head)
	f.row("2026-11-21T03:00:00Z", "terminal:t7", "S-0001", `{"status":"review"}`)
	f.issue("S-0002", "doing", "profile: claude", f.later("S-0002", "t1", why))
	f.questions("S-0002", q(1, "users", "open", "2026-11-21T02:00:00Z"))
	f.issue("S-0003", "doing", "profile: chief-engineer", f.later("S-0003", "t1", why))
	f.questions("S-0003", q(1, "business", "open", "2026-11-21T02:00:00Z"))
	r := f.read()
	want(t, r, "S-0001#1=merge", "S-0002#1=decision", "S-0003#1=decision")
	wantCall(t, r, "S-0001#1", true, "")
	wantCall(t, r, "S-0002#1", true, "")
	wantCall(t, r, "S-0003#1", true, "")
}

// 受け入れ条件 4: later_until が今日か過去なら呼び、明日なら置いておく。担当の端末が書いた日付は来たものとして読む。
func TestLaterUntil(t *testing.T) {
	until := func(f *fake, id, by, day string) string {
		f.row("2026-11-21T05:30:00Z", "terminal:"+by, id, `{"later_until":"`+day+`"}`)
		return "later_until: \"" + day + "\""
	}
	f := newFake(t, 4)
	f.decision("S-0001", f.later("S-0001", "t1", why), until(f, "S-0001", "t1", "2026-11-21"))
	f.decision("S-0002", f.later("S-0002", "t1", why), until(f, "S-0002", "t1", "2026-11-20"))
	f.decision("S-0003", f.later("S-0003", "t1", why), until(f, "S-0003", "t1", "2026-11-22"))
	f.decision("S-0004", f.later("S-0004", "t1", why), until(f, "S-0004", "t7", "2026-11-22"))
	f.decision("S-0005", f.later("S-0005", "t1", why), until(f, "S-0005", "t1", "2026-11-2x"))
	r := f.read()
	wantCall(t, r, "S-0001#1", true, why)
	wantCall(t, r, "S-0002#1", true, why)
	wantCall(t, r, "S-0003#1", false, why)
	wantCall(t, r, "S-0004#1", true, why)
	wantCall(t, r, "S-0005#1", true, why)
	if u := itemOf(t, r, "S-0003#1").LaterUntil; u != "2026-11-22" {
		t.Errorf("later_until = %q, want 2026-11-22", u)
	}
	// 担当の端末が later_until を消した: 最後に書いたのがその時点の Chief of Staff でないので、日が来たものとして読む
	f.decision("S-0006", f.later("S-0006", "t1", why))
	until(f, "S-0006", "t1", "2026-11-22")
	f.row("2026-11-21T06:00:00Z", "terminal:t7", "S-0006", `{"later_until":null}`)
	// Chief of Staff が消したなら日付は無い（置いたまま）
	f.decision("S-0007", f.later("S-0007", "t1", why))
	until(f, "S-0007", "t1", "2026-11-22")
	f.row("2026-11-21T06:00:00Z", "terminal:t1", "S-0007", `{"later_until":null}`)
	// 文字列でない値（kanban を通さずに書いた issue.yaml。引用符の無い日付）は、形の違う値として日が来たものと読む
	f.decision("S-0008", f.later("S-0008", "t1", why), "later_until: 2026-11-30")
	// 末尾に空白のある日付は形が違う（空白を落として未来の日付と読まない）
	f.decision("S-0009", f.later("S-0009", "t1", why), until(f, "S-0009", "t1", "2099-12-31 "))
	r = f.read()
	wantCall(t, r, "S-0009#1", true, why)
	wantCall(t, r, "S-0006#1", true, why)
	wantCall(t, r, "S-0007#1", false, why)
	wantCall(t, r, "S-0008#1", true, why)
	// 日は config.yaml の timezone で読む
	f.write("kanban/config.yaml", "version: 3\nprefix: S\nchief_of_staff: t1\ntimezone: Pacific/Kiritimati\n")
	// キリバスは UTC+14 で、2026-11-22 00:00 を過ぎている
	r = f.read()
	wantCall(t, r, "S-0003#1", true, why)
}

// 受け入れ条件 5: 後ろで待つ課題が置いておけなければ呼ぶ。止め（hold）で後ろに入った review の課題は、印があっても呼ぶ。
func TestLaterHolds(t *testing.T) {
	f := newFake(t, 4)
	f.decision("S-0001", f.later("S-0001", "t1", why))
	f.issue("S-0002", "todo", "profile: claude", "after: [S-0001]")
	f.decision("S-0003", f.later("S-0003", "t1", why))
	f.issue("S-0004", "todo", "profile: claude", "after: [S-0003]", f.later("S-0004", "t1", "続きの手順も急がない"))
	f.decision("S-0005", f.later("S-0005", "t1", why))
	f.issue("S-0006", "review", "profile: claude", "merge: person", "hold: S-0005", f.later("S-0006", "t1", why))
	f.row("2026-11-21T05:10:00Z", "terminal:t1", "S-0006", `{"hold":"S-0005"}`)
	r := f.read()
	wantCall(t, r, "S-0001#1", true, why)
	wantCall(t, r, "S-0003#1", false, why)
	wantCall(t, r, "S-0005#1", true, why)
	if h := itemOf(t, r, "S-0005#1").Holds; !slices.Equal(h, []string{"S-0006"}) {
		t.Errorf("S-0005 の holds = %v, want [S-0006]", h)
	}
}

// 受け入れ条件 6: 手作業の行は印 later が無ければ呼び、あれば置いておく。問いも印 only_owner も無いオーナーの課題の行は、
// 印 later があっても呼ぶ。
func TestLaterTask(t *testing.T) {
	f := newFake(t, 4)
	f.issue("S-0001", "todo", "who: person", "only_owner: ホストで焼き直す")
	f.issue("S-0002", "todo", "who: person", "only_owner: ホストで焼き直す", f.later("S-0002", "t1", why))
	f.issue("S-0003", "todo", "who: person", f.later("S-0003", "t1", why))
	// doing のオーナーの課題も置いておける（オーナーの課題か、todo・queued の課題）
	f.issue("S-0004", "doing", "who: person", "only_owner: ホストで焼き直す", f.later("S-0004", "t1", why))
	r := f.read()
	want(t, r, "S-0001=task", "S-0002=task", "S-0003=decision", "S-0004=task")
	wantCall(t, r, "S-0004", false, why)
	wantCall(t, r, "S-0001", true, "")
	wantCall(t, r, "S-0002", false, why)
	if it := itemOf(t, r, "S-0003"); !it.Call {
		t.Errorf("問いも印 only_owner も無いオーナーの課題の行: call = false, want true")
	}
}

// 受け入れ条件 7: 呼んでいる行が先、置いてある行が後（古い順）。枠は印で変わらない。出力は schema を通り version は 1。
// call が文字列の出力は schema で落ち、call の無い行は通る。
func TestLaterOrderAndSchema(t *testing.T) {
	f := newFake(t, 2)
	// 置いてある行（古い S-0001 と新しい S-0002）と、呼んでいる行（新しい S-0003 と、後ろで待つ S-0005 のある S-0004）
	f.issue("S-0001", "todo", "who: person", f.later("S-0001", "t1", why))
	f.questions("S-0001", q(1, "users", "open", "2026-11-20T01:00:00Z"))
	f.issue("S-0002", "todo", "who: person", f.later("S-0002", "t1", why))
	f.questions("S-0002", q(1, "users", "open", "2026-11-21T03:00:00Z"))
	f.decision("S-0003")
	f.issue("S-0004", "todo", "who: person")
	f.questions("S-0004", q(1, "users", "open", "2026-11-21T08:00:00Z"))
	f.issue("S-0005", "todo", "profile: claude", "after: [S-0004]")
	// 担当の課題の問い（枠に数える）
	f.issue("S-0006", "doing", "profile: claude")
	f.questions("S-0006", q(1, "users", "open", "2026-11-21T04:00:00Z"))
	r := f.read()
	var order []string
	for _, it := range r.Items {
		order = append(order, it.Question)
	}
	if !slices.Equal(order, []string{"S-0004#1", "S-0003#1", "S-0006#1", "S-0001#1", "S-0002#1"}) {
		t.Fatalf("並び = %v, want 呼んでいる S-0004（後ろ 1 件）, S-0003, S-0006 のあとに置いてある S-0001（古い）, S-0002", order)
	}
	slots := r.Slots
	if r.Version != 1 {
		t.Errorf("version = %d, want 1", r.Version)
	}
	// 枠は印の有無で変わらない: 担当の課題に印を書いても（doing なので置いておけない）、オーナーの課題の印を消しても同じ
	f.issue("S-0006", "doing", "profile: claude", f.later("S-0006", "t1", why))
	f.issue("S-0001", "todo", "who: person")
	if r2 := f.read(); r2.Slots != slots {
		t.Errorf("印を変えたあとの slots = %+v, want %+v", r2.Slots, slots)
	}
	b, err := contract.Encode(contract.SchemaWaiting, r)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.SchemaWaiting.Check(b); err != nil {
		t.Fatalf("出力が schema を通らない: %v", err)
	}
	// 行を書き換えて schema に照らす
	mutate := func(edit func(it map[string]any)) []byte {
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		for _, it := range v["items"].([]any) {
			edit(it.(map[string]any))
		}
		out, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	bad := mutate(func(it map[string]any) { it["call"] = "false" })
	if !bytes.Contains(bad, []byte(`"call":"false"`)) {
		t.Fatalf("call を書き換えられない（当たる場合が無い）: %s", bad)
	}
	if err := contract.SchemaWaiting.Check(bad); err == nil {
		t.Errorf("call が文字列の出力が schema を通った")
	}
	none := mutate(func(it map[string]any) { delete(it, "call") })
	if bytes.Contains(none, []byte(`"call"`)) {
		t.Fatalf("call を外せない: %s", none)
	}
	if err := contract.SchemaWaiting.Check(none); err != nil {
		t.Errorf("call の無い行が schema を通らない: %v", err)
	}
}

// 受け入れ条件 8 の Go の側: 人が読む出力は、呼んでいる行と置いてある行を分け、置いてある行に印の理由を出す。
func TestLaterText(t *testing.T) {
	f := newFake(t, 4)
	f.decision("S-0001", f.later("S-0001", "t1", why))
	f.decision("S-0002")
	text := Text(f.read())
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 5 || lines[1] != "呼んでいるもの 1 件（行燈を灯す）" || !strings.HasPrefix(lines[2], "判断 S-0002#1") ||
		!strings.HasPrefix(lines[3], "置いてあるもの 1 件") || !strings.HasPrefix(lines[4], "判断 S-0001#1") ||
		!strings.HasSuffix(lines[4], "。置いた理由: "+why) {
		t.Errorf("人が読む出力:\n%s", text)
	}
	f.decision("S-0002", f.later("S-0002", "t1", why))
	if text := Text(f.read()); !strings.Contains(text, "\n呼んでいるものは無い\n置いてあるもの 2 件") {
		t.Errorf("呼んでいる行の無い出力:\n%s", text)
	}
}
