#!/usr/bin/env bash
# 検証の skill（.claude/skills/verify-andon/）の形（ADR-0035 決めたこと 4・5、受け入れ条件 5・6）。文書だけの PR でも走る（tests/lib/scope.sh のいつもの分）。
#   1. SKILL.md は名指されたときだけ読まれる設定で、Codex 向けの置き場（.agents/skills/verify-andon）が同じ中身を指す
#   2. 目次（features/README.md の表）と features/*.md と scripts/journeys/*.sh が1対1
#   3. 機能のファイルの節の形と、「変えたら動かすパス」が repo に在ること
#      3b. 細かい機能の見るファイルが、変えたら動かすパスの下のテストでないファイルを全部名指し、internal/・cmd/・package/ の下は
#          地図か目次の「地図が覆わないもの」に入る（製品の変更が足したファイルや package を、同じ変更で地図に載せさせる）
#      3c. 見るファイルを変えた変更は、変えたファイルを見る細かい機能を commit の本文の Verify-features: の行で名指し、名前は地図に在る
#          （比べる base は CI が ANDON_VERIFY_BASE で渡す。手元は origin/main との分かれ目。どちらも無ければ飛ばす）。
#          Go のファイルでコメントだけを変えたもの（scripts/gocode.go で字句の並びが同じ。//go: の指令と cgo の前置きは字句に数える）は名指しを求めない
#   4. 手順が名指す command の実在（andon の葉は andon help --all の表と照らす。tests/lib/andon-refs.sh で、tests/commands.sh と同じ読み方。
#      scripts/・tests/ の名前・repo のパス）
#      手順が drive.sh を名指すときは、いつも検証する commit を添える（省ける書き方を残さない）
#   5. scripts/features.sh: 当たる範囲で機能の名前、当たらない範囲（文書・Go のテスト・Go のコメントだけ）で何も出さず 0
#   6. 自分の確かめ: 地図のファイルを1つ消すか、無い command を名指した写しで、この検査が落ちる
#   7. 記録を読む道（dispatcher-news・boot・worker-launch〈wl-resume を含む〉・gate の gt-answer）: 偽のセル（パッケージの置き場・HOME・hc・kanban・Claude Code のセッションの記録）で、証拠が足りないときに 0 で終わらない。
#      drive.sh は検証する commit を省けず、省く・空・短い・16進数でないと 2 で断り、道を動かさない（検証の途中で次の commit が当たっても、新しい間を読まない）
#      drive.sh は細かい機能ごとの結果を1行ずつ出し、3つ目の引数で名指した細かい機能（見る道が検証の担当だけのもの）だけで 0 と 3 を決める。FAIL は名指しによらず 1
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
verify_base=${ANDON_VERIFY_BASE:-} # 3c の比べる base（tests/lib/andon.sh が環境を切り離す前に読む）
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

# 3c. 変更ごとの名指し。見るファイルを変えた変更は、その commit の本文の Verify-features: の行で、変えたファイルを見る細かい機能を名指す。
#   見るファイルに載っているだけでは、変えた振る舞いを通る細かい機能があるとは限らない（載ったファイルの新しい振る舞いを、どの細かい機能も
#   通らないまま当たる）。変えた側が名指せなければ、同じ変更で細かい機能を足す
#   - 変えたファイル（テストでないもの）が細かい機能の見るファイルに入るなら、それを見る細かい機能の1つ以上が名指されている
#   - 名指した名前は、どれも地図の細かい機能にある（同じ変更で足した細かい機能も、同じ変更で消した細かい機能も数える）
#   - 見るファイルは HEAD と base の地図の両方から引く（同じ変更で地図から外したファイル・消したファイルも名指しを求める）
# 名指しの行は、比べる base から HEAD までの commit の本文のどこにあってもよい（後から足すなら、行だけの commit を積む）
# see_pairs: 機能のファイル（stdin）から「細かい機能<TAB>見るファイル」を1行ずつ出す
see_pairs() {
  sed -n '/^## 細かい機能$/,/^## /p' | awk '/^- `/ { match($0, /`[^`]+`/); s = substr($0, RSTART + 1, RLENGTH - 2); next } /^  - 見るファイル: / { n = split($0, a, "`"); for (i = 2; i < n; i += 2) print s "\t" a[i] }'
}
declare -A subsee=() oldsee=() # HEAD の地図・base の地図（base は下の本物の確かめで読む）
for name in $files; do
  while IFS=$'\t' read -r s e; do
    subsee[$s]="${subsee[$s]:+${subsee[$s]} }$e"
  done < <(see_pairs <"$skill/features/$name.md")
done
[ "${#subsee[@]}" -gt 30 ] || die "細かい機能と見るファイルの対が ${#subsee[@]} 件しか読めない（読み方が壊れている）"
# named_gaps <変えたファイル（1行1つ）> <commit の本文>: 足りないものを1行ずつ出す（何も出さなければ通る）
named_gaps() {
  local t n s e want covered line
  local -a names=() some
  while IFS= read -r line; do
    if [[ "$line" =~ ^[a-z0-9-]+(,?\ *[a-z0-9-]+)*,?\ *$ ]]; then
      read -ra some <<<"${line//,/ }"
      names+=("${some[@]}")
    else
      echo "Verify-features: の行の書き方が違う（細かい機能の名前をコンマで並べる）: Verify-features: $line"
    fi
  done < <(sed -n 's/^Verify-features:[[:space:]]*//p' <<<"$2")
  for n in "${names[@]}"; do
    [ -n "${subsee[$n]+x}${oldsee[$n]+x}" ] || echo "Verify-features: で名指した $n は、地図の細かい機能に無い"
  done
  while IFS= read -r t; do
    [ -n "$t" ] || continue
    want=() covered=false
    for s in "${!subsee[@]}" "${!oldsee[@]}"; do
      for e in ${subsee[$s]:-} ${oldsee[$s]:-}; do
        if inpart "$e" "$t"; then want+=("$s"); break; fi
      done
    done
    [ "${#want[@]}" -gt 0 ] || continue
    for s in "${want[@]}"; do
      for n in "${names[@]}"; do
        if [[ "$n" == "$s" ]]; then covered=true; fi
      done
    done
    [ "$covered" = true ] || echo "$t を変えたのに、それを見る細かい機能（$(tr ' ' '\n' <<<"${want[*]}" | sort -u | tr '\n' ' ' | sed 's/ $//')）のどれも Verify-features: で名指していない"
  done <<<"$1"
}
# code_files <repo> <base> <head> <変えたファイル（1行1つ）>: Go のファイルのうち、コメントだけを変えたもの（scripts/gocode.go で字句の並びが
#   前後で同じ）を除いて出す。振る舞いを変えるコメント（//go: の指令・cgo の前置きなど）の変更は残す（features.sh と同じ読み方）
code_files() {
  local gofiles code
  gofiles=$(grep -E '\.go$' <<<"$4" | grep -vE '_test\.go$' || true)
  [ -n "$gofiles" ] || { printf '%s\n' "$4"; return; }
  code=$(cd "$root" && go run "$root/$skill/scripts/gocode.go" "$1" "$2" "$3" <<<"$gofiles") || die "scripts/gocode.go が $2..$3 の Go のファイルを読めない"
  { grep -vE '\.go$' <<<"$4" || true; grep -E '_test\.go$' <<<"$4" || true; echo "$code"; } | grep . || true
}
# 名指しの確かめが何も見ていないことを避ける（地図の写しは要らない。読んだ対で確かめる）
gaps=$(named_gaps $'internal/issue/resume.go\ndocs/contract.md' '')
grep -qF 'internal/issue/resume.go を変えたのに' <<<"$gaps" || die "名指しの無い変更で、見るファイルの internal/issue/resume.go を拾わない: $gaps"
! grep -qF 'docs/contract.md' <<<"$gaps" || die "見るファイルでない docs/contract.md に名指しを求める: $gaps"
gaps=$(named_gaps 'internal/issue/resume.go' $'fix\n\nVerify-features: wl-nosuch')
grep -qF '名指した wl-nosuch は、地図の細かい機能に無い' <<<"$gaps" || die "地図に無い名前を拾わない: $gaps"
grep -qF 'internal/issue/resume.go を変えたのに' <<<"$gaps" || die "地図に無い名前だけで、変えたファイルを覆ったことにする: $gaps"
gaps=$(named_gaps 'internal/issue/resume.go' $'fix\n\nVerify-features: gt-route')
grep -qF 'internal/issue/resume.go を変えたのに' <<<"$gaps" || die "変えたファイルを見ない細かい機能の名指しで通る: $gaps"
gaps=$(named_gaps 'internal/issue/resume.go' $'fix\n\nVerify-features: wl-resume の行で名指す')
grep -qF 'Verify-features: の行の書き方が違う' <<<"$gaps" || die "名前でない語の並ぶ行を、書き方の誤りとして拾わない: $gaps"
gaps=$(named_gaps $'internal/issue/resume.go\npackage/skills/worker/SKILL.md\ninternal/issue/resume_test.go' $'a\n\nVerify-features: wl-resume\n---\nb\n\nVerify-features: wl-first-turn, wl-record')
[ -z "$gaps" ] || die "見るファイルを覆う名指し（置き場の見るファイル・複数の commit・コンマで並べた名前）で通らない: $gaps"
# 同じ変更で地図から外した見るファイル（base の地図にだけある）にも名指しを求め、base にだけある名前は名指せる
oldsee=([wl-gone]=internal/issue/gone.go)
gaps=$(named_gaps 'internal/issue/gone.go' '')
grep -qF 'internal/issue/gone.go を変えたのに、それを見る細かい機能（wl-gone）' <<<"$gaps" || die "base の地図にだけある見るファイルの変更に名指しを求めない: $gaps"
gaps=$(named_gaps 'internal/issue/gone.go' $'x\n\nVerify-features: wl-gone')
[ -z "$gaps" ] || die "base の地図にだけある細かい機能を名指せない: $gaps"
oldsee=()
if [ "$self" = true ]; then
  # 比べる base: CI の tests の job は PR の base（merge commit の1つ目の親）を ANDON_VERIFY_BASE で渡す。手元は origin/main との分かれ目
  grep -qE '^ +echo "ANDON_VERIFY_BASE=' .github/workflows/tests.yml || die ".github/workflows/tests.yml の tests の job が ANDON_VERIFY_BASE を渡さない（CI で名指しを見ない）"
  base=$verify_base
  if [ -z "$base" ] && [ "${CI:-}" != true ] && git rev-parse -q --verify origin/main >/dev/null; then
    base=$(git merge-base HEAD origin/main) || die "HEAD と origin/main の分かれ目が読めない（shallow か、関係の無い履歴）。ANDON_VERIFY_BASE で base を渡す"
  fi
  if [ -z "$base" ]; then
    echo "skip verify-andon: 変更ごとの名指し（比べる base が無い。PR の CI は ANDON_VERIFY_BASE で渡す）"
  else
    git rev-parse -q --verify "$base^{commit}" >/dev/null || die "比べる base $base が読めない（CI は履歴を全部取る）"
    changed=$(git -c core.quotepath=off diff --name-only --no-renames "$base" HEAD)
    while IFS= read -r p; do
      while IFS=$'\t' read -r s e; do
        oldsee[$s]="${oldsee[$s]:+${oldsee[$s]} }$e"
      done < <(git show "$base:$p" | see_pairs)
    done < <(git ls-tree --name-only "$base" "$skill/features/" | grep '\.md$' || true)
    code=$(code_files "$root" "$base" HEAD "$changed") # 分けて受ける（引数の中の $(…) の失敗は set -e で止まらない）
    gaps=$(named_gaps "$code" "$(git log --format=%B "$base..HEAD")")
    [ -z "$gaps" ] || die "変更ごとの名指しが足りない（変えた振る舞いを通る細かい機能を commit の本文の「Verify-features: <名前>, …」の行で名指す。名指せなければ、同じ変更で細かい機能を足す。features/README.md の「変更ごとの名指し」）:
$gaps"
    ok "verify-andon: ${base:0:12}..HEAD で変えたファイル $(grep -c . <<<"$changed") 件のうち、細かい機能の見るファイルに入るものは、どれもそれを見る細かい機能が名指されている"
  fi
fi

# 手順が drive.sh を名指すときは、いつも検証する commit を添える（省いて打つと 2 で断られる。省ける書き方を写させない）
uses=$(cat "$skill/SKILL.md" "$skill"/features/*.md | grep -oE 'scripts/drive\.sh [^`|]*' || true)
[ "$(grep -c . <<<"$uses")" -gt 10 ] || die "手順が drive.sh を名指す所が $(grep -c . <<<"$uses") 件しか読めない（読み方が壊れている）"
bare=$(grep -vxE 'scripts/drive\.sh (<機能>|[a-z0-9-]+) <検証する commit>( <名指した細かい機能>)?' <<<"$uses" | sed -n 1p || true)
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
  # CoS の手順（verify.md）は skill を repo の外へ展開して打つ。偽の go は、打った所（試しの repo）の外では落ちる（mise の shim と同じ）
  ext=$tmp/ext && mkdir -p "$ext" && cp -r "$skill" "$ext/"
  realgo=$(cd "$root" && go env GOROOT)/bin/go
  mkdir -p "$tmp/gobin" && printf '#!/usr/bin/env bash\n[ "$PWD" = "%s" ] || { echo "go: 打った所の外で呼ばれた: $PWD" >&2; exit 1; }\nexec "%s" "$@"\n' "$repo" "$realgo" >"$tmp/gobin/go" && chmod +x "$tmp/gobin/go"
  feat() { (cd "$repo" && PATH="$tmp/gobin:$PATH" bash "$ext/verify-andon/scripts/features.sh" "$1"); }
  # gatepkg <範囲>: internal/gate/ の Go のファイル（地図が名指す gate.go と同じ package）の差分で、gate・hold・report-ready を出す
  gatepkg() { [ "$(feat "$1" | tr '\n' ' ')" = "gate hold report-ready " ]; }
  # 試しの repo には go.mod が無く import は辿らない。同じ package（地図が名指す internal/gate/gate.go などと同じフォルダ）の機能は出す
  got=$(feat "$c0..$c1" | tr '\n' ' ')
  [ "$got" = "gate hold report-ready " ] || die "features.sh: internal/gate/ の差分で gate hold report-ready にならない: $got"
  got=$(feat "$c0..$c2" | tr '\n' ' ')
  [ "$got" = "dispatcher-news gate hold hooks report-ready " ] || die "features.sh: internal/gate/ と package/hooks/ の差分で dispatcher-news gate hold hooks report-ready にならない: $got"
  [ "$(feat "$c4..$c5")" = digest ] || die "features.sh: internal/digest/ の差分で digest だけにならない: $(feat "$c4..$c5" | tr '\n' ' ')"
  got=$(feat "$c5..$c6" | tr '\n' ' ')
  [ "$got" = "digest hold measure otel report-ready waiting worker-launch " ] || die "features.sh: internal/ledger/ask.go の差分で digest hold measure otel report-ready waiting worker-launch にならない: $got"
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
  got=$(feat "$c11..$c12" | tr '\n' ' ')
  [[ " $got" == *" gate "* ]] || die "features.sh: ASCII でない名前のファイル（internal/gate/新しい.go）の差分で gate を返さない: $got"
  # 目次の「地図が覆わないもの」は機能のパス（cmd/kanban/）より先に効く。kanban migrate を通らない report-ready を選ばない
  c13=$(put cmd/kanban/migrate.go x)
  out=$(feat "$c12..$c13") || die "features.sh: 地図が覆わないものだけの範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: 地図が覆わないもの（cmd/kanban/migrate.go）だけの差分で $out を出す"
  c14=$(put cmd/kanban/main.go x)
  # cmd/kanban/ は report-ready のパス。同じ package の routines.go（dispatcher-news）と questions.go（waiting）の機能も出す
  got=$(feat "$c13..$c14" | tr '\n' ' ')
  [ "$got" = "dispatcher-news report-ready waiting " ] || die "features.sh: cmd/kanban/main.go の差分で dispatcher-news report-ready waiting にならない: $got"
  # dispatcher が巡ごとに呼ぶ生の記録の片付け（internal/otel/prune.go）は dispatcher-news とハーネスの受け口（otel）を選ぶ。
  # 受け口（serve.go）も同じ package なので、dispatcher-news（prune.go を名指す）を選ぶ
  c15=$(put internal/otel/prune.go x)
  [ "$(feat "$c14..$c15" | tr '\n' ' ')" = "dispatcher-news otel " ] || die "features.sh: internal/otel/prune.go の差分で dispatcher-news と otel にならない: $(feat "$c14..$c15" | tr '\n' ' ')"
  c16=$(put internal/otel/serve.go x)
  [ "$(feat "$c15..$c16" | tr '\n' ' ')" = "dispatcher-news otel " ] || die "features.sh: internal/otel/serve.go の差分で dispatcher-news と otel にならない: $(feat "$c15..$c16" | tr '\n' ' ')"
  # Go のファイルでコメントだけを変えた差分は機能を出さず、名指しも求めない。コードの字句・//go: の指令・cgo の前置き・文字列の中の // を
  # 変えた差分は、今までどおり機能を出し、名指しを求める
  g0=$(put internal/gate/g.go $'package gate\n\n// F は 1 を返す\nfunc F() int { return 1 } // 行の終わり\n\nvar s = "// x"')
  g1=$(put internal/gate/g.go $'package gate\n\n// F は\n// 1 を返す。\n/* 囲み */\nfunc F() int { return 1 }\n\nvar s = "// x"')
  out=$(feat "$g0..$g1") || die "features.sh: Go のコメントだけの範囲で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: Go のコメントだけを変えた差分（internal/gate/g.go）で $out を出す"
  out=$(feat "$g0...$g1") || die "features.sh: Go のコメントだけの範囲（...）で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: Go のコメントだけを変えた差分（... の範囲）で $out を出す"
  g2=$(put internal/gate/g.go $'package gate\n\n// F は\n// 1 を返す。\n/* 囲み */\nfunc F() int { return 2 }\n\nvar s = "// x"')
  gatepkg "$g1..$g2" || die "features.sh: Go のコードの字句を変えた差分で gate を返さない: $(feat "$g1..$g2" | tr '\n' ' ')"
  gatepkg "$g0..$g2" || die "features.sh: Go のコメントとコードを変えた差分で gate を返さない: $(feat "$g0..$g2" | tr '\n' ' ')"
  g3=$(put internal/gate/g.go $'package gate\n\n// F は\n// 1 を返す。\n/* 囲み */\nfunc F() int { return 2 }\n\nvar s = "// y"')
  gatepkg "$g2..$g3" || die "features.sh: Go の文字列の中の // を変えた差分で gate を返さない: $(feat "$g2..$g3" | tr '\n' ' ')"
  g4=$(put internal/gate/g.go $'//go:build linux\n\npackage gate\n\n// F は\n// 1 を返す。\n/* 囲み */\nfunc F() int { return 2 }\n\nvar s = "// y"')
  gatepkg "$g3..$g4" || die "features.sh: Go の //go:build の指令を足した差分で gate を返さない: $(feat "$g3..$g4" | tr '\n' ' ')"
  g7=$(put internal/gate/b.go $'package gate\n\nfunc B() {}')
  g8=$(put internal/gate/b.go $'//  +build ignore\n\npackage gate\n\nfunc B() {}')
  gatepkg "$g7..$g8" || die "features.sh: Go の // と +build の間に空白を2つ置いた指令を足した差分で gate を返さない: $(feat "$g7..$g8" | tr '\n' ' ')"
  g9=$(put internal/gate/b.go $'//\t+build linux\n\npackage gate\n\nfunc B() {}')
  gatepkg "$g8..$g9" || die "features.sh: Go の // と +build の間にタブを置いた指令に変えた差分で gate を返さない: $(feat "$g8..$g9" | tr '\n' ' ')"
  g5=$(put internal/gate/c.go $'package gate\n\n// #include <a.h>\nimport "C"')
  g6=$(put internal/gate/c.go $'package gate\n\n// #include <b.h>\nimport "C"')
  gatepkg "$g5..$g6" || die "features.sh: cgo の前置き（import \"C\" の前のコメント）を変えた差分で gate を返さない: $(feat "$g5..$g6" | tr '\n' ' ')"
  # Go は import "\x43" も import "C" として読む（パスは引用符を外して比べる）
  g10=$(put internal/gate/e.go $'package gate\n\n// #define VALUE 1\nimport "\\x43"')
  g11=$(put internal/gate/e.go $'package gate\n\n// #define VALUE 2\nimport "\\x43"')
  gatepkg "$g10..$g11" || die "features.sh: エスケープで書いた import \"\\x43\" の cgo の前置きを変えた差分で gate を返さない: $(feat "$g10..$g11" | tr '\n' ' ')"
  # 3c の名指し: 見るファイル（internal/issue/resume.go）でコメントだけを変えた変更は名指し無しで通り、コードの字句を変えた変更は名指しを求める
  r0=$(put internal/issue/resume.go $'package issue\n\n// Resume は続ける\nfunc Resume() {}')
  r1=$(put internal/issue/resume.go $'package issue\n\n// Resume は端末で続きを始める。\nfunc Resume() {}')
  r2=$(put internal/issue/resume.go $'package issue\n\n// Resume は端末で続きを始める。\nfunc Resume() { panic(0) }')
  code=$(code_files "$repo" "$r0" "$r1" $'internal/issue/resume.go\ndocs/x.md')
  [ "$code" = docs/x.md ] || die "名指しの確かめ: Go のコメントだけを変えた internal/issue/resume.go を除かない: $code"
  gaps=$(named_gaps "$code" '')
  [ -z "$gaps" ] || die "名指しの確かめ: Go のコメントだけを変えた変更に名指しを求める: $gaps"
  code=$(code_files "$repo" "$r1" "$r2" 'internal/issue/resume.go')
  gaps=$(named_gaps "$code" '')
  grep -qF 'internal/issue/resume.go を変えたのに' <<<"$gaps" || die "名指しの確かめ: Go のコードの字句を変えた変更に名指しを求めない: $gaps"
  code=$(code_files "$repo" "$r0" "$r2" 'internal/issue/resume.go')
  [ "$code" = internal/issue/resume.go ] || die "名指しの確かめ: Go のコメントとコードを変えた internal/issue/resume.go を除く: $code"
  rc=0; (cd "$repo" && bash "$root/$skill/scripts/features.sh") 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "features.sh: 範囲が無いのに終了コード $rc（2 でない）"
  rc=0; bash "$skill/scripts/drive.sh" no-such-feature 0123456789abcdef0123456789abcdef01234567 2>/dev/null || rc=$?
  [ "$rc" = 2 ] || die "drive.sh: 知らない機能で終了コード $rc（2 でない）"
  ok "verify-andon: features.sh は当たる範囲で機能の名前を出し、文書と Go のテスト（testdata/・テスト用の package を含む）と Go のコメントだけなら何も出さず 0。名指しの確かめも Go のコメントだけの変更を除き、コードの字句・指令・cgo の前置きの変更は残す。drive.sh は知らない機能を 2 で断る"
fi

# 5b. features.sh は Go のファイルを package の単位で読む。地図の行から共通の部品（ledger・spawn・delegate）や同じ package の
#     呼び出し元（merge.go・settle.go）が漏れても、地図が名指す package と同じ package か、それが repo の中で推移的に import する
#     package の差分なら、その機能を出す。地図の写しから過去に漏れた行を消し、go.mod と import を持つ試しの repo で確かめる
if [ "$self" = true ]; then
  mrepo=$tmp/mod
  git init -q "$mrepo"
  mput() { mkdir -p "$mrepo/$(dirname "$1")" && printf '%s\n' "$2" >"$mrepo/$1" && git -C "$mrepo" add -A && git -C "$mrepo" commit -qm "$1" && git -C "$mrepo" rev-parse HEAD; }
  mput go.mod $'module example.com/m\n\ngo 1.22' >/dev/null
  mput internal/issue/hold.go $'package issue\n\nfunc Hold() {}' >/dev/null
  mput internal/digest/digest.go $'package digest\n\nimport (\n\t"fmt"\n\n\t"example.com/m/internal/ledger"\n)\n\nvar _ = fmt.Sprint(ledger.X)' >/dev/null
  mput internal/measure/measure.go $'package measure\n\nimport "example.com/m/internal/ledger"\n\nvar _ = ledger.X' >/dev/null
  mput internal/ledger/ledger.go $'package ledger\n\nvar X = 1' >/dev/null
  # create は delegate を import し、delegate と spawn は互いに import する（循環でも止まる。spawn は create から2段先）
  mput internal/create/create.go $'package create\n\nimport "example.com/m/internal/delegate"\n\nvar _ = delegate.D' >/dev/null
  mput internal/delegate/delegate.go $'package delegate\n\nimport "example.com/m/internal/spawn"\n\nvar D = spawn.S' >/dev/null
  mput internal/spawn/spawn.go $'package spawn\n\nimport _ "example.com/m/internal/delegate"\n\nvar S = 1' >/dev/null
  mput internal/triage/triage.go $'package triage\n\nfunc T() {}' >/dev/null
  # 字句に分けられないファイルがあっても、ほかのファイルの import は読む
  m0=$(mput internal/create/broken.go 'package create (')
  m1=$(mput internal/issue/settle.go $'package issue\n\nfunc Settle() {}')
  m2=$(mput internal/ledger/ask.go $'package ledger\n\nfunc Ask() {}')
  m3=$(mput internal/spawn/run.go $'package spawn\n\nfunc Run() {}')
  m4=$(mput internal/triage/sort.go $'package triage\n\nfunc Sort() {}')
  m5=$(mput internal/delegate/cmd.go $'package delegate\n\nfunc Cmd() {}')
  cut=$tmp/cut && mkdir -p "$cut" && cp -r "$skill" "$cut/"
  drop() { # drop <機能> <パス>: 地図の写しの機能から、変えたら動かすパスの行を消す（過去の漏れを写す）
    local f=$cut/verify-andon/features/$1.md
    grep -qxF -- "- \`$2\`" "$f" || die "5b の写し: $1.md の変えたら動かすパスに $2 の行が無い（消す行を見失った）"
    grep -vxF -- "- \`$2\`" "$f" >"$f.new" && mv "$f.new" "$f"
  }
  drop hold internal/issue/merge.go
  drop hold internal/issue/settle.go
  drop digest internal/ledger/
  drop measure internal/ledger/
  drop create internal/spawn/
  drop create internal/delegate/
  mkdir -p "$tmp/mgobin" && printf '#!/usr/bin/env bash\n[ "$PWD" = "%s" ] || { echo "go: 打った所の外で呼ばれた: $PWD" >&2; exit 1; }\nexec "%s" "$@"\n' "$mrepo" "$realgo" >"$tmp/mgobin/go" && chmod +x "$tmp/mgobin/go"
  mfeat() { (cd "$mrepo" && PATH="$tmp/mgobin:$PATH" bash "$cut/verify-andon/scripts/features.sh" "$1") | tr '\n' ' '; }
  got=$(mfeat "$m0..$m1")
  [[ " $got" == *" hold "* ]] || die "features.sh: hold.md が名指す internal/issue/hold.go と同じ package の settle.go の差分で hold を返さない（地図の行から漏れた呼び出し元）: $got"
  got=$(mfeat "$m1..$m2")
  [[ " $got" == *" digest "* && " $got" == *" measure "* ]] || die "features.sh: digest と measure の package が import する internal/ledger/ の差分で digest と measure を返さない（地図の行から漏れた共通の部品）: $got"
  got=$(mfeat "$m2..$m3")
  [[ " $got" == *" create "* ]] || die "features.sh: create の package が2段先で import する internal/spawn/ の差分で create を返さない（推移的な import）: $got"
  # 当たらない package（どの機能の package も import しない internal/triage/ は triage のパスだけ）では、import を辿った機能を出さない
  got=$(mfeat "$m3..$m4")
  [ "$got" = "triage " ] || die "features.sh: どの機能の package も import しない internal/triage/ の差分で triage だけにならない: $got"
  # 「地図が覆わないもの」（internal/delegate/cmd.go）は、package を辿っても先に効く
  out=$(mfeat "$m4..$m5") || die "features.sh: 地図が覆わないものだけの範囲（import のある repo）で 0 で終わらない"
  [ -z "$out" ] || die "features.sh: 地図が覆わないもの（internal/delegate/cmd.go）だけの差分で、同じ package を辿って $out を出す"
  ok "verify-andon: features.sh は Go のファイルを package の単位で読み、地図の行から漏れた同じ package の呼び出し元（settle.go）・共通の部品（ledger）・2段先の import（spawn）の差分でも機能を出し、import されない package と地図が覆わないものでは出さない"
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
  # 偽の hc: get dispatcher は dispatcher.json、read は $c/screen（無ければ空）を出す。ほかは 1
  printf '#!/bin/sh\ncase "$1 $2" in\n"get dispatcher") cat "%s/dispatcher.json" ;;\n"read "*) cat "%s/screen" 2>/dev/null; exit 0 ;;\n*) exit 1 ;;\nesac\n' "$c" "$c" >"$c/bin/hc"; chmod +x "$c/bin/hc"
  echo x >"$c/gen"; echo "{\"$c/gen\": \"$(sha256sum "$c/gen" | cut -d' ' -f1)\"}" >"$c/state/andon/boot.json"
  echo "{\"hooks\": {\"Stop\": [{\"hooks\": [{\"command\": \"$c/pkg/hooks/stop\"}]}]}}" >"$c/managed.json"
  links() { rm -f "$c/home/.agents/skills"/andon-*; for n in "$@"; do ln -s "$c/pkg/skills/$n" "$c/home/.agents/skills/andon-$n"; done; }
  journey() { # journey <道> → 終了コードを $jrc に、出力を $jout に
    jrc=0; jout=$(env PATH="$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" CLAUDE_CONFIG_DIR="$c/claude" \
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

  # 結果の記録の内訳（ot-breakdown。scripts/breakdown.sh）: 偽のセルの課題 1 件で、done と結果の記録と置き場の形ごとに判定を見る
  bk=$c/ws/projects/b/kanban; bl=$bk/issues/B-0001/library; btr=$c/state/andon/otel/b/issues/B-0001/traces.jsonl
  mkdir -p "$bl" "$(dirname "$btr")"
  echo '{"at": "2026-01-01T00:03:00Z", "role": "worker", "profile": "claude", "issue": "B-0001"}' >"$bl/launches.jsonl"
  bcheck() { # bcheck → 出力を $bout に（FAIL があっても止まらない）
    bout=$(env PATH="$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" \
      VERIFY_TMP="$c" VERIFY_RUN=verify-selftest bash -c '. "$1/cell.sh" >/dev/null; . "$1/breakdown.sh"; breakdown_check' _ "$skill/scripts" 2>&1) || true
  }
  bdone() { echo '{"at": "2026-01-01T00:04:00Z", "by": "terminal:t1", "id": "B-0001", "change": {"status": "doing"}}' >"$bk/log.jsonl"
    [ "$1" = done ] || return 0
    echo '{"at": "2026-01-01T00:09:00Z", "by": "terminal:t1", "id": "B-0001", "change": {"status": "done"}}' >>"$bk/log.jsonl"; }
  bout_of() { # bout_of <内訳のトークン（JSON か null）> <セッションのトークン> [<知識の読み（JSON。省くと {"index_reads": 1, "pages": []}。- でキーを書かない）>]
    local k=${3:-'{"index_reads": 1, "pages": []}'}
    if [ "$k" = - ]; then k=; else k=", \"knowledge\": $k"; fi
    echo "{\"at\": \"2026-01-01T00:10:00Z\", \"breakdown\": $( [ "$1" = null ] && echo null || echo "{\"tokens\": {\"input\": $1, \"output\": 0, \"cache_read\": 0, \"cache_write\": 0}$k}"), \"sessions\": [{\"cost\": {\"tokens\": {\"input\": $2, \"output\": 0, \"cache_read\": 0, \"cache_write\": 0}}}]}" >"$bl/outcome.json"; }
  bdone doing; bcheck
  grep -qF 'note ot-breakdown: 当てたあとに起動した Claude Code の担当の課題が、まだ片付いていない' <<<"$bout" || die "ot-breakdown: 片付いていない課題で、まだ片付いていない note を出さない: $bout"
  if grep -q '^FAIL' <<<"$bout"; then die "ot-breakdown: 片付いていない課題で落ちる: $bout"; fi
  bdone done; bcheck
  grep -qF 'に done になったのに、結果の記録' <<<"$bout" || die "ot-breakdown: done から時間が経っても結果の記録が無い課題で落ちない: $bout"
  bout_of 100 102; bcheck
  grep -qF 'ok   ot-breakdown: 当てたあとに片付いた担当の課題 1 件' <<<"$bout" || die "ot-breakdown: 上下 5% に入る内訳で ok にならない: $bout"
  grep -qF 'note ot-knowledge: 当てたあとに片付いた担当の課題 1 件の内訳に、読んだ wiki のページが無い' <<<"$bout" || die "ot-knowledge: ページを読んでいない内訳で note を出さない: $bout"
  if grep -q '^FAIL' <<<"$bout"; then die "ot-knowledge: ページを読んでいない内訳で落ちる: $bout"; fi
  bout_of 100 102 '{"index_reads": 1, "pages": [{"path": "topics/tool-quirks.md", "reads": 2}]}'; bcheck
  grep -qF 'ok   ot-knowledge: 当てたあとに片付いた担当の課題の内訳に、読んだ wiki のページがある（B-0001(topics/tool-quirks.md×2)）' <<<"$bout" || die "ot-knowledge: ページを読んだ内訳で ok にならない: $bout"
  bout_of 100 102 -; bcheck
  grep -qF 'FAIL ot-knowledge:' <<<"$bout" && grep -qF 'knowledge のキーが無い' <<<"$bout" || die "ot-knowledge: knowledge のキーの無い内訳で落ちない: $bout"
  # 置き場の記録が wiki のページを名指しているのに、内訳のページが空（数えていない）
  echo "{\"resourceSpans\": [{\"scopeSpans\": [{\"spans\": [{\"name\": \"claude_code.tool\", \"attributes\": [{\"key\": \"file_path\", \"value\": {\"stringValue\": \"$c/ws/wiki/topics/tool-quirks.md\"}}]}]}]}]}" >"$btr"
  bout_of 100 102; bcheck
  grep -qF 'FAIL ot-knowledge:' <<<"$bout" && grep -qF 'wiki のページを名指しているのに' <<<"$bout" || die "ot-knowledge: 置き場の記録がページを名指すのに内訳のページが空で落ちない: $bout"
  rm -f "$btr"
  bout_of 0 102 -; bcheck
  if grep -q 'FAIL ot-knowledge' <<<"$bout"; then die "ot-knowledge: モデルの呼び出しの無い内訳でキーを求めて落ちる: $bout"; fi
  bout_of 200 100; bcheck
  grep -qF 'の上下 5% に入らない' <<<"$bout" || die "ot-breakdown: 上下 5% に入らない内訳で落ちない: $bout"
  bout_of null 100; bcheck
  grep -qF 'note ot-breakdown: b/B-0001 は置き場に記録が無く' <<<"$bout" || die "ot-breakdown: 置き場に記録の無い null の内訳で note を出さない: $bout"
  if grep -q '^FAIL' <<<"$bout"; then die "ot-breakdown: 置き場に記録の無い null の内訳で落ちる: $bout"; fi
  echo '{"resourceSpans": []}' >"$btr"; bcheck
  grep -qF '記録があるのに、内訳が null' <<<"$bout" || die "ot-breakdown: 置き場に記録があるのに null の内訳で落ちない: $bout"
  echo '{"at": "2026-01-01T00:10:00Z", "sessions": []}' >"$bl/outcome.json"; bcheck
  grep -qF 'breakdown のキーが無い' <<<"$bout" || die "ot-breakdown: breakdown のキーの無い結果の記録で落ちない: $bout"
  rm -rf "$c/ws/projects/b" "$c/state/andon/otel/b"   # 後の場面が偽の課題を拾わない
  ok "verify-andon: ot-breakdown は、done から時間の経った課題の結果の記録が無い・置き場に記録があるのに内訳が null・キーが無い・上下 5% に入らないと落ち、片付いていないか記録が無ければ note、入れば ok"

  # 検証の途中で次の commit が当たった: 検証する commit（VERIFY_COMMIT）が当たっていた間だけを、次に当てた時刻まで読む
  lib=$c/ws/projects/v/kanban/issues/V-0002/library; mkdir -p "$lib"
  # 起動は andon issue resume（launch が resume）で、セッションの記録 sess の最初のメッセージの最後の段落が [resume] で始まる（wl-resume が確かめられ、0 で終わる）
  launch() { echo "{\"at\": \"$1\", \"issue\": \"V-0002\", \"role\": \"worker\", \"launch\": \"resume\", \"terminal\": \"t$2\", \"harness\": {\"name\": \"claude\"}, \"session\": \"sess\", \"package\": {\"commit\": \"$2\", \"source\": \"applied\", \"drift\": false, \"root\": \"$c/pkg\"}}"; }
  umsg() { jq -cn --arg t "$1" --arg m "$2" '{type: "user", timestamp: $t, origin: {kind: "human"}, message: {role: "user", content: $m}}'; } # umsg <時刻> <文>: セッションの記録のオーナーの印の user の行
  rmsg='<pasted_content id="ab12">\nAndon の担当として働く。\n\n[resume] 課題 `v/V-0002` の続き。セルが起き直した。worktree は同じ場所に戻した。\n</pasted_content id="ab12">\n'
  sess=$c/claude/projects/-v/sess.jsonl; mkdir -p "${sess%/*}"
  { umsg 2026-01-01T00:02:05.000Z "$(printf "$rmsg")"; umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")"; } >"$sess"
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

  # wl-resume: resume で起動した端末の最初のメッセージの最後の段落が [resume] で始まるか。今当たっている beef02（00:10 から）の間の起動 1 件を差し替えて見る
  wl() { # wl <起動の記録の行> <セッションの記録の行…>: 起動の記録と sess を置き換えて worker-launch を動かす
    echo "$1" >"$lib/launches.jsonl"; shift
    printf '%s\n' "$@" >"$sess"
    journey worker-launch
  }
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")")"
  [ "$jrc" = 0 ] && grep -qF 'ok   wl-resume' <<<"$jout" || die "wl-resume: 最後の段落が [resume] で始まるのに ok でない（$jrc）: $jout"
  wl "$(launch 2026-01-01T00:12:00Z beef02 | jq -c '.launch = "start"')" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")")"
  [ "$jrc" = 3 ] && grep -qF 'note wl-resume' <<<"$jout" || die "wl-resume: resume の起動が無いのに未確認の 3 で終わらない（$jrc）: $jout"
  wl "$(launch 2026-01-01T00:12:00Z beef02 | jq -c '.launch = "fresh"')" "$(umsg 2026-01-01T00:12:05.000Z '課題 `v/V-0002`。依頼書は x')"
  [ "$jrc" = 3 ] && grep -qF 'resume の起動 0 件' <<<"$jout" || die "wl-resume: --fresh の起動を resume に数える（$jrc）: $jout"
  # 印の無い文・resume なのに start の形の文は壊れ。印が文の頭にある文は、製品（spawn.Marked・振り返りの machineText）と同じく印つきと読む
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg" | sed 's/\[resume\] //')")"
  [ "$jrc" = 1 ] && grep -qF 'FAIL wl-resume' <<<"$jout" || die "wl-resume: 最後の段落に印が無いのに落ちない（$jrc）: $jout"
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf 'Andon の担当として働く。\n\n課題 `v/V-0002`。\n依頼書は x')")"
  [ "$jrc" = 1 ] && grep -qF 'FAIL wl-resume' <<<"$jout" || die "wl-resume: resume の最初のメッセージが start の形なのに落ちない（$jrc）: $jout"
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf '[resume] 課題 `v/V-0002` の続き。セルが起き直した。\n\n足した段落。')")"
  [ "$jrc" = 0 ] && grep -qF 'ok   wl-resume' <<<"$jout" || die "wl-resume: 印が文の頭にあるのに ok でない（製品の判定と食い違う）（$jrc）: $jout"
  # 起動の直後に入る裏の作業の知らせ（origin.kind が task-notification）と道具の結果は、最初のメッセージに数えない
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:02.000Z '<task-notification>x</task-notification>' | jq -c '.origin.kind = "task-notification"')" \
    "$(umsg 2026-01-01T00:12:03.000Z x | jq -c 'del(.origin) | .message.content = [{type: "tool_result", content: "y"}]')" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")")"
  [ "$jrc" = 0 ] && grep -qF 'ok   wl-resume' <<<"$jout" || die "wl-resume: 起動の直後の知らせや道具の結果を最初のメッセージと読む（$jrc）: $jout"
  # 同じ課題の次の起動のあとの文を、この起動の証拠にしない（次の起動の前に文が無ければ未確認）
  wl "$(launch 2026-01-01T00:12:00Z beef02; launch 2026-01-01T00:13:00Z beef02 | jq -c '.launch = "start" | .session = "other"')" "$(umsg 2026-01-01T00:13:05.000Z "$(printf "$rmsg")")"
  [ "$jrc" = 3 ] && grep -qF 'note wl-resume' <<<"$jout" || die "wl-resume: 次の起動のあとの印つきの文を前の起動の証拠にした（$jrc）: $jout"
  # 前に当てた commit を見るときは、次に当てた時刻のあとの文を証拠にしない（c0ffee は 00:00 から 00:10 まで）
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x",\n "previous": [{"commit": "c0ffee", "applied_at": "2026-01-01T00:00:00Z"}]}\n' >"$c/pkg/applied.json"
  echo "$(launch 2026-01-01T00:02:00Z c0ffee)" >"$lib/launches.jsonl"; umsg 2026-01-01T00:11:00.000Z "$(printf "$rmsg")" >"$sess"
  VERIFY_COMMIT=c0ffee journey worker-launch
  [ "$jrc" = 3 ] && grep -qF 'note wl-resume' <<<"$jout" || die "wl-resume: 次に当てた時刻のあとの文を前の commit の証拠にした（$jrc）: $jout"
  printf '{"commit": "beef02", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x"}\n' >"$c/pkg/applied.json"
  # 起動の前の文（前に印の付いた resume）を、この起動の証拠にしない。起動のあとの印の無い文で落ちる
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:11:59.000Z "$(printf "$rmsg")")" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg" | sed 's/\[resume\] //')")"
  [ "$jrc" = 1 ] && grep -qF 'FAIL wl-resume' <<<"$jout" || die "wl-resume: 起動の前の文を証拠にした（$jrc）: $jout"
  # content が道具の結果と同じ配列の形でも読む
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")" | jq -c '.message.content = [{type: "text", text: .message.content}]')"
  [ "$jrc" = 0 ] && grep -qF 'ok   wl-resume' <<<"$jout" || die "wl-resume: content が配列の形の文を読まない（$jrc）: $jout"
  # セッションの記録が無い（Codex）: 画面に印の行があれば ok、無ければ未確認の 3（壊れにしない）
  wl "$(launch 2026-01-01T00:12:00Z beef02 | jq -c '.harness.name = "codex" | .session = null')"
  [ "$jrc" = 3 ] && grep -qF 'note wl-resume' <<<"$jout" || die "wl-resume: 画面にもセッションの記録にも読めないのに未確認の 3 で終わらない（$jrc）: $jout"
  printf '› [resume] 課題 `v/V-0002` の続き。セルが起き直した。\n' >"$c/screen"
  wl "$(launch 2026-01-01T00:12:00Z beef02 | jq -c '.harness.name = "codex" | .session = null')"
  rm -f "$c/screen"
  # 画面に前の起動の印つきの行だけが残り、今回の最初のメッセージが届いていなくても、画面だけでは今回の起動の文と分けられない。ok にせず未確認の 3
  [ "$jrc" = 3 ] && grep -qF '画面の続きの行は全部印 [resume] つきだが' <<<"$jout" && ! grep -qF 'ok   wl-resume' <<<"$jout" \
    || die "wl-resume: 画面の印つきの行だけで ok にした（前の起動の印を今回の証拠にする）（$jrc）: $jout"
  # 画面は前の起動の印つきの文も映す。印の無い続きの行が1つでもあれば ok にしない（壊れとも言えないので未確認）
  printf '› [resume] 課題 `v/V-0002` の続き。セルが起き直した。\n…\n› 課題 `v/V-0002` の続き。セルが起き直した。\n' >"$c/screen"
  wl "$(launch 2026-01-01T00:12:00Z beef02 | jq -c '.harness.name = "codex" | .session = null')"
  rm -f "$c/screen"
  [ "$jrc" = 3 ] && grep -qF '画面に印の無い続きの行がある' <<<"$jout" || die "wl-resume: 画面に印の無い続きの行があるのに未確認の 3 で終わらない（$jrc）: $jout"
  # Claude Code の担当はセッションの記録で決める。画面に前の起動の印つきの行があっても、記録の印の無い文で落ちる
  printf '› [resume] 課題 `v/V-0002` の続き。セルが起き直した。\n' >"$c/screen"
  wl "$(launch 2026-01-01T00:12:00Z beef02)" "$(umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg" | sed 's/\[resume\] //')")"
  rm -f "$c/screen"
  [ "$jrc" = 1 ] && grep -qF 'FAIL wl-resume' <<<"$jout" || die "wl-resume: 画面の前の起動の印で、記録の印の無い文を ok にした（$jrc）: $jout"
  { umsg 2026-01-01T00:02:05.000Z "$(printf "$rmsg")"; umsg 2026-01-01T00:12:05.000Z "$(printf "$rmsg")"; } >"$sess"
  ok "verify-andon: wl-resume は、resume の起動のあとの最初のオーナーの印の文の頭にも最後の段落の頭にも [resume] が無いと落ち、知らせ・次の起動や次に当てたあとの文・画面の前の起動の印を証拠にせず、確かめられなければ未確認の 3 で終わる"

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

  # gate の gt-skip: 自分の commit の無い課題（報告の commit が既定の branch に入っている）が PR 無しで review に入ったら、
  # 担当がそのあと doing に戻していない（gate の fix を受けていない）。偽のセルに repo の clone（.ghq）を置き、本物の kanban の CLI で返す
  r=$c/ws/.ghq/github.com/example/sample-site
  rm -rf "$r" && git init -q "$r" && git -C "$r" commit -q --allow-empty -m one && git -C "$r" update-ref refs/remotes/origin/main HEAD \
    && git -C "$r" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main || die "gt-skip の場面の repo が作れない"
  ssetup() { # ssetup <review のあとに担当が doing に戻すか true/false>: kanban を作り直し、PR 無しの課題を review に入れる
    rm -rf "$g" && mkdir -p "$g" && (cd "$g" && "$kanban" init --prefix G --repo github.com/example/sample-site >/dev/null) || die "gt-skip の場面の kanban が作れない"
    local id
    id=$(KANBAN_ROOT=$g HC_TERMINAL_ID=tc "$kanban" add "検証" --repo github.com/example/sample-site)
    KANBAN_ROOT=$g HC_TERMINAL_ID=tw "$kanban" start "$id" >/dev/null
    mkdir -p "$g/kanban/issues/$id/library"
    echo "{\"commit\": \"$(git -C "$r" rev-parse HEAD)\"}" >"$g/kanban/issues/$id/library/report.json"
    KANBAN_ROOT=$g HC_TERMINAL_ID=tw "$kanban" report "$id" <<<"## Report" >/dev/null
    KANBAN_ROOT=$g HC_TERMINAL_ID=tw "$kanban" review "$id" >/dev/null || die "gt-skip の場面の課題を review にできない"
    if [ "$1" = true ]; then KANBAN_ROOT=$g HC_TERMINAL_ID=tw "$kanban" start "$id" >/dev/null; fi
  }
  # gate の記録が無い場面なので、gt-record・gt-route は未確認で、道の終了コードは 3（gt-skip の ok・note は終了コードを変えない）
  ssetup false; journey gate
  [ "$jrc" = 3 ] && grep -qF 'ok   gt-skip' <<<"$jout" || die "gate: PR 無しの課題が review のままなのに gt-skip が ok でない（$jrc）: $jout"
  ssetup true; journey gate
  [ "$jrc" = 1 ] && grep -qF 'FAIL gt-skip' <<<"$jout" || die "gate: PR 無しの課題を review のあと担当が doing に戻したのに gt-skip が落ちない（$jrc）: $jout"
  git -C "$r" commit -q --allow-empty -m mine && git -C "$r" update-ref refs/remotes/origin/main HEAD~1 || die "gt-skip の場面の自分の commit が作れない"
  ssetup true; journey gate
  [ "$jrc" = 3 ] && grep -qF 'note gt-skip' <<<"$jout" || die "gate: 自分の commit のある課題の fix を gt-skip が数えた（$jrc）: $jout"
  ok "verify-andon: gate の gt-skip は、自分の commit の無い PR 無しの課題を review のあと担当が doing に戻すと落ち、review のままなら ok、自分の commit のある課題は数えない"

  # drive.sh は検証する commit を省けない。検証の途中で次の commit が当たったあと、省いて打つと新しい commit の間を読み、
  # 検証する commit の出来事を見ずに未確認や誤りになっていた。偽のセルで、drive.sh を利用者と同じく通しで打つ
  old=c0ffee0000000000000000000000000000000000 new=beef020000000000000000000000000000000000
  printf '{"commit": "%s", "applied_at": "2026-01-01T00:10:00Z", "image_commit": "x",\n "previous": [{"commit": "%s", "applied_at": "2026-01-01T00:00:00Z"}]}\n' "$new" "$old" >"$c/pkg/applied.json"
  { launch 2026-01-01T00:02:00Z "$old"; launch 2026-01-01T00:12:00Z "$new"; } >"$lib/launches.jsonl"
  drive() { # drive <引数…> → 終了コードを $drc に、出力（標準エラーも）を $dout に
    drc=0; dout=$(env PATH="$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" CLAUDE_CONFIG_DIR="$c/claude" TMPDIR="$c" \
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

  # 当てた直後の検証で、変えた細かい機能は ok なのに、本物の出来事がまだ無い別の細かい機能（wl-resume）が未確認で、機能の終了コードが 3 になり、
  # 記録の期待と受け入れ条件の書き方を担当ごとに考えていた。drive.sh は細かい機能ごとの結果を1行ずつ出し、名指した細かい機能だけで終了コードを決める
  launch 2026-01-01T00:12:00Z "$new" | jq -c '.launch = "start"' >"$lib/launches.jsonl" # resume の起動が無い: wl-resume は未確認、wl-record は ok
  drive worker-launch "$new"
  [ "$drc" = 3 ] || die "drive.sh: 名指しが無いのに、道の終了コード 3 のままでない（$drc）: $dout"
  grep -qx '結果 wl-record ok（補い）' <<<"$dout" && grep -q '^結果 wl-resume 未確認（補い）: note wl-resume' <<<"$dout" && grep -q '^結果 wl-first-turn 外で見る（補い）: 見る道: Chief of Staff' <<<"$dout" \
    || die "drive.sh: 細かい機能ごとの結果（ok・未確認と note）を1行ずつ出さない: $dout"
  drive worker-launch "$new" wl-record
  [ "$drc" = 0 ] && grep -qx '結果 wl-record ok（名指し）' <<<"$dout" && grep -q '^結果 wl-resume 未確認（補い）: note wl-resume' <<<"$dout" \
    || die "drive.sh: 名指した wl-record が ok なのに 0 で終わらないか、名指さない wl-resume の未確認を補いとして出さない（$drc）: $dout"
  drive worker-launch "$new" wl-record,wl-resume
  [ "$drc" = 3 ] && grep -qF '名指したうち未確認: wl-resume' <<<"$dout" || die "drive.sh: 名指した wl-resume が未確認なのに 3 で終わらない（$drc）: $dout"
  drive worker-launch "$new" "wl-record gt-route"
  [ "$drc" = 0 ] && grep -qF '別の機能の gt-route は数えない' <<<"$dout" || die "drive.sh: 別の機能の細かい機能を名指しに数える（$drc）: $dout"
  # 名指しが別の機能か外で見るものだけ: 決める細かい機能が無く、補いの wl-resume の未確認で 3 にしない（Verify-features: wl-first-turn の範囲の形）
  drive worker-launch "$new" gt-route
  [ "$drc" = 0 ] && grep -qF 'drive.sh で確かめるものがこの機能に無い' <<<"$dout" || die "drive.sh: 名指しが別の機能のものだけなのに、補いの未確認で 0 でない（$drc）: $dout"
  drive worker-launch "$new" wl-first-turn
  [ "$drc" = 0 ] && grep -q '^結果 wl-resume 未確認（補い）' <<<"$dout" || die "drive.sh: 名指しが外で見る wl-first-turn だけなのに、補いの未確認で 0 でない（$drc）: $dout"
  # 見る道が drive.sh の外（Chief of Staff の試し）の細かい機能は、名指しても終了コードに数えない（数えると、いつも 3 になる）
  drive worker-launch "$new" wl-record,wl-first-turn
  [ "$drc" = 0 ] && grep -q '^結果 wl-first-turn 外で見る（名指し）' <<<"$dout" || die "drive.sh: drive.sh の外で見る wl-first-turn を名指しの終了コードに数える（$drc）: $dout"
  refused "のあとの細かい機能が地図に無い" worker-launch "$new" wl-nosuch
  refused "のあとの細かい機能が空の" worker-launch "$new" ,
  # 名指さない細かい機能の FAIL は、今までどおり 1（変えていない道の壊れを見落とさない）
  launch 2026-01-01T00:12:00Z "$old" | jq -c --arg a 2026-01-01T00:12:00Z '.launch = "start" | .at = $a' >"$lib/launches.jsonl"
  drive worker-launch "$new" wl-terminal
  [ "$drc" = 1 ] && grep -qx '結果 wl-record FAIL（補い）' <<<"$dout" || die "drive.sh: 名指さない wl-record が FAIL なのに 1 で終わらない（$drc）: $dout"
  # 道が FAIL を出したあと 3 で終わる（otel は受け口が無いと FAIL を出し、Claude Code の起動が無いと 3 で抜ける）: 壊れを未確認にせず 1
  for a in "" ot-restart gt-route ot-records; do
    drive otel "$new" ${a:+"$a"}
    [ "$drc" = 1 ] && grep -q '^FAIL ot-' <<<"$dout" && grep -qF '終了コード 1（FAIL の行がある。道の終了コードは 3）' <<<"$dout" \
      || die "drive.sh: 道が FAIL を出して 3 で終わったのに、名指し「${a:-無し}」で 1 にしない（$drc）: $dout"
  done
  # 出力の写しが途中で切れる（tee が先頭の数行だけを書いて落ちる）: 後ろの FAIL を読めないまま、名指した ok で 0 にしない
  mkdir -p "$c/teebin"
  printf '#!/bin/sh\nf=$1; n=0\nwhile IFS= read -r l; do echo "$l"; n=$((n + 1)); [ "$n" -gt 3 ] || echo "$l" >>"$f"; done\nexit 1\n' >"$c/teebin/tee"; chmod +x "$c/teebin/tee"
  drc=0; dout=$(env PATH="$c/teebin:$c/bin:$PATH" HOME="$c/home" XDG_STATE_HOME="$c/state" ANDON_WORKSPACE="$c/ws" VERIFY_PACKAGE="$c/pkg" CLAUDE_CONFIG_DIR="$c/claude" TMPDIR="$c" \
    bash "$skill/scripts/drive.sh" otel "$new" ot-listen 2>&1) || drc=$?
  [ "$drc" = 1 ] && grep -qF 'FAIL 道の出力の写し' <<<"$dout" || die "drive.sh: 出力の写しが途中で切れたのに 1 にしない（$drc）: $dout"
  ok "verify-andon: drive.sh は細かい機能ごとの結果を1行ずつ出し、名指した細かい機能だけで 0 と 3 を決め、名指さない未確認は補いとして出し、FAIL は名指しによらず 1。地図に無い名前は 2 で断る"
fi
