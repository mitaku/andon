#!/usr/bin/env bash
# dispatcher のマージ（features/merge.md）。本物の andon dispatcher run --once を、偽のセルの repo（オーナーが条件表と委譲を commit）の
# プロジェクトで回し、merge: auto の approve を dispatcher がマージしたあと、PR の読み直し（gh pr view）が一時に落ちたときの道を通す。
# 偽の gh は gate とマージと読み直しの呼び出しを知る（事実は $tmp/dg/ のファイル）:
#   PR の事実 view.json・checks（graphql）・ファイル files.json・差分 diff。印の書き込みは marks に、マージは merges に足して merged を置く。
#   viewfail（数）があれば、マージのあとの読み直し（--json state,mergeCommit）がその回数だけ 502 で落ちる。
#   rereadfail（数）があれば、gh pr merge の失敗のあとの読み直し（--json state,mergeCommit,headRefOid）が同じに落ちる。
#   race があれば、gh pr merge はオーナーが直前にマージしていた形で失敗する（merged を置いてから 1 で終わる）。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

d=$tmp/dg; mkdir -p "$d"
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >>"$tmp/gh.calls"
st() { if [ -f $d/merged ]; then echo MERGED; else echo OPEN; fi; }
flaky() { n=\$(cat "$d/\$1" 2>/dev/null) && [ "\$n" -gt 0 ] || return 0; echo \$((n - 1)) >"$d/\$1"; echo "HTTP 502: Bad Gateway" >&2; exit 1; }
case "\$*" in
  "pr view 7 --repo example/sample-site --json headRefOid -q .headRefOid"|"pr view https://github.com/example/sample-site/pull/7 --json headRefOid -q .headRefOid") jq -r .headRefOid $d/view.json ;;
  "pr view https://github.com/example/sample-site/pull/7 --json headRefName -q .headRefName") jq -r .headRefName $d/view.json ;;
  "pr view 7 --repo example/sample-site --json state,mergeCommit"|"pr view 7 --repo example/sample-site --json state,mergeCommit,headRefOid"|"pr view 7 --repo example/sample-site --json state,mergeCommit,mergedAt")
    [ "\$7" != state,mergeCommit ] || flaky viewfail
    [ "\$7" != state,mergeCommit,headRefOid ] || flaky rereadfail
    jq --arg s "\$(st)" '{state: \$s, headRefOid, mergeCommit: (if \$s == "MERGED" then {oid: .headRefOid} else null end), mergedAt: (if \$s == "MERGED" then "2026-01-01T00:00:00Z" else null end)}' $d/view.json ;;
  "pr view 7 --repo example/sample-site --json state,mergeCommit,headRefOid,statusCheckRollup,headRefName,isCrossRepository,baseRefName")
    jq --arg s "\$(st)" --arg m "\$(tail -1 $d/marks 2>/dev/null)" '{state: \$s, headRefOid, headRefName, isCrossRepository, baseRefName, mergeCommit: (if \$s == "MERGED" then {oid: .headRefOid} else null end),
      statusCheckRollup: (if \$m == "" then [] else [{__typename: "StatusContext", context: "andon/gate", state: (\$m | ascii_upcase)}] end)}' $d/view.json ;;
  "repo view example/sample-site --json defaultBranchRef -q .defaultBranchRef.name") echo main ;;   # settle がマージされた PR の base と比べる
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
  "pr merge 7 --repo example/sample-site "*) echo "\$*" >>$d/merges; touch $d/merged
    if [ -f $d/race ]; then rm $d/race; echo "Pull request example/sample-site#7 was already merged" >&2; exit 1; fi ;;
  *) echo "偽の gh: 知らない呼び出し \$*" >&2; exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/gh"

# 偽のセルの repo: オーナーが条件表（squash）と委譲（preset work）を commit する。プロジェクト mg の kanban の Chief of Staff は tc
ws=${ANDON_WORKSPACE:?}
git init -q "$ws" && printf 'owner = "owner@example.com"\n' >"$ws/andon.toml"
echo '{"repos": {"github.com/example/sample-site": {"merge": "squash"}}}' >"$ws/gate.local.json"
mkdir -p "$ws/projects/mg"
echo '{"version": 1, "project": {"title": "Merge", "prefix": "M"}, "repos": ["github.com/example/sample-site"], "delegation": {"version": 1, "project": "mg", "preset": "work"}}' >"$ws/projects/mg/project.json"
git -C "$ws" add -A && GIT_AUTHOR_EMAIL=owner@example.com GIT_COMMITTER_EMAIL=owner@example.com git -C "$ws" commit -qm plant
cd "$ws/projects/mg" && KANBAN_ROOT=$PWD "$kanban" init --prefix M --repo github.com/example/sample-site >/dev/null && export KANBAN_ROOT=$PWD
HC_TERMINAL_ID=tc "$kanban" config set chief_of_staff=tc >/dev/null
# dispatcher は Chief of Staff の端末の形（address・state）を読む。読めなければその kanban を回さない
echo '{"address":"tc","state":"live","status":{"state":"working"},"labels":{"role":"chief-of-staff","project":"mg"}}' >"$tmp/hc/tc.json"
dlog=$tmp/dispatcher.log
dispatch() { : >"$tmp/gh.calls"; : >"$dlog"; "$andon" dispatcher run --once --project mg >/dev/null 2>"$dlog"; }
# reads <json の欄>: この巡で偽の gh に打った gh pr view 7 --json <欄> の数
reads() { grep -cxF "gh pr view 7 --repo example/sample-site --json $1" "$tmp/gh.calls" || true; }
gate_merged() { jq -r '.merged // false' "$KANBAN_ROOT"/kanban/issues/"$1"/library/gate/*.json 2>/dev/null; }
# asked <id>: 課題に出た問いの文（library/questions/<n>.json の question）を1行1つ。あとで閉じられた問いも出る。
# マージできなかった問いは文に $cannot を含む。切り離した場所では finish が片付けに失敗した問い（「マージしたが片付けに失敗した」）も出るが、
# それはこの道の外（worktree が本物の clone の worktree でない）なので数えない。同じ巡の settle がオーナーのマージとして問いを閉じることもある
cannot='dispatcher がマージできなかった'
asked() { cat "$KANBAN_ROOT"/kanban/issues/"$1"/library/questions/*.json 2>/dev/null | jq -r '.question'; }
show() { # show <名前> <id>: 巡の出力と、偽の gh への読み直しと、課題の今を証拠に出す
  echo "-- $1: $2 で andon dispatcher run --once（終了コード $drc）"
  sed 's/^/  | /' "$dlog"; grep -F 'gh pr ' "$tmp/gh.calls" | sed 's/^/  gh.calls: /'
  echo "  status: $("$kanban" show "$2" --json | jq -r .issue.status)・gate の記録の merged: $(gate_merged "$2")"
  asked "$2" | sed 's/^/  問い: /'
}
# mkpr <タイトル>: Chief of Staff が merge: auto の課題を切り、担当 tw が受け持って文書1行の PR 7 を出し、報告して ready する（review: off・レベル A）。
# 標準出力は課題の id だけ。どこかで落ちたら 1 を返す（呼び手が fail を数える）
mkpr() {
  local i wt
  i=$(HC_TERMINAL_ID=tc "$kanban" add "$run の $1" --profile claude --repo github.com/example/sample-site) && [ -n "$i" ] || return 1
  wt=$tmp/wt-$i
  git init -q -b "verify/$i" "$wt" && mkdir -p "$wt/docs" && echo "$i" >"$wt/docs/verify.md" && git -C "$wt" add -A && git -C "$wt" commit -qm "$i" || return 1
  HC_TERMINAL_ID=tc "$kanban" set "$i" review=off must_fix=major merge=auto "worktree=$wt" "branch=verify/$i" >/dev/null || return 1
  (cd "$wt" && HC_TERMINAL_ID=tw "$kanban" start "$i" >/dev/null) || return 1
  jq -n --arg h "$(git -C "$wt" rev-parse HEAD)" --arg b "verify/$i" '{state: "OPEN", isDraft: false, headRefName: $b, headRefOid: $h, baseRefName: "main",
    mergeStateStatus: "CLEAN", author: {login: "verify-bot"}, changedFiles: 1, isCrossRepository: false}' >"$d/view.json"
  echo '[[{"filename":"docs/verify.md","additions":1,"deletions":0}]]' >"$d/files.json"
  printf '%s\n' 'diff --git a/docs/verify.md b/docs/verify.md' '--- /dev/null' '+++ b/docs/verify.md' '@@ -0,0 +1 @@' "+$i" >"$d/diff"
  rm -f "$d/merged" "$d/merges" "$d/marks" "$d/viewfail" "$d/rereadfail" "$d/race"
  (cd "$wt" && HC_TERMINAL_ID=tw "$andon" issue report "$i" --pr https://github.com/example/sample-site/pull/7 <<<"## Report" >/dev/null \
    && HC_TERMINAL_ID=tw "$andon" issue ready "$i" >/dev/null) >&2 || return 1
  echo "$i"
}

# mg-reread: gh pr merge は通り、そのあとの読み直しが1回だけ落ちる。問いにせず読み直して MERGED を確かめ、dispatcher のマージとして記録する
a=$(mkpr "読み直しが1回落ちる PR") || fail "前提: mg-reread の課題を ready まで進められない"
echo 1 >"$d/viewfail"
dispatch; drc=$?
show mg-reread "$a"
[ "$drc" = 0 ] && [ -s "$d/merges" ] && [ "$(cat "$d/viewfail")" = 0 ] && [ "$(reads state,mergeCommit)" = 2 ] \
  && grep -qF "$a の PR https://github.com/example/sample-site/pull/7 をマージした" "$dlog" && ! grep -qF "$a をマージしない" "$dlog" \
  && [ "$(gate_merged "$a")" = true ] && ! asked "$a" | grep -qF "$cannot" \
  && ok "mg-reread: マージのあとの読み直しが1回落ちても問いにせず、2回目で MERGED を確かめて gate の記録に merged を書いた" \
  || fail "mg-reread: 読み直しが1回落ちただけで問いにしたか、読み直していないか、マージを記録していない"

# mg-reread-exhausted: 読み直しが3回とも落ちる。MERGED を確かめられないので片付けず、gate の記録に merged を書かず、問いにする
b=$(mkpr "読み直しが3回落ちる PR") || fail "前提: mg-reread-exhausted の課題を ready まで進められない"
echo 3 >"$d/viewfail"
dispatch; drc=$?
show mg-reread-exhausted "$b"
[ "$drc" = 0 ] && [ -s "$d/merges" ] && [ "$(reads state,mergeCommit)" = 3 ] && grep -qF "$b をマージしない" "$dlog" \
  && [ "$(gate_merged "$b")" != true ] && ! grep -qF "$b の PR https://github.com/example/sample-site/pull/7 をマージした" "$dlog" \
  && [ "$(asked "$b" | grep -cF "$cannot")" = 1 ] \
  && ok "mg-reread-exhausted: 読み直しが3回とも落ちたら、MERGED を確かめずに片付けず、gate の記録に merged を書かず、問いを1つ出した" \
  || fail "mg-reread-exhausted: 3回読み直していないか、確かめずに片付けたか、問いを出していない"

# mg-reread-race: オーナーが直前に同じ head でマージしていて gh pr merge が失敗し、そのあとの読み直しが1回だけ落ちる。
# 問いにせず読み直して MERGED と同じ head を確かめ、オーナーのマージの道（settle）へ渡す
c=$(mkpr "オーナーが先にマージして読み直しが1回落ちる PR") || fail "前提: mg-reread-race の課題を ready まで進められない"
touch "$d/race"; echo 1 >"$d/rereadfail"
dispatch; drc=$?
show mg-reread-race "$c"
[ "$drc" = 0 ] && [ -s "$d/merges" ] && [ ! -e "$d/race" ] && [ "$(cat "$d/rereadfail")" = 0 ] && [ "$(reads state,mergeCommit,headRefOid)" = 2 ] \
  && grep -qF "$c の PR https://github.com/example/sample-site/pull/7 は gh pr merge の前に同じ head でマージされていた" "$dlog" \
  && ! grep -qF "$c をマージしない" "$dlog" && [ "$(gate_merged "$c")" != true ] && ! asked "$c" | grep -qF "$cannot" \
  && ok "mg-reread-race: gh pr merge の失敗のあとの読み直しが1回落ちても問いにせず、2回目で MERGED と同じ head を確かめてオーナーのマージの道へ渡した" \
  || fail "mg-reread-race: 読み直しが1回落ちただけで問いにしたか、読み直していないか、オーナーのマージの道へ渡していない"

touch "$tmp/hc.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件"
[ "$fails" = 0 ]
