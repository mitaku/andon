# ADR-0045: 行燈を Stream Deck のキーに出す（仕様・設計。2026-11-22）

- 状態: **決定（2026-11-22。オーナーが決めた）**。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: Herdr App の左の列の行燈と同じ灯りを、オーナーの机の上の Elgato Stream Deck のキー 1 つに出す。キーに出す中身、押したときの動き、灯りの値を App の外から読む口、Stream Deck の plugin の作りと配り方を決める。あわせて、herdr-core の API から、動いている役の端末の数を 2 つ目のキーに出す。オーナーの手元に新しい部品を 1 つ足す決定と、オーナーに見える挙動を足す決定を含む。
- 語は [design.md](../design.md) §0.1。灯りの状態と、それを決める場所は [ADR-0043](0043-lamp-lights-on-owner-waits.md) が決めた。この文書は、その値を読んで出す側だけを決める。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、キー 1 つに色と件数を出すことと、押して待っているものへ飛ぶことにする | この ADR | 生きている |
| 2 | 灯りの元は Herdr App に置き、App を閉じている間は「分からない」を出す | この ADR | まだ実装していない |
| 3 | Stream Deck の plugin を Andon の repo に作り、Marketplace には出さない | この ADR | まだ実装していない |
| 4 | plugin は、App が書く `shared.json` の `inbox.lamp` を 2 秒ごとに読む | この ADR | まだ実装していない |
| 5 | `inbox.lamp` に `version` と `at` を足し、mod `inbox` は描画を待たずに置き直す | この ADR | まだ実装していない |
| 6 | 「分からない」は plugin の表示だけの状態にし、灯りの状態は 3 つのままにする | この ADR | まだ実装していない |
| 7 | キーには、行燈と同じ色と、待っているものの合計と、内訳を 2 行まで出す | この ADR | まだ実装していない |
| 8 | 押すと mod `inbox` に待っているものを開かせ、Herdr App を前に出す。App は起動しない | この ADR | まだ実装していない |
| 9 | mod `inbox` に、待っているものを順に開く command `inbox.call` を足す | この ADR | まだ実装していない |
| 10 | plugin は npm の依存を持たず、Stream Deck の WebSocket の約束を直接話す | この ADR | まだ実装していない |
| 11 | plugin の名前は `andon.lamp` にし、`package/host/streamdeck/` に置く | この ADR | まだ実装していない |
| 12 | `host-install` は、置き場を指定されたときだけ plugin を置く | この ADR | まだ実装していない |
| 13 | 設定の画面は作らず、App の置き場と実行ファイルは環境変数で替える | この ADR | まだ実装していない |
| 14 | `inbox.lamp` の形は Andon の約束に足さず、mod と plugin をつないだ検査で守る | この ADR | まだ実装していない |
| 15 | herdr-core の API から、動いている役の端末の数を出す 2 つ目のキーを足す | この ADR | まだ実装していない |

## 問題（オーナーの言葉で。言い直しの段落）

オーナー（2026-11-21）: 「StreamDeckにもランプの状態を出したりできるとよさそう」。前提として、オーナーの介在は扱っているものの性質上まだ多いが、もっと減らして、なおかつ効率的にできそうだと言った。

言い直し（オーナーが同意 2026-11-22）。オーナーは、行燈が灯ったことを、Herdr App を見ていないときにも机の上で気づきたい。行燈は [ADR-0043](0043-lamp-lights-on-owner-waits.md) で、返事が来ただけでは灯らず、オーナーが呼ばれているときだけ灯るものになった。これを Stream Deck のキーに出せば、別の窓で作業していても、灯ったら手を止められ、消えていれば App を見に行かなくて済む。目的は、オーナーが App を見に行く回数を減らし、呼ばれてから動くまでの時間を短くすることである。

モデル: 調べなかった（灯りの値を読んで出す部品を足すだけである。項目の状態の定義、状態を動かす command の順と断り方、自動で動く部品の順、役の間のやりとりは変えない）。

## 読んで分かった事実（2026-11-22。main `ec2e934`、herdr-app `317e673`、Elgato の文書）

1. **灯りを決める場所と値。** 灯りは mod `inbox` が決め、`shared("inbox.lantern")` に `"red"`・`"amber"`・`null` を置く。あわせて `shared("inbox.lamp")` に、内訳 `{light, failed, blocked, merge, decision, reply, unread, unreadable}` を置く（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 2・10）。同じ文書は、Stream Deck の題がこの値を読む画面を足せば済むと書き、外へ出す形を約束にするかはこの題が決めると書いている。mod は、描画のたびと、30 秒のタイマーの回の初めに、灯りを決めて値を置く。タイマーの回は、そのあとでセルの `andon waiting` を呼ぶ。返った結果は共有の値に置かれるが、それを灯りに入れるのは次の描画である。描画が止まっていれば、次のタイマーの回まで灯りに入らない。値を置くかどうかは、前の値と JSON の文字で比べて決めている。
2. **App は共有の値をファイルに書く。** Herdr App は、どの mod でも `setShared` で値を変えるたびに、App の設定フォルダの `shared.json` を書き直す。中身は、キーと値をそのまま並べた 1 つの JSON である。書き方は、別の名前のファイルに書いてから置き換える形で、続けて来た変更は最新の 1 つにまとめる。App を閉じるときは、書き終えてから終わる。同じ値をもう一度置いても、書かないし、ほかの mod も描き直さない（`src/mods.rs`・`src/persist.rs`）。App の mod の手引きは、このファイルを「Herdr が書く mod の状態」として挙げている。App を閉じたあとも、最後の値はファイルに残る。
3. **mod にできること。** mod は、手元とつないだマシンで command を動かせる（`exec`）。HTTP を送れる（`fetch`）。App の設定フォルダの下にファイルを書ける。mod の写しは、置かれたブロックがレイアウトにある間は、隠れていても生きていて、タイマーも動く。別のレイアウトに切り替えると、写しは作り直される。値を置くと、全部の mod が描き直される（App の mod の手引き）。App のタイマーの実装の注釈は、見えない窓でもタイマーを止めないと書いている。窓を最小化している間に描画が走るかは、ソースからは言えない。
4. **App を外から動かす口。** `herdr-app do <command>` は、動いている App に command を 1 つ送る。終了コードは、届けば 0、App がエラーを返せば 1、App の窓が無ければ 2 である。`<mod>.<command>` の形の command は、表示されている mod の写しにだけ届く。表示されている写しが無ければ、エラーになる。引数の無い `herdr-app` は、App が動いていれば、その窓を前に出すよう頼み、返事を 3 秒まで待って終わる。動いていなければ、自分が App になる。App の起動の途中にもう 1 つ起動すると、窓が 2 つになる。窓を前に出すだけの command は、`do` で送れる一覧に無い（`src/control.rs`・`src/main.rs`・App の command の一覧）。Windows では、実行ファイルは `%LOCALAPPDATA%\Programs\herdr-app\herdr-app.exe` で、インストーラが利用者の PATH に足す。App の設定フォルダは、環境変数 `HERDR_APP_HOME`、無ければ `XDG_CONFIG_HOME` の下の `herdr-app`、それも無ければホームの `.config/herdr-app` である（[operate.md](../operate.md)）。
5. **mod `inbox` の「次へ」と絞り込み。** command `inbox.next` は、Needs you の行を古い順に回り、無くなれば未読の印のある役の行へ行く。オーナー待ちの件数の行へは行かない。件数の行は、押すとオーナー待ちのペインを開く。ペインの無いレイアウトでは、そのプロジェクトの Chief of Staff の端末へ行く（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 5・7）。灯りは、マシンの絞り込みと検索に関わらず、全部を数える。左の列に並ぶ行は、絞り込みと検索に従う。
6. **kanban のオーナー待ちの問いは、ほとんどがマージの依頼である。** 60 時間に開いた問い 103 件のうち、99 件が gate からのマージの依頼だった。この数に、止まった端末は入っていない（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 事実 5）。マージの依頼の答えまでの時間は、中央値が 24 分で、長いほうの 4 分の 1 は 53 分以上だった（[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 事実 3）。
7. **Stream Deck の plugin の形。** Stream Deck のアプリは Windows と macOS で動き、Linux には無い。plugin は Node.js のプログラムで、アプリが持つ Node.js で動く。アプリの 7.1 から 7.6 は、Node.js の 20・24・26 を持ち、plugin の `manifest.json` がどれで動くかを選ぶ。アプリは plugin を、ポート番号・plugin の名前・登録の語・アプリの情報の 4 つの引数で起動する。plugin は、そのポートの WebSocket につなぎ、登録の 1 通を送る。そのあとは JSON を 1 通ずつやりとりする。この約束は、Elgato の文書の「WebSocket API」にある。同じ文書は、Node.js でない実行ファイルの plugin を勧めず、公式の SDK を勧めている。
8. **公式の SDK。** npm の `@elgato/streamdeck` は 3.0.6（2026-11-20）で、入れると実行時のパッケージは 6 つになる（`@elgato/streamdeck`・`@elgato/utils`・`@elgato/schemas`・`@streamdeck/core`・`ws`・`zod`）。雛形は TypeScript で、バンドラで 1 つのファイルにまとめる。Node.js は、22 から WebSocket のクライアントを組み込みで持つ。
9. **キーに出せるもの。** plugin は、自分のキーの画像を `setImage` で替えられる。画像は、SVG を符号化して `data:` の URL にしたものを渡せて、大きさは 144x144 が勧められている。機種ごとの大きさには、アプリが合わせる。`manifest.json` で `UserTitleEnabled` を偽にすると、利用者はタイトルを重ねられない。利用者がキーに自分の画像を設定すると、plugin の画像は出ない。更新は 1 秒に 10 回までと決められていて、状態が変わったときに替えるよう勧められている。キーが見えると `willAppear`、消えると `willDisappear`、押すと `keyDown` が届く。PC がスリープから戻ると、見えているキーの全部に、`willAppear` がもう一度届く。失敗は `showAlert` で知らせるよう勧められている。アクションは、指定しなければ、複数の動きをまとめるキーにも入れられる。そこでは絵が出ない。plugin が開ける URL は、既定のブラウザで開くものだけで、アプリを前に出す口は無い。
10. **plugin の入れ方。** plugin は、名前が plugin の UUID に `.sdPlugin` を付けたフォルダである。Elgato の文書が挙げる入れ方は、配布用の 1 ファイル（`.streamDeckPlugin`）を開くことと、開発用の道具でフォルダをつなぐことである。どちらの道具も npm の `@elgato/cli` にある。Marketplace にも署名にも頼らずに入れられる。Windows の plugin の置き場が `%APPDATA%\Elgato\StreamDeck\Plugins` であることは、Elgato の文書には無く、検索で見つけた。plugin のアイコンは PNG で、256 と 512 の 2 枚が要る。キーの見本の画像とアクションのアイコンは SVG でよい。
11. **Windows は、別のプロセスが窓を前に出すことを制限している。** 前に出せないときは、タスクバーのボタンが光るだけになる（Microsoft の文書の `SetForegroundWindow`）。Stream Deck のキーから起動した `herdr-app` が App の窓を実際に前に出せるかは、オーナーの手元でしか確かめられない。
12. **Node.js は Windows で、切り離していない子のプロセスを、自分が終わるときに終わらせる。** Node.js は、子のプロセスを 1 つの組に入れ、自分が終わるとその組を丸ごと終わらせる。切り離す指定で起動した子は、組に入らない（libuv の `src/win/process.c` の注釈と、Node.js の `child_process` の文書）。plugin の子として Herdr App が起動すると、Stream Deck を終えるか plugin が落ちたときに、App が強制で終わる。Stream Deck のアプリ自身が plugin を同じような組に入れているかは、分からない。
13. **既製の plugin。** command か HTTP の結果で 3 つ以上の見た目を切り替えられる既製の plugin は、Stateful Executor（2026-02）と API Monkey（2026-07）で、どちらも個人の小さなプロジェクトである。色と件数を 1 枚に描くことと、押したときの動きを決めることはできない。
14. **Andon の手元の部品の今の形。** `package/host/` の mod は 6 つで、npm の依存は無い。外の部品は、mod `kanban-board` に写して置いた js-yaml の 1 つである。repo の npm の依存は、モデルの検査器の 1 つだけである。mod の検査は `tests/host.sh` が Node.js で動かし、偽の API に対して本物の `main.js` を動かす。Node.js のバージョンは `mise.toml` の 26 である。`bin/bootstrap host-install` は、イメージの中の `package/host/` を手元へ写し、今あるファイルを上書きしない。WSL では、App の置き場を `HERDR_APP_HOME` で渡す。セルの `bin/bootstrap` は、セルを作ったときの雛形の写しで、あとから Andon に追従しない（[operate.md](../operate.md)）。あとから雛形に足した置き先を、前に作ったセルの `bin/bootstrap` は知らない。`package/host/**` は gate のレベル C で、オーナーがマージする。
15. **ほかの題との重なり。** [ADR-0044](0044-owner-digest.md) のダイジェストは、既定で毎朝 1 回で、マージごとの通知は足さない。呼ばれたことを App の外でその場で知る道は、ほかに無い。
16. **herdr-core は、自前のクライアント向けの API を公開している。** Herdr のサイトの herdr-core のページは、同じ API が自前のクライアントに開かれていると書き、例に Stream Deck を挙げている。API は JSON-RPC で、`hc schema` が定義を出す。`hc` の 0.1.17 では口が 32 個ある。`hc connect` は、この API を標準入出力で話す。`events.subscribe` は、手元とつないだマシンの端末の今の状態を返し、そのあとの変化を順に通知する。15 秒ごとに、生きている印も送る。受け手が遅れたときは、遅れたと知らせて、今の状態を送り直す。1 つの購読に、つないだマシンの端末も入る（定義の説明と、App の `src/hc.rs` の注釈）。`hc events --json` は、これを 1 行に 1 つの JSON で出し続ける。端末ごとに、状態の語（`working`・`blocked`・`done` など）と、ラベルが入る。Andon が起動した役の端末は、ラベル `role` を持つ（セルの `hc` の 0.1.17 で確かめた）。`machine.exec` は、つないだマシンで command を動かす。`doc.get` と `doc.set` は、JSON の文書を core に置き、変わると通知する。Herdr App は、自分のレイアウトをこの文書に置いている。この API に、行燈の値は流れていない。kanban のオーナー待ちも流れていない。mod が core の文書に書く口は、App の mod の手引きに無い。mod が呼べるのは `hc` の command で、`hc` に文書を書く command は無い（`hc --help` と、herdr-core の `cli.rs` の注釈）。

確かめていないこと。Windows での `hc` の実行ファイルの名前と置き場。App のインストーラは、herdr-core を `%LOCALAPPDATA%\Programs\herdr-core\` に入れる。App の窓を最小化している間に、描画が走るか。Stream Deck が、plugin のフォルダを置き場に写しただけで読むか。Stream Deck が plugin を起動するときの作業フォルダ。macOS での plugin の置き場。どれも、実装の課題かオーナーの手元で確かめる（決めたこと 5・12・15 と受け入れ条件 14・18）。

## 前提（同意済み。番号）

オーナーが 2026-11-22 に、入口の確かめで同意した。4 は、App が起動していなくても出せるかを聞いたうえで、「基本的に非アクティブでも Herdr App は開いている」と答えた。6 は、タッチ帯に文を出せるかを聞いたうえで、範囲の問いでキーだけを選んだ。

1. 問題は、行燈が Herdr App の左の列にしか無く、別の窓で作業していると灯ったことに気づけないことである。
2. 何もしないと、気づく道は App を見に行くことだけである。毎朝のダイジェストでは、気づくまでの時間は縮まらない（事実 15）。
3. 灯す条件は新しく決めない。[ADR-0043](0043-lamp-lights-on-owner-waits.md) の消灯・琥珀・赤と内訳を、そのまま読んで出す。
4. Herdr App を開いている間だけ出れば足りる。App を閉じている間は「分からない」を出して、消灯と見分けられるようにする。
5. Stream Deck のアプリと Herdr App は、同じ PC の Windows 側で動いている。plugin は Andon の repo に入れ、オーナーが手元で 1 回入れる。Elgato の Marketplace には出さない。
6. 出す先は、どの機種にもあるふつうのキーにする。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

### 範囲と灯りの元

1. **2026-11-22 範囲は、キー 1 つに色と件数を出すことと、押して待っているものへ飛ぶことにする（オーナーが決めた）。** 押すと、Herdr App が前に出て、待っているものが開く。
   理由: キーはどの機種にもあり、色だけで呼ばれているかが分かる。押して待っているものへ飛べれば、気づいてから動くまでが 1 手になる。
   捨てた案: キーに加えて、Stream Deck + のタッチ帯に、件数と待っているものの題を出す（機種が限られる。題を出すには、mod `inbox` が題も渡すように広げる必要がある。あとから足せる）／色だけを出し、押しても何もしない（気づいたあとに App を探して切り替える手が残る）／plugin を作らず、既製の plugin で出す手順書だけを書く（事実 13）。
2. **2026-11-22 灯りの元は Herdr App に置き、App を閉じている間は「分からない」を出す（オーナーが決めた）。** plugin は灯りを自分で決めない。mod `inbox` が決めた値を読む。
   理由: 灯りを決める場所は 1 か所である（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 10）。App は、非アクティブでも基本的に開いている（前提 4）。押して飛ぶ先も App である。
   細部: オーナーは、入口でこれを決めた。出口で、herdr-core の API を直接読む案（下の捨てた案の 1 つ目）と比べて、もう一度これを選んだ。入口の問いは、その案を選択肢に入れていなかった。
   捨てた案: plugin が herdr-core の API のクライアントになり、端末の状態とオーナー待ちを自分で読んで、灯りを決める（事実 16。App を閉じていても灯る。Herdr が公開している約束だけで作れる。しかし、灯りの規則が mod `inbox` と plugin の 2 か所になる。規則には、子の端末の状態を親に畳むことや、役の端末の扱いがある。オーナーが返事を見たかの記憶は App の中にしか無く、plugin には写せない。キーの色と App の行燈が食い違う恐れがある。セルの `andon waiting` を呼ぶ回数も 2 倍になる。押して飛ぶ先は、この案でも App である）／セルに灯りを決める command を足し、plugin がセルごとに SSH で聞く（App が要らなくなる。しかし、灯りを決める式が mod とセルの 2 か所になる。plugin に、セルの一覧と SSH の設定を持たせることになる。Windows の OpenSSH は接続を使い回せないので、聞くたびに鍵の交換がかかる）。
3. **2026-11-22 Stream Deck の plugin を Andon の repo に作り、Marketplace には出さない（オーナーが決めた）。** オーナーが、手元の Stream Deck のアプリに 1 回入れる。
   理由: 前提 5。既製の plugin では、色と件数を 1 枚に描くことも、押したときの動きを決めることもできない（事実 13）。
   捨てた案: Elgato のアプリを通さずに、デバイスを直接動かす（アプリとデバイスを取り合い、オーナーのほかのキーが使えなくなる）。

### 読む口

4. **2026-11-22 plugin は、App が書く `shared.json` の `inbox.lamp` を 2 秒ごとに読む（Chief Engineer が決めた）。** ファイルは、App の設定フォルダの `shared.json` である。読むのは、キー `inbox.lamp` の値だけである。
   理由: App は、値が変わるたびにこのファイルを書き直している（事実 2）。mod に書く口を足さなくて済み、mod `inbox` は今までどおり何も書かない。値が変わってからキーが変わるまでは、長くて 2 秒である。
   細部:
   - 読むのは、キーが 1 つ以上見えている間だけにする。見えているキーが無くなったら、読むのをやめる。タイマーは 1 本にする。
   - 見えているキーは、キーの名前（`context`）の集まりで持つ。同じキーの `willAppear` がもう一度届いても、数は増えない（事実 9）。
   - キーの画像を替えるのは、出す中身が変わったときと、`willAppear` が届いたときである。`willAppear` が届いたキーには、今の画像をいつも送る。
   - ファイルを開けなかった回と、JSON として読めなかった回は、前に読めた値を保つ。App がファイルを置き換える一瞬に読みが失敗しても、キーが変わらないようにする。保った値は、古くなれば「分からない」になる（決めたこと 6）。
   受け入れること: `shared.json` の名前と形は、Herdr App のものである。Herdr が外に約束しているのは、herdr-core の API のほうである（事実 16）。App がファイルを変えると、キーは「分からない」になる。灯りが出なくなるが、誤った色は出ない。
   乗り換える条件: mod が herdr-core の文書に書けるようになったら、mod `inbox` は灯りの値をそこへ置き、plugin は API の通知で受ける形に替える。そのときは、2 秒ごとの読みが要らなくなる。値を決める場所は mod `inbox` のままである。
   捨てた案: mod `inbox` が、灯りだけの別のファイルを書く（mod に書く口が増える。App がもう同じことをしている）／mod が plugin へ HTTP で送る（plugin があとから起動したときに、今の値が分からない。ポートの取り決めが要る）／plugin が `herdr-app` の command を数秒ごとに起動して聞く（Windows では、起動のたびにプロセスを作る。共有の値を読む command は App に無い）／mod `inbox` が灯りの値を herdr-core の文書に置き、plugin が API の通知で受ける（Herdr が公開している約束に乗れる。しかし、mod から文書に書く口が今は無い。事実 16。上の「乗り換える条件」のとおり、口ができたら替える）。
5. **2026-11-22 `inbox.lamp` に `version` と `at` を足し、mod `inbox` は描画を待たずに置き直す（Chief Engineer が決めた）。** 値の形は次のとおりにする。

   ```json
   {"version": 1, "at": 1795312345678, "light": "amber", "failed": 0, "blocked": 1, "merge": 2, "decision": 1, "reply": 0, "unread": 1, "unreadable": 0}
   ```

   `version` は 1 である。`at` は、mod `inbox` がこの値を決めた時刻で、1970 年からのミリ秒である。ほかの項目は [ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 10 のままである。mod `inbox` は、次のどれかのときに値を置き直す。`at` のほかの項目のどれかが、置いてある値と違う。置いてある `at` が 30 秒以上前である。置いてある `at` が今より先である。
   理由: App を閉じたあとも、最後の値はファイルに残る（事実 2）。時刻が無いと、plugin は、古い消灯と今の消灯を見分けられない。消灯のまま呼ばれていることに気づけないのが、いちばん害が大きい。
   細部:
   - mod `inbox` は、`andon waiting` の結果を共有の値に置いた直後にも、描画を待たずに灯りを決め直して置く。今は、結果が灯りに入るのは次の描画である（事実 1）。描画が止まっていると、新しい `at` が古い中身に付き、plugin は古い色を今の色として出す。
   - タイマーは足さない。今ある 30 秒のタイマーの回が、灯りを決めて値を置く。
   - 中身が同じで、置いてある `at` が 30 秒より新しいときは、置かない。値を置くと全部の mod が描き直されるので、描画のたびに置くと止まらなくなる。mod の写しが複数あっても、置くのは 30 秒に 1 回である。
   - 前の値と比べるときは、項目を 1 つずつ比べる。JSON の文字で比べると、App が項目の順を変えたときに、毎回置き直すことになる。
   - `inbox.lantern` は変えない。
   - これは [ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 10 の値に、項目を 2 つ足す改めである。その決定は Chief Engineer が決めたもので、足す理由は上のとおりである。実装の PR が、その文書の索引の 10 の行を直す。
   オーナーの手元で最初に確かめること: App の窓を最小化して 5 分置き、`shared.json` の `inbox.lamp` の `at` が進むこと。進まなければ、灯りの元を App に置く前提 4 が、最小化の間は成り立たない。そのときは Chief of Staff に返す。
   捨てた案: plugin が、ファイルの更新時刻で新しさを見る（ファイルは、ほかの mod が値を置いても書き直される。mod `inbox` が生きているかは分からない）／plugin が、App のプロセスが動いているかを見る（App は動いているが mod `inbox` がレイアウトに無いときを、見分けられない）。
6. **2026-11-22 「分からない」は plugin の表示だけの状態にし、灯りの状態は 3 つのままにする（Chief Engineer が決めた）。** plugin は、次のどれかのときに「分からない」を出す。まだ 1 回も値を読めていない。読めた `shared.json` に `inbox.lamp` が無い。`version` が 1 でない。`light` が `"red"`・`"amber"`・`null` のどれでもない。`at` が数でない。`at` が今より 90 秒より前である。`at` が今より 90 秒より先である。新しい値を 1 回読めれば、すぐ灯りに戻る。
   理由: 90 秒は、mod `inbox` が値を置き直す間隔の 3 回ぶんである。App を閉じたあと、長くて 90 秒は前の色が残る。
   細部:
   - 前のバージョンの mod `inbox` は、`at` を持たない値を置く。そのときも「分からない」になる。plugin は `inbox.lantern` を読まない。新しさが分からないからである。
   - PC がスリープから戻った直後は、`at` が古いので「分からない」になる。mod `inbox` の次のタイマーの回で戻る。長くて 30 秒である。
   捨てた案: 灯りの状態に 4 つ目を足し、mod が置く（App が動いていないときは、置く者がいない）。

### キー

7. **2026-11-22 キーには、行燈と同じ色と、待っているものの合計と、内訳を 2 行まで出す（Chief Engineer が決めた。形は仮で、オーナーが見て違うと言った所を直す）。**

   | 出すもの | キーの見た目 |
   |---|---|
   | 赤 | 地を行燈の赤 `#D8453B` で塗り、字は白にする |
   | 琥珀 | 地を行燈の琥珀 `#F0A202` で塗り、字は暗い色にする |
   | 消灯 | 暗い地に、灯っていない行燈の絵だけを出す。数は出さない |
   | 分からない | 暗い地に、行燈の絵と「?」を出す。消灯と見分けられる形にする |

   赤と琥珀のときは、合計を大きく 1 つ出す。合計は、`failed`・`blocked`・`decision`・`merge`・`reply` を足した数である。その下に、0 でない種類を、数と短い語（`failed`・`blocked`・`decide`・`merge`・`reply`）で、この順に 2 行まで出す。3 つ目からの種類は、合計にだけ数える。`unread` と `unreadable` は出さない。
   理由: 色の値は、左の列の行燈と同じにする（[ADR-0023](0023-sidebar-andon-look.md) 決めたこと 2）。順は、今すぐ仕事を止めているものを先にした。
   細部:
   - 画像は 144x144 の SVG で、数と語も画像の中に描く。タイトルは使わず、`UserTitleEnabled` を偽にする。
   - 点滅も、動きも、音も足さない。色が変わるときは、画像を 1 回替えるだけである（[ADR-0023](0023-sidebar-andon-look.md) 決めたこと 3・4 と同じ向き）。
   - 同じアクションを複数のキーに置いたときは、どのキーにも同じものを出す。
   - 複数の動きをまとめるキーには、入れられないようにする（`SupportedInMultiActions` を偽にする。事実 9）。
8. **2026-11-22 押すと mod `inbox` に待っているものを開かせ、Herdr App を前に出す。App は起動しない（押して App へ飛ぶことと、マージの依頼が待っているときも App のオーナー待ちのペインを開くことは、オーナーが決めた。ほかの細部は Chief Engineer が決めた）。** plugin は、キーが押されると、まず `herdr-app do inbox.call` を起動する（決めたこと 9）。その終了コードで、次を決める（事実 4）。

   | 終了コード | App の様子 | plugin がすること |
   |---|---|---|
   | 0 | 動いていて、mod `inbox` に届いた | 引数の無い `herdr-app` を起動して、窓を前に出す |
   | 1 | 動いているが、表示されている mod `inbox` が無い | 同じく窓を前に出し、そのキーに `showAlert` を 1 回出す |
   | 2 | 動いていない | 何も起動せず、`showAlert` を 1 回出す |
   | 起動できない・5 秒で終わらない | 分からない | 何も起動せず、`showAlert` を 1 回出す |

   理由: どこへ飛ぶかは、待っているものを知っている mod `inbox` が決める。引数の無い `herdr-app` は、App が動いていなければ自分が App になる（事実 4）。plugin の子として App が起動すると、Stream Deck を終えたときに App が強制で終わる（事実 12）。先に `do` の終了コードで App が動いていると確かめれば、引数の無い `herdr-app` は窓を前に出すだけで終わる。
   細部:
   - 灯っていても消灯でも「分からない」でも、同じ順で起動する。消灯のときは、App が前に出るだけである（決めたこと 9）。
   - 引数の無い `herdr-app` は、切り離して起動する。出力はつながず、終わるのを待たない。作業フォルダは利用者のホームにする。確かめたあとに App が終わっていて、これが App になったときも、plugin の道連れにしないためである。
   - どの command も、名前と引数を分けて渡し、シェルを通さない。窓を出さない指定で起動する。
   - 前の押下の command が終わるまでは、次の押下を捨てる。
   受け入れること:
   - Windows が前に出すことを断ると、タスクバーの App のボタンが光るだけになる（事実 11）。そのときも、待っているものは App の中で開いている。オーナーの手元で確かめて、光るだけだったら、次の手を Chief of Staff に返す。
   - App を閉じている間は、押しても App は起動しない。オーナーが自分で App を起動する。
   捨てた案: App が動いていなければ、キーから起動する（Stream Deck を終えると App が道連れになる恐れがある。切り離して起動しても、Stream Deck 自身が plugin を組に入れているかは分からない。起動の途中にもう一度押すと、窓が 2 つになる。事実 4・12。App は基本的に開いているので、まず起動しない形にする）／引数の無い `herdr-app` を先に起動し、終わったら `inbox.call` を送る（App が動いていないと、1 つ目が App になる。終わるまでの時間で見分ける決まりは、App の返事が遅いと外れる）／マージの依頼が待っているときは、いちばん古い PR をブラウザで直接開く（マージまでが 1 手減り、窓を前に出す制限にも当たらない。しかし、灯りの値に PR の URL と順番を足すことになり、mod `inbox` が読むものが増える。判断と止まった端末は、どのみち App で開く。App のペインから判断に答える口もある。まず App へ飛ぶ形で使ってみる）／plugin が、灯りの内訳を見て、飛ぶ先を自分で決める（飛ぶ先の決まりが、mod と plugin の 2 か所になる）。
9. **2026-11-22 mod `inbox` に、待っているものを順に開く command `inbox.call` を足す（Chief Engineer が決めた）。** 開く先は、灯りの元になっているものである。Needs you の行を古い順に並べ、そのあとに、オーナー待ちの件数の行を、いちばん古い待ちの古い順に並べる。`inbox.call` は、前の回に開いた先の次を開く。前の回が無いか、前の回の先がもう並びに無ければ、初めの 1 つを開く。終わりまで行ったら、初めに戻る。並びが空なら、何もしない。
   理由: 今ある `inbox.next` は、オーナー待ちの件数の行へは行かず、未読の返事へは行く（事実 5）。キーは、灯りの元へ飛ぶ。kanban のオーナー待ちのほとんどは、件数の行のほうにある（事実 6）。止まった端末が 1 つ残っていても、もう一度押せば次へ行ける。
   細部:
   - 並びは、マシンの絞り込みと検索を掛ける前のものから作る。灯りが、絞り込みと検索に関わらず全部を数えるからである（事実 5）。
   - Needs you の行を開く動きは、その行を押したときと同じにする。件数の行を開く動きも、その行を押したときと同じにする（事実 5）。
   - `inbox.next` は変えない。キーの割り当ては足さない。README の command の一覧に 1 行足す。
   - 左の列を隠していると、`inbox.call` は届かない。そのときは、plugin が App を前に出して `showAlert` を出す（決めたこと 8）。
   捨てた案: `inbox.next` が件数の行も回るように変える（キーボードの「次へ」の動きが変わる。この題の範囲の外である）／いつも、いちばん古い 1 つを開く（止まった端末が残っている間、何度押しても同じ行が開き、マージの依頼へ行けない）。

### 作りと配り方

10. **2026-11-22 plugin は npm の依存を持たず、Stream Deck の WebSocket の約束を直接話す（Chief Engineer が決めた）。** plugin は、Node.js の組み込みの部品だけを使う JavaScript のファイルにする。`manifest.json` は、Node.js の 26 と、Stream Deck のアプリの 7.1 以上を指定する。
    理由: この plugin が使う約束は、登録、キーが見えた、キーが消えた、キーが押された、画像を替える、警告を出す、の 6 つである。Node.js は、WebSocket のクライアントを組み込みで持つ（事実 8）。`package/host/` の部品は、どれも依存を持たず、ビルドもしない（事実 14）。イメージを焼くときに npm を呼ばなくて済む。Node.js を 26 にするのは、repo の検査が動く Node.js と揃えるためである。
    細部:
    - plugin は、起動の引数からポート番号と plugin の名前と登録の語を読み、`127.0.0.1` のそのポートにつないで、登録の 1 通を送る。
    - WebSocket が切れたら、plugin は終わる。つなぎ直しは作らない。
    - 知らない通知は、読み捨てる。
    受け入れること: Elgato の文書は公式の SDK を勧めている（事実 7）。約束が変わったときは、自分で追う。追えなくなったら、公式の SDK に替える。
    捨てた案: 公式の SDK `@elgato/streamdeck` を使う（実行時のパッケージが 6 つ入り、TypeScript とバンドラのビルドが要る。事実 8。型と、つなぎ直しと、ログが手に入る。しかし、使う約束は 6 つで、repo に 2 つ目の npm のプロジェクトと、ビルドした成果物の置き場が増える）。
11. **2026-11-22 plugin の名前は `andon.lamp` にし、`package/host/streamdeck/` に置く（Chief Engineer が決めた）。** フォルダは `package/host/streamdeck/andon.lamp.sdPlugin/` である。行燈のアクションの UUID は `andon.lamp.light`、表示の名前は「Lamp」、分類は「Andon」にする。2 つ目のアクションは、決めたこと 15 で足す。
    細部:
    - `manifest.json` の OS には、Windows と macOS を書く。確かめるのは Windows だけである。macOS は、動くように書くが、受け入れ条件に入れない。
    - plugin のアイコンの PNG と、キーの見本の画像は、Andon のロゴ（`docs/assets/logo/`）から作って、フォルダに置く。
    - フォルダの README に、何を読み、何を起動するかを書く。mod の README と同じ形にする。
12. **2026-11-22 `host-install` は、置き場を指定されたときだけ plugin を置く（Chief Engineer が決めた）。** 環境変数 `STREAMDECK_PLUGINS` に Stream Deck の plugin の置き場が入っているとき、`bin/bootstrap host-install` は、そこへ `andon.lamp.sdPlugin` を写す。今あるフォルダは上書きしない。入っていないときは、置かずに、置き方を 1 行で案内する。
    理由: Stream Deck を持たない利用者には要らない。mod と同じ口で置けば、手順が 1 つで済む（事実 14）。
    細部:
    - WSL では、Windows の置き場を `wslpath` で変えて渡す。置く前に Stream Deck のアプリを終え、置いたあとに起動する。そのあと、アクション「Lamp」をキーに置く。
    - 新しいバージョンに置き直すときは、mod と同じに、前のフォルダを退避してから打ち直す。
    - 外すときは、Stream Deck のアプリを終えて、フォルダを消す。
    - 前に作ったセルの `bin/bootstrap` は、この置き先を知らない（事実 14）。そのセルでは、`bin/bootstrap` を今の雛形（`init/bin/bootstrap`）で置き換えてから打つ。実装の課題は、雛形の `bin/bootstrap` がセルごとの値を持たず、そのまま置き換えられることを確かめる。
    - 手順は [operate.md](../operate.md) の `host-install` の節に書く。
    実装の課題が最初に確かめること: Elgato の検査器が `manifest.json` を通すこと。オーナーの手元で最初に確かめること: フォルダを写して Stream Deck のアプリを起動し直すと、plugin が読まれること（事実 10）。読まれなければ、配布用の 1 ファイルを作る形に替える。
    捨てた案: 配布用の 1 ファイルを作って、オーナーがダブルクリックで入れる（作る道具が npm にある。置き直しのたびに作ることになる）。
13. **2026-11-22 設定の画面は作らず、App の置き場と実行ファイルは環境変数で替える（Chief Engineer が決めた）。** plugin は、App の設定フォルダを、App と同じ順で見つける（事実 4）。`herdr-app` は PATH から起動する。PATH に無ければ、Windows では `%LOCALAPPDATA%\Programs\herdr-app\herdr-app.exe` を使う。別の場所にあるときは、環境変数 `HERDR_APP_BIN` で渡す。
    理由: 前提 5 の形では、どちらも既定のままで動く。設定の画面を足すと、HTML の画面と、設定の約束が増える。
    細部: plugin は、自分の環境変数をそのまま `herdr-app` に渡す。App を別の置き場で動かしているのに、Stream Deck がその環境変数を持たないと、キーは「分からない」になり、押すと `showAlert` が出る。誤った色は出ない。
14. **2026-11-22 `inbox.lamp` の形は Andon の約束に足さず、mod と plugin をつないだ検査で守る（Chief Engineer が決めた）。** 形の正は、`package/host/inbox/README.md` の表に置く。検査は、本物の mod `inbox` が置いた値を、本物の plugin に読ませる（受け入れ条件 11）。
    理由: Andon の約束（[contract.md](../contract.md) と `schema/`）は、セルの command が schema に照らして書く JSON のものである。この値は、オーナーの手元で、mod が書いて plugin が読む。mod と plugin は別々に置き直されるので、食い違いは `version` で見分ける（決めたこと 6）。

### herdr-core の API から足すもの

15. **2026-11-22 herdr-core の API から、動いている役の端末の数を出す 2 つ目のキーを足す（API で plugin の価値が上がるものを組み込むことは、オーナーが決めた。何を組み込むかと細部は、Chief Engineer が決めた）。** plugin に、2 つ目のアクション「Working」を足す。UUID は `andon.lamp.working` である。キーには、ラベル `role` を持ち、状態が `working` の端末の数を出す。手元とつないだマシンの全部を合わせて数える。
    理由: 行燈の消灯は、順調に動いているときと、何も動いていないときの、どちらでも同じである。行燈は、呼ばれているときだけ灯すと決めたからである（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 2）。動いている数が見えれば、消灯のまま 0 になったときに、次の仕事を入れる時だと分かる。この数は、herdr-core の API がそのまま返す事実で、灯りの規則ではない。灯りを決める場所は、mod `inbox` の 1 か所のままである。API の通知で受けるので、変わればすぐ替わり、App を閉じていても出る。
    細部:
    - plugin は、「Working」のキーが 1 つ以上見えている間だけ、`hc events --json` を子のプロセスとして 1 つ起動し、出てくる行を読む（事実 16）。初めの 1 通の端末の一覧から数え、そのあとの通知で直す。遅れたと知らされたら、送り直された一覧で数え直す。見えているキーが無くなったら、子を終わらせる。
    - この子は切り離さない。plugin が終われば、一緒に終わる。
    - 行が 45 秒届かないか、子が終わったら、キーに「?」を出す。45 秒は、生きている印の 3 回ぶんである。そのあと、30 秒おきに起動し直す。
    - 見た目は、暗い地に、数を大きく 1 つと、小さく `working` の語である。0 のときは、数の代わりに `idle` の語を、暗めに出す。琥珀と赤では塗らない。その 2 色は行燈のものである。点滅も音も足さない。
    - 押すと、`herdr-app status` を起動する。0 で終われば、引数の無い `herdr-app` を決めたこと 8 と同じ指定で起動して、App を前に出す。0 でなければ、何も起動せず、`showAlert` を出す。
    - `hc` は PATH から起動する。別の場所にあるときは、環境変数 `HC_BIN` で渡す。
    - 「Lamp」のキーは、`hc` を使わない。`hc` が無いか古いときは、「Working」のキーだけが「?」になる。
    - npm の依存は足さない（決めたこと 10）。
    オーナーの手元で最初に確かめること: Windows で `hc events --json` が起動でき、セルの端末の変化が届くこと。
    組み込まなかったもの: 「Lamp」のキーが「分からない」の間、herdr-core から読んだ、止まっている端末の数で補う（止まった端末の読み方が、mod `inbox` と plugin の 2 か所になる。App は基本的に開いている）／止まっている端末の題や、報告の文をキーに出す（キーには収まらない。タッチ帯に出す題のときに、この API から読める）／キーから端末に返事を送る（やらないこと）／行燈の値を API の通知で受ける（mod から書く口が無い。決めたこと 4 の乗り換える条件）。

## やらないこと（範囲の外。理由）

- **タッチ帯、ダイヤル、Neo の情報帯に出すこと。** 決めたこと 1。機種が合えば、あとから足せる。
- **キーの点滅、動く文字、音、通知。** 決めたこと 7。
- **プロジェクトごとのキー。** 灯りも、動いている数も、全部のプロジェクトを合わせた 1 つである（[ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 10）。
- **動いている数で色を付けること、0 になったときに知らせること。** 決めたこと 15。数を出すだけにする。
- **Stream Deck からマージすること、判断に答えること。** マージは GitHub で、判断は App のペインで答える（[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 決めたこと 17）。
- **マージの依頼の PR を、キーからブラウザで直接開くこと。** 決めたこと 8 の捨てた案。使ってみて、App を経るのが遠回りだとオーナーが言ったら、次の題にする。
- **App を閉じている間も灯すことと、キーから App を起動すること。** 決めたこと 2・8。
- **plugin が herdr-core の API につないで、灯りを自分で決めること。** 決めたこと 2 の捨てた案。灯りを決める場所を 2 か所にしない。
- **灯す条件と、灯りの状態の数を変えること。** [ADR-0043](0043-lamp-lights-on-owner-waits.md) 決めたこと 2〜4 のままにする。
- **`inbox.next` と、左の列の行燈の見た目を変えること。** 決めたこと 9。mod `andon` のコードは変えない。
- **Marketplace に出すこと、Linux で動かすこと、設定の画面。** 決めたこと 3・13。
- **`inbox.lamp` を `schema/` に足すこと。** 決めたこと 14。
- **前に作ったセルの `bin/bootstrap` を、Andon に追従させる仕組み。** この題では、置き換える手順を書くだけにする（決めたこと 12）。仕組みは別の題である。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 3 つにする。plugin と mod は `bash tests/host.sh`。plugin は、本物のコードに、偽の WebSocket と、一時のフォルダの `shared.json` と、偽の時計と、偽の起動の口を渡して動かす（新しい `tests/host/streamdeck.mjs`）。mod `inbox` は、今ある `tests/host/screen.mjs` で動かす。置き方は `bash tests/andon-init.sh`。オーナーの手元で見る条件は `manual` にする。画像を見る条件は、`setImage` の `data:` の URL を解いてから見る。

| # | 条件 | 証拠（command と見る出力） |
|---|---|---|
| 1 | plugin の入口に、ポート番号・plugin の名前・登録の語・アプリの情報の 4 つの引数を渡すと、`127.0.0.1` のそのポートへつなぎ、登録の語と plugin の名前を入れた 1 通を初めに送る。WebSocket が切れると、plugin は終わる。知らない通知が届いても、何も送らない | `bash tests/host.sh`（`streamdeck.mjs` の 3 つの場合。偽の WebSocket が受けた宛先と 1 通目を比べる） |
| 2 | `inbox.lamp` の `light` が `null`・`"amber"`・`"red"` で、`at` が新しいとき、見えているキーに `setImage` が 1 通ずつ送られる。琥珀の画像には `#F0A202`、赤の画像には `#D8453B` が入り、消灯の画像にはどちらも入らない。同じ値をもう一度読んでも、`setImage` は送られない | `bash tests/host.sh`（4 つの場合） |
| 3 | `failed` 1・`blocked` 1・`decision` 1・`merge` 2 のとき、画像に合計の 5 と、「1 failed」と「1 blocked」が入り、「decide」と「merge」は入らない。`merge` 2 だけのとき、合計の 2 と「2 merge」が入る。消灯のとき、数は入らない | `bash tests/host.sh`（3 つの場合） |
| 4 | まだ読めていない、`inbox.lamp` が無い、`version` が 2、`light` が決まった 3 つでない、`at` が無い、`at` が文字、`at` が 91 秒先、のどれでも、「分からない」の画像が送られる。その画像は、消灯の画像と違う。そのあと読める値に戻すと、灯りの画像が送られる | `bash tests/host.sh`（7 つの場合） |
| 5 | 琥珀を出したあと、ファイルを変えずに偽の時計を 91 秒進めると、「分からない」の画像が送られる。`at` の新しい値を書くと、琥珀に戻る。琥珀を出したあと、ファイルを消すか JSON でない中身にしても、90 秒たつまでは琥珀のままで、`setImage` は送られない | `bash tests/host.sh`（3 つの場合） |
| 6 | `keyDown` が届くと、初めに `herdr-app do inbox.call` が起動される。それが 0 で終わると、引数の無い `herdr-app` が、切り離す指定と、ホームの作業フォルダで起動され、`showAlert` は送られない。1 で終わると、同じく起動され、`showAlert` が 1 通送られる。2 で終わる、起動できない、5 秒で終わらない、のどれでも、ほかに何も起動されず、`showAlert` が 1 通送られる。前の押下が終わる前の `keyDown` では、何も起動されない。どの起動も、シェルを通さず、窓を出さない指定になっている | `bash tests/host.sh`（偽の起動の口が受けた呼び出しを並べ、決まった一覧と比べる。6 つの場合） |
| 7 | App の置き場は、`HERDR_APP_HOME`、`XDG_CONFIG_HOME` の下の `herdr-app`、ホームの `.config/herdr-app` の順に見つける。`HERDR_APP_BIN` があれば、その値が起動される。無ければ `herdr-app` が起動される。Windows で PATH に無いときは、`%LOCALAPPDATA%` の下の実行ファイルが起動される | `bash tests/host.sh`（6 つの場合。環境変数と OS の名前は、偽のものを渡す） |
| 8 | キーが見える前と、見えているキーが無くなったあとは、ファイルを読まない。キーが 2 つ見えていると、両方に同じ画像が送られる。あとから見えたキーには、今の画像が送られる。同じキーの `willAppear` がもう一度届くと、そのキーに今の画像が送られ、そのあと `willDisappear` が 1 回届けば、読むのをやめる。タイマーは 1 本である | `bash tests/host.sh`（5 つの場合） |
| 9 | plugin のフォルダに `node_modules` も lock ファイルも無く、コードが読み込むのは `node:` で始まる部品だけである。`manifest.json` の plugin の UUID がフォルダの名前と合い、Node.js は 26、アプリは 7.1 以上、`UserTitleEnabled` と `SupportedInMultiActions` は偽である。Elgato の検査器が manifest を通す | `bash tests/host.sh`（フォルダと manifest を見る行。当たる場合があることを同じ所で確かめる）と、担当が 1 回打つ `npx @elgato/cli validate <フォルダ>` の出力 |
| 10 | mod `inbox` が置く `inbox.lamp` に、`version` の 1 と `at` がある。中身が変わると、すぐ置き直す。変わらないとき、30 秒たつ前は置かず、30 秒たつと `at` だけ新しくして置く。置いてある `at` が今より先なら、置き直す。項目の順だけが違う値が置いてあっても、置き直さない。偽の `exec` が `andon waiting` の結果を返すと、描画もタイマーも進めずに、`inbox.lamp` の `merge` が変わる。mod の写しを 2 つ動かしても、30 秒に置くのは 1 回である。`inbox.lantern` の値は変わらない | `bash tests/host.sh`（`screen.mjs` に、偽の `setShared` が受けた回数と値を見る 7 つの場合） |
| 11 | 本物の mod `inbox` が偽の API の下で置いた `inbox.lamp` を、そのまま `shared.json` に書いて本物の plugin に読ませると、同じ色と合計の画像が送られる | `bash tests/host.sh`（1 つの場合） |
| 12 | `inbox.call` は、Needs you の行が 2 つと件数の行が 1 つあるとき、呼ぶたびに、古いほうの行、新しいほうの行、件数の行、古いほうの行の順に開く。件数の行は、ペインのあるレイアウトではペインを開く操作を呼び、無いレイアウトでは Chief of Staff の端末へ行く。マシンの絞り込みか検索で行が隠れていても、同じ順に開く。並びが空なら、何も呼ばない。`inbox.next` の動きは変わらない | `bash tests/host.sh`（`screen.mjs` の 5 つの場合と、今ある検査が通ったまま） |
| 13 | `STREAMDECK_PLUGINS` に一時のフォルダを渡して `host-install` を打つと、`andon.lamp.sdPlugin` が置かれる。もう一度打つと、触らないと出る。渡さないと、置かずに案内の 1 行を出し、ほかのものは今までどおり置かれる。`package/host/inbox/README.md` に、`inbox.lamp` の項目の名前の全部と `inbox.call` がある | `bash tests/andon-init.sh`（3 つの場合）と `bash tests/host.sh`（README を見る行。当たる場合があることを同じ所で確かめる） |
| 14 | オーナーの手元で、次を見る。フォルダを写して Stream Deck を起動し直すと、アクション「Lamp」が出る。キーが App の行燈と同じ色になる。Chief of Staff がマージを頼むと、App を見ていなくても、1 分以内にキーが琥珀になり、件数が出る。マージすると、片付けのあとに消える。App の窓を最小化して 5 分置いても、キーは「分からない」にならない。App を閉じると、2 分以内に「分からない」になる。押すと、App が前に出て、待っているものが開く。もう一度押すと、次のものが開く。前に出ずにタスクバーが光るだけなら、そう記録する。Stream Deck を終えても、App は終わらない | `manual`（オーナーが plugin を入れ、mod を置き直して、半日使って見る） |
| 15 | 「Working」のキーが見えると、`hc events --json` が 1 つ起動される。初めの 1 通に、`role` のラベルがあって `working` の端末が 2 つ、`role` の無い `working` の端末が 1 つ、`role` があって `blocked` の端末が 1 つあると、画像に 2 と `working` が入る。状態が変わった通知を 1 通流すと、3 に替わる。遅れたという知らせと新しい一覧を流すと、その一覧の数に替わる。0 のとき、画像に `idle` が入る。どの画像にも `#F0A202` と `#D8453B` は入らない | `bash tests/host.sh`（`streamdeck.mjs` に、偽の子のプロセスが行を流す 5 つの場合） |
| 16 | 偽の時計を 46 秒進めて行が届かないと、「?」の画像が送られる。子が終わっても「?」が送られ、30 秒後に起動し直される。「Working」のキーが無くなると、子は終わらされる。`HC_BIN` があれば、その値が起動される。「Lamp」のキーだけが見えているときは、`hc` は起動されない。`hc` が起動できなくても、「Lamp」のキーの画像は今までどおり送られる | `bash tests/host.sh`（6 つの場合） |
| 17 | 「Working」のキーの `keyDown` で、`herdr-app status` が起動される。0 で終わると、引数の無い `herdr-app` が切り離す指定で起動される。0 でないと、ほかに何も起動されず、`showAlert` が 1 通送られる。`manifest.json` にアクションが 2 つあり、どちらも `UserTitleEnabled` と `SupportedInMultiActions` が偽である | `bash tests/host.sh`（3 つの場合と、manifest を見る行） |
| 18 | オーナーの手元で、次を見る。「Working」のキーの数が、動いている役の端末の数と合う。担当が動き出すと、数秒で増える。App を閉じても、数は出たままである。Stream Deck を終えると、`hc events` のプロセスが残らない | `manual`（オーナーが、キーを置いて半日使って見る） |

[operate.md](../operate.md) の `host-install` の節の入れ方と置き直し方と外し方、plugin のフォルダの README、CHANGELOG の `host-install` の項は、検査では見ない。レビューが差分で見る。

## 価値の仮説

1. **誰の何が変わるか。** オーナーが、別の窓で作業していても、呼ばれたことにキーの色で気づき、1 回押して待っているものへ行けるようになる。呼ばれていないかを確かめるために App へ切り替えなくて済む。あわせて、行燈が消えているときに、ラインが動いているのか空いているのかを、2 つ目のキーの数で見分けられる。
2. **見る信号。** オーナーの目視。あわせて、[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 事実 3 と同じ数え方で、gate のマージの依頼の答えまでの時間を、plugin がオーナーの手元に入った日から数える。この数は目安である。同じ時期に、行燈の灯し方とオーナー待ちのペインも変わっている。
3. **見る日。** 2026-12-15。
4. **誰が要るか。** オーナー。Stream Deck に plugin を入れてキーを置き、mod `inbox` を置き直して使う。
5. **効いたとどう分かるか。** オーナーが、キーが灯ったのを見て動いた回が、日に 1 回以上あると言える。確かめるためだけに App へ切り替えることが減ったと言える。マージの依頼の答えまでの時間の中央値が、24 分より短い。2 つ目のキーは、数が 0 になったのを見て次の仕事を入れた回があると言えれば、効いている。
6. **何があれば止めるか。** キーの色が、App の行燈と食い違っているのを見た。App を開いているのに、「分からない」が日に何度も出る。スリープから戻った直後の 30 秒は、これに数えない。または、オーナーがキーを見なくなった。初めの 2 つのときは、plugin のフォルダを消して外し、食い違いの元を調べる。mod `inbox` に足した項目と command は、残しても害が無い。最後のときは、外して終える。2 つ目のキーは、数が実際と合わないか、見て動いた回が 1 回も無ければ、キーを外す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **plugin と、mod `inbox` の 2 つの直しを、1 つの課題で作る。** 決めたこと 4〜14。`package/host/streamdeck/andon.lamp.sdPlugin/`、`package/host/inbox/` の `main.js` と README、`tests/host/streamdeck.mjs`、`tests/host/screen.mjs`、`tests/host.sh`、`init/bin/bootstrap` の `host-install`、`tests/andon-init.sh`、[operate.md](../operate.md)、CHANGELOG。受け入れ条件 1〜13。アクションは「Lamp」の 1 つで作る。今ある検査は、`inbox.lamp` を 8 つの項目の一致で見ているので、`version` と `at` に合わせて書き換える。レベル C で、オーナーがマージする。課題は、オーナーの手元でしか確かめられないこと（受け入れ条件 14）を、PR の説明に並べる。
2. **plugin に、アクション「Working」を足す。** 決めたこと 15。1 がマージされたあとに始める。同じ plugin のコードと `tests/host/streamdeck.mjs` を直すからである。受け入れ条件 15〜17。mod `inbox` には触れない。レベル C で、オーナーがマージする。1 と分けるのは、行燈のキーを先に手元へ届けるためである。
3. **オーナーの手: plugin を入れ、mod `inbox` を置き直し、キーを置く。** 1 がマージされたあと。2 がマージされたら、plugin を置き直して、「Working」のキーを置き、受け入れ条件 18 を見る。手順は [operate.md](../operate.md) の `host-install` の節。前に作ったセルでは、先に `bin/bootstrap` を置き換える（決めたこと 12）。入れたら、受け入れ条件 14 を見る。最小化で `at` が止まる、フォルダを写しただけでは読まれない、押しても前に出ない、のどれかがあれば、Chief of Staff に言う。
4. **観察。** 価値の仮説の 2 と 5 と 6 を、見る日に数える。

1 の実装の PR は、この文書の索引の行と、[ADR-0043](0043-lamp-lights-on-owner-waits.md) の索引の 10 の行を直す。

## 開いている問い（残った問い。決断の種類つき）

無し。出口で聞いた 1 問は、オーナーが 2026-11-22 に答えた。押したとき、マージの依頼が待っていれば、App のオーナー待ちのペインを開く（決めたこと 8。`users`）。

## 出典

- オーナーの発言: 2026-11-21（Chief of Staff との対話「StreamDeckにもランプの状態を出したりできるとよさそう」）と、2026-11-22（Chief Engineer との対話の入口の確かめ「HerdrAppが起動していなくても出せうるの？、基本的に非アクティブでもHerdrAppは開いていると思う。タッチ帯にメッセージは流せたりするの？」と、範囲の答え「キーの行燈と、押して飛ぶ」と、出口の答え「App のペインを開く」と、出口の指摘「https://herdr.dev/core/ にStreamDeckが記載されてるのは確認済みだよね？」と、その答え「App が決めた値を読む」と、そのあとの発言「APIを元にPluginの価値が上がるものがありそうならそれも組み込もう」）。
- repo の中の文書: [design.md](../design.md) §0.1・§1.8、[operate.md](../operate.md) の `host-install` と「Herdr App の mod とレイアウト」、[ADR-0015](0015-inbox-needs-you.md)、[ADR-0023](0023-sidebar-andon-look.md) 決めたこと 2〜5、[ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 事実 3 と決めたこと 12・17、[ADR-0043](0043-lamp-lights-on-owner-waits.md) 事実 5 と決めたこと 2〜10、[ADR-0044](0044-owner-digest.md)、`package/host/inbox/README.md`、`package/host/andon/README.md`、`package/host/waiting/README.md`、`package/gate.json`、`CHANGELOG.md`。
- コード: `package/host/inbox/main.js`、`package/host/andon/brand.js`、`init/bin/bootstrap`、`tests/host.sh`、`tests/host/screen.mjs`、`tests/andon-init.sh`、`model/package.json`、`mise.toml`。
- Herdr App（[herdrdev/herdr-app](https://github.com/herdrdev/herdr-app) の commit `317e673f1f1568b876cbaa0b41dabeb9153ecbea`）: `defaults/AGENTS.md`（mod の手引き）、`src/mods.rs`、`src/persist.rs`、`src/control.rs`、`src/main.rs`、`src/window.rs`、`distribution/install.ps1`、`README.md`、`src/hc.rs`、command の一覧、タイマーの実装。
- herdr-core: Herdr のサイトの [herdr-core のページ](https://herdr.dev/core/)（2026-11-22 に読んだ）、`hc schema` と `hc --help`（0.1.17）、[herdrdev/herdr-core](https://github.com/herdrdev/herdr-core) の `CHANGELOG.md` と `crates/herdr-core/src/cli.rs`。
- Elgato の文書: [WebSocket API の Plugin](https://docs.elgato.com/streamdeck/sdk/references/websocket/plugin)、[Manifest](https://docs.elgato.com/streamdeck/sdk/references/manifest)、[Plugin Environment](https://docs.elgato.com/streamdeck/sdk/introduction/plugin-environment)、[Keys](https://docs.elgato.com/streamdeck/sdk/guides/keys)、[Plugin Guidelines](https://docs.elgato.com/streamdeck/sdk/style-guide/plugin-guidelines)、[System](https://docs.elgato.com/streamdeck/sdk/guides/system)、[Distribution](https://docs.elgato.com/streamdeck/sdk/introduction/distribution)、[streamdeck link](https://docs.elgato.com/streamdeck/cli/commands/link)。npm の registry の [@elgato/streamdeck](https://registry.npmjs.org/@elgato/streamdeck)。
- ほかの外の文書: Microsoft の [SetForegroundWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setforegroundwindow)、Node.js の [Global objects](https://nodejs.org/docs/latest-v24.x/api/globals.html) と [child_process](https://nodejs.org/docs/latest-v24.x/api/child_process.html)、libuv の [src/win/process.c](https://github.com/libuv/libuv/blob/v1.x/src/win/process.c)、既製の plugin の [Stateful Executor](https://github.com/pandomic/stream-deck-stateful-executor) と [API Monkey](https://github.com/ft-t/apimonkey)。
- 測ったもの: 公式の SDK を入れたときのパッケージの数（`npm install --package-lock-only` の結果を数えた）。Node.js 26.13.0 に組み込みの `WebSocket` があること。`package/host/` の mod の数と、npm の依存の数。
- 反論: 別のコンテキストの読むだけのサブエージェントに、この設計を壊させた（2026-11-22）。返った指摘は、コードと外の文書に照らしてから、事実 1・4・5・9・10・12・14 と、決めたこと 4・5・6・8・9・10・12 と、受け入れ条件に入れた。
