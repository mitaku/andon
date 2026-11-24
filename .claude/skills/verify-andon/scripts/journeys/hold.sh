#!/usr/bin/env bash
# PR を止める（features/hold.md）。切り離した kanban で、担当が報告した PR を Chief of Staff が止め（andon issue hold）、
# 止めている間 gate が待つこと、担当が hold を消しても止まったままのこと、Chief of Staff が外す（andon issue unhold）ことを通す。
# 後半は本物の andon dispatcher --once を偽のセルの repo で回し、止めている間はマージしないこと（外すとマージする対照つき）と、
# approve のあとに止めた判定を捨てることを通す。
# 偽の gh は止める口が打つ2つの呼び出しだけを知るものに替える（sandbox.sh の偽の gh はこの2つを知らない）:
#   pr view <n> --repo <owner>/<repo> --json state,mergeCommit,headRefOid,statusCheckRollup は $tmp/gh/view、
#   api -X POST repos/<owner>/<repo>/statuses/<sha> … は記録して 0。ほかは記録して 99。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >>"$tmp/gh.calls"
case "\$1 \$2" in
  "pr view") [ "\$7" = state,mergeCommit,headRefOid,statusCheckRollup ] && exec cat "$tmp/gh/view" ;;
  "api -X") [ "\$3" = POST ] && exit 0 ;;
esac
echo "偽の gh: 知らない呼び出し \$*" >&2
exit 99
EOF
chmod +x "$tmp/bin/gh"

# 担当の worktree と PR。PR の head の印 andon/gate は緑（gate が今の commit を渡した）
git init -q "$tmp/wt" && echo one >"$tmp/wt/f" && git -C "$tmp/wt" add f && git -C "$tmp/wt" commit -qm one
head=$(git -C "$tmp/wt" rev-parse HEAD)
pr=https://github.com/example/sample-site/pull/7
jq -n --arg h "$head" '{state: "OPEN", headRefOid: $h, mergeCommit: null,
  statusCheckRollup: [{__typename: "StatusContext", context: "andon/gate", state: "SUCCESS", description: "andon gate: approve"}]}' >"$tmp/gh/view"

ledger_init V
id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の止める PR" --profile claude)
why=$(HC_TERMINAL_ID=tc "$kanban" add "$run のオーナーの答えを待つ問い")
gone=$(HC_TERMINAL_ID=tc "$kanban" add "$run の片付いた問い")
HC_TERMINAL_ID=tc "$kanban" set "$id" review=off must_fix=major merge=person \
  "worktree=$tmp/wt" repo=github.com/example/sample-site >/dev/null
# 担当の端末 tw が受け持つ（kanban start が agent に tw を書く。止めの読み方は agent に書かれた端末を担当と読む）
HC_TERMINAL_ID=tw "$kanban" start "$id" >/dev/null || fail "担当が kanban start で受け持てない（前提）"
HC_TERMINAL_ID=tc "$kanban" done "$gone" >/dev/null
lib=$tmp/project/kanban/issues/$id/library
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
echo "{\"labels\":{\"issue\":\"$id\"}}" >"$tmp/hc/tw.json"
hold_of() { "$kanban" show "$id" --json | jq -r '.issue.hold // ""'; }

# 担当が報告し（PR の URL が report.json に入る）、Chief of Staff が review に入れた状態にする
HC_TERMINAL_ID=tw "$andon" issue report "$id" --pr "$pr" <<<"## Report
$run の止める PR" >/dev/null || fail "担当の報告が書けない（前提）"
HC_TERMINAL_ID=tc "$kanban" set "$id" status=review >/dev/null
export HC_TERMINAL_ID=tc

# hd-refuse: 理由の課題が自分自身・片付いた課題なら、何も書かずに断る
step "hd-refuse: 理由の課題に $id 自身を渡す" "$andon" issue hold "$id" "$id" \
  && fail "hd-refuse: 自分自身を理由にした hold が通った" \
  || { grep -qF "自身は書けない" "$tmp/step.out" && [ -z "$(hold_of)" ] && ok "hd-refuse: 自分自身を理由にすると断り、hold を書かない" || fail "hd-refuse: 断ったのに hold を書いた"; }
step "hd-refuse: 片付いた課題 $gone を理由に渡す" "$andon" issue hold "$id" "$gone" \
  && fail "hd-refuse: 片付いた課題を理由にした hold が通った" \
  || { grep -qF "もう片付いている" "$tmp/step.out" && [ -z "$(hold_of)" ] && ok "hd-refuse: 片付いた課題を理由にすると断り、hold を書かない" || fail "hd-refuse: 断ったのに hold を書いた"; }

# hd-hold: hold に理由の課題を書き、緑の印の commit に黄（pending）を書いてから返る。別の口（kanban show・log.jsonl）で読み直す
: >"$tmp/gh.calls"
step "hd-hold: Chief of Staff が $id を止める（理由の課題 $why）" "$andon" issue hold "$id" "$why" \
  && [ "$(hold_of)" = "$why" ] \
  && grep -qF "\"by\":\"terminal:tc\",\"id\":\"$id\",\"change\":{\"hold\":\"$why\"}" "$tmp/project/kanban/log.jsonl" \
  && grep -qxF "gh api -X POST repos/example/sample-site/statuses/$head -f context=andon/gate -f state=pending -f description=andon gate: メンテナーの確認を待っています" "$tmp/gh.calls" \
  && ok "hd-hold: hold=$why が Chief of Staff の端末の行で残り、PR の head に黄の印を書いた" \
  || fail "hd-hold: hold が無いか、黄の印を書いていない"
echo "  kanban show: $("$kanban" show "$id" --json | jq -c '.issue | {id, status, agent, hold}')"
sed 's/^/  gh.calls: /' "$tmp/gh.calls"

# hd-gate-wait: 止めている間、gate は GitHub を読まずに wait を返し、記録を書かない
: >"$tmp/gh.calls"
step "hd-gate-wait: 止めている課題に andon gate を打つ" "$andon" gate "$id"
rc=$?
[ "$rc" = 5 ] && jq -e --arg w "$why" '.decision == "wait" and (.reasons[0] | contains($w))' "$tmp/step.out" >/dev/null \
  && [ ! -s "$tmp/gh.calls" ] && ! compgen -G "$lib/gate/*.json" >/dev/null \
  && ok "hd-gate-wait: gate は終了コード 5 の wait で理由の課題 $why を名指し、GitHub を読まず記録も書かない" \
  || fail "hd-gate-wait: 止めているのに gate が wait でないか、GitHub を読んだか、記録を書いた"

# hd-worker: 担当（agent の端末 tw）が hold を消しても止めたまま（gate は wait のまま）
# 担当が消せたこと（今の hold が空で、hold を最後に変えた行が terminal:tw）を先に確かめる。
# 消せていなければ gate は hold の欄だけで wait を返し、担当が消した止めの読み方を通らないまま ok になる
wrc=0; HC_TERMINAL_ID=tw "$kanban" set "$id" hold= >"$tmp/step.out" 2>&1 || wrc=$?
echo "  担当 tw が kanban set $id hold= を打った（終了コード $wrc）。今の hold: '$(hold_of)'"
last_hold_by() { jq -sr --arg id "$id" '[.[] | select(.id == $id and (.change | has("hold")))] | last | .by // ""' "$tmp/project/kanban/log.jsonl"; }
cleared=false
[ "$wrc" = 0 ] && [ -z "$(hold_of)" ] && [ "$(last_hold_by)" = terminal:tw ] && cleared=true
[ "$cleared" = true ] || fail "hd-worker: 担当が hold を消せていない（終了コード $wrc・hold '$(hold_of)'・最後に変えた端末 $(last_hold_by)）"
: >"$tmp/gh.calls"
step "hd-worker: 担当が消したあとに andon gate を打つ" "$andon" gate "$id"
rc=$?
[ "$cleared" = true ] && [ "$rc" = 5 ] && jq -e --arg w "$why" '.decision == "wait" and (.reasons[0] | contains($w))' "$tmp/step.out" >/dev/null && [ ! -s "$tmp/gh.calls" ] \
  && ok "hd-worker: 担当が消した hold は止めたままと読み、gate は wait のまま" \
  || fail "hd-worker: 担当が hold を消したら gate が止めを外した"

# hd-unhold: Chief of Staff が外す。担当が消した止めも外せて、印は書かない。2回目は止めていないと返す
: >"$tmp/gh.calls"
step "hd-unhold: Chief of Staff が止めを外す" "$andon" issue unhold "$id" \
  && grep -qF "止めを外した（理由の課題 $why）" "$tmp/step.out" && [ ! -s "$tmp/gh.calls" ] \
  && jq -se --arg id "$id" '[.[] | select(.id == $id and (.change | has("hold")))] | last | .by == "terminal:tc" and (.change.hold // "") == ""' \
    "$tmp/project/kanban/log.jsonl" >/dev/null \
  && ok "hd-unhold: 担当が消した止めも理由の課題 $why ごと外し、印は書かない" \
  || fail "hd-unhold: 止めを外せないか、印を書いた"
step "hd-unhold: もう一度外す" "$andon" issue unhold "$id" \
  && grep -qF "止めていない" "$tmp/step.out" \
  && ok "hd-unhold: 外したあとは止めていないと返す" || fail "hd-unhold: 外したあとも止めていると読む"
jq -c --arg id "$id" 'select(.id == $id and (.change | has("hold"))) | {by, change}' "$tmp/project/kanban/log.jsonl" | sed 's/^/  log.jsonl: /'

# ---- dispatcher の巡（hd-dispatch-*）: 本物の andon dispatcher --once を、偽のセルの repo（オーナーが条件表と委譲を commit）の
# プロジェクトで回す。偽の gh を、gate とマージと読み直しの呼び出しを知るものに替える（事実は $tmp/dg/ のファイル）:
#   PR の事実 view.json・checks（graphql）・ファイル files.json・差分 diff。印の書き込みは marks に、マージは merges に足して merged を置く
d=$tmp/dg; mkdir -p "$d"
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >>"$tmp/gh.calls"
st() { if [ -f $d/merged ]; then echo MERGED; else echo OPEN; fi; }
case "\$*" in
  "pr view 7 --repo example/sample-site --json headRefOid -q .headRefOid"|"pr view https://github.com/example/sample-site/pull/7 --json headRefOid -q .headRefOid") jq -r .headRefOid $d/view.json ;;
  "pr view 7 --repo example/sample-site --json state,mergeCommit"|"pr view 7 --repo example/sample-site --json state,mergeCommit,headRefOid"|"pr view 7 --repo example/sample-site --json state,mergeCommit,mergedAt")
    jq --arg s "\$(st)" '{state: \$s, headRefOid, mergeCommit: (if \$s == "MERGED" then {oid: .headRefOid} else null end), mergedAt: (if \$s == "MERGED" then "2026-01-01T00:00:00Z" else null end)}' $d/view.json ;;
  "pr view 7 --repo example/sample-site --json state,mergeCommit,headRefOid,statusCheckRollup")
    jq --arg s "\$(st)" --arg m "\$(tail -1 $d/marks 2>/dev/null)" '{state: \$s, headRefOid, mergeCommit: (if \$s == "MERGED" then {oid: .headRefOid} else null end),
      statusCheckRollup: (if \$m == "" then [] else [{__typename: "StatusContext", context: "andon/gate", state: (\$m | ascii_upcase)}] end)}' $d/view.json ;;
  "pr view 7 --repo example/sample-site --json state,commits,comments,reviews") jq --arg s "\$(st)" '{state: \$s, commits: [{oid: .headRefOid, messageHeadline: "x"}], comments: [], reviews: []}' $d/view.json ;;
  "pr view https://github.com/example/sample-site/pull/7 --json state,mergeCommit,files") jq --arg s "\$(st)" '{state: \$s, mergeCommit: null, files: []}' $d/view.json ;;
  "pr view 7 --repo example/sample-site --json "*) cat $d/view.json ;;
  "api graphql "*) jq '{data: {viewer: {login: "verify-bot"}, repository: {defaultBranchRef: {name: "main"}, pullRequest: {headRefOid: .headRefOid,
      commits: {nodes: [{commit: {oid: .headRefOid, statusCheckRollup: {contexts: {pageInfo: {hasNextPage: false, endCursor: "1"},
        nodes: [{__typename: "CheckRun", name: "test", status: "COMPLETED", conclusion: "SUCCESS"}]}}}}]}}}}}' $d/view.json ;;
  "api --paginate --slurp repos/example/sample-site/pulls/7/files") cat $d/files.json ;;
  "pr diff 7 --repo example/sample-site") cat $d/diff ;;
  "api repos/example/sample-site/compare/"*) echo '{"ahead_by":0,"files":[]}' ;;
  "api -X POST repos/example/sample-site/statuses/"*) printf '%s\n' "\$@" | sed -n 's/^state=//p' >>$d/marks ;;
  "pr comment 7 --repo example/sample-site --body "*) echo "https://github.com/example/sample-site/pull/7#issuecomment-1" ;;
  "pr merge 7 --repo example/sample-site "*) echo "\$*" >>$d/merges; touch $d/merged ;;
  *) echo "偽の gh: 知らない呼び出し \$*" >&2; exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/gh"

# 偽のセルの repo: オーナーが条件表（squash）と委譲（preset work）を commit する。プロジェクト hd の kanban の Chief of Staff は tc
ws=${ANDON_WORKSPACE:?}
git init -q "$ws" && printf 'owner = "owner@example.com"\n' >"$ws/andon.toml"
echo '{"repos": {"github.com/example/sample-site": {"merge": "squash"}}}' >"$ws/gate.local.json"
mkdir -p "$ws/projects/hd"
echo '{"version": 1, "project": {"title": "Hold", "prefix": "W"}, "repos": ["github.com/example/sample-site"], "delegation": {"version": 1, "project": "hd", "preset": "work"}}' >"$ws/projects/hd/project.json"
git -C "$ws" add -A && GIT_AUTHOR_EMAIL=owner@example.com GIT_COMMITTER_EMAIL=owner@example.com git -C "$ws" commit -qm plant
cd "$ws/projects/hd" && KANBAN_ROOT=$PWD "$kanban" init --prefix W --repo github.com/example/sample-site >/dev/null && export KANBAN_ROOT=$PWD
HC_TERMINAL_ID=tc "$kanban" config set chief_of_staff=tc >/dev/null
# dispatcher は Chief of Staff の端末の形（address・state）を読む。読めなければその kanban を回さない
echo '{"address":"tc","state":"live","status":{"state":"working"},"labels":{"role":"chief-of-staff","project":"hd"}}' >"$tmp/hc/tc.json"
dlog=$tmp/dispatcher.log
dispatch() { : >"$tmp/gh.calls"; : >"$dlog"; "$andon" dispatcher --once --project hd >/dev/null 2>"$dlog"; }
# patrolled: 巡がこの kanban を回した（回さなかったときの行が無い）。回していなければ、gate もマージも起きないのは当たり前で証拠にならない
patrolled() { ! grep -qF "この巡はこの kanban を回さない" "$dlog"; }
gated() { compgen -G "$KANBAN_ROOT/kanban/issues/$1/library/gate/*.json" >/dev/null; }
# mkpr <タイトル> <merge>: Chief of Staff が課題を切り、担当 tw が受け持って文書1行の PR 7 を出し、報告して ready する（review: off・レベル A）。
# 標準出力は課題の id だけ。どこかで落ちたら 1 を返す（呼び手が fail を数える）
mkpr() {
  local i wt
  i=$(HC_TERMINAL_ID=tc "$kanban" add "$run の $1" --profile claude --repo github.com/example/sample-site) && [ -n "$i" ] || return 1
  wt=$tmp/wt-$i
  git init -q -b "verify/$i" "$wt" && mkdir -p "$wt/docs" && echo "$i" >"$wt/docs/verify.md" && git -C "$wt" add -A && git -C "$wt" commit -qm "$i" || return 1
  HC_TERMINAL_ID=tc "$kanban" set "$i" review=off must_fix=major "merge=$2" "worktree=$wt" "branch=verify/$i" >/dev/null || return 1
  (cd "$wt" && HC_TERMINAL_ID=tw "$kanban" start "$i" >/dev/null) || return 1
  jq -n --arg h "$(git -C "$wt" rev-parse HEAD)" --arg b "verify/$i" '{state: "OPEN", isDraft: false, headRefName: $b, headRefOid: $h, baseRefName: "main",
    mergeStateStatus: "CLEAN", author: {login: "verify-bot"}, changedFiles: 1, isCrossRepository: false}' >"$d/view.json"
  echo '[[{"filename":"docs/verify.md","additions":1,"deletions":0}]]' >"$d/files.json"
  printf '%s\n' 'diff --git a/docs/verify.md b/docs/verify.md' '--- /dev/null' '+++ b/docs/verify.md' '@@ -0,0 +1 @@' "+$i" >"$d/diff"
  rm -f "$d/merged" "$d/merges"
  (cd "$wt" && HC_TERMINAL_ID=tw "$andon" issue report "$i" --pr https://github.com/example/sample-site/pull/7 <<<"## Report" >/dev/null \
    && HC_TERMINAL_ID=tw "$andon" issue ready "$i" >/dev/null) >&2 || return 1
  echo "$i"
}
dwhy=$(HC_TERMINAL_ID=tc "$kanban" add "$run の答えを待つ問い")

# hd-dispatch-hold: merge: auto の課題を、dispatcher が gate を回す前に止める。巡で gate もマージも起きない
a=$(mkpr "自動でマージする PR" auto) || fail "前提: merge: auto の課題を ready まで進められない"
HC_TERMINAL_ID=tc "$andon" issue hold "$a" "$dwhy" >/dev/null || fail "前提: $a を止められない"
dispatch; drc=$?
echo "-- hd-dispatch-hold: 止めた $a（merge: auto）で andon dispatcher --once（終了コード $drc）"
sed 's/^/  | /' "$dlog"; sed 's/^/  gh.calls: /' "$tmp/gh.calls"
[ "$drc" = 0 ] && patrolled && [ ! -s "$d/merges" ] && ! gated "$a" && [ "$("$kanban" show "$a" --json | jq -r .issue.status)" = review ] \
  && ok "hd-dispatch-hold: 止めている $a に dispatcher は gate を回さず、マージ（gh pr merge）もせず、課題は review のまま" \
  || fail "hd-dispatch-hold: 止めているのに dispatcher が gate を回したか、マージしたか、課題が動いた"

# hd-dispatch-release: 外すと、次の巡で gate が回り、approve で dispatcher がマージする（上の確かめが何も見ていないことの対照）
HC_TERMINAL_ID=tc "$andon" issue unhold "$a" >/dev/null || fail "前提: $a の止めを外せない"
dispatch; drc=$?
echo "-- hd-dispatch-release: 外した $a で andon dispatcher --once（終了コード $drc）"
sed 's/^/  | /' "$dlog"; sed 's/^/  merges: /' "$d/merges" 2>/dev/null
gated "$a" && jq -c '{decision, tier}' "$KANBAN_ROOT"/kanban/issues/"$a"/library/gate/*.json | sed 's/^/  gate: /'
gated "$a" && [ -s "$d/merges" ] \
  && ok "hd-dispatch-release: 止めを外すと、dispatcher が gate を回してマージした（止めがマージを止めていた）" \
  || fail "hd-dispatch-release: 外しても gate が回らないかマージしない（この場面では止めがマージを止めたと言えない）"

# hd-dispatch-discard: merge: person で gate が approve を決めたあとに止める。次の巡で判定を捨て、merge: auto に変えてもマージしない
p=$(mkpr "オーナーがマージする PR" person) || fail "前提: merge: person の課題を ready まで進められない"
dispatch
gated "$p" && jq -c '{decision, tier}' "$KANBAN_ROOT"/kanban/issues/"$p"/library/gate/*.json | sed 's/^/  gate: /'
gated "$p" && jq -e '.decision == "approve"' "$KANBAN_ROOT"/kanban/issues/"$p"/library/gate/*.json >/dev/null && [ ! -s "$d/merges" ] \
  || fail "前提: $p で gate が approve を決めていないか、merge: person なのにマージした"
HC_TERMINAL_ID=tc "$andon" issue hold "$p" "$dwhy" >/dev/null || fail "前提: $p を止められない"
HC_TERMINAL_ID=tc "$kanban" set "$p" merge=auto >/dev/null
dispatch; drc=$?
echo "-- hd-dispatch-discard: approve のあとに止めて merge: auto にした $p で andon dispatcher --once（終了コード $drc）"
sed 's/^/  | /' "$dlog"
[ "$drc" = 0 ] && patrolled && grep -qF "$p は gate の判定（approve）のあとに止められた" "$dlog" && [ ! -s "$d/merges" ] \
  && ok "hd-dispatch-discard: approve のあとに止めた $p の判定を dispatcher が捨て、merge: auto にしてもマージしない" \
  || fail "hd-dispatch-discard: approve のあとに止めた判定を捨てないか、マージした"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件"
sed 's/^/  /' "$tmp/hc.calls"
[ "$fails" = 0 ]
