---
読む時: 片付けるとき（`merged`・`closed` の通知・調べる課題・やめる課題）と、マージのあとの影響（`<id> impact`）を受けたとき
消す条件: 届く行ごとの表は「dispatcher がオーナーのマージも閉じた PR も片付け、届く行が打つべき1手を持つようになったら」。影響の表は「`andon issue impact` が課題ごとに判断（影響なし・rebase・止めて確かめる）の推奨まで出すようになったら」
---

# 片付け

- dispatcher がマージしたもの（`<id> merged: <PR> をマージして片付けた`）は、dispatcher が `andon issue finish` 済み。
- オーナーがマージしたもの（`review` の課題も、担当が働いている `doing` の課題も）も dispatcher が見回りで拾う（PR ごとに1分に1回）。届く行で決める:

  | 届く行 | dispatcher がしたこと | すること |
  |---|---|---|
  | `<id> merged: <PR> はオーナーがマージした（…）。gate の問いを閉じて片付けた` | gate の問い（`from: gate`）を「オーナーがマージした」と閉じ、`andon issue finish` 済み | 何もしない（wiki だけ。下） |
  | `<id> merged: <PR> はオーナーがマージした（…）が、片付けていない: PR が既定の branch でない base（<branch>）にマージされた。…` | 片付けていない。オーナーが積んだ PR を前の課題の branch へマージしたので、変更は main に入っておらず、課題は `done` にならない（ADR-0067 決めたこと 15） | 前の課題の PR にまとめて main に入れるつもりかを、オーナーに1問で確かめる（ask-one-by-one skill）。まとめるなら、前の課題の PR が main に入って片付いたあと、積んだ課題の変更も main に入っているので、`andon issue finish <id>` で積んだ課題を手で片付ける（gate は open でない PR に載せ替えを頼まない） |
  | `<id> merged: <PR> はオーナーがマージした（…）が、片付けていない: <理由>` | 片付けていない（gate の外の問いが開いている・commit していない変更がある・finish が失敗した） | 理由の問いを先に閉じる（担当の問いなら答えを返す・要らなくなった問いなら `kanban answer --decided-by chief-of-staff`・`kanban received`）か、finish の理由を直す（commit していない変更は担当に聞くか、オーナーへ）。dispatcher が次の巡で片付ける |
  | `<id> closed: <PR> はマージされずに閉じた` | 何もしない（片付けていない） | オーナーが閉じた理由を確かめ、やめるならオーナーの go を取って `andon issue finish <id>`、出し直すなら担当へ返す |

- どちらのマージでも、finish のあと dispatcher が動いている課題への影響を見て、影響があれば `<id> impact` で届く（下の「マージのあとの影響」）。
- どちらのマージでも、片付けるとオーナー待ちか動いている課題が1つ減る。`start.md` の 1 で枠を数え直し、空いていれば起動できる課題を起動する。
- 調べる課題（`deliverable: candidates`）は PR が無いので、報告と `candidates.json` を読んで直す課題を切ったあと（切らない候補は報告の Next に残っている）、オーナーの go は要らず `andon issue finish <id>`。commit していない変更があって止まったら、担当に捨てさせる（報告の Next に「commit は捨ててよい」と書かせる）。
- dispatcher が止まっているときは、`review`・`doing` の課題なら `andon issue settle <id>` を打つ（dispatcher が打つのと同じ。マージを確かめ、gate の問いを閉じて片付ける。出力の `notify[]` が届くはずだった行）。`report.json` に `pr` が無い課題をオーナーがマージしたときだけ、`gh pr view <n> --repo <owner/repo> --json state -q .state` が `MERGED` を確かめてから `andon issue finish <id>`（レビュアーの子課題と detached worktree も片付く）。
  受け取られていない問い（`open`・`answered`。レビュアーの子課題のものも）があると、端末にも worktree にも触らずに止まる（課題は元の状態のまま）。commit していない変更があっても止まる。そのときは担当に聞くか、オーナーへ。やめる課題も、問いを閉じて（`kanban answer --decided-by chief-of-staff`・`kanban received`）から打ち直す。
- マージせずにやめる課題は、**オーナーの go を取ってから** `andon issue finish <id>`（branch は残る）。その課題の上に積んだ課題（`kanban list --json | jq '.[] | select(.stacked_on == "<id>")'`）があれば、go を取る同じ返事で「S-y（タイトル）がこの上に積んである。やめると、S-y もやめるか、この変更なしで続けるかを次に聞く」と言う（ADR-0067 決めたこと 14。次の問いは、gate の `behind` の通知が届いてから `watch.md` の表で出す）。
- レビュアーの課題は、元の課題の `andon issue finish` で片付く。元の課題をやめずにレビュアーだけ残っていれば `andon issue finish <レビュアーの id>` と detached の worktree の `git worktree remove`。
- finish のたびに、報告とオーナーの手直しを wiki の `raw/` に写して ingest する（`wiki.md`）。
- 太らせて寝かせない（規約4）: MERGED を確かめたら `andon issue finish`。
- 復旧は git・テスト・報告が正（規約5）。報告は kanban の `issues/<id>/report.md`。
- 残りの見つけ方（規約4・6 の診断）: `kanban list`（`doing`・`review` のまま古いもの）、`hc ls --label project=<slug>`、`git worktree list`、`kanban lint`。

## マージのあとの影響（`<id> impact`）

1本のマージで、同じ repo で動いている担当（`doing`・`review`）の前提が変わる。事実は dispatcher が `andon issue impact` で集めた（`<news>` の行は要約。全部は `andon issue impact <マージした課題の id>` を打って JSON で読む。読むだけ）。
**判断と担当への依頼は自分のもの**（委譲の外の決断だけオーナーへ）。担当の代わりに rebase しない・dispatcher に頼まない。課題ごとに上から見て、最初に当たった行で決める:

| 事実（`issues[]` の1件） | 判断 | すること |
|---|---|---|
| 通知そのものが `<id> impact: … の影響を集められなかった（…）`（dispatcher が3回打って諦めた） | 確かめる | `andon issue impact <id>` を手で打って JSON を読み、下の行で決める。また落ちれば理由（gh・fetch）をオーナーへ知らせる |
| `error`（branch・worktree が読めない） | 確かめる | `kanban show <id>`・`git worktree list` で branch を確かめる（規約6）。直せなければオーナーへ知らせる |
| `stacked_on` が、マージした課題そのもの（前の課題の PR の上に積んだ課題） | 影響なし（gate が載せ替えを頼む） | 何もしない。依頼書が前の課題のファイルを名指していても（`brief_refs`）、前の課題の変更は積んだときにもう読んでいるので、手を止めさせない。dispatcher が gate を回し直し、gate が担当へ `andon issue restack` での載せ替えを頼む（ADR-0067 決めたこと 11・12） |
| `stacked_on` があり、マージしたのがほかの課題 | rebase を頼まない | `conflict`・`overlap`・`checks_moved` があっても rebase を頼まない（前の課題が main に入ったときの載せ替えで、まとめて取り込む）。`brief_refs` があれば、下の「前提」の行と同じに一度止めて向きを確かめる（rebase は頼まない）。無ければ何もしない |
| マージした PR が、その課題の**前提**を変えた: `brief_refs` があるか、PR が ADR・設計書・約束（contract・schema）・skill を変えていて、依頼書の「なぜ」「入力」「完了条件」に関わる（PR と依頼書を読んで決める） | **一度止めて向きを確かめる** | 担当へ `hc send <agent> "<マージした id>（タイトル）が入り <PR> で <ファイル> が変わった。次の commit の前に手を止めて読み、依頼書の追記を待って"`。向きが変わらなければ「変わらない。そのまま続けて」、変われば依頼書に `## 追記（<日付>）` を足して「追記した。読んでから続けて」（`conflict` もあれば rebase も一緒に頼む）。向きを決めるのが決断（4種・委譲の外）ならオーナーへ1問（ask-one-by-one skill）。担当は止めたまま |
| `conflict: true` | **rebase を頼む** | 担当へ `hc send <agent> "<マージした id>（タイトル）が main に入り、<conflict_files> でぶつかる。origin の main に rebase して衝突を解き、検査を通して push。review にいたら kanban start で戻してから。Review: on か、差分がレベル B・C ならレビューを頼み直す"` |
| `checks_moved` があって、`review` にいる | **rebase を頼む** | 担当へ `hc send <agent> "<マージした id>（タイトル）が main に入り、検査の置き場 <checks_moved> が変わった。origin の main に rebase して新しい検査で checks を通し直し、push。kanban start で戻してから。Review: on か、差分がレベル B・C ならレビューを頼み直す"`。gate が通したあとでも頼む（古い base の checks のままマージされると main が赤になる） |
| `overlap` だけ（衝突しない）で、`review` にいる | **rebase を頼む** | 同じ文（「ぶつからないが <overlap> が重なる。rebase して検査を通し直す」）。マージの前に新しい main で検査を通すため |
| `overlap` だけで、`doing`（`checks_moved` があっても） | 影響なし（いまは） | 何もしない。担当は PR を出す前に main を取り直す（同じファイルの次のマージでまた届く。検査の置き場が古い base のまま ready すると gate が返す） |
| 上のどれでもない | 影響なし | 何もしない |

- 決めたことは、オーナーがいるターンならその返事で知らせる（`S-0011（…）に rebase を頼んだ。S-0012（…）は止めて、向きを確かめている。`）。影響なしだけなら `andon news ack` を打って文は書かない。
- **rebase を頼むときは、`Review: on` の課題と、`Review: off` で差分がレベル B・C の課題（gate がレビューを求める）には「review-request を打ち直して ready」まで書く**。rebase で差分が変わると、最後のレビューが見た commit と食い違い、`andon issue ready` は stale で止まる。衝突の無い rebase で差分が同じなら、review-request は前のレビューを引き継いで頼まず、担当は報告を書き直して ready へ進む。回数が `review_rounds` を超えるなら、頼む前に `kanban set <id> review_rounds=<数>` で上げておく。
- 同じ担当に同じマージで二度頼まない。rebase を頼んだ担当が `blocked`・答えないときは `blocked.md`・`watch.md` の表で扱う。
- **ほかの「前提が変わった」も同じ口で担当へ届ける**: 動いている課題の依頼書に追記したら、そのターンで担当へ `hc send <agent> "依頼書に追記した（<要点>）。読んでから続けて"`（kanban の依頼書は担当の画面に届かない）。
  オーナーの答え（`blocked.md` の「オーナーに聞くこと」）が、問うた課題のほかに動いている課題の前提にも触れるなら、その担当にも同じく知らせる。
