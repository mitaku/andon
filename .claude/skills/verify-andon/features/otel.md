# ハーネスの受け口

Claude Code で起動した担当・レビュアー・Chief of Staff・Chief Engineer は、トレースとログをセルの中の受け口（`andon otel serve`。`127.0.0.1:4318`）へ送る。起動の行（`package/templates/`）が `andon otel env` の出す設定を渡し、`andon otel env` は受け口が止まっていれば起動する。受け口は、個人の属性を落として秘密の形を伏せてから、課題ごとのファイル（`~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl`）に書く。プロジェクトの `project.json` の `otel.enabled` が `false` なら送らない。

## 細かい機能

- `ot-listen` 受け口が `127.0.0.1:4318` で listen し、`GET /` が名前 `andon-otel` を返す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/otel/serve.go`、`internal/otel/env.go`
- `ot-records` 当てたあとに起動した Claude Code の担当の課題の `traces.jsonl` に `claude_code.llm_request` の span があり、resource 属性に `andon.issue` と `andon.role` があり、`user.email` を持つ行が無い。見る道: 検証の担当だけ。
  - 見るファイル: `internal/otel/otel.go`、`internal/otel/env.go`、`internal/otel/prune.go`、`package/templates/`
- `ot-restart` 受け口を止めてから担当を起動すると、起動が通り、起動のあと受け口が listen している。その担当の端末を片付けたあとも listen している。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/otel/env.go`、`internal/otel/serve.go`、`package/templates/`

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue start <id>` で担当を起動する（dispatcher が起動するレビュアーも同じ）。起動の行が `andon otel env` を打つ。
- セルが立つとき、`andon boot` が受け口を起動する。
- オーナーが `project.json` に `"otel": {"enabled": false}` を書いて commit すると、次の起動から送らない。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff か dispatcher が Claude Code のプロファイルで1つ以上の端末を起動し、その担当が 1 ターン以上動いている。無ければ `ot-records` は終了コード 3（未確認）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh otel <検証する commit>` を打つ。受け口に `GET /` を打ち、置き場のファイルを読むだけで、何も起動しない・止めない。
- **受け口を読む（`ot-listen`）。** `ok   ot-listen` が見えれば、受け口が listen している。
- **記録を読む（`ot-records`。ADR-0060 受け入れ条件 14）。** 記録は、Chief of Staff が起動した担当が 1 ターン以上動いたあとに読む。当てたあとの起動ごとに、課題の `traces.jsonl` の数が出る。`ok   ot-records` が見えれば、`claude_code.llm_request` と `andon.issue`・`andon.role` が載り、`user.email` が落ちている。証拠には、同じ置き場に打った `jq -c 'select(.resourceSpans[0].scopeSpans[].spans[].name == "claude_code.llm_request") | .resourceSpans[0].resource.attributes | map(select(.key | startswith("andon."))) ' ~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl | sort -u` の出力と、`grep -c '"user.email"' <同じファイル>` が 0 であることを添える。
- **受け口が止まっていても起動が通ることを読む（`ot-restart`。ADR-0060 受け入れ条件 15）。** 検証の担当は自分で担当を起動しない。報告の `## Next` に「受け口を止めてから試しの課題を 1 件起動する」と書き、Chief of Staff に頼む。止め方は、`ss -ltnp 'sport = :4318'` で受け口の PID を引いて `kill <PID>` する（`pkill -f` は使わない）。止めたあと `curl -s -m 2 http://127.0.0.1:4318/` が返らないことを確かめる。Chief of Staff が起動した担当の端末を `hc read <address>` で読み、Claude Code が立ち上がったことを確かめる。起動のあと `curl -s -m 2 http://127.0.0.1:4318/` が `andon-otel` を返すこと、その担当の端末が片付いたあと（`hc get <address>` が見つからない）にも同じく返すことを確かめ、打った command と出力を報告に書く。

## 変えたら動かすパス

- `internal/otel/`
- `package/templates/`

## 既に見ている検査

- `go test ./internal/otel/`（受け口の受けて書く処理・片付け・`andon otel env` の出す並びと送らない場合・template の行が `andon otel env` を渡すこと）
- `go test ./internal/dispatcher/`（片付けが `project.json` の寿命と上限を読む）
- `go test ./internal/delegate/`（`project.json` の `otel` の形）

## 落とし穴

- 置き場の記録は、担当が 1 ターン以上動いて Claude Code が送ったあとにできる。起動した直後に読むと記録が無く、`ot-records` は未確認になる。壊れに数えない。
- Codex のプロファイルの起動は送らない（Codex の取り込みは別の課題）。`ot-records` は Codex の起動を数えない。
- `project.json` の `otel.enabled` が `false` のプロジェクトでは、記録が無いのが正しい。読む前に `project.json` を見る。
- 受け口を止めると、止めている間の記録は欠ける。`ot-restart` を確かめたら、受け口が listen していることを最後にもう一度確かめる。
- オーナーが自分で打った `claude` は送らない。自分の手で起動した Claude Code の記録が無いことは壊れではない。
