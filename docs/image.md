# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は版つきのバイナリを直接取得し、固定 SHA-256 を確認してから配置する。版と checksum の出典は [公式 manifest](https://downloads.claude.ai/claude-code-releases/2.1.322/manifest.json)。セル固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

焼く手順は `image/build.sh <andon の checkout> <tag>` の1本だけ。`tests/image.sh`（CI も）とセルの `bin/bootstrap rebuild --from`（[operate.md](operate.md) の 3）が同じものを打つ。要るのは docker と bash だけ。焼いた元の commit（汚れた checkout なら末尾に `-dirty`）を label `org.opencontainers.image.revision` に付ける（`bin/bootstrap status` が出す）。

台帳の CLI `kanban`（`cmd/kanban/`。P3c）と `andon`（`cmd/andon/`）は、`image/Dockerfile` の最初の段 `bin`（golang の image。版は `mise.toml` の go と揃え、`tests/image.sh` が確かめる）で linux の静的バイナリに焼き、`/usr/local/bin/kanban`・`/opt/andon/bin/andon` に COPY する（`/usr/local/bin/andon` はそこへの symlink）。`kanban --version` は commit の先頭12文字。別名 `deck` は置かない。private release の取得や取得用 token は使わない（人の決定 2026-11-05、decisions.md §9）。ホストは同じ `andon` を image から取り出して使う（[operate.md](operate.md) の 2）。

テストは `andon:test` を作り、ネットワークを切ったコンテナで `/opt/andon`・`andon help` と主要な CLI を確かめる。同じ image の actionlint・ghalint・pinact で イメージ配布 workflow（tests.yml）の構文・権限・固定 SHA も検査する。対話の bash（`bash -ic`・`bash -lic`）で補完・`HISTFILE`・プロンプト・fzf のキー割り当てが効くことと、`rg`・`fzf`・`jq` が動くことも見る。続けてセルを立てる道を通す: image から取り出した `andon` で `andon init` → `bin/bootstrap up`（`FROM andon:test`）→ compose の起動で `andon boot` が `ready` まで走り、managed settings・プロファイル・`CLAUDE.md` の2行（`environment.md` と `AGENTS.local.md`）・skill のリンクができたことを確かめる。Docker が使えなければローカルは SKIP を表示し、CI は失敗する。

## セルの bash（対話）

シェルは bash のまま。設定は `image/bashrc`（image の `/etc/andon-bashrc`。`~/.bashrc` の末尾が読む）に1か所にまとめてある。ssh のログインシェル（`~/.profile` → `~/.bashrc`）・`bin/bootstrap enter`・App の端末のどれでも同じに効く。

- **道具**: ripgrep（`rg`）・fzf・jq。aqua で版と checksum を固定している（`image/aqua/`）。fzf は Ctrl-R（履歴検索）・Ctrl-T（ファイル）・Alt-C（cd）が効く。mise の補完が呼ぶ `usage` も入っている。
- **Tab 補完**: bash-completion（apt）と、git・gh・hc・andon・kanban・just・mise。gh・just・mise は各 CLI の生成物、hc は `hc --help` の一覧から image を焼くときに作る（hc に生成の口が無い）、andon・kanban はサブコマンド名だけ（`image/completions/`。フラグは補わない）。どれも `/usr/share/bash-completion/completions/` に置き、最初の Tab を待たずに登録する。
- **履歴**: `~/.local/state/bash/history`。`state` の volume の下なので、作り直しても残る。追記式・大きめの上限・重複と先頭空白の行は残さない。
- **プロンプト**: `セルの名前:ディレクトリ (git のブランチ)`。色は控えめ。

## 画面を見る道具（playwright-cli）

担当が自分の変更を画面で確かめる道具は、焼いてある Playwright に同梱の `playwright cli`（[ADR-0018](adr/0018-screen-check-tooling.md)）。新しい備品は足さず、配線だけを焼く。

- **入口**: `playwright`（`portless` と同じく `image/agent-wrapper` 経由でセル固定の node で起動する）と `playwright-cli`（`playwright cli "$@"` を呼ぶ wrapper。`image/playwright-cli`）を `~/.local/bin` に置く。実体の置き場 `/opt/andon-tools/bin` は PATH に入れない。
- **設定**: `~/.playwright/cli.config.json`（`image/playwright/cli.config.json`）。cwd に依らず効く全体の設定で、ブラウザを焼いた Chromium に（同梱の既定は Google Chrome を探して落ちる）、出力（スナップショット・スクリーンショット）を `/tmp/playwright-cli` に置く（同梱の既定は cwd の `.playwright-cli/` で、担当の worktree の git に出る）。`~/.playwright` は volume に無いので image に焼く。
- **確かめ**: `tests/image.sh` がネットワークを切ったコンテナで、コンテナの中の静的ページを `open` → `snapshot`（ref が出る）→ `screenshot`（PNG）→ `close` し、git の worktree に何も出ないことを見る。

## 信頼の画面（Claude Code・Codex）

セルでは信頼の画面（trust this folder）を出さない。セル＝コンテナが信頼の境界なので、boot と、起動する直前の `andon task start`・`andon chief-engineer` が、セルの repo（`/workspace`）と、台帳に載った案件の repo の根（`projects/*/kanban/config.yaml` の `repos` にあり、`/workspace/.ghq/<host>/<owner>/<repo>` に clone されたもの）を信頼済みにする。ghq の下の repo を全部は信頼しない（台帳に載せるのは Chief of Staff と人）。Claude Code は `$CLAUDE_CONFIG_DIR/.claude.json` の `projects["<根>"].hasTrustDialogAccepted: true`、Codex は `~/.codex/config.toml` の `[projects."<根>"] trust_level = "trusted"`。足りない所だけ足し、今ある値には触らない。

足すのが根だけで足りる理由は、効く範囲の実測（image の版。使い捨ての HOME で各条件を起動し、最初の画面だけを読んだ）:

| 信頼した所 → 起動した所 | Claude Code 2.1.318 | Codex 0.166.0 |
|---|---|---|
| git の外の親 → その下の dir | 効く（`/` まで親を遡る） | 信頼の画面がもともと出ない |
| git の repo の根 → repo の中の dir | 効く | 効く（画面に「Trusting will apply to the repository root」と出る） |
| repo の根より上の親 → repo の中の dir | 効かない（根で止まる） | 効かない |
| 元の repo の根 → その worktree | 効く | 効く |
| worktree の置き場（実測は旧の `~/.herdr/worktrees`。今の `/workspace/.worktrees` も worktree の根より上の親）→ worktree | 効かない | 効かない |

`tests/image.sh` が、セルの claude と codex を `/workspace` の下の新しい dir で起動しても信頼の画面が出ないこと（対照に `/tmp` の新しい repo では出ること）を確かめる。版を上げたら同じ表を測り直す。

## Codex の承認と sandbox

セル（コンテナ）が信頼の境界なので、Codex を承認の確認で止めない。boot が毎起動、`~/.codex/config.toml` の top-level に無いキーだけを先頭（BOM があればその後ろ）に足す。キーの有無は TOML のパーサで読み、あるキーは値が何でもそのまま、TOML として読めなければ書かない:

- `approval_policy = "never"` — 確認の画面を出さない。
- `sandbox_mode = "danger-full-access"` — 担当とレビュアーは worktree の外（台帳の `projects/<slug>/kanban` とその `.lock`。`kanban report`・`kanban review`・`library/`）に書き、`git push`（ネットワーク）と hc の socket を使う。`workspace-write` に `writable_roots`・`network_access` を足す形は、台帳の置き場がセルごとに違ううえ socket の扱いもあり、その割にセルの外へは届かない（境界はコンテナ）ので採らない。Claude Code もセルでは sandbox なしで動く。

Codex の自動の記憶は boot で止める。`features.memories = false` で機能を止め、`memories.generate_memories = false` と `memories.use_memories = false` も設定し、既存の `true` も覆す。承認・sandbox の既定とは扱いが異なる。`memories`・`features` の表が無ければ先頭に inline table を足す。いずれかの表が既にあり変更が必要なときは TOML を再生成し、コメントや書式を含む元の設定は `config.toml.bak` に退避する。ほかの設定の値とファイルの権限は保つ。既存の記憶ファイルは消さない。Codex 0.166.0 で設定を受け付けることを確認した。設定の意味は [OpenAI の構成リファレンス](https://developers.openai.com/codex/config-reference) に従う。生成を止めると新しいスレッドが記憶生成の入力として保存されず、利用を止めると既存の記憶が今後のセッションに挿入されない。

user 層の設定だけでは、信頼した repo の `.codex/config.toml` が記憶を有効に戻せる。そのため、配布する `codex`・`codex-review` のプロファイルは、通常起動と再開の両方で `-c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false` を渡す。CLI の設定が project 層より優先されるため、repo の設定が `true` でも、このプロファイルから起動した Codex の記憶は無効になる。boot が生成したプロファイルを `tests/andon-boot.sh` で検査する。

Claude Code の managed settings の deny のうちコマンドの頭で引くもの（今は `gh pr merge`）を、Codex にも execpolicy の rules で引く。`package/codex.rules` を boot が `~/.codex/rules/andon.rules` に写す（人の rules は別のファイルに書く）。rules は承認の方針の前に引かれ、`forbidden` は `never` でも拒む（Codex 0.166.0 の [`codex-rs/core/src/exec_policy.rs`](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/core/src/exec_policy.rs) の `create_exec_approval_requirement_for_parsed_commands`。当たらないコマンドだけが `render_decision_for_unmatched_command` で `never` → Allow になる）。`bash -lc "a && b"` のような平らな script は1つずつに割って引く。

**Claude Code と同じ広さではない。** rules は token の頭の一致で、引数の値を読み飛ばせない。Codex が拒むのは直接の形（絶対パスを含む）と、引数を取らない wrapper（`env`・`command`・`exec`・`nohup`・`nice`・`sudo`・`xargs`）の形だけ。Claude Code の deny が見る、引数を取る wrapper（`timeout 30 …`・`env FOO=bar …`）・先頭の代入（`FOO=bar gh pr merge`）・`$( )` の中は拒めない（`tests/image.sh` が「拒めないことが分かっている形」として持つ）。守る相手は間違える担当で、偽る担当ではない（下の「残る危険」と同じ D1）。マージを本当に止めるのはゲートと、public にしたあとのブランチ保護。

人のファイルの書き換えの deny（`Edit(//workspace/andon.toml)` など。[ADR-0005](adr/0005-cell-repo.md) 決めたこと 6）も Codex には引けない。rules は argv の頭の一致で、`sed -i … <path>` の書き先のパスに当たらず、`apply_patch` の書き込みにも当たらない。Codex の担当が人のファイルを書いても、機械が読むもの（`project.json`・`gate.local.json`）は author の照合（[contract.md](contract.md) §11）で信じられない（セルの repo の既定の作者は owner でない。下の「git の作者」）。

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

- セルの repo の作者を owner にしないのは、委譲の照合（[contract.md](contract.md) §11。`project.json`・`gate.local.json` を最後に変えた commit の author ＝ owner）を、担当や Chief of Staff の commit が素通りしないため。上の Codex の段のとおり、Codex の担当には人のファイルの deny が引けず、この照合が守り。
- 人が owner のファイルを commit するときは、ホストの checkout で打つか、セルの中なら作者を明示する（`GIT_AUTHOR_NAME=<name> GIT_AUTHOR_EMAIL=<owner> git commit …`・`andon delegate grant` も同じ環境変数で）。セルの repo の `git config user.email`（repo の中だけの設定）で合わせない: その設定はセルの中の担当にも効き、照合が素通りになる。
- `owner` が空・`andon.toml` に無いときは、案件の repo もセルの作者で書く（commit は通り、owner は名乗らない。委譲はもともと無委譲として読まれる）。ログに1行出す。
- `~/.gitconfig` に `user.name` か `user.email` が直に書いてあれば、人の設定として何もしない（ログに `触らない`）。boot の `includeIf` 以外の節（`gh auth setup-git` の credential helper など）には触らない。何度打っても同じ。

## CI と配布

配る版と日々のサイクルを分ける。日々の直しはセルで andon の checkout から焼いて試す（`bin/bootstrap rebuild --from`。[operate.md](operate.md) の 3）ので、マージも GHCR も要らない。

PR と main の push は、tests の job（`packages` の権限を持たない）で image を焼いて検査するだけで、GHCR に書かない。GHCR に書くのは `vX.Y.Z` の tag の push でだけ動く release の job: tag の名前と「main に含まれる commit か」を先に確かめ、同じ job の中で `tests/*.sh`（`image/build.sh` で焼いた `andon:test` の検査を含む）を通した image を `ghcr.io/<owner>/andon:vX.Y.Z` に push する（image を artifact で受け渡さない。GitHub のストレージ枠を使うため）。マージのたびに約 1 GiB の版が増えて private の package のストレージ枠に当たったので、main の push での `:<40桁のcommit SHA>` の push はやめた。いま GHCR にある SHA の版は残る（tag を切るまでのセルはそれを使う）。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更と tag を切るのは人が行う。

`packages: write` は release の job だけに付ける（step の `if` は token の権限を狭めないので、job で分ける）。PR で動く job の token は GHCR に書けない。`workflow_test.go`（`go test`）が、`.github/workflows/` の全部の workflow で、tag でだけ動く job 以外（job に permissions が無ければ workflow の top のものを継ぐ）に `packages`・`contents` の書き込みと GHCR への push が無いことを確かめる。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu と golang は digest、Python・uv tool は直接指定する版を固定する。

### CI の所要時間

tests の job は、ほとんどが image の build と `tests/image.sh` の検査だった（PR で 6〜7 分）。次の4つで縮める。どれも check の名前（`tests`・`go`）は変えず、飛ばすときも job は成功で終わる（ゲートは PR の head の checks が揃うのを待つので、workflow ごと `paths` で外さない）。

- **andon・kanban を1回だけ焼く**: job の先頭で `go build` し、`ANDON_BIN`・`KANBAN_BIN` で `tests/*.sh` に渡す（go の workflow と同じ）。
- **並べる**: `tests/lib/run-tests.sh` が `tests/*.sh` を runner の CPU 数だけ並べて走らせる。出力は1本ごとに受けて、終わった順に1本ずつまとめて出す（混ざらない）。最後に1本ごとの秒の表を、ログと job の summary に出す。release の job も同じものを打つ。
- **image に関係しない PR では `tests/image.sh` を飛ばす**（下の表）。
- **文書だけの PR では、文書を読む検査だけを走らせる**（下の「文書だけの PR」）。go の job もこれに従う。

**image の build の cache は使わない。** buildx の GitHub Actions cache（`type=gha`、`mode=max`）を試したが、Go だけを変えた PR で `tests/image.sh` が 224 秒から 307 秒に延びた。約 1 GiB の層を cache から取り込む時間と、docker-container の builder から docker へ渡す `--load`（約 70 秒）が、build で浮く時間を上回る。cache を持たないので、release の job が PR の書いたものを読む心配も無い。

**image に関係しない PR では `tests/image.sh` を飛ばす。** PR の変更（merge commit と base の差分。rename は元と先の両方）を `tests/lib/image-scope.sh` が分ける。飛ばしてよいのは、変更が全部、下の表の「飛ばす」に入るときだけ。知らないパスは走らせる側に倒す。main の push では必ず全部走る（release も）。

| 変更 | image の build と runtime の検査 | 理由 |
|---|---|---|
| `image/`・`package/`・`schema/`・`init/`・`cmd/`・`internal/`（`_test.go` を除く）・`go.mod`・`go.sum`・`embed.go`・`process/states/issue.yaml`（`andon` に埋め込む）・`.dockerignore` | 走らせる | image の build の context に入る（`.dockerignore` が通すもの）。Go の変更も、image の中の `andon` で `andon init`・`boot` を通す検査に効く |
| `tests/image.sh`・`tests/lib/pty-probe.py`・`tests/lib/run-tests.sh`・`tests/lib/image-scope.sh` | 走らせる | 検査そのものと、それを走らせ分ける道具 |
| `.github/`・`mise.toml` | 走らせる | `tests/image.sh` が image の中の actionlint・ghalint・pinact で `tests.yml` を検査し、Dockerfile の golang の版を `mise.toml` と突き合わせる |
| `docs/`・`tests/` のほか・`cmd/`・`internal/`・root の `*_test.go`（`package/` などの下のものは image に入るので含めない）・root の `README.md`・`CHANGELOG.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` | 飛ばす | image に入らず、`tests/image.sh` も読まない |
| 上のどれでもないパス | 走らせる | 知らないものは安全な側に倒す |

**文書だけの PR では、文書を読む検査だけを走らせる。** PR の変更が全部 `docs/` の下（ADR・決定の記録・設計書・`docs/assets/`）か、root の `README.md`・`CHANGELOG.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` のときを、`tests/lib/docs-scope.sh` が文書だけと判定する。`tests/README.md` やコード・知らないパスが1つでも混ざれば全部走る。main の push と tag では必ず全部走る。

- go の job: gofmt・vet・`go test ./...`・build・`tests/andon*.sh`・`kanban.sh` を飛ばし、文書を読む Go のテスト（`internal/chiefengineer` の `TestDocumentedInvocations`。`docs/design.md`・`README.md` に書いた `andon chief-engineer` の呼び出しを打つ）だけを走らせる。
- tests の job: `tests/lib/scope.sh` が選ぶ `tests/*.sh` だけを走らせる（下の「手元は関係する検査だけ」の表と同じ）。文書を読む検査は次のとおり。

| 検査 | 読む文書 | 文書だけの PR で走らせるとき |
|---|---|---|
| `public.sh`・`public-selftest.sh` | repo の全部のファイル（秘密・禁止の語） | いつも |
| `smoke.sh` | `README.md`・`LICENSE`・`docs/decisions.md`・`docs/decisions/` があるか・形 | いつも |
| `ci.sh` | 文書は読まない（判定の道具の検査。安いので回す） | いつも |
| `words.sh`・`words-selftest.sh` | repo の全部の文書とコード（ADR-0013 で置き換えた「今」の語）。selftest は ADR-0013 の写しを読む | いつも |
| `schema.sh` | `docs/contract.md` の例を schema と突き合わせる | `docs/contract.md` |
| `package.sh` | `docs/contract.md` §4 の `acceptance[]` の行を skill と突き合わせる | `docs/contract.md`・`docs/adr/` |
| `kanban.sh` | `docs/ledger.md`。schema・`docs/ledger.md`・`cmd/kanban` が指す `docs/` の文書があるか | `docs/ledger.md`・消した（動かした）`docs/` の文書 |
| `process.sh` | `docs/process/` が `process/` から作り直して変わらないか・`docs/dispatcher.md` §7.2 | `docs/process/`・`docs/dispatcher.md` |

ほかの `tests/*.sh`（`andon-*.sh`・`host.sh`・`sshd-env.sh`・`image.sh`）は、文書をコメントやテストの中の例の名前として書くだけで、repo の文書を読まない。

**手元は関係する検査だけ。** `tests/*.sh` を全部手元で回すと長い（セルで 20 分を超えた）。`tests/run --changed [<base>]` は、`<base>`（既定は `origin/main` との merge-base）からの差分と未 commit・未追跡のファイルを `tests/lib/scope.sh` で分け、関係する `tests/*.sh` だけを並べて走らせ、1本ごとの秒を出す。全部は `tests/run`（引数なし）か CI に任せる。`go test ./...` は別に打つ。

| 変えたもの | 走らせる `tests/*.sh` |
|---|---|
| どれでも | `public.sh`・`public-selftest.sh`・`smoke.sh`・`ci.sh`・`words.sh`・`words-selftest.sh`（安く全体を見るもの） |
| `docs/contract.md` | `schema.sh`・`package.sh` を足す（例を schema と、§4 の行を skill と突き合わせる） |
| `docs/ledger.md`・`tests/kanban/`・消した（動かした）`docs/` の文書 | `kanban.sh` を足す |
| `docs/process/`・`docs/dispatcher.md`・`process/`・`schema/process/`・`tests/process/` | `process.sh` を足す |
| `docs/adr/` | `package.sh` を足す |
| `tests/<x>.sh` | その `tests/<x>.sh` を足す |
| `docs/` のほか・`tests/README.md`・`cmd/`・`internal/`・root の `*_test.go`・root の `README.md`・`CHANGELOG.md` など | 足さない |
| `tests/lib/` | `image.sh` 以外の全部 |
| それ以外（`cmd/`・`internal/`・`package/`・`schema/`・`init/`・`image/`・`.github/` と知らないパス） | `image.sh` 以外の全部 |
| CI の表で「走らせる」もの | `image.sh` も（Docker が無ければ SKIP） |

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（人の決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
