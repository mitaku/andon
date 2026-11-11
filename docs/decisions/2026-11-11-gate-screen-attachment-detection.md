# ゲートが「画面の証拠」と数える添付は、PR の本文かコメントにある GitHub の添付の URL（user-attachments/assets）だけ

- 日付: 2026-11-11
- 出典: ADR-0018 受け入れ条件 3（「添付の検出の形は担当が決め、テストに固定する」）。課題 A-0112

ゲートの条件表の `screen_paths` に当たるファイルが PR の差分にあるとき、ゲートは `gh pr view --json body,comments` の本文とコメントのどれかに `https://github.com/user-attachments/assets/<id>` があれば画面の証拠があると数える。無ければ `fix`（[contract.md](../contract.md) §7）。

- 数えるもの: `gh pr create`／`gh pr comment --attach` が本文に足す画像（`![alt](https://github.com/user-attachments/assets/<id>)`）と動画（URL だけ）、Web の画面で上げた画像（`<img src="…/user-attachments/assets/<id>">`）。どの書き方でも同じ URL の形になるので、URL だけを見る。
- 数えないもの: 外の画像の URL（GitHub の権限で守られず、消えうる）、書き換えられていない相対パス（`![x](./a.png)`。`--attach` で上げ損ねたもの）、画像・動画でない添付（`user-attachments/files/`）。
- 見ないもの: 添付がどの commit で撮られたか、誰が付けたか。head が進んだあとに撮り直したかは、レビュアーの観点（ADR-0018 決めたこと 4）と人が見る。
- 既定の条件表（`package/gate.json`）には `screen_paths` を足さない（無ければ空。Andon の repo は画面を持たない）。
