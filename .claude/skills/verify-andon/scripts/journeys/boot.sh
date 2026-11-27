#!/usr/bin/env bash
# boot（features/boot.md）。当てたあと apply-main.sh が打ち直した andon boot の生成物を読む。読むだけ。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon
managed=${ANDON_MANAGED_SETTINGS:-/etc/claude-code/managed-settings.json}

# bt-ran: apply-main.sh は dispatcher の端末を片付けてから andon boot を打ち直し、boot が dispatcher を起動する。
# dispatcher の端末が最後に当てたあとに起動していれば（今の状態なので、検証する commit でなく最後に当てた時刻と比べる）、boot は当てたあとに走った（boot.json は中身が変わらなければ書き直さないので、時刻は見ない）
d=$(hc get dispatcher --json 2>/dev/null) || d=
ms=$(jq -r '.created_ms // 0' <<<"$d" 2>/dev/null || echo 0)
echo "-- bt-ran: dispatcher の端末の起動 $( [ "$ms" -gt 0 ] && date -u -d "@$((ms / 1000))" +%Y-%m-%dT%H:%M:%SZ || echo 無し)"
[ "$ms" -ge "$now_ms" ] && ok "bt-ran: boot が最後に当てたあとに走り、dispatcher を起動した" \
  || fail "bt-ran: dispatcher が当てる前から動いている（apply-main.sh の boot の打ち直しが通っていない）"

# bt-tray: ローカルの tray（andon tray serve）の控えの pid が、当てたパッケージの serve で、最後に当てた時刻より後に起動し、一覧の面が答える。
# パッケージに tray が無ければ（Node かビルドした tray の無いイメージ）boot は起動しないので、note にして確かめない
if [ ! -f "$pkg/tray/scripts/serve.mjs" ]; then
  echo "note bt-tray: $pkg/tray が無い（tray を入れていないパッケージ）。ローカルの tray は確かめていない"
else
  ctl=$state/tray/serve.json
  tpid=$(jq -r '.pid // empty' "$ctl" 2>/dev/null) || tpid=
  tlist=$(jq -r '.list // empty' "$ctl" 2>/dev/null) || tlist=
  # 一覧の面は名指したホスト名のほかの Host を断る（421）。控えの list_origin のホスト名を Host に付けて打つ
  thost=$(jq -r '.list_origin // empty' "$ctl" 2>/dev/null) || thost=
  thost=${thost#*://}; thost=${thost%%[:/]*}
  targv=
  [ -n "$tpid" ] && mapfile -d '' -t targv 2>/dev/null <"/proc/$tpid/cmdline"
  tstarted=0
  if [ -n "$tpid" ] && st=$(cat "/proc/$tpid/stat" 2>/dev/null); then
    read -ra f <<<"${st##*) }"
    tstarted=$(($(awk '/^btime /{print $2}' /proc/stat) * 1000 + ${f[19]:-0} * 1000 / $(getconf CLK_TCK)))
  fi
  code=$([ -n "$tlist" ] && [ -n "$thost" ] && curl -s -m 5 -o /dev/null -w '%{http_code}' -H "Host: $thost" "$tlist/" 2>/dev/null) || code=
  echo "-- bt-tray: 控え ${tpid:+pid $tpid・}${tlist:-無い}・argv ${targv[*]:-読めない}・起動 $( [ "$tstarted" -gt 0 ] && date -u -d "@$((tstarted / 1000))" +%Y-%m-%dT%H:%M:%SZ || echo ?)・GET / ${code:-答えない}"
  if [ "${#targv[@]}" = 3 ] && [ "${targv[0]}" = "$pkg/bin/andon" ] && [ "${targv[1]} ${targv[2]}" = "tray serve" ] && [ "$tstarted" -ge "$now_ms" ] && [ "$code" = 200 ]; then
    ok "bt-tray: ローカルの tray が当てたパッケージで、最後に当てた時刻より後に起動し、一覧の面が答える"
  else
    fail "bt-tray: 控えが無いか、控えの pid が当てたパッケージの serve でないか、当てる前から動いているか、一覧の面が 200 を返さない"
  fi
fi

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

# bt-managed: managed settings が当てたパッケージの hook を指す
jq -e --arg h "$pkg/hooks/" '[.hooks[][].hooks[]?.command] | length > 0 and all(startswith($h))' "$managed" >/dev/null \
  && ok "bt-managed: managed settings の hook は全部 $pkg/hooks を指す" || fail "bt-managed: managed settings の hook が $pkg/hooks を指さない"

# bt-skills: パッケージの skill（<pkg>/skills/<名>/SKILL.md）ごとに、~/.agents/skills/andon-<名> の symlink がその置き場を指す。
# 余分な andon-* の symlink がパッケージの外を指していても落とす
want=$(find "$pkg/skills" -mindepth 2 -maxdepth 2 -name SKILL.md -printf '%h\n' 2>/dev/null | sort)
echo "-- bt-skills: パッケージの skill $(grep -c . <<<"$want" || true) 本"
[ -n "$want" ] || fail "bt-skills: $pkg/skills に SKILL.md を持つ skill が無い"
while IFS= read -r dir; do
  [ -n "$dir" ] || continue
  l=$HOME/.agents/skills/andon-$(basename "$dir")
  t=$(readlink -f "$l" 2>/dev/null) || t=
  echo "  | $(basename "$l") → ${t:-無い}"
  [ -L "$l" ] && [ "$t" = "$(readlink -f "$dir")" ] || fail "bt-skills: $l が $dir を指す symlink でない"
done <<<"$want"
while IFS= read -r l; do
  [ -n "$l" ] || continue
  case "$(readlink -f "$l")" in "$(readlink -f "$pkg/skills")"/*) ;; *) fail "bt-skills: $l がパッケージの外（$(readlink "$l")）を指す" ;; esac
done < <(find "$HOME/.agents/skills" -maxdepth 1 -name 'andon-*' -type l 2>/dev/null)
[ "$fails" = 0 ] && ok "bt-skills: パッケージの skill ごとに andon-<名> の symlink が置き場を指す"
[ "$fails" = 0 ]
