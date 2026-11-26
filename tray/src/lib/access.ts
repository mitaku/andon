// Cloudflare Access の JWT を Worker でも確かめる（ADR-0064 決めたこと 17）。
// 見るのは、署名（チームの公開鍵）・アルゴリズム（RS256 だけ）・aud・iss・期限。
// Access の設定の値が無ければ、どの要求も断る（検査を通さない組み立ては src/local.ts にだけ置く）。

import type { AccessEnv } from "./env";
import { error } from "./http";

interface JWK {
  kid?: string;
  kty: string;
  n?: string;
  e?: string;
  alg?: string;
}

// 公開鍵の控え。チームのドメインごとに、取った時刻と鍵を持つ。知らない kid が来たら1度だけ取り直す
const cache = new Map<string, { at: number; keys: JWK[] }>();
const CACHE_MS = 60 * 60 * 1000;

export function clearAccessKeyCache() {
  cache.clear();
}

function b64urlBytes(s: string): Uint8Array {
  const bin = atob(s.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (s.length % 4)) % 4));
  return Uint8Array.from(bin, (c) => c.charCodeAt(0));
}

function b64urlJSON(s: string): Record<string, unknown> | null {
  try {
    const v = JSON.parse(new TextDecoder().decode(b64urlBytes(s)));
    return v && typeof v === "object" && !Array.isArray(v) ? v : null;
  } catch {
    return null;
  }
}

async function keys(team: string, fetcher: typeof fetch, refresh: boolean): Promise<JWK[]> {
  const hit = cache.get(team);
  if (hit && !refresh && Date.now() - hit.at < CACHE_MS) return hit.keys;
  const res = await fetcher(`${team}/cdn-cgi/access/certs`);
  if (!res.ok) throw new Error(`Access の公開鍵を取れない: ${res.status}`);
  const body = (await res.json()) as { keys?: JWK[] };
  const got = Array.isArray(body.keys) ? body.keys : [];
  cache.set(team, { at: Date.now(), keys: got });
  return got;
}

export type AccessResult = { ok: true; email?: string } | { ok: false; reason: string };

// verifyAccessJWT: JWT を確かめる。nowSeconds と fetcher は検査が差し替える口
export async function verifyAccessJWT(
  token: string | null,
  env: AccessEnv,
  nowSeconds = Math.floor(Date.now() / 1000),
  fetcher: typeof fetch = fetch,
): Promise<AccessResult> {
  const team = env.ACCESS_TEAM_DOMAIN?.replace(/\/+$/, "");
  const aud = env.ACCESS_AUD;
  if (!team || !aud) return { ok: false, reason: "Access の設定（ACCESS_TEAM_DOMAIN・ACCESS_AUD）が無い" };
  if (!token) return { ok: false, reason: "Access の JWT が無い" };
  const parts = token.split(".");
  if (parts.length !== 3) return { ok: false, reason: "JWT の形でない" };
  const [h, p, s] = parts as [string, string, string];
  const header = b64urlJSON(h);
  const payload = b64urlJSON(p);
  if (!header || !payload) return { ok: false, reason: "JWT を読めない" };
  if (header.alg !== "RS256") return { ok: false, reason: `アルゴリズムが RS256 でない: ${String(header.alg)}` };
  const kid = typeof header.kid === "string" ? header.kid : undefined;

  let sig: Uint8Array;
  try {
    sig = b64urlBytes(s);
  } catch {
    return { ok: false, reason: "署名を読めない" };
  }
  const signed = new TextEncoder().encode(`${h}.${p}`);
  let verified = false;
  for (const refresh of [false, true]) {
    let list: JWK[];
    try {
      list = await keys(team, fetcher, refresh);
    } catch (e) {
      return { ok: false, reason: String(e instanceof Error ? e.message : e) };
    }
    const candidates = list.filter((k) => k.kty === "RSA" && (kid === undefined || k.kid === kid));
    for (const jwk of candidates) {
      const key = await crypto.subtle.importKey(
        "jwk",
        { kty: "RSA", n: jwk.n, e: jwk.e, alg: "RS256", ext: true },
        { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" },
        false,
        ["verify"],
      );
      if (await crypto.subtle.verify("RSASSA-PKCS1-v1_5", key, sig, signed)) {
        verified = true;
        break;
      }
    }
    // 鍵が見つかったのに合わないなら、取り直しても同じなので打ち切る
    if (verified || candidates.length > 0) break;
  }
  if (!verified) return { ok: false, reason: "署名が合わない" };

  const auds = Array.isArray(payload.aud) ? payload.aud : [payload.aud];
  if (!auds.includes(aud)) return { ok: false, reason: "aud が違う" };
  if (payload.iss !== team) return { ok: false, reason: "iss が違う" };
  if (typeof payload.exp !== "number" || payload.exp <= nowSeconds) return { ok: false, reason: "期限が切れている" };
  if (typeof payload.nbf === "number" && payload.nbf > nowSeconds) return { ok: false, reason: "まだ使えない JWT" };
  return { ok: true, email: typeof payload.email === "string" ? payload.email : undefined };
}

// requireAccess: 断るなら 403 の応答を、通すなら null を返す
export async function requireAccess(req: Request, env: AccessEnv, nowSeconds?: number, fetcher?: typeof fetch) {
  const r = await verifyAccessJWT(req.headers.get("Cf-Access-Jwt-Assertion"), env, nowSeconds, fetcher);
  return r.ok ? null : error(403, "access", r.reason);
}
