# 課題の一生のモデルは repo の根の model/ に置き、入口は model/check、CI は workflow model の job model で回す

- 日付: 2026-11-19
- 出典: A-0012（ADR-0025 の依頼書に切る案 M1。ADR は「入口の名前と置き場は実装の課題が決める」とした）

- 置き場は repo の根の `model/`。モデル（`IssueLife.tla`）・規則（`IssueRules.tla`・`IssueRules.cfg`）・壊した見本（`broken/`）・控え（`expected.txt`）・検査器のバージョン（`package.json`・`package-lock.json`）を置く。`process/`（定義の YAML だけ）と `docs/`（文書だけの PR と見なされる）の下には置かない。
- 入口は `model/check`。手元も CI も同じこれを打つ。検査器は lock のとおりに一時の場所へ `npm ci --ignore-scripts` で入れ、npm の置き場も一時の場所にする。モデルの写しを一時の場所で回すので、repo にも HOME にも書かない。`MODEL_CHECKER` に TLC と同じ旗を受けるコマンドを渡すと、それで回す（TLC と見比べるとき）。
- 壊した見本は、規則のモジュールを `EXTENDS` して行動を1つだけ直す前の形に差し替えたモジュールにする。規則の並びは `IssueRules.cfg` から作るので、見本も本物と同じ規則を調べる。
- CI は新しい workflow `model` の job `model` で回す（tests・go の workflow に混ぜない）。秘密を参照せず、権限は `contents: read` だけ。Node は runner に入っているものを使い、setup の action を足さない。
- 規則の名前は `R<番号>_<英字の名前>`（例 `R3_CleanupIsWhole`）。検査器の出力に番号が出る。
