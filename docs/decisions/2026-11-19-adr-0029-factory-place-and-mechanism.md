# ADR-0029: 工場の2つの面を言い分ける（セルは場所、Andon はしくみ）

- 日付: 2026-11-19
- 出典: ADR-0029（[adr/0029-factory-place-and-mechanism.md](../adr/0029-factory-place-and-mechanism.md)。オーナーが Chief Engineer との対話の中で決めた。元は人の発言 2026-11-19「Andon の SoftwareFactory 部分は少し定義の再構築が必要かも、やることは変わらないのだけれども工場（CELL）としての境界を作り、規律を作るものだと思った」「Cell=工場だし、Andon=工場ではある。Cellは境界・空間で、Andonは工場のしくみというイメージ」）

「Andon は工場そのもの」（2026-11-04）は覆さず、どの面の名前かを足す。セルは工場の場所の面で、境界と空間。信頼の境界ごとに1つ立てる長寿命のコンテナと、その中に入るセルの repo でできている。Andon は工場のしくみの面で、境界の中で守る規律（役・工程・ゲート・約束）を指す。セルの repo は、外の語では Home Repo（Matt Pocock）に当たる。「セル」の語と識別子は残す。表示名「Andon — a one-person software factory」も残し、直後の説明文を2つの面で組み直す。用語集は Andon・セルの定義を直し、「セルの repo」の項を足す。「工場」「Kit」は項にしない。識別子・コマンド・repo 名・schema は変えない。同じ対話で出た Chief of Staff の単位の題は、ADR-0031（提案）に分けた。中身・理由・捨てた案は ADR が正。
