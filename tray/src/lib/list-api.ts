// 一覧の面の API（ADR-0064 決めたこと 17〜21）。Access の確かめは呼ぶ側（src/list.ts）がする。
//   GET  /api/reports?view=unread|recent|old  報告の一覧（既定は未読。old は最後に上がってから 30 日を過ぎたもの）
//   POST /api/reports/<id>/open               既読を書き、報告の面の署名付き URL を返す
//   GET  /api/cells                           セルの一覧（鍵の有無と、直近7日に上がった枚数と開いた枚数）
//   POST /api/cells/<セル>/key                 鍵を発行し直す（鍵は1度だけ返す。前の鍵は効かなくなる）
//   POST /api/cells/<セル>/revoke              鍵を失効させる
// 書き込む要求は Origin が一覧の面のオリジンと同じでなければ断る。GET に書き込みを持たせない。

import type { ListEnv } from "./env";
import { error, json, LIST_HEADERS, randomHex, sha256Hex, withHeaders } from "./http";
import { openAs } from "./formats";
import { decodeName, objectKey, REPORT_ID, segments } from "./names";
import { signedURL } from "./sign";

const DAY_MS = 24 * 60 * 60 * 1000;
export const OLD_AFTER_MS = 30 * DAY_MS;
// セルの画面の枚数を数える幅（ADR-0064 決めたこと 26・価値の仮説 2）
export const COUNT_DAYS = 7;

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
  return json({ url, version: r.version, content_type: r.content_type, title: r.title, open_as: openAs(r.content_type) });
}

// cells: 鍵を持つセルと、報告を上げたセルを1つの一覧にする。鍵の無いセル（ローカルの tray のセル）は key が "none"。
// 枚数は、直近 COUNT_DAYS 日に上げた回数（上げ直しも1回に数える）と、開いた報告の数（同じ報告を何度開いても1枚）。
// andon tray status が読むアップロードの口の GET /v1/stats と同じ数え方にする
async function cells(env: ListEnv, now: number): Promise<Response> {
  const since = now - COUNT_DAYS * DAY_MS;
  const [keys, cellsWithReports, uploads, opens] = await env.DB.batch<Record<string, unknown>>([
    env.DB.prepare("SELECT cell, created_at, revoked_at FROM keys"),
    env.DB.prepare("SELECT DISTINCT cell FROM reports WHERE version > 0"),
    env.DB.prepare(
      "SELECT r.cell AS cell, COUNT(*) AS n FROM uploads u JOIN reports r ON r.id = u.report_id WHERE u.at >= ?1 GROUP BY r.cell",
    ).bind(since),
    env.DB.prepare(
      "SELECT r.cell AS cell, COUNT(DISTINCT o.report_id) AS n FROM opens o JOIN reports r ON r.id = o.report_id WHERE o.at >= ?1 GROUP BY r.cell",
    ).bind(since),
  ]);
  const count = (rows: Record<string, unknown>[] | undefined) =>
    new Map((rows ?? []).map((r) => [String(r.cell), Number(r.n)]));
  const up = count(uploads?.results);
  const op = count(opens?.results);
  const byCell = new Map<string, { created_at: string | null; key: "active" | "revoked" | "none" }>();
  for (const r of cellsWithReports?.results ?? []) byCell.set(String(r.cell), { created_at: null, key: "none" });
  for (const r of keys?.results ?? []) {
    byCell.set(String(r.cell), {
      created_at: new Date(Number(r.created_at)).toISOString(),
      key: r.revoked_at === null ? "active" : "revoked",
    });
  }
  return json({
    days: COUNT_DAYS,
    cells: [...byCell.entries()]
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([cell, c]) => ({
        cell,
        created_at: c.created_at,
        key: c.key,
        revoked: c.key === "revoked",
        uploads: up.get(cell) ?? 0,
        opens: op.get(cell) ?? 0,
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
    return req.method === "GET" ? cells(env, now) : error(405, "method", "GET だけ");
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
