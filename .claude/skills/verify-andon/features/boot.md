# boot

`andon boot` は、パッケージ（`/opt/andon`）からセルへ配布物（managed settings・プロファイル・CLAUDE.md の行・`~/.agents/skills` の symlink など）を生成し、portless・ローカルの tray・sshd・dispatcher を起動する。セルの起動のたびと、`image/apply-main.sh apply` で当てたあとに打ち直される。

## 細かい機能

- `bt-ran` 当てたあとに boot が走り、dispatcher を起動し直した。見る道: 検証の担当だけ。
  - 見るファイル: `internal/boot/boot.go`、`internal/boot/start.go`、`internal/boot/owners.go`、`image/apply-main.sh`
- `bt-tray` 当てたあと、ローカルの tray（`andon tray serve`）の控え `~/.local/state/andon/tray/serve.json` があり、その pid のプロセスが当てたパッケージの `andon tray serve` で、最後に当てた時刻より後に起動し、一覧の面が `GET /` に 200 を返す。パッケージに tray が無ければ（Node かビルドした tray の無いイメージ）note で未確認。見る道: 検証の担当だけ。
  - 見るファイル: `internal/boot/start.go`、`internal/tray/serve.go`、`internal/tray/cmd.go`、`image/apply-main.sh`
- `bt-generated` boot が書いたファイルが、控え（`~/.local/state/andon/boot.json`）の sha256 のまま在る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/boot/codex.go`、`internal/boot/trust.go`、`internal/boot/gitauthor.go`、`package/codex.rules`、`package/templates/`、`package/environment.md`、`package/writing.md`、`init/.dockerignore`、`init/.gitignore`、`init/AGENTS.local.md`、`init/Dockerfile.tmpl`、`init/andon.toml.tmpl`、`init/files/`、`init/gate.local.json`、`init/local/`、`init/projects/`、`init/wiki/`
- `bt-entry` ホストの `bin/bootstrap`（薄い入口）が、打つたびにセルの image から本体を `.andon/bootstrap` に取り出して走らせる。分ける前の入口のまま上げると、`docker compose` が止まって入口を置き換える 1 行を出す。見る道: オーナーが見る。
  - 見るファイル: `init/bin/bootstrap`、`package/host/bootstrap`、`package/compose.yaml`
- `bt-managed` managed settings の hook が `/opt/andon/hooks` を指す。見る道: 検証の担当だけ。
  - 見るファイル: `package/settings.json`
- `bt-skills` `/opt/andon/skills` の skill ごとに、`~/.agents/skills/andon-<名>` の symlink がその置き場を指す。リンクが無いもの・パッケージの外を指す `andon-*` があれば落ちる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/boot/extskills.go`、`package/skills/`、`package/gh-skills.txt`

## 辿り着き方（利用者から）

- セルが立つとき、`andon boot` が走る。
- Chief of Staff かオーナーが `image/apply-main.sh apply` を打つと、最後に `andon boot` が打ち直される。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた記録 `/opt/andon/applied.json` がある。無ければ終了コード 3。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh boot <検証する commit>` を打つ。生成物と控えを読むだけで、boot は打たない。
- **走ったことを読む。** `bt-ran` の行に dispatcher の端末の起動の時刻が出る。当てた時刻より後で、`ok   bt-ran` が見えれば動いた。
- **生成物を読む。** `bt-generated` の行に控えのファイルの数と、控えと違うものの数が出る。違うものが 0 で、`ok   bt-generated` が見えれば動いた。
- **ローカルの tray を読む。** `bt-tray` の行に控えの pid・実体・起動の時刻と、一覧の面の `GET /` の答えが出る。`ok   bt-tray` が見えれば、当てたパッケージで起動し直している。`note bt-tray` なら、パッケージに tray が無い。報告の `## Report` にそう書く。
- **配った先を読む。** `bt-managed` と `bt-skills` の行に、hook の置き場と skill の symlink の先が出る。`ok   bt-managed` が見え、`bt-skills` に FAIL が無ければ動いた。

- **入口を見る（`bt-entry`）。** ホストで動くので、検証の担当は動かせない。報告の `acceptance[]` に `manual` と書き、オーナーがセルのホストで見る次の証拠を `evidence` に書く。
  - 分ける前の入口のセル: `bin/bootstrap rebuild`（Andon を開発するセルでは `rebuild --from <checkout>`）が、image を焼いたあと `セルの入口 bin/bootstrap が古い` の行で止まり、行の最後の「: 」のあとに `docker run --rm andon-<slug>:local andon cell init --print bin/bootstrap …` の 1 行が出る。`docker ps` のセルのコンテナの ID は前後で同じ。
  - その 1 行を打ったあと: `grep -x 'entry=[0-9]*' bin/bootstrap` が番号を出し、`bin/bootstrap rebuild` が作り直す。`cmp .andon/bootstrap <(docker run --rm andon-<slug>:local cat /opt/andon/host/bootstrap)` が何も出さない。そのあとの `bin/bootstrap host-install` で mod `waiting` が置かれる。

## 変えたら動かすパス

- `internal/boot/`
- `internal/tray/serve.go`
- `internal/tray/cmd.go`
- `package/settings.json`
- `package/codex.rules`
- `package/templates/`
- `package/environment.md`
- `package/writing.md`
- `package/skills/`
- `package/gh-skills.txt`
- `image/apply-main.sh`
- `init/.dockerignore`
- `init/.gitignore`
- `init/AGENTS.local.md`
- `init/Dockerfile.tmpl`
- `init/andon.toml.tmpl`
- `init/files/`
- `init/gate.local.json`
- `init/local/`
- `init/projects/`
- `init/wiki/`
- `init/bin/bootstrap`
- `package/host/bootstrap`
- `package/compose.yaml`

## 既に見ている検査

- `tests/andon-boot.sh`（偽の HOME・偽のパッケージで2回打ち、2回目で何も変わらないこと）
- `tests/apply-main.sh`（当てて、焼いた andon の help・version を打つ）
- `go test ./internal/boot/`（ローカルの tray の段は、偽のパッケージと控えで、名前を付けること・付けそこねを出すこと・起動しない場合）
- `go test ./internal/tray/`（serve の控えの書き方と読み方・`andon tray status` の出し方）
- tray の CI の job の e2e（セルの側。偽のセルで boot を打ち、ローカルの tray が起動し、files サーバが無いこと）
- `tests/andon-init.sh`（`bt-entry`。偽の docker で、入口が本体を取り出す順・焼く回数・名指しの本体・入口の控え）
- `tests/image.sh`（`bt-entry`。docker のある CI で、薄い入口から起動し、分ける前の入口の `docker compose` が止まって置き換えの 1 行を出す）

## 落とし穴

- `bt-tray` は今動いている serve を読む。当てたあとにオーナーが手で止めて起動し直したなら、起動の時刻は当てた時刻より後でも、boot が起動したとは限らない。
- 検証の担当は serve を止めない。止めると、報告の送りが `files/` へ倒れる。

- `bt-entry` の本体は、`image/apply-main.sh` では変わらない。入口が取り出すのはセルの image の本体なので、当てたあとの `/opt/andon/host/bootstrap` と比べない。

- `image/apply-main.sh` は、PATH に `go` が無いときだけ、`mise which go` の前に `mise install` を打つ。セルの PATH に mise の shims の go がある間は、この分岐は当てても走らず、`bt-ran` の ok はこの分岐を通った証拠にならない。この分岐は `tests/apply-main.sh` の場面 14 が見る。

- boot は中身が変わらなければ控え（`boot.json`）を書き直さない。控えの時刻は、boot が走ったかの証拠にならない。
- 検証の担当は `andon boot` を打たない。dispatcher を起動し直し、動いている見回りを止める。
- オーナーが手で直したファイルは `bt-generated` で「控えと違う」と出る。壊れか手直しかを、そのファイルの中身と `git log` で書き分ける。
- `package/gh-skills.txt` の外の skill は、boot が `gh` のログインがあるときだけ入れる。`bt-skills` が見るのは `/opt/andon/skills` の symlink だけで、この一覧の skill は見ない。一覧を変えたときは、`~/.agents/skills` に入ったかを `ls` で別に読む。
