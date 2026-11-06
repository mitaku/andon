# 建屋を立てて1本流す（手順）

- 状態: **手順**（2026-11-05 起草。[design.md](design.md) §5.2 の P7 の準備）。読み手は人。ホスト（docker と bash がある Linux か WSL）で打つ。
- 入力（正）: andon main の `README.md`・`init/`・`internal/plantinit`・`internal/boot`・`docs/image.md`・[courier.md](courier.md)・[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md)。コードを読んで書いた。**打って確かめたものではない**（実測は P7 で人がする。この文書の「通ったら見えるもの」と食い違えば、実測が正で、この文書を直す）。
- 書き方: `<owner>`（GitHub の持ち主）・`<版>`（image の tag）・`<email>` は自分の値に置き換える。建屋の名前（slug）は例として `home`。

## 0. 先に知っておくこと: 通らなかった所（直した）

コードを読んで見つけた3つは直した。F1「image に `andon` が焼かれていない」と `package/environment.md` が無いこと（image が `/opt/andon/bin/andon` を持ち、`tests/image.sh` が CI の実ビルドで 2・3・5 の道を通す）、F2「`andon chief-of-staff` の中身が無い」（9）、F3「`andon task start` の中身が無い」（10）。

P7 で見つけた F4「建屋の `andon` が uid 1001 で、ホストの利用者（uid 1000）の持ち物の建屋の repo を読み書きできない」も直した。`andon` は uid・gid 1000 で動く（WSL・cloud-init の既定の利用者と同じ）。`.ssh/authorized_keys` が読めず `ssh は無効` になる・`mkdir /workspace/artifacts: permission denied` の警告・7 の `ghq get` と 8 の `mkdir` が落ちる、はこれが原因だった。前の image で作った volume（中身が uid 1001 の持ち物）は、boot が起動のたびに `andon` の持ち物へ直すので、作り直さなくてよい（`down -v` は要らない）。

P7 で見つけた F5「Herdr App から起こした端末に `claude` が見えず、Chief of Staff が `env: 'claude': No such file or directory` で落ちる」も直した。sshd が image の環境を引き継がず、App が SSH 越しに起こす herdr-core serve の `PATH` に `~/.local/bin` と aqua の道具が無かった。image が sshd に環境を渡し、boot が herdr-core を `~/.local/bin`（App が置く場所）に揃える（4 の「Herdr App から建屋へつなぐ」）。

## 1. image を取る

**やること**: private の image を取る。GHCR の package は private なので、`read:packages` を持つ token が要る。

```bash
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u <github-user> --password-stdin
docker pull ghcr.io/<owner>/andon:<版>      # <版> は main の 40桁の commit SHA か vX.Y.Z
```

- **通ったら見えるもの**: `docker image ls ghcr.io/<owner>/andon` に行が出る。
- **落ちたら見る所**:
  - `denied`・`unauthorized` — token に `read:packages` があるか。組織の package なら SSO の承認。package の「Manage access」にこのアカウントがあるか。
  - `manifest unknown` — tag は main の配布完了後に出る（[image.md](image.md)）。40桁の SHA は GitHub の Actions の「tests」の publish job で確かめる。
  - `no matching manifest` — 配布は Linux amd64 だけ（[image.md](image.md)）。

## 2. 建屋の repo を作る（`andon init`）

建屋の repo は `andon init <slug>` が作る（雛形は `init/`。作ったあとは Andon に追従しない）。置き場は `~/src/andon-<slug>`（`andon-up` が `$HOME/src/andon-*` を探す。`ANDON_PLANTS` で変えられる）。

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
- `--owner` は `andon.toml` の `owner`。委譲のファイル（`delegation.json`）をこの email の commit だけが信じられるようにする。
- **通ったら見えるもの**: `~/src/andon-home/` に `Dockerfile`（`FROM` ＋ 2行）・`andon.toml`（`slug = "home"`）・`bin/bootstrap`・`local/`・`gate.local.json`・`AGENTS.local.md`・`wiki/`・`projects/`。標準出力に `建屋の repo を作った`。パッケージのファイルは無い（design.md §5.1 の 1）。
- **落ちたら見る所**: 取り出しの `docker run` が落ちた（`manifest unknown` など）— 1 の pull を先に通す。`~/.local/bin/andon.new` が残っていれば消す。`はすでにあり、空でない` — 上書きしない。別の `--dir` か、中身を退避する。`slug は英小文字・数字・-` — slug の綴り。
- 建屋の repo を git で持つなら `git init` して commit する（`.ghq/`・`.env` などは雛形の `.gitignore` が外している）。

## 3. 起動する（`bin/bootstrap`）

建屋の repo は **uid 1000 の利用者**の持ち物にする（建屋の `andon` は uid 1000 で動き、建屋の repo をそのまま `/workspace` に載せる）。WSL・cloud-init の既定の利用者は 1000（`id -u` で確かめる）。違うと `bin/bootstrap` は起こす前に止まる。

```bash
cd ~/src/andon-home
bin/bootstrap --dry-run     # docker を打たずに、打つものを出す
bin/bootstrap               # 初回: image を焼く → compose.yaml を取り出す → 起動 → herdr に入る
```

番号（`SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT`）は建屋の `.env` に書く。**建屋を2つ以上立てるなら、ずらす**（ホストの同じ番号を取り合う）。書かなければ既定。

**`.env` に番号を書く前に、ホストで使われている番号を確かめる**（既定は `SSH_PORT=2222`・`PORTLESS_PORT=1355`。ほかの建屋・別の docker・ホストのほかのものと同じ番号だと、起動がポート使用中で落ちる）。

```bash
docker ps -a --format '{{.Names}}\t{{.Ports}}'     # 公開している port（止まっているコンテナも拾う）
ss -ltn                                            # いまホストで listen している番号
grep -rn -i '^ *\(Port\|LocalForward\)' ~/.ssh/config ~/.ssh/config.d/    # ssh config で使っている番号
```

- WSL＋Windows の App なら、Windows 側の ssh config（`C:\Users\<user>\.ssh\config`。4 の「WSL＋Windows の App から入る」）も同じ grep で見る。
- **通ったら見えるもの**: 選んだ `SSH_PORT`・`PORTLESS_PORT`・`FILES_PORT` のどれも、上の3つの出力に出てこない。選んだ番号を `.env` に書いてから `bin/bootstrap`。
- **落ちたら見る所**: `bin/bootstrap` が `port is already allocated`・`address already in use` で落ちる — 出た番号を上の3つで引き、持ち主の側を動かすか、`.env` の番号を変えて打ち直す。起動のあとで `.env` の番号を変えたなら、`host-install` が置いた ssh config の `Port`・`LocalForward` も同じ番号にする（4 の落ちたら見る所）。

- **何が起きるか**: `Dockerfile` の `FROM` から `andon-home:local` を焼く（`local/setup.sh` を走らせる）→ image の `/opt/andon/compose.yaml` を `.andon/` に取り出す → `docker compose up -d` → ログに `[andon boot] ready` が出るまで待つ（最長5分）→ `herdr` を開く。もう一度打てば同じセッションに戻る。
- **通ったら見えるもの**: `bin/bootstrap status` で `dev` が Up。`bin/bootstrap logs` に `[andon boot] ready`（と portless・sshd・クーリエを起こした行）。herdr の画面が開く。andon が uid 1001 だった版の image から上げた最初の起動では、`持ち主を直した: /opt/andon-home/…（uid 1001 → 1000）` の行も出る（volume ごとに1回だけ）。
- **落ちたら見る所**:
  - `Dockerfile の FROM を埋める` — 2 の `--image`。
  - `建屋の repo（…）の持ち主が uid <n>` — 建屋の repo が uid 1000 の利用者の持ち物でない。uid 1000 の利用者で 2 の `andon init` からやり直す（uid を合わせ直す仕組みは無い）。
  - `docker デーモンに繋がらない` — docker が起きているか。
  - 5分待っても `ready` が出ず `準備の完了を確かめられなかった` — `bin/bootstrap logs` で boot の警告の行を見る。`andon help` が `command not found` なら `FROM` の image が古い（`andon` を焼く前の版）。
  - ポートが使用中 — `.env` の番号を変えて `bin/bootstrap` を打ち直す。
  - `local/setup.sh` が落ちる — `bin/bootstrap` の build のログ。建屋固有の追加（`local/aqua.yaml`・`local/setup.sh`）が原因。

## 4. ホストの道具を置く（`host-install`）

```bash
bin/bootstrap host-install
```

`andon-up`（PC を起こしたあとに全建屋を起こす）を `~/.local/bin` に、herdr の設定を `~/.config/herdr/`、ssh config を `~/.ssh/config.d/andon-home.conf` に、cloud-init の雛形を `.andon/` に置く。**今あるファイルは上書きしない**。

- **通ったら見えるもの**: `+ <path>`（新しく置いた）か `= <path>`（同じ）。違うものは `! <path> はすでにあり、雛形と違う`（触らない。入れ替えるなら退避してから打ち直す）。`~/.ssh/config` に `Include config.d/*` が無ければ、足す1行を案内する。
- **落ちたら見る所**: `.env の SSH_PORT・PORTLESS_PORT は数字だけ`。`andon-up` が見つからない — `~/.local/bin` が PATH にあるか。 WSL で `HERDR_APP_HOME` が未指定なら、ほかのホストの道具も置く前に止まる。下の手順で App の設定フォルダを指定して打ち直す。

### 手元の端末から SSH で入る

`host-install` が置いた ssh config（`~/.ssh/config.d/andon-home.conf`）の `Host andon-home` で、手元の端末から建屋の中へ入る。sshd は**公開鍵が置いてあるときだけ**起きる。鍵は人の手元にある既存のものを使う（作り方はここでは扱わない）。

```bash
cd ~/src/andon-home
git check-ignore .ssh/authorized_keys     # .ssh/ が .gitignore 済みか（パスが出ればよい）
mkdir -p .ssh && chmod 700 .ssh
cat ~/.ssh/<公開鍵のファイル>.pub >> .ssh/authorized_keys     # <公開鍵のファイル> は自分の .pub（例 id_ed25519.pub）
chmod 600 .ssh/authorized_keys
bin/bootstrap down && bin/bootstrap up    # 起こし直す（動いているときの up は何もしない）
bin/bootstrap logs                        # 確かめたら Ctrl-C
ssh andon-home
```

- 建屋の repo はコンテナの `/workspace` に載っているので、ホストで置いた `.ssh/authorized_keys` を boot が読み、建屋の中の `~/.ssh/authorized_keys` に写す。ホスト鍵は volume に残るので、作り直しても変わらない。
- `git check-ignore` が何も出さなければ、`.ssh/` が `.gitignore` に無い。公開鍵を commit しないよう、先に `.gitignore` に `.ssh/` を足す。
- **通ったら見えるもの**: ログに `sshd を起動した（コンテナの 22 番。ホスト側の番号は .env の SSH_PORT）`。`ssh andon-home` で建屋の中のシェルに入り、`hostname` が slug（`home`）。ssh config の `LocalForward` で、手元のブラウザから portless の名前（`http://<app>.home.localhost:<PORTLESS_PORT>`）も引ける。
- **落ちたら見る所**:
  - ログに `ssh は無効（使うなら .ssh/authorized_keys に公開鍵を置いて再起動）` — `.ssh/authorized_keys` が無い。置く場所は建屋の repo の直下（`~/src/andon-home/.ssh/`）。置いたあと、`down` → `up` で起こし直す。
  - ログに `ssh は無効（.ssh/authorized_keys があるが読めない: …）` — 鍵はあるが、建屋の `andon`（uid 1000）から読めない。`ls -ln .ssh .ssh/authorized_keys` で持ち主が 1000 か見る（`sudo` で作ると root の持ち物になる）。違えば `sudo chown -R 1000:1000 .ssh` で直し、`down` → `up`。
  - `Connection refused`・`Connection timed out` — `.env` の `SSH_PORT` と、ssh config の `Port` が同じか（`.env` を変えたあと `host-install` を打ち直すなら、古い `~/.ssh/config.d/andon-home.conf` を退避してから。`host-install` は今あるファイルを上書きしない）。ほかの建屋や別のものと `SSH_PORT` が重なると、compose の起動がポート使用中で落ちる（3 の落ちたら見る所）。
  - `Permission denied (publickey)` — `authorized_keys` に入っている公開鍵が、手元でつかわれる鍵と対か。`ssh -v andon-home` で提示された鍵を見る。置いたあとで起こし直していないときも同じ（boot が起動時に写す）。
  - `ssh: Could not resolve hostname andon-home` — `~/.ssh/config` の先頭に `Include config.d/*` の1行が無い。`host-install` の出力が案内する1行を足す。
  - `REMOTE HOST IDENTIFICATION HAS CHANGED`（同じ `SSH_PORT` で別の建屋に入ったとき。ProxyJump 先の別マシンが同じ番号で待っていて、その記録と `[127.0.0.1]:<SSH_PORT>` がぶつかるときも出る）— **消す前に、記録の指紋と建屋のホスト鍵の指紋を照らす**。
    ```bash
    ssh-keygen -F "[127.0.0.1]:<SSH_PORT>" -f ~/.ssh/known_hosts -l     # 手元に記録されている指紋
    bin/bootstrap sh        # 建屋の中のシェル（建屋の repo で）。入ったら次の1行、終わったら exit
    ssh-keygen -lf ~/.ssh/ssh_host_ed25519_key.pub     # 建屋のホスト鍵の指紋
    ```
    建屋のホスト鍵は boot が volume に置く（建屋の中の `~/.ssh/ssh_host_ed25519_key.pub`。作り直しても変わらない）。**指紋（`SHA256:…`）が同じ**なら、記録は正しいので消さない（別の原因を探す）。**違う**なら、記録は別のマシンのもの。その行だけ `ssh-keygen -R "[127.0.0.1]:<SSH_PORT>"` で消して入り直す。ProxyJump 先の記録を残したいなら、`SSH_PORT` を変える（3）ほうが安全。

### WSL＋Windows の App から入る

WSL に建屋があり、Herdr App が Windows 側で動くとき、`host-install` が置くのは **WSL 側**の `~/.ssh/config.d/andon-home.conf` だけ（Windows 側の ssh config と鍵には触らない。`host-install` はこのために変えない）。App の Add Machine が使う Windows 側の ssh config は、人が書く。建屋の sshd は WSL の `127.0.0.1:<SSH_PORT>` で待っていて、WSL の localhost 転送（既定で有効）で Windows の `127.0.0.1:<SSH_PORT>` からも届く。

Windows の `C:\Users\<user>\.ssh\config` に足す（`<…>` は自分の値。番号は `.env` の `SSH_PORT`）:

```
Host andon-home
  HostName 127.0.0.1
  Port <SSH_PORT>
  User andon
  IdentityFile C:\Users\<user>\.ssh\<秘密鍵のファイル>
```

`IdentityFile` の鍵の公開鍵（`<秘密鍵のファイル>.pub`）が、建屋の repo の `.ssh/authorized_keys` に入っていること（上の「手元の端末から SSH で入る」。WSL の鍵とは別の鍵なら、その公開鍵も足して `bin/bootstrap down && bin/bootstrap up`）。そのうえで App の Add Machine に `andon-home` を指定する。

```powershell
ssh andon-home hostname      # Windows の PowerShell から。App が使うのと同じ設定で届くか
```

- **通ったら見えるもの**: `hostname` が slug（`home`）を返す。App の Add Machine で `andon-home` が繋がり、建屋の Space が出る。
- **落ちたら見る所**: `Connection refused` — WSL 側で `ssh andon-home` が通るかを先に見る（通らなければ上の「手元の端末から SSH で入る」）。WSL で通り Windows で通らないなら、localhost 転送が切れている（`%USERPROFILE%\.wslconfig` の `[wsl2]` の `localhostForwarding=false`）か、Windows 側のほかのものが同じ番号を取っている（`netstat -ano | findstr :<SSH_PORT>`）。`Permission denied (publickey)` — Windows 側の `IdentityFile` の公開鍵が `authorized_keys` に無い。`ssh -v andon-home` で提示された鍵を見る。`Bad owner or permissions` — Windows の秘密鍵の ACL。自分だけが読める形にする。

### Herdr App から建屋へつなぐ（Add Machine）

App の Add Machine で建屋（`Host andon-home`）を足すと、App は SSH 越しに herdr-core を建屋の `~/.local/bin/herdr-core` に置き（Copy HC）、`herdr-core serve` を起こす。以後の Update HC もこのファイルを差し替える。**建屋の herdr-core の版は App が決める**。

- SSH のセッションには image の環境（`PATH`・`AQUA_*`・`GHQ_ROOT`・`LANG`・`CLAUDE_CONFIG_DIR` など）が渡る（image の `/etc/ssh/sshd_config.d/andon.conf` の `SetEnv`）。App の serve から起きる端末にも `claude`・`hc`・`gh` が見える。
- boot は `~/.local/bin/herdr-core` が無いか symlink のときだけ image の種（aqua の herdr-core）へ繋ぐ。**実ファイル（App が置いたもの）は触らない**。`~/.local/bin/hc` は herdr-core への相対 symlink。`~/.local/bin` は PATH で aqua より先なので、boot がクーリエを起こす `hc` も、それが起こす core も、App の serve と同じ herdr-core になる。
- `~/.local/bin` はコンテナ層。image を上げて作り直すと、App が置いた herdr-core は消えて image の種に戻る。App で Update HC を打ち直し（App が古い版を知らせればそれに従う）、建屋の中で `hc server restart`（core を新しい版で起こし直す。端末は生きたまま）。`hc --version` と `hc server status` の `version` が App の版と同じなら揃っている。
- **App の serve を起こし直す要るとき**（App に建屋が出ない・serve が古い版のまま・SSH の環境を変えたあと）: App でその machine を切って繋ぎ直す（serve は繋ぐときに起きる）。それでも出なければ `bin/bootstrap down` → `up` で建屋ごと起こし直し、上の Update HC から。
- **落ちたら見る所**: 端末が `env: 'claude': No such file or directory` で落ちる・SSH のシェルで `hc`・`gh` が打てない — image が古い（SSH に環境が渡らない版）。`ssh andon-home 'echo $PATH'` の先頭が `/opt/andon-home/.local/bin` か。版が割れている（`hc --version` と `hc server status` の `version` が違う）— `readlink ~/.local/bin/herdr-core ~/.local/bin/hc` を見てから `hc server restart`。

### Herdr App の名前付け mod

同じ `host-install` が App の設定フォルダの `mods/andon/` に名前付け mod を置く。置き場は App と同じく `HERDR_APP_HOME`、未指定なら `XDG_CONFIG_HOME/herdr-app`、それも未指定なら `~/.config/herdr-app` の順で選ぶ。App が起動時に使う環境変数と揃えて実行する。既存のディレクトリは、不完全でも中身を補わずそのまま残す。更新するときは退避してから打ち直す。

**WSL で実行し、App は Windows 側で動く場合**は、Windows の App の設定フォルダを WSL から見たパスで `HERDR_APP_HOME` に指定する。WSL では未指定だと書き込み前に止まり、案内を出す。Windows の既定値は `%USERPROFILE%\.config\herdr-app`。App 側で `HERDR_APP_HOME` や `XDG_CONFIG_HOME` を指定しているなら、その設定に合う実際のフォルダを選ぶ。

```bash
# <user> は Windows の利用者名に置き換える。App の置き場を変えていればそのパスを渡す。
HERDR_APP_HOME="$(wslpath 'C:\Users\<user>\.config\herdr-app')" bin/bootstrap host-install
```

WSL 内で Linux 版 App を動かしている場合も、その App の設定フォルダを `HERDR_APP_HOME` で明示する。Windows のプロファイルや App の環境をインストーラーが推測することはない。

App の標準レイアウトは mod の追加だけでは起動しない。人が App の command palette でレイアウトを複製し、その個人用レイアウトの常に表示する row または column の `children` に次の block を1つ加える（他の block は残す）。`host-install` はレイアウトを変更しない。

```json
{ "id": "andon-names", "type": "block", "size": { "px": 1 }, "content": { "mod": "andon" } }
```

この block は空で、名前付けだけを行う。別のレイアウトへ切り替えるなら、そこにも block が必要。人が App のあるマシンで `herdr-app check <App の設定フォルダ>/mods/andon` を実行し（Windows の App は Windows 側で実行する）、個人用レイアウトで `sample-site / CoS`・`sample-site / CE A-1 設計`・`sample-site work / A-2 修正` が見えることを確認する。App のエラーは `herdr-app status` で見る。

mod は端末のラベルと実際の pane の所属から既存の Space・tab を名前付けする。同じ名前なら何もせず、作成・選択・フォーカス・削除は行わない。レビュアー自身の題で担当の tab を上書きしない。題の空白・制御文字は1つの空白に揃え、前後の空白を除く。空白だけの題では命名しない。役ごとのアイコンは含まない。

API の根拠は [Herdr App の mod guide](https://github.com/herdrdev/herdr-app/blob/903dc810da0c839ba865d6b190f3aeeceb3a636b/defaults/AGENTS.md) と [snapshot の実装](https://github.com/herdrdev/herdr-app/blob/903dc810da0c839ba865d6b190f3aeeceb3a636b/src/window.rs)。端末の `labels`、pane の `terminal`・`tab`、Space の `id`・`machine`・`tabs` を読む。リモートの App ID にだけ machine の接頭辞が付く。

## 5. 建屋に入って、立ち上がりを確かめる

```bash
bin/bootstrap               # herdr に入る。bash で入るなら: bin/bootstrap sh
```

中で（シェルのペインで）:

```bash
hostname                    # slug（home）
andon help                  # sub-command の一覧
hc ls                       # 端末の一覧。courier が出る
ls ~/.config/herdr-core/templates     # claude・sonnet・fable・codex・codex-review・claude-review・chief-of-staff・chief-engineer
ls -l ~/.agents/skills                # andon-<name> が /opt/andon/skills/<name> への symlink
cat ~/.claude/CLAUDE.md               # @ の2行（environment.md と AGENTS.local.md）
cat /etc/claude-code/managed-settings.json
```

- **通ったら見えるもの**: 上の全部。`hc get courier` が通る（クーリエは `andon courier` を常駐させた端末）。
- **落ちたら見る所**: どれも出なければ boot が走っていない（`bin/bootstrap logs`）。boot は失敗しても続ける作りなので、警告の行を探す。生成だけ失敗したときは `andon boot` を中で打ち直せば同じ結果になる（何度打っても同じ）。`hc` が無い・端末が起きない — herdr-core は初めて `hc` を叩くと起きる。

## 6. 中でログインする（建屋の中で取る）

ブラウザは建屋に無い。URL が出たら**手元のブラウザで開く**（建屋から開こうとしない）。トークンをホストから持ち込まない。

```bash
gh auth login               # GitHub.com → HTTPS → device code（URL とコードを手元で）
gh auth setup-git           # git の credential helper を gh にする（clone・push が通らないとき）
claude                      # 初回に /login。URL を手元で開き、コードを貼る
codex login --device-auth   # Codex を使うとき
```

ログインは volume（`claude`・`codex`・`gh` など）に残る。作り直しても消えない。

- **通ったら見えるもの**: `gh auth status` が Logged in。`claude` が対話に入る。`codex login status`。
- **落ちたら見る所**: `gh` のスコープ（`repo`・`workflow`・`read:packages`）。device code の期限切れは打ち直す。`claude` が毎回ログインを求める — `claude` の volume が付いているか（`bin/bootstrap sh` で `ls ~/.claude`）。

## 7. Andon の repo を迎える

```bash
ghq get github.com/<owner>/andon        # GHQ_ROOT=/workspace/.ghq の下に入る
ghq list
```

- **通ったら見えるもの**: `/workspace/.ghq/github.com/<owner>/andon`。
- **落ちたら見る所**: `Repository not found` — `gh auth status` と repo の権限。`could not read Username` — `gh auth setup-git`。

## 8. プロジェクト `andon` の台帳を作る

台帳はプロジェクトごとに `projects/<slug>/.flightdeck/`（git の外）。slug はプロジェクト名＝フォルダ名。

```bash
mkdir -p /workspace/projects/andon && cd /workspace/projects/andon
kanban init --prefix A --repo github.com/<owner>/andon
kanban list ; kanban lint
```

- **通ったら見えるもの**: `created …/.flightdeck`。`config.yaml` に `prefix: A` と `repos`。`kanban lint` が問題なし。
- **落ちたら見る所**: `already a ledger` — 作ってある（そのまま使う）。`--repo must be host/owner/repo` — 綴り。`checkClock` の失敗 — コンテナの時計。
- `projects/andon/coordinator.md`（人のファイル: 目標・並行の上限・独立レビューを入れる条件）は任意。無ければ並行の上限は 2。

## 9. Chief of Staff を起こす

```bash
cd /workspace/projects/andon
andon chief-of-staff        # 起こす（動いていればそれを使う）。画面には乗らない
```

コマンドは `hc attach` を打たない（[ADR-0002](adr/0002-herdr-spaces.md) 決めたこと 14）。Chief of Staff はラベル `herdr.space=andon-andon`・`herdr.tab=andon-andon-cos` で Herdr App の指揮側の Space に出るので、人は App のその tab で話す。最後の行に出た場所が「<Space 名> / <tab 名>」の形で出る（`andon / CoS`）。名前を付けるのは App の mod（S5）で、入るまでは Space と tab が名無し（Space はフォルダ名・tab は番号）に見える。App の無い所で話すときは人が自分で `hc attach <address>`。

起きた Chief of Staff は、skill の手順で最初に `andon chief-of-staff` を打ち、自分を `config.yaml` の `coordinator` に載せる。端末を起き直して address が変わっても、これで直る。

- **通ったら見えるもの**: `andon chief-of-staff: andon の Chief of Staff は <address>（andon-cos・起こした）` と `andon chief-of-staff: 出た場所 andon / CoS（<address>）`。App に Chief of Staff の Space と tab が出る（画面は移らない）。`hc ls` に `role=chief-of-staff project=andon` の端末（名前 `andon-cos`）。`config.yaml` の `coordinator` がその address。もう一度打つと起こさずに `動いているものを使う`。Chief of Staff が自分の skill（system prompt に入った役の手順）に従って台帳を読む。
- **App から起こした端末が `env: 'claude': No such file or directory`（exit 127）で落ち、終了コード 3 になる** — App の serve に image の環境が渡っていない（4 の「Herdr App から建屋へつなぐ」）。直した image に上げ、App で machine を繋ぎ直してから打ち直す。`config.yaml` の `coordinator` が落ちた端末を指したままでも、打ち直せば新しく起こした address に書き換わる（落ちた端末は `hc ls` に出ないので数えない）。
- **最初のメッセージが届いたかは** Chief of Staff の画面（`hc read <address>`）で、台帳を読み始めたかで見る。`andon chief-of-staff` の終了コード 0 だけでは分からない（hook で止まっても 0 で返った）。入力欄にメッセージが残って動かないときは、`UserPromptSubmit operation blocked by hook` の行が出ていないかを見る。
- **落ちた Chief of Staff を起こし直す**: image を上げ直し（`bin/bootstrap` で作り直す）、落ちた端末を `hc kill <address>` してから `andon chief-of-staff`。`hc server restart` が `no systemd user manager here: the core ends with whatever started it` を出すのは、systemd の無いコンテナである建屋では正常（core は起こした側と一緒に終わる）。
- **落ちたら見る所**: `プロファイル … の template が無い` — 5 の `ls ~/.config/herdr-core/templates`（boot の生成）。`Chief of Staff が 2 人いる` — `hc ls --label role=chief-of-staff --label project=andon` を見て1人に減らす（`hc kill`）。`最初のメッセージが届いていない`（信頼の画面など）— App のその tab（か `hc attach <address>`）で画面に答えてから、出た `hc send` を打つ（終了コード 3）。`出た場所 App の Space の外` — ラベルより前に起きた Chief of Staff を使った。`hc kill` してから打ち直すと Space に出る。起きるが skill が効かない — 10 の S4 の確かめ。クーリエがこの台帳を飛ばす — `coordinator` の端末が live でない（`hc get <address>`）。**初めて見る台帳は、クーリエが控えるだけで動かさない**（[courier.md](courier.md) §2）。1回の見回りのあとに効く。

## 10. 小さい PR を1本流す

### 先に: マージの委譲と owner の email

依頼書の `Merge: auto` でマージされるには、ゲートが approve するだけでは足りず、**委譲の `merge` がそのレベルを許している**ことが要る（[contract.md](contract.md) §11）。委譲（`projects/andon/delegation.json`）と条件表（`gate.local.json`）は、**それを最後に変えた commit の author の email が `andon.toml` の `owner` と一致するときだけ**信じられる。違うと無委譲（全部人）・条件表を重ねない、として読まれる。

```bash
cd ~/src/andon-home
grep '^owner' andon.toml                    # owner の email（2 の --owner）
git config user.email                       # 建屋の repo の commit で使われる email
git config user.email '<owner の email>'    # 違うなら合わせる（この repo の中だけ）
andon delegate grant merge B --evidence '<根拠のパスか引用>'    # projects/andon/ で打つ（か --project andon を付ける）。B はマージを許すレベルの上限（A / B / C）
andon delegate show                                              # 同じく projects/andon/ で（か --project andon）
git log -1 --format='%ae' -- projects/andon/delegation.json gate.local.json
```

- `grant` は `delegation.json` と `delegation.md` を書いて commit する。Chief of Staff・担当・クーリエの端末では打てない（人が打つ）。`andon` プロジェクトで許せる上限は `B`（contract.md §11）。
- **通ったら見えるもの**: `grant` が `projects/andon/delegation.json: …` の1行だけを出し、`注意:` の行が出ない。`andon delegate show` に `merge` の値が出る。最後の `git log` の email が `owner` と同じ。`gate.local.json` を commit し直したときも、同じ email で。
- **落ちたら見る所**: `注意: commit の author（… <…>）が andon.toml の owner（…）でない。読み手は無委譲として読む` — `git config user.email` が owner と違うまま commit した。email を合わせて、`delegation.json` を直す commit を打ち直す（`git commit --amend --reset-author` で author を直すか、`grant` を打ち直す）。`注意: andon.toml に owner が無い` — `andon.toml` の最上段に `owner = "<email>"` を足して commit する。ゲートが `needs_human` で `委譲 merge: … の外` — `show` の値がレベルより低い。`gate.local.json を重ねない: … が owner でない` — 同じ email で `gate.local.json` を commit し直す。

人は依頼を1つ渡す。Chief of Staff が依頼書に切る。Andon 自身の小さい変更（レベル A か B。例: 文書の誤字・テストの追加）を選ぶ。依頼書の Settings は `Review: on`・`Merge: auto`（design.md §5.1 の 2）。ゲートの条件表の `repos` に `github.com/<owner>/andon` を入れておく（レベルの決めは人。建屋の `gate.local.json`）。

流れ（人が触らないところ）: 依頼書 → 担当を起こす → PR → `review-request` → クーリエがレビュアーを起こす → `ready`（レビュー）→ クーリエがゲート → approve → クーリエがマージ → `andon task finish`。

担当を起こすのは Chief of Staff の `andon task start <id>`（`projects/andon/` で打つ）。worktree と branch（`~/.herdr/worktrees/andon/andon-<id>`・`andon/<id>-<題の英字>`）・`hc run`（プロファイルは `task.yaml` の `profile`）・`kanban start`（`doing`・`agent` がその端末）を1行でする。担当はラベル `herdr.space=andon-andon-work`・`herdr.tab=andon-andon-<id>` で App の作業側の Space にタスクの tab として出て（最後の行 `出た場所 andon work / <id> <題>`）、クーリエが起こすレビュアーは担当と同じ tab に横に並ぶ。`andon task finish` で端末が片付くと tab が空になって消える。

- **通ったら見えるもの**: 下の「§5.1 の確かめ方」の対応表。
- **落ちたら見る所**: `andon task start` が終了コード 3（`最初のメッセージが届いていない`。信頼の画面など）— `hc read <address>` で画面を読んで答え、出た `hc send` を打つ（台帳はもう `doing`）。担当が `deck start` を打たない — ADR-0001 の S4（下）。PR が出ない・checks が落ちる — `gh pr checks`。`review-request` を拾わない — `hc read courier`・`~/.local/state/andon/courier/<slug>.json`。ゲートが `needs_human` — 台帳の問い（`kanban questions`、`from: gate`）と `tasks/<id>/library/gate/<head sha>.json` の `reasons`。マージされない — 条件表の `repos.<repo>.merge`。

## 11. クーリエの確かめ

```bash
hc get courier ; hc read courier                       # 見回りの記録
jq . ~/.local/state/andon/courier/home.json 2>/dev/null   # 控え（<slug> は台帳のプロジェクト名なら andon.json）
```

控えは `~/.local/state/andon/courier/<プロジェクト名>.json`（`XDG_STATE_HOME` があればその下）。待ちの通知は `pending`、フックで渡したものは `hook` に残る。通知は台帳と端末の状態から拾う（[courier.md](courier.md) §2.2・§2.3）。**通ったら見えるもの**: 10 の流れの各段（`review`・`ready`・`gate`・`merged`）が、Chief of Staff に `<news>` として1通にまとまって届く。

## 12. 実測する（P6b と ADR-0001 S4）

本物の Claude Code でしか確かめられないもの。**見る場所**を書く。落ちたら直しのタスクにする。

### ADR-0001 S4: 担当1つの最初の turn

[adr/0001-skills-distribution.md](adr/0001-skills-distribution.md) の受け入れ条件 3。10 の担当（`--plugin-dir /opt/andon` と `--append-system-prompt-file …/worker/SKILL.md` で起きる）を、**最初の turn が終わる前に** `hc read andon-a-1` で見る。

- 最初のメッセージに「先に SKILL.md を読む」の行が無いのに、担当が**最初に `deck start <id>` を打つ**。
- `/andon:worker` の名で skill を持つ（担当の画面で `/` を打って候補を見る）。
- `ANDON_HOME=<checkout>/package` で起こした担当（開発中の回避の口）は、checkout の plugin と skill が入る（`hc templates` の本文の `/opt/andon` が `ANDON_HOME` に置き換わっている）。
- 落ちたら: boot の生成（5 の `~/.agents/skills`・template の本文）、`package/.claude-plugin/plugin.json`、template の `command`（`--plugin-dir`・`--append-system-prompt-file`）。

### P6b: フックの4つ（[courier.md](courier.md) §3・§4・§5）

| 見ること | どう起こすか | どこで見るか |
|---|---|---|
| **Stop の block** — turn 中の通知が、Stop で続きの turn として届く | Chief of Staff が長めの turn にいる間に、担当を `deck review` させて通知を積む | Chief of Staff の画面で、turn の終わりに自動で次の turn が始まり `<news>` を読む。控えの `hook` に渡した turn。担当側: `doing` で報告も問いも無いまま止まろうとすると block（`stop-check` の控え `~/.local/state/andon/stop-check/<slug>-<id>.json`） |
| **additionalContext** — 人が送信したとき、待ちが文脈に添えられる | 通知が積まれている間に、人が何か送る | Chief of Staff に「いま添えられた通知を言って」と聞く。待ちが無くても `<delegation project="…">` が毎回添えられる |
| **離席時だけ `hc send`** — 入力欄が空で idle のときだけ送る | Chief of Staff を idle にし、人は何も打たず5秒以上待つ。次に入力欄に書きかけを置いて同じことをする | 空のとき: `hc read` に `<news>` が入る。書きかけのとき: **送られない**（次の回に持ち越す）。薄い字（候補）を書きかけと見誤らない（courier.md §4.3） |
| **Esc のあとの届け直し** — 渡した turn を中断したら1回だけ届け直す | `<news>` を渡された turn を Esc で止める | 次の turn で同じ通知が**1回だけ**もう一度届く（控えの `hook` から消えるまで）。2回以上届くなら落ち |

落ちたら: `/etc/claude-code/managed-settings.json` の `hooks`（`/opt/andon/hooks/stop`・`user-prompt-submit`）が付いているか。フックは落ちたら通す作りなので、**フックが何も出さない**ときは、`andon courier drain` を手で打って出力を見る。

## 13. §5.1 の確かめ方への対応表

[design.md](design.md) §5.1 の 1〜6 を、どの段で・何で確かめるか。

| # | 確かめること | どの段 | 何で見るか |
|---|---|---|---|
| 1 | 建屋がパッケージから立つ | 2・3・5・6 | 建屋の repo にパッケージのファイルが無い（`ls ~/src/andon-home`。`Dockerfile` は `FROM` ＋ 2行）。5 の確かめが全部通る。6 のログインが建屋の中で済む |
| 2 | 1本流れる | 10 | `kanban show <id>` の `status` が `done`。`tasks/<id>/log.jsonl` に `doing` → `review` → `done`。`tasks/<id>/library/gate/<head sha>.json` の決定が approve。PR が `MERGED`（`gh pr view <n> --json state`）。担当の端末が片付いた（`hc ls`・`git worktree list`） |
| 3 | 人が触っていない | 10 | `jq -r '.by' /workspace/projects/andon/.flightdeck/tasks/<id>/log.jsonl \| sort \| uniq -c` の `by` が、Chief of Staff・クーリエ・担当・レビュアーの `terminal:` だけ（人のユーザー名が無い）。Chief of Staff の turn が「依頼を受ける」と「結果を見る」の2つだけ（`hc read andon-cos`） |
| 4 | 流れたのは Andon 自身の変更 | 10 | PR の repo が `<owner>/andon`（`gh pr view <n> --json url`）。レベル A か B（ゲートの記録の `tier`） |
| 5 | 自分をテストできる | 7・10 | 建屋の中で `cd /workspace/.ghq/github.com/<owner>/andon && for f in tests/*.sh; do bash "$f"; done` が green（Go・docker は repo の `mise.toml`・環境の備品に依る。`tests/image.sh` は docker が無ければ SKIP） |
| 6 | 仮設は止まっていない | （ホスト・仮設側） | 仮設の建屋の台帳と端末に異常が無い（`deck lint`・`hc ls`）。仮設のプロジェクトが、Andon の建屋ができるまで Andon の仕事を持っている。この建屋の側では何もしない |

数える指標（design.md §5.1 の最後）の最初の1回は、10 の `log.jsonl` から数える: 人の `by` の行の数（介在回数）・依頼書からマージまでの時間・ゲートの `fix`＋`needs_human` の数。
