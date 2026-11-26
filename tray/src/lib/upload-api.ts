// アップロードの口（ADR-0064 決めたこと 15）。約束の見本は contract/upload-v1.json。
//   PUT /v1/reports/<プロジェクト>/<名前>  本文はファイルの中身。プロジェクトが無いときは "-"
//     Content-Type    中身の種類（必須）
//     X-Tray-Kind     publish・digest・report（必須）
//     X-Tray-Title    タイトル。UTF-8 をパーセントで符号化（必須）
//     X-Tray-Sha256   本文の sha256 の16進（必須。本文と合わなければ断る）
//   答え: {"url": 開く URL, "version": バージョンの番号, "bumped": バージョンが上がったか}
//   GET /v1/meta  tray のバージョン
// セルは呼ぶ側が決める（Cloudflare の tray は API キー〈src/upload.ts〉、ローカルの tray はセルの名前〈src/local-upload.ts〉）。
// 本文にセルの名前を書かせない。欄は足すだけで変え、消すか意味を変えるときは /v2/ を足す。

import pkg from "../../package.json";
import type { UploadEnv } from "./env";
import { error, json, randomHex, sha256Hex } from "./http";
import { decodeHeader, decodeName, segments } from "./names";

export const MAX_BYTES = 20 * 1024 * 1024;
export const KINDS = ["publish", "digest", "report"] as const;

export function meta(): Response {
  return json({ version: pkg.version });
}

// cellForKey: Authorization: Bearer の鍵からセルを引く。失効した鍵・知らない鍵は null
export async function cellForKey(req: Request, db: D1Database): Promise<string | null> {
  const m = /^Bearer ([A-Za-z0-9_-]{16,200})$/.exec(req.headers.get("Authorization") ?? "");
  if (!m) return null;
  const hash = await sha256Hex(m[1] as string);
  const r = await db.prepare("SELECT cell FROM keys WHERE hash = ?1 AND revoked_at IS NULL").bind(hash).first<{ cell: string }>();
  return r?.cell ?? null;
}

async function put(req: Request, env: UploadEnv, cell: string, project: string, name: string, now: number) {
  const kind = req.headers.get("X-Tray-Kind");
  if (!kind || !(KINDS as readonly string[]).includes(kind)) return error(400, "kind", "X-Tray-Kind は publish・digest・report のどれか");
  const title = decodeHeader(req.headers.get("X-Tray-Title"));
  if (title === null) return error(400, "title", "X-Tray-Title が無いか読めない");
  const contentType = req.headers.get("Content-Type");
  if (!contentType) return error(400, "content_type", "Content-Type が無い");
  const want = req.headers.get("X-Tray-Sha256");
  if (!want || !/^[0-9a-f]{64}$/.test(want)) return error(400, "sha256", "X-Tray-Sha256 が無いか16進の64字でない");
  const declared = Number(req.headers.get("Content-Length") ?? "0");
  if (declared > MAX_BYTES) return error(413, "too_large", "20 MiB を超える");
  const body = await req.arrayBuffer();
  if (body.byteLength > MAX_BYTES) return error(413, "too_large", "20 MiB を超える");
  if ((await sha256Hex(body)) !== want) return error(400, "sha256", "X-Tray-Sha256 が本文と合わない");

  // id だけを先に取る（version 0 の行は一覧に出ない）。同じ組が在れば、その id を使う
  await env.DB.prepare(
    `INSERT INTO reports (id, cell, project, name, updated_at) VALUES (?1, ?2, ?3, ?4, ?5)
     ON CONFLICT (cell, project, name) DO NOTHING`,
  )
    .bind(randomHex(), cell, project, name, now)
    .run();
  const cur = await env.DB.prepare("SELECT id, sha256, version FROM reports WHERE cell = ?1 AND project = ?2 AND name = ?3")
    .bind(cell, project, name)
    .first<{ id: string; sha256: string; version: number }>();
  if (!cur) return error(500, "internal", "報告の行を作れない");
  const url = `${env.LIST_ORIGIN}/r/${cur.id}`;
  // 中身が同じ出し直しはバージョンを上げない（読み終えた物を未読に戻さない）。同じ名前への上げが重なると、D1 の sha256 と
  // R2 の中身が食い違って残りうるので、R2 の中身の sha256 も合うときだけ飛ばす（出し直しで食い違いが直る）
  if (cur.version > 0 && cur.sha256 === want && (await env.BUCKET.head(cur.id))?.customMetadata?.sha256 === want) {
    return json({ url, version: cur.version, bumped: false });
  }

  // R2 を先に書き、あとで D1 のバージョンを上げる（中身の無いバージョンを一覧に出さない）
  await env.BUCKET.put(cur.id, body, { httpMetadata: { contentType }, customMetadata: { sha256: want } });
  const up = await env.DB.prepare(
    `UPDATE reports SET version = version + 1, sha256 = ?2, title = ?3, kind = ?4, content_type = ?5, size = ?6, updated_at = ?7
     WHERE id = ?1 RETURNING version`,
  )
    .bind(cur.id, want, title, kind, contentType, body.byteLength, now)
    .first<{ version: number }>();
  if (!up) return error(500, "internal", "バージョンを上げられない");
  await env.DB.prepare("INSERT INTO uploads (report_id, version, at) VALUES (?1, ?2, ?3)").bind(cur.id, up.version, now).run();
  return json({ url, version: up.version, bumped: true });
}

// uploadAPI: セルが決まったあとの要求に答える。now は検査が時刻を決めるための口
export async function uploadAPI(req: Request, env: UploadEnv, cell: string, now = Date.now()): Promise<Response> {
  const seg = segments(new URL(req.url).pathname);
  if (seg[0] !== "v1") return error(404, "not_found", "無い");
  if (seg.length === 2 && seg[1] === "meta") return req.method === "GET" ? meta() : error(405, "method", "GET だけ");
  if (seg.length === 4 && seg[1] === "reports") {
    if (req.method !== "PUT") return error(405, "method", "PUT だけ");
    const project = decodeName(seg[2] ?? "");
    const name = decodeName(seg[3] ?? "");
    if (project === null) return error(400, "project", "プロジェクトの名前を読めない");
    if (name === null) return error(400, "name", "報告の名前を読めない");
    return put(req, env, cell, project, name, now);
  }
  return error(404, "not_found", "無い");
}
