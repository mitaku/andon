# tests/

`for f in tests/*.sh; do bash "$f"; done` で全部回る。

要るのは bash・python3（標準ライブラリだけ）・jq・Go（`mise.toml` の版）。`tests/andon*.sh` は `andon` を `go build` で一時ディレクトリに焼いて回す（`ANDON_BIN` があればそれを使う。CI の go の workflow が渡す）。`tests/andon-boot.sh` は偽の HOME・偽の建屋・偽のパッケージで `andon boot` を2回打ち、生成と冪等を見る（PATH に hc を置かないので起こさない）。`tests/andon-triage.sh` は台帳の CLI（deck）があるときだけ回り、無ければ skip する。Go の単体テストは `go test ./...`。`tests/schema.sh` は `schema/` と `docs/contract.md` の例を突き合わせる（検査器は `tests/lib/schema_check.py`。schema に新しいキーワードを使うなら、先に検査器に足す。pattern には ECMA-262 と Python で意味が割れる `.`・`\s`・`\d`・`\w`・`\b` を使わず、明示した ASCII の文字クラスで書く。lint が落とす）。`tests/schema.sh` は制約を緩めた schema の写しで自分を回す自己検査を持つので、1分ほどかかる。

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

## image の検査

`tests/image.sh` は Docker があれば実際にビルドしてコンテナを起動する。手順と CI の配布条件は [docs/image.md](../docs/image.md)。Docker が無いローカルでは SKIP、CI では失敗する。
