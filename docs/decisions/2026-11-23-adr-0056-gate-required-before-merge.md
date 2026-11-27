# ADR-0056: gate が判定するまで、PR をマージできないようにする

- 日付: 2026-11-23
- 出典: ADR-0056（[adr/0056-gate-required-before-merge.md](../adr/0056-gate-required-before-merge.md)。Chief Engineer の課題。オーナーが対話の中で決めた）
- 覆す: [ADR-0054](../adr/0054-gate-status-by-decision.md) 決めたこと 2 のうち、オーナー待ちでは印を書かないこと（一部）。同 3（前の判定の印が残るときだけ、オーナー待ちを黄で上書きする）。同 7 のうち、開いている PR のオーナー待ちの赤い印を直さないこと（一部）。記録は [2026-11-22-adr-0054-gate-status-by-decision.md](2026-11-22-adr-0054-gate-status-by-decision.md)。
- 覆された（一部。repo の GitHub の auto-merge を切ることと、切り替えの手のうちそれを切る手）: [2026-11-27-adr-0068-reserve-then-update.md](2026-11-27-adr-0068-reserve-then-update.md)

独立レビューや gate の判定が終わる前の PR が、4 日で 21 本マージされていた。gate の印（commit status `andon/gate`）を main の必須の check にし、gate が今の commit を判定するまで、GitHub がマージを断るようにする。印は、通したときとオーナー待ちが緑、差し戻しが赤、Chief of Staff が止めている間が黄で、判定の前は書かない。gate は、オーナーがマージする変更かどうかを、検査とレビューのあとに見る。オーナー待ちになるのは、検査とレビューが済んだ PR だけになる。Chief of Staff は `andon issue hold` で PR を止め、`andon issue unhold` で外す。止める command は、マージと同じ鍵の中で緑の印を黄に書き換えてから返る。止めは自動では外れない。dispatcher は、PR を読むたびに印をあるべき形に揃える。Chief Engineer の文書の PR には、渡されたときに dispatcher が緑を書く。印が無くてもマージできる抜け道は作らず、要るときはオーナーが ruleset の行を外す。ruleset に行を足すことと、repo の GitHub の auto-merge を切ることは、実装を当てたあとにオーナーがする。

捨てた案:

- **新しい名前の印を足して、そちらを必須にする**: 印の行が 2 つになり、読み方も 2 つになる。
- **PR を draft で出し、gate が判定したら外す**: 誰でも 1 押しで外せて、push のあとに隙間が残る。
- **gate の見る順は変えず、検査とレビューが済んだオーナー待ちだけを緑にする**: 押せないマージの依頼がオーナーに届く。緑かどうかを決める式が、gate の判定と別に要る。
- **kanban の待つ相手（`after`）で止め、理由の課題が済んだら自動で外す**: 答えが「直す」のとき、Chief of Staff が担当へ伝える前に PR が渡る。
- **止めるときはキーだけを書き、印は dispatcher の次の巡が直す**: モデルで調べると、その間に緑の印が残り、マージできた。
- **ruleset の越えてよい相手に管理者を足す**: 同じアカウントで動く dispatcher のマージも、印なしで通るようになる。
