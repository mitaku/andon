#!/usr/bin/env bash
# doctor.sh: このセルの Andon が動かす値打ちのある状態かを、読むだけで確かめる。何も書かない・何も起動しない。
#
# 見るもの: andon と kanban の置き場、`andon version` の commit と中身の食い違い、当てた記録（/opt/andon/applied.json）、
# dispatcher の端末が live で当てたあとに起動したか。最後に「当てた commit」を1行で出す（features.sh の範囲の右端に使う）。
# 終了コード: 0 は動かしてよい、1 は動かす前に直すものがある（理由を FAIL の行に出す）。
set -uo pipefail
ws=${ANDON_WORKSPACE:-/workspace}
fails=0
ok() { echo "ok   $*"; }
bad() { echo "FAIL $*"; fails=$((fails + 1)); }

andon=$(command -v andon) && ok "andon: $andon" || bad "andon が PATH に無い"
kanban=$(command -v kanban) && ok "kanban: $kanban（$(kanban --version 2>/dev/null | head -1)）" || bad "kanban が PATH に無い"
v=$(andon version 2>/dev/null) || v=
commit=$(jq -r '.commit // empty' <<<"$v" 2>/dev/null)
if [ -z "$commit" ]; then
  bad "andon version が commit を出さない"
elif [ "$(jq -r '.drift // false' <<<"$v")" = true ]; then
  bad "andon version が中身の食い違いを出す: $(jq -c '.reasons // .drift_reasons' <<<"$v")"
else
  ok "andon version: commit $commit（中身は commit と同じ）"
fi

applied=/opt/andon/applied.json
if [ -f "$applied" ]; then
  a_commit=$(jq -r .commit "$applied") a_at=$(jq -r .applied_at "$applied")
  ok "当てた記録: commit $a_commit（$a_at。イメージの元 $(jq -r .image_commit "$applied")）"
  [ "$a_commit" = "$commit" ] || bad "当てた commit $a_commit と andon version の commit $commit が違う"
else
  a_at=
  echo "note 当てた記録が無い（イメージのバージョンのまま）。当てたあとに起きた出来事を読む機能は、確かめられない"
fi

d=$(hc get dispatcher --json 2>/dev/null) || d=
if [ -z "$d" ]; then
  bad "dispatcher の端末が無い（hc get dispatcher）"
else
  st=$(jq -r .state <<<"$d") argv0=$(jq -r '.argv[0] // empty' <<<"$d") ms=$(jq -r '.created_ms // 0' <<<"$d")
  [ "$st" = live ] && ok "dispatcher: live（$(jq -r '.argv | join(" ")' <<<"$d")）" || bad "dispatcher が live でない: $st"
  case "$argv0" in /opt/andon/*) ;; *) bad "dispatcher が /opt/andon の andon で動いていない: $argv0" ;; esac
  if [ -n "$a_at" ]; then
    a_ms=$(($(date -u -d "$a_at" +%s) * 1000))
    [ "$ms" -ge "$a_ms" ] && ok "dispatcher は当てたあとに起動した" || bad "dispatcher が当てる前から動いている（apply-main.sh が boot を打ち直していない）"
  fi
fi

ls -d "$ws"/projects/*/kanban >/dev/null 2>&1 && ok "セルの本物の kanban: $(ls -d "$ws"/projects/*/kanban | tr '\n' ' ')" || bad "セルの kanban が $ws/projects/*/kanban に無い"
echo "commit $commit"
[ "$fails" = 0 ]
