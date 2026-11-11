# Chief of Staff が提供元に切られて止まっても「続けて」を打たず、同じ止まりにログを1行だけ残す。入力欄を読む仕組みは消す

- 日付: 2026-11-11
- 出典: ADR-0019 決めたこと 4・5（[adr/0019-cos-input-box.md](../adr/0019-cos-input-box.md)）。課題 A-0134（切る案 W5）
- 覆す: decisions.md §16 の行（2026-11-06。提供元に切られたときの「続けて」）のうち、Chief of Staff に入力欄が空と確かめられたときだけ打つ分

- dispatcher は Chief of Staff の端末に「続けて」を打たない。提供元に切られて `error` で止まったら（見分けは担当と同じ）、ログに1行だけ残す。同じ止まりには1回で、`error`・`working` の間は持ち越し、ほかの状態に戻ったら区切る（Claude Code 自身の約 3 分のやり直しで `working` を挟んでも1行）。回数（控えの `Retry`）は Chief of Staff の分を持たない。通知は積まない（知らせる相手が止まっている）。人は Herdr App の `error` で気づく。
- 入力欄を読む仕組み（`hc attach --observe` の ANSI から薄い字を除いて入力欄が空かを見る判定）を消した。dispatcher が Chief of Staff の端末に打つのは `hc get`（と `status.msg` が無い `error` のときの `hc read`）だけ。
- 担当・レビュアーへの「続けて」は今のまま（1・2・4・8 分あと、5 回目に切られたら知らせる）。
