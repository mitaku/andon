# ハーネスの受け口

Claude Code と Codex で起動した担当・レビュアー・Chief of Staff・Chief Engineer は、トレースとログをセルの中の受け口（`andon otel serve`。`127.0.0.1:4318`）へ送る。起動の行（`package/templates/`）が `andon otel env` の出す設定を渡し（Codex は `--codex-env` と `--codex`）、`andon otel env` は受け口が止まっていれば起動する。受け口は、個人の属性を落として秘密の形を伏せてから、課題ごとのファイル（`~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl`）に書く。プロジェクトの `project.json` の `otel.enabled` が `false` なら送らない。

## 細かい機能

- `ot-listen` 受け口が `127.0.0.1:4318` で listen し、`GET /` が名前 `andon-otel` を返す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/otel/serve.go`、`internal/otel/env.go`
- `ot-applied` 当てたあと、listen している受け口のプロセス（`/opt/andon/bin/andon otel serve`）の実体が当てたパッケージで（`/proc/<pid>/exe` が `(deleted)` でない）、起動の時刻が最後に当てた時刻より後。`image/apply-main.sh` の apply と revert が受け口を止め、打ち直した boot が当てたパッケージで起動し直す。見る道: 検証の担当だけ。
  - 見るファイル: `image/apply-main.sh`
- `ot-records` 当てたあとに起動した Claude Code の担当の課題の `traces.jsonl` に `claude_code.llm_request` の span があり、resource 属性に `andon.issue` と `andon.role` があり、`user.email` を持つ行が無い。見る道: 検証の担当だけ。
  - 見るファイル: `internal/otel/otel.go`、`internal/otel/env.go`、`internal/otel/prune.go`、`package/templates/`
- `ot-codex` 当てたあとに Codex のプロファイル（`codex`・`codex-review`）で起動した課題の置き場に、Codex の記録（`traces.jsonl` か `logs.jsonl`）が 1 行以上あり、resource 属性に `andon.issue` と `andon.role` があり、`user.email` と `codex.tool_result` の `output` を持つ行が無い。見る道: 検証の担当だけ。
  - 見るファイル: `internal/otel/env.go`、`internal/otel/otel.go`、`package/templates/codex.md`、`package/templates/codex-review.md`
- `ot-restart` 受け口を止めてから担当を起動すると、起動が通り、起動のあと受け口が listen している。その担当の端末を片付けたあとも listen している。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/otel/env.go`、`internal/otel/serve.go`、`package/templates/`
- `ot-trace` `ot-records` が記録を数えた課題の 1 つで、当てたパッケージの `andon issue trace <id> --otlp` が終了コード 0 で、出力の span がどれも同じ traceId を持ち、親の無い span が 1 つだけで、ハーネスの `claude_code.llm_request` の span が入り、`claude_code.interaction` の 1 つ以上が同じ session の起動の span の下に入っている。数えた課題のレビュアーの課題に dispatcher の控えの `gone` の項目があれば、その課題を選び、項目の数だけ「端末が死んでいた」の段がレビューの段の下に出る（そういう課題が無ければ note を出し、その部分は未確認）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/trace/trace.go`、`internal/trace/harness.go`、`internal/trace/output.go`
- `ot-breakdown` 当てたあとに当てた dispatcher が書いた、Claude Code の担当の課題の結果の記録（`library/outcome.json`）に `breakdown` があり、そのトークンの合計（4 種の和）が同じ記録の `sessions[].cost` のトークンの合計の上下 5% に入る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/trace/breakdown.go`、`internal/dispatcher/outcome.go`、`internal/ledger/worked.go`、`schema/outcome.schema.json`

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue start <id>` で担当を起動する（dispatcher が起動するレビュアーも同じ）。起動の行が `andon otel env` を打つ。
- セルが立つとき、`andon boot` が受け口を起動する。
- オーナーが `project.json` に `"otel": {"enabled": false}` を書いて commit すると、次の起動から送らない。
- オーナーか役が、プロジェクトの置き場で `andon issue trace <id>` を打ち、課題 1 件の流れを表か `--json` か `--otlp` で読む（読むだけ）。
- 担当の課題が片付く（`andon issue finish`・Chief of Staff の `kanban done`）と、次の見回りで dispatcher が結果の記録に内訳 `breakdown` を書く。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff か dispatcher が Claude Code のプロファイルで1つ以上の端末を起動し、その担当が 1 ターン以上動いている。無ければ `ot-records` は終了コード 3（未確認）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh otel <検証する commit>` を打つ。受け口に `GET /` を打ち、置き場のファイルを読むだけで、何も起動しない・止めない。
- **受け口を読む（`ot-listen`）。** `ok   ot-listen` が見えれば、受け口が listen している。
- **受け口が当てたパッケージで動いているかを読む（`ot-applied`）。** `ok   ot-applied` が見えれば、受け口の実体は当てたパッケージで、当てたあとに起動している。`fail` なら当てる道具が受け口を起動し直していない（前のパッケージの受け口が、前のコードで記録を書いている）。
- **記録を読む（`ot-records`。ADR-0060 受け入れ条件 14）。** 記録は、Chief of Staff が起動した担当が 1 ターン以上動いたあとに読む。当てたあとの起動ごとに、課題の `traces.jsonl` の数が出る。`ok   ot-records` が見えれば、`claude_code.llm_request` と `andon.issue`・`andon.role` が載り、`user.email` が落ちている。証拠には、同じ置き場に打った `jq -c 'select(.resourceSpans[0].scopeSpans[].spans[].name == "claude_code.llm_request") | .resourceSpans[0].resource.attributes | map(select(.key | startswith("andon."))) ' ~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl | sort -u` の出力と、`grep -c '"user.email"' <同じファイル>` が 0 であることを添える。
- **課題 1 件の流れを出す（`ot-trace`。ADR-0060 受け入れ条件 8〜10 のセルの本物での確かめ）。** `ot-records` が ok になった回だけ動く。`ok   ot-trace` が見えれば、本物の kanban と置き場から 1 本のトレースが出て、ハーネスの記録が起動の下に結ばれている。`note ot-trace` が出たら、レビュアーの端末が死んでいた間は確かめていないと報告に書く。表の形は `cd <プロジェクトの置き場> && andon issue trace <id>` で読み、証拠に添える。
- **結果の記録の内訳を読む（`ot-breakdown`。ADR-0060 受け入れ条件 16）。** `ot-records` が ok になった回だけ動く。当てた時刻より後に dispatcher が書いた（`at` が当てた時刻より後の）Claude Code の担当の課題の `outcome.json` ごとに、`breakdown` のトークンの合計と `sessions[].cost` のトークンの合計と、その差の割合が出る。`ok   ot-breakdown` が見えれば、内訳が書かれ、2 つの合計が上下 5% に入る。片付いた課題がまだ無ければ `note` が出て未確認になる（道の終了コードは変わらない）。Chief of Staff に、試しの課題を 1 件片付けてもらってから打ち直す。`FAIL` なら、端末を片付けるときに最後の分が受け口へ送られ切っていないか（その課題の `traces.jsonl` の最後の行の時刻と、`log.jsonl` の done の行の時刻を比べる）を調べ、結果を報告に書く。証拠には、`jq -c '{issue, b: ([.breakdown.tokens[]] | add), s: ([.sessions[].cost.tokens // {} | .[]] | add)}' <その outcome.json>` の出力を添える。
- **Codex の記録を読む（`ot-codex`。ADR-0060 受け入れ条件 17）。** 記録は、当てたあとに Codex のプロファイルで起動したレビュアー（か担当）が 1 ターン以上動いたあとに読む。Codex の起動ごとに、課題の `traces.jsonl` と `logs.jsonl` の行の数が出る。`ok   ot-codex` が見えれば、Codex の記録が課題に結ばれて書かれ、個人の属性とツールの出力の写しが落ちている。記録が無いときは `note` が出て、道の終了コードは変わらない（受け入れ条件 17 は、無いこと自体を報告に書く条件）。そのときは、`hc read <その起動の端末>` で Codex が立ち上がったかと、受け口のログ（`/tmp/andon-otel.log` の「書かずに捨てた」の行）を読み、どちらの欠けかを報告に書く。証拠には、`jq -c '(.resourceSpans // .resourceLogs)[0].resource.attributes | map(select(.key | startswith("andon.")))' ~/.local/state/andon/otel/<slug>/issues/<id>/*.jsonl | sort -u` の出力と、`grep -c '"user.email"' <同じファイル>` が 0 であることを添える。
- **受け口が止まっていても起動が通ることを読む（`ot-restart`。ADR-0060 受け入れ条件 15）。** 検証の担当は自分で担当を起動しない。報告の `## Next` に「受け口を止めてから試しの課題を 1 件起動する」と書き、Chief of Staff に頼む。止め方は、`ss -ltnp 'sport = :4318'` で受け口の PID を引いて `kill <PID>` する（`pkill -f` は使わない）。止めたあと `curl -s -m 2 http://127.0.0.1:4318/` が返らないことを確かめる。Chief of Staff が起動した担当の端末を `hc read <address>` で読み、Claude Code が立ち上がったことを確かめる。起動のあと `curl -s -m 2 http://127.0.0.1:4318/` が `andon-otel` を返すこと、その担当の端末が片付いたあと（`hc get <address>` が見つからない）にも同じく返すことを確かめ、打った command と出力を報告に書く。

## 変えたら動かすパス

- `internal/otel/`
- `internal/trace/`
- `package/templates/`
- `image/apply-main.sh`
- `internal/dispatcher/outcome.go`
- `internal/ledger/worked.go`
- `schema/outcome.schema.json`

## 既に見ている検査

- `go test ./internal/otel/`（受け口の受けて書く処理・片付け・`andon otel env` の出す並びと送らない場合・template の行が `andon otel env` を渡すこと・Codex の見本〈ソースから組んだ形〉の振り分けと落とす属性）
- `go test ./internal/dispatcher/`（片付けが `project.json` の寿命と上限を読む）
- `go test ./internal/delegate/`（`project.json` の `otel` の形）
- `go test ./internal/trace/`（`andon issue trace` の段の導き方・起動への結び方・`--otlp` の 1 本のトレース・記録が無いときの 1 行。内訳の数え方。偽の kanban と偽の置き場で）
- `go test ./internal/dispatcher/` の `TestOutcomeBreakdown`（受け口に見本を POST した置き場から、結果の記録に内訳が入り、レビュアーの課題の分が親に入らず、記録の無い課題は `null`）

## 落とし穴

- 置き場の記録は、担当が 1 ターン以上動いて Claude Code が送ったあとにできる。起動した直後に読むと記録が無く、`ot-records` は未確認になる。壊れに数えない。
- `ot-records` は Codex の起動を数えない（Codex は `ot-codex` が読む）。Codex の送る中身は、`codex-review` で起動したレビュアーをセルで動かして確かめた（記録が課題に結ばれ、`user.email` と `codex.tool_result` の `output` を持つ行が無い）。Codex のバージョンが上がると送る中身が変わりうるので、バージョンを上げたあとは `ot-codex` を読み直す。Codex の記録が無いことは、`ot-codex` の `note` として報告に書き、壊れには数えない。
- `project.json` の `otel.enabled` が `false` のプロジェクトでは、記録が無いのが正しい。読む前に `project.json` を見る。
- 受け口を止めると、止めている間の記録は欠ける。`ot-restart` を確かめたら、受け口が listen していることを最後にもう一度確かめる。
- `image/apply-main.sh apply` は受け口を止めて当てたパッケージで起動し直す（`ot-applied`）。それより前のバージョンの道具で当てたときや、起動し直しに失敗したときは、受け口が当てる前から動き続ける。検証する commit を当てる前から動く受け口は、その commit の `internal/otel/` の変更（落とす属性など）を持たない。`ot-listen` の下に受け口の起動の時刻と実体が出て、当てる前から動いていれば `note ot-listen` が出る。そのときの `ot-codex`・`ot-records` の落とした・残ったの判定は、`ok` も `FAIL` も前のコードの受け口の結果で、検証する commit の受けて書く処理は未確認になる。受け口の実体の commit（当てた記録の `previous` で、受け口の起動より前に当てた一番新しい commit）と検証する commit の間で `git log <受け口の commit>..<検証する commit> -- internal/otel/` が空なら、受けて書く処理は同じなので、判定はそのまま検証する commit の結果として書く。空でなければ未確認と書き、`FAIL` は前のコードの結果と書き分けて、受け口を起動し直す課題を `## Next` で Chief of Staff に頼む。受け口が当てた時刻より後に起動していても、それより前の記録は別の受け口が書いたので、同じく書き分ける（`note ot-listen` が出る）。
- Codex のログは `timeUnixNano` を `"0"` で送り、時刻は `observedTimeUnixNano` にだけある（v0.166.0）。`ot-codex` は両方を読んで起動より後の行を数える。
- オーナーが自分で打った `claude` は送らない。自分の手で起動した Claude Code の記録が無いことは壊れではない。
