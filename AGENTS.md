# AGENTS.md（この repo で働く担当の指針）

- テストは `for f in tests/*.sh; do bash "$f"; done`。commit の前に通す。
- この repo は最終的に public にする。秘密・個人の環境に固有の値（建屋名・ホスト名・ポート番号・メールアドレス・ユーザー名・ホームのパス）・案件に固有の名前を入れない。例は架空の名前（`example.com`・`<owner>`・`sample-site`）で。`tests/public.sh` が検査する。禁止リストは repo の外に置く（書き方は `tests/README.md`。リストの語を repo に書かない）。`.github/workflows/**` と `tests/public*.sh` を弱める変更は人が見る。
- 設計の正は `docs/design.md`、決定の記録は `docs/decisions.md`。設計書に無いものを足さない。迷ったら推奨案で進めて報告に書く。
- 1ファイル1読み手。人用の文書（`README.md`・`docs/`）に機械が読む行を埋めない。AI 用の文書（この `AGENTS.md`・`REVIEW.md`・`package/skills/`）に人向けの経緯を書かない。
- 約束の正は `schema/`（P2 以降）。schema を変えたら `docs/contract.md` と tests も直す。
- `package/` の変更は建屋で実測する（P3 以降）。
- main へ直接 push しない。マージは人がする。
