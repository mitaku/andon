# 決定の記録

Andon の決定を、日付と出典つきで 1 行ずつ残す。**上書きしない。覆したら下に新しい行を足し、古い行に「→ 覆された」と書く。**

出典の書き方:

- **設計書 §6 問いN** — [design.md](design.md) の §6 の問いと、その「人の答え」の列。
- **ADR-NNNN** — [adr/](adr/) の文書（Chief Engineer の題。design.md §1.8）。ADR 1本につきこの記録には1行（要点と番号）。各決定の中身・理由・捨てた案は ADR が正。
- **仮設の記録（private）** — 仮設の工場（Andon の前身の実装）の設計文書・ゲートの文書・トリアージの文書。private で、public の読み手には届かない。決定の中身はここに写してあるので、設計の正はこの文書。仮設の語（yard・コーディネーター・段 A/B/C・知らせ）は、Andon の語（建屋・Chief of Staff・レベル A/B/C・通知）に直して写す。対応は design.md §2.4。

## 1. 設計書 §6 の人の答え（27 問）

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | モチーフ: 意味は三色ランプ（緑＝流れている・黄＝人の判断待ち・赤＝止まった）、絵は行燈 | 設計書 §6 問い1 |
| 2026-11-05 | 役の識別子は `chief-of-staff`・`chief-engineer`・`worker`。係は `courier`（`runner` は CI の runner と紛れる。日本語の説明は水すまし） | 設計書 §6 問い2 |
| 2026-11-05 | 「yard」は仮設の語として対応表へ。建屋の repo は `andon-<slug>`。セルフホストの建屋は `home`（repo `andon-home`） | 設計書 §6 問い3 |
| 2026-11-05 | 仮設でやる範囲: Chief of Staff がゲートでマージする経路はやる。乾いた走り 5 本は Andon の PR 5 本で兼ねる。ほかは凍結（保守タスクの整理は Chief of Staff） | 設計書 §6 問い4 |
| 2026-11-05 | base image は `ghcr.io/<owner>/andon`（private）。台帳の CLI は image に焼く（P5 までは CI の token で release から。移した後は同じ repo で作る） | 設計書 §6 問い5 |
| 2026-11-05 | 台帳の CLI と形式を Andon のモノレポに移す。名前は `kanban`、形式 v2 はそのまま、段取り K1〜K8 は持ち主と。時期は P5 のあと | 設計書 §6 問い6 |
| 2026-11-05 | 案件 repo の `AGENTS.md`・`REVIEW.md` はレベル B に下げる（独立レビュー付き）。Andon の repo と建屋の repo はレベル C のまま | 設計書 §6 問い7 |
| 2026-11-05 | ダブルループの構造に名前を付けない。「インナーループ／アウターループ」と §1.4 の図で足りる。「変則スター」の語は使わない | 設計書 §6 問い8 |
| 2026-11-05 | 仮設の docs は Andon に移さず指す。決定だけ `docs/decisions.md` に写す | 設計書 §6 問い9 |
| 2026-11-05 | 建屋の repo の自動 push は捨てる（commit と push は Chief of Staff や Chief Engineer が意識して行うことで、仕組みは要らない） | 設計書 §6 問い10 |
| 2026-11-05 | 文書は日本語から始め、必要になったら英語を足す。README の英語の要約・schema の `description` と識別子は英語 | 設計書 §6 問い11 |
| 2026-11-05 | 最初の版の Chief of Staff・フックは Claude Code だけ（Codex の同等物は後） | 設計書 §6 問い12 |
| 2026-11-05 | ライセンスは Apache-2.0。P1 で `LICENSE` を置く | 設計書 §6 問い13 |
| 2026-11-05 | 名前は「Andon」のまま。商標は取らない。README の冒頭で製造業のアンドン製品と区別する。public にする前に人が商標の登録簿をもう一度引く | 設計書 §6 問い14 |
| 2026-11-05 | public にするまでに要る相談 F10（台帳の CLI の配布）は、台帳の CLI を移せば Andon の中で片付く。F11 は `docs/` に写せれば足り、F12 は要らない | 設計書 §6 問い15 |
| 2026-11-05 | 公開する時期は P8（セルフホストに移る）と同時。公開の直前に履歴全体を `tests/public.sh` で検査する | 設計書 §6 問い16 |
| 2026-11-05 | 名前は、意味が重なる部品だけ TPS の語（かんばん・水すまし・主査）で呼ぶ。識別子は、ローマ字が英語になっている語だけローマ字（`kanban`）、ほかは英語（`courier`・`retro`）。TPS の語とソフトの系譜の語が競えば系譜の語 | 設計書 §6 問い17 |
| 2026-11-05 | Chief Engineer は英語のまま、識別子 `chief-engineer`（TPS の「主査」は出典として括弧に） | 設計書 §6 問い18 |
| 2026-11-05 | 参照した「Pstack」は poteto 作の pstack（Claude Code の plugin）。Chief Engineer の問いは、互いに依らないものをまとめて出す（1 問ずつは `andon.toml` で切替） | 設計書 §6 問い19 |
| 2026-11-05 | Chief Engineer は台帳のタスクとして回す（道具が担当と同じ。履歴が残る） | 設計書 §6 問い20 |
| 2026-11-05 | 案件のマスタの書き手: 合意事項と用語は Chief Engineer、Chief of Staff の設定と wiki は Chief of Staff | 設計書 §6 問い21 |
| 2026-11-05 | observer（自己改善）の出口は PR だけ。案件側は Chief of Staff の判断、Andon のラインは人 | 設計書 §6 問い22 |
| 2026-11-05 | 監視の入口は P8 の後。まず案件 repo の main の CI の失敗、次にエラー監視の webhook。利用の統計は入れない | 設計書 §6 問い23 |
| 2026-11-05 | `andon` の言語は Go（台帳の CLI と同じ module。1 バイナリ） | 設計書 §6 問い24 |
| 2026-11-05 | 委譲のファイルは建屋の repo の `projects/<slug>/delegation.json`（人だけが書ける。履歴が git） | 設計書 §6 問い25 |
| 2026-11-05 | 型 `hobby` は全部任せる（public への公開・production へのデプロイも。戻せない公開も含む）。「何を作るか」と Andon 自身のパッケージが常に人、はそのまま | 設計書 §6 問い26 |
| 2026-11-05 | 委譲の期限は無し（`until: null`）が既定。型が無いプロジェクトは `work`（安全側） | 設計書 §6 問い27 |

## 2. 仮設の設計文書の決定（Andon の像と形）

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-04 | 騙す担当は相手にしない。段 1 まで（来歴は固定のプログラムが作る。別 UID は後から足せる形にだけ）。偽ろうとするならもっと簡単に偽れるので、塞いでも割に合わない | 仮設の記録（private）D1 |
| 2026-11-04 | 人が Chief of Staff に話す口は、まず各建屋の Chief of Staff の端末。最終的には通知から返す。問いは今から「1 通で答えられる形」に揃える | 仮設の記録（private）D2 |
| 2026-11-04 | クラウド常駐の建屋を射程に入れる（手元＋クラウド）。使い捨てスロットまでは行かない | 仮設の記録（private）D3 |
| 2026-11-04 | 最初の到達点は「手元の建屋で人無しで 1 本流れる」。クラウドと通知はその後 | 仮設の記録（private）D4 |
| 2026-11-04 | ゲートが通したものは自動でマージする。人はレベル C と不可逆な操作だけ見る | 仮設の記録（private）D5 |
| 2026-11-04 | 目指す像: 判断だけする人、と、建屋が増える・散らばる | 仮設の記録（private）D6 |
| 2026-11-04 | コンセプトは選択的 Dark Factory（止められるものは機械、判断が要るものは LLM、残りだけ人）。出典: freee の講演 | 仮設の記録（private）D7 |
| 2026-11-04 | 人に灯りをつける判断は 5 種: 事業判断・利用者の挙動・外との契約や制約・不可逆なリスク・ラインそのものの変更（レベル C） | 仮設の記録（private）D8 |
| 2026-11-04 | 内側のループ（タスクのライン）と外側のループ（retro）の両方を回す設計にする。出典: mattpocock の chief-of-staff・retro | 仮設の記録（private）D9 |
| 2026-11-04 | retro の改善を入れる判断はラインを 2 つに分ける: 案件 repo の検査・レビュー基準・指針は Chief of Staff の判断で人の手を借りない。建屋自身のライン（ゲート・SKILL・規約）の変更だけ人 | 仮設の記録（private）D10 |
| 2026-11-04 | 形は、内側はスター、外側の所見のうち建屋のラインに関わるものは建屋の Chief of Staff に集める（→ 2026-11-05 に構造の名前は付けないと決定。§1 問い8） | 仮設の記録（private）D11 |
| 2026-11-04 | 知識の置き場: wiki を建屋の側に置く。範囲は建屋を横断する知識だけ。案件のドメイン知識は案件側で各 Chief of Staff が持つ | 仮設の記録（private）D12 |
| 2026-11-04 | 名前: 「コーディネーター」は合わない。Chief of Staff を案として書き、最終決定は人（→ 2026-11-05 に決定。§1 問い2） | 仮設の記録（private）D13 |
| 2026-11-04 | 末端（担当・レビュアー）は指揮と別のマシンに出せる形にする。台帳は指揮のマシンに 1 つ。末端は台帳に直接触らず、作業の入口を command に閉じる。台帳の形式は変えない | 仮設の記録（private）D14 |
| 2026-11-04 | 「ハーネスとしては薄く、ゲートやループの質を高める」。画面や文（Markdown・ANSI）から推す層を厚くせず、機械が読むものは機械向けの約束にする。背景: Markdown や画面の文を正規表現で読む形は、レビューを重ねても端が尽きなかった | 仮設の記録（private）D15 |

## 3. ゲートの決定（自動マージ）

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-02 | Chief of Staff が人の確認を待たずにマージしてよい。自動マージゲートを構築してよい（2026-11-03 に、下敷きは他社の公開されたリスク判定と Anthropic の code review と決定） | 仮設の記録（private）ゲート D0 |
| 2026-11-03 | レベル A のマージは、依頼の go ＋ Chief of Staff の検収 ＋ 検査 green でよい。危ない面はレベル B・C で絞る | 仮設の記録（private）ゲート H1 |
| 2026-11-03 | GitHub のプランは上げない。現在の規模ではゲートで運用する | 仮設の記録（private）ゲート H2 |
| 2026-11-03 | 【仮】skill のファイルはレベル B（独立レビュー）のまま。入れ子も同じ。ハーネスの設定ディレクトリ全体をレベル C とする規則には、skill の例外を設ける | 仮設の記録（private）ゲート H3 |
| 2026-11-03 | 【仮】乾いた走りは PR 5 本（人が後で変えうる） | 仮設の記録（private）ゲート H4 |
| 2026-11-03 | レベルの決め方: レベル A を明示的な許可リストにし、未知のパスは最低 B。エージェント・外部ボットが読む設定・規約は深さを問わず C | 仮設の記録（private）ゲート D1 |
| 2026-11-03 | tests の分類: 行数の除外だけ。tests 配下でも skill は B・設定は C、実行される tests のスクリプトは B。独立レビューが要るときは、レビューの確認を先に読み、current でなければ検査を回さない | 仮設の記録（private）ゲート D2 |
| 2026-11-03 | レビュー作成者の偽装は、残る危険として受け入れコードでは塞がない。ゲートが相手にするのは間違える担当で、騙そうとする担当ではない。守りは自動セキュリティレビューとレベル C | 仮設の記録（private）ゲート D3 |
| 2026-11-04 | ゲートが読むもの（依頼書の Settings・レビューの指摘の数と重さ）は Markdown でなく機械向けの形で持つ。Markdown の依頼書・報告は人が読むためのもの | 仮設の記録（private）ゲート D4 |
| 2026-11-06 | ゲートは PR の head の GitHub の checks（`statusCheckRollup`。自分の `andon/gate` は除く）を事実に入れる。落ちた（failure・cancelled・timed_out・startup_failure）は `fix`（担当が直す）、action_required・知らない値は `needs_human`、走っている・1つも無いは `wait`（記録しない。クーリエが見回りのたびに回し直す）。neutral・skipped は通す。1つも無いまま review から `github_checks_wait_minutes`（既定 30）を過ぎたら `needs_human`、CI の無い repo は条件表の `github_checks: optional`。走っているものは時間で人へ上げない。クーリエはマージの直前に head と checks を読み直し、変わっていれば打たない | 人・Chief of Staff（2026-11-06。private の repo でブランチ保護が効かず、CI の落ちた PR を伝令がマージしうる）、Y-142 の依頼書の推し |
| 2026-11-06 | 上の行に足す: checks は100件ずつ全ページ読み、同じ check の実行は最新の1つだけを数える（workflow は表示名でなく `Workflow.id` で分け、新旧を判別できないときは悪い方を残す。レビュー 2 回目〈Y-153〉の major 2件）。マージの直前の読み直しと `gh pr merge` の間に同じ head で checks が変わる隙間は受け入れる（サーバー側で強制できるのはブランチ保護だけで、private の Free では使えない）。public にしたら ruleset で required checks を付けて塞ぐ | Y-142 の独立レビュー 1 回目（Y-150）の major 3件と Chief of Staff の振り分け |

## 4. トリアージの決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-03 | 溜まったものをエージェントだけで仕分けてよい。不要なものは不要として処理し、直すものは直して入れる | 仮設の記録（private）トリアージ D0 |
| 2026-11-03 | 【仮】周期は週 1 回（月曜の朝） | 仮設の記録（private）トリアージ P1 |
| 2026-11-03 | 【仮】直す・まとめるの上限は 1 回 3 タスク | 仮設の記録（private）トリアージ P2 |
| 2026-11-03 | 【仮】不要の確認は事後の要約で見る（閉じたものは戻せる） | 仮設の記録（private）トリアージ P3 |
| 2026-11-03 | 【仮】reflect はトリアージへ統合する | 仮設の記録（private）トリアージ P4 |
| 2026-11-03 | 【仮】積み重ねた PR（Stacked PR）は採らない。小さい PR を順に流す | 仮設の記録（private）トリアージ P5 |
| 2026-11-03 | 推奨案があり後で変えられる判断は、Chief of Staff が仮決めしてよい。人には人が要る判断だけ届ける | 仮設の記録（private）トリアージ P6 |

## 5. P1 での決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | 固有の値の禁止リストは repo の外に置く（手元は `~/.config/andon/public-denylist`、CI は secret）。リストが無いときは秘密の形だけ検査して通す。リストの語を履歴に残さない（design.md §2.6 の 1 を上書き） | 人の決定（P1 の依頼書の追記） |

## 6. herdr-flightdeck の決定（台帳・オーケストレーション）

台帳の CLI と形式の元の持ち主 herdr-flightdeck（private）の決定のうち、台帳に関わる D14・D16・D17・D20〜D22 を写す。写してよいと向こうの人が決めた（2026-11-05、§7 の 2026-11-05 の行）。仮設の語は Andon の語に直してある（コーディネーター→Chief of Staff、yard→建屋、知らせ→通知）。日付は向こうで決めた日。出典の「herdr-flightdeck の記録（private）」は private で、public の読み手には届かない。決定の中身はここに写してあるので、設計の正はこの文書。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-02 | 柱 4: 計画は `.flightdeck/` の台帳に置き、変えるのは小さな CLI（`deck`。ロック・検査・ログ）、板は読むだけ。1 タスク 1 フォルダ（`task.yaml`・`brief.md`・`report.md`・`library/`）。カードは担当の状態を出し、そこへ飛べる。理由: 計画はコードのそばに。1 つのファイルを多数の worktree で共有すると書き込みが失われるがフォルダなら失われない。CLI は全エージェントが呼べる。板は見るだけで、mod に書き込みを増やさない。置き場所は D17 で替わった | herdr-flightdeck の記録（private）D14 |
| 2026-11-02 | オーケストレーションは建屋の側に置き、台帳の道具の側には置かない。依頼を割る・worktree とエージェントを付ける・報告を集める・レビューに送るのは、建屋の skill（担当用と Chief of Staff 用）が `deck` と `hc` の上で行う。道具の側が出すのは `deck` と mod と台帳の形式（契約）だけで、形式の変更は版を付けて告げる。理由: 仕事の組み方は建屋ごとに違い、よく変わる。道具と見え方は変わらない | herdr-flightdeck の記録（private）D16 |
| 2026-11-02 | 書く・長い・人が要りうる仕事は、サブエージェントではなく台帳のタスクにして、Chief of Staff が担当を起こす（worktree・プロファイル・報告が付き、板に出る）。担当の中のサブエージェントは、その turn のうちに返る短い読み取り（検索・探索・自分の差分の独立レビュー）だけで、書き込み・PR・エージェントの起動はしない。担当が仕事を割りたければ報告の `## Next` に書いて Chief of Staff に返す。Chief of Staff 自身は仕事をしない（タスクを切る読み取りのサブエージェントだけ可） | herdr-flightdeck の記録（private）D16 の 2026-11-02 の追記 |
| 2026-11-02 | 台帳は案件ごとに 1 つ、git の外に置く。建屋は複数の案件（`projects/<slug>/`）を持ち、案件が信頼境界で、Chief of Staff が 1 人（人が話す相手）、タスクごとに担当を起こす（スター型）。台帳は案件の `.flightdeck/` で、`config.yaml` に `repos` と Chief of Staff、タスクに `repo` と `profile`。全案件を見る側は全台帳を読むだけで書かない。プロファイルは建屋が定義し、台帳は名前を表示するだけ。理由: 台帳を建屋の根に 1 つ置くと、どの Chief of Staff も全案件のタスクを書けてしまい、「案件の仕事は案件の Chief of Staff のもの」が作法だけになる。D14 の「建屋の根に置く」を替える | herdr-flightdeck の記録（private）D17 |
| 2026-11-02 | 繰り返す仕事（ルーティン）は台帳のファイル `routines/<name>.yaml` に置き、`deck` が起こす。足すタスクと起動条件（時刻 `schedule` か、タスクがある状態に入ること `on`）を書く。時刻は建屋が数分ごとに回す `deck tick`、出来事はタスクを動かすコマンドが追加する。ルーティンごとに未完了は `open` 件まで（積み過ぎを防ぐブレーキ）。追加されたタスクに担当を起こすのは Chief of Staff。理由: 起動は台帳に対する決まった規則なので、プログラムが漏れも重複もなく、トークンを使わずに行う。何をいつやるかは人が書く | herdr-flightdeck の記録（private）D20 |
| 2026-11-03 | 人への問いは台帳に置き、人は Chief of Staff から聞く。担当は人が要るとき端末で聞かず `deck ask` で問いを書いて止まる。Chief of Staff は未回答の問いを全部読み、判断点でないものは自分で答え、残りを 1 つの文書にまとめて人に出す（事実・理由・問い・推し・問いごとの名前）。答えは `deck answer` で台帳に書き、担当へ渡し、担当が `received` にする。人が見る「要対応」は担当ごとでなく Chief of Staff の 1 つだけ。理由: 5 人が聞くと人は問いごとに文脈を替え、同じ問いが何通りにも来る。Chief of Staff の頭の中だけでまとめると、圧縮や再起動や取りこぼしで失われる。台帳なら、担当が答えを受けるまで問いは開いたままで、確かめは数えるだけでできる（文書に全問・全答が受領済み・問いのある停止中の担当が全て）。未確認: 担当の指示だけで端末で聞くのを防げるか（最後の確かめはそのため） | herdr-flightdeck の記録（private）D21 |
| 2026-11-03 | 通知は 1 つの配達役が Chief of Staff に届け、担当は端末に打ち込まない。担当の `hc send` が、人が書きかけの入力行に貼り付いて一緒に送られた。担当は台帳（`deck report`・`deck review`・`deck ask`）で止まり、`--no-notify` で起こす。建屋ごとに 1 つの配達役（旧 `yard-news`。Andon では `courier`）が、台帳の `log.jsonl` を前回の続きから読み、エージェントの状態（止まった・終わった・20 分変化なしの作業中）を見て、Chief of Staff が待機中で入力行が空のとき（`hc read` で確かめる。淡色の文字は空とみなし、書きかけは空とみなさない）、待っている全部を 1 つの `<news>` メッセージにして送る（データであり依頼ではない。足すことが無ければ `<no-reply>` と答える）。画面が読めない・`hc read` と食い違う・許可リスト外の制御列があるときは何も送らない。理由: 人が Chief of Staff の端末に直接打つので、書き手は 1 つで、入力行が空になるのを待たねばならない。台帳から拾うので、担当が言い忘れても失われない。未確認: Codex の入力行の見え方。向こうの `docs/yard-news.md` は 2026-11-03 の設計の記録で、正は Andon の `docs/` に移してよい（§7 の F8） | herdr-flightdeck の記録（private）D22 |

## 7. 相談 F1〜F12・K1〜K8 への回答で決まったこと

[design.md](design.md) §3.2・§3.4・§3.5 の相談への、herdr-flightdeck 側の回答（回答案を向こうの人が 2026-11-05 に決定）。台帳の CLI は `kanban` として Andon のモノレポに移る（§1 問い 5・6・15・16）。以下は向こうの決定の写しで、Andon 側の設計は design.md と contract.md が正。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | 移す時期は決定のまま（P5 のあと P7 の前）。早めない。P3a〜P3c の間だけ CI に読み取り専用の token を置いて release から取る。移したら token は消す（F9・K1） | herdr-flightdeck の記録（private）相談への回答 §4 の 1 |
| 2026-11-05 | 移すコードに Apache-2.0 を付けてよい。`cmd/kanban` は herdr-flightdeck の `cmd/deck`（tag）の写しで、写した日に Apache-2.0 を付けた、と Andon 側で出典を残す（F10） | herdr-flightdeck の記録（private）相談への回答 §4 の 2 |
| 2026-11-05 | 台帳の決定（D14・D16・D17・D20〜D22）を Andon の `docs/decisions.md` に写してよい。文書ごとでなく決定だけ。出典に private の印（F11。§6 が写し） | herdr-flightdeck の記録（private）相談への回答 §4 の 3 |
| 2026-11-05 | herdr-flightdeck（mod・Web ボード）を公開するかは、今は決めない。Andon の README は「あれば使える」と書く。Web ボードは凍結中で台帳を読まず、Andon の通知から返す口の候補に当たらない（F12） | herdr-flightdeck の記録（private）相談への回答 §4 の 4 |
| 2026-11-05 | 移す前に、台帳の CLI の最後の release `deck-v0.4.0` を出す（別タスク）。中身は形式 v2 に足すだけの変更: 問いの `kind` に `line`（F3）、`agent` の説明の明確化（F4）、予約キーの注記（F6）、schema の一覧の修正。`deck-v0.4.0` から写し、写した時点で向こうの `cmd/deck` は凍結、以後の直しは Andon（K1・K7・K8） | herdr-flightdeck の記録（private）相談への回答 §4 の 5・K1・K8 |
| 2026-11-05 | `kind` に 5 つ目 `line` を受ける。移す前に台帳の側で足すので、P5 に `line` の例外が要らない（F3） | herdr-flightdeck の記録（private）相談への回答 F3 |
| 2026-11-05 | `task.yaml` の `agent` は、台帳と同じ machine なら `t3`、別の machine なら full address `<machine>/t3`。読み手は `/` を含む値を full address として引く。`machine` キーは足さない（`machine`＝どこで起こすか、`agent`＝どこで動いているか、は別物）（F4） | herdr-flightdeck の記録（private）相談への回答 F4 |
| 2026-11-05 | 代筆の印は `by` を変えず、`log.jsonl` の行に任意キー `for` を足す。移したあと Andon で（F5） | herdr-flightdeck の記録（private）相談への回答 F5 |
| 2026-11-05 | 予約キー（`review`・`must_fix`・`reviewer`・`review_rounds`・`merge`・`machine`・`pr`）を形式の文書に注記する。形式は読まないが、別の意味で使われないように。`pr` は仮設のゲートが読み、Andon では `report.json`（F6） | herdr-flightdeck の記録（private）相談への回答 F6 |
| 2026-11-05 | `kanban events`（`--follow`）は移したあと Andon で、courier（P6a）と同じ repo で作る。cursor は不透明な文字列（中身はバイト位置）、follow は数秒のポーリング。向こうでは作らない（F1） | herdr-flightdeck の記録（private）相談への回答 F1 |
| 2026-11-05 | 問いの形（`options[]`・`facts[]`・`reason`・`remaining`・`from`）を `questions.yaml` の形式の項目にするのは v3 で。それまでは `library/questions/<n>.json` のまま（F2） | herdr-flightdeck の記録（private）相談への回答 F2 |
| 2026-11-05 | 台帳の入れ物を DB・MCP にするかは今は決めない。決めるときの条件: 書き手は台帳の CLI 1 つのまま、読み手（板）の扱い（ファイルを残して DB は写し、または読み手も CLI／server 経由にして版を上げる）を先に決める（F7） | herdr-flightdeck の記録（private）相談への回答 F7 |
| 2026-11-05 | courier（旧 yard news）の設計の正は Andon の `docs/` に移してよい。向こうの文書は記録として残る。mod との約束のラベル `news.replied=<turn>` は向こうの持ち物で、変えるときは Andon から向こうへ知らせる（F8） | herdr-flightdeck の記録（private）相談への回答 F8 |
| 2026-11-05 | 移すまでの間、Andon の CI は読み取り専用の token で release から台帳の CLI を取って image に焼く。移したら同じ repo でビルドして token を消す（F9） | herdr-flightdeck の記録（private）相談への回答 F9 |
| 2026-11-05 | 公開の準備（F10・F11）は、台帳の CLI と形式が Andon に移れば Andon の中で片付く。`concept.md` は移さず公開しない。形式の文書と schema は移る（F10・F11） | herdr-flightdeck の記録（private）相談への回答 F10・F11 |
| 2026-11-05 | 形式の正は、移したら Andon の `docs/ledger.md` と `schema/ledger/`。向こうの `docs/ledger-format.md` は v2 の記録として残し、冒頭に正の移動を 1 行。向こうの schema は消す（K2） | herdr-flightdeck の記録（private）相談への回答 K2 |
| 2026-11-05 | 向こうの mod・Web ボードが依るのは台帳の CLI でなく**ファイルの形式**なので、CLI を移しても壊れない。移したあとの片付けは向こうの PR（CLI・テスト・CI・文書）（K3） | herdr-flightdeck の記録（private）相談への回答 K3 |
| 2026-11-05 | 最初の `kanban` の release は、`deck-v0.4.0` と同じ振る舞い。確かめは向こうのテストスクリプト（任意のバイナリに走る 103 件）をそのまま持ち込んで通す。bash への書き直しはその後で、書き直すなら同じ 103 件が同じ結果になることを並べて確かめる。`kanban --version` は新しく足す（K4） | herdr-flightdeck の記録（private）相談への回答 K4 |
| 2026-11-05 | 移す間の互換: `deck` の alias は 1 版だけ。形式 v2 のまま、`.flightdeck/` も `FLIGHTDECK_ROOT` も変えない（環境変数の名前は形式の一部）。改名は v3 と一緒で、alias の版では両方読む（K5） | herdr-flightdeck の記録（private）相談への回答 K5 |
| 2026-11-05 | v3 は版を上げないと変えられないものだけ: `.flightdeck/` → `.kanban/`、`FLIGHTDECK_ROOT` → `KANBAN_ROOT`、予約キーの正式化、問いの形（F2）。足す変更（F3〜F6）は v3 を待たない。移行の道具 `kanban migrate`（フォルダ名と `version` の書き換え）を付ける。向こうの板の対応は v3 の tag の前に Andon から知らせを受けて出す（K6） | herdr-flightdeck の記録（private）相談への回答 K6 |
| 2026-11-05 | 移したあとの台帳の CLI と形式の持ち主は Andon。知らせは双方向: 向こうから Andon へは要望を、Andon から向こうへは形式の変更（版・新しいキー・ラベル名）を tag の前に。向こうが持ち続けるのは mod の読み方と Web ボードの畳み方（K7） | herdr-flightdeck の記録（private）相談への回答 K7 |

## 8. P2 での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | schema は JSON Schema draft 2020-12（台帳の形式の schema と同じ）。`$id` は `https://andon.example.com/schema/<名前>/<版>` | P2 の担当の推し（contract.md §10） |
| 2026-11-05 | schema の検査は python3 の標準ライブラリで書いた小さな検査器（`tests/lib/schema_check.py`）。使うキーワードだけを実装し、知らないキーワードがあれば検査できないとして落ちる。建屋に JSON Schema の検査器が無く、勝手に入れないため | P2 の担当の推し（依頼書「道具は建屋に今あるもので」） |
| 2026-11-05 | schema は書き手の約束として知らないキーを拒む（`additionalProperties: false`）。台帳の項目（`task-keys`）だけは台帳の形式に倣って許す | P2 の依頼書の完了条件（知らないキーの例が落ちる）・contract.md §0 の 4 |
| 2026-11-05 | 台帳の項目の値は文字列（`kanban set` が文字列で書く）。`review_rounds` は `"2"` | P2 で `deck set` を実測 |
| 2026-11-05 | ゲートの記録の `review.state` に `off`（タスクが `review: off`）を足す | P2 の担当の推し（contract.md §7 に無かった） |
| 2026-11-05 | 検査器の pattern は ECMA-262 の意味に合わせる（`$` は末尾の改行の前に合わない）。`integer` は小数部が 0 の数（`1.0`）も受ける | P2 の独立レビュー 1 回目の指摘 |
| 2026-11-05 | 日付・時刻は暦にある値だけを通す（うるう年まで pattern で見る）。`findings[].file`・`waived[].file` は絶対パスと `..` の段を拒む。ゲートの記録の `review` は `state` ごとに `task`・`commit` の有無を決める | P2 の独立レビュー 1 回目の指摘 |
| 2026-11-05 | `tests/schema.sh` の必須キーの期待値は schema から読まずテストに固定し、制約を緩めた schema の写しで検査全体が落ちることを自己検査する | P2 の独立レビュー 1 回目の指摘 |
| 2026-11-05 | schema の pattern には ECMA-262 と Python で意味が割れる表現（`.`・`\s`・`\d`・`\w`・`\b`）を使わず、明示した ASCII の文字クラスで書く（lint が落とす）。検査器を ECMA-262 に寄せることはしない（`$` の置き換えだけ残す） | P2 の独立レビュー 2 回目の指摘・Chief of Staff の方針 |
| 2026-11-05 | 入れ子の object の必須のキーも、テストに固定した期待値で検査する（委譲の grant の `id` など） | P2 の独立レビュー 2 回目の指摘 |

## 9. P3a での決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | P3a では `deck` を image に焼かず、private release の取得用 token も置かない。台帳の CLI は P3c で同じ repo の `kanban` をビルドして入れる。§7 の F9 の暫定配布案をこの決定で置き換える | 人の回答（P3a の CI に必要な取得用 token の相談） |

## 10. P3b-2 での決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | schema の検査器を `tests/lib/schema_check.py` から `package/lib/andon/schema_check.py` に移す。P5 で `andon task report`・`ask` が command になるまで、担当・Chief of Staff・Chief Engineer は `report.json` と問いの JSON を jq で書き、建屋でこの検査器と `schema/`（image が `/opt/andon/schema` に置く。`ANDON_SCHEMA` で差し替え）に通す。tests も同じ検査器を使う | Y-71 のレビュー（Y-75）の指摘と Chief of Staff の方針（skill は contract.md・schema/ を正に書く） |
| 2026-11-05 | `andon chief-engineer` は `--repo <host/owner/repo>` を取り、その repo に worktree と branch を切って Chief Engineer を起こす（文書の PR と `report.json` を同じタスクから返す。design.md §1.8）。repo は台帳の `config.yaml` から推さない（台帳はファイルを直接読まない約束） | Y-71 のレビュー（Y-75）の指摘と Chief of Staff の方針 |
| 2026-11-05 | 上の行を改める: `andon chief-engineer "<題>"` は `--repo` を省ける。省いたら台帳の `config.yaml` の `repos` が1つのときその repo を使い、2つ以上（か無い・読めない形）なら `--repo <host/owner/repo>` を求める（終了コード 2）。`repos` は台帳の CLI が書く形（`repos:` の下の `  - <repo>`）だけを読む（台帳の CLI に repos を出す口が無いため。出す口ができたら替える） | Y-71 のレビュー 2 回目（Y-78）の指摘と Chief of Staff の推奨案 |

## 11. P5 での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | `andon gate` は判定と記録（`library/gate/<head>.json`・PR の commit status `andon/gate` とコメント）までで、**マージしない**。マージと finish はクーリエ（P6a） | contract.md §6・§7、courier.md §5 |
| 2026-11-05 | ゲートは PR の場所を `report.json` の `pr` だけから読む（`task.yaml` の `pr` は読まない）。`report.json` が無い・形が違う・台帳と PR が合わないときは、信じられる PR が無いので記録も GitHub への書き込みもせず `fix` | contract.md §4・仮設のゲート（template-yard #41） |
| 2026-11-05 | 委譲が無い（無い・読めない・author が owner でない）ときの `merge` は `none` で、ゲートはどのレベルでも `needs_human`（人がマージする）。conflict・base が既定ブランチでない・作者が建屋の gh アカウントでない・fork・files の件数の食い違いは、委譲の範囲に関わらず `needs_human`（仮設ではレベル C の理由だったもの。委譲 `merge: C` で通らないように分けた） | contract.md §7・§11、design.md §1.10 |
| 2026-11-05 | `must_fix` 以上の指摘を担当が `waived` に入れたら、ゲートは `needs_human`（担当がレビュアーを覆した。人が判断する）。`must_fix_open` は `waived` に無いものだけを数える。同じ head で Must fix が残って `fix` を返した記録があれば、2度目は `needs_human`（仮設の控えファイルを記録で置き換えた） | 担当の推し（chief-of-staff skill の数え方と schema の description に合わせた） |
| 2026-11-05 | `andon task review-request` の回は、レビュアーの子タスク（profile が `*-review`）の `findings.json` の `commit` のうち違うものの数 ＋ 1。`previous` は最新の `findings.json` の `commit`。古い commit を見たレビューは前の回と同じ commit なので回が増えない。`HEAD` のレビューがもうあれば頼まない | 担当の推し（`report.json` は毎回書き直すので、回を控える場所が無い） |
| 2026-11-05 | `andon task report` は `--pr` を省くと前の `report.json` の `pr` を引き継ぐ（ゲートが読むのはこれだけなので、書き忘れで落ちないように）。ほかのキーは毎回書き直す。`review-request` は前の `report.json` に `review_request` を足して書く | 担当の推し |
| 2026-11-05 | 建屋の repo は `ANDON_WORKSPACE`（既定 `/workspace`。boot と同じ）。`andon.toml` は最上段の `owner = "…"` の行だけを読む（TOML の依存を足さない）。委譲の author の照合は、commit の author の email が `owner` と同じこと（P4 の `andon.toml` の雛形の定義） | 担当の推し・P4 の `init/andon.toml.tmpl` |
| 2026-11-05 | `andon delegate` は `--project <slug>` を取る（省くと cwd の `projects/<slug>/`）。型（`preset`）を変える command は作らない（設計書に無い。人が JSON を書いて commit する。次の `andon delegate` が検査する） | 担当の推し |
| 2026-11-05 | 担当の command と CoS の検収の照合は、レビュアーのうち作成の行の `by` が `terminal:<coordinator>` のものだけを数える（仮設と同じ）。クーリエが起こすレビュアー（P6a）を数える形は P6a で足す | 仮設のゲート・担当の推し |
| 2026-11-05 | 建屋の `gate.local.json` も委譲と同じ読み方: 建屋の repo の HEAD のものを、最後に変えた commit の author の email が `owner` のときだけ重ねる（作業ツリー・未 commit の変更は読まない）。信じられなければ重ねず、repos が無いのでゲートは `needs_human` | P5 の担当の推し（自動のセキュリティレビューの指摘） |
| 2026-11-05 | 約束の JSON の入口（`report.json`・`findings.json`・問いの JSON・条件表・委譲・`--data`）は全部 `contract.Decode` を通り、存在するキーの null・型違い・知らないキーを拒む（schema と同じ）。null を許すのは tag `andon:"nullable"` のキー（`review_request.previous`・委譲の `until`・`revoked`）だけ。条件表の glob はリテラルを rune 単位で逃がす | P5 の独立レビュー 1 回目（Y-82）の指摘と Chief of Staff の方針 |

## 12. P3c での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | `cmd/kanban/` は herdr-flightdeck の `cmd/deck`（tag `deck-v0.4.0`）の写しで、写した 2026-11-05 から Apache-2.0（§7 の F10）。出典は `cmd/kanban/main.go` の冒頭と `tests/kanban/test-deck.mjs` の冒頭に書いた。変えたのは command の名前（help・使い方・エラーの接頭辞 `kanban:`）と文書への参照、`--version`（K4。版は `-ldflags "-X main.version=…"`、無ければ `dev`）だけ | design.md §3.5・§5.2 |
| 2026-11-05 | `deck` の別名は名前で振る舞いを変えない（同じバイナリ。help も同じ。help の末尾に別名の1行）。image は `/usr/local/bin/deck` → `kanban` の symlink。環境変数 `FLIGHTDECK_ROOT` と `.flightdeck/` は変えない（K5）。テストの時計 `DECK_NOW` も変えない（振る舞いテストが写しのまま通るように） | §7 の K5 |
| 2026-11-05 | 振る舞いテストは向こうの `test-deck.mjs` を写しのまま持ち込み（K4）、`tests/kanban.sh` が node で回す。bash への書き直しは後（設計書 §3.5 の `tests/kanban.sh` は今は node の入口）。node は CI では必須、手元に無ければ skip | §7 の K4 |
| 2026-11-05 | image の `kanban` は Dockerfile の中でビルドせず、`tests/image.sh` が焼いたバイナリを build context `kanban` で COPY する（設計書 §3.5「CI が焼いたバイナリを COPY」。Go の image を足さない） | design.md §3.5 |
| 2026-11-05 | `internal/` は台帳の CLI に `kanban` だけを打つ（`ledger.CLI`）。別名の `deck` には落ちない（PATH の `deck` が版違いでも名前だけで選んでしまうため。1回目のレビューの major）。無ければ「kanban が無い」と入れ方を案内して止める。image と CI は `kanban` を入れる。テストの偽物は `kanban` の名前で置く | 依頼書（internal/ の deck を kanban に）・Y-87 のレビュー |
| 2026-11-05 | `tests/andon-task.sh`・`andon-gate.sh`・`andon-triage.sh` は台帳の CLI を自分で焼く（`tests/lib/kanban.sh`）ので、go があれば CI の両方の workflow で skip しない。`tests/schema.sh` の command の出力の節も同じ。go の workflow は 4 target（linux/amd64・linux/arm64・darwin/arm64・windows/amd64）のビルドを確かめる。release は作らない（Andon の tag で一緒に。後の工程） | 依頼書の完了条件 |

## 13. P6a での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | 11 の「クーリエが起こすレビュアーを数える形は P6a で足す」を改める: クーリエは台帳の CLI を `HC_TERMINAL_ID=courier` で打ち（`by` は `terminal:courier`）、作成の行の `by` が `terminal:<coordinator>` か `terminal:courier` のレビュアーを数える。偽装は §3 の D3 と同じく残る危険として受け入れる | courier.md §5、Y-86 の担当の推し |
| 2026-11-05 | クーリエのインナーループは台帳の状態から拾う: `doing` のタスクの `report.json` の `review_request` を `<task>@<commit>` ごとに1回、`review` に入った `log.jsonl` の行ごとに1回ゲートを回す（同じ commit で `review` に入り直したら回し直す。ゲートの「同じ head で2度目の Must fix」が効くように）。初めて見る台帳は控えるだけ。ゲートが判定できない（終了コード 3）・マージ・片付けに失敗したときは問い（`from: gate`） | courier.md §2.1・§5、Y-86 の担当の推し |
| 2026-11-05 | 上限を超えた回は、`review_request.extra` ではなく今の `review_rounds` と `round` で決める。起こさなかったら通知 `report`。Chief of Staff が `review_rounds` を上げれば、クーリエが次の見回りで起こす（Chief of Staff が手でレビュアーを起こす手順を残さない） | courier.md §5（`review_request.extra` は Chief of Staff の判断）、Y-86 の担当の推し |
| 2026-11-05 | マージは条件表の `repos.<repo>.merge` の方式で `gh pr merge --match-head-commit <ゲートの head>`。方式が無ければマージせず問い。マージしたらゲートの記録の `merged` を true にし、`andon task finish` を打つ。`andon task finish` はレビュアーの子タスクと、クーリエが作った detached worktree（`~/.herdr/worktrees/<repo 名>/<slug>-<id>-r<回>`）も片付ける（人がマージしたあと Chief of Staff が打つときも同じ1つの command で済む） | contract.md §7、仮設の `yard task finish`、Y-86 の担当の推し |
| 2026-11-05 | `andon task finish` は仮設の `yard task finish` を移したもの（aqua の policy の出し入れは仮設だけのものなので移さない）。人の手直しを集める `library/corrections.json`（design.md §1.9 の (b)）は P6a に入れない（形が schema に無い。別の手で） | Y-86 の担当の推し |
| 2026-11-05 | クーリエの通知は控え（`~/.local/state/andon/courier/<slug>.json` の `pending`）に積むだけ。Chief of Staff に届ける口（フック・離席時の `hc send`）は P3b・P6b | courier.md §5、Y-86 の担当の推し |
| 2026-11-05 | 台帳の CLI は `worktree` を HOME の下なら `~/…` で書く（`deck start`。今の `kanban start` も同じ）。`andon task` は読むときに HOME に開く | Y-86 で見つけた（本物の台帳の半分以上が `~/` 形） |
| 2026-11-05 | マージの直前に、タスクの予約キー・履歴（`by`）・委譲を読み直す。ゲートのときと1つでも違えば（`merge: person` に変わった・委譲の `merge` が変わった・review でなくなった）`gh pr merge` を打たず、控えに書かずに次の見回りでゲートから回し直す | P6a の独立レビュー 1 回目（Y-88）の major と Chief of Staff の方針 |
| 2026-11-05 | レビュアーの起動は段（detached worktree・レビュアーのタスク・端末・最初のメッセージ）ごとに控えに書き、最初のメッセージが届くまで成功にしない。起こした（started）・起こさない（skipped。設定・上限・head 違い）・起こしそこねた（failed。fetch・`hc run`・最初のメッセージなどの一時の失敗）を分け、failed は次の見回りで済んだ段の続きから（タスクも端末も二つにしない。生きている端末へは最初のメッセージを送り直す）。3回続けば通知 `report` | 同上 |

## 14. ADR（Chief Engineer の題）

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | skill の配布: Claude Code 系は `/opt/andon` を plugin `andon` として `--plugin-dir` で読み（`/andon:<name>`）、役の skill を `--append-system-prompt-file` で system prompt に足す。Codex など `.agents/skills` を読むハーネス向けは boot が `~/.agents/skills/andon-<name>` の symlink。作業側の別マシンは image で立てる。PR 時の stale 検査は入れず、版の照合は `machine` と一緒（`ANDON_HOME` が checkout なら照合しない）。やらないこと: Skills over MCP・独自エージェント・marketplace での配布・Intent／npm 同梱 | [ADR-0001](adr/0001-skills-distribution.md) |
| 2026-11-05 | Chief Engineer の題は ADR（`docs/adr/NNNN-<題>.md`。番号と状態つき。形は design.md §1.8 のまま）。decisions.md には ADR 1本につき1行。部品の生きた設計（courier・image・ledger）と核（design・contract・decisions・operate）は `docs/` 直下のまま | 人の決定 2026-11-05（Y-84 の対話） |
| 2026-11-05 | Herdr App の Space と tab: 建屋から指す口は hc のラベル `herdr.space`・`herdr.tab` だけで、書くのは Andon の command とクーリエ。Space はプロジェクト × 2 段（指揮側 `andon-<slug>`・作業側 `andon-<slug>-work`。試作 S0 の結果で確定）、tab はタスク 1 つ（レビュアーは担当と同じ tab に横に並ぶ）。id は command が決定的に作る。command は attach を打たない（出た場所を「<Space 名> / <tab 名>」で出す。人に端末を指す言い方も同じ形）。名前は建屋から付けられないので、Andon 同梱の App の mod が人の App の中でラベルから付ける（Space `<slug>`・`<slug> work`、tab `CoS`・`CE <id> <題>`・`<id> <題>`。並行して upstream に機能を頼む）。片付けは空になって消えるのに任せる。仮設の `yard task` には入れない。やらないこと: ペインの分割・`herdr-app` や建屋の `herdr` CLI・Chief of Staff による Space の操作・`where`・mod が名前以外に触ること | [ADR-0002](adr/0002-herdr-spaces.md)（Y-105・Y-106 の対話） |
| 2026-11-06 | Herdr App の作業側は Space ＝ タスク（`herdr.space=andon-<slug>-<id>`・名前 `<id> <題>`。tab は担当の 1 つで id は今まで同じ `andon-<slug>-<id>`・名前 `<id>`。レビュアーは担当と同じ tab）。Space のフォルダが担当の worktree になり、App の Files・Changes がタスクを指す。ADR-0002 の決めたこと 1・2 の作業側を覆す。指揮側は変えない | [ADR-0003](adr/0003-task-spaces.md)（人 2026-11-06。Y-147） |

## 15. P6b での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | フックは managed settings の `Stop`・`UserPromptSubmit` に1つずつ（`/opt/andon/hooks/stop`・`user-prompt-submit`）。どの端末でも同じフックが走るので、役は command が自分の端末のラベルで見分ける: `role=chief-of-staff` なら `andon courier drain`（`project=` から slug を引く。`--project` は省く）、`task=` なら `andon task stop-check`。どちらでもない・`andon` が無い・落ちたときは何も出さずに通す | contract.md §9、Y-90 の担当の推し |
| 2026-11-05 | フックで渡した `<news>` の `news.replied` は、画面ではなく次の `Stop` の入力の `last_assistant_message` で決める（`<no-reply>` だけなら、そのときの端末の `turn` で書く）。フックの文は画面に `</news>` として出るとは限らないため。③で送ったものは仮設どおり画面で決める | courier.md §3.1、Y-90 の担当の推し |
| 2026-11-05 | フックで渡した `<news>` は、渡した turn の `Stop` を見るまで控え（`hook`。端末・経路・`session_id`・`prompt_id`・渡した通知）に残す。`Stop` の入力の `prompt_id`（と `session_id`）が渡した turn と合えば確かめ済みにし、そのときだけ `news.replied` を書く（`Stop` の block で渡したものは続きの turn、`stop_hook_active` の `Stop`）。合わない `Stop`・次の `UserPromptSubmit` が来たら（人が中断した・フックの出力が捨てられた）、渡した通知を1回だけ届け直す（届け直した分はもう届け直さない）。控えを書けてから出力する | P6b の独立レビュー 1 回目（Y-96）の major・minor と Chief of Staff の方針 |
| 2026-11-05 | `stop-check` は控え（止めた回数）を書けなければ止めずに通す（数えられないまま止め続けない） | 同上 |
| 2026-11-05 | クーリエの見回りと `drain` は控えを lock（`<slug>.json.lock` の flock）で取り合う。見回りは取れるまで待ち、`drain` は 10 秒待って取れなければ通す（待ちは次の口で届く） | courier.md §3（フックが落ちたときは通す）、Y-90 の担当の推し |
| 2026-11-05 | `stop-check` の「前回の通過」は、通したときの `log.jsonl` の行の数を控え（`~/.local/state/andon/stop-check/<slug>-<id>.json`）に持つ。`doing` に入り直した行があればそこから数える。続けて止めるのは 3 回まで（`stop_hook_active` が続く間）で、超えたら通して理由を1行出す（フックが続きのターンを回し続けて費用を燃やさないため） | contract.md §9、Y-90 の担当の推し |
| 2026-11-05 | `UserPromptSubmit` では、待ちが無くても委譲の一覧（`andon delegate show --json` を1行に詰めたもの）を `<delegation project="<slug>">` で毎回添える | contract.md §9、Y-90 の担当の推し |
| 2026-11-05 | 通知の文は `kind` ごとに1行（`review-requested`・`gate`・`merged` も）。`stalled` の分は通知の `at` と `detail.status_ms` の差。題・名前は制御文字と `<>` を空白に替えて60字に切る（`</news>` を題から漏らさない） | courier.md §3.1、Y-90 の担当の推し |

## 16. P7 の前提（F2・F3）での決定

担当が推しで進めたもの。覆すなら下に行を足す。

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-05 | `andon chief-of-staff` は仮設の `yard coordinator` を移したもの。端末の名前は `<slug>-cos`（担当の `<slug>-<id>` と並べて見分ける）、cwd は台帳の root、ラベル `role=chief-of-staff project=<slug>`。動いているものはラベルで探し、名前と cwd が合うときだけ使う。2人なら止まる。`config.yaml` の `coordinator` は違うときだけ `kanban config set` で書く。`--no-attach` でなく TTY から打ったときだけ `hc attach` | operate.md の F2、Y-102 の担当の推し |
| 2026-11-05 | `andon task start` は仮設の `yard task start` を移したもの（aqua の policy の出し入れは移さない）。起こせるのは `todo` で worktree・branch の無いタスクだけ。`hc run` のあと、担当の cwd でその端末として `kanban start` を打ち、`doing` と `agent` を command が結ぶ（`andon chief-engineer` と同じ）。最初のメッセージが届かなくても結んだまま、送り直す `hc send` を案内して終了コード 3（Chief of Staff は `deck set agent` を打たなくてよい） | operate.md の F3、Y-102 の担当の推し |
| 2026-11-05 | `andon task start` は `task.yaml` に `machine` があるタスクを起こさずに止まる（`hc run --on` はまだ扱わない。黙って指揮側で起こさない） | contract.md §2 の `machine`、Y-102 の担当の推し |
| 2026-11-05 | 起こす command（`chief-of-staff`・`chief-engineer`・`task start`）の共通部分（worktree と branch・`hc templates`・`hc run`）は `internal/spawn` に置く | Y-102 の担当の推し |
| 2026-11-06 | image の `andon` は uid・gid 1000（`ubuntu` 利用者を外す）。建屋の repo の持ち主が 1000 でなければ `bin/bootstrap` が起こす前に止まる（合わせ直す仕組みは作らない）。`andon boot` は HOME の volume の持ち主が自分でなければ `sudo chown -R` で直す（uid 1001 の版の volume を作り直さずに使う）。sshd は鍵が「無い」と「読めない」を別の行で出す | operate.md の F4（P7 で SSH の段が止まった）、Y-116 の依頼書の推し |
| 2026-11-06 | managed settings（`package/settings.json`）から herdr（TUI）の連携フック2つ（`herdr-auto-title`・`herdr-agent-state`）を外す。前者は tab 名を付けるが Andon では App の mod が付ける（ADR-0002）、後者は herdr（TUI）の socket に状態を送るもので hc と Herdr App の建屋では何もしない。建屋にファイルが無く、最初のメッセージが `UserPromptSubmit operation blocked by hook` で止まった。`tests/package.sh` が、フックの command が指すファイルが package に在ることを確かめる | operate.md §9（P7 で止まった）、Y-124 |
| 2026-11-06 | `stop-check` は待つのが正しいターンを止めない: 自分の問いが開いたまま（`answered` が来ていない）か、`report.json` の `review_request` の `commit` を指すレビュアーの `findings.json` がまだ無いとき。答え・レビューが届いたのに報告も問いも無く終わるターンは今までどおり止める | P7 の A-1（レビュー待ちのターンが3回続けて止められた）、Y-132 |
| 2026-11-06 | 信頼の画面は建屋では出さない（建屋＝コンテナが信頼の境界）。boot と、起こす直前の `andon task start`・`andon chief-engineer` が、建屋の repo と、台帳に載った案件の repo の根（`projects/*/.flightdeck/config.yaml` の `repos` にあって `<GHQ_ROOT>/<host>/<owner>/<repo>` に clone されたもの。ghq の下を全部は信頼しない）を Claude Code（`.claude.json` の `projects["<根>"].hasTrustDialogAccepted`）と Codex（`~/.codex/config.toml` の `[projects."<根>"] trust_level = "trusted"`）で信頼済みにする。足りない所だけ足し、今ある値は触らない（Codex で人が `untrusted` にした表も覆さない）。repo は実体のパスで実体の GHQ_ROOT の内側にあるものだけ（symlink で外へ逃げるものは足さない）。動いている Claude Code の書き込みを古い中身で上書きしないよう、足すものが無ければ書かず、書く直前に読み直して変わっていれば読み直した中身からやり直す（5回で諦めてログに1行）。読み直しから rename までの隙間は受け入れる。形が読めなければ書かずにログに1行。worktree の置き場は足さない: 実測（Claude Code 2.1.318・Codex 0.166.0）で、git の中は repo の根で判定し、worktree は元の repo の根の信頼が効き、根より上の親は効かない（docs/image.md） | 人の決定 2026-11-06（P7 で担当が信頼の画面で止まった）、Y-128 |
| 2026-11-06 | 最初のメッセージが届かなかった担当・Chief Engineer は、command が端末にラベル `first_message.sent=false` を付け、クーリエが見回りで送り直す（`idle` のときだけ。送れたらラベルを外す。`working`・`done` は手で送られたとみなす。二度送らない）。終了コード 3 は残し、案内は「クーリエが送り直す。急ぐときだけ手で」。Chief of Staff はクーリエが見張らないので手で送るまま | 人の決定 2026-11-06（ブロッカーが外れたら続きが動く）、courier.md §5、Y-128 |
| 2026-11-06 | クーリエの端末は「前に出ているもの（`foreground.argv`）が `andon courier`」で動いていると見る。無ければ起こし、中身がシェルに替わっていれば（core の起き直しで `--keep` が替えた）片付けて（`hc keep --off`・`kill`・`rm`）同じ名前・ラベルで起こし直す。シェルには打ち込まない（人の打ちかけに足して実行してしまう。独立レビューの指摘）。確かめるのは boot と Chief of Staff のフック（`andon courier drain`）。core の起き直しは boot を通らないが、クーリエが回るのは Chief of Staff が生きている台帳だけなので、Chief of Staff のターンごとのフックで足りる（クーリエの自己監視では戻せない・hc に打ち直しの機能は無い） | courier.md §5、Y-131（P7 の A-1 で見えた 1） |
| 2026-11-06 | 初めて見る台帳でも、`doing` のタスクの今の `review_request` は拾う（commit が PR の head と同じなので過去の出来事ではない）。二重に起こさない守りは台帳のレビュアーの子タスク（依頼書にその commit）で持ち、`todo` の子は live な端末（ラベル `project=`・`task=` の両方）とも照らす。起こさなかった回は、同じ commit でも担当が頼み直せば（報告の行が増えれば）拾い直す。起こした回は二重に起こさない | courier.md §2.1・§5、Y-131（P7 の A-1 で見えた 2） |

## 17. public にする準備での決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-06 | GitHub の squash merge が commit の author 名に持ち主の profile の名前（実名）を入れるのは、そのまま許容する（email は noreply。履歴を書き換えない） | 人（Y-126 の履歴の検査） |
| 2026-11-06 | 前身の private の repo の名前は、由来の記録として docs・tests・commit メッセージに残す（外さない。その repo は public にしない） | 人（Y-126 の履歴の検査。design.md §2.6 の「公開できない依存への参照」） |
| 2026-11-06 | repo の description は "A one-person software factory: AI coding agents take tasks from a ledger through review to merge, and call the human only for decisions only they can make."、topics は `ai-agents`・`coding-agents`・`claude-code`・`codex`・`multi-agent`・`software-factory`・`human-in-the-loop`・`developer-tools`。設定は public にするときに Chief of Staff が打つ | 人（Y-126 の案A） |
| 2026-11-06 | 名前 Andon は一般の用語なので、そのまま使う（商標は J-PlatPat と USPTO で確かめた） | 人 |
| 2026-11-06 | 同じ名前の別のもの（AI のツールや MCP server）と見分けるため、表示名に修飾を添える: 「Andon — a one-person software factory」。README の見出しの直下・英語の段の書き出し・description・social preview の画像（`docs/assets/social-preview.png`）で揃える。repo 名・コマンド・image 名は `andon` のまま | 人（Y-126） |

## 18. 配布の線での決定

| 日付 | 決定 | 出典 |
|---|---|---|
| 2026-11-06 | GHCR へは `vX.Y.Z` の tag の push のときだけ（tests を通した image を）push する。main へのマージごとの `:<sha>` の push はやめる（約 1 GiB の版がマージごとに増え、private の package のストレージ枠に当たった）。日々の試しは建屋で andon の checkout から焼く（`bin/bootstrap rebuild --from`。焼く手順は `image/build.sh` の1本で、tests・CI と同じ）。tag を切るまでの建屋は、いま GHCR にある SHA の版を使う | 人（2026-11-06）、Y-136 |
