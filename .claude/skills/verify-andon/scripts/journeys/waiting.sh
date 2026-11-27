#!/usr/bin/env bash
# オーナー待ち（features/waiting.md）。切り離した kanban で、Chief of Staff がオーナーの課題を足し（andon issue ask --new --now）、
# オーナーと同じく andon waiting list で判断の行として読む。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

ledger_init V
p1=$(HC_TERMINAL_ID=tc "$kanban" add "$run の親の課題 1" --profile claude)
p2=$(HC_TERMINAL_ID=tc "$kanban" add "$run の親の課題 2" --profile claude)
export HC_TERMINAL_ID=tc
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
q="{\"kind\": \"users\", \"question\": \"$run の書き出しは CSV だけにするか\", \"options\": [{\"label\": \"CSV だけ\", \"pick\": true}, {\"label\": \"Excel も\"}], \"from\": \"chief-of-staff\"}"
owner_of() { "$kanban" show "$1" --json | jq -c '.issue | {id, title, parent, who, status}'; }

# ow-ask-new: オーナーの課題が足され、who: person と問いを持つ
step "ow-ask-new: 親 $p1 の下にオーナーの課題を足す" "$andon" issue ask --new --now --parent "$p1" - <<<"$q"
a=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${a:-無い}: $( [ -n "$a" ] && owner_of "$a")"
[ -n "$a" ] && "$kanban" show "$a" --json | jq -e --arg p "$p1" '.issue.who == "person" and .issue.parent == $p and (.issue.title | endswith("（オーナーの判断待ち）"))' >/dev/null \
  && jq -e '.kind == "users" and .from == "chief-of-staff"' "$tmp/project/kanban/issues/$a/library/questions/1.json" >/dev/null \
  && ok "ow-ask-new: オーナーの課題 $a が who: person・親 $p1 で足され、問い 1 が library/questions/1.json に置かれる" \
  || fail "ow-ask-new: オーナーの課題か問いが無い・形が違う"

# ow-same-title: 同じ問いでも親が違えば別のオーナーの課題になる。同じ親で打ち直すと断り、課題を増やさない
step "ow-same-title: 同じ問いを親 $p2 の下に足す" "$andon" issue ask --new --now --parent "$p2" - <<<"$q"
b=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${b:-無い}: $( [ -n "$b" ] && owner_of "$b")"
[ -n "$b" ] && [ "$b" != "$a" ] && [ "$("$kanban" show "$b" --json | jq -r .issue.parent)" = "$p2" ] \
  && ok "ow-same-title: 親の違う同じタイトルは取り違えず、別のオーナーの課題 $b になる" \
  || fail "ow-same-title: 親の違う同じ問いが別の課題にならない"
n0=$("$kanban" list --json | jq length)
step "ow-same-title: 同じ問いを親 $p1 の下に打ち直す" "$andon" issue ask --new --now --parent "$p1" - <<<"$q" \
  && fail "ow-same-title: 問いを持つ同じオーナーの課題があるのに通った" \
  || { grep -qF "同じ問いのオーナーの課題 $a がもうある" "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n0" ] \
    && ok "ow-same-title: 同じ親の同じ問いは、同じ問いの課題 $a があると言って断り、課題を増やさない" \
    || fail "ow-same-title: 断った理由が同じ問いの課題でないか、断ったのに課題が増えた"; }

# ow-waiting: オーナー待ちに、2つの課題が判断の行として出る（答えられる問い）
step "ow-waiting: andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
jq -c '{slots, items: [.items[] | {kind, id, question, answerable}]}' "$tmp/step.out" 2>/dev/null | sed 's/^/  waiting: /'
jq -e --arg a "$a" --arg b "$b" '[.items[] | select(.kind == "decision" and .answerable == true) | .id] | sort == ([$a, $b] | sort)' "$tmp/step.out" >/dev/null \
  && ok "ow-waiting: オーナー待ちに $a と $b が答えられる判断の行として出る" \
  || fail "ow-waiting: 判断の行が足した2つの課題と違う"
step "ow-waiting: andon waiting list を打つ（人が読む出力）" "$andon" waiting list \
  && grep -qF "$a" "$tmp/step.out" && ok "ow-waiting: 人が読む出力にも $a が出る" || fail "ow-waiting: 人が読む出力に $a が無い"

# ow-old-name: 名前を変える前の andon waiting --json（古いホストの部品が打つ形）は、新しい名前を1行で言って終了コード 2 で終わり、何も出さない
rc=0; step "ow-old-name: 古い名前 andon waiting --json を打つ" "$andon" waiting --json || rc=$?
[ "$rc" = 2 ] && [ "$(wc -l <"$tmp/step.out")" = 1 ] && grep -qF 'andon waiting list に名前が変わった' "$tmp/step.out" \
  && ok "ow-old-name: andon waiting --json は andon waiting list を1行で言って終了コード 2 で終わり、オーナー待ちを出さない" \
  || fail "ow-old-name: 古い名前が終了コード 2 と新しい名前の1行で断られない（終了コード $rc）"
# 2語の古い名前・消えた名詞だけ・子の無い1語の古い名前・名詞が生きている名詞だけの古い名前のフラグだけの形・
# 1語目が1語の葉のまま残る2語の古い名前も、同じく断る（振り分けの分かれ道ごとに1つ。機械が打つ葉は ADR-0053 切る案 5）。
# 断れずに実行しても本物を書かないよう、置き場は一時の空の所に向ける
mkdir -p "$tmp/old-name/ws" "$tmp/old-name/pkg"
for pair in 'delegate show --json|andon delegation show' 'delegate|andon delegation' 'measure --json|andon metrics show' \
  'dispatcher --once|andon dispatcher run' 'signal delivered fp 1|andon signal mark-delivered' 'version stamp unknown|andon package stamp'; do
  old=${pair%%|*} neu=${pair#*|}
  rc=0; step "ow-old-name: 古い名前 andon $old を打つ" env ANDON_WORKSPACE="$tmp/old-name/ws" ANDON_HOME="$tmp/old-name/pkg" "$andon" $old || rc=$?
  [ "$rc" = 2 ] && [ "$(wc -l <"$tmp/step.out")" = 1 ] && grep -qF "$neu に名前が変わった" "$tmp/step.out" \
    && ok "ow-old-name: andon $old は $neu を1行で言って終了コード 2 で終わる" \
    || fail "ow-old-name: andon $old が終了コード 2 と $neu の1行で断られない（終了コード $rc）"
done

# ow-json: JSON を出せる葉は --json を受け、標準出力が JSON として読める（ADR-0053 決めたこと 13）。いつも JSON を出す葉（issue show・version・
# gate）は葉が --json を読まず、振り分けが外して渡す。--help は JSON の出し方の行を持ち、JSON を出せない葉は持たない
step "ow-json: andon issue show $p1 --json を打つ" "$andon" issue show "$p1" --json \
  && jq -e --arg p "$p1" '.id == $p' "$tmp/step.out" >/dev/null \
  && step "ow-json: andon version --json を打つ" "$andon" version --json && jq -e '.root' "$tmp/step.out" >/dev/null \
  && ok "ow-json: いつも JSON を出す issue show と version は --json を受けて JSON を出す" \
  || fail "ow-json: issue show か version が --json を受けないか、JSON を出さない"
rc=0; step "ow-json: andon gate $p1 --dry-run --json を打つ（report.json が無いので fix）" "$andon" gate "$p1" --dry-run --json || rc=$?
[ "$rc" = 1 ] && jq -e '.decision == "fix"' "$tmp/step.out" >/dev/null \
  && ok "ow-json: gate は --json を受け、判定を JSON で出す（fix は終了コード 1）" \
  || fail "ow-json: gate が --json を受けないか、判定を JSON で出さない（終了コード $rc）"
step "ow-json: andon issue show --help を打つ" "$andon" issue show --help >/dev/null \
  && grep -qx 'JSON: --json を付けても付けなくても、JSON を標準出力に出す' "$tmp/step.out" \
  && step "ow-json: andon issue finish --help を打つ" "$andon" issue finish --help >/dev/null && ! grep -q '^JSON: ' "$tmp/step.out" \
  && ok "ow-json: JSON を出せる葉の --help は JSON の行を持ち、JSON を出せない葉は持たない" \
  || fail "ow-json: --help の JSON の行が違う"

# ow-tray-deploy: Cloudflare へ出す葉 andon tray deploy は、一覧の葉に載り、--help がセルの外で打つことを言う。
# トークンの無い本番の打ち方は、設定のファイルも node も読む前に終了コード 2 で断る（Cloudflare に触れない）
step "ow-tray-deploy: andon help --all を打つ" "$andon" help --all >/dev/null \
  && grep -q '^ *tray deploy ' "$tmp/step.out" \
  && step "ow-tray-deploy: andon tray deploy --help を打つ" "$andon" tray deploy --help >/dev/null \
  && grep -qF 'bin/bootstrap tray-deploy' "$tmp/step.out" \
  && ok "ow-tray-deploy: andon tray deploy は一覧の葉に載り、--help がセルの外の bin/bootstrap tray-deploy から打つと言う" \
  || fail "ow-tray-deploy: andon tray deploy が一覧に無いか、--help が打つ場所を言わない"
rc=0; step "ow-tray-deploy: トークンなしで andon tray deploy --config を打つ" env -u CLOUDFLARE_API_TOKEN "$andon" tray deploy --config "$tmp/none.json" || rc=$?
if [ "$rc" = 2 ] && grep -qF 'CLOUDFLARE_API_TOKEN が無い' "$tmp/step.out"; then
  ok "ow-tray-deploy: トークンの無い本番の打ち方は終了コード 2 で断る"
else
  fail "ow-tray-deploy: トークンの無い本番の打ち方を断らない（終了コード $rc）"
fi

# ow-later: Chief of Staff が印 later を書いたオーナーの課題は、呼ばずに置いてある行になる。日（later_until）が来るか、
# 印を担当の端末が書き直すと、また呼ぶ
row_of() { jq -c --arg id "$1" '.items[] | select(.id == $id) | {id, call, later, later_until}' "$tmp/step.out" 2>/dev/null; }
why="$run 急がない。次のリリースで決めればよい"
step "ow-later: Chief of Staff（tc）が $a に印 later を書く" "$kanban" set "$a" later="$why"
step "ow-later: 印のあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a") $(row_of "$b")"
jq -e --arg a "$a" --arg b "$b" --arg w "$why" '[.items[] | select(.id == $a or .id == $b) | [.id, .call, (.later // "")]] == [[$b, true, ""], [$a, false, $w]]' "$tmp/step.out" >/dev/null \
  && ok "ow-later: $a は call: false で印の理由を持ち、呼んでいる $b の後に並ぶ" \
  || fail "ow-later: 印を書いた $a が置いてある行にならない・並びが違う"
step "ow-later: 印のあとに andon waiting list を打つ（人が読む出力）" "$andon" waiting list
grep -q '^置いてあるもの 1 件' "$tmp/step.out" && grep -qF "置いた理由: $why" "$tmp/step.out" \
  && ok "ow-later: 人が読む出力に「置いてあるもの 1 件」と置いた理由が出る" \
  || fail "ow-later: 人が読む出力に置いてあるものの見出しか理由が無い"
step "ow-later: Chief of Staff が $a に先の日 later_until=2999-12-31 を書く" "$kanban" set "$a" later_until=2999-12-31
step "ow-later: 先の日のあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == false and .later_until == "2999-12-31"' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 日が先なら $a は置いたままで、呼ぶ日 2999-12-31 が出る" \
  || fail "ow-later: 日が先なのに $a を呼んだか、呼ぶ日が出ない"
step "ow-later: Chief of Staff が $a に来た日 later_until=2000-01-01 を書く" "$kanban" set "$a" later_until=2000-01-01
step "ow-later: 日が来たあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == true' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 日が来た $a はまた呼ぶ（call: true）" \
  || fail "ow-later: 日が来たのに $a を呼ばない"
step "ow-later: Chief of Staff が later_until を消す" "$kanban" set "$a" later_until=
step "ow-later: 担当の端末 tw が $a の印 later を書き直す" env HC_TERMINAL_ID=tw "$kanban" set "$a" later="$why"
step "ow-later: 担当が書いたあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == true and (has("later") | not)' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 担当の端末が書いた印は信じず、$a をまた呼ぶ" \
  || fail "ow-later: 担当の端末が書いた印で $a を置いたままにした"

# ow-ask-later: 作る口は呼ぶか置いておくかを言い切らせる。旗が無ければ理由を言って断り、Chief of Staff の --later は印を書いて
# 置いてある行にし、担当の端末の --later は断る。どの断りでも課題は増えない
q2="{\"kind\": \"users\", \"question\": \"$run の一覧の既定の並びを変えるか\", \"options\": [{\"label\": \"変えない\", \"pick\": true}, {\"label\": \"新しい順\"}], \"from\": \"chief-of-staff\"}"
why2="$run 急がない。答えを待つ仕事が無い"
n1=$("$kanban" list --json | jq length)
step "ow-ask-later: 旗なしで ask --new を打つ" "$andon" issue ask --new --parent "$p2" - <<<"$q2" \
  && fail "ow-ask-later: 旗なしの ask --new が通った" \
  || { grep -qF '呼ぶか置いておくかを言い切る' "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n1" ] \
    && ok "ow-ask-later: 旗なしの ask --new は、呼ぶか置いておくかを言い切れと言って断り、課題を増やさない" \
    || fail "ow-ask-later: 旗なしの断りの理由が違うか、課題が増えた"; }
step "ow-ask-later: 担当の端末 tw が --later を打つ" env HC_TERMINAL_ID=tw "$andon" issue ask --new --later "$why2" --parent "$p2" - <<<"$q2" \
  && fail "ow-ask-later: 担当の端末の --later が通った" \
  || { grep -qF -- '--later で印を書くのは Chief of Staff の端末だけ' "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n1" ] \
    && ok "ow-ask-later: 担当の端末の --later は、Chief of Staff の端末だけと言って断り、課題を増やさない" \
    || fail "ow-ask-later: 担当の端末の --later の断りの理由が違うか、課題が増えた"; }
step "ow-ask-later: Chief of Staff（tc）が --later --until で足す" "$andon" issue ask --new --later "$why2" --until 2999-12-31 --parent "$p2" - <<<"$q2"
c=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
step "ow-ask-later: 足したあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $( [ -n "$c" ] && row_of "$c")"
[ -n "$c" ] && jq -e --arg c "$c" --arg w "$why2" '.items[] | select(.id == $c) | .call == false and .later == $w and .later_until == "2999-12-31"' "$tmp/step.out" >/dev/null \
  && ok "ow-ask-later: --later --until で足した $c は、印の理由と呼ぶ日を持つ置いてある行（call: false）になる" \
  || fail "ow-ask-later: --later --until で足した課題が置いてある行にならない"

# ow-slots: オーナー待ちの担当の課題が上限（既定の 2）以上でも、動いている課題が上限より少なければ空きは正になる
for i in 1 2; do
  w=$("$kanban" add "$run の判断を待つ担当の課題 $i" --profile claude)
  step "ow-slots: 担当の端末 tw$i が $w を doing にする" env HC_TERMINAL_ID="tw$i" "$kanban" start "$w"
  step "ow-slots: $w が判断の問いを出す" env HC_TERMINAL_ID="tw$i" "$kanban" ask "$w" "$run の担当 $i の問い" --kind users
done
step "ow-slots: andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  slots: $(jq -c .slots "$tmp/step.out" 2>/dev/null)"
jq -e '.slots.waiting >= .slots.limit and .slots.working < .slots.limit and .slots.free == .slots.limit - .slots.working and .slots.free > 0 and (.slots | has("waiting_full") | not)' "$tmp/step.out" >/dev/null \
  && ok "ow-slots: オーナー待ちが上限以上でも、空き（slots.free）は上限から動いている数を引いた正の数で、waiting_full は無い" \
  || fail "ow-slots: オーナー待ちが上限以上で空きが 0 か、waiting_full がある"
step "ow-slots: andon waiting list を打つ（人が読む出力）" "$andon" waiting list
if grep -q '起動しない' "$tmp/step.out"; then fail "ow-slots: 人が読む出力に、オーナー待ちで新しい課題を起動しないの文がある"
elif grep -qE 'の枠: 上限 2、動いている 0、オーナー待ち 2、空き 2$' "$tmp/step.out"; then ok "ow-slots: 人が読む枠の行は空き 2 で、新しい課題を起動しないの文が無い"
else fail "ow-slots: 人が読む枠の行が違う"; fi

# ow-stackable: 前の課題が今の回の gate（approve）を通って review にいて、after の済んでいない相手がその1つだけの queued の課題は
# stackable に出る。待つ相手が2つ残る queued の課題は出ない（ADR-0067 決めたこと 4・6）
sh=1111111111111111111111111111111111111111
# 積めるのは repo のある課題だけなので、切り離した kanban に repo を1つ登録する（kanban の CLI に登録の口は無い）
sed -i 's|^repos: \[\]$|repos:\n  - github.com/example/app|' "$tmp/project/kanban/config.yaml"
grep -q '^  - github.com/example/app$' "$tmp/project/kanban/config.yaml" || fail "ow-stackable: 切り離した kanban に repo を登録できない"
sp=$("$kanban" add "$run の積む先の課題" --profile claude --repo github.com/example/app)
sw=$("$kanban" add "$run の動いている課題" --profile claude --repo github.com/example/app)
step "ow-stackable: 担当の端末 tw3 が $sp を doing にする" env HC_TERMINAL_ID=tw3 "$kanban" start "$sp"
step "ow-stackable: 担当の端末 tw3 が $sp を review にする" env HC_TERMINAL_ID=tw3 "$kanban" review "$sp"
step "ow-stackable: 担当の端末 tw4 が $sw を doing にする" env HC_TERMINAL_ID=tw4 "$kanban" start "$sw"
mkdir -p "$tmp/project/kanban/issues/$sp/library/gate"
jq -n --arg c "$sh" '{commit: $c, pr: "https://github.com/example/app/pull/1"}' >"$tmp/project/kanban/issues/$sp/library/report.json"
jq -n --arg h "$sh" --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{head: $h, at: $at, decision: "approve", reasons: []}' >"$tmp/project/kanban/issues/$sp/library/gate/$sh.json"
sn=$("$kanban" add "$run の積める課題" --profile claude --repo github.com/example/app --after "$sp")
s2=$("$kanban" add "$run の待つ相手が2つの課題" --profile claude --repo github.com/example/app --after "$sp,$sw")
step "ow-stackable: andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  stackable: $(jq -c .stackable "$tmp/step.out" 2>/dev/null)"
jq -e --arg n "$sn" --arg p "$sp" --arg s2 "$s2" '(.stackable | index({id: $n, on: $p})) != null and ([.stackable[] | select(.id == $s2)] | length) == 0' "$tmp/step.out" >/dev/null \
  && ok "ow-stackable: $sn は $sp の上に積める課題として stackable に出て、待つ相手が2つの $s2 は出ない" \
  || fail "ow-stackable: stackable に $sn が無いか、待つ相手が2つの $s2 が出た"
step "ow-stackable: andon waiting list を打つ（人が読む出力）" "$andon" waiting list
grep -qF "$sn（$sp の上）" "$tmp/step.out" && grep -q '^積める課題 [0-9]* 件: ' "$tmp/step.out" \
  && ok "ow-stackable: 人が読む出力に「積める課題 N 件」の行が出て、$sn（$sp の上）を持つ" \
  || fail "ow-stackable: 人が読む出力に積める課題の行が無い"

# ow-anywhere: どこで打っても読む。一時のセルにプロジェクト2つ（x・y）と1つ（solo）を作り、KANBAN_ROOT を外して打つ
c2=$tmp/cell2 c1=$tmp/cell1
for p in "$c2/projects/x" "$c2/projects/y" "$c1/projects/solo"; do
  mkdir -p "$p" && "$kanban" init --root "$p" --prefix "$(basename "$p" | tr a-z A-Z)" >/dev/null \
    && KANBAN_ROOT=$p "$kanban" add "$run の $(basename "$p") の課題" >/dev/null || fail "ow-anywhere: 一時のセルの $p を作れない"
done
mkdir -p "$c1/notes"
at() { local d=$1 ws=$2; shift 2; (cd "$d" && env -u KANBAN_ROOT ANDON_WORKSPACE="$ws" "$@"); }
step "ow-anywhere: 2つのセルの根で andon waiting list --project x を打つ" at "$c2" "$c2" "$andon" waiting list --project x \
  && grep -q '^x の枠: ' "$tmp/step.out" && ok "ow-anywhere: セルの根で --project x を付けると x のオーナー待ちを出す" \
  || fail "ow-anywhere: セルの根の --project x で x の枠の行が出ない"
for d in "$c1" "$c1/notes"; do
  step "ow-anywhere: 1つのセルの ${d#"$tmp"/} で andon waiting list を打つ" at "$d" "$c1" "$andon" waiting list \
    && grep -q '^solo の枠: ' "$tmp/step.out" && ok "ow-anywhere: プロジェクトが1つのセルの ${d#"$tmp"/} では、省いても solo のオーナー待ちを出す" \
    || fail "ow-anywhere: 1つのセルの ${d#"$tmp"/} で省くと solo の枠の行が出ない"
done
step "ow-anywhere: 1つのセルの根で kanban list を打つ" at "$c1" "$c1" "$kanban" list \
  && grep -qF "$run の solo の課題" "$tmp/step.out" && ok "ow-anywhere: プロジェクトが1つのセルの根で、kanban list も solo の kanban を読む" \
  || fail "ow-anywhere: 1つのセルの根で kanban list が solo の課題を出さない"
rc=0; step "ow-anywhere: 2つのセルの根で --project を省いて andon waiting list を打つ" at "$c2" "$c2" "$andon" waiting list || rc=$?
[ "$rc" = 2 ] && grep -qF 'セルのプロジェクト: x・y' "$tmp/step.out" && grep -qF 'andon waiting list --project <slug>' "$tmp/step.out" \
  && ok "ow-anywhere: 2つのセルの根で省くと、終了コード 2 で x と y と --project を付ける1行を出す" \
  || fail "ow-anywhere: 2つのセルの根で省いたときに終了コード 2 と一覧が出ない（終了コード $rc）"
rc=0; step "ow-anywhere: 2つのセルの根で andon waiting list --project nosuch を打つ" at "$c2" "$c2" "$andon" waiting list --project nosuch || rc=$?
[ "$rc" = 2 ] && grep -qF 'プロジェクト nosuch は無い' "$tmp/step.out" \
  && ok "ow-anywhere: 無いプロジェクトの --project は終了コード 2 で断る" \
  || fail "ow-anywhere: 無いプロジェクトの --project が終了コード 2 で断られない（終了コード $rc）"
# --project を自分で読む葉（委譲）も同じに決まる。kanban の無い projects/<slug>/ の下でも、今いるフォルダで決まる
mkdir -p "$c2/projects/z/notes"
step "ow-anywhere: 2つのセルの根で andon delegation show --project x を打つ" at "$c2" "$c2" "$andon" delegation show --project x \
  && grep -q '^x: ' "$tmp/step.out" \
  && step "ow-anywhere: 1つのセルの notes で andon delegation show を打つ" at "$c1/notes" "$c1" "$andon" delegation show \
  && grep -q '^solo: ' "$tmp/step.out" \
  && step "ow-anywhere: kanban の無い projects/z/notes で andon delegation show を打つ" at "$c2/projects/z/notes" "$c2" "$andon" delegation show \
  && grep -q '^z: ' "$tmp/step.out" \
  && ok "ow-anywhere: delegation show も、--project・セルに1つだけ・kanban の無い projects/<slug>/ の下でプロジェクトが決まる" \
  || fail "ow-anywhere: delegation show のプロジェクトの決まり方が違う"
# --project は動詞のあとならどこでも受ける。後ろに置いた signal mark-delivered は usage にならず、記録に無い信号として断る（記録は一時の HOME）
rc=0; step "ow-anywhere: andon signal mark-delivered fp 3 --project x を打つ" at "$c2" "$c2" env HC_TERMINAL_ID=dispatcher "$andon" signal mark-delivered fp 3 --project x || rc=$?
[ "$rc" = 1 ] && grep -qF '記録に無い信号: fp' "$tmp/step.out" \
  && ok "ow-anywhere: 後ろに置いた --project を signal mark-delivered が受け、x の記録を見る" \
  || fail "ow-anywhere: 後ろに置いた --project を signal mark-delivered が受けない（終了コード $rc）"
# 使い方の文はどこで打つかを言わず、振り分けが足す --project の行を持つ
bad=
for l in "retro collect" "signal collect" "design start"; do
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  step "ow-anywhere: andon $l --help を打つ" "$andon" $l --help >/dev/null
  grep -q '^プロジェクト: --project <slug>' "$tmp/step.out" && ! grep -q 'projects/<slug>/ .*で打つ' "$tmp/step.out" || bad+=" $l"
done
[ -z "$bad" ] && ok "ow-anywhere: プロジェクトを読む葉の --help は --project の行を持ち、projects/<slug>/ で打つと言わない" \
  || fail "ow-anywhere: --help に --project の行が無いか、projects/<slug>/ で打つと言う:$bad"
step "ow-anywhere: kanban --help を打つ" "$kanban" --help
grep -qF 'only projects/<slug>/ with a kanban' "$tmp/step.out" \
  && ok "ow-anywhere: kanban --help は、上の projects/ に kanban が1つだけならそれ、の段を言う" \
  || fail "ow-anywhere: kanban --help に根の探し方の最後の段が無い"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
