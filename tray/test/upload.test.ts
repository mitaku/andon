// アップロードの口（受け入れ条件 11・14）
import { env } from "cloudflare:test";
import { describe, expect, it } from "vitest";
import contract from "../contract/upload-v1.json";
import { sha256Hex } from "../src/lib/http";
import { listAPI } from "../src/lib/list-api";
import { viewFace } from "../src/lib/view-face";
import { OLD_CONTENT_MS, uploadAPI } from "../src/lib/upload-api";
import upload from "../src/upload";
import { LIST_ORIGIN, listEnv, putKey, putRequest, UPLOAD_ORIGIN, uploadEnv, viewEnv } from "./helpers";

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
    expect(await (await env.BUCKET.get(`${c1?.id}/${c1?.sha256}`))?.text()).toBe("c1 の中身");
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

  it("同じ名前へ違う中身を並行に上げても、開いて読む中身は D1 が記録した中身と同じ", async () => {
    const key = "tray_race00000000000000";
    await putKey("c1", key);
    // 食い違いが残る順を作る: A の R2 の書き込み → B の R2 の書き込みと D1 の更新 → A の D1 の更新。
    // R2 と D1 の口を包み、A の書き込みが終わったら B を送り、A の更新は B の更新が終わるまで待たせる
    const shaA = await sha256Hex("# A");
    const shaB = await sha256Hex("# B");
    let aPut!: () => void;
    let bUpdated!: () => void;
    const aPutDone = new Promise<void>((ok) => (aPut = ok));
    const bUpdateDone = new Promise<void>((ok) => (bUpdated = ok));
    const bucket = new Proxy(env.BUCKET, {
      get(target, prop) {
        if (prop !== "put") return Reflect.get(target, prop).bind(target);
        return async (k: string, value: ArrayBuffer, opts?: R2PutOptions) => {
          const r = await target.put(k, value, opts);
          if (new TextDecoder().decode(value) === "# A") aPut();
          return r;
        };
      },
    });
    const db = new Proxy(env.DB, {
      get(target, prop) {
        if (prop !== "prepare") return Reflect.get(target, prop).bind(target);
        return (sql: string) => {
          const stmt = target.prepare(sql);
          if (!sql.startsWith("UPDATE reports SET version")) return stmt;
          return {
            bind: (...args: unknown[]) => {
              const bound = stmt.bind(...args);
              return {
                first: async <T>() => {
                  if (args[1] === shaA) await bUpdateDone;
                  const r = await bound.first<T>();
                  if (args[1] === shaB) bUpdated();
                  return r;
                },
              };
            },
          };
        };
      },
    });
    const racing = { ...uploadEnv(), DB: db, BUCKET: bucket };
    const sendRacing = async (body: string) => {
      const res = await upload.fetch(await putRequest({ name: "race", title: body.slice(2), body, key }), racing);
      return { status: res.status, body: (await res.json()) as Record<string, unknown> };
    };
    const a = sendRacing("# A");
    await aPutDone;
    const [ra, rb] = await Promise.all([a, sendRacing("# B")]);
    expect([ra.status, rb.status]).toEqual([200, 200]);

    const id = String(ra.body.url).split("/").pop() as string;
    const row = await env.DB.prepare("SELECT sha256, title FROM reports WHERE id = ?1").bind(id).first<{ sha256: string; title: string }>();
    expect([row?.title, row?.sha256]).toEqual(["A", shaA]);
    const open = await listAPI(
      new Request(`${LIST_ORIGIN}/api/reports/${id}/open`, { method: "POST", headers: { Origin: LIST_ORIGIN } }),
      listEnv(),
    );
    const { url } = (await open.json()) as { url: string };
    const page = await (await viewFace(new Request(url), viewEnv())).text();
    expect(page, "D1 は A を記録したのに、読めた中身が違う").toContain("<h1>A</h1>");
    // 同じ中身の出し直しは、読めている中身のまま上げない
    const again = await send(await putRequest({ name: "race", title: "A", body: "# A", key }));
    expect(again.body.bumped).toBe(false);
  });
});

describe("前の中身の掃除", () => {
  const keys = async (id: string) => (await env.BUCKET.list({ prefix: `${id}/` })).objects.map((o) => o.key).sort();
  const at = async (name: string, body: string, now: number, project = "andon") => {
    const res = await uploadAPI(await putRequest({ project, name, body }), uploadEnv(), "c1", now);
    return String(((await res.json()) as { url: string }).url).split("/").pop() as string;
  };

  it("上げ直すと、同じ報告の前の中身のうち R2 に置いてから1日を過ぎたものだけを消す", async () => {
    const t = Date.now();
    const id = await at("clean", "1", t);
    const other = await at("clean", "x", t, "other");
    await at("clean", "2", t);
    const [s1, s2, s3, s4] = await Promise.all(["1", "2", "3", "4"].map((b) => sha256Hex(b)));
    expect(await keys(id), "置いたばかりの前の中身は残す").toEqual([`${id}/${s1}`, `${id}/${s2}`].sort());
    await at("clean", "3", t + OLD_CONTENT_MS - 60_000);
    expect(await keys(id), "1日に満たない前の中身は残す").toEqual([`${id}/${s1}`, `${id}/${s2}`, `${id}/${s3}`].sort());
    await at("clean", "4", t + OLD_CONTENT_MS + 60_000);
    expect(await keys(id), "1日を過ぎた前の中身を消し、今の中身は残す").toEqual([`${id}/${s4}`]);
    expect(await keys(other), "ほかの報告の中身には触れない").toHaveLength(1);
    // 同じ中身の出し直しはバージョンを上げず、何も消さない
    await at("clean", "4", t + 3 * OLD_CONTENT_MS);
    expect(await keys(id)).toEqual([`${id}/${s4}`]);
  });

  // 重なる上げ直しの中身: 1日より前の中身へ戻す（X）と、新しい中身（W）
  for (const back of ["X", "W"]) {
    it(`掃除が一覧を読んだあとに、上げ直し（${back}）が重なっても、D1 が指す中身は消さない`, async () => {
      const t = Date.now();
      const name = `race-${back}`;
      const id = await at(name, "X", t);
      await at(name, "Y", t + 60_000);
      const sx = await sha256Hex(back);
      // A（中身 Z）の掃除が一覧を読んだ直後に、B（中身 X に戻す）が上げ直しを済ませる。消す直前に D1 を読み直す所と
      // 消す所の間に重なった分は、次の検査（同じ中身の上げ直しが置き直す）が受ける
      let raced = false;
      const racing = new Proxy(env.BUCKET, {
        get(target, prop) {
          if (prop === "list") {
            return async (opts?: R2ListOptions) => {
              const listed = await target.list(opts);
              if (!raced) {
                raced = true;
                await uploadAPI(await putRequest({ name, body: back }), uploadEnv(), "c1", t + 2 * OLD_CONTENT_MS);
              }
              return listed;
            };
          }
          return Reflect.get(target, prop).bind(target);
        },
      });
      await uploadAPI(await putRequest({ name, body: "Z" }), { ...uploadEnv(), BUCKET: racing }, "c1", t + 2 * OLD_CONTENT_MS);
      expect(raced).toBe(true);
      const row = await env.DB.prepare("SELECT sha256 FROM reports WHERE id = ?1").bind(id).first<{ sha256: string }>();
      expect(row?.sha256).toBe(sx);
      expect(await env.BUCKET.head(`${id}/${sx}`), "D1 が指す中身が R2 に在る").not.toBeNull();
    });
  }

  it("D1 が指す中身が R2 に無ければ、同じ中身の上げ直しが置き直す（バージョンは上げない）", async () => {
    const id = await at("heal", "1", Date.now());
    const s1 = await sha256Hex("1");
    await env.BUCKET.delete(`${id}/${s1}`);
    const res = await uploadAPI(await putRequest({ name: "heal", body: "1" }), uploadEnv(), "c1");
    expect(await res.json()).toMatchObject({ version: 1, bumped: false });
    expect(await env.BUCKET.head(`${id}/${s1}`)).not.toBeNull();
  });

  it("前の中身を消せなくても、上げは成り立つ", async () => {
    const broken = new Proxy(env.BUCKET, {
      get(target, prop) {
        if (prop === "list") return async () => Promise.reject(new Error("list が落ちた"));
        return Reflect.get(target, prop).bind(target);
      },
    });
    await uploadAPI(await putRequest({ name: "keep", body: "1" }), uploadEnv(), "c1");
    const res = await uploadAPI(await putRequest({ name: "keep", body: "2" }), { ...uploadEnv(), BUCKET: broken }, "c1", Date.now() + 2 * OLD_CONTENT_MS);
    expect(res.status).toBe(200);
    expect(((await res.json()) as { version: number }).version).toBe(2);
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

describe("枚数（andon tray status）", () => {
  const key = "tray_stats000000000000";
  it("GET /v1/stats はそのセルの直近の日数に上げた回数と開いた報告の数だけを返す", async () => {
    await putKey("c1", key);
    await putKey("c2", "tray_otherkey000000000");
    const now = Date.now();
    for (const body of ["v1", "v2"]) expect((await send(await putRequest({ name: "a", body, key }))).status).toBe(200);
    expect((await send(await putRequest({ name: "b", body: "x", key }))).status).toBe(200);
    expect((await send(await putRequest({ name: "c", body: "x", key: "tray_otherkey000000000" }))).status).toBe(200);
    const ids = (await env.DB.prepare("SELECT id, cell FROM reports ORDER BY name").all<{ id: string; cell: string }>()).results;
    // a を2度、c（ほかのセル）を1度開き、8日前に上げて開いた古い記録を1つ置く
    for (const r of [ids[0], ids[0], ids[2]])
      await env.DB.prepare("INSERT INTO opens (report_id, version, at) VALUES (?1, 1, ?2)").bind(r?.id, now).run();
    await env.DB.prepare("INSERT INTO uploads (report_id, version, at) VALUES (?1, 9, ?2)").bind(ids[1]?.id, now - 8 * 86400000).run();
    await env.DB.prepare("INSERT INTO opens (report_id, version, at) VALUES (?1, 9, ?2)").bind(ids[1]?.id, now - 8 * 86400000).run();
    const stats = (days?: string) =>
      send(new Request(`${UPLOAD_ORIGIN}/v1/stats${days ? `?days=${days}` : ""}`, { headers: { Authorization: `Bearer ${key}` } }));
    expect(await stats()).toEqual({ status: 200, body: { days: 7, uploaded: 3, opened: 1 } });
    expect(await stats("9")).toEqual({ status: 200, body: { days: 9, uploaded: 4, opened: 2 } });
    for (const bad of ["0", "91", "x", "7.5"]) expect((await stats(bad)).status).toBe(400);
    // 鍵の無い要求は断る（ほかのセルの数を見せない）
    expect((await send(new Request(`${UPLOAD_ORIGIN}/v1/stats`))).status).toBe(401);
  });
});
