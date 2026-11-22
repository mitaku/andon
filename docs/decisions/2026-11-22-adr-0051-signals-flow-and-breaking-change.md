# ADR-0051: 外の信号の受け口を実際に流し、main の失敗を壊した変更に結ぶ

- 日付: 2026-11-22
- 出典: ADR-0051（[adr/0051-signals-flow-and-breaking-change.md](../adr/0051-signals-flow-and-breaking-change.md)。Chief Engineer の課題。元はオーナーの発言 2026-11-21「Signalをもう少し強化したいと思っています。」）

外の信号の受け口（ADR-0011）は、Andon を開発するセルで一度も動いていなかった。読む出所を書く `project.json` の `signals` が、空のままだったからである。このセルでは、main の CI の失敗を `now`、issue を `later` で読む。issue への書き戻しは付けない。`andon create` は、repo を渡されたプロジェクトの下書きに、main の CI の失敗を `now` で読む 1 行を入れる。下書きはオーナーが commit するまで読まれない。`github-ci` の信号の `release` は、最新の run の commit ではなく、失敗が始まった run の commit にする。読んだ 100 件の run を古いほうへ辿って決める。出来事が続いている間、信号の記録の `release` は書き換えない。通知 `signal` の `detail` に `release` を足す。Chief of Staff は、その commit の PR を引き、失敗が始まった変更の候補として依頼書に書く。command は足さない。工場の中の出来事は、信号の記録に乗せない。着手できる課題が起動されないまま残る件は、同じ日に決まった別の ADR が扱うので、この文書では決めない。Sentry・Linear・Cloudflare を読む command は、ADR-0011 のまま、使う案件が出たときにする。
