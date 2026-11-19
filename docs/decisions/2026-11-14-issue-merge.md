# ゲートのあとのマージと片付けを andon issue merge に出し、自分の記録 <slug>.merge.json を持つ

- 日付: 2026-11-14
- 出典: 課題 A-0097（[ADR-0017](../adr/0017-process-definitions.md) の切る案 P7b。決めたこと 5・6、受け入れ条件 7 の merge。コマンドの名前は「切る案の担当が揃える」とした仮の名）

ゲートを回してからマージと片付けまで（[dispatcher.md](../dispatcher.md) §5 の P6a）を、dispatcher の中から `andon issue merge <id>` に出した。

ここで置いたこと:

- 名前は仮の名のまま `andon issue merge`。merge は `andon gate` を同じプロセスで回し、`fix` は担当へ送り、`needs_human` は問いにする。approve ＋ `merge: auto` ＋ 委譲の範囲内なら、マージして finish する。ゲートの結果ごとの手は、ゲートの状態（`process/states/gate.yaml`）の1本にまとめたいので、マージだけを切り出さなかった。
- 記録は `~/.local/state/andon/dispatcher/<slug>.merge.json`（課題 → `row`・`sha`・`decision`・`merged`・`wait`・`approved`）で、書き手は merge だけ。dispatcher の控え `<slug>.json` から `gates`・`approved`・`waits` を外した。記録は `review` にいない課題の分を merge が捨てるので、終わった課題の分は溜まらない。
- gh の上限の待ち（`gh_reset`・`gh_wait`）は台帳ごとの待ちで、外の信号の collect も読むので dispatcher の控えに残した。merge は上限で落ちたら `state: rate-limited` を返すだけにした。
- 「approve のまま 10 分」はコマンドにしない（ADR-0017 決めたこと 6）。merge は、マージに進み始めた時刻を記録の `approved` に書き、決まったら消す。dispatcher はそれを読み、10 分たっても残っていれば通知 `gate` を1回だけ積む。二度積まないための控えは `stuck`（課題 → 届けた `approved`）。dispatcher.md §7.2 には、この処理の行（#25）を足した（A-0091 の Next から頼まれた判断）。
- 出力は stdout の JSON（`issue`・`state`・`auto`・`log[]`・`notify[]`）で、待ちに積むのは dispatcher。出力も記録も schema にしない（settle・impact と同じく、約束は通知の `detail`。形は [contract.md](../contract.md) §6）。
- 記録の lock は台帳ごとの `<slug>.issue.lock` を使う。finish と問いは merge の中で打つ（別プロセスで打つと、同じ lock を待って止まる）。
- 記録が無いとき（入れ替えの直後・消えたとき）は、今の head（`report.json` の `commit`）のゲートの記録 `library/gate/<head>.json` がその review 入りのあとに書かれていれば引き継ぎ、同じ行の問いや通知を出し直さない。この引き継ぎがあるので、dispatcher は初めて見る台帳でも merge を打つ（前は、ゲートを回さずに記録だけ置いた）。
