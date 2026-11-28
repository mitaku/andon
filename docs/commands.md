# andon のコマンド

andon の葉（実際に打てる1つのコマンド）の一覧。`andon help --all --markdown` が葉の表から作るので、手で直さない。
使い方・フラグ・終了コードは、セルの中で `andon <名詞> <動詞> --help` を打って読む。

## オーナーが打つ

| コマンド | 打つ者 | 説明 |
|---|---|---|
| `andon help` | オーナー・役・機械 | この一覧を出す（--all で全部の葉、--all --markdown で docs/commands.md） |
| `andon open` | オーナー・役 | プロジェクトの Chief of Staff を開く（動いていれば、そのまま使う。slug を省くと今いるプロジェクト、決まらなければ fzf で選ぶ） |
| `andon design start` | オーナー・役 | オーナーと仕様・設計を対話する Chief Engineer を、kanban の課題として起動する |
| `andon project create` | オーナー | プロジェクトを立てる（projects/&lt;slug&gt;/ に kanban・project.json の下書き・AGENTS.md） |
| `andon project list` | オーナー | セルのプロジェクト（kanban のある projects/&lt;slug&gt;/）の slug を並べる（読むだけ） |
| `andon waiting list` | オーナー・役・機械 | オーナー待ち（マージの依頼・判断・手作業）と並行の枠を出す |
| `andon waiting answer` | オーナー・機械 | ペインから判断の問いに選択肢で答える |
| `andon delegation show` | オーナー・役 | 委譲（project.json の delegation）を読んで出す |
| `andon delegation grant` | オーナー | 委譲を足す |
| `andon delegation narrow` | オーナー | 委譲を狭める |
| `andon delegation revoke` | オーナー | 委譲を1件取り消す |
| `andon metrics show` | オーナー・役 | kanban と記録からラインの流れ・効率・対抗の指標を数える |
| `andon signal auth` | オーナー | 外の信号の出所のトークンをセルの中で聞いて置く |
| `andon digest auth` | オーナー | ダイジェストを送る Slack の bot のトークンをセルの中で聞いて置く |
| `andon tray auth` | オーナー | Cloudflare の tray の API キーをセルの中で聞いて置く |
| `andon tray status` | オーナー・役 | ローカルの tray と Cloudflare の tray の URL・到達・バージョン・直近 7 日の枚数を出す |
| `andon tray deploy` | オーナー | Cloudflare の tray をオーナーのアカウントへ出す（セルの外で bin/bootstrap tray-deploy から打つ） |
| `andon cell init` | オーナー | セルの repo の雛形（&lt;slug&gt;-cell）を作る |
| `andon issue trace` | オーナー・役 | 課題 1 件の流れ（段・起動・プロンプト・モデルの呼び出しとツール）を 1 本の木にして出す（読むだけ） |

## 役が打つ

| コマンド | 打つ者 | 説明 |
|---|---|---|
| `andon issue create` | 役 | 担当の課題を足し、予約キーを書き、依頼書の Settings 欄を予約キーから書く |
| `andon issue start` | 役・機械 | worktree と branch を切り、担当を起動して kanban start で結ぶ |
| `andon issue resume` | 役・機械 | 止まった担当・Chief Engineer を同じ worktree でセッションの続きから起動する |
| `andon issue finish` | 役・機械 | 端末と worktree を片付け、origin に入った branch を消して done にする |
| `andon issue hold` | 役 | 課題の PR を止める（理由の課題を書き、渡していれば印を黄にしてから返る） |
| `andon issue unhold` | 役 | 課題の PR の止めを外す |
| `andon issue impact` | 役・機械 | マージした PR が、動いている課題にぶつかる・重なるかを JSON で出す |
| `andon gate` | 役 | 課題の PR を予約キー・findings.json・条件表・委譲で照合し、記録を書く（マージはしない） |
| `andon issue show` | 役 | 予約キー・report.json・レビューの照合を JSON で出す |
| `andon issue report` | 役 | report.json を書き、標準入力の Markdown で kanban に報告する |
| `andon issue review-request` | 役 | report.json に review_request を足して、レビューを頼む |
| `andon issue candidates` | 役 | 候補の一覧の JSON を検査し、library/candidates.json に置く |
| `andon issue ask` | 役 | 問いの JSON を検査し、kanban に問う |
| `andon issue record` | 役 | worktree で command を走らせ、出力と終了コードを証拠として残す |
| `andon issue ready` | 役 | HEAD・report.json・PR・レビューが揃っていれば kanban を review にする |
| `andon issue restack` | 役 | 積んだ課題を、前の課題が main に入ったあと既定の branch の先へ載せ替える |
| `andon news ack` | 役 | 届いた &lt;news&gt; にオーナーへ言うことが無いと示す |
| `andon publish` | 役 | 生成したファイルを tray へ出し、オーナーに渡す URL を1行返す |
| `andon triage collect` | 役 | kanban の溜まったものと repo ごとの最新の main を JSON に集める |
| `andon triage apply` | 役 | 仕分けの決定（decisions.json）を kanban に当てる |
| `andon triage verdict` | 役 | 判断の閉じと課題を閉じる判定を1件ずつ「閉じてよい」か「覆す」と書く |
| `andon retro collect` | 役 | 振り返りの入力を JSON に集める |
| `andon retro patterns` | 役 | パターンの一覧を検査して library/retro.json に書く |
| `andon retro apply` | 役 | パターンの一覧の fix を直す課題に切る |
| `andon digest collect` | 役 | 期間の記録を library/digest/input.json に集める |
| `andon digest build` | 役 | 担当が書いた JSON を照らし、1 枚の HTML とメッセージを書く |
| `andon digest send` | 役 | ダイジェストを tray と送り先へ出す |
| `andon digest check` | 役・オーナー | 送り先へ確かめの小さなファイルを送る |

## 機械が打つ

| コマンド | 打つ者 | 説明 |
|---|---|---|
| `andon version` | 機械 | パッケージの置き場の commit と、中身がその commit と食い違うかを JSON で出す |
| `andon boot` | 機械・オーナー | パッケージからセルへ配布物を生成し、portless・ローカルの tray・sshd・dispatcher を起動する（毎起動） |
| `andon issue merge` | 機械 | review の課題に gate を回し、委譲の範囲内ならマージして片付ける |
| `andon issue settle` | 機械・役 | オーナーがマージした PR の課題の、gate の問いを閉じて片付ける |
| `andon issue board` | 機械・オーナー | ボードが読む課題だけ（開いている課題・片付いたばかりの課題とその相手）を出す（読むだけ） |
| `andon issue history` | 機械・オーナー | 片付いた課題を新しい順に件数まで出す（ボードの History が読む。読むだけ） |
| `andon issue session` | 機械 | 自分の端末のラベル session を出す（テンプレートが読む） |
| `andon issue stop-check` | 機械 | 担当・レビュアーの Stop フック: doing で報告も問いも無ければ止める |
| `andon dispatcher run` | 機械 | kanban を回り、レビュアーの起動・gate・merge: auto のマージと片付けを回す（常駐） |
| `andon dispatcher drain` | 機械 | Chief of Staff のフック: 待ちの通知を届ける |
| `andon dispatcher wait` | 機械 | 待ち役のフック: 離席中に合図が来るまで待つ |
| `andon dispatcher rewake` | 機械 | 待ち役のフック: 合図で Chief of Staff を起動し直す |
| `andon signal collect` | 機械 | project.json の signals の出所を読み、信号の記録を書く |
| `andon signal mark-delivered` | 機械 | 通知 signal で届けた出来事を記録に書く |
| `andon tray serve` | 機械 | ローカルの tray を動かす常駐（boot が起動する） |
| `andon otel serve` | 機械 | ハーネスのトレースとログを 127.0.0.1:4318 で受け、課題ごとのファイルに書く |
| `andon otel env` | 機械 | 役の起動の行に、Claude Code か Codex が受け口へトレースとログを送る設定を出す（template が読む） |
| `andon tools env` | 機械 | 役の起動の行に、端末の課題の worktree の mise.toml をいちばん弱い既定として渡す設定を出す（template が読む） |
| `andon package stamp` | 機械 | 焼くときに置き場の中身を数えて version.json を書く |
| `andon schema check` | 機械・役 | JSON を約束の schema と照らす |
| `andon schema batch` | 機械・役 | 1行1件の表を、検査器を1度読むだけでまとめて照らす |
| `andon process render` | 機械 | process/ の YAML から docs/process/&lt;名&gt;.md を作る |
| `andon process json` | 機械 | YAML の定義を JSON にして出す |
