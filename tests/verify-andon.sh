#!/usr/bin/env bash
# 検証の skill（.claude/skills/verify-andon/）の形（ADR-0035 決めたこと 4・5、受け入れ条件 5・6）。文書だけの PR でも走る（tests/lib/scope.sh のいつもの分）。
#   1. SKILL.md は名指されたときだけ読まれる設定で、Codex 向けの置き場（.agents/skills/verify-andon）が同じ中身を指す
#   2. 目次（features/README.md の表）と features/*.md と scripts/journeys/*.sh が1対1
#   3. 機能のファイルの節の形と、「変えたら動かすパス」が repo に在ること
#   4. 手順が名指す command の実在（andon の葉は andon help --all の表と照らす。tests/lib/andon-refs.sh で、tests/commands.sh と同じ読み方。
#      scripts/・tests/ の名前・repo のパス）
#   5. scripts/features.sh: 当たる範囲で機能の名前、当たらない範囲（文書・Go のテストだけ）で何も出さず 0
#   6. 自分の確かめ: 地図のファイルを1つ消すか、無い command を名指した写しで、この検査が落ちる
#   7. 記録を読む道（dispatcher-news・boot）: 偽のセル（パッケージの置き場・HOME・hc）で、証拠が足りないときに 0 で終わらない
# 使い方: bash tests/verify-andon.sh [--skill <skill の置き場>]（--skill は 6 の写しを見るときだけ。andon は tests/lib/andon.sh が用意し、写しの回にも渡す）
set -euo pipefail
cd "$(dirname "$0")/.."
root=$PWD
skill=.claude/skills/verify-andon self=true
if [ "${1:-}" = --skill ]; then skill=${2:?}; self=false; fi
die() { echo "FAIL verify-andon: $*" >&2; exit 1; }
ok() { echo "ok   $*"; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/andon.sh
. tests/lib/andon-refs.sh
export ANDON_BIN=$andon # 6 の写しの回が焼き直さない

# 1. 名指されたときだけ読まれる・Codex 向けの symlink
[ -f "$skill/SKILL.md" ] || die "$skill/SKILL.md が無い"
fm=$(awk 'NR == 1 && $0 != "---" { exit } NR > 1 && $0 == "---" { exit } NR > 1' "$skill/SKILL.md")
grep -qx 'name: verify-andon' <<<"$fm" || die "SKILL.md の frontmatter に name: verify-andon が無い"
grep -qx 'disable-model-invocation: true' <<<"$fm" || die "SKILL.md が名指されたときだけ読まれる設定（disable-model-invocation: true）でない"
grep -q '^description: ..' <<<"$fm" || die "SKILL.md の frontmatter に description が無い"
if [ "$self" = true ]; then
  [ -L .agents/skills/verify-andon ] || die ".agents/skills/verify-andon が symlink でない（ADR-0004）"
  [ "$(readlink .agents/skills/verify-andon)" = ../../.claude/skills/verify-andon ] || die ".agents/skills/verify-andon が ../../.claude/skills/verify-andon を指さない"
  [ -f .agents/skills/verify-andon/SKILL.md ] || die ".agents/skills/verify-andon から SKILL.md に届かない"
fi
ok "verify-andon: SKILL.md は名指されたときだけ読まれ、Codex 向けの置き場は symlink で同じ中身を指す"

# 2. 目次 ↔ 機能のファイル ↔ 道
idx=$skill/features/README.md
[ -f "$idx" ] || die "目次 $idx が無い"
listed=$(grep -oE '^\| [^|]+ \| \[[a-z0-9-]+\.md\]\(\./[a-z0-9-]+\.md\)' "$idx" | sed -E 's/.*\(\.\/([a-z0-9-]+)\.md\)$/\1/' | sort)
files=$(find "$skill/features" -maxdepth 1 -name '*.md' ! -name README.md -printf '%f\n' | sed 's/\.md$//' | sort)
journeys=$(find "$skill/scripts/journeys" -maxdepth 1 -name '*.sh' -printf '%f\n' | sed 's/\.sh$//' | sort)
[ -n "$listed" ] || die "目次の表に機能の行が無い"
[ "$listed" = "$files" ] || die "目次の表と features/*.md が食い違う: 表だけ [$(comm -23 <(echo "$listed") <(echo "$files") | tr '\n' ' ')] ファイルだけ [$(comm -13 <(echo "$listed") <(echo "$files") | tr '\n' ' ')]"
[ "$files" = "$journeys" ] || die "features/*.md と scripts/journeys/*.sh が食い違う: 地図だけ [$(comm -23 <(echo "$files") <(echo "$journeys") | tr '\n' ' ')] 道だけ [$(comm -13 <(echo "$files") <(echo "$journeys") | tr '\n' ' ')]"
ok "verify-andon: 目次の表・features/*.md・scripts/journeys/*.sh が1対1（$(wc -w <<<"$files") 機能）"

# 3. 節の形と、変えたら動かすパスの実在
want='細かい機能
辿り着き方（利用者から）
動かし方
変えたら動かすパス
既に見ている検査
落とし穴'
for name in $files; do
  f=$skill/features/$name.md
  head -1 "$f" | grep -q '^# ' || die "$f が H1 で始まらない"
  [ "$(grep '^## ' "$f" | sed 's/^## //')" = "$want" ] || die "$f の H2 が「$(tr '\n' '・' <<<"$want")」の順でない"
  sed -n '/^## 動かし方$/,/^## /p' "$f" | grep -q '^前提:$' || die "$f の動かし方が「前提:」で始まらない"
  sed -n '/^## 動かし方$/,/^## /p' "$f" | grep -qF "scripts/drive.sh $name" || die "$f の動かし方が drive.sh $name を名指さない"
  sed -n '/^## 細かい機能$/,/^## /p' "$f" | grep '^- `' | grep -vqE '見る道: (検証の担当だけ|Chief of Staff が試しの課題を起動し、検証の担当が画面を読む|オーナーが見る)。$' \
    && die "$f の細かい機能に、見る道（3つのどれか）の無い行がある"
  mapfile -t parts < <(sed -n '/^## 変えたら動かすパス$/,/^## /p' "$f" | sed -n 's/^- `\([^`]*\)`.*/\1/p')
  [ "${#parts[@]}" -gt 0 ] || die "$f の「変えたら動かすパス」が空"
  for p in "${parts[@]}"; do
    case "$p" in
      */) [ -d "$root/$p" ] || die "$f の変えたら動かすパス $p が repo に無い" ;;
      *'*'*) compgen -G "$root/$p" >/dev/null || die "$f の変えたら動かすパス $p に当たるファイルが無い" ;;
      *) [ -e "$root/$p" ] || die "$f の変えたら動かすパス $p が repo に無い" ;;
    esac
  done
done
ok "verify-andon: 機能のファイルは6つの節をこの順に持ち、変えたら動かすパスが repo に在る"

# 4. 手順が名指す command の実在
andon_leaves >"$tmp/leaves"
grep -qx 'issue record' "$tmp/leaves" || die "andon help --all から葉が読めない（下の確かめが何も見ない）"
andon_refs md "$skill/SKILL.md" "$skill"/features/*.md >"$tmp/refs"
bad=$(andon_bad_refs "$tmp/leaves" <"$tmp/refs" | sed -n 1p)
[ -z "$bad" ] || die "手順が名指す ${bad#*: } が無い（${bad%%: andon *}）"
n=$(wc -l <"$tmp/refs")
# `` で囲んだ語と、コードの囲み（```）の中の行
spans=$( {
  cat "$skill/SKILL.md" "$skill"/features/*.md | grep -v '^```' | grep -oE '`[^`]+`' | tr -d '`'
  cat "$skill/SKILL.md" "$skill"/features/*.md | awk '/^```/ { inside = !inside; next } inside'
} | sort -u)
while IFS= read -r s; do
  read -r -a w <<<"$s"
  for t in "${w[@]}"; do
    case "$t" in
      *'<'* | *'*'*) ;;
      scripts/*) [ -e "$skill/$t" ] || die "手順が名指す $t が $skill に無い" ; n=$((n + 1)) ;;
      .claude/skills/verify-andon/*) [ -e "$skill/${t#.claude/skills/verify-andon/}" ] || die "手順が名指す $t が無い"; n=$((n + 1)) ;;
      tests/*.sh | image/*.sh | docs/*.md | process/* | package/* | internal/* | cmd/* | schema/*)
        [ -e "$root/${t%%[),。]*}" ] || die "手順が名指す $t が repo に無い"; n=$((n + 1)) ;;
    esac
  done
done <<<"$spans"
for s in "$skill"/scripts/*.sh "$skill"/scripts/journeys/*.sh; do
  bash -n "$s" || die "$s が bash として読めない"
done
for s in features.sh doctor.sh drive.sh; do [ -x "$skill/scripts/$s" ] || die "scripts/$s が実行できない"; done
for s in "$skill"/scripts/journeys/*.sh; do [ -x "$s" ] || die "$s が実行できない"; done
[ "$n" -gt 10 ] || die "名指しの確かめが $n 件しか見ていない（読み方が壊れている）"
ok "verify-andon: 手順が名指す command・スクリプト・パス $n 件が在り、スクリプトは bash として読める"

# 5. features.sh: 当たる範囲・当たらない範囲（文書・Go のテストだけ）。drive.sh は知らない機能を 2 で断る
if [ "$self" = true ]; then
  repo=$tmp/repo
  export GIT_CONFIG_GLOBAL=$tmp/gitconfig GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
  git init -q "$repo"
  put() { mkdir -p "$repo/$(dirname "$1")" && echo "$2" >"$repo/$1" && git -C "$repo" add -A && git -C "$repo" commit -qm "$1" && git -C "$repo" rev-parse HEAD; }
  c0=$(put README.md base)
  c1=$(put internal/gate/x.go x)
  c2=$(put package/hooks/stop x)
  c3=$(put docs/x.md x)
  c4=$(put internal/gate/x_test.go x)
  feat() { (cd "$repo" && bash "$root/$skill/scripts/features.sh" "$1"); }
  [ "$(feat "$c0..$c1")" = gate ] || die "features.sh: internal/gate/ の差分で gate だけにならない: $(feat "$c0..$c1" | tr '\n' ' ')"
  got=$(feat "$c0..$c2" | tr '\n' ' ')
  [ "$got" = "dispatcher-news gate hooks " ] || die "features.sh: internal/gate/ と package/hooks/ の差分で dispatcher-news gate hooks にならない: $got"
  out=$(feat "$c2..$c4") || die "features.sh: 当たらない範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: 文書と Go のテストだけの差分で $out を出す"
  rc=0; (cd "$repo" && bash "$root/$skill/scripts/features.sh") 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "features.sh: 範囲が無いのに終了コード $rc（2 でない）"
  rc=0; bash "$skill/scripts/drive.sh" no-such-feature 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "drive.sh: 知らない機能で終了コード $rc（2 でない）"
  ok "verify-andon: features.sh は当たる範囲で機能の名前を出し、文書と Go のテストだけなら何も出さず 0。drive.sh は知らない機能を 2 で断る"
fi

# 6. 自分の確かめ: 写しを壊すと落ちる（上の確かめが何も見ていないことを避ける）
if [ "$self" = true ]; then
  broke() { local want=$1 rc=0 err; shift; err=$(bash "$0" --skill "$@" 2>&1 >/dev/null) || rc=$?; [ "$rc" = 1 ] && grep -qF -- "$want" <<<"$err" || die "壊した写しで落ちない（want: $want, rc $rc）: $err"; }
  cp -r "$skill" "$tmp/a" && rm "$tmp/a/features/gate.md"
  broke '目次の表と features/*.md が食い違う' "$tmp/a"
  cp -r "$skill" "$tmp/b" && printf '\n`andon no-such-command`\n' >>"$tmp/b/SKILL.md"
  broke 'andon no-such-command が無い' "$tmp/b"
  cp -r "$skill" "$tmp/c" && sed -i 's/andon issue record <自分の課題>/andon issue recrd <自分の課題>/' "$tmp/c/SKILL.md"
  broke 'andon issue recrd が無い' "$tmp/c"
  cp -r "$skill" "$tmp/d" && sed -i 's|^- `internal/gate/`$|- `internal/no-such/`|' "$tmp/d/features/gate.md"
  broke 'internal/no-such/ が repo に無い' "$tmp/d"
  cp -r "$skill" "$tmp/e" && sed -i 's/^disable-model-invocation: true$//' "$tmp/e/SKILL.md"
  broke 'disable-model-invocation' "$tmp/e"
  ok "verify-andon: 地図のファイルを消す・無い command を名指す・無いパスを書く・設定を外すと、この検査が落ちる"
fi

# 7. 記録を読む道: 偽のセルで、証拠が足りないときに 0 で終わらない（本物の /opt/andon・HOME・hc には触らない）
if [ "$self" = true ]; then
  c=$tmp/cell; mkdir -p "$c/pkg/bin" "$c/pkg/hooks" "$c/pkg/skills/worker" "$c/pkg/skills/retro" "$c/bin" "$c/home/.agents/skills" "$c/state/andon/dispatcher" "$c/ws"
  echo '{"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z"}' >"$c/pkg/applied.json"
  touch "$c/pkg/skills/worker/SKILL.md" "$c/pkg/skills/retro/SKILL.md"
  ms=$(($(date -u -d 2026-01-01T00:01:00Z +%s) * 1000))
  echo "{\"state\": \"live\", \"argv\": [\"$c/pkg/bin/andon\"], \"created_ms\": $ms, \"labels\": {}}" >"$c/dispatcher.json"
  printf '#!/bin/sh\n[ "$1 $2" = "get dispatcher" ] && cat "%s/dispatcher.json"\n' "$c" >"$c/bin/hc"; chmod +x "$c/bin/hc"
  echo x >"$c/gen"; echo "{\"$c/gen\": \"$(sha256sum "$c/gen" | cut -d' ' -f1)\"}" >"$c/state/andon/boot.json"
  echo "{\"hooks\": {\"Stop\": [{\"hooks\": [{\"command\": \"$c/pkg/hooks/stop\"}]}]}}" >"$c/managed.json"
  links() { rm -f "$c/home/.agents/skills"/andon-*; for n in "$@"; do ln -s "$c/pkg/skills/$n" "$c/home/.agents/skills/andon-$n"; done; }
  journey() { # journey <道> → 終了コードを $jrc に、出力を $jout に
    jrc=0; jout=$(env PATH="$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" \
      ANDON_MANAGED_SETTINGS="$c/managed.json" VERIFY_TMP="$c" VERIFY_RUN=verify-selftest bash "$skill/scripts/journeys/$1.sh" 2>&1) || jrc=$?
  }
  at='"at": "2026-01-01T00:02:00Z"'
  echo "{$at, \"issue\": \"V-0001\", \"kind\": \"review\"}" >"$c/state/andon/dispatcher/v.recorded.jsonl"
  journey dispatcher-news
  [ "$jrc" = 3 ] || die "dispatcher-news: 記録だけの通知で終了コード $jrc（3 でない）: $jout"
  echo "{$at, \"project\": \"v\", \"acked\": true, \"news\": [{\"issue\": \"V-0001\", \"kind\": \"review\"}]}" >"$c/state/andon/dispatcher/v.delivered.jsonl"
  journey dispatcher-news
  [ "$jrc" = 0 ] || die "dispatcher-news: 届けた通知があるのに終了コード $jrc: $jout"
  links worker retro; journey boot
  [ "$jrc" = 0 ] || die "boot: 揃った偽のセルで終了コード $jrc: $jout"
  links; journey boot
  [ "$jrc" = 1 ] && grep -qF 'andon-worker が' <<<"$jout" || die "boot: ~/.agents/skills の andon-* が無いのに落ちない（$jrc）: $jout"
  links worker; ln -s "$c/ws" "$c/home/.agents/skills/andon-retro"; journey boot
  [ "$jrc" = 1 ] && grep -qF 'andon-retro が' <<<"$jout" || die "boot: andon-retro がパッケージの外を指すのに落ちない（$jrc）: $jout"
  links worker retro; ln -s "$c/ws" "$c/home/.agents/skills/andon-extra"; journey boot
  [ "$jrc" = 1 ] && grep -qF 'andon-extra がパッケージの外' <<<"$jout" || die "boot: 余分な andon-extra がパッケージの外を指すのに落ちない（$jrc）: $jout"
  ok "verify-andon: 記録を読む道は、記録だけの通知なら未確認の 3 で終わり、skill のリンクが無い・パッケージの外を指すと落ちる"
fi
