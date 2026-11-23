# ADR-0058: エージェントが PR に書いたものを、オーナーの手直しの記録に入れない

- 日付: 2026-11-23
- 出典: ADR-0058（[adr/0058-agents-stay-out-of-pr-conversation.md](../adr/0058-agents-stay-out-of-pr-conversation.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元は、振り返りが、担当の書いた PR のコメントをオーナーの手直しの記録に 2 件見つけたこと）
- 覆す: [ADR-0018](../adr/0018-screen-check-tooling.md) 決めたこと 3 のうち、PR を出したあとの画面の写しを `gh pr comment --attach` で添付すること（一部）。

セルのエージェントはオーナーと同じ GitHub のアカウントで書くので、オーナーの手直しの記録（`corrections.json`）の「PR の上の手直し」に、担当のコメントと Chief Engineer の commit が入っていた。どの役のエージェントも PR の会話欄（会話のコメント・レビューの本文・インラインのレビューコメント）に書かず、伝えることは PR の本文に書く。`gh pr comment` と `gh pr review` は、設定の deny と Codex の rules で断る。PR を出したあとの画面の写しは `gh pr edit --attach` で本文に足す。コメントを拾う規則は変えない。commit は、親が 2 つ以上のものと、Chief Engineer の課題のものを、オーナーの commit に数えない。担当の課題の境目は、担当の最後の報告のままにする。断れない書き方で書かれたコメントは今までどおり拾い、振り返りが中身を読んで見分ける。

捨てた案:

- **印を付けて書ける口を残す**: エージェントが会話欄に書いた 3 件は、どれも本文で足りた。使われていない口を作ることになる。
- **Andon の側に、書いたコメントの記録を持つ**: 記録と約束が 1 つ増える。
- **セルの GitHub の身元を分ける**: オーナーが入口で範囲の外にした。
- **`gh issue comment` と `gh api` も断る**: issue に書く用と、読む用があり、command の頭では分けられない。
- **Chief Engineer の手引きに、報告のあとに push したら報告を書き直すと足す**: 文だけの決まりで、Chief of Staff へ同じ知らせがもう一度届く。
- **commit にセルの印を付ける**: 印が commit と一緒に外へ出る。オーナーの commit は記録に 1 件も無かった。
