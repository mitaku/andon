# 流れと状態の定義の器（ADR-0017 P1）: YAML は `andon process json` で JSON にして schema に通し、生成物は `docs/process/` の README 以外

- 日付: 2026-11-11
- 出典: ADR-0017（決めたこと 1・2・16、受け入れ条件 1・2）の切る案 P1（A-0089。担当が推奨案で進めた）

- 定義の YAML の検査は、`andon process json <file>` が YAML を JSON にし、`tests/process.sh` が `schema_check.py` で `schema/process/*.schema.json` に通す（セルにも CI にも YAML の読み手〈PyYAML・yq〉が無く、Go の YAML の読み手は andon のバイナリに入っているため）。`andon process render` は知らないキーで止まるだけで、schema を写さない。
- `next[]` と `triggers[].start` の id が `steps[]` に在ることは schema ではなく `tests/process.sh` が見る（決めたこと 16 のとおり）。
- `docs/process/` の `README.md`（人が書く入口）以外の `.md` は生成物。`andon process render` は定義の無い生成物を消し、`--check` は作り直すと変わるファイルを並べて終了コード 1。YAML の `name` はファイルの名前と同じで、ワークストリームとステートマシンで名前を重ねない（同じ `docs/process/<名>.md` になるため）。
- 決めたこと 16 が 1 つの値と並びの両方で書いている `skill`（`skill: chief-of-staff#5` と `skill[]`）と、1 つの状態に記録の項目が複数ありうる `states[].record` は、1 つの文字列と並びのどちらも受ける。
- 入れ子の 1 件の必須（`triggers[]` の `name`・`kind`・`start`、`steps[]` の `id`・`title`・`by`、`transitions[]` の `from`・`to`・`on`・`by`、`states[]` の `id`・`title`）は schema で弾く。根の必須は決めたこと 16 のとおり 3 つだけ。
