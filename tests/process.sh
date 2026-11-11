#!/usr/bin/env bash
# 流れと状態の定義（ADR-0017 受け入れ条件 1・2）。
#   1. schema/process/ の2本が schema の決まりを守る。
#   2. process/**/*.yaml と見本（tests/process/process/）が schema を通る（andon process json で JSON にして schema_check.py へ）。
#      ワークストリームの triggers[].start と steps[].next[].id が steps[] に在る。
#   3. 見本を壊すと落ちる（知らないキー・enum の外・必須の欠け・やる者の種類に合わないキー・在らない id）。
#   3b. process/states/ の transitions[].once: true が on の「§7.2 #<n>」で dispatcher.md §7.2 の行を指す（受け入れ条件 4）。
#   4. andon process render --check: repo の docs/process/ と見本の生成物（tests/process/docs/process/）が作り直しても変わらない。
#      生成物に機械が読む塊（```json・```yaml）が無い。
#   5. ワークストリームが 8 本。見守り・仕分けは決めたこと 16 の項目を全部持ち、skill が SKILL の節を指す。残り 6 本は name・title・purpose だけ（受け入れ条件 5）。
# 道具は python3（標準ライブラリだけ）と jq と andon（tests/lib/andon.sh）。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/andon.sh

check_py=package/lib/andon/schema_check.py
fail=0
ok() { echo "ok   $1"; }
ng() { echo "FAIL $1"; fail=1; }

# 1. schema そのもの
for s in workstream state; do
  if out=$(python3 "$check_py" lint "schema/process/$s.schema.json" 2>&1); then ok "schema/process/$s の決まり"
  else ng "schema/process/$s の決まり"; printf '     %s\n' "$out"; fi
done

# kind_of <YAML のパス> → workstream | state（置き場で決まる）
kind_of() {
  case "$1" in
    */process/workstreams/*.yaml | process/workstreams/*.yaml) echo workstream ;;
    */process/states/*.yaml | process/states/*.yaml) echo state ;;
    *) return 1 ;;
  esac
}

# refs <JSON> → steps[] に無い id を 1 行 1 つで出す（ワークストリームだけ）
refs() {
  jq -r '([.steps[]?.id]) as $ids
    | ([.triggers[]?.start] + [.steps[]?.next[]?.id])[]
    | select(. as $x | $ids | index($x) | not)' "$1"
}

# check <YAML> → 0 通る / 1 落ちる（理由は $tmp/why）
check() {
  local yaml=$1 kind json=$tmp/def.json
  kind=$(kind_of "$yaml") || { echo "置き場が process/workstreams・process/states でない" >"$tmp/why"; return 1; }
  "$andon" process json "$yaml" >"$json" 2>"$tmp/why" || return 1
  python3 "$check_py" validate "schema/process/$kind.schema.json" "$json" >"$tmp/why" 2>&1 || return 1
  if [ "$kind" = workstream ]; then
    local missing
    missing=$(refs "$json")
    [ -z "$missing" ] || { echo "steps[] に無い id: $missing" >"$tmp/why"; return 1; }
  fi
  return 0
}

# 2. 本物と見本が通る
mapfile -t defs < <(find process tests/process/process -name '*.yaml' 2>/dev/null | sort)
for f in "${defs[@]}"; do
  if check "$f"; then ok "$f が schema を通る"; else ng "$f が通らない"; sed 's/^/     /' "$tmp/why"; fi
done
stray=$(find process -type f ! -name '*.yaml' ! -name .gitkeep 2>/dev/null | sort)
[ -z "$stray" ] && ok "process/ には定義の YAML だけ" || ng "process/ に YAML でないファイル: $stray"
stray=$(find process -name '*.yaml' ! -path 'process/workstreams/*' ! -path 'process/states/*' 2>/dev/null | sort)
[ -z "$stray" ] && ok "process/ の YAML は workstreams/ か states/ に在る" || ng "置き場の外の YAML: $stray"

# 3. 見本を壊すと落ちる（見本 ｜ 見本の JSON に当てる jq ｜ 説明）。壊した JSON をそのまま YAML として置く
ws=tests/process/process/workstreams/sample-watch.yaml
st=tests/process/process/states/sample-item.yaml
mkdir -p "$tmp/bad/process/workstreams" "$tmp/bad/process/states"
while IFS='|' read -r base filter desc; do
  case "$base" in '' | '#'*) continue ;; esac
  src=$ws; [ "$base" = st ] && src=$st
  out="$tmp/bad/process/$( [ "$base" = st ] && echo states || echo workstreams )/broken.yaml"
  # JSON は YAML でもある
  "$andon" process json "$src" | jq "$filter" >"$out"
  if check "$out"; then ng "壊した見本が通る: $desc"; else ok "壊した見本は落ちる: $desc（$(head -1 "$tmp/why")）"; fi
done <<'BAD'
ws|.reads = ["x"]|ワークストリームの知らないキー（reads[]）
ws|.steps[0].color = "red"|step の知らないキー
ws|.triggers[0].kind = "cron"|triggers[].kind の enum の外
ws|del(.triggers[0].notification)|kind notification に notification が無い
ws|del(.triggers[1].every)|kind interval に every が無い
ws|del(.purpose)|purpose が無い
ws|del(.name)|name が無い
ws|.steps[1].by = "robot"|steps[].by の enum の外
ws|.steps[0].skill = "chief-of-staff#5"|by machine の step に skill
ws|.steps[1].command = "andon x"|by ai の step に command
ws|.steps[2].once = true|by person の step に once
ws|.steps[1].asks[0].kind = "scope"|asks[].kind が灯りの種類でない
ws|.triggers[0].start = "nowhere"|triggers[].start が steps[] に無い
ws|.steps[1].next[0].id = "nowhere"|next[].id が steps[] に無い
ws|.name = "Watch"|name が識別子でない
st|.transitions[0].by = "robot"|transitions[].by の enum の外
st|.transitions[0].extra = 1|transition の知らないキー
st|del(.source)|source が無い
st|del(.title)|title が無い
st|.states[0].record = "Pending"|record が <記録>:<項目> でない
st|.positions = ["sample-LogPos"]|positions[] が <記録>:<項目> でない
BAD

# 3b. 「2 回走らせない」の導出（受け入れ条件 4）: process/states/ の transitions[].once: true の 1 件ごとに、
#     on が「§7.2 #<n>」で dispatcher.md §7.2 の行を指し、その行の 走らせる者・二重に走ったら・今の守り が空でない。
# once_rows <dispatcher.md> → §7.2 の表の行のうち、走らせる者・二重に走ったら・今の守り が空でない行の # を 1 行 1 つ
once_rows() {
  python3 - "$1" <<'PY'
import re, sys
lines = open(sys.argv[1], encoding="utf-8").read().split("\n")
inside = False
for l in lines:
    if l.startswith("### "):
        inside = l.startswith("### 7.2 ")
        continue
    if not inside or not l.startswith("|"):
        continue
    cells = [c.strip() for c in re.split(r"(?<!\\)\|", l.strip())[1:-1]]
    if len(cells) == 5 and cells[0].isdigit() and all(cells[2:5]):
        print(cells[0])
PY
}
# once_check <JSON の定義> <行の # の一覧のファイル> → 0 揃う / 1 食い違う（理由は stdout に 1 行ずつ）
once_check() {
  jq -r '.transitions[]? | select(.once == true) | "\(.from) → \(.to)\t\(.on)"' "$1" | {
    bad=0
    while IFS=$'\t' read -r edge on; do
      n=$(printf '%s\n' "$on" | grep -oE '§7\.2 #[0-9]+' | head -1 | sed 's/.*#//') || true
      if [ -z "$n" ]; then echo "$edge: once: true なのに on が §7.2 #<n> を指さない"; bad=1
      elif ! grep -qx "$n" "$2"; then echo "$edge: §7.2 #$n の行が無いか、走らせる者・二重に走ったら・今の守り が空"; bad=1; fi
    done
    exit "$bad"
  }
}
once_rows docs/dispatcher.md >"$tmp/rows"
[ -s "$tmp/rows" ] && ok "dispatcher.md §7.2 の表が読める（$(wc -l <"$tmp/rows") 行）" || ng "dispatcher.md §7.2 の表が読めない"
nonce=0
for f in process/states/*.yaml; do
  [ -e "$f" ] || continue
  "$andon" process json "$f" >"$tmp/def.json"
  nonce=$((nonce + $(jq '[.transitions[]? | select(.once == true)] | length' "$tmp/def.json")))
  if out=$(once_check "$tmp/def.json" "$tmp/rows"); then ok "$f の once: true が dispatcher.md §7.2 の行を指す"
  else ng "$f の once: true が §7.2 と食い違う"; printf '     %s\n' "$out"; fi
done
[ "$nonce" -gt 0 ] && ok "once: true の移り方が $nonce 件ある" || ng "once: true の移り方が 1 件も無い"
# 壊すと落ちる（§7.2 を指さない・在らない行を指す・守りの欄が空の行を指す）
printf '%s\n' '| # | 処理 | 走らせる者 | 二重に走ったら | 今の守り |' '|---|---|---|---|---|' \
  '| 1 | 見本 | dispatcher | 二度起きる | 控え |' '| 2 | 見本 | dispatcher | 二度起きる |  |' >"$tmp/table"
printf '### 7.2 表\n\n' | cat - "$tmp/table" >"$tmp/disp.md"
once_rows "$tmp/disp.md" >"$tmp/rows-sample"
[ "$(cat "$tmp/rows-sample")" = 1 ] && ok "§7.2 の行の読み方: 守りの欄が空の行は数えない" || ng "§7.2 の行の読み方が違う: $(tr '\n' ' ' <"$tmp/rows-sample")"
while IFS='|' read -r on desc; do
  jq -n --arg on "$on" '{transitions: [{from: "a", to: "b", on: $on, by: "machine", once: true}]}' >"$tmp/once.json"
  if once_check "$tmp/once.json" "$tmp/rows-sample" >/dev/null; then ng "壊した once が通る: $desc"; else ok "壊した once は落ちる: $desc"; fi
done <<'BAD'
起こす|§7.2 を指さない
起こす（§7.2 #9）|在らない行を指す
起こす（§7.2 #2）|守りの欄が空の行を指す
BAD
jq -n '{transitions: [{from: "a", to: "b", on: "起こす（§7.2 #1）", by: "machine", once: true}, {from: "b", to: "c", on: "x", by: "ai"}]}' >"$tmp/once.json"
once_check "$tmp/once.json" "$tmp/rows-sample" >/dev/null && ok "§7.2 の行を指す once は通る（once で無い移り方は見ない）" || ng "§7.2 の行を指す once が落ちる"

# 4. 生成
if "$andon" process render --check . 2>"$tmp/why"; then ok "docs/process/ は process/ から作り直しても変わらない"
else ng "docs/process/ が process/ と食い違う（andon process render で作り直す）"; sed 's/^/     /' "$tmp/why"; fi
if "$andon" process render --check tests/process 2>"$tmp/why"; then ok "見本の生成物（tests/process/docs/process/）が変わらない"
else ng "見本の生成物が変わる"; sed 's/^/     /' "$tmp/why"; fi
[ -f docs/process/README.md ] && ok "docs/process/README.md（人向けの入口）がある" || ng "docs/process/README.md が無い"
machine=$(grep -lE '^```(json|yaml|yml)' docs/process/*.md tests/process/docs/process/*.md 2>/dev/null | grep -v '/README.md$' || true)
[ -z "$machine" ] && ok "生成物に機械が読む塊が無い" || ng "生成物に機械が読む塊: $machine"
for f in tests/process/process/workstreams/sample-watch.yaml tests/process/process/states/sample-item.yaml; do
  md=tests/process/docs/process/$(basename "$f" .yaml).md
  case "$(kind_of "$f")" in
    workstream) grep -q '^flowchart LR$' "$md" && ok "$md は flowchart" || ng "$md に flowchart が無い" ;;
    state) grep -q '^stateDiagram-v2$' "$md" && ok "$md は stateDiagram-v2" || ng "$md に stateDiagram-v2 が無い" ;;
  esac
done

# 5. ワークストリーム（受け入れ条件 5。ADR-0017 決めたこと 3・9・16）
#    8 本がそろう。中身のある見守り・仕分けは項目を全部持ち（record は自分の記録ファイルがある手順だけ）、残り 6 本は name・title・purpose だけ。
#    by: ai の step の skill が package/skills/<名>/SKILL.md の節（「## <n>.」の見出し）を指す。
want="intake launch observe receive relay retrospective triage watch"
have=$(find process/workstreams -name '*.yaml' -printf '%f\n' | sed 's/\.yaml$//' | sort | tr '\n' ' ' | sed 's/ $//')
[ "$have" = "$want" ] && ok "ワークストリームが 8 本ある" || ng "ワークストリームが 8 本でない: $have"
# full_check <JSON> → 決めたこと 16 の項目が欠けた所を 1 行 1 つ
full_check() {
  jq -r '
    (["triggers", "steps", "counts"][] as $k | select((.[$k] // []) | length == 0) | "\($k) が無い"),
    (.steps[]? | . as $s
      | (["input", "output"][] | select($s[.] == null) | "\($s.id): \(.) が無い"),
        (if $s.by == "machine" then (["command"][] | select($s[.] == null) | "\($s.id): \(.) が無い")
         elif $s.by == "ai" then (["skill", "decides"][] | select($s[.] == null) | "\($s.id): \(.) が無い")
         else empty end))' "$1"
}
# skill_check <JSON> → 在らない SKILL の節を 1 行 1 つ
skill_check() {
  jq -r '.steps[]? | select(.by == "ai") | .skill | if type == "array" then .[] else . end' "$1" | sort -u |
    while IFS= read -r ref; do
      name=${ref%%#*} sec=; [ "$name" != "$ref" ] && sec=${ref#*#}
      md=package/skills/$name/SKILL.md
      if [ ! -f "$md" ]; then echo "$ref: $md が無い"
      elif [ -n "$sec" ] && ! grep -qE "^## ${sec//./\\.}\. " "$md"; then echo "$ref: $md に「## $sec.」の節が無い"; fi
    done
}
for n in watch triage; do
  "$andon" process json "process/workstreams/$n.yaml" >"$tmp/ws.json"
  out=$(full_check "$tmp/ws.json")
  [ -z "$out" ] && ok "$n.yaml が決めたこと 16 の項目を全部持つ" || { ng "$n.yaml の項目が欠ける"; printf '     %s\n' "$out"; }
  out=$(skill_check "$tmp/ws.json")
  [ -z "$out" ] && ok "$n.yaml の skill が SKILL の節を指す" || { ng "$n.yaml の skill が在らない節を指す"; printf '     %s\n' "$out"; }
done
for n in intake launch observe receive relay retrospective; do
  keys=$("$andon" process json "process/workstreams/$n.yaml" | jq -c 'keys')
  [ "$keys" = '["name","purpose","title"]' ] && ok "$n.yaml は name・title・purpose だけ" || ng "$n.yaml のキーが name・title・purpose でない: $keys"
done
"$andon" process json process/workstreams/watch.yaml >"$tmp/ws.json"
while IFS='@' read -r filter desc; do
  [ -n "$filter" ] || continue
  jq -e "$filter" "$tmp/ws.json" >/dev/null && ok "見守り: $desc" || ng "見守りに無い: $desc"
done <<'WATCH'
any(.triggers[]; .kind == "notification" and .notification == "estimate")@きっかけに ADR-0014 の estimate の通知
any(.triggers[]; .kind == "interval")@きっかけに間隔
any(.steps[]; (.input // []) | index("report.plan"))@手順の入るものに report.json の plan[]（report.plan）
any(.steps[]; .by == "ai" and ((.decides // []) as $d | ["途中報告を書かせる", "分ける", "起こし直す", "延長する"] | all(. as $x | $d | index($x))))@AI の手順の decides に 途中報告を書かせる・分ける・起こし直す・延長する
any(.counts[]; startswith("flow.wait."))@数えるものに状態ごとの時間（flow.wait.<status>）
WATCH
"$andon" process json process/workstreams/triage.yaml >"$tmp/ws.json"
jq -e 'any(.triggers[]; .kind == "notification" and .notification == "review" and .start == "check")' "$tmp/ws.json" >/dev/null &&
  ok "仕分け: 通知 review から check（仕分けを確かめる）が始まる" || ng "仕分けに 通知 review → check のきっかけが無い"
# 壊すと落ちる
jq 'del(.counts)' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(full_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: counts が無い" || ng "counts が無いのに通る"
jq '(.steps[] | select(.by == "machine")) |= del(.command)' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(full_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: by machine に command が無い" || ng "command が無いのに通る"
jq '(.steps[] | select(.by == "ai")) |= (.skill = "chief-of-staff#99")' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(skill_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: 在らない SKILL の節" || ng "在らない SKILL の節が通る"
jq '(.steps[] | select(.by == "ai")) |= (.skill = "no-such-skill")' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(skill_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: 在らない SKILL" || ng "在らない SKILL が通る"

exit "$fail"
