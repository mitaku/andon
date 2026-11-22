# v0.0.x の間は、Andon のバージョンのタグを Chief of Staff が打つ

- 日付: 2026-11-22
- 出典: オーナー（2026-11-22「一通り落ち着きそうなところでv0.0.14のバージョンアップをしましょう。v0.0.xの間はCoSがタグを打ってもらったほうがよさそうです」）

Andon の repo の v0.0.x のタグは、Andon を開発するセルの Chief of Staff が打って push する。前は、[operate.md](../operate.md) §14 と [image.md](../image.md) の「CI と配布」に、打つのはオーナーと書いていた。

- 打つのは、CHANGELOG の節の PR のマージ commit が `origin/main` にあり、その commit の main の `go`・`model`・`tests` が通ったとき。
- push するのはそのタグの名前 1 つだけ。release の job の終わりまで見て、オーナーに知らせる。
- 既にあるタグは、動かさない・消さない。
- 許すのはタグの push だけ。委譲の値（`publish` を含む）は変えない。ほかの公開は今までどおりオーナーの go が要る。
- 効くのは v0.0.x の間だけ。v0.1.0 を切る前に、オーナーが決め直す。手順の中でオーナーを必ず通るとは言えなくなったので、Chief of Staff は、[operate.md](../operate.md) §14 の 2 の見直しの答えをオーナーから受けるまで v0.1.0 のタグを打たない。
