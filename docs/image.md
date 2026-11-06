# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は版つきのバイナリを直接取得し、固定 SHA-256 を確認してから配置する。版と checksum の出典は [公式 manifest](https://downloads.claude.ai/claude-code-releases/2.1.318/manifest.json)。建屋固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

台帳の CLI `kanban`（`cmd/kanban/`。P3c）は同じ repo からビルドして入れる。`tests/image.sh` が linux の静的バイナリを焼き、build context `kanban`（`docker build --build-context kanban=<dir>`）で渡して `/usr/local/bin/kanban` に COPY する。`deck` は1版だけの別名（`/usr/local/bin/deck` → `kanban` の symlink）。private release の取得や取得用 token は使わない（人の決定 2026-11-05、decisions.md §9）。

`andon`（`cmd/andon/`）も同じく焼き、build context `andon` で渡して `/opt/andon/bin/andon` に COPY する（`/usr/local/bin/andon` はそこへの symlink）。ホストは同じバイナリを image から取り出して使う（[operate.md](operate.md) の 2）。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon`・`andon help` と主要な CLI を確かめる。同じ image の actionlint・ghalint・pinact で イメージ配布 workflow（tests.yml）の構文・権限・固定 SHA も検査する。対話の bash（`bash -ic`・`bash -lic`）で補完・`HISTFILE`・プロンプト・fzf のキー割り当てが効くことと、`rg`・`fzf`・`jq` が動くことも見る。続けて建屋を立てる道を通す: image から取り出した `andon` で `andon init` → `bin/bootstrap up`（`FROM andon:test`）→ compose の起動で `andon boot` が `ready` まで走り、managed settings・プロファイル・`CLAUDE.md` の2行（`environment.md` と `AGENTS.local.md`）・skill のリンクができたことを確かめる。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## 建屋の bash（対話）

シェルは bash のまま。設定は `image/bashrc`（image の `/etc/andon-bashrc`。`~/.bashrc` の末尾が読む）に1か所にまとめてある。ssh のログインシェル（`~/.profile` → `~/.bashrc`）・`bin/bootstrap enter`・App の端末のどれでも同じに効く。

- **道具**: ripgrep（`rg`）・fzf・jq。aqua で版と checksum を固定している（`image/aqua/`）。fzf は Ctrl-R（履歴検索）・Ctrl-T（ファイル）・Alt-C（cd）が効く。mise の補完が呼ぶ `usage` も入っている。
- **Tab 補完**: bash-completion（apt）と、git・gh・hc・andon・deck（kanban）・just・mise。gh・just・mise は各 CLI の生成物、hc は `hc --help` の一覧から image を焼くときに作る（hc に生成の口が無い）、andon・kanban・deck はサブコマンド名だけ（`image/completions/`。フラグは補わない）。どれも `/usr/share/bash-completion/completions/` に置き、最初の Tab を待たずに登録する。
- **履歴**: `~/.local/state/bash/history`。`state` の volume の下なので、作り直しても残る。追記式・大きめの上限・重複と先頭空白の行は残さない。
- **プロンプト**: `建屋の名前:ディレクトリ (git のブランチ)`。色は控えめ。

## Codex の承認と sandbox

建屋（コンテナ）が信頼の境界なので、Codex を承認の確認で止めない。boot が毎起動、`~/.codex/config.toml` の top-level に無いキーだけを先頭に足す（あるキーは値が何でもそのまま）:

- `approval_policy = "never"` — 確認の画面を出さない。
- `sandbox_mode = "danger-full-access"` — 担当とレビュアーは worktree の外（台帳の `projects/<slug>/.flightdeck` とその `.lock`。`deck report`・`deck review`・`library/`）に書き、`git push`（ネットワーク）と hc の socket を使う。`workspace-write` に `writable_roots`・`network_access` を足す形は、台帳の置き場が建屋ごとに違ううえ socket の扱いもあり、その割に建屋の外へは届かない（境界はコンテナ）ので採らない。Claude Code も建屋では sandbox なしで動く。

Claude Code の managed settings の deny と同じ線は、Codex の execpolicy の rules で引く。`package/codex.rules` を boot が `~/.codex/rules/andon.rules` に写す（人の rules は別のファイルに書く）。今は `gh pr merge` を `forbidden`。rules は承認の方針の前に引かれ、`forbidden` は `never` でも拒む（Codex 0.166.0 の [`codex-rs/core/src/exec_policy.rs`](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/core/src/exec_policy.rs) の `create_exec_approval_requirement_for_parsed_commands`。当たらないコマンドだけが `render_decision_for_unmatched_command` で `never` → Allow になる）。`bash -lc "a && b"` のような平らな script は1つずつに割って引く。`$(…)` などを含む script は割らないので当たらない（Claude Code の deny も頭で引くので同じ線）。deny のうち `Bash(git diff *--output*)` などは allow の穴を塞ぐもので、allow を持たない Codex には引かない。2か所の一覧は `tests/package.sh` が突き合わせる。Codex 自身の危険の判定（`rm -f` など）は `never` の下では確認ではなく拒否になる。

Codex のフックは配っていない（フックは Claude Code だけ。[contract.md](contract.md)）ので、フックの信頼の画面は出ない。

`tests/image.sh` は、モデルを呼ばずに `codex debug prompt-input` の permissions の文（実効の設定から組む）と `codex execpolicy check` で確かめる。

## CI と配布

PR はビルドとテストだけを行い、push しない。main の push では、tests の job が通った同じ job の中で、検査済み image（`andon:test`）を `ghcr.io/<owner>/andon:<40桁のcommit SHA>` に push する。image を artifact で受け渡さない（GitHub のストレージ枠を使うため）。`vX.Y.Z` の tag では main に含まれる commit かを検査し、既存の SHA image に版 tag だけを付ける。tag の実行は publish job が行い、ビルドせず、SHA image が無ければ失敗する（main の配布完了後に tag を出す）。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更は人が行う。

GHCR への push は tests の job（main の push の step だけ）と publish job（tag）に付与する Actions の `GITHUB_TOKEN`（`packages: write`）を使う。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu は digest、Python・uv tool は直接指定する版を固定する。

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（人の決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
