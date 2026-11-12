# ADR-0022: Chief Engineer の役は残し、Chief of Staff に Advisor（Fable）を試験運用で付ける（仕様・設計。2026-11-12。Chief Engineer の課題 A-0161）

- 状態: **提案（2026-11-12）**。人が 2026-11-12 に「決めた」と言った（Chief Engineer の対話 A-0161）。人が PR をマージした日が決定の日（[decisions/2026-11-12-adr-0022-advisor-and-chief-engineer.md](../decisions/2026-11-12-adr-0022-advisor-and-chief-engineer.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。試験運用の結果（見る日 2026-11-27）は「決めたこと」に下から足す。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。
- 主題: Claude Code の Advisor（メインのモデルが途中で強いモデルに助言を求める機能）を使えば、仕様・設計を人と対話する役（Chief Engineer。[design.md](../design.md) §1.8）を外せるか。結論は「外さない。Advisor は Chief of Staff の判断を強めるために、試験運用で付ける」。プロファイルと skill の変更は Andon のラインの変更（レベル C。人がマージ）。
- 語は [design.md](../design.md) §0.1。機械が読む形（`contract.md`・`schema/`）は変えない。
- 入力: 依頼書 A-0161（人の発言 2026-11-12 と Chief of Staff の経緯）、design.md §1.8、[ADR-0009](0009-cos-default-entry.md)、[ADR-0001](0001-skills-distribution.md)、[ADR-0004](0004-project-layout.md)、`package/templates/chief-engineer.md`・`chief-of-staff.md`、`package/skills/chief-of-staff/SKILL.md`・`chief-engineer/SKILL.md`、台帳（これまでの Chief Engineer の課題 13 件）、セルの wiki、Advisor とカスタムエージェントの公式の説明（出典）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-12）: 「あとは Claude の Advisor 機能を使うことで ChiefEngineer を除外できるのでは？とも思っている」。

言い直し（人が同意 2026-11-12）。人が減らしたいのは役の数。今は「何を作るか」を詰めるたびに Chief of Staff が Chief Engineer を別の端末に起こし、人はそちらへ移って話し、文書の PR を経て Chief of Staff に戻る。Chief Engineer を分けた理由の1つは、問いを重ねる仕事に最上位のモデル（Fable）を当てたいことだった。Advisor を使えば、Opus で動く Chief of Staff が要所で Fable の助言を借りられる。それなら設計の対話も Chief of Staff がそのまま受け持てて、Chief Engineer の役（command・プロファイル・skill・題ごとの課題）を外せるのではないか、がこの題。困りごとは「役と受け渡しが1段多いこと」で、Fable の費用や Chief Engineer の対話の質そのものではない。

対話の中で人の案は2度動いた（2026-11-12）。

- 「会話相手を1つに絞り、設計の役は CoS（＋Advisor）をベースにしたほうがいいかなと思っている、それで足りないのであれば CoS の判断で Fable の席やカスタムエージェント（ChiefEngineer）を呼び出す流れができればいい」。
- そのあと、設計の題が1題ずつになることとラインの通知が対話に混ざることを見て: 「通知も来たりすることを考えると分離されているのが好ましいね、ChiefEngineer の席は残したまま、Advisor をつける（CE を呼び出すか判断する）のがいいかもね」。

### 確かめた事実（Advisor。2026-11-12）

- メインのモデルが、作業の途中で自分より強いモデルに助言を求める。呼ぶかどうかはメインのモデルが決める（方針を決める前・同じエラーが続くとき・完了を言う前が典型）。人は「advisor に相談してから続けて」と頼める。呼ぶ回数を決めたり強いたりする設定は無い。
- advisor が見るのはメインの会話の全体（tool の呼び出しと結果を含む）。返すのは助言の文章だけで、返す先はメインのモデル。advisor は tool を使えず、人と直接やりとりしない。
- 入れ方は `/advisor`（選んだ値は利用者の設定の `advisorModel` に残る）・設定の `advisorModel`・起動フラグ `--advisor <model>`（そのセッションだけ。`claude --help` には出ない）。
- 組み合わせ: メインが Opus なら advisor に Fable を置ける。メインが Fable 5.2 なら advisor は Fable 5.2（か Mythos 5.2）に限られる。
- 版: Claude Code v2.1.260 以上。Fable 5.2 の advisor は v2.1.321 以上（セルは 2.1.322）。公式には experimental。
- 費用: advisor を呼んだ分は advisor のモデルの単価で別に数えられ、プランの利用上限にも数えられる（単価は未確認）。
- セルの今: `advisorModel` の設定は無い。全部の役が同じ HOME を使うので、どこかの端末で `/advisor` を打つと、担当やレビュアーにも付く。

### 確かめた事実（カスタムエージェント。2026-11-12）

- 定義は `.claude/agents/<name>.md`（プロジェクト・利用者・plugin の `agents/`）。モデル（`fable`）・effort・読ませる skill を定義ごとに書ける。
- メインとは別の文脈で動き、メインに戻るのは報告だけ。メインは同じエージェントに文脈を保ったまま続きを頼める（Claude Code を起こし直したあとにどこまで続けられるかは公式の説明に明記が無い）。
- 人は `/tasks` からそのエージェントを開いて直接打てる。カスタムエージェントは選択式のダイアログ（AskUserQuestion）を使えない。
- カスタムエージェントは自分でもサブエージェントを起こせる（既定で3段まで）。設定した advisor を継ぐ。
- Andon の過去の決定: 置き場は ADR-0004 決めたこと 1 が例に挙げている（プロジェクトの `.claude/agents/`）。パッケージの `agents/` は置けば plugin の部品として読まれる（ADR-0001 決めたこと 2。今は空）。役のセッションそのものを `claude --agent <役>` で起こす形は人が退けた（ADR-0001。2026-11-05）。

### 確かめた事実（台帳。2026-11-12）

- これまで Chief Engineer に回した課題は 15 件（動いている 2 件を含む）。終わった 13 件は全部が文書（ADR）の PR になり、「決断が無い小さな話」として文書なしで返った例（ADR-0009 決めたこと 4）は 0 件。
- 2026-11-12 は Chief Engineer の課題が2つ（A-0160・A-0161）並んで動いた。

## 前提（同意済み。番号）

1. （人が同意 2026-11-12）設計の問いと人の答えは Chief of Staff の会話に載る。2026-11-04 に役を分けた理由（1つの議論で数十の判断が載って文脈が太り、ラインを回す判断と混ざる）は Advisor では消えない。対話が Chief of Staff に載ることは受け入れ、太る分は、重い読みと検討を外（Fable の席・カスタムエージェント）に出すことと、決めたことをその都度文書に書くことで受ける。→ 改めた（前提 2。同じ日）。
2. （人の言葉 2026-11-12）設計の対話は Chief of Staff の会話と分ける。会話相手を Chief of Staff の1本にすると、設計の題は1題ずつになり（今は別々の端末で同時に詰められる）、ラインの通知が対話の合間に入る。だから Chief Engineer の席（別の端末・別の文脈）は残し、Advisor は Chief of Staff に付ける。前提 1 の「対話が Chief of Staff に載ることは受け入れる」は取り下げる。
3. （人の言葉 2026-11-12）「Advisor は Experimental だしトークン制限もあるので、試験運用である前提でいきましょう」。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-12 Chief Engineer の役は外さない。振り分けの線（判断〈構造の違う案が2つ以上・外との約束・何を作るか〉は Chief Engineer へ。Chief of Staff は設計を詰めない）も動かさない。Advisor は Chief of Staff の判断を強めるために使う。**
   理由: 線は効いている（終わった 13 題は全部が文書の PR になった。1問で済む決断は今も Chief of Staff がその場で人に聞いている）。設計の対話を Chief of Staff に載せると、題が1題ずつになり、ラインの通知と混ざる（前提 2）。Advisor が代われるのは「強いモデルの知恵」だけで、文脈の分離は代われない。
   捨てた案: Chief Engineer を外し、Chief of Staff ＋ Advisor が設計の対話を受け持つ（人の最初の案。前提 2 で人が退けた）／Chief Engineer をカスタムエージェントにして Chief of Staff の中から呼ぶ（人は `/tasks` から話せるが、選択式のダイアログが使えず、起こし直したあとの続きが公式に明記されていない。席を残すと決めたので要らない）／小さな設計は Chief of Staff が Advisor の助言を借りてその場で詰め、足りなければ Chief Engineer を呼ぶ（「小さい」の線引きが裁量になり、決定の記録の置き場が新しく要る）。
2. **2026-11-12 Advisor（Fable）を付けるのは Chief of Staff だけ。** 担当・レビュアーへ広げるかは、効きを見てから別の題で決める。
   理由: 変えるものが1つなので効きが見やすい。Advisor は experimental で、呼ばれるたびに会話の全体を読み、その分が advisor のモデルの単価で別に掛かる。Fable で動く役（Chief Engineer・`fable` の席）は advisor も Fable に限られ、得が薄い。
   捨てた案: Chief of Staff と Opus・Sonnet の担当（公式の用途には一番合うが、課題の数だけ費用が増え、効きの出所が分かれる）／Claude 系の全部の役（experimental の機能にライン全体が依る）／まだ入れず、人が手で `/advisor` を試すだけ。
3. **2026-11-12 入れ方は、パッケージの `chief-of-staff` のプロファイル（`package/templates/chief-of-staff.md`）の起動行に `--advisor fable` を1つ足す。** レベル C（人がマージ）。パッケージの版上げで全セルへ届く。
   理由: 起動フラグはそのセッションだけに効くので、Chief of Staff だけに付く（2）。
   捨てた案: `/advisor` で入れる（選んだ値が利用者の設定に残り、同じ HOME を使う担当やレビュアーにも付く）／このセルの `templates.local` だけで先に試す（人だけが書ける場所。効いたらパッケージへ、の2段になる）／managed settings に `advisorModel`（全部の役に付く）。
4. **2026-11-12 Chief of Staff の skill の「人の入口」に1行だけ足す: 振り分けに迷うとき（直す課題か Chief Engineer か）と、2回差し戻して切り直すときは、advisor に相談してから決める。** それ以外の場面は書かず、モデルの判断に任せる。
   理由: 呼ぶかどうかはモデルが決め、回数を強いる設定は無い。人が挙げた使いどころ（Chief Engineer を呼ぶかの判断）は、公式の既定のきっかけに入っていない。Chief of Staff の skill は 506 行あり、長さを減らす課題（A-0123）が待っているので、足すのは1行にとどめる。
   捨てた案: 書かない（skill は増えないが、振り分けで呼ばれるかが分からない）／相談する場面を節として並べる（振り分け・依頼書を切る前・検収・人に問いを上げる前。呼ばれる回数と費用が増え、skill も長くなる）。
5. **2026-11-12 効いたかは、オーナーの目視で 2026-11-27 に見る**（下の「価値の仮説」）。advisor が呼ばれた回数を数える信号は足さない。
   捨てた案: 価値の仮説を置かない（担当へ広げるかを決める材料が残らない）。
6. **2026-11-12 試験運用として入れる**（前提 3）。試験運用のあいだは次のとおり。
   - design.md §1.8 の役の表と本文は書き換えない。Chief of Staff のプロファイルを書いている所に「Advisor（Fable）を試験運用。ADR-0022」と1行で指すだけ。
   - 見る日（2026-11-27）に、オーナーが「続ける・止める・広げる」を決める。結果はこの ADR の「決めたこと」に下から足す。
   - 止めるときは、プロファイルのフラグと skill の1行を戻す（1つの PR）。
   - プランの利用上限に当たってラインが止まったら、見る日を待たずに止める。

## やらないこと（範囲の外。理由）

- **Chief Engineer を外す・カスタムエージェントにする・振り分けの線を動かす**（決めたこと 1）。
- **担当・レビュアーに Advisor を付ける**（決めたこと 2）。見る日の結果を見てから別の題。
- **セルにカスタムエージェントを足す口を作る**。人が対話の途中で挙げた案（「必要に応じて Cell に置けるようにし」）だが、Chief Engineer の席を残すと決めたので、この題では決めない（開いている問い）。
- **advisor が呼ばれた回数を数える信号・`andon measure` の欄**（決めたこと 5）。試験運用のあいだは目視。
- **Codex 側の対応**。Advisor は Claude Code の機能。
- **`/advisor` をセルで打つ運用**。設定が HOME に残り、全部の役に付く（決めたこと 3）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は2つ。どちらも既存のもの: `tests/package.sh`（template と skill の文言）と、セルでの実測（`package/` の変更の決まり）。

1. **`tests/package.sh`**: `chief-of-staff` の template の `command` に `--advisor fable` がある。ほかのプロファイル（`chief-engineer`・`claude`・`sonnet`・`fable`・`claude-review`）の `command`・`resume` には `--advisor` が無い。
2. **`tests/package.sh`**: Chief of Staff の skill の「人の入口」の節に、advisor に相談する場面（振り分けに迷うとき・2回差し戻して切り直すとき）の1行がある。
3. **セルで実測（マージの前）**: 変えた template で Chief of Staff を起こすと起動し、`/advisor` の表示で Fable が付いている。「advisor に相談してから続けて」と頼むと画面に `Advising` の行が出て、結果が `Unavailable` でない。
4. **セルで実測（マージの前）**: `claude` のプロファイルで起こした担当には advisor が付いていない（HOME の設定に `advisorModel` が書かれていない）。
5. **セルで実測（マージの前）**: advisor が付かない条件（`CLAUDE_CODE_DISABLE_ADVISOR_TOOL=1` を付けて起こす）でも、`--advisor fable` の付いた起動行で Chief of Staff の起動が止まらない。止まるなら、マージせずに報告する（入れ方を決め直す。Chief of Staff は人の入口なので、起動しないことが一番重い）。

## 価値の仮説

1. **誰の何が変わるか**: オーナー。Chief of Staff の振り分けと切り直しの判断が良くなり、オーナーが振り分けを言い直す手と差し戻しが減る。
2. **見る信号**: オーナーの目視（Chief of Staff の画面の `Advising` の行とその前後の判断、`/usage` の費用）。参考に `andon measure` の差し戻し率（入れる前後の2週間。ほかの変更の影響も受けるので参考にとどめる）。
3. **見る日**: 2026-11-27（マージ見込み 2026-11-13 の14日後）。
4. **誰が要るか**: オーナー（画面と費用を見る）。
5. **効いたとどう分かるか**: 2週間で、振り分けか切り直しの場面で advisor が1回以上呼ばれ、オーナーが「助言で判断が良くなった」と見た例が1つ以上ある。
6. **何があれば止めるか**: 一度も呼ばれない／費用が見合わないとオーナーが見た／プランの利用上限に当たってラインが止まる／Chief of Staff の起動が失敗する・応答が目立って遅くなる。止め方はプロファイルのフラグと skill の1行を戻す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 切る課題 | 受け入れ条件 | 依存 |
|---|---|---|---|
| V1 | `package/templates/chief-of-staff.md` の起動行に `--advisor fable`、Chief of Staff の skill の「人の入口」に1行、`tests/package.sh` の検査、design.md に試験運用の1行（決めたこと 6）。マージの前にセルで実測する。レベル C（人がマージ） | 1〜5 | この ADR の PR のマージ |
| V2 | 観察: 価値の仮説（人の手。見る日 2026-11-27） | 価値の仮説 5・6 | V1 |

- V1 は Chief of Staff の skill と templates に触る。同じファイルに触る課題（A-0123〈CoS skill の行数・承認の線・古い続きの文〉・A-0047〈W5: skill・templates の語の置き換え〉）とぶつからないように並べる。
- V1 がマージされても、動いている Chief of Staff には付かない。パッケージの版が上がり、Chief of Staff を起こし直したときから効く。

## 開いている問い（残った問い。決断の種類つき）

- **担当・レビュアーに Advisor を広げるか**（ラインの変更）。見る日（2026-11-27）の結果を見てから、要れば別の題。
- **セルにカスタムエージェントを足す口を作るか**（ラインの変更）。人が対話の途中で挙げた。使いどころが出たら別の題。
- **advisor の単価**（事実。未確認）。見る日に `/usage` で実際の費用を見る。

## 出典（読んだもの・捨てた案の出所）

- 人の発言 2026-11-12（Chief of Staff の端末。依頼書 A-0161 の経緯）と、Chief Engineer の対話 A-0161（同じ日）。人が「決めた」と言った日: 2026-11-12（最後の確かめ「共有できたか」への答え）。
- Claude Code の docs「Advisor」（https://code.claude.com/docs/en/advisor ）。
- Claude API の docs「Advisor tool」（https://platform.claude.com/docs/en/agents-and-tools/tool-use/advisor-tool ）。
- Anthropic のブログ「The advisor strategy」（https://claude.com/blog/the-advisor-strategy ）。
- Claude Code の docs「Subagents」（https://code.claude.com/docs/en/sub-agents ）・「Agent teams」（https://code.claude.com/docs/en/agent-teams ）。
- 公式の説明は、読むだけのサブエージェントが 2026-11-12 に読んだ（要約を通して読んだので、細かい値は実装の課題がセルで実測して確かめる）。
- design.md §1.8（役を分けた理由・プロファイルの案 A〜C）、ADR-0009（人の入口は Chief of Staff・誤った振り分けの戻し方）、ADR-0001（`--plugin-dir`・`claude --agent` を退けた経緯）、ADR-0004（`.claude/agents/` の置き場）。
- 台帳: profile が `chief-engineer` の課題 15 件の `issue.yaml` と `report.json`（2026-11-12）。
- セルの wiki: `entities/owner.md`、`sources/2026-11-11-owner-effort-high.md`（対話の役も effort high）、`sources/2026-11-11-owner-agent-teams.md`（サブエージェントは調べものだけ）。
- 反論のサブエージェントは走らせていない（外向きの面が無く、止め方がフラグと1行を戻すだけで、不可逆な設計が無いため）。
