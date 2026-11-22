# ADR-0046: README を、初めて来た読み手が README だけで判断できる形に組み直す

- 日付: 2026-11-22
- 出典: ADR-0046（[adr/0046-readme-for-first-time-readers.md](../adr/0046-readme-for-first-time-readers.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-21「README自体をもう少しブラッシュアップをかけたいと思っています」）
- 覆す: [decisions.md](../decisions.md) §17 の 2026-11-06 の description の決定のうち、3 語（一部。tasks・a ledger・the human を issues・a kanban・the owner に替える）。[ADR-0029](../adr/0029-factory-place-and-mechanism.md) のやらないことのうち「description を変えること」（一部。同じ 3 語）。[ADR-0033](../adr/0033-adr-inventory-and-slimming.md) 決めたこと 6 のうち、ADR の一覧の置き場（一部。README から `docs/adr/README.md` へ。検査が見ることは変えない）

2026-11-21 の 1 日で repo の入口を 2,402 人が見て、その大半は README しか読んでいない。README を、初めて来た読み手が数分で「何であるか・どう回るか・自分に合うか」を判断できる形に組み直す。`README.md` を英語にし、日本語は `README.ja.md` に置く。冒頭の説明のすぐ下に、個人が学習と実験で作っている不安定なものだという注記を置く。課題 1 本の流れと行燈の図を Mermaid で 1 枚足し、オーナーが呼ばれる決断を表で見せる。ADR の一覧は `docs/adr/README.md` に移し、検査の読み先も移す。2 つの README の構成がずれないことは検査 1 本で守る。repo の description の 3 語（tasks・a ledger・the human）を、issues・a kanban・the owner に替える。Andon 自身の開発の流れの数は README に載せない。README の本文の PR は、独立レビューが通れば自動でマージする。画面の写真・動画・social preview・topics は範囲の外。

捨てた案:

- **1 ファイルのまま日本語を先に置く**: Chief Engineer の推奨だったが、オーナーが GitHub の慣習に合わせて英語の `README.md` を選んだ。
- **README を短い入口にして中身を docs に置く**: 読み手の大半は README の先へ進まない。
- **手で描いた SVG の図**: 言語ごとに 1 枚要り、直すたびに座標を触る。
- **流れの数（マージした PR の数など）を README に載せる**: Chief Engineer の推奨だったが、オーナーが「数は別にいらない」と選んだ。
- **README の本文の PR をオーナーが読んでからマージする**: Chief Engineer の推奨だったが、オーナーが自動のマージを選んだ。
