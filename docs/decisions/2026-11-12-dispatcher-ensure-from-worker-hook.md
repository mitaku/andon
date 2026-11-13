# dispatcher の端末の起動し直しは、担当・レビュアーの Stop フックからも打つ

- 日付: 2026-11-12
- 出典: 課題 A-0095（ADR-0017 の切る案 P6）。ADR-0017 決めたこと 8 の (e)「dispatcher 自身の起動し直しを Chief of Staff のフック以外にも持つ。boot の定期か巡ごとの自己確認。P6 で決める」

- 担当・レビュアーの Stop フック（`andon issue stop-check`）は、止めるか通すかを決める前に、Chief of Staff のフックと同じ確かめ（`dispatcher.Ensure`。待たない）を打つ。core の起き直しで dispatcher の端末の中身がシェルに替わっていれば、片付けて起動し直す。
- 理由: dispatcher は Chief of Staff の留守にも回るので、Chief of Staff のフックだけでは留守の間に戻せない。留守の間に動いているのは担当で、そのターンの終わりごとにフックが走る。
- 捨てた案:
  - 巡ごとの自己確認: core の起き直しで消えるのは dispatcher のプロセスそのもの（hc の keep は失ったプログラムをシェルに替える）なので、巡は回らず確かめる者がいない。
  - boot の定期: boot はセルの起動ごとに1回で、定期に打つ仕組みが無い。足すと cron か常駐がもう1つ要る（dispatcher.md §7.2 #8 のとおり cron は置かない）。
- 残る隙: Chief of Staff も担当も動いていない間は戻らない。その間は担当の側から拾う出来事も出ないので、次に誰かのターンが終わったときに戻れば足りる。
