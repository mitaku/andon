---
読む時: 振り返りの課題を起動する・報告（通知 `<id> report`）を受ける・`andon retro apply` を打つとき（オーナーが振り返りを頼んだときも）と、振り返りが切った課題を起動するとき
消す条件: 足す・起動する・当てる・続けるの段は「`andon retro apply` が当てたあと finish と足した課題の起動まで回し、オーナーへの1行を出すようになったら」
---

# 振り返り

ふりかえり（Retrospective）のワークストリームの AI の手。流れの正は andon の repo の `process/workstreams/retrospective.yaml` で、このファイルはやり方だけを書く。
拾い方と判定は retro skill、パターンの一覧の形と command が断る線は `contract.md` §21 にあり、ここには書かない。

1. **足す**: ルーチン `retro`（`routines/retro.yaml`。例は retro skill の `assets/retro.yaml`。kanban へ写すのはオーナーの go で。名前は `retro` のまま）が毎朝1件足す。片付いた担当の課題が `project.json` の `chief_of_staff.retro_every` 件（既定 25）たまると、dispatcher が数えて同じルーチンを撃つので、朝を待たずに足されることもある。
   枠が空いたままのときは、ルーチンが足した課題を除いた片付いた担当の課題が `retro_every` の半分以上で、片付いていない振り返りが無ければ、dispatcher が同じルーチンを撃つ（`idle.md`。ADR-0047 決めたこと 8）。
   オーナーが頼んだら `kanban routine run retro`（`routine: retro` が付き、前の振り返りとして数えられる。`open` と `paused` はそのまま効く）。
2. **起動する**: オーナーに聞かずに `andon issue start <id>`（`start.md`。枠〈`andon waiting` の `slots.free`〉とぶつかりは守る）。
3. **報告が届いたら**: 通知は `<id> report`（PR の無い課題なので `review` の通知は来ない。報告を書いたときと `review` に入ったときの2度届きうる）。課題が `doing` のままなら待つ。`review` に入っていれば、報告の表と `issues/<id>/library/retro.json` を読み、`andon retro apply <id> --dry-run` を打つ。検収で見ること:
   - パターンが根拠の課題で本当に起きたか（command は根拠の数と実在と新しさしか確かめない）。直すパターンごとに、根拠の課題の記録を1件以上開いて読む（現地現物。`fix` の数に上限は無く、検収が量を抑える最後の段）。
   - 段が上から選ばれているか。`skill`・`text` の `not_upstream` が具体的か。
   - `summary` と依頼書に、オーナーが打った文の本文・オーナーの名前が入っていないか（PR の要約の元になる）。
   - `fix` と `drop` の理由が読めるか。`drop` の理由が数の都合（多すぎる・枠が無い）になっていないか。`fix` が優先の順に並んでいるか。前の直しの見立てが「再発したのに文を足す」になっていないか。
   読めない・合わないなら、担当に `hc send` で戻す（`andon retro patterns` から打ち直させる）。
4. **当てる**: 自分の端末で `andon retro apply <id>`（オーナーの課題に置いておく印を書くので、ほかの端末では何も書かずに断る）。終了コード 1 は理由を読んで担当に戻す。記録は `library/retro-applied.json`（途中で落ちても、打ち直せば残りから続ける）。
   当てたら `andon issue finish <id>`（PR の無い課題。端末を片付けて done にする）。
5. **続ける**: `apply` が足した直す課題（`parent` が振り返りの課題）は、予約キーを既定（`review=off`・`merge=person`・`estimate=medium` など）で持って足される。`profile` を書き、既定から変える Settings だけ書き直してから（`brief.md`・`cut.md`。Andon のパッケージと `AGENTS.md` に触れる直しは、既定の条件表でレベル C。委譲の `merge` が C でなければオーナーがマージ）、`start.md` の手順で起動する。依頼書の本文は `apply` が書いたものを書き換えない。足りなければ追記する。差分はレベル B・C になるので、`review=off` のままでも gate がレビューを求める。依頼書に「レビュー: 差分はレベル B・C なので、Review: off でも worker skill の §4 を Review: on と同じに通す」と追記する（`verify.md` の雛形の `レビュー:` の行と同じ）。
   オーナーの課題（タイトルが「Andon へ伝えるか: …」。このプロジェクトが Andon の repo を持たないとき）は、`apply` が決まった理由の印 `later` を書くので、置いてある行になって行燈を灯さない。返事で1問ずつ聞かない（ask-one-by-one §5。オーナーが「今見る」と言えば印を消して聞く）。
   改善の PR のラベル `retro` が repo に無ければ、最初の1回に `gh label create retro --repo <owner>/<repo>` で作る。
6. **オーナーへ**: 知らせる（オーナーがいるターンならその返事で。いなければ kanban の問いとボードで足りる）。「振り返りでパターンを a 件見つけた。直す課題を b 件、オーナーの課題を c 件切った。d 件は捨てた。」。捨てたものはオーナーが `retro.json` で見て戻せる（戻す＝課題に切る）。空きで dispatcher が撃った回も同じ返事に 1 文にまとめる（`idle.md` の「オーナーへ」）。
   前にオーナーへ知らせたあとに自動でマージされた振り返りの直し（`parent` が振り返りの課題）があれば、PR の番号とタイトルを1行で並べる。skill に触れたものと、振り返り自身を直したものは、そう書き添える。オーナーは止めず、あとから読む。
7. **止める**: ルーチンの `paused: true` はオーナーが書く。見る日（ADR-0028 の価値の仮説）に、止める条件に当たればオーナーに伝える。
