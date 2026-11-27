// アップロードの口の入口（Cloudflare の tray。ADR-0064 決めたこと 15・16・19）。Authorization: Bearer の API キーでセルを決める。
// 束縛: DB・BUCKET・LIST_ORIGIN（署名の鍵には届かない）

import type { UploadEnv } from "./lib/env";
import { error } from "./lib/http";
import { cellForKey, meta, uploadAPI } from "./lib/upload-api";

export default {
  async fetch(req: Request, env: UploadEnv): Promise<Response> {
    const url = new URL(req.url);
    if (url.pathname === "/v1/meta" && req.method === "GET") return meta();
    const cell = await cellForKey(req, env.DB);
    if (cell === null) return error(401, "key", "API キーが無いか、知らないか、失効している");
    return uploadAPI(req, env, cell);
  },
} satisfies ExportedHandler<UploadEnv>;
