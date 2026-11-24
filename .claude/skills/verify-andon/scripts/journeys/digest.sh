#!/usr/bin/env bash
# ダイジェスト（features/digest.md）。切り離した kanban とセルの repo で、担当が打つ道（collect → build → send）をそのまま通す。
# 送り先（project.json の digest）を書かないので、send は files/ だけに出す。Slack へは何も送らない。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

# セルの repo（files/ の親。project.json を読む HEAD）。送り先の無い project.json を owner の commit で置く
export ANDON_WORKSPACE=$tmp/ws
ledger_init V
mkdir -p "$ANDON_WORKSPACE"
git init -q "$ANDON_WORKSPACE" && git -C "$ANDON_WORKSPACE" commit -q --allow-empty -m init

# 期間に片付いた課題 1 件と、ダイジェストの課題
done_id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の片付いた課題" --profile claude)
HC_TERMINAL_ID=tc "$kanban" set "$done_id" status=doing >/dev/null
HC_TERMINAL_ID=tc "$kanban" set "$done_id" status=done >/dev/null
id=$(HC_TERMINAL_ID=tc "$kanban" add "$run のダイジェスト" --profile claude)
HC_TERMINAL_ID=tc "$kanban" set "$id" status=doing >/dev/null
d=$tmp/project/kanban/issues/$id/library/digest
export HC_TERMINAL_ID=tw
echo "{\"labels\":{\"issue\":\"$id\"}}" >"$tmp/hc/tw.json"

# dg-collect: 期間の記録が input.json に集まり、片付いた課題が入る
step "dg-collect: 期間の記録を集める" "$andon" digest collect "$id" \
  && jq -e --arg d "$done_id" 'any(.issues[]; .id == $d)' "$d/input.json" >/dev/null \
  && ok "dg-collect: input.json が書かれ、期間に片付いた $done_id が入る" \
  || fail "dg-collect: input.json が無いか、片付いた課題が入らない"
jq -c '{since, until, issues: [.issues[] | {id, status}], skipped}' "$d/input.json" 2>/dev/null | sed 's/^/  input.json: /'

# dg-build-refuse: 入力に無い課題を指す文は、理由1行で断り何も書かない
bad="{\"headline\": [\"$run の要旨の1文目。\", \"2文目。\"], \"themes\": [{\"title\": \"無い課題\", \"body\": [\"入力に無い課題を指す。\"], \"issues\": [\"V-9999\"]}], \"concerns\": []}"
step "dg-build-refuse: 入力に無い課題を指す文を渡す" "$andon" digest build "$id" - <<<"$bad" \
  && fail "dg-build-refuse: 入力に無い課題を指したのに build が通った" \
  || { [ ! -e "$d/digest.json" ] && [ ! -e "$d/digest.html" ] \
    && ok "dg-build-refuse: 入力に無い課題を指す文は断り、digest.json と digest.html を書かない" \
    || fail "dg-build-refuse: 断ったのに digest.json か digest.html が書かれた"; }

# dg-build: 照らした文から digest.json・digest.html・message.txt が書かれる
good="{\"headline\": [\"$run の要旨の1文目。\", \"オーナーの判断を待っているものは無い。\"], \"themes\": [{\"title\": \"片付けた\", \"body\": [\"課題を1つ片付けた。\"], \"issues\": [\"$done_id\"]}], \"concerns\": []}"
step "dg-build: 担当の文を渡して組み立てる" "$andon" digest build "$id" - <<<"$good" \
  && [ -s "$d/digest.json" ] && grep -q '<svg' "$d/digest.html" && grep -qF "$run の要旨の1文目" "$d/message.txt" \
  && ok "dg-build: digest.json・図を持つ digest.html・要旨の入った message.txt が書かれる" \
  || fail "dg-build: 3つのファイルが揃わないか、中身が違う"
ls -l "$d" 2>/dev/null | sed 's/^/  | /'

# dg-send: 送り先の無い project.json では files/ だけに出し、URL と sent.json を残す
step "dg-send: 送る" "$andon" digest send "$id"
url=$(grep -m1 '^http' "$tmp/step.out")
f=$(find "$ANDON_WORKSPACE/files" -name '*.html' 2>/dev/null | sed -n 1p)
echo "  | files/: ${f:-無い}"
jq -c . "$d/sent.json" 2>/dev/null | sed 's/^/  sent.json: /'
[ -n "$url" ] && [ -n "$f" ] && cmp -s "$f" "$d/digest.html" && jq -e '(.destinations | length) > 0 and all(.destinations[]; .kind == "files" and .ok)' "$d/sent.json" >/dev/null \
  && ok "dg-send: digest.html が files/ にそのまま出て URL が返り、sent.json の送り先が files だけ" \
  || fail "dg-send: files/ に出ないか、URL か sent.json が無いか、files のほかの送り先がある"

# dg-check-refuse: 送り先が無ければ、確かめのファイルもどこへも送らない
step "dg-check-refuse: 送り先の無いまま確かめを打つ" "$andon" digest check \
  && fail "dg-check-refuse: 送り先が無いのに check が通った" \
  || { grep -qF '送り先の設定が無いので何も送らない' "$tmp/step.out" \
    && ok "dg-check-refuse: 送り先が無ければ何も送らずに断る" || fail "dg-check-refuse: 断りの理由が違う"; }

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
