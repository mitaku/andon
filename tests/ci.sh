#!/usr/bin/env bash
# CI の tests の job が使う道具（tests/lib/image-scope.sh・tests/lib/run-tests.sh）の振る舞い。
set -euo pipefail
cd "$(dirname "$0")/.."
die() { echo "FAIL ci: $*" >&2; exit 1; }
ok() { echo "ok   $*"; }

# 1. image-scope: image に入らず tests/image.sh も読まない変更だけなら false。知らないパスは true
scope() { printf '%s\n' "$@" | bash tests/lib/image-scope.sh; }
[ "$(scope docs/image.md README.md AGENTS.md REVIEW.md SECURITY.md LICENSE)" = false ] || die "人の文書だけで true"
[ "$(scope tests/andon-task.sh tests/kanban/test-deck.mjs workflow_test.go internal/task/task_test.go)" = false ] || die "ほかの tests・Go のテストだけで true"
[ "$(scope)" = false ] || die "変更が無いのに true"
for p in image/Dockerfile image/build.sh package/skills/x/SKILL.md schema/report.schema.json cmd/andon/main.go \
  internal/task/task.go init/x go.mod go.sum embed.go .dockerignore mise.toml .github/workflows/tests.yml \
  tests/image.sh tests/lib/pty-probe.py tests/lib/run-tests.sh tests/lib/image-scope.sh docs-new/x NEW.md; do
  [ "$(scope docs/image.md "$p")" = true ] || die "$p を含むのに false"
done
ok "image-scope: 文書・ほかの tests だけなら飛ばし、image に入るもの・知らないパスでは走らせる"

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
ok "run-tests: 出力を1本ずつまとめ、所要時間の表・skip・落ちたときの exit 1"
