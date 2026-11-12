# measure に課題ごとの見込みと実働を足す

- 日付: 2026-11-12
- 出典: 課題 A-0064（[ADR-0014](../adr/0014-issue-estimate.md) の切る案 E5。決めたこと 9・11 の 6）

`andon measure` に、課題ごとの見込みと実働を足した。`--json` では `issues[]` と `estimate`（[contract.md](../contract.md) §13・`schema/measure.schema.json`）を出し、人向けの表では効率とトークンの間に「見込み」の節を置く。版は上げない。新しい記録のファイルは作らない。

ここで置いたこと:

- 見込みと実働は dispatcher と同じ数え方で数える。見込みは `ledger.Estimate`（最後に書いたのが Chief of Staff の予約キー `estimate`）で読み、段を分に直すのは `ledger.EstimateMinutes`、実働は `ledger.Worked` で数える。measure 用の数え方を別に作ると、通知のズレと記録のズレが食い違う。
- 出す課題は、期間に最後の列へ最後に動いた担当の課題。レビュアー・Chief Engineer・人の課題は除く（ADR-0014 決めたこと 8）。
- 実働が見込みの分以下（比が 1 以下）なら「収まった」と数える。割合の分母は見込みのある課題だけで、見込みの無い課題は `none` に別に数える。
- 端末が死んでいた区間は dispatcher の控え（`<slug>.json`）の `gone` から読む。dispatcher は `Gone` を課題が `done` になったときに捨てていたので、閉じた課題を数える measure には残らなかった。そこで `Gone` は `done` のあとも残し、`todo`・`queued` に戻ったときか台帳から消えたときに捨てるようにした。段の控え（`Estimate`）は今までどおり `done` で捨てる。控えが無いとき（別のマシン・初めての台帳）は、死んでいた区間を引かない。
- 担当の見込み `agent_estimate_minutes` は並べて出すだけで、比べない。比と収まった割合は Chief of Staff の見込みで数える（切る側の見込みの当たり外れを測るため。ADR-0014 決めたこと 2）。
