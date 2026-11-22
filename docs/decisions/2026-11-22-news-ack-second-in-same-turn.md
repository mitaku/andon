# 確かめ済みの `news.ack` は dispatcher が消し、同じ turn の2回目の `<news>` の ack も数える（ADR-0010 決めたこと 8 の続き）

- 日付: 2026-11-22
- 出典: 課題 A-0305（調べる課題 A-0302 の候補 c1）。推奨で進めたもの（オーナーの答えは PR のマージ）

- 困りごと: `Stop` の block の続きは hc の `turn` が進まない。同じ turn に `<news>` を2回渡して2回 ack すると、2回目の ack は前の ack と同じ値のラベルになり、渡すときに控えた `ack_before` と同じなので `acked: false` と記録されていた。
- 決めたこと: dispatcher は、`<news>` の turn が終わって確かめ済みにしたとき（Stop の中でも、lock の外で見た終わりを次の drain が拾うときも）、ラベル `news.ack` を消す。次の `<news>` を渡すときの `ack_before` は空になり、2回目の ack を数える。ack の無い2回目はラベルが無いので数えない。ラベルを消せなかったときは前の値を `ack_before` に控え、数えない側に倒す。
- 変わらないもの: `andon news ack` の書き方（`news.ack=<turn>`）と `news.replied` の約束。ボードが読むのは `news.replied` だけで、`news.ack` は Chief of Staff と dispatcher の間のラベル。
