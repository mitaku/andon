# tests/*.sh は tests の workflow だけで回し、settings の帰属は attribution、ENABLE_TOOL_SEARCH は settings の env の1か所にする

- 日付: 2026-11-14
- 出典: 課題 A-0204（A-0118 の見送り3件）

- `tests/andon*.sh`・`tests/kanban.sh` を go の workflow で回すのをやめる。tests の workflow（`tests.yml`）の tests の job が `tests/*.sh` を全部回すので、二重になっていた。release の job も tests の job と同じく、先に andon・kanban を焼いて `ANDON_BIN`・`KANBAN_BIN` で渡す（前は渡さず、`tests/*.sh` が1本ずつ runner の go で焼いていた）。`workflow_test.go` が確かめる。
- Go の版は `mise.toml` の go に揃える。`go.mod` の go と、`image/Dockerfile` の段 bin の golang を `tests/image.sh` の冒頭（docker が無くても走る）で突き合わせる。
- `tests/host.sh` は `tests/kanban.sh` と同じく、node が無ければ手元では skip し、CI では落ちる。
- `package/settings.json` の帰属は `"attribution": false` で消す。`includeCoAuthoredBy` は Claude Code v2.0.62 から非推奨で、`false` の値は v2.1.281 から受け付ける（image の Claude Code は 2.1.322）。
- `ENABLE_TOOL_SEARCH=auto` は managed settings の env だけに置き、`image/Dockerfile` の ENV と sshd の SetEnv から外す。managed settings の env は Claude Code の起動のしかた（docker exec・ssh）によらず読まれる。`auto` は、未設定のとき（MCP の道具の定義をいつも後から読み込む）と働きが違うので残す。
- allow の `Bash(git diff)`・`Bash(git status)` を外す。末尾の「 *」が唯一の `*` の規則は素の命令にも当たるので、`Bash(git diff *)`・`Bash(git status *)` で足りる。セルの Claude Code 2.1.325 で、allow の `Bash(gh pr checks:*)` が素の `gh pr checks` を通し、allow に無い `gh pr list` を断ることを確かめた。重なる行が戻らないことは `tests/package.sh` が見る。
