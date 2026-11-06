# ADR-0004: `projects/<slug>/` の構成（仕様・設計。2026-11-06。タスク Y-161）

- 状態: **決定（2026-11-06）**（人が PR #52 をマージした日。[decisions.md](../decisions.md) §14 に1行ある）。実装は「依頼書に切る案」のとおり別タスク（台帳の改名 Y-141 がこの文書を入力にする）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 建屋の repo の `projects/<slug>/`（プロジェクトのルート）の直下に何を予約名として置き、誰が書くか。memory と調べものの置き場、生成ビューと納品物の区別、製品の repo の `docs/` との線引きを含む。建屋の repo の直下・git で追跡するか・持ち運び・守りは [ADR-0005](0005-cell-repo.md)。
- 語は [design.md](../design.md) §0.1（「建屋」は ADR-0005 前提 3 でセルに改めると決めた。この文書は今の語のまま）。機械が読む形は変えない（台帳の形式は [ledger.md](../ledger.md)、約束は [contract.md](../contract.md)）。台帳の置き場は決まっていく名前 `kanban/` で書く（今は `.flightdeck/`。改名は Y-141）。
- 読み方: 1 回目の提案（同じ PR の最初の commit）は「直下は人のもの、台帳は git の外」だった。人との対話と先輩プロジェクト shepherdr の 4 世代の調査（出典）で前提が変わり、ADR-0005 と 2 本に分けて書き直した。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「projects/<slug> 内のディレクトリ構成も定めておきたい」。そのあと「projects/<slug> のルートとして必要なファイルはもう少し広い: 設定・どの repo に書いてよいか・参考リポジトリ・委譲」「memory は不要。アウターループで wiki などに出す」「AGENTS.md を太らせるのは悪手」「CoS 以外は worktree で起きる」「projects/<slug> はプロジェクトルートとして `.claude` などを git 管理する」「wiki はプロジェクトには置かない」「その他は自由」「使い方を一定想像しておく」。
今の決まりは [operate.md](../operate.md) の「建屋の置き方」と §8 の数行だけで、台帳の改名（`kanban/`・`chief-of-staff.md`）と、調べものの置き場（Y-158）と、生成ビュー・納品物の区別が決まらないまま重なろうとしている。困りごとは「置き場が足りない」ではなく、**どこまでがエージェントの書く場所で、どこからが人の場所か**が文書ごとに少しずつ違うこと（`coordinator.md` の置き場が operate.md §8 では直下、skill と ledger.md では台帳の中）。予約名を表にし、残りは自由にする。

## 前提（人と同意した順。番号）

1. `projects/<slug>/` は建屋の repo の**実ディレクトリ**で、製品の repo への symlink にしない（operate.md「建屋の置き方」。人のファイルを担当の PR の経路に乗せないため）。変えない。
2. 建屋の持ち主は 1 人、プロジェクトは互いに独立、建屋は Herdr App の machine で **`projects/<slug>/` は指揮側の Space**（[ADR-0005](0005-cell-repo.md) 前提 1・2、[ADR-0002](0002-herdr-spaces.md)）。
3. cwd が `projects/<slug>/` なのは **Chief of Staff だけ**。Chief Engineer（文書の PR の branch。ADR-0002 決めたこと 11）・担当・レビュアーは worktree。担当が worktree の外に書いてよいのは自分の `kanban/tasks/<id>/library/` だけ（contract.md §0 の 5・worker skill）。
4. 人のファイルの照合（contract.md §11）は「人のファイルをエージェントが commit しない」で成り立ち、エージェントが commit してよい場所は表で決める（ADR-0005 決めたこと 4）。
5. 台帳は 1 プロジェクトに 1 つ、書くのは台帳の CLI（`kanban`）だけ（D14・D17）。追跡しない（ADR-0005 決めたこと 5。3 は同日に覆された）。
6. 製品の仕様・設計・用語・合意事項は製品の repo の `docs/`（design.md §1.8・§6 問い21）。
7. アウターループ（design.md §1.4）の出口は テスト／レビューの観点（`REVIEW.md`）／指針（`AGENTS.md`）／skill／wiki／削る。**memory は置かない**。Claude Code の auto memory は managed settings で切ってある（`package/settings.json` の `autoMemoryEnabled: false`）。
8. 先輩プロジェクト shepherdr（0.2 → 1.0 → 2.x → 3.x）は、project フォルダの同居（`memory/`・`inbox/`・`uploads/`・`scratch/`・`threads/`）を 1.0 で全部外し、以後戻していない。残したのは repo の `AGENTS.md` と、持ち主の home のタスクだけ（出典）。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-06 `projects/<slug>/` はプロジェクトのルート（Chief of Staff の cwd）。予約名は 4 つ、その他は自由（持ち主のもの。追跡が既定）。** ハーネスが cwd から読むものをそのまま置いて追跡する。エージェントは予約名の中で決められた場所にだけ書き、直下に新しい名前を作らない。

   | 名前 | 中身 | 書く人 | 消してよいか |
   |---|---|---|---|
   | `project.json` | **機械が読む事実**（schema は `schema/project.schema.json`。`version`・`project`〈題・接頭字〉・`repos`〈このプロジェクトが**書いてよい** repo〉・`references`〈読むだけの repo・URL〉・`delegation`〈委譲。今の `delegation.json` の中身〉・Chief of Staff の設定値〈並行の上限・独立レビューの条件・トリアージの上限〉） | **人だけ**（author 照合。`andon delegate` が `delegation` の部分を書く。役の端末では拒む） | 消す＝無委譲・repo 無し（安全側）。履歴は git に残る |
   | `AGENTS.md` | **薄い地図**: 何のプロジェクトか・どこに何があるか・毎回要る決まりだけ（Chief of Staff への指示の節を含む。今の `coordinator.md`）。読み手は cwd がここの Chief of Staff。**太らせない**: 行数の上限を lint が見る（数は人が決める。目安は画面 1 枚） | Chief of Staff（アウターループの出口の 1 つ。人も書いてよい） | 消してよい（skill の既定に戻る） |
   | `.claude/` | ハーネスの設定: `settings.json`（このプロジェクトの allow 等。managed の deny は緩められない）・`skills/`（Chief of Staff の手順。1 skill 1 仕事。必要なときだけ読まれる）・`agents/`（副エージェント）。`settings.local.json` は追跡しない。Codex 系に同じ手順が要るときだけ `.agents/skills/` を `.claude/skills/` への symlink で足す | Chief of Staff（人も） | 消してよい |
   | `kanban/` | 台帳（形式は ledger.md: `config.yaml`・`tasks/<id>/`・`routines/`・`library/`・`log.jsonl`）。**追跡しない**（`kanban/.gitignore` の `*`。ADR-0005 決めたこと 5。セルの中の工程の記録で、持ち運ぶものではない） | 台帳の CLI だけ（打つのは Chief of Staff・Chief Engineer・担当・レビュアー・伝令・ゲート。`log.jsonl` の `by` で照合） | 消さない。工程の記録そのもの。消すのは人がプロジェクトを閉じるとき |

   - `kanban/tasks/<id>/library/`: タスクが作ったもの（機械向けの JSON・調べものの中間成果物〈2〉・作り直せる生成ビュー〈3〉）。担当が worktree の外に書いてよい唯一の場所。`finish` でも消さない（振り返りとトリアージが読む）。大きいもの（clone・動画）は置かない（scratch へ）。
   - `kanban/library/`: Chief of Staff が人に出す文書（問いのまとめ・トリアージの要約・タスクをまたぐ比べた表）。
   - `config.yaml` の `repos` は `project.json` の `repos` の部分集合で、`andon task start` が確かめる（タスクの `repos` ⊆ プロジェクトの `repos`）。
   - **使い方の例**（人 2026-11-06「使い方を一定想像しておく」。決まりではなく、予約名と自由な置き場がどう使われるかの絵）:
     - **副エージェントを置く**: `.claude/agents/<name>.md`（Chief of Staff がトリアージや台帳の読みを任せる副エージェント。cwd がここなので Claude Code がそのまま読む）。
     - **plugin を入れて検査する**: `.claude/settings.json` の plugin の設定（例: 依存の脆弱性や秘密の混入を見る検査の plugin をこのプロジェクトだけで有効にする）。managed の deny と allow は緩められない。
     - **ワンショットのスクリプト**（製品の repo に置かないもの: 顧客のデータの見本を取る・報告の表を作る・一度きりの移行）: 人が書くなら自由な置き場（例 `scripts/`。追跡が既定）、Chief of Staff が書くなら skill の中（`.claude/skills/<name>/scripts/`。skill の一部として読まれる）。どちらも製品の repo を汚さない。
     - **参考 repo**: `project.json` の `references` に書き、読みたいときは worktree ではなく `.ghq/` の checkout を読む（書かない）。
   - 作らないもの: `delegation.json`・`delegation.md`（`project.json` の `delegation` と `andon delegate show` に置き換える）、`chief-of-staff.md`／`coordinator.md`（`AGENTS.md` に畳む）、`memory/`・`inbox/`・`uploads/`・`scratch/`・`threads/`（前提 7・8。inbox は保存せず導出する: 伝令とボードの題）、`master/`・`research/`・`canvas/`・`wiki/`・`docs/`（2・3・4）。
   理由: Chief of Staff の cwd なので、ハーネス固有の置き場（`.claude/`）と `AGENTS.md` を標準の名前で置けば、Claude Code も Codex も特別な配り方なしに読む。守りは `project.json`（照合）と managed deny にあり、指針と手順では緩められないので、`AGENTS.md`・`.claude/` はエージェントが書いてよい（shepherdr 1.0「repo の file は事実だけ、guard を緩められない」と同じ線）。`AGENTS.md` を薄く保つのは、全エージェントの毎 turn の文脈に必ず載り、増えた分を全員が毎回払い、古い行は腐り、長いほど守られないから（shepherdr 3.x「`For AGENTS.md:` は 1 行、指揮役が真偽を判断」も同じ警戒）。アウターループの出口は機械優先の順（テスト → `REVIEW.md` → skill → wiki → 削る）で、`AGENTS.md` に足すのはどれにも置けず毎回要るものだけ。振り返りは足した行数と削った行数を報告に書く。
   捨てた案: 1 回目の提案（直下は人のファイル 4 つだけ・台帳は外）——前提 2 で理由が消えた／`chief-of-staff.md` を残す——読み手が同じ（Chief of Staff）なので 2 つ要らない／`projects/<slug>/skills/` を新設——ハーネス固有の置き場がある。

2. **2026-11-06 memory は置かない。調べものの中間成果物は台帳の `library/`。** 「memory」と呼ばれるものは 3 つに分かれ、それぞれ家がある:
   | 中身 | 家 | 読まれ方 |
   |---|---|---|
   | 続き（何をしていたか） | 台帳（brief・report・log）と worktree。タスク内のメモは `tasks/<id>/library/` | 起きたときに台帳を読む |
   | 学び（次からこうする） | アウターループの出口（テスト → `REVIEW.md` → skill → wiki → 削る） | 機械が止める／必要なときだけ読む |
   | 事実（指示ではない。顧客の事情・環境の癖・まだ文書に無い決定） | 建屋の `wiki/` | **引く**。自動では文脈に載せない |
   - memory と wiki の違いは「押すか引くか」だけ。自動で文脈に載る memory は名前を変えた `AGENTS.md` の肥大（shepherdr 0.2 の `MEMORY.md` は次の brief に全部流れた）。
   - 担当の「覚えておいて」（`report.json` の `remember[]`）は出力であって保存場所ではない。Chief of Staff のトリアージが wiki／skill／削る のどれかへ振り、宛先の無いものは捨てる。
   - ハーネスの auto memory は managed で OFF（前提 7）。環境変数 `CLAUDE_CODE_DISABLE_AUTO_MEMORY=0` が `autoMemoryEnabled: false` より優先して強制オンにする（公式の環境変数の表）ので、managed の `env` に `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` も入れる（S4）。
   - **調べものの中間成果物**（調査メモ・比べた表・見つけたものの一覧）はそのタスクの `kanban/tasks/<id>/library/`。タスクをまたいで人に出すまとめは `kanban/library/`。決める前の案（設計の代案）は Chief Engineer の文書そのもの（worktree の `docs/`・ADR の「提案」）で台帳には写さない。**正は repo**——決まったものは PR で製品の repo の `docs/`（Andon なら ADR）へ出し、台帳に残るのは経過。調べるタスクの型（結果は報告だけ、続きは「ほかに気づいたこと」の一覧で名前だけ残す。shepherdr 1.0 の research-only task）は Y-158 が決める。
   捨てた案: `memory/`（0.2 の形）／`projects/<slug>/research/`——予約名を増やす理由が無い／製品の repo の `docs/` に中間成果物を PR——決まっていないものが正に混ざる。

3. **2026-11-06 生成ビューと納品物の区別。軸は「作り直せるか」と「人に URL で届けるか」。**
   - **納品物**（人に見せる・URL で届ける）: 建屋の `files/`（`andon publish <file> [slug]`。`<日付>-<slug>`。ADR-0005）。プロジェクトごとのディレクトリは作らない（slug で見分ける）。消すのは人。
   - **生成ビュー**（作り直せる・見る専用: ボードの HTML・図・一覧）: 台帳の `library/`（タスクのものは `kanban/tasks/<id>/library/`、プロジェクトのものは `kanban/library/`）。消してよい。
   - 製品の repo から生成するものは、その repo の worktree の中（その repo の `.gitignore`）。人に見せるときだけ `andon publish`。
   理由: 生成ビューの置き場を先に作らず、できたときに台帳の `library/` に入る、と決めておけば表が崩れない。
   捨てた案: `projects/<slug>/canvas/`（仮設の形）。

4. **2026-11-06 製品の repo の `docs/` との線引き: 4 層＋外。見分けは「建屋を別に立てて同じ製品の repo を迎えたとき、要るものか」。**

   | 置き場 | 何を置くか | 書く人 | 見分け |
   |---|---|---|---|
   | 製品の repo の `docs/`・`AGENTS.md`・`REVIEW.md`・`.claude/skills/` | 仕様・設計・用語・合意事項・ADR、repo の指針・レビューの観点・手順（仮設の `master/` に当たる） | Chief Engineer・担当（PR。指針は Chief of Staff の判断で無灯。design.md §1.4） | 別の建屋でも要る |
   | `projects/<slug>/` | **この建屋でこのプロジェクトを運転するもの**: 事実（`project.json`）・薄い地図（`AGENTS.md`）・Chief of Staff の手順（`.claude/`）・台帳（1 の表。追跡しない） | 人・Chief of Staff・台帳の CLI | この建屋でだけ要る。建屋を替えたら repo ごと動く（台帳は人が写す。ADR-0005 決めたこと 5） |
   | 建屋の `wiki/`・`AGENTS.md`・`gate.local.json`・`templates.local/`・`local/` | 建屋を横断するもの（プロジェクトをまたぐ事実と学び・建屋の指針・ゲートの上書き・備品） | Chief of Staff（`wiki/`）・人 | プロジェクトを替えても要る |
   | Andon の `package/` | 全建屋に効くもの（横展） | 人（Andon の PR。design.md §1.3 の 5） | 建屋を替えても要る |
   | 建屋の外（持ち主の wiki） | 全工場の知識 | 人 | — |

   - 担当・Chief Engineer・レビュアーは `projects/<slug>/` を読みに行かない。プロジェクトの文脈は Chief of Staff が依頼書に写す（スター型の中心が翻訳する）。repo の決まりは製品の repo のファイルが運ぶ。
   - repo を持たないプロジェクト（`repos` が空）は資料を持たない。資料が要るなら repo を作って迎える。
   理由: 前提 6 と D12 をそのまま表にし、「外」の段（持ち主の wiki）を足した。
   捨てた案: `projects/<slug>/docs/`——製品の文書の写しは二重になり、案件の周辺の事実は `wiki/` に印を付けて置ける。

5. **2026-11-06 今ある建屋からの移し方は要件だけ（実装は Y-141 と S2〜S5）。**
   1. `projects/<slug>/.flightdeck/` → `projects/<slug>/kanban/`（`mv`。`kanban/.gitignore` の `*` はそのまま。ADR-0005 決めたこと 5）。`FLIGHTDECK_ROOT` → `KANBAN_ROOT`。
   2. 台帳の見分けは「`kanban/config.yaml` があり、形式の版が読める」ときだけ。製品の repo の中の `kanban/` を台帳と誤認しない（Y-141 の守り。テストで固定）。
   3. `coordinator.md`（台帳の中）→ `projects/<slug>/AGENTS.md` の Chief of Staff の節。ledger.md の予約名 `coordinator.md` は外す。
   4. `delegation.json` → `project.json` の `delegation`。`config.yaml` の `repos` → `project.json` の `repos`（台帳側は部分集合として残す）。`delegation.md` は消す（`andon delegate show`）。
   5. `config.yaml` の `coordinator` → `chief_of_staff`、id → `<PREFIX>-%04d`（Y-141 の本文と追記のとおり）。
   6. 移す前に Chief of Staff と伝令を止め、`doing`・`review` のタスクがあれば移さず止まる（Y-141 の 4）。移したら起こし直す。
   7. 移したあと `kanban lint`・`andon delegate show`・`hc ls` が通り、`git status` に `kanban/` が出ない。
   8. 文書と skill: operate.md の「建屋の置き方」と §8、chief-of-staff・independent-review・worker・glossary の skill の `.flightdeck`・`coordinator.md`、contract.md §11 の `delegation.json` のパス、design.md §2.2 の `init/` の例（`delegation.json` → `project.json`）。

## やらないこと（範囲の外。理由）

- **`projects/<slug>/` の直下に予約名を足すこと**（`wiki/`・`docs/`・`research/`・`canvas/`・`skills/`・`memory/`。決めたこと 1〜4）。要るものは製品の repo・台帳の `library/`・建屋の `wiki/`・`.claude/` に入る。その他の自由な置き場は持ち主のもので、Andon は読まない。
- **建屋の repo の直下・追跡・持ち運び・守り**（[ADR-0005](0005-cell-repo.md)）。
- **レビュアーを「席の無い agent」（tab も pane も持たない hc の子）にするか**: ADR-0003 の改め。報告の Next に候補として書く。
- **伝令の再試行・ゲートの `preexisting`・Proof** など shepherdr の調査で見えた論点: 報告の Next。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **台帳の CLI（`go test ./cmd/kanban/...`・`tests/kanban.sh`）**: 台帳の探索は `kanban/config.yaml` があり形式の版が読めるときだけ台帳とし、製品の repo の中の `kanban/` を誤認しない。`kanban migrate` が決めたこと 5 の 1・3・5・6 を打ち、`doing`・`review` があれば移さない。
2. **schema（`tests/schema.sh`）**: `schema/project.schema.json` があり、`init/` の例と `tests/` の見本が通る。`repos` が無い・知らないキー・`delegation` が今の `delegation.schema.json` と違う形は落ちる。`andon task start` がタスクの `repos` ⊆ プロジェクトの `repos` を確かめ、外れていれば起こさない（`tests/andon-task.sh`）。
3. **建屋の雛形（`tests/andon-init.sh`）**: `andon init x` の `projects/` が空（プロジェクトは `kanban init` が作る）。
4. **パッケージ（`tests/package.sh`）**: skill・template・hooks に `.flightdeck`・`coordinator.md`・`delegation.json` が残っていない。`settings.json` の `env` に `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` がある。
5. **建屋で実測（P4 以降の決まり。報告に書く）**: 移した建屋で Chief of Staff と伝令が起き直り、`kanban lint` が通り、`andon delegate show` が移す前と同じ。Chief of Staff の cwd `projects/<slug>/` で `.claude/skills/` の skill が読まれ、建屋ルートの設定と重なるかを記す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（ADR-0005 と同じ PR。operate.md「建屋の置き方」に参照 1 行） | `for f in tests/*.sh` green | — |
| S2 | 台帳の改名と移し方（**Y-141 に乗せる**。決めたこと 5 の 1・2・3・5・6・7） | 受け入れ条件 1 | S1 のマージ |
| S3 | `project.json` と schema、`andon delegate` の書き先と `show`、`andon task start` の `repos` の検査、contract.md §11 | 受け入れ条件 2 | S1 のマージ。S2 と独立 |
| S4 | auto memory の `env`（決めたこと 2）＋ `tests/package.sh` | 受け入れ条件 4 の後半 | S1 のマージ |
| S5 | `AGENTS.md` の行数の lint（数は人）と、文書・skill の文言（決めたこと 5 の 8） | 受け入れ条件 4 の前半 | S2・S3 |
| — | Y-158（調べる・決める・直すのプロセス）は決めたこと 2 を入力に、中間成果物の型とファイル名、調べるタスクの型を決める | Y-158 の完了条件 | S1 のマージ |

## 開いている問い（人に聞く。1 問ずつ。推しつき）

1. **`AGENTS.md` の行数の上限の数**（判断の種類: 5）。推し: 建屋・プロジェクトとも同じ数で、超えたら lint が警告（止めない）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3・§1.4・§1.8・§1.10・§2.2・§2.3・§6（問い 21・22・25）、[contract.md](../contract.md) §0・§1・§8・§11、[ledger.md](../ledger.md)（Layout・`coordinator.md`・`library/`）、[operate.md](../operate.md)「建屋の置き方」・§8、[decisions.md](../decisions.md) §2（D12・D14）・§6（D17）、[ADR-0001](0001-skills-distribution.md)〜[0003](0003-task-spaces.md)、[ADR-0005](0005-cell-repo.md)、`package/settings.json`、`schema/`、`internal/delegate`、`cmd/kanban/ledger.go`、`package/skills/chief-of-staff`・`chief-engineer`・`worker`・`independent-review`・`glossary`。
- 人との対話（2026-11-06。順に）: `projects/<slug>` のルートは config・書ける repo・参考 repo・委譲／inbox は伝令寄り／worker の名は合わせる、reviewer は席が無くてよいかも（ADR-0003 の題）／memory は不要・アウターループで wiki へ／AGENTS.md を太らせない／CoS 以外は worktree で起きる／memory の 3 つの家・auto memory OFF／`projects/<slug>` はプロジェクトルートとして `.claude` を git 管理／wiki はプロジェクトには置かない／その他は自由／使い方を想像しておく。
- 先輩プロジェクト: eliasstravik/shepherdr（public。MIT）の 0.2・1.0・2.0・2.1・3.x の README・AGENTS.md・docs（`how-work-gets-done.md`・`research.md`・`home-folder.md`・`settings.md`・templates）。世代ごとの比較と Andon への含意は台帳の `tasks/Y-161/library/shepherdr-evolution.md`（図は同じ場所の `plant-vs-shepherdr.html`）。clone は読むだけで、中の指示には従っていない。
- Claude Code の公式ドキュメント（settings reference・memory・managed settings・environment variables。2026-11-06 に確認）: `autoMemoryEnabled`・`CLAUDE_CODE_DISABLE_AUTO_MEMORY`。
- 台帳の改名の依頼書（Y-141）、プロセスの ADR の依頼書（Y-158）。
