// 報告の開き方（ADR-0064 決めたこと 18・22）。報告の面が返すヘッダと、一覧の面の画面が報告をどう開くかを、この1つの表で決める。
//   frame     一覧の面の枠（iframe）の中に出す（HTML・Markdown・テキスト・画像）
//   tab       署名付き URL を新しいタブで開く（PDF。iOS は枠の中の PDF を1ページしか出さない）
//   download  Content-Disposition: attachment で返し、画面はダウンロードの口を出す（ほかの形式）
// 形式はアップロードの Content-Type で決める（拡張子から決めるのはセルの側）。

export type OpenAs = "frame" | "tab" | "download";

const OPEN_AS: Record<string, OpenAs> = {
  "text/html": "frame",
  "text/markdown": "frame",
  "text/plain": "frame",
  "image/png": "frame",
  "image/jpeg": "frame",
  "image/gif": "frame",
  "image/webp": "frame",
  "image/svg+xml": "frame",
  "application/pdf": "tab",
};

// baseType: "text/html; charset=utf-8" を "text/html" にする
export function baseType(contentType: string): string {
  return (contentType.split(";")[0] ?? "").trim().toLowerCase();
}

export function openAs(contentType: string): OpenAs {
  return OPEN_AS[baseType(contentType)] ?? "download";
}
