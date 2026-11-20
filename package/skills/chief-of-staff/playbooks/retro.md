---
読む時: 振り返りの課題を起動する・`review` を受ける・`andon retro apply` を打つとき（オーナーが振り返りを頼んだときも）と、振り返りが切った課題を起動するとき
消す条件: 足す・起動する・当てる・続けるの段は「`andon retro apply` が当てたあと finish と足した課題の起動まで回し、オーナーへの1行を出すようになったら」
---

# 振り返り

ふりかえり（Retrospective）のワークストリームの AI の手。流れの正は andon の repo の `process/workstreams/retrospective.yaml` で、このファイルはやり方だけを書く。
拾い方と判定は retro skill、パターンの一覧の形と command が断る線は `contract.md` §21 にあり、ここには書かない。

1. **足す**: ルーチン `retro`（`routines/retro.yaml`。例は retro skill の `assets/retro.yaml`。kanban へ写すのはオーナーの go で。名前は `retro` のまま）が毎朝1件足す。
   オーナーが頼んだら `kanban routine run retro`（`routine: retro` が付き、前の振り返りとして数えられる。`open` と `paused` はそのまま効く）。
2. **起動する**: オーナーに聞かずに `andon issue start <id>`（`start.md`。並行の上限とぶつかりは守る）。
3. **`review` が来たら**: 報告の表と `issues/<id>/library/retro.json` を読み、`andon retro apply <id> --dry-run` を打つ。検収で見ること:
   - パターンが根拠の課題で本当に起きたか（command は根拠の数と実在と新しさしか確かめない）。直すパターンごとに、根拠の課題の記録を1件以上開いて読む（現地現物。`fix` の数に上限は無く、検収が量を抑える最後の段）。
   - 段が上から選ばれているか。`skill`・`text` の `not_upstream` が具体的か。
   - `summary` と依頼書に、オーナーが打った文の本文・オーナーの名前が入っていないか（PR の要約の元になる）。
   - `fix` と `drop` の理由が読めるか。`drop` の理由が数の都合（多すぎる・枠が無い）になっていないか。`fix` が優先の順に並んでいるか。前の直しの見立てが「再発したのに文を足す」になっていないか。
   読めない・合わないなら、担当に `hc send` で戻す（`andon retro patterns` から打ち直させる）。
4. **当てる**: `andon retro apply <id>`。終了コード 1 は理由を読んで担当に戻す。記録は `library/retro-applied.json`（途中で落ちても、打ち直せば残りから続ける）。
   当てたら `andon issue finish <id>`（PR の無い課題。端末を片付けて done にする）。
5. **続ける**: `apply` が足した直す課題（`parent` が振り返りの課題）は、ほかの課題と同じ決まりで Settings を書いてから（`brief.md`・`cut.md`。Andon のパッケージと `AGENTS.md` に触れる直しはレベル C で、オーナーがマージ）、`start.md` の手順で起動する。依頼書の本文は `apply` が書いたものを書き換えない。足りなければ追記する。
   オーナーの課題（題が「Andon へ伝えるか: …」。このプロジェクトが Andon の repo を持たないとき）は ask-one-by-one で1問ずつ。
   改善の PR のラベル `retro` が repo に無ければ、最初の1回に `gh label create retro --repo <owner>/<repo>` で作る。
6. **オーナーへ**: 知らせる（オーナーがいるターンならその返事で。いなければ kanban の問いとボードで足りる）。「振り返りでパターンを a 件見つけた。直す課題を b 件、オーナーの課題を c 件切った。d 件は捨てた。」。捨てたものはオーナーが `retro.json` で見て戻せる（戻す＝課題に切る）。
   前にオーナーへ知らせたあとに自動でマージされた振り返りの直し（`parent` が振り返りの課題）があれば、PR の番号と題を1行で並べる。skill に触れたものと、振り返り自身を直したものは、そう書き添える。オーナーは止めず、あとから読む。
7. **止める**: ルーチンの `paused: true` はオーナーが書く。見る日（ADR-0028 の価値の仮説）に、止める条件に当たればオーナーに伝える。
