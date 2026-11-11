# boot が同梱の skill を symlink で張り、外の skill の一覧を gh-skills.txt に置いて gh skill で入れる

- 日付: 2026-11-11
- 出典: ADR-0018 決めたこと 5・前提 4・受け入れ条件 2（S3。A-0111）

`andon boot` の生成が、イメージの備品の置き場（`ANDON_TOOLS`。既定 `/opt/andon-tools`）に同梱の skill（`playwright-cli`）を `<claude>/skills/playwright-cli` と `~/.agents/skills/playwright-cli` に symlink で張る。備品の置き場の下を指す symlink は boot のものとして張り替え・外し（備品に無くなった skill も外す）、実ディレクトリや別の先を指すものには触らない。

外の skill の一覧は、パッケージの `gh-skills.txt`（既定は注だけで空）とセルの `local/gh-skills.txt` を合わせたもの。1行に `<owner>/<repo> <skill>`、空行と `#` からの行は飛ばし、形の違う行はログに1行出して飛ばす。boot の起動（`Start`。ネットワークが要るので生成でなく起動の手）が、gh がログイン済みなら、置き場に同じ名前が無いものだけを `gh skill install <repo> <skill> --agent <agent> --scope user`（latest）で入れ、毎起動 `gh skill update --all` を打つ。一覧が空なら gh を打たない。gh が無い・ログインしていなければ飛ばして1行。

`gh skill install` の `--agent` は値を1つしか取らない（gh 2.103.3 で実測: `--agent claude-code --agent codex` は最後の codex だけに入る）ので、claude-code と codex で1回ずつ打つ。置き場に同じ名前があれば打たないのは、gh が「already installed」で rc 1 を返すのと、利用者の同名の skill を `--force` で潰さないため（入っているものは `update --all` が上げる）。gh の出力はログに出さない（失敗のときだけ最後の1行）。1回の打ち込みは 2 分で切る。
