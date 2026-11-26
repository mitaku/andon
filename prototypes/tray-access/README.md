# tray の試作: Access とホーム画面からの確かめ

捨てる前提の試作で、main にマージしない。確かめるのは、Cloudflare Access の内側に置いた一覧の面を、iPhone のホーム画面に追加した画面から開いて、Access のログインを通れるかである。あわせて、別オリジンの報告を枠の中に出せるかと、PDF がどう開くかを見る。

## 置いてある物

| 置き場 | 中身 |
|---|---|
| `list/worker.js` | 一覧の面の Worker。`Cf-Access-Jwt-Assertion` の無い要求を 403 で断る。`/` に確かめの画面、`/api/check` に Access を通ったかを返す口を持つ。 |
| `list/public/pwa/` | manifest とアイコン。静的なファイルで、Worker のコードを通らずに返る。 |
| `view/worker.js` | 報告の面の Worker。見本の報告と3ページの PDF を、tray の設計の報告の面のヘッダ（`sandbox` 付きの CSP）で返す。`/sample.pdf?plain=1` は CSP を付けずに返す。署名付き URL は持たない。 |
| `*/wrangler.example.jsonc` | 設定の見本。写して `wrangler.local.jsonc` にし、ホスト名を埋める。`wrangler.local.jsonc` は commit しない。 |

ホスト名、アカウントの ID、Access のチームと `aud` は、この repo に書かない。

## デプロイの手順

セルの外の PC で打つ。Cloudflare のトークンはこの手順の間だけ環境変数に置き、セルに置かない。

1. Workers を有料プランにする（ダッシュボードの Workers & Pages → Plans）。
2. 一覧の面と報告の面のホスト名を、Cloudflare に置いたドメインの下に1つずつ決める。以下では `<一覧>` と `<報告>` と書く。
3. Access のアプリを2つ作る（Zero Trust → Access → Applications → Self-hosted）。
   - 1つ目はドメインを `<一覧>` にする。ポリシーは Allow で、Include にオーナーのメールアドレス1つを入れる。セッションは 30 日にする。
   - 2つ目はドメインを `<一覧>`、パスを `pwa/*` にする。ポリシーは Bypass で、Include を Everyone にする。manifest とアイコンを cookie 無しで取れるようにするためである。
   - `<報告>` には Access を置かない。
4. この試作の branch を取る。`<branch>` は試作を頼んだ報告に書いてある。

   ```
   git clone https://github.com/<owner>/andon.git && cd andon
   git checkout <branch>
   cd prototypes/tray-access
   ```

5. 設定を写してホスト名を埋める。

   ```
   cp list/wrangler.example.jsonc list/wrangler.local.jsonc
   cp view/wrangler.example.jsonc view/wrangler.local.jsonc
   # 2つのファイルの <一覧の面のホスト名> と <報告の面のホスト名> を書き換える
   ```

6. トークンを用意する。ダッシュボードの My Profile → API Tokens で、テンプレート「Edit Cloudflare Workers」から作る。打つシェルにだけ入れる。

   ```
   read -rs CLOUDFLARE_API_TOKEN && export CLOUDFLARE_API_TOKEN
   export CLOUDFLARE_ACCOUNT_ID=<アカウントの ID>
   ```

7. 報告の面、一覧の面の順にデプロイする。andon のイメージの Node で打つ。

   ```
   for d in view list; do
     docker run --rm -it -u "$(id -u):$(id -g)" -e HOME=/tmp \
       -e CLOUDFLARE_API_TOKEN -e CLOUDFLARE_ACCOUNT_ID \
       -v "$PWD":/w -w /w/$d --entrypoint npx ghcr.io/<owner>/andon:<バージョン> \
       -y wrangler@4.168.0 deploy -c wrangler.local.jsonc
   done
   ```

   PC に Node 20 以上があれば、各フォルダで `npx -y wrangler@4.168.0 deploy -c wrangler.local.jsonc` と打ってもよい。

8. PC のブラウザで `https://<一覧>/` を開く。Access のログインのあとに確かめの画面が出れば、デプロイは済んでいる。Access を通さずに Worker に届いた要求は 403 になる。

## iPhone で確かめること

1. Safari で `https://<一覧>/` を開いてログインし、共有の口から「ホーム画面に追加」を選ぶ。アイコンと名前「tray」が出るかを見る。
2. Safari を閉じ、ホーム画面のアイコンから開く。Access のログインを求められたら通す。ログインの画面がアプリの中に出たか、Safari に移ったかを見る。
3. 画面の「1. 開き方」が「ホーム画面から開いた」になり、「2. API の要求」が「Access を通って返った」になるかを見る。
4. 「3. 枠の中の報告」に見本の報告が出て、黄色の行が「スクリプトが動いた」になるかを見る。枠の中を 60 行目まで送れるかも見る。
5. 「4. PDF」の2つのリンクを開き、3ページとも見えるか、どこで開いたか、元の画面に戻れるかを見る。
6. アプリを閉じてから開き直し、ログインをもう一度求められるかを見る。

## 片付け

確かめが済んだら、2つの Worker（`tray-proto-list` と `tray-proto-view`）と Access のアプリを消す。本物の tray は別の課題で `tray/` に作る。
