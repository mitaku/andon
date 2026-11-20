# tests/package/10b-chief-of-staff-size.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  10b. Chief of Staff の行数・承認の段・起動の go・続きの文（A-0123）。
# === 10b. Chief of Staff の行数・承認の段・起動の go・続きの文（A-0123。A-0117 の c6・c9・c10・c13） ===
! grep -qE '2行まで|オーナー(へ|に)1行|1行で(言|知らせ|伝え|返)' <<<"$cosall" && grep -q 'オーナーへの文は短く、オーナーがすることを先に書く。.*`/opt/andon/writing.md`' "$cos" \
  && pass "chief-of-staff skill にオーナーへの文の行数の上限が無く、「短く。オーナーがすることを先に」と writing.md を指す" || fail "chief-of-staff skill に行数の上限が残る"
cos4=$(cat "$pb/blocked.md")
grep -q '^     | Yes と答える | オーナーへ回す |$' <<<"$cos4" && grep -q 'force-with-lease` も含む' <<<"$cos4" && ! grep -q '外へ出さないものだけ' <<<"$cosall" \
  && grep -q '^4\. \*\*後戻りできない操作はオーナーの go\*\*。.*線は worker skill §5 と同じ' "$cos" \
  && pass "chief-of-staff の原則4が worker skill §5 の線を指し、blocked.md に Yes と答える操作とオーナーへ回す操作が並ぶ（自分の PR の branch への force-with-lease は Yes）" || fail "chief-of-staff の原則4か blocked.md に承認の段の表が無い"
grep -q '^2\. \*\*オーナーの go を取ってから\*\*起動する。.*委譲の範囲の中.*`projects/<slug>/AGENTS.md` の Chief of Staff の節が「起動にオーナーの go は要らない」' "$pb/start.md" \
  && pass "chief-of-staff の start.md の起動の go に、ルーチン・委譲・プロジェクトの設定の例外がある" || fail "chief-of-staff の start.md に起動の go の例外が無い"
! grep -q '続きをして。終わったら kanban' <<<"$cosall" && grep -q '続きをして。終わり方は worker skill §7（andon issue report → andon issue ready）' "$pb/watch.md" \
  && pass "chief-of-staff skill の担当への続きの文が worker skill §7 を指す" || fail "chief-of-staff skill の続きの文が古い"
