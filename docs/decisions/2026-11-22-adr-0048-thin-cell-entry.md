# ADR-0048: セルの入口 `bin/bootstrap` を薄くし、本体を image から取り出す

- 日付: 2026-11-22
- 出典: ADR-0048（[adr/0048-thin-cell-entry.md](../adr/0048-thin-cell-entry.md)。Chief Engineer の課題。元はオーナーの 1 行 2026-11-22「前に作ったセルの bin/bootstrap を Andon に追従させる」）

セルの repo の `bin/bootstrap` を、image を焼いて本体を取り出すだけの薄い入口にする。今の中身は本体として `package/host/bootstrap` に移し、image の `/opt/andon/host/bootstrap` から、打つたびに `.andon/bootstrap` へ取り出して走らせる。Andon のバージョンを上げると、入口の動きも一緒に変わる。前に作ったセルは、上げたときに古い入口が `docker compose` のところで止まって案内を出し、オーナーが `andon init --print bin/bootstrap` を使う 1 行で、入口を 1 回置き換える。`host-install` が今あるファイルを上書きしない決まりと、雛形のほかのファイルは変えない。番号ごとの中身、理由、捨てた案は ADR が正。
