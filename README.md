# Andon

**English summary.** Andon is a software factory for one person: a selective "dark factory" where machines handle what machines can decide, LLMs handle what needs judgment, and a human is called only for the few decisions that are theirs. It is built as one repo with three kinds of readers (humans, AI agents, machines). It is not the manufacturing-floor andon product. The documents are in Japanese for now; English will be added when needed. Everything in this repo must be fit to be public (no secrets, no personal or project-specific values; see `docs/design.md` §2.6).

私のソフトウェアファクトリー（選択的 Dark Factory）。製造業のアンドン製品ではない。設計は docs/ に PR で入る。最終的に public にするつもりで建てる（秘密と、案件や個人に固有の値は入れない。`docs/design.md` §2.6）。

## 文書の地図

- [docs/design.md](docs/design.md) — Andon とは何か（語彙と役: Chief Engineer・Chief of Staff・クーリエ・ゲート。リーン開発と TPS の系譜・委譲・Warp との違い）・repo の構成と道具の選定・散らばっているツールの棚卸し・仮設からの引っ越しとセルフホストの線・決めきれない点
- [docs/contract.md](docs/contract.md) — 機械向けの約束（台帳の項目・指摘・報告・問い・通知・ゲート・端末の土台・フック・委譲）。正は `schema/`（JSON Schema 7本。`tests/schema.sh` が例と突き合わせる）
- [docs/courier.md](docs/courier.md) — クーリエ（常駐の係）の設計: 何を拾い、Chief of Staff にいつどう届け（フック2つと離席時の送り）、どこまで動かすか（判断はしない）
- [docs/skills-distribution.md](docs/skills-distribution.md) — skill の配布の設計（建屋の中: plugin `andon` と `.agents/skills` の symlink・役の skill を system prompt に足す。外: Andon が届く所に届く。やらないこと: MCP・marketplace）
- [docs/decisions.md](docs/decisions.md) — 決定の記録（日付・出典）。設計書 §6 の人の答えと、仮設の設計文書の決定
- [AGENTS.md](AGENTS.md) — この repo で働く担当の指針
- [REVIEW.md](REVIEW.md) — この repo のレビュアーの観点
- [LICENSE](LICENSE) — Apache-2.0

## ディレクトリ

`docs/design.md` §2.2 の構成のとおり。中身がまだ無いディレクトリは `.gitkeep` だけを置く（`schema/ledger/`・`package/` の `bin/`・`hooks/`・`cmd/kanban/`）。`package/` には skill（`skills/`）・プロファイル（`templates/`）・ゲートの条件表（`gate.json`）・managed settings の既定（`settings.json`）・statusline（`lib/andon/`）・建屋の compose（`compose.yaml`）・ホスト用の雛形（`host/`）が入っている（建屋へ配る仕組みはまだ無い）。中身は P3 以降の PR で入る（手の順は設計書 §5.2）。

## 建屋を立てる

建屋の repo は `andon init <slug>` で作る（`andon-<slug>`。雛形は `init/`。作ったあとは Andon に追従しない）。ホストに要るのは docker と bash だけ。

```bash
andon init <slug> --image ghcr.io/<owner>/andon:<版> --owner <email>
cd andon-<slug> && bin/bootstrap            # イメージを焼き、compose.yaml をイメージから取り出して起動し、herdr に入る
bin/bootstrap host-install                  # ホストの andon-up・ssh config・herdr の設定を置く（今あるものは上書きしない）
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）は建屋の `.env` に書く。`bin/bootstrap --dry-run` は docker を打たずに、打つものを出す。

## テスト

```bash
for f in tests/*.sh; do bash "$f"; done
```

PR ごとに GitHub Actions が同じものを回す（`.github/workflows/tests.yml`）。`tests/public.sh` は、秘密の形とrepo の外に置いた固有の値の禁止リストを全ファイルに当てる（`tests/README.md`）。
