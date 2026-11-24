#!/usr/bin/env bash
# CI の go・tests の job と手元の tests/run が使う道具（tests/lib/image-scope.sh・docs-scope.sh・scope.sh・run-tests.sh）と tests/run の振る舞い。
set -euo pipefail
cd "$(dirname "$0")/.."
die() { echo "FAIL ci: $*" >&2; exit 1; }
ok() { echo "ok   $*"; }

# 1. image-scope: image に入らず tests/image.sh も読まない変更だけなら false。知らないパスは true
scope() { printf '%s\n' "$@" | bash tests/lib/image-scope.sh; }
[ "$(scope docs/image.md README.md CHANGELOG.md AGENTS.md REVIEW.md SECURITY.md LICENSE)" = false ] || die "人用の文書だけで true"
[ "$(scope tests/andon-issue.sh tests/kanban/test-deck.mjs workflow_test.go internal/issue/issue_test.go)" = false ] || die "ほかの tests・Go のテストだけで true"
[ "$(scope CHANGELOG.md)" = false ] || die "CHANGELOG.md だけで true"
[ "$(scope model/IssueLife.tla model/IssueRules.tla model/expected.txt model/broken/BrokenReady.tla model/package-lock.json model/README.md)" = false ] || die "モデルだけで true"
[ "$(scope)" = false ] || die "変更が無いのに true"
[ "$(scope image/apply-main.sh tests/apply-main.sh)" = false ] || die "image に入らない image/apply-main.sh で true"
# repo の中の skill の置き場（ADR-0035 受け入れ条件 9）: 文と補助のスクリプトだけの変更で image を焼かない。ほかの .claude/ は知らないパス
skill='.claude/skills/verify-andon'
[ "$(scope $skill/SKILL.md $skill/features/gate.md $skill/scripts/drive.sh .agents/skills/verify-andon)" = false ] || die "検証の skill の文だけで true"
[ "$(scope .claude/settings.json)" = true ] || die ".claude/ の skill でないもので false"
# yomiyasu の許可の一覧: image に入らないので一覧だけの変更で image を焼かない。名前の似た別の置き場は知らないパス
[ "$(scope .yomiyasu-words/allow.txt)" = false ] || die ".yomiyasu-words/ の一覧だけで true"
[ "$(scope .yomiyasu-wordsx/allow.txt)" = true ] || die ".yomiyasu-words/ でない名前の似た置き場で false"
grep -q 'yomiyasu-words' .dockerignore && die ".yomiyasu-words/ を image に入れるなら image-scope.sh も直す"
grep -q '^!package/$' .dockerignore || die ".dockerignore の !package/ の行が読めない（上の否定が何も見ていない）"
grep -q 'COPY .*image/apply-main' image/Dockerfile && die "image/apply-main.sh を image に入れるなら image-scope.sh も直す"
grep -q 'COPY .*image/' image/Dockerfile || die "Dockerfile の COPY image/ の行が読めない（上の否定が何も見ていない）"
for p in image/Dockerfile image/build.sh package/skills/x/SKILL.md schema/report.schema.json cmd/andon/main.go \
  internal/issue/issue.go init/x go.mod go.sum embed.go .dockerignore mise.toml .github/workflows/tests.yml \
  package/lib/andon/x_test.go init/x_test.go image/x_test.go \
  tests/image.sh tests/lib/pty-probe.py tests/lib/run-tests.sh tests/lib/image-scope.sh docs-new/x NEW.md; do
  [ "$(scope docs/image.md "$p")" = true ] || die "$p を含むのに false"
done
ok "image-scope: 文書・ほかの tests だけなら飛ばし、image に入るもの・知らないパスでは走らせる"

# 1a. docs-scope: 変更が全部 docs/ の下か root の人用の文書なら true。コード・tests・知らないパスが1つでも混ざれば、空でも false
docs() { printf '%s\n' "$@" | bash tests/lib/docs-scope.sh; }
[ "$(docs docs/adr/0023-x.md docs/decisions/2026-11-12-x.md)" = true ] || die "ADR と決定の記録だけで false（PR #218 の形）"
[ "$(docs docs/assets/logo/x.svg README.md CHANGELOG.md AGENTS.md REVIEW.md SECURITY.md LICENSE)" = true ] || die "人用の文書だけで false"
[ "$(docs)" = false ] || die "変更が無いのに true"
for p in internal/issue/issue.go workflow_test.go tests/README.md tests/lib/docs-scope.sh .github/workflows/go.yml \
  package/skills/worker/SKILL.md schema/report.schema.json go.mod docs-new/x NEW.md; do
  [ "$(docs docs/image.md "$p")" = false ] || die "$p を含むのに true"
done
# 判定の付いたあとも入力を終わりまで読む（CI は pipefail で git diff から渡す）
( set -o pipefail; { echo internal/x.go; seq 100000 | sed 's|^|docs/|'; } | bash tests/lib/docs-scope.sh >/dev/null ) || die "途中で抜けて前のコマンドが落ちる"
ok "docs-scope: 文書だけなら true、コード・tests・知らないパスが混ざるか空なら false"

# 1b. scope（手元の tests/run --changed）: 文書だけならいつもの分、触った test とそれが読む文書の分を足す。コードでは
#     重い検査を除く全部と、変えたパスを見る重い検査。知らないパスでは全部。image.sh は image-scope と同じとき
pick() { printf '%s\n' "$@" | bash tests/lib/scope.sh | tr '\n' ' '; }
has() { grep -qw -- "$2" <<<"$1"; }
always='tests/agents-md.sh tests/ci.sh tests/public-selftest.sh tests/public.sh tests/shell-and.sh tests/smoke.sh tests/verify-andon.sh tests/words-selftest.sh tests/words.sh '
# with <検査>…: いつもの分に検査を足し、scope.sh と同じ並び（名前の順）で出す
with() { printf '%s\n' $always "$@" | sort | tr '\n' ' '; }
heavy='tests/andon-dispatcher.sh tests/schema.sh tests/apply-main.sh tests/andon-issue.sh tests/andon-gate.sh'
emitters=$(bash tests/schema.sh --emitters | tr '\n' ' ')
[ "$(pick docs/image.md)" = "$always" ] || die "文書だけ: $(pick docs/image.md)"
[ "$(pick README.md)" = "$always" ] || die "README.md だけ（adr-index.sh は読まない。ADR-0055）: $(pick README.md)"
[ "$(pick docs/adr/README.md)" = "$(with tests/adr-index.sh tests/package.sh)" ] || die "ADR の一覧で adr-index.sh・package.sh が無い: $(pick docs/adr/README.md)"
[ "$(pick docs/adr/0033-adr-inventory-and-slimming.md)" = "$(with tests/adr-index.sh tests/package.sh)" ] || die "ADR で adr-index.sh・package.sh が無い: $(pick docs/adr/0033-adr-inventory-and-slimming.md)"
for p in docs/decisions.md docs/decisions/2026-11-21-adr-0041-delegation-cap-out-of-command.md; do
  [ "$(pick "$p")" = "$(with tests/adr-index.sh)" ] || die "決定の記録で adr-index.sh だけを足さない: $(pick "$p")"
done
[ "$(pick tests/adr-new)" = "$(with tests/adr-new.sh)" ] || die "ADR の番号を取る command で adr-new.sh だけを足さない（ADR-0055）: $(pick tests/adr-new)"
[ "$(pick CHANGELOG.md)" = "$(with tests/adr-index.sh)" ] || die "CHANGELOG.md で adr-index.sh だけを足さない（一覧の作り直し忘れ。ADR-0055）: $(pick CHANGELOG.md)"
# AGENTS.md だけの PR でも、表の形を見る agents-md.sh が走る（ADR-0035 受け入れ条件 10）。CI の文書だけの PR も同じ scope.sh で選ぶ
[ "$(printf '%s\n' AGENTS.md | bash tests/lib/docs-scope.sh)" = true ] || die "AGENTS.md だけが文書だけにならない"
has "$(pick AGENTS.md)" tests/agents-md.sh || die "AGENTS.md だけで agents-md.sh が無い: $(pick AGENTS.md)"
[ "$(pick AGENTS.md)" = "$always" ] || die "AGENTS.md だけ: $(pick AGENTS.md)"
# contract.md は schema.sh・package.sh。schema.sh を選んだら、読ませる emitters も並べる（schema.sh が1本ずつ回し直さない）
got=$(pick docs/contract.md)
for t in tests/package.sh tests/schema.sh $emitters; do has "$got" "$t" || die "contract.md で $t が無い: $got"; done
[ "$(wc -w <<<"$got")" -eq $(($(wc -w <<<"$always $emitters") + 2)) ] || die "contract.md でほかの検査まで足す: $got"
[ "$(pick docs/no-such-doc.md)" = "$(with tests/kanban.sh)" ] || die "消した文書で kanban.sh が無い: $(pick docs/no-such-doc.md)"
[ "$(pick tests/andon-gate.sh)" = "$(with tests/andon-gate.sh)" ] || die "触った test が無い"
[ "$(pick tests/host/mod.mjs)" = "$(with tests/host.sh)" ] || die "tests/host/ で host.sh が無い: $(pick tests/host/mod.mjs)"
[ "$(pick tests/package/01-names.sh)" = "$(with tests/package.sh)" ] || die "tests/package/ で package.sh が無い: $(pick tests/package/01-names.sh)"
# yomiyasu の許可の一覧だけ: 正と突き合わせる package.sh を足すだけ。image.sh と重い検査は選ばない
got=$(pick .yomiyasu-words/allow.txt)
[ "$got" = "$(with tests/package.sh)" ] || die ".yomiyasu-words/ の一覧だけで package.sh だけを足さない: $got"
for t in tests/image.sh $heavy; do has "$got" "$t" && die ".yomiyasu-words/ の一覧だけで $t: $got"; done
[ "$(wc -w <<<"$(pick .yomiyasu-wordsx/allow.txt)")" -eq "$(ls tests/*.sh | wc -l)" ] || die ".yomiyasu-words/ でない名前の似た置き場で全部にならない"
grep -q tests/process.sh <<<"$(pick process/states/gate.yaml)" && ! grep -q tests/andon-issue.sh <<<"$(pick process/states/gate.yaml)" || die "埋め込まない状態の定義で process.sh を足すだけにならない: $(pick process/states/gate.yaml)"
[ "$(pick schema/process/state.schema.json)" = "$(with tests/image.sh tests/process.sh)" ] || die "schema/process/ で schema.sh まで足す: $(pick schema/process/state.schema.json)"
[ "$(pick internal/issue/issue_test.go)" = "$always" ] || die "Go のテストだけ: $(pick internal/issue/issue_test.go)"
[ "$(pick workflow_test.go cmd/kanban/main_test.go)" = "$always" ] || die "image に入らない Go のテストだけ: $(pick workflow_test.go)"
grep -q tests/image.sh <<<"$(pick package/lib/andon/x_test.go)" || die "package/ の下の _test.go で image.sh が無い"
n_all=$(ls tests/*.sh | wc -l)
for p in NEW go.mod mise.toml .github/workflows/tests.yml; do
  got=$(pick "$p")
  [ "$(wc -w <<<"$got")" -eq "$n_all" ] || die "$p（知らないパス・全部に効くもの）で全部にならない: $got"
done
[ "$(wc -w <<<"$(pick tests/lib/kanban.sh)")" -eq $((n_all - 1)) ] || die "tests/lib/kanban.sh で image.sh 以外の全部にならない"
grep -q tests/image.sh <<<"$(pick tests/lib/kanban.sh)" && die "image に関係しない tests/lib で image.sh"
grep -q tests/image.sh <<<"$(pick tests/lib/pty-probe.py)" || die "pty-probe.py で image.sh が無い"
[ "$(pick tests/lib/scope.sh tests/lib/docs-scope.sh)" = "$always" ] || die "判定の道具で ci.sh のほかを足す: $(pick tests/lib/scope.sh)"
[ "$(pick image/apply-main.sh)" = "$(with tests/apply-main.sh)" ] || die "image/apply-main.sh で apply-main.sh だけを足さない: $(pick image/apply-main.sh)"
# 検証の skill の文と補助のスクリプトだけ（ADR-0035 受け入れ条件 9）: いつもの分だけ。語の検査と skill の形の検査を選び、image と重い検査は選ばない
got=$(pick $skill/SKILL.md $skill/features/gate.md $skill/scripts/journeys/gate.sh .agents/skills/verify-andon)
[ "$got" = "$always" ] || die "検証の skill の文だけ: $got"
for t in tests/words.sh tests/verify-andon.sh; do has "$got" "$t" || die "検証の skill の文だけで $t が無い: $got"; done
for t in tests/image.sh $heavy; do has "$got" "$t" && die "検証の skill の文だけで $t: $got"; done
[ "$(wc -w <<<"$(pick .claude/settings.json)")" -eq "$n_all" ] || die ".claude/ の skill でないもので全部にならない"
ok "scope: 検証の skill の置き場（.claude/skills/・.agents/skills/）の文だけなら、image の build と重い検査を選ばず、語の検査と skill の形の検査を選ぶ"
ok "scope: 文書だけならいつもの分、触ったものに関係する分を足し、知らないパスでは全部（image.sh は image に関係するとき）"

# 1c. scope の重い検査（ADR-0032 決めたこと 4）: コードでは軽い検査を全部と、変えたパスを見る重い検査だけ
[ "$(bash tests/lib/scope.sh --heavy | cut -d' ' -f1 | sort -u | tr '\n' ' ')" = "$(tr ' ' '\n' <<<"$heavy" | sort | tr '\n' ' ')" ] \
  || die "scope.sh の重い検査が $heavy でない: $(bash tests/lib/scope.sh --heavy | cut -d' ' -f1 | sort -u | tr '\n' ' ')"
got=$(pick internal/measure/measure.go)
for t in tests/andon-dispatcher.sh tests/schema.sh tests/apply-main.sh tests/andon-issue.sh tests/andon-gate.sh; do has "$got" "$t" && die "internal/measure/measure.go で $t: $got"; done
has "$got" tests/andon-measure.sh || die "internal/measure/measure.go で andon-measure.sh が無い: $got"
[ "$(wc -w <<<"$got")" -eq $((n_all - 5)) ] || die "internal/measure/measure.go で重い検査を除く全部にならない: $got"
for t in tests/andon-dispatcher.sh tests/andon-issue.sh tests/andon-gate.sh; do has "$(pick internal/dispatcher/loop.go)" "$t" || die "internal/dispatcher/loop.go で $t が無い"; done
has "$(pick internal/dispatcher/loop.go)" tests/apply-main.sh && die "internal/dispatcher/loop.go で apply-main.sh"
got=$(pick package/skills/worker/SKILL.md)
for t in $heavy; do has "$got" "$t" && die "skill の文で $t: $got"; done
has "$(pick package/gate.json)" tests/andon-gate.sh && has "$(pick package/gate.json)" tests/andon-dispatcher.sh || die "package/gate.json で gate・dispatcher の検査が無い"
has "$(pick package/lib/andon/schema_check.py)" tests/schema.sh || die "schema_check.py で schema.sh が無い"
for p in internal/schema/schema.go internal/contract/schema.go; do has "$(pick "$p")" tests/schema.sh || die "$p（andon schema の検査器）で schema.sh が無い"; done
has "$(pick schema/report.schema.json)" tests/schema.sh || die "schema/ で schema.sh が無い"
has "$(pick internal/pkgversion/pkgversion.go)" tests/apply-main.sh || die "internal/pkgversion で apply-main.sh が無い"
for p in internal/cli/cli.go process/states/issue.yaml embed.go init/x schema/report.schema.json; do
  for t in tests/andon-dispatcher.sh tests/andon-issue.sh tests/andon-gate.sh; do has "$(pick "$p")" "$t" || die "$p（andon issue が使う）で $t が無い"; done
done
# 知らないパスは重い検査を含む（安全な側に倒す）
for t in $heavy; do has "$(pick NEW)" "$t" || die "知らないパスで $t が無い"; done
# 見る部分の Go の package は import で閉じている（cmd/andon・internal/cli は全部の入口なので除く）。
# 重い検査が打つ package が import する package を変えても、その検査が選ばれる
gobin=$(command -v go || mise which go 2>/dev/null || true)
if [ -n "$gobin" ]; then
  mod=$("$gobin" list -m)
  while read -r t; do
    parts=" $(bash tests/lib/scope.sh --heavy | awk -v t="$t" '$1 == t { print $2 }' | tr '\n' ' ')"
    pkgs=$(tr ' ' '\n' <<<"$parts" | grep -E '^(cmd|internal)/[^/]+/$|^embed\.go$' | grep -vxE 'cmd/andon/|internal/cli/' \
      | sed -e "s|^embed\.go\$|$mod|" -e "s|^|./|" -e "s|^\./$mod\$|.|" -e 's|/$||' || true)
    [ -n "$pkgs" ] || continue
    for d in $(cd . && "$gobin" list -deps $pkgs | grep -E "^$mod(/|\$)"); do
      case "$d" in "$mod") part=embed.go ;; *) part=${d#"$mod"/}/ ;; esac
      [[ "$parts" == *" $part "* ]] || die "$t の見る部分が $part を欠く（import している）"
    done
  done < <(bash tests/lib/scope.sh --heavy | cut -d' ' -f1 | sort -u)
  # 否定の確かめ（die しない）が何も見ていないことを避ける: dispatcher の検査は triage を import で辿っている
  "$gobin" list -deps ./internal/dispatcher | grep -qx "$mod/internal/triage" || die "go list -deps が import を出さない"
  ok "scope: 重い検査の見る部分の Go の package は import で閉じている"
elif [ "${CI:-false}" = true ]; then die "CI に go が無い"
else echo "skip scope の import の確かめ（go が無い）"; fi
ok "scope: 重い検査は見る部分を変えたときだけ（retro は dispatcher・schema・apply-main を外す）。知らないパスは重い検査を含む"

# 1d. scope --isolation（CI の job isolation が tests/isolation に渡す検査）: 検査の土台では image.sh を除く全部、
#     tests/ の検査と置き場ではその検査、Go では smoke.sh（go test は tests/isolation が毎回回す）。ほかは空で、job は回さない
iso() { printf '%s\n' "$@" | bash tests/lib/scope.sh --isolation | tr '\n' ' '; }
for p in tests/lib/isolate.sh tests/lib/kanban.sh tests/lib/andon.sh tests/lib/andon-refs.sh tests/lib/run-tests.sh tests/isolation .github/workflows/tests.yml; do
  got=$(iso "$p")
  [ "$(wc -w <<<"$got")" -eq $((n_all - 1)) ] || die "検査の土台 $p で image.sh を除く全部にならない: $got"
  has "$got" tests/image.sh && die "検査の土台 $p で image.sh（docker が要る）: $got"
done
# 検査が読む置き場で、隔離の漏れが main に入った形（tests/host/ の検査がセルの kanban を読んだ）
[ "$(iso tests/host/screen.mjs)" = "tests/host.sh " ] || die "tests/host/ で host.sh だけにならない: $(iso tests/host/screen.mjs)"
[ "$(iso package/host/kanban-board/index.js)" = "tests/host.sh " ] || die "host.sh が見る plugin のコードで host.sh だけにならない: $(iso package/host/kanban-board/index.js)"
[ "$(iso tests/kanban/test-deck.mjs)" = "tests/kanban.sh " ] || die "tests/kanban/ で kanban.sh だけにならない: $(iso tests/kanban/test-deck.mjs)"
[ "$(iso tests/package/01-names.sh)" = "tests/package.sh " ] || die "tests/package/ で package.sh だけにならない: $(iso tests/package/01-names.sh)"
[ "$(iso tests/andon-gate.sh)" = "tests/andon-gate.sh " ] || die "触った検査だけにならない: $(iso tests/andon-gate.sh)"
got=$(iso tests/schema.sh)
for t in tests/schema.sh $emitters; do has "$got" "$t" || die "schema.sh で $t が無い（emitters を並べる）: $got"; done
for p in internal/issue/issue.go cmd/andon/main.go internal/issue/issue_test.go internal/issue/testdata/x.json workflow_test.go embed.go go.mod go.sum; do
  [ "$(iso "$p")" = "tests/smoke.sh " ] || die "Go の $p で smoke.sh だけにならない: $(iso "$p")"
done
for p in docs/image.md README.md AGENTS.md tests/README.md package/skills/worker/SKILL.md schema/report.schema.json image/Dockerfile \
  model/IssueLife.tla mise.toml tests/lib/scope.sh tests/lib/docs-scope.sh tests/lib/image-scope.sh tests/lib/pty-probe.py tests/image.sh tests/no-such.sh; do
  [ -z "$(iso "$p")" ] || die "$p で偽のセルの検査を選ぶ: $(iso "$p")"
done
[ -z "$(iso)" ] || die "変更が無いのに偽のセルの検査を選ぶ"
( set -o pipefail; { echo tests/isolation; seq 100000 | sed 's|^|docs/|'; } | bash tests/lib/scope.sh --isolation >/dev/null ) || die "--isolation が途中で抜けて前のコマンドが落ちる"
# job isolation が PR で、選んだ検査を tests/isolation に渡す
# 行ごと比べる（注釈にした行・|| true を付けた行・step の if は当たらない）
job=$(sed -n '/^  isolation:$/,/^  [a-z][a-z-]*:$/p' .github/workflows/tests.yml)
[ -n "$job" ] || die "tests.yml に job isolation が無い"
grep -qxF "    if: github.event_name == 'pull_request'" <<<"$job" || die "job isolation が PR で動かない"
grep -qxF '          git diff --name-only --no-renames HEAD^1 HEAD | bash tests/lib/scope.sh --isolation | tee "$RUNNER_TEMP/isolation"' <<<"$job" \
  || die "job isolation が PR の変更を scope.sh --isolation で選ばない"
grep -qxF '          tests/isolation "${tests[@]}"' <<<"$job" || die "job isolation が選んだ検査を tests/isolation に渡さない"
# 落ちても成功にしない・選んだのに飛ばさない: job に continue-on-error が無く、tests/isolation を打つ step は選んだときだけ動き、
# tests/isolation の行で終わる（後ろに true などを足して終了コードを消さない）。選ぶ step は一覧が空でないとき run=true を出す
grep -qE '^[[:space:]]*continue-on-error:' <<<"$job" && die "job isolation に continue-on-error がある（落ちても成功になる）"
step='' cur=''
while IFS= read -r l; do
  [[ "$l" == '      - '* ]] && cur=''
  cur+="$l"$'\n'
  [ "$l" = '          tests/isolation "${tests[@]}"' ] && step=$cur
done <<<"$job"
[ -n "$step" ] || die "job isolation に tests/isolation を打つ step が無い"
[ "$(grep -cE '^[[:space:]]*if:' <<<"$step")" = 1 ] && grep -qxF "        if: steps.scope.outputs.run == 'true'" <<<"$step" \
  || die "tests/isolation を打つ step の if が選んだときだけ（steps.scope.outputs.run == 'true'）でない: $(grep -E '^[[:space:]]*if:' <<<"$step")"
after=$(sed -n '/^          tests\/isolation "\${tests\[@\]}"$/,/^      - /p' <<<"$job" | sed '1d' | grep -vE '^      - |^  [a-z][a-z-]*:$|^[[:space:]]*(#|$)' || true)
[ -z "$after" ] || die "tests/isolation を打つ行の後ろに行がある（終了コードを消しうる）: $after"
grep -qxF '        id: scope' <<<"$job" && grep -qxF '          if [ -s "$RUNNER_TEMP/isolation" ]; then' <<<"$job" \
  && grep -qxF '            echo "run=true" >>"$GITHUB_OUTPUT"' <<<"$job" || die "選ぶ step が、一覧が空でないとき run=true を出さない"
ok "scope --isolation: 検査の土台では全部、tests/ の検査と置き場ではその検査、Go では smoke.sh を、job isolation が tests/isolation で回す"

# 2. run-tests: 1本ごとに出力をまとめ（混ざらない）、表を出し、落ちたものがあれば 1。skip は走らせない
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for n in a b; do printf 'for i in 1 2 3; do echo %s$i; sleep 0.1; done\n' "$n" >"$tmp/$n.sh"; done
printf 'echo bad; exit 3\n' >"$tmp/c.sh"
printf 'touch %q/ran\n' "$tmp" >"$tmp/d.sh"
set +e
out=$(ANDON_TEST_JOBS=3 ANDON_TEST_SKIP="$tmp/d.sh" GITHUB_ACTIONS=false GITHUB_STEP_SUMMARY="$tmp/summary" \
  bash tests/lib/run-tests.sh "$tmp/a.sh" "$tmp/b.sh" "$tmp/c.sh" "$tmp/d.sh")
st=$?
set -e
[ "$st" = 1 ] || die "落ちたものがあるのに exit $st"
grep -A3 "^== $tmp/a.sh" <<<"$out" | tail -3 | tr '\n' ' ' | grep -qx 'a1 a2 a3 ' || die "a.sh の出力がまとまっていない: $out"
grep -A3 "^== $tmp/b.sh" <<<"$out" | tail -3 | tr '\n' ' ' | grep -qx 'b1 b2 b3 ' || die "b.sh の出力がまとまっていない: $out"
grep -q "^== $tmp/c.sh（FAIL exit 3" <<<"$out" || die "c.sh の FAIL が出ていない"
grep -qx "| $tmp/d.sh | skip | - |" <<<"$out" || die "d.sh が skip と出ていない"
[ ! -e "$tmp/ran" ] || die "skip した d.sh が走った"
grep -q "| $tmp/c.sh | FAIL(3) |" "$tmp/summary" || die "GITHUB_STEP_SUMMARY に表が無い"
# 流す指定（ANDON_TEST_STREAM）: 溜めずに「[パス] 」付きで流し、終わったときは見出しだけ。落ちたときの exit も伝わる
printf 'echo s1; sleep 0.2; echo s2; exit 4\n' >"$tmp/s.sh"
set +e
out=$(ANDON_TEST_STREAM="$tmp/s.sh" bash tests/lib/run-tests.sh "$tmp/s.sh" "$tmp/a.sh")
st=$?
set -e
[ "$st" = 1 ] || die "流す s.sh が落ちたのに exit $st"
grep -qx "\[$tmp/s.sh\] s1" <<<"$out" || die "s.sh の出力が流れていない: $out"
grep -q "^== $tmp/s.sh（FAIL exit 4" <<<"$out" || die "s.sh の exit 4 が伝わっていない"
grep -qx 's1' <<<"$out" && die "流した出力が見出しの下に重ねて出た"
grep -qx 'a1' <<<"$out" || die "流さない a.sh がまとまって出ていない"
ok "run-tests: ANDON_TEST_STREAM は溜めずに印付きで流し、exit を伝える"
# command の出力を読む検査（ANDON_TEST_EMIT_READER）: 同じ回の emitters に ANDON_EMIT_DIR を渡し、終わってから読み手を
# ANDON_EMITTED で起動する。読み手は emitters の1度目の実行が写したものと終了コード・出力を読める。渡していない emitter の結果は無い
em="$tmp/em"; mkdir "$em"
q=$(printf %q "$em")
printf 'if [ "${1:-}" = --emitters ]; then printf "%%s\\n" %s/e1.sh %s/e2.sh %s/e3.sh; exit 0; fi\n' "$q" "$q" "$q" >"$em/r.sh"
cat >>"$em/r.sh" <<'EOF'
echo "emitted=${ANDON_EMITTED:+set}"
[ -n "${ANDON_EMITTED:-}" ] || exit 0
echo "out1=$(cat "$ANDON_EMITTED/out/e1.json")"
echo "st1=$(cat "$ANDON_EMITTED/e1.status") st2=$(cat "$ANDON_EMITTED/e2.status")"
echo "log1=$(cat "$ANDON_EMITTED/e1.log")"
[ -e "$ANDON_EMITTED/e3.status" ] && echo "e3=ran" || echo "e3=none"
EOF
printf 'echo e1-log; sleep 0.3; echo e1-out >"$ANDON_EMIT_DIR/e1.json"\n' >"$em/e1.sh"
printf 'echo e2-log; exit 5\n' >"$em/e2.sh"
printf 'touch %s/e3-ran\n' "$q" >"$em/e3.sh"
printf 'echo "a-emit=${ANDON_EMIT_DIR:-none}"\n' >"$em/a.sh"
set +e
out=$(ANDON_TEST_JOBS=4 ANDON_TEST_EMIT_READER="$em/r.sh" GITHUB_ACTIONS=false \
  bash tests/lib/run-tests.sh "$em/r.sh" "$em/e1.sh" "$em/e2.sh" "$em/a.sh")
st=$?
set -e
[ "$st" = 1 ] || die "落ちた e2.sh があるのに exit $st"
grep -qx 'emitted=set' <<<"$out" || die "読み手に ANDON_EMITTED が渡らない: $out"
grep -qx 'out1=e1-out' <<<"$out" || die "読み手が e1.sh の写したものを読めない（e1.sh を待たずに起動した）: $out"
grep -qx 'st1=0 st2=5' <<<"$out" || die "emitters の終了コードが置かれていない: $out"
grep -qx 'log1=e1-log' <<<"$out" || die "emitters の出力が置かれていない: $out"
grep -qx 'e3=none' <<<"$out" || die "渡していない e3.sh の結果がある: $out"
[ ! -e "$em/e3-ran" ] || die "渡していない e3.sh を run-tests が走らせた"
grep -qx 'a-emit=none' <<<"$out" || die "emitters でない a.sh に ANDON_EMIT_DIR が渡った: $out"
[ "$(grep -n "^== $em/e1.sh" <<<"$out" | cut -d: -f1)" -lt "$(grep -n "^== $em/r.sh" <<<"$out" | cut -d: -f1)" ] || die "読み手が e1.sh より先に終わった: $out"
grep -q "^| $em/r.sh | ok |" <<<"$out" || die "読み手が表に ok と出ない: $out"
# emitters を1本も渡さない回は、読み手に ANDON_EMITTED を渡さない（読み手が自分で走らせる）
out=$(ANDON_TEST_EMIT_READER="$em/r.sh" GITHUB_ACTIONS=false bash tests/lib/run-tests.sh "$em/r.sh" "$em/a.sh")
grep -qx 'emitted=' <<<"$out" || die "emitters が無い回に ANDON_EMITTED が渡った: $out"
# 既定の読み手は tests/schema.sh で、emitters の並びをそこから読む
[ "$(bash tests/schema.sh --emitters | grep -c '^tests/andon-.*\.sh$')" -ge 9 ] || die "tests/schema.sh --emitters が emitters を出さない"
while IFS= read -r e; do [ -f "$e" ] || die "tests/schema.sh --emitters の $e が無い"; grep -q 'ANDON_EMIT_DIR' "$e" || die "$e が ANDON_EMIT_DIR に写さない"; done < <(bash tests/schema.sh --emitters)
ok "run-tests: 読み手（tests/schema.sh）は同じ回の emitters が写したものを読み、回し直さない。emitters を待ってから起動する"
ok "run-tests: 出力を1本ずつまとめ、所要時間の表・skip・落ちたときの exit 1"

# 3. run: 同時に走るのは1本だけ。2本目は鍵（flock）が外れるまで理由を1行出して待ち、1本目が終わってから走る。
#    鍵を持つのは検査を待つ runner（tests/run が exec した run-tests.sh）で、親だけが落ちて鍵が外れることは無い。
#    runner は鍵を検査に渡さないので、検査が裏に残したプロセスが生きていても次の回は走る。写しで本物の run-tests.sh に偽の検査を走らせる
r="$tmp/run"
mkdir -p "$r/tests/lib"
cp tests/run "$r/tests/run"
cp tests/lib/run-tests.sh "$r/tests/lib/run-tests.sh"
cat >"$r/tests/x.sh" <<'EOF'
d=$RUN_DIR
echo "start $RUN_ID" >>"$d/log"
echo "$PPID" >"$d/runner-$RUN_ID"
sleep 30 </dev/null >/dev/null 2>&1 &
echo $! >"$d/daemon-$RUN_ID"
for _ in $(seq 600); do [ -e "$d/release-$RUN_ID" ] && break; sleep 0.05; done
echo "end $RUN_ID" >>"$d/log"
exit "${RUN_EXIT:-0}"
EOF
trap 'kill $(cat "$r"/daemon-* 2>/dev/null) 2>/dev/null; rm -rf "$tmp"' EXIT
waitfor() { local _; for _ in $(seq 200); do "$@" && return 0; sleep 0.05; done; return 1; } # 10 秒まで
b_waits_or_runs() { grep -qs '走っている。終わるまで待つ' "$r/b.out" || grep -qsx 'start b' "$r/log"; }
# ANDON_BIN: run-tests が並べる前に mise install を打たないように
run=(env ANDON_TEST_LOCK="$r/lock" RUN_DIR="$r" ANDON_BIN=unused GITHUB_ACTIONS=false bash "$r/tests/run")
RUN_ID=a "${run[@]}" >"$r/a.out" 2>&1 &
pa=$!
waitfor grep -qsx 'start a' "$r/log" || die "run: 1本目が検査を起動しない: $(cat "$r/a.out")"
[ "$(cat "$r/runner-a")" = "$pa" ] || die "run: 検査を待つ runner（pid $(cat "$r/runner-a")）が tests/run（pid $pa）でない（親だけが落ちると鍵が外れる）"
[ "$(cat "$r/lock")" = "pid $pa $r" ] || die "run: 鍵のファイルに鍵を持つ回の控えが無い: $(cat "$r/lock")"
RUN_ID=b RUN_EXIT=3 "${run[@]}" >"$r/b.out" 2>&1 &
pb=$!
waitfor b_waits_or_runs || die "run: 2本目が待ちも走りもしない: $(cat "$r/b.out")"
sleep 0.3
! grep -qx 'start b' "$r/log" || die "run: 1本目が走っている間に2本目が走った（鍵を取っていない）: $(cat "$r/log")"
[ "$(grep -c . "$r/b.out")" = 1 ] || die "run: 待つときの理由が1行でない: $(cat "$r/b.out")"
grep -qF "tests/run: ほかの tests/run（pid $pa " "$r/b.out" || die "run: 待つ理由に鍵を持つ回の pid が無い: $(cat "$r/b.out")"
touch "$r/release-a"
wait "$pa" || die "run: 1本目が 0 で終わらない: $(cat "$r/a.out")"
kill -0 "$(cat "$r/daemon-a")" || die "run: 前提が崩れた: 1本目の検査が裏に残したプロセスが生きていない（鍵を渡さないことを確かめられない）"
waitfor grep -qx 'start b' "$r/log" || die "run: 1本目が終わっても（裏のプロセスが生きていると）2本目が走らない: $(cat "$r/log")"
[ "$(cat "$r/log")" = "$(printf 'start a\nend a\nstart b')" ] || die "run: 1本目が終わる前に2本目が走った: $(cat "$r/log")"
touch "$r/release-b"
set +e
wait "$pb"
st=$?
set -e
[ "$st" = 1 ] || die "run: 検査が落ちたのに run-tests の終了コード 1 を伝えない（exit $st）"
grep -qF '| tests/x.sh | FAIL(3) |' "$r/b.out" || die "run: 2本目の表に検査の FAIL(3) が無い: $(cat "$r/b.out")"
ok "run: 2本目は理由を1行出して1本目が終わるまで待つ。鍵は検査を待つ runner が持ち、検査に渡さない。終了コードは run-tests のまま"

# 4. isolation: 番兵に数千のファイルを書かれても、出力はフォルダごとの件数のまとめで1画面に収まり、全部の差分はファイルに残る。
#    まとめに隠れずに、別の置き場への小さな書き込みと読んだファイルも出る。go test を偽の go に替えて、tests/isolation を回す。
#    偽の go は、mise の shim が番兵の HOME へ go を入れ直した形（2000 のファイル）・60 のフォルダへの書き込み・
#    mise と同じ上のフォルダ（XDG_DATA_HOME）の別の置き場への1ファイル・セルの repo の根の1ファイルの書き換え・番兵の1ファイルの読みをする。
#    偽の mise は go の場所だけを答え、偽の kanban は何もしない。
#    CI の形（GITHUB_ACTIONS=true）で回す: 一時の置き場は job と一緒に消えるので、全部の差分を畳んだ group でログにも出す。
#    GITHUB_STEP_SUMMARY は外す（入れ子の run-tests.sh が、在りもしない検査の表を job の summary に足さないように）
iso=$tmp/iso
mkdir -p "$iso/bin" "$iso/tmp"
cat >"$iso/bin/go" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  env) echo /nonexistent ;;
  test)
    d=$HOME/.local/share/mise/installs/go/9.9.9/src
    mkdir -p "$d" && (cd "$d" && seq 2000 | sed 's/$/.go/' | xargs touch)
    for i in $(seq 60); do mkdir -p "$HOME/.cache/many/d$i" && : >"$HOME/.cache/many/d$i/f"; done
    mkdir -p "$XDG_DATA_HOME/andon" && echo '{}' >"$XDG_DATA_HOME/andon/state.json"
    echo x >>"$ANDON_WORKSPACE/AGENTS.local.md"
    cat "$HOME/.claude/CLAUDE.md" >/dev/null ;;
esac
EOF
printf '#!/usr/bin/env bash\n[ "$1 $2" = "which go" ] || exit 1\necho "$(dirname "$0")/go"\n' >"$iso/bin/mise"
printf '#!/usr/bin/env bash\nexit 0\n' >"$iso/bin/kanban"
chmod +x "$iso/bin/go" "$iso/bin/mise" "$iso/bin/kanban"
set +e
env -u GITHUB_STEP_SUMMARY PATH="$iso/bin:$PATH" TMPDIR="$iso/tmp" GITHUB_ACTIONS=true \
  bash tests/isolation tests/no-such-test.sh >"$iso/out" 2>&1
st=$?
set -e
# seen は畳んだ group の外（ログを開いて目に入る行）。まとめの確かめは seen だけを見る（group の中の全部の差分に当てない）。
# die で出すのは seen だけで、入れ子の run-tests.sh の ::error の行も外す（ci.sh が落ちたときに、在りもしない検査の注記を CI に出さない）
awk '/^::group::/ { g = 1; next } /^::endgroup::/ { g = 0; next } !g' "$iso/out" >"$iso/seen"
outside() { grep -v '^::' "$iso/seen" | head -c 3000; }
inside() { awk '/^::group::全部の差分/ { g = 1; next } /^::endgroup::/ { g = 0 } g' "$iso/out"; }
if grep -q 'atime が動かない' "$iso/seen"; then
  echo "skip isolation のまとめ（一時の置き場の atime が動かない。$(grep 'atime が動かない' "$iso/seen")）"
else
  [ "$st" = 1 ] || die "isolation: 番兵に書いても 1 で終わらない（exit $st）: $(outside)"
  lines=$(wc -l <"$iso/seen")
  [ "$lines" -le 60 ] || die "isolation: 番兵に 2000 のファイルを書かれた出力が、畳んだ group の外に $lines 行（60 行を超え、どこに書いたかが1画面で分からない）: $(outside)"
  grep -qF 'FAIL 検査が番兵（セルの本物の代わり）に書いた: ' "$iso/seen" || die "isolation: 書いたことの見出しが無い: $(outside)"
  # 60 のフォルダは 40 か所を超えるので、そのフォルダだけを1つ上（./home/.cache/many）にまとめる。
  # ほかの置き場は細かいまま出す（mise と同じ上のフォルダの andon の1ファイルが、mise の塊に溶けない）。「ほか」にも隠さない
  grep -q '\./home/\.cache/many/d[0-9]' "$iso/seen" && die "isolation: か所の多いフォルダを1つ上にまとめない: $(outside)"
  grep -q 'ほか .* か所' "$iso/seen" && die "isolation: まとめれば出せるか所を「ほか」に隠した: $(outside)"
  n=$(sed -nE 's|^  増えた +([0-9]+) .*  \./home/\.local/share/mise$|\1|p' "$iso/seen")
  [ -n "$n" ] && [ "$n" -ge 2000 ] || die "isolation: mise の置き場（./home/.local/share/mise）に増えた数が出ない（$n）: $(outside)"
  n=$(sed -nE 's|^  増えた +([0-9]+) .*  \./home/\.cache/many$|\1|p' "$iso/seen")
  [ -n "$n" ] && [ "$n" -ge 120 ] || die "isolation: 60 のフォルダ（./home/.cache/many）に増えた数が出ない（$n）: $(outside)"
  grep -qE '^  増えた +2  消えた +0  変わった +0  \./home/\.local/share/andon$' "$iso/seen" || die "isolation: mise と同じ上のフォルダの別の置き場への1ファイル（./home/.local/share/andon）が、mise の塊に溶けた: $(outside)"
  grep -qE '^  増えた +0  消えた +0  変わった +1  \./AGENTS\.local\.md$' "$iso/seen" || die "isolation: 1ファイルの書き換え（./AGENTS.local.md）がまとめに隠れた: $(outside)"
  grep -qxF '  ./home/.claude/CLAUDE.md' "$iso/seen" || die "isolation: 読んだファイルが出ない: $(outside)"
  keep=$(sed -n 's/^  全部の差分: //p' "$iso/seen")
  [ -n "$keep" ] && [ -s "$keep" ] || die "isolation: 全部の差分を残したファイルのパスが出ないか、ファイルが無い（$keep）: $(outside)"
  [ "$(sed -n 's/^  全部: //p' "$iso/seen")" = "$keep" ] || die "isolation: 読んだファイルの全部を、差分と同じファイルに残さない: $(outside)"
  [ "$(grep -cE '^> .*/installs/go/9\.9\.9/src/[0-9]+\.go( |$)' "$keep")" -ge 4000 ] || die "isolation: 残したファイルに全部の差分（2000 のファイルの stat と sha256）が無い: $(head -c 2000 "$keep")"
  grep -qxF './home/.claude/CLAUDE.md' "$keep" || die "isolation: 残したファイルに読んだファイルが無い"
  # CI では一時の置き場が job と一緒に消えるので、全部を畳んだ group でログにも出す。まとめは group の後ろ（ログの終わり）に出す
  [ "$(inside | grep -cE '^> .*/installs/go/9\.9\.9/src/[0-9]+\.go( |$)')" -ge 4000 ] && inside | grep -qxF './home/.claude/CLAUDE.md' \
    || die "isolation: CI（GITHUB_ACTIONS=true）で、全部の差分と読んだファイルを畳んだ group でログに出さない: $(outside)"
  [ "$(grep -n '^::endgroup::' "$iso/out" | tail -1 | cut -d: -f1)" -lt "$(grep -n '^FAIL 検査が番兵（セルの本物の代わり）に書いた: ' "$iso/out" | cut -d: -f1)" ] \
    || die "isolation: CI で、まとめが全部の差分の group より前にある（ログの終わりに出ない）: $(outside)"
  ok "isolation: 番兵に数千のファイルを書かれても、フォルダごとの件数のまとめは $lines 行。小さな書き込みと読んだファイルも出し、全部の差分はファイル（CI では畳んだ group にも）に残す"
fi
