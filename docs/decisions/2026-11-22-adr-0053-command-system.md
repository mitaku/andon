# ADR-0053: andon のコマンドを「名詞 動詞」に組み直し、どこで打っても動くようにする

- 日付: 2026-11-22
- 出典: ADR-0053（[adr/0053-command-system.md](../adr/0053-command-system.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-22「andon commandのリファレンスとコマンドの体系を一度整理してみませんか」）

`andon` のコマンドを、`andon <名詞> <動詞>` の2語に組み直す。物差しは Command Line Interface Guidelines（clig.dev）にする。1語で打てるのは、andon そのものを扱う `open`・`help`・`version`・`boot` だけにする。役の名前だけのコマンドをやめ、`andon chief-of-staff` は `andon open` に寄せ、`andon chief-engineer` は `andon design start` にする。どのコマンドもセルの中のどこで打っても動き、プロジェクトは `--project`、`KANBAN_ROOT`、今いるフォルダ、セルに1つだけ、の順で決める。コマンドの表を1つ持ち、振り分けと `andon help` と人向けの一覧 `docs/commands.md` と補完をそこから作る。人向けの文書はその1枚だけで、詳細は `--help` が持つ。名前は一度に替え、別名は作らない。古い名前を打つと、新しい名前を言って断る。別のバージョンから呼ばれる `andon boot` と `andon issue session`、フックが打つコマンドは名前を変えない。担当の課題を足すのと予約キーを書くのを1回で打つ `andon issue create` を足し、依頼書の Settings 欄は command が書く。kanban の CLI は名前を変えない。配る文の中のコマンドが表にあることを検査で見る（ADR-0001 決めたこと 5 のうち、検査を入れないという部分を覆す）。ADR と決定の記録の中の古い名前は替えない。
