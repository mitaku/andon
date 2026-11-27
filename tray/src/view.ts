// 報告の面の入口（Cloudflare の tray。ADR-0064 決めたこと 16・18）。前に何も置かず、署名付き URL だけで開く。
// 束縛: BUCKET・SIGNING_KEY・LIST_ORIGIN（D1 には届かない）

import type { ViewEnv } from "./lib/env";
import { viewFace } from "./lib/view-face";

export default {
  fetch(req: Request, env: ViewEnv): Promise<Response> {
    return viewFace(req, env);
  },
} satisfies ExportedHandler<ViewEnv>;
