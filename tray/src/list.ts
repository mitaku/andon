// 一覧の面の入口（Cloudflare の tray。ADR-0064 決めたこと 16・17）。Access の内側に置き、/api/ の下はどの要求でも
// Access の JWT を確かめる。画面のファイル（public/）は Worker を通らずに返る（public/_headers がヘッダを付ける）。
// 束縛: DB・SIGNING_KEY・VIEW_ORIGIN・ACCESS_TEAM_DOMAIN・ACCESS_AUD

import { requireAccess } from "./lib/access";
import type { AccessEnv, ListEnv } from "./lib/env";
import { error, LIST_HEADERS, withHeaders } from "./lib/http";
import { listAPI } from "./lib/list-api";

export default {
  async fetch(req: Request, env: ListEnv & AccessEnv): Promise<Response> {
    const url = new URL(req.url);
    if (!url.pathname.startsWith("/api/")) return withHeaders(error(404, "not_found", "無い"), LIST_HEADERS);
    const denied = await requireAccess(req, env);
    if (denied) return withHeaders(denied, LIST_HEADERS);
    return listAPI(req, env);
  },
} satisfies ExportedHandler<ListEnv & AccessEnv>;
