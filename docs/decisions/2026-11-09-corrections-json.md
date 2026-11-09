# 人の手直しは dispatcher が done のときに corrections.json へ1回だけ集め、人の commit は email でなく「最後の報告より後」で見分ける

- 日付: 2026-11-09
- 出典: 人の依頼（変える前に測る）・設計書 §1.9 の (b)・タスク A-0003

design.md §1.9 の (b) の表の 1〜3 を `issues/<id>/library/corrections.json`（[contract.md](../contract.md) §14・`schema/corrections.schema.json`）に集める。設計書に無かった細部を次のとおり決めた。

- **合図と回数**: `log.jsonl` の `{"status": "done"}` の行（`andon issue finish`。マージの影響 §2.5 と同じ合図）。ファイルが在れば書かない（それが「集めた」の印）。レビュアーの課題には書かない。
- **「無かった」と「集めていない」**: 手直しが無ければ空の配列で書き、読めなければ書かない。打ち直すのは次の見回りの1回だけで、それも駄目なら新しい通知の種類 `corrections`（`detail` は `error` だけ）を Chief of Staff へ積む。
- **人の commit**: PR の commit のうち `report.json` の `commit`（担当の最後の報告）より後のもの。author の email では見分けない: 担当も repo の持ち主の GitHub noreply アドレスで commit するので、email では人と担当が分かれず、持ち主の email の書き方（`<数>+<login>@…` と `<login>@…`）でも揺れる。
- **人のコメント**: PR の会話欄のコメント・レビューの本文・インラインのレビューコメント（設計の表の 2「レビューのコメント」）のうち、`andon gate: ` で始まらないもの（ゲートは人と同じ token でコメントを書く）。インラインでは `user.type` が `Bot` のものも除く。会話欄とレビューの本文は `gh pr view` が bot を見分ける項目を返さないので除かない（多めに拾う）。
- **答えが推奨案と違う**: 台帳の答え（自由文）が推奨案の `label` と字で違うもの。意味は読まない（同じ意味の言い換えも拾う。多めに拾い、振り返りが捨てる）。ゲートの問い（`from: gate`）は推奨どおりでも `override` に入れ、`answer` には入れない。範囲の問い（`kind: scope`）は Chief of Staff の答えなので拾わない。
- **人が覆した指摘**: マージした PR の `waived` のうち `must_fix` 以上（Chief of Staff が書いた `must_fix`。無ければ全部）。ゲートは担当がレビュアーを覆したものを `needs_human` にするので、マージされたなら人が通した。

捨てた案:

- **author の email が持ち主のものを人の commit にする**（レクチャーの書き方）: 上のとおり担当の commit も入る。
- **答えを選択肢の label に照らして `picked` を選択肢の名前にする**: 自由文の答えを読むことになる（contract.md §0 の原則1）。
