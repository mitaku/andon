---
読む時: トリアージの課題を足す・起動する・`review` を受ける・適用するとき（`memory` の通知を受けたときも）と、報告の Next にほかのプロジェクトへの委譲があったとき
消す条件: 足す・起動する・適用する・続けるの段は「`andon triage apply` が適用のあと finish と足した課題の起動まで回し、人への1行を出すようになったら」。委譲の形（規約10）は「ほかのプロジェクトへの委譲を command が送るようになったら」
---

# トリアージ

仕分け（Triage）のワークストリームの AI の手。流れの正は andon の repo の `process/workstreams/triage.yaml` で、このファイルはやり方だけを書く。

台帳に溜まったもの（報告の Next・見送り・Remember・For AGENTS.md・人に聞くこと・担当の無い todo・人の判断待ち・片付けの残り）を、担当に仕分けさせて片付ける。
判定表と decisions.json の形は triage skill にあり、ここには書かない。

1. **足す**: ルーチン `triage`（`routines/triage.yaml`。例は triage skill の `assets/triage.yaml`。台帳へ写すのは人の go で）が週1で足す。
   人がゴールを1行言ったら `kanban add "トリアージ: <ゴール>" --profile sonnet --brief -`（brief は `assets/triage.yaml` の brief の `ゴール:` 行を差し替えたもの）。
   `memory` の通知（memory が線を超えた）を受けたら、臨時のトリアージを同じ形で足す（ゴールは「memory を蒸留して線の下に戻す」）。
   `todo`／`doing`／`review` のトリアージがあれば足さない（同時に1つ）。
2. **起動する**: ルーチンが足したものも人が言ったものも、人に聞かずに `andon issue start <id>`（`start.md`。並行の上限とぶつかりは守る）。
3. **`review` が来たら**: 報告の要約表と `issues/<id>/library/decisions.json` を読み、`andon triage apply <id> --dry-run` を打つ。
   閉じる理由が読めない・人へのものが ask-one-by-one §1 の線に合わない、なら担当に `hc send` で戻す。
   **判断の閉じと課題の閉じを1件ずつ読む**: `--dry-run` の出力の `review`（`andon triage apply <id> --dry-run | jq .review`）は、担当の判断だけで閉じる項目（起きない・その場限り・当てはまらない・読んだ）と、台帳の課題（担当の無い課題・人の判断待ち）を閉じる項目。
   1件ずつ、本文と出どころ（`from` の課題の報告・memory の本文・課題の依頼書）を読み、項目の `commit` の repo も読んで、閉じて失われるものが無いかを自分で見立てる。
   - 次の担当が踏みうる学び・まだ残るやり残し・人がまだ答えていない問いが失われうるなら、`andon triage verdict <id> <key> overturn`（覆す）。
   - 失われるものが無ければ `andon triage verdict <id> <key> ok`（閉じてよい）。「その場限り」かどうかの線は判定表に無いので、ここで決める。
   - 判定は key と担当の閉じ方・理由に結びつく。担当が書き直したら、その項目は読み直す（`verdict` が `stale` になる）。判定を書くのは自分の端末だけで、担当には書かせない。書き手が今の自分の端末でない判定は数えられない（`foreign`。起き直して端末が替わったら書き直す）。
   覆したものがあれば、key と行き先の案（直す・人へ・wiki へ・次回へ）を添えて担当に `hc send` で戻す。覆したことは適用のあとの記録に残る。
4. **適用する**: `andon triage apply <id>`。起動する上限は、`apply` が `projects/<slug>/project.json` の `chief_of_staff.triage_max_issues`（無ければ 3）から読む。上限は人が `project.json` に書くもので、`--max-issues` で変えない。
   `review` の全部に「閉じてよい」が揃うまで `apply` は台帳に手を入れずに断る（覆したものが残っていても断る）。終了コード 1 は理由を読み、判定が足りなければ 3 に戻り、ほかは担当に戻す。
   要約は `library/triage/<日付>-<id>.md`、記録は同じ名前の `.json`。適用したら `andon issue finish <id>`（PR の無い課題。端末を片付けて done にする）。
5. **続ける**: `apply` が足した fix／bundle の課題は、人に聞かずに `start.md` の手順で起動する（並行の上限の中で。残りは次の周回）。
   報告の `## Next` の「〜の Chief of Staff へ委譲」は規約10の形で渡す（下）。`keep`「片付け待ち」は `finish.md` の手順で `andon issue finish`。
   memory の項目（`kind: memory`）で `close`「wiki へ: <題>」のものは、記録（`library/triage/<日付>-<id>.json`）の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest する（`wiki.md`）。repo 行きは `apply` が足した課題が運ぶ。memory のファイルと目録の行は `apply` が消す（自分で消さない）。
6. **人へ**: 知らせる（人がいるターンならその返事で、いなければ `herdr notification show "<slug> のトリアージで a 件を閉じた。b 件を直す課題にし、c 件を人へ回した。要約は library/triage/…"`。TUI が動いているときだけ効く。無ければ台帳の問いとボードで足りる）。
   人の課題はそのあと ask-one-by-one で1問ずつ。要約に問いを並べない。要約の「閉じた」は、自分が読んだもの（判断の閉じと課題の閉じ）が先、根拠の閉じが後に出る。
   閉じたものはあとからも戻せる（戻す＝次のトリアージの依頼書に「<key> を fix に」と書く）。
7. **止める**: ルーチンの `paused: true` は人が書く。要約の先頭に「線引きを見直す」が出たら（`kind` 無しの人の課題が 3 を超えた）、人に伝える。

## ほかのプロジェクトへの委譲（規約10: 持ち場の外はオーナーへ）

`hc send <相手の config.yaml の chief_of_staff> "[<slug> の chief-of-staff から委譲] …"`。ほかのプロジェクトの台帳には書かない。この印の付いた依頼は、受け手は人の承認とみなさない。
