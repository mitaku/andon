---
読む時: トリアージの課題を足す・起動する・`review` を受ける・適用するとき（`memory` の通知を受けたときも）と、報告の Next にほかのプロジェクトへの委譲があったとき
消す条件: 足す・起動する・適用する・続けるの段は「`andon triage apply` が適用のあと finish と足した課題の起動まで回し、オーナーへの1行を出すようになったら」。委譲の形（規約10）は「ほかのプロジェクトへの委譲を command が送るようになったら」
---

# トリアージ

仕分け（Triage）のワークストリームの AI の手。流れの正は andon の repo の `process/workstreams/triage.yaml` で、このファイルはやり方だけを書く。

kanban に溜まったもの（報告の Next・見送り・Remember・For AGENTS.md・オーナーに聞くこと・担当の無い todo・オーナーの判断待ち・片付けの残り）を、担当に仕分けさせて片付ける。
判定表と decisions.json の形は triage skill にあり、ここには書かない。

1. **足す**: ルーチン `triage`（`routines/triage.yaml`。例は triage skill の `assets/triage.yaml`。kanban へ写すのはオーナーの go で）が週1で足す。
   オーナーがゴールを1行言ったら `kanban add "トリアージ: <ゴール>" --profile sonnet --brief -`（brief は `assets/triage.yaml` の brief の `ゴール:` 行を差し替えたもの）。
   枠が空いたままのときは、仕分けていない項目が 1 回分（50 件）以上で、片付いていないトリアージが無ければ、dispatcher が同じルーチンを撃つ（`idle.md`。ADR-0047 決めたこと 7）。足されたものは週1の回と同じく扱う。
   `memory` の通知（memory が線を超えた）を受けたら、臨時のトリアージを同じ形で足す（ゴールは「memory を蒸留して線の下に戻す」）。
   `todo`／`doing`／`review` のトリアージがあれば足さない（同時に1つ）。
2. **起動する**: ルーチンが足したものもオーナーが言ったものも、オーナーに聞かずに `andon issue start <id>`（`start.md`。枠〈`andon waiting` の `slots.free`〉とぶつかりは守る）。
3. **`review` が来たら**: 報告の要約表と `issues/<id>/library/decisions.json` を読み、`andon triage apply <id> --dry-run` を打つ。
   閉じる理由が読めない・オーナーへのものが ask-one-by-one §1 の線に合わない・判断の推奨案と選択肢がオーナーがペインで答えられるほど具体でない、なら担当に `hc send` で戻す。
   **判断の閉じと課題の閉じを1件ずつ読む**: `--dry-run` の出力の `review`（`andon triage apply <id> --dry-run | jq .review`）は、担当の判断だけで閉じる項目（起きない・その場限り・当てはまらない・読んだ）と、kanban の課題（担当の無い課題・オーナーの判断待ち）を閉じる項目。
   1件ずつ、本文と出どころ（`from` の課題の報告・memory の本文・課題の依頼書）を読み、項目の `commit` の repo も読んで、閉じて失われるものが無いかを自分で見立てる。
   - 次の担当が踏みうる学び・まだ残るやり残し・オーナーがまだ答えていない問いが失われうるなら、`andon triage verdict <id> <key> overturn`（覆す）。
   - 失われるものが無ければ `andon triage verdict <id> <key> ok`（閉じてよい）。「その場限り」かどうかの線は判定表に無いので、ここで決める。
   - 判定は key と担当の閉じ方・理由に結びつく。担当が書き直したら、その項目は読み直す（`verdict` が `stale` になる）。判定を書くのは自分の端末だけで、担当には書かせない。書き手が今の自分の端末でない判定は数えられない（`foreign`。起き直して端末が替わったら書き直す）。
   覆したものがあれば、key と行き先の案（直す・オーナーへ・wiki へ・次回へ）を添えて担当に `hc send` で戻す。覆したことは適用のあとの記録に残る。
4. **適用する**: `andon triage apply <id>`。起動する上限は、`apply` が `projects/<slug>/project.json` の `chief_of_staff.triage_max_issues`（無ければ 3）から読む。上限はオーナーが `project.json` に書くもので、`--max-issues` で変えない。
   `review` の全部に「閉じてよい」が揃うまで `apply` は kanban に手を入れずに断る（覆したものが残っていても断る）。終了コード 1 は理由を読み、判定が足りなければ 3 に戻り、ほかは担当に戻す。
   閉じるオーナーの課題に、答えられてまだ受け取られていない問いがあると、`apply` は何も書かずに断る。答えを受け取ってから（ask-one-by-one §3）、担当にその項目を仕分け直させる。
   要約は `library/triage/<日付>-<id>.md`、記録は同じ名前の `.json`。適用したら `andon issue finish <id>`（PR の無い課題。端末を片付けて done にする）。
5. **続ける**: `apply` が足した fix／bundle の課題は、予約キーを既定（`review` は決定の値・`merge=person`・`estimate=medium` など）で持って足される。既定から変えるものだけ書き直し（`review=on` の課題の profile を替えたら、`reviewer` も別ブランドに書き直す）、オーナーに聞かずに `start.md` の手順で起動する（枠〈`andon waiting` の `slots.free`〉の中で。残りは次の周回）。
   報告の `## Next` の「〜の Chief of Staff へ委譲」は規約10の形で渡す（下）。`keep`「片付け待ち」は `finish.md` の手順で `andon issue finish`。
   memory の項目（`kind: memory`）で `close`「wiki へ: <タイトル>」のものは、記録（`library/triage/<日付>-<id>.json`）の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest する（`wiki.md`）。repo 行きは `apply` が足した課題が運ぶ。memory のファイルと目録の行は `apply` が消す（自分で消さない）。
6. **オーナーへ**: 知らせる（オーナーがいるターンならその返事で、いなければ `herdr notification show "<slug> のトリアージで a 件を閉じた。b 件を直す課題にし、c 件をオーナーへ回した。要約は library/triage/…"`。TUI が動いているときだけ効く。無ければ kanban の問いとボードで足りる）。
   空きで dispatcher が撃った回は、回のたびに通知を出さず、オーナーがいるターンの返事に 1 文にまとめて添える（`idle.md` の「オーナーへ」）。
   オーナーの課題は ask-one-by-one skill §3 と同じ形で足される。判断は `apply` が問いの JSON（`from: chief-of-staff`、`facts` の先頭が出どころ）にして `andon issue ask --new` と同じ関数で置くので、行燈が灯り、オーナーはペインから選択肢で答えられる。手作業は印 `only_owner` を持つ課題になり、灯らない。答えの受け取りは ask-one-by-one §3 のとおり。要約に問いを並べない。要約の「閉じた」は、自分が読んだもの（判断の閉じと課題の閉じ）が先、根拠の閉じが後に出る。
   閉じたものはあとからも戻せる（戻す＝次のトリアージの依頼書に「<key> を fix に」と書く）。
7. **止める**: ルーチンの `paused: true` はオーナーが書く。要約の先頭に「線引きを見直す」が出たら（種類を決めきれなかった判断〈`unsure`〉が 3 を超えた）、オーナーに伝える。

## ほかのプロジェクトへの委譲（規約10: 持ち場の外は、その持ち場の CoS へ）

`hc send <相手の config.yaml の chief_of_staff> "[<slug> の chief-of-staff から委譲] …"`。ほかのプロジェクトの kanban には書かない。この印の付いた依頼は、受け手はオーナーの承認とみなさない。
