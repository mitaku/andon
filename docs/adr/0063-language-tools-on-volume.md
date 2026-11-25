# ADR-0063: 言語の道具を image に焼かず、案件の repo の mise.toml を正にして volume に置く（仕様・設計。2026-11-25）

- 状態: **決定（2026-11-25）**。オーナーが 2026-11-25 に「決めた」と答えた。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書く。
- 主題: セルを作り直すたびに、mise が入れた言語の道具（go・golangci-lint・node）が消え、誰かが手で入れ直すまで担当の検査が落ちることを止める。バージョンは案件の repo の `mise.toml` だけが決め、image はバージョンを持たない。入れた道具は、消えてもよいキャッシュとして volume に残す。宣言を満たす手は、Andon が役の端末を起動する直前に打つ。
- 語は [design.md](../design.md) §0.1。image と compose の今の形は [image.md](../image.md)、セルの作り直しの手順は [operate.md](../operate.md) にある。
- この文書で「宣言を満たす」は、repo の `mise.toml` が名指す道具とバージョンを、その repo の中で `mise install` を打って入れることを指す。「役の端末」は、Andon がプロファイルから起動する Chief of Staff・Chief Engineer・担当・レビュアーの端末を指す。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、道具の volume、端末の起動で宣言を満たすこと、image の既定の go をやめること、`mise.toml` の無い所の既定までにする | この ADR | まだ実装していない |
| 2 | 言語の道具のバージョンの正は案件の repo の `mise.toml` で、image はバージョンを持たない | この ADR | まだ実装していない |
| 3 | base image が持つのは、mise 本体と、PATH に入れた shims の置き場までにする | この ADR | まだ実装していない |
| 4 | 入れた道具は compose の volume に置き、消えてもよいキャッシュとして扱う | この ADR | まだ実装していない |
| 5 | volume の名前は `mise` で、`~/.local/share/mise` を丸ごと載せる。mise の cache は載せない | この ADR | まだ実装していない |
| 6 | 宣言を満たす手は、Andon が担当・レビュアー・Chief Engineer の端末を起動する直前に、その worktree で打つ | `internal/tools/tools.go` と起動の口（`internal/issue/start.go`・`internal/issue/resume.go`・`internal/chiefengineer/chiefengineer.go`・`internal/dispatcher/loop.go`） | 生きている |
| 7 | 宣言を満たせなくても起動は止めず、警告を出して続ける。待つ上限は 60 秒で、過ぎても入れる手は止めない | `internal/tools/tools.go` | 生きている |
| 8 | gate の検査と `image/apply-main.sh` も、道具を使う前に宣言を満たす。gate は、満たせなければ判定しない | `internal/gate/gate.go`・`image/apply-main.sh` | 生きている |
| 9 | image の mise の既定の go の設定をやめる | この ADR | まだ実装していない |
| 10 | `mise.toml` の無い所では、役の端末と gate の検査だけが、自分の worktree の `mise.toml` をいちばん弱い既定として引く。端末へは `andon tools env` が渡す | `internal/tools/env.go`・`package/templates/`・`internal/gate/gate.go` | 生きている |
| 11 | 古いバージョンの掃除は自動でしない。手で打つ command を運用の手引きに書く | この ADR | まだ実装していない |
| 12 | 実装は 2 本に分け、端末の起動で宣言を満たす変更を先に入れる | この ADR | 生きている（端末の起動で宣言を満たす 1 本目を入れた。image の既定の go をやめる 2 本目はまだ） |

## 問題（オーナーの言葉で。言い直しの段落）

オーナーの依頼（2026-11-25。ホストの担当との話を Chief of Staff が中継した）は、このテーマをほかの課題より先に進めることと、セルの側のつなぎ（`compose.override.yaml` など）を作らずに Andon 本体で直すことである。

言い直し（オーナーが 2026-11-25 に同意した）。困っているのは、セルを作り直すたびに言語の道具が消え、誰かが気づいて手で入れ直すまで、担当の検査が `go: command not found` で落ちることである。原因は 2 つある。入れた道具がコンテナの中にしか無いことと、宣言を満たす役を Andon の誰も持っていないことである。直したい形は、バージョンを案件の repo の `mise.toml` だけが決め、image はバージョンを一切持たず、入れた道具は消えてもよいキャッシュとして volume に残り、Andon が宣言を満たす、というものである。

モデル: 調べなかった（課題の状態の定義、状態を動かす command の順や断り方、dispatcher と gate とレビューの順、役の間のやりとりは変えない。足すのは、端末を起動する前と gate の検査の前に打つ手だけである。gate は、この手が失敗したとき、PR の head を取れなかったときと同じく判定せずに返し、dispatcher が次の見回りで打ち直す。今ある道を通るだけで、新しい状態も新しい断り方も足さない）

## 読んで分かった事実（2026-11-25。main `a96bad7c` と、Andon を開発しているセルで実測）

実測は、セルの本物の mise の置き場に触れないよう、一時の置き場（`MISE_DATA_DIR`・`MISE_CACHE_DIR`・`MISE_STATE_DIR`・`MISE_CONFIG_DIR` を一時のフォルダへ向けた非対話のシェル）で行った。mise のバージョンは 2026.10.23 である。

**今の形**

- 入れた道具は `~/.local/share/mise` にある。合わせて 553MB で、内訳は go 1.27.3 が 283MB、node 26.13.0 が 230MB、golangci-lint 2.15.0 が 40MB である。shims も同じ置き場の `shims/` にある。
- `package/compose.yaml` が HOME の下に重ねる volume は 12 個で、`~/.local/share/mise` は入っていない。だから作り直しで消える。`~/.local/state` は volume `state` で、mise が覚えている設定の一覧（`~/.local/state/mise/tracked-configs`）はここに残る。
- image が mise について持つものは、aqua が入れる mise 本体、`~/.local/bin/mise` のリンク、ENV の PATH と sshd の `SetEnv` の PATH に入れた `~/.local/share/mise/shims`、対話のシェルの `mise activate`（`~/.bashrc`）、mise の補完、既定の go のバージョンを書いた `~/.config/mise/config.toml` である。最後の 1 つは go 本体を伴わない設定だけで、PR #623 が入れた。
- Andon の Go のコード（`internal/`・`cmd/`）は mise を 1 度も呼んでいない。`andon issue start` も `andon boot` も、宣言を満たす手を持たない。mise を呼ぶのは検査のシェルと `image/apply-main.sh` だけである。
- 役の端末は、プロファイルの `command` と `resume` の行から起動する。行は 9 つのプロファイルに 17 行ある。17 行とも `env $(andon otel env …) …` の形で、`andon otel env` が出した環境変数を役のプログラムへ渡している。この環境変数は、役が道具から打つシェルにも届く（Chief Engineer の端末で実測）。
- 端末を起動する口は 5 つある。`andon issue start`、`andon issue resume`、`andon chief-engineer`、`andon chief-of-staff`、dispatcher がレビュアーを起動する口である。5 つとも `hc run` でプロファイルを起動し、プロファイルの行を通る。
- `hc run` は、役のプログラムが入力を受けられるようになるのを待って最初のメッセージを送り、送れたかどうかが決まってから返る（hc の約束の説明）。`andon issue resume` は、届かなければ起動した端末を片付けて失敗する。`andon issue start` は終了コード 3 で返す。だから、プロファイルの行の中で長く待つ手を打つと、最初のメッセージが届かなくなる。`andon otel env` が行の中で待つのは 3 秒までである。
- 端末の cwd は役で違う。担当と Chief Engineer は自分の worktree の根である。レビュアーは kanban の課題のフォルダ（`issues/<レビュアーの課題>/`）で、PR の head を読む detached worktree ではない。Chief of Staff は kanban の root である。レビュアーと Chief of Staff の cwd から git の根を引くとセルの repo になり、そこに `mise.toml` は無い。
- 課題の worktree は kanban の `worktree` に書いてある。担当と Chief Engineer は起動の前に書く。レビュアーは、起動のあとに打つ `kanban start` が書く。
- レビュアーも案件の道具を使う。Andon を開発しているセルで mise が覚えている設定 1055 件のうち、271 件がレビュアーの worktree の `mise.toml` である。
- gate は、セルの条件表に検査の command があるとき、一時の worktree（`/tmp` の下）でその command を打つ（`internal/gate/gate.go` の `runChecks`）。`TMPDIR` は worktree の外へ向ける。command が落ちると判定は「直す」になる。PR の head を取れなかったときは判定せずに返り、dispatcher が次の見回りで打ち直す。
- `image/apply-main.sh` は、PATH に `go` が無いとき `mise which go` を聞き、取れなければ「go が無い」と止まる。

**mise の自動インストール**

- 空の置き場では、shim が 1 つも無いので、道具を打つと `command not found` になる。mise の自動インストールは効かない。shim は、その道具のどれかのバージョンが入ったあとに作られる。
- 同じ道具の別のバージョンが入っていれば、shim を打った時に、宣言のバージョンを自動で入れてから動く。
- どのバージョンも入っていない別の道具は、同じ `mise.toml` に書いてあっても shim が無く、`command not found` になる。
- `mise install` を 3 本同時に打っても、shim を 3 本同時に打って入れさせても、1 本が入れて残りは「もう入っている」で終わり、3 本とも正しいバージョンで動いた。
- 空の置き場へ go・golangci-lint・node の 3 つを入れる時間は、同じセルで 7 秒の回と 11 秒の回があり、go だけで 22 秒かかった回もあった。全部入っているときの `mise install` は 0.03 秒で終わる。
- `mise install` を途中で止めると、入れかけのフォルダは `~/.local/share/mise/installs/` に残り、入れかけの印は `~/.local/state/mise/incomplete-installs/` に残る。印があれば、次の `mise install` が入れ直す。印だけが消えると、mise は入っていると見なし、道具は引けないままになる。
- mise の cache（`~/.cache/mise`。143MB）を空にし、network を使わない設定（`MISE_OFFLINE=1`）にしても、入れた道具は動いた。cache は道具を動かすのに要らない。
- `[tools]` と値だけの `[env]` を書いた `mise.toml` は、`mise trust` を打たなくても `mise install` と shim が動いた。

**`mise.toml` の無い所**

- `mise.toml` が無く、既定の設定も無い所で shim を打つと、mise は「バージョンが決まっていない」と言って終了コード 1 で落ちる。
- `mise.toml` の無い所で go が引けなかった出来事は 2 件あり（PR #623 の根拠）、どちらも担当が一時の置き場で go を打った場面だった。
- 環境変数 `MISE_SYSTEM_CONFIG_FILE` に worktree の `mise.toml` を渡すと、`mise.toml` の無い所でも、その `mise.toml` のバージョンが引けた。mise の公式の説明では、この変数は `/etc/mise` の設定の代わりに使う 1 つのファイルを指し、設定の中でいちばん先に読まれて、あとから読む設定に負ける。実測でも、`~/.config/mise/config.toml` があればそちらが勝ち、近い `mise.toml` があればそれが勝った。
- 環境変数 `MISE_GLOBAL_CONFIG_FILE` でも同じことができるが、こちらは `~/.config/mise/config.toml` を読まなくする。セルが自分の既定を `~/.config/mise/config.toml` に書いていると、役の端末でだけ効かなくなる。
- どちらの変数も、指した先のファイルが無くても、`mise.toml` のある所の動きは変わらなかった。

**掃除**

- `mise prune` は、覚えている設定のどれも名指していないバージョンを消す。消えた設定の控えは先に掃き、動いているプロセスも確かめる。`--dry-run` で、消すものだけを出せる。

**隣で見つけたこと（今回は直さない）**

- ENV の PATH は、aqua の道具を mise の shims より先に置いている（PR #414 が決めた順）。aqua は Andon の備品として node を入れている。だから、役が道具から打つシェルでは、案件の `mise.toml` が node を別のバージョンで宣言しても、備品の node が先に引かれる。Andon を開発しているセルは両方が 26.13.0 なので、今は困っていない。

## 前提（同意済み。番号）

オーナーが 2026-11-25 に、5 つとも同意した。

1. 問題は、道具が消えることと、入れ直す役がいないことである。go のバージョンの選び方ではない。
2. 何もしないと、バージョンを上げる作り直しのたびに、同じ入れ直しが起きる。
3. 新しい道具は足さない。mise 本体、PATH の shims、今ある volume `state`、Andon が端末を起動する口で足りる。
4. 各セルには、次のバージョンへの作り直しで届く。その最初の作り直しでは volume が空なので、もう一度だけ入れ直しが起きるが、Andon が自動で入れる。
5. volume はセルごとに 1 つで、セルの間では共有しない。消しても正しさは壊れず、入れ直す時間がかかるだけである。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

日付はどれも 2026-11-25 である。

1. **範囲は、道具の volume、端末の起動で宣言を満たすこと、image の既定の go をやめること、`mise.toml` の無い所の既定までにする（オーナーが決めた）**

この 4 つを作る。備品の node が `mise.toml` の node より先に引かれることは、別のテーマにする。古いバージョンの掃除は自動にしない。

理由: 急ぐテーマである。node の順を直すには PATH の順を変えることになり、備品の node で動いている Playwright と portless と npm を確かめ直す分、届くのが遅れる。

捨てた案:

- **node の順も直す**: 上の理由で、別のテーマにした。
- **volume と宣言を満たす手だけにする**: image の既定の go を消すだけにすると、担当が一時の置き場で go を探し直す手間（2 件起きた）が戻る。

2. **言語の道具のバージョンの正は案件の repo の `mise.toml` で、image はバージョンを持たない（オーナーが決めた）**

base の image も、セルの image も、言語の道具のバージョンを焼かない。

理由: 写しを持つと必ず遅れる。そのうえ、go を上げる PR の worktree と main の worktree は、同時に別のバージョンを要る。image の 1 つの値では両方を満たせない。

細部: `image/Dockerfile` の最初の段 `bin` の golang は、Andon 自身の `kanban` と `andon` を焼くためのもので、セルの中で役が使う道具ではない。この段と、`tests/image.sh` がそのバージョンを `mise.toml` と突き合わせる検査は変えない（Chief Engineer が決めた）。

3. **base image が持つのは、mise 本体と、PATH に入れた shims の置き場までにする（オーナーが決めた）**

image は、道具を入れる仕組みだけを持つ。aqua が入れる mise 本体、`~/.local/bin/mise` のリンク、ENV の PATH の `~/.local/share/mise/shims` は、今のまま残す。

細部: volume の中の shims は `~/.local/bin/mise` を指す。このパスは volume をまたいで残る約束になるので、image は変えない（Chief Engineer が決めた）。変えるときは、同じ変更で、宣言を満たす手に `mise reshim` を足す。

4. **入れた道具は compose の volume に置き、消えてもよいキャッシュとして扱う（オーナーが決めた）**

volume は `package/compose.yaml` に足す。どのセルでも同じなので、セルの側の `compose.override.yaml` には書かない。volume は状態ではない。正しさは `mise.toml` が決め、volume を消しても、入れ直す時間がかかるだけである。

5. **volume の名前は `mise` で、`~/.local/share/mise` を丸ごと載せる。mise の cache は載せない（Chief Engineer が決めた）**

理由: 道具（`installs/`）と shims（`shims/`）は組で要る。shims だけが消えると、道具が残っていても `command not found` になり、shims を作り直す手がもう 1 つ要る。丸ごと載せれば、mise の置き場の中の分け方に Andon が立ち入らずに済む。cache は、空にしても入れた道具が動いた（上の事実）ので載せない。

捨てた案:

- **`installs/` だけを載せる**: 作り直しのたびに shims を作り直す手が要る。
- **cache も載せる**: 143MB を残しても、道具を動かすのに要らない。入れ直すときに取り直すだけである。
- **セルの間で 1 つの volume を分け合う**: 別のコンテナが同じ置き場へ同時に入れることになる。前提 5 で、セルごとに 1 つと決めてある。

細部:

- `package/compose.yaml` の `volumes` に `- mise:/opt/andon-home/.local/share/mise` を足し、末尾の一覧に `mise:` を足す。
- `image/Dockerfile` の、volume を重ねる置き場を先に作る `mkdir -p` に `"${HOME}/.local/share/mise"` を足す。無い場所に volume を重ねると root の持ち物になる。
- `internal/boot/owners.go` の `HomeVolumes` に `.local/share/mise` を足す。compose と揃っていることは、今ある `TestHomeVolumesMatchCompose` が見る。

6. **宣言を満たす手は、Andon が担当・レビュアー・Chief Engineer の端末を起動する直前に、その worktree で打つ（Chief Engineer が決めた。きっかけを Andon が持つことはオーナーが決めた）**

端末を起動する口のうち、worktree を持つ 4 つ（`andon issue start`、`andon issue resume`、`andon chief-engineer`、dispatcher がレビュアーを起動する口）は、`hc run` を打つ前に、その課題の worktree の根で宣言を満たす。根に `mise.toml` があれば、そこで `mise install` を打つ。無ければ何もしない。入れ終わってから端末を起動するので、役のプログラムは、道具が入った状態で始まる。

理由: 新しく起動する担当も、作り直しのあとに続きから起動し直す担当も、レビュアーも、起動の口を必ず通る。起動の口は worktree の場所を知っているので、cwd が worktree でないレビュアーでも、正しい `mise.toml` で入れられる。`hc run` より前に打つので、hc が最初のメッセージを待つ時間と重ならない。入っているときは 0.03 秒で終わるので、ふだんは待たない。

捨てた案:

- **プロファイルの行の中で打つ**（行が通す葉が `mise install` を打つ）: 起動の口を 1 つも変えずに済むが、行の中で待つ間、hc は役のプログラムの準備を待っている。空の置き場へ入れる 7〜22 秒がこの待ちに食い込み、最初のメッセージが届かなくなる（上の事実）。レビュアーの cwd からは worktree が引けない。
- **worktree を作るときにだけ打つ**: 作り直しのあとに続きから起動し直す担当は、worktree がもう在るので通らない。volume が空の作り直しのあとは、今回の事故と同じ `go: command not found` になる。
- **`andon boot` が、kanban に載せた repo の checkout で打つ**: boot は worktree の `mise.toml` を知らない。go を上げる PR の worktree のバージョンは入らず、worktree の側の手が別に要る。起動のたびに network を使う手が boot に増える。
- **mise の自動インストールに任せる**: 空の置き場では効かない（上の事実）。

細部:

- 宣言を満たす手は 1 つの関数にまとめ、4 つの口と gate（決めたこと 8）が同じものを呼ぶ。渡すのは worktree の根のパスである。
- `mise` が PATH に無ければ、何も打たずに先へ進む。
- 何かを入れたときだけ、打った command の stderr に、入れたことを 1 行出す。入っているときは何も出さない。
- `andon chief-of-staff` は worktree を持たないので、打たない。
- 検査は、本物の `mise install` を打たない。起動の口を通る検査は、偽の `mise` を PATH に置くか、`mise.toml` の無い repo を使う。HOME を替えた検査が本物の mise を呼ぶと、替えた HOME の下へ道具を入れ直してしまう。

7. **宣言を満たせなくても起動は止めず、警告を出して続ける。待つ上限は 60 秒で、過ぎても入れる手は止めない（Chief Engineer が決めた）**

`mise install` が失敗しても、起動の口は警告を 1 行出して、端末の起動へ進む。60 秒を過ぎても終わらないときは、`mise install` を止めずに手放し、警告を出して起動へ進む。

理由: network が落ちているときに、道具を使わない仕事（文書だけの課題・調べる課題）まで起動できなくなるのは、得より損が大きい。道具が要る担当は `command not found` を見て気づき、`mise install` を打てる。待つのは `andon issue start` を打った Chief of Staff か dispatcher の見回りなので、上限は短くする。実測の 7〜22 秒に対して、60 秒あれば足りる。過ぎたときに止めないのは、止めると入れかけが残り、入れかけの印が別の volume（`state`）にあるからである（上の事実）。手放した `mise install` は、そのまま入れ終える。同時に打たれた別の `mise install` や shim は、入れ終わるのを待ってから動く（上の事実）。

捨てた案:

- **失敗したら起動を止める**: 上の理由。
- **上限を置かない**: 詰まった network で、Chief of Staff の `andon issue start` と dispatcher の見回りがいつまでも返らない。
- **上限で `mise install` を止める**: 入れかけが残る。入れ直すには、もう 1 度最初から取り直すことになる。

細部:

- 警告は、何が起きたかと、打ち直す command（worktree の中で `mise install`）を書く。
- dispatcher がレビュアーを起動する口と gate は、担当が同じ commit の `mise.toml` で入れ終えたあとに通ることがほとんどで、そのときは待たない。
- `package/environment.md` の道具の節に、言語の道具は Andon が端末を起動する前に入れることと、無いときは repo の中で `mise install` を打つことを 1 行で足す。

8. **gate の検査と `image/apply-main.sh` も、道具を使う前に宣言を満たす。gate は、満たせなければ判定しない（Chief Engineer が決めた）**

`internal/gate/gate.go` の `runChecks` は、一時の worktree を作ったあと、検査の command を打つ前に、決めたこと 6 と同じ手を打つ。`mise install` が失敗したか、60 秒を過ぎても終わらないときは、検査の command を打たず、PR の head を取れなかったときと同じく、判定せずに失敗を返す。検査の command には、環境変数 `MISE_SYSTEM_CONFIG_FILE` に一時の worktree の `mise.toml` を渡す（決めたこと 10）。`image/apply-main.sh` は、PATH に `go` が無く `mise` があるとき、`mise which go` を聞く前に、Andon の checkout で `mise install` を打つ。

理由: どちらも役の端末を通らずに道具を使う。volume が空の作り直しのあと、担当を 1 人も起動しないうちに gate が走ると、検査の command が `command not found` で落ち、PR の中身と関係なく「直す」と判定してしまう。宣言を満たせないまま検査を打っても、同じ誤った判定になる。判定せずに返せば、dispatcher が次の見回りで打ち直す（上の事実）。検査の `TMPDIR` は worktree の外なので、検査が一時の置き場で道具を打つときも、決めたこと 10 の既定が要る。`image/apply-main.sh` は、同じ場面で「go が無い」と止まる。

捨てた案:

- **gate でも、満たせなければ警告だけ出して検査を打つ**: 道具が無いことを、PR の欠陥として担当へ返してしまう。

細部: 起動の口（決めたこと 7）と gate で、失敗の扱いだけが違う。起動の口は続け、gate は判定しない。打ち直しのたびに失敗が続くときは、dispatcher のログに、gate を回せない理由として `mise install` の失敗が出る。

9. **image の mise の既定の go の設定をやめる（オーナーが決めた）**

`image/Dockerfile` から `ARG DEFAULT_GO_VERSION` と、`~/.config/mise/config.toml` を書く行を消す。`tests/image.sh` の、この値を `mise.toml` と突き合わせる検査と、`mise.toml` の無い所で既定の go を引く検査も消す。PR #623 が入れた形を覆す。

理由: 決めたこと 2 の、image がバージョンを持たないことに反する。go 本体を伴わない設定だけなので、空の置き場では何も引けない。go を上げる PR の担当が一時の置き場で go を打つと、黙って古いバージョンが動く。

見える変化: オーナーの対話のシェルと Chief of Staff の端末では、repo の外で `go` を打つと、mise が「バージョンが決まっていない」と言って落ちる。repo と worktree の中は今までどおり引ける。担当とレビュアーと Chief Engineer の端末と、gate の検査は、決めたこと 10 で repo の外でも引ける。

捨てた案:

- **go 本体まで image に焼く**: image が 283MB 増える。バージョンの写しを image に持つことになり、決めたこと 2 に反する。
- **`andon boot` が、kanban に載せた repo の main の `mise.toml` から既定を作る**: 起動のたびに作り直す写しである。main が進んでも次の起動まで古い。kanban に載せた repo が 2 つ以上あると、どれを既定にするかが決まらない。go を上げる PR の担当が黙って古い go を引く点は、今と変わらない。

細部: セルが自分の既定を持ちたいときは、今までどおりセルの `local/setup.sh` で `~/.config/mise/config.toml` を書ける。決めたこと 10 の既定より、そちらが勝つ。

10. **`mise.toml` の無い所では、役の端末と gate の検査だけが、自分の worktree の `mise.toml` をいちばん弱い既定として引く。端末へは `andon tools env` が渡す（Chief Engineer が決めた）**

`andon tools env` という葉を足す。プロファイルの `command` と `resume` の 17 行は、`andon otel env` の隣でこの葉を通す。葉は、その端末の課題の worktree の根に `mise.toml` があれば、stdout に `MISE_SYSTEM_CONFIG_FILE=<worktree の根>/mise.toml` の 1 行を出す。プロファイルの行がこれを役のプログラムへ渡す。役が道具から打つシェルは、一時の置き場でも、repo の外へ展開した検査の中でも、自分の worktree の `mise.toml` のバージョンを引く。葉は読んで 1 行出すだけで、`mise` を呼ばず、待たない。

理由: 写しを作らないので、遅れない。go を上げる PR の担当は上げた go を、main の担当は main の go を、同じ時に引ける。`mise.toml` を書き換えれば、次に打った時から新しいバージョンが引ける。mise の設定の中でいちばん弱いので、セルの既定（`~/.config/mise/config.toml`）も、打った所に近い `mise.toml` も、今までどおり勝つ。`hc run` には環境変数を渡す口が無いので、プロファイルの行が、役のプログラムへ環境変数を渡せるただ 1 つの場所である。

捨てた案:

- **`MISE_GLOBAL_CONFIG_FILE` で渡す**: セルの `~/.config/mise/config.toml` が、役の端末でだけ読まれなくなる（上の事実）。
- **既定を持たず、担当の手順に書く**: 担当が repo の外で go を打つたびに、打つ所を変えるか実体を探し直す。2 件起きたことが戻る。
- **`andon boot` が既定を作る**: 決めたこと 9 の捨てた案と同じ。
- **葉が cwd の git の根から worktree を引く**: レビュアーの cwd は kanban の課題のフォルダで、根を引くとセルの repo になる（上の事実）。

細部:

- 葉の名前は `andon tools env`。引数は取らない。`internal/cli/leaves.go` の表に載せ、`docs/commands.md` と補完を作り直す。
- worktree は、その端末の課題の kanban の `worktree` から引く。端末から課題を引く手は、`andon otel env` と同じ（端末のラベルの `project` と `issue`）にする。課題が引けないか、`worktree` が空のときは、打たれた所の git の根を使う。
- レビュアーは、今は起動のあとに `worktree` が書かれる。dispatcher は、レビュアーを起動する前に、レビュアーの課題の `worktree` を kanban に書く。
- 引いた根に `mise.toml` が無ければ、何も出さずに終了コード 0 で終わる。Chief of Staff の端末と、repo の無い課題の端末は、この道を通る。
- 根のパスに空白の文字があるときは、行を出さずに警告を 1 行出す（`env $(…)` が語で割るため）。
- プロファイルの行は、Claude Code の行が `env $(andon otel env) $(andon tools env) CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude …`、Codex の行が `env $(andon otel env --codex-env) $(andon tools env) codex …` の形になる。`andon otel env` の検査（`internal/otel/env_test.go`）が行の形を字面で見ているので、同じ PR で直す。起動の行の形を写している [contract.md](../contract.md) §30 も、同じ PR で直す。
- セルが自分で足したプロファイル（セルの repo の `templates.local/`）には届かない。同じ句を写すことを [image.md](../image.md) のプロファイルの節に書く。
- 変数を持つのは、Andon がプロファイルから起動した端末とそこから起動したプロセス、それに gate の検査の command だけである。オーナーの対話のシェルと、dispatcher 自身は持たない。
- worktree を片付けたあとも端末が残っていると、変数は無いファイルを指す。そのとき `mise.toml` のある所の動きは変わらない（上の事実）。

11. **古いバージョンの掃除は自動でしない。手で打つ command を運用の手引きに書く（オーナーが決めた）**

[operate.md](../operate.md) に、道具の volume の節を足す。書くのは、volume がキャッシュであること、古いバージョンを消す `mise prune`（先に `mise prune --dry-run` で消すものを見る）、丸ごと消すときの手（セルを止めて volume を消す。次に端末を起動するときに入れ直す）、volume `state` だけを消さないこと（入れかけの印がそこにある。`state` を消すなら `mise` も消す）である。

理由: 増えるのは、go を 1 つ上げるごとに約 283MB、node を 1 つ上げるごとに約 230MB である。急いで消す量ではない。自動で消す手は、動いている検査が使っているバージョンを消す事故の種になる。

捨てた案:

- **`andon boot` が `mise prune` を打つ**: boot はセルが動いている間にも打ち直せる。消してよいかを、mise が覚えている設定の一覧だけに頼ることになる。得る量に対して、事故の種が大きい。

12. **実装は 2 本に分け、端末の起動で宣言を満たす変更を先に入れる（Chief Engineer が決めた）**

1 本目は、起動の口と gate と `image/apply-main.sh` が宣言を満たす手と、`andon tools env` とプロファイルの行である（決めたこと 6・7・8・10）。2 本目は、volume と、image の既定の go をやめる変更である（決めたこと 3・5・9・11）。

理由: 1 本目は、セルを作り直さずに当てられる。当てた時から、道具が消えても、次に端末を起動するときに入れ直す。今回の事故は、volume が届く前に止まる。2 本目は compose と image に触れるので、オーナーがマージし、セルの作り直しで届く。image の既定の go を先に消すと、決めたこと 10 の既定が無い間、`mise.toml` の無い所で go が引けなくなる。だからこの順にする。

## やらないこと（範囲の外。理由）

- **備品の node が `mise.toml` の node より先に引かれることを直す**: 決めたこと 1 で別のテーマにした。事実は「隣で見つけたこと」に書いた。
- **古いバージョンを自動で掃除する**: 決めたこと 11。
- **mise の信頼（`mise trust`）を Andon が足す**: `[tools]` と値だけの `[env]` は信頼が要らなかった。信頼が要る書き方の `mise.toml` では、`mise install` が落ちて警告になり、起動は続く。要るセルが出たら、別のテーマにする。
- **`mise.toml` 以外の宣言（`.tool-versions` など）を見る**: 正は `mise.toml` と決めた（決めたこと 2）。
- **オーナーの対話のシェルと Chief of Staff の端末に、repo の外の既定を持たせる**: どの repo のバージョンを引くかが決まらない。repo の中で打てば引ける。
- **`andon boot` に宣言を満たす手を足す**: 決めたこと 6 の捨てた案。kanban に載せた repo の checkout の中で、手で道具を使う人は、道具が無ければ `mise install` を 1 度打つ。
- **`andon chief-of-staff` の起動で宣言を満たす**: Chief of Staff は worktree を持たない。
- **HOME を替えた検査の中で mise の置き場を保つ**（`MISE_DATA_DIR` を ENV に固定する）: 検査の隔離の話で、今回の事故の原因ではない。
- **セルの側のつなぎ（`compose.override.yaml` など）を作る**: オーナーの依頼。
- **段 `bin` の golang のバージョンの決め方を変える**: 決めたこと 2 の細部。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 4 つ使う。起動の口と gate と葉の動きは、偽の `mise` を PATH に置いた Go の検査と、今ある `tests/andon-issue.sh` などの起動の検査で見る。image の中身は、今ある `tests/image.sh` の 3 つの段（docker の要らない静的な段、焼いた image を `docker run` する段、compose で起動する段）で見る。docker の段は CI の tests の job で走る（セルには docker が無い）。セルの本物の動きは、当てたあとの検証で見る。検査は、セルの本物の mise の置き場に書かず、本物の `mise install` を打たない。

1 本目（端末の起動で宣言を満たす）:

1. worktree の根に `mise.toml` があるとき、`andon issue start`、`andon issue resume`、`andon chief-engineer`、dispatcher がレビュアーを起動する口は、どれも `hc run` より前に、偽の `mise` を、worktree の根を cwd にして引数 `install` で 1 回呼ぶ。レビュアーでは、cwd が kanban の課題のフォルダでなく、PR の head の worktree の根である。`mise.toml` が無いときは呼ばない。証拠: `go test ./...` の、4 つの口それぞれの検査の出力（呼び出しの記録に、偽の `mise` と `hc run` の順が出る）。
2. 偽の `mise` が終了コード 1 で終わるときも、上限を過ぎても終わらないとき（検査では上限を短くする）も、4 つの口は警告を出して `hc run` を打つ。過ぎたとき、偽の `mise` のプロセスは止められずに残り、あとで自分で終わる。証拠: 同じ。
3. gate の `runChecks` は、一時の worktree の根に `mise.toml` があるとき、最初の検査の command より前に偽の `mise` を `install` で呼び、検査の command の環境に `MISE_SYSTEM_CONFIG_FILE=<一時の worktree>/mise.toml` を渡す。偽の `mise` が失敗するか上限を過ぎると、検査の command は 1 つも打たれず、`runChecks` は失敗を返して、判定の記録に「直す」が書かれない。証拠: `go test ./internal/gate/...` の出力。
4. `andon tools env` は、端末の課題の kanban の `worktree` の根に `mise.toml` があるとき、stdout に `MISE_SYSTEM_CONFIG_FILE=<根>/mise.toml` の 1 行だけを出す。打った所が kanban の課題のフォルダ（レビュアーの cwd）でも、出るのは課題の worktree の根である。課題が引けないときは、打った所の git の根で同じことをする。根に `mise.toml` が無いときは、stdout が空で終了コード 0 になる。どの場合も `mise` を呼ばない。証拠: `go test ./...` の、この葉の検査の出力。
5. dispatcher がレビュアーを起動するとき、`hc run` より前に、レビュアーの課題の kanban に `worktree` が書いてある。証拠: `bash tests/andon-dispatcher.sh` の、レビュアーを起動する場面の出力。
6. `package/templates/` の 17 行が、どれも `$(andon tools env)` を `env` に渡している。渡さない行を 1 つ作ると検査が落ちる。証拠: `go test ./...` の、プロファイルの行の検査の出力。
7. `image/apply-main.sh` は、PATH に `go` が無く `mise` があるとき、`mise which go` より前に、Andon の checkout で `mise install` を打つ。証拠: `bash tests/apply-main.sh` の出力（偽の `mise` の呼び出しの記録）。
8. `docs/commands.md` と補完が、葉の表から作り直したものと同じである。証拠: `bash tests/commands.sh` の出力。
9. 起動の口を通る検査が、替えた HOME の下へも、セルの本物の置き場へも、道具を入れない。証拠: PR の checks の job isolation（`tests/isolation`）の結果。

2 本目（volume と、image の既定の go をやめる）:

10. `package/compose.yaml` が volume `mise` を `/opt/andon-home/.local/share/mise` に載せ、`HomeVolumes` と揃っている。証拠: `go test ./internal/boot/...` の `TestHomeVolumesMatchCompose` の出力。
11. `image/Dockerfile` に `DEFAULT_GO_VERSION` と、`~/.config/mise/config.toml` を書く行が無い。否定の検査には、当たる場合があることを確かめる行を添える。証拠: `bash tests/image.sh` の静的な段の出力。
12. 焼いた image の非対話のシェル（`bash --norc --noprofile`）で、`mise.toml` のある所から `go version` が `mise.toml` のバージョンを出す（今ある検査を残す）。`mise.toml` の無い一時の置き場からは、`MISE_SYSTEM_CONFIG_FILE` にその `mise.toml` を渡したときに同じバージョンを出す。証拠: CI の tests の job の `tests/image.sh` の出力。
13. compose で起動したセルで、`~/.local/share/mise` が andon の持ち物である。volume の中（`~/.local/share/mise/installs/` の下）に本体を置いた偽の go が、コンテナを作り直したあと（`docker compose down` のあと `up`。volume は消さない）も、非対話のシェルから引ける。偽の go を `mise link` で volume の外のパスへつなぐ形は使わない（置かれるのは外への symlink で、作り直しで先が消える）。証拠: CI の tests の job の `tests/image.sh` の compose の段の出力。
14. Andon を開発しているセルで、2 本目を含む image へ作り直したあと、誰も `mise install` を打たずに、新しく起動した担当の端末、続きから起動し直した担当の端末、レビュアーの端末の 3 つで、worktree の中の `go version` と、一時の置き場の `go version` が `mise.toml` のバージョンを出す。Codex のプロファイルの端末でも、`MISE_SYSTEM_CONFIG_FILE` が道具のシェルに届いている。もう 1 度作り直したあと、`~/.local/share/mise/installs` の道具の更新時刻が、作り直しより前のままである。証拠: オーナーが作り直しを打ち、当てたあとの検証の課題が見る（`manual`）。

## 価値の仮説

1. 誰の何が変わるか: Andon を使うオーナーと Chief of Staff が、セルを作り直したあとに言語の道具を手で入れ直さなくなる。担当の検査が、作り直しのあとに `go: command not found` で落ちなくなる。
2. 見る信号: 信号の記録に当たる出所は無いので、オーナーの目視にする。見るものは、受け入れ条件 14 の 3 つ（誰も `mise install` を打っていないこと、起動した端末で `go version` が出ること、2 度目の作り直しで道具の更新時刻が動かないこと）である。
3. 見る日: 2026-12-09。2 本目のマージの見込み（2026-11-27 ごろ）から、バージョンを上げる作り直しが 2 回済む見込みの日である。
4. 誰が要るか: オーナー。作り直し（`bin/bootstrap rebuild`）を打つ。
5. 効いたとどう分かるか: 2 本目を含むバージョンへ上げたあとの 2 回の作り直しで、手の `mise install` が 0 回で、`command not found` で落ちた検査が 0 件である。
6. 何があれば止めるか: volume に残った道具が原因の壊れ（古い shims・持ち物の食い違い・壊れた入れかけ）が 2 回起きたら、volume をやめ、端末の起動のたびに入れ直す形（1 本目だけの形）へ戻す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **端末の起動で宣言を満たす。** 宣言を満たす関数と、それを呼ぶ 4 つの起動の口と gate の `runChecks`、dispatcher がレビュアーの `worktree` を起動の前に書くこと、`andon tools env` の葉、プロファイルの 17 行、`image/apply-main.sh`、`package/environment.md` の 1 行、[contract.md](../contract.md) §30 の起動の行の写し、[image.md](../image.md) のプロファイルの節、葉の表と `docs/commands.md` と補完、検証の地図。決めたこと 6・7・8・10、受け入れ条件 1〜9。依存は無い。プロファイルと `image/` に触れる。作り直さずに当てられる。
2. **道具の volume を足し、image の既定の go をやめる。** `package/compose.yaml`、`image/Dockerfile`、`HomeVolumes`、`tests/image.sh`、[image.md](../image.md) の冒頭と検査の表、[operate.md](../operate.md) の道具の volume の節、`AGENTS.md` の Go のバージョンの行。決めたこと 3・5・9・11、受け入れ条件 10〜13。1 に依存する（1 が当たったあとにマージする）。compose と image に触れるので、オーナーがマージする。
3. **当てたあとの検証。** 受け入れ条件 14。2 に依存し、オーナーの作り直しが要る。

## 開いている問い（残った問い。決断の種類つき）

無い。決めたこと 9 は、2026-11-25 の出口の問いにオーナーが答えた。オーナーは同じ日に、この文書の全体に「決めた」と答えた。

## 出典

- オーナーの発言: 2026-11-25（依頼の経緯。ホストの担当との話を Chief of Staff が中継した）、2026-11-25（入口の言い直しと前提 5 つへの同意、範囲の答え）、2026-11-25（出口の問いで、image の既定の go の設定を消すと答えた）。
- repo の中の文書とコード: [image.md](../image.md)、[operate.md](../operate.md)、`image/Dockerfile`、`image/sshd_config.d/andon.conf`、`image/apply-main.sh`、`package/compose.yaml`、`package/templates/`、`internal/boot/owners.go`、`internal/boot/start.go`、`internal/spawn/spawn.go`、`internal/issue/start.go`、`internal/issue/resume.go`、`internal/dispatcher/loop.go`、`internal/gate/gate.go`、`internal/otel/env.go`、`internal/chiefengineer/chiefengineer.go`、`internal/chiefofstaff/chiefofstaff.go`、[contract.md](../contract.md) §30、`tests/image.sh`、`tests/lib/run-tests.sh`、`mise.toml`。
- 前の変更: PR #414（mise の shims を image の PATH に入れた）、PR #623（image に mise の既定の go の設定を入れた）。
- 外の文書: mise の設定の説明 https://mise.jdx.dev/configuration.html と https://mise.jdx.dev/configuration/settings.html （`system_config_file`・`global_config_file` と、設定を読む順）。
