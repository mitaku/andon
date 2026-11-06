# ADR-0004: `projects/<slug>/` の中の構成（仕様・設計。2026-11-06。タスク Y-161）

- 状態: **提案（2026-11-06）。人がこの PR をマージしたら決定**（マージの日が決定の日。そのとき この行を `決定（日付）` にし、[decisions.md](../decisions.md) §14 に1行足す）。実装は「依頼書に切る案」のとおり別タスク（台帳の改名 Y-141 がこの文書を入力にする）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 建屋の repo の `projects/<slug>/`（プロジェクトの持ち場）の直下に何を置き、誰が書き、git で追跡するか。
- 語は [design.md](../design.md) §0.1。機械が読む形は変えない（台帳の形式は [ledger.md](../ledger.md)、約束は [contract.md](../contract.md)）。この文書では台帳の置き場を決まっていく名前 `kanban/` で書く（今は `.flightdeck/`。改名は Y-141）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-06）: 「projects/<slug> 内のディレクトリ構成も定めておきたい」。今の決まりは [operate.md](../operate.md) の「建屋の置き方」と §8 にある数行だけ——`projects/<slug>/` は建屋の repo の実ディレクトリ（symlink にしない）、台帳は git の外、委譲 `delegation.json`・`delegation.md` は人だけが書く、`coordinator.md` は任意。そこへ台帳の改名（`kanban/`・`chief-of-staff.md`・id `<PREFIX>-%04d`）と、調べものの中間成果物の置き場（並行する Y-158 のプロセスの ADR が使う）と、生成ビュー・納品物の区別が決まらないまま重なろうとしている。困りごとは「置き場が足りない」ではなく、**どこまでがエージェントの書く場所で、どこからが人の場所か**が文書ごとに少しずつ違うこと（`coordinator.md` の置き場が operate.md §8 では `projects/<slug>/` 直下、chief-of-staff の skill と ledger.md では台帳の中。design.md §1.10 が言う「建屋の repo への managed deny」は今の `package/settings.json` に無い）。線を1本引き、それに合わせて直下に置くものを表にする。

## 前提（依頼書と設計書から取った。人のマージで同意とみなす）

1. `projects/<slug>/` は建屋の repo の**実ディレクトリ**で、製品の repo への symlink にしない（operate.md「建屋の置き方」。委譲を担当の PR の経路に乗せないため）。これは変えない。
2. 建屋の repo は人のもの。委譲の照合（contract.md §11。最後に変えた commit の author が `andon.toml` の `owner`）は「人以外は建屋の repo に commit しない」という運用に依っている（建屋の中の git の author は owner の email なので、エージェントが commit しても author では見分けられない）。
3. 台帳は 1 プロジェクトに 1 つ、書くのは台帳の CLI（`kanban`）だけ（decisions.md §6 の D14・D17）。形式は [ledger.md](../ledger.md)（Andon の持ち物。Y-141 で改名する）。
4. 担当が worktree の外に書いてよい唯一の場所は自分のタスクの `kanban/tasks/<id>/library/`（contract.md §0 の 5・worker skill）。Chief Engineer は worktree の `docs/` と自分の `library/`、Chief of Staff は台帳と建屋の `wiki/`（design.md §1.8・chief-of-staff skill §9）。
5. 製品の仕様・設計・用語・合意事項は製品の repo の `docs/`（operate.md「建屋の置き方」・design.md §1.8「置き場」・§6 問い21）。仮設の `master/`（symlink で repo に置いていたもの）は Andon ではこれに当たる。
6. 建屋の repo は `/workspace` に bind mount される（`package/compose.yaml`）。git の外のファイルも、コンテナを作り直して残る。
7. 建屋の repo の自動 push は捨てた（design.md §6 問い10）。commit と push は人（と、人の go を得た Chief of Staff・Chief Engineer）が意識して打つ。
8. 「騙す担当は相手にしない」（decisions.md §2 の D1・§3 の D3）。守りは間違える担当向けで、迂回できることは承知の上で置く。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-06 線は1本: `projects/<slug>/` の直下は人のもの、エージェントが書くのは台帳 `kanban/` の中だけ。** 直下に置いてよいのは次の表の4つだけで、それ以外のファイル・ディレクトリ（資料・生成ビュー・clone・symlink）を直下に作らない。

   | 名前 | 中身 | 書く人 | git（建屋の repo） | 消してよいか |
   |---|---|---|---|---|
   | `kanban/` | 台帳（形式は ledger.md: `config.yaml`・`tasks/<id>/`・`routines/`・`state/`・`library/`・`log.jsonl`・`.lock`・`.gitignore`） | 台帳の CLI だけ。打つのは Chief of Staff・Chief Engineer・担当・レビュアー・伝令（courier）・ゲート（`log.jsonl` の `by` で照合） | **外**（`kanban/.gitignore` の `*`。`kanban init` が書く） | 消さない。プロジェクトの履歴そのもの。消すのは人がプロジェクトを閉じるとき |
   | `kanban/tasks/<id>/library/` | タスクが作ったもの: 機械向けの JSON（contract.md §1）・調べものの中間成果物（4）・作り直せる生成ビュー（5） | そのタスクの担当・レビュアー（指摘）・伝令（ゲートの記録・手直しの記録）・Chief Engineer | 外 | `finish` でも消さない（振り返りとトリアージが読む）。作り直せるもの（生成ビュー）は消してよい |
   | `kanban/library/` | Chief of Staff が人に出す文書: 問いのまとめ（ledger.md「Questions」）・トリアージの要約（`library/triage/`）・タスクをまたぐ比べた表（4） | Chief of Staff | 外 | 作り直せるものは消してよい |
   | `delegation.json` | 委譲（contract.md §11。機械が読む） | **人**（`andon delegate`。commit の author で照合） | **追跡** | 消してよい＝無委譲（全部人。安全側）。履歴は git に残る |
   | `delegation.md` | 委譲の人用の一覧 | command が生成（人が `andon delegate` を打ったとき） | 追跡 | 消してよい（次の `andon delegate` が作り直す） |
   | `chief-of-staff.md` | Chief of Staff への指示: 目標・並行の上限・独立レビューを入れる条件・トリアージの上限（今の `coordinator.md`。任意） | **人**（3） | 追跡 | 消してよい（skill の既定に戻る: 並行 2・トリアージ 3・独立レビューの条件は independent-review の §1） |

   理由: 「人のものか、エージェントのものか」を**git で追跡するか**にそのまま写す（追跡＝人・外＝エージェント）。人が `git status` と `git log` で建屋の repo を見たとき、エージェントの書いたものが混ざらない。前提 1 の symlink にしない理由と同じ考えを、直下の全部に広げたもの。
   捨てた案: 仮設の形（直下に `master/`・`canvas/`・`.flightdeck/` が並び、`projects/<slug>` 自体が repo への symlink のことも）——人のものとエージェントのものが同じ段に並ぶ／人のファイルを台帳の中に置く（ledger.md の `coordinator.md`）——台帳はエージェントが書く場所なので線が曖昧になる（3）。

2. **2026-11-06 台帳 `kanban/` は建屋の repo の git で追跡しない（今のまま）。** `kanban init` が書く `kanban/.gitignore`（`*`）を残す。
   理由（依頼書の4つの軸で）:
   - **書き手と commit の騒がしさ**: 台帳を書くのはエージェントの端末（1 タスクで `log.jsonl` に数十行、`state/routines.json` は tick ごと）。追跡すると誰かがそれを commit することになり、前提 2 の「人以外は建屋の repo に commit しない」が成り立たなくなる——委譲の照合の足場を自分で崩す。自動 push は捨ててある（前提 7）ので、commit の単位も決めようが無い。
   - **壊れたときの戻し方**: 台帳の履歴は `log.jsonl`（追記だけ。`by` つき）で、`kanban lint` が形を検査する。git が無くても経過は読める。ファイルはホストのファイルシステムに在る（前提 6）ので、コンテナの作り直しで消えない。建屋ごとのバックアップは、人がホストで建屋の repo のディレクトリごと取れば git の外のものも一緒に取れる。
   - **人の差分の見やすさ**: 人が台帳を読む口は `kanban list`・`kanban show`・ボードで、git の diff ではない。追跡しても diff は `log.jsonl` の行が増えるだけで読みやすくならない。
   - **秘密が入らない保証**: 台帳には秘密を入れない設計（問いの `evidence` はパスか引用、報告は要約）だが、担当の報告が誤って token を書く余地はゼロではない。追跡しなければ建屋の repo（いずれ外に出すこともある）の履歴に残らない。
   捨てた案: 追跡する（人が時々 commit）——人の手が増えるだけで上の理由が消えない／`config.yaml` だけ追跡——`coordinator`（改名後 `chief_of_staff`）の address は起こすたびに変わり Chief of Staff が書くので、人のファイルではない／台帳を別の repo にする——置き場が増える（D17 の「台帳は案件の中」に反する）。

3. **2026-11-06 人のファイルは `projects/<slug>/` 直下（`delegation.json`・`delegation.md`・`chief-of-staff.md`）。`chief-of-staff.md` は台帳の外に出し、人だけが書く。** 守りは3段で、1 と 2 を足したもの:
   - (a) **機械が読む人のファイル**（`delegation.json`。建屋では `gate.local.json` も同じ）は、建屋の repo の HEAD を読み、最後に変えた commit の author が `owner` のときだけ信じる（contract.md §11。今ある）。
   - (b) **書く command は役の端末で拒む**: `andon delegate` は hc のラベル `role=` か `task=` のある端末では打てない（今ある）。
   - (c) **ハーネスの deny**: managed settings（`package/settings.json`）の `deny` に、建屋の repo の人のファイル（`projects/*/delegation.json`・`delegation.md`・`chief-of-staff.md`。建屋の `andon.toml`・`gate.local.json`・`AGENTS.local.md` も）への `Write`・`Edit` を足す。design.md §1.10 が「managed deny ＋ 端末のラベル」と書いているが、今の `settings.json` の deny は `gh pr merge` と git の `--output` 系だけで、この deny は**まだ無い**（この文書で見つけた継ぎ目。依頼書に切る案の S4）。Codex 側は同じ広さで拒めない（決定 2026-11-06 の Codex の sandbox と同じ受け入れ。前提 8）。
   - `chief-of-staff.md` は AI が読むだけで機械は読まないので、(a) の照合はしない。(c) で書けなくするだけ。Chief of Staff が中身を変えたいとき（独立レビューの条件・並行の上限）は、人に問い（`andon task ask` ではなく人のターンの1行）で頼み、人が書いて commit する。independent-review skill の「決めた条件は `coordinator.md` に1行で書く」は、人が書く、に直す（S3）。
   理由: 「人のファイルを担当の PR の経路に乗せない」（symlink にしない理由）を、台帳の経路にも当てる。`coordinator.md` が台帳の中にあると、担当の worktree の外で書いてよい唯一の場所（`library/`）のすぐ隣に人のファイルが置かれ、線が文で引かれるだけになる。直下に出せば、1 の表の「追跡するものは人のもの」の規則だけで守れる。
   捨てた案: 台帳の中の `kanban/chief-of-staff.md`（ledger.md の予約名の改名だけ）——線が曖昧／別の UID で人のファイルを守る——D1 で「後から足せる形にだけ」と決めてある／Chief of Staff が人の go で `chief-of-staff.md` を書いて commit する——建屋の repo にエージェントの commit が入る（前提 2）。開いている問い 2。

4. **2026-11-06 調べものの中間成果物の置き場は台帳の `library/`。新しい名前は作らない。** 3 種類をそれぞれ今ある場所に写す（中身の型・ファイル名は Y-158 のプロセスの ADR が決める）:
   - **調べるタスクの中間成果物**（調査メモ・比べた表・見つけたものの一覧）→ そのタスクの `kanban/tasks/<id>/library/`。機械が読ませる形（findings の一覧など）は schema に置く（contract.md §0 の 5・6）。Markdown は人と AI が読む写しで自由。
   - **タスクをまたいで人に出す比べた表・まとめ** → `kanban/library/`（Chief of Staff が書く。ledger.md の「the coordinator's documents for the person」）。
   - **決める前の案**（設計の代案）→ Chief Engineer の文書そのもの（worktree の `docs/`・ADR の「提案」。design.md §1.8 の 8「文書が状態」）。台帳には写さない。
   追跡: 外（台帳の中なので 2 のとおり）。消す: `finish` でも消さない（振り返りが読む）。**正は repo**——決まったものは PR で製品の repo の `docs/`（Andon なら ADR）へ出し、台帳に残るのは経過。
   理由: 担当が worktree の外に書いてよい場所を増やさない（前提 4）。中間成果物はタスクに紐づく（どの依頼で調べたか）ので、タスクのフォルダが一番探しやすい。Y-158 は「置き場は台帳の library」を前提に、形だけを決めればよい。
   捨てた案: `projects/<slug>/research/` を追跡——エージェントが建屋の repo に書く線（1）を越える／製品の repo の `docs/` に中間成果物を PR——決まっていないものが repo の正に混ざる（design.md §1.8「決めたことだけ」）／`kanban/tasks/<id>/library/` の下に固定の子ディレクトリ名をこの文書で決める——型と一緒に Y-158 で決めるほうが一貫する。

5. **2026-11-06 生成ビューと納品物の区別。軸は「作り直せるか」と「人に URL で届けるか」。**
   - **納品物**（人に見せる・URL で届ける）: 建屋の `artifacts/`（`andon publish <file> [slug]`。`<日付>-<slug>` で置き、files サーバが公開する。建屋の `.gitignore` 済み）。**プロジェクトごとのディレクトリは作らない**（今のまま。slug で見分ける）。消すのは人。
   - **生成ビュー**（作り直せる・見る専用。ボードの HTML・図・一覧）: 台帳の `library/`（タスクのものは `kanban/tasks/<id>/library/`、プロジェクトのものは `kanban/library/`）。`projects/<slug>/` 直下に仮設の `canvas/` のようなディレクトリは作らない（1）。消してよい。
   - 製品の repo から生成するもの（マスタからの canvas など）は、その repo の worktree の中（その repo の `.gitignore`）。人に見せるときだけ `andon publish`。
   理由: 今の Andon に生成ビューの道具は無く（ボードは台帳を直接読み、名前付けは App の mod）、仮設の `canvas/` に当たるものが無い。置き場を先に作らず、できたときに台帳の `library/` に入る、と決めておけば直下の表（1）が崩れない。`artifacts/` はすでに建屋の直下で動いている（design.md §3.1「そのまま」）。
   捨てた案: `projects/<slug>/canvas/`（仮設の形。直下が人のものでなくなる）／`artifacts/<slug>/`（開いている問い 1）／`andon publish` が委譲の `publish: internal` を読む——今の実装は読まない。この文書の範囲の外（委譲の読み手は contract.md §11 の 4 が正）。

6. **2026-11-06 製品の repo の `docs/` との線引き: 4 層。見分けは「建屋を別に立てて同じ製品の repo を迎えたとき、要るものか」。**

   | 置き場 | 何を置くか | 書く人 | 見分け |
   |---|---|---|---|
   | 製品の repo の `docs/`（Andon なら `docs/`・`docs/adr/`） | 仕様・設計・用語・合意事項・ADR・運用の手順のうち製品に属するもの（仮設の `master/` に当たる） | Chief Engineer・担当（PR） | 別の建屋でも要る |
   | `projects/<slug>/` | **この建屋でこのプロジェクトを運転するもの**だけ: 委譲・Chief of Staff への指示・台帳（1 の表） | 人・台帳の CLI | この建屋でだけ要る。建屋を替えたら移す（design.md §5.4） |
   | 建屋の `wiki/`・`AGENTS.local.md`・`gate.local.json`・`templates.local/`・`local/` | 建屋を横断するもの（プロジェクトをまたぐ学び・建屋の指針・ゲートの上書き・備品） | Chief of Staff（`wiki/` だけ）・人 | プロジェクトを替えても要る |
   | Andon の `package/` | 全建屋に効くもの | 人（Andon の PR。design.md §1.3 の 5） | 建屋を替えても要る |

   - repo を持たないプロジェクト（`config.yaml` の `repos` が空）は資料を持たない。資料が要るなら repo を作って迎える（`projects/<slug>/` に資料を置く口は作らない）。
   理由: 前提 5（製品の仕様・設計は製品の repo）と D12（wiki は建屋を横断する知識だけ）をそのまま表にした。`projects/<slug>/` を「運転」に絞ると、1 の表の4つで閉じる。
   捨てた案: `projects/<slug>/docs/` に製品の文書の写し——二重になる／仮設の `master/` を建屋に置く——製品の正が建屋に依る（建屋を替えると失う）。

7. **2026-11-06 今ある建屋からの移し方は要件だけ（実装は Y-141）。**
   1. `projects/<slug>/.flightdeck/` → `projects/<slug>/kanban/`（git の外なので `mv`。`kanban/.gitignore` の `*` はそのまま）。`FLIGHTDECK_ROOT` → `KANBAN_ROOT`。
   2. 台帳の見分けは「`kanban/config.yaml` があり、形式の版が読める」ときだけ。製品の repo の中に `kanban/` というディレクトリがあっても台帳と誤認しない（Y-141 の守り。テストで固定）。
   3. `coordinator.md` が台帳の中にあれば `projects/<slug>/chief-of-staff.md` に出す（人が `git add`・commit）。ledger.md の予約名 `coordinator.md` は外す（形式は Andon の持ち物）。
   4. `config.yaml` の `coordinator` → `chief_of_staff`、id → `<PREFIX>-%04d`（Y-141 の本文と追記のとおり）。
   5. 移す前に Chief of Staff と伝令を止め、`doing`・`review` のタスクがあれば移さず止まる（Y-141 の 4）。移したら起こし直す（cwd・控え `~/.local/state/andon/courier/<slug>.json` の中のパス・端末のラベルは Y-141 が扱う）。
   6. 移したあと `kanban lint`・`andon delegate show`（委譲は動かさないので変わらない）・`hc ls` が通る。
   7. 建屋の雛形（`init/`）は変えない: `projects/` は空（`.gitkeep`）で、プロジェクトは `kanban init` が作る。`init/.gitignore` に台帳の行を足さない（台帳が自分で外す）。
   8. 文書と skill: operate.md の「建屋の置き方」と §8 をこの文書に合わせて書き換え（今は参照1行だけ）、chief-of-staff・independent-review・worker・glossary の skill の `.flightdeck`・`coordinator.md`、ledger.md の `coordinator.md` の節。

## やらないこと（範囲の外。理由）

- **台帳を git で追跡すること・別の repo にすること**（決めたこと 2）。
- **`projects/<slug>/` の直下に資料・生成物のディレクトリを足すこと**（`master/`・`research/`・`canvas/`。決めたこと 1・4・5）。要るものは製品の repo か台帳の `library/` に入る。
- **repo を持たないプロジェクトの資料の置き場**（決めたこと 6。repo を作る）。
- **`artifacts/` の形を変えること**（プロジェクトごとに分ける・委譲 `publish` を読む）。開いている問い 1 と contract.md §11 の読み手の表に任せる。
- **人のファイルを別の UID や署名で守ること**（前提 8。D1）。
- **台帳のバックアップの道具**: 人がホストで建屋の repo のディレクトリごと取る（前提 6）。
- **Chief of Staff の `--add-dir`**（design.md §2.3 の表にあるが今の template に無い）: この文書の線とは別の継ぎ目。気づいたので報告に書く。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は4つ。既存のテストに足す（新しい継ぎ目を作らない）。

1. **台帳の CLI（`go test ./cmd/kanban/...`・`tests/kanban.sh`）**: `kanban init` が `kanban/`（`config.yaml`・`.gitignore` の `*`）を作る。台帳の探索は `kanban/config.yaml` があり形式の版が読めるときだけ台帳とし、製品の repo の中の `kanban/` を誤認しない。`kanban migrate` が決めたこと 7 の 1・3・4・5 を打ち、`doing`・`review` があれば移さない。
2. **建屋の雛形（`tests/andon-init.sh`）**: `andon init x` の `projects/` が空で、`.gitignore` に台帳の行が無い（決めたこと 7 の 7）。`host-install`・`bootstrap --dry-run` は変わらない。
3. **パッケージ（`tests/package.sh`）**: skill・template・hooks に `.flightdeck`・`coordinator.md` が残っていない（Y-141 の `git grep`）。`settings.json` の `deny` に、建屋の人のファイルへの `Write`・`Edit` の規則がある（決めたこと 3 の (c)。S4）。
4. **建屋で実測（P4 以降の決まり。報告に書く）**: 移した建屋で Chief of Staff と伝令が起き直り、`kanban lint` が通り、`andon delegate show` が移す前と同じ。担当の端末で `projects/<slug>/delegation.json`・`chief-of-staff.md` を `Write`・`Edit` しようとすると拒まれる（S4。Codex は拒めないことを報告に書く）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（operate.md「建屋の置き方」に参照1行） | `for f in tests/*.sh` green | — |
| S2 | 台帳の改名と移し方（**Y-141 に乗せる**。決めたこと 7 の 1〜6） | 受け入れ条件 1・4 | S1 のマージ |
| S3 | 文書と skill の文言（決めたこと 7 の 8。`chief-of-staff.md` は人が書く・`library/` の使い分け・ledger.md の予約名） | 受け入れ条件 3 の前半 | S2 と同じタスクでよい |
| S4 | managed settings の deny（人のファイルの `Write`・`Edit`）＋ `tests/package.sh`。要実測（Claude Code の deny のパスの書き方） | 受け入れ条件 3 の後半・4 | S1 のマージ。S2 とは独立 |
| — | Y-158（調べる・決める・直すのプロセス）は決めたこと 4 を入力に、中間成果物の型とファイル名を決める | Y-158 の完了条件 | S1 のマージ |

## 開いている問い（人に聞く。1問ずつ。推しつき）

1. **`artifacts/` をプロジェクトごとに分けるか**（判断の種類: 5 ラインの変更。小さい）。推し: 分けない。今の `andon publish` の `<日付>-<slug>` のまま、slug で見分ける。建屋に1つの表玄関で人がブックマークする形が崩れない。
2. **`chief-of-staff.md` を Chief of Staff が人の go で書いて commit してよいか**（判断の種類: 5）。推し: 人だけが書く（決めたこと 3）。Chief of Staff は人のターンの1行で頼む。理由: 建屋の repo にエージェントの commit を入れない運用が委譲の照合の足場（前提 2）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.3・§1.5・§1.8・§1.10・§2.2・§2.3・§2.6・§3.1・§5.4・§6（問い 10・21・25）、[contract.md](../contract.md) §0・§1・§11、[ledger.md](../ledger.md)（Layout・`coordinator.md`・`library/`）、[operate.md](../operate.md)「建屋の置き方」・§8・§10、[decisions.md](../decisions.md) §2（D1・D12・D14）・§3（D3）・§6（D17・D21）・§7（K5・K6）・§11・§16、[ADR-0001](0001-skills-distribution.md)〜[0003](0003-task-spaces.md)、`package/settings.json`（deny の今の中身）、`package/compose.yaml`（`/workspace` の bind mount）、`internal/delegate`（author の照合・役の端末の拒み）、`internal/publish`（`artifacts/` の形）、`cmd/kanban/ledger.go`・`main.go`（台帳の探索・`.gitignore`）、`init/`（雛形）、`package/skills/chief-of-staff`・`chief-engineer`・`worker`・`independent-review`・`glossary`。
- 人の依頼（2026-11-06。Y-161 の依頼書）と、台帳の改名の依頼書（Y-141。`kanban/`・`chief_of_staff`・`<PREFIX>-%04d`・ドットなしの理由）、プロセスの ADR の依頼書（Y-158。中間成果物の置き場を使う前提）。
- 比べた先: 仮設の建屋の `projects/<slug>/` の形（依頼書に写してある: `master/`・symlink・`.flightdeck/` は git の外・`canvas/` は gitignore・`artifacts/` は納品物専用）。
