# Chief Engineer の端末の `stalled`・`blocked`・`ready` は記録だけ、`working` の `stalled` は進みが 20 分止まったときだけ（ADR-0010 決めたこと 1 の表に足す）

- 日付: 2026-11-10
- 出典: 人の依頼（2026-11-10「通知のノイズを直す課題を切って」）。課題 A-0036。推奨で進めたもの（人の答えは PR のマージ）

- Chief Engineer の課題（`profile: chief-engineer`）の `stalled`・`blocked`（問い・許可の確認とも）・`ready` は記録だけにする。Chief Engineer は人と直接話す役で、人の返事を待つ `idle` と `AskUserQuestion` の `blocked` が普通。Chief of Staff は人への問いに答えないので、届いても画面を読んで「人の返事待ち」と言い返すだけになる（2026-11-10 に A-0034・A-0035 で 7 回）。`ready` は `andon chief-engineer` が起動した直後に出るもので、もう起きている。`error`・`finished`・`review`（文書の PR）・`report`・`asked` は今までどおり届ける。
- 許可の確認の `blocked` も届けない。人が Chief Engineer と話している最中に出るもので、人がその場で答える。
- `working` のまま 20 分の `stalled` は、`status.msg` と画面の字（数字・記号・空白を除く）も 20 分替わらないときだけ届ける。`status_ms` は状態が変わった時刻なので、1つのターンで長く働く担当（A-0022・A-0044・A-0038）が止まったように見えていた。

捨てた案: 画面をそのまま比べる（経過の時計と回る印で毎回替わり、本当に止まった担当にも届かなくなる）。`status.msg` だけで比べる（Codex は `working` の間 `msg` を出さない）。
