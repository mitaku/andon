// Workers の検査に共通の道具。
import { env } from "cloudflare:test";
import { sha256Hex } from "../src/lib/http";

export const LIST_ORIGIN = "https://tray.example.com";
export const VIEW_ORIGIN = "https://tray-view.example.com";
export const UPLOAD_ORIGIN = "https://tray-up.example.com";
export const SIGNING_KEY = "test-signing-key";
export const TEAM = "https://team.cloudflareaccess.com";
export const AUD = "test-aud";

export const listEnv = () => ({ DB: env.DB, SIGNING_KEY, VIEW_ORIGIN, ACCESS_TEAM_DOMAIN: TEAM, ACCESS_AUD: AUD });
export const viewEnv = () => ({ BUCKET: env.BUCKET, SIGNING_KEY, LIST_ORIGIN });
export const uploadEnv = () => ({ DB: env.DB, BUCKET: env.BUCKET, LIST_ORIGIN });

// putKey: セルの鍵を表に直に置く（発行の口は list.test.ts が見る）
export async function putKey(cell: string, key: string, revoked = false) {
  await env.DB.prepare("INSERT OR REPLACE INTO keys (cell, hash, created_at, revoked_at) VALUES (?1, ?2, ?3, ?4)")
    .bind(cell, await sha256Hex(key), Date.now(), revoked ? Date.now() : null)
    .run();
}

export interface PutOptions {
  project?: string;
  name: string;
  body: string;
  key?: string;
  kind?: string;
  title?: string;
  contentType?: string;
}

// putRequest: アップロードの口への要求を作る（名前とタイトルはパーセントで符号化する）
export async function putRequest(o: PutOptions): Promise<Request> {
  const headers: Record<string, string> = {
    "Content-Type": o.contentType ?? "text/markdown; charset=utf-8",
    "X-Tray-Kind": o.kind ?? "publish",
    "X-Tray-Title": encodeURIComponent(o.title ?? o.name),
    "X-Tray-Sha256": await sha256Hex(o.body),
  };
  if (o.key) headers.Authorization = `Bearer ${o.key}`;
  const path = `/v1/reports/${encodeURIComponent(o.project ?? "andon")}/${encodeURIComponent(o.name)}`;
  return new Request(`${UPLOAD_ORIGIN}${path}`, { method: "PUT", headers, body: o.body });
}

function b64url(bytes: ArrayBuffer | Uint8Array): string {
  let s = "";
  for (const b of new Uint8Array(bytes)) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

const enc = (v: unknown) => b64url(new TextEncoder().encode(JSON.stringify(v)));

export interface Signer {
  jwks: { keys: JsonWebKey[] };
  sign(payload: Record<string, unknown>, header?: Record<string, unknown>): Promise<string>;
}

// signer: Access のチームの鍵の代わり。kid を付けた RSA の鍵で JWT を作る
export async function signer(kid = "k1"): Promise<Signer> {
  const pair = (await crypto.subtle.generateKey(
    { name: "RSASSA-PKCS1-v1_5", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
    true,
    ["sign", "verify"],
  )) as CryptoKeyPair;
  const pub = (await crypto.subtle.exportKey("jwk", pair.publicKey)) as JsonWebKey;
  return {
    jwks: { keys: [{ ...pub, kid, alg: "RS256", use: "sig" } as JsonWebKey] },
    async sign(payload, header = {}) {
      const h = enc({ alg: "RS256", kid, typ: "JWT", ...header });
      const p = enc(payload);
      const sig = await crypto.subtle.sign("RSASSA-PKCS1-v1_5", pair.privateKey, new TextEncoder().encode(`${h}.${p}`));
      return `${h}.${p}.${b64url(sig)}`;
    },
  };
}

export const nowSeconds = () => Math.floor(Date.now() / 1000);

export function goodClaims(extra: Record<string, unknown> = {}) {
  return { aud: [AUD], iss: TEAM, exp: nowSeconds() + 600, iat: nowSeconds(), email: "owner@example.com", ...extra };
}
