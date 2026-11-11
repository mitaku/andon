# ADR-0018: 担当が画面を見て確かめる道具は、焼いてある Playwright の `playwright cli` を配線し、証拠を PR に添付してゲートが見る

- 日付: 2026-11-11
- 出典: ADR-0018（[adr/0018-screen-check-tooling.md](../adr/0018-screen-check-tooling.md)。Chief Engineer の課題 A-0108。オーナーが前提 4 つと決定 6 つを 1 問ずつ確かめて「決めた」）

担当が自分の変更を画面で確かめる道具は、イメージに焼いてある Playwright 1.62.1 に同梱の `playwright cli` だけ（新しい備品は足さない。Vercel の agent-browser は再開の条件つきでやらない）。足すのは配線: `playwright-cli` の wrapper と PATH、Chromium を既定にする設定と出力の置き場（`~/.playwright/cli.config.json`）、同梱の skill を boot が `$CLAUDE_CONFIG_DIR/skills/playwright-cli` と `~/.agents/skills/playwright-cli` に symlink で張る（版はイメージと構造で揃う）。外の既製の skill は boot が `gh skill install`（latest）と `gh skill update --all` で入れる口を持つが、既定の一覧は空。証拠はスクリーンショットを `gh pr create`／`comment --attach` で PR に添付し、`report.json` の `acceptance[].evidence` に添付の URL と撮った command を書く。「画面に触れる変更には証拠が要る」はゲートが判定する: 条件表に `screen_paths`（glob。既定は空。案件の `gate.local.json` で指す）を足し、当たる差分の PR に画像の添付が無ければ `fix`。レビュアーの依頼書の型にも観点を 1 行足す。同梱の skill の `allowed-tools`（`npx:*`）が許可の確認を飛ばす穴は、managed settings に `allowManagedPermissionRulesOnly: true` を足して塞ぐ。製品 repo の E2E（Playwright Test）の決まりは別の題。切る案 S1〜S6 は ADR が正。
