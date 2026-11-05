# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は仮設と同じ latest チャネルの種を入れる。建屋固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit の名前付き context 対応）と GitHub CLI を用意し、release を読めるアカウントで認証する。

```sh
DECK_REPOSITORY='<owner>/herdr-flightdeck' bash tests/image.sh
```

`deck-v0.4.0` の Linux バイナリをダウンロードし、固定した SHA-256 を検証してから名前付き context で COPY する。token は Docker の ARG・ENV・context に入らない。実行後に取得ファイルを消す。amd64 と arm64 に対応し、ローカルの Docker サーバと同じ architecture でビルドする。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon` と主要な CLI を確かめる。P3a では `package/` が空なので `andon help` はまだ検査しない。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## CI と配布

PR はビルドとテストだけ。main の push では `ghcr.io/<owner>/andon:<40桁のcommit SHA>`、`vX.Y.Z` の tag では同じ SHA tag と版 tag を push する。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更は人が行う。

人が repository secret `DECK_RELEASE_TOKEN` に、release 元の private repository の Contents を読むだけの token を置く。release 元が同じ owner の `herdr-flightdeck` でない場合は repository variable `DECK_REPOSITORY` に `<owner>/<repo>` を置く。取得用 token は台帳 CLI をこの repo に移したら消す（decisions.md §7、F9）。GHCR への push は Actions の `GITHUB_TOKEN`（`packages: write`）だけを使う。

secret が渡らない fork の PR は private release を取得できず失敗する。台帳 CLI の移管でこの制約を取り除く。`pull_request_target` は使わない。
