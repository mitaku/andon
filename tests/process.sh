#!/usr/bin/env bash
# 流れと状態の定義（ADR-0017 受け入れ条件 1・2）。
#   1. schema/process/ の2本が schema の決まりを守る。
#   2. process/**/*.yaml と見本（tests/process/process/）が schema を通る（andon process json で JSON にして andon schema check へ）。
#      ワークストリームの triggers[].start と steps[].next[].id が steps[] に在る。
#   3. 見本を壊すと落ちる（知らないキー・enum の外・必須の欠け・やる者の種類に合わないキー・在らない id）。
#   3b. process/states/ の transitions[].once: true が on の「§7.2 #<n>」で dispatcher.md §7.2 の行を指す（受け入れ条件 4）。
#   4. andon process render --check: repo の docs/process/ と見本の生成物（tests/process/docs/process/）が作り直しても変わらない。
#      生成物に機械が読む塊（```json・```yaml）が無い。
#   5. ワークストリームが 9 本。見守り・仕分け・ふりかえり・起動・ダイジェストは決めたこと 16 の項目を全部持ち、skill が SKILL の節を指す。残り 4 本は name・title・purpose だけ（受け入れ条件 5。ふりかえりは ADR-0028 決めたこと 12、起動は ADR-0042 決めたこと 10・11、ダイジェストは ADR-0044 決めたこと 3・4）。
# 道具は python3（標準ライブラリだけ。schema の決まりは package/lib/andon/schema_check.py lint）と jq と andon（tests/lib/andon.sh）。
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
  "$andon" schema check "schema/process/$kind.schema.json" "$json" >"$tmp/why" 2>&1 || return 1
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
stray=$(find process -type f ! -name '*.yaml' 2>/dev/null | sort)
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
起動する|§7.2 を指さない
起動する（§7.2 #9）|在らない行を指す
起動する（§7.2 #2）|守りの欄が空の行を指す
BAD
jq -n '{transitions: [{from: "a", to: "b", on: "起動する（§7.2 #1）", by: "machine", once: true}, {from: "b", to: "c", on: "x", by: "ai"}]}' >"$tmp/once.json"
once_check "$tmp/once.json" "$tmp/rows-sample" >/dev/null && ok "§7.2 の行を指す once は通る（once で無い移り方は見ない）" || ng "§7.2 の行を指す once が落ちる"

# 4. 生成
if "$andon" process render --check . 2>"$tmp/why"; then ok "docs/process/ は process/ から作り直しても変わらない"
else ng "docs/process/ が process/ と食い違う（andon process render で作り直す）"; sed 's/^/     /' "$tmp/why"; fi
if "$andon" process render --check tests/process 2>"$tmp/why"; then ok "見本の生成物（tests/process/docs/process/）が変わらない"
else ng "見本の生成物が変わる"; sed 's/^/     /' "$tmp/why"; fi
# 生成した文書の語（ADR-0040 受け入れ条件 5）: やる者の kanban は「kanban の CLI」、入力と出力は「kanban（review）」の形で出る。
# 古い語が無いことは tests/words.sh が見る
if grep -q '| kanban の CLI | `kanban add` |' docs/process/issue.md && grep -q '`kanban（review）`' docs/process/triage.md \
  && grep -q '| kanban の CLI |' tests/process/docs/process/sample-item.md; then
  ok "生成した文書に「kanban の CLI」と「kanban（review）」が出る"
else ng "生成した文書に「kanban の CLI」か「kanban（review）」が無い（internal/process の byName と process/ の入出力の名前）"; fi
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
#    9 本がそろう。中身のある見守り・仕分け・ふりかえり・起動・ダイジェストは項目を全部持ち（record は自分の記録ファイルがある手順だけ）、残り 4 本は name・title・purpose だけ。
#    by: ai の step の skill が package/skills/<名>/SKILL.md の節（「## <n>.」の見出し）か、package/skills の下の手引きのファイル（<名>/playbooks/<x>.md）を指す。
want="digest intake launch observe receive relay retrospective triage watch"
have=$(find process/workstreams -name '*.yaml' -printf '%f\n' | sed 's/\.yaml$//' | sort | tr '\n' ' ' | sed 's/ $//')
[ "$have" = "$want" ] && ok "ワークストリームが 9 本ある" || ng "ワークストリームが 9 本でない: $have"
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
      case $ref in */*) [ -f "package/skills/$ref" ] || echo "$ref: package/skills/$ref が無い"; continue ;; esac
      name=${ref%%#*} sec=; [ "$name" != "$ref" ] && sec=${ref#*#}
      md=package/skills/$name/SKILL.md
      if [ ! -f "$md" ]; then echo "$ref: $md が無い"
      elif [ -n "$sec" ] && ! grep -qE "^## ${sec//./\\.}\. " "$md"; then echo "$ref: $md に「## $sec.」の節が無い"; fi
    done
}
for n in watch triage retrospective launch digest; do
  "$andon" process json "process/workstreams/$n.yaml" >"$tmp/ws.json"
  out=$(full_check "$tmp/ws.json")
  [ -z "$out" ] && ok "$n.yaml が決めたこと 16 の項目を全部持つ" || { ng "$n.yaml の項目が欠ける"; printf '     %s\n' "$out"; }
  out=$(skill_check "$tmp/ws.json")
  [ -z "$out" ] && ok "$n.yaml の skill が SKILL の節を指す" || { ng "$n.yaml の skill が在らない節を指す"; printf '     %s\n' "$out"; }
done
for n in intake observe receive relay; do
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
any(.steps[]; .by == "ai" and ((.decides // []) as $d | ["途中報告を書かせる", "分ける", "起動し直す", "延長する"] | all(. as $x | $d | index($x))))@AI の手順の decides に 途中報告を書かせる・分ける・起動し直す・延長する
any(.counts[]; startswith("flow.wait."))@数えるものに状態ごとの時間（flow.wait.<status>）
WATCH
# 仕分けと振り返りも PR の無い課題なので、報告は通知 report で届く（review の通知は来ない）
report_check='any(.triggers[]; .kind == "notification" and .notification == "report" and .start == "check")'
"$andon" process json process/workstreams/triage.yaml >"$tmp/ws.json"
jq -e "$report_check" "$tmp/ws.json" >/dev/null &&
  ok "仕分け: 通知 report から check（仕分けを確かめる）が始まる" || ng "仕分けに 通知 report → check のきっかけが無い"
jq '(.triggers[] | select(.start == "check")).notification = "review"' "$tmp/ws.json" >"$tmp/bad.json"
! jq -e "$report_check" "$tmp/bad.json" >/dev/null && ok "壊した定義は落ちる: 仕分けのきっかけが review" || ng "review の通知で仕分けを確かめる定義が通る"
while IFS='@' read -r filter desc; do
  [ -n "$filter" ] || continue
  jq -e "$filter" "$tmp/ws.json" >/dev/null && ok "仕分け: $desc" || ng "仕分けに無い: $desc"
done <<'TRIAGE'
(.steps[] | select(.id == "sort") | .decides) == ["close", "fix", "bundle", "person", "keep"]@仕分ける段の decides が close・fix・bundle・person・keep（triage skill の 5 値）
any(.steps[]; .id == "check" and ((.decides // []) | any(contains("andon triage verdict"))) and ((.output // []) | index("library/verdicts.json")))@確かめる段に、判断の閉じと課題の閉じを読んで andon triage verdict で書く手と、その記録 library/verdicts.json
any(.steps[]; .id == "apply" and ((.input // []) | index("library/verdicts.json")))@当てる段が Chief of Staff の判定（library/verdicts.json）を読む
TRIAGE
# 起動（ADR-0042 決めたこと 10・11）: 枠は andon waiting が数え、free が 0 なら起動せずに待つ。古い数え方（kanban の並行の上限）を入るものに書かない
"$andon" process json process/workstreams/launch.yaml >"$tmp/launch.json"
while IFS='@' read -r filter desc; do
  [ -n "$filter" ] || continue
  jq -e "$filter" "$tmp/launch.json" >/dev/null && ok "起動: $desc" || ng "起動に無い: $desc"
done <<'LAUNCH'
any(.steps[]; .by == "machine" and .command == "andon waiting --json" and any(.next[]; .id == "hold"))@枠を数える段が andon waiting --json で、枠が無ければ待つ段へ分かれる
any(.steps[]; .id == "hold" and .by == "ai" and ((.skill | if type == "array" then . else [.] end) | index("ask-one-by-one#3")))@待つ段が ask-one-by-one §3 の出す順を指す
any(.steps[]; .by == "machine" and .command == "andon issue start")@起動する段が andon issue start
any(.triggers[]; .kind == "notification" and .notification == "asked")@きっかけに通知 asked（オーナー待ちに入って枠が空く）
LAUNCH
old=$(grep -l '並行の上限' process/workstreams/*.yaml || true)
[ -z "$old" ] && ok "ワークストリームに古い数え方（並行の上限を kanban で数える）が無い" || ng "古い数え方が残る: $old"
printf '%s\n' '    input: [notification.ready, kanban(並行の上限)]' | grep -q '並行の上限' \
  && ok "古い数え方の検査が当たる場合がある（検査が見ている）" || ng "古い数え方の検査が何も見ていない"
"$andon" process json process/workstreams/retrospective.yaml >"$tmp/retro.json"
while IFS='@' read -r filter desc; do
  [ -n "$filter" ] || continue
  jq -e "$filter" "$tmp/retro.json" >/dev/null && ok "ふりかえり: $desc" || ng "ふりかえりに無い: $desc"
done <<RETRO
$report_check@通知 report から check（検収）が始まる
[.steps[] | select(.by == "machine") | .command] == ["andon retro collect", "andon retro patterns", "andon retro apply", "andon issue finish"]@機械の手順が collect → patterns → apply → finish
any(.steps[]; .command == "andon retro apply" and .once == true and .record == "library/retro-applied.json")@当てる手順は1回だけで、記録は library/retro-applied.json
any(.steps[]; .by == "ai" and ((.skill | if type == "array" then . else [.] end) | index("retro#3")))@AI の手順が retro skill の直す先の段（§3）を指す
RETRO
jq '(.triggers[] | select(.start == "check")).notification = "review"' "$tmp/retro.json" >"$tmp/bad.json"
! jq -e "$report_check" "$tmp/bad.json" >/dev/null && ok "壊した定義は落ちる: ふりかえりのきっかけが review" || ng "review の通知で検収する定義が通る"
# ダイジェスト（ADR-0044 決めたこと 3・4）: PR の無い課題なので、報告は通知 report で届く
"$andon" process json process/workstreams/digest.yaml >"$tmp/digest.json"
digest_report='any(.triggers[]; .kind == "notification" and .notification == "report" and .start == "check")'
while IFS='@' read -r filter desc; do
  [ -n "$filter" ] || continue
  jq -e "$filter" "$tmp/digest.json" >/dev/null && ok "ダイジェスト: $desc" || ng "ダイジェストに無い: $desc"
done <<DIGEST
$digest_report@通知 report から check（送った結果を見て片付ける）が始まる
[.steps[] | select(.by == "machine") | .command] == ["andon digest collect", "andon digest build", "andon digest send", "andon issue finish"]@機械の手順が collect → build → send → finish
any(.steps[]; .command == "andon digest send" and .once == true and .record == "library/digest/sent.json")@送る手順は1回だけで、記録は library/digest/sent.json
any(.steps[]; .id == "check" and ([.next[]?.id] == ["finish"]))@確かめた段からは片付けへだけ進む（送る段へ戻る輪が無い）
DIGEST
jq '(.steps[] | select(.id == "check")).next += [{"id": "send", "when": "送れていない"}]' "$tmp/digest.json" >"$tmp/bad.json"
! jq -e 'any(.steps[]; .id == "check" and ([.next[]?.id] == ["finish"]))' "$tmp/bad.json" >/dev/null && ok "壊した定義は落ちる: check から send へ戻る" || ng "check から send へ戻る定義が通る"
jq '(.triggers[] | select(.start == "check")).notification = "review"' "$tmp/digest.json" >"$tmp/bad.json"
! jq -e "$digest_report" "$tmp/bad.json" >/dev/null && ok "壊した定義は落ちる: ダイジェストのきっかけが review" || ng "review の通知で片付ける定義が通る"
# 壊すと落ちる
jq 'del(.counts)' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(full_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: counts が無い" || ng "counts が無いのに通る"
jq '(.steps[] | select(.by == "machine")) |= del(.command)' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(full_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: by machine に command が無い" || ng "command が無いのに通る"
jq '(.steps[] | select(.by == "ai")) |= (.skill = "chief-of-staff#99")' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(skill_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: 在らない SKILL の節" || ng "在らない SKILL の節が通る"
jq '(.steps[] | select(.by == "ai")) |= (.skill = "no-such-skill")' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(skill_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: 在らない SKILL" || ng "在らない SKILL が通る"
jq '(.steps[] | select(.by == "ai")) |= (.skill = "chief-of-staff/playbooks/no-such.md")' "$tmp/ws.json" >"$tmp/bad.json"; [ -n "$(skill_check "$tmp/bad.json")" ] && ok "壊した定義は落ちる: 在らない手引き" || ng "在らない手引きが通る"

exit "$fail"
