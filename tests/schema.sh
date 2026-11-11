#!/usr/bin/env bash
# 約束の検査（docs/contract.md・schema/）。
#   1. schema/ の11本が schema の決まり（draft 2020-12・$id の版・description と識別子は英語）を守る。
#   2. contract.md の ```json の例が、属する節の schema を全部通る。
#   3. 例に知らないキーを足す・必須のキーを消すと落ちる。値や入れ子を壊した形も落ちる（下の表）。
#   5. command（andon issue・andon gate・andon delegate・andon dispatcher・andon measure）が書いた JSON が schema を通る（tests/andon-issue.sh・andon-gate.sh・andon-dispatcher.sh・andon-measure.sh に書かせる）。
# 道具は python3（標準ライブラリだけ。package/lib/andon/schema_check.py）と jq。
set -euo pipefail
cd "$(dirname "$0")/.."

check_py=package/lib/andon/schema_check.py
schema_dir="${ANDON_SCHEMA_DIR:-schema}"   # 自己検査（下の 4）が緩めた写しを指す
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail=0
ok()  { echo "ok   $1"; }
ng()  { echo "FAIL $1"; fail=1; [ -z "${ANDON_SCHEMA_DIR:-}" ] || exit 1; }   # 緩めた写しの自己検査は最初の FAIL で止める

# 節の番号 → schema（contract.md の冒頭の表と同じ）
schema_of() {
  case "$1" in
    2) echo issue-keys ;; 3) echo findings ;; 4) echo report ;; 5) echo question ;;
    6) echo notification ;; 7) echo gate ;; 11) echo project ;; 12) echo candidates ;; 13) echo measure ;; 14) echo corrections ;; 15) echo signal ;; *) return 1 ;;
  esac
}
expected="candidates corrections findings gate issue-keys measure notification project question report signal"

# 1. schema そのもの
actual="$(cd "$schema_dir" && ls -1 *.schema.json | sed 's/\.schema\.json$//' | sort | tr '\n' ' ' | sed 's/ $//')"
if [ "$actual" = "$expected" ]; then ok "schema/ は11本（$expected）"; else ng "schema/ の顔ぶれが違う: $actual"; fi
for name in $expected; do
  if out="$(python3 "$check_py" lint "$schema_dir/$name.schema.json" 2>&1)"; then
    ok "schema/$name の決まり"
  else
    ng "schema/$name の決まり"; printf '     %s\n' "$out"
  fi
done

# 版（ADR-0008 決めたこと 2）: task を issue に改めた5本は /2、改名した issue-keys は /1。task-keys は無い
for v in notification/2 report/2 question/2 gate/2 project/2 issue-keys/1 findings/1 candidates/1 measure/1 corrections/1 signal/1; do
  id="$(jq -r '."$id"' "$schema_dir/${v%/*}.schema.json")"
  if [ "$id" = "https://andon.example.com/schema/$v" ]; then ok "schema/${v%/*} の \$id は $v"; else ng "schema/${v%/*} の \$id が $v でない: $id"; fi
done
[ ! -e "$schema_dir/task-keys.schema.json" ] && ok "schema/task-keys.schema.json は無い" || ng "schema/task-keys.schema.json が残る"

# validate <schema 名> <instance のファイル> → 0 通る / 1 落ちる / 2 検査できない
validate() { python3 "$check_py" validate "$schema_dir/$1.schema.json" "$2" >"$tmp/why" 2>&1; }

expect() { # expect pass|fail <schema 名> <ファイル> <説明>
  local want=$1 name=$2 file=$3 desc=$4 rc=0
  validate "$name" "$file" || rc=$?
  if [ "$rc" -eq 2 ]; then ng "$desc（検査できない: $(head -1 "$tmp/why")）"; return; fi
  if [ "$want" = pass ] && [ "$rc" -eq 0 ]; then ok "$desc"
  elif [ "$want" = fail ] && [ "$rc" -eq 1 ]; then ok "$desc（落ちる: $(head -1 "$tmp/why")）"
  else ng "$desc（want $want）"; sed 's/^/     /' "$tmp/why"; fi
}

# 検査器そのもの（schema と instance を直に書く。終了コード ｜ schema ｜ instance ｜ 説明）
while IFS='|' read -r want sch inst desc; do
  case "$want" in ''|'#'*) continue ;; esac
  printf '%s\n' "$sch" >"$tmp/self.schema.json"; printf '%s\n' "$inst" >"$tmp/self.json"
  rc=0; python3 "$check_py" validate "$tmp/self.schema.json" "$tmp/self.json" >/dev/null 2>&1 || rc=$?
  if [ "$rc" -eq "$want" ]; then ok "検査器: $desc"; else ng "検査器: $desc（rc=$rc、want $want）"; fi
done <<'SELF'
2|{"type": "string", "format": "date"}|"x"|知らないキーワード（format）で止まる
0|{"type": "integer"}|1.0|integer は 1.0 を受ける（2020-12）
1|{"type": "integer"}|1.5|integer は 1.5 を拒む
1|{"type": "integer"}|true|integer は真偽値を拒む
1|{"type": "string", "pattern": "^[a-z.]+$"}|"main.py\n"|$ は末尾の改行の前に合わない（ECMA-262）
0|{"type": "string", "pattern": "^a[$]$"}|"a$"|文字クラスの中の $ は字のまま
0|{"type": "array", "uniqueKeys": ["id"]}|[{"id": "c1"}, {"id": "c2"}, {"x": 1}]|uniqueKeys: 値が違えば通る（キーの無い要素は数えない）
1|{"type": "array", "uniqueKeys": ["id"]}|[{"id": "c1"}, {"id": "c2"}, {"id": "c1"}]|uniqueKeys: 値が重なれば落ちる
0|{"type": "array", "uniqueKeys": ["id"]}|[{"id": 1}, {"id": true}]|uniqueKeys: 1 と true は重ならない（JSON の等しさ）
SELF

# schema の決まりのうち、ECMA-262 と Python で意味が割れる表現を pattern に使わないこと（使えば lint が落ちる）
for p in '^a.b$' '^[^/\s]+$' '^\d+$' '^\w+$' '^\bx$'; do
  jq -n --arg p "$p" '{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://andon.example.com/schema/x/1",
                       "description": "x", "type": "string", "pattern": $p}' >"$tmp/div.schema.json"
  if python3 "$check_py" lint "$tmp/div.schema.json" >/dev/null 2>&1; then ng "検査器: pattern $p が lint を通ってしまう"
  else ok "検査器: pattern $p は lint で落ちる"; fi
done

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
# 必須のキーは schema から読まず、ここで固定する（schema の必須を外す変更がテストからも消えないように）
declare -A required=(
  [issue-keys]=""
  [findings]="commit findings"
  [report]=""   # commit は repo のある課題でだけ必須（andon issue report・ready が確かめる。ADR-0006 の repo の無い調べる課題）
  [question]="kind question options"
  [notification]="at project kind detail"
  [gate]=""
  [gate-record]="at id pr head tier tier_reasons changed_lines checks github_checks review acceptance delegation decision reasons merged"
  [project]="version project repos"
  [candidates]="candidates"
  [measure]="project since until days value flow efficiency counter prs skipped"
  [corrections]="at corrections"
  [signal]="signals"
)
mutate() { # mutate <元> <jq の式> → 新しいファイルの名前を出す
  local out; out="$(mktemp "$tmp/m-XXXX.json")"
  jq -c "$2" "$1" >"$out"; echo "$out"
}
for key in "${!first[@]}"; do
  name="${key%-record}"; src="${first[$key]}"
  if [ "$key" != issue-keys ]; then   # 台帳の項目は台帳の形式に倣って知らないキーを許す
    expect fail "$name" "$(mutate "$src" '. + {"x_unknown": 1}')" "$key: 知らないキー x_unknown を足すと落ちる"
  fi
  for k in ${required[$key]}; do
    expect fail "$name" "$(mutate "$src" "del(.\"$k\")")" "$key: 必須のキー $k を消すと落ちる"
  done
done

# 入れ子の object の必須のキーと知らないキー（例の key ｜ object を指す jq の道 ｜ 必須のキー。これも schema から読まない）
while IFS='|' read -r key at keys; do
  case "$key" in ''|'#'*) continue ;; esac
  name="${key%-record}"; src="${first[$key]}"
  expect fail "$name" "$(mutate "$src" "$at += {\"x_unknown\": 1}")" "$key: $at に知らないキーを足すと落ちる"
  for k in $keys; do
    expect fail "$name" "$(mutate "$src" "del($at.\"$k\")")" "$key: $at の必須のキー $k を消すと落ちる"
  done
done <<'NESTED'
findings|.findings[0]|severity file line
report|.review_request|round limit commit previous reviewer extra
report|.waived[0]|severity file line reason
report|.acceptance[0]|id result evidence
report|.risk|one_way
report|.observation|hypothesis result evidence
report|.next[0]|to text
question|.options[0]|label pick
notification|.detail|report pr
gate|.repos[]|
gate-record|.review|state must_fix_open waived issue commit
gate-record|.checks[0]|cmd ok
gate-record|.github_checks|state names
gate-record|.acceptance|pass fail manual
gate-record|.delegation|merge ok
project|.project|title prefix
project|.delegation|version project
project|.chief_of_staff|
project|.delegation.grants[0]|id scope value by at evidence
project|.delegation.grants[1].revoked|at by reason
candidates|.candidates[0]|id text in_scope
measure|.value|observed days_to_observe no_hypothesis
measure|.value.observed|count hit miss unknown
measure|.value.days_to_observe|count days
measure|.flow|lead_time wait question_open closed wip_end
measure|.flow.lead_time|count hours
measure|.flow.wait.todo|count hours
measure|.flow.closed|count per_day
measure|.efficiency|merged_prs human_touches auto_merge gate
measure|.efficiency.human_touches|total per_pr
measure|.efficiency.auto_merge|count rate
measure|.efficiency.gate|records fix needs_human rate
measure|.counter|reverted follow_ups reopened corrected
measure|.counter.corrected|corrections by_tier
measure|.counter.corrected.by_tier|A B C none
measure|.counter.reopened|corrections by_tier
measure|.counter.reopened.by_tier|A B C none
measure|.prs[0]|issue pr merged_at tier auto human_touches lead_time_hours reverted corrections
measure|.skipped[0]|source reason
corrections|.corrections[0]|kind question decision_kind recommended picked
corrections|.corrections[1]|kind comments human_commits
corrections|.corrections[1].comments[0]|author at body
corrections|.corrections[1].human_commits[0]|sha headline files
corrections|.corrections[2]|kind finding reason
corrections|.corrections[2].finding|severity file line
corrections|.corrections[3]|kind question recommended answer
project|.signals[0]|source weight
project|.signals[2]|source weight
signal|.signals[0]|source project key url title weight fingerprint first_seen last_seen count active episodes delivered_episode
signal|.signals[1]|source project key url title weight fingerprint first_seen last_seen count active episodes delivered_episode
NESTED

# 3b. 値と入れ子（pass|fail ｜ 例の key ｜ jq の式 ｜ 説明。jq の式は | を含んでよい。説明は含まない）
while IFS='|' read -r want key rest; do
  case "$want" in ''|'#'*) continue ;; esac
  desc="${rest##*|}"; expr="${rest%|*}"
  name="${key%-record}"
  expect "$want" "$name" "$(mutate "${first[$key]}" "$expr")" "$key: $desc"
done <<'EOF'
pass|issue-keys|. + {"links": ["https://example.com"]}|台帳の形式のキーは通る
pass|issue-keys|{}|予約キーが1つも無くても通る（無いときの振る舞いは読み手が持つ）
fail|issue-keys|.review = "yes"|review が on/off でない
fail|issue-keys|.must_fix = "Major"|must_fix の綴りが大文字
fail|issue-keys|.review_rounds = 2|review_rounds が数（kanban set は文字列で書く）
fail|issue-keys|.review_rounds = "0"|review_rounds が 0
fail|issue-keys|.merge = "auto-merge"|merge が auto/person でない
fail|issue-keys|.reviewer = "claude"|reviewer がレビュアーのプロファイルでない
pass|issue-keys|.deliverable = "candidates"|deliverable が candidates（調べる課題）
fail|issue-keys|.deliverable = "survey"|deliverable が pr/candidates でない（型の名前は値でない）
pass|issue-keys|.estimate = "large"|estimate が large（見込みの段）
fail|issue-keys|.estimate = "中"|estimate が段の名（値は英字の small/medium/large）
fail|issue-keys|.estimate = "60"|estimate が分の数
fail|issue-keys|.deliverable = "Candidates"|deliverable の綴りが大文字
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
fail|findings|.findings[0].file = "src/main.py\n"|file の末尾に LF
fail|findings|.findings[0].file = "src/main.py\r"|file の末尾に CR
fail|findings|.findings[0].file = "/outside"|file が絶対パス
fail|findings|.findings[0].file = "../outside"|file が repo の外（..）
fail|findings|.findings[0].file = "src/../../outside"|file の途中に ..
pass|findings|.findings[0].file = "src/main.py"|file が repo からの相対パス
pass|findings|.findings[0].file = "..hidden/x"|.. で始まる名前は .. の段ではない
fail|findings|.findings[0].file = "\u2028/../../outside"|file の .. の段の前に行区切り文字 U+2028
fail|findings|.findings[0].file = "a/\u2028b/../../../outside"|file の途中の段に U+2028
pass|findings|.findings[0].preexisting = true|preexisting が true（課題の前からあった問題）
pass|findings|.findings[0].preexisting = false|preexisting が false
fail|findings|.findings[0].preexisting = "true"|preexisting が文字列
fail|findings|.findings[0].preexisting = null|preexisting が null
pass|report|{commit}|commit だけでも通る
pass|report|.observation = {"hypothesis": "返事の下書きボタンで返信が速くなる", "result": "hit", "evidence": ["signal:abc123"]}|observation（hit・証拠つき）
pass|report|.observation = {"hypothesis": "h", "result": "unknown", "evidence": []}|observation の unknown は証拠が空でも通る
fail|report|.observation = {"hypothesis": "h", "result": "maybe", "evidence": []}|observation.result が enum の外
fail|report|.observation = {"hypothesis": "", "result": "hit", "evidence": []}|observation.hypothesis が空
fail|report|.observation = {"hypothesis": "h", "result": "hit"}|observation に evidence が無い
fail|report|.observation = {"hypothesis": "h", "result": "hit", "evidence": [""]}|observation.evidence に空の行
pass|report|{}|commit 無し（repo の無い課題）でも通る
fail|report|.pr = "#12"|pr が URL でない
fail|report|.commit = "9c4e1b0"|commit が40桁でない
fail|report|.review_request.commit = "9c4e1b0"|review_request.commit が40桁でない
fail|report|del(.review_request.previous)|review_request に previous が無い
fail|report|.review_request.round = 0|review_request.round が 0
fail|report|.review_request += {"note": "x"}|review_request に知らないキー
pass|report|.review_request.previous = .commit|再レビューの previous が sha
fail|report|.review_request.previous = "HEAD~1"|previous が sha でも null でもない
fail|report|del(.waived[0].reason)|waived に reason が無い
fail|report|.waived[0].file = "../x"|waived の file が repo の外
fail|report|.waived[0].file = "\u2028/../../outside"|waived の file の .. の段の前に U+2028
fail|report|.pr = "https://github.com/\ufeffowner/andon/pull/1"|pr の owner に BOM（U+FEFF）
fail|report|.pr = "https://github.com/own er/andon/pull/1"|pr の owner に空白
fail|report|.pr = "https://github.com/\u00e9/andon/pull/1"|pr の owner に ASCII の外の字
fail|report|.acceptance[0].result = "ok"|acceptance の result が pass/fail/manual でない
pass|report|del(.risk)|risk の無い古い report.json
pass|report|.risk = {"one_way": false}|risk の one_way: false（impact 無し）
fail|report|.risk.one_way = "true"|risk.one_way が真偽でない
fail|report|.risk.impact = [""]|risk.impact に空の行
fail|report|.risk.impact = "x"|risk.impact が配列でない
fail|report|.next[0].to = "human"|next の to が知らない宛先
fail|report|.next[0].to = "task"|古い宛先 to: task（report/2 は issue）
fail|report|.remember = "x"|remember が配列でない
fail|question|.kind = "other"|知らない kind
pass|question|.kind = "scope" | .from = "issue"|kind scope（範囲の問い）
fail|question|.kind = "Scope"|kind の綴りが大文字
fail|question|.question = "a\nb"|question が2行
fail|question|.options = [.options[0]]|options が1つ
fail|question|.options[0].pick = false|先頭が推奨案でない
fail|question|.options |= [.[1], .[0]]|推奨案が先頭にない
fail|question|.options[1].pick = true|推奨案が2つ
pass|question|.options[1].pick = false|2つ目の pick: false は通る
fail|question|.options[0] += {"note": "x"}|選択肢に知らないキー
fail|question|.remaining = -1|remaining が負
fail|question|.from = "person"|from が知らない書き手
fail|question|.from = "task"|古い書き手 from: task（question/2 は issue）
pass|notification|.kind = "report" | .detail = {report: .detail.report}|kind report
pass|notification|.kind = "asked" | .detail = {n: 1, kind: "line", question: "x"}|kind asked
pass|notification|.kind = "asked" | .detail = {n: 1, kind: "scope", question: "x"}|kind asked（範囲の問い）
fail|notification|.kind = "asked" | .detail = {n: 1, kind: "other", question: "x"}|asked の kind が知らない値
pass|notification|.kind = "ready" | .detail = {title: "x"}|kind ready（routine 無し）
pass|notification|.kind = "review-requested" | .detail = {round: 1, reviewer: "codex-review", reviewer_issue: "A-13"}|kind review-requested
pass|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json"}|kind gate
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}|kind merged
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "person"}|kind merged（人がマージした）
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "person", held: "x"}|kind merged（人がマージしたが片付けていない）
fail|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", held: "x"}|merged の held に by が無い
fail|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "dispatcher"}|merged の by が person でない
pass|notification|.kind = "closed" | .detail = {pr: .detail.pr}|kind closed
fail|notification|.kind = "closed" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}|closed に知らないキー
pass|notification|.kind = "stalled" | .detail = {agent: "t7", status_ms: 1793845966681} | del(.issue)|kind stalled（issue 無し）
pass|notification|.kind = "impact" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", issues: [{issue: "A-14", conflict: false, overlap: [], brief_refs: ["docs/a.md"], error: "x"}]}|kind impact
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", issues: []}|impact の issues が空
pass|notification|.kind = "impact" | .detail = {pr: .detail.pr, error: "3 回打って失敗"}|kind impact（集められなかった）
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr}|impact に issues も error も無い
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr, error: "x", sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", issues: [{issue: "A-14", conflict: false, overlap: [], brief_refs: []}]}|impact に error と issues の両方
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", issues: [{issue: "A-14", conflict: false, overlap: []}]}|impact の issues の1件に brief_refs が無い
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", issues: [{issue: "A-14", conflict: false, overlap: [], brief_refs: [], screen: "x"}]}|impact の issues の1件に知らないキー
fail|notification|del(.detail.pr)|review の detail に pr が無い
fail|notification|.detail += {"text": "x"}|detail に知らないキー
fail|notification|.kind = "asked"|asked なのに detail が review の形
pass|notification|.kind = "corrections" | .detail = {error: "2 回打って失敗"}|kind corrections（集められなかった）
fail|notification|.kind = "corrections" | .detail = {error: ""}|corrections の error が空
pass|notification|.kind = "signal" | .detail = {source: "github-ci", key: "github.com/example/app/ci.yml@main", url: "https://github.com/example/app/actions/runs/1", title: "x", weight: "now"} | del(.issue)|kind signal（issue 無し）
fail|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", weight: "now"}|signal の detail に title が無い
fail|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", title: "x", weight: "now", count: 2}|signal の detail に知らないキー（記録のキー）
fail|notification|.kind = "signal" | .detail = {source: "datadog", key: "x", url: "https://example.com/x", title: "x", weight: "now"}|signal の source が知らない出所
fail|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", title: "a\nb", weight: "now"}|signal の title が2行
fail|notification|.at = "2026-11-04 09:00:00"|at が UTC の形でない
fail|notification|.at = "2026-02-30T09:00:00Z"|at が暦に無い日
fail|notification|.at = "2026-11-04T24:00:00Z"|at の時が 24
fail|notification|.issue = "../x"|issue が id の形でない
fail|notification|.task = .issue | del(.issue)|古いキー task（notification/2 は issue。ADR-0008）
fail|notification|.kind = "review-requested" | .detail = {round: 1, reviewer: "codex-review", reviewer_task: "A-13"}|古いキー reviewer_task
fail|notification|.kind = "impact" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", tasks: [{task: "A-14", conflict: false, overlap: [], brief_refs: []}]}|古いキー tasks・task（impact）
pass|gate|{}|条件表は空でも通る（gate.local.json）
fail|gate|.repos[] += {"auto": true}|条件表の repos に知らないキー
fail|gate|.repos[].merge = "fast-forward"|merge の方法が知らない値
fail|gate|.repos[].tier_overrides["AGENTS.md"] = "D"|tier_overrides の値が A/B/C でない
fail|gate|.size_review_lines = "500"|size_review_lines が文字列
fail|gate|.repos[] += {"github_checks": "off"}|github_checks が知らない値
pass|gate-record|.review = {state: "missing", must_fix_open: 0, waived: 0}|review が missing（issue・commit 無し）
pass|gate-record|.review = {state: "off", must_fix_open: 0, waived: 0}|review が off（issue・commit 無し）
pass|gate-record|.review.state = "stale"|review が stale（issue・commit あり）
fail|gate-record|del(.review.issue)|current なのに issue が無い
fail|gate-record|.review.task = .review.issue | del(.review.issue)|古いキー review.task（gate/2 は issue）
fail|gate-record|del(.review.commit)|current なのに commit が無い
fail|gate-record|.review.state = "stale" | del(.review.commit)|stale なのに commit が無い
fail|gate-record|.review.state = "missing"|missing なのに issue・commit がある
fail|gate-record|.review = {state: "off", issue: "A-13", must_fix_open: 0, waived: 0}|off なのに issue がある
fail|gate-record|.at = "2026-13-01T09:10:00Z"|at の月が 13
fail|gate-record|.review.state = "unknown"|review.state が知らない値
fail|gate-record|.decision = "approved"|decision が知らない値
pass|gate-record|.decision = "wait" | .github_checks = {state: "pending", names: ["test"]}|wait（andon gate の出力。記録には書かない）
fail|gate-record|.github_checks.state = "green"|github_checks.state が知らない値
fail|gate-record|.tier = "D"|tier が A/B/C でない
fail|gate-record|. + {"size_review_lines": 500}|記録に条件表のキーが混ざる
fail|gate-record|.checks[0] += {"exit": 0}|checks に知らないキー
pass|project|{version: 1, project: {title: "x", prefix: "S"}, repos: []}|repos が空・任意のキーが無くても通る
fail|project|del(.repos)|repos が無い
fail|project|.repos = "github.com/example/app"|repos が配列でない
fail|project|.repos[0] = "example/app"|repos の要素が <host>/<owner>/<repo> でない
fail|project|.references[0] = "ftp://example.com/x"|references が repo でも https の URL でもない
pass|project|.references = ["https://example.com/x"]|references に https の URL
fail|project|.project.prefix = "s"|prefix が英大文字でない
fail|project|.project.title = ""|title が空
fail|project|.version = 2|知らない version
fail|project|.delegation = null|delegation が null
fail|project|.delegation += {"x_unknown": 1}|delegation が今の委譲と違う形（知らないキー）
fail|project|del(.delegation.project)|delegation が今の委譲と違う形（project が無い）
fail|project|.delegation = {preset: "work", grants: []}|delegation が今の委譲と違う形（version・project が無い）
fail|project|.chief_of_staff.parallel = 0|parallel が 0
fail|project|.chief_of_staff.review_when = "外向きの面"|review_when が配列でない
fail|project|.chief_of_staff.triage_max_issues = "3"|triage_max_issues が文字列
fail|project|.chief_of_staff.triage_max_tasks = .chief_of_staff.triage_max_issues | del(.chief_of_staff.triage_max_issues)|古いキー triage_max_tasks（project/2 は triage_max_issues）
fail|project|.delegation.version = 2|委譲: 知らない version
pass|project|.signals = []|signals が空（何も取りに行かない）
pass|project|del(.signals)|signals が無い
pass|project|.signals = [{source: "github-ci", weight: "now"}]|signals: repo も任意のキーも無い出所
fail|project|.signals[0].weight = "soon"|signals: weight が now/later でない
fail|project|.signals[0].weight = "Now"|signals: weight の綴りが大文字
fail|project|.signals[0].source = "datadog"|signals: source が enum の外
fail|project|.signals[0].repo = "https://example.com/x"|signals: repo が <host>/<owner>/<repo> でない
fail|project|.signals[0].every = 0|signals: every が 0
fail|project|.signals[0].every = "10"|signals: every が文字列
fail|project|.signals[1].filter = ""|signals: filter が空
fail|project|.signals[1].write_back = "yes"|signals: write_back が真偽値でない
fail|project|.signals[2].org = null|signals: org が null
fail|project|.signals = {}|signals が配列でない
fail|project|.delegation.version = "1"|委譲: version が文字列
fail|project|.delegation.preset = "free"|委譲: 知らない preset
pass|project|.delegation = {version: 1, project: "sample-site"}|委譲: preset も grants も無くても通る（型 work）
fail|project|.delegation.grants[0].scope = "merge"|委譲: scope merge に publish の値
pass|project|.delegation.grants[0] |= (.scope = "merge" | .value = "B")|委譲: scope merge の値 B
pass|project|.delegation.grants[0] |= (.scope = "irreversible" | .value = ["delete-branch", "force-push"])|委譲: scope irreversible の配列
fail|project|.delegation.grants[0] |= (.scope = "irreversible" | .value = ["delete-remote"])|委譲: delete-remote は委譲できない
fail|project|.delegation.grants[0] |= (.scope = "profile" | .value = "true")|委譲: 真偽値の scope に文字列
fail|project|del(.delegation.grants[0].evidence)|委譲: grant に evidence が無い
fail|project|.delegation.grants[0].by = "chief-of-staff"|委譲: by が owner でない
fail|project|.delegation.grants[0].until = "2026/12/31"|委譲: until が日付の形でない
fail|project|.delegation.grants[0].at = "9999-99-99"|委譲: at が暦に無い（9999-99-99）
fail|project|.delegation.grants[0].at = "2026-02-30"|委譲: at が暦に無い（2月30日）
fail|project|.delegation.grants[0].at = "2026-13-01"|委譲: at の月が 13
fail|project|.delegation.grants[0].until = "2026-02-29"|委譲: until がうるう年でない年の 2月29日
pass|project|.delegation.grants[0].until = "2028-02-29"|委譲: until がうるう年の 2月29日
pass|project|.delegation.grants[0].until = "2000-02-29"|委譲: 2000 年はうるう年
fail|project|.delegation.grants[0].until = "2100-02-29"|委譲: 2100 年はうるう年でない
fail|project|.delegation.grants[1].revoked.at = "2026-11-31"|委譲: revoked.at が暦に無い（11月31日）
fail|project|del(.delegation.grants[1].revoked.reason)|委譲: revoked に reason が無い
fail|project|.delegation.grants[0].id = "g1"|委譲: grant の id が d<n> でない
fail|project|.delegation.grants[0] += {"note": "x"}|委譲: grant に知らないキー
pass|candidates|del(.commit)|commit 無し（repo の無い課題）
pass|candidates|.candidates = []|候補が無い
pass|candidates|.candidates[0] |= del(.file, .line)|場所を指さない候補
pass|candidates|.candidates[0].in_scope = false|in_scope が false
fail|candidates|.candidates[1].id = "c1"|id が重なる
fail|candidates|.candidates += [.candidates[0] | .text = "別の文"]|id が重なる（中身が違っても）
fail|candidates|.candidates[0].id = "1"|id が c<n> でない
fail|candidates|.candidates[0].id = "c0"|id の番号が 0
fail|candidates|.candidates[0].in_scope = "true"|in_scope が文字列
fail|candidates|.candidates[0].text = ""|text が空
fail|candidates|.candidates[0].text = "a\nb"|text が2行
fail|candidates|.candidates[0].line = 0|line が 0
fail|candidates|.candidates[0].file = "../outside"|file が repo の外（..）
fail|candidates|.candidates[0].file = "/outside"|file が絶対パス
fail|candidates|.candidates[0] += {"severity": "minor"}|候補に指摘のキー（findings と混ぜない）
fail|candidates|.commit = "9c4e1b0"|commit が40桁でない
fail|candidates|.commit = null|commit が null（repo の無い課題は書かない）
fail|candidates|.candidates = {}|candidates が配列でない
pass|measure|.prs = [] | .skipped = [] | .efficiency.auto_merge.rate = null | .flow.lead_time.hours = null|記録が無い（空の配列・率が null）
pass|measure|.skipped[0] |= del(.line)|飛ばした記録に line が無い（ファイル・repo）
pass|measure|.flow.wait = {}|待ち時間の状態が無い
fail|measure|.flow.wait.todo = 1|待ち時間の値が {count, hours} でない
fail|measure|.efficiency.auto_merge.rate = -0.5|率が負
fail|measure|.efficiency.auto_merge.rate = "50%"|率が文字列
fail|measure|.flow.wip_end = -1|WIP が負
fail|measure|.value.observed.hit = -1|仮説を確かめた数が負
fail|measure|.value.days_to_observe.days = "3"|見るまでの日数が文字列
fail|measure|.value.no_hypothesis = 1.5|仮説「無し」の数が整数でない
fail|measure|del(.value)|価値が無い
pass|measure|.value.days_to_observe = {count: 0, days: null}|見るまでの日数が null（観察が無い）
fail|measure|.flow.closed.count = 1.5|件数が整数でない
fail|measure|.days = 0|days が 0
fail|measure|.since = "2026/11/01"|since が YYYY-MM-DD でない
fail|measure|.counter.reopened.corrections = "unknown"|corrections が recorded/none でない
fail|measure|.prs[0].tier = "D"|tier が A/B/C/none でない
fail|measure|.prs[0].pr = "#1"|pr が URL でない
fail|measure|.skipped[0].line = 0|飛ばした行の line が 0
fail|measure|.skipped[0].reason = "a\nb"|飛ばした理由が2行
pass|corrections|.corrections = []|手直しが無い（空の配列）
fail|corrections|.corrections = null|corrections が null（集めていないなら書かない）
fail|corrections|.at = "2026-02-30T09:00:00Z"|at が暦に無い日
fail|corrections|.corrections[0].kind = "comment"|知らない kind（端末に送った文は拾わない）
fail|corrections|.corrections[0].decision_kind = "scope"|answer の decision_kind に scope（範囲の問いは Chief of Staff の答え）
fail|corrections|.corrections[0].question = 0|question が 0
fail|corrections|.corrections[0].picked = ""|picked が空
fail|corrections|.corrections[1].comments = [] | .corrections[1].human_commits = []|pr にコメントも commit も無い
pass|corrections|.corrections[1].comments = []|pr に commit だけ
pass|corrections|.corrections[1].human_commits = []|pr にコメントだけ
pass|corrections|.corrections[1].comments[0] += {file: "docs/a.md", line: 3}|インラインのレビューコメント（file・line つき）
fail|corrections|.corrections[1].comments[0].line = 0|コメントの line が 0
fail|corrections|.corrections[1].human_commits[0].sha = "9c4e1b0"|human_commits の sha が40桁でない
fail|corrections|.corrections[2].finding.severity = "blocker"|override の指摘に知らない severity
fail|corrections|.corrections[2].finding.line = 0|override の指摘の line が 0
fail|corrections|.corrections[2] += {"answer": "x"}|指摘の override に問いのキーが混ざる
fail|corrections|.corrections[3].kind = "answer"|問いの override を answer にすると形が合わない
pass|signal|.signals = []|まだ何も見ていない（空の配列）
pass|signal|del(.signals[0].release)|release が無い
fail|signal|.signals[1].fingerprint = .signals[0].fingerprint|fingerprint が重なる
fail|signal|.signals[0].fingerprint = "AF3C0A047C19A9C56867FE43AFE6794DD408EB8F"|fingerprint が大文字
fail|signal|.signals[0].fingerprint = "af3c0a0"|fingerprint が40桁でない
fail|signal|.signals[0].weight = "soon"|weight が now/later でない
fail|signal|.signals[0].source = "datadog"|source が enum の外
fail|signal|.signals[0].title = "a\nb"|title が2行
fail|signal|.signals[0].key = ""|key が空
fail|signal|.signals[0].url = "http://example.com/x"|url が https でない
fail|signal|.signals[0].count = 0|count が 0
fail|signal|.signals[0].episodes = 0|episodes が 0
fail|signal|.signals[0].delivered_episode = -1|delivered_episode が負
fail|signal|.signals[0].active = "yes"|active が真偽値でない
fail|signal|.signals[0].first_seen = "2026-02-30T09:00:00Z"|first_seen が暦に無い日
fail|signal|.signals[0].release = "9c4e1b0"|release が40桁でない
fail|signal|.signals[0] += {"body": "stack trace"}|本文を持つ（持たない）
fail|signal|.signals = {}|signals が配列でない
EOF

# 4. 自己検査: 制約を緩めた schema の写しでは、この検査全体が落ちること（期待値を schema から作っていない証し）
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  while IFS='|' read -r name expr desc; do
    case "$name" in ''|'#'*) continue ;; esac
    loose="$tmp/loose"; rm -rf "$loose"; cp -r schema "$loose"
    jq "$expr" "schema/$name.schema.json" >"$loose/$name.schema.json"
    if ANDON_SCHEMA_DIR="$loose" bash "$0" >/dev/null 2>&1; then ng "自己検査: $desc でも通ってしまう"
    else ok "自己検査: $desc と落ちる"; fi
  done <<'LOOSE'
findings|.required -= ["commit"]|findings の必須から commit を外す
findings|.additionalProperties = true|findings の知らないキーを許す
gate|."$defs".record.required -= ["decision"]|ゲートの記録の必須から decision を外す
report|."$defs".sha.pattern = "^[0-9a-f]+$"|sha の桁を緩める
project|."$defs".grant.required -= ["id"]|grant の必須から id を外す
project|.required -= ["repos"]|project の必須から repos を外す
project|."$defs".delegation.additionalProperties = true|delegation の知らないキーを許す
report|.properties.review_request.required -= ["previous"]|review_request の必須から previous を外す
findings|."$defs".finding.properties.file.pattern = "^[^\\u0000-\\u001f\\u007f]+$"|file の .. の段の検査を外す
candidates|del(.properties.candidates.uniqueKeys)|候補の id の一意を外す
candidates|."$defs".candidate.required -= ["in_scope"]|候補の必須から in_scope を外す
candidates|.additionalProperties = true|候補の一覧の知らないキーを許す
question|.properties.kind.enum -= ["scope"]|問いの kind から scope を外す
issue-keys|del(.properties.deliverable)|台帳の項目から deliverable を外す
measure|.required -= ["skipped"]|流れの数えの必須から skipped を外す
measure|."$defs".rate.anyOf[0].minimum = -1|率の負を許す
corrections|.required -= ["corrections"]|手直しの必須から corrections を外す
corrections|."$defs".pr.anyOf = [{}]|pr の「コメントか commit のどちらか」を外す
notification|.properties.kind.enum -= ["corrections"]|通知の kind から corrections を外す
notification|.properties.kind.enum -= ["signal"]|通知の kind から signal を外す
notification|."$defs".signal.additionalProperties = true|通知 signal の detail の知らないキーを許す
project|."$defs".signal.properties.weight.enum += ["soon"]|signals の weight を緩める
project|."$defs".signal.required -= ["source"]|signals の必須から source を外す
signal|del(.properties.signals.uniqueKeys)|信号の fingerprint の一意を外す
signal|."$defs".signal.required -= ["delivered_episode"]|信号の必須から delivered_episode を外す
signal|."$defs".signal.additionalProperties = true|信号の知らないキーを許す
LOOSE
fi

# 5. command の出力（tests/andon-issue.sh・andon-gate.sh・andon-dispatcher.sh・andon-measure.sh・andon-signal.sh が ANDON_EMIT_DIR に写したもの）。
#    台帳の CLI（kanban）は両方が tests/lib/kanban.sh で焼く。go が無くて両方が skip したときだけ skip
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  mkdir "$tmp/emit"
  skipped=0
  for t in andon-issue andon-gate andon-dispatcher andon-measure andon-signal; do
    if ! ANDON_EMIT_DIR="$tmp/emit" bash "tests/$t.sh" >"$tmp/emit.log" 2>&1; then ng "tests/$t.sh が落ちた"; sed 's/^/     /' "$tmp/emit.log" | tail -5
    elif grep -q '^skip: ' "$tmp/emit.log"; then skipped=1; fi
  done
  if [ "$skipped" = 1 ]; then
    echo "skip command の出力（$(grep '^skip: ' "$tmp/emit.log" | head -1)）"
  else
    while IFS='|' read -r file name desc; do
      case "$file" in ''|'#'*) continue ;; esac
      if [ -f "$tmp/emit/$file" ]; then expect pass "$name" "$tmp/emit/$file" "command の出力: $desc が schema/$name を通る"
      else ng "command の出力: $desc（$file）が無い"; fi
    done <<'EMIT'
report-1.json|report|andon issue report の report.json
report-review-request.json|report|andon issue review-request の report.json
issue-keys.json|issue-keys|andon issue show の settings
question.json|question|andon issue ask の questions/<n>.json
question-scope.json|question|andon issue ask の questions/<n>.json（kind: scope）
candidates.json|candidates|andon issue candidates の candidates.json
report-norepo.json|report|andon issue report の report.json（repo の無い課題。commit 無し）
candidates-norepo.json|candidates|andon issue candidates の candidates.json（repo の無い課題。commit 無し）
issue-keys-candidates.json|issue-keys|andon issue show の settings（deliverable: candidates）
gate-record.json|gate|andon gate の記録（approve）
gate-record-stale.json|gate|andon gate の記録（stale）
gate-record-wait.json|gate|andon gate の出力（wait。記録には書かない）
project.json|project|andon delegate grant の project.json
gate-record-merged.json|gate|andon dispatcher がマージしたあとのゲートの記録（merged）
question-gate.json|question|andon dispatcher が needs_human で出した問い（from: gate）
notification-review-requested.json|notification|andon dispatcher の通知 review-requested
notification-gate.json|notification|andon dispatcher の通知 gate
notification-merged.json|notification|andon dispatcher の通知 merged
notification-merged-person.json|notification|andon dispatcher の通知 merged（人がマージした）
notification-merged-held.json|notification|andon dispatcher の通知 merged（人がマージしたが片付けていない）
notification-closed.json|notification|andon dispatcher の通知 closed
notification-impact.json|notification|andon dispatcher の通知 impact（andon issue impact から）
measure.json|measure|andon measure --json の出力
measure-empty.json|measure|andon measure --json の出力（記録が無い台帳）
corrections-empty.json|corrections|andon dispatcher の corrections.json（手直し無し）
corrections-answer.json|corrections|andon dispatcher の corrections.json（answer）
signals.json|signal|andon signal collect の信号の記録
corrections-pr.json|corrections|andon dispatcher の corrections.json（pr）
corrections-pr-inline.json|corrections|andon dispatcher の corrections.json（pr。インラインのレビューコメント）
corrections-override.json|corrections|andon dispatcher の corrections.json（override）
notification-corrections.json|notification|andon dispatcher の通知 corrections（集められなかった）
notification-impact-error.json|notification|andon dispatcher の通知 impact（集められず諦めたとき）
notification-asked.json|notification|andon dispatcher の通知 asked
notification-report.json|notification|andon dispatcher が log.jsonl から拾った通知 report
notification-review.json|notification|andon dispatcher が log.jsonl から拾った通知 review
notification-ready.json|notification|andon dispatcher が ready の差から拾った通知 ready（routine つき）
notification-finished.json|notification|andon dispatcher の通知 finished（端末が無い）
notification-blocked.json|notification|andon dispatcher の通知 blocked
notification-error.json|notification|andon dispatcher の通知 error
notification-stalled.json|notification|andon dispatcher の通知 stalled
EMIT
  fi
fi

# 6. contract.md §8（端末の土台との約束）の表に Herdr App の置き場のラベルと人に端末を指す言い方がある（ADR-0002 受け入れ条件 4）。
# 端末のラベルは schema に入れず §8 の表だけで持つので、行があることをここで見る
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  sec8="$(awk '/^## 8\. /{on=1; next} /^## /{on=0} on' docs/contract.md)"
  row="$(grep '^| Herdr App の置き場のラベル |' <<<"$sec8" || true)"
  if grep -q '`herdr.space=<id>`・`herdr.tab=<id>`' <<<"$row" && grep -q '持ち主は Herdr App' <<<"$row" && grep -q 'dispatcher' <<<"$row" && grep -q '| Herdr App |$' <<<"$row"; then
    ok "contract.md §8: herdr.space・herdr.tab の行（持ち主 Herdr App・書き手 command と dispatcher・読み手 App）"
  else ng "contract.md §8 に herdr.space・herdr.tab の行が無い・形が違う"; fi
  if grep -q '^| 人に端末を指す言い方 | `<Space 名> / <tab 名>`' <<<"$sec8"; then ok "contract.md §8: 人に端末を指す言い方の行"
  else ng "contract.md §8 に人に端末を指す言い方の行が無い"; fi
fi

exit "$fail"
