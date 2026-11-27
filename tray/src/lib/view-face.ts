// 報告の面（ADR-0064 決めたこと 18・22）。一覧の面と別のオリジンで、署名付き URL の報告だけを返す。
//   GET /r/<id>/<中身の sha256>?e=<期限>&s=<署名>
// HTML・Markdown・テキスト・画像・PDF は開き、ほかは Content-Disposition: attachment で返す。
// Markdown とテキストは決まった HTML の型に入れる（Markdown の中の HTML は消さない。sandbox で出すので HTML の報告と同じ扱い）。

import { marked } from "marked";
import type { ViewEnv } from "./env";
import { baseType, openAs } from "./formats";
import { escapeHTML } from "./http";
import { objectKey, REPORT_ID, SHA256, segments } from "./names";
import { checkSignature } from "./sign";

const SANDBOX = "allow-scripts allow-popups allow-popups-to-escape-sandbox";

export function viewHeaders(listOrigin: string): Record<string, string> {
  const csp = [
    "default-src 'none'",
    "script-src 'unsafe-inline' data:",
    "style-src 'unsafe-inline' data:",
    "img-src data: blob:",
    "media-src data: blob:",
    "font-src data:",
    "connect-src 'none'",
    "form-action 'none'",
    "base-uri 'none'",
    `frame-ancestors ${listOrigin}`,
    `sandbox ${SANDBOX}`,
  ].join("; ");
  return {
    "Content-Security-Policy": csp,
    "X-Content-Type-Options": "nosniff",
    "Referrer-Policy": "no-referrer",
    "Cache-Control": "no-store",
    "X-Robots-Tag": "noindex",
  };
}

function page(title: string, body: string): string {
  return `<!doctype html><html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${escapeHTML(title)}</title>
<style>body{font:16px/1.7 system-ui,sans-serif;max-width:46rem;margin:0 auto;padding:1rem;color-scheme:light dark}pre{white-space:pre-wrap;word-break:break-word}img{max-width:100%}table{border-collapse:collapse}td,th{border:1px solid #8888;padding:.2rem .5rem}</style></head><body>${body}</body></html>`;
}

function shortPage(status: number, title: string, message: string, listOrigin: string): Response {
  return new Response(page(title, `<p>${escapeHTML(message)}</p>`), {
    status,
    headers: { ...viewHeaders(listOrigin), "Content-Type": "text/html; charset=utf-8" },
  });
}

// viewFace: 報告の面の要求に答える。nowSeconds は検査が時刻を決めるための口
export async function viewFace(req: Request, env: ViewEnv, nowSeconds?: number): Promise<Response> {
  const url = new URL(req.url);
  const seg = segments(url.pathname);
  if (req.method !== "GET" && req.method !== "HEAD") return shortPage(405, "使えない", "GET だけ", env.LIST_ORIGIN);
  if (seg.length !== 3 || seg[0] !== "r" || !REPORT_ID.test(seg[1] ?? "") || !SHA256.test(seg[2] ?? "")) {
    return shortPage(404, "無い", "報告が無い", env.LIST_ORIGIN);
  }
  const key = objectKey(seg[1] as string, seg[2] as string);
  const check = await checkSignature(key, url.searchParams.get("e"), url.searchParams.get("s"), env.SIGNING_KEY, nowSeconds);
  if (check === "bad") return shortPage(403, "開けない", "署名が合わない", env.LIST_ORIGIN);
  if (check === "expired") {
    return shortPage(410, "期限切れ", "この URL の期限が切れた。一覧から開き直す。", env.LIST_ORIGIN);
  }
  const obj = await env.BUCKET.get(key);
  if (!obj) return shortPage(404, "無い", "報告が無い", env.LIST_ORIGIN);
  const contentType = obj.httpMetadata?.contentType ?? "application/octet-stream";
  const base = baseType(contentType);
  const headers: Record<string, string> = { ...viewHeaders(env.LIST_ORIGIN) };
  if (base === "text/markdown") {
    const html = await marked.parse(await obj.text(), { async: true });
    return new Response(page("報告", html), { headers: { ...headers, "Content-Type": "text/html; charset=utf-8" } });
  }
  if (base === "text/plain") {
    const html = `<pre>${escapeHTML(await obj.text())}</pre>`;
    return new Response(page("報告", html), { headers: { ...headers, "Content-Type": "text/html; charset=utf-8" } });
  }
  headers["Content-Type"] = contentType;
  if (openAs(contentType) === "download") headers["Content-Disposition"] = "attachment";
  return new Response(obj.body, { headers });
}
