# ADR-0010: 通知を「記録だけ」と「届ける」に分け、`<no-reply>` を `andon news ack` に替える（仕様・設計。2026-11-09。課題 A-0009）

- 状態: **提案（2026-11-09）**。人が PR をマージした日が決定の日（[decisions/2026-11-09-adr-0010-news-ack.md](../decisions/2026-11-09-adr-0010-news-ack.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: dispatcher が Chief of Staff に届ける通知のうち、Chief of Staff が受けても何もしないものを届けないこと（記録には残す）。Chief of Staff が「人に言うことが無い」と示す方法を、答えの文字列（`<no-reply>`）から command（`andon news ack`）に替えること。移行のしかた。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。この文書が足す約束（記録だけの通知の置き場・ラベル `news.ack`）は「決めたこと」に形を書き、実装の課題が contract.md・`schema/`・dispatcher.md・chief-of-staff skill に写す（それまでは提案）。
- 入力: 依頼書 A-0009（人の発言と Chief of Staff の推し。2026-11-09）、[dispatcher.md](../dispatcher.md) §2.2〜§3.1・§5、[contract.md](../contract.md) §0・§6・§8・§9、chief-of-staff skill §5 の「`<news>` の読み方」の表、[decisions.md](../decisions.md) の 2026-11-03（配達役を1つに）・2026-11-05（F8・`news.replied` の決め方）の行、`internal/dispatcher/`（今の拾い方。出典）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-09）: 「no-reply と返すとかってどうなんだろうね。もっと上手いことできないのかな」。

言い直し。今の dispatcher は、拾った出来事をほぼ全部 `<news>` にして Chief of Staff に届ける。Chief of Staff は人に言うことが無ければ `<no-reply>` とだけ返し、`Stop` フック（`andon dispatcher drain`）がその答えの文字列を照合して、ボードとの約束のラベル `news.replied=<turn>` を書く。困りごとは3つ:

1. **機械がモデルの自由文を文字列で照合している。** contract.md §0 の原則1（機械は Markdown・自由文を読まない。判定に使う値は JSON／YAML のキーにだけ持つ）と食い違う。前後に1文字足せば外れ、外れたターンはボードで「人が見る返事」に数えられる。
2. **受けても何もしない通知にも、モデルのターンを1回使う。** chief-of-staff skill の表で「受けたら」が「何もしない」の行（`review-requested`・担当の `review`・`gate: fix`・`merged` など）は、届くたびにターンになり、その答えが `<no-reply>` になるだけ。
3. **人の既定の入口である Chief of Staff の画面に、人向けでないターンが並ぶ。** A-0008 が Chief of Staff を人の既定の入口にしている最中で、画面の多くが `<news>` と `<no-reply>` の往復になると、人の読む行が埋もれる。

足りないのは2つ——(a) 「判断が要る」と「記録だけ」を dispatcher が通知の種類で分ける線、(b) 「言うことが無い」を文字でなく機械が読める値で示す口。

## 前提（番号。依頼書・現地から取ったもの。同意していない行は「推し」と記す）

1. **contract.md §0 原則1**: 機械は自由文を読まない。→ `<no-reply>` の照合は外す（決めたこと 4・5）。
2. **contract.md §0 原則3**: 1つの約束に書き手は1人。→ 記録だけの通知は dispatcher だけが書き、`news.ack` は Chief of Staff だけが書く（決めたこと 2・4）。
3. **design.md §1.9 の 5（薄いハーネス）・依頼書の「守ること」**: 新しい役を足さない。→ headless の Chief of Staff を別に立てる案は捨てた（捨てた案 1）。
4. **decisions.md §7 の F8（2026-11-05）**: `news.replied=<turn>` はボード（herdr-flightdeck 側）の持ち物で、変えるときは Andon から知らせる。→ ラベルの**意味**（「そのターンは人が見る返事ではない」）は変えない。変えるのは Andon の側で**書くきっかけ**だけ（決めたこと 5）。
5. **dispatcher.md §2.2・§5（P6a）**: インナーループ（レビュアーを起動する・ゲート・マージ・`fix` を担当へ返す・片付け）は dispatcher が回し、Chief of Staff はターンを使わない。判断が要るときは別の通知（`asked`・`gate: approve` で止まった・`report` で起動しなかった）で届く。→ インナーループが自分で進めた結果の通知は記録だけでよい（決めたこと 1）。
6. **Chief of Staff の推し（依頼書。覆してよい）**: kind ごとの分け方・記録だけの JSON Lines・`andon news ack`・移行の間は `<no-reply>` も受ける。→ 決めたこと 1〜6 でほぼそのまま取った。違えたのは `gate: approve` の届け方とレビュアーの課題の `report`（決めたこと 1 の注。理由は捨てた案 3・4）。
7. **今の文書どうしの食い違い（直さない。報告に残す）**: chief-of-staff skill §5 の表は、プロバイダのエラーを「3回」「1・5・15分」、`working` の静かさを「60分」と書く。dispatcher.md §2.3・§2.4 は「5回目に切られたら知らせる」「1・2・4・8分」「20分」。依頼書の「プロバイダのエラー3回」は skill の写し。**dispatcher.md を正とする**（この文書の表は dispatcher.md の言い方で書く）。skill の表を揃えるのは実装の課題（依頼書に切る案 3）。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-09 通知の kind ごとに「届ける」か「記録だけ」かを固定する。** 判断の線は「Chief of Staff がそれを受けて、すること（判断・人への1行・担当への依頼）があるか」。インナーループが自分で次の工程に進めた結果と、別の通知で同じことが届くものは記録だけにする。

   | `kind`（と条件） | 扱い | 理由（受けたら何をするか） |
   |---|---|---|
   | `review-requested` | **記録だけ** | dispatcher がレビュアーを起動した。Chief of Staff は何もしない（skill の表） |
   | `review`（担当の課題。`issue.yaml` の `profile` が `chief-engineer` でない） | **記録だけ** | dispatcher がゲートを回す。判断が要れば `gate: approve` で止まったとき・`asked` で届く |
   | `review`（Chief Engineer の課題。`profile: chief-engineer`） | 届ける | Chief Engineer の文書を受けて課題に切る（skill §1） |
   | `gate`（`decision: fix`） | **記録だけ** | dispatcher が理由を担当へ返した。Chief of Staff は何もしない |
   | `gate`（`decision: needs_human`） | **記録だけ** | 同じ巡で `asked`（`from: gate`）が届く。重複 |
   | `gate`（`decision: approve`。dispatcher がマージしないと決まっている: `merge: person`・委譲の外・`merge: auto` を最後に書いたのが Chief of Staff でない） | 届ける（すぐ） | 人にマージを頼む（skill §6 の「approve で止まったとき」） |
   | `gate`（`decision: approve`。`merge: auto` でマージに進む） | **記録だけ**。approve を記録してから **10 分**たっても同じ課題の `merged` も `asked` も無ければ、そのとき「approve のまま止まった」として `gate` を届ける | ふつうは同じ巡で `merged` まで進む。進まないときだけ Chief of Staff が確かめる |
   | `merged` | **記録だけ** | dispatcher がマージして片付けた。影響は `impact` で別に届く |
   | `report`（担当の課題・調べる課題・repo の無い課題） | 届ける | 報告を読む（起動しなかったレビュー依頼・`## 人に聞くこと`・調べる課題の候補） |
   | `report`（レビュアーの課題。`parent` があり、dispatcher が起動したもの） | **記録だけ** | 結果の宛先は元の課題の担当で、Chief of Staff は中継しない（skill §6） |
   | `ready` | 届ける（**今は**） | 並行の上限と、動いている課題とのぶつかりを確かめて `andon issue start` する。素通り（確かめて起動するだけ）の数を記録で見てから見直す（やらないこと 2） |
   | `asked` | 届ける | 問いを間引いて人へか、自分で答える（skill §4・§8） |
   | `impact` | 届ける | rebase を頼む・止めて向きを確かめる（skill §7） |
   | `blocked`・`error`・`finished`・`stalled` | 届ける | 許可の確認・人へ知らせる・続きを促す（skill §4・§5）。プロバイダに切られた `error` は、dispatcher が「続けて」を4回打ったあと5回目に切られたときだけ（dispatcher.md §2.4。今と同じ） |
   | `signal`（最初の版の後。P6 の後） | 届ける | トリアージの候補にする |

   - 「届ける」は今と同じ経路（①②③。dispatcher.md §3）で、同じ `<news>` の文に描く。「記録だけ」は `<news>` に載せず、③の `hc send` のきっかけにもしない（待ちに数えない）。
   - `gate: approve` の 10 分は dispatcher の時計で、記録だけの行の `at` から測る。見回りが1分ごとなので、マージが遅い repo（merge queue）でも10巡は待つ。値は仮の値で、ほかの間（dispatcher.md §2.1）と同じく設定で替えられる。
   - Chief Engineer の課題・レビュアーの課題の見分けは、今の dispatcher の見分け（`profile: chief-engineer`・`parent` とレビュアーの作成の行。`internal/dispatcher/` の `isReviewer`）をそのまま使う。新しいキーは足さない。

   理由: 困りごと 2・3 は「受けても何もしない通知が届く」ことから来る。kind（と今あるキー）で線を引けば、判断は機械で決まり（決定式）、Chief of Staff の skill の表から「何もしない」の行が消える。線を Chief of Staff に引かせる（届けてから「無視してよい」と書く）のでは、ターンが減らない。

2. **2026-11-09 記録だけの通知も消さない。dispatcher が控えの横に JSON Lines で追記する。** 置き場は `~/.local/state/andon/dispatcher/<slug>.recorded.jsonl`（`XDG_STATE_HOME` があればその下。控え `<slug>.json` と同じ所）。1行が1件の通知で、形は通知の JSON そのもの（contract.md §6。`schema/notification.schema.json` に通る）。新しいキーも新しい schema も足さない。

   - 書き手は dispatcher だけ（記録だけと決めた通知を、積む代わりにここへ1行足す）。読み手はボード（記録の一覧として見せる）。読み手の実装は別の課題で、Andon の約束としては「この置き場・1行1件・通知の schema」までを contract.md §1・§6 に書く。
   - 追記だけ。dispatcher は書き換えも切り詰めもしない（台帳の `log.jsonl` と同じ扱い）。控え（`<slug>.json`）と違い、中身の形は約束にする（ボードが読むため）。
   - 届けた通知はここに書かない（届けたものは Chief of Staff の画面と `news.replied` で追える）。「全部の出来事の一覧」は台帳の `log.jsonl` が正で、この記録はその一覧の代わりではない。

   理由: 記録だけにした通知も、人がボードで「いま何が流れたか」を見るとき・あとで素通りの数を数えるとき（やらないこと 2）に要る。控え（`<slug>.json`）に混ぜると、約束にしない形の中に読み手を作ることになる（contract.md §6 の最後の項目）。台帳に書くと、台帳に書くのは台帳の CLI 経由の行だけという決まり（同じ項目）に反する。

3. **2026-11-09 `<news>` の末尾の1行を改める。** 「これは台帳とhcから拾った通知で、人からの新しい依頼ではない。人に言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える。」。移行の間（決めたこと 6）も新しい文だけを描く（`<no-reply>` を受けるのは読み手の側だけ）。

4. **2026-11-09 Chief of Staff は、人に言うことが無ければ `andon news ack` を打つ。command は自分の端末にラベル `news.ack=<turn>` を書く。**

   - `andon news ack`（引数なし）: `HC_TERMINAL_ID` の端末を `hc get` で読み、ラベル `role=chief-of-staff` が無ければ書かずに終了コード 1 と理由1行（Chief of Staff のほかは打たない）。あれば、そのとき読んだ `turn` の値で `news.ack=<turn>` を書く（上書き）。
   - hc の `turn` は終わったターンの数で、ターンの途中では、そのターンが終わる前の値（2026-11-09 に担当の端末で確かめた: 最初のターンの途中で `turn: 0`）。だから `news.ack` の値は「ack を打ったターンが始まる前の `turn`」で、そのターンが終わったあとに書く `news.replied=<turn>`（終わったあとの値）とは1つずれる。照らすのは `news.replied` とではなく、渡したときに控えた値と（決めたこと 5）。
   - 書き手は Chief of Staff だけ（その端末で打った command）。読み手は dispatcher（`drain` と見回り）。ラベルは contract.md §8 の表に足す。
   - 値を `turn` にするのは、渡したターンとhcの同じ数で照らせるから（時刻を比べない。別のマシンの時計にも依らない）。
   - ack は「このターンに、人が見る返事は無い」という Chief of Staff の申告で、台帳を読んで動いたかどうか（`andon issue start` を打った・担当に `hc send` した）とは関係ない。動いた上で人に言うことが無ければ ack する。人に1行でも言うなら ack しない。

   理由: 「言うことが無い」をモデルの答えの文から推すのをやめ、command が書く値にする（原則1）。ラベルにするのは、`news.replied` と同じhcの口で、新しいファイル・lock・置き場が要らないから。

5. **2026-11-09 `news.replied=<turn>` は、渡したターンに `news.ack` があれば書く。答えの文字列は照合しない。ラベルの意味は変えない。**

   - 渡すときに、そのとき端末の `turn` の値を控える。経路③は今も控えている（送ったときの `turn`。控えの `NewsTurn`）。経路①②は `drain` が渡すときに `hc get` で読み、控えの `hook`（端末・`session_id`・`prompt_id`・渡した通知）に足す。
   - 経路①②（フック）: 渡したターンの `Stop` で `drain` が確かめ済みにするとき（今と同じ `prompt_id`・`session_id` の照合。decisions.md の 2026-11-05 の行）、ラベル `news.ack` の値が控えた `turn` と同じなら `news.replied=<turn>`（今と同じく、そのときの端末の `turn`）を書く。`last_assistant_message` は読まない。
   - 経路③（`hc send`）: 送ったターンが終わって入力待ちに戻ったのを見回りが見たとき（今と同じ `turn` が進んだかの見分け）、`news.ack` が控えた `NewsTurn` と同じなら `news.replied=<turn>` を書く。画面の文は読まない。
   - 要実測（実装の課題）: `Stop` の `decision: block` で始まる続きのターンで、渡したときの `turn` と ack を打ったときの `turn` が同じ値になるか（contract.md §9 の「要実測」と同じ所）。違えば、控える値を「渡したあとに最初に始まるターンの値」に合わせ、この項目に下から足す。前のターンの ack（値が控えより小さい）は数えない、は変えない。
   - ack が無いターンは今と同じく書かない（ボードは人が見る返事に数える）。安全側（原則2）: 申告が無いものは人が見る。
   - `news.replied` の意味（「そのターンは人が見る返事ではない」）・名前・値（`<turn>`）は変えない。変わるのは Andon の側で書くきっかけだけなので、F8 の「変えるときは向こうへ知らせる」には当たらないが、きっかけが変わったことは decisions に1件書き、herdr-flightdeck 側にも1行知らせる（依頼書に切る案 4）。

6. **2026-11-09 移行の間は `<no-reply>` も受け、ack を入れた版の次の版で外す。**

   - ack を入れた版（実装の課題がマージされて切る版。v0.0.5 の見込み）: `drain` と見回りは、`news.ack` が合うか、**または**今までどおり答えが `<no-reply>` だけなら `news.replied` を書く。`<news>` の末尾の文と skill は ack だけを教える。
   - その次の版（v0.0.6 の見込み）: `<no-reply>` の照合を外す。外す版の CHANGELOG の「上げるときにすること」に「Chief of Staff の skill を入れ直す（`<no-reply>` は受けない）」と書く。
   - 版の番号は、ack が実際に入った版で読み替える（「ack を入れた版の次の版で外す」が決定。番号は見込み）。

   理由: Chief of Staff の skill は image に焼かれ、セルの Chief of Staff は起き直すまで古い skill のまま `<no-reply>` を返しうる。1版だけ両方を受ければ、image を上げたセルの Chief of Staff が起き直すまでの間も `news.replied` が書かれる（contract.md §10 の「古い版を読む側を1版だけ残す」と同じ長さ）。

7. **2026-11-10 決めたこと 1 の表の2つの行を、実装（A-0010）で今ある項目に合わせて読む。**（[decisions/2026-11-10-record-only-gated-review-held-merged.md](../decisions/2026-11-10-record-only-gated-review-held-merged.md)）

   - `review` を記録だけにするのは、dispatcher がゲートを回す課題（`gated`）だけ。Chief Engineer の課題に加えて、調べる課題（`deliverable: candidates`）・repo の無い課題など、ゲートを回さない課題の `review` は届ける（記録だけの理由「dispatcher がゲートを回す」が成り立たないため。ゲートを回さない課題を記録だけにすると、誰も読まない）。
   - `merged` のうち `held`（人がマージしたが片付けていない。dispatcher.md §2.7）は届ける（記録だけの理由「マージして片付けた」が成り立たない。Chief of Staff が問いを閉じるか finish する）。
   - 表に無い `kind`（`closed`・`corrections`）は今までどおり届ける。

## やらないこと（範囲の外。理由）

1. **コード・schema・contract.md・dispatcher.md・skill を、この PR で直すこと**: この文書は提案。写すのは実装の課題（依頼書に切る案）。
2. **`ready` を記録だけにすること**: いま `ready` を受けた Chief of Staff は、並行の上限と動いている課題とのぶつかりを確かめてから起動する（判断がある）。ルーチンが足した課題など、確かめて起動するだけの「素通り」が多いなら dispatcher に寄せる余地があるが、数を見てからにする（数え方は、`ready` を届けたターンが `andon issue start` と ack で終わった数。決めるのは別の ADR か小さな決定）。
3. **担当の `report` を細かく分けること**（`andon issue review-request` の報告の書き込みで届く `report` など）: 今は kind ＝ `report` で一律に届ける。記録だけにするには「dispatcher がレビュアーを起動したか」を報告の通知と突き合わせる必要があり、線が kind で引けなくなる。2 と同じく数を見てから。
4. **`<news>` の文の形（1行1件・まとめて1通）を変えること**: 困りごとは届く数と答えの照合で、文の形ではない。
5. **人に届ける・人を呼ぶ線を変えること**: 人へ上げるのは今までどおり Chief of Staff（dispatcher は人を呼ばない。dispatcher.md §0）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は `tests/andon-dispatcher.sh`（dispatcher の見回りと `drain`）と、ack の command のテスト。

1. 記録だけの kind（決めたこと 1 の表）の出来事は、`drain` の出力（`<news>`）に載らず、`<slug>.recorded.jsonl` に1行ずつ足され、その行が `schema/notification.schema.json` に通る。記録だけの出来事しか無い巡では、③の `hc send` を打たない。
2. 届ける kind の出来事は、今までどおり `<news>` に載る。`<news>` の末尾の文は決めたこと 3 のもの。
3. `gate: approve`（`merge: auto`）のあと 10 分 `merged` も `asked` も無ければ、`gate` が1回だけ `<news>` に載る。`merge: person` の approve は同じ巡で載る。
4. `andon news ack` は、`role=chief-of-staff` の端末では `news.ack=<turn>` を書き、ほかの端末では書かずに終了コード 1。
5. フックで渡したターンの `Stop` で、`news.ack` が渡したときに控えた `turn` と同じなら `news.replied=<turn>` が書かれ、違えば（ack が無い・前のターンの値）書かれない。③で送ったターンも同じ。答えの文字列が何であっても結果は変わらない（移行の間は 6 を除く）。
6. 移行の版では、ack が無くても答えが `<no-reply>` だけなら `news.replied` が書かれる。次の版のテストではこの項目を消し、`<no-reply>` だけの答えで書かれないことを確かめる。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **記録だけに分ける**（dispatcher）: 決めたこと 1・2、受け入れ条件 1〜3。contract.md §1・§6 に `<slug>.recorded.jsonl` と表、dispatcher.md §2.2・§3 に「記録だけ」を写す。
2. **`andon news ack` と `news.replied` の書くきっかけ**（command・`drain`・見回り）: 決めたこと 3〜6、受け入れ条件 4〜6。contract.md §6・§8・§9 を写す。1 と独立。
3. **chief-of-staff skill を揃える**: §5 の `<news>` の読み方の表から記録だけの行を消し、`<no-reply>` を ack に、§7 の「影響なしだけなら `<no-reply>`」を ack に。前提 7 の食い違い（プロバイダのエラーの回数と間・`working` の分）を dispatcher.md に揃える。1・2 のあと（同じ版に入れる）。
4. **herdr-flightdeck 側へ知らせる**: `news.replied` の書くきっかけが ack になった（意味は同じ）と1行。ボードが `<slug>.recorded.jsonl` を読むかは向こうの判断。2 のあと。Chief of Staff か人。
5. **`<no-reply>` を外す**: 決めたこと 6 の次の版。受け入れ条件 6 の入れ替え・CHANGELOG の1行。

## 開いている問い（人に聞く。1 問ずつ。推奨案つき）

無し。`gate: approve` の 10 分と `ready` の見直しは、仮の値・数を見てからと決めた（決めたこと 1・やらないこと 2）。

## 捨てた案（理由）

1. **headless の Chief of Staff を別に立てる**（通知は画面の無い Chief of Staff が受け、人の入口の Chief of Staff には人向けのことだけを渡す）: 新しい役を足すことになる（前提 3）。2人の Chief of Staff が同じ台帳を持つと、課題を起動する・依頼書を書く・人に問うの書き手が2人になり（原則3）、どちらがどこまで決めたかを合わせる約束が要る。困りごと 2・3 は、届けない通知を減らすこと（決めたこと 1）でほぼ消える。
2. **`<no-reply>` の照合を残し、文字列を厳しくする**（前後の空白だけ許す・タグの形を固定する）: 機械が自由文を読むこと自体が原則1と食い違う。厳しくするほど、モデルが1文字足したときに外れる。
3. **`gate: approve` を条件によらず届けず、N 分たってから届ける**（依頼書の推し）: `merge: person`・委譲の外の approve は、dispatcher がマージしないことがその場で決まっていて、Chief of Staff は人にマージを頼む（skill §6）。N 分待つと、人への依頼がその分遅れるだけで、待って解ける見込みが無い。だから dispatcher がマージしないと決まっている approve はすぐ届け、マージに進むものだけ N 分待つ（決めたこと 1）。
4. **レビュアーの課題の `report` も届ける**（依頼書の推しは `report` を一律に届ける）: レビュアーの結果の宛先は元の課題の担当で、Chief of Staff は指摘を中継しない（skill §6）。受けてすることが無いので、kind が `report` でも記録だけにする。見分けは今の dispatcher が持っている（決めたこと 1 の最後の注）。
5. **ack をファイルに書く**（`~/.local/state/andon/<slug>.ack.json` に時刻を書き、渡した時刻と比べる）: 新しい置き場と lock が要り、時刻の比べ合いは別のマシンの Chief of Staff で崩れうる。ラベルなら `news.replied` と同じ口・同じ数（`turn`）で照らせる。
6. **ack を台帳に書く**（`deck` の行にする）: 台帳は課題の記録で、Chief of Staff の画面の扱いは課題に属さない。台帳の `log.jsonl` に課題でない行を足すことになる。
7. **Chief of Staff に、受けた通知ごとに「無視してよい」と書いて届ける**（届けたまま skill で何もしないを教える）: ターンが減らない（困りごと 2）。
8. **移行の間を置かず、すぐ `<no-reply>` を外す**: image を上げてから Chief of Staff が起き直すまで、古い skill の答えで `news.replied` が書かれず、ボードに人向けでないターンが並ぶ。1版だけ両方を受けるのは安い（決めたこと 6）。

## 出典（読んだもの）

- [dispatcher.md](../dispatcher.md) §2.1.1（1巡の順序）・§2.2（通知の元）・§2.3・§2.4（止まった担当・プロバイダに切られたとき）・§3・§3.1（経路と `<news>` の文）・§5（P6a）。
- [contract.md](../contract.md) §0（原則）・§6（通知）・§8（ラベル）・§9（フック）・§10（互換と版）。
- chief-of-staff skill（`package/skills/chief-of-staff/SKILL.md`）§5 の「`<news>` の読み方」の表・§6・§7。
- [decisions.md](../decisions.md) の 2026-11-03（通知は1つの配達役が届ける）・2026-11-05（F8・フックで渡した `<news>` の `news.replied` の決め方・控えの `hook`）の行。
- `internal/dispatcher/`（`isReviewer`・`profile: chief-engineer` の見分け・`report` と `review` の積み方）。
