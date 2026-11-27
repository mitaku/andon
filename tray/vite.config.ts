// 一覧の面の画面のビルド（ADR-0064 決めたこと 25）。web/ の画面を dist/public に作る。public/ の物（_headers・/pwa/ の
// manifest とアイコン）はそのまま写る。Worker の束は package.json の build が bun build で dist/workers に作る。
// ローカルの tray（scripts/serve.mjs）と Cloudflare の一覧の面は、dist/public を画面のファイルとして配る。
import solid from "@solidjs/vite-plugin";
import stylex from "@stylexjs/unplugin";
import { defineConfig } from "vite";

export default defineConfig({
  root: "web",
  publicDir: "../public",
  plugins: [stylex.vite(), solid()],
  // UI SDK の手引き: @ui-sdk/core を先に束ねると、部品が別の写しを読んで dev で落ちる
  optimizeDeps: { exclude: ["@ui-sdk/core"] },
  build: {
    outDir: "../dist/public",
    emptyOutDir: true,
    // 画面のファイルにデータを入れない（決めたこと 17）。ソースマップも出さない
    sourcemap: false,
  },
});
