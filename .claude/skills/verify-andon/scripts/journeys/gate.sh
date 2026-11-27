#!/usr/bin/env bash
# gate（features/gate.md）。当てたあとに dispatcher が回した gate の記録（library/gate/<head>.json）を読む。読むだけ。
# 当てたあと（検証する commit が当たっていた間）に gate が1回も回っていなければ、gt-record・gt-route は読まず、
# gt-answer（dispatcher が閉じた問い。オーナーのマージでも閉じるので gate の記録が無くても起きる）だけを読んで 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"

recs=$(for f in "$ws"/projects/*/kanban/issues/*/library/gate/*.json; do
  [ -f "$f" ] && jq -c --arg f "$f" '. + {file: $f}' "$f" 2>/dev/null
done | dsince)
n=$(grep -c . <<<"$recs" || true)
nrec=$n # 下の gt-answer の read が n を問いの番号に使う
echo "-- gt-record: 当てたあとの gate の記録 $n 件"
if [ "$n" = 0 ]; then
  echo "note 当てたあとに gate が回っていない。gt-record・gt-route は未確認。次の課題が review に入ったあとで打ち直す"
else
  jq -c '{at, id, pr, head: .head[0:12], tier, decision, merged, reasons: (.reasons | length), checks: .github_checks.state, review: .review.state}' <<<"$recs" | sed 's/^/  | /'
  # behind（前の PR 待ち。ADR-0067 決めたこと 8）は、理由の 1 行目が前の課題を名指す決まった文で、dispatcher はマージしない（merged が false）
  bad=$(jq -c 'select((.decision | IN("approve", "fix", "needs_human", "behind") | not) or (.tier | IN("A", "B", "C") | not) or (.head | test("^[0-9a-f]{40}$") | not) or (.reasons | type) != "array"
    or (.decision == "behind" and (((.reasons[0] // "") | test("^前の課題 [A-Z]+-[0-9]+ の PR がまだ main に入っていない$") | not) or .merged != false)))' <<<"$recs")
  [ -z "$bad" ] && ok "gt-record: 当てたあとの gate $n 回は、全部が判定・レベル・head・理由の形を持つ（behind は前の課題を名指す理由が先頭で、マージしていない）" \
    || { fail "gt-record: 形の違う gate の記録がある"; sed 's/^/  | /' <<<"$bad"; }

  # gt-route: 判定が kanban に届いた（fix は担当へ、needs_human は問い、approve はマージかオーナーへ、behind は review のまま）。課題の今の status を並べる。
  # behind は問いを出さないので、その記録を名指す gate の問い（facts に「gate の記録 <パス>」）があれば落とす。
  # $ws を cwd にしても kanban は見つからない（projects/*/kanban が複数あり得る）ので、記録のパスから kanban の親を KANBAN_ROOT に渡す
  unread=0 f0=$fails
  while IFS=$'\t' read -r id decision merged file; do
    st=$(KANBAN_ROOT="${file%%/kanban/issues/*}" kanban show "$id" --json 2>/dev/null | jq -r '.issue.status // empty')
    [ -n "$st" ] || { st='?'; unread=$((unread + 1)); }
    echo "  | $id: 判定 $decision・merged $merged・今の status $st"
    [ "$decision" = behind ] || continue
    for q in "${file%/gate/*}"/questions/*.json; do
      [ -f "$q" ] || continue
      jq -e --arg f "gate の記録 $file" '.from == "gate" and ((.facts // []) | index($f))' "$q" >/dev/null 2>&1 \
        && fail "gt-route: 前の PR 待ち（behind）の記録 $file を名指す gate の問い ${q##*/} がある（behind は問いを出さない）"
    done
  done < <(jq -r '[.id, .decision, (.merged | tostring), .file] | @tsv' <<<"$recs")
  if [ "$unread" != 0 ]; then
    fail "gt-route: 課題 $unread 件の今の status を kanban show で読めない"
  elif [ "$fails" = "$f0" ]; then
    ok "gt-route: 判定ごとの課題の今の status を並べた（読み方は features/gate.md）。behind の記録を名指す gate の問いは無い"
  fi
fi

# gt-answer: dispatcher が閉じた問い（kanban の履歴で by が terminal:dispatcher の received）は、gate の問い（from: gate）だけ。
# dispatcher が答えも書いた問いは、印が machine で、「決め直した」の答えが名指す gate の記録が在る。閉じた問いが無ければ note（終了コードは変えない）
closed=0 f0=$fails
for k in "$ws"/projects/*/kanban; do
  [ -f "$k/log.jsonl" ] || continue
  while IFS=$'\t' read -r id n; do
    closed=$((closed + 1))
    from=$(jq -r '.from // "?"' "$k/issues/$id/library/questions/$n.json" 2>/dev/null || echo '?')
    q=$(KANBAN_ROOT="${k%/kanban}" kanban show "$id" --json 2>/dev/null | jq -c --argjson n "$n" '.questions[]? | select(.n == $n) | {answeredBy, decidedBy, answer}')
    echo "  | $id#$n: from $from・$(jq -r '"答えた \(.answeredBy // "?")・印 \(.decidedBy // "無し")・\(.answer // "答え無し")"' <<<"${q:-null}")"
    [ "$from" = gate ] || fail "gt-answer: dispatcher が gate の外の問い $id#$n（from $from）を閉じた"
    [ -n "$q" ] || { fail "gt-answer: 問い $id#$n を kanban show で読めない"; continue; }
    [ "$(jq -r '.answeredBy // empty' <<<"$q")" = terminal:dispatcher ] || continue
    [ "$(jq -r .decidedBy <<<"$q")" = machine ] || fail "gt-answer: dispatcher が答えた $id#$n の印が machine でない"
    rec=$(jq -r '.answer' <<<"$q" | sed -n 's/.*決め直した（記録 \([^）]*\)）.*/\1/p')
    [ -z "$rec" ] || [ -f "$rec" ] || fail "gt-answer: $id#$n の答えが名指す gate の記録 $rec が無い"
  done < <(dsince "$k/log.jsonl" | jq -r 'select(.by == "terminal:dispatcher" and .change.status == "received" and .change.question) | [.id, (.change.question | tostring)] | @tsv')
done
echo "-- gt-answer: 当てたあとに dispatcher が閉じた問い $closed 件"
if [ "$closed" = 0 ]; then
  echo "note gt-answer: 当てたあとに dispatcher が閉じた問いが無い。gate がオーナーへ回した PR の head が動くか、オーナーがマージしたあとで打ち直す"
elif [ "$fails" = "$f0" ]; then
  ok "gt-answer: dispatcher が閉じた問い $closed 件は全部 gate の問いで、dispatcher の答えは印が machine、名指す gate の記録が在る"
fi
# gt-skip: 当てたあとに PR 無しで review に入った課題（report.json に pr が無い。検証・マージ後のレビュー）のうち、
# 報告の課題（deliverable: report を書いた時点の Chief of Staff が書いた。ADR-0066 決めたこと 7）と、報告の commit が既定の branch に
# 入っているもの（自分の commit が無い）には、dispatcher が gate を回さず fix を送らない。報告の課題は commit を見ずに数える。
# 報告の課題と読むのは、review に入った行より前の deliverable を最後に変えた行が report で、その行の書き手がその行の時点の
# Chief of Staff の端末だったときだけ（印の無い間に正しく fix を受けた行を、あとから書いた印で数えない。照合は ledger.TrustedThen と同じ辿り方）。
# pr が無いときの gate の fix は、gate の記録も通知の記録も書かない。後から残るのは kanban の履歴で、fix を受けた担当は
# 自分の端末で kanban start を打って doing に戻す。そこで review に入った行の次の status の行が、同じ端末の doing でないことを見る。
# andon issue merge の記録（<slug>.merge.json）がその行の分を残していれば（課題が片付くと消える）、decision が fix でないことも見る。
# commit が既定の branch に入っているかは repo の clone（.ghq）で見る。読めなければ数えない
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher
skipped=0 f0=$fails
for k in "$ws"/projects/*/kanban; do
  [ -f "$k/log.jsonl" ] || continue
  slug=${k%/kanban}; slug=${slug##*/}
  cos=$(sed -n 's/^chief_of_staff: *//p' "$k/config.yaml" 2>/dev/null | tr -d "\"'" | sed -n 1p)
  rows=$(jq -c -n '[inputs] | to_entries[] | select(.value.change.status) | .value + {row: (.key + 1)}' "$k/log.jsonl")
  while IFS=$'\t' read -r id row by; do
    rep=$k/issues/$id/library/report.json
    [ -f "$rep" ] && [ -z "$(jq -r '.pr // empty' "$rep" 2>/dev/null)" ] || continue
    show=$(KANBAN_ROOT="${k%/kanban}" kanban show "$id" --json 2>/dev/null) || continue
    repo=$(jq -r '.issue.repo // empty' <<<"$show")
    [ -n "$repo" ] && [ "$(jq -r '.issue.deliverable // "pr"' <<<"$show")" != candidates ] || continue
    commit=$(jq -r '.commit // empty' "$rep")
    form=$(jq -rs --arg id "$id" --argjson r "$row" --arg cur "$cos" '
      def coss: [.[] | select(.config) | .config | select(has("chief_of_staff") or has("coordinator")) | (.coordinator // .chief_of_staff)];
      . as $all
      | ([$all[0:$r - 1] | to_entries[] | select(.value.id == $id and ((.value.change // {}) | has("deliverable")))] | last) as $d
      | if $d == null or $d.value.change.deliverable != "report" then "pr" else
          ($all[0:$d.key] | coss | last) as $then
          | (if $then != null then $then elif ($all | coss | length) > 0 then null else $cur end) as $c
          | if $c != null and $c != "" and $d.value.by == "terminal:" + $c then "report" else "pr" end
        end' "$k/log.jsonl" 2>/dev/null)
    if [ "$form" = report ]; then
      what="報告の課題（commit ${commit:0:12} は見ない）"
    else
      git -C "$ws/.ghq/$repo" merge-base --is-ancestor "$commit" refs/remotes/origin/HEAD 2>/dev/null || continue
      what="commit ${commit:0:12}（既定の branch に入っている）"
    fi
    skipped=$((skipped + 1))
    dec=$(jq -r --arg id "$id" --argjson r "$row" '.[$id] | select(. != null and .row == $r) | .decision // "決まっていない"' "$state/$slug.merge.json" 2>/dev/null)
    next=$(jq -r --arg id "$id" --argjson r "$row" 'select(.id == $id and .row > $r) | "\(.change.status) \(.by)"' <<<"$rows" | sed -n 1p)
    echo "  | $id: pr 無し・$what・review にした $by・次の status ${next:-無し（review のまま）}・merge の記録 ${dec:-無し}"
    [ "$dec" != fix ] || fail "gt-skip: PR を出さない課題 $id に gate が fix を返した（merge の記録）"
    [ "$next" != "doing $by" ] || fail "gt-skip: PR を出さない課題 $id を、review のあと担当（$by）が doing に戻した（fix を受けた形。Chief of Staff が戻したなら担当の画面で確かめる）"
  done < <(jq -c 'select(.change.status == "review")' <<<"$rows" | dsince | jq -r '[.id, (.row | tostring), .by] | @tsv')
done
echo "-- gt-skip: 当てたあとに PR 無しで review に入った、報告の課題か自分の commit の無い課題 $skipped 件"
if [ "$skipped" = 0 ]; then
  echo "note gt-skip: 当てたあとに、報告の課題か自分の commit の無い課題が PR 無しで review に入っていない。検証の課題が kanban review で返ったあとで打ち直す"
elif [ "$fails" = "$f0" ]; then
  ok "gt-skip: PR 無しで返った報告の課題か自分の commit の無い課題 $skipped 件は、gate の fix を受けた形（担当が doing に戻す・merge の記録の fix）が無い"
fi
[ "$fails" = 0 ] || exit 1
[ "$nrec" -gt 0 ] || exit 3
