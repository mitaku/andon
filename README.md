<p><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo/andon-wordmark-dark.svg"><img src="docs/assets/logo/andon-wordmark.svg" alt="andon" height="64"></picture></p>

# Andon

**Andon — a one-person software factory.** AI coding agents take issues from a ledger through implementation, review and merge, and the human is called only for the decisions that only they can make.

一人で使うソフトウェア工場。エージェントが台帳の課題を実装・レビュー・マージまで流し、人は自分にしかできない決断のときだけ呼ばれる。

## English summary

**What it is.** Andon — a one-person software factory — is the package, the contracts and the documents for running a software "line" with AI agents. One long-lived container (a *cell*) is set up per trust boundary (one client, one hobby, ...). Inside it, a coordinating agent (the *Chief of Staff*) reads a file-based issue ledger, starts one worker agent per issue in its own git worktree, asks a reviewer agent of a different model family to review the pull request, and lets a gate decide from rules what can merge without a person. The human is called (the andon light turns on) only for five kinds of decisions: what to build, how users should experience it, promises to the outside, irreversible risks, and changes to the line itself. Which of these a person must answer can be delegated per project, except what to build.

The design calls this a *selective dark factory*. A "dark factory" is a factory that runs with the lights off because no person is on the floor. Andon is selective: machines stop what machines can decide (tests, gates), language models handle what needs judgment, and a person sees only what is left. The name comes from the andon cord and light of the Toyota Production System. Andon is not a manufacturing-floor andon product.

**Why a cell.** Andon is a one-person factory, and a cell is one station in it: one per trust boundary, not one root for the whole factory. The root directory of each cell is itself a git repository (the *cell repo*, `/workspace` in the container). Everything the cell's agents need hangs from that root: projects with their ledger, project instructions and skills (`projects/<slug>/`), the cell's wiki (`wiki/`), the cell's own instructions (`AGENTS.local.md`), and the client repositories it works on (`.ghq/`). So instructions, skills and the wiki are versioned, can be moved to another machine, and the cell can be rebuilt from a clone; the ledger stays outside git as a log that lives with the cell. The container image draws the boundary and carries the tools (Claude Code, Codex, herdr and hc, Playwright with a pre-installed browser, and Andon itself under `/opt/andon`), so every cell has the same tools at the same versions, the host needs only Docker and bash, and an Andon upgrade is one pull request that bumps `FROM`. Client repositories are only cloned into the cell; workers change them through pull requests from per-issue worktrees, and the ledger and the cell settings never go into them. Details (Japanese): [The cell](#セルの考え方), [ADR-0005](docs/adr/0005-cell-repo.md), [ADR-0016](docs/adr/0016-cell-wiki-llm-wiki.md).

**Who it is for.** One developer who already runs several coding agents in terminals and wants them to keep working while the developer is away, without giving up control of merges and irreversible operations. It is built for its author's own use first.

**Status.** Experimental. It is used by one person, and the interfaces change without notice. Most documents are written in Japanese; English is added only where needed. There are no releases with compatibility promises yet; what changed in each tagged version, and what to do when upgrading, is in [CHANGELOG.md](CHANGELOG.md). The host must be Linux or WSL with Docker and bash. The cell image bundles agent CLIs (Claude Code, Codex) and the terminal workspace herdr; you bring your own accounts for them.

**Getting started.** The step-by-step procedure (Japanese) is [docs/operate.md](docs/operate.md): pull the image, create a cell repo with `andon init <slug>`, run `bin/bootstrap`, then start the Chief of Staff and run one issue. The commands are also in [Set up a cell](#セルを立てる) below.

**Map.**

| Path | What is there |
|---|---|
| [docs/design.md](docs/design.md) | What Andon is, its vocabulary, the repo layout and tool choices (the main design document) |
| [docs/vision.md](docs/vision.md) | Vision, mission and values on one page, with a figure of the line (stages, roles, mechanisms, and what Andon does not have yet). Proposal |
| [docs/contract.md](docs/contract.md), [schema/](schema/) | Machine-readable contracts (ledger keys, findings, reports, questions, notifications, gate, delegation). `schema/` is the source of truth |
| [docs/operate.md](docs/operate.md) | How to set up a cell and run one issue |
| [docs/dispatcher.md](docs/dispatcher.md), [docs/ledger.md](docs/ledger.md), [docs/image.md](docs/image.md) | The dispatcher (resident notifier), the ledger CLI, the container image |
| [docs/adr/](docs/adr/), [docs/decisions.md](docs/decisions.md), [docs/decisions/](docs/decisions/) | Architecture decision records and the decision log (one file per decision from 2026-11-07) |
| `cmd/andon`, `cmd/kanban`, `internal/` | The `andon` CLI and the ledger CLI (`kanban`), in Go |
| `package/` | What is shipped into the cell at `/opt/andon`: skills, agent profiles, gate rules, managed settings, hooks, compose file, host templates |
| `image/`, `init/` | The base image, and the template that `andon init` copies into a new cell repo |
| `tests/` | Shell tests run on every pull request, including `tests/public.sh` (no secrets or personal values) |

**Contributing and security.** This is a one-person project and does not take pull requests for now. To report a vulnerability, see [SECURITY.md](SECURITY.md).

**License.** [Apache-2.0](LICENSE).

## 日本語

設計の言葉では「選択的 Dark Factory」と呼んでいる。Dark Factory は、床に人がいないので灯りを消して回る工場のこと。Andon はそのうち、止められるものは機械（テスト・ゲート）、判断が要るものは LLM、決断が要るものだけ人が見る。名前は TPS のアンドン（人を呼ぶ灯り）から。製造業のアンドン製品ではない。設計は docs/ に PR で入る。最終的に public にするつもりで建てる（秘密と、案件や個人に固有の値は入れない。`docs/design.md` §2.6）。

文書と commit メッセージにある `Y-<数>`（`Y-123` など）は、前身の仮設の工場の台帳の番号で、この repo の課題の id（`A-0001` の形）とは別物。履歴は書き換えない。

今の状態: 実験的で、1人で使っている。約束（コマンド・形式）は断りなく変わる。文書は日本語が中心。1人用の repo なので、今は PR を受け付けていない。脆弱性の知らせ方は [SECURITY.md](SECURITY.md)。

### 文書の地図

- [docs/vision.md](docs/vision.md) — ビジョン・ミッション・バリューの1枚（それぞれ既存の決定に遡れる形）と、工程・役・仕組みの図（今あるもの・無いものを分けて描く）。提案
- [docs/design.md](docs/design.md) — Andon とは何か（語彙と役: Chief Engineer・Chief of Staff・dispatcher・ゲート。リーン開発と TPS の系譜・委譲・Warp との違い）・repo の構成と道具の選定・散らばっているツールの棚卸し・仮設からの引っ越しとセルフホストの線・決めきれない点
- [docs/contract.md](docs/contract.md) — 機械向けの約束（台帳の項目・指摘・報告・問い・通知・ゲート・hc・フック・委譲・候補・流れの数え・人の手直し）。正は `schema/`（JSON Schema 10本。`tests/schema.sh` が例と突き合わせる）
- [docs/dispatcher.md](docs/dispatcher.md) — dispatcher（常駐の係）の設計: 何を拾い、Chief of Staff にいつどう届け（フック2つと離席中の待ち役）、どこまで動かすか（判断はしない）
- [docs/operate.md](docs/operate.md) — セルを立てて1本流す手順（ホストで打つ順。各段の「通ったら見えるもの」「落ちたら見る所」・設計書 §5.1 の確かめ方との対応表・フックと skill の実測の見る場所・いま通らない所の回り道と直しの課題）
- [docs/adr/](docs/adr/) — Chief Engineer の題ごとの ADR（番号・状態つき。設計書 §1.8）。全部の地図:
  - [0001 skill の配布](docs/adr/0001-skills-distribution.md): 決定
  - [0002 Herdr App の Space と tab への役の置き方](docs/adr/0002-herdr-spaces.md): 決定（作業側の Space と tab は 0003 が覆した）
  - [0003 作業側の Space を課題ごとにする](docs/adr/0003-task-spaces.md): 決定
  - [0004 `projects/<slug>/` の構成](docs/adr/0004-project-layout.md): 決定
  - [0005 セルの repo の直下と、追跡・持ち運び・守り](docs/adr/0005-cell-repo.md): 決定
  - [0006 調べる・決める・直すを分ける](docs/adr/0006-survey-fix-decide.md): 提案
  - [0007 仮設の工場にあって Andon に無いものの棚卸しと、塞ぐ順](docs/adr/0007-interim-gap-inventory.md): 提案
  - [0008 台帳のカードを「課題（issue）」に改名する](docs/adr/0008-issue-naming.md): 提案
  - [0009 Chief of Staff を人の既定の入口にする](docs/adr/0009-cos-default-entry.md): 決定
  - [0010 通知を「記録だけ」と「届ける」に分ける（`andon news ack`）](docs/adr/0010-news-ack.md): 提案
  - [0011 信号の受け口と価値のループ](docs/adr/0011-signals-and-value-loop.md): 提案
  - [0012 止まった担当を resume で起動し直す・worktree の置き場](docs/adr/0012-resume-and-worktree-home.md): 提案
  - [0013 役の呼び名を短くし、人に向けた日本語をふだんの言い方に揃える](docs/adr/0013-short-role-names-and-plain-japanese.md): 決定
  - [0014 課題の見込みと実際のズレを測り、Chief of Staff に届ける](docs/adr/0014-issue-estimate.md): 提案
  - [0015 inbox を Needs you と役への入口に絞る](docs/adr/0015-inbox-needs-you.md): 提案
  - [0016 セルの wiki を LLM Wiki の手法に乗せる](docs/adr/0016-cell-wiki-llm-wiki.md): 決定
  - [0017 仕事の流れと状態を定義し、その上で見張り役（dispatcher）を細くする](docs/adr/0017-process-definitions.md): 決定
  - [0018 担当が画面を見て確かめる道具](docs/adr/0018-screen-check-tooling.md): 決定
  - [0019 人の書きかけに `<news>` を混ぜない](docs/adr/0019-cos-input-box.md): 提案
  - [0020 auto memory を気づきの受け皿にして wiki へ蒸留する](docs/adr/0020-auto-memory-as-intake.md): 提案
  - [0021 pstack をセルに入れて試す](docs/adr/0021-pstack-in-cell.md): 決定
  - [0022 Chief Engineer の役は残し、Chief of Staff に Advisor（Fable）を試験運用で付ける](docs/adr/0022-advisor-and-chief-engineer.md): 提案
- [docs/decisions.md](docs/decisions.md) — 決定の記録（日付・出典）。設計書 §6 の人の答えと、仮設の設計文書の決定。2026-11-07 からの決定は [docs/decisions/](docs/decisions/) に1件1ファイル
- [AGENTS.md](AGENTS.md) — この repo で働く担当の指針
- [REVIEW.md](REVIEW.md) — この repo のレビュアーの観点
- [SECURITY.md](SECURITY.md) — 脆弱性の知らせ方
- [LICENSE](LICENSE) — Apache-2.0

### ディレクトリ

`docs/design.md` §2.2 の構成のとおり。中身がまだ無いディレクトリは `.gitkeep` だけを置く（`schema/ledger/`・`package/` の `bin/`・`cmd/kanban/`）。`package/` には skill（`skills/`）・プロファイル（`templates/`）・ゲートの条件表（`gate.json`）・managed settings の既定（`settings.json`）・statusline（`lib/andon/`）・Claude Code のフック（`hooks/`。Stop・UserPromptSubmit）・セルの compose（`compose.yaml`）・ホスト用の雛形（`host/`）が入っている（セルへ配る仕組みはまだ無い）。中身は P3 以降の PR で入る（手の順は設計書 §5.2）。

### セルの考え方

Andon は一人のソフトウェア工場で、セルはその中の持ち場にあたる。信頼の境界（1つの顧客・1つの趣味など）ごとに1つ立てて長く動かし続けるコンテナで、セルの repo（`<slug>-cell`。コンテナの中の `/workspace`）と、それを動かすイメージの2つでできている。

考え方の中心は、**セルごとのルートディレクトリがそのまま git のリポジトリ（セルの repo）になっている**こと。プロジェクト（`projects/<slug>/`）・案件の repo（`.ghq/`）・wiki（`wiki/`）・セルの指針（`AGENTS.local.md`）は、みなこのディレクトリの下にある。根は工場全体に1つではなく、セルごとに1つある。ホームディレクトリに近いと考えてよい。設定とコンテキストが置いてあり、作業はいつもここから始まる。

| 考え方 | 何がうれしいか |
|---|---|
| **セルの repo が、プロジェクトに要るものを持つ。** プロジェクトの指針（`projects/<slug>/AGENTS.md`）と skill（`projects/<slug>/.claude/skills/`）・セルの指針（`AGENTS.local.md`）・wiki・台帳（`projects/<slug>/kanban/`） | エージェントは起動するたびに、同じ場所から同じコンテキストを読む。指針・skill・wiki は git の履歴に残るので、別のマシンへ持ち運べ、壊れても clone から立て直せる。台帳は同じディレクトリにあるが git の外に置く（経過の記録で、セルの寿命のもの）。 |
| **イメージが境界になる。** セルは案件（信頼の境界）ごとに1つ立てる | 案件のあいだで、ファイル・資格情報・ログインが混ざらない。境界の中では、エージェントに「このフォルダを信頼するか」を毎回聞かずに任せられる。 |
| **要る道具はイメージが持つ。** エージェントの CLI（Claude Code・Codex）・端末の herdr と hc・Playwright と焼いたブラウザ・Andon の command と skill（`/opt/andon`） | どのセルでも同じ道具が同じ版で揃い、ホストに要るのは docker と bash だけ。セルに固有の道具は、セルの repo の `local/` で足す。セルの repo にパッケージのファイルは無いので、Andon を直したら `FROM` の版を上げる PR 1本で全セルへ広がる。 |
| **案件の repo は、セルの中へ追加するだけ。** `.ghq/` に clone し、担当は課題ごとの worktree（`.worktrees/`）で働く | 案件の repo に台帳・セルの設定・作業の途中のものを置かない。案件の repo に入るのは担当の PR だけで、ふだんの git の repo のまま使える。`.ghq/` と `.worktrees/` はセルの git の外。 |

何を追跡し何を外すか・別のマシンへの動かし方は [ADR-0005](docs/adr/0005-cell-repo.md)、`projects/<slug>/` の中は [ADR-0004](docs/adr/0004-project-layout.md)、wiki の形は [ADR-0016](docs/adr/0016-cell-wiki-llm-wiki.md)、語は [docs/design.md](docs/design.md) §0.1、パッケージがセルのどこに効くかは同じく §2.3、イメージの中身は [docs/image.md](docs/image.md)。

### セルを立てる

セルの repo は `andon init <slug>` で作る（`<slug>-cell`。雛形は `init/`。作ったあとは Andon に追従しない）。ホストに要るのは docker と bash だけ。

```bash
andon init <slug> --image ghcr.io/<owner>/andon:<版> --owner <email>
cd <slug>-cell && bin/bootstrap            # イメージを焼き、compose.yaml をイメージから取り出して起動し、herdr に入る
bin/bootstrap host-install                  # ホストの andon-up・ssh config・herdr の設定を置く（今あるものは上書きしない）
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）はセルの `.env` に書く。`bin/bootstrap --dry-run` は docker を打たずに、打つものを出す。

セルの中では `andon create <slug>` でプロジェクトを立て（`projects/<slug>/` に台帳・`project.json` の下書き・`AGENTS.md`。足りないものは聞く。`project.json` は人が見て commit する）、`andon open <slug>` でプロジェクトの Chief of Staff を開く（`projects/<slug>/` に cd して `andon chief-of-staff` を打つのと同じ）。slug を省くと、台帳のあるプロジェクトの一覧から fzf で選ぶ。

### テスト

```bash
for f in tests/*.sh; do bash "$f"; done
```

PR ごとに GitHub Actions が同じものを回す（`.github/workflows/tests.yml`）。`tests/public.sh` は、秘密の形とrepo の外に置いた固有の値の禁止リストを全ファイルに当てる（`tests/README.md`）。
