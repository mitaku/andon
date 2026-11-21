#!/usr/bin/env bash
# hook（features/hooks.md）。セルの本物の managed settings が /opt/andon/hooks の4本を指すかを読み、
# 切り離した kanban で、担当の端末の Stop フック（/opt/andon/hooks/stop）を Claude Code と同じく標準入力の JSON で打つ。
set -uo pipefail
managed=${ANDON_MANAGED_SETTINGS:-/etc/claude-code/managed-settings.json}
. "$(dirname "$0")/../sandbox.sh"

# hk-wired: managed settings の hook が当てたパッケージの4本を指し、4本とも実行できる
want='Stop /opt/andon/hooks/stop
Stop /opt/andon/hooks/stop-wait
StopFailure /opt/andon/hooks/stop-failure
UserPromptSubmit /opt/andon/hooks/user-prompt-submit'
got=$(jq -r '.hooks | to_entries[] | .key as $k | .value[].hooks[]? | "\($k) \(.command)"' "$managed" 2>/dev/null | sort)
echo "-- hk-wired: $managed の hooks"
sed 's/^/  | /' <<<"$got"
[ "$got" = "$(sort <<<"$want")" ] && ok "hk-wired: managed settings の hook は /opt/andon/hooks の4本だけを指す" \
  || fail "hk-wired: managed settings の hook が4本と違う"
for h in stop stop-wait stop-failure user-prompt-submit; do
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

# hk-other-pass: 担当でも Chief of Staff でもない端末では、4本とも何も出さずに 0 で通す
export HC_TERMINAL_ID=tx
for h in stop stop-wait stop-failure user-prompt-submit; do
  if step "hk-other-pass: ラベルの無い端末の $h" timeout 30 "/opt/andon/hooks/$h" <<<'{}' && [ ! -s "$tmp/step.out" ]; then :; else
    fail "hk-other-pass: ラベルの無い端末で $h が何かを出すか 0 で終わらない"
  fi
done
[ "$fails" = 0 ] && ok "hk-other-pass: ラベルの無い端末では4本とも何も出さずに通す"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
