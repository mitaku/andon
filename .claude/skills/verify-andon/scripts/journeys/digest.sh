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
# オーナーの判断 2 件: Chief of Staff（tc）が呼ぶもの（--now）と、置いておくもの（--later）を足す（dg-later）
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
q="{\"kind\": \"users\", \"question\": \"$run の書き出しは CSV だけにするか\", \"options\": [{\"label\": \"CSV だけ\", \"pick\": true}, {\"label\": \"Excel も\"}], \"from\": \"chief-of-staff\"}"
why="$run 急がない。答えを待つ仕事が無い"
called=$(HC_TERMINAL_ID=tc "$andon" issue ask --new --now - <<<"$q" | sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p')
placed=$(HC_TERMINAL_ID=tc "$andon" issue ask --new --later "$why" - <<<"${q/CSV だけにするか/並びを変えるか}" | sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p')
echo "  | 呼ぶ判断: ${called:-無い}。置いておく判断: ${placed:-無い}"
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
step "dg-collect: andon digest collect --help を打つ" "$andon" digest collect --help >/dev/null
grep -q '^プロジェクト: --project <slug>' "$tmp/step.out" && ! grep -q 'projects/<slug>/ .*で打つ' "$tmp/step.out" \
  && ok "dg-collect: --help は --project の行を持ち、projects/<slug>/ で打つと言わない" \
  || fail "dg-collect: --help に --project の行が無いか、projects/<slug>/ で打つと言う"

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

# dg-later: 冒頭の呼ばれている数と琥珀は呼んでいる行だけで、節 2 は呼んでいるものと置いてあるものに分かれ、短い文は 2 つの数を書く
grep -E -o '<h3>(呼んでいるもの|置いてあるもの)（[0-9]+ 件）</h3>|置いた理由: [^<]*' "$d/digest.html" 2>/dev/null | sed 's/^/  | /'
grep -E '^(オーナーを呼んでいるもの|置いてあるもの):' "$d/message.txt" 2>/dev/null | sed 's/^/  message.txt: /'
html=$(cat "$d/digest.html" 2>/dev/null) msg=$(cat "$d/message.txt" 2>/dev/null)
if [ -n "$called" ] && [ -n "$placed" ] && grep -qF '<div class="c-amber"><b>1</b><span>オーナーが呼ばれている</span>' <<<"$html" \
  && grep -qF '呼んでいるもの（1 件）' <<<"$html" && grep -qF '置いてあるもの（1 件）' <<<"$html" && grep -qF "置いた理由: $why" <<<"$html" \
  && [[ "${html#*置いてあるもの（}" == *"$placed"* ]] && [[ "${html#*置いてあるもの（}" != *"$called"* ]] \
  && grep -qF 'オーナーを呼んでいるもの: 1 件（マージの依頼 0・判断 1・手作業 0）' <<<"$msg" && grep -qF '置いてあるもの: 1 件' <<<"$msg"; then
  ok "dg-later: 呼ぶ $called は呼んでいるものに、置いた $placed は置いた理由つきで置いてあるものに分かれ、呼ばれている数と短い文の数は 1"
else
  fail "dg-later: 節 2 が分かれないか、置いた判断が呼んでいるものに入ったか、数が違う"
fi

# dg-send: 送り先の無い project.json では files/ だけに出し、置いたパスと sent.json を残す。
# 切り離した道はローカルの tray へ送らない（tests/lib/isolate.sh）ので、files の行は files/ に倒れた path の行になる
step "dg-send: 送る" "$andon" digest send "$id"
url=$(grep -m1 '^files/' "$tmp/step.out")
f=$(find "$ANDON_WORKSPACE/files" -name '*.html' 2>/dev/null | sed -n 1p)
echo "  | files/: ${f:-無い}"
jq -c . "$d/sent.json" 2>/dev/null | sed 's/^/  sent.json: /'
[ -n "$url" ] && [ -n "$f" ] && cmp -s "$f" "$d/digest.html" && jq -e --arg p "$url" '(.destinations | length) > 0 and all(.destinations[]; .kind == "files" and .ok and .path == $p)' "$d/sent.json" >/dev/null \
  && ok "dg-send: digest.html が files/ にそのまま出て置いたパスが返り、sent.json の送り先が files だけ" \
  || fail "dg-send: files/ に出ないか、パスか sent.json が無いか、files のほかの送り先がある"

# dg-check-refuse: 送り先が無ければ、確かめのファイルもどこへも送らない
step "dg-check-refuse: 送り先の無いまま確かめを打つ" "$andon" digest check \
  && fail "dg-check-refuse: 送り先が無いのに check が通った" \
  || { grep -qF '送り先の設定が無いので何も送らない' "$tmp/step.out" \
    && ok "dg-check-refuse: 送り先が無ければ何も送らずに断る" || fail "dg-check-refuse: 断りの理由が違う"; }

# dg-tray: 切り離した置き場で当てたパッケージのローカルの tray を起動し、送りを入れて（ANDON_TRAY_OFF=0）打ち直す。
# files/ に倒れた files の行は tray の URL に置き直され、andon publish は tray の URL を返し、andon tray status が枚数を出す。
# 一覧の面は 0 番（空いている番号）で起動する（セルの本物の tray が既定の番号を持っている）。データは切り離した HOME の下
pkg=${VERIFY_PACKAGE:-/opt/andon}
if [ ! -f "$pkg/tray/scripts/serve.mjs" ]; then
  echo "note dg-tray: $pkg/tray が無い（tray を入れていないパッケージ）。tray へ送る道は確かめていない"
else
  printf 'TRAY_PORT=0\nPORTLESS_PORT=1355\n' >>"$ANDON_WORKSPACE/.env"
  ctl=$HOME/.local/state/andon/tray/serve.json
  ANDON_TRAY_OFF=0 "$andon" tray serve >"$tmp/serve.log" 2>&1 &
  spid=$!
  for _ in $(seq 120); do [ -s "$ctl" ] && break; kill -0 "$spid" 2>/dev/null || break; sleep 0.5; done
  echo "  | 控え: $(cat "$ctl" 2>/dev/null || echo 無い)"
  if [ ! -s "$ctl" ]; then
    sed 's/^/  | serve: /' "$tmp/serve.log"
    fail "dg-tray: 切り離したローカルの tray が起動しない"
  else
    step "dg-tray: tray が動いたあとに送り直す" env ANDON_TRAY_OFF=0 "$andon" digest send "$id"
    turl=$(grep -m1 '^http' "$tmp/step.out")
    jq -c . "$d/sent.json" 2>/dev/null | sed 's/^/  sent.json: /'
    echo "dg-tray の確かめ" >"$tmp/tray-check.md"
    step "dg-tray: publish" env ANDON_TRAY_OFF=0 "$andon" publish "$tmp/tray-check.md" tray-check
    purl=$(grep -m1 '^http' "$tmp/step.out")
    step "dg-tray: status" env ANDON_TRAY_OFF=0 "$andon" tray status
    if [ -n "$turl" ] && jq -e --arg u "$turl" '.destinations[0] | .kind == "files" and .ok and .url == $u and (has("path") | not)' "$d/sent.json" >/dev/null \
      && [[ "$purl" == http*"/r/"* ]] && ! [ -e "$ANDON_WORKSPACE/files/tray-check.md" ] && grep -qF '上がった 2 枚' "$tmp/step.out"; then
      ok "dg-tray: files/ に倒れた files の行が tray の URL に置き直り、publish が tray の URL を返して files/ に写さず、status が 2 枚と数える"
    else
      fail "dg-tray: files の行が置き直らないか、publish が tray の URL を返さないか files/ に写したか、status の枚数が違う"
    fi
  fi
  kill -TERM "$spid" 2>/dev/null
  wait "$spid" 2>/dev/null
  [ ! -e "$ctl" ] || fail "dg-tray: 止めた serve の控えが残った"
fi

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
