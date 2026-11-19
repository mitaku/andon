# 振り返りの入力は andon retro collect が台帳とセッションログと GitHub から1つの JSON に集め、前の振り返りは routine と retro.json で見分ける

- 日付: 2026-11-19
- 出典: [ADR-0028](../adr/0028-improvement-loop.md) 決めたこと 4・6・8・12・14 と「実装が決めてよいこと」

ADR-0028 が実装に任せた細部を次のとおり決めた。形と欄の意味は [contract.md](../contract.md) §20・`schema/retro.schema.json`。

- **入力は1つの JSON**: 標準出力に出すだけで保存しない（`andon measure` と同じ）。課題ごとの束・人が打った文・前の直しを同じ出力に入れる。このセルの 7 日ぶんで約 80 KB（課題 27 件・人の文 42 件）だったので、要約と本文には分けない。
- **束に入れる記録**: 起動の記録は `memory_index` を落として写す（課題ごとに数 KB あり、全文は `library` のパスから読める）。ほかの記録（結果・手直し・指摘）はそのまま写す。日報は行を読まずに丸ごと写す。
- **前の振り返りの見分け方**: `issue.yaml` の `routine` が `retro`（ルーチンが足した）か、`library/retro.json`（振り返りが書くパターンの一覧）がある課題。振り返りが切った直しは、`parent` がその課題の課題。パターンの一覧を書く command と `andon retro apply`（ADR-0028 の P1-2）は、この名前（ルーチン `retro`・`retro.json`・直す課題の `parent`）に合わせる。
- **人が打った文**: ADR-0028 決めたこと 14 の 1 の規則に、2つを足した。1つは包みの種類で、`<hc from=` のほかに通知の `<news`・ハーネスの `<task-notification>` も除き、貼った文の包みの内側も見る。もう1つは、Andon の command が端末の外から `hc send` した定型の文（dispatcher の再開の文と、`andon issue merge` のゲートの差し戻し）を除くこと。このセルで実測すると、この2つは hc の包みが付かずに人の印のまま担当のセッションに残り、ゲートの差し戻しが人の文に混ざった。送る側の定数（`dispatcher.RetryText`・`issue.FixMarker`）で照らすので、文の意味は読まない。
- **Codex**: rollout の user の文には人の印が無かった（このセルで実測）。Codex の端末の分は抜かない（ADR-0028 決めたこと 4 の「受け入れること」のとおり）。
- **Chief of Staff のセッション**: 起動の記録の `session` が `null` なので、ログの行の `cwd` がプロジェクトの根かで引く。担当とレビュアーも、起動の記録に無い古いセッションは `cwd`（台帳の `issues/<id>/` と `worktree`）で引く。
- **秘密の形**: 秘密鍵の塊・Anthropic と OpenAI の API キー・GitHub のトークン・AWS のアクセスキー・Slack のトークン・Google の API キー・JWT・`Bearer` のトークン・鍵の名前（`api_key`・`token`・`password` など）に続く値。書き出した JSON の文字列に当て、伏せたあと JSON として読めることを確かめる。多めに伏せる。
- **当て直しの印**: 当て直しの記録（`applied.json`）は最後の1回しか残らないので、起動の記録のパッケージの commit が課題の中で2通り以上あることでも印を付ける。起動の記録の無い課題は `null`。
- **前の直しのマージの日**: GitHub のマージされた PR の一覧の `mergedAt` を先に読み、読めなければ `outcome.json` の `merged_at`。マージのあとの課題の数は、期間に限らず数える。
- **形式モデル**: 読むだけの command なので、課題の一生の動きは変えない。`model/` は変えない。
