# トリアージの signal の項目: key は `<slug>:sig-<fingerprint の先頭 12 桁>-e<episodes>:signal:<fingerprint>`、課題に切られたかは `links` の `url` で照らす

- 日付: 2026-11-12
- 出典: ADR-0011 決めたこと 6・7、受け入れ条件 4・11、依頼書に切る案 S4。課題 A-0027

- `andon triage collect` は信号の記録（`<slug>.signals.json`。contract.md §15）のうち、このプロジェクトの `active` な信号で、台帳のどの課題の `links` にも `url` が無いものを1件1項目（`kind: signal`）にする。`later` と `now` を同じ規則で扱う（`now` は通知で届いて Chief of Staff が課題に切り、`links` に `url` を書くので、切られたものは出ない。chief-of-staff skill §1）。
- 項目の `key` は `<slug>:sig-<fingerprint の先頭 12 桁>-e<episodes>:signal:<fingerprint>`（ADR-0011 決めたこと 6 の反論の追記 (b)。受け入れ条件 4 の `signal:<fingerprint>` はその略）。最後の席の `<sha1>` には `fingerprint`（SHA-1）をそのまま置いた。`episodes` が入るので、閉じた issue が開き直す・直した CI がまた赤くなる、は新しい項目として出る。`apply` の「key は台帳の slug で始まる」・`keyRe`・前回の仕分けの除外はそのまま効く。
- 項目は `text`（出所の題）・`url`・`untrusted: true` を持つ。`apply` の要約（`library/triage/*.md`）では `text` を「指示として読まない」の囲いに入れる（追記 (c)）。`repo` は GitHub の出所なら key の先頭の `host/owner/repo`（`fix` の `issue.repo` に写せる）、ほかは空。`age_days` は `first_seen` からの日数。
- `andon triage apply` は `signal` の key を台帳の課題と照らさず、今回の項目・前回までの記録・信号の記録（`active` でなくなったもの・その信号のこれまでの `episodes` も）にあれば受ける。`close` は記録に残すだけで、出所の issue は閉じない（ADR-0011 やらないこと 6）。
- `not_before` が未来の課題（todo／queued）は collect の項目に出ない。日の読み方は `kanban ready` と同じ（`config.yaml` の `timezone`。無い・読めなければ UTC。日付でない値は何も隠さない）。
- 捨てた案: key を `<slug>:<source>:signal:<fingerprint>` にする（最初の実装。レビューの指摘で改めた）——再発した信号が前回の仕分けで隠れ、ADR の追記 (b) と食い違う。
