// ブラウザの検査（ADR-0064 決めたこと 28・受け入れ条件 13）。ビルドしたローカルの tray を一時の置き場で起動して、Chromium で開く。
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "browser",
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  reporter: [["list"]],
  use: { ...devices["Desktop Chrome"], trace: "off" },
});
