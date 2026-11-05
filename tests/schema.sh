#!/usr/bin/env bash
# 約束の検査（docs/contract.md・schema/）。
#   1. schema/ の7本が schema の決まり（draft 2020-12・$id の版・description と識別子は英語）を守る。
#   2. contract.md の ```json の例が、属する節の schema を全部通る。
#   3. 例に知らないキーを足す・必須のキーを消すと落ちる。値や入れ子を壊した形も落ちる（下の表）。
# 道具は python3（標準ライブラリだけ。tests/lib/schema_check.py）と jq。
set -euo pipefail
cd "$(dirname "$0")/.."

check_py=tests/lib/schema_check.py
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail=0
ok()  { echo "ok   $1"; }
ng()  { echo "FAIL $1"; fail=1; }

# 節の番号 → schema（contract.md の冒頭の表と同じ）
schema_of() {
  case "$1" in
    2) echo task-keys ;; 3) echo findings ;; 4) echo report ;; 5) echo question ;;
    6) echo notification ;; 7) echo gate ;; 11) echo delegation ;; *) return 1 ;;
  esac
}
expected="delegation findings gate notification question report task-keys"

# 1. schema そのもの
actual="$(cd schema && ls -1 *.schema.json | sed 's/\.schema\.json$//' | sort | tr '\n' ' ' | sed 's/ $//')"
if [ "$actual" = "$expected" ]; then ok "schema/ は7本（$expected）"; else ng "schema/ の顔ぶれが違う: $actual"; fi
for name in $expected; do
  if out="$(python3 "$check_py" lint "schema/$name.schema.json" 2>&1)"; then
    ok "schema/$name の決まり"
  else
    ng "schema/$name の決まり"; printf '     %s\n' "$out"
  fi
done

# validate <schema 名> <instance のファイル> → 0 通る / 1 落ちる / 2 検査できない
validate() { python3 "$check_py" validate "schema/$1.schema.json" "$2" >"$tmp/why" 2>&1; }

expect() { # expect pass|fail <schema 名> <ファイル> <説明>
  local want=$1 name=$2 file=$3 desc=$4 rc=0
  validate "$name" "$file" || rc=$?
  if [ "$rc" -eq 2 ]; then ng "$desc（検査できない: $(head -1 "$tmp/why")）"; return; fi
  if [ "$want" = pass ] && [ "$rc" -eq 0 ]; then ok "$desc"
  elif [ "$want" = fail ] && [ "$rc" -eq 1 ]; then ok "$desc（落ちる: $(head -1 "$tmp/why")）"
  else ng "$desc（want $want）"; sed 's/^/     /' "$tmp/why"; fi
}

# 検査器が実装していないキーワード（例 format）を黙って飛ばさず、検査できない（2）で止まること
printf '%s\n' '{"type": "string", "format": "date"}' >"$tmp/unknown-keyword.schema.json"
printf '%s\n' '"x"' >"$tmp/x.json"
rc=0; python3 "$check_py" validate "$tmp/unknown-keyword.schema.json" "$tmp/x.json" >/dev/null 2>&1 || rc=$?
if [ "$rc" -eq 2 ]; then ok "検査器は知らないキーワードで止まる"; else ng "検査器が知らないキーワードを飛ばした（rc=$rc）"; fi

# 2. contract.md の例
python3 "$check_py" examples docs/contract.md >"$tmp/examples.tsv"
declare -A first=()   # schema 名 → 最初の例のファイル（3 の元）
i=0
while IFS=$'\t' read -r sec json; do
  i=$((i + 1))
  if ! name="$(schema_of "$sec")"; then ng "contract.md §$sec の例 $i に schema が無い"; continue; fi
  f="$tmp/ex-$i.json"; printf '%s\n' "$json" >"$f"
  expect pass "$name" "$f" "contract.md §$sec の例 $i が schema/$name を通る"
  # gate は条件表と記録の2つ。記録（decision を持つ）を gate-record として別に控える
  key="$name"; if [ "$name" = gate ] && jq -e 'has("decision")' "$f" >/dev/null; then key=gate-record; fi
  [ -n "${first[$key]:-}" ] || first[$key]="$f"
done <"$tmp/examples.tsv"
for key in $expected gate-record; do
  [ -n "${first[$key]:-}" ] || ng "contract.md に schema/$key の例が無い"
done

# 3a. 知らないキー・無いキー（例から機械的に作る）
mutate() { # mutate <元> <jq の式> → 新しいファイルの名前を出す
  local out; out="$(mktemp "$tmp/m-XXXX.json")"
  jq -c "$2" "$1" >"$out"; echo "$out"
}
for key in "${!first[@]}"; do
  name="${key%-record}"; src="${first[$key]}"
  if [ "$key" != task-keys ]; then   # 台帳の項目は台帳の形式に倣って知らないキーを許す
    expect fail "$name" "$(mutate "$src" '. + {"x_unknown": 1}')" "$key: 知らないキー x_unknown を足すと落ちる"
  fi
  req_path='.required'; [ "$key" = gate-record ] && req_path='."$defs".record.required'
  for k in $(jq -r "($req_path // [])[]" "schema/$name.schema.json"); do
    expect fail "$name" "$(mutate "$src" "del(.\"$k\")")" "$key: 必須のキー $k を消すと落ちる"
  done
done

# 3b. 値と入れ子（pass|fail ｜ 例の key ｜ jq の式 ｜ 説明。jq の式は | を含んでよい。説明は含まない）
while IFS='|' read -r want key rest; do
  case "$want" in ''|'#'*) continue ;; esac
  desc="${rest##*|}"; expr="${rest%|*}"
  name="${key%-record}"
  expect "$want" "$name" "$(mutate "${first[$key]}" "$expr")" "$key: $desc"
done <<'EOF'
pass|task-keys|. + {"links": ["https://example.com"]}|台帳の形式のキーは通る
pass|task-keys|{}|予約キーが1つも無くても通る（無いときの振る舞いは読み手が持つ）
fail|task-keys|.review = "yes"|review が on/off でない
fail|task-keys|.must_fix = "Major"|must_fix の綴りが大文字
fail|task-keys|.review_rounds = 2|review_rounds が数（kanban set は文字列で書く）
fail|task-keys|.review_rounds = "0"|review_rounds が 0
fail|task-keys|.merge = "auto-merge"|merge が auto/person でない
fail|task-keys|.reviewer = "claude"|reviewer がレビュアーのプロファイルでない
pass|findings|.findings = []|指摘が無い
fail|findings|.commit = "2a17f63"|commit が40桁でない
fail|findings|.commit |= ascii_upcase|commit が大文字
fail|findings|.findings[0].line = 0|line が 0
fail|findings|.findings[0].line = "120"|line が文字列
fail|findings|.findings[0].severity = "blocker"|知らない severity
fail|findings|.findings[0].file = ""|file が空
fail|findings|.findings[0].file = "a\u0001b"|file に制御文字
fail|findings|.findings[0] += {"title": "x"}|指摘に知らないキー
fail|findings|del(.findings[0].line)|指摘に line が無い
pass|report|{commit}|commit だけでも通る
fail|report|.pr = "#12"|pr が URL でない
fail|report|del(.review_request.previous)|review_request に previous が無い
fail|report|.review_request.round = 0|review_request.round が 0
fail|report|.review_request += {"note": "x"}|review_request に知らないキー
pass|report|.review_request.previous = .commit|再レビューの previous が sha
fail|report|.review_request.previous = "HEAD~1"|previous が sha でも null でもない
fail|report|del(.waived[0].reason)|waived に reason が無い
fail|report|.acceptance[0].result = "ok"|acceptance の result が pass/fail/manual でない
fail|report|.next[0].to = "human"|next の to が知らない宛先
fail|report|.remember = "x"|remember が配列でない
fail|question|.kind = "other"|知らない kind
fail|question|.question = "a\nb"|question が2行
fail|question|.options = [.options[0]]|options が1つ
fail|question|.options[0].pick = false|先頭が推奨案でない
fail|question|.options |= [.[1], .[0]]|推奨案が先頭にない
fail|question|.options[1].pick = true|推奨案が2つ
pass|question|.options[1].pick = false|2つ目の pick: false は通る
fail|question|.options[0] += {"note": "x"}|選択肢に知らないキー
fail|question|.remaining = -1|remaining が負
fail|question|.from = "person"|from が知らない書き手
pass|notification|.kind = "report" | .detail = {report: .detail.report}|kind report
pass|notification|.kind = "asked" | .detail = {n: 1, kind: "line", question: "x"}|kind asked
pass|notification|.kind = "ready" | .detail = {title: "x"}|kind ready（routine 無し）
pass|notification|.kind = "review-requested" | .detail = {round: 1, reviewer: "codex-review", reviewer_task: "A-13"}|kind review-requested
pass|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json"}|kind gate
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}|kind merged
pass|notification|.kind = "stalled" | .detail = {agent: "t7", status_ms: 1793845966681} | del(.task)|kind stalled（task 無し）
fail|notification|del(.detail.pr)|review の detail に pr が無い
fail|notification|.detail += {"text": "x"}|detail に知らないキー
fail|notification|.kind = "asked"|asked なのに detail が review の形
fail|notification|.kind = "signal"|signal はまだ schema に無い
fail|notification|.at = "2026-11-04 09:00:00"|at が UTC の形でない
fail|notification|.task = "../x"|task が id の形でない
pass|gate|{}|条件表は空でも通る（gate.local.json）
fail|gate|.repos[] += {"auto": true}|条件表の repos に知らないキー
fail|gate|.repos[].merge = "fast-forward"|merge の方法が知らない値
fail|gate|.repos[].tier_overrides["AGENTS.md"] = "D"|tier_overrides の値が A/B/C でない
fail|gate|.size_review_lines = "500"|size_review_lines が文字列
pass|gate-record|.review = {state: "missing", must_fix_open: 0, waived: 0}|review が missing（task・commit 無し）
fail|gate-record|.review.state = "unknown"|review.state が知らない値
fail|gate-record|.decision = "approved"|decision が知らない値
fail|gate-record|.tier = "D"|tier が A/B/C でない
fail|gate-record|. + {"size_review_lines": 500}|記録に条件表のキーが混ざる
fail|gate-record|.checks[0] += {"exit": 0}|checks に知らないキー
fail|delegation|.version = 2|知らない version
fail|delegation|.version = "1"|version が文字列
fail|delegation|.preset = "free"|知らない preset
pass|delegation|{version: 1, project: "sample-site"}|preset も grants も無くても通る（型 work）
fail|delegation|.grants[0].scope = "merge"|scope merge に publish の値
pass|delegation|.grants[0] |= (.scope = "merge" | .value = "B")|scope merge の値 B
pass|delegation|.grants[0] |= (.scope = "irreversible" | .value = ["delete-branch", "force-push"])|scope irreversible の配列
fail|delegation|.grants[0] |= (.scope = "irreversible" | .value = ["delete-remote"])|delete-remote は委譲できない
fail|delegation|.grants[0] |= (.scope = "profile" | .value = "true")|真偽値の scope に文字列
fail|delegation|del(.grants[0].evidence)|grant に evidence が無い
fail|delegation|.grants[0].by = "chief-of-staff"|by が owner でない
fail|delegation|.grants[0].until = "2026/12/31"|until が日付の形でない
fail|delegation|del(.grants[1].revoked.reason)|revoked に reason が無い
fail|delegation|.grants[0].id = "g1"|grant の id が d<n> でない
fail|delegation|.grants[0] += {"note": "x"}|grant に知らないキー
EOF

exit "$fail"
