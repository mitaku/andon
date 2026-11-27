#!/usr/bin/env node
// ローカルの tray を miniflare（workerd）で起動する（ADR-0064 決めたこと 6・15・26）。Node で動かす。
// `andon tray serve`（セルの側）がこれを起動する。端から端の検査とブラウザの検査も、これで一時の置き場に起動する。
//
//   node scripts/serve.mjs --cell <セル> --data <置き場> [--port <番号>] [--upload-port <番号>]
//                          [--list-host <名前>] [--view-host <名前>] [--public-port <番号>]
//
// 一覧の面と報告の面は --port で 127.0.0.1 に開き、Host（--list-host・--view-host）で振り分ける。アップロードの口は
// --upload-port で 127.0.0.1 にだけ開く。番号を 0 にすると空いた番号を使う。--public-port はオーナーが開く番号
// （portless の番号）で、アップロードの答えの URL に使う。省くと --port と同じ。
// 起動のたびに migrations/ の D1 の移行を当てる。D1 と R2 は --data の下に置く。署名の鍵は起動のたびに作る
// （署名付き URL の期限は5分なので、起動し直しで前の URL が開けなくなるだけ）。
// 動き始めたら、stdout に1行の JSON（list・upload・list_origin・view_origin）を出す。SIGTERM・SIGINT で止まる。

import { randomBytes } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { parseArgs } from "node:util";
import { fileURLToPath } from "node:url";
import { Miniflare } from "miniflare";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const { values: o } = parseArgs({
  options: {
    cell: { type: "string" },
    data: { type: "string" },
    port: { type: "string", default: "0" },
    "upload-port": { type: "string", default: "0" },
    "list-host": { type: "string" },
    "view-host": { type: "string" },
    "public-port": { type: "string" },
  },
});
if (!o.cell || !o.data) {
  console.error("使い方: serve.mjs --cell <セル> --data <置き場> [--port N] [--upload-port N] [--list-host H] [--view-host H] [--public-port N]");
  process.exit(2);
}
const listHost = o["list-host"] ?? `tray.${o.cell}.localhost`;
const viewHost = o["view-host"] ?? `tray-view.${o.cell}.localhost`;
const COMPAT = "2026-11-01";

function worker(name, file, env, extra = {}) {
  return {
    config: {
      name,
      compatibilityDate: COMPAT,
      manifest: {
        mainModule: `${name}.js`,
        modules: { [`${name}.js`]: { type: "esm", contents: readFileSync(join(root, "dist/workers", file), "utf8") } },
      },
      env,
      ...extra.config,
    },
    ...(extra.dev ? { dev: extra.dev } : {}),
  };
}

const store = { DB: { type: "d1", id: "tray" }, BUCKET: { type: "r2", name: "tray" } };
const signingKey = randomBytes(32).toString("hex");

// freePort: アップロードの答えの URL に一覧の面のオリジンが要るので、起動の前に番号を決める。
// 0 なら 127.0.0.1 で空いた番号を1つ取る。
async function freePort(want) {
  if (want !== "0") return Number(want);
  const { createServer } = await import("node:net");
  return new Promise((ok, ng) => {
    const s = createServer();
    s.once("error", ng);
    s.listen(0, "127.0.0.1", () => {
      const p = s.address().port;
      s.close(() => ok(p));
    });
  });
}
const port = await freePort(o.port);
const uploadPort = await freePort(o["upload-port"]);
const publicPort = o["public-port"] ?? String(port);
const listOrigin = `http://${listHost}:${publicPort}`;
const viewOrigin = `http://${viewHost}:${publicPort}`;

const mf = new Miniflare({
  host: "127.0.0.1",
  port,
  resourcePersistencePath: resolve(o.data),
  telemetry: { enabled: false },
  enableDevRegistry: false,
  workers: [
    worker(
      "local",
      "local.js",
      {
        ...store,
        SIGNING_KEY: { type: "text", value: signingKey },
        LIST_HOST: { type: "text", value: listHost },
        VIEW_HOST: { type: "text", value: viewHost },
        ASSETS: { type: "assets" },
      },
      // runWorkerFirst: 報告の面のホスト名の移動（Sec-Fetch-Mode: navigate）も、画面のファイルより先に Worker が受ける
      {
        config: {
          assets: {
            directory: join(root, "dist/public"),
            hasUserWorker: true,
            notFoundHandling: "single-page-application",
            runWorkerFirst: true,
          },
        },
      },
    ),
    worker(
      "upload",
      "local-upload.js",
      { ...store, LIST_ORIGIN: { type: "text", value: listOrigin }, CELL: { type: "text", value: o.cell } },
      { dev: { unsafeDirectSockets: [{ host: "127.0.0.1", port: uploadPort }] } },
    ),
  ],
});

await mf.ready;
const db = await mf.getD1Database("DB", "local");
await migrate(db);
process.stdout.write(
  `${JSON.stringify({
    list: `http://127.0.0.1:${port}`,
    upload: `http://127.0.0.1:${uploadPort}`,
    list_origin: listOrigin,
    view_origin: viewOrigin,
  })}\n`,
);

async function stop() {
  await mf.dispose();
  process.exit(0);
}
process.on("SIGTERM", stop);
process.on("SIGINT", stop);

// migrate: migrations/ の *.sql を名前の順に当てる。当てた名前は d1_migrations に残す（wrangler と同じ表）
async function migrate(db) {
  await db
    .prepare(
      "CREATE TABLE IF NOT EXISTS d1_migrations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL)",
    )
    .run();
  const { results } = await db.prepare("SELECT name FROM d1_migrations").all();
  const done = new Set(results.map((r) => r.name));
  const dir = join(root, "migrations");
  for (const name of readdirSync(dir).filter((f) => f.endsWith(".sql")).sort()) {
    if (done.has(name)) continue;
    const stmts = statements(readFileSync(join(dir, name), "utf8"));
    await db.batch([...stmts.map((s) => db.prepare(s)), db.prepare("INSERT INTO d1_migrations (name) VALUES (?1)").bind(name)]);
  }
}

// statements: 行頭の -- の行を除き、; で文に割る（移行の SQL は文字列の中に ; を書かない）
export function statements(sql) {
  return sql
    .split("\n")
    .filter((l) => !/^\s*--/.test(l))
    .join("\n")
    .split(";")
    .map((s) => s.trim())
    .filter(Boolean);
}
