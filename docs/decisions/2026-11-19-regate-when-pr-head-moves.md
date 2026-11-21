# gate が決めたあとに PR の head が動いたら、dispatcher が gate を回し直す

- 日付: 2026-11-19
- 出典: オーナーの決定 2026-11-19（gate のあとに head が動いたら「ゲートを自動で回し直すほうで」）

- `andon issue merge` は、gate が approve か `needs_human` を決めたあと（dispatcher がまだマージしていない）も、`review` の課題の PR の head を課題ごとに1分に1回だけ聞く。OPEN の PR の head が gate の判定した head と違えば、前の head の gate の問い（`from: gate`）を閉じ、今の head で gate を回し直す（dispatcher.md §5 の P6a）。
- `fix` を決めたあとは聞かない。担当が `doing` に戻って `andon issue ready` からやり直すので、そこで新しい review の行として gate が回る。
- MERGED・CLOSED の PR は回し直さない。オーナーが GitHub で自分でマージするのは止めない（オーナーのマージは settle の道。dispatcher.md §2.7）。
- 捨てた案: 回し直さずに、head が動いたことを Chief of Staff からオーナーに知らせる形。
