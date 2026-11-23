# command が足す担当の課題とルーチンが足した課題は、予約キーを既定で持って起動できる

- 日付: 2026-11-23
- 出典: Chief of Staff の依頼（2026-11-23）。トリアージの apply が足した直す課題・ルーチンが足したトリアージ・その直す課題が、予約キーの無いまま `andon issue start` に断られ、同じ日に3回 Chief of Staff が手で書いた
- 覆す: [decisions/2026-11-22-issue-start-refuses-unset-settings.md](2026-11-22-issue-start-refuses-unset-settings.md) の「コマンドは値を決めず、何も書かない」（ルーチンが足した課題に限る）

- 値は決まった既定（`internal/ledger` の `DefaultSettings`。Chief of Staff の `playbooks/brief.md` の表の既定）: `must_fix=major`・`reviewer` は実装と別ブランド（`codex*` のプロファイルなら `claude-review`、ほかは `codex-review`）・`review_rounds=2`・`merge=person`・`estimate=medium`。`review` はトリアージの決定の `review`（束ならどれか1つが `on` で `on`）、ほかは `off`。ルーチンの定義に欄は足さない。
- `andon triage apply` は、`fix`・`bundle` の課題を `kanban add` した直後に、その課題へ予約キーを全部 `kanban set` する。依頼書の `## Settings` に同じ値の写しを書く。`andon retro apply` は、担当の課題（`who: person` でない）に同じことをする（`review` は `off`）。打ち直しで、前の回が足した課題に `review` が無ければ書き、あれば触らない。`profile` は今までどおり書かない。どちらも Chief of Staff の端末で打つので、`andon issue start` と gate の履歴の照合が通る。
- ルーチンが足した課題（`log.jsonl` に `by: routine:<名前>`・`fired: <id>` の行があり、課題の `routine` がその名前）は、dispatcher の `kanban tick` が足すので、足すときに Chief of Staff の書いた値を持てない。そこで `andon issue start` が、Chief of Staff の端末（`HC_TERMINAL_ID` が `config.yaml` の `chief_of_staff`）で打たれたときだけ、要るキーのうち無いものを既定（`review=off`）で埋めて起動する。書くのは起動の前で、`--dry-run` は書かずに確かめの行で埋めるキーを出す。書いてあるキーは変えない（照合が外れていれば今までどおり断る）。`routine` の欄だけがあって行が無い課題と、ほかの端末で打ったときは埋めない。
- 変えないもの: Chief of Staff が切る課題は今までどおり断り、値を選ぶ場所を残す。gate・dispatcher の履歴の照合（`terminal:<chief_of_staff>` だけを信じる）は広げない。
