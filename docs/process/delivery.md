# 通知の配達

> このファイルは `andon process render` が [`process/states/delivery.yaml`](../../process/states/delivery.yaml) から作ったもの。直すときは YAML を直して作り直す（手で直さない）。

出所: dispatcher.md §2.2・§3・§3.2・§4・ADR-0010・ADR-0017 決めたこと 4 の (3)・5。拾う位置（log.jsonl の位置と ready の前回）は状態でないので、ここに置く

## 図

```mermaid
stateDiagram-v2
  state "待ち（届ける通知。同じ課題の同じ出来事は新しい 1 件に置き換える）" as s_pending
  state "離席時に hc send で渡した（経路③。送った端末と turn を控える）" as s_away_sent
  state "フックで渡した（経路①②。渡した turn を控える）" as s_hooked
  state "確かめた（news.replied を書いた数を控える）" as s_acked
  state "記録だけの置き場に書けなかった" as s_unrecorded
  state "記録だけにした（届けない）" as s_recorded
  [*] --> s_pending: 届ける通知を拾った
  [*] --> s_recorded: 記録だけの通知を拾った
  [*] --> s_unrecorded: 記録だけの通知を置き場に書けなかった
  s_unrecorded --> s_recorded: 次の見回りで先に書いた
  s_pending --> s_hooked: 通知を Chief of Staff へ届ける（ターンの終わり・人が送った。経路①②。§7.2 #7）
  s_pending --> s_away_sent: 通知を Chief of Staff へ届ける（離席の守りがそろった。経路③。§7.2 #7）
  s_hooked --> s_acked: 渡した turn の Stop が来た
  s_hooked --> s_hooked: 確かめないまま別の turn が来た（1 回だけ届け直す）
  s_away_sent --> s_acked: news を渡した turn が進んだ
  s_acked --> [*]: 終わり
  s_recorded --> [*]: 終わり
```

## 状態

| id | 状態 | 記録 |
|---|---|---|
| `pending` | 待ち（届ける通知。同じ課題の同じ出来事は新しい 1 件に置き換える） | `dispatcher:Pending` |
| `away-sent` | 離席時に hc send で渡した（経路③。送った端末と turn を控える） | `dispatcher:Coord`・`dispatcher:NewsTurn` |
| `hooked` | フックで渡した（経路①②。渡した turn を控える） | `dispatcher:Hook` |
| `acked` | 確かめた（news.replied を書いた数を控える） | `dispatcher:Replied` |
| `unrecorded` | 記録だけの置き場に書けなかった | `dispatcher:Unrecorded` |
| `recorded` | 記録だけにした（届けない） |  |

状態でない項目（拾う位置とデータ）: `dispatcher:LogPos`・`dispatcher:LogHead`・`dispatcher:LogHold`・`dispatcher:Ready`

## 移り方

| から | へ | 何が起きたら | 誰が | コマンド | 2 回起きてはいけない |
|---|---|---|---|---|---|
| （始まり・終わり） | `pending` | 届ける通知を拾った | 機械 |  |  |
| （始まり・終わり） | `recorded` | 記録だけの通知を拾った | 機械 |  |  |
| （始まり・終わり） | `unrecorded` | 記録だけの通知を置き場に書けなかった | 機械 |  |  |
| `unrecorded` | `recorded` | 次の見回りで先に書いた | 機械 |  |  |
| `pending` | `hooked` | 通知を Chief of Staff へ届ける（ターンの終わり・人が送った。経路①②。§7.2 #7） | 機械 | `andon dispatcher drain` | はい |
| `pending` | `away-sent` | 通知を Chief of Staff へ届ける（離席の守りがそろった。経路③。§7.2 #7） | 機械 | `hc send --if idle` | はい |
| `hooked` | `acked` | 渡した turn の Stop が来た | AI |  |  |
| `hooked` | `hooked` | 確かめないまま別の turn が来た（1 回だけ届け直す） | 機械 |  |  |
| `away-sent` | `acked` | news を渡した turn が進んだ | AI |  |  |
| `acked` | （始まり・終わり） | 終わり | 機械 |  |  |
| `recorded` | （始まり・終わり） | 終わり | 機械 |  |  |
