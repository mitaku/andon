# ADR-0002: Herdr App の Space と tab への役の置き方（仕様・設計。2026-11-05。Chief Engineer のタスク Y-105）

- 状態: **決定（2026-11-05）**。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改め（試作の結果を含む）は「決めたこと」に下から足す。
- 主題: Chief of Staff・Chief Engineer・担当とレビュアーを、人の Herdr App（herdrdev/herdr-app）の Space と tab にどう置くか。
- 語は [design.md](../design.md) §0.1。機械が読む形は端末のラベル（[contract.md](../contract.md) §8）。

## 問題（人の言葉で。言い直しの段落）

Andon の 3 つの役（Chief of Staff・Chief Engineer・担当とレビュアー）が人の Herdr App の画面でどう見えるかを、ラインの構造に合わせて揃えたい。今は `hc run` で起きた担当・レビュアーが、起こした Chief of Staff の tab の下にペインの無い子として畳まれ、人の窓口である Chief of Staff・人と設計を話す Chief Engineer・黙って働く担当が 1 つの Space に混ざる。困りごとは「人が答えるべきものを見つけられない」ではなく、ラインの流れ（どのプロジェクトで誰が何人、今どこか）が Space と tab の構造に写っていないこと。TPS で言う大部屋（obeya）の見える化で、見た目の好みの話ではない（人が同意 2026-11-05）。

## 前提（同意済み。番号）

人が 2026-11-05 に 1〜7 すべてに同意。

1. 困りごとは上の言い直しのとおり「流れの構造が Space・tab に写っていない」こと。
2. 何もしなくても、人が答えるべきものはボード（inbox）で見つかる（`task=` のラベルで担当を独立の行にし、peek で開く）。この題で増えるのは俯瞰であって、灯りの口ではない。
3. 建屋の中から Space・tab を指す口は hc のラベル `herdr.space`・`herdr.tab` だけで、それで足りる。建屋の中の `herdr` CLI（別物の多重化ツール。人の App の Space は作れない）は使わず、`herdr-app` CLI（Windows 側の窓を動かす。建屋には無い）を建屋に入れない。新しい約束は contract.md §8 のラベル表に行を足すだけ。
4. ペインの分割は範囲の外。建屋から打てない（hc にペインの概念が無く、`herdr-app do pane.split` は窓の側）ので、Andon が指すのは Space と tab まで。
5. ラベルを書くのは Andon の command（`andon chief-of-staff`・`andon chief-engineer`・`andon task start`・クーリエ）だけ。Chief of Staff に `hc label` や `herdr-app` を直接打たせない。Space の作成・消滅はラベルと端末の寿命の副作用で起き、Chief of Staff は Space を「操作」しない。
6. Andon の command は App の画面を動かす操作（select・focus・peek・attach）を打たない。`hc attach` は人が自分で打ったときだけ。これが「フォーカスを奪わない」の決まりの実体。
7. Space は 1 マシンに属す（App の規則）。作業側を別マシンに出した担当はそのマシンの Space に出る。マシン横断の俯瞰は Space ではなくボードの仕事。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-05 Space はプロジェクト × 2 段。** `<slug>`（指揮側: Chief of Staff の tab と Chief Engineer の tab）と `<slug> work`（作業側: tab ＝ タスク）。
   理由: design.md §1.5 の指揮側／作業側の 2 段がそのまま写り、作業側を別マシンに出したとき `work` の Space がそのマシンへ移るだけで形が変わらない。人と話す 2 役が 1 つの Space に集まる。
   捨てた案: 役で 3 つ（CoS・Chief Engineer・作業用。プロジェクトが増えると CoS の Space に全プロジェクトの CoS が並ぶ）／プロジェクトごとに 1 つで tab に全部（tab が混む）。
2. **2026-11-05 作業側の tab はタスク 1 つ**（担当。レビュアーは同じタスクの tab）。上限は既にある並行の上限（`coordinator.md`）で効くので、新しい上限は置かない。
   捨てた案: tab ＝ 流れ（1 つの文書から切った束）。
3. **2026-11-05 片付けは、空になって消えるのに任せる。** `andon task finish` は端末を kill・rm するので tab が空になり、App の規則（最後の tab が閉じれば Space も閉じる）で消える見込み（要実測）。command は Space・tab を閉じない。
   捨てた案: `work` の Space を残す（次のタスクが同じ id に来る）。
4. **2026-11-05 人が起こす 2 役（`andon chief-of-staff`・`andon chief-engineer`）は、command が起こす・ラベルを付ける・人の端末に `hc attach` するまでを一度にする（オールインワン）。** 前提 6 の「attach は人が打ったときだけ」は、人が打った command の中の attach を含む（機械が起こすものは attach しない）。`--no-attach` は skill の自己確認用に残す。
   理由: 人の手順を減らす（人 2026-11-05）。要実測: ラベルの Space の pane と attach した pane に二重に出ないか。二重に出るなら、ラベルを人の端末から写す／attach をやめる、のどちらかを実測で選ぶ（開いている問い）。
   捨てた案: command はラベルを付けず、人が起こす 2 役は人の場所に従う（習慣に頼る）。
5. **2026-11-05 レビュアーは担当と同じ `herdr.tab` を付けて並べる**（クーリエが起こすとき）。1 つの tab でタスクの実装とレビューが見える。hc にペインの概念は無いが、同じ tab id の端末は App がその tab に並べる見込み（要実測）。
   捨てた案: ラベルを付けず担当の sub-agent にする（今の形。peek で覗く）。
6. **2026-11-05 id は command が決定的に作る。** `herdr.space=andon-<slug>`（指揮側）・`andon-<slug>-work`（作業側）、`herdr.tab=andon-<slug>-<タスク id>`（Chief of Staff は `andon-<slug>-cos`）。起こし直しても同じ場所に戻り、名前は人が App で一度付ければよい。
   捨てた案: 起こすたびに乱数の id（App の形に寄せる。戻る場所が無い）。
7. **2026-11-05 実装の前に試作で確かめ、その結果で 1 の指揮側の割り方を決め直す。** 人の案: 同じ tab id の端末が App で並ぶ（事実上の split）なら、**マシンに 1 つの Space に全プロジェクトの Chief of Staff を並べる**形もある。試作（依頼書に切る案の S0）で「新しい id の Space を App が前に出すか・同じ tab id の 2 端末が並ぶか・空になった tab と Space が消えるか・attach との二重表示」を人が App で見て答え、結果をこの節に下から足す。1 の指揮側（`andon-<slug>`）はそれまで仮。作業側（`andon-<slug>-work`。tab ＝ タスク）は変えない。
   理由: 要実測が 4 つあり、どれも人の App を見ないと確かめられない。試してから決める（人 2026-11-05）。
8. **2026-11-05 Chief Engineer の tab は担当と同じ規則** `herdr.tab=andon-<slug>-<タスク id>`（指揮側の Space の中）。題が終われば tab が空になって消える。
   捨てた案: 固定の `andon-<slug>-ce`（題ごとの区切りが見えない）。
9. **2026-11-05 仮設の建屋の `yard task` には入れない。** 実装は Andon の command だけ（`andon task start`・`andon chief-of-staff`・`andon chief-engineer`・クーリエ）。試作（7）は仮設の建屋で打てるので、見える化の確認はそれで足りる。
   理由: 仮設には「運用を止めない直し」だけ入れる（design.md §5.3）。

## やらないこと（範囲の外。理由）

- **ペインの分割を建屋から指すこと**（前提 4）。並べたいものは同じ `herdr.tab` にする（決めたこと 5）。
- **建屋に `herdr-app` を入れる・建屋の `herdr` CLI で Space を作ること**（前提 3）。
- **Chief of Staff が Space・tab を操作すること**（作る・閉じる・名前を付ける・前に出す。前提 5・6）。名前は人が App で付ける。
- **ラベル `where` を書くこと**: 書き手は App の見込み。Andon は読まず書かない。
- **クーリエと通知の係自身を App に出すこと**: `--no-parent` のまま（人が見るものではない）。
- **ラベルの on/off の設定**: App の無い建屋（クラウド常駐）でもラベルは無害なので、常に付ける。
- **ボード（herdr-flightdeck）の変更**: inbox は `task=` で担当を独立の行にしており、Space で絞らないのは向こうの方針（prior-art.md「Space filter: Leave」）。Space の見える化は App の標準の画面で得る。
- **ラベルの値を schema に入れること**: 端末のラベルは contract.md §8 の表だけで持つ（今までどおり）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 4 つ。既存のテストに足す（新しい継ぎ目を作らない）。

1. **試作（S0。人が App で見る）**: `hc run --bg --no-parent --label herdr.space=andon-try --label herdr.tab=andon-try-1 -- sh` の形で偽の端末を起こし、人が答える 4 項目を報告に書く: (a) 知らない id で App に新しい Space が出るか・そのとき前に出る（人の画面が移る）か、(b) 同じ `herdr.tab` で起こした 2 つ目の端末がその tab に並ぶ（split）か・向き、(c) 端末を終えると tab と Space が消えるか、(d) ラベルを付けた端末に `hc attach` したとき二重に出るか。結果は決めたこと 7 の下に追記する。
2. **`andon chief-of-staff`・`andon chief-engineer`（偽の hc。`internal/chiefengineer` のテストと、F2 で建てる chief-of-staff のテスト）**: `hc run` の引数に `--label herdr.space=andon-<slug>` と `--label herdr.tab=andon-<slug>-<タスク id>`（Chief of Staff は `andon-<slug>-cos`）がある。起こしたあと `hc attach <address>` を打つ。`--no-attach` では打たない。
3. **`andon task start`（偽の hc。`tests/andon-task.sh`）とクーリエのレビュアー起動（`internal/courier` のテスト）**: `andon task start` は `--label herdr.space=andon-<slug>-work --label herdr.tab=andon-<slug>-<id>` を付け、`--on <machine>` でも同じ。クーリエは親タスクの担当の端末（`task.yaml` の `agent`）の `herdr.space`・`herdr.tab` を `hc get --json` で読んで同じ値を付け、読めなければ付けない。`andon task finish` はラベルに触らない（打つ hc は kill・rm だけ）。
4. **約束の文書（`tests/public.sh`・`tests/schema.sh`）**: contract.md §8 のラベル表に `herdr.space=<id>`・`herdr.tab=<id>`（持ち主 Herdr App・書き手 command とクーリエ・読み手 App）の行がある。schema は変えない。
5. **建屋で実測（P4 以降の決まり。報告に書く）**: 担当を 1 つ起こすと App の `andon-<slug>-work` に tab が出て、レビュアーが同じ tab に並び、finish で消える。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（design.md §1.5 と contract.md §8・decisions.md から指す） | `bash tests/*.sh` green | — |
| S0 | 試作: 偽の端末でラベルの振る舞いを人が見る（Chief of Staff が人の go を取って打つ。人が閉じる。`review: off`） | 受け入れ条件 1 の 4 項目が報告にあり、この文書に追記の PR | S1 のマージ |
| S2 | `andon chief-engineer`（と F2 の `andon chief-of-staff`）にラベルと attach | 受け入れ条件 2 | S0（二重表示の答え）・S1 |
| S3 | `andon task start` にラベル・クーリエのレビュアーに同じ tab・`--on` でも同じ | 受け入れ条件 3 | S0・S1。`andon task start` を建てるタスクに添える |
| S4 | 建屋で実測 | 受け入れ条件 5。落ちたら直しのタスク | S2・S3・建屋（P7） |

## 開いている問い（残った問い。判断の種類つき）

- **指揮側の割り方**（ラインの変更。試作 S0 の (b) の結果で人が決める）: プロジェクトごとの `andon-<slug>` か、マシンに 1 つの Space に全プロジェクトの Chief of Staff を並べるか。
- **attach との二重表示が出たときの選び方**（ラインの変更。S0 の (d) の結果で）: ラベルを人の端末から写す／command の attach をやめる。
- **Space・tab の名前を command から付けられるか**（事実。App の API 次第。今は人が付ける）。
- **別マシンで起こした担当が App に出るか**（事実。Herdr が起こしていない端末は出ない見込み。S4 で見る）。
- **`where` ラベルの書き手**（事実。App の見込み）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3〜§1.8・§5、[courier.md](../courier.md)、[contract.md](../contract.md) §8・§9、[operate.md](../operate.md)、`internal/chiefengineer`・`internal/task`・`internal/courier`・`internal/boot`、`package/skills/chief-of-staff`。
- Herdr App: https://github.com/herdrdev/herdr-app （README・`defaults/AGENTS.md`・`docs/commands.md`。2026-11-05 に読んだ）。要点: Space は 1 マシンの仕事の置き場。Herdr が起こす端末にはラベル `herdr.space=<id>`・`herdr.tab=<id>` が付き、そのラベルを持つ端末はそこに出る。知らない id は新しい Space か tab になる。`hc run` で起きた sub-agent は親の tab にペインの無いまま属し `terminal.peek` で覗く。`herdr-app` は窓を、`hc` は herdr-core を動かす。
- herdr-core: `hc --help`・`hc --skill`（窓は並べない。`herdr-app --skill` を使え）・`hc run --help`・`hc get --json`（2026-11-05 の実機: Chief of Staff の端末に `herdr.space`・`herdr.tab`・`where`、担当に `where` だけ）。
- ボード（herdr-flightdeck。private）: `docs/concept.md`・`docs/mod-design.md`（`task=` の担当を独立の行にする決定 2026-11-03・未確認の項 §9）・`herdr-app/scripts/fake-agent.mjs`（`hc run --label herdr.space=… --label herdr.tab=…` で新しい Space に出す例）。
