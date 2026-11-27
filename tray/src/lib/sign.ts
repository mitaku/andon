// 報告の面の署名付き URL（ADR-0064 決めたこと 18）。署名は HMAC-SHA256 で、対象は R2 のキー（報告の id と中身の sha256）と期限。期限は5分。

export const SIGNED_TTL_SECONDS = 5 * 60;

async function key(secret: string): Promise<CryptoKey> {
  if (!secret) throw new Error("署名の鍵（SIGNING_KEY）が無い");
  return crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, [
    "sign",
    "verify",
  ]);
}

function message(key: string, exp: number): Uint8Array {
  return new TextEncoder().encode(`${key}\n${exp}`);
}

function b64url(bytes: ArrayBuffer): string {
  let s = "";
  for (const b of new Uint8Array(bytes)) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function fromB64url(s: string): Uint8Array | null {
  if (!/^[A-Za-z0-9_-]+$/.test(s)) return null;
  try {
    const bin = atob(s.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (s.length % 4)) % 4));
    return Uint8Array.from(bin, (c) => c.charCodeAt(0));
  } catch {
    return null;
  }
}

// signedURL: 報告の面で R2 のキー（<id>/<sha256>）を開く URL。nowSeconds は検査が時刻を決めるための口
export async function signedURL(viewOrigin: string, objKey: string, secret: string, nowSeconds = Math.floor(Date.now() / 1000)) {
  const exp = nowSeconds + SIGNED_TTL_SECONDS;
  const sig = b64url(await crypto.subtle.sign("HMAC", await key(secret), message(objKey, exp)));
  return `${viewOrigin}/r/${objKey}?e=${exp}&s=${sig}`;
}

export type SignCheck = "ok" | "bad" | "expired";

// checkSignature: 署名が R2 のキーと期限に合うかを先に見て、合うときだけ期限を見る（期限の切れた本物と、偽物を分ける）
export async function checkSignature(
  objKey: string,
  e: string | null,
  s: string | null,
  secret: string,
  nowSeconds = Math.floor(Date.now() / 1000),
): Promise<SignCheck> {
  if (!e || !s || !/^[0-9]{1,12}$/.test(e)) return "bad";
  const sig = fromB64url(s);
  if (!sig) return "bad";
  const exp = Number(e);
  const ok = await crypto.subtle.verify("HMAC", await key(secret), sig, message(objKey, exp));
  if (!ok) return "bad";
  return exp > nowSeconds ? "ok" : "expired";
}
