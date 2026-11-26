// 報告の面（受け入れ条件 12）と、一覧の面の画面のファイルのヘッダ
import { env } from "cloudflare:test";
import { beforeEach, describe, expect, it } from "vitest";
import { LIST_HEADERS } from "../src/lib/http";
import { SIGNED_TTL_SECONDS, signedURL } from "../src/lib/sign";
import { viewFace } from "../src/lib/view-face";
import view from "../src/view";
import { LIST_ORIGIN, SIGNING_KEY, VIEW_ORIGIN, viewEnv } from "./helpers";

const A = "a".repeat(32);
const B = "b".repeat(32);

beforeEach(async () => {
  await env.BUCKET.put(A, "<!doctype html><p>A の報告</p>", { httpMetadata: { contentType: "text/html; charset=utf-8" } });
  await env.BUCKET.put(B, "# B の報告\n\n<b>太字</b>", { httpMetadata: { contentType: "text/markdown" } });
});

// now を渡すときは、時刻を決められる viewFace を直に打つ
const get = (url: string, now?: number) =>
  now === undefined ? view.fetch(new Request(url), viewEnv()) : viewFace(new Request(url), viewEnv(), now);
const now = () => Math.floor(Date.now() / 1000);

describe("署名（受け入れ条件 12）", () => {
  it("署名付き URL で開け、報告の面のヘッダが付く", async () => {
    const res = await get(await signedURL(VIEW_ORIGIN, A, SIGNING_KEY));
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("A の報告");
    const csp = res.headers.get("Content-Security-Policy") ?? "";
    for (const d of [
      "default-src 'none'",
      "connect-src 'none'",
      "form-action 'none'",
      "base-uri 'none'",
      `frame-ancestors ${LIST_ORIGIN}`,
      "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox",
    ]) {
      expect(csp).toContain(d);
    }
    expect(csp).not.toContain("allow-same-origin");
    expect(csp).not.toContain("allow-top-navigation");
    expect(res.headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(res.headers.get("Referrer-Policy")).toBe("no-referrer");
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(res.headers.get("X-Robots-Tag")).toBe("noindex");
  });

  it("署名の無い URL を断る", async () => {
    expect((await get(`${VIEW_ORIGIN}/r/${A}`)).status).toBe(403);
    const u = new URL(await signedURL(VIEW_ORIGIN, A, SIGNING_KEY));
    u.searchParams.delete("s");
    expect((await get(u.href)).status).toBe(403);
  });

  it("期限の切れた URL を断り、開き直すよう促す", async () => {
    const issued = now() - SIGNED_TTL_SECONDS - 1;
    const res = await get(await signedURL(VIEW_ORIGIN, A, SIGNING_KEY, issued));
    expect(res.status).toBe(410);
    const text = await res.text();
    expect(text).toContain("開き直す");
    expect(text).not.toContain("A の報告");
  });

  it("期限の5分の内は開け、過ぎたら断る", async () => {
    const url = await signedURL(VIEW_ORIGIN, A, SIGNING_KEY, 1_000_000);
    expect((await get(url, 1_000_000 + SIGNED_TTL_SECONDS - 1)).status).toBe(200);
    expect((await get(url, 1_000_000 + SIGNED_TTL_SECONDS)).status).toBe(410);
  });

  it("別の報告の署名を付けた URL を断る", async () => {
    const forB = new URL(await signedURL(VIEW_ORIGIN, B, SIGNING_KEY));
    const res = await get(`${VIEW_ORIGIN}/r/${A}${forB.search}`);
    expect(res.status).toBe(403);
    expect(await res.text()).not.toContain("A の報告");
  });

  it("期限を延ばした URL と、別の鍵で署名した URL を断る", async () => {
    const u = new URL(await signedURL(VIEW_ORIGIN, A, SIGNING_KEY));
    u.searchParams.set("e", String(Number(u.searchParams.get("e")) + 3600));
    expect((await get(u.href)).status).toBe(403);
    expect((await get(await signedURL(VIEW_ORIGIN, A, "other-key"))).status).toBe(403);
  });

  it("id の形でないパスを断る", async () => {
    expect((await get(`${VIEW_ORIGIN}/r/..%2F${A}`)).status).toBe(404);
    expect((await get(`${VIEW_ORIGIN}/x/${A}`)).status).toBe(404);
  });
});

describe("形式", () => {
  it("Markdown は HTML の型に入れ、中の HTML を消さない", async () => {
    const res = await get(await signedURL(VIEW_ORIGIN, B, SIGNING_KEY));
    expect(res.headers.get("Content-Type")).toBe("text/html; charset=utf-8");
    const text = await res.text();
    expect(text).toContain("<h1>B の報告</h1>");
    expect(text).toContain("<b>太字</b>");
  });

  it("テキストは文字を逃がして <pre> に入れる", async () => {
    const id = "c".repeat(32);
    await env.BUCKET.put(id, "<script>x</script>", { httpMetadata: { contentType: "text/plain" } });
    const text = await (await get(await signedURL(VIEW_ORIGIN, id, SIGNING_KEY))).text();
    expect(text).toContain("<pre>&#60;script&#62;x&#60;/script&#62;</pre>");
  });

  it("開かない形式はダウンロードにする", async () => {
    const id = "d".repeat(32);
    await env.BUCKET.put(id, "a,b", { httpMetadata: { contentType: "text/csv" } });
    const res = await get(await signedURL(VIEW_ORIGIN, id, SIGNING_KEY));
    expect(res.headers.get("Content-Disposition")).toBe("attachment");
    expect(res.headers.get("Content-Security-Policy")).toContain("sandbox");
  });
});

describe("一覧の面の画面のファイル", () => {
  it("public/_headers は、Worker の応答と同じヘッダを全部のパスに付ける", () => {
    const lines = env.HEADERS_FILE.split("\n").filter((l) => l.trim() !== "");
    expect(lines[0]).toBe("/*");
    const got = Object.fromEntries(
      lines.slice(1).map((l) => {
        const i = l.indexOf(":");
        return [l.slice(0, i).trim(), l.slice(i + 1).trim()];
      }),
    );
    expect(got).toEqual(LIST_HEADERS);
  });
});
