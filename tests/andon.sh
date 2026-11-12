#!/usr/bin/env bash
# tests/andon.sh — 入口 andon（振り分け・一覧・--help・知らない sub-command）。
# 中身（publish・triage）の振る舞いは go test と tests/andon-triage.sh が見る。台帳も hc も起動しない。
set -u
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
cd "$tmp" || exit 2   # 台帳の外で打つ

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }
subs="issue chief-of-staff create open chief-engineer dispatcher news publish triage gate delegate boot"

# === andon help: 一覧 ===
"$andon" help >"$tmp/out" 2>"$tmp/err"; r=$?
ok=1
for s in $subs help; do grep -q "^  $s  " "$tmp/out" || ok=0; done
[ "$r" = 0 ] && [ "$ok" = 1 ] && [ ! -s "$tmp/err" ] && pass "andon help は各 sub-command を1行ずつ並べる" || fail "help: rc=$r $(cat "$tmp/out" "$tmp/err")"
# news は ADR-0010 の andon news ack で sub-command になった（仮設の news〈今の dispatcher〉とは別の口）
for s in yard coordinator; do grep -qw "$s" "$tmp/out" && fail "help に仮設の名前 $s が載っている"; done
"$andon" >"$tmp/out2" 2>&1; r=$?
[ "$r" = 0 ] && cmp -s "$tmp/out" "$tmp/out2" && pass "引数なしは andon help と同じ" || fail "引数なし: rc=$r"

# === 知らない sub-command: 一覧を stderr に出して rc 2 ===
"$andon" bogus >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && [ ! -s "$tmp/out" ] && grep -q '知らない sub-command: bogus' "$tmp/err" && grep -q '^  dispatcher  ' "$tmp/err" \
  && pass "知らない sub-command は一覧を stderr に出して rc 2" || fail "bogus: rc=$r $(cat "$tmp/err")"
for old in coordinator; do
  "$andon" "$old" >/dev/null 2>&1; r=$?
  [ "$r" = 2 ] && pass "仮設の名前（$old）は sub-command にならない" || fail "$old: rc=$r"
done

# === andon <sub> --help: 使い方を出して rc 0（中身は起動しない） ===
for s in $subs; do
  "$andon" "$s" --help >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 0 ] && grep -q "^usage: andon $s" "$tmp/out" && [ ! -s "$tmp/err" ] \
    && pass "andon $s --help" || fail "andon $s --help: rc=$r $(cat "$tmp/out" "$tmp/err")"
done

# === 中身に渡る: 引数の誤りは中身の usage と終了コード ===
"$andon" triage >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon triage collect' "$tmp/err" && pass "andon triage（引数なし）は中身の usage で rc 2" || fail "andon triage: rc=$r $(cat "$tmp/err")"
"$andon" dispatcher --bogus >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon dispatcher' "$tmp/err" && pass "andon dispatcher（知らない引数）は usage で rc 2" || fail "andon dispatcher: rc=$r $(cat "$tmp/err")"
"$andon" chief-of-staff start >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon chief-of-staff' "$tmp/err" && pass "andon chief-of-staff（知らない引数）は usage で rc 2" || fail "andon chief-of-staff: rc=$r $(cat "$tmp/err")"
"$andon" issue start X-1 extra >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon issue' "$tmp/err" && pass "andon issue start（余分な引数）は usage で rc 2" || fail "andon issue start: rc=$r $(cat "$tmp/err")"
"$andon" task start X-1 >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '知らない sub-command: task' "$tmp/err" && pass "andon task は知らない command（別名なし。ADR-0008 決めたこと 7）" || fail "andon task: rc=$r $(cat "$tmp/err")"
{ "$andon" --help; "$andon" issue --help; } 2>&1 | grep -qiw 'task' && fail "andon --help・andon issue --help に task が出る" || pass "andon --help・andon issue --help に task が出ない"
"$andon" chief-engineer >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon chief-engineer' "$tmp/err" && pass "andon chief-engineer（引数なし）は usage で rc 2" || fail "andon chief-engineer: rc=$r $(cat "$tmp/err")"
"$andon" publish >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon publish' "$tmp/err" && pass "andon publish（引数なし）は usage で rc 2" || fail "andon publish: rc=$r $(cat "$tmp/err")"
"$andon" publish "$tmp/none.html" >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'not a file' "$tmp/err" && pass "andon publish（無いファイル）は中身の誤りで rc 1" || fail "andon publish: rc=$r $(cat "$tmp/err")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
