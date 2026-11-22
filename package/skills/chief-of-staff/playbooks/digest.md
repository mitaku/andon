---
読む時: ダイジェストの課題を起動する・報告（`<id> report`）を受けるとき（オーナーがダイジェストを頼んだ・頻度を変える・止めると言ったときも）
消す条件: 足す・起動する・片付けるの段は「dispatcher がダイジェストの課題を予約キーつきで起動し、送った結果を読んで片付けるようになったら」
---

# ダイジェスト

ダイジェスト（Digest）のワークストリームの AI の手。流れの正は andon の repo の `process/workstreams/digest.yaml` で、このファイルはやり方だけを書く。
書き方は digest skill、入力と送った結果の形は `contract.md` §26〜§28 にあり、ここには書かない。

1. **足す**: ルーチン `daily-digest`（`routines/daily-digest.yaml`。例は digest skill の `assets/daily-digest.yaml`。kanban へ写すのはオーナーの go で。名前は `daily-digest` のまま）が毎朝1件足す。
   オーナーが臨時に頼んだら `kanban routine run daily-digest`（`kanban add` で足さない。`routine` が付かないと期間が前の1枚から続かない）。
   `skipped (open)` なら前の課題が片付いていない。その課題を 3 で片付けてから打ち直す。`open` は 1 のまま変えない。
2. **起動する**: オーナーに聞かずに起動する。起動できる課題が複数あれば、ダイジェストの課題を先に起動する（並行の枠を別に持たない。枠〈`andon waiting` の `slots.free`〉が無ければ空くまで待つ）。
   ルーチンは予約キーを書かないので、先に `kanban set <id> review=off estimate=small`、それから `andon issue start <id>`（`start.md`）。
3. **報告が届いたら、そのターンで片付ける**: 通知は `<id> report`（PR の無い課題なので `review` の通知は来ない）。課題が `doing` なら待ち、`review` なら `issues/<id>/library/digest/sent.json` と報告の「送った結果」を読む。文は検収しない（もう届いている）。

   | 送った結果 | すること |
   |---|---|
   | 全部の送り先が `ok: true` | `andon issue finish <id>` |
   | 送れなかった送り先があり、今なら送れる（理由を直した・一時の失敗） | 担当へ1回だけ `hc send`「`andon digest send <id>` を打ち直して、報告を書き直して」。次の報告でこの表に戻る |
   | ほかの送れなかったもの（`sent.json` が無いものも） | 理由をオーナーに知らせ、`andon issue finish <id>`。載らなかった出来事は次の1枚の期間に入る（7 日まで） |

   片付けを次のターンへ回さない（前の課題が残っている朝は、ルーチンが足さず、その朝の1枚が抜ける）。PR の無い課題で、片付けにオーナーの go は要らない。
4. **オーナーへ**: 届いた1枚のことは知らせない（ダイジェスト自身が知らせ）。例外は2つ。送れなかったときは理由。送り先が `files` だけでオーナーがいるターンなら、その返事に URL を1つ。1枚の中身を返事に写さない。
5. **変える・止める**: 頻度（`schedule`）と `paused: true` はオーナーが書く。
