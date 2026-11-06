# ADR-0005: 建屋（セル）の repo の直下と、追跡・持ち運び・守り（仕様・設計。2026-11-06。タスク Y-161）

- 状態: **提案（2026-11-06）。人がこの PR をマージしたら決定**（マージの日が決定の日。そのとき この行を `決定（日付）` にし、[decisions.md](../decisions.md) §14 に1行足す）。実装は「依頼書に切る案」のとおり別タスク。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 建屋の repo の直下に何を予約名として置くか、何を git で追跡するか（台帳を含む）、別のマシンへどう動かすか、人のファイルをどう守るか。`projects/<slug>/` の中は [ADR-0004](0004-project-layout.md)。
- 語は [design.md](../design.md) §0.1。**「建屋」は人がセル（cell）に改めると決めた（前提 3）**。この文書は今の語のまま書く。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「建屋自体がリポジトリになるものと考えると建屋側の構造も考えるべき」「建屋を別 PC で動かそうとしたとき kanban が消えちゃう」「複数人でワークするファクトリーじゃない」「建屋という名前も変えていいかも」。
ADR-0004 の 1 回目の提案は「台帳は git の外」で、それは bind mount が残る同じマシンの作り直ししか守らない。困りごとは、**何が建屋の repo に残るべきもので、何がそのマシンの寿命のものか**の基準が無いこと。基準を 1 本引き、直下の予約名を表にし、台帳を持ち運べるようにし、人のファイルの守りをファイルごとの線に直す。

## 前提（人と同意した順。番号）

1. **建屋の持ち主は 1 人。プロジェクトは互いに独立。** 建屋の repo は持ち主の状態の箱（private）。台帳は工場の中の工程の記録で、人と人が共有する issue の置き場ではない。repo や案件全体の issue を他の人と回すなら外部のトラッカー（GitHub Issues など）が正で、Chief of Staff がそれを扱う（書き込みは人の go。タスクには key だけ）。複数人の建屋は別の ADR。
2. **建屋は Herdr App の machine**（[ADR-0002](0002-herdr-spaces.md) 前提 8）。持ち主は全建屋を Herdr App とボードで見る（大部屋。design.md §1.7）。だから建屋が N 個あっても同じ形で読めること（schema で validate できること）が要る。
3. **語の改め（人 2026-11-06）**: 「建屋」は **セル（cell）** に改める。リーンのセル生産（必要なものを全部中に持ち、独立して回る小さな単位）の語で、前提 1・2（持ち主は 1 人・独立・同じ形・持ち運べる）がその定義そのもの。Andon＝工場（構造）、セル＝案件ごとの自己完結した単位、ライン＝セルの中の工程、大部屋＝全セルを見る面、と並ぶ。セルの repo の名前は `<slug>-cell`。置き換え（§0.1・§1.7 の系譜の表・文書・skill・command・`andon init` の雛形の repo 名）は別タスク（S6）。
4. 人のファイルの照合（contract.md §11。最後に変えた commit の author が `andon.toml` の `owner`）は、**人のファイルをエージェントが commit しない**ことで成り立つ。エージェントが commit してよい場所は表で決める（`wiki/` は既にそう。decisions.md D12・問い21）。守りは「騙す担当」向けではない（D1・D3）。
5. 建屋の repo は git repo（`andon init` が作る）。自動 push は捨てた（問い10。2026-11-05）が、その理由「建屋の repo の commit は稀」はこの文書で変わる（決めたこと 3）。
6. 先輩プロジェクト shepherdr は「Work never moves between machines」「Nothing carries over」で、工場の持ち運びを一度も持たなかった（出典）。Andon は建屋が git repo なのでそれを得られる。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-06 追跡の基準は 1 本: 「建屋を別のマシンに建て直すとき要るか」。** 要る＝建屋の repo で追跡する。要らない（そのマシンの寿命のもの: 端末・worktree・lock・一時の状態・生成し直せるもの）＝追跡しない。「人のものか、エージェントのものか」は別の列（書く人）で持ち、追跡と混ぜない。
   理由: 建屋は持ち主 1 人の箱（前提 1）なので、追跡の目的は人の差分の見やすさでも共同作業でもなく、**持ち運びと建て直し**だけ。
   捨てた案: ADR-0004 の 1 回目「追跡＝人のもの・外＝エージェントのもの」——台帳が git の外になり、別のマシンへ動かすと計画と `log.jsonl` が消える（人の指摘）。

2. **2026-11-06 建屋の repo の直下。予約名は次の表、その他は自由（持ち主のもの。追跡が既定）。** エージェントは予約名の中で決められた場所にだけ書き、直下に新しい名前を作らない。

   | 名前 | 中身 | 書く人 | 追跡 | Herdr の単位 |
   |---|---|---|---|---|
   | `andon.toml` | 建屋の設定（`owner`・版・自動 push の切り替え〈3〉） | **人** | する | machine |
   | `AGENTS.md`・`AGENTS.local.md` | 建屋の指針（薄く。行数の上限は ADR-0004 の lint と同じ）。`.local` は boot が生成する環境メモ | 人／boot | する／しない | — |
   | `gate.local.json`・`templates.local/`・`local/`・`bin/` | ゲートの上書き・プロファイルの上書き・備品・`bootstrap` | **人** | する | — |
   | `projects/<slug>/` | プロジェクト（ADR-0004）。指揮側の Space | ADR-0004 の表 | する（台帳を含む。3） | space（指揮側） |
   | `wiki/` | 建屋を横断する知識。1 ファイル 1 件・日付・出典・痩せた `index.md`。プロジェクト固有の事実も index の節か front matter の印で**ここ**に置く（プロジェクトに wiki は作らない） | Chief of Staff（D12） | する | — |
   | `files/` | 納品物（人に URL で届けるもの。`andon publish` が `<日付>-<slug>` で置き、files サーバが公開する）。**今の `artifacts/` の改名** | `andon publish`（担当・Chief of Staff・人） | する（大きいものは開いている問い 1） | — |
   | `.env`・`.env.*`・`.ssh/`・`.andon/`・`.ghq/`・`.codex/` | 秘密・番号・鍵・パッケージの取り出し・迎えた repo・ハーネスの状態 | 人／boot | しない（`init/.gitignore` のまま） | — |
   | worktree・端末・伝令の控え | `~/.herdr/worktrees/`・hc・`~/.local/state/andon/` | 機械 | 建屋の外 | space（作業側）・pane |

   理由: 直下の既存の雛形（`init/`）を変えず、`wiki/`・`files/` の 2 つを「建て直しに要る」側に置いただけ。`artifacts` → `files` は「生成物」の語感を消し、納品物の面（files サーバのルート）と名前を揃える。
   捨てた案: `artifacts/` のまま——`.gitignore` 済みで、建て直すと納品物が消える／`projects/<slug>/wiki/`——段が増えると「どこに書くか」の迷いが増え、1 プロジェクトの建屋で空の段ができる。段は「製品 repo の `docs/` → 建屋の `wiki/` → 持ち主の wiki（建屋の外）」の 3 つで足りる／建屋ルートに `.claude/`——開いている問い 4。

3. **2026-11-06 台帳 `kanban/` は追跡する（`.lock`・`state/`・一時ファイルを除く）。commit と push は伝令がする。自動 push の決定（問い10）を、前提が変わった分だけ決め直す。** 伝令は 1 巡の終わりに `kanban/`（と `files/`・`wiki/`）に差分があれば 1 commit（メッセージは機械的。例 `kanban: <slug> <日付> <変えたタスクの id>`）し、`andon.toml` で切れる既定 on の push をする。
   - 履歴の正は `log.jsonl`（追記だけ。`by` つき）で、git の履歴は**運ぶための箱**。読みやすさは要らない（前提 1）。
   - 書き手は建屋に同時に 1 つ（指揮側。「1 つの home に書き手は 1 つ」）なので conflict は起きない。
   - **マシン固有の値**（`config.yaml` の `coordinator`、`task.yaml` の `agent`・`worktree`・`machine`）は追跡されるが、別のマシンでは無効。動かした先で端末が無いタスクは伝令が `stalled` として知らせ、起こし直しの対象にする（Y-155 の「シェルの端末を見誤らない」と同じ扱い）。将来は `state/` に分けて `config.yaml` から外す（台帳の形式の版上げ。K6）。
   - **動かす手順**（operate.md の「引っ越し」の段に書く）: 古い側で伝令を止める → commit・push → 新しい側で clone → `andon init` が既存の台帳を見つける → worktree は PR の branch から戻す（design.md §5.4）→ `doing` の担当を起こし直す。
   - `kanban init` が書く `kanban/.gitignore` を `*` から「`.lock`・`state/`・`*.tmp-*`」に変える（ledger.md の `.gitignore` の節と「not kept in git」の文も直す。Y-141）。
   理由: 問い10 の「捨てる」は「建屋の repo の commit は稀」が前提だった。台帳を追跡すると稀でなくなる。元の推奨（残す・`andon.toml` で切れる）に戻るだけで、仕組みは仮設の post-commit hook ではなく伝令の 1 段にする（止められる・ログに残る）。台帳の価値は「計画はコードのそば、履歴は `log.jsonl`」で、shepherdr が持ち越さなかったものを Andon は持ち越す。
   捨てた案: 人が commit する——忘れると消える／台帳だけ別 repo・orphan branch——建屋＝1 repo で建て直せる単純さを壊す／rsync で運ぶ——壊れたときに残らない／追跡しない（1 回目）——指摘のとおり消える。

4. **2026-11-06 人のファイルとエージェントの境界と守り。** 人のファイル（人だけが書く）は `andon.toml`・`projects/<slug>/project.json`・`gate.local.json`・`templates.local/`・`local/`・建屋の `AGENTS.md`。エージェントが commit してよいのは `wiki/`・`files/`・`projects/<slug>/AGENTS.md`・`projects/<slug>/.claude/`・`kanban/`（表の「書く人」のとおり）。守りは 3 段:
   - (a) **機械が読む人のファイル**（`project.json`・`gate.local.json`）は、建屋の repo の HEAD を読み、最後に変えた commit の author が `owner` のときだけ信じる（contract.md §11。今ある。持ち主が 1 人なので照合はこの 1 行で足りる）。
   - (b) **書く command は役の端末で拒む**: `andon delegate` は hc のラベル `role=` か `task=` のある端末では打てない（今ある）。
   - (c) **ハーネスの deny**: managed settings（`package/settings.json`）の `deny` に、人のファイルへの `Write`・`Edit` を足す。design.md §1.10 が「managed deny ＋ 端末のラベル」と書くが、今の `settings.json` の deny は `gh pr merge` と git の `--output` 系だけで、この deny は**まだ無い**（見つけた継ぎ目。S3）。Codex 側は同じ広さで拒めない（受け入れ済み。前提 4）。
   理由: 守りを緩められるのは人のファイルだけ、という構造にしておけば、エージェントの書く場所を増やしても（ADR-0004）線は 1 本のまま（shepherdr 1.0「repo の file は事実だけ、guard を緩められない」と同じ線）。
   捨てた案: 「人以外は建屋の repo に commit しない」（ADR-0004 の 1 回目の前提）——`wiki/` が既に例外で、台帳を追跡するなら成り立たない。ファイルごとの線に替えた。

## やらないこと（範囲の外。理由）

- **複数人の建屋**（前提 1）。人のファイルの照合を 1 人から複数へ広げるのは別の ADR。
- **台帳の形式そのものの変更**（マシン固有の値を `state/` へ分けるのは K6 の版上げで。決めたこと 3 で「将来」と書いた）。
- **`files/` をプロジェクトごとに分けること**（建屋に 1 つの表玄関で人がブックマークする形が崩れる）。
- **人のファイルを別の UID や署名で守ること**（前提 4。D1）。
- **`projects/<slug>/` の中**（ADR-0004）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **台帳の CLI（`go test ./cmd/kanban/...`・`tests/kanban.sh`）**: `kanban init` が書く `kanban/.gitignore` が `.lock`・`state/`・`*.tmp-*` だけ。
2. **建屋の雛形（`tests/andon-init.sh`）**: `andon init x` の `.gitignore` に `artifacts/` の行が無く `files/` が追跡され、`wiki/index.md` がある。`host-install`・`bootstrap --dry-run` は変わらない。
3. **パッケージ（`tests/package.sh`）**: `settings.json` の `deny` に人のファイル（`andon.toml`・`projects/*/project.json`・`gate.local.json`）への `Write`・`Edit` の規則がある。skill・template・hooks に `artifacts/` が残っていない。
4. **建屋で実測（P4 以降の決まり。報告に書く）**: 伝令の 1 巡で `kanban/` の差分が 1 commit になり push される（`andon.toml` で切ると止まる）。別のマシンで clone して `andon init` すると台帳が見つかり、`doing` のタスクが `stalled` で知らされる。担当の端末で `project.json` を `Write`・`Edit` しようとすると拒まれる（Codex は拒めないことを報告に書く）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（ADR-0004 と同じ PR） | `for f in tests/*.sh` green | — |
| S2 | `kanban/.gitignore` の書き換えと ledger.md の文（**Y-141 に乗せる**）、`git add` を含む移し方 | 受け入れ条件 1 | S1 のマージ |
| S3 | managed settings の deny（人のファイルの `Write`・`Edit`）＋ `tests/package.sh`。要実測（deny のパスの書き方） | 受け入れ条件 3 の前半 | S1 のマージ |
| S4 | 伝令の commit・push の段（`andon.toml` で切れる）＋ decisions.md に問い10 を決め直す行 ＋ operate.md の「引っ越し」の段 | 受け入れ条件 4 の commit・push | S2 |
| S5 | `artifacts/` → `files/`（`andon publish`・files サーバ・`init/.gitignore`・docs・skill） | 受け入れ条件 2・3 の後半 | S1 のマージ。S2 と独立 |
| S6 | 語の改め「建屋」→「セル（cell）」（design.md §0.1・§1.7、文書・skill・command、`andon init` の repo 名 `<slug>-cell`、decisions.md に 1 行） | `git grep 建屋` が design.md の系譜の「仮設の語」の行だけ | S1 のマージ。ほかと独立 |

## 開いている問い（人に聞く。1 問ずつ。推しつき）

1. **`files/` の大きいもの**（動画・大きい HTML）を追跡するか（判断の種類: 5）。推し: 追跡する。大きさの上限を `andon publish` が見て、超えたら拒んで理由を出す。LFS は入れない（建て直しに要らないものは納品物ではない）。
2. **push の周期**（判断の種類: 5）。推し: 1 巡ごと（差分があるときだけ）。失敗は次の巡で続きから。周期を別に設ける理由が無い。
3. **建屋ルートに `.claude/` を置くか**（判断の種類: 5）。推し: 置かない。建屋全体の設定は managed（`package/settings.json`）で配れていて、cwd `projects/<slug>/` からの重なり方は版で変わる（ADR-0004 の受け入れ条件で実測してから決める）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3〜§1.5・§1.7・§1.10・§2.2・§2.6・§3.1・§5.4・§6（問い 10・21）、[contract.md](../contract.md) §8・§11、[ledger.md](../ledger.md)（Layout・`task.yaml`・`log.jsonl`・`.gitignore`）、[operate.md](../operate.md)「建屋の置き方」・§8・§10、[decisions.md](../decisions.md) §2（D1・D12）・§3（D3）・§6（D17・D22）・§7（K6）、[ADR-0002](0002-herdr-spaces.md)、`package/settings.json`、`init/`、`internal/delegate`、`internal/publish`、`cmd/kanban/ledger.go`。
- 人との対話（2026-11-06）: 建屋自体が repo／schema で validate、`files` の名／Herdr の単位で揃える／別 PC で kanban が消える／複数人の工場ではない／台帳は工場の中、共有は外部トラッカー／その他は自由／建屋をセルに改める、repo は `<slug>-cell`。
- 先輩プロジェクト: eliasstravik/shepherdr（public。MIT）の 1.0 `settings.md`（repo の file は事実だけ）・3.x README（「Work never moves between machines」「Nothing carries over」）。世代ごとの比較は台帳の `tasks/Y-161/library/shepherdr-evolution.md`。
- 台帳の改名の依頼書（Y-141）。
