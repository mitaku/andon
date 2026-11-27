// 一覧の面の画面（ADR-0064 決めたこと 20・21・23・24・19）。ローカルの tray を一時の置き場に起動して、スマホの幅で操作する。
import { devices, expect, test } from "@playwright/test";
import { put, startTray, stopTray, type Tray } from "./tray";

// スマホの1列を先に作る（決めたこと 21）。WebKit は使わず、Chromium にスマホの幅と touch を与える
test.use({ viewport: devices["iPhone 13"].viewport, hasTouch: true, isMobile: true });

let tray: Tray;

test.beforeAll(async () => {
  tray = await startTray("cell-a");
});

test.afterAll(async () => {
  await stopTray(tray);
});

test("一覧はセルとプロジェクトでまとめて新しい順に並べ、絞り込め、開いた報告は未読から消えて30日に既読で残る", async ({ page }) => {
  await put(tray, "a1", "<p>a1</p>", "text/html; charset=utf-8", "proj-a");
  await put(tray, "b1", "<p>b1</p>", "text/html; charset=utf-8", "proj-b");
  await put(tray, "a2", "<p>a2</p>", "text/html; charset=utf-8", "proj-a");

  await page.goto(`${tray.list_origin}/`);
  // まとめは中の最新の報告が新しい順（proj-a の a2 が最後に上がった）。行も新しい順
  await expect(page.locator("section h2")).toHaveText(["cell-a / proj-a", "cell-a / proj-b"]);
  const a = page.getByRole("region", { name: "cell-a / proj-a" });
  await expect(a.getByRole("link")).toHaveCount(2);
  await expect(a.getByRole("link").first()).toContainText("a2");
  // 1行は、タイトル・日時・セル・プロジェクト・未読の印
  const row = a.getByRole("link").first();
  await expect(row.getByRole("img", { name: "未読" })).toBeVisible();
  await expect(row.locator("time")).toHaveAttribute("datetime", /^\d{4}-\d\d-\d\dT/);
  await expect(row).toContainText("cell-a");
  await expect(row).toContainText("proj-a");

  // セルとプロジェクトで絞る（UI SDK の Select）
  await page.getByTestId("group-filter").click();
  await page.getByRole("option", { name: "cell-a / proj-b" }).click();
  await expect(page.locator("section h2")).toHaveText(["cell-a / proj-b"]);

  // 開いて、帯の戻る口で戻る。開いた報告は未読から消える
  await page.getByRole("link", { name: /b1/ }).click();
  await expect(page.frameLocator("iframe").locator("p")).toHaveText("b1");
  await page.getByRole("link", { name: /一覧/ }).click();
  await expect(page).toHaveURL(`${tray.list_origin}/`);
  await expect(page.locator("section h2")).toHaveText(["cell-a / proj-a"]);

  // 30日には既読で残り、未読の印が無い。30日から開いたときは、戻る口が30日へ戻る
  await page.getByRole("link", { name: "30日", exact: true }).click();
  await expect(page).toHaveURL(`${tray.list_origin}/all`);
  const b1 = page.getByRole("link", { name: /b1/ });
  await expect(b1).toBeVisible();
  await expect(b1.getByRole("img", { name: "未読" })).toHaveCount(0);
  await b1.click();
  await page.getByRole("link", { name: /30日/ }).first().click();
  await expect(page).toHaveURL(`${tray.list_origin}/all`);

  // 古い報告は、最後に上がってから30日を過ぎた報告だけ（ここでは無い）
  await page.getByRole("link", { name: "古い報告", exact: true }).click();
  await expect(page.getByText("古い報告は無い。")).toBeVisible();
});

test("セルの画面で鍵を1度だけ見せ、失効と作り直しができ、直近7日の枚数を出す", async ({ page }) => {
  await page.goto(`${tray.list_origin}/cells`);
  // 報告を上げたセル（鍵は無い）が、上がった枚数と一緒に並ぶ
  const self = page.locator('[data-cell="cell-a"]');
  await expect(self).toContainText("鍵は無い");
  await expect(self).toContainText(/上がった \d+ 枚/);

  await page.getByLabel("セルの名前").fill("cell-b");
  await page.getByRole("button", { name: "鍵を作る" }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("cell-b の鍵");
  const key = (await page.getByTestId("issued-key").textContent()) ?? "";
  expect(key).toMatch(/^tray_[0-9a-f]{64}$/);
  await dialog.getByRole("button", { name: "閉じる" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  // 閉じたら鍵はもう画面に無い
  await expect(page.getByText(key)).toHaveCount(0);
  const b = page.locator('[data-cell="cell-b"]');
  await expect(b).toContainText("鍵は有効");

  // 同じ名前で作ろうとすると、作り直しの口へ促す（前の鍵を黙って効かなくしない）
  await page.getByLabel("セルの名前").fill("cell-b");
  await page.getByRole("button", { name: "鍵を作る" }).first().click();
  await expect(page.getByRole("alert")).toContainText("作り直す");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // 失効は確かめてから。やめれば何も変わらない
  await b.getByRole("button", { name: "失効させる" }).click();
  const ask = page.getByRole("alertdialog");
  await expect(ask).toContainText("cell-b の鍵を失効させる？");
  await ask.getByRole("button", { name: "やめる" }).click();
  await expect(b).toContainText("鍵は有効");
  await b.getByRole("button", { name: "失効させる" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "失効させる" }).click();
  await expect(b).toContainText("鍵は失効");

  // 作り直すと、別の鍵を1度だけ見せる
  await b.getByRole("button", { name: "作り直す" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "作り直す" }).click();
  const again = (await page.getByTestId("issued-key").textContent()) ?? "";
  expect(again).toMatch(/^tray_[0-9a-f]{64}$/);
  expect(again).not.toBe(key);
  await page.getByRole("dialog").getByRole("button", { name: "閉じる" }).click();
  await expect(b).toContainText("鍵は有効");
});

test("画面の一覧が古いままでも、有効な鍵を持つセルの鍵を作る口は前の鍵を効かなくしない", async ({ page }) => {
  await page.goto(`${tray.list_origin}/cells`);
  await expect(page.locator("main")).toContainText("cell-a");
  // 画面を開いたあと、別の画面（ここでは API を直に）で cell-c の鍵を作る。この画面の一覧には cell-c が無い
  const first = await page.evaluate(async () => {
    const res = await fetch("/api/cells/cell-c/key", { method: "POST" });
    return ((await res.json()) as { key: string }).key;
  });
  expect(first).toMatch(/^tray_[0-9a-f]{64}$/);
  await expect(page.locator('[data-cell="cell-c"]')).toHaveCount(0);

  await page.getByLabel("セルの名前").fill("cell-c");
  await page.getByRole("button", { name: "鍵を作る" }).first().click();
  await expect(page.getByRole("alert")).toContainText("作り直す");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  // 一覧は読み直され、前の鍵は有効のまま
  await expect(page.locator('[data-cell="cell-c"]')).toContainText("鍵は有効");
});

test("ホーム画面への追加は manifest とアイコンだけで支え、service worker を置かない", async ({ page, request }) => {
  await page.goto(`${tray.list_origin}/`);
  const manifestHref = await page.locator('link[rel="manifest"]').getAttribute("href");
  const touchIcon = await page.locator('link[rel="apple-touch-icon"]').getAttribute("href");
  expect(manifestHref).toBe("/pwa/manifest.webmanifest");
  expect(touchIcon?.startsWith("/pwa/")).toBe(true);

  const res = await request.get(`${tray.list}${manifestHref}`, { headers: { Host: new URL(tray.list_origin).host } });
  expect(res.status()).toBe(200);
  const manifest = (await res.json()) as { start_url: string; display: string; icons: { src: string; sizes: string }[] };
  expect(manifest.start_url).toBe("/");
  expect(manifest.display).toBe("standalone");
  expect(manifest.icons.map((i) => i.sizes)).toEqual(expect.arrayContaining(["192x192", "512x512"]));
  for (const src of [...manifest.icons.map((i) => i.src), touchIcon as string]) {
    // アイコンも /pwa/ の下に置く（Access はこのパスだけを素通しにする）
    expect(src.startsWith("/pwa/")).toBe(true);
    const icon = await request.get(`${tray.list}${src}`, { headers: { Host: new URL(tray.list_origin).host } });
    expect(icon.status(), src).toBe(200);
    expect(icon.headers()["content-type"], src).toBe("image/png");
  }
  // service worker は登録しない
  await page.waitForTimeout(300);
  expect(await page.evaluate(async () => (await navigator.serviceWorker.getRegistrations()).length)).toBe(0);
});

// 背景の色（デザインシステムの background の HEX。light は #f9f5ed、dark は #0e1623）
for (const [scheme, background] of [
  ["light", [0xf9, 0xf5, 0xed]],
  ["dark", [0x0e, 0x16, 0x23]],
] as const) {
  test(`色はデザインシステムの ${scheme} の組を prefers-color-scheme で使う`, async ({ browser }) => {
    const ctx = await browser.newContext({ colorScheme: scheme });
    const page = await ctx.newPage();
    await page.goto(`${tray.list_origin}/cells`);
    await expect(page.locator("main")).toBeVisible();
    // ビルドは OKLCH を lab などに書き直すので、キャンバスに塗って sRGB で比べる
    const rgb = await page.evaluate(() => {
      const c = document.createElement("canvas").getContext("2d") as CanvasRenderingContext2D;
      c.fillStyle = getComputedStyle(document.documentElement).backgroundColor;
      c.fillRect(0, 0, 1, 1);
      return [...c.getImageData(0, 0, 1, 1).data.slice(0, 3)];
    });
    for (const [i, v] of background.entries()) expect(Math.abs((rgb[i] as number) - v), `${rgb} と ${background}`).toBeLessThanOrEqual(1);
    await ctx.close();
  });
}
