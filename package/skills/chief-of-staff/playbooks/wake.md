---
読む時: 起動したとき（1回）と、人と話すターンの最初
消す条件: 予約キーの書き直しの段は「`andon chief-of-staff` がアドレスの変更時に、開いた課題の予約キーを同じ値で書き直すようになったら」。打つコマンドと場所の名前の段は「素の版（原則と目次だけ）で同じ場面を流して、結果が変わらなかったら」
---

# 起動したとき・毎ターンの最初

## 打つコマンド

- 打つコマンドは `kanban`（台帳）・`hc`（端末とエージェント）・`andon issue`（起動する・片付ける）・`andon chief-of-staff`（自分を台帳に載せる）・`andon chief-engineer`（Chief Engineer を起動する。`cut.md` の「人の入口」）・`gh`・`git`（セルの wiki だけ。`wiki.md`）。`hc attach` は打たない（人の画面に乗るのは人が自分で打つときだけ）。

## 端末の場所の名前

- **人に端末を指すときは「<Space 名> / <tab 名>」で言う**（人の Herdr App の画面の場所。App が端末の `where` に書く形と同じ）。人は App で hc の address が見えない。
  address は末尾に添えるだけ（`S-0003 返事の下書きボタン / S-0003（t7）`）。名前の決まり: Space は `<slug>`（自分と Chief Engineer）と課題ごとの `<id> <題>`（担当とレビュアー）、
  tab は自分が `CoS`・Chief Engineer が `CE <id> <題>`・担当が `<id>`（レビュアーは担当と同じ tab に並ぶ）。`andon chief-of-staff`・`andon chief-engineer`・`andon issue start` は最後に `出た場所 …` の行で同じ形を出すので、それを写す。
  置き場はコマンドがラベル `herdr.space`・`herdr.tab` で決める（`contract.md` §8）。自分では `hc label` も `herdr-app` も打たない（Space・tab を作る・閉じる・名前を付ける・前に出す、をしない）。

## 起動したら1回

1. `andon chief-of-staff` を打つ。自分が見つかり、`config.yaml` の `chief_of_staff` が自分のアドレスに直る（起動し直すとアドレスが変わる）。
   Chief of Staff は1プロジェクトに1人（規約1）: 形は `config.yaml` の `chief_of_staff` と、ラベル `role=chief-of-staff project=<slug>`。起動するのは人が打つ `andon chief-of-staff`（動いていれば使う・2人なら止まる・そのまま画面に乗る）。
   「2人いる」で止まったら、自分以外の1人を人に知らせる（自分で止めない）。
   **アドレスが変わったら、開いている課題（`done` と、レビュアーの課題 `-rN` を除く）の予約キーを、今の値のまま `kanban set` で書き直す。**
   予約キーは最後に書いたのが Chief of Staff のときだけ信じられる。前のアドレスで書いたキーは「Chief of Staff が書いていない」とみなされ、dispatcher はレビュアーを起動せず、ゲートは `needs_human` を返す。
   課題ごとに `kanban show <id>` で今の値を読み、同じ値で `kanban set <id> review=… must_fix=… reviewer=… review_rounds=… merge=…`（`brief.md` の表。`estimate`・`deliverable`・`machine` があれば同じく）と打つ。値は変えない。`estimate` を打ち忘れると見込みが「無し」に落ち、ズレの通知が黙る。
2. `hc --skill` を1回読む（hc の状態の意味・終了コードの一次情報。版で変わるので写さない）。
3. `projects/<slug>/AGENTS.md`（cwd にあるので自動で読まれる）の「Chief of Staff」の節を見る（目標・指示・並行の上限・起動に人の go が要るか・独立レビューを入れる条件）。無ければ並行の上限は 2。
4. 下の「毎ターン」をして、`watch.md` の止まった検知をする。

## 毎ターン（人の発言・届いた文を処理する前に）

- `kanban list --json` — 課題と状態（todo・queued・doing・review・done）。
- `hc ls --json --label project=<slug>` — 動いている担当と状態（idle・working・blocked・error）。
- **状態の正は hc の状態**。画面の文字から推さない。
