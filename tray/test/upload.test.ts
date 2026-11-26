// アップロードの口（受け入れ条件 11・14）
import { env } from "cloudflare:test";
import { describe, expect, it } from "vitest";
import contract from "../contract/upload-v1.json";
import upload from "../src/upload";
import { putKey, putRequest, UPLOAD_ORIGIN, uploadEnv } from "./helpers";

async function send(req: Request) {
  const res = await upload.fetch(req, uploadEnv());
  return { status: res.status, body: (await res.json()) as Record<string, unknown> };
}

async function rows(project = "andon", name?: string) {
  const q = name
    ? env.DB.prepare("SELECT * FROM reports WHERE project = ?1 AND name = ?2 ORDER BY cell").bind(project, name)
    : env.DB.prepare("SELECT * FROM reports WHERE project = ?1 ORDER BY cell").bind(project);
  return (await q.all<{ id: string; cell: string; version: number; sha256: string }>()).results;
}

describe("約束の見本", () => {
  it("見本の要求が通り、答えの欄が見本と同じ形", async () => {
    await putKey("c1", "tray_contractkey000000");
    const r = contract.request;
    const headers = { ...r.headers, Authorization: "Bearer tray_contractkey000000" };
    const res = await upload.fetch(
      new Request(`${UPLOAD_ORIGIN}${r.path}`, { method: r.method, headers, body: r.body }),
      uploadEnv(),
    );
    expect(res.status).toBe(contract.response.status);
    const body = (await res.json()) as Record<string, unknown>;
    const want = contract.response.body as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(Object.keys(want).sort());
    for (const [k, v] of Object.entries(want)) expect(typeof body[k], k).toBe(typeof v);
    expect(body.version).toBe(1);
    expect(body.bumped).toBe(true);
    expect(String(body.url)).toMatch(/^https:\/\/tray\.example\.com\/r\/[0-9a-f]{32}$/);
    const [row] = await rows("andon", "2026-11-27-週の報告");
    expect(row?.cell).toBe("c1");
  });

  // 見本の要求の欄はどれも要る。1つ消すと断る（見本の欄を消すと、この検査が落ちる）
  for (const field of Object.keys(contract.request.headers).filter((h) => h !== "Authorization")) {
    it(`要求の欄 ${field} が無ければ断る`, async () => {
      await putKey("c1", "tray_contractkey000000");
      const r = contract.request;
      const headers: Record<string, string> = { ...r.headers, Authorization: "Bearer tray_contractkey000000" };
      delete headers[field];
      const req = new Request(`${UPLOAD_ORIGIN}${r.path}`, { method: r.method, headers, body: r.body });
      // Request は本文から Content-Type を補うので、消したことを確かめる
      if (field === "Content-Type") req.headers.delete("Content-Type");
      expect((await send(req)).status).toBe(400);
    });
  }
});

describe("鍵（受け入れ条件 11）", () => {
  it("鍵の無い要求を断る", async () => {
    const res = await send(await putRequest({ name: "a", body: "x" }));
    expect(res.status).toBe(401);
    expect(await rows()).toHaveLength(0);
  });

  it("知らない鍵を断る", async () => {
    await putKey("c1", "tray_known000000000000");
    const res = await send(await putRequest({ name: "a", body: "x", key: "tray_unknown0000000000" }));
    expect(res.status).toBe(401);
  });

  it("失効させた鍵を断る", async () => {
    await putKey("c1", "tray_revoked0000000000", true);
    const res = await send(await putRequest({ name: "a", body: "x", key: "tray_revoked0000000000" }));
    expect(res.status).toBe(401);
    expect(await rows()).toHaveLength(0);
  });

  it("あるセルの鍵で、別のセルの報告を上書きできない", async () => {
    await putKey("c1", "tray_cellone0000000000");
    await putKey("c2", "tray_celltwo0000000000");
    const a = await send(await putRequest({ name: "same", body: "c1 の中身", key: "tray_cellone0000000000" }));
    expect(a.status).toBe(200);
    const b = await send(await putRequest({ name: "same", body: "c2 の中身", key: "tray_celltwo0000000000" }));
    expect(b.status).toBe(200);
    expect(b.body.version).toBe(1);
    const got = await rows("andon", "same");
    expect(got.map((r) => [r.cell, r.version])).toEqual([
      ["c1", 1],
      ["c2", 1],
    ]);
    const c1 = got.find((r) => r.cell === "c1");
    expect(await (await env.BUCKET.get(c1?.id ?? ""))?.text()).toBe("c1 の中身");
  });
});

describe("バージョン", () => {
  it("同じ中身はバージョンを上げず、違う中身は上げる", async () => {
    await putKey("c1", "tray_versions000000000");
    const key = "tray_versions000000000";
    const first = await send(await putRequest({ name: "v", body: "1", key }));
    const same = await send(await putRequest({ name: "v", body: "1", key }));
    const next = await send(await putRequest({ name: "v", body: "2", key }));
    expect([first.body.version, first.body.bumped]).toEqual([1, true]);
    expect([same.body.version, same.body.bumped]).toEqual([1, false]);
    expect([next.body.version, next.body.bumped]).toEqual([2, true]);
    expect(first.body.url).toBe(next.body.url);
    const ups = await env.DB.prepare("SELECT version FROM uploads ORDER BY version").all<{ version: number }>();
    expect(ups.results.map((u) => u.version)).toEqual([1, 2]);
  });

  it("同じ名前への2つの上げが重なって R2 と D1 が食い違っても、出し直しで R2 の中身が直る", async () => {
    const key = "tray_race00000000000000";
    await putKey("c1", key);
    const first = await send(await putRequest({ name: "race", body: "A", key }));
    const id = String(first.body.url).split("/").pop() as string;
    // 重なりの結果: R2 は後から書いた B、D1 は先に更新を終えた A を持つ
    const b = new TextEncoder().encode("B");
    const bsha = [...new Uint8Array(await crypto.subtle.digest("SHA-256", b))].map((x) => x.toString(16).padStart(2, "0")).join("");
    await env.BUCKET.put(id, b, { customMetadata: { sha256: bsha } });
    const again = await send(await putRequest({ name: "race", body: "A", key }));
    expect(again.body.bumped).toBe(true);
    expect(await (await env.BUCKET.get(id))?.text()).toBe("A");
  });
});

describe("名前とヘッダ", () => {
  const key = "tray_names000000000000";
  const bad: [string, string][] = [
    ["%2F で段をまたぐ名前", "a%2Fb"],
    ["制御文字", "a%0Ab"],
    ["解けない符号化", "a%E3%81"],
    ["200 バイトを超える名前", "%E3%81%82".repeat(67)],
  ];
  for (const [what, seg] of bad) {
    it(`${what}を断る`, async () => {
      await putKey("c1", key);
      const req = await putRequest({ name: "ok", body: "x", key });
      const res = await send(new Request(`${UPLOAD_ORIGIN}/v1/reports/andon/${seg}`, req));
      expect(res.status).toBe(400);
    });
  }

  it("200 バイトちょうどの UTF-8 の名前は受ける", async () => {
    await putKey("c1", key);
    const name = `${"あ".repeat(66)}xy`;
    expect(new TextEncoder().encode(name).length).toBe(200);
    expect((await send(await putRequest({ name, body: "x", key }))).status).toBe(200);
  });

  it("プロジェクトの無い報告は - で受ける", async () => {
    await putKey("c1", key);
    expect((await send(await putRequest({ project: "-", name: "n", body: "x", key }))).status).toBe(200);
    expect(await rows("-")).toHaveLength(1);
  });

  it("本文と合わない sha256 と、知らない種類を断る", async () => {
    await putKey("c1", key);
    const req = await putRequest({ name: "s", body: "x", key });
    req.headers.set("X-Tray-Sha256", "0".repeat(64));
    expect((await send(req)).status).toBe(400);
    expect((await send(await putRequest({ name: "k", body: "x", key, kind: "other" }))).status).toBe(400);
    expect(await rows()).toHaveLength(0);
  });

  it("20 MiB を超える本文を断る", async () => {
    await putKey("c1", key);
    const big = "a".repeat(20 * 1024 * 1024 + 1);
    expect((await send(await putRequest({ name: "big", body: big, key }))).status).toBe(413);
  });

  it("GET /v1/meta は鍵なしでバージョンを返す", async () => {
    const res = await send(new Request(`${UPLOAD_ORIGIN}/v1/meta`));
    expect(res.status).toBe(200);
    expect(typeof res.body.version).toBe("string");
  });
});
