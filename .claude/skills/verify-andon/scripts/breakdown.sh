# breakdown.sh: journeys/otel.sh が cell.sh のあとに source する。結果の記録の内訳を比べる breakdown_check を持つ（ot-breakdown。ADR-0060 受け入れ条件 16）。
# 読むだけ。何も書かない・何も起動しない。tests/verify-andon.sh が偽のセルで直に呼んで、判定を確かめる。
# 使うもの: cell.sh の $ws・$a_at・since・ok・fail・$fails。BREAKDOWN_GRACE（秒。既定 600）は、done から結果の記録を待つ間。
#
# 当てたあとに Claude Code の担当として起動した課題ごとに:
#   - まだ done でなければ数えない。
#   - done から BREAKDOWN_GRACE を過ぎても outcome.json が無ければ fail（dispatcher が結果の記録を書けていない）。過ぎていなければ note。
#   - outcome.json に breakdown のキーが無ければ fail（当てた dispatcher が書いていない）。
#   - breakdown が null なのに課題の置き場の traces.jsonl に行があれば fail（記録があるのに数えていない）。行が無ければ note（受け口が止まっていた）。
#   - 内訳のトークンの合計（4 種の和）が sessions[].cost のトークンの合計の上下 5% に入らなければ fail、入れば ok に数える。
#   - 内訳が null でなければ、知識の読み breakdown.knowledge（ot-knowledge。ADR-0061 受け入れ条件 13）を見る。キーが無ければ fail
#     （当てた dispatcher が数えていない）。読んだページがあれば、その課題を ok に数える。ページが空なのに課題の置き場の
#     traces.jsonl が wiki のページ（<ANDON_WORKSPACE>/wiki/ の entities・topics・sources の下の .md）を名指していれば fail（数えていない）。
# 比べた課題があれば ok、片付いた課題が無いか比べられなければ note（未確認）。ot-knowledge は、ページを読んだ課題があれば ok、
# 無ければ note（wiki のページを読んだ担当の課題を Chief of Staff に 1 件片付けてもらってから打ち直す）。
breakdown_check() {
  local root=${XDG_STATE_HOME:-$HOME/.local/state}/andon/otel grace=${BREAKDOWN_GRACE:-600} now kanban id slug o oat done_at bsum ssum tr kpages kraw
  local nb=0 ndone=0 bpairs=() nk=0 kread=()
  now=$(date -u +%s)
  while IFS=$'\t' read -r kanban id; do
    [ -n "$id" ] || continue
    slug=$(basename "$(dirname "$kanban")")
    done_at=$(jq -r --arg id "$id" 'select(.id == $id and (.change.status? // null) != null) | [.at, .change.status] | @tsv' "$kanban/log.jsonl" 2>/dev/null | tail -n 1)
    [ "${done_at#*$'\t'}" = done ] || continue
    done_at=${done_at%%$'\t'*}
    ndone=$((ndone + 1))
    o=$kanban/issues/$id/library/outcome.json
    if [ ! -f "$o" ]; then
      if [ $((now - $(date -u -d "$done_at" +%s))) -ge "$grace" ]; then
        fail "ot-breakdown: $slug/$id は $done_at に done になったのに、結果の記録 $o が無い（dispatcher が書けていない）"
      else
        echo "note ot-breakdown: $slug/$id は $done_at に done になったばかりで、結果の記録はまだ無い。次の見回りのあとに打ち直す"
      fi
      continue
    fi
    oat=$(jq -r '.at // ""' "$o" 2>/dev/null) || oat=
    if [[ "$oat" < "$a_at" ]]; then continue; fi
    if [ "$(jq -r 'has("breakdown")' "$o" 2>/dev/null)" != true ]; then
      fail "ot-breakdown: $o に breakdown のキーが無い（当てた dispatcher が書いていない）"
      continue
    fi
    read -r bsum ssum < <(jq -r '[([.breakdown.tokens // {} | .[]] | add // "null"), ([.sessions // [] | .[].cost.tokens // {} | .[]] | add // "null")] | @tsv' "$o" 2>/dev/null) || true
    echo "  | $slug/$id: 内訳のトークン ${bsum:-?}・セッションの記録のトークン ${ssum:-?}"
    if [ "${bsum:-null}" = null ]; then
      tr=$root/$slug/issues/$id/traces.jsonl
      if [ -n "$(grep -m 1 -v '^[[:space:]]*$' "$tr" 2>/dev/null)" ]; then
        fail "ot-breakdown: $slug/$id は置き場 $tr に記録があるのに、内訳が null（数えていないか、読めなかった。dispatcher のログを見る）"
      else
        echo "note ot-breakdown: $slug/$id は置き場に記録が無く、内訳が null（受け口が止まっていた）。比べていない"
      fi
      continue
    fi
    if [ "$bsum" = 0 ]; then
      : # モデルの呼び出しの span が無い内訳（Claude Code の span が無ければ knowledge も無い）は知識の読みを見ない
    elif [ "$(jq -r '.breakdown | has("knowledge")' "$o" 2>/dev/null)" != true ]; then
      fail "ot-knowledge: $o の内訳に knowledge のキーが無い（当てた dispatcher が知識の読みを数えていない）"
    else
      nk=$((nk + 1))
      kpages=$(jq -r '[.breakdown.knowledge.pages[]? | "\(.path)×\(.reads)"] | join(",")' "$o" 2>/dev/null) || kpages=
      echo "  | $slug/$id: 知識の読み 目録 $(jq -r '.breakdown.knowledge.index_reads // 0' "$o" 2>/dev/null)・ページ ${kpages:-無し}"
      tr=$root/$slug/issues/$id/traces.jsonl
      if [ -n "$kpages" ]; then
        kread+=("$id($kpages)")
      elif kraw=$(grep -o -m 1 -E "$ws/wiki/(entities|topics|sources)/[A-Za-z0-9_.@+/-]+\.md([^A-Za-z0-9_.@+/-]|$)" "$tr" 2>/dev/null); then
        fail "ot-knowledge: $slug/$id は置き場 $tr の記録が wiki のページを名指しているのに（${kraw%%.md*}.md）、内訳のページが空（数えていない）"
      fi
    fi
    if [ "${ssum:-null}" = null ] || [ "$ssum" = 0 ]; then
      echo "note ot-breakdown: $slug/$id はセッションの記録のトークンが無い（セッションの記録が読めない）。比べていない"
      continue
    fi
    nb=$((nb + 1))
    if awk -v b="$bsum" -v s="$ssum" 'BEGIN { d = (b - s) / s; if (d < 0) d = -d; exit !(d <= 0.05) }'; then
      bpairs+=("$id")
    else
      fail "ot-breakdown: $slug/$id の内訳のトークン $bsum がセッションの記録のトークン $ssum の上下 5% に入らない。端末を片付けるときに最後の分が送られ切っていないかを調べる"
    fi
  done < <(for f in "$ws"/projects/*/kanban/issues/*/library/launches.jsonl; do
    since "$f" | jq -r --arg k "${f%/issues/*}" 'select(.role == "worker" and ((.profile // "") | test("codex") | not)) | [$k, .issue] | @tsv'; done | sort -u)
  if [ "$nb" -gt 0 ]; then
    ok "ot-breakdown: 当てたあとに片付いた担当の課題 $nb 件の結果の記録に内訳があり、トークンの合計がセッションの記録の上下 5% に入る（${bpairs[*]}）"
  elif [ "$ndone" = 0 ]; then
    echo "note ot-breakdown: 当てたあとに起動した Claude Code の担当の課題が、まだ片付いていない。Chief of Staff に試しの課題を 1 件片付けてもらってから打ち直す"
  else
    echo "note ot-breakdown: 当てたあとに片付いた担当の課題 $ndone 件は、どれも比べられなかった（上の note）"
  fi
  if [ "${#kread[@]}" -gt 0 ]; then
    ok "ot-knowledge: 当てたあとに片付いた担当の課題の内訳に、読んだ wiki のページがある（${kread[*]}）"
  else
    echo "note ot-knowledge: 当てたあとに片付いた担当の課題 $nk 件の内訳に、読んだ wiki のページが無い。wiki のページを読んだ担当の課題を Chief of Staff に 1 件片付けてもらってから打ち直す"
  fi
}
