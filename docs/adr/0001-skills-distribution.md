# ADR-0001: skill の配布（仕様・設計。2026-11-05。Chief Engineer のタスク Y-84）

- 状態: **決定（2026-11-05）**。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: Andon の skill（`package/skills/`。chief-of-staff・worker・chief-engineer・independent-review・ask-one-by-one・triage・glossary）を、建屋の中と、public にしたあとの外（他の人の手元）にどう届けるか。
- 語は [design.md](../design.md) §0.1。機械が読む形は変えない（skill は AI が読む Markdown。[contract.md](../contract.md) §0 原則1）。

## 問題（人の言葉で。言い直しの段落）

Andon の skill 7 本は AI が読む Markdown で、中で `deck`・`andon task …`・`andon gate`・schema の検査器を打つ。だから skill と `andon` CLI と `schema/` の版が揃っていないと壊れる。建屋の中では `package/` が image に `/opt/andon` として焼かれ、`andon boot` が `~/.claude/skills/andon-<name>` の symlink を張り、プロファイルの最初の3行が `/opt/andon/skills/<name>/SKILL.md` を絶対パスで指す。つまり建屋の中では版の一致は「同じ image に入っている」ことで構造的に取れている。困りごとは建屋の外——作業側を別のマシンに出すとき、ホスト側、public にしたあとに他の人の手元——にどの経路で届けるか（人が同意 2026-11-05）。

## 前提（同意済み。番号）

人が 2026-11-05 に 1〜7 すべてに同意。

1. 軸は「版が揃うこと」と「届く範囲」で、「どのプロトコルで配るか」ではない。プロトコルは軸を満たす手段として後で選ぶ。
2. 建屋の中は今の形（image に焼く＋boot が張る＝同じ単位で運ぶ）で足りている。足りないのは小さい継ぎ目（置き場・Codex・stale の検査）で、新しい運び方ではない。
3. Andon の skill は Andon 専用（`deck`・`andon`・schema を打つ）で、建屋の無い所で単体で使えることは目標にしない。
4. 「外」は (a) 作業側の別マシン、(b) ホスト（`andon init`・`bootstrap` を打つ所）、(c) public 後に自分の建屋を立てる他人、の3つ。どれも `andon`（image か binary）が届く場所で、`andon` 無しの読み手は範囲の外。
5. 今日の host（Claude Code・Codex）に無い口（Skills over MCP）に依る設計は採らず、再開の条件を書いて置く。
6. 運ぶ口は1つ。ファイルと MCP の二重は採らない。
7. skill の配り方は Andon 自身の配り方（image と binary の release）に乗せ、別の経路（npm・marketplace・MCP）を Andon のために増やさない。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-05 範囲は「選んで広げる」。Andon が届く所に届く、を前提にする。** 建屋の中の継ぎ目（2〜4）を固め、外は「`andon`（image）が届く所には skill も届く」を設計書に書くだけ。public の読み手向けの案内（`npx skills add` の直接パス等）は書かない。
   捨てた案: 保つ（Codex の発見と注入の継ぎ目が残る）／広げる（MCP・marketplace の口。前提 5〜7）／減らす（文書だけ）。
2. **2026-11-05 Claude Code 系は `/opt/andon` を plugin として `--plugin-dir` で読ませ、Codex 系は利用者ローカルに名前空間つきの symlink を張る。** `package/.claude-plugin/plugin.json`（`{"name": "andon"}`）を1枚足す（`/opt/andon/skills/<name>/SKILL.md` は plugin の配置そのもの）。Claude Code 系のプロファイルの hc の template の `command` に `--plugin-dir /opt/andon` を足し、skill は `/andon:worker` の形になる（`Skill(andon:worker)` の規則・`skills:` の事前読み込みも同じ名）。今の `~/.claude/skills/andon-<name>` の symlink は**やめる**（二重に読まれて `/andon-worker` と `/andon:worker` が並ぶのを避ける）。Codex・Pi など `.agents/skills` を読むハーネス向けには `andon boot` が `~/.agents/skills/andon-<name>` → `/opt/andon/skills/<name>` の symlink を張る（名前空間は無いので接頭字で代える）。
   理由: `andon:xxx` の呼び出し方が欲しい（人 2026-11-05）。plugin の名前空間は plugin として読ませたときだけ付く。`--plugin-dir` はその場で読み（コピー無し・状態ファイル無し）、`--append-system-prompt-file` と同じ「command が運ぶ」形で、boot の `ANDON_HOME` 置換もそのまま効く。実体は root 所有の `/opt/andon` のまま。
   捨てた案: managed settings に `directory` 型の marketplace を登録して強制（人が手で打つ `claude` にも効くが、`~/.claude/plugins` の登録と「not cached」の落とし穴が増える。必要が出たら）／`~/.claude/skills/andon/` を skills-directory plugin にする（plugin の中身は root の外へ出る symlink を捨てる規則があり、`/opt/andon` を指す形が通るか要実測）／admin の置き場（`/etc/claude-code/.claude/skills`・`/etc/codex/skills`。managed の方針で利用者・project の skill を止めるときに要る）／利用者ローカルの `andon-` 接頭字の symlink のまま（名前空間が付かない）。
   副作用（承知の上）: `/opt/andon/hooks/hooks.json`・`agents/`・`commands/` を置けば plugin の部品として読まれる（今は空。将来 P6b のフックをここに乗せる選択肢になる）。
3. **2026-11-05 Claude Code 系のプロファイルは `--append-system-prompt-file` で役の skill を system prompt に足す。** hc の template の `command` に `--append-system-prompt-file /opt/andon/skills/<役>/SKILL.md` を加え、本文の「先に `/opt/andon/skills/<役>/SKILL.md` を読む」の行を消す（最初のメッセージはタスクの2行だけ）。glossary など役の skill が名前で指す skill は、2 の plugin（Claude Code）と置き場（Codex）から発見させる（注入しない）。Codex 系のプロファイル（`codex`・`codex-review`）は今の形（最初のメッセージで絶対パスを指す）のまま。
   理由: skill が最初の turn の前にコンテキストに入り、読み飛ばす余地が無い。既定の system prompt を保つ（`--append-system-prompt-file` は末尾に足すだけ）。版は session の最初に固定される（`--system-prompt-snapshot` の既定）。
   捨てた案: `claude --agent <役>`（agent 定義の本文で system prompt が置き換わる危険。空の本文なら保てるが、プロファイルが hc の template と agent 定義の2ファイルになる。人が退けた 2026-11-05）／注入せず発見だけに任せる（description で選ばせる。読み飛ばしの余地が残る）／今のまま最初のメッセージで読ませる。
4. **2026-11-05 作業側の別マシンは Andon の image で立てる**（同じ `bootstrap`。design.md §1.5）。だから skill を `andon` の binary に go:embed しない。
   捨てた案: binary だけのマシンに `andon skill path <name>` で出す（写しが2つになる。再開の条件: binary だけで担当を動かすマシンが要るとき）。
5. **2026-11-05 PR 時の stale 検査（skill が打つ command が `andon help` にあるか）は入れない。** 版の照合は、将来 `andon task start --on <machine>` を建てるときに `andon version` の比較として添える。**開発中（`ANDON_HOME` が checkout を指すとき）は skill と CLI の版の違いは異常ではない**ので、照合には回避の口を持つ（`ANDON_HOME` が `/opt/andon` でなければ照合しない、等。決めるのはそのタスク）。
   理由: schema の検査（`tests/package.sh` が skill の塊を schema に通す）で約束の側は押さえてある。command の名前の検査は小さいが、開発中に邪魔になる形を先に入れない。

## やらないこと（範囲の外。理由）

- **Skills over MCP**（SEP-2640。クーリエが localhost の HTTP で出す案も）: Claude Code・Codex が未対応（client matrix 2026-11-05）。承認が manifest の digest に結び付くので、andon を上げるたびに再承認になる。運ぶ口が2つになる（前提 6）。再開の条件: Claude Code か Codex が対応を公表し、かつファイルを読めない host（クラウドのセッション等）で担当を動かす必要が出たとき。
- **独自エージェント**（Vercel AI SDK 等で建てる）: ハーネスは Claude Code と Codex（design.md §2.7）。既製のハーネスを担当に足す（Pi など）のは別の題で、この文書の置き場（`.agents/skills`）はそれにも効く。
- **marketplace での配布**（public の読み手向けの plugin 化）: skill は建屋が無いと動かない（前提 3）。plugin の形（決定 2）は建屋の中で `--plugin-dir` で読むためだけ。再開の条件: skill を建屋の外でも動く形に割る題が立ったとき。
- **TanStack Intent／npm 同梱**: Andon は npm の package ではない。「使う対象と同じ単位で運ぶ」は image で取れている。
- **public の読み手向けの配布の案内**: Andon が届く所に届けばよい（決定 1）。Vercel の `skills` CLI は直接パスで取れるが、Andon は何もしない。
- **admin の置き場への移動・managed の marketplace・skill を binary に埋める**: 決定 2・4 の捨てた案。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は3つ。既存のテストに足す（新しい継ぎ目を作らない）。

1. **`andon boot`（偽の HOME。`tests/andon-boot.sh`・`internal/boot/boot_test.go`）**: 2回打って同じ結果で、`~/.agents/skills/andon-<name>` が `<pkg>/skills/<name>` を指す symlink。パッケージから消えた skill の `andon-*` リンクは外れ、`andon-` で始まらないもの（利用者の skill）には触らない（2026-11-11 改め、[ADR-0018](0018-screen-check-tooling.md) 決めたこと 5: boot が張った、イメージの備品の置き場 `/opt/andon-tools` の下を指す symlink〈同梱の skill。`playwright-cli`〉も boot のもので、張り替え・外す。`andon-` で始まらず `/opt/andon-tools` の下も指さないものには触らない）。`~/.claude/skills/andon-*` の古いリンクは外す（仮設の形からの移行）。`.agents/skills` が symlink（仮設の形）なら今の `.claude/skills` と同じく止める。
2. **`package/`（`tests/package.sh`）**: `package/.claude-plugin/plugin.json` があり `name` が `andon`。Claude Code 系の template（`chief-of-staff`・`chief-engineer`・`claude`・`sonnet`・`fable`・`claude-review`）の `command` が `--plugin-dir /opt/andon` と `--append-system-prompt-file /opt/andon/skills/<役>/SKILL.md` を持ち、その skill が `package/skills/` にある。本文に「先に … を読む」の行が無い。Codex 系（`codex`・`codex-review`）は本文で `/opt/andon/skills/<役>/SKILL.md` を指す（今のまま）。`andon chief-engineer` の最初のメッセージ（`internal/chiefengineer`）も「先に読む」の行を落とす（`chiefengineer_test.go`）。
3. **建屋で実測（P4 以降の決まり。報告に書く）**: `--plugin-dir` と `--append-system-prompt-file` で起動した担当が、`/andon:worker` の名で skill を持ち、最初の turn で役の skill の手順（`deck start`）を打つ。`ANDON_HOME=<checkout>/package` で起動したとき checkout の plugin と skill が入る（boot が template を生成するとき、本文の `/opt/andon` を `ANDON_HOME` に置き換える。これが決定 5 の「開発中の回避の口」の実体）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（design.md §1.8・§2.2・§2.3 と decisions.md・README・chief-engineer skill から指す） | `bash tests/*.sh` green | — |
| S2 | boot: `~/.agents/skills/andon-<name>` の symlink（`~/.claude/skills/andon-*` は外す）＋ template 生成時の `/opt/andon` → `ANDON_HOME` の置換 ＋ tests | 受け入れ条件 1 と 3 の置換 | S1 のマージ |
| S3 | package: `.claude-plugin/plugin.json`・Claude Code 系 template に `--plugin-dir` と `--append-system-prompt-file`・「先に読む」の行を消す・`andon chief-engineer` の最初のメッセージ・tests | 受け入れ条件 2 | S1 のマージ（S2 と並行可） |
| S4 | 建屋で実測（担当1つを起動して最初の turn を見る） | 受け入れ条件 3。落ちたら直しのタスク | S2・S3・建屋（P7） |
| — | `andon version` と `--on <machine>` での照合（回避の口つき） | `machine` を建てるタスクに添える（決定 5） | `machine` |

## 開いている問い（残った問い。判断の種類つき）

- 担当のハーネスに **Pi** を足すか（人が「今後検討」と言った 2026-11-05。ラインの変更。別の題）。この文書の置き場 `~/.agents/skills` と、最初のメッセージで絶対パスを指す Codex の形がそのまま使える見込み。
- admin の置き場や managed の marketplace に移すか（ラインの変更。managed の方針で利用者・project の skill を止めるとき、または人が手で打つ `claude` にも Andon の skill を効かせたいときに戻す）。

## 出典（読んだもの・捨てた案の出所）

- Andon の側: [design.md](../design.md) §1.5・§1.8・§1.10・§2.1〜§2.7・§5・§6、[dispatcher.md](../dispatcher.md)、[contract.md](../contract.md) §8・§9、[decisions.md](../decisions.md)、`package/skills/*/SKILL.md`、`package/templates/*.md`、`internal/boot/boot.go`（skill の symlink）、`internal/chiefengineer/chiefengineer.go`（最初のメッセージの絶対パス）、`image/Dockerfile`（`package/` → `/opt/andon`）、`tests/package.sh`。
- MCP Skills extension（SEP-2640 Final。`io.modelcontextprotocol/skills`）: 概要 https://modelcontextprotocol.io/extensions/skills/overview 、仕様 https://github.com/modelcontextprotocol/ext-skills （`specification/beta/skills.mdx`）、client matrix https://modelcontextprotocol.io/extensions/client-matrix 、実装の一覧 `docs/implementations.md`（同 repo）。2026-11-05 に読んだ。
- Agent Skills の仕様: https://agentskills.io/specification
- Claude Code: skills（置き場と優先。enterprise＝managed settings のディレクトリの `.claude/skills/`） https://code.claude.com/docs/en/skills 、plugins https://code.claude.com/docs/en/plugins 、plugin の読み込み https://code.claude.com/docs/en/plugins/loading 、組織の管理（`extraKnownMarketplaces`・`enabledPlugins`・`strictPluginOnlyCustomization`・seed） https://code.claude.com/docs/en/plugins/org 、marketplace のホスト https://code.claude.com/docs/en/plugins/host-marketplace 、managed settings https://code.claude.com/docs/en/managed-settings 、settings https://code.claude.com/docs/en/settings
- Codex: skills（置き場 `.agents/skills`・`~/.agents/skills`・`/etc/codex/skills`。配布は plugin） https://developers.openai.com/codex/skills
- Claude Code の plugin（`--plugin-dir` でその場で読む・`.claude-plugin/plugin.json`・`/<plugin>:<skill>` の名前空間・root の外へ出る symlink は捨てる） https://code.claude.com/docs/en/plugins/manifest-reference 、https://code.claude.com/docs/en/plugins/components
- Claude Code の `--agent`（主セッションを agent 定義で起動する。managed settings のディレクトリの `.claude/agents/` が最優先。空の prompt なら既定の system prompt を保つ〈v2.1.281 以降〉。`skills` で skill の全文を起動時に注入） https://code.claude.com/docs/en/sub-agents 、CLI https://code.claude.com/docs/en/cli-reference
- Vercel の `skills` CLI（`npx skills add <owner>/<repo>` で GitHub の repo から 79 のエージェントの置き場へ写す。見つけるのは repo の `skills/`・`.claude/skills/`・`.agents/skills/` などで、`package/skills/` は直接パスを指すときだけ） https://github.com/vercel-labs/skills （README・docs/creating-skills.md・docs/commands.md・docs/agents.md）
- TanStack Intent（npm の package に skill を同梱し、`npx @tanstack/intent list|load` で読む。版は package と一緒に上がる） https://tanstack.com/table/v9/docs/agent-skills
