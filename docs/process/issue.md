# 課題

> このファイルは `andon process render` が [`process/states/issue.yaml`](../../process/states/issue.yaml) から作ったもの。直すときは YAML を直して作り直す（手で直さない）。

出所: ledger.md（Status rules・Questions）・worker skill §2〜§7・ADR-0017 決めたこと 4 の (1)・15。記録は台帳で、dispatcher の控えに項目を持たない

## 図

```mermaid
stateDiagram-v2
  state "待つ課題がある（queued）" as s_queued
  state "引き取れるのを待つ（todo）" as s_todo
  state "担当が働いている（doing）" as s_doing
  state "返した（review）" as s_review
  state "片付けた（done）" as s_done
  [*] --> s_todo: 課題を足した
  [*] --> s_queued: after つきで課題を足した
  s_queued --> s_todo: 待つ課題が全部 done になった
  s_todo --> s_doing: 担当を起こした（§7.2 #10）
  s_doing --> s_doing: 報告・レビュー依頼・候補の一覧・問いを書いた
  s_doing --> s_review: 確かめて返した
  s_review --> s_doing: 直しが届いた
  s_review --> s_done: マージのあと片付けた
  s_done --> [*]: 終わり
```

## 状態

| id | 状態 | 記録 |
|---|---|---|
| `queued` | 待つ課題がある（queued） |  |
| `todo` | 引き取れるのを待つ（todo） |  |
| `doing` | 担当が働いている（doing） |  |
| `review` | 返した（review） |  |
| `done` | 片付けた（done） |  |

## 移り方

| から | へ | 何が起きたら | 誰が | コマンド | 2 回起きてはいけない |
|---|---|---|---|---|---|
| （始まり・終わり） | `todo` | 課題を足した | 台帳の CLI | `kanban add` |  |
| （始まり・終わり） | `queued` | after つきで課題を足した | 台帳の CLI | `kanban add` |  |
| `queued` | `todo` | 待つ課題が全部 done になった | 台帳の CLI | `kanban done` |  |
| `todo` | `doing` | 担当を起こした（§7.2 #10） | AI | `andon issue start` | はい |
| `doing` | `doing` | 報告・レビュー依頼・候補の一覧・問いを書いた | AI | `andon issue report・review-request・candidates・ask` |  |
| `doing` | `review` | 確かめて返した | AI | `andon issue ready` |  |
| `review` | `doing` | 直しが届いた | 台帳の CLI | `kanban start` |  |
| `review` | `done` | マージのあと片付けた | 機械 | `andon issue finish` |  |
| `done` | （始まり・終わり） | 終わり | 台帳の CLI |  |  |
