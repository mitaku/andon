# Base image のビルド

`image/Dockerfile` は Ubuntu 24.04 に備品と `package/`（`/opt/andon`）を焼く。CLI は `image/aqua/` の版と checksum で固定し、プロジェクトの言語は mise で管理する。Claude Code は版つきのバイナリを直接取得し、固定 SHA-256 を確認してから配置する。版と checksum の出典は [公式 manifest](https://downloads.claude.ai/claude-code-releases/2.1.318/manifest.json)。建屋固有の設定と起動処理は後続の工程で足す。

## ローカルで確かめる

Docker（BuildKit 対応）を用意する。

```sh
bash tests/image.sh
```

焼く手順は `image/build.sh <andon の checkout> <tag>` の1本だけ。`tests/image.sh`（CI も）と建屋の `bin/bootstrap rebuild --from`（[operate.md](operate.md) の 3）が同じものを打つ。要るのは docker と bash だけ。焼いた元の commit（汚れた checkout なら末尾に `-dirty`）を label `org.opencontainers.image.revision` に付ける（`bin/bootstrap status` が出す）。

台帳の CLI `kanban`（`cmd/kanban/`。P3c）と `andon`（`cmd/andon/`）は、`image/Dockerfile` の最初の段 `bin`（golang の image。版は `mise.toml` の go と揃え、`tests/image.sh` が確かめる）で linux の静的バイナリに焼き、`/usr/local/bin/kanban`・`/opt/andon/bin/andon` に COPY する（`/usr/local/bin/andon` はそこへの symlink）。`kanban --version` は commit の先頭12文字。`deck` は1版だけの別名（`/usr/local/bin/deck` → `kanban` の symlink）。private release の取得や取得用 token は使わない（人の決定 2026-11-05、decisions.md §9）。ホストは同じ `andon` を image から取り出して使う（[operate.md](operate.md) の 2）。

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

## Codex の承認と sandbox

建屋（コンテナ）が信頼の境界なので、Codex を承認の確認で止めない。boot が毎起動、`~/.codex/config.toml` の top-level に無いキーだけを先頭（BOM があればその後ろ）に足す。キーの有無は TOML のパーサで読み、あるキーは値が何でもそのまま、TOML として読めなければ書かない:

- `approval_policy = "never"` — 確認の画面を出さない。
- `sandbox_mode = "danger-full-access"` — 担当とレビュアーは worktree の外（台帳の `projects/<slug>/.flightdeck` とその `.lock`。`deck report`・`deck review`・`library/`）に書き、`git push`（ネットワーク）と hc の socket を使う。`workspace-write` に `writable_roots`・`network_access` を足す形は、台帳の置き場が建屋ごとに違ううえ socket の扱いもあり、その割に建屋の外へは届かない（境界はコンテナ）ので採らない。Claude Code も建屋では sandbox なしで動く。

Claude Code の managed settings の deny のうちコマンドの頭で引くもの（今は `gh pr merge`）を、Codex にも execpolicy の rules で引く。`package/codex.rules` を boot が `~/.codex/rules/andon.rules` に写す（人の rules は別のファイルに書く）。rules は承認の方針の前に引かれ、`forbidden` は `never` でも拒む（Codex 0.166.0 の [`codex-rs/core/src/exec_policy.rs`](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/core/src/exec_policy.rs) の `create_exec_approval_requirement_for_parsed_commands`。当たらないコマンドだけが `render_decision_for_unmatched_command` で `never` → Allow になる）。`bash -lc "a && b"` のような平らな script は1つずつに割って引く。

**Claude Code と同じ広さではない。** rules は token の頭の一致で、引数の値を読み飛ばせない。Codex が拒むのは直接の形（絶対パスを含む）と、引数を取らない wrapper（`env`・`command`・`exec`・`nohup`・`nice`・`sudo`・`xargs`）の形だけ。Claude Code の deny が見る、引数を取る wrapper（`timeout 30 …`・`env FOO=bar …`）・先頭の代入（`FOO=bar gh pr merge`）・`$( )` の中は拒めない（`tests/image.sh` が「拒めないことが分かっている形」として持つ）。守る相手は間違える担当で、偽る担当ではない（下の「残る危険」と同じ D1）。マージを本当に止めるのはゲートと、public にしたあとのブランチ保護。

人のファイルの書き換えの deny（`Edit(//workspace/andon.toml)` など。[ADR-0005](adr/0005-cell-repo.md) 決めたこと 6）も Codex には引けない。rules は argv の頭の一致で、`sed -i … <path>` の書き先のパスに当たらず、`apply_patch` の書き込みにも当たらない。Codex の担当が人のファイルを書いても、機械が読むもの（`project.json`・`gate.local.json`）は author の照合（[contract.md](contract.md) §11）で信じられない。

deny のうち `Bash(git diff *--output*)` などは allow の穴を塞ぐもので、allow を持たない Codex には引かない。2か所の一覧は `tests/package.sh` が突き合わせる。Codex 自身の危険の判定（`rm -f` など）は `never` の下では確認ではなく拒否になる。

Codex のフックは配っていない（フックは Claude Code だけ。[contract.md](contract.md)）ので、フックの信頼の画面は出ない。

`tests/image.sh` は、モデルを呼ばずに `codex debug prompt-input` の permissions の文（実効の設定から組む）と `codex execpolicy check` で確かめる。

## CI と配布

配る版と日々のサイクルを分ける。日々の直しは建屋で andon の checkout から焼いて試す（`bin/bootstrap rebuild --from`。[operate.md](operate.md) の 3）ので、マージも GHCR も要らない。

PR と main の push は、tests の job（`packages` の権限を持たない）で image を焼いて検査するだけで、GHCR に書かない。GHCR に書くのは `vX.Y.Z` の tag の push でだけ動く release の job: tag の名前と「main に含まれる commit か」を先に確かめ、同じ job の中で `tests/*.sh`（`image/build.sh` で焼いた `andon:test` の検査を含む）を通した image を `ghcr.io/<owner>/andon:vX.Y.Z` に push する（image を artifact で受け渡さない。GitHub のストレージ枠を使うため）。マージのたびに約 1 GiB の版が増えて private の package のストレージ枠に当たったので、main の push での `:<40桁のcommit SHA>` の push はやめた。いま GHCR にある SHA の版は残る（tag を切るまでの建屋はそれを使う）。配布は Linux amd64 から始める。GHCR の package は private のまま運用し、公開への変更と tag を切るのは人が行う。

`packages: write` は release の job だけに付ける（step の `if` は token の権限を狭めないので、job で分ける）。PR で動く job の token は GHCR に書けない。`workflow_test.go`（`go test`）が、`.github/workflows/` の全部の workflow で、tag でだけ動く job 以外（job に permissions が無ければ workflow の top のものを継ぐ）に `packages`・`contents` の書き込みと GHCR への push が無いことを確かめる。checkout は認証情報を残さない。Actions は commit SHA、Ubuntu と golang は digest、Python・uv tool は直接指定する版を固定する。

### CI の所要時間

tests の job は、ほとんどが image の build と `tests/image.sh` の検査だった（PR で 6〜7 分）。次の3つで縮める。どれも check の名前（`tests`）は変えず、飛ばすときも job は成功で終わる。

- **andon・kanban を1回だけ焼く**: job の先頭で `go build` し、`ANDON_BIN`・`KANBAN_BIN` で `tests/*.sh` に渡す（go の workflow と同じ）。
- **並べる**: `tests/lib/run-tests.sh` が `tests/*.sh` を runner の CPU 数だけ並べて走らせる。出力は1本ごとに受けて、終わった順に1本ずつまとめて出す（混ざらない）。最後に1本ごとの秒の表を、ログと job の summary に出す。release の job も同じものを打つ。
- **image に関係しない PR では `tests/image.sh` を飛ばす**（下の表）。

**image の build の cache は使わない。** buildx の GitHub Actions cache（`type=gha`、`mode=max`）を試したが、Go だけを変えた PR で `tests/image.sh` が 224 秒から 307 秒に延びた。約 1 GiB の層を cache から取り込む時間と、docker-container の builder から docker へ渡す `--load`（約 70 秒）が、build で浮く時間を上回る。cache を持たないので、release の job が PR の書いたものを読む心配も無い。

**image に関係しない PR では `tests/image.sh` を飛ばす。** PR の変更（merge commit と base の差分。rename は元と先の両方）を `tests/lib/image-scope.sh` が分ける。飛ばしてよいのは、変更が全部、下の表の「飛ばす」に入るときだけ。知らないパスは走らせる側に倒す。main の push では必ず全部走る（release も）。

| 変更 | image の build と runtime の検査 | 理由 |
|---|---|---|
| `image/`・`package/`・`schema/`・`init/`・`cmd/`・`internal/`（`_test.go` を除く）・`go.mod`・`go.sum`・`embed.go`・`.dockerignore` | 走らせる | image の build の context に入る（`.dockerignore` が通すもの）。Go の変更も、image の中の `andon` で `andon init`・`boot` を通す検査に効く |
| `tests/image.sh`・`tests/lib/pty-probe.py`・`tests/lib/run-tests.sh`・`tests/lib/image-scope.sh` | 走らせる | 検査そのものと、それを走らせ分ける道具 |
| `.github/`・`mise.toml` | 走らせる | `tests/image.sh` が image の中の actionlint・ghalint・pinact で `tests.yml` を検査し、Dockerfile の golang の版を `mise.toml` と突き合わせる |
| `docs/`・`tests/` のほか・`*_test.go`・root の `README.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` | 飛ばす | image に入らず、`tests/image.sh` も読まない |
| 上のどれでもないパス | 走らせる | 知らないものは安全な側に倒す |

**手元は関係する検査だけ。** `tests/*.sh` を全部手元で回すと長い（建屋で 20 分を超えた）。`tests/run --changed [<base>]` は、`<base>`（既定は `origin/main` との merge-base）からの差分と未 commit・未追跡のファイルを `tests/lib/scope.sh` で分け、関係する `tests/*.sh` だけを並べて走らせ、1本ごとの秒を出す。全部は `tests/run`（引数なし）か CI に任せる。`go test ./...` は別に打つ。

| 変えたもの | 走らせる `tests/*.sh` |
|---|---|
| どれでも | `public.sh`・`public-selftest.sh`・`smoke.sh`・`ci.sh`（安く全体を見るもの） |
| `docs/contract.md` | `schema.sh` を足す（例を schema と突き合わせる） |
| `docs/ledger.md`・`tests/kanban/` | `kanban.sh` を足す |
| `docs/adr/` | `package.sh` を足す |
| `tests/<x>.sh` | その `tests/<x>.sh` を足す |
| `docs/` のほか・`tests/README.md`・`*_test.go`・root の `README.md` など | 足さない |
| `tests/lib/` | `image.sh` 以外の全部 |
| それ以外（`cmd/`・`internal/`・`package/`・`schema/`・`init/`・`image/`・`.github/` と知らないパス） | `image.sh` 以外の全部 |
| CI の表で「走らせる」もの | `image.sh` も（Docker が無ければ SKIP） |

## 残る危険

workflow を書き換えた branch をこの repo に push できる者は、マージ前でも GHCR に push できる。workflow 内の権限制限だけでは、この操作を防げない。
D1 の守る相手は「間違える担当」であり「偽る担当」ではないため、この意図的な回避は対象外として受け入れる（人の決定、2026-11-05）。

apt のパッケージと npm・PyPI の推移的な依存は今回固定しないため、ビルド全体の完全な再現性は保証しない。sudo の権限を実行時に絞る判断も後続工程に残す。
