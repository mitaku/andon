# `Stop` の block の続きは hc の `turn` が進まないので、渡す前から同じ値の `news.ack` は数えない。`andon news ack` は渡し中を見ずにラベルを書く（ADR-0010 決めたこと 8）

- 日付: 2026-11-10
- 出典: 課題 A-0011（ADR-0010 の「依頼書に切る案」2 の実装）。推奨で進めたもの（人の答えは PR のマージ）

- 実測（担当の端末で `andon issue stop-check` が block したとき）: `Stop` の `decision: block` で始まる続きは、hc の `turn` を進めない。ADR-0010 決めたこと 5 の「要実測」は、渡したときの `turn` と ack を打ったときの `turn` が同じになる側だった。控える値は変えない。
- そのため、同じ hc の `turn` の中で、ack を打ったあとに `Stop` の block で次の `<news>` を渡すと、前の ack が新しい `<news>` の ack に見える。経路①②は、渡すときのラベル `news.ack` の値を控え（控えの `hook.ack_before`）、渡す前からその値なら ack として数えない（`news.replied` を書かない。安全側）。経路③は送るのが入力待ちのとき（`turn` が進んだあと）なので、この重なりは起きない。
- `andon news ack` は ADR-0010 決めたこと 4 のとおり、`role=chief-of-staff` の端末なら渡し中かどうかを見ずにラベルを書く。渡し中が無いときの ack は、照らす控えが無いので `news.replied` を書かせない（依頼書の「渡し中が無ければ何もしない」は、この形で満たす）。
- 移行の間に `news.replied` を何で書いたか（`ack`・`no-reply`）は、dispatcher の控えの `replied` に数える（約束にしない。`<no-reply>` を外す版を決めるときに読む）。

捨てた案: `andon news ack` が dispatcher の控えを読んで渡し中の turn を書く（依頼書の書き方）。ADR-0010 決めたこと 4・捨てた案 5 がラベルと `turn` を選んでいて、控えの lock を Chief of Staff の command が取ることになる。
