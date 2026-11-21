# 起動の記録は課題の library/launches.jsonl に起動1回1行で足し、Chief of Staff の分は kanban の chief-of-staff/ に置く

- 日付: 2026-11-19
- 出典: [ADR-0028](../adr/0028-improvement-loop.md) 決めたこと 3・7・14 の 5 と「実装が決めてよいこと」

ADR-0028 が実装に任せた細部を次のとおり決めた。形と欄の意味は [contract.md](../contract.md) §18・`schema/launch.schema.json`。

- **ファイル**: 課題の `library/launches.jsonl`。1回の起動に1行の JSON Lines で、追記だけ。「起動し直しても前の1件は変わらない」（受け入れ条件 2）を、前の行を読み書きせずに守れる。ADR の案の `launch.json`（1つの JSON）は、足すたびに全体を読み直して書くので、書きかけで前の記録を壊しうる。
- **Chief of Staff の置き場**: kanban の `chief-of-staff/launches.jsonl`。課題のフォルダと同じ形を、課題を持たない役のために kanban の直下に置いた。kanban の CLI は知らないフォルダを読まない。
- **書く時**: 値は `hc run` の直前に集め、`hc run` が端末を起動できたあとに端末の address を足して書く。起動できなかったときは書かない。書けなくても起動は戻さない（端末はもう動いている）。
- **起動行**: 起動の前に template があるかを確かめるときに `hc templates --json` から読んだ `command` をそのまま使う（hc を2度呼ばない）。dispatcher のレビュアーの起動は確かめの段が無いので、集めるときに読む。
- **食い違いの印**: パッケージの置き場の食い違い（`andon version`）と、セルの repo の commit していない変更を数える。案件の repo の commit していない変更は数えない。起動し直しでは担当の作業中の変更があるのが普通で、印が毎回立つと振り返りが課題を根拠から外しすぎる。`repo.dirty` は事実として残す。
- **外の skill と plugin**: skill の置き場の、`SKILL.md` を持つ実ディレクトリ（`gh skill install` が `metadata` に書く repo・ref・tree の sha・バージョン）と、Claude Code の `installed_plugins.json`（バージョン・commit）。Andon のパッケージと image の備品を指す symlink は数えない（パッケージの commit と image で辿れる）。
- **auto memory の目録**: Claude Code の起動だけ写す。Codex は読まないので `null`。
