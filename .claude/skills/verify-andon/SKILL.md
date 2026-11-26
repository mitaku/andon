---
name: verify-andon
description: Andon の repo の検証の skill。マージした変更をセルへ当てたあと（image/apply-main.sh apply）、当てた差分に当たる機能（担当の起動・報告と ready・gate・dispatcher の通知・hook・boot・ダイジェスト・流れの数え・プロジェクトを立てる・オーナー待ち・PR を止める・トリアージ・振り返り）を、セルに入っている本物の andon・`kanban`・`/opt/andon` で動かし、走らせた記録を証拠に残して片付ける手順と機能の地図を持つ。検証の課題の依頼書がこの SKILL.md のパスを名指したときだけ読む
disable-model-invocation: true
---

# verify-andon

**あなたは検証の担当。** Chief of Staff がマージ済みの変更をセルへ当てたあと（`image/apply-main.sh apply`）に切った検証の課題を受け持つ。当てた差分に当たる機能を、セルの本物で利用者と同じ道で動かし、走らせた記録を証拠に残し、跡を片付ける。担当の決まり（worker skill）はそのまま効く。この skill はその上に、検証の課題の手順を足す。

| する | しない |
|---|---|
| 当てた差分に当たる機能を `scripts/drive.sh` で動かし、`andon issue record` で記録に撮る | 製品のコード（`cmd/`・`internal/`・`package/`・`schema/` など）を直す。動かなかった機能は報告に書き、直す課題は Chief of Staff が切る |
| 地図と手順の食い違いを、この skill の置き場（`.claude/skills/verify-andon/`）の中だけで直し、PR 1本にする | エージェントを起動する。本物のエージェントが要る機能は、地図の「見る道」のとおり Chief of Staff かオーナーに回す |
| セルの本物の記録・端末を読む（`hc get`・`hc read`・kanban のファイル） | セルの本物の kanban に書く・ほかの担当の端末に `hc send`・`hc kill` する・`andon boot`・`andon gate`・`andon dispatcher drain` を手で打つ |

## 1. 当てたものと範囲を決める

検証の担当は当てない。当てるのは Chief of Staff で、検証の担当は当たったものを確かめる。

1. 依頼書から、前の検証が見た commit（無ければ依頼書が書く始まりの commit）を読む。
2. 検証する commit は依頼書の `当てた commit:` の行。`drive.sh` には毎回この commit を2つ目の引数で渡す（省くと使い方の誤りの終了コード 2 で断る）。検証の途中で次の commit が当たっても、検証する commit は変えない。本物の記録を読む行は、その commit が当たっていた間（次に当てた時刻まで）の出来事だけを読む（前に当てた commit と時刻は `/opt/andon/applied.json` の `previous` にある）。切り離して動かす道と今の状態を読む行（dispatcher の端末・boot の生成物）は、今当たっているパッケージを見る。検証の課題の外で今当たっているものを読むときは、`andon version` の `commit`（`scripts/doctor.sh` の最後の行にも出る）を渡す。
3. 自分の worktree で `git fetch origin` し、`bash .claude/skills/verify-andon/scripts/features.sh <前の commit>..<当てた commit>` を打つ。出た名前が、この課題で動かす機能。何も出なければ、動かす機能は無い（報告にそう書いて終える）。
4. 依頼書が機能を名指していれば、それも動かす。
5. 名指した細かい機能を集める。範囲の commit の本文の `Verify-features:` の行（`git log --format=%B <前の commit>..<当てた commit> | sed -n 's/^Verify-features: *//p'`）の名前と、依頼書が名指した細かい機能を合わせる。`drive.sh` の3つ目の引数に、合わせた名前を「,」で区切って1つで渡す（別の機能の名前が混じっていても、`drive.sh` は数えずに捨てる）。名前が1つも無ければ3つ目の引数を省く。

自分の worktree は課題を起動した時点の main。当てた commit より新しいことがある。動かすのはセルに当たっている本物で、worktree の checkout から焼いたものではない。worktree から使うのは、この skill と `tests/lib/isolate.sh`（切り離しの土台）だけ。

## 2. 診断する

`bash .claude/skills/verify-andon/scripts/doctor.sh` を打つ。読むだけで、何も書かず何も起動しない。見るのは、andon と kanban の置き場・`andon version` の commit と中身の食い違い・当てた記録・dispatcher の端末が live で当てたあとに起動したか。

- 終了コード 0 なら動かしてよい。
- 1 なら `FAIL` の行が理由。当て方が壊れている（例: 当てた commit と `andon version` の commit が違う・dispatcher が当てる前から動いている）。機能を動かさず、`FAIL` の行を報告に書いて Chief of Staff へ返す。
- 様子がおかしいと思ったら、いつでも打ち直す。

## 3. 機能を動かす

機能ごとに、地図（`features/<機能>.md`）の「動かし方」を読み、次の1行で動かして記録に撮る。

```
andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh <機能> <検証する commit>
andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh <機能> <検証する commit> <名指した細かい機能>
```

1つ目は名指した細かい機能が無いとき、2つ目はあるとき（1 の 5）。`--expect` はどちらも付けない（`pass` を期待する）。`drive.sh` が 0 以外で終わると `andon issue record` も「pass を期待したが終了コード …」で 0 以外を返すが、記録は書いてあるので打ち直さない。

`drive.sh` は1回の呼び出しの中で、一時の置き場を作り・動かし・片付け・跡が残っていないことを確かめるまでを済ませる。エージェントの Bash は呼び出しごとに環境が戻るので、手順を呼び出しに分けない（分けると、後の呼び出しがセルの本物の kanban を見る）。

`drive.sh` は最後に、地図の細かい機能ごとの結果を `結果 <細かい機能> <ok・未確認・FAIL・外で見る>（<名指し・補い>）` の1行ずつで出す。未確認の行には、その名前を含む `note` の行が添う。「外で見る」は見る道が検証の担当だけでない細かい機能で、名指しても終了コードに数えない。4 のとおり、見る道で別に証拠を取る。名指しがあれば、終了コードは名指した細かい機能だけで決まる。名指さない細かい機能の未確認は「補い」の行に出るだけで、終了コードを変えない。名指しが外で見るものか別の機能のものだけなら、決める細かい機能が無いので 0 になる（補いの未確認は終了コードを変えない）。FAIL の行が1つでもあれば、名指しによらず、道が 3 で終わっても終了コード 1 になる。名指しが無ければ、終了コードは道の終了コードのまま。

`drive.sh` の終了コードは次のとおり。`andon issue record` の終了コードとは別で、記録の `exit_code` に入る。報告の `acceptance[]` の行は、終了コードごとにこの表の1通りで書く。`run` を書くのは 0 の行だけ（期待 `pass` と終了コードが合う記録だけを `run` で指せる。worker skill §7）。

| 終了コード | 意味 | acceptance の行 | 報告に書くこと |
|---|---|---|---|
| 0 | 動いた証拠が取れた（名指しがあれば、名指した細かい機能が全部 ok） | `result: pass`・`run: <記録の番号>` | 機能の名前・記録の番号・名指した細かい機能の `結果` の行。補いの未確認があれば、その細かい機能の名前と `note` の要約も |
| 1 | 動かなかった（壊れ）か、跡が残った | `result: fail`・`run` は書かず、`evidence` に `runs/<n>`（終了コード 1）と `FAIL` の行 | `FAIL` の行・記録の番号・地図の食い違いか製品の壊れかの見立て |
| 2 | 使い方の誤り（検証する commit が無い・先頭の 7 文字以上の16進数でない・知らない機能の名前・地図に無い細かい機能の名前） | 書かない | 直して打ち直す。報告には書かない |
| 3 | 前提が無く確かめられなかった（未確認。名指しがあれば、名指した細かい機能のどれかが未確認） | `result: manual`・`run` は書かず、`evidence` に `runs/<n>`（終了コード 3）・未確認の細かい機能・何が起きれば確かめられるか | 未確認の細かい機能の `結果` の行と `note`、何が起きれば確かめられるか。動いたとは書かない |

未確認は待って解消しない。当てた直後で本物の出来事がまだ無いのは前提が無いということで、課題の中で打ち直しを待たずに 3 の行で書いて返す（次の検証の課題が、その commit の間をまた読むことはない）。

動かし方の形は2つある（地図の `README.md` の「動かし方の2つの形」）。切り離して動かす機能は一時の kanban と偽の `hc`・`gh` で本物の andon・`kanban`・`/opt/andon` を動かし、本物の記録を読む機能は、検証する commit が当たっていた間（当てた時刻から次に当てた時刻まで。今も当たっていれば今まで）にセルで本当に起きた出来事を読む。

`drive.sh` が終了コード 0 以外で終わったら、地図の「落とし穴」を読む。`drive.sh` を書き換えて通すことはしない（下の 6）。

## 4. 証拠を残す

- 証拠は `andon issue record` の記録（`library/runs/<n>.json` と `<n>.log`）。`<n>.log` に `drive.sh` の出力そのもの（打った command・終了コード・出力・結果の状態・片付けの確かめ）が入る。
- 記録は commit していない変更の無い worktree で撮る（`dirty` が false）。地図を直したら、commit してから撮り直す。
- 報告の `acceptance[]` に、機能ごとに1行を書き、`run` に記録の番号を入れる。依頼書が受け入れ条件の番号を名指していれば、その `id` で書く。
- 地図の「見る道」が「Chief of Staff が試しの課題を起動し、検証の担当が画面を読む」の細かい機能は、`hc read <address>` で読んだ行を `evidence` に写す。「オーナーが見る」の細かい機能は `manual` と書き、見る場所を `evidence` に書く。
- 証拠の基準: 利用者と同じ道を通す（内部の関数や検査のための口を使わない）。操作と結果の両方を残す。kanban に書く機能は、書いたあとに別の口で読み直す。mock（偽の `hc`・`gh`）は、本物の端末と GitHub に届かないための境界だけに置く。

## 5. 片付ける

- `drive.sh` は、自分が作った一時の置き場（`$TMPDIR` の下の `verify-<機能>-<時刻>-<pid>.*`）だけを消す。最後に、一時の置き場が無く、セルの本物の kanban（`/workspace/projects/*/kanban`）と `hc ls` に跡の印（`verify-<機能>-…`）が無いことを確かめて `ok   片付けた` を出す。
- 記録（`library/runs/`）は片付けで消えない。証拠はそこに残る。片付けたあとに `ls <自分の課題の library>/runs/` で記録があることを確かめる。
- 名前で端末やプロセスを止めない（`pkill -f`・`hc kill <名前>`）。止めるのは自分が起動したものだけで、`drive.sh` は端末を起動しない。
- `drive.sh` が途中で殺されて一時の置き場が残ったら、`ls -d ${TMPDIR:-/tmp}/verify-*` で見つけ、自分が作ったもの（印の時刻と pid）だけを消す。

## 6. 地図と手順を直す

動かして見つけた食い違い（地図の command が無い・見える証拠の書き方が違う・変えたら動かすパスが足りない）は、この課題の PR 1本で直す。

- 直してよいのは `.claude/skills/verify-andon/` の中だけ。製品のコードが壊れていると見たら直さず、報告の `## Next` に `to: chief-of-staff` で1行書く。
- 直し方は地図に合わせる。動かない道を弱めて通さない（前提が無いなら、地図の「落とし穴」に書き、終了コード 3 にする）。
- 機能を足すときは、`features/<機能>.md`（形は `features/README.md` の「機能のファイルの形」）・`features/README.md` の表の行・`scripts/journeys/<機能>.sh` の3つを同じ PR で足す。
- 直したら `bash tests/verify-andon.sh` を通す。目次とファイルの対応・節の形・手順が名指す command の実在・`features.sh` の振る舞いを見る。
- PR は worker skill §3 のとおり。`.claude/skills/` は gate の条件表で自動でマージできる側。

## 7. 報告する

worker skill §7 の書式で書く。`## Report` の先頭に次の3つを置く。

- 見た commit の範囲（`<前の commit>..<当てた commit>`）と、`features.sh` が出した機能の名前。
- 機能ごとの結果（動いた・動かなかった・未確認）と記録の番号。
- 動かなかった機能の `FAIL` の行と見立て。直す課題を切るのは Chief of Staff なので、`## Next` に `to: chief-of-staff` で1行ずつ。

次の検証の課題は、この報告の「見た commit の範囲」の右端から始まる。

## 8. 補助のスクリプト

どれも `.claude/skills/verify-andon/scripts/` にあり、repo の根から打つ。

| スクリプト | 打ち方 | すること |
|---|---|---|
| `features.sh` | `bash .claude/skills/verify-andon/scripts/features.sh <前>..<後>` | 差分が当たる機能の名前を1行1つで出す。当たらなければ何も出さず 0。地図の「変えたら動かすパス」が正で、Go のファイルはパスの package と、それが推移的に import する package まで当たる（`scripts/gopkgs.go`） |
| `doctor.sh` | `bash .claude/skills/verify-andon/scripts/doctor.sh` | 読むだけの診断。最後の行に `commit <当てた commit>` |
| `drive.sh` | `bash .claude/skills/verify-andon/scripts/drive.sh <機能> <検証する commit>` | 1機能を動かし、片付けと跡の確かめまで。2つ目の引数は省けず、本物の記録を読む道が読む間を決める。3つ目の引数（名指した細かい機能）は省ける。最後に細かい機能ごとの結果を出す。終了コードは 3 の表 |
| `journeys/<機能>.sh` | `drive.sh` が呼ぶ。直に打たない | 機能ごとの道。`sandbox.sh`（切り離して動かす）か `cell.sh`（本物の記録を読む）を読む |
| `sandbox.sh`・`cell.sh`・`common.sh` | `journeys/*.sh` が読む | 一時の置き場・偽の `hc` と `gh`・証拠を出す関数 |
