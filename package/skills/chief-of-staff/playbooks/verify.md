---
読む時: マージした変更をセルへ当てたとき（`image/apply-main.sh apply`）と、検証の課題の報告が届いたとき
消す条件: 「当てる command が、差分が機能の地図に当たるかを見て、検証の課題を自分で足すようになったら」
---

# 当てたあとの検証

セルへ当てた変更が、セルの本物で利用者と同じ道で動くかを、独立の検証の担当に確かめさせる（ADR-0035 決めたこと 4・9）。手順と機能の地図は、当てた repo の検証の skill（`.claude/skills/verify-andon/`）が持つ。自分は、切るかを決めて切り、届いた報告から直す課題を切るだけ。機能を自分で動かさない。

効くのは、当てた commit に `.claude/skills/verify-andon/` があるとき（Andon の repo）だけ。無ければ何もしない。ほかの案件の repo には広げない。

## 当てたら

1. **動いている検証の課題があれば切らない。** 同時に動く検証の課題は1つ。`kanban list --json | jq -r '.[] | select(.title | startswith("検証: ")) | .id'`（done でない課題）が1件でも出れば、ここで終える。今回当てた差分は、その課題を片付けたあとの回（下の「報告が届いたら」の 3）がまとめて見る。
2. **始まりの commit を決める。** 前の検証が見た commit から見る。
   - 最後の検証の課題（`kanban list --all --json | jq -r '[.[] | select(.title | startswith("検証: "))] | last | .id'`）の報告 `issues/<id>/report.md` の、`## Report` の先頭にある見た commit の範囲の右端。
   - その報告に範囲が無い（診断で止まった）なら、その課題の依頼書の `前の検証が見た commit:` の行をそのまま使う。
   - 検証の課題がまだ1つも無ければ、今回当てる前に当たっていた commit（当てる前の `image/apply-main.sh status` の当てた commit）。
3. **差分が地図に当たるかを引く。** 地図は当てた commit のものを使う（checkout の作業ツリーは古いことがある）。当てるのに使った Andon の checkout で打つ:

   ```
   d=<scratchpad>/verify-map; rm -rf "$d" && mkdir -p "$d"
   git archive <当てた commit> .claude/skills/verify-andon | tar -x -C "$d"
   bash "$d/.claude/skills/verify-andon/scripts/features.sh" <始まり>..<当てた commit>
   ```

   | 結果 | すること |
   |---|---|
   | `git archive` が落ちた（当てた commit に検証の skill が無い） | 何もしない |
   | 何も出ず、終了コード 0（差分が地図の「変えたら動かすパス」に当たらない） | 切らない。次の回も同じ始まりから見る |
   | 機能の名前が出た | 4 で切る |
   | 0 以外（地図が壊れている） | 切らない。出力の理由を読み、地図を直す課題に切る（`cut.md`） |

4. **切る。** タイトルは「検証: <当てた commit の先頭 12 桁> を当てたあとの機能」。プロファイルは `claude`。依頼書は次の雛形で、Settings は `brief.md` の「Settings の埋め方」のとおり予約キーも書く:

   ```
   # <id> 検証: <当てた commit の先頭 12 桁> を当てたあとの機能
   解決すべき問題: セルへ <当てた commit> を当てた。差分が当たる機能（<features.sh が出した名前>）が、セルの本物で利用者と同じ道で動くかが確かめられていない。
   なぜ: セルへ当てたあとの検証（ADR-0035 決めたこと 4・9）。
   読む skill: .claude/skills/verify-andon/SKILL.md（worktree の中のもの。Skill tool では呼べないので、ファイルを読んで従う）
   前の検証が見た commit: <始まり>
   当てた commit: <当てた commit>
   機能: <features.sh が出した名前>
   完了条件: 当たる機能ごとに andon issue record の記録がある。報告の ## Report の先頭に、見た commit の範囲（<始まり>..<当てた commit>）と機能ごとの結果（動いた・動かなかった・未確認）がある。地図と手順の食い違いを直したなら、その PR がある。
   守ること: 製品のコードを直さない（直してよいのは .claude/skills/verify-andon/ の中だけ）。エージェントを起動しない。セルの本物の kanban に書かない。地図を直さなかったときは PR が無いので、andon issue report のあと kanban review <id> で返す（andon issue ready は打たない）。

   ## Settings
   - Review: off
   - Merge: person
   - Estimate: 中
   ```

   - `Merge` は、委譲がこのプロジェクトのマージを任せていれば `auto`（`brief.md` の表）。地図の直しは `.claude/skills/` の中だけなので、gate の条件表で自動でマージできる側に入る。
   - 起動は `start.md` のとおり（枠〈`andon waiting` の `slots.free`〉とぶつかりの確かめは守る）。枠が無くて待たせた課題も done でないので、1 が次の切り過ぎを止める。
5. **オーナーへ**: 当てた知らせに1行添える。「検証の課題 <id>（<機能>）を切った」「差分が地図に当たらないので検証は切らない」「検証の課題 <id> が動いているので、片付いたあとにまとめて見る」のどれか。

## 報告が届いたら

検証の課題は、地図を直したら PR つきの `<id> review`、直さなければ `<id> report` で届く。

1. 報告の機能ごとの結果を読み、`## Next` の `to: chief-of-staff` の行を1行ずつ扱う。動かなかった機能は、直す課題に切る（`cut.md`。製品の壊れは検証の担当が直さない）。未確認の機能は壊れとして切らず、何が起きれば確かめられるかを、オーナーに知らせるときに添える。
2. 片付ける。`<id> report`（PR が無い）は、読んで切ったらオーナーの go 無しで `andon issue finish <id>`。`<id> review`（地図の直しの PR）は、gate と dispatcher が回す（`gate.md`・`finish.md`）。
3. 片付けたら、「当てたら」の 2 で始まりの commit を決め直す。報告に見た範囲が無い（診断で止まった）ときも、2 のとおり依頼書の `前の検証が見た commit:` が始まりになる。始まりが今当たっている commit（`/opt/andon/applied.json` の `commit`）と違えば、それを当てた commit として「当てたら」の 3 から回す。動いている間に当てた差分は、ここでまとめて見る。
