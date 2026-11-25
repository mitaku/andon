#!/usr/bin/env bash
# ハーネスの受け口（features/otel.md）。受け口が listen しているかと、当てたあとに起動した Claude Code の担当・レビュアーの
# 課題の置き場（~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl）を読む。読むだけ。何も起動しない・止めない。
# 当てたあとに Claude Code で起動した課題がまだ無いか、どの課題もまだ記録を持たなければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
root=${XDG_STATE_HOME:-$HOME/.local/state}/andon/otel

# ot-listen: 127.0.0.1:4318 が andon の受け口の名前を返す（今の状態）
name=$(curl -s -m 2 http://127.0.0.1:4318/ 2>/dev/null | tr -d '\r\n') || name=
echo "-- ot-listen: GET http://127.0.0.1:4318/ → ${name:-返らない}"
[ "$name" = andon-otel ] && ok "ot-listen: 受け口が listen し、名前 andon-otel を返す" || fail "ot-listen: 受け口が listen していないか、ほかのプロセスがポートを取っている"
# 当てる道具は受け口を起動し直す（ot-applied）が、前のバージョンの道具で当てたときや失敗したときは残る。検証する commit を当てる前から動く受け口は、
# その commit の受け口のコード（internal/otel/ の受けて書く処理・落とす属性）を持たない。記録の読み方が変わるので書き出す。
# 見えるのは今動いている受け口だけで、検証する間に記録を書いた受け口が別のこともある（起動が当てた時刻より後のとき）。
# 起動の時刻は ps の lstart（秒まで。ps と date は同じ TZ で読む）。当てた時刻と同じ秒の起動は当てたあとに数える。
rfound=0
while read -r rpid rargs; do
  case "$rargs" in *"andon otel serve"*) ;; *) continue ;; esac
  rfound=$((rfound + 1))
  rs=$(date -d "$(ps -o lstart= -p "$rpid" 2>/dev/null)" +%s 2>/dev/null) || rs=
  rstart=$([ -n "$rs" ] && date -u -d "@$rs" +%Y-%m-%dT%H:%M:%SZ || echo ?)
  rexe=$(readlink "/proc/$rpid/exe" 2>/dev/null) || rexe=?
  echo "  | 受け口: pid $rpid・起動 $rstart・実体 $rexe"
  if [ "$rstart" = ? ]; then
    echo "note ot-listen: 受け口の起動の時刻が読めない。置き場の記録を当てた commit の受け口が書いたかは確かめていない"
  elif [[ "$rstart" < "$a_at" ]]; then
    echo "note ot-listen: 受け口は検証する commit を当てた $a_at より前から動いている。ot-codex・ot-records の落とした・残ったの判定（ok も FAIL も）は前のコードの受け口の結果で、検証する commit の internal/otel/ の受けて書く処理は確かめていない"
  else
    echo "note ot-listen: 受け口は $rstart に起動した。検証する間（$a_at から）のうちそれより前の記録は、今は動いていない別の受け口が書いた"
  fi
done < <(ps -eo pid=,args= 2>/dev/null)
[ "$rfound" -gt 0 ] || echo "note ot-listen: 受け口のプロセスが ps に見えない（ps が無いか、別の名前空間）。置き場の記録を書いた受け口のコードは確かめていない"

# ot-applied: 受け口のプロセス（argv が「<パッケージ>/bin/andon otel serve」）の実体が当てたパッケージで、起動の時刻が最後に当てた時刻より後（今の状態）
rpid=
for p in /proc/[0-9]*; do
  mapfile -d '' -t argv 2>/dev/null <"$p/cmdline" || continue
  if [ "${#argv[@]}" = 3 ] && [ "${argv[0]}" = "$pkg/bin/andon" ] && [ "${argv[1]} ${argv[2]}" = "otel serve" ]; then rpid=${p#/proc/}; fi
done
if [ -z "$rpid" ]; then
  fail "ot-applied: $pkg/bin/andon otel serve のプロセスが無い"
else
  exe=$(readlink "/proc/$rpid/exe" 2>/dev/null) || exe=
  st=$(cat "/proc/$rpid/stat" 2>/dev/null) || st=
  read -ra f <<<"${st##*) }"
  btime=$(awk '/^btime /{print $2}' /proc/stat)
  started_ms=$((btime * 1000 + ${f[19]:-0} * 1000 / $(getconf CLK_TCK))) # stat の 22 番目（状態から数えて 20 番目）は起動からの tick
  echo "-- ot-applied: pid $rpid・実体 ${exe:-読めない}・起動 $(date -u -d "@$((started_ms / 1000))" +%Y-%m-%dT%H:%M:%SZ)"
  if [ -n "$exe" ] && [[ "$exe" != *"(deleted)" ]] && [ "$started_ms" -ge "$now_ms" ]; then
    ok "ot-applied: 受け口の実体は当てたパッケージで、最後に当てた時刻より後に起動している"
  else
    fail "ot-applied: 受け口が前のパッケージのまま動いている（実体が消えたファイルか、起動が最後に当てた時刻より前）"
  fi
fi

# ot-codex: 当てたあとに Codex のプロファイルで起動した課題ごとに、置き場の traces.jsonl と logs.jsonl を読む（ADR-0060 受け入れ条件 17）。
# 記録が無いことは報告に書く条件なので、note にして道の終了コードを変えない。個人の属性かツールの出力が残っていれば fail。
# 数えるのは、その課題を当てたあとに Codex で最初に起動した時刻より後の時刻（span の startTimeUnixNano・ログの timeUnixNano）を
# 持つ行だけ（同じ課題の前の起動の行で ok にしない）。起動の記録の時刻は秒までなので、起動と同じ秒の行は前の起動のものと
# 見分けられず、数えない（次の秒の頭から数える。Codex は立ち上がりに 1 秒より長くかかる）。
# 個人の属性とツールの出力は、前の行も含めてファイルの全部を見る。
# codex_tied は、resource に andon.issue と andon.role があり、時刻 $t（ナノ秒）以後の記録を持つ行を出す。
# Codex のログは timeUnixNano を "0" で送り、時刻は observedTimeUnixNano にだけある（v0.166.0）ので、それも読む。
codex_tied='select([.. | objects | (.startTimeUnixNano?, .timeUnixNano?, .observedTimeUnixNano?) | select(type == "string" or type == "number") | tonumber] | (max // 0) >= $t)
  | (.resourceSpans // .resourceLogs)[0].resource.attributes | map(.key) | select(index("andon.issue") != null and index("andon.role") != null)'
cx=0
cx_fails0=$fails
prev=
while IFS=$'\t' read -r kanban id at; do
  [ -n "$id" ] || continue
  [ "$kanban/$id" != "$prev" ] || continue # 同じ課題は最初の起動の時刻で数える（並びは課題ごとに時刻の順）
  prev=$kanban/$id
  slug=$(basename "$(dirname "$kanban")")
  t=$(date -u -d "$at" +%s 2>/dev/null) || { echo "  | $slug/$id: 起動の時刻 $at が読めない"; continue; }
  for kind in traces logs; do
    f=$root/$slug/issues/$id/$kind.jsonl
    [ -f "$f" ] || { echo "  | $slug/$id: Codex の $kind.jsonl が無い"; continue; }
    tied=$(jq -c --argjson t "$((t + 1))000000000" "$codex_tied" "$f" 2>/dev/null | grep -c . || true)
    email=$(grep -c '"user.email"' "$f" || true)
    output=$(jq -c '.. | objects | select(.attributes? and ((.attributes | map(select(.key == "event.name")) | .[0].value.stringValue) == "codex.tool_result" or .eventName? == "codex.tool_result")) | .attributes[] | select(.key == "output")' "$f" 2>/dev/null | grep -c . || true)
    echo "  | $slug/$id: Codex の $kind.jsonl の、起動（$at）より後で andon.issue・andon.role を持つ行 $tied・user.email を持つ行 $email・codex.tool_result の output $output"
    [ "$email" = 0 ] || fail "ot-codex: $f に user.email が残っている"
    [ "$output" = 0 ] || fail "ot-codex: $f に codex.tool_result の output が残っている"
    [ "$tied" -gt 0 ] && cx=$((cx + 1))
  done
done < <(for f in "$ws"/projects/*/kanban/issues/*/library/launches.jsonl; do
  since "$f" | jq -r --arg k "${f%/issues/*}" 'select(.profile | test("codex")) | [$k, .issue, .at] | @tsv'; done | sort -t$'\t' -k1,1 -k2,2 -k3,3)
if [ "$fails" -gt "$cx_fails0" ]; then
  : # ot-codex の FAIL の行が理由。ok を出さない
elif [ "$cx" -gt 0 ]; then
  ok "ot-codex: 当てたあとに Codex で起動した課題の記録 $cx 本が課題に結ばれ、user.email と codex.tool_result の output が無い"
else
  echo "note ot-codex: 当てたあとに Codex で起動した課題の記録が無い（Codex の起動が無いか、送っていない・結べていない）。受け入れ条件 17 のとおり、その事実を報告に書く"
fi

# ot-records: 当てたあとに Claude Code のプロファイル（codex でない）で起動した課題ごとに、置き場の traces.jsonl を読む
lines=$(since "$ws"/projects/*/kanban/issues/*/library/launches.jsonl | jq -c 'select(.profile | test("codex") | not)')
n=$(grep -c . <<<"$lines" || true)
echo "-- ot-records: 当てたあとに Claude Code で起動した課題の起動 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに Claude Code の担当もレビュアーも起動していない。Chief of Staff が次に課題を起動したあとで打ち直す"; exit 3; }
seen=0 seenl=()
while IFS=$'\t' read -r kanban id; do
  [ -n "$id" ] || continue
  slug=$(basename "$(dirname "$kanban")")
  f=$root/$slug/issues/$id/traces.jsonl
  [ -f "$f" ] || { echo "  | $slug/$id: 記録が無い（まだターンが無いか、受け口が止まっていた）"; continue; }
  llm=$(jq -c 'select(.resourceSpans[0] | (.resource.attributes | map(.key) | (index("andon.issue") != null and index("andon.role") != null))
    and ([.scopeSpans[].spans[].name] | index("claude_code.llm_request") != null))' "$f" 2>/dev/null | grep -c . || true)
  email=$(grep -c '"user.email"' "$f" || true)
  echo "  | $slug/$id: claude_code.llm_request と andon.issue・andon.role を持つ行 $llm・user.email を持つ行 $email"
  [ "$email" = 0 ] || fail "ot-records: $f に user.email が残っている"
  if [ "$llm" -gt 0 ]; then seen=$((seen + 1)); tk=$kanban tid=$id; seenl+=("$kanban"$'\t'"$id"); fi
done < <(for f in "$ws"/projects/*/kanban/issues/*/library/launches.jsonl; do
  since "$f" | jq -r --arg k "${f%/issues/*}" 'select(.profile | test("codex") | not) | [$k, .issue] | @tsv'; done | sort -u)
[ "$fails" = 0 ] || exit 1
[ "$seen" -gt 0 ] || { echo "note 当てたあとに起動した課題のどれにも、まだ claude_code.llm_request の記録が無い。担当が 1 ターン以上動いたあとで打ち直す"; exit 3; }
ok "ot-records: 当てたあとに起動した $seen 件の課題の記録に claude_code.llm_request と andon.issue・andon.role があり、user.email が無い"

# ot-trace: ot-records が数えた課題の 1 つで、当てたパッケージの andon issue trace --otlp を打つ（読むだけ）。
# span がどれも同じ traceId で、親の無い span が 1 つだけで、ハーネスの claude_code.llm_request が入っている。
# 数えた課題のうち、レビュアーの課題（<id>-r<k>。issue.yaml があるもの）に dispatcher の控えの gone の項目がある課題を先に選び、
# その項目の数だけ「端末が死んでいた」の段がレビューの段の下に出ることも見る（無ければ未確認の note）
dstate=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher
rgone=0
for p in "${seenl[@]}"; do
  k=${p%%$'\t'*} i=${p#*$'\t'}
  n=0
  while IFS=$'\t' read -r rid rn; do
    [ -n "$rid" ] && [ -f "$k/issues/$rid/issue.yaml" ] && n=$((n + rn))
  done < <(jq -r --arg i "$i" '.gone // {} | to_entries[] | select(.key | startswith($i + "-r")) | [.key, (.value | length)] | @tsv' "$dstate/$(basename "$(dirname "$k")").json" 2>/dev/null)
  if [ "$n" -gt 0 ]; then tk=$k tid=$i rgone=$n; break; fi
done
tout=$(cd "$(dirname "$tk")" && KANBAN_ROOT=$(dirname "$tk") "$pkg/bin/andon" issue trace "$tid" --otlp 2>&1) && trc=0 || trc=$?
read -r ntrace nroot nllm nall < <(jq -r '[.resourceSpans[].scopeSpans[].spans[]]
  | [(map(.traceId) | unique | length), (map(select((.parentSpanId // "") == "")) | length), (map(select(.name == "claude_code.llm_request")) | length), length] | @tsv' <<<"$tout" 2>/dev/null) || true
# interaction の親（起動の span）の andon.session と interaction の session.id を突き合わせる。合う数・合わない数（session の合う起動が無く、時刻で入れたもの）
read -r nsame ndiff < <(jq -r '[.resourceSpans[].scopeSpans[].spans[]] as $s
  | ($s | map({key: .spanId, value: ((.attributes // []) | map(select(.key == "andon.session")) | .[0].value.stringValue // "")}) | from_entries) as $sess
  | [$s[] | select(.name == "claude_code.interaction") | {p: ($sess[.parentSpanId] // ""), s: ((.attributes // []) | map(select(.key == "session.id")) | .[0].value.stringValue // "")} | select(.p != "")]
  | [(map(select(.p == .s)) | length), (map(select(.p != .s)) | length)] | @tsv' <<<"$tout" 2>/dev/null) || true
ngone=$(jq -r '[.resourceSpans[].scopeSpans[].spans[]] as $s | ($s | map({key: .spanId, value: .name}) | from_entries) as $n
  | [$s[] | select((.name | startswith("端末が死んでいた")) and (($n[.parentSpanId] // "") | startswith("レビュー ")))] | length' <<<"$tout" 2>/dev/null) || ngone=
echo "-- ot-trace: レビュアーの課題の dispatcher の控えの gone の項目 $rgone・レビューの段の下の「端末が死んでいた」の段 ${ngone:-読めない}"
[ "$rgone" -gt 0 ] || echo "note ot-trace: 数えた課題のレビュアーの課題に、dispatcher の控えの gone の項目が無い。レビュアーの端末が死んでいた間をレビューの段の下に出すことは確かめていない"
echo "-- ot-trace: $(basename "$(dirname "$tk")")/$tid の andon issue trace --otlp: 終了コード $trc・span ${nall:-読めない}・traceId ${ntrace:-?} 種・親の無い span ${nroot:-?}・claude_code.llm_request ${nllm:-?}・同じ session の起動の下の interaction ${nsame:-?}・session の合わない起動の下の interaction ${ndiff:-?}"
[ "${ndiff:-0}" = 0 ] || echo "note ot-trace: session の合う起動が無く、時刻で前の起動の下に入れた interaction が ${ndiff} 個ある（起動の記録に session が無いか、記録の外で起動し直した）"
if [ "$trc" = 0 ] && [ "${ntrace:-0}" = 1 ] && [ "${nroot:-0}" = 1 ] && [ "${nllm:-0}" -gt 0 ] && [ "${nsame:-0}" -gt 0 ] && { [ "$rgone" = 0 ] || [ "${ngone:-0}" = "$rgone" ]; }; then
  ok "ot-trace: 課題 $tid の流れが 1 本のトレースになり、ハーネスのモデルの呼び出しが入り、interaction が同じ session の起動の下に入っている$([ "$rgone" = 0 ] || echo "。レビュアーの端末が死んでいた間 $rgone 件がレビューの段の下にある")"
else
  fail "ot-trace: andon issue trace $tid --otlp が 1 本のトレースにならないか、結び先が違う（出力の先頭: $(sed -n 1p <<<"$tout" | cut -c1-200)）"
fi
# 段の表の 2 行（ADR-0061 決めたこと 16）: 依頼書が wiki のページを名指す課題と、report.json に申告のある課題を 1 件ずつ探し、
# 当てたパッケージの andon issue trace --json にその段（brief_wiki・gaps）が出ることを見る（読むだけ。無ければ未確認の note）
for want in brief_wiki gaps; do
  sk= sid=
  for d in "$ws"/projects/*/kanban/issues/*/; do
    case $want in
      brief_wiki) grep -qE "$ws/wiki/(entities|topics|sources)/[A-Za-z0-9_.@+/-]+\.md|(^|[^A-Za-z0-9_.@+/~-])wiki/(entities|topics|sources)/[A-Za-z0-9_.@+/-]+\.md" "$d/brief.md" 2>/dev/null || continue ;;
      gaps) jq -e '(.gaps // []) | length > 0' "$d/library/report.json" >/dev/null 2>&1 || continue ;;
    esac
    sk=${d%/issues/*} sid=$(basename "$d"); break
  done
  if [ -z "$sid" ]; then
    echo "note ot-trace: 段 $want の元の記録を持つ課題が無い。この段は確かめていない"
    continue
  fi
  nst=$(cd "$(dirname "$sk")" && KANBAN_ROOT=$(dirname "$sk") "$pkg/bin/andon" issue trace "$sid" --json 2>/dev/null | jq -r --arg k "$want" '[.tree.children[] | select(.kind == $k)] | length' 2>/dev/null) || nst=
  echo "-- ot-trace: $(basename "$(dirname "$sk")")/$sid の段 $want ${nst:-読めない}"
  [ "${nst:-0}" -gt 0 ] && ok "ot-trace: 課題 $sid の流れに段 $want が出る" || fail "ot-trace: 課題 $sid は段 $want の元の記録を持つのに、andon issue trace --json にその段が無い"
done
[ "$fails" = 0 ] || exit 1

# ot-breakdown: 当てたあとに片付いた Claude Code の担当の課題の結果の記録の内訳を比べる（ADR-0060 受け入れ条件 16。判定は scripts/breakdown.sh）
. "$(dirname "$0")/../breakdown.sh"
breakdown_check
[ "$fails" = 0 ] || exit 1
echo "note ot-restart（受け口を止めてから担当を起動する）はこの道では動かさない。features/otel.md の手順で、Chief of Staff の試しの課題で確かめる"
