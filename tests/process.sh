#!/usr/bin/env bash
# 流れと状態の定義（ADR-0017 受け入れ条件 1・2）。
#   1. schema/process/ の2本が schema の決まりを守る。
#   2. process/**/*.yaml と見本（tests/process/process/）が schema を通る（andon process json で JSON にして schema_check.py へ）。
#      ワークストリームの triggers[].start と steps[].next[].id が steps[] に在る。
#   3. 見本を壊すと落ちる（知らないキー・enum の外・必須の欠け・やる者の種類に合わないキー・在らない id）。
#   4. andon process render --check: repo の docs/process/ と見本の生成物（tests/process/docs/process/）が作り直しても変わらない。
#      生成物に機械が読む塊（```json・```yaml）が無い。
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

exit "$fail"
