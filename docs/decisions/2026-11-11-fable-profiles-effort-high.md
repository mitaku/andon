# fable と chief-engineer のプロファイルは effort high を明示して起動する

- 日付: 2026-11-11
- 出典: 人の判断（2026-11-11 夕）。課題 A-0129。`2026-11-11-claude-profiles-model-alias.md` の「effort は既定で起動する」を覆す

- `package/templates` の `fable`・`chief-engineer` は、`command` と `resume` に `--effort high` を明示して起動する。`--model` は別名（`fable`）のまま。
- ほかのプロファイル（`claude`・`sonnet`・`claude-review`・Codex 系）は触らない。
