// 受け入れ条件 13: 報告の中のスクリプトは、一覧の面の API を呼べず、一覧の面の文書に触れない。
// 報告のスクリプトが枠を外のページへ移しても、移った先は保存領域を使えず、一覧の面の画面を別のページへ移せない。
// あわせて、開くと既読になることを見る（ADR-0064 決めたこと 28）。
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";
import { put, startTray, stopTray, type Tray } from "./tray";

let tray: Tray;
let outside: Server;
let outsideOrigin: string;
const hits: string[] = [];

// 外のページ: 枠の中から移ってくる先。保存領域と、一覧の面への手出しを試し、結果を文書に書く
const AWAY = `<!doctype html><title>外</title><p id="away">…</p><script>
const out = {};
try { localStorage.setItem("k", "v"); out.localStorage = "ok"; } catch (e) { out.localStorage = e.name; }
try { sessionStorage.setItem("k", "v"); out.sessionStorage = "ok"; } catch (e) { out.sessionStorage = e.name; }
try { out.parent = String(parent.document.title); } catch (e) { out.parent = e.name; }
try { top.location.href = location.origin + "/top-from-away"; out.top = "tried"; } catch (e) { out.top = e.name; }
document.getElementById("away").textContent = JSON.stringify(out);
</script>`;

test.beforeAll(async () => {
  tray = await startTray();
  outside = createServer((req, res) => {
    hits.push(req.url ?? "");
    res.setHeader("Content-Type", "text/html; charset=utf-8");
    res.end(req.url === "/away" ? AWAY : "<!doctype html><title>外の空のページ</title>");
  });
  await new Promise<void>((ok) => outside.listen(0, "127.0.0.1", ok));
  outsideOrigin = `http://127.0.0.1:${(outside.address() as AddressInfo).port}`;
});

test.afterAll(async () => {
  await stopTray(tray);
  await new Promise((ok) => outside.close(ok));
});

// 報告: 読み込みで一覧の面への手出しを試し、ボタンで枠を外のページへ移す
function attack() {
  return `<!doctype html><title>攻め</title><p id="r">…</p><button id="go">外へ</button><script>
const out = {};
try { out.parent = String(parent.document.title); } catch (e) { out.parent = e.name; }
try { out.parentStorage = String(parent.localStorage.length); } catch (e) { out.parentStorage = e.name; }
try { localStorage.setItem("k", "v"); out.localStorage = "ok"; } catch (e) { out.localStorage = e.name; }
try { top.location.href = ${JSON.stringify(`${tray.list_origin}/?moved=1`)}; out.top = "tried"; } catch (e) { out.top = e.name; }
fetch(${JSON.stringify(`${tray.list_origin}/api/reports`)}, { method: "GET" })
  .then(() => { out.fetch = "ok"; }, (e) => { out.fetch = e.name; })
  .then(() => fetch(${JSON.stringify(`${tray.list_origin}/api/cells/x/key`)}, { method: "POST" }))
  .then(() => { out.post = "ok"; }, (e) => { out.post = e.name; })
  .finally(() => { document.getElementById("r").textContent = JSON.stringify(out); });
document.getElementById("go").onclick = () => { location.href = ${JSON.stringify(`${outsideOrigin}/away`)}; };
</script>`;
}

test("報告のスクリプトは一覧の面に触れず、外へ移っても sandbox の中に居て、開くと既読になる", async ({ page }) => {
  // アップロードの答えの URL（一覧の面の報告の画面）を開く
  const { url } = await put(tray, "attack", attack());
  await page.goto(url);
  const listPage = page.url();

  const frame = page.frameLocator("iframe");
  const r = frame.locator("#r");
  await expect(r).not.toHaveText("…");
  const got = JSON.parse((await r.textContent()) ?? "{}");
  expect(got.parent).toBe("SecurityError");
  expect(got.parentStorage).toBe("SecurityError");
  expect(got.localStorage).toBe("SecurityError");
  expect(got.fetch).toBe("TypeError");
  expect(got.post).toBe("TypeError");

  // 枠の要素にも sandbox が付く（応答のヘッダだけに頼らない）
  const sandbox = await page.locator("iframe").getAttribute("sandbox");
  expect(sandbox?.split(" ").sort()).toEqual(["allow-popups", "allow-popups-to-escape-sandbox", "allow-scripts"]);

  // 外のページへ移る
  await frame.locator("#go").click();
  const away = frame.locator("#away");
  await expect(away).not.toHaveText("…");
  const there = JSON.parse((await away.textContent()) ?? "{}");
  expect(there.localStorage).toBe("SecurityError");
  expect(there.sessionStorage).toBe("SecurityError");
  expect(there.parent).toBe("SecurityError");
  expect(hits).toContain("/away");
  expect(hits).not.toContain("/top-from-away");

  // 一覧の面の画面はどこへも移っていない
  await page.waitForTimeout(500);
  expect(page.url()).toBe(listPage);
  expect(page.frames().map((f) => f.url())).toContain(`${outsideOrigin}/away`);

  // 一覧の API を呼べないので、鍵は作られていない。開いた報告は既読になった
  // （/api/cells は報告を上げた鍵の無いセルも並べるので、鍵を持つセルを数える）
  const keys = await page.evaluate(
    async () => (await (await fetch("/api/cells")).json()).cells.filter((c: { key: string }) => c.key !== "none").length,
  );
  expect(keys).toBe(0);
  const unread = await page.evaluate(async () => (await (await fetch("/api/reports")).json()).reports.length);
  expect(unread).toBe(0);
});

test("一覧の面は、ほかのページの枠に入らない", async ({ page }) => {
  // 枠の外で開けば、画面の根（#app）は在る（下の否定が何も見ずに通らないための確かめ）
  await page.goto(`${tray.list_origin}/`);
  await expect(page.locator("#app")).toHaveCount(1);
  await page.goto(`${outsideOrigin}/frame`);
  await page.setContent(`<iframe src="${tray.list_origin}/"></iframe>`);
  await page.waitForTimeout(1000);
  const child = page.frames().find((f) => f.url().startsWith(tray.list_origin));
  // frame-ancestors 'none' で読み込みが止まり、一覧の面の文書は枠の中に無い
  const loaded = child ? await child.evaluate(() => document.getElementById("app") !== null).catch(() => false) : false;
  expect(loaded).toBe(false);
});
