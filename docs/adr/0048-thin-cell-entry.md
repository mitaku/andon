# ADR-0048: セルの入口 `bin/bootstrap` を薄くし、本体を image から取り出す（仕様・設計。2026-11-22）

- 状態: **決定（2026-11-22。オーナーが決めた）**。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: セルの repo の `bin/bootstrap` は、セルを作ったときの雛形の写しで、Andon のバージョンを上げても変わらない。この文書は、入口の動きが Andon のバージョンに追従する形と、前に作ったセルをその形へ移す手を決める。ホストで動くコードの置き場を変える決定と、オーナーがセルを上げるときの手を変える決定を含む。
- 語は [design.md](../design.md) §0.1。セルの repo の直下の予約名と、オーナーのファイルの線は [ADR-0005](0005-cell-repo.md) が決めた。image を焼き直さずにパッケージを当てる口は [ADR-0027](0027-apply-package-without-rebuild.md) が決めた。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、`bin/bootstrap` の追従と、前に作ったセルの移し方までにする | この ADR | 生きている |
| 2 | 入口を、セルの repo の薄い入口と、image の中の本体の 2 つに分ける | `init/bin/bootstrap`・`package/host/bootstrap` | 生きている |
| 3 | 薄い入口の仕事を 6 つに限り、本体との約束を作業フォルダと引数と変数 1 つにする | `init/bin/bootstrap`・`package/host/bootstrap` | 生きている |
| 4 | 本体は打つたびに image から取り出し、前に取り出したものを走らせない | `init/bin/bootstrap` | 生きている |
| 5 | image が無いか、image に本体が無いときは、入口が 1 回焼いてから見直す | `init/bin/bootstrap` | 生きている |
| 6 | image を見ずに、名指しした本体で動かす口を 1 つ持つ | `init/bin/bootstrap`・`docs/operate.md` | 生きている |
| 7 | `--dry-run` は docker を打たず、help は image を焼かない | `init/bin/bootstrap` | 生きている |
| 8 | `rebuild` は、image を焼いたあと入口から入り直して、新しい本体で作り直す | `package/host/bootstrap` | 生きている |
| 9 | 本体は `package/host/bootstrap` に置き、gate のレベルは C になる | `package/host/bootstrap`・`package/gate.json` | 生きている |
| 10 | 前に作ったセルは、上げたときに古い入口が止まって案内を出し、オーナーが 1 行を 1 回打つ | `package/compose.yaml`・`docs/operate.md` | 生きている |
| 11 | 雛形の 1 ファイルを出す口 `andon init --print` を足す | `internal/plantinit/plantinit.go` | 生きている |
| 12 | 薄い入口は変えないことを原則にし、変えるときは番号を上げて本体が断る | `init/bin/bootstrap`・`package/host/bootstrap`・`tests/andon-init.sh` | 生きている |
| 13 | Stream Deck の plugin の置き先は本体へ移し、ADR-0045 の「雛形で置き換える」手をこの ADR の手に替える | `package/host/bootstrap`・`docs/operate.md` | 生きている |
| 14 | 本体は、焼き直さずに当てる口では変わらないものに入れる | `docs/operate.md` | 生きている |
| 15 | 案内の 1 行は行の最後に出し、`exec` は止まらないことを受け入れ条件 10 に写す | `package/compose.yaml`・`tests/image.sh` | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナーの 1 行（2026-11-22）: 「前に作ったセルの bin/bootstrap を Andon に追従させる」。

Andon のバージョンを上げても、ホストで打つ入口の `bin/bootstrap` だけは、セルを作ったときの写しのまま変わらない。そのため、あとから入口に入った直しが前に作ったセルに届かず、届いていないことに誰も気づけない。Andon を開発しているセルの入口は v0.0.11 の雛形と同じで、そのあとの変更 8 件が届いていない。そのうち動きが変わるものは 4 件で、引数なしで起動だけにする直し、help の直し、mod `waiting` を置く直し、Stream Deck の plugin を置く直しである。オーナーと言い直しを確かめた時点では 7 件と 3 件で、そのあと plugin の置き先が main に入った。目的は、Andon のバージョンを上げれば入口の動きも一緒に新しくなり、オーナーが覚えておく手が増えないことである。

モデル: 調べなかった（課題の状態を動かす流れを変えない。変えるのは、ホストで動く入口のファイルの置き場と取り出し方である）

## 読んで分かった事実（2026-11-22。main `d040e68`）

1. **雛形は写したあと追従しない。** `andon init` は、`andon` に埋め込んだ `init/` をセルの repo に写す（`internal/plantinit/plantinit.go`）。[design.md](../design.md) §2.2 と [operate.md](../operate.md) の 2 は「作ったあとは Andon に追従しない」と書く。v0.0.13 の CHANGELOG の「上げるときにすること」は、「セルの `bin/bootstrap` は `rebuild` では変わらない」と書く。
2. **`init/bin/bootstrap` はセルごとの値を持たない。** 名前が `.tmpl` でなく、slug は `andon.toml` から、`FROM` は `Dockerfile` から、番号は `.env` から読む。418 行である。
3. **入口の変更は 15 件ある。** 最初の commit（2026-11-05）から今までで、`host-install` の置き先を足したものが 4 件、動きと守りを変えたものが 5 件、語と注記だけを変えたものが 5 件、最初の 1 件である。バージョンごとに見ると、v0.0.12 までに 8 件、v0.0.13 で 5 件、そのあとに 2 件入った。
4. **Andon を開発しているセルの入口は v0.0.11 の雛形と同じである。** `git show v0.0.11:init/bin/bootstrap` と 1 字も違わない。セルの repo では、作ったときの 1 つの commit のあと、このファイルは変わっていない。今の雛形との差は、足す 72 行と消す 17 行で、mod の一覧に `waiting` が無く、Stream Deck の plugin の置き先も無い。
5. **雛形から写るほかのファイルは、オーナーが書くものである。** `andon.toml`、`Dockerfile`、`gate.local.json`、`AGENTS.local.md`、`local/` は、セルごとの中身をオーナーが書く。あとから Andon が雛形を変えて、前に作ったセルに手当てが要ったのは 3 件である。`artifacts/` を `files/` に改めた移し替え（[operate.md](../operate.md) に手順がある）、wiki を新しい形へ移す移し替え（CHANGELOG に、Chief of Staff が 1 回すると書いてある）、`.gitignore` の `memory/` の 1 行（[operate.md](../operate.md) の 2 に 1 行の手順がある）である。初めの 2 件は、セルにたまった中身を動かす移し替えで、雛形のファイルを届けるだけでは済まない。
6. **入口は今も、パッケージを image から取り出している。** `compose.yaml` は、打つたびに `docker create` と `docker cp` で `.andon/compose.yaml` に取り出す。一時の名前に取り出してから `mv` で置き換える。`host-install` は、`/opt/andon/host/` を一時のフォルダに取り出して写す。`.andon/` はセルの `.gitignore` が外している。
7. **入口と image の中身は、どちらの向きにもずれる。** `host-install` が置く mod の名前は入口に並べてあり、中身は image から取り出す。入口が古いと、image にある mod が置かれない（事実 4）。入口だけを新しくして image が古いと、`cp -R` が無いフォルダで落ち、`set -e` でそこで止まる。
8. **設計の初めの表は、入口が自分の更新を image から取り出すと書いていた。** [design.md](../design.md) §3 の表の `bin/bootstrap` の行は「イメージから `compose.yaml` と自分の更新版を取り出す」と書く。更新を取り出す部分は作られていない。§2.3 は「セルの repo にパッケージのファイルは 1 つも無い。だから雛形の更新の取り込みが要らない」と書くが、`bin/bootstrap` はパッケージと一緒に変わるコードで、この文の例外になっている。
9. **`bin/` はオーナーのファイルである。** [ADR-0005](0005-cell-repo.md) の決めたこと 2 の表は、`bin/` の書く人をオーナーとする。決めたこと 6 は、managed settings の deny に `Edit(//workspace/bin/**)` を足した。boot がセルの repo に書くのは `files/` のフォルダだけで、オーナーのファイルは読むだけである（`internal/boot/`）。
10. **セルの repo は、セルの中から書ける。** セルの repo はコンテナの `/workspace` に載る。`bin/bootstrap` も `.andon/compose.yaml` も、セルの中のプロセスが書き換えられる場所にあり、ホストで使われる。deny が塞ぐのは Claude Code の道具と Bash のふつうの形までである（ADR-0005 決めたこと 6）。
11. **gate のレベルは、`init/**` が B、`package/host/**` と `package/compose.yaml` が C である**（`package/gate.json`）。ホストで動くコードのうち、レベル B なのは `init/bin/bootstrap` だけである。`package/host/andon-up` は C である。
12. **`compose.yaml` は、渡されていない変数で止まる書き方を、もう使っている。** `image: ${ANDON_IMAGE:?bin/bootstrap から起動する}` と `hostname: ${ANDON_SLUG:?bin/bootstrap から起動する}` である。Compose の文書は、`${VAR:?error}` を「値があって空でなければその値、そうでなければ誤りで終わる」と書く。誤りの文の形と、どの sub-command でも動く前に止まるかは、文書に書かれていない。
13. **焼き直さずに当てる口は、ホストが取り出すものを変えない。** `image/apply-main.sh apply` は、セルの中の `/opt/andon` を入れ替える。ホストの入口が取り出すのは image の中身なので、`compose.yaml` と `host/` は焼き直すまで変わらない（[operate.md](../operate.md) の「当てないもの」）。
14. **入口の検査は 2 つの継ぎ目にある。** `tests/andon-init.sh` は、`package/` を偽の image の根（`ANDON_IMAGE_ROOT`）に写し、偽の docker で入口を動かす。`pass` の呼び出しは 78 か所ある。`package/host/` の下のファイルを変えると、`tests/lib/scope.sh` は今のままでこの検査を選ぶ。`tests/image.sh` は、CI の本物の docker で、`andon init` から `bin/bootstrap up` までを通す。セルの中には docker が無い。
15. **古い入口の `host-install` は、`docker compose` を打たない。** image があるかを見て、`host/` を取り出して写すだけである。初めのバージョンから今まで同じである。
16. **今の入口は、image に頼らずに焼き直せる。** `rebuild` と `rebuild --from` の中身はセルの repo にあり、手元に image が 1 つも無くても動く。`rebuild --from` が焼く `andon:local-…` の image は、レジストリに無い。[operate.md](../operate.md) の 3 は、`rebuild --from` を、マージ前の branch を試す日々の手として書く。
17. **ホストの `andon` は、手で取り出し直す。** [operate.md](../operate.md) の 2 は、`andon init` を打つために image から `andon` を `~/.local/bin` に取り出し、「image を上げたら取り出し直す」と書く。配っているのは Linux の amd64 だけである。
18. **Stream Deck の plugin の置き先は、もう入口に入っている。** `init/bin/bootstrap` の関数 `install_streamdeck` が、`host-install` の終わりに、image の `host/streamdeck/andon.lamp.sdPlugin` を環境変数 `STREAMDECK_PLUGINS` の置き場へ写す。`host-install` は初めに、`STREAMDECK_PLUGINS` が絶対パスかを確かめる。入口は、ほかにも `HERDR_APP_HOME`、`ANDON_HOST_BIN`、`XDG_CONFIG_HOME` を環境変数から読む。[operate.md](../operate.md) の 4 の「前に作ったセル」は、セルの中で `andon init x --dir` を打って雛形を作り、`/workspace/bin/bootstrap` に写す手順を書く。`tests/andon-init.sh` は、plugin の置き方、plugin の無い前の image で置かずに知らせること、`bin/bootstrap` が雛形と同じであることを見る。

## 前提（同意済み。番号）

オーナーが 2026-11-22 に、言い直しと合わせて同意した。

1. いちばんの害は、誤りが出ずに、黙って動きが違うことである。
2. 何もしないと、`host-install` の置き先を足すたびに、前に作ったセルでは置かれない。次は Stream Deck の plugin で起きる（[ADR-0045](0045-stream-deck-lamp.md) 決めたこと 12）。
3. 入口は今も `compose.yaml` とホストの道具を image から取り出している。同じ道で届け、新しい配り方は作らない。
4. 届ける手は、`Dockerfile` の `FROM` を上げて `bin/bootstrap rebuild` を打つ今の手に乗せる。ホストに要るのは docker と bash だけ、のままにする。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-22 範囲は、`bin/bootstrap` の中身が Andon のバージョンに追従する仕組みと、前に作ったセルの移し方までにする（オーナーが決めた）。** `host-install` が今あるファイルを上書きしない決まりは変えない。雛形のほかのファイルに行を届ける口は作らない。
   理由: `host-install` の決まりは、オーナーが手を入れた mod を守るためのもので、別の話である（[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 決めたこと 18、[ADR-0043](0043-lamp-lights-on-owner-waits.md) 事実 12）。雛形のほかのファイルで手当てが要ったのは 3 件で、2 件は中身の移し替え、1 件は 1 行の手順である（事実 5）。
   細部: 入口でこの範囲を聞いたとき、雛形のほかのファイルで手当てが要ったのは 1 件だと伝えた。正しくは 3 件である（事実 5）。出口で訂正を伝え、オーナーは範囲をそのままにすると確かめた。
   捨てた案: `host-install` が置いたものも追従させる（上書きしない決まりを改めることになり、設計が大きくなる）。`.gitignore` のような行を届ける口も入れる（行を届けるだけで済んだのは 1 件で、1 行の手順で済んでいる）。仕組みを作らず、置き換える手順と気づく口だけにする（置き換えが毎回オーナーの手になる）。

2. **2026-11-22 入口を 2 つに分ける（Chief Engineer が決めた）。** セルの repo の `bin/bootstrap` は、薄い入口にする。今の `bin/bootstrap` の中身は「本体」と呼び、image の中に置く。薄い入口は、打たれるたびに image から本体を取り出して、引数をそのまま渡して走らせる。
   理由: 本体が image と同じバージョンになるので、Andon を上げると入口の動きも一緒に変わる。本体が置く mod の名前と、image の中の mod が、いつも同じバージョンで揃う（事実 7 のずれが、どちらの向きにも起きなくなる）。`compose.yaml` と同じ道に乗るので、配り方が増えない（前提 3）。セルの repo に残るのは、image を焼いて本体を取り出すだけのコードになり、[design.md](../design.md) §2.3 の「セルの repo にパッケージのファイルは無い」に近づく（事実 8）。
   捨てた案:
   - **写しのまま、止めて置き換えさせる。** 入口は今の 1 本のままにし、雛形が変わったら番号を上げ、番号の合わない入口を止めて、オーナーが置き換える。小さく作れる。ただし、入口の動きが変わるたびに、オーナーが全部のセルで置き換えることになる。直近では 1 つのバージョンに 5 件の変更が入った（事実 3）。番号を上げ忘れた変更は、今と同じく黙って届かない。
   - **セルの中の boot が、`bin/bootstrap` を雛形と同じに保つ。** オーナーの手が要らない。ただし、入口が変わるたびに、追跡しているオーナーのファイルを機械が書き換え、セルの repo に commit していない差分が出る。手を入れた入口を潰さないために、これまでの雛形の全部のバージョンの控えを持ち、入口を変えるたびに足すことになる。焼き直さずに当てたセルでは、入口だけが image より新しくなる（事実 7・13）。
   - **入口をホストの `andon`（Go）に移す。** bash の 418 行と、78 か所で確かめている検査を書き直す。ホストに `andon` が要るようになり、前提 4 と合わない。ホストの `andon` も手で取り出し直すもので、追従しない問題が場所を変えて残る（事実 17）。

3. **2026-11-22 薄い入口の仕事を 6 つに限り、本体との約束を 3 つにする（Chief Engineer が決めた）。** 薄い入口がするのは、セルの repo の根へ移る、`andon.toml` の slug と `Dockerfile` の `FROM` を読んで確かめる、docker に繋がるかを見る、image が無ければ焼く、本体を取り出す、本体を走らせる、の 6 つである。本体との約束は、作業フォルダがセルの repo の根であること、引数をそのまま渡すこと、環境変数 `ANDON_ENTRY` に入口の形の番号（初めは `1`）を入れること、の 3 つである。
   理由: 薄い入口は追従しないので、変える理由を残さない。本体が要る値（slug、`FROM`、番号）は、本体が自分で読み直す。入口から値を渡すと、渡す値を足すたびに入口を変えることになる。
   細部:
   - slug と `FROM` の読み方と、埋まっていないときの止まり方は、今の入口と同じにする。image の名前は今と同じ `andon-<slug>:local` である。
   - uid の確かめ、`FROM` と手元の image の照らし合わせ、`compose.yaml` の取り出し、sub-command の全部は、本体に置く。
   - 本体は、セルの repo の根を、自分のファイルの場所からでなく、作業フォルダから決める。今の入口は自分の場所の 1 つ上を根にしているので、ここは書き換える。help は、本体が自分のファイルの先頭の注記を出す。案内の文に出す名前は、今までどおり `bin/bootstrap` である。
   - 本体は、`ANDON_ENTRY` が無いときは止まり、入口から打つよう言う。
   - 薄い入口は、環境変数を消さずに本体へ引き継ぐ。本体は、`STREAMDECK_PLUGINS`、`HERDR_APP_HOME`、`ANDON_HOST_BIN` などを、今の入口と同じに読む（事実 18）。

4. **2026-11-22 本体は打つたびに取り出し、前に取り出したものを走らせない（Chief Engineer が決めた）。** 薄い入口は、打たれるたびに `docker create` と `docker cp` で、image の `/opt/andon/host/bootstrap` をセルの repo の `.andon/` に取り出す。プロセスごとに違う一時の名前に取り出してから、`mv` で `.andon/bootstrap` に置き換え、それを走らせる。
   理由: 本体がいつも手元の image と同じになる。前に取り出したものを走らせる形にすると、古いかを見る仕組みが要り、セルの中から書ける場所にあるファイルをホストで走らせ続けることになる（事実 10）。毎回取り出すと、`.andon/bootstrap` がセルの中で書き換えられても、次に打つときに image のものへ戻る。`mv` で置き換えるのは、長く走っている本体（`logs`、`herdr`）が読んでいるファイルを、別の端末で打った入口が書き換えないためである。一時の名前をプロセスごとに変えるのは、同時に打った 2 つが同じ一時のファイルに書かないためである。
   細部: 今の `compose.yaml` の取り出しも一時の名前が 1 つなので、本体へ移すときに同じ形に直す。
   捨てた案: image の ID を控え、変わったときだけ取り出す（docker の呼び出しが打つたびに減るが、上の理由で取らない）。ファイルに置かず、取り出した中身をそのまま bash に渡す（何が走ったかを後から読めない）。

5. **2026-11-22 image が無いか、image に本体が無いときは、薄い入口が 1 回焼いてから見直す（Chief Engineer が決めた）。** 焼くのは今と同じ `docker build -t andon-<slug>:local <セルの repo>` である。焼けないか、焼いたあとも本体が無ければ止まり、`Dockerfile` の `FROM` を本体のあるバージョン以上に上げる手と、決めたこと 6 の口を言う。
   理由: 初めて打つセルは image が無い。前に作ったセルで、image を上げる前に入口だけを置き換えた場合も、`FROM` がレジストリのバージョンに上げてあれば、焼き直すだけで先へ進める。
   細部:
   - 焼く前に、何のために焼くかを 1 行出す。
   - どの sub-command でも同じに動く。image の無いセルで `status` や `down` を打っても焼くが、初めて打つのはふつう起動なので、sub-command ごとの分けは入口に持たせない。help だけは焼かない（決めたこと 7）。
   - `FROM` が手元だけの tag（`andon:local-…`）で、その image が手元に無いときは、焼けずに止まる。今の入口は、この状態からでも `rebuild --from` で焼き直せる（事実 16）。薄い入口では、決めたこと 6 の口で焼き直す。

6. **2026-11-22 image を見ずに、名指しした本体で動かす口を 1 つ持つ（Chief Engineer が決めた）。** 環境変数 `ANDON_ENTRY_BODY` に本体のファイルのパスが入っているとき、薄い入口は、image を見ず、焼かず、取り出さずに、そのファイルを本体として走らせる。走らせる前に、image の本体でなくそのファイルで動いていることを、標準エラーに 1 行出す。
   理由: 入口を分けると、image を焼き直すコードが image の中から来る。壊れた本体を焼いてしまうと、`rebuild` を含むどの sub-command も壊れた本体で動き、入口からは直せなくなる。今の入口は、同じ誤りを、もう一度 `rebuild --from` を打てば直せる（事実 16）。Andon を開発するセルは、検査で本物の docker を通せないまま、マージ前の branch を `rebuild --from` で焼く。image に頼らない道を 1 つ残す。
   細部:
   - 直し方は、`ANDON_ENTRY_BODY=<直した checkout>/package/host/bootstrap bin/bootstrap rebuild --from <直した checkout>` である。[operate.md](../operate.md) の 3 に、壊れた本体を焼いたときと、image が 1 つも無いときの戻し方として書く。
   - 検査の偽の image の根（`ANDON_IMAGE_ROOT`）があるときも、同じ形で動く。薄い入口は docker を見ず、その根の中の `opt/andon/host/bootstrap` をそのまま走らせる。`--dry-run` のときも、そうでないときも同じである。
   捨てた案: 手で `docker build` を打つ手順を書くだけにする（`rebuild --from` は、base の image を焼いて `FROM` を書き換える手を含むので、手で打つと長い）。

7. **2026-11-22 `--dry-run` は docker を打たず、help は image を焼かない（Chief Engineer が決めた）。** `--dry-run` のとき、薄い入口は何も書かず、docker も打たず、前に取り出した `.andon/bootstrap` も走らせない。名指しの本体か偽の image の根があれば（決めたこと 6）、その本体を `--dry-run` で走らせる。無ければ、焼く、取り出す、本体を走らせる、の 3 つを打つものとして出して終わる。help は、ほかの sub-command と同じ道で本体を取り出し、本体の先頭の注記を出す。image が無い、image に本体が無い、docker に繋がらない、のどれかのときは、焼かずに、入口が短い案内を出す。
   理由: `--dry-run` は「docker を打たずに、打つものを出す」と約束している（[operate.md](../operate.md) の 3）。前に取り出したファイルを走らせると、決めたこと 4 の理由と食い違う。確かめるつもりで打った `--dry-run` が、セルの中から書けるファイルをホストで走らせることになる。help のために image を焼くと、数分待たせる。
   引き受けること: 本物のセルで `--dry-run` を打つと、出るのは 3 行だけになる。今は、起動までに打つものが全部出る。全部を見たいときは、オーナーが `ANDON_ENTRY_BODY` に本体のファイルを名指しして打つ。

8. **2026-11-22 `rebuild` は、image を焼いたあと入口から入り直す（Chief Engineer が決めた）。** 本体は、`rebuild` で image を焼いたあと、薄い入口を、help に出さない sub-command `_recreate` で打ち直す。薄い入口は新しい image から本体を取り出し、新しい本体が `compose.yaml` を取り出して `up -d --force-recreate` を打つ。
   理由: 本体と `compose.yaml` は組である。本体は `compose.yaml` が要る変数を渡すので、古い本体が新しい `compose.yaml` で作り直すと、足りない変数で止まることがある。
   細部:
   - `rebuild --from <checkout>` も同じ順にする。`_recreate` は、uid の確かめをして、`compose.yaml` を取り出し、作り直す。
   - 名指しの本体で打った `rebuild`（決めたこと 6）は、入り直すときに名指しを外し、焼いた image の本体で作り直す。
   - 新しい本体が壊れていて作り直しで止まったとき、動いているコンテナは前の image のままである。直し方は決めたこと 6 の細部である。

9. **2026-11-22 本体は `package/host/bootstrap` に置く。gate のレベルは B から C になる（オーナーが決めた）。** image の中では `/opt/andon/host/bootstrap` になる。`init/bin/bootstrap` は薄い入口になり、レベルは B のままである。
   理由: ホストで動くものは `package/host/` にあり、その変更はオーナーがマージする（事実 11）。今は `init/bin/bootstrap` だけが、ホストで動くのにレベル B である。この文書のあとは、本体の変更が、上げた全部のセルのホストで動くようになる。当たる範囲が広がるので、ほかのホストのコードと同じ線に揃える。
   捨てた案: `package/lib/andon/` に置いてレベル B のままにする（オーナーのマージは増えないが、ホストで動くコードが、セルの中の command の資材と同じ場所と線に入る）。
   細部: `host-install` は `host/` の中身を名前で写すので、本体がホストの置き場に写ることはない。Andon を開発するセルで本体の変更をレベル B に下げるかは、オーナーがセルの条件表の上書きで決められる（[ADR-0041](0041-delegation-cap-out-of-command.md) 決めたこと 11 と同じ口）。

10. **2026-11-22 前に作ったセルは、上げたときに古い入口が止まって案内を出し、オーナーが 1 行を 1 回打つ（オーナーが決めた）。** `package/compose.yaml` に、`ANDON_ENTRY` が渡されていないと止まる 1 行を足す。古い入口は `ANDON_ENTRY` を渡さないので、新しい image に上げたあとは、`docker compose` を打つところで止まり、置き換え方の案内が出る。オーナーは、セルの repo で次の 1 行を打って入口を置き換え、`bin/bootstrap rebuild` を打ち直す。

    ```bash
    docker run --rm andon-<slug>:local andon init --print bin/bootstrap > bin/bootstrap.new \
      && install -m 755 bin/bootstrap.new bin/bootstrap && rm bin/bootstrap.new
    ```

    理由: 黙って古いまま動くことが、いちばんの害である（前提 1）。止まるのは、オーナーが上げる手を打っているその端末で、直し方が一緒に出る。`bin/` はオーナーだけが書くという線（事実 9）を変えずに済む。置き換えはセルごとに 1 回で、そのあとの入口の変更は本体に入るので、手は要らない。
    細部:
    - 古い入口で `rebuild` か `rebuild --from` を打つと、image は新しく焼かれ、作り直すところで止まる。これまでに出した 13 のバージョンの入口は、どれも、`docker compose` の初めの呼び出しの失敗でそのまま終わる。起動を待つ 5 分の繰り返しには入らない。`docker compose` は設定を読むところで止まるので、動いているコンテナは前の image のまま動き続ける見込みである。Compose の文書はそこまで書いていないので、CI の本物の docker で確かめる（事実 12、受け入れ条件 10）。
    - 止まったあとは、古い入口の `up`、`down`、`status`、`logs`、`sh`、`herdr` も同じ案内で止まる。`andon-up` は、そのセルを失敗として出し、記録に案内が残る。
    - **古い入口の `host-install` は止まらない。** `docker compose` を打たないからである（事実 15）。新しい image に上げたあと、入口を置き換えずに `host-install` を打つと、今と同じく、古い入口が知っている置き先だけが黙って置かれる。オーナーは、その前の `rebuild` で案内を見ている。CHANGELOG の「上げるときにすること」と [operate.md](../operate.md) の 4 に、入口を置き換えてから `host-install` を打つよう書く。
    - 案内に出る 1 行は、そのまま貼って動く形にする。`compose.yaml` は全部のセルで同じなので、image の名前は、古い入口も渡している変数 `ANDON_IMAGE` を文の中で開いて出す。Compose の文書は、誤りの文の中に変数を書く例を載せているが、文がどう出るかは書いていない。実装の課題は、CI で出た文を初めに確かめる。開かれないと分かったら、案内の文は image の名前を言葉で言い、[operate.md](../operate.md) の節の名前を指す形にする。
    - 置き換える前の入口に手を入れていたときは、置き換えでその行が消える。CHANGELOG の「上げるときにすること」に、先に `git diff` で見るよう書く。セルごとの違いを書く場所は、今までどおり `.env`、`compose.override.yaml`、`local/` である。
    - `FROM` がレジストリのバージョンのセルは、順を逆にして、入口を先に置き換えてもよい。`docker run` に渡す image を、上げた先の `FROM` の image にする。そのあとの `bin/bootstrap rebuild` は、決めたこと 5 の焼き直しを通って進む。`rebuild --from` で上げるセルは、古い入口で `rebuild --from` を先に打つ。
    - 分けたあとに作ったセルを、分ける前のバージョンへ戻すときは、セルの repo の履歴に古い入口が無い。古い image の中で `andon init` を打って、古い入口を取り出す。この手を [operate.md](../operate.md) の「GHCR のバージョンへ戻す」に書く。前に作ったセルは、セルの repo の git で `bin/bootstrap` を前のものへ戻せる。
    捨てた案:
    - **セルが自分で書き換える。** 止める行を入れず、セルの中の boot が、手を入れていない `bin/bootstrap` だけを薄い入口に書き換える。オーナーの手は要らない。ただし、オーナーのファイルを機械が書く初めての例になり、ADR-0005 の決めたこと 2 を一部覆す。手を入れていないことを見分けるために、これまでの雛形 15 件の控えを持つ。1 回の移し替えのための仕組みが、そのあとも boot に残る。
    - **止めずに、知らせるだけにする。** boot が起動の記録に警告を 1 行出す。古い入口は動き続ける。オーナーが起動の記録を読まなければ、今と同じく黙って古いままになる。

11. **2026-11-22 雛形の 1 ファイルを出す口を足す（Chief Engineer が決めた）。** `andon init --print <雛形の中のパス>` は、埋め込んだ雛形のそのファイルを標準出力に出す。セルの repo は作らず、何も書かない。名前が `.tmpl` のファイルと、無いパスは断る。
    理由: 薄い入口の正は `andon` に埋め込んだ `init/bin/bootstrap` の 1 つである。image に写しをもう 1 つ置くと、焼き直さずに当てる口（事実 13）にも同じ写しが要り、2 つがずれる。
    捨てた案: `andon init --dir` を、今あるセルの repo に上書きできるようにする（オーナーのファイルを全部潰しうる）。

12. **2026-11-22 薄い入口は変えないことを原則にし、変えるときは番号を上げる（Chief Engineer が決めた）。** 薄い入口の動きを変えるときは、`ANDON_ENTRY` の番号を上げる。本体は、渡された番号が自分の要る番号より小さいと止まり、決めたこと 10 と同じ 1 行を出す。番号を上げたバージョンの CHANGELOG の「上げるときにすること」に、置き換える手を書く。
    理由: 薄い入口は追従しないので、変えると、また全部のセルでオーナーの手が要る。変えたことが黙って届かないままにならないよう、番号で止める。
    細部: 検査は、薄い入口の中身の控え（ハッシュ）と番号を並べて持つ。中身が変わって控えが前のままなら落ち、番号を上げるか、動きが変わらないなら控えだけを直すよう言う。`AGENTS.md` の表に、この決まりと検査の名前の対を 1 行足す。

13. **2026-11-22 Stream Deck の plugin の置き先は本体へ移し、「雛形で置き換える」手を替える（Chief Engineer が決めた）。** [ADR-0045](0045-stream-deck-lamp.md) の決めたこと 12 は、`host-install` が plugin を置くことを決め、細部に「前に作ったセルでは、`bin/bootstrap` を今の雛形で置き換えてから打つ」と書いた。plugin の置き先は、もう `init/bin/bootstrap` に入っている（事実 18）。この文書の実装は、その置き先ごと本体へ移す。雛形で置き換える手は、この文書の決めたこと 10 の手に替える。
    理由: ADR-0045 の決めたこと 12 は Chief Engineer が決めたもので、雛形で置き換える手は、追従の仕組みが無い間のつなぎだった。
    細部:
    - 関数 `install_streamdeck` と、`host-install` の初めの `STREAMDECK_PLUGINS` の確かめは、動きを変えずに本体へ移す。薄い入口には残さない。
    - plugin の無い image で置かずに知らせる枝も、そのまま移す。本体と `host/` は同じ image から取り出すので、ふつうの道ではこの枝に入らなくなる。名指しの本体（決めたこと 6）を古い image と組み合わせたときに入る。
    - [operate.md](../operate.md) の 4 の「前に作ったセル」の手順は、セルの中の `andon` で雛形を作る。古い入口が止まったとき、動いているコンテナは前の image のままで、セルの中の `andon` も前のものである。この手順で出るのは古い入口なので、決めたこと 10 の順では使えない。実装の課題は、この段と CHANGELOG の「上げるときにすること」の行を、新しい image をホストで `docker run` する決めたこと 10 の 1 行に書き換える。
    - `tests/andon-init.sh` の Stream Deck の検査は、薄い入口から打って、どれも通す。`bin/bootstrap` が雛形と同じであることを見る検査は、薄い入口を見る形でそのまま残す。
    - ADR-0045 の残りの実装が `init/bin/bootstrap` に触れるときは、この文書の実装と並べて走らせない。

14. **2026-11-22 本体は、焼き直さずに当てる口では変わらないものに入れる（Chief Engineer が決めた）。** `image/apply-main.sh apply` のあとも、ホストの入口が取り出す本体は、image を焼いたときのままである。[operate.md](../operate.md) の「当てないもの」に、本体を足す。
    理由: 本体は `compose.yaml` と `host/` と同じ image から取り出すので、3 つがいつも同じバージョンで揃う（事実 13）。

15. **2026-11-25 決めたこと 10 の案内の出方を、CI の本物の docker で確かめた結果に合わせる（実装の担当が確かめた）。** 決めたこと 10 の細部が「CI で確かめる」とした 2 点は、次のとおりだった。
    - 案内の文の中の `${ANDON_IMAGE}` は開かれ、image の名前が入って出る。ただし Compose の補間の文は改行を持てない。改行を入れると、`ANDON_ENTRY=1` を渡しても設定を読めない。置き換えの 1 行は、案内の行の最後の「: 」のあとに出る。オーナーはその部分を 1 行として打つ。
    - `docker compose exec` は、補間の誤りがあっても止まらない。`up -d`、`up -d --force-recreate`、`down`、`ps`、`logs` は止まる。古い入口の `sh` と `herdr` は、`exec` の前に `ps -q` と `up -d` を打ち、そこで止まるので、古い入口から見た動きは決めたこと 10 の細部のとおりである。受け入れ条件 10 の検査は、`exec` の代わりに `ps -q dev` を見る。

## やらないこと（範囲の外。理由）

- **古い入口の `host-install` を止めること。** 新しい image に上げたあとも、古い入口の `host-install` は黙って動く（決めたこと 10）。止めるには、`host/` の中の並びを古い入口が読めない形に変えることになり、出る誤りが案内にならない。
- **`host-install` が置いたものの追従。** mod、`andon-up`、herdr の設定、ssh config は、今までどおり今あるファイルを上書きしない（決めたこと 1）。
- **雛形のほかのファイルの追従。** `.gitignore`、`andon.toml`、`Dockerfile` などは、今までどおり作ったあと追従しない（決めたこと 1）。
- **薄い入口の gate のレベルを変えること。** `init/**` はレベル B のままにする。薄い入口は、変えないことを原則にする（決めたこと 12）。
- **セルの中から `bin/bootstrap` と `.andon/` に書けることを塞ぐこと。** 今の入口と `compose.yaml` と同じ形で、新しい穴ではない（事実 10）。塞ぐには、セルの repo の載せ方を変えることになる。
- **ホストの `andon` を追従させること。** `andon init` のために取り出したホストの `andon` は、今までどおり手で取り出し直す（事実 17）。入口を置き換える 1 行は、ホストの `andon` を使わない。
- **入口の sub-command を足す・変えること。** 本体の中身は、今の `bin/bootstrap` の動きのまま移す。変えるのは、決めたこと 3 の根の決め方と help の読み先、決めたこと 4 の一時の名前、決めたこと 8 の `rebuild` の順だけである。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 4 つにする。入口と本体の動きは `bash tests/andon-init.sh` で見る。今と同じに、`package/` を偽の image の根に写し、薄い入口から動かす。docker の呼び出しの順は、同じ検査の偽の docker の記録で見る。`andon init --print` は `go test ./internal/plantinit/`。本物の docker が要るものは、CI の `tests/image.sh` で見る。前に作ったセルの移し替えは、オーナーが手元のセルで見る。

| # | 条件 | 証拠（command と見る出力） |
|---|---|---|
| 1 | `andon init x` が置く `bin/bootstrap` は薄い入口で、`host-install`、`rebuild`、mod の名前を含まない。偽の image の根の `opt/andon/host/bootstrap` が本体である。今の検査が見ていることは、薄い入口から打って、どれも通る。`bin/bootstrap` を書き換えて help を見ている 2 つの検査は、本体を書き換える形に直す。docker の無い PATH で、偽の image の根だけを渡して `host-install` と `--help` と知らない sub-command が今と同じに動く。`STREAMDECK_PLUGINS` を渡した `host-install` は、薄い入口から打っても、今と同じに plugin を置く。薄い入口は `install_streamdeck` を含まない | `bash tests/andon-init.sh`（`all passed`。`pass` の呼び出しの数が、直す前の 78 か所より減っていない。直す前は merge-base の木で数える） |
| 2 | 偽の image の根を渡さず、偽の docker で `bin/bootstrap up` を打つと、`create`、`cp`（`/opt/andon/host/bootstrap` から `.andon/` の一時の名前へ）、`rm` がこの順に呼ばれ、`.andon/bootstrap` が置かれる。もう一度打つと、もう一度取り出す。一時の名前のファイルは残らない | `bash tests/andon-init.sh`（偽の docker の記録の行） |
| 3 | `.andon/bootstrap` を別の中身に書き換えてから打つと、image の本体で動き、ファイルは image の本体に戻る。`--dry-run` で打つと、書き換えた中身は走らず、打つものの 3 行が出る | `bash tests/andon-init.sh` |
| 4 | 本体は `ANDON_ENTRY` が `1` で起動される。`ANDON_ENTRY` を渡さずに本体を `bash` で直接打つと、0 でない終了で、入口から打つよう言う。本体が要る番号を 2 にした写しを薄い入口から打つと、0 でない終了で、置き換えの 1 行が出る。本体を、セルの repo の外に置いて、セルの repo を作業フォルダにして打つと、そのセルの `andon.toml` を読む | `bash tests/andon-init.sh` |
| 5 | 偽の docker で image が無いとき、`build` が 1 回呼ばれてから取り出す。image はあるが本体が無いとき、`build` が 1 回呼ばれ、それでも無ければ 0 でない終了で、`FROM` を上げる手と `ANDON_ENTRY_BODY` を言う。`build` は 2 回呼ばれない。`build` が失敗したときも同じ案内で止まる | `bash tests/andon-init.sh` |
| 6 | `ANDON_ENTRY_BODY` に本体のファイルを渡すと、docker の `create`、`cp`、`build` は呼ばれず、そのファイルが走り、標準エラーに名指しで動いていることの 1 行が出る。image が無くても `rebuild --from` が base の image を焼くところまで進む。無いファイルを渡すと、0 でない終了で理由を言う | `bash tests/andon-init.sh` |
| 7 | `--dry-run` は、docker を呼ばず、セルの repo に何も書かない。偽の image の根も名指しも無いとき、焼く、取り出す、走らせる、の 3 行を出して 0 で終わる。image が無いときの `--help` は、`build` を呼ばずに案内を出して 0 で終わる | `bash tests/andon-init.sh` |
| 8 | 偽の docker で `rebuild` を打つと、`build` のあとに本体がもう一度取り出され、そのあとで `compose … up -d --force-recreate` が呼ばれる。`rebuild --from` でも同じ順である。名指しの本体で打っても、作り直しは焼いた image の本体で行われる。`_recreate` は help に出ない | `bash tests/andon-init.sh`（偽の docker の記録の順） |
| 9 | `andon init --print bin/bootstrap` の出力は、`andon init x` が置く `bin/bootstrap` と同じで、作業フォルダに何も書かない。`--print Dockerfile.tmpl` と、無いパスは、0 でない終了で理由を言う | `go test ./internal/plantinit/` |
| 10 | 本物の docker で、`ANDON_IMAGE` と `ANDON_SLUG` だけを渡して（古い入口が渡すものと同じ）`docker compose …` の `up -d`、`up -d --force-recreate`、`down`、`ps`、`logs`、`exec` を打つと、どれも 0 でない終了で、案内の文が出る。出た文の中の 1 行は、image の名前が入っていて、そのまま打つと薄い入口が出る。動いているコンテナは、その前後で ID が変わらず、動いたままである。`ANDON_ENTRY=1` を足すと通る | CI の `tests/image.sh`（`ok` の行。手元に docker が無いと飛ばされるので、CI の記録で見る） |
| 11 | 本物の docker で、`andon init` から `bin/bootstrap up` までが薄い入口から通り、`.andon/bootstrap` が image の `/opt/andon/host/bootstrap` と同じである。`docker run --rm <image> andon init --print bin/bootstrap` の出力が、セルの `bin/bootstrap` と同じである | CI の `tests/image.sh` |
| 12 | 薄い入口の中身を 1 字変えて、控えを直さないと、検査が落ち、番号を上げるか控えを直すよう言う | `bash tests/andon-init.sh`（壊した写しで落ちることを、同じ検査の中で確かめる） |
| 13 | 前に作ったセルで、古い入口で新しいバージョンへ `rebuild`（Andon を開発しているセルでは `rebuild --from`）を打つと、案内が出て止まり、セルは動いたままである。案内の 1 行を打って `rebuild` を打ち直すと、作り直される。そのあと `bin/bootstrap host-install` を打つと、mod `waiting` が置かれる | `manual`（オーナーが、Andon を開発しているセルのホストで見る） |

[operate.md](../operate.md) の 2・3・4（「前に作ったセル」の段を含む）と「当てないもの」と「GHCR のバージョンへ戻す」、[design.md](../design.md) §2.2・§2.3・§3 の表、README、`package/compose.yaml` の注記、CHANGELOG の「上げるときにすること」は、検査では見ない。レビューが差分で見る。

## 価値の仮説

1. **誰の何が変わるか。** オーナーが Andon を上げたあと、入口に入った新しい動きが、前に作ったセルでも、手を足さずに効くようになる。`host-install` の置き先が増えても、入口を置き換える手が要らない。
2. **見る信号。** オーナーの目視。あわせて、repo で数えられるものを 2 つ見る。入口を置き換えたバージョンのあとの CHANGELOG の「上げるときにすること」に、入口を置き換える行が無いこと。`init/bin/bootstrap` の変更の数（`git log --oneline -- init/bin/bootstrap`）。
3. **見る日。** 2026-12-08。
4. **誰が要るか。** オーナー。セルごとに 1 回、入口を置き換える。そのあと、少なくとも 1 回、Andon のバージョンを上げる。
5. **効いたとどう分かるか。** 入口を置き換えたあとに本体へ入った変更が、前に作ったセルで、`rebuild` のあと何も足さずに効いた。たとえば、`host-install` に足された置き先が、入口を置き換えずに置かれた。入口を手で置き換えたのは、セルごとに 1 回だけである。`init/bin/bootstrap` の変更は、入口を分けた 1 件のあと 0 件である。
6. **何があれば止めるか。** 見る日までに、薄い入口の番号を上げることになった。または、本体を取り出せずに入口が動かないことが、決めたこと 5 の止まり方のほかで起きた。または、決めたこと 6 の口を、開発の試しのほかで使うことになった。どのときも、原因を調べ、薄い入口に残す仕事を決め直す。入口が動かないときは、セルの repo の git で `bin/bootstrap` を前のものへ戻し、`compose.yaml` の止める行を外したバージョンへ上げる。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **入口を分ける。** 決めたこと 2〜14 を 1 つの課題で作る。`init/bin/bootstrap`、`package/host/bootstrap`、`package/compose.yaml`、`internal/plantinit/`、`tests/andon-init.sh`、`tests/image.sh`、`AGENTS.md`、[operate.md](../operate.md)、[design.md](../design.md)、README、CHANGELOG。受け入れ条件 1〜12。実装の課題が初めに確かめるのは、受け入れ条件 10 の、案内の文がどう出るかである（決めたこと 10）。`package/host/**` と `package/compose.yaml` に触れるのでレベル C で、オーナーがマージする。ADR-0045 の残りの実装が `init/bin/bootstrap` に触れるときは、並べて走らせない（決めたこと 13）。
2. **オーナーの手: 前に作ったセルの入口を置き換える。** 1 の入ったバージョンへ上げるとき。手順は CHANGELOG の「上げるときにすること」と [operate.md](../operate.md)。Andon を開発しているセルで、受け入れ条件 13 を見る。案内が出ない、セルが止まる、作り直せない、のどれかがあれば、Chief of Staff に言う。
3. **観察。** 価値の仮説の 2 と 5 を、見る日に見る。

## 開いている問い（残った問い。決断の種類つき）

無し。

## 出典

- repo の中: [design.md](../design.md) §2.2・§2.3・§3、[operate.md](../operate.md) の 2・3・4、[ADR-0005](0005-cell-repo.md)、[ADR-0027](0027-apply-package-without-rebuild.md)、[ADR-0041](0041-delegation-cap-out-of-command.md)、[ADR-0042](0042-owner-waits-do-not-stop-the-line.md)、[ADR-0043](0043-lamp-lights-on-owner-waits.md)、[ADR-0045](0045-stream-deck-lamp.md)、CHANGELOG の v0.0.13。
- コード: `init/bin/bootstrap`、`internal/plantinit/plantinit.go`、`internal/boot/`、`package/compose.yaml`、`package/gate.json`、`package/settings.json`、`package/host/andon-up`、`image/Dockerfile`、`image/apply-main.sh`、`tests/andon-init.sh`、`tests/image.sh`。
- 外の文書: Compose の変数の補間 https://docs.docker.com/reference/compose-file/interpolation/ （2026-11-22 に読んだ）。
- オーナーの発言: 2026-11-22（初めの 1 行。言い直しと前提 4 つへの同意。範囲は bin/bootstrap の追従と前のセルの移し方まで。前に作ったセルは止めて案内し、手で 1 行。本体は `package/host/` に置いてレベル C。最後の確かめで「決めた」）。
