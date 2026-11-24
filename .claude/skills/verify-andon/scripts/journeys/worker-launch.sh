#!/usr/bin/env bash
# 担当の起動（features/worker-launch.md）。当てたあとにセルで起動した担当・レビュアーの起動の記録（library/launches.jsonl）を読み、
# 当てたパッケージで起動したかと、resume で起動し直した端末の最初のメッセージに印 [resume] があるか（wl-resume）を見る。読むだけ。
# 当てたあとの起動がまだ無いか、resume の起動を1件も確かめられなければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"

lines=$(since "$ws"/projects/*/kanban/issues/*/library/launches.jsonl)
n=$(grep -c . <<<"$lines" || true)
echo "-- wl-record: 当てたあとの起動の記録 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに担当もレビュアーも起動していない。Chief of Staff が次に課題を起動したあとで打ち直す"; exit 3; }
jq -c '{at, issue, role, launch, profile, terminal, package: .package | {commit, source, drift}}' <<<"$lines" | sed 's/^/  | /'
bad=$(jq -c --arg c "$a_commit" --arg p "$pkg" 'select(.package.commit != $c or .package.drift != false or .package.root != $p)' <<<"$lines")
[ -z "$bad" ] && ok "wl-record: 当てたあとの起動 $n 件は、全部が当てた commit のパッケージ（$pkg・食い違い無し）で起動した" \
  || { fail "wl-record: 当てた commit と違うパッケージか、食い違いのあるパッケージで起動した"; sed 's/^/  | /' <<<"$bad"; }

# wl-terminal: 起動した端末のうち今も動いているものが、記録の template と課題のラベルで立っている（読むだけ）
while IFS=$'\t' read -r addr issue; do
  [ -n "$addr" ] || continue
  t=$(hc get "$addr" --json 2>/dev/null) || { echo "  | $addr（$issue）は片付いている"; continue; }
  echo "  | $addr（$issue）: $(jq -c '{state, labels: (.labels | {issue, role})}' <<<"$t")"
  [ "$(jq -r '.labels.issue // empty' <<<"$t")" = "$issue" ] || fail "wl-terminal: $addr のラベル issue= が $issue でない"
done < <(jq -r '[.terminal, .issue] | @tsv' <<<"$lines" | sort -u)
[ "$fails" = 0 ] && ok "wl-terminal: 動いている端末は、起動した課題のラベルを持つ"

# wl-review-brief: 当てたあとに dispatcher が起動したレビュアーの課題の依頼書が、対象の repo の REVIEW.md を先に読む指示を持つ（読むだけ）。
# レビュアーの起動が無ければ note を出して先へ進む（wl-resume は別に見る）
rv=$(jq -r 'select(.role == "reviewer" and .launch == "start") | .issue' <<<"$lines" | sort -u)
nrv=$(grep -c . <<<"$rv" || true)
echo "-- wl-review-brief: 当てたあとのレビュアーの起動 $nrv 件"
if [ "$nrv" = 0 ]; then
  echo "note wl-review-brief: 当てたあとにレビュアーが起動していない。Review: on の課題のレビューが起動したあとで打ち直す"
else
  rvbad=0
  while read -r issue; do
    brief=$(ls "$ws"/projects/*/kanban/issues/"$issue"/brief.md 2>/dev/null | sed -n 1p)
    if [ -z "$brief" ]; then
      fail "wl-review-brief: $issue の依頼書が読めない"; rvbad=1
    elif grep -qF '`REVIEW.md` があれば先に読み' "$brief"; then
      echo "  | $issue: 依頼書に REVIEW.md を読む指示がある"
    else
      fail "wl-review-brief: $issue の依頼書（$brief）に REVIEW.md を読む指示が無い"; rvbad=1
    fi
  done <<<"$rv"
  if [ "$rvbad" = 0 ]; then ok "wl-review-brief: 当てたあとのレビュアー $nrv 件の依頼書は、全部 REVIEW.md を読む指示を持つ"; fi
fi

# wl-resume: 当てたあとに andon issue resume（launch が resume。--fresh の fresh は印を付けないので数えない）で起動した端末の最初のメッセージに、
# 送り手の印 [resume] がある。判定は製品（spawn.Marked と振り返りの machineText）と同じで、貼った文の包みを外した文の頭か、最後の段落の頭が印。
# 読むのは、Claude Code なら起動の記録の session のセッションの記録（<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl）で、起動の at から、
# 同じ課題の次の起動か次に当てた時刻の前までの、最初のオーナーの印（origin.kind が human）の user の文（hc が打ち込んだ文はこの印を持つ。
# 道具の結果・裏の作業の知らせ〈origin.kind が task-notification〉・isMeta・isSidechain は除く。振り返りの typed と同じ見分け方）。印が無ければ壊れ。
# セッションの記録が読めない（Codex・記録が無い）ときは hc read の画面（履歴を含む）の、その課題の「の続き」の行を参考に出すだけで、未確認にする
# （画面は前の起動の文や担当の言い直しも映し、今回の起動の文と結び付けられないので、ok の証拠にも壊れの証拠にもしない）。1件も確かめられなければ終了コード 3
res=$(jq -c 'select(.launch == "resume")' <<<"$lines")
nres=$(grep -c . <<<"$res" || true)
echo "-- wl-resume: 当てたあとの resume の起動 $nres 件"
conf=$(mktemp "$tmp/wl-resume.XXXXXX")
cfg=${CLAUDE_CONFIG_DIR:-$HOME/.claude}
while IFS=$'\x1f' read -r at issue addr harness session; do
  [ -n "$at" ] || continue
  up=$(jq -rs --arg i "$issue" --arg a "$at" --arg u "$a_until" '[(.[] | select(.issue == $i and .at > $a) | .at), ($u | select(. != ""))] | min // empty' <<<"$lines")
  f=
  if [ "$harness" = claude ] && [ -n "$session" ]; then
    f=$(find "$cfg/projects" -mindepth 2 -maxdepth 2 -name "$session.jsonl" 2>/dev/null | sed -n 1p)
  fi
  if [ -n "$f" ]; then
    msg=$(jq -c --arg t "${at:0:19}" --arg u "${up:0:19}" 'select(.type == "user" and .origin.kind == "human" and .isMeta != true and .isSidechain != true
        and (.timestamp // "")[0:19] >= $t and ($u == "" or (.timestamp // "")[0:19] < $u))
      | .message.content | if type == "string" then . elif type == "array" then (map(select(.type == "text") | .text) | join("\n")) else "" end
      | select(length > 0)' "$f" 2>/dev/null | sed -n 1p)
    if [ -z "$msg" ]; then
      echo "  | $addr（$issue・$at）: セッションの記録 $f に、起動から ${up:-今} までの user の文が無い。未確認"
      continue
    fi
    if jq -e 'sub("^\\s+"; "") | sub("\\s+$"; "") | sub("^<pasted_content[^>]*>\\s*"; "") | sub("\\s*</pasted_content[^>]*>$"; "")
        | startswith("[resume] ") or (split("\n\n") | last | sub("^\\s+"; "") | startswith("[resume] "))' <<<"$msg" >/dev/null; then
      echo "  | $addr（$issue・$at）: セッションの記録の最初のメッセージに印 [resume] がある"; echo x >>"$conf"
    else
      fail "wl-resume: $addr（$issue・$at）の最初のメッセージの頭にも最後の段落の頭にも [resume] が無い: $(jq -r 'sub("\\s*</pasted_content[^>]*>\\s*$"; "") | split("\n\n") | last | .[0:80]' <<<"$msg")"
    fi
    continue
  fi
  seen=$(hc read "$addr" --lines 5000 2>/dev/null | grep -E "課題 \`[^\`]*/$issue\` の続き" || true)
  if [ -z "$seen" ]; then
    echo "  | $addr（$issue・$at）: セッションの記録が読めず（harness ${harness:--}・session ${session:--}）、画面にも続きの行が無い。未確認"
  elif grep -vqE "\[resume\] 課題 \`[^\`]*/$issue\` の続き" <<<"$seen"; then
    echo "  | $addr（$issue・$at）: 画面に印の無い続きの行がある（前の起動の文か、担当の言い直しかもしれない）。未確認"
  else
    # 続きから起動すると前の起動の印つきの文も画面に映り、今回の起動の文と結び付けられない。画面だけでは確かめたことにしない
    echo "  | $addr（$issue・$at）: 画面の続きの行は全部印 [resume] つきだが、今回の起動の文か前の起動の文かを画面では分けられない。未確認（参考）"
  fi
done < <(jq -r '[.at, .issue, .terminal, (.harness.name // ""), (.session // "")] | join("\u001f")' <<<"$res")
nconf=$(grep -c . "$conf" || true)
[ "$fails" = 0 ] || exit 1
if [ "$nconf" = 0 ]; then
  echo "note wl-resume: 当てたあとの resume の起動を、画面でもセッションの記録でも確かめられなかった（起動 $nres 件）。wl-record と wl-terminal は確かめたが、wl-resume は未確認。Chief of Staff が andon issue resume で担当を起動し直したあとで打ち直す"
  exit 3
fi
ok "wl-resume: 当てたあとの resume の起動 $nconf 件は、最初のメッセージに印 [resume] がある（確かめられなかったもの $((nres - nconf)) 件）"
