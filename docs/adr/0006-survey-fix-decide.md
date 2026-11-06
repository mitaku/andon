# ADR-0006: 調べる・決める・直すを分ける（タスクの型・候補の一覧・範囲が食い違ったときの止まり方）（仕様・設計。2026-11-06。タスク Y-158）

- 状態: **提案（2026-11-06）**。人が PR をマージした日が決定の日（[decisions.md](../decisions.md) §14 の行の日付）。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 依頼が「何を変えるか」まで決まっていないとき、ラインがどう回るか。(a) 調べるだけのタスクの型（依頼書・中間成果物の形と名前・完了条件・レビューの要否）、(b) 直すタスクの担当が、依頼書の範囲と見つけたものが食い違ったときにどう止まり、誰が決めるか、(c) Chief of Staff がその判断を自分で下してよい線と、人に回す線、(d) 依頼を受けたときの切り方（調べる／直す／Chief Engineer）、(e) 既存の決定との関係と変える所。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。この文書が足す約束（候補の一覧・問いの種類・台帳の予約キー）は「決めたこと」に形を書き、実装のタスクが `schema/` と contract.md に写す（それまでは提案）。
- 入力: 依頼書（人の問いと Chief of Staff の推し。2026-11-06）、[ADR-0004](0004-project-layout.md) 決めたこと 2（中間成果物の置き場。この文書は置き場を足さず、型と名前だけ決める）、andon main の設計と skill とコード（出典）。P7 で流した依頼の記録は建屋にあり、この担当は読めなかった（仮設の台帳に残る依頼書から、A-7 の題が「docs の誤字を1つ直す」だったことだけ分かる）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「docs の誤字を1つ直して」と頼んで担当が3つ見つけたら、どうするのが正しいソフトウェア開発のプロセスか。担当が黙って1つに絞るのも、3つとも直すのも違う。中間成果物があり、判断を挟むべき。

言い直し。今のラインの工程は2種類しかない: 人と Chief Engineer が**決める**（文書の PR。design.md §1.8）と、担当が**直す**（実装の PR。worker skill）。「**調べる**」が工程として無い。だから依頼書の前提（誤字は1つある）が現地で崩れたとき、担当には「自分で範囲を決める（1つに絞る・3つ直す）」か「人に聞く」しか無く、後者は灯りがつく判断の4種（design.md §1.3）のどれでもないので ask-one-by-one skill が間引く。困りごとは「範囲を決める判断に席が無い」こと: 担当は自工程の品質は持つが範囲は依頼書のもの、人が見るのは4種とレベル C だけ、Chief of Staff が持つと書いてあるのは許可の確認と依頼書の Settings だけ。足りないのは3つ——(1) 範囲が決まっていない依頼を回す工程（調べる）、(2) 範囲が崩れたときに担当が止まる口と、そこで Chief of Staff が決める根拠、(3) 見つけたが今は直さないものを、捨てずに・勝手に広げずに残す形。

## 前提（番号。人の問いと依頼書から取ったもの。同意していない行は「推し」と記す）

1. **人**: 担当は黙って絞らない・勝手に広げない。範囲を決めるのは担当ではない。
2. **人**: 中間成果物があり、そこに判断を挟む。
3. **[ADR-0004](0004-project-layout.md) 決めたこと 2**: 調べものの中間成果物の置き場はそのタスクの `kanban/tasks/<id>/library/`。タスクをまたぐまとめは `kanban/library/`。決める前の案（設計の代案）は Chief Engineer の文書。新しい置き場は作らない。
4. **design.md §1.3・§1.10**: 人に灯りをつける判断は5種だけで、「範囲をどうするか」はそのどれでもない。だから範囲の判断は人を呼ばず、Chief of Staff が持つ。候補の中に4種・レベル C・委譲の外に当たるものがあれば、その分だけ人へ（ask-one-by-one skill §1 の線のまま）。
5. **contract.md §0**: 機械は Markdown を読まない。担当は `library/` の JSON を手で書かず、command が形を確かめて書く。約束は小さく、テストで固定する。
6. **薄いハーネス（design.md §1.9 の 5）**: 新しい役を足さない。調べる・止まる・決めるは今の役（担当・Chief of Staff・dispatcher）と今の口（台帳の問い・報告・`library/`）で回す。
7. **Chief of Staff の推し（入力。覆してよい）**: 何を変えるかが決まっていない依頼は調べるタスクと直すタスクに分ける／直すタスクは範囲が食い違ったら直す前に止まる／設計の判断は Chief Engineer へ／決まっていて小さいものは1つのタスクのまま。→ 決めたこと 1・3・6 で、「分ける」を「調べること自体が仕事のときだけ分け、小さい依頼は直すタスクの止まり方で足りる」に絞った（理由は決めたこと 6）。ほかはそのまま。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-06 タスクの型は3つ: 調べる・直す・決める。型は依頼書の完了条件と、`task.yaml` の予約キー `deliverable`（7）で分かる。**

   | 型 | 成果（完了条件） | 誰が起こす・プロファイル | 独立レビュー | ゲート | 片付け |
   |---|---|---|---|---|---|
   | **調べる**（survey） | **候補の一覧** `library/candidates.json`（候補が 0 でも書く）と報告。PR は出さない。push もしない | Chief of Staff が `andon task start`。既定 `claude`（深い調べものは `fable`、grep で済む小さいものは `sonnet`） | **無し**（レビュアーは commit の差分を読む役で、候補の一覧には差分が無い。大きい調べもので反論が要れば Chief Engineer の題にし、その 6「反論」で） | **回さない**（`deliverable: candidates` を dispatcher が読む。7） | 候補を読んで切ったあと Chief of Staff が `andon task finish`（PR の無いタスク。トリアージと同じ） |
   | **直す**（fix） | PR（今までどおり。worker skill） | Chief of Staff。今までどおり | 今までどおり（Settings） | 今までどおり | 今までどおり |
   | **決める**（Chief Engineer） | 文書の PR（design.md §1.8） | 人か Chief of Staff が `andon chief-engineer` | 無し（検収は人が対話の中で） | docs のレベル | 今までどおり |

   - 調べるタスクの**依頼書の形**（Chief of Staff が書く。雛形は chief-of-staff skill §2 に足す）:

     ```
     # <id> 調べる: <問い>
     問い: <1行。何を知りたいか・何を見つけるか>
     範囲: <どこを見るか（repo・ディレクトリ・期間）と、見ない所>
     なぜ: <1行。人の依頼か文書のどこから来たか>
     完了条件: library/candidates.json（候補 0 でも）と報告（## Report に分かったこと・計ったこと、## Next に切る案）。PR は出さない。
     守ること: 直さない（直し方は候補の text に1行）。push・PR・エージェントを起こさない。<触らない場所・同時に動くタスク>

     ## Settings
     - Deliverable: candidates ★調べる
     - Review: off
     ```

   - 調べるタスクは repo を持ってよい（worktree で読む・走らせる。commit してよいが push しない）。読むだけで足りるなら repo の無いタスクでもよい（cwd は `tasks/<id>/`。読むのは `.ghq/` の checkout。ADR-0004 決めたこと 1 の「参考 repo」と同じ）。
   - 担当の終わり方: `andon task candidates <id> -`（2）→ `andon task report <id>`（人が読む報告。`next[]` に `to: chief-of-staff`「candidates.json から切る」）→ `andon task ready <id>`（`deliverable: candidates` なら PR の代わりに `candidates.json` が在って形が合うことを確かめて `kanban review`。7）。dispatcher は `review` の通知を届け、ゲートは回さない。Chief of Staff は候補を読んで決めたこと 5 のとおりに切り、`andon task finish`。
   - 調べるタスクの中で見つけた「直し方が1つで小さいもの」も直さない（型を守る。混ぜると「調べたついでに直した」が PR になり、範囲の判断を飛ばす）。

   理由: 調べる工程を**担当の仕事**にすると、人がいなくても進み、worktree・報告・`library/`・finish が今の道具で済む。Chief Engineer にしないのは、Chief Engineer は人と話す役（事実を集めるのは Chief Engineer 自身の 1「読む」でも副エージェントにやらせる）で、調べるタスクはその副エージェントを台帳のタスクに格上げしたもの。「型」を `deliverable` で機械に見せるのは、PR の無い `review` をゲートが `fix`（「report.json に pr が無い」）にしてしまう今の継ぎ目（`internal/gate/gate.go`・`internal/dispatcher/dispatcher.go`。repo の無いタスクだけ飛ばしている）を、暗黙の「repo が無ければ」ではなく明示の値で越えるため。
   捨てた案: 調べるを Chief Engineer の仕事にする（上）／調べるタスクを repo の無いタスクに限る（コードの調べものは worktree で走らせたい。読むだけなら repo 無しでよい、は残した）／報告の節だけで済ます（機械が読める形が無く、切る・トリアージが Markdown を読むことになる。contract.md §0 原則 1）／調べると直すを常に1つのタスクで（小さい依頼ではそれでよい。決めたこと 3・6）。

2. **2026-11-06 中間成果物の名前と形: 候補の一覧 `kanban/tasks/<id>/library/candidates.json`。担当が `andon task candidates <id> <json|->` で書く（command が形を確かめる。手で書かない）。schema は `schema/candidates.schema.json`（`$id` は `https://andon.example.com/schema/candidates/1`）。**

   ```json
   {"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
    "candidates": [
      {"id": "c1", "text": "docs/design.md:42 「建家」→「建屋」", "file": "docs/design.md", "line": 42, "in_scope": true},
      {"id": "c2", "text": "docs/operate.md:17 「起動する」が2回", "file": "docs/operate.md", "line": 17, "in_scope": false},
      {"id": "c3", "text": "README の英語の段に古い command 名。直し方は語の置換だけでなく段の書き直し", "file": "README.md", "line": 30, "in_scope": false}]}
   ```

   | キー | 要るか | 意味 |
   |---|---|---|
   | `commit` | repo のあるタスクでは必須（40桁・小文字。見た commit＝書いた時点の `HEAD`。command が書く）。repo の無いタスクでは書かない | 候補がどの状態を見て出たか。古くなった候補を見分ける |
   | `candidates[]` | 必須（空でもよい） | 候補。1件＝直すかもしれないもの1つ |
   | `candidates[].id` | 必須。`c1` … 一意 | 問いの選択肢・答え・依頼書から指す名前 |
   | `candidates[].text` | 必須。1行 | 何が・どこに・どう直すか（直し方を含む）。人と Chief of Staff が読む |
   | `candidates[].file`・`line` | 任意（`findings.json` と同じ形: repo からの相対パス・1 以上） | 場所を指せるとき |
   | `candidates[].in_scope` | 必須。真偽値 | 依頼書の範囲（やること・完了条件）の**中**か。直すタスクが止まるときの振り分けの元（3）。調べるタスクでは「問いの答えに当たるか」 |

   - キーはこれだけ（`additionalProperties: false`。将来 `tier`・`kind` を足すなら版を上げずに足せる。contract.md §0 原則 4）。書き手は担当（調べるタスク・直すタスク）。読み手: Chief of Staff（切る・答える）・人（ボードと報告から）・トリアージ（将来。やらないこと）・振り返り。機械が判定に使う値ではない（ゲートも dispatcher も読まない）が、形を固定するのは、問いの `facts` が指す先として・依頼書が `links` で指す先として、同じ形で読めるため。
   - 担当が書き直すたびに全体を書く（`report.json` と同じ）。答えが出て直したあとも消さない（経過。`finish` でも残る）。
   - 人が読む分は報告の `## Report` に同じ一覧を表で写す（1行1候補。`id`・場所・何・範囲の中か）。
   - 候補が**多い**（目安 10 を超える）ときは、調べるタスクなら全部書き、直すタスクなら「数と種類」を問いに書いて候補は代表だけ（範囲の判断は数を見れば足りる。全部直すかは別の調べるタスク）。

   理由: 中間成果物の本体は「見つけたものの一覧」で、調べるタスクの成果と、直すタスクが止まるときに返すものが**同じ形**になる（止まった直すタスクは、その場で小さな調べるタスクになっている）。1つの形にしておけば、Chief of Staff の読み方・問いの `facts`・依頼書の `links` が型をまたいで同じになる。名前「候補」（`candidates`）は「直すかもしれないが決めていない」を表す語で、レビュアーの `findings.json`（指摘。直す前提）と役が違うので別の名前にした。
   捨てた案: `findings.json` を流用（レビュアーの約束で、ゲートが読む。混ぜるとゲートが候補を指摘として数える）／`report.json` の中の配列（報告は毎回全体を書き直し、`--data` で渡し直す。候補は問いの前に置いて Chief of Staff が切るまで残る「もの」なので、ファイルとして置くほうが置き場の決定〈ADR-0004〉に合う）／自由な Markdown だけ（機械が読めない。上）／`kanban/library/`（タスクをまたぐまとめの置き場。候補はタスクのもの）。
   仮決め（人が違うと言ったら直す。4種のどれでもない）: 名前 `candidates`・キーの名前・「10 を超えたら代表だけ」の目安。

3. **2026-11-06 直すタスクの止まり方: 止まるのは「担当が範囲を決めなければ進めない」ときだけ。範囲が決まっていて、その外に見つけただけなら止まらず、直して候補に残す。**

   止まる条件（どれか1つ。依頼書の前提が現地で崩れたとき）:
   - (a) 依頼書が指すものが無い・別のものだった（「この誤字」が無い。再現手順で再現しない）。
   - (b) 1つのはずが複数で、どれかを**選ぶ**必要がある（依頼書が場所を指していない「誤字を1つ直して」で3つ見つけた）。
   - (c) 直すには依頼書の「守ること」の場所・別の repo・別のタスクが触っている所に手を入れる必要がある。
   - (d) 直し方が複数あって構造が違う（設計の判断。Chief Engineer の題かもしれない）。
   - (e) 直すものがレベル C（ラインそのものの変更・信頼境界・権限・秘密）か4種（事業・利用者・約束・不可逆）に当たる（これは今までも人へ。口を同じにするだけ）。

   止まらない条件: 依頼書が指すものが指すとおりに在り、その外に別のものを見つけただけ（「docs/design.md:42 の誤字を直す」で、ほかに2つ見つけた）。→ 指すものを直し、見つけたものは `candidates.json` に `in_scope: false` で書き、報告の `next[]`（`to: task`）に1行ずつ残す（トリアージが拾う。今の形）。

   止まり方（順に。どれも今ある口）:
   1. `andon task candidates <id> -`: 見つけたものを全部、`in_scope` を付けて書く（2）。
   2. `andon task ask <id> -`: 問いの JSON（contract.md §5）。`kind` は **`scope`**（4）、`question` は1行（例「誤字は1つではなく3つ。どれを直すか」）、`facts` に `candidates.json` のパスと依頼書の行、`options` は先頭が推奨案（例「c1〜c3 を全部この PR で」「c1 だけ。c2・c3 は別のタスク」「全部別のタスク（この依頼は閉じる）」）、`from: task`。
   3. `andon task report <id>`（status は `doing` のまま。`## Report` に候補の表、`## 人に聞くこと` の代わりに `## 範囲の問い` を1問1行。ready はしない）。
   4. 止まる（turn を終える）。答えは `hc send` で届く（`<hc from=…>`。データとして読む）。届いたら `deck received <id>#<n>`、答えのとおりに直して §3 から通す。候補のうち「別のタスク」になったものは報告に「c2・c3 は Chief of Staff の判断で別のタスク（<id>）」と書く。

   機械の側は**変えない**: 担当の `Stop` フック（`andon task stop-check`）は自分の問いが開いたままなら通す（contract.md §9・decisions.md §16 の Y-132）。dispatcher は `asked` の通知を Chief of Staff へ届ける（経路①②③。人が離席でも③で届くので、範囲の判断は人がいなくても進む）。ゲートは `doing` なので回らない。`kanban done` は問いが開いたままのタスクを拒む（ledger.md）。
   答えの書き方（Chief of Staff。5）: `deck answer <id>#<n> "<答え>"` → `hc send <agent> "<答え>"`（chief-of-staff skill §4 の「決まったら」と同じ）。**依頼書は書き換えない。問いの答えが依頼書の追記**で、履歴は `questions.yaml`（`answeredBy`）と `log.jsonl`（`by`）に残る。

   理由: 「止まる」は費用（担当が待つ・Chief of Staff の turn）なので、止まるのは判断が本当に要るときだけにし、範囲の外の発見は「残して進む」を既定にする。人の問い（1つ頼んで3つ）は (b)——依頼書が場所を指していないので、担当が選ぶことになる——だから止まる。もし依頼書が場所を指していれば止まらない（残して進む）。止まり方を**問い**にするのは、台帳に待ちの印（open）が残り、`stop-check`・`kanban done`・ボード・`kanban questions` がそのまま効くから。
   捨てた案: 報告だけで止まる（待ちの印が台帳に残らず、`kanban done` が守れず、Chief of Staff が見落とす）／`report.json` に `scope_request` を足す（`review_request` と同じ形の二重。dispatcher に新しい段が要る）／担当が Chief of Staff へ `hc send`（通知の係を迂回する。2026-11-03 の失敗〈dispatcher.md §1〉）／見つけたら必ず止まる（費用。範囲の外は残して進むで足りる）／範囲の外も黙って直す（前提 1）。

4. **2026-11-06 問いの `kind` に `scope`（範囲）を足す（6つ目）。宛先は Chief of Staff で、人を呼ばない。**
   - `schema/question.schema.json` の `kind` の enum に `scope` を足す（キーを足すのと同じく版は上げない。知らない読み手は「無い」として安全側＝人へ回す）。`andon task ask` も受ける。
   - 台帳の `kanban ask --kind scope` も受ける（`cmd/kanban` の `questionKinds`。ledger.md の「Added in version 2 (no version change)」に1行）。理由: ボードが「人の判断待ち」を数えるとき `scope` を**除く**ため（灯りではない。「Needs you」に数えない）。contract.md §5 の「5つ目は台帳が受けない」の文は今の `kanban` と合っていない（受ける）ので、実装のタスクで直す。
   - `from` は `task`（直すタスク）。調べるタスクは問いを出さない（成果が候補の一覧そのもの）。
   - ask-one-by-one skill §1 に1行足す: `scope` の問いは**まず Chief of Staff が答える**（5 の線）。候補の中に4種・レベル C・委譲の外に当たるものがあれば、その候補についてだけ人の問いにする（`kind` はその種類。`facts` に元の `scope` の問いと `candidates.json`）。`scope` のまま人へ出さない。
   捨てた案: `kind` 無しで出す（schema で必須。誰宛てか分からない）／既存の5種のどれかを借りる（例 `line`。意味が違い、人へ回る）／Chief of Staff 宛ての別の口（`report.json` の `next[{to: chief-of-staff}]`。待ちの印が残らない。3）。

5. **2026-11-06 Chief of Staff の判断点。範囲の判断は Chief of Staff のもの。委譲に新しい `scope` は足さない。**

   | 候補が | Chief of Staff が自分で決める | 人へ（1問ずつ。ask-one-by-one skill） | Chief Engineer へ |
   |---|---|---|---|
   | 場所 | `project.json` の `repos` の中（`andon task start` が確かめるのと同じ線） | 外の repo | — |
   | レベル | A・B（docs・tests・command。ゲートの条件表 `tier_*_paths` で見る） | C（`**/AGENTS.md`・`.claude/**`・workflows・条件表・image）。ただし委譲の `line-change` が `project`／`plant` なら、案件 repo の指針・観点（`project`）・建屋の設定（`plant`）まで自分で | — |
   | 種類 | 4種のどれでもない（誤字・壊れたリンク・落ちるテスト・既に決まった形への揃え） | 事業（何を作るか。委譲できない）・利用者（使い勝手の好み）・約束（外向き・公開）・不可逆（消す・本番・push 先・権限・秘密） | — |
   | 大きさ | 候補の合計が元の依頼の PR に収まる（目安: テストを除いて 500 行未満＝独立レビューの線の中）か、候補ごとに小さい直すタスクに切れる | — | — |
   | 直し方 | 1つに決まる | — | 構造の違う案が2つ以上ある・外との約束に触る・「何を作るか」に戻る（人に `andon chief-engineer "<題>"` を勧める。chief-of-staff skill の冒頭） |

   - **決め方**: 表の4列が全部「自分で」の候補だけを、推奨案どおりか、違う組み合わせで選ぶ。1つでも「人へ」の候補があれば、その候補だけ人の問いにし（ask-one-by-one skill §3 の人のタスク。`facts` に `scope` の問いと `candidates.json`）、残りは先に答えてよい（担当は答えのとおりに進み、人の分は候補のまま残す）。「Chief Engineer へ」の候補は題として人に勧め、候補のまま残す。
   - **決めた結果の書き方**（台帳の履歴）: `deck answer <id>#<n>` の文に、候補ごとの行き先を `id` で書く（例「c1・c2 はこの PR で。c3 は別のタスク S-0012（README の英語の段）。c4 は人へ（S-0013 で聞く）」）。別のタスクにしたものは `deck add` し、依頼書の「なぜ」に元のタスクの番号と候補の `id`、`links` に `candidates.json` のパス。捨てたものは答えに理由を1行（例「c5 は起きない: その repo に条件が無い」）。記録はこれで足りる: `questions.yaml`（問い・答え・`answeredBy`）、`log.jsonl`（`by`）、新しいタスクの依頼書、担当の報告。`corrections.json`（design.md §1.9 の (b)）には、人が推奨案と違う答えをしたときだけ、今までどおり dispatcher が `finish` で写す（`scope` の問いを Chief of Staff が答えたものは人の手直しではないので写さない）。
   - **調べるタスクの候補を切るとき**も同じ表で（問いは無いので答えは書かず、切ったタスクの依頼書と、調べるタスクの `finish` が記録）。切らなかった候補は捨てずに、調べるタスクの報告の `next[]`（`to: task`）に残す（トリアージが拾う）。
   - 委譲（`project.json` の `delegation`）に `scope` を**足さない**。範囲の判断そのものは Chief of Staff の持ち場（ラインを回す判断）で、人が見るのは候補が引き起こす**操作**の種類とレベル——それは既存の `merge`・`line-change`・`irreversible` と4種で決まる。

   理由: 前提 4。判断の種類は増やさず、「誰が判断するか」も委譲で変えず、Chief of Staff が今も持つ「タスクの範囲内の許可」の線を「範囲そのもの」に1段広げるだけにした。表の列は、既にある線（`repos`・レベル・4種・独立レビューの大きさ）を並べたもので、新しい基準を作っていない。
   捨てた案: 委譲に `scope` を足す（判断の種類を増やす。委譲の表は操作の種類で、範囲はそこに乗らない）／全部人へ（人の介在率が上がる。前提 4 に反する）／担当が推奨案で進めて後から戻す（前提 1。範囲の判断は「後から戻せる」の線にあっても、人が「黙って絞るのは違う」と言った）／依頼書を書き換えて答えにする（依頼書は Chief of Staff のものだが、`doing` の途中で書き換えると担当の読んだものと食い違う。問いの答えのほうが時刻と `by` が残る）。

6. **2026-11-06 依頼を受けたときの切り方。分かれ目は「依頼書の『やること』に、どのファイル・どこを・どう変えるかが書けるか」。**

   | 書けるか | 切り方 |
   |---|---|
   | 書ける（場所と直し方が決まっている） | **直すタスク 1つ**（今までどおり。止まり方 3 が既定で付くので、依頼書に「見つけたのが違えば止まる」と書かなくてよい） |
   | 書けない。知りたいのは「何が・どこが・なぜ」で、調べれば決まる（「良くして」「なぜ遅い」「どれを直すか決めて」「全部の誤字」） | **調べるタスク** → 候補を Chief of Staff が切る（5）→ 直すタスク（複数でよい） |
   | 書けないが、依頼が小さく候補も少なそう（1〜3 件）で、同じ担当が続けて直すほうが安い | **直すタスク 1つ**で、依頼書の「やること」に「見つけたものを候補にして止まり、範囲の答えを待ってから直す」と書く（止まり方 3 の (b) を最初から使う。調べると直すを1つの端末で） |
   | 書けない。決まらないのは事実ではなく判断（構造の違う案が2つ以上・外との約束・何を作るか） | **Chief Engineer**（人に `andon chief-engineer "<題>"` を勧める。文書の「依頼書に切る案」から直すタスク） |

   例（依頼 → 切り方 → 理由）:
   1. 「docs の誤字を1つ直して」（どれかを指していない）→ 直すタスク（`sonnet`）。1つなら直す。3つ見つけたら止まり（3 の (b)）、Chief of Staff が「3つともこの PR で」（レベル A・小・4種でない。5）と答える。人の turn はゼロ。
   2. 「テストが遅い。なぜ」→ 調べるタスク（`claude`）。候補＝遅い原因と直し方（計測つき）。Chief of Staff が候補から直すタスクを切る。「テストの枠組みを替える」のような構造の候補は Chief Engineer の題として人に勧める。
   3. 「ログインが落ちる。再現手順は issue #12」→ 直すタスク。再現しない・原因が別の module にある → 止まる（3 の (a)・(c)）。
   4. 「API のエラーの返し方を全体で揃えたい」→ Chief Engineer（外との約束＋構造の違う案）。Chief of Staff は切らず、人に題を勧める。
   5. 「依存の版上げ」→ 直すタスク（`sonnet`）。版上げで落ちたテストの原因が版上げと別 → 止まる（3 の (c)。別のタスクの候補にして、版上げだけ出すか、を問う）。

   理由: 推し（前提 7）の「決まっていない依頼は調べると直すに分ける」は正しいが、小さい依頼で毎回2タスクは重い（起こす・報告・finish が2回）。分かれ目を「依頼書に書けるか」と「調べること自体が仕事か」に置き、小さいものは止まり方 3 で同じ担当が続ける。
   捨てた案: 常に分ける（上）／常に1つ（調べる仕事の成果が PR の無い報告になり、型が無いと `review` でゲートが `fix` を返す）／Chief of Staff が自分で調べる（chief-of-staff skill の「深い調査はしない」。読むだけのサブエージェントで切る材料を取るのは今までどおり可）。

7. **2026-11-06 `task.yaml` の予約キー `deliverable`: `pr`（既定）／`candidates`。書き手は Chief of Staff（`kanban set <id> deliverable=candidates`。依頼書の Settings に `Deliverable:` の行を写す）。**

   | 読み手 | `candidates` のとき |
   |---|---|
   | `andon task ready` | `report.json` の `pr` を要らず、代わりに `library/candidates.json` が在って schema に合い（repo があれば）`commit` が `HEAD` と同じことを確かめて `kanban review` |
   | dispatcher | `review` の通知は届けるが、`andon gate` を回さない（`internal/dispatcher/dispatcher.go` の「レビュアー・Chief Engineer・repo の無いタスクは飛ばす」の並びに足す）。`review_request` があっても起こさない |
   | `andon gate`（手で打たれたとき） | 回さず「`deliverable: candidates` のタスクにゲートは無い」を1行出して終了コード 3（誤り。記録しない） |
   | ボード | 型の表示（調べる） |
   | 担当 | 自分の型を知る（`andon task show` の `settings`） |

   - 無い・知らない値は `pr`（今までどおり）。照合はほかの予約キーと同じ（`log.jsonl` で最後に書いたのが Chief of Staff でなければ信じず、`pr` として読む＝安全側）。
   - `andon task start` は変えない（調べるタスクも worktree と branch を作る。使わないだけ）。`andon task finish` は変えない（PR の無いタスクの片付けは今もできる。commit していない変更があれば止まるのも今までどおり——調べるタスクは commit しないか、しても push しないので、`finish` の前に Chief of Staff が担当に捨てさせるか、`git worktree remove --force` の go を人から取る。報告の `## Next` に「commit は捨ててよい」と書かせる）。

   理由: 決めたこと 1。値の名前は「成果の形」で、型の名前（調べる）ではなく成果（候補）を指す——Chief Engineer のタスクは `profile` で見分けていて `deliverable` を書かない（文書の PR なので `pr` のまま）。
   捨てた案: `kind: survey`（問いの `kind` と語が重なる）／repo の無さで見分ける（暗黙。コードの調べものに worktree が要る）／`review: off` と `pr` の無さの組み合わせで推す（原則 2: 黙って既定に落とさない）。

8. **2026-11-06 既存の決定との関係と、変える所の一覧（実装のタスクが直す。この文書の PR では変えない）。**

   | 既存 | 関係 | 変える所 |
   |---|---|---|
   | design.md §1.3（判断は5種） | 変えない。範囲の判断は灯りではない（4） | §1.3 の表の下に1文「範囲の判断（候補のどれを直すか）は Chief of Staff が持ち、灯りはつかない。ADR-0006」 |
   | design.md §1.8（役の表。Chief of Staff の「持つ判断」） | 広げる | 「タスクの範囲内の許可・案件側のアウターループ」に「範囲（候補のどれを直すか。委譲と4種の中で）」を足す |
   | design.md §1.4（ダブルループ） | インナーループの入口に「調べる」が加わる | 図の `B 依頼書` の前に「調べる（候補）」を 1 段。文に1行 |
   | design.md §4（機械向けの約束の表） | 行を足す | 「候補 `tasks/<id>/library/candidates.json`。書き手 担当。読み手 Chief of Staff・トリアージ・振り返り」 |
   | design.md §1.10 委譲 | 変えない（5） | — |
   | contract.md §1・§2・§5・新 §12 | 足す | §1 の木に `candidates.json`、§2 の表に `deliverable`、§5 の `kind` に `scope`（「台帳が受けない」の文を直す）、§12 に候補の形（2 の表と例） |
   | `schema/` | 足す | `candidates.schema.json`（新）、`question.schema.json`（`scope`）、`task-keys.schema.json`（`deliverable`） |
   | ledger.md | 1行 | Questions の `kind` に `scope`、「Added in version 2」に1行。`kanban ask --kind scope` |
   | dispatcher.md §5（P6a の表） | 1行 | `deliverable: candidates` の `review` はゲートを回さず通知だけ |
   | worker skill | §2 に止まる条件（3）、§5 に `scope` の形、新しい節「調べるタスクのとき」（1）、`## 範囲の問い` の見出し、`andon task candidates` | — |
   | chief-of-staff skill | §1 に切り方の表と例（6）、§2 に調べるタスクの雛形と `Deliverable:` の行（1・7）、§4 の「`## 人に聞くこと`」の並びに `scope` の答え方（5）、§7 に調べるタスクの finish | — |
   | ask-one-by-one skill §1 | 1段 | `scope` はまず Chief of Staff（5 の表）。4種に当たる候補だけ人へ |
   | triage skill | 変えない（候補の残りは報告の `next[]` に乗り、今の `next` として拾われる） | — |
   | glossary skill | 語を足す | **候補**（candidate）: 調べて見つけた、直すかもしれないが決めていないもの1つ。一覧は `candidates.json`。_避ける_: 指摘（レビュアーの語）・issue。**調べるタスク**（survey）・**直すタスク**（fix）: タスクの型。`deliverable` |
   | chief-engineer skill | 1行 | 骨格 1「読む」で、事実集めが大きいときは Chief of Staff に調べるタスクを頼んでよい（`next[{to: task}]`。試作の頼み方と同じ） |
   | `internal/task` | 足す | `candidates` の sub command、`ask` の `scope`、`ready` の `deliverable` |
   | `internal/dispatcher`・`internal/gate` | 足す | 7 の表 |
   | `cmd/kanban` | 足す | `--kind scope` |
   | decisions.md §14 | 1行（この文書の PR で足す） | — |

## やらないこと（範囲の外。理由）

- **候補から自動でタスクを切る command**（`andon task cut` など）: 切るのは Chief of Staff（Chief Engineer の文書と同じ扱い）。候補が溜まって手で切るのが重くなったら次の ADR。
- **調べるタスクの独立レビュー・反論**: 差分が無い。反論が要る大きさなら Chief Engineer の題。
- **人への問いの形・判断の4種・委譲の `scope` の変更**（4・5）。
- **トリアージの `collect` が `candidates.json` を直接拾うこと**: 残りは報告の `next[]` に乗るので今の口で足りる。二重に拾う形を作らない。
- **担当が候補の直しを試しに commit して見せること**: 調べるタスクは PR を出さない。試作は Chief Engineer の 4 の「捨てる前提の小さい試作」の形で。
- **範囲の判断の自動化**（候補のレベルを command が判定して Chief of Staff に代わる）: 判断は Chief of Staff。ゲートの条件表を候補に当てる道具は、要るときに足す。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **schema（`tests/schema.sh`）**: `schema/candidates.schema.json` があり、contract.md §12 の例が通る。`commit` 無しの例（repo の無いタスク）も通る。`id` の重複・`in_scope` 無し・知らないキーは落ちる。`question.schema.json` が `kind: scope` を通す。`task-keys.schema.json` が `deliverable: candidates` を通し、`andon task show` の出力が schema に合う。
2. **担当の command（`tests/andon-task.sh`・`go test ./internal/task/...`）**: `andon task candidates <id> -` が形を確かめて `library/candidates.json` に書き（repo があれば `commit` は `HEAD`）、形が違えば書かずに終了コード 1 と理由1行。`andon task ask` が `kind: scope` を受けて `kanban ask --kind scope` を打つ。`andon task ready` が `deliverable: candidates` のとき `pr` 無しで `candidates.json` を要って `review` にし、`pr`（既定）のときは今までどおり `pr` を要る。`stop-check` は `scope` の問いが開いたままのターンを通す。
3. **dispatcher（`tests/andon-dispatcher.sh`）**: `deliverable: candidates` のタスクが `review` に入ると通知 `review` だけで `andon gate` を打たない。`kind: scope` の問いが `asked` で届く（離席の経路③でも）。`deliverable` を最後に書いたのが Chief of Staff でなければ `pr` として読む（ゲートを回す）。
4. **ゲート（`tests/andon-gate.sh`）**: `deliverable: candidates` のタスクに `andon gate` を打つと終了コード 3 と理由1行、記録を書かない。
5. **台帳（`tests/kanban.sh`）**: `kanban ask --kind scope` が通り、`questions.yaml` に `kind: scope` が残る。
6. **パッケージ（`tests/package.sh`）**: skill の文の塊（`candidates` の JSON・`scope` の問い・調べるタスクの雛形の Settings）が schema に通る。仮設の語（推し・知らせ）が無い。
7. **建屋で実測（P4 以降の決まり。報告に書く）**: 「docs の誤字を1つ直す」を直すタスクで流し、3つ見つけた担当が候補と `scope` の問いで止まり、Chief of Staff が答え、PR が1本出てマージまで**人の turn がゼロ**。調べるタスクを1つ流し、候補の一覧から Chief of Staff が直すタスクを切って `finish` できる。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（decisions.md §14 に1行） | CI green（`tests/public.sh` を含む） | — |
| S2 | 約束: `schema/candidates.schema.json`・`question` の `scope`・`task-keys` の `deliverable`、contract.md §1・§2・§5・§12、glossary の語（候補・調べる／直す・`deliverable`） | 受け入れ条件 1 | S1 のマージ |
| S3 | 担当の command: `andon task candidates`・`ask` の `scope`・`ready` の `deliverable`、`cmd/kanban` の `--kind scope`、ledger.md の1行 | 受け入れ条件 2・5 | S2 |
| S4 | dispatcher とゲート: `deliverable: candidates` でゲートを回さない・手で打てば終了コード 3、dispatcher.md §5 の1行 | 受け入れ条件 3・4 | S2。S3 と独立 |
| S5 | skill と設計書の文: worker・chief-of-staff・ask-one-by-one・chief-engineer の skill、design.md §1.3・§1.4・§1.8・§4（決めたこと 8 の表） | 受け入れ条件 6 | S3・S4（command が在ってから書く） |
| S6 | 建屋で実測（受け入れ条件 7）。食い違えば実測が正で、この文書の「決めたこと」に下から足す | 報告 | S5 |

## 開いている問い（人に聞く。1 問ずつ。推奨案つき）

1. **`scope` の問いを Chief of Staff が自分で答えたとき、人に1行知らせるか**（判断の種類: 利用者〈人の使い勝手〉）。推奨案: 知らせない（委譲の範囲の中と同じ扱い。記録は `questions.yaml` と担当の報告に残り、ボードと振り返りで読める）。人が「知りたい」なら、chief-of-staff skill の「人に言うことがあれば1行」の形で、答えた turn に1行。

## 出典（読んだもの・捨てた案の出所）

- 依頼書（Y-158。人の問い 2026-11-06、Chief of Staff の推し、追記 2 本）。
- Andon の側: [design.md](../design.md) §1.3・§1.4・§1.5・§1.8・§1.9（(a)・(b)）・§1.10・§4、[contract.md](../contract.md) §0・§1・§2・§4・§5・§7・§9・§11、[dispatcher.md](../dispatcher.md) §1・§2・§3・§5・§6、[operate.md](../operate.md)「役の配置と起動」、[ledger.md](../ledger.md)（`task.yaml`・Questions・`log.jsonl`）、[decisions.md](../decisions.md) §2（D8・D10・D15）・§14・§16（Y-132・Y-141）、[ADR-0001](0001-skills-distribution.md)〜[ADR-0005](0005-cell-repo.md)、`schema/`（`report`・`question`・`findings`・`task-keys`）、`package/skills/`（chief-of-staff・worker・chief-engineer・ask-one-by-one・triage・independent-review・glossary）、`internal/task/stopcheck.go`・`task.go`（`ready`・`ask`）、`internal/dispatcher/dispatcher.go`（`review` を飛ばす条件）・`loop.go`（`review`・`startReviewer`）、`internal/gate/gate.go`（`pr` が無いときの `early`）、`cmd/kanban/questions.go`（`questionKinds`）、`internal/triage/collect.go`（`next` の拾い方）。
- 仮設の台帳（読むだけ）: 依頼書 Y-131・Y-132・Y-133・Y-137・Y-148・Y-181（P7 で見えたこと。A-7 の題「docs の誤字を1つ直す」）。P7 の建屋の台帳そのものは読めていない。
- 先輩プロジェクトの調べるだけのタスク（research-only task）は ADR-0004 決めたこと 2 の出典（shepherdr 1.0）を経由して。本文は読んでいない。
