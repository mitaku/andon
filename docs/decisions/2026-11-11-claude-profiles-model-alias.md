# Claude のプロファイルはバージョンを書かず別名（fable・opus・sonnet）で起動し、Fable は既定の effort にする

- 日付: 2026-11-11
- 出典: オーナーの依頼（2026-11-11）。課題 A-0109

- `package/templates` の Claude のプロファイル（`chief-of-staff`・`claude`・`sonnet`・`fable`・`chief-engineer`・`claude-review`）の `--model` は、バージョンを書かず別名（`opus`・`sonnet`・`fable`）にする。新しいバージョンは別名が解けるので、バージョンアップのたびに template を書き換えない。
- `fable`・`chief-engineer` は `--effort` を外し、モデルの既定で起動する。ほかのプロファイルの `--effort`（`claude-review` の medium）は今のまま。
- Codex のプロファイルは触らない。
