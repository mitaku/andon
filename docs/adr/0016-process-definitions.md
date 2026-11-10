# ADR-0016: 仕事の流れと状態を定義し、その上で見張り役（dispatcher）を細くする（仕様・設計。2026-11-10。Chief Engineer の課題 A-0085）

- 状態: **提案（2026-11-10。下書き）**。対話はオーナーが寝ている間に Chief of Staff と進めた。オーナーの決断に当たる項目は「仮（Chief of Staff の推奨。オーナー未確認）」と記して「決めたこと」に置き、オーナーが確かめるまで確定としない。オーナーが「オーナーに確かめること」を順に確かめ、「決めた」と言った日が決定の日。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: Chief of Staff の定常の仕事（ワークストリーム）と、もの（課題・担当・通知・レビュアーの起動・人のマージ・外のお知らせ）の状態の移り方を、人と機械が読める定義として1か所に書くこと。その定義から「機械がやる手順」を独立したコマンドに出し、dispatcher に残すのを「定義に沿っていつ起こすか」と「通知を配る」だけにすること。
- 語は glossary skill と [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。この文書が足す約束（定義のファイルと schema・出すコマンド・記録ファイルの分け方）は「決めたこと」に形を書き、実装の課題が写す（それまでは提案）。
- 入力: 指示文 P11（オーナー。2026-11-10。セルの `files/p11-process-definitions.md`。repo の外）、オーナーの言葉（2026-11-10。「問題」の段に写し）、[dispatcher.md](../dispatcher.md) §2.1.1・§2.2〜§2.8・§3.2・§5・§7、[contract.md](../contract.md) §2・§6・§7・§15、[ledger.md](../ledger.md)（Status rules・Routines・Questions）、chief-of-staff skill §0〜§10、worker skill §2・§5・§7、[ADR-0007](0007-interim-gap-inventory.md)（C1 `kanban tick`・A5〜A7・G4）、[ADR-0010](0010-news-ack.md)（届ける／記録だけ）、[ADR-0011](0011-signals-and-value-loop.md)（外のお知らせ。決めたこと 5・6・8・16、S2・S3、Factory の常駐を外した理由）、[ADR-0013](0013-short-role-names-and-plain-japanese.md)（人に向けた言葉）、[ADR-0014](0014-issue-estimate.md)（見込みとズレ）、`internal/dispatcher/`（state.go・loop.go・pickup.go ほか。v0.0.8 相当の main `98383a6`）、台帳（A-0026 S3 は `after: A-0085` で止めてある。dispatcher を触る課題 A-0036・A-0039・A-0040・A-0062・A-0063・A-0066・A-0067・A-0070・A-0071）、[ADR-0012](0012-resume-and-worktree-home.md)（resume の R3・R4）。

## 問題（人の言葉で。言い直しの段落）

オーナー（2026-11-10）: 「dispatcher の役割の肥大化というか決定論化は今後の課題かもね」「CoS のワークストリームを定義しておくといいかもね」「さらに細かくは SKILL 化するなどもあるが、BPM を活用したいところである」「BPM と、ステートマシンあたりが明文化されていれば改善にも役に立ちそう」「BPM 自体は JSON でも YAML でもいい。コメントの残しやすさなど運用のしやすさから考えてもいい。機械用は JSON、人間用は Markdown とかでもよい」。担当がとても長く作業を続けている場合に、どういう仕組みにするのがあるべき姿か、も考えたい。

言い直し（Chief of Staff が 2026-11-10 に「合っている」と言った。オーナーの確かめは朝）。オーナーが困っているのは、Chief of Staff の定常の仕事（受付・仕分け・起動・見守り・受け取り・取り次ぎ）と、課題や担当といった「もの」の状態の移り方が、どこにも定義として書かれていないこと。定義が無いので、chief-of-staff skill の §1〜§10 と dispatcher.md と dispatcher の記録ファイルの項目が、黙ってその代わりを背負っている。その結果、改善したいときにどこを直せばよいか指せず、機械に任せる仕事が増えるたびに dispatcher が受け皿として太り続ける（テスト以外 15 ファイル・約 4,000 行・1巡が 12 段・1つの記録ファイルに 8 種類の状態が混ざる。指示文の「13 ファイル・約 5,500 行」はテストを含めた数に近い）。欲しいのは、仕事の流れ（ワークストリーム）と、ものの一生（ステートマシン）を人と機械が読める定義として1か所に書き、その定義から「機械がやる手順」を独立したコマンドに出して、dispatcher に残すのを「いつ起こすか」と「通知を配る」だけにすること。定義は改善の起点（定義する → 動かす → 測る → 直す）であって、外の BPM の仕組みを入れる話ではない。

## 前提（同意済み。番号。Chief of Staff が 2026-11-10 に 1〜4 全部に同意。オーナーの確かめは朝）

1. **問題の所在**: 困りごとの芯は「仕事の流れと状態の定義が無いこと」で、dispatcher の行数の多さそのものではない。行数と手順の数は症状で、定義を書かずに行数だけ減らしても（コードを分けるだけでは）、次に任せる仕事が来たときにまた同じ所へ積まれる。
2. **何もしないと**: 台帳に並ぶ dispatcher の課題（外のお知らせ S3〈A-0026〉・見込みの通知 E3〈A-0062〉・`kanban tick` R1〈A-0066〉・人のマージの誤報 R2〈A-0067〉・自動 resume R4〈A-0040〉・lock R7〈A-0071〉）がこのまま進むと、全部が dispatcher の1巡と1つの記録ファイルに積まれる。1巡の段は 12 を超え、記録の項目は 19 を超えて増え続け、dispatcher.md §2.1.1 の表と実装を一緒に直す約束が改善のたびに重くなる。定義が無いので、どの段がどの仕事に属するかを後から指せない。
3. **既にあるもので済む部分**: 新しい役も新しい常駐も作らない。次は既にあり、定義はそれを指すだけにする。(a) 課題のステータス（`todo`・`queued`・`doing`・`review`・`done`）と問いの状態（`open`・`answered`・`received`）は [ledger.md](../ledger.md) に定義がある。(b) 外のお知らせの記録（`active`・`episodes`・`delivered_episode`）と届ける／記録だけの表（`internal/dispatcher/route.go`）は ADR-0011・ADR-0010 で形が決まっている。(c) 見守りの芯（見込みの段・1・2・4 倍で Chief of Staff にだけ届ける・担当には送らない）は ADR-0014 で決まっている。(d) 既にコマンドの形のもの（`andon gate`・`andon issue impact`・`andon issue finish`・`andon signal collect`）はそのまま。
4. **配り方と役**: 定義の形と既定は andon の repo（`docs/`・`schema/`）に置き、パッケージの版上げで全セルに届く。プロジェクトごとの値（見込みの段の分など）は `project.json`。ラインそのものの変更なので、定義と dispatcher を触る PR はレベル C（人がマージ）。判断を持つのは今までどおり Chief of Staff と人で、コマンドと dispatcher は判断しない。

### 読んだ事実（2026-11-10。main `98383a6`。読むだけのサブエージェントが `internal/dispatcher/` を洗った）

- dispatcher はテスト以外 15 ファイル・3,987 行（Go のテスト 2,022 行、`tests/andon-dispatcher.sh` 1,126 行）。指示文の「約 5,500 行」はテストを含めた数に近い。
- 1巡の本体は `internal/dispatcher/dispatcher.go` の `pass`（12 段）: (1) Chief of Staff の端末を `hc get`。無ければ**丸ごと飛ばす**（lock も控えも触らない）→ (2) 控えの lock → (3) 控えを読む → (4) 台帳と `log.jsonl` を1回読む → (5) 拾う（ready の差・log の続き・担当の端末・Chief of Staff の端末）→ (6) 課題ごと（`doing` の `review_request` → レビュアー、`review` → ゲートと人のマージ）→ (7) 人のマージの控えを捨てる → (8) 影響 → (9) 手直し → (10) approve のまま止まった → (11) 配る（経路③）→ (12) 控えを保存。
- 記録ファイル（`~/.local/state/andon/dispatcher/<slug>.json`。`state.go` の `State`）は 19 項目で、8 種の状態が混ざる: 配達（`Pending`・`Coord`・`NewsTurn`・`Hook`・`Replied`・`Unrecorded`）、レビュアーの起動（`Reviewers`。`Started`・`Skipped`・`Failed`・`Attempts`・`OverLimit` が別々の項目）、ゲート（`Gates`・`Approved`）、人のマージ（`Watch`）、影響（`Impact`）、手直し（`Corrections`）、ログの位置（`LogPos`・`LogHead`・`LogHold`）、担当の見張り（`Ready`・`Sent`・`Retry`・`First`）。`Reviewers` と `Gates` は消す所が無く、終わった課題の分も溜まり続ける。
- 時間で回るもの（知らせに反応しない）: 見回りの間隔（1 分・busy なら 5 秒）、人のマージの `gh pr view`（PR ごと 1 分）、approve のまま止まった（10 分）、stalled（15 分・20 分）、「続けて」（1・2・4・8 分）、影響・手直しの打ち直し（次の巡。3 回・2 回）。`kanban tick`・routines・schedule の記述は dispatcher の Go に無い。
- 外のお知らせ（`signal`）: dispatcher の Go に取得も配達も無い。`route.go` の表に無い kind は既定で「届ける」。
- lock は控えの lock（`<slug>.json.lock`）1つを巡の全段で持つ。`drain`（フック）も同じ lock を 10 秒待つ。
- `kanban tick` はルーチンの `schedule` を撃って**課題を1件足す**だけ（ledger.md の Routines）。ルーチンにコマンドを打つ道は無い。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

「仮」と記した項目は Chief of Staff の推奨（2026-11-10 夜。オーナー未確認）。オーナーが朝に確かめて「決めた」と言うまで、確定ではない。確かめの一覧は次の節。

1. **仮 2026-11-10 定義の形は案 C: YAML を正本にし、JSON Schema で検査し、人が読む Markdown（表と Mermaid の図）はコマンドが YAML から作り、定義と実装の食い違いはテストで見る。dispatcher は定義を読まない。**（指示文の推奨のまま。Chief of Staff が選んだ）
   理由: 機械が読むのは YAML だけ、人が読むのは生成した Markdown だけ、で「1ファイル1読み手」を保てる。テストが定義と実装を照らすので、黙ってずれない。
   捨てた案: 案 A（Markdown の表と図だけを人が書く。機械は読まずテストも無い。定義と実装がまた黙ってずれる）／案 B（YAML を dispatcher がランタイムで読み、タイマーと振り分けを動かす。定義がランタイムの入力になり、セルごとの差と読み手が増える。指示文も「後で決める」）。
2. **仮 2026-11-10 置き場: YAML は repo の根の `process/`（`process/workstreams/<名>.yaml`・`process/states/<名>.yaml`）、schema は `schema/process/*.schema.json`、生成した Markdown は `docs/process/<名>.md`。プロジェクトごとの値は `project.json`。**（Chief of Staff が選んだ）
   理由: 読み手ごとに置き場が分かれる（`docs/` は人、`schema/` は機械、`package/` はセルに配るもの）。dispatcher が後で定義を読む（案 B）ときは `package/` へ写すだけ。
   捨てた案: `docs/process/` に YAML と生成 Markdown を並べる（`docs/` に機械が読むファイルが混ざる）／`package/process/` に置いてセルに配る（今は読まないのに配ることになる）。
3. **仮 2026-11-10 ワークストリームは 8 本（受付・仕分け・起動・見守り・受け取り・取り次ぎ・結果を見る・ふりかえり）。名前と目的を一覧に置き、この題で中身を書くのは見守りと仕分けの 2 本。残り 6 本は同じ形で後から。**（Chief of Staff が選んだ）
   理由: きっかけが 1 本 1 種類（人の依頼・外のお知らせと間隔・通知・間隔・報告と合否・問い・見る日・終わりと週 1）になり、改善のとき指しやすい。今の skill との対応: 受付と起動＝§1〜§3、取り次ぎ＝§4・§8、見守り＝§5、受け取り＝§6・§7、仕分け＝§10、ふりかえり＝§11 と finish の時、結果を見る＝ADR-0011 の観察の課題。
   捨てた案: 6 本にまとめる（受付＋起動、受け取り＋取り次ぎ。ファイルは減るが、きっかけが混ざる）。
4. **仮 2026-11-10 ものの一生（ステートマシン）は 8 本。記録ファイルの 19 項目はどれか 1 本の状態か「拾う位置」に対応し、対応表を定義の中に持つ。**（Chief of Staff が選んだ。指示文の「まず課題と担当」＋手順 2 の 4 本を広げた）
   (1) 課題: 台帳のステータスと問いは [ledger.md](../ledger.md) を指すだけ。Andon が足すのは「課題の型と工程」の層（`deliverable`・レビュー依頼・ready・ゲート・片付け）。
   (2) 担当の端末: hc の状態（`idle`・`working`・`blocked`・`error`・`done`・無い）の上に dispatcher が置く判定（最初のメッセージ未達・「続けて」N 回・stalled・見込みの段〈ADR-0014〉・Gone を見た時刻〈ADR-0014 E3〉・resume した〈ADR-0012 R4〉）。記録の `Sent`・`Retry`・`First`（P2b で 1 つの map に畳む）。
   (3) 通知の配達: 待ち → 渡した（経路①②③）→ 確かめた（ack）。記録の `Pending`・`Coord`・`NewsTurn`・`Hook`・`Replied`・`Unrecorded`。
   (4) レビュアーの起動: 無し → 起こし中（worktree → 子課題 → 端末 → 最初のメッセージ）→ 起こした／見送った（理由つき）／失敗 N 回／上限で別ブランドに替えて起こし直す（`fallback.go`）。記録の `Reviewers`（`Started`・`Skipped`・`Failed`・`Attempts`・`OverLimit` を状態に畳む）。
   (5) ゲートとマージ: `review` の行ごとに 待つ → approve／fix／needs_human → マージした → 片付けた（approve のまま 10 分止まった、を含む）。記録の `Gates`・`Approved`（決めたこと 6 の (2) の記録へ）。
   (6) 人のマージの見張り: 見ている（1 分に 1 回）→ MERGED／CLOSED → 片付けた／片付けられない（理由。同じ理由は 1 回）。記録の `Watch`。
   (7) 片付けのあとの仕事（影響と手直し。同じ形）: 待ち → 打ち直し N → 済み／諦めた（通知）。記録の `Impact`・`Corrections`。
   (8) 外のお知らせ: ADR-0011 決めたこと 8・16（`active`・`episodes`・`delivered_episode`）を形にするだけ。記録は `<slug>.signals.json`（`internal/signal` の構造体のうちこの 3 つが状態。残りは出所のデータと位置）。
   理由: `<slug>.json` の項目は (2)〜(7) のどれか 1 本の状態か「拾う位置」（`LogPos`・`LogHead`・`LogHold`・`Ready`。状態ではないので配達の記録の側に置く）に対応し、1 本も残さなければ「記録の項目がどの状態か」を全部指せる。(1) と (8) は `<slug>.json` に項目を持たない。
   捨てた案: 指示文の 4 本（レビュアー・配達・人のマージ・外のお知らせ）＋課題と担当だけ（ゲートと片付けのあとの項目に対応が無いまま残る）。
5. **仮 2026-11-10 記録ファイルを分ける。コマンドに出す仕事はそれぞれ自分の記録を、同じ置き場（`~/.local/state/andon/dispatcher/`）に別ファイルで持つ（書き手はそのコマンドだけ）。dispatcher の `<slug>.json` に残すのは配達の項目（`Pending`・`Coord`・`NewsTurn`・`Hook`・`Replied`・`Unrecorded`）と拾う位置（`LogPos`・`LogHead`・`LogHold`・`Ready`）、担当の端末の判定（`Sent`・`Retry`・`First` と ADR-0014 E3・ADR-0012 R4 の控えを 1 つの map に畳む）、留守の始まり（決めたこと 8）だけ。**（Chief of Staff が選んだ）
   理由: 「1つの約束に書き手は1人」を記録の中にも当てる。lock もファイルごとになり、巡の全段で1つを持たなくてよい。担当の端末の判定は hc を読むだけの判定で、別コマンドに出すと往復（別プロセスからの `hc get`）が増えるので拾う側に残す。
   書き手を 1 人に保つ細部（反論から。2026-11-10）: (a) `log.jsonl` の `done` の行を拾うのは dispatcher だけで、コマンドには `<id>` を引数で渡す（拾う位置を 3 重にしない）。(b) コマンドは出す通知を stdout の JSON で返し、`Pending` に積むのは dispatcher だけ（コマンドは `<slug>.json` に触らない。dispatcher がコマンドを同期で打つ間に同じ lock を取り合って止まらない）。(c) `<slug>.signals.json` の `delivered_episode` も dispatcher が直に書かず、`andon signal` の口（仮 `andon signal delivered <source> <episode>`）で書く。(d) settle は dispatcher の控え（`Gates[].Merged`）を読まず、ゲートの記録 `library/gate/<head>.json` の `merged` を読む（同じ巡の中で控えが保存される前に打たれても誤報にならない）。
   捨てた案: 1ファイルのまま項目を整理する（書き手が複数になる）／担当の端末の判定もコマンドに出す（往復が増える）／コマンドが `<slug>.json` の待ちに直に積む（書き手 2 人・lock の取り合い）。
6. **仮 2026-11-10 コマンドに出すのは 5 つ。小さく自己完結な順に 1 課題ずつ: (1) 人のマージを拾って片付ける（仮 `andon issue settle`）→ (2) ゲートのあとのマージと片付け（仮 `andon issue merge`。`andon gate` を打ち、approve と委譲の範囲なら `gh pr merge` と `finish`。記録の `Gates`・`Approved` はこの記録へ）→ (3) 手直しを集める（仮 `andon issue corrections`）→ (4) 影響の打ち直しと諦め（`andon issue impact` はもうコマンド。残るのは回数の記録だけ。回数は dispatcher が打った分だけ数える別の口〈仮 `--retry`〉で、Chief of Staff が読むために手で打った分は数えない）→ (5) レビュアーを起こす（一番大きい。使用量の上限で別ブランドに替えて起こし直す `fallback.go` と最初のメッセージの送り直しも同じコマンドと記録に入れる。ADR-0012 の resume R3〈A-0039〉・R4〈A-0040〉のあと）。「approve のまま 10 分止まった」はコマンドにしない（届ける時機の判定で、配る側が merge の記録を読んで出す）。どのコマンドも 2 回打っても同じ結果になり、自分の記録を自分で持つ（決めたこと 5）。**（Chief of Staff が選んだ。5 つ目は反論 B2 を受けて 2026-11-10 に足した）
   理由: S3 に要るのは手順 2 の定義だけで、コマンドの切り出しは要らない（`andon signal collect` はもうコマンド）。小さいものから出して記録の形を固め、一番大きいレビュアーは resume の課題とぶつからない順で最後。マージを出さないと `Gates`・`Approved` に持ち主が無く、主題の「dispatcher に残すのは『いつ起こすか』と『通知を配る』だけ」が偽になる。
   捨てた案: レビュアーを先に（loop.go の 722 行が先に減るが、resume の課題とぶつかる）／定義（手順 1・2）だけで止めて出すのは別の題（指示文の「この題で入れる: 手順 1〜3」と違う）／ゲートのあとのマージを dispatcher に残し主題と目安を直す（反論 B2・F3。持ち主の無い記録が残る）／ADR-0007 R2〈A-0067〉の誤報を settle に吸収する（直す所は approve の道で settle の外。反論 I1。R2 は単独で先に流す）。
7. **仮 2026-11-10 ルーチンは「課題を足すもの」のまま。間隔で回る機械の仕事（`kanban tick`・`andon signal collect`・人のマージの見張り・stalled・見込みのズレ）は、ワークストリーム定義の「きっかけ」に間隔つきで書き、dispatcher の 1 巡を「定義のきっかけ表のうち時が来たものを打つ」だけにする（tick もその 1 つ。打つのは dispatcher だけ）。**（指示文の推奨「何をいつ回すかはルーチンに書く」と違う。Chief of Staff が選んだ）
   理由: 台帳の形式（ledger.md・`kanban`）を変えない。ADR-0011 決めたこと 5（取得は dispatcher の巡で）・ADR-0007 R1〈A-0066〉（tick は dispatcher が打つ。G4 はこれで閉じる。まだ入っていない）と合う。「いつ回すか」が定義の 1 か所に集まる。
   細部（反論から。2026-11-10）: (a) `andon signal collect` の間隔は ADR-0011 決めたこと 5 のまま出所ごとに `project.json` の `signals[].every`。きっかけ表の行は「`signals[].every` を見る」と指すだけで、YAML に数を書かない（間隔の置き場を 2 か所にしない）。(b) R1〈A-0066〉は単独で先に流し、P11 は「tick の行をきっかけ表に移す」だけにする（反論 I2。吸収すると P10 の鎖の最後に回り、ルーチンが 1 つも動かず、R7 の lock も遅れる）。(c) [ledger.md](../ledger.md) の Routines の「The yard runs `kanban tick` every 5 minutes (a cron entry)」の 1 文だけは「dispatcher が打つ」に直す（形式ではなく誰が打つかの文。やらないこと 8 の例外）。(d) ADR-0011 受け入れ条件 9 の「§2.1.1 の順の表に collect の段がある」は、P11 のあとは「きっかけ表の行がある」と読む（ADR-0011 に下から足す）。
   捨てた案: ルーチンに「コマンドを打つ」種類を足す（台帳の CLI がコマンドを実行する側になり、ADR-0011 決めたこと 5 を覆す）／ルーチンが「取りに行く課題」を足して担当が打つ（ADR-0011 が捨てた案。読むだけの仕事に担当のターンを使う）／間隔を YAML に写す（`project.json` と二重）。
8. **仮 2026-11-10 「Chief of Staff の端末が無ければ飛ばす」は配る段と、ゲートのあとのマージと片付け（`andon issue merge`）にだけ掛ける。拾う・記録する・ほかのコマンドを打つ段（`kanban tick`・レビュアーの起動・ゲートの判定まで・人のマージの拾い・影響・手直し・`andon signal collect`）は Chief of Staff が居なくても回し、届ける通知は待ちに積む。戻ったときは今と同じ 1 通の `<news>` で、先頭に「留守の間（いつからいつまで）に N 件」の 1 行を足すだけ。要約する小さい AI（下読み役）は入れない。**（Chief of Staff が選んだ。Chief Engineer の推奨は「マージと finish も留守で進める（委譲は人が出したもので Chief of Staff の在席は条件でない）」だったが、Chief of Staff は「不可逆な操作は留守にしない」を取った。オーナーの確かめ 1 問）
   理由: Chief of Staff は起きたときに台帳を自分で読む（skill §0）ので、届くのは「判断が要るもの」だけで足りる。新しい役も形も足さない。ADR-0007 R1 の「Chief of Staff が生きているときだけ tick」は ADR-0007 に下から改める（留守でも tick）。
   細部（反論から。2026-11-10）: (a) `chief_of_staff` が設定に無い台帳は今どおり丸ごと飛ばす（留守と区別する）。(b) 留守の始まりは配達の記録に 1 項目持つ（決めたこと 5）。留守の間は approve のまま 10 分の判定も止める（マージを待つのは設計）。(c) 戻った最初の `<news>` は経路③（deliver）でもフック（drain）でも留守の 1 行が付く。(d) 「N 件」は置き換えの前の数で数え、問いの通知 `asked` は問いの n ごとに別に積む（同じ課題の 2 つ目の問いが 1 つ目を消さない）。(e) dispatcher 自身の起こし直し（core の起き直しで端末の中身がシェルに替わったとき）を Chief of Staff のフック以外にも持つ（boot の定期か巡ごとの自己確認。P6 で決める。今の `terminal.go` の「Chief of Staff が動けばフックが走る」の前提と dispatcher.md §7.2 #1 を直す）。
   捨てた案: 下読み役を入れてまとめさせる（新しい役）／留守の間は記録だけにして届けない（問い・impact を見落としうる）／マージも留守で進める（Chief Engineer の推奨。Chief of Staff が取らなかった）／コマンドを打つ段も留守で回さない（受け入れ条件 13 が成り立たない）。
9. **仮 2026-11-10 見守りの段は ADR-0014 のまま（見込みの 1・2・4 倍で Chief of Staff にだけ届ける。担当には送らない・止めない。一律の上限と費用は取らない）。見守りのワークストリーム定義は ADR-0014 を指す。途中報告を書かせる・分ける・起こし直す・延長する、は届いた Chief of Staff が決めて `hc send` する（定義の「自分で決めてよいこと」の欄）。夜間のターン数・費用の上限は取らない。**（指示文の表〈1.5 倍で担当に途中報告・2 倍で窓口が決める・夜間は上限〉と違う。Chief of Staff が選んだ）
   理由: 同日のオーナーの決定（ADR-0014）を覆さない。指示文の表の「対応」は Chief of Staff が選べる手として定義に残る。
   捨てた案: 指示文の表で ADR-0014 決めたこと 6・7 を改める（オーナーが朝に決めるなら、ADR-0014 に下から足す）。
10. **仮 2026-11-10 担当の計画は `report.json` の `plan[]`（1 件 `id`・`text`・`done`）。`andon issue report --data` で書き、`pr` と同じく次の報告に引き継ぐ（引き継ぎの形は ADR-0014 E4〈A-0063〉が `estimate` で入れるものに合わせる。P4 は A-0063 のあと）。進み具合は `done` の数。見守りの「見るもの」に入れる。schema にキーを足すだけで版は上げない。**（約束の変更。Chief of Staff が選んだ）
   理由: 今の計画は報告の Markdown の `## 計画` だけで機械が読めない。`issue.yaml` の予約キーは Chief of Staff しか書けない。担当が機械向けに書ける口は `report.json` だけ。今引き継いでいるのは `pr` だけで、`estimate` の欄と引き継ぎは A-0063 が入れる（反論 D1）。
   捨てた案: `library/plan.json` を別に置く（置き場と schema が 1 つ増える）／機械が読む計画を足さない（見守りは時間だけで見る）。
11. **仮 2026-11-10 詰まりの兆しは、この題では (a) commit が無い時間と (b) 変更の量の 2 つを数える。値は (a) 見込みの分だけ commit が無い（小 30・中 60・大 120 分）、(b) テストを除いた変更が 500 行超（横展開の既定の上限と同じ）。どちらも ADR-0014 の `estimate` の通知（段 1・2・4 倍を越えたとき）に `detail.sign` として乗せ、段の間では出さない（遅れてよい。新しい kind も別の通知も足さない）。見込みの無い課題は数えない（ADR-0014 決めたこと 8 のとおり通知が無い）。指揮側の worktree の git で数える。「テスト」の線はパスの規則（仮: `tests/` と `*_test.*`。P5 で決める）。**（Chief of Staff が選んだ。値は仮で、measure の外れ方を見てから直す）
   理由: 指揮側の git だけで数えられる。(c) 同じテストが続けて落ちる、はテストの出力を受ける口（フック・記録）が無い。(d) 依頼書の範囲の外のファイル、は依頼書が Markdown で機械が読まない（contract.md §0 原則 1）。段の通知に乗せるだけなので、控えのキーは ADR-0014 E3 の `<issue>|estimate|<値>|<段>` で足り、兆しの別キーは持たない（反論 E2）。
   捨てた案: (a) だけ（変更の量を見ない）／4 つ全部（(c) にはテストの出力の口、(d) には依頼書の機械向けの欄が先に要る）／兆しだけで `estimate` を出す（ADR-0014 受け入れ条件 3「同じ段で 2 回積まれない」を破る）。
12. **仮 2026-11-10 細くしたあとの dispatcher の大きさの目安: 1 巡の段はきっかけ表の行の数（定義の YAML で数える）で 8 以下、`<slug>.json` の一番上の項目は 12 以下（配達 6・拾う位置 4・担当の判定の map 1・留守の始まり 1）、テスト以外 2,700 行以下（概算: 今の 3,987 からコマンド 5 つの約 1,360 行を引き、tick・S3・E3・留守の分を足す。最初の版の実数で切り直してよい）。数えるのは measure ではなく、repo のテスト（上限を超えたら落ちる）と版上げの CHANGELOG の 1 行。**（指示文の「measure に足す」と違う。Chief of Staff が選んだ。数は反論 F1〜F3 を受けて 2026-11-10 に数え直した）
   理由: measure は台帳の記録を数えるコマンドで、repo の行数はプロジェクトの指標ではなく andon の版の性質。段を `pass()` の関数で数えると 1 つに包むだけで通るので、きっかけ表の行で数える。
   捨てた案: measure に足す（台帳の外のものを読むことになる）／目安を置かない（定義と段の対応だけをテストで見る）／6 段・10 項目・2,500 行（最初の数。列挙と概算で届かない）。
13. **仮 2026-11-10 図と生成した Markdown でも役の呼び方は用語集の語（CoS・CE・dispatcher・ゲート・担当・人）。指示文の「窓口・相談役・見張り役・検証ゲート」は使わない。定義の名（ワークストリーム・ステートマシン・きっかけ表・8 本のワークストリームの名・見込み・留守）は用語集に足す。**（Chief of Staff が選んだ）
   理由: ADR-0013（同日のオーナーの決定）と用語集の「作った訳語は使わない」に従う。訳語が増えると文書と画面で役の名がずれる。この ADR の本文では、指示文の語を引くときだけ括弧で添える。図と生成物に出る語が用語集に無いと、人とエージェントが別の語で呼び始める（反論 J9）。
   捨てた案: 用語集に「人向けの呼び方」の列を足して指示文の語を使う（ADR-0013 に下から足す小さな改め）。
14. **仮 2026-11-10 S3〈A-0026〉は 1 課題のまま。依頼書を「定義 `process/states/signal.yaml` に沿って、`andon signal collect` をきっかけ表の間隔（`project.json` の `signals[].every`）で打ち、未配達の `now` を配る」に書き直し、`after` をこの ADR ではなく「手順 2 の定義の課題（外のお知らせの分）」に替える。**（Chief of Staff が選んだ）
   理由: 外のお知らせの状態（`active`・`episodes`・`delivered_episode`。届けるのは `active` で `delivered_episode < episodes` の `now`）は ADR-0011 決めたこと 8・16 でもう決まっていて、手順 2 はそれを YAML に写すだけ。手順 3 のコマンド化を待たずに流せる。`delivered_episode` の書き方は決めたこと 5 の (c)。
   捨てた案: S3 をこの ADR の切る案に吸収する（ADR-0011 との対応が 1 段遠くなる）／手順 3 が全部終わるまで止める（外のお知らせが長く止まる）。
15. **仮 2026-11-10 定義に無い移り方を断るのは `andon issue …`（と dispatcher が打つコマンド）。`kanban` は汎用の台帳の CLI のまま触らない。課題の定義 `process/states/issue.yaml` に「型ごとの移り方と、それを打つコマンド」を書き、テストが定義に無い移り方をコマンドが断ることを確かめる。担当・レビュアー・Chief Engineer が `deck` を直に打つ移り方（担当の `deck start`、レビュアーの `deck report`・`deck review`、Chief Engineer の `deck review`）は定義に `by: deck` と書き、断らない（文書の約束のまま。反論 G1）。**（指示文の「台帳のコマンドが断る」と少し違う。Chief of Staff が選んだ）
   理由: ledger.md は「誰が何を変えてよいかは形式の外。セルの skill が決める」と書く。`andon issue start`（`todo` だけ）・`finish`（開いた問いで止まる）はもう一部を断っている。台帳の形式を変えず、断る場所が Andon の約束の層に揃う。
   捨てた案: `kanban` にも移り方の表を持たせて断る（台帳の形式が変わり、人が手で直す道も狭まる）。
16. **2026-11-10 定義の書き方（指示文の推奨のまま。Chief Engineer が形にした。オーナーの確かめは「推奨どおりでよいか」の 1 問）。**
   - 1 本のワークストリーム（`process/workstreams/<名>.yaml`）の項目: `name`（英字の識別子）・`title`（人が読む名）・`purpose`（目的）・`triggers[]`（きっかけ。`kind`: `person`／`notification`／`interval`／`date`／`event`、`notification` は通知の `kind`、`interval` は `every`〈分〉か `project.json` のキーの名）・`reads[]`（見るもの。台帳のキー・記録・コマンドの出力）・`decides[]`（自分で決めてよいこと）・`asks[]`（人に聞くこと。灯りの種類つき）・`leaves[]`（残すもの。台帳・wiki・記録）・`counts[]`（数えるもの。measure のキー）・`skill[]`（使う SKILL の節。`chief-of-staff#5` の形）・`tasks[]`（機械がやるタスク。`name`・`command`・`trigger`〈`triggers[]` の名〉・`record`〈自分の記録ファイル〉・`once`〈2 回起きてはいけないか〉）。schema で必須なのは `name`・`title`・`purpose` だけ（残り 6 本はこの 3 つだけのファイル）。中身のある 2 本が項目を全部持つことは `tests/process.sh` が見る。
   - 1 本のステートマシン（`process/states/<名>.yaml`）の項目: `name`・`title`・`source`（定義の出所。ledger.md・ADR の番号）・`states[]`（`id`・`title`・`record`〈記録ファイルと項目。例 `dispatcher:Pending`・`reviewers:Started`〉）・`positions[]`（状態でない項目。拾う位置とデータ。例 `dispatcher:LogPos`）・`transitions[]`（`from`・`to`・`on`〈何が起きたら〉・`by`〈`machine`／`ai`／`person`／`deck`〉・`command`〈打つコマンド。任意〉・`once`〈2 回起きてはいけないか。dispatcher.md §7 の表はここから導く〉）。
   - 機械が読むのは YAML だけ（schema は `schema/process/workstream.schema.json`・`state.schema.json`。未知のキーは弾く）。コメントは短い補足だけで、決めた理由は ADR に書く。
   - 人が読む Markdown（`docs/process/<名>.md`）は `andon process render` が YAML から作る（表と Mermaid の図）。テストは作り直して差分が無いことを見る。機械は生成物を読まない。
   - Mermaid で BPMN の形をまねる（本物の BPMN の XML は使わない）: 出来事（始まり・終わり・通知）は `((…))`、やることは `(…)` で、やる者の種類で色を分ける（機械＝dispatcher・ゲート・コマンド／AI＝CoS・CE・担当／人）、分かれ道は `{…}`、誰がやるかは subgraph の帯、作業の途中で時間が来たら別の道へ（BPMN の境界タイマー）は点線の矢印に「⏱ 見込みの 2 倍」のように書く。ステートマシンは `stateDiagram-v2`。
   - 1 本の定義ファイルは「人の 1 手」の中身を書かない。AI の 1 手の「どうやるか」は今どおり SKILL に書き、定義は `skill[]` でその節を指す。
   理由: 指示文の「書き方の推奨」と同じ。足したのは識別子の名前と、`once` を §7 の表の元にすること、`positions[]`（反論 H1）と `by: deck`（反論 G1）だけ。
   捨てた案: 本物の BPMN 2.0 の XML（差分が読めない・AI が直すと壊しやすい・GitHub でそのまま見えない・今は流れを外の仕組みで動かさない）／JSON を正本にする（コメントが残しにくい。人が書く定義は YAML、機械が書く記録は JSON、の andon の線に合わせる）。
17. **仮 2026-11-10 課題の状態ごとの時間（`todo`→`queued`→`doing`→`review`→`done` の各区間。台帳の `log.jsonl` の status の行の差）を `andon measure` の行に足す。**（指示文の「この題で入れる: 状態ごとの時間を measure に出す」。反論 J7 で抜けに気づいた。Chief of Staff が選んだ）
   理由: 台帳の記録だけで数えられる（measure の範囲。決めたこと 12 の「repo の行数」とは別）。見守りの「数えるもの」に入れる。
   捨てた案: 入れない（指示文の範囲に入っている）。

## オーナーに確かめること（朝。順に。仮の項目だけ。Chief of Staff が 1 問ずつ出す）

「はい」で推奨（Chief of Staff が選んだ案）が通る。違えば「決めたこと」の該当の番号を書き直し、捨てた案から選ぶか新しい案を足す。順 12 だけは Chief of Staff が Chief Engineer の推奨と違う案を取ったので、両方を見せる。

| 順 | 決めたこと | 問い（1 行） | 推奨 |
|---|---|---|---|
| 1 | 1 | 定義の形は案 C（YAML 正本・schema・生成 Markdown・テスト。dispatcher は読まない）でよいか | はい |
| 2 | 2 | 置き場は `process/`（YAML）・`schema/process/`・`docs/process/`（生成）でよいか | はい |
| 3 | 7 | 間隔で回る機械の仕事はルーチンでなく定義の「きっかけ表」に書き、ルーチンは課題を足すだけのままでよいか（指示文の推奨と違う。ledger.md の 1 文だけ直す） | はい |
| 4 | 9 | 見守りの段は ADR-0014 のまま（1・2・4 倍。担当には送らない。夜間の上限は無し）でよいか（指示文の表と違う） | はい |
| 5 | 12 | dispatcher の大きさは measure でなく repo のテストと CHANGELOG で数えてよいか（指示文と違う）。目安はきっかけ表 8 行・記録 12 項目・2,700 行でよいか | はい |
| 6 | 13 | 図と人向けの文書でも役の呼び方は用語集の語（CoS・CE・dispatcher）でよいか（指示文の窓口・相談役・見張り役は使わない） | はい |
| 7 | 15 | 定義に無い移り方を断るのは `andon issue` のコマンドで、`kanban` は触らず、`deck` を直に打つ移り方は断らなくてよいか（指示文は「台帳のコマンド」） | はい |
| 8 | 3 | ワークストリームは 8 本。中身は見守りと仕分けの 2 本から、でよいか | はい |
| 9 | 4 | ステートマシンは 8 本（指示文の 4 本＋課題・担当に、ゲートとマージ・片付けのあとの仕事を足す）でよいか | はい |
| 10 | 5 | 記録ファイルはコマンドごとに別、dispatcher には配達・拾う位置・担当の端末の判定・留守の始まりだけ（指示文の「配る分だけ」より多い）、でよいか | はい |
| 11 | 6 | コマンドに出すのは 5 つ（ゲートのあとのマージを含む）、順は 人のマージ → マージ → 手直し → 影響 → レビュアー、でよいか。R1〈A-0066〉・R2〈A-0067〉は吸収せず先に流す | はい |
| 12 | 8 | Chief of Staff の留守は配る段とゲートのあとのマージだけ止め、tick・起動・拾いは回す、でよいか。Chief Engineer の推奨は「マージも留守で進める」、Chief of Staff は「不可逆な操作は留守にしない」。どちらにするか | Chief of Staff の案 |
| 13 | 10 | 担当の計画は `report.json` の `plan[]`（引き継ぎは A-0063 の形）でよいか | はい |
| 14 | 11 | 詰まりの兆しは commit 無し（見込みの分）と変更の量（500 行超）の 2 つを、段の通知にだけ載せる、でよいか | はい |
| 15 | 14 | S3 は 1 課題のまま、`after` を手順 2 の外のお知らせの定義に替える、でよいか | はい |
| 16 | 16 | 定義の書き方は指示文の推奨のまま（識別子の名・`once`・`positions[]`・`by: deck` を足した）でよいか | はい |
| 17 | 17 | 課題の状態ごとの時間を measure に足す、でよいか | はい |
| 18 | — | 共有できたか: この文書で決めたとしてよいか | はい |

## やらないこと（範囲の外。理由）

1. **dispatcher が YAML の定義そのものを読んで、タイマーと振り分けを動かすこと**（案 B）: 定義がランタイムの入力になる。この題では定義と実装の食い違いをテストで見るまで（決めたこと 1。指示文の「後で決める」）。
2. **Camunda・Temporal などの外の BPM の仕組みを入れること**: 手元のセルには重く、薄く保つ方針（design.md §1.9 の 5）と合わない（指示文）。
3. **新しい役・新しい常駐を作ること**（dispatcher を 2 つに割る・下読み役・目標を持つ workstream の常駐）: 前提 3。ADR-0011 決めたこと 2 と同じ。
4. **ルーチンに「コマンドを打つ」種類を足すこと**: 決めたこと 7。台帳の形式を変えない。
5. **見守りで担当に途中報告を送る・止める・夜間にターン数や費用の上限で止めること**: 決めたこと 9。ADR-0014 決めたこと 6・7・やらないこと のまま。
6. **詰まりの兆しのうち「同じテストが続けて落ちる」「依頼書の範囲の外のファイルを触る」を数えること**: 決めたこと 11。テストの出力を受ける口と、依頼書の機械向けの欄が先に要る。
7. **残り 6 本のワークストリーム（受付・起動・受け取り・取り次ぎ・結果を見る・ふりかえり）の中身を書くこと**: 決めたこと 3。名前と目的だけ置き、同じ形で後から（困ったら分ける）。
8. **`kanban`（台帳の CLI・ledger.md の形式）を変えること**: 決めたこと 15。例外は ledger.md の Routines の「誰が tick を打つか」の 1 文（決めたこと 7 の (c)。形式ではない）。
9. **通知を配る道（Stop フック・UserPromptSubmit フック・離席時の `hc send`）と ADR-0010 の ack・`news.replied` の約束を変えること**: 指示文の「守ること」。配達の記録の項目は残す。
10. **人向けの新しい訳語（窓口・相談役・見張り役・検証ゲート）を文書と図に入れること**: 決めたこと 13。
11. **dispatcher の行数を measure に足すこと**: 決めたこと 12。
12. **留守の間にゲートのあとのマージと finish を進めること**: 決めたこと 8（Chief of Staff の案。オーナーの確かめ 順 12）。
13. **コード・schema・skill・dispatcher.md をこの PR で直すこと**: この文書は提案。写すのは実装の課題（切る案）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 6 つ（高いものから）: (i) `tests/schema.sh`（schema と例）、(ii) `tests/process.sh`（生成した Markdown と YAML の照合・§7 の表の導出・大きさの上限）、(iii) `go test ./internal/dispatcher/...`（記録の項目と状態の対応〈反射〉・1 巡の段ときっかけ表・コマンドの冪等）、(iv) `tests/andon-dispatcher.sh`（偽の台帳と偽の `hc` で見回り）、(v) `tests/package.sh`・`tests/public.sh`（skill と文書）、(vi) セルの実測。`tests/lib/scope.sh` に `process/**`・`schema/process/**`・`docs/process/**` → `tests/process.sh` の対応を足す（P1）。

1. **schema**: `schema/process/workstream.schema.json`・`state.schema.json` があり、`process/**/*.yaml` が全部通る。必須は `name`・`title`・`purpose`（ワークストリーム）と `name`・`title`・`source`（ステートマシン）だけ。未知のキー・`triggers[].kind` の enum の外・`transitions[].by` の enum の外は弾く（i）。
2. **生成**: `andon process render` が `docs/process/<名>.md`（表と Mermaid の図。ワークストリームは `flowchart`、ステートマシンは `stateDiagram-v2`）を書き、作り直して差分が無い（ii）。生成物に機械が読む行は無い。
3. **8 本のステートマシン**: `process/states/` に `issue`・`worker`・`delivery`・`reviewer`・`gate`・`human-merge`・`aftermath`・`signal` があり、dispatcher と各コマンドの記録ファイルの Go の構造体の**一番上の項目**が、どれか 1 本の `states[].record` か `positions[]` に 1 回ずつ出る（iii。`go test` が反射で項目名を読む）。P2b の時点では今の `State` だけを対象にし、P7〜P10 と merge の課題はその PR で対応表を直す。`signal` は `internal/signal` の構造体のうち `active`・`episodes`・`delivered_episode` が `states[].record`、残りが `positions[]`。
4. **「2 回走らせない」の導出**: `transitions[].once: true` の 1 件ごとに dispatcher.md §7.2 の行（走らせる者・二重に走ったら・今の守り）がある（ii。render が §7 の表を作るか、テストが突き合わせる）。
5. **ワークストリーム 2 本**: `process/workstreams/watch.yaml`（見守り）と `triage.yaml`（仕分け）が決めたこと 16 の項目を全部持ち（`tests/process.sh` が見る。schema は必須にしない）、`skill[]` の節が chief-of-staff skill に在る。残り 6 本は `name`・`title`・`purpose` だけのファイルがある（ii・v）。見守りの `triggers[]` に ADR-0014 の `estimate` の通知と間隔、`reads[]` に `report.json` の `plan[]`、`decides[]` に途中報告を書かせる・分ける・起こし直す・延長する、`counts[]` に状態ごとの時間（決めたこと 17）がある。
6. **留守でも拾う**: Chief of Staff の端末が無い巡で、拾う・記録する・コマンドを打つ段は回り（`kanban tick`・レビュアーの起動・ゲートの判定・人のマージの拾い・影響・手直しが進む）、配る段と `andon issue merge` だけ飛ぶ。待ちは `Pending` に残り、`asked` は n ごとに別。端末が戻った最初の `<news>` の先頭に「留守の間（<from>〜<to>）に N 件」の 1 行が、経路③（deliver）でもフック（drain）でも付く。`chief_of_staff` が設定に無い台帳は丸ごと飛ぶ（iv）。
7. **コマンド 5 つ**: `andon issue settle <id>`（人のマージを拾って片付ける）・`andon issue merge <id>`（ゲートのあとのマージと片付け）・`andon issue corrections <id>`（手直しを集める）・`andon issue impact` の打ち直しと諦めの記録（`--retry`）・`andon review start <id>`（レビュアーを起こす。上限の起こし直しと送り直しを含む）が、それぞれ自分の記録ファイル（`<slug>.<名>.json`）を持ち、2 回打っても結果が同じ（起こすのは 1 回・書くのは 1 回・通知は 1 回）。通知は stdout の JSON で返し、`Pending` に積むのは dispatcher。settle は `library/gate/<head>.json` の `merged` を読み、dispatcher のマージを人のマージと誤報しない。dispatcher の `<slug>.json` にそれらの項目が無い（iii・iv）。名前は仮で、切る案の担当が揃える。
8. **計画**: `report.json` の `plan[]`（`id`・`text`・`done`）が schema を通り、`andon issue report` が前の報告から A-0063 と同じ形で引き継ぐ（i・iii）。
9. **詰まりの兆し**: 偽の worktree で、見込みの分だけ commit が無い・テストを除いた変更が 500 行超のとき、段（1・2・4 倍）の `estimate` の通知の `detail.sign` に `no-commit`／`large-diff` が載る。段の間では通知が出ない。見込みの無い課題には出ない（iv）。
10. **大きさ**: テストが きっかけ表の行（≤ 8。定義の YAML で数える）・`<slug>.json` の一番上の項目（≤ 12。反射で数える）・テスト以外の行数（≤ 2,700）を数え、超えたら落ちる（ii・iii）。版上げの CHANGELOG にその 3 つの数がある。
11. **移り方を断る**: `process/states/issue.yaml` に無い移り方（例: `todo` から `andon issue ready`、`done` の課題への `review-request`）を `andon issue …` が終了コード 1 と理由 1 行で断る（iii）。`by: deck` の移り方は断らない。`kanban` は変わらない。
12. **文書と skill**: dispatcher.md §2.1.1 が「きっかけ表」（定義）を指す形になり固定の段の表が消える、§7.2 のうち dispatcher とコマンドの行（#2〜#8・#17〜#19 とマージ）が定義から導いた表になる（#1・#9〜#16 は手で書いたまま）。chief-of-staff skill §5・§10 が見守り・仕分けの定義を指す。contract.md に `plan[]`・記録ファイルの一覧・`detail.sign`。glossary に決めたこと 13 の語。ledger.md の Routines の 1 文。ADR-0007（R1 の「生きているときだけ」）と ADR-0011（受け入れ条件 9）に下から足す。`tests/public.sh` が通る（v）。
13. **実測**: セルで Chief of Staff を 10 分止めて、担当の `review` → ゲートの判定 → 人のマージの拾いが進み、`merge: auto` の approve はマージを待ち、戻った最初の `<news>` に留守の 1 行が出て、戻った巡で `andon issue merge` が打たれる（vi）。
14. **状態ごとの時間**: `andon measure` の行に課題の状態ごとの時間があり、`schema/measure.schema.json` を通る（i・iii）。

## 価値の仮説（ADR-0011 決めたこと 12 の 6 項目）

1. 誰の何が変わるか: オーナーと Chief of Staff が、改善したい所を定義の 1 行（ワークストリームの項目か移り方）で指せる。機械に任せる仕事が dispatcher でなくコマンドと定義に積まれ、dispatcher が太らない。
2. 見る信号: オーナーの目視（定義を指して改善を頼めたか）。大きさのテストの 3 つの数（CHANGELOG）。dispatcher を触った課題の依頼書が `process/` の節を `links` で指す数（台帳）。
3. 見る日: 切る案の最後の実装（P11）がマージされてから 30 日後（Chief of Staff が `not_before` に絶対の日付で写す）。
4. 誰が要るか: オーナー。ほかの人は要らない。
5. 効いたとどう分かるか: 30 日で (a) dispatcher を触った課題のうち、定義（YAML）の変更を伴うものが 3 分の 2 以上、(b) 決めたこと 12 の 3 つの数が保たれている、(c) S3 と E3 が定義のきっかけ表の 1 行とコマンドで入った。
6. 何があれば止めるか: 同じ変更で YAML・生成物・実装の 3 か所を直す手間が、実装だけを直すより明らかに重い（Chief of Staff が報告で 3 回続けて書く）なら、案 A（Markdown だけ）に戻す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

Settings の既定: Review on・Must fix major・Merge person（レベル C。前提 4）。dispatcher を触る課題（A-0036・A-0039・A-0040・A-0062・A-0066・A-0067・A-0070・A-0071）との順は Chief of Staff が並べる。R1〈A-0066〉・R2〈A-0067〉は吸収せず今の `after` のまま先に流す（決めたこと 6・7）。A-0036（Chief Engineer の端末の通知を届けない。doing）は `route.go`・`pickup.go` を触るので、P6 はそのマージのあと。

| # | 課題 | 決めたこと | 受け入れ条件 | 依存 |
|---|---|---|---|---|
| P1 | 定義の器: `schema/process/*.schema.json`・`process/` の置き場・`andon process render`・`tests/process.sh`（生成の照合）・`tests/lib/scope.sh` の対応・`docs/process/README.md`（人向けの入口） | 1・2・16 | 1・2 | 無し |
| P2a | 外のお知らせの一生 `process/states/signal.yaml`（ADR-0011 決めたこと 8・16 を写す） | 4(8)・14 | 3 の `signal` の分 | P1 |
| P2b | 残り 7 本のステートマシンと、今の `State` の項目との対応表（反射のテスト）・`once` から §7.2 を導く | 4・5 | 3・4 | P1 |
| P3 | ワークストリーム: 見守り・仕分けの 2 本と残り 6 本の名前。chief-of-staff skill §5・§10 を定義を指す形に。glossary に決めたこと 13 の語 | 3・9・13 | 5・12 | P1 |
| P4 | `report.json` の `plan[]`・worker skill の `## 計画` の機械向けの写し・contract.md §4 | 10 | 8 | A-0063 |
| P5 | 詰まりの兆し (a)(b) を段の `estimate` の通知の `detail.sign` に | 11 | 9 | ADR-0014 E3〈A-0062〉・P4 |
| P6 | 留守でも拾う・起こす。配る段と merge だけ飛ばす。留守の始まりの記録・戻った最初の `<news>` の留守の 1 行（deliver・drain）・`asked` を n ごとに・dispatcher 自身の起こし直し・dispatcher.md §2.1 と §7.2 #1 | 8 | 6・13 | P2b・A-0036 |
| P7 | 人のマージを拾って片付ける → `andon issue settle`（自分の記録。ゲートの記録の `merged` を読む） | 5・6 | 7 の settle | P2b・A-0067 |
| P7b | ゲートのあとのマージと片付け → `andon issue merge`（自分の記録。`Gates`・`Approved` を移す。approve のまま 10 分は配る側がこの記録を読む） | 5・6 | 7 の merge | P7 |
| P8 | 手直しを集める → `andon issue corrections`（自分の記録） | 5・6 | 7 の corrections | P2b |
| P9 | 影響の打ち直しと諦め → `andon issue impact --retry` が自分の記録を持つ | 5・6 | 7 の impact | P2b |
| P10 | レビュアーを起こす → `andon review start`（自分の記録。上限の起こし直しと送り直しを含む。ADR-0012 R3〈A-0039〉・R4〈A-0040〉のあと） | 5・6 | 7 の review start | P2b・A-0039・A-0040 |
| P11 | 1 巡を「きっかけ表のうち時が来たものを打つ」に（`kanban tick` の行は R1〈A-0066〉のあとに移すだけ。signal の間隔は `project.json` を指す）。大きさのテストと CHANGELOG。dispatcher.md §2.1.1・§7.2。ledger.md の 1 文。ADR-0007・ADR-0011 に下から足す | 7・12 | 10・12 | P6〜P10・P7b・A-0066 |
| P12 | `andon issue …` が `issue.yaml` の定義に無い移り方を断る（`by: deck` は断らない） | 15 | 11 | P2b |
| P13 | 観察: 価値の仮説（30 日後。`not_before`） | — | — | P11 |
| P14 | 課題の状態ごとの時間を `andon measure` に足す（`schema/measure.schema.json`） | 17 | 14 | 無し |
| S3 | A-0026 の依頼書を書き直し、`after` を P2a に替える（決めたこと 14。`delivered_episode` の書き方は決めたこと 5 の (c)） | 14 | — | P2a |

## 開いている問い（残った問い。決断の種類つき）

- オーナーの確かめ（上の 18 問。種類は `line`＝ラインそのものの変更。順 12 は `irreversible` でもある）以外に決断は無し。
- 決断ではなく実測で決まるもの（切る案の担当が確かめる）: 詰まりの兆しの値（決めたこと 11。measure の外れ方を見て直す）、大きさの目安の数（決めたこと 12。最初の版の数で切り直してよい）、コマンドの名前（受け入れ条件 7）、「テスト」のパスの規則（決めたこと 11）。
- 反論で確かめきれなかった疑い（切る案の担当が読む）: ADR-0012 R4〈A-0040〉が足す「resume した」の控えの形（決めたこと 4 の (2) の map に入れる前提）。A-0036 の差分と P6 のぶつかり（依存に書いた）。

## 出典（読んだもの・捨てた案の出所）

- 指示文 P11（オーナー。2026-11-10。セルの `files/`。repo の外）と、その「背景」の節（3 層の整理・呼び方の話・BPM と BPMN とステートマシンの区別）。オーナーの言葉は「問題」の段に写し。
- Chief of Staff の答え（2026-11-10 夜。オーナーが寝ている間の対話。端末 andon / CoS）。
- 反論（2026-11-10。別の文脈の読むだけのサブエージェントに「この設計を壊せ」と頼んだ。証明つき 27 件・疑い 5 件。直した所は各項目の「反論から」「細部」と受け入れ条件・切る案に写した。残した疑いは「開いている問い」）。
- [dispatcher.md](../dispatcher.md) §0・§2.1〜§2.8・§3・§5・§6・§7（§7.2 の表・§7.3 の G1〜G8・§7.4）、[contract.md](../contract.md) §0・§2・§4・§6・§7・§13・§15、[ledger.md](../ledger.md)（Status rules・Routines・Questions・`log.jsonl`）、[design.md](../design.md) §1.3・§1.7・§1.8・§1.9・§2.1〜§2.3。
- [ADR-0007](0007-interim-gap-inventory.md)（C1・A5〜A7・G4・R1・R2・R7）、[ADR-0010](0010-news-ack.md)（決めたこと 1〜8・捨てた案 1）、[ADR-0011](0011-signals-and-value-loop.md)（前提 6・決めたこと 2・5・6・8・12・16・受け入れ条件 9・S2・S3）、[ADR-0012](0012-resume-and-worktree-home.md)（R3・R4）、[ADR-0013](0013-short-role-names-and-plain-japanese.md)、[ADR-0014](0014-issue-estimate.md)（決めたこと 1〜11・やらないこと・E1〜E6）、[ADR-0006](0006-survey-fix-decide.md) 前提 11（Factory の工程と workstream）。
- skill: chief-of-staff（§0〜§11・`<news>` の読み方の表）、worker（§2・§5・§7）、ask-one-by-one、glossary、chief-engineer。
- `internal/dispatcher/`（state.go・dispatcher.go・loop.go・pickup.go・deliver.go・drain.go・humanmerge.go・corrections.go・impact.go・route.go・news.go・ack.go・fallback.go・screen.go・terminal.go と各テスト。読むだけのサブエージェントが 2026-11-10 に洗った。「読んだ事実」の節）、`internal/issue/issue.go`・`internal/signal/signal.go`・`internal/contract/contract.go`、`schema/`（project・ledger・measure・notification・report）、`cmd/kanban/routines.go`、`tests/run`・`tests/lib/scope.sh`。
- 台帳（`deck list` 2026-11-10。A-0026 の `after`、dispatcher を触る課題の一覧と `after` の鎖）、セルの wiki の index。
- 捨てた案の出所: 本物の BPMN の XML・外の BPM の仕組みは指示文（オーナーが捨てた）。ルーチンで取りに行く案は ADR-0011 決めたこと 5。一律の上限と担当への送りは ADR-0014。下読み役は指示文の「背景」。マージを留守で進める案は Chief Engineer（Chief of Staff が取らなかった）。
