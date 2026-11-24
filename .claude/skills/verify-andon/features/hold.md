# PR を止める

Chief of Staff は、別の問い（理由の課題）の答えが出るまで課題の PR をマージさせたくないとき、`andon issue hold <id> <理由の課題>` で止める。止めると予約キー `hold` に理由の課題が入り、gate が今の commit を渡していれば PR の印 `andon/gate` が黄（「メンテナーの確認を待っています」）になる。止めている間は gate が回らず、dispatcher はマージしない（gate のあとに止めたら、その判定を捨てる）。担当が `hold` を消しても止めは外れず、外すのは Chief of Staff の `andon issue unhold <id>` だけ。

## 細かい機能

- `hd-refuse` 理由の課題が自分自身か片付いた課題なら、何も書かずに断る。見る道: 検証の担当だけ。
- `hd-hold` `hold` に理由の課題が Chief of Staff の端末の行で残り、渡している commit に黄の印を書いてから返る。見る道: 検証の担当だけ。
- `hd-gate-wait` 止めている課題の gate は GitHub を読まずに wait（終了コード 5）を返し、記録を書かない。見る道: 検証の担当だけ。
- `hd-worker` 担当（課題の `agent` の端末）が `hold` を消しても、止めたままと読む。見る道: 検証の担当だけ。
- `hd-unhold` Chief of Staff が外すと `hold` が消え、印は書かない。外したあとは「止めていない」と返す。見る道: 検証の担当だけ。
- `hd-dispatch-hold` `merge: auto` の課題を gate の前に止めると、dispatcher の巡は gate を回さず、マージもしない。見る道: 検証の担当だけ。
- `hd-dispatch-release` 止めを外すと、次の巡で gate が回り、approve で dispatcher がマージする（`hd-dispatch-hold` の対照）。見る道: 検証の担当だけ。
- `hd-dispatch-discard` gate が approve を決めたあとに止めると、次の巡で dispatcher が判定を捨て、`merge: auto` に変えてもマージしない。見る道: 検証の担当だけ。

## 辿り着き方（利用者から）

- Chief of Staff が、オーナーに待つよう頼む前に `andon issue hold <id> <理由の課題>` を打つ（Chief of Staff の手引き `package/skills/chief-of-staff/playbooks/gate.md` の「PR を止める」）。
- 答えが出て担当が直したあと、Chief of Staff が `andon issue unhold <id>` を打つ。
- 理由の課題が done になると、dispatcher が止めている課題ごとに通知 `hold` を Chief of Staff へ届ける（外し忘れの知らせ）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh hold` を打つ。切り離した kanban に、担当の端末 `tw` が `kanban start` で受け持って `andon issue report --pr` で報告した課題と、理由の課題・片付いた課題を作り、Chief of Staff の端末 `tc` として止めて外す。偽の `gh` は、PR を読む `gh pr view <n> --repo <owner>/<repo> --json state,mergeCommit,headRefOid,statusCheckRollup`（印 `andon/gate` が緑の PR を返す）と、印を書く `gh api -X POST …/statuses/<sha>` だけを知る。
- **断られる。** `hd-refuse` の行が2つ出る。自分自身と片付いた課題を理由に渡した `andon issue hold` が終了コード 1 で理由1行を返し、`hold` が空のままで、`ok   hd-refuse` が2行見えれば動いた。
- **止める。** `hd-hold` の行に `andon issue hold` の出力（「PR の印を黄にした」）、`kanban show` の `hold`、偽の `gh` への2つの呼び出し（`state=pending` と説明文「メンテナーの確認を待っています」）が出る。`ok   hd-hold` が見えれば動いた。
- **gate が待つ。** `hd-gate-wait` の行に `andon gate` の終了コード 5 と、理由の課題を名指す `{"decision":"wait",…}` が出る。`ok   hd-gate-wait` が見えれば、GitHub を読まず記録も書いていない。
- **担当が消しても止まる。** `hd-worker` の行の前に、担当 `tw` が `kanban set <id> hold=` を打った終了コードと今の `hold` が出る。道は、担当が消せた（`hold` が空で、`hold` を最後に変えた行が `terminal:tw`）ことを確かめてから `andon gate` を打つ。gate がまた wait を返し、`ok   hd-worker` が見えれば動いた。担当が消せていなければ `FAIL hd-worker: 担当が hold を消せていない` が出る（`hold` の欄が埋まったままだと gate はそれだけで wait を返し、担当が消した止めの読み方を通らない）。
- **外す。** `hd-unhold` の行に `andon issue unhold` の出力（「止めを外した（理由の課題 …）」）と、もう一度外したときの「止めていない」が出る。`ok   hd-unhold` が2行見え、最後の `log.jsonl` の行で `hold` を最後に変えたのが `terminal:tc` なら動いた。
- **dispatcher の巡で確かめる。** 続けて、偽のセルの repo（オーナーが条件表と委譲 preset work を commit したもの）のプロジェクト `hd` で、本物の `andon dispatcher --once --project hd` を回す。課題は担当が文書1行の PR を出して `andon issue ready` まで進めたもの（`review: off`・レベル A）。偽の `gh` は gate・マージ・読み直しの呼び出しを知るものに替え、`gh pr merge` は記録だけしてマージ済みにする。
  - `hd-dispatch-hold`: `merge: auto` の課題を止めてから回す。巡の出力と偽の `gh` への呼び出しが出る。gate の記録が無く、`gh pr merge` が呼ばれず、課題が review のままで、`ok   hd-dispatch-hold` が見えれば動いた。
  - `hd-dispatch-release`: 止めを外してまた回す。dispatcher の「PR … をマージした」の行・`gh pr merge … --match-head-commit <sha>`・gate の記録（`approve`・レベル A）が出て、`ok   hd-dispatch-release` が見えれば、止めがマージを止めていた。
  - `hd-dispatch-discard`: `merge: person` の課題で gate に approve を決めさせてから止め、`merge: auto` に書き換えて回す。dispatcher の「… は gate の判定（approve）のあとに止められた。判定を捨てて gate の問いを閉じる」の行が出て、`gh pr merge` が呼ばれず、`ok   hd-dispatch-discard` が見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/issue/hold.go`
- `internal/dispatcher/hold.go`
- `internal/gate/gate.go`
- `internal/gate/mark.go`
- `internal/ledger/ledger.go`
- `internal/issue/merge.go`
- `internal/issue/settle.go`

## 既に見ている検査

- `tests/andon-issue.sh`（偽の gh で、hold・unhold・印を書けないとき・渡していない commit・マージ済みの PR・鍵の取り方を見る）
- `tests/andon-gate.sh`（止めている課題の gate が GitHub を読まずに wait を返し、担当が消した止めでも待つこと）
- `tests/andon-dispatcher.sh`（止めている課題をマージせず、理由の課題が済んだら通知 `hold` を積むこと）
- `go test ./internal/ledger/`・`go test ./internal/waiting/`

## 落とし穴

- 止めの読み方は、`hold` を消した端末が課題の `agent` に書かれた端末かで決まる。担当の端末を `agent` にしないで消させると、止めが外れたと読まれ `hd-worker` が落ちる。道は担当に `kanban start` で受け持たせる。
- 担当が消した行と Chief of Staff が外した行は、どちらも `log.jsonl` に `"hold":null` で残る。外したかは値でなく、最後に変えた行の `by` で読む。
- 印を書くのは、gate が今の commit を渡しているとき（印が緑か、この回の gate の判定が approve・needs_human）だけ。偽の PR の印を緑にしないと、`andon issue hold` は「まだ渡していないので印は書かない」と返し、`hd-hold` が落ちる。
- 検証の担当は、セルの本物の課題に `andon issue hold`・`unhold`・`andon gate` を打たない。本物の PR の印と、Chief of Staff の止めを書き換える。
- dispatcher は、Chief of Staff の端末の形（`hc get <address> --json` の address と state）を読めなければ、その kanban を回さずに 0 で終わる。回さなかった巡では gate もマージも起きないので、`hd-dispatch-hold` は「この巡はこの kanban を回さない」の行が無いことも確かめる。`hd-dispatch-release` が落ちたら、まずこの行を探す。
- dispatcher がマージするのは、gate を回したその巡の中だけ。approve のまま残った判定は、`merge` を後から `auto` に書き換えても、head が動かなければマージに使われない。`hd-dispatch-discard` が見るのは、止めたあとに判定を捨てることと、マージしないこと。
- 担当の worktree の branch は、PR の branch（偽の `view.json` の `headRefName`）と同じ名前にする。違うと gate は fix を返し、`hd-dispatch-release` が落ちる。
- `andon dispatcher --once` を打つのは、`ANDON_WORKSPACE`・HOME・状態の置き場を一時へ向けた切り離しの中だけ。セルの本物で打たない（本物の dispatcher の見回りと鍵に割り込む）。
- 止めている PR がオーナー待ちの一覧でマージの依頼に出ないこと、理由の課題が済んだときの通知 `hold` は、この道では見ない。前者は `go test ./internal/waiting/`、後者は `tests/andon-dispatcher.sh` が見ている。本物の通知は、`features/dispatcher-news.md` の `dn-delivered` の行に `<id> hold` が出るかで読む。
