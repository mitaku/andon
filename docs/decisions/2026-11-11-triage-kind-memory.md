# トリアージの memory の項目: key はファイル名と本文から、wiki 行きは close「wiki へ」で表し、apply が keep 以外の memory を消す

- 日付: 2026-11-11
- 出典: ADR-0020 決めたこと 3・5、受け入れ条件 3・4。A-0144 の実測（`/workspace/memory/` はサブフォルダを切らず、全セッションが直下に1件1ファイルと共有の `MEMORY.md` を置く）。課題 A-0145

- `andon triage collect` は `<ANDON_WORKSPACE>/memory/`（既定 `/workspace/memory/`）の直下の `.md`（`MEMORY.md` を除く）を1件1項目（`kind: memory`）にする。`from` はファイル名から `.md` を落とし、id に使えない文字を `-` にしたもの。`key` は今までと同じ `<slug>:<from>:memory:<本文の sha1>` で、ファイルごとに分かれる。`text` はファイルの本文、`repo` は空、`age_days` はファイルの更新からの日数。置き場が無い・空なら項目は無い。
- 理由: key の形を変えずに済み、決定の検査（`keyRe`）・記録・前回の仕分けの除外がそのまま効く。本文の sha1 が key に入るので、collect のあとにファイルが書き換わると `apply` は key を知らないとして止まり、確かめていない中身を消さない。
- `andon triage apply` は、`keep` 以外に仕分けた memory の項目について、記録（`library/triage/`）に本文と理由を書いたあとで、ファイルと、目録でそのファイルを指す行（`(<ファイル名>)` を含む行）を消す。目録の行が無くても落ちない（同時書き込みで欠けうる。A-0144）。`--dry-run` は消さない。出どころが課題ではないので、課題があるかの確かめ（preflight）は memory の項目を飛ばす。
- wiki 行きの memory は新しい decision を足さず、`close`「wiki へ: <題>」と、トリアージの報告の `## Next` の1行で Chief of Staff に渡す。本文は記録に言葉のまま残るので、`wiki/raw/memory/` へはそこから写せる（Chief of Staff の手順は M3 で chief-of-staff skill に書く）。
- 捨てた案: wiki 行きに decision `wiki` を足す——decisions.json の形・要約・skill の表が増え、Chief of Staff が反映する道は close と変わらない。
