# measure の先頭「価値: 未計測」を価値の行に替える（ADR-0011 決めたこと 18）

- 日付: 2026-11-10
- 出典: [ADR-0011](../adr/0011-signals-and-value-loop.md) 決めたこと 18・切る案 S8・課題 A-0029。[2026-11-09 の決定](2026-11-09-measure-flow-metrics.md)が「価値のループができてから入る」と置いた枠を埋める

`andon measure` の先頭は、価値のループ（観察の課題の `report.json` の `observation`）ができたので、「価値: 未計測」の1行を価値の行に替えた。並べ方は 価値 → 流れ → 効率のまま。`--json` に `value` を足した（[contract.md](../contract.md) §13・`schema/measure.schema.json`）。

- (a) **仮説を確かめた数**: 期間に報告された観察の課題の数と、`observation.result` の内訳（`hit`・`miss`・`unknown`）。
- (b) **題から結果を見るまでの日数**: Chief Engineer の課題が最後の列へ動いた時刻から観察の報告まで（中央値の日数）。
- (c) **価値の仮説「無し」の題の数**: 期間に最後の列へ動いた Chief Engineer の課題のうち、観察の課題が無いもの。

新しい記録は足さない。今ある記録（`log.jsonl`・`issue.yaml`・`report.json`）から導く。読むだけ・何も書かない・保存しないは変わらない。

ADR に書かれていなかったので、ここで置いたこと:

- **観察の課題と Chief Engineer の課題を結ぶのは `issue.yaml` の `parent`**（台帳の既存のキー。新しいキーは足さない）。観察の課題を足す Chief of Staff の skill〈ADR-0011 S7〉が `--parent <Chief Engineer の課題>` で足す前提。`parent` が Chief Engineer の課題でない観察の報告は (a) には数えるが (b) の日数には入れない。
- **報告した時刻**は、観察の課題が `log.jsonl` で最初に `review`（なければ最後の列）へ動いた時刻（`report.json` に時刻が無いため）。
- **観察の課題の見分け**は、`parent` が Chief Engineer の課題で、`report.json` に `observation` がある非レビュアーの課題。通常の epic の子課題は観察の課題に数えない（`not_before` は一般の準備時期のキーなので識別に使わない）。そのため観察の報告が出る前の題は (c) に入り、報告が出ると外れる。不正な `observation.result` は数えず `skipped` に出す。
- **(c) は「観察の課題が無い」ことで導く**。文書の価値の仮説は Markdown で機械が読まない（ADR 決めたこと 12）ので、「無し」と書いたかは見ない。観察の課題をまだ足していない done の題も入る。
