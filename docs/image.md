# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は仮設と同じ latest チャネルの種を入れる。建屋固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

台帳の CLI は P3c で `kanban` を同じ repo からビルドして入れる。P3a では `deck` を焼かず、private release の取得や取得用 token は不要（人の決定 2026-11-05、decisions.md §9）。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon` と主要な CLI を確かめる。P3a では `package/` が空なので `andon help` はまだ検査しない。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## CI と配布

PR はビルドとテストだけ。main の push では `ghcr.io/<owner>/andon:<40桁のcommit SHA>`、`vX.Y.Z` の tag では同じ SHA tag と版 tag を push する。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更は人が行う。

GHCR への push は Actions の `GITHUB_TOKEN`（`packages: write`）だけを使う。
