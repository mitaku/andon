package cli

import (
	"io"

	"andon/internal/board"
	"andon/internal/boot"
	"andon/internal/chiefengineer"
	"andon/internal/create"
	"andon/internal/delegate"
	"andon/internal/digest"
	"andon/internal/dispatcher"
	"andon/internal/gate"
	"andon/internal/issue"
	"andon/internal/measure"
	"andon/internal/open"
	"andon/internal/otel"
	"andon/internal/pkgversion"
	"andon/internal/plantinit"
	"andon/internal/process"
	"andon/internal/publish"
	"andon/internal/retro"
	"andon/internal/schema"
	"andon/internal/signal"
	"andon/internal/tools"
	"andon/internal/trace"
	"andon/internal/triage"
	"andon/internal/waiting"
)

// who は葉を打つ者（ADR-0053 決めたこと 8 の「打つ者」の欄）。役は Chief of Staff・Chief Engineer・担当・レビュアー、
// 機械は dispatcher・フック・テンプレート・compose・ホストの部品・検査・イメージを焼く道具。
type who int

const (
	owner who = iota
	role
	machine
)

// exit は 0・1・2 のほかの終了コードか、2 を別の意味で使う葉の終了コード（決めたこと 15）。
type exit struct {
	code  int
	means string
}

type runFunc func(args []string, stdin io.Reader, stdout, stderr io.Writer) int

// jsonForm は JSON を出せる葉の出し方（決めたこと 13）。どの形でも、--json を付ければ JSON を標準出力に出す。
type jsonForm int

const (
	noJSON     jsonForm = iota
	jsonAlways          // いつも JSON を出す。葉は --json を読まず、振り分けが外して渡す
	jsonFlag            // 葉が --json を読む。付けないと人向けに出す
	jsonDryRun          // 書く葉。葉が --dry-run と一緒の --json を読み、書くはずのことを JSON で出す
)

// leaf は葉の表の1行（決めたこと 11）。andon は表に載った葉だけを実行し、help・docs/commands.md・bash の補完もここから作る。
type leaf struct {
	name     string   // 「名詞 動詞」か1語
	who      []who    // 打つ者。主に打つ者を先に書く
	about    string   // 1行の説明
	usage    string   // andon <葉> --help の中身（葉のパッケージが持つ）
	examples []string // andon に続けて打つ例
	exits    []exit
	writes   bool     // 状態を変えるか
	project  bool     // プロジェクトを読むか（--project を受ける。決め方は project.go）
	ownFlag  bool     // --project を葉が自分で読む（振り分けは確かめるだけで、外さない）
	hook     bool     // フックが打つ（決まらなくても 2 を返さず葉に渡す。決めたこと 15）
	every    bool     // --project を省くと全部のプロジェクトを回る（dispatcher）
	json     jsonForm // JSON を出せるか、どう出すか（決めたこと 13）
	slugArg  bool     // 引数にプロジェクトの slug を取るか（補完が slug を補う）
	run      runFunc
}

// leaves は葉の表。並びは群ごとの help と docs/commands.md の並び。
var leaves = []leaf{
	// andon そのもの
	{name: "help", who: []who{owner, role, machine}, about: "この一覧を出す（--all で全部の葉、--all --markdown で docs/commands.md）",
		usage: helpUsage, examples: []string{"help --all"}},
	{name: "open", who: []who{owner, role}, about: "プロジェクトの Chief of Staff を開く（動いていれば、そのまま使う。slug を省くと今いるプロジェクト、決まらなければ fzf で選ぶ）",
		usage: open.Usage, examples: []string{"open sample-site", "open"},
		exits: []exit{{3, "起動したが最初のメッセージが届いていない"}}, writes: true, slugArg: true, run: open.Main},
	{name: "version", who: []who{machine}, about: "パッケージの置き場の commit と、中身がその commit と食い違うかを JSON で出す",
		usage: pkgversion.Usage, examples: []string{"version"}, json: jsonAlways, run: pkgversion.Main},
	{name: "boot", who: []who{machine, owner}, about: "パッケージからセルへ配布物を生成し、portless・files・sshd・dispatcher を起動する（毎起動）",
		usage: boot.Usage, examples: []string{"boot"}, writes: true, run: boot.Main},

	// オーナーが打つ
	{name: "design start", who: []who{owner, role}, about: "オーナーと仕様・設計を対話する Chief Engineer を、kanban の課題として起動する",
		usage: chiefengineer.Usage, examples: []string{`design start "返事の下書きボタン"`},
		exits: []exit{{3, "起動したが最初のメッセージが届いていない"}}, writes: true, project: true, run: chiefengineer.Main},
	{name: "project create", who: []who{owner}, about: "プロジェクトを立てる（projects/<slug>/ に kanban・project.json の下書き・AGENTS.md）",
		usage: create.Usage, examples: []string{"project create sample-site --repo github.com/<owner>/sample-site"}, writes: true, run: create.Main},
	{name: "project list", who: []who{owner}, about: "セルのプロジェクト（kanban のある projects/<slug>/）の slug を並べる（読むだけ）",
		usage: open.ListUsage, examples: []string{"project list"}, run: open.ListMain},
	{name: "waiting list", who: []who{owner, role, machine}, about: "オーナー待ち（マージの依頼・判断・手作業）と並行の枠を出す",
		usage: waiting.Usage, examples: []string{"waiting list", "waiting list --json"}, project: true, ownFlag: true, json: jsonFlag, run: waiting.Main},
	{name: "waiting answer", who: []who{owner, machine}, about: "ペインから判断の問いに選択肢で答える",
		usage: waiting.AnswerUsage, examples: []string{"waiting answer S-0012#1 --option 1"}, writes: true, project: true, ownFlag: true, run: waitingAnswer},
	{name: "delegation show", who: []who{owner, role}, about: "委譲（project.json の delegation）を読んで出す",
		usage: delegate.ShowUsage, examples: []string{"delegation show --json"}, project: true, ownFlag: true, json: jsonFlag, run: delegate.ShowMain},
	{name: "delegation grant", who: []who{owner}, about: "委譲を足す",
		usage: delegate.GrantUsage, examples: []string{"delegation grant merge B --evidence ADR-0041"}, writes: true, project: true, ownFlag: true, run: delegate.GrantMain},
	{name: "delegation narrow", who: []who{owner}, about: "委譲を狭める",
		usage: delegate.NarrowUsage, examples: []string{"delegation narrow merge A --evidence 戻す"}, writes: true, project: true, ownFlag: true, run: delegate.NarrowMain},
	{name: "delegation revoke", who: []who{owner}, about: "委譲を1件取り消す",
		usage: delegate.RevokeUsage, examples: []string{"delegation revoke d-3 --reason 期限"}, writes: true, project: true, ownFlag: true, run: delegate.RevokeMain},
	{name: "metrics show", who: []who{owner, role}, about: "kanban と記録からラインの流れ・効率・対抗の指標を数える",
		usage: measure.Usage, examples: []string{"metrics show --since 2026-11-01"}, project: true, json: jsonFlag, run: measure.Main},
	{name: "signal auth", who: []who{owner}, about: "外の信号の出所のトークンをセルの中で聞いて置く",
		usage: signal.AuthUsage, examples: []string{"signal auth sentry"}, writes: true, run: signal.AuthMain},
	{name: "digest auth", who: []who{owner}, about: "ダイジェストを送る Slack の bot のトークンをセルの中で聞いて置く",
		usage: digest.AuthUsage, examples: []string{"digest auth slack"}, writes: true, run: digest.AuthMain},
	{name: "cell init", who: []who{owner}, about: "セルの repo の雛形（<slug>-cell）を作る",
		usage: plantinit.Usage, examples: []string{"cell init sample"}, writes: true, run: plantinit.Main},

	// 役が打つ
	{name: "issue create", who: []who{role}, about: "担当の課題を足し、予約キーを書き、依頼書の Settings 欄を予約キーから書く",
		usage: issue.CreateUsage, examples: []string{`issue create "返事の下書きボタン" --repo github.com/<owner>/sample-site --brief brief.md review=off must_fix=major merge=person estimate=small`},
		writes: true, project: true, json: jsonFlag, run: issue.Create},
	{name: "issue start", who: []who{role, machine}, about: "worktree と branch を切り、担当を起動して kanban start で結ぶ",
		usage: issue.StartUsage, examples: []string{"issue start S-0012 --dry-run"},
		exits: []exit{{3, "起動したが最初のメッセージが届いていない"}}, writes: true, project: true, json: jsonDryRun, run: issue.Start},
	{name: "issue resume", who: []who{role, machine}, about: "止まった担当・Chief Engineer を同じ worktree でセッションの続きから起動する",
		usage: issue.ResumeUsage, examples: []string{"issue resume S-0012"}, writes: true, project: true, json: jsonDryRun, run: issue.Resume},
	{name: "issue finish", who: []who{role, machine}, about: "端末と worktree を片付け、origin に入った branch を消して done にする",
		usage: issue.FinishUsage, examples: []string{"issue finish S-0012"}, writes: true, project: true, run: issue.Finish},
	{name: "issue hold", who: []who{role}, about: "課題の PR を止める（理由の課題を書き、渡していれば印を黄にしてから返る）",
		usage: issue.HoldUsage, examples: []string{"issue hold S-0012 S-0015"}, writes: true, project: true, run: issue.Hold},
	{name: "issue unhold", who: []who{role}, about: "課題の PR の止めを外す",
		usage: issue.UnholdUsage, examples: []string{"issue unhold S-0012"}, writes: true, project: true, run: issue.Unhold},
	{name: "issue impact", who: []who{role, machine}, about: "マージした PR が、動いている課題にぶつかる・重なるかを JSON で出す",
		usage: issue.ImpactUsage, examples: []string{"issue impact S-0012"},
		exits: []exit{{4, "PR がまだ MERGED でない"}}, project: true, json: jsonAlways, run: issue.Impact},
	{name: "gate", who: []who{role}, about: "課題の PR を予約キー・findings.json・条件表・委譲で照合し、記録を書く（マージはしない）",
		usage: gate.Usage, examples: []string{"gate S-0012 --dry-run"},
		exits:  []exit{{1, "fix（担当へ戻す）"}, {3, "道具か kanban の誤り"}, {4, "needs_human（オーナーが見る）"}, {5, "wait（記録しない）"}},
		writes: true, project: true, json: jsonAlways, run: gate.Main},
	{name: "issue show", who: []who{role}, about: "予約キー・report.json・レビューの照合を JSON で出す",
		usage: issue.ShowUsage, examples: []string{"issue show S-0012"}, project: true, json: jsonAlways, run: issue.Show},
	{name: "issue trace", who: []who{owner, role}, about: "課題 1 件の流れ（段・起動・プロンプト・モデルの呼び出しとツール）を 1 本の木にして出す（読むだけ）",
		usage: trace.Usage, examples: []string{"issue trace S-0012", "issue trace S-0012 --otlp"}, project: true, json: jsonFlag, run: issueTrace},
	{name: "issue report", who: []who{role}, about: "report.json を書き、標準入力の Markdown で kanban に報告する",
		usage: issue.ReportUsage, examples: []string{"issue report S-0012 --pr https://github.com/<owner>/sample-site/pull/7 < report.md"},
		writes: true, project: true, run: issue.Report},
	{name: "issue review-request", who: []who{role}, about: "report.json に review_request を足して、レビューを頼む",
		usage: issue.ReviewRequestUsage, examples: []string{"issue review-request S-0012 < /dev/null"}, writes: true, project: true, run: issue.ReviewRequest},
	{name: "issue candidates", who: []who{role}, about: "候補の一覧の JSON を検査し、library/candidates.json に置く",
		usage: issue.CandidatesUsage, examples: []string{"issue candidates S-0012 -"}, writes: true, project: true, run: issue.Candidates},
	{name: "issue ask", who: []who{role}, about: "問いの JSON を検査し、kanban に問う",
		usage: issue.AskUsage, examples: []string{"issue ask S-0012 -"}, writes: true, project: true, run: issue.Ask},
	{name: "issue record", who: []who{role}, about: "worktree で command を走らせ、出力と終了コードを証拠として残す",
		usage: issue.RecordUsage, examples: []string{"issue record S-0012 -- go test ./..."}, writes: true, project: true, run: issue.Record},
	{name: "issue ready", who: []who{role}, about: "HEAD・report.json・PR・レビューが揃っていれば kanban を review にする",
		usage: issue.ReadyUsage, examples: []string{"issue ready S-0012"}, writes: true, project: true, run: issue.Ready},
	{name: "news ack", who: []who{role}, about: "届いた <news> にオーナーへ言うことが無いと示す",
		usage: dispatcher.NewsUsage, examples: []string{"news ack"}, writes: true, run: dispatcher.NewsAck},
	{name: "publish", who: []who{role}, about: "生成したファイルを files/ へ公開し、URL を返す",
		usage: publish.Usage, examples: []string{"publish report.html sample-site"}, writes: true, run: publish.Main},
	{name: "triage collect", who: []who{role}, about: "kanban の溜まったものと repo ごとの最新の main を JSON に集める",
		usage: triage.CollectUsage, examples: []string{"triage collect --limit 50"},
		exits: []exit{{4, "項目が上限を超えた"}}, project: true, json: jsonAlways, run: triage.Collect},
	{name: "triage apply", who: []who{role}, about: "仕分けの決定（decisions.json）を kanban に当てる",
		usage: triage.ApplyUsage, examples: []string{"triage apply S-0030 --dry-run"}, writes: true, project: true, run: triage.Apply},
	{name: "triage verdict", who: []who{role}, about: "判断の閉じと課題を閉じる判定を1件ずつ「閉じてよい」か「覆す」と書く",
		usage: triage.VerdictUsage, examples: []string{"triage verdict S-0030 S-0012 ok"}, writes: true, project: true, run: triage.Verdict},
	{name: "retro collect", who: []who{role}, about: "振り返りの入力を JSON に集める",
		usage: retro.CollectUsage, examples: []string{"retro collect --days 7"}, project: true, json: jsonAlways, run: retro.CollectMain},
	{name: "retro patterns", who: []who{role}, about: "パターンの一覧を検査して library/retro.json に書く",
		usage: retro.PatternsUsage, examples: []string{"retro patterns S-0040 -"}, writes: true, project: true, run: retro.PatternsMain},
	{name: "retro apply", who: []who{role}, about: "パターンの一覧の fix を直す課題に切る",
		usage: retro.ApplyUsage, examples: []string{"retro apply S-0040 --dry-run"}, writes: true, project: true, run: retro.ApplyMain},
	{name: "digest collect", who: []who{role}, about: "期間の記録を library/digest/input.json に集める",
		usage: digest.CollectUsage, examples: []string{"digest collect S-0050"}, writes: true, project: true, run: digest.CollectMain},
	{name: "digest build", who: []who{role}, about: "担当が書いた JSON を照らし、1 枚の HTML とメッセージを書く",
		usage: digest.BuildUsage, examples: []string{"digest build S-0050 -"}, writes: true, project: true, run: digest.BuildMain},
	{name: "digest send", who: []who{role}, about: "ダイジェストを files/ と送り先へ出す",
		usage: digest.SendUsage, examples: []string{"digest send S-0050"}, writes: true, project: true, run: digest.SendMain},
	{name: "digest check", who: []who{role, owner}, about: "送り先へ確かめの小さなファイルを送る",
		usage: digest.CheckUsage, examples: []string{"digest check"}, writes: true, project: true, run: digest.CheckMain},

	// 機械が打つ
	{name: "issue merge", who: []who{machine}, about: "review の課題に gate を回し、委譲の範囲内ならマージして片付ける",
		usage: issue.MergeUsage, examples: []string{"issue merge S-0012"}, writes: true, project: true, json: jsonAlways, run: issue.Merge},
	{name: "issue settle", who: []who{machine, role}, about: "オーナーがマージした PR の課題の、gate の問いを閉じて片付ける",
		usage: issue.SettleUsage, examples: []string{"issue settle S-0012"}, writes: true, project: true, json: jsonAlways, run: issue.Settle},
	{name: "issue board", who: []who{machine, owner}, about: "ボードが読む課題だけ（開いている課題・片付いたばかりの課題とその相手）を出す（読むだけ）",
		usage: board.BoardUsage, examples: []string{"issue board --json --since 2026-11-22T13:00:00Z"}, project: true, ownFlag: true, json: jsonFlag, run: board.BoardMain},
	{name: "issue history", who: []who{machine, owner}, about: "片付いた課題を新しい順に件数まで出す（ボードの History が読む。読むだけ）",
		usage: board.HistoryUsage, examples: []string{"issue history --json --limit 200 --match 下書き"}, project: true, ownFlag: true, json: jsonFlag, run: board.HistoryMain},
	{name: "issue session", who: []who{machine}, about: "自分の端末のラベル session を出す（テンプレートが読む）",
		usage: issue.SessionUsage, examples: []string{"issue session"}, run: issue.Session},
	{name: "issue stop-check", who: []who{machine}, about: "担当・レビュアーの Stop フック: doing で報告も問いも無ければ止める",
		usage: issue.StopCheckUsage, examples: []string{"issue stop-check S-0012 < hook.json"}, project: true, hook: true, run: issue.StopCheck},
	{name: "dispatcher", who: []who{machine}, about: "kanban を回り、レビュアーの起動・gate・merge: auto のマージと片付けを回す（常駐）",
		usage: dispatcher.Usage, examples: []string{"dispatcher --once"}, writes: true, project: true, ownFlag: true, every: true, run: dispatcher.Main},
	{name: "dispatcher drain", who: []who{machine}, about: "Chief of Staff のフック: 待ちの通知を届ける",
		usage: dispatcher.DrainUsage, examples: []string{"dispatcher drain < hook.json"}, writes: true, project: true, ownFlag: true, hook: true, run: dispatcher.Drain},
	{name: "dispatcher wait", who: []who{machine}, about: "待ち役のフック: 離席中に合図が来るまで待つ",
		usage: dispatcher.WaitUsage, examples: []string{"dispatcher wait --timeout 600"}, project: true, ownFlag: true, hook: true, run: dispatcherWait},
	{name: "dispatcher rewake", who: []who{machine}, about: "待ち役のフック: 合図で Chief of Staff を起動し直す",
		usage: dispatcher.RewakeUsage, examples: []string{"dispatcher rewake < hook.json"}, project: true, ownFlag: true, hook: true, run: dispatcherRewake},
	{name: "signal collect", who: []who{machine}, about: "project.json の signals の出所を読み、信号の記録を書く",
		usage: signal.CollectUsage, examples: []string{"signal collect --due"}, writes: true, project: true, ownFlag: true, run: signal.CollectMain},
	{name: "signal delivered", who: []who{machine}, about: "通知 signal で届けた出来事を記録に書く",
		usage: signal.DeliveredUsage, examples: []string{"signal delivered <fingerprint> <episode>"}, writes: true, project: true, ownFlag: true, run: signal.DeliveredMain},
	{name: "boot serve-files", who: []who{machine}, about: "files サーバ（boot が自分で起動する）",
		usage: boot.ServeFilesUsage, examples: []string{"boot serve-files /workspace/files 8080"}, run: boot.ServeFilesMain},
	{name: "otel serve", who: []who{machine}, about: "ハーネスのトレースとログを 127.0.0.1:4318 で受け、課題ごとのファイルに書く",
		usage: otel.ServeUsage, examples: []string{"otel serve"}, writes: true, run: otel.ServeMain},
	{name: "otel env", who: []who{machine}, about: "役の起動の行に、Claude Code か Codex が受け口へトレースとログを送る設定を出す（template が読む）",
		usage: otel.EnvUsage, examples: []string{"otel env", "otel env --codex"}, run: otel.EnvMain},
	{name: "tools env", who: []who{machine}, about: "役の起動の行に、端末の課題の worktree の mise.toml をいちばん弱い既定として渡す設定を出す（template が読む）",
		usage: tools.EnvUsage, examples: []string{"tools env"}, run: tools.EnvMain},
	{name: "version stamp", who: []who{machine}, about: "焼くときに置き場の中身を数えて version.json を書く",
		usage: pkgversion.StampUsage, examples: []string{"version stamp unknown"}, writes: true, run: pkgversion.StampMain},
	{name: "schema check", who: []who{machine, role}, about: "JSON を約束の schema と照らす",
		usage: schema.CheckUsage, examples: []string{"schema check report library/report.json"},
		exits: []exit{{1, "合わない（理由は標準出力）"}, {2, "検査できない（使い方の誤りも）"}}, run: schema.CheckMain},
	{name: "schema batch", who: []who{machine, role}, about: "1行1件の表を、検査器を1度読むだけでまとめて照らす",
		usage: schema.BatchUsage, examples: []string{"schema batch cases.tsv"}, run: schema.BatchMain},
	{name: "process render", who: []who{machine}, about: "process/ の YAML から docs/process/<名>.md を作る",
		usage: process.RenderUsage, examples: []string{"process render --check"}, writes: true, run: process.RenderMain},
	{name: "process json", who: []who{machine}, about: "YAML の定義を JSON にして出す",
		usage: process.JSONUsage, examples: []string{"process json process/states/issue.yaml"}, json: jsonAlways, run: process.JSONMain},
}

// renames は名前を変える葉の、今の名前と新しい名前（ADR-0053 決めたこと 8 の 18 個のうち、まだ変えていないもの。dispatcher と
// dispatcher --once は1行）。名前を変えるまでの間は、葉の形の決まり（2語・1語の4つ・1語の葉を名詞にしない）から外れていても、
// ここにある葉だけは表に置いてよい。変えたら retired へ移す。
var renames = map[string]string{
	"gate":             "issue gate",
	"issue ready":      "issue submit",
	"publish":          "file publish",
	"dispatcher":       "dispatcher run",
	"signal delivered": "signal mark-delivered",
	"boot serve-files": "cell serve-files",
	"version stamp":    "package stamp",
}

// retired は名前を変えた葉の、古い名前と新しい名前（決めたこと 17）。古い名前は実行せず、新しい名前を1行で言って終了コード 2 で断る。
// 別名ではない。消す時期は決めない。
var retired = map[string]string{
	"chief-of-staff":  "open",
	"chief-engineer":  "design start",
	"create":          "project create",
	"waiting":         "waiting list",
	"delegate show":   "delegation show",
	"delegate grant":  "delegation grant",
	"delegate narrow": "delegation narrow",
	"delegate revoke": "delegation revoke",
	"measure":         "metrics show",
	"init":            "cell init",
}

// oneWord は1語で打てる葉（決めたこと 5）。
var oneWord = map[string]bool{"open": true, "help": true, "version": true, "boot": true}

// featured は andon help の先頭に例を出す、オーナーがよく打つ葉（決めたこと 12）。
var featured = []string{"open", "waiting list", "design start"}

// issueTrace は andon issue trace。dispatcher の控えのパスをここで渡す（trace が dispatcher に依らないように）。
func issueTrace(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return trace.Main(args, stdout, stderr, dispatcher.StatePath)
}

func waitingAnswer(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return waiting.Answer(args, stdout, stderr)
}

// フックが打つ wait と rewake は、終了コード 2 が Claude Code への合図なので、使い方の誤りでも 0 を返す（決めたこと 15）。
func dispatcherWait(args []string, stdin io.Reader, _, stderr io.Writer) int {
	return dispatcher.WaitMain(args, stdin, stderr)
}

func dispatcherRewake(args []string, stdin io.Reader, _, stderr io.Writer) int {
	return dispatcher.RewakeMain(args, stdin, stderr)
}
