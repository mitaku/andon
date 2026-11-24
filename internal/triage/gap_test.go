package triage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type gapIssue struct {
	id, title, status, profile string
	reportMD, reportJSON       string
}

// CLI はシェルの組み込みだけで書く（PATH を空にして、hc と git をセルの本物から読まない）。
func gapLedger(t *testing.T, issues []gapIssue) *ledger {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "nobin"))
	write := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(dir, "demo")
	kb := filepath.Join(root, "kanban")
	var list []map[string]any
	for i, it := range issues {
		one := map[string]any{"id": it.id, "title": it.title, "status": it.status, "repo": "example.invalid/me/app",
			"created": time.Date(2026, 11, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05Z")}
		if it.profile != "" {
			one["profile"] = it.profile
		}
		list = append(list, one)
		b, _ := json.Marshal(map[string]any{"issue": one})
		write(filepath.Join(dir, "show", it.id+".json"), string(b)+"\n", 0o644)
		if it.reportMD != "" {
			write(filepath.Join(kb, "issues", it.id, "report.md"), it.reportMD, 0o644)
		}
		if it.reportJSON != "" {
			write(filepath.Join(kb, "issues", it.id, "library", "report.json"), it.reportJSON, 0o644)
		}
	}
	b, _ := json.Marshal(list)
	write(filepath.Join(dir, "list.json"), string(b)+"\n", 0o644)
	cli := filepath.Join(dir, "cli")
	write(cli, `#!/bin/sh
case $1 in
list) f='`+filepath.Join(dir, "list.json")+`' ;;
show) f='`+filepath.Join(dir, "show")+`'/$2.json ;;
*) exit 9 ;;
esac
[ -f "$f" ] || exit 1
while IFS= read -r ln; do printf '%s\n' "$ln"; done < "$f"
`, 0o755)
	return &ledger{cli: cli, root: root, slug: "demo", dir: kb, now: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)}
}

const gapCommit = "0123456789abcdef0123456789abcdef01234567"

func gapReport(gaps string) string {
	return `{"commit": "` + gapCommit + `", "gaps": [` + gaps + `]}`
}

// 申告を持つ報告から、andon triage collect が種類 gap の項目を 1 件ずつ出す（ADR-0061 受け入れ条件 3）。
// 項目の文に種類と先と本文があり、同じ先を名指した別の課題の申告（課題の id と種類と本文）が添えられる。
// Chief Engineer の課題の申告も項目になる。gaps の無い報告からは出ず、人向けの報告の ## Gaps の行はどの項目にもならない。
func TestCollectPicksGapsFromReportJSON(t *testing.T) {
	l := gapLedger(t, []gapIssue{
		{id: "T-0001", title: "担当の課題", status: "done",
			reportMD: "## Report\n本文\n## Next\n- 次の一手\n" +
				"## Gaps\n- wrong AGENTS.md: 一覧が 1 つ違う。直し方: 合わせる\n- 見送りの語を含む申告の行\nFor AGENTS.md: 申告の節の行\n- missing package/skills/worker/SKILL.md: 置き場が書いていない\n" +
				"## Remember\n- 学び\n",
			reportJSON: gapReport(`{"kind": "wrong", "where": "AGENTS.md", "text": "一覧が 1 つ違う", "proposal": "合わせる"},
				{"kind": "missing", "where": "package/skills/worker/SKILL.md", "text": "置き場が書いていない"}`)},
		{id: "T-0002", title: "仕様: 置き場", status: "review", profile: "chief-engineer",
			reportMD:   "## Report\n検討した\n",
			reportJSON: gapReport(`{"kind": "hard_to_find", "where": "./package/skills/worker/SKILL.md", "text": "遅れて見つけた"}`)},
		{id: "T-0003", title: "動いている課題", status: "doing",
			reportMD: "## Report\n作業中\n",
			reportJSON: gapReport(`{"kind": "missing", "where": "package/skills/worker/SKILL.md", "text": "作業中に気づいた"},
				{"kind": "missing", "where": "unknown", "text": "どこにも無い"}`)},
		{id: "T-0004", title: "申告の無い課題", status: "done",
			reportMD: "## Report\n本文\n## Remember\n- 別の学び\n", reportJSON: `{"commit": "` + gapCommit + `"}`},
		{id: "T-0005", title: "先の言えない申告", status: "done",
			reportMD: "## Report\n本文\n", reportJSON: gapReport(`{"kind": "missing", "where": "unknown", "text": "探した先が言えない"}`)},
	})
	items, err := l.collectFrom(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	gaps := map[string]Item{}
	for _, it := range items {
		if it.Kind == "gap" {
			gaps[it.Text] = it
		}
		for _, w := range []string{"見送りの語を含む申告の行", "申告の節の行", "- wrong AGENTS.md", "- missing package"} {
			if strings.Contains(it.Text, w) {
				t.Errorf("## Gaps の節の行が種類 %s の項目になった: %q", it.Kind, it.Text)
			}
		}
	}
	wrong := "wrong AGENTS.md: 一覧が 1 つ違う 直し方: 合わせる"
	missing := "missing package/skills/worker/SKILL.md: 置き場が書いていない"
	ce := "hard_to_find ./package/skills/worker/SKILL.md: 遅れて見つけた"
	unknown := "missing unknown: 探した先が言えない"
	var texts []string
	for k := range gaps {
		texts = append(texts, k)
	}
	slices.Sort(texts)
	want := []string{ce, missing, unknown, wrong}
	slices.Sort(want)
	if !slices.Equal(texts, want) {
		t.Fatalf("種類 gap の項目: %q\nwant %q", texts, want)
	}
	if it := gaps[wrong]; it.From != "T-0001" || it.Key != itemKey("demo", "T-0001", "gap", wrong) || it.Repo != "example.invalid/me/app" || it.Prior != nil {
		t.Errorf("AGENTS.md の申告: %+v（ほかに同じ先の申告は無い）", it)
	}
	wantPrior := []PriorGap{{"T-0002", "hard_to_find", "遅れて見つけた"}, {"T-0003", "missing", "作業中に気づいた"}}
	if it := gaps[missing]; !slices.Equal(it.Prior, wantPrior) {
		t.Errorf("同じ先の別の課題の申告が添えられない: %+v\nwant %+v", it.Prior, wantPrior)
	}
	wantPrior = []PriorGap{{"T-0001", "missing", "置き場が書いていない"}, {"T-0003", "missing", "作業中に気づいた"}}
	if it := gaps[ce]; it.From != "T-0002" || !slices.Equal(it.Prior, wantPrior) {
		t.Errorf("Chief Engineer の課題の申告: %+v\nwant prior %+v", it, wantPrior)
	}
	if it := gaps[unknown]; it.Prior != nil {
		t.Errorf("先の言えない申告どうしを同じ先と見た: %+v", it.Prior)
	}
	b, _ := json.Marshal(gaps[missing])
	if !strings.Contains(string(b), `"prior":[{"issue":"T-0002","kind":"hard_to_find","text":"遅れて見つけた"}`) {
		t.Errorf("項目の JSON に prior が無い: %s", b)
	}
	var others []string
	for _, it := range items {
		if it.Kind != "gap" && it.From == "T-0001" {
			others = append(others, it.Kind+" "+it.Text)
		}
	}
	slices.Sort(others)
	if !slices.Equal(others, []string{"next - 次の一手", "remember - 学び"}) {
		t.Errorf("T-0001 のほかの項目: %q", others)
	}
}

// 種類 gap の項目を閉じ方 first（1 件目）で閉じた判定は、apply --dry-run を通り、Chief of Staff の「閉じてよい」を待つ表に出る。
// 種類 agents の項目を first で閉じた判定は断られる（ADR-0061 受け入れ条件 4 を ADR-0062 決めたこと 13 で読み替えたもの）。
func TestApplyDryRunTakesFirstForGap(t *testing.T) {
	gap := "missing package/skills/worker/SKILL.md: 置き場が書いていない"
	issues := []gapIssue{{id: "T-0001", title: "担当の課題", status: "done",
		reportMD:   "## Report\n本文\n## Remember\n- 学び\nFor AGENTS.md: 作法の行\n",
		reportJSON: gapReport(`{"kind": "missing", "where": "package/skills/worker/SKILL.md", "text": "置き場が書いていない"}`)}}
	gkey := itemKey("demo", "T-0001", "gap", gap)
	akey := itemKey("demo", "T-0001", "agents", "For AGENTS.md: 作法の行")
	run := func(t *testing.T, key string) (map[string]any, error) {
		t.Helper()
		l := gapLedger(t, issues)
		lib := filepath.Join(l.dir, "issues", "T-0900", "library")
		if err := os.MkdirAll(lib, 0o755); err != nil {
			t.Fatal(err)
		}
		col, _ := json.Marshal(map[string]any{"commits": map[string]string{"example.invalid/me/app": gapCommit}, "items": []any{}})
		dec, _ := json.Marshal(map[string]any{"goal": "g", "collected": "2026-11-24T00:00:00Z", "items": []any{
			map[string]any{"key": key, "commit": gapCommit, "decision": "close", "closing": "first", "reason": "1 件目: 根拠が 1 件"}}})
		for name, b := range map[string][]byte{"items.json": col, "decisions.json": dec} {
			if err := os.WriteFile(filepath.Join(lib, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var out bytes.Buffer
		if err := runApply(l, "T-0900", 3, true, l.now, &out); err != nil {
			return nil, err
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got, nil
	}
	t.Run("gap は通り、判定を待つ", func(t *testing.T) {
		got, err := run(t, gkey)
		if err != nil {
			t.Fatal(err)
		}
		review, _ := got["review"].([]any)
		if len(review) != 1 {
			t.Fatalf("review: %v", got["review"])
		}
		row := review[0].(map[string]any)
		if row["key"] != gkey || row["kind"] != "gap" || row["closing"] != "first" || row["verdict"] != "none" || row["text"] != gap {
			t.Errorf("review の行: %v", row)
		}
	})
	t.Run("agents は断る", func(t *testing.T) {
		_, err := run(t, akey)
		if err == nil || !strings.Contains(err.Error(), "閉じ方 first は agents の項目に使えない") {
			t.Fatalf("agents の first を断らない: %v", err)
		}
	})
}

// 種類 remember・memory・next の項目の閉じ方 first（Andon の skill とパッケージの直しで根拠が 1 件）は、Chief of Staff の
// 「閉じてよい」があれば当たり、無ければ kanban にも記録の置き場にも手を入れない。種類 agents の first は断る（ADR-0062 受け入れ条件 12）。
// 偽の kanban の CLI は list と show のほかを打たれると落ちるので、当たった回は kanban を書いていない。
func TestApplyFirstForRememberMemoryNext(t *testing.T) {
	issues := []gapIssue{{id: "T-0001", title: "担当の課題", status: "done", reportJSON: `{"commit": "` + gapCommit + `"}`,
		reportMD: "## Report\n本文\n## Next\n- Andon の手引きに置き場を足す\n## Remember\n- skill の順を直す\nFor AGENTS.md: 作法の行\n"}}
	nkey := itemKey("demo", "T-0001", "next", "- Andon の手引きに置き場を足す")
	rkey := itemKey("demo", "T-0001", "remember", "- skill の順を直す")
	akey := itemKey("demo", "T-0001", "agents", "For AGENTS.md: 作法の行")
	memBody := "skill triage の表の順が分かりにくい（T-0001）\n"
	mkey := itemKey("demo", memoryFrom("skill-order.md"), "memory", memBody)
	const reason = "1 件目: 根拠が 1 件"
	setup := func(t *testing.T, keys []string, verdicts bool) (*ledger, string) {
		t.Helper()
		l := gapLedger(t, issues)
		l.memory = filepath.Join(t.TempDir(), "memory")
		if err := os.MkdirAll(l.memory, 0o755); err != nil {
			t.Fatal(err)
		}
		mem := filepath.Join(l.memory, "skill-order.md")
		if err := os.WriteFile(mem, []byte(memBody), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(l.dir, "config.yaml"), []byte("chief_of_staff: t5\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lib := filepath.Join(l.dir, "issues", "T-0900", "library")
		if err := os.MkdirAll(lib, 0o755); err != nil {
			t.Fatal(err)
		}
		var items, vs []any
		for _, k := range keys {
			it := map[string]any{"key": k, "decision": "close", "closing": "first", "reason": reason}
			if k != mkey {
				it["commit"] = gapCommit
			}
			items = append(items, it)
			vs = append(vs, verdict{Key: k, Verdict: verdictOK, Closing: "first", Reason: reason, By: "terminal:t5", At: "2026-11-24T01:00:00Z"})
		}
		col, _ := json.Marshal(map[string]any{"commits": map[string]string{"example.invalid/me/app": gapCommit}, "items": []any{}})
		dec, _ := json.Marshal(map[string]any{"goal": "g", "collected": "2026-11-24T00:00:00Z", "items": items})
		files := map[string][]byte{"items.json": col, "decisions.json": dec}
		if verdicts {
			files[verdictFileName], _ = json.Marshal(map[string]any{"verdicts": vs})
		}
		for name, b := range files {
			if err := os.WriteFile(filepath.Join(lib, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return l, mem
	}
	t.Run("閉じてよいがあれば当たる", func(t *testing.T) {
		l, mem := setup(t, []string{nkey, rkey, mkey}, true)
		var out bytes.Buffer
		if err := runApply(l, "T-0900", 3, false, l.now, &out); err != nil {
			t.Fatalf("当たらない: %v", err)
		}
		recs, _ := filepath.Glob(filepath.Join(l.dir, "library", "triage", "*-T-0900.json"))
		if len(recs) != 1 {
			t.Fatalf("記録: %v", recs)
		}
		b, err := os.ReadFile(recs[0])
		if err != nil {
			t.Fatal(err)
		}
		var rec struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, it := range rec.Items {
			v, _ := it["verdict"].(map[string]any)
			if it["closing"] == "first" && v["verdict"] == verdictOK {
				got = append(got, it["key"].(string))
			}
		}
		slices.Sort(got)
		want := []string{nkey, rkey, mkey}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("記録の first の項目: %q want %q", got, want)
		}
		if _, err := os.Stat(mem); err == nil {
			t.Error("1 件目で閉じた memory のファイルが残った")
		}
	})
	t.Run("閉じてよいが無ければ手を入れない", func(t *testing.T) {
		l, mem := setup(t, []string{nkey, rkey, mkey}, false)
		var out bytes.Buffer
		err := runApply(l, "T-0900", 3, false, l.now, &out)
		if err == nil || !strings.Contains(err.Error(), "Chief of Staff の判定が 3 件足りない") {
			t.Fatalf("判定の無い first を断らない: %v", err)
		}
		if _, err := os.Stat(filepath.Join(l.dir, "library", "triage")); err == nil {
			t.Error("断ったのに記録の置き場を作った")
		}
		if _, err := os.Stat(mem); err != nil {
			t.Errorf("断ったのに memory のファイルを消した: %v", err)
		}
	})
	t.Run("agents の first は断る", func(t *testing.T) {
		l, _ := setup(t, []string{akey}, true)
		var out bytes.Buffer
		err := runApply(l, "T-0900", 3, false, l.now, &out)
		if err == nil || !strings.Contains(err.Error(), "閉じ方 first は agents の項目に使えない") {
			t.Fatalf("agents の first を断らない: %v", err)
		}
		if _, err := os.Stat(filepath.Join(l.dir, "library", "triage")); err == nil {
			t.Error("断ったのに記録の置き場を作った")
		}
	})
}
