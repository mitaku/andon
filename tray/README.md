# tray

セルが出した報告（`andon file publish` の HTML や Markdown・ダイジェスト・担当の報告）を1か所に集めて読む場所。設計は [ADR-0064](../docs/adr/0064-tray.md)。

## 3つの面

| 面 | 入口 | 守り |
|---|---|---|
| 一覧の面 | `src/list.ts`（`/api/`） | Cloudflare Access の JWT（aud・iss・期限・署名）。書き込みは同じ Origin だけ |
| 報告の面 | `src/view.ts`（`/r/<id>/<sha256>?e=&s=`） | 中身ごとの R2 のキーに結び付けた、5分で切れる HMAC の署名付き URL。CSP の sandbox で、一覧の面と別のオリジン |
| アップロードの口 | `src/upload.ts`（`PUT /v1/reports/<プロジェクト>/<名前>`） | セルごとの API キー。キーはセルを決め、ほかのセルの報告は書けない |

D1 の表は `migrations/`、アップロードの約束と見本は `contract/upload-v1.json`。

## 一覧の面の画面

画面は `web/` に置き、`vite build` で `dist/public/` に作る。画面のファイルにデータは入れず、データは一覧の面の API から読む。

| パス | 画面 | 置き場 |
|---|---|---|
| `/` | 未読の報告。セルとプロジェクトでまとめ、新しい順に並べ、まとめで絞り込める | `web/src/pages/list.tsx` |
| `/all` | 30日の報告（既読も並べる） | 同じ |
| `/old` | 古い報告（最後に上がってから30日を過ぎたもの） | 同じ |
| `/r/<id>` | 報告。戻る口だけの細い帯と、別オリジンの報告の面を出す枠 | `web/src/pages/report.tsx` |
| `/cells` | セルと鍵。鍵を作る・作り直す・失効させる。鍵は作ったときに1度だけ見せる。直近7日の上がった枚数と開いた報告の数も出す | `web/src/pages/cells.tsx` |

報告の開き方は `src/lib/formats.ts` の1つの表で決める。HTML・Markdown・テキスト・画像は枠の中に、PDF は新しいタブに出し、ほかの形式はダウンロードにする。報告の面のヘッダ（`Content-Disposition`）と画面の開き方は、同じ表を読む。

スタックは Solid 2.0・`@solidjs/router`・UI SDK（`@ui-sdk/solid`）・StyleX で、バージョンは `package.json` で固定する。色はオーナーのデザインシステムの値を `web/src/theme.stylex.ts` の変数にして、`prefers-color-scheme` で明るい組と暗い組を切り替える。ホーム画面への追加は `public/pwa/` の manifest とアイコンだけで支え、service worker は置かない（`public/` は `dist/public/` へそのまま写る）。

ローカルの tray は `src/local.ts`（一覧の面と報告の面を Host で振り分ける。Access は通さない）と `src/local-upload.ts`（127.0.0.1 だけで開くアップロードの口）を、`scripts/serve.mjs` が miniflare で起動する。

## Cloudflare へのデプロイ

`scripts/deploy.mjs` が wrangler で、D1 と R2（どちらも `andon-tray`）を無ければ作り、D1 の移行を当て、3つの面を別々の Worker（`andon-tray-list`・`andon-tray-view`・`andon-tray-upload`）にして出す。束縛は面ごとに要るものだけを持たせ、workers.dev の名前は切って、ホスト名（custom domain）だけで開く。署名の鍵は打つたびに作り直して secret で渡す。アカウントの ID・3つのホスト名・Access のチームと aud は、セルの repo の `tray-deploy.json` から読み、この repo には書かない（形の見本は `deploy.example.json`）。

オーナーはセルの外で `CLOUDFLARE_API_TOKEN=<トークン> bin/bootstrap tray-deploy` と打つ。andon のイメージから使い捨てのコンテナを起動し、`andon tray deploy` がこのスクリプトを Node で動かす。手順の全体（有料プラン・R2・Access・鍵・`project.json` の `tray`）は [operate.md](../docs/operate.md) §8。`bun run deploy-check` は Cloudflare に触れず、見本の設定で組んだ3つの Worker の設定を `wrangler deploy --dry-run` に通す。

上げ直しで古くなった中身は、R2 に置いてから1日を過ぎたものを、同じ報告の次の上げ直しで消す（`src/lib/upload-api.ts` の `dropOld`。[決定の記録](../docs/decisions/2026-11-27-tray-old-content-one-day.md)）。

## 検査

Bun と Node のバージョンは repo の `mise.toml`。

```sh
bun install --frozen-lockfile --ignore-scripts
bun run typecheck
bun run test      # Workers の検査（workerd の上）
bun run build     # Workers を dist/workers に、画面を dist/public に作る
bun run deploy-check   # デプロイの設定を wrangler に確かめさせる（Cloudflare には触れない）
bun run e2e       # ローカルの tray を一時の置き場に起動して、アップロードから既読までを通す
bunx playwright install chromium && bun run browser   # 画面・開く形式・報告の面の sandbox をブラウザで確かめる
```

CI では `.github/workflows/tray.yml` の job `tray` が同じものを回す。依存は外から取るコードなので、この job は秘密を読まず、書き込みの権限も持たない。
