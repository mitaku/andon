# CHANGELOG

版ごとに、人の目に効く変更と、上げるときにすることを書く。細かい変更は各版の範囲の commit（`git log <前の版>..<版>`）で遡れる。`(#n)` は PR の番号。

- 書くのは**版を切るときだけ**。PR ごとには触らない（手順は [docs/operate.md](docs/operate.md) の「版を切る」）。
- 互換の約束はまだ無い（[README](README.md) の Status）。「上げるときにすること」に無い限り、版を上げて `rebuild` するだけでよい。

## v0.0.4 — 2026-11-07

課題（issue）への改名、`kanban/` への台帳の移動、セルの repo の構成、調べる・決める・直すの分け方が入った版。**前の版の台帳をそのままでは読まない。**

### 変わったこと

- **カードの名前が「課題（issue）」に**: 台帳の `tasks/<id>/task.yaml` が `issues/<id>/issue.yaml` に、command が `andon issue …`、端末のラベルが `issue=`、schema が `/2` に、台帳の形式が 3 になった（[ADR-0008](docs/adr/0008-issue-naming.md)。#75 #86 #87 #88 #89 #90）。
- **台帳の置き場が `kanban/` に**: `projects/<slug>/.flightdeck/` から `projects/<slug>/kanban/` へ。`coordinator` は `chief_of_staff` に、課題の id は 4 桁（`A-0005`）に。`kanban migrate` で移す（#60）。
- **セルの repo と `projects/<slug>/` の構成**: セルを 1 つの repo（`<slug>-cell`）として持ち、台帳は git の外に置く。「建屋」の語は「セル（cell）」に（[ADR-0004](docs/adr/0004-project-layout.md)・[ADR-0005](docs/adr/0005-cell-repo.md)。#52 #56 #57 #58 #59 #84）。`artifacts/` は `files/` に改め、`andon publish` に大きさの上限を入れた（#66）。人のファイルの書き換えは managed settings の deny で拒み、auto memory も止める（#56 #59）。
- **調べる・決める・直すを分ける**: 課題の型・候補の一覧（`andon task candidates`）・止まり方・`deliverable`（[ADR-0006](docs/adr/0006-survey-fix-decide.md)。#69 #76 #77 #80 #81 #85）。
- **`andon create` と `andon open [slug]`**: プロジェクトを対話で立てる。プロジェクトを選んで Chief of Staff を開く（#78 #71）。
- **dispatcher の強化**: 1 巡の順序を表で固定し、二重に走らせない守りを書いた。provider に切られたときの再試行を持たせた。レビュアーの課題の id は `<親>-r<回>` に（#65 #70 #74 #67）。マージのあと、動いている課題への影響を Chief of Staff が判断できる（#82）。
- **ゲート**: 記録の `at` を checks を読んだ時刻にし、通した checks の名前を全部残す（#64）。
- **App の mod とレイアウトを取り込んだ**: herdr-flightdeck の mod 5つ（`andon`・`inbox`・`kanban-board`・`context-header`・`files-plus`）が Andon の `host-install` で入る（#73）。mod は課題をラベル `issue=` で読む（#87）。
- **文書**: ビジョン・ミッション・バリューと図（[docs/vision.md](docs/vision.md)）、別のマシンへ引っ越す手順、「ルーチン」の表記（#79 #68 #62）。
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
