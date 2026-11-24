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

// fakePersonKanban は list・add・set・ask だけを受ける偽の kanban の CLI。課題は無い（list は空）。
// add は T-0100 から順に id を出し、打った command は FAKE_LOG に1行ずつ（add はタイトルも）残す。ask は <id>#1 を返す。
const fakePersonKanban = `#!/usr/bin/env bash
log=$FAKE_LOG
case $1 in
list) echo '[]' ;;
add) cat >/dev/null; n=$(( $(grep -c '^add ' "$log") + 100 )); echo "add $*" >>"$log"; echo "T-0$n" ;;
set) echo "$*" >>"$log" ;;
ask) cat >/dev/null; echo "$*" >>"$log"; echo "$2#1" ;;
*) echo "$*" >>"$log"; exit 9 ;;
esac
`

func personLedger(t *testing.T) (*ledger, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(bin, "kanban")
	if err := os.WriteFile(cli, []byte(fakePersonKanban), 0o755); err != nil {
		t.Fatal(err)
	}
	logf := filepath.Join(dir, "log")
	if err := os.WriteFile(logf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", logf)
	root := filepath.Join(dir, "demo")
	return &ledger{cli: cli, root: root, slug: "demo", dir: filepath.Join(root, "kanban"), now: time.Now()}, logf
}

func readLog(t *testing.T, logf string) []string {
	t.Helper()
	b, err := os.ReadFile(logf)
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

// トリアージの apply は、判断の person をオーナーの課題と問い（出し手は Chief of Staff、事実の先頭は出どころ）にして置き、
// 手作業の person を who: person と印 only_owner を持つ課題にする。種類 line が通り、種類を決めきれなかった判断の問いには
// その旨の事実の行が足される（ADR-0057 受け入れ条件 10）。
func TestApplyPersonPlacesQuestionOrHandWorkMark(t *testing.T) {
	key := "demo:T-5:question:" + strings.Repeat("0", 40)
	for _, c := range []struct {
		name  string
		it    item
		log   []string
		facts []string
	}{
		{
			name: "判断",
			it:   item{"key": key, "decision": "person", "kind": "promise", "question": "外へ公開するか", "options": []any{"公開する", "公開しない"}, "reason": "外との約束"},
			log: []string{
				"add add --brief - -- 外へ公開するか（オーナーの判断待ち）",
				"set T-0100 who=person",
				"ask T-0100 - --kind promise",
			},
			facts: []string{key},
		},
		{
			name: "line の判断",
			it:   item{"key": key, "decision": "person", "kind": "line", "question": "検査を足すか", "options": []any{"足す", "足さない", "あとで"}, "reason": "ラインの変更"},
			log: []string{
				"add add --brief - -- 検査を足すか（オーナーの判断待ち）",
				"set T-0100 who=person",
				"ask T-0100 - --kind line",
			},
			facts: []string{key},
		},
		{
			name: "種類を決めきれなかった判断",
			it: item{"key": key, "decision": "person", "kind": "business", "unsure": true, "question": "この機能を作るか", "options": []any{"作る", "作らない"},
				"reason": "何を作るかの判断か、使い勝手の好みかが分からない"},
			log: []string{
				"add add --brief - -- この機能を作るか（オーナーの判断待ち）",
				"set T-0100 who=person",
				"ask T-0100 - --kind business",
			},
			facts: []string{key, "トリアージは種類を決めきれなかった: 何を作るかの判断か、使い勝手の好みかが分からない"},
		},
		{
			name: "手作業",
			it:   item{"key": key, "decision": "person", "only_owner": "実機はオーナーの手元にだけある", "question": "実機で表示を確かめる（今週中）", "reason": "目視が完了条件"},
			log: []string{
				"add add 実機で表示を確かめる（今週中） --brief -",
				"set T-0100 who=person only_owner=実機はオーナーの手元にだけある",
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			l, logf := personLedger(t)
			if err := checkPersons([]item{c.it}); err != nil {
				t.Fatalf("正しい形が断られた: %v", err)
			}
			id, _, err := l.applyGroup("T-0001", "2026-11-23", []item{c.it}, nil)
			if err != nil || id != "T-0100" {
				t.Fatalf("applyGroup = %q, %v", id, err)
			}
			if got := readLog(t, logf); !slices.Equal(got, c.log) {
				t.Errorf("打った kanban の CLI:\n%q\nwant\n%q", got, c.log)
			}
			qpath := filepath.Join(l.dir, "issues", id, "library", "questions", "1.json")
			if c.facts == nil {
				if _, err := os.Stat(qpath); err == nil {
					t.Errorf("手作業に問いを置いた: %s", qpath)
				}
				return
			}
			b, err := os.ReadFile(qpath)
			if err != nil {
				t.Fatal(err)
			}
			var q struct {
				Kind    string   `json:"kind"`
				Facts   []string `json:"facts"`
				From    string   `json:"from"`
				Reason  string   `json:"reason"`
				Options []struct {
					Label string `json:"label"`
					Pick  bool   `json:"pick"`
				} `json:"options"`
			}
			if err := json.Unmarshal(b, &q); err != nil {
				t.Fatal(err)
			}
			if q.From != "chief-of-staff" || q.Kind != c.it.str("kind") || !slices.Equal(q.Facts, c.facts) || q.Reason != c.it.str("reason") {
				t.Errorf("問い: %s", b)
			}
			if len(q.Options) != len(c.it.options()) || !q.Options[0].Pick || q.Options[0].Label != c.it.options()[0] || q.Options[1].Pick {
				t.Errorf("選択肢の先頭が推奨案でない: %s", b)
			}
		})
	}
}

// トリアージの apply は、person の項目が判断と手作業のどちらの形でもなければ、kanban に何も打たずに断る
// （ADR-0057 受け入れ条件 11）。偽の kanban の CLI は打たれたら記録に残るので、記録が空なら課題の数は変わっていない。
func TestApplyRefusesPersonOfNeitherForm(t *testing.T) {
	key := "demo:T-5:question:" + strings.Repeat("0", 40)
	base := func(extra map[string]any) map[string]any {
		it := map[string]any{"key": key, "decision": "person", "question": "外へ公開するか", "reason": "外との約束"}
		for k, v := range extra {
			it[k] = v
		}
		return it
	}
	for name, c := range map[string]struct {
		it   map[string]any
		want string
	}{
		"kind も only_owner も無い":   {base(nil), "kind も only_owner も無い"},
		"両方ある":                    {base(map[string]any{"kind": "promise", "options": []any{"a", "b"}, "only_owner": "実機"}), "両方がある"},
		"kind があって options が 1 つ": {base(map[string]any{"kind": "promise", "options": []any{"公開する"}}), "options が 1 行の文字列 2 つ以上でない"},
		"pick を持つ":                {base(map[string]any{"kind": "promise", "options": []any{"a", "b"}, "pick": "a"}), "pick は使わない"},
	} {
		t.Run(name, func(t *testing.T) {
			l, logf := personLedger(t)
			lib := filepath.Join(l.dir, "issues", "T-0001", "library")
			if err := os.MkdirAll(lib, 0o755); err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(map[string]any{"goal": "g", "collected": "2026-11-23T00:00:00Z", "items": []any{c.it}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(lib, "decisions.json"), b, 0o644); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = runApply(l, "T-0001", 3, false, time.Now(), &out)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("断らないか理由が違う: %v（want %q）", err, c.want)
			}
			if got := readLog(t, logf); got != nil {
				t.Errorf("断る前に kanban の CLI を打った: %q", got)
			}
			if _, err := os.Stat(filepath.Join(l.dir, "library", "triage")); err == nil {
				t.Error("断ったのに記録の置き場を作った")
			}
		})
	}
}

// person の形の確かめ。判断は kind（灯りの 5 種）と 2 つ以上の options、手作業は only_owner。
// unsure は判断にだけ、bool で付ける。
func TestCheckPersons(t *testing.T) {
	key := "demo:T-5:question:" + strings.Repeat("0", 40)
	decision := func(extra map[string]any) item {
		it := item{"key": key, "decision": "person", "question": "q", "reason": "r", "kind": "users", "options": []any{"a", "b"}}
		for k, v := range extra {
			it[k] = v
		}
		return it
	}
	hand := item{"key": key, "decision": "person", "question": "q", "reason": "r", "only_owner": "実機"}
	handNullOptions := item{"key": key, "decision": "person", "question": "q", "reason": "r", "only_owner": "実機", "options": nil, "kind": nil}
	for _, it := range []item{decision(nil), decision(map[string]any{"kind": "line"}), decision(map[string]any{"unsure": true}), hand, handNullOptions} {
		if err := checkPersons([]item{it}); err != nil {
			t.Errorf("%v が断られた: %v", it, err)
		}
	}
	// 同じ question の判断が 2 つあれば断る（あとの方は当てる途中で、問いを持つ同じタイトルの課題に当たる）。手作業は数えない。
	other := decision(map[string]any{"key": "demo:T-6:question:" + strings.Repeat("0", 40)})
	if err := checkPersons([]item{decision(nil), other}); err == nil || !strings.Contains(err.Error(), "同じものが 2 つ") {
		t.Errorf("同じ question の判断 2 つが通った: %v", err)
	}
	if err := checkPersons([]item{hand, decision(nil)}); err != nil {
		t.Errorf("同じ question の手作業と判断が断られた: %v", err)
	}
	for name, it := range map[string]item{
		"問いが空":           decision(map[string]any{"question": ""}),
		"問いが 2 行":        decision(map[string]any{"question": "a\nb"}),
		"問いが - で始まる":     decision(map[string]any{"question": "-x"}),
		"知らない kind":      decision(map[string]any{"kind": "scope"}),
		"選択肢が空の文字列":      decision(map[string]any{"options": []any{"a", ""}}),
		"選択肢が文字列でない":     decision(map[string]any{"options": []any{"a", map[string]any{"label": "b"}}}),
		"unsure が文字列":    decision(map[string]any{"unsure": "yes"}),
		"手作業の理由が 2 行":    {"key": key, "decision": "person", "question": "q", "reason": "r", "only_owner": "a\nb"},
		"手作業に options":   {"key": key, "decision": "person", "question": "q", "reason": "r", "only_owner": "x", "options": []any{"a", "b"}},
		"手作業に unsure":    {"key": key, "decision": "person", "question": "q", "reason": "r", "only_owner": "x", "unsure": true},
		"kind が null だけ": {"key": key, "decision": "person", "question": "q", "reason": "r", "kind": nil},
	} {
		if err := checkPersons([]item{it}); err == nil {
			t.Errorf("%s が通った", name)
		}
	}
}
