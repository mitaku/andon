# パターンの一覧は andon retro patterns が集め直した入力に照らして書き、andon retro apply が同じ期間で照らし直してから直す課題に切る

- 日付: 2026-11-19
- 出典: [ADR-0028](../adr/0028-improvement-loop.md) 決めたこと 2・8・9・10・13・14 と「実装が決めてよいこと」。形と欄の意味は [contract.md](../contract.md) §21・`schema/patterns.schema.json`

ADR-0028 が実装に任せた細部を次のとおり決めた。

- **名前**: パターンの一覧のファイルは、前の振り返りを見分ける名前（[2026-11-19-retro-collect.md](2026-11-19-retro-collect.md)）に合わせて `library/retro.json`。schema は振り返りの入力が `retro` を使っているので `patterns`。書く command は `andon retro patterns <id> [--days N] <json|->`、当てる command は `andon retro apply <id> [--dry-run]`。当てた記録は `library/retro-applied.json`。
- **書くときの照らし方**: 入力の控えを担当から受け取らず、command が同じ `--days` で集め直す（GitHub も読む）。担当が書き換えた控えで根拠の実在と新しさを通せないようにするため。集めた期間（`since`・`until`・`days`）と前の振り返りは command が `retro.json` に書く。
- **当てるときの照らし方**: `retro.json` の `until` と `days` で集め直して、同じ検査をもう一度通す。手で書き換えた `retro.json` も当てずに断れる。GitHub は読まないので、前の直しの見立ての過不足は書くときだけ確かめる。
- **前の振り返り**: 集めるとき、自分自身と、期間の終わりより後に片付いた振り返りの課題は数えない。当てる前に振り返りの課題を片付けても、新しさの検査が自分に当たらない。
- **根拠を数える単位**: 課題（レビュアーの子は親）と、人が打った文のセッションのオーナー（担当とレビュアーのセッションはその課題、Chief of Staff のセッションはセッションごと）。課題とその課題のセッションで打たれた文は1件と数える（ADR-0028 決めたこと 14 の 2 の「同じ課題の中の2回」）。
- **振り返り自身**: 直す先の `paths` が `package/skills/retro`・`internal/retro` とその下を指せば断る。名前で決めず、パスで決める。
- **Andon の repo を持たないプロジェクト**: 直す先の種類を `andon`（Andon のパッケージ）と `project`（案件の repo）に分け、`andon` で `repo` が無いかプロジェクトの `repos` に無ければ、人の課題（`who: person`）にする。Andon の repo を名前で見分けない（fork の名前は変わりうる）。
- **足す課題**: `parent` を振り返りの課題にする（前の直しを見分ける印）。`profile` と予約キー（Settings）は書かず、起動する前に Chief of Staff がほかの課題と同じ決まりで書く（ADR-0028 決めたこと 15）。依頼書に人が打った文の本文は写さない（id と役と時刻だけ）。
- **途中で落ちたとき**: 1件足すたびに記録を書き直し、打ち直すと残りから続ける。全部足したら `complete` にし、それからは断る。
- **振り返りの課題の終わり方**: repo の無い課題なので、観察の課題と同じく `andon issue report` のあと `kanban review`（`andon issue ready` は PR を確かめるので打たない）。
- **ルーチンの例**: `package/skills/retro/assets/retro.yaml`。毎朝1回（ADR-0028 決めたこと 13）、`profile` の既定は `claude`。
- **形式モデル**: 課題を足すだけで、課題の一生の動きは変えない。`model/` は変えない。
