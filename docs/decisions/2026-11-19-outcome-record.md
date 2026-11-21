# 結果の記録は dispatcher が片付けのあとの段で課題の library/outcome.json に1回だけ書き、コストは measure と同じ数え方で起動の記録の全部のセッションから読む

- 日付: 2026-11-19
- 出典: [ADR-0028](../adr/0028-improvement-loop.md) 決めたこと 7・12 と「実装が決めてよいこと」

ADR-0028 が実装に任せた細部を次のとおり決めた。形と欄の意味は [contract.md](../contract.md) §19・`schema/outcome.schema.json`、段の置き場は [dispatcher.md](../dispatcher.md) §2.12。

- **ファイル**: 課題の `library/outcome.json`（ADR の案のまま）。1つの JSON で、在れば書かない。オーナーの手直し（`corrections.json`）と同じく、`done` の行を拾った課題ごとに1回だけで、ファイルの有無が「書いた」の印になる。
- **書く課題**: オーナーの課題（`who: person`）を除く全部。レビュアー・Chief Engineer・調べる課題にも書く。それぞれのセッションのコストを持つのはその課題で、レビュアーを親に束ねるのは読み手（振り返り）。親の記録にはレビュアーの子の課題の数だけを書く。
- **数え方の共有**: トークンの数え方は `andon measure` のものを `internal/sessionlog` に移し、measure と dispatcher が同じ関数を呼ぶ。measure は dispatcher を import しているので、dispatcher から measure は呼べない。モデルの id・effort・ターン数も同じ関数で、同じ行から読む。
- **ターン数**: Claude Code は本線の user の行の `promptId` の数（道具の結果の行も同じ `promptId` を持つので、重ねずに数えるとオーナーか役が入力した回数になる）。Codex は `turn_context` の `turn_id` の数。印の無い古い記録は `null`（0 と分ける）。
- **Codex のセッション**: 起動のときに id が決まらないので、cwd が課題の置き場（台帳の `issues/<id>/`・`worktree`・起動の記録の `repo.path`）の rollout を `session_meta` の id ごとに並べる。どれもその課題だけの場所なので、ほかの課題の記録は混ざらない（measure と同じ考え）。
- **PR の状態**: 片付けを拾ったあとに `gh pr view --json state,mergeCommit,mergedAt` で1回だけ読む。`open` のまま片付いて、あとでオーナーがマージしても書き直さない。
- **諦めたとき**: 読めなければ次の見回りで1回だけ打ち直し、それも駄目ならログに1行で控えから外す。手直しの `corrections` と違って通知は足さない。読み手は振り返りだけで、無い記録は `null` として読む（ADR-0028 決めたこと 12）ので、Chief of Staff が手で集め直すほどの値が無い。gh の上限で止めている間は打たず、回数にも数えない。
- **形式モデル**: 課題のステータスの移り方は変えない（`done` のあとに台帳の外のファイルを1つ書くだけ）。`model/` は変えない。`model/check` はそのまま通る。
