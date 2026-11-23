# tests/package/35-cos-idle.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置き、cos・pb は前の節が置く）。
#  35. 枠が空いたままの通知 idle の手（ADR-0047 受け入れ条件 11。決めたこと 13・14・18）: 手引き idle.md があり、SKILL.md の目次と watch.md の <news> の表に
#      idle の行がある。idle.md が決めたこと 14 の 7 つの手を順に持つ。triage.md・retro.md の「足す」が空きで撃つことを書き、wiki.md の lint が idle を
#      きっかけにし、verify.md が検証の課題と check-verify の課題を同時に動かさない。当たる場合があることを同じ所で確かめる。
# === 35. 枠が空いたままの通知 idle の手（ADR-0047） ===
idle_row='^| 通知 `idle`（枠が空いたまま） | `idle.md` |$'
news_row='^| `idle: 枠が N つ空いたまま。…` | .* | `idle.md`'
miss=
[ -f "$pb/idle.md" ] || miss="$miss [idle.md が無い]"
sed -n '/^## 手引き（状況が来たら読む）$/,/^## 手引きの扱い$/p' "$cos" | grep -q "$idle_row" || miss="$miss [SKILL.md の目次に idle の行]"
sed -n '/^## `<news>` の読み方$/,/^## 留守のあと/p' "$pb/watch.md" | grep -q "$news_row" || miss="$miss [watch.md の <news> の表に idle の行]"
[ -z "$miss" ] && pass "ADR-0047 A11: 手引き idle.md があり、SKILL.md の目次と watch.md の <news> の表に idle の行がある" || fail "ADR-0047 A11:$miss"

# idle.md の「受けたら」が決めたこと 14 の 7 つの手を、この順に持つ
steps=$(sed -n '/^## 受けたら（この順）$/,/^## オーナーへ$/p' "$pb/idle.md" 2>/dev/null)
miss=
n=0
for want in '`andon waiting --json`' '`start.md` のとおりに起動する' 'Chief Engineer に渡す' 'オーナーに 1 回だけ言う' '残った枠の中で起動する' '`library/triage/`' 'Chief Engineer の課題' '`kanban lint` を打って読む' '`wiki.md` の lint の条件' '`andon news ack` を打って黙る'; do
  at=$(grep -nF -- "$want" <<<"$steps" | sed -n 1p | cut -d: -f1)
  if [ -z "$at" ]; then miss="$miss [$want が無い]"; elif [ "$at" -lt "$n" ]; then miss="$miss [$want の順]"; else n=$at; fi
done
for i in 1 2 3 4 5 6 7; do grep -q "^$i\. \*\*" <<<"$steps" || miss="$miss [手 $i]"; done
grep -qF '`chief_of_staff.idle_minutes` に 0' "$pb/idle.md" || miss="$miss [止める設定]"
[ -z "$miss" ] && pass "ADR-0047 決めたこと 14: idle.md が読み直す・起動されていない課題・空きが足した課題・持ったまま・lint・wiki の lint・ack の 7 つの手を順に持つ" || fail "ADR-0047 決めたこと 14: idle.md の手:$miss"

# ほかの手引きを揃える（決めたこと 13・14・18）
miss=
sed -n '/^1\. \*\*足す\*\*/,/^2\. /p' "$pb/triage.md" | grep -q '枠が空いたままのとき.*dispatcher が同じルーチンを撃つ（`idle.md`' || miss="$miss [triage.md の足す]"
sed -n '/^1\. \*\*足す\*\*/,/^2\. /p' "$pb/retro.md" | grep -q '枠が空いたままのとき.*dispatcher が同じルーチンを撃つ（`idle.md`' || miss="$miss [retro.md の足す]"
grep -q '^- \*\*lint\*\*: .*通知 `idle` を受けたターン（`idle.md` の 6）' "$pb/wiki.md" || miss="$miss [wiki.md の lint のきっかけ]"
grep -q '^読む時: .*通知 `idle`' "$pb/start.md" || miss="$miss [start.md の読む時]"
grep -q '検証の課題.*`check-verify`.*同時に動かさない' "$pb/verify.md" || miss="$miss [verify.md の check-verify]"
[ -z "$miss" ] && pass "ADR-0047 決めたこと 13・14・18: triage.md・retro.md の足すが空きを書き、wiki.md の lint と start.md が idle を読み、verify.md が check-verify と検証を同時に動かさない" || fail "ADR-0047 ほかの手引き:$miss"

# 検査が当たる場合があること: 行の無い目次と表では上の切り出しが当たらない
! printf '%s\n' '## 手引き（状況が来たら読む）' '| `<news>` が届いた | `watch.md` |' '## 手引きの扱い' | sed -n '/^## 手引き（状況が来たら読む）$/,/^## 手引きの扱い$/p' | grep -q "$idle_row" \
  && ! printf '%s\n' '## `<news>` の読み方' '| `memory: …` | … | `triage.md` の 1 |' '## 留守のあと' | sed -n '/^## `<news>` の読み方$/,/^## 留守のあと/p' | grep -q "$news_row" \
  && printf '%s\n' '| `idle: 枠が N つ空いたまま。…` | 空いた | `idle.md`（起動する） |' | grep -q "$news_row" \
  && pass "ADR-0047 A11: idle の行の無い目次と表では検査が落ち、ある表では当たる（検査が見ている）" || fail "ADR-0047 A11: idle の行の検査が何も見ていない"
