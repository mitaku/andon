---
name: triage
description: 台帳のトリアージを受け持つ担当が読む。依頼書の1行目のゴールの読み方・`andon triage collect` の打ち方・項目ごとの確かめ方と判定表（close / fix / bundle / person / keep）・decisions.json の形・報告（人向けの要約表）の書き方・読むだけの決まりを持つ。依頼書が「トリアージ」で始まり、この SKILL.md を読めと書いてあるときに使う
---

# triage

**台帳に溜まったもの（報告の `## Next`・見送り・`## Remember`・`For AGENTS.md:`・`## 人に聞くこと`・担当の無い todo・人の判断待ち・片付けの残り）を、
1つのゴールの下で項目ごとに仕分ける。** 拾うのは `andon triage collect`、台帳に手を入れるのは Chief of Staff が打つ `andon triage apply`。
あなたは**確かめて仕分ける**だけで、決めたことは `decisions.json` と報告に書く。

- 手順の土台は worker skill（`deck start`・報告の書式・`deck report`／`deck review`）。この SKILL はその上に仕分けの手順を足す。
- トリアージは repo の無い課題。cwd は台帳の `issues/<自分の id>/`。書いてよいのは `issues/<自分の id>/library/` と報告（`deck report`）だけ。
- 用語は glossary skill に従う。

| する | しない |
|---|---|
| `andon triage collect`・`andon triage apply <自分の id> --dry-run` | `andon triage apply` を `--dry-run` 無しで打つ（打つのは Chief of Staff） |
| 確かめるために読む（`gh pr view`・`git -C <repo> grep`・`deck show`・`git worktree list`・docs） | `deck add`・`deck done`・`deck set`（ほかの課題）・元の報告を書き換える |
| `issues/<id>/library/decisions.json` と報告を書く | リポジトリに書く・commit・PR・エージェントを起こす |

## 1. ゴールを読む

- 依頼書の `ゴール:` の行（人が言ったときは1行目の「トリアージ: <ゴール>」）が、**今回どの項目を扱うか**のフィルタ。
  ゴールの外の項目は閉じず、起こさず、`keep`（理由にゴールを書く）。
- 既定のゴール（ルーチンの依頼書）: 前回のトリアージのあとに増えた項目を全部仕分け、台帳の未処理を 0 にする。
- ゴールが新しい仕事（「〜を作る」）なら仕分けず、報告の `## Next` に「普通の課題に切る: <要点>」と書いて返す。

## 2. 拾う

```
andon triage collect > library/items.json        # cwd は issues/<自分の id>/
```

- 出力は項目の JSON 配列（`key`・`kind`・`from`・`repo`・`text`・`age_days`）。古い順に並ぶ。前回までに仕分けた `key` は出ない（`keep` は出る）。
- `kind` は `issue`（担当の無い todo／queued）・`person`（人の判断待ち）・`next`・`waived`（見送り）・`remember`・`agents`（`For AGENTS.md:`）・
  `question`（`## 人に聞くこと`）・`leftover`（done なのに worktree か端末が残っている）・`unreadable`（報告が読めない）。
- **終了コード 4**（項目が上限 50 を超えた）: 依頼書に件数の指定（「古い順に N 件」）があればそれ、無ければ 50。
  `--limit` を増やして全件を取り、**先頭（古い順）から N 件だけ**仕分ける。残りは decisions.json に入れない（次回も出る）。報告に残りの件数を書く。
- 拾うのは機械なので、見出しで切っただけの行も混じる（見送りの見出しの行・「ここまで」の行など）。そういう行も1項目として仕分ける（`close`「その場限り」でよい）。

## 3. 確かめて仕分ける

項目ごとに、**言われたことを推さず実物で確かめて**から（現地現物）、下の表を上から当て、最初に当たったもので決める。
7（bundle）は他の項目を見ないと決まらないので、先に全項目を見て「直せる」ものを repo ごとに数えてから当てる。`reason` は1行（改行を入れない）。

確かめ方の例: `gh pr view <n> --repo <owner/repo> --json state`・`git -C "$GHQ_ROOT/<repo>" grep -n <語>`・`git ls-remote --heads`・
`deck show <id>`（同じ内容の課題が done か）・`git worktree list`・`hc ls --all --label project=<slug>`・docs の「決めたこと」の表。

| 順 | 条件 | decision |
|---|---|---|
| 1 | ゴールの外（§1） | `keep`（理由にゴール） |
| 2 | **対象が無い**: 言う PR が MERGED／branch が消えた／ファイル・関数が無い／「人が PR #n をマージする」で PR が MERGED／「andon issue finish」で worktree も端末も無い | `close`「対象が無い: <確かめ方>」 |
| 3 | **済んでいる**: 同じ内容の課題が done／リポジトリの文書に既に入っている（`git grep`）／docs の「決めたこと」で決まっている | `close`「済み: <どこで>」 |
| 4 | **起きない**: 見送りの `trivial`（起きる条件を示せない）／`minor` のうち、起きる条件がその repo に無い | `close`「起きない: <理由>」 |
| 5 | **人にしかできない**: 実機・別のセル・外部サービスの UI（Revoke・支払い・プランの変更）・人の目視が完了条件 | `person`（`kind` を付けない。`question` に期限の目安） |
| 6 | **灯りがつく決断**（呼び出し＝事業〔何を作るか〕・利用者＝使い勝手の好み・約束＝ほかの人や外部への約束と公開。定位置停止＝不可逆〔消す・本番・push 先・権限・秘密〕）、またはゲートのレベル C（ラインそのものの変更・信頼境界・権限・秘密・1000 行超） | `person`（`kind: business\|users\|promise\|irreversible`。`pick` に推奨案を1行） |
| 7 | **直せて、束ねられる**: 8 の条件を満たし、同じ repo で各々が小さい（目安 50 行以下）ものが、今回の項目に 3 つ以上ある | `bundle`（同じ `bundle` 名。1課題1 PR、commit は意図ごと） |
| 8 | **直せる**: 完了条件が 1〜3 行で書け、差分が 1 repo に収まり、レベル A か B、可逆 | `fix`（`issue` を付ける。§4） |
| 9 | 待つものがある（`after` に書ける id・リリース待ち・作業中の課題） | `keep`（理由に待つものの id） |
| 10 | それ以外（判断がつかない） | `person`（`kind` 無し。`question` は「判断がつかない: <何が分からないか>」） |

種類ごとの決まり:

- **「人が PR #n をマージする」**: PR が open なら `close` ではなく `keep`（「待つ: PR #n のマージ」）。MERGED なら 2 で `close`。
- **`leftover`**: PR が MERGED（repo の無い課題は done）なら `keep`「片付け待ち: andon issue finish <id>」。片付けるのは Chief of Staff で、片付けば次回は出ない。
  MERGED でない・commit していない変更があるなら 6 の `irreversible`（消してよいかは人）。
- **`question`**（人の課題になっていない `## 人に聞くこと`）: 表のとおり 5・6 が先。灯りがつく決断に当たれば、推奨案があり後で変えられても `person`。
  5・6 に当たらず推奨案があり後で変えられるものだけが 8 の `fix`（推奨案で決めて文書の「決めたこと」に「仮（<日付>・Chief of Staff）」と書く課題）。
  例: 使い勝手の既定値は `person`（`users`）。テストの置き場や文書の節の分け方は `fix`。ask-one-by-one skill §1 と同じ線。
- **`agents`**（`For AGENTS.md:`）: 行き先が決まっているので `fix`。同じ repo に 3 つ以上なら `bundle`（「AGENTS.md に n 行足す」。profile は `sonnet`）。
- **`remember`**: (a) リポジトリの作法 → `agents` と同じ。(b) Andon の skill・パッケージの直し → ラインの変更なので 6 の `person`（レベル C）。
  この台帳が Andon 自身のプロジェクトでなければ `close`「Andon へ委譲: <要点>」とし、報告の `## Next` に「Andon のプロジェクトの Chief of Staff へ委譲: <要点>」を1行（渡すのは Chief of Staff。規約10）。
  (c) レビューで繰り返し出た指摘 → その repo の `REVIEW.md` への `fix`。(d) その場限り → `close`「その場限り」。
- **`unreadable`**: 自分で `report.md` を開いて読む。拾うものがあれば中身で表に当て（1 key に 1 decision なので、直すものは `fix` 1つにまとめる）、無ければ `close`「読んだ: 拾うものなし」。
- **`issue`**（担当の無い todo／queued）を `close` できるのは、今も担当が無い todo／queued のときだけ（`apply` が確かめる）。`close` すると `apply` がその報告の末尾に理由を足して done にする。

人に届くものを減らす線（ask-one-by-one skill §1 と同じ）: 5 と 6 と 10 だけが人へ行く。`kind` 無しの `person` は「エージェントで決められたのでは」と要約で目立つので、
本当に分からないときだけ使う。

## 4. decisions.json

`issues/<自分の id>/library/decisions.json`（`andon triage apply` が読む）。キーと形は `apply` のコードと同じ:

```json
{
  "goal": "台帳の未処理を 0 にする",
  "collected": "2026-11-10T00:05:00Z",
  "items": [
    {"key": "collect が返した key", "decision": "close", "reason": "対象が無い: <owner>/sample-site#24 は MERGED"},
    {"key": "collect が返した key", "decision": "fix", "reason": "起きる条件があり直し方が報告にある",
     "issue": {"title": "修正の題", "repo": "github.com/<owner>/sample-site", "profile": "claude", "review": "on", "brief": "何をどこで直すか・完了条件・守ること"}},
    {"key": "collect が返した key", "decision": "bundle", "bundle": "sample-site-agents-md", "reason": "AGENTS.md に 2 行",
     "issue": {"title": "AGENTS.md に検査のコマンドを足す", "repo": "github.com/<owner>/sample-site", "profile": "sonnet", "review": "off", "brief": "足す行"}},
    {"key": "collect が返した key", "decision": "person", "kind": "promise", "reason": "レベル A を人が見ずに入れる線",
     "question": "レベル A の PR を検収と検査だけでマージしてよいか", "pick": "よい。危ない面はレベル B・C で絞っている"},
    {"key": "collect が返した key", "decision": "keep", "reason": "待つ: S-0004 のマージ"}
  ]
}
```

- `key` は `collect` の出力をそのまま写す（`<slug>:<id>:<kind>:<本文の sha1>`）。書き換えない・作らない。1 key に1つ。`text` は書かない（`apply` が収集結果から取る）。
- `decision` は 5 値。`reason` は空にしない・改行しない。`goal` と `collected`（collect を打った時刻）も空にしない。
- `fix`／`bundle` は `issue`（`title`・`repo`＝`host/owner/repo`・`profile`・`review`＝`on`／`off`・`brief`）が必須。`bundle` は `bundle` 名も必須で、同じ名前は同じ repo に限る。
  - `brief` は `apply` が依頼書の「解決すべき問題:」の下に置く（完了条件・守ること は汎用の1行しか付かない）ので、**具体的な完了条件と守ることは `brief` の中に書く**。出どころ（`<id>` の何の行か）も1行。
  - `review` はレベル B（command・スクリプト・案件の `.claude/skills/**`）か independent-review skill §1 の条件に当たれば `on`。`profile` は chief-of-staff skill §1 の表で選ぶ（文書の小直しは `sonnet`）。
  - レベル C に当たる変更は `fix` にしない（表の 6 で `person`）。
- `person` は `question`（1行・`-` で始めない）が必須。`kind` は `business`／`users`／`promise`／`irreversible` か書かない。`kind` があれば `pick`（推奨案）が必須。
- `fix`／`bundle` が上限（既定 3。bundle は 1 つで数える）を超えた分は、`apply` が `keep`「次回（起こす上限）」に落とす。並べる順（先に書いたものが残る）＝優先の順。

書いたら確かめる:

```
andon triage apply <自分の id> --dry-run > /dev/null    # 終了コード 0 が完了条件。1 なら stderr の理由を直す
```

## 5. 報告して返す

worker skill §7 の書式で、`## Report` に人向けの要約を書く。人が読むのはこれと `apply` の要約だけなので、表で短く。

```
## Report
ゴール: <1行>。拾った <n>（うち仕分けた <m>・次回へ <n-m>）。
閉じる <a> ・ 直す <b>（うち束ね <c>） ・ 人へ <d>（kind 無し <e>） ・ 次回 <f> ・ 読めない <g>
decisions.json: issues/<id>/library/decisions.json。`andon triage apply <id> --dry-run` は通った。

| decision | 出どころ | 項目（短く） | 理由 |
|---|---|---|---|
（close・fix・bundle・person・keep の順。人へのものは問いと推奨案も）
## Next
- Chief of Staff: andon triage apply <id> --max-issues <上限>
- （委譲・片付け待ちがあれば1行ずつ）
## Remember
```

- 問いを `## 人に聞くこと` に並べない（人の問いは `apply` が人の課題にし、Chief of Staff が1問ずつ出す）。`deck ask` もしない（誰も待っていない問いなので）。
- `deck report <id>` → `deck review <id>`。Chief of Staff へ `hc send` しない（dispatcher が届ける）。
- 直しが届いたら（`<hc from=…>`）、`deck start <id>` で戻し、decisions.json を直して `--dry-run` から通し直す。
