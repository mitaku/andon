---
読む時: 片付けるとき（`merged`・`closed` の通知・調べる課題・やめる課題）と、マージのあとの影響（`<id> impact`）を受けたとき
消す条件: 届く行ごとの表は「dispatcher が人のマージも閉じた PR も片付け、届く行が打つべき1手を持つようになったら」。影響の表は「`andon issue impact` が課題ごとに判断（影響なし・rebase・止めて確かめる）の推奨まで出すようになったら」
---

# 片付け

- dispatcher がマージしたもの（`<id> merged: <PR> をマージして片付けた`）は、dispatcher が `andon issue finish` 済み。
- 人がマージしたもの（`review` の課題）も dispatcher が見回りで拾う（PR ごとに1分に1回）。届く行で決める:

  | 届く行 | dispatcher がしたこと | すること |
  |---|---|---|
  | `<id> merged: <PR> は人がマージした（…）。ゲートの問いを閉じて片付けた` | ゲートの問い（`from: gate`）を「人がマージした」と閉じ、`andon issue finish` 済み | 何もしない（wiki だけ。下） |
  | `<id> merged: <PR> は人がマージした（…）が、片付けていない: <理由>` | 片付けていない（ゲートの外の問いが開いている・finish が失敗した） | 理由の問いを先に閉じる（担当の問いなら答えを返す・要らなくなった問いなら `kanban answer`・`kanban received`）か、finish の理由を直す。dispatcher が次の巡で片付ける |
  | `<id> closed: <PR> はマージされずに閉じた` | 何もしない（片付けていない） | 人が閉じた理由を確かめ、やめるなら人の go を取って `andon issue finish <id>`、出し直すなら担当へ返す |

- どちらのマージでも、finish のあと dispatcher が動いている課題への影響を見て、影響があれば `<id> impact` で届く（下の「マージのあとの影響」）。
- 調べる課題（`deliverable: candidates`）は PR が無いので、報告と `candidates.json` を読んで直す課題を切ったあと（切らない候補は報告の Next に残っている）、人の go は要らず `andon issue finish <id>`。commit していない変更があって止まったら、担当に捨てさせる（報告の Next に「commit は捨ててよい」と書かせる）。
- dispatcher が止まっているときは、`review` の課題なら `andon issue settle <id>` を打つ（dispatcher が打つのと同じ。マージを確かめ、ゲートの問いを閉じて片付ける。出力の `notify[]` が届くはずだった行）。`review` を通らなかった課題を人がマージしたときだけ、`gh pr view <n> --repo <owner/repo> --json state -q .state` が `MERGED` を確かめてから `andon issue finish <id>`（レビュアーの子課題と detached worktree も片付く）。
  開いた問いがあると止まる。commit していない変更があっても止まる。そのときは担当に聞くか、人へ。
- マージせずにやめる課題は、**人の go を取ってから** `andon issue finish <id>`（branch は残る）。
- レビュアーの課題は、元の課題の `andon issue finish` で片付く。元の課題をやめずにレビュアーだけ残っていれば `andon issue finish <レビュアーの id>` と detached の worktree の `git worktree remove`。
- finish のたびに、報告と人の手直しを wiki の `raw/` に写して ingest する（`wiki.md`）。
- 太らせて寝かせない（規約4）: MERGED を確かめたら `andon issue finish`。
- 復旧は git・テスト・報告が正（規約5）。報告は台帳の `issues/<id>/report.md`。
- 残りの見つけ方（規約4・6 の診断）: `kanban list`（`doing`・`review` のまま古いもの）、`hc ls --label project=<slug>`、`git worktree list`、`kanban lint`。

## マージのあとの影響（`<id> impact`）

1本のマージで、同じ repo で動いている担当（`doing`・`review`）の前提が変わる。事実は dispatcher が `andon issue impact` で集めた（`<news>` の行は要約。全部は `andon issue impact <マージした課題の id>` を打って JSON で読む。読むだけ）。
**判断と担当への依頼は自分のもの**（委譲の外の決断だけ人へ）。担当の代わりに rebase しない・dispatcher に頼まない。課題ごとに上から見て、最初に当たった行で決める:

| 事実（`issues[]` の1件） | 判断 | すること |
|---|---|---|
| 通知そのものが `<id> impact: … の影響を集められなかった（…）`（dispatcher が3回打って諦めた） | 確かめる | `andon issue impact <id>` を手で打って JSON を読み、下の行で決める。また落ちれば理由（gh・fetch）を人へ知らせる |
| `error`（branch・worktree が読めない） | 確かめる | `kanban show <id>`・`git worktree list` で branch を確かめる（規約6）。直せなければ人へ知らせる |
| マージした PR が、その課題の**前提**を変えた: `brief_refs` があるか、PR が ADR・設計書・約束（contract・schema）・skill を変えていて、依頼書の「なぜ」「入力」「完了条件」に関わる（PR と依頼書を読んで決める） | **一度止めて向きを確かめる** | 担当へ `hc send <agent> "<マージした id>（題）が入り <PR> で <ファイル> が変わった。次の commit の前に手を止めて読み、依頼書の追記を待って"`。向きが変わらなければ「変わらない。そのまま続けて」、変われば依頼書に `## 追記（<日付>）` を足して「追記した。読んでから続けて」（`conflict` もあれば rebase も一緒に頼む）。向きを決めるのが決断（4種・委譲の外）なら人へ1問（ask-one-by-one skill）。担当は止めたまま |
| `conflict: true` | **rebase を頼む** | 担当へ `hc send <agent> "<マージした id>（題）が main に入り、<conflict_files> でぶつかる。origin の main に rebase して衝突を解き、検査を通して push。review にいたら kanban start で戻してから。Review: on ならレビューを頼み直す"` |
| `overlap` だけ（衝突しない）で、`review` にいる | **rebase を頼む** | 同じ文（「ぶつからないが <overlap> が重なる。rebase して検査を通し直す」）。マージの前に新しい main で検査を通すため |
| `overlap` だけで、`doing` | 影響なし（いまは） | 何もしない。担当は PR を出す前に main を取り直す（同じファイルの次のマージでまた届く） |
| 上のどれでもない | 影響なし | 何もしない |

- 決めたことは、人がいるターンならその返事で知らせる（`S-0011（…）に rebase を頼んだ・S-0012（…）は止めて向きを確かめている`）。影響なしだけなら `andon news ack` を打って文は書かない。
- **rebase を頼むときは、`Review: on` の課題には「review-request を打ち直して ready」まで書く**。rebase で head が変わると、最後のレビューが見た commit と食い違い、`andon issue ready` は stale で止まる。回数が `review_rounds` を超えるなら、頼む前に `kanban set <id> review_rounds=<数>` で上げておく。
- 同じ担当に同じマージで二度頼まない。rebase を頼んだ担当が `blocked`・答えないときは `blocked.md`・`watch.md` の表で扱う。
- **ほかの「前提が変わった」も同じ口で担当へ届ける**: 動いている課題の依頼書に追記したら、そのターンで担当へ `hc send <agent> "依頼書に追記した（<要点>）。読んでから続けて"`（台帳の依頼書は担当の画面に届かない）。
  人の答え（`blocked.md` の「人に聞くこと」）が、問うた課題のほかに動いている課題の前提にも触れるなら、その担当にも同じく知らせる。
