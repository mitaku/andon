# 待ちを除いた実働を数える関数と段→分の表は internal/ledger に置く（ADR-0014 E2）

- 日付: 2026-11-11
- 出典: ADR-0014 決めたこと 5・11 の 1・2・7、切る案 E2。課題 A-0061

- 実働の関数（`ledger.Worked`）と段→分の表（`ledger.EstimateMinutes`。small 30・medium 60・large 120）は `internal/ledger` に置く。ADR の切る案は `internal/dispatcher` と書いたが、読み手は dispatcher（E3 の通知）と `andon measure`（E5 の記録）の2つで、measure は dispatcher を import しない。両方が既に読む ledger に置けば、表も数え方も1つで済む。予約キー `estimate` の値の確かめ（`SettingsOf`）も同じ表を読む。
- 関数は台帳の `log.jsonl` の行・課題の一覧（レビュアーの子課題を `parent` と `profile` で選ぶ）・Chief of Staff のアドレス・dispatcher が Gone を見た時刻の列・今の時刻を受け取る。Gone の時刻を控えに書くのは E3（dispatcher の通知）の範囲で、この関数は受け取るだけ。
- 「最初の列に戻ったら数え直し」は、status が `todo` か `queued`（doing より前の列）の行で、それまでの doing の区間を捨てることとして読む。
