# Andon の機械向けの約束

- 状態: 設計の提案（2026-11-04 起草、2026-11-05 書き直し、2026-11-05 schema に合わせて直した、2026-11-07 [ADR-0006](adr/0006-survey-fix-decide.md) の候補・`deliverable`・問いの `scope` を足した、2026-11-09 オーナーの手直し `corrections.json`〈§14〉を足した、2026-11-10 [ADR-0011](adr/0011-signals-and-value-loop.md) の信号の記録〈§15〉・リリースの名前〈§16〉・`project.json` の `signals`〈§11〉・通知 `signal`〈§6〉を足した、2026-11-19 [ADR-0028](adr/0028-improvement-loop.md) の起動の記録〈§18〉と結果の記録〈§19〉と振り返りの入力〈§20〉を足し、同じ日にパターンの一覧〈§21〉を足した、2026-11-20 [ADR-0034](adr/0034-prevention-before-tests.md) の kanban の行〈§22〉と引き取れる課題〈§23〉を足し、同じ日に [ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) の走らせた記録〈§24〉を足した、2026-11-21 [ADR-0042](adr/0042-owner-waits-do-not-stop-the-line.md) のオーナー待ち〈§25〉を足した、2026-11-22 [ADR-0044](adr/0044-owner-digest.md) のダイジェストの入力〈§26〉と中身〈§27〉と送った結果〈§28〉を足し、同じ日に [ADR-0049](adr/0049-owner-touches-by-kind.md) の介在回数の数え方で流れの数え〈§13〉をバージョン 2 にした）。**正は `schema/`**（JSON Schema。draft 2020-12）で、この文書はそれを人が読めるように説明したもの。食い違えば schema が正。この文書の ```` ```json ```` の例は全部、`tests/schema.sh` がその節の schema に通す。

| 節 | schema |
|---|---|
| §2 kanban の項目 | `schema/issue-keys.schema.json` |
| §3 指摘 | `schema/findings.schema.json` |
| §4 報告の機械向けの部分 | `schema/report.schema.json` |
| §5 問い | `schema/question.schema.json` |
| §6 通知 | `schema/notification.schema.json` |
| §7 gate の条件表と記録 | `schema/gate.schema.json`（条件表か記録のどちらか一方に合う） |
| §11 プロジェクトの事実と委譲 | `schema/project.schema.json` |
| §12 候補の一覧 | `schema/candidates.schema.json` |
| §13 流れの数え（`andon measure --json` の出力） | `schema/measure.schema.json` |
| §14 オーナーの手直し | `schema/corrections.schema.json` |
| §15 信号の記録 | `schema/signal.schema.json` |
| §17 届けた通知の記録 | `schema/delivered.schema.json` |
| §18 起動の記録 | `schema/launch.schema.json` |
| §19 結果の記録 | `schema/outcome.schema.json` |
| §20 振り返りの入力（`andon retro collect` の出力） | `schema/retro.schema.json` |
| §21 パターンの一覧（`retro.json`） | `schema/patterns.schema.json` |
| §22 kanban の行（`log.jsonl`） | `schema/ledger/log.v3.schema.json` |
| §23 引き取れる課題（`kanban ready --json` の出力） | `schema/ledger/ready.v3.schema.json` |
| §24 走らせた記録（`runs/<n>.json`） | `schema/run.schema.json` |
| §25 オーナー待ち（`andon waiting --json` の出力） | `schema/waiting.schema.json` |
| §26 ダイジェストの入力（`input.json`） | `schema/digest-input.schema.json` |
| §27 ダイジェストの中身（`digest.json`） | `schema/digest.schema.json` |
| §28 ダイジェストを送った結果（`sent.json`） | `schema/digest-sent.schema.json` |

`schema/process/` の2本（`workstream`・`state`）は、エージェントと機械の間の約束ではなく、オーナーが書く流れと状態の定義（`process/` の YAML）の形。読み方は [process/README.md](process/README.md)、検査は `tests/process.sh`（[ADR-0017](adr/0017-process-definitions.md)）。

- 読み手: command（`andon issue …`・`andon gate`・`andon triage`・`andon retro`・`andon delegate`・`andon measure`・`andon waiting`・`andon digest`）・dispatcher（`andon dispatcher`）・フック・ボード（オーナーの Interface）・振り返り。語は [design.md](design.md) §0.1。
- kanban を読み書きするコマンドは `kanban`（`cmd/kanban`。command と形式は [ledger.md](ledger.md)）。
- kanban の形式（形式 3。[ledger.md](ledger.md)・`schema/ledger/`）は Andon が持つ（[design.md](design.md) §3.5 で移した）。Andon のキーは「Any other key is allowed and kept」の上に置き、形式そのものを変えるときは形式のバージョンで変える。

## 0. 原則

1. **Markdown を読まない。** 依頼書・報告・レビューの報告・skill は人と AI のもの。機械が判定に使う値は、ここに並ぶ JSON／YAML のキーにだけ持つ（オーナーの決定 2026-11-04。仮設の工場で Markdown を正規表現で読む形は端が尽きなかった）。
   例外は次の4つだけで、どれも判定に使う値を読むのではなく、文から行や語を拾うか探すだけ。新しく足さない（4つ目は [ADR-0044](adr/0044-owner-digest.md) 決めたこと 6 が決めた。[記録](decisions/2026-11-22-digest-reads-decision-lines.md)）。
   - `andon triage collect` は `done`・`review` の課題の `report.md` から `## Next`・`## Remember`・`## オーナーに聞くこと`（古い見出し `## 人に聞くこと` も同じに読む）・`For AGENTS.md:`・見送りの行を拾って項目にする（見出しが無い・読めない報告は `unreadable` の項目にして、トリアージの担当が自分で読む）。`## Remember` の「計画との差:」で始まる行と、報告が無いか見出しの無いオーナーの課題（`who: person` か題の末尾が「（オーナーの判断待ち）」。古い末尾「（人の判断待ち）」も同じに読む）は `unreadable` にもせず、項目にしない（[ADR-0030](adr/0030-triage-close-safety-net.md) 決めたこと 5 の 2。振り返り〈`report.json` の `remember`〉と `andon measure` の読み方は変わらない）。
   - `andon issue impact` は、マージした PR のファイルの path か名前が依頼書（`brief.md`）に出るかを探す（`brief_refs`。§6 の「マージのあとの影響」）。
   - `andon issue resume` と dispatcher（起き直したレビュアーを拾うとき）は、dispatcher がレビュアーの依頼書に書いた「対象: … commit <sha>。」の行から commit を引く（§8）。
   - `andon digest collect` は、課題の repo の `docs/` の文書の「決めたこと」の節から、Chief Engineer の印の付いた番号つきの行を拾う（§26 の `decided.decisions`。オーナーに見せるだけ）。
2. **無い・読めない・知らない値は安全側に倒す。** gate は `needs_human`、dispatcher は送らない／起動しない、担当の command は止まって理由を1行出す。黙って既定に落とさない（既定があるキーはこの文書に「既定」と明記する）。
3. **1つの約束に1人の書き手。** `issue.yaml` の予約キーは Chief of Staff、`findings.json` はレビュアー、`report.json` と `candidates.json` とパターンの一覧（`retro.json`。§21）は担当、走らせた記録（§24）は担当が打つ `andon issue record`、問いの JSON は問うた者、通知と gate の記録とオーナーの手直し（`corrections.json`）と結果の記録（`outcome.json`。§19）と信号の記録（§15）は dispatcher、起動の記録（§18）はエージェントを起動する command、委譲と `project.json` はオーナー。履歴は `log.jsonl` の `by`（kanban）・端末のラベル・git の author（委譲）で照合する。
4. **読み手は知らないキーを無視し、書き手は知らないキーを保つ**（ledger-format の互換の規則と同じ）。キーを増やすのはバージョンを上げない。キーを消す・意味を変えるのはバージョンを上げ、`schema/` の `$id` と `docs/decisions.md` に書く。
5. **置き場は kanban の中。** 担当が worktree の外に書いてよい唯一の場所 `issues/<id>/library/` を、機械向けの置き場にも使う。作業側が別のマシンのときは `<worktree>.issue/outbox/` に書き、dispatcher が引いて同じ場所に置く（design.md §1.5）。`project.json`（委譲を含む）だけはセルの repo（§11。書き手がオーナーだから）。
6. **形は小さく、テストで固定する。** キーは名前を決めたものだけ（余分なキーは書かない。読み手は無視する）。schema は書き手の約束なので、知らないキーを拒む（`additionalProperties: false`。kanban の項目 §2 と委譲の値を除く）。`tests/schema.sh` が例と schema を突き合わせる。command（`andon issue`・`andon gate`・`andon delegate`）の出力も `tests/schema.sh` が schema に通す（`tests/andon-issue.sh`・`tests/andon-gate.sh` に書かせる）。加えて `andon` は、約束の JSON を書く前に、自分に埋め込んだ `schema/` の直下の schema と照らし、合わなければ書かない。command は 0 でない終了コードで終わり、dispatcher はその巡の控えを書かずにログに1行出す（[ADR-0034](adr/0034-prevention-before-tests.md) 決めたこと 6）。検査器は1つで、既製の Go の検査器（`github.com/google/jsonschema-go`）に拡張のキーワード `uniqueKeys` を足したもの（`internal/contract`）。テストとセルで JSON を照らすのも同じ検査器で、口は `andon schema check <schema> <json>`（`<schema>` は `report` などの名前か `*.schema.json` のパス。合えば 0、合わなければ 1、検査できなければ 2）。検査器の知らないキーワードがある schema は、黙って飛ばさずに検査できないとして断る。検査器は Go の正規表現を使うので、`pattern` は ECMA-262 と Go で同じ意味になる書き方だけを使う（schema そのものの決まりは `schema_check.py lint` が見る）。

## 1. 置き場と書き手

```
projects/<slug>/kanban/
  config.yaml                     chief_of_staff（kanban config set。Chief of Staff の端末）
  issues/<id>/
    issue.yaml                     kanban の項目 ＋ Andon の予約キー（§2。Chief of Staff が kanban set）
    brief.md                      人・AI 用（Settings は写し。機械は判定に読まない。例外は §0 の1）
    report.md                     人・AI 用（機械は判定に読まない。例外は §0 の1）
    questions.yaml                kanban の問い（§5。text は人向け）
    library/
      report.json                 担当（§4）
      findings.json               レビュアー（§3。レビュアーの課題の側）
      candidates.json             担当（§12。調べる課題の成果と、範囲の問いで止まる直す課題）
      questions/<n>.json          問うた者（§5）
      runs/<n>.json・<n>.log      andon issue record（§24。担当が打つ。走らせた記録と出力。書き足すだけ）
      gate/<head sha>.json        andon gate（§7。gate の記録。andon issue merge が回し、マージしたら merged を書く）
      corrections.json            dispatcher（§14。finish〈done〉のとき1回だけ。オーナーの手直し: 問いの答え・PR の上の直し・指摘の覆し。design.md §1.9 の (b)）
      launches.jsonl              エージェントを起動する command（§18。起動のたびに1行足す。バージョンと起動行）
      outcome.json                dispatcher（§19。finish〈done〉のとき1回だけ。結果・gate の判定・実働・セッションごとのコスト）
      retro.json                  振り返りの担当（§21。andon retro patterns が検査して書く。パターンの一覧）
      retro-applied.json          andon retro apply（§21。Chief of Staff が打つ。足した課題と捨てたパターン）
  chief-of-staff/launches.jsonl   andon chief-of-staff（§18。Chief of Staff は課題を持たないので kanban の直下）
  routines/<name>.yaml            オーナー（kanban のルーチン。形式は ledger.md。Andon が読む欄 idle_hours は §22 の下）
  log.jsonl                       kanban の CLI が書く（§22）。by で履歴を照合
projects/<slug>/project.json      オーナー（§11。書いてよい repo・委譲など。セルの repo。git の履歴が照合の元）
~/.local/state/andon/dispatcher/<slug>.json   dispatcher の控え（§6。kanban の外）
~/.local/state/andon/dispatcher/<slug>.signals.json   dispatcher（§15。外の信号の記録。kanban の外）
~/.local/state/andon/dispatcher/<slug>.merge.json   andon issue merge（§6。gate のあとのマージと片付けの記録。dispatcher は読むだけ。kanban の外）
~/.local/state/andon/dispatcher/<slug>.recorded.jsonl   dispatcher（§6。記録だけの通知。1行1件・追記だけ。kanban の外）
~/.local/state/andon/dispatcher/<slug>.delivered.jsonl  dispatcher（§17。届けた通知の記録。渡したターン1つに1行・追記だけ。kanban の外）
~/.local/state/andon/dispatcher/.memory.json   dispatcher（§6 の memory。線を超えたと積んだか。セルに1つ。kanban の外）
```

## 2. kanban の項目（`issue.yaml` の Andon の予約キー）

依頼書の `## Settings` の**正**。Chief of Staff が `kanban set <id> review=on must_fix=major …` で書く。依頼書の欄は人が読む写し。

| キー | 値 | 無いとき | 書き手 | 読み手 | 意味 |
|---|---|---|---|---|---|
| `review` | `on` / `off` | gate は `needs_human`（レビューの照合を回さない）。dispatcher は「レビューを頼まれても起動しない」 | Chief of Staff | gate・dispatcher・担当 | 独立レビューを工程に入れるか |
| `must_fix` | `critical` / `major` / `minor` / `trivial` | gate は `needs_human` | Chief of Staff | gate・担当 | この重さ以上の指摘は担当が直す |
| `reviewer` | プロファイルの名前（`codex-review` / `claude-review`） | 既定: 実装のプロファイルと別ブランド（Claude → `codex-review`、Codex → `claude-review`） | Chief of Staff | dispatcher（起動する）・gate（別ブランドの照合） | レビュアーのプロファイル |
| `review_rounds` | 1 以上の整数（を表す文字列。下の注） | 既定: `"2"` | Chief of Staff | dispatcher・担当 | 頼める回数の上限。超えた回は `report.json` の `review_request.extra` で頼み、起動するかは dispatcher ではなく Chief of Staff が決める |
| `merge` | `auto` / `person` | 既定: `person`（安全側） | Chief of Staff | dispatcher・gate | `auto`: gate が approve し、委譲（§11）の `merge` がそのレベルを許していれば dispatcher がマージし finish する。`person`: approve でも止まり、オーナーに通知する。**委譲の範囲を広げることはできない**（狭めるだけ） |
| `machine` | hcの machine の名前 | 既定: 指揮側と同じマシン | Chief of Staff | `andon issue start`・dispatcher | 作業側（担当・レビュアー）を起動するマシン。**まだ使えない**: `machine` のある課題は `andon issue start` と `resume` が起動せずに止まる（黙って指揮側で起動しない） |
| `deliverable` | `pr` / `candidates` | 既定: `pr` | Chief of Staff（依頼書の Settings の `Deliverable:` の写し） | `andon issue ready`・dispatcher・`andon gate`・ボード・担当 | 課題の成果の形。`pr`: PR（直す課題・Chief Engineer の文書）。`candidates`: PR を出さず `library/candidates.json`（§12）が成果（調べる課題）。`andon issue ready` は `pr` の代わりに `candidates.json` が在って形が合い（repo があれば）`commit` が `HEAD` と同じことを確かめる。dispatcher は `review` の通知だけ届けて gate を回さず、`review_request` があっても起動しない。`andon gate` を手で打てば回さずに終了コード 3（[ADR-0006](adr/0006-survey-fix-decide.md) 決めたこと 7）。`andon issue ready` と dispatcher は、値が `candidates` なら履歴の照合が外れていても `candidates` として読む（Chief of Staff が起き直して書き直す前でも PR を求めない。照合が外れていると、`andon gate` は理由の1行でそう言う。決めたこと 17） |
| `estimate` | `small` / `medium` / `large`（段→分は下の表） | 見込み無し（既定の段で埋めない。比べず、記録に「見込み無し」と数える） | Chief of Staff（依頼書を切るとき。依頼書の Settings の `Estimate:` の写し。段の名 小・中・大） | dispatcher・`andon measure`・担当 | 課題の大きさの見込み（目安であって上限ではない）。担当の課題（直す・調べる）だけ。レビュアー・Chief Engineer の課題には置かない（[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 2・3・8） |
| `only_owner` | 空でない 1 行の文字列（オーナーにしかできない理由） | 手作業の印が無い（`who: person` の課題なら、灯りの種類の問いが無ければ判断として数える） | Chief of Staff（`kanban set <id> who=person only_owner="<理由>"`）・`andon triage apply`（手作業の `person`） | `andon waiting` | オーナーの課題（`who: person`）が手作業だという印。`andon waiting` は、この印を持つオーナーの課題を手作業（§25 の `task`）に数え、値の 1 行目を行の `reason` に出す。`who: person` が無い課題の印は読まない。履歴の照合はしない（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 2・3） |
| `acceptance` | 受け入れ条件の番号を、報告の `acceptance[].id` と同じ綴りでコンマで並べた文字列（`A4,A13`。空白を入れない） | 番号を持たない課題（何も確かめない） | Chief of Staff（Chief Engineer の文書から課題を切るとき。依頼書の完了条件が名指す番号の写し） | `andon issue ready`・担当 | 課題が満たす受け入れ条件。`andon issue ready` は、番号ごとに `report.json` の `acceptance[]` に同じ `id` の行（`pass`・`fail`・`manual` のどれでもよい）が無ければ断り、足りない番号を出す。形が違う値も断る（黙って通さない）。履歴の照合は見ない（担当が書き換えても、求める行が増えるか、キーを消して今までどおりになるだけ。[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 3） |
| `hold` | 理由の課題の id（`A-0015`。1つ） | 止めていない | Chief of Staff（`andon issue hold <id> <理由の課題>` が書き、`andon issue unhold <id>` が消す） | `andon gate`・dispatcher（`andon issue merge`・`settle`）・`andon waiting` | Chief of Staff が、理由の課題の答えが担当の直しに入るまで、この課題の PR を止めている（[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 7〜11）。止めている間、gate は回らず（`andon gate` は GitHub を読まずに `wait`。§7）、dispatcher はマージせず、判定のあとに止めたら判定を捨てて gate の問いを閉じ、PR の印は黄（`andon gate: メンテナーの確認を待っています`）。`andon waiting` はマージの依頼に出さず、理由の課題の行の `holds` に数える（§25）。理由の課題が done になっても外れない（dispatcher が通知 `hold` で1回知らせる。§6）。値が空でも、最後にこのキーを変えた行を書いたのがこの課題の担当かそのレビュアーの端末なら、止めているものとして読む（担当が自分で外せないように。理由は消す直前の値）。止め（`log.jsonl`・子の課題の一覧）を読めなければ止めているものとして扱う |

`kanban show <id> --json` の `.issue`（Andon の予約キー以外は kanban の形式のキー）:

```json
{"id": "A-0012", "title": "andon issue ready を足す", "status": "doing",
 "review": "on", "must_fix": "major", "reviewer": "codex-review",
 "review_rounds": "2", "merge": "person", "machine": "vps", "deliverable": "pr",
 "estimate": "medium", "acceptance": "A4,A13", "hold": "A-0015"}
```

`estimate` の段→分（値の意味。待ちを除いた実働の目安で、規則ではない。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 3・11 の 7）:

| 値（依頼書の Settings の段の名） | 分 |
|---|---|
| `small`（小） | 30 |
| `medium`（中） | 60 |
| `large`（大） | 120 |

表の正は `internal/ledger` の `EstimateMinutes`（Go の定数はこれ1つ。dispatcher と `andon measure` が読む）。待ちを除いた実働の数え方（[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 5）は同じ所の `Worked`。

- **値は全部文字列**。`kanban set` は値を文字列で書くので、`review_rounds` も `"2"`（1 以上の整数を表す数字の列）。YAML に手で `review_rounds: 2` と書いた数は schema を通らない（読み手は「無い」として既定を使う）。
- kanban の形式は知らないキーを許して保つので、この schema は**知らないキーを拒まない**。どれも必須ではない（無いときの振る舞いは上の表）。
- **履歴の照合**: gate と dispatcher は、`log.jsonl` でそのキーを最後に変えた行の `by` が `terminal:<config.yaml の chief_of_staff>` で、その値が今の `issue.yaml` と同じときだけ信じる。違えば `needs_human`。`chief_of_staff` が無ければ信じない。`log.jsonl` の id は 4 桁に揃えて読む（移す前の行は `A-5` のまま。[ledger.md](ledger.md) の Writing）。
- **レビュアーの課題**（`parent` が元の課題で `profile` が実装と別ブランドの `*-review`）は、作成の行（最初の `add`）の `by` が `terminal:<chief_of_staff>` か `terminal:dispatcher`（dispatcher が起動したもの）のときだけ、gate と `andon issue ready` がレビューに数える。id は `<元の課題の id>-r<回>`（`A-0005-r1`。dispatcher が `kanban add --round <回>` で付ける。2026-11-06 より前の子は通し番号のまま読む。[ledger.md](ledger.md) の Writing）。数えるのは id の形ではなく `parent` と `profile`。
- 値の綴りは小文字（`machine` を除く）。知らない値は「無い」と同じ。
- `estimate` も履歴の照合を要る（最後に変えたのが Chief of Staff でなければ「無い」）。要らないと担当が自分の課題の見込みを書き換えてズレを消せる。Chief of Staff が起動し直してアドレスが変わったら、ほかの予約キーと一緒に打ち直す（打ち直さないと見込みが「無し」になる。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 11 の 5）。
- 読み手が `kanban show <id> --json` の `.issue` から取る（ファイルを直接読まない）。
- **起動の前の確かめ**: `andon issue start` は、要るキーが無い・知らない値か、履歴の照合が外れている課題を起動せずに終了コード 1 で断り、キーの名前と、Chief of Staff の端末で打てば直る `kanban set` の行を最後に出す（無いキーは値を選ぶ場所を残し、コマンドは値を書かない）。要るキーは `review`・`estimate` と、PR を出す課題（repo があって `deliverable` が `candidates` でない）なら `must_fix`・`merge`、そのうえ `review` が `off` でなければ `reviewer`・`review_rounds`（`off` でも値があれば確かめる。差分がレベル B・C なら dispatcher はレビュアーを起動し、値のあるキーに照合を求める）。Chief Engineer の課題は見ない（[決定](decisions/2026-11-22-issue-start-refuses-unset-settings.md)）。ただしルーチンが足した課題（`log.jsonl` に `by: routine:<名前>`・`fired: <id>` の行がある）と、トリアージの apply が足した課題（仕分けの記録の `created_issue`、または足す前の印 `adding` と同じ題で、Chief of Staff の端末が足した課題。`log.jsonl` に足した行が無い課題〈`kanban add` が `issue.yaml` を書いたあと行を足す前に止まったもの〉は、印を残した時刻 `adding_at` より後に作られた〈`created`〉ときだけ数える。`log.jsonl` が空なら数えない。apply が予約キーを書く前に止まった課題）は、Chief of Staff の端末で打てば、無いキーと、既定と同じ値なのに照合が外れたキーを既定（下）で書いてから起動する（`--dry-run` は書かない。既定と違う値は変えない。[決定](decisions/2026-11-23-commands-write-default-settings.md)）。
- **起動の前の写しの書き直し**: `andon issue start` は、確かめを通って既定を書いたあと、担当を起動する前に、依頼書の `## Settings` の写し（見出しの直後に続く `- 名前: 値` の行）を予約キーに合わせて書き直す。課題を足したあと `kanban set` で予約キーを書き直しても、担当に渡る写しが正と食い違わないようにするため。行の値は `SettingsOf` が読む値（gate・dispatcher が読む値と同じ。知らない値は既定か空）で、値の頭が同じ行と知らない名前の行はそのまま残し、値の無い行は消し、行の無いキーで `issue.yaml` にあるものは並びの末尾に足す。写しの後ろ（地の文・追記の節）は触らず、見出しが無い依頼書には書き足さない。見出しが2つ以上あればどれも書き直し、コード例（```か~~~の囲み。閉じるのは開いたのと同じ文字で同じ数以上の並び）の中の見出しは触らない。kanban の CLI と同じ鍵（`kanban/.lock`）を取ってから依頼書を読み直し（確かめのあと書くまでの間に足された追記を消さない）、一時のファイルから置き換える。`--dry-run` は書き直したものを出すだけで書かない。Chief Engineer の課題は見ない。
- **command が足す担当の課題**: `andon triage apply`（`fix`・`bundle`。足した課題を記録に書いてから）と `andon retro apply`（担当の課題）は、足した直後に予約キーを全部 `kanban set` する（Chief of Staff が打つので照合が通る）。値は `internal/ledger` の `DefaultSettings`: `must_fix=major`・`merge=person`・`estimate=medium`、`review` はトリアージの決定の値（振り返りとルーチンは `off`）。`review` が `on` のときだけ `reviewer`（実装と別ブランド）と `review_rounds=2` も書く（`off` で書くと、あとで profile を替えたとき同じブランドに固定される）。

## 3. 指摘（`findings.json`）

レビュアーの課題の `library/findings.json`（キーはこの名前だけ）:

```json
{"commit": "2a17f63e53ac7f9554d78782e2bfc4d079454455",
 "findings": [{"severity": "major", "file": "internal/issue/issue.go", "line": 120},
              {"severity": "major", "file": "internal/ledger/ledger.go", "line": 40, "preexisting": true}]}
```

| キー | 値 | 意味 |
|---|---|---|
| `commit` | 40桁・小文字 | 見た commit。PR の head と違えば stale（PR の差分が同じなら引き継ぐ。下） |
| `findings[].severity` | `critical` / `major` / `minor` / `trivial` | 重さ。起きる条件を具体的に示せないものは `trivial` |
| `findings[].file` | repo からの相対パス（空でなく、制御文字・改行が無く、`/` で始まらず、`..` の段を含まない） | — |
| `findings[].line` | 1 以上の整数 | — |
| `findings[].preexisting` | 真偽（任意。無ければ `false`） | 課題の前からあった問題（差分の外の古い問題）。gate は `must_fix` に数えない。ただし `file`・`line` が PR の差分で足した行なら数える（[decisions/2026-11-09-review-preexisting.md](decisions/2026-11-09-review-preexisting.md)） |

- 無い・JSON でない・形が違う → missing。`commit` 違い → stale。報告の Markdown は読まない。
- `commit` が head と違っても、PR の差分が同じなら current として head に引き継ぐ。PR の差分は、その commit と `origin/HEAD` の merge-base から commit までの diff で、hunk の行番号と index の行を除いて、空白も含めて同じなら同じとする（衝突の無い rebase で head だけが変わった。git は課題の worktree で打つ）。空白の違いを無視する `git patch-id` は使わない。worktree・commit・`origin/HEAD` が無いときは引き継がない。`andon issue review-request` は、引き継いだレビューがあれば頼まない（[decisions/2026-11-20-review-carry-same-patch.md](decisions/2026-11-20-review-carry-same-patch.md)）。
- 書き手はレビュアー。読み手: gate（`must_fix` 以上の件数）・担当（振り分け。`must_fix` 以上を直し、未満は `report.json` の `waived` に理由つきで残す。`preexisting: true` は振り分けない）・振り返り（繰り返し出た指摘を観点へ）。
- gate が `must_fix` に数えないもの（どちらも機械が記録から決める。申告で gate を緩めない）:
  - `preexisting: true` の指摘（場所が PR の差分で足した行なら数える）。
  - **2回目以降の回**の critical でない指摘で、**前の回**の `findings.json` に同じ `file` の指摘が無いもの（新しい指摘。2回目以降は直しの確認だけ。[decisions/2026-11-09-review-later-rounds.md](decisions/2026-11-09-review-later-rounds.md)）。回は kanban から決める: gate が数えるレビュアーの子課題を古い順に並べ、最新の1つの前で `findings.json` が読めて `commit` が最新と違う最初のものが前の回（`andon issue review-request` の数え方と同じ）。無ければ1回目。
- `andon issue ready`・`andon issue show` の照合は `preexisting` の場所を確かめない（PR の差分を持たない下見）。正は gate。
- 将来キーを足すなら（例: `title`・`kind`）バージョンを上げずに足せる（原則4）。

## 4. 報告の機械向けの部分（`report.json`）

担当の課題の `library/report.json`。担当が書き直すたびに全体を書く（`report.md` と同じ）。書き手は担当。`kanban report` の直前に書く——dispatcher は `log.jsonl` の `{"report": "write"}` を合図に `report.json` を読む（新しい合図の種類を足さない）。

```json
{"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "pr": "https://github.com/<owner>/andon/pull/12",
 "review_request": {"round": 1, "limit": 2, "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "previous": null, "reviewer": "codex-review", "extra": false},
 "estimate": {"size": "large", "reason": "直す所が3つの package にまたがる"},
 "plan": [{"id": "p1", "text": "schema に足す。終了条件: tests/schema.sh が通る", "done": true},
          {"id": "p2", "text": "andon issue report で引き継ぐ。終了条件: tests/andon-issue.sh が通る", "done": false}],
 "waived": [{"severity": "minor", "file": "internal/issue/issue.go", "line": 88, "reason": "起きる条件がこの repo に無い"}],
 "acceptance": [{"id": "A1", "result": "pass", "evidence": "bash tests/andon-issue.sh（出力 library/A1.log）"},
                {"id": "A2", "result": "pass", "evidence": "go test ./internal/issue/ -run TestRecord（記録 runs/3.json）", "run": 3}],
 "risk": {"one_way": true, "impact": ["sample-site の本番の表 orders から列 note を消す"]},
 "next": [{"to": "person", "text": "PR #12 を読む（レベル C: package/gate.json）"},
          {"to": "chief-of-staff", "text": "マージ後に andon issue finish A-0012"},
          {"to": "issue", "text": "lib/andon/triage が足す課題にも review・must_fix を書く"}],
 "observation": {"hypothesis": "返事の下書きボタンで返信にかかる時間が減る", "result": "hit", "evidence": ["signal:3f9a1c"]},
 "remember": ["機械が判定に使うものは Markdown に埋めない"],
 "agents_md": ["テストは bash tests/*.sh"]}
```

| キー | 要るか | 読み手 | 意味 |
|---|---|---|---|
| `commit` | repo のある課題では必須（schema では任意。`andon issue report` が書き、`andon issue ready` が確かめる）。repo の無い課題では書かない | dispatcher・gate | 書いた時点の HEAD（40桁・小文字）。`andon issue ready` が `HEAD` ＝ `commit` ＝ PR の head を確かめる |
| `pr` | PR を出したら必須（schema では任意。確かめるのは `andon issue ready`） | gate・dispatcher | 今の報告の `PR:` 行の代わり。gate はこれだけを読む。`https://github.com/<owner>/<repo>/pull/<n>` で、owner と repo は `/` を除く印字できる ASCII |
| `review_request` | レビューを頼むときだけ。中の6つのキーは全部必須（`previous` は初回 `null`、`round`・`limit` は 1 以上の整数） | dispatcher | dispatcher が `kanban add --parent <id>` ＋ detached worktree ＋ `hc run <reviewer>` で起動する。`round` ≤ `limit` でなければ起動しない（`extra: true` なら Chief of Staff の判断待ち）。`previous` があれば再レビュー（前の回からの違いだけ。`previous` が `commit` の祖先なら差分 `previous..commit`、rebase をまたぐなら `git range-diff` で PR 自身の差分どうしの違いを見せる。祖先か・PR の base を確かめられなければ素の差分は使わせず `gh pr diff` を案内する）。`commit` が PR の head と違えば起動しない。`andon issue report` は、`HEAD` が前の `report.json` の `commit` とも `review_request.commit` とも同じ間は引き継ぎ、`HEAD` が進めば引き継がない（`pr`・`estimate`・`plan` と同じ。レビュー依頼中に報告を書き直しても依頼が消えない。[decisions/2026-11-20-report-carries-review-request.md](decisions/2026-11-20-report-carries-review-request.md)） |
| `estimate` | 任意（担当が依頼書の見込みと違うと見たときだけ） | Chief of Staff・`andon measure` | 担当の見込み `{"size": "small" \| "medium" \| "large", "reason": "…"}`（2つとも必須。`size` は §2 の `estimate` と同じ値、`reason` は1行）。担当は止まらず進み、報告の `## 計画` にも同じ1行を書く。課題の見込み（§2 の `estimate`）は変えず、dispatcher のズレの通知は Chief of Staff の見込みと比べたまま。`andon issue report` は `--data` に無ければ前の `report.json` から引き継ぐ（`pr` と同じ。担当は毎回全体を書き直すので、引き継がないと最終の報告から消える）。足してもバージョンは上げない（原則4。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 4・11 の 3） |
| `plan[]` | 任意（担当が計画を書いたら） | Chief of Staff（見守り） | 担当の計画。報告の `## 計画` の機械向けの写しで、1 件 `{"id", "text", "done": true \| false}`（3つとも必須。`id` は計画の中で一意な短い名〈`p1` など〉、`text` は作業と終了条件の1行、`done` は終了条件を満たしたか）。進み具合は `done` の数。見守り（`process/workstreams/watch.yaml` の `report.plan`）が読む。`andon issue report` は `--data` に無ければ前の `report.json` から引き継ぎ、あれば丸ごと置き換える（`estimate` と同じ）。足してもバージョンは上げない（原則4。[ADR-0017](adr/0017-process-definitions.md) 決めたこと 10） |
| `waived` | 任意 | トリアージ・振り返り | 見送った指摘（`findings.json` と同じ3つ ＋ `reason`） |
| `acceptance[]` | 任意（Chief Engineer の文書に受け入れ条件があるとき） | gate・振り返り | `{"id", "result": "pass" \| "fail" \| "manual", "evidence"}`。`evidence` はテストの command と、その出力（ユースケースの実行の跡を含む）のパス。証拠になるのは、その commit で走らせた本物の出力だけで、前もって用意した（staged な）出力と、確かめる継ぎ目を mock に置き換えたテストの結果は証拠でない。本物の証拠が無い条件は `pass` と書かない。`fail` があれば `fix`、`manual` があればレベルを1つ上げる（design.md §1.9 の (a)）。`run` は任意で、証拠が走らせた記録（§24）のときにその `id`（1 以上の整数）を書く。記録でない証拠（CI の検査の名前・画面の添付の URL・`manual`）では書かない（[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 2）。`run` のある行だけ、`andon issue ready` と gate が記録の中身を見る（§24）。足してもバージョンは上げない（原則4） |
| `risk` | 任意（変更そのものが戻せないとき） | gate・オーナー | 担当の一方通行の申告 `{"one_way": true \| false, "impact": ["…"]}`（`one_way` は必須、`impact` は任意）。`one_way: true` はデータの移行・消す操作・外への約束のように revert で戻らない変更。gate はレベルを C に上げ、`tier_reasons` に「担当の申告: one_way」と書く。`false`・無しでレベルを下げない（申告は厳しくする向きにだけ効く。原則2）。gate が読むのは `one_way` だけで、`impact` は人が読む欄。足してもバージョンは上げない（原則4。[decisions/2026-11-09-report-risk-one-way.md](decisions/2026-11-09-report-risk-one-way.md)） |
| `next[]` | 任意 | トリアージ | `to` は `person` / `chief-of-staff` / `issue`（次の課題に切るもの） |
| `observation` | 観察の課題（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 7）の報告でだけ | Chief of Staff・Chief Engineer・measure | Chief Engineer の文書の価値の仮説を、信号の記録で確かめた結果 `{"hypothesis", "result": "hit" \| "miss" \| "unknown", "evidence": []}`（3つとも必須。`evidence` は読んだ信号の key・URL・パスで、空の配列も可）。ラインは結果で何もしない（Chief of Staff がオーナーに1行で返す）。足してもバージョンは上げない（原則4） |
| `remember[]`・`agents_md[]` | 任意 | トリアージ・振り返り | 人用の報告の `## Remember`・`For AGENTS.md:` の機械向けの写し |

- 担当の `andon issue ready <id>`: まずこの課題に受け取られていない問い（`open`・`answered`・読めない `questions.yaml`。`kanban done`・`kanban review` と `andon issue finish` と同じ確かめ）が無いことを確かめ、あれば `doing` のまま終了コード 1 と理由1行（問いを残して `review` に入ると、マージのあとの片付けが問いで断られるため）。ただし gate の問い（`from: gate`）のうち、事実の gate の記録の行（`gate の記録 <…>/library/gate/<head>.json`。前のバージョンが書いた「ゲートの記録 」で始まる行も同じに読む）が `HEAD` と違う head を指すものは数えない（担当が `doing` に戻って rebase した。前の head について出た問い）。数えなかった問いだけが残るときは、その問いだけを名指す `kanban review <id> --past <id>#<n>,…` で入る（越えた問いの番号は `log.jsonl` の `forced`）。名指しの外の問いが確かめのあとに `kanban ask` で足されていれば、`kanban` が kanban の lock の中で断り、`doing` のまま残る。越えた問いは、gate が今の head で決めたときに `andon issue merge` が「PR の head が動いた（前 → 今）」と答えて閉じる。どの head か読めない gate の問いは数える。次に `report.json` を検証（必須キー・予約キー `acceptance`〈§2〉が名指す番号ごとに `acceptance[]` の行がある・`acceptance[]` の `run` のある行が指す記録が証拠になる〈§24〉・`commit` ＝ `git rev-parse HEAD` ＝ `gh pr view --json headRefOid`・`review: on` なら最新のレビュアーの課題の `findings.json` が `commit` を指す）してから `kanban review <id>`。合わなければ終了コード 1 と理由1行。`deliverable: candidates`（§2。履歴の照合は見ない。Chief of Staff が起き直して書き直す前でも PR を求めないため）なら `pr` とレビューを要らず、代わりに `library/candidates.json`（§12）が在って形が合い、repo があれば `report.json` と `candidates.json` の `commit` が `HEAD` と同じことを確かめる。
- 担当の `andon issue review-request <id>`: `review_request` を `report.json` に書き（`round` は前の回から数える）、`kanban report` する。担当は `## レビュー依頼` の文を書かなくてよい。その課題のレビュアーの課題がまだ `todo`・`doing` なら、書く前に終了コード 1 で断る（回は進まない）。`HEAD` を見ている子課題（依頼書に `commit <HEAD>。` がある）なら「もう頼んである」、古い commit を見ている子課題なら「まだ動いている」と書く。レビュアーが 2 人走ると、照合が最後に終わったほうだけを読み、`ready` も頼み直しも断られて進めなくなるため（[ADR-0032](adr/0032-shorten-time-to-merge.md) 決めたこと 8）。
- `andon issue start`・`report`・`review-request`・`candidates`・`record`・`ask`・`ready`・`finish` は、課題の今の `status` から打てるものが [`process/states/issue.yaml`](../process/states/issue.yaml) の移り方（`command`）に在るときだけ動く。無ければ kanban に書く前に終了コード 1 と理由1行で断る（例: `todo` の課題への `ready`、`done` の課題への `review-request`）。定義はバイナリに埋め込んである。`by: kanban` の移り方（担当の `kanban start`、レビュアー・Chief Engineer の `kanban report`・`kanban review`）は `kanban` を直に打つので断らない（[ADR-0017](adr/0017-process-definitions.md) 決めたこと 15）。
- `andon issue start` は、レビュアーの子課題（`parent` があり `profile` が `*-review`）を、何も作らず kanban にも書かずに終了コード 1 と理由1行で断る。レビュアーを起動するのは dispatcher だけで、元の課題の担当と同じ置き場に、レビューする commit の worktree を切って起動する（[dispatcher.md](dispatcher.md) §5 の P6a）。
- 作業側（別のマシン）の担当は同じ command を打つ（設計だけ。`machine` を使う起動はまだ使えない。上の `machine` の行）。command は `<worktree>.issue/outbox/` に書くだけで、dispatcher が引いて指揮側の `library/` に置き、`kanban report`／`review` を打つ（by は dispatcher。design.md §3.2 F5）。

## 5. 問いの形

オーナーが「コンテキストなしで読めて、1通で答えられる」形。2つに書く:

1. `kanban ask <id> "<text>" --kind <k>`: `questions.yaml` の `text` は人向けの描画（下の JSON から command が作る。何の決断か・事実・理由・選択肢・推奨案・あと何問）。`kind` は下の JSON の `kind` と同じ。kanban の CLI は灯りの決断（`line` まで。design.md §1.3）と `scope` を受ける（`kanban ask --kind scope`。[ledger.md](ledger.md) の Questions）。
2. `issues/<id>/library/questions/<n>.json`（`n` は kanban が付けた番号）:

```json
{"kind": "line",
 "question": "案件 repo の AGENTS.md をレベル B（独立レビュー付きで自動）に下げるか",
 "facts": ["今は **/AGENTS.md がどの repo でもレベル C", "振り返りの指針の1行がオーナーで律速", "委譲は line-change: project（§11）"],
 "reason": "アウターループの改善を無灯で入れるため",
 "options": [{"label": "下げる（独立レビュー付き）", "pick": true}, {"label": "レベル C のまま"}],
 "remaining": 2,
 "from": "gate"}
```

| キー | 要るか | 意味 |
|---|---|---|
| `kind` | 必須 | `business` / `users` / `promise` / `irreversible` / `line`（灯りの決断。design.md §1.3）か `scope`（範囲。直す課題が候補のどれを直すかを問う。宛先は Chief of Staff で、灯りではない＝ボードの「Needs you」に数えない。`facts` に `candidates.json`〈§12〉のパスと依頼書の行、`options` は候補の `id` で書き、`from` は `issue`。[ADR-0006](adr/0006-survey-fix-decide.md) 決めたこと 3・4） |
| `question` | 必須 | 1行（改行を含まない）。問いそのもの |
| `facts[]`・`reason` | 任意 | 事実と理由。写さず、パスか URL で指す。委譲の範囲外で出す問いは `facts` に「委譲 `<scope>: <値>` の外」を1行 |
| `options[]` | 必須（2つ以上） | `label` と `pick`。**先頭**が `pick: true`（推奨案）で、2つ目からは `pick` を書かないか `false` |
| `remaining` | 任意 | この問いのあとに続く問いの数（「あと N 問」。0 以上の整数） |
| `from` | 任意 | `issue` / `gate` / `retro` / `chief-of-staff` / `chief-engineer` |

範囲の問い（`kind: scope`）:

```json
{"kind": "scope",
 "question": "誤字は1つではなく3つ。どれを直すか",
 "facts": ["kanban/issues/A-0012/library/candidates.json", "依頼書の「やること」: docs の誤字を1つ直す"],
 "options": [{"label": "c1〜c3 を全部この PR で", "pick": true}, {"label": "c1 だけ。c2・c3 は別の課題"}, {"label": "全部別の課題（この依頼は閉じる）"}],
 "from": "issue"}
```

- 書き手: 担当（`andon issue ask <id> <json>`）、dispatcher（gate の `needs_human`。`from: gate`）、Chief of Staff・Chief Engineer（オーナーに出すとき）、`andon triage apply`（トリアージの判断を置くとき。`from: chief-of-staff`。§15 の下のトリアージの読み方）。command が JSON を検証して `text` を描き、`kanban ask` を打つ。Chief of Staff がオーナーの判断を新しく聞くときは `andon issue ask --new [--parent <id>] [--after <id>,…] <json>` を打つ。command は JSON を確かめてから、オーナーの課題を足し（タイトルは `question` に「（オーナーの判断待ち）」を付けたもの、`brief.md` は JSON から書いた人が読む写し）、`who: person` を書き、問いを置いて `<id>#<n>` を出す。形が違うか `kind` が `scope` なら何も足さない。同じタイトルで同じ `parent`（どちらも無いのも同じ）の片付いていない課題が問いを持たずにあれば、足し直さずにその課題へ続きを置き（途中で落ちた回の打ち直し）、問いを持っていれば断る。ただし問いがこの問いと同じ文の1つだけで、問いの控え（`library/questions/1.json`）が無ければ（`kanban ask` のあと控えを書く前に落ちた回）、控えだけを書き直して `<id>#1` を出す。`parent` の違う同じタイトルの課題は別の課題と見る。`--after` を付けた課題は `queued` で作られ、その課題にも問いを置く。`queued` の課題に問いを置けるのはオーナーの課題だけで、担当の課題は断る（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 7）。
- 読み手: Chief of Staff（`kanban questions --json` と並べて読む）・通知の口（将来）・ボードの返事欄（相談 F2）。答えは今までどおり `kanban answer`・`kanban received`。gate の問い（`from: gate`）は、課題の PR をオーナーが GitHub でマージしたら dispatcher が「オーナーがマージした（<PR>）」と答えて受け取りまで閉じる（[dispatcher.md](dispatcher.md) §2.7）。gate の判定のあとに PR の head が動いたときも、gate を回し直す前に gate の問いを「PR の head が gate のあとに動いた（<前> → <今>）。gate が今の head で回し直す」と答えて閉じ、回し直した gate が今の head で決めたときは、前の head の gate の問いを「PR の head が動いた（<前> → <今>）。gate が今の head で決め直した（記録 <パス>）」と答えて閉じる。この3つの機械の答えは、§14 の `answer` にも `override` にも数えない。
- 誰が決めたかの印（[ADR-0049](adr/0049-owner-touches-by-kind.md) 決めたこと 5・6）: 答えを書く者は `kanban answer <id>#<n> --decided-by <値> "<答え>"` で、答えと一緒に `questions.yaml` の `decidedBy` を書く。値は `owner`（オーナーがその問いに答えた）・`chief-of-staff`（Chief of Staff が範囲の問いに答えた・委譲の中で自分で決めた・要らなくなった問いを片付けた）・`machine`（command が事実を見て閉じた）の3つで、kanban の CLI がほかの値と名前の違う旗を断る。コードの書き手は旗を必ず付ける。`andon waiting answer` は `owner`、`andon issue settle`・`andon issue merge` が gate の問いを閉じる上の3つの答えは `machine` を書く。旗の無い答えはキーを持たない（印の無い答え）。

## 6. 通知

dispatcher の中では JSON。届けるときだけ `<news>` の文に描く（文は JSON から作る。文から推さない）。拾い方・届ける経路・離席の判定の理由は [dispatcher.md](dispatcher.md)。
`kind`（と今ある項目）ごとに「届ける」か「記録だけ」かが決まっている（下の「届けるか」。ADR-0010 決めたこと 1・7）。記録だけの通知は `<news>` に載らず、下の記録だけの置き場に1行ずつ書かれる。

```json
{"at": "2026-11-04T09:00:00Z", "project": "andon", "issue": "A-0012", "kind": "review",
 "detail": {"report": "/workspace/projects/andon/kanban/issues/A-0012/report.md", "pr": "https://github.com/<owner>/andon/pull/12"}}
```

| `kind` | 元 | `detail` |
|---|---|---|
| `report` | `log.jsonl` の `{"report": "write" \| "append"}` | `report` のパス |
| `review` | `{"status": "review"}` | `report`・`pr` |
| `asked` | `{"question": n, "status": "open"}`。dispatcher 自身が出した問い（`by` が `terminal:dispatcher`）は出した巡で積み、この行からは積み直さない（[dispatcher.md](dispatcher.md) §2.2） | `n`・`kind`・`question`（§5 の JSON から） |
| `answered` | `{"question": n, "status": "answered"}` のうち、書き手（`by`）が `terminal:` で始まらないもの。オーナーのペインの `andon waiting answer`（§25）が端末の外で書いた答えに当たる。Chief of Staff・dispatcher・担当など端末の中で書いた答えは積まない。読んだときに問いがもう `answered` でなければ（担当が受け取った）積まない。`questions.yaml` が読めなければ1回だけ次の見回りまで待って読み直す（[ADR-0042](adr/0042-owner-waits-do-not-stop-the-line.md) 決めたこと 18・19） | `n`・`kind`（§5 の JSON から。無ければ `questions.yaml` の `kind`）・`answer`（`questions.yaml` の答えの文） |
| `ready` | `kanban ready --json` の前回との差（オーナーの課題を除く） | `title`・`routine`、待つ相手（`after`）のうち PR を成果にする課題がマージされずに done になったものがあれば `abandoned[]`（id）、マージされたかを確かめられなかったものがあれば `unchecked[]`（id。見分け方は [dispatcher.md](dispatcher.md) §2.2） |
| `review-requested` | `report.json` の `review_request`（dispatcher が起動したとき） | `round`・`reviewer`・`reviewer_issue` |
| `gate` | dispatcher が `andon gate` を回した結果（`wait` は通知しない。§7） | `decision`（`approve` / `fix` / `needs_human`）・`reasons[]`・`record`（§7 のパス）、`fix` を担当へ送れなかった（担当の端末が無い）ときだけ `unsent`（担当の address）、`fix` で担当（`agent`）がいないときだけ `no_agent`（`true`） |
| `merged` | gate の approve ＋ `merge: auto` ＋ 委譲の範囲内でマージして finish したとき。または `review`・`doing` の課題の PR をオーナーがマージしたのを見つけ、gate の問いを閉じて finish したとき（`by: person`）・gate の外の問いが開いている／finish が失敗して（commit していない変更など）片付けなかったとき（`by: person`・`held`。同じ理由では1回。dispatcher.md §2.7） | `pr`・`sha`、オーナーのマージのときだけ `by`（`person`）と、片付けなかったときだけ `held`（理由） |
| `closed` | `review` か `doing` の課題の PR がマージされずに閉じた（片付けない。閉じるたびに1回だけ。dispatcher.md §2.7） | `pr` |
| `impact` | `log.jsonl` の `{"status": "done"}`（Chief of Staff が打った finish も拾う）で、PR が MERGED の課題に dispatcher が `andon issue impact` を1回打ち、影響のある課題があったとき（下の「マージのあとの影響」） | `pr`・`sha`・`issues[]`（`issue`・`conflict`・`overlap[]`・`brief_refs[]`・読めなかったときだけ `error`。ファイルは20まで）。集められずに諦めたとき（3回失敗）は `pr`・`error` だけ |
| `corrections` | `log.jsonl` の `{"status": "done"}` の課題の `corrections.json`（§14）を、次の見回りの打ち直しでも集められなかったとき（書いていない） | `error` |
| `blocked`・`error`・`finished`・`stalled` | hcの状態（`hc events` か見回り） | `agent`・`status_ms`（hc の `status_ms`。その状態に入った時刻の Unix ミリ秒）。画面の文は入れない |
| `resumed` | 見回りで `doing` の課題の端末が Gone（無い・live でない・素のシェル）を見て、dispatcher が `andon issue resume` を1回打って起動できたとき（dispatcher.md §2.9。起動できなければ `finished`） | `agent`（起動した端末）・`from`（Gone を見た端末） |
| `estimate` | 見回りで、`doing` の担当の課題（レビュアー・Chief Engineer・オーナーの課題を除く）の実働（待ちを除いた時間。§2 の `Worked`）が、見込み（§2 の `estimate`。Chief of Staff が書いたものだけ）の 1・2・4 倍を超えたとき。段ごとに1回（一度に複数の段を超えたら上の段だけ）。見込みの無い課題には出さない。担当には何も送らない（dispatcher.md §2.10。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 6〜8） | `estimate`（段の値）・`estimate_minutes`（見込みの分）・`worked_minutes`（実働の分。切り捨て）・`ratio`（実働÷見込み。小数2桁）・`stage`（`1`・`2`・`4`）・`sign`（詰まりの兆し。兆しがあるときだけ。下の表） |
| `signal` | 信号の記録（§15）の `weight: now` の信号を、出来事（`episodes`）ごとに最初に見たとき（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・8。`later` は通知しない） | `source`・`key`・`url`・`title`・`weight`（記録の写し）・`release`（記録にあるときだけ。同じ値。`github-ci` なら失敗が始まった run の commit で、Chief of Staff が壊した変更の候補の PR を引く。`<news>` には先頭の 12 桁。[ADR-0051](adr/0051-signals-flow-and-breaking-change.md) 決めたこと 7・8）。`title` は外の人の文なので、`<news>` に描くときは「信用しない」の囲いに入れる |
| `memory` | 見回りで、セルの auto memory の置き場（`<ANDON_WORKSPACE>/memory` の直下の `.md`。目録 `MEMORY.md` を除く）の件数が線（30 件）を超えたとき。線以下に戻るまで1回で、kanban が2つ以上あってもセルに1通。`issue` は無い（dispatcher.md §2.11。[ADR-0020](adr/0020-auto-memory-as-intake.md) 決めたこと 4） | `count`（件数）・`line`（線） |
| `idle` | 見回りで、`project.json` の `chief_of_staff.idle_minutes` 分（既定 15）おきに枠を見て、前の回も今の回も枠が 1 つ以上空いていた回のうち、前の回にも今の回にも `kanban ready` に出ている課題（オーナーの課題とレビュアーの子課題を除く。同じ課題は ready に出ている間 1 回だけ）・この回に撃ったルーチン・前に知らせた結果と違う `kanban lint` の problem のどれかがあるとき。届く前に次の `idle` を積むときは、前の `unstarted` と `fired` を新しいほうに足し、新しいほうの `lint` が 0 なら前の件数を残す。`issue` は無い（dispatcher.md §2.13。[ADR-0047](adr/0047-idle-slots-run-upkeep.md) 決めたこと 11・12） | `free`（今の回の `slots.free`。1 以上）・`unstarted`（起動されないまま残った課題の id の配列）・`fired`（この回に撃ったルーチンの名前の配列。撃った順）・`lint`（`kanban lint` の problem の件数。前に知らせた結果と同じか 0 件なら 0） |
| `hold` | `log.jsonl` の `{"status": "done"}` の課題を理由に止めている課題（§2 の `hold`）が、まだ止めているとき（止めは理由の課題が済んでも外れない。done の行は1回だけ読むので1回だけ。[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 7） | `reason`（done になった理由の課題の id） |

- **届けるか**（表の正は `internal/dispatcher/route.go` の `RouteOf`。理由は ADR-0010 決めたこと 1・7）:

  | `kind`（と条件） | 扱い |
  |---|---|
  | `review-requested`・`resumed` | 記録だけ |
  | `review`（dispatcher が gate を回す課題） | 記録だけ |
  | `review`（gate を回さない課題のうち PR のあるもの: Chief Engineer の課題〈`profile: chief-engineer`〉など。PR の無い調べる課題・観察の課題は `report` で届く） | 届ける |
  | `gate`（`decision: fix`・`needs_human`。`unsent`・`no_agent` が無い） | 記録だけ（`needs_human` は同じ巡の `asked` で届く） |
  | `gate`（`decision: fix`。`unsent` あり。担当へ送れなかった） | 届ける |
  | `gate`（`decision: fix`。`no_agent` あり。担当がいない） | 届ける |
  | `gate`（`decision: approve`。dispatcher がマージしない: `merge: person`・委譲の外・`merge: auto` を最後に書いたのが Chief of Staff でない） | 届ける（すぐ） |
  | `gate`（`decision: approve`。`merge: auto` でマージに進む） | 記録だけ。`andon issue merge` の記録の `approved` から10分たってもマージ・問い・別の答えのどれも決まらなければ、そのとき同じ形の通知を1回だけ届ける（`review` から出た課題は数えない） |
  | `merged`（`held` が無い。Chief Engineer の課題でない） | 記録だけ |
  | `merged`（`held` が無い。Chief Engineer の課題: `profile: chief-engineer`。オーナーが文書の PR をマージした） | 届ける |
  | `merged`（`held` あり。片付けていない） | 届ける |
  | `report`（レビュアーの課題: `parent` があり `profile` が `*-review`） | 記録だけ |
  | `stalled`・`blocked`・`ready`（Chief Engineer の課題: `profile: chief-engineer`） | 記録だけ |
  | `report`（ほか）・`asked`・`answered`・`ready`・`impact`・`closed`・`corrections`・`signal`・`blocked`・`error`・`finished`・`stalled`・`estimate`・`memory`・`idle`・`hold` | 届ける |

- **記録だけの置き場** `~/.local/state/andon/dispatcher/<slug>.recorded.jsonl`（`XDG_STATE_HOME` があればその下。控えと同じ所）: 1行が1件の通知で、形はこの節の通知の JSON そのもの（`schema/notification.schema.json` に通る）。書き手は dispatcher だけで、記録だけと決めた通知を、待ちに積む代わりにここへ1行足す。追記だけ（書き換えも切り詰めもしない）。書けなければ控えに残し、次の見回りで先に書く（記録から落とさない・順を保つ）。一部だけ書いて失敗したら書く前の大きさに戻し、書き直すときはファイルの末尾にもう在る行を飛ばす（控えを保存する前に落ちても二重に書かない）。読み手はボード（記録の一覧）と `andon measure`（§13 の `news`）。届けた通知はここに書かない（渡したターンの終わりに §17 の記録に書く）。全部の出来事の一覧は kanban の `log.jsonl` が正で、この記録はその代わりではない。
- `estimate` の `sign` は、段の通知を積むときに課題の worktree（`issue.yaml` の `worktree`）の git で数えた兆しの配列（[ADR-0017](adr/0017-process-definitions.md) 決めたこと 11）。段の間には数えず、兆しだけで通知を出さない。worktree が無い・git で読めない兆しは数えない。

  | 値 | 兆し |
  |---|---|
  | `no-commit` | worktree の `HEAD` の commit（一度も commit していなければ切った元の commit）から、見込みの分より長く経った |
  | `large-diff` | `origin/HEAD` との merge-base から作業ツリーまでの変更（追跡しているファイル）の、足した行と消した行の和が 500 を超えた。テスト（`tests` という名のディレクトリの下と、名前が `*_test.*` のファイル）とバイナリは数えない |

- `at` は UTC の `YYYY-MM-DDTHH:MM:SSZ`（暦にある日時。gate の記録の `at` も同じ）。キーは `at`・`project`・`kind`・`detail` が必須、`issue` は課題についての通知のときだけ。`detail` のキーは `kind` ごとに上の表のものだけで、全部必須（`ready` の `routine`・`abandoned`・`unchecked`、`gate` の `unsent`・`no_agent`、`merged` の `by`・`held`、`estimate` の `sign` だけ任意。`unsent` と `no_agent` は `decision: fix` のときだけで、一緒には書かない。`held` は `by` と一緒）。
- 届け方（届ける通知だけ）: ①Chief of Staff の `Stop` フックが `andon dispatcher drain` を打ち、待ちがあれば `{"hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": "<news>…</news>"}}`（続きのターンになる）。②`UserPromptSubmit` フックが `additionalContext` に添える（委譲の一覧 `andon delegate show --json` も同じ口で添える）。どちらも同じ JSON から同じ文を描く。③オーナーが離席の間は、Chief of Staff の `Stop` フックが `asyncRewake` で起動する待ち役（`andon dispatcher wait`）が、届ける待ちがあれば合図だけを打って Chief of Staff を起動し、そのターンの終わりの①で届く。dispatcher は Chief of Staff の入力欄へ打たない（[ADR-0019](adr/0019-cos-input-box.md) 決めたこと 1・8）。
- **オーナーのマージの片付け**（`merged`（`by: person`）・`closed`）: `andon issue settle <id>` は、`review` か `doing` の課題の PR をオーナーがマージしていたら、gate の問いを閉じて finish する（dispatcher.md §2.7）。dispatcher が打つのは、インナーループを通る課題と、`review` か `doing` にいる Chief Engineer の課題（Chief Engineer の課題は `doing` の間は片付けず、印を揃えるだけ）。積む通知は stdout の JSON で返し、待ちに積むのは dispatcher。
  出力は `issue`・`state`（`skip`〈`review` にいない・`pr` が無い・別の repo の PR〉・`watching`〈まだ OPEN か、1分たたず聞いていない〉・`auto`〈gate の記録の `merged` が `true`。dispatcher がマージした〉・`closed`・`held`・`finished`）・`notify[]`（`kind` と `detail`。形は上の表の `merged`・`closed`）。
  記録は `~/.local/state/andon/dispatcher/<slug>.settle.json`（課題 → `pr`・`commit`・`at`・`state`〈`MERGED`・`CLOSED`〉・`head`・`sha`・`held`。書き手は settle だけ）。2回打っても同じ結果になる（GitHub に聞く・知らせる・片付けるのは1回）。CLOSED と記録した PR も1分に1回聞き直し、開き直してマージされたら片付ける。出力と記録は Chief of Staff と dispatcher だけが読むので schema にしない（通知の `detail` が約束）。
- **gate のあとのマージと片付け**（`gate`・`merged`・`asked`・`report`）: `andon issue merge <id>` は、`review` の課題に gate（§7）を `review` に入った `log.jsonl` の行ごとに1回だけ回し、approve ＋ `merge: auto` ＋ 委譲の範囲内ならマージして finish する。ただし approve か `needs_human` を決めたあと（dispatcher がマージしていない）に PR の head が判定した head から動けば、前の head の gate の問い（`from: gate`）を閉じて、今の head で gate を回し直す。PR の head を聞くのは課題ごとに1分に1回。`fix` は理由を担当へ `hc send`（文の先頭に送り手の印 `[gate] `。dispatcher.md §5。送れなければ通知 `gate` に `unsent`、担当がいなければ `no_agent`）、`needs_human` とマージできなかったときは問い（`from: gate`）を出す。ただし `gh pr merge` の失敗が「Base branch was modified. Review and try the merge again」なら問いにせず、次に打ったとき gate から照合し直して打ち直す（3回まで。超えたら問い。dispatcher.md §5 の P6a）。積む通知は stdout の JSON で返し、待ちに積むのは dispatcher。
  出力は `issue`・`state`（`skip`・`wait`〈checks を待つ〉・`rate-limited`〈gh の上限。いつ回し直すかは dispatcher が kanban ごとに決める〉・`regate`〈マージの直前に読み直して変わった・base が動いて `gh pr merge` が通らなかった。次に打ったとき回し直す〉・`hold`〈Chief of Staff が止めている（§2 の `hold`）。gate を回さず GitHub も読まない。approve か needs_human を決めたあとに止められたら、判定を記録から捨てて gate の問いを閉じる。止めてから外すまでに一度も打たれなかったときも、gate の記録の時刻以後に `hold` を書いた行が log にあれば、同じに捨てて今の head で gate から回す（印の揃えも、その判定から緑に戻さない）。外したら次に打ったとき今の head で gate から回す。[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 9〉・`approve`・`fix`・`needs_human`・`merged`）・`auto`（通知 `gate` の approve のあとマージに進む）・`log[]`（dispatcher のログに書く行）・`notify[]`（`kind` と `detail`。形は上の表）。
  記録は `~/.local/state/andon/dispatcher/<slug>.merge.json`（課題 → `row`〈review に入った行〉・`sha`・`decision`・`merged`・`wait`〈checks を待つ head〉・`approved`〈`merge: auto` の approve でマージに進み始めた時刻。決まったら消す〉・`retries`〈base が動いて通らなかったマージを打ち直した回数。決まったら消す〉・`polled`〈approve か `needs_human` を決めたあと、PR の head を最後に読んだ時刻〉。書き手は merge だけ）。dispatcher は `approved` を読んで「approve のまま10分」を届ける（§3.2 の表）。記録が無ければ、その行のあとに書かれた今の head（`report.json` の `commit`）の gate の記録を引き継いで問いや通知を出し直さない。2回打っても同じ結果になる（gate・マージ・問い・通知は行ごとと head ごとに1回）。出力と記録は Chief of Staff と dispatcher だけが読むので schema にしない（通知の `detail` が約束）。
- **マージのあとの影響**（`impact`）: `andon issue impact <id>` は、課題の PR がマージされたあと、同じ repo の `doing`／`review` の担当の課題（レビュアーを除く）ごとに、機械で分かる事実だけを JSON で出す（読むだけ。書くのは `git fetch` だけ。PR がまだ MERGED でなければ終了コード 4）。
  出力は `issue`・`pr`・`sha`（マージの commit）・`files[]`（マージした PR が触ったファイル）・`issues[]`。`issues[]` の1件は `issue`・`title`・`status`・`agent`・`branch`・`head`・`pr`・`contains`（branch がもうマージの commit を含む）・`conflict`／`conflict_files[]`（branch とマージの commit を `git merge-tree` で合わせてぶつかるか）・`overlap[]`（両方が触ったファイル。worktree の commit していない変更と、まだ `git add` していない新しいファイルも数える）・`brief_refs[]`（マージした PR のファイルのうち、その課題の依頼書に path か名前が出るもの）・`error`（読めなかった理由。worktree があるのに読めない・在るかを確かめられない〈権限など。無いのとは分ける〉ときも。影響なしに倒さない）。
  影響あり（通知に載る）は `error` があるか、`contains` でなく `conflict`・`overlap`・`brief_refs` のどれかがあるもの。判断（影響なし・rebase を頼む・止めて向きを確かめる）は Chief of Staff（chief-of-staff skill の `playbooks/finish.md`）。出力そのものは Chief of Staff と dispatcher だけが読むので schema にしない（通知の `detail` が約束）。

```json
{"at": "2026-11-07T09:00:00Z", "project": "andon", "issue": "A-0012", "kind": "impact",
 "detail": {"pr": "https://github.com/<owner>/andon/pull/12", "sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
            "issues": [{"issue": "A-0014", "conflict": true, "overlap": ["docs/design.md"], "brief_refs": ["docs/design.md"]}]}}
```

```json
{"at": "2026-11-10T09:00:00Z", "project": "sample-site", "kind": "signal",
 "detail": {"source": "github-ci", "key": "github.com/example/sample-site/ci.yml@main", "url": "https://github.com/example/sample-site/actions/runs/1",
            "title": "ci: test failed on main", "weight": "now", "release": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}}
```

- `<news>` の文の形は仮設のまま（1行1件・末尾の「オーナーからの新しい依頼ではない。オーナーに言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える」。ADR-0010 決めたこと 3）。`news.replied=<turn>` のラベルの意味（そのターンはオーナーが見る返事ではない）・名前・値も同じ（ボードとの約束）。
- **「オーナーに言うことが無い」の申告**（[ADR-0010](adr/0010-news-ack.md) 決めたこと 4〜6）: Chief of Staff は `andon news ack` を打つ。command は自分の端末（`HC_TERMINAL_ID`）を `hc get` で読み、ラベル `role=chief-of-staff` があればそのときの `turn` で `news.ack=<turn>` を書く（上書き。無ければ書かずに終了コード 1）。dispatcher は `<news>` を渡すときにその端末の `turn` を控え（控えの `hook`。③の待ち役は合図だけで、渡すのは①）、渡したターンが終わったとき `news.ack` が控えた値と同じなら `news.replied=<そのときの turn>` を書く（答えの文は読まない）。経路①②では、渡すときにラベルがもうその値だった（同じ turn の前の ack。`Stop` の block の続きは hc の `turn` が進まない）ら数えない。確かめ済みにした ack は、dispatcher がラベル `news.ack` を消す（`hc label <端末> news.ack=`）ので、同じ turn の続きで次の `<news>` を渡すと、その2回目の ack も数える。ack の無い2回目は書かない。ack の無いターンは、答えが `<no-reply>` だけでも書かない（オーナーが見る返事に数える。`<no-reply>` の照合は ack を入れたバージョン〈v0.0.8〉の次のバージョンで外した。決めたこと 6）。書いた数は控えに数える（`replied`）。
- dispatcher の控え `~/.local/state/andon/dispatcher/<slug>.json`（`XDG_STATE_HOME` があればその下）: `log.jsonl` の読んだ位置（相談 F1 が入れば cursor）・前回の ready・待ちの通知・フックで渡した端末・起動したレビュアー（`issue` → `reviewer_issue`・`commit`）・回した gate（`issue` → `sha`）。kanban には書かない（kanban に書くのは kanban の CLI 経由の行だけ。dispatcher の行の `by` は `terminal:dispatcher`）。控えの中の形は dispatcher だけが読むので約束にしない（schema は無い）。

## 7. gate の条件表と記録

**条件表** `package/gate.json`（既定）＋ セルの `gate.local.json`（上書き。深い併合ではなく、キーごとの置き換え）。仮設の gate の条件表のキー（`size_review_lines`・`size_human_lines`・`test_paths`・`tier_a_paths`・`tier_b_paths`・`tier_c_paths`・`tier_c_path_exceptions`・`tier_c_added_line_patterns`・`repos{merge, strip_prefix, check[]}`）に、`repos.<repo>.tier_overrides`（パスの glob → レベル）・`repos.<repo>.github_checks`・`github_checks_wait_minutes`・`screen_paths`（下）を足す。既定の条件表で案件 repo の `AGENTS.md`・`REVIEW.md` は **B**（オーナーの決定 2026-11-05。design.md §6 問い7。Andon の repo とセルの repo は C のまま）。条件表を変えるのはレベル C（オーナー）。

```json
{"size_review_lines": 500, "size_human_lines": 1000, "github_checks_wait_minutes": 30,
 "test_paths": ["tests/**"],
 "tier_a_paths": ["docs/**", "tests/**", "README*"],
 "tier_b_paths": ["package/lib/**", "**/*.sh"],
 "tier_c_paths": ["image/**", ".github/workflows/**", "package/gate.json", "**/AGENTS.md", "**/.claude/**"],
 "tier_c_path_exceptions": {"**/.claude/**": ["**/.claude/skills/**"]},
 "tier_c_added_line_patterns": ["\\bsudo\\b", "curl[^|]*\\|\\s*(ba)?sh"],
 "repos": {"github.com/<owner>/sample-site": {"merge": "squash", "check": ["bash tests/*.sh"], "github_checks": "required",
                                            "tier_overrides": {"AGENTS.md": "B", "REVIEW.md": "B"}}}}
```

- キーはどれも必須ではない（`gate.local.json` は置き換えるキーだけを持つ）。知らないキーは拒む。`repos.<repo>.merge` は `merge` / `squash` / `rebase`、`tier_overrides` の値は `A` / `B` / `C`。重ねたあとに `size_review_lines`・`size_human_lines`・`github_checks_wait_minutes` が無ければ、gate は誤り（終了コード 3）。
- `repos.<repo>.github_checks`: PR の head に GitHub の checks（Actions の CI など）が要るか。`required`（既定）は1つも無ければ待ち、`optional` は1つも無くても通す（CI の無い repo）。どちらでも、在る checks は通っていなければ止める。
- `github_checks_wait_minutes`: `required` の repo で checks が1つも無いとき、課題が `review` に入った `log.jsonl` の行の `at` からこの分だけ待ち、過ぎたら `needs_human`（push の直後で checks がまだ付いていないのと、CI が無いのとを分ける）。
- `screen_paths`: 画面のファイルの glob の配列（既定は無し＝空。案件の `gate.local.json` で `web/**`・`**/*.css` などを指す。ADR-0018 決めたこと 2）。PR の変更されたファイル（`strip_prefix` を外し、rename の元も）が当たり、PR の本文とコメントのどれにも GitHub の添付の URL（`https://github.com/user-attachments/assets/<id>`。`gh pr create`／`edit`／`comment --attach` と Web の画面で上げた画像・動画）が無ければ `fix`、`reasons[]` に当たったファイルごとに `screen_paths: <file> に当たるが画面の証拠（添付）が無い`。外の画像の URL・書き換えられていない相対パス（`![x](./a.png)`）・画像でない添付（`user-attachments/files/`）は添付と数えない。どの commit で撮ったかは見ない。
- `tier_c_added_line_patterns`: 差分の追加行に当てる正規表現の配列。gate は差分を hunk ごとに読み、パターンに当たった文字列を追加行と削除行で数える（hunk の中は見出し `@@ -a,b +c,d @@` の行数で数えるので、行の頭が `+++`・`---` の行も追加行・削除行に数える。見出しの `--- a/…`・`+++ b/…` は数えない）。同じ文字列の数が追加行のほうで多い hunk が1つでもあれば、`tier_reasons[]` に `追加行 <パターン>` を書いてレベル C にする。追加行が当たっていても、どの hunk でも増えていなければ、`追加行 <パターン>（増えていない）` をレベル B の理由にする（もとからあった語を書き換えた行で、オーナーでなく独立レビューが見る。`docs/**` だけの差分も A にならない。[ADR-0041](adr/0041-delegation-cap-out-of-command.md) 決めたこと 4）。新しいファイルと足すだけの hunk は C になり、別の hunk の削除行では打ち消さない（ファイルを消して別の名前で足した変更も C）。削除行だけが当たるときは理由にしない。どのパスにも、どのパターンにも同じ決まりを使う。

**記録** `issues/<id>/library/gate/<head sha>.json`（`andon gate` が書く。`--dry-run` では書かない。dispatcher の道では `andon issue merge` が回し、マージしたら `merged` を書く）:

```json
{"at": "2026-11-04T09:10:00Z", "id": "A-0012", "pr": "https://github.com/<owner>/andon/pull/12", "head": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "tier": "B", "tier_reasons": ["internal/issue/issue.go は tier_b_paths"], "changed_lines": 212,
 "checks": [{"cmd": "bash tests/*.sh", "ok": true}],
 "github_checks": {"state": "success", "names": ["tests"]},
 "review": {"state": "current", "issue": "A-0013", "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "must_fix_open": 0, "waived": 1},
 "acceptance": {"pass": 3, "fail": 0, "manual": 0},
 "delegation": {"merge": "B", "ok": true},
 "decision": "approve", "reasons": [], "merged": true}
```

- `at` は gate が PR の head の GitHub の checks を読んだ時刻（`github_checks` はそのとき読んだもの）。gate を始めた時刻ではない（条件表の `check` を回すあいだに checks が変わっても、記録は読んだ時刻と中身で揃う）。
- キーは全部必須。`review.state` は `current`（`findings.json` の `commit` が `head`、または PR の差分が同じ。§3）/ `stale`（違う）/ `missing`（無い・読めない）/ `off`（課題の `review: off`）。`current`・`stale` では `issue` と `commit` が必須、`missing`・`off` では2つとも書かない。`delegation.merge` は委譲が無ければ `none`。
- `decision` と `reasons[]` は決定式（純粋関数）の出力そのまま。PR のコメントと commit status（`andon/gate`）の説明文は、記録から作った**外の人が読む文**で、記録の文をそのまま写さない（記録の正は kanban の側。[ADR-0026](adr/0026-pr-rules-for-outside-readers.md) 決めたこと 9）。内輪の語（`needs_human`・`tier`・`delegation`・`report.json`・`issue.yaml`・条件表）を使わない。
  - 頭は必ず `andon gate: `（dispatcher がこの頭で gate のコメントを見分け、オーナーの手直しから除く。§14）。続けて判定ごとの見出し: `approve` は「確かめが通った変更です」、`fix` は「作った側が直してから出し直す変更です」、`needs_human` は「メンテナーの判断が要る変更です」。commit status の説明文は、`approve` 以外では見出しに最初の理由（コメントと同じ外向けの文）を括弧で足し、理由が 2 件以上なら「、ほか N 件」を足す（例「andon gate: メンテナーの判断が要る変更です（自動でマージしてよい範囲を超える変更）」）。140 字を超えるときは理由を「…」で切る（[ADR-0054](adr/0054-gate-status-by-decision.md) 決めたこと 5）。
  - コメントの本文は、見出しと commit のあとに、理由（`reasons[]` を1件ずつ外向けの文に直したもの）・変更の区分（`tier` の A／B／C を「自動の確かめだけで足りる変更」「レビューが要る変更」「メンテナーが見る変更」に、`tier_reasons[]` を外向けの文に直したもの）と変更の行数・検査ごとの結果・CI（`github_checks`）・レビュー・自動でマージしてよい範囲の中か外か。
  - 外向けの文の表は `internal/gate/comment.go` にある。表に無い記録の文は「そのほかの理由」と書き、記録の文を外へ出さない（記録の文を足したら表にも足す。足し忘れは `go test ./internal/gate/` が落とす）。
- **印**（commit status `andon/gate`。印の決まりの正はここ。[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 3・18、[ADR-0054](adr/0054-gate-status-by-decision.md) 決めたこと 8）: gate と dispatcher が PR の今の head に書く。状態ごとの印は次の表のとおり。

  | 状態 | 印 | 説明文 |
  |---|---|---|
  | 判定の前（レビュー中・担当が直している・CI を待っている。gate の `wait`） | 書かない | — |
  | 通した（`approve`） | 緑（`success`） | 「andon gate: 確かめが通った変更です」 |
  | オーナー待ち（`needs_human`） | 緑（`success`） | 「andon gate: メンテナーの判断が要る変更です（<最初の理由>）」 |
  | 差し戻し（`fix`） | 赤（`failure`） | 「andon gate: 作った側が直してから出し直す変更です（<最初の理由>）」 |
  | Chief of Staff が止めている（§2 の `hold`。`andon issue hold`） | 緑だった印を黄（`pending`）に書き換える。渡していない head（判定の前・`fix`）には書かない | 「andon gate: メンテナーの確認を待っています」 |
  | 判定のあとに、課題が `doing` に戻った | 緑だった印を赤（`failure`）に書き換える | 「andon gate: 作った側が直しています」 |
  | Chief Engineer の文書の PR が `review` に入った（gate を通らない） | 緑（`success`） | 「andon gate: 仕様・設計の文書です。メンテナーがマージします」 |

  - 緑は「マージしてよい」を指す。オーナー待ちはオーナーがマージしてよい状態なので緑にし、赤くしない。担当が直している commit は、前に渡した commit でも今は渡していないので赤にする。`doing` に戻ったときの説明文は、ADR の文（「担当が直しています」）を、外の読み手に内輪の語を出さない決まりに合わせて「作った側が」と書く。
  - 同じ commit に同じ状態と説明文の印が既にあれば、書き直さない。ただし、その印にリンク（`target_url`）が無く（前のバージョンの印か、コメントを書けなかった回の印）、gate が今回コメントの URL を得たときは、リンクを付けて書き直す（ADR-0054 決めたこと 6）。
  - gate は判定のたび、コメントを先に書き、`gh pr comment` が出したコメントの URL を印のリンク（`target_url`）にする。コメントを書けなかったときは、リンクの無い印を書く（ADR-0054 決めたこと 6）。`wait` は印もコメントも書かない。
  - 決めるのは純粋関数（`internal/gate/mark.go` の `WantMark`・`DecisionMark`）で、表のテストは `go test ./internal/gate/`。
  - **dispatcher が揃える**（ADR-0056 決めたこと 5・6）: dispatcher が見回りで打つ `andon issue settle`（§13 の「オーナーのマージの片付け」）は、PR の状態を聞く 1 分に 1 回の読み（`gh pr view --json state,mergeCommit,headRefOid,statusCheckRollup`）で、開いている PR の今の head の印を読み、上の表と違えば書き直す。GitHub に聞く回数は増やさない。あるべき印は、その head を gate が判定した結果（`library/gate/<head>.json` の `decision`・`reasons`）と、課題の status（`review`・`doing`）と、Chief Engineer の課題か（`profile: chief-engineer`）から決める。判定の無い head（gate の前・`wait`・gate のあとに push された head）には書かない。使う判定は、dispatcher の merge の記録（`<slug>.merge.json`）が、課題が今 `review` に入っている行（`log.jsonl` で最後に `review` に入った行）についてこの head の判定を持つときだけ（同じ head のまま `review` に入り直し、今の回の gate がまだ決めていなければ、前の回の判定から書き戻さない。時刻は秒までで、gate は `review` に入った秒と同じ秒に判定を書くことがあるので、時刻では比べない）。閉じた PR とマージされた PR には書かない。`gh pr view` の `statusCheckRollup` は説明文を出さないので、今の印とは状態だけで比べる（前のバージョンが書いた、オーナー待ちの赤い印も緑に直る。ADR-0054 決めたこと 7 をこの部分だけ覆す）。書くのは課題の鍵（`<slug>.issue.lock`）の中で、書けなければ settle の出力の `log` に書いて次の読みで書き直す。dispatcher は `log` の行を自分のログに写す。
  - 残る隙間: 課題が `doing` に戻ってから dispatcher が次に PR を読むまで（長くて 1 分ほど）、前の緑が残る。
- **決定式の順**（`internal/gate/decide.go` の `Decide`。[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 4）: kanban と PR が合わない（`fix`）→ PR が open でない・draft（`fix`）→ PR の事情（衝突・base・作者・fork・ファイルの件数。`needs_human`）→ 予約キーを信じられない（`needs_human`）→ 条件表に repo が無い（`needs_human`）→ 条件表の検査 → GitHub の checks が落ちた（`fix`）→ レビュー → 受け入れ条件 → 実行の記録 → 画面の証拠 → GitHub の checks を通すか決められない（`needs_human`）・CI の待ち（`wait`）・CI が来ないまま待ちを過ぎた（`needs_human`）→ **委譲の外（`needs_human`）** → `approve`。委譲の範囲の外かは、通す直前に見る。差し戻す理由があれば、オーナーがマージする変更でも先に担当へ差し戻し、CI が走っている間は待つ。オーナー待ちの理由が委譲だけなら、検査・CI・レビュー・受け入れ条件は通っている。委譲の外の変更でも、GitHub の checks を通すか決められないときと CI が来ないまま待ちを過ぎたときも、条件表の検査は回す（オーナー待ちの緑の印を付ける前に、差し戻す理由を拾う）。衝突は `mergeStateStatus` が `DIRTY` か、`mergeable` が `CONFLICTING` のとき（必須の check が足りない PR は `BLOCKED` になり、衝突と重なったときの順は GitHub の文書に無い。決めたこと 13）。
- `github_checks`: PR の head の GitHub の checks（GraphQL の `pullRequest.headRefOid` と head の commit の `statusCheckRollup.contexts` を、100件ずつ全ページ読む。ページの間に head が動いたら判定しない。gate 自身の commit status `andon/gate` は数えない。同じ check〈CheckRun は app・workflow の id〈表示名でなく GraphQL の `Workflow.id`。別ファイルの同じ表示名の workflow を束ねない〉・名前、status は context〉の実行が複数あれば、最新〈CheckRun は `databaseId`、status は `createdAt`〉の1つだけを数える＝再実行の前の failure を数えない。新旧を判別できない〈同じ時刻・同じ id〉ときは悪い方を残す）を1つの状態にまとめたもの。`state` は一番悪いもの: `failure`（failure・cancelled・timed_out・startup_failure・status の error）＞ `unknown`（action_required・stale・知らない値）＞ `pending`（終わっていない・status の pending・expected）＞ `success`（success・skipped・neutral。neutral は GitHub のブランチ保護と同じく通す）。1つも無ければ `none`（gate 自身の `andon/gate` だけのときも）。`names` はその状態にした checks の名前（`success` では通した checks 全部。`none` では空）。
- GitHub の checks の決定: `failure` → `fix`（担当が直せるもの）。`unknown` → `needs_human`。`pending`、または `required` の repo で `none` → **`wait`**（ほかに止める理由が無いときだけ。止める理由があればそちらを返す）。`none` のまま `github_checks_wait_minutes` を過ぎたら `needs_human`。`pending` は時間でオーナーへ上げない（走り終われば success か failure になる）。checks が通っていないあいだは条件表の `check` を回さない。
- **止めている課題は判定しない**（[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 9・10）: `andon gate` は gate の鍵（`library/.gate.lock`）を取ってから課題を読み直し、§2 の `hold` で止めていれば（値が空でも、最後に消したのがこの課題の担当かそのレビュアーの端末なら止めているものとして）、GitHub を1度も読まずに `{"decision": "wait", "reasons": ["Chief of Staff が PR を止めている（理由の課題 <id>）。…"]}` を出して終了コード 5 で返す。記録・コメント・印は書かない。`--dry-run` も同じ。`github_checks_wait_minutes` は数えない（止めている間に過ぎても `needs_human` にしない）。止めを読めなければ止めているものとして `wait`。
- **止める口**（`andon issue hold <id> <理由の課題>`・`andon issue unhold <id>`。Chief of Staff の端末で打つ。決めたこと 7・8）: どちらも課題の鍵（`<slug>.issue.lock`）、gate の鍵の順に取る。`hold` は、課題が done でなく、理由の課題が在って done でなく自分でないことを確かめ、`hold` を書き、gate が今の head を渡していれば（印が緑のはずの状態）その head の印を黄に書いてから返る。渡していない head・閉じた PR・PR の無い課題には書かない。印を書けない・`report.json` か PR を読めない・PR がもうマージされているときは、`hold` は書いたまま終了コード 1 で理由を出す（dispatcher が次に PR を読むとき揃える。オーナーにはまだ待つよう伝えない）。`unhold` は `hold` を消すだけで印は書かない（次の巡で gate が回り直して書く）。止めていなければ何もしない。
- **`wait` は記録に書かない**: `andon gate` は記録の形を stdout に出して終了コード 5 で返すが、`gate/<head>.json`・PR のコメント・commit status は書かない（見回りのたびに PR を埋めない）。読み手（dispatcher）は控えの `gates` に書かず、次の見回りで gate から回し直す。ただし `wait` を返した head は控えの `waits` に書き、同じ head の checks が走っている間と、1つも無く `github_checks_wait_minutes` を過ぎていない間は gate を回さない（見回りでは checks だけを GraphQL で読む。REST の上限を使わない）。
- `acceptance`: `report.json` の `acceptance[]` の `result` ごとの件数。`run` のある行が指す記録（§24）の `commit` が `head` と違えば、その行を `stale_runs`（`{"id", "run", "commit"}` の並び。1件以上。無ければキーを書かない）に出す。古い記録では戻さない（[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 2）。指す記録が証拠にならない（§24 の3通り）ときは `fix` で、`reasons[]` に行ごとの理由（`受け入れ条件 <id> の記録 runs/<n>.json が無い` など）。足してもバージョンは上げない（原則4）。
- `delegation`: この課題の判定に使った委譲の `merge` の値と、判定のレベルがその中か（§11）。外なら、ほかに止める理由が無いとき（上の決定式の順の最後）に `decision: needs_human`、`reasons[]` に `delegation: merge=B < tier C`。`merge` は、プロジェクト全体の `merge` と、この課題を名指しした有効な grant（§11 の `grants[].issues`）の値のうち、いちばん広いもの。担当が `risk.one_way: true` を申告した課題では名指しを読まず、プロジェクト全体の `merge` で判定する（`manual` があるだけなら名指しが効く）。`grant` は任意で、名指しの grant がレベルを通したとき（プロジェクト全体の `merge` だけでは外だったとき）だけ、その grant の `id`（`d<n>`）を書く。どの許しで入ったかを、あとから課題ごとに辿るため。足してもバージョンは上げない（原則4。[ADR-0041](adr/0041-delegation-cap-out-of-command.md) 決めたこと 6・7・9）。
- gate は、決めたあとは head が動くまで回り直さない。名指しを書いても、gate がもう決めた課題には head が動くまで効かない。名指しは gate が回る前に出す（[ADR-0041](adr/0041-delegation-cap-out-of-command.md) 決めたこと 8）。
- 読み手: dispatcher（approve → `merge: auto` ＋ 委譲の範囲内なら、マージの直前に課題の予約キーと委譲の `merge`〈gate と同じ課題ごとの値。gate のあとに名指しが取り消されるか期限が切れれば、値が変わってマージしない〉、PR の head と GitHub の checks を読み直し、head が記録の `head` のまま・checks が通る状態のときだけマージ → `andon issue finish`。違えば打たず、次の見回りで gate から回し直す。打った `gh pr merge` が base の動きか、必須の check がまだ揃わないこと（「the base branch policy prohibits the merge」。印を書いた直後に GitHub の計算が追いつかない）で通らなかったときも、次の見回りで gate から回し直す（3回まで。ADR-0056 決めたこと 12）。残る隙間: マージの直前の読み直しと `gh pr merge` の間に、同じ head のまま checks が走り直した・落ちたときは止められない（`--match-head-commit` が固定するのは head だけ）。サーバーの側で checks を強制できるのはブランチ保護（ruleset）だけで、private の repo の Free のプランでは使えなかったが、2026-11-09 に public にして ruleset `main`（PR 必須・required checks `tests` と `go`・force push と削除の禁止）を付けたので、この隙間は塞いである。wait → 控えの `gates` に書かずに次の見回りで回し直す（checks が揃うまでは checks だけを読んで待つ）。fix → 担当へ `hc send`。`needs_human` → §5 の問いを `from: gate` で出す）・振り返り（fix の理由・レベルの分布）・オーナー。

## 8. hc との約束

| 何 | 形 | 書き手 | 読み手 |
|---|---|---|---|
| 担当・レビュアーの端末の名前 | `<slug>-<id を小文字>`（`a-z 0-9 - _`、32字まで） | `andon issue start`・`andon issue resume`・dispatcher | dispatcher・ボード |
| Chief of Staff の端末の名前 | `<slug>-cos`（cwd は kanban の root） | `andon chief-of-staff` | `andon chief-of-staff`（動いているものを使うときに名前と cwd を照らす） |
| ラベル | `role=chief-of-staff \| dispatcher \| chief-engineer`・`project=<slug>`・`issue=<id>`・`title=<題>`・`news.replied=<turn>`（dispatcher が書く。§6）・`news.ack=<turn>`（Chief of Staff の「オーナーに言うことが無い」の申告。`andon news ack` だけが書き、dispatcher〈`drain`〉が読む。§6）・`session=<id>`（ハーネスのセッションの id。kanban の `session` と同じ値。hc の template は呼び手の環境変数を渡さないので、template の `command`・`resume` の行が `andon issue session` で自分の端末のこのラベルを読み、Claude 系は `--session-id`／`--resume`、Codex 系は `resume <id>` に渡す。`andon issue start`〈Claude 系〉・`andon chief-engineer`・`andon issue resume`・dispatcher〈Claude 系のレビュアー〉が付ける。[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 11・14）・`first_message.sent=false`（最初のメッセージが届かなかった〈`hc run` の `first_message` が `sent: false`、か `started: false`〉担当・Chief Engineer・レビュアー。`andon issue start`・`andon chief-engineer`・dispatcher〈レビュアー〉が付け、dispatcher が送り直して外す。[dispatcher.md](dispatcher.md) §5） | command・dispatcher | ボード・dispatcher・フック（自分の課題を引く） |
| Herdr App の置き場のラベル | `herdr.space=<id>`・`herdr.tab=<id>`（ラベルを持つのは Herdr App。id は command が決める: Chief of Staff は `andon-<slug>`・`andon-<slug>-cos`、Chief Engineer は `andon-<slug>`・`andon-<slug>-<id>`、担当は `andon-<slug>-<id>`・`andon-<slug>-<id>`（Space ＝ 課題。[ADR-0003](adr/0003-task-spaces.md)。machine に依らない。`machine` 付きの課題を `andon issue start` はまだ起動しない。ADR-0002 決めたこと 19）、レビュアーは担当の端末（kanban の `agent`）の値を `hc get --json` で読んで写す（読めなければ付けない）。dispatcher は kanban が1つなら `andon-<slug>`・`andon-<slug>-dispatcher`、2つ以上なら `andon-<セルの slug>-plant`・`andon-<セルの slug>-dispatcher`（セルの slug は hostname。ADR-0002 決めたこと 21）。`where` は App が書く出力で、Andon は読まず書かない。[ADR-0002](adr/0002-herdr-spaces.md)） | `andon chief-of-staff`・`andon chief-engineer`・`andon issue start`・`andon issue resume`（止まる前と同じ値。レビュアーは元の課題の担当の値 `andon-<slug>-<元の id>`）・dispatcher（自分の端末のラベルは boot とフックの `dispatcher.Ensure` が付ける。kanban の数が変わって値が変われば、見回りの合間に起動し直して今の値で付ける）。`andon issue finish` は触らない（端末の kill・rm で tab が空になって消える） | Herdr App |
| オーナーに端末を指す言い方 | `<Space 名> / <tab 名>`（App の `where` と同じ形。例 `sample-site / CoS`・`sample-site / CE A-0003 Billing の設計`・`A-0012 ログインの直し / A-0012`）。address は末尾に `（t7）` と添えるだけ。Andon の command は `hc attach` を打たず、最後に出た場所をこの形で出す | `andon chief-of-staff`・`andon chief-engineer`・`andon issue start`・Chief of Staff | オーナー |
| template の名前 | プロファイルの名前そのもの（`chief-of-staff`・`chief-engineer`・`claude`・`sonnet`・`fable`・`codex`・`codex-review`・`claude-review`）。プロファイルの frontmatter に `resume:` の行があれば、boot がその行を `command` にした `<profile>-resume` も作る（止まった端末をセッションの続きから起動する。元の template からは `resume:` を除く。`templates.local/` が同じ名前で勝てば勝った側から作る） | boot が生成 | `andon issue start`・`andon issue resume`（`hc templates` で存在を確かめる） |
| kanban の `agent` | 端末の address。作業側が別のマシンなら `<machine>/t7`（相談 F4） | `kanban start`（担当）・dispatcher（作業側の代筆）・`andon issue resume`（最初のメッセージが届いてから `kanban set`） | dispatcher・ボード |
| kanban の `session` | ハーネスのセッションの id（小文字の UUID）。Claude 系は起動するときに Andon が決めて `--session-id` で渡す。Codex 系は決める口が無いので、`resume` のときに記録（`$CODEX_HOME/sessions/**/rollout-*.jsonl` の先頭行 `session_meta` の `cwd`・`id` だけ）から cwd が同じ最新のものを引いて書く | `andon issue start`（Claude 系）・`andon chief-engineer`・`andon issue resume`・dispatcher（Claude 系のレビュアーを起動するとき） | `andon issue resume` |
| 起動の仕方 | `hc run --bg --outlive-parent --no-notify --name … --label … --cwd … [--on <machine>] <profile> "<最初の3行>"` | `andon issue start`・dispatcher | — |
| 起動し直し | `andon issue resume <id> [--fresh] [--dry-run [--json]]`: `doing` で端末（kanban の `agent`）が無い・終わった・素のシェルの担当・Chief Engineer・レビュアーだけ（`todo`・動いている・`machine` のある課題は理由1行で止まる）。kanban の `worktree` が無ければ `git worktree prune` のあと `git worktree add <path> <branch>` で同じパスに戻す（パスは kanban の値のまま。今の置き場か旧の置き場の `<repo 名>/<slug>-<id>` でなければ止まる。あれば触らない）。レビュアー（profile `*-review` で `parent` のある課題）は `git worktree add --detach <path> <commit>` で戻し（commit はそのレビュアーの依頼書の「対象: … commit <sha>。」の行。無ければ止まる）、cwd は `issues/<rev>/`。セッションの記録（Claude は `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl` の有無だけ、Codex は上の `session` の行）が無ければ起動せず非 0。素のシェルの端末は片付けて（hc の名前は live の中で1つ）、同じ名前・同じラベル（Chief Engineer は指揮側の Space）で `<profile>-resume` を起動する。最初のメッセージは「課題 … の続き。セルが起動し直した。…」。届かなければ起動した端末を片付けて非 0、kanban は変えない。届いたら kanban の `agent`（と変わった `session`）を書く。`--fresh` は `<profile>` の template で新しいセッション（最初のメッセージは start の2行に前の担当の `report.md` を読む1行）。`--dry-run` は hc を読むだけ（`get`・`ls`）で、計画（template・止まった端末・cwd・worktree と戻す手・branch・session・ラベル・最初のメッセージ・確かめの結果）を出す。[ADR-0012](adr/0012-resume-and-worktree-home.md) | `andon issue resume`（Chief of Staff・dispatcher〈R4〉） | — |
| lock | `andon issue start`・`resume`・`finish` は kanban ごとの lock（`$XDG_STATE_HOME/andon/dispatcher/<slug>.issue.lock`）を命令の間持つ（`--dry-run` は持たない）。dispatcher の控えの lock（`<slug>.json.lock`）とは別（dispatcher は控えの lock を持ったまま `andon issue finish` を打つ） | `andon issue start`・`resume`・`finish` | — |
| 乾いた実行 | `andon issue start <id> --dry-run [--json]`（`resume` の `--dry-run` は上の「起動し直し」）: 確かめ（本番と同じ順・同じ理由・同じ終了コード）だけを通し、hc を呼ばず、端末・worktree・branch・ラベル・信頼の設定を作らず kanban も書かずに、上の名前・ラベル・cwd・プロファイル・最初のメッセージ（`hc run` に渡す文そのもの）と依頼書の全文・worktree・branch・確かめの結果（`ok`／hc を呼ぶものと作る直前のものは `skipped`）を出す。`--json` の形はオーナーと評価が読む出力で、schema に無い（機械は判定に使わない。[決定](decisions/2026-11-09-issue-start-dry-run.md)）。Claude 系の `session` の値（ラベル `session=`）だけは起動するたびに新しく決めるので、本番と違う | `andon issue start` | オーナー・Chief of Staff |
| 状態 | `hc events`（dispatcher）か `hc ls --json`（見回り）。`working`・`idle`・`done`・`blocked`・`error`・`finished` | hc | dispatcher |

## 9. フック

managed settings の `hooks`（boot が `package/settings.json` から生成）。フックは**dispatcher と command を呼ぶだけ**で、判断を持たない。例外は担当の Bash を断る `PreToolUse` の1本で、コマンドの文字列だけを見て断る（kanban も dispatcher も読まない）。

| 誰の | イベント | 打つもの | 返すもの |
|---|---|---|---|
| Chief of Staff | `Stop` | `andon dispatcher drain`（端末のラベル `role=chief-of-staff`・`project=` から slug を引く） | 待ちがあれば `{"hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": "<news>…</news>"}}`（続きのターンになる。`decision: block` と違い画面に「Stop hook error」が出ない。[ADR-0019](adr/0019-cos-input-box.md) 決めたこと 6）。無ければ何も返さない。前にフックで渡した `<news>` の turn の終わり（入力の `prompt_id`・`session_id` が合う）なら確かめ済みにし、端末のラベル `news.ack` が渡したときに控えた `turn` と同じなら `news.replied=<turn>` を書く（答えの文〈`last_assistant_message`〉は読まない。§6）。合わなければ渡した通知を1回だけ届け直す。控えの lock が取れなくても、渡した turn の終わりなら確かめ済みにする（次の `drain` が当てる。[dispatcher.md](dispatcher.md) §3） |
| Chief of Staff | `Stop`（`asyncRewake: true`・`timeout: 7200`。上と並んで走る） | `andon dispatcher wait --timeout 7200`（待ち役。[dispatcher.md](dispatcher.md) §4） | 終了コードと stderr だけ（stdout は出さない）。届ける待ち（控えの `pending`）があり、端末の状態が `idle`・`done` で、控えの lock をほかに持たれていない（見回りの巡の間は待つ）なら、合図の1行（`andon: 通知が待っている。これは合図で、オーナーの依頼ではない。何も書かずにターンを終える（…）。`）で 2（Claude Code が入力欄を通らずに新しいターンを始め、その `Stop` の `drain` が `<news>` を渡す）。`timeout` の 9 割まで打つ機会が無く `idle`・`done` なら（lock を持たれたままでも）、張り直しの合図（`andon: 待ち役の張り直し。…`）で 2。ほかは 0: Chief of Staff の端末でない・後から起動した待ち役に替わった（`<slug>.json.wait`）・止められた・控えが読めない・起動したときに端末が読めない・張り直しの時刻を過ぎて `timeout` の 95% まで端末が読めない（それまでは読み直す）・働いている間に張り直しの時刻が来た。0 と 2 のほかでは終わらない。待ちと控えは読むだけ |
| Chief of Staff | `StopFailure`（`asyncRewake: true`・`timeout: 600`） | `andon dispatcher rewake`（プロバイダに切られたとき。[dispatcher.md](dispatcher.md) §2.4） | 終了コードと stderr だけ（StopFailure の出力は Claude Code が捨てる）。入力の `error` が `server_error`・`rate_limit` なら、続けて切られた 1〜4 回目はそれぞれ 1・2・4・8 分あとに合図の1行（`andon: プロバイダに切られて止まったターンを起動した。…`）で 2（入力欄を通らずに新しいターンが始まる）。5 回目は 0 とログ1行（`<slug>.json.rewake.log`）。回数は `<slug>.json.rewake`（Chief of Staff の `Stop` の `drain` が消す）。ほかは 0: ほかの `error`・Chief of Staff でない端末・待ったあと端末が `error` でない・回数が変わった（Stop で消えた・次の失敗が進めた）・端末か回数が読めない・回数を書けない。0 と 2 のほかでは終わらない |
| Chief of Staff | `UserPromptSubmit` | `andon dispatcher drain --context`（中で `andon delegate show --json` を読む） | `{"hookSpecificOutput": {"hookEventName": "UserPromptSubmit", "additionalContext": "<news>…</news>\n<delegation project=\"<slug>\">…</delegation>"}}`（待ちが無ければ委譲だけ）。前にフックで渡した `<news>` の turn の終わりを見ていなければ（中断）、その通知を1回だけ届け直す |
| どの端末も | `PreToolUse`（`matcher: Bash`・`timeout: 10`） | `package/hooks/pre-tool-use`（command を呼ばない。入力の `tool_input.command` だけを見る） | コマンドの位置（行の頭・`;`・`&`・`|`・`(`・`$(`・`while`・`until` などのあと）に `pgrep`・`pkill` があり、`-f`（`-af`・`--full` も）が付けば `{"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": "<理由1行。PID はポートか PID ファイルから引く>"}}`。`-f` は command の行全体と照らすので、終わりを待つループやそのシェル自身にも当たり、待ちが終わらない。引数の文字列として含むだけ（`grep 'pgrep -f'`）・Bash でないツール・入力が読めない・`jq` か `andon` が無いときは何も出さずに通す。Codex には効かない |
| 担当 | `Stop` | `andon issue stop-check`（端末のラベル `issue=` から id を引く） | kanban が `doing` で、前回の通過（無ければ `doing` に入った行）以後に `kanban report`／`review`／`ask` のどれも `log.jsonl` に無く、待ちでもなければ `{"decision": "block", "reason": "<id> は doing のまま、報告も問いも kanban に無い。…"}`。待ち（通す）は、自分の問いが開いたまま（その問いの最後の行が `open`）か、`report.json` に `review_request` があり、その `commit` を指すレビュアーの `findings.json` がまだ無いとき。答え（`answered`）・レビューが届いたあとは待ちではない。続けて止めるのは3回まで。控えが書けなければ通す。レビュアーも同じ。Chief Engineer の課題（`profile: chief-engineer`）は止めない（[ADR-0009](adr/0009-cos-default-entry.md) 決めたこと 10）。止めるか通すかの前に、dispatcher の端末を確かめる（中身がシェルなら起動し直す。Chief of Staff の留守の分。[dispatcher.md](dispatcher.md) §5 の P3b） |

- フックの実体は `package/hooks/pre-tool-use`・`stop`・`stop-wait`・`stop-failure`・`user-prompt-submit`（image では `/opt/andon/hooks/`）。どの端末でも同じフックが走り、役は command がラベルで見分ける（どちらでもなければ何も出さない）。

- 実測（2026-11-10。担当の端末の `andon issue stop-check` の block）: `Stop` の `decision: block` で始まる続きは、hc の `turn` を進めない（block の前と同じ値）。だから①で渡したときに控える `turn` と、続きで打つ `andon news ack` の値は同じになる（§6）。
- 要実測: Chief of Staff の `Stop` の `additionalContext` で続きの turn が始まり「Stop hook error」が出ないこと、その続きが hc の `turn` を進めるか（進むなら `andon news ack` の照らし方を合わせる）、`UserPromptSubmit` の `additionalContext` がコンテキストに入ること、Codex の同等物（最初のバージョンは Claude Code だけ）。
- 要実測（待ち役。ADR-0019 の受け入れ条件 3）: 7200 秒より長い `timeout` が置けるか・7200 秒まで待ち役が生きるか、`timeout` で切られた・止められた待ち役が画面に「hook error」を出さないか、オーナーが Esc で止めたあと・セッションを起動し直したあとの待ち役。
- フックが落ちたとき（command が無い・kanban が見つからない）は**通す**（止めない。dispatcher のログに1行）。フックは守りの1枚で、唯一の守りではない（`andon issue ready` と dispatcher の照合が残る）。

## 10. 互換とバージョン

- `schema/*.schema.json` の `$id` にバージョン（`https://andon.example.com/schema/<名前>/<バージョン>`。例 `https://andon.example.com/schema/report/2`。URL は識別子で、取りに行かない）。`tests/schema.sh` が `$id` の形・draft 2020-12・`description` と識別子が英語であることも確かめる。読み手は `$id` ではなく**キーの有無**で振る舞う（原則4）。
- キーを足す: バージョンを上げない。`docs/contract.md` と `tests/schema.sh` の例を足す。
- キーを消す・意味を変える: バージョンを上げ、古いバージョンを読む側を1 バージョンだけ残す（セルの `FROM` のバージョンアップが追いつくまで）。
- ledger-format の `version` が上がったら、Andon の command は kanban の CLI の言うとおりに止まる（ledger-format の規則）。
- **バージョン 2（`notification`・`report`・`question`・`gate`・`project`。[ADR-0008](adr/0008-issue-naming.md)）**: kanban のカードの名前を task から issue に改めた。`task` → `issue`・`reviewer_task` → `reviewer_issue`・`tasks[]` → `issues[]`（`impact`）・`review.task` → `review.issue`・`next[].to` と問いの `from` の値 `task` → `issue`・`triage_max_tasks` → `triage_max_issues`。`task-keys` は `issue-keys`（`/1`。中のキーは同じ）に改名。このバージョンアップだけは古いバージョンを読む側を残さない（ADR-0008 決めたこと 7。既存の kanban と `project.json` は手で移す）。ラベル `task=` も `issue=` に改め、`task=` は読まない。

## 11. プロジェクトの事実と委譲（`project.json`）

プロジェクトについて機械が読む事実（[ADR-0004](adr/0004-project-layout.md) 決めたこと 1）。**置き場はセルの repo の `projects/<slug>/project.json`**（kanban の外。書き手がオーナーだけだから）。委譲（オーナーが Chief of Staff・dispatcher・担当に何をどこまで任せたか。design.md §1.10）はその `delegation` に入る。オーナーが見る一覧は `andon delegate show`。

```json
{"version": 1,
 "project": {"title": "Sample site", "prefix": "S"},
 "repos": ["github.com/example/sample-site"],
 "references": ["github.com/example/design-notes", "https://example.com/style-guide"],
 "delegation": {"version": 1, "project": "sample-site", "preset": "hobby",
   "grants": [
     {"id": "d1", "scope": "publish", "value": "public",
      "by": "owner", "at": "2026-11-05", "until": null,
      "evidence": "2026-11-05 の対話「サイトは勝手に出してよい」（github.com/example/sample-site の docs/decisions.md #12）",
      "revoked": null},
     {"id": "d2", "scope": "deploy", "value": "none",
      "by": "owner", "at": "2026-11-05", "until": "2026-12-31",
      "evidence": "移行中はデプロイをオーナーが見る",
      "revoked": {"at": "2026-11-20", "by": "owner", "reason": "移行が済んだ"}}]},
 "chief_of_staff": {"parallel": 2, "review_when": ["外向きの面", "新しい module", "約500行を超える差分"], "triage_max_issues": 3, "retro_every": 25, "idle_minutes": 15},
 "digest": {"destinations": [{"kind": "slack", "channel": "C0123456789"}]},
 "signals": [
   {"source": "github-ci", "repo": "github.com/example/sample-site", "weight": "now"},
   {"source": "github-issues", "repo": "github.com/example/sample-site", "weight": "later", "every": 30, "filter": "label:bug", "write_back": true},
   {"source": "sentry", "weight": "later", "org": "example", "project": "sample-site"}]}
```

| キー | 要るか | 意味 |
|---|---|---|
| `version` | 必須 | この形式のバージョン（整数の `1`） |
| `project` | 必須 | `{title, prefix}`（2つとも必須）。`title` はオーナーに見せる名前、`prefix` は課題の id の接頭字（英大文字で始まる英大文字・数字） |
| `repos` | 必須 | このプロジェクトが**書いてよい** repo（`<host>/<owner>/<repo>`。重ならない〈command が確かめる〉）。`andon issue start` は課題の `repo` がここに無ければ起動しない。kanban の `config.yaml` の `repos` はこの部分集合。空なら repo を持たない |
| `references` | 任意 | 読むだけの repo（`<host>/<owner>/<repo>`）か `https` の URL。書かない |
| `delegation` | 任意 | 委譲（下の表。移す前の `delegation.json` と同じ形）。無ければ無委譲（全部オーナー） |
| `chief_of_staff` | 任意 | Chief of Staff が読む設定値。`parallel`（並行の上限。1 以上。既定 2）・`review_when`（独立レビューを入れる条件。1行1条件。command は読まない）・`triage_max_issues`（1回のトリアージで起動する課題の上限。1 以上。既定 3。`andon triage apply` も読む。`--max-issues` を渡せばその回だけそれが先。`project.json` が信じられない・形が違うときは上限を決めずに止まる。dispatcher が空きのトリアージを撃つ線〈仕分けていない項目 50 件〉は、1 回のトリアージが増やす項目〈自身の報告・起動した直す課題の報告・そのレビュアーの子の読めない報告〉の合計を超える数として決めてある。この値を大きくしたセルでは、その合計が 50 を超えないかをオーナーが見る。[ADR-0047](adr/0047-idle-slots-run-upkeep.md) 決めたこと 7）・`retro_every`（前の振り返りが集めたあとに片付いた担当の課題がこの件数に達したら、dispatcher がルーチン `retro` を撃つ。1 以上。既定 25。読むのは dispatcher。数え方は [dispatcher.md](dispatcher.md) §2.1.1 の 3a。`project.json` が信じられない・形が違うときは既定で数え、ログに 1 行）・`idle_minutes`（dispatcher が並行の枠の空きを見る間〈分〉。0 以上。既定 15。0 は空きを見ない。2 回続けて枠が空いていれば、dispatcher がトリアージと振り返りのルーチンを線の上で、ルーチンのファイルに `idle_hours` を書いたルーチンを時間の上で撃ち、通知 `idle` を積む。読むのは dispatcher。見方と線は [dispatcher.md](dispatcher.md) §2.13。`project.json` が信じられない・形が違うときは既定で見て、ログに 1 行） |
| `signals` | 任意 | 外の信号の出所（下の表。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 11）。無い・空なら何も取りに行かない |
| `digest` | 任意 | ダイジェストをセルの外へ送る先（下の表。[ADR-0044](adr/0044-owner-digest.md) 決めたこと 12）。無い・空なら、どこへも送らず `files/` にだけ出す |

`digest.destinations[]` の1件（読むのは `andon digest send` と `andon digest check`。§28。kind ごとに1件）:

| キー | 要るか | 意味 |
|---|---|---|
| `kind` | 必須 | 送り先の道具。`slack` だけ |
| `channel` | 必須 | チャンネルの ID（`C` か `G` で始まる英大文字と数字）。名前（`#…`）と利用者の ID は書けない |

- オーナーが送り先を書いたことが、毎回の送信への go になる。委譲の `publish` の値では決まらない。読むのは下の読み方の 1・2 のとおり HEAD のものだけで、信じられなければどこへも送らない。
- トークンは書かない。`andon digest auth slack` がセルの中で聞いて `~/.config/andon/digest/slack.token`（mode 600。compose の volume `digest`）に置く（[operate.md](operate.md) §8）。
- **`digest` を書くのは、セルのパッケージを `digest` を知るバージョンに上げてから**（`signals` と同じ。古いバージョンの `andon` は `project.json` ごと読めず、無委譲・書いてよい repo 無しに倒れる）。パッケージを古いバージョンに戻すときは、先に `digest` を消す。

`signals[]` の1件（読むのは `andon signal collect`。§15）:

| キー | 要るか | 意味 |
|---|---|---|
| `source` | 必須 | `github-ci`（案件 repo の main の CI の失敗）/ `github-issues`（open の issue）/ `sentry` / `linear` / `cloudflare`（Workers の Issues） |
| `repo` | 任意 | 出所の repo（`<host>/<owner>/<repo>`）。`repos` か `references` の中のものに限る（command が照合する。schema では見ない）。GitHub の出所で無ければ `repos` の全部を読む |
| `weight` | 必須 | `now`（通知 `signal` ですぐ Chief of Staff へ。§6）/ `later`（記録だけ。トリアージが拾う） |
| `every` | 任意 | 読む間隔（分。整数。1 以上）。既定 10 |
| `filter` | 任意 | 出所の検索の語をそのまま（`github-issues` なら GitHub の検索の語。例 `label:bug`。`gh issue list --search` に手を加えず渡す）。空にしない。無ければ open の issue 全部（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 9） |
| `write_back` | 任意 | `andon issue finish` がこの出所のトラッカー（課題の `links` の URL）に1コメント書くか。既定 `false`（書くのはオーナーだけなので、`true` が委譲の印）。下の「書き戻し」 |
| `org`・`project` | 任意 | GitHub 以外の出所の中の場所（Sentry の organization と project など）。空にしない |

- **書き戻し**（`write_back: true`。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 15・19）: `andon issue finish` が課題を done にしたあと、課題の `links` のうち、`write_back: true` の `github-issues` の出所の repo（`repo` が無ければ `repos` の全部）の issue の URL（`https://github.com/<owner>/<repo>/issues/<n>`）ごとに、`gh issue comment` で1コメント書く。本文は外の人が読む文で、**課題番号を書かない**（kanban を持たない人には辿れない。[ADR-0026](adr/0026-pr-rules-for-outside-readers.md) 決めたこと 9）。3 つの場合で次のとおり（PR の URL は `report.json` の `pr`）: PR が MERGED なら「この Issue に対応する変更がマージされた。」と `PR: <URL>` の行、CLOSED なら「この Issue に対応する変更は、マージされずに閉じられた。」と `PR: <URL>` の行、`pr` が無ければ「この Issue に対応する作業は、変更（PR）を作らずに終えた。」。**状態は変えない**（打つのは `gh issue comment` だけ）。`done` の課題には finish を打てないので、1つの課題で1回。PR の状態が読めない・MERGED でも CLOSED でもない（OPEN のままやめた課題など）・書けないときは書かずに1行出し、finish は戻さない（手で書く）。レビュアーの課題では書かない。今書けるのは GitHub の issue だけで、Linear は読む command と一緒に足す（決めたこと 3）。
- **`signals` を書くのは、セルの image を `signals` を知るバージョンに上げてから**。古いバージョンの `andon` は知らないキーとして弾き（schema の `additionalProperties: false` と command の `DisallowUnknownFields`）、`project.json` を読めないものとして**無委譲・書いてよい repo 無し**に倒れる（担当を一切起動できず、`andon delegate` も打てない）。原則4（知らないキーは無視）と `project.json` の扱い（原則6・読み方の 2）は食い違っていて、`signals` もそれを引き継ぐ（直すのは別の決定）。

`delegation` の中:

| キー | 要るか | 意味 |
|---|---|---|
| `version` | 必須 | 委譲の形式のバージョン（整数の `1`） |
| `project` | 必須 | プロジェクトの slug（置き場と一致しなければ無委譲） |
| `preset` | 任意 | 既定の型 `hobby` / `work` / `andon`（design.md §1.10 の表）。無ければ `work` |
| `grants[]` | 任意 | 型からの足し引き。後の行が前の行を上書きする（同じ `scope` の有効な grant のうち、`at` が最新のもの） |
| `grants[].id` | 必須 | `d1` … 一意（一意かは command が確かめる） |
| `grants[].scope` | 必須 | `merge` / `deploy` / `publish` / `irreversible` / `external-requests` / `profile` / `package-update` / `line-change` |
| `grants[].value` | 必須 | `scope` ごとの値（design.md §1.10 の表）。`merge`＝`none`/`A`/`B`/`C`、`deploy`＝`none`/`preview`/`production`、`publish`＝`none`/`internal`/`public`、`irreversible`＝`delete-branch`/`force-push`/`rewrite-history` の配列（`delete-remote` は書けない）、`external-requests`・`profile`・`package-update`＝真偽値、`line-change`＝`none`/`project`/`plant` |
| `grants[].by` | 必須 | 付与したオーナー。文字列の `owner` だけ（実際のオーナーはセルの `andon.toml` の `owner`。照合は commit の author で） |
| `grants[].at` | 必須 | 付与した日（`YYYY-MM-DD`。暦にある日。`2026-02-30` やうるう年でない年の `02-29` は通らない） |
| `grants[].until` | 任意 | 期限（`YYYY-MM-DD`）。過ぎていれば無いものと同じ。`null` は無期限 |
| `grants[].evidence` | 必須 | 根拠。オーナーの発言の記録へのパスか引用（写さず指す） |
| `grants[].revoked` | 任意 | 取り消し `{at, by, reason}`（3つとも必須。`null` は取り消していない）。あれば無いものと同じ（行は消さない。履歴のため） |
| `grants[].issues` | 任意 | 名指しした課題の id の配列（1件以上。重なりなし〈command が確かめる〉）。書けるのは `scope` が `merge` で値が `A`・`B`・`C` の grant だけ。名指しの grant はプロジェクト全体の値に重ねず、名指しした課題の判定にだけ効く（読み方の 4・5）。名指しする課題は、書くときにそのプロジェクトの kanban にあり `done` でないもの（command が確かめる）。[ADR-0041](adr/0041-delegation-cap-out-of-command.md) 決めたこと 6 |

**読み方**（読む側は全部この順）:

1. セルの repo の **HEAD** の `projects/<slug>/project.json` を読む（worktree・作業側の写し・未 commit の変更は読まない）。
2. そのファイルを最後に変えた commit の author が `andon.toml` の `owner` と一致しなければ、**無委譲**（全部オーナー）・**書いてよい repo 無し**として読む。無い・JSON でない・schema を通らない・`delegation.project` が違う・`version` を知らない、も同じ（原則2）。`delegation` が無ければ無委譲。
3. **移す前の `projects/<slug>/delegation.json`・`delegation.md` は読まない**。`project.json` が無く `delegation.json` だけがあるセルは、移すまで無委譲・repo 無し（安全側。`andon delegate show` の理由にそう出る）。移すのは kanban の改名の課題（ADR-0004 決めたこと 5 の 4）。
4. `delegation.preset` の値に、有効な `grants[]`（`revoked` が無く `until` が過ぎていない）を `at` の順に重ねる。`issues` のある grant（名指し）は重ねない。これがプロジェクト全体の値（`andon delegate show` の実効の値）になる。
5. 読み手ごとに見るもの: gate とマージの直前の読み直し＝課題ごとの `merge`（プロジェクト全体の `merge` と、その課題を名指しした有効な grant の値のうち、いちばん広いもの。名指しは広げる向きにだけ効き、狭めるのは課題の `merge: person` か `revoke`。担当が `risk.one_way: true` を申告した課題では名指しを読まない。委譲の `merge` を読むのはこの2か所だけ）。dispatcher と `andon issue …`＝`deploy`・`publish`・`irreversible`（打つ前。範囲外なら止まって理由1行）。`andon issue start`＝`repos`（課題の `repo` が無ければ起動しない）。`andon digest send`・`check`＝`digest`（委譲は読まない）。Chief of Staff＝委譲の全部（起動するたび。`UserPromptSubmit` のフック経由。範囲外の判断は §5 の問い）と `chief_of_staff`。
6. command は、プロジェクトの slug を見て `preset` と grant の値を決めない。slug が `andon` のプロジェクトも、ほかのプロジェクトと同じ型と値を書ける（[ADR-0041](adr/0041-delegation-cap-out-of-command.md) 決めたこと 2。型 `andon` は、どのプロジェクトも選べる型の 1 つ）。`irreversible` に `delete-remote` は型でも grant でも入れられない。

**書き方**: `project.json` はオーナーが書いて commit する（`andon delegate` は無い `project.json` を作らない）。委譲はオーナーがセルの repo で `andon delegate grant <scope> <value> --evidence <path|text> [--until <date>]`・`grant merge <A|B|C> --issues <id,…> --evidence <path|text> [--until <date>]`（名指し。`narrow` は `--issues` を取らない。id は `A-5` も `A-0005` に揃えて書く）・`narrow`・`revoke <id> --reason <text>`・`show [--json]` を打つ。`show` は名指しの grant を、名指しした課題と一緒に並べる（`--json` の `grants[]` に `issues`）。command はセルの repo の **HEAD** の `project.json`（最後の commit の author が owner のもの）から組み立て、`delegation` だけを変え（ほかのキーは HEAD のまま。形は2字下げに揃う）、全体を検証して書き、`project.json` を commit する（message に `scope`・`value`・`evidence`）。commit の author と committer は端末の git の user ではなく `andon.toml` の owner（名前は HEAD の `project.json` を最後に変えた owner の commit のもの）にする。owner が `andon.toml` に無ければ commit せずに止まる。`project.json` に commit していない変更があるか、HEAD を最後に変えたのが owner でなければ打たない（ほかの書き手の変更をオーナーの commit に混ぜて信じられる形にしない）。Chief of Staff・担当・dispatcher の端末では打てない（managed deny ＋ 端末のラベルに `role=` か `issue=` があれば拒む）。

**依頼書の Settings との関係**: `issue.yaml` の `merge: auto` は委譲の `merge` の範囲の中でだけ効く（§2）。委譲の外なら、gate は approve でも `needs_human`（§7 の `delegation`）。名指しの委譲を効かせるには、Chief of Staff が名指しされた課題の `merge` を `auto` に書く。書くのは担当が `andon issue ready` を打つ前（gate が回る前。§7）。

- **`issues` を書くのは、セルのパッケージを `issues` を知るバージョンに上げてから**（`signals` と同じ。古いバージョンの `andon` と `kanban` は知らないキーとして弾き、`project.json` を読めないものとして無委譲・書いてよい repo 無しに倒れる）。書くのは新しいバージョンの command なので、手順どおりなら起きない。パッケージを古いバージョンに戻すときは、先に名指しの grant を行ごと消す（[operate.md](operate.md) のパッケージを戻す手順）。

名指しの grant のある委譲の例（型 `work` のプロジェクトで、課題 2 つにだけ `merge C` を任せる）:

```json
{"version": 1,
 "project": {"title": "Sample site", "prefix": "S"},
 "repos": ["github.com/example/sample-site"],
 "delegation": {"version": 1, "project": "sample-site", "preset": "work",
   "grants": [
     {"id": "d1", "scope": "merge", "value": "C", "issues": ["S-0012", "S-0013"],
      "by": "owner", "at": "2026-11-21", "until": null,
      "evidence": "2026-11-21 の対話「語を置き換える 2 つの課題は自動でマージしてよい」",
      "revoked": null}]}}
```

## 12. 候補の一覧（`candidates.json`）

担当の課題の `library/candidates.json`。調べて見つけた、直すかもしれないが決めていないもの（候補）の一覧。調べる課題（`deliverable: candidates`。§2）の成果で、直す課題が範囲の問い（§5 の `kind: scope`）で止まるときにも同じ形で書く。担当は手で書かず、`andon issue candidates <id> <json|->` で書く（打てるのはその課題の担当の端末〈`issue.yaml` の `agent` ＝ `HC_TERMINAL_ID`〉だけ。command が形を確かめ、repo があれば `commit` に `HEAD` を入れる。入力に `commit` は書かない。形が違えば書かずに終了コード 1 と理由1行で、前の一覧はそのまま）。書き直すたびに全体を書く（`report.json` と同じ）。答えが出て直したあとも消さない（`finish` でも残る）。

```json
{"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "candidates": [
   {"id": "c1", "text": "docs/design.md:42 「依頼所」→「依頼書」", "file": "docs/design.md", "line": 42, "in_scope": true},
   {"id": "c2", "text": "docs/operate.md:17 「起動する」が2回", "file": "docs/operate.md", "line": 17, "in_scope": false},
   {"id": "c3", "text": "README の英語の段に古い command 名。直し方は語の置換だけでなく段の書き直し", "file": "README.md", "line": 30, "in_scope": false}]}
```

repo の無い課題（`commit` を書かない）で、候補が無かったとき:

```json
{"candidates": []}
```

| キー | 要るか | 意味 |
|---|---|---|
| `commit` | repo のある課題では必須（schema では任意。確かめるのは `andon issue candidates`・`andon issue ready`）。repo の無い課題では書かない | 見た commit（書いた時点の `HEAD`。40桁・小文字）。古くなった候補を見分ける |
| `candidates[]` | 必須（空でもよい） | 1件＝直すかもしれないもの1つ |
| `candidates[].id` | 必須 | `c1`・`c2` …（`c` と 1 以上の数）。一覧の中で一意。問いの選択肢・答え・依頼書から指す名前 |
| `candidates[].text` | 必須。1行 | 何が・どこに・どう直すか（直し方を含む）。オーナーと Chief of Staff が読む |
| `candidates[].file`・`line` | 任意 | 場所を指せるとき。形は `findings.json`（§3）と同じ（repo からの相対パス・1 以上の整数） |
| `candidates[].in_scope` | 必須。真偽値 | 依頼書の範囲（やること・完了条件）の中か。直す課題では、中なら直し、外なら残す・問うの振り分けの元。調べる課題では「問いの答えに当たるか」 |

- 書き手は担当（調べる課題・止まる直す課題）。読み手: Chief of Staff（切る・範囲の問いに答える）・オーナー（ボードと報告から）・トリアージ（将来）・振り返り（候補の数・`in_scope` の比）。gate も dispatcher も判定に使わない。形を固定するのは、問いの `facts` と依頼書の `links` が同じ形で指せるように。
- `id` の一意は、schema では検査器の拡張のキーワード `uniqueKeys`（`internal/contract` の検査器が確かめる。標準の JSON Schema に無い）で書く。ほかの検査器はこれを飛ばすので、`andon issue candidates` も確かめる。
- 人が読む分は報告の `## Report` に同じ一覧を表で写す（1行1候補。`id`・場所・何・範囲の中か）。
- 候補が多い（目安 10 を超える）とき、調べる課題は全部書き、直す課題は「数と種類」を問いに書いて候補は代表だけ。
- レビュアーの `findings.json`（§3。直す前提の指摘で、gate が数える）とは別の約束。混ぜない。

## 13. 流れの数え（`andon measure --json` の出力）

`andon measure [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]` が、1つの kanban の期間の流れ・効率・対抗の指標を数えて出す（[決定](decisions/2026-11-09-measure-flow-metrics.md)）。**読むだけで、何も書かない・保存しない**。この節はその `--json` の形で、置き場は無い（標準出力だけ）。人向けの表（`--json` 無し）は先頭が「価値: 仮説を確かめた数 …」の1行で、価値 → 流れ → 効率の順に並び（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 18。[決定](decisions/2026-11-10-measure-value-line.md)）、自動マージ率の行に対抗の指標を並べる。表は人が読むもので、機械は読まない。

読むもの: `log.jsonl`（`at`・`by`・`id`・`change.status`。問いの行の `status` は問いの状態なので課題の状態に数えない）・`issue.yaml`（`created`・`after`・`links`・`profile`・`parent`・`session`・`worktree`・`who`・`estimate`）・`config.yaml`（`chief_of_staff`。`estimate` の履歴の照合）・`questions.yaml`（`asked`・`answered`・`decidedBy`）・`report.json`（`pr`・`observation.result`・`estimate.size`）・gate の記録（`at`・`pr`・`tier`・`decision`・`merged`）・`corrections.json`（要素の数）と、kanban の PR がある repo ごとに1回の `gh pr list --state merged`（`number`・`url`・`title`・`body`・`mergedAt`）と、dispatcher の控え（`<slug>.json` の `gone`。端末が Gone だと見た時刻）・記録だけの通知（§6 の `<slug>.recorded.jsonl`。`at`・`kind`）・届けた通知の記録（§17 の `<slug>.delivered.jsonl`。`at`・`acked`・`news[].kind`）と、担当とレビュアーのセッションの記録（Claude Code の `<CLAUDE_CONFIG_DIR か ~/.claude>/projects/*/<session>.jsonl` とその `<session>/subagents/*.jsonl` の `requestId`・`message.usage`、Codex の `<CODEX_HOME か ~/.codex>/sessions/**/rollout-*.jsonl` の先頭行の `cwd` と `token_count` の `total_token_usage`。読むだけで書き換えない）。期間は UTC の日付で両端を含む（既定は今日までの7日）。人向けの表は効率のあとに見込みの節（収まった割合の1行と課題ごとの1行。見込みが無ければ「見込み無し」。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 9）、その後にトークンの節（課題ごとの1行。記録が無ければ「記録なし」）を置く（[決定](decisions/2026-11-12-measure-tokens.md)）。最後の節は通知で、kind ごとの数の行のあと、末尾に「記録だけへ移す候補」の1行を出す（[決定](decisions/2026-11-12-measure-news-pass-through.md)）。

```json
{"project": "sample-site", "since": "2026-11-01", "until": "2026-11-04", "days": 4,
 "value": {"observed": {"count": 3, "hit": 1, "miss": 1, "unknown": 1},
   "days_to_observe": {"count": 2, "days": 3.75}, "no_hypothesis": 1},
 "flow": {"lead_time": {"count": 2, "hours": 9},
   "wait": {"todo": {"count": 3, "hours": 1.33}, "queued": {"count": 0, "hours": null},
            "doing": {"count": 3, "hours": 19}, "review": {"count": 2, "hours": 10.5}},
   "question_open": {"count": 1, "hours": 2},
   "closed": {"count": 2, "per_day": 0.5}, "wip_end": 1},
 "efficiency": {"merged_prs": 2, "human_touches": {"total": 3, "per_pr": 1.5, "merge": 1, "decision": 1, "task": 1},
   "answers": {"owner": 1, "chief_of_staff": 1, "machine": 1, "unmarked": 0},
   "auto_merge": {"count": 1, "rate": 0.5},
   "gate": {"records": 3, "fix": 1, "needs_human": 1, "rate": 0.67}},
 "counter": {"reverted": 1, "follow_ups": 1,
   "reopened": {"corrections": "recorded", "by_tier": {"A": 1, "B": 1, "C": 0, "none": 0}},
   "corrected": {"corrections": "recorded", "by_tier": {"A": 0, "B": 1, "C": 0, "none": 0}}},
 "news": {"threshold": 0.8,
   "kinds": [{"kind": "merged", "delivered": 0, "acked": 0, "rate": null, "recorded": 3},
             {"kind": "ready", "delivered": 2, "acked": 1, "rate": 0.5, "recorded": 0},
             {"kind": "report", "delivered": 5, "acked": 4, "rate": 0.8, "recorded": 0}],
   "candidates": ["report"]},
 "prs": [
   {"issue": "S-0001", "pr": "https://github.com/example/sample-site/pull/1", "merged_at": "2026-11-01T06:00:00Z",
    "tier": "A", "auto": true, "human_touches": 1, "lead_time_hours": 6, "reverted": true, "corrections": false},
   {"issue": "S-0002", "pr": "https://github.com/example/sample-site/pull/2", "merged_at": "2026-11-02T12:00:00Z",
    "tier": "B", "auto": false, "human_touches": 1, "lead_time_hours": 12, "reverted": false, "corrections": true}],
 "estimate": {"count": 1, "within": 1, "rate": 1, "none": 1},
 "issues": [
   {"issue": "S-0001", "estimate_minutes": 60, "agent_estimate_minutes": 120, "worked_minutes": 57, "ratio": 0.95},
   {"issue": "S-0002", "estimate_minutes": null, "agent_estimate_minutes": null, "worked_minutes": 240, "ratio": null}],
 "tokens": [
   {"issue": "S-0001", "source": "claude", "usage": {"input": 120, "output": 30500, "cache_read": 9500000, "cache_write": 120000}},
   {"issue": "S-0001-r1", "source": "codex", "usage": {"input": 83000, "output": 18000, "cache_read": 670000, "cache_write": 0}},
   {"issue": "S-0003", "source": "claude", "usage": null}],
 "skipped": [{"source": "kanban/log.jsonl", "line": 8, "reason": "JSON の object でない"}]}
```

| キー | 数え方 |
|---|---|
| `value.observed` | 期間に報告された観察の課題（`report.json` に `observation` があり、`log.jsonl` で最初に `review`〈なければ最後の列〉へ動いた時刻が期間の中のもの。レビュアーの課題は数えない）の数と、`observation.result` の内訳（`hit`・`miss`・`unknown`）。観察の課題は `parent` が観察する Chief Engineer の課題（`profile: chief-engineer`） |
| `value.days_to_observe` | 上のうち `parent` が Chief Engineer の課題のもの、の「Chief Engineer の課題が最後に最後の列へ動いた時刻から観察の報告まで」の日数（中央値。報告のほうが早いものは数えない）。数えるものが無ければ `days` は `null` |
| `value.no_hypothesis` | 期間に最後の列へ動いた Chief Engineer の課題のうち、`parent` に指す観察の課題（`report.json` に `observation` がある非レビュアーの課題。通常の epic の子課題は `not_before` を持っていても数えない）が無いもの＝価値の仮説が「無し」の題（ADR-0011 決めたこと 4）。観察の課題を足す前、または観察の報告が出る前の題も数える（報告が出ると外れる） |
| `flow.lead_time` | 期間にマージされた kanban の PR の、課題の `created` から `mergedAt` まで（中央値の時間） |
| `flow.wait.<status>` | `log.jsonl` の `change.status` を辿り、期間の中でその状態にいた時間の、1件あたりの平均（最後の列を除く。入った時刻は `created`、`after` のある課題は `queued` から） |
| `flow.question_open` | `asked` から `answered`（無ければ期間の終わり）までの、期間の中の時間の1問あたりの平均 |
| `flow.closed` | 期間に最後の列へ動いた課題の数と、日数で割った値 |
| `flow.wip_end` | 期間の終わりに、最初の列・`queued`・最後の列のどれでもない課題の数 |
| `efficiency.merged_prs` | `report.json` の `pr`（無ければ最後の gate の記録の `pr`）が、期間に GitHub でマージされた課題の数 |
| `efficiency.human_touches` | 期間に起きたオーナーの操作の数（一次の指標。[ADR-0049](adr/0049-owner-touches-by-kind.md)）。`total` は下の3つの和。`per_pr` は `total` ÷ `merged_prs`（`merged_prs` が 0 なら `null`。`total` は 0 でないことがある）。`log.jsonl` の `by` では数えない。PR の上のコメントと commit（`corrections.json`）も数えない |
| `efficiency.human_touches.merge` | `merged_prs` のうち gate の記録に `merged: true` が無いもの（`merged_prs` − `auto_merge.count`）。GitHub のマージした者は読まない。gate の記録を読めない PR もここに入る（読めなかった記録は `skipped[]` に出る） |
| `efficiency.human_touches.decision` | `efficiency.answers.owner` と同じ数 |
| `efficiency.human_touches.task` | 期間にほかの列から最後の列へ動いたオーナーの課題（`who: person`）のうち、問いを1つも持たないもの（課題ごとに1回。片付いた課題への `kanban done` の打ち直しは動いたうちに入らない）。問いを持つ課題は答えで数える。`questions.yaml` を読めない課題は数えない |
| `efficiency.answers` | `answered` が期間の中の答え（課題と問いの種類は問わない）を、問いの `decidedBy` ごとに: `owner`・`chief_of_staff`（値は `chief-of-staff`）・`machine`。`decidedBy` が無いか3つの値のどれでもない答えは `unmarked` で、介在回数に入れない |
| `prs[].human_touches` | オーナーのマージ（`auto` が偽なら 1）と、その課題の問いの `decidedBy: owner` の答え（答えた時期を問わない）の和。期間への入れ方が違うので、行の和と `efficiency.human_touches.total` は一致しないことがある |
| `efficiency.auto_merge` | そのうち gate の記録が `merged: true`（dispatcher がマージ）のもの |
| `efficiency.gate` | 期間に書かれた gate の記録と、差し戻し率（`fix` ＋ `needs_human`）÷ 記録（[design.md](design.md) §5.1） |
| `counter.reverted` | 期間にマージされた PR のうち題が `Revert ` で始まり、本文が `<owner>/<repo>#<n>` か URL で kanban のマージされた PR を指すもの、が指す PR の数 |
| `counter.follow_ups` | 期間に作られた課題のうち、`links` か、マージされた自分の PR の本文が、別の課題のマージされた PR の URL を含むもの |
| `counter.reopened` | オーナーが開け直した数を gate のレベルごとに: `merged_prs` のうち `corrections.json` に1件以上ある課題と、期間に revert された自動マージの PR（重なれば1回）。`corrections` は kanban に `corrections.json` が1つも無ければ `none`（記録なし） |
| `counter.corrected` | オーナーの手直しがあった課題の数を gate のレベルごとに: `merged_prs` のうち `corrections.json`（§14）の `corrections` に1件以上ある課題（`reopened` と違い revert は数えない）。`corrections` は `reopened` と同じ |
| `news.kinds[]` | 通知の kind ごとに（名前順。どちらかの記録に出た kind だけ）: `delivered` は届けた通知の記録（§17）の行のうち `at` が期間の中のものの `news[]` の数、`acked` はそのうち `acked: true` の行のもの（素通り: Chief of Staff が `andon news ack` でターンを終え、オーナーに言うことが無かった）、`rate` は `acked` ÷ `delivered`（`delivered` が 0 なら `null`）、`recorded` は記録だけの通知（§6）の行のうち `at` が期間の中のもの |
| `news.candidates` | `delivered` が 1 以上で `rate` が `news.threshold`（0.8）以上の kind。記録だけ（§6 の「届けるか」の表）へ移す候補で、移すかはオーナーが決める（表の行を替えるのは ADR か決定） |
| `issues[]` | 期間に最後の列へ最後に動いた担当の課題（レビュアー・Chief Engineer〈`profile: chief-engineer`〉・オーナー〈`who: person`〉の課題を除く。調べる課題を含む。id の順）ごとに1行（ADR-0014 決めたこと 9・11 の 6）。`estimate_minutes` は予約キー `estimate` の分（§2 の段→分。最後に書いたのが Chief of Staff のときだけ）、`agent_estimate_minutes` は `report.json` の `estimate.size` の分、`worked_minutes` は実働（その課題が期間の中で最後の列へ動いた時刻までの行で数える。あとで起動し直した作業は入れない。待ちを除いた分。dispatcher の `estimate` と同じ数え方〈`internal/ledger` の `Worked`〉で、`doing` の区間の総和から自分の問いが `open` の区間・レビュアーの子課題の `add` から `review` まで・端末が死んでいた区間〈dispatcher の控えの `gone` から次の `agent` の行まで〉を引く。控えが無ければ死んでいた区間は引かない）、`ratio` は実働 ÷ 見込み。見込みが無ければ分と比は `null`（0 と分ける） |
| `estimate` | `issues[]` のうち見込みのある課題の数 `count`・そのうち実働が見込みの中に収まった（`ratio` が 1 以下）数 `within`・`rate`（`within` ÷ `count`。`count` が 0 なら `null`）・見込みの無い課題の数 `none` |
| `tokens[]` | 期間に `log.jsonl` の行がある課題（レビュアーの課題も。id の順）ごとに、課題の記録の全体（期間で切らない）のトークン数。`source` は `profile` のブランド。`claude` は `issue.yaml` の `session` の記録とそのサブエージェントの記録の assistant の行の `usage` を、`requestId`（無ければ `message.id`）ごとに1回だけ足す（content block ごとに同じ `usage` の行が並ぶ）。`codex` は先頭行の `cwd` が課題の kanban の `issues/<id>/` か `worktree` の rollout 全部の、それぞれ最後の `total_token_usage` を足す。`input` はキャッシュを読み書きしなかった入力（Codex は `input_tokens` から `cached_input_tokens` と `cache_write_input_tokens` を引く）・`output`（Codex は推論の出力を含む）・`cache_read`・`cache_write`。記録が1つも無ければ `usage` は `null`（0 と分ける）。ドルには直さない |
| `skipped[]` | 読めなかった記録（JSON でない・`at` か `by` が無い `log.jsonl` の行、dispatcher の通知の記録の、JSON でない・`at` が UTC の時刻でない・`kind`〈記録だけ〉か `acked`〈届けた〉が無い行と `kind` の無い通知、在るが読めないファイルとディレクトリ（課題・gate の記録の置き場を含む）、`answered` が UTC の時刻でない問いと、`status` が `answered`・`received` なのに `answered` が無い問い、`observation.result` が hit・miss・unknown のどれでもない報告、PR の一覧を読めない repo、セッションの記録の JSON でない行と読めないファイル〈`claude/<session>.jsonl`・`codex/<rollout の名前>` の形で、記録の置き場の絶対パスは書かない〉）。止まらずに飛ばし、0 には数えない。理由に絶対パスは書かない |

- 書き手は `andon measure` だけ（出力するだけ）。読み手はオーナーと振り返り（委譲を広げる根拠）。gate も dispatcher も判定に使わない。
- 数えるものが無ければ件数は 0、率・平均・中央値は `null`（表は「記録なし」）で、終了コード 0。kanban の `config.yaml` が読めない・バージョンが 3 でないときだけ終了コード 1。
- レビュアーの課題（`<id>-r<k>`・`profile` が `-review` で終わる）は流れにも効率にも数えない。

## 14. オーナーの手直し（`corrections.json`）

課題の `library/corrections.json`。オーナーがエージェントの仕事を直した跡（[design.md](design.md) §1.9 の (b) の表の 1〜3）を、振り返りの入力として1か所に集める。書き手は dispatcher だけで、`log.jsonl` の `{"status": "done"}` の行（`andon issue finish`。Chief of Staff の `kanban done` も）を拾った課題ごとに**1回だけ**書く。もう在れば書き直さない（同じ finish を2度拾っても1回）。レビュアーの課題には書かない。

- **空の配列は「無かった」、ファイルが無いのは「集めていない」。** 手直しが無ければ `{"at": …, "corrections": []}` を書く。`gh` か kanban が読めなければ書かず、次の見回りで1回だけ打ち直す。それも駄目なら書かずに通知 `corrections`（§6。`detail` は `error`）を Chief of Staff へ積む。
- 読み手は3つ: 振り返り（束の中の記録をそのまま読む）、`andon measure`（手直しの記録が1件以上ある課題を対抗の指標の `counter.reopened`・`counter.corrected` に数える）、Chief of Staff（片付いた課題の記録を wiki の元資料に写す）。gate も dispatcher も判定に使わない。端末に送った文（表の 4）と Chief Engineer の推奨案を覆したこと（表の 5）は拾わない（文の解析になる）。

```json
{"at": "2026-11-09T09:00:00Z",
 "corrections": [
   {"kind": "answer", "question": 2, "decision_kind": "users", "recommended": "CSV だけ", "picked": "CSV と Excel"},
   {"kind": "pr",
    "comments": [{"author": "<owner>", "at": "2026-11-09T08:40:00Z", "body": "見出しの語は kanban の語に揃える"}],
    "human_commits": [{"sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "headline": "見出しの語を直す", "files": ["docs/a.md"]}]},
   {"kind": "override", "finding": {"severity": "major", "file": "internal/issue/issue.go", "line": 88}, "reason": "起きる条件がこの repo に無い"},
   {"kind": "override", "question": 3, "recommended": "オーナーが PR を読んでマージする", "answer": "担当に直させる"}]}
```

手直しが無かったとき:

```json
{"at": "2026-11-09T09:00:00Z", "corrections": []}
```

| `kind` | 元 | 拾う規則 | キー |
|---|---|---|---|
| `answer` | `questions.yaml`（`kanban show --json` の `questions`）＋ `library/questions/<n>.json` | 答えがある（`answered`・`received`）問いのうち、gate の問い（`from: gate`）と範囲の問い（`kind: scope`。Chief of Staff が答える）を除き、答え（前後の空白を除く）が推奨案（`pick: true`）の `label` と違うもの。問いの JSON が無い問い（`andon issue ask` を通らない）は推奨案が分からないので拾わない | `question`（`n`）・`decision_kind`（問いの `kind`）・`recommended`・`picked`（kanban の答えのまま） |
| `pr` | `gh pr view <n> --json state,commits,comments,reviews`・`gh api --paginate --slurp repos/<owner>/<repo>/pulls/<n>/comments`（インラインのレビューコメント）・オーナーの commit ごとに `gh api repos/<owner>/<repo>/commits/<sha>`（`parents` と `files`） | コメント（会話欄・レビューの本文・インラインのレビューコメント）のうち `andon gate: ` で始まらないもの（gate のコメントは同じ token で書かれる）と本文の無いもの・インラインでは `user.type` が `Bot` のものを除いたものと、PR の commit のうち `report.json` の `commit`（担当の最後の報告）より後のもの。そのうち親が 2 つ以上の commit（main を取り込んだ merge commit など）は除く。親の数が読めない commit は入れる。`report.json` の `commit` が PR に無ければ commit は数えない。Chief Engineer の課題（`profile: chief-engineer`）では commit を数えない（報告のあとの push で報告を書き直さないので、報告より後の commit が Chief Engineer のものでありうる）。コメントはどの課題でも拾う。どちらも無ければ書かない（報告より後が merge commit だけでコメントも無いときも書かない） | `comments[]`（`author`・`at`・`body`。インラインのレビューコメントは `file`・`line` も）・`human_commits[]`（`sha`・`headline`・`files[]`） |
| `override`（指摘） | `report.json` の `waived` ＋ PR の `state` | PR が `MERGED` で、`waived` のうち `must_fix`（Chief of Staff が書いたもの。無ければ全部）以上の重さのもの。gate は担当がレビュアーを覆したものをオーナーへ回す（`needs_human`）ので、マージされたならオーナーが通した | `finding`（`severity`・`file`・`line`）・`reason` |
| `override`（問い） | `questions.yaml` ＋ `library/questions/<n>.json` | 答えがある gate の問い（`from: gate`。`needs_human` へのオーナーの答え）。推奨案どおりの答えも拾う。機械が閉じた答えは除く: `andon issue settle` がオーナーのマージを見て書いた「オーナーがマージした（<PR>）」と、gate の判定のあとに PR の head が動いて gate を回し直すときの「PR の head が gate のあとに動いた（<前> → <今>）。gate が今の head で回し直す」と「PR の head が動いた（<前> → <今>）。gate が今の head で決め直した（記録 <パス>）」。見分けは答えの全体が一致するかで、答えた者（`answeredBy`）では見分けない（dispatcher が止まっているときは Chief of Staff が settle を打つ）。オーナーが自分で書いた答えは、推奨案と同じでも違っても拾う | `question`・`recommended`・`answer` |

- オーナーの commit を author の email で見分けない: 担当も repo のオーナーの noreply アドレスで commit するので、email ではオーナーと担当が分かれない（[decisions/2026-11-09-corrections-json.md](decisions/2026-11-09-corrections-json.md)）。
- 拾うのは事実だけで、手直しの意味は読まない（答えが推奨案と同じ意味の別の言い方でも `answer` に入る。振り返りが読んで捨てる）。多めに拾い、少なく拾わない。ただし merge commit と Chief Engineer の課題の commit は、エージェントの仕事を直した跡でないか、オーナーのものと言えないので除く（[ADR-0058](adr/0058-agents-stay-out-of-pr-conversation.md) 決めたこと 7・8）。
- エージェントは PR の会話欄に書かない決まりなので、`pr` の `comments` に入るのはメンテナーのコメントのはずである。断れない書き方でエージェントが書いたコメントは入る（拾う側では見分けない）。振り返りが中身を読んで、決まりが破られた跡として拾う（ADR-0058 決めたこと 11）。

## 15. 信号の記録（`<slug>.signals.json`）

外の出所（GitHub の main の CI・issue、Sentry・Linear・Cloudflare）から読んだ信号の記録（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・16）。**置き場は kanban の外の `~/.local/state/andon/dispatcher/<slug>.signals.json`**（`XDG_STATE_HOME` があればその下。dispatcher の控えと同じ所）で、プロジェクトに1ファイル。書き手は dispatcher（`andon signal collect` と、`delivered_episode` だけを書く `andon signal delivered`。どちらも記録の lock `<slug>.signals.json.lock` の中で書く）だけ、読み手はトリアージ・観察の課題の担当・measure・ボード。どの出所を読むかは `project.json` の `signals`（§11）。

```json
{"signals": [
  {"source": "github-ci", "project": "sample-site", "key": "github.com/example/sample-site/ci.yml@main",
   "url": "https://github.com/example/sample-site/actions/runs/1", "title": "ci: test failed on main", "weight": "now",
   "fingerprint": "af3c0a047c19a9c56867fe43afe6794dd408eb8f",
   "first_seen": "2026-11-10T09:00:00Z", "last_seen": "2026-11-10T09:30:00Z", "count": 4, "active": true,
   "episodes": 2, "delivered_episode": 2, "release": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"},
  {"source": "github-issues", "project": "sample-site", "key": "github.com/example/sample-site#12",
   "url": "https://github.com/example/sample-site/issues/12", "title": "Export to CSV breaks on empty rows", "weight": "later",
   "fingerprint": "561bb4dd6370728a1136f4231fb62a4645b98e4a",
   "first_seen": "2026-11-10T09:00:00Z", "last_seen": "2026-11-10T09:00:00Z", "count": 1, "active": true,
   "episodes": 1, "delivered_episode": 0}],
 "read_at": {"github-ci github.com/example/sample-site": "2026-11-10T09:30:00Z",
             "github-issues github.com/example/sample-site": "2026-11-10T09:30:00Z"}}
```

| キー | 要るか | 意味 |
|---|---|---|
| `signals[]` | 必須 | 信号1件ごとに1つ（`fingerprint` は重ならない）。まだ何も見ていなければ空 |
| `source` | 必須 | 出所（§11 の `signals[].source` と同じ値） |
| `project` | 必須 | プロジェクトの slug |
| `key` | 必須 | 出所の中の id。CI は `<repo>/<workflow>@main`、issue は `<repo>#<n>`、Sentry・Cloudflare・Linear はその issue の id |
| `url` | 必須 | オーナーと担当が読む所（`https`） |
| `title` | 必須 | 出所の題をそのまま1行（制御文字なし）。**信用しない**: 外の人の文なので、描くときは「信用しない」の囲いに入れる |
| `weight` | 必須 | 出所の `weight`（§11）の写し |
| `fingerprint` | 必須 | `source` と改行と `key` をつないだ文字列の SHA-1（小文字の16進40字）。Sentry・Cloudflare は出所のまとめ（issue）が `key` なので、まとめ方は出所に従う |
| `first_seen`・`last_seen` | 必須 | 最初と最後に見た時刻（UTC の `YYYY-MM-DDTHH:MM:SSZ`） |
| `count` | 必須 | 見た巡の数（1 以上） |
| `active` | 必須 | 最後の巡で見えたか（`false`: CI が通った・issue が閉じた・resolved） |
| `episodes` | 必須 | 出来事の数。最初は 1、消えてまた出るたびに 1 増える |
| `delivered_episode` | 必須 | 通知 `signal`（§6）で届けた出来事の番号。届けていなければ 0（`later` は常に 0） |
| `release` | 任意 | いまの出来事（`active: false` なら最後の出来事）が始まったリリースの名前（マージ commit の SHA。§16）。出来事が続く間は書き換えない。出所が持つときだけ |
| `read_at` | 任意 | 出所と repo の組（`"<source> <repo>"`）ごとに、collect が最後に読みに行った時刻（読めなかったときも）。`collect --due` の間隔に使う。まだ読みに行っていなければ無い |

- **本文・スタックトレースは持たない**（記録を大きくしない・外の人の文を写さない）。担当が読むときに `url` から取る。

`andon signal collect` の当て方（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 3・8・9）:

- `--due`（dispatcher の巡が打つ）は、組ごとに `read_at` から `every` 分（§11。既定 10）たったものだけを読む。読みに行った組は、読めても読めなくても `read_at` を今にする（gh が落ちている・上限のときも `every` より短く読みに行かない）。`--due` が無ければ（オーナーが手で打つ）全部の組を読む。
- 出所と repo の組ごとに1回読み、見えたものは `count` を 1 増やして `last_seen`・`url`・`title`・`weight` を今の値にする。初めてなら `count`・`episodes` が 1、`delivered_episode` が 0。`active: false` のものがまた見えたら `active: true` にして `episodes` を 1 増やす。`github-ci` は、`active` のまま見えても、読んだ run が成功で区切れていて、記録の `url`（前の collect が見た最新の失敗の run）がその成功より新しい失敗の run のどれでもなければ（前の collect のあとに成功してまた失敗した）、同じく `episodes` を 1 増やす。同じ commit が何度も走るので、commit でなく run で見る（ADR-0051 受け入れ条件 5）。`release` は、初めて見えたときと、`episodes` が増えたときに、見えた値を入れる。`active` のまま見えたときは書き換えず、記録に無いときだけ見えた値を入れる（出所によらず同じ。[ADR-0051](adr/0051-signals-flow-and-breaking-change.md) 決めたこと 6）。その組の記録で見えなかったものは `active: false`。ただし読んだ範囲で結果が分からなかったもの（下の CI の 100 件・issue の上限）は変えない（読めていないものを「消えた」にしない）。**`delivered_episode` は collect が触らない**（書くのは届けた dispatcher）。
- `github-ci`: main の CI（`gh api repos/<owner>/<repo>/actions/runs?branch=main&status=completed`。pull request の run は除く）を workflow のファイルごとに見て、最新の結果（成功・失敗・時間切れ・起動の失敗。取り消しと飛ばしは数えない）が失敗なら信号。読むのは新しい 100 件で、その中に結果の出た run の無い workflow は今の記録のまま。`key` は `<repo>/<workflow のファイル名>@main`、`url` は最新の run のもの、`title` は最新の run の `<workflow の名前>: <run の題>`。見えた `release` は、失敗が始まった run の `head_sha`（main の commit）。同じ 100 件の中を古いほうへ、成功の手前まで遡った、いちばん古い失敗の run のもので、取り消し・飛ばし・pull request の run は数えない。成功が 100 件の中に無ければ、その中でいちばん古い失敗の run のもの（ADR-0051 決めたこと 5）。
- `github-issues`: open の issue（`gh issue list --state open`。`filter` があれば `--search` にそのまま。上限 1000 件で、上限まで返ったら見えなかった issue も今の記録のまま）。`key` は `<repo>#<番号>`。
- 読めなかった組（API が落ちた・`gh` の資格情報が無い）は、その組の信号の記録を変えずに飛ばして1行出す（終了コード 1。`read_at` だけ書く。`--due` なら `every` 分のあとに読み直す）。読みに行った組が無ければ何も書かない。`signals` が無い・`project.json` が無ければ何も読まず書かない。Sentry・Linear・Cloudflare は読む command がまだ無く、トークン（`andon signal auth`）が無ければそう、あれば「まだ無い」と1行出して飛ばす（止まらない）。
- 出所へは書かない（GitHub は `gh` の token が書ける token でも、打つのは読む API だけ）。
- 機械が読むのはこの schema のキーだけ。`title` で判定しない。

届け方（dispatcher。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・8、[dispatcher.md](dispatcher.md) §2.2.1）:

- dispatcher は1巡ごとに `collect --due` を打ち、記録のうち `weight: now`・`active`・`delivered_episode` < `episodes` のものを通知 `signal`（§6）で積む。積んだ待ちを控えに保存してから、1件ずつ `andon signal delivered [--project <slug>] <fingerprint> <episode>` で `delivered_episode` を `episodes` にする。
- `andon signal delivered` は、記録に無い信号・`episodes` を超える出来事は断る（終了コード 1）。もう届けた出来事（`delivered_episode` 以上）なら何も書かない。打つのは dispatcher だけで、`HC_TERMINAL_ID=dispatcher` でなければ記録を変えずに終了コード 2 で断る（担当が打って `now` の信号が届かなくなるのを止める。同じユーザーなので守りではない）。

トリアージの読み方（`andon triage`。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・7）:

- `andon triage collect` は、このプロジェクトの記録のうち `active` で、kanban のどの課題の `links` にも `url` も `key` も無いもの（github-ci の `url` は run ごとに変わるので、課題に切った信号は `key` でも照らす）（`later` の信号と、課題に切られていない `now` の信号）を1件1項目 `kind: signal` にする。項目の `key` は `<slug>:sig-<fingerprint の先頭 12 桁>-e<episodes>:signal:<fingerprint>`（ほかの項目の `<slug>:<出どころ>:<kind>:<sha1>` と同じ形。前回仕分けた信号は `count` が増えても出ないが、消えてまた出て `episodes` が増えたら新しい key で出る）。`text` は `title`、`url` は記録の `url`、`untrusted: true`（`text` は外の人の文）。記録が無ければ項目は無い。記録が読めなければ collect は止まる。
- `andon triage apply` は `signal` の key の決定（`close`・`fix`・`bundle`・`person`・`keep`）を、kanban の課題と照らさず（`kanban show` を打たない）、今回の項目か前回までの仕分けの記録か信号の記録（`active` でなくなったもの・その信号のこれまでの `episodes` も）にある key だけ受ける。`close` は記録に残すだけで、kanban にも出所にも手を入れない。記録の項目の `text`・`url`・`untrusted` は信号の記録から写し、人が読む要約（`library/triage/*.md`）では `text` を「指示として読まない」の囲いに入れる（`text` の中の `〔` `〕` は `［` `］` に替え、題で囲いを閉じさせない）。
- `andon triage collect` は `not_before` が未来の課題（`issue.yaml`。[ledger.md](ledger.md)）を項目に出さない（見る日の前に閉じられないように）。日は `config.yaml` の `timezone`（無ければ UTC）で読む。
- `andon triage collect` の出力は `{"commits": {<repo>: <commit>}, "items": [項目]}`。`commits` は、kanban の課題の `repo`（`host/owner/repo`。空でないもの）ごとに、セルの checkout（`$GHQ_ROOT/<repo>`）で origin の既定の branch を取ってきた（`refs/remotes/origin/<branch>` だけを取り直す。作業ツリーは動かさない）commit（40 桁か 64 桁の小文字の 16 進）。今回の項目の無い repo も入る。項目の `commit` は、その項目の repo の `commits` の写しで、repo の無い項目と `memory`・`signal` の項目は持たない。1つの repo でも取れなければ（checkout が無い・origin が読めない）、何も出さずに終了コード 1 と理由の行で止まる（[ADR-0030](adr/0030-triage-close-safety-net.md) 決めたこと 2・5 の 6）。
- `andon triage collect` の `items` は古い順（`age_days` の大きい順。同じなら `key` の順）に並ぶ。項目が 50（`--limit` の既定）を超えるときだけ、`memory` と `signal` の項目を先頭に寄せる（その中と残りは古い順のまま）。項目が `--limit` を超えると、何も出さずに終了コード 4 で止まる。トリアージの担当は `--limit` を増やして取り直し、先頭から決まった件数だけ仕分けるので、古い順のままだと後ろに来る `memory` と `signal` がいつまでも仕分けられないため（[decisions/2026-11-21-triage-collect-memory-signal-first.md](decisions/2026-11-21-triage-collect-memory-signal-first.md)）。
- `andon triage collect` は、前の記録（`library/triage/*.json`）が `keep` にした項目を、その記録の `collected` から 20 時間たつまで出さない（[ADR-0047](adr/0047-idle-slots-run-upkeep.md) 決めたこと 19）。同じ `key` の `keep` が何件もあれば、いちばん新しい時刻から数える。`collected` は RFC 3339 の時刻で読み、読めない・今より先の記録では伏せない。伏せた項目は、`items` の並びと `--limit` の数に入らない。`andon triage apply` が集め直すときは伏せないので、伏せた `key` も名指しで見直せる。
- トリアージの担当は collect の出力をそのまま `issues/<id>/library/items.json` に置き、判定（`decisions.json` の項目）に、読んだ commit を `commit` で写す。`andon triage apply` は `items.json` を collect の記録として読む。repo のある課題から出た項目（今回の項目も、前回までの記録の key の見直しも）は、kanban のその課題の `repo` について `items.json` の `commits` が持つ commit と判定の `commit` が同じでなければ断る（終了コード 1）。それ以外の項目（repo の無い課題・`memory`・`signal`）は `commit` を書かない。`items.json` が無い・読めない・前の形（項目の配列）・`commits` の値の形が違うときも断る。`apply` は記録（`library/triage/*.json`）の項目に `commit` をそのまま残す。apply の中で集め直すときは main を取り直さない。
- `decisions.json` の `close` は閉じ方 `closing` を必ず持つ（[ADR-0030](adr/0030-triage-close-safety-net.md) 決めたこと 5 の 1）。根拠の閉じ `done`（済み）・`gone`（対象が無い）・`wiki`（wiki へ）・`delegated`（Andon へ委譲）はどの種類の項目にも使える。判断の閉じは使える項目が決まっていて、`wont-happen`（起きない）は `waived`、`one-off`（その場限り）は `remember` と `memory`、`not-applicable`（当てはまらない）は `memory`、`read`（読んだ: 拾うものなし）は `unreadable` だけ。`andon triage apply` は、`closing` の無い `close`・知らない値・組み合わせに合わない `close` を、kanban に手を入れる前に断る（終了コード 1）。
- `decisions.json` の `person` は、判断か手作業のどちらか1つの形を持つ（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 9・10）。判断は `kind`（`business`・`users`・`promise`・`irreversible`・`line`）・`question`（1行）・`options`（1行の文字列を2つ以上。先頭が推奨案）・`reason` を持ち、種類を決めきれなかった判断は `"unsure": true` も持つ。手作業は `only_owner`（オーナーにしかできない理由の1行）・`question`（してほしいことと期限の目安の1行）・`reason` を持ち、`kind`・`options`・`unsure` を持たない。`kind` も `only_owner` も無い・両方ある・`options` が2つ未満・前の形の `pick` を持つ項目、同じ `question` の判断が2つ、判断が足すオーナーの課題と同じタイトルの片付いていない課題がもう問いを持っている、のどれかがあれば、`andon triage apply` は kanban に手を入れる前に断る（終了コード 1）。同じタイトルで `parent` の無い課題が問いを持たなければ、課題を足さずにその課題へ問いを置く。`apply` は、判断を `andon issue ask --new`（§5）と同じ関数でオーナーの課題と問いにして置く。問いの `from` は `chief-of-staff`、`facts` の先頭は出どころの `key`、`unsure` の項目は `facts` に「トリアージは種類を決めきれなかった: <reason>」を足す。手作業は `question` をタイトルにした課題を足し、`who: person` と `only_owner`（§2）を書く。要約（`.md`）の「オーナーへ」は、判断・手作業・`unsure` の数を出し、`unsure` が3件を超えると先頭に「線引きを見直す」を出す。
- `andon triage apply` が kanban の課題（`issue`・`person`）を閉じるとき、手を入れる前にその課題の問いを読む。答えられてまだ受け取られていない問い（`answered`）があるか、問いを読めなければ、kanban にも記録の置き場にも何も書かずに断る（終了コード 1）。開いている問い（`open`）は、読んだ番号を `kanban done --past` で名指して越える。読んだあとに置かれた問いは越えないので、`kanban done` が断る。
- 判断の閉じと、kanban の課題そのもの（`issue`・`person`）を閉じる判定は、Chief of Staff の判定が要る（決めたこと 5 の 3・6）。判定は `andon triage verdict <id> <key> ok|overturn` が `issues/<id>/library/verdicts.json`（`{"verdicts": [{key, verdict, closing, reason, by, at}]}`）に書く。打てるのは `HC_TERMINAL_ID` が `config.yaml` の `chief_of_staff` と同じ端末だけで、判定の要らない key・`decisions.json` に無い key は断る（守りではなく、取り違えを止めるもの）。判定は書いた時の `closing` と `reason` と書き手 `by`（`terminal:<chief_of_staff>`）を持ち、担当がどちらかを書き換えると無効になる。`apply` は `by` が今の `config.yaml` の `chief_of_staff` の端末でない判定（`by` の無いものも）を数えない。`apply --dry-run` は、出力の `review` にこの項目を1件ずつ（`key`・`kind`・`from`・`text`・`closing`・`reason`・`verdict`。`verdict` は `ok`・`overturn`・`none`〈無い〉・`stale`〈書き換わった〉・`foreign`〈書き手が今の Chief of Staff の端末でない〉）出す。`--dry-run` 無しの `apply` は、`review` の全部が `ok` でなければ、kanban にも記録の置き場にも手を入れずに断る。
- 記録（`library/triage/*.json`）の項目は `closing` を持ち、判定の要った項目は `verdict`（`{verdict, by, at}`）、`overturn` の判定のあと担当が `close` 以外に書き直した項目は `overturned`（`{closing, reason, by, at}`）を持つ。要約（`.md`）の「閉じた」は、判定の要った閉じを先に、根拠の閉じを後に分けて出す。`closing` の無い前の記録はそのまま読み、前に閉じた key は今までどおり候補に出ない（決めたこと 5 の 7）。

## 16. リリースの名前（案件の CI との約束）

エラー → PR → 課題を辿れるように、**案件の CI はリリースの名前をマージ commit の SHA（40字）にする**（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 17）。Sentry の release、Cloudflare の Worker の version の tag がこれに当たる。付けるのは案件の CI の仕事で、Andon は形だけ約束する（command は足さない）。

- 信号の記録（§15）は出所が持てば `release` にその SHA を写す。
- 辿るのは担当（トリアージ・観察）で、手で: `gh pr list --search <sha> --state merged` で PR を、kanban の `pr` キーで課題を引く。

## 17. 届けた通知の記録（`<slug>.delivered.jsonl`）

`~/.local/state/andon/dispatcher/<slug>.delivered.jsonl`（`XDG_STATE_HOME` があればその下。控えと同じ所）。dispatcher がフックで `<news>` を Chief of Staff に渡し、**そのターンの終わりを見たとき**に1行足す（[決定](decisions/2026-11-12-measure-news-pass-through.md)）。§6 の記録だけの置き場と対になり、`andon measure`（§13 の `news`）が kind ごとの素通りを数える。

```json
{"at": "2026-11-12T01:00:00Z", "project": "sample-site", "acked": true,
 "news": [{"issue": "S-0012", "kind": "report"}, {"kind": "error"}]}
```

| キー | 意味 |
|---|---|
| `at` | dispatcher（`andon dispatcher drain`）がそのターンの終わりを見た時刻（UTC） |
| `project` | プロジェクトの slug |
| `acked` | そのターンに `andon news ack` があった（端末のラベル `news.ack` が渡したときに控えた `turn` と同じ。§6）。ラベルの一致だけで決め、`news.replied` のラベルを書けたかとは分ける（書けなくても `true`） |
| `news[]` | そのターンで渡した通知（`issue` は課題の通知だけ・`kind`）。順は `<news>` と同じ。1件以上 |

- 書き手は dispatcher だけ（`drain` が、渡した turn の終わりを Stop で見たときと、lock の外で見た終わり〈`noteStop`〉を次の `drain` で確かめ済みにしたとき）。追記だけで、書き換えも切り詰めもしない。読み手は `andon measure`。
- 終わりを見ないまま別のターンが来た（届け直す）ターンは書かない。同じ内容の行（同じ秒に同じ通知を渡した別のターン）も飛ばさずに書く。届け直した通知は、届け直したターンの終わりを見たときに書く。
- 数えのための記録なので、書けなくても `<news>` の届けは止めない（`drain` の stderr に1行。その行は数えから落ちる）。
- `<no-reply>` の照合は v0.0.9 で外したので、素通りは `acked` だけで数える（[ADR-0010](adr/0010-news-ack.md) 決めたこと 6）。この記録の前のバージョンの控えにあった `replied`（kind を分けない合計）は読まない。

## 18. 起動の記録（`launches.jsonl`）

エージェントを起動したときのバージョン（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 3・7・14 の 5）。担当・レビュアー・Chief Engineer は課題の `library/launches.jsonl`、Chief of Staff は kanban の `chief-of-staff/launches.jsonl` に、**起動1回に1行**（JSON Lines）を足す。書き手はエージェントを起動する command（`andon issue start`・`andon issue resume`〈`--fresh` も〉・dispatcher のレビュアーの起動〈使用量の上限で起動し直すときも〉・`andon chief-engineer`・`andon chief-of-staff`）。値は `hc run` の直前に集め、`hc run` が端末を起動できたら端末の address を足して書く。追記だけで、前の行は書き換えない。読み手は振り返り（gate も dispatcher も判定に使わない）。

```json
{"at": "2026-11-19T09:00:00Z", "role": "worker", "launch": "start", "issue": "S-0012", "profile": "claude", "template": "claude",
 "command": "s=$(andon issue session); claude --model opus --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md ${s:+--session-id \"$s\"}",
 "terminal": "t42", "session": "0f8fad5b-d9cb-469f-a165-70867728950e",
 "harness": {"name": "claude", "version": "2.1.0 (Claude Code)"},
 "package": {"root": "/opt/andon", "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "source": "applied", "drift": false, "reasons": []},
 "workspace": {"path": "/workspace", "commit": "3d2e1f089c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a", "dirty": false},
 "repo": {"name": "github.com/example/sample-site", "path": "/workspace/.worktrees/sample-site/sample-site-S-0012",
          "commit": "7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f089c4e1b0d", "dirty": false},
 "external": [{"kind": "skill", "name": "sample-skill", "path": "/home/andon/.agents/skills/sample-skill", "source": "https://github.com/example/skills",
               "ref": "refs/tags/v1.0.0", "version": "1.0.0", "revision": "1f0e9c7b5a3d2e1f089c4e1b0d7a3f5e2c8b6d4a"}],
 "drift": false, "drift_reasons": [], "memory_index": "- [テストは tests/run --changed](test-changed.md) — 変えたものだけ走らせる\n"}
```

| キー | 意味 |
|---|---|
| `at` | 集めた時刻（`hc run` の直前。UTC） |
| `role` | `worker`・`reviewer`・`chief-engineer`・`chief-of-staff`。課題の起動ではプロファイルから決める（`chief-engineer` は Chief Engineer、`*-review` はレビュアー、ほかは担当） |
| `launch` | `start`（新しい端末）・`resume`（止まった端末をセッションの続きから。`andon issue resume`）・`fresh`（新しいセッションで。`--fresh`）・`fallback`（使用量の上限で止まったレビュアーを別のプロファイルで） |
| `issue` | 課題の id。Chief of Staff は `null` |
| `profile`・`template` | プロファイルの名前と、走らせた hc の template（続きからは `<profile>-resume`） |
| `command` | template の command（起動行。モデルと effort を含む）。起動の前に template があるかを確かめるときに `hc templates --json` から読んだもの。読めなければ `null` |
| `terminal` | 起動した端末の address |
| `session` | ハーネスのセッションの id。起動のときに決まらないもの（Codex の新しい起動・Chief of Staff）は `null`。片付けのあとにセッションログを引くのは、この全部の行（`issue.yaml` の `session` は起動し直しで上書きされる） |
| `harness` | `name`（`claude`・`codex`。プロファイルのブランド）と `version`（`<name> --version` の1行目。打てなければ `null`） |
| `package` | `andon version` の出力（パッケージの置き場の `root`・`commit`・`source`・`drift`・`reasons`） |
| `workspace` | セルの repo（`ANDON_WORKSPACE`。既定 `/workspace`）の `commit`（HEAD）と `dirty`（`git status --porcelain` が空でない。git が無視するものは数えない）。git の checkout でなければ `null` |
| `repo` | 案件の repo（`name`）と、エージェントが働く worktree（`path`。レビュアーは PR の head を読む detached worktree）の `commit` と `dirty`。repo の無い課題と Chief of Staff は `null` |
| `external` | 外から入る skill（`<claude>/skills` と `~/.agents/skills` の、`SKILL.md` を持つ実ディレクトリ。Andon のパッケージと image の備品を指す symlink は数えない。Codex は `~/.agents/skills` だけ）と、Claude Code の plugin（`<claude>/plugins/installed_plugins.json`）。skill の `source`・`ref`・`version`・`revision` は `gh skill install` が `SKILL.md` の `metadata` に書く `github-repo`・`github-ref`・`version`・`github-tree-sha`、plugin の `version`・`revision` は `version`・`gitCommitSha`。無いものは `null`。置き場が無ければ `[]`、読めなければ `null` |
| `drift` | 当時の文を git から復元できない印。`package.drift` が `true` か、セルの repo に commit していない変更がある（`workspace.dirty`）なら `true`。どちらかを確かめられず、`true` でもなければ `null` |
| `drift_reasons` | `drift` が `true` か `null` の理由（1行ずつ）。`false` なら空 |
| `memory_index` | auto memory の目録（`<セルの repo>/memory/MEMORY.md`）の写し。Claude Code の起動だけ（Codex は読まないので `null`）。無ければ `null` |

- **取れない値は `null` と明示する**。キーは全部書く。記録が書けなくても起動は戻さない（端末はもう動いている。command の stderr か dispatcher のログに1行）。`hc run` が落ちた起動と、最初のメッセージが届かずに `andon issue resume` が片付けた起動は書かない。最初のメッセージが届かずに残った端末（`andon issue start`・`andon chief-engineer`・`andon chief-of-staff` の終了コード 3、dispatcher の送り直し）は書く。
- **案件の repo の `dirty` は `drift` に数えない。** 起動し直しでは worktree に担当の作業中の変更があるのが普通で、エージェントが読む文（`AGENTS.md` など）の変更とは分けられない。`repo.dirty` は事実として残し、振り返りが読む。
- 当時の文の復元は git から: `git show <package.commit>:package/<パス>`（パッケージの置き場はこの repo の `package/` を写したもの）・セルの repo と案件の repo はそれぞれの `commit`。外の skill と plugin はバージョンの名前だけで、文はその repo の履歴から辿る。`--model opus` のような別名が実際にどのモデルになったかは、ここには無い（片付けのあとにセッションログから読む）。

## 19. 結果の記録（`outcome.json`）

課題の `library/outcome.json`。課題が片付いたときの結果・head ごとの gate の最後の判定・実働・セッションごとのコストを、振り返りの入力として1つにまとめる（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 7）。書き手は dispatcher だけで、`log.jsonl` の `{"status": "done"}` の行（`andon issue finish`。Chief of Staff の `kanban done` も）を拾った課題ごとに**1回だけ**書く。もう在れば書き直さない（同じ finish を2度拾っても1回。§14 と同じ）。オーナーの課題（`who: person`）には書かない。レビュアー・Chief Engineer・調べる課題には書く（それぞれのセッションのコストを持つ。親に束ねるのは読み手）。読み手は振り返りだけで、gate も dispatcher も判定に使わない。

```json
{"at": "2026-11-19T10:00:00Z", "issue": "S-0012", "result": "merged",
 "pr": "https://github.com/example/sample-site/pull/12", "merge_commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "merged_at": "2026-11-19T09:58:00Z",
 "gates": [{"head": "7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f089c4e1b0d", "at": "2026-11-19T09:20:00Z", "tier": "B", "decision": "fix", "reasons": ["Must fix 以上が 1 件"], "merged": false},
           {"head": "3d2e1f089c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a", "at": "2026-11-19T09:55:00Z", "tier": "B", "decision": "approve", "reasons": [], "merged": true}],
 "reviewers": 2, "worked_minutes": 95,
 "sessions": [
   {"harness": "claude", "session": "0f8fad5b-d9cb-469f-a165-70867728950e",
    "cost": {"models": ["claude-opus-5-5"], "efforts": ["high"], "turns": 6, "tokens": {"input": 1200, "output": 35000, "cache_read": 2400000, "cache_write": 180000}}},
   {"harness": "claude", "session": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "cost": null}]}
```

PR 無しで片付いた課題で、起動の記録の無い（この仕組みの前に起動した）とき:

```json
{"at": "2026-11-19T10:00:00Z", "issue": "S-0013", "result": "none", "pr": null, "merge_commit": null, "merged_at": null,
 "gates": [], "reviewers": 0, "worked_minutes": 20, "sessions": null}
```

| キー | 意味・元 |
|---|---|
| `at` | dispatcher が集めた時刻（UTC） |
| `issue` | 課題の id |
| `result` | `merged`（PR がマージされた）・`closed`（マージされずに閉じられた）・`open`（PR が開いたまま片付いた）・`none`（`report.json` に `pr` が無い）。片付けを拾ったあとに `gh pr view <n> --repo <owner>/<repo> --json state,mergeCommit,mergedAt` で読む。CI の失敗と revert は値にしない（CI の失敗は `gates` の理由で、revert はマージ後のことなので振り返りが GitHub から読む） |
| `pr` | `report.json` の `pr`。`none` なら `null` |
| `merge_commit`・`merged_at` | GitHub のマージ commit と時刻。`merged` でなければ `null` |
| `gates[]` | `library/gate/<head>.json` を1枚ずつ写したもの（`head`・`at`・`tier`・`decision`・`reasons`・`merged`）。記録は head ごとに最後の判定だけなので、同じ head で2度判定したら後の1つ。`at` の順。gate が回らなかった課題（`library/gate/` が無い）は空。置き場が一覧できなければ書かない |
| `reviewers` | kanban の、`parent` がこの課題でプロファイルが `*-review` の課題の数（頼み直しも数える） |
| `worked_minutes` | 実働（分）。`log.jsonl` を書くときに読み直し、最後の `done` の行までを（同じ秒にあとから書かれた行は数えない。読めない・`done` の行が無ければ書かない）、`andon measure` と同じ数え方（`ledger.Worked`。問い・レビュー・端末が死んでいた間を除く）で |
| `sessions` | 起動の記録（§18）にある全部のセッション。起動の記録が無ければ `null`（さかのぼって作らない） |
| `sessions[].harness`・`session` | ハーネスと、セッションの id。Claude Code は起動の記録の `session`（同じセッションは1つ。`session` が `null` の起動は1つにまとめて `cost` は `null`）。Codex は起動のときに id が決まらないので、cwd が課題の置き場（kanban の `issues/<id>/`・`issue.yaml` の `worktree`・起動の記録の `repo.path`）の rollout を、`session_meta` の id ごとに1つ。rollout が1つも無ければ `session` も `cost` も `null` の1件 |
| `sessions[].cost` | セッションログから、書くときに読む。ログが無ければ `null`（Claude Code は既定で 30 日で消す）。Claude Code のログのファイルかサブエージェントの置き場、Codex の rollout を読み切れなかったときも `null`（未計測。トークンが0のセッションと分ける）。トークンの数え方は `andon measure` と同じ（`internal/sessionlog`）。`models` は実際に使われたモデルの id（サブエージェントのものも。出た順）、`efforts` は effort（出た順。記録に無ければ空）、`turns` はターン数（Claude Code は本線の user の行の `promptId` の数、Codex は `turn_context` の `turn_id` の数。印が無ければ `null`） |

- **ファイルが無いのは「集めていない」。** 読めないもの（gh・kanban・gate の記録・起動の記録）があれば書かず、次の見回りで1回だけ打ち直す。それも駄目なら dispatcher のログに1行で諦める（通知は積まない。§14 の `corrections` と違い、Chief of Staff が手で集め直す口は無い。振り返りは無い欄を `null` として読む）。gh の上限で止めている間（控えの `GHReset`）は打たず、回数にも数えない。
- 片付けたあとに状態が変わる PR（`open` のまま片付いて、あとでオーナーがマージした）は、書いたときの状態のまま。書き直さない（「1回だけ書く」。マージ後のことは振り返りが GitHub から読む）。

## 20. 振り返りの入力（`andon retro collect` の出力）

`andon retro collect [--days N]` が、振り返り（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 8 の「集める」）の入力を1つの JSON にまとめて標準出力に出す。**読むだけで、何も書かない・保存しない**。置き場は無い（§13 と同じ）。期間は打った時刻から N 日（既定 7。1〜90）。kanban のファイルを直に読む（kanban の CLI は打たない）。

束は2つの形で出す（[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 6）。前の振り返りが集めたあと（境目 `boundary` より後）に片付いた束は全文で `issues[]` に、それより前に片付いた束は要約で `summaries[]` に入れる。境目より後の束が 100 件を超えたら、新しい順に 100 件を全文にし、残りを要約にして `cut` に書く。根拠として指せる id は、全文と要約のどちらの束の id も同じ。要約の束の全文は `andon retro collect [--days N] --issue <id>` で引く。`--issue` は、期間に片付いた担当の課題1件の束を `issues[]` の1件と同じ形の object で出す（境目は問わない。オーナーの文と前の直しは集めない。読めなかったものは stderr に1行ずつ）。期間の束でなければ終了コード 1。

バージョン 2（`retro/2`）: 全文の束の `launches[]` と `outcome` を、パターンを拾うのに使う所だけに改めた。全文は `library` の `launches.jsonl` と `outcome.json` にある。

```json
{"project": "sample-site", "at": "2026-11-19T12:00:00Z", "since": "2026-11-12T12:00:00Z", "until": "2026-11-19T12:00:00Z",
 "previous_retro": {"issue": "S-0040", "done_at": "2026-11-18T09:30:00Z", "until": "2026-11-18T09:10:00Z"},
 "boundary": "2026-11-18T09:10:00Z",
 "issues": [
   {"id": "S-0012", "title": "返事の下書きボタン", "profile": "claude", "repo": "github.com/example/sample-site",
    "done_at": "2026-11-18T10:00:00Z", "library": "/workspace/projects/sample-site/kanban/issues/S-0012/library",
    "estimate": "medium", "agent_estimate": null, "worked_minutes": 95, "pr": "https://github.com/example/sample-site/pull/12",
    "launches": [{"at": "2026-11-18T08:00:00Z", "profile": "claude", "model": "opus", "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}],
    "outcome": {"result": "merged", "worked_minutes": 95,
                "cost": {"sessions": 2, "measured": 1, "tokens": {"input": 1200, "output": 35000, "cache_read": 2400000, "cache_write": 180000}}},
    "gates": [{"head": "3d2e1f089c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a", "at": "2026-11-18T09:55:00Z", "tier": "B", "decision": "approve", "reasons": [], "merged": true}],
    "reviews": [{"id": "S-0012-r1", "profile": "codex-review", "reason": "first",
                 "findings": {"commit": "3d2e1f089c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a", "findings": [{"severity": "major", "file": "app/reply.ts", "line": 40}]},
                 "outcome": null}],
    "waived": [], "corrections": [{"kind": "answer", "question": 2, "decision_kind": "users", "recommended": "CSV だけ", "picked": "CSV と Excel"}],
    "remember": ["返事の下書きは保存の前に空白を詰める"], "agents_md": [], "journal": "- 理解: 完了条件が2通りに読めて迷った\n",
    "after_merge": {"merge_commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "merged_at": "2026-11-18T09:58:00Z",
                    "checks": {"state": "failure", "failed": ["tests"]},
                    "reverted_by": [{"pr": "https://github.com/example/sample-site/pull/13", "merged_at": "2026-11-18T15:00:00Z"}]},
    "reapplied": false, "reapplied_reasons": []}],
 "summaries": [
   {"id": "S-0005", "title": "一覧の並び順", "done_at": "2026-11-15T10:00:00Z", "pr": "https://github.com/example/sample-site/pull/5",
    "findings": ["major — app/list.ts:12 空の一覧で並べ替えが落ちる。"], "gate_fixes": ["Must fix 以上が 1 件"], "corrections": false, "remember": 2}],
 "cut": null,
 "typed": [{"id": "h-6f1c2d3e-0000-4000-8000-000000000001", "session": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "at": "2026-11-18T11:00:00Z",
            "role": "chief-of-staff", "issue": null, "text": "PR の題に課題番号を書かないで。鍵は [REDACTED:github]", "truncated": false}],
 "fixes": [{"issue": "S-0041", "title": "見送りの理由を報告に必ず書かせる", "retro": "S-0040", "pr": "https://github.com/example/sample-site/pull/20",
            "merged_at": "2026-11-16T09:00:00Z", "issues_after": 6, "recurred_before": 1}],
 "redacted": 1,
 "skipped": [{"source": "github.com/example/other", "reason": "マージされた PR の一覧が読めない（gh pr list: HTTP 404）。この repo のマージ後のことは null"}]}
```

| キー | 意味・元 |
|---|---|
| `previous_retro` | 片付いた振り返りの課題のうち、最後に `done` になったもの（`log.jsonl` の時刻）。振り返りの課題は、`issue.yaml` の `routine` が `retro`（ルーチン retro が足した）か、`library/retro.json`（振り返りが書くパターンの一覧）があるもの。無ければ `null`。`until` は前の振り返りが集めた時刻（その `retro.json` の `until`）。`retro.json` が無い・形が違う・`until` が片付いた時刻より後なら `null`（読めなかったときは `skipped` に1行） |
| `boundary` | 境目。`previous_retro.until`、それが `null` なら `previous_retro.done_at`。前の振り返りが無ければ `null`（どの束も新しい）。境目より後に片付いた束と打たれた文が、パターンの「新しい出来事」になる（§21） |
| `issues[]` | 全文の束。期間に `done` になった担当の課題のうち、境目より後のもの（新しい順に 100 件まで）。片付いた順。レビュアーの子（`parent` があり `*-review` か `<親>-r<k>`）・オーナーの課題（`who: person`）・Chief Engineer の課題（`profile: chief-engineer`）・振り返りの課題は入れない。根拠として指す id は課題の id |
| `issues[].launches` | 起動の記録（§18）の行ごとの `at`・`profile`・`model`（`command` の `--model` か `-m` の値。無ければ `null`）・`commit`（`package.commit`。無ければ `null`）。全文は `library` の `launches.jsonl`。ファイルが無ければ `null` |
| `issues[].outcome` | 結果の記録（§19）の `result`・`worked_minutes` と、`sessions[]` のコストの合計 `cost`（`sessions` はセッションの数、`measured` はそのうち `cost` があった数、`tokens` はその合計）。`sessions` が `null` なら `cost` は `null`。全文は `library` の `outcome.json`。無ければ `null`（この仕組みの前の課題） |
| `issues[].gates` | gate の記録（§7）の `head`・`at`・`tier`・`decision`・`reasons`・`merged`。判定の時刻の順 |
| `issues[].reviews` | レビュアーの子の課題ごとの `findings.json`（§3）と `outcome.json` の写し。無ければ `null`。`reason` はその回を頼んだ理由で、レビュアーの依頼書の「再レビュー: 前の回の commit」の段（dispatcher が書く）と、その commit を見た前の回の `findings.json` から決める。`first` は1回目、`fixed` は前の回に critical か major の指摘（課題の前からあった指摘を除く）があった回、`rebased` は前の回の commit が今の commit の祖先でない（rebase をまたいだ）回、`rerequested` はどれでもない回（前の回の結果が古い commit を見ていた・CI や軽い指摘を直した・前の回が結果を残さなかった）。依頼書が無い、または2回目以降なのに再レビューの段が無ければ `null` |
| `issues[].estimate`・`agent_estimate`・`worked_minutes` | `issue.yaml` の `estimate`・`report.json` の `estimate.size`・`outcome.json` の `worked_minutes` |
| `issues[].waived`・`remember`・`agents_md` | `report.json` の写し |
| `issues[].corrections` | `corrections.json`（§14）の `corrections`。ファイルが無ければ `null`（集めていない） |
| `issues[].journal` | 日報（`library/journal.md`）を丸ごと写したもの（行を読まない）。無ければ `null` |
| `issues[].after_merge` | マージ後のこと。`report.json` の `pr` がマージされていれば GitHub から読む: repo ごとに1回の `gh pr list --repo <owner>/<repo> --state merged --limit 1000 --json number,url,title,body,mergedAt,mergeCommit` と、マージ commit ごとの `gh api --paginate repos/<owner>/<repo>/commits/<sha>/check-runs?per_page=100`（ページを最後まで読む）。`checks.state` は check run の `conclusion` が `failure`・`timed_out`・`startup_failure`・`action_required` のものが1つでもあれば `failure`（`cancelled` は数えない）、終わっていないものがあれば `pending`、check run が無ければ `none`。`reverted_by` は、題が `Revert ` で始まり本文に `<owner>/<repo>#<n>` か PR の URL があるマージされた PR（`andon measure` と同じ照合）。PR が無い・マージされていない・一覧が読めないなら `null`。信号の記録（§15）は使わない（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 14 の 4） |
| `issues[].reapplied` | 課題の途中でパッケージが当て直された印。当て直しの記録（パッケージの置き場の `applied.json` の `applied_at` と、前に当てた分の `previous[].applied_at`。[ADR-0027](adr/0027-apply-package-without-rebuild.md)）のどれかが最初の起動と片付けの間にあるか、起動の記録のパッケージの commit（`launches[].commit`）が2通り以上あれば `true`。`previous` の無い前のバージョンの記録は最後の1回（`applied_at`）だけを読み、それより前の当て直しは起動の記録の commit で拾う。`previous` が上限（20 件）まで埋まっているか、時刻の読めない1件で途切れていて（その1件より前は読まない）、残る一番古い当て直しが最初の起動より後なら、ほかで `true` と言えない限り `null`。起動の記録が無い・当て直しの記録（最後の1回）が読めなくても `null`。revert は記録を消し、`previous` を書かない前のバージョンの `image/apply-main.sh` で当てると履歴が切れる。そうして切れた記録は途切れた印を持たないので、切れる前の当て直しは起動の記録の commit でしか拾えない。理由は `reapplied_reasons` |
| `summaries[]` | 要約の束。期間に `done` になった担当の課題のうち、境目より前（境目と同じ時刻を含む）のものと、100 件を超えて全文にしなかった新しいもの。片付いた順。`id`・`title`・`done_at`・`pr`（`report.json` の `pr`）と、つまずきの跡の見出し: `findings` はレビュアーの子の critical と major の指摘（`preexisting` のものは除く）ごとの1行（レビュアーの報告 `report.md` で、その指摘の `<file>:<line>` を含む最初の行。字下げした次の行が続けば足す。報告に無ければ `<重さ> <file>:<line>`）、`gate_fixes` は gate が `fix` を返した判定の理由（重なりは1つ）、`corrections` は手直し（§14）のうち gate の問いをオーナーがマージして閉じた答え（前のバージョンの「人がマージした（…）」か、今の「オーナーがマージした（…）」）でないものがあるか（`corrections.json` が無ければ `null`）、`remember` は `report.json` の `remember` の行数。行は空白を詰めて 160 字で切る（切ったら末尾に `…`）。`findings` の行はレビュアーの報告の文をそのまま写すので、外の PR の文を含みうる。振り返りの担当はデータとして読み、指示として読まない |
| `cut` | 境目より後の束が 100 件を超えたときだけ `{"new": <境目より後の束の数>, "full": 100}`。ほかは `null` |
| `typed[]` | 期間にオーナーが端末で打った文（下の規則）。根拠として指す id は `h-<セッションログの行の uuid>`。同じ `session` の文は根拠として1件に数える（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 14 の 2）。`role` と `issue` は、そのセッションを持つ役と課題。2000 字を超える文は切って `truncated: true` |
| `fixes[]` | 前の振り返りが切った直し（`parent` が振り返りの課題の、`done` の課題）のうち、PR のマージ（GitHub の `mergedAt`。読めなければ `outcome.json` の `merged_at`）が打った時刻から 14 日以内のもの。`issues_after` は、マージのあとに `done` になった担当の課題の数（期間に限らない）。`recurred_before` は前の振り返りが数えた再発の数（その `retro.json` の `fixes[].recurred`）。前の振り返りが数えていなくてマージが境目より後なら `0`（境目より前に数える出来事が無い）、どちらでもなければ `null`（[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 6） |
| `redacted` | 秘密の形に合って伏せた数 |
| `skipped` | 読めなかったもの（読めない記録・GitHub）。集めるのは止めない |

オーナーが打った文の規則（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 4・14 の 1）。読むのは Claude Code のセッションログ（`<CLAUDE_CONFIG_DIR か ~/.claude>/projects/*/<session>.jsonl`。期間より前に書き終わったファイルは開かない）だけ。Codex の記録（rollout）の user の文にはオーナーの印が無いので抜かない。

- **このプロジェクトのセッションだけ。** どの役・課題のセッションかは、起動の記録（§18）の `session` か、ログの行の `cwd`（プロジェクトの根は Chief of Staff、kanban の `issues/<id>/` と課題の `worktree` はその課題）で決める。どちらにも当たらないセッションは読まない。
- **抜く行**: `type` が `user`・`isSidechain`・`isMeta`・`isCompactSummary` が真でない・`origin.kind` が `human`・中身が文字列か text の block（道具の結果の block があれば抜かない）・時刻が期間の中。
- **除く行**: セッションの最初のオーナーの印の文（起動の最初のメッセージ）。包みのある文（先頭が `<hc from=`・`<news`・`<task-notification>`。貼った文の包み `<pasted_content …>` の内側も見る）。Andon の command が端末の外から `hc send` した文（hc の包みが付かずオーナーの印になる）。dispatcher と gate は、担当・レビュアーへ送る文の先頭に送り手の印 `[dispatcher] `・`[gate] ` を置く（dispatcher.md §5 の「送り手の印」）。この印で照らし、文の意味は読まない。貼った文の包みの内側も見る。
- 文の意味は読まない。抜いた文はこの出力にだけ置き、PR には写さない（[ADR-0028](adr/0028-improvement-loop.md) 前提 7）。

秘密を伏せる: 出力の JSON を書いたあと、文字列の中で秘密の形に合う所を `[REDACTED:<形の名前>]` に置き換える（鍵の名前に続く値は名前を残して値だけ）。形は秘密鍵の塊・Anthropic と OpenAI の API キー・GitHub のトークン・AWS のアクセスキー・Slack のトークン・Google の API キー・JWT・`Bearer` のトークン・`api_key`／`token`／`password` などの名前に続く値（値の長さは問わない）。一覧の正は `internal/redact/redact.go` で、ダイジェスト（§26）と共有する（検査は偽の値で `internal/retro/retro_test.go`）。多めに伏せる。伏せたあと JSON として読めなければ出さずに終了コード 1。

## 21. パターンの一覧（`retro.json`）

振り返りの課題の `library/retro.json`。振り返りの担当が、振り返りの入力（§20）から拾った繰り返し（パターン）と、前の直しごとの再発の数と見立てを書く（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 8 の「判定を書く」）。担当は手で書かず、`andon retro patterns <id> [--days N] <json|->` で書く（打てるのはその課題の担当の端末〈`agent` ＝ `HC_TERMINAL_ID`〉で、課題が `doing` のときだけ）。入力は `patterns` と `fixes` の2つだけで、残りの欄（`retro`・`at`・`since`・`until`・`days`・`previous_retro`）は command が書く。command は打った時刻から `--days`（既定 7。collect と同じ値を渡す）の入力を集め直し（GitHub も読む）、下の検査を全部通したときだけ書く。断るときは書かずに終了コード 1 と理由1行で、前の一覧はそのまま。書き直すたびに全体を書く。

```json
{"retro": "S-0050", "at": "2026-11-19T12:00:00Z", "since": "2026-11-12T12:00:00Z", "until": "2026-11-19T12:00:00Z", "days": 7,
 "previous_retro": {"issue": "S-0040", "done_at": "2026-11-18T09:30:00Z"},
 "patterns": [
   {"id": "p1", "summary": "PR の題に課題番号が入り、オーナーが2回直した", "evidence": ["S-0012", "h-6f1c2d3e-0000-4000-8000-000000000001"],
    "level": "test", "action": "add", "target": {"kind": "project", "repo": "github.com/example/sample-site", "paths": ["tests/pr-title.sh"]},
    "effect": "題に課題番号が入った PR が CI で落ちる", "risk": "題の検査が、課題番号に似た語を誤って拾う", "decision": "fix", "reason": "テストで止められ、根拠が2件ある"},
   {"id": "p2", "summary": "見送りの理由が報告に無く、検収で2回戻した", "evidence": ["S-0012", "S-0015"],
    "level": "skill", "action": "change", "target": {"kind": "andon", "repo": "github.com/example/andon", "paths": ["package/skills/worker/SKILL.md"]},
    "not_upstream": "見送りの理由は文で、機械が確かめる値が無い", "effect": "報告の見送りに理由が付く", "risk": "worker skill が長くなる",
    "decision": "drop", "reason": "前の直しが2日前にマージされたばかりで、まだ効き目が分からない"}],
 "fixes": [{"issue": "S-0041", "recurred": 0, "view": "マージのあと 6 件片付き、再発は無い。残す"}]}
```

| キー | 意味 |
|---|---|
| `retro`・`at`・`since`・`until`・`days` | command が書く。書いた振り返りの課題と、集めた入力の期間。`andon retro apply` は `until` と `days` で同じ期間を集め直す |
| `previous_retro` | command が書く。集めたときの前の振り返り（期間の終わりまでに片付いた振り返りの課題のうち最後。この課題自身は数えない） |
| `patterns[].id` | `p1`・`p2` …。一覧の中で一意。直す課題の依頼書から指す名前 |
| `patterns[].summary` | 何が何回起きたかの1行。直す課題の題にもなる（`-` で始めない）。オーナーのコメントやオーナーが打った文の本文を写さない（[ADR-0028](adr/0028-improvement-loop.md) 前提 7） |
| `patterns[].evidence` | 根拠の id。入力（§20）の `issues[].id`・`summaries[].id`（課題）か `typed[].id`（オーナーが打った文。`h-…`） |
| `patterns[].level` | 直す先の段（[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 9。効き目の確かな順）。`design`＝設計・型（schema・command の照合・状態の定義）・`test`＝テスト・lint（gate の条件表を含む）・`check`＝決まった手順の確かめ（hook・command の中の確かめ）・`skill`＝skill・指針（プロファイル・手引き・`AGENTS.md`・`REVIEW.md`）・`text`＝文面（最後の手段） |
| `patterns[].action` | `add`（足す）・`change`（変える）・`remove`（削る。効いていない指示を消すのも直し） |
| `patterns[].target` | 直す先。`kind` は `andon`（Andon のパッケージ）か `project`（案件の repo のテスト・`AGENTS.md`・`REVIEW.md` など）。`repo` は `<host>/<owner>/<repo>`（`project` は必須でプロジェクトの `repos` の中。`andon` は持っていれば書く）。`paths` は repo の中の相対パス（1つ以上） |
| `patterns[].not_upstream` | 上の段（設計・テスト・確かめ）で止められない理由の1行。`level` が `skill` か `text` なら必須 |
| `patterns[].effect`・`risk` | 期待する効果（何が起きなくなるか）とリスク。1行ずつ |
| `patterns[].decision`・`reason` | `fix`（直す課題に切る）か `drop`（捨てる。持ち越さない）と、その理由の1行。`drop` の理由は中身だけで、数の都合を書かない。`fix` のパターンは優先の順に並べる（[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 3） |
| `fixes[]` | 入力の `fixes[]`（前の振り返りが切った直しのうちマージから 14 日以内のもの）の全部に1件ずつ、過不足なく。`recurred` はマージのあとに同じパターンが起きた数で、入力の `recurred_before` に全文の束で見つけた再発を足したもの（`recurred_before` が `null` なら要約の束でも数える。[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 6）、`view` は見立ての1行（残す・効き目の確かな段へ振り分け直す・効かなかった文を削る候補にする）。再発したものは、文を足さずに上の段か削る候補をパターンに出す（決めたこと 6） |

command が断ること（`andon retro patterns` と `andon retro apply` の両方。[ADR-0028](adr/0028-improvement-loop.md) 決めたこと 8）:

- 根拠が別々の2件に満たないパターン。数える単位は課題（レビュアーの子の課題は親）と、オーナーが打った文はそのセッションを持つもの（担当とレビュアーのセッションはその課題、Chief of Staff のセッションはセッションごと）。同じ課題の中の2回・同じセッションの中の2回・課題とその課題の担当が打たれた文は、1件と数える（決めたこと 14 の 2）。
- 根拠が入力に無い id を指しているパターン。
- 根拠のどれも境目（§20 の `boundary`。前の振り返りが集めた時刻）より後でない（後に片付いた課題か、後に打たれた文が1件も無い）パターン（決めたこと 14 の 3）。前の振り返りが無ければ問わない。根拠は全文の束・要約の束・オーナーの文のどれの id でもよい。
- `level` が `skill` か `text` なのに `not_upstream` が無いパターン。
- 判定（`decision`）の無いパターン。`fix` の数は確かめない（[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 1）。

`target.paths` が振り返り自身（`package/skills/retro`・`internal/retro` とその下）を指すパターンも断らない（[ADR-0039](adr/0039-retro-scale-with-flow.md) 決めたこと 12。Andon のバージョンが v0.0.x の間の決まりで、決めたこと 15）。`level` が `skill` か `text` なら `not_upstream` が要るのは、ほかの直す先と同じ。
- オーナーの課題になる `fix` のパターン（`target.kind` が `andon` で、`target.repo` が無いかプロジェクトの `repos` に無い）が2つ、同じ `summary` を持つ。2つ目が同じ問いのオーナーの課題になり、`apply` が1つ目を足したあとで断られるため（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 11）。
- 形の誤り（知らないキー・空の文・改行・`project` の `repo` がプロジェクトの `repos` に無い）。`andon retro patterns` だけは、`fixes[]` が入力の前の直しと過不足なく合うかも確かめる（`apply` は GitHub を読まないので照らさない）。

確かめないこと: そのパターンが根拠の課題で本当に起きたか。見るのは、検収する Chief of Staff と、直す PR を読むレビュアーとオーナー。

**当てる**（`andon retro apply <id> [--dry-run]`。Chief of Staff が検収して打つ）: `retro.json` を読み、`until` と `days` の期間で入力を集め直して上の検査を通し（GitHub は読まない）、`fix` のパターンごとに、一覧の順に課題を1件 `kanban add` する。足す課題の `parent` は振り返りの課題（§20 が前の直しを見分ける印）、題は `summary`、`repo` は `target.repo`。依頼書には、要約・直す先（段・足すか変えるか削るか・repo と paths）・上の段で止められない理由・期待する効果・リスク・完了条件・守ること（改善の PR の決まり）・根拠（課題は題と片付いた時刻と PR と、その下の字下げした1行に、課題の `report.md`・日報〈`library/journal.md`〉・レビュアーの子〈`<id>-r<回>`〉の `report.md` と `library/findings.json` のうち在るものの絶対パス。オーナーが打った文は id と役と時刻だけ。どちらも本文は写さない）・PR の本文に書いてよい根拠の PR・辿り方を書く。`target.kind` が `andon` で、`target.repo` が無いかプロジェクトの `repos` に無ければ、直す課題ではなくオーナーの課題（`who: person`。題は「Andon へ伝えるか: <summary>」）にする（決めたこと 2）。オーナーの課題は、`andon issue ask --new`（§5）と同じ関数で、問いと一緒に足す（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 11）。題と依頼書は上のまま（依頼書には推奨案の1行を足す）。問いは `kind` が `promise`、`question` が題と同じ文、`options` が「Andon のメンテナーへ伝える」（推奨案）と「捨てる」、`facts` の先頭が `<振り返りの id>/library/retro.json のパターン <id>`、`from` が `retro`。`profile` は書かない（起動する前に Chief of Staff が切る決まりで書く）。担当の課題には予約キー（§2）を既定で書く（§2 の「command が足す担当の課題」）。`--dry-run` は足す課題（オーナーの課題は、置く問いの JSON〈§5〉を `question` に持つ）と捨てるパターンを JSON で出すだけで、何も書かない。

当てた記録は振り返りの課題の `library/retro-applied.json`（書き手は `andon retro apply`。読むのはオーナーと Chief of Staff で、機械は判定に使わないので schema は持たない）。`retro`・`applied_at`・`complete`（全部を足したら `true`）・`issues[]`（`pattern`・`issue`・`who`〈`worker` か `person`〉・`title`）・`dropped[]`（`pattern`・`reason`）。1件足すたびに書き直すので、途中で kanban の CLI が落ちても足した分が残り、打ち直すと残りから続ける。kanban add のあと記録の保存で落ちたときは、足した課題が記録に無い。打ち直しは、この振り返りを parent に持つ同じ題の課題が kanban にあれば足さずに記録へ写すので、同じ課題を重ねて足さない。見つけた課題がオーナーの課題で、`who: person` か問いが無ければ（前の回が課題を足したあと、問いを置く前に落ちた）、課題を足さずに無い方だけを置く。問いはあるのに控えの `library/questions/1.json` が無ければ（`kanban ask` のあと控えを書く前に落ちた）、問いは足さずに控えだけを書き直す。同じ振り返りを当てる `andon retro apply` は1本ずつで、ほかの apply が `library/retro-applied.json.lock` を持っている間は kanban を照らす前に断る（`--dry-run` は書かないので取らない）。`complete` が `true` なら断る（当て済み）。

## 22. kanban の行（`log.jsonl`）

`projects/<slug>/kanban/log.jsonl`。kanban の CLI が kanban を変えるたびに1行（JSON Lines）を足す。書き手は `kanban` だけで、追記だけ。読み手は dispatcher（通知の元・結果の記録の実働・オーナーの手直し）・`andon issue`（stop-check・gate のあとのマージ）・`andon gate`（予約キーを Chief of Staff が書いたかの照合）・`andon measure`・`andon retro collect`。形の正は `schema/ledger/log.v3.schema.json`（[ADR-0034](adr/0034-prevention-before-tests.md) 決めたこと 7）で、どのコマンドが何を書くかの説明は [ledger.md](ledger.md) の `log.jsonl` の節。

行の形は4つで、キーで見分ける。`config` があれば config の行、`routine` があればルーチンの行、`migrate` があれば移した行、どれも無ければ課題の行。

```json
{"at": "2026-11-02T10:03:00Z", "by": "terminal:t3", "id": "S-0012", "change": {"status": "doing", "agent": "t3", "branch": "sample-site/S-0012-add-the-button"}}
```

```json
{"at": "2026-11-02T10:05:00Z", "by": "terminal:dispatcher", "id": "S-0012", "change": {"question": 1, "status": "open"}}
```

```json
{"at": "2026-11-02T09:00:00Z", "by": "terminal:t2", "config": {"chief_of_staff": "t2"}}
```

```json
{"at": "2026-11-03T00:00:00Z", "by": "routine:triage", "routine": "triage", "fired": "S-0013"}
```

```json
{"at": "2026-11-06T00:00:00Z", "by": "terminal:t5", "migrate": {"from": ".flightdeck", "ids": {"S-5": "S-0005"}}}
```

| 行 | キー | 意味 |
|---|---|---|
| どれも | `at` | 書いた時刻（UTC。`YYYY-MM-DDTHH:MM:SSZ`） |
| どれも | `by` | 書いた端末（`terminal:<HC_TERMINAL_ID>`。dispatcher は `terminal:dispatcher`）。端末の外では利用者の名前、それも無ければ `unknown`。ルーチンの行は `routine:<名前>` |
| 課題 | `id` | 課題の id。4 桁に揃える前の行は短い id（`S-5`）のまま。読み手は 4 桁に揃えて読む |
| 課題 | `change` | 変えたキーと新しい値（文字列・文字列の並び・消したキーは `null`）。`kanban set` はどのキーもこの形で書く。ほかに、`add`（`kanban add` の題）・`report`（`write` か `append`）・`question`（問いの番号。`status` は `open`・`answered`・`received`）・`forced`（`--force` か `--past` で越えた問いの番号。`status` と一緒）・`freedBy`（`kanban done` で `queued` から出した課題。`status` と一緒） |
| config | `config` | `kanban config set` が変えたキーと新しい値（消したら `null`）。2026-11-06 より前の行は `coordinator` |
| ルーチン | `routine`・`fired`・`skipped` | ルーチンの名前と、足した課題の id（`fired`）か足さなかった理由（`skipped`。`paused`・`open`）。どちらか一方 |
| 移した | `migrate` | `kanban migrate` が移した元のフォルダの名前（`from`）と、改めた id（`ids`。古い id → 新しい id） |

- 知らないキーは書かない（`additionalProperties: false`）。読み手は読めない行を飛ばす。キーを足すのは ledger.md と `schema/ledger/log.v3.schema.json` を同じ PR で直す。
- ルーチンのファイル `routines/<name>.yaml` に、Andon が読む欄 `idle_hours`（1 以上の整数。時間）を書ける。kanban の CLI は読まない（ルーチンの schema はほかのキーを許して残す）。書いたルーチンは、枠が空いたままの回に、ルーチンの行のうち `fired` のあるいちばん新しい行の `at` から `idle_hours` 時間以上たっていれば（1 度も足していなければすぐ）dispatcher が撃つ。値が 1 以上の整数でなければ欄が無いものとして扱う（[dispatcher.md](dispatcher.md) §2.13。[ADR-0047](adr/0047-idle-slots-run-upkeep.md) 決めたこと 9）。
- 本物の `kanban` に全部の形の行を書かせて schema に通すのは `cmd/kanban` のテスト（`TestLogAndReadyFitTheLedgerSchemas`）。Go のテストが手で書く `log.jsonl` は、補助 `internal/ledger/ledgertest` の `Log` を通して書く。補助は行が schema に合わなければテストを落とす。読めない行の扱いを確かめるテストだけは、補助を通さずに書く。

## 23. 引き取れる課題（`kanban ready --json` の出力）

`kanban ready --json` が出す課題の並び。`todo` か `queued` で、`after` が全部済み、`not_before` が先でない課題を、`issue.yaml` の値に `id`（フォルダの名前）を足した形で並べる。読めない課題は並べない。読み手は dispatcher（ready の差を通知 `ready` にする。オーナーの課題とレビュアーの子課題は除く）と `andon issue start`（起動してよいかの照合）。形の正は `schema/ledger/ready.v3.schema.json`（[ADR-0034](adr/0034-prevention-before-tests.md) 決めたこと 7）。

```json
[{"id": "S-0013", "title": "Add the button", "status": "todo", "repo": "github.com/example/sample-site", "profile": "claude",
  "created": "2026-11-02T10:00:00Z", "updated": "2026-11-02T10:00:00Z"},
 {"id": "S-0014", "title": "Decide the copy", "status": "queued", "who": "person", "after": ["S-0012"], "review": "on"}]
```

- 課題の項目は `schema/ledger/issue.v3.schema.json` の項目の写しで、`id` と `status` が必須、`status` は `todo` か `queued` に絞る。写しが issue.v3 とずれないことは `cmd/kanban` のテスト（`TestReadySchemaCopiesTheIssueSchema`）が見る。kanban の形式に倣い、知らないキー（Andon の予約キー〈§2〉を含む）は許す。
- 本物の出力を schema に通すのは §22 と同じ `cmd/kanban` のテスト。Go のテストが偽の `kanban` に返させる ready は、補助 `internal/ledger/ledgertest` の `Ready` を通して書く。

## 24. 走らせた記録（`runs/<n>.json`）

担当が証拠として走らせた command の記録（[ADR-0035](adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 2・8・11）。担当の課題の `library/runs/<n>.json` に記録1件、同じ番号の `library/runs/<n>.log` に出力を置く。書き手は `andon issue record <id> [--expect pass|fail] -- <command> [<arg>…]` だけで、担当は手で書かない。報告の `acceptance[]` の行は `run` にこの `id` を書いて記録を指す（§4）。

`andon issue record` のすること:

- `doing` の課題にだけ打てる（§4 の移り方の守り）。打てるのはその課題の担当の端末に限らない（長い検査を `hc run --bg` の端末で撮れるように）。
- 課題の `worktree`（無ければ今のディレクトリ）で、`--` のあとの command を shell を通さずに走らせる。パイプなどが要るときは `-- bash -c '…'` と書く。標準入力は渡さない。
- 標準出力と標準エラーを1つにまとめ、端末へ流しながら `runs/<n>.log` に残す。1 MiB を超えたら終わりの側を残し、ログの先頭に切った量の1行を置く。
- repo のある課題では、走らせる前の `HEAD` と、commit していない変更の有無（`git status --porcelain`。追跡していないファイルも数え、無視するファイルは数えない）を記録する。
- 記録は、終了コードが期待と違っても書く。期待（`pass` は終了コード 0、`fail` は 0 以外）と違えば終了コード 1 と理由1行で終わる。command を起動できない（無い command・打ち間違い）ときは、落ちた記録にせず何も書かずに終了コード 1。使い方の誤りは 2。`--` のあとに空の引数が1つでもあれば、走らせる前に使い方の誤りとして断る（記録の `command` の要素は空でない）。
- 番号 `n` は、`runs/` にある記録と出力の最大 ＋ 1。出力のファイルを排他で作って番号を取るので、同じ課題で並べて打っても重ならない。記録は書き足すだけで、前の記録を書き直さない。`finish` でも消さない。

```json
{"id": 3, "command": ["go", "test", "./internal/issue/", "-run", "TestRecord"],
 "expect": "pass", "exit_code": 0,
 "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "dirty": false,
 "started_at": "2026-11-20T09:12:03Z", "finished_at": "2026-11-20T09:12:41Z",
 "output": "runs/3.log", "output_bytes": 1824, "truncated": false}
```

| キー | 要るか | 意味 |
|---|---|---|
| `id` | 必須 | 記録の番号 `n`（1 以上）。課題の中で一意。`acceptance[].run` から指す名前 |
| `command` | 必須 | `--` のあとに渡した command と引数（1つ以上。空の引数は無い） |
| `expect` | 必須 | `pass`（終了コード 0 を期待する。既定）か `fail`（0 以外を期待する。誤りを戻すと検査が落ちることを示す記録。決めたこと 8） |
| `exit_code` | 必須 | command の終了コード。signal で終わったら `-1` |
| `commit` | repo のある課題では必須（schema では任意。書くのは `andon issue record`）。repo の無い課題では書かない | 走らせる前の worktree の `HEAD`（40桁・小文字） |
| `dirty` | `commit` があるときだけ、必ず書く | 走らせる前に commit していない変更があったか（`git status --porcelain` が空でない。git が無視するものは数えない。yomiyasu の記録の `.yomiyasu/` は、yomiyasu がフォルダを作るときに中に置く `.gitignore` で外れる）。`pass` を期待する記録は、`false` のものだけが証拠になる。`fail` を期待する記録は、誤りを戻した木で撮るので `true` でよい（決めたこと 11） |
| `started_at`・`finished_at` | 必須 | 始めた時刻と終わった時刻（UTC） |
| `output` | 必須 | 出力の置き場。`library/` からの相対で、`runs/<id>.log` |
| `output_bytes` | 必須 | command が書いた出力の全体のバイト数（切る前） |
| `truncated` | 必須 | 上限を超えて先頭を切ったか |

- 記録は担当が手でも書けるので、強さは `report.json` と同じ自己申告（決めたこと 2 の「受け入れること」）。減るのは「走らせずに pass と書く」ことで、出力が残るので Chief of Staff・オーナー・振り返りが後から読める。
- `andon issue ready` と gate は、`acceptance[]` の行のうち `run` のあるものだけ、指す記録を確かめる（決めたこと 2・11。`internal/contract` の `CheckRuns`）。断るのは3通り: 記録か出力（`runs/<n>.log`）が無い・読めない／終了コードが記録の `expect` と違う／`pass` を期待する記録が `dirty: true`。`fail` を期待する記録は `dirty: true` でよい。`andon issue ready` は終了コード 1 と、行ごとの理由を1行にまとめて返し、`review` にしない。gate は `fix` を返す（§7）。
- 記録の `commit` が head と違う（レビューや CI の直しで head が進んだ）のは断らない。gate が記録の `acceptance.stale_runs` に出すだけ（§7）。`run` の無い行（CI の検査の名前・画面の添付の URL・`manual`）は今までどおり通る。

## 25. オーナー待ち（`andon waiting --json` の出力）

`andon waiting [--json] [--project <slug>]` が、オーナーが動くまで先へ進まないもの（オーナー待ち）と並行の枠を数えて出す（[ADR-0042](adr/0042-owner-waits-do-not-stop-the-line.md) 決めたこと 8・9）。**読むだけで、何も書かない・保存しない**。置き場は無い（標準出力だけ）。数える式はこの command だけが持ち、Chief of Staff が枠を数えるときも、問いを出す順を決めるときも、オーナーのペインが一覧を描くときも、この出力を使う（決めたこと 14）。`--json` を付けない出力は同じ中身を人が読む行で出し、機械は読まない。

読むもの: kanban の `config.yaml`（`columns`・`timezone`・`chief_of_staff`）・`issues/<id>/issue.yaml`・`questions.yaml`・`log.jsonl`（課題が最後に `review` へ動いた行と、`merge` を最後に書いた行の `by`）と、課題の `library/` の `report.json`（`commit`・`pr`）・gate の記録（今の head の `library/gate/<head>.json`）・問いの JSON（`library/questions/<n>.json` の `from`・`facts`・`kind`）と、`project.json` の `chief_of_staff.parallel`（作者の照合は通さずファイルを読む。読めない・形が違えば上限 2 で数え、理由を標準エラーに1行出す）。GitHub は読まず、git も呼ばない。kanban の CLI は数えるのには使わず、`--project` も `KANBAN_ROOT` も無いときに kanban の根を聞く（`kanban root`）だけである。

```json
{"version": 1, "at": "2026-11-21T10:00:00Z", "project": "sample-site",
 "slots": {"limit": 4, "working": 3, "waiting": 2, "free": 1, "waiting_full": false},
 "items": [
   {"kind": "merge", "id": "S-0021", "title": "名指しの口を足す", "since": "2026-11-21T04:43:35Z",
    "question": "S-0021#1", "pr": "https://github.com/example/sample-site/pull/41",
    "reason": "delegation: merge=B < tier C", "holds": ["S-0022", "S-0023"]},
   {"kind": "decision", "id": "S-0030", "title": "書き出しの形を決める", "since": "2026-11-21T02:12:13Z",
    "question": "S-0030#1", "ask": {"kind": "users", "question": "書き出しは CSV だけにするか", "facts": ["依頼書の「やること」"],
     "options": [{"label": "CSV だけ", "pick": true}, {"label": "Excel も"}]}, "answerable": true, "holds": []},
   {"kind": "task", "id": "S-0031", "title": "image を焼き直す", "since": "2026-11-21T08:55:43Z",
    "reason": "ホストで image を焼き直す", "holds": []}]}
```

オーナー待ちは 3 種で、今の head について決め終えたものだけを数える。`done`（`columns` の最後の列）の課題は数えない。

| `kind` | 数えるもの | `since` |
|---|---|---|
| `merge`（マージの依頼） | 課題が `review` にいて、次のどれか。1 つ目は、今の head（`report.json` の `commit`）について gate が出した問い（問いの JSON の `from: gate`）が `open` のまま。どの head の問いかは、事実の gate の記録の行から読む（読めない問いは今の head の問いとして数える）。2 つ目は、今の head の gate の記録が、課題が最後に `review` へ動いたあとに書かれて `approve` で、課題が `merge: auto` でないか、`merge: auto` を最後に書いたのが Chief of Staff の端末でない（§2 の履歴の照合）。3 つ目は、Chief Engineer の課題（`profile: chief-engineer`）で `report.json` に `pr` がある。今の head の gate の記録が `merged: true` の課題は数えない。課題1件に1行 | 1 つ目は問いの `asked`、2 つ目は gate の記録の `at`、3 つ目は最後に `review` へ動いた時刻 |
| `decision`（判断） | 2 つある。1 つ目は、灯りの種類（`business`・`users`・`promise`・`irreversible`・`line`）の問いが `open` で、出したのが gate でない。問い1件に1行。2 つ目は、オーナーの課題（`who: person`）で、起動できる（`todo` か `queued`）か `doing` にいて、灯りの種類の問いを1つも持たず（`scope` と種類の無い問いは数えない）、印 `only_owner`（§2）も無い。課題1件に1行で、`question` と `ask` を持たず、`answerable` は `false`、`reason` に直す 1 手を書く。どちらも、オーナーの課題は `after` の相手が全部 `done` になるまで出さない。`not_before` の日付では隠さない | 1 つ目は問いの `asked`、2 つ目は課題の `created` |
| `task`（手作業） | 印 `only_owner` を持つオーナーの課題で、起動できる（`todo` か `queued` で、`after` の相手が全部 `done`、`not_before` の日を過ぎた）か `doing` にいて、`open` か `answered` の問いを持たない | 課題の `created` |

- オーナーの課題は、問いを持つ判断か、印 `only_owner` を持つ手作業のどちらかの形で作る。どちらでもない課題は作り方の誤りで、判断の 2 つ目の行にして灯す。灯りの種類の問いが全部受け取られたオーナーの課題は、印が無ければどの行にも出さない（片付けるのは Chief of Staff）。人が読む出力は、判断の 2 つ目の行に、直す 1 手を 2 つ並べて出す（判断なら `andon issue ask <id> -`、手作業なら `kanban set <id> only_owner="<理由>"`。[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 決めたこと 2・4・5）。
- `answered` の問い（答えが書かれて担当がまだ受け取っていない）と、`scope` の問い（Chief of Staff が答える）は数えない。`doing` の課題は、gate の問いが開いていてもマージの依頼に数えない。
- **Chief of Staff が止めた課題**（§2 の `hold`。[ADR-0056](adr/0056-gate-required-before-merge.md) 決めたこと 11）: どの形（gate の問い・approve・Chief Engineer の文書の PR）でもマージの依頼に出さない。止めた理由の課題の行があれば、その `holds` に止めた課題（とその後ろで `after` で待つもの）を数える。枠では、担当の課題ならオーナー待ち（`slots.waiting`）に数える。値が空でも、最後に消したのがこの課題の担当かそのレビュアーの端末なら止めているものとして数える。止めの読み方は gate と同じ（`internal/ledger` の `HoldOf`）。オーナーが理由の課題の問いに答えたあと（答えを担当が受け取り、Chief of Staff が `andon issue unhold` で外すまで）は、止めた課題はどの行にも出ず、枠ではオーナー待ちのまま数える（[ADR-0057](adr/0057-owner-issue-decision-or-hand-work.md) 事実 19 の、答えから外すまでの間の出し方）。理由の課題が done になれば、dispatcher が通知 `hold`（§6）で Chief of Staff に外し忘れを知らせる。
- 委譲は読み直さない。approve のあと dispatcher が自動でマージする途中の課題は数えない。委譲が gate のあとに変わっていれば、dispatcher がマージの直前に断って gate の問いを出すので、次の読みで 1 つ目として数えられる。オーナーがマージしてから dispatcher が片付けるまでの 1 分ほどは一覧に残る。

| キー | 意味 |
|---|---|
| `version` | 出力の形のバージョン。今は 1 |
| `at` | 数えた時刻（UTC） |
| `project` | プロジェクトの slug（kanban の根のディレクトリの名前） |
| `slots.limit` | `project.json` の `chief_of_staff.parallel`。無ければ 2 |
| `slots.waiting` | `doing` か `review` の担当の課題（レビュアーの子課題〈`<id>-r<k>`・`profile` が `-review` で終わる〉・Chief Engineer の課題・オーナーの課題を除く）のうち、`items` に `merge` か `decision` の行があるものか、Chief of Staff が止めているものの数 |
| `slots.working` | 同じ課題のうち、残りの数 |
| `slots.waiting_full` | `waiting` が `limit` 以上のとき `true`。新しい課題を起動しない |
| `slots.free` | `waiting_full` なら 0。そうでなければ `limit` から `working` を引いた数で、0 より小さくしない |
| `items[]` | オーナー待ちの行。`holds` の数の多い順、同じなら `since` の古い順。Chief Engineer の文書の PR とオーナーの課題は、行には出るが枠には数えない |
| `items[].question` | `merge` の 1 つ目は gate の問い、`decision` の 1 つ目はその問い（`<id>#<n>`）。`decision` の 2 つ目は持たない |
| `items[].pr` | `report.json` の `pr`（`merge` の行で、あれば） |
| `items[].reason` | 人が読む理由。`merge` の 1 つ目は gate の記録の `reasons`（無ければ問いの文）、2 つ目と 3 つ目は待つわけの1文。`decision` の 2 つ目は問いが置かれていないことと直す 1 手。`task` は印 `only_owner` の値の 1 行目 |
| `items[].ask` | `decision` の行の問いの JSON（§5）の写し。問いの JSON が無い・形が違う問いは `ask` を持たない |
| `items[].answerable` | `decision` の行だけ。オーナーのペインから答えられるか（`ask` があり、種類が `irreversible` でない。2 つ目の行は `false`）。答える口は下の `andon waiting answer`（決めたこと 17） |
| `items[].holds` | その課題を `after` で待っている済んでいない課題と、その課題を理由に止めた課題（§2 の `hold`）の id（直接のものと、その後ろで待つもの全部。id の順） |

- 書き手は `andon waiting` だけ（出力するだけ）。読み手は Chief of Staff とオーナーのペイン。gate も dispatcher も判定に使わない。
- **答える口** `andon waiting answer <id>#<n> --option <番号> [--project <slug>]`: オーナーのペインが、判断の行（`answerable: true`）の問いに、問いの JSON の選択肢の1つで答える（決めたこと 17・18）。番号は `ask.options` の並びの 1 から。書くのは `kanban answer <id>#<n> --decided-by owner -- "<選んだ選択肢の label>"` の1つだけで、答えの文はその `label`、印は `owner`。次のどれかなら何も書かずに終了コード 1 で断る: herdr の端末の中で打たれた（`HC_TERMINAL_ID` がある。担当も Chief of Staff も端末の中で動き、App の `exec` は端末の外で動く）・問いが `open` でない・問いの JSON が無いか読めない・`from: gate`・種類が `irreversible`・種類が判断（`business`・`users`・`promise`・`line`）でない・番号が選択肢の外。使い方の誤りは終了コード 2。`kanban answer` も `open` の問いだけを通すので、読んでから書くまでに答えが入っても、答えは1つだけになる。書いた答えの行は書き手が端末でないので、dispatcher が通知 `answered`（§6）で Chief of Staff に届ける。
- kanban の `config.yaml` が読めないとき、`log.jsonl` が読めない（課題があるのに無い）とき、`--project` の kanban が無いときは終了コード 1。履歴が無いと、課題が `review` に入った時刻と `merge` の書き手が分からず、マージの依頼を動いている仕事に数えてしまうため、数えずに止まる。読めない `issue.yaml` は数えずに理由を標準エラーに出し、終了コード 0。

## 26. ダイジェストの入力（`library/digest/input.json`）

`andon digest collect <id>` が、ダイジェストの課題 `<id>` の期間の記録を集めて書く（[ADR-0044](adr/0044-owner-digest.md) 決めたこと 3）。**書くのはこのファイルだけ**で、kanban・記録・GitHub は読むだけ。書く前に schema に照らし、合わなければ書かずに終了コード 1。読めなかった記録は止まらずに `skipped` に1件ずつ並べ、標準エラーにも1行ずつ出す（黙って数えない所を作らない）。

期間（決めたこと 5）: 同じルーチン（`issue.yaml` の `routine`）が足したダイジェストの課題のうち、`sent.json`（§28）の送り先が全部済んでいるもので `until` がいちばん新しいものの `until` から、今まで。無ければ 24 時間前から。7 日より前へは遡らず、切ったときは `truncated: true`。組み立てただけ・送りそこねたダイジェストは数えない。

読むもの: kanban の `config.yaml`（`columns`・`timezone`）・`issues/<id>/issue.yaml`・`log.jsonl`（期間に `done` へ動いた行）・`routines/*.yaml`（`schedule` と `paused`）、課題の `library/` の `report.json`（`pr`・`review_request.extra`）・`outcome.json`（§19）・gate の記録（`library/gate/<head>.json` の `decision`・`delegation.grant`）・`digest/sent.json`、dispatcher の `<slug>.delivered.jsonl`（§17）と `<slug>.signals.json`（§15）、`andon waiting` と同じ数え方（§25）、`andon measure` と同じ数え方（§13。直近 7 日とその前の 7 日）、GitHub のマージされた PR の一覧（`gh pr list`。repo ごとに1回）。代わりに決めたこと（`decided`）のために、`issues/<id>/questions.yaml`・`library/questions/<n>.json`・`library/retro-applied.json`（§21）・kanban の `library/triage/*.json` と、課題の `repo` の最新の main も読む。main は checkout から object を借りた一時の bare の写しに origin から取り直す（checkout には何も書かず、写しは読み終えたら消す）。取り直せない repo は古い木で数えずに `skipped` に出す（次のダイジェストが同じ所から読む）。前に送ったダイジェストの `library/digest/input.json` の `decided.mains` と `skipped` も読む。

```json
{"version": 1, "project": "sample-site", "issue": "S-0100", "title": "デイリーダイジェスト",
 "at": "2026-11-21T10:00:00Z", "since": "2026-11-20T10:00:00Z", "until": "2026-11-21T10:00:00Z",
 "previous": {"issue": "S-0090", "until": "2026-11-20T10:00:00Z", "sent_at": "2026-11-20T10:05:00Z"}, "truncated": false,
 "issues": [
   {"id": "S-0001", "title": "返事の下書きボタン", "status": "done", "done_at": "2026-11-21T08:01:00Z",
    "pr": "https://github.com/example/sample-site/pull/1", "merged_at": "2026-11-21T08:00:00Z", "merged_by": "dispatcher",
    "tier": "B", "grant": "d2", "retro": false},
   {"id": "S-0002", "title": "一覧の並び", "status": "done", "done_at": "2026-11-21T09:10:00Z",
    "pr": "https://github.com/example/sample-site/pull/2", "merged_at": "2026-11-21T09:00:00Z", "merged_by": "person",
    "tier": "C", "grant": null, "retro": false}],
 "waiting": {"version": 1, "at": "2026-11-21T10:00:00Z", "project": "sample-site",
  "slots": {"limit": 2, "working": 1, "waiting": 0, "free": 1, "waiting_full": false}, "items": []},
 "concerns": [{"kind": "gate_fix", "issue": "S-0003", "title": "検索の速さ", "at": "2026-11-21T07:00:00Z", "url": null}],
 "numbers": {"days": [{"date": "2026-11-21", "merged": 2, "auto": 1}],
  "current": {"since": "2026-11-15", "until": "2026-11-21", "merged": 9, "auto": 5, "closed": 12, "lead_time_hours": 3.5, "question_open_hours": 1.2, "tokens": 1200000},
  "previous": {"since": "2026-11-08", "until": "2026-11-14", "merged": 7, "auto": 3, "closed": 10, "lead_time_hours": null, "question_open_hours": null, "tokens": null}},
 "next": {"ready": 3, "observations": [{"id": "S-0040", "title": "観察: 下書きボタン", "not_before": "2026-11-25"}],
  "routines": [{"name": "daily-digest", "title": "デイリーダイジェスト", "next": "2026-11-22T10:00:00Z"}]},
 "decided": {
  "decisions": [{"text": "2026-11-21 一覧を新しい順にする。", "mark": "Chief Engineer が決めた", "repo": "github.com/example/sample-site",
    "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "path": "docs/adr/0003-list.md", "line": 42,
    "url": "https://github.com/example/sample-site/blob/9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08/docs/adr/0003-list.md?plain=1#L42"}],
  "triage": [{"issue": "S-0050", "title": "トリアージ", "at": "2026-11-21T06:00:00Z", "closed": 12, "cut": 2, "path": "kanban/library/triage/2026-11-21-S-0050.json"}],
  "retro": [{"issue": "S-0051", "title": "振り返り", "at": "2026-11-21T07:00:00Z", "fix": 1, "dropped": 4, "path": "kanban/issues/S-0051/library/retro-applied.json"}],
  "answers": [{"issue": "S-0052", "title": "誤字を直す", "n": 1, "question": "どれを直すか", "answer": "c1 だけ", "at": "2026-11-21T08:00:00Z", "path": "kanban/issues/S-0052/questions.yaml"}],
  "mains": [{"repo": "github.com/example/sample-site", "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"}]},
 "redacted": 0, "skipped": []}
```

| キー | 意味 |
|---|---|
| `version` | 形のバージョン。今は 1 |
| `issue`・`title` | ダイジェストの課題とその題（題が空なら id） |
| `at`・`since`・`until` | 集めた時刻と期間（UTC。`until` は `at` と同じ） |
| `previous` | 期間の始まりにした前のダイジェスト（`sent_at` は送り先が最後に済んだ時刻）。無ければ `null` |
| `issues[]` | 期間に `done` へ動いたか、`outcome.json` の `merged_at` が期間の中の課題。レビュアーの子課題（`<id>-r<k>`・`profile` が `-review` で終わる）、このダイジェストの課題、同じルーチンのダイジェストは数えない。id の順 |
| `issues[].merged_by` | `outcome.json` の `result` が `merged` で、`merged_at` か `done_at` が期間の中の課題だけ（結果の記録は `done` のあとに書かれるので、前の期間にマージして今の期間に片付いた課題は前のダイジェストに載っていない）。`gates[]` に `merged: true` があれば `dispatcher`、無ければ `person`。そうでなければ `null`。PR のある課題が期間に片付いたのに `outcome.json` がまだ無いときは、マージを数えずに `skipped` に出す |
| `issues[].tier`・`grant` | `tier` はマージした gate（無ければ最後の gate）のレベル。`grant` はマージした head の gate の記録の `delegation.grant`（名指しの委譲でマージしたとき） |
| `issues[].retro` | 親の課題が振り返りのルーチン（`routine: retro`）の課題か |
| `waiting` | 集めた時刻の `andon waiting --json`（§25）と同じもの。build は §25 の schema にも照らす |
| `concerns[]` | 期間のうまくいっていないこと。下の表の 6 種。同じ種類と課題は一番新しい1件にし、時刻の順 |
| `numbers` | `days` は直近 7 日（UTC の日）のマージした PR と、そのうち自動の数。`current`・`previous` は直近 7 日とその前の 7 日の `andon measure` の数（`tokens` は入力・出力・キャッシュの合計。読めなければ `null`） |
| `next.ready` | `todo` の担当の課題（レビュアーとオーナーの課題を除く）のうち、`after` の相手が全部 `done` で `not_before` の日を過ぎたものの数 |
| `next.observations[]` | 親が Chief Engineer の課題で、済んでおらず、`not_before` が 7 日以内の課題（観察の課題） |
| `next.routines[]` | kanban のルーチン。`next` は次に足す時刻で、止めてある・読めない `schedule` は `null` |
| `decided` | 期間にオーナーの代わりに決められたこと（ADR-0044 決めたこと 6 の節 5）。下の表の4つと、次のダイジェストのための `mains`。collect はいつも書く。前のバージョンの入力には無く、build は「入っていない」と描く（「無かった」とは描かない） |
| `redacted` | 秘密の形を伏せた数（§20 と同じ伏せ方。一覧の正は `internal/redact/redact.go`） |
| `skipped[]` | 読めなかったもの（`source` は kanban の根からのパスか出所の名前、`reason` は1行） |

| `concerns[].kind` | 数えるもの |
|---|---|
| `stalled` | `delivered.jsonl` で期間に届けた `stalled`・`error` の通知（止まった担当） |
| `gate_fix` | 期間に書かれた gate の記録で `decision: fix` のもの |
| `review_over` | 済んでいない課題で、`report.json` の `review_request.extra` が `true`（レビューの上限を超えた）。時刻はその課題の期間の最後の履歴 |
| `ci_failed` | `signals.json` の `github-ci` の信号で `active: true` のもの（main の CI の失敗）。`url` は信号の URL |
| `revert` | 期間にマージされた PR で、題が `Revert ` で始まるもの。`url` は PR |
| `estimate` | 期間に `done` へ動いた課題で、`outcome.json` の `worked_minutes` が見込み（§2 の `estimate`）の 2 倍を超えたもの |

| `decided` のキー | 集めるもの |
|---|---|
| `decisions[]` | 期間の前と期間の終わりの main（first-parent）の `docs/` の下の `.md` を比べて、増えた決定の行。期間の前の main は前のダイジェストの `mains`、終わりは取り直した main の先（fast-forward の push は commit の時刻が古いまま入るので、時刻だけでは切らない）。`mains` が無い・その commit が main に無い・期間を7日で切ったときは committer の時刻で切る。決定の行は、見出しが「決めたこと」で始まる節（下の小見出しを含む。コードの囲みは除く）の番号つきの行で、頭の太字の 1 文の中か、閉じの `**` のすぐ後ろの全角の括弧に「Chief Engineer が」を含むもの。`mark` は括弧の中（書いてあるとおり）、`text` は太字の 1 文からその括弧を除いたもの。期間の前の変わった文書に、同じ `text` の行か、残りのうち同じ文書・番号・日付（1 文か括弧の中の最初の日付）の行があれば、増えた行に数えない（語・本文・括弧の中だけの直しと、文書の移しを新しい決定として出さない。同じ番号と日付のまま中身を決め直した行も出ない）。期間の中で足して直した行は最後の形の1行で、足して取り消した行は出ない。`commit` は期間の終わりの main で、`path`・`line`・`url` はその木のもの（`url` は GitHub でなければ `null`）。repo・文書・行の順 |
| `triage[]` | 期間に `done` へ動いたトリアージの課題の記録（`library/triage/<日付>-<id>.json`）。記録に当てた時刻が無いので、当てた直後の片付けの時刻で切る。`closed` は `close` の項目の数、`cut` は `fix`・`bundle` から足した課題の数（束ねたものは 1。オーナーの課題は数えない） |
| `retro[]` | `applied_at` が期間の中で `complete: true` の振り返りの記録。`fix` は `issues` の数、`dropped` は `dropped` の数。当て終えていない記録は `skipped` に出す。`applied_at` は当て始めの時刻なので、前のダイジェストの `skipped` にある記録は期間の前の時刻でも数える |
| `answers[]` | `answered` が期間の中の `kind: scope` の問い。`answeredBy` が `terminal:<id>` のものだけで、`terminal:dispatcher` と端末の外（オーナーの shell）の答えは載せない（オーナーが打った文をセルの外へ出さない）。`question` は問いの JSON（§5）の1行で、無ければ kanban の `text` の頭の行 |
| `mains[]` | repo ごとの読んだ main の commit。読めなかった repo は前のダイジェストの値のまま（前にも無ければ項目が無い） |

## 27. ダイジェストの中身（`library/digest/digest.json`）

ダイジェストの担当が書く文（要旨・テーマ・気になること）。担当は標準入力で `andon digest build <id> -` に渡し、build が照らしてから `digest.json` に写す（決めたこと 3・8）。数・一覧・図の位置と色は build が §26 の入力から描き、担当は書かない。

```json
{"headline": ["返事の下書きボタンが入り、返信の手間が減った。", "オーナーの判断を待っているものは無い。"],
 "themes": [
   {"title": "返信を速くする", "body": ["問い合わせの画面に下書きのボタンを足した。"], "issues": ["S-0001"],
    "figure": {"type": "before_after", "before": ["毎回一から書く"], "after": ["下書きを直して送る"]}},
   {"title": "一覧", "body": ["並びを新しい順にした。"], "issues": ["S-0002"],
    "figure": {"type": "steps", "steps": ["開く", "並べ替える", "選ぶ"], "focus": 2}}],
 "concerns": [{"text": "検索の速さで gate の差し戻しが続いた。", "issues": ["S-0001"]}]}
```

| キー | 決まり |
|---|---|
| `headline[]` | 要旨。2〜5 文。1 文は1行（改行を含まない） |
| `themes[]` | テーマ。8 つまで。`body` は 1〜3 文、`issues` は 1 件以上で、§26 の `issues[]` にある課題だけ |
| `themes[].figure` | 図（任意。決めたこと 9）。`before_after` は `before`・`after` がそれぞれ 1〜3 行。`steps` は 2〜6 段で、`focus` は目立たせる段（1 から） |
| `concerns[]` | 担当が添える気になること。3 つまで。`issues` は 1 件以上で、§26 の `issues[]`・`concerns[].issue`・`waiting.items[].id` にある課題だけ（止まっている課題やオーナーを待っている課題も根拠に指せる） |

- build は次のどれかなら理由を1行ずつ標準エラーに出し、終了コード 1 で何も書かない: schema に合わない（知らない図の型を含む）・入力に無い課題を指す（テーマは `issues[]` だけ、気になることは上の 3 つ）・要旨が 2〜5 文でない・テーマが 9 つ以上・テーマの文が 4 つ以上・気になることが 4 つ以上・図の行や段が多い・`focus` が段の数を超える。
- どのテーマにも入らないマージ済みの課題は、build が「そのほか」に題で並べる（黙って落とさない）。
- 組み立てる前に、入力と担当の文の文字列から秘密の形を伏せる（決めたこと 17）。
- build は `digest.json`・`digest.html`（下）・`message.txt`（下）の3つを書く。
  - `digest.html`: 依存の無い単一の HTML（決めたこと 7）。インラインの SVG の図を持ち、`<script>`・`<link>`・`<a>` のほかで外を指す `src`・`href` を持たず、スタイル（`<style>` の中と `style` 属性）に `url(` を持たない。`prefers-color-scheme: dark` で暗い配色に変わり、スマホの幅で読める。色と大きさはクラスで決め、値をスタイルに渡さない。琥珀はマージの依頼と判断の行と冒頭の「オーナーが呼ばれている」数だけ、赤は止まった担当・main の CI の失敗・revert の行だけ（決めたこと 10）。1 MiB より小さい。build は書く前にこれを確かめ、通らなければ書かない。
  - `message.txt`: Slack などに添える短い文（決めたこと 13）。題・プロジェクト・期間・要旨・オーナーを待っているものの数と上から 5 件（題と PR の URL）・マージした数と自動の数・files/ の URL。装飾の記法を使わず、3000 字まで。超えたら待っているものの行を下から削り、削った数を出す。

## 28. ダイジェストを送った結果（`library/digest/sent.json`）

`andon digest send <id>` が、送り先ごとの結果を書く（決めたこと 15・16）。送り先は、いつも出す `files` と、`project.json` の `digest` に書かれた `slack`（§11）。`digest.html` を `files/<日付>-digest-<project>-<id>.html` に出し（`andon publish` と同じ置き場と URL。日付は期間の終わりの日）、URL を標準出力に出す。そのあと Slack へ、短い文（`files/` の URL を含む。3,000 字まで）を添えたファイル `digest-<project>-<日付>.html` を 1 回の投稿で送る（決めたこと 13）。

```json
{"issue": "S-0100", "since": "2026-11-20T10:00:00Z", "until": "2026-11-21T10:00:00Z",
 "destinations": [{"kind": "files", "at": "2026-11-21T10:01:00Z", "ok": true, "url": "http://files.cell.localhost:1355/2026-11-21-digest-sample-site-S-0100.html"},
  {"kind": "slack", "at": "2026-11-21T10:01:00Z", "ok": true, "channel": "C0123456789", "file": "F0123456789"}]}
```

| キー | 意味 |
|---|---|
| `issue`・`since`・`until` | 送ったダイジェストの課題と期間（§26 の写し） |
| `destinations[]` | 送り先ごとの結果。送り先 1 つに 1 行 |
| `destinations[].kind` | `files` か `slack`。kind ごとに1行 |
| `destinations[].at` | その回の send を打った時刻 |
| `destinations[].ok` | 送れたか。`true` なら `files` は `url`、`slack` は `file`（Slack のファイルの id）、`false` なら `error`（1行の理由）を持つ |
| `destinations[].channel` | `slack` の行だけ。送った、または送ろうとしたチャンネル |

- 済んだ送り先（`ok: true`）には、打ち直しても送らない（まだの Slack へ送る回だけは、`files/` の同じ名前に同じ 1 枚を置き直す。組み立て直していても、短い文の URL と Slack のファイルが同じ中身になる）。送ったあとに設定のチャンネルが変わっても、その 1 枚は送り直さない。送りそこねた送り先は打ち直しで送り直す。そのときの送り先は打ち直した時点の設定で、設定から消えていれば行を落とす。
- 送る前に、まだの送り先を `ok: false` で書き、1つ送るごとに書き直す。途中で落ちても済んだことにならない。Slack に届いたあと、書く前に落ちたときは、打ち直しで同じものがもう一度届く（届くのは 1 度か 2 度で、落ちることは無い）。同時に打った send は1つずつ走り（`sent.json.lock`）、入力と HTML は鍵を取ってから読む。
- `project.json` か `digest` が無い・空なら `files` だけに出す。信じられない・形が違う・git が HEAD を読めない（無いと言い切れない）ときも `files` だけに出して理由を標準エラーに1行出す。ただし前に送りそこねた Slack の行があれば残して終了コード 1 にする（届いていない期間を送ったことにしない）。
- `files` に出せなければ Slack へ送らない。Slack へ送れなければ、`files` の URL を出したうえで終了コード 1。
- 送る中身は組み立てたダイジェストだけ。send は `input.json` と `digest.json` から組み立て直し、`digest.html` と同じでなければ送らない。短い文も同じ JSON から作り、`message.txt` は読まない（`message.txt` は控え）。
- `andon digest check` は、`digest` の送り先へ確かめの小さなテキストのファイルを send と同じ 3 回の呼び出しで送る。`sent.json` は書かない。設定が無い・読めなければ送らずに終了コード 1。
- 集め直して期間が変わったときは、前の結果を捨てて送り直す。
- 次のダイジェストの期間は、送り先が全部済んだ `sent.json` の `until` から始まる（§26）。
- build は `digest.html` に組み立てた入力の期間の印（`<meta name="andon-digest-period">`）を入れる。send は今の `input.json` の期間と照らし、違えば（組み立てたあとに集め直した）送らずに終了コード 1 で、組み立て直すよう言う。古い HTML を新しい期間として送ると、間の出来事がどの 1 枚にも載らないため。
