# base が動いて通らなかった自動マージは、gate から照合し直して3回まで打ち直す

- 日付: 2026-11-22
- 出典: gate が approve した自動マージの PR（https://github.com/mitaku/andon/pull/485 ）の `gh pr merge` が、別の PR（https://github.com/mitaku/andon/pull/483 ）のマージの1秒後に「Base branch was modified. Review and try the merge again.」で失敗し、メンテナーのマージ待ちになった。

`andon issue merge` は、この文の失敗だけを問いにせず、決めたことを書かずに終わる。次に打たれたとき（dispatcher の次の見回り）に gate から回し直し、マージの直前の読み直しも通ったときだけ打ち直す。

- head が動いた、衝突した、gate の判定が変わったときは、回し直した gate の答えに従い、打たない。
- 打ち直しは3回まで（仮の値）。回数は merge の記録の `retries` に持ち、決まったら消す。超えたら今までどおり問いにする。
- ほかの失敗の扱い、gate の判定、委譲、`--match-head-commit` は変えない。

引き受けたリスク:

- 打ち直しごとに gate が回るので、PR に gate のコメントが最大3つ増え、REST の読みと検査も増える。
- GitHub の失敗の文が変われば当たらず、今までどおり問いになる。
- GitHub が衝突を計算し終える前は gate が衝突を見られない。そのときは GitHub がマージを断り、問いになる。

捨てた案:

- **`gh pr merge` だけをその場で打ち直す**: base が動いたあとの衝突と判定を見ずに打つことになる。
- **その場で待って gate から回し直す**: 待つ間は課題の lock と dispatcher の見回りが止まる。次の見回りに回せば待ちは要らない。
