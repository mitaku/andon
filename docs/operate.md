# セルを立てて1本流す（手順）

- 状態: **手順**（2026-11-05 起草。[design.md](design.md) §5.2 の P7 の準備）。読み手は人。ホスト（docker と bash がある Linux か WSL）で打つ。
- 入力（正）: andon main の `README.md`・`init/`・`internal/plantinit`・`internal/boot`・`docs/image.md`・[dispatcher.md](dispatcher.md)・[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md)。コードを読んで書いた。**打って確かめたものではない**（実測は P7 で人がする。この文書の「通ったら見えるもの」と食い違えば、実測が正で、この文書を直す）。
- 書き方: `<owner>`（GitHub の持ち主）・`<版>`（image の tag）・`<email>` は自分の値に置き換える。セルの名前（slug）は例として `home`。

## 役の配置と起動（一覧）

誰が・どの端末で・何をきっかけに起こすか・止まったら誰が戻すか。コードで確かめた形（`internal/boot`・`internal/chiefofstaff`・`internal/chiefengineer`・`internal/issue`・`internal/dispatcher`）。ほかの文書（[dispatcher.md](dispatcher.md)・[contract.md](contract.md) §8・skill）はここへリンクする。ラベルの値の正は contract.md §8。

| 役 | 端末の名前 | 出る場所（Space / tab） | 起こすのは・きっかけ | 止まったら誰が戻すか |
|---|---|---|---|---|
| dispatcher | `dispatcher`（セルに1つ） | 台帳が1つならその指揮側の Space の tab「dispatcher」、2つ以上なら `<セルの slug> plant` の tab「dispatcher」 | boot（起動のたび。`andon boot`）。`hc run --keep` で起こす | boot と Chief of Staff のフックが、前に出ているものが `andon dispatcher` でなければ片付けて起こし直す（dispatcher.md §5）。作り直しの直後は前に出ているものが取れるまで boot が数秒読み直し、取れなければ「確かめられなかった」と書く（「すでに動いている」とは書かない）。戻らなければ人が `andon boot` |
| Chief of Staff | `<slug>-cos` | 指揮側の Space `<slug>` の tab「CoS」 | 人が `projects/<slug>/` で `andon chief-of-staff`（動いていればそれを使う） | 人が `hc kill` してから打ち直す。起き直した Chief of Staff 自身も同じ command を打てる。作り直しで戻った端末（top-level の argv ごと素のシェル）は、打ち直せば command が数えずに片付け（keep を外して kill・rm）、起こし直して chief_of_staff を合わせる。数えるのは hc の agent が Claude Code の端末だけで、どちらとも言えない端末（人が中で使っているなど）があれば触らずに止まる |
| Chief Engineer | `<slug>-<id を小文字>` | 指揮側の Space `<slug>` の tab（課題ごと） | Chief of Staff か人が `andon chief-engineer "<題>"`（Chief of Staff は経緯を `--brief -` で stdin から渡し、依頼書の既定の行の下に入る）。1プロジェクトに同時に複数いてよい | 最初のメッセージが届かなければ dispatcher が送り直す。それ以外は Chief of Staff が `hc read` で見て人へ。作り直しで戻った端末（top-level の argv ごと素のシェル）・端末が無い Chief Engineer は、`andon issue resume <id>` が同じ worktree でセッションの続きから起こす（記録が無ければ止まり、人の go で `--fresh`）。`andon chief-engineer` はほかの Chief Engineer の端末に触らない（[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 12）。Stop フック（`andon issue stop-check`）は Chief Engineer の課題を止めない（人との対話の間、報告も問いも書かずにターンを終えるのが正常） |
| 担当 | `<slug>-<id を小文字>` | 作業側の Space ＝ 課題（[ADR-0003](adr/0003-task-spaces.md)）。ラベルの id は `andon-<slug>-<id>`、App での名前は `<id> <題>`、tab は `<id>` | Chief of Staff が `andon issue start <id>` | 最初のメッセージが届かなければ dispatcher が送り直す。止まりは dispatcher が `<news>` で Chief of Staff に知らせ、Chief of Staff が `hc send` で続けさせる。作り直しで戻った端末（top-level の argv ごと素のシェル）は、dispatcher が終わった端末（`finished`）として知らせる。`doing` のままなら `andon issue resume <id>` で同じ worktree・branch のままセッションの続きから起こす（worktree が消えていれば戻す。記録が無ければ止まり、人の go で `--fresh`。[ADR-0012](adr/0012-resume-and-worktree-home.md)） |
| レビュアー | `<slug>-<レビューの id を小文字>` | 担当と同じ Space・tab に横に並ぶ | dispatcher。担当の `andon issue review-request` の報告を拾って起こす | dispatcher が起こしたものも、担当と同じく最初のメッセージが届かなければ（`hc run` の `sent: false`、か `sent: true` でも `started: false`）ラベル `first_message.sent=false` を付けて預け、dispatcher が次の見回りでレビュアーの文を送り直す（[dispatcher.md](dispatcher.md) §5）。それ以外は担当の依頼で Chief of Staff が起こし直す |

- **dispatcher とゲート**: 担当の `andon issue ready`（`status: review`）を拾った dispatcher が `andon gate` を回す。ゲートは PR の head の GitHub の checks が揃うまで `wait`（終了コード 5）を返し、**通知も問いも出さない**。dispatcher が見回りのたびにゲートから回し直す。checks が1つも無いまま 30 分たつと `needs_human`。
- **最初のメッセージ**: 担当・Chief Engineer・レビュアーの起こしは `hc run` で最初のメッセージを渡す。届かなかった（`sent: false`、か `sent: true` でも `started: false`＝送ったが動き出さない）ら、端末にラベル `first_message.sent=false` が付き、dispatcher が見回りで入力を受けられる状態（`idle`）になるのを待って送り直す。人が手で打つのは、急ぐときだけ。
- **信頼の画面**: セルでは出ない（boot と、起こす直前の `andon issue start`・`andon chief-engineer` が repo の根を信頼済みにする。[image.md](image.md)）。出たら想定外なので、Chief of Staff は `hc keys` で答えず、人へ。
- **役のラベルの付いた端末は Andon の持ち物**: ラベル `role=chief-of-staff`・`role=chief-engineer`・`issue=…` の付いた端末で中身が素のシェル（起こした command も前に出ているものも引数の無いシェル）なら、作り直しで戻ったものとして `andon chief-of-staff`・`andon chief-engineer` が片付ける。人がシェルとして使う場所ではないので、シェルが要るなら別の端末を開く。dispatcher の `finished` の知らせ（担当・レビュアーの戻った端末）は知らせるだけで、片付けはしない。
- **Chief of Staff が答えてよい範囲**: 担当の許可の確認で、課題の範囲内のものだけ（skill chief-of-staff の §4）。範囲外・不可逆は人。auto mode の判定が `hc keys` を拒んだら、許可ルールを足さず人へ回す。

## セルの置き方

セルの repo（`~/src/<slug>-cell`。コンテナの `/workspace`）と、製品の repo（ソース）を分ける。

- `projects/<slug>/` は**セルの repo の実ディレクトリ**（symlink にしない）。台帳（`kanban/`。git の外）と、人が書くプロジェクトの事実（`project.json`: 書いてよい repo・委譲など。[contract.md](contract.md) §11）を置く。
- ソースは ghq の製品の repo（`/workspace/.ghq/<host>/<owner>/<repo>`）。担当は `/workspace/.worktrees/<repo 名>/<slug>-<id>` の worktree で働く（セルの repo の中。置き場の `.gitignore` が `*` なのでセルの git には出ない。[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 6）。
- セルの repo で `git clean -x` を打たない（`.worktrees/` が消える。`git status` には出ないが `git clean -ndx` は消すと言う。[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 13 の 6）。
- 台帳とソースは `config.yaml` の `repos` でつなぐ（repo が複数でも並べるだけ）。書いてよい repo の正は `project.json` の `repos`（`config.yaml` の `repos` はその部分集合。`andon issue start` が両方を確かめる）。
- 製品の仕様・設計の文書は、製品の repo の `docs/` に置く。
- `projects/<slug>/` の予約名（`project.json`・`AGENTS.md`・`.claude/`・`kanban/`）と誰が書くかは [ADR-0004](adr/0004-project-layout.md)、セルの repo の直下・git で追跡するか（台帳は追跡しない）・別のマシンへの動かし方・人のファイルの守りは [ADR-0005](adr/0005-cell-repo.md)（どちらも提案。マージで決定。実装は台帳の改名と一緒）。
- **納品物 `files/`**: `andon publish` が `<日付>-<slug>` で置き、files サーバが公開する。git で追跡する（ADR-0005）。1 ファイル 20 MiB を超えるものは `andon publish` が拒む。
- **symlink にしない理由**: 委譲は「人だけが書く」。これを、セルの repo は人のもの、担当は製品の repo の worktree で書く、という分け方で守っている。`projects/<slug>` を製品の repo への symlink にすると `project.json`（委譲・書いてよい repo）が製品の repo に入り、担当の PR で書き換えうる。

## 0. 先に知っておくこと: 通らなかった所（直した）

コードを読んで見つけた3つは直した。F1「image に `andon` が焼かれていない」と `package/environment.md` が無いこと（image が `/opt/andon/bin/andon` を持ち、`tests/image.sh` が CI の実ビルドで 2・3・5 の道を通す）、F2「`andon chief-of-staff` の中身が無い」（9）、F3「`andon issue start` の中身が無い」（10）。

P7 で見つけた F4「セルの `andon` が uid 1001 で、ホストの利用者（uid 1000）の持ち物のセルの repo を読み書きできない」も直した。`andon` は uid・gid 1000 で動く（WSL・cloud-init の既定の利用者と同じ）。`.ssh/authorized_keys` が読めず `ssh は無効` になる・`mkdir /workspace/artifacts: permission denied` の警告・7 の `ghq get` と 8 の `mkdir` が落ちる、はこれが原因だった。前の image で作った volume（中身が uid 1001 の持ち物）は、boot が起動のたびに `andon` の持ち物へ直すので、作り直さなくてよい（`down -v` は要らない）。

P7 で見つけた F5「Herdr App から起こした端末に `claude` が見えず、Chief of Staff が `env: 'claude': No such file or directory` で落ちる」も直した。sshd が image の環境を引き継がず、App が SSH 越しに起こす herdr-core serve の `PATH` に `~/.local/bin` と aqua の道具が無かった。image が sshd に環境を渡し、boot が herdr-core を `~/.local/bin`（App が置く場所）に揃える（4 の「Herdr App からセルへつなぐ」）。

### 前の名前から移す（`artifacts/` → `files/`）

前のセルは納品物を `artifacts/` に置いていた。`files/` に改めたので、セルの中で一度だけ移す。

```bash
cd /workspace
mv artifacts files           # 無ければ何もしない（files/ が既にあれば中身を files/ へ移す）
git add files                # 追跡する（.gitignore の artifacts/ の行は消してよい）
# files サーバは古い artifacts/ を見ているので、起こし直す（ポートから PID を引いて止め、boot をもう一度）
port=$(grep -m1 '^FILES_PORT=' .env | cut -d= -f2)   # .env に無ければ boot の既定の番号
pid=$(ss -ltnp "sport = :$port" | grep -o "pid=[0-9]*" | head -1 | cut -d= -f2)
[ -n "$pid" ] && kill "$pid"
andon boot
```

URL（`http://files.<セル>.localhost:<番号>/…`）は変わらない。

## 1. image を取る

**やること**: image を取る。GHCR の package `andon` は Public なので、`docker login` なしで pull できる（token は要らない）。

```bash
docker pull ghcr.io/<owner>/andon:<版>      # <版> は vX.Y.Z（配る版）。tag を切るまでは、いまある 40桁の commit SHA の版
```

版ごとの変更と、上げるときにすることは [CHANGELOG.md](../CHANGELOG.md)。

- **通ったら見えるもの**: `docker image ls ghcr.io/<owner>/andon` に行が出る。
- **落ちたら見る所**:
  - `denied`・`unauthorized` — image の名前（`ghcr.io/<owner>/andon`）と owner の綴りを見る。古い `docker login` の資格情報が邪魔をしていることがある（`docker logout ghcr.io` してから pull し直す）。fork や自前の registry に置いた private の image を取るときだけ、`read:packages` を持つ token で `docker login` する。
  - `manifest unknown` — GHCR に版が出るのは `vX.Y.Z` の tag を push して tests の job が通ったときだけ（[image.md](image.md)）。main へのマージでは出ない。P7 のように tag を切る前のセルは、すでに GHCR にある 40桁の commit SHA の版（main へのマージで出していた頃のもの）を使う。その版のまま、日々の直しは 3 の「andon を直してセルで試す」で手元で焼く。
  - `no matching manifest` — 配布は Linux amd64 だけ（[image.md](image.md)）。

## 2. セルの repo を作る（`andon init`）

セルの repo は `andon init <slug>` が作る（雛形は `init/`。作ったあとは Andon に追従しない）。置き場は `~/src/<slug>-cell`（`andon-up` が `$HOME/src/*-cell` と、前の形の `$HOME/src/andon-*` を探す。`ANDON_PLANTS` で変えられる）。

前の形の名前（`~/src/andon-<slug>`）で作ったセルは、そのままでよい。改めるなら `bin/bootstrap down` → `mv ~/src/andon-<slug> ~/src/<slug>-cell` → 新しい場所で `bin/bootstrap up`（compose の project と volume は slug から決まるので残る。`~/.config/andon/skip` に書いた名前は書き直す）。

ホストの `andon` は image から取り出す（release は無い）。image の中の `andon` は linux の静的バイナリで、配布は amd64 だけ（ホストが Linux か WSL の amd64 なら、そのまま動く）。

```bash
mkdir -p ~/.local/bin ~/src
# 一時ファイルに取り出してから置き換える（取り出しが落ちても、前の andon を空にしない）
docker run --rm ghcr.io/<owner>/andon:<版> cat /opt/andon/bin/andon > ~/.local/bin/andon.new \
  && install -m 755 ~/.local/bin/andon.new ~/.local/bin/andon && rm ~/.local/bin/andon.new
# image の版と揃っているか（2つの sha256 が同じなら揃っている）
docker run --rm ghcr.io/<owner>/andon:<版> sha256sum /opt/andon/bin/andon ; sha256sum ~/.local/bin/andon
cd ~/src && andon init home --image ghcr.io/<owner>/andon:<版> --owner <email>
```

- image を上げたら取り出し直す（ホストの `andon` は image の版と揃える。揃っているかは上の sha256 の2行で比べる）。`~/.local/bin` が PATH に無ければ足す。
- `--owner` は `andon.toml` の `owner`。`project.json`（委譲・書いてよい repo）をこの email の commit だけが信じられるようにする。
- **通ったら見えるもの**: `~/src/home-cell/` に `Dockerfile`（`FROM` ＋ 2行）・`andon.toml`（`slug = "home"`）・`bin/bootstrap`・`local/`・`gate.local.json`・`AGENTS.local.md`・`wiki/`・`projects/`。標準出力に `セルの repo を作った`。パッケージのファイルは無い（design.md §5.1 の 1）。
- **落ちたら見る所**: 取り出しの `docker run` が落ちた（`manifest unknown` など）— 1 の pull を先に通す。`~/.local/bin/andon.new` が残っていれば消す。`はすでにあり、空でない` — 上書きしない。別の `--dir` か、中身を退避する。`slug は英小文字・数字・-` — slug の綴り。
- セルの repo を git で持つなら `git init` して commit する（`.ghq/`・`.env` などは雛形の `.gitignore` が外している）。

## 3. 起動する（`bin/bootstrap`）

セルの repo は **uid 1000 の利用者**の持ち物にする（セルの `andon` は uid 1000 で動き、セルの repo をそのまま `/workspace` に載せる）。WSL・cloud-init の既定の利用者は 1000（`id -u` で確かめる）。違うと `bin/bootstrap` は起こす前に止まる。

```bash
cd ~/src/home-cell
bin/bootstrap --dry-run     # docker を打たずに、打つものを出す
bin/bootstrap               # 初回: image を焼く → compose.yaml を取り出す → 起動 → herdr に入る
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）はセルの `.env` に書く。**セルを2つ以上立てるなら、ずらす**（ホストの同じ番号を取り合う）。書かなければ既定。

**`.env` に番号を書く前に、ホストで使われている番号を確かめる**（既定は `SSH_PORT=2222`・`PORTLESS_PORT=1355`。ほかのセル・別の docker・ホストのほかのものと同じ番号だと、起動がポート使用中で落ちる）。

```bash
docker ps -a --format '{{.Names}}\t{{.Ports}}'     # 公開している port（止まっているコンテナも拾う）
ss -ltn                                            # いまホストで listen している番号
grep -rn -i '^ *\(Port\|LocalForward\)' ~/.ssh/config ~/.ssh/config.d/    # ssh config で使っている番号
```

- WSL＋Windows の App なら、Windows 側の ssh config（`C:\Users\<user>\.ssh\config`。4 の「WSL＋Windows の App から入る」）も同じ grep で見る。
- **通ったら見えるもの**: 選んだ `SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT` のどれも、上の3つの出力に出てこない。選んだ番号を `.env` に書いてから `bin/bootstrap`。
- **落ちたら見る所**: `bin/bootstrap` が `port is already allocated`・`address already in use` で落ちる — 出た番号を上の3つで引き、持ち主の側を動かすか、`.env` の番号を変えて打ち直す。起動のあとで `.env` の番号を変えたなら、`host-install` が置いた ssh config の `Port`・`LocalForward` も同じ番号にする（4 の落ちたら見る所）。

- **何が起きるか**: `Dockerfile` の `FROM` から `andon-home:local` を焼く（`local/setup.sh` を走らせる）→ image の `/opt/andon/compose.yaml` を `.andon/` に取り出す → `docker compose up -d` → ログに `[andon boot] ready` が出るまで待つ（最長5分）→ `herdr` を開く。もう一度打てば同じセッションに戻る。
- **通ったら見えるもの**: `bin/bootstrap status` で `dev` が Up。`bin/bootstrap logs` に `[andon boot] ready`（と portless・sshd・dispatcher を起こした行）。herdr の画面が開く。andon が uid 1001 だった版の image から上げた最初の起動では、`持ち主を直した: /opt/andon-home/…（uid 1001 → 1000）` の行も出る（volume ごとに1回だけ）。
- **落ちたら見る所**:
  - `Dockerfile の FROM を埋める` — 2 の `--image`。
  - `セルの repo（…）の持ち主が uid <n>` — セルの repo が uid 1000 の利用者の持ち物でない。uid 1000 の利用者で 2 の `andon init` からやり直す（uid を合わせ直す仕組みは無い）。
  - `docker デーモンに繋がらない` — docker が起きているか。
  - 5分待っても `ready` が出ず `準備の完了を確かめられなかった` — `bin/bootstrap logs` で boot の警告の行を見る。`andon help` が `command not found` なら `FROM` の image が古い（`andon` を焼く前の版）。
  - ポートが使用中 — `.env` の番号を変えて `bin/bootstrap` を打ち直す。
  - `local/setup.sh` が落ちる — `bin/bootstrap` の build のログ。セル固有の追加（`local/aqua.yaml`・`local/setup.sh`）が原因。

### andon を直してセルで試す（日々のサイクル: `rebuild --from`）

andon を直したら、マージも GHCR も通さず、セルの中の andon の checkout（7 で迎えたもの。ブランチのままでよい）から image を焼いて、その image でセルを作り直す。ホストで、セルの repo から打つ。

```bash
cd ~/src/home-cell
bin/bootstrap rebuild --from .ghq/github.com/<owner>/andon    # セルの /workspace/.ghq/… はホストのここ
bin/bootstrap status                                          # 最後の行に FROM と元の commit
```

- **何が起きるか**: checkout の `image/build.sh` で base image `andon:local-<commit の先頭12文字>`（checkout が汚れていれば末尾に `-dirty`）を焼く（kanban・andon の go build も docker の中。ホストに go は要らない）→ セルの `Dockerfile` の `FROM` をその tag に書き換える → セルの image を焼き直す（手元にしか無い tag なので `--pull` は付けない）→ compose で作り直す。commit はホストの git で読む（ホストに git が無ければ、今の `FROM` の image の git で読む）。
- **通ったら見えるもの**: `[andon-home] <checkout>（commit: <40桁>）から andon:local-… を焼く` と `Dockerfile の FROM: <前> → andon:local-…`。`bin/bootstrap status` の最後の行が `FROM: andon:local-…（commit: <40桁>）`。
- **GHCR の版へ戻す**: `Dockerfile` の `FROM` を `ghcr.io/<owner>/andon:<版>`（上の行に出た「前」の値）に書き戻して `bin/bootstrap rebuild`（1 の pull が先）。セルの repo を git で持っているなら `git checkout Dockerfile` でもよい。
- **落ちたら見る所**: `image/build.sh が無い` — `--from` が andon の checkout を指していないか、`image/build.sh` を持つ前の版の checkout（`git pull` する）。焼きが落ちる — 出力の `docker build` のログ（checkout の変更が原因なら直して打ち直す）。`--from` の焼き直しはセルの image が変わるだけで、volume（ログイン・履歴）は残る。
- 手元で焼いた `andon:local-…` は増えていくので、要らなくなったら `docker image rm` で消す。ホストの `andon`（2）を合わせるなら、2 の取り出しの `ghcr.io/<owner>/andon:<版>` を `andon:local-…` に置き換えて打つ。

## 4. ホストの道具を置く（`host-install`）

```bash
bin/bootstrap host-install
```

`andon-up`（PC を起こしたあとに全セルを起こす）を `~/.local/bin` に、herdr の設定を `~/.config/herdr/`、ssh config を `~/.ssh/config.d/andon-home.conf` に、cloud-init の雛形を `.andon/` に、Herdr App の mod とレイアウト `andon` を App の設定フォルダに（下の「Herdr App の mod とレイアウト」）置く。**今あるファイルは上書きしない**。最後に、App のレイアウトを `andon` に切り替える1手を案内する。

- **通ったら見えるもの**: `+ <path>`（新しく置いた）か `= <path>`（同じ）。違うものは `! <path> はすでにあり、雛形と違う`（触らない。入れ替えるなら退避してから打ち直す）。`~/.ssh/config` に `Include config.d/*` が無ければ、足す1行を案内する。
- **落ちたら見る所**: `.env の SSH_PORT・PORTLESS_PORT は数字だけ`。`andon-up` が見つからない — `~/.local/bin` が PATH にあるか。 WSL で `HERDR_APP_HOME` が未指定なら、ほかのホストの道具も置く前に止まる。下の手順で App の設定フォルダを指定して打ち直す。

### 手元の端末から SSH で入る

`host-install` が置いた ssh config（`~/.ssh/config.d/andon-home.conf`）の `Host andon-home` で、手元の端末からセルの中へ入る。sshd は**公開鍵が置いてあるときだけ**起きる。鍵は人の手元にある既存のものを使う（作り方はここでは扱わない）。

```bash
cd ~/src/home-cell
git check-ignore .ssh/authorized_keys     # .ssh/ が .gitignore 済みか（パスが出ればよい）
mkdir -p .ssh && chmod 700 .ssh
cat ~/.ssh/<公開鍵のファイル>.pub >> .ssh/authorized_keys     # <公開鍵のファイル> は自分の .pub（例 id_ed25519.pub）
chmod 600 .ssh/authorized_keys
bin/bootstrap down && bin/bootstrap up    # 起こし直す（動いているときの up は何もしない）
bin/bootstrap logs                        # 確かめたら Ctrl-C
ssh andon-home
```

- セルの repo はコンテナの `/workspace` に載っているので、ホストで置いた `.ssh/authorized_keys` を boot が読み、セルの中の `~/.ssh/authorized_keys` に写す。ホスト鍵は volume に残るので、作り直しても変わらない。
- `git check-ignore` が何も出さなければ、`.ssh/` が `.gitignore` に無い。公開鍵を commit しないよう、先に `.gitignore` に `.ssh/` を足す。
- **通ったら見えるもの**: ログに `sshd を起動した（コンテナの 22 番。ホスト側の番号は .env の SSH_PORT）`。`ssh andon-home` でセルの中のシェルに入り、`hostname` が slug（`home`）。ssh config の `LocalForward` で、手元のブラウザから portless の名前（`http://<app>.home.localhost:<PORTLESS_PORT>`）も引ける。
- **落ちたら見る所**:
  - ログに `ssh は無効（使うなら .ssh/authorized_keys に公開鍵を置いて再起動）` — `.ssh/authorized_keys` が無い。置く場所はセルの repo の直下（`~/src/home-cell/.ssh/`）。置いたあと、`down` → `up` で起こし直す。
  - ログに `ssh は無効（.ssh/authorized_keys があるが読めない: …）` — 鍵はあるが、セルの `andon`（uid 1000）から読めない。`ls -ln .ssh .ssh/authorized_keys` で持ち主が 1000 か見る（`sudo` で作ると root の持ち物になる）。違えば `sudo chown -R 1000:1000 .ssh` で直し、`down` → `up`。
  - `Connection refused`・`Connection timed out` — `.env` の `SSH_PORT` と、ssh config の `Port` が同じか（`.env` を変えたあと `host-install` を打ち直すなら、古い `~/.ssh/config.d/andon-home.conf` を退避してから。`host-install` は今あるファイルを上書きしない）。ほかのセルや別のものと `SSH_PORT` が重なると、compose の起動がポート使用中で落ちる（3 の落ちたら見る所）。
  - `Permission denied (publickey)` — `authorized_keys` に入っている公開鍵が、手元でつかわれる鍵と対か。`ssh -v andon-home` で提示された鍵を見る。置いたあとで起こし直していないときも同じ（boot が起動時に写す）。
  - `ssh: Could not resolve hostname andon-home` — `~/.ssh/config` の先頭に `Include config.d/*` の1行が無い。`host-install` の出力が案内する1行を足す。
  - `REMOTE HOST IDENTIFICATION HAS CHANGED`（同じ `SSH_PORT` で別のセルに入ったとき。ProxyJump 先の別マシンが同じ番号で待っていて、その記録と `[127.0.0.1]:<SSH_PORT>` がぶつかるときも出る）— **消す前に、記録の指紋とセルのホスト鍵の指紋を照らす**。
    ```bash
    ssh-keygen -F "[127.0.0.1]:<SSH_PORT>" -f ~/.ssh/known_hosts -l     # 手元に記録されている指紋
    bin/bootstrap sh        # セルの中のシェル（セルの repo で）。入ったら次の1行、終わったら exit
    ssh-keygen -lf ~/.ssh/ssh_host_ed25519_key.pub     # セルのホスト鍵の指紋
    ```
    セルのホスト鍵は boot が volume に置く（セルの中の `~/.ssh/ssh_host_ed25519_key.pub`。作り直しても変わらない）。**指紋（`SHA256:…`）が同じ**なら、記録は正しいので消さない（別の原因を探す）。**違う**なら、記録は別のマシンのもの。その行だけ `ssh-keygen -R "[127.0.0.1]:<SSH_PORT>"` で消して入り直す。ProxyJump 先の記録を残したいなら、`SSH_PORT` を変える（3）ほうが安全。

### WSL＋Windows の App から入る

WSL にセルがあり、Herdr App が Windows 側で動くとき、`host-install` が置くのは **WSL 側**の `~/.ssh/config.d/andon-home.conf` だけ（Windows 側の ssh config と鍵には触らない。`host-install` はこのために変えない）。App の Add Machine が使う Windows 側の ssh config は、人が書く。セルの sshd は WSL の `127.0.0.1:<SSH_PORT>` で待っていて、WSL の localhost 転送（既定で有効）で Windows の `127.0.0.1:<SSH_PORT>` からも届く。

Windows の `C:\Users\<user>\.ssh\config` に足す（`<…>` は自分の値。番号は `.env` の `SSH_PORT`）:

```
Host andon-home
  HostName 127.0.0.1
  Port <SSH_PORT>
  User andon
  IdentityFile C:\Users\<user>\.ssh\<秘密鍵のファイル>
```

`IdentityFile` の鍵の公開鍵（`<秘密鍵のファイル>.pub`）が、セルの repo の `.ssh/authorized_keys` に入っていること（上の「手元の端末から SSH で入る」。WSL の鍵とは別の鍵なら、その公開鍵も足して `bin/bootstrap down && bin/bootstrap up`）。そのうえで App の Add Machine に `andon-home` を指定する。

```powershell
ssh andon-home hostname      # Windows の PowerShell から。App が使うのと同じ設定で届くか
```

- **通ったら見えるもの**: `hostname` が slug（`home`）を返す。App の Add Machine で `andon-home` が繋がり、セルの Space が出る。
- **落ちたら見る所**: `Connection refused` — WSL 側で `ssh andon-home` が通るかを先に見る（通らなければ上の「手元の端末から SSH で入る」）。WSL で通り Windows で通らないなら、localhost 転送が切れている（`%USERPROFILE%\.wslconfig` の `[wsl2]` の `localhostForwarding=false`）か、Windows 側のほかのものが同じ番号を取っている（`netstat -ano | findstr :<SSH_PORT>`）。`Permission denied (publickey)` — Windows 側の `IdentityFile` の公開鍵が `authorized_keys` に無い。`ssh -v andon-home` で提示された鍵を見る。`Bad owner or permissions` — Windows の秘密鍵の ACL。自分だけが読める形にする。

### Herdr App からセルへつなぐ（Add Machine）

App の Add Machine でセル（`Host andon-home`）を足すと、App は SSH 越しに herdr-core をセルの `~/.local/bin/herdr-core` に置き（Copy HC）、`herdr-core serve` を起こす。以後の Update HC もこのファイルを差し替える。**セルの herdr-core の版は App が決める**。

- SSH のセッションには image の環境（`PATH`・`AQUA_*`・`GHQ_ROOT`・`LANG`・`CLAUDE_CONFIG_DIR` など）が渡る（image の `/etc/ssh/sshd_config.d/andon.conf` の `SetEnv`）。App の serve から起きる端末にも `claude`・`hc`・`gh` が見える。
- boot は `~/.local/bin/herdr-core` が無いか symlink のときだけ image の種（aqua の herdr-core）へ繋ぐ。**実ファイル（App が置いたもの）は触らない**。`~/.local/bin/hc` は herdr-core への相対 symlink。`~/.local/bin` は PATH で aqua より先なので、boot が dispatcher を起こす `hc` も、それが起こす core も、App の serve と同じ herdr-core になる。
- `~/.local/bin` はコンテナ層。image を上げて作り直すと、App が置いた herdr-core は消えて image の種に戻る。App で Update HC を打ち直し（App が古い版を知らせればそれに従う）、セルの中で `hc server restart`（core を新しい版で起こし直す。端末は生きたまま）。`hc --version` と `hc server status` の `version` が App の版と同じなら揃っている。
- **App の serve を起こし直す必要があるとき**（App にセルが出ない・serve が古い版のまま・SSH の環境を変えたあと）: App でその machine を切って繋ぎ直す（serve は繋ぐときに起きる）。それでも出なければ `bin/bootstrap down` → `up` でセルごと起こし直し、上の Update HC から。
- **落ちたら見る所**: 端末が `env: 'claude': No such file or directory` で落ちる・SSH のシェルで `hc`・`gh` が打てない — image が古い（SSH に環境が渡らない版）。`ssh andon-home 'echo $PATH'` の先頭が `/opt/andon-home/.local/bin` か。版が割れている（`hc --version` と `hc server status` の `version` が違う）— `readlink ~/.local/bin/herdr-core ~/.local/bin/hc` を見てから `hc server restart`。

### Herdr App の mod とレイアウト

同じ `host-install` が App の設定フォルダに、Andon の mod 5つ（`mods/` の下）とレイアウト `andon`（`layouts/andon.json`・`andon.svg`）を置く。置き場は App と同じく `HERDR_APP_HOME`、未指定なら `XDG_CONFIG_HOME/herdr-app`、それも未指定なら `~/.config/herdr-app` の順で選ぶ。App が起動時に使う環境変数と揃えて実行する。既存の mod のディレクトリは、不完全でも中身を補わずそのまま残し、既存のレイアウトのファイルも上書きしない。更新するときは退避してから打ち直す（herdr-flightdeck の `inbox` などを前に入れていれば、それも退避してから）。

| mod | 何をする | 中身 |
|---|---|---|
| `andon` | Space・tab に名前を付ける（下の段） | `package/host/andon/` |
| `inbox` | 人が手を動かすもの（Needs you）と、プロジェクトの役（Chief of Staff・Chief Engineer）への入口（Projects）の2つの節。担当は止まったときだけ出る。読むだけでラベルを書かない（ADR-0015） | `package/host/inbox/README.md` |
| `kanban-board` | 台帳 `projects/<slug>/kanban/` を読むだけのボード（`ctrl-shift-y`） | `package/host/kanban-board/README.md` |
| `context-header` | pane の見出し（名前・状態・場所・blocked の問い） | `package/host/context-header/README.md` |
| `files-plus` | App の Files に画像・Mermaid・Markdown・HTML の表示を足したもの | `package/host/files-plus/README.md` |

由来は前身の herdr-flightdeck（private）の `herdr-app/`（commit `afc57ac`。`work-board` を `kanban-board` に改名）。ライセンスは各 README の Origin・Licence の節（files-plus は Herdr の `files` mod の改変で Apache-2.0、同梱の Pele は MIT、inbox の Lucide のアイコンは ISC、kanban-board の js-yaml は MIT）。

**WSL で実行し、App は Windows 側で動く場合**は、Windows の App の設定フォルダを WSL から見たパスで `HERDR_APP_HOME` に指定する。WSL では未指定だと書き込み前に止まり、案内を出す。Windows の既定値は `%USERPROFILE%\.config\herdr-app`。App 側で `HERDR_APP_HOME` や `XDG_CONFIG_HOME` を指定しているなら、その設定に合う実際のフォルダを選ぶ。

```bash
# <user> は Windows の利用者名に置き換える。App の置き場を変えていればそのパスを渡す。
HERDR_APP_HOME="$(wslpath 'C:\Users\<user>\.config\herdr-app')" bin/bootstrap host-install
```

WSL 内で Linux 版 App を動かしている場合も、その App の設定フォルダを `HERDR_APP_HOME` で明示する。Windows のプロファイルや App の環境をインストーラーが推測することはない。

**人が App でやる1手**: レイアウトを `andon` に切り替える。App のあるマシンで次を打つ（Windows の App は Windows 側で。command palette でレイアウトに `andon` を選んでもよい）。`host-install` は App のレイアウトを切り替えない。

```bash
herdr-app do layout.switch --name andon
```

レイアウト `andon` は App の標準の `main` の agents の欄を `inbox` にし、pane の見出しを `context-header`、Files を `files-plus` にしたもの。名前付けの block（`{ "id": "andon-names", "type": "block", "size": { "px": 1 }, "content": { "mod": "andon" } }`）を左の列に最初から持つので、レイアウトを複製して block を足す手は要らない。自分のレイアウトを使い続けるなら、その常に表示する row または column の `children` にこの block を1つ加える（他の block は残す）。

キーは人のもの（`keys.json`・`settings.json` は `host-install` が触らない）。使うなら各 mod の README の Keys の断片（`inbox` の `ctrl-shift-j`・`ctrl-shift-h`、`kanban-board` の `ctrl-shift-y`。同じ断片が各 mod の `keys.json`）を自分の `keys.json` に足す。前の inbox の `ctrl-shift-d`（`inbox.settle`）・`ctrl-shift-u`（`mod.open:inbox`）は無くなったので、足してあれば自分の `keys.json` から消す（残しても unknown command になるだけ）。

この block は空で、名前付けだけを行う。別のレイアウトへ切り替えるなら、そこにも block が必要。人が App のあるマシンで `herdr-app check <App の設定フォルダ>/mods/andon`（ほかの mod も同じ形で）を実行し（Windows の App は Windows 側で実行する）、レイアウト `andon` で `sample-site / CoS`・`sample-site / CE A-0001 設計`・`A-0002 修正 / A-0002` が見えることを確認する。App のエラーは `herdr-app status` で見る。

mod は端末のラベルと実際の pane の所属から既存の Space・tab を名前付けする。同じ名前なら何もせず、作成・選択・フォーカス・削除は行わない。レビュアー自身の題で担当の tab を上書きしない。dispatcher（`role=dispatcher`）は tab を「dispatcher」に、台帳が2つ以上のときのセルの Space を `<セルの slug> plant` にする（ADR-0002 決めたこと 21）。題の空白・制御文字は1つの空白に揃え、前後の空白を除く。空白だけの題では命名しない。役ごとのアイコンは含まない（それは inbox が描く: CoS は `clipboard-list`、CE は `drafting-compass`。App 自身のサイドバーには効かない）。

API の根拠は [Herdr App の mod guide](https://github.com/herdrdev/herdr-app/blob/903dc810da0c839ba865d6b190f3aeeceb3a636b/defaults/AGENTS.md) と [snapshot の実装](https://github.com/herdrdev/herdr-app/blob/903dc810da0c839ba865d6b190f3aeeceb3a636b/src/window.rs)。端末の `labels`、pane の `terminal`・`tab`、Space の `id`・`machine`・`tabs` を読む。リモートの App ID にだけ machine の接頭辞が付く。

## 5. セルに入って、立ち上がりを確かめる

```bash
bin/bootstrap               # herdr に入る。bash で入るなら: bin/bootstrap sh
```

中で（シェルのペインで）:

```bash
hostname                    # slug（home）
andon help                  # sub-command の一覧
hc ls                       # 端末の一覧。dispatcher が出る
ls ~/.config/herdr-core/templates     # claude・sonnet・fable・codex・codex-review・claude-review・chief-of-staff・chief-engineer
ls -l ~/.agents/skills                # andon-<name> が /opt/andon/skills/<name> への symlink、playwright-cli が /opt/andon-tools の下への symlink
ls -l ~/.claude/skills                # playwright-cli（同梱の skill。外の skill は local/gh-skills.txt に書いたものだけ）
cat ~/.claude/CLAUDE.md               # @ の2行（environment.md と AGENTS.local.md）
cat /etc/claude-code/managed-settings.json
head -3 ~/.codex/config.toml          # approval_policy・sandbox_mode（無かったときに boot が足したもの）
codex execpolicy check -r ~/.codex/rules/andon.rules gh pr merge 1   # "decision":"forbidden"
```

- **通ったら見えるもの**: 上の全部。`hc get dispatcher` が通る（dispatcher は `andon dispatcher` を常駐させた端末）。App に tab「dispatcher」が出る（台帳が1つならその指揮側の Space、2つ以上なら `<セルの slug> plant`）。ラベルは `hc get dispatcher --json` の `herdr.space`・`herdr.tab`。
- **落ちたら見る所**: どれも出なければ boot が走っていない（`bin/bootstrap logs`）。boot は失敗しても続ける作りなので、警告の行を探す。生成だけ失敗したときは `andon boot` を中で打ち直せば同じ結果になる（何度打っても同じ）。`hc` が無い・端末が起きない — herdr-core は初めて `hc` を叩くと起きる。

## 6. 中でログインする（セルの中で取る）

ブラウザはセルに無い。URL が出たら**手元のブラウザで開く**（セルから開こうとしない）。トークンをホストから持ち込まない。

```bash
gh auth login               # GitHub.com → HTTPS → device code（URL とコードを手元で）
gh auth setup-git           # git の資格情報を gh にする（これが無いと git の clone・fetch が認証で落ちる）
claude                      # 初回に /login。URL を手元で開き、コードを貼る
codex login --device-auth   # Codex を使うとき
```

ログインは volume（`claude`・`codex`・`gh` など）に残る。作り直しても消えない。git の資格情報（credential helper を gh にする設定）は `~/.gitconfig` にあり volume の外。初回は上の `gh auth setup-git` を打つ。そのあと `gh` にログインしていれば boot が毎起動同じ手を打つので、作り直しても戻る。

- **通ったら見えるもの**: `gh auth status` が Logged in。`claude` が対話に入る。`codex login status`。
- **落ちたら見る所**: `andon issue start` が `origin の既定の branch が分からない（…）: …could not read Username…。認証が原因らしい` で止まる — `gh auth status` でログインを確かめ、`gh auth setup-git` を打つ（git の stderr が文言に添えて出る）。`gh` のスコープ（`repo`・`workflow`・`read:packages`）。device code の期限切れは打ち直す。`claude` が毎回ログインを求める — `claude` の volume が付いているか（`bin/bootstrap sh` で `ls ~/.claude`）。

## 7. Andon の repo を迎える

```bash
ghq get github.com/<owner>/andon        # GHQ_ROOT=/workspace/.ghq の下に入る
ghq list
```

- **通ったら見えるもの**: `/workspace/.ghq/github.com/<owner>/andon`。迎えた repo の根は、台帳の `config.yaml` の `repos` に載っていれば、`andon issue start`・`andon chief-engineer` が起こす前に Claude Code と Codex の信頼に足す（boot と同じ手。担当とレビュアーの worktree は元の repo の根で判定されるので、信頼の画面が出ない）。
- **落ちたら見る所**: `Repository not found` — `gh auth status` と repo の権限。`could not read Username` — `gh auth status` でログインを確かめ、`andon boot` を打ち直す（boot が `gh auth setup-git` を打つ）。

## 8. プロジェクト `andon` の台帳を作る

台帳はプロジェクトごとに `projects/<slug>/kanban/`（git の外）。slug はプロジェクト名＝フォルダ名。`projects/<slug>/` は実ディレクトリ（symlink にしない。理由は上の「セルの置き方」）。

```bash
cd /workspace
andon create andon --repo github.com/<owner>/andon --prefix A --title Andon   # 足りない引数は、端末なら既定値を示して聞く
cd projects/andon && kanban list ; kanban lint
# 人のファイル: project.json（このプロジェクトが書いてよい repo。形は contract.md §11）と AGENTS.md。中身を見て owner の email で commit する
git add project.json AGENTS.md && git commit -m 'projects/andon'
andon delegate show          # 型と効いている値（commit の前は「無委譲」）
```

- **`andon create` がすること**: `projects/<slug>/` に台帳（`kanban init`。prefix と repos を渡す）・`project.json` の下書き（`version`・`project`〈題・prefix〉・`repos`。`delegation` は無し＝無委譲。Andon が読むのと同じ検査を通す）・`AGENTS.md` の雛形（薄い地図の見出しだけ。[ADR-0004](adr/0004-project-layout.md) 決めたこと 1）を作り、`repos` のうち `GHQ_ROOT` の下に clone の無いものを `ghq get` で迎える。**commit はしない**（人のファイル）。最後に commit の仕方と `andon open <slug>` を出す。
- **聞くもの**: 引数に無い slug・repos（1 行に 1 つ、空行で終わり）・prefix（既定は slug の `-` で区切った頭文字。`sample-site` → `SS`）・題（既定は slug）。聞いたときは最後に確かめる。標準入力が端末でなければ聞かない: slug が無ければ止まり、ほかは既定（repos 無し）。
- **通ったら見えるもの**: `作った: projects/andon/（kanban/・project.json・AGENTS.md）`。`config.yaml` に `prefix: A` と `repos`。`kanban lint` が問題なし。
- **落ちたら見る所**（どれも何も作らずに止まる）: `が既にある` — 作ってある（そのまま使う。手で作るときの `kanban init` は `already a ledger`）。`slug は英小文字・数字・-`・`repo は <host>/<owner>/<repo>`・`prefix は英大文字で始まる` — 綴り。`この端末（role=…）では打てない` — 役の端末。人の端末で打つ。`ghq get … が落ちた` — `gh auth status`。`… があるが git の clone でない` — `GHQ_ROOT` の下の空のディレクトリなどを確かめて退ける。`題に改行や制御文字がある` — 題は 1 行。`kanban init が落ちた` — 理由の文（`checkClock` ならコンテナの時計）。
- `project.json` が無い・owner の commit でない・`repos` に無い repo の課題は、`andon issue start` が起こさない（理由を1行出す）。移す前の `delegation.json` しか無いセルは、移すまで無委譲・repo 無しとして読まれる（読まない。安全側。移すのは台帳の改名の手順）。
- 外の信号の出所（`project.json` の `signals`。[contract.md](contract.md) §11）は、**セルの image を `signals` を知る版に上げてから**書く。古い版の `andon` は知らないキーとして `project.json` ごと弾き、無委譲・repo 無しに倒れて担当を一切起こせない。
- 信号を読むのは `andon signal collect`（`projects/<slug>/` で打つか `--project <slug>`。[contract.md](contract.md) §15）。GitHub（`github-ci`・`github-issues`）は `gh` の資格情報をそのまま使う。Sentry・Linear・Cloudflare のトークンは `andon signal auth <source>` がセルの中で聞いて `~/.config/andon/signals/<source>.token`（mode 600。compose の volume `signals`）に置く。**トークンは読み取り専用の scope で作る**（Linear はコメントが書けるまで。[ADR-0011](adr/0011-signals-and-value-loop.md) 決めたこと 10・15）。置き場は守りではない（セルの中の役は同じユーザーで読める）。守りは scope。最初の版で読むのは GitHub だけで、ほかの出所は「読む command がまだ無い」と1行出して飛ばす。
- **通ったら見えるもの**: 出所と repo ごとに `github-ci github.com/<owner>/<repo>: 見えている 1 件（新しい 1・また出た 0・消えた 0）`。**落ちたら見る所**: `読めないので記録を変えずに飛ばす` — `gh auth status`（その出所の記録は変えず、終了コード 1。次に打てば読み直す）。`信号の記録が読めない` — `~/.local/state/andon/dispatcher/<slug>.signals.json` が壊れている（退ければ次の collect が作り直す。`delivered_episode` も消えるので、`now` の信号は届け直される）。
- `projects/andon/AGENTS.md` の「Chief of Staff」の節（目標・並行の上限・独立レビューを入れる条件）は任意。無ければ並行の上限は 2。
- 課題の id は `A-0001` から 4 桁で振られる。打つときは `A-1` でもよい（同じ課題）。

### 前の置き場から移す（`.flightdeck/` → `kanban/`）

台帳の置き場は 2026-11-06 に `projects/<slug>/.flightdeck/` から `projects/<slug>/kanban/` に改めた。同時に `config.yaml` の `coordinator` を `chief_of_staff` に、課題の id を 4 桁（`A-5` → `A-0005`）に、台帳の中の `coordinator.md` を `projects/<slug>/AGENTS.md` の「Chief of Staff」の節に改めた（[ledger.md](ledger.md) の「Moving from `.flightdeck/`」・[ADR-0004](adr/0004-project-layout.md) 決めたこと 5）。前の名前を読む互換は無いので、前の版のセルを上げるときは台帳ごとに `kanban migrate` を1回打つ。台帳は今までどおり git の外（`kanban/.gitignore` の `*`。ADR-0005 決めたこと 5）。

1. **上げる前に**: `doing`・`review` の課題を片付ける（マージして `andon issue finish`、または `todo` に戻す）。worktree・branch・端末のラベル・Space の名前に id が入っているので、`kanban migrate` はそういう課題があると何も変えずに止まる。確かめ方: `grep -lE '^status: (doing|review)' projects/<slug>/.flightdeck/tasks/*/task.yaml`（何も出なければよい）。
   委譲を控える: `andon delegate show > ~/delegation-before.txt`（前の版の `andon` は `delegation.json` を読む。6 で比べる）。
2. **image を上げて起動する**（`rebuild`）。新しい版の `kanban`・dispatcher は `.flightdeck/` を読まないので、移すまで dispatcher はその台帳を飛ばし、`kanban` は `no ledger` で止まる。
3. **Chief of Staff と dispatcher を止める**: Chief of Staff（`hc ls --label role=chief-of-staff --label project=<slug>`）を `hc kill <address>`、次に `hc kill dispatcher`。Chief of Staff を先に止めるのは、そのターンのフックが dispatcher を起こし直すため。
4. **移す**:

   ```bash
   cd /workspace/projects/<slug>
   kanban migrate              # 移した id の数、AGENTS.md に移したか、dispatcher の控えを直したか、project.json の下書きを書いたかを出す
   kanban lint ; kanban list --all
   git status --short          # kanban/ は出ない（git の外）。project.json（と、あれば AGENTS.md）は人のファイルとして 5 で commit する
   ```

   - `kanban migrate` がすること: `tasks/<id>/` と `task.yaml` の `id`・`parent`・`after` を 4 桁に揃える、`coordinator` を `chief_of_staff` に（値・コメント・位置はそのまま）、`.flightdeck/` を `kanban/` に、`coordinator.md` を `AGENTS.md` の末尾の `## Chief of Staff` に足して消す、dispatcher の控え（`~/.local/state/andon/dispatcher/<slug>.json`。`XDG_STATE_HOME` があればその下）の中の id を新しい id に、`project.json` が無ければ下書きを書く（`version: 1`・`project` は `config.yaml` の `title`〈無ければフォルダ名〉と `prefix`・`repos` は `config.yaml` の `repos`・`delegation` は `delegation.json` の中身そのまま。commit はしない。`delegation.json` は消さない）、`log.jsonl` に `migrate` の1行。
   - 書き換えないもの: `log.jsonl` の前の行・`brief.md`・`report.md`・問い（読むときに `A-5` も `A-0005` と同じ id とみなす）、branch と worktree の名前、ルーチン（id を持たない）。控えには台帳の置き場の path が入っていない（プロジェクト名で引く）ので、path の書き換えは要らない。stop-check の控え（`~/.local/state/andon/stop-check/<slug>-<id>.json`）は `doing` の課題のものなので、移したあとは読まれない。消してよい。
   - 止まったとき（どれも何も変えずに止まる）: `still in doing or review` — 1 に戻る。`kanban/ is already there` — 手で作った `kanban/` があるので、中身を見てから退ける。`is not the dispatcher's state` — 控えを直すか消す（消すと初めて見る台帳として扱われる）。`delegation.json is not JSON`・`the project.json drafted … can't be read by Andon` — 理由の文を読んで `delegation.json` か `config.yaml` の `prefix`（英大文字で始まる英大文字・数字）を直す。直せなければ `project.json` を先に手で書いて commit してから打つ（あれば下書きは作らない）。`is not format 2` — `config.yaml` の `version` を引用符なしの `2` にする。`is a symbolic link` — リンクを実体に置き換える。
5. **project.json を見て commit する**: 下書きの `project.title`・`prefix`（英大文字で始まる英大文字・数字。違えば直す）・`repos`（書いてよい repo。足りなければ足す）を確かめる。`coordinator.md` にあった並行の上限などは、`AGENTS.md` の節のままでも、`project.json` の `chief_of_staff`（`parallel`・`review_when`・`triage_max_issues`）に写してもよい。下書きを作らなかった（もうあった）ときは、§8 の `project.json` の例と `delegation.json` の中身から手で書く。`git add project.json` し、`AGENTS.md` があれば（`coordinator.md` を移したか、前からあれば）それも `git add` して commit する（`git add project.json; [ -f AGENTS.md ] && git add AGENTS.md; git commit -m …`。どちらも人のファイル。author が `andon.toml` の `owner` であること）。`kanban/` は git の外なので commit しない。
   確かめる: `andon delegate show` が 1 で控えたものと同じ型と効いている値（scope ごとの行）を出す（`diff ~/delegation-before.txt <(andon delegate show)` で見比べる。前の版と言い回しが違う行は読んで比べる。`無委譲` と出たら `project.json` が commit されていないか、owner の commit でないか、形が違う。理由の文を読んで直し、commit し直す）。同じなら `git rm delegation.json delegation.md`（あるものだけ）して commit。
6. **起こし直す**: `andon chief-of-staff`（`config.yaml` の `chief_of_staff` が新しい address に直る）。dispatcher は Chief of Staff の最初のターンのフックか `andon boot` で起きる。控えの id は 4 に移してあるので、初めて見る台帳としては扱われない。
7. **自分のスクリプト**: `FLIGHTDECK_ROOT` を使っていれば `KANBAN_ROOT` に改める。

### 台帳を形式 3 に手で移す（`tasks/` → `issues/`。課題への改名）

台帳のカードの名前を 2026-11-07 に task から「課題（issue）」に改めた（[ADR-0008](adr/0008-issue-naming.md)。台帳の形式は 3）。形式 2 の台帳は、`kanban`・`andon`・dispatcher・ゲートのどれも読まず、`config.yaml is format 2; move it by hand (docs/operate.md)` の 1 行で止まる。移す command は無いので、人が台帳ごとに次の順で手で移す（変わるのは名前だけ。[ledger.md](ledger.md) の「Changes from version 2」と同じ）。

1. **`doing`・`review` の課題を片付ける**（マージして `andon task finish`、または `todo` に戻す）。ラベル `task=` を持つ担当・レビュアー・Chief Engineer の端末が居なくなる。確かめ方: `grep -lE '^status: (doing|review)' projects/<slug>/kanban/tasks/*/task.yaml` が何も出ない。
2. **パッケージの版を上げる**（`rebuild`。`andon`・`kanban` が形式 3 を読む）。
3. **フォルダと名前を改める**: `mv kanban/tasks kanban/issues`。各 `issues/<id>/task.yaml` を `issue.yaml` に（`for f in kanban/issues/*/task.yaml; do mv "$f" "${f%task.yaml}issue.yaml"; done`）。
4. **`kanban/config.yaml` の `version: 2` を `version: 3` に**（コメントとほかのキーはそのまま）。
5. **Andon が `library/` に書いた値を改める**: `issues/*/library/report.json` の `"to": "task"` を `"to": "issue"` に、`issues/*/library/questions/*.json` の `"from": "task"` を `"from": "issue"` に（`sed -i`。無ければ飛ばす。`done` の課題のものも。読む側に古い値の互換が無いため）。
6. **dispatcher の控えを消す**: `~/.local/state/andon/dispatcher/<slug>.json`（`XDG_STATE_HOME` があればその下）。控えが守るのはレビュアーの二重起こしとゲートの重複で、動いている課題が無ければ要らない。次の見回りで作り直される。
7. **`projects/<slug>/project.json` の `triage_max_tasks` を `triage_max_issues` に直して commit する**（無ければ何もしない。人のファイルなので author は `andon.toml` の `owner`）。
8. **人の App の mod を置き直す**: `host-install` は今あるファイルを上書きしないので、App の設定フォルダ（「Herdr App の mod とレイアウト」）の `mods/` から Andon の mod 5つ（`andon`・`inbox`・`kanban-board`・`context-header`・`files-plus`）を消してから `bin/bootstrap host-install` を打つ。置き直したあと、仮設の端末（ラベル `task=`）には名前が付かなくなる（新しい mod はラベル `issue=` だけを読む。仮設が止まるまでの間）。
9. **`andon boot`**。`kanban lint` が通れば済み。

`log.jsonl` の行・`brief.md`・`report.md` は触らない（過去の行は当時の言葉のまま）。動いている課題がある台帳で 3 以降をすると、端末のラベル `task=` と台帳が食い違い、dispatcher が担当を見失う。だから 1 を先に、`kanban lint` を最後に。

### 別のマシンへ引っ越す（セルを動かす）

セルを別のマシン（別の PC・別のホスト）へ動かすときの手順。台帳 `kanban/` は git で追跡しない（[ADR-0005](adr/0005-cell-repo.md) 決めたこと 5）ので、セルの repo を push・clone しても台帳は付いてこない。**台帳を写すのは人が手でする**。

- **写さなかったとき失うもの**: 台帳の全部（課題の一覧と状態・`log.jsonl`・依頼書・報告・問い・ルーチン）。セルの repo（人のファイル・`wiki/`・`files/`・`projects/<slug>/AGENTS.md`）と、決定・進行中の仕事（製品の repo の `docs/`・PR と branch）は GitHub に残るので失わない。台帳が無いと新しい側の `projects/<slug>/` は空のプロジェクトになり、`kanban init` からやり直す（課題の id は `0001` から振り直し。前の PR や branch の名前に残る id とは別物になる）。台帳に残るのは経過で、セルの寿命のもの。要らなければ写さなくてよい。
- **写す前に `doing`・`review` を片付けておく理由**: `issue.yaml` の `agent`・`worktree`・`machine` と、端末のラベル・Space・worktree・branch は、**古いマシンの**ものだから。写した先で `doing` のまま残ると、動いている担当がいないのに動いているように見え、直しは起こし直すところから始まる。片付けてから写せば（マージして `andon issue finish`、または `todo` に戻す）、新しい側で起こし直す担当が減る。

手順（古い側 → 新しい側）:

1. **古い側で止める**: 動かす直前に、`doing`・`review` の課題を片付ける（上）。Chief of Staff（`hc ls --label role=chief-of-staff --label project=<slug>`）を `hc kill <address>`、次に `hc kill dispatcher`。Chief of Staff を先に止めるのは、そのターンのフックが dispatcher を起こし直すため。
2. **セルの repo を push する**（人のファイルだけが入る）: `git status --short` で `andon.toml`・`projects/*/project.json`・`AGENTS.md`・`gate.local.json`・`templates.local/`・`local/`・`wiki/`・`files/` に未 commit が無いか見て、commit（author が `andon.toml` の `owner`）して `git push`。`.env`・`.ssh/`・`.ghq/` は追跡されない（新しい側で作り直す）。
3. **`kanban/` を写す**（手で。手段は例）:

   ```bash
   # 古い側。プロジェクトごとに 1 つ（.lock と state/ は写さなくてよい）
   cd /workspace/projects
   tar -czf ~/kanban-<slug>.tgz --exclude='.lock' --exclude='state' <slug>/kanban
   scp ~/kanban-<slug>.tgz <新しい側のホスト>:~/
   ```

4. **新しい側でセルの repo を迎える**: `git clone` した repo を `~/src/<slug>-cell` に置く（または [§2](#2-セルの-repo-を作るandon-init) の `andon init` で作った repo に、clone した中身を重ねる）。続けて §3 の起動、§5〜§6 のログイン（`gh`・`claude`・Codex は中で取り直す。持ち越さない）。`.env` と `.ssh/authorized_keys` は新しい側で作る。
5. **台帳を置く**: `tar -xzf ~/kanban-<slug>.tgz -C /workspace/projects`（セルの中で）。`cd /workspace/projects/<slug> && kanban lint ; kanban list --all`。置く前に `andon init` したセルで `kanban/` が空に作られていたら、退けてから置く（`kanban init` は `already a ledger` で止まる）。
6. **worktree を戻す → resume**: `ghq get github.com/<owner>/<repo>` で製品の repo を迎える（branch が GitHub に push 済みであること。push 前の変更は古い側にしか無い）。台帳の `worktree` は古いマシンのパスのままなので、新しい側の置き場（`/workspace/.worktrees/<repo 名>/<slug>-<id>`）に直してから、`doing` だった課題ごとに `andon issue resume <id> --dry-run` で計画を見て、`andon issue resume <id>`（worktree を戻し、同じ branch で起こす）。セッションの記録は新しい側に無いので、resume は記録が無いと止まる。そのときは人の go で `andon issue resume --fresh <id>`（[ADR-0012](adr/0012-resume-and-worktree-home.md) 決めたこと 7）。
7. **`doing` の担当を起こし直す**: `andon chief-of-staff`（`config.yaml` の `chief_of_staff` が新しい address に直る）。写した台帳の `agent`・`machine`・`chief_of_staff` は古いマシンのままで、新しい側では無効。dispatcher は端末の無い `doing` の担当を自動で resume する（1つの死につき1回。[dispatcher.md](dispatcher.md) §2.3・§2.9）ので、6 を手で打っていなければそれを待ってよい。1 で片付けておけば、この段の課題は少ない。

## 9. Chief of Staff を起こす

```bash
cd /workspace/projects/andon
andon chief-of-staff        # 起こす（動いていればそれを使う）。画面には乗らない
andon open andon            # 同じこと。どこからでも打てる（slug を省くと fzf で選ぶ。fzf が無い・端末でなければ一覧を出して終了コード 2）
```

コマンドは `hc attach` を打たない（[ADR-0002](adr/0002-herdr-spaces.md) 決めたこと 14）。Chief of Staff はラベル `herdr.space=andon-andon`・`herdr.tab=andon-andon-cos` で Herdr App の指揮側の Space に出るので、人は App のその tab で話す。最後の行に出た場所が「<Space 名> / <tab 名>」の形で出る（`andon / CoS`）。名前を付けるのは App の mod（S5）で、入るまでは Space と tab が名無し（Space はフォルダ名・tab は番号）に見える。App の無い所で話すときは人が自分で `hc attach <address>`。

起きた Chief of Staff は、skill の手順で最初に `andon chief-of-staff` を打ち、自分を `config.yaml` の `chief_of_staff` に載せる。端末を起き直して address が変わっても、これで直る。

- **通ったら見えるもの**: `andon chief-of-staff: andon の Chief of Staff は <address>（andon-cos・起こした）` と `andon chief-of-staff: 出た場所 andon / CoS（<address>）`。App に Chief of Staff の Space と tab が出る（画面は移らない）。`hc ls` に `role=chief-of-staff project=andon` の端末（名前 `andon-cos`）。`config.yaml` の `chief_of_staff` がその address。もう一度打つと起こさずに `動いているものを使う`。Chief of Staff が自分の skill（system prompt に入った役の手順）に従って台帳を読む。
- **App から起こした端末が `env: 'claude': No such file or directory`（exit 127）で落ち、終了コード 3 になる** — App の serve に image の環境が渡っていない（4 の「Herdr App からセルへつなぐ」）。直した image に上げ、App で machine を繋ぎ直してから打ち直す。`config.yaml` の `chief_of_staff` が落ちた端末を指したままでも、打ち直せば新しく起こした address に書き換わる（落ちた端末は `hc ls` に出ないので数えない）。
- **最初のメッセージが届いたかは** Chief of Staff の画面（`hc read <address>`）で、台帳を読み始めたかで見る。`andon chief-of-staff` の終了コード 0 だけでは分からない（hook で止まっても 0 で返った）。入力欄にメッセージが残って動かないときは、`UserPromptSubmit operation blocked by hook` の行が出ていないかを見る。
- **落ちた Chief of Staff を起こし直す**: image を上げ直し（`bin/bootstrap` で作り直す）、落ちた端末を `hc kill <address>` してから `andon chief-of-staff`。`hc server restart` が `no systemd user manager here: the core ends with whatever started it` を出すのは、systemd の無いコンテナであるセルでは正常（core は起こした側と一緒に終わる）。
- **落ちたら見る所**: `プロファイル … の template が無い` — 5 の `ls ~/.config/herdr-core/templates`（boot の生成）。`Chief of Staff が 2 人いる` — `hc ls --label role=chief-of-staff --label project=andon` を見て1人に減らす（`hc kill`）。`最初のメッセージが届いていない` — App のその tab（か `hc read <address>`）で画面を見る。信頼の画面はセルでは出ないはず（上の「信頼の画面」）。出ていれば想定外なので人が答えてから、出た `hc send` を打つ（終了コード 3。Chief of Staff は dispatcher が見張らないので手で送る）。`出た場所 App の Space の外` — ラベルより前に起きた Chief of Staff を使った。`hc kill` してから打ち直すと Space に出る。起きるが skill が効かない — 10 の S4 の確かめ。dispatcher がこの台帳を飛ばす — `chief_of_staff` の端末が live でない（`hc get <address>`）。**初めて見る台帳は、dispatcher が通知の元を控えるだけで送らない**（今のレビュー依頼は拾う。[dispatcher.md](dispatcher.md) §2.1）。通知は1回の見回りのあとに効く。dispatcher の端末が素のシェルのまま — 次の Chief of Staff のターン（フック）か `andon boot` が端末を片付けて起こし直す（[dispatcher.md](dispatcher.md) §5）。

## 10. 小さい PR を1本流す

### 先に: マージの委譲と owner の email

依頼書の `Merge: auto` でマージされるには、ゲートが approve するだけでは足りず、**委譲の `merge` がそのレベルを許している**ことが要る（[contract.md](contract.md) §11）。委譲（`projects/andon/project.json` の `delegation`）と条件表（`gate.local.json`）は、**それを最後に変えた commit の author の email が `andon.toml` の `owner` と一致するときだけ**信じられる。違うと無委譲（全部人）・条件表を重ねない、として読まれる。

```bash
cd ~/src/home-cell
grep '^owner' andon.toml                    # owner の email（2 の --owner）
git config user.email                       # ホストのこの checkout で使われる email。owner と違うならホストの ~/.gitconfig で合わせる
andon delegate grant merge B --evidence '<根拠のパスか引用>'    # projects/andon/ で打つ（か --project andon を付ける）。B はマージを許すレベルの上限（A / B / C）
andon delegate show                                              # 同じく projects/andon/ で（か --project andon）
git -C ~/src/home-cell log -1 --format='%ae' -- projects/andon/project.json gate.local.json
```

- 人のファイルの commit はホストのこの checkout で打つ。セルの中で打つなら作者を明示する（`GIT_AUTHOR_NAME=<name> GIT_AUTHOR_EMAIL=<owner の email> andon delegate grant …`）。セルの中の既定の作者は、セルの repo では owner でない（[image.md](image.md) の「git の作者」）。`git config user.email`（repo の中だけの設定）で合わせない: セルの中の担当にも効き、委譲の照合が素通りになる。
- `grant` は `project.json` の `delegation` だけを書いて commit する（`project.json` は §8 で先に人が書く。無い・commit していない変更がある・最後の commit が owner でない、なら書かない）。Chief of Staff・担当・dispatcher の端末では打てない（人が打つ）。`andon` プロジェクトで許せる上限は `B`（contract.md §11）。
- **通ったら見えるもの**: `grant` が `projects/andon/project.json: …` の1行だけを出し、`注意:` の行が出ない。`andon delegate show` に `merge` の値が出る。最後の `git log` の email が `owner` と同じ。`gate.local.json` を commit し直したときも、同じ email で。
- **落ちたら見る所**: `注意: commit の author（… <…>）が andon.toml の owner（…）でない。読み手は無委譲として読む` — commit の作者が owner でなかった（セルの中で作者を明示せずに打った・ホストの email が違う）。作者を owner にして、`project.json` を直す commit を打ち直す（`git commit --amend --reset-author` で author を直すか、`grant` を打ち直す）。`注意: andon.toml に owner が無い` — `andon.toml` の最上段に `owner = "<email>"` を足して commit する。ゲートが `needs_human` で `委譲 merge: … の外` — `show` の値がレベルより低い。`gate.local.json を重ねない: … が owner でない` — 同じ email で `gate.local.json` を commit し直す。

人は依頼を1つ渡す。Chief of Staff が依頼書に切る。Andon 自身の小さい変更（レベル A か B。例: 文書の誤字・テストの追加）を選ぶ。依頼書の Settings は `Review: on`・`Merge: auto`（design.md §5.1 の 2）。ゲートの条件表の `repos` に `github.com/<owner>/andon` を入れておく（レベルの決めは人。セルの `gate.local.json`）。

流れ（人が触らないところ）: 依頼書 → 担当を起こす → PR → `review-request` → dispatcher がレビュアーを起こす → `ready`（レビュー）→ dispatcher がゲート → approve → dispatcher がマージ → `andon issue finish` → 次の見回りで dispatcher が同じ repo の動いている課題への影響（`andon issue impact`）を見て、ぶつかる・重なるものがあれば Chief of Staff に `impact` で届ける（rebase を頼むか・止めて向きを確かめるかは Chief of Staff。[dispatcher.md](dispatcher.md) §2.5）。

担当を起こすのは Chief of Staff の `andon issue start <id>`（`projects/andon/` で打つ）。worktree と branch（`/workspace/.worktrees/andon/andon-<id>`・`andon/<id>-<題の英字>`）・`hc run`（プロファイルは `issue.yaml` の `profile`）・`kanban start`（`doing`・`agent` がその端末）を1行でする。担当はラベル `herdr.space=andon-andon-<id>`・`herdr.tab=andon-andon-<id>` で App に課題の Space として出て（最後の行 `出た場所 <id> <題> / <id>`。ADR-0003）、dispatcher が起こすレビュアーは担当と同じ tab に横に並ぶ。`andon issue finish` で端末が片付くと tab が空になり、Space ごと消える。

- **通ったら見えるもの**: 下の「§5.1 の確かめ方」の対応表。
- **落ちたら見る所**: `andon issue start` が終了コード 3（`最初のメッセージが届いていない`。信頼の画面など）— 台帳はもう `doing`。画面が止まっていれば App のその tab（か `hc read <address>`）で答える。最初のメッセージは**dispatcher が次の見回りで送り直す**（端末のラベル `first_message.sent=false`。[dispatcher.md](dispatcher.md) §5）。急ぐときだけ、出た `hc send` を手で打つ（打てば dispatcher は送らない）。信頼の画面そのものは出ないはず（boot と `andon issue start` が repo の根を信頼済みにする。7）。担当が `deck start` を打たない — ADR-0001 の S4（下）。PR が出ない・checks が落ちる — `gh pr checks`。`review-request` を拾わない — `hc read dispatcher`・`~/.local/state/andon/dispatcher/<slug>.json`。ゲートが `needs_human` — 台帳の問い（`kanban questions`、`from: gate`）と `issues/<id>/library/gate/<head sha>.json` の `reasons`。マージされない — 条件表の `repos.<repo>.merge`。

## 11. dispatcher の確かめ

```bash
hc get dispatcher ; hc read dispatcher                       # 見回りの記録
jq . ~/.local/state/andon/dispatcher/home.json 2>/dev/null   # 控え（<slug> は台帳のプロジェクト名なら andon.json）
```

控えは `~/.local/state/andon/dispatcher/<プロジェクト名>.json`（`XDG_STATE_HOME` があればその下）。待ちの通知は `pending`、フックで渡したものは `hook` に残る。通知は台帳と端末の状態から拾う（[dispatcher.md](dispatcher.md) §2.2・§2.3）。**通ったら見えるもの**: 10 の流れの各段（`review`・`ready`・`gate`・`merged`）が、Chief of Staff に `<news>` として1通にまとまって届く。

### 前の名前の係から移す（courier → dispatcher）

常駐の係は 2026-11-06 に名前を dispatcher に改めた（[decisions.md](decisions.md)・[ADR-0002](adr/0002-herdr-spaces.md) 決めたこと 22）。前の名前を読む互換は無い。前の版のセルを上げるときは次のとおり。

1. **上げる前に**: `status: review` の課題と、レビュアーが動いている（`review_request` を出した）課題が無いときに上げる。あれば片付くのを待つ（理由は 3）。
2. **image を上げて起動する**（`rebuild`）。boot が前の係の端末（名前もラベルも `courier`・`role=courier`）を片付けて（keep を外して kill・rm）、`dispatcher` を起こす。片付けるのは前に出ているものが前の係（`andon courier`）か引数の無いシェル（作り直しで戻った中身）のときだけで、起動のログに `前の版の係の端末 <address>（courier）を片付けた` と `dispatcher を起こした` が出る。前に出ているものが取れないあいだ（作り直しの直後）は取れるまで最大 5 秒読み直し、それでも取れなければ、それ以外（人が中で何か動かしている）と同じく触らずに `… なので触らない` と書き、片付けに失敗したとき（`hc keep`・`kill`・`rm` が落ちた）も含めて、**その起動では dispatcher を起こさない**（前の係が動いたままだと二重に回る。ログに `dispatcher は起こさない` が出る）。人が `hc kill <address>` で片付けてから `andon boot` を打ち直す。ほかの端末には触らない。前の tab `andon-<x>-courier` は端末が無くなれば App から消える。
3. **控えの置き場が替わる**: `~/.local/state/andon/courier/` は読まない。新しい `~/.local/state/andon/dispatcher/` に控えが無いので、どの台帳も**初めて見る台帳**として扱われる（[dispatcher.md](dispatcher.md) §2.1）:
   - 止まっていた間の出来事（報告・問い・止まり）は通知にしない。最初の見回りは位置を控えるだけ。Chief of Staff は起きたときと同じく台帳を自分で読む（`deck list`）。
   - `doing` の課題の今の `review_request` は拾う。前の係が同じ commit のレビュアーの子課題を作っていれば、それを引き継いで二重に起こさない（起こした回として控える）。ただしその子課題は作成の `by` が前の名前（`terminal:courier`）なので、**ゲートはそのレビューを数えない**。同じ commit のまま頼み直しても、引き継いだ回なので起こし直さない。担当に**新しい commit を push してから、次の回として**頼み直させる（直すものが無ければ `git commit --allow-empty`。同じ回の番号ではレビュアーの detached worktree の置き場がぶつかる。上限を超えれば Chief of Staff が `review_rounds` を上げる）。commit が変われば新しい回として dispatcher がレビュアーを起こし、作成の `by` は `terminal:dispatcher` になる（Chief of Staff が担当に `hc send` で頼む）。1 で待つのはこのためで、待てるなら待つ。
   - すでに `status: review` の課題はゲートを回さない（初めて見た `review` の行は控えるだけ）。担当に `review` へ入り直させる（`deck start <id>` で `doing` に戻してから `andon issue ready`）か、人が PR を見てマージし Chief of Staff が `andon issue finish`。
   - 最初のメッセージの送り直し（`first_message.sent=false`）は2回目の見回りから。
4. **人の手元**: App の名前付け mod を入れ直す（`mods/andon/` を退避してから `host-install`。上の「Herdr App の mod とレイアウト」）。入れ直すまで、新しい tab は名前が付かない。
5. **片付け**: 2 と 3 を確かめたら、前の控え `~/.local/state/andon/courier/` を消してよい（読む者はいない）。

## 12. 実測する（P6b と ADR-0001 S4）

本物の Claude Code でしか確かめられないもの。**見る場所**を書く。落ちたら直しの課題にする。

### ADR-0001 S4: 担当1つの最初の turn

[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md) の受け入れ条件 3。10 の担当（`--plugin-dir /opt/andon` と `--append-system-prompt-file …/worker/SKILL.md` で起きる）を、**最初の turn が終わる前に** `hc read andon-a-1` で見る。

- 最初のメッセージに「先に SKILL.md を読む」の行が無いのに、担当が**最初に `deck start <id>` を打つ**。
- `/andon:worker` の名で skill を持つ（担当の画面で `/` を打って候補を見る）。
- `ANDON_HOME=<checkout>/package` で起こした担当（開発中の回避の口）は、checkout の plugin と skill が入る（`hc templates` の本文の `/opt/andon` が `ANDON_HOME` に置き換わっている）。
- 落ちたら: boot の生成（5 の `~/.agents/skills`・template の本文）、`package/.claude-plugin/plugin.json`、template の `command`（`--plugin-dir`・`--append-system-prompt-file`）。

### P6b: フックの4つ（[dispatcher.md](dispatcher.md) §3・§4・§5）

| 見ること | どう起こすか | どこで見るか |
|---|---|---|
| **Stop の block** — turn 中の通知が、Stop で続きの turn として届く | Chief of Staff が長めの turn にいる間に、担当を `deck review` させて通知を積む | Chief of Staff の画面で、turn の終わりに自動で次の turn が始まり `<news>` を読む。控えの `hook` に渡した turn。担当側: `doing` で報告も問いも無いまま止まろうとすると block（`stop-check` の控え `~/.local/state/andon/stop-check/<slug>-<id>.json`） |
| **additionalContext** — 人が送信したとき、待ちが文脈に添えられる | 通知が積まれている間に、人が何か送る | Chief of Staff に「いま添えられた通知を言って」と聞く。待ちが無くても `<delegation project="…">` が毎回添えられる |
| **離席時だけ `hc send`** — 入力欄が空で idle のときだけ送る | Chief of Staff を idle にし、人は何も打たず5秒以上待つ。次に入力欄に書きかけを置いて同じことをする | 空のとき: `hc read` に `<news>` が入る。書きかけのとき: **送られない**（次の回に持ち越す）。薄い字（候補）を書きかけと見誤らない（dispatcher.md §4.3） |
| **Esc のあとの届け直し** — 渡した turn を中断したら1回だけ届け直す | `<news>` を渡された turn を Esc で止める | 次の turn で同じ通知が**1回だけ**もう一度届く（控えの `hook` から消えるまで）。2回以上届くなら落ち |

落ちたら: `/etc/claude-code/managed-settings.json` の `hooks`（`/opt/andon/hooks/stop`・`user-prompt-submit`）が付いているか。フックは落ちたら通す作りなので、**フックが何も出さない**ときは、`andon dispatcher drain` を手で打って出力を見る。

## 13. §5.1 の確かめ方への対応表

[design.md](design.md) §5.1 の 1〜6 を、どの段で・何で確かめるか。

| # | 確かめること | どの段 | 何で見るか |
|---|---|---|---|
| 1 | セルがパッケージから立つ | 2・3・5・6 | セルの repo にパッケージのファイルが無い（`ls ~/src/home-cell`。`Dockerfile` は `FROM` ＋ 2行）。5 の確かめが全部通る。6 のログインがセルの中で済む |
| 2 | 1本流れる | 10 | `kanban show <id>` の `status` が `done`。`issues/<id>/log.jsonl` に `doing` → `review` → `done`。`issues/<id>/library/gate/<head sha>.json` の決定が approve。PR が `MERGED`（`gh pr view <n> --json state`）。担当の端末が片付いた（`hc ls`・`git worktree list`） |
| 3 | 人が触っていない | 10 | `jq -r '.by' /workspace/projects/andon/kanban/issues/<id>/log.jsonl \| sort \| uniq -c` の `by` が、Chief of Staff・dispatcher・担当・レビュアーの `terminal:` だけ（人のユーザー名が無い）。Chief of Staff の turn が「依頼を受ける」と「結果を見る」の2つだけ（`hc read andon-cos`） |
| 4 | 流れたのは Andon 自身の変更 | 10 | PR の repo が `<owner>/andon`（`gh pr view <n> --json url`）。レベル A か B（ゲートの記録の `tier`） |
| 5 | 自分をテストできる | 7・10 | セルの中で `cd /workspace/.ghq/github.com/<owner>/andon && for f in tests/*.sh; do bash "$f"; done` が green（Go・docker は repo の `mise.toml`・環境の備品に依る。`tests/image.sh` は docker が無ければ SKIP） |
| 6 | 仮設は止まっていない | （ホスト・仮設側） | 仮設のセルの台帳と端末に異常が無い（`deck lint`・`hc ls`）。仮設のプロジェクトが、Andon のセルができるまで Andon の仕事を持っている。このセルの側では何もしない |

数える指標（design.md §5.1 の最後）の最初の1回は、10 の `log.jsonl` から数える: 人の `by` の行の数（介在回数）・依頼書からマージまでの時間・ゲートの `fix`＋`needs_human` の数。数えるのは `andon measure --since <日付> --until <日付>`（`projects/andon/` で打つ。読むだけ。`--json` は [contract.md](contract.md) §13 の形）。

## 14. 版を切る

版ごとの変更と「上げるときにすること」は [CHANGELOG.md](../CHANGELOG.md) に書く。**書くのは版を切るときだけ**で、PR ごとには触らない（全部の PR が版と CHANGELOG を触るとぶつかるので、その形は持たない。[design.md](design.md) §5）。

1. **CHANGELOG の節を足す PR**: 一番上に `## vX.Y.Z — 日付` を足す。前の版からの範囲は `git log <前の版>..origin/main`。各版に「変わったこと」（人の目に効くもの。PR の番号と ADR へのリンクで遡れるように）と「上げるときにすること」（手作業・互換が無いこと・この手順書の節への参照）。
2. **マージ**（人か Chief of Staff）。
3. **annotated tag**: マージした commit に `git tag -a vX.Y.Z -m "vX.Y.Z: <一行>"` を打って push する（打つのは人。tag の push で GHCR に image が出る。[image.md](image.md) の「CI と配布」）。
