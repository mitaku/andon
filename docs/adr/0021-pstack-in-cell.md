# ADR-0021: pstack をセルに入れて試す（仕様・設計。2026-11-12。Chief Engineer の課題 A-0160）

- 状態: **決定（2026-11-12）**。人が対話の中で「決めた」と言った。セルに入れるのは「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- **改め（2026-11-12。同じ課題 A-0160。PR #210 のマージのあと）**: plugin の登録と自動更新は、セルの利用者の `settings.json` に書かれた状態を正とする。決めたこと 7 を下から足し、受け入れ条件 3・切る案の細部・開いている問いを合わせた。
- 主題: poteto（Lauren Tan）作の pstack を、Andon を作るセルに plugin として入れ、担当の実装の作法の底上げと、skill の作りの学びに使う。
- 語は [design.md](../design.md) §0.1。パッケージ（`package/`）は変えない。セルの設定の変更だけで、Andon の repo の PR はこの文書のほかに出ない。

## 問題（人の言葉で。言い直しの段落）

Andon 自身を作る仕事（このセルで andon の repo を触る担当）の出来を上げたい。worker skill はラインの回し方（台帳・報告・レビュー・人を待つ）は持つが、良い実装の作法（書く前に仕組みを理解する・差分を最小にする・本物を動かして確かめる・根本原因を直す）は薄い。そこを自前で書き足さず、外で磨かれている pstack を借りる。あわせて、pstack がどう作られているかを、使いながら学ぶ。まずこのセルで試し、パッケージの既定（全セル）に入れるかは結果を見て別に決める（人が同意 2026-11-12）。

人の言葉（2026-11-12）: 「Pstackは有用だと思っているので導入することでAndonの開発が良くなるのではないかと思っている」「poteto:modeは使わないと思っているけれども節々のSKILLや、どのように作られているか使いながら学べるから」。

## 前提（同意済み。番号）

1. 入口の `poteto-mode`（回し方）は使わない。使うのは個々の skill。pstack の回し方（実装をサブエージェントへ任せる・複数モデルで審査する・人を待たずに進む・PR をマージまで運ぶ）が worker skill・ゲート・独立レビューとぶつかる所は、Andon の決まりが勝つ（人 2026-11-12）。
2. 作りを学ぶために、本家の形のまま丸ごと入れる。使いたい数本だけを抜いて入れたり、Andon 向けに書き換えた写しを持ったりはしない。`poteto-mode` も入るが、呼ばなければ動かない（人が同意 2026-11-12）。
3. 試しはこのセルだけで行い、パッケージは変えない。全セルに配るかは試しのあとに別に決める（言い直しに含めて同意 2026-11-12）。

確かめた事実（前提の裏づけ）:

- 本家は `github.com/cursor/plugins` の `pstack/`（MIT。2026-11-12 の時点で 0.15.36）。skill は 57 本（`principle-*` が 24、ほか 33）、`poteto-mode` の下に playbook が 23。同じ repo の `harness/pstack/claude-code/` に、公式の Claude Code 向けの写しがある（marketplace の名前は `cursor-plugins`、plugin の名前は `pstack`）。Codex 向けは無い。
- 57 本のうち 56 本が `disable-model-invocation: true`（名前で呼ばれたときだけ動く。モデルが自分から起動できるのは `setup-pstack` だけ）。本家の対応表（`docs/harness/claude-code.md`）は「Skill tool は pstack の skill のほとんどを拒むので、`SKILL.md` を Read して従う」と書いている。
- `poteto-mode` の Feature・Bug fix の playbook は、実装をサブエージェントへ任せることを必須にしている。worker skill は「サブエージェントは読むだけ」で、オーナーも「サブエージェントは調べものだけ」と言っている（セルの wiki）。
- 原則（`principle-*`）と `in-your-own-words`・`sharpen`・`tdd`・`benchmark-checklist` はサブエージェントを起動しない。`how` は読むだけのサブエージェントを起動する。`why` は書ける型のサブエージェントを起動する。`architect`・`arena`・`swarm`・`interrogate` は、書くサブエージェントか複数モデルを起動する。
- セルの `~/.claude` は作り直しても残る volume で、Andon の設定は managed settings に入る。利用者の plugin の登録（`~/.claude/plugins`・`~/.claude/settings.json`）は boot に上書きされない。`claude plugin marketplace add`・`claude plugin install` は非対話で打てる。
- 今の外の skill の入れ口（`gh-skills.txt`。[ADR-0018](0018-screen-check-tooling.md)）は skill を1本ずつ入れる形で、plugin を入れる口は boot に無い。
- このセルの担当のプロファイルは、台帳の記録で Claude Code 系が 127 件（`claude` 86・`sonnet` 39・`fable` 2）、Codex が 1 件。レビュアーは `codex-review` が 98 件。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-12 入れるのは本家の公式の Claude Code 向けの写し（`pstack@cursor-plugins`）。plugin として入れる。** セルの中で `claude plugin marketplace add cursor/plugins` と `claude plugin install pstack@cursor-plugins`（user scope）を1回打つ。
   理由: 本文が本家そのままで作りを学べる。hook も他の CLI の起動も無く、入れただけでは何も動かない。このセルの担当はほぼ Claude Code。人の言葉「pstack cursor-plugins をpluginとして入れればいいと思います」。
   捨てた案: michael-denyer/pstack-claude（Codex にも入り tag で固定できるが、毎セッション「`poteto-mode` を呼べ」と足す hook を持ち、方針の書き換え 57 件が本家との差になる）／ericlitman/open-pstack（Codex・Grok の CLI を起動して複数モデルで回す前提。前提 1 と合わない）／backnotprop/pstack（plugin でなく、入れ方が対話の選択だけ。`pstack:` の名前が付かず、`how`・`why` などの名前がそのまま利用者の skill の置き場に並ぶ）。
2. **2026-11-12 担当に数行の指示を足す。置き場はセルの `AGENTS.local.md`。** 読ませるのは、読むだけで完結する skill に限る。人が自分のセッションで名前で呼ぶ（`/pstack:<名前>`）のは自由。
   理由: 入れただけでは担当の動きは変わらない（56 本は名前で呼ばれたときだけ動く）。常時の指示があれば使った記録が溜まり、効き目を数で見られる。
   捨てた案: 入れるだけにして、人が名指ししたときだけ使う（呼ばなければ効き目が分からない）／担当とレビュアーの両方に指示する（試しの途中でレビューの厳しさが変わり、前後の数を比べにくい）。
3. **2026-11-12 担当に読ませるのは、絞った原則 6 本と `how`。** 工程と skill の対応は次のとおり。担当は、読んだ skill と、それで変えた選択を報告に1行ずつ書く。

   | 担当の工程（worker skill §2） | 場面 | 読む skill |
   |---|---|---|
   | 理解 | 複数のファイルにまたがる仕組みを変える | `how` |
   | 計画 | 差分の大きさを決める | `principle-laziness-protocol`・`principle-subtract-before-you-add` |
   | 計画 | 状態や分岐を持つコードを書く | `principle-model-the-domain` |
   | 計画 | バグを直す | `principle-fix-root-causes` |
   | 確認 | テストを書く・直す | `principle-test-behavior-not-implementation` |
   | 確認 | 完成と言う前 | `principle-prove-it-works` |

   指示には次も書く: `poteto-mode` と、サブエージェントに書かせる skill（`architect`・`arena`・`swarm`・`interrogate` など）は使わない。ラインの決まりは worker skill が正。
   理由: 原則 24 本の「どの場面でどれを読むか」の索引は `poteto-mode` の中にしか無い。`poteto-mode` を使わないので、場面と skill の対応はセルの指示に書く。少ないほうが毎回読まれ、どれが効いたかを報告から拾いやすい。足す・減らすは判定のときに決める。
   捨てた案: 原則 24 本すべて（`poteto-mode` の原則の索引の節だけを読ませる。ほかの節の回し方まで読んで引きずられる恐れ）／`how` だけ（差分の大きさ・確かめ方・根本原因の作法を試せない）。
4. **2026-11-12 版は自動で最新に上げる。** marketplace `cursor-plugins` の自動更新を入れる。試しの始めと判定のときに、入っている版の番号を報告に書く（途中で版が変わったことが分かるように）。
   理由: 人が選んだ（yomiyasu の試しと同じ扱い。[ADR-0013](0013-short-role-names-and-plain-japanese.md) 決めたこと 19）。本家の直しがすぐ届く。
   捨てた案: 入れた版に留める（Chief Engineer の推奨。試しの途中で本文が変わらず前後の数を比べやすい。担当が読む指示が誰の目も通らずに変わることも無い。人は自動で上げるほうを選んだ）／手で上げる（CoS が差分を読んでから `claude plugin update` を打つ）。
5. **2026-11-12 効いたかは台帳のレビューの記録で見る。** 指示を入れたあとにレビューを受けた 30 件を、今の数と比べる。中身は「価値の仮説」の節。
   捨てた案: 数の線を置かず、オーナーが報告を読んで判定する（30 件では数のぶれが大きい、という理由で出した案）／60 件たまるまで待つ（判定が1週間遅れる）。
6. **2026-11-12 「作りを学ぶ」は課題にしない。** オーナーが自分のセッションで名前で呼んで試す。判定の報告には、担当の「読んだ skill と変えた選択」の表だけを付ける。skill の作り直しは、既にある課題（A-0123 など）で続ける。
   捨てた案: 判定の課題に「pstack の作りと Andon の skill の作りを比べ、取り入れる候補を並べる」を1項目足す／今すぐ調べる課題を切り、worker・CoS skill を pstack の形（短い葉と別ファイルの手順）に割る案まで調べる（「skill の大きさはこの題で扱わない」を覆すことになる）。
7. **2026-11-12 plugin の登録と自動更新は、セルの利用者の `~/.claude/settings.json` に書かれた状態を正とする。**（改め。PR #210 のマージのあと）コマンドで入れても手で書いても、次の状態になっていればよい。対話の `/plugin` の画面は要らない。

   ```json
   {
     "extraKnownMarketplaces": {
       "cursor-plugins": {
         "source": { "source": "github", "repo": "cursor/plugins" },
         "autoUpdate": true
       }
     },
     "enabledPlugins": { "pstack@cursor-plugins": true }
   }
   ```

   理由: 人の言葉（2026-11-12）「結果としてsettings.jsonに記述されることになると思うのでその状態で良いように思った」。Claude Code の文書で確かめた: 外の marketplace の自動更新は `extraKnownMarketplaces.<marketplace の名前>.autoUpdate` に書け、利用者の設定に書ける。`claude plugin marketplace add` は利用者の設定の `extraKnownMarketplaces` に、`claude plugin install` は `enabledPlugins` に書く。2つのキーを手で書いておくだけでも、次のセッションの始めに marketplace が取られ、plugin が入る。
   Chief Engineer の読み: ここで言う `settings.json` は、セルの利用者の設定（`~/.claude/settings.json`。volume に残り、boot は書き換えない）を指す。パッケージの `package/settings.json`（managed settings。全セルに配る）ではない（前提 3）。人に確かめを出したが、答えはまだ無い（開いている問い）。
   捨てた案: 人が `/plugin` の画面で自動更新を入れる（開いている問いに置いていた回り道。要らなくなった）。

## やらないこと（範囲の外。理由）

- **`poteto-mode` を使わない**（前提 1）。担当の指示にも、依頼書にも書かない。
- **担当の決まりを緩めない。** サブエージェントは読むだけ・マージはゲート・人の決断は待つ、は worker skill のまま。pstack の「人を待たない」（`principle-never-block-on-the-human`）は担当に読ませない。
- **`setup-pstack` を走らせない。** モデルの割り当て表（`~/.claude/rules/pstack-models.md`）を書かない。複数モデルの審査を使わないので要らない。表が無いとき、`how` のサブエージェントはセッションのモデルで動く（本家の対応表の決まり）。
- **Codex には入れない。** 本家の写しは Claude Code だけ。Codex の担当（台帳の記録で 1 件）とレビュアーには指示を足さない。
- **レビュアー・CoS・CE には指示を足さない**（決めたこと 2）。CE の骨格は既に pstack から借りている（[design.md](../design.md) §1.8）。
- **Andon が既に持つものを pstack に替えない。** PR の書式と検査（worker skill §3）、別ブランドの独立レビュー（§4。`interrogate`・`arena` の代わり）、日本語の文（`writing.md`・yomiyasu。`unslop`・`technical-writing` は英語向け）、気づきの蒸留（[ADR-0020](0020-auto-memory-as-intake.md)。`reflect`・`correct`・`recall` の代わり）、画面の確かめ（ADR-0018。`create-verification-skill` の代わり）。
- **パッケージを変えない**（前提 3）。boot に plugin を入れる口を足さない。`package/gh-skills.txt` も触らない。全セルへの配り方は、判定で「続ける」になったときの別の題。
- **pstack の本文を Andon の repo に写さない・訳さない。** セルの指示に書くのは、場面と skill の名前の対応だけ。
- **skill の大きさ（worker 291 行・CoS 503 行）はこの題で扱わない**（依頼書の経緯。A-0123 などで続ける）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目はセルの実測だけ（Andon の repo のコードは変えないので、`tests/` に足すものは無い）。確かめた結果は課題の報告に書く。

1. **入った**（P1）: セルで `claude plugin list` に `pstack@cursor-plugins` が有効で出る。セルを再起動したあとも出る。新しく起動した Claude Code のセッションで `/pstack:how` が候補に出る。入っている版の番号が報告に書いてある。
2. **入れただけでは動かない**（P1）: 指示を足す前に起動した担当1つのセッションの記録に、pstack の skill を読んだ跡が無い。`~/.claude/rules/pstack-models.md` が無い。
3. **自動更新が入っている**（P1）: セルの `~/.claude/settings.json` に、決めたこと 7 の2つのキー（`extraKnownMarketplaces` の `cursor-plugins` に `autoUpdate: true`、`enabledPlugins` に `pstack@cursor-plugins`）がある。セルを再起動したあとも残っている。
4. **担当が読む**（P1）: 指示を足したあとに起動した Claude Code の担当が、決めたこと 3 の場面で、名指しされた skill の本文に辿り着いて読んでいる（セッションの記録で確かめる）。報告に「読んだ skill と、それで変えた選択」の行がある。版が上がって置き場のパスが変わっても辿り着ける書き方になっている。
5. **引きずられない**（P1・P2）: その担当が、サブエージェントに書かせていない・`poteto-mode` を読んでいない・人の決断を待たずに進めていない。
6. **判定**（P2。人の手）: 見る日に、「価値の仮説」の 2 の数と、担当の「読んだ skill と変えた選択」の表が1つの報告にまとまり、人が「続ける」「全セルへの配り方を別の題にする」「やめる」のどれかを答えている。「やめる」なら、`AGENTS.local.md` の行が消え、`claude plugin uninstall pstack@cursor-plugins` のあと、新しいセッションに `pstack:` の skill が出ない。

## 価値の仮説

1. **誰の何が変わるか**: このセルの Claude Code の担当の成果。1回目のレビューで重い指摘が付く課題が減る。オーナーは担当の報告の行から、どの skill がどう効いたかを読める。
2. **見る信号**: 台帳の記録（外の信号の出所は無い）。主の数は、1回目のレビュー（`issues/<id>-r1/library/findings.json`）で `critical` か `major` が付いた課題の割合。基準は 48%（2026-11-12 の時点で 60 件中 29 件）。添える数は、2回目以降のレビューが要った課題の割合（基準 51%。61 件中 31 件）、`andon measure` の `doing` の待ち時間（基準 1.6 時間。2026-11-05〜11-12）、担当の報告の「読んだ skill と変えた選択」の行の数。
3. **見る日**: 2026-11-20（指示を入れたあとにレビューを受けた課題が 30 件に届いていなければ、届いた日）。
4. **誰が要るか**: オーナー（判定: 続ける・全セルへの配り方を別の題にする・やめる）。
5. **効いたとどう分かるか**: 指示を入れたあとの 30 件で、主の数が 35% 以下。かつ、その担当の報告の過半に「変えた選択」の行がある。
6. **何があれば止めるか**: 担当が pstack の回し方に引きずられた例（サブエージェントに書かせた・人を待たずに決めた・マージしようとした）が 1 件でも出たら、指示を直すか外す。`doing` の待ち時間が 2.4 時間を超えたらやめる。主の数が 45% 以上なら外れ。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

セルの設定の変更だけで、Andon の repo の PR は出ない（この文書の PR を除く）。

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| P1 | セルに入れる: `claude plugin marketplace add cursor/plugins`・`claude plugin install pstack@cursor-plugins`・自動更新を有効にする。担当1つで「入れただけでは動かない」を確かめてから、`AGENTS.local.md` に決めたこと 3 の指示を足す。担当1つで「読む」「引きずられない」を確かめる | 受け入れ条件 1〜5 | この文書のマージ |
| P2 | 観察（人の手を含む）: 見る日に数と表をまとめ、人に判定を聞く | 受け入れ条件 6 | P1 のあと。2026-11-20 か、30 件たまった日 |
| P3 | 判定のあとの課題: 「全セルへ」なら CE の課題（boot に plugin を入れる口を足すか・指示の行をパッケージのどこに置くか・Codex の担当にどう読ませるか）。「やめる」なら設定を消す | 判定の答えによる | P2 |

P1 で CoS が決める細部（案。人の決断ではない）:

- **誰が打つか**: plugin の登録はセルの HOME に書く。`AGENTS.local.md` はセルの設定（ラインの変更）。委譲の範囲に入るかは CoS が見る。入らなければ人の手。
- **自動更新の入れ方**: 決めたこと 7 の状態にする。`claude plugin marketplace add`・`claude plugin install` を打ったあと、`extraKnownMarketplaces` の `cursor-plugins` に `autoUpdate: true` を足す（コマンドがこのキーを書くかは P1 で見る。書かなければ手で足す）。対話でないセッションでは plugin が裏で入るので、最初のターンにまだ無いことがある（Claude Code の文書。`CLAUDE_CODE_SYNC_PLUGIN_INSTALL=1` で入り終わるのを待つ）。
- **指示の文の案**（`AGENTS.local.md`。担当向け。Codex の担当は plugin が無いので対象から外す）:
  「Claude Code の担当は、pstack（plugin `pstack@cursor-plugins`）の次の skill を、当てはまる工程で読んでから進む。理解: 複数のファイルにまたがる仕組みを変えるとき `how`。計画: 差分の大きさを決めるとき `principle-laziness-protocol` と `principle-subtract-before-you-add`、状態や分岐を持つコードを書くとき `principle-model-the-domain`、バグを直すとき `principle-fix-root-causes`。確認: テストを書く・直すとき `principle-test-behavior-not-implementation`、完成と言う前に `principle-prove-it-works`。Skill tool が拒むので、plugin の置き場の `skills/<名前>/SKILL.md` を Read する（`how` は同じ置き場の `docs/harness/claude-code.md` の対応表も読む）。報告に、読んだ skill と、それで変えた選択を1行ずつ書く。`poteto-mode` と、サブエージェントに書かせる skill（`architect`・`arena`・`swarm`・`interrogate` など）は使わない。ラインの決まりは worker skill が正。」
- **置き場の辿り方**: plugin の置き場は `~/.claude/plugins/cache/<marketplace>/<plugin>/<版>/` で、版が上がるとパスが変わり、古い版は 14 日後に消える（Claude Code の文書）。指示には固定のパスを書かず、辿り方（`claude plugin list --json` の `installPath`）を書く。どの書き方で担当が辿り着けるかは P1 で実測する（受け入れ条件 4）。
- **`AGENTS.local.md` は全部の役が読む。** 指示の文は「Claude Code の担当は」で始め、CoS・CE・レビュアーが自分への指示と読まないようにする。
- **基準の数の取り方**: P2 は、指示を足した時刻より後に起動された担当の課題だけを 30 件数える。指示を足した時刻を P1 の報告に書く。

## 開いている問い（残った問い。決断の種類つき）

- **判定**（ラインの変更。P2 で人が答える）: 続ける・全セルへの配り方を別の題にする・やめる。読ませる skill を足すか減らすかも、そのときに決める。
- **決めたこと 7 の `settings.json` が指すもの**（ラインの変更に当たるかの確かめ。人が答える）: Chief Engineer はセルの利用者の `~/.claude/settings.json` と読んだ。パッケージの `package/settings.json` に書く話なら、前提 3（パッケージは変えない）を覆す別の決断になる。
- 「自動更新を非対話で入れられるか」は閉じた（決めたこと 7。`settings.json` のキーで入る）。
- **design.md §1.8 の pstack の出所の表記**（事実の直し。決断は無い）: 今の文は「Claude Code の plugin `cursor/pstack`」と書いているが、本家の置き場は `cursor/plugins` の `pstack/`。直すかは CoS が見る。

## 出典（読んだもの・捨てた案の出所）

- 人の言葉（2026-11-12）: CoS との対話（依頼書の経緯）と、この対話での言い直し・前提 1〜2・決断 6 問への答え。
- pstack の本家: `github.com/cursor/plugins`（2026-11-12 の時点で commit 2eaf465。`pstack/` を最後に変えた commit は c742dd6、0.15.36）。読んだのは `pstack/README.md`・`pstack/plugin.json`・`pstack/LICENSE`・`pstack/docs/harness/claude-code.md`・`harness/pstack/README.md`・`harness/pstack/claude-code/`（`.claude-plugin/plugin.json`・`agents/poteto-agent.md`）・`.claude-plugin/marketplace.json`・`pstack/skills/poteto-mode/SKILL.md` と `playbooks/{feature,bug-fix,opening-a-pr,babysit}.md`・`pstack/skills/{how,why,architect,interrogate,show-me-your-work,poteto-help}/SKILL.md`・`pstack/skills/principle-{never-block-on-the-human,prove-it-works,laziness-protocol,guard-the-context-window,fix-root-causes,test-behavior-not-implementation,subtract-before-you-add,model-the-domain}/SKILL.md`・全 skill の frontmatter。本文は写していない。
- 移植3つ（読むだけのサブエージェントが調べた。2026-11-12）: `github.com/michael-denyer/pstack-claude`（v0.9.136。`README.md`・`docs/reference.md`・`tools/forks.json`・`tools/upstream.json`・`plugins/pstack/hooks/`）、`github.com/ericlitman/open-pstack`（v1.21.0。`README.md`・`UPSTREAM.md`・`docs/reference.md`・`plugins/pstack/skills/poteto-mode/references/{model-catalog,provider-dispatch}.md`）、`github.com/backnotprop/pstack`（`README.md`・`MIRROR.md`・`SYNC-LOG.md`）。どれも実際には入れていない。
- Andon の側: 依頼書（A-0160）、[design.md](../design.md) §1.8（CE の骨格が pstack から借りたもの）・§5（置き場の表）、[ADR-0001](0001-skills-distribution.md)、[ADR-0013](0013-short-role-names-and-plain-japanese.md) の改め（yomiyasu の試しの形）、[ADR-0018](0018-screen-check-tooling.md)、`docs/decisions/2026-11-11-boot-bundled-and-gh-skills.md`、`internal/boot/extskills.go`・`internal/boot/boot.go`（managed settings の置き方）、`package/settings.json`、`package/compose.yaml`（HOME の volume）、`package/skills/worker/SKILL.md`・`package/skills/independent-review/SKILL.md`（「pstack を使う repo」の節）、`package/templates/claude.md`。セルの wiki の `entities/owner.md`・`sources/2026-11-11-owner-agent-teams.md`。
- 道具の help（2026-11-12。セルの中）: `claude plugin --help`・`claude plugin install --help`・`claude plugin marketplace --help`（Claude Code 2.1.322）、`codex plugin --help`（0.166.0）、`gh skill install --help`（2.103.3）。
- 数: `andon measure --since 2026-11-05`（2026-11-12）、台帳の `issues/*-r<回>/library/findings.json`（1回目のレビューの `severity`）と `issue.yaml` の `profile`。
- 改め（2026-11-12）: 人の言葉（この対話。PR #210 のマージのあと）。Claude Code の文書（読むだけのサブエージェントが調べた。2026-11-12）: https://code.claude.com/docs/en/plugins/loading.md ・ https://code.claude.com/docs/en/plugins/org.md ・ https://code.claude.com/docs/en/plugins/install.md ・ https://code.claude.com/docs/en/plugins/cli-reference.md ・ https://code.claude.com/docs/en/plugins/create-marketplace.md 。セルの `~/.claude/settings.json`（plugin の項目はまだ無い）と `claude plugin marketplace add --help`（`--scope user` が既定）。marketplace の名前 `cursor-plugins` は本家の `.claude-plugin/marketplace.json` の `name`。
- 反論のサブエージェントは走らせていない（セルの設定で戻せる試しで、外向きの面も戻せない設計も無い）。
