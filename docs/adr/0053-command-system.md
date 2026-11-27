# ADR-0053: andon のコマンドを「名詞 動詞」に組み直し、どこで打っても動くようにする（仕様・設計。2026-11-22）

- 状態: **決定（2026-11-22）**。オーナーが対話の中で「決めた」と言った。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: `andon` のコマンドの名前、打つ場所、フラグ、help、人向けの文書を、1つの決まりに揃えること。Chief of Staff の定型の手順のうち、今ある動きを順に打つだけのものを1つのコマンドに畳むこと。
- 物差しは Command Line Interface Guidelines（ https://clig.dev/ ）。語は [design.md](../design.md) §0.1。改名で別名を作らない前例は [ADR-0008](0008-issue-naming.md) 決めたこと 7 と [ADR-0052](0052-title-and-theme-not-dai.md) 決めたこと 9。Chief of Staff をセルに1人にする検討（[ADR-0031](0031-cell-chief-of-staff.md)。提案）と並んで読む。
- この文書で「葉」は、`andon issue start` のように実際に打てる1つのコマンドを指す。「名詞」は1語目、「動詞」は2語目を指す。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は全部の葉。名前と形を決まりに合わせて組み直す | この ADR | まだ実装していない |
| 2 | 物差しは clig.dev。書いていない2点は Andon の決まりとして決める | この ADR | まだ実装していない |
| 3 | andon はセルの道具で、プロジェクトは引数。どの葉もセルの中のどこで打っても動く | `internal/cli/project.go` | 生きている |
| 4 | プロジェクトは `--project`、`KANBAN_ROOT`、今いるフォルダ、セルに1つだけ、の順で決める | `internal/cli/project.go`・`docs/ledger.md` の Finding the root | 生きている |
| 5 | 葉の形は `andon <名詞> <動詞>` の2語。1語で打てるのは、andon そのものを扱う `open`・`help`・`version`・`boot` だけ | この ADR | まだ実装していない |
| 6 | 名詞は扱うものの名前にし、役の名前を名詞にしない。Chief of Staff を開くのは `andon open`、Chief Engineer を起動するのは `andon design start` | この ADR | まだ実装していない |
| 7 | 2語目は動詞か、その葉が出す・受け取るものの名前にする。形容詞と過去分詞は使わない | この ADR | まだ実装していない |
| 8 | 新しい葉の一覧は「細部」の表のとおり。18 の葉の名前が変わり、37 は変わらない | この ADR | まだ実装していない |
| 9 | 別のバージョンから呼ばれる葉は、名前を変えないか、呼び手の入れ直しを手順に書く | この ADR | まだ実装していない |
| 10 | 機械だけが打つ葉は、名詞を分けずに、表の「打つ者」の欄で分ける | この ADR | まだ実装していない |
| 11 | 葉の表を1つ持ち、振り分けと help と人向けの一覧と補完をそこから作る | この ADR | まだ実装していない |
| 12 | `andon help` は例を先に出し、誰が打つかの3群に分ける | この ADR | まだ実装していない |
| 13 | フラグは `--project`・`--json`・`--dry-run`・`--help` を全部の葉で同じ意味にする | `internal/cli/leaves.go`・`internal/cli/cli.go`・`internal/cli/project.go` | 生きている |
| 14 | 状態を変えた葉は、何をしたかと次に打つ1手を出す。断るときは直す1手を出す | `internal/issue/start.go`・`internal/issue/resume.go`・`internal/chiefofstaff/chiefofstaff.go`・`internal/chiefengineer/chiefengineer.go` | 生きている |
| 15 | 終了コードは 0・1・2 の意味を揃え、3 以上は葉ごとに help に書く。フックが打つ葉は 2 を使わない | `internal/cli/leaves.go`・`internal/cli/project.go` | 生きている |
| 16 | 人向けの文書は生成した一覧 `docs/commands.md` の1枚だけ。詳細は `--help` が持つ | この ADR | まだ実装していない |
| 17 | 名前は群ごとに一度に替え、別名は作らない。古い名前を打つと新しい名前を言って断る | この ADR | まだ実装していない |
| 18 | `andon issue create` を足す。担当の課題を足すのと予約キーを書くのを1回で打つ | `internal/issue/create.go` | 生きている |
| 19 | 依頼書の Settings 欄は command が予約キーから書く | `internal/ledger/ledger.go` | 生きている |
| 20 | kanban の CLI は名前を変えない。足すのは、kanban の場所の探し方の最後の1段だけ。`hc` は案内だけ | `cmd/kanban/ledger.go`・`docs/ledger.md` の Finding the root | 生きている |
| 21 | 配る文の中の andon のコマンドが表にあることを、検査で見る | この ADR | まだ実装していない |
| 22 | CLI の枠組みは入れず、表は自作する | この ADR | まだ実装していない |
| 23 | ADR と決定の記録の中の古い名前は替えない | この ADR | まだ実装していない |

## 問題（オーナーの言葉で。言い直しの段落）

オーナーは 2026-11-22 に「andon commandのリファレンスとコマンドの体系を一度整理してみませんか」「少し認知負荷が高いコマンドだなと思うのと、勝手が悪いなという雰囲気を感じるときがあります」と言った。入口の対話で、困りごとは3つに絞れた。全体の見通しが悪いこと、名前から何をするか分からないこと、打つ場所に縛られることである。オーナーの言葉では「andon chief-of-staffというのもコマンド名としては何するものかわからない」「Andonの立ち位置を考えたときに projects/<slug>配下じゃないと動かないコマンドというのも何か違和感があります」「人間向けのdocsは詳細はいらない気がします」。前日にも「andonコマンドが、プロジェクトについて強く認識しているのは違和感がある」と言っている。楽にしたい相手は第一にオーナーで、打つとき、AI の打ったものを読むとき、設計を判断するときの3つの場面である。

事実（repo とセルで数えた。2026-11-22）:

1. `andon help` は `help` を除いて 21 個を1列に並べる。葉まで数えると 55 通りある。誰が打つかの区別は無い。
2. 21 個は 16 日で 10 個から増えた。最初のバージョンが 10、4つ目が 12、8つ目が 15、12 個目が 16、13 個目が 19、今の main が 21 である。
3. 55 通りを主に打つ者で分けると、オーナーが手で打つものが 14、AI の役が打つものが 21、機械と開発だけが打つものが 20 である。`andon waiting` のように、オーナーも役も機械も打つ葉がある。
4. 一覧の説明に載っていない葉が 15 ある。`andon help` の1行と各パッケージの使い方の文が、別々に手で書かれているためである。サブコマンドの一覧を手で持つ所がほかに 3 つあり（`internal/cli/cli_test.go` が 19 個、`tests/andon.sh` が 16 個、bash の補完 `image/completions/andon` が 16 個）、どれも 21 個に足りない。
5. 5日間のセッションの記録で、役が `andon`・`kanban`・`hc` を打ったのは約 10,000 回、そのうち使い方の誤りは 9 回、`andon help` を引いたのは 25 回だった。AI の役はほぼ誤っていない。
6. プロジェクトの決め方は2通りに割れている。多くの葉は kanban の CLI に場所を聞く（`KANBAN_ROOT`、上のフォルダ、repo が載っているプロジェクトの順）。`andon delegate` と `andon signal` は、`--project` か `projects/<slug>/` の下だけを見る。どちらも、セルの根で打つと、プロジェクトが1つしかなくても断る。`--project <slug>` を取るのは 13 通りだけである。使い方の文は「`projects/<slug>/` で打つ」と書いている。
7. 語の並びが揃っていない。`andon issue start` は名詞と動詞、`andon create` は動詞だけ、`andon chief-of-staff` は役の名前だけ、`andon waiting` は形容詞だけである。
8. JSON の出し方が揃っていない。`--json` を取る葉が 5、付けなくてもいつも JSON を出す葉が 9 ある。
9. 終了コードは、0 が成功、1 が失敗、2 が使い方の誤りという形が多いが、揃ってはいない。プロジェクトが決まらないとき、`andon issue` と `andon waiting` は 1、`andon delegate` と `andon signal` は 2 を返す。`andon schema check` の 2 は「検査できない」である。3 と 4 は葉によって意味が違う。フックが打つ `andon dispatcher wait` と `rewake` は、Claude Code が終了コード 2 を合図として読むので、使い方の誤りでも 0 を返す。
10. コマンドのリファレンスの文書は無い。使い方は各葉の `--help`、役の skill、[contract.md](../contract.md)、[operate.md](../operate.md) に分かれている。
11. Chief of Staff は5日間で `kanban add` を 126 回、`kanban set` を 211 回、`andon issue start` を 173 回打った。`kanban set` のうち 97 回は、1回で予約キーを6つ以上書いている。同じ値を依頼書の Settings 欄にも手で書いている（`package/skills/chief-of-staff/playbooks/brief.md`）。
12. 別のバージョンから呼ばれる葉が 4 種類ある。
    - コンテナの command の `andon boot`。compose の定義はコンテナを作るときに決まる。セルに main を当てる道具（`image/apply-main.sh`）はバイナリだけを入れ替えるので、コンテナを起動し直すと、古い定義が新しいバイナリを打つ。当てたものを戻すときは、手元のスクリプトが、戻した古いバイナリに `boot` を打つ。
    - 端末のテンプレートの `andon issue session`。同じ名前のテンプレートはセルの repo の `templates.local/` が勝つので、セルの repo に写しがありうる。断られるとセッションの id が空のまま起動する。
    - ホストの部品（`package/host/` の 3 つ）の `andon waiting --json` と `andon waiting answer`。ホストに入れた写しは、セルの andon と別の時に上がる。3 つのうち、食い違ったときに理由を出すのは 2 つで、1 つは「セルの andon を上げる」と逆向きの案内を出す。残りの 1 つは何も出さない。
    - 動いている dispatcher。boot とフックは、端末の前に出ているものの引数がちょうど `andon dispatcher` かで、dispatcher が動いているかを見る。
13. フックのスクリプト（`package/hooks/`）は、パッケージの置き場ごとバイナリと一緒に入れ替わる。
14. 名前を変える 18 の葉の文字列は、repo に約 800 か所ある。内訳は skill が 66、テンプレートとホストの部品などが 53、検査が 171、ADR を除く文書が 224、流れの定義と schema とモデルが 49、Go が 229 である。変わらない `andon waiting answer` と、コマンドでない文を含む上限の数である。ADR と決定の記録の中には、別に約 350 か所ある。
15. コマンドの名前に見えて、替えてはいけない文字列がある。gate が PR に書くコメントの頭 `andon gate: `（dispatcher がこの頭で見分ける）と、boot が出すログの頭 `[andon boot]`（セルの repo の起動のスクリプトの写しが待つ）である。
16. `andon create` は課題の id の接頭字の形だけを見て、プロジェクトの間で重なるかは見ない。
17. 引数は葉ごとに手で読んでいる。依存しているライブラリは 4 つである。既製の CLI の枠組み cobra は v1.10.2（2025-12-03）で、依存を 4 つ持つ。
18. skill が名指す command が実物にあるかを見る検査は、検証の skill についてだけ既にある（`tests/verify-andon.sh`）。この検査は `internal/cli/cli.go` と `internal/issue/issue.go` を字で読んでいる。
19. 流れの定義（`process/states/issue.yaml`）とモデルの対応表（`model/issue-map.txt`）が `andon issue ready` を名指している。対応表を変える PR は、オーナーが見る。`package/` の skill・フック・テンプレート・ホストの部品・compose の定義と `image/` を変える PR も、オーナーが見る。
20. 担当の端末で `kanban ready` が 58 回打たれていたが、中身を読むと全部が Andon の開発（検査と、その文字列を含むファイルの編集）だった。`andon issue ready` との取り違えは見つからなかった。

モデル: 調べなかった（状態を動かす流れを変えない。変えるのは葉の名前と入口の形で、足す葉は今ある動きを同じ順に打つだけである）

## 前提（同意済み。番号）

1. 困りごとは3つである。全体の見通しが悪いこと、名前から何をするか分からないこと、打つ場所に縛られること。
2. 楽にしたい相手は第一にオーナーである。AI の役が誤らないことは、今も足りているので狙いにしない（事実 5）。
3. 何もしなければ、コマンドは今の速さで増え、一覧と実物のずれも増える（事実 2・4）。
4. 人向けの文書に詳細は書かない。詳細は各葉の `--help` が持ち、人向けには短い全体図だけを置く。
5. 設計の物差しは clig.dev にする。打つ場所の決め方と、機械だけが打つ葉の扱いは clig.dev に書いていないので、Andon の決まりとして決める。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-22 範囲は全部の葉。名前と形を決まりに合わせて組み直す。**（オーナーが決めた。Chief Engineer の推奨は「決まりは全部の葉に決め、名前を変えるのはオーナーが打つ葉と決まりから外れる葉だけ」だった）
   AI の役と機械が打つ葉、Chief of Staff の定型を畳む直し、kanban の CLI の見直しを含む。
   理由: オーナーが入口で選んだ。
   捨てた案: help と全体図だけ直す（名前と打つ場所の困りごとが残る）／オーナーが打つ葉だけ直す（残りが別の決まりのまま残り、次に足す葉が迷う）。

2. **2026-11-22 物差しは clig.dev。書いていない2点は Andon の決まりとして決める。**（オーナーが決めた）
   使うのは、名前（短く、覚えやすく、紛らわしい名前を並べない）、サブコマンドの語順を揃えること、help（短く、例を先に）、標準の名前のフラグ、人向けの出力と `--json` の出し分け、状態を変えたら言うこと、次のコマンドを勧めること、人向けに書き直したエラーである。
   clig.dev に書いていないのは、どのフォルダで打ったかに頼ることの是非と、機械だけが打つ葉の扱いである。前者は決めたこと 3・4、後者は 10・12 で決める。
   理由: オーナーが提案した。広く読まれている指針で、次に葉を足す者も同じ物差しを引ける。

3. **2026-11-22 andon はセルの道具で、プロジェクトは引数。どの葉もセルの中のどこで打っても動く。**（オーナーが決めた）
   今いるフォルダは、プロジェクトを省いたときの手がかりの1つとして使うだけにする。使い方の文から「`projects/<slug>/` で打つ」を消す。
   理由: セルにはプロジェクトがいくつも入る。andon はその上にいる道具で、1つのプロジェクトのフォルダの中の道具ではない。Chief of Staff の作業フォルダをセルの根にする検討（ADR-0031 の向き 3）も、プロジェクトを指す口が要ると書いている。この決まりは、その検討がどちらに決まっても使える。
   捨てた案: 今のまま、葉ごとに打つ場所を help に書く（事実 6 の割れが残る）。

4. **2026-11-22 プロジェクトは次の順で決める。`--project <slug>`、環境変数 `KANBAN_ROOT`、今いるフォルダ、セルにプロジェクトが1つだけならそれ。**（Chief Engineer が決めた）
   今いるフォルダからの決め方は、今の kanban の CLI と同じである。上のフォルダに kanban があるか、kanban の repos に載った repo の checkout の中なら、そのプロジェクトになる。どれでも決まらなければ、プロジェクトの一覧と `--project <slug>` を付ける1行を出して、終了コード 2 で断る。
   理由: clig.dev の優先の順（フラグ、環境変数、プロジェクトの設定）に合う。プロジェクトが1つのセルでは何も付けずに済む。
   細部: `--project` はプロジェクトを読む葉の全部が受ける。受ける位置は動詞のあとならどこでもよい。`--project` と `KANBAN_ROOT` が食い違えば `--project` を使う。名詞 `project` の葉と `open` は、slug を引数で受ける。セルの葉（`cell`・`file`・`package`・`schema`・`process`・`version`・`boot`）はプロジェクトを読まない。`dispatcher run` は省くと全部のプロジェクトを回る。今いるフォルダで決まるときは、「1つだけ」の段まで進まない。担当の worktree で打つ葉が、別のプロジェクトを読むことは無い。
   ADR-0031 の案 6（課題を `<slug>/<id>` の形で受け、接頭字を重ねない）は、プロジェクトを指す別の口である。その検討を決めるときに、id を取る葉でその形も受けるかを決める。この ADR では足さない。
   捨てた案: 環境変数 `ANDON_PROJECT` を足す（`KANBAN_ROOT` と役目が重なり、名前が2つになる）／課題の id の接頭字からプロジェクトを決める（接頭字の重なりを誰も断っていない。事実 16）／短い形 `-p` を足す（clig.dev は1字のフラグをよく使うものに限り、`-p` はポートに使われることが多い）。

5. **2026-11-22 葉の形は `andon <名詞> <動詞>` の2語。1語で打てるのは、andon そのものを扱う `open`・`help`・`version`・`boot` だけ。**（Chief Engineer が決めた。`open` を1語にすることはオーナーが決めた）
   名詞は何を扱うか、動詞はどうするかを言う。引数とフラグはそのあとに置く。1語の葉は名詞にしない。名詞だけを打つと、その名詞の help を出す。
   1語の 4 つは、andon を開く、andon の使い方を見る、andon のバージョンを見る、andon を起動する、である。
   理由: clig.dev はサブコマンドの語順を揃えることを求め、名詞を先に置く形が多いと書いている。今いちばん葉の多い `andon issue` がこの形で、役は誤らずに打っている（事実 5）。名詞だけで常駐が始まる今の `andon dispatcher` のような葉を無くせる。`help` と `version` は、どの CLI でも1語である。`open` で開くものは Chief of Staff しか無く、名詞を足しても情報が増えない（オーナーの指摘）。`boot` は決めたこと 9 の理由で変えない。
   捨てた案: 動詞を先に置く（`andon create project`。今の 37 の葉の多くを並べ替えることになる）／よく打つ葉に、2語の名前と1語の近道の両方を持たせる（同じことに名前が2つでき、別名を作らない前例に反する）／3語まで許す（`andon dev schema check`。深さが葉ごとに変わり、覚えにくい）。

6. **2026-11-22 名詞は扱うものの名前にし、役の名前を名詞にしない。Chief of Staff を開くのは `andon open`、Chief Engineer を起動するのは `andon design start`。**（Chief Engineer が決めた。オーナーが打つ葉の名前はオーナーが決めた。Chief Engineer の初めの推奨は `andon cos open` と `andon ce start` だった）
   名詞は、オーナーが口で言う語から選ぶ。プロジェクト、設計、課題、オーナー待ち、委譲、信号、指標、セルなどである。役の名前だけの葉（`andon chief-of-staff`・`andon chief-engineer`）は無くす。
   理由: `andon chief-of-staff` が何をするか分からないのは、名前が誰が動くかを言い、何が起きるかを言わないからである。Chief of Staff を開く入口は今 2 つあり、`andon open` の1つにまとめる。`andon design start` は「設計の検討を始める」と読める。Chief of Staff をセルに1人にする検討（ADR-0031 の向き 3）も `andon open` で開くと書いていて、その検討が決まっても、どちらの名前も変えずに済む。
   捨てた案: 役の短い名前を名詞にする（`andon cos open`・`andon ce start`。`cos open` は `open` と同じ意味で、名詞が情報を足さない。`ce` も誰が動くかを言うだけである。オーナーの指摘）／`andon project open`（Chief of Staff がセルに1人になると、開くものがプロジェクトでなくなる。オーナーは、andon がプロジェクトを強く意識することに違和感があると言っている）／`andon theme start`（検討の単位を「テーマ」と呼ぶ決まりと語は揃うが、コマンドの名前としては画面の配色と取り違えやすい）／役の長い名前に動詞を足す（`andon chief-of-staff open`。いちばんよく打つ葉が長い）。

7. **2026-11-22 2語目は動詞か、その葉が出す・受け取るものの名前にする。形容詞と過去分詞は使わない。**（Chief Engineer が決めた）
   同じことには同じ動詞を使う。並べるのは `list`、1件を出すのは `show`、入力を JSON に集めるのは `collect`、決めたことを当てるのは `apply`、作るのは `create`、起動するのは `start`、続きから起動するのは `resume`、片付けるのは `finish` である。
   役が成果を渡す葉は、渡すものの名前で呼ぶ（`report`・`review-request`・`candidates`・`patterns`・`verdict`）。1つのものを読んで出す葉も、そのものの名前で呼んでよい（`impact`・`session`）。
   理由: 渡すものの名前は、約束の JSON のキーやファイルの名前と同じ綴りで、役はそこから葉を引ける。形容詞と過去分詞は、聞いているのか変えるのかが読み取れない（`ready` は「揃ったか」とも「揃ったことにする」とも読める）。
   捨てた案: 2語目を動詞だけにする（`issue report` などの 7 つを言い換えることになり、約束のキーとの対応が切れる）。

8. **2026-11-22 新しい葉の一覧は下の表のとおり。18 の葉の名前が変わり、37 は変わらず、2つを足す。**（オーナーが打つ葉の名前はオーナーが決めた。ほかは Chief Engineer が決めた）
   理由と捨てた案は、決めたこと 5〜7・9・10 のとおり。葉ごとの理由は表の右の欄に書く。
   細部（打つ者の欄は、オーナー、役、機械の3つで、主に打つ者を先に書く。役は Chief of Staff・Chief Engineer・担当・レビュアーを指す。機械は dispatcher・フック・テンプレート・compose・ホストの部品・検査・イメージを焼く道具を指す）:

   | 新しい葉 | 今の葉 | 打つ者 | 変える理由 |
   |---|---|---|---|
   | `open [slug]` | `open`・`chief-of-staff` | オーナー、役 | 同じことをする入口が2つあり、片方は役の名前だけである。`open` に寄せる |
   | `design start "<タイトル>"` | `chief-engineer "<タイトル>"` | オーナー、役 | 役の名前だけで、何が起きるかが名前に無い |
   | `project create [slug]` | `create` | オーナー | 何を作るかが名前に無い |
   | `project list` | （足す） | オーナー | プロジェクトを並べる、読むだけの葉が無い |
   | `waiting list` | `waiting` | オーナー、役、機械 | 2語に揃える |
   | `waiting answer` | 同じ | オーナー、機械 | — |
   | `delegation show`・`grant`・`narrow`・`revoke` | `delegate show`・`grant`・`narrow`・`revoke` | オーナー（`show` は役も） | 1語目が動詞である。約束のキーも `delegation` である |
   | `metrics show` | `measure` | オーナー、役 | 動詞だけで、何を出すかが名前に無い |
   | `signal auth` | 同じ | オーナー | — |
   | `cell init <slug>` | `init` | オーナー | 何を作るかが名前に無い |
   | `issue create "<タイトル>"` | （足す。決めたこと 18） | 役 | — |
   | `issue start`・`resume`・`finish`・`impact` | 同じ | 役、機械 | — |
   | `issue gate <id>` | `gate <id>` | 役 | 1語で、課題に何をするかが名前に無い |
   | `issue show`・`report`・`review-request`・`candidates`・`ask`・`record` | 同じ | 役 | — |
   | `issue submit <id>` | `issue ready <id>` | 役 | 形容詞である（決めたこと 7） |
   | `issue merge`・`settle` | 同じ | 機械（`settle` は役も） | — |
   | `issue session`・`stop-check` | 同じ | 機械 | —（決めたこと 9・10） |
   | `news ack` | 同じ | 役 | — |
   | `file publish <file>` | `publish <file>` | 役 | 2語に揃える |
   | `triage collect`・`apply`・`verdict` | 同じ | 役 | — |
   | `retro collect`・`patterns`・`apply` | 同じ | 役 | — |
   | `digest collect`・`build`・`send` | 同じ | 役 | — |
   | `dispatcher run [--once]` | `dispatcher`・`dispatcher --once` | 機械 | 名詞だけで常駐が始まる |
   | `dispatcher drain`・`wait`・`rewake` | 同じ | 機械 | —（決めたこと 10） |
   | `signal collect` | 同じ | 機械 | — |
   | `signal mark-delivered` | `signal delivered` | 機械 | 過去分詞である（決めたこと 7） |
   | `boot` | 同じ | 機械、オーナー | —（決めたこと 9） |
   | `cell serve-files` | `boot serve-files` | 機械 | 1語の葉を名詞にしない |
   | `package stamp <revision>` | `version stamp <revision>` | 機械 | 1語の葉を名詞にしない |
   | `schema check`・`batch` | 同じ | 機械、役 | — |
   | `process render`・`json` | 同じ | 機械 | — |
   | `version` | 同じ | 機械 | — |
   | `help` | 同じ | 全員 | — |

   名詞は 17 になる（`design`・`project`・`waiting`・`delegation`・`metrics`・`signal`・`cell`・`issue`・`news`・`file`・`triage`・`retro`・`digest`・`dispatcher`・`package`・`schema`・`process`）。
   `open` は、今の `open` と `chief-of-staff` を合わせた葉である。slug を省くと決めたこと 4 の順でプロジェクトを決め、決まらず端末で打っていれば今と同じく選ばせる。
   名前を名指しで決めていた過去の決定は、名前だけを改める。`andon waiting` と `andon waiting --json`（[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 決めたこと 9・14、[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 8。どれも Chief Engineer が決めた）は `andon waiting list` になる。command を andon に置くという決定の中身は変えない。
   Go のパッケージの名前は変えない。
   同じ日に決まった ADR が足す葉は、この決まりで読む。[ADR-0048](0048-thin-cell-entry.md) 決めたこと 11 の `andon init --print` は `andon cell init --print` になる。先に入ったほうの名前を、あとの PR が合わせる。[ADR-0051](0051-signals-flow-and-breaking-change.md) の `andon signal trace` は2語で、そのまま表に足す。これから足す葉も、足す PR が表に足す。

9. **2026-11-22 別のバージョンから呼ばれる葉は、名前を変えないか、呼び手の入れ直しを手順に書く。**（Chief Engineer が決めた）
   `andon boot` と `andon issue session` は名前を変えない。`andon waiting` は `waiting list` に変え、ホストの部品を同じ PR で直し、当てたあとにオーナーがホストで部品を入れ直す。`dispatcher` を `dispatcher run` に変えるときは、動いているかを見るコードが、古い形と新しい形の両方を dispatcher として読む。
   理由: 同じ commit で呼び手を直せない葉がある（事実 12）。`boot` は、名前を変えると、main を当てたあとにコンテナを起動し直したセルが立ち上がらなくなり、当てたものを戻す道も壊れる。`issue session` は、セルの repo に写しがあると、断られたことが分からないまま、続きから起動できないセッションができる。どちらも黙って壊れるので、名前を変えない。`waiting` は、入れ直すまでオーナー待ちの表示が出ないだけで、何も壊さない。
   細部: ホストの部品は、セルの andon と合わないときに「セルの andon とこの部品のバージョンが合わない。部品を入れ直す」と出すように直す。効くのは新しい部品だけなので、今回の入れ直しまでは、古い部品が逆向きの案内を出すか、何も出さない（事実 12）。入れ直しの手順は、名前を変える PR の本文と [operate.md](../operate.md) と CHANGELOG に書き、当てる Chief of Staff がオーナーに伝える。
   捨てた案: `boot` も変え、焼き直したイメージでだけ当てる（main を当てる道具が使えなくなる）／`waiting` も変えない（名詞だけで動く葉が残る）／`waiting` だけ古い名前もしばらく動かす（別名になる。オーナーは前に、別名を作らず手で移すほうを選んでいる）。

10. **2026-11-22 機械だけが打つ葉は、名詞を分けずに、表の「打つ者」の欄で分ける。**（Chief Engineer が決めた）
    フックが打つ `dispatcher drain`・`wait`・`rewake` と `issue stop-check`、テンプレートが打つ `issue session` は、今の名詞の下に残す。help では機械が打つ葉の群に出す（決めたこと 12）。
    理由: 通知が Chief of Staff に届く道（`dispatcher drain`）は、ラインでいちばん止めてはいけない道である。名前を変えても、オーナーにも役にも見える得が無い。打つ者は表の欄で言える。
    捨てた案: フックとテンプレートが打つ葉を名詞 `hook` に集める（`issue session` は決めたこと 9 で動かせず、半分だけ移ることになる。名詞を呼び手で名づけると、決めたこと 6 と食い違う）／機械が打つ葉を全部 `andon internal …` に移す（`issue merge` や `signal collect` は役も読む課題と信号の話で、名詞を離すと探しにくい）／別のバイナリに分ける（焼く物と検査が増える）。

11. **2026-11-22 葉の表を1つ持ち、振り分けと help と人向けの一覧と補完をそこから作る。**（Chief Engineer が決めた）
    表は Go の中に置く。葉ごとに、名詞、動詞、打つ者、1行の説明、使い方の文、例、3 以上の終了コード、状態を変えるか、プロジェクトを読むか、JSON を出せるかを持つ。`andon` は表に載った葉だけを実行する。
    理由: 一覧と実物がずれるのは、一覧と使い方の文と振り分けが別々に書かれているからである（事実 4）。振り分けが表を読めば、載っていない葉は打てず、ずれが起きない。
    細部: 今は名詞ごとのパッケージが自分で2語目を振り分けている。それを表に移す。葉の中の引数の読み方は変えない。手で持つサブコマンドの一覧（事実 4 の 3 つ）は消し、bash の補完は表から作る。検証の skill の検査（事実 18）は、Go のファイルを字で読むのをやめ、`andon help --all` の出力を読む。
    捨てた案: 今のまま、一覧と実物を照らす検査を足す（ずれを後から見つけるだけで、書く場所は2つのまま残る）。

12. **2026-11-22 `andon help` は例を先に出し、誰が打つかの3群に分ける。**（Chief Engineer が決めた）
    引数なしの `andon` と `andon help` は、1行の説明、オーナーがよく打つ例を 3 つ、オーナーが打つ葉の一覧、役が打つ名詞の1行、機械が打つ名詞の1行、続きの引き方の順に出す。`andon help --all` は全部の葉を1行ずつ、打つ者と一緒に出す。`andon <名詞> --help` はその名詞の葉を打つ者ごとに出す。`andon <名詞> <動詞> --help` は使い方、例、フラグ、終了コードを出す。`-h` はどこでも `--help` と同じにする。
    理由: clig.dev は、help を短くし、例とよく使うものを先に出すことを求めている。最初の画面をオーナーの葉に絞れば、55 を読まずに済む。
    細部: help の文は今と同じく日本語で書く。最後の行で、課題の読み書きは `kanban`、端末は `hc` と案内する。知らない名詞や動詞を打たれたら、綴りの近い葉を1つ勧めて終了コード 2 で断る。
    捨てた案: 目的で分ける（`hc` の help の形。「始める」「見る」のような見出しは葉が増えるたびに置き場を迷う。打つ者は葉ごとに決まっている）／機械が打つ葉を help に一切出さない（開発する者が引けない）。

13. **2026-11-22 フラグは `--project`・`--json`・`--dry-run`・`--help` を全部の葉で同じ意味にする。**（Chief Engineer が決めた）
    JSON を出せる葉（表の欄で決まる）は、`--json` を付ければ必ず JSON を標準出力に出す。付けないときは、人向けの表示を持つ葉は人向けに出し、持たない葉は今と同じく JSON を出す。`--dry-run` は、何も書かずに、書くはずのことを出す。
    理由: 呼ぶ側は「`--json` を付ければ JSON」とだけ覚えればよい。いつも JSON を出す葉に人向けの表示を新しく作ることはしない。読む者がいないからである（事実 8 の 9 つは役と機械だけが読む）。
    細部: 長い名前を必ず持つ。標準入力を読む引数は `-`、フラグの終わりは `--` で、今と同じである。値を1行で出す葉（`issue session`）、ファイルに書いてパスを出す葉（`digest collect`）、合うかどうかを終了コードで返す葉（`schema check`・`process render --check`）は、JSON を出せる葉に数えない。`process render --check` は `--dry-run` とは意味が違う（差分があれば落ちる）ので、そのまま残す。書く葉が書いたものを JSON で出す形は、今ある `issue start --dry-run --json` のままにし、広げない。
    捨てた案: 全部の読む葉に人向けの表示を作る（`triage collect` などに、誰も読まない表示を足すことになる）。

14. **2026-11-22 状態を変えた葉は、何をしたかと次に打つ1手を出す。断るときは直す1手を出す。**（Chief Engineer が決めた）
    理由: clig.dev が求めている。Chief of Staff の手引きは、終了コードごとに次に打つ手を表で持っている（`package/skills/chief-of-staff/playbooks/start.md`）。葉が自分で言えば、その表は要らなくなる。
    細部: まず `issue create`・`issue start`・`issue resume`・`open`・`design start` に入れる。`issue start` が予約キーの不足で断るときに直す行を出す今の形が見本である。ほかの葉は、触るときに揃える。

15. **2026-11-22 終了コードは 0・1・2 の意味を揃え、3 以上は葉ごとに help に書く。フックが打つ葉は 2 を使わない。**（Chief Engineer が決めた）
    0 は成功、1 は失敗か断り、2 は使い方の誤りである。プロジェクトが決まらないのは使い方の誤りで、どの葉も 2 を返す。3 以上を使う葉は、表に数と意味を書き、`--help` に出す。
    理由: 0・1・2 は多くの葉で揃っているが、プロジェクトが決まらないときの数が割れている（事実 9）。3 以上を全体で振り直すと、gate と dispatcher の約束の数が変わる。読むのは機械だけで、オーナーの困りごとに効かない。
    細部: 例外は 2 つある。フックが打つ `dispatcher wait` と `rewake` は、終了コード 2 が合図の意味なので、使い方の誤りでも 0 を返す今の形のままにする。`schema check` の 2 は「検査できない」のままにし、help に書く。
    捨てた案: 3 以上も全体で1つの意味に振り直す。

16. **2026-11-22 人向けの文書は、生成した一覧 `docs/commands.md` の1枚だけにする。詳細は `--help` が持つ。**（Chief Engineer が決めた。詳細を書かないことはオーナーが決めた）
    `docs/commands.md` は `andon help --all` と同じ中身で、表から作る。葉ごとに1行で、打つ者と説明を出す。使い方やフラグは載せない。
    理由: オーナーが、人向けの文書に詳細は要らないと言った。セルの外の人は `andon` を打てないので、全体図を1枚だけ repo に置く。手で書くと、また実物とずれる。
    細部: 作るのは `andon help --all --markdown`。検査は、作り直して差分が無いことを見る。`docs/process/` を定義から作って検査で見る今の形と同じである。
    捨てた案: 葉ごとの詳しいリファレンスを `docs/` に置く（オーナーが要らないと言った）／文書を置かず `andon help` だけにする（セルの外の人が全体図を読めない）。

17. **2026-11-22 名前は群ごとに一度に替え、別名は作らない。古い名前を打つと、新しい名前を1行で言って終了コード 2 で断る。**（Chief Engineer が決めた。別名を作らないことは、オーナーが前に決めた）
    葉の名前を変える PR は、その葉を打つ文（skill・テンプレート・ホストの部品・検査・文書・流れの定義・モデルの対応表）を同じ PR で直す。古い名前は実行しない。
    理由: 別名を作らないのは ADR-0008 決めたこと 7 でオーナーが決め、ADR-0052 決めたこと 9 も同じにしている。古い名前で実行もすると、古い文がいつまでも残る。新しい名前を言うだけなら、古い skill の文を持ったまま動いている担当が、次の1手で直せる。
    細部: 古い名前と新しい名前の対は、決めたこと 8 の表の 18 個を表に持つ。`andon waiting` と `andon dispatcher` は、名詞だけの形が古い葉だった。この 2 つは、名詞だけを打つと help を出さずに、新しい名前（`andon waiting list`・`andon dispatcher run`）を言って断る。消す時期は決めない。持っていても害が無く、消す日を決めると忘れた頃に壊れる。替えない文字列は、gate のコメントの頭 `andon gate: ` と、boot のログの頭 `[andon boot]` である（事実 15）。`signal delivered` を打つのは dispatcher だけである。当てるときに dispatcher を起動し直せなかった間は、古い dispatcher が古い名前を打って断られ、同じ信号の通知が重ねて届くことがある。起動し直せば止まる。
    捨てた案: 古い名前を1バージョンだけ動かし、警告を出す（clig.dev の勧め。利用者が1人で、前例が別名を作らないと決めている）／古い名前を何も言わずに消す（動いている担当が、古い skill の文のまま打って止まる）。

18. **2026-11-22 `andon issue create` を足す。担当の課題を足すのと予約キーを書くのを1回で打つ。**（Chief Engineer が決めた）
    `kanban add` と同じ引数を取り、続けて予約キーを `key=value` で受ける。予約キーは schema で先に確かめ、予約キーでないキーか、知らない値があれば、何も足さずに断る。確かめが通ったら課題を足し、キーを書き、足した id と次に打つ `andon issue start <id>` を出す。
    理由: Chief of Staff は課題を切るたびに、`kanban add`、6つ前後のキーの `kanban set`、`andon issue start` の3回を打っている（事実 11）。キーの書き忘れは、担当が実装を終えたあとで止まる原因になっていた（決定の記録 2026-11-22「予約キーが揃っていない課題を起動せずに断る」）。
    細部: 書き手は打った端末になるので、gate の履歴の照合はそのまま通る。状態の動きは `kanban add` と同じで、新しい動きは足さない。`kanban add` と `kanban set` はそのまま残る。既定から変えた理由は `--reason <key>=<文>` で渡せる（決めたこと 19 が Settings 欄に写す）。
    捨てた案: `kanban add` に予約キーのフラグを足す（予約キーは Andon の決まりで、kanban の CLI は汎用のまま保つ。[ADR-0017](0017-process-definitions.md) 決めたこと 15）／`andon issue start` にキーのフラグを足す（足すのと起動するのが離れている場面で、キーの無い課題が kanban に残る）／足して起動までを1回で打つ（起動の前に、動いている課題とぶつからないかを Chief of Staff が見る段がある）／予約キーでないキーも受けて `kanban set` に渡す（綴りを誤った予約キーが通る。オーナーの課題を切る手順をまとめないという ADR-0042 の「やらないこと」にも触れる）。

19. **2026-11-22 依頼書の Settings 欄は command が予約キーから書く。**（Chief Engineer が決めた）
    `andon issue create`・`andon issue start`・`andon issue resume` が、`issue.yaml` の予約キーから依頼書の Settings 欄を書く。Chief of Staff は手で写さない。
    理由: 今は同じ値を、`kanban set` と依頼書の2か所に手で書いている。正は `issue.yaml` で、依頼書は人が読む写しである。手引きも、command が作るようになったらこの段を消すと書いている。
    細部: 欄の見出し、行の並び、値の書き方は、今の雛形（`package/skills/chief-of-staff/playbooks/brief.md`）のままにする。見込みは「小・中・大」と書く。既定から変えた行には印を付け、`--reason` があれば理由を添える。欄が無い依頼書には足す。予約キーを置かない課題（Chief Engineer の課題、レビュアーの課題）の依頼書は触らない。起動のあとに `kanban set` で変えた値は、次に起動するまで欄に出ない。欄を読む機械は無い。
    捨てた案: 依頼書から Settings 欄を無くし、担当に `andon issue show` を読ませる（オーナーが依頼書を読むときに、設定が見えなくなる）。

20. **2026-11-22 kanban の CLI は名前を変えない。足すのは、kanban の場所の探し方の最後の1段だけ。`hc` は案内だけにする。**（オーナーが決めた。Chief Engineer の推奨どおり）
    kanban は課題を読み書きする汎用の道具のまま残す。Andon の決まりを伴う操作（予約キー、役の起動、片付け）は andon が持つ。役は、同じ仕事の葉が andon にあるときは andon を打つ。kanban の場所の探し方に、「上に `projects/` があり、kanban が1つだけならそれ」を最後の段として足す。andon は今と同じく kanban の CLI に場所を聞くので、決めたこと 4 の「1つだけ」は kanban と andon で同じに効く。
    理由: kanban は help が1画面に収まり、役は誤らずに打っている（事実 5）。困りごとの元は kanban の名前ではなく、Andon の決まりである予約キーを `kanban set` で手で書かせていたことで、それは決めたこと 18・19 で無くなる。場所の探し方だけを揃えないと、セルの根で andon は動き、案内された先の kanban が断る。`hc` は外の製品で、変えられない。
    細部: kanban に `--project` は足さない。プロジェクトが2つ以上あるセルの根では、今と同じく `KANBAN_ROOT` で指すか、フォルダを移る。
    捨てた案: kanban の葉を全部 andon に取り込み、役とオーナーが打つ CLI を1つにする（`andon issue list`・`set` など約 20 の葉が増え、同じことを2つの CLI でできるようになる。skill の中の kanban の呼び出しを全部書き換える）。

21. **2026-11-22 配る文の中の andon のコマンドが表にあることを、検査で見る。**（オーナーが決めた。Chief Engineer の推奨どおり。[ADR-0001](0001-skills-distribution.md) 決めたこと 5 のうち、検査を入れないという部分を覆す）
    `package/skills/` の文、`package/hooks/` のスクリプト、`package/templates/` のテンプレートの中の、`` ` `` で囲んだ語とコードの囲みの中の `andon <名詞> <動詞>` を、同じ commit の表と照らす。表に無い葉があれば落とす。
    理由: 約 800 か所を書き換える（事実 14）。skill に古い名前が残ると役が1手遠回りし、フックやテンプレートに残ると黙って何もしなくなる。ADR-0001 が検査を入れなかった理由は、「schema の検査で約束の側は押さえてある」と「開発中に邪魔になる形を先に入れない」だった。葉の名前は schema の検査では押さえられず、同じ形の検査は検証の skill について既に入っていて、邪魔になっていない（事実 18）。
    細部: 見方は事実 18 の検査と同じにし、その検査とまとめる。古い名前を説明として挙げる所（用語集）は、行に印を付けて除く。文書（`docs/`）は見ない。ホストの部品は、引数を配列で渡すので、この検査では見えない。ホストの部品が打つ葉は 2 つだけなので、部品の検査で引数を見る。検査は、古い名前を1つ書き戻すと落ちることを確かめる。
    捨てた案: 検査を入れず、書き換えを grep で確かめる（次に葉の名前を変えるときに、また手で探すことになる）。

22. **2026-11-22 CLI の枠組みは入れず、表は自作する。**（Chief Engineer が決めた）
    理由: [design.md](../design.md) §2.7 が、CLI の枠組みを入れず依存を最小にすると決めている。要るのは、表からの振り分け、help の3群、一覧と補完の生成、綴りの近い葉の勧めで、数百行で書ける。葉の中の引数の読み方は変えないので、約 10,000 回打たれている形（事実 5）を壊さない。
    捨てた案: cobra に載せ替える（事実 17。依存が 4 つから 8 つに増える。全部の葉の引数の読み方を書き直すことになり、help の雛形も日本語に作り直す。得るのは補完と文書の生成で、どちらも表から作れる）。

23. **2026-11-22 ADR と決定の記録の中の古い名前は替えない。**（Chief Engineer が決めた）
    理由: 過去は直さないという [ADR-0026](0026-pr-rules-for-outside-readers.md) 決めたこと 10 のままにする。課題への改名（ADR-0008）も、ADR と決定の記録は替えなかった。古い文書を読んだ役が古い名前を打っても、決めたこと 17 の断りが新しい名前を言う。
    捨てた案: ADR と決定の記録の約 350 か所も替える（語を替えたときは、オーナーがそのつど例外として決めた。コマンドの名前は、断りが直す道を持つので、例外にする理由が無い）。

## やらないこと（範囲の外。理由）

- **状態を動かす流れを変えること。** `andon triage apply` と `andon retro apply` が片付けと起動まで回すこと、Chief of Staff のアドレスが変わったときに予約キーを書き直すこと、dispatcher が届く行ごとに打つ1手を持つことは、流れを変える。モデルで調べる別のテーマにする。
- **Chief of Staff をセルに1人にすること。** ADR-0031 の検討で決める。この ADR の名前と決まりは、どちらに決まっても使える（決めたこと 3・4・6）。
- **オーナーの課題を切る手順をまとめること。** [ADR-0042](0042-owner-waits-do-not-stop-the-line.md) の「やらないこと」のままにする。`andon issue create` は予約キーだけを受ける（決めたこと 18）。
  → 2026-11-23 に、[ADR-0057](0057-owner-issue-decision-or-hand-work.md) 決めたこと 7 が、この 1 行を覆した。オーナーの課題と問いは `andon issue ask --new` の 1 回で置く。`andon issue create` が予約キーだけを受けることは変わらない（オーナーが決めた）。
- **`andon triage verdict` で判定をまとめて打つこと。** 1件ずつ command で書くと [ADR-0030](0030-triage-close-safety-net.md) が決めている。繰り返して打てば同じで、覆すほどの得が無い。
- **セルに main を当てる道具を andon の葉にすること。** 当てる commit から取り出したスクリプトで当てるという [ADR-0027](0027-apply-package-without-rebuild.md) 決めたこと 5 の理由は、今も成り立つ。
- **Chief Engineer の端末の問いを `andon waiting list` に出すこと。** ADR-0043 が、本文の問いを kanban に置く形で決めた。
- **kanban の CLI の葉を足す・名前を変えること。** 決めたこと 20。
- **英語の help。** 読む者はオーナーと日本語で動く役である。
- **3 以上の終了コードの振り直し。** 決めたこと 15。
- **葉の中の引数の読み方を書き直すこと。** 決めたこと 22。変えるのは、決めたこと 13 のフラグを足す所だけである。
- **接頭字の重なりを `project create` が断ること。** 決めたこと 4 で接頭字を使わない。ADR-0031 の案 6 と一緒に決める。

## 受け入れ条件（テストできる形。条件ごとに、走らせる command と見る出力）

テストする継ぎ目は3つにする。表そのもの（`go test ./internal/cli`）、焼いたバイナリを一時のセルで打つシェルの検査（今ある `tests/andon*.sh` と、足す `tests/commands.sh`）、セルの本物で動かす検証の課題である。

- A1 表の全部の葉が2語で、1語は `open`・`help`・`version`・`boot` だけである。`go test ./internal/cli` の表を回す検査が通る。3語の葉を表に足すと落ちる。
- A2 表に載った葉は全部 `andon <名詞> <動詞> --help` と `-h` が終了コード 0 で使い方を出し、載っていない2語は終了コード 2 になる。名詞だけを打つと、その名詞の葉の一覧が出る（決めたこと 17 の 2 つを除く）。綴りを1字誤った葉を打つと、正しい葉を含む1行が標準エラーに出る。`go test ./internal/cli` で見る。
- A3 決めたこと 8 の表の古い名前 18 個は、どれも打つと新しい名前を含む1行を標準エラーに出し、終了コード 2 になる。実行はしない。`go test ./internal/cli` で、古い名前ごとに見る。
- A4 `andon help` の出力に、表で主に打つ者がオーナーの葉が全部あり、主に打つ者が機械の葉が1つも無い。`andon help --all` には表の全部の葉がある。`bash tests/andon.sh` で見る。
- A5 どこで打っても動く。`bash tests/commands.sh` が一時のセルにプロジェクトを2つ作り、次を見る。セルの根で `--project a` を付けると a の kanban を読む。`projects/a/` では省いても a を読む。プロジェクトが1つのセルでは、根で省いても読み、`kanban list` も同じ kanban を読む。2つのセルの根で省くと、終了コード 2 と2つの slug が出る。プロジェクトを読む葉の全部（表の欄で選ぶ）について、`--project` を受けることを見る。
- A6 JSON を出せる葉の全部（表の欄で選ぶ）が `--json` を受け、標準出力が JSON として読める。`bash tests/commands.sh` が葉ごとに `jq -e .` に通す。
- A7 `docs/commands.md` と bash の補完のファイルは、作り直しても変わらない。`bash tests/commands.sh` が、表から作り直したものと比べる。表の説明を1つ変えて作り直さないと落ちる。
- A8 `package/skills/`・`package/hooks/`・`package/templates/` の中の `andon <名詞> <動詞>` は、全部が表にある。`bash tests/commands.sh` で見る。skill に古い名前を1つ書き戻すと落ちる。ホストの部品が打つ引数は、部品の検査で `waiting list --json` と `waiting answer` であることを見る。
- A9 `andon issue create "<タイトル>" --repo <repo> review=off estimate=small must_fix=major merge=person` は、課題を1件足し、予約キーを書き、足した id と `andon issue start <id>` の行を出す。続けて打つ `andon issue start <id> --dry-run` が、予約キーの理由で断らない。`review=maybe` のような知らない値か、`who=person` のような予約キーでないキーを渡すと、課題を足さずに終了コード 1 で断る。`bash tests/andon-issue.sh` で見る。
- A10 `andon issue create` のあと、依頼書の Settings 欄の行が、予約キーを今の雛形の書き方で写したものと合う（`estimate=medium` なら `- Estimate: 中`）。`kanban set` で値を変えてから `andon issue start` を打つと、欄が新しい値になる。Chief Engineer の課題の依頼書は変わらない。`bash tests/andon-issue.sh` で見る。
- A11 プロジェクトが決まらないとき、プロジェクトを読む葉はどれも終了コード 2 を返す。3 以上の終了コードを使う葉は、`--help` にその数と意味が出る。`dispatcher wait` と `rewake` は、知らない引数でも 0 を返す。`go test ./internal/cli` と `bash tests/commands.sh` で見る。
- A12 `issue create`・`issue start`・`issue resume`・`open`・`design start` は、成功したときに何をしたかの1行を出し、続きがあれば次の葉を出す。`issue start` が予約キーの不足で断るときは、直す行を出す。`bash tests/andon-issue.sh` などで、出力に次の葉の文字列があることを見る。
- A13 boot とフックは、端末の前に出ているものが `andon dispatcher` でも `andon dispatcher run` でも、dispatcher が動いていると読む。`go test ./internal/dispatcher` で見る。
- A14 `andon boot` と `andon issue session` は、名前を変えたあとも今と同じ引数で動く。`bash tests/andon-boot.sh` と `go test ./internal/issue` で見る。
- A15 セルの本物で、名前を変えたあともラインが回る。オーナーが見る（manual）。検証の課題の担当が、main を当てたセルで次を動かして出力を報告に貼る。Chief of Staff のターンが終わると待ちの通知が届く。担当が `issue submit` で review に入り、gate が回ってマージまで進む。コンテナを起動し直しても boot が走る。当てたものを戻す道具が通る。ホストの部品を入れ直したあと、オーナー待ちが App に出る。

## 価値の仮説

1. **誰の何が変わるか。** オーナーが、コマンドを打つときと AI の打ったコマンドを読むときに、名前から何をするかが分かり、どこで打つかを考えなくて済む。`andon help` の最初の画面が、自分の打つ葉だけになる。
2. **見る信号。** 信号の記録は無い。1つはオーナーの目視で、見る日に感想を1行もらう。もう1つは、観察の課題の担当がセルのセッションの記録から数える数である。数えるのは、当てたあと7日間に役が打った andon の回数、使い方の誤り（終了コード 2）の回数、古い名前を打って断られた回数、Chief of Staff が1回の `kanban set` で予約キーを6つ以上書いた回数である。
3. **見る日。** 2026-12-13。その日に「依頼書に切る案」の 1〜7 がセルに当たって7日たっていなければ、当たってから7日後。
4. **誰が要るか。** オーナー（感想を1行。当てたあとにホストの部品を入れ直す）と、観察の課題の担当。
5. **効いたとどう分かるか。** オーナーが、何をするコマンドか分からない、どこで打つのか分からない、と言わない。当てて3日目からの4日間で、古い名前を打って断られた回数が 0 である。使い方の誤りが、直す前の割合（約 10,000 回に 9 回）の2倍を超えない。Chief of Staff が1回の `kanban set` で予約キーを6つ以上書いた回数が 0 である（直す前は5日で 97 回）。
6. **何があれば止めるか。** 当てたあと、通知が届かない、gate が回らない、コンテナが立ち上がらないなど、ラインが止まる。そのときは当てたものを戻し、原因を調べる課題を切る。または、オーナーが新しい名前のほうが分かりにくいと言う。そのときは、その葉の名前だけを選び直すかをオーナーと決める。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

`package/` の skill・フック・テンプレート・ホストの部品と、モデルの対応表を変える PR は、オーナーが見る（事実 19）。3・4・5・7 はそれに当たる。

1. **表と help。名前は変えない。** 葉の表を作り、振り分けと `andon help`（3群・例・`--all`・近い葉の勧め）と `docs/commands.md` と bash の補完を表から作る。配る文を表と照らす検査を足し、検証の skill の検査をそれにまとめる。決めたこと 11・12・16・21。A1・A2・A4・A7・A8。依存は無い。
2. **どこで打っても動く。** kanban の場所の探し方に最後の1段を足し、プロジェクトを読む葉の全部に `--project` を通し、決まらないときの終了コードを 2 に揃える。使い方の文から「`projects/<slug>/` で打つ」を消す。決めたこと 3・4・15・20。A5・A11。1 のあと（表の欄を使う）。
3. **オーナーが打つ葉の名前を替える。** `open`（`chief-of-staff` を寄せる）・`design start`・`project create`・`project list`・`waiting list`・`delegation`・`metrics show`・`cell init`。古い名前の断りを足す。ホストの部品と入れ直しの手順を同じ PR で直す。決めたこと 8・9・17。A3・A8・A15。1 のあと。「題」をタイトルとテーマに替える作業（ADR-0052）が使い方の文を触るので、その PR のあとにする。セルの入口を薄くする検討が並んで動いているので、`cell init` はその結果と突き合わせてから切る。
4. **役が打つ葉の名前を替える。** `issue submit`・`issue gate`・`file publish`。skill と流れの定義とモデルの対応表を同じ PR で直す。決めたこと 8・17。A3・A15。1 のあと。
5. **機械が打つ葉の名前を替える。** `dispatcher run`・`signal mark-delivered`・`cell serve-files`・`package stamp`。dispatcher が動いているかの見方を両方の形に広げる。決めたこと 8・9・17。A3・A13・A14・A15。1 のあと。
6. **フラグと出力を揃える。** `--json` を JSON を出せる葉の全部に通し、3 以上の終了コードを help に出し、5 つの葉に次の1手を出させる。決めたこと 13・14・15。A6・A11・A12。1 のあと。
7. **担当の課題を1回で切る。** `andon issue create` と Settings 欄の生成。Chief of Staff の手引きの二度書きの段を消す。決めたこと 18・19。A9・A10。1 のあと。
8. **観察。** 価値の仮説の 2・5 を数える。1〜7 がセルに当たって7日後。

3・4・5 を当てるときは、動いている担当が古い skill の文を持っている。古い名前の断り（A3）が次の1手を言うので、止めずに当ててよい。当てたら dispatcher を起動し直す（今の道具がしている）。

## 開いている問い

無し。

## 出典

- オーナーの発言 2026-11-22（Chief of Staff の端末と、Chief Engineer との対話）。2026-11-21 の発言は Chief of Staff の端末。
- Command Line Interface Guidelines https://clig.dev/ （2026-11-22 に全文を読んだ）。
- repo: `internal/cli/cli.go`、各パッケージの使い方の文、`package/hooks/`、`package/templates/`、`package/host/`、`package/skills/chief-of-staff/playbooks/`、`init/bin/bootstrap`、`image/apply-main.sh`、`image/completions/andon`、`tests/verify-andon.sh`、`model/issue-map.txt`、`go.mod`。
- 過去の決定: [ADR-0001](0001-skills-distribution.md) 決めたこと 5、[ADR-0008](0008-issue-naming.md) 決めたこと 7、[ADR-0026](0026-pr-rules-for-outside-readers.md) 決めたこと 10、[ADR-0030](0030-triage-close-safety-net.md)、[ADR-0031](0031-cell-chief-of-staff.md)（提案）、[ADR-0017](0017-process-definitions.md) 決めたこと 15、[ADR-0027](0027-apply-package-without-rebuild.md) 決めたこと 5、[ADR-0042](0042-owner-waits-do-not-stop-the-line.md)、[ADR-0043](0043-lamp-lights-on-owner-waits.md)、[ADR-0048](0048-thin-cell-entry.md) 決めたこと 11、[ADR-0051](0051-signals-flow-and-breaking-change.md)、[ADR-0052](0052-title-and-theme-not-dai.md) 決めたこと 9、[design.md](../design.md) §2.7、決定の記録 2026-11-22「予約キーが揃っていない課題を起動せずに断る」。
- 数は、セルのセッションの記録（2026-11-18 から 2026-11-22）と repo の grep で数えた。
