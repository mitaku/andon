# tests/package/05-statusline.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   5. statusline が JSON を読んで1行を出す。
# === 5. statusline ===
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp"}' | bash package/lib/andon/statusline 2>"$tmp/err")
case $out in *Sample*42%*) [ ! -s "$tmp/err" ] && pass "statusline は model と context% を出す" || fail "statusline の stderr: $(cat "$tmp/err")" ;;
  *) fail "statusline の出力: $out" ;; esac
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp","cost":{"total_duration_ms":3725000},"effort":{"level":"high"}}' | HOME=$tmp bash package/lib/andon/statusline)
case $out in *1h2m*high*) pass "statusline は入力の cost.total_duration_ms と effort.level を出す" ;; *) fail "statusline の時間と effort: $out" ;; esac
out=$(printf '' | bash package/lib/andon/statusline)
[ "$out" = Claude ] && pass "statusline は空の入力で Claude" || fail "statusline（空）: $out"
