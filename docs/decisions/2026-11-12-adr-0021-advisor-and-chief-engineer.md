# ADR-0021: Chief Engineer の役は残し、Chief of Staff にだけ Advisor（Fable）を試験運用で付ける

- 日付: 2026-11-12
- 出典: ADR-0021（[adr/0021-advisor-and-chief-engineer.md](../adr/0021-advisor-and-chief-engineer.md)。Chief Engineer の課題 A-0161。オーナーが対話の中で決めた。元は人の発言 2026-11-12「あとは Claude の Advisor 機能を使うことで ChiefEngineer を除外できるのでは？とも思っている」）

Claude Code の Advisor（メインのモデルが途中で強いモデルに助言を求める機能）で Chief Engineer の役を外す案は採らない。Advisor が代われるのは強いモデルの知恵だけで、設計の対話を Chief of Staff の会話と分けること（題を並べて詰められる・ラインの通知と混ざらない）は代われないため。振り分けの線（判断の題は Chief Engineer へ）も動かさない。Advisor（Fable）は Chief of Staff にだけ付け、入れ方は `chief-of-staff` のプロファイルの起動行に `--advisor fable`。Chief of Staff の skill の「人の入口」に、振り分けに迷うときと2回差し戻して切り直すときは advisor に相談する、の1行を足す。Advisor は experimental で利用上限にも数えられるので、試験運用として入れる: design.md §1.8 は書き換えず、2026-11-27 にオーナーが目視で効きを見て「続ける・止める・広げる」を決める。止めるときはフラグと1行を戻す。決めたこと 6 件・捨てた案（Chief Engineer をカスタムエージェントにする案を含む）・受け入れ条件・価値の仮説・切る案 V1〜V2 は ADR が正。
