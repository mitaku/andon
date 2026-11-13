# ADR-0015: inbox を Needs you と役への入口に絞る（仕様・設計。2026-11-10。Chief Engineer の課題 A-0074）

- 状態: **決定（2026-11-10）**。人が 2026-11-10 に「決めた」と言った（Chief Engineer の対話 A-0074）。人が PR をマージした日が決定の日（[decisions/2026-11-10-adr-0015-inbox-needs-you.md](../decisions/2026-11-10-adr-0015-inbox-needs-you.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: Andon 同梱の App の mod `inbox`（`package/host/inbox`）が何を出すか。人が手を動かすもの（Needs you）と、プロジェクトで動く役（Chief of Staff・Chief Engineer）への入口の2つに絞り、担当の Done・Working・Idle を人の視界から下げる。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`（この題は新しい約束を足さない。前提 4）。
- 入力: 依頼書 A-0074（人の気付き 2026-11-10 と Chief of Staff の経緯）、`package/host/inbox/`（main.js・README.md・RECIPE.md）、[ADR-0002](0002-herdr-spaces.md)（決めたこと 2・15・18・21）、[ADR-0009](0009-cos-default-entry.md)（人の入口は Chief of Staff）、[ADR-0010](0010-news-ack.md)（担当の通知は dispatcher が Chief of Staff へ）、[ledger.md](../ledger.md)（Needs you is not a status・Every stopped agent has asked）、[decisions.md](../decisions.md) 2026-11-06（mod の取り込み）、台帳 A-0036（Chief Engineer の端末の通知を Chief of Staff に届けない）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-10）: 「Mod の Inbox だけど、あんまり Done にも Working にも興味がないのかもしれないと思い始めてきた」「Need You とプロジェクトで動く Agent（主に CoS）へのアクセスが担保されていれば良い気がするね」「それこそが本当の Andon だしね」。

言い直し（人が同意 2026-11-10）。人が App の左の列（inbox）に求めるのは「自分が手を動かすべきものに迷わず着けること」で、それは2つに尽きる。1つは Needs you（人にしか答えられない問い・マージの依頼・Chief of Staff と Chief Engineer の返事）、もう1つはプロジェクトで動く役（主に Chief of Staff、題があるときの Chief Engineer）の端末へすぐ行けること。いまの inbox は herdr-flightdeck 由来の「エージェントを全部さばく受信箱」で、担当がターンを終えるたびに Done に積み上がる。けれど担当の Done と Working は dispatcher が拾って Chief of Staff が処理しており、人が見ても何も起きない。困りごとは「人が何とも思わない情報が、人の一番目につく場所に多く出ている（雑音）」こと。目的は、inbox を「人の Needs you」と「役への入口」の2つに絞り、それ以外を人の視界から下げること。見た目の好みの話ではなく、design.md §1.4 の「人の入口は Chief of Staff」の線と、灯りがつくときだけ人を呼ぶ Andon の線を画面に写す話。

事実（人に聞かず読んで確かめたもの）:
- Needs you は台帳の状態ではなく hc の live な状態（`blocked`）から来る（ledger.md）。担当の `blocked`（許可の確認）は、まず Chief of Staff が委譲の中で答える役で、人へ上がるのは Chief of Staff が中継したときだけ（chief-of-staff skill §4、ask-one-by-one §1）。
- 同梱の inbox は `news.replied` を読んでいない（contract.md §8 の表は読み手に「ボード」と書くが、`package/host/inbox/main.js` にその行は無い）。Chief of Staff・Chief Engineer の返事は「人が見るまで Needs you（Reply）、見たら Idle」（README）。
- ledger.md の「Every stopped agent has asked」は inbox が台帳も見る前提で書かれているが、同梱の inbox は台帳を読まない（読むのは kanban-board）。
- 2026-11-10 の実測（`hc ls`）: 担当 3 つが `done` のまま並び（A-0057・A-0029・A-0011。dispatcher が拾い済み）、人の Done に出ていた。

## 前提（同意済み。番号）

人が 2026-11-10 に 1〜5 に答えた（2 は人の言葉で直した）。

1. 人が inbox で手を動かすのは「人の Needs you」（Chief of Staff・Chief Engineer の返事と問い、Chief of Staff が中継した担当の止まり）と「役への入口」の2つだけ。担当の Done・Working・Idle は人の仕事ではない（dispatcher と Chief of Staff が処理する）。
2. Done は課題の片付けで勝手に消えるので、人の手間は増えない。困りごとは手間ではなく、人が何とも思わない情報が多く出ていること（雑音）。
3. 流れの俯瞰（誰が何をしているか・どの課題がどこか）は kanban-board と App の Spaces の一覧が既に持つ。inbox が Working・Idle・Settled の棚で俯瞰を兼ねる必要は無い。
4. 配り方は今のまま: `package/host/inbox` に同梱し `host-install` が人の App に置く。新しい約束（端末のラベル・schema・台帳の形）は足さず、今あるラベル（`role`・`project`・`issue`・`title`）と hc の状態だけで絞る。
5. 範囲は inbox mod だけ。dispatcher の通知のノイズ（A-0036）・kanban-board・名前を付ける andon mod・レイアウトの列の割りはこの題で触らない。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-10 inbox の節を2つに絞る: Needs you と、役への入口。Done の節と Working・Idle・Settled の棚を消す。**
   - **Needs you**: hc の状態が `blocked` の端末（担当・レビュアー・役のどれでも。sub-agent の `error` も今までどおり）と、人と話す役（`role=chief-of-staff`・`role=chief-engineer`）が終えたターン（Reply。人が見るまで）。
   - **役への入口**: プロジェクトで動く役の端末（Chief of Staff と、題があるときの Chief Engineer）。idle でも working でも常に出る。
   - 担当・レビュアーは `blocked` のときだけ出る。ターンを終えた（Herdr の `done`）・働いている・idle の担当は出さない（俯瞰は kanban-board と Spaces。前提 3）。
   理由: 人の言う2つがそのまま節になる。担当の `blocked` は安全側で残す（Chief of Staff が取りこぼしても・委譲の外の許可でも、人の画面に灯りがつく。ledger.md「Needs you は blocked から来る」の線を保つ）。
   捨てた案: A「Done の節だけ消す」（棚の雑音が残り、役への入口が担保されない）／C「役だけを出し担当を一切出さない」（Chief of Staff が止まったとき灯りがつかない）。

2. **2026-11-10 役への入口の節の見出しは「Projects」。** 行はプロジェクトごとに Chief of Staff → Chief Engineer（題ごと）の順。
   理由: 人の言葉「プロジェクトで動く Agent（主に CoS）へのアクセス」にいちばん近く、mod のほかの文字（Needs you）と同じ英語。「役」は人が「微妙」と言った。
   捨てた案: Staff（人と話す役の集まり）／Leads（ラインを率いる役）／見出しなしでプロジェクト名を小見出しにする（プロジェクトが1つのときに見出しが slug だけになる）。

3. **2026-11-10 役の行は1つだけで、Needs you のとき（Reply・blocked）は Needs you の節に上がり、見たら Projects に戻る。**
   理由: 灯りがついたものは全部いちばん上に（三色ランプの線）。役は少ないので入口が迷子にならない。今の mod の動き（節は行の状態で決まる）と同じで、作りも小さい。
   捨てた案: Projects に留まりバッジだけ変える（入口は動かないが、Needs you の節と役の節の2か所を見ることになる）。

4. **2026-11-10 dispatcher（`role=dispatcher`）は Projects に出さない。** 人と話す役ではない。止まりは Spaces の tab「dispatcher」（ADR-0002 決めたこと 21・22）と、端末が `error` になれば今までどおり Needs you で見える。
   捨てた案: 各プロジェクトの末尾かセルの行として出す（dispatcher は `project` ラベルを持たず、プロジェクトの行にならない）。

5. **2026-11-10 settle を消す（出典: 人）。** 行の ✓・キー `d`・`u`・`ctrl-shift-d`（`inbox.settle`）・ラベル `settled=` の読み書き・Settled の棚を全部消す。Needs you の行は、端末の状態が変わる（Chief of Staff か人が答える・役の返事を人が見る）まで出続ける。mod は hc に何も書かなくなる（読むだけ）。
   理由: 払う手が要るのは「人が見ても何もしない行」があるからで、絞ったあとの Needs you にそれは無い。
   捨てた案: 残して Settled の棚だけ消す（推奨案だった）／残して棚も残す。

6. **2026-11-10 ボードのダイアログ（ヘッダーのボタン・`ctrl-shift-u`・`mod.open:inbox`・`mod.json` の `overlay`・マシンごとのレーン）を消す。** 俯瞰は kanban-board（`ctrl-shift-y`。ボタンはヘッダーに残す）と Spaces（前提 3）。
   捨てた案: 2 列（Needs you・Projects）のダイアログとして残す。

7. **2026-11-10 仮（Chief Engineer の推奨。灯りの決断ではない。人が違うと言ったら直す）: ペインの無い役の端末（レイアウトが `andon` でなく、`herdr.tab` の tab に乗っていない Chief of Staff・Chief Engineer）が終えたターンは、人がその行を開いた（peek した）ときの `turn` を mod が覚え、同じ `turn` のあいだは Reply にしない。** 今の README は「ペインの無い役は `done` のままなので settle せよ」としているが、settle が無くなる（5）ので、見たことを mod が覚える。覚える場所は今ある `shared("inbox.memory")`（新しい約束は足さない）。
   捨てた案: Reply のまま出し続ける（ペインの無い役を使う人の Needs you が消えない）／ラベルを書く（mod は読むだけ、の 5 に反する）。

## やらないこと（範囲の外。理由）

- **担当・レビュアーの Done・Working・Idle を inbox に出すこと**（前提 1・3。俯瞰は kanban-board と Spaces）。
- **settle とラベル `settled=` の読み書き**（決めたこと 5）。mod は hc に何も書かない。`settled=` の行は contract.md §8 に無い（herdr-flightdeck 由来の mod だけの印だった）ので約束の文書は変えない。人が前に付けた `settled=` のラベルは読まず、端末と一緒に消える。
- **inbox が台帳を読むこと**（ledger.md「Every stopped agent has asked」の読み手に inbox を数えること）。前提 4。台帳を読む画面は kanban-board。ledger.md のその行の「(work-board, inbox)」は、実装の課題が「kanban-board」だけに直す（inbox を外す）。
- **dispatcher の通知のノイズ**（A-0036）・**kanban-board**・**名前を付ける andon mod**・**レイアウトの列の割り**（`layouts/andon.json` の inbox 2 : spaces 1）（前提 5）。inbox が短くなった分は、レイアウトの最小の高さ（`min: 120`）の範囲で Spaces が使う。割りを変えたくなったら別の題。
- **`news.replied` を inbox が読むこと**: 今も読んでおらず、役の返事は「人が見るまで Needs you」で足りる。contract.md §8 の読み手「ボード」は herdr-flightdeck の Web ボードを指す（変えない）。
- **ペインの無い担当（sub-agent）の畳み込みの変更**: 担当は blocked のときだけ出るので、親に畳む今の規則（urgent・peek）はそのまま。
- **人の `keys.json` を host-install が直すこと**: キーは人のもの（operate.md）。消えるキー（`ctrl-shift-d`・`ctrl-shift-u`）は README と operate.md の断片から外し、人が自分の `keys.json` から消す（残しても「unknown command」で害は無い）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 2 つ。新しい継ぎ目を作らない。

1. **`tests/host/screen.mjs`（偽の herdr API で mod を読み込む。既にある）**: 偽の `agents()`・`terminals()` を渡して `model()` の節を確かめる。
   - `role=chief-of-staff` の端末（idle）→ `projects` に 1 行。`working` でも `projects`。`done`（人が見ていない）→ `needs`（`kind: Reply`）。`blocked` → `needs`。
   - `role=chief-engineer issue=<id>` の端末 → `projects`（同じプロジェクトの Chief of Staff の次）。
   - `issue=<id>`（role 無し）の担当: `blocked` → `needs`、`done`・`working`・`idle` → どの節にも無い（`m.rows` に出ない）。
   - sub-agent の `error` は今までどおり親を `needs` にする（既存の規則が壊れていない）。
   - `role=dispatcher` の端末は `projects` に出ない。`error` なら `needs`。
   - `m` に `done`・`working`・`idle`・`settled` のキーが無い。`hc([...])` が一度も呼ばれない（偽の `hc` が呼ばれたら落とす）。
   - 2 プロジェクト（`project=` が 2 種）のとき `projects` はプロジェクトの名前順 → Chief of Staff → Chief Engineer の順。
   - 仮 7: ペインの無い役の `done` の行を `open` したあと、同じ `turn` のあいだは `needs` に出ない。`turn` が進めば出る。
   - `mod.json` に `overlay` が無い。`keys.json` に `inbox.settle`・`mod.open:inbox` が無く、`inbox.next`・`inbox.focus` はある。既存の「役ごとのアイコン」のテストは通ったまま。
2. **文書（`tests/public.sh`・`tests/package.sh`）**: `package/host/inbox/README.md`・`RECIPE.md` が新しい形を書き（Done・棚・settle・ボードの節が無い）、`docs/operate.md` の mod の表と keys の断片、`docs/ledger.md` の「(work-board, inbox)」が直っている。`tests/run --changed` green。
3. **セルで実測（P4 以降の決まり。報告に書く）**: `host-install` で置き直した App で、担当がターンを終えても inbox に出ず、Chief of Staff の返事が Needs you（Reply）に出て、見たら Projects に戻る。担当が許可で止まると Needs you に出て、Chief of Staff が答えると消える。

## 価値の仮説（P6 で形を決める）

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| I1 | この文書の PR（ADR-0015・`decisions/` の 1 ファイル） | `tests/run --changed` green。人がマージ | — |
| I2 | inbox の mod を絞る: 節を Needs you・Projects の 2 つに、Done・棚・settle・ボードのダイアログを消す。`tests/host/screen.mjs` に受け入れ条件 1、README・RECIPE・operate.md・ledger.md を受け入れ条件 2 のとおり。レベル A（package の mod。約束に触れない） | 受け入れ条件 1・2 | I1 |
| I3 | セルで実測（`host-install` で置き直す。operate.md の手順 8） | 受け入れ条件 3。落ちたら直しの課題 | I2 のマージ |

## 開いている問い（残った問い。決断の種類つき）

- 無し（2026-11-10 の対話で閉じた）。仮 7 は人が違うと言ったら「決めたこと」に下から足して直す。

## 出典（読んだもの・捨てた案の出所）

- 依頼書 A-0074（人の気付き 2026-11-10、Chief of Staff の経緯）。人の答え 2026-11-10（言い直し・前提 1〜5・決めたこと 1〜6。前提 2 と「役」の語は人の言葉で直した）。
- `package/host/inbox/`（main.js の節の定義 `RANK`・`SHELVES`・`BOARD_COLUMNS`・`TALKS_TO_YOU`・`model()`、README.md、RECIPE.md、mod.json、keys.json）、`package/host/layouts/andon.json`、`tests/host/screen.mjs`。
- [ADR-0002](0002-herdr-spaces.md)（前提 2「灯りの口はボード」、決めたこと 15・18・21・22）、[ADR-0009](0009-cos-default-entry.md)（人の入口は Chief of Staff）、[ADR-0010](0010-news-ack.md)（担当の通知の宛先は Chief of Staff）、[ledger.md](../ledger.md)（Needs you is not a status・Every stopped agent has asked）、[contract.md](../contract.md) §8（ラベルの表に `settled=` は無い）、[operate.md](../operate.md)（host-install・keys は人のもの・手順 8）、[decisions.md](../decisions.md) 2026-11-06（mod の取り込み）、chief-of-staff skill §4・ask-one-by-one §1（担当の許可は Chief of Staff が答える）、台帳 A-0036。
- 2026-11-10 の `hc ls`（担当 3 つが `done` のまま Done に出ていた）。
