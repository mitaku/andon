#!/usr/bin/env bash
# `andon retro patterns`・`andon retro apply` とルーチン retro の例。一時の台帳（kanban init のあと課題のファイルを直に置く）・偽の gh・
# 一時の Claude Code のセッションログで回す。hc の端末は使わない（HC_TERMINAL_ID を担当の名前にするだけ）。
# 規則の細部は internal/retro/patterns_test.go。ここはバイナリの入口・ADR-0028 の受け入れ条件 11〜17・20・ADR-0039 の受け入れ条件 1・2・断ったら何も書かないことを見る。
# ANDON_EMIT_DIR があれば retro.json を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
date -u -d '1 day ago' +%s >/dev/null 2>&1 || { echo "skip: GNU date が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
here=$(cd "$(dirname "$0")/.." && pwd)
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET
ago() { date -u -d "$1 ago" +%Y-%m-%dT%H:%M:%SZ; }
die() { echo "FAIL $*" >&2; exit 1; }
pr=https://github.com/example/sample-site/pull

# 偽の gh: マージされた PR の一覧（#12 だけ）。check-runs は無い（読まれれば落とす）
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
case "\$*" in
"pr list --repo example/sample-site --state merged --limit 1000 --json number,url,title,body,mergedAt,mergeCommit")
  echo '[{"number": 12, "url": "$pr/12", "title": "x", "body": "", "mergedAt": "$(ago '1 day')", "mergeCommit": null}]' ;;
*) echo "unexpected: \$*" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"
export ANDON_HOME=$tmp/pkg CLAUDE_CONFIG_DIR=$tmp/claude CODEX_HOME=$tmp/codex
mkdir -p "$ANDON_HOME"

# 台帳: S-0010 は前の振り返り（2日前に片付いた）。S-0011 はその前、S-0012・S-0013 は後に片付いた担当の課題。S-0020 が今回の振り返り
root=$tmp/sample-site
mkdir -p "$root"
(cd "$root" && kanban init --prefix S --repo github.com/example/sample-site >/dev/null)
export KANBAN_ROOT=$root
k=$root/kanban
yaml() { local id=$1; shift; mkdir -p "$k/issues/$id/library"; printf 'id: %s\ntitle: %s の題\n%b' "$id" "$id" "$*" >"$k/issues/$id/issue.yaml"; }
yaml S-0010 'status: done\nprofile: claude\nroutine: retro\n'
yaml S-0011 'status: done\nprofile: claude\nrepo: github.com/example/sample-site\n'
yaml S-0012 'status: done\nprofile: claude\nrepo: github.com/example/sample-site\n'
printf '{"pr": "%s/12"}\n' "$pr" >"$k/issues/S-0012/library/report.json"
yaml S-0013 'status: done\nprofile: claude\nrepo: github.com/example/sample-site\n'
yaml S-0020 'status: doing\nprofile: claude\nroutine: retro\nagent: t9\n'
for x in "S-0010 2 days" "S-0011 3 days" "S-0012 1 day" "S-0013 1 day"; do
  set -- $x
  printf '{"at":"%s","by":"terminal:t1","id":"%s","change":{"status":"done"}}\n' "$(ago "$2 $3")" "$1"
done >>"$k/log.jsonl"

# セッションログ: Chief of Staff の2セッション（cwd がプロジェクトの根）。最初の文は起動の文なので抜かれない
mkdir -p "$CLAUDE_CONFIG_DIR/projects/-sample-site"
hum() { jq -cn --arg u "$1" --arg at "$2" --arg cwd "$root" --arg text "$3" \
  '{type: "user", uuid: $u, timestamp: $at, cwd: $cwd, origin: {kind: "human"}, promptSource: "typed", message: {role: "user", content: $text}}'; }
{ hum a0 "$(ago '4 days')" '起動'; hum a1 "$(ago '3 days')" '題に番号を書かないで'; hum a2 "$(ago '1 day')" 'また題に番号が入っている'; } \
  >"$CLAUDE_CONFIG_DIR/projects/-sample-site/c0c0c0c0-0000-4000-8000-000000000001.jsonl"
{ hum b0 "$(ago '5 days')" '起動'; hum b1 "$(ago '4 days')" '前にも言ったが題に番号'; } \
  >"$CLAUDE_CONFIG_DIR/projects/-sample-site/c0c0c0c0-0000-4000-8000-000000000002.jsonl"

KANBAN_ROOT=$root "$andon" retro collect >"$tmp/in.json"
jq -e '[.typed[].id] == ["h-b1", "h-a1", "h-a2"] and .previous_retro.issue == "S-0010"' "$tmp/in.json" >/dev/null \
  || die "検査の前提: 入力 $(jq -c '{typed: [.typed[].id], previous_retro}' "$tmp/in.json")"

# p <evidence の JSON の配列> [jq の式]: パターン1つの入力
p() {
  jq -cn --argjson ev "$1" '{patterns: [{id: "p1", summary: "PR の題に課題番号が入り、人が直した", evidence: $ev, level: "test", action: "add",
    target: {kind: "project", repo: "github.com/example/sample-site", paths: ["tests/pr-title.sh"]},
    effect: "題に課題番号が入った PR が CI で落ちる", risk: "誤検知", decision: "fix", reason: "テストで止められる"}], fixes: []}' | jq -c "${2:-.}"
}
out=$k/issues/S-0020/library/retro.json
refuse() { # refuse <説明> <理由の語> <入力>: 断り（終了コード 1・理由の行）、retro.json を書かない
  local rc=0
  HC_TERMINAL_ID=t9 "$andon" retro patterns S-0020 - <<<"$3" >/dev/null 2>"$tmp/err" || rc=$?
  [ "$rc" = 1 ] || die "$1: 断らなかった（rc=$rc）"
  grep -q "$2" "$tmp/err" || die "$1: 理由に「$2」が無い: $(cat "$tmp/err")"
  [ ! -e "$out" ] || die "$1: 断ったのに retro.json がある"
}
accept() {
  HC_TERMINAL_ID=t9 "$andon" retro patterns S-0020 - <<<"$2" >"$tmp/ok" 2>"$tmp/err" || die "$1: 断った: $(cat "$tmp/err")"
  [ -e "$out" ] || die "$1: retro.json が無い"
  rm -f "$out"
}

# 11: 1回しか起きていないパターンは通らない
refuse "根拠が1件" "別々の2件に満たない" "$(p '["S-0012"]')"
refuse "同じ課題を2回" "別々の2件に満たない" "$(p '["S-0012", "S-0012"]')"
refuse "同じセッションの文を2つ" "別々の2件に満たない" "$(p '["h-a1", "h-a2"]')"
refuse "入力に無い id" "集めた入力" "$(p '["S-0012", "S-0099"]')"
accept "別々の課題2件" "$(p '["S-0012", "S-0013"]')"
accept "課題1件と人の文1件" "$(p '["S-0013", "h-a2"]')"
echo 'ok 11: 根拠が別々の2件に満たないパターンは断る（課題2件・課題と人の文なら通る）'

# 12: 新しい出来事が無ければ出せない
refuse "根拠が全部前の振り返りより前" "前の振り返り S-0010" "$(p '["S-0011", "h-b1"]')"
accept "1件だけ後" "$(p '["S-0011", "S-0012"]')"
echo 'ok 12: 根拠のどれも前の振り返りより後でなければ断る（1件後なら通る）'

# 13: skill か文面を直す先にするなら、上の段で止められない理由が要る
refuse "skill の段に理由が無い" "上の段で止められない理由" "$(p '["S-0012", "S-0013"]' '.patterns[0].level = "skill"')"
refuse "文面の段に理由が無い" "上の段で止められない理由" "$(p '["S-0012", "S-0013"]' '.patterns[0].level = "text"')"
echo 'ok 13: skill と文面の段は、上の段で止められない理由が無ければ断る'

# 14: 振り返り自身を直す先にできない
refuse "skill retro" "振り返り自身" "$(p '["S-0012", "S-0013"]' '.patterns[0].target = {kind: "andon", paths: ["package/skills/retro/SKILL.md"]}')"
refuse "andon retro" "振り返り自身" "$(p '["S-0012", "S-0013"]' '.patterns[0].target = {kind: "andon", paths: ["internal/retro/apply.go"]}')"
echo 'ok 14: 振り返り自身（skill retro・andon retro）を直す先にすると断る'

# 担当の端末でない・doing でない
rc=0; HC_TERMINAL_ID=t1 "$andon" retro patterns S-0020 - <<<"$(p '["S-0012", "S-0013"]')" >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '担当' "$tmp/err" && [ ! -e "$out" ] || die "ほかの端末から書けた（rc=$rc）: $(cat "$tmp/err")"
echo 'ok パターンの一覧は、その課題の担当の端末からだけ書ける'

# 15・16・17: 当てる。p1 は案件の repo のテスト（直す課題）、p2 は Andon のパッケージ（この台帳の repos に Andon が無いので人の課題）、p3 は捨てる。
# p4〜p6 も直す課題で、fix は 5 件（ADR-0039 受け入れ条件 1: 上限は無く、全部が一覧の順に課題になる）
good=$(p '["S-0012", "h-a2"]' '.patterns += [
  (.patterns[0] + {id: "p2", summary: "見送りの理由が報告に無く、検収で戻した", evidence: ["S-0012", "S-0013"], level: "skill", action: "change",
    target: {kind: "andon", repo: "github.com/example/andon", paths: ["package/skills/worker/SKILL.md"]}, not_upstream: "見送りの理由は文で、機械が確かめる値が無い"}),
  (.patterns[0] + {id: "p3", summary: "見込みより長くかかった", evidence: ["S-0012", "S-0013"], decision: "drop", reason: "外の障害で、繰り返しではない"}),
  (.patterns[0] + {id: "p4", summary: "テストの名前が古いまま残り、2回直した", evidence: ["S-0012", "S-0013"], target: {kind: "project", repo: "github.com/example/sample-site", paths: ["tests/names.sh"]}}),
  (.patterns[0] + {id: "p5", summary: "リンクの切れが残り、2回直した", evidence: ["S-0012", "S-0013"], target: {kind: "project", repo: "github.com/example/sample-site", paths: ["tests/links.sh"]}}),
  (.patterns[0] + {id: "p6", summary: "lint の指摘が残り、2回直した", evidence: ["S-0012", "S-0013"], target: {kind: "project", repo: "github.com/example/sample-site", paths: ["tests/lint.sh"]}})]')
HC_TERMINAL_ID=t9 "$andon" retro patterns S-0020 - <<<"$good" >"$tmp/ok" 2>"$tmp/err" || die "正しい一覧を断った: $(cat "$tmp/err")"
emit "$out" patterns.json
jq -e '.retro == "S-0020" and .days == 7 and .previous_retro.issue == "S-0010" and ([.patterns[].id] == ["p1", "p2", "p3", "p4", "p5", "p6"])' "$out" >/dev/null || die "retro.json: $(jq -c . "$out")"
grep -q 'パターン 6 件、fix 5 件・drop 1 件' "$tmp/ok" || die "書いた行: $(cat "$tmp/ok")"
cp "$out" "$tmp/good.json"

issues() { kanban list --all --json | jq -r '.[].id' | sort | tr '\n' ' '; }
before=$(issues)
# 判定の無いパターン・根拠の足りないパターン（手で書き換えた retro.json）は、当てずに断る。台帳は変わらない（ADR-0039 受け入れ条件 2）
jq 'del(.patterns[2].decision)' "$tmp/good.json" >"$out"
rc=0; "$andon" retro apply S-0020 >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '判定が無い' "$tmp/err" || die "判定の無い一覧を当てた（rc=$rc）: $(cat "$tmp/err")"
jq '.patterns[0].evidence = ["S-0012"]' "$tmp/good.json" >"$out"
rc=0; "$andon" retro apply S-0020 >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '別々の2件に満たない' "$tmp/err" || die "根拠1件の一覧を当てた（rc=$rc）: $(cat "$tmp/err")"
[ "$(issues)" = "$before" ] && [ ! -e "$k/issues/S-0020/library/retro-applied.json" ] || die "断ったのに台帳が変わった: $(issues)"
echo 'ok 15: 判定の無いパターン・根拠の足りないパターンは当てずに断る（台帳は変わらない）'

cp "$tmp/good.json" "$out"
"$andon" retro apply S-0020 --dry-run >"$tmp/dry.json" || die "--dry-run が落ちた"
jq -e '[.issues[] | [.pattern, .who]] == [["p1", "worker"], ["p2", "person"], ["p4", "worker"], ["p5", "worker"], ["p6", "worker"]] and .dropped == [{pattern: "p3", reason: "外の障害で、繰り返しではない"}]' "$tmp/dry.json" >/dev/null \
  || die "--dry-run: $(jq -c . "$tmp/dry.json")"
[ "$(issues)" = "$before" ] && [ ! -e "$k/issues/S-0020/library/retro-applied.json" ] || die "--dry-run が台帳に書いた"

"$andon" retro apply S-0020 >"$tmp/apply.out" 2>"$tmp/err" || die "当てられない: $(cat "$tmp/err")"
rec=$k/issues/S-0020/library/retro-applied.json
jq -e '.complete == true and ([.issues[] | [.pattern, .who]] == [["p1", "worker"], ["p2", "person"], ["p4", "worker"], ["p5", "worker"], ["p6", "worker"]]) and .dropped == [{pattern: "p3", reason: "外の障害で、繰り返しではない"}]' "$rec" >/dev/null \
  || die "記録: $(jq -c . "$rec")"
fix=$(jq -r '.issues[0].issue' "$rec") person=$(jq -r '.issues[1].issue' "$rec")
kanban show "$fix" --json >"$tmp/fix.json"
jq -e '.issue.parent == "S-0020" and .issue.repo == "github.com/example/sample-site" and .issue.title == "PR の題に課題番号が入り、人が直した" and (.issue.who // "") != "person"' "$tmp/fix.json" >/dev/null \
  || die "直す課題: $(jq -c .issue "$tmp/fix.json")"
brief=$(jq -r .brief "$tmp/fix.json")
for want in '解決すべき問題: PR の題に課題番号が入り、人が直した' '直す先: テスト・lint の段で足す' 'tests/pr-title.sh' '期待する効果: 題に課題番号が入った PR が CI で落ちる' \
  'リスク: 誤検知' '- S-0012: S-0012 の題' '- h-a2: 人が打った文' "PR の本文に書いてよい根拠の PR: $pr/12" 'S-0020 の library/retro.json のパターン p1'; do
  grep -qF -- "$want" <<<"$brief" || die "直す課題の依頼書に「$want」が無い: $brief"
done
! grep -q 'また題に番号が入っている' <<<"$brief" || die "依頼書に人の文の本文が入った"
grep -q 'また題に番号が入っている' "$CLAUDE_CONFIG_DIR/projects/-sample-site/c0c0c0c0-0000-4000-8000-000000000001.jsonl" || die "検査の前提: 人の文がログにある"
[ "$(kanban list --all --json | jq '[.[] | select(.parent == "S-0020")] | length')" = 5 ] || die "足した課題が5件でない: $(issues)"
# 足した順（課題の番号の順）が一覧の順と同じ
order=$(jq -r '[.issues[].issue] | join(" ")' "$rec")
[ "$order" = "$(jq -r '[.issues[].issue] | sort | join(" ")' "$rec")" ] || die "足した順が一覧の順でない: $order"
[ "$(kanban list --all --json | jq -r '[.[] | select(.parent == "S-0020") | .title] | join("|")')" = \
  "$(jq -r '[.patterns[] | select(.decision == "fix") | if .target.kind == "andon" then "Andon へ伝えるか: " + .summary else .summary end] | join("|")' "$tmp/good.json")" ] \
  || die "足した課題の題の並びが一覧の順でない: $(kanban list --all --json | jq -c '[.[] | select(.parent == "S-0020") | [.id, .title]]')"
f2=$(jq -r '[.issues[] | select(.who == "worker") | .issue] | join("・")' "$rec")
grep -q "直す課題 4 件（$f2）・人の課題 1 件（$person）・捨てた 1 件" "$tmp/apply.out" || die "当てた行: $(cat "$tmp/apply.out")"
! grep -q '上限' "$tmp/ok" "$tmp/apply.out" || die "上限の理由の行が出た: $(cat "$tmp/ok" "$tmp/apply.out")"
echo 'ok 16: fix ごとに直す課題が1件足され（fix 5 件なら 5 件、一覧の順に）、依頼書に要約・根拠・直す先・期待する効果・リスク。drop は理由つきで記録に残る'

kanban show "$person" --json >"$tmp/person.json"
jq -e '.issue.who == "person" and .issue.parent == "S-0020" and (.issue.repo // "") == "" and (.issue.title | startswith("Andon へ伝えるか: "))' "$tmp/person.json" >/dev/null \
  || die "人の課題: $(jq -c .issue "$tmp/person.json")"
echo 'ok 17: Andon の repo を持たないプロジェクトでは、パッケージの直しは人の課題（who=person）'

# 当て済みの一覧は当て直さない
rc=0; "$andon" retro apply S-0020 >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '当て済み' "$tmp/err" && [ "$(kanban list --all --json | jq '[.[] | select(.parent == "S-0020")] | length')" = 5 ] \
  || die "当て直した（rc=$rc）: $(cat "$tmp/err")"
echo 'ok 当て済みの振り返りは当て直さない'

# 20: ルーチンの例は台帳の形式に通り、毎日撃たれ、手で撃て、paused で止まる
ex=$here/package/skills/retro/assets/retro.yaml
grep -qE '^schedule: "[0-9]+ [0-9]+ \* \* \*"$' "$ex" || die "ルーチンの例が毎日でない: $(grep schedule "$ex")"
for paused in false true; do
  r=$tmp/routine-$paused
  mkdir -p "$r" && KANBAN_ROOT=$r kanban init --root "$r" --prefix R >/dev/null
  mkdir -p "$r/kanban/routines"
  cp "$ex" "$r/kanban/routines/retro.yaml"
  [ "$paused" = false ] || echo 'paused: true' >>"$r/kanban/routines/retro.yaml"
  KANBAN_ROOT=$r kanban lint >"$tmp/lint" 2>&1 || die "ルーチンの例が kanban lint に通らない: $(cat "$tmp/lint")"
  KANBAN_ROOT=$r kanban routine run retro >"$tmp/run" 2>&1 || die "kanban routine run retro が落ちた: $(cat "$tmp/run")"
  n=$(KANBAN_ROOT=$r kanban list --all --json | jq '[.[] | select(.routine == "retro")] | length')
  if [ "$paused" = false ]; then [ "$n" = 1 ] || die "手で撃って足されない: $(cat "$tmp/run")"
  else [ "$n" = 0 ] && grep -q 'paused' "$tmp/run" || die "paused なのに足された: $(cat "$tmp/run")"; fi
done
echo 'ok 20: ルーチンの例は kanban lint に通り、毎日撃たれ、手で撃つと routine: retro の課題が1件足され、paused で足されない'
