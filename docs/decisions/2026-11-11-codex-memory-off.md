# Codex の自動の記憶を boot で止める

- 日付: 2026-11-11
- 出典: [ADR-0007](../adr/0007-interim-gap-inventory.md) の C5・R9。A-0072。

Codex 0.166.0 には記憶の生成と利用を止める設定がある。boot が `features.memories`・`memories.generate_memories`・`memories.use_memories` を `false` にする。既存の `true` も覆す。セルの外から見えない記憶を育てず、学びは台帳と wiki に残すため。

設定を再生成する場合は元のファイルを退避し、ほかの設定の値と権限は保持する。既存の記憶ファイルは削除しない。設定の意味と退避の扱いは [image.md](../image.md) に記す。

信頼した repo の project 層は user 層より優先されるため、配布する Codex の担当・レビュアーのプロファイルでも、通常起動と再開の両方に3設定を無効化する CLI 引数を付ける。boot の user 設定と合わせて、作業する repo の設定で記憶が有効に戻ることを防ぐ。
