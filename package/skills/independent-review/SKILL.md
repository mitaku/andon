---
name: independent-review
description: Chief of Staff が、担当の成果（PR の差分）を実装と別ブランドのレビュアーに独立レビューさせるときに読む（台帳のレビュアーの課題）。入れる条件・プロファイルの選び方・dispatcher がレビュアーを起動する形・依頼書の型と結果の宛先・振り分けと周回の上限（担当の工程）を持つ
argument-hint: "[PR の URL か番号]"
---

# independent-review

**担当の成果（PR）を、実装と別ブランドのレビュアーに独立レビューさせる。** 読むのは Chief of Staff。
レビューは担当の工程で、頼むのも指摘を振り分けるのも担当（worker skill の §4）。Chief of Staff は入れるかを決め（依頼書の Settings）、レビュアーは dispatcher が起動する。
レビュアー側の決まりは worker skill の §6。

## 1. 入れるか決める

次のどれかに当たれば入れる。

1. 外向きの面に触れる（listen・認証・exec・秘密の扱い）
2. 新しい module を足す
3. テストを除いて約500行を超える
4. 人が求めた

**入れる条件はプロジェクトごとに決める**。趣味・個人のプロジェクトでは独立レビューを要らないとしてよく、
その場合の既定は「①か④のときだけ入れる」。決めた条件は `projects/<slug>/AGENTS.md` の「Chief of Staff」の節に1行で書く。

Chief of Staff が依頼書を書くときにこの条件で決め、Settings の `Review` に `on` / `off` で書く（既定 `off`）。

## 2. プロファイルを選ぶ

- **実装と別ブランド**。依頼書の Settings の `Reviewer`（既定 `codex-review`）。
  - Claude が書いたもの（profile `claude`・`sonnet`・`fable`）→ `codex-review`。
  - Codex が書いたもの（profile `codex`）→ `claude-review`。
- **強さはプロファイルの template が決める**（`package/templates/`）。`codex-review` は gpt-6-luna・effort max、`claude-review` は opus・effort medium。依頼書でモデルや effort を変えない。
- プロファイルは hc の template（`hc templates`）。無ければ人に作ってもらうよう頼む。Chief of Staff は作らない。

## 3. レビュアーを起動する（dispatcher）

担当が `andon issue review-request` を打つと、dispatcher が拾ってレビュアーを起動する（回ごとに子課題 `<id>-r<回>` と detached の worktree。起動する条件と手順は `docs/dispatcher.md` §5 の P6a）。**Chief of Staff は起動しない。**
上限を超えた回（`extra: true`）は起動されず、通知 `report` で Chief of Staff に届く。起動するかの決め方は chief-of-staff skill の `playbooks/gate.md` の「上限を超えた回」。
信頼の画面はセルでは出ない。出たら答えずに人へ回す（`chief-of-staff/playbooks/blocked.md`）。codex を初めて使う repo のフックの確認画面も人へ回す。

## 4. 依頼書の型

dispatcher が書く（`internal/dispatcher` の `reviewerBrief`。型を変えるときはそこと、この節を一緒に変える）。レビュアー側の決まりは worker skill の §6。要点:

- **plan と担当の報告は見せない**。渡すのは差分と仕様の正（元の課題の依頼書の完了条件・README・docs の該当節）だけ。
- 見てほしいこと（重い順）: 一番怖いところ（外向きの面）・正しさ（並行性・失敗の波及・境界）・仕様と実装の食い違い・テストが主張を本当に検証しているか。
  加えて、画面に触れる差分（条件表の `screen_paths` に当たるファイル）に画面の証拠（PR の画像の添付）が無い・証拠と差分が合わないときは major の指摘にする。
- **結果の宛先は元の課題の担当だけ**。Chief of Staff・dispatcher へは送らない（Chief of Staff へは dispatcher が届ける）。

## 5. 振り分け・周回・片付け

振り分けと周回は担当の工程（worker skill の §4）。ゲートが数えない指摘（`preexisting: true` と、2回目以降の回の critical でない新しい指摘）は contract.md §3。Chief of Staff がするのは次だけ:

- 指摘を中継しない（レビュアーの `hc send` は担当に直接届く）。
- 上限を使い切っても critical / major が残るときの追加の回は、`playbooks/gate.md` の「上限を超えた回」で決める。
- 課題の前からの問題（`preexisting: true`）を直すかは、報告に残った名前を読んで決める（切るなら課題にする）。
- レビュアーが使用量の上限で止まったときの起動し直しは dispatcher がする（`docs/dispatcher.md` §2.8）。`reviewer` を書き換えない。
- 片付けは元の課題の `andon issue finish` で、レビュアーの子課題と detached の worktree も片付く（`docs/dispatcher.md` §2.7）。
- 台帳のルーチン（`on: [review]`）でレビューを自動で足さない。入れる条件は Chief of Staff の判断で、依頼書は PR ごとに変わるので。

## pstack を使う repo

pool が Claude だけだと judge / arena の cross-judge が回らない。その repo では、
この別ブランドの独立レビューを cross-judge の代わりにしてよい。

## ルール

- レビュアーに plan・担当の報告を見せない。
- レビュアーには直させない。直すのは担当。
