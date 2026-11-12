# auto memory を managed settings で入れ、`allow` は足さない。今あるセルの `.gitignore` は operate.md の1行で直す

- 日付: 2026-11-11
- 出典: ADR-0020 決めたこと 6・8・9、受け入れ条件 2・6・7。A-0144 の実測（Claude Code 2.1.322: `autoMemoryDirectory` の下の `.md` への Write/Edit は cwd に関係なく自動で許可され、サブフォルダは切られない）。課題 A-0146

- `package/settings.json` は `autoMemoryEnabled: true`・`autoMemoryDirectory: "/workspace/memory"` で、`env` の `CLAUDE_CODE_DISABLE_AUTO_MEMORY` は外した。memory の設定を書くのはここだけ。
- `permissions.allow` に `/workspace/memory/**` は足さない。置き場の `.md` は Claude Code が自分で書き込みを許すので、確認は出ない（A-0144 の (c)）。`deny` は変えない。`tests/package.sh` は、allow に memory の置き場が入っていないことも見る。
- 環境メモ（`package/environment.md`）の「気づきを残す（auto memory）」に、決めたこと 6 の書き方と、A-0144 で分かった2つ（`.md` をフォルダの直下に書く・本文に課題番号を書く）を置いた。
- 今あるセルの `.gitignore` は、`docs/operate.md` §2 の1行（`memory/` を足す）で直す。CHANGELOG は版を切るときだけ書くので、版を切るときに「上げるときにすること」へ写す。
