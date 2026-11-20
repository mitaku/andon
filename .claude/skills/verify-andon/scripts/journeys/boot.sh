#!/usr/bin/env bash
# boot（features/boot.md）。当てたあと apply-main.sh が打ち直した andon boot の生成物を読む。読むだけ。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon
managed=${ANDON_MANAGED_SETTINGS:-/etc/claude-code/managed-settings.json}

# bt-ran: apply-main.sh は dispatcher の端末を片付けてから andon boot を打ち直し、boot が dispatcher を起動する。
# dispatcher の端末が当てたあとに起動していれば、boot は当てたあとに走った（boot.json は中身が変わらなければ書き直さないので、時刻は見ない）
d=$(hc get dispatcher --json 2>/dev/null) || d=
ms=$(jq -r '.created_ms // 0' <<<"$d" 2>/dev/null || echo 0)
echo "-- bt-ran: dispatcher の端末の起動 $( [ "$ms" -gt 0 ] && date -u -d "@$((ms / 1000))" +%Y-%m-%dT%H:%M:%SZ || echo 無し)"
[ "$ms" -ge "$a_ms" ] && ok "bt-ran: boot が当てたあとに走り、dispatcher を起動した" \
  || fail "bt-ran: dispatcher が当てる前から動いている（apply-main.sh の boot の打ち直しが通っていない）"

# bt-generated: boot が書いたファイルが、控えの sha256 のまま在る（生成物が揃い、あとから書き換わっていない）
n=0 diff=0
while IFS=$'\t' read -r path sum; do
  n=$((n + 1))
  got=$(sha256sum "$path" 2>/dev/null | cut -d' ' -f1)
  [ "$got" = "$sum" ] || { echo "  | 控えと違う: $path"; diff=$((diff + 1)); }
done < <(jq -r 'to_entries[] | select(.value | type == "string" and test("^[0-9a-f]{64}$")) | [.key, .value] | @tsv' "$state/boot.json")
echo "-- bt-generated: 控えにあるファイル $n 個、控えと違うもの $diff 個"
[ "$n" -gt 0 ] && [ "$diff" = 0 ] && ok "bt-generated: boot が書いたファイル $n 個が控えのまま在る" \
  || fail "bt-generated: 控えが空か、控えと違うファイルがある"

# bt-managed: managed settings が当てたパッケージの hook を指す。~/.agents/skills の andon の skill が /opt/andon を指す
jq -e '[.hooks[][].hooks[]?.command] | length > 0 and all(startswith("/opt/andon/hooks/"))' "$managed" >/dev/null \
  && ok "bt-managed: managed settings の hook は全部 /opt/andon/hooks を指す" || fail "bt-managed: managed settings の hook が /opt/andon を指さない"
links=$(find "$HOME/.agents/skills" -maxdepth 1 -name 'andon-*' -type l 2>/dev/null)
echo "-- bt-skills: ~/.agents/skills の andon-* $(grep -c . <<<"$links" || true) 本"
while IFS= read -r l; do
  [ -n "$l" ] || continue
  t=$(readlink -f "$l"); echo "  | $(basename "$l") → $t"
  case "$t" in /opt/andon/skills/*) [ -f "$t/SKILL.md" ] || fail "bt-skills: $l の先に SKILL.md が無い" ;; *) ;; esac
done <<<"$links"
[ "$fails" = 0 ]
