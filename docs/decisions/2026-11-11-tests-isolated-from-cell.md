# 検査はセルの本物（台帳・HOME）から切り離して走る。切り離しは tests/lib/isolate.sh、確かめるのは tests/isolation

- 日付: 2026-11-11
- 出典: 人 2026-11-11（担当の検査がセルの台帳の chief_of_staff を書き換え、dispatcher がその台帳を約2時間飛ばした。家の CLAUDE.md も書き換わった）。課題 A-0149

- 原因: 台帳の CLI は、cwd の上に kanban/ が無いと、git の main checkout から上の `projects/*/kanban` で、その repo を repos に持つ台帳を探す。担当が TMPDIR を自分の worktree の中に置くと、検査の一時ディレクトリは Andon の repo の中になり、「台帳の外」を確かめる検査（`tests/andon-chief-of-staff.sh` の「台帳の外で打つと止まる」・`tests/kanban/test-deck.mjs` の古い形式の台帳）がセルの本物の台帳に書いた。`internal/issue`・`internal/boot` の Go の検査は HOME だけを一時に向け、lock（ready・start・dispatcher の端末）を本物の `~/.local/state/andon/dispatcher/`（`XDG_STATE_HOME` があればその下）に作っていた。
- `tests/*.sh` は `tests/lib/andon.sh`・`kanban.sh` から `tests/lib/isolate.sh` を読む。一時ディレクトリの根の親を `GIT_CEILING_DIRECTORIES` に足し（git は一時ディレクトリから上の repo を探さない）、受け継いだ `KANBAN_ROOT`・`CLAUDE_CONFIG_DIR`・`CODEX_HOME`・`ANDON_HOME`・`XDG_STATE_HOME`・`HC_*` を外す。
- Go の検査は、本物の HOME の下に書く置き場（`XDG_STATE_HOME` など）を `t.Setenv` で一時に向ける。
- 確かめ方は `tests/isolation`（手で打つ。全部を回すので CI には入れない）。偽のセル（番兵）を作り、KANBAN_ROOT・HOME・CLAUDE_CONFIG_DIR・XDG_STATE_HOME などを番兵へ、TMPDIR を repo の写しの中に向けて `go test ./...` と `tests/*.sh` を回し、番兵の台帳と HOME が1つでも変われば落ちる。検査の土台（`tests/lib/`・fixture）を変えたら打つ。
