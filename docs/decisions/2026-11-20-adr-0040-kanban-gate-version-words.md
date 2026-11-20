# ADR-0040: 「台帳」「ゲート」「版」をやめ、kanban・gate・バージョンと書く

- 日付: 2026-11-20
- 出典: ADR-0040（[adr/0040-kanban-gate-version-words.md](../adr/0040-kanban-gate-version-words.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-20「残す、用語集の台帳、ゲートは削っていいと思います」）
- 覆す: [2026-11-10-adr-0013-short-role-names-and-plain-japanese.md](2026-11-10-adr-0013-short-role-names-and-plain-japanese.md) のうち、もともと日本語で言う語の例に「台帳」を挙げた所（一部）

Andon の文から「台帳」「ゲート」「版」の 3 語を外す。「台帳」は kanban、「ゲート」は gate と、識別子のまま書く。「版」は「バージョン」と書き、「最新版」のように製品や文書の種類を言う形は残す。識別子は変えない。設計書 §3.5 の「日本語の呼び名は『台帳』のまま」は改める。用語集の 2 つの項は、名前を kanban と gate に替えて残す。替える範囲は repo の全部で、ADR と決定の記録も語だけ替える。3 語が戻らないことは語の検査で守る。`writing.md` の、yomiyasu と重なる 5 行は外さない（ADR-0038 決めたこと 23）。中身・理由・捨てた案は ADR が正。
