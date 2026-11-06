# ロゴ

Andon の記号（A の行燈）とクーリエの記号（提灯を運ぶ）。どちらも 64 の格子に手で描いた SVG で、ワードマーク（andon・courier）の字も手で描いたパス。フォントは使っていない。

| ファイル | 中身 |
|---|---|
| `andon-mark.svg`・`andon-mark-dark.svg` | Andon の記号だけ |
| `andon-wordmark.svg`・`andon-wordmark-dark.svg` | Andon の記号と andon の字 |
| `courier-mark.svg`・`courier-mark-dark.svg` | クーリエの記号だけ |
| `courier-wordmark.svg`・`courier-wordmark-dark.svg` | クーリエの記号と courier の字 |

`-dark` は暗い背景用。GitHub の README では `<picture>` と `prefers-color-scheme` で出し分ける（README.md の冒頭が例）。

**窓の色が状態の印になる。** 行燈の窓と提灯の灯りは、緑＝流れている・黄＝人の判断待ち・赤＝止まった。ファイルの既定は黄（灯りがついて人を呼んでいる。[design.md](../../design.md) §0 の「灯り」）。
色は `style="fill:var(--glow,…)"` で塗っているので、HTML に inline すれば CSS の変数で替えられる（`--glow`・`--glow-hi`。線は `--ink`、提灯の紙は `--paper`）。緑は `#2F9E62`／`#8FD8AE`、赤は `#D8453B`／`#F29A92`。
