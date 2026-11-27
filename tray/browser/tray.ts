// ブラウザの検査とその手前の道具。scripts/serve.mjs でローカルの tray を一時の置き場に起動する。
import { type ChildProcess, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export interface Tray {
  proc: ChildProcess;
  dir: string;
  list: string;
  upload: string;
  list_origin: string;
  view_origin: string;
}

export async function startTray(cell = "demo"): Promise<Tray> {
  const dir = mkdtempSync(join(tmpdir(), "tray-"));
  const proc = spawn("node", ["scripts/serve.mjs", "--cell", cell, "--data", join(dir, "data")], {
    stdio: ["ignore", "pipe", "inherit"],
  });
  const line = await new Promise<string>((ok, ng) => {
    let buf = "";
    proc.stdout?.on("data", (d) => {
      buf += String(d);
      const i = buf.indexOf("\n");
      if (i >= 0) ok(buf.slice(0, i));
    });
    proc.once("exit", (code) => ng(new Error(`serve.mjs が止まった: ${code}`)));
  });
  return { proc, dir, ...JSON.parse(line) };
}

export async function stopTray(t: Tray) {
  await new Promise((ok) => {
    t.proc.once("exit", ok);
    t.proc.kill("SIGTERM");
  });
  rmSync(t.dir, { recursive: true, force: true });
}

// put: アップロードの口へ報告を上げ、答えを返す
export async function put(t: Tray, name: string, body: string | Uint8Array, contentType = "text/html; charset=utf-8", project = "andon") {
  const res = await fetch(`${t.upload}/v1/reports/${encodeURIComponent(project)}/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: {
      "Content-Type": contentType,
      "X-Tray-Kind": "publish",
      "X-Tray-Title": encodeURIComponent(name),
      "X-Tray-Sha256": createHash("sha256").update(body).digest("hex"),
    },
    body,
  });
  if (!res.ok) throw new Error(`アップロードが ${res.status}: ${await res.text()}`);
  return (await res.json()) as { url: string; version: number; bumped: boolean };
}
