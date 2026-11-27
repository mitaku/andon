# tray

セルが出した報告（`andon publish` の HTML や Markdown・ダイジェスト・担当の報告）を1か所に集めて読む場所。設計は [ADR-0064](../docs/adr/0064-tray.md)。

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

## 検査

Bun と Node のバージョンは repo の `mise.toml`。

```sh
bun install --frozen-lockfile --ignore-scripts
bun run typecheck
bun run test      # Workers の検査（workerd の上）
bun run build     # Workers を dist/workers に、画面を dist/public に作る
bun run e2e       # ローカルの tray を一時の置き場に起動して、アップロードから既読までを通す
bunx playwright install chromium && bun run browser   # 画面・開く形式・報告の面の sandbox をブラウザで確かめる
```

CI では `.github/workflows/tray.yml` の job `tray` が同じものを回す。依存は外から取るコードなので、この job は秘密を読まず、書き込みの権限も持たない。
