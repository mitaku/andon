# AGENTS.md（この repo で働く担当の指針）

- テストは `go test ./...` と `tests/run --changed`（変えたものに関係する `tests/*.sh` だけを並べて走らせる）。commit の前に通す。手元では関係する検査だけ、全部（`tests/run`）は CI に任せる。Go の版は `mise.toml`（`go.mod` と揃える）。`gofmt -l .` が空であること。
- `tests/run` を同時に2本走らせない（mise の置き場と CPU を取り合う）。`tests/isolation` は15分以上かかるので、hc の端末（`hc run --bg`）で裏に回す。`package/` を変えると `tests/run --changed` がほぼ全部を走らせて遅い。skill だけの変更なら `tests/package.sh` と CI で足りる。
- 検査はセルの本物（台帳・HOME）に書かない。`tests/*.sh` は `tests/lib/andon.sh`・`kanban.sh` を読んで切り離す（`tests/lib/isolate.sh`）。Go の検査は HOME の下の置き場を `t.Setenv` で一時に向ける。検査の土台（`tests/lib/`・fixture・HOME や台帳を読む検査）を変えたら `tests/isolation` を打つ（偽のセルで全部を回し、番兵が変われば落ちる）。
- この repo は public。秘密・個人の環境に固有の値（セル名・ホスト名・ポート番号・メールアドレス・ユーザー名・ホームのパス）・案件に固有の名前を入れない。例は架空の名前（`example.com`・`<owner>`・`sample-site`）で。`tests/public.sh` が検査する。禁止リストは repo の外に置く（書き方は `tests/README.md`。リストの語を repo に書かない）。`.github/workflows/**` と `tests/public*.sh` を弱める変更は人が見る。
- 設計の正は `docs/design.md` と `docs/adr/`、決定の記録は `docs/decisions/` に1件1ファイルで足す（書き方は `docs/decisions.md` の冒頭。`docs/decisions.md` の表には行を足さない）。迷ったら推奨案で進めて報告に書く。
- boot の実測は `tests/andon-boot.sh` の `env -i` と一時領域の指定を使う。`HOME` だけを変える手順は使わない。
- `package/skills` の文を `playbooks/` へ移したら、`tests/words.sh` の禁止語に当たらないか回す。
- 否定の grep（`! grep -q <古い名前>`）には、当たる場合があることを同じ所で確かめる行を添える（添えないと、検査が何も見ていなくても通る）。
- 1ファイル1読み手。人用の文書（`README.md`・`docs/`）に機械が読む行を埋めない。AI 用の文書（この `AGENTS.md`・`REVIEW.md`・`package/skills/`）に人向けの経緯を書かない。
- `andon issue report` は `report.json` の `review_request` を消す。レビュー依頼中に報告を書き直したら、同じ commit で `andon issue review-request` を打ち直す。
- `go:embed` で repo のファイルを足したら、`image/Dockerfile` の `COPY` と `.dockerignore` も直す。
- 約束の正は `schema/`。schema を変えたら `docs/contract.md` と tests も直す。
- `package/` の変更はセルで実測する。
- main へ直接 push しない。マージは人がする。
