# tests/

`for f in tests/*.sh; do bash "$f"; done` で全部回る。CI は `tests/lib/run-tests.sh` で並べて回し、1本ごとの秒を表に出す（`tests/ci.sh` がその道具と、image の検査を飛ばしてよい PR の判定 `tests/lib/image-scope.sh` を見る。[docs/image.md](../docs/image.md) の「CI の所要時間」）。

要るのは bash・python3（標準ライブラリだけ）・jq・Go（`mise.toml` の版）・Node.js（手元の推奨版は `mise.toml`、CI は runner に同梱の版）。`tests/andon*.sh` は `andon` を `go build` で一時ディレクトリに焼いて回す（`ANDON_BIN` があればそれを使う。CI の tests の workflow が渡す）。go が PATH に無ければ `mise exec` で焼く。`tests/lib/run-tests.sh` は並べる前に `mise install` を一度打つ（入っていない go を並べた `mise exec` が同時に入れると、入れ終わる前の go を exec して落ちる）。`tests/andon-boot.sh` は偽の HOME・偽のセル・偽のパッケージで `andon boot` を2回打ち、生成（信頼の設定を含む）と冪等を見る（PATH に hc を置かないので起動しない）。`tests/andon-init.sh` は `andon init x` でセルの repo を作り、偽のイメージ（`package/` を一時ディレクトリの `opt/andon` に写したもの。`ANDON_IMAGE_ROOT`）で `bin/bootstrap --dry-run` と `host-install`（偽の HOME）を回す（docker を PATH に置かない）。`tests/andon-issue.sh`・`andon-gate.sh`・`andon-triage.sh` は台帳の CLI（`kanban`）を `tests/lib/kanban.sh` で一時ディレクトリに焼いて PATH の先頭に置く（`KANBAN_BIN` があればそれを使う。CI の tests の workflow が渡す）。`tests/andon-issue.sh` の start は bare な origin とその clone・偽の hc で、worktree と branch・`hc run`・`kanban start`・最初のメッセージが届かないときの案内・finish での片付けまでを見る。`tests/andon-chief-of-staff.sh` も同じ `kanban` と偽の hc で、起動する・二重に起動しない・`chief_of_staff` が直る・2人なら止まるを見る。`tests/andon-dispatcher.sh` も同じ `kanban` と偽の hc・gh で、Chief of Staff が台帳に1行も書かずに review-request → レビュアー → ゲート → マージ → finish が通るか（と fix・needs_human・merge: person・上限を超えた回・マージ前の読み直し・レビュアーの起動の再試行・finish の片付け）を見る。`tests/kanban.sh` は herdr-flightdeck の `deck-v0.4.0` の振る舞いテスト（`tests/kanban/test-deck.mjs`。写しのまま。node が要る）を `kanban` に当て、`kanban --help` に別名 `deck` が無いこと・`--version` を見る。Go の単体テストは `go test ./...`。`tests/package.sh` は `package/` の skill・プロファイル（顔ぶれ・frontmatter・仮設の語が残っていないこと）・`gate.json`（schema に合う）・`settings.json`・statusline を見る。skill に書いた機械向けの手順（`report.json`・問いの JSON・`issue.yaml` の予約キー）は、skill の塊をそのまま抜いて偽の kanban で回し、schema に通す。`tests/schema.sh` は `schema/` と `docs/contract.md` の例を突き合わせる（検査器は `package/lib/andon/schema_check.py`。schema に新しいキーワードを使うなら、先に検査器に足す。pattern には ECMA-262 と Python で意味が割れる `.`・`\s`・`\d`・`\w`・`\b` を使わず、明示した ASCII の文字クラスで書く。lint が落とす）。`tests/schema.sh` は制約を緩めた schema の写しで自分を回す自己検査を持つので、2分ほどかかる。

ほかの検査（細部は各 `.sh` の先頭のコメント）: `tests/andon-create.sh`（`andon create`）・`tests/andon-impact.sh`（`andon issue impact`）・`tests/andon-measure.sh`（`andon measure` の入口・表・終了コード）・`tests/andon-signal.sh`（`andon signal collect`・`auth`）・`tests/andon-dispatcher-signal.sh`（dispatcher の巡の、外の信号の段）・`tests/process.sh`（流れと状態の定義 `process/` と `schema/process/`）・`tests/sshd-env.sh`（sshd の `SetEnv` と Dockerfile の `ENV`）・`tests/words.sh`（置き換えた語が戻っていないか。`tests/words-selftest.sh` がその自己検査）・`tests/public-selftest.sh`（`tests/public.sh` の自己検査）・`tests/smoke.sh`（型を示す1本）。

## セルの本物から切り離す

検査はセルの本物の台帳と HOME に書かない。`tests/*.sh` は `tests/lib/andon.sh`・`kanban.sh` を読むと `tests/lib/isolate.sh` も読み、一時ディレクトリから上の git の repo を探さないようにして（`GIT_CEILING_DIRECTORIES`）、受け継いだ `KANBAN_ROOT`・`CLAUDE_CONFIG_DIR`・`CODEX_HOME`・`ANDON_HOME`・`XDG_STATE_HOME`・`XDG_CONFIG_HOME`・`XDG_DATA_HOME`・`HC_*` を外し、`ANDON_WORKSPACE`・`GHQ_ROOT` は一時ディレクトリの中の無い場所へ向ける（外すと既定がセルの本物を指す）。台帳の CLI は、上に `kanban/` が無いと git の main checkout から上の `projects/*/kanban` を探すので、TMPDIR が repo の中にあると、一時ディレクトリで打った検査がセルの台帳に届く（[decisions/2026-11-11-tests-isolated-from-cell.md](../docs/decisions/2026-11-11-tests-isolated-from-cell.md)）。

確かめるのは `tests/isolation [<tests/x.sh>...]`（手で打つ。全部で十数分）。偽のセル（番兵の台帳と HOME と、セルの repo の根・`GHQ_ROOT`）を作り、`KANBAN_ROOT`・`HOME`・`CLAUDE_CONFIG_DIR`・`XDG_STATE_HOME` などを番兵へ、TMPDIR を repo の写しの中に向けて `go test ./...` と `tests/*.sh` を回す。番兵が1つでも変われば、変わったものを出して落ちる（go が自分で書く `~/.config/go` の telemetry は数えない）。`ANDON_TEST_SKIP=tests/image.sh` で image の検査を飛ばせる。

## public 検査の禁止リスト

`tests/public.sh` は、秘密の形に加えて、**固有の値の禁止リスト**を全ファイルに当てる。リストの中身（セル名・ユーザー名・案件名など）が git の履歴に残らないよう、**リストは repo の外に置く**。

- 手元: `${ANDON_PUBLIC_DENYLIST:-$HOME/.config/andon/public-denylist}`
- CI: secret `ANDON_PUBLIC_DENYLIST`（中身を env で渡し、ワークフローが一時ファイルに書いて読む）。secret は人が置く。
- リストが無いときは、秘密の形だけを検査し、「禁止リストが無いので固有の値は見ていない」と出して通る。**public にする前の検査（design.md §5.3 の 8）は、必ずリストを置いて回す。**

書き方: 1 行に 1 語。大文字小文字は区別しない。`#` で始まる行と空行は無視。例:

```
# セル名
example-building
# 案件名
example-project
```

## image の検査

`tests/image.sh` は Docker があれば実際にビルドしてコンテナを起動する。セルの claude と codex を `/workspace` の下の新しい dir で起動し、信頼の画面が出ないことも見る（最初の画面は `tests/lib/pty-probe.py` で pty に起動して読む。標準ライブラリだけ）。手順と CI の配布条件は [docs/image.md](../docs/image.md)。Docker が無いローカルでは SKIP、CI では失敗する。

`tests/host.sh` は実物の mod を Node.js の VM に読み（node が無ければ手元では skip、CI では落ちる。`tests/kanban.sh` と同じ）、偽の `herdr`・`gpui-kit` API で名前付け・同名の無操作・レビュアーの除外・別マシンの配置・rename 以外を呼ばないことを検証する（`tests/host/mod.mjs`）。画面の mod（inbox・kanban-board・context-header・files-plus）は `tests/host/screen.mjs` が、JavaScript の構文（`node --check`）・`mod.json`・`keys.json`・レイアウト `andon` の名前付けの block、Andon に合わせた所（context-header の名前・kanban-board の台帳の見つけ方）を見る。`tests/andon-init.sh` は mod とレイアウトの初回配置、既存ディレクトリ・レイアウト・リンクの保護、App の環境変数による置き場と WSL の明示指定も確かめる。本物の App の描画とレイアウトへの配置は、このテストには含まれない。
