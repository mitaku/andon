# schema の照合は andon の検査器 1 つにし、テストとセルは andon schema check で照らす

- 日付: 2026-11-20
- 出典: ADR-0034 決めたこと 6 の 2 段目（[adr/0034-prevention-before-tests.md](../adr/0034-prevention-before-tests.md)。依頼書に切る案 7）
- 覆す: decisions.md §8 の行（2026-11-05。python3 の標準ライブラリで書いた小さな検査器）と、§10 の行（2026-11-05。担当・Chief of Staff・Chief Engineer と tests は `schema_check.py` で照らす）

JSON を schema と照らす検査器は、`andon` が約束の JSON を書く前に照らすもの（`internal/contract` の `Checker`。既製の `github.com/google/jsonschema-go` に拡張のキーワード `uniqueKeys` を足したもの）だけにする。テスト（`tests/schema.sh`・`tests/process.sh`・`tests/package.sh`・`tests/andon-create.sh`・`tests/image.sh`）とセルで JSON を手で確かめるときは `andon schema check <schema> <json>` を打つ。`<schema>` は埋め込んだ名前（`report` など）か `*.schema.json` のパスで、合えば 0、合わなければ 1、検査できなければ 2 で終わる。まとめて照らす口は `andon schema batch`。Go のテスト（`cmd/kanban` の台帳の schema の検査と `internal/ledger/ledgertest`）も同じ `Checker` を使う。`package/lib/andon/schema_check.py` は、schema そのものの決まりの lint と文書の例の取り出しだけを持つ。

検査器は、知らないキーワードのある schema を黙って飛ばさずに検査できないとして断る（Python の検査器が持っていた守りを残す）。`uniqueKeys` は、値の形に沿ってたどれる場所（`properties`・`additionalProperties`・`items`・`prefixItems`・`$ref`・`allOf`）でだけ読み、`anyOf`・`oneOf`・`not`・`if`・`then`・`else` の中に置いた schema も同じく断る。読まない場所に置けると、一意を確かめたつもりで確かめない schema になるため。
