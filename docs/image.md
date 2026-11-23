# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` のバージョンと checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code はバージョンつきのバイナリを直接取得し、固定 SHA-256 を確認してから配置する。バージョンと checksum の出典は [公式 manifest](https://downloads.claude.ai/claude-code-releases/2.1.322/manifest.json)。セル固有の設定と起動処理は boot（`andon boot`。[operate.md](operate.md)）が起動のたびに足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

焼く手順は `image/build.sh <andon の checkout> <tag>` の1本だけ。`tests/image.sh`（CI も）とセルの `bin/bootstrap rebuild --from`（[operate.md](operate.md) の 3）が同じものを打つ。要るのは docker と bash だけ。焼いた元の commit（汚れた checkout なら末尾に `-dirty`）を label `org.opencontainers.image.revision` に付ける（`bin/bootstrap status` が出す）。

kanban の CLI `kanban`（`cmd/kanban/`。P3c）と `andon`（`cmd/andon/`）は、`image/Dockerfile` の最初の段 `bin`（golang の image。バージョンは `mise.toml` の go と揃え、`tests/image.sh` が確かめる）で linux の静的バイナリに焼き、`/usr/local/bin/kanban`・`/opt/andon/bin/andon` に COPY する（`/usr/local/bin/andon` はそこへの symlink）。`kanban --version` は commit の先頭12文字。別名 `deck` は置かない。最後の段で、焼いた元の commit と `/opt/andon` の中身の digest を `/opt/andon/version.json` に焼く（`andon version stamp`）。`andon version` がそれを読み、パッケージの commit と、中身がその commit と食い違うか（手で書き換えた・汚れた checkout から焼いた）を JSON で返す（[ADR-0028](adr/0028-improvement-loop.md) P0-1）。private release の取得や取得用 token は使わない（オーナーの決定 2026-11-05、decisions.md §9）。ホストは同じ `andon` を image から取り出して使う（[operate.md](operate.md) の 2）。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon`・`andon help` と主要な CLI を確かめる。同じ image の actionlint・ghalint・pinact で `.github/workflows/` の全部の workflow の構文・権限・固定 SHA も検査する。対話の bash（`bash -ic`・`bash -lic`）で補完・`HISTFILE`・プロンプト・fzf のキー割り当てが効くことと、`rg`・`fzf`・`jq` が動くことも見る。続けてセルを立てる道を通す: image から取り出した `andon` で `andon init` → `bin/bootstrap up`（`FROM andon:test`）→ compose の起動で `andon boot` が `ready` まで走り、managed settings・プロファイル・`CLAUDE.md` の3行（`environment.md`・`writing.md`・`AGENTS.local.md`）・skill のリンクができたことを確かめる。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## セルの bash（対話）

シェルは bash のまま。設定は `image/bashrc`（image の `/etc/andon-bashrc`。`~/.bashrc` の末尾が読む）に1か所にまとめてある。ssh のログインシェル（`~/.profile` → `~/.bashrc`）・`bin/bootstrap herdr`・App の端末のどれでも同じに効く。

- **道具**: ripgrep（`rg`）・fzf・jq。aqua でバージョンと checksum を固定している（`image/aqua/`）。fzf は Ctrl-R（履歴検索）・Ctrl-T（ファイル）・Alt-C（cd）が効く。mise の補完が呼ぶ `usage` も入っている。
- **Tab 補完**: bash-completion（apt）と、git・gh・hc・andon・kanban・just・mise。gh・just・mise は各 CLI の生成物、hc は `hc --help` の一覧から image を焼くときに作る（hc に生成の口が無い）、andon・kanban はサブコマンド名だけ（`image/completions/`。フラグは補わない。andon の分は葉の表から `andon help --all --bash` で作り、`tests/commands.sh` が作り直しても変わらないことを見る）。どれも `/usr/share/bash-completion/completions/` に置き、最初の Tab を待たずに登録する。
- **履歴**: `~/.local/state/bash/history`。`state` の volume の下なので、作り直しても残る。追記式・大きめの上限・重複と先頭空白の行は残さない。
- **プロンプト**: `セルの名前:ディレクトリ (git のブランチ)`。色は控えめ。

## 画面を見る道具（playwright-cli）

担当が自分の変更を画面で確かめる道具は、焼いてある Playwright に同梱の `playwright cli`（[ADR-0018](adr/0018-screen-check-tooling.md)）。新しい備品は足さず、配線だけを焼く。

- **入口**: `playwright`（`portless` と同じく `image/agent-wrapper` 経由でセル固定の node で起動する）と `playwright-cli`（`playwright cli "$@"` を呼ぶ wrapper。`image/playwright-cli`）を `~/.local/bin` に置く。実体の置き場 `/opt/andon-tools/bin` は PATH に入れない。
- **設定**: `~/.playwright/cli.config.json`（`image/playwright/cli.config.json`）。cwd に依らず効く全体の設定で、ブラウザを焼いた Chromium に（同梱の既定は Google Chrome を探して落ちる）、出力（スナップショット・スクリーンショット）を `/tmp/playwright-cli` に置く（同梱の既定は cwd の `.playwright-cli/` で、担当の worktree の git に出る）。`~/.playwright` は volume に無いので image に焼く。
- **確かめ**: `tests/image.sh` がネットワークを切ったコンテナで、コンテナの中の静的ページを `open` → `snapshot`（ref が出る）→ `screenshot`（PNG）→ `close` し、git の worktree に何も出ないことを見る。

## 信頼の画面（Claude Code・Codex）

セルでは信頼の画面（trust this folder）を出さない。セル＝コンテナが信頼の境界なので、boot と、起動する直前の `andon issue start`・`andon chief-engineer` が、セルの repo（`/workspace`）と、kanban に載った案件の repo の根（`projects/*/kanban/config.yaml` の `repos` にあり、`/workspace/.ghq/<host>/<owner>/<repo>` に clone されたもの）を信頼済みにする。ghq の下の repo を全部は信頼しない（kanban に載せるのは Chief of Staff と人）。Claude Code は `$CLAUDE_CONFIG_DIR/.claude.json` の `projects["<根>"].hasTrustDialogAccepted: true`、Codex は `~/.codex/config.toml` の `[projects."<根>"] trust_level = "trusted"`。足りない所だけ足し、今ある値には触らない。

Claude Code には、配布するプロファイル（パッケージの `templates/` とセルの `templates.local/`）の `command:`・`resume:` の行が `--add-dir` で渡すフォルダも足す（Chief of Staff の `/opt/andon/skills/chief-of-staff/playbooks` と `/workspace/wiki`）。足すのはそのフォルダだけで、親は足さない。Codex のプロファイルは `--add-dir` を使わないので、Codex には足さない。

足すのが根だけで足りる理由は、効く範囲の実測（測ったバージョンは image が固定しているもの。この表はバージョンを書かない。使い捨ての HOME で各条件を起動し、最初の画面だけを読んだ）:

| 信頼した所 → 起動した所 | Claude Code | Codex |
|---|---|---|
| git の外の親 → その下の dir | 効く（`/` まで親を遡る） | 信頼の画面がもともと出ない |
| git の repo の根 → repo の中の dir | 効く | 効く（画面に「Trusting will apply to the repository root」と出る） |
| repo の根より上の親 → repo の中の dir | 効かない（根で止まる） | 効かない |
| 元の repo の根 → その worktree | 効く | 効く |
| worktree の置き場（実測は旧の `~/.herdr/worktrees`。今の `/workspace/.worktrees` も worktree の根より上の親）→ worktree | 効かない | 効かない |
| 信頼していない git の外の dir を `--add-dir` で渡す（起動した所は信頼済み） | 信頼の画面が出る。答えはそのセッションだけで残らない | （測っていない。`--add-dir` を使わない） |
| `--add-dir` の dir そのもの（か、git の外の親）→ その dir を `--add-dir` で渡す | 効く。symlink を通して渡しても、実体のパスの信頼が効く | （同上） |
| 信頼した repo の中の dir を `--add-dir` で渡す | 効く（画面は出ない） | （同上） |

`tests/image.sh` が、セルの claude と codex を `/workspace` の下の新しい dir で起動しても信頼の画面が出ないこと（対照に `/tmp` の新しい repo では出ること）を確かめる。Claude Code か Codex のバージョンを上げたら同じ表を測り直す。

## Codex の承認と sandbox

セル（コンテナ）が信頼の境界なので、Codex を承認の確認で止めない。boot が毎起動、`~/.codex/config.toml` の top-level に無いキーだけを先頭（BOM があればその後ろ）に足す。キーの有無は TOML のパーサで読み、あるキーは値が何でもそのまま、TOML として読めなければ書かない:

- `approval_policy = "never"` — 確認の画面を出さない。
- `sandbox_mode = "danger-full-access"` — 担当とレビュアーは worktree の外（kanban の `projects/<slug>/kanban` とその `.lock`。`kanban report`・`kanban review`・`library/`）に書き、`git push`（ネットワーク）と hc の socket を使う。`workspace-write` に `writable_roots`・`network_access` を足す形は、kanban の置き場がセルごとに違ううえ socket の扱いもあり、その割にセルの外へは届かない（境界はコンテナ）ので採らない。Claude Code もセルでは sandbox なしで動く。

Codex の自動の記憶は boot で止める。`features.memories = false` で機能を止め、`memories.generate_memories = false` と `memories.use_memories = false` も設定し、既存の `true` も覆す。承認・sandbox の既定とは扱いが異なる。`memories`・`features` の表が無ければ先頭に inline table を足す。いずれかの表が既にあり変更が必要なときは TOML を再生成し、コメントや書式を含む元の設定は `config.toml.bak` に退避する。ほかの設定の値とファイルの権限は保つ。既存の記憶ファイルは消さない。Codex 0.166.0 で設定を受け付けることを確認した。設定の意味は [OpenAI の構成リファレンス](https://developers.openai.com/codex/config-reference) に従う。生成を止めると新しいスレッドが記憶生成の入力として保存されず、利用を止めると既存の記憶が今後のセッションに挿入されない。

user 層の設定だけでは、信頼した repo の `.codex/config.toml` が記憶を有効に戻せる。そのため、配布する `codex`・`codex-review` のプロファイルは、通常起動と再開の両方で `-c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false` を渡す。CLI の設定が project 層より優先されるため、repo の設定が `true` でも、このプロファイルから起動した Codex の記憶は無効になる。boot が生成したプロファイルを `tests/andon-boot.sh` で検査する。

Claude Code の managed settings の deny のうちコマンドの頭で引くもの（今は `gh pr merge`・`gh pr comment`・`gh pr review`）を、Codex にも execpolicy の rules で引く。`package/codex.rules` を boot が `~/.codex/rules/andon.rules` に写す（オーナーの rules は別のファイルに書く）。rules は承認の方針の前に引かれ、`forbidden` は `never` でも拒む（Codex 0.166.0 の [`codex-rs/core/src/exec_policy.rs`](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/core/src/exec_policy.rs) の `create_exec_approval_requirement_for_parsed_commands`。当たらないコマンドだけが `render_decision_for_unmatched_command` で `never` → Allow になる）。`bash -lc "a && b"` のような平らな script は1つずつに割って引く。

**Claude Code と同じ広さではない。** rules は token の頭の一致で、引数の値を読み飛ばせない。Codex が拒むのは直接の形（絶対パスを含む）と、引数を取らない wrapper（`env`・`command`・`exec`・`nohup`・`nice`・`sudo`・`xargs`）の形だけ。Claude Code の deny が見る、引数を取る wrapper（`timeout 30 …`・`env FOO=bar …`）・先頭の代入（`FOO=bar gh pr merge`）・`$( )` の中は拒めない（`tests/image.sh` が「拒めないことが分かっている形」として持つ）。守る相手は間違える担当で、偽る担当ではない（下の「残る危険」と同じ D1）。マージを本当に止めるのは gate と、public にしたあとのブランチ保護。

オーナーのファイルの書き換えの deny（`Edit(//workspace/andon.toml)` など。[ADR-0005](adr/0005-cell-repo.md) 決めたこと 6）も Codex には引けない。rules は argv の頭の一致で、`sed -i … <path>` の書き先のパスに当たらず、`apply_patch` の書き込みにも当たらない。Codex の担当がオーナーのファイルを書いても、機械が読むもの（`project.json`・`gate.local.json`）は author の照合（[contract.md](contract.md) §11）で信じられない（セルの repo の既定の作者は owner でない。下の「git の作者」）。

Claude Code の許可の規則は managed だけが効く（`package/settings.json` の `allowManagedPermissionRulesOnly: true`。[ADR-0018](adr/0018-screen-check-tooling.md) 決めたこと 6）。skill の frontmatter の `allowed-tools`（同梱の `playwright-cli` の skill は `Bash(npx:*)`・`Bash(npm:*)` を持つ）と user・project・local の `allow` は無視され、許可の確認は Chief of Staff が答える。足りない allow は `package/settings.json` に足す。

deny のうち `Bash(git diff *--output*)` などは allow の穴を塞ぐもので、allow を持たない Codex には引かない。2か所の一覧は `tests/package.sh` が突き合わせる。Codex 自身の危険の判定（`rm -f` など）は `never` の下では確認ではなく拒否になる。

Codex のフックは配っていない（フックは Claude Code だけ。[contract.md](contract.md)）ので、フックの信頼の画面は出ない。

`tests/image.sh` は、モデルを呼ばずに `codex debug prompt-input` の permissions の文（実効の設定から組む）と `codex execpolicy check` で確かめる。

## git の作者

セルの `~` は作り直すと消えるので、git の既定の作者は boot が毎起動書く（`internal/boot/gitauthor.go`）。`~/.gitconfig` に `includeIf` を2つ、この順に置く（後ろが勝つ）:

| 当たる repo | include するファイル | 作者 |
|---|---|---|
| `gitdir:/workspace/`（セルの repo） | `~/.config/andon/git/cell.gitconfig` | `Andon agent <andon-agent@cell.invalid>`。owner と一致しない |
| `gitdir:$GHQ_ROOT/`（案件の repo と、その worktree。worktree は元の repo の gitdir で当たる） | `~/.config/andon/git/product.gitconfig` | `andon.toml` の `owner`（名前は noreply なら login、ほかは `@` の前） |

- セルの repo の作者を owner にしないのは、委譲の照合（[contract.md](contract.md) §11。`project.json`・`gate.local.json` を最後に変えた commit の author ＝ owner）を、担当や Chief of Staff の commit が素通りしないため。上の Codex の段のとおり、Codex の担当にはオーナーのファイルの deny が引けず、この照合が守り。
- author ＝ owner で照合するファイル（`project.json`・`gate.local.json`）をオーナーが commit するときは、ホストの checkout で打つか、セルの中なら作者を明示する（`GIT_AUTHOR_NAME=<name> GIT_AUTHOR_EMAIL=<owner> git commit …`。`andon delegate grant` は自分で owner を作者にして commit する）。セルの repo の `git config user.email`（repo の中だけの設定）で合わせない: その設定はセルの中の担当にも効き、照合が素通りになる。
- `owner` が空・`andon.toml` に無いときは、案件の repo もセルの作者で書く（commit は通り、owner は名乗らない。委譲はもともと無委譲として読まれる）。ログに1行出す。
- `~/.gitconfig` に `user.name` か `user.email` が直に書いてあれば、オーナーの設定として何もしない（ログに `触らない`）。boot の `includeIf` 以外の節（`gh auth setup-git` の credential helper など）には触らない。何度打っても同じ。

## CI と配布

配るバージョンと日々のサイクルを分ける。日々の直しはセルで andon の checkout から焼いて試す（`bin/bootstrap rebuild --from`。[operate.md](operate.md) の 3）ので、マージも GHCR も要らない。

PR と main の push は、tests の job（`packages` の権限を持たない）で image を焼いて検査するだけで、GHCR に書かない。GHCR に書くのは `vX.Y.Z` の tag の push でだけ動く release の job: tag の名前と「main に含まれる commit か」を先に確かめ、同じ job の中で `tests/*.sh`（`image/build.sh` で焼いた `andon:test` の検査を含む）を通した image を `ghcr.io/<owner>/andon:vX.Y.Z` に push する（image を artifact で受け渡さない。GitHub のストレージ枠を使うため）。マージのたびに約 1 GiB のバージョンが増えて private の package のストレージ枠に当たったので、main の push での `:<40桁のcommit SHA>` の push はやめた。いま GHCR にある SHA のバージョンは残る（tag を切るまでのセルはそれを使う）。配布は Linux amd64 から始める。GHCR の package は Public（2026-11-13 に、認証なしで tag の一覧が取れることを確かめた）。tag は、v0.0.x の間は Andon を開発するセルの Chief of Staff が切る（手順は [operate.md](operate.md) §14。v0.1.0 の前にオーナーが決め直す）。

`packages: write` は release の job だけに付ける（step の `if` は token の権限を狭めないので、job で分ける）。PR で動く job の token は GHCR に書けない。`workflow_test.go`（`go test`）が、`.github/workflows/` の全部の workflow で、tag でだけ動く job 以外（job に permissions が無ければ workflow の top のものを継ぐ）に `packages`・`contents` の書き込みと GHCR への push が無いことを確かめる。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu と golang は digest、Python・uv tool は直接指定するバージョンを固定する。

### CI の所要時間

tests の job は、ほとんどが image の build と `tests/image.sh` の検査だった（PR で 6〜7 分）。次の4つで縮める。どれも check の名前（`tests`・`go`）は変えず、飛ばすときも job は成功で終わる（gate は PR の head の checks が揃うのを待つので、workflow ごと `paths` で外さない）。

- **andon・kanban を1回だけ焼く**: job の先頭で `go build` し、`ANDON_BIN`・`KANBAN_BIN` で `tests/*.sh` に渡す。release の job も同じく焼いて渡す。`tests/*.sh` を回すのは tests の workflow だけで、go の workflow は回さない（二重に回さない。`workflow_test.go` が確かめる）。
- **並べる**: `tests/lib/run-tests.sh` が `tests/*.sh` を runner の CPU 数だけ並べて走らせる。出力は1本ごとに受けて、終わった順に1本ずつまとめて出す（混ざらない）。ただし `tests/image.sh` だけは溜めずに、1行ごとに「[tests/image.sh] 」を付けて流す（job が上限で打ち切られても、どこまで進んだかがログに残る）。tests の job の上限は45分。最後に1本ごとの秒の表を、ログと job の summary に出す。release の job も同じものを打つ。長い `tests/image.sh` と `tests/andon-dispatcher.sh` を先に起動する。`tests/schema.sh` は command の出力を schema に通すのに、写す側の検査（`andon-dispatcher.sh` など11本）が同じ回で写したものを読む。写す側が全部終わってから最後に起動し、回し直さない（[ADR-0032](adr/0032-shorten-time-to-merge.md) 決めたこと 2。前は自分の中で11本をもう一度走らせ、102〜220 秒かかっていた）。
- **image に関係しない PR では `tests/image.sh` を飛ばす**（下の表）。
- **文書だけの PR では、文書を読む検査だけを走らせる**（下の「文書だけの PR」）。go の job もこれに従う。

**image の build の cache は使わない。** buildx の GitHub Actions cache（`type=gha`、`mode=max`）を試したが、Go だけを変えた PR で `tests/image.sh` が 224 秒から 307 秒に延びた。約 1 GiB の層を cache から取り込む時間と、docker-container の builder から docker へ渡す `--load`（約 70 秒）が、build で浮く時間を上回る。cache を持たないので、release の job が PR の書いたものを読む心配も無い。

**image の build の apt は、CI の tests の job だけ Azure の mirror から取る。** 取得元が `archive.ubuntu.com`・`security.ubuntu.com` だけだと、その日の速さに引きずられる（2026-11-19 は 70〜90 kB/s で、`tests/image.sh` が 194 秒から 1358 秒に延びた）。tests の job の `tests/*.sh` を回す step だけが `ANDON_APT_MIRROR=http://azure.archive.ubuntu.com/ubuntu/`（runner と同じ mirror）を置き、`image/build.sh` がそれを build-arg の `APT_MIRROR` に渡す。`image/Dockerfile` は build の中でだけ sources をそこへ向け、playwright の段の apt が済んだら元に戻す。手元・release の job・`bin/bootstrap rebuild --from` は渡さないので、取得元は今のまま。release の job の取得元は変えない（`workflow_test.go` が確かめる）。

**apt の索引は build の中で1回だけ取る。** apt の段が取った索引（`/var/lib/apt/lists`）を build の cache（`RUN --mount=type=cache`）に置き、playwright の段がそれで Chromium の依存（`playwright install-deps`。入れる顔ぶれは `--with-deps` と同じ）を入れる。playwright は必ず `apt-get update` を打つので、その RUN だけ update を飛ばす `apt-get` を PATH の先に置く。飛ばすのは、apt の段が update した時刻（索引の cache の `andon-updated-at`）が1時間以内のときだけ。手元で apt の段が層の cache から来て索引が前の build のものなら、古いので update する（古い索引のまま古い依存を入れない）。索引が無い・取得元と合わず install が落ちたときも update からやり直す。索引は層に残らない。取得元と取得の回数は build のログで見る: apt の段の「apt の取得元: …」の行と `Get:` の行、playwright の段の「apt-get update を飛ばす」の行。

**image に関係しない PR では `tests/image.sh` を飛ばす。** PR の変更（merge commit と base の差分。rename は元と先の両方）を `tests/lib/image-scope.sh` が分ける。飛ばしてよいのは、変更が全部、下の表の「飛ばす」に入るときだけ。知らないパスは走らせる側に倒す。main の push では必ず全部走る（release も）。

| 変更 | image の build と runtime の検査 | 理由 |
|---|---|---|
| `image/`・`package/`・`schema/`・`init/`・`cmd/`・`internal/`（`_test.go` を除く）・`go.mod`・`go.sum`・`embed.go`・`process/states/issue.yaml`（`andon` に埋め込む）・`.dockerignore` | 走らせる | image の build の context に入る（`.dockerignore` が通すもの）。Go の変更も、image の中の `andon` で `andon init`・`boot` を通す検査に効く |
| `tests/image.sh`・`tests/lib/pty-probe.py`・`tests/lib/run-tests.sh`・`tests/lib/image-scope.sh` | 走らせる | 検査そのものと、それを走らせ分ける道具 |
| `.github/`・`mise.toml` | 走らせる | `tests/image.sh` が image の中の actionlint・ghalint・pinact で `.github/workflows/` の全部を検査し、`go.mod` の go と Dockerfile の golang のバージョンを `mise.toml` と突き合わせる |
| `docs/`・`tests/` のほか・`cmd/`・`internal/`・root の `*_test.go`（`package/` などの下のものは image に入るので含めない）・root の `README.md`・`CHANGELOG.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` | 飛ばす | image に入らず、`tests/image.sh` も読まない |
| `image/apply-main.sh` | 飛ばす | セルに main の commit を当てる道具（[ADR-0027](adr/0027-apply-package-without-rebuild.md)）。`image/` の下だが Dockerfile は COPY しない |
| `model/` | 飛ばす | 課題の一生のモデルと検査器のバージョン。image に入らず、`tests/image.sh` も読まない。回すのは model の workflow の job（[ADR-0025](adr/0025-formal-model-in-repo.md)） |
| `.claude/skills/`・`.agents/skills/` | 飛ばす | repo の中の skill の置き場（検証の skill。[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 4）。image にも `andon` にも入らない。ほかの `.claude/` は知らないパスとして走らせる |
| 上のどれでもないパス | 走らせる | 知らないものは安全な側に倒す |

**文書だけの PR では、文書を読む検査だけを走らせる。** PR の変更が全部 `docs/` の下（ADR・決定の記録・設計書・`docs/assets/`）か、root の `README.md`・`CHANGELOG.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` のときを、`tests/lib/docs-scope.sh` が文書だけと判定する。`tests/README.md` やコード・知らないパスが1つでも混ざれば全部走る。main の push と tag では必ず全部走る。

- go の job: gofmt・vet・golangci-lint・`go test ./...`・build を飛ばし、文書を読む Go のテスト（`internal/chiefengineer` の `TestDocumentedInvocations`。`docs/design.md`・`README.md` に書いた `andon chief-engineer` の呼び出しを打つ）だけを走らせる。
- tests の job: `tests/lib/scope.sh` が選ぶ `tests/*.sh` だけを走らせる（下の「手元は関係する検査だけ」の表と同じ）。文書を読む検査は次のとおり。

| 検査 | 読む文書 | 文書だけの PR で走らせるとき |
|---|---|---|
| `public.sh`・`public-selftest.sh` | repo の全部のファイル（秘密・禁止の語） | いつも |
| `smoke.sh` | `README.md`・`LICENSE`・`docs/decisions.md`・`docs/decisions/` があるか・形 | いつも |
| `ci.sh` | 文書は読まない（判定の道具の検査。安いので回す） | いつも |
| `shell-and.sh` | 文書は読まない（`tests/` の下のシェルで、set -e で止まらない `A && B` の判定を探す。安いので回す） | いつも |
| `words.sh`・`words-selftest.sh` | repo の全部の文書とコード（ADR-0013 で置き換えた「今」の語）。selftest は ADR-0013 の写しを読む | いつも |
| `agents-md.sh` | root の `AGENTS.md` の「決まり｜守らせるもの」の表と、守らせるものが名指したファイル（[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 6） | いつも |
| `verify-andon.sh` | 検証の skill（`.claude/skills/verify-andon/`）の目次・機能の地図と、手順が名指す command の実在（[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 5。andon のコマンドは `andon help --all` の表と照らす） | いつも |
| `schema.sh` | `docs/contract.md` の例を schema と突き合わせる | `docs/contract.md`（出力を読む11本も並べる。下の「手元は関係する検査だけ」） |
| `package.sh` | `docs/contract.md` §4 の `acceptance[]` の行を skill と突き合わせる | `docs/contract.md`・`docs/adr/` |
| `adr-index.sh` | `docs/adr/` の索引・状態行（[ADR-0033](adr/0033-adr-inventory-and-slimming.md)）と同じ番号の ADR が 2 本無いこと、ADR ごとの決定の記録（`docs/decisions/`・`docs/decisions.md` §14）、`docs/adr/README.md` の一覧の見出しのバージョンと `CHANGELOG.md` の一番上のバージョン（[ADR-0055](adr/0055-adr-prs-without-conflicts.md)） | `docs/adr/`・`docs/decisions.md`・`docs/decisions/`・`CHANGELOG.md` |
| `kanban.sh` | `docs/ledger.md`。schema・`docs/ledger.md`・`cmd/kanban` が指す `docs/` の文書があるか | `docs/ledger.md`・消した（動かした）`docs/` の文書 |
| `process.sh` | `docs/process/` が `process/` から作り直して変わらないか・`docs/dispatcher.md` §7.2 | `docs/process/`・`docs/dispatcher.md` |
| `commands.sh` | `docs/commands.md` が andon の葉の表から作り直して変わらないか（[ADR-0053](adr/0053-command-system.md) 決めたこと 16）。配る文の andon のコマンドは `package/` を見る | `docs/commands.md` |

ほかの `tests/*.sh`（`andon-*.sh`・`host.sh`・`sshd-env.sh`・`image.sh`）は、文書をコメントやテストの中の例の名前として書くだけで、repo の文書を読まない。

**手元は関係する検査だけ。** `tests/*.sh` を全部手元で回すと長い（セルで 20 分を超えた）。`tests/run --changed [<base>]` は、`<base>`（既定は `origin/main` との merge-base）からの差分と未 commit・未追跡のファイルを `tests/lib/scope.sh` で分け、関係する `tests/*.sh` だけを並べて走らせ、1本ごとの秒を出す。全部は `tests/run`（引数なし）か CI に任せる。`go test ./...` は別に打つ。`tests/run` は worktree をまたいで同時に1本だけ走る。鍵のファイル（既定は `/tmp/andon-tests-run.lock`）を flock で取り、ほかの回が走っていれば理由を1行出して、その回が終わるまで待つ。CI は今までどおり全部を走らせる（文書だけの PR を除く）。

重い検査は、その検査が見る部分を変えたときだけ手元で回す（[ADR-0032](adr/0032-shorten-time-to-merge.md) 決めたこと 4）。コードを変えたら、重い検査を除く全部と、変えたパスを見る重い検査を走らせる。知らないパスでは、重い検査を含む全部を走らせる。

| 変えたもの | 走らせる `tests/*.sh` |
|---|---|
| どれでも | `public.sh`・`public-selftest.sh`・`smoke.sh`・`ci.sh`・`words.sh`・`words-selftest.sh`・`agents-md.sh`・`verify-andon.sh`・`shell-and.sh`（安く全体を見るもの） |
| `docs/contract.md` | `schema.sh`・`package.sh` を足す（例を schema と、§4 の行を skill と突き合わせる） |
| `docs/ledger.md`・`tests/kanban/`・消した（動かした）`docs/` の文書 | `kanban.sh` を足す |
| `tests/host/` | `host.sh` を足す |
| `tests/package/` | `package.sh` を足す（`package.sh` の節。入口の `tests/package.sh` が順に読む） |
| `.yomiyasu-words/` | `package.sh` を足す（yomiyasu の許可の一覧。節の `tests/package/23-yomiyasu-words.sh` が正と突き合わせる。image には入らない） |
| `docs/process/`・`docs/dispatcher.md`・`process/`・`schema/process/`・`tests/process/` | `process.sh` を足す |
| `docs/adr/` | `package.sh`・`adr-index.sh` を足す |
| `docs/decisions.md`・`docs/decisions/`・root の `CHANGELOG.md` | `adr-index.sh` を足す |
| `tests/adr-new`（ADR の番号を取る command） | `adr-new.sh` を足す |
| `tests/<x>.sh` | その `tests/<x>.sh` を足す |
| `image/apply-main.sh` | `apply-main.sh` を足す |
| `docs/` のほか・`tests/README.md`・`cmd/`・`internal/`・root の `*_test.go`・root の `README.md`・`.claude/skills/`・`.agents/skills/` など | 足さない |
| `tests/lib/scope.sh`・`docs-scope.sh`・`image-scope.sh` | 足さない（判定の道具は、いつも走る `ci.sh` が見る） |
| `tests/lib/` のほか | `image.sh` 以外の全部 |
| `cmd/`・`internal/`・`package/`・`schema/`・`init/`・`image/`・`embed.go`・andon に埋め込む `process/states/issue.yaml` | 重い検査を除く全部と、変えたパスを見る重い検査（下の表） |
| それ以外（`go.mod`・`go.sum`・`mise.toml`・`.dockerignore`・`.github/`・`model/` と知らないパス） | `image.sh` 以外の全部 |
| `schema.sh` を走らせるとき | `schema.sh` が出力を読む11本（`andon-issue.sh`・`andon-dispatcher.sh` など。`bash tests/schema.sh --emitters`）も並べる。並べないと `schema.sh` が自分の中で1本ずつ走らせて遅い |
| CI の表で「走らせる」もの | `image.sh` も（Docker が無ければ SKIP） |

重い検査と、その検査が見る部分は次のとおり（正は `tests/lib/scope.sh` の表。`bash tests/lib/scope.sh --heavy` で出る）。Go の package は、検査が打つ command の package と、それが import する package。`tests/ci.sh` が `go list -deps` と突き合わせ、import を足して表から漏れると落ちる。`cmd/andon` と `internal/cli` は全部の command の入口なので、どの検査の部分にも入る。

| 重い検査 | 見る部分 |
|---|---|
| `andon-dispatcher.sh` | andon issue が使う Go の package（下）・`package/gate.json`・`package/hooks/` |
| `andon-issue.sh` | andon issue が使う Go の package・`package/gate.json` |
| `andon-gate.sh` | andon issue が使う Go の package・`package/gate.json` |
| `apply-main.sh` | `image/apply-main.sh`・`cmd/andon`・`cmd/kanban`・`internal/cli`・`internal/pkgversion` と、`cmd/kanban` が import する `internal/ledger`・`delegate`・`spawn`・`contract`、`contract` が import する root の `embed.go` |
| `schema.sh` | `schema/` の直下の `*.schema.json`・`schema/ledger/` の `*.schema.json`・`docs/contract.md`・`package/lib/andon/schema_check.py`・照合に打つ `andon schema` の Go の package（`cmd/andon`・`internal/cli`・`internal/schema`・`internal/contract`・root の `embed.go`） |

andon issue が使う Go の package は、`cmd/andon`・`cmd/kanban`・`internal/cli` と、`internal/issue`・`dispatcher`・`gate` とそれが import するもの（`boot`・`chiefengineer`・`process`・`triage`・`retro`・`ghpr`・`signal`・`launch`・`sessionlog`・`pkgversion`・`ledger`・`flock`・`delegate`・`spawn`・`contract`・`redact`・`waiting`）、root の `embed.go` と、それが埋め込む `init/`・`process/states/issue.yaml`・`schema/` の直下の `*.schema.json`。たとえば `internal/measure/` だけを変えたら、重い検査は1本も走らない。skill の文（`package/skills/`）だけを変えたときも同じ。

### 検査の隔離

検査がセルの本物（kanban・HOME）を読み書きしないかは、`tests/isolation` が偽のセル（番兵）で回して確かめる（`tests/README.md`）。全部を回すと、セルでは 15 分を超え、GitHub の runner では約 5 分かかる（tests の job は約 3 分半）。tests の job に足すとその分だけ延びるので、PR では別の job の isolation が、PR の変更に関係する分だけを並べて回す。

回す検査は `tests/lib/scope.sh --isolation` が PR の変更（tests の job と同じ差分）から選び、`tests/isolation` に渡す。`tests/isolation` は `go test ./...` を毎回回し、渡した `tests/*.sh` を番兵の中で回す。選ぶものが無ければ回さず、job は成功で終わる。選び方は `tests/ci.sh` が確かめる。

| 変えたもの | 番兵の中で回す `tests/*.sh` |
|---|---|
| 検査の土台（`tests/lib/` のうち判定の道具と `pty-probe.py` を除くもの・`tests/isolation`・job を書く `.github/workflows/tests.yml`） | `image.sh` を除く全部 |
| 検査が読む置き場（`tests/host/`・`tests/kanban/`・`tests/package/`・`tests/process/`・`tests/adr-new`） | その置き場を読む検査（`host.sh` など） |
| `package/host/`（`host.sh` が回す plugin。HOME や上のフォルダの kanban を読む） | `host.sh` |
| `tests/<x>.sh` | その `tests/<x>.sh`（`schema.sh` なら出力を読む11本も） |
| Go（`cmd/`・`internal/`・root の `*.go`・`go.mod`・`go.sum`） | `smoke.sh`（`go test ./...` の隔離を見る。数分） |
| それ以外（文書・`package/` のほか・`schema/`・`image/`・`model/`・`mise.toml` など） | 回さない |

`image.sh` は docker が要るので番兵の中では回さない（tests の job が回す）。main の push では回さない。main で隔離が壊れていないかは、Andon を開発するセルの週1回の点検 `check-isolation`（[ADR-0047](adr/0047-idle-slots-run-upkeep.md) 決めたこと 18）が全部を回して見る。

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（オーナーの決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
