package triage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	ledgercli "andon/internal/ledger"
)

// Chief of Staff の判定（ADR-0030 決めたこと 5 の 3・6）。判断の閉じと、kanban の課題を閉じる判定は、
// Chief of Staff が1件ずつ「閉じてよい」（ok）か「覆す」（overturn）かを andon triage verdict で書き、
// トリアージの課題の library/verdicts.json に置く。判定は key と、書いた時の担当の閉じ方と理由に結びつく。
const (
	verdictOK       = "ok"
	verdictOverturn = "overturn"
	verdictFileName = "verdicts.json"
)

type verdict struct {
	Key     string `json:"key"`
	Verdict string `json:"verdict"`
	Closing string `json:"closing"`
	Reason  string `json:"reason"`
	By      string `json:"by"`
	At      string `json:"at"`
}

type verdictFile struct {
	Verdicts []verdict `json:"verdicts"`
}

func verdictPath(l *ledger, id string) string {
	return filepath.Join(l.dir, "issues", id, "library", verdictFileName)
}

func readVerdicts(path string) (map[string]verdict, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]verdict{}, nil
	}
	var f verdictFile
	if err != nil || json.Unmarshal(b, &f) != nil {
		return nil, fail("library/%s を読めない", verdictFileName)
	}
	got := map[string]verdict{}
	for _, v := range f.Verdicts {
		got[v.Key] = v
	}
	return got, nil
}

// matches は判定が今の担当の判定（閉じ方と理由）に対して書かれたものか。担当が書き換えたら無効。
func (v verdict) matches(it item) bool {
	return v.Closing == it.closing() && v.Reason == it.str("reason")
}

// writer は判定を書いてよい端末（config.yaml の chief_of_staff）の by。chief_of_staff が無ければ空で、どの判定も通さない。
func writer(l *ledger) string {
	if cos := ledgercli.At(l.root).ChiefOfStaff(); cos != "" {
		return "terminal:" + cos
	}
	return ""
}

// verdictState は今の判定の状態（--dry-run の表に出す）: ok・overturn・none（無い）・stale（理由か閉じ方が書き換わった）・
// foreign（書き手が今の Chief of Staff の端末でない。by の無いものも）。by は writer の値。
func verdictState(vs map[string]verdict, by string, it item) string {
	v, ok := vs[it.key()]
	switch {
	case !ok:
		return "none"
	case by == "" || v.By != by:
		return "foreign"
	case !v.matches(it):
		return "stale"
	default:
		return v.Verdict
	}
}

func reviewTable(items []item, vs map[string]verdict, by string) []any {
	got := []any{}
	for _, it := range items {
		if !it.needsVerdict() {
			continue
		}
		got = append(got, obj{
			"key": it.key(), "kind": it.keyPart(2), "from": it.keyPart(1), "text": it["text"],
			"closing": it.closing(), "reason": it["reason"], "verdict": verdictState(vs, by, it),
		})
	}
	return got
}

// 足りなければ kanban に手を入れる前に断る。通れば items に判定（verdict）と、覆した記録（overturned）を書く。
// 判定は、書き手（by）が今の Chief of Staff の端末のものだけを数える。
func checkVerdicts(items []item, vs map[string]verdict, by string) error {
	var missing []string
	for _, it := range items {
		if !it.needsVerdict() {
			continue
		}
		switch verdictState(vs, by, it) {
		case verdictOK:
		case verdictOverturn:
			return fail("Chief of Staff が覆した close が残っている: %s（担当に戻し、行き先を書き直させる）", it.key())
		default:
			missing = append(missing, it.key())
		}
	}
	if len(missing) > 0 {
		return fail("Chief of Staff の判定が %d 件足りない（無い・判定のあとで閉じ方か理由が変わった・書き手が今の Chief of Staff の端末でない。andon triage verdict <id> <key> ok|overturn で書く）: %s",
			len(missing), missing[0])
	}
	for _, it := range items {
		v, ok := vs[it.key()]
		switch {
		case !ok || v.By != by:
		case it.needsVerdict():
			it["verdict"] = obj{"verdict": v.Verdict, "by": v.By, "at": v.At}
		case v.Verdict == verdictOverturn:
			// 覆した close を、担当が行き先を書き直したもの。覆したことを記録に残す（決めたこと 5 の 4）。
			it["overturned"] = obj{"closing": v.Closing, "reason": v.Reason, "by": v.By, "at": v.At}
		}
	}
	return nil
}

// runVerdict は Chief of Staff の端末でだけ書く。
func runVerdict(l *ledger, id, key, value string, now time.Time, stdout io.Writer) error {
	if value != verdictOK && value != verdictOverturn {
		return fail("判定は ok（閉じてよい）か overturn（覆す）")
	}
	cos := ledgercli.At(l.root).ChiefOfStaff()
	if me := os.Getenv("HC_TERMINAL_ID"); cos == "" || me != cos {
		return fail("判定を書くのは Chief of Staff の端末だけ（config.yaml の chief_of_staff: %q・HC_TERMINAL_ID: %q）", cos, me)
	}
	b, err := os.ReadFile(filepath.Join(l.dir, "issues", id, "library", "decisions.json"))
	if err != nil {
		return fail("decisions.json が無い: %s", id)
	}
	_, items, ok := parseDecisions(b, l.slug)
	if !ok {
		return fail("decisions.json が不正（andon triage apply %s --dry-run で理由を見る）", id)
	}
	if err := checkClosings(items); err != nil {
		return err
	}
	i := slices.IndexFunc(items, func(it item) bool { return it.key() == key })
	if i < 0 {
		return fail("decisions.json に無い key: %s", key)
	}
	it := items[i]
	if !it.needsVerdict() {
		return fail("判定の要らない項目: %s（判定を書くのは判断の閉じと、課題を閉じる判定だけ）", key)
	}
	path := verdictPath(l, id)
	vs, err := readVerdicts(path)
	if err != nil {
		return err
	}
	vs[key] = verdict{Key: key, Verdict: value, Closing: it.closing(), Reason: it.str("reason"),
		By: "terminal:" + cos, At: now.Format(time.RFC3339)}
	var f verdictFile
	for _, v := range vs {
		f.Verdicts = append(f.Verdicts, v)
	}
	slices.SortFunc(f.Verdicts, func(a, b verdict) int { return strings.Compare(a.Key, b.Key) })
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, marshalJSON(f), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	word := "閉じてよい"
	if value == verdictOverturn {
		word = "覆す"
	}
	_, err = fmt.Fprintf(stdout, "%s: %s（閉じ方 %s・理由 %s）\n", key, word, it.closing(), it.str("reason"))
	return err
}
