# ADR-0038: Chief Engineer は状態を動かす設計を statechart のモデルで調べ、pstack と yomiyasu を全セルに配る

- 日付: 2026-11-20
- 出典: ADR-0038（[adr/0038-statechart-in-chief-engineer-design.md](../adr/0038-statechart-in-chief-engineer-design.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-20「アイデア：反論を回す際にstatechart使ってもらう、もしくは反論を回す前にstatechartを使う　とよいかも？」）

Chief Engineer は、状態を動かす流れそのものを変える設計のとき、案をオーナーに見せる前に、TLA+ のモデルと規則を repo の外に書いて pstack の statechart で調べる。反論のサブエージェントにもモデルを渡す。調べ終えたモデルは課題の資料として開発へ渡し、担当が実装の PR で repo の `model/` に入れる。課題の一生に触れる設計では repo のモデルの写しに案を足し、statechart と、`model/check` に足す下書きを調べる口（repo と同じ検査器と「行ける」の形の規則）の 2 つで調べる。

そのために、pstack と、文の推敲の skill の yomiyasu を、パッケージの既定に参照で書いて全部のセルに配る。バージョンは固定せず自動で最新に上げる（オーナーが選んだ。Chief Engineer は固定を推した）。yomiyasu の「依頼書・wiki のページ・課題の報告の 3 種類を推敲する」決まりは `package/writing.md` に書く。pstack で標準にするのは「入っていて、名指されたら使える」までで、役への指示として決めたのは Chief Engineer の statechart だけ。

捨てた案は、反論のサブエージェントにモデルを書かせる、オーナーが案を選んだあとに調べる、設計文書の PR で repo にモデルを入れる、セルごとの設定に参照を書く、バージョンをパッケージで固定する、image に Java を焼く。効いたかは、当てはまるテーマで決定のあとに見つかる順番の穴の数を 2026-12-04 に数えて見る。
