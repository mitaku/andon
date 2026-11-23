#!/usr/bin/env bash
# hook（features/hooks.md）。セルの本物の managed settings が /opt/andon/hooks の5本を指すかを読み、
# 切り離した kanban で、担当の端末の Stop フック（/opt/andon/hooks/stop）と PreToolUse フック（/opt/andon/hooks/pre-tool-use）を
# Claude Code と同じく標準入力の JSON で打つ。
set -uo pipefail
managed=${ANDON_MANAGED_SETTINGS:-/etc/claude-code/managed-settings.json}
. "$(dirname "$0")/../sandbox.sh"

# hk-wired: managed settings の hook が当てたパッケージの5本を指し、5本とも実行できる
hooks='pre-tool-use stop stop-wait stop-failure user-prompt-submit'
want='PreToolUse /opt/andon/hooks/pre-tool-use
Stop /opt/andon/hooks/stop
Stop /opt/andon/hooks/stop-wait
StopFailure /opt/andon/hooks/stop-failure
UserPromptSubmit /opt/andon/hooks/user-prompt-submit'
got=$(jq -r '.hooks | to_entries[] | .key as $k | .value[].hooks[]? | "\($k) \(.command)"' "$managed" 2>/dev/null | sort)
echo "-- hk-wired: $managed の hooks"
sed 's/^/  | /' <<<"$got"
[ "$got" = "$(sort <<<"$want")" ] && ok "hk-wired: managed settings の hook は /opt/andon/hooks の5本だけを指す" \
  || fail "hk-wired: managed settings の hook が5本と違う"
# PreToolUse は Bash の呼び出しだけに効く（matcher）
pre=$(jq -r '.hooks.PreToolUse[]? | select(any(.hooks[]?; .command == "/opt/andon/hooks/pre-tool-use")) | .matcher' "$managed" 2>/dev/null)
echo "  | PreToolUse の matcher: ${pre:-無し}"
[ "$pre" = Bash ] && ok "hk-wired: PreToolUse の hook は matcher Bash で配線されている" \
  || fail "hk-wired: PreToolUse の hook の matcher が Bash でない"
for h in $hooks; do
  [ -x "/opt/andon/hooks/$h" ] || fail "hk-wired: /opt/andon/hooks/$h が実行できない"
done

# 担当の課題（doing）。端末 tw のラベル issue= がこの課題を指す
ledger_init V
id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の hook" --profile claude)
HC_TERMINAL_ID=tc "$kanban" set "$id" status=doing >/dev/null
echo "{\"labels\":{\"issue\":\"$id\"}}" >"$tmp/hc/tw.json"
export HC_TERMINAL_ID=tw

# hk-stop-block: doing で報告も問いも無いまま止まろうとすると、Stop フックが block を返す
step "hk-stop-block: 報告の無い担当の Stop フック" /opt/andon/hooks/stop <<<'{"stop_hook_active":false}'
jq -e '.decision == "block" and (.reason | length > 0)' "$tmp/step.out" >/dev/null \
  && ok "hk-stop-block: 報告も問いも無い doing の担当は block（理由つき）" || fail "hk-stop-block: block が出ない"

# hk-stop-pass: 報告を書けば、同じ Stop フックは何も出さずに通す
HC_TERMINAL_ID=tw "$kanban" report "$id" <<<"## Report
$run の検証" >/dev/null
step "hk-stop-pass: 報告を書いた担当の Stop フック" /opt/andon/hooks/stop <<<'{"stop_hook_active":false}'
[ ! -s "$tmp/step.out" ] && ok "hk-stop-pass: 報告のあとは何も出さずに通す" || fail "hk-stop-pass: 報告のあとも何かを出す"

# hk-pre-deny: コマンドの位置にある pgrep -f の Bash の呼び出しを、PreToolUse フックが deny と理由で返す
pretool() { jq -n --arg c "$1" '{tool_name: "Bash", tool_input: {command: $c}}'; }
step "hk-pre-deny: pgrep -f で待つ Bash の呼び出し" /opt/andon/hooks/pre-tool-use <<<"$(pretool 'while pgrep -f sample-site; do sleep 1; done')"
jq -e '.hookSpecificOutput | .hookEventName == "PreToolUse" and .permissionDecision == "deny" and (.permissionDecisionReason | length > 0)' "$tmp/step.out" >/dev/null \
  && ok "hk-pre-deny: コマンドの位置の pgrep -f は deny（理由つき）" || fail "hk-pre-deny: deny が出ない"

# hk-pre-pass: ふつうの呼び出しと、'pgrep -f' を引数の文字列に含むだけの呼び出しは、何も出さずに 0 で通す
for c in 'git status' "grep -n 'pgrep -f' AGENTS.md"; do
  if step "hk-pre-pass: Bash の呼び出し $c" /opt/andon/hooks/pre-tool-use <<<"$(pretool "$c")" && [ ! -s "$tmp/step.out" ]; then
    ok "hk-pre-pass: $c は何も出さずに通す"
  else
    fail "hk-pre-pass: $c で何かを出すか 0 で終わらない"
  fi
done

# hk-other-pass: 担当でも Chief of Staff でもない端末では、5本とも何も出さずに 0 で通す
export HC_TERMINAL_ID=tx
other=0
for h in $hooks; do
  if step "hk-other-pass: ラベルの無い端末の $h" timeout 30 "/opt/andon/hooks/$h" <<<'{}' && [ ! -s "$tmp/step.out" ]; then :; else
    fail "hk-other-pass: ラベルの無い端末で $h が何かを出すか 0 で終わらない"; other=1
  fi
done
[ "$other" = 0 ] && ok "hk-other-pass: ラベルの無い端末では5本とも何も出さずに通す"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
