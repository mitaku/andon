# ADR-0009: Chief of Staff を人の既定の入口にする（CoS と Chief Engineer の skill）（仕様・設計。2026-11-09。Chief Engineer の課題 A-0008）

- 状態: **決定（2026-11-09）**。人が対話の中で「決めた」と言った（2026-11-09）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: 人が話す相手を基本 Chief of Staff にし、何を作るかを詰めるときだけ Chief Engineer と直接話す形に、`package/skills/chief-of-staff/SKILL.md` と `package/skills/chief-engineer/SKILL.md` を寄せる。skill の変更は Andon のラインの変更（レベル C。人がマージ）。
- 語は [design.md](../design.md) §0.1。機械が読む形（`contract.md`・`schema/`）は変えない。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-09）: 「思想としては極力 CoS との会話だけにしたい。しかし CoS を全て挟んでも仕方がないので Chief Engineer とも必要に応じて会話する」「管理職には違いないんだけれども、優秀な参謀補佐官（CoS）に委譲をしていき、開発が進むようにしていくスタイル」「今はまだ Wiki 運用のアウターループがない」「仕事の進め方の整備も必要」。

人は、仕事を進める相手を基本 Chief of Staff 1人にしたい。管理の大半を参謀に委ね、人の手は「決まりのファイルを書く」「レベル C の PR を押す」「決断に答える」に絞る。ただし「何を作るか」だけは委ねられないので、その話が始まったときだけ Chief Engineer と直接話す。困っているのは、今の skill がその形になっていないこと: (1) 設計の話を Chief of Staff が人に「`andon chief-engineer` を打って」と差し戻し、人が同じ話を2回する。(2) 依頼書が repo を読めば分かることで太り、差し戻しが続いても依頼書を直す手順が無い。(3) 案件をまたぐ学びを wiki に残す手順が無く、Chief Engineer・Chief of Staff が引くものが育たない。(4) Chief Engineer の文書に「作ったものが効いたか」を見る節が無い。これらを skill の文で直したいが、skill は人が触れる面なので、太らせずに直したい（人が同意 2026-11-09）。

## 前提（同意済み。番号）

人が 2026-11-09 に 1〜6 すべてに同意。

1. 新しい役・新しい機械の約束（schema・予約キー）は足さない。足すのは skill の文と、要るなら小さな command の口だけ。
2. 「人が同じ話を2回しない」が振り分けの芯。Chief Engineer が最初に読む経緯は、人が Chief of Staff に言ったことの写し（Chief of Staff の言い直しでも可）で、人がもう一度言わなくて済めばよい。
3. wiki の書き手は Chief of Staff のまま（design.md §6 問い21・ADR-0005 を動かさない）。Chief Engineer と担当は wiki を読むだけ。製品に効く学びは repo（`AGENTS.md`・テスト・`REVIEW.md`）へ、ラインの回し方と案件をまたぐ事実はセルの wiki へ（層で分ける）。
4. P6（信号と価値のループ）に依るもの——Chief Engineer の「読む」に観察の結果、文書に「価値の仮説」の節——は、この題では見出しだけ置く。中身は P6 の題で決める。
5. 配り方は今のまま: skill はパッケージの版上げで全セルへ（ADR-0001）。この題の PR はレベル C で人がマージし、Chief of Staff 向け・Chief Engineer 向けで PR を分ける。
6. 何もしないと: 人か Chief of Staff が手で経緯を渡せば回るが（この題の起動のしかたがそうだった）、人頼み・Chief of Staff の裁量頼みで、セルごとにばらつく。wiki は空のまま。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

人が 2026-11-09 に 1〜11 を決めた（10 問の束のあと、別のコンテキストのレビュアーの反論 5 件を人に見せて 3・5・6・10 を改めた）。

1. **2026-11-09 人の入口は Chief of Staff。振り分けの線は CoS skill §1 の表の「判断（構造の違う案が2つ以上・外との約束・何を作るか）→ Chief Engineer」の行そのもの。当たれば Chief of Staff が人に勧めるのでなく、自分で `andon chief-engineer` を打つ。人の go は別に取らない（人が目の前で話している依頼の続き）。** skill の先頭に「人の入口」の節として出す。
   理由: 線は既にあり、足りないのは「誰が打つか」だけ。人が同じ話を2回しない（前提 2）。
   捨てた案: 人が「設計で」と明示したときだけ起動する（2回話す場面が減らない）／新しい線を引く。
2. **2026-11-09 経緯は依頼書に書く。`andon chief-engineer` に `--brief -` を足し、Chief of Staff が経緯（人の言葉の写し・読んだもの・決めたこと・出典）を stdin で渡す。** 依頼書は今の2行（Chief Engineer の課題。題は人の1行…）の下に経緯が続く形。省けば今のまま。Chief Engineer が最初に読むのは依頼書なので、送り直しが要っても残る。
   理由: `hc send` は台帳に残らず、画面が入力を受けられないと届かない（この題の起動のしかたがそうだった）。
   捨てた案: `hc send` のまま（command は変えないが残らない）／両方（二重）。
   直し漏れを一緒に直す: 「依頼書は人の1行の題」の文（CE skill・design.md §1.8 本文）、design.md §1.8 の図（人が打つ形）、CoS skill の「打つコマンド」の一覧に `andon chief-engineer`。
3. **2026-11-09 Chief Engineer は1プロジェクトに同時に複数いてよい。`andon chief-engineer` の「1プロジェクトに1人」の検査を外す（作り直しで戻った端末の片付けは残す）。CE skill と design.md §1.8 の WIP 制限の行（「同時に2人いない」「前の題が終わらないまま新しい題を頼まれたら一度渡す」）を外す。**
   理由（反論から）: command は課題の状態を見ず Claude Code が動いていれば止まり、端末を片付ける finish は文書の PR のマージ後。Andon の docs はレベル C（人がマージ）なので、マージ待ちの間は判断の題が全部止まる。
   捨てた案: command が課題の状態 `review` を見て数えず端末を片付ける／文書を渡したら Chief of Staff がすぐ finish（design.md §1.8 の決まりを変える）／今のまま（マージを人が早く押す）。
4. **2026-11-09 誤った振り分けはどちらの向きも既存の口で戻す。** 直す課題に切ってしまい担当の範囲の問いで「Chief Engineer へ」の候補が出たら、Chief of Staff が（人に題を勧めるのでなく）`andon chief-engineer --brief -` で CE を起動して経緯を渡し、担当の課題は止めたまま（finish は人の go）。小さな話を CE に回してしまったら、CE が骨格の「言い直す」の段で「決断が無い（直し方が1つ・約束に触れない）」と見たとき文書を書かず、`report.json` の `next`（`to: chief-of-staff`「直す課題に切る: …」）を書いて `deck review`。Chief of Staff はそれを切ったら、人の go 無しで `andon issue finish`（PR が無い。調べる課題と同じ扱い）。
   捨てた案: 人が見て戻す（人の手が増える）。
5. **2026-11-09 依頼書の型に原則を1行足す: 「担当は最初に repo を読む。repo を読めば分かること（作法・構造・直し方）は書かず、なぜ・制約・範囲の外・今回の完了を書く。触る場所はぶつかりの照合の手がかりとして短く」。欄は増やさず、「守ること」の説明に「範囲の外（今回やらないこと）」を足す。** 置き場は §2 の「欄の読み方」の頭（雛形の2行目には置かない。`tests/package.sh` が2行目を「解決すべき問題:」と見る）。
   理由: 原典（Osmani「repo に無いコンテキストだけを渡す」）は既存の「やり方を縛らない」と別の原則（人 2026-11-09）。触る場所は §1・§3 の並行の照合に要るので「短く」で残す（反論）。
   捨てた案: 欄「範囲の外:」を足す（担当が読む欄は worker skill に列挙してあり、足すなら worker skill も直すことになる。人が「守ることに書く」に戻した）／既存の文と置き換える（別の原則なので両方要る）／場所の照合を `issue.yaml` のキーや command に移す（新しい約束。前提 1）。
6. **2026-11-09 同じ課題で2回差し戻して通らなければ、手直しを重ねず、依頼書を直した新しい課題に切る。** Chief of Staff は担当へ差し戻すとき（ゲートの `fix` でも人の差し戻しでも）、先に依頼書に `## 差し戻し（日付）` を1行足してから `hc send` する——この追記の数が記録。2回目の差し戻しのあとも通らなければ、依頼書を直した新しい課題を `deck add`（「なぜ」に元の番号と差し戻しの要点）、元の課題は人の go で `andon issue finish`（PR は閉じる。branch は残る）。レビューの回（`review_rounds`）は担当の工程なので数えない。
   理由（反論から）: `andon issue start` は `todo` しか起動できず finish は `done` にするので、同じ課題は起動し直せない。人の差し戻しは台帳に記録が無く、ゲートの記録は同じ head で上書きされる。依頼書への追記なら新しい約束を足さずに数えられる。
   捨てた案: 同じ課題を finish → start で起動し直す（動かない）／レビューの回も数える／人の差し戻しだけ数える／数を書かず Chief of Staff の判断に任せる／P2（`corrections.json`）まで待つ。
7. **2026-11-09 wiki の手順を Chief of Staff の skill に書く（command にしない）。** 形は Karpathy の LLM wiki（原資料は読むだけ・wiki は LLM が育てる・決まりは指示文に。操作は ingest・query・lint。出典は下）。
   - **何を**: 学び（プロジェクト固有でも横断でも。固有は front matter の印）・確かめた事実・**オーナーの思考と志向**（決断の傾向・推奨案を覆した理由）。製品に効くもの（仕様・作法・テスト）は repo へ（前提 3）。決定は ADR と `decisions/`（wiki には写さない）。
   - **いつ（ingest）**: 課題の片付け（`andon issue finish`）のとき報告の Remember と確かめた事実から。人の答え（推奨案と違う答え・志向が見えた）のとき。
   - **形**: ADR-0005 のまま（1ファイル1件・日付・出典・痩せた `index.md`）。`log.md` は足さず、セルの repo の git の履歴で代える。→ 覆された（[ADR-0016](0016-cell-wiki-llm-wiki.md)。元資料の層・育つページ・`log.md`）
   - **lint**: 書くたびに `index.md` の近い項を読み、矛盾すれば古いページに「→ 覆された（新しいページ）」を1行足す（上書きしない。`decisions/` と同じ規律）。定期の課題は起動しない。
   - **query**: Chief Engineer の「読む」（既にある）と、Chief of Staff が切るとき index を引く。
   - **道具**: Chief of Staff の template に `--add-dir /workspace/wiki`（design.md §2.3 が書いているとおりに直す）。commit・push は Chief of Staff（問い10）。書き手は Chief of Staff だけ（問い21）。
   理由: 人の選択（2026-11-09。「SKILL がいい」。Karpathy の gist を根拠に）。
   捨てた案: `andon wiki add` の command（形と index と commit を機械で揃える）／トリアージの周回でまとめて書く／振り返りのルーチンに任せる（まだ無い）／Remember を全部写す／人が指したものだけ／lint をトリアージに足す／`log.md` を持つ。
8. **2026-11-09 skill の行数の上限は置かない（都度判断）。** design.md §2.1 の3「行数が増える変更は、まず command にできないか問う」のまま。
   捨てた案: `tests/package.sh` が行数を数えて今の値を天井にする／数を決める（CoS 300・CE 100 など）。
9. **2026-11-09 Chief Engineer の枠は最小。骨格の「読む」に「前の価値の仮説の結果（観察の課題の報告。P6 で形が決まるまでは無い）」を1行、文書の形に「## 価値の仮説（P6 で形を決める）」の空の見出し。** 中身は P6 の題。
   捨てた案: 見出しに3問（誰が要るか・効いたとどう分かるか・何があれば止めるか）の名前まで入れる／この題では触らない。
10. **2026-11-09 `andon issue stop-check` は Chief Engineer の課題（profile `chief-engineer`）を止めない。** 人と端末で対話している間、報告も問いも書かずにターンを終えるのが正常（待つことが本業）。離席の人を待つときは今どおり台帳の問いを書く。
    理由（反論から）: 今は doing の課題に報告か問いを毎ターン求め、最大3回止める。この題でも問いを出すたびに途中経過の報告を書き直した。
    捨てた案: CE が毎ターン途中経過の報告を書く／触らない。
11. **2026-11-09 design.md を直す1本を切る案に入れる。** §1.4「ラインの入口は Chief Engineer」→「人の入口は Chief of Staff。何を作るかを詰めるときだけ Chief Engineer（起動するのは Chief of Staff。人が直接打ってもよい）」。§1.8 の表の起動のしかたを「Chief of Staff か人」、本文と図の「人が打つ」「依頼書は人の1行の題」「同時に2人いない」を 2・3 のとおりに。operate.md は既に「Chief of Staff か人」。
    捨てた案: ADR だけで持つ（設計の正と skill が食い違う）。

## やらないこと（範囲の外。理由）

- **新しい役・新しい機械の約束**（schema・`issue.yaml` の予約キー・通知の種類）。前提 1。差し戻しの回数も依頼書の追記で数える（6）。
- **`andon wiki` の command**。人の選択（7）。形が崩れたら戻す候補（開いている問い）。
- **skill の行数の上限**（8）。
- **P6 の中身**（信号の受け口・価値の仮説の形・観察の課題）。この題のあとの Chief Engineer の題。
- **worker skill の変更**。「範囲の外」を欄にしない形（5）にしたので触らない。
- **人の差し戻しや手直しの機械の記録**（P2 の `corrections.json`）。入ったら 6 の数え方を見直す。
- **振り返りのルーチン**（アウターループの実装）。wiki の ingest は Chief of Staff の手で始める（7）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は4つ。既存のものに足す: `tests/package.sh`（skill と template の文言）、`internal/chiefengineer/chiefengineer_test.go`、`internal/issue` の stop-check のテスト、セルでの実測（`package/` の変更の決まり）。

1. **`andon chief-engineer`（Go のテスト）**: `--brief -` で stdin の文が依頼書の既定の2行の下に入る。省けば今の依頼書。Chief Engineer のラベルの端末で Claude Code が動いていても起動する（「動いている」で止まらない）。作り直しで戻った端末は今どおり片付ける。
2. **`andon issue stop-check`（Go のテスト）**: profile が `chief-engineer` の doing の課題で、報告も問いも無いターンに `block` を返さない。ほかの profile は今のまま。
3. **CoS skill（`tests/package.sh`）**: 先頭に「人の入口」の節があり、「`andon chief-engineer` を勧める」の文が無い。打つコマンドの一覧に `andon chief-engineer` がある。§2 の欄の読み方の頭に原則の1行、「守ること」の説明に「範囲の外」。雛形の2行目は「解決すべき問題:」のまま（今の検査が green）。差し戻しの追記（`## 差し戻し（日付）`）と新しい課題に切る手順がある。wiki の節（何を・いつ・形・lint）がある。
4. **CoS の template（`tests/package.sh`）**: command に `--add-dir /workspace/wiki` がある。
5. **CE skill（`tests/package.sh`）**: 「1プロジェクトに1題」「前の題が終わらないまま」の行が無い。「読む」に価値の仮説の結果の1行、文書の形に「## 価値の仮説」の見出し、「言い直す」の段に「決断が無ければ文書を書かず `next` で返す」がある。
6. **design.md**: §1.4 と §1.8 が 11 のとおり（文言の grep）。
7. **セルで実測**（S6。報告に書く）: Chief of Staff が設計の依頼を受けて `andon chief-engineer --brief -` で CE を起動し、CE の最初のターンが依頼書の経緯を読んだと分かる。CE が問いを出しただけのターンで Stop フックが止めない。Chief of Staff が wiki に1件書いて commit・push できる（許可の確認が出ない）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

skill・command・template の変更は Andon のラインの変更（レベル C。人がマージ）。1本ずつ小さく。

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この ADR と `docs/decisions/` の1件（この課題の PR） | `bash tests/*.sh` green | — |
| S2 | command: `andon chief-engineer --brief -`・1人の検査を外す・`andon issue stop-check` の Chief Engineer の除外 ＋ Go のテスト ＋ `docs/operate.md` の該当行 | 受け入れ条件 1・2 | S1 のマージ |
| S3 | CoS skill（人の入口・打つコマンド・依頼書の原則と範囲の外・差し戻し・wiki の節）＋ template の `--add-dir` ＋ `tests/package.sh` | 受け入れ条件 3・4 | S2 のマージ（skill が `--brief -` を指すため） |
| S4 | CE skill（WIP の行を外す・読むの1行・価値の仮説の見出し・文書なしで返す段）＋ `tests/package.sh` | 受け入れ条件 5 | S1 のマージ（S2・S3 と並行可） |
| S5 | design.md §1.4・§1.8（表・本文・図） | 受け入れ条件 6 | S1 のマージ（S2〜S4 と並行可） |
| S6 | セルで実測（Chief of Staff が1題を起動し、CE の最初のターンと Stop フック、wiki の1件を確かめる） | 受け入れ条件 7。落ちたら直しの課題 | S2・S3 のマージとセルの版上げ |

## 開いている問い（残った問い。決断の種類つき）

- **差し戻しの回数を機械の記録にするか**（ラインの変更）。P2 の `corrections.json` が入ったら、依頼書の追記（6）からそちらへ移すかを見る。
- **Chief Engineer が複数のときの App の見え方**（ラインの変更）。Space `<slug>` の tab `CE <id> <題>` は課題ごとなので増えても壊れないはず。S6 の実測で確かめる。
- **wiki の手順を command に下ろす時期**（ラインの変更）。形が崩れる・index が痩せたままでいられない、と分かったら `andon wiki add` を戻す（design.md §1.9「文で試し、守られないと分かってから機械へ」）。
- **wiki が数百ページを超えたときの検索**（ラインの変更。Karpathy の gist の「中規模まで index で足りる」）。今は決めない。
- **価値の仮説の形**（事業・ラインの変更）。P6 の題。

## 出典（読んだもの・捨てた案の出所）

- 人の言葉（2026-11-09）: レクチャー §1.5・§1.6（「思想としては極力 CoS との会話だけにしたい…」「管理職には違いないんだけれども、優秀な参謀補佐官（CoS）に委譲…」「今はまだ Wiki 運用のアウターループがない」「仕事の進め方の整備も必要」）、指示文 P7、この対話（Q1〜Q10・代案・反論 5 件への答え）。
- Andon の側: [design.md](../design.md) §1.4・§1.8・§1.9・§1.10・§2.1・§2.3・§6 問い10・21、[ADR-0001](0001-skills-distribution.md)、[ADR-0004](0004-project-layout.md)（AGENTS.md を太らせない・出口の順）、[ADR-0005](0005-cell-repo.md)（wiki の形・書き手・人のファイルの照合）、[ADR-0006](0006-survey-fix-decide.md)（判断と決断の置き場）、`package/skills/{chief-of-staff,chief-engineer,worker,ask-one-by-one,glossary}/SKILL.md`、`package/templates/{chief-of-staff,chief-engineer}.md`、`package/settings.json`、`internal/chiefengineer/chiefengineer.go`、`internal/issue/{start,finish,stopcheck}.go`、`internal/spawn/spawn.go`、`tests/package.sh`、[operate.md](../operate.md)「役の配置と起動」、[dispatcher.md](../dispatcher.md)。
- 人の KB（private。mitaku/kb）: `wiki/andon-flow-design.md`（あるべきフロー。人の入口・2つの層・3つの戻り）、`wiki/sources/x-addyosmani-coding-agents-manager.md`（Osmani の要約と示唆）。
- Addy Osmani「Your coding agents made you a manager」（2026-11-08。https://x.com/addyosmani/status/2119332146485891501 ）: 「repo を読んでも分からないことだけを渡す」「2回直して駄目なら依頼を書き直して起動し直す」の出所（5・6）。
- Andrej Karpathy「LLM wiki」の gist（https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f ）: wiki の手順を指示文（skill）に持つ形・ingest／query／lint・index と log の出所（7。人が示した）。
- 反論: 別のコンテキストの読むだけのサブエージェント（2026-11-09）。証明された指摘 8 件のうち、同じ課題を起動し直せない・差し戻しが数えられない・CE が1人で止まる・Stop フック・欄を担当が読まない・原則と照合のぶつかり・直し漏れ、を 3・5・6・10 と 2 の直し漏れに反映。
