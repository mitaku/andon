# ADR-0007: 仮設の工場にあって Andon に無いものの棚卸しと、塞ぐ順（仕様・設計。2026-11-06。課題 Y-143。2026-11-10 に今に合わせて出し直し: 課題 A-0058）

- 状態: **提案**。人が PR をマージした日に `決定（日付）` にする。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 出し直し（2026-11-10）: 最初の提案は PR #72（2026-11-06）。引っ越しが済んでも開いたままで古くなったので、人の決定（2026-11-10。「今に合わせて直してからマージする」）で、今の main に合わせた PR #135 で置き換えた。決めたこと 1〜4 は 2026-11-06 の観察と判断のまま残し、今の区分（済・残る・やらない）は**決めたこと 5** に下から足した。依頼書に切る案は残るものだけにした。
- 主題: 仮設の工場（Andon の前身。今も Andon を建てている建屋）で**いま実際に回っている仕組み**のうち、Andon にまだ無いもの・一部しか無いもの・設計だけのものを棚卸しし、引っ越し（Andon の開発をセルフホストの建屋 `home` へ移す。[design.md](../design.md) §5.2 の P8）の**前に塞ぐもの**と**移ってから建てるもの**と**やらないもの**に分ける。（2026-11-10: 引っ越しは済んだ。決めたこと 5）
- 語は [design.md](../design.md) §0.1（「建屋」はセルに改めると決めたが〈[ADR-0005](0005-cell-repo.md) 前提 3〉、まだ改めていないので今の語のまま）。仮設の語（yard・deck・コーディネーター・yard news）は design.md §2.4 の対応で Andon の語に直して書く。仮設の台帳の課題の番号（`Y-…`）は、その課題の依頼書と報告が出典であることを示すためにだけ引く。（2026-11-10: 「建屋」は今の語でセル。台帳のカードは課題〈[ADR-0008](0008-issue-naming.md)〉、エージェントは「起動する」〈[ADR-0013](0013-short-role-names-and-plain-japanese.md) 決めたこと 4〉に語だけ揃えた。決めたこと 5 から下は今の語で書く）
- 読み方: 表は「見つけたものを事実で書く」。塞ぎ方は案で、実装はこの ADR の「依頼書に切る案」のとおり別課題。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「P7 を見ていて、Andon に足りないところが結構ありそう。たとえば CI の checks をゲートが見ていなかった」。

P7（建屋 `home` を立てて Andon の PR を 1 本流す。[operate.md](../operate.md)）は通ったが、通る途中で 25 件ほどの控え（番号つきの所見。最後の番号は 25）が出て、その多くは仮設の工場では**人の手か yard の仕組みが黙って補っていたもの**だった（起動のたびに戻るもの・止まった係を起動し直すこと・CI を目で見てからマージすること）。Andon は仮設の道具を一から Go と約束の形に建て直したので、「仮設にはあったが Andon には写していない」ものが、実際に流してみるまで見えなかった。困りごとは「足りないものがある」ことではなく、**何が足りないかの一覧が無く、引っ越しの前に塞ぐべきものと後でよいものの線が無い**こと。線が無いと、引っ越しのたびに P7 と同じ形で 1 つずつ踏む。

## 前提（人と同意した順。番号）

1. **引っ越し（Y-129）を始める条件はもう満たした**（人と合意 2026-11-06）: P7 が operate.md §13 まで通る・信頼の画面と最初のメッセージの送り直し（Y-128）・建屋で checkout から image を焼く口（Y-136）はマージ済み。public への切り替え（Y-130）は、引っ越して `home` で開発が回ることを確かめてから。
2. **棚卸しの入力**（人の依頼書）: P7 の控えとそれを直した課題（仮設の台帳の Y-121〜Y-142 と、その後の Y-148・Y-155・Y-159・Y-179・Y-180・Y-181）／仮設の工場でいま回っている仕組み（台帳・通知の係・レビュアーの起動のしかた・検収・トリアージ・ルーチン・publish・memory・coordinator の設定・質問の間引き・herdr の連携・portless。正は仮設の環境メモと skill と `lib/`）／Andon の docs／人が「よい記事」と言った記事（出典）／ADR-0004 で見つかった継ぎ目／Y-161 の調べ（shepherdr の 4 世代）の候補／[dispatcher.md](../dispatcher.md) §7.3 の「守りが無い所」G1〜G8。
3. **「仮設ではどう回っているか」は 2026-11-06 の仮設の skill と `lib/` の実装**で書く（人の手か仕組みかを分ける）。仮設の設計文書に書いてあって動いていないものは「無い」と数える。
4. **塞ぐ順の基準**（この ADR で決める。決めたこと 3）: 引っ越しの前に要る＝**`home` で Andon の仕事を回し始めた日から、人が気づかずにラインが止まる、または人が毎回手で補うことになるもの**。移ってから＝Andon の建屋の上で Andon の課題として建てられ、無い間は灯りが 1 つ増えるだけのもの（design.md §5.1「その先」の列）。やらない＝設計で捨てたもの・外部の部品の持ち物・数えてから決めるもの。
5. **状態の正は hc の報告**で、画面の文字ではない（hc の決まり: status はプログラムの OSC 7501 の報告。画面から推さない）。記事の herdr（TUI）は画面の検出で状態を決めるので、同じ語（idle・done・blocked）でも意味が違う所がある（決めたこと 2 の表 F）。建屋には両方ある: 役の端末（Chief of Staff・dispatcher・担当・レビュアー）は hc の端末で、人は Herdr App（SSH 越し）か、`bin/bootstrap` で入る herdr（TUI）のペインから `hc attach` で見る。
6. **引っ越しそのものの手順は決まっている**（operate.md の「別のマシンへ引っ越す」〈Y-167〉と「前の置き場から移す」〈Y-141〉）。この ADR は手順を書き直さない。引っ越しの前に**仮設の台帳に残る Andon の課題を `home` の台帳へ渡す束**を作るのは Y-129 の仕事で、この ADR の「依頼書に切る案」はその束の先頭に置く候補。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

（2026-11-10: 1〜4 は 2026-11-06 の観察と判断のまま残す。引っ越しのあとの今の区分は 5）

1. **2026-11-06 置き場はこの ADR。** design.md の節にしない。
   理由: 棚卸しは日付のある 1 回の観察で、design.md（構造の正）に残すと古くなる。塞いだら「決めたこと」に下から足し、全部塞いだら `覆された` ではなく「済み」の印を付けて記録として残す。[decisions.md](../decisions.md) には 1 行。
   捨てた案: design.md §5.3（仮設の間の決まり）に表を足す——§5.3 は決まりの節で、観察の表が入ると読み手が変わる。

2. **2026-11-06 棚卸しの表。** 列は 仮設の工場では（人の手か仕組みか）／Andon では（ある・一部・無い・設計だけ）／無いと何が起きるか（P7 で実際に起きたか）／塞ぎ方の案／大きさ／順（**前**＝引っ越しの前・**後**＝移ってから・**やらない**・**済**＝P7 の直しで入った）。順の理由は 3。

   **A. ライン（レビュー・ゲート・マージ）**

   | # | 項目 | 仮設の工場では | Andon では | 無いと何が起きるか（P7 で） | 塞ぎ方の案 | 大きさ | 順 |
   |---|---|---|---|---|---|---|---|
   | A1 | ゲートが PR の CI（GitHub の checks）を見る | **人の手**。Chief of Staff が `gh pr checks` を目で見てからマージする。仮設のゲート（`yard gate`）の L2 は条件表の検査を自分で回すだけ | **ある**（Y-142・Y-179。`wait`／`fix`／`needs_human`、マージの直前の読み直し、`github_checks: optional`） | CI の落ちた PR を dispatcher がマージしうる。**P7 で起きた**（人が気づいた。Y-142 の動機） | 済み。残る隙間（読み直しと `gh pr merge` の間）は public にしたら ruleset で塞ぐ（decisions.md §3） | — | 済 |
   | A2 | レビュアーを起動する・見た commit の照合 | **人の手**。Chief of Staff が detached worktree と子課題を手で作り起動する。検収で `findings.json` の `commit` と PR の head を照らす | **ある**（dispatcher が起動する。`andon issue ready` とゲートの `review.state` が照らす） | — | 済み。レビュアーの id を `<親>-r<回>` にするのは Y-177（進行中） | — | 済 |
   | A3 | `preexisting`（課題の前からあった指摘）と 2 回目以降のレビューの規則（Y-161 の C1・C2） | **無い**。再レビューの依頼書に「前の回の commit を書き、差分だけ見る・問題なしの観点はやり直さない」はある | **一部**。independent-review の依頼書の型に同じ文はある。`preexisting` のキーも「2 回目以降は修正確認だけ、新規は critical＋証拠だけ」の規則も無い | 直した差分の外の指摘で回が収まらない。仮設で起きた（independent-review の design-source）。P7 では未発生 | `findings.json` に任意キー `preexisting: true`（ゲートは `must_fix_open` に数えない）＋ worker skill §6 に 2 回目以降の規則。版は上げない（contract.md §0 の 4） | 小 | 後 |
   | A4 | 証拠（Proof）を受け入れ条件の書き方に（Y-161 の D1。記事の「確かめ役には差分とテスト出力を渡す」） | **無い**。完了条件は依頼書の文で、担当の報告の「テスト結果」を Chief of Staff が読む | **一部**。`report.json` の `acceptance[]` は schema とゲート（`fail` → `fix`・`manual` → レベルを 1 つ上げる）にある。証拠の質（本物・staged 不可・mock は証拠でない）は書かれておらず、P7 では使われていない（Chief Engineer の文書を通した PR がまだ無い） | 「通した」が言葉だけになる（記事「要約を確かめさせれば要約が正しいかどうかしか分からない」）。P7 では未発生（レビュアーは差分を読んだ） | contract.md §4 の `evidence` の書き方（テストの command と出力のパス。staged な出力は証拠でない）と、chief-engineer skill の受け入れ条件の節に 1 段 | 中 | 後 |
   | A5 | マージのあとの片付けと、人のマージとの重なり（G3） | **人の手**。人がマージ → Chief of Staff が `MERGED` を見て `yard task finish` | **ある**（`merge: auto`＋委譲の範囲なら dispatcher がマージして finish。`person` は Chief of Staff が人に頼む）。**守りが無い**: 人が先にマージすると dispatcher の `gh pr merge` が失敗し、`irreversible` の問い（誤報）が出る | 誤報の灯りが 1 つつく。P7 では未発生。セルフホストではレベル C（docs 以外のライン・人がマージ）と A／B（dispatcher）が同じ台帳に混ざるので起きやすい | 失敗のあと `gh pr view --json state,mergeCommit` で `MERGED` かつ head がゲートの `head` と同じなら、問いにせず `finish` へ。dispatcher.md §7.3 の G3 を §7.2 の #5 へ | 小 | **前** |
   | A6 | ゲートを手で打つ・dispatcher を 2 本打つ・boot の lock（G1・G2・G6） | （`yard gate` は人が打つだけ。係は 1 つ） | **無い** | 記録の上書き・PR のコメントの二重・ログが荒れる。実害小（dispatcher.md §7.3） | G1 `.dispatcher.lock`・G2 head ごとの lock・G6 `boot.lock` | 小 | 後 |
   | A7 | `andon issue start`・`chief-of-staff` の「見る→起動する」の lock（G5） | 同じ（`yard task`） | **無い** | 同時に 2 本打つと担当が 2 人になる。打つのは Chief of Staff 1 人なので起きにくい | 台帳の lock の中で `doing` を先に予約してから起動する | 小 | 後 |

   **B. 通知・止まった検知・起動し直し**

   | # | 項目 | 仮設の工場では | Andon では | 無いと何が起きるか（P7 で） | 塞ぎ方の案 | 大きさ | 順 |
   |---|---|---|---|---|---|---|---|
   | B1 | 通知の配り方（人の書きかけに貼り付けない） | **仕組み**。通知の係が `hc send`（入力欄が空のときだけ。薄い字の組み直し） | **ある**（dispatcher の ①`Stop` ②`UserPromptSubmit` ③離席時の `hc send`。P6b）。ただし ①②③の効きは「本物の Claude Code で実測」のまま（dispatcher.md の冒頭・operate.md §12 の 4 項目）。P7 では通知が届いてラインは回ったが、4 項目を見た記録が無い | 効いていない口があれば、通知が届かず Chief of Staff が起動しない（③の判定が外れれば書きかけに貼り付く）。P7 では起きていない（届いた） | operate.md §12 の表のとおり人が 1 つずつ試し、結果を §12 に書く。落ちた項目は課題にする | 小（人） | **前** |
   | B2 | 止まった担当の検知（`stalled`） | **仕組み**（idle 15 分・working 20 分） | **ある**（同じ値。dispatcher.md §2.3） | — | 済み | — | 済 |
   | B3 | プロバイダのエラーの続き打ち | **仕組み**（1・5・15 分で 3 回。4 回目は人へ） | **ある**（同じ）。起動するときの一時の失敗の再試行（shepherdr の B2）は Y-172（進行中） | — | Y-172 を待つ | — | 済 |
   | B4 | 最初のメッセージの送り直し | **人の手**（`yard task start` の終了コード 3 → Chief of Staff が `hc send`） | **ある**（`first_message.sent=false`・`started`・dispatcher が送り直す。Y-128・Y-148） | **P7 で起きた**（信頼の画面で止まった担当・Codex のレビュアーに届かない） | 済み | — | 済 |
   | B5 | 入力欄に貼られたまま残った最初のメッセージ（Y-148 の実測） | 同じ（人が画面を見る） | **無い**。`sent: true`・`started: false` で入力欄に文が残っていると、送り直しで同じ指示が 1 回の送信に 2 つ並ぶ。hc の status はエージェントの状態で、入力欄の中身は見ていない | 同じ指示が 2 回並ぶだけ（害は小さい）。**P7 で起きた**（Codex のレビュアー） | 送る前に `hc read` で入力欄の行に最初のメッセージの 1 行目が残っていれば `hc keys <t> enter` だけ押す | 小 | 後 |
   | B6 | 作り直しで戻った端末（中身が素のシェル）の見分け | **仕組み**。通知の係は `--keep` の端末にシェルで打ち込んで中身を戻す | **ある**（前に出ているものの argv で見る・片付けて起動し直す・戻った担当は `finished`。Y-131・Y-155・Y-180） | **P7 で起きた**（係が止まったまま誰も気づかない） | 済み | — | 済 |
   | B7 | 人がいないときに人へ知らせる口 | **仕組み**。`herdr notification show`（herdr〈TUI〉の外側の端末に出る）。人は TUI のペインにいる | **一部**。`package/environment.md` と chief-of-staff skill §5・§10 が `herdr notification show` を指す。建屋では herdr（TUI）は `bin/bootstrap` で入るときだけ動き、Herdr App（SSH 越し）で使う人には届かない。役の端末は hc の端末で TUI のペインではない（Y-124 で TUI の連携フックも外した） | App で使っているとき、Chief of Staff が効かないコマンドを打つ（失敗して通るだけ。人には何も届かない）。P7 は App で行われた | skill と environment.md の文を「herdr（TUI）が動いているときだけ効く。無ければ台帳の問いとボードで足りる」に直す。App への通知の口は外部の部品（Y-161 の P2。オーナーへ） | 小 | 後（D1 と同じ課題で） |
   | B8 | `hc events`（見回りの代わり） | 無い（見回り） | **無い**（見回り。dispatcher.md §2.3「使えればそれ」） | 無くても動く（待ちがあれば 5 秒・無ければ 1 分） | 要らない | — | やらない |

   **C. ルーチン・トリアージ・アウターループ・記憶**

   | # | 項目 | 仮設の工場では | Andon では | 無いと何が起きるか（P7 で） | 塞ぎ方の案 | 大きさ | 順 |
   |---|---|---|---|---|---|---|---|
   | C1 | ルーチンの `schedule` を撃つ（`kanban tick`） | **仕組み**。通知の係が見回りの最初に 1 分ごとに `deck tick` | **無い**（G4。誰も打たない。順だけ dispatcher.md §2.1.1 で「3 と 4 の間」と固定済み） | **ルーチンが 1 つも動かない**（週 1 のトリアージも）。アウターループの入口が開かない。P7 では未発生（A-1〜A-7 は人が足した） | dispatcher の 1 巡の 3 と 4 の間で `kanban tick`（Chief of Staff が生きている台帳だけ。`tick` で足した課題を同じ巡の `ready` で拾う）。§7.2 の #8 を「今の守り」へ、§7.3 の G4 を消す | 小 | **前** |
   | C2 | トリアージ | **仕組み**＋人の go。ルーチン `triage`（週 1・sonnet）＋ `yard triage collect`／`apply` | **ある**（`andon triage collect`／`apply`・triage skill・`assets/triage.yaml`）。ルーチンのファイルを台帳に写すのは人の go（skill §10） | C1 に依る | C1 のあと、`routines/triage.yaml` を `home` の台帳に写す（人の go） | 小（人） | **前**（C1 と一緒） |
   | C3 | 振り返り（retro）と人の手直しの記録（`corrections.json`） | **設計だけ**（reflect はトリアージへ統合。仮設の決定 P4） | **設計だけ**（design.md §1.4・§1.9 (b)・contract.md §1。`corrections.json` は P6a に入れていない。`retro` のルーチンも無い） | 学びは `remember[]`・`agents_md[]` をトリアージが振るだけ（仮設も同じ）。人の手直し（問いの答えが推奨案と違った・指摘の覆し）は数えられない | 移ってから、まず `corrections.json` の 1（問いの答え）と 3（覆し）を `andon issue finish` で集める。`retro` のルーチンはそのあと（最初の題を Chief Engineer で流す。D2） | 中 | 後 |
   | C4 | `agents_md` を製品 repo の `AGENTS.md` に入れる手順（Y-161 の A4） | **仕組み**。トリアージの `agents` → `fix`（担当の PR） | **ある**（triage skill に同じ判定） | — | 済み | — | 済 |
   | C5 | memory と wiki | `memory/`（index だけ・空）。書くのは Chief of Staff（規約 9） | **一部**。置き場は `init/wiki/index.md`、書き手は Chief of Staff（規約 9）、ハーネスの auto memory は managed で OFF（ADR-0004 決めたこと 2・Y-165）。1 件 1 ファイルの形と lint は無い。**Codex 系の memory 相当は未確認** | 置き場はあり、学びは溜まっていない（仮設でも空）。Codex に自動の記憶があれば、建屋の外から見えない記憶が育つ | wiki の形と lint は、wiki を書く題が出てから。Codex の記憶は `~/.codex` の設定（`config.toml`・`memories`）を確かめ、あれば boot で切る | 小 | 後 |
   | C6 | プロジェクトの設定（Chief of Staff への指示・並行の上限） | `coordinator.md`（台帳の中） | **ある**（`projects/<slug>/AGENTS.md` の Chief of Staff の節と `project.json` の `chief_of_staff`。Y-141・Y-166）。行数の lint は Y-169（todo） | — | Y-169 | 小 | 後 |

   **D. 役・起動・人との対話**

   | # | 項目 | 仮設の工場では | Andon では | 無いと何が起きるか（P7 で） | 塞ぎ方の案 | 大きさ | 順 |
   |---|---|---|---|---|---|---|---|
   | D1 | Chief of Staff が wiki に書ける（design.md §2.3 の `--add-dir`） | （コーディネーターの cwd は建屋のルートなので `memory/` は cwd の中） | **無い**。design.md §2.3 は「Chief of Staff の template に `--add-dir`（wiki・projects）」と書くが、`package/templates/chief-of-staff.md` の command に無い。cwd は `projects/<slug>/` なので `projects` は見えるが、`wiki/` は cwd の外（ADR-0004 で見つけた継ぎ目） | Chief of Staff が wiki に書こうとすると、cwd の外への書き込みで許可を求めて止まるか拒まれる。規約 9 の出口が塞がる。P7 では wiki を書く場面が無く未発生 | template の command に `--add-dir <建屋の repo>/wiki`（boot が `/opt/andon` と同じく `ANDON_WORKSPACE` に置き換える）。`projects` は cwd なので足さない。`tests/package.sh` で template が `--add-dir` を持つことを見る | 小 | **前** |
   | D2 | Chief Engineer（人と設計を対話する役） | **無い**（Chief of Staff が兼ねるか、最上位のプロファイルの担当課題。この ADR もその形） | **ある**（command・skill・template・台帳の課題）。**P7 では実測していない**（A-1〜A-7 は担当だけ） | 人と設計を詰める話が Chief of Staff に流れ、コンテキストが太る（§1.8 の反省そのもの） | 移ってから、Andon の題を 1 つ `andon chief-engineer` で流す（最初の題は C3 の retro でよい）。落ちたら課題 | 小（人） | 後 |
   | D3 | 人への質問の間引きと 1 問ずつ | **skill**（ask-one-by-one） | **ある**（skill＋問いの JSON＋`kanban ask`。`kind` に `line` も） | — | 済み | — | 済 |
   | D4 | 担当の許可の確認に Chief of Staff が答える（`hc keys`） | **人の手**（Chief of Staff が `hc keys`。信頼の画面は worktree のときだけ） | **ある**（skill §4）。ただし **P7 で Chief of Staff の auto mode が `hc keys` を拒んだ**（信頼の画面。Y-133 の 5）。信頼の画面は Y-128 で出なくなり、Codex は承認を求めない（Y-145）。Claude Code の担当の許可の確認は残る | Chief of Staff が答えられず人へ回る（規約 3 どおり。灯りが増えるだけ） | 許可ルールは足さない（auto mode の判定は人の安全のため）。人へ回った回数をトリアージで数え、多ければ別の題 | — | やらない（数える） |
   | D5 | herdr との連携（状態の報告・tab 名） | **仕組み**。herdr（TUI）の連携フック（`herdr-agent-state`・`herdr-auto-title`） | hc の状態はプログラムの OSC 7501 の報告（フックは要らない。Y-124）。tab・Space 名は Herdr App の mod（ADR-0002・0003）。担当の tab 名が付かない件（Y-181）は人の App の側と見立てた | — | 済み。Y-181 は人の App の版の確認待ち | — | 済 |
   | D6 | Codex の Chief of Staff とフック | 無い | **無い**（最初の版は Claude Code だけ。問い 12） | — | — | — | やらない |

   **E. 建屋・道具・引っ越し**

   | # | 項目 | 仮設の工場では | Andon では | 無いと何が起きるか（P7 で） | 塞ぎ方の案 | 大きさ | 順 |
   |---|---|---|---|---|---|---|---|
   | E1 | portless・files サーバ・publish | **仕組み**（起動スクリプト・`yard publish`） | **ある**（boot・`andon publish`。`files/` は追跡） | — | 済み | — | 済 |
   | E2 | 作り直しても戻るもの（git の資格情報・ログインシェル・SSH の環境・uid） | **仕組み**（起動スクリプト） | **ある**（Y-121・Y-137・uid 1000） | **P7 で起きた**（控えの 11・12・13・F4・F5） | 済み | — | 済 |
   | E3 | 直して建屋で試す（checkout から image を焼く） | （雛形の取り込み） | **ある**（`bootstrap rebuild --from`。Y-136） | — | 済み | — | 済 |
   | E4 | 建屋の repo の自動 push | **仕組み**（post-commit フック） | **捨てた**（問い 10。ADR-0005 の 5 で再確認） | — | — | — | やらない |
   | E5 | 作業側を別のマシンに出す（`machine`・G7） | 無い | **設計だけ**（`machine` のある課題は `start` が止まる。`stop-check` は台帳を直接読む） | 今は起きない（前提の違反） | design.md §1.5。大きい。引っ越しの後 | 大 | 後 |
   | E6 | 外の信号（`signal`）・指標の計測 | 無い | **設計だけ**（問い 23 で P8 の後。指標は §5.1 の「最初の 1 回」も未集計） | — | P8 の後。指標の最初の 1 回は P7 の `log.jsonl` から数えられる（operate.md §13 の末尾） | 中 | 後 |
   | E7 | dispatcher の控えを失ったとき（G8）と引っ越し | （ホストの `yards-up` が手元にしか無いものを数える） | **ある**（operate.md「別のマシンへ引っ越す」。Y-167）。ただし手順は `state/` を写さず、dispatcher の控え（`~/.local/state/andon/dispatcher/`）に触れていない | 写さなければ「初めて見る台帳」。`review` の課題のゲートが回らず、担当に `review` へ入り直させる手間（operate.md §11 の移し方と同じ） | 引っ越しの段の 3 に「dispatcher の控えも写す。写さなければ §11 の 3 の手順」を 1 行 | 小 | 後（Y-129 の手順に） |
   | E8 | public への切り替え | — | 準備済み（Y-126）。切り替えは Y-130 | — | 引っ越しの後（前提 1） | — | 後 |

   **F. 記事（herdr〈TUI〉で複数のエージェントを見守る）との違い**。記事は herdr（TUI。画面の検出で状態を決める）の話で、建屋の役の端末は hc（プログラムの報告で状態を決める）なので、同じ語でも違う所がある。

   | # | 記事の学び | herdr（TUI）では | hc と Herdr App（建屋）では | Andon の扱い |
   |---|---|---|---|---|
   | F1 | 状態は画面の検出で決まり、**新しい形の確認画面は idle に見える**（`agent explain` で `rule: none`） | ペインの最下部の画面を検出ルールと照らす。既知の承認画面だけ `blocked`、ほかは `idle` | 状態はプログラムの OSC 7501 の報告（`idle`・`working`・`done`・`blocked`・`error`）。画面から推さない。ただし**報告が無い状態は同じ形で `idle` に見える**: Codex のレビュアーの空の入力欄（Y-148）、フックで止まった最初のメッセージ（Y-124）、入力欄に文が残ったまま（B5） | 担当の `idle` は `first_message.started`（送り直し）と `stalled`（15 分）で拾う（ある）。B5 は後。`agent explain` 相当は無い（報告なので理由も無い）→ やらない |
   | F2 | `done` と `idle` の違いは**見たかどうか**。緑の印は仕事が終わった証明ではない | クライアントごとの既読。`agent read` では既読にならない | hc の `done` は「ターンが終わった」、`idle` は待機。**既読の概念は無い**。既読はボードと App の側で、Andon が書くのは Chief of Staff の `<no-reply>` のターンの `news.replied` だけ | 担当の `done` は dispatcher が報告（台帳）と `finished` で拾うので「見た」は要らない。人の既読はボード（外部の部品）→ やらない |
   | F3 | **続きの状態は会話ではなくファイルに**（`progress.md`）。会話の再開（`--resume`）は長い会話をそのまま戻す | integration の hook で session の id を保存し `claude --resume`（Codex は戻らない） | hc の `--keep` は中身を戻さない（素のシェルになる）。Andon は再開を使わず、戻った担当は `finished` として起動し直す（Y-155）。続きは台帳（brief・report・`log.jsonl`）と worktree。報告か問いを書くまで止まれない（`stop-check`）。memory は置かない（ADR-0004） | ある（書くだけ）。記事の `progress.md` に当たるのは `report.md`／`report.json` |
   | F4 | **作り手と確かめ役を分け**、確かめ役には要約でなく差分とテスト出力を渡す。別ベンダーだと片方の見落としを拾う | 同じ tab に並べ `agent prompt` で渡す | 別ブランドのレビュアー・detached worktree の差分・plan と報告を見せない（independent-review）・担当と同じ tab に横に並ぶ（ADR-0003） | ある。証拠の質（A4）が残る |
   | F5 | **並列化は曖昧さも増やす**。目標・受け入れ基準・触ってよい範囲と担当を先に書く | `worktree create` で並べる前に書く | 依頼書の完了条件・守ること（触る場所）・ぶつかりの確認・並行の上限（chief-of-staff skill §2・§3）。Chief Engineer の受け入れ条件（テストできる形） | ある。「受け入れ基準はコマンドで判定できる形」は A4 と同じ題 |
   | F6 | 進まなかった理由を**情報不足・権限不足・モデルの力不足**の 3 つに分けて次の依頼に返す | 人が手で | 無い（報告の `next[]`・`remember[]` は自由形式） | C3 で `corrections.json` を入れるとき、種類の列を検討。今はやらない |
   | F7 | 取り決め（人の確認なしにマージしない等）は記憶でなく `AGENTS.md`・フック・CI に | — | managed の deny（`gh pr merge`・人のファイル）・`stop-check`・ゲート・`tests/public.sh`（ポカヨケ） | ある |
   | F8 | 同時に動かす数は責任を持てる数で決め、例外だけ見る | サイドバーの `blocked`・`done` だけ見る | 並行の上限（`project.json` の `chief_of_staff.parallel`。既定 2）。人が見るのは灯り（問い・レベル C）とボードの「自分待ち」 | ある |

3. **2026-11-06 塞ぐ順。** 前提 4 の基準で 3 つに分けた。

   - **引っ越しの前に要る（5 件。全部小さい）**: C1 `kanban tick`（＋C2 ルーチンの写し）・D1 Chief of Staff の `--add-dir`（＋B7 の文言）・A5 人のマージとの重なり（G3）・B1 P6b の実測の記録。
     理由: どれも `home` で回し始めた日から効く。C1 が無いとアウターループが**一度も始まらない**（トリアージのルーチンが撃たれない。仮設では通知の係が撃っていたので気づかなかった）。D1 が無いと規約 9 の出口（wiki）が**物理的に書けない**。A5 はセルフホストでレベル C と A／B が同じ台帳に混ざった日から起きうる誤報。B1 は仮設の台帳と担当がある今のうちに実測しておけば、落ちたとき仮設で直せる（移ったあとに落ちると、直す担当を `home` で起動する前に通知が届かない形になりうる）。
     捨てた案: C3（retro）を前に入れる——無い間は仮設と同じ（トリアージが拾う）で、引っ越しで悪くならない／A3・A4 を前に入れる——レビューの質の話で、ラインが止まる話ではない。
   - **移ってから（Andon の建屋の上で Andon の課題として）**: A3・A4・A6・A7・B5・C3・C5・C6（Y-169）・D2・E5・E6・E7・E8（Y-130）と、仮設で進行中のもの（Y-172・Y-177・Y-160・Y-158・Y-181・Y-152・Y-170・Y-182）は仮設で終われば終わり、終わらなければ Y-129 の束で渡す。
     理由: 無い間は灯りが 1 つ増えるか、仮設と同じ手間が残るだけで、ラインは止まらない。E5・E6 は design.md §5.1 で「その先」と決めてある。
   - **やらない**: B8（`hc events`）・D4（許可ルールを足す。数える）・D6（Codex の Chief of Staff）・E4（自動 push）・F1 の `agent explain` 相当・F2 の既読・F6 の 3 分類。
     理由: 設計で捨てたもの（問い 10・12）、外部の部品の持ち物（既読・App の通知）、無くても動くもの（`hc events`）、数えてから決めるもの（D4）。

4. **2026-11-06 「済」の行は消さない。** P7 の控えがどの直しで入ったかの対応表として残す（A1・A2・B2〜B4・B6・C4・D3・D5・E1〜E3）。

5. **2026-11-10 引っ越しが済んだので、区分を今の main で付け直す。** 「引っ越しの前・移ってから」の区分は使い終えた。各行は **済**（入った PR）・**残る**・**やらない** のどれかにする。判断（決めたこと 1〜4）は変えない。変えたのは古くなった事実と、それに伴う区分だけ。
   - **引っ越しの事実**: 2026-11-09 に、Andon の仕事はセル `home` の台帳から出るようになった。main の PR は、仮設の台帳の番号（`Y-…`）の最後が #100（Y-219。2026-11-09）で、Andon の台帳の番号（`A-…`）の最初が #101（A-0008。2026-11-09）。public への切り替え（E8）は人の決定（2026-11-08）で済んだ（#94・#95）。
   - **元の「引っ越しの前」の 5 件はどれも引っ越しの条件にならなかった**: 引っ越しのときに入っていたのは、どれも無かった。その後に入ったのは D1 だけ（#110）。C1・C2・A5・B1・B7 は残る。前提 4 の基準（回し始めた日から、人が気づかずにラインが止まる、または人が毎回手で補う）で「前」だったという事実は残る。
   - **開いている問い 1（S5 を引っ越しの条件にするか）は答えずに済んだ**: 引っ越しが先に済んだ。
   - **決めたこと 1 の「[decisions.md](../decisions.md) には 1 行」**: decisions.md の表は 2026-11-07 に凍結した（[decisions/2026-11-07-decision-log-one-file-each.md](../decisions/2026-11-07-decision-log-one-file-each.md)）。この ADR の 1 件は [decisions/](../decisions/) に 1 ファイルで足す。

   今の区分（2026-11-10。根拠は origin の main の `bd92577`）:

   | # | 元の順 | 今 | 根拠 |
   |---|---|---|---|
   | A1 | 済 | 済 | Y-142・Y-179（#64）。読み直しと `gh pr merge` の間の隙間は public にしたあと ruleset で塞いだ（#95） |
   | A2 | 済 | 済 | レビュアーの id `<親>-r<回>`（#67。Y-177） |
   | A3 | 後 | 済 | `findings.json` の `preexisting`・2 回目以降は直しの確認だけ（#109。A-0005。[decisions/2026-11-09-review-preexisting.md](../decisions/2026-11-09-review-preexisting.md)・[review-later-rounds](../decisions/2026-11-09-review-later-rounds.md)） |
   | A4 | 後 | 残る（一部済） | `acceptance[]` は使い始めた（#107。A-0004）。CE skill に受け入れ条件の節がある。証拠の質（staged な出力・mock は証拠でない）は [contract.md](../contract.md) §4 の `acceptance[]` の行に無い |
   | A5 | **前** | 残る | 人のマージを dispatcher が拾う口（#114。A-0021。[dispatcher.md](../dispatcher.md) §2.7）は `merge: person`・`needs_human` の課題だけ。dispatcher が打つ `gh pr merge` の失敗は今も問いになる（`internal/dispatcher/loop.go` の `gh pr merge に失敗`）。dispatcher.md §7.3 の G3 のまま |
   | A6 | 後 | 残る | dispatcher.md §7.3 の G1・G2・G6 のまま |
   | A7 | 後 | 残る（一部済） | `andon issue start` は台帳ごとの issue lock を持つ（[ADR-0012](0012-resume-and-worktree-home.md) 決めたこと 13 の 4）。`andon chief-of-staff` は dispatcher.md §7.3 の G5 のまま |
   | B1 | **前** | 残る（人） | [operate.md](../operate.md) §12 は見る手順だけで、4 項目の見た日と結果の記録が無い |
   | B2 | 済 | 済 | dispatcher.md §2.3 |
   | B3 | 済 | 済 | プロバイダに切られたときの再試行（#74。Y-172） |
   | B4 | 済 | 済 | Y-128・Y-148 |
   | B5 | 後 | 残る | 最初のメッセージを送り直す前に入力欄を読まない（`internal/dispatcher/pickup.go` の `sendFirst`） |
   | B6 | 済 | 済 | Y-131・Y-155・Y-180（#63） |
   | B7 | 後 | 残る | `package/environment.md`（「人に気づいてほしいことは `herdr notification show` で出す」）と chief-of-staff skill の 2 か所の文言は変わっていない |
   | B8 | やらない | やらない | 決めたこと 3 |
   | C1 | **前** | 残る | dispatcher.md §2.1.1 の表の下（「今は入っていない」）、§7.2 の #8、§7.3 の G4 のまま |
   | C2 | **前** | 残る（人） | command と skill はある。`home` の台帳にルーチンは無い。C1 が無いと写しても撃たれない |
   | C3 | 後 | 残る（一部済） | `corrections.json` は finish のときに集める（#105。A-0003。dispatcher.md §2.6）。ルーチン `retro` は無い |
   | C4 | 済 | 済 | triage skill |
   | C5 | 後 | 残る | wiki の形と lint は無い。boot は `~/.codex/config.toml` に `approval_policy`・`sandbox_mode` を足すだけで、Codex の記憶は確かめていない（`internal/boot/codex.go`） |
   | C6 | 後 | やらない | 人の答え（2026-11-07）: `AGENTS.md` の行数に上限は無く、lint は作らない（#83。Y-169。[decisions.md](../decisions.md) §20） |
   | D1 | **前** | 済 | Chief of Staff の template の command に `--add-dir /workspace/wiki`（#110。A-0015） |
   | D2 | 後 | 済 | CE は `home` の台帳の課題として動いている（ADR-0013 は A-0035、ADR-0014 は A-0056） |
   | D3 | 済 | 済 | ask-one-by-one skill |
   | D4 | やらない | やらない | 決めたこと 3（数える） |
   | D5 | 済 | 済 | App の mod（#73。Y-152）。Y-181 は人の App の側のまま |
   | D6 | やらない | やらない | 決めたこと 3 |
   | E1〜E3 | 済 | 済 | 決めたこと 4 |
   | E4 | やらない | やらない | 決めたこと 3 |
   | E5 | 後 | 残る | `andon issue start` は `machine` のある課題で止まる（`internal/issue/start.go`）。design.md §5.1 の「その先」 |
   | E6 | 後 | 済（[ADR-0011](0011-signals-and-value-loop.md) に引き継いだ） | 指標を数える `andon measure`（#103。A-0002）。外の信号は ADR-0011（S1 #127・S2 #133）。残りは ADR-0011 の依頼書に切る案が持つ |
   | E7 | 後 | 済 | operate.md「別のマシンへ引っ越す」の 1 で `doing`・`review` を片付けてから台帳を写す。`review` の課題が無いので、dispatcher の控えを写さなくてもゲートの回し損ねは起きない |
   | E8 | 後 | 済 | public にした（人の決定 2026-11-08。#94・#95） |
   | F1 | — | 残る（B5）・やらない（`agent explain` 相当） | B5 と決めたこと 3 |
   | F2 | — | やらない | 決めたこと 3 |
   | F3 | — | 済 | 事実が変わった: 止まった担当はセッションの続きから起動し直す（`andon issue resume`。#129。ADR-0012）。続きの正が台帳と worktree であることは同じ |
   | F4 | — | 済（証拠の質は A4） | independent-review |
   | F5 | — | 済 | chief-of-staff skill §2・§3 |
   | F6 | — | やらない | 決めたこと 3。`corrections.json` は入ったが種類の列は足していない |
   | F7・F8 | — | 済 | 決めたこと 2 の表のまま |

   決めたこと 3 の「移ってから」に挙げた仮設で進行中の課題は、Y-181 のほかは main に入った: Y-172（#74）・Y-177（#67）・Y-160（#71）・Y-158（#69）・Y-152（#73）・Y-170（#84）・Y-182（#78）。決めたこと 3 の「Y-129 の束で渡す」と S1〜S5 の行き先は、下の「依頼書に切る案」の対応の列。

## やらないこと（範囲の外。理由）

- 決めたこと 3 の「やらない」の 7 件（理由はそこに）。
- **引っ越しの手順そのもの**（operate.md の 2 つの段が正。Y-129 が束を作る）。
- **仮設の工場を直すこと**（design.md §5.3 の 1。仮設は読むだけ）。
- **記事の herdr（TUI）の CLI（`agent prompt`・`pane wait-output`・`worktree create`）を Andon の道具に写すこと**: 役の端末は hc で、同じ仕事は `hc send`・`hc wait`・`andon issue start` が持つ。
- **新しい役や約束を足すこと**: この ADR の塞ぎ方は全部、今ある dispatcher・template・skill・contract の任意キーの中に収まる（`preexisting` も版を上げない）。
- （2026-11-10）**決めたこと 5 で区分を付け直すときに、新しい判断を足すこと**: 区分を変えるだけで済まないものは、決めずに人へ回す。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

（2026-11-10 に書き直した。最初の 1〜6 は PR #72 の版にある。2〜5 の中身は下の R1〜R4 の完了条件に移し、6〈引っ越しの条件〉は引っ越しが済んだので外した）

1. **この文書（`tests/public.sh`・`tests/run --changed`）**: 固有の値（セル名・案件名・ホスト名・番号）が無い。[decisions/](../decisions/) に 1 ファイル、README の ADR の地図に 1 つ。[vision.md](../vision.md) の ADR-0007 への言及が決めたこと 5 と食い違わない。
2. **決めたこと 5 の表**: 表 A〜F の全部の行が「済（PR）・残る・やらない」のどれかで、残るものはどれも下の「依頼書に切る案」のどれかに入っている（E5 と C5 の wiki の形は切らない。理由は表の下）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

（2026-11-10 に書き直した。残るものだけ。最初の S1〜S5 との対応: S1〈この ADR の PR〉→ この PR。S2 → R1。S3 → D1 は済、B7 だけ R3。S4 → R2。S5 → R4。最後の行〈Y-129 の束〉→ 引っ越しが済んだので無し）

| # | 題 | なぜ | 完了条件 | 大きさ・プロファイル・Settings | 依存 |
|---|---|---|---|---|---|
| R1 | dispatcher の 1 巡で `kanban tick` を打つ（C1） | ルーチンが 1 つも撃たれず、週 1 のトリアージも始まらない。アウターループの入口が開かない。順は dispatcher.md §2.1.1 で固定済み | `andon dispatcher` が各プロジェクトの台帳で、Chief of Staff が生きているときだけ、1 巡の 3 と 4 の間に `kanban tick` を 1 回打つ。失敗はログ 1 行で巡を止めない。`tick` が足した課題が同じ巡の `ready` の差で拾われる（`go test ./internal/dispatcher/...`・`tests/andon-dispatcher.sh`）。dispatcher.md §2.1・§2.1.1・§7.2 #8・§7.3 G4 を直し、`decisions/` に 1 ファイル | 小・claude・Review on（dispatcher の変更）・Must fix major・Merge 人 | この ADR のマージ |
| R2 | 人のマージと dispatcher のマージが重なったときの誤報を無くす（A5・G3） | `merge: auto` の課題を人が先にマージすると、dispatcher の `gh pr merge` が失敗して `irreversible` の問い（誤報）が出る。人のマージを拾う口（dispatcher.md §2.7）は `merge: person`・`needs_human` の課題だけ | `gh pr merge` の失敗のあと `gh pr view --json state,mergeCommit` を読み、`MERGED` で head がゲートの記録の `head` と同じなら、問いを出さず finish へ進む（§2.7 と同じ道。ゲートの記録の `merged` は書かない）。違えば今までどおり問い（`go test ./internal/dispatcher/...`）。dispatcher.md §7.3 の G3 を §7.2 の #5 へ移し、`decisions/` に 1 ファイル | 小・claude・Review on（マージの段）・Merge 人 | この ADR のマージ。R1 と独立 |
| R3 | `herdr notification show` の文言を「herdr〈TUI〉が動いているときだけ効く」に直す（B7） | Herdr App で使っている人には届かず、Chief of Staff が効かないコマンドを打つ | `package/environment.md` と chief-of-staff skill の 2 か所が「TUI が動いているときだけ効く。無ければ台帳の問いとボードで足りる」と書く（`tests/package.sh`） | 小・sonnet・Review off・Merge 人 | この ADR のマージ。R1・R2 と独立 |
| R4 | （人）P6b の 4 項目を実測して記録し、`routines/triage.yaml` を台帳に写す（B1・C2） | operate.md §12 は手順だけで、フックの 4 つが本物の Claude Code で効いたかの記録が無い。ルーチンは人の go で写すと決めてある（triage skill） | operate.md §12 の 4 項目それぞれに「見た日・結果」の 1 行。落ちた項目は課題にする（Chief of Staff が切る）。台帳に `routines/triage.yaml` があり `kanban routine list` に出る | 小・人（`who: person`） | 実測は今できる。ルーチンの写しは R1 のマージ（`tick` が無いと撃たれない） |
| R5 | 受け入れ条件の証拠の質を書く（A4） | 「通した」が言葉だけになる。`acceptance[]` は使い始めたが、証拠に何を書けばよいかが無い | contract.md §4 の `acceptance[]` の行と CE skill の受け入れ条件の節に、証拠はテストの command と出力のパスで、staged な出力と mock は証拠でない、と 1 段（`tests/package.sh`）。版は上げない | 小・claude・Review off・Merge 人 | この ADR のマージ |
| R6 | 入力欄に残った最初のメッセージを送り直しで重ねない（B5） | `hc send` で貼った文が入力欄に残ると、送り直しで同じ指示が 1 回の送信に 2 つ並ぶ | `sendFirst` が送る前に `hc read` で入力欄を読み、最初のメッセージの 1 行目が残っていれば Enter だけ押す（`go test ./internal/dispatcher/...`） | 小・claude・Review on・Merge 人 | この ADR のマージ |
| R7 | lock の守りを足す（A6・A7。G1・G2・G5・G6） | 手で 2 本打つと記録の上書き・PR のコメントの二重・Chief of Staff が 2 人になる。どれも実害は小さい | dispatcher.md §7.3 の「直すなら」のとおり `.dispatcher.lock`・ゲートの head ごとの lock・`andon chief-of-staff` の lock・`boot.lock`。直したものは §7.3 から §7.2 へ移す（`go test ./...`） | 小〜中・claude・Review on・Merge 人。1 つずつ切ってよい | この ADR のマージ |
| R8 | 振り返りのルーチン `retro` の題（C3） | `corrections.json` は集まるようになったが、読んで環境を直す口が無い | CE の課題として題を立て、文書（ADR）にする。ルーチンの実装はその文書が切る | CE（`andon chief-engineer`） | この ADR のマージ |
| R9 | Codex の記憶を確かめる（C5） | Codex に自動の記憶があれば、セルの外から見えない記憶が育つ（ADR-0004 で auto memory を止めたのと同じ理由） | Codex の今の版で記憶の設定（`~/.codex/config.toml`）を調べ、あれば boot で切る（`go test ./internal/boot/...`）、無ければ報告に書いて閉じる | 小・codex・Review on（`claude-review`）・Merge 人 | この ADR のマージ |

E5（作業側を別のマシンに出す）は切らない。design.md §5.1 の「その先」で、題が立ってから CE で決める。C5 の wiki の形と lint も切らない。wiki を書く題が出てから（決めたこと 2 の C5 の行）。

## 開いている問い（人に聞く。1 問ずつ。推しつき）

1. ~~S5（人の実測）を引っ越しの条件にするか~~。（2026-11-10: 答えずに済んだ。引っ越しが先に済んだ。決めたこと 5）

## 出典（読んだもの・捨てた案の出所）

- 人の依頼書（Y-143。補足 3 つ: 記事・Y-148 の実測・ADR-0004 の継ぎ目と Y-161 の候補と G1〜G8）。
- P7 の控えを直した仮設の台帳の課題（依頼書と報告）: Y-121・Y-122・Y-123・Y-124・Y-125・Y-126・Y-128・Y-129・Y-130・Y-131（A-1 の 1・2）・Y-132（3）・Y-133（4・5）・Y-134・Y-136・Y-137（11・12・13）・Y-140・Y-141・Y-142・Y-148（6）・Y-155（v0.0.1 の 17）・Y-159（v0.0.2 の 21）・Y-173（G1〜G8）・Y-179（v0.0.3 の 25）・Y-180（22）・Y-181（24）。P7 の課題（Y-107）は人の課題で、控えの番号はこれらの依頼書に写っているものを引いた。
- 仮設の工場の正（読むだけ）: 環境メモ（マルチエージェント・フォーメーションの節）、skill（yard-coordinator §0〜§10・yard-task-agent・independent-review・ask-one-by-one・yard-triage）、`lib/yard/`（news の見回りの周期と `deck tick`・idle 15 分・working 20 分・プロバイダのエラーの 1・5・15 分／gate の L1〜L4／publish／triage）、template-yard の `docs/orchestration.md` §3.3（ルーチンと通知の係）・§3.4（reflect）。
- Andon の側: [design.md](../design.md) §0.1・§1.3〜§1.5・§1.8〜§1.10・§2.3・§2.4・§5.1〜§5.4、[contract.md](../contract.md) §0〜§9・§11、[dispatcher.md](../dispatcher.md) §2〜§7、[operate.md](../operate.md)（役の配置と起動・§8・§11・§12・§13）、[decisions.md](../decisions.md) §3・§6・§13〜§18、[ADR-0001](0001-skills-distribution.md)〜[0005](0005-cell-repo.md)、`package/skills/`（chief-of-staff・worker・chief-engineer・independent-review・triage）、`package/templates/chief-of-staff.md`、`package/settings.json`・`package/hooks/`・`package/environment.md`、`schema/report.schema.json`・`gate.schema.json`、`internal/dispatcher`（pickup.go の再送・`tick` と `hc events` の不在）・`internal/gate/decide.go`（`acceptance`）・`init/wiki/`。
- Y-161 の調べ `tasks/Y-161/library/shepherdr-evolution.md` §8.3（memory の 3 つの家）・§9（取り入れる A1〜E1・取り入れない N1〜N12・保留 P1〜P3）。
- 記事: 「Herdrで、複数のエージェントを見守りたい」（2026-11-06。herdr 0.9.5・Claude Code 2.1.318・codex-cli 0.166.0 で確かめたとある）。読んだのは状態の検出・done と idle・integration と再開・別のエージェントからの操作・仕事の変わり方・手順のまとめの節。
- hc の決まり: `hc --skill`（status はプログラムの OSC 7501 の報告。画面から推さない）。
- 出し直し（2026-11-10。A-0058）: origin の main の `bd92577` の [dispatcher.md](../dispatcher.md) §2.1・§2.1.1・§2.6・§2.7・§7.2・§7.3、[operate.md](../operate.md)「別のマシンへ引っ越す」・§12、[contract.md](../contract.md) §4、[decisions.md](../decisions.md) §20、[decisions/](../decisions/)、[ADR-0011](0011-signals-and-value-loop.md)〜[ADR-0013](0013-short-role-names-and-plain-japanese.md)、`package/templates/chief-of-staff.md`・`package/environment.md`・`package/skills/chief-of-staff`、`internal/dispatcher/loop.go`・`pickup.go`・`humanmerge.go`、`internal/issue/start.go`、`internal/boot/codex.go`、main の commit の一覧（PR 番号と日付）、PR #72 の本文。
