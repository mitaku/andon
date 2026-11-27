#!/usr/bin/env bash
# 裏の tests/run が走っている間の書き込みを断る、この repo の Claude Code の PreToolUse のフック（.claude/hooks/tests-run-edit）。
# .claude/settings.json が Edit・Write・MultiEdit・NotebookEdit の PreToolUse でフックを指し、鍵（ANDON_TEST_LOCK）が flock で持たれ、
# 控えの pid が生き、書く先の worktree が控えの PWD と同じときだけ、裏の tests/run が終わるか止めてから書く理由の1行で deny すること。
# 別の worktree・入れ子の worktree・終わった回の控え・鍵が無いときは通すこと。
# 初めの2つは、裏で tests/run を回している間に同じ worktree の台本を Edit で書き換えた出来事と、誤りの戻しを当てかけた出来事を写したもの。
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0
ok() { echo "ok   $1"; }
ng() { echo "FAIL $1"; fail=1; }
h=.claude/hooks/tests-run-edit
root=$PWD
tmp=$(mktemp -d)
holders=()
cleanup() {
  local p
  for p in "${holders[@]+"${holders[@]}"}"; do kill "$p" 2>/dev/null || true; done
  rm -rf "$tmp"
}
trap cleanup EXIT

# フックの在りかと繋ぎ方。settings の command（Claude Code が sh で打つ）をそのまま使って当てる
hook_cmd=$(jq -r '[.hooks.PreToolUse[]? | select(.matcher as $m | ["Edit", "Write", "MultiEdit", "NotebookEdit"] | all(test("^(" + $m + ")$")))
  | .hooks[]? | select(.type == "command") | .command | select(contains(".claude/hooks/tests-run-edit"))][0] // empty' .claude/settings.json 2>/dev/null || true)
if [ -n "$hook_cmd" ] && [ -x "$h" ] && python3 -c 'import ast, sys; ast.parse(open(sys.argv[1]).read())' "$h" 2>/dev/null; then
  ok ".claude/settings.json の PreToolUse（Edit・Write・MultiEdit・NotebookEdit）が .claude/hooks/tests-run-edit を呼び、フックは実行できる"
else
  ng ".claude/settings.json に Edit・Write・MultiEdit・NotebookEdit の PreToolUse のフック（.claude/hooks/tests-run-edit）が無いか、フックが実行できない"
fi

# worktree の置き場。wt と wt-2 は名前の頭が同じ別の worktree、wt/nested は wt の中の入れ子の worktree（.git のファイル）
mkdir -p "$tmp/wt/tests" "$tmp/wt-2" "$tmp/wt/nested" "$tmp/sp ace/wt" "$tmp/plain"
git init -q "$tmp/wt"
git init -q "$tmp/wt-2"
echo 'gitdir: /nowhere' >"$tmp/wt/nested/.git"
git init -q "$tmp/sp ace/wt"
ln -s "$tmp/wt" "$tmp/link"
echo 'echo a' >"$tmp/wt/tests/a.sh"

# hold <lock> <控えの pid（self ならこの回）> <PWD>: tests/run と同じに鍵を flock で取り、控えを書いて待つ。控えを書くまで待って返る
hold() {
  local lock=$1 who=$2 dir=$3
  (
    exec 9>>"$lock"
    flock 9
    if [ "$who" = self ]; then echo "pid $BASHPID $dir" >"$lock"; else echo "pid $who $dir" >"$lock"; fi
    exec sleep 300
  ) &
  holders+=("$!")
  local i
  for i in $(seq 100); do grep -q '^pid ' "$lock" 2>/dev/null && return 0; sleep 0.1; done
  ng "鍵を取る回が控えを書かない（$lock）"
}
release() {
  local p=${holders[-1]}
  kill "$p" 2>/dev/null || true
  wait "$p" 2>/dev/null || true
  unset 'holders[-1]'
}

# pt <lock> <tool> <file_path> [<cwd>]: settings の command でフックに入力を渡し、出力を返す（フックが無ければ空）
pt() {
  [ -n "$hook_cmd" ] || return 0
  local key=file_path
  [ "$2" = NotebookEdit ] && key=notebook_path
  jq -n --arg t "$2" --arg k "$key" --arg f "$3" --arg d "${4:-$root}" \
    '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $d, tool_input: {($k): $f}}' |
    ANDON_TEST_LOCK=$1 CLAUDE_PROJECT_DIR=$root /bin/sh -c "$hook_cmd" 2>/dev/null || true
}
is_deny() {
  jq -e '.hookSpecificOutput | .hookEventName == "PreToolUse" and .permissionDecision == "deny"
    and (.permissionDecisionReason | test("tests/run") and test("終わるか止めてから") and (contains("\n") | not))' >/dev/null 2>&1 <<<"$1"
}
# deny <説明> <lock> <tool> <file_path> [<cwd>] / pass <同じ>: 断るか、何も出さずに通すか
deny() {
  local what=$1 out
  shift
  out=$(pt "$@")
  if is_deny "$out"; then ok "断る: $what"; else ng "断らない: $what（出力: ${out:-空}）"; fi
}
pass() {
  local what=$1 out
  shift
  out=$(pt "$@")
  if [ -z "$out" ]; then ok "通す: $what"; else ng "通さない: $what（出力: $out）"; fi
}

lock=$tmp/lock
hold "$lock" self "$tmp/wt"
# 出来事を写した形
deny "裏の tests/run が走る worktree の台本（tests/a.sh）を Edit で書き換える" "$lock" Edit "$tmp/wt/tests/a.sh"
deny "裏の tests/run が走る worktree に、誤りの戻しを Write で当てる" "$lock" Write "$tmp/wt/internal/gate/gate.go"
# 同じ種類の形
deny "MultiEdit で書き換える" "$lock" MultiEdit "$tmp/wt/tests/a.sh"
deny "NotebookEdit で書き換える" "$lock" NotebookEdit "$tmp/wt/n.ipynb"
deny "worktree の根の直下のファイル" "$lock" Write "$tmp/wt/AGENTS.md"
deny "symlink を通したパス" "$lock" Edit "$tmp/link/tests/a.sh"
deny "cwd からの相対のパス" "$lock" Edit "tests/a.sh" "$tmp/wt"
deny "まだ無いフォルダの下の新しいファイル" "$lock" Write "$tmp/wt/new/dir/x.md"
# 通す形
pass "名前の頭が同じ別の worktree" "$lock" Edit "$tmp/wt-2/x.sh"
pass "PWD の下の入れ子の worktree" "$lock" Edit "$tmp/wt/nested/x.sh"
pass "worktree の外（一時の置き場）" "$lock" Write "$tmp/plain/x.txt"
out=$(jq -n --arg f "$tmp/wt/tests/a.sh" '{hook_event_name: "PreToolUse", tool_name: "Read", tool_input: {file_path: $f}}' |
  ANDON_TEST_LOCK=$lock CLAUDE_PROJECT_DIR=$root /bin/sh -c "${hook_cmd:-false}" 2>/dev/null || true)
if [ -n "$hook_cmd" ] && [ -z "$out" ]; then ok "通す: 書かない道具（Read）"; else ng "通さない: 書かない道具（Read）（出力: ${out:-フックが無い}）"; fi
release

# 鍵が外れたあと（ファイルは残り、中身は終わった回の控え）
pass "tests/run が終わって鍵が外れた（控えは残る）" "$lock" Edit "$tmp/wt/tests/a.sh"
# 鍵は持たれているが、控えの pid が終わっている（鍵を取った回が控えを書き直す前）
true &
dead=$!
wait "$dead"
hold "$lock" "$dead" "$tmp/wt"
pass "控えの pid が終わっている" "$lock" Edit "$tmp/wt/tests/a.sh"
release
# 鍵のファイルが無い。フックは鍵のファイルを作らない
pass "鍵のファイルが無い" "$tmp/none.lock" Edit "$tmp/wt/tests/a.sh"
if [ ! -e "$tmp/none.lock" ]; then ok "フックは鍵のファイルを作らない"; else ng "フックが鍵のファイルを作った"; fi
# 空白を含む PWD
hold "$tmp/lock2" self "$tmp/sp ace/wt"
deny "空白を含む PWD の worktree" "$tmp/lock2" Edit "$tmp/sp ace/wt/x.sh"
pass "空白を含む PWD とは別の worktree" "$tmp/lock2" Edit "$tmp/wt/tests/a.sh"
release

# 本物の tests/run が書く控えの形で断る。tests/run を写した worktree で、走らせる検査の代わりに待つだけの台本を置く
real=$tmp/real
mkdir -p "$real/tests/lib"
git init -q "$real"
cp tests/run "$real/tests/run"
printf '%s\n' '#!/usr/bin/env bash' 'exec sleep 300' >"$real/tests/lib/run-tests.sh"
ANDON_TEST_LOCK=$tmp/lock3 bash "$real/tests/run" >/dev/null 2>&1 &
holders+=("$!")
for i in $(seq 100); do grep -q '^pid ' "$tmp/lock3" 2>/dev/null && break; sleep 0.1; done
deny "本物の tests/run が鍵を持つ worktree" "$tmp/lock3" Edit "$real/tests/run"
pass "本物の tests/run が鍵を持つ worktree とは別の worktree" "$tmp/lock3" Edit "$tmp/wt/tests/a.sh"
release
pass "本物の tests/run が止まったあと" "$tmp/lock3" Edit "$real/tests/run"

exit "$fail"
