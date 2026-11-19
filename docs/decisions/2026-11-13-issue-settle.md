# 人のマージの拾いと片付けを andon issue settle に出し、自分の記録 <slug>.settle.json を持つ

- 日付: 2026-11-13
- 出典: 課題 A-0096（[ADR-0017](../adr/0017-process-definitions.md) の切る案 P7。決めたこと 5・6、受け入れ条件 7 の settle。コマンドの名前は「切る案の担当が揃える」とした仮の名）

人のマージの拾いと片付け（[dispatcher.md](../dispatcher.md) §2.7）を、dispatcher の中から `andon issue settle <id>` に出した。

ここで置いたこと:

- 名前は仮の名のまま `andon issue settle`。記録は `~/.local/state/andon/dispatcher/<slug>.settle.json`（課題 → `pr`・`commit`・`at`・`state`・`head`・`sha`・`held`）で、書き手は settle だけ。dispatcher の控え `<slug>.json` から `watch` を外した。
- 出力は stdout の JSON（`issue`・`state`・`notify[]`）。待ちに積むのは dispatcher。出力と記録は schema にしない（`andon issue impact` と同じく、約束は通知の `detail`。形は [contract.md](../contract.md) §6）。
- dispatcher がマージしたかは、ゲートの記録 `library/gate/<マージされた head>.json` の `merged` で見分ける。head は settle が `gh pr view` で読む `headRefOid`。
- 記録の lock は、台帳ごとの `<slug>.issue.lock`（start・finish・resume と同じ）を使う。finish は settle の中で打つ（別プロセスにすると同じ lock を待って止まる）。
- `report.json` の `pr` か `commit` が替われば、新しい PR として見直す（`doing` に戻って直した課題が、前の CLOSED の記録で止まらない）。`review` にいない課題の記録は、settle が打たれたときに捨てる。
- `merge: auto` で dispatcher の `gh pr merge` の直前に人がマージしていたときは、同じ巡で settle が聞き直して片付ける。settle がその PR を1分以内に聞いていたら、次に聞く巡で片付く（前は同じ巡で、GitHub に聞き直さなかった）。
