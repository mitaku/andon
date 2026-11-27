# ADR-0068: マージの予約を合図に、dispatcher が main を取り込ませてからマージする

- 日付: 2026-11-27
- 出典: ADR-0068（[adr/0068-reserve-then-update.md](../adr/0068-reserve-then-update.md)。Chief Engineer の課題。元は ADR-0067 の「やらないこと」の 1 と 2 と、ADR-0065 決めたこと 2）
- 覆す: [ADR-0056](../adr/0056-gate-required-before-merge.md) 決めたこと 15（repo の GitHub の auto-merge を切る）と、同 16 の切り替えの手のうち auto-merge を切る手（一部。印を必須にすることと、抜け道を作らないことは生きている）。記録は [2026-11-23-adr-0056-gate-required-before-merge.md](2026-11-23-adr-0056-gate-required-before-merge.md)。

オーナーのマージを待つ PR は、main にほかの PR が入るたびに、担当の取り直し（rebase して検査と報告をやり直すこと）になっていた。5 日で、衝突の無い取り直しが 51 回、合計 356 分である。古い base の PR を差し戻す gate の守りは、オーナーのマージには届かず、オーナーがマージした 27 本のうち 13 本が古い base のまま入っていた。main の ruleset に base の最新を求める設定を入れ、遅れた PR のマージは GitHub に断らせる。オーナーは、遅れた PR では GitHub の auto-merge を予約する。予約を見た dispatcher が、GitHub の API で main を PR の branch に merge で取り込ませる。担当は呼ばず、担当の commit は書き換えない。取り込みの形の head は、報告とレビューと gate の問いを引き継ぎ、gate が新しい head で判定し直して緑の印を書くと、GitHub がマージする。取り込みは repo ごとに 1 本ずつ進める。仕組みが働くのは、ruleset が base の最新と印 `andon/gate` の両方を求める repo だけで、ほかの repo は今の守りのまま動く。GitHub の auto-merge は切らず、gate が初めて通す head に緑を書く前に予約を外し、dispatcher が生きていてはいけない予約を外す。衝突を解く取り直しは、今のまま担当が解き、オーナーが見る。予約した PR は、オーナーの判断待ちの一覧に「予約済み」と出し、判断待ちの数から外して別に数える。実装の前に、設計が頼る GitHub の動きを、捨てる前提の repo で実測する。

捨てた案:

- **コードを変えず、Chief of Staff の手引きから衝突の無い rebase の頼みを外す**: 古い base のまま main に入ることを受け入れることになる。守りが入る前に、main の検査が 2 回赤になった。
- **今のボタンのまま、main が動くたびに dispatcher がマージ待ちの PR 全部に main を取り込ませる**: 待つ PR の数だけ、main が動くたびに検査が回る。続けて押す 2 本目は、検査が終わるまで押せない。
- **予約を合図にして、担当が 1 回だけ rebase する**: 担当がマージ 1 本につき 1 回動き、報告と問いが出し直しになる。
- **base の最新を GitHub に求めさせず、main が動いたら待つ PR の印を黄にして止める**: 印が黄になる前に押せる。続けて押す 2 分以内のマージが通り抜ける。
- **GitHub の merge queue に任せる**: organization が持つ repo だけの機能で、個人のアカウントが持つ repo では使えない。
- **dispatcher がセルの中で rebase して force push する**: 担当の commit を書き換え、報告とレビューが指す commit が消える。
- **base の最新だけを求める repo でも仕組みを働かせる**: 印が必須でないと、取り込んだ head を gate が判定し直す前に GitHub がマージする。
- **差分が同じ merge commit なら取り込みとして引き継ぐ**: 衝突を手で解いた merge も当てはまる。自動で合わせた結果と同じ、の条件を足した。
- **gate の問いが指す head を新しい head に書き換える**: オーナーの判断待ちの一覧が、その問いを古い問いとして飛ばす。
- **積んだ課題の載せ替えを dispatcher に移す**: 載せ替えは rebase で、積む起動と載せ替えの口がまだ無く、回数の数も無い。担当の口のままにする。
- **モデルの検査器を TLC に替えて、積んだ課題と予約の重なりも CI で調べる**: 検査器を選んだのはオーナーで、この設計は検査を 2 つの写しに分ければ足りる。重なりは検討の中で 1 度調べた。
