// ローカルの tray の入口（Host で振り分ける・知らない Host を断る）
import { env } from "cloudflare:test";
import { describe, expect, it } from "vitest";
import local from "../src/local";
import localUpload from "../src/local-upload";
import { putRequest } from "./helpers";

const LIST_HOST = "tray.demo.localhost";
const VIEW_HOST = "tray-view.demo.localhost";

function localEnv() {
  return {
    DB: env.DB,
    BUCKET: env.BUCKET,
    SIGNING_KEY: "k",
    LIST_HOST,
    VIEW_HOST,
    ASSETS: { fetch: async () => new Response("<!doctype html>画面", { headers: { "Content-Type": "text/html" } }) } as unknown as Fetcher,
  };
}

describe("ローカルの tray", () => {
  it("名指した2つのホスト名のほかの Host を断る", async () => {
    for (const host of ["evil.example.com", "127.0.0.1", "tray.other.localhost"]) {
      const res = await local.fetch(new Request(`http://${host}:1355/api/reports`), localEnv());
      expect(res.status, host).toBe(421);
    }
  });

  it("アップロードした報告が一覧に出て、開くと報告の面の URL が返り、その URL で読める", async () => {
    const up = await localUpload.fetch(await putRequest({ name: "n", body: "# 見出し" }), {
      DB: env.DB,
      BUCKET: env.BUCKET,
      LIST_ORIGIN: `http://${LIST_HOST}:1355`,
      CELL: "demo",
    });
    const { url } = (await up.json()) as { url: string };
    expect(url).toMatch(/^http:\/\/tray\.demo\.localhost:1355\/r\/[0-9a-f]{32}$/);
    const listed = await local.fetch(new Request(`http://${LIST_HOST}:1355/api/reports`), localEnv());
    expect(((await listed.json()) as { reports: { cell: string }[] }).reports.map((r) => r.cell)).toEqual(["demo"]);
    const id = url.split("/").pop();
    const opened = await local.fetch(
      new Request(`http://${LIST_HOST}:1355/api/reports/${id}/open`, {
        method: "POST",
        headers: { Origin: `http://${LIST_HOST}:1355` },
      }),
      localEnv(),
    );
    const { url: viewURL } = (await opened.json()) as { url: string };
    expect(viewURL.startsWith(`http://${VIEW_HOST}:1355/r/${id}?`)).toBe(true);
    const res = await local.fetch(new Request(viewURL), localEnv());
    expect(res.status).toBe(200);
    expect(await res.text()).toContain("<h1>見出し</h1>");
    expect(res.headers.get("Content-Security-Policy")).toContain(`frame-ancestors http://${LIST_HOST}:1355`);
  });

  it("画面のファイルにも一覧の面のヘッダを付ける", async () => {
    const res = await local.fetch(new Request(`http://${LIST_HOST}:1355/r/${"a".repeat(32)}`), localEnv());
    expect(res.headers.get("Content-Security-Policy")).toBe("frame-ancestors 'none'");
  });
});
