# ADR（Andon の設計の決定）

Chief Engineer がテーマごとに書いた、Andon 自身の設計の決定の記録。書き方の決まりは [design.md](../design.md) §1.8 にある。

- 1 本 1 ファイルで、名前は `NNNN-<英字の短い名前>.md`。NNNN は 4 桁の通し番号で、やめた検討の番号は詰めない。
- 1 行目の下の状態行が、決定・提案・覆された のどれかを言う。その下の「決定の索引」で、決めたことごとに今の正の置き場と、今も生きているかを読む。
- 決定は上書きしない。構造が変わる覆しは新しい ADR に書き、小さな改めは同じ ADR の「決めたこと」に下から足す。
- 下の一覧は、バージョンを切るときに作り直す写しで、手で直さない。今の一覧は `bash tests/adr-index.sh --list` で出せる。

## 一覧

CHANGELOG の一番上が v0.0.14 のときに作り直した 55 本。それより新しい ADR と変わった状態は `bash tests/adr-index.sh --list` で出せる。

- [0001 skill の配布](0001-skills-distribution.md): 決定
- [0002 Herdr App の Space と tab への役の置き方](0002-herdr-spaces.md): 決定
- [0003 作業側の Space をタスクごとにする](0003-task-spaces.md): 決定
- [0004 `projects/<slug>/` の構成](0004-project-layout.md): 決定
- [0005 建屋（セル）の repo の直下と、追跡・持ち運び・守り](0005-cell-repo.md): 決定
- [0006 調べる・決める・直すを分ける（工程の背骨・タスクの型・候補の一覧・横展開・範囲が食い違ったときの止まり方）](0006-survey-fix-decide.md): 決定
- [0007 仮設の工場にあって Andon に無いものの棚卸しと、塞ぐ順](0007-interim-gap-inventory.md): 決定
- [0008 kanban のカードを「課題（issue）」に改名する（語も識別子も改める・課題 ⊃ 作業〈task〉・形式 v3〈手で移す〉・依頼書の欄を「解決すべき問題」に）](0008-issue-naming.md): 決定
- [0009 Chief of Staff をオーナーの既定の入口にする（CoS と Chief Engineer の skill）](0009-cos-default-entry.md): 決定
- [0010 通知を「記録だけ」と「届ける」に分け、`<no-reply>` を `andon news ack` に替える](0010-news-ack.md): 決定
- [0011 信号の受け口と価値のループ](0011-signals-and-value-loop.md): 決定
- [0012 止まった担当を resume で起動し直す・worktree の置き場](0012-resume-and-worktree-home.md): 決定
- [0013 役の呼び名を短くし、オーナーに向けた日本語をふだんの言い方に揃える](0013-short-role-names-and-plain-japanese.md): 決定
- [0014 課題の見込みと実際のズレを測り、Chief of Staff に届ける](0014-issue-estimate.md): 決定
- [0015 inbox を Needs you と役への入口に絞る](0015-inbox-needs-you.md): 決定
- [0016 セルの wiki を LLM Wiki の手法に乗せる](0016-cell-wiki-llm-wiki.md): 決定
- [0017 仕事の流れと状態を定義し、その上で見張り役（dispatcher）を細くする](0017-process-definitions.md): 決定
- [0018 担当が画面を見て確かめる道具](0018-screen-check-tooling.md): 決定
- [0019 オーナーの書きかけに `<news>` を混ぜない](0019-cos-input-box.md): 決定
- [0020 auto memory を気づきの受け皿にして wiki へ蒸留する](0020-auto-memory-as-intake.md): 決定
- [0021 pstack をセルに入れて試す](0021-pstack-in-cell.md): 決定
- [0022 Chief Engineer の役は残し、Chief of Staff に Advisor（Fable）を試験運用で付ける](0022-advisor-and-chief-engineer.md): 決定
- [0023 サイドバーに「あんどん」らしさを見た目で出す](0023-sidebar-andon-look.md): 決定
- [0024 担当が課題ごとに日報（過程・感想・改善の思いつき）を書く](0024-worker-journal.md): 決定
- [0025 課題の一生の規則とモデルを repo に置き、機械が確かめ続ける](0025-formal-model-in-repo.md): 決定
- [0026 PR と repo に残る文は、セルの外の読み手に通じるように書く](0026-pr-rules-for-outside-readers.md): 決定
- [0027 開発中の Andon のパッケージを、イメージを焼き直さずにセルへ当てる](0027-apply-package-without-rebuild.md): 決定
- [0028 実行の記録から skill とプロンプトを直し続けるループ（記録・振り返り・改善の PR）](0028-improvement-loop.md): 決定
- [0029 工場の2つの面を言い分ける（セルは場所、Andon はしくみ）](0029-factory-place-and-mechanism.md): 決定
- [0030 トリアージが誤って閉じた項目を拾う網](0030-triage-close-safety-net.md): 決定
- [0031 Chief of Staff をセルに1人にする](0031-cell-chief-of-staff.md): 提案
- [0032 書き終えてから main に入るまでの時間を縮める](0032-shorten-time-to-merge.md): 決定
- [0033 ADR の棚卸しと記述の整理](0033-adr-inventory-and-slimming.md): 決定
- [0034 検査の仕分けから、テストより手前に足す道具と直す設計を決める](0034-prevention-before-tests.md): 決定
- [0035 pstack の correct と create-verification-skill を Andon の開発サイクルに取り入れるか](0035-pstack-correct-and-verification-skill.md): 決定
- [0036 役の名前を「人」から「オーナー」に改める](0036-owner-not-hito.md): 決定
- [0037 Chief Engineer が自分で決める範囲を広げる](0037-chief-engineer-autonomy.md): 決定
- [0038 Chief Engineer が設計の検討で statechart のモデルを使い、開発と形式検証へつなげる](0038-statechart-in-chief-engineer-design.md): 決定
- [0039 振り返りの直す課題の上限を外し、改善のペースを流量に合わせる](0039-retro-scale-with-flow.md): 決定
- [0040 「台帳」「ゲート」「版」をやめ、kanban・gate・バージョンと書く](0040-kanban-gate-version-words.md): 決定
- [0041 委譲の上限を andon コマンドから外し、オーナーが前もって任せた PR をオーナーを待たずにマージする](0041-delegation-cap-out-of-command.md): 決定
- [0042 オーナー待ちが、待っていない仕事を止めないようにする](0042-owner-waits-do-not-stop-the-line.md): 決定
- [0043 行燈は、オーナーが呼ばれているときだけ灯す](0043-lamp-lights-on-owner-waits.md): 決定
- [0044 オーナーが読むダイジェストを定期に作り、Slack へ送る](0044-owner-digest.md): 決定
- [0045 行燈を Stream Deck のキーに出す](0045-stream-deck-lamp.md): 決定
- [0046 README を、初めて来た読み手が README だけで判断できる形に組み直す](0046-readme-for-first-time-readers.md): 決定
- [0047 枠が空いたままのとき、トリアージと振り返りと点検を回す](0047-idle-slots-run-upkeep.md): 決定
- [0048 セルの入口 `bin/bootstrap` を薄くし、本体を image から取り出す](0048-thin-cell-entry.md): 決定
- [0049 オーナーの介在回数を、操作の種類で数える](0049-owner-touches-by-kind.md): 決定
- [0050 重い command を外のマシンで回し、定期の仕事を API キーで対話なしに動かす](0050-external-compute.md): 提案
- [0051 外の信号の受け口を実際に流し、main の失敗を壊した変更に結ぶ](0051-signals-flow-and-breaking-change.md): 決定
- [0052 「題」をやめ、タイトル・テーマと書く](0052-title-and-theme-not-dai.md): 決定
- [0053 andon のコマンドを「名詞 動詞」に組み直し、どこで打っても動くようにする](0053-command-system.md): 決定
- [0054 gate は、オーナーのマージ待ちの PR に赤い印を付けない](0054-gate-status-by-decision.md): 決定
- [0055 ADR を並べて書いても PR が衝突しない作りにする](0055-adr-prs-without-conflicts.md): 決定
