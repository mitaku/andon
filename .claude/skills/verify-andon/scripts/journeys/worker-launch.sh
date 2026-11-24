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

# wl-resume: 当てたあとに andon issue resume（launch が resume。--fresh の fresh は印を付けないので数えない）で起動した端末の最初のメッセージの
# 最後の段落が、送り手の印 [resume] で始まる。hc read の画面（履歴を含む）に印つきの行が見えれば ok。Claude Code は長い貼り付けを畳み、
# 古い行は流れるので、見えなければ起動の記録の session のセッションの記録（<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl）で、
# 起動の at より後の最初の「の続き。セルが起き直した」の user の文を読む。貼った文の包みを外した最後の段落が [resume] で始まらなければ壊れ。
# どちらでも読めない（端末が片付いた・Codex・記録が無い）ものは未確認。1件も確かめられなければ終了コード 3
res=$(jq -c 'select(.launch == "resume")' <<<"$lines")
nres=$(grep -c . <<<"$res" || true)
echo "-- wl-resume: 当てたあとの resume の起動 $nres 件"
conf=$(mktemp "$tmp/wl-resume.XXXXXX")
cfg=${CLAUDE_CONFIG_DIR:-$HOME/.claude}
while IFS=$'\t' read -r at issue addr harness session; do
  [ -n "$at" ] || continue
  screen=$(hc read "$addr" --lines 5000 2>/dev/null || true)
  if grep -qE "\[resume\] 課題 \`[^\`]*/$issue\` の続き" <<<"$screen"; then
    echo "  | $addr（$issue・$at）: 画面に印 [resume] の行がある"; echo x >>"$conf"; continue
  fi
  f=
  if [ "$harness" = claude ] && [ -n "$session" ]; then
    f=$(find "$cfg/projects" -mindepth 2 -maxdepth 2 -name "$session.jsonl" 2>/dev/null | sed -n 1p)
  fi
  [ -n "$f" ] || { echo "  | $addr（$issue・$at）: 画面に印が見えず、セッションの記録も読めない（harness $harness・session ${session:--}）。未確認"; continue; }
  last=$(jq -r --arg t "${at:0:19}" 'select(.type == "user" and (.timestamp // "")[0:19] >= $t)
    | .message.content | if type == "string" then . elif type == "array" then (map(select(.type == "text") | .text) | join("\n")) else empty end
    | select(contains("の続き。セルが起き直した"))
    | sub("^\\s+"; "") | sub("\\s+$"; "") | sub("^<pasted_content[^>]*>\\s*"; "") | sub("\\s*</pasted_content[^>]*>$"; "")
    | split("\n\n") | last | sub("^\\s+"; "") | @json' "$f" 2>/dev/null | sed -n 1p)
  if [ -z "$last" ]; then
    echo "  | $addr（$issue・$at）: セッションの記録 $f に、起動のあとの続きの文が無い。未確認"
  elif jq -e 'startswith("[resume] ")' <<<"$last" >/dev/null; then
    echo "  | $addr（$issue・$at）: セッションの記録の最初のメッセージの最後の段落が [resume] で始まる"; echo x >>"$conf"
  else
    fail "wl-resume: $addr（$issue・$at）の最初のメッセージの最後の段落が [resume] で始まらない: $(jq -r '.[0:80]' <<<"$last")"
  fi
done < <(jq -r '[.at, .issue, .terminal, (.harness.name // ""), (.session // "")] | @tsv' <<<"$res")
nconf=$(grep -c . "$conf" || true)
[ "$fails" = 0 ] || exit 1
if [ "$nconf" = 0 ]; then
  echo "note wl-resume: 当てたあとの resume の起動を、画面でもセッションの記録でも確かめられなかった（起動 $nres 件）。wl-record と wl-terminal は確かめたが、wl-resume は未確認。Chief of Staff が andon issue resume で担当を起動し直したあとで打ち直す"
  exit 3
fi
ok "wl-resume: 当てたあとの resume の起動 $nconf 件は、最初のメッセージの最後の段落が [resume] で始まる（確かめられなかったもの $((nres - nconf)) 件）"
