<p><picture><source media="(prefers-color-scheme: dark)" srcset="assets/logo/dispatcher-mark-dark.svg"><img src="assets/logo/dispatcher-mark.svg" alt="dispatcher の記号" height="64"></picture></p>

# dispatcher の設計

- 状態: **設計**（2026-11-05 起草）。実装は P3b（通知だけ）・P6a（インナーループ）・P6b（フックと離席時の送り）で入る（[design.md](design.md) §5.2）。P6a のインナーループと P6b のフック・離席時の送りは実装済み（`andon dispatcher`・`andon dispatcher drain`・`andon issue stop-check`・`package/hooks/`。決定は [decisions.md](decisions.md) §13・§15）。①②の効きと③の離席の判定は本物の Claude Code での実測が残る（P7）。§2.2・§2.3 の台帳と端末の状態からの拾い（P3b の通知の元）も実装済み（`internal/dispatcher/pickup.go`）。§2.1 の見回りの最初の `kanban tick` はまだ入っていない。
- この文書が dispatcher の設計の正。前の設計（仮設の工場の通知の係）は記録として向こうに残る（相談 F8 の回答。[decisions.md](decisions.md) §7）。読んだものは末尾の「出典」。
- 語は [design.md](design.md) §0.1。機械が読む形（通知の JSON・控え・フック・端末のラベル）の正は [contract.md](contract.md) と `schema/` で、この文書はそれを**なぜその形か**と**いつ何をするか**で説明する。食い違えば contract.md と `schema/` が正。

---

## 0. 一言で

**dispatcher は、セルに1つ常駐して台帳と端末を周期で回り、拾ったことを Chief of Staff に届け、決まった次の工程を起こすプログラム。判断はしない。**

出典は TPS の水すまし（design.md §1.7）: 決まった経路を決まった周期で回り、各工程に部品を届け、できたものを引き取る。工程を知っていて、通るたびに異常に気づく。dispatcher も同じで、**次に何が起きるべきかが決まっている所だけ**を動かし、決まっていない所は通知か問いにして人と Chief of Staff に渡す。

| する | しない |
|---|---|
| 台帳の履歴（`log.jsonl`）・ready の差・端末の状態を拾う | 拾ったことの意味を判断する（検収・優先・次の課題） |
| 通知を JSON で積み、Chief of Staff に届ける（§3） | 担当の画面の文を通知に入れる |
| レビュアーを起こす・ゲートを回す・マージして片付ける・人がマージした PR を拾って片付ける（P6a。§5・§2.7） | ゲートの決定を覆す・追加のレビューを起こすか決める・問いに答える（自分が出したゲートの問いを、人がマージした事実で閉じるのは除く。§2.7） |
| 台帳の CLI（`kanban`）と command（`andon issue …`・`andon gate`）を打つ | 台帳のファイルを直接書く |
| 自分の控え・ログ・Chief of Staff への送り・`news.replied` のラベルを書く | 人を呼ぶ（灯りをつけるのは問いとレベル C だけ。design.md §1.3） |

---

## 1. なぜ1つの係にまとめるか

前の工場では、担当が終わったときに自分で `hc send` で Chief of Staff に知らせていた。人が Chief of Staff の入力欄に長い文を書いている間にその知らせが届き、書きかけの後ろに貼られて一緒に送られた（2026-11-03）。端末の土台の `--if idle` では守れない——人が打っている間も Chief of Staff の状態は `idle` だから。

そこで形を3つ決めた（[decisions.md](decisions.md) §6。2026-11-03）:

1. **担当は Chief of Staff に打ち込まない。** 報告・レビュー入り・問いは台帳に書くだけ（`andon issue report`・`review`・`ask`）。担当は `--no-notify` で起こし、止まった・終わったは dispatcher が端末の土台から拾う。
2. **Chief of Staff に打ち込むのは dispatcher だけ。** 書き手が1つなら、入力欄が空になるのを待つ守りを1か所に置ける。
3. **台帳から拾う。** 担当が言い忘れても、台帳に書いた時点で失われない。

Andon ではこれに、ターンの切れ目で渡すフック（§3 の経路①②）を足す。打ち込み（経路③）は人が離れているときだけの残りの口になり、書きかけに貼り付く危険は「離席の判定が外れたとき」だけに縮む。

---

## 2. 何を拾うか

### 2.1 見回り

- セルの各プロジェクトの台帳を回る。**待ちが無ければ1分ごと、待ちがあれば5秒ごと**（仮の値。設定で替えられる）。
- プロジェクトの Chief of Staff（`config.yaml` の `chief_of_staff`）の端末が生きていなければ、その台帳は丸ごと飛ばす（ログに1行。理由が変わったときだけ）。Chief of Staff は起きたときに台帳を自分で読むので、留守の間に積む必要が無い。
- 見回りの最初に `kanban tick` を打つ（ルーチンの `schedule` が来たものを課題にする）。台帳の CLI の仕事を、周期を持つ dispatcher が代わりに起こすだけ。
- **初めて見る台帳**（控えが無い）は、通知の元（§2.2・§2.3）について今の ready と `log.jsonl` の位置を控えるだけで送らない（過去の出来事を一度に流さない）。
  ただし `doing` の課題の今の `review_request`（§5）は拾う。commit が PR の head と同じことを確かめてから起こすので、過去の出来事ではなく、いま止まっている工程だから（dispatcher が止まっている間に出たレビュー依頼を取りこぼさない。2026-11-06 の P7）。
  二重に起こさない守りは控えではなく台帳で持つ: その commit のレビュアーの子課題がもうあれば引き継ぐ（§5 の「二度起こさない」）。

### 2.1.1 1 巡の順序（固定）

見回りの1巡は、プロジェクト（`projects/*` を名前順）ごとに、次の順で打つ。**順は約束**で、変えるときはこの表と `internal/dispatcher/dispatcher.go` の `pass` を一緒に直す（前の `one_pass` の固定順を持つ shepherdr〈MIT〉の scheduler と同じ考え。出来事の扱いを順から決められる）。

| # | 段 | 何をする | 読み・書き |
|---|---|---|---|
| 0 | 生きているか | Chief of Staff の端末を `hc get`。無ければこのプロジェクトは**丸ごと飛ばす**（下の段も回らない。§2.1） | 読むだけ |
| 1 | 控えの lock | `drain`（フック）と控えを取り合わないよう、控えの lock を巡の終わりまで持つ | — |
| 2 | 控えを読む | 控え（初めてなら「初めて見る台帳」＝`first`）を読む | 読むだけ |
| 3 | 台帳を1回だけ読む | 課題の一覧と `log.jsonl` の全行を**この1回だけ**読む。以降の段はこの写しを使う | 読むだけ |
| 4 | 通知の元を拾う（`collect`） | 次の a→b→c の順に、届ける通知は `Pending` へ積み、記録だけの通知は記録の置き場に1行足す（§2.2・§2.3・§3.2。4〜5c で出る通知はどれも同じ） | 控えと記録に書く |
| 4a | ready の差 | `kanban ready --json` を前回と比べる（`ready`） | |
| 4b | `log.jsonl` の続き | 前回の位置から（`report`・`review`・`asked`）。Chief of Staff 自身の行は飛ばす。`{"status": "done"}` は誰の行でも控えの `Impact` と `Corrections` に積む（§2.5・§2.6） | |
| 4c | 止まった担当 | `doing` の担当を `hc get`。最初のメッセージの送り直し・提供元に切られたときの「続けて」（§2.4）・`blocked`／`error`／`finished`／`stalled` | 端末に打つことがある |
| 4d | 切られた Chief of Staff | Chief of Staff の端末を `hc get`。提供元に切られて止まっていれば、入力欄が空のときだけ「続けて」（§2.4） | 端末に打つことがある |
| 5 | 課題ごとの工程 | 一覧の順に1課題ずつ。**`doing` で `review_request` があれば**レビュアーを起こす（`review-requested`）、**`review` ならゲートを回し**（`gate`・マージ・`fix`・問い）、**続けて人がマージしたかを見る**（PR ごとに1分に1回だけ `gh pr view`。§2.7）。レビュアー・Chief Engineer・repo の無い課題・`deliverable: candidates` の課題は飛ばす（§5 の P6a） | 台帳・端末に書く |
| 5a | 人のマージの控えを捨てる | `review` にいない課題の控えの `Watch` を捨てる（§2.7） | 控えに書く |
| 5b | マージのあとの影響 | 控えの `Impact` の課題ごとに `andon issue impact` を1回打ち、影響のある課題があれば `impact` を積む（§2.5）。担当には打たない | 控えに書く |
| 5c | 人の手直し | 控えの `Corrections` の課題ごとに `library/corrections.json` を1回だけ書く（§2.6）。もう在れば書かない | 台帳の `library/` と控えに書く |
| 5d | approve のまま止まった | 記録だけにした `gate` の approve（`merge: auto`。控えの `Approved`）のうち、10分たっても `merged` も `asked` も無いものを `Pending` へ1回だけ積む。`review` にいない課題の分は捨てる（§3.2） | 控えに書く |
| 6 | 届ける（経路③） | 初めて見る台帳でなければ、§4 の守りがそろったときだけ `hc send`。送ったら `Pending` を空にする | 端末に打つ |
| 7 | 控えを書く・lock を放す | 控えを保存する。動いている課題（`doing`／`review`）か待ちがあれば busy（次は5秒後。無ければ1分後） | 控えに書く |

`kanban tick`（§2.1）は入ったら**3 と 4 の間**に置く（ルーチンが足した課題を同じ巡の `ready` で拾えるように）。今は入っていない。

同じ巡で起きた出来事の扱い（上の順の帰結）:

- **台帳の写しは3で固定する。** 4・5・6 で dispatcher 自身が書いたもの（ゲートの記録・レビュアーの課題の作成・マージ・`finish`）は、同じ巡の中では写しに出ない。台帳に書いた `log.jsonl` の行は**次の巡**で拾う。
- **報告と止まりが同時でも、どちらも失わない。** 4b（`report`）→4c（`blocked`）の順に積み、種類が違えば置き換えず、1通の `<news>` に積んだ順で並ぶ。優先順位は付けない（どちらを先に見るかは Chief of Staff）。置き換わるのは**同じ課題の同じ種類**だけ（§2.2）。
- **4 で積んだものは、同じ巡の6で届けうる。** 例: `review` に入った（4b の `review`）課題のゲートが5で回れば、`review` と `gate` が同じ巡で1通になる。`approve` から `merged` まで進めば `merged` も同じ通に載る。
- **ゲートが `wait` のとき**は控えに書かず、通知も出さない。次の巡でゲートから回し直す（5）。
- **5 の中**では、1課題につき `doing` の工程か `review` の工程のどちらか1つだけが走る（状態は排他）。別の課題の工程は互いに独立で、一覧の順に走る（ある課題の失敗で後ろを止めない）。
- **初めて見る台帳**（2）は、4 は位置を控えるだけで送らない。5 のうち、いま止まっている `doing` の `review_request` は拾い、`review` のゲートは回さず記録だけ置く。6 は飛ばす（§2.1）。
- **控えの保存は7の1回が基本。** 例外は最初のメッセージの送り直しと「続けて」（4c・4d）で、送る前に「送りかけ」・回数だけ先に保存する（二度送らないため。§2.4・§5）。途中で落ちた巡の積みは残らないが、次の巡が同じ出来事を台帳から拾い直す（拾う元が台帳なので失われない。§1 の3）。

### 2.2 通知の元

`kind` と `detail` の形は [contract.md](contract.md) §6。ここでは拾い方だけ。

| 出来事 | 拾う元 | `kind` |
|---|---|---|
| 報告が書かれた | `log.jsonl` の `{"report": "write" \| "append"}` | `report` |
| レビューに入った | `log.jsonl` の `{"status": "review"}` | `review` |
| 問いが来た | `log.jsonl` の `{"question": n, "status": "open"}`。中身は問いの JSON（contract.md §5） | `asked` |
| 引き取れる課題が増えた | `kanban ready --json` の**前回との差**（人の課題を除いた id の集まりで比べる） | `ready` |
| レビューを頼まれ、起こした | `report.json` の `review_request`（P6a） | `review-requested` |
| ゲートが判定した | dispatcher が回した `andon gate` の結果（P6a） | `gate` |
| マージして片付けた | approve ＋ `merge: auto` ＋ 委譲の範囲内（P6a） | `merged` |
| マージのあと、動いている課題にぶつかる・重なる | `log.jsonl` の `{"status": "done"}`（dispatcher と Chief of Staff の finish）→ `andon issue impact`（§2.5） | `impact` |
| 片付けた課題の人の手直しを集められなかった | `log.jsonl` の `{"status": "done"}` → `corrections.json`（§2.6）が次の見回りでも集められない | `corrections` |
| 担当が止まった・落ちた・終わった・静か | 端末の土台の状態（§2.3） | `blocked`・`error`・`finished`・`stalled` |
| 外の信号（main の CI の失敗・webhook） | **最初の版の後**（design.md §1.9 の (a)・§6 問い23） | `signal` |

- `log.jsonl` は前回の続きから読む。読んだ位置は dispatcher の控えに持つ（相談 F1 の `kanban events --follow` が入れば、その cursor に替える。cursor は不透明な文字列として扱う）。
- 位置には最初の行の hash を添える。台帳の log は追記だけなので、短くなった・最初の行が替わった（手で書き直した）ときは新しいファイルとして先頭から読む。
- **ready は履歴ではなく差で拾う。** ルーチンが足した課題・`queued` から戻った課題・人が足した課題を、同じ1本の道で拾える（出来事ごとに拾い方を増やさない）。
- **Chief of Staff 自身が起こした変化は知らせない**（`by` が `terminal:<chief_of_staff>` の行）。自分のしたことを自分に知らせても答えが増えるだけ。人が手で起こした変化は知らせる。
- 積むとき、**同じ課題の同じ出来事は新しい1件に置き換える**。
- `review` の `detail.pr` は `report.json` の `pr`。PR の無い報告（レビュアー・repo の無い課題）は `report` として知らせる（schema が `pr` を要するため）。
- 拾った通知を Chief of Staff に届けるか、記録だけにするかは §3.2 の表で決まる。この表の `kind` はどれも拾い続け、記録だけのものも消さない。
- 問いの JSON（`library/questions/<n>.json`）は台帳の行のあとに置かれるので、無ければ1回だけ次の見回りまで待つ（その行から読み直す）。それでも無い・読めなければ飛ばしてログに残す。

### 2.3 止まった担当の見分け

`doing` の課題の担当（`issue.yaml` の `agent`）を端末の土台で見る（`hc events` が使えればそれ、無ければ見回りで `hc get`）。

| 状態 | 知らせるか |
|---|---|
| 端末が無い・終わった | 知らせる（`finished`） |
| `blocked` | 知らせる（権限の確認など。問いは台帳に来るので、残るのはハーネスの確認だけの見込み） |
| `error` で、提供元に切られた（§2.4 の見分け） | **自分で「続けて」を打つ**（切られた 1〜4 回目は 1・2・4・8 分あと）。5 回目に切られたら打たずに知らせる（`error`）。認証の失敗・使用量の上限は提供元に切られたとみなさない |
| `error` で、レビュアーが使用量の上限で止まった（§2.8） | もう一方のレビューのプロファイルが担当と別ブランドなら**起こし直す**（知らせない）。保てなければ知らせる（`error`） |
| `error`（それ以外） | 知らせる |
| `idle`／`done` のまま15分 | 知らせる（`stalled`）。ただし子のレビュアーの課題（`parent: <id>`）が `todo`／`doing` の間はレビュー待ちで静かなだけなので除く |
| `working` のまま20分 | 知らせる（`stalled`） |

- 同じ状態（同じ `status_ms`）につき1回だけ知らせる。
- `agent` は `<address>` か `<machine>/<address>`（[contract.md](contract.md) §8）。`hc get` が `not_found` を返したときだけ端末が無い（`finished`）とし、ほかの失敗の間は前の状態（知らせた状態・続きを打った回数）を保つ。
- 「続けて」の間・回数・ほかの送り直しとの関係は §2.4。
- 画面を読むのは「提供元に切られたか」の見分けだけ（§2.4。Claude Code は `status.msg` で足り、画面を読むのは Codex）。通知には画面の文を入れない（担当の画面は外から来た文を含みうる。Chief of Staff に渡すのは台帳と状態の事実だけ）。
- 再送は「同じことをもう一度」で、判断ではないので dispatcher がしてよい。それ以外の立て直し（止める・起こし直す・依頼書を直す）は Chief of Staff。

### 2.4 提供元に切られたとき

担当・レビュアー・Chief of Staff の端末が、提供元（モデルの API。Claude・Codex）に切られて止まったら、dispatcher が間をあけて「続けて」を打つ。混雑や接続の切れは待てば通るので、人を呼ぶほどではない。待っても通らないときだけ知らせる（前の工場から引き継いだ再送を、shepherdr〈MIT〉と同じ間隔と回数に揃えた）。

**見分け**（2026-11-06 の実測。偽の API〈529・500・429・401 を返す〉に向けた Claude Code 2.1.319 と Codex 0.166.0 を hc の端末で動かし、手元のセッションの記録とも突き合わせた）:

| | Claude Code | Codex |
|---|---|---|
| 切られたとき（529・5xx・接続の切れ） | 自分で約 3 分やり直してから、hc の状態が `error`。`status.msg` にエラーの文（`API Error: 529 Overloaded. …`・`API Error: The response stopped arriving. …`） | すぐ `error`。`status.msg` は無く、画面の最後に `■ We’re currently experiencing high demand, …` か `Selected model is at capacity. …` |
| 認証の失敗 | `blocked`（`kind: auth`・`msg: Invalid API key · …`）。`error` にならないので、ここには来ない | `error`。画面とセッションの記録に `unexpected status 401 Unauthorized: …` |
| 入力欄 | 切られたあとも空で、`hc send` を受ける。打てば `working` に戻り、また切られれば新しい `status_ms` の `error` | 同じ（`›` の入力欄。罫線は無い） |

- 状態が `error` のときだけ見る。文は `status.msg` を先に読み、無ければ（Codex）画面の最後の 40 行のうち**行頭が `■` の最後の行だけ**を読む（Codex がターンを止めたときに出す行）。ほかの行（エージェントの出力・ファイルの中身）は、提供元の語を含んでいても見分けに使わない。
- 提供元に切られた、は文が混雑・rate limit・接続・5xx の語に当たり、認証（`401`・`403`・`/login`・`authenticat` など）と使用量の上限（`usage limit`・`quota`・`billing` など。待っても直らない）の語に当たらないとき。それ以外の `error` は今までどおり知らせる（§2.3）。

**間と回数**:

- 切られた 1〜4 回目は、それぞれ **1・2・4・8 分**あとに「続けて」を打つ。測るのは、その `error` を dispatcher が初めて見た時刻と前に打った時刻の遅いほう（打っても同じ `error` のまま `status_ms` が変わらないことがある）。どちらも dispatcher の時計。
- `status_ms` は端末の土台の時計（別のマシンの担当ならそのマシン）なので、時刻としては比べない。値が変われば新しい切断、同じなら同じ切断、の見分けにだけ使う。
- **5 回目に切られたら打たない。** 担当・レビュアーは Chief of Staff に `error` を1回だけ知らせ、Chief of Staff が人に上げる（dispatcher は人を呼ばない。§0）。打った直後の見回りがまだ前の切断の `error` を見ているだけのときは数えない（最後に打ったときと違う `status_ms` の `error` か、打ってから 8 分同じ `error` のままで、5 回目とみる）。
- **打つ前に**、回数と時刻を1つ進めた控えを保存する（打ってから巡の終わりの保存の前に落ちても、再開後に同じ切断へ打ち直さず、上限も超えない）。保存できなければ打たない。送れなかった・届いたか分からない試行も回数と時刻に数える（送れないまま見回りごとに打ち続けない）。回数は `error`・`working` の間だけ持ち越し、ほかの状態（`idle`・`done`・`blocked` など）に戻ったら区切る（担当も Chief of Staff も同じ）。控えは `Retry`（担当は `<issue>|<agent>`、Chief of Staff は `:cos|<address>`）。
- 文は「提供元（モデルの API）に切られて止まった。止まる前の続きを、今の許可の範囲で続けて（dispatcher から。人の依頼ではない）」。人の依頼と取り違えないよう、送り手と範囲を書く。

**Chief of Staff**:

- 人が打つ端末なので、入力欄が空と確かめられたときだけ打つ（§4.2 の読み方。入力欄 → 状態の順に読み直し、同じ `error` のままのときだけ）。確かめられない回（人が打ちかけている・罫線の無い入力欄）は打たず、その回も数える。
- 5 回目に切られたら、知らせる相手が止まっているので通知は積まず、ログに1行残して止める。人は Herdr App で Chief of Staff の `error` に気づく。
- 初めて見る台帳では打たない（担当と同じ）。

**ほかの送り直しと二重に送らない**:

- **最初のメッセージの送り直し**（§5）: 最初のメッセージがまだ届いていない端末（ラベル `first_message.sent=false` で控えの `First` にも無い）には「続けて」を打たない（続ける仕事を受け取っていない）。`error` として知らせる。最初のメッセージの送り直しは `idle` のときだけで、`error` の間は送らないので、同じ見回りで2つが同じ端末に届くことは無い。控えの `First` にある（送ったがラベルを外せなかった）端末は届いたとみて、続きを打つ。
- **`stalled`**: 「続けて」は `error` のときだけ、`stalled` は `idle`・`done`・`working` のときだけで、状態で分かれる。`stalled` は知らせるだけで打たない。打ったあと `working` のまま 20 分たてば `stalled` を知らせる。
- **経路③の `<news>`**（§3）: Chief of Staff に送るのは `idle`・`done` のときだけで、「続けて」は `error` のときだけ。

### 2.5 マージのあとの影響

1本がマージされると、同じ repo で動いている担当（`doing`・`review`）の前提が変わる: main とぶつかる、同じファイルを触っている、依頼書の入力（ADR・設計書）が書き換わった。放っておくと担当は古い前提のまま進み、ぶつかりはゲートの `conflict`（マージの直前）まで分からない。
そこで**機械で分かる事実だけ**を dispatcher が集めて Chief of Staff に渡し、**判断と担当への依頼は Chief of Staff** が持つ（ADR-0006 決めたこと 14 の 機械→文→人 の順。判断の表は chief-of-staff skill §7）。

- **合図はマージではなく `done` の行**。マージは2つの道で起きる（dispatcher の `merge: auto`・人のマージ）が、どちらも `andon issue finish` で `done` になる（人がマージしたときは、dispatcher が見回りで MERGED を見つけて打つ。§2.7。Chief of Staff が手で打っても同じ）。`done` を拾えば1本の道で両方を拾える。Chief of Staff が打った行も拾う（影響は Chief of Staff もまだ見ていない事実なので、「自分のしたことを自分に知らせない」の外）。
- 拾った id は控えの `Impact` に積み、5b で `andon issue impact <id>` を打つ。レビュアー・Chief Engineer・repo の無い課題・`report.json` に `pr` の無い課題は打たない。PR が MERGED でない（マージせずにやめた）ときは command が終了コード 4 を返し、ログに1行で終える。失敗は次の見回りで打ち直し、3回で諦めて、`pr`・`error` だけの `impact` を Chief of Staff に積む（ログだけにしない。待ちの通知は届くまで控えに残るので、手で `andon issue impact` を打つきっかけは失われない）。
- `andon issue impact` は読むだけ（書くのは `git fetch` だけ）。同じ repo の `doing`／`review` の担当の課題ごとに、branch がもうマージの commit を含むか・`git merge-tree` でぶつかるか・両方が触ったファイル（worktree の commit していない変更と未追跡のファイルも。worktree が読めなければ `error`）・依頼書に名前が出るマージのファイル、を出す（形は contract.md §6）。GitHub の `mergeable` はマージの直後には計算中のことが多いので使わず、手元の git で決める。
- 影響のある課題が1つも無ければ知らせない（ログに1行）。あれば `impact` を1件積み、同じ巡の6で届けうる（`merged` と同じ通に載ることもある）。
- **担当に `hc send` しない**。rebase を頼む・止めて向きを確かめるは Chief of Staff の判断（§6 の「判断しない」）。rebase そのものは担当がする（Chief of Staff も dispatcher もしない）。
- 初めて見る台帳では `done` も位置を控えるだけで拾わない（過去のマージを流さない）。`log.jsonl` を先頭から読み直したとき（§2.2）は古い `done` も拾うが、古いマージの commit は動いている branch に含まれる（`contains`）ので、ほとんど知らせにならない。

### 2.6 人の手直し

人がエージェントの仕事を直した跡（design.md §1.9 の (b) の表の 1〜3）を、振り返りの入力として課題の `library/corrections.json` に1回だけ集める。形と拾う規則は [contract.md](contract.md) §14。

- **合図は §2.5 と同じ `done` の行**。拾った id は控えの `Corrections` に積み、5c で集める。`corrections.json` がもう在れば書かない（同じ finish を2度拾っても1回）。レビュアーの課題には書かない。
- 読むのは台帳（`kanban show --json` の `questions`・`library/questions/<n>.json`・`report.json`・予約キー `must_fix`）と、`report.json` に `pr` があれば `gh pr view --json state,commits,comments,reviews`・インラインのレビューコメント（`gh api …/pulls/<n>/comments`）と人の commit ごとの `gh api …/commits/<sha>`。
- 手直しが無ければ空の配列で書く。読めないものがあれば書かず、次の見回りで1回だけ打ち直し、それも駄目なら通知 `corrections`（`error`）を積んで控えから外す（「集めていない」をファイルの無さで残し、Chief of Staff が手で集めるか決める）。
- 判断はしない。拾うのは事実だけで、どれを定義（テスト・`REVIEW.md`・`AGENTS.md`・skill）の直しにするかは振り返り（PR を通る）。

### 2.7 人がマージした PR

`merge: person` やゲートの `needs_human` の課題は、人が GitHub で PR をマージしても台帳は `review` のまま残る。気づくのが Chief of Staff だけだと、ゲートの問い（`from: gate`）は開いたまま、finish も影響（§2.5）も走らず、finish は開いた問いで止まるので Chief of Staff が `kanban answer`・`kanban received` を手で打つことになる。
そこで dispatcher が見回りで PR の状態を見て、**dispatcher が自分でマージしたときと同じ道**（finish → `done` の行 → 次の巡で §2.5・§2.6）に乗せる。マージはしない（拾うのはマージ済みのものだけ）。ゲートの判定も変えない。

- **見るのは `review` にいて `report.json` に `pr` がある、インナーループを通る課題だけ**（§5 の P6a と同じ見分け）。PR の repo が `issue.yaml` の `repo` と違えば見ない（ほかの repo の PR がマージされても、この課題は片付かない）。dispatcher が自分でマージした課題（控えの `Gates[].Merged`）は見ない。
- **GitHub に聞くのは PR ごとに1分に1回**（`gh pr view <n> --repo <owner/repo> --json state,mergeCommit`）。待ちがあると見回りは5秒ごとなので、打った時刻を控えの `Watch` に持つ（gh の上限を食わない）。MERGED・CLOSED と分かったら、もう聞かない。
- **MERGED**: その課題のまだ受け取られていない問いを見る（`kanban questions --json` と `library/questions/<n>.json` の `from`）。
  - ゲートの問い（`from: gate`。重なったものも）は、`open` なら「人がマージした（<PR>）」と `kanban answer` し、`kanban received` して閉じる。そのあと `andon issue finish` し、通知 `merged`（`by: person`）を積む。
  - **ゲートの外の問い**（担当の `ask` など。問いの JSON が無いものも）が開いていれば、**片付けず**に通知 `merged`（`by: person`・`held` に理由）を1回だけ積む。finish の拒みを黙って越えないため。問いが閉じたら次の巡で片付ける（GitHub には聞き直さない）。finish が失敗したときも同じ（`held` に理由。同じ理由では二度知らせない）。
  - ゲートの記録の `merged` は書かない（`merged: true` は dispatcher がマージした印。contract.md §7・§13 の `auto_merge`）。
- **CLOSED**（マージなし）: 片付けない。通知 `closed` を1回だけ積む。やめるかは Chief of Staff と人が決める（chief-of-staff skill §7）。
- 控えの `Watch` は `review` にいなくなった課題（片付いた・`doing` に戻った）から捨てる。`report.json` の `pr` が替われば新しい PR として見直す。

### 2.8 レビュアーが使用量の上限で止まったとき

使用量の上限（§2.4 の見分けの「待っても直らない」側。`usage limit`・`quota`・`billing` など）は待っても通らないので「続けて」は打たない。止まったのが**レビュアー**（`parent` があり `profile` が `*-review` の `doing` の課題）なら、もう一方のレビューのプロファイルで起こし直せるかを見る（[decisions/2026-11-09-reviewer-allowance-fallback.md](decisions/2026-11-09-reviewer-allowance-fallback.md)）。

- 起こし直すのは、もう一方（`codex-review` ↔ `claude-review`）が元の課題の担当の `profile` と**別ブランド**のときだけ。担当が Claude で `codex-review` が尽きたなら `claude-review` へは逃げない（同じブランドのレビューはゲートが数えない）。今どおり `error` を知らせる。
- 手順: 止まった端末を `hc kill`・`hc rm`（同じ名前 `<slug>-<子>` で起こすため）→ `kanban set <子> profile=<先> fallback="<元> → <先>（使用量の上限で止まった。端末 <元の端末>）"`（`by` は `terminal:dispatcher`。逃げたことは `log.jsonl` のこの1行に残る）→ §5 のレビュアーと同じ `hc run`（同じ cwd・同じ最初のメッセージ・同じ detached worktree）→ `kanban set <子> agent=<新しい端末>`。最初のメッセージが届かなければ §5 と同じくラベル `first_message.sent=false` を付けて見回りに預ける。
- 起こし直しは1回だけ: 替えた先も上限で止まれば、その先のもう一方は担当と同じブランドになるので、知らせる。
- 途中で失敗したら起こし直さず、ログに1行。端末を片付けたあとの失敗は、次の見回りで端末が無い（`finished`）として知らせる。
- 担当・Chief of Staff の使用量の上限は今どおり知らせる（人が枠を足すか待つかを決める）。初めて見る台帳では起こし直さない。

---

## 3. 誰に・いつ・どう届けるか

宛先は**プロジェクトの Chief of Staff だけ**。人には届けない（人はボードで台帳と端末を見る）。担当に届けるのは P6a の `fix` だけ（§5）。
届けるのは §3.2 の表で「届ける」の通知だけ。「記録だけ」の通知は経路①②③のどれにも載らず、③の `hc send` のきっかけにもならない（待ちに数えない）。

通知は dispatcher の中で JSON のまま積み（contract.md §6）、**届けるときだけ** `<news>` の文に描く。経路は3つで、どれも同じ JSON から同じ文を描く。

| # | 経路 | いつ | 打つもの | 書きかけに貼り付くか |
|---|---|---|---|---|
| ① | Chief of Staff の `Stop` フック | Chief of Staff のターンが終わるとき | `andon dispatcher drain`（slug は端末のラベルから）→ 待ちがあれば `{"decision": "block", "reason": "<news>…</news>"}` で続きのターンにする | 貼り付かない（入力欄を通らない） |
| ② | Chief of Staff の `UserPromptSubmit` フック | 人が送ったとき | 同じ（`--context`）＋ 委譲の一覧。`additionalContext` に添える | 貼り付かない（人の文の横に文脈として入る） |
| ③ | 離席時だけ `hc send` | ①②が来ない間（Chief of Staff がターンを終えて入力を待っていて、人も打っていない） | §4 の守りがそろったら `hc send --if idle` | 判定が外れたときだけ（§4.4） |

- **①②が主、③が残り。** ターンの中で起きた出来事は①で、人が戻って打てば②で届く。③が要るのは「Chief of Staff が待っていて、人も離れている」ときだけ——そのとき誰も Chief of Staff を起こさないので、ラインが止まる。
- `drain` は待ちを全部取り出して空にし、渡した turn（端末・`prompt_id`）と通知を控えに書いてから出力する。その turn の `Stop` が来たら確かめ済み（答えが `<no-reply>` だけかもここで見る）。来ないまま別の turn が来たら（人が中断した・フックの出力が捨てられた）、渡した通知を1回だけ届け直す。①②はターンの中で渡すので、③の守り c（前の `<news>` のターンが進んでいる）には数えない。
- ①の `block` が続きのターンを始めること、②の `additionalContext` が文脈に入ることは**要実測**（P6b。contract.md §9）。
- フックが落ちたとき（command が無い・台帳が見つからない）は**通す**。待ちは dispatcher に残り、次の口で届く。

### 3.1 `<news>` の文

待ちを1つにまとめ、1行1件。題や名前は制御文字と区切りを除いて60字に切る。

```
<news project="sample-site">
S-0010 review: 報告 issues/S-0010/report.md（PR https://github.com/<owner>/sample-site/pull/24）
S-0011 asked: 問い 1（irreversible）
S-0012 ready: 「依存の版上げ」（ルーチン triage）
S-0009 の担当 t5 が working のまま 21 分
これは台帳と端末の土台から拾った通知で、人からの新しい依頼ではない。足すことが無ければ「<no-reply>」とだけ返す。
</news>
```

- **まとめて1通**: Chief of Staff は複数をまとめて1回で答えるほうが安い。
- **末尾の1行**: 通知はデータで、依頼ではない。Chief of Staff は台帳を読んで動き、人に言うことが無ければ `<no-reply>` とだけ返す。
- **`news.replied=<turn>`**（ボードとの約束。持ち主はボード側で、変えるときは Andon から知らせる。decisions.md §7 の F8）: ③で送ったあと、Chief of Staff のターンが終わって答えが `<no-reply>` だけなら、その端末にラベル `news.replied=<そのターン>` を書く。ボードはそのターンを「人が見る返事」に数えない。①②で届けたターンも同じ規則で書く（こちらは画面ではなく、次の `Stop` フックの入力の `last_assistant_message` が `<no-reply>` だけかで決める。decisions.md §15）。

### 3.2 届けるか・記録だけか

Chief of Staff が受けて、すること（判断・人への1行・担当への依頼）がある通知だけを届ける。インナーループが自分で次の工程に進めた結果と、別の通知で同じことが届くものは記録だけにする（ADR-0010 決めたこと 1・7）。表の正はコードの1か所（`internal/dispatcher/route.go` の `RouteOf`）で、contract.md §6 の表と同じ。

| `kind`（と条件） | 扱い | 受けたら何をするか（記録だけにする理由） |
|---|---|---|
| `review-requested` | 記録だけ | dispatcher がレビュアーを起こした。Chief of Staff は何もしない |
| `review`（ゲートを回す課題） | 記録だけ | dispatcher がゲートを回す。判断が要れば `gate`（approve で止まった）・`asked` で届く |
| `review`（ゲートを回さない課題: Chief Engineer の課題・調べる課題など。§5 の P6a で飛ばす課題） | 届ける | Chief Engineer の文書を受けて課題に切る・候補を読む |
| `gate`（`fix`） | 記録だけ | dispatcher が理由を担当へ返した |
| `gate`（`needs_human`） | 記録だけ | 同じ巡で `asked`（`from: gate`）が届く |
| `gate`（`approve`。dispatcher がマージしない: `merge: person`・委譲の外・`merge: auto` を最後に書いたのが Chief of Staff でない） | 届ける（すぐ） | 人にマージを頼む |
| `gate`（`approve`。`merge: auto` でマージに進む） | 記録だけ。10分たっても同じ課題の `merged` も `asked` も無ければ1回だけ届ける（§2.1.1 の 5d） | ふつうは同じ巡で `merged` まで進む。進まないときだけ Chief of Staff が確かめる |
| `merged`（`held` が無い） | 記録だけ | dispatcher（か人）がマージして片付けた。影響は `impact` で別に届く |
| `merged`（`held` あり） | 届ける | 片付けていない理由を読んで、問いを閉じる・`andon issue finish` する |
| `report`（レビュアーの課題） | 記録だけ | 結果の宛先は元の課題の担当で、Chief of Staff は中継しない |
| `report`（ほか）・`asked`・`ready`・`impact`・`closed`・`corrections`・`blocked`・`error`・`finished`・`stalled` | 届ける | 報告を読む・問いを間引く・起こす・rebase を頼む・人へ知らせる・続きを促す |

- **記録だけの置き場**は控えの横の `~/.local/state/andon/dispatcher/<slug>.recorded.jsonl`（`XDG_STATE_HOME` があればその下）。1行1件・通知の JSON そのまま（contract.md §1・§6）。書き手は dispatcher だけで、追記だけ。読み手はボード。
- **10分**は dispatcher の時計で、記録した approve の `at` から測る（見回りが1分ごとなので、merge queue の repo でも10巡は待つ）。仮の値で、ほかの間（§2.1）と同じくコードの定数（`approveWait`）。回し直したゲート（§2.1.1 の 5 の読み直しで次の巡に持ち越した approve）では測り直さない。ゲートが別の答えを出した・`review` から出た課題の分は捨てる。
- 課題の見分けは今ある項目だけ（`gated`・`isReviewer`。新しいキーは足さない）。

---

## 4. 離席の判定（経路③の守り）

### 4.1 守り（5秒ごとに確かめ、そろわなければ送らず次の回に持ち越す）

- **a.** Chief of Staff の端末の状態が `idle` か `done`。
- **b.** **入力欄が空**（§4.2）。
- **c.** 前の `<news>` を送ってから、Chief of Staff のターンが進んでいる（まだ答えている途中なら待つ）。
- **d.** 送る直前に **状態 → 入力欄 → 状態とターン** の順で読み直す（入力欄を読む間に人が送ってターンが始まると、入力欄は空に見える。読んだあとにもターンが変わっていないかを見る）。`idle` なら `hc send --if idle`。`done` は `--if idle` に拒まれるので、確かめた直後に `--if` なしで送る。どちらも `hc send` 自身が人の手が止まるのを待つ（二重の守り）。

**迷ったら送らない。** 画面が取れない・入力欄が読めない・形が分からないときは送らず、次の回に持ち越す（contract.md §0 の原則2）。送らなかった通知は失われない（①②でも届く）。

### 4.2 入力欄の読み方

Claude Code の画面は、最後の2本の罫線（`─` が20字以上並ぶ行）の間に入力欄がある。

```
────────────────────────────────
❯
────────────────────────────────
  <モデル> │ <文脈> │ <場所>
```

- 2本の罫線の間が `❯` だけの1行なら**空**（`❯` の後ろの NBSP は空白として扱う）。
- `❯ ` の後ろに字が残れば**書きかけ**、間が2行以上なら**書きかけ**。
- 罫線が2本無い・間が `❯` で始まらない（選択画面など）なら**分からない**。送らない。

### 4.3 薄い字（ANSI の dim）で学んだこと

文字だけでは、人の書きかけと Claude Code の**薄い字**（新しいセッションのヒント `❯ Try "…"`、ターンの終わりに出る入力の候補）を分けられない。前の工場では最初、薄い字を「書きかけ」とみなして送らなかったので、候補が出たままの間は通知が届かず、約40分・11件が溜まった（2026-11-04）。そこで画面を色ごと読んで、薄い字を除いてから入力欄を見る形にした。その過程で学んだこと:

1. **文字の正は `hc read`、色は別の口から。** `hc read` は文字だけで色や属性を返さない。色は `hc attach --observe`（打たない・大きさも変えない）の ANSI の列から組み直す。
2. **組み直した画面を文字の正にしない。** attach の出力が静かになっても描き終えたとは限らない（再描画が間を置いて分かれて届く）。組み直した入力欄（薄い字も残したもの）が `hc read` の入力欄と**一字も違わないときだけ**、薄い字を除いて決める。違えば送らない。
3. **解釈する制御列は許可の一覧で持つ。** 一覧は、端末の土台が待機中の Claude Code の画面を描き直すときに実際に出す列（本物の端末を observe で数えた）。一覧に無い列が1つでもあれば、組み直した属性を信じられないので送らない。「知らない列は読み飛ばす」形は、`CSI ! p`（DECSTR。SGR を通常に戻す）を読まずに dim のまま本文を消す、という取り違えを生んだ。
4. **属性を控えて戻す列に気をつける。** `ESC 7`／`ESC 8`・`CSI s`／`CSI u` は位置と一緒に属性も控えて戻す。端末の土台の再描画には出ないので一覧から外した（出れば送らない）。同じ控えを使う `?1049h`／`?1049l` は dim を控えて戻す。DECSTR は控えも通常に戻す。**dim が分からない間に空白でない字を描く画面は送らない**（空白は薄くても空白なので結果を変えない）。
5. **SGR の色は組ごと1つの属性として読む。** `38:2::r:g:b` のような colon 形式の色の中の `2` を dim と取り違えない。
6. **ヒントの形を特別扱いしない。** `❯ Try "` を文字で覚える形は、候補の文が変わるたびに外れる。薄い字なら同じ仕組みで空になる。

Andon ではこれを Go で書き直した（`internal/dispatcher/screen.go`。design.md §2.7 の (1)。画面を正規表現で読む形は端が尽きない）。列の許可の一覧と再現の画面は表のテスト（`screen_test.go`）にした。

### 4.4 残る危険（ゼロにはできない）

端末の土台には入力欄の予約も「入力欄が空なら打つ」の条件も無い（`hc send` の条件は状態・強制・待ちだけ）。最後の確かめから `hc send` が打つまでの短い間に人が打ち始める・送ると、通知はその文に続けて送られうる。読み直し（d）は窓を短くするだけ。塞ぐには端末の土台の側の仕組みが要る。Andon が①②を主にするのは、この窓を通る送りを減らすため。

- **Codex** の入力欄の形は未確認。Chief of Staff は最初の版では Claude Code だけ（contract.md §9）。

---

## 5. 手ごとの役（P3b → P6a → P6b）

dispatcher は一度に作らず、ラインの手に合わせて育てる（design.md §5.2）。

### P3b: 通知だけ

前の工場の通知の係を Go の `andon dispatcher` に書き直す。§2 の拾い方と §4 の経路③だけ（フックはまだ無い）。端末の名前とラベルは `role=dispatcher`。控えは `~/.local/state/andon/dispatcher/<slug>.json`（contract.md §6）。

**端末を起こす・中身を戻す**: 端末はセルに1つで、名前は `dispatcher`。`hc run --keep` で起こすので、core が起き直すと（`hc server restart`・Herdr App の Add Machine や Update HC で serve が替わる）名前とラベルはそのままで、中身がシェルに替わる（hc は元の command を打ち直さない）。
そこで「端末がある」ではなく「前に出ているもの（`hc get` の `foreground.argv`）が `andon dispatcher`」で動いていると見る。無ければ起こす。シェルなら、そのシェルには打ち込まず（入力欄に人の打ちかけがあれば、それに足して実行してしまう）、dispatcher 専用の端末なので `hc keep --off`・`hc kill`・`hc rm` で片付けて、同じ名前・ラベルで起こし直す（名前で引くと片付けた端末に当たりうるので address で打つ）。前に何も出ていない（替わる途中）・シェルでも `andon dispatcher` でもないもの（人が何か動かしている）が出ているときは触らない。
確かめるのは boot（毎起動）と Chief of Staff のフック（`andon dispatcher drain`。毎ターン）。core の起き直しは boot を通らないが、dispatcher が回るのは Chief of Staff が生きている台帳だけ（§2.1）で、Chief of Staff が動けば必ずフックが走るので、これで足りる。
作り直し（`bin/bootstrap down` のあと作り直す）のあとは、herdr-core が前の端末を名前とラベルのまま中身を素のシェルにして戻し、戻った直後は前に出ているものがまだ取れないことがある。boot は取れるまで数秒読み直してから同じ見分けをし、取れないままなら「確かめられなかった」とログに書く（「すでに動いている」とは書かない。フックは待たずに次のターンに任せる）。Chief of Staff・Chief Engineer の戻った端末（top-level の argv ごと素のシェル。起こした端末は template の command のまま）はそれぞれの command の打ち直しで、担当・レビュアーの戻った端末は見回りが終わった端末（`finished`）として知らせて戻す（2026-11-06 の P7）。
dispatcher 自身の見張りでは戻せない（止まっているのは dispatcher）。hc にも元の command を打ち直す機能は無い。複数のフックと boot が重なっても二重に打たないよう、確かめは lock の中で1つずつ（2026-11-06 の P7）。

### P6a: インナーループを回す

Chief of Staff が1ターンも使わずに、レビュー依頼から片付けまでが流れる。どの段も**決まった次の工程**で、判断はゲートと Chief of Staff と人が持つ。

| 拾うもの | dispatcher がすること | 判断を持つ者 |
|---|---|---|
| `report.json` の `review_request`（`round` ≤ `review_rounds`） | レビュアーを起こす: `kanban add --parent <id> --round <round>`（子の id は `<id>-r<round>`。通し番号を使わず、detached worktree の名前 `<slug>-<id>-r<round>` と揃う） ＋ 担当の commit の detached worktree ＋ `hc run <reviewer>`（`--no-notify`）。通知 `review-requested` | 起こすかは `issue.yaml` の `review`・`reviewer`（Chief of Staff が書く） |
| `review_request.extra`（上限を超えた回） | **起こさない**。通知にして Chief of Staff へ | Chief of Staff |
| `{"status": "review"}` | `andon gate` を回す。記録 `library/gate/<sha>.json`。通知 `gate` | ゲート（決定式） |
| `deliverable: candidates`（調べる課題）の `{"status": "review"}`・`review_request` | **ゲートを回さず、レビュアーも起こさない**。通知だけ（PR が無いので `report`）。候補の一覧は Chief of Staff が読んで切る。`deliverable` を最後に書いたのが Chief of Staff でなければ `pr` として読む（[ADR-0006](adr/0006-survey-fix-decide.md) 決めたこと 7） | Chief of Staff |
| ゲートが `wait`（PR の head の GitHub の checks が走っている・まだ1つも無い。contract.md §7） | **待つ**: 控えに書かず、通知も問いも出さない。次の見回りでゲートから回し直す（checks が揃えば判定に進む。1つも無いまま `github_checks_wait_minutes` を過ぎたらゲートが `needs_human` を返す） | ゲート |
| ゲートが `approve` ＋ `merge: auto` ＋ 委譲の `merge` がそのレベルを許す | マージ（条件表の `repos.<repo>.merge` の方式）→ `andon issue finish`（`library/corrections.json` に人の手直しを集める。design.md §1.9 の (b)）。通知 `merged` | ゲート・委譲（人） |
| ゲートが `approve` で `merge: person`、または委譲の外 | マージしない。通知 `gate` のまま止まる | 人 |
| `review` の課題の PR を人がマージした（1分に1回だけ `gh pr view`。§2.7） | ゲートの問いを「人がマージした」と閉じて `andon issue finish`。通知 `merged`（`by: person`）。ゲートの外の問いが開いていれば片付けずに通知 `merged`（`held`）。マージせずに閉じた PR は通知 `closed` を1回 | 人（マージ） |
| ゲートが `fix` | 理由を担当へ `hc send`（担当は人が打つ端末ではないので、離席の守りは要らない） | ゲート |
| ゲートが `needs_human` | 問いを出す（contract.md §5。`from: gate`）。通知 `asked` | 人 |

- **二度起こさない・二度回さない**: 起こしたレビュアー（`issue` → `reviewer_issue`・`commit`）と回したゲート（`issue` → `sha`。`review` に入った `log.jsonl` の行ごと）を控えに持ち、同じ組では打たない。
  控えに無い組（初めて見た台帳・控えを無くした）は台帳を見る: parent がその課題のレビュアーで、依頼書にその commit がある課題があれば、`todo` でなければもう受けているとして起こさない。`todo` でも端末が起きていることがある（`hc run` は通ったが `kanban start` の前に落ちた）ので、ラベル `project=<slug>`・`issue=<子>` の live な端末を引く（課題の id はプロジェクトをまたいで同じことがあるので、返った端末のラベルも両方で確かめる）。何か受けていれば（`turn` が 1 以上）起こさず、まだなら address を引き継いで最初のメッセージから続ける。端末が無いときだけ起こすところから続ける。
- **同じ commit の出し直し**: 起こさなかった回（設定・上限・head 違いなどで止めた回）は、担当が `andon issue review-request` で頼み直せば（その課題の報告の行が増えれば）拾い直す。起こした回は、同じ commit で頼み直しても二重に起こさない。
- **台帳に書くときの `by` は `terminal:dispatcher`**（`HC_TERMINAL_ID=dispatcher` で打つ）。ゲートと `andon issue ready` は、作成の行の `by` が Chief of Staff か dispatcher のレビュアーを数える。
- **レビュアーの依頼書に結果の宛先を書く**: 元の課題（`parent`）の担当（`issue.yaml` の `agent`）だけ。Chief of Staff・dispatcher へは送らせない（Chief of Staff へは dispatcher が通知で届ける）。
- **上限を超えた回**は今の `review_rounds` で決める。Chief of Staff が `review_rounds` を上げれば、次の見回りで起こす。
- **マージの直前に読み直す**: 課題の予約キー・履歴・委譲、PR の head と GitHub の checks（ゲートと同じ読み方。contract.md §7）がゲートのときと違えば（push された・checks が走り直した・落ちた）打たず、次の見回りでゲートから回し直す。残る隙間: マージの直前の読み直しと `gh pr merge` の間に、同じ head のまま checks が走り直した・落ちたときは止められない（`--match-head-commit` が固定するのは head だけ）。サーバーの側で checks を強制できるのはブランチ保護（ruleset）だけで、private の repo の Free のプランでは使えないので、この隙間は受け入れる。public にしたら ruleset で既定ブランチに required checks を付けて塞ぐ（public にする手順に入れてある）。
- **起こしそこねたら続きから**: 一時の失敗（fetch・`hc run`・信頼の画面）は控えに分けて持ち、次の見回りで済んだ段の続きからやり直す（二重に起こさない）。レビュアーの端末が起きたのに最初のメッセージが届いていなければ、担当と同じくラベル `first_message.sent=false` を付けて起こしたことにし（通知 `review-requested`）、送り直しは次の項目の見回りに預ける。ラベルを付けられない・`kanban start` が通らなかった（見回りは `doing` しか見ない）ときだけ、起こしたことにせず次の見回りで同じ端末へ `hc send` する。
- **担当の最初のメッセージも送り直す**: `andon issue start`・`andon chief-engineer` が最初のメッセージを届けられなかった（終了コード 3）端末には、command がラベル `first_message.sent=false` を付けて預ける（dispatcher が起こしたレビュアーも同じ。上の項目）。届かなかった、は `hc run` の `first_message` が `sent: false`（Codex 0.166.0 は入力欄の背景が動き続けて出力が落ち着かず、hc が送らずに返ることが多い）か、`sent: true` でも `started: false`（送ったが数秒で `working` にならない。起動の途中で入力を捨てられたとみる）のとき。`started` が無い（状態を報告しない program）は届いたとみる。dispatcher は見回り（§2.3 の `hc get`）でこのラベルの `doing` の担当を拾い、`idle`（か状態がまだ無い。入力欄が空のまま）なら同じ最初のメッセージ（レビュアーにはレビュアーの文）を `hc send` し、ラベルを外してログに1行。`blocked`・`error` の間は送らず次の見回りへ（`hc send` も画面が入力を受けられなければ拒む）。`working`・`done` なら人が手で送ったとみなし、送らずにラベルだけ外す。送る前に控え（`first`）へ「送りかけ」を書いて保存し、送れなければその場で消して保存する。送ったあとラベルを外す前に落ちても、外しそこねても、控えにあれば二度と送らない（送っている最中に落ちたときは送らない側に倒れ、担当は `idle` のまま `stalled` で Chief of Staff に届く）。初めて見る台帳では送らない。
- **レビュアーの台帳の作業場所は detached worktree**: レビュアーの `kanban start` は、そのレビュアーの detached worktree（`<slug>-<id>-r<回>`）を cwd にして打つ。台帳の `worktree` はその worktree、`branch` は書かれない（detached）、`repo` は担当の課題と同じになる。台帳の `issues/<子>/` で打つと、台帳がセルの git の中にあるとき、セルの本体の checkout と branch が作業場所として書かれる（P7 v0.0.2 の 21）。端末の cwd（`hc run --cwd`）は今までどおり `issues/<子>/`。
- **片付け**は `andon issue finish <id>` の1つ（レビュアーの子課題と detached worktree も片付く。子課題の `worktree` が `<slug>-<親>-r<回>` の detached worktree で `branch` が無ければ、その worktree を消す）。人がマージしたときは dispatcher が見回りで拾って同じ command を打つ（§2.7）。finish は、台帳の `worktree` が repo の本体の checkout（`git worktree list` の先頭）なら、何も消さずに止まる。
- **安全側**: `issue.yaml` の値が読めない・履歴の照合が合わない（contract.md §2）・委譲が読めないときは、起こさない・マージしない。ゲートは `needs_human` を返し、dispatcher はそれを問いにするだけ。
- 不可逆な操作（デプロイ・公開・削除）は、command が委譲を読んでから打つ（design.md §1.10）。範囲外なら command が止まって理由を1行出し、dispatcher はそれを通知にする。

### P6b: フックで届ける

§3 の経路①②を入れ、③を「離席時だけ」に絞る。担当の `Stop` フック（`andon issue stop-check`。報告か問いを書くまで止まれない。問いの答え・レビュアーの結果を待つターンは止めない）も同じ手で入る。テストは `tests/andon-dispatcher.sh`（turn 中の通知が `Stop` で届く・送信時に添えられる・離席時だけ `hc send`・`doing` で報告無しの `Stop` が block。問いが開いている・レビュー待ちの `Stop` は通す）。本物の Claude Code で1日、実測する。

---

## 6. やらないこと

- **判断しない。** 検収・優先・次の課題・追加のレビュー・問いの答え・ゲートの覆しはしない（自分が出したゲートの問いを、人がマージした事実で閉じるのは答えではなく記録。§2.7）。dispatcher が動かすのは「次が決まっている」所だけ。
- **人を呼ばない。** 灯りをつけるのは問いとレベル C（design.md §1.3）。dispatcher は問いを台帳に出すまで。
- **台帳のファイルを直接書かない。** 書くのは台帳の CLI と command 経由だけ。控えは台帳の外に置く。
- **頼まれていないエージェントを起こさない。** 起こすのは `review_request` のレビュアーだけ（担当を起こすのは Chief of Staff の `andon issue start`）。
- **担当の画面の文を運ばない。** 見るのは状態と、提供元のエラーかの見分けだけ。
- **端末の土台を変えない。** 入力欄の予約などが要れば、持ち主に相談する（§4.4）。
- **外の信号（`signal`）は最初の版に入れない**（design.md §6 問い23）。

---

## 7. 「Nothing runs twice」の表: どの処理を、どの機の誰が、1 回だけ走らせるか

別のマシンに作業側を出す（[design.md](design.md) §1.5）前に、常駐・定期の処理ごとに「走らせる者」と「二重に走ったら」と「今の守り」を固定する。考え方は shepherdr〈MIT〉の README の "Nothing runs twice"（定期の処理は 1 つの係が 1 本の道で走らせ、ほかは走らせない）と同じ。守りを足すのはこの表の外（課題にする）で、ここは**今ある守りと無い所を事実で書く**。

### 7.1 前提

- **指揮側は 1 つのマシン**（台帳・dispatcher・Chief of Staff・ゲート・控え）。作業側（担当・レビュアー）はそのマシンでも別のマシンでもよい（design.md §1.5）。この表の「誰」は、指揮側のマシンの中の誰か。
- **走らせる者は 1 者**: 定期の処理は dispatcher だけが走らせる。Chief of Staff・人が手で打ってよい command（`andon gate`・`andon issue finish` など）は、dispatcher が打つものと**同じ command**なので、手で打つと二重に走りうる（下の「二重に走ったら」）。
- **守りの種類**: ① OS の lock（`flock`。プロセスが死ねば OS が放す。ネットワークのファイルシステムや別マシンをまたいでは効かない）／② 控え（`~/.local/state/andon/dispatcher/<slug>.json`。同じ組を 2 度打たない）／③ 端末のラベル（`hc` の `role=`・`project=`・`issue=`・`first_message.sent`。端末が生きている間だけ）／④ 台帳の状態（`status`・`worktree`・`branch`・履歴）／⑤ GitHub 側（`--match-head-commit`・commit status）／⑥ 冪等（何度打っても同じ結果）。

### 7.2 表

| # | 処理 | 走らせる者（指揮側のマシンで 1 つ） | 二重に走ったら | 今の守り |
|---|---|---|---|---|
| 1 | dispatcher 自身の起動 | `andon boot` の `dispatcher` の段と、Chief of Staff の Stop フック（`drain` の `ensure`）。**どちらも `Ensure`** | 2 つの `andon dispatcher` が同じ台帳を回す（同じレビュアーを起こす・同じ PR をマージしにいく） | ①`.terminal.lock`（取れなければ `ErrBusy` ですぐ諦める）＋③端末の名前 `dispatcher`（`Ensure` は名前で引き、live なら起こさない）。前の版の係の端末が残っていれば起こさない（boot の `dropStaleRoles`）。**手で `andon dispatcher` を別のシェルで打つのは防がない**（7.3 の G1。ただし下の #2 の lock が巡の中身を直列にする） |
| 2 | dispatcher の 1 巡（§2.1.1 の 0〜7） | dispatcher。プロジェクトごとに直列 | 同じ出来事を二度拾う・二度通知する・二度起こす | ①控えの lock（`<slug>.json.lock`）を巡の終わりまで持つ（待たずに諦める。取れなければその巡のそのプロジェクトは飛ばしてログに 1 行）。`drain`（フック）も同じ lock を取る（10 秒待って、取れなければ通して次の口へ）。②控えの位置（`log_pos`・`ready`・`Sent`・`Retry`・`First`） |
| 3 | レビュアーを起こす（P6a） | dispatcher（`review_request` を拾ったとき） | 同じ commit のレビュアーが 2 人いる（指摘が二重・端末と費用が二重） | ②控えの `Reviewers`（`<issue>@<commit>`）＋④台帳の子課題（控えが無いときは parent・依頼書の commit で引き継ぐ）＋③ラベル `project=`・`issue=<子>` の live な端末（`hc run` は通ったが `kanban start` の前に落ちた分）。起こしそこねは続きから |
| 4 | ゲートを回す | dispatcher（`review` に入った `log.jsonl` の行ごとに 1 回）。Chief of Staff が `andon gate` を手で打つこともある | 記録 `library/gate/<head>.json` が上書きされる・PR に commit status とコメントが二重に付く。判定は同じ事実から決まるので**同じ決定**（事実が間に変わっていれば後の方が新しい） | ②控えの `Gates`（`<issue>#<行>`）。**手で打つ分は防がない**（G2） |
| 5 | マージして片付ける | dispatcher（approve ＋ `merge: auto` ＋ 委譲の範囲内）。`merge: person` は人 | 2 度目の `gh pr merge` が失敗する→問い（`irreversible`）が出る。人が先にマージしたときも同じ。head が替わっていれば止まる | ⑤`--match-head-commit`（判定した head だけ）＋マージ直前の読み直し（`stillAuthorized`・`ChecksStill`）＋②控えの `Gates[].Merged`。`finish` は④台帳の `worktree`・`branch` が空なら何も消さない（本体の checkout は消さない）。**問いが出るのは誤報になる**（G3） |
| 6 | 担当の最初のメッセージの送り直し・提供元に切られたときの「続けて」（担当・レビュアー・Chief of Staff。§2.4・P6a） | dispatcher | 同じメッセージを 2 度送る・「続けて」を 5 回以上打つ・最初のメッセージの前に「続けて」を打つ | ③ラベル `first_message.sent=false`（送ったら外す）＋②控えの `First`（送る前に「送りかけ」を保存。外しそこねても二度と送らない）・`Retry`（回数。4 回まで。打つ前に保存する。最初のメッセージが届いていない端末には打たない）。送っている最中に落ちたら**送らない側に倒れる**（`stalled` で気づく） |
| 7 | 通知を Chief of Staff へ届ける（§3） | dispatcher（経路③の `hc send`）と、Chief of Staff 自身のフック（経路①②の `drain`） | 同じ通知が 2 度届く（`<news>` が二重） | ①控えの lock（#2 と同じ）＋②控えの `Pending`（届けたら空にする）・`Hook`・`NewsTurn`（同じ turn に 2 度送らない）。`<no-reply>` で `news.replied` を書く。迷ったら送らない |
| 8 | `kanban tick`（routine の `schedule`） | **今は誰も走らせていない**（§2.1 の「入っていない」）。入れるときは dispatcher が 1 巡の 3 と 4 の間（§2.1.1）で打つ | schedule の時刻に 2 度課題が足される | 台帳の側にある: ①台帳の lock（`kanban/.lock`。書き手は 1 つ）の中で `state/routines.json` の「最後に撃った時刻」を読んで書く＝同じ時刻は 1 度だけ撃つ。逃した時刻は 1 度にまとめる。`open`（未完了の数）で積み上げを止める。**tick を誰が走らせるか（dispatcher だけ）を入れるまで、人が cron で打てば二重に走る**（G4） |
| 9 | 台帳への書き込み（`kanban`） | 打つ者は誰でも（Chief of Staff・担当・dispatcher・人）。ファイルは台帳の CLI だけが書く | 同時の書き込みで id の重複・行の混ざり | ①`kanban/.lock`（書き手は 1 つ。待つのは 10 秒）。書き込みは一時ファイル＋rename |
| 10 | 担当・Chief of Staff・Chief Engineer を起こす（`andon issue start`・`chief-of-staff`・`chief-engineer`） | Chief of Staff（人が `chief-of-staff` を打つ） | 同じ課題の worktree と branch が二重に作られる・Chief of Staff が 2 人になる | ④台帳（`todo` だけ起こせる・`worktree`・`branch` が空でなければ起こし済みで断る）。1 プロジェクト 1 人は③ラベル `role=chief-of-staff`・`project=` の live な端末を数えて 2 人以上なら止まる。起こせなかったら worktree と branch を戻す。`andon issue start`・`resume`・`finish` は台帳ごとの issue lock（`<slug>.issue.lock`。控えの lock とは別）を命令の間持つ。**`andon chief-of-staff`・`andon chief-engineer` の「見る→起こす」の間は塞がない**（G5） |
| 11 | 担当の Stop フック（`andon issue stop-check`） | 担当・レビュアー自身（その端末のフック） | 止める回数が数え間違う | 担当の課題ごとの状態（`Blocks`。3 回で通す）。端末ごとなので他と取り合わない |
| 12 | 信頼の書き込み（boot の `Trust`・`andon issue start` の直前の `TrustFromOS`） | boot と、起こす command（呼び手が書く） | 動いている Claude Code と `.claude.json` を取り合って、後から書いた方が負ける | ⑥足すものが無ければ書かない（2 回目は何も書かない）＋書く直前の読み直しと 5 回のやり直し。**読み直しから rename までの短い隙間は受け入れる**（decisions.md）。Codex の設定は今ある表に触らない |
| 13 | boot の生成（配布物・skill・テンプレート・symlink） | `andon boot`（セルの起動ごとに 1 回） | 同時に 2 本走ると `.bak` が増えるか、後の方が先の書き込みを上書きする | ⑥同じ内容なら書き直さない（`boot.json` の sha256 控え）。人が書いたものは `.bak` に退避してから書く（黙って消さない）。boot 自体の lock は無い（G6） |
| 14 | boot の起動の段（portless・files サーバ・sshd・git の資格情報） | `andon boot` | 同じポートで 2 つ起動しようとして後の方が bind に失敗する | ⑥起こす前にポートが listen しているかを見る。`gh auth setup-git` は何度打っても同じ。lock は無い（G6） |
| 15 | `andon publish`（`files/` への書き出し） | 打った者（担当・Chief of Staff・人） | 同じ日・同じ slug は上書き | ⑥同じ日・同じ slug の再公開は上書きと決めてある。一時ファイル＋rename。上限 20 MiB |
| 16 | hooks（Stop・UserPromptSubmit） | Claude Code が呼ぶ（managed settings）。判断は持たず command を呼ぶだけ | #7・#11 に同じ | 呼ぶ先の command の守り（#7・#11・#1）。UserPromptSubmit の `drain --context` は読むだけで控えを書かない |
| 17 | マージのあとの影響を集める（`andon issue impact`。§2.5） | dispatcher（`log.jsonl` の `{"status": "done"}` の行ごとに 1 回）。Chief of Staff が手で打つこともある | 同じマージの `impact` が 2 度届く。command は読むだけ（`git fetch` のほか書かない）なので、事実が壊れることは無い | ②控えの `log_pos`（`done` の行は 1 度しか読まない）と `Impact`（打ったら外す。失敗は 3 回まで打ち直し、諦めたら `error` の `impact` を積む）。⑥command は読むだけで何度打っても同じ事実から同じ出力。**手で打つ分は防がない**が、手で打った Chief of Staff はもう読んでいる |
| 18 | 人の手直しを集める（`corrections.json`。§2.6） | dispatcher（`log.jsonl` の `{"status": "done"}` の行ごとに 1 回）。ほかの書き手はいない | `corrections.json` が書き直される（`at` が替わる）。2 度目の読みの間に PR のコメントが増えていれば中身も替わる | ④`corrections.json` が在れば書かない（ファイルが「集めた」の印）＋②控えの `log_pos`（`done` の行は 1 度しか読まない）と `Corrections`（書いたら外す。失敗は次の見回りの 1 回だけ打ち直し、諦めたら `corrections` を積んで外す）。書くのは一時ファイル＋rename |
| 19 | 人がマージした PR を拾って片付ける（§2.7） | dispatcher（`review` の課題の PR ごとに1分に1回 `gh pr view`）。Chief of Staff が手で `andon issue finish` することもある | ゲートの問いを二度閉じようとする・finish が二度走る（2 度目は `done` の課題なので台帳の CLI が受けるか止まる）。通知 `merged` が二度届く | ①控えの lock（#2）＋②控えの `Watch`（MERGED・CLOSED と分かった PR はもう聞かない・同じ `held` は二度知らせない・片付けたら外す）＋④台帳の `status`（`review` でなくなれば見ない）。マージは打たない |

### 7.3 守りが無い所（直さない。報告のための一覧）

| # | どこ | 起きること | 重さ | 直すなら |
|---|---|---|---|---|
| G1 | `andon dispatcher` に**プロセス単位の singleton の lock が無い**（#1） | 人が別のシェルで `andon dispatcher` を打つと 2 本回る。巡の中身は #2 の控えの lock で直列になり、後から来た方の巡はそのプロジェクトを飛ばしてログに 1 行出す。レビュアーを二重に起こしはしないが、ログが荒れ、片方が `drain` と lock を取り合って 10 秒待たせることがある | 低（実害は小さい） | `andon dispatcher` の先頭で、控えの置き場の `.dispatcher.lock` を非ブロックで取る（`.terminal.lock` と同じ作り）。取れなければ 1 行出して終わる |
| G2 | ゲートに lock が無い（#4）。Chief of Staff が `andon gate` を手で打つのと dispatcher の巡が重なる | 記録が上書きされ、PR に commit status とコメントが二重に付く。判定は同じ事実から決まる | 低 | `andon gate` が `<head>.json` を作る間の lock を取る。または commit status とコメントを head ごとに 1 度だけ付ける |
| G3 | 人が PR をマージするのと dispatcher のマージが重なる（#5） | dispatcher の `gh pr merge` が失敗して、すでに MERGED なのに「マージできなかった」という `irreversible` の問いが出る（誤報） | 低〜中（人を呼ぶ） | 失敗のあと `gh pr view` で `MERGED` かつ `--match-head-commit` と同じ head なら、問いにせず `finish` へ進む |
| G4 | `kanban tick` の走らせ手が決まっていない（#8） | 入れる前は routine の `schedule` が動かない。人が cron で打つ形で入れれば、dispatcher が打つ分と二重になりうる（台帳の側の守りで同じ時刻は 1 度しか撃たない） | 低 | dispatcher だけが打つと決めて §2.1.1 の 3 と 4 の間に入れる（この文書で順は固定済み）。cron は置かない |
| G5 | `andon chief-of-staff` の「見る→起こす」の間に lock が無い（#10。`andon issue start` は台帳ごとの issue lock を命令の間持つので塞いだ。[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 13 の 4） | 同時に 2 本打つと Chief of Staff が 2 人になる（次に打ったとき 2 人以上で止まる）。打つのは 1 人（人）なので起きにくい | 低 | lock の中で数えてから起こす |
| G6 | `andon boot` に lock が無い（#13・#14） | 2 本同時に走ると生成物の書き込みが重なる。起動の段はポートで守られる | 低（セルの起動ごとに 1 回） | 控えの置き場の `boot.lock` |
| G7 | **別のマシンをまたぐと、①の lock は効かない**（#1・#2・#7・#9） | 控えも台帳の lock も指揮側のマシンの普通のファイル。指揮側が 2 つのマシンに分かれる（dispatcher を 2 台で回す）形は、今の守りでは**成り立たない**。作業側のマシンから台帳を直接書く形も同じ（`flock` は NFS などで効かない保証が無い） | 前提の違反（今は起きない） | 指揮側は 1 つのマシンと決めたまま（design.md §1.5）、作業側からの入口を `andon issue …` の command に閉じ、別のマシンなら `hc` 越しに指揮側で打つ。**作業側の Stop フック（`stop-check`）が台帳を読む分は、別のマシンでは指揮側へ写す形にする必要がある**（#11。この PR の外） |
| G8 | 控えが無くなると（消した・別のマシンへ移した）、「初めて見る台帳」として扱う（§2.1） | 通知の元は位置を控えるだけで過去を流さない。ただし**ゲートを回した記録の控え（`Gates`）も失う**ので、`review` の課題のゲートは回さず記録だけ置く。レビュアーは台帳と端末のラベルで引き継ぐ（#3）。マージ済みの印は `library/gate/<head>.json` の `merged` | 低 | 控えの置き場を台帳の外（`~/.local/state`）に置くのは決めたこと（§6）。移すときは控えごと移す |

### 7.4 この表を保つ

- 常駐・定期の処理を足すとき（`kanban tick` を入れる・新しい routine の道・外の信号〈`signal`〉・別のマシンの dispatcher）は、**この表に行を足してから**入れる。「走らせる者は 1 者か」「二重に走ったら何が起きるか」「守りは何か」を空欄にしない。
- 守りを足したら、7.3 の該当行を消して 7.2 の「今の守り」へ移す。

---

## 出典

- 前の工場の通知の係の設計（2026-11-03。private の記録）: herdr-flightdeck の `docs/yard-news.md`（担当が打ち込んで書きかけに貼り付いた件・Shepherdr〈公開の OSS〉の news の形・入力欄の読み方・`<no-reply>` と `news.replied`・人の決定 2026-11-03）と、template-yard の `docs/orchestration.md` §3.3（知らせの元・届ける前の守り・残る危険）。
- 前の工場の実装: template-yard の `lib/yard/news`（見回り・止まった担当の見分け・提供元のエラーの再送・入力欄の読み方・ANSI の dim の組み直し）と、その変更の記録（2026-11-04。薄い字のサジェストで約40分・11件が溜まった件、組み直しを `hc read` と突き合わせる形・制御列の許可の一覧・控えて戻す列・colon 形式の色。独立レビューの指摘から）。
- shepherdr（MIT）の scheduler: 提供元に切られたときの「続けて」の間（1・2・4・8 分）と、5 回目に切られたら人へ上げる形（§2.4）。
- Andon の側: [design.md](design.md) §0.1・§1.7・§3.2 F1・F8・§5.2 P3b・P6a・P6b、[contract.md](contract.md) §6・§8・§9、[decisions.md](decisions.md) §6（2026-11-03 の通知の決定）・§7（F1・F8 の回答）。
