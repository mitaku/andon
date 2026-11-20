# 振り返りの入力は、境目より後の束を全文で、前の束を要約で出し、入力の形を retro/2 に上げる

- 日付: 2026-11-20
- 出典: [ADR-0039](../adr/0039-retro-scale-with-flow.md) 決めたこと 6 と「実装が決めてよいこと」
- 覆す: [2026-11-19-retro-collect.md](2026-11-19-retro-collect.md) の「入力は1つの JSON」のうち「要約と本文には分けない」と、「束に入れる記録」のうち起動の記録と結果の記録の写し方

ADR-0039 が実装に任せた細部を次のとおり決めた。形と欄の意味は [contract.md](../contract.md) §20・§21・`schema/retro.schema.json`。

- **境目の置き場**: `previous_retro.until`（前の振り返りの `retro.json` の `until`。無い・形が違う・片付いた時刻より後なら `null`）と、境目そのものの `boundary` を入力に足す。`andon retro patterns` の「新しい出来事」も `boundary` で照らす。
- **要約の束の欄**: `summaries[]` の1件は `id`・`title`・`done_at`・`pr`・`findings`・`gate_fixes`・`corrections`・`remember`。指摘の1行目は、レビュアーの報告で指摘の `<file>:<line>` を含む最初の行（字下げした次の行が続けば足す）とする。`findings.json` は場所と重さしか持たないため。手直しの有無は、ゲートの問いをオーナーがマージして閉じた答え（「人がマージした」と、今の版の「オーナーがマージした」）を除いて数える。行は 160 字で切る。
- **全文を引く口**: `andon retro collect --issue <id>`。`issues[]` の1件と同じ形の object を1つ出す。期間の担当の束でなければ終了コード 1。
- **全文の束の重い欄**: 起動の記録は `at`・`profile`・`model`（起動行の `--model` か `-m`）・`commit`、結果の記録は `result`・`worked_minutes`・`cost`（セッションの数・コストを読めた数・トークンの合計）にする。写しの意味が変わるので、`schema/retro.schema.json` の版を 2 に上げた。読み手は振り返りの担当だけで、古い版を読む口は残さない。
- **100 件の切り**: `cut` に `{"new": <境目より後の束の数>, "full": 100}` を書く。
- **前の再発の数**: `fixes[].recurred_before`。前の振り返りの `retro.json` の `fixes[].recurred`、無くてマージが境目より後なら `0`、どちらでもなければ `null`。`andon retro patterns` は `recurred` がこの数より少ないものを断る（足し算の数え方を command でも守らせる）。
