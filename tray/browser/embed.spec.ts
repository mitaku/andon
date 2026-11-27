// ADR-0064 受け入れ条件 9（ブラウザの側）: andon が埋め込んだ写し（internal/tray の Embed が出す形で、Go の TestEmbed が固定する。
// <head> に <base target="_blank">、# で始まるリンクに target="_self"）は、# のリンクが同じ枠の中で動き、外へのリンクは新しいタブで開く。
import { expect, test } from "@playwright/test";
import { put, startTray, stopTray, type Tray } from "./tray";

let tray: Tray;

test.beforeAll(async () => {
  tray = await startTray();
});

test.afterAll(async () => {
  await stopTray(tray);
});

const EMBEDDED = `<!doctype html><html><head><base target="_blank"/><title>埋めた報告</title></head><body>
<a id="toc" href="#sec" target="_self">目次</a> <a id="out" href="https://example.com/">外</a>
<div style="height:3000px"></div><h2 id="sec">節</h2></body></html>`;

test("埋めた写しの # のリンクは同じ枠で動き、外へのリンクは新しいタブで開く", async ({ page, context }) => {
  const { url } = await put(tray, "embedded", EMBEDDED);
  await page.goto(url);
  const listPage = page.url();
  const frame = page.frameLocator("iframe");

  // # のリンク: 枠の文書の URL の # が変わり、一覧の面の画面は移らず、新しいタブも開かない
  const pages = context.pages().length;
  await frame.locator("#toc").click();
  await expect.poll(() => page.frames().some((f) => f.url().endsWith("#sec"))).toBe(true);
  expect(page.url()).toBe(listPage);
  expect(context.pages().length).toBe(pages);

  // 外へのリンク: 新しいタブで開き、枠の中は外のページへ移らない（外へは出さず、ここで答える）
  await context.route("https://example.com/**", (r) => r.fulfill({ body: "<title>外</title>", contentType: "text/html" }));
  const [popup] = await Promise.all([page.waitForEvent("popup"), frame.locator("#out").click()]);
  await popup.waitForLoadState();
  expect(popup.url()).toBe("https://example.com/");
  expect(page.frames().some((f) => f.url().startsWith("https://example.com"))).toBe(false);
  expect(page.url()).toBe(listPage);
});
