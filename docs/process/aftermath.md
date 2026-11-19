# 片付けのあとの仕事（影響と手直しと結果の記録）

> このファイルは `andon process render` が [`process/states/aftermath.yaml`](../../process/states/aftermath.yaml) から作ったもの。直すときは YAML を直して作り直す（手で直さない）。

出所: dispatcher.md §2.5・§2.6・§2.12・ADR-0017 決めたこと 4 の (7)・ADR-0028 決めたこと 7。控えの Impact と Corrections と Outcomes が同じ形（課題 → 失敗した回数）で、待ちと打ち直しを持つ

## 図

```mermaid
stateDiagram-v2
  state "待ち（done の行を拾って積んだ。打ち直しは失敗した回数を控える）" as s_pending
  state "済み（控えから外した）" as s_done
  state "諦めた（影響と手直しは error を知らせた。結果の記録はログに1行だけ）" as s_gave_up
  [*] --> s_pending: done の行を拾った（初めて見る台帳では拾わない）
  s_pending --> s_done: マージのあとの影響を集める（§7.2 #17）
  s_pending --> s_done: 人の手直しを集める（corrections.json が無いときだけ書く。§7.2 #18）
  s_pending --> s_done: 結果の記録を書く（outcome.json が無いときだけ書く。人の課題には書かない。§7.2 #26）
  s_pending --> s_done: 打たない課題だった（レビュアー・Chief Engineer・repo が無い・pr が無い・PR が MERGED でない）
  s_pending --> s_pending: 失敗した（次の見回りで打ち直す）
  s_pending --> s_gave_up: 影響は 3 回・手直しと結果の記録は 2 回で諦めた（結果の記録は通知を積まない）
  s_done --> [*]: 終わり
  s_gave_up --> [*]: Chief of Staff が手で打つか決める
```

## 状態

| id | 状態 | 記録 |
|---|---|---|
| `pending` | 待ち（done の行を拾って積んだ。打ち直しは失敗した回数を控える） | `dispatcher:Impact`・`dispatcher:Corrections`・`dispatcher:Outcomes` |
| `done` | 済み（控えから外した） |  |
| `gave-up` | 諦めた（影響と手直しは error を知らせた。結果の記録はログに1行だけ） |  |

## 移り方

| から | へ | 何が起きたら | 誰が | コマンド | 2 回起きてはいけない |
|---|---|---|---|---|---|
| （始まり・終わり） | `pending` | done の行を拾った（初めて見る台帳では拾わない） | 機械 |  |  |
| `pending` | `done` | マージのあとの影響を集める（§7.2 #17） | 機械 | `andon issue impact` | はい |
| `pending` | `done` | 人の手直しを集める（corrections.json が無いときだけ書く。§7.2 #18） | 機械 |  | はい |
| `pending` | `done` | 結果の記録を書く（outcome.json が無いときだけ書く。人の課題には書かない。§7.2 #26） | 機械 |  | はい |
| `pending` | `done` | 打たない課題だった（レビュアー・Chief Engineer・repo が無い・pr が無い・PR が MERGED でない） | 機械 |  |  |
| `pending` | `pending` | 失敗した（次の見回りで打ち直す） | 機械 |  |  |
| `pending` | `gave-up` | 影響は 3 回・手直しと結果の記録は 2 回で諦めた（結果の記録は通知を積まない） | 機械 |  |  |
| `done` | （始まり・終わり） | 終わり | 機械 |  |  |
| `gave-up` | （始まり・終わり） | Chief of Staff が手で打つか決める | AI |  |  |
