-- tray の D1 の表（ADR-0064）。ローカルの tray は起動のたびに当て、Cloudflare の tray はデプロイが当てる。

-- 報告。セルとプロジェクトと名前の組で1行（決めたこと 9）。id は乱数で、R2 のキーにそのまま使う（決めたこと 18）。
-- version が 0 の行は、アップロードの途中で id だけを取った行で、一覧に出さない。
CREATE TABLE reports (
  id TEXT PRIMARY KEY,
  cell TEXT NOT NULL,
  project TEXT NOT NULL,
  name TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  sha256 TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 0,
  read_version INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL,
  UNIQUE (cell, project, name)
);
CREATE INDEX reports_updated_at ON reports (updated_at);

-- 上がった記録と開いた記録。1行ずつ足して残す（決めたこと 20）。時刻はミリ秒。
CREATE TABLE uploads (
  report_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  at INTEGER NOT NULL
);
CREATE TABLE opens (
  report_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  at INTEGER NOT NULL
);

-- セルの API キー。サーバは鍵の SHA-256 だけを持つ（決めたこと 19）。
CREATE TABLE keys (
  cell TEXT PRIMARY KEY,
  hash TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL,
  revoked_at INTEGER
);
