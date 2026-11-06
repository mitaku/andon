# Andon の環境メモ（建屋）

エージェントが毎セッション読む、建屋の環境のこと。建屋に固有のことは建屋の `AGENTS.local.md`、プロジェクトの決まりは各リポジトリの `AGENTS.md` にある。用語は `/andon:glossary` に従う。

## いる場所

- 建屋（案件ごとの長寿命のコンテナ）の中。`hostname` が建屋の名前（slug）。建屋の repo は `/workspace`。
- リポジトリは `/workspace/.ghq` の下（`ghq list`・`ghq get github.com/<owner>/<repo>`）。`projects/` の下に clone を置かない。
- プロジェクトの台帳は `projects/<slug>/kanban/`。台帳は台帳の CLI（`kanban`。`deck` は同じもの）で読み書きする。
- このシェルは herdr のペインの中。端末は `hc`（`hc ls`・`hc get`・`hc read`・`hc send`）。長く走らせるものはペインか `hc run --bg` で起こす（一時的なシェルで起こしたものは一緒に片付く）。
- Andon の仕事の入口は `andon`（`andon help`）。パッケージは `/opt/andon`（読むだけ）。

## 道具と認証

- git は HTTPS。資格情報は `gh` の token（`gh auth setup-git`）。SSH 鍵を作らない・持ち込まない。
- 認証は建屋の中で取る。ブラウザは無いので、URL が出たら人に見せて手元で開いてもらう。device code があればそれを使う。ホストからトークンを持ち込まない。
- CLI は aqua（建屋の `local/aqua.yaml`。イメージの `/opt/andon-aqua/aqua.yaml` は触らない）、言語ランタイムはリポジトリの `mise.toml`。`apt install` は最後の手段。
- Playwright のブラウザはイメージに焼いてある（`PLAYWRIGHT_BROWSERS_PATH`）。入れ直さない。
- 秘密をファイルに書かない（`.env` にも）。

## 人に見せる

- dev サーバはコンテナの中で `127.0.0.1` に listen し、`portless alias <app> <port>` で名前を付ける。人は `http://<app>.<建屋の名前>.localhost:<PORTLESS_PORT>` で見る（番号は建屋の `.env`）。`0.0.0.0` も compose の書き換えも要らない。
- フロントから別のアプリを呼ぶときも portless の名前で呼ぶ（`localhost:<port>` は人の手元を指す）。
- 生成した HTML などは `andon publish <file>` で `artifacts/` へ出し、返った URL を伝える。
- 人に気づいてほしいことは `herdr notification show` で出す。

## 気をつけること

- 止めるときは `pkill -f` を使わない（自分のスクリプトにも当たる）。ポートから PID を引いて `kill` する。
- 建屋を動かしているマシン（ホスト）には出られない。コンテナの中で完結させる。
- `.ghq/` は git の外。建屋の repo に commit しない。
