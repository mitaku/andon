# CHANGELOG

版ごとに、人の目に効く変更と、上げるときにすることを書く。細かい変更は各版の範囲の commit（`git log <前の版>..<版>`）で遡れる。`(#n)` は PR の番号。

- 書くのは**版を切るときだけ**。PR ごとには触らない（手順は [docs/operate.md](docs/operate.md) の「版を切る」）。
- 互換の約束はまだ無い（[README](README.md) の Status）。「上げるときにすること」に無い限り、版を上げて `rebuild` するだけでよい。

## v0.0.6 — 2026-11-10

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
