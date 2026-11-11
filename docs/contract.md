# Andon の機械向けの約束

- 状態: 設計の提案（2026-11-04 起草、2026-11-05 書き直し、2026-11-05 schema に合わせて直した、2026-11-07 [ADR-0006](adr/0006-survey-fix-decide.md) の候補・`deliverable`・問いの `scope` を足した、2026-11-09 人の手直し `corrections.json`〈§14〉を足した、2026-11-10 [ADR-0011](adr/0011-signals-and-value-loop.md) の信号の記録〈§15〉・リリースの名前〈§16〉・`project.json` の `signals`〈§11〉・通知 `signal`〈§6〉を足した）。**正は `schema/`**（JSON Schema。draft 2020-12）で、この文書はそれを人が読めるように説明したもの。食い違えば schema が正。この文書の ```` ```json ```` の例は全部、`tests/schema.sh` がその節の schema に通す。

| 節 | schema |
|---|---|
| §2 台帳の項目 | `schema/issue-keys.schema.json` |
| §3 指摘 | `schema/findings.schema.json` |
| §4 報告の機械向けの部分 | `schema/report.schema.json` |
| §5 問い | `schema/question.schema.json` |
| §6 通知 | `schema/notification.schema.json` |
| §7 ゲートの条件表と記録 | `schema/gate.schema.json`（条件表か記録のどちらか一方に合う） |
| §11 プロジェクトの事実と委譲 | `schema/project.schema.json` |
| §12 候補の一覧 | `schema/candidates.schema.json` |
| §13 流れの数え（`andon measure --json` の出力） | `schema/measure.schema.json` |
| §14 人の手直し | `schema/corrections.schema.json` |
| §15 信号の記録 | `schema/signal.schema.json` |

`schema/process/` の2本（`workstream`・`state`）は、エージェントと機械の間の約束ではなく、人が書く流れと状態の定義（`process/` の YAML）の形。読み方は [process/README.md](process/README.md)、検査は `tests/process.sh`（[ADR-0017](adr/0017-process-definitions.md)）。

- 読み手: command（`andon issue …`・`andon gate`・`andon triage`・`andon delegate`・`andon measure`）・dispatcher（`andon dispatcher`）・フック・ボード（人の Interface）・振り返り。語は [design.md](design.md) §0.1。
- 台帳の CLI をこの文書では `kanban` と書く（[design.md](design.md) §3.5。仮設の工場では `deck`。同じ command）。
- 台帳の形式（ledger-format v2。別プロジェクトの持ち物）は**変えない**。Andon のキーは「Any other key is allowed and kept」の上に置く。形式そのものに触れたいことは design.md §3.2 の相談に並べた。台帳の CLI と形式を Andon に移したら（§3.5）、「変えない」は「形式の版で変える」になる。それまでこの文書は v2 の上に乗る。

## 0. 原則

1. **Markdown を読まない。** 依頼書・報告・レビューの報告・skill は人と AI のもの。機械が判定に使う値は、ここに並ぶ JSON／YAML のキーにだけ持つ（人の決定 2026-11-04。仮設の工場で Markdown を正規表現で読む形は端が尽きなかった）。
2. **無い・読めない・知らない値は安全側に倒す。** ゲートは `needs_human`、dispatcher は送らない／起こさない、担当の command は止まって理由を1行出す。黙って既定に落とさない（既定があるキーはこの文書に「既定」と明記する）。
3. **1つの約束に1人の書き手。** `issue.yaml` の予約キーは Chief of Staff、`findings.json` はレビュアー、`report.json` と `candidates.json` は担当、問いの JSON は問うた者、通知とゲートの記録と人の手直し（`corrections.json`）と信号の記録（§15）は dispatcher、委譲と `project.json` は人。履歴は `log.jsonl` の `by`（台帳）・端末のラベル・git の author（委譲）で照合する。
4. **読み手は知らないキーを無視し、書き手は知らないキーを保つ**（ledger-format の互換の規則と同じ）。キーを増やすのは版を上げない。キーを消す・意味を変えるのは版を上げ、`schema/` の `$id` と `docs/decisions.md` に書く。
5. **置き場は台帳の中。** 担当が worktree の外に書いてよい唯一の場所 `issues/<id>/library/` を、機械向けの置き場にも使う。作業側が別のマシンのときは `<worktree>.issue/outbox/` に書き、dispatcher が引いて同じ場所に置く（design.md §1.5）。`project.json`（委譲を含む）だけはセルの repo（§11。書き手が人だから）。
6. **形は小さく、テストで固定する。** キーは名前を決めたものだけ（余分なキーは書かない。読み手は無視する）。schema は書き手の約束なので、知らないキーを拒む（`additionalProperties: false`。台帳の項目 §2 と委譲の値を除く）。`tests/schema.sh` が例と schema を突き合わせる。command（`andon issue`・`andon gate`・`andon delegate`）の出力も `tests/schema.sh` が schema に通す（`tests/andon-issue.sh`・`tests/andon-gate.sh` に書かせる）。

## 1. 置き場と書き手

```
projects/<slug>/kanban/
  config.yaml                     chief_of_staff（kanban config set。Chief of Staff の端末）
  issues/<id>/
    issue.yaml                     台帳の項目 ＋ Andon の予約キー（§2。Chief of Staff が kanban set）
    brief.md                      人・AI 用（Settings は写し。機械は読まない）
    report.md                     人・AI 用（機械は読まない）
    questions.yaml                台帳の問い（§5。text は人向け）
    library/
      report.json                 担当（§4）
      findings.json               レビュアー（§3。レビュアーの課題の側）
      candidates.json             担当（§12。調べる課題の成果と、範囲の問いで止まる直す課題）
      questions/<n>.json          問うた者（§5）
      gate/<head sha>.json        dispatcher（§7。ゲートの記録）
      corrections.json            dispatcher（§14。finish〈done〉のとき1回だけ。人の手直し: 問いの答え・PR の上の直し・指摘の覆し。design.md §1.9 の (b)）
  log.jsonl                       台帳の CLI が書く。by で履歴を照合
projects/<slug>/project.json      人（§11。書いてよい repo・委譲など。セルの repo。git の履歴が照合の元）
~/.local/state/andon/dispatcher/<slug>.json   dispatcher の控え（§6。台帳の外）
~/.local/state/andon/dispatcher/<slug>.signals.json   dispatcher（§15。外の信号の記録。台帳の外）
~/.local/state/andon/dispatcher/<slug>.recorded.jsonl   dispatcher（§6。記録だけの通知。1行1件・追記だけ。台帳の外）
```

## 2. 台帳の項目（`issue.yaml` の Andon の予約キー）

依頼書の `## Settings` の**正**。Chief of Staff が `kanban set <id> review=on must_fix=major …` で書く。依頼書の欄は人が読む写し。

| キー | 値 | 無いとき | 書き手 | 読み手 | 意味 |
|---|---|---|---|---|---|
| `review` | `on` / `off` | ゲートは `needs_human`（レビューの照合を回さない）。dispatcher は「レビューを頼まれても起こさない」 | Chief of Staff | ゲート・dispatcher・担当 | 独立レビューを工程に入れるか |
| `must_fix` | `critical` / `major` / `minor` / `trivial` | ゲートは `needs_human` | Chief of Staff | ゲート・担当 | この重さ以上の指摘は担当が直す |
| `reviewer` | プロファイルの名前（`codex-review` / `claude-review`） | 既定: 実装のプロファイルと別ブランド（Claude → `codex-review`、Codex → `claude-review`） | Chief of Staff | dispatcher（起こす）・ゲート（別ブランドの照合） | レビュアーのプロファイル |
| `review_rounds` | 1 以上の整数（を表す文字列。下の注） | 既定: `"2"` | Chief of Staff | dispatcher・担当 | 頼める回数の上限。超えた回は `report.json` の `review_request.extra` で頼み、起こすかは dispatcher ではなく Chief of Staff が決める |
| `merge` | `auto` / `person` | 既定: `person`（安全側） | Chief of Staff | dispatcher・ゲート | `auto`: ゲートが approve し、委譲（§11）の `merge` がそのレベルを許していれば dispatcher がマージし finish する。`person`: approve でも止まり、人に通知する。**委譲の範囲を広げることはできない**（狭めるだけ） |
| `machine` | 端末の土台の machine の名前 | 既定: 指揮側と同じマシン | Chief of Staff | `andon issue start`・dispatcher | 作業側（担当・レビュアー）を起こすマシン |
| `deliverable` | `pr` / `candidates` | 既定: `pr` | Chief of Staff（依頼書の Settings の `Deliverable:` の写し） | `andon issue ready`・dispatcher・`andon gate`・ボード・担当 | 課題の成果の形。`pr`: PR（直す課題・Chief Engineer の文書）。`candidates`: PR を出さず `library/candidates.json`（§12）が成果（調べる課題）。`andon issue ready` は `pr` の代わりに `candidates.json` が在って形が合い（repo があれば）`commit` が `HEAD` と同じことを確かめる。dispatcher は `review` の通知だけ届けてゲートを回さず、`review_request` があっても起こさない。`andon gate` を手で打てば回さずに終了コード 3（[ADR-0006](adr/0006-survey-fix-decide.md) 決めたこと 7） |
| `estimate` | `small` / `medium` / `large`（段→分は下の表） | 見込み無し（既定の段で埋めない。比べず、記録に「見込み無し」と数える） | Chief of Staff（依頼書を切るとき。依頼書の Settings の `Estimate:` の写し。段の名 小・中・大） | dispatcher・`andon measure`・担当 | 課題の大きさの見込み（目安であって上限ではない）。担当の課題（直す・調べる）だけ。レビュアー・Chief Engineer の課題には置かない（[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 2・3・8） |

`kanban show <id> --json` の `.issue`（Andon の予約キー以外は台帳の形式のキー）:

```json
{"id": "A-0012", "title": "andon issue ready を足す", "status": "doing",
 "review": "on", "must_fix": "major", "reviewer": "codex-review",
 "review_rounds": "2", "merge": "person", "machine": "vps", "deliverable": "pr",
 "estimate": "medium"}
```

`estimate` の段→分（値の意味。待ちを除いた実働の目安で、規則ではない。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 3・11 の 7）:

| 値（依頼書の Settings の段の名） | 分 |
|---|---|
| `small`（小） | 30 |
| `medium`（中） | 60 |
| `large`（大） | 120 |

- **値は全部文字列**。`kanban set` は値を文字列で書くので、`review_rounds` も `"2"`（1 以上の整数を表す数字の列）。YAML に手で `review_rounds: 2` と書いた数は schema を通らない（読み手は「無い」として既定を使う）。
- 台帳の形式は知らないキーを許して保つので、この schema は**知らないキーを拒まない**。どれも必須ではない（無いときの振る舞いは上の表）。
- **履歴の照合**: ゲートと dispatcher は、`log.jsonl` でそのキーを最後に変えた行の `by` が `terminal:<config.yaml の chief_of_staff>` で、その値が今の `issue.yaml` と同じときだけ信じる。違えば `needs_human`。`chief_of_staff` が無ければ信じない。`log.jsonl` の id は 4 桁に揃えて読む（移す前の行は `A-5` のまま。[ledger.md](ledger.md) の Writing）。
- **レビュアーの課題**（`parent` が元の課題で `profile` が実装と別ブランドの `*-review`）は、作成の行（最初の `add`）の `by` が `terminal:<chief_of_staff>` か `terminal:dispatcher`（dispatcher が起こしたもの）のときだけ、ゲートと `andon issue ready` がレビューに数える。id は `<元の課題の id>-r<回>`（`A-0005-r1`。dispatcher が `kanban add --round <回>` で付ける。2026-11-06 より前の子は通し番号のまま読む。[ledger.md](ledger.md) の Writing）。数えるのは id の形ではなく `parent` と `profile`。
- 値の綴りは小文字（`machine` を除く）。知らない値は「無い」と同じ。
- `estimate` も履歴の照合を要る（最後に変えたのが Chief of Staff でなければ「無い」）。要らないと担当が自分の課題の見込みを書き換えてズレを消せる。Chief of Staff が起き直してアドレスが変わったら、ほかの予約キーと一緒に打ち直す（打ち直さないと見込みが「無し」になる。[ADR-0014](adr/0014-issue-estimate.md) 決めたこと 11 の 5）。
- 読み手が `kanban show <id> --json` の `.issue` から取る（ファイルを直接読まない）。

## 3. 指摘（`findings.json`）

レビュアーの課題の `library/findings.json`（キーはこの名前だけ）:

```json
{"commit": "2a17f63e53ac7f9554d78782e2bfc4d079454455",
 "findings": [{"severity": "major", "file": "internal/issue/issue.go", "line": 120},
              {"severity": "major", "file": "internal/ledger/ledger.go", "line": 40, "preexisting": true}]}
```

| キー | 値 | 意味 |
|---|---|---|
| `commit` | 40桁・小文字 | 見た commit。PR の head と違えば stale |
| `findings[].severity` | `critical` / `major` / `minor` / `trivial` | 重さ。起きる条件を具体的に示せないものは `trivial` |
| `findings[].file` | repo からの相対パス（空でなく、制御文字・改行が無く、`/` で始まらず、`..` の段を含まない） | — |
| `findings[].line` | 1 以上の整数 | — |
| `findings[].preexisting` | 真偽（任意。無ければ `false`） | 課題の前からあった問題（差分の外の古い問題）。ゲートは `must_fix` に数えない。ただし `file`・`line` が PR の差分で足した行なら数える（[decisions/2026-11-09-review-preexisting.md](decisions/2026-11-09-review-preexisting.md)） |

- 無い・JSON でない・形が違う → missing。`commit` 違い → stale。報告の Markdown は読まない。
- 書き手はレビュアー。読み手: ゲート（`must_fix` 以上の件数）・担当（振り分け。`must_fix` 以上を直し、未満は `report.json` の `waived` に理由つきで残す。`preexisting: true` は振り分けない）・振り返り（繰り返し出た指摘を観点へ）。
- ゲートが `must_fix` に数えないもの（どちらも機械が記録から決める。申告でゲートを緩めない）:
  - `preexisting: true` の指摘（場所が PR の差分で足した行なら数える）。
  - **2回目以降の回**の critical でない指摘で、**前の回**の `findings.json` に同じ `file` の指摘が無いもの（新しい指摘。2回目以降は直しの確認だけ。[decisions/2026-11-09-review-later-rounds.md](decisions/2026-11-09-review-later-rounds.md)）。回は台帳から決める: ゲートが数えるレビュアーの子課題を古い順に並べ、最新の1つの前で `findings.json` が読めて `commit` が最新と違う最初のものが前の回（`andon issue review-request` の数え方と同じ）。無ければ1回目。
- `andon issue ready`・`andon issue show` の照合は `preexisting` の場所を確かめない（PR の差分を持たない下見）。正はゲート。
- 将来キーを足すなら（例: `title`・`kind`）版を上げずに足せる（原則4）。

## 4. 報告の機械向けの部分（`report.json`）

担当の課題の `library/report.json`。担当が書き直すたびに全体を書く（`report.md` と同じ）。書き手は担当。`kanban report` の直前に書く——dispatcher は `log.jsonl` の `{"report": "write"}` を合図に `report.json` を読む（新しい合図の種類を足さない）。

```json
{"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "pr": "https://github.com/<owner>/andon/pull/12",
 "review_request": {"round": 1, "limit": 2, "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "previous": null, "reviewer": "codex-review", "extra": false},
 "waived": [{"severity": "minor", "file": "internal/issue/issue.go", "line": 88, "reason": "起きる条件がこの repo に無い"}],
 "acceptance": [{"id": "A1", "result": "pass", "evidence": "bash tests/andon-issue.sh"}],
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
| `commit` | repo のある課題では必須（schema では任意。`andon issue report` が書き、`andon issue ready` が確かめる）。repo の無い課題では書かない | dispatcher・ゲート | 書いた時点の HEAD（40桁・小文字）。`andon issue ready` が `HEAD` ＝ `commit` ＝ PR の head を確かめる |
| `pr` | PR を出したら必須（schema では任意。確かめるのは `andon issue ready`） | ゲート・dispatcher | 今の報告の `PR:` 行の代わり。ゲートはこれだけを読む。`https://github.com/<owner>/<repo>/pull/<n>` で、owner と repo は `/` を除く印字できる ASCII |
| `review_request` | レビューを頼むときだけ。中の6つのキーは全部必須（`previous` は初回 `null`、`round`・`limit` は 1 以上の整数） | dispatcher | dispatcher が `kanban add --parent <id>` ＋ detached worktree ＋ `hc run <reviewer>` で起こす。`round` ≤ `limit` でなければ起こさない（`extra: true` なら Chief of Staff の判断待ち）。`previous` があれば再レビュー（差分 `previous..commit` だけ）。`commit` が PR の head と違えば起こさない |
| `waived` | 任意 | トリアージ・振り返り | 見送った指摘（`findings.json` と同じ3つ ＋ `reason`） |
| `acceptance[]` | 任意（Chief Engineer の文書に受け入れ条件があるとき） | ゲート・振り返り | `{"id", "result": "pass" \| "fail" \| "manual", "evidence"}`。`evidence` はテストの command かユースケースの実行の跡のパス。`fail` があれば `fix`、`manual` があればレベルを1つ上げる（design.md §1.9 の (a)）。足しても版は上げない（原則4） |
| `risk` | 任意（変更そのものが戻せないとき） | ゲート・人 | 担当の一方通行の申告 `{"one_way": true \| false, "impact": ["…"]}`（`one_way` は必須、`impact` は任意）。`one_way: true` はデータの移行・消す操作・外への約束のように revert で戻らない変更。ゲートはレベルを C に上げ、`tier_reasons` に「担当の申告: one_way」と書く。`false`・無しでレベルを下げない（申告は厳しくする向きにだけ効く。原則2）。ゲートが読むのは `one_way` だけで、`impact` は人が読む欄。足しても版は上げない（原則4。[decisions/2026-11-09-report-risk-one-way.md](decisions/2026-11-09-report-risk-one-way.md)） |
| `next[]` | 任意 | トリアージ | `to` は `person` / `chief-of-staff` / `issue`（次の課題に切るもの） |
| `observation` | 観察の課題（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 7）の報告でだけ | Chief of Staff・Chief Engineer・measure | Chief Engineer の文書の価値の仮説を、信号の記録で確かめた結果 `{"hypothesis", "result": "hit" \| "miss" \| "unknown", "evidence": []}`（3つとも必須。`evidence` は読んだ信号の key・URL・パスで、空の配列も可）。ラインは結果で何もしない（Chief of Staff がオーナーに1行で返す）。足しても版は上げない（原則4） |
| `remember[]`・`agents_md[]` | 任意 | トリアージ・振り返り | 人用の報告の `## Remember`・`For AGENTS.md:` の機械向けの写し |

- 担当の `andon issue ready <id>`: `report.json` を検証（必須キー・`commit` ＝ `git rev-parse HEAD` ＝ `gh pr view --json headRefOid`・`review: on` なら最新のレビュアーの課題の `findings.json` が `commit` を指す）してから `kanban review <id>`。合わなければ終了コード 1 と理由1行。`deliverable: candidates`（§2。最後に書いたのが Chief of Staff のときだけ。ほかは `pr` として読む）なら `pr` とレビューを要らず、代わりに `library/candidates.json`（§12）が在って形が合い、repo があれば `report.json` と `candidates.json` の `commit` が `HEAD` と同じことを確かめる。
- 担当の `andon issue review-request <id>`: `review_request` を `report.json` に書き（`round` は前の回から数える）、`kanban report` する。担当は `## レビュー依頼` の文を書かなくてよい。
- `andon issue start`・`report`・`review-request`・`candidates`・`ask`・`ready`・`finish` は、課題の今の `status` から打てるものが [`process/states/issue.yaml`](../process/states/issue.yaml) の移り方（`command`）に在るときだけ動く。無ければ台帳に書く前に終了コード 1 と理由1行で断る（例: `todo` の課題への `ready`、`done` の課題への `review-request`）。定義はバイナリに埋め込んである。`by: deck` の移り方（担当の `deck start`、レビュアー・Chief Engineer の `deck report`・`deck review`）は `kanban` を直に打つので断らない（[ADR-0017](adr/0017-process-definitions.md) 決めたこと 15）。
- 作業側（別のマシン）の担当は同じ command を打つ。command は `<worktree>.issue/outbox/` に書くだけで、dispatcher が引いて指揮側の `library/` に置き、`kanban report`／`review` を打つ（by は dispatcher。design.md §3.2 F5）。

## 5. 問いの形

人が「文脈なしで読めて、1通で答えられる」形。2つに書く:

1. `kanban ask <id> "<text>" --kind <k>`: `questions.yaml` の `text` は人向けの描画（下の JSON から command が作る。何の決断か・事実・理由・選択肢・推奨案・あと何問）。`kind` は下の JSON の `kind` と同じ。台帳の CLI は灯りの決断（`line` まで。design.md §1.3）と `scope` を受ける（`kanban ask --kind scope`。[ledger.md](ledger.md) の Questions）。
2. `issues/<id>/library/questions/<n>.json`（`n` は台帳が付けた番号）:

```json
{"kind": "line",
 "question": "案件 repo の AGENTS.md をレベル B（独立レビュー付きで自動）に下げるか",
 "facts": ["今は **/AGENTS.md がどの repo でもレベル C", "振り返りの指針の1行が人で律速", "委譲は line-change: project（§11）"],
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

- 書き手: 担当（`andon issue ask <id> <json>`）、dispatcher（ゲートの `needs_human`。`from: gate`）、Chief of Staff・Chief Engineer（人に出すとき）。command が JSON を検証して `text` を描き、`kanban ask` を打つ。
- 読み手: Chief of Staff（`kanban questions --json` と並べて読む）・通知の口（将来）・ボードの返事欄（相談 F2）。答えは今までどおり `kanban answer`・`kanban received`。ゲートの問い（`from: gate`）は、課題の PR を人が GitHub でマージしたら dispatcher が「人がマージした（<PR>）」と答えて受け取りまで閉じる（[dispatcher.md](dispatcher.md) §2.7。§14 の `answer` には数えない）。

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
| `asked` | `{"question": n, "status": "open"}` | `n`・`kind`・`question`（§5 の JSON から） |
| `ready` | `kanban ready --json` の前回との差（人の課題を除く） | `title`・`routine` |
| `review-requested` | `report.json` の `review_request`（dispatcher が起こしたとき） | `round`・`reviewer`・`reviewer_issue` |
| `gate` | dispatcher が `andon gate` を回した結果（`wait` は通知しない。§7） | `decision`（`approve` / `fix` / `needs_human`）・`reasons[]`・`record`（§7 のパス） |
| `merged` | ゲートの approve ＋ `merge: auto` ＋ 委譲の範囲内でマージして finish したとき。または `review` の課題の PR を人がマージしたのを見つけ、ゲートの問いを閉じて finish したとき（`by: person`）・ゲートの外の問いが開いている／finish が失敗して片付けなかったとき（`by: person`・`held`。同じ理由では1回。dispatcher.md §2.7） | `pr`・`sha`、人のマージのときだけ `by`（`person`）と、片付けなかったときだけ `held`（理由） |
| `closed` | `review` の課題の PR がマージされずに閉じた（片付けない。1回だけ。dispatcher.md §2.7） | `pr` |
| `impact` | `log.jsonl` の `{"status": "done"}`（Chief of Staff が打った finish も拾う）で、PR が MERGED の課題に dispatcher が `andon issue impact` を1回打ち、影響のある課題があったとき（下の「マージのあとの影響」） | `pr`・`sha`・`issues[]`（`issue`・`conflict`・`overlap[]`・`brief_refs[]`・読めなかったときだけ `error`。ファイルは20まで）。集められずに諦めたとき（3回失敗）は `pr`・`error` だけ |
| `corrections` | `log.jsonl` の `{"status": "done"}` の課題の `corrections.json`（§14）を、次の見回りの打ち直しでも集められなかったとき（書いていない） | `error` |
| `blocked`・`error`・`finished`・`stalled` | 端末の土台の状態（`hc events` か見回り） | `agent`・`status_ms`（hc の `status_ms`。その状態に入った時刻の Unix ミリ秒）。画面の文は入れない |
| `resumed` | 見回りで `doing` の課題の端末が Gone（無い・live でない・素のシェル）を見て、dispatcher が `andon issue resume` を1回打って起こせたとき（dispatcher.md §2.9。起こせなければ `finished`） | `agent`（起こした端末）・`from`（Gone を見た端末） |
| `signal` | 信号の記録（§15）の `weight: now` の信号を、出来事（`episodes`）ごとに最初に見たとき（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・8。`later` は通知しない） | `source`・`key`・`url`・`title`・`weight`（記録の写し）。`title` は外の人の文なので、`<news>` に描くときは「信用しない」の囲いに入れる |

- **届けるか**（表の正は `internal/dispatcher/route.go` の `RouteOf`。理由は ADR-0010 決めたこと 1・7）:

  | `kind`（と条件） | 扱い |
  |---|---|
  | `review-requested`・`resumed` | 記録だけ |
  | `review`（dispatcher がゲートを回す課題） | 記録だけ |
  | `review`（ゲートを回さない課題: Chief Engineer の課題〈`profile: chief-engineer`〉・調べる課題など） | 届ける |
  | `gate`（`decision: fix`・`needs_human`） | 記録だけ（`needs_human` は同じ巡の `asked` で届く） |
  | `gate`（`decision: approve`。dispatcher がマージしない: `merge: person`・委譲の外・`merge: auto` を最後に書いたのが Chief of Staff でない） | 届ける（すぐ） |
  | `gate`（`decision: approve`。`merge: auto` でマージに進む） | 記録だけ。記録の `at` から10分たっても同じ課題の `merged` も `asked` も無ければ、そのとき同じ通知を1回だけ届ける（`review` から出た課題は数えない） |
  | `merged`（`held` が無い） | 記録だけ |
  | `merged`（`held` あり。片付けていない） | 届ける |
  | `report`（レビュアーの課題: `parent` があり `profile` が `*-review`） | 記録だけ |
  | `stalled`・`blocked`・`ready`（Chief Engineer の課題: `profile: chief-engineer`） | 記録だけ |
  | `report`（ほか）・`asked`・`ready`・`impact`・`closed`・`corrections`・`signal`・`blocked`・`error`・`finished`・`stalled` | 届ける |

- **記録だけの置き場** `~/.local/state/andon/dispatcher/<slug>.recorded.jsonl`（`XDG_STATE_HOME` があればその下。控えと同じ所）: 1行が1件の通知で、形はこの節の通知の JSON そのもの（`schema/notification.schema.json` に通る）。書き手は dispatcher だけで、記録だけと決めた通知を、待ちに積む代わりにここへ1行足す。追記だけ（書き換えも切り詰めもしない）。書けなければ控えに残し、次の見回りで先に書く（記録から落とさない・順を保つ）。一部だけ書いて失敗したら書く前の大きさに戻し、書き直すときはファイルの末尾にもう在る行を飛ばす（控えを保存する前に落ちても二重に書かない）。読み手はボード（記録の一覧）。届けた通知はここに書かない。全部の出来事の一覧は台帳の `log.jsonl` が正で、この記録はその代わりではない。
- `at` は UTC の `YYYY-MM-DDTHH:MM:SSZ`（暦にある日時。ゲートの記録の `at` も同じ）。キーは `at`・`project`・`kind`・`detail` が必須、`issue` は課題についての通知のときだけ。`detail` のキーは `kind` ごとに上の表のものだけで、全部必須（`ready` の `routine` と `merged` の `by`・`held` だけ任意。`held` は `by` と一緒）。
- 届け方（届ける通知だけ）: ①Chief of Staff の `Stop` フックが `andon dispatcher drain` を打ち、待ちがあれば `{"hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": "<news>…</news>"}}`（続きのターンになる）。②`UserPromptSubmit` フックが `additionalContext` に添える（委譲の一覧 `andon delegate show --json` も同じ口で添える）。③人が離席のときだけ `hc send --if idle`（入力欄が空のとき）。どれも同じ JSON から同じ文を描く。
- **マージのあとの影響**（`impact`）: `andon issue impact <id>` は、課題の PR がマージされたあと、同じ repo の `doing`／`review` の担当の課題（レビュアーを除く）ごとに、機械で分かる事実だけを JSON で出す（読むだけ。書くのは `git fetch` だけ。PR がまだ MERGED でなければ終了コード 4）。
  出力は `issue`・`pr`・`sha`（マージの commit）・`files[]`（マージした PR が触ったファイル）・`issues[]`。`issues[]` の1件は `issue`・`title`・`status`・`agent`・`branch`・`head`・`pr`・`contains`（branch がもうマージの commit を含む）・`conflict`／`conflict_files[]`（branch とマージの commit を `git merge-tree` で合わせてぶつかるか）・`overlap[]`（両方が触ったファイル。worktree の commit していない変更と、まだ `git add` していない新しいファイルも数える）・`brief_refs[]`（マージした PR のファイルのうち、その課題の依頼書に path か名前が出るもの）・`error`（読めなかった理由。worktree があるのに読めない・在るかを確かめられない〈権限など。無いのとは分ける〉ときも。影響なしに倒さない）。
  影響あり（通知に載る）は `error` があるか、`contains` でなく `conflict`・`overlap`・`brief_refs` のどれかがあるもの。判断（影響なし・rebase を頼む・止めて向きを確かめる）は Chief of Staff（chief-of-staff skill §7）。出力そのものは Chief of Staff と dispatcher だけが読むので schema にしない（通知の `detail` が約束）。

```json
{"at": "2026-11-07T09:00:00Z", "project": "andon", "issue": "A-0012", "kind": "impact",
 "detail": {"pr": "https://github.com/<owner>/andon/pull/12", "sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
            "issues": [{"issue": "A-0014", "conflict": true, "overlap": ["docs/design.md"], "brief_refs": ["docs/design.md"]}]}}
```

```json
{"at": "2026-11-10T09:00:00Z", "project": "sample-site", "kind": "signal",
 "detail": {"source": "github-ci", "key": "github.com/example/sample-site/ci.yml@main", "url": "https://github.com/example/sample-site/actions/runs/1",
            "title": "ci: test failed on main", "weight": "now"}}
```

- `<news>` の文の形は仮設のまま（1行1件・末尾の「人からの新しい依頼ではない。人に言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える」。ADR-0010 決めたこと 3）。`news.replied=<turn>` のラベルの意味（そのターンは人が見る返事ではない）・名前・値も同じ（ボードとの約束）。
- **「人に言うことが無い」の申告**（[ADR-0010](adr/0010-news-ack.md) 決めたこと 4〜6）: Chief of Staff は `andon news ack` を打つ。command は自分の端末（`HC_TERMINAL_ID`）を `hc get` で読み、ラベル `role=chief-of-staff` があればそのときの `turn` で `news.ack=<turn>` を書く（上書き。無ければ書かずに終了コード 1）。dispatcher は `<news>` を渡すときにその端末の `turn` を控え（経路①②は控えの `hook`、③は送ったときの `turn`）、渡したターンが終わったとき `news.ack` が控えた値と同じなら `news.replied=<そのときの turn>` を書く（答えの文は読まない）。経路①②では、渡すときにラベルがもうその値だった（同じ turn の前の ack。`Stop` の block の続きは hc の `turn` が進まない）ら数えない。ack の無いターンは、答えが `<no-reply>` だけでも書かない（人が見る返事に数える。`<no-reply>` の照合は ack を入れた版〈v0.0.8〉の次の版で外した。決めたこと 6）。書いた数は控えに数える（`replied`）。
- dispatcher の控え `~/.local/state/andon/dispatcher/<slug>.json`（`XDG_STATE_HOME` があればその下）: `log.jsonl` の読んだ位置（相談 F1 が入れば cursor）・前回の ready・待ちの通知・③で送った turn と端末・フックで渡した端末・起こしたレビュアー（`issue` → `reviewer_issue`・`commit`）・回したゲート（`issue` → `sha`）。台帳には書かない（台帳に書くのは台帳の CLI 経由の行だけ。dispatcher の行の `by` は `terminal:dispatcher`）。控えの中の形は dispatcher だけが読むので約束にしない（schema は無い）。

## 7. ゲートの条件表と記録

**条件表** `package/gate.json`（既定）＋ セルの `gate.local.json`（上書き。深い併合ではなく、キーごとの置き換え）。仮設のゲートの条件表のキー（`size_review_lines`・`size_human_lines`・`test_paths`・`tier_a_paths`・`tier_b_paths`・`tier_c_paths`・`tier_c_path_exceptions`・`tier_c_added_line_patterns`・`repos{merge, strip_prefix, check[]}`）に、`repos.<repo>.tier_overrides`（パスの glob → レベル）・`repos.<repo>.github_checks`・`github_checks_wait_minutes`・`screen_paths`（下）を足す。既定の条件表で案件 repo の `AGENTS.md`・`REVIEW.md` は **B**（人の決定 2026-11-05。design.md §6 問い7。Andon の repo とセルの repo は C のまま）。条件表を変えるのはレベル C（人）。

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

- キーはどれも必須ではない（`gate.local.json` は置き換えるキーだけを持つ）。知らないキーは拒む。`repos.<repo>.merge` は `merge` / `squash` / `rebase`、`tier_overrides` の値は `A` / `B` / `C`。重ねたあとに `size_review_lines`・`size_human_lines`・`github_checks_wait_minutes` が無ければ、ゲートは誤り（終了コード 3）。
- `repos.<repo>.github_checks`: PR の head に GitHub の checks（Actions の CI など）が要るか。`required`（既定）は1つも無ければ待ち、`optional` は1つも無くても通す（CI の無い repo）。どちらでも、在る checks は通っていなければ止める。
- `github_checks_wait_minutes`: `required` の repo で checks が1つも無いとき、課題が `review` に入った `log.jsonl` の行の `at` からこの分だけ待ち、過ぎたら `needs_human`（push の直後で checks がまだ付いていないのと、CI が無いのとを分ける）。
- `screen_paths`: 画面のファイルの glob の配列（既定は無し＝空。案件の `gate.local.json` で `web/**`・`**/*.css` などを指す。ADR-0018 決めたこと 2）。PR の変更されたファイル（`strip_prefix` を外し、rename の元も）が当たり、PR の本文とコメントのどれにも GitHub の添付の URL（`https://github.com/user-attachments/assets/<id>`。`gh pr create`／`comment --attach` と Web の画面で上げた画像・動画）が無ければ `fix`、`reasons[]` に当たったファイルごとに `screen_paths: <file> に当たるが画面の証拠（添付）が無い`。外の画像の URL・書き換えられていない相対パス（`![x](./a.png)`）・画像でない添付（`user-attachments/files/`）は添付と数えない。どの commit で撮ったかは見ない。

**記録** `issues/<id>/library/gate/<head sha>.json`（dispatcher が書く。`--dry-run` では書かない）:

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

- `at` は ゲートが PR の head の GitHub の checks を読んだ時刻（`github_checks` はそのとき読んだもの）。ゲートを始めた時刻ではない（条件表の `check` を回すあいだに checks が変わっても、記録は読んだ時刻と中身で揃う）。
- キーは全部必須。`review.state` は `current`（`findings.json` の `commit` が `head`）/ `stale`（違う）/ `missing`（無い・読めない）/ `off`（課題の `review: off`）。`current`・`stale` では `issue` と `commit` が必須、`missing`・`off` では2つとも書かない。`delegation.merge` は委譲が無ければ `none`。
- `decision` と `reasons[]` は決定式（純粋関数）の出力そのまま。PR のコメントと commit status（`andon/gate`）には同じ内容を写す（記録の正は台帳の側）。
- `github_checks`: PR の head の GitHub の checks（GraphQL の `pullRequest.headRefOid` と head の commit の `statusCheckRollup.contexts` を、100件ずつ全ページ読む。ページの間に head が動いたら判定しない。ゲート自身の commit status `andon/gate` は数えない。同じ check〈CheckRun は app・workflow の id〈表示名でなく GraphQL の `Workflow.id`。別ファイルの同じ表示名の workflow を束ねない〉・名前、status は context〉の実行が複数あれば、最新〈CheckRun は `databaseId`、status は `createdAt`〉の1つだけを数える＝再実行の前の failure を数えない。新旧を判別できない〈同じ時刻・同じ id〉ときは悪い方を残す）を1つの状態にまとめたもの。`state` は一番悪いもの: `failure`（failure・cancelled・timed_out・startup_failure・status の error）＞ `unknown`（action_required・stale・知らない値）＞ `pending`（終わっていない・status の pending・expected）＞ `success`（success・skipped・neutral。neutral は GitHub のブランチ保護と同じく通す）。1つも無ければ `none`（ゲート自身の `andon/gate` だけのときも）。`names` はその状態にした checks の名前（`success` では通した checks 全部。`none` では空）。
- GitHub の checks の決定: `failure` → `fix`（担当が直せるもの）。`unknown` → `needs_human`。`pending`、または `required` の repo で `none` → **`wait`**（ほかに止める理由が無いときだけ。止める理由があればそちらを返す）。`none` のまま `github_checks_wait_minutes` を過ぎたら `needs_human`。`pending` は時間で人へ上げない（走り終われば success か failure になる）。checks が通っていないあいだは条件表の `check` を回さない。
- **`wait` は記録に書かない**: `andon gate` は記録の形を stdout に出して終了コード 5 で返すが、`gate/<head>.json`・PR のコメント・commit status は書かない（見回りのたびに PR を埋めない）。読み手（dispatcher）は控えに書かず、次の見回りでゲートから回し直す。
- `delegation`: 読んだ委譲の `merge` の値と、判定のレベルがその中か（§11）。外なら approve でも `decision: needs_human`、`reasons[]` に `delegation: merge=B < tier C`。
- 読み手: dispatcher（approve → `merge: auto` ＋ 委譲の範囲内なら、マージの直前に PR の head と GitHub の checks を読み直し、head が記録の `head` のまま・checks が通る状態のときだけマージ → `andon issue finish`。違えば打たず、次の見回りでゲートから回し直す。残る隙間: マージの直前の読み直しと `gh pr merge` の間に、同じ head のまま checks が走り直した・落ちたときは止められない（`--match-head-commit` が固定するのは head だけ）。サーバーの側で checks を強制できるのはブランチ保護（ruleset）だけで、private の repo の Free のプランでは使えなかったが、2026-11-09 に public にして ruleset `main`（PR 必須・required checks `tests` と `go`・force push と削除の禁止）を付けたので、この隙間は塞いである。wait → 控えに書かずに次の見回りで回し直す。fix → 担当へ `hc send`。`needs_human` → §5 の問いを `from: gate` で出す）・振り返り（fix の理由・レベルの分布）・人。

## 8. 端末の土台（hc）との約束

| 何 | 形 | 書き手 | 読み手 |
|---|---|---|---|
| 担当・レビュアーの端末の名前 | `<slug>-<id を小文字>`（`a-z 0-9 - _`、32字まで） | `andon issue start`・`andon issue resume`・dispatcher | dispatcher・ボード |
| Chief of Staff の端末の名前 | `<slug>-cos`（cwd は台帳の root） | `andon chief-of-staff` | `andon chief-of-staff`（動いているものを使うときに名前と cwd を照らす） |
| ラベル | `role=chief-of-staff \| dispatcher \| chief-engineer`・`project=<slug>`・`issue=<id>`・`title=<題>`・`news.replied=<turn>`（dispatcher が書く。§6）・`news.ack=<turn>`（Chief of Staff の「人に言うことが無い」の申告。`andon news ack` だけが書き、dispatcher〈`drain` と見回り〉が読む。§6）・`session=<id>`（ハーネスのセッションの id。台帳の `session` と同じ値。hc の template は呼び手の環境変数を渡さないので、template の `command`・`resume` の行が `andon issue session` で自分の端末のこのラベルを読み、Claude 系は `--session-id`／`--resume`、Codex 系は `resume <id>` に渡す。`andon issue start`〈Claude 系〉・`andon chief-engineer`・`andon issue resume`・dispatcher〈Claude 系のレビュアー〉が付ける。[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 11・14）・`first_message.sent=false`（最初のメッセージが届かなかった〈`hc run` の `first_message` が `sent: false`、か `started: false`〉担当・Chief Engineer・レビュアー。`andon issue start`・`andon chief-engineer`・dispatcher〈レビュアー〉が付け、dispatcher が送り直して外す。[dispatcher.md](dispatcher.md) §5） | command・dispatcher | ボード・dispatcher・フック（自分の課題を引く） |
| Herdr App の置き場のラベル | `herdr.space=<id>`・`herdr.tab=<id>`（持ち主は Herdr App。id は command が決める: Chief of Staff は `andon-<slug>`・`andon-<slug>-cos`、Chief Engineer は `andon-<slug>`・`andon-<slug>-<id>`、担当は `andon-<slug>-<id>`・`andon-<slug>-<id>`（Space ＝ 課題。[ADR-0003](adr/0003-task-spaces.md)。machine に依らない。`machine` 付きの課題を `andon issue start` はまだ起こさない。ADR-0002 決めたこと 19）、レビュアーは担当の端末（台帳の `agent`）の値を `hc get --json` で読んで写す（読めなければ付けない）。dispatcher は台帳が1つなら `andon-<slug>`・`andon-<slug>-dispatcher`、2つ以上なら `andon-<セルの slug>-plant`・`andon-<セルの slug>-dispatcher`（セルの slug は hostname。ADR-0002 決めたこと 21）。`where` は App が書く出力で、Andon は読まず書かない。[ADR-0002](adr/0002-herdr-spaces.md)） | `andon chief-of-staff`・`andon chief-engineer`・`andon issue start`・`andon issue resume`（止まる前と同じ値。レビュアーは元の課題の担当の値 `andon-<slug>-<元の id>`）・dispatcher（自分の端末のラベルは boot とフックの `dispatcher.Ensure` が付ける）。`andon issue finish` は触らない（端末の kill・rm で tab が空になって消える） | Herdr App |
| 人に端末を指す言い方 | `<Space 名> / <tab 名>`（App の `where` と同じ形。例 `sample-site / CoS`・`sample-site / CE A-0003 Billing の設計`・`A-0012 ログインの直し / A-0012`）。address は末尾に `（t7）` と添えるだけ。Andon の command は `hc attach` を打たず、最後に出た場所をこの形で出す | `andon chief-of-staff`・`andon chief-engineer`・`andon issue start`・Chief of Staff | 人 |
| template の名前 | プロファイルの名前そのもの（`chief-of-staff`・`chief-engineer`・`claude`・`sonnet`・`fable`・`codex`・`codex-review`・`claude-review`）。プロファイルの frontmatter に `resume:` の行があれば、boot がその行を `command` にした `<profile>-resume` も作る（止まった端末をセッションの続きから起こす。元の template からは `resume:` を除く。`templates.local/` が同じ名前で勝てば勝った側から作る） | boot が生成 | `andon issue start`・`andon issue resume`（`hc templates` で存在を確かめる） |
| 台帳の `agent` | 端末の address。作業側が別のマシンなら `<machine>/t7`（相談 F4） | `kanban start`（担当）・dispatcher（作業側の代筆）・`andon issue resume`（最初のメッセージが届いてから `kanban set`） | dispatcher・ボード |
| 台帳の `session` | ハーネスのセッションの id（小文字の UUID）。Claude 系は起こすときに Andon が決めて `--session-id` で渡す。Codex 系は決める口が無いので、`resume` のときに記録（`$CODEX_HOME/sessions/**/rollout-*.jsonl` の先頭行 `session_meta` の `cwd`・`id` だけ）から cwd が同じ最新のものを引いて書く | `andon issue start`（Claude 系）・`andon chief-engineer`・`andon issue resume`・dispatcher（Claude 系のレビュアーを起こすとき） | `andon issue resume` |
| 起こし方 | `hc run --bg --outlive-parent --no-notify --name … --label … --cwd … [--on <machine>] <profile> "<最初の3行>"` | `andon issue start`・dispatcher | — |
| 起こし直し | `andon issue resume <id> [--fresh] [--dry-run [--json]]`: `doing` で端末（台帳の `agent`）が無い・終わった・素のシェルの担当・Chief Engineer・レビュアーだけ（`todo`・動いている・`machine` のある課題は理由1行で止まる）。台帳の `worktree` が無ければ `git worktree prune` のあと `git worktree add <path> <branch>` で同じパスに戻す（パスは台帳の値のまま。今の置き場か旧の置き場の `<repo 名>/<slug>-<id>` でなければ止まる。あれば触らない）。レビュアー（profile `*-review` で `parent` のある課題）は `git worktree add --detach <path> <commit>` で戻し（commit はそのレビュアーの依頼書の「対象: … commit <sha>。」の行。無ければ止まる）、cwd は `issues/<rev>/`。セッションの記録（Claude は `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl` の有無だけ、Codex は上の `session` の行）が無ければ起こさず非 0。素のシェルの端末は片付けて（hc の名前は live の中で1つ）、同じ名前・同じラベル（Chief Engineer は指揮側の Space）で `<profile>-resume` を起こす。最初のメッセージは「課題 … の続き。セルが起き直した。…」。届かなければ起こした端末を片付けて非 0、台帳は変えない。届いたら台帳の `agent`（と変わった `session`）を書く。`--fresh` は `<profile>` の template で新しいセッション（最初のメッセージは start の2行に前の担当の `report.md` を読む1行）。`--dry-run` は hc を読むだけ（`get`・`ls`）で、計画（template・止まった端末・cwd・worktree と戻す手・branch・session・ラベル・最初のメッセージ・確かめの結果）を出す。[ADR-0012](adr/0012-resume-and-worktree-home.md) | `andon issue resume`（Chief of Staff・dispatcher〈R4〉） | — |
| lock | `andon issue start`・`resume`・`finish` は台帳ごとの lock（`$XDG_STATE_HOME/andon/dispatcher/<slug>.issue.lock`）を命令の間持つ（`--dry-run` は持たない）。dispatcher の控えの lock（`<slug>.json.lock`）とは別（dispatcher は控えの lock を持ったまま `andon issue finish` を打つ） | `andon issue start`・`resume`・`finish` | — |
| 乾いた実行 | `andon issue start <id> --dry-run [--json]`（`resume` の `--dry-run` は上の「起こし直し」）: 確かめ（本番と同じ順・同じ理由・同じ終了コード）だけを通し、hc を呼ばず、端末・worktree・branch・ラベル・信頼の設定を作らず台帳も書かずに、上の名前・ラベル・cwd・プロファイル・最初のメッセージ（`hc run` に渡す文そのもの）と依頼書の全文・worktree・branch・確かめの結果（`ok`／hc を呼ぶものと作る直前のものは `skipped`）を出す。`--json` の形は人と評価が読む出力で、schema に無い（機械は判定に使わない。[決定](decisions/2026-11-09-issue-start-dry-run.md)）。Claude 系の `session` の値（ラベル `session=`）だけは起こすたびに新しく決めるので、本番と違う | `andon issue start` | 人・Chief of Staff |
| 状態 | `hc events`（dispatcher）か `hc ls --json`（見回り）。`working`・`idle`・`done`・`blocked`・`error`・`finished` | hc | dispatcher |

## 9. フック

managed settings の `hooks`（boot が `package/settings.json` から生成）。フックは**dispatcher と command を呼ぶだけ**で、判断を持たない。

| 誰の | イベント | 打つもの | 返すもの |
|---|---|---|---|
| Chief of Staff | `Stop` | `andon dispatcher drain`（端末のラベル `role=chief-of-staff`・`project=` から slug を引く） | 待ちがあれば `{"hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": "<news>…</news>"}}`（続きのターンになる。`decision: block` と違い画面に「Stop hook error」が出ない。[ADR-0019](adr/0019-cos-input-box.md) 決めたこと 6）。無ければ何も返さない。前にフックで渡した `<news>` の turn の終わり（入力の `prompt_id`・`session_id` が合う）なら確かめ済みにし、端末のラベル `news.ack` が渡したときに控えた `turn` と同じなら `news.replied=<turn>` を書く（答えの文〈`last_assistant_message`〉は読まない。§6）。合わなければ渡した通知を1回だけ届け直す。控えの lock が取れなくても、渡した turn の終わりなら確かめ済みにする（次の `drain` が当てる。[dispatcher.md](dispatcher.md) §3） |
| Chief of Staff | `UserPromptSubmit` | `andon dispatcher drain --context`（中で `andon delegate show --json` を読む） | `{"hookSpecificOutput": {"hookEventName": "UserPromptSubmit", "additionalContext": "<news>…</news>\n<delegation project=\"<slug>\">…</delegation>"}}`（待ちが無ければ委譲だけ）。前にフックで渡した `<news>` の turn の終わりを見ていなければ（中断）、その通知を1回だけ届け直す |
| 担当 | `Stop` | `andon issue stop-check`（端末のラベル `issue=` から id を引く） | 台帳が `doing` で、前回の通過（無ければ `doing` に入った行）以後に `kanban report`／`review`／`ask` のどれも `log.jsonl` に無く、待ちでもなければ `{"decision": "block", "reason": "<id> は doing のまま、報告も問いも台帳に無い。…"}`。待ち（通す）は、自分の問いが開いたまま（その問いの最後の行が `open`）か、`report.json` に `review_request` があり、その `commit` を指すレビュアーの `findings.json` がまだ無いとき。答え（`answered`）・レビューが届いたあとは待ちではない。続けて止めるのは3回まで。控えが書けなければ通す。レビュアーも同じ。Chief Engineer の課題（`profile: chief-engineer`）は止めない（[ADR-0009](adr/0009-cos-default-entry.md) 決めたこと 10） |

- フックの実体は `package/hooks/stop`・`user-prompt-submit`（image では `/opt/andon/hooks/`）。どの端末でも同じフックが走り、役は command がラベルで見分ける（どちらでもなければ何も出さない）。

- 実測（2026-11-10。担当の端末の `andon issue stop-check` の block）: `Stop` の `decision: block` で始まる続きは、hc の `turn` を進めない（block の前と同じ値）。だから①で渡したときに控える `turn` と、続きで打つ `andon news ack` の値は同じになる（§6）。
- 要実測: Chief of Staff の `Stop` の `additionalContext` で続きの turn が始まり「Stop hook error」が出ないこと、その続きが hc の `turn` を進めるか（進むなら `andon news ack` の照らし方を合わせる）、`UserPromptSubmit` の `additionalContext` が文脈に入ること、Codex の同等物（最初の版は Claude Code だけ）。
- フックが落ちたとき（command が無い・台帳が見つからない）は**通す**（止めない。dispatcher のログに1行）。フックは守りの1枚で、唯一の守りではない（`andon issue ready` と dispatcher の照合が残る）。

## 10. 互換と版

- `schema/*.schema.json` の `$id` に版（`https://andon.example.com/schema/<名前>/<版>`。例 `https://andon.example.com/schema/report/2`。URL は識別子で、取りに行かない）。`tests/schema.sh` が `$id` の形・draft 2020-12・`description` と識別子が英語であることも確かめる。読み手は `$id` ではなく**キーの有無**で振る舞う（原則4）。
- キーを足す: 版を上げない。`docs/contract.md` と `tests/schema.sh` の例を足す。
- キーを消す・意味を変える: 版を上げ、古い版を読む側を1版だけ残す（セルの `FROM` の版上げが追いつくまで）。
- ledger-format の `version` が上がったら、Andon の command は台帳の CLI の言うとおりに止まる（ledger-format の規則）。
- **版 2（`notification`・`report`・`question`・`gate`・`project`。[ADR-0008](adr/0008-issue-naming.md)）**: 台帳のカードの名前を task から issue に改めた。`task` → `issue`・`reviewer_task` → `reviewer_issue`・`tasks[]` → `issues[]`（`impact`）・`review.task` → `review.issue`・`next[].to` と問いの `from` の値 `task` → `issue`・`triage_max_tasks` → `triage_max_issues`。`task-keys` は `issue-keys`（`/1`。中のキーは同じ）に改名。この版上げだけは古い版を読む側を残さない（ADR-0008 決めたこと 7。既存の台帳と `project.json` は手で移す）。ラベル `task=` も `issue=` に改め、`task=` は読まない。

## 11. プロジェクトの事実と委譲（`project.json`）

プロジェクトについて機械が読む事実（[ADR-0004](adr/0004-project-layout.md) 決めたこと 1）。**置き場はセルの repo の `projects/<slug>/project.json`**（台帳の外。書き手が人だけだから）。委譲（オーナー〈人〉が Chief of Staff・dispatcher・担当に何をどこまで任せたか。design.md §1.10）はその `delegation` に入る。人が見る一覧は `andon delegate show`。

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
      "evidence": "移行中はデプロイを人が見る",
      "revoked": {"at": "2026-11-20", "by": "owner", "reason": "移行が済んだ"}}]},
 "chief_of_staff": {"parallel": 2, "review_when": ["外向きの面", "新しい module", "約500行を超える差分"], "triage_max_issues": 3},
 "signals": [
   {"source": "github-ci", "repo": "github.com/example/sample-site", "weight": "now"},
   {"source": "github-issues", "repo": "github.com/example/sample-site", "weight": "later", "every": 30, "filter": "label:bug", "write_back": true},
   {"source": "sentry", "weight": "later", "org": "example", "project": "sample-site"}]}
```

| キー | 要るか | 意味 |
|---|---|---|
| `version` | 必須 | この形式の版（整数の `1`） |
| `project` | 必須 | `{title, prefix}`（2つとも必須）。`title` は人に見せる名前、`prefix` は課題の id の接頭字（英大文字で始まる英大文字・数字） |
| `repos` | 必須 | このプロジェクトが**書いてよい** repo（`<host>/<owner>/<repo>`。重ならない〈command が確かめる〉）。`andon issue start` は課題の `repo` がここに無ければ起こさない。台帳の `config.yaml` の `repos` はこの部分集合。空なら repo を持たない |
| `references` | 任意 | 読むだけの repo（`<host>/<owner>/<repo>`）か `https` の URL。書かない |
| `delegation` | 任意 | 委譲（下の表。移す前の `delegation.json` と同じ形）。無ければ無委譲（全部人） |
| `chief_of_staff` | 任意 | Chief of Staff が読む設定値。`parallel`（並行の上限。1 以上。既定 2）・`review_when`（独立レビューを入れる条件。1行1条件。command は読まない）・`triage_max_issues`（1回のトリアージで起こす課題の上限。1 以上。既定 3） |
| `signals` | 任意 | 外の信号の出所（下の表。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 11）。無い・空なら何も取りに行かない |

`signals[]` の1件（読むのは `andon signal collect`。§15）:

| キー | 要るか | 意味 |
|---|---|---|
| `source` | 必須 | `github-ci`（案件 repo の main の CI の失敗）/ `github-issues`（open の issue）/ `sentry` / `linear` / `cloudflare`（Workers の Issues） |
| `repo` | 任意 | 出所の repo（`<host>/<owner>/<repo>`）。`repos` か `references` の中のものに限る（command が照合する。schema では見ない）。GitHub の出所で無ければ `repos` の全部を読む |
| `weight` | 必須 | `now`（通知 `signal` ですぐ Chief of Staff へ。§6）/ `later`（記録だけ。トリアージが拾う） |
| `every` | 任意 | 読む間隔（分。整数。1 以上）。既定 10 |
| `filter` | 任意 | 出所の検索の語をそのまま（`github-issues` なら GitHub の検索の語。例 `label:bug`。`gh issue list --search` に手を加えず渡す）。空にしない。無ければ open の issue 全部（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 9） |
| `write_back` | 任意 | `andon issue finish` がこの出所のトラッカー（課題の `links` の URL）に1コメント書くか。既定 `false`（書くのは人だけなので、`true` が委譲の印） |
| `org`・`project` | 任意 | GitHub 以外の出所の中の場所（Sentry の organization と project など）。空にしない |

- **`signals` を書くのは、セルの image を `signals` を知る版に上げてから**。古い版の `andon` は知らないキーとして弾き（schema の `additionalProperties: false` と command の `DisallowUnknownFields`）、`project.json` を読めないものとして**無委譲・書いてよい repo 無し**に倒れる（担当を一切起こせず、`andon delegate` も打てない）。原則4（知らないキーは無視）と `project.json` の扱い（原則6・読み方の 2）は食い違っていて、`signals` もそれを引き継ぐ（直すのは別の決定）。

`delegation` の中:

| キー | 要るか | 意味 |
|---|---|---|
| `version` | 必須 | 委譲の形式の版（整数の `1`） |
| `project` | 必須 | プロジェクトの slug（置き場と一致しなければ無委譲） |
| `preset` | 任意 | 既定の型 `hobby` / `work` / `andon`（design.md §1.10 の表）。無ければ `work` |
| `grants[]` | 任意 | 型からの足し引き。後の行が前の行を上書きする（同じ `scope` の有効な grant のうち、`at` が最新のもの） |
| `grants[].id` | 必須 | `d1` … 一意（一意かは command が確かめる） |
| `grants[].scope` | 必須 | `merge` / `deploy` / `publish` / `irreversible` / `external-requests` / `profile` / `package-update` / `line-change` |
| `grants[].value` | 必須 | `scope` ごとの値（design.md §1.10 の表）。`merge`＝`none`/`A`/`B`/`C`、`deploy`＝`none`/`preview`/`production`、`publish`＝`none`/`internal`/`public`、`irreversible`＝`delete-branch`/`force-push`/`rewrite-history` の配列（`delete-remote` は書けない）、`external-requests`・`profile`・`package-update`＝真偽値、`line-change`＝`none`/`project`/`plant` |
| `grants[].by` | 必須 | 付与した人。文字列の `owner` だけ（実際の人はセルの `andon.toml` の `owner`。照合は commit の author で） |
| `grants[].at` | 必須 | 付与した日（`YYYY-MM-DD`。暦にある日。`2026-02-30` やうるう年でない年の `02-29` は通らない） |
| `grants[].until` | 任意 | 期限（`YYYY-MM-DD`）。過ぎていれば無いものと同じ。`null` は無期限 |
| `grants[].evidence` | 必須 | 根拠。人の発言の記録へのパスか引用（写さず指す） |
| `grants[].revoked` | 任意 | 取り消し `{at, by, reason}`（3つとも必須。`null` は取り消していない）。あれば無いものと同じ（行は消さない。履歴のため） |

**読み方**（読む側は全部この順）:

1. セルの repo の **HEAD** の `projects/<slug>/project.json` を読む（worktree・作業側の写し・未 commit の変更は読まない）。
2. そのファイルを最後に変えた commit の author が `andon.toml` の `owner` と一致しなければ、**無委譲**（全部人）・**書いてよい repo 無し**として読む。無い・JSON でない・schema を通らない・`delegation.project` が違う・`version` を知らない、も同じ（原則2）。`delegation` が無ければ無委譲。
3. **移す前の `projects/<slug>/delegation.json`・`delegation.md` は読まない**。`project.json` が無く `delegation.json` だけがあるセルは、移すまで無委譲・repo 無し（安全側。`andon delegate show` の理由にそう出る）。移すのは台帳の改名の課題（ADR-0004 決めたこと 5 の 4）。
4. `delegation.preset` の値に、有効な `grants[]`（`revoked` が無く `until` が過ぎていない）を `at` の順に重ねる。
5. 読み手ごとに見るもの: ゲート＝`merge`。dispatcher と `andon issue …`＝`deploy`・`publish`・`irreversible`（打つ前。範囲外なら止まって理由1行）。`andon issue start`＝`repos`（課題の `repo` が無ければ起こさない）。Chief of Staff＝委譲の全部（起きるたび。`UserPromptSubmit` のフック経由。範囲外の判断は §5 の問い）と `chief_of_staff`。
6. Andon のセルの `andon` プロジェクトは `preset` を `andon` 以外にできない（command が拒む。`merge` の上限 `B`・`line-change: none`）。`irreversible` に `delete-remote` は型でも grant でも入れられない。

**書き方**: `project.json` は人が書いて commit する（`andon delegate` は無い `project.json` を作らない）。委譲は人がセルの repo で `andon delegate grant <scope> <value> --evidence <path|text> [--until <date>]`・`narrow`・`revoke <id> --reason <text>`・`show [--json]` を打つ。command はセルの repo の **HEAD** の `project.json`（最後の commit の author が owner のもの）から組み立て、`delegation` だけを変え（ほかのキーは HEAD のまま。形は2字下げに揃う）、全体を検証して書き、`project.json` を commit する（message に `scope`・`value`・`evidence`）。commit の author と committer は端末の git の user ではなく `andon.toml` の owner（名前は HEAD の `project.json` を最後に変えた owner の commit のもの）にする。owner が `andon.toml` に無ければ commit せずに止まる。`project.json` に commit していない変更があるか、HEAD を最後に変えたのが owner でなければ打たない（ほかの書き手の変更を人の commit に混ぜて信じられる形にしない）。Chief of Staff・担当・dispatcher の端末では打てない（managed deny ＋ 端末のラベルに `role=` か `issue=` があれば拒む）。

**依頼書の Settings との関係**: `issue.yaml` の `merge: auto` は委譲の `merge` の範囲の中でだけ効く（§2）。委譲の外なら、ゲートは approve でも `needs_human`（§7 の `delegation`）。

## 12. 候補の一覧（`candidates.json`）

担当の課題の `library/candidates.json`。調べて見つけた、直すかもしれないが決めていないもの（候補）の一覧。調べる課題（`deliverable: candidates`。§2）の成果で、直す課題が範囲の問い（§5 の `kind: scope`）で止まるときにも同じ形で書く。担当は手で書かず、`andon issue candidates <id> <json|->` で書く（打てるのはその課題の担当の端末〈`issue.yaml` の `agent` ＝ `HC_TERMINAL_ID`〉だけ。command が形を確かめ、repo があれば `commit` に `HEAD` を入れる。入力に `commit` は書かない。形が違えば書かずに終了コード 1 と理由1行で、前の一覧はそのまま）。書き直すたびに全体を書く（`report.json` と同じ）。答えが出て直したあとも消さない（`finish` でも残る）。

```json
{"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "candidates": [
   {"id": "c1", "text": "docs/design.md:42 「台長」→「台帳」", "file": "docs/design.md", "line": 42, "in_scope": true},
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
| `candidates[].text` | 必須。1行 | 何が・どこに・どう直すか（直し方を含む）。人と Chief of Staff が読む |
| `candidates[].file`・`line` | 任意 | 場所を指せるとき。形は `findings.json`（§3）と同じ（repo からの相対パス・1 以上の整数） |
| `candidates[].in_scope` | 必須。真偽値 | 依頼書の範囲（やること・完了条件）の中か。直す課題では、中なら直し、外なら残す・問うの振り分けの元。調べる課題では「問いの答えに当たるか」 |

- 書き手は担当（調べる課題・止まる直す課題）。読み手: Chief of Staff（切る・範囲の問いに答える）・人（ボードと報告から）・トリアージ（将来）・振り返り（候補の数・`in_scope` の比）。ゲートも dispatcher も判定に使わない。形を固定するのは、問いの `facts` と依頼書の `links` が同じ形で指せるように。
- `id` の一意は、schema では検査器の拡張のキーワード `uniqueKeys`（`package/lib/andon/schema_check.py`。標準の JSON Schema に無い）で書く。ほかの検査器はこれを飛ばすので、`andon issue candidates` も確かめる。
- 人が読む分は報告の `## Report` に同じ一覧を表で写す（1行1候補。`id`・場所・何・範囲の中か）。
- 候補が多い（目安 10 を超える）とき、調べる課題は全部書き、直す課題は「数と種類」を問いに書いて候補は代表だけ。
- レビュアーの `findings.json`（§3。直す前提の指摘で、ゲートが数える）とは別の約束。混ぜない。

## 13. 流れの数え（`andon measure --json` の出力）

`andon measure [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]` が、1つの台帳の期間の流れ・効率・対抗の指標を数えて出す（[決定](decisions/2026-11-09-measure-flow-metrics.md)）。**読むだけで、何も書かない・保存しない**。この節はその `--json` の形で、置き場は無い（標準出力だけ）。人向けの表（`--json` 無し）は先頭が「価値: 仮説を確かめた数 …」の1行で、価値 → 流れ → 効率の順に並び（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 18。[決定](decisions/2026-11-10-measure-value-line.md)）、自動マージ率の行に対抗の指標を並べる。表は人が読むもので、機械は読まない。

読むもの: `log.jsonl`（`at`・`by`・`id`・`change.status`。問いの行の `status` は問いの状態なので課題の状態に数えない）・`issue.yaml`（`created`・`after`・`links`・`profile`・`parent`）・`questions.yaml`（`asked`・`answered`）・`report.json`（`pr`・`observation.result`）・ゲートの記録（`at`・`pr`・`tier`・`decision`・`merged`）・`corrections.json`（要素の数）と、台帳の PR がある repo ごとに1回の `gh pr list --state merged`（`number`・`url`・`title`・`body`・`mergedAt`）。期間は UTC の日付で両端を含む（既定は今日までの7日）。

```json
{"project": "sample-site", "since": "2026-11-01", "until": "2026-11-04", "days": 4,
 "value": {"observed": {"count": 3, "hit": 1, "miss": 1, "unknown": 1},
   "days_to_observe": {"count": 2, "days": 3.75}, "no_hypothesis": 1},
 "flow": {"lead_time": {"count": 2, "hours": 9},
   "wait": {"todo": {"count": 3, "hours": 1.33}, "queued": {"count": 0, "hours": null},
            "doing": {"count": 3, "hours": 19}, "review": {"count": 2, "hours": 10.5}},
   "question_open": {"count": 1, "hours": 2},
   "closed": {"count": 2, "per_day": 0.5}, "wip_end": 1},
 "efficiency": {"merged_prs": 2, "human_touches": {"total": 3, "per_pr": 1.5},
   "auto_merge": {"count": 1, "rate": 0.5},
   "gate": {"records": 3, "fix": 1, "needs_human": 1, "rate": 0.67}},
 "counter": {"reverted": 1, "follow_ups": 1,
   "reopened": {"corrections": "recorded", "by_tier": {"A": 1, "B": 1, "C": 0, "none": 0}},
   "corrected": {"corrections": "recorded", "by_tier": {"A": 0, "B": 1, "C": 0, "none": 0}}},
 "prs": [
   {"issue": "S-0001", "pr": "https://github.com/example/sample-site/pull/1", "merged_at": "2026-11-01T06:00:00Z",
    "tier": "A", "auto": true, "human_touches": 1, "lead_time_hours": 6, "reverted": true, "corrections": false},
   {"issue": "S-0002", "pr": "https://github.com/example/sample-site/pull/2", "merged_at": "2026-11-02T12:00:00Z",
    "tier": "B", "auto": false, "human_touches": 2, "lead_time_hours": 12, "reverted": false, "corrections": true}],
 "skipped": [{"source": "kanban/log.jsonl", "line": 8, "reason": "JSON の object でない"}]}
```

| キー | 数え方 |
|---|---|
| `value.observed` | 期間に報告された観察の課題（`report.json` に `observation` があり、`log.jsonl` で最初に `review`〈なければ最後の列〉へ動いた時刻が期間の中のもの。レビュアーの課題は数えない）の数と、`observation.result` の内訳（`hit`・`miss`・`unknown`）。観察の課題は `parent` が観察する Chief Engineer の課題（`profile: chief-engineer`） |
| `value.days_to_observe` | 上のうち `parent` が Chief Engineer の課題のもの、の「Chief Engineer の課題が最後に最後の列へ動いた時刻から観察の報告まで」の日数（中央値。報告のほうが早いものは数えない）。数えるものが無ければ `days` は `null` |
| `value.no_hypothesis` | 期間に最後の列へ動いた Chief Engineer の課題のうち、`parent` に指す観察の課題（`report.json` に `observation` がある非レビュアーの課題。通常の epic の子課題は `not_before` を持っていても数えない）が無いもの＝価値の仮説が「無し」の題（ADR-0011 決めたこと 4）。観察の課題を足す前、または観察の報告が出る前の題も数える（報告が出ると外れる） |
| `flow.lead_time` | 期間にマージされた台帳の PR の、課題の `created` から `mergedAt` まで（中央値の時間） |
| `flow.wait.<status>` | `log.jsonl` の `change.status` を辿り、期間の中でその状態にいた時間の、1件あたりの平均（最後の列を除く。入った時刻は `created`、`after` のある課題は `queued` から） |
| `flow.question_open` | `asked` から `answered`（無ければ期間の終わり）までの、期間の中の時間の1問あたりの平均 |
| `flow.closed` | 期間に最後の列へ動いた課題の数と、日数で割った値 |
| `flow.wip_end` | 期間の終わりに、最初の列・`queued`・最後の列のどれでもない課題の数 |
| `efficiency.merged_prs` | `report.json` の `pr`（無ければ最後のゲートの記録の `pr`）が、期間に GitHub でマージされた課題の数 |
| `efficiency.human_touches` | その課題の `log.jsonl` の行のうち `by` が `terminal:`・`routine:` でないもの（一次の指標）。`per_pr` はマージした PR あたり |
| `efficiency.auto_merge` | そのうちゲートの記録が `merged: true`（dispatcher がマージ）のもの |
| `efficiency.gate` | 期間に書かれたゲートの記録と、差し戻し率（`fix` ＋ `needs_human`）÷ 記録（[design.md](design.md) §5.1） |
| `counter.reverted` | 期間にマージされた PR のうち題が `Revert ` で始まり、本文が `<owner>/<repo>#<n>` か URL で台帳のマージされた PR を指すもの、が指す PR の数 |
| `counter.follow_ups` | 期間に作られた課題のうち、`links` か、マージされた自分の PR の本文が、別の課題のマージされた PR の URL を含むもの |
| `counter.reopened` | 人が開け直した数をゲートのレベルごとに: `merged_prs` のうち `corrections.json` に1件以上ある課題と、期間に revert された自動マージの PR（重なれば1回）。`corrections` は台帳に `corrections.json` が1つも無ければ `none`（記録なし） |
| `counter.corrected` | 人の手直しがあった課題の数をゲートのレベルごとに: `merged_prs` のうち `corrections.json`（§14）の `corrections` に1件以上ある課題（`reopened` と違い revert は数えない）。`corrections` は `reopened` と同じ |
| `skipped[]` | 読めなかった記録（JSON でない・`at` か `by` が無い `log.jsonl` の行、在るが読めないファイルとディレクトリ（課題・ゲートの記録の置き場を含む）、`answered` が UTC の時刻でない問い、`observation.result` が hit・miss・unknown のどれでもない報告、PR の一覧を読めない repo）。止まらずに飛ばし、0 には数えない。理由に絶対パスは書かない |

- 書き手は `andon measure` だけ（出力するだけ）。読み手は人と振り返り（委譲を広げる根拠）。ゲートも dispatcher も判定に使わない。
- 数えるものが無ければ件数は 0、率・平均・中央値は `null`（表は「記録なし」）で、終了コード 0。台帳の `config.yaml` が読めない・版が 3 でないときだけ終了コード 1。
- レビュアーの課題（`<id>-r<k>`・`profile` が `-review` で終わる）は流れにも効率にも数えない。

## 14. 人の手直し（`corrections.json`）

課題の `library/corrections.json`。人がエージェントの仕事を直した跡（[design.md](design.md) §1.9 の (b) の表の 1〜3）を、振り返りの入力として1か所に集める。書き手は dispatcher だけで、`log.jsonl` の `{"status": "done"}` の行（`andon issue finish`。Chief of Staff の `kanban done` も）を拾った課題ごとに**1回だけ**書く。もう在れば書き直さない（同じ finish を2度拾っても1回）。レビュアーの課題には書かない。

- **空の配列は「無かった」、ファイルが無いのは「集めていない」。** 手直しが無ければ `{"at": …, "corrections": []}` を書く。`gh` か台帳が読めなければ書かず、次の見回りで1回だけ打ち直す。それも駄目なら書かずに通知 `corrections`（§6。`detail` は `error`）を Chief of Staff へ積む。
- 読み手は振り返りだけ。ゲートも dispatcher も判定に使わない。端末に送った文（表の 4）と Chief Engineer の推奨案を覆したこと（表の 5）は拾わない（文の解析になる）。

```json
{"at": "2026-11-09T09:00:00Z",
 "corrections": [
   {"kind": "answer", "question": 2, "decision_kind": "users", "recommended": "CSV だけ", "picked": "CSV と Excel"},
   {"kind": "pr",
    "comments": [{"author": "<owner>", "at": "2026-11-09T08:40:00Z", "body": "見出しの語は台帳の語に揃える"}],
    "human_commits": [{"sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "headline": "見出しの語を直す", "files": ["docs/a.md"]}]},
   {"kind": "override", "finding": {"severity": "major", "file": "internal/issue/issue.go", "line": 88}, "reason": "起きる条件がこの repo に無い"},
   {"kind": "override", "question": 3, "recommended": "人が PR を読んでマージする", "answer": "担当に直させる"}]}
```

手直しが無かったとき:

```json
{"at": "2026-11-09T09:00:00Z", "corrections": []}
```

| `kind` | 元 | 拾う規則 | キー |
|---|---|---|---|
| `answer` | `questions.yaml`（`kanban show --json` の `questions`）＋ `library/questions/<n>.json` | 答えがある（`answered`・`received`）問いのうち、ゲートの問い（`from: gate`）と範囲の問い（`kind: scope`。Chief of Staff が答える）を除き、答え（前後の空白を除く）が推奨案（`pick: true`）の `label` と違うもの。問いの JSON が無い問い（`andon issue ask` を通らない）は推奨案が分からないので拾わない | `question`（`n`）・`decision_kind`（問いの `kind`）・`recommended`・`picked`（台帳の答えのまま） |
| `pr` | `gh pr view <n> --json state,commits,comments,reviews`・`gh api --paginate --slurp repos/<owner>/<repo>/pulls/<n>/comments`（インラインのレビューコメント）・人の commit ごとに `gh api repos/<owner>/<repo>/commits/<sha>` | コメント（会話欄・レビューの本文・インラインのレビューコメント）のうち `andon gate: ` で始まらないもの（ゲートのコメントは同じ token で書かれる）と本文の無いもの・インラインでは `user.type` が `Bot` のものを除いたものと、PR の commit のうち `report.json` の `commit`（担当の最後の報告）より後のもの。`report.json` の `commit` が PR に無ければ commit は数えない。どちらも無ければ書かない | `comments[]`（`author`・`at`・`body`。インラインのレビューコメントは `file`・`line` も）・`human_commits[]`（`sha`・`headline`・`files[]`） |
| `override`（指摘） | `report.json` の `waived` ＋ PR の `state` | PR が `MERGED` で、`waived` のうち `must_fix`（Chief of Staff が書いたもの。無ければ全部）以上の重さのもの。ゲートは担当がレビュアーを覆したものを人へ回す（`needs_human`）ので、マージされたなら人が通した | `finding`（`severity`・`file`・`line`）・`reason` |
| `override`（問い） | `questions.yaml` ＋ `library/questions/<n>.json` | 答えがあるゲートの問い（`from: gate`。`needs_human` への人の答え）。推奨案どおりの答えも拾う | `question`・`recommended`・`answer` |

- 人の commit を author の email で見分けない: 担当も repo の持ち主の noreply アドレスで commit するので、email では人と担当が分かれない（[decisions/2026-11-09-corrections-json.md](decisions/2026-11-09-corrections-json.md)）。
- 拾うのは事実だけで、手直しの意味は読まない（答えが推奨案と同じ意味の別の言い方でも `answer` に入る。振り返りが読んで捨てる）。多めに拾い、少なく拾わない。

## 15. 信号の記録（`<slug>.signals.json`）

外の出所（GitHub の main の CI・issue、Sentry・Linear・Cloudflare）から読んだ信号の記録（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 6・16）。**置き場は台帳の外の `~/.local/state/andon/dispatcher/<slug>.signals.json`**（`XDG_STATE_HOME` があればその下。dispatcher の控えと同じ所）で、プロジェクトに1ファイル。書き手は dispatcher（`andon signal collect`）だけ、読み手はトリアージ・観察の課題の担当・measure・ボード。どの出所を読むかは `project.json` の `signals`（§11）。

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
   "episodes": 1, "delivered_episode": 0}]}
```

| キー | 要るか | 意味 |
|---|---|---|
| `signals[]` | 必須 | 信号1件ごとに1つ（`fingerprint` は重ならない）。まだ何も見ていなければ空 |
| `source` | 必須 | 出所（§11 の `signals[].source` と同じ値） |
| `project` | 必須 | プロジェクトの slug |
| `key` | 必須 | 出所の中の id。CI は `<repo>/<workflow>@main`、issue は `<repo>#<n>`、Sentry・Cloudflare・Linear はその issue の id |
| `url` | 必須 | 人と担当が読む所（`https`） |
| `title` | 必須 | 出所の題をそのまま1行（制御文字なし）。**信用しない**: 外の人の文なので、描くときは「信用しない」の囲いに入れる |
| `weight` | 必須 | 出所の `weight`（§11）の写し |
| `fingerprint` | 必須 | `source` と改行と `key` をつないだ文字列の SHA-1（小文字の16進40字）。Sentry・Cloudflare は出所のまとめ（issue）が `key` なので、まとめ方は出所に従う |
| `first_seen`・`last_seen` | 必須 | 最初と最後に見た時刻（UTC の `YYYY-MM-DDTHH:MM:SSZ`） |
| `count` | 必須 | 見た巡の数（1 以上） |
| `active` | 必須 | 最後の巡で見えたか（`false`: CI が通った・issue が閉じた・resolved） |
| `episodes` | 必須 | 出来事の数。最初は 1、消えてまた出るたびに 1 増える |
| `delivered_episode` | 必須 | 通知 `signal`（§6）で届けた出来事の番号。届けていなければ 0（`later` は常に 0） |
| `release` | 任意 | 見えたリリースの名前（マージ commit の SHA。§16）。出所が持つときだけ |

- **本文・スタックトレースは持たない**（記録を大きくしない・外の人の文を写さない）。担当が読むときに `url` から取る。

`andon signal collect` の当て方（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 3・8・9）:

- 出所と repo の組ごとに1回読み、見えたものは `count` を 1 増やして `last_seen`・`url`・`title`・`weight`・`release` を今の値にする。初めてなら `count`・`episodes` が 1、`delivered_episode` が 0。`active: false` のものがまた見えたら `active: true` にして `episodes` を 1 増やす。その組の記録で見えなかったものは `active: false`。ただし読んだ範囲で結果が分からなかったもの（下の CI の 100 件・issue の上限）は変えない（読めていないものを「消えた」にしない）。**`delivered_episode` は collect が触らない**（書くのは届けた dispatcher）。
- `github-ci`: main の CI（`gh api repos/<owner>/<repo>/actions/runs?branch=main&status=completed`。pull request の run は除く）を workflow のファイルごとに見て、最新の結果（成功・失敗・時間切れ・起動の失敗。取り消しと飛ばしは数えない）が失敗なら信号。読むのは新しい 100 件で、その中に結果の出た run の無い workflow は今の記録のまま。`key` は `<repo>/<workflow のファイル名>@main`、`title` は `<workflow の名前>: <run の題>`、`release` は run の `head_sha`（main の commit）。
- `github-issues`: open の issue（`gh issue list --state open`。`filter` があれば `--search` にそのまま。上限 1000 件で、上限まで返ったら見えなかった issue も今の記録のまま）。`key` は `<repo>#<番号>`。
- 読めなかった組（API が落ちた・`gh` の資格情報が無い）は、その組の記録を変えずに飛ばして1行出す（終了コード 1。次に打てば読み直す）。1組も読めなければ記録を書かない。`signals` が無い・`project.json` が無ければ何も読まず書かない。Sentry・Linear・Cloudflare は読む command がまだ無く、トークン（`andon signal auth`）が無ければそう、あれば「まだ無い」と1行出して飛ばす（止まらない）。
- 出所へは書かない（GitHub は `gh` の token が書ける token でも、打つのは読む API だけ）。
- 機械が読むのはこの schema のキーだけ。`title` で判定しない。

## 16. リリースの名前（案件の CI との約束）

エラー → PR → 課題を辿れるように、**案件の CI はリリースの名前をマージ commit の SHA（40字）にする**（[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 17）。Sentry の release、Cloudflare の Worker の version の tag がこれに当たる。付けるのは案件の CI の仕事で、Andon は形だけ約束する（command は足さない）。

- 信号の記録（§15）は出所が持てば `release` にその SHA を写す。
- 辿るのは担当（トリアージ・観察）で、手で: `gh pr list --search <sha> --state merged` で PR を、台帳の `pr` キーで課題を引く。
