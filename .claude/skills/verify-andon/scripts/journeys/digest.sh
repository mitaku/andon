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
  echo "note dg-tray-old: $pkg/tray が無い（tray を入れていないパッケージ）。出し直しの掃除は確かめていない"
  echo "note dg-tray-remote: $pkg/tray が無い（tray を入れていないパッケージ）。project.json の tray の読み方は確かめていない"
else
  printf 'TRAY_PORT=0\nPORTLESS_PORT=1355\n' >>"$ANDON_WORKSPACE/.env"
  ctl=$HOME/.local/state/andon/tray/serve.json
  # serve は検証するパッケージ（$pkg）の tray を動かす（既定の /opt/andon なら ANDON_HOME を付けないのと同じ）
  ANDON_TRAY_OFF=0 ANDON_HOME="$pkg" "$andon" tray serve >"$tmp/serve.log" 2>&1 &
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
    traystatus=$(cat "$tmp/step.out")
    if [ -n "$turl" ] && jq -e --arg u "$turl" '.destinations[0] | .kind == "files" and .ok and .url == $u and (has("path") | not)' "$d/sent.json" >/dev/null \
      && [[ "$purl" == http*"/r/"* ]] && ! [ -e "$ANDON_WORKSPACE/files/tray-check.md" ] && grep -qF '上がった 2 枚' "$tmp/step.out"; then
      ok "dg-tray: files/ に倒れた files の行が tray の URL に置き直り、publish が tray の URL を返して files/ に写さず、status が 2 枚と数える"
    else
      fail "dg-tray: files の行が置き直らないか、publish が tray の URL を返さないか files/ に写したか、status の枚数が違う"
    fi
    # dg-tray-old: 同じ slug で中身を変えて出し直すと、報告は新しいバージョンになり、アップロードの口が前の中身の掃除を通る。
    # 前の中身は置いてから1日に満たないので残り、前に開いた署名付き URL（5分）で読める。今の中身も開ける。
    # 続けて、切り離したローカルの tray の R2（miniflare の sqlite）で、前の中身の置いた時刻を 25 時間前と 23 時間前に、
    # ほかの報告の中身を 25 時間前に書き換えてから出し直し、同じ報告の 25 時間前の中身だけが消えるのを見る
    lst=$(jq -r '.list // empty' "$ctl" 2>/dev/null) lo=$(jq -r '.list_origin // empty' "$ctl" 2>/dev/null)
    rid=${purl##*/r/}
    # open_report <id>: 一覧の面の既読の口を、ローカルの tray の一覧の面の番号へ Host を付けて打ち、署名付き URL を出す
    open_report() { curl -s -X POST -H "Host: ${lo#http://}" -H "Origin: $lo" "$lst/api/reports/$1/open" | jq -r '.url // empty'; }
    # fetch_view <署名付き URL>: 報告の面の URL を、同じ番号へ Host を付けて読む
    fetch_view() { local hp=${1#http://}; hp=${hp%%/*}; curl -s -H "Host: $hp" "$lst${1#http://"$hp"}"; }
    u1=$(open_report "$rid")
    echo "dg-tray-old の確かめ 2" >"$tmp/tray-check.md"
    step "dg-tray-old: 同じ slug で中身を変えて publish" env ANDON_TRAY_OFF=0 "$andon" publish "$tmp/tray-check.md" tray-check
    u2=$(open_report "$rid")
    # Markdown は報告の面が HTML にして出すので、本文の段落（<p>…</p>）を取り出して比べる
    b1=$(fetch_view "$u1" | sed -n 's/.*<p>\([^<]*\)<\/p>.*/\1/p') b2=$(fetch_view "$u2" | sed -n 's/.*<p>\([^<]*\)<\/p>.*/\1/p')
    echo "  | 前の URL の中身: ${b1:-読めない}・今の URL の中身: ${b2:-読めない}"
    if [ -n "$u1" ] && [ -n "$u2" ] && [[ "$u1" != "$u2" ]] && [[ "$b1" == "dg-tray の確かめ" ]] && [[ "$b2" == "dg-tray-old の確かめ 2" ]]; then
      ok "dg-tray-old: 出し直すと今の中身が開け、置いてから1日に満たない前の中身は前の URL で読める"
    else
      fail "dg-tray-old: 出し直したあと、今の中身か前の中身が読めない"
    fi
    # r2 keys <prefix> は置いた順にキーを出し、r2 age <key> <時間> はそのキーの置いた時刻を今からその時間前にする
    r2db=$(find "$HOME/.local/state/andon/tray/data/r2/miniflare-R2BucketObject" -name '*.sqlite' ! -name metadata.sqlite 2>/dev/null | sed -n 1p)
    r2() {
      node -e '
        const { DatabaseSync } = require("node:sqlite");
        const [db, op, a, h] = process.argv.slice(1);
        const d = new DatabaseSync(db);
        if (op === "keys") for (const r of d.prepare("SELECT key FROM _mf_objects WHERE key LIKE ? ORDER BY uploaded, key").all(a + "%")) console.log(r.key);
        else if (op === "age") d.prepare("UPDATE _mf_objects SET uploaded = ? WHERE key = ?").run(Date.now() - Number(h) * 3600e3, a);
      ' "$r2db" "$@"
    }
    echo "dg-tray-old の確かめ 3" >"$tmp/tray-check.md"
    step "dg-tray-old: 3 つ目の中身を publish" env ANDON_TRAY_OFF=0 "$andon" publish "$tmp/tray-check.md" tray-check
    mapfile -t k < <(r2 keys "$rid/")
    oid=${turl##*/r/}
    okey=$(r2 keys "$oid/" | sed -n 1p)
    echo "  | R2: ${r2db:-無い}。この報告の中身 ${#k[@]} 個（${k[*]}）。ほかの報告の中身 ${okey:-無い}"
    if [ "${#k[@]}" = 3 ] && [ -n "$okey" ]; then
      r2 age "${k[0]}" 25 && r2 age "${k[1]}" 23 && r2 age "$okey" 25 || fail "dg-tray-old: R2 の置いた時刻を書き換えられない"
      echo "dg-tray-old の確かめ 4" >"$tmp/tray-check.md"
      step "dg-tray-old: 置いた時刻を戻したあとに publish" env ANDON_TRAY_OFF=0 "$andon" publish "$tmp/tray-check.md" tray-check
      after=$(r2 keys "$rid/")
      echo "  | 出し直したあとのこの報告の中身: $(tr '\n' ' ' <<<"$after")。ほかの報告の中身: $(r2 keys "$oid/" | tr '\n' ' ')"
      if ! grep -qxF "${k[0]}" <<<"$after" && grep -qxF "${k[1]}" <<<"$after" && grep -qxF "${k[2]}" <<<"$after" \
        && [ "$(grep -c . <<<"$after")" = 3 ] && grep -qxF "$okey" < <(r2 keys "$oid/"); then
        ok "dg-tray-old: 出し直すと、同じ報告の置いてから 25 時間の中身だけが消え、23 時間の中身・前の中身・今の中身・ほかの報告の中身は残る"
      else
        fail "dg-tray-old: 消えた中身が違う（25 時間の中身が残ったか、残すべき中身が消えた）"
      fi
    else
      fail "dg-tray-old: 出し直した3つの中身か、ほかの報告の中身が R2 に無い"
    fi
    # dg-tray-remote: owner の commit で project.json に tray を書くと、そのプロジェクトは Cloudflare の tray へ送ると読む。
    # 鍵を置かないので Cloudflare へは何も出ず、publish はローカルの tray に倒れて理由を出す。owner でない commit の tray は読まない
    slug=$(sed -n 's/^Cloudflare の tray: \(.*\) は選んでいない.*/\1/p' <<<"$traystatus")
    echo "  | プロジェクト: ${slug:-引けない}"
    printf 'owner = "%s"\n' "$GIT_AUTHOR_EMAIL" >"$ANDON_WORKSPACE/andon.toml"
    mkdir -p "$ANDON_WORKSPACE/projects/${slug:-none}"
    printf '{"version": 1, "project": {"title": "verify", "prefix": "V"}, "repos": [], "tray": {"upload": "https://tray-up.invalid"}}\n' >"$ANDON_WORKSPACE/projects/${slug:-none}/project.json"
    git -C "$ANDON_WORKSPACE" add -A && git -C "$ANDON_WORKSPACE" commit -q -m "tray を選ぶ"
    step "dg-tray-remote: status" env ANDON_TRAY_OFF=0 "$andon" tray status
    chosen=$(cat "$tmp/step.out")
    echo "dg-tray-remote の確かめ" >"$tmp/tray-remote.md"
    step "dg-tray-remote: publish" env ANDON_TRAY_OFF=0 "$andon" publish "$tmp/tray-remote.md" tray-remote
    rurl=$(grep -m1 '^http' "$tmp/step.out")
    published=$(cat "$tmp/step.out")
    printf ' \n' >>"$ANDON_WORKSPACE/projects/${slug:-none}/project.json"
    git -C "$ANDON_WORKSPACE" add -A && GIT_AUTHOR_EMAIL=someone@example.com git -C "$ANDON_WORKSPACE" commit -q -m "owner でない"
    step "dg-tray-remote: owner でない commit の status" env ANDON_TRAY_OFF=0 "$andon" tray status
    if [ -n "$slug" ] && grep -qF "Cloudflare の tray: $slug は https://tray-up.invalid へ送る" <<<"$chosen" && grep -qF 'API キー: 無い' <<<"$chosen" \
      && [[ "$rurl" == http*"/r/"* ]] && grep -qF 'API キーが無い' <<<"$published" \
      && grep -qF "Cloudflare の tray: $slug の設定を読めない" "$tmp/step.out" && grep -qF 'owner でない' "$tmp/step.out"; then
      ok "dg-tray-remote: owner の commit の tray は Cloudflare へ送ると読み、鍵が無いと publish はローカルの tray に倒れ、owner でない commit の tray は読まない"
    else
      fail "dg-tray-remote: project.json の tray の読み方が違う（status・publish の出力を見る）"
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
