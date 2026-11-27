// 面に共通の応答の組み立て。

// 一覧の面が返す全部の応答に付けるヘッダ（決めたこと 18）。画面のファイルは Worker を通らないので、
// Cloudflare では public/_headers が同じものを付ける（test/view.test.ts が突き合わせる）。
export const LIST_HEADERS: Record<string, string> = {
  "Content-Security-Policy": "frame-ancestors 'none'",
  "X-Content-Type-Options": "nosniff",
  "Referrer-Policy": "no-referrer",
};

export function json(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store", ...headers },
  });
}

// error: 機械が読む理由（短い英字）と、人が読む1行を JSON で返す
export function error(status: number, code: string, message: string, headers: Record<string, string> = {}): Response {
  return json({ error: code, message }, status, headers);
}

export function withHeaders(res: Response, headers: Record<string, string>): Response {
  const out = new Response(res.body, res);
  for (const [k, v] of Object.entries(headers)) out.headers.set(k, v);
  return out;
}

export function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}

export async function sha256Hex(data: ArrayBuffer | Uint8Array | string): Promise<string> {
  const bytes = typeof data === "string" ? new TextEncoder().encode(data) : data;
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// randomId: 報告の id（R2 のキー）と API キーの種。16 バイトの乱数を16進で
export function randomHex(bytes = 16): string {
  const b = crypto.getRandomValues(new Uint8Array(bytes));
  return [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
}
