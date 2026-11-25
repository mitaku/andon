# dispatcher のマージ

gate が approve を決め、`merge: auto` で委譲の範囲の中なら、dispatcher が `gh pr merge` を打ってマージし、`gh pr view` で PR を読み直して `MERGED` とマージの commit を確かめてから、gate の記録に `merged` を書いて片付ける。`gh pr merge` が失敗したときも PR を読み直し、オーナーが先に同じ head でマージしていれば、問いにせずオーナーのマージの道へ渡す。どちらの読み直しも、`gh pr view` が一時に落ちただけでは問いにせず、間をおいて3回まで読み直す。3回とも読めなければ、確かめずに片付けず、オーナーへの問いにする。

## 細かい機能

- `mg-reread` `gh pr merge` が通ったあとの読み直しが1回落ちても問いにせず、読み直して `MERGED` を確かめ、gate の記録に `merged` を書く。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/merge.go`
- `mg-reread-exhausted` マージのあとの読み直しが3回とも落ちたら、gate の記録に `merged` を書かず、課題を片付けず、問いを1つ出す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/merge.go`
- `mg-reread-race` オーナーが先にマージしていて `gh pr merge` が失敗し、そのあとの読み直しが1回落ちても問いにせず、読み直して `MERGED` と同じ head を確かめ、オーナーのマージの道へ渡す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/merge.go`

## 辿り着き方（利用者から）

- 担当が `andon issue ready <id>` で `review` に入れ、Chief of Staff が `merge: auto` を書いた課題を、dispatcher の見回りが gate からマージまで進める。
- オーナーは、マージできなかった課題の問い（`from: gate`）だけを受け取る。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh merge <検証する commit>` を打つ。偽のセルの repo（オーナーが条件表と委譲 preset work を commit したもの）のプロジェクト `mg` で、本物の `andon dispatcher --once --project mg` を回す。課題は担当が文書1行の PR で `andon issue ready` まで進めた `merge: auto` のもの（レベル A）。偽の `gh` の `gh pr merge` は記録してマージ済みにし、読み直しを決めた回数だけ 502 で落とす。
- **1回落ちる。** `mg-reread` の行に、巡の出力（「PR … をマージした」）、偽の `gh` への `gh pr view 7 … --json state,mergeCommit` が2行、`status`・gate の記録の `merged: true`・問い `[]` が出る。`ok   mg-reread` が見えれば動いた。
- **3回とも落ちる。** `mg-reread-exhausted` の行に、巡の出力（「… をマージしない: マージを打ったが PR を読み直せない（3 回）」）、読み直しが3行、`status: review`・`merged` が true でないこと・開いた問い1つが出る。`ok   mg-reread-exhausted` が見えれば、確かめずに片付けていない。
- **オーナーが先にマージしていた。** `mg-reread-race` の行に、巡の出力（「… は gh pr merge の前に同じ head でマージされていた」）、`gh pr view 7 … --json state,mergeCommit,headRefOid` が2行（読み直し）、問い `[]` が出る。`ok   mg-reread-race` が見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/issue/merge.go`

## 既に見ている検査

- `tests/andon-dispatcher.sh`（4d-2〜4d-4: オーナーが先にマージしたとき、マージの前後の読み直しが1回落ちたとき）
- `go test ./internal/issue/`（`MergedBefore`・`RetryMerge`）

## 落とし穴

- 偽の `gh` は、読み直しの欄（`state,mergeCommit` と `state,mergeCommit,headRefOid`）ごとに落とす回数を持つ。settle が打つ読み（`statusCheckRollup` などの欄を足したもの）は落とさない。欄の違う読みを数えると、読み直しの回数がずれる。
- 3回とも落ちる回は、読み直しの間に2秒ずつ待つので、巡が4秒ほど長くなる。止まったと読まない。
- 切り離した場所の課題の worktree は、本物の clone の worktree でない。dispatcher がマージしたあとの finish は片付けに失敗し、「マージしたが片付けに失敗した」の問いが出る。道はこの問いを数えず、マージできなかった問い（「dispatcher がマージできなかった」）だけを数える。同じ巡の settle が、マージできなかった問いを「オーナーがマージした」と閉じることもあるので、問いの開閉でなく、出た問いの文で読む。
- 当てたコードが直す前のもの（読み直しを打ち直さない）なら、3つとも FAIL になる。検証する commit が当たっているか、`andon version` の commit で確かめる。
