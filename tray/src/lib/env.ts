// 3つの面が持つ束縛（ADR-0064 決めたこと 16）。面ごとの Worker は、ここから要るものだけを持つ。
// 一覧の面: DB・SIGNING_KEY・VIEW_ORIGIN・ACCESS_TEAM_DOMAIN・ACCESS_AUD（・画面のファイル）
// 報告の面: BUCKET・SIGNING_KEY・LIST_ORIGIN
// アップロードの口: DB・BUCKET・LIST_ORIGIN

export interface ListEnv {
  DB: D1Database;
  SIGNING_KEY: string;
  VIEW_ORIGIN: string;
}

export interface AccessEnv {
  // Access のチームのドメイン（https://<チーム>.cloudflareaccess.com）。JWT の iss と公開鍵の置き場になる
  ACCESS_TEAM_DOMAIN?: string;
  // Access のアプリの Application Audience（AUD）タグ
  ACCESS_AUD?: string;
}

export interface ViewEnv {
  BUCKET: R2Bucket;
  SIGNING_KEY: string;
  LIST_ORIGIN: string;
}

export interface UploadEnv {
  DB: D1Database;
  BUCKET: R2Bucket;
  LIST_ORIGIN: string;
}
