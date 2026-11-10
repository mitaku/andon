# boot が git の既定の作者を書く。案件の repo は owner、セルの repo は owner でない作者

- 日付: 2026-11-09
- 出典: 人の依頼と決定（2026-11-09。作者はオーナーの GitHub noreply）・課題 A-0013 の範囲の問い（案 c1。人が同意）

セルの `~` は作り直すと消えるため、git の作者が無く担当・Chief of Staff の commit が止まった。boot が毎起動、`~/.gitconfig` の `includeIf` で既定の作者を書く（[image.md](../image.md) の「git の作者」）。

- 案件の repo（`$GHQ_ROOT` の下と、その worktree）は `andon.toml` の `owner`。オーナーの noreply を owner にしているセルでは、人の決定どおりオーナーの noreply になる。
- セルの repo（`/workspace`）は owner と一致しない作者（`Andon agent <andon-agent@cell.invalid>`）。委譲の照合（[contract.md](../contract.md) §11）は author の email だけで人を見分けるので、セルの repo の既定の作者を owner にすると、担当や Chief of Staff の commit が照合を素通りする。照合は変えない。
- 人が owner のファイルを commit するのは、ホストの checkout か、セルの中で作者を明示して（`GIT_AUTHOR_EMAIL` など）。セルの repo の `git config user.email` で合わせる案内（operate.md §10）はやめた（担当にも効くため）。
- `owner` が空なら、案件の repo もセルの作者で書く（commit は通し、owner を名乗らない）。
- `~/.gitconfig` に直に書かれた `user.name`・`user.email` は人の設定として触らない。

捨てた案:

- **セル全体の既定の作者を owner にする**（依頼の元の形）: 照合が素通りになる。弱まりを受け入れる案も、照合を署名などで強める案（構造の変更で別の課題）も採らない。
- **セルの repo に既定の作者を置かない**: Chief of Staff の wiki の commit が止まり、skill に作者の渡し方を足すことになる。
