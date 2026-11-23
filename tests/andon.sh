#!/usr/bin/env bash
# tests/andon.sh — 入口 andon（振り分け・一覧・--help・知らない sub-command）。
# 中身（publish・triage）の振る舞いは go test と tests/andon-triage.sh が見る。kanban も hc も起動しない。
set -u
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
cd "$tmp" || exit 2   # kanban の外で打つ

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }
# === tests/lib/isolate.sh: 受け継いだ置き場をセルの本物から外す（A-0200） ===
# セルの値を持って source したとき、HOME より先に読む置き場は外れ、既定がセルの本物を指すものは一時ディレクトリの中を指す
got=$(env ANDON_WORKSPACE=/workspace GHQ_ROOT=/workspace/.ghq XDG_CONFIG_HOME=/x/config XDG_DATA_HOME=/x/data XDG_STATE_HOME=/x/state \
  KANBAN_ROOT=/x/k CLAUDE_CONFIG_DIR=/x/c CODEX_HOME=/x/cx ANDON_HOME=/x/a HC_TERMINAL_ID=t1 tmp="$tmp/iso" bash -c '
  mkdir -p "$tmp"; . "$1"
  for v in KANBAN_ROOT CLAUDE_CONFIG_DIR CODEX_HOME ANDON_HOME XDG_STATE_HOME XDG_CONFIG_HOME XDG_DATA_HOME HC_TERMINAL_ID; do
    [ -z "${!v+x}" ] || echo "$v が残る"
  done
  for v in ANDON_WORKSPACE GHQ_ROOT; do case "${!v}" in "$tmp"/*) ;; *) echo "$v=${!v}" ;; esac; done' _ "$andon_root/tests/lib/isolate.sh" 2>&1)
[ -z "$got" ] && pass "isolate.sh は受け継いだ置き場を外し、ANDON_WORKSPACE・GHQ_ROOT を一時ディレクトリの中へ向ける" || fail "isolate.sh: $got"

# === 葉の表（ADR-0053 決めたこと 11）: andon help --all の群と行から、葉と主に打つ者を読む ===
# 行は「  <葉>  <打つ者>  <説明>」。群の見出し「<打つ者>が打つ:」の下の行は、その者が主に打つ葉
"$andon" help --all >"$tmp/all" 2>"$tmp/err"; r=$?
leaves_of() { awk -v g="$1が打つ:" '/^[^ ].*が打つ:$/ { on = ($0 == g); next } on && /^  [a-z]/ { sub(/^  /, ""); sub(/   *.*/, ""); print }' "$tmp/all"; }
owner_leaves=$(leaves_of オーナー); role_leaves=$(leaves_of 役); machine_leaves=$(leaves_of 機械)
all_leaves=$(printf '%s\n' "$owner_leaves" "$role_leaves" "$machine_leaves" | grep -v '^$')
n=$(printf '%s\n' "$all_leaves" | wc -l)
[ "$r" = 0 ] && [ ! -s "$tmp/err" ] && [ -n "$owner_leaves" ] && [ -n "$role_leaves" ] && [ -n "$machine_leaves" ] && [ "$n" -ge 50 ] \
  && printf '%s\n' "$all_leaves" | grep -qx 'issue start' && printf '%s\n' "$owner_leaves" | grep -qx 'open' \
  && pass "andon help --all は全部の葉（$n）を、主に打つ者の群に分けて1行ずつ出す" || fail "help --all: rc=$r n=$n $(cat "$tmp/err")"

# === A4 andon help: 主に打つ者がオーナーの葉は全部あり、機械の葉は1つも無い（行の形「  <葉>  」で見る） ===
"$andon" help >"$tmp/out" 2>"$tmp/err"; r=$?
miss=$(printf '%s\n' "$owner_leaves" | while IFS= read -r l; do grep -qF "  $l  " "$tmp/out" && grep -q "^  $l  " "$tmp/out" || echo "$l"; done)
extra=$(printf '%s\n' "$machine_leaves" | while IFS= read -r l; do grep -q "^  $l  " "$tmp/out" && echo "$l"; done)
[ "$r" = 0 ] && [ ! -s "$tmp/err" ] && [ -z "$miss" ] && [ -z "$extra" ] \
  && pass "andon help はオーナーの葉を全部並べ、機械の葉を並べない" || fail "help: rc=$r 無いオーナーの葉=[$miss] 出た機械の葉=[$extra]"
# 例が先、役と機械は名詞の1行、続きの引き方（決めたこと 12）
grep -n '' "$tmp/out" | awk -F: '/^[0-9]+:例:$/ {e=$1} /^[0-9]+:オーナーが打つ:$/ {o=$1} /^[0-9]+:役が打つ: / {r=$1} /^[0-9]+:機械が打つ: / {m=$1}
  END { exit !(e && o && r && m && e < o && o < r && r < m) }' && grep -q '^  andon open ' "$tmp/out" && grep -q 'andon help --all' "$tmp/out" \
  && pass "andon help は例・オーナーの葉・役と機械の名詞・続きの引き方の順" || fail "help の並び: $(cat "$tmp/out")"
# 検査が何も見ていなくても通らないこと: 機械の葉の行を足した help は落ちる
{ cat "$tmp/out"; echo "  dispatcher drain  x"; } >"$tmp/out.bad"
printf '%s\n' "$machine_leaves" | while IFS= read -r l; do grep -q "^  $l  " "$tmp/out.bad" && echo "$l"; done | grep -q . \
  && pass "機械の葉の行があれば、上の検査が見つける" || fail "機械の葉の行を足しても見つからない"
# news は ADR-0010 の andon news ack で sub-command になった（仮設の news〈今の dispatcher〉とは別の口）
for s in yard coordinator; do grep -qw "$s" "$tmp/out" "$tmp/all" && fail "help に仮設の名前 $s が載っている"; done
"$andon" >"$tmp/out2" 2>&1; r=$?
[ "$r" = 0 ] && cmp -s "$tmp/out" "$tmp/out2" && pass "引数なしは andon help と同じ" || fail "引数なし: rc=$r"

# === 知らない名前: 綴りの近い葉を勧めて rc 2 ===
"$andon" bogus >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && [ ! -s "$tmp/out" ] && grep -q '知らないコマンド: bogus' "$tmp/err" && grep -q 'andon help' "$tmp/err" \
  && pass "知らない名前は rc 2 で andon help を案内する" || fail "bogus: rc=$r $(cat "$tmp/err")"
"$andon" issue recrd S-1 >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && [ ! -s "$tmp/out" ] && grep -q '近いのは andon issue record$' "$tmp/err" \
  && pass "1字誤った葉は、正しい葉を勧めて rc 2" || fail "issue recrd: rc=$r $(cat "$tmp/err")"
for old in coordinator; do
  "$andon" "$old" >/dev/null 2>&1; r=$?
  [ "$r" = 2 ] && pass "仮設の名前（$old）は sub-command にならない" || fail "$old: rc=$r"
done

# === andon <葉> --help と -h: 使い方を出して rc 0（中身は起動しない）。全部の葉 ===
bad=$(printf '%s\n' "$all_leaves" | while IFS= read -r l; do
  for h in --help -h; do
    # shellcheck disable=SC2086 # 葉の名前は1語か2語で、語に分けて渡す
    "$andon" $l "$h" >"$tmp/out" 2>"$tmp/err"; r=$?
    [ "$r" = 0 ] && grep -q "^usage: andon $l" "$tmp/out" && [ ! -s "$tmp/err" ] || echo "$l $h rc=$r"
  done
done)
[ -z "$bad" ] && pass "全部の葉の --help と -h は使い方を出して rc 0" || fail "葉の --help: $bad"

# === 名詞だけ: その名詞の葉の一覧（名詞だけで動く今の葉 waiting・dispatcher を除く） ===
"$andon" triage >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q '^usage: andon triage <動詞>' "$tmp/out" && grep -q '^  triage collect  ' "$tmp/out" && grep -q '^  triage verdict  ' "$tmp/out" \
  && pass "andon triage（名詞だけ）は triage の葉の一覧で rc 0" || fail "andon triage: rc=$r $(cat "$tmp/out" "$tmp/err")"

# === 中身に渡る: 引数の誤りは中身の usage と終了コード ===
"$andon" triage verdict >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon triage verdict' "$tmp/err" && pass "andon triage verdict（引数なし）は中身の usage で rc 2" || fail "andon triage verdict: rc=$r $(cat "$tmp/err")"
"$andon" dispatcher --bogus >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon dispatcher' "$tmp/err" && pass "andon dispatcher（知らない引数）は usage で rc 2" || fail "andon dispatcher: rc=$r $(cat "$tmp/err")"
"$andon" chief-of-staff start >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon chief-of-staff' "$tmp/err" && pass "andon chief-of-staff（知らない引数）は usage で rc 2" || fail "andon chief-of-staff: rc=$r $(cat "$tmp/err")"
"$andon" issue start X-1 extra >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon issue' "$tmp/err" && pass "andon issue start（余分な引数）は usage で rc 2" || fail "andon issue start: rc=$r $(cat "$tmp/err")"
"$andon" task start X-1 >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '知らないコマンド: task' "$tmp/err" && pass "andon task は知らない command（別名なし。ADR-0008 決めたこと 7）" || fail "andon task: rc=$r $(cat "$tmp/err")"
{ "$andon" --help; "$andon" issue --help; } 2>&1 | grep -qiw 'task' && fail "andon --help・andon issue --help に task が出る" || pass "andon --help・andon issue --help に task が出ない"
"$andon" chief-engineer >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon chief-engineer' "$tmp/err" && pass "andon chief-engineer（引数なし）は usage で rc 2" || fail "andon chief-engineer: rc=$r $(cat "$tmp/err")"
"$andon" publish >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon publish' "$tmp/err" && pass "andon publish（引数なし）は usage で rc 2" || fail "andon publish: rc=$r $(cat "$tmp/err")"
"$andon" publish "$tmp/none.html" >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'not a file' "$tmp/err" && pass "andon publish（無いファイル）は中身の誤りで rc 1" || fail "andon publish: rc=$r $(cat "$tmp/err")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
