<p><picture><source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo/andon-wordmark-dark.svg"><img src="docs/assets/logo/andon-wordmark.svg" alt="andon" height="64"></picture></p>

# Andon

**Andon — a one-person software factory.** AI coding agents take tasks from a ledger through implementation, review and merge, and the human is called only for the decisions that only they can make.

一人で使うソフトウェア工場。エージェントが台帳のタスクを実装・レビュー・マージまで流し、人は自分にしかできない判断のときだけ呼ばれる。

## English summary

**What it is.** Andon — a one-person software factory — is the package, the contracts and the documents for running a software "line" with AI agents. One long-lived container (a *plant*) is set up per trust boundary (one client, one hobby, ...). Inside it, a coordinating agent (the *Chief of Staff*) reads a file-based task ledger, starts one worker agent per task in its own git worktree, asks a reviewer agent of a different model family to review the pull request, and lets a gate decide from rules what can merge without a person. The human is called (the andon light turns on) only for five kinds of decisions: what to build, how users should experience it, promises to the outside, irreversible risks, and changes to the line itself. Which of these a person must answer can be delegated per project, except what to build.

The design calls this a *selective dark factory*. A "dark factory" is a factory that runs with the lights off because no person is on the floor. Andon is selective: machines stop what machines can decide (tests, gates), language models handle what needs judgment, and a person sees only what is left. The name comes from the andon cord and light of the Toyota Production System. Andon is not a manufacturing-floor andon product.

**Who it is for.** One developer who already runs several coding agents in terminals and wants them to keep working while the developer is away, without giving up control of merges and irreversible operations. It is built for its author's own use first.

**Status.** Experimental. It is used by one person, and the interfaces change without notice. Most documents are written in Japanese; English is added only where needed. There are no releases with compatibility promises yet. The host must be Linux or WSL with Docker and bash. The plant image bundles agent CLIs (Claude Code, Codex) and the terminal workspace herdr; you bring your own accounts for them.

**Getting started.** The step-by-step procedure (Japanese) is [docs/operate.md](docs/operate.md): pull the image, create a plant repo with `andon init <slug>`, run `bin/bootstrap`, then start the Chief of Staff and run one task. The commands are also in [Set up a plant](#建屋を立てる) below.

**Map.**

| Path | What is there |
|---|---|
| [docs/design.md](docs/design.md) | What Andon is, its vocabulary, the repo layout and tool choices (the main design document) |
| [docs/vision.md](docs/vision.md) | Vision, mission and values on one page, with a figure of the line (stages, roles, mechanisms, and what Andon does not have yet). Proposal |
| [docs/contract.md](docs/contract.md), [schema/](schema/) | Machine-readable contracts (ledger keys, findings, reports, questions, notifications, gate, delegation). `schema/` is the source of truth |
| [docs/operate.md](docs/operate.md) | How to set up a plant and run one task |
| [docs/dispatcher.md](docs/dispatcher.md), [docs/ledger.md](docs/ledger.md), [docs/image.md](docs/image.md) | The dispatcher (resident notifier), the ledger CLI, the container image |
| [docs/adr/](docs/adr/), [docs/decisions.md](docs/decisions.md) | Architecture decision records and the decision log |
| `cmd/andon`, `cmd/kanban`, `internal/` | The `andon` CLI and the ledger CLI (`kanban`, alias `deck`), in Go |
| `package/` | What is shipped into the plant at `/opt/andon`: skills, agent profiles, gate rules, managed settings, hooks, compose file, host templates |
| `image/`, `init/` | The base image, and the template that `andon init` copies into a new plant repo |
| `tests/` | Shell tests run on every pull request, including `tests/public.sh` (no secrets or personal values) |

**Contributing and security.** This is a one-person project and does not take pull requests for now. To report a vulnerability, see [SECURITY.md](SECURITY.md).

**License.** [Apache-2.0](LICENSE).

## 日本語

設計の言葉では「選択的 Dark Factory」と呼んでいる。Dark Factory は、床に人がいないので灯りを消して回る工場のこと。Andon はそのうち、止められるものは機械（テスト・ゲート）、判断が要るものは LLM、残りだけ人が見る。名前は TPS のアンドン（人を呼ぶ灯り）から。製造業のアンドン製品ではない。設計は docs/ に PR で入る。最終的に public にするつもりで建てる（秘密と、案件や個人に固有の値は入れない。`docs/design.md` §2.6）。

今の状態: 実験的で、1人で使っている。約束（コマンド・形式）は断りなく変わる。文書は日本語が中心。1人用の repo なので、今は PR を受け付けていない。脆弱性の知らせ方は [SECURITY.md](SECURITY.md)。

### 文書の地図

- [docs/vision.md](docs/vision.md) — ビジョン・ミッション・バリューの1枚（それぞれ既存の決定に遡れる形）と、工程・役・仕組みの図（今あるもの・無いものを分けて描く）。提案
- [docs/design.md](docs/design.md) — Andon とは何か（語彙と役: Chief Engineer・Chief of Staff・dispatcher・ゲート。リーン開発と TPS の系譜・委譲・Warp との違い）・repo の構成と道具の選定・散らばっているツールの棚卸し・仮設からの引っ越しとセルフホストの線・決めきれない点
- [docs/contract.md](docs/contract.md) — 機械向けの約束（台帳の項目・指摘・報告・問い・通知・ゲート・端末の土台・フック・委譲）。正は `schema/`（JSON Schema 7本。`tests/schema.sh` が例と突き合わせる）
- [docs/dispatcher.md](docs/dispatcher.md) — dispatcher（常駐の係）の設計: 何を拾い、Chief of Staff にいつどう届け（フック2つと離席時の送り）、どこまで動かすか（判断はしない）
- [docs/operate.md](docs/operate.md) — 建屋を立てて1本流す手順（ホストで打つ順。各段の「通ったら見えるもの」「落ちたら見る所」・設計書 §5.1 の確かめ方との対応表・フックと skill の実測の見る場所・いま通らない所の回り道と直しのタスク）
- [docs/adr/](docs/adr/) — Chief Engineer の題ごとの ADR（番号・状態つき。設計書 §1.8）。[0001 skill の配布](docs/adr/0001-skills-distribution.md): 建屋の中は plugin `andon` と `.agents/skills` の symlink・役の skill を system prompt に足す。外は Andon が届く所に届く。やらないこと: MCP・marketplace。[0002 Herdr App の Space](docs/adr/0002-herdr-spaces.md): 指揮側と作業側の Space・tab はタスク・口は hc のラベルだけ・試してから指揮側の割り方を決め直す。[0003 タスクごとの Space](docs/adr/0003-task-spaces.md): 作業側を Space ＝ タスクにし、App の Files・Changes がタスクの worktree を指す
- [docs/decisions.md](docs/decisions.md) — 決定の記録（日付・出典）。設計書 §6 の人の答えと、仮設の設計文書の決定
- [AGENTS.md](AGENTS.md) — この repo で働く担当の指針
- [REVIEW.md](REVIEW.md) — この repo のレビュアーの観点
- [SECURITY.md](SECURITY.md) — 脆弱性の知らせ方
- [LICENSE](LICENSE) — Apache-2.0

### ディレクトリ

`docs/design.md` §2.2 の構成のとおり。中身がまだ無いディレクトリは `.gitkeep` だけを置く（`schema/ledger/`・`package/` の `bin/`・`cmd/kanban/`）。`package/` には skill（`skills/`）・プロファイル（`templates/`）・ゲートの条件表（`gate.json`）・managed settings の既定（`settings.json`）・statusline（`lib/andon/`）・Claude Code のフック（`hooks/`。Stop・UserPromptSubmit）・建屋の compose（`compose.yaml`）・ホスト用の雛形（`host/`）が入っている（建屋へ配る仕組みはまだ無い）。中身は P3 以降の PR で入る（手の順は設計書 §5.2）。

### 建屋を立てる

建屋の repo は `andon init <slug>` で作る（`andon-<slug>`。雛形は `init/`。作ったあとは Andon に追従しない）。ホストに要るのは docker と bash だけ。

```bash
andon init <slug> --image ghcr.io/<owner>/andon:<版> --owner <email>
cd andon-<slug> && bin/bootstrap            # イメージを焼き、compose.yaml をイメージから取り出して起動し、herdr に入る
bin/bootstrap host-install                  # ホストの andon-up・ssh config・herdr の設定を置く（今あるものは上書きしない）
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）は建屋の `.env` に書く。`bin/bootstrap --dry-run` は docker を打たずに、打つものを出す。

建屋の中では `andon open <slug>` でプロジェクトの Chief of Staff を開く（`projects/<slug>/` に cd して `andon chief-of-staff` を打つのと同じ）。slug を省くと、台帳のあるプロジェクトの一覧から fzf で選ぶ。

### テスト

```bash
for f in tests/*.sh; do bash "$f"; done
```

PR ごとに GitHub Actions が同じものを回す（`.github/workflows/tests.yml`）。`tests/public.sh` は、秘密の形とrepo の外に置いた固有の値の禁止リストを全ファイルに当てる（`tests/README.md`）。
