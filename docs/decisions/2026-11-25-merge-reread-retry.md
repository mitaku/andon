# dispatcher のマージの前後の読み直しは、gh pr view が一時に落ちても 2 秒おいて 3 回まで読み直してから問いにする

- 日付: 2026-11-25
- 出典: Chief of Staff の依頼（2026-11-25）。base が動いたマージの打ち直しの課題の担当の報告が、マージが通ったあとの `gh pr view` が一時に落ちると、マージ済みでも「MERGED にならない」の問いになると書いた。直す前の木で、偽の `gh` の読み直しを1回だけ落とすと問いが出ることを `tests/andon-dispatcher.sh` で確かめた

- `andon issue merge` は、`gh pr merge` が通ったあとの読み直し（`gh pr view --json state,mergeCommit`）と、`gh pr merge` が失敗したあとの読み直し（`--json state,mergeCommit,headRefOid`）を、`gh` が失敗したら 2 秒おいて 3 回まで（初めの1回を含む）打ち直す（`internal/issue/merge.go` の `ghRereads`・`ghRereadWait`）。
- 3 回とも読めなければ、今までどおりに進む。マージが通ったあとなら、`MERGED` を確かめずに片付けず `irreversible` の問いにする。失敗したあとなら、base の動きの打ち直しか問い。
- 2 秒・3 回の理由: 落ちるのは GitHub の 502 などの一時の失敗で、数秒で戻る。打ち直しは課題の lock を持つ間に巡の中で待つので、待ちは最大 4 秒に抑える。仮の値で、問いが残るなら上げる。
- 失敗の種類（404・認証・rate limit）で打ち直しを分けない。待ちは最大 4 秒で、問いになる結果は変わらない。
- 3 回とも読めないとき、問いにせず次の見回りへ回す案は採らない。次の見回りは gate からやり直し、マージ済みの PR に `gh pr merge` を打ち直して、オーナーのマージとして片付けてしまうため。
- 変えないもの: gate の判定、settle、打ち直しを待つ間の読み直し（読めなければ gate へ進み、そのあと同じ読み直しを通る）、PR の head の見張り（読めなければ次の見回りで聞く）。
