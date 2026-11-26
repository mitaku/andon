import { applyD1Migrations, env } from "cloudflare:test";
import { beforeEach } from "vitest";
import { clearAccessKeyCache } from "../src/lib/access";

await applyD1Migrations(env.DB, env.TEST_MIGRATIONS);

// D1 と R2 は検査のファイルの中で共有されるので、検査ごとに空にする
beforeEach(async () => {
  await env.DB.batch(["reports", "uploads", "opens", "keys"].map((t) => env.DB.prepare(`DELETE FROM ${t}`)));
  const listed = await env.BUCKET.list();
  if (listed.objects.length > 0) await env.BUCKET.delete(listed.objects.map((o) => o.key));
  clearAccessKeyCache();
});
