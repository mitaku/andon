# CHANGELOG

バージョンごとに、オーナーの目に効く変更と、上げるときにすることを書く。細かい変更は各バージョンの範囲の commit（`git log <前のバージョン>..<バージョン>`）で遡れる。`(#n)` は PR の番号。

- 書くのは**バージョンを切るときだけ**。PR ごとには触らない（手順は [docs/operate.md](docs/operate.md) の「バージョンを切る」）。
- `package/host/` の雛形を変えたバージョンには、「上げるときにすること」に **host-install** の項を足す（変わったファイル。退避の手順は [docs/operate.md](docs/operate.md) の「バージョンを上げるとき: host-install が置いたものを退避する」）。
- 互換の約束はまだ無い（[README](README.md) の冒頭の注記）。「上げるときにすること」に無い限り、バージョンを上げて `rebuild` するだけでよい。

## 未リリース

### 変わったこと

- **セルの入口 `bin/bootstrap` を、薄い入口と image の中の本体に分けた**（[ADR-0048](docs/adr/0048-thin-cell-entry.md)）: セルの repo の `bin/bootstrap` は薄い入口になり、打たれるたびにセルの image から本体（`/opt/andon/host/bootstrap`）を `.andon/bootstrap` に取り出して走らせる。sub-command の中身は本体が持つので、これからは Andon を上げて `rebuild` すれば入口の動きも新しくなる。`andon cell init --print <パス>` で雛形の 1 ファイルを出せる。image を見ずに本体を名指しする口 `ANDON_ENTRY_BODY` を足した。`--dry-run` は docker を打たず、焼く・取り出す・走らせるの 3 行だけを出す。
- **オーナーが打つコマンドの名前を変えた**（[ADR-0053](docs/adr/0053-command-system.md) の切る案 3）: 古い名前は別名にせず、打つと新しい名前を 1 行で言って終了コード 2 で終わる。名詞だけの `andon waiting` も断る。

  | 古い名前 | 新しい名前 |
  |---|---|
  | `andon chief-of-staff` | `andon open`（slug を省くと今いるプロジェクト） |
  | `andon chief-engineer` | `andon design start` |
  | `andon create` | `andon project create` |
  | `andon waiting` | `andon waiting list` |
  | `andon delegate show`・`grant`・`narrow`・`revoke` | `andon delegation show`・`grant`・`narrow`・`revoke` |
  | `andon measure` | `andon metrics show` |
  | `andon init` | `andon cell init` |

  プロジェクトの slug を 1 行ずつ出す `andon project list` を足した。
- **機械が打つコマンドの名前を変えた**（[ADR-0053](docs/adr/0053-command-system.md) の切る案 5）: 打つのは dispatcher・image の build・`image/apply-main.sh` で、オーナーが手で打つことはまず無い。古い名前は同じく、打つと新しい名前を 1 行で言って終了コード 2 で終わる。名詞だけの `andon dispatcher` と `andon dispatcher --once` も断る。

  | 古い名前 | 新しい名前 |
  |---|---|
  | `andon dispatcher`・`andon dispatcher --once` | `andon dispatcher run`・`andon dispatcher run --once` |
  | `andon signal delivered` | `andon signal mark-delivered` |
  | `andon version stamp` | `andon package stamp` |

  boot とフックは、dispatcher の端末の前に出ているものが `andon dispatcher run` でも、前のバージョンが起動した `andon dispatcher` でも、dispatcher が動いていると読む。`andon boot` と `andon issue session` の名前と引数は変えていない。

### 上げるときにすること

- **前に作ったセルの入口を、1 回だけ置き換える**: 分ける前の入口のまま `FROM` を上げて `bin/bootstrap rebuild` を打つと、image を焼いたあと `docker compose` のところで止まり、置き換える 1 行（image の名前入り）を案内の行の最後に出す。動いているコンテナは前の image のまま動き続ける。セルの repo でその 1 行を打ち、`bin/bootstrap rebuild` を打ち直す（手順は [docs/operate.md](docs/operate.md) の「前に作ったセルの入口を置き換える」）。
  - **置き換える前に `git diff bin/bootstrap` を見る**: 入口に手を入れていたら、置き換えでその行が消える。セルごとの違いは `.env`・`compose.override.yaml`・`local/` に書く。置き換えた `bin/bootstrap` はセルの repo に commit する。
  - **`host-install` は入口を置き換えてから打つ**: 古い入口の `host-install` は `docker compose` を打たないので止まらず、古い入口が知っている置き先だけを黙って置く。
  - v0.0.14 の「今あるセルの `bin/bootstrap` を置き換える」の手順（セルの中の `andon` で雛形を作る）は使わない。古い入口が止まったとき、セルの中の `andon` は前の image のもので、古い入口が出る。
- **host-install（雛形が変わった）**: App の mod の `inbox`・`kanban-board`・`waiting` が `andon waiting list --json` を打つようになった。セルを `rebuild` してから、この 3 つを退避して `bin/bootstrap host-install` で入れ直す（[docs/operate.md](docs/operate.md) の「バージョンを上げるとき: host-install が置いたものを退避する」）。部品とセルの andon のバージョンが合わないと、`inbox` と `waiting` は部品を入れ直すよう 1 行で出す。
- **古い名前を打っている手元のスクリプトや控えを直す**: 上の表のとおり新しい名前に書き換える。`rebuild` のあと、Chief of Staff は `andon open` で開く。
- **dispatcher は起動し直すまで古い名前を打つ**: `image/apply-main.sh apply` で当てただけで dispatcher を起動し直せなかった（見回りが 15 分終わらなかった）間は、前のバージョンの dispatcher が `andon signal delivered` を打って断られ、同じ外の信号の通知が重ねて届くことがある。見回りが終わってから dispatcher の端末を片付けて（`hc keep dispatcher --off`・`hc kill dispatcher`・`hc rm dispatcher`）`andon boot` を打てば、boot が新しい名前で起動し直して止まる。`andon boot` だけでは、古い形の dispatcher も動いていると読むので起動し直さない。`rebuild` と、dispatcher を起動し直せた当て方では起きない。このバージョンより前へ戻す（`revert`・古い commit の `apply`）ときに起動し直せなかった間も同じで、残った新しい dispatcher が戻した andon に `andon signal mark-delivered` を打って断られる。片付けてから `andon boot` を打つ手も同じ。
- **入口を分ける前のバージョンへ戻すとき**: 入口も前のものに戻してから `rebuild` を打つ（[docs/operate.md](docs/operate.md) の「GHCR のバージョンへ戻す」）。

### 直したこと

- **届けた通知の記録が、`answered` と `hold` を含むターンを落とした**: v0.0.14 と v0.0.15 の既知の不具合。`delivered`（届けた通知の記録）の schema に種類 `answered` と `hold` を足し、通知と記録の種類の一覧がずれたら検査が落ちるようにした。どちらかを渡したターンも、一緒に渡したほかの通知ごと記録に残り、`andon metrics show` の届けた通知の数と、ダイジェストの止まった担当に数えられる。直す前に落ちたターンは記録に戻らない。schema のバージョンは変わらない。

## v0.0.15 — 2026-11-25

急がないオーナー待ちを置いておく形（ADR-0062）と、オーナーの課題を判断か手作業のどちらかにする形（ADR-0057）が入り、行燈・ペイン・Stream Deck のキー・ダイジェストが、今オーナーを呼んでいるものだけを数えるようになったバージョン。ほかに、Chief of Staff が PR を止める口（ADR-0056）、ボードの表示 History（ADR-0059）、ハーネスのトレースとログをセルに残す受け口（ADR-0060）、担当の知識の欠けの申告（ADR-0061）、枠が空いたときにトリアージと振り返りを撃つ dispatcher（ADR-0047）、エージェントが PR の会話欄に書かない設定（ADR-0058）が入った。Andon を開発する側では、コードのコメントを減らす一連の変更と、当てたあとに確かめる機能の一覧の拡張が入った。**kanban の形式（3）と、どの schema のバージョンも変わらない**。`project.json` の新しい欄 `chief_of_staff.idle_minutes` と `otel` は v0.0.14 が読めないので、書く順と戻す順がある。`image/Dockerfile` が変わったので `rebuild` が要る。`bin/bootstrap` と `package/compose.yaml` は変わらない。`package/host/` の mod 3 つと Stream Deck の plugin が変わり、mod `inbox` と plugin は一緒に置き直す。

### 変わったこと

- **急がないオーナー待ちを置いておく**（[ADR-0062](docs/adr/0062-later-owner-waits.md)。ADR は #584）: Chief of Staff が予約キー `later` に急がない理由を書いた課題の判断と手作業を、`andon waiting` は今は呼ばない「置いてある」行として後ろに並べる。`later_until` の日が来たときと、後ろで置いておけない課題が待っているときは呼ぶ（#599）。判断を作る `andon issue ask --new` は、`--now`（呼ぶ）か `--later "<理由>"`（置いておく）のどちらかを必ず受ける（#610）。トリアージと振り返りの apply も、オーナーへ回す項目ごとに呼ぶか置いておくかを書く（#615）。行燈・Waiting on you のペイン・ボードの Flow・Stream Deck のキーは呼んでいるものだけで灯り、置いてあるものは「Later」として分けて出す（#617）。ダイジェストの「オーナーが呼ばれている」の数と、`andon measure` の問いが開いていた時間も、置いてあるものを分けて数える（#619）。置いておくか呼ぶかは、オーナーが Chief of Staff に言う。ペインにそのボタンは無い。
- **オーナーの課題は、判断か手作業のどちらか**（[ADR-0057](docs/adr/0057-owner-issue-decision-or-hand-work.md)。ADR は #509）: 問いも印 `only_owner` も無いオーナーの課題を、手作業でなく判断として灯す（#519）。オーナーの課題と問いを `andon issue ask --new` の 1 回で置く（#521）。トリアージの apply はオーナーへの判断を問いにして置き、手作業に印を付ける（#523）。振り返りの apply が足すオーナーの課題には、Andon へ伝えるかの問いを置く（#525）。ペインの手作業の行には依頼書の手順とリンクが出て（#573）、ペインは閉じるボタンと Esc で閉じられる（#587）。答えには、誰が決めたかの印 `decidedBy`（オーナー・Chief of Staff・機械）が付く（#510・#512）。
- **Chief of Staff が PR を止める口と、gate の印**（[ADR-0056](docs/adr/0056-gate-required-before-merge.md)。ADR は #503）: `andon issue hold <id> <理由の課題>` で止めた PR は、`andon issue unhold` で外すまで gate が回らず、dispatcher もマージしない。理由の課題が片付くと通知 `hold` で知らせる（#530）。gate はオーナー待ちの PR にも緑の印を付け、dispatcher は PR を読むたびに印を課題の状態に揃える（#517）。main で検査の置き場が変わったあとの古い base の PR は、gate が rebase まで通さない（#596）。マージのあとの影響の通知にも、検査の置き場の動きが出る（#598）。`andon/gate` を main の必須の check にする切り替えは、まだしていない。
- **ボードの表示 History**（[ADR-0059](docs/adr/0059-board-leaves-old-done-to-history.md)。ADR は #526）: 片付いて 24 時間を超えた課題を Board と List から外し、表示 History で見せる（#563）。ボードは毎回すべての課題を読まず、新しい `andon issue board`・`andon issue history` から要る課題だけを受ける（#562・#569）。セルの `andon` が古いときは、今までの読み方に戻る。
- **ハーネスのトレースとログの受け口**（[ADR-0060](docs/adr/0060-otel-trace.md)。ADR は #572）: セルの中の受け口 `andon otel serve`（`127.0.0.1:4318`）が、Claude Code と Codex が出すトレースとログを受ける。個人の属性を落として秘密の形を伏せてから、作り直しても残る volume `state` の `~/.local/state/andon/otel/` に課題ごとに書く。dispatcher が 1 日に 1 回、30 日を過ぎた記録と 10 GB を超えた分を消す（#577）。役を起動する行が、受け口へ送る設定を渡す（Claude Code は #592、Codex は #601・#604）。受け口は boot が起動し、`image/apply-main.sh` でパッケージを当てたときも、当てたパッケージで起動し直す（#606）。送らない設定と記録の寿命は、`project.json` の `otel` で変えられる。記録を課題 1 件の流れに縫う `andon trace` と、`andon measure` の内訳は、まだ無い。
- **担当が知識の欠けを申告する**（[ADR-0061](docs/adr/0061-knowledge-factory.md)。ADR は #580）: 担当は、知識が無かった所、誤っていた所、辿り着くのが遅れた所を、報告の `gaps` に書く。`andon issue report` は前の報告から引き継ぐ（#585）。トリアージは申告を 1 件ずつ拾い（#588）、振り返りの入力には、申告と期間に聞かれた問いが載る（#590）。担当の最初のメッセージには、セルの wiki の目録を指す 1 行が付く（#589）。
- **枠が空いたときに点検を回す**（[ADR-0047](docs/adr/0047-idle-slots-run-upkeep.md) の実装）: dispatcher は `chief_of_staff.idle_minutes`（既定 15 分）おきに並行の枠を見る。2 回続けて空いていれば、トリアージと振り返りのルーチンを線の上で撃ち、通知 `idle` を積む（#504）。ルーチンのファイルに `idle_hours` を書けば、そのルーチンも空いたときに撃つ（#534）。Chief of Staff が `idle` を受けたときの手は、手引き `idle.md` にある（#536）。トリアージは、次回に回した項目を 20 時間は出さない（#497）。
- **エージェントは PR の会話欄に書かない**（[ADR-0058](docs/adr/0058-agents-stay-out-of-pr-conversation.md)。ADR は #508）: Claude Code の設定で `gh pr comment` と `gh pr review` を断り、Codex の rules でも同じ口を閉じた。伝えることは PR の本文に書く（#514）。オーナーの手直しの記録から、merge commit と Chief Engineer の課題の commit を外した（#531）。
- **オーナーへの文と問い**: Chief of Staff の最後の返事に、用語集の「オーナーへはふだんの言葉で言う語」があると、Stop フックが言い換えを渡して 1 回だけ言い直させる（#581）。Chief Engineer が範囲を聞くときは、向きの呼び名を選択肢にせず、入るものと入らないものを具体の文で言う（#576）。
- **担当の起動とレビュー**: 見込みが小・中の担当を effort medium で起動するプロファイル `claude-medium` を足した（#586）。担当を起動する前に、依頼書の Settings の写しを予約キーから書き直す（#555）。トリアージと振り返りの apply とルーチンが足す課題には、予約キーの既定が書かれる（#520）。受け入れ条件の番号は、`A4` と `4` のどちらでも同じ番号と読む（#541）。担当の skill は、起動の文と揃えて読むだけのサブエージェントを許し（#543）、反論の段に「足した検査の素通り」の観点を足した（#579）。`hc wait` の途中の返りを終わりと読まない手順にし（#549・#622）、`andon issue start` が `doing` にした課題では `kanban start` を打たない（#546）。再開の最初のメッセージには送り手の印が付く（#594）。レビュアーは、変えた振る舞いを通る検証の道も見る（#621）。
- **信号**（[ADR-0051](docs/adr/0051-signals-flow-and-breaking-change.md)）: main の CI の失敗の `release` を、失敗が始まった run の commit に留める（#496）。通知 `signal` にその commit が載り、Chief of Staff はその PR を壊した変更の候補として引く（#533）。`kanban migrate` の `project.json` の下書きにも、repo があれば `github-ci` の 1 行が入る（#567）。
- **数え方と止まりの直し**: マージされた PR が 1000 本を超えても、`andon measure`・ダイジェスト・振り返りが古い PR を落とさない（#506）。片付いた課題への `kanban done` の打ち直しを、片付けた行に数えない（#564）。介在回数の説明を、操作の種類で数える形に揃えた（#566）。merge の記録が消えても、マージを打ち直す待ちの approve を黙って止めず、gate から回し直す（#570）。
- **名前を指す語をタイトルとテーマに揃えた**（[ADR-0052](docs/adr/0052-title-and-theme-not-dai.md) の実装）: 書き方の決まり・用語集・skill（#507）、コマンドの出力とコードのコメント（#558）、生きている文書（#582）、ADR と決定の記録（#591）で、名前を指す語を「タイトル」、Chief Engineer が受け持つ検討の単位を「テーマ」と書き、語の検査の範囲に入れた。
- **コードのコメントを減らした**: Go のコメントを、文書を指す制約・外の振る舞い・API の約束だけに減らし、コードの言い換えと、文書に写しの無い説明を消した。振る舞いは変えていない。`internal/issue`（#575）、`internal/dispatcher`（#595）、`cmd/kanban`（#603）、`internal/retro`（#605）、`internal/ledger`（#607）、`internal/boot`（#608）、`internal/triage`（#611）、`internal/gate`（#612）、`internal/digest`（#613）、`internal/measure`・`waiting`・`contract`（#614）、残りの小さい場所（#616）。
- **当てたあとに確かめる機能の一覧を広げた**: Andon を開発するセルで、マージした変更をセルへ当てたあとに動かして確かめる機能の一覧（検証の skill の機能の地図）に、次を足した。ダイジェスト・流れの数え・プロジェクトを立てる・オーナー待ちと gate の問いを閉じる処理（#539）、置いておく印と呼ぶ日（#600）、トリアージと振り返りの apply（#618）、ハーネスの受け口と生の記録の片付け（#583・#604・#609）、PR を止める道（#560）、dispatcher の通知の枠の空き・手直しの記録・信号（#513・#532・#535・#537）、担当の起動と再開の印（#545・#597）、受け入れ条件の番号の突き合わせ（#542）、hook の道と gate の道の直し（#511・#515・#522）。細かい機能ごとに見るファイルを持たせ、地図に載らないファイルと package を検査で落とす（#578）。見るファイルを変えた commit は、変えた振る舞いを通る細かい機能を `Verify-features:` の行で名指す（#602）。Go のコメントだけの変更は名指しが要らない（#620）。`drive.sh` は検証する commit を必ず受け、細かい機能ごとの結果を出す（#561・#624）。当て直しの記録 `applied.json` に前に当てた commit と時刻を残し、振り返りの当て直しの印もそれを読む（#516・#527）。
- **README を英語と日本語に分けた**（[ADR-0046](docs/adr/0046-readme-for-first-time-readers.md) の実装）: `README.md`（英語）と `README.ja.md`（日本語）を同じ構成で書き直し、2 つの構成を検査で突き合わせる（#574）。モデルに置き換わっていくとしても Andon を作る理由の段落を足した（#571）。
- **コマンドの表と ADR の番号**: andon の葉を 1 つの表にまとめ、help・`docs/commands.md`・bash の補完をそこから作る（#501。[ADR-0053](docs/adr/0053-command-system.md) の一部）。ADR の一覧を `docs/adr/README.md` に移してバージョンを切る PR だけが作り直し、ADR の番号は `tests/adr-new` で取る（#498・#505。[ADR-0055](docs/adr/0055-adr-prs-without-conflicts.md)）。
- **Andon を開発する人向けの整理**: `AGENTS.md` と `REVIEW.md` の決まり（#499・#518・#529・#538・#551）、検査の書き方を守らせる検査（`A && B` の受け手は #550・#554、pipefail の下で途中で抜ける読み手は #593、検査のシェルを Bash で書き換える呼び出しを断るフックは #553）、PR で検査の隔離を確かめる job isolation（#540・#565・#568）、検査の安定（#500・#524・#528・#544・#552）、振り返りの直しの依頼書（#556・#559）、文書の小直し（#502・#547・#548・#557）。

### 上げるときにすること

上から順に行う。

- **`merge: auto` の `review` の課題が片付いてから上げる**: 下で Chief of Staff を起動し直すとアドレスが変わる。予約キーを書き直す前に gate が回った課題は自動でマージされず、オーナーの判断待ちになる。キーを書き直しても、PR の head が動くまで戻らない。
- **image を上げて `rebuild` する**: セルの `Dockerfile` の `FROM` を v0.0.15 に上げて `bin/bootstrap rebuild` を打つ。`image/Dockerfile` が mise の既定の go（`~/.config/mise/config.toml`。repo の `mise.toml` と同じ 1.27.3）を焼くようになり、`mise.toml` の無い所でも非対話のシェルで go が引ける（#623）。`image/apply-main.sh` で当てたセルにはこの設定が当たらないので、`rebuild` が要る。`package/compose.yaml` は変わらず、volume も増えない。ハーネスのトレースとログは、前からある volume `state` に書かれる。受け口は `rebuild` のあとの boot が起動する。
- **`bin/bootstrap` は置き換えなくてよい**: `init/bin/bootstrap` は変わらない。v0.0.14 の「今あるセルの `bin/bootstrap` を置き換える」を済ませていないセルだけ、その手順で置き換える。
- **互換**: kanban の形式（3）は変わらず、kanban のための手作業は無い。`issue.yaml` に任意の予約キー `hold`・`later`・`later_until`、`questions.yaml` に任意のキー `decidedBy` が増えた。v0.0.14 はどれも知らないキーとして読み飛ばす（下の「戻すとき」）。`schema/` で変わったのは次のとおりで、どの schema もバージョンは変わらない。
  - `project` に任意の `chief_of_staff.idle_minutes` と `otel` が増えた。v0.0.14 は読めない（下の「`project.json` の新しい欄」）。
  - `notification` に種類 `idle` と `hold`、種類 `signal` の `release`、種類 `impact` の課題ごとの `checks_moved` が増えた。`delivered` に種類 `idle` が増えた。v0.0.14 の dispatcher は、待っているこれらの通知を控えに持つと控えを書けない（下の「戻すとき」）。
  - `waiting`（`andon waiting --json`）の行に `call`・`later`・`later_until` と、手作業の行の `steps`・`links` が増えた。`call` は必須でなく、無い行は前の決まり（マージの依頼と判断は呼ぶ、手作業は呼ばない）で読む。
  - `report` に任意の `gaps`、`issue-keys` に `hold`・`later`・`later_until` が増えた。v0.0.14 は `gaps` を持つ `report.json` を読めない（下の「戻すとき」）。
  - `retro`（`andon retro collect` の出力）に、必須の `questions` と、要約の束の必須の `gaps` が増えた。
  - `signal` の `release` は、見えた run の commit でなく、失敗が始まった run の commit になった。出来事が続く間は書き換えないので、今ある記録の値は、その出来事が終わるまで前の値のまま残る。
  - `issue-board`（`andon issue board`・`andon issue history` の出力）は新しい。
  - **既知の不具合（v0.0.14 から残る）**: `delivered`（届けた通知の記録）の schema に、`answered` と、新しい `hold` が無い。どちらかを渡したターンは、一緒に渡したほかの通知ごと、この記録に残らない。通知は届くが、`andon measure` の届けた通知の数と、ダイジェストの止まった担当から落ちる。
- **host-install**: 雛形が変わった。`host-install` は今あるものを上書きしないので、変わったものを退避してから打ち直す（手順は [docs/operate.md](docs/operate.md) の「Herdr App の mod とレイアウト」の「新しいバージョンに置き直す」と、「Stream Deck に行燈のキーを置く」の「置き直す」）。

  | 置かれるもの | 変わったこと | すること |
  |---|---|---|
  | mod `inbox` | `andon waiting` の行の `call` で数え、置いてある行は灯さない。`inbox.lamp` をバージョン 2 にし、`task` と `later` を足した（`main.js`・README・RECIPE） | 退避して置き直す。Stream Deck の plugin と一緒に |
  | mod `waiting` | 置いてある行を「Later」の見出しの下に分ける。手作業の行に手順とリンクを出す。閉じるボタンと Esc（`main.js`・`mod.json`・README・RECIPE） | 退避して置き直す |
  | mod `kanban-board` | 表示 History と、`andon issue board`・`andon issue history` の呼び出し、Flow の「Later」の印（`main.js`・README・RECIPE） | 退避して置き直す |
  | Stream Deck の plugin `andon.lamp.sdPlugin` | `inbox.lamp` のバージョン 2 だけを読み、手作業を `task` と描く（`key.js`・README） | 使っているなら、退避して `STREAMDECK_PLUGINS` を指定して打ち直す。`inbox` と片方だけを置き直すと、キーは「?」になる |

  `andon-up`・herdr の設定・ssh config・cloud-init の雛形・レイアウト `andon`・mod `andon`・`context-header`・`files-plus` は変わらない。App のあるマシンで、セルの repo から次を打つ。WSL＋Windows の App なら、先に `export HERDR_APP_HOME="$(wslpath 'C:\Users\<user>\.config\herdr-app')"` を打つ。`host-install` の行にだけ付けると、1 行目の `app` が既定の置き場を指し、古い mod が退避されずに残る。

  ```bash
  app="${HERDR_APP_HOME:-${XDG_CONFIG_HOME:-$HOME/.config}/herdr-app}"   # WSL＋Windows なら、上の export を先に打つ
  old="$app-andon-old-$(date +%Y%m%d%H%M)"; mkdir -p "$old/mods"
  for m in inbox waiting kanban-board; do [ -e "$app/mods/$m" ] && mv "$app/mods/$m" "$old/mods/"; done
  # Stream Deck を使うときだけ。先に Stream Deck のアプリを終える
  plugins=<plugin の置き場>      # WSL なら wslpath に通したパス
  mv "$plugins/andon.lamp.sdPlugin" "$old/"
  STREAMDECK_PLUGINS="$plugins" bin/bootstrap host-install      # Stream Deck を使わないなら、plugin の 2 行を飛ばし、STREAMDECK_PLUGINS を付けない
  for m in inbox waiting kanban-board; do herdr-app check "$app/mods/$m"; done
  ```

  打ち直したら、Stream Deck のアプリを起動する。
- **Chief of Staff と担当を起動し直す**: skill・テンプレート・`environment.md`・`writing.md`・`package/settings.json`・`package/codex.rules` の変更は、動いている役に効かない。テンプレートの起動の行が受け口へ送る設定を渡すようになったので、起動し直すまでの役のトレースとログは残らない。Chief of Staff（と動いている担当）を `hc kill` し、`andon chief-of-staff` で起動し直す。dispatcher は boot が起動し直す。Chief of Staff のアドレスが変わるので、Chief of Staff は開いている課題の予約キーを同じ値で書き直す。書き直すまで `andon issue start` はその課題を断り、gate は自動マージをしない。セルの `templates.local/` でテンプレートを置き替えているなら、boot はそちらを使うので、`package/templates/` の同じ名前のファイルと差分を見て、`command:` と `resume:` の行の `andon otel env` と本文の変更を写す。写さないと、その役のトレースとログは黙って残らない。
- **今あるオーナーの課題を仕分ける**（[ADR-0057](docs/adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 12）: `andon waiting` は、問いも印 `only_owner` も無いオーナーの課題を、手作業でなく判断に数えて灯す。上げた直後は、今ある手作業の行が問いの番号の無い判断の行に移り、行燈が灯る。Chief of Staff は、起動し直したあとの最初の `andon waiting` で、その行を 1 件ずつ直す。判断なら `andon issue ask <id> -` で問いを置き、オーナーにしかできない手作業なら `kanban set <id> only_owner="<理由>"` を書く。依頼書の文から機械で仕分けることはしない。
- **セルの側の文の語を直す**（[ADR-0052](docs/adr/0052-title-and-theme-not-dai.md) 決めたこと 11）: Andon の文は漢字 1 字の「題」をやめ、名前を指す所は「タイトル」、Chief Engineer が受け持つ検討の単位は「テーマ」と書く。セルの側の文はパッケージのバージョンアップでは届かないので、Chief of Staff がセルの指針（`AGENTS.local.md` とプロジェクトの `AGENTS.md`）と wiki のページを直す。`wiki/log.md` の冒頭の、見出しの形を言う行も直す。kanban に残る過去の依頼書と報告、wiki の発言の写しは直さない。
- **残ったオーナー待ちに、置いておく印を書く**（[ADR-0062](docs/adr/0062-later-owner-waits.md) の「依頼書に切る案」7）: 上げた直後は印 `later` を持つ課題が無いので、オーナー待ちは全部呼ぶ。上の仕分けのあと、Chief of Staff は急がないオーナー待ちの課題に `kanban set <id> later="<置いておく理由>"` を書く。判断にも手作業にも書く。基準は skill `ask-one-by-one` §3 の 3 つで、3 つを全部満たすときだけ置き、迷ったら呼ぶ。`andon waiting` は、その時点の Chief of Staff の端末が書いた印だけを信じるので、オーナーは手で書かない。
- **wiki の目録の節を並べ替える**: `andon init` の雛形の目録は、節を Overview・Entities・Topics・Sources の順に並べるようになった（#589）。担当は目録を Sources の節の手前まで読む。今あるセルの目録はバージョンアップでは変わらないので、Sources の節が Entities・Topics より先にあるセルでは、Chief of Staff が手引き `wiki.md` の「目録の並べ替え」で 1 回並べ替える。
- **枠が空いたときの点検は、上げると動き出す**: dispatcher は既定で 15 分おきに並行の枠を見て、2 回続けて空いていれば、ルーチン `triage` と `retro` を線の上で撃ち、Chief of Staff に通知 `idle` を積む。止めるなら、下の「`project.json` の新しい欄」で `idle_minutes` に `0` を書く。
- **`project.json` の新しい欄（任意）**: 次の 2 つは v0.0.14 が知らないキーとして弾き、`project.json` を読めないものとして無委譲・書いてよい repo 無しに倒れる。書くなら、同じ `project.json` を読むセルを全部 v0.0.15 に上げてから、オーナーが書いて commit する（[docs/contract.md](docs/contract.md) §11）。
  - `chief_of_staff.idle_minutes`: dispatcher が枠の空きを見る間（分）。既定 15。`0` なら見ない。
  - `otel`: `{"enabled": false}` なら、役の起動が受け口へ送らない。`days`（既定 30）と `max_gb`（既定 10）で、生の記録の寿命と合計の上限を変える。
- **v0.0.14 へ戻すとき**: image を下げる前に、次の順で戻す。mod と Stream Deck の plugin は戻さなくてよい。前の `andon` の出力（`call` の無い行、`andon issue board` が無いこと）でも、今までどおりに描く。
  1. `project.json` に `chief_of_staff.idle_minutes` か `otel` があれば、オーナーがキーごと消して commit する。残すと v0.0.14 は `project.json` を読めず、無委譲・書いてよい repo 無しに倒れる。
  2. `andon issue hold` で止めている課題を探す（`grep -l '^hold:' /workspace/projects/*/kanban/issues/*/issue.yaml`）。v0.0.14 は `hold` を読まないので、止めた PR にも gate が回り、`merge: auto` ならマージされる。下げたあとも止めておきたい PR は、下げる前に Chief of Staff が `kanban set <id> merge=person` に書き換える。v0.0.14 では、gate が通してもオーナーのマージ待ちになる。止めなくてよくなった PR だけ、理由の課題を片付けて `andon issue unhold` で外す。
  3. 置いておく印 `later` も v0.0.14 は読まないので、置いてあったオーナー待ちはまた呼び、行燈が灯る。印は消さなくてよい。上げ直せば、また効く。
  4. 片付いていない課題（`todo`・`doing`・`review`）で、`profile: claude-medium` のものは、Chief of Staff が `kanban set <id> profile=claude` で書き換える。v0.0.14 の image には template `claude-medium` が無く、`andon issue start` も起動し直しも断る。
  5. 片付いていない課題の `library/report.json` に `gaps` があれば（`grep -l '"gaps"' /workspace/projects/*/kanban/issues/*/library/report.json`）、下げる前にその課題を片付けるか、担当に `andon issue report <id> --data '{"gaps": []}'` で報告を書き直させる。v0.0.14 は知らないキーを持つ `report.json` を形が違うとして断るので、担当は報告も ready も打てず、gate はオーナーの判断待ちに倒れる。片付いた課題の報告は残してよい。
  6. 2 分待って dispatcher に見回らせてから、セルの中で `grep -lE '"kind": "(idle|hold|impact|signal)"' ~/.local/state/andon/dispatcher/*.json` を打ち、何も出ないことを確かめる。`idle`・`hold` は v0.0.14 の知らない種類で、`impact` は課題ごとの `checks_moved` を、`signal` は `release` を v0.0.14 の知らない欄として持つ。ファイル名が出たら、Chief of Staff に受け取らせ、そのターンが終わってから打ち直す。残したまま下げると、v0.0.14 の dispatcher は控えを書けず、通知が届かない。`idle` は枠が空いている間また積まれるので、何も出なかったらすぐ 7 へ進む。
  7. `FROM` を戻して `rebuild` する。

## v0.0.14 — 2026-11-23

オーナー待ちが待っていない仕事を止めない形（ADR-0042）と、オーナーが呼ばれているときだけ灯る行燈（ADR-0043）、行燈を出す Stream Deck のキー（ADR-0045）、期間の記録を 1 枚にして Slack へも送れるダイジェスト（ADR-0044）が入ったバージョン。ほかに、`andon measure` の介在回数の数え直し（ADR-0049）、main の CI の失敗を読む下書き（ADR-0051）、起動・レビュー・マージで担当が止まる所の直しが入った。**kanban の形式（3）は変わらない。schema のバージョンは `measure` だけが 1 から 2 に上がる**（`andon measure --json` の読み手は直しが要る）。`project.json` の新しい欄 `digest` は v0.0.13 が読めないので、書く順と戻す順がある。`package/host/` が変わり、今あるセルは `bin/bootstrap` の置き換えと host-install が要る。

### 変わったこと

- **オーナー待ちが、待っていない仕事を止めない**（[ADR-0042](docs/adr/0042-owner-waits-do-not-stop-the-line.md)。ADR は #448）: オーナー待ちと並行の枠を読む `andon waiting` を足した（#453）。並行の枠にオーナー待ちを数えず、オーナー待ちが上限に達したら新しい起動を止める（#457）。待ちの後ろにしか着手できる課題が無いときは、その時点でオーナーに知らせる（#467）。Chief of Staff は問いをダイアログでなく返事の本文に書き（#452）、Chief Engineer は本文の問いで待つ前にその問いを kanban に置く（#461）。Chief Engineer の文書から切った課題を、もとの課題に `parent` で結ぶ（#451）。
- **Herdr App のオーナー待ちのペインと Flow**（ADR-0042）: 右のパネルに mod `waiting` のペインを足した（#458）。判断の問いには、ペインから選択肢で答えられる（#465）。答えは `andon waiting answer` が書き、通知 `answered` で Chief of Staff に届く（#460）。戻せない決断はペインから答えられず、マージは GitHub のまま。ボードには、課題の連なりを順番の段で見せる表示 Flow を足した（#459）。
- **行燈は、オーナーが呼ばれているときだけ灯る**（[ADR-0043](docs/adr/0043-lamp-lights-on-owner-waits.md)。ADR は #456）: 琥珀は、止まった端末と、マージの依頼か判断のオーナー待ちで灯す。役がターンを終えただけの返事は、未読の印にする（#462）。オーナー待ちを読めないプロジェクトでは、今までどおり返事でも灯る。
- **Stream Deck のキー「Lamp」**（[ADR-0045](docs/adr/0045-stream-deck-lamp.md)。ADR は #473）: 行燈の色と件数をキーに出し、押すと待っているものを App で開く（#477）。動いている端末の数を出す 2 つ目のキーは、まだ無い。
- **ダイジェスト**（[ADR-0044](docs/adr/0044-owner-digest.md)。ADR は #466）: 期間の記録を集めて 1 枚の HTML にし、`files/` に出す `andon digest`（#472）、書く担当の skill とルーチンの例（#478）、代わりに決めたことの節（#480）を足した。オーナーが `project.json` に送り先を書けば Slack へも送る（#487）。送り先を書いたことが、毎回の送信へのオーナーの go になる。
- **介在回数を操作の種類で数える**（[ADR-0049](docs/adr/0049-owner-touches-by-kind.md)。ADR は #475）: `andon measure` の介在回数を、マージ・判断・オーナーの手作業の課題の和に改め、答えを決めた者ごとの数を足した（#486）。答えに決めた者の印を書く口はまだ無いので、判断は当面 0 になり、答えは `unmarked` に入る。前のバージョンの数とは比べられない。
- **main の CI の失敗を読む**（[ADR-0051](docs/adr/0051-signals-flow-and-breaking-change.md)。ADR は #481）: repo を渡して `andon create` すると、`project.json` の下書きに `github-ci` を `now` で読む 1 行が入る（#490）。失敗を壊した変更に結ぶところは、まだ実装していない。
- **課題の起動**: `andon issue start` は、予約キーが揃っていないか、今の Chief of Staff が書いたものでない課題を起動せずに断り、直す `kanban set` の行を出す（#464）。memory の目録が空でも起動の記録を書ける（#439）。
- **レビュー**: `review: off` の課題でも差分がレベル B・C なら、gate と同じ条件でレビューを受ける（#450）。担当は 1 回目のレビューの前に、読むだけのサブエージェントに自分の差分を壊させる（#463）。レビュアーの依頼書に、前のバージョンの形と落ちたあとの起き直しの観点（#446）と、差分の起点が merge-base であること（#485）を書く。再レビューの差分の読み方を、rebase をまたぐ場合に揃えた（#470・#471）。`review-request` の stdin を省くと、前の報告の本文を残して依頼の行を足す（#454）。
- **gate・マージ・通知の直し**: base が動いて通らなかった自動マージを、gate から照合し直して 3 回まで打ち直す（#494）。gate が早く `fix` を返すときの出力を、いつも読める JSON にした（#439）。機械が閉じた gate の問いを、オーナーの手直しに数えない（#449）。届けた通知の ack の数え方（#474）と、届けたターンの見分け（#442）を直した。
- **記録・振り返り・トリアージの直し**: `andon issue record` は空の引数を走らせる前に断る（#437）。`andon retro apply` を打ち直しても課題を重ねず、`retro collect` の入力にレビューの回の理由が載る（#445）。トリアージの束ねた依頼書で同じ段落を繰り返さない（#441）。
- **決定の文書だけが入ったもの（このバージョンに実装は無い）**: README の組み直し（[ADR-0046](docs/adr/0046-readme-for-first-time-readers.md)。#483）、枠が空いたときに点検などを回す形（[ADR-0047](docs/adr/0047-idle-slots-run-upkeep.md)。#484）、薄いセルの入口（[ADR-0048](docs/adr/0048-thin-cell-entry.md)。#482）、「題」をタイトル・テーマと書く直し（[ADR-0052](docs/adr/0052-title-and-theme-not-dai.md)。#479）、コマンドの組み直し（[ADR-0053](docs/adr/0053-command-system.md)。#492）、マージ待ちの PR に gate が赤い印を付けない形（[ADR-0054](docs/adr/0054-gate-status-by-decision.md)。#491）、ADR の一覧と番号を並行の PR で衝突しない持ち方にする形（[ADR-0055](docs/adr/0055-adr-prs-without-conflicts.md)。#493）。外のマシンで回す設計（[ADR-0050](docs/adr/0050-external-compute.md)。#476）は提案で、決定していない。
- **Andon を開発する人向けの整理**: skill と文書の小直し（#432・#436・#444）、`AGENTS.md` の決まり（#435・#440）、ADR の索引と試しの終了の記録（#433・#443）、検査の切り離しと安定（#438・#447・#455・#468・#488）、検証の skill の機能の地図（#469・#489）。

### 上げるときにすること

上から順に行う。

- **`merge: auto` の `review` の課題が片付いてから上げる**: 下で Chief of Staff を起動し直すとアドレスが変わる。予約キーを書き直す前に gate が回った課題は自動でマージされず、オーナーの判断待ちになる。キーを書き直しても、PR の head が動くまで戻らない。
- **image を上げて `rebuild` する**: セルの `Dockerfile` の `FROM` を v0.0.14 に上げて `bin/bootstrap rebuild` を打つ。`package/compose.yaml` に volume `digest`（ダイジェストのトークンの置き場）が増え、`image/Dockerfile` がその置き場を作る。volume は `rebuild` が作るので手で作らない。前からある `bin/bootstrap` のままで `rebuild` できる。`image/apply-main.sh` で当てたセルも、compose は当たらないので `rebuild` が要る。
- **互換**: kanban の形式（3）は変わらず、kanban のための手作業は無い。`schema/` で変わったのは次のとおり。
  - `measure` は 1 から 2 に上がり、**互換が無い**。`human_touches` の意味と項目が変わり、`answers` が増えた。`andon measure --json` を読むものを自作していれば直す。
  - `project` に任意の `digest`、`notification` に種類 `answered`、`retro`（`andon retro collect` の出力）に必須の `reviews[].reason` が、バージョンを変えずに増えた。v0.0.13 が書いた `project.json` と通知は v0.0.14 がそのまま読める。逆は読めない（下の「戻すとき」）。
  - `waiting`・`digest`・`digest-input`・`digest-sent` の 4 本は新しい。
  - **既知の不具合**: `delivered`（届けた通知の記録）の schema に `answered` が無い。`answered` を渡したターンは、一緒に渡したほかの通知ごと、この記録に残らない。通知は届くが、`andon measure` の届けた通知の数と、ダイジェストの止まった担当から落ちる。
- **今あるセルの `bin/bootstrap` を置き換える**: `bin/bootstrap` はセルの repo の持ち物で、`rebuild` では変わらない。前のままで host-install を打つと、mod `waiting` も Stream Deck の plugin も置かれない。`rebuild` のあと、host-install の前に、新しい image の雛形で置き換えて commit する（手順は [docs/operate.md](docs/operate.md) の「Stream Deck に行燈のキーを置く」の「前に作ったセル」。Stream Deck を使わなくても要る）。
- **host-install**: 雛形が変わった。`host-install` は今あるものを上書きしないので、変わったものを退避してから打ち直す（手順は [docs/operate.md](docs/operate.md) の「バージョンを上げるとき: host-install が置いたものを退避する」）。

  | 置かれるもの | 変わったこと | すること |
  |---|---|---|
  | mod `inbox` | 行燈の灯し方、オーナー待ちの行、未読の印、`inbox.lamp` の `version` と `at`、command `inbox.call`（`main.js`・`waiting.svg`・README・RECIPE） | 退避して置き直す |
  | mod `kanban-board` | 表示 Flow とオーナー待ちの印（`main.js`・README・RECIPE） | 退避して置き直す |
  | mod `waiting` | 新しい | 打ち直せば置かれる |
  | レイアウト `andon` | 右のパネルに `waiting`（`layouts/andon.json`・`andon.svg`） | 2 つとも退避して置き直す |
  | Stream Deck の plugin `andon.lamp.sdPlugin` | 新しい（ADR-0045） | 使うなら `STREAMDECK_PLUGINS` を指定して打つ。`inbox` が前のままだとキーは「?」になる |
  | mod `andon` | README だけ（行燈の説明） | 動きは変わらない。説明も揃えるなら退避して置き直す |

  `andon-up`・herdr の設定・ssh config・cloud-init の雛形・mod `context-header`・`files-plus` は変わらない。
- **Chief of Staff と担当を起動し直す**: skill・テンプレート・`environment.md`・`writing.md` の変更と、Chief of Staff のダイアログを外す起動行は、動いている Claude Code に効かない。Chief of Staff（と動いている担当）を `hc kill` し、`andon chief-of-staff` で起動し直す。dispatcher は boot が起動し直す。Chief of Staff のアドレスが変わるので、Chief of Staff は開いている課題の予約キーを同じ値で書き直す。書き直すまで `andon issue start` はその課題を断り、gate は自動マージをしない。
- **並行の上限の書き方を見る**: 枠は `andon waiting` が数え、上限は `project.json` の `chief_of_staff.parallel`（無ければ 2）から読む。プロジェクトの `AGENTS.md` に上限や「doing と review の数」という数え方を書いていたら、上限は `project.json` に移し、古い数え方の文は消す。
- **ダイジェストを使うなら（任意）**: ルーチン `daily-digest` は自動では入らない。オーナーの go で Chief of Staff が [例](package/skills/digest/assets/daily-digest.yaml) を kanban に写す。Slack へも送るなら、次の順で行う（[docs/operate.md](docs/operate.md) §8）。
  1. セルを v0.0.14 に上げて `rebuild` する。同じ `project.json` を読むセルが別のマシンにもあれば、先に全部を上げる。
  2. Slack のアプリを作って送り先のチャンネルに招き、セルの中で `andon digest auth slack` を打つ。
  3. オーナーが `project.json` に `digest` を書いて commit する。
  4. `andon digest check` で届くことを確かめる。
- **今あるプロジェクトの `signals`（任意）**: 下書きの 1 行は新しく作るプロジェクトにだけ入る。要るなら、オーナーが `project.json` に `"signals": [{"source": "github-ci", "weight": "now"}]` を足して commit する。v0.0.13 も読める欄なので、順は問わない。
- **v0.0.13 へ戻すとき**: image を下げる前に、次の順で戻す。
  1. `project.json` に `digest` があれば、オーナーがキーごと消して commit する。残すと v0.0.13 は `project.json` を読めず、無委譲・書いてよい repo 無しに倒れる。
  2. ルーチン `daily-digest` があれば止める（`paused: true`）。v0.0.13 には skill も command も無い。
  3. mod とレイアウトを退避したものに戻し、mod `waiting` を消す。これでペインから新しい答えが入らない。`andon waiting answer` も、下げ終わるまで打たない。
  4. 2 分待って dispatcher に見回らせてから、セルの中で `grep -l '"kind": "answered"' ~/.local/state/andon/dispatcher/*.json` を打ち、何も出ないことを確かめる。ファイル名が出たら、Chief of Staff に受け取らせ、そのターンが終わってから打ち直す。残したまま下げると、v0.0.13 の dispatcher は控えを書けず、通知が届かない。
  5. セルの repo で `bin/bootstrap` を置き換える前の commit のものに戻す（`git checkout <置き換える前の commit> -- bin/bootstrap`）。新しいもののままだと、v0.0.13 の image には mod `waiting` が無いので host-install が途中で落ちる。
  6. `FROM` を戻して `rebuild` する。

## v0.0.13 — 2026-11-21

実行の記録から skill と手順を直し続ける振り返りのループ（ADR-0028・ADR-0039）と、書き終えてから main に入るまでの時間を縮める直し（ADR-0032）が入ったバージョン。ほかに、走らせた記録を証拠にする受け入れ条件と検証の skill（ADR-0035）、テストより手前の検査（golangci-lint・書く前の schema の照合。ADR-0034）、課題を名指しする merge の委譲（ADR-0041）、Chief Engineer が自分で決める範囲とモデルでの検討（ADR-0037・ADR-0038）、pstack と yomiyasu の全セルへの配布、役の名前「オーナー」と語 kanban・gate・バージョンへの直し（ADR-0036・ADR-0040）が入った。**kanban の形式（3）と、前からある schema のバージョンは変わらない**（`schema/` の変更は、項目を足したのと新しい schema を足しただけ）。`package/host/` も変わらない。

### 変わったこと

- **振り返りのループ**（[ADR-0028](docs/adr/0028-improvement-loop.md)）: パッケージの置き場の commit と食い違いを読む `andon version`（P0-1。#286）、担当を起動するたびの起動の記録（P0-2。#289）、片付いた課題の結果とコストの記録（P0-3。#290）を足した。振り返りの入力を集める `andon retro collect`（P1-1。#295）と、パターンの一覧を書く `andon retro patterns`・直す課題に切る `andon retro apply`（P1-2。#300）を足した。
- **振り返りのペースを流量に合わせる**（[ADR-0039](docs/adr/0039-retro-scale-with-flow.md)。ADR は #375）: 1 回に切る直す課題の数の上限を外し（#378）、片付いた担当の課題が `project.json` の `chief_of_staff.retro_every`（既定 25）件たまったら dispatcher が振り返りのルーチンを撃つ（#396）。入力は前の振り返りのあとの束を全文、それより前を要約で出す（#390）。振り返りは自分自身（skill retro と `andon retro`）も直す先にできる（#385）。振り返りの skill に、`AGENTS.md` の表を読む手順と、検査を足す直しの受け入れ条件を足した（#366）。
- **書き終えてから main に入るまでを縮める**（[ADR-0032](docs/adr/0032-shorten-time-to-merge.md)。ADR は #325）: レビューを CI と並べ、レビュアーを 1 人ずつにして、動いている間の頼み直しを command が断る（#308・#329）。結果を残さずに返ったレビュアーのあとは次の回を起動し直す（#310）。衝突の無い rebase で差分が同じなら前のレビューを今の head に引き継ぎ（#364・#428）、差分が変わったときの再レビューの依頼書には PR の差分どうしの違いを書く（#399）。`review-request` と `ready` は push の直後に PR の head が追いつくのを待つ（#424）。`andon issue report` を打ち直しても、HEAD が同じ間はレビュー依頼を引き継ぐ（#386）。手元の検査は、重い検査を見る部分を変えたときだけ選ぶ（#332）。`tests/run` は同時に 1 本だけ走る（#407）。`tests/schema.sh` を速くした（#282・#328）。
- **走らせた記録を証拠にする受け入れ条件と、検証の skill**（[ADR-0035](docs/adr/0035-pstack-correct-and-verification-skill.md)。ADR は #340）: 走らせた command を kanban に記録する `andon issue record`（#363）を足し、`andon issue ready` と gate が受け入れ条件の行が指す記録を確かめる（#376）。課題が予約キー `acceptance` で受け入れ条件の番号を持てば、その番号の行が無い報告を `ready` が断る（#384）。当てた変更をセルの本物で動かす検証の skill（`.claude/skills/verify-andon/`。#367）と、当てたあと検証の課題を切る Chief of Staff の手順（#373）を足した。`AGENTS.md` を「決まり｜守らせるもの」の表にし、表の形を検査する（#348）。
- **テストより手前の検査**（[ADR-0034](docs/adr/0034-prevention-before-tests.md)。ADR は #336）: golangci-lint を CI に足し（#338）、errcheck も入れた（#380）。`andon` は約束の JSON を書く前に schema と照らし（#346。image の `andon` が schema を埋め込む）、照合は Go の検査器 1 つに寄せた（#388）。kanban の行と `kanban ready --json` に schema を足した（#358）。`andon issue report` は課題の repo と違う repo の PR を断る（#345）。課題の一生のモデルに、gate が決めたあとの変化と自動マージの規則を足した（#341）。フックの 1 行目と、`.github/workflows/` の全部の workflow を静的に検査する（#337・#339）。
- **merge の委譲**（[ADR-0041](docs/adr/0041-delegation-cap-out-of-command.md)。ADR は #411）: `andon delegate` がプロジェクトの名前で委譲の上限を決めない（#413）。課題を名指しして merge を委譲する口 `andon delegate grant merge … --issues` を足した（#419。担当が戻せないと申告した PR には効かない）。gate の追加行のパターンは、当たる箇所が hunk で増えたときだけレベル C の理由にする（#412）。
- **Chief Engineer**: 案を作り込んでから持っていき、オーナーに聞くのは 7 種の決定だけにする（[ADR-0037](docs/adr/0037-chief-engineer-autonomy.md)。#356・#362）。推奨案はオーナーの好みに合わせず、自分のプロダクトとして最適と考える案にする（#343）。状態を動かす設計を statechart のモデルで調べ（[ADR-0038](docs/adr/0038-statechart-in-chief-engineer-design.md)。#360・#372）、`model/check --draft` で下書きの規則を調べる（#371）。設計の検討で踏んだ学びを手順に足した（#421）。
- **pstack と yomiyasu を全セルに配る**（ADR-0038。#368）: managed settings で pstack の plugin を有効にし、外の skill の既定の一覧に yomiyasu を入れた。boot が上げるのは、出所が一覧の repo と同じ skill だけ。yomiyasu には、レビューの観点の語を言い換えさせない一覧を置いた（#377）。
- **トリアージが誤って閉じた項目を拾う網**（[ADR-0030](docs/adr/0030-triage-close-safety-net.md)。ADR は #296）: collect が repo ごとに最新の main を取って commit を出し、apply が判定の commit を照らす（#306）。課題を閉じる判定は、Chief of Staff が 1 件ずつ読んでから apply が当てる（#311）。collect は「計画との差:」の行と報告の無い課題を項目にせず（#303）、項目が 50 を超えるときは memory と signal を先頭に寄せる（#423）。1 回に起動する上限を `project.json` の `triage_max_issues` から読む（#319）。
- **担当の手順**: PR を出す前に、失敗の波及・前のバージョンの形・落ちたあとの起き直しを自分の差分に当てる（#370）。検査を足すか変える PR では、落ちるべき入力の変種で落ちることを確かめる（#426）。rebase や push の前に PR の状態を見て、マージ済みなら手を止める（#323）。
- **dispatcher と課題の一生の直し**: gate が決めたあとに PR の head が動いたら今の head で回し直す（#312）。閉じた PR を開き直してマージしたときも片付ける（#309）。オーナーが Chief Engineer の文書の PR をマージしたら片付ける（#304）。gate が前の head について出したマージの問いを `ready` が数えない（#365・#379）。PR の無い課題も、受け取られていない問いがあれば review に入れない（#315）。gate の `fix` で担当がいないときは、理由を通知 `gate` で Chief of Staff に届ける（#288）。待つ相手が外れた queued の課題を todo に戻し（#313）、待つ相手がマージされずに片付いたことを知らせに添える（#322）。担当が自分の問いの答えを待つ間は stalled を知らせない（#429）。ほかに、dispatcher が自分の問いを積み直す・Chief of Staff の起き直しで課題が止まる・初めて見る kanban の死んだ担当を見落とす・レビュアーの子課題を引き取れる課題として出す、などを直した（#301・#305・#307・#314・#316）。dispatcher と gate が担当へ送る文の先頭に、送り手の名前を角括弧で置く（#320）。
- **セルと image**: セルのタイムゾーンを `.env` の `CELL_TZ` で決め、既定を `Asia/Tokyo` にした（#318）。非対話のシェルでも `mise.toml` の go を引けるよう、mise の shims を PATH に入れた（#414）。image に焼いた commit を `/opt/andon/version.json` に残す（#286）。`bin/bootstrap` を引数なしで打つと起動だけにし、herdr には入らない（TUI で入るのは `bin/bootstrap herdr`。#317）。セルの根の `CLAUDE.md` への書き込みを deny で止め（#298）、Bash の呼び出しの `pgrep -f`・`pkill -f` を PreToolUse のフックで断る（#409）。boot がプロファイルの `--add-dir` のフォルダも信頼済みにする（#299）。
- **語と書き方**: 役の名前を「人」から「オーナー」に改め（[ADR-0036](docs/adr/0036-owner-not-hito.md)。ADR は #347）、用語集・skill・コマンドの出力・文書を直した（#369・#383・#391・#392・#398・#408）。gate が PR に書くコメントでは「メンテナー」と書く（#383）。「台帳」「ゲート」「版」をやめて kanban・gate・バージョンと書き（[ADR-0040](docs/adr/0040-kanban-gate-version-words.md)。ADR は #387）、repo の全部を直して語の検査で戻らないようにした（#389・#394・#395・#397・#401・#404・#406・#410・#415・#416）。オーナーへの文を日本語で書くことを `writing.md` の冒頭に書いた（#294）。工場の 2 つの面（セルは場所、Andon はしくみ）を言い分けた（[ADR-0029](docs/adr/0029-factory-place-and-mechanism.md)。#291・#292）。
- **ADR の棚卸し**（[ADR-0033](docs/adr/0033-adr-inventory-and-slimming.md)。ADR は #334）: 全部の ADR の冒頭に決定の索引を足して検査で守り（#335・#344・#349・#350・#351・#352）、実装が済んだ細部を今の正の場所への参照に替えた（#354・#355・#357・#359・#361）。ADR ごとに決定の記録があることを照らす（#418）。Chief of Staff をセルに 1 人にする案は [ADR-0031](docs/adr/0031-cell-chief-of-staff.md)（提案）に置いた（#291）。

### 上げるときにすること

- image を上げて `rebuild` する。kanban の形式（3）と、前からある schema のバージョンは変わらない。`schema/` の変更は、項目を足したもの（予約キー `acceptance`・委譲の `issues`・`retro_every` など）と、新しい schema（起動の記録・結果の記録・振り返り・パターン・走らせた記録・kanban の行と `kanban ready --json`）だけで、前のバージョンの kanban・`library/` はそのまま読める。**kanban のための手作業は無く、互換を壊す変更も無い**。
- **セルのタイムゾーンが日本時間になる**: `rebuild` のあと、`.env` に `CELL_TZ` を書いていないセルは `Asia/Tokyo` で動く（前は UTC）。UTC のままにするなら `.env` に `CELL_TZ=UTC` と書く。kanban の `config.yaml` の `timezone` とは別のもので、揃えるならオーナーが両方を書く（[docs/operate.md](docs/operate.md) §3 のタイムゾーンの段）。
- **Chief of Staff と担当を起こし直す**: skill（chief-of-staff・chief-engineer・worker・retro・triage）・テンプレート・managed settings（pstack・フック・deny）・boot が入れる `environment.md` と `writing.md` の変更は、動いている Claude Code に効かない。Chief of Staff（と動いている担当）を `hc kill` して `andon chief-of-staff` で起こし直す（[docs/operate.md](docs/operate.md) の Chief of Staff の行）。dispatcher は boot が起こし直す。yomiyasu は、gh にログインしているセルで boot が入れる。
- **セルの `bin/bootstrap` は `rebuild` では変わらない**: `bin/bootstrap` はセルの repo の持ち物なので、前のものがそのまま動く（引数なしで herdr に入る）。引数なしで起動だけにする形（#317）は、新しく作るセルから入る。
- **セルの側の文の語を直す**: Andon の文は「台帳」「ゲート」「版」をやめ、kanban・gate・バージョンと書く（[ADR-0040](docs/adr/0040-kanban-gate-version-words.md)）。セルの側の文はパッケージのバージョンアップでは届かないので、セルの指針（`AGENTS.local.md` とプロジェクトの `AGENTS.md`）・wiki のページ・kanban のルーチンの定義（振り返りとトリアージ。kanban の CLI で直す）の 3 語を直す。kanban に残る過去の依頼書と報告、wiki の発言の写し、memory は直さない。

## v0.0.12 — 2026-11-19

課題の一生の規則を形式モデルにして CI で照らし続ける形（ADR-0025）と、人のマージの拾い・マージ後の片付けを `andon issue` に出した版（ADR-0017 P7・P7b）。ほかに、課題の見込みと実働の通知（ADR-0014）、担当の日報と計画（ADR-0024・ADR-0017 P4）、「PR と repo の文は外の読み手に通じるように書く」決まり（ADR-0026）、Chief of Staff の留守でも dispatcher が回る形（ADR-0017 P6）、全体の点検が入った。**台帳の形式（3）・schema の版（`/2`）は変わらない**（`schema/` の変更は項目と値を足しただけ）。

### 変わったこと

- **課題の一生の規則を形式モデルにして CI で確かめる**（[ADR-0025](docs/adr/0025-formal-model-in-repo.md)）: `model/` に TLA+ のモデルと規則 R1〜R6 を置き、入口 `model/check` と CI の job `model` を足した（M1。#276）。モデルと `process/states/issue.yaml` を対応表で 2 方向に照らす（M2。#279）。形式モデルの決まりと、人が見る変更の線を置いた（M3。#277。ADR は #267）。
- **`andon issue ready` と `finish` が、受け取られていない問いを残したまま進まない**: `ready` は断って課題を `doing` のまま残し（#271）、`finish` は何も消さずに止まる（片付けは全部か無し。#265）。
- **人のマージと、ゲートのあとのマージを `andon issue` に出した**（[ADR-0017](docs/adr/0017-process-definitions.md) P7・P7b）: 人のマージの拾いと片付けは `andon issue settle`（記録は `<slug>.settle.json`。#251）、ゲートのあとのマージと片付けは `andon issue merge`（記録は `<slug>.merge.json`。#263）。`doing` のうちに人がマージした PR も、`review` と同じ道で片付ける（#272）。
- **dispatcher の守りと通知**: Chief of Staff の留守でも拾う・起動する・マージし、配る段だけ止める（P6。#246）。ゲートの `fix` を担当へ送れなければ、通知 `gate` に `unsent` を書いて Chief of Staff に届ける（#269）。「Connection lost mid-response」を提供元のエラーと見分ける（#264）。置き場のラベルが今の Place と違えば、見回りの合間に起こし直す（#233）。外の信号の段として `collect --due` と `now` の信号の通知 `signal` を足した（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) S3。#241）。
- **課題の見込みと実働**（[ADR-0014](docs/adr/0014-issue-estimate.md)）: `report.json` に担当の見込み `estimate`（E4。#240）、実働が見込みの 1・2・4 倍を超えたら Chief of Staff に届ける通知（E3。#237）、段の通知に詰まりの兆し `detail.sign`（`no-commit`・`large-diff`。ADR-0017 P5。#250）、`andon measure` に課題ごとの見込みと実働（E5。#244）。
- **担当の計画と日報**: `report.json` に計画 `plan[]` を足し、`andon issue report` が引き継ぐ（ADR-0017 P4。#245）。worker skill に日報（`library/journal.md`）の節を足した（[ADR-0024](docs/adr/0024-worker-journal.md) J1。#235）。レビュアーの依頼書に `findings.json` の絶対パスを書く（#261）。
- **memory の件数が線（30）を超えたら Chief of Staff に 1 度届ける**（[ADR-0020](docs/adr/0020-auto-memory-as-intake.md) M4。#242）。
- **PR と repo の文を、セルの外の読み手に通じる形で書く**（[ADR-0026](docs/adr/0026-pr-rules-for-outside-readers.md)。#270）: PR と文書の書き方の決まりを直し（#273）、ゲートのコメントと外の Issue への書き戻しを外の人が読む文にした（#275）。boot が `writing.md` を 3 本目として CLAUDE.md と Codex の AGENTS.md に入れる（[ADR-0013](docs/adr/0013-short-role-names-and-plain-japanese.md) V3。#248）。skill の書き方の行を `writing.md` を指す形に直し（V2。#247）、「今」の語を docs・README（W3。#230）・`package/`（W5。#236）・`internal`・`cmd`・`tests`（W6。#239）で置き換え、`tests/words.sh` を足した。
- **chief-of-staff skill を原則と手引き（`playbooks/`）に分けた**（ADR-0017 P9。#249）。人への文の行数の上限を外し、許可の確認の線と起動の go の例外を決まりの中に書いた（#238）。
- **開発中のパッケージをイメージを焼き直さずにセルへ当てる道**（[ADR-0027](docs/adr/0027-apply-package-without-rebuild.md)）: `image/apply-main.sh` と検査を足し（S1。#280。ADR は #274）、動いている端末の一覧と `image/` のずれの警告を足した（S2。#283）。
- **実行の記録から skill とプロンプトを直し続けるループ**（[ADR-0028](docs/adr/0028-improvement-loop.md)）: ADR のみ（#284）。
- **プロファイル `claude`・`chief-of-staff` に `--effort high` を明示した**（#268）。
- **全体の点検**: 壊れ・食い違い・要らないものを直した（#252・#253）。CLI の壊れ 8 件（#256）、約束と skill の食い違い（contract・ledger・triage・independent-review。#259）、skill の古いパスと人向けの経緯（#266）、設計・運用・image 文書と ADR の状態行（#258）、守りの揃え直し（`AGENTS.worker.md`・`--force-with-lease`・`public.sh`・isolation。#257）、CI と設定（`tests/*.sh` の二重実行・Go の版の検査・settings の重なり。#260）、重複した読みの 1 本化（#262）、boot が何も写さない `genHerdr` を外した（#255）、AGENTS.md の作法の追加（#254）。
- **image と CI**: build の apt を CI だけ Azure の mirror から取り、索引の取得を 1 回にした（#281）。CI の tests で `image.sh` の出力を流し、job の上限を 45 分にした（#278）。`andon-signal.sh` の API 失敗の検査を直した（#243）。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、前の版の台帳・`library/` はそのまま読める。**台帳のための手作業は無く、互換を壊す変更も無い**。
- **Chief of Staff を起こし直す**: skill（chief-of-staff・worker）・プロファイル（`--effort high`）・boot が入れる `writing.md` の変更は、動いている Claude Code に効かない。Chief of Staff（と動いている担当）を `hc kill` して `andon chief-of-staff` で起こし直す（[docs/operate.md](docs/operate.md) の Chief of Staff の行）。dispatcher は boot が起こし直す。
- **host-install は要らない**: `package/host/` は `andon-up`・`cloud-init.yaml`・`andon` の mod（`main.js`）・`files-plus`・`kanban-board` の文言とコメントが変わっただけで、動きは変わらない。退避して打ち直さなくてよい（打ち直すなら [docs/operate.md](docs/operate.md) §4「版を上げるとき: host-install が置いたものを退避する」）。

## v0.0.11 — 2026-11-12

サイドバーに行燈の見た目が入り、Chief of Staff に Advisor（Fable）を試験運用で付け、auto memory を全部の役に入れた版。ほかに、台帳の CLI の別名 `deck` の廃止、pstack の試用（ADR-0021）、担当の日報（ADR-0024）、dispatcher と gate の rate limit・lock の守りが入った。**台帳の形式（3）・schema の版（`/2`）は変わらない**（`schema/` の変更は項目と値を足しただけ）。

### 変わったこと

- **台帳の CLI の別名 `deck` を外した**: image の `/usr/local/bin/deck`・Tab 補完・`kanban --help` の行を消し、skill・文書・コード・テストの台帳の CLI を指す `deck` を `kanban` にした（#211）。
- **サイドバーに「あんどん」らしさを出した**（[ADR-0023](docs/adr/0023-sidebar-andon-look.md)）: Inbox の Needs you を、答えれば進む行は琥珀、失敗して止まった行は赤にし、一番上を Herdr のロゴから Andon の行燈にした（L2・L3。#222）。畳んだ列には行燈を出さなくてよい（#218。ADR は #217）。
- **Chief of Staff に Advisor（Fable）を試験運用で付けた**（[ADR-0022](docs/adr/0022-advisor-and-chief-engineer.md)）: Chief Engineer の役は残す。`chief-of-staff` のプロファイルに `--advisor fable` を足した（V1。#224。ADR は #212）。
- **auto memory を全部の役に入れた**（[ADR-0020](docs/adr/0020-auto-memory-as-intake.md) M3）: managed settings で `autoMemoryEnabled: true`、置き場は `/workspace/memory`。環境メモ・chief-of-staff skill・`init/.gitignore` も合わせた（#192）。
- **提供元に切られた Chief of Staff を `StopFailure` のフックで起こす**（[ADR-0019](docs/adr/0019-cos-input-box.md) 決めたこと 10）: `hooks/stop-failure` が `andon dispatcher rewake` を打つ（#198）。
- **dispatcher と gate の守りを足した**: gate が gh の rate limit で落ちたら問いにせず reset まで待って回し直す（#228）。gate の checks 待ちの回し直しで REST を読まない（#221）。dispatcher・gate・chief-of-staff・boot の lock（[ADR-0007](docs/adr/0007-interim-gap-inventory.md) R7。#209）。最初のメッセージを送り直す前に入力欄を読み、残っていれば Enter だけ押す（#201）。dispatcher の端末の出力の行に時刻を付けた（#231）。見回りが Chief of Staff に送らないことを確かめる検査の競合を直した（#207・#232）。
- **全役が `hc` の手引きを読む**: 環境メモと chief-of-staff skill に `hc --skill` を読む行を足した。Chief of Staff は起きたとき、ほかの役は要るときだけ読む（#229）。
- **担当の指針**: 担当が課題ごとに日報（過程・感想・改善の思いつき）を書く（[ADR-0024](docs/adr/0024-worker-journal.md)。#227）。最初のメッセージで `AGENTS.worker.md` を指す（[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) R6。#204）。受け入れ条件の証拠の質を書いた（R5・A4。#203）。worker skill に読み返しの 1 行を足し、日本語の脱字を直した（#223）。`writing.md` に URL を全角の括弧・文字と離して書く決まりを足した（#214）。
- **pstack を試す**（[ADR-0021](docs/adr/0021-pstack-in-cell.md)）: 担当の作法を外の pstack から借りる。plugin の登録と自動更新は、セルの利用者の `settings.json` に書かれた状態を正とする（#210・#215・#213。`design.md` の出所は #216）。パッケージの既定は変えない。
- **triage**: `kind: signal` の項目を拾い、`apply` が signal の key を通す（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) S4。#206）。課題に切られた信号を `links` の `url` に加えて `key` でも照らす（github-ci の run ごとの url に対応。#226）。
- **`andon issue finish` が出所の issue に結果を書き戻す**（ADR-0011 S9）: `write_back: true` の GitHub issue に 1 コメント書く（#205）。
- **`andon measure`**: 課題ごとのトークン数（#208）と、通知の素通りの数（#202）を足した。
- **`bin/bootstrap up` が古い image で黙って起きない**: 手元の image が Dockerfile の FROM から焼いたものでないと分かれば、起こさず `rebuild` を打つよう止める（#219）。
- **image**: Claude Code を 2.1.318 から 2.1.322 に上げた。
- **README と文書**: セルの考え方とよさの節と English summary の Why a cell（#200）、`herdr notification show` の文言（#199）、host-install の退避の手順（#225）、CI で文書だけの PR は文書を読む検査だけを走らせる（#220）。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、前の版の台帳・`library/` はそのまま読める。**台帳のための手作業は無い**。
- **`deck` を打っているものは `kanban` に直す**: image から別名 `deck` を外したので、セルの中のスクリプトや手順書で `deck` を打っていれば `kanban` にする。
- **Chief of Staff を起こし直す**: managed settings（auto memory・`StopFailure` のフック）・プロファイル（`--advisor fable`）・skill の変更は、動いている Claude Code に効かない。Chief of Staff（と動いている担当）を `hc kill` して `andon chief-of-staff` で起こし直す（[docs/operate.md](docs/operate.md) の Chief of Staff の行）。dispatcher は boot が起こし直す。
- **前の雛形で作ったセルは `.gitignore` に `memory/` を足す**: セルの repo で `grep -qx 'memory/' .gitignore || echo 'memory/' >> .gitignore` を 1 回打つ（`/workspace/memory/` を git で追わない。[docs/operate.md](docs/operate.md) §2）。
- **host-install**: サイドバーの行燈で `package/host/` の `andon` の mod（`main.js`・`brand.js`・`lantern-*.svg`・`LICENSE`・`README.md`）、`inbox` の mod、レイアウト（`layouts/andon.json`・`andon.svg`）が変わった。`andon-up`・herdr の設定・ssh config・cloud-init の雛形は変わっていない。`host-install` は今あるファイルを上書きしないので、App の設定フォルダの `mods/andon`・`mods/inbox` と `layouts/andon.json`・`layouts/andon.svg` を退避してから `bin/bootstrap host-install` を打つ（[docs/operate.md](docs/operate.md) §4「版を上げるとき: host-install が置いたものを退避する」）。置き直さなければ、前の見た目のまま動く。
- **pstack を使うなら**: セルの利用者の `~/.claude/settings.json` に `extraKnownMarketplaces`（`cursor-plugins`）と `enabledPlugins`（`pstack@cursor-plugins`）を書く。書かなければ何も入らない。

## v0.0.10 — 2026-11-12

離席中の Chief of Staff を、入力欄へ打たずに起こす形（ADR-0019）に替えた版。dispatcher は `<news>` を入力欄へ `hc send` しなくなり、Stop の待ち役が合図で起こす。ほかに、担当が画面を見て確かめる道具（ADR-0018）、止まった担当の自動の resume（ADR-0012）、課題の見込みと実働（ADR-0014）、仕事の流れと状態の定義（ADR-0017）、人への書き方の置き場 `package/writing.md`（ADR-0013）、triage が拾う memory（ADR-0020）が入った。**台帳の形式（3）・schema の版（`/2`）は変わらない**（`schema/` の変更は項目と値を足しただけ）。auto memory そのもの（ADR-0020 の M3）はまだ入っていない。

### 変わったこと

- **離席中は待ち役が Chief of Staff を起こし、入力欄へ `<news>` を打たない**（[ADR-0019](docs/adr/0019-cos-input-box.md) W4・W5）: Stop の `asyncRewake` のフック `hooks/stop-wait` が `andon dispatcher wait` を起こし、届ける通知があって Chief of Staff が入力待ちなら合図の 1 行で新しいターンを始める。入力欄の書きかけは残る。timeout は 7200 秒で、その 9 割で張り直す。dispatcher の入力欄の確かめ・離席時の `hc send`・「続けて」を打つ道・控えの `coord`・`news_turn` は消した（#184・#189。前の確かめの寄せは #165）。
- **Stop の drain は `additionalContext` で渡す**: `decision: block` をやめ、画面の「Stop hook error」を出さない。控えの lock を諦めても、渡した turn の終わりを控えの外に残し、次の送信で `<news>` を届け直さない（#174・#181）。
- **止まった担当を 1 回だけ自動で起こし直す**: 端末が Gone の `doing` の担当を、dispatcher が `andon issue resume` で 1 回起こす（通知の kind に `resumed`。[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) R4。#173）。resume の文書・Chief of Staff の止まった担当の表・worker と Chief Engineer の続きの読み方・`git clean -x` の注意（R5。#176）。
- **`andon issue` が定義に無い状態の移り方を断る**: `process/states/issue.yaml` を `andon` に埋め込み、無い移り方は打てない（[ADR-0017](docs/adr/0017-process-definitions.md) P12。#177）。
- **仕事の流れと状態の定義**: 残り 7 本のステートマシンと記録の項目の対応表（P2b。#159）、外のお知らせの一生 `process/states/signal.yaml`（P2a。#166）、見守り・仕分けの定義と残り 6 本のワークストリームの名前・Chief of Staff skill §5・§10・用語集（P3。#188）。
- **課題の見込みと実働**: 予約キー `estimate`（`small`・`medium`・`large`）と依頼書の Estimate の行（[ADR-0014](docs/adr/0014-issue-estimate.md) E1。#162）。待ちを除いた実働を `log.jsonl` と Gone の時刻から数える `ledger.Worked` と、段→分の表（E2。#182）。
- **担当が画面を見て確かめる**（[ADR-0018](docs/adr/0018-screen-check-tooling.md)）: image に `playwright-cli` の wrapper と PATH、焼いた Chromium と出力先 `/tmp/playwright-cli` の既定を入れた（S2。#161）。boot が同梱の skill を symlink で張り、外の skill を `gh skill` で入れる口（`package/gh-skills.txt`・セルの `local/gh-skills.txt`。S3。#179）。ゲートに `screen_paths`（画面のパスを変える PR は GitHub の添付が無いと `fix`。既定は無し。S4。#163）。skill・設定・文書を画面の確かめに揃えた（`allowManagedPermissionRulesOnly`・worker §2・§3・§7・independent-review の観点・environment.md。S5。#167）。
- **managed の許可ルールだけが効く**: `allowManagedPermissionRulesOnly: true`。担当の `git push --force-with-lease` を通す許可を足し（`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` の env は消した。#160）、main・`--all`・`--mirror` に当たる形は managed の deny で断る（#169）。
- **dispatcher が live でない Chief of Staff の台帳を飛ばすとき 1 行出す**: Chief of Staff の端末が live でない台帳を飛ばし始めたとき、ログ 1 行と herdr 通知を出す（#194）。
- **dispatcher の再開の文に、やり直しの許しを書く**: 提供元のエラーのあとの「続けて」に、やり直しに承認が要らないことを足した（#195）。
- **Codex の自動の記憶を止める**: boot が `~/.codex/config.toml` の `features.memories`・`memories.generate_memories`・`memories.use_memories` を `false` にし、`codex`・`codex-review` のプロファイルは同じ `-c` を渡す（#183）。
- **`fable` と `chief-engineer` は `--effort high` を明示して起こす**（#168。[design.md](docs/design.md) §1.8 の表も合わせた）。
- **人への書き方を `package/writing.md` に置いた。全役に配るのは次の版（V3）**: 番号の 2 行と 3 行・語の決まり・用語集のパス（[ADR-0013](docs/adr/0013-short-role-names-and-plain-japanese.md) V1。#175・#185）。ADR-0013 の改めで、細かい書き方の推敲を yomiyasu に任せられるかをセル 1 つで試す（#190）。
- **triage が memory を拾う**: `collect` が `/workspace/memory` の 1 件 1 ファイルを `kind: memory` で拾い、`apply` が `keep` 以外の memory と目録の行を消す。収集のあと書き換わったものは消さず、記録に `memory_kept` を書く（[ADR-0020](docs/adr/0020-auto-memory-as-intake.md) 決めたこと 3・5。#191・#193）。auto memory を書かせる側（M3）はまだ無い。
- **skill の手直し**: worker（承認の表・書く場所を 1 か所に・報告の字数・hc の reach は指示ではない。#171）、Chief Engineer の「読む」から常設の副エージェントを外す（#164）、価値の仮説の 6 項目・観察の課題（`--parent` と `not_before`）・signal の受け方・worker の観察の報告（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) S7。#180）、`AGENTS.md`・`REVIEW.md` の古い注意書き（#172）。
- **検査**: 検査をセルの本物の台帳と HOME から切り離し、偽のセルで確かめる `tests/isolation`（#187）。
- **文書（動作は変えない）**: [ADR-0019](docs/adr/0019-cos-input-box.md)（#170）・[ADR-0020](docs/adr/0020-auto-memory-as-intake.md)（提案。#186）を足し、README の ADR の地図を 0001〜0019 に揃えた（#178）。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、前の版の台帳・`library/` はそのまま読める（`schema/` は項目・値を足しただけ）。**台帳のための手作業は無い**。dispatcher の控え（`<slug>.json`）に残る `coord`・`news_turn` は読み捨てる。
- **Chief of Staff を起こし直す**: 待ち役の Stop フック（`stop-wait`）と `allowManagedPermissionRulesOnly` は managed settings の変更で、動いている Claude Code には効かない。Chief of Staff（と動いている担当）を `hc kill` して `andon chief-of-staff` で起こし直す（[docs/operate.md](docs/operate.md) の Chief of Staff の行）。dispatcher は boot が起こし直す。効いたかは operate の「待ち役」の確かめ（`hc read dispatcher` に Chief of Staff への `hc send` が無いこと）。
- **managed の許可ルールだけが効く**: セルで user・project・local の `allow` に頼っていた許可は無視される。必要なものは `package/settings.json` に足す（足りなければ Chief of Staff が許可の確認に答える）。
- **Codex の設定が書き換わる**: `~/.codex/config.toml` に `memories`・`features` の表がすでにあって `false` でないと、boot が TOML を作り直し、元を `config.toml.bak` に退避する（表が無ければ先頭に足すだけ）。既存の記憶ファイルは消えない。動いている Codex は起こし直すまで前の設定のまま。
- **外の skill を使うなら**: セルの `local/gh-skills.txt` に `<owner>/<repo> <skill>` を書き、`gh auth login` 済みにする（既定は空。書かなければ何も入れない。同梱の `playwright-cli` は boot が張る）。
- **画面を変える PR に添付を求めるなら**: セルの `gate.local.json` の `screen_paths` に画面のパスを書く（既定は無しで、何も変わらない）。
- **App の mod の置き直しは要らない**（`package/host` は変わっていない）。
- `memories`／auto memory: ADR-0020 の M3（auto memory を有効にして気づきを書かせる側）は入っていないので、`CLAUDE_CODE_DISABLE_AUTO_MEMORY` と `autoMemoryEnabled: false` は前の版のまま。`/workspace/memory` に置いたファイルがあれば triage が拾う。

## v0.0.9 — 2026-11-11

dispatcher がルーチンを撃つようになり、`<news>` の答えを ack だけにし、Claude のプロファイルをモデルの別名に揃えた版。仕事の流れと状態の定義（ADR-0017）の器と、セルの wiki の新しい形（ADR-0016）が入った。**台帳の形式（3）・schema の版（`/2`）は変わらない**（`schema/process/` は新しいファイルで、既存の schema は変えていない）。

### 変わったこと

- **dispatcher が `kanban tick` を打つ**: 1 巡の 3（台帳を読む）と 4（拾う）の間で 1 回打ち、ルーチンの `schedule` が来たものを課題にする。cron は置かない（[ADR-0007](docs/adr/0007-interim-gap-inventory.md) 切る案 R1。#148）。
- **`<news>` の答えは `andon news ack` だけ**: 末尾の 1 行を「人に言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える」に直し、`<no-reply>` だけの答えで `news.replied` を書く移行の道を外した（[ADR-0010](docs/adr/0010-news-ack.md) 決めたこと 3・6。#154）。
- **Claude のプロファイルはモデルの別名で起こす**: `opus`・`sonnet`・`fable` に揃え（版を書かない）、`fable` と `chief-engineer` は `--effort` を外して既定（Fable 5.2 は medium）にした（#157）。
- **dispatcher の誤報と雑音を減らした**: 人が先に同じ head をマージしていたら `gh pr merge` の失敗を問いにせず片付ける（R2。#153）。Chief Engineer の stalled・blocked・ready は記録だけにし、進んでいる working を stalled にしない（#145）。
- **`andon issue resume` がレビュアーにも効く**（[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) 決めたこと 8・13 の 5。#144）。
- **`andon delegate` が `andon.toml` の owner を author と committer にして commit する**: 人の端末の git の設定に関わらず、委譲がそのまま信じられる（#155）。
- **セルの wiki を LLM Wiki の形に**: `raw/`・`sources/`・`entities/`・`topics/`・`overview.md`・`index.md`・`log.md` と ingest・query・lint（[ADR-0016](docs/adr/0016-cell-wiki-llm-wiki.md) K1。#149）。前の 3 件の手順は chief-of-staff skill と glossary へ（K2。#152）。
- **App の inbox を Needs you と Projects の 2 つに**: Done・棚・settle・ボードのダイアログを消した（[ADR-0015](docs/adr/0015-inbox-needs-you.md) I2。#142）。context-header の hover の ✓（settle）も消した（#150）。
- **流れと状態の定義の器**: `schema/process/`・`process/`・`andon process render`・`tests/process.sh`（[ADR-0017](docs/adr/0017-process-definitions.md) P1。#151）。中身の定義は次の版から。
- **`andon measure` の先頭に価値の行**（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) 決めたこと 18。#137）。
- **CI**: CHANGELOG.md だけの PR では image の検査を飛ばす（#147）。
- **文書（動作は変えない）**: [ADR-0016](docs/adr/0016-cell-wiki-llm-wiki.md)（#143）・[ADR-0017](docs/adr/0017-process-definitions.md)（#146）・[ADR-0018](docs/adr/0018-screen-check-tooling.md)（担当が画面を確かめる道具。実装はまだ。#156）を足した。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式・schema の版は変わらず、**互換のための手作業は無い**。
- **App の mod を置き直す**: inbox と context-header が変わったので、App の設定フォルダの `mods/` から Andon の mod 5 つを消して `bin/bootstrap host-install`（[docs/operate.md](docs/operate.md) の「Herdr App の mod とレイアウト」）。
- **セルの wiki を新しい形へ移す**（前の形のページが残るセルだけ。chief-of-staff skill §11 の「移行」。Chief of Staff が 1 回する）。
- `<no-reply>` だけの答えは `news.replied` にならない。Chief of Staff は `andon news ack` で答える。

## v0.0.8 — 2026-11-10

記録だけの通知を Chief of Staff に届けず ack で答えるようにし、外の信号（GitHub の main の CI・issue）を読む command を足した版。**台帳の形式（3）・schema の版（`/2`）は変わらない**（`schema/` は触っていない）。

### 変わったこと

- **記録だけの通知を Chief of Staff に届けない**: dispatcher がインナーループで自分で進めた結果（`review-requested`・ゲートを回す課題の `review`・`gate` の `fix`／`needs_human`・`merge: auto` で進む `approve`・`held` の無い `merged`・レビュアーの課題の `report`）は `<news>` に載らず、`~/.local/state/andon/dispatcher/<slug>.recorded.jsonl` に1行ずつ足す。人の判断が要るもの（マージしない `approve`・`held` のある `merged`・`asked` など）は今までどおり届く。`merge: auto` の `approve` はマージにも問いにも進まず10分たったときだけ1回届く。（[ADR-0010](docs/adr/0010-news-ack.md) 決めたこと 1・2・7。#120）。
- **`andon signal collect`・`andon signal auth`**: `project.json` の `signals` に書いた外の出所のうち、`github-ci`（案件 repo の main の CI の失敗）と `github-issues`（open の issue）を `gh` で読み、信号の記録（`<slug>.signals.json`）に書く。出所へは書かない。読めなかった組は記録を変えずに飛ばす。Sentry・Linear・Cloudflare は読む command がまだ無い。`signal auth` は出所のトークンを置く（置き場 `~/.config/andon/signals` は新しい volume）。`project.json` の `repo` と `filter` の読み方を `docs/contract.md` §15 に足した（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) S2、決めたこと 3・8・9・10。#133）。
- **`andon news ack`**: Chief of Staff が届いた `<news>` に「人に言うことが無い」と示す command。端末のラベル `news.ack=<turn>` を書き、dispatcher がそのターンが終わったときに `news.replied` を書く（答えの文は読まない）。移行の間は、ack が無くても答えが `<no-reply>` だけなら今までどおり `news.replied` を書く（次の版で外す）。同じ turn の前の ack は数えない（ADR-0010 決めたこと 4〜6・8。#138）。chief-of-staff skill も `<no-reply>` から `andon news ack` に揃えた（#141）。
- **レビュアーの `deck start` が worktree と branch を上書きしない**: dispatcher が最初の `start` で決めたレビュアーの課題の `worktree`・`branch` を、あとの `start` が打った場所から取り直して壊さない（#122）。
- **文書（動作は変えない）**: [ADR-0015](docs/adr/0015-inbox-needs-you.md)（App の inbox を Needs you と Projects の 2 つに絞る。#140）、[ADR-0014](docs/adr/0014-issue-estimate.md)（課題の見込みと実際のズレを測り Chief of Staff に届ける。提案。一律の上限で担当を止めない。実装はまだ無い。#134）を足し、[ADR-0007](docs/adr/0007-interim-gap-inventory.md)（暫定のすき間の一覧）を引っ越しのあとの今に合わせて出し直した（#135）。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、前の版の台帳・`library/` はそのまま読める。**互換のための手作業は無い**。
- 新しい volume `signals`（`~/.config/andon/signals`）はセルの compose に足してあり、`rebuild` で作られる。外の信号を使うときだけ `project.json` の `signals` を書き、`andon signal auth` でトークンを置く（置かなければ何も読まない）。
- 記録だけの通知は `<news>` に出なくなる。経緯を見たいときは `log.jsonl`（全部の出来事の正）か `<slug>.recorded.jsonl` を読む。

## v0.0.7 — 2026-11-10

v0.0.6 の release が出なかった分を直して出し直す版。v0.0.6 の tag の release の run が、イメージのビルド中の `apt` の取得が 404 で落ち、イメージも release も出なかった。v0.0.6 の tag は動かさず、直しを含めて v0.0.7 として切る。**台帳の形式（3）・schema の版（`/2`）は変わらない**。

### 変わったこと

- **イメージのビルドで `apt` の取得に失敗したら、`update` からやり直す**: パッケージの索引とプールのずれで 404 になっても、ビルドが落ちない（#131）。v0.0.6 からの機能の変更は無い。

### 上げるときにすること

- **v0.0.6 から上げる人も、v0.0.5 から上げる人も、v0.0.7 を使えばよい**。v0.0.6 は release が出ていない（イメージが無い）ので、使わない。
- 上げるときにすることは **v0.0.6 の節**のとおり。この版で足すことは無い。

## v0.0.6 — 2026-11-10

**release は出ていない**（イメージのビルドが `apt` の 404 で落ちた）。v0.0.7 を使う。

止まった担当と Chief Engineer を同じ worktree でセッションの続きから起こせるようにし、worktree の置き場をセルの repo の中へ移した版。台帳に任意のキー（`session`・`not_before`）、`report.json` に任意の `observation`、信号の記録の schema が入った。**台帳の形式（3）・schema の版（`/2`）は変わらない**（足したキーは全部任意）。

### 変わったこと

- **`andon issue resume <id> [--fresh] [--dry-run]`**: セルが起き直して端末が素のシェルに戻った（か端末が無い）`doing` の担当と Chief Engineer を、同じ worktree・branch のままセッションの続きから起こす。worktree が消えていれば戻す。セッションの記録が無ければ止まり、人の go で `--fresh`（新しいセッション。前の `report.md` を読ませる）。Claude 系は起こすときに Andon がセッションの id を決めて台帳の `session` に書く（Codex 系は `resume` のときに記録から引く）。ラベル `session=<id>` を足し、template の `command`・`resume:` の行が `andon issue session` で読む。`start`・`resume`・`finish` は台帳ごとの issue lock を持つ。`andon chief-engineer` は戻った端末を片付けなくなった。レビュアーは対象外。**dispatcher はまだ自動では打たない**（打つのは Chief of Staff か人。[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) 決めたこと 3・7・9・11〜14。#129）。
- **worktree の置き場が `/workspace/.worktrees/<repo 名>/<slug>-<id>` に**（旧は `~/.herdr/worktrees`）。セルの repo の中なので、セルが起き直しても残る。置き場の根に `.gitignore`（`*`）を書くので、セルの git には出ない。`start`・dispatcher・`finish` の 3 か所の置き場を揃えた（[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) 決めたこと 6。#123 #125）。
- **台帳の `not_before`**: `YYYY-MM-DD` を書くと、`kanban ready` がその日まで出さない（status も `after` も変えない）。日は `config.yaml` の `timezone`（無ければ UTC）で読む。`kanban lint` が日付でない値を報告する（[ADR-0011](docs/adr/0011-signals-and-value-loop.md) 決めたこと 7。#119）。
- **`report.json` に `observation`**: 観察の課題の報告で、価値の仮説と信号で確かめた結果（`hit`・`miss`・`unknown`）と根拠を書く。任意。ラインは結果で何もしない（ADR-0011 決めたこと 7。#121）。
- **信号の記録の schema**: `schema/signal.schema.json`（`<slug>.signals.json`）、通知 `signal`、`project.json` の `signals`（外の信号の出所）。**schema と受け口だけで、信号を取りに行く command はまだ無い**（ADR-0011 の S1。#127）。
- **用語集**: Chief Engineer の略称 CE、カタカナの決まり、「人への書き方」の節（結論を先に・文で書く・括弧は識別子だけ）。CE の問いは ask-one-by-one の §2 に（[ADR-0013](docs/adr/0013-short-role-names-and-plain-japanese.md) 決めたこと 2・3・5・6。#126）。
- **ADR と決定の記録の語を置き換えた**: ふだんカタカナで言う語をカタカナに戻した（費用→コスト・持ち主→オーナーなど）。凍結した行にも及ぶが、語だけで決定の中身は変えない（ADR-0013 決めたこと 1・4。#128）。
- **文書**: ADR-0012（resume と置き場。#123）と ADR-0013（役の呼び名と人に向けた日本語。#124）を足した。ADR-0012 は提案、ADR-0013 は決定（状態は各 ADR の冒頭）。ADR-0011（提案）の実装はここまでで S1（schema）・`not_before`・`observation` だけ。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、足したキー（`session`・`not_before`・`observation`・`signals`・通知の `signal`）は任意で、前の版の台帳・`library/` はそのまま読める。**互換のための手作業は無い**。
- **`<profile>-resume` の template は `rebuild` の boot が作る**（boot は起動のたびにプロファイルを作り直す）。別に boot をやり直す手は要らない。ただし、セルの `templates.local/` に `claude`・`sonnet`・`fable`・`codex`・`chief-engineer` を置き替えた複製があれば、それには `resume:` の行が無く、`<profile>-resume` が作られない（`resume` が「template が無い」で止まる）。複製に `package/templates/` の `resume:` の行を足す。
- **動いている課題の worktree は旧の置き場のまま**（`~/.herdr/worktrees/…`）。台帳の `worktree` の値が正で、動き続ける。`andon issue finish` は旧の置き場の値も対として受けるが、**1 版だけ**（[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) 決めたこと 6）。旧の置き場は image の外にあるので、セルが起き直すと消える。上げるときに `doing` の課題が残っていれば、先に片付けるか、上げたあとに `resume` で戻す（`resume` は台帳の値の旧の置き場のまま戻す）。新しく起こす課題から `/workspace/.worktrees` になる。
- **上げる前に起こした担当は、台帳に `session` が無い**。そうした課題を `resume` すると「セッションを記録する前に起きた課題」で止まる。続けるなら人の go で `andon issue resume --fresh <id>`。上げたあとに `start` した課題から続きから起こせる。
- **セルの repo で `git clean -x`（`-dx` も）を打たない**。`/workspace/.worktrees` は `.gitignore` で除いているので、`git status` には出ないが、`git clean -x` は ignore を無視して worktree ごと消す（[ADR-0012](docs/adr/0012-resume-and-worktree-home.md) 決めたこと 13 の 6）。消す前に `git clean -ndx` で確かめる。
- `not_before` を書くのは、台帳の `kanban` がこの版以降のとき。ファイルを自分で読む板（mod など）は知らないキーを無視するので、日付の前でも `ready` に見える。`kanban ready` だけが止める（[docs/ledger.md](docs/ledger.md)）。
- 外の信号の出所（`project.json` の `signals`）は、**image を上げてから**書く。古い版の `andon` は知らないキーとして `project.json` ごと弾き、担当を一切起こせなくなる（[contract.md](docs/contract.md) §11・[docs/operate.md](docs/operate.md)）。信号を取りに行く command はこの版には無い。
- 起き直したあとの扱いは [docs/operate.md](docs/operate.md) 冒頭「役の配置と起動（一覧）」の担当・Chief Engineer の行（`resume`）。

## v0.0.5 — 2026-11-10

人の入口を Chief of Staff にし、マージの検知・人の手直しの記録・流れの数え・レビューを詰まらせない工夫が入った版。**台帳の形式・schema の版は変わらない**（足したキーは全部任意）。

### 変わったこと

- **Chief of Staff が人の既定の入口に**: 判断が要る話は Chief of Staff が `andon chief-engineer --brief -` で Chief Engineer に渡し、経緯は依頼書で引き継ぐ。Chief Engineer の「1人の検査」を外し、stop-check は Chief Engineer の課題を止めない。CoS skill を人の入口に書き直した。Chief Engineer の skill は WIP の行を外し、価値の仮説の席と、文書なしで返す段を足した。設計書 §1.4・§1.8 も入口は Chief of Staff に（[ADR-0009](docs/adr/0009-cos-default-entry.md)。#101 #106 #110 #115 #117）。
- **人が GitHub でマージした課題を dispatcher が拾って片付ける**: `review` の課題の PR が MERGED なら `andon issue finish` までし、通知 `merged`（`by: person`）を出す。マージなしで閉じた PR は通知 `closed` を 1 回だけ（#114）。
- **`finish` が squash マージ済みの PR の branch を消す**（#116）。
- **レビューを詰まらせない**: 指摘の `preexisting`（課題の前からあった問題。PR が足した行なら数える）、2 回目以降のレビューは直しの確認だけ（新しい指摘は critical だけを数える）、レビュアーが使用量の上限で止まったら別ブランドが保てるときだけもう一方のレビューのプロファイルで起こし直す（#109）。
- **人の手直しを集める**: 課題が `done` になったとき、dispatcher が `library/corrections.json` に 1 回だけ集める（人の答え・PR へのコメントと commit・ゲートの指摘への上書き）。集められなければ通知 `corrections`（#105）。
- **`andon measure`**: 台帳と記録から、価値 → 流れ → 効率の順にラインの流れを数える（待ち時間・流量・WIP・対抗の指標。`--json` の出力は `schema/measure.schema.json`）（#103 #112）。
- **`andon issue start --dry-run [--json]`**: 担当を起こす前に、渡す依頼書・ラベル・確かめの結果を出す。hc を呼ばず何も作らない（#108）。
- **`report.json` に `risk`**: 担当の一方通行の申告（`one_way: true` でゲートの段階が C に上がる。下げる向きには使わない）。`acceptance[]` を担当が使い始める（#107）。
- **`andon boot` が git の既定の作者を書く**: 案件の repo は `andon.toml` の `owner`、セルの repo は owner でない作者に。人の commit はホストの checkout か、セルの中で作者を明示して打つ（#113）。
- **App の inbox**: Chief of Staff と Chief Engineer のアイコンを役ごとに分けた（#100）。
- **直し**: `tests/run` が並べる前に `mise install` を一度打つ（#104）。公開の検査が当たりの中身をログに出さない・前身の台帳の番号 `Y-<数>` の説明（#94）。
- **文書**: image は Public で `docker login` 不要に合わせた（#95）。提案の ADR を足した: 通知を「記録だけ」と「届ける」に分け `<no-reply>` を `andon news ack` に替える（[ADR-0010](docs/adr/0010-news-ack.md)。#102）、信号の受け口と価値のループ（[ADR-0011](docs/adr/0011-signals-and-value-loop.md)。#111）。この 2 つは**提案で、まだ動かない**。

### 上げるときにすること

- image を上げて `rebuild` する。台帳の形式（3）・schema の版（`/2`）は変わらず、足したキー（`preexisting`・`risk`・通知の `closed`／`corrections`）は任意で、前の版の台帳・`library/` はそのまま読める。dispatcher の控え（`state.json`）に足したキー（`corrections`・`watch`）も無くてよい。**互換のための手作業は無い**。
- **App の mod を置き直す**: inbox（`main.js`・役ごとのアイコンの `.svg`）が変わった。`host-install` は今あるファイルを上書きしないので、App の設定フォルダの `mods/inbox` を退避してから `bin/bootstrap host-install` を打つ（[docs/operate.md](docs/operate.md) の「Herdr App の mod とレイアウト」）。置き直さなければ、前のアイコンのまま動く。
- セルの中の git の既定の作者が変わる（`andon boot` が書く）。人が owner として commit するのはホストの checkout か、作者を明示して打つ（[docs/operate.md](docs/operate.md) §10）。

## v0.0.4 — 2026-11-07

課題（issue）への改名、`kanban/` への台帳の移動、セルの repo の構成、調べる・決める・直すの分け方が入った版。**前の版の台帳をそのままでは読まない。**

### 変わったこと

- **カードの名前が「課題（issue）」に**: 台帳の `tasks/<id>/task.yaml` が `issues/<id>/issue.yaml` に、command が `andon issue …`、端末のラベルが `issue=`、schema が `/2` に、台帳の形式が 3 になった（[ADR-0008](docs/adr/0008-issue-naming.md)。#75 #86 #87 #88 #89 #90）。
- **台帳の置き場が `kanban/` に**: `projects/<slug>/.flightdeck/` から `projects/<slug>/kanban/` へ。`coordinator` は `chief_of_staff` に、課題の id は 4 桁（`A-0005`）に。`kanban migrate` で移す（#60）。
- **セルの repo と `projects/<slug>/` の構成**: セルを 1 つの repo（`<slug>-cell`）として持ち、台帳は git の外に置く。「建屋」の語は「セル（cell）」に（[ADR-0004](docs/adr/0004-project-layout.md)・[ADR-0005](docs/adr/0005-cell-repo.md)。#52 #56 #57 #58 #59 #84）。`artifacts/` は `files/` に改め、`andon publish` に大きさの上限を入れた（#66）。人のファイルの書き換えは managed settings の deny で拒み、auto memory も止める（#56 #59）。
- **調べる・決める・直すを分ける**: 課題の型・候補の一覧（`andon issue candidates`）・止まり方・`deliverable`（[ADR-0006](docs/adr/0006-survey-fix-decide.md)。#69 #76 #77 #80 #81 #85）。
- **`andon create` と `andon open [slug]`**: プロジェクトを対話で立てる。プロジェクトを選んで Chief of Staff を開く（#78 #71）。
- **dispatcher の強化**: 1 巡の順序を表で固定し、二重に走らせない守りを書いた。provider に切られたときの再試行を持たせた。レビュアーの課題の id は `<親>-r<回>` に（#65 #70 #74 #67）。マージのあと、動いている課題への影響を Chief of Staff が判断できる（#82）。
- **ゲート**: 記録の `at` を checks を読んだ時刻にし、通した checks の名前を全部残す（#64）。
- **App の mod とレイアウトを取り込んだ**: herdr-flightdeck の mod 5つ（`andon`・`inbox`・`kanban-board`・`context-header`・`files-plus`）が Andon の `host-install` で入る（#73）。mod は課題をラベル `issue=` で読む（#87）。
- **文書**: 決定の記録は `docs/decisions/` に 1 件 1 ファイルで足し、[decisions.md](docs/decisions.md) の表は凍結(#92)。ビジョン・ミッション・バリューと図（[docs/vision.md](docs/vision.md)）、別のマシンへ引っ越す手順、「ルーチン」の表記（#79 #68 #62）。
- **直し**: boot が前に出ているものを取れるまで読み直す（#63）。CI の tests の job を短くし、手元で `tests/run --changed` が使える（#61）。

### 上げるときにすること

- **互換は無い**。形式 2 の台帳は `kanban`・`andon`・dispatcher・ゲートのどれも読まず、1 行で止まる。
- 前の版（v0.0.3 以前）から上げる: 先に台帳を `.flightdeck/` → `kanban/` に移す（[docs/operate.md](docs/operate.md) の「前の置き場から移す」）。
- **台帳を形式 3 に手で移す**: [docs/operate.md](docs/operate.md) の「台帳を形式 3 に手で移す」の 9 手順（動いている課題の片付け → image を上げる → `tasks/` を `issues/` に → `version: 3` → `library/` の値 → dispatcher の控えを消す → `project.json` → mod の置き直し → `andon boot`）。
- **App の mod を置き直す**: `host-install` は今あるファイルを上書きしないので、`mods/` の Andon の mod 5つを消してから `bin/bootstrap host-install` を打つ（上の 8 番目の手順）。

## v0.0.3 — 2026-11-06

常駐の係の名前を dispatcher に改め、レビュアーの作業場所の記録を直した版。

### 変わったこと

- 常駐の係の名前を courier（伝令）から **dispatcher** に改めた（#53）。
- レビュアーの `kanban start` を、レビュアーの detached worktree で打つ。`andon task finish` は、台帳の `worktree` がセルの本体の checkout のときは何も消さずに止まる（#51）。
- 文書の脱字の直し（#48 #49）。

### 上げるときにすること

- image を上げて `rebuild` する。常駐の係の名前が変わったので、動いている旧名の係は起こし直す（[docs/operate.md](docs/operate.md) の「前の名前の係から移す」）。

## v0.0.2 — 2026-11-06

作り直しで戻った端末の見誤りを直した版。

### 変わったこと

- 作り直しで herdr-core が戻した、中身がシェルの端末を、伝令・Chief of Staff・Chief Engineer が動いていると見誤らない（#47）。

### 上げるときにすること

- image を上げて `rebuild` する。

## v0.0.1 — 2026-11-06

P7（セルを立てて 1 本流す）で見つかった直しを入れた最初の版。それまでの commit は、設計書と機械向けの約束、Go の `andon`、image、セルの雛形、クーリエ（今の dispatcher）、skill の配布、Space の割り方（[ADR-0001](docs/adr/0001-skills-distribution.md)・[ADR-0002](docs/adr/0002-herdr-spaces.md)・[ADR-0003](docs/adr/0003-task-spaces.md)）の土台。

### 変わったこと

- `vX.Y.Z` の tag を push したときだけ GHCR へ image を出す。main へのマージでは出ない（#39 #37）。
- ゲートが PR の head の GitHub の checks（CI）を見てからマージを許す。`gh pr merge` は rules で拒む（#42 #41）。
- 作業側の Space を課題（worktree）ごとに、伝令を指揮側の Space の tab に（#43 #45）。
- 伝令が止まらない・止まっても取りこぼさない。届かなかった最初のメッセージを送り直す（#40 #35 #44）。
- セルの SSH・bash・git の資格情報: 手元から SSH で入る、ホスト鍵を volume に、作り直しても資格情報とログインシェルが戻る（#24 #27 #30 #38 #32）。
- 手順書 [docs/operate.md](docs/operate.md) と README、SECURITY.md（#17 #29 #33 #46）。

### 上げるときにすること

- 最初の版。[docs/operate.md](docs/operate.md) の手順で立てる。
