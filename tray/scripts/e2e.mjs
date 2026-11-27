#!/usr/bin/env node
// 端から端の検査（ADR-0064 決めたこと 28）。ビルドしたローカルの tray を一時の置き場で起動し、アップロードの口から
// 一覧・既読・報告の面までを HTTP で通す。job tray の中でだけ回す（tests/ のシェルにしない）。
// 先に `bun run build` を打っておく。終了コード 0 で全部通った。
//
// 後半はセルの側（ADR-0064 受け入れ条件 1・4・15）: 偽のセルで andon boot を打ってローカルの tray を起動させ、andon publish と
// andon digest send の出力の URL・一覧・既読・files/ を見る。andon のバイナリは ANDON_BIN（job tray が repo から焼く）。
// ANDON_BIN が無ければ後半を飛ばす（CI では落とす）。

import { execFileSync, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
  chmodSync,
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
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

// cell: 偽のセルでセルの側の command を通す（受け入れ条件 1・4・15）
async function cell() {
  const andon = process.env.ANDON_BIN;
  if (!andon) {
    check(!process.env.CI, "セルの側は ANDON_BIN が無いので飛ばした（CI では落とす）");
    return;
  }
  const c = join(dir, "cell");
  const pkg = join(c, "opt/andon");
  const ws = join(c, "workspace");
  const bin = join(c, "bin");
  const fake = join(c, "fake");
  const repo = resolve(root, "..");
  for (const d of [bin, fake, join(c, "home"), join(c, "state"), join(ws, "projects")]) mkdirSync(d, { recursive: true });
  // パッケージは repo の package/ と schema/、tray はビルドした repo の tray/
  cpSync(join(repo, "package"), pkg, { recursive: true });
  cpSync(join(repo, "schema"), join(pkg, "schema"), { recursive: true });
  symlinkSync(root, join(pkg, "tray"));
  symlinkSync(resolve(andon), join(bin, "andon"));
  symlinkSync(process.execPath, join(bin, "node"));
  // 番号は空いたもの（セルで動いている本物の tray と取り合わない）
  writeFileSync(join(ws, ".env"), "TRAY_PORT=0\nPORTLESS_PORT=1355\n");
  const env = {
    PATH: bin,
    HOME: join(c, "home"),
    ANDON_HOME: pkg,
    ANDON_WORKSPACE: ws,
    ANDON_MANAGED_SETTINGS: join(c, "managed-settings.json"),
    XDG_STATE_HOME: join(c, "state"),
    TMPDIR: c,
    ANDON_TRAY_OFF: "0",
  };
  const run = (args, extra = {}, input = "") => {
    try {
      const out = execFileSync(join(bin, "andon"), args, { env: { ...env, ...extra }, cwd: c, input, encoding: "utf8", stdio: "pipe" });
      return { rc: 0, out, err: "" };
    } catch (e) {
      return { rc: e.status ?? -1, out: String(e.stdout ?? ""), err: String(e.stderr ?? "") };
    }
  };
  const control = join(c, "state/andon/tray/serve.json");
  let pid = 0;
  try {
    // 受け入れ条件 15: boot のあとローカルの tray が動き、files サーバは無い
    const boot = run(["boot"]);
    const said = boot.out + boot.err;
    check(boot.rc === 0 && /ローカルの tray: http:\/\/tray\.[^ ]+\.localhost:1355\//.test(said), `boot がローカルの tray を起動した（rc ${boot.rc}）`);
    check(!/files: http/.test(said) && !said.includes("files サーバ"), "boot は files サーバを起動しない");
    check(run(["boot", "serve-files", ws, "0"]).rc === 2, "葉 andon boot serve-files は無い");
    check(existsSync(control), "ローカルの tray の控え（serve.json）がある");
    const t = JSON.parse(readFileSync(control, "utf8"));
    pid = t.pid;
    await new Promise((ok) => setTimeout(ok, 5000));
    const mb = rss(pid);
    check(mb > 0 && mb < 600, `boot が起動した tray の止まっている間のメモリは 600 MB を超えない（andon tray serve と子の RSS 合計 ${mb} MB）`);

    // 受け入れ条件 1: andon publish の1行がローカルの tray の URL で、一覧・既読・出し直し・files/ に無い
    const md = join(c, "notes.md");
    writeFileSync(md, "# 週の報告\n\n本文\n");
    const p1 = run(["publish", md, "weekly"]);
    const url = p1.out.trim();
    check(p1.rc === 0 && url.startsWith(`${t.list_origin}/r/`) && p1.out.split("\n").length === 2, `publish の1行はローカルの tray の URL（${p1.out.trim()} ${p1.err}）`);
    const id = url.split("/").pop();
    let l = await list(t);
    const row = l.json?.reports?.find((r) => r.id === id);
    check(row?.unread === true && row?.title === "週の報告", "出した報告が一覧に未読で出て、タイトルは最初の見出し");
    await open(t, id);
    check(!(await list(t)).json?.reports?.some((r) => r.id === id), "開くと既読になる");
    check(run(["publish", md, "weekly"]).out.trim() === url && !(await list(t)).json?.reports?.some((r) => r.id === id), "同じ中身の出し直しは既読のまま");
    writeFileSync(md, "# 週の報告\n\n直した本文\n");
    check(run(["publish", md, "weekly"]).out.trim() === url && (await list(t)).json?.reports?.some((r) => r.id === id), "同じ slug で中身を変えると未読に戻る");
    const a1 = run(["publish", md]).out.trim();
    const a2 = run(["publish", md]).out.trim();
    check(a1.startsWith(`${t.list_origin}/r/`) && a2.startsWith(`${t.list_origin}/r/`) && a1 !== a2, "slug を省いて2度出すと2件の報告になる");
    check(!existsSync(join(ws, "files")), "tray に届いた報告は files/ に無い");

    // 受け入れ条件 4: andon digest send のページが同じ一覧に並び、sent.json の files の行の url がローカルの tray の URL
    const sent = digest(ws, fake, bin, run);
    const files = sent?.destinations?.find((d) => d.kind === "files");
    check(files?.ok === true && files?.url?.startsWith(`${t.list_origin}/r/`), `ダイジェストの files の行の url はローカルの tray の URL（${JSON.stringify(sent?.destinations)}）`);
    l = await list(t, "recent");
    check(l.json?.reports?.some((r) => r.kind === "digest" && `${t.list_origin}/r/${r.id}` === files?.url), "ダイジェストが同じ一覧に並ぶ");
    check(!existsSync(join(ws, "files")), "ダイジェストも files/ に写さない");

    const st = run(["tray", "status"]);
    check(st.rc === 0 && /直近 7 日: 上がった [1-9][0-9]* 枚、開いた 1 枚/.test(st.out), `andon tray status が枚数を出す（${st.out.trim().split("\n").join(" / ")}）`);
  } finally {
    if (pid) {
      try {
        process.kill(pid, "SIGTERM");
      } catch {}
      for (let i = 0; i < 100 && alive(pid); i++) await new Promise((ok) => setTimeout(ok, 100));
      check(!alive(pid) && !existsSync(control), "andon tray serve は TERM で止まり、控えを消す");
    }
  }
}

function alive(pid) {
  try {
    process.kill(pid, 0);
    return !/^State:\s+Z/m.test(readSafe(`/proc/${pid}/status`));
  } catch {
    return false;
  }
}

// digest: 一時の kanban（ファイルを直に置く。tests/andon-digest.sh と同じ形）で collect・build・send を通し、sent.json を返す。
// git・kanban・gh は偽（送り先の設定は無い・PR は無い）
function digest(ws, fake, bin, run) {
  const k = join(ws, "projects/sample-site/kanban");
  const put = (p, body) => {
    mkdirSync(dirname(p), { recursive: true });
    writeFileSync(p, body);
  };
  put(join(k, "config.yaml"), "version: 3\nprefix: S\nchief_of_staff: t1\n");
  put(
    join(k, "issues/S-0100/issue.yaml"),
    'id: S-0100\ntitle: デイリーダイジェスト\nstatus: doing\nroutine: daily-digest\nprofile: claude\ncreated: "2026-11-01T00:00:00Z"\n',
  );
  put(join(k, "log.jsonl"), "");
  const script = (name, body) => {
    writeFileSync(join(fake, name), `#!/bin/sh\n${body}\n`);
    chmodSync(join(fake, name), 0o755);
  };
  script("git", '[ "$3" = ls-tree ] && exit 0\nexit 128');
  script("kanban", "exit 99");
  script("gh", 'echo "[]"');
  const extra = { PATH: `${fake}:${bin}:/bin:/usr/bin`, KANBAN_ROOT: join(ws, "projects/sample-site") };
  const text = '{"headline": ["動きは無かった。", "待っているものも無い。"], "themes": [], "concerns": []}';
  for (const args of [
    ["digest", "collect", "S-0100"],
    ["digest", "build", "S-0100", "-"],
    ["digest", "send", "S-0100"],
  ]) {
    const r = run(args, extra, args[1] === "build" ? text : "");
    check(r.rc === 0, `andon ${args.join(" ")}（rc ${r.rc}${r.rc === 0 ? "" : ` ${r.out}${r.err}`}）`);
    if (r.rc !== 0) return null;
  }
  return JSON.parse(readFileSync(join(k, "issues/S-0100/library/digest/sent.json"), "utf8"));
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

  await cell();
} catch (e) {
  console.error(e);
  failed++;
} finally {
  rmSync(dir, { recursive: true, force: true });
}

console.log(failed === 0 ? "端から端の検査: 全部通った" : `端から端の検査: ${failed} 件が落ちた`);
process.exit(failed === 0 ? 0 : 1);
