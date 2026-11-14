# force-with-lease の allow を「origin と1語」に狭め、旗と + の1語は deny で断る

- 日付: 2026-11-13
- 出典: 課題 A-0200（A-0118 の見送りと、A-0200 のレビュー 1 回目の指摘 2 件）。[2026-11-10-force-with-lease-own-branch-only.md](2026-11-10-force-with-lease-own-branch-only.md) の allow の形を狭める

- `package/settings.json` の allow は `Bash(git push --force-with-lease)` と `Bash(git push --force-with-lease origin *)` の2本にする。前の `Bash(git push --force-with-lease *)` は、`-fu` のように短い旗を束ねた形や `+v1.0` のような `+` 付きの refspec を通し、lease を越えて強制で更新できた。
- `origin` のあとは1語だけを許す。2語以上（`origin * *`）、旗で始まる1語（`origin -*`）、`+` で始まる1語（`origin +*`）は deny で断る。旗を束ねる書き方は数え切れないので、旗を1つずつ deny に並べる形はとらない。
- `origin` より前に旗を置く形（`git push --force-with-lease -u origin HEAD`・`--force-if-includes`）と、`origin` 以外の remote は allow に当たらず、人に聞く。
- `--force-with-lease` と併せる `--force`・`-f`・`--delete`・`-d`・`--prune`・`--tags`・`--follow-tags`・`tag`・`:<ref>`・`+:<ref>`・`refs/tags/` の形も deny に置く。`:<ref>` で終わる形は、規則の末尾が `:*` だと頭の線（「 *」）と読まれるので、末尾を `:**` と書く。当たる・当たらないの例は `tests/package.sh`。
