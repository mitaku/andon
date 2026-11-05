# 建屋を立てて1本流す（手順）

- 状態: **手順**（2026-11-05 起草。[design.md](design.md) §5.2 の P7 の準備）。読み手は人。ホスト（docker と bash がある Linux か WSL）で打つ。
- 入力（正）: andon main の `README.md`・`init/`・`internal/plantinit`・`internal/boot`・`docs/image.md`・[courier.md](courier.md)・[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md)。コードを読んで書いた。**打って確かめたものではない**（実測は P7 で人がする。この文書の「通ったら見えるもの」と食い違えば、実測が正で、この文書を直す）。
- 書き方: `<owner>`（GitHub の持ち主）・`<版>`（image の tag）・`<email>` は自分の値に置き換える。建屋の名前（slug）は例として `home`。

## 0. 先に知っておくこと: いま通らない所が2つある

コードを読んで見つけた。各段に回り道を書いた。直しは末尾の「直しのタスク」。

| # | 通らない所 | どの段 | 回り道 |
|---|---|---|---|
| F2 | `andon chief-of-staff` の中身が無い（`中身はまだ無い`）。Chief of Staff の skill が最初に打つ `andon chief-of-staff --no-attach`（自分を台帳に載せる）も同じ | 9 | `hc run` で起こし、`kanban config set coordinator=<address>` を人が打つ |
| F3 | `andon task start` の中身が無い（`中身はまだ無い`）。Chief of Staff が担当を起こせない | 10 | 人が worktree を切って `hc run` し `kanban start` で結ぶ（`andon chief-engineer` の4段と同じ） |

（F1「image に `andon` が焼かれていない」と `package/environment.md` が無いことは直した。image が `/opt/andon/bin/andon` を持ち、`tests/image.sh` が CI の実ビルドで 2・3・5 の道を通す）

## 1. image を取る

**やること**: private の image を取る。GHCR の package は private なので、`read:packages` を持つ token が要る。

```bash
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
docker pull ghcr.io/<owner>/andon:<版>      # <版> は main の 40桁の commit SHA か vX.Y.Z
```

- **通ったら見えるもの**: `docker image ls ghcr.io/<owner>/andon` に行が出る。
- **落ちたら見る所**:
  - `denied`・`unauthorized` — token に `read:packages` があるか。組織の package なら SSO の承認。package の「Manage access」にこのアカウントがあるか。
  - `manifest unknown` — tag は main の配布完了後に出る（[image.md](image.md)）。40桁の SHA は GitHub の Actions の「tests」の publish job で確かめる。
  - `no matching manifest` — 配布は Linux amd64 だけ（[image.md](image.md)）。

## 2. 建屋の repo を作る（`andon init`）

建屋の repo は `andon init <slug>` が作る（雛形は `init/`。作ったあとは Andon に追従しない）。置き場は `~/src/andon-<slug>`（`andon-up` が `$HOME/src/andon-*` を探す。`ANDON_PLANTS` で変えられる）。

ホストの `andon` は image から取り出す（release は無い）。image の中の `andon` は linux の静的バイナリで、配布は amd64 だけ（ホストが Linux か WSL の amd64 なら、そのまま動く）。

```bash
mkdir -p ~/.local/bin ~/src
# 一時ファイルに取り出してから置き換える（取り出しが落ちても、前の andon を空にしない）
docker run --rm ghcr.io/<owner>/andon:<版> cat /opt/andon/bin/andon > ~/.local/bin/andon.new \
  && install -m 755 ~/.local/bin/andon.new ~/.local/bin/andon && rm ~/.local/bin/andon.new
# image の版と揃っているか（2つの sha256 が同じなら揃っている）
docker run --rm ghcr.io/<owner>/andon:<版> sha256sum /opt/andon/bin/andon ; sha256sum ~/.local/bin/andon
cd ~/src && andon init home --image ghcr.io/<owner>/andon:<版> --owner <email>
```

- image を上げたら取り出し直す（ホストの `andon` は image の版と揃える。揃っているかは上の sha256 の2行で比べる）。`~/.local/bin` が PATH に無ければ足す。
- `--owner` は `andon.toml` の `owner`。委譲のファイル（`delegation.json`）をこの email の commit だけが信じられるようにする。
- **通ったら見えるもの**: `~/src/andon-home/` に `Dockerfile`（`FROM` ＋ 2行）・`andon.toml`（`slug = "home"`）・`bin/bootstrap`・`local/`・`gate.local.json`・`AGENTS.local.md`・`wiki/`・`projects/`。標準出力に `建屋の repo を作った`。パッケージのファイルは無い（design.md §5.1 の 1）。
- **落ちたら見る所**: 取り出しの `docker run` が落ちた（`manifest unknown` など）— 1 の pull を先に通す。`~/.local/bin/andon.new` が残っていれば消す。`はすでにあり、空でない` — 上書きしない。別の `--dir` か、中身を退避する。`slug は英小文字・数字・-` — slug の綴り。
- 建屋の repo を git で持つなら `git init` して commit する（`.ghq/`・`.env` などは雛形の `.gitignore` が外している）。

## 3. 起動する（`bin/bootstrap`）

```bash
cd ~/src/andon-home
bin/bootstrap --dry-run     # docker を打たずに、打つものを出す
bin/bootstrap               # 初回: image を焼く → compose.yaml を取り出す → 起動 → herdr に入る
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）は建屋の `.env` に書く。**建屋を2つ以上立てるなら、ずらす**（ホストの同じ番号を取り合う）。書かなければ既定。

- **何が起きるか**: `Dockerfile` の `FROM` から `andon-home:local` を焼く（`local/setup.sh` を走らせる）→ image の `/opt/andon/compose.yaml` を `.andon/` に取り出す → `docker compose up -d` → ログに `[andon boot] ready` が出るまで待つ（最長5分）→ `herdr` を開く。もう一度打てば同じセッションに戻る。
- **通ったら見えるもの**: `bin/bootstrap status` で `dev` が Up。`bin/bootstrap logs` に `[andon boot] ready`（と portless・sshd・クーリエを起こした行）。herdr の画面が開く。
- **落ちたら見る所**:
  - `Dockerfile の FROM を埋める` — 2 の `--image`。
  - `docker デーモンに繋がらない` — docker が起きているか。
  - 5分待っても `ready` が出ず `準備の完了を確かめられなかった` — `bin/bootstrap logs` で boot の警告の行を見る。`andon help` が `command not found` なら `FROM` の image が古い（`andon` を焼く前の版）。
  - ポートが使用中 — `.env` の番号を変えて `bin/bootstrap` を打ち直す。
  - `local/setup.sh` が落ちる — `bin/bootstrap` の build のログ。建屋固有の追加（`local/aqua.yaml`・`local/setup.sh`）が原因。

## 4. ホストの道具を置く（`host-install`）

```bash
bin/bootstrap host-install
```

`andon-up`（PC を起こしたあとに全建屋を起こす）を `~/.local/bin` に、herdr の設定を `~/.config/herdr/`、ssh config を `~/.ssh/config.d/andon-home.conf` に、cloud-init の雛形を `.andon/` に置く。**今あるファイルは上書きしない**。

- **通ったら見えるもの**: `+ <path>`（新しく置いた）か `= <path>`（同じ）。違うものは `! <path> はすでにあり、雛形と違う`（触らない。入れ替えるなら退避してから打ち直す）。`~/.ssh/config` に `Include config.d/*` が無ければ、足す1行を案内する。
- **落ちたら見る所**: `.env の SSH_PORT・PORTLESS_PORT は数字だけ`。`andon-up` が見つからない — `~/.local/bin` が PATH にあるか。

## 5. 建屋に入って、立ち上がりを確かめる

```bash
bin/bootstrap               # herdr に入る。bash で入るなら: bin/bootstrap sh
```

中で（シェルのペインで）:

```bash
hostname                    # slug（home）
andon help                  # sub-command の一覧
hc ls                       # 端末の一覧。courier が出る
ls ~/.config/herdr-core/templates     # claude・sonnet・fable・codex・codex-review・claude-review・chief-of-staff・chief-engineer
ls -l ~/.agents/skills                # andon-<name> が /opt/andon/skills/<name> への symlink
cat ~/.claude/CLAUDE.md               # @ の2行（environment.md と AGENTS.local.md）
cat /etc/claude-code/managed-settings.json
```

- **通ったら見えるもの**: 上の全部。`hc get courier` が通る（クーリエは `andon courier` を常駐させた端末）。
- **落ちたら見る所**: どれも出なければ boot が走っていない（`bin/bootstrap logs`）。boot は失敗しても続ける作りなので、警告の行を探す。生成だけ失敗したときは `andon boot` を中で打ち直せば同じ結果になる（何度打っても同じ）。`hc` が無い・端末が起きない — herdr-core は初めて `hc` を叩くと起きる。

## 6. 中でログインする（建屋の中で取る）

ブラウザは建屋に無い。URL が出たら**手元のブラウザで開く**（建屋から開こうとしない）。トークンをホストから持ち込まない。

```bash
gh auth login               # GitHub.com → HTTPS → device code（URL とコードを手元で）
gh auth setup-git           # git の credential helper を gh にする（clone・push が通らないとき）
claude                      # 初回に /login。URL を手元で開き、コードを貼る
codex login --device-auth   # Codex を使うとき
```

ログインは volume（`claude`・`codex`・`gh` など）に残る。作り直しても消えない。

- **通ったら見えるもの**: `gh auth status` が Logged in。`claude` が対話に入る。`codex login status`。
- **落ちたら見る所**: `gh` のスコープ（`repo`・`workflow`・`read:packages`）。device code の期限切れは打ち直す。`claude` が毎回ログインを求める — `claude` の volume が付いているか（`bin/bootstrap sh` で `ls ~/.claude`）。

## 7. Andon の repo を迎える

```bash
ghq get github.com/<owner>/andon        # GHQ_ROOT=/workspace/.ghq の下に入る
ghq list
```

- **通ったら見えるもの**: `/workspace/.ghq/github.com/<owner>/andon`。
- **落ちたら見る所**: `Repository not found` — `gh auth status` と repo の権限。`could not read Username` — `gh auth setup-git`。

## 8. プロジェクト `andon` の台帳を作る

台帳はプロジェクトごとに `projects/<slug>/.flightdeck/`（git の外）。slug はプロジェクト名＝フォルダ名。

```bash
mkdir -p /workspace/projects/andon && cd /workspace/projects/andon
kanban init --prefix A --repo github.com/<owner>/andon
kanban list ; kanban lint
```

- **通ったら見えるもの**: `created …/.flightdeck`。`config.yaml` に `prefix: A` と `repos`。`kanban lint` が問題なし。
- **落ちたら見る所**: `already a ledger` — 作ってある（そのまま使う）。`--repo must be host/owner/repo` — 綴り。`checkClock` の失敗 — コンテナの時計。
- `projects/andon/coordinator.md`（人のファイル: 目標・並行の上限・独立レビューを入れる条件）は任意。無ければ並行の上限は 2。

## 9. Chief of Staff を起こす

**F2 の回り道（いまの形）**:

```bash
cd /workspace/projects/andon
hc run --bg --no-parent --name andon-cos --label role=chief-of-staff --label project=andon \
  --cwd /workspace/projects/andon chief-of-staff
hc ls                                       # andon-cos の address を控える
kanban config set coordinator=<address>      # クーリエはこの端末が live な台帳だけを見る
hc attach andon-cos                          # 画面に乗る
```

`andon chief-of-staff` が入れば、`projects/andon/` で `andon chief-of-staff` と打つだけになる（動いていれば使う・2人なら止まる・画面に乗る）。Chief of Staff の skill が最初に打つ `andon chief-of-staff --no-attach` は、F2 の間は落ちる。落ちたら、`config.yaml` の `coordinator` を人が合わせたと伝えて続けさせる。端末を起き直すと address が変わるので、そのたびに `kanban config set` を打ち直す。

- **通ったら見えるもの**: `hc ls` に `role=chief-of-staff project=andon` の端末。`config.yaml` の `coordinator` がその address。Chief of Staff が自分の skill（system prompt に入った役の手順）に従って台帳を読む。
- **落ちたら見る所**: `プロファイル … の template が無い` — 5 の `ls ~/.config/herdr-core/templates`（boot の生成）。起きるが skill が効かない — 10 の S4 の確かめ。クーリエがこの台帳を飛ばす — `coordinator` の端末が live でない（`hc get <address>`）。**初めて見る台帳は、クーリエが控えるだけで動かさない**（[courier.md](courier.md) §2）。1回の見回りのあとに効く。

## 10. 小さい PR を1本流す

人は依頼を1つ渡す。Chief of Staff が依頼書に切る。Andon 自身の小さい変更（レベル A か B。例: 文書の誤字・テストの追加）を選ぶ。依頼書の Settings は `Review: on`・`Merge: auto`（design.md §5.1 の 2）。ゲートの条件表の `repos` に `github.com/<owner>/andon` を入れておく（レベルの決めは人。建屋の `gate.local.json`）。

流れ（人が触らないところ）: 依頼書 → 担当を起こす → PR → `review-request` → クーリエがレビュアーを起こす → `ready`（レビュー）→ クーリエがゲート → approve → クーリエがマージ → `andon task finish`。

**F3 の回り道（いまの形）**: 担当を起こす `andon task start` が無いので、人が4段を打つ（`internal/chiefengineer` が Chief Engineer にしていることと同じ）。

1. 台帳に載せる: `kanban add "<題>" --repo github.com/<owner>/andon --profile claude --brief -`（依頼書は標準入力）→ id（`A-1`）。
2. worktree を切る: `git -C /workspace/.ghq/github.com/<owner>/andon worktree add --no-track -b andon/A-1-<短い名前> ~/.herdr/worktrees/andon/andon-A-1 origin/main`。
3. 起こす: `hc run --bg --name andon-a-1 --label task=A-1 --label project=andon --cwd <worktree> claude "タスク andon/A-1。依頼書は <brief.md の絶対パス>"`。
4. 結ぶ: `kanban start A-1`（`status: doing`・`agent` がその端末に）。

- **通ったら見えるもの**: 下の「§5.1 の確かめ方」の対応表。
- **落ちたら見る所**: 担当が `deck start` を打たない — ADR-0001 の S4（下）。PR が出ない・checks が落ちる — `gh pr checks`。`review-request` を拾わない — `hc read courier`・`~/.local/state/andon/courier/<slug>.json`。ゲートが `needs_human` — 台帳の問い（`kanban questions`、`from: gate`）と `tasks/<id>/library/gate/<head sha>.json` の `reasons`。マージされない — 条件表の `repos.<repo>.merge`。

## 11. クーリエの確かめ

```bash
hc get courier ; hc read courier                       # 見回りの記録
jq . ~/.local/state/andon/courier/home.json 2>/dev/null   # 控え（<slug> は台帳のプロジェクト名なら andon.json）
```

控えは `~/.local/state/andon/courier/<プロジェクト名>.json`（`XDG_STATE_HOME` があればその下）。待ちの通知は `pending`、フックで渡したものは `hook` に残る。通知は台帳と端末の状態から拾う（[courier.md](courier.md) §2.2・§2.3）。**通ったら見えるもの**: 10 の流れの各段（`review`・`ready`・`gate`・`merged`）が、Chief of Staff に `<news>` として1通にまとまって届く。

## 12. 実測する（P6b と ADR-0001 S4）

本物の Claude Code でしか確かめられないもの。**見る場所**を書く。落ちたら直しのタスクにする。

### ADR-0001 S4: 担当1つの最初の turn

[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md) の受け入れ条件 3。10 の担当（`--plugin-dir /opt/andon` と `--append-system-prompt-file …/worker/SKILL.md` で起きる）を、**最初の turn が終わる前に** `hc read andon-a-1` で見る。

- 最初のメッセージに「先に SKILL.md を読む」の行が無いのに、担当が**最初に `deck start <id>` を打つ**。
- `/andon:worker` の名で skill を持つ（担当の画面で `/` を打って候補を見る）。
- `ANDON_HOME=<checkout>/package` で起こした担当（開発中の回避の口）は、checkout の plugin と skill が入る（`hc templates` の本文の `/opt/andon` が `ANDON_HOME` に置き換わっている）。
- 落ちたら: boot の生成（5 の `~/.agents/skills`・template の本文）、`package/.claude-plugin/plugin.json`、template の `command`（`--plugin-dir`・`--append-system-prompt-file`）。

### P6b: フックの4つ（[courier.md](courier.md) §3・§4・§5）

| 見ること | どう起こすか | どこで見るか |
|---|---|---|
| **Stop の block** — turn 中の通知が、Stop で続きの turn として届く | Chief of Staff が長めの turn にいる間に、担当を `deck review` させて通知を積む | Chief of Staff の画面で、turn の終わりに自動で次の turn が始まり `<news>` を読む。控えの `hook` に渡した turn。担当側: `doing` で報告も問いも無いまま止まろうとすると block（`stop-check` の控え `~/.local/state/andon/stop-check/<slug>-<id>.json`） |
| **additionalContext** — 人が送信したとき、待ちが文脈に添えられる | 通知が積まれている間に、人が何か送る | Chief of Staff に「いま添えられた通知を言って」と聞く。待ちが無くても `<delegation project="…">` が毎回添えられる |
| **離席時だけ `hc send`** — 入力欄が空で idle のときだけ送る | Chief of Staff を idle にし、人は何も打たず5秒以上待つ。次に入力欄に書きかけを置いて同じことをする | 空のとき: `hc read` に `<news>` が入る。書きかけのとき: **送られない**（次の回に持ち越す）。薄い字（候補）を書きかけと見誤らない（courier.md §4.3） |
| **Esc のあとの届け直し** — 渡した turn を中断したら1回だけ届け直す | `<news>` を渡された turn を Esc で止める | 次の turn で同じ通知が**1回だけ**もう一度届く（控えの `hook` から消えるまで）。2回以上届くなら落ち |

落ちたら: `/etc/claude-code/managed-settings.json` の `hooks`（`/opt/andon/hooks/stop`・`user-prompt-submit`）が付いているか。フックは落ちたら通す作りなので、**フックが何も出さない**ときは、`andon courier drain` を手で打って出力を見る。

## 13. §5.1 の確かめ方への対応表

[design.md](design.md) §5.1 の 1〜6 を、どの段で・何で確かめるか。

| # | 確かめること | どの段 | 何で見るか |
|---|---|---|---|
| 1 | 建屋がパッケージから立つ | 2・3・5・6 | 建屋の repo にパッケージのファイルが無い（`ls ~/src/andon-home`。`Dockerfile` は `FROM` ＋ 2行）。5 の確かめが全部通る。6 のログインが建屋の中で済む |
| 2 | 1本流れる | 10 | `kanban show <id>` の `status` が `done`。`tasks/<id>/log.jsonl` に `doing` → `review` → `done`。`tasks/<id>/library/gate/<head sha>.json` の決定が approve。PR が `MERGED`（`gh pr view <n> --json state`）。担当の端末が片付いた（`hc ls`・`git worktree list`） |
| 3 | 人が触っていない | 10 | `jq -r '.by' /workspace/projects/andon/.flightdeck/tasks/<id>/log.jsonl \| sort \| uniq -c` の `by` が、Chief of Staff・クーリエ・担当・レビュアーの `terminal:` だけ（人のユーザー名が無い）。Chief of Staff の turn が「依頼を受ける」と「結果を見る」の2つだけ（`hc read andon-cos`） |
| 4 | 流れたのは Andon 自身の変更 | 10 | PR の repo が `<owner>/andon`（`gh pr view <n> --json url`）。レベル A か B（ゲートの記録の `tier`） |
| 5 | 自分をテストできる | 7・10 | 建屋の中で `cd /workspace/.ghq/github.com/<owner>/andon && for f in tests/*.sh; do bash "$f"; done` が green（Go・docker は repo の `mise.toml`・環境の備品に依る。`tests/image.sh` は docker が無ければ SKIP） |
| 6 | 仮設は止まっていない | （ホスト・仮設側） | 仮設の建屋の台帳と端末に異常が無い（`deck lint`・`hc ls`）。仮設のプロジェクトが、Andon の建屋ができるまで Andon の仕事を持っている。この建屋の側では何もしない |

数える指標（design.md §5.1 の最後）の最初の1回は、10 の `log.jsonl` から数える: 人の `by` の行の数（介在回数）・依頼書からマージまでの時間・ゲートの `fix`＋`needs_human` の数。

## 直しのタスク（コードは直さない。担当を切るのは Chief of Staff）

| 題 | 内容 | 受け入れの見方 |
|---|---|---|
| **F2: `andon chief-of-staff`** | usage のとおり: `projects/<slug>/` で打つと Chief of Staff を起こし（`hc run`）、`config.yaml` の `coordinator` を合わせ、動いていれば使い、2人なら止まり、`--no-attach` でなければ `hc attach`。`andon chief-engineer`（`internal/chiefengineer`）が手本 | `tests/` に偽の hc で起こす・動いていれば起こさない・coordinator が直る |
| **F3: `andon task start`** | 台帳のタスクに worktree と branch を切り、`hc run` で template を起こし、`kanban start` で結ぶ（`chiefengineer` の4段）。最初のメッセージは skill を読ませる行を持たない（ADR-0001） | `tests/andon-task.sh` に start の正負。10 の4段が1行になる |
