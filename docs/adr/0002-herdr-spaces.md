# ADR-0002: Herdr App の Space と tab への役の置き方（仕様・設計。2026-11-05。Chief Engineer のタスク Y-105）

- 状態: **決定（2026-11-05）。決めたこと 1・2（作業側の Space と tab。それを確定した 12・作業側の名前の 16 を含む）は覆された → [ADR-0003](0003-task-spaces.md)**（2026-11-06）。同日、試作 S0 の結果で前提 8〜11 と決めたこと 10〜18 を足した（Chief Engineer のタスク Y-106。構造は変えていないので同じ ADR）。2026-11-06 に伝令の置き場を足し、「やらないこと」の1行を覆した（決めたこと 21。Y-140）。同日、その係の名前を dispatcher に改めた（決めたこと 22。Y-123）。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改め（試作の結果を含む）は「決めたこと」に下から足す。
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

S0 の結果を受けて 2026-11-05 に足した前提（Y-106。人が同意）。3 と 5 は 9 のとおり改める。

8. 建屋は 1 コンテナ（App からは 1 machine）に N プロジェクト。プロジェクトごとに `projects/<slug>/` の台帳と Chief of Staff がいる。この文書の `<slug>` はプロジェクトの slug。
9. **置き場（Space・tab の作成・配置・消滅）は Andon の command がラベルで擬似的に操作し、名前だけは建屋の外（人の App に入れた Andon の mod か、人）が付ける。** 建屋から名前を付ける口は無い（App は `herdr.space`・`herdr.tab` 以外のラベルを読まない。`where` は App の出力で入力ではない）。
10. 人が端末を見つける手がかりは hc の address ではなく、Space 名・tab 名・端末の題の 3 つ。端末の題はプログラム（Claude Code）が会話から自分で付けるもので、Andon の command は決められない。
11. ラベルで採用された pane と `hc attach` の pane は別物で、両方あれば二重に出る（App の仕様）。どちらかを捨てる。

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

   **S0 の結果（2026-11-05。Chief of Staff が人の go で打ち、人が App で見た。読み解きは Y-106 で Herdr App の公開ソースと照らした）。** 打ったのは `hc run --bg --no-parent --no-notify --label herdr.space=andon-try --label herdr.tab=andon-try-1 -- sh` を 2 つ。
   - (a) 人の見え方: 「新しい Space は出ない。2 つは『workspace』という名前の Space に出た」。**読み解き: Space は id `andon-try` で新しく作られていた。** App は知らない id の Space と tab をその id で作り、名前は付けない（`src/window.rs` の `adopt`）。名無しの Space の名前は「最初の端末の cwd で `git rev-parse --show-toplevel` した結果のフォルダ名」（repo でなければ cwd のフォルダ名。`src/model.rs` の `space_name`）で、試作の端末の cwd が建屋 repo の中だったので建屋 repo のフォルダ名が出た。名無しの tab は番号。画面は移らない（App は表示中の Space を変えない）。つまり崩れたのは「id が名前として見える」「人が一度付けた名前が残る」の 2 つで、「ラベルで Space を作れる」は崩れていない。
   - (b) 同じ `herdr.tab` の 2 つは同じ tab に**横に並んで** split された（App はその tab の最後の pane の横に足す）。
   - (c) 2 つを kill・rm すると tab も Space も消えた（最後の tab が閉じると Space も閉じる。残るのは名前とフォルダだけで id は残らず、同じ id で起こし直すと名無しの新しい Space になる）。
   - (d) `hc attach` すると二重に出た（前提 11）。
   - おまけ: `where` ラベルは App が「<Space 名> / <tab 名>」を書く出力で、子の端末は親の値を継ぐ。Andon は読まず書かない（開いている問いの答え）。
8. **2026-11-05 Chief Engineer の tab は担当と同じ規則** `herdr.tab=andon-<slug>-<タスク id>`（指揮側の Space の中）。題が終われば tab が空になって消える。
   捨てた案: 固定の `andon-<slug>-ce`（題ごとの区切りが見えない）。
9. **2026-11-05 仮設の建屋の `yard task` には入れない。** 実装は Andon の command だけ（`andon task start`・`andon chief-of-staff`・`andon chief-engineer`・クーリエ）。試作（7）は仮設の建屋で打てるので、見える化の確認はそれで足りる。
   理由: 仮設には「運用を止めない直し」だけ入れる（design.md §5.3）。

S0 の結果で決め直したこと（2026-11-05。Y-106。人が番号で答えた）。

10. **2026-11-05 指揮側はプロジェクトごと。1 の 2 段（`<slug>` と `<slug> work`）を仮から確定にする。** 作業側を別マシンに出しても形が変わらない。
    捨てた案: machine に 1 つの指揮側 Space に全プロジェクトの Chief of Staff と Chief Engineer を並べる（前提 8 で構造上は取れるが、人が答える相手がプロジェクトごとに分かれている方が静か）／プロジェクトの Space 1 つに Chief of Staff・Chief Engineer・タスクの tab を全部並べる（作業側の流れが指揮側に混ざる）。
11. **2026-11-05 Chief Engineer は worktree で起きたまま、指揮側の Space に出す。** 文書の PR に branch が要る。置き場は id で決まるので cwd は関係ない。
    捨てた案: プロジェクトの root で起こす。
12. **2026-11-05 作業側の tab ＝ タスク 1 つ（2 を確定）。レビュアーは担当と同じ tab に横に並ぶ（5 を確定。S0 の (b)）。**
    捨てた案: `<slug> work` に tab 1 つで全タスクを split（pane が積まれて読めず、どのタスクが blocked かが tab の状態で分からない）。
13. **2026-11-05 id は command が決定的に作る（6 を確定）。** 起こし直しても同じ場所に戻り、打つ場所に依らない。人が手で作っていた Space は移行で閉じる。
    捨てた案: `andon chief-of-staff` を打った端末の `herdr.space` を写す（打つ場所を間違えると別の Space に出る）。
14. **2026-11-05 command の attach をやめる（4 を改める）。** 人が起こす 2 役もラベルだけで App に出し、command は最後に出た場所を「<Space 名> / <tab 名>」で出す。`--no-attach` は要らなくなるので消す。最初のメッセージが届かないとき（信頼の画面など）は人が App のその tab で答える。前提 6 は「Andon は attach も打たない」に戻る。
    捨てた案: ラベルをやめ attach だけ（Space の見える化を捨てることになる）。
15. **2026-11-05 名前は Andon 同梱の App の mod が付ける（前提 9）。** Andon が `package/host/` に mod を同梱し、`host-install` が人の App の mods（`~/.config/herdr-app/mods/`）に置く。mod は App の中で端末の一覧を読み、command が既に付けているラベル `project`・`role`・`task`・`title` から `space.rename`・`tab.rename` を打つ。**名前だけ。** 置き場・画面・Space の作成と消滅には触れない（design.md §4 の「Herdr App は触らない」をこの 1 点だけ緩める）。
    並行して、採用時にラベル `herdr.space.name`・`herdr.tab.name` を名前にする機能を Herdr App に issue で頼む（labels.md が `herdr.` 接頭の追加を受け付けている）。入れば mod は要らなくなる。
    捨てた案: 人が App で付ける（タスクの tab は毎回付けることになり、空になった Space の名前は消える。規則が人任せ）／cwd のフォルダ名で既定名を狙う（tab は番号のままで、作業側の Space 名は担当の入れ替わりで変わる）。
16. **2026-11-05 名前の規則。** Space は `<slug>`（指揮側）と `<slug> work`（作業側）。tab は Chief of Staff が `CoS`、Chief Engineer が `CE <id> <題>`、担当が `<id> <題>`（レビュアーは同じ tab なので付けない）。題は台帳の `title`（ラベル `title=` に既にある）。
17. **2026-11-05 Chief of Staff が人に端末を指す言い方は「<Space 名> / <tab 名>」**（App の `where` と同じ形。例 `sample-site / CoS`・`sample-site work / A-12 ログインの直し`）。address は末尾に添えるだけ。command の出力も同じ形。
18. **2026-11-05 Chief of Staff と Chief Engineer のアイコンを役ごとに変える（人の希望）。** App はプログラム名（状態報告の `app`、無ければ argv の先頭）で `programs.json` の項目を引き、アイコンはそこで決まる。手段は実装で確かめる: 役ごとの wrapper 名（`andon-cos` など）＋ `host-install` が置く `programs.json` の項目が第一候補、効かなければ mod で描くか upstream に頼む。
19. **2026-11-05 受け入れ条件 3 の「`--on <machine>` でも同じ」は、作業側を別のマシンに出すタスクへ回す（Y-108 のレビュー Y-111 の指摘。Chief of Staff が決めた）。** `andon task start` は今までどおり `machine` 付きのタスクを起こさずに止まる（[decisions.md](../decisions.md) の 2026-11-05「machine があるタスクを起こさずに止まる」）。ラベルの値は slug・id・題だけで決まり machine に依らない（`internal/task` の `workLabels`）ので、そのタスクは `hc run --on` を足すだけでよい。
    理由: `--on` を渡すだけでは、指揮側に作った worktree（repo の無いタスクは `tasks/<id>/`）が別のマシンに無く、cwd が成り立たない。別のマシンの worktree の作り方は design.md §1.5 の「その先」で決める。
    捨てた案: この題で `hc run --on` を通す（作業側の置き場の設計を先取りする）。

20. **2026-11-05 mod の置き場は App の設定フォルダに従う（15 の補足）。** `HERDR_APP_HOME` → `XDG_CONFIG_HOME/herdr-app` → `~/.config/herdr-app` の順で選び、その `mods/andon/` に置く。WSL では Windows 側との取り違えを避けるため `HERDR_APP_HOME` の明示指定を求め、未指定ならホストの道具を書き込む前に止める。
21. **2026-11-06 伝令（courier）を App に出す。置き場は建屋の台帳の数で決める（出典: 人）。** 伝令は建屋に1つで台帳をまたぐ。台帳が1つなら、その指揮側の Space `andon-<slug>`（名前は `<slug>`）に tab `andon-<slug>-courier`（名前は「伝令」）。2つ以上（無いときも）なら、建屋の名前の Space `andon-<建屋の slug>-plant`（名前は `<建屋の slug> plant`）に tab `andon-<建屋の slug>-courier`（名前は「伝令」）。建屋の slug は hostname を `a-z 0-9 -` に揃えたもの。ラベルを付けるのはクーリエを起こす `courier.Ensure`（boot と Chief of Staff のフックが打つ。起こし直しも同じ）、名前を付けるのは Andon の mod（伝令は `project` ラベルを持たないので、mod は自分の tab id から名前を導く）。日本語の呼び名は「伝令」（Y-123）、英語の識別子は courier のまま。
    理由: P7 で伝令が止まっていたのに誰も気づけなかった（人の意向 2026-11-06）。App で見えれば止まりに気づける。
    覆したこと: 「やらないこと」の「クーリエと通知の係自身を App に出すこと」（当時の理由は「人が見るものではない」）。通知の係は今回も出さない（`--no-parent` のまま）。
    捨てた案: 台帳が増えても指揮側のどれか1つの Space に出す（どの Space かが台帳の並び順で決まり、足すたびに動く）。`project` ラベルを伝令に付ける（ラベルの読み手のボードとフックが、伝令を台帳の担当と取り違えうる）。
22. **2026-11-06 21 の係の名前を dispatcher に改める（出典: 人）。** 21 の名前の行を次に替える: tab の id は `andon-<slug>-dispatcher`・`andon-<建屋の slug>-dispatcher`、名前は「dispatcher」。端末の名前とラベルは `dispatcher`・`role=dispatcher`、ラベルを付けるのは `dispatcher.Ensure`。日本語の文でも「dispatcher」と書く（カタカナにしない）。置き場（台帳の数で決める）と mod が tab id から名前を導く形は 21 のまま。前の名前の端末は建屋の boot が片付ける（名前もラベルも前の名前の端末で、前に出ているものが前の係かシェルのときだけ。operate.md「前の名前の係から移す」）。
    理由: 21 の日本語の呼び名も英語の識別子も、ソフトウェア開発の言葉として聞き馴染みが無い。仕事（台帳と端末を見回る・起きたことを Chief of Staff に知らせる・決まった次の工程を起こす。判断はしない）に一番近い、聞き馴染みのある語にした。
    覆したこと: 21 の「日本語の呼び名」と「英語の識別子はそのまま」。

## やらないこと（範囲の外。理由）

- **ペインの分割を建屋から指すこと**（前提 4）。並べたいものは同じ `herdr.tab` にする（決めたこと 5）。
- **建屋に `herdr-app` を入れる・建屋の `herdr` CLI で Space を作ること**（前提 3）。
- **Chief of Staff が Space・tab を操作すること**（作る・閉じる・名前を付ける・前に出す。前提 5・6）。名前は Andon の mod が人の App の中で付ける（決めたこと 15）。
- **ラベル `where` を書くこと**: 書き手は App（S0 で確認）。Andon は読まず書かない。
- **mod が名前以外に触ること**（Space や tab を作る・閉じる・選ぶ・pane を動かす）。mod の仕事は `space.rename`・`tab.rename` だけ。
- **建屋から名前を付けようとすること**（`where` や独自のラベルで）: App は読まない（前提 9）。
- **通知の係自身を App に出すこと**: `--no-parent` のまま（人が見るものではない）。クーリエは 2026-11-06 に出すことに覆した（決めたこと 21。当時は「クーリエと通知の係自身」を出さないとしていた）。
- **ラベルの on/off の設定**: App の無い建屋（クラウド常駐）でもラベルは無害なので、常に付ける。
- **ボード（herdr-flightdeck）の変更**: inbox は `task=` で担当を独立の行にしており、Space で絞らないのは向こうの方針（prior-art.md「Space filter: Leave」）。Space の見える化は App の標準の画面で得る。
- **ラベルの値を schema に入れること**: 端末のラベルは contract.md §8 の表だけで持つ（今までどおり）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 4 つ。既存のテストに足す（新しい継ぎ目を作らない）。

1. **試作（S0。人が App で見る）**: `hc run --bg --no-parent --label herdr.space=andon-try --label herdr.tab=andon-try-1 -- sh` の形で偽の端末を起こし、人が答える 4 項目を報告に書く: (a) 知らない id で App に新しい Space が出るか・そのとき前に出る（人の画面が移る）か、(b) 同じ `herdr.tab` で起こした 2 つ目の端末がその tab に並ぶ（split）か・向き、(c) 端末を終えると tab と Space が消えるか、(d) ラベルを付けた端末に `hc attach` したとき二重に出るか。結果は決めたこと 7 の下に追記する。
2. **`andon chief-of-staff`・`andon chief-engineer`（偽の hc。`internal/chiefengineer`・`internal/chiefofstaff` のテスト）**: `hc run` の引数に `--label herdr.space=andon-<slug>` と `--label herdr.tab=andon-<slug>-<タスク id>`（Chief of Staff は `andon-<slug>-cos`）がある。**`hc attach` は打たない**（決めたこと 14。`--no-attach` は無い）。最後に標準出力に「<Space 名> / <tab 名>」の形で出た場所を出す。
3. **`andon task start`（偽の hc。`tests/andon-task.sh`）とクーリエのレビュアー起動（`internal/courier` のテスト）**: `andon task start` は `--label herdr.space=andon-<slug>-work --label herdr.tab=andon-<slug>-<id>` を付け、`--on <machine>` でも同じ（`--on` は作業側を別のマシンに出すタスクへ回した。決めたこと 19）。クーリエは親タスクの担当の端末（`task.yaml` の `agent`）の `herdr.space`・`herdr.tab` を `hc get --json` で読んで同じ値を付け、読めなければ付けない。`andon task finish` はラベルに触らない（打つ hc は kill・rm だけ）。
4. **約束の文書（`tests/public.sh`・`tests/schema.sh`）**: contract.md §8 のラベル表に `herdr.space=<id>`・`herdr.tab=<id>`（持ち主 Herdr App・書き手 command とクーリエ・読み手 App）の行がある。schema は変えない。
5. **mod（`package/host/` の中。偽の App の API でテスト）**: ラベル `project=<slug> role=chief-of-staff` の端末がある Space に `<slug>`、その tab に `CoS` を付ける。`role=chief-engineer task=<id> title=<題>` の tab に `CE <id> <題>`、`task=<id>`（role 無し）の tab に `<id> <題>`、`herdr.space=andon-<slug>-work` の Space に `<slug> work`。既に同じ名前なら打たない。`space.rename`・`tab.rename` 以外の command を打たない。`host-install` が mod を `~/.config/herdr-app/mods/andon/` に置く（既にあれば触らない）。
6. **建屋で実測（P4 以降の決まり。報告に書く）**: `andon chief-of-staff` で App に `<slug> / CoS` が出て二重に出ない。担当を 1 つ起こすと `<slug> work / <id> <題>` が出て、レビュアーが同じ tab に横に並び、finish で消える。Chief of Staff と Chief Engineer のアイコンが役ごとに違う（決めたこと 18。効かなければ直しのタスク）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（design.md §1.5 と contract.md §8・decisions.md から指す） | `bash tests/*.sh` green | — |
| S0 | 試作: 偽の端末でラベルの振る舞いを人が見る（済 2026-11-05。結果は決めたこと 7 の下） | 受け入れ条件 1 の 4 項目が報告にあり、この文書に追記の PR | S1 のマージ |
| S2 | `andon chief-engineer`・`andon chief-of-staff` にラベル。attach と `--no-attach` を消し、出た場所を出す | 受け入れ条件 2 | S1 |
| S3 | `andon task start` にラベル・クーリエのレビュアーに同じ tab・`--on` でも同じ | 受け入れ条件 3 | S1。`andon task start` を建てるタスクに添える |
| S5 | 名前を付ける mod（`package/host/`）と `host-install`。役ごとのアイコン（wrapper 名 ＋ `programs.json`）も同じタスクで試す | 受け入れ条件 5 | S1。S2・S3 とは独立 |
| S6 | Herdr App に issue: 採用時に `herdr.space.name`・`herdr.tab.name` を名前にする（人が出す。文面を Chief of Staff が用意） | issue の URL が報告にある | — |
| S7 | Chief of Staff の skill と contract.md §8 に「人に端末を指す言い方」（決めたこと 17）を足す | `tests/public.sh`・`tests/schema.sh` green | S1 |
| S4 | 建屋で実測。移行で、人が手で作っていた Space を閉じる | 受け入れ条件 6。落ちたら直しのタスク | S2・S3・S5・建屋（P7） |

## 開いている問い（残った問い。判断の種類つき）

2026-11-05（Y-106）に閉じた問い: 指揮側の割り方（決めたこと 10）・attach との二重表示（14）・名前を command から付けられるか（付けられない。mod が付ける。15）・`where` の書き手（App。前提 9）。

- **別マシンで起こした担当が App に出るか**（事実。Herdr が起こしていない端末は出ない見込み。S4 で見る）。
- **mod から端末のラベルが読めるか**（事実。App 自身は core の記録のラベルを読んでいるので読める見込み。S5 で実測。読めなければ `herdr.`〜のラベルを頼む S6 を待つ）。
- **役ごとのアイコンの手段**（2026-11-09 に閉じた問い: mod で描く（inbox。CoS=`clipboard-list`・CE=`drafting-compass`）。App 自身が描くサイドバーには効かない）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3〜§1.8・§5、[dispatcher.md](../dispatcher.md)、[contract.md](../contract.md) §8・§9、[operate.md](../operate.md)、`internal/chiefengineer`・`internal/task`・`internal/dispatcher`・`internal/boot`、`package/skills/chief-of-staff`。
- Herdr App: https://github.com/herdrdev/herdr-app （README・`defaults/AGENTS.md`・`docs/commands.md`・`docs/labels.md`。S0 の読み解きに `src/window.rs` の `adopt`・`reconcile`・`write_where`・`folder_info`、`src/model.rs` の `space_name`・`tab_name`、`src/programs.rs`、`defaults/mods/agents/main.js`。2026-11-05 に読んだ）。要点: Space は 1 マシンの仕事の置き場。Herdr が起こす端末にはラベル `herdr.space=<id>`・`herdr.tab=<id>` が付き、そのラベルを持つ端末はそこに出る。知らない id は新しい Space か tab になる。`hc run` で起きた sub-agent は親の tab にペインの無いまま属し `terminal.peek` で覗く。`herdr-app` は窓を、`hc` は herdr-core を動かす。
- herdr-core: `hc --help`・`hc --skill`（窓は並べない。`herdr-app --skill` を使え）・`hc run --help`・`hc get --json`（2026-11-05 の実機: Chief of Staff の端末に `herdr.space`・`herdr.tab`・`where`、担当に `where` だけ）。
- ボード（herdr-flightdeck。private）: `docs/concept.md`・`docs/mod-design.md`（`task=` の担当を独立の行にする決定 2026-11-03・未確認の項 §9）・`herdr-app/scripts/fake-agent.mjs`（`hc run --label herdr.space=… --label herdr.tab=…` で新しい Space に出す例）。
