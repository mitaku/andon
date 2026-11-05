# Andon

**English summary.** Andon is a software factory for one person: a selective "dark factory" where machines handle what machines can decide, LLMs handle what needs judgment, and a human is called only for the few decisions that are theirs. It is built as one repo with three kinds of readers (humans, AI agents, machines). It is not the manufacturing-floor andon product. The documents are in Japanese for now; English will be added when needed. Everything in this repo must be fit to be public (no secrets, no personal or project-specific values; see `docs/design.md` §2.6).

私のソフトウェアファクトリー（選択的 Dark Factory）。製造業のアンドン製品ではない。設計は docs/ に PR で入る。最終的に public にするつもりで建てる（秘密と、案件や個人に固有の値は入れない。`docs/design.md` §2.6）。

## 文書の地図

- [docs/design.md](docs/design.md) — Andon とは何か（語彙と役: Chief Engineer・Chief of Staff・クーリエ・ゲート。リーン開発と TPS の系譜・委譲・Warp との違い）・repo の構成と道具の選定・散らばっているツールの棚卸し・仮設からの引っ越しとセルフホストの線・決めきれない点
- [docs/contract.md](docs/contract.md) — 機械向けの約束（台帳の項目・指摘・報告・問い・通知・ゲート・端末の土台・フック・委譲）。正は `schema/`（まだ無い）
- [docs/decisions.md](docs/decisions.md) — 決定の記録（日付・出典）。設計書 §6 の人の答えと、仮設の設計文書の決定
- [AGENTS.md](AGENTS.md) — この repo で働く担当の指針
- [REVIEW.md](REVIEW.md) — この repo のレビュアーの観点
- [LICENSE](LICENSE) — Apache-2.0

## ディレクトリ

`docs/design.md` §2.2 の構成のとおり。中身がまだ無いディレクトリは `.gitkeep` だけを置く（`schema/`・`package/`・`image/`・`init/`・`cmd/kanban/`）。中身は P2 以降の PR で入る（手の順は設計書 §5.2）。

## テスト

```bash
for f in tests/*.sh; do bash "$f"; done
```

PR ごとに GitHub Actions が同じものを回す（`.github/workflows/tests.yml`）。`tests/public.sh` は、秘密の形とrepo の外に置いた固有の値の禁止リストを全ファイルに当てる（`tests/README.md`）。
