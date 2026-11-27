// パスの段の読み方（ADR-0064 決めたこと 9・15・17）。パスは URL で1度だけ読み、パーセントの符号化は段ごとに1度だけ解く。
// 解いたあとの値でルートを比べない（%2F で段をまたがせない）。

const MAX_BYTES = 200;

// segments: pathname（符号化されたまま）を "/" で割り、先頭の空の段を落とす。段の符号化は解かない
export function segments(pathname: string): string[] {
  const parts = pathname.split("/");
  if (parts[0] === "") parts.shift();
  return parts;
}

// decodeName: 1つの段を解き、名前として受けられるかを見る。受けられなければ null。
// 断るのは、解けない符号化・空・"/"・制御文字・UTF-8 で 200 バイトを超えるもの。
export function decodeName(segment: string): string | null {
  let s: string;
  try {
    s = decodeURIComponent(segment);
  } catch {
    return null;
  }
  if (s === "" || s.includes("/")) return null;
  // 制御文字（C0・DEL・C1）
  if (/[\u0000-\u001f\u007f-\u009f]/.test(s)) return null;
  if (new TextEncoder().encode(s).length > MAX_BYTES) return null;
  return s;
}

// decodeHeader: ヘッダで渡す UTF-8 の値（タイトル）を解く。解けない・制御文字があれば null
export function decodeHeader(value: string | null): string | null {
  if (value === null) return null;
  let s: string;
  try {
    s = decodeURIComponent(value);
  } catch {
    return null;
  }
  if (/[\u0000-\u001f\u007f-\u009f]/.test(s)) return null;
  return s;
}

export const REPORT_ID = /^[0-9a-f]{32}$/;

// 中身の sha256（16進の64字）
export const SHA256 = /^[0-9a-f]{64}$/;

// objectKey: R2 のキー。報告の id と中身の sha256 の組で、中身ごとに別のキーにする。報告の面の署名もこの組に結び付ける
export function objectKey(id: string, sha256: string): string {
  return `${id}/${sha256}`;
}
