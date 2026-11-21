# tests/package/25-pgrep-full.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp・andon は入口が置く）。
#  25. pgrep -f・pkill -f を断る PreToolUse のフック: settings が Bash の PreToolUse で package/hooks/pre-tool-use を指し、
#      終わりを待つループの形（while pgrep -f …・until ! pgrep -f …）と pkill -f を deny して理由の1行を返し、文字列として含むだけの呼び出しは通す。
# === 25. pgrep -f・pkill -f を断るフック ===
h=package/hooks/pre-tool-use
jq -e '.hooks.PreToolUse | any(.[]; .matcher == "Bash" and any(.hooks[]; .type == "command" and .command == "/opt/andon/hooks/pre-tool-use"))' package/settings.json >/dev/null \
  && [ -x "$h" ] && sh -n "$h" \
  && pass "PreToolUse（matcher Bash）のフックは package/hooks/pre-tool-use を指す" || fail "settings に Bash の PreToolUse のフック（package/hooks/pre-tool-use）が無い"

# フックは andon が PATH に無ければ通す（最初の実行行）。検査では入口が焼いた andon を PATH に置く
mkdir -p "$tmp/pt-bin"; ln -sf "$andon" "$tmp/pt-bin/andon"
pt() { jq -n --arg t "$1" --arg c "$2" '{hook_event_name: "PreToolUse", tool_name: $t, tool_input: {command: $c}}' | PATH="$tmp/pt-bin:$PATH" sh "$h" 2>/dev/null; }
pt_deny() { pt Bash "$1" | jq -e '.hookSpecificOutput | .hookEventName == "PreToolUse" and .permissionDecision == "deny" and (.permissionDecisionReason | test("ポート") and test("PID ファイル") and (contains("\n") | not))' >/dev/null 2>&1; }

# 断る: 振り返りが拾った2つの形（ループ自身の行に当たって待ちが終わらない）と、同じ種類の形
miss=
while IFS= read -r c; do
  [ -n "$c" ] || continue
  pt_deny "$c" || miss="$miss [$c]"
done <<'EOF'
while pgrep -f "go test" >/dev/null; do sleep 5; done
until ! pgrep -f 'tests/run'; do sleep 10; done
pkill -f "andon dispatcher"
pgrep -af tests/isolation
pgrep --full 'hc run' && echo still
sleep 30; pgrep -fl gopls
bash -c 'while pgrep -f x; do sleep 1; done'
n=$(pgrep -f -c 'go test')
while pgrep '-f' "go test"; do sleep 5; done
until ! "pgrep" -f x; do sleep 1; done
\pkill "--full" gopls
while /usr/bin/pgrep -af x; do sleep 1; done
EOF
[ -z "$miss" ] && pass "pgrep -f・pkill -f の呼び出し（終わりを待つループの形を含む）をフックが deny し、ポートか PID ファイルから引く理由の1行を返す" \
  || fail "pgrep -f・pkill -f の呼び出しを deny しない:$miss"

# 通す: 文字列として含むだけ・-f の無い pgrep・Bash でないツール・入力が JSON でない（落ちたときは通す。contract.md §9）
extra=
while IFS= read -r c; do
  [ -n "$c" ] || continue
  out=$(pt Bash "$c"); [ -z "$out" ] || extra="$extra [$c]"
done <<'EOF'
grep -n 'pgrep -f' package/environment.md
git log --grep 'pkill -f'
pgrep -x hc
pkill -TERM -x gopls
kill "$(cat /tmp/x.pid)"
EOF
out=$(pt Read "pgrep -f x"); [ -z "$out" ] || extra="$extra [Read]"
out=$(printf 'not json' | PATH="$tmp/pt-bin:$PATH" sh "$h" 2>/dev/null); rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [壊れた入力]"
out=$(jq -n '{tool_name: "Bash", tool_input: {command: "pkill -f x"}}' | PATH=/usr/bin:/bin sh "$h" 2>/dev/null); rc=$?; { [ -z "$out" ] && [ "$rc" = 0 ]; } || extra="$extra [andon が無い]"
[ -z "$extra" ] && pass "pgrep -f を文字列として含むだけの呼び出し・-f の無い pgrep・Bash でないツール・壊れた入力・andon の無い環境はフックが通す" \
  || fail "フックが通すはずの呼び出しを断る:$extra"
