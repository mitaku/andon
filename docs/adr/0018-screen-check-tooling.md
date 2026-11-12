# ADR-0018: 担当が画面を見て確かめる道具（仕様・設計。2026-11-11。Chief Engineer の課題 A-0108）

- 状態: **決定（2026-11-11）**。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 担当が自分の変更を画面（ブラウザ）で確かめる道具と決まりを、セルの標準にするか。するならどの道具を・どう届け・証拠をどこに残すか。
- 語は [design.md](../design.md) §0.1。

## 問題（人の言葉で。言い直しの段落）

Andon の担当は Web の画面を持つ案件でも、今はテストとレビューと検査だけで「確認」を終えており、自分の変更が画面でどう見えるかを見る手段も、見ろという決まりも無い。Playwright は備品としてイメージに焼いてあるが PATH にも skill にも無く、担当は存在を知らない。困りごとは「画面の変更を担当が自分の目で確かめず、人が手元のブラウザで見て初めて壊れていると分かる」こと。決めたいのは、(1) 担当が画面を見て確かめることを Andon の確認の工程の標準にするか、(2) するならどの道具（素の Playwright／既製のエージェント向け CLI と skill／MCP）を、(3) どう届けるか（イメージの備品・PATH・skill・environment.md の行）、(4) 証拠（スクリーンショット）を報告と PR のどこに残すか（人が同意 2026-11-11）。

人の期待（2026-11-11）: 検証のゲートを手厚くする。PR に証憑を正しく載せる（画像を添付する）。Andon 自身の開発には E2E は要らないが、今後 Web サービスを作ることがある。

## 前提（同意済み。番号）

1. **2026-11-11** 困りごとの芯は「担当が画面を見る道具が無い」ことではなく、「画面で確かめた証拠が PR と報告に無いので、ゲート（検証）と人が画面の壊れを拾えない」こと。だから道具の選定より先に、確認の工程（worker skill）と証拠の置き場（PR の添付・`report.json` の `acceptance[].evidence`）を決める。道具は証拠を作る手段で、それを満たす最小のものを選ぶ。
2. **2026-11-11** Andon 自身には今、Web の画面を持つ課題が無い（ボードは別プロジェクトの持ち物）が、**Web の案件を待たずに今全部入れる**: 決まり（skill と証拠の形）に加え、操作の道具（クリック・入力・遷移ができる既製の CLI）もイメージに焼いて skill を配る。人の決定（推奨は「決まりと最小の道具だけ今、操作の道具は最初の Web の案件で実測」だったが、人が「今全部」を選んだ）。
3. **2026-11-11** 「撮って PR に貼る」は今あるもので足りる: イメージの Playwright 1.62.1（`playwright screenshot`。セルで実測済み、1枚 6.5 秒）、セルの gh 2.103.3 の `--attach`（`gh pr create`／`edit`／`comment --attach <画像>`。公式の機能。v2.99.0 以降。PNG・JPEG・GIF・WebP・SVG と MP4・MOV・WebM、画像は 10 MB まで、1 command に 50 件まで、repo への push 権限が要る）、`report.json` の `acceptance[].evidence`。足りないのは (a) 操作（クリック・入力・遷移）、(b) 担当にそれを知らせる skill と決まり、(c) PATH（`playwright` の実体は `/opt/andon-tools/bin` で PATH に無い）。
4. **2026-11-11** 配り方: 道具（CLI とブラウザ）はイメージの備品として焼く（`image/Dockerfile`。版と checksum を固定）。外の既製の skill は、セルの gh（2.103.3）にある公式の `gh skill install <owner>/<repo> <skill> --agent claude-code --agent codex --scope user` を boot が打って入れる（preview の機能。gh のログインとネットワークが要る。セルで実測: tag に固定でき、frontmatter に出所と tag と tree の sha が残り、`gh skill update` が無印のものを上げ、`--pin` のものは飛ばす）。入れる skill の一覧はパッケージの既定（セルの標準）を持ち、セルの local で足せる。**版は latest で入れ、boot が毎起動 `gh skill update --all` を打つ。ただし CLI と同じ repo で版が揃う skill（playwright-cli）だけは、イメージに焼いた CLI の版と同じ tag に `--pin` し、Andon の版上げで一緒に上げる**（人の案「gh skill でセルの既定として管理」「latest で入れて update していきたい」を取り込んだ）。同梱の playwright-cli の skill は、反論のあと symlink で張ることにした（決めたこと 5）ので、この `--pin` の規則が効くのは、外の skill で CLI と同じ repo・同じ版のものを入れるときだけ。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-11 操作の道具は、イメージに焼いてある Playwright 1.62.1 に同梱の `playwright cli` だけ。新しい備品は足さない。** 足すのは配線: PATH（`playwright`・`playwright-cli` の入口）、Chromium を既定にする設定（同梱の既定は Google Chrome を探して落ちる）、出力（`.playwright-cli/`）の置き場、同梱の skill（`playwright-core/lib/tools/skills/playwright-cli/`）を boot が claude-code と codex の置き場に入れる（入れ方は決めたこと 5）。
   理由: 前提1（証拠を作る最小の道具）。備品・ブラウザ・版が1つで済み、E2E（Playwright Test）と同じ engine・同じ版で見て、手で確かめた操作を spec に昇格できる（同梱の skill に `test-generation.md`・`playwright-tests.md`）。セルで実測: `playwright cli open --browser=chromium http://127.0.0.1:…` → `snapshot` → `screenshot` → `close` が headless で通った。
   捨てた案: Vercel の agent-browser を焼く（Rust の単体 binary。Chrome for Testing をもう1つ焼く・aqua の自前 registry・版の追従が2本・E2E の spec は書けない。強みの Electron・Slack・cloud browser の特化 skill は Andon の用途に無い）／両方焼く（使い分けの決まりが要る）／Playwright を外して agent-browser に替える（製品の E2E の土台をイメージから失う。`~/.cache` は volume に無いので落とし直しがセルの作り直しごとに起きる）／Playwright MCP（公式が「コーディングエージェントには CLI＋skill」と書く。ADR-0001 の「運ぶ口は1つ」）。
2. **2026-11-11 「画面に触れる変更には画面の証拠が要る」はゲートが判定して止める。** 条件表（`package/gate.json`・セルの `gate.local.json`）に `screen_paths`（glob の配列。既定は空）を足し、当たる差分の PR に画像の添付が無ければ `fix`（担当へ戻す。自働化。灯りはつかない）。案件の gate.local.json で画面のファイル（例 `web/**`・`**/*.css`・`**/*.tsx`）を指す。Andon の repo は既定の空のまま（画面を持たない）。依頼書の完了条件（Chief of Staff が「画面の証拠」と書く）と担当の判断（書いてなくても画面に触れたら撮る）も併せて持つ。
   理由: 人の期待「検証のゲートを手厚く」。センサーで止める（design.md §1.2）。条件表の既存の形（パスの glob）に1キー足すだけで、約束の版は上げない（contract.md §0 原則4。任意キー）。
   捨てた案: ゲートを変えず依頼書と担当の判断だけ（「通した」が言葉だけになる危険が残る。A-0069 と同じ題）／レビュアーに持たせる（Review: on の課題だけに効く）。
3. **2026-11-11 証拠の形: スクリーンショットを `gh pr create`／`comment --attach` で PR に添付し、`report.json` の `acceptance[].evidence` にその添付の URL と撮った command（`playwright-cli` の列）を書く。** 変更後を必ず、前後の対は見た目の直しのとき。リファクタやバックエンドだけの変更には要らない（添付は読み手の checkout を省くためのもの）。
   理由: 人とレビュアーは PR で見え、ゲートと振り返りは `report.json` で追える。添付は repo の権限で守られ、worktree を片付けても残る。
   捨てた案: 画像を repo に commit（履歴が重くなる）／PR の添付だけ（ゲートと振り返りが追えない）／セルの `files/`（`andon publish`。セルの外から見えない）。
4. **2026-11-11 範囲は「選んで広げる」: レビュアーの観点を1行足す。** independent-review の依頼書の型に「画面に触れる差分（`screen_paths`）に証拠が無い・証拠と差分が合わない」を指摘する観点を足す（重さは major）。製品 repo で Playwright Test（E2E）を置くときの決まり（版・ブラウザの置き場・CI）は別の題（最初の Web の案件で立てる）。
   捨てた案: 広げる（E2E の決まりも決める。製品が無い今は事実が足りない）／保つ／減らす（ゲートの規則を外す）。
5. **2026-11-11 同梱の skill は boot が symlink で張る**（`~/.claude/skills/playwright-cli`〈`CLAUDE_CONFIG_DIR/skills`〉と `~/.agents/skills/playwright-cli` → イメージの実体 `playwright-core/lib/tools/skills/playwright-cli`）。Codex 向けの `andon-*` と同じ形。版はイメージと構造で揃い、Playwright を上げれば一緒に上がる。boot の「`andon-` で始まらないものに触らない」規則は「自分が張った symlink（`/opt/andon-tools` の下を指すもの）も自分のもの」と広げ、ADR-0001 の受け入れ条件 1 の文を直す（覆しではなく改め）。外の skill 用の `gh skill install`／`update --all` の口（前提 4）は残すが、既定の一覧は空。
   理由（反論 1・2。セルで実測）: `gh skill install --from-local` は `--pin` と両立せず（`--from-local and --pin cannot be used together`）、写しの frontmatter は `metadata.local-path` だけで版が無く、`gh skill update --all` が毎回「no GitHub metadata … skipped」で飛ばす。写しの置き場 `~/.claude`・`~/.agents` は volume なので、イメージの Playwright を上げても古い写しが残る。揃えるには毎起動 `--force` で上書きするしかなく、利用者の同名の skill を黙って潰す。
   捨てた案: `--from-local --force` を毎起動／microsoft/playwright-cli の repo から tag で入れる（repo の版〈v0.1.x〉と同梱の CLI の版〈Playwright 1.62.1〉は別の番号で、揃える根拠が無い）。
6. **2026-11-11 managed settings（`package/settings.json`）に `allowManagedPermissionRulesOnly: true` を足す。** 許可の規則は managed だけになり、skill の `allowed-tools` と user／project／local の `allow` は無視される。
   理由（反論 3）: 同梱の skill の frontmatter は `allowed-tools: Bash(playwright-cli:*) Bash(npx:*) Bash(npm:*)` で、担当がこの skill を読んだ turn の中では `npx <任意の package>` が許可の確認なしで通る（Claude Code の docs: allowed-tools はその turn の間だけ許可、deny と ask は勝つ）。Node の案件で npm／npx は正当に使うので deny にできない。Andon はもともと「常に許可」を選ばず（chief-of-staff skill §4）、許可の確認は Chief of Staff が答える設計なので、managed だけにしても失うものが無い。必要な allow は `package/settings.json` に足す。
   捨てた案: 受け入れる（その turn だけの穴。セルが境界）／managed に `ask` の `Bash(npx:*)`・`Bash(npm:*)`（`npm test` も毎回確認になり Chief of Staff の答える回数が増える）／deny（Node の案件が止まる）。

## やらないこと（範囲の外。理由）

- **agent-browser（Vercel Labs）を焼く**: 決めたこと 1 の捨てた案。再開の条件: Electron・Slack・cloud browser の特化の用途が出たとき、または `playwright cli` で担当が画面を確かめられない案件が実測で出たとき。そのときも Playwright は外さない（E2E の土台）。
- **Playwright MCP**: 運ぶ口を増やさない（ADR-0001 前提 6）。公式も CLI＋skill を推奨。
- **Anthropic の Claude in Chrome・Desktop の Browser pane**: 人の手元の Chrome／Desktop アプリが要り、セル（CLI・display 無し）からは使えない（公式 docs）。
- **製品 repo の E2E（Playwright Test）の決まり**: 版・ブラウザの置き場（`~/.cache` は volume に無い）・CI は別の題。最初の Web の案件で立てる。焼いたブラウザ（chromium-1234）が効くのは製品も Playwright 1.62.x のときだけ、は事実として書いておく。
- **人が手元のブラウザで見る道の変更**: portless の名前と `andon publish` は今のまま。
- **動画（`video-start`）の証拠**: 必須にしない。担当が流れを見せたいときに添付してよい（gh は MP4・MOV・WebM を受ける）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は5つ（高いものから）。既存の継ぎ目に足し、新しい継ぎ目は作らない。証拠はテストの command と出力のパス（staged な出力と mock は証拠でない）。

1. **`tests/image.sh`（image）**: ネットワークを切ったコンテナで `playwright-cli --version` が PATH で引け（`playwright-cli` は `playwright cli "$@"` を呼ぶ wrapper。symlink では引数を足せない。`/opt/andon-tools/bin` は PATH に無いまま）、`playwright-cli open http://127.0.0.1:<port>`（コンテナの中で立てた静的ページ）→ `snapshot` に ref が出る → `screenshot` が PNG を書く → `close`、が設定なしで通る（Chromium が既定。Google Chrome を探さない。既定を変える口は `~/.playwright/cli.config.json`〈`{"browser":{"browserName":"chromium"}}`。cwd に依らず効く。`~/.playwright` は volume に無いので image に焼くか boot が毎起動書く〉。Dockerfile の `ENV` を足すなら `image/sshd_config.d/andon.conf` の `SetEnv` も同じ行に足す〈`tests/sshd-env.sh`〉）。`.playwright-cli/` が担当の worktree の git に `??` で出ない（既定は cwd の `.playwright-cli/`。同じ設定ファイルの `outputDir`〈絶対パス〉で worktree の外に出すのが筋。セルの git に global excludes は無い）。
2. **`internal/boot/boot_test.go`・`tests/andon-boot.sh`（boot。偽の HOME と偽の `gh`）**: boot が (i) 同梱の skill の symlink を `$CLAUDE_CONFIG_DIR/skills/playwright-cli` と `~/.agents/skills/playwright-cli` に張る（決めたこと 5。`/opt/andon-tools` の下を指す symlink は boot のものとして張り替え・外す。実ディレクトリや別の先を指す symlink には触らない。`tests/image.sh` の偽の `gh` は `auth status` 以外を `auth setup-git` に差し替えているので、boot が `gh skill` を打つなら直す）、(ii) パッケージの一覧（既定は空。ファイルの名前と形は担当が決め、セルの `local/` で足せる）の外の skill を `gh skill install <repo> <skill>`（latest）で入れ、(iii) `gh skill update --all` を打つ（一覧が空なら (ii)(iii) は打たない）。gh がログインしていなければ (ii)(iii) は飛ばしてログ1行（`gh auth setup-git` と同じ扱い）。2回打って同じ結果。ADR-0001 の `~/.agents/skills/andon-*` の symlink と `--plugin-dir` の plugin はそのまま。利用者の skill（`andon-` で始まらず、`/opt/andon-tools` の下も指さないもの）には触らない。
3. **`go test ./internal/gate/...`・`tests/schema.sh`（ゲート）**: 条件表に任意キー `screen_paths`（glob の配列。既定 `[]`）。当たるファイルが差分にあり、PR の本文かコメントに GitHub の添付（画像）が無ければ `decision: fix`、`reasons[]` に `screen_paths: <file> に当たるが画面の証拠（添付）が無い`。添付があれば通る。`gate.local.json` のキーの置き換えで効く。`schema/gate.schema.json`・`docs/contract.md` §7 に足す（版は上げない。原則 4）。添付の検出の形（`user-attachments` の URL か、`![…](…)` の画像）は担当が `gh pr view --json body,comments` の実物で決め、テストに固定する。
4. **`tests/package.sh`（skill と文書）**: `package/settings.json` に `allowManagedPermissionRulesOnly: true`（決めたこと 6）。worker skill §2 の確認の工程に「画面に触れる変更は dev サーバを `127.0.0.1` で立て `playwright-cli` で確かめ、スクリーンショットを撮る」、§3 に `gh pr create/comment --attach`、§7 に `acceptance[].evidence` へ添付の URL と command、の段。independent-review の依頼書の型に観点1行（決めたこと 4）。`package/environment.md` の「ブラウザは無い」を「人が見るブラウザは無い（認証の URL は手元で）。担当が画面を見るには `playwright-cli`」に直す。`tests/public.sh` が通る。
5. **セルで実測（S6）**: 静的な画面1枚の試作の課題（レベル A・`review: off`）で担当を1つ起動し、PR に添付が付き、`report.json` の `acceptance[].evidence` に添付の URL と command があり、`screen_paths` を指した `gate.local.json` のもとでゲートが添付の無い PR を `fix` にし、添付のある PR を通す。報告に command と出力のパス。

## 価値の仮説

画面に触れる PR のうち証拠の添付があるものの割合が上がり、マージ後に人が画面の壊れを見つけて直す回数（`corrections`・`reverted`。[contract.md](../contract.md) の measure）が減る。最初の Web の案件で計る（形は P6 で決める。[ADR-0011](0011-signals-and-value-loop.md) 決めたこと 7 の観察の課題）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | レベル・依存 |
|---|---|---|---|
| S1 | この文書の PR（ADR・`docs/decisions/` の1件。README の ADR の地図は A-0073 が揃える） | `tests/public.sh` green | A。— |
| S2 | image: `playwright-cli` の wrapper と PATH、`~/.playwright/cli.config.json`（Chromium の既定と `outputDir`。image に焼くか boot が書く）、`tests/image.sh` | 受け入れ条件 1 | C（image）。S1 のマージ |
| S3 | boot: 同梱の skill の symlink（ADR-0001 受け入れ条件 1 の文の改め）、外の skill の一覧（既定は空）と `gh skill install`／`update --all`、ログインしていないときの飛ばし、tests | 受け入れ条件 2 | B。S1（S2 と並行可） |
| S4 | gate: `screen_paths`、添付の検出、schema・contract.md §7、go test | 受け入れ条件 3 | B。S1（S2・S3 と並行可） |
| S5 | skills・設定・文書: `allowManagedPermissionRulesOnly`、worker §2・§3・§7、independent-review の型、environment.md、`tests/package.sh` | 受け入れ条件 4 | C（package/skills）。S1 |
| S6 | セルで実測（イメージを焼き直してから。A-0106 の v0.0.9 と合わせる） | 受け入れ条件 5。落ちたら直しの課題 | S2〜S5 のマージとイメージの焼き直し |

## 開いている問い（残った問い。決断の種類つき）

- 製品 repo の E2E（Playwright Test）の決まり（版・ブラウザの置き場・CI）。最初の Web の案件で Chief Engineer の題にする（ライン）。
- `gh skill` は preview で「予告なく変わる」。コマンドが変わったら boot が落ちる。gh の版上げ（aqua）のときに `tests/andon-boot.sh` の偽の `gh` を実物のヘルプと照らす（担当の手順。決断ではない）。
- `allowManagedPermissionRulesOnly` で、人が手で打つ `claude`（セルの中）の「常に許可」も効かなくなる。困れば managed の allow に足す（運用。決断ではない）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.2・§1.3・§1.8・§2.7、[ADR-0001](0001-skills-distribution.md)（skill の配り方。前提 6・7）、[ADR-0007](0007-interim-gap-inventory.md) A4／R5（証拠の質。A-0069）、[contract.md](../contract.md) §4（`acceptance[]`）・§7（条件表）、`package/skills/worker/SKILL.md` §2・§3・§7、`package/skills/independent-review/SKILL.md` §4、`package/environment.md`、`image/Dockerfile`（Playwright 1.62.1 を npm のグローバルに。`PLAYWRIGHT_BROWSERS_PATH`。`playwright install --with-deps chromium`）、`package/compose.yaml`（volume。`~/.cache` は無い）、`package/gate.json`、`tests/image.sh`（`PLAYWRIGHT_BROWSERS_PATH` の有無だけ）、`tests/host/screen.mjs`（Playwright を使っていない。Node の VM）。
- 反論（別のコンテキストの読むだけのサブエージェント。2026-11-11。証明できた 6 件: `--from-local` と `--pin` の両立不可・volume の写しが古いまま残る・`allowed-tools` の `npx:*`・`.playwright-cli/` が cwd に出る・`~/.playwright/cli.config.json` が cwd に依らず効くが volume に無い・`--from-local` は gh のログイン不要。壊せなかった: plugin `andon:*` と personal `playwright-cli` の名前の衝突無し・`gh pr view --json body` に添付が `![alt](https://github.com/user-attachments/assets/<id>)` で出る〈cli/cli の attach_test〉・`tests/public.sh` は通る）。
- セルで実測（2026-11-11）: `/opt/andon-tools/bin/playwright screenshot`（6.5 秒）、`playwright cli open --browser=chromium http://127.0.0.1:…` → `snapshot` → `screenshot` → `close`（既定の `--browser` は Google Chrome を探して落ちる。`file:` は既定で拒む）、同梱の skill の置き場 `playwright-core/lib/tools/skills/`、`gh skill install`（tag に固定・frontmatter のメタデータ・`--dir`・`--from-local`）・`gh skill update --all`、`gh pr create --help` の `--attach`。
- GitHub CLI の `--attach`（v2.99.0 から。画像 10 MB・動画 10／100 MB・1 command 50 件・push 権限）: https://github.blog/changelog/2026-09-01-github-cli-media-in-issues-pull-requests-and-comments 、https://docs.github.com/en/github-cli/github-cli/attaching-files-with-github-cli 。REST／GraphQL には添付の API が無い。
- GitHub CLI の `gh skill`（v2.90.0 から。preview）: https://cli.github.com/manual/gh_skill_install
- Playwright の agent CLI（`@playwright/cli` は `playwright-core` の CLI client を呼ぶ薄い package。1.59 以上は `npx playwright cli`）: https://github.com/microsoft/playwright-cli 、https://playwright.dev/agent-cli/installation 、https://playwright.dev/agent-cli/configuration 。Playwright MCP の README（CLI＋skill の推奨）: https://github.com/microsoft/playwright-mcp
- agent-browser（Vercel Labs。v0.42.0。Rust の単体 binary・CDP・コンテナ検出・skill は CLI が返す）: https://github.com/vercel-labs/agent-browser 、https://agent-browser.dev
- Claude Code の browser（Claude in Chrome は同じマシンの Chrome と `/login` が前提。Browser pane は Desktop アプリ専用で SSH では無い）: https://code.claude.com/docs/en/chrome 、https://code.claude.com/docs/en/desktop
- Claude Code の skill の置き場: https://code.claude.com/docs/en/skills 、Agent Skills の仕様: https://agentskills.io/specification
