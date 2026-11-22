# オーナーのマージの見張り

> このファイルは `andon process render` が [`process/states/human-merge.yaml`](../../process/states/human-merge.yaml) から作ったもの。直すときは YAML を直して作り直す（手で直さない）。

出所: dispatcher.md §2.7・ADR-0017 決めたこと 4 の (6)・5・6。andon issue settle の記録（<slug>.settle.json。書き手は settle だけ）の 1 件（課題ごと）が PR・聞いた時刻・MERGED／CLOSED・知らせた理由を持つ。dispatcher は review・doing の課題ごとに settle を打ち、返った通知を積むだけ

## 図

```mermaid
stateDiagram-v2
  state "見ている（PR ごとに 1 分に 1 回 gh pr view。State が空）" as s_watching
  state "MERGED と分かった（gate の記録の merged が true なら dispatcher がマージしたので、ここで止まる）" as s_merged
  state "CLOSED（マージなし）と知らせた（開き直されうるので、PR ごとに 1 分に 1 回 gh pr view を続ける）" as s_closed
  state "片付けられない（gate の外の問いが開いている・finish が失敗した〈commit していない変更など〉。同じ理由は 1 回だけ知らせる）" as s_held
  state "片付けた" as s_finished
  [*] --> s_watching: review か doing にいて report.json に pr がある（dispatcher が andon issue settle を打つ）
  s_watching --> s_merged: MERGED になった（dispatcher の gh pr merge の前か、打ち直しを待つ間にオーナーがマージしていたときも）
  s_watching --> s_closed: CLOSED になった
  s_merged --> s_finished: オーナーがマージした PR を拾って片付ける（gate の問いを閉じる。§7.2 #19）
  s_merged --> s_held: gate の外の問いが開いている・finish が失敗した
  s_held --> s_finished: オーナーがマージした PR を拾って片付ける（問いが閉じた次の巡。§7.2 #19）
  s_watching --> [*]: review にも doing にもいなくなった・report.json の pr か commit が替わった（記録を捨てる）
  s_closed --> s_watching: 開き直された（OPEN に戻った。また閉じたら知らせる）
  s_closed --> s_merged: 開き直してマージされた
  s_closed --> [*]: やめるかを Chief of Staff とオーナーが決める
  s_finished --> [*]: 終わり
```

## 状態

| id | 状態 | 記録 |
|---|---|---|
| `watching` | 見ている（PR ごとに 1 分に 1 回 gh pr view。State が空） | `settle:State` |
| `merged` | MERGED と分かった（gate の記録の merged が true なら dispatcher がマージしたので、ここで止まる） |  |
| `closed` | CLOSED（マージなし）と知らせた（開き直されうるので、PR ごとに 1 分に 1 回 gh pr view を続ける） |  |
| `held` | 片付けられない（gate の外の問いが開いている・finish が失敗した〈commit していない変更など〉。同じ理由は 1 回だけ知らせる） | `settle:Held` |
| `finished` | 片付けた |  |

状態でない項目（拾う位置とデータ）: `settle:PR`・`settle:Commit`・`settle:At`・`settle:Head`・`settle:Sha`

## 移り方

| から | へ | 何が起きたら | 誰が | コマンド | 2 回起きてはいけない |
|---|---|---|---|---|---|
| （始まり・終わり） | `watching` | review か doing にいて report.json に pr がある（dispatcher が andon issue settle を打つ） | 機械 | `andon issue settle` |  |
| `watching` | `merged` | MERGED になった（dispatcher の gh pr merge の前か、打ち直しを待つ間にオーナーがマージしていたときも） | オーナー |  |  |
| `watching` | `closed` | CLOSED になった | オーナー |  |  |
| `merged` | `finished` | オーナーがマージした PR を拾って片付ける（gate の問いを閉じる。§7.2 #19） | 機械 | `andon issue settle` | はい |
| `merged` | `held` | gate の外の問いが開いている・finish が失敗した | 機械 | `andon issue settle` |  |
| `held` | `finished` | オーナーがマージした PR を拾って片付ける（問いが閉じた次の巡。§7.2 #19） | 機械 | `andon issue settle` | はい |
| `watching` | （始まり・終わり） | review にも doing にもいなくなった・report.json の pr か commit が替わった（記録を捨てる） | 機械 | `andon issue settle` |  |
| `closed` | `watching` | 開き直された（OPEN に戻った。また閉じたら知らせる） | オーナー |  |  |
| `closed` | `merged` | 開き直してマージされた | オーナー |  |  |
| `closed` | （始まり・終わり） | やめるかを Chief of Staff とオーナーが決める | オーナー |  |  |
| `finished` | （始まり・終わり） | 終わり | 機械 |  |  |
