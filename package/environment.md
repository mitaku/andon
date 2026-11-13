# Andon の環境メモ（セル）

エージェントが毎セッション読む、セルの環境のこと。セルに固有のことはセルの `AGENTS.local.md`、プロジェクトの決まりは各リポジトリの `AGENTS.md` にある。書き方と語は `/opt/andon/writing.md` に従う（語の定義は用語集 `/opt/andon/skills/glossary/SKILL.md`）。

## いる場所

- セル（案件ごとの長寿命のコンテナ）の中。`hostname` がセルの名前（slug）。セルの repo は `/workspace`。
- リポジトリは `/workspace/.ghq` の下（`ghq list`・`ghq get github.com/<owner>/<repo>`）。`projects/` の下に clone を置かない。
- プロジェクトの台帳は `projects/<slug>/kanban/`。台帳は台帳の CLI（`kanban`）で読み書きする。
- このシェルは herdr のペインの中。端末は `hc`（`hc ls`・`hc get`・`hc read`・`hc send`）。長く走らせるものはペインか `hc run --bg` で起動する（一時的なシェルで起動したものは一緒に片付く）。skill に書いていない `hc` の操作が要るときだけ、先に `hc --skill` で手引きを読む（状態の意味・終了コード。版で変わるので写さない。毎回は読まない）。
- Andon の仕事の入口は `andon`（`andon help`）。パッケージは `/opt/andon`（読むだけ）。

## 道具と認証

- git は HTTPS。資格情報は `gh` の token（`gh auth setup-git`）。SSH 鍵を作らない・持ち込まない。
- 認証はセルの中で取る。人が見るブラウザは無いので、URL が出たら人に見せて手元で開いてもらう。device code があればそれを使う。ホストからトークンを持ち込まない。
- CLI は aqua（セルの `local/aqua.yaml`。イメージの `/opt/andon-aqua/aqua.yaml` は触らない）、言語ランタイムはリポジトリの `mise.toml`。`apt install` は最後の手段。
- Playwright のブラウザはイメージに焼いてある（`PLAYWRIGHT_BROWSERS_PATH`）。入れ直さない。
- 担当が画面を見て確かめるには `playwright-cli`（焼いた Chromium を使う。使い方は skill `playwright-cli`）。dev サーバは `127.0.0.1` で立てて開く。
- 秘密をファイルに書かない（`.env` にも）。認証の資格情報は道具が自分の置き場に置く（`gh auth login`・`claude auth login`・`andon signal auth`）。自分でファイルに書かない。

## 人に見せる

- dev サーバはコンテナの中で `127.0.0.1` に listen し、`portless alias <app> <port>` で名前を付ける。人は `http://<app>.<セルの名前>.localhost:<PORTLESS_PORT>` で見る（番号はセルの `.env`）。`0.0.0.0` も compose の書き換えも要らない。
- フロントから別のアプリを呼ぶときも portless の名前で呼ぶ（`localhost:<port>` は人の手元を指す）。
- 生成した HTML などは `andon publish <file>` で `files/` へ出し、返った URL を伝える。
- 人に気づいてほしいことは `herdr notification show` で出す。herdr（TUI）が動いているときだけ効く（Herdr App で使う人には届かない）。無ければ台帳の問いとボードで足りる。

## 気づきを残す（auto memory）

- Claude Code の auto memory は `/workspace/memory/`（git の外）。全部の repo・役・セッションが、このフォルダと1つの `MEMORY.md`（目録）を共有する。Codex の記憶は切ってあり、Codex で動く役は報告の `## Remember` で残す。
- memory は蒸留までの受け皿。週1のトリアージで仕分けられ、wiki か repo へ出るか捨てられて、ファイルは消える。
- 1件1ファイルの `.md` を、フォルダの直下に書く（トリアージが拾うのは直下の `.md` だけ。`.md` 以外は許可の確認で止まる）。
- 書くのは、確かめた事実と、起きた条件と理由のついた学び。本文に課題番号（無ければ repo）を書く。
- 続き・決定・秘密は書かない。続きは台帳、決定は文書の PR へ。
- memory に書いても、報告の `## Remember` は今までどおり書く。

## 気をつけること

- 止めるときは `pkill -f` を使わない（自分のスクリプトにも当たる）。ポートから PID を引いて `kill` する。
- セルを動かしているマシン（ホスト）には出られない。コンテナの中で完結させる。
- `.ghq/` は git の外。セルの repo に commit しない。
