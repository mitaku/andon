---
読む時: wiki に元資料を置く・ingest・query・lint をするとき（finish のあと・人の答えが推奨案と違ったとき・人が資料を持ち込んだとき・トリアージの memory の写し・課題を切る前に引くとき）
消す条件: commit と push の段は「wiki の commit と push を command がするようになったら」。層と ingest の手順は「素の版（原則と目次だけ）で ingest を流して、触るページと log の形が変わらなかったら」。移行の段は「前の形の `wiki/<日付>-<短い名>.md` が残るセルが無くなったら」
---

# wiki

セルの `/workspace/wiki/`（セルの repo）。形は LLM Wiki: 書いたら変えない元資料の層（`raw/`）と、Chief of Staff が育てる wiki の層（`sources/`・`entities/`・`topics/`・`overview.md`）を分け、`index.md`（目録）と `log.md`（時系列）を置き、ingest・query・lint で回す。
書くのは Chief of Staff だけで、Chief Engineer と担当は読むだけ。人は wiki のページを書かない（元資料を持ち込む・問う・読んで直させる）。手順はこのファイルが正（command は無い）。
セルに固有の慣習（そのセルで増えた実体の名・書き方の癖）はセルの `AGENTS.local.md` に書く。wiki の中に規約のファイルは置かない。

書き手の線（規約8）: 案件の合意事項と用語は Chief Engineer、Chief of Staff の設定と wiki は Chief of Staff。大きな再編や文書を作る仕事は課題に出す。
記憶の規律（規約9）: memory（auto memory。`/workspace/memory/`）は蒸留までの受け皿。残すものはトリアージを通して wiki と repo へ（`triage.md`）。残す決定は担当に PR でリポジトリの文書へ書かせる。プロジェクトの設定は `projects/<slug>/AGENTS.md` の Chief of Staff の節。セルを横断する運用の学びはセルの wiki へ（書くのは Chief of Staff だけ）。

- **何を**: 学び（プロジェクト固有も横断も。固有は front matter の `project: <slug>`）・確かめた事実・**オーナーの思考と志向**（決断の傾向・推奨案を覆した理由。`entities/owner.md` に集める）。
  製品に効くもの（仕様・作法・テスト）は repo へ（課題に切る）。決定は ADR と `decisions/` が正で、wiki は指すだけで中身を写さない。
- **層**:
  - `raw/` — 元資料。書いたら変えない。4種: (1) 人の発言の写し（`raw/<日付>-<短い名>.md`。言葉のまま・日付つき。要約しない）、(2) 人が持ち込む外の資料（記事・資料）、(3) finish した課題の記録の写し（`raw/<id>/report.md` と、あれば `raw/<id>/corrections.json`）、(4) トリアージで wiki 行きになった memory の写し（`raw/memory/<日付>-<短い名>.md`。言葉のまま）。指摘（findings）・ゲートの記録は写さず、要れば台帳を指す。
  - `sources/` — 元資料に1対1の要約（`raw/` と同じ名）。front matter に `date`・`source`（`raw/` のパス）。
  - `entities/` — 実体（役・部品・案件・オーナー）ごとに1ページ。例 `entities/owner.md`・`entities/dispatcher.md`。
  - `topics/` — 概念・運用の題ごとに1ページ（比べたもの・まとめもここ）。例 `topics/review-after-rebase.md`（rebase のあとのレビューの頼み直し）。
  - `overview.md` — wiki 全体のまとめ1枚。
  - `index.md` — 目録。種類ごとの節（Sources・Entities・Topics）に、ページごとに1行（リンク・1行の要約）。query は最初にこれを読む。
  - `log.md` — 追記だけの時系列。ingest・query・lint の1件に1項で、先頭を `## [YYYY-MM-DD] ingest | <題>`（`query`・`lint` も同じ形）に揃える。`grep '^## \[' wiki/log.md` で引ける。
- **元資料を置くとき**（置いたらその場で ingest）:
  - 書く前に秘密（token・鍵・パスワード）と、人が出さないでと言った個人の情報を `<伏せた: 種類>` に置き換える（「言葉のまま」の唯一の例外）。伏せたかを確かめられない資料は写さず、台帳か元の場所を指すだけにする。
  - 人の発言: 人の答えが推奨案と違ったとき・志向や理由が見えたとき（`blocked.md` の「人に聞くこと」）・人が「残して」と言ったとき。決断そのものは問いの答えと ADR に残るので写さない。
  - 外の資料: 人が資料を置いて「取り込んで」と言ったとき。
  - 課題の記録: `andon issue finish` のあと（dispatcher の `merged` の片付けも）、台帳の `issues/<id>/report.md` と `issues/<id>/library/corrections.json`（あれば）を `raw/<id>/` に写す。調べる課題・トリアージも同じ。
  - memory: トリアージの `apply` のあと、`close`「wiki へ」になった memory の本文を、記録から `raw/memory/` に写す（`triage.md` の 5）。memory の置き場（`/workspace/memory/`）からは直接写さない（`apply` が消す）。
- **いつ（ingest）**: 元資料を `raw/` に置いたその場で、1件ずつ（溜めない）。
  1. `sources/` に要約のページを書く（全部は写さない。次に効くもの・確かめた事実・志向）。
  2. 関わる `entities/`・`topics/` のページを直す（無ければ作る）。主張ごとに出典（`sources/` のページ）へリンクする。
  3. `index.md` に足し、直したページの1行も直す。全体の見え方が変われば `overview.md` も直す。
  4. `log.md` の末尾に `## [YYYY-MM-DD] ingest | <題>` と、触ったページを1行。
- **query**: 引く口は3つ — Chief Engineer の「読む」・課題を切るとき（`cut.md`）・人が Chief of Staff に聞いたとき。`index.md` → ページの順で読み、出典（`sources/` のページか `raw/` の元資料）つきで答える。
  人に答えたもののうち次も使えるものは `topics/` のページにして（戻すかは自分の判断）、`index.md` に足し、`log.md` に `## [YYYY-MM-DD] query | <問い>` を足す。
- **lint**: ingest 約10件ごと（`log.md` の前の `lint` の項から `ingest` の項を数える）と、人が頼んだとき。定期の課題は作らない。
  見るもの: ページ間の矛盾・新しい元資料に覆された古い主張・孤立ページ（入るリンクが無い）・名前は出るのにページが無い重要な概念・リンク漏れ。
  矛盾は古いページを上書きせず、末尾に `→ 覆された（<新しいページ>）` を1行足す。終わったら `log.md` に `## [YYYY-MM-DD] lint | <見つけた数と直した数>`。
- **移行**（前の形の `wiki/<日付>-<短い名>.md` が残るセルで1回）: 無いフォルダと `log.md` を足す（形は `andon init` の雛形と同じ）。
  前のページの人の言葉の部分を `raw/` に発言の写しとして置き直し、上の ingest をし直す（`entities/owner.md` と `topics/` ができる）。repo に置くべき手順は直す課題に切り、wiki のページは skill・ADR を指すだけにする。前のページは消す（git に残る）。
- **commit・push**: `git -C /workspace add wiki && git -C /workspace commit --only -m "wiki: <要点>" -- wiki && git -C /workspace push`（`--only -- wiki` で、wiki の外が staged でも commit に入れない）。
