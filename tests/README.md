# tests/

`for f in tests/*.sh; do bash "$f"; done` で全部回る。

## public 検査の禁止リスト

`tests/public.sh` は、秘密の形に加えて、**固有の値の禁止リスト**を全ファイルに当てる。リストの中身（建屋名・ユーザー名・案件名など）が git の履歴に残らないよう、**リストは repo の外に置く**。

- 手元: `${ANDON_PUBLIC_DENYLIST:-$HOME/.config/andon/public-denylist}`
- CI: secret `ANDON_PUBLIC_DENYLIST`（中身を env で渡し、ワークフローが一時ファイルに書いて読む）。secret は人が置く。
- リストが無いときは、秘密の形だけを検査し、「禁止リストが無いので固有の値は見ていない」と出して通る。**public にする前の検査（design.md §5.3 の 8）は、必ずリストを置いて回す。**

書き方: 1 行に 1 語。大文字小文字は区別しない。`#` で始まる行と空行は無視。例:

```
# 建屋名
example-building
# 案件名
example-project
```
