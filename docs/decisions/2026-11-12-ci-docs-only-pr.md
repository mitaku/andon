# 文書だけの PR では、CI の go と tests の job が文書を読む検査だけを走らせる

- 日付: 2026-11-12
- 出典: 人（2026-11-12）「docs配下の場合ってCIもう少し減らせそうじゃない？」・課題 A-0184

PR の変更が全部 `docs/` の下か、root の `README.md`・`CHANGELOG.md`・`AGENTS.md`・`REVIEW.md`・`SECURITY.md`・`LICENSE` のときを文書だけとする（`tests/lib/docs-scope.sh`）。そのとき go の job は文書を読む Go のテスト（`TestDocumentedInvocations`）だけ、tests の job は `tests/lib/scope.sh` が選ぶ文書を読む `tests/*.sh` だけを走らせる。表は [image.md](../image.md) の「CI の所要時間」。

ここで置いたこと:

- **job と check の名前は変えず、飛ばしても job は成功で終わる。** workflow ごと `paths` で外すと、ゲートが待つ PR の head の checks が出ない。
- **go の job でも、文書を読む Go のテストは残す。** `docs/design.md`・`README.md` に書いた `andon chief-engineer` の呼び出しを打つテストがあり、飛ばすと文書の誤りが main に入る。
- **`tests/README.md` は文書に含めない。** `tests/` の下はコードと一緒に変わることが多く、分ける得が小さい。
- **main の push と tag では必ず全部走る。** 文書とコードが混ざった PR も全部走る。
