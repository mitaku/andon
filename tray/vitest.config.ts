// Workers の検査（ADR-0064 決めたこと 28）。@cloudflare/vitest-plugin で、検査を workerd の上で回す。
// D1 と R2 は検査のファイルごとに切り離され、setup が移行を当てる。
import { readFileSync } from "node:fs";
import { cloudflareTest, readD1Migrations } from "@cloudflare/vitest-plugin";
import { defineConfig } from "vitest/config";

export default defineConfig(async () => ({
  plugins: [
    cloudflareTest({
      miniflare: {
        compatibilityDate: "2026-11-01",
        d1Databases: ["DB"],
        r2Buckets: ["BUCKET"],
        bindings: {
          TEST_MIGRATIONS: await readD1Migrations("migrations"),
          // 画面のファイルに付けるヘッダ（Cloudflare では Worker を通らない）。src/lib/http.ts の LIST_HEADERS と突き合わせる
          HEADERS_FILE: readFileSync("public/_headers", "utf8"),
        },
      },
    }),
  ],
  test: {
    include: ["test/**/*.test.ts"],
    setupFiles: ["test/setup.ts"],
  },
}));
