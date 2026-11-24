#!/usr/bin/env bash
# 検証の skill（.claude/skills/verify-andon/）の形（ADR-0035 決めたこと 4・5、受け入れ条件 5・6）。文書だけの PR でも走る（tests/lib/scope.sh のいつもの分）。
#   1. SKILL.md は名指されたときだけ読まれる設定で、Codex 向けの置き場（.agents/skills/verify-andon）が同じ中身を指す
#   2. 目次（features/README.md の表）と features/*.md と scripts/journeys/*.sh が1対1
#   3. 機能のファイルの節の形と、「変えたら動かすパス」が repo に在ること
#      3b. 細かい機能の見るファイルが、変えたら動かすパスの下のテストでないファイルを全部名指し、internal/・cmd/・package/ の下は
#          地図か目次の「地図が覆わないもの」に入る（製品の変更が足したファイルや package を、同じ変更で地図に載せさせる）
#   4. 手順が名指す command の実在（andon の葉は andon help --all の表と照らす。tests/lib/andon-refs.sh で、tests/commands.sh と同じ読み方。
#      scripts/・tests/ の名前・repo のパス）
#      手順が drive.sh を名指すときは、いつも検証する commit を添える（省ける書き方を残さない）
#   5. scripts/features.sh: 当たる範囲で機能の名前、当たらない範囲（文書・Go のテストだけ）で何も出さず 0
#   6. 自分の確かめ: 地図のファイルを1つ消すか、無い command を名指した写しで、この検査が落ちる
#   7. 記録を読む道（dispatcher-news・boot・worker-launch・gate の gt-answer）: 偽のセル（パッケージの置き場・HOME・hc・kanban）で、証拠が足りないときに 0 で終わらない。
#      drive.sh は検証する commit を省けず、省く・空・短い・16進数でないと 2 で断り、道を動かさない（検証の途中で次の commit が当たっても、新しい間を読まない）
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
  # 節を変数で受けてから grep する（pipefail の下で grep -q が先に終わると、長い節の sed が SIGPIPE で落ちる）
  run_sec=$(sed -n '/^## 動かし方$/,/^## /p' "$f")
  grep -q '^前提:$' <<<"$run_sec" || die "$f の動かし方が「前提:」で始まらない"
  grep -qF "scripts/drive.sh $name <検証する commit>" <<<"$run_sec" || die "$f の動かし方が drive.sh $name <検証する commit> を名指さない"
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

# 3b. 見るファイルと地図の外。製品の変更がファイルや package を足したとき、検証の課題が地図の穴を見つける前に、同じ変更で地図を直させる
#   - 細かい機能の行ごとに「見るファイル:」の行があり、名指すファイルは repo に在り、同じ機能の「変えたら動かすパス」の下にある。
#     置き場（<dir>/）で名指せるのは Go のファイルを持たない置き場だけ（Go のファイルは1つずつ書く。足した .go が名指しなしで通らない）
#   - 「変えたら動かすパス」の下のテストでないファイルは、どれかの細かい機能の見るファイルに入る
#   - internal/・cmd/・package/ の下のテストでないファイルは、どれかの機能の「変えたら動かすパス」か、目次の「地図が覆わないもの」に入る
#   - 「地図が覆わないもの」はどの細かい機能の道も通らないもの。機能のパスの下にあっても先に効き（features.sh も機能を返さない）、
#     見るファイルに名指さない。機能のパスを丸ごと外す置き場は書かない
# パスとテストの読み方は scripts/features.sh と同じ（hits。テストでないファイルは *_test.go・testdata/・internal/ の下で親の package の名前に test を付けた package〈internal/ledger/ledgertest/〉を除いたもの）
inpart() {
  case "$1" in
    */) [[ "$2" == "$1"* ]] ;;
    *'*'*) [[ "$2" == $1 && "${2%/*}" == "${1%/*}" ]] ;;
    *) [[ "$2" == "$1" ]] ;;
  esac
}
tracked=$(git -C "$root" -c core.quotepath=off ls-files -- internal cmd package schema process init image | grep -vE '_test\.go$|(^|/)testdata/' | grep -vE '^internal/([^/]+/)*([a-z]+)/\2test/' || true)
[ "$(grep -c '^internal/.*\.go$' <<<"$tracked")" -gt 100 ] || die "git ls-files から internal/ の Go のファイルが読めない（下の確かめが何も見ない）"
allparts=() seeall=()
for name in $files; do
  f=$skill/features/$name.md
  mapfile -t parts < <(sed -n '/^## 変えたら動かすパス$/,/^## /p' "$f" | sed -n 's/^- `\([^`]*\)`.*/\1/p')
  allparts+=("${parts[@]}")
  sub=$(sed -n '/^## 細かい機能$/,/^## /p' "$f")
  lack=$(awk '/^- `/ { line = $0; if ((getline nx) <= 0 || nx !~ /^  - 見るファイル: `/) { print line; exit } }' <<<"$sub")
  [ -z "$lack" ] || die "$f の細かい機能に、すぐ下に「  - 見るファイル: 」の行の無いものがある: $lack"
  mapfile -t see < <(grep '^  - 見るファイル: ' <<<"$sub" | grep -oE '`[^`]+`' | tr -d '`')
  [ "${#see[@]}" -gt 0 ] || die "$f の見るファイルが読めない"
  for e in "${see[@]}"; do
    case "$e" in
      */) [ -d "$root/$e" ] || die "$f の見るファイル $e が repo に無い"
        ! awk -v e="$e" 'index($0, e) == 1 && /\.go$/ { found = 1; exit } END { exit !found }' <<<"$tracked" || die "$f の見るファイル $e は Go のファイルを持つ置き場（Go のファイルは1つずつ書く）" ;;
      *'*'*) die "$f の見るファイル $e に * がある（ファイルか、Go のファイルを持たない置き場で書く）" ;;
      *) [ -f "$root/$e" ] || die "$f の見るファイル $e が repo に無い" ;;
    esac
    under=false
    for p in "${parts[@]}"; do
      if inpart "$p" "$e"; then under=true; break; fi # 置き場の e も、p と同じか p の下だけ（p より広い置き場は外のファイルまで名指す）
    done
    [ "$under" = true ] || die "$f の見るファイル $e が、同じ機能の変えたら動かすパスの下に無い（features.sh が、$e の差分でこの機能を選ばない）"
  done
  seeall+=("${see[@]}")
done
outside=$(sed -n '/^## 地図が覆わないもの$/,/^## /p' "$idx" | grep -oE '^- (`[^`]+`・)*`[^`]+`' | grep -oE '`[^`]+`' | tr -d '`' || true)
[ -n "$outside" ] || die "目次の「地図が覆わないもの」に置き場の行が読めない"
for e in $outside; do
  [ -e "$root/$e" ] || die "目次の「地図が覆わないもの」の $e が repo に無い"
  for p in "${allparts[@]}"; do
    ! inpart "$e" "$p" || die "目次の「地図が覆わないもの」の $e が、機能の変えたら動かすパス $p を丸ごと外す"
  done
done
# 「地図が覆わないもの」は機能のパスより先に効く（features.sh もその差分で機能を返さない）。どの細かい機能の道も通らないファイルを、名指しだけで覆ったことにしない
nseen=0 nall=0 nout=0
while IFS= read -r t; do
  [ -n "$t" ] || continue
  out=false
  for e in $outside; do
    if inpart "$e" "$t"; then out=true; break; fi
  done
  if [ "$out" = true ]; then
    for e in "${seeall[@]}"; do
      ! inpart "$e" "$t" || die "$t が、目次の「地図が覆わないもの」に入るのに、細かい機能の見るファイル $e に名指されている（どちらかに決める）"
    done
    nout=$((nout + 1)) nall=$((nall + 1))
    continue
  fi
  mapped=false
  for p in "${allparts[@]}"; do
    if inpart "$p" "$t"; then mapped=true; break; fi
  done
  if [ "$mapped" = true ]; then
    named=false
    for e in "${seeall[@]}"; do
      if inpart "$e" "$t"; then named=true; break; fi
    done
    [ "$named" = true ] || die "地図の変えたら動かすパスの下の $t が、どの細かい機能の見るファイルにも無い（それを動かす細かい機能の「見るファイル:」に足すか、細かい機能を足す）"
    nseen=$((nseen + 1))
  else
    case "$t" in internal/* | cmd/* | package/*) ;; *) continue ;; esac
    die "$t が地図の外（どの機能の変えたら動かすパスにも、目次の「地図が覆わないもの」にも無い。機能のファイルの「変えたら動かすパス」と細かい機能の「見るファイル:」に足すか、覆わない理由を目次に書く）"
  fi
  nall=$((nall + 1))
done <<<"$tracked"
[ "$nseen" -gt 100 ] || die "見るファイルの確かめが $nseen 件しか見ていない（読み方が壊れている）"
[ "$nout" -gt 50 ] || die "「地図が覆わないもの」に入るファイルが $nout 件しか読めない（読み方が壊れている）"
ok "verify-andon: 変えたら動かすパスの下のファイル $nseen 件はどれも細かい機能の見るファイルに入り、internal/・cmd/・package/ の下は地図か「地図が覆わないもの」（$nout 件）に入る（$nall 件）"

# 手順が drive.sh を名指すときは、いつも検証する commit を添える（省いて打つと 2 で断られる。省ける書き方を写させない）
uses=$(cat "$skill/SKILL.md" "$skill"/features/*.md | grep -oE 'scripts/drive\.sh [^`|]*' || true)
[ "$(grep -c . <<<"$uses")" -gt 10 ] || die "手順が drive.sh を名指す所が $(grep -c . <<<"$uses") 件しか読めない（読み方が壊れている）"
bare=$(grep -vxE 'scripts/drive\.sh (<機能>|[a-z0-9-]+) <検証する commit>' <<<"$uses" | sed -n 1p || true)
[ -z "$bare" ] || die "手順が drive.sh を検証する commit なしで名指す: $bare"
ok "verify-andon: 手順が drive.sh を名指す $(grep -c . <<<"$uses") か所は、どれも検証する commit を添える"

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
  c5=$(put internal/digest/x.go x)
  c6=$(put internal/ledger/ask.go x)
  c7=$(put internal/spawn/x.go x)
  c8=$(put internal/delegate/x.go x)
  feat() { (cd "$repo" && bash "$root/$skill/scripts/features.sh" "$1"); }
  [ "$(feat "$c0..$c1")" = gate ] || die "features.sh: internal/gate/ の差分で gate だけにならない: $(feat "$c0..$c1" | tr '\n' ' ')"
  got=$(feat "$c0..$c2" | tr '\n' ' ')
  [ "$got" = "dispatcher-news gate hooks " ] || die "features.sh: internal/gate/ と package/hooks/ の差分で dispatcher-news gate hooks にならない: $got"
  [ "$(feat "$c4..$c5")" = digest ] || die "features.sh: internal/digest/ の差分で digest だけにならない: $(feat "$c4..$c5" | tr '\n' ' ')"
  got=$(feat "$c5..$c6" | tr '\n' ' ')
  [ "$got" = "digest measure report-ready waiting " ] || die "features.sh: internal/ledger/ask.go の差分で digest measure report-ready waiting にならない: $got"
  got=$(feat "$c6..$c7" | tr '\n' ' ')
  [[ " $got" == *" create "* ]] || die "features.sh: internal/spawn/ の差分で create を返さない: $got"
  got=$(feat "$c7..$c8" | tr '\n' ' ')
  [[ " $got" == *" create "* ]] || die "features.sh: internal/delegate/ の差分で create を返さない: $got"
  out=$(feat "$c2..$c4") || die "features.sh: 当たらない範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: 文書と Go のテストだけの差分で $out を出す"
  c9=$(put internal/gate/testdata/x.diff x)
  c10=$(put internal/ledger/ledgertest/x.go x)
  out=$(feat "$c8..$c10") || die "features.sh: testdata/ とテスト用の package だけの範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: testdata/ とテスト用の package（ledgertest）だけの差分で $out を出す"
  c11=$(put internal/gate/latest/x.go x)
  [ "$(feat "$c10..$c11")" = gate ] || die "features.sh: 名前が test で終わるだけの置き場（internal/gate/latest/）の差分で gate を返さない: $(feat "$c10..$c11" | tr '\n' ' ')"
  # git は ASCII でないパスを既定で "…\346…" と引用符で包んで出す。包んだまま読むと、どの機能にも当たらず黙って抜ける
  c12=$(put internal/gate/新しい.go x)
  [ "$(feat "$c11..$c12")" = gate ] || die "features.sh: ASCII でない名前のファイル（internal/gate/新しい.go）の差分で gate を返さない: $(feat "$c11..$c12" | tr '\n' ' ')"
  # 目次の「地図が覆わないもの」は機能のパス（cmd/kanban/）より先に効く。kanban migrate を通らない report-ready を選ばない
  c13=$(put cmd/kanban/migrate.go x)
  out=$(feat "$c12..$c13") || die "features.sh: 地図が覆わないものだけの範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: 地図が覆わないもの（cmd/kanban/migrate.go）だけの差分で $out を出す"
  c14=$(put cmd/kanban/main.go x)
  [ "$(feat "$c13..$c14")" = report-ready ] || die "features.sh: cmd/kanban/main.go の差分で report-ready だけにならない: $(feat "$c13..$c14" | tr '\n' ' ')"
  # dispatcher が巡ごとに呼ぶ生の記録の片付け（internal/otel/prune.go）は dispatcher-news とハーネスの受け口（otel）を選び、
  # boot と役の起動が起動する受け口（serve.go）は otel だけを選ぶ
  c15=$(put internal/otel/prune.go x)
  [ "$(feat "$c14..$c15" | tr '\n' ' ')" = "dispatcher-news otel " ] || die "features.sh: internal/otel/prune.go の差分で dispatcher-news と otel にならない: $(feat "$c14..$c15" | tr '\n' ' ')"
  c16=$(put internal/otel/serve.go x)
  [ "$(feat "$c15..$c16")" = otel ] || die "features.sh: internal/otel/serve.go の差分で otel だけにならない: $(feat "$c15..$c16" | tr '\n' ' ')"
  rc=0; (cd "$repo" && bash "$root/$skill/scripts/features.sh") 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "features.sh: 範囲が無いのに終了コード $rc（2 でない）"
  rc=0; bash "$skill/scripts/drive.sh" no-such-feature 0123456789abcdef0123456789abcdef01234567 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "drive.sh: 知らない機能で終了コード $rc（2 でない）"
  ok "verify-andon: features.sh は当たる範囲で機能の名前を出し、文書と Go のテスト（testdata/・テスト用の package を含む）だけなら何も出さず 0。drive.sh は知らない機能を 2 で断る"
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
  cp -r "$skill" "$tmp/f" && sed -i 's/drive\.sh <機能> <検証する commit>$/drive.sh <機能> [<検証する commit>]/' "$tmp/f/SKILL.md"
  broke 'drive.sh を検証する commit なしで名指す' "$tmp/f"
  # 見るファイル（3b）: 製品の変更がファイルを足して細かい機能に名指さない・地図の外に package を足す・Go の置き場を丸ごと名指す・
  # 見るファイルの行を落とす・同じ機能の変えたら動かすパスの外を名指す
  cp -r "$skill" "$tmp/g" && sed -i 's/`internal\/dispatcher\/idle\.go`、//' "$tmp/g/features/dispatcher-news.md"
  ! grep -qF 'internal/dispatcher/idle.go' "$tmp/g/features/dispatcher-news.md" || die "写し g から idle.go の名指しを外せない（自分の確かめが何も見ない）"
  broke 'internal/dispatcher/idle.go が、どの細かい機能の見るファイルにも無い' "$tmp/g"
  cp -r "$skill" "$tmp/h" && sed -i '/^- `internal\/schema\/`/d' "$tmp/h/features/README.md"
  broke 'internal/schema/schema.go が地図の外' "$tmp/h"
  cp -r "$skill" "$tmp/i" && sed -i 's/見るファイル: `internal\/gate\/gate\.go`/見るファイル: `internal\/gate\/`/' "$tmp/i/features/gate.md"
  broke '見るファイル internal/gate/ は Go のファイルを持つ置き場' "$tmp/i"
  cp -r "$skill" "$tmp/j" && sed -i '0,/^  - 見るファイル: /{/^  - 見るファイル: /d}' "$tmp/j/features/measure.md"
  broke '見るファイル: 」の行の無いものがある: - `ms-flow`' "$tmp/j"
  cp -r "$skill" "$tmp/k" && sed -i '/^  - 見るファイル: /s/`package\/gate\.json`$/`package\/settings.json`/' "$tmp/k/features/gate.md"
  broke '見るファイル package/settings.json が、同じ機能の変えたら動かすパスの下に無い' "$tmp/k"
  cp -r "$skill" "$tmp/l" && sed -i '/^  - 見るファイル: /s/`package\/hooks\/`、`package\/settings\.json`/`package\/`、`package\/settings.json`/' "$tmp/l/features/hooks.md"
  grep -qF '見るファイル: `package/`、' "$tmp/l/features/hooks.md" || die "写し l の hk-wired を package/ に替えられない（自分の確かめが何も見ない）"
  broke '見るファイル package/ が、同じ機能の変えたら動かすパスの下に無い' "$tmp/l"
  # 道が通らないファイル（kanban migrate）を見るファイルに名指して覆ったことにする・覆わないもので機能のパスを丸ごと外す
  cp -r "$skill" "$tmp/m" && sed -i '/^  - 見るファイル: /s/`cmd\/kanban\/main\.go`/`cmd\/kanban\/main.go`、`cmd\/kanban\/migrate.go`/' "$tmp/m/features/report-ready.md"
  grep -qF '`cmd/kanban/migrate.go`' "$tmp/m/features/report-ready.md" || die "写し m の rr-ready に migrate.go を名指せない（自分の確かめが何も見ない）"
  broke 'cmd/kanban/migrate.go が、目次の「地図が覆わないもの」に入るのに、細かい機能の見るファイル cmd/kanban/migrate.go に名指されている' "$tmp/m"
  cp -r "$skill" "$tmp/n" && sed -i 's/^- `internal\/schema\/`/- `internal\/gate\/`・`internal\/schema\/`/' "$tmp/n/features/README.md"
  broke '目次の「地図が覆わないもの」の internal/gate/ が、機能の変えたら動かすパス internal/gate/ を丸ごと外す' "$tmp/n"
  ok "verify-andon: 地図のファイルを消す・無い command を名指す・無いパスを書く・設定を外す・drive.sh の commit を省ける書き方にする・足したファイルを見るファイルに名指さない・地図の外に package を置く・見るファイルの書き方を誤ると、この検査が落ちる"
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

  # 検証の途中で次の commit が当たった: 検証する commit（VERIFY_COMMIT）が当たっていた間だけを、次に当てた時刻まで読む
  lib=$c/ws/projects/v/kanban/issues/V-0002/library; mkdir -p "$lib"
  launch() { echo "{\"at\": \"$1\", \"issue\": \"V-0002\", \"role\": \"worker\", \"terminal\": \"t$2\", \"package\": {\"commit\": \"$2\", \"source\": \"applied\", \"drift\": false, \"root\": \"$c/pkg\"}}"; }
  { launch 2026-01-01T00:02:00Z c0ffee; launch 2026-01-01T00:12:00Z beef02; } >"$lib/launches.jsonl"
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x",\n "previous": [{"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z"}]}\n' >"$c/pkg/applied.json"
  VERIFY_COMMIT=c0f journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" && grep -qF '次に当てた時刻 2026-01-01T00:10:00Z' <<<"$jout" \
    || die "worker-launch: 前に当てた c0ffee の間（次に当てた 00:10 まで）だけを読まない（$jrc）: $jout"
  VERIFY_COMMIT=beef02 journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" || die "worker-launch: 今当たっている beef02 の起動だけを読まない（$jrc）: $jout"
  journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" || die "worker-launch: 検証する commit を省くと今当たっているものを読まない（$jrc）: $jout"
  # 今の状態を読む行（dn-live・bt-ran）は、検証する commit でなく最後に当てた時刻と比べる
  VERIFY_COMMIT=c0ffee journey dispatcher-news
  [ "$jrc" = 1 ] && grep -qF 'dn-live' <<<"$jout" || die "dispatcher-news: dispatcher が最後に当てる前から動いているのに、前の commit の検証で落ちない（$jrc）: $jout"
  links worker retro; VERIFY_COMMIT=c0ffee journey boot
  [ "$jrc" = 1 ] && grep -qF 'FAIL bt-ran' <<<"$jout" || die "boot: dispatcher が最後に当てる前から動いているのに、前の commit の検証で落ちない（$jrc）: $jout"
  sed -i "s/\"created_ms\": [0-9]*/\"created_ms\": $(($(date -u -d 2026-01-01T00:11:00Z +%s) * 1000))/" "$c/dispatcher.json"
  VERIFY_COMMIT=c0ffee journey dispatcher-news
  [ "$jrc" = 0 ] || die "dispatcher-news: c0ffee の間に届けた通知があり dispatcher も最後に当てたあとに起動したのに終了コード $jrc: $jout"
  # 同じ commit を続けて当てた: 最初に当てた時刻から読む（c0ffee を 00:00 と 00:05 に当て、beef02 を 00:10 に当てた）
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x",\n "previous": [{"commit": "c0ffee", "applied_at": "2026-01-01T00:05:00Z"}, {"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z"}]}\n' >"$c/pkg/applied.json"
  VERIFY_COMMIT=c0ffee journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" && grep -qF '当てた時刻 2026-01-01T00:00:00Z' <<<"$jout" \
    || die "worker-launch: c0ffee を続けて2回当てたのに、最初に当てた 00:00 から読まない（$jrc）: $jout"
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:11:00Z", "image_commit": "x",\n "previous": [{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z"}, {"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z"}]}\n' >"$c/pkg/applied.json"
  journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" || die "worker-launch: 今の beef02 を続けて2回当てたのに、最初に当てた 00:10 から読まない（$jrc）: $jout"
  # 当ててから前の dispatcher が見回りを終えるまで（00:10〜00:13）に届けた通知は、前の c0ffee の dispatcher が書いた。
  # dispatcher の記録は切り替わった時刻（switched_at）で区切り、起動の記録は当てた時刻で区切る
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x", "switched_at": "2026-01-01T00:13:00Z",\n "previous": [{"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z", "switched_at": "2026-01-01T00:01:00Z"}]}\n' >"$c/pkg/applied.json"
  echo '{"at": "2026-01-01T00:12:00Z", "project": "v", "acked": true, "news": [{"issue": "V-0001", "kind": "review"}]}' >"$c/state/andon/dispatcher/v.delivered.jsonl"
  VERIFY_COMMIT=c0ffee journey dispatcher-news
  [ "$jrc" = 0 ] && grep -qF '届けた通知 1 件' <<<"$jout" || die "dispatcher-news: 前の dispatcher が見回りを終えるまでに届けた通知を c0ffee の間に読まない（$jrc）: $jout"
  journey dispatcher-news
  [ "$jrc" = 3 ] || die "dispatcher-news: beef02 の dispatcher が切り替わる前の通知を beef02 の間に読む（$jrc。3 でない）: $jout"
  VERIFY_COMMIT=c0ffee journey worker-launch
  [ "$jrc" = 0 ] && grep -qF '当てたあとの起動の記録 1 件' <<<"$jout" || die "worker-launch: 起動の記録を当てた時刻でなく切り替わった時刻で区切る（$jrc）: $jout"
  # 前のバージョンの記録（previous が無い）と、記録に無い commit: 間が決まらないので未確認の 3。落ちない
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x"}\n' >"$c/pkg/applied.json"
  VERIFY_COMMIT=c0ffee journey worker-launch
  [ "$jrc" = 3 ] && grep -qF 'c0ffee を当てた記録が無い' <<<"$jout" || die "worker-launch: previous の無い記録で前の commit を 3 で断らない（$jrc）: $jout"
  journey worker-launch
  [ "$jrc" = 0 ] || die "worker-launch: previous の無い記録で今当たっている commit を読めない（$jrc）: $jout"
  VERIFY_COMMIT=dead01 journey worker-launch
  [ "$jrc" = 3 ] || die "worker-launch: 記録に無い commit で終了コード $jrc（3 でない）: $jout"
  ok "verify-andon: 記録を読む道は、検証する commit が当たっていた間を次に当てた時刻まで読み、前のバージョンの記録や記録に無い commit は未確認の 3 で終わる"

  # gate の gt-answer: dispatcher が閉じた問いは gate の問いだけで、「決め直した」の答えが名指す gate の記録が在る。
  # 偽のセルに本物の kanban の CLI で kanban を作り、dispatcher の端末として問いを出して答えて受け取る（今の時刻なので、上の beef02 の間に入る）
  . tests/lib/kanban.sh
  g=$c/ws/projects/g head=$(printf 'a%.0s' {1..40})
  gsetup() { # gsetup <問いの from> <答え>: kanban を作り直し、gate の記録 1 つと、dispatcher が閉じた問い 1 つを置く（from が空なら問いを置かない）
    rm -rf "$g" && mkdir -p "$g" && (cd "$g" && "$kanban" init --prefix G >/dev/null) || die "gt-answer の場面の kanban が作れない"
    local id lib
    id=$(KANBAN_ROOT=$g HC_TERMINAL_ID=tc "$kanban" add "gate の問い")
    lib=$g/kanban/issues/$id/library && mkdir -p "$lib/gate" "$lib/questions"
    echo "{\"at\": \"2026-01-01T00:12:00Z\", \"id\": \"$id\", \"pr\": \"https://github.com/example/sample-site/pull/7\", \"head\": \"$head\", \"tier\": \"C\", \"decision\": \"needs_human\", \"merged\": false, \"reasons\": []}" >"$lib/gate/$head.json"
    [ -n "$1" ] || return 0
    KANBAN_ROOT=$g HC_TERMINAL_ID=dispatcher "$kanban" ask "$id" - --kind line <<<"マージするか" >/dev/null
    echo "{\"from\": \"$1\"}" >"$lib/questions/1.json"
    KANBAN_ROOT=$g HC_TERMINAL_ID=dispatcher "$kanban" answer --decided-by machine -- "$id#1" "${2//<lib>/$lib}" >/dev/null
    KANBAN_ROOT=$g HC_TERMINAL_ID=dispatcher "$kanban" received "$id#1" >/dev/null
  }
  gsetup gate "PR の head が動いた（bbbbbbbbbbbb → aaaaaaaaaaaa）。gate が今の head で決め直した（記録 <lib>/gate/$head.json）"
  journey gate
  [ "$jrc" = 0 ] && grep -qF 'ok   gt-answer' <<<"$jout" || die "gate: dispatcher が gate の問いを記録つきで閉じたのに gt-answer が ok でない（$jrc）: $jout"
  gsetup gate "PR の head が動いた（bbbbbbbbbbbb → cccccccccccc）。gate が今の head で決め直した（記録 <lib>/gate/cccc.json）"
  journey gate
  [ "$jrc" = 1 ] && grep -qF 'FAIL gt-answer' <<<"$jout" && grep -qF 'gate の記録' <<<"$jout" || die "gate: 答えが名指す gate の記録が無いのに落ちない（$jrc）: $jout"
  gsetup chief-of-staff "Chief of Staff の問いを dispatcher が閉じた"
  journey gate
  [ "$jrc" = 1 ] && grep -qF 'gate の外の問い' <<<"$jout" || die "gate: dispatcher が gate の外の問いを閉じたのに落ちない（$jrc）: $jout"
  gsetup "" ""
  journey gate
  [ "$jrc" = 0 ] && grep -qF 'note gt-answer' <<<"$jout" || die "gate: dispatcher が閉じた問いが無いのに note gt-answer が出ない（$jrc）: $jout"
  ok "verify-andon: gate の gt-answer は、gate の外の問いを閉じたか、答えが名指す gate の記録が無いと落ち、閉じた問いが無ければ note を出す"

  # drive.sh は検証する commit を省けない。検証の途中で次の commit が当たったあと、省いて打つと新しい commit の間を読み、
  # 検証する commit の出来事を見ずに未確認や誤りになっていた。偽のセルで、drive.sh を利用者と同じく通しで打つ
  old=c0ffee0000000000000000000000000000000000 new=beef020000000000000000000000000000000000
  printf '{"commit": "%s", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x",\n "previous": [{"commit": "%s", "applied_at": "2026-01-01T00:00:00Z"}]}\n' "$new" "$old" >"$c/pkg/applied.json"
  { launch 2026-01-01T00:02:00Z "$old"; launch 2026-01-01T00:12:00Z "$new"; } >"$lib/launches.jsonl"
  drive() { # drive <引数…> → 終了コードを $drc に、出力（標準エラーも）を $dout に
    drc=0; dout=$(env PATH="$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" TMPDIR="$c" \
      bash "$skill/scripts/drive.sh" "$@" 2>&1) || drc=$?
  }
  refused() { # refused <何が違うか> <drive.sh の引数…>: 使い方の誤り 2 で断り、道を動かさない（当てた記録を読む行が出ない）
    local what=$1; shift
    drive "$@"
    [ "$drc" = 2 ] && grep -qF 'drive.sh <機能> <検証する commit>' <<<"$dout" || die "drive.sh: 検証する commit が${what}のに使い方の誤り 2 で断らない（$drc）: $dout"
    ! grep -qF '当てたあとの起動の記録' <<<"$dout" || die "drive.sh: 検証する commit が${what}のに道を動かした: $dout"
  }
  refused 無い worker-launch
  refused 空な worker-launch ""
  refused "6 文字で短い" worker-launch c0ffee
  refused 大文字な worker-launch C0FFEE0000
  refused "16進数でない" worker-launch 今当たっているもの
  refused "あとに余分な引数がある" worker-launch "$old" extra
  drive worker-launch "${old:0:12}"
  [ "$drc" = 0 ] && grep -qF "検証する commit $old・当てた時刻 2026-01-01T00:00:00Z・次に当てた時刻 2026-01-01T00:10:00Z" <<<"$dout" \
    && grep -qF '当てたあとの起動の記録 1 件' <<<"$dout" && grep -qF 'ok   片付けた' <<<"$dout" \
    || die "drive.sh: 前に当てた $old を渡したのに、その間（次に当てた 00:10 まで）だけを読んで片付けない（$drc）: $dout"
  drive worker-launch "$new"
  [ "$drc" = 0 ] && grep -qF "検証する commit $new・当てた時刻 2026-01-01T00:10:00Z・次に当てた時刻 無し" <<<"$dout" \
    && grep -qF '当てたあとの起動の記録 1 件' <<<"$dout" || die "drive.sh: 今当たっている $new を渡したのに、その間を読まない（$drc）: $dout"
  ok "verify-andon: drive.sh は検証する commit を省く・空・短い・16進数でない・余分な引数を 2 で断って道を動かさず、渡した commit が当たっていた間だけを読む"
fi
