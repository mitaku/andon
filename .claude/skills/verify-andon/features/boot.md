# boot

`andon boot` は、パッケージ（`/opt/andon`）からセルへ配布物（managed settings・プロファイル・CLAUDE.md の行・`~/.agents/skills` の symlink など）を生成し、portless・files・sshd・dispatcher を起動する。セルの起動のたびと、`image/apply-main.sh apply` で当てたあとに打ち直される。

## 細かい機能

- `bt-ran` 当てたあとに boot が走り、dispatcher を起動し直した。見る道: 検証の担当だけ。
- `bt-generated` boot が書いたファイルが、控え（`~/.local/state/andon/boot.json`）の sha256 のまま在る。見る道: 検証の担当だけ。
- `bt-managed` managed settings の hook が `/opt/andon/hooks` を指す。見る道: 検証の担当だけ。
- `bt-skills` `/opt/andon/skills` の skill ごとに、`~/.agents/skills/andon-<名>` の symlink がその置き場を指す。リンクが無いもの・パッケージの外を指す `andon-*` があれば落ちる。見る道: 検証の担当だけ。

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
- **配った先を読む。** `bt-managed` と `bt-skills` の行に、hook の置き場と skill の symlink の先が出る。`ok   bt-managed` が見え、`bt-skills` に FAIL が無ければ動いた。

## 変えたら動かすパス

- `internal/boot/`
- `package/settings.json`
- `package/codex.rules`
- `package/templates/`
- `package/environment.md`
- `package/writing.md`
- `package/skills/`
- `package/gh-skills.txt`
- `image/apply-main.sh`
- `init/`

## 既に見ている検査

- `tests/andon-boot.sh`（偽の HOME・偽のパッケージで2回打ち、2回目で何も変わらないこと）
- `tests/apply-main.sh`（当てて、焼いた andon の help・version を打つ）
- `go test ./internal/boot/`

## 落とし穴

- boot は中身が変わらなければ控え（`boot.json`）を書き直さない。控えの時刻は、boot が走ったかの証拠にならない。
- 検証の担当は `andon boot` を打たない。dispatcher を起動し直し、動いている見回りを止める。
- オーナーが手で直したファイルは `bt-generated` で「控えと違う」と出る。壊れか手直しかを、そのファイルの中身と `git log` で書き分ける。
- `package/gh-skills.txt` の外の skill は、boot が `gh` のログインがあるときだけ入れる。`bt-skills` が見るのは `/opt/andon/skills` の symlink だけで、この一覧の skill は見ない。一覧を変えたときは、`~/.agents/skills` に入ったかを `ls` で別に読む。
