# 段の estimate の通知に詰まりの兆し detail.sign を載せ、「テスト」はパスの規則で分ける

- 日付: 2026-11-13
- 出典: 課題 A-0094（[ADR-0017](../adr/0017-process-definitions.md) の切る案 P5。決めたこと 11 が「P5 で決める」とした「テスト」のパスの規則）

dispatcher は、段（見込みの 1・2・4 倍）の `estimate` の通知を積むときだけ、課題の worktree（`issue.yaml` の `worktree`）の git で詰まりの兆しを数え、`detail.sign` に載せる。形は [contract.md](../contract.md) §6 と `schema/notification.schema.json`。

ここで置いたこと:

- `no-commit`: worktree の `HEAD` の commit 時刻から、見込みの分（小 30・中 60・大 120）より長く経った。一度も commit していなければ、切った元の commit から数える。
- `large-diff`: `origin/HEAD` との merge-base から作業ツリーまでの変更（追跡しているファイル）で、足した行と消した行の和が 500 を超えた。まだ commit していない変更も数える。rename は数え分けない（`--no-renames`）。
- 「テスト」の線: パスのどこかに `tests` という名のディレクトリがあるファイルと、名前が `*_test.*` のファイル。バイナリも数えない。
- 兆しが無ければ `sign` を置かない。worktree が無い課題（repo の無い課題）や、git で読めない兆し（`origin/HEAD` が無いなど）は数えない。ネットワークには出ない（fetch しない）。
- Chief of Staff の `<news>` の1行の末尾に「兆し: no-commit・large-diff」を足す。

値は仮のまま（ADR-0017 決めたこと 11）。measure の外れ方を見てから直す。
