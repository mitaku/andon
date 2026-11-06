# Andon の機械向けの約束

- 状態: 設計の提案（2026-11-04 起草、2026-11-05 書き直し、2026-11-05 schema に合わせて直した）。**正は `schema/`**（JSON Schema。draft 2020-12）で、この文書はそれを人が読めるように説明したもの。食い違えば schema が正。この文書の ```` ```json ```` の例は全部、`tests/schema.sh` がその節の schema に通す。

| 節 | schema |
|---|---|
| §2 台帳の項目 | `schema/task-keys.schema.json` |
| §3 指摘 | `schema/findings.schema.json` |
| §4 報告の機械向けの部分 | `schema/report.schema.json` |
| §5 問い | `schema/question.schema.json` |
| §6 通知 | `schema/notification.schema.json` |
| §7 ゲートの条件表と記録 | `schema/gate.schema.json`（条件表か記録のどちらか一方に合う） |
| §11 委譲 | `schema/delegation.schema.json` |

- 読み手: command（`andon task …`・`andon gate`・`andon triage`・`andon delegate`）・クーリエ（`andon courier`）・フック・ボード（人の Interface）・振り返り。語は [design.md](design.md) §0.1。
- 台帳の CLI をこの文書では `kanban` と書く（[design.md](design.md) §3.5。仮設の工場では `deck`。同じ command）。
- 台帳の形式（ledger-format v2。別プロジェクトの持ち物）は**変えない**。Andon のキーは「Any other key is allowed and kept」の上に置く。形式そのものに触れたいことは design.md §3.2 の相談に並べた。台帳の CLI と形式を Andon に移したら（§3.5）、「変えない」は「形式の版で変える」になる。それまでこの文書は v2 の上に乗る。

## 0. 原則

1. **Markdown を読まない。** 依頼書・報告・レビューの報告・skill は人と AI のもの。機械が判定に使う値は、ここに並ぶ JSON／YAML のキーにだけ持つ（人の決定 2026-11-04。仮設の工場で Markdown を正規表現で読む形は端が尽きなかった）。
2. **無い・読めない・知らない値は安全側に倒す。** ゲートは `needs_human`、クーリエは送らない／起こさない、担当の command は止まって理由を1行出す。黙って既定に落とさない（既定があるキーはこの文書に「既定」と明記する）。
3. **1つの約束に1人の書き手。** `task.yaml` の予約キーは Chief of Staff、`findings.json` はレビュアー、`report.json` は担当、問いの JSON は問うた者、通知とゲートの記録はクーリエ、委譲は人。履歴は `log.jsonl` の `by`（台帳）・端末のラベル・git の author（委譲）で照合する。
4. **読み手は知らないキーを無視し、書き手は知らないキーを保つ**（ledger-format の互換の規則と同じ）。キーを増やすのは版を上げない。キーを消す・意味を変えるのは版を上げ、`schema/` の `$id` と `docs/decisions.md` に書く。
5. **置き場は台帳の中。** 担当が worktree の外に書いてよい唯一の場所 `tasks/<id>/library/` を、機械向けの置き場にも使う。作業側が別のマシンのときは `<worktree>.task/outbox/` に書き、クーリエが引いて同じ場所に置く（design.md §1.5）。委譲だけは建屋の repo（§11。書き手が人だから）。
6. **形は小さく、テストで固定する。** キーは名前を決めたものだけ（余分なキーは書かない。読み手は無視する）。schema は書き手の約束なので、知らないキーを拒む（`additionalProperties: false`。台帳の項目 §2 と委譲の値を除く）。`tests/schema.sh` が例と schema を突き合わせる。command（`andon task`・`andon gate`・`andon delegate`）の出力も `tests/schema.sh` が schema に通す（`tests/andon-task.sh`・`tests/andon-gate.sh` に書かせる）。

## 1. 置き場と書き手

```
projects/<slug>/.flightdeck/
  config.yaml                     coordinator（kanban config set。Chief of Staff の端末）
  tasks/<id>/
    task.yaml                     台帳の項目 ＋ Andon の予約キー（§2。Chief of Staff が kanban set）
    brief.md                      人・AI 用（Settings は写し。機械は読まない）
    report.md                     人・AI 用（機械は読まない）
    questions.yaml                台帳の問い（§5。text は人向け）
    library/
      report.json                 担当（§4）
      findings.json               レビュアー（§3。レビュアーのタスクの側）
      questions/<n>.json          問うた者（§5）
      gate/<head sha>.json        クーリエ（§7。ゲートの記録）
      corrections.json            クーリエ（finish のとき。人の手直し: 問いの答え・PR の上の直し・指摘の覆し。design.md §1.9 の (b)）
  log.jsonl                       台帳の CLI が書く。by で履歴を照合
projects/<slug>/delegation.json   人（§11。建屋の repo。git の履歴が照合の元）
projects/<slug>/delegation.md     command が生成する人用の一覧（機械は読まない）
~/.local/state/andon/courier/<slug>.json   クーリエの控え（§6。台帳の外）
```

## 2. 台帳の項目（`task.yaml` の Andon の予約キー）

依頼書の `## Settings` の**正**。Chief of Staff が `kanban set <id> review=on must_fix=major …` で書く。依頼書の欄は人が読む写し。

| キー | 値 | 無いとき | 書き手 | 読み手 | 意味 |
|---|---|---|---|---|---|
| `review` | `on` / `off` | ゲートは `needs_human`（レビューの照合を回さない）。クーリエは「レビューを頼まれても起こさない」 | Chief of Staff | ゲート・クーリエ・担当 | 独立レビューを工程に入れるか |
| `must_fix` | `critical` / `major` / `minor` / `trivial` | ゲートは `needs_human` | Chief of Staff | ゲート・担当 | この重さ以上の指摘は担当が直す |
| `reviewer` | プロファイルの名前（`codex-review` / `claude-review`） | 既定: 実装のプロファイルと別ブランド（Claude → `codex-review`、Codex → `claude-review`） | Chief of Staff | クーリエ（起こす）・ゲート（別ブランドの照合） | レビュアーのプロファイル |
| `review_rounds` | 1 以上の整数（を表す文字列。下の注） | 既定: `"2"` | Chief of Staff | クーリエ・担当 | 頼める回数の上限。超えた回は `report.json` の `review_request.extra` で頼み、起こすかはクーリエではなく Chief of Staff が決める |
| `merge` | `auto` / `person` | 既定: `person`（安全側） | Chief of Staff | クーリエ・ゲート | `auto`: ゲートが approve し、委譲（§11）の `merge` がそのレベルを許していればクーリエがマージし finish する。`person`: approve でも止まり、人に通知する。**委譲の範囲を広げることはできない**（狭めるだけ） |
| `machine` | 端末の土台の machine の名前 | 既定: 指揮側と同じマシン | Chief of Staff | `andon task start`・クーリエ | 作業側（担当・レビュアー）を起こすマシン |

`kanban show <id> --json` の `.task`（Andon の予約キー以外は台帳の形式のキー）:

```json
{"id": "A-12", "title": "andon task ready を足す", "status": "doing",
 "review": "on", "must_fix": "major", "reviewer": "codex-review",
 "review_rounds": "2", "merge": "person", "machine": "vps"}
```

- **値は全部文字列**。`kanban set` は値を文字列で書くので、`review_rounds` も `"2"`（1 以上の整数を表す数字の列）。YAML に手で `review_rounds: 2` と書いた数は schema を通らない（読み手は「無い」として既定を使う）。
- 台帳の形式は知らないキーを許して保つので、この schema は**知らないキーを拒まない**。どれも必須ではない（無いときの振る舞いは上の表）。
- **履歴の照合**: ゲートとクーリエは、`log.jsonl` でそのキーを最後に変えた行の `by` が `terminal:<config.yaml の coordinator>` で、その値が今の `task.yaml` と同じときだけ信じる。違えば `needs_human`。`coordinator` が無ければ信じない。
- **レビュアーのタスク**（`parent` が元のタスクで `profile` が実装と別ブランドの `*-review`）は、作成の行（最初の `add`）の `by` が `terminal:<coordinator>` か `terminal:courier`（クーリエが起こしたもの）のときだけ、ゲートと `andon task ready` がレビューに数える。
- 値の綴りは小文字（`machine` を除く）。知らない値は「無い」と同じ。
- 読み手が `kanban show <id> --json` の `.task` から取る（ファイルを直接読まない）。

## 3. 指摘（`findings.json`）

レビュアーのタスクの `library/findings.json`（キーはこの名前だけ）:

```json
{"commit": "2a17f63e53ac7f9554d78782e2bfc4d079454455",
 "findings": [{"severity": "major", "file": "package/lib/andon/task", "line": 120}]}
```

| キー | 値 | 意味 |
|---|---|---|
| `commit` | 40桁・小文字 | 見た commit。PR の head と違えば stale |
| `findings[].severity` | `critical` / `major` / `minor` / `trivial` | 重さ。起きる条件を具体的に示せないものは `trivial` |
| `findings[].file` | repo からの相対パス（空でなく、制御文字・改行が無く、`/` で始まらず、`..` の段を含まない） | — |
| `findings[].line` | 1 以上の整数 | — |

- 無い・JSON でない・形が違う → missing。`commit` 違い → stale。報告の Markdown は読まない。
- 書き手はレビュアー。読み手: ゲート（`must_fix` 以上の件数）・担当（振り分け。`must_fix` 以上を直し、未満は `report.json` の `waived` に理由つきで残す）・振り返り（繰り返し出た指摘を観点へ）。
- 将来キーを足すなら（例: `title`・`kind`）版を上げずに足せる（原則4）。最初の版では足さない。

## 4. 報告の機械向けの部分（`report.json`）

担当のタスクの `library/report.json`。担当が書き直すたびに全体を書く（`report.md` と同じ）。書き手は担当。`kanban report` の直前に書く——クーリエは `log.jsonl` の `{"report": "write"}` を合図に `report.json` を読む（新しい合図の種類を足さない）。

```json
{"commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "pr": "https://github.com/<owner>/andon/pull/12",
 "review_request": {"round": 1, "limit": 2, "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "previous": null, "reviewer": "codex-review", "extra": false},
 "waived": [{"severity": "minor", "file": "package/lib/andon/task", "line": 88, "reason": "起きる条件がこの repo に無い"}],
 "acceptance": [{"id": "A1", "result": "pass", "evidence": "bash tests/andon-task.sh"}],
 "next": [{"to": "person", "text": "PR #12 を読む（レベル C: package/gate.json）"},
          {"to": "chief-of-staff", "text": "マージ後に andon task finish A-12"},
          {"to": "task", "text": "lib/andon/triage が足すタスクにも review・must_fix を書く"}],
 "remember": ["機械が判定に使うものは Markdown に埋めない"],
 "agents_md": ["テストは bash tests/*.sh"]}
```

| キー | 要るか | 読み手 | 意味 |
|---|---|---|---|
| `commit` | 必須 | クーリエ・ゲート | 書いた時点の HEAD（40桁・小文字）。`andon task ready` が `HEAD` ＝ `commit` ＝ PR の head を確かめる |
| `pr` | PR を出したら必須（schema では任意。確かめるのは `andon task ready`） | ゲート・クーリエ | 今の報告の `PR:` 行の代わり。ゲートはこれだけを読む。`https://github.com/<owner>/<repo>/pull/<n>` で、owner と repo は `/` を除く印字できる ASCII |
| `review_request` | レビューを頼むときだけ。中の6つのキーは全部必須（`previous` は初回 `null`、`round`・`limit` は 1 以上の整数） | クーリエ | クーリエが `kanban add --parent <id>` ＋ detached worktree ＋ `hc run <reviewer>` で起こす。`round` ≤ `limit` でなければ起こさない（`extra: true` なら Chief of Staff の判断待ち）。`previous` があれば再レビュー（差分 `previous..commit` だけ）。`commit` が PR の head と違えば起こさない |
| `waived` | 任意 | トリアージ・振り返り | 見送った指摘（`findings.json` と同じ3つ ＋ `reason`） |
| `acceptance[]` | 任意（Chief Engineer の文書に受け入れ条件があるとき） | ゲート・振り返り | `{"id", "result": "pass" \| "fail" \| "manual", "evidence"}`。`evidence` はテストの command かユースケースの実行の跡のパス。`fail` があれば `fix`、`manual` があればレベルを1つ上げる（design.md §1.9 の (a)）。足しても版は上げない（原則4） |
| `next[]` | 任意 | トリアージ | `to` は `person` / `chief-of-staff` / `task`（次のタスクに切るもの） |
| `remember[]`・`agents_md[]` | 任意 | トリアージ・振り返り | 人用の報告の `## Remember`・`For AGENTS.md:` の機械向けの写し |

- 担当の `andon task ready <id>`: `report.json` を検証（必須キー・`commit` ＝ `git rev-parse HEAD` ＝ `gh pr view --json headRefOid`・`review: on` なら最新のレビュアーのタスクの `findings.json` が `commit` を指す）してから `kanban review <id>`。合わなければ終了コード 1 と理由1行。
- 担当の `andon task review-request <id>`: `review_request` を `report.json` に書き（`round` は前の回から数える）、`kanban report` する。担当は `## レビュー依頼` の文を書かなくてよい。
- 作業側（別のマシン）の担当は同じ command を打つ。command は `<worktree>.task/outbox/` に書くだけで、クーリエが引いて指揮側の `library/` に置き、`kanban report`／`review` を打つ（by はクーリエ。design.md §3.2 F5）。

## 5. 問いの形

人が「文脈なしで読めて、1通で答えられる」形。2つに書く:

1. `kanban ask <id> "<text>" --kind <k>`: `questions.yaml` の `text` は人向けの描画（下の JSON から command が作る。何の判断か・事実・理由・選択肢・推奨案・あと何問）。`kind` は台帳の4種（`business` / `users` / `promise` / `irreversible`）。5つ目（ラインの変更）は台帳が受けないので省く（相談 F3）。
2. `tasks/<id>/library/questions/<n>.json`（`n` は台帳が付けた番号）:

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
| `kind` | 必須 | `business` / `users` / `promise` / `irreversible` / `line`（判断5種。design.md §1.3） |
| `question` | 必須 | 1行（改行を含まない）。問いそのもの |
| `facts[]`・`reason` | 任意 | 事実と理由。写さず、パスか URL で指す。委譲の範囲外で出す問いは `facts` に「委譲 `<scope>: <値>` の外」を1行 |
| `options[]` | 必須（2つ以上） | `label` と `pick`。**先頭**が `pick: true`（推奨案）で、2つ目からは `pick` を書かないか `false` |
| `remaining` | 任意 | この問いのあとに続く問いの数（「あと N 問」。0 以上の整数） |
| `from` | 任意 | `task` / `gate` / `retro` / `chief-of-staff` / `chief-engineer` |

- 書き手: 担当（`andon task ask <id> <json>`）、クーリエ（ゲートの `needs_human`。`from: gate`）、Chief of Staff・Chief Engineer（人に出すとき）。command が JSON を検証して `text` を描き、`kanban ask` を打つ。
- 読み手: Chief of Staff（`kanban questions --json` と並べて読む）・通知の口（将来）・ボードの返事欄（相談 F2）。答えは今までどおり `kanban answer`・`kanban received`。

## 6. 通知

クーリエの中では JSON。届けるときだけ `<news>` の文に描く（文は JSON から作る。文から推さない）。拾い方・届ける経路・離席の判定の理由は [courier.md](courier.md)。

```json
{"at": "2026-11-04T09:00:00Z", "project": "andon", "task": "A-12", "kind": "review",
 "detail": {"report": "/workspace/projects/andon/.flightdeck/tasks/A-12/report.md", "pr": "https://github.com/<owner>/andon/pull/12"}}
```

| `kind` | 元 | `detail` |
|---|---|---|
| `report` | `log.jsonl` の `{"report": "write" \| "append"}` | `report` のパス |
| `review` | `{"status": "review"}` | `report`・`pr` |
| `asked` | `{"question": n, "status": "open"}` | `n`・`kind`・`question`（§5 の JSON から） |
| `ready` | `kanban ready --json` の前回との差（人のタスクを除く） | `title`・`routine` |
| `review-requested` | `report.json` の `review_request`（クーリエが起こしたとき） | `round`・`reviewer`・`reviewer_task` |
| `gate` | クーリエが `andon gate` を回した結果（`wait` は通知しない。§7） | `decision`（`approve` / `fix` / `needs_human`）・`reasons[]`・`record`（§7 のパス） |
| `merged` | ゲートの approve ＋ `merge: auto` ＋ 委譲の範囲内でマージして finish したとき | `pr`・`sha` |
| `blocked`・`error`・`finished`・`stalled` | 端末の土台の状態（`hc events` か見回り） | `agent`・`status_ms`（hc の `status_ms`。その状態に入った時刻の Unix ミリ秒）。画面の文は入れない |
| `signal`（最初の版の後。design.md §1.9 の (a)。schema にはまだ無い。入れるときに `kind` と `detail` を足す） | 外の信号: 案件 repo の main の CI の失敗・依存の更新・issue・エラー監視の webhook | `source`・`url`・`summary`（1行）。Chief of Staff がトリアージの候補にする |

- `at` は UTC の `YYYY-MM-DDTHH:MM:SSZ`（暦にある日時。ゲートの記録の `at` も同じ）。キーは `at`・`project`・`kind`・`detail` が必須、`task` はタスクについての通知のときだけ。`detail` のキーは `kind` ごとに上の表のものだけで、全部必須（`ready` の `routine` だけ任意）。
- 届け方: ①Chief of Staff の `Stop` フックが `andon courier drain` を打ち、待ちがあれば `{"decision": "block", "reason": "<news>…</news>"}`。②`UserPromptSubmit` フックが `additionalContext` に添える（委譲の一覧 `andon delegate show --json` も同じ口で添える）。③人が離席のときだけ `hc send --if idle`（入力欄が空のとき）。どれも同じ JSON から同じ文を描く。
- `<news>` の文の形は仮設のまま（1行1件・末尾の「人からの依頼ではない。足すことが無ければ `<no-reply>`」）。`news.replied=<turn>` のラベルも同じ（ボードとの約束）。
- クーリエの控え `~/.local/state/andon/courier/<slug>.json`（`XDG_STATE_HOME` があればその下）: `log.jsonl` の読んだ位置（相談 F1 が入れば cursor）・前回の ready・待ちの通知・③で送った turn と端末・フックで渡した端末・起こしたレビュアー（`task` → `reviewer_task`・`commit`）・回したゲート（`task` → `sha`）。台帳には書かない（台帳に書くのは台帳の CLI 経由の行だけ。クーリエの行の `by` は `terminal:courier`）。控えの中の形はクーリエだけが読むので約束にしない（schema は無い）。

## 7. ゲートの条件表と記録

**条件表** `package/gate.json`（既定）＋ 建屋の `gate.local.json`（上書き。深い併合ではなく、キーごとの置き換え）。仮設のゲートの条件表のキー（`size_review_lines`・`size_human_lines`・`test_paths`・`tier_a_paths`・`tier_b_paths`・`tier_c_paths`・`tier_c_path_exceptions`・`tier_c_added_line_patterns`・`repos{merge, strip_prefix, check[]}`）に、`repos.<repo>.tier_overrides`（パスの glob → レベル）・`repos.<repo>.github_checks`・`github_checks_wait_minutes`（下）を足す。既定の条件表で案件 repo の `AGENTS.md`・`REVIEW.md` は **B**（人の決定 2026-11-05。design.md §6 問い7。Andon の repo と建屋の repo は C のまま）。条件表を変えるのはレベル C（人）。

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
- `github_checks_wait_minutes`: `required` の repo で checks が1つも無いとき、タスクが `review` に入った `log.jsonl` の行の `at` からこの分だけ待ち、過ぎたら `needs_human`（push の直後で checks がまだ付いていないのと、CI が無いのとを分ける）。

**記録** `tasks/<id>/library/gate/<head sha>.json`（クーリエが書く。`--dry-run` では書かない）:

```json
{"at": "2026-11-04T09:10:00Z", "id": "A-12", "pr": "https://github.com/<owner>/andon/pull/12", "head": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08",
 "tier": "B", "tier_reasons": ["package/lib/andon/task は tier_b_paths"], "changed_lines": 212,
 "checks": [{"cmd": "bash tests/*.sh", "ok": true}],
 "github_checks": {"state": "success", "names": []},
 "review": {"state": "current", "task": "A-13", "commit": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "must_fix_open": 0, "waived": 1},
 "acceptance": {"pass": 3, "fail": 0, "manual": 0},
 "delegation": {"merge": "B", "ok": true},
 "decision": "approve", "reasons": [], "merged": true}
```

- キーは全部必須。`review.state` は `current`（`findings.json` の `commit` が `head`）/ `stale`（違う）/ `missing`（無い・読めない）/ `off`（タスクの `review: off`）。`current`・`stale` では `task` と `commit` が必須、`missing`・`off` では2つとも書かない。`delegation.merge` は委譲が無ければ `none`。
- `decision` と `reasons[]` は決定式（純粋関数）の出力そのまま。PR のコメントと commit status（`andon/gate`）には同じ内容を写す（記録の正は台帳の側）。
- `github_checks`: PR の head の GitHub の checks（GraphQL の `pullRequest.headRefOid` と head の commit の `statusCheckRollup.contexts` を、100件ずつ全ページ読む。ページの間に head が動いたら判定しない。ゲート自身の commit status `andon/gate` は数えない。同じ check〈CheckRun は app・workflow の id〈表示名でなく GraphQL の `Workflow.id`。別ファイルの同じ表示名の workflow を束ねない〉・名前、status は context〉の実行が複数あれば、最新〈CheckRun は `databaseId`、status は `createdAt`〉の1つだけを数える＝再実行の前の failure を数えない。新旧を判別できない〈同じ時刻・同じ id〉ときは悪い方を残す）を1つの状態にまとめたもの。`state` は一番悪いもの: `failure`（failure・cancelled・timed_out・startup_failure・status の error）＞ `unknown`（action_required・stale・知らない値）＞ `pending`（終わっていない・status の pending・expected）＞ `success`（success・skipped・neutral。neutral は GitHub のブランチ保護と同じく通す）。1つも無ければ `none`。`names` はその状態にした checks の名前（`success`・`none` では空）。
- GitHub の checks の決定: `failure` → `fix`（担当が直せるもの）。`unknown` → `needs_human`。`pending`、または `required` の repo で `none` → **`wait`**（ほかに止める理由が無いときだけ。止める理由があればそちらを返す）。`none` のまま `github_checks_wait_minutes` を過ぎたら `needs_human`。`pending` は時間で人へ上げない（走り終われば success か failure になる）。checks が通っていないあいだは条件表の `check` を回さない。
- **`wait` は記録に書かない**: `andon gate` は記録の形を stdout に出して終了コード 5 で返すが、`gate/<head>.json`・PR のコメント・commit status は書かない（見回りのたびに PR を埋めない）。読み手（クーリエ）は控えに書かず、次の見回りでゲートから回し直す。
- `delegation`: 読んだ委譲の `merge` の値と、判定のレベルがその中か（§11）。外なら approve でも `decision: needs_human`、`reasons[]` に `delegation: merge=B < tier C`。
- 読み手: クーリエ（approve → `merge: auto` ＋ 委譲の範囲内なら、マージの直前に PR の head と GitHub の checks を読み直し、head が記録の `head` のまま・checks が通る状態のときだけマージ → `andon task finish`。違えば打たず、次の見回りでゲートから回し直す。残る隙間: マージの直前の読み直しと `gh pr merge` の間に、同じ head のまま checks が走り直した・落ちたときは止められない（`--match-head-commit` が固定するのは head だけ）。サーバーの側で checks を強制できるのはブランチ保護（ruleset）だけで、private の repo の Free のプランでは使えないので、この隙間は受け入れる。public にしたら ruleset で既定ブランチに required checks を付けて塞ぐ（public にする手順に入れてある）。wait → 控えに書かずに次の見回りで回し直す。fix → 担当へ `hc send`。`needs_human` → §5 の問いを `from: gate` で出す）・振り返り（fix の理由・レベルの分布）・人。

## 8. 端末の土台（hc）との約束

| 何 | 形 | 書き手 | 読み手 |
|---|---|---|---|
| 担当・レビュアーの端末の名前 | `<slug>-<id を小文字>`（`a-z 0-9 - _`、32字まで） | `andon task start`・クーリエ | クーリエ・ボード |
| Chief of Staff の端末の名前 | `<slug>-cos`（cwd は台帳の root） | `andon chief-of-staff` | `andon chief-of-staff`（動いているものを使うときに名前と cwd を照らす） |
| ラベル | `role=chief-of-staff \| courier \| chief-engineer`・`project=<slug>`・`task=<id>`・`title=<題>`・`news.replied=<turn>` | command・クーリエ | ボード・クーリエ・フック（自分のタスクを引く） |
| Herdr App の置き場のラベル | `herdr.space=<id>`・`herdr.tab=<id>`（持ち主は Herdr App。id は command が決める: Chief of Staff は `andon-<slug>`・`andon-<slug>-cos`、Chief Engineer は `andon-<slug>`・`andon-<slug>-<id>`、担当は `andon-<slug>-work`・`andon-<slug>-<id>`（machine に依らない。`machine` 付きのタスクを `andon task start` はまだ起こさない。ADR-0002 決めたこと 19）、レビュアーは担当の端末（台帳の `agent`）の値を `hc get --json` で読んで写す（読めなければ付けない）。`where` は App が書く出力で、Andon は読まず書かない。[ADR-0002](adr/0002-herdr-spaces.md)） | `andon chief-of-staff`・`andon chief-engineer`・`andon task start`・クーリエ。`andon task finish` は触らない（端末の kill・rm で tab が空になって消える） | Herdr App |
| 人に端末を指す言い方 | `<Space 名> / <tab 名>`（App の `where` と同じ形。例 `sample-site / CoS`・`sample-site / CE A-3 Billing の設計`・`sample-site work / A-12 ログインの直し`）。address は末尾に `（t7）` と添えるだけ。Andon の command は `hc attach` を打たず、最後に出た場所をこの形で出す | `andon chief-of-staff`・`andon chief-engineer`・`andon task start`・Chief of Staff | 人 |
| template の名前 | プロファイルの名前そのもの（`chief-of-staff`・`chief-engineer`・`claude`・`sonnet`・`fable`・`codex`・`codex-review`・`claude-review`） | boot が生成 | `andon task start`（`hc templates` で存在を確かめる） |
| 台帳の `agent` | 端末の address。作業側が別のマシンなら `<machine>/t7`（相談 F4） | `kanban start`（担当）・クーリエ（作業側の代筆） | クーリエ・ボード |
| 起こし方 | `hc run --bg --outlive-parent --no-notify --name … --label … --cwd … [--on <machine>] <profile> "<最初の3行>"` | `andon task start`・クーリエ | — |
| 状態 | `hc events`（クーリエ）か `hc ls --json`（見回り）。`working`・`idle`・`done`・`blocked`・`error`・`finished` | hc | クーリエ |

## 9. フック

managed settings の `hooks`（boot が `package/settings.json` から生成）。フックは**クーリエと command を呼ぶだけ**で、判断を持たない。

| 誰の | イベント | 打つもの | 返すもの |
|---|---|---|---|
| Chief of Staff | `Stop` | `andon courier drain`（端末のラベル `role=chief-of-staff`・`project=` から slug を引く） | 待ちがあれば `{"decision": "block", "reason": "<news>…</news>"}`。無ければ何も返さない。前にフックで渡した `<news>` の turn の終わり（入力の `prompt_id`・`session_id` が合う）なら確かめ済みにし、答え（`last_assistant_message`）が `<no-reply>` だけなら `news.replied=<turn>` を書く。合わなければ渡した通知を1回だけ届け直す |
| Chief of Staff | `UserPromptSubmit` | `andon courier drain --context`（中で `andon delegate show --json` を読む） | `{"hookSpecificOutput": {"hookEventName": "UserPromptSubmit", "additionalContext": "<news>…</news>\n<delegation project=\"<slug>\">…</delegation>"}}`（待ちが無ければ委譲だけ）。前にフックで渡した `<news>` の turn の終わりを見ていなければ（中断）、その通知を1回だけ届け直す |
| 担当 | `Stop` | `andon task stop-check`（端末のラベル `task=` から id を引く） | 台帳が `doing` で、前回の通過（無ければ `doing` に入った行）以後に `kanban report`／`review`／`ask` のどれも `log.jsonl` に無く、待ちでもなければ `{"decision": "block", "reason": "<id> は doing のまま、報告も問いも台帳に無い。…"}`。待ち（通す）は、自分の問いが開いたまま（その問いの最後の行が `open`）か、`report.json` に `review_request` があり、その `commit` を指すレビュアーの `findings.json` がまだ無いとき。答え（`answered`）・レビューが届いたあとは待ちではない。続けて止めるのは3回まで。控えが書けなければ通す。レビュアーも同じ |

- フックの実体は `package/hooks/stop`・`user-prompt-submit`（image では `/opt/andon/hooks/`）。どの端末でも同じフックが走り、役は command がラベルで見分ける（どちらでもなければ何も出さない）。

- 要実測: `Stop` の `decision: block` で続きの turn が始まること、`UserPromptSubmit` の `additionalContext` が文脈に入ること、Codex の同等物（最初の版は Claude Code だけ）。
- フックが落ちたとき（command が無い・台帳が見つからない）は**通す**（止めない。クーリエのログに1行）。フックは守りの1枚で、唯一の守りではない（`andon task ready` とクーリエの照合が残る）。

## 10. 互換と版

- `schema/*.schema.json` の `$id` に版（`https://andon.example.com/schema/<名前>/<版>`。例 `https://andon.example.com/schema/report/1`。URL は識別子で、取りに行かない）。`tests/schema.sh` が `$id` の形・draft 2020-12・`description` と識別子が英語であることも確かめる。読み手は `$id` ではなく**キーの有無**で振る舞う（原則4）。
- キーを足す: 版を上げない。`docs/contract.md` と `tests/schema.sh` の例を足す。
- キーを消す・意味を変える: 版を上げ、古い版を読む側を1版だけ残す（建屋の `FROM` の版上げが追いつくまで）。
- ledger-format の `version` が上がったら、Andon の command は台帳の CLI の言うとおりに止まる（ledger-format の規則）。

## 11. 委譲（`delegation.json`）

オーナー（人）が Chief of Staff・クーリエ・担当に何をどこまで任せたか（design.md §1.10）。**置き場は建屋の repo の `projects/<slug>/delegation.json`**（台帳の外。書き手が人だけだから）。人用の一覧 `delegation.md` を同じ場所に command が生成する（機械は読まない）。

```json
{"version": 1, "project": "sample-site", "preset": "hobby",
 "grants": [
   {"id": "d1", "scope": "publish", "value": "public",
    "by": "owner", "at": "2026-11-05", "until": null,
    "evidence": "2026-11-05 の対話「サイトは勝手に出してよい」（projects/sample-site/master/decisions.md #12）",
    "revoked": null},
   {"id": "d2", "scope": "deploy", "value": "none",
    "by": "owner", "at": "2026-11-05", "until": "2026-12-31",
    "evidence": "移行中はデプロイを人が見る",
    "revoked": {"at": "2026-11-20", "by": "owner", "reason": "移行が済んだ"}}]}
```

| キー | 要るか | 意味 |
|---|---|---|
| `version` | 必須 | この形式の版（整数の `1`） |
| `project` | 必須 | プロジェクトの slug（置き場と一致しなければ無委譲） |
| `preset` | 任意 | 既定の型 `hobby` / `work` / `andon`（design.md §1.10 の表）。無ければ `work` |
| `grants[]` | 任意 | 型からの足し引き。後の行が前の行を上書きする（同じ `scope` の有効な grant のうち、`at` が最新のもの） |
| `grants[].id` | 必須 | `d1` … 一意（一意かは command が確かめる） |
| `grants[].scope` | 必須 | `merge` / `deploy` / `publish` / `irreversible` / `external-requests` / `profile` / `package-update` / `line-change` |
| `grants[].value` | 必須 | `scope` ごとの値（design.md §1.10 の表）。`merge`＝`none`/`A`/`B`/`C`、`deploy`＝`none`/`preview`/`production`、`publish`＝`none`/`internal`/`public`、`irreversible`＝`delete-branch`/`force-push`/`rewrite-history` の配列（`delete-remote` は書けない）、`external-requests`・`profile`・`package-update`＝真偽値、`line-change`＝`none`/`project`/`plant` |
| `grants[].by` | 必須 | 付与した人。文字列の `owner` だけ（実際の人は建屋の `andon.toml` の `owner`。照合は commit の author で） |
| `grants[].at` | 必須 | 付与した日（`YYYY-MM-DD`。暦にある日。`2026-02-30` やうるう年でない年の `02-29` は通らない） |
| `grants[].until` | 任意 | 期限（`YYYY-MM-DD`）。過ぎていれば無いものと同じ。`null` は無期限 |
| `grants[].evidence` | 必須 | 根拠。人の発言の記録へのパスか引用（写さず指す） |
| `grants[].revoked` | 任意 | 取り消し `{at, by, reason}`（3つとも必須。`null` は取り消していない）。あれば無いものと同じ（行は消さない。履歴のため） |

**読み方**（読む側は全部この順）:

1. 建屋の repo の **HEAD** の `projects/<slug>/delegation.json` を読む（worktree・作業側の写し・未 commit の変更は読まない）。
2. そのファイルを最後に変えた commit の author が `andon.toml` の `owner` と一致しなければ、**無委譲**（全部人）として読む。無い・JSON でない・schema を通らない・`project` が違う・`version` を知らない、も同じ（原則2）。
3. `preset` の値に、有効な `grants[]`（`revoked` が無く `until` が過ぎていない）を `at` の順に重ねる。
4. 読み手ごとに見る `scope`: ゲート＝`merge`。クーリエと `andon task …`＝`deploy`・`publish`・`irreversible`（打つ前。範囲外なら止まって理由1行）。Chief of Staff＝全部（起きるたび。`UserPromptSubmit` のフック経由。範囲外の判断は §5 の問い）。
5. Andon の建屋の `andon` プロジェクトは `preset` を `andon` 以外にできない（command が拒む。`merge` の上限 `B`・`line-change: none`）。`irreversible` に `delete-remote` は型でも grant でも入れられない。

**書き方**: 人が建屋の repo で `andon delegate grant <scope> <value> --evidence <path|text> [--until <date>]`・`narrow`・`revoke <id> --reason <text>`・`show [--json]` を打つ。command は JSON を検証して書き、`delegation.md` を生成し、commit する（message に `scope`・`value`・`evidence`）。Chief of Staff・担当・クーリエの端末では打てない（managed deny ＋ 端末のラベルに `role=` があれば拒む）。

**依頼書の Settings との関係**: `task.yaml` の `merge: auto` は委譲の `merge` の範囲の中でだけ効く（§2）。委譲の外なら、ゲートは approve でも `needs_human`（§7 の `delegation`）。
