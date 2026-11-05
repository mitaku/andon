#!/usr/bin/env bash
# tests/andon.sh — 入口 andon（振り分け・一覧・--help・知らない sub-command・骨だけの sub-command）。
# 中身（publish・triage）の振る舞いは go test と tests/andon-triage.sh が見る。台帳も hc も起こさない。
set -u
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
cd "$tmp" || exit 2   # 台帳の外で打つ

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }
subs="task chief-of-staff courier publish triage gate"

# === andon help: 一覧 ===
"$andon" help >"$tmp/out" 2>"$tmp/err"; r=$?
ok=1
for s in $subs help; do grep -q "^  $s  " "$tmp/out" || ok=0; done
[ "$r" = 0 ] && [ "$ok" = 1 ] && [ ! -s "$tmp/err" ] && pass "andon help は各 sub-command を1行ずつ並べる" || fail "help: rc=$r $(cat "$tmp/out" "$tmp/err")"
for s in yard coordinator news; do grep -qw "$s" "$tmp/out" && fail "help に仮設の名前 $s が載っている"; done
"$andon" >"$tmp/out2" 2>&1; r=$?
[ "$r" = 0 ] && cmp -s "$tmp/out" "$tmp/out2" && pass "引数なしは andon help と同じ" || fail "引数なし: rc=$r"

# === 知らない sub-command: 一覧を stderr に出して rc 2 ===
"$andon" bogus >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && [ ! -s "$tmp/out" ] && grep -q '知らない sub-command: bogus' "$tmp/err" && grep -q '^  courier  ' "$tmp/err" \
  && pass "知らない sub-command は一覧を stderr に出して rc 2" || fail "bogus: rc=$r $(cat "$tmp/err")"
for old in coordinator news; do
  "$andon" "$old" >/dev/null 2>&1; r=$?
  [ "$r" = 2 ] && pass "仮設の名前（$old）は sub-command にならない" || fail "$old: rc=$r"
done

# === andon <sub> --help: 使い方を出して rc 0（中身は起こさない） ===
for s in $subs; do
  "$andon" "$s" --help >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 0 ] && grep -q "^usage: andon $s" "$tmp/out" && [ ! -s "$tmp/err" ] \
    && pass "andon $s --help" || fail "andon $s --help: rc=$r $(cat "$tmp/out" "$tmp/err")"
done

# === 骨だけの sub-command: 中身が無いと言って rc 1 ===
for s in task chief-of-staff courier gate; do
  "$andon" "$s" start X-1 >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && [ ! -s "$tmp/out" ] && grep -q '中身はまだ無い' "$tmp/err" \
    && pass "andon $s は中身はまだ無いと言って rc 1" || fail "andon $s: rc=$r $(cat "$tmp/err")"
done

# === 中身に渡る: 引数の誤りは中身の usage と終了コード ===
"$andon" triage >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon triage collect' "$tmp/err" && pass "andon triage（引数なし）は中身の usage で rc 2" || fail "andon triage: rc=$r $(cat "$tmp/err")"
"$andon" publish >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon publish' "$tmp/err" && pass "andon publish（引数なし）は usage で rc 2" || fail "andon publish: rc=$r $(cat "$tmp/err")"
"$andon" publish "$tmp/none.html" >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'not a file' "$tmp/err" && pass "andon publish（無いファイル）は中身の誤りで rc 1" || fail "andon publish: rc=$r $(cat "$tmp/err")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
