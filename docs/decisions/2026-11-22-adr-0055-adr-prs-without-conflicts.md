# ADR-0055: ADR を並べて書いても PR が衝突しない作りにする

- 日付: 2026-11-22
- 出典: ADR-0055（[adr/0055-adr-prs-without-conflicts.md](../adr/0055-adr-prs-without-conflicts.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-22「ADRがコンフリクトしてるので解消させてください（これは作りを見直さないといけないね」）
- 覆す: [ADR-0046](../adr/0046-readme-for-first-time-readers.md) 決めたこと 9 のうち、新しい ADR の PR が一覧に 1 行足すことと、検査が見ることを変えないこと（一部）。[ADR-0033](../adr/0033-adr-inventory-and-slimming.md) 決めたこと 6 のうち、README の ADR の一覧の状態を ADR の状態行と突き合わせること（一部）。

Chief Engineer が何人も同時に ADR を書くと、どの PR も ADR の一覧の末尾に 1 行足すので、1 本がマージされるたびに残りが衝突していた。ADR の一覧は `docs/adr/README.md` に残すが、ADR の PR は一覧に触らない。一覧は `tests/adr-index.sh` が ADR の 1 行目と状態行から書き、バージョンを切る PR が作り直す。検査は ADR ごとの行と状態の語を求めず、一覧の見出しのバージョンを CHANGELOG と突き合わせる。ADR の番号は `tests/adr-new` が鍵を取って決め、その場でファイルを置く。同じ番号の ADR が 2 本あると検査が落ちる。

捨てた案:

- **一覧を repo に置かない**: Chief Engineer の推奨だったが、オーナーが、一覧を残して別の PR で作る形を選んだ。
- **ADR のマージごと、または 1 日 1 回、一覧を作り直す**: 担当の起動か定期のルーチンが増える。欠けて困ると分かったら足す。
- **`.gitattributes` の `merge=union`**: GitHub の PR のマージが見ない。
- **番号を PR の番号や日付の名前にする**: 書いている間に番号が無い。番号で指す行が多い。
