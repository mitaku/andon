// 一覧の面の API（受け入れ条件 3・10）
import { env } from "cloudflare:test";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import list from "../src/list";
import { LIST_HEADERS } from "../src/lib/http";
import { OLD_AFTER_MS } from "../src/lib/list-api";
import { uploadAPI } from "../src/lib/upload-api";
import { goodClaims, LIST_ORIGIN, listEnv, nowSeconds, putRequest, type Signer, signer, TEAM, uploadEnv } from "./helpers";

let team: Signer;
let certsFetches = 0;

beforeEach(async () => {
  team = await signer();
  certsFetches = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input instanceof Request ? input.url : input);
      if (url === `${TEAM}/cdn-cgi/access/certs`) {
        certsFetches++;
        return Response.json(team.jwks);
      }
      return new Response("not found", { status: 404 });
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

async function call(path: string, init: RequestInit & { jwt?: string | null } = {}, envOverride = {}) {
  const headers = new Headers(init.headers);
  const jwt = init.jwt === undefined ? await team.sign(goodClaims()) : init.jwt;
  if (jwt !== null) headers.set("Cf-Access-Jwt-Assertion", jwt);
  const res = await list.fetch(new Request(`${LIST_ORIGIN}${path}`, { ...init, headers }), { ...listEnv(), ...envOverride });
  return res;
}

async function upload(name: string, body: string, now = Date.now()) {
  const res = await uploadAPI(await putRequest({ name, body }), uploadEnv(), "c1", now);
  expect(res.status).toBe(200);
  const url = ((await res.json()) as { url: string }).url;
  return url.split("/").pop() as string;
}

async function reports(view = "unread") {
  const res = await call(`/api/reports?view=${view}`);
  expect(res.status).toBe(200);
  return ((await res.json()) as { reports: { id: string; name: string; unread: boolean }[] }).reports;
}

describe("Access の JWT（受け入れ条件 10）", () => {
  it("正しい JWT は通り、応答に一覧の面のヘッダが付く", async () => {
    const res = await call("/api/reports");
    expect(res.status).toBe(200);
    for (const [k, v] of Object.entries(LIST_HEADERS)) expect(res.headers.get(k)).toBe(v);
  });

  const cases: [string, () => Promise<string | null>][] = [
    ["JWT が無い", async () => null],
    ["aud が違う", () => team.sign(goodClaims({ aud: ["other-aud"] }))],
    ["期限が切れた", () => team.sign(goodClaims({ exp: nowSeconds() - 1 }))],
    ["iss が違う", () => team.sign(goodClaims({ iss: "https://other.cloudflareaccess.com" }))],
    ["まだ使えない（nbf）", () => team.sign(goodClaims({ nbf: nowSeconds() + 600 }))],
    [
      "署名が合わない（別の鍵で同じ kid）",
      async () => (await signer("k1")).sign(goodClaims()),
    ],
    [
      "署名を書き換えた",
      async () => {
        const t = await team.sign(goodClaims());
        return `${t.slice(0, -4)}${t.endsWith("AAAA") ? "BBBB" : "AAAA"}`;
      },
    ],
    [
      "中身を書き換えた（署名はそのまま）",
      async () => {
        const [h, , s] = (await team.sign(goodClaims())).split(".");
        const p = btoa(JSON.stringify(goodClaims({ email: "x@example.com" }))).replace(/=+$/, "");
        return `${h}.${p}.${s}`;
      },
    ],
    ["アルゴリズムが none", () => team.sign(goodClaims(), { alg: "none" })],
    ["アルゴリズムが HS256", () => team.sign(goodClaims(), { alg: "HS256" })],
    ["JWT の形でない", async () => "not-a-jwt"],
  ];
  for (const [what, make] of cases) {
    it(`${what}要求を断る`, async () => {
      const res = await call("/api/reports", { jwt: await make() });
      expect(res.status).toBe(403);
      for (const [k, v] of Object.entries(LIST_HEADERS)) expect(res.headers.get(k)).toBe(v);
    });
  }

  it("Access の設定が無ければ、正しい形の JWT でも断る", async () => {
    expect((await call("/api/reports", {}, { ACCESS_AUD: "" })).status).toBe(403);
    expect((await call("/api/reports", {}, { ACCESS_TEAM_DOMAIN: undefined })).status).toBe(403);
  });

  it("公開鍵は控え、知らない kid のときだけ取り直す", async () => {
    await call("/api/reports");
    await call("/api/reports");
    expect(certsFetches).toBe(1);
    const other = await signer("k2");
    team.jwks.keys.push(...other.jwks.keys);
    expect((await call("/api/reports", { jwt: await other.sign(goodClaims()) })).status).toBe(200);
    expect(certsFetches).toBe(2);
  });

  it("/api/ の外は Worker が答えない（画面のファイルは Worker を通らない）", async () => {
    expect((await call("/", { jwt: null })).status).toBe(404);
  });

  it("パーセントで符号化したパスで Access の確かめを外せない", async () => {
    for (const p of ["/%61pi/reports", "/api%2Freports", "/%2Fapi/reports"]) {
      expect((await call(p, { jwt: null })).status, p).toBe(404);
    }
  });
});

describe("Origin（受け入れ条件 10）", () => {
  it("Origin の無い・違う書き込みを断り、同じオリジンだけ通す", async () => {
    const id = await upload("o", "x");
    const path = `/api/reports/${id}/open`;
    expect((await call(path, { method: "POST" })).status).toBe(403);
    expect((await call(path, { method: "POST", headers: { Origin: "https://evil.example.com" } })).status).toBe(403);
    expect((await call(path, { method: "POST", headers: { Origin: "null" } })).status).toBe(403);
    expect((await reports())[0]?.unread).toBe(true);
    expect((await call(path, { method: "POST", headers: { Origin: LIST_ORIGIN } })).status).toBe(200);
    expect(await reports()).toHaveLength(0);
  });

  it("GET で既読を書けない", async () => {
    const id = await upload("g", "x");
    expect((await call(`/api/reports/${id}/open`)).status).toBe(405);
    expect((await reports())[0]?.unread).toBe(true);
  });
});

describe("既読", () => {
  const open = (id: string) => call(`/api/reports/${id}/open`, { method: "POST", headers: { Origin: LIST_ORIGIN } });

  it("開くと既読になり、同じ中身の出し直しは既読のまま、違う中身は未読に戻る", async () => {
    const id = await upload("r", "1");
    const res = await open(id);
    const body = (await res.json()) as { url: string; version: number };
    expect(body.version).toBe(1);
    expect(body.url).toMatch(new RegExp(`^https://tray-view\\.example\\.com/r/${id}/[0-9a-f]{64}\\?e=\\d+&s=[A-Za-z0-9_-]+$`));
    expect(await reports()).toHaveLength(0);
    await upload("r", "1");
    expect(await reports()).toHaveLength(0);
    await upload("r", "2");
    expect((await reports()).map((r) => r.id)).toEqual([id]);
    const opens = await env.DB.prepare("SELECT version FROM opens").all<{ version: number }>();
    expect(opens.results.map((o) => o.version)).toEqual([1]);
  });

  it("知らない id を開けない", async () => {
    expect((await open("0".repeat(32))).status).toBe(404);
    expect((await open("../x")).status).toBe(404);
  });
});

describe("30 日（受け入れ条件 3）", () => {
  it("最後に上がった日時が 30 日より前の報告は、一覧に出ず、古い報告のページに出る", async () => {
    const now = Date.now();
    const oldId = await upload("old", "x", now - OLD_AFTER_MS - 60_000);
    const edgeId = await upload("edge", "x", now - OLD_AFTER_MS + 60_000);
    const newId = await upload("new", "x", now);
    const ids = (rs: { id: string }[]) => rs.map((r) => r.id).sort();
    expect(ids(await reports("unread"))).toEqual([edgeId, newId].sort());
    expect(ids(await reports("recent"))).toEqual([edgeId, newId].sort());
    expect(ids(await reports("old"))).toEqual([oldId]);
    // 消してはいない。古い報告も開ける
    const res = await call(`/api/reports/${oldId}/open`, { method: "POST", headers: { Origin: LIST_ORIGIN } });
    expect(res.status).toBe(200);
    // 上げ直すと、最後に上がった日時が変わり、一覧に戻る
    await upload("old", "y", now);
    expect(ids(await reports("unread"))).toEqual([edgeId, newId, oldId].sort());
    expect(await reports("old")).toHaveLength(0);
  });

  it("知らない view を断る", async () => {
    expect((await call("/api/reports?view=all")).status).toBe(400);
  });
});

describe("セルの鍵", () => {
  const post = (path: string) => call(path, { method: "POST", headers: { Origin: LIST_ORIGIN } });

  it("発行した鍵は1度だけ返り、発行し直すと前の鍵は効かず、失効させると今の鍵も効かない", async () => {
    const { key: k1 } = (await (await post("/api/cells/c9/key")).json()) as { key: string };
    expect(k1).toMatch(/^tray_[0-9a-f]{64}$/);
    const stored = await env.DB.prepare("SELECT hash FROM keys WHERE cell = 'c9'").first<{ hash: string }>();
    expect(stored?.hash).not.toBe(k1);
    const { key: k2 } = (await (await post("/api/cells/c9/key")).json()) as { key: string };
    const { cellForKey } = await import("../src/lib/upload-api");
    const req = (k: string) => new Request("https://x/", { headers: { Authorization: `Bearer ${k}` } });
    expect(await cellForKey(req(k1), env.DB)).toBeNull();
    expect(await cellForKey(req(k2), env.DB)).toBe("c9");
    expect((await post("/api/cells/c9/revoke")).status).toBe(200);
    expect(await cellForKey(req(k2), env.DB)).toBeNull();
    const cells = (await (await call("/api/cells")).json()) as { cells: { cell: string; revoked: boolean }[] };
    expect(cells.cells).toEqual([expect.objectContaining({ cell: "c9", revoked: true })]);
  });

  it("鍵の口も Access と Origin を確かめる", async () => {
    expect((await call("/api/cells/c9/key", { method: "POST", headers: { Origin: LIST_ORIGIN }, jwt: null })).status).toBe(403);
    expect((await call("/api/cells/c9/key", { method: "POST" })).status).toBe(403);
    expect(await env.DB.prepare("SELECT count(*) AS n FROM keys").first("n")).toBe(0);
  });
});
