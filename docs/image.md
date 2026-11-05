# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は版つきのバイナリを直接取得し、固定 SHA-256 を確認してから配置する。版と checksum の出典は [公式 manifest](https://downloads.claude.ai/claude-code-releases/2.1.318/manifest.json)。建屋固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

台帳の CLI `kanban`（`cmd/kanban/`。P3c）は同じ repo からビルドして入れる。`tests/image.sh` が linux の静的バイナリを焼き、build context `kanban`（`docker build --build-context kanban=<dir>`）で渡して `/usr/local/bin/kanban` に COPY する。`deck` は1版だけの別名（`/usr/local/bin/deck` → `kanban` の symlink）。private release の取得や取得用 token は使わない（人の決定 2026-11-05、decisions.md §9）。

`andon`（`cmd/andon/`）も同じく焼き、build context `andon` で渡して `/opt/andon/bin/andon` に COPY する（`/usr/local/bin/andon` はそこへの symlink）。ホストは同じバイナリを image から取り出して使う（[operate.md](operate.md) の 2）。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon`・`andon help` と主要な CLI を確かめる。同じ image の actionlint・ghalint・pinact で イメージ配布 workflow（tests.yml）の構文・権限・固定 SHA も検査する。続けて建屋を立てる道を通す: image から取り出した `andon` で `andon init` → `bin/bootstrap up`（`FROM andon:test`）→ compose の起動で `andon boot` が `ready` まで走り、managed settings・プロファイル・`CLAUDE.md` の2行（`environment.md` と `AGENTS.local.md`）・skill のリンクができたことを確かめる。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## CI と配布

PR は読み取り権限だけでビルドとテストを行う。main の push では検査済み image を artifact で publish job に渡し、`ghcr.io/<owner>/andon:<40桁のcommit SHA>` に push する。`vX.Y.Z` の tag では main に含まれる commit かを検査し、既存の SHA image に版 tag だけを付ける。tag の実行ではビルドせず、SHA image が無ければ失敗する（main の配布完了後に tag を出す）。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更は人が行う。

GHCR への push は publish job だけに付与する Actions の `GITHUB_TOKEN`（`packages: write`）を使う。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu は digest、Python・uv tool は直接指定する版を固定する。

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（人の決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
