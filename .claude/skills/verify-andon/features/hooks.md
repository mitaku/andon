# hook

Claude Code が managed settings から呼ぶ4本の hook（`/opt/andon/hooks/stop`・`stop-wait`・`stop-failure`・`user-prompt-submit`）。担当の Stop では、`doing` で報告も問いも無いまま止まろうとすると `{"decision":"block"}` で止める。Chief of Staff では待ちの通知を届け、どちらでもない端末では何も出さずに通す。

## 細かい機能

- `hk-wired` managed settings の hook が `/opt/andon/hooks` の4本だけを指し、4本とも実行できる。見る道: 検証の担当だけ。
- `hk-stop-block` 報告も問いも無い `doing` の担当の Stop が block と理由を返す。見る道: 検証の担当だけ。
- `hk-stop-pass` 報告を書いた担当の Stop は何も出さずに通す。見る道: 検証の担当だけ。
- `hk-other-pass` ラベルの無い端末では4本とも何も出さずに 0 で通す。見る道: 検証の担当だけ。
- `hk-cos-news` turn 中の通知が、Chief of Staff の Stop で続きの turn として届く。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
- `hk-wait` 離席中に `stop-wait` が合図で Chief of Staff の新しいターンを始め、入力欄の書きかけが残る。見る道: 人が見る。
- `hk-esc` 渡した turn を Esc で止めたら、次の turn で1回だけ届け直す。見る道: 人が見る。

## 辿り着き方（利用者から）

- 担当・レビュアー・Chief of Staff の Claude Code が turn を終えるたびに、Stop の2本が走る。
- 人が Chief of Staff に何かを送るたびに、`user-prompt-submit` が走る。
- プロバイダに切られて止まると、`stop-failure` が走る。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- managed settings の置き場は既定の `/etc/claude-code/managed-settings.json`（`ANDON_MANAGED_SETTINGS` で変えられる）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh hooks` を打つ。
- **つながりを読む。** `hk-wired` の行に managed settings の4行（`Stop /opt/andon/hooks/stop` など）が出る。`ok   hk-wired` が見えれば動いた。
- **止める。** 切り離した kanban の `doing` の課題を端末 `tw` のラベル `issue=` に結び、`/opt/andon/hooks/stop` に `{"stop_hook_active":false}` を渡す。出力が `{"decision":"block","reason":"…"}` で、`ok   hk-stop-block` が見えれば動いた。
- **通す。** `kanban report` のあとに同じ hook を打つ。出力が空で、`ok   hk-stop-pass` が見えれば動いた。
- **素通りする。** ラベルの無い端末 `tx` で4本を打つ。4本とも出力が空で終了コード 0、`ok   hk-other-pass` が見えれば動いた。
- **Chief of Staff に届く（`hk-cos-news`）。** 検証の担当は報告の `## Next` に「試しの課題を1件起動し、Chief of Staff の長めの turn の間に報告させる」と書く。起動したあと `hc read <Chief of Staff の address>` で、turn の終わりに次の turn が自動で始まり `<news>` を読んだことを確かめる。
- **待ち役と Esc（`hk-wait`・`hk-esc`）。** 人が見る。報告の `acceptance[]` に `manual` と書き、見る場所は `docs/operate.md` の「12. 実測する」の表。

## 変えたら動かすパス

- `package/hooks/`
- `internal/issue/stopcheck.go`
- `internal/dispatcher/drain.go`
- `internal/dispatcher/wait.go`
- `internal/dispatcher/rewake.go`
- `internal/boot/`
- `package/settings.json`

## 既に見ている検査

- `tests/andon-dispatcher.sh`（drain・wait・rewake を偽の端末で見る）
- `tests/package.sh`（hook のファイルと managed settings の timeout の突き合わせ）
- `go test ./internal/issue/ ./internal/dispatcher/`

## 落とし穴

- hook は落ちても何も出さずに通す作り。出力が空なのは「通した」とも「落ちた」とも読める。`hk-stop-block` で block が出ることを先に確かめ、空の出力だけを証拠にしない。
- `stop-wait` は Chief of Staff の端末では通知が来るまで最長 2 時間待つ。`drive.sh` は Chief of Staff のラベルを持つ端末を作らず、`timeout 30` を付けて打つ。
- 本物の Chief of Staff の端末で hook を手で打たない。待ちの通知を本物の Chief of Staff から抜き取ってしまう。
