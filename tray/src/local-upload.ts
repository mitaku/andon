// ローカルの tray のアップロードの口（セルの中だけ。デプロイの設定に載せない。ADR-0064 決めたこと 15）。
// 一覧の面と別の番号で 127.0.0.1 にだけ開き、portless の名前を付けない。鍵は求めず、セルは起動の設定（CELL）で決まる。
// 束縛: DB・BUCKET・LIST_ORIGIN・CELL

import type { UploadEnv } from "./lib/env";
import { uploadAPI } from "./lib/upload-api";

export interface LocalUploadEnv extends UploadEnv {
  CELL: string;
}

export default {
  fetch(req: Request, env: LocalUploadEnv): Promise<Response> {
    return uploadAPI(req, env, env.CELL);
  },
} satisfies ExportedHandler<LocalUploadEnv>;
