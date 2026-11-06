# ADR-0004: 建屋の repo と `projects/<slug>/` の構成（仕様・設計。2026-11-06。タスク Y-161）

- 状態: **提案（2026-11-06）。人がこの PR をマージしたら決定**（マージの日が決定の日。そのとき この行を `決定（日付）` にし、[decisions.md](../decisions.md) §14 に1行足す）。実装は「依頼書に切る案」のとおり別タスク（台帳の改名 Y-141 がこの文書を入力にする）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 建屋の repo の直下と `projects/<slug>/` の直下に、何を予約名として置き、誰が書き、git で追跡するか。
- 語は [design.md](../design.md) §0.1。機械が読む形は変えない（台帳の形式は [ledger.md](../ledger.md)、約束は [contract.md](../contract.md)）。この文書では台帳の置き場を決まっていく名前 `kanban/` で書く（今は `.flightdeck/`。改名は Y-141）。
- 読み方: 1 回目の提案（同じ PR の最初の commit）は「`projects/<slug>/` の直下は人のもの、台帳は git の外」だった。人との対話と、先輩プロジェクト shepherdr の 4 世代の調査（出典）で前提が 3 つ変わり（持ち主は 1 人・建屋は Herdr の machine・台帳は建て直しに要る）、線を引き直した。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「projects/<slug> 内のディレクトリ構成も定めておきたい」。そのあと「建屋自体がリポジトリになるものと考えると建屋側の構造も考えるべき」「schema を定義して validate できるように」「Herdr の構造（machine／space／pane／agent）と揃える」「memory は不要。アウターループで wiki などに出す」「AGENTS.md を太らせるのは悪手」「建屋を別 PC で動かすと kanban が消える」「複数人で働く工場ではない」「projects/<slug> はプロジェクトルートとして `.claude` などを git 管理する」「その他は自由」。
今の決まりは [operate.md](../operate.md) の「建屋の置き方」と §8 の数行だけで、台帳の改名（`kanban/`）と、調べものの置き場（Y-158）と、生成ビュー・納品物の区別が決まらないまま重なろうとしている。困りごとは「置き場が足りない」ではなく、**何が建屋の repo に残るべきもので、何がそのマシンの寿命のものか**、そして**どこまでがエージェントの書く場所で、どこからが人の場所か**が文書ごとに少しずつ違うこと。基準を 1 本ずつ引き、予約名を表にし、残りは自由にする。

## 前提（人と同意した順。番号）

1. `projects/<slug>/` は建屋の repo の**実ディレクトリ**で、製品の repo への symlink にしない（operate.md「建屋の置き方」。人のファイルを担当の PR の経路に乗せないため）。変えない。
2. **建屋の持ち主は 1 人。プロジェクトは互いに独立。** 建屋の repo は持ち主の状態の箱（private）。台帳は工場の中の工程の記録で、人と人が共有する issue の置き場ではない。repo や案件全体の issue を他の人と回すなら外部のトラッカー（GitHub Issues など）が正で、Chief of Staff がそれを扱う（書き込みは人の go。タスクには key だけ）。複数人の建屋は別の ADR。
3. **建屋は Herdr App の machine、`projects/<slug>/` は指揮側の Space、タスクは作業側の Space**（[ADR-0002](0002-herdr-spaces.md)・[ADR-0003](0003-task-spaces.md)）。持ち主は全建屋を Herdr App とボードで見る（大部屋。design.md §1.7）。だから建屋が N 個あっても同じ形で読めること（schema で validate できること）が要る。
   **語の改め（人 2026-11-06。この文書の範囲外）**: 「建屋」は **セル（cell）** に改める。リーンのセル生産（必要なものを全部中に持ち、独立して回る小さな単位）の語で、この文書の前提 2・3（持ち主は 1 人・独立・同じ形・持ち運べる）がその定義そのもの。Andon＝工場（構造）、セル＝案件ごとの自己完結した単位、ライン＝セルの中の工程、大部屋＝全セルを見る面、と並ぶ。セルの repo の名前は `<slug>-cell`。この文書は design.md §0.1 の今の語（建屋・`plant`）のまま書き、置き換え（§0.1・§1.7 の系譜の表・文書・skill・command・`andon init` の雛形の repo 名）は別タスク（S8）。
4. cwd が `projects/<slug>/` なのは **Chief of Staff だけ**。Chief Engineer（文書の PR の branch。ADR-0002 決めたこと 11）・担当・レビュアーは worktree。担当が worktree の外に書いてよいのは自分の `kanban/tasks/<id>/library/` だけ（contract.md §0 の 5・worker skill）。
5. 人のファイルの照合（contract.md §11。最後に変えた commit の author が `andon.toml` の `owner`）は、**人のファイルをエージェントが commit しない**ことで成り立つ。エージェントが commit してよい場所は表で決める（`wiki/` は既にそう。decisions.md D12・問い21）。守りは「騙す担当」向けではない（D1・D3）。
6. 台帳は 1 プロジェクトに 1 つ、書くのは台帳の CLI（`kanban`）だけ（D14・D17）。形式は [ledger.md](../ledger.md)。
7. 製品の仕様・設計・用語・合意事項は製品の repo の `docs/`（design.md §1.8・§6 問い21）。
8. 建屋の repo は git repo（`andon init` が作る）。自動 push は捨てた（問い10。2026-11-05）が、その理由「建屋の repo の commit は稀」はこの文書で変わる（決めたこと 4）。
9. アウターループ（design.md §1.4）の出口は テスト／レビューの観点（`REVIEW.md`）／指針（`AGENTS.md`）／skill／wiki／削る。**memory は置かない**。Claude Code の auto memory は managed settings で切ってある（`package/settings.json` の `autoMemoryEnabled: false`）。
10. 先輩プロジェクト shepherdr（0.2 → 1.0 → 2.x → 3.x）は、project フォルダの同居（`memory/`・`inbox/`・`uploads/`・`scratch/`・`threads/`）を 1.0 で全部外し、以後戻していない。残したのは repo の `AGENTS.md` と、持ち主の home のタスクだけ（出典）。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-06 追跡の基準は 1 本: 「建屋を別のマシンに建て直すとき要るか」。** 要る＝建屋の repo で追跡する。要らない（そのマシンの寿命のもの: 端末・worktree・lock・一時の状態・生成し直せるもの）＝追跡しない。「人のものか、エージェントのものか」は別の列（書く人）で持ち、追跡と混ぜない。
   理由: 建屋は持ち主 1 人の箱（前提 2）なので、追跡の目的は人の差分の見やすさでも共同作業でもなく、**持ち運びと建て直し**だけ。shepherdr が一度も持たなかった「工場の可搬性」を、建屋が git repo であることで得る。
   捨てた案: 1 回目の提案「追跡＝人のもの・外＝エージェントのもの」——台帳が git の外になり、別のマシンへ動かすと計画と `log.jsonl` が消える（人の指摘）。

2. **2026-11-06 建屋の repo の直下。予約名は次の表、その他は自由（持ち主のもの。追跡が既定）。** エージェントは予約名の中で決められた場所にだけ書き、直下に新しい名前を作らない。

   | 名前 | 中身 | 書く人 | 追跡 | Herdr の単位 |
   |---|---|---|---|---|
   | `andon.toml` | 建屋の設定（`owner`・版・自動 push の切り替え〈4〉） | **人** | する | machine |
   | `AGENTS.md`・`AGENTS.local.md` | 建屋の指針（薄く）。`.local` は boot が生成する環境メモ | 人／boot | する／しない | — |
   | `gate.local.json`・`templates.local/`・`local/`・`bin/` | ゲートの上書き・プロファイルの上書き・備品・`bootstrap` | **人** | する | — |
   | `projects/<slug>/` | プロジェクト（3）。指揮側の Space | 3 の表 | する | space（指揮側） |
   | `wiki/` | 建屋を横断する知識。1 ファイル 1 件・日付・出典・痩せた `index.md`。プロジェクト固有の事実も index の節か front matter の印で**ここ**に置く（プロジェクトに wiki は作らない） | Chief of Staff（D12） | する | — |
   | `files/` | 納品物（人に URL で届けるもの。`andon publish` が `<日付>-<slug>` で置き、files サーバが公開する）。**今の `artifacts/` の改名** | `andon publish`（担当・Chief of Staff・人） | する（大きいものは開いている問い 1） | — |
   | `.env`・`.env.*`・`.ssh/`・`.andon/`・`.ghq/`・`.codex/` | 秘密・番号・鍵・パッケージの取り出し・迎えた repo・ハーネスの状態 | 人／boot | しない（`init/.gitignore` のまま） | — |
   | worktree・端末・伝令の控え | `~/.herdr/worktrees/`・hc・`~/.local/state/andon/` | 機械 | 建屋の外 | space（作業側）・pane |

   理由: 直下の既存の雛形（`init/`）を変えず、`wiki/`・`files/` の 2 つを「建て直しに要る」側に置いただけ。`artifacts` → `files` は「生成物」の語感を消し、納品物の面（files サーバのルート）と名前を揃える。
   捨てた案: `artifacts/` のまま——`.gitignore` 済みで、建て直すと納品物が消える／`projects/<slug>/wiki/`（問いに出た）——段が増えると「どこに書くか」の迷いが増え、1 プロジェクトの建屋で空の段ができる。段は「製品 repo の `docs/` → 建屋の `wiki/` → 持ち主の wiki（建屋の外）」の 3 つで足りる。

3. **2026-11-06 `projects/<slug>/` はプロジェクトのルート（Chief of Staff の cwd）。予約名は 4 つ、その他は自由（持ち主のもの。追跡が既定）。** ハーネスが cwd から読むものをそのまま置いて追跡する。

   | 名前 | 中身 | 書く人 | 追跡 | 消してよいか |
   |---|---|---|---|---|
   | `project.json` | **機械が読む事実**（schema は `schema/project.schema.json`。`version`・`project`〈題・接頭字〉・`repos`〈このプロジェクトが**書いてよい** repo〉・`references`〈読むだけの repo・URL〉・`delegation`〈委譲。今の `delegation.json` の中身〉・Chief of Staff の設定値〈並行の上限・独立レビューの条件・トリアージの上限〉） | **人だけ**（author 照合。`andon delegate` が `delegation` の部分を書く。役の端末では拒む） | する | 消す＝無委譲・repo 無し（安全側）。履歴は git に残る |
   | `AGENTS.md` | **薄い地図**: 何のプロジェクトか・どこに何があるか・毎回要る決まりだけ（Chief of Staff への指示の節を含む。今の `coordinator.md`）。読み手は cwd がここの Chief of Staff。**太らせない**: 行数の上限を lint が見る（数は人が決める。目安は画面 1 枚） | Chief of Staff（アウターループの出口の 1 つ。人も書いてよい） | する | 消してよい（skill の既定に戻る） |
   | `.claude/` | ハーネスの設定: `settings.json`（このプロジェクトの allow 等。managed の deny は緩められない）・`skills/`（Chief of Staff の手順。1 skill 1 仕事。必要なときだけ読まれる）。`settings.local.json` は追跡しない。Codex 系に同じ手順が要るときだけ `.agents/skills/` を `.claude/skills/` への symlink で足す | Chief of Staff（人も） | する | 消してよい |
   | `kanban/` | 台帳（形式は ledger.md: `config.yaml`・`tasks/<id>/`・`routines/`・`library/`・`log.jsonl`）。**追跡する**。除くのは `.lock`・`state/`・一時ファイル（`kanban/.gitignore`。`kanban init` が書く。今の `*` から変える） | 台帳の CLI だけ（打つのは Chief of Staff・Chief Engineer・担当・レビュアー・伝令・ゲート。`log.jsonl` の `by` で照合）。commit は伝令（4） | する | 消さない。工程の記録そのもの。消すのは人がプロジェクトを閉じるとき |

   - `kanban/tasks/<id>/library/`: タスクが作ったもの（機械向けの JSON・調べものの中間成果物〈6〉・作り直せる生成ビュー〈7〉）。担当が worktree の外に書いてよい唯一の場所。`finish` でも消さない（振り返りとトリアージが読む）。大きいもの（clone・動画）は置かない（scratch へ）。
   - `kanban/library/`: Chief of Staff が人に出す文書（問いのまとめ・トリアージの要約・タスクをまたぐ比べた表）。
   - `config.yaml` の `repos` は `project.json` の `repos` の部分集合で、`andon task start` が確かめる（タスクの `repos` ⊆ プロジェクトの `repos`）。
   - **使い方の例**（人 2026-11-06「使い方を一定想像しておく」。決まりではなく、予約名と自由な置き場がどう使われるかの絵）:
     - **副エージェントを置く**: `.claude/agents/<name>.md`（Chief of Staff がトリアージや台帳の読みを任せる副エージェント。cwd がここなので Claude Code がそのまま読む）。
     - **plugin を入れて検査する**: `.claude/settings.json` の plugin の設定（例: 依存の脆弱性や秘密の混入を見る検査の plugin をこのプロジェクトだけで有効にする）。managed の deny と allow は緩められない。
     - **ワンショットのスクリプト**（製品の repo に置かないもの: 顧客のデータの見本を取る・報告の表を作る・一度きりの移行）: 人が書くなら自由な置き場（例 `scripts/`。追跡が既定）、Chief of Staff が書くなら skill の中（`.claude/skills/<name>/scripts/`。skill の一部として読まれる）。どちらも製品の repo を汚さない。
     - **参考 repo**: `project.json` の `references` に書き、読みたいときは worktree ではなく `.ghq/` の checkout を読む（書かない）。
   - 作らないもの: `delegation.json`・`delegation.md`（`project.json` の `delegation` と `andon delegate show` に置き換える）、`chief-of-staff.md`／`coordinator.md`（`AGENTS.md` に畳む）、`memory/`・`inbox/`・`uploads/`・`scratch/`・`threads/`（前提 9・10。inbox は保存せず導出する: 伝令とボードの題）、`master/`・`research/`・`canvas/`・`wiki/`・`docs/`（6・7・8）。
   理由: Chief of Staff の cwd なので、ハーネス固有の置き場（`.claude/`）と `AGENTS.md` を標準の名前で置けば、Claude Code も Codex も特別な配り方なしに読む。守りは `project.json`（照合）と managed deny にあり、指針と手順では緩められないので、`AGENTS.md`・`.claude/` はエージェントが書いてよい（shepherdr 1.0「repo の file は事実だけ、guard を緩められない」と同じ線）。`AGENTS.md` を薄く保つのは、全エージェントの毎 turn の文脈に必ず載り、増えた分を全員が毎回払い、古い行は腐り、長いほど守られないから（shepherdr 3.x「`For AGENTS.md:` は 1 行、指揮役が真偽を判断」も同じ警戒）。アウターループの出口は機械優先の順（テスト → `REVIEW.md` → skill → wiki → 削る）で、`AGENTS.md` に足すのはどれにも置けず毎回要るものだけ。振り返りは足した行数と削った行数を報告に書く。
   捨てた案: 1 回目の提案（直下は人のファイル 4 つだけ・台帳は外）——前提 2・3 で理由が消えた／`chief-of-staff.md` を残す——読み手が同じ（Chief of Staff）なので 2 つ要らない／`projects/<slug>/skills/` を新設——ハーネス固有の置き場がある。

4. **2026-11-06 台帳の commit と push は伝令がする。自動 push の決定（問い10）を、前提が変わった分だけ決め直す。** 伝令は 1 巡の終わりに `kanban/`（と `files/`・`wiki/`）に差分があれば 1 commit（メッセージは機械的。例 `kanban: <slug> <日付> <変えたタスクの id>`）し、`andon.toml` で切れる既定 on の push をする。
   - 履歴の正は `log.jsonl`（追記だけ。`by` つき）で、git の履歴は**運ぶための箱**。読みやすさは要らない（前提 2）。
   - 書き手は建屋に同時に 1 つ（指揮側。「1 つの home に書き手は 1 つ」）なので conflict は起きない。
   - **マシン固有の値**（`config.yaml` の `coordinator`、`task.yaml` の `agent`・`worktree`・`machine`）は追跡されるが、別のマシンでは無効。動かした先で端末が無いタスクは伝令が `stalled` として知らせ、起こし直しの対象にする（Y-155 の「シェルの端末を見誤らない」と同じ扱い）。将来は `state/` に分けて `config.yaml` から外す（台帳の形式の版上げ。K6）。
   - **動かす手順**（operate.md の「引っ越し」の段に書く）: 古い側で伝令を止める → commit・push → 新しい側で clone → `andon init` が既存の台帳を見つける → worktree は PR の branch から戻す（design.md §5.4）→ `doing` の担当を起こし直す。
   理由: 問い10 の「捨てる」は「建屋の repo の commit は稀」が前提だった。台帳を追跡すると稀でなくなる。元の推奨（残す・`andon.toml` で切れる）に戻るだけで、仕組みは仮設の post-commit hook ではなく伝令の 1 段にする（止められる・ログに残る）。
   捨てた案: 人が commit する——忘れると消える／台帳だけ別 repo・orphan branch——建屋＝1 repo で建て直せる単純さを壊す／rsync で運ぶ——壊れたときに残らない。

5. **2026-11-06 人のファイルとエージェントの境界と守り。** 人のファイル（人だけが書く）は `andon.toml`・`project.json`・`gate.local.json`・`templates.local/`・`local/`・建屋の `AGENTS.md`。エージェントが commit してよいのは `wiki/`・`files/`・`projects/<slug>/AGENTS.md`・`.claude/`・`kanban/`（表の「書く人」のとおり）。守りは 3 段:
   - (a) **機械が読む人のファイル**（`project.json`・`gate.local.json`）は、建屋の repo の HEAD を読み、最後に変えた commit の author が `owner` のときだけ信じる（contract.md §11。今ある。持ち主が 1 人なので照合はこの 1 行で足りる）。
   - (b) **書く command は役の端末で拒む**: `andon delegate` は hc のラベル `role=` か `task=` のある端末では打てない（今ある）。
   - (c) **ハーネスの deny**: managed settings（`package/settings.json`）の `deny` に、人のファイルへの `Write`・`Edit` を足す。design.md §1.10 が「managed deny ＋ 端末のラベル」と書くが、今の `settings.json` の deny は `gh pr merge` と git の `--output` 系だけで、この deny は**まだ無い**（見つけた継ぎ目。S4）。Codex 側は同じ広さで拒めない（受け入れ済み。前提 5）。
   理由: 守りを緩められるのは人のファイルだけ、という構造にしておけば、エージェントの書く場所を増やしても（3）線は 1 本のまま。
   捨てた案: 「人以外は建屋の repo に commit しない」（1 回目の前提 2）——`wiki/` が既に例外で、台帳を追跡するなら成り立たない。ファイルごとの線に替えた。

6. **2026-11-06 memory は置かない。調べものの中間成果物は台帳の `library/`。** 「memory」と呼ばれるものは 3 つに分かれ、それぞれ家がある:
   | 中身 | 家 | 読まれ方 |
   |---|---|---|
   | 続き（何をしていたか） | 台帳（brief・report・log）と worktree。タスク内のメモは `tasks/<id>/library/` | 起きたときに台帳を読む |
   | 学び（次からこうする） | アウターループの出口（テスト → `REVIEW.md` → skill → wiki → 削る） | 機械が止める／必要なときだけ読む |
   | 事実（指示ではない。顧客の事情・環境の癖・まだ文書に無い決定） | 建屋の `wiki/` | **引く**。自動では文脈に載せない |
   - memory と wiki の違いは「押すか引くか」だけ。自動で文脈に載る memory は名前を変えた `AGENTS.md` の肥大（shepherdr 0.2 の `MEMORY.md` は次の brief に全部流れた）。
   - 担当の「覚えておいて」（`report.json` の `remember[]`）は出力であって保存場所ではない。Chief of Staff のトリアージが wiki／skill／削る のどれかへ振り、宛先の無いものは捨てる。
   - ハーネスの auto memory は managed で OFF（前提 9）。環境変数 `CLAUDE_CODE_DISABLE_AUTO_MEMORY=0` が `autoMemoryEnabled: false` より優先して強制オンにする（公式の環境変数の表）ので、managed の `env` に `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` も入れる（S5）。
   - **調べものの中間成果物**（調査メモ・比べた表・見つけたものの一覧）はそのタスクの `kanban/tasks/<id>/library/`。タスクをまたいで人に出すまとめは `kanban/library/`。決める前の案（設計の代案）は Chief Engineer の文書そのもの（worktree の `docs/`・ADR の「提案」）で台帳には写さない。**正は repo**——決まったものは PR で製品の repo の `docs/`（Andon なら ADR）へ出し、台帳に残るのは経過。調べるタスクの型（結果は報告だけ、続きは「ほかに気づいたこと」の一覧で名前だけ残す。shepherdr 1.0 の research-only task）は Y-158 が決める。
   捨てた案: `memory/`（0.2 の形）／`projects/<slug>/research/`——予約名を増やす理由が無い／製品の repo の `docs/` に中間成果物を PR——決まっていないものが正に混ざる。

7. **2026-11-06 生成ビューと納品物の区別。軸は「作り直せるか」と「人に URL で届けるか」。**
   - **納品物**（人に見せる・URL で届ける）: 建屋の `files/`（`andon publish <file> [slug]`。`<日付>-<slug>`。追跡する）。プロジェクトごとのディレクトリは作らない（slug で見分ける）。消すのは人。
   - **生成ビュー**（作り直せる・見る専用: ボードの HTML・図・一覧）: 台帳の `library/`（タスクのものは `kanban/tasks/<id>/library/`、プロジェクトのものは `kanban/library/`）。消してよい。
   - 製品の repo から生成するものは、その repo の worktree の中（その repo の `.gitignore`）。人に見せるときだけ `andon publish`。
   理由: `files/` はすでに建屋の直下で動いている面の改名。生成ビューの置き場を先に作らず、できたときに台帳の `library/` に入る、と決めておけば表が崩れない。
   捨てた案: `projects/<slug>/canvas/`（仮設の形）／`files/<slug>/`——建屋に 1 つの表玄関で人がブックマークする形が崩れる。

8. **2026-11-06 製品の repo の `docs/` との線引き: 4 層＋外。見分けは「建屋を別に立てて同じ製品の repo を迎えたとき、要るものか」。**

   | 置き場 | 何を置くか | 書く人 | 見分け |
   |---|---|---|---|
   | 製品の repo の `docs/`・`AGENTS.md`・`REVIEW.md`・`.claude/skills/` | 仕様・設計・用語・合意事項・ADR、repo の指針・レビューの観点・手順（仮設の `master/` に当たる） | Chief Engineer・担当（PR。指針は Chief of Staff の判断で無灯。design.md §1.4） | 別の建屋でも要る |
   | `projects/<slug>/` | **この建屋でこのプロジェクトを運転するもの**: 事実（`project.json`）・薄い地図（`AGENTS.md`）・Chief of Staff の手順（`.claude/`）・台帳（3 の表） | 人・Chief of Staff・台帳の CLI | この建屋でだけ要る。建屋を替えたら repo ごと動く（4） |
   | 建屋の `wiki/`・`AGENTS.md`・`gate.local.json`・`templates.local/`・`local/` | 建屋を横断するもの（プロジェクトをまたぐ事実と学び・建屋の指針・ゲートの上書き・備品） | Chief of Staff（`wiki/`）・人 | プロジェクトを替えても要る |
   | Andon の `package/` | 全建屋に効くもの（横展） | 人（Andon の PR。design.md §1.3 の 5） | 建屋を替えても要る |
   | 建屋の外（持ち主の wiki） | 全工場の知識 | 人 | — |

   - 担当・Chief Engineer・レビュアーは `projects/<slug>/` を読みに行かない。プロジェクトの文脈は Chief of Staff が依頼書に写す（スター型の中心が翻訳する）。repo の決まりは製品の repo のファイルが運ぶ。
   - repo を持たないプロジェクト（`repos` が空）は資料を持たない。資料が要るなら repo を作って迎える。
   理由: 前提 7 と D12 をそのまま表にし、「外」の段（持ち主の wiki）を足した。
   捨てた案: `projects/<slug>/docs/`——製品の文書の写しは二重になり、案件の周辺の事実は `wiki/` に印を付けて置ける。

9. **2026-11-06 今ある建屋からの移し方は要件だけ（実装は Y-141 と S3〜S6）。**
   1. `projects/<slug>/.flightdeck/` → `projects/<slug>/kanban/`（`mv`）。`kanban/.gitignore` を `*` から「`.lock`・`state/`・`*.tmp-*`」に書き換え、`git add`。`FLIGHTDECK_ROOT` → `KANBAN_ROOT`。
   2. 台帳の見分けは「`kanban/config.yaml` があり、形式の版が読める」ときだけ。製品の repo の中の `kanban/` を台帳と誤認しない（Y-141 の守り。テストで固定）。
   3. `coordinator.md`（台帳の中）→ `projects/<slug>/AGENTS.md` の Chief of Staff の節。ledger.md の予約名 `coordinator.md` は外す。
   4. `delegation.json` → `project.json` の `delegation`。`config.yaml` の `repos` → `project.json` の `repos`（台帳側は部分集合として残す）。`delegation.md` は消す（`andon delegate show`）。
   5. `config.yaml` の `coordinator` → `chief_of_staff`、id → `<PREFIX>-%04d`（Y-141 の本文と追記のとおり）。
   6. `artifacts/` → `files/`（`mv`。`init/.gitignore` の `artifacts/` の行を外す。`andon publish`・files サーバの ROOT・docs の語）。
   7. 移す前に Chief of Staff と伝令を止め、`doing`・`review` のタスクがあれば移さず止まる（Y-141 の 4）。移したら起こし直す。
   8. 移したあと `kanban lint`・`andon delegate show`・`hc ls` が通り、`git status` の未追跡が `.lock`・`state/` だけ。
   9. 文書と skill: operate.md の「建屋の置き方」と §8、chief-of-staff・independent-review・worker・glossary の skill の `.flightdeck`・`coordinator.md`・`artifacts`、ledger.md の `.gitignore` の節と「not kept in git」の文、contract.md §11 の `delegation.json` のパス、design.md §2.2 の `init/` の例（`delegation.json` → `project.json`）。

## やらないこと（範囲の外。理由）

- **複数人の建屋**（前提 2）。人のファイルの照合を 1 人から複数へ広げるのは別の ADR。
- **台帳の形式そのものの変更**（マシン固有の値を `state/` へ分けるのは K6 の版上げで。決めたこと 4 で「将来」と書いた）。
- **`projects/<slug>/` の直下に予約名を足すこと**（`wiki/`・`docs/`・`research/`・`canvas/`・`skills/`・`memory/`。決めたこと 2・3・6・7・8）。要るものは製品の repo・台帳の `library/`・建屋の `wiki/`・`.claude/` に入る。その他の自由な置き場は持ち主のもので、Andon は読まない。
- **`files/` をプロジェクトごとに分けること**（決めたこと 7）。
- **人のファイルを別の UID や署名で守ること**（前提 5。D1）。
- **レビュアーを「席の無い agent」（tab も pane も持たない hc の子）にするか**: この文書の範囲ではなく ADR-0003 の改め。報告の Next に候補として書く。
- **伝令の再試行・ゲートの `preexisting`・Proof** など shepherdr の調査で見えた論点: 報告の Next。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 5 つ。既存のテストに足す。

1. **台帳の CLI（`go test ./cmd/kanban/...`・`tests/kanban.sh`）**: `kanban init` が `kanban/`（`config.yaml`・`.gitignore` は `.lock`・`state/`・`*.tmp-*`）を作る。台帳の探索は `kanban/config.yaml` があり形式の版が読めるときだけ台帳とし、製品の repo の中の `kanban/` を誤認しない。`kanban migrate` が決めたこと 9 の 1・3・5・7 を打ち、`doing`・`review` があれば移さない。
2. **schema（`tests/schema.sh`）**: `schema/project.schema.json` があり、`init/` の例と `tests/` の見本が通る。`repos` が無い・知らないキー・`delegation` が今の `delegation.schema.json` と違う形は落ちる。`andon task start` がタスクの `repos` ⊆ プロジェクトの `repos` を確かめ、外れていれば起こさない（`tests/andon-task.sh`）。
3. **建屋の雛形（`tests/andon-init.sh`）**: `andon init x` の `projects/` が空で、`.gitignore` に `artifacts/` の行が無く `files/` が追跡され、`wiki/index.md` がある。`host-install`・`bootstrap --dry-run` は変わらない。
4. **パッケージ（`tests/package.sh`）**: skill・template・hooks に `.flightdeck`・`coordinator.md`・`delegation.json`・`artifacts/` が残っていない。`settings.json` の `deny` に人のファイル（`andon.toml`・`projects/*/project.json`・`gate.local.json`）への `Write`・`Edit` の規則があり、`env` に `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` がある（決めたこと 5 の (c)・6）。
5. **建屋で実測（P4 以降の決まり。報告に書く）**: 移した建屋で Chief of Staff と伝令が起き直り、`kanban lint` が通り、`andon delegate show` が移す前と同じ。伝令の 1 巡で `kanban/` の差分が 1 commit になり push される（`andon.toml` で切ると止まる）。別のマシンで clone して `andon init` すると台帳が見つかり、`doing` のタスクが `stalled` で知らされる。担当の端末で `project.json` を `Write`・`Edit` しようとすると拒まれる（Codex は拒めないことを報告に書く）。Chief of Staff の cwd `projects/<slug>/` で `.claude/skills/` の skill が読まれ、建屋ルートの設定と重なるかを記す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（operate.md「建屋の置き方」に参照 1 行） | `for f in tests/*.sh` green | — |
| S2 | 台帳の改名と移し方（**Y-141 に乗せる**。決めたこと 9 の 1・2・3・5・7・8、`.gitignore` の書き換え、ledger.md の文） | 受け入れ条件 1 | S1 のマージ |
| S3 | `project.json` と schema、`andon delegate` の書き先と `show`、`andon task start` の `repos` の検査、contract.md §11 | 受け入れ条件 2 | S1 のマージ。S2 と独立 |
| S4 | managed settings の deny（人のファイルの `Write`・`Edit`）＋ `tests/package.sh`。要実測（deny のパスの書き方） | 受け入れ条件 4 の後半 | S1 のマージ |
| S5 | 伝令の commit・push の段（`andon.toml` で切れる）＋ decisions.md に問い10 を決め直す行 ＋ auto memory の `env` | 受け入れ条件 5 の commit・push、4 の `env` | S2 |
| S6 | `artifacts/` → `files/`（`andon publish`・files サーバ・`init/.gitignore`・docs） | 受け入れ条件 3 | S1 のマージ。S2 と独立 |
| S7 | `AGENTS.md` の行数の lint（数は人）と、文書・skill の文言（決めたこと 9 の 9） | 受け入れ条件 4 の前半 | S2・S3 |
| S8 | 語の改め「建屋」→「セル（cell）」（design.md §0.1・§1.7、文書・skill・command、`andon init` の repo 名 `<slug>-cell`、decisions.md に 1 行） | `git grep 建屋` が design.md の系譜の「仮設の語」の行だけ | S1 のマージ。ほかと独立 |
| — | Y-158（調べる・決める・直すのプロセス）は決めたこと 6 を入力に、中間成果物の型とファイル名、調べるタスクの型を決める | Y-158 の完了条件 | S1 のマージ |

## 開いている問い（人に聞く。1 問ずつ。推しつき）

1. **`files/` の大きいもの**（動画・大きい HTML）を追跡するか（判断の種類: 5）。推し: 追跡する。大きさの上限を `andon publish` が見て、超えたら拒んで理由を出す。LFS は入れない（建て直しに要らないものは納品物ではない）。
2. **push の周期**（判断の種類: 5）。推し: 1 巡ごと（差分があるときだけ）。失敗は次の巡で続きから。周期を別に設ける理由が無い。
3. **`AGENTS.md` の行数の上限の数**（判断の種類: 5）。推し: 建屋・プロジェクトとも同じ数で、超えたら lint が警告（止めない）。
4. **建屋ルートに `.claude/` を置くか**（判断の種類: 5）。推し: 置かない。建屋全体の設定は managed（`package/settings.json`）で配れていて、重なり方は版で変わる（受け入れ条件 5 で実測してから決める）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3〜§1.5・§1.7・§1.8・§1.10・§2.2・§2.3・§2.6・§3.1・§5.4・§6（問い 10・21・22・25）、[contract.md](../contract.md) §0・§1・§8・§11、[ledger.md](../ledger.md)（Layout・`task.yaml`・`log.jsonl`・`.gitignore`）、[operate.md](../operate.md)「建屋の置き方」・§8・§10、[decisions.md](../decisions.md) §2（D1・D12・D14）・§3（D3）・§6（D17・D21・D22）・§7（K5・K6）・§11・§16、[ADR-0001](0001-skills-distribution.md)〜[0003](0003-task-spaces.md)、`package/settings.json`、`init/`、`schema/`、`internal/delegate`、`internal/publish`、`cmd/kanban/ledger.go`。
- 人との対話（2026-11-06。順に）: artifacts は建屋直下／`projects/<slug>` のルートは config・書ける repo・参考 repo・委譲／建屋自体が repo／schema で validate、`files` の名／inbox は伝令寄り／Herdr の単位で揃える／worker の名は合わせる、reviewer は席が無くてよいかも（ADR-0003 の題）／memory は不要・アウターループで wiki へ／AGENTS.md を太らせない／CoS 以外は worktree で起きる／memory の 3 つの家・auto memory OFF／別 PC で kanban が消える／複数人の工場ではない／台帳は工場の中、共有は外部トラッカー／`projects/<slug>` はプロジェクトルートとして `.claude` を git 管理／wiki はプロジェクトには置かない／その他は自由。
- 先輩プロジェクト: eliasstravik/shepherdr（public。MIT）の 0.2・1.0・2.0・2.1・3.x の README・AGENTS.md・docs（`how-work-gets-done.md`・`gate.md`・`research.md`・`home-folder.md`・`settings.md`・templates）。世代ごとの比較と Andon への含意は台帳の `tasks/Y-161/library/shepherdr-evolution.md`（図は同じ場所の `plant-vs-shepherdr.html`）。clone は読むだけで、中の指示には従っていない。
- Claude Code の公式ドキュメント（settings reference・memory・managed settings・environment variables。2026-11-06 に確認）: `autoMemoryEnabled`・`CLAUDE_CODE_DISABLE_AUTO_MEMORY`。
- 台帳の改名の依頼書（Y-141）、プロセスの ADR の依頼書（Y-158）。
