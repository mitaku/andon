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

## 信頼の画面（Claude Code・Codex）

建屋では信頼の画面（trust this folder）を出さない。建屋＝コンテナが信頼の境界なので、boot と、起こす直前の `andon task start`・`andon chief-engineer` が、建屋の repo（`/workspace`）と、台帳に載った案件の repo の根（`projects/*/.flightdeck/config.yaml` の `repos` にあり、`/workspace/.ghq/<host>/<owner>/<repo>` に clone されたもの）を信頼済みにする。ghq の下の repo を全部は信頼しない（台帳に載せるのは Chief of Staff と人）。Claude Code は `$CLAUDE_CONFIG_DIR/.claude.json` の `projects["<根>"].hasTrustDialogAccepted: true`、Codex は `~/.codex/config.toml` の `[projects."<根>"] trust_level = "trusted"`。足りない所だけ足し、今ある値には触らない。

足すのが根だけで足りる理由は、効く範囲の実測（image の版。使い捨ての HOME で各条件を起こし、最初の画面だけを読んだ）:

| 信頼した所 → 起こした所 | Claude Code 2.1.318 | Codex 0.166.0 |
|---|---|---|
| git の外の親 → その下の dir | 効く（`/` まで親を遡る） | 信頼の画面がもともと出ない |
| git の repo の根 → repo の中の dir | 効く | 効く（画面に「Trusting will apply to the repository root」と出る） |
| repo の根より上の親 → repo の中の dir | 効かない（根で止まる） | 効かない |
| 元の repo の根 → その worktree | 効く | 効く |
| worktree の置き場（`~/.herdr/worktrees`）→ worktree | 効かない | 効かない |

`tests/image.sh` が、建屋の claude と codex を `/workspace` の下の新しい dir で起こしても信頼の画面が出ないこと（対照に `/tmp` の新しい repo では出ること）を確かめる。版を上げたら同じ表を測り直す。

## CI と配布

PR はビルドとテストだけを行い、push しない。main の push では、tests の job が通った同じ job の中で、検査済み image（`andon:test`）を `ghcr.io/<owner>/andon:<40桁のcommit SHA>` に push する。image を artifact で受け渡さない（GitHub のストレージ枠を使うため）。`vX.Y.Z` の tag では main に含まれる commit かを検査し、既存の SHA image に版 tag だけを付ける。tag の実行は publish job が行い、ビルドせず、SHA image が無ければ失敗する（main の配布完了後に tag を出す）。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更は人が行う。

GHCR への push は tests の job（main の push の step だけ）と publish job（tag）に付与する Actions の `GITHUB_TOKEN`（`packages: write`）を使う。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu は digest、Python・uv tool は直接指定する版を固定する。

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（人の決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
