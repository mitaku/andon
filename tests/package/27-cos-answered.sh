# tests/package/27-cos-answered.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置き、pb は前の節が置く）。
#  27. ペインの答えを受ける段（ADR-0042 受け入れ条件 19・決めたこと 18・19）: 手引き watch.md の通知の表に answered の行があり、
#      問いを出した相手へ答えを渡すことと、オーナーへの返事に1行で書くことがその行にある。当たる場合があることを同じ所で確かめる。
# === 27. 通知 answered を受けたら（ADR-0042 受け入れ条件 19） ===
ans_row=$(grep '^| `<id> answered: ' "$pb/watch.md")
asked_row=$(grep '^| `<id> asked: ' "$pb/watch.md")
[ -n "$asked_row" ] \
  && grep -q 'hc send <agent>' <<<"$ans_row" \
  && grep -q 'kanban received' <<<"$ans_row" \
  && grep -q 'オーナーへ.*1行で書く' <<<"$ans_row" \
  && grep -q 'andon news ack` で黙らない' <<<"$ans_row" \
  && pass "watch.md の通知の表に answered の行があり、問いを出した相手へ渡し、オーナーへ1行で書く段がある" \
  || fail "watch.md の通知の表に answered の行が無いか、相手へ渡す・オーナーへ1行で書く段が無い（asked の行: ${asked_row:+あり}）"
