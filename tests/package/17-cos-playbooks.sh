# tests/package/17-cos-playbooks.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  17. CoS の原則と手引き（A-0195）: SKILL.md は役・原則・手引きの目次・手引きの扱い・規約の行き先だけで 10KB 以下、目次の手引きが全部在り、手引きの先頭に「読む時」と「消す条件」。
# === 17. Chief of Staff の原則と手引き（A-0195） ===
# SKILL.md（毎ターン system prompt に載る）は役・原則・手引きの目次・手引きの扱い・規約の行き先だけで 10KB 以下。
# 目次の手引きは全部在り、在る手引きは全部目次に載り、どれも先頭に「読む時」と「消す条件」を持つ。template は SKILL.md を載せたまま、手引きの置き場を --add-dir で足す。
miss=
[ "$(wc -c <"$cos")" -le 10240 ] || miss="$miss [SKILL.md が $(wc -c <"$cos") バイト]"
got=$(grep '^## ' "$cos" | tr '\n' '|')
[ "$got" = '## 原則（モデルや command が替わっても守る）|## 手引き（状況が来たら読む）|## 手引きの扱い|## 規約1〜10 の行き先|' ] || miss="$miss [SKILL.md の節: $got]"
for i in 1 2 3 4 5 6 7 8 9 10 11; do grep -q "^$i\. \*\*" "$cos" || miss="$miss [原則 $i]"; done
grep -q '`/opt/andon/skills/chief-of-staff/playbooks/`' "$cos" || miss="$miss [手引きの置き場]"
listed=$(sed -n '/^## 手引き（状況が来たら読む）$/,/^## 手引きの扱い$/p' "$cos" | grep -o '| `[a-z-]*\.md` |$' | tr -d '|` ' | sort | tr '\n' ' ')
have=$(cd "$pb" && ls -1 *.md | sort | tr '\n' ' ')
[ -n "$have" ] && [ "$listed" = "$have" ] || miss="$miss [目次と手引きが食い違う: 目次 $listed／在る $have]"
for f in "$pb"/*.md; do
  [ "$(sed -n 1p "$f")" = '---' ] && [ -n "$(fm "$f" 読む時)" ] && [ -n "$(fm "$f" 消す条件)" ] || miss="$miss [$f の先頭に 読む時・消す条件 が無い]"
done
for n in $(seq 1 10); do grep -q "^| $n\. .* | \`[a-z]*\.md\` |$" "$cos" || miss="$miss [規約 $n の行き先]"; done
case " $(fm package/templates/chief-of-staff.md command) " in *" --append-system-prompt-file /opt/andon/skills/chief-of-staff/SKILL.md "*) ;; *) miss="$miss [template が SKILL.md を載せていない]" ;; esac
# 手引きは Read で読む。/opt/andon は作業の場の外で、--add-dir が無いと Read が許可の確認で止まる（A-0195 の実測）
case " $(fm package/templates/chief-of-staff.md command) " in *" --add-dir /opt/andon/skills/chief-of-staff/playbooks "*) ;; *) miss="$miss [template に手引きの --add-dir が無い]" ;; esac
[ -z "$miss" ] && pass "chief-of-staff の SKILL.md は原則と目次だけで 10KB 以下、目次の手引きが全部在り、どれも 読む時・消す条件 を持つ" || fail "chief-of-staff の原則と手引き:$miss"

# ADR-0026 受け入れ条件 A1〜A4（案 1）: PR と文書の書き方の決まりは、セルの外の読み手に通じるように
w=package/skills/worker/SKILL.md; ce=package/skills/chief-engineer/SKILL.md
sec3=$(sed -n '/^## 3\. 検査・commit・PR$/,/^## 4\./p' "$w")
chk3() { grep -qF -- "$2" <<<"$sec3" && pass "ADR-0026 A1: worker §3 に $1" || fail "ADR-0026 A1: worker §3 に $1 が無い"; }
chk3 "タイトル・commit に課題番号を書かない" 'PR のタイトル・commit のタイトル・commit の本文に入れない'
chk3 "メモの行の形" '`Memo: 内部の課題番号 <id>`（repo が英語なら `Memo: internal issue <id>`）'
chk3 "6つの見出しと必須の3節" '必須は「変更理由」「やったこと」「検証結果と確かめ方」の3節'
for h in 変更範囲 トレードオフ 影響範囲; do grep -q "^     | $h | あれば |" <<<"$sec3" || fail "ADR-0026 A1: worker §3 の表に $h が無い"; done
chk3 "長い物だけ details に畳む" '長い物だけ `<details>`／`<summary>` に畳む'
chk3 "状態の図" '変更前と変更後の状態の図（Mermaid）'
chk3 "ほかの PR は URL か owner/repo#n" '`owner/repo#n`'
chk3 "セルの物を指さず案件の repo は Andon の用語を書かない" '案件の repo では Andon の用語'
# ADR-0026 A2
grep -qF 'PR 本文の「トレードオフ」と報告の「見送り」にも写す' "$w" && pass "ADR-0026 A2: worker §4 の見送りの写し先は トレードオフ" || fail "ADR-0026 A2: worker §4 の見送りの写し先が トレードオフ でない"
# ADR-0026 A3
d3=$(sed -n '/^# <タイトル>（仕様・設計。日付）$/p' "$ce" docs/design.md | wc -l)
[ "$d3" = 2 ] && ! grep -q 'Chief Engineer の課題 id）' "$ce" docs/design.md \
  && grep -qF 'オーナーの発言の日付だけ' "$ce" && grep -qF 'オーナーの発言の日付だけ' docs/design.md && grep -qF 'オーナーの発言の日付だけ' docs/decisions.md \
  && ! grep -qF '`sources/` のページか `raw/` の元資料' "$ce" \
  && pass "ADR-0026 A3: CE skill・design.md §1.8・decisions.md の出典はタイトルに課題 id を求めず、セルの wiki と kanban を指さない" || fail "ADR-0026 A3: 文書の形か出典の書き方が古い"
# 否定の検査が当たる場合があること: 古い形を持つ文字列では上の検査が落ちる
! printf '%s\n' '# <タイトル>（仕様・設計。日付・Chief Engineer の課題 id）' | grep -q '^# <タイトル>（仕様・設計。日付）$' && pass "ADR-0026 A3: 古いタイトルの行は新しい形に当たらない（検査が見ている）" || fail "ADR-0026 A3: 検査が何も見ていない"
# ADR-0026 A4
grep -q '^- 番号だけで指さない。この決まりはセルの中のオーナーへの文に効く。PR のタイトルや本文・commit・repo のファイルの文は' package/writing.md \
  && pass "ADR-0026 A4: writing.md の番号の決まりに範囲（セルの中のオーナーへの文。PR と repo の文は除く）がある" || fail "ADR-0026 A4: writing.md の番号の決まりに範囲が無い"
