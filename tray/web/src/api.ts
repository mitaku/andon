// 一覧の面の API（src/lib/list-api.ts）を呼ぶ口。同じオリジンだけを呼ぶ。書き込みは POST で、ブラウザが Origin を付ける。
import type { OpenAs } from "../../src/lib/formats";

export type View = "unread" | "recent" | "old";

export interface Report {
  id: string;
  title: string;
  cell: string;
  project: string;
  name: string;
  kind: string;
  content_type: string;
  version: number;
  unread: boolean;
  updated_at: string;
}

export interface Opened {
  url: string;
  version: number;
  content_type: string;
  title: string;
  open_as: OpenAs;
}

export interface Cell {
  cell: string;
  created_at: string | null;
  key: "active" | "revoked" | "none";
  uploads: number;
  opens: number;
}

export class APIError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { Accept: "application/json" }, credentials: "same-origin" });
  let body: { message?: string } & Record<string, unknown> = {};
  try {
    body = await res.json();
  } catch {
    // Access のログインの画面などが返ったとき
  }
  if (!res.ok) throw new APIError(res.status, body.message ?? `${res.status}`);
  return body as T;
}

export const listReports = (view: View) =>
  call<{ view: View; reports: Report[] }>(`/api/reports?view=${view}`).then((r) => r.reports);

export const openReport = (id: string) => call<Opened>(`/api/reports/${encodeURIComponent(id)}/open`, { method: "POST" });

export const listCells = () => call<{ days: number; cells: Cell[] }>("/api/cells");

export const issueKey = (cell: string) =>
  call<{ cell: string; key: string }>(`/api/cells/${encodeURIComponent(cell)}/key`, { method: "POST" });

// 有効な鍵があると 409 で断る（前の鍵を効かなくするのは reissueKey だけ）
export const reissueKey = (cell: string) =>
  call<{ cell: string; key: string }>(`/api/cells/${encodeURIComponent(cell)}/reissue`, { method: "POST" });

export const revokeKey = (cell: string) =>
  call<{ cell: string; revoked: true }>(`/api/cells/${encodeURIComponent(cell)}/revoke`, { method: "POST" });
