#!/usr/bin/env node
// 端から端の検査（ADR-0064 決めたこと 28）。ビルドしたローカルの tray を一時の置き場で起動し、アップロードの口から
// 一覧・既読・報告の面までを HTTP で通す。job tray の中でだけ回す（tests/ のシェルにしない）。
// 先に `bun run build` を打っておく。終了コード 0 で全部通った。
//
// セルの側の送り（andon publish）を足したら、アップロードの口を直に打つ所をその command に替える。

import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtempSync, readdirSync, readFileSync, rmSync } from "node:fs";
import { request } from "node:http";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const dir = mkdtempSync(join(tmpdir(), "tray-e2e-"));
const data = join(dir, "data");
let failed = 0;

function check(ok, what) {
  console.log(`${ok ? "ok  " : "FAIL"} ${what}`);
  if (!ok) failed++;
}

function start() {
  const proc = spawn(process.execPath, [join(root, "scripts/serve.mjs"), "--cell", "e2e", "--data", data], {
    stdio: ["ignore", "pipe", "inherit"],
  });
  return new Promise((ok, ng) => {
    let buf = "";
    proc.stdout.on("data", (d) => {
      buf += String(d);
      const i = buf.indexOf("\n");
      if (i >= 0) ok({ proc, ...JSON.parse(buf.slice(0, i)) });
    });
    proc.once("exit", (code) => ng(new Error(`serve.mjs が止まった: ${code}`)));
  });
}

function stop(t) {
  return new Promise((ok) => {
    t.proc.once("exit", ok);
    t.proc.kill("SIGTERM");
  });
}

// http: 127.0.0.1 の番号へ、Host を付けて打つ（*.localhost を名前で引かない）
function http(base, path, { method = "GET", host, headers = {}, body } = {}) {
  const u = new URL(base);
  return new Promise((ok, ng) => {
    const req = request(
      { host: u.hostname, port: u.port, path, method, headers: { ...(host ? { Host: host } : {}), ...headers } },
      (res) => {
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => {
          const text = Buffer.concat(chunks).toString("utf8");
          let json;
          try {
            json = JSON.parse(text);
          } catch {}
          ok({ status: res.statusCode, headers: res.headers, text, json });
        });
      },
    );
    req.on("error", ng);
    if (body !== undefined) req.write(body);
    req.end();
  });
}

function put(t, name, body, { kind = "publish", project = "andon", type = "text/markdown; charset=utf-8" } = {}) {
  return http(t.upload, `/v1/reports/${encodeURIComponent(project)}/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: {
      "Content-Type": type,
      "X-Tray-Kind": kind,
      "X-Tray-Title": encodeURIComponent(name),
      "X-Tray-Sha256": createHash("sha256").update(body).digest("hex"),
    },
    body,
  });
}

const hostOf = (origin) => new URL(origin).host;
const list = (t, view = "unread") => http(t.list, `/api/reports?view=${view}`, { host: hostOf(t.list_origin) });
const open = (t, id) =>
  http(t.list, `/api/reports/${id}/open`, { method: "POST", host: hostOf(t.list_origin), headers: { Origin: t.list_origin } });

// rss: serve.mjs とその子（workerd）の RSS の合計（MB）。Linux の /proc から読む
function rss(pid) {
  const kids = (p) => {
    let out = [p];
    for (const t of readdirSafe(`/proc/${p}/task`)) {
      const c = readSafe(`/proc/${p}/task/${t}/children`).trim();
      for (const k of c ? c.split(/\s+/) : []) out = out.concat(kids(Number(k)));
    }
    return out;
  };
  let kb = 0;
  for (const p of kids(pid)) {
    const m = /VmRSS:\s+(\d+)/.exec(readSafe(`/proc/${p}/status`));
    if (m) kb += Number(m[1]);
  }
  return Math.round(kb / 1024);
}
function readSafe(p) {
  try {
    return readFileSync(p, "utf8");
  } catch {
    return "";
  }
}
function readdirSafe(p) {
  try {
    return readdirSync(p);
  } catch {
    return [];
  }
}

try {
  let t = await start();

  const a = await put(t, "2026-11-27-a", "# A\n\n本文");
  check(a.status === 200 && a.json?.version === 1 && a.json?.bumped === true, `上げると version 1（${a.status} ${a.text}）`);
  check(a.json?.url?.startsWith(`${t.list_origin}/r/`), "答えの URL は一覧の面の報告の画面");
  const id = a.json?.url?.split("/").pop();

  let l = await list(t);
  check(l.status === 200 && l.json?.reports?.length === 1 && l.json.reports[0].unread === true, "一覧に未読で出る");
  check(l.headers["content-security-policy"] === "frame-ancestors 'none'", "一覧の面の応答は枠に入らない");

  const o = await open(t, id);
  check(o.status === 200 && o.json?.url?.startsWith(`${t.view_origin}/r/${id}/`), "開くと報告の面の署名付き URL が返る");
  const v = new URL(o.json.url);
  const page = await http(t.list, `${v.pathname}${v.search}`, { host: v.host });
  check(page.status === 200 && page.text.includes("<h1>A</h1>"), "署名付き URL で報告が読める");
  check(/sandbox allow-scripts/.test(page.headers["content-security-policy"] ?? ""), "報告の面は sandbox で出す");
  const unsigned = await http(t.list, v.pathname, { host: v.host });
  check(unsigned.status === 403, "署名の無い URL は断る");

  l = await list(t);
  check(l.json?.reports?.length === 0, "開いた報告は既読になる");
  const same = await put(t, "2026-11-27-a", "# A\n\n本文");
  check(same.json?.bumped === false && (await list(t)).json?.reports?.length === 0, "同じ中身の出し直しは既読のまま");
  const changed = await put(t, "2026-11-27-a", "# A\n\n直した本文");
  check(changed.json?.version === 2 && (await list(t)).json?.reports?.[0]?.id === id, "違う中身の出し直しは未読に戻る");

  await put(t, "2026-11-27-a-0101", "x", { type: "text/plain" });
  await put(t, "2026-11-27-a-0102", "x", { type: "text/plain" });
  await put(t, "週のダイジェスト", "<!doctype html><p>d</p>", { kind: "digest", type: "text/html" });
  await put(t, "A-0001", "# 課題の報告", { kind: "report", project: "-" });
  l = await list(t, "recent");
  check(l.json?.reports?.length === 5, `ダイジェストと担当の報告が同じ一覧に並ぶ（${l.json?.reports?.length} 件）`);

  const bad = await http(t.list, "/api/reports", { host: "evil.example.com" });
  check(bad.status === 421, "知らない Host は断る");
  const viaUpload = await http(t.upload, "/api/reports", { host: hostOf(t.list_origin) });
  check(viaUpload.status === 404, "アップロードの口は一覧の API を持たない");

  // 止まっている間のメモリ（ADR-0064 決めたこと 26 の 600 MB）
  await new Promise((ok) => setTimeout(ok, 5000));
  const mb = rss(t.proc.pid);
  check(mb > 0 && mb < 600, `止まっている間のメモリは 600 MB を超えない（RSS 合計 ${mb} MB）`);

  // 起動し直しても報告は残り、移行は当て直さない
  await stop(t);
  t = await start();
  l = await list(t, "recent");
  check(l.json?.reports?.length === 5, "起動し直しても報告が残る");
  await stop(t);
} catch (e) {
  console.error(e);
  failed++;
} finally {
  rmSync(dir, { recursive: true, force: true });
}

console.log(failed === 0 ? "端から端の検査: 全部通った" : `端から端の検査: ${failed} 件が落ちた`);
process.exit(failed === 0 ? 0 : 1);
