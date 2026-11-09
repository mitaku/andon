# レビュアーの指摘に preexisting を足す。ゲートは数えず、PR が足した行なら数える

- 日付: 2026-11-09
- 出典: 人の依頼（2026-11-09。andon の次の手のレクチャー §3.D「レビューを詰まらせない」）・課題 A-0005。先輩プロジェクト shepherdr の 4 世代の整理（「Follow-ups are named, not done」。課題の前からあった問題は差し戻さず、名前だけ残す）。[ADR-0004](../adr/0004-project-layout.md) が「ゲートの `preexisting`」を報告の Next に挙げたもの

レビュアーの `findings.json`（[contract.md](../contract.md) §3）の指摘に任意のキー `preexisting`（真偽）を足す。課題の前からあった問題（差分の外の古い問題）なら `true`。版は上げない（contract.md §0 の原則4）。

- ゲートは `preexisting: true` の指摘を `must_fix` の数に入れない。差分の外の古い問題でレビューが回り続けるのを止める。
- **ただし、指摘の場所（`file`・`line`）が PR の差分で足した行なら数える。** 課題で足した行が課題の前からあったはずはない。申告でゲートを緩めない（原則2）ための、機械が確かめられる線。ゲートは `gh pr diff` の unified diff から足した行を読む（すでに読んでいる差分。新しい取得は無い）。
- レビュアーは `preexisting` を差し戻しの理由にせず、課題にもしない（課題を足すのは Chief of Staff）。報告に名前だけ残す（場所と1行）。担当は振り分けない（直さない・`waived` にも書かない）。
- `andon issue ready`・`andon issue show` の照合は場所を確かめない（PR の差分を持たない下見）。正はゲート。

人が決めていないことで推奨で進めたもの:

- 差分で足した行に当たる `preexisting` を数える（レビュアーの申告をそのまま信じない）。足した行の隣の古い行の指摘は数えない（隣かどうかは機械に決められない）。
- ゲートの記録（`gate.schema.json`）に数えなかった件数は足さない。人はレビュアーの報告で名前を読む。

捨てた案:

- **`preexisting` をそのまま信じる**: レビュアーが課題で足した行を `preexisting` と書くと、Must fix が素通りする。
- **ゲートが前のコミットの `git blame` で確かめる**: 一時 worktree と blame が要り、差分の足した行で足りる線に比べて重い。
- **`preexisting` の指摘を Chief of Staff への課題に自動で切る**: 「名前だけ残す」に反し、機械が課題を増やす（ADR-0006 の、切るのは Chief of Staff）。
