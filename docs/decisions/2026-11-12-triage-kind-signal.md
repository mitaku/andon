# トリアージの signal の項目: key は `<slug>:<source>:signal:<fingerprint>`、課題に切られたかは `links` の `url` で照らす

- 日付: 2026-11-12
- 出典: ADR-0011 決めたこと 6・7、受け入れ条件 4・11、依頼書に切る案 S4。課題 A-0027

- `andon triage collect` は信号の記録（`<slug>.signals.json`。contract.md §15）のうち、このプロジェクトの `active` な信号で、台帳のどの課題の `links` にも `url` が無いものを1件1項目（`kind: signal`）にする。`later` と `now` を同じ規則で扱う（`now` は通知で届いて Chief of Staff が課題に切り、`links` に `url` を書くので、切られたものは出ない。chief-of-staff skill §1）。
- 項目の `key` は `<slug>:<source>:signal:<fingerprint>`。ADR の書いた `signal:<fingerprint>` を、ほかの項目の key の形（`<slug>:<出どころ>:<kind>:<40桁の16進>`）に揃えて置いた。理由: `apply` の「key は台帳の slug で始まる」（ほかの台帳を指させない）・`keyRe`・前回の仕分けの除外がそのまま効き、`fingerprint` は SHA-1 で本文の sha1 の席に収まる。`from` の席は出所の名前で、人が key を見て出所が分かる。
- 項目は `text`（出所の題）・`url`・`untrusted: true` を持つ。`repo` は GitHub の出所なら key の先頭の `host/owner/repo`（`fix` の `issue.repo` に写せる）、ほかは空。`age_days` は `first_seen` からの日数。
- `andon triage apply` は `signal` の key を台帳の課題と照らさず、今回の項目・前回までの記録・信号の記録（`active` でなくなったものも）にあれば受ける。`close` は記録に残すだけで、出所の issue は閉じない（ADR-0011 やらないこと 6）。
- `not_before` が未来の課題（todo／queued）は collect の項目に出ない。日の読み方は `kanban ready` と同じ（`config.yaml` の `timezone`。無い・読めなければ UTC。日付でない値は何も隠さない）。
- 捨てた案: key に `episodes` を入れて、消えてまた出た信号をトリアージに出し直す——ADR は key を `fingerprint` で決めている。出し直しが要るかは使ってみてから決める。
