#!/usr/bin/env node
// Cloudflare の tray をデプロイする（ADR-0064 決めたこと 16・17・23・27）。Node で動かす。
// オーナーがセルの外（セルを動かしている PC）で、andon のイメージから打つ（`bin/bootstrap tray-deploy` → `andon tray deploy`）。
// セルの中には Cloudflare の資格情報を置かない。トークンはこの1回の実行の環境変数 CLOUDFLARE_API_TOKEN にだけ渡す。
//
//   node scripts/deploy.mjs --config <設定のファイル> [--dry-run]
//
// 設定のファイル（セルの repo の tray-deploy.json。公開の repo には書かない値）:
//   {"account_id": "<32 桁の16進>",
//    "hosts": {"list": "<一覧の面のホスト名>", "view": "<報告の面のホスト名>", "upload": "<アップロードの口のホスト名>"},
//    "access": {"team_domain": "https://<チーム>.cloudflareaccess.com", "aud": "<Access のアプリの AUD タグ（64 桁の16進）>"}}
//
// すること（どれも何度打ってもよい）:
//   1. D1 のデータベースと R2 のバケット（どちらも andon-tray）が無ければ作る
//   2. D1 の移行（migrations/）を当てる
//   3. 3つの面を、面ごとに別の Worker（andon-tray-list・-view・-upload）にして、要る束縛だけを持たせて出す。
//      どれも workers.dev と preview の URL を切り、ホスト名（custom domain）だけで開く
//   4. 署名の鍵を打つたびに作り直し、一覧の面と報告の面に secret で渡す（署名付き URL の期限は5分なので、出し直しで前の URL が開けなくなるだけ）
// --dry-run は Cloudflare に触れず、組んだ設定を wrangler の deploy --dry-run に通すだけ（トークンは要らない。tray の CI の job が回す）。

import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

// 名前は固定する（1人のオーナーのアカウントに tray は1つ）
export const NAMES = {
  d1: "andon-tray",
  r2: "andon-tray",
  list: "andon-tray-list",
  view: "andon-tray-view",
  upload: "andon-tray-upload",
};
// scripts/serve.mjs と同じ
const COMPAT = "2026-11-01";
// --dry-run で D1 の id の代わりに書く値
const DRY_D1_ID = "00000000-0000-0000-0000-000000000000";

const hostRe = /^(?=.{1,253}$)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]$/;

// readConfig: 設定のファイルの中身を読んで確かめる。知らないキーと欠けたキーは誤り
export function readConfig(text) {
  let c;
  try {
    c = JSON.parse(text);
  } catch (e) {
    throw new Error(`JSON として読めない: ${e.message}`);
  }
  const obj = (v, at, keys) => {
    if (!v || typeof v !== "object" || Array.isArray(v)) throw new Error(`${at} が object でない`);
    for (const k of Object.keys(v)) if (!keys.includes(k)) throw new Error(`${at} に知らないキー ${k} がある`);
    for (const k of keys) if (typeof v[k] !== "string") throw new Error(`${at}.${k} が無いか文字列でない`);
  };
  if (!c || typeof c !== "object" || Array.isArray(c)) throw new Error("設定が object でない");
  for (const k of Object.keys(c)) if (!["account_id", "hosts", "access"].includes(k)) throw new Error(`知らないキー ${k} がある`);
  if (typeof c.account_id !== "string" || !/^[0-9a-f]{32}$/.test(c.account_id)) throw new Error("account_id が 32 桁の16進（小文字）でない");
  obj(c.hosts, "hosts", ["list", "view", "upload"]);
  for (const k of ["list", "view", "upload"]) {
    if (!hostRe.test(c.hosts[k])) throw new Error(`hosts.${k} がホスト名（英小文字・数字・- を . で区切ったもの）でない: ${c.hosts[k]}`);
  }
  if (new Set(Object.values(c.hosts)).size !== 3) throw new Error("hosts の3つのホスト名が重なっている（面ごとに別の名前にする）");
  obj(c.access, "access", ["team_domain", "aud"]);
  const team = /^https:\/\/([^/]+)\/?$/.exec(c.access.team_domain);
  if (!team || !hostRe.test(team[1] ?? "")) throw new Error(`access.team_domain が https://<チーム>.cloudflareaccess.com の形でない: ${c.access.team_domain}`);
  if (!/^[0-9a-f]{64}$/.test(c.access.aud)) throw new Error("access.aud が 64 桁の16進（Access のアプリの AUD タグ）でない");
  return { ...c, access: { ...c.access, team_domain: `https://${team[1]}` } };
}

// workerConfigs: 3つの Worker の wrangler の設定（JSON）。束縛は面ごとに要るものだけ（決めたこと 16）
export function workerConfigs(c, d1Id, dir = root) {
  const origin = (h) => `https://${h}`;
  const base = (name, file, host) => ({
    name,
    main: join(dir, "dist/workers", file),
    compatibility_date: COMPAT,
    account_id: c.account_id,
    no_bundle: true,
    // ホスト名だけで開く（workers.dev の名前から一覧の面に Access を通らずに届く道を作らない）
    workers_dev: false,
    preview_urls: false,
    routes: [{ pattern: host, custom_domain: true }],
  });
  const d1 = { binding: "DB", database_name: NAMES.d1, database_id: d1Id, migrations_dir: join(dir, "migrations") };
  const r2 = { binding: "BUCKET", bucket_name: NAMES.r2 };
  return {
    // 一覧の面: D1・署名の鍵（secret）・画面のファイル。/api/ の下だけが Worker を通り、Access の JWT を確かめる（決めたこと 17）
    list: {
      ...base(NAMES.list, "list.js", c.hosts.list),
      d1_databases: [d1],
      vars: { VIEW_ORIGIN: origin(c.hosts.view), ACCESS_TEAM_DOMAIN: c.access.team_domain, ACCESS_AUD: c.access.aud },
      assets: { directory: join(dir, "dist/public"), not_found_handling: "single-page-application", run_worker_first: ["/api/*"] },
    },
    // 報告の面: R2・署名の鍵（secret）。D1 には届かない（決めたこと 18）
    view: {
      ...base(NAMES.view, "view.js", c.hosts.view),
      r2_buckets: [r2],
      vars: { LIST_ORIGIN: origin(c.hosts.list) },
    },
    // アップロードの口: R2・D1。署名の鍵には届かない（決めたこと 15・19）
    upload: {
      ...base(NAMES.upload, "upload.js", c.hosts.upload),
      d1_databases: [d1],
      r2_buckets: [r2],
      vars: { LIST_ORIGIN: origin(c.hosts.list) },
    },
  };
}

function main() {
  const { values: o } = parseArgs({ options: { config: { type: "string" }, "dry-run": { type: "boolean", default: false } } });
  if (!o.config) {
    console.error("使い方: deploy.mjs --config <設定のファイル> [--dry-run]");
    process.exit(2);
  }
  const dry = o["dry-run"];
  let c;
  try {
    c = readConfig(readFileSync(o.config, "utf8"));
  } catch (e) {
    console.error(`deploy: ${o.config}: ${e.message}`);
    process.exit(2);
  }
  if (!dry && !process.env.CLOUDFLARE_API_TOKEN) {
    console.error("deploy: CLOUDFLARE_API_TOKEN が無い（この1回の実行にだけ渡す。docs/operate.md の Cloudflare の tray の手順）");
    process.exit(2);
  }

  // wrangler の置き場（ログ・控え）は一時の置き場に閉じる。HOME の下に書かない
  const tmp = mkdtempSync(join(tmpdir(), "andon-tray-deploy-"));
  const env = {
    ...process.env,
    CLOUDFLARE_ACCOUNT_ID: c.account_id,
    WRANGLER_SEND_METRICS: "false",
    WRANGLER_LOG_PATH: join(tmp, "logs"),
    XDG_CONFIG_HOME: join(tmp, "config"),
    CI: "true",
  };
  const wranglerJs = join(root, "node_modules/wrangler/bin/wrangler.js");
  // wrangler: 打って、落ちたら理由を出して止まる。capture なら stdout を返す。probe なら落ちても止まらず null を返す
  const wrangler = (args, { capture = false, probe = false } = {}) => {
    console.error(`deploy: wrangler ${args.join(" ")}`);
    const r = spawnSync(process.execPath, [wranglerJs, ...args], {
      cwd: tmp,
      env,
      encoding: "utf8",
      stdio: ["ignore", capture || probe ? "pipe" : "inherit", probe ? "pipe" : "inherit"],
    });
    if (probe) return r.status === 0 ? r.stdout : null;
    if (r.status !== 0) {
      rmSync(tmp, { recursive: true, force: true });
      console.error(`deploy: wrangler ${args[0]} ${args[1] ?? ""} が落ちた（終了コード ${r.status}）。上の出力を見る。何度打ち直してもよい`);
      process.exit(1);
    }
    return r.stdout ?? "";
  };

  try {
    let d1Id = DRY_D1_ID;
    if (!dry) {
      d1Id = ensureD1(wrangler);
      ensureR2(wrangler);
    }
    const cfgs = workerConfigs(c, d1Id);
    const file = (name) => join(tmp, `${name}.json`);
    for (const [k, v] of Object.entries(cfgs)) writeFileSync(file(k), JSON.stringify(v, null, 2));
    const secrets = join(tmp, "secrets.json");
    writeFileSync(secrets, JSON.stringify({ SIGNING_KEY: randomBytes(32).toString("hex") }), { mode: 0o600 });

    if (dry) {
      for (const k of ["upload", "view", "list"]) wrangler(["deploy", "--dry-run", "-c", file(k), ...(k === "upload" ? [] : ["--secrets-file", secrets])]);
      console.log("dry-run: 3つの Worker の設定が wrangler の確かめを通った（Cloudflare には触れていない）");
      return;
    }
    wrangler(["d1", "migrations", "apply", NAMES.d1, "--remote", "-c", file("list")]);
    wrangler(["deploy", "-c", file("upload")]);
    wrangler(["deploy", "-c", file("view"), "--secrets-file", secrets]);
    wrangler(["deploy", "-c", file("list"), "--secrets-file", secrets]);
    console.log(`出した: 一覧の面 https://${c.hosts.list}/ ・報告の面 https://${c.hosts.view}/ ・アップロードの口 https://${c.hosts.upload}/`);
    console.log(`次: 一覧の面のセルの画面で鍵を作り、セルで andon tray auth に渡す。project.json に "tray": {"upload": "https://${c.hosts.upload}"} を書く`);
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

// ensureD1: D1 のデータベースが無ければ作り、id を返す
function ensureD1(wrangler) {
  const find = () => {
    const list = JSON.parse(wrangler(["d1", "list", "--json"], { capture: true }));
    return (Array.isArray(list) ? list : []).find((d) => d.name === NAMES.d1)?.uuid;
  };
  let id = find();
  if (!id) {
    wrangler(["d1", "create", NAMES.d1]);
    id = find();
  }
  if (!id) {
    console.error(`deploy: D1 のデータベース ${NAMES.d1} を作ったのに一覧に無い`);
    process.exit(1);
  }
  return id;
}

// ensureR2: R2 のバケットが無ければ作る（info が落ちたら無いと読む。読めないだけなら、作るのが落ちてそこで止まる）
function ensureR2(wrangler) {
  if (wrangler(["r2", "bucket", "info", NAMES.r2], { probe: true }) === null) wrangler(["r2", "bucket", "create", NAMES.r2]);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();
