# tray

セルが出した報告（`andon publish` の HTML や Markdown・ダイジェスト・担当の報告）を1か所に集めて読む場所。設計は [ADR-0064](../docs/adr/0064-tray.md)。

## 3つの面

| 面 | 入口 | 守り |
|---|---|---|
| 一覧の面 | `src/list.ts`（`/api/`） | Cloudflare Access の JWT（aud・iss・期限・署名）。書き込みは同じ Origin だけ |
| 報告の面 | `src/view.ts`（`/r/<id>/<sha256>?e=&s=`） | 中身ごとの R2 のキーに結び付けた、5分で切れる HMAC の署名付き URL。CSP の sandbox で、一覧の面と別のオリジン |
| アップロードの口 | `src/upload.ts`（`PUT /v1/reports/<プロジェクト>/<名前>`） | セルごとの API キー。キーはセルを決め、ほかのセルの報告は書けない |

D1 の表は `migrations/`、アップロードの約束と見本は `contract/upload-v1.json`。画面（`public/`）は仮の最小のもの。

ローカルの tray は `src/local.ts`（一覧の面と報告の面を Host で振り分ける。Access は通さない）と `src/local-upload.ts`（127.0.0.1 だけで開くアップロードの口）を、`scripts/serve.mjs` が miniflare で起動する。

## 検査

Bun と Node のバージョンは repo の `mise.toml`。

```sh
bun install --frozen-lockfile --ignore-scripts
bun run typecheck
bun run test      # Workers の検査（workerd の上）
bun run build
bun run e2e       # ローカルの tray を一時の置き場に起動して、アップロードから既読までを通す
bunx playwright install chromium && bun run browser   # 報告の面の sandbox をブラウザで確かめる
```

CI では `.github/workflows/tray.yml` の job `tray` が同じものを回す。依存は外から取るコードなので、この job は秘密を読まず、書き込みの権限も持たない。
