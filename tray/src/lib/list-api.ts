// 一覧の面の API（ADR-0064 決めたこと 17〜21）。Access の確かめは呼ぶ側（src/list.ts）がする。
//   GET  /api/reports?view=unread|recent|old  報告の一覧（既定は未読。old は最後に上がってから 30 日を過ぎたもの）
//   POST /api/reports/<id>/open               既読を書き、報告の面の署名付き URL を返す
//   GET  /api/cells                           セルの一覧
//   POST /api/cells/<セル>/key                 鍵を発行し直す（鍵は1度だけ返す。前の鍵は効かなくなる）
//   POST /api/cells/<セル>/revoke              鍵を失効させる
// 書き込む要求は Origin が一覧の面のオリジンと同じでなければ断る。GET に書き込みを持たせない。

import type { ListEnv } from "./env";
import { error, json, LIST_HEADERS, randomHex, sha256Hex, withHeaders } from "./http";
import { decodeName, objectKey, REPORT_ID, segments } from "./names";
import { signedURL } from "./sign";

export const OLD_AFTER_MS = 30 * 24 * 60 * 60 * 1000;

interface ReportRow {
  id: string;
  cell: string;
  project: string;
  name: string;
  title: string;
  kind: string;
  content_type: string;
  version: number;
  read_version: number;
  updated_at: number;
}

function row(r: ReportRow) {
  return {
    id: r.id,
    title: r.title,
    cell: r.cell,
    project: r.project,
    name: r.name,
    kind: r.kind,
    content_type: r.content_type,
    version: r.version,
    unread: r.read_version < r.version,
    updated_at: new Date(r.updated_at).toISOString(),
  };
}

async function listReports(url: URL, env: ListEnv, now: number): Promise<Response> {
  const view = url.searchParams.get("view") ?? "unread";
  const since = now - OLD_AFTER_MS;
  let where: string;
  if (view === "unread") where = "updated_at >= ?1 AND read_version < version";
  else if (view === "recent") where = "updated_at >= ?1";
  else if (view === "old") where = "updated_at < ?1";
  else return error(400, "view", "view は unread・recent・old のどれか");
  const { results } = await env.DB.prepare(
    `SELECT id, cell, project, name, title, kind, content_type, version, read_version, updated_at FROM reports
     WHERE version > 0 AND ${where} ORDER BY cell, project, updated_at DESC`,
  )
    .bind(since)
    .all<ReportRow>();
  return json({ view, reports: results.map(row) });
}

async function openReport(id: string, env: ListEnv, now: number): Promise<Response> {
  if (!REPORT_ID.test(id)) return error(404, "not_found", "報告が無い");
  const r = await env.DB.prepare(
    "UPDATE reports SET read_version = version WHERE id = ?1 AND version > 0 RETURNING version, sha256, content_type, title",
  )
    .bind(id)
    .first<{ version: number; sha256: string; content_type: string; title: string }>();
  if (!r) return error(404, "not_found", "報告が無い");
  await env.DB.prepare("INSERT INTO opens (report_id, version, at) VALUES (?1, ?2, ?3)").bind(id, r.version, now).run();
  // 開いた時点の中身（D1 が指す sha256）に結び付けた URL を返す
  const url = await signedURL(env.VIEW_ORIGIN, objectKey(id, r.sha256), env.SIGNING_KEY, Math.floor(now / 1000));
  return json({ url, version: r.version, content_type: r.content_type, title: r.title });
}

async function cells(env: ListEnv): Promise<Response> {
  const { results } = await env.DB.prepare("SELECT cell, created_at, revoked_at FROM keys ORDER BY cell").all<{
    cell: string;
    created_at: number;
    revoked_at: number | null;
  }>();
  return json({
    cells: results.map((c) => ({
      cell: c.cell,
      created_at: new Date(c.created_at).toISOString(),
      revoked: c.revoked_at !== null,
    })),
  });
}

async function issueKey(cell: string, env: ListEnv, now: number): Promise<Response> {
  const key = `tray_${randomHex(32)}`;
  const hash = await sha256Hex(key);
  await env.DB.prepare(
    `INSERT INTO keys (cell, hash, created_at, revoked_at) VALUES (?1, ?2, ?3, NULL)
     ON CONFLICT (cell) DO UPDATE SET hash = excluded.hash, created_at = excluded.created_at, revoked_at = NULL`,
  )
    .bind(cell, hash, now)
    .run();
  return json({ cell, key });
}

async function revokeKey(cell: string, env: ListEnv, now: number): Promise<Response> {
  const r = await env.DB.prepare("UPDATE keys SET revoked_at = ?2 WHERE cell = ?1 RETURNING cell").bind(cell, now).first();
  return r ? json({ cell, revoked: true }) : error(404, "not_found", "そのセルの鍵が無い");
}

async function route(req: Request, env: ListEnv, now: number): Promise<Response> {
  const url = new URL(req.url);
  const seg = segments(url.pathname);
  if (seg[0] !== "api") return error(404, "not_found", "無い");
  const write = req.method !== "GET" && req.method !== "HEAD";
  if (write && req.headers.get("Origin") !== url.origin) return error(403, "origin", "Origin が一覧の面のオリジンでない");

  if (seg.length === 2 && seg[1] === "reports") {
    return req.method === "GET" ? listReports(url, env, now) : error(405, "method", "GET だけ");
  }
  if (seg.length === 4 && seg[1] === "reports" && seg[3] === "open") {
    return req.method === "POST" ? openReport(seg[2] ?? "", env, now) : error(405, "method", "POST だけ");
  }
  if (seg.length === 2 && seg[1] === "cells") {
    return req.method === "GET" ? cells(env) : error(405, "method", "GET だけ");
  }
  if (seg.length === 4 && seg[1] === "cells" && (seg[3] === "key" || seg[3] === "revoke")) {
    if (req.method !== "POST") return error(405, "method", "POST だけ");
    const cell = decodeName(seg[2] ?? "");
    if (cell === null || cell === "-") return error(400, "cell", "セルの名前を読めない");
    return seg[3] === "key" ? issueKey(cell, env, now) : revokeKey(cell, env, now);
  }
  return error(404, "not_found", "無い");
}

// listAPI: /api/ の下の要求に答える。応答には一覧の面のヘッダを付ける。now は検査が時刻を決めるための口
export async function listAPI(req: Request, env: ListEnv, now = Date.now()): Promise<Response> {
  return withHeaders(await route(req, env, now), LIST_HEADERS);
}
