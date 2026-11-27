// ローカルの tray の入口（セルの中だけ。デプロイの設定に載せない。ADR-0064 決めたこと 16・26）。
// 一覧の面と報告の面を1つにまとめ、Host で振り分ける。名指した2つのホスト名のほかの Host は断る。
// Access の確かめを通さない組み立てはこのファイルにだけ置く（セルの中の portless の名前は、セルを動かしている PC からしか開けない）。
// オリジンは要求の URL（portless が渡す Host とポート）から作る。
// 束縛: DB・BUCKET・SIGNING_KEY・LIST_HOST・VIEW_HOST・ASSETS（画面のファイル）

import { error, LIST_HEADERS, withHeaders } from "./lib/http";
import { listAPI } from "./lib/list-api";
import { viewFace } from "./lib/view-face";

export interface LocalEnv {
  DB: D1Database;
  BUCKET: R2Bucket;
  SIGNING_KEY: string;
  LIST_HOST: string;
  VIEW_HOST: string;
  ASSETS: Fetcher;
}

export default {
  async fetch(req: Request, env: LocalEnv): Promise<Response> {
    const url = new URL(req.url);
    const port = url.port ? `:${url.port}` : "";
    const listOrigin = `${url.protocol}//${env.LIST_HOST}${port}`;
    const viewOrigin = `${url.protocol}//${env.VIEW_HOST}${port}`;
    if (url.hostname === env.LIST_HOST) {
      if (url.pathname.startsWith("/api/")) {
        return listAPI(req, { DB: env.DB, SIGNING_KEY: env.SIGNING_KEY, VIEW_ORIGIN: viewOrigin });
      }
      return withHeaders(await env.ASSETS.fetch(req), LIST_HEADERS);
    }
    if (url.hostname === env.VIEW_HOST) {
      return viewFace(req, { BUCKET: env.BUCKET, SIGNING_KEY: env.SIGNING_KEY, LIST_ORIGIN: listOrigin });
    }
    return error(421, "host", "この tray のホスト名でない");
  },
} satisfies ExportedHandler<LocalEnv>;
