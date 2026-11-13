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
4. **適用する**: `andon triage apply <id> --max-issues <AGENTS.md の Chief of Staff の節の「トリアージ: 起動する上限 N」。無ければ 3>`。終了コード 1 は理由を読んで担当に戻す。
   要約は `library/triage/<日付>-<id>.md`、記録は同じ名前の `.json`。適用したら `andon issue finish <id>`（PR の無い課題。端末を片付けて done にする）。
5. **続ける**: `apply` が足した fix／bundle の課題は、人に聞かずに `start.md` の手順で起動する（並行の上限の中で。残りは次の周回）。
   報告の `## Next` の「〜の Chief of Staff へ委譲」は規約10の形で渡す（下）。`keep`「片付け待ち」は `finish.md` の手順で `andon issue finish`。
   memory の項目（`kind: memory`）で `close`「wiki へ: <題>」のものは、記録（`library/triage/<日付>-<id>.json`）の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest する（`wiki.md`）。repo 行きは `apply` が足した課題が運ぶ。memory のファイルと目録の行は `apply` が消す（自分で消さない）。
6. **人へ**: 知らせる（人がいるターンならその返事で、いなければ `herdr notification show "<slug> トリアージ: 閉じた a・直す b・人へ c。要約は library/triage/…"`。TUI が動いているときだけ効く。無ければ台帳の問いとボードで足りる）。
   人の課題はそのあと ask-one-by-one で1問ずつ。要約に問いを並べない。閉じたものは人が要約で見て戻せる（戻す＝次のトリアージの依頼書に「<key> を fix に」と書く）。
7. **止める**: ルーチンの `paused: true` は人が書く。要約の先頭に「線引きを見直す」が出たら（`kind` 無しの人の課題が 3 を超えた）、人に伝える。

## ほかのプロジェクトへの委譲（規約10: 持ち場の外はオーナーへ）

`hc send <相手の config.yaml の chief_of_staff> "[<slug> の chief-of-staff から委譲] …"`。ほかのプロジェクトの台帳には書かない。この印の付いた依頼は、受け手は人の承認とみなさない。
