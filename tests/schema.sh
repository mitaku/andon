#!/usr/bin/env bash
# 約束の検査（docs/contract.md・schema/）。
#   1. schema/ の21本と、schema/ledger/ のうち約束の節（contract.md §22・§23）を持つ2本（log.v3・ready.v3）が
#      schema の決まり（draft 2020-12・$id のバージョン・description と識別子は英語）を守る。
#   2. contract.md の ```json の例が、属する節の schema を全部通る。
#   3. 例に知らないキーを足す・必須のキーを消すと落ちる。値や入れ子を壊した形も落ちる（下の表）。
#   4. 制約を緩めた schema の写しでは、この検査全体が落ちる（期待値を schema から作っていない証し）。
#   5. command（andon issue・andon gate・andon delegate・andon dispatcher・andon measure・andon retro collect と patterns・andon signal・andon chief-engineer・andon chief-of-staff・andon waiting・andon digest）と
#      kanban の CLI が書いた JSON が schema を通る（下の emitters の検査が ANDON_EMIT_DIR に写す。*.jsonl は1行ずつ）。
#      tests/lib/run-tests.sh が同じ回で emitters の検査を走らせたときは、その1度目の実行が写したもの（ANDON_EMITTED）を読み、回し直さない。
#      ANDON_EMITTED に結果の無い検査（単独で bash tests/schema.sh を打った・run-tests.sh に渡されていない）は、ここで走らせて写させる。
#   6. contract.md §8（hc との約束）の表に、Herdr App の置き場のラベルとオーナーに端末を指す言い方がある。
# 道具は andon（tests/lib/andon.sh。照合は andon schema。command が書く前に照らすのと同じ検査器）と、
# python3（標準ライブラリだけ。package/lib/andon/schema_check.py。schema そのものの決まりの lint と、文書の例の取り出し）と jq。
# 速さ: 検査は1件ずつ起動せず、queue で溜めて flush で andon schema batch（照合）と schema_check.py batch（lint）に1回ずつ渡す。
set -euo pipefail
cd "$(dirname "$0")/.."

# command の出力を写す検査（下の 5）。tests/lib/run-tests.sh は --emitters でこの並びを読み、先に走らせて写させる
emitters=(andon-issue andon-gate andon-dispatcher andon-dispatcher-signal andon-measure andon-retro andon-retro-apply andon-signal andon-chief-of-staff andon-waiting andon-digest)
if [ "${1:-}" = --emitters ]; then printf 'tests/%s.sh\n' "${emitters[@]}"; exit 0; fi

check_py=package/lib/andon/schema_check.py
schema_dir="${ANDON_SCHEMA_DIR:-schema}"   # 自己検査（下の 4）が緩めた写しを指す
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
. tests/lib/andon.sh
export ANDON_BIN="$andon"   # 自己検査（下の 4）と emitters の検査が焼き直さない

fail=0
ok()  { echo "ok   $1"; }
ng()  { echo "FAIL $1"; fail=1; [ -z "${ANDON_SCHEMA_DIR:-}" ] || exit 1; }   # 緩めた写しの自己検査は最初の FAIL で止める

# queue <終了コードの期待 0|1|2> <理由を出すか 0|1> <説明> <validate|lint> <schema> [<instance>]: 検査を溜める
# flush: 溜めた検査を、照合（validate）は andon schema batch、lint は schema_check.py batch に1回ずつ渡し、
#        溜めた順に ok / FAIL を出す（期待は呼び出し側が固定する。flush が見るのは終了コードだけ）
q_cases="$tmp/q.tsv"; q_meta="$tmp/q.meta"; : >"$q_cases"; : >"$q_meta"
queue() {
  local want=$1 show=$2 desc=$3; shift 3
  printf '%s\t%s\t%s\n' "$want" "$show" "$desc" >>"$q_meta"
  local IFS=$'\t'; printf '%s\n' "$*" >>"$q_cases"
}
flush() {
  [ -s "$q_cases" ] || return 0
  awk -F'\t' -v v="$tmp/q.v" -v l="$tmp/q.l" 'BEGIN { printf "" >v; printf "" >l }
    $1 == "validate" { print $2 "\t" $3 >v; next } { print >l }' "$q_cases"
  "$andon" schema batch "$tmp/q.v" >"$tmp/q.vout"
  : >"$tmp/q.lout"; [ ! -s "$tmp/q.l" ] || python3 "$check_py" batch "$tmp/q.l" >"$tmp/q.lout"
  # 溜めた順に並べ直す（照合と lint の結果をそれぞれの順に読む）
  local kind _rest line
  : >"$tmp/q.out"
  exec 5<"$tmp/q.vout" 6<"$tmp/q.lout"
  while IFS=$'\t' read -r kind _rest; do
    if [ "$kind" = validate ]; then IFS= read -r -u 5 line || line=''; else IFS= read -r -u 6 line || line=''; fi
    [ -z "$line" ] || printf '%s\n' "$line" >>"$tmp/q.out"
  done <"$q_cases"
  exec 5<&- 6<&-
  local n_in n_out; n_in="$(wc -l <"$q_cases")"; n_out="$(wc -l <"$tmp/q.out")"
  if [ "$n_in" -ne "$n_out" ]; then ng "検査器の batch が $n_in 件に $n_out 件しか返さない"; : >"$q_cases"; : >"$q_meta"; return; fi
  local want show desc rc why
  while IFS=$'\t' read -r -u 3 want show desc && IFS=$'\t' read -r -u 4 rc why; do
    if [ "$rc" -eq 2 ] && [ "$want" -ne 2 ]; then ng "$desc（検査できない: $why）"
    elif [ "$rc" -eq "$want" ]; then
      if [ "$show" = 1 ] && [ "$rc" -eq 1 ]; then ok "$desc（落ちる: $why）"; else ok "$desc"; fi
    else
      ng "$desc（rc=$rc、want $want）"; [ -z "$why" ] || printf '     %s\n' "$why"
    fi
  done 3<"$q_meta" 4<"$tmp/q.out"
  : >"$q_cases"; : >"$q_meta"
}

# 節の番号 → schema（contract.md の冒頭の表と同じ）
schema_of() {
  case "$1" in
    2) echo issue-keys ;; 3) echo findings ;; 4) echo report ;; 5) echo question ;;
    6) echo notification ;; 7) echo gate ;; 11) echo project ;; 12) echo candidates ;; 13) echo measure ;; 14) echo corrections ;; 15) echo signal ;; 17) echo delivered ;; 18) echo launch ;; 19) echo outcome ;; 20) echo retro ;; 21) echo patterns ;;
    22) echo ledger-log ;; 23) echo ledger-ready ;; 24) echo run ;; 25) echo waiting ;;
    26) echo digest-input ;; 27) echo digest ;; 28) echo digest-sent ;; *) return 1 ;;
  esac
}
expected="candidates corrections delivered digest digest-input digest-sent findings gate issue-keys launch measure notification outcome patterns project question report retro run signal waiting"
# kanban の形式（schema/ledger/）のうち、約束の節を持つもの。名前 → schema/ のファイル（.schema.json を除く）
ledger="ledger-log ledger-ready"
schema_file() {
  case "$1" in ledger-log) echo ledger/log.v3 ;; ledger-ready) echo ledger/ready.v3 ;; *) echo "$1" ;; esac
}

# 1. schema そのもの
actual="$(cd "$schema_dir" && ls -1 *.schema.json | sed 's/\.schema\.json$//' | sort | tr '\n' ' ' | sed 's/ $//')"
if [ "$actual" = "$expected" ]; then ok "schema/ は21本（$expected）"; else ng "schema/ の顔ぶれが違う: $actual"; fi
actual="$(cd "$schema_dir/ledger" && ls -1 *.schema.json | sed 's/\.schema\.json$//' | sort | tr '\n' ' ' | sed 's/ $//')"
if [ "$actual" = "config.v3 issue.v3 log.v3 questions.v3 ready.v3 routine.v3" ]; then ok "schema/ledger/ は6本（$actual）"; else ng "schema/ledger/ の顔ぶれが違う: $actual"; fi
for name in $expected $ledger; do
  queue 0 0 "schema/$(schema_file "$name") の決まり" lint "$schema_dir/$(schema_file "$name").schema.json"
done
flush

# バージョン（ADR-0008 決めたこと 2）: task を issue に改めた5本は /2、改名した issue-keys は /1。task-keys は無い
# kanban の行と ready の2本は kanban の形式のバージョン（3）
for v in notification/2 report/2 question/2 gate/2 project/2 issue-keys/1 findings/1 candidates/1 measure/2 corrections/1 signal/1 delivered/1 launch/1 outcome/1 retro/2 patterns/1 run/1 waiting/1 digest/1 digest-input/1 digest-sent/1 ledger-log/3 ledger-ready/3; do
  f="$(schema_file "${v%/*}")"
  id="$(jq -r '."$id"' "$schema_dir/$f.schema.json")"
  if [ "$id" = "https://andon.example.com/schema/$v" ]; then ok "schema/$f の \$id は $v"; else ng "schema/$f の \$id が $v でない: $id"; fi
done
[ ! -e "$schema_dir/task-keys.schema.json" ] && ok "schema/task-keys.schema.json は無い" || ng "schema/task-keys.schema.json が残る"

# validate <schema 名> <instance のファイル> → 0 通る / 1 落ちる / 2 検査できない
expect() { # expect pass|fail <schema 名> <ファイル> <説明>（溜めるだけ。flush で走る）
  local want=0; [ "$1" = pass ] || want=1
  queue "$want" 1 "$4" validate "$schema_dir/$(schema_file "$2").schema.json" "$3"
}

# 検査器そのもの（schema と instance を直に書く。終了コード ｜ schema ｜ instance ｜ 説明）
self=0
while IFS='|' read -r want sch inst desc; do
  case "$want" in ''|'#'*) continue ;; esac
  self=$((self + 1))
  printf '%s\n' "$sch" >"$tmp/self-$self.schema.json"; printf '%s\n' "$inst" >"$tmp/self-$self.json"
  queue "$want" 0 "検査器: $desc" validate "$tmp/self-$self.schema.json" "$tmp/self-$self.json"
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

# schema の決まりのうち、ECMA-262・Python・Go で意味が割れるか読めない表現を pattern に使わないこと（使えば lint が落ちる）
for p in '^a.b$' '^[^/\s]+$' '^\d+$' '^\w+$' '^\bx$' '^(?!/)x$' '^(?=x)x$' '^(?<=x)y$' '^(?<!x)y$' '^(a)\1$' '^[^\u0000-\u001f]*$'; do
  self=$((self + 1))
  jq -n --arg p "$p" '{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://andon.example.com/schema/x/1",
                       "description": "x", "type": "string", "pattern": $p}' >"$tmp/div-$self.schema.json"
  queue 1 0 "検査器: pattern $p は lint で落ちる" lint "$tmp/div-$self.schema.json"
done
flush

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
flush
for key in $expected $ledger gate-record; do
  [ -n "${first[$key]:-}" ] || ng "contract.md に schema/$(schema_file "$key") の例が無い"
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
  [measure]="project since until days value flow efficiency counter news prs estimate issues tokens skipped"
  [corrections]="at corrections"
  [signal]="signals"
  [delivered]="at project acked news"
  [launch]="at role launch issue profile template command terminal session harness package workspace repo external drift drift_reasons memory_index"
  [outcome]="at issue result pr merge_commit merged_at gates reviewers worked_minutes sessions"
  [retro]="project at since until previous_retro boundary issues summaries cut typed fixes redacted skipped"
  [patterns]="retro at since until days previous_retro patterns fixes"
  [run]="id command expect exit_code started_at finished_at output output_bytes truncated"
  [waiting]="version at project slots items"
  [digest-input]="version project issue title at since until previous truncated issues waiting concerns numbers next redacted skipped"
  [digest]="headline themes concerns"
  [digest-sent]="issue since until destinations"
  [ledger-log]="at by id change"   # 最初の例は課題の行。ほかの形の行は 3b で作る
  [ledger-ready]=""                # 並び。課題の必須（id・status）は 3b で消す
)
# gen <pass|fail> <例の key> <jq の式> <説明>: 例を式で壊した（変えた）ものの検査を溜める。
# 式は例の key ごとに1本の jq にまとめて走らせる（式1つに1回 jq を呼ばない）。式は1つの値を返すこと。
gen() {
  printf '%s\n' "$3" >>"$tmp/g-$2.expr"
  printf '%s\t%s\n' "$1" "$4" >>"$tmp/g-$2.meta"
}
gen_flush() {
  local f key name n i line want desc prog
  for f in "$tmp"/g-*.expr; do
    [ -e "$f" ] || continue
    key="${f##*/g-}"; key="${key%.expr}"; name="${key%-record}"
    prog="$(sed 's/^/(/; s/$/),/' "$f")"; prog="${prog%,}"
    jq -c "$prog" "${first[$key]}" >"$tmp/g-$key.out"
    n="$(wc -l <"$f")"
    [ "$(wc -l <"$tmp/g-$key.out")" -eq "$n" ] || { ng "$key: 式 $n 本から出た値の数が違う"; continue; }
    i=0
    while IFS= read -r line && IFS=$'\t' read -r -u 3 want desc; do
      i=$((i + 1))
      printf '%s\n' "$line" >"$tmp/m-$key-$i.json"
      expect "$want" "$name" "$tmp/m-$key-$i.json" "$key: $desc"
    done <"$tmp/g-$key.out" 3<"$tmp/g-$key.meta"
    rm -f "$f" "$tmp/g-$key.meta"
  done
  flush
}
for key in "${!first[@]}"; do
  name="${key%-record}"; src="${first[$key]}"
  if [ "$key" != issue-keys ] && [ "$key" != ledger-ready ]; then   # kanban の項目と ready の課題は kanban の形式に倣って知らないキーを許す（ready は並び）
    gen fail "$key" '. + {"x_unknown": 1}' "知らないキー x_unknown を足すと落ちる"
  fi
  for k in ${required[$key]}; do
    gen fail "$key" "del(.\"$k\")" "必須のキー $k を消すと落ちる"
  done
done

# 入れ子の object の必須のキーと知らないキー（例の key ｜ object を指す jq の道 ｜ 必須のキー。これも schema から読まない）
# kanban の行（ledger-log）の最初の例は課題の行で、入れ子の change はどのキーも許す。ほかの形の行の入れ子は 3b で作る
while IFS='|' read -r key at keys; do
  case "$key" in ''|'#'*) continue ;; esac
  name="${key%-record}"; src="${first[$key]}"
  gen fail "$key" "$at += {\"x_unknown\": 1}" "$at に知らないキーを足すと落ちる"
  for k in $keys; do
    gen fail "$key" "del($at.\"$k\")" "$at の必須のキー $k を消すと落ちる"
  done
done <<'NESTED'
findings|.findings[0]|severity file line
report|.review_request|round limit commit previous reviewer extra
report|.waived[0]|severity file line reason
report|.acceptance[0]|id result evidence
report|.risk|one_way
report|.estimate|size reason
report|.plan[0]|id text done
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
measure|.efficiency|merged_prs human_touches answers auto_merge gate
measure|.efficiency.human_touches|total per_pr merge decision task
measure|.efficiency.answers|owner chief_of_staff machine unmarked
measure|.efficiency.auto_merge|count rate
measure|.efficiency.gate|records fix needs_human rate
measure|.counter|reverted follow_ups reopened corrected
measure|.counter.corrected|corrections by_tier
measure|.counter.corrected.by_tier|A B C none
measure|.counter.reopened|corrections by_tier
measure|.counter.reopened.by_tier|A B C none
measure|.prs[0]|issue pr merged_at tier auto human_touches lead_time_hours reverted corrections
measure|.skipped[0]|source reason
measure|.news|threshold kinds candidates
measure|.news.kinds[0]|kind delivered acked rate recorded
measure|.estimate|count within rate none
measure|.issues[0]|issue estimate_minutes agent_estimate_minutes worked_minutes ratio
measure|.tokens[0]|issue source usage
measure|.tokens[0].usage|input output cache_read cache_write
delivered|.news[0]|kind
corrections|.corrections[0]|kind question decision_kind recommended picked
corrections|.corrections[1]|kind comments human_commits
corrections|.corrections[1].comments[0]|author at body
corrections|.corrections[1].human_commits[0]|sha headline files
corrections|.corrections[2]|kind finding reason
corrections|.corrections[2].finding|severity file line
corrections|.corrections[3]|kind question recommended answer
project|.signals[0]|source weight
project|.signals[2]|source weight
project|.digest|destinations
project|.digest.destinations[0]|kind channel
signal|.signals[0]|source project key url title weight fingerprint first_seen last_seen count active episodes delivered_episode
signal|.signals[1]|source project key url title weight fingerprint first_seen last_seen count active episodes delivered_episode
launch|.harness|name version
launch|.package|root commit source drift reasons
launch|.workspace|path commit dirty
launch|.repo|name path commit dirty
launch|.external[0]|kind name path source ref version revision
outcome|.gates[0]|head at tier decision reasons merged
outcome|.sessions[0]|harness session cost
outcome|.sessions[0].cost|models efforts turns tokens
outcome|.sessions[0].cost.tokens|input output cache_read cache_write
retro|.previous_retro|issue done_at until
retro|.issues[0]|id title profile repo done_at library estimate agent_estimate worked_minutes pr launches outcome gates reviews waived corrections remember agents_md journal after_merge reapplied reapplied_reasons
retro|.issues[0].launches[0]|at profile model commit
retro|.issues[0].outcome|result worked_minutes cost
retro|.issues[0].outcome.cost|sessions measured tokens
retro|.issues[0].outcome.cost.tokens|input output cache_read cache_write
retro|.issues[0].gates[0]|head at tier decision reasons merged
retro|.issues[0].reviews[0]|id profile reason findings outcome
retro|.issues[0].after_merge|merge_commit merged_at checks reverted_by
retro|.issues[0].after_merge.checks|state failed
retro|.issues[0].after_merge.reverted_by[0]|pr merged_at
retro|.summaries[0]|id title done_at pr findings gate_fixes corrections remember
retro|.typed[0]|id session at role issue text truncated
retro|.fixes[0]|issue title retro pr merged_at issues_after recurred_before
retro|.skipped[0]|source reason
patterns|.previous_retro|issue done_at
patterns|.patterns[0]|id summary evidence level action target effect risk decision reason
patterns|.patterns[0].target|kind paths
patterns|.fixes[0]|issue recurred view
waiting|.slots|limit working waiting free waiting_full
waiting|.items[0]|kind id title since holds
waiting|.items[1].ask|kind question options
waiting|.items[1].ask.options[0]|label
digest-input|.previous|issue until sent_at
digest-input|.issues[0]|id title status done_at pr merged_at merged_by tier grant retro
digest-input|.concerns[0]|kind issue title at url
digest-input|.numbers|days current previous
digest-input|.numbers.days[0]|date merged auto
digest-input|.numbers.current|since until merged auto closed lead_time_hours question_open_hours tokens
digest-input|.next|ready observations routines
digest-input|.next.observations[0]|id title not_before
digest-input|.next.routines[0]|name title next
digest-input|.decided|decisions triage retro answers mains
digest-input|.decided.mains[0]|repo commit
digest-input|.decided.decisions[0]|text mark repo commit path line url
digest-input|.decided.triage[0]|issue title at closed cut path
digest-input|.decided.retro[0]|issue title at fix dropped path
digest-input|.decided.answers[0]|issue title n question answer at path
digest|.themes[0]|title body issues
digest|.themes[0].figure|type before after
digest|.themes[1].figure|type steps focus
digest|.concerns[0]|text issues
digest-sent|.destinations[0]|kind at ok
digest-sent|.destinations[1]|kind at ok channel
NESTED

# 3b. 値と入れ子（pass|fail ｜ 例の key ｜ jq の式 ｜ 説明。jq の式は | を含んでよい。説明は含まない）
while IFS='|' read -r want key rest; do
  case "$want" in ''|'#'*) continue ;; esac
  desc="${rest##*|}"; expr="${rest%|*}"
  name="${key%-record}"
  gen "$want" "$key" "$expr" "$desc"
done <<'EOF'
pass|issue-keys|. + {"links": ["https://example.com"]}|kanban の形式のキーは通る
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
pass|issue-keys|.acceptance = "A4,A13"|acceptance が受け入れ条件の番号のコンマの並び
fail|issue-keys|.acceptance = "A1, A4"|acceptance の番号の間に空白
fail|issue-keys|.acceptance = ["A1"]|acceptance が並び（kanban set は文字列で書く）
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
pass|report|del(.estimate)|estimate の無い report.json（見込みに反論しない）
fail|report|.estimate.size = "大"|estimate.size が段の名（値は英字の small/medium/large）
fail|report|.estimate.size = 120|estimate.size が分の数
fail|report|.estimate.reason = ""|estimate.reason が空
pass|report|del(.plan)|plan の無い report.json
fail|report|.plan[0].done = "yes"|plan の done が真偽でない
fail|report|.plan[0].text = ""|plan の text が空
fail|report|.plan[0].id = ""|plan の id が空
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
pass|report|.acceptance[0].run = 1|acceptance が走らせた記録を指す
fail|report|.acceptance[1].run = 0|acceptance の run が 0
fail|report|.acceptance[1].run = "3"|acceptance の run が文字列（記録の id は数）
fail|report|.acceptance[1].run = null|acceptance の run が null（記録でない証拠は書かない）
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
pass|notification|.kind = "answered" | .detail = {n: 1, kind: "users", answer: "CSV だけ"}|kind answered（オーナーがペインで答えた）
fail|notification|.kind = "answered" | .detail = {n: 1, kind: "users"}|answered に答えの文が無い
fail|notification|.kind = "answered" | .detail = {n: 1, kind: "users", answer: ""}|answered の答えの文が空
fail|notification|.kind = "answered" | .detail = {n: 1, kind: "users", answer: "x", question: "x"}|answered に知らないキー
fail|notification|.kind = "answered" | .detail = {n: 0, kind: "users", answer: "x"}|answered の n が 0
pass|notification|.kind = "ready" | .detail = {title: "x"}|kind ready（routine 無し）
pass|notification|.kind = "review-requested" | .detail = {round: 1, reviewer: "codex-review", reviewer_issue: "A-13"}|kind review-requested
pass|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json"}|kind gate
pass|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json", no_agent: true}|kind gate（fix で担当がいない）
fail|notification|.kind = "gate" | .detail = {decision: "approve", reasons: [], record: "/x/gate/a.json", no_agent: true}|gate の no_agent が fix でない
fail|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json", no_agent: false}|gate の no_agent が true でない
fail|notification|.kind = "gate" | .detail = {decision: "fix", reasons: ["x"], record: "/x/gate/a.json", no_agent: true, unsent: "t7"}|gate に no_agent と unsent が一緒
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}|kind merged
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "person"}|kind merged（オーナーがマージした）
pass|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "person", held: "x"}|kind merged（オーナーがマージしたが片付けていない）
fail|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", held: "x"}|merged の held に by が無い
fail|notification|.kind = "merged" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", by: "dispatcher"}|merged の by が person でない
pass|notification|.kind = "closed" | .detail = {pr: .detail.pr}|kind closed
fail|notification|.kind = "closed" | .detail = {pr: .detail.pr, sha: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}|closed に知らないキー
pass|notification|.kind = "stalled" | .detail = {agent: "t7", status_ms: 1793845966681} | del(.issue)|kind stalled（issue 無し）
pass|notification|.kind = "resumed" | .detail = {agent: "t9", from: "t7"}|kind resumed
fail|notification|.kind = "resumed" | .detail = {agent: "t9"}|resumed に from が無い
fail|notification|.kind = "resumed" | .detail = {agent: "t9", from: "t7", status_ms: 0}|resumed に知らないキー
pass|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 31, ratio: 1.03, stage: 1}|kind estimate
fail|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 31, ratio: 1.03, stage: 3}|estimate の stage が 1・2・4 でない
fail|notification|.kind = "estimate" | .detail = {estimate: "中", estimate_minutes: 60, worked_minutes: 61, ratio: 1.02, stage: 1}|estimate の値が段の名
fail|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 31, ratio: 1.03}|estimate に stage が無い
fail|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 31, ratio: 1.03, stage: 1, agent: "t7"}|estimate に知らないキー
pass|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 61, ratio: 2.03, stage: 2, sign: ["no-commit", "large-diff"]}|estimate に詰まりの兆し
fail|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 61, ratio: 2.03, stage: 2, sign: []}|estimate の sign が空（兆しが無ければ置かない）
fail|notification|.kind = "estimate" | .detail = {estimate: "small", estimate_minutes: 30, worked_minutes: 61, ratio: 2.03, stage: 2, sign: ["slow"]}|estimate の sign が知らない値
pass|notification|.kind = "memory" | .detail = {count: 31, line: 30} | del(.issue)|kind memory（issue 無し）
fail|notification|.kind = "memory" | .detail = {count: 31} | del(.issue)|memory に line が無い
fail|notification|.kind = "memory" | .detail = {count: 31, line: 30, dir: "/x"} | del(.issue)|memory に知らないキー
pass|notification|.kind = "idle" | .detail = {free: 3, unstarted: ["S-0031", "S-0032"], fired: ["triage"], lint: 0} | del(.issue)|kind idle（issue 無し）
pass|notification|.kind = "idle" | .detail = {free: 1, unstarted: [], fired: [], lint: 2} | del(.issue)|kind idle（lint だけ）
fail|notification|.kind = "idle" | .detail = {free: 0, unstarted: [], fired: [], lint: 2} | del(.issue)|idle の free が 0
fail|notification|.kind = "idle" | .detail = {free: 1, unstarted: [], lint: 2} | del(.issue)|idle に fired が無い
fail|notification|.kind = "idle" | .detail = {free: 1, unstarted: ["a/b"], fired: [], lint: 0} | del(.issue)|idle の unstarted が id でない
fail|notification|.kind = "idle" | .detail = {free: 1, unstarted: [], fired: [""], lint: 0} | del(.issue)|idle の fired が空の名前
fail|notification|.kind = "idle" | .detail = {free: 1, unstarted: [], fired: [], lint: -1} | del(.issue)|idle の lint が負
fail|notification|.kind = "idle" | .detail = {free: 1, unstarted: [], fired: [], lint: 0, ready: []} | del(.issue)|idle に知らないキー
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
pass|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", title: "x", weight: "now", release: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"} | del(.issue)|kind signal に失敗が始まった commit（release）
fail|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", title: "x", weight: "now", release: "9c4e1b0d7a3f"}|signal の release が40桁でない
fail|notification|.kind = "signal" | .detail = {source: "github-ci", key: "x", url: "https://example.com/x", title: "x", weight: "now", release: "9C4E1B0D7A3F5E2C8B6D4A1F0E9C7B5A3D2E1F08"}|signal の release が大文字
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
pass|gate|{screen_paths: ["web/**", "**/*.css"]}|screen_paths は glob の配列（ADR-0018）
fail|gate|{screen_paths: "web/**"}|screen_paths が文字列
fail|gate|{screen_paths: [""]}|screen_paths に空の glob
pass|gate-record|.review = {state: "missing", must_fix_open: 0, waived: 0}|review が missing（issue・commit 無し）
pass|gate-record|.review = {state: "off", must_fix_open: 0, waived: 0}|review が off（issue・commit 無し）
pass|gate-record|.review.state = "stale"|review が stale（issue・commit あり）
fail|gate-record|del(.review.issue)|current なのに issue が無い
fail|gate-record|.review.task = .review.issue | del(.review.issue)|古いキー review.task（gate/2 は issue）
pass|gate-record|.delegation += {grant: "d3"}|delegation.grant（レベルを通した名指しの grant）
fail|gate-record|.delegation += {grant: "3"}|delegation.grant が grant の id（d<n>）でない
fail|gate-record|.delegation += {grant: null}|delegation.grant が null
fail|gate-record|del(.review.commit)|current なのに commit が無い
fail|gate-record|.review.state = "stale" | del(.review.commit)|stale なのに commit が無い
fail|gate-record|.review.state = "missing"|missing なのに issue・commit がある
fail|gate-record|.review = {state: "off", issue: "A-13", must_fix_open: 0, waived: 0}|off なのに issue がある
pass|gate-record|.acceptance.stale_runs = [{id: "A1", run: 3, commit: "0000000000000000000000000000000000000001"}]|acceptance.stale_runs（記録が head と違う commit。ADR-0035）
fail|gate-record|.acceptance.stale_runs = []|acceptance.stale_runs が空（無ければ書かない）
fail|gate-record|.acceptance.stale_runs = [{id: "A1", run: 3}]|acceptance.stale_runs の行に commit が無い
fail|gate-record|.acceptance.stale_runs = [{id: "A1", run: 0, commit: "0000000000000000000000000000000000000001"}]|acceptance.stale_runs の run が 0
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
fail|project|.chief_of_staff.retro_every = 0|retro_every が 0
pass|project|.chief_of_staff.idle_minutes = 0|idle_minutes が 0（空きを見ない）
fail|project|.chief_of_staff.idle_minutes = -1|idle_minutes が負
fail|project|.chief_of_staff.idle_minutes = "15"|idle_minutes が文字列
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
pass|project|del(.digest)|digest が無い（セルの外へ送らない）
pass|project|.digest.destinations = []|digest の送り先が空（セルの外へ送らない）
pass|project|.digest.destinations[0].channel = "G0123456789"|digest: G で始まるチャンネルの ID
fail|project|.digest = null|digest が null
fail|project|.digest.destinations[0].kind = "email"|digest: 送り先の kind が enum の外
fail|project|.digest.destinations[0].channel = "#digest"|digest: channel が ID でなく名前
fail|project|.digest.destinations[0].channel = "U0123456789"|digest: channel が利用者の ID
fail|project|.digest.destinations[0].channel = " C0123456789"|digest: channel の前に空白
fail|project|.digest.destinations[0].channel = "c0123456789"|digest: channel が小文字
fail|project|.digest.destinations += [.digest.destinations[0] + {channel: "C0000000000"}]|digest: 同じ kind の送り先が2つ
fail|project|.delegation.version = "1"|委譲: version が文字列
fail|project|.delegation.preset = "free"|委譲: 知らない preset
pass|project|.delegation = {version: 1, project: "sample-site"}|委譲: preset も grants も無くても通る（型 work）
fail|project|.delegation.grants[0].scope = "merge"|委譲: scope merge に publish の値
pass|project|.delegation.grants[0] |= (.scope = "merge" | .value = "B")|委譲: scope merge の値 B
pass|project|.delegation.grants[0] |= (.scope = "merge" | .value = "C" | .issues = ["S-0012", "S-0013"])|委譲: 名指しの grant（merge C を課題 2 つに）
fail|project|.delegation.grants[0] |= (.scope = "deploy" | .value = "production" | .issues = ["S-0012"])|委譲: 名指しは merge のほかの範囲に書けない
fail|project|.delegation.grants[0] |= (.scope = "merge" | .value = "none" | .issues = ["S-0012"])|委譲: 名指しの merge の値が none
fail|project|.delegation.grants[0] |= (.scope = "merge" | .value = "C" | .issues = [])|委譲: 名指しが空
fail|project|.delegation.grants[0] |= (.scope = "merge" | .value = "C" | .issues = ["s/12"])|委譲: 名指しが課題の id でない
fail|project|.delegation.grants[0] |= (.scope = "merge" | .value = "C" | .issues = null)|委譲: 名指しが null
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
pass|measure|.efficiency.human_touches.per_pr = null|マージされた PR が無くても介在がある（per_pr が null）
fail|measure|.efficiency.human_touches = {total: 3, per_pr: 1.5}|前のバージョンの介在の形（内訳が無い）
fail|measure|.efficiency.human_touches.task = -1|オーナーの課題の数が負
fail|measure|.efficiency.human_touches.merge = "1"|マージの数が文字列
fail|measure|.efficiency.answers.unmarked = 1.5|印の無い答えの数が整数でない
fail|measure|.efficiency.answers += {"chief-of-staff": 1}|答えの欄に印の値（chief-of-staff）をそのまま使う
fail|measure|.value.observed.hit = -1|仮説を確かめた数が負
fail|measure|.value.days_to_observe.days = "3"|見るまでの日数が文字列
fail|measure|.value.no_hypothesis = 1.5|仮説「無し」の数が整数でない
fail|measure|del(.value)|価値が無い
pass|measure|.value.days_to_observe = {count: 0, days: null}|見るまでの日数が null（観察が無い）
pass|measure|.news = {threshold: 0.8, kinds: [], candidates: []}|通知の記録が無い
fail|measure|.news.kinds[0].acked = -1|素通りの数が負
fail|measure|.news.kinds[0].rate = "80%"|素通りの率が文字列
fail|measure|.news.candidates = "report"|候補が配列でない
fail|measure|del(.news)|通知の節が無い
pass|measure|.tokens = []|期間に動いた課題が無い
pass|measure|.tokens[0].usage = null|記録が無い課題（usage が null）
fail|measure|.tokens[0].usage = 0|記録が無いのを 0 と書く
fail|measure|.tokens[0].usage.output = -1|トークン数が負
fail|measure|.tokens[0].usage.input = 1.5|トークン数が整数でない
fail|measure|.tokens[0].source = "none"|source が claude/codex でない
fail|measure|.tokens[0] += {"dollars": 1}|ドルの欄を足す
pass|delivered|.acked = false|ack の無いターン
pass|delivered|.news = [{kind: "ready"}]|課題の無い通知だけ
pass|delivered|.news = [{kind: "estimate", issue: "S-0001"}, {kind: "memory"}]|見込みとのズレと memory の量の通知（notification の kind は全部通る）
pass|delivered|.news = [{kind: "idle"}]|枠の空きの通知
fail|delivered|.news = []|渡した通知が無い
fail|delivered|.news[0].kind = "nope"|知らない kind
fail|delivered|.acked = "true"|acked が文字列
fail|delivered|.at = "2026-11-12"|at が時刻でない
fail|delivered|.news[0].issue = "a/b"|issue に /
pass|launch|.command = null | .harness.version = null | .session = null | .external = null | .memory_index = null|取れない値は null
pass|launch|.role = "chief-of-staff" | .issue = null | .repo = null|Chief of Staff は課題も repo も null
pass|launch|.repo = null|repo の無い課題
pass|launch|.drift = true | .drift_reasons = ["パッケージ: 中身が焼いたときと違う"]|食い違いの印と理由
pass|launch|.drift = null | .drift_reasons = ["セルの repo の commit を読めない"] | .workspace.commit = null | .workspace.dirty = null|確かめられないときは null と理由
pass|launch|.package.commit = null | .package.source = "none" | .package.drift = null | .package.reasons = ["version.json が無い"]|パッケージのバージョンが分からない
pass|launch|.launch = "fallback" | .role = "reviewer"|使用量の上限で起動し直したレビュアー
pass|launch|.external = []|外の skill も plugin も無い
fail|launch|.role = "chief-of-staff"|Chief of Staff なのに課題がある
fail|launch|.issue = null|担当なのに課題が null
fail|launch|.drift = true|drift が true なのに理由が無い
fail|launch|.drift_reasons = ["x"]|drift が false なのに理由がある
fail|launch|.role = "human"|知らない役
fail|launch|.launch = "restart"|知らない起動のしかた
fail|launch|.session = "abc"|session が UUID でない
fail|launch|.package.commit = "abc"|commit が 40 桁でない
fail|launch|.package.source = "git"|package の source が知らない値
fail|launch|.harness.name = "gemini"|知らないハーネス
fail|launch|.external[0].kind = "agent"|external の kind が知らない値
fail|launch|.terminal = ""|terminal が空
fail|launch|.at = "2026-11-19"|at が時刻でない
fail|launch|.workspace.dirty = "no"|dirty が真偽値でない
pass|outcome|.result = "closed" | .merge_commit = null | .merged_at = null|閉じられた PR
pass|outcome|.result = "open" | .merge_commit = null | .merged_at = null|開いたまま片付いた PR
pass|outcome|.result = "none" | .pr = null | .merge_commit = null | .merged_at = null | .gates = []|PR 無し
pass|outcome|.sessions = null|起動の記録が無い（この仕組みの前の課題）
pass|outcome|.sessions = []|セッションが無い
pass|outcome|.sessions[0].cost.turns = null | .sessions[0].cost.efforts = []|ターンと effort の印が無い記録
pass|outcome|.sessions[0] = {harness: "codex", session: null, cost: null}|rollout の見つからない Codex
fail|outcome|.result = "reverted"|reverted は結果の値でない（マージ後のことは振り返りが読む）
fail|outcome|.result = "ci_failed"|ci_failed は結果の値でない（gate の理由で読む）
fail|outcome|.pr = "#12"|pr が URL でない
fail|outcome|.merge_commit = "abc"|merge_commit が 40 桁でない
fail|outcome|.merged_at = "2026-11-19"|merged_at が時刻でない
fail|outcome|.gates[0].decision = "maybe"|知らない判定
fail|outcome|.gates[0].tier = "D"|知らないレベル
fail|outcome|.reviewers = -1|reviewers が負
fail|outcome|.worked_minutes = 1.5|worked_minutes が整数でない
fail|outcome|.sessions[0].harness = "gemini"|知らないハーネス
fail|outcome|.sessions[0].session = ""|session が空
fail|outcome|.sessions[0].cost.tokens.input = -1|トークンが負
fail|outcome|.sessions[0].cost.tokens += {"dollars": 1}|ドルの欄を足す
fail|outcome|.sessions[0].cost.models = [""]|モデルの id が空
fail|outcome|.issue = "a/b"|issue に /
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
pass|signal|del(.read_at)|read_at が無い（まだ読みに行っていない）
fail|signal|.read_at = {"github-ci github.com/example/sample-site": "2026-11-10 09:30"}|read_at の時刻が形の外
fail|signal|.read_at = []|read_at が object でない
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
pass|retro|.previous_retro = null|前の振り返りが無い
pass|retro|.issues[0].launches = null|起動の記録の無い課題（この仕組みの前）
pass|retro|.issues[0].after_merge = null|マージされていない課題
pass|retro|.issues[0].reapplied = null|当て直しを確かめられない
fail|retro|.typed[0].role = "person"|オーナーの文の role が知らない値
fail|retro|.typed[0].id = "u1"|オーナーの文の id が h- で始まらない
fail|retro|.issues[0].after_merge.checks.state = "error"|checks の state が知らない値
fail|retro|.redacted = -1|伏せた数が負
fail|retro|.issues[0].estimate = "中"|見込みが段の名
pass|retro|. + {"previous_retro": (.previous_retro + {"until": null}), "boundary": .previous_retro.done_at}|前の振り返りに retro.json が無い（境目は片付いた時刻）
pass|retro|. + {"previous_retro": null, "boundary": null}|前の振り返りが無い（どの束も新しい）
pass|retro|.cut = {"new": 130, "full": 100}|新しい束が 100 件を超えた
fail|retro|.cut = {"new": 130}|切ったのに全文の数が無い
fail|retro|.cut = {"new": 0, "full": 100}|切った数が 0
pass|retro|.summaries[0].corrections = null|手直しを集めていない要約
pass|retro|.summaries[0].pr = null|PR の無い要約
fail|retro|.summaries[0].remember = -1|Remember の行数が負
fail|retro|.summaries[0].findings = "major"|指摘の行が配列でない
fail|retro|.summaries[0].journal = "x"|要約に日報がある
pass|retro|.issues[0].outcome.cost = null|セッションの記録の無い結果
pass|retro|.issues[0].launches[0] += {"model": null, "commit": null}|起動行にモデルが無い・パッケージの commit が無い
fail|retro|.issues[0].launches[0].memory_index = "x"|起動の記録に目録がある
fail|retro|.issues[0].outcome.gates = []|結果の記録に gate の写しがある
pass|retro|.fixes[0].recurred_before = null|前の振り返りが数えていない前の直し
fail|retro|.fixes[0].recurred_before = -1|前の再発の数が負
pass|retro|.issues[0].reviews[0].reason = null|回の理由を決められないレビュー
fail|retro|.issues[0].reviews[0].reason = "ci"|回の理由が知らない値
pass|patterns|.previous_retro = null|前の振り返りが無い
pass|patterns|. + {"patterns": [], "fixes": []}|パターンも前の直しも無い
pass|patterns|.patterns[0].target = {"kind": "andon", "paths": ["package/skills/worker/SKILL.md"]}|Andon の直しで repo を持たない
fail|patterns|del(.patterns[1].not_upstream)|skill の段なのに上の段で止められない理由が無い
fail|patterns|.patterns[0] += {"level": "text"}|文面の段なのに上の段で止められない理由が無い
fail|patterns|del(.patterns[0].target.repo)|project なのに repo が無い
fail|patterns|.patterns[0].level = "prompt"|知らない段
fail|patterns|.patterns[0].action = "rewrite"|知らない action
fail|patterns|.patterns[0].target.kind = "cell"|知らない直す先の種類
fail|patterns|.patterns[0].decision = "keep"|知らない判定（持ち越しは無い）
fail|patterns|.patterns[0].evidence = []|根拠が空
fail|patterns|.patterns[0].target.paths = []|直す先の paths が空
fail|patterns|.patterns[0].target.paths = ["/opt/andon/x"]|直す先の paths が絶対
fail|patterns|.patterns[0].id = "1"|id が p で始まらない
fail|patterns|.patterns[1].id = "p1"|id が重なる
fail|patterns|.patterns[0].summary = "-x"|題が - で始まる
fail|patterns|.patterns[0].effect = "a\nb"|期待する効果に改行
fail|patterns|.patterns[0].reason = " "|理由が空白だけ
fail|patterns|.fixes[0].recurred = -1|再発の数が負
fail|patterns|.days = 0|days が 0
pass|run|. + {"expect": "fail", "exit_code": 1, "dirty": true}|落ちることを期待する記録（誤りを戻した木）
pass|run|del(.commit, .dirty)|repo の無い課題の記録（commit も dirty も無い）
pass|run|.exit_code = -1|signal で終わった
pass|run|. + {"truncated": true, "output_bytes": 2097152}|出力を切った
pass|run|. + {"id": 12, "output": "runs/12.log"}|2桁の番号
fail|run|del(.dirty)|commit があるのに dirty が無い
fail|run|del(.commit)|dirty があるのに commit が無い
fail|run|.expect = "error"|expect が pass/fail でない
fail|run|.id = 0|id が 0
fail|run|.id = "3"|id が文字列
fail|run|.command = []|command が空
fail|run|.command = "go test ./..."|command が配列でない（shell の1行でない）
fail|run|.command[0] = ""|command に空の引数
fail|run|.exit_code = -2|exit_code が -1 より小さい
fail|run|.exit_code = 1.5|exit_code が整数でない
fail|run|.dirty = "no"|dirty が真偽でない
fail|run|.commit = "9c4e1b0"|commit が40桁でない
fail|run|.started_at = "2026-11-20 09:12:03"|started_at が UTC の RFC 3339 でない
fail|run|.finished_at = "2026-02-30T09:00:00Z"|finished_at が暦に無い日
fail|run|.output = "/tmp/3.log"|output が絶対パス
fail|run|.output = "runs/../report.json"|output が runs/<n>.log でない
fail|run|.output_bytes = -1|output_bytes が負
fail|run|.truncated = "no"|truncated が真偽でない
fail|run|. + {"stdout": "ok"}|出力の本文を記録に持つ（持たない）
pass|ledger-log|.change = {"links": ["https://example.com"], "phase": null}|kanban set の並びと消したキー
pass|ledger-log|.change = {"add": "Add the button"}|kanban add
pass|ledger-log|.change = {"report": "append"}|kanban report --append
pass|ledger-log|.change = {"after": null, "status": "todo"}|after を空にして todo に出た
pass|ledger-log|.change = {"status": "done", "forced": [1, 3]}|done --force が越えた問い
pass|ledger-log|.change = {"status": "todo", "freedBy": "S-0011"}|done で queued から出た
pass|ledger-log|.id = "S-5"|4 桁に揃える前の id
pass|ledger-log|.by = "unknown"|端末の外で利用者の名前も無い
fail|ledger-log|.at = "2026-11-02"|at が日付だけ
fail|ledger-log|.at = "2026-11-02T10:03:00+09:00"|at が UTC でない
fail|ledger-log|.by = ""|by が空
fail|ledger-log|.id = "../S-0012"|id がフォルダの外を指す
fail|ledger-log|.change = "doing"|change が object でない
fail|ledger-log|.change.round = 1|change の値が数
fail|ledger-log|.change = {"report": "replace"}|report が write・append でない
fail|ledger-log|.change = {"question": 1}|question に status が無い
fail|ledger-log|.change = {"question": 1, "status": "closed"}|問いの status が知らない値
fail|ledger-log|.change = {"question": 0, "status": "open"}|question が 0
fail|ledger-log|.change = {"forced": [1]}|forced に status が無い
fail|ledger-log|.change = {"status": "done", "forced": []}|forced が空
fail|ledger-log|.change = {"freedBy": "S-0011"}|freedBy に status が無い
fail|ledger-log|.change = {"after": "S-0011"}|after が並びでない
fail|ledger-log|. + {"config": {"chief_of_staff": "t2"}}|課題の行と config の行が混ざる
pass|ledger-log|{at, by, config: {"chief_of_staff": null}}|config の行（消したキー）
pass|ledger-log|{at, by, config: {"coordinator": "t5"}}|2026-11-06 より前の config の行
fail|ledger-log|{at, by, config: {"chief_of_staff": 2}}|config の値が数
fail|ledger-log|{at, config: {"chief_of_staff": "t2"}}|config の行に by が無い
pass|ledger-log|{at, by: "routine:triage", routine: "triage", skipped: "paused"}|ルーチンが止めてある
fail|ledger-log|{at, by: "routine:triage", routine: "triage"}|ルーチンの行に fired も skipped も無い
fail|ledger-log|{at, by: "routine:triage", routine: "triage", fired: "S-0013", skipped: "open"}|ルーチンの行に fired と skipped の両方
fail|ledger-log|{at, by: "routine:triage", routine: "triage", skipped: "later"}|skipped が open・paused でない
fail|ledger-log|{at, by, routine: "triage", fired: "S-0013"}|ルーチンの行の by が routine: でない
fail|ledger-log|{at, by: "routine:triage", routine: "triage", fired: "S-0013", id}|ルーチンの行に id
pass|ledger-log|{at, by, migrate: {from: ".flightdeck", ids: {}}}|移した行（改めた id が無い）
fail|ledger-log|{at, by, migrate: {from: ".flightdeck"}}|migrate に ids が無い
fail|ledger-log|{at, by, migrate: {ids: {}}}|migrate に from が無い
fail|ledger-log|{at, by, migrate: {from: ".flightdeck", ids: {}, x_unknown: 1}}|migrate に知らないキー
fail|ledger-log|{at, by, migrate: {from: ".flightdeck", ids: {"S-5": "../S-0005"}}}|改めた id がフォルダの外を指す
fail|ledger-log|{at, by, migrate: {from: ".flightdeck", ids: {}}, x_unknown: 1}|移した行に知らないキー
pass|ledger-ready|[]|引き取れる課題が無い
pass|ledger-ready|.[0] += {"mine": 1}|課題の知らないキーは通る
fail|ledger-ready|del(.[0].id)|課題の id が無い
fail|ledger-ready|del(.[0].status)|課題の status が無い
fail|ledger-ready|.[0].status = "doing"|status が todo・queued でない
fail|ledger-ready|.[1].after = "S-0012"|after が並びでない
fail|ledger-ready|.[1].who = "agent"|who が person でない
fail|ledger-ready|.[0].created = "2026-11-02"|created が日付だけ
fail|ledger-ready|.[0] += {"broken": "can't read"}|読めない課題
fail|ledger-ready|.[0]|並びでない
pass|waiting|.items = []|オーナー待ちが無い
pass|waiting|.items[1] |= del(.ask)|問いの JSON の無い判断の行
pass|waiting|.items[1] |= (del(.ask, .question) | .answerable = false | .reason = "オーナーに聞く問いが置かれていない")|問いの番号の無い判断の行（ADR-0057）
fail|waiting|.version = 2|version が 1 でない
fail|waiting|.slots.limit = 0|上限が 0
fail|waiting|.slots.free = -1|空きが負
fail|waiting|.slots.waiting_full = "no"|waiting_full が真偽値でない
fail|waiting|.items[0].kind = "review"|kind が merge・decision・task でない
fail|waiting|.items[0].question = "S-0021"|question が <id>#<n> でない
fail|waiting|.items[0].since = "2026-11-21"|since が日付だけ
fail|waiting|.items[0].holds = "S-0022"|holds が並びでない
fail|waiting|.items[1].ask.kind = "scope"|判断の行の問いの種類が scope
fail|waiting|.items[1].ask.from = "gate"|判断の行の問いを gate が出した
fail|waiting|.items[1].ask.options = [{"label": "一つ"}]|選択肢が 1 つ
fail|waiting|.items[1].answerable = "yes"|answerable が真偽値でない
pass|digest-input|.previous = null|前に送ったダイジェストが無い
pass|digest-input|.issues = [] | .concerns = [] | .skipped = []|期間に何も無い
pass|digest-input|.issues[0].merged_by = null | .issues[0].merged_at = null|期間にマージされていない片付け
pass|digest-input|.next.routines[0].next = null|止めてあるルーチン
pass|digest-input|.skipped = [{source: "github.com/example/sample-site", reason: "読めない"}]|読めなかったもの
fail|digest-input|.version = 2|version が 1 でない
fail|digest-input|.since = "2026-11-20"|since が日付だけ
fail|digest-input|.truncated = "no"|truncated が真偽値でない
fail|digest-input|.issues[0].merged_by = "owner"|merged_by が dispatcher・person でない
fail|digest-input|.issues[0].tier = "D"|tier が A・B・C でない
fail|digest-input|.issues[0].grant = "2"|grant が d<n> でない
fail|digest-input|.issues[0].id = "a/b"|課題の id に /
fail|digest-input|.issues[1].id = .issues[0].id|課題が重なる
fail|digest-input|.issues[0].pr = "http://example.com/1"|pr が GitHub の PR の URL でない
fail|digest-input|.concerns[0].kind = "slow"|気になることの種類を知らない
fail|digest-input|.concerns[0].url = "javascript:alert(1)"|気になることの url が https でない
fail|digest-input|.numbers.days[0].merged = -1|日のマージが負
fail|digest-input|.numbers.current.lead_time_hours = "3"|時間が数でない
fail|digest-input|.next.ready = -1|着手できる数が負
fail|digest-input|.next.observations[0].not_before = "11/25"|見る日が日付でない
fail|digest-input|.redacted = -1|伏せた数が負
fail|digest-input|.waiting = []|waiting が object でない
fail|digest-input|.waiting |= del(.slots)|waiting に slots が無い
pass|digest-input|del(.decided)|前のバージョンの入力（代わりに決めたことが無い）
pass|digest-input|.decided.decisions[0].url = null|GitHub でない repo の決定の行
pass|digest-input|.decided = {decisions: [], triage: [], retro: [], answers: [], mains: []}|代わりに決めたことが無い期間
fail|digest-input|.decided = null|代わりに決めたことが null
fail|digest-input|.decided.mains[0].commit = "main"|読んだ main が commit でない
fail|digest-input|.decided.decisions[0].commit = "9c4e1b0"|決定の行の commit が短い
fail|digest-input|.decided.decisions[0].line = 0|決定の行の行番号が 0
fail|digest-input|.decided.decisions[0].mark = ""|決定の行の印が空
fail|digest-input|.decided.decisions[0].url = "javascript:alert(1)"|決定の行の url が https でない
fail|digest-input|.decided.triage[0].closed = -1|トリアージで閉じた数が負
fail|digest-input|.decided.retro[0].at = "2026-11-21"|振り返りを当てた時刻が日付だけ
fail|digest-input|.decided.answers[0].n = 0|範囲の問いの番号が 0
pass|digest|.concerns = []|担当が添える気になることが無い
pass|digest|.themes = []|テーマが無い
pass|digest|.themes[0] |= del(.figure)|図の無いテーマ
fail|digest|.headline = ["一文だけ。"]|要旨が 1 文
fail|digest|.headline[0] = "一。\n二。"|要旨の文に改行
fail|digest|.headline[0] = "  "|要旨の文が空白だけ
fail|digest|.themes[0].body = []|テーマの文が無い
fail|digest|.themes[0].issues = []|テーマが課題を指さない
fail|digest|.themes[0].issues = ["a/b"]|テーマが指す id に /
fail|digest|.themes[0].figure.type = "pie"|図の型を知らない
fail|digest|.themes[0].figure.before = []|before_after の前が空
fail|digest|.themes[1].figure.steps = ["一つ"]|steps が 1 段
fail|digest|.themes[1].figure.focus = 0|focus が 0
fail|digest|.themes[1].figure += {before: ["x"]}|steps に before を混ぜる
fail|digest|.concerns[0].issues = []|気になることが根拠の課題を指さない
pass|digest-sent|.destinations[0] = {kind: "files", at: "2026-11-21T10:01:00Z", ok: false, error: "files/ を作れない"}|送りそこねた送り先
fail|digest-sent|.destinations = []|送り先が無い
fail|digest-sent|.destinations[1].kind = "email"|送り先の種類を知らない（ほかは slack の行の形のまま。kind の enum だけが落とす）
pass|digest-sent|.destinations = [.destinations[0]]|送り先が files だけ（前のバージョンの形）
pass|digest-sent|.destinations[1] = {kind: "slack", at: "2026-11-21T10:01:00Z", ok: false, channel: "C0123456789", error: "not_in_channel"}|Slack へ送りそこねた
fail|digest-sent|del(.destinations[1].file)|Slack へ送れたのに file が無い
fail|digest-sent|del(.destinations[1].channel)|slack の行に channel が無い
fail|digest-sent|.destinations[1].url = "https://example.com/x"|slack の行に url
fail|digest-sent|.destinations[0].channel = "C0123456789"|files の行に channel
fail|digest-sent|.destinations[0].file = "F0001"|files の行に file
fail|digest-sent|.destinations[1] += {ok: false, error: "x"}|送れなかったのに file がある
fail|digest-sent|.destinations[0] |= del(.url)|送れたのに url が無い
fail|digest-sent|.destinations[0] += {error: "x"}|送れたのに error がある
fail|digest-sent|.destinations[0].ok = false|送れなかったのに error が無い（url がある）
fail|digest-sent|.destinations[0].error = "一行目\n二行目" | .destinations[0].ok = false | .destinations[0] |= del(.url)|error が1行でない
fail|digest-sent|.destinations += [.destinations[0]]|同じ送り先が2行
fail|digest-sent|.until = "2026-11-21"|until が日付だけ

EOF

gen_flush

# 4. 自己検査: 制約を緩めた schema の写しでは、この検査全体が落ちること（期待値を schema から作っていない証し）
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  # 緩めた写しごとの再実行は互いに独立なので、並べて走らせて結果を溜めた順に出す
  par="$(nproc 2>/dev/null || echo 2)"; [ "$par" -le 8 ] || par=8
  n=0; descs=()
  while IFS='|' read -r name expr desc; do
    case "$name" in ''|'#'*) continue ;; esac
    n=$((n + 1)); descs[n]="$desc"
    loose="$tmp/loose-$n"; cp -r schema "$loose"
    jq "$expr" "schema/$(schema_file "$name").schema.json" >"$loose/$(schema_file "$name").schema.json"
    while [ "$(jobs -rp | wc -l)" -ge "$par" ]; do wait -n || true; done
    { if ANDON_SCHEMA_DIR="$loose" bash "$0" >/dev/null 2>&1 </dev/null; then echo passed; else echo failed; fi >"$tmp/loose-$n.result"; } &
  done <<'LOOSE'
findings|.required -= ["commit"]|findings の必須から commit を外す
findings|.additionalProperties = true|findings の知らないキーを許す
gate|."$defs".record.required -= ["decision"]|gate の記録の必須から decision を外す
report|."$defs".sha.pattern = "^[0-9a-f]+$"|sha の桁を緩める
project|."$defs".grant.required -= ["id"]|grant の必須から id を外す
project|.required -= ["repos"]|project の必須から repos を外す
project|."$defs".delegation.additionalProperties = true|delegation の知らないキーを許す
report|.properties.review_request.required -= ["previous"]|review_request の必須から previous を外す
findings|."$defs".finding.properties.file.pattern = "^[^\\x00-\\x1f\\x7f]+$"|file の .. の段の検査を外す
candidates|del(.properties.candidates.uniqueKeys)|候補の id の一意を外す
candidates|."$defs".candidate.required -= ["in_scope"]|候補の必須から in_scope を外す
candidates|.additionalProperties = true|候補の一覧の知らないキーを許す
question|.properties.kind.enum -= ["scope"]|問いの kind から scope を外す
issue-keys|del(.properties.deliverable)|kanban の項目から deliverable を外す
measure|.required -= ["skipped"]|流れの数えの必須から skipped を外す
measure|."$defs".rate.anyOf[0].minimum = -1|率の負を許す
corrections|.required -= ["corrections"]|手直しの必須から corrections を外す
corrections|."$defs".pr.anyOf = [{}]|pr の「コメントか commit のどちらか」を外す
notification|.properties.kind.enum -= ["corrections"]|通知の kind から corrections を外す
notification|.properties.kind.enum -= ["answered"]|通知の kind から answered を外す
notification|.properties.kind.enum -= ["signal"]|通知の kind から signal を外す
notification|."$defs".signal.additionalProperties = true|通知 signal の detail の知らないキーを許す
notification|."$defs".resumed.required -= ["from"]|通知 resumed の必須から from を外す
notification|."$defs".estimate.properties.stage.enum += [3]|通知 estimate の段を緩める
notification|."$defs".memory.required -= ["line"]|通知 memory の必須から line を外す
project|."$defs".signal.properties.weight.enum += ["soon"]|signals の weight を緩める
project|."$defs".signal.required -= ["source"]|signals の必須から source を外す
signal|del(.properties.signals.uniqueKeys)|信号の fingerprint の一意を外す
signal|."$defs".signal.required -= ["delivered_episode"]|信号の必須から delivered_episode を外す
signal|."$defs".signal.additionalProperties = true|信号の知らないキーを許す
launch|.required -= ["memory_index"]|起動の記録の必須から memory_index を外す
launch|del(.else)|起動の記録の「drift が false でなければ理由がある」を外す
launch|del(.allOf)|起動の記録の「Chief of Staff は課題が null・ほかは課題がある」を外す
launch|."$defs".external.additionalProperties = true|external の知らないキーを許す
outcome|.properties.result.enum += ["reverted"]|結果の値に reverted を足す
outcome|.required -= ["sessions"]|結果の記録の必須から sessions を外す
outcome|."$defs".cost.additionalProperties = true|cost の知らないキーを許す
retro|.required -= ["typed"]|振り返りの入力の必須から typed を外す
retro|."$defs".bundle.required -= ["after_merge"]|束の必須から after_merge を外す
retro|."$defs".typed.properties.role.enum += ["person"]|オーナーの文の role を緩める
retro|."$defs".after_merge.properties.checks.anyOf[0].properties.state.enum += ["error"]|checks の state を緩める
retro|."$defs".fix.additionalProperties = true|前の直しの知らないキーを許す
retro|."$defs".summary.additionalProperties = true|要約の束の知らないキーを許す
retro|."$defs".summary.required -= ["findings"]|要約の束の必須から findings を外す
retro|."$defs".launch.additionalProperties = true|起動の記録の写しの知らないキーを許す
retro|."$defs".outcome.additionalProperties = true|結果の記録の写しの知らないキーを許す
retro|.required -= ["boundary"]|振り返りの入力の必須から boundary を外す
retro|."$defs".review.properties.reason.anyOf[0].enum += ["ci"]|レビューの回の理由を緩める
patterns|."$defs".pattern.allOf = []|skill と文面の段の「上の段で止められない理由」を外す
patterns|."$defs".pattern.properties.decision.enum += ["keep"]|判定に keep を足す
patterns|."$defs".pattern.required -= ["decision"]|パターンの必須から decision を外す
patterns|."$defs".pattern.properties.target.allOf = []|project の repo の必須を外す
patterns|del(.properties.patterns.uniqueKeys)|パターンの id の一意を外す
run|.required -= ["exit_code"]|記録の必須から exit_code を外す
run|del(.else)|記録の「commit が無ければ dirty も無い」を外す
run|del(.then)|記録の「commit があれば dirty もある」を外す
run|.properties.expect.enum += ["error"]|記録の expect を緩める
run|.properties.output.pattern = "^.+$"|記録の output の置き場を緩める
report|del(.properties.acceptance.items.properties.run.minimum)|acceptance の run の下限を外す
ledger-log|."$defs".issueLine.additionalProperties = true|課題の行の知らないキーを許す
ledger-log|."$defs".at.pattern = "^.+$"|kanban の行の at の時刻の形を緩める
ledger-log|."$defs".change.allOf = []|change の「問い・forced・freedBy には status」を外す
ledger-log|del(."$defs".routineLine.oneOf)|ルーチンの行の「fired か skipped のどちらか」を外す
ledger-log|."$defs".migrateLine.properties.migrate.required -= ["ids"]|移した行の必須から ids を外す
ledger-ready|."$defs".issue.required -= ["status"]|ready の課題の必須から status を外す
ledger-ready|."$defs".issue.properties.status.enum += ["doing"]|ready の status を緩める
ledger-ready|del(."$defs".issue.not)|ready に読めない課題を許す
waiting|.required -= ["slots"]|オーナー待ちの必須から slots を外す
waiting|."$defs".item.properties.kind.enum += ["review"]|行の kind を緩める
waiting|."$defs".ask.properties.kind.enum += ["scope"]|判断の行の問いの種類に scope を許す
waiting|.properties.slots.properties.free.minimum = -10|空きの負を許す
digest-input|.required -= ["truncated"]|ダイジェストの入力の必須から truncated を外す
digest-input|."$defs".issue.properties.merged_by.anyOf[0].enum += ["owner"]|merged_by を緩める
digest-input|."$defs".decision.properties.commit.pattern = "^.+$"|決定の行の commit の形を緩める
digest-input|.properties.decided.required -= ["decisions"]|代わりに決めたことの必須から decisions を外す
digest|."$defs".issues.minItems = 0|テーマと気になることが課題を指さなくてよい
digest|.properties.headline.minItems = 1|要旨の下限を 1 文に緩める
digest-sent|."$defs".destination.properties.kind.enum += ["email"]|送り先の種類を緩める
digest-sent|del(."$defs".destination.allOf)|送り先の kind と ok ごとの形を外す
project|del(.properties.digest.properties.destinations.uniqueKeys)|digest の送り先の kind ごとに1件を外す
project|del(."$defs".digest_destination.properties.channel.pattern)|digest の channel の形を外す
LOOSE
  wait
  for i in $(seq 1 "$n"); do
    if [ "$(cat "$tmp/loose-$i.result")" = passed ]; then ng "自己検査: ${descs[i]} でも通ってしまう"
    else ok "自己検査: ${descs[i]} と落ちる"; fi
  done
fi

# 5. command の出力（emitters の検査が ANDON_EMIT_DIR に写したもの）。
#    ANDON_EMITTED は tests/lib/run-tests.sh が渡す置き場: out/ に写したもの、<名前>.status に終了コード、<名前>.log に出力。
#    結果の無い検査はここで走らせ、同じ out/ に写させる。kanban の CLI は両方が tests/lib/kanban.sh で焼く。
#    go が無くて skip した検査があるときだけ skip
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  emitted="${ANDON_EMITTED:-}"
  if [ -n "$emitted" ]; then emit_dir="$emitted/out"; else emit_dir="$tmp/emit"; fi
  mkdir -p "$emit_dir"
  skipped=0 skip_line='' reused=0
  for t in "${emitters[@]}"; do
    log="$tmp/emit-$t.log"
    if [ -n "$emitted" ] && [ -f "$emitted/$t.status" ]; then
      st="$(cat "$emitted/$t.status")"; cp "$emitted/$t.log" "$log" 2>/dev/null || : >"$log"; reused=$((reused + 1))
    else
      st=0; ANDON_EMIT_DIR="$emit_dir" bash "tests/$t.sh" >"$log" 2>&1 </dev/null || st=$?
    fi
    if [ "$st" != 0 ]; then ng "tests/$t.sh が落ちた（exit $st）"; sed 's/^/     /' "$log" | tail -5
    elif grep -q '^skip: ' "$log"; then skipped=1; [ -n "$skip_line" ] || skip_line="$(grep -m1 '^skip: ' "$log")"; fi
  done
  [ "$reused" = 0 ] || echo "     emitters の ${#emitters[@]} 本のうち $reused 本は、同じ回の1度目の実行が写したものを読んだ"
  if [ "$skipped" = 1 ]; then
    echo "skip command の出力（$skip_line）"
  else
    while IFS='|' read -r file name desc; do
      case "$file" in ''|'#'*) continue ;; esac
      if [ ! -f "$emit_dir/$file" ]; then ng "command の出力: $desc（$file）が無い"
      elif [ "${file%.jsonl}" != "$file" ]; then   # 1行1件の置き場は行ごとに通す
        n=0
        while IFS= read -r line; do
          n=$((n + 1)); printf '%s\n' "$line" >"$tmp/emit-$file-$n.json"
          expect pass "$name" "$tmp/emit-$file-$n.json" "command の出力: $desc の $n 行目が schema/$(schema_file "$name") を通る"
        done <"$emit_dir/$file"
        [ "$n" -gt 0 ] || ng "command の出力: $desc（$file）が空"
      else expect pass "$name" "$emit_dir/$file" "command の出力: $desc が schema/$(schema_file "$name") を通る"; fi
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
gate-record-stale-runs.json|gate|andon gate の記録（受け入れ条件の行が指す記録が head と違う commit）
gate-record-wait.json|gate|andon gate の出力（wait。記録には書かない）
gate-record-named.json|gate|andon gate の出力（名指しの grant がレベルを通した）
project.json|project|andon delegate grant の project.json
project-named.json|project|andon delegate grant --issues の project.json（名指しの grant）
gate-record-merged.json|gate|andon dispatcher がマージしたあとの gate の記録（merged）
question-gate.json|question|andon dispatcher が needs_human で出した問い（from: gate）
notification-review-requested.json|notification|andon dispatcher の通知 review-requested
notification-gate.json|notification|andon dispatcher の通知 gate
notification-merged.json|notification|andon dispatcher の通知 merged
notification-merged-person.json|notification|andon dispatcher の通知 merged（オーナーがマージした）
notification-merged-held.json|notification|andon dispatcher の通知 merged（オーナーがマージしたが片付けていない）
notification-closed.json|notification|andon dispatcher の通知 closed
notification-impact.json|notification|andon dispatcher の通知 impact（andon issue impact から）
measure.json|measure|andon measure --json の出力
measure-empty.json|measure|andon measure --json の出力（記録が無い kanban）
corrections-answer.json|corrections|andon dispatcher の corrections.json（answer）
signals.json|signal|andon signal collect の信号の記録
signals-delivered.json|signal|andon dispatcher が collect と delivered で書いた信号の記録（read_at・delivered_episode）
notification-signal.json|notification|andon dispatcher の通知 signal（now の信号）
notification-idle.json|notification|andon dispatcher の通知 idle（枠の空き）
corrections-pr-inline.json|corrections|andon dispatcher の corrections.json（pr。インラインのレビューコメント）
corrections-override.json|corrections|andon dispatcher の corrections.json（override）
notification-corrections.json|notification|andon dispatcher の通知 corrections（集められなかった）
notification-impact-error.json|notification|andon dispatcher の通知 impact（集められず諦めたとき）
notification-asked.json|notification|andon dispatcher の通知 asked
notification-report.json|notification|andon dispatcher が log.jsonl から拾った通知 report
notification-review.json|notification|andon dispatcher が log.jsonl から拾った通知 review
notification-ready.json|notification|andon dispatcher が ready の差から拾った通知 ready（routine つき）
launch-start.json|launch|andon issue start の起動の記録
launch-drift.json|launch|andon issue start の起動の記録（パッケージの置き場が食い違う・Codex）
launch-norepo.json|launch|andon issue start の起動の記録（repo の無い課題）
launch-resume.json|launch|andon issue resume の起動の記録
launch-chief-engineer.json|launch|andon chief-engineer の起動の記録
launch-reviewer.json|launch|andon dispatcher がレビュアーを起動したときの起動の記録
launch-chief-of-staff.json|launch|andon chief-of-staff の起動の記録
retro.json|retro|andon retro collect の出力
retro-nogh.json|retro|andon retro collect の出力（GitHub が読めない）
retro-split.json|retro|andon retro collect の出力（全文と要約に分けた）
retro-cut.json|retro|andon retro collect の出力（新しい束が 100 件を超えた）
patterns.json|patterns|andon retro patterns の retro.json
run.json|run|andon issue record の runs/<n>.json
run-fail.json|run|andon issue record の runs/<n>.json（落ちることを期待して落ちた）
run-norepo.json|run|andon issue record の runs/<n>.json（repo の無い課題。commit 無し）
ledger-log.jsonl|ledger-log|kanban の CLI が書いた log.jsonl（andon dispatcher がレビューからマージまで回した kanban）
ledger-ready.json|ledger-ready|kanban ready --json の出力（オーナーの課題・レビュアーの子課題・ルーチンの課題）
waiting.json|waiting|andon waiting --json の出力（問いの番号の無い判断の行を含む）
waiting-empty.json|waiting|andon waiting --json の出力（オーナー待ちが無い kanban）
digest-input.json|digest-input|andon digest collect の input.json
digest-input-previous.json|digest-input|andon digest collect の input.json（前に送ったダイジェストのあと）
digest.json|digest|andon digest build の digest.json
digest-sent.json|digest-sent|andon digest send の sent.json
EMIT
    flush
  fi
fi

# 6. contract.md §8（hc との約束）の表に Herdr App の置き場のラベルとオーナーに端末を指す言い方がある（ADR-0002 受け入れ条件 4）。
# 端末のラベルは schema に入れず §8 の表だけで持つので、行があることをここで見る
if [ -z "${ANDON_SCHEMA_DIR:-}" ]; then
  sec8="$(awk '/^## 8\. /{on=1; next} /^## /{on=0} on' docs/contract.md)"
  row="$(grep '^| Herdr App の置き場のラベル |' <<<"$sec8" || true)"
  if grep -q '`herdr.space=<id>`・`herdr.tab=<id>`' <<<"$row" && grep -q 'ラベルを持つのは Herdr App' <<<"$row" && grep -q 'dispatcher' <<<"$row" && grep -q '| Herdr App |$' <<<"$row"; then
    ok "contract.md §8: herdr.space・herdr.tab の行（ラベルを持つのは Herdr App・書き手 command と dispatcher・読み手 App）"
  else ng "contract.md §8 に herdr.space・herdr.tab の行が無い・形が違う"; fi
  if grep -q '^| オーナーに端末を指す言い方 | `<Space 名> / <tab 名>`' <<<"$sec8"; then ok "contract.md §8: オーナーに端末を指す言い方の行"
  else ng "contract.md §8 にオーナーに端末を指す言い方の行が無い"; fi
fi

exit "$fail"
