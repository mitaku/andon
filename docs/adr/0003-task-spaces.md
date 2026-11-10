# ADR-0003: 作業側の Space をタスクごとにする（2026-11-06。Y-147）

- 状態: **決定（2026-11-06）**。[ADR-0002](0002-herdr-spaces.md) の決めたこと 1（2 段）の作業側と 2（作業側の tab はタスク 1 つ。12 で確定）を覆す。指揮側（`<slug>` の Space・CoS と CE の tab）は ADR-0002 のまま。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。
- 主題: 担当とレビュアーを人の Herdr App のどの Space・tab に置くか。
- 語は [design.md](../design.md) §0.1。機械が読む形は端末のラベル（[contract.md](../contract.md) §8）。

## 問題

ADR-0002 では作業側を `<slug> work` の Space 1 つにまとめ、tab をタスクごとにした。App の Space は 1 つの作業の置き場で、Files・Changes の欄は Space のフォルダ（最初の端末の cwd の git の根）を指す。作業側の Space が 1 つだと、そのフォルダは最初に起動した担当の worktree になり、ほかのタスクの worktree は App から見えない。人はタスク（＝worktree）ごとに Space があるほうがよいと言った（人 2026-11-06。P7 のあと）。

## 決めたこと

1. **2026-11-06 作業側は Space ＝ タスク。** id は `herdr.space=andon-<slug>-<タスク id>`、名前は `<id> <題>`（ADR-0002 決めたこと 16 の担当の tab 名をそのまま Space 名にする）。題は台帳の `title`（ラベル `title=`）。
   理由: Space のフォルダが担当の worktree になり、App の Files・Changes がそのタスクを指す。片付けは ADR-0002 決めたこと 3 と S0 の (c) のまま（`andon task finish` が端末を片付けると最後の tab が閉じ、Space も閉じる）。並ぶ Space の数は並行の上限（`coordinator.md`）で抑えられるので、新しい上限は置かない。
   捨てた案: 今の `<slug> work` の Space 1 つに tab ＝ タスク（Files・Changes が最初の担当の worktree しか指さない）。
2. **2026-11-06 tab は担当の 1 つ。** id は `herdr.tab=andon-<slug>-<タスク id>`（ADR-0002 決めたこと 6 と同じ値）、名前は `<id>`。レビュアーは今までどおり担当と同じ Space・tab に横に並ぶ（クーリエが担当の端末の `herdr.space`・`herdr.tab` を写す。写し方は変えない）。
   理由: tab の id を変えなければクーリエの `workerPlace` は値を写すだけで済む。Space 名に題があるので、tab 名は短い `<id>` にする。
   捨てた案: レビューごとに tab を分ける（クーリエの写し方が変わり、1 つの画面で実装とレビューが並ばなくなる）／tab 名も `<id> <題>`（「<Space 名> / <tab 名>」で題が 2 回出る）。
3. **2026-11-06 人に端末を指す言い方**（ADR-0002 決めたこと 17）は、担当で `<id> <題> / <id>`（例 `A-12 ログインの直し / A-12`）。`andon task start` の `出た場所` の行も同じ形。
4. **2026-11-06 別マシンに出す担当（ADR-0002 決めたこと 19）は変わらない。** ラベルの値は slug・id・題だけで決まり machine に依らないので、Space ＝ タスクでもそのマシンの Space に出るだけ。

## 出典

- 人（2026-11-06。P7 のあと）: 作業側は worktree（＝タスク）ごとに Space があるほうがよい。
- ADR-0002 の S0 の結果（(b) 同じ tab の端末は横に並ぶ・(c) 最後の tab が閉じると Space も閉じる）と、App の Space のフォルダの決め方（`src/model.rs` の `space_name`。ADR-0002 の出典）。
