#!/usr/bin/env bash
# プロジェクトを立てる（features/create.md）。切り離したセルの repo で、オーナーと同じく andon project create を端末でなく打つ。
# repo の clone は先に一時の GHQ_ROOT に置き、ghq get（ネットワーク）を打たせない。
# dispatcher の置き場合わせは偽の hc に届き、本物の dispatcher には触れない（偽の hc get dispatcher は Gone を返す）。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

export ANDON_WORKSPACE=$tmp/ws GHQ_ROOT=$tmp/ghq
mkdir -p "$ANDON_WORKSPACE/projects"
repo=github.com/example/sample-site
git init -q "$GHQ_ROOT/$repo" && git -C "$GHQ_ROOT/$repo" commit -q --allow-empty -m init
slug=verify-$$
p=$ANDON_WORKSPACE/projects/$slug
echo '{"state":"exited","labels":{}}' >"$tmp/hc/dispatcher.json"

# cr-role-refuse: 役の端末（ラベル role=）では打てず、何も作らない
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
step "cr-role-refuse: Chief of Staff の端末で打つ" env HC_TERMINAL_ID=tc "$andon" project create "$slug" --repo "$repo" </dev/null \
  && fail "cr-role-refuse: 役の端末で create が通った" \
  || { [ ! -e "$p" ] && ok "cr-role-refuse: 役の端末では断り、projects/$slug を作らない" || fail "cr-role-refuse: 断ったのに projects/$slug が在る"; }

# cr-create: オーナーの端末（役のラベルの無い端末）で打つと、kanban・project.json の下書き・AGENTS.md ができる
echo '{"labels":{}}' >"$tmp/hc/to.json"
step "cr-create: オーナーの端末で打つ" env HC_TERMINAL_ID=to "$andon" project create "$slug" --repo "$repo" --prefix VC --title "$run のプロジェクト" </dev/null
rc=$?
echo "  | projects/$slug: $(cd "$p" 2>/dev/null && find . -maxdepth 2 | sort | tr '\n' ' ')"
jq -c . "$p/project.json" 2>/dev/null | sed 's/^/  project.json: /'
[ "$rc" = 0 ] && [ -f "$p/kanban/config.yaml" ] && [ -f "$p/AGENTS.md" ] \
  && jq -e --arg r "$repo" '.project.prefix == "VC" and .repos == [$r] and (.delegation == null) and any(.signals[]; .source == "github-ci")' "$p/project.json" >/dev/null \
  && ok "cr-create: kanban・委譲の無い project.json の下書き（prefix・repos・CI の信号の行）・AGENTS.md ができる" \
  || fail "cr-create: 終了コード $rc か、作ったものが足りないか形が違う"
# 作った kanban を別の口（kanban の CLI）で読み直す: 課題を足すと接頭字 VC の番号になる
id=$(cd "$p" && KANBAN_ROOT=$p HC_TERMINAL_ID=to "$kanban" add "$run の最初の課題" --repo "$repo" 2>&1)
echo "  | kanban add: $id"
[[ "$id" == VC-* ]] && ok "cr-create: 作った kanban に課題を足すと $id（接頭字 VC）になる" || fail "cr-create: 作った kanban に接頭字 VC の課題を足せない"
grep -q '^hc run --bg .*--name dispatcher .* dispatcher$' "$tmp/hc.calls" \
  && ok "cr-create: dispatcher が無いので、置き場を合わせて hc run で起動しようとした（偽の hc に届いた）" \
  || fail "cr-create: dispatcher の置き場合わせの hc run が無い"

# cr-list: andon project list が、立てたプロジェクトの slug を1行で出す（読むだけ）
step "cr-list: andon project list を打つ" "$andon" project list
[ "$(cat "$tmp/step.out")" = "$slug" ] && ok "cr-list: andon project list が立てた $slug だけを1行で出す" \
  || fail "cr-list: andon project list の出力が $slug の1行でない"

# cr-exists-refuse: 同じ slug を打ち直すと、何も変えずに断る
s0=$(find "$p" -type f -exec sha256sum {} + | sort)
step "cr-exists-refuse: 同じ slug を打ち直す" env HC_TERMINAL_ID=to "$andon" project create "$slug" </dev/null \
  && fail "cr-exists-refuse: 在る slug で create が通った" \
  || { grep -qF "projects/$slug が既にある" "$tmp/step.out" && [ "$s0" = "$(find "$p" -type f -exec sha256sum {} + | sort)" ] \
    && ok "cr-exists-refuse: 在る slug は断り、projects/$slug の中身を変えない" || fail "cr-exists-refuse: 理由が違うか、中身が変わった"; }

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
