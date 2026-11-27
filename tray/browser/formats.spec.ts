// 受け入れ条件 6: Markdown・画像・PDF・テキストが開け、ほかの形式はダウンロードになる（ADR-0064 決めたこと 18・22）。
// 一覧の面の画面から開く。PDF は報告の面の CSP の sandbox の下で、新しいタブの PDF ビューアに出ることを見る。
// PDF ビューアは headless shell に無い（PDF がダウンロードになる）ので、この検査は full の Chromium で回す。
import { deflateSync } from "node:zlib";
import { expect, type Page, test } from "@playwright/test";
import { put, startTray, stopTray, type Tray } from "./tray";

test.use({ channel: "chromium" });

let tray: Tray;

test.beforeAll(async () => {
  tray = await startTray();
});

test.afterAll(async () => {
  await stopTray(tray);
});

// pdf: 文字を1行ずつ書いた n ページの PDF（xref の位置を数えて作る）
function pdf(pages: number): Buffer {
  const objs: string[] = [];
  const kids = Array.from({ length: pages }, (_, i) => `${4 + i * 2} 0 R`).join(" ");
  objs.push("<< /Type /Catalog /Pages 2 0 R >>");
  objs.push(`<< /Type /Pages /Kids [${kids}] /Count ${pages} >>`);
  objs.push("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>");
  for (let i = 0; i < pages; i++) {
    const text = `BT /F1 24 Tf 72 720 Td (tray page ${i + 1} / ${pages}) Tj ET`;
    objs.push(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents ${5 + i * 2} 0 R >>`);
    objs.push(`<< /Length ${text.length} >>\nstream\n${text}\nendstream`);
  }
  let out = "%PDF-1.4\n";
  const at: number[] = [];
  objs.forEach((o, i) => {
    at.push(out.length);
    out += `${i + 1} 0 obj\n${o}\nendobj\n`;
  });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n${at.map((n) => `${String(n).padStart(10, "0")} 00000 n \n`).join("")}`;
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(out, "latin1");
}

// png: 1色の w×h の PNG
function png(w: number, h: number): Buffer {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  const crc = (b: Buffer) => {
    let c = 0xffffffff;
    for (const x of b) c = (crcTable[(c ^ x) & 0xff] as number) ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type: string, data: Buffer) => {
    const t = Buffer.from(type, "latin1");
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(Buffer.concat([t, data])));
    return Buffer.concat([len, t, data, sum]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr.set([8, 2, 0, 0, 0], 8);
  const row = Buffer.concat([Buffer.from([0]), Buffer.from(Array.from({ length: w }, () => [23, 78, 134]).flat())]);
  const raw = Buffer.concat(Array.from({ length: h }, () => row));
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

// openFromList: 一覧（未読）から、タイトルの行を押して報告の画面へ移る
async function openFromList(page: Page, title: string) {
  await page.goto(`${tray.list_origin}/`);
  await page.getByRole("link", { name: new RegExp(title) }).click();
  await expect(page).toHaveURL(/\/r\/[0-9a-f]{32}$/);
}

async function unreadTitles(page: Page): Promise<string[]> {
  return page.evaluate(async () =>
    ((await (await fetch("/api/reports?view=unread")).json()).reports as { title: string }[]).map((r) => r.title),
  );
}

test("Markdown・テキスト・画像は枠の中に開き、開くと既読になる", async ({ page }) => {
  await put(tray, "md", "# 見出しの報告\n\n本文の **太字**。\n\n<b id=\"raw\">中の HTML</b>\n", "text/markdown; charset=utf-8");
  await put(tray, "txt", "<script>テキスト</script>\n2行目\n", "text/plain; charset=utf-8");
  await put(tray, "img", png(120, 60), "image/png");

  await openFromList(page, "md");
  let frame = page.frameLocator("iframe");
  await expect(frame.locator("h1")).toHaveText("見出しの報告");
  await expect(frame.locator("strong")).toHaveText("太字");
  await expect(frame.locator("#raw")).toHaveText("中の HTML");
  await expect(page.getByRole("link", { name: /一覧/ })).toBeVisible();

  await openFromList(page, "txt");
  frame = page.frameLocator("iframe");
  await expect(frame.locator("pre")).toContainText("<script>テキスト</script>");

  await openFromList(page, "img");
  const img = page.frameLocator("iframe").locator("img");
  await expect(img).toBeVisible();
  await expect.poll(() => img.evaluate((e: HTMLImageElement) => [e.naturalWidth, e.naturalHeight])).toEqual([120, 60]);

  // 枠の要素の sandbox は報告の面の CSP と同じ組
  const sandbox = await page.locator("iframe").getAttribute("sandbox");
  expect(sandbox?.split(" ").sort()).toEqual(["allow-popups", "allow-popups-to-escape-sandbox", "allow-scripts"]);
  expect(await unreadTitles(page)).toEqual([]);
});

test("PDF は枠に入れず、報告の面の CSP の sandbox の下で、新しいタブの PDF ビューアに出る", async ({ page, context }) => {
  await put(tray, "pdf", pdf(3), "application/pdf");
  await openFromList(page, "pdf");
  await expect(page.locator("iframe")).toHaveCount(0);
  const link = page.getByRole("link", { name: "PDF を新しいタブで開く" });
  expect(await link.getAttribute("target")).toBe("_blank");

  const downloads: string[] = [];
  context.on("page", (p) => p.on("download", (d) => downloads.push(d.suggestedFilename())));
  const [tab, response] = await Promise.all([
    context.waitForEvent("page"),
    context.waitForEvent("response", (r) => r.url().startsWith(tray.view_origin)),
    link.click(),
  ]);
  // 新しいタブの応答にも報告の面の CSP の sandbox が付く（headers() は CSP を正しく返さないので allHeaders() で読む）
  const headers = await response.allHeaders();
  expect(headers["content-type"]).toBe("application/pdf");
  expect(headers["content-security-policy"]).toContain("sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox");
  expect(headers["content-disposition"]).toBeUndefined();
  // Chromium の PDF ビューア（拡張の枠）が文書を出す。ダウンロードにはならない
  await expect
    .poll(() => tab.frames().map((f) => f.url()), { timeout: 10_000 })
    .toContainEqual(expect.stringMatching(/^chrome-extension:\/\/[a-p]{32}\/index\.html/));
  expect(downloads).toEqual([]);
  expect(await unreadTitles(page)).toEqual([]);
});

test("ほかの形式は枠に入れず、ダウンロードの口から中身がそのまま落ちる", async ({ page }) => {
  const body = "a,b\n1,2\n";
  await put(tray, "csv", body, "text/csv");
  await openFromList(page, "csv");
  await expect(page.locator("iframe")).toHaveCount(0);
  await expect(page.getByText("text/csv")).toBeVisible();
  const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("link", { name: "ダウンロードする" }).click()]);
  const path = await download.path();
  const { readFileSync } = await import("node:fs");
  expect(readFileSync(path, "utf8")).toBe(body);
  // 画面はそのまま（報告の面へ移らない）
  await expect(page).toHaveURL(/\/r\/[0-9a-f]{32}$/);
  expect(await unreadTitles(page)).toEqual([]);
});
