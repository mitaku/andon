---- MODULE IssueLife ----
\* 課題 1 つの一生（kanban に入ってから片付くまで）と、それに関わる役の動き（行動）。規則は IssueRules.tla に分けてある。
\* 行動ごとに、写したコードの場所を書く（repo からの相対パスと関数）。課題の一生の動きを変えたら、同じ PR でここを直す。
\* 課題は 1 つ・PR は 1 つ・レビューの回は区別しない。commit は「いまの head」と「それより古いもの」の 2 つに分けて見る。
\* 省いたものは README.md の「モデルが省いたもの」。

VARIABLES
    status,     \* kanban の status: todo・doing・review・done
    worker,     \* 担当の端末: none（まだ起こしていない）・live・gone（落ちた・片付けで消した）
    wt,         \* 担当の worktree: none・here・removed（finish が消した）
    pr,         \* PR: none・open・closed（マージせずに閉じた。開き直されうる）・merged
    reviewing,  \* 動いているレビュアーが見ている commit の集まり: "head"（いまの head）・"old"（head より古い commit）
    ci,         \* いまの head の CI（GitHub の checks）: none（PR がまだ無い）・running・failed・passed
    result,     \* 照合が読むレビュー（結果を出したレビュアーのうち最後に終わったもの。internal/ledger/ledger.go reviewers）:
                \* none・old（head より古い commit を見た。stale）・clean（head を見て Must fix 以上が無い）・mustfix（head を見て Must fix 以上がある）
    headSeen,   \* いまの head を見たレビューの結果がある（どれが最後かに関わらず。review-request はこれがあると断る）
    dirty,      \* 担当に直すことがある（head のレビューの Must fix・gate の fix で届いた直し）。CI が落ちたことは ci で表す
    gate,       \* いまの review の行に対する gate の答え: none・approve・fix・human
    fixSent,    \* gate の fix が担当の端末に届いた（hc send が通った）
    told,       \* いまの gate の答えが Chief of Staff に届いた（<news> か問い）
    q,          \* 担当の問いが受け取られていない（kanban received の前）
    cancel,     \* オーナーがやめると決め、Chief of Staff の finish が通った
    mergeAuto,  \* issue.yaml の merge: auto で、委譲の範囲内。Chief of Staff とオーナーが変えうる（MergeSettingChanges）
    moved,      \* gate が approve か needs_human を決めたあとに、PR の head が動いた（決めた head はもういまの head でない）
    autoMerged, \* dispatcher が自分でマージした（オーナーのマージでない。規則 R8 が見る）
    \* ---- ペインから判断の問いに答える口（ADR-0042 決めたこと 17〜19） ----
    ans,        \* いまの問いの答えを書いたのは: none（まだ open）・cos（Chief of Staff が会話で受けて kanban answer）・pane（オーナーがペインから andon waiting answer）
    news,       \* ペインの答えの通知 answered: none・pending（dispatcher が積んだ）・read（Chief of Staff が読んだ）
    sent,       \* 答えが担当の端末に届いた（Chief of Staff の hc send が通った）
    twice,      \* いまの問いに答えが 2 度書かれた（規則 R10 が見る）
    \* ---- gate の印と止め（ADR-0056 決めたこと 3・5・7〜10・17） ----
    mark,       \* いまの head の印 andon/gate: none（書かれていない）・ok（success）・red（failure）・held（pending。問いで止めている）
    hold,       \* Chief of Staff がこの課題の PR を止めている（issue.yaml の予約キー hold。andon issue hold が書き、andon issue unhold が消す）
    required,   \* repo の ruleset が andon/gate を必須にしている（オーナーが設定する。途中で変わらない）
    rel,        \* gate がいまの head を approve か needs_human と判定した（マージを渡した）
    badMerge    \* 渡していない head か、止めている間にマージされた（規則 R12 が見る）

vars == <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, news, sent, twice, mark, hold, required, rel, badMerge>>
qVars == <<ans, news, sent, twice>>
\* commit とレビューの変数（担当の push・CI・レビュアーが動かす）
reviewVars == <<reviewing, ci, result, headSeen, dirty>>

Init ==
    /\ status = "todo" /\ worker = "none" /\ wt = "none" /\ pr = "none"
    /\ reviewing = {} /\ ci = "none" /\ result = "none" /\ headSeen = FALSE /\ dirty = FALSE
    /\ gate = "none" /\ fixSent = FALSE /\ told = FALSE
    /\ q = FALSE /\ cancel = FALSE /\ mergeAuto \in BOOLEAN
    /\ moved = FALSE /\ autoMerged = FALSE
    /\ ans = "none" /\ news = "none" /\ sent = FALSE /\ twice = FALSE
    /\ mark = "none" /\ hold = FALSE /\ required \in BOOLEAN /\ rel = FALSE /\ badMerge = FALSE

\* andon issue finish（internal/issue/finish.go finish）: 何かを消す前に closable で受け取られていない問いを確かめ、
\* あれば何もしない（片付けは全部か無し）。無ければ端末を消し（removeTerminals）、worktree を消し、最後に kanban done。
\* 呼び手が ~q を確かめてから使う。
Finish ==
    /\ ~q
    /\ worker' = IF worker = "none" THEN "none" ELSE "gone"
    /\ wt' = IF wt = "none" THEN "none" ELSE "removed"
    /\ status' = "done"
    /\ reviewing' = {}
    /\ UNCHANGED <<ci, result, headSeen, dirty, qVars>>

\* andon issue start（internal/issue/start.go start。transition.go が todo の課題だけを通す）: worktree と branch を作り、担当の端末を起こす
ChiefOfStaffStarts ==
    /\ status = "todo"
    /\ status' = "doing" /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* andon issue ask（internal/issue/issue.go ask。doing の担当が打つ）
WorkerAsks ==
    /\ status = "doing" /\ worker = "live" /\ ~q
    /\ q' = TRUE /\ ans' = "none" /\ news' = "none" /\ sent' = FALSE /\ twice' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, cancel, mergeAuto, moved, autoMerged>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* オーナーが Chief of Staff との会話で答え、Chief of Staff が kanban answer を打つ（cmd/kanban/questions.go cmdAnswer。moveQuestion が open の問いだけを通す）
ChiefOfStaffAnswers ==
    /\ q /\ ans = "none" /\ status # "done"
    /\ ans' = "cos" /\ twice' = (twice \/ ans # "none")
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, news, sent>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* オーナーがペインで選択肢を選ぶ（internal/waiting/answer.go Answer → kanban answer。開いている問いだけを通す。端末の外から打つ）
OwnerAnswersInPane ==
    /\ q /\ ans = "none" /\ status # "done"
    /\ ans' = "pane" /\ twice' = (twice \/ ans # "none")
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, news, sent>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* dispatcher が log.jsonl の answered の行（書き手が端末でない）を見て、通知 answered を積む（internal/dispatcher/pickup.go logRow・answered）
DispatcherNotifiesAnswer ==
    /\ q /\ ans = "pane" /\ news = "none"
    /\ news' = "pending"
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, sent, twice>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* Chief of Staff が答えを担当へ hc send する（package/skills/chief-of-staff/playbooks/watch.md）。自分で書いた答えはすぐ、ペインの答えは通知を読んでから。端末が無ければ送れない
ChiefOfStaffRelays ==
    /\ q /\ ~sent /\ worker = "live"
    /\ (ans = "cos" \/ (ans = "pane" /\ news = "pending"))
    /\ sent' = TRUE
    /\ news' = IF ans = "pane" THEN "read" ELSE news
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, twice>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* 担当が答えを受け取る（cmd/kanban/questions.go cmdReceived）。届いた答えは担当の会話の記録に残るので、端末が死んで起き直しても届いたままとする
AnswerReceived ==
    /\ q /\ sent /\ worker = "live" /\ status # "done"
    /\ q' = FALSE /\ ans' = "none" /\ news' = "none" /\ sent' = FALSE /\ twice' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, cancel, mergeAuto, moved, autoMerged>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* gh pr create と andon issue report --pr（internal/issue/issue.go report）。PR を出すと head の CI が走り出す
WorkerOpensPR ==
    /\ status = "doing" /\ worker = "live" /\ pr = "none"
    /\ pr' = "open" /\ ci' = "running"
    /\ UNCHANGED <<status, worker, wt, reviewing, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* GitHub の CI が head で終わった（gate が PR の head の checks を読む。internal/gate/decide.go GitHubChecks）
CIFinishes ==
    /\ ci = "running"
    /\ ci' \in {"passed", "failed"}
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* 担当が直して commit し、push した（worker skill §3・§4）。直すことがあるとき（CI が落ちた・Must fix・gate の直し）だけ。
\* head が新しくなるので、動いているレビュアーは古い commit を見ていることになり、照合が読むレビューも古くなる。CI は走り直す
WorkerPushes ==
    /\ mark' = "none" /\ rel' = FALSE
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ (dirty \/ ci = "failed")
    /\ ci' = "running" /\ dirty' = FALSE /\ headSeen' = FALSE
    /\ reviewing' = IF reviewing = {} THEN {} ELSE {"old"}
    /\ result' = IF result = "none" THEN "none" ELSE "old"
    /\ UNCHANGED <<status, worker, wt, pr, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* 担当が PR の branch を衝突の無い rebase で既定の branch の先へ載せ直し、push した（worker skill §7。Chief of Staff に rebase を頼まれた）。
\* head は新しくなるが PR の差分（行番号のほか）は変わらないので、照合はそれまでのレビューを新しい head に引き継ぐ
\* （internal/ledger/review.go CheckReview・SamePatch）。動いているレビュアーの結果も、返れば引き継ぐ。review-request は
\* 引き継いだレビューがあると断る（internal/issue/issue.go rounds）。走り直すのは CI だけ。
\* 衝突を解いて差分が変わった rebase は WorkerPushes と同じ（レビューは古くなる）。担当に直すことがあるときに限らない
WorkerRebasesSamePatch ==
    /\ mark' = "none" /\ rel' = FALSE
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ ci' = "running"
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* andon issue review-request（internal/issue/issue.go reviewRequest）→ dispatcher がレビュアーを起こす（internal/dispatcher/loop.go reviewRequest）。
\* review-request は CI を読まない（PR を出したらすぐ頼める）。断るのは internal/issue/issue.go rounds の 2 つ:
\*   その課題のレビュアーが動いている間（見ている commit に関わらず。ADR-0032 決めたこと 8）・head のレビューの結果がもうある
ReviewerStarts ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ reviewing = {} /\ ~headSeen
    /\ reviewing' = {"head"}
    /\ UNCHANGED <<status, worker, wt, pr, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* レビュアーが findings.json を書いて返した。照合は最後に終わったレビュアーを読む（internal/ledger/ledger.go reviewers・review.go CheckReview）
ReviewerReturns ==
    /\ \E c \in reviewing :
        /\ reviewing' = reviewing \ {c}
        /\ IF c = "head"
           THEN /\ result' \in {"clean", "mustfix"}
                /\ headSeen' = TRUE
                /\ dirty' = (dirty \/ result' = "mustfix")
           ELSE /\ result' = "old"
                /\ UNCHANGED <<headSeen, dirty, qVars>>
    /\ UNCHANGED <<status, worker, wt, pr, ci, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* andon issue ready（internal/issue/issue.go ready）: unreceived で受け取られていない問いがあれば断り、doing のまま残す。
\* gate の問いのうち前の head について出たもの（staleGateQuestions）は数えない（q は担当の問いだけ。gate が今の head で決めたとき
\* internal/issue/merge.go closeStaleGateQuestions が閉じる）。
\* 照合が head のレビューを読めなければ（CheckReview が current でない）断る。CI は読まない（gate が待つ）
\* 受け入れ条件の行が指す記録が証拠にならなくても断る（contract.CheckRuns。担当が撮り直して打ち直すので、モデルでは歩まないだけ）
\* 課題の予約キー acceptance が名指す受け入れ条件の行が報告に無くても断る（checkAcceptanceRows。担当が行を足して打ち直すので、モデルでは歩まないだけ）
WorkerReady ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open" /\ ~q
    /\ result \in {"clean", "mustfix"}
    /\ status' = "review" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewVars, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* セルが起き直した・端末が落ちた（hc の端末が無くなる）
WorkerTerminalDies ==
    /\ worker = "live" /\ status \in {"doing", "review"}
    /\ worker' = "gone"
    /\ UNCHANGED <<status, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* dispatcher の見回り（internal/dispatcher/pickup.go watch。doing の課題だけを見る）: 担当の端末が Gone なら
\* andon issue resume（internal/dispatcher/resume.go resumeGone → internal/issue/resume.go resume。worktree が無ければ branch から戻す）
DispatcherResumes ==
    /\ status = "doing" /\ worker = "gone"
    /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<status, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* gate が決めたことを捨てて、今の head で回し直す（GateSaysFix・GateAsksHuman・GateApproves の始まりの 1 つ）。PR が OPEN のときだけ:
\*   - approve か needs_human を決めたあとに head が動いた。前の head の gate の問いを閉じて回し直す（internal/issue/merge.go
\*     mergeOne → headMoved）。report.json の commit が前の head のままなので、回し直した gate は fix を返す
\*     （docs/dispatcher.md の「回し直す」の行）。dispatcher が自分でマージする approve なら、マージの直前の読み直し（approve の ChecksStill）
\*   - dispatcher が自分でマージする approve（told が偽）で、マージの直前に読み直すと任されていない（approve の stillAuthorized）か、
\*     gh pr merge が base の動きで通らなかった（approve の RetryMerge）。base はモデルに無いので、この approve はいつでも回し直せるとする
\* コードでは、読み直しが通らない巡は決めたことを書かずに終わり（regate）、次の巡で gate が決め直す。モデルはその 2 歩を 1 歩にまとめた
Regate ==
    /\ pr = "open"
    /\ gate \in {"approve", "human"}
    /\ moved \/ (gate = "approve" /\ ~told)

\* gate が fix（internal/issue/merge.go mergeOne → sendFix）: 理由を担当へ hc send。送れなければ通知 `gate` に unsent を、
\* 担当（agent）がいなければ no_agent を書き、dispatcher が Chief of Staff に届ける（internal/dispatcher/route.go RouteOf。送れた fix は記録だけ）。
\* 理由（internal/gate/decide.go Decide）は CI の失敗・レビューの stale や Must fix・受け入れ条件など。直しは担当に直すことを渡すが、
\* 照合が古いレビューを読んでいる（stale）ときは、stale だけが理由のこともあり、直すことがあるとは限らない。
\* PR が閉じていれば fix（internal/gate/decide.go Decide の「PR が open でない」）。回し直しは Regate
GateSaysFix ==
    /\ ~hold /\ mark' = "red" /\ rel' = FALSE
    /\ status = "review"
    /\ ((gate = "none" /\ pr \in {"open", "closed"}) \/ Regate)
    /\ gate' = "fix" /\ fixSent' = (worker = "live") /\ told' = (worker # "live" \/ (gate = "none" /\ told))
    /\ dirty' = (dirty \/ result # "old")
    /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* gate が needs_human（internal/issue/merge.go mergeOne → askGate）: 問い（from: gate）を出す。通知 asked が届く
GateAsksHuman ==
    /\ ~hold /\ mark' = "ok" /\ rel' = TRUE
    /\ status = "review" /\ pr = "open"
    /\ (gate = "none" \/ Regate)
    /\ gate' = "human" /\ told' = TRUE /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, fixSent, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* gate が approve（internal/issue/merge.go mergeOne）。merge: person なら通知 `gate` が届き、オーナーのマージを待つ（route.go RouteOf）。
\* approve は head の checks が通り、照合が head のレビューを読めて Must fix 以上が無いときだけ（internal/gate/decide.go Decide。
\* checks が走っている間は wait で何も変えない）
GateApproves ==
    /\ ~hold /\ mark' = "ok" /\ rel' = TRUE
    /\ status = "review" /\ pr = "open"
    /\ (gate = "none" \/ Regate)
    /\ ci = "passed" /\ result = "clean"
    /\ gate' = "approve" /\ told' = ~mergeAuto /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, fixSent, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* オーナーが needs_human の問いに「担当に直させる」と答え、Chief of Staff が担当へ送った
ChiefOfStaffSendsBack ==
    /\ status = "review" /\ gate = "human" /\ worker = "live"
    /\ gate' = "fix" /\ fixSent' = TRUE /\ dirty' = TRUE /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, told, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* gate の fix が担当に届かなかった（端末が無い・送ったあとに落ちた）とき、Chief of Staff が担当を起動し直して直しを送る
\* （chief-of-staff の playbooks/watch.md の gate: fix の行: kanban set status=doing → andon issue resume → hc send）。
\* 担当はそのあと kanban start を打つ（WorkerTakesFix）。kanban set で一時 doing にする段（issue.yaml の移り方の外）はまとめた
ChiefOfStaffRevives ==
    /\ status = "review" /\ gate = "fix" /\ worker # "live"
    /\ worker' = "live" /\ wt' = "here" /\ fixSent' = TRUE
    /\ UNCHANGED <<status, pr, reviewVars, gate, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* 担当が直しを受け、kanban start で doing に戻る（cmd/kanban/main.go cmdStart）
WorkerTakesFix ==
    /\ status = "review" /\ gate = "fix" /\ fixSent /\ worker = "live"
    /\ status' = "doing" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewVars, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* approve ＋ merge: auto（internal/issue/merge.go approve → finishMerged）: gate が approve を決めたときに merge: auto で委譲の
\* 範囲内だった（mergeOne の res.Auto。通知 `gate` はオーナーのマージ待ちにならないので told が偽）。マージの直前に、課題の Settings と
\* 委譲（stillAuthorized）、PR の head と checks（ChecksStill）を読み直し、head が動かず、まだ任されているときだけマージする
\* （通らなければ Regate）。マージしたら同じプロセスで finish。finish が断ったら問い（from: gate）を出し、PR はマージしたまま残る
DispatcherMerges ==
    /\ status = "review" /\ gate = "approve" /\ ~told /\ pr = "open"
    /\ ~moved /\ mergeAuto
    /\ ~hold /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ hold)
    /\ pr' = "merged" /\ autoMerged' = TRUE
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewVars, qVars>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, fixSent, q, cancel, mergeAuto, moved, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

\* オーナーが GitHub で PR をマージした。gate が決めたあと（approve で merge: person・needs_human の問いにオーナーが答えた）と、
\* gate を待たずに先にマージした（doing のうち・gate が決める前・fix のあと）の 2 つに分ける。どちらも同じ動きで、
\* 分けるのは規則 R7（IssueRules.tla）が先回りのマージを「マージまで行ける」の道に数えないため
HumanMergesDecided ==
    /\ pr = "open" /\ status = "review" /\ gate \in {"approve", "human"}
    /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ ~rel \/ hold)
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

HumanMergesEarly ==
    /\ pr = "open" /\ status \in {"doing", "review"} /\ ~(status = "review" /\ gate \in {"approve", "human"})
    /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ ~rel \/ hold)
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

\* andon issue settle（internal/issue/settle.go settleOne → closeAndFinish）: review か doing の課題の PR が MERGED で、
\* gate の外の問いが無ければ finish。問いがあれば片付けずに通知 merged（held）。dispatcher がマージした PR は
\* DispatcherMerges が同じ歩みで片付けるので、ここには来ない（settle は mergedByGate で何もしない）
SettleFinishes ==
    /\ status \in {"doing", "review"} /\ pr = "merged" /\ ~q
    /\ Finish
    /\ UNCHANGED <<pr, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* オーナーの go で Chief of Staff が andon issue finish（やめる。internal/issue/finish.go finish）。問いがあれば finish が断るので何も変わらない
ChiefOfStaffCancels ==
    /\ status \in {"doing", "review"} /\ pr # "merged" /\ ~cancel /\ ~q
    /\ cancel' = TRUE
    /\ Finish
    /\ UNCHANGED <<pr, gate, fixSent, told, q, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* 判定のあとに PR の head が動く: gate が approve か needs_human を決めたあと、review のうちに担当かオーナーが PR の branch に push した
\* （docs/dispatcher.md の「回し直す」の行）。PR が閉じている間の push も入れる。開き直すと、gate は判定した head と違う head を
\* 読む（headMoved と approve の ChecksStill はどちらも PR の今の head を読む）。head が新しくなるので、CI は走り直し、レビューは
\* 古くなる（WorkerPushes と同じ）。gate は PR が開いてから Regate で回し直す
HeadMovesAfterGate ==
    /\ mark' = "none" /\ rel' = FALSE
    /\ status = "review" /\ gate \in {"approve", "human"} /\ pr \in {"open", "closed"}
    /\ moved' = TRUE
    /\ ci' = "running" /\ headSeen' = FALSE
    /\ reviewing' = IF reviewing = {} THEN {} ELSE {"old"}
    /\ result' = IF result = "none" THEN "none" ELSE "old"
    /\ UNCHANGED <<status, worker, wt, pr, dirty, gate, fixSent, told, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge>>

\* PR が閉じて開き直される: オーナーが GitHub で PR をマージせずに閉じた・閉じた PR を開き直した。andon issue settle が PR を
\* 1 分に 1 回聞き直して知る（internal/issue/settle.go settleOne。CLOSED は開き直されうるので、MERGED になるまで聞き続ける）。
\* 閉じても片付けない（通知 closed。やめるかは Chief of Staff とオーナーが決める。ChiefOfStaffCancels）
PRClosesOrReopens ==
    /\ status \in {"doing", "review"}
    /\ \/ pr = "open" /\ pr' = "closed"
       \/ pr = "closed" /\ pr' = "open"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* merge の設定と委譲が変わる: Chief of Staff が issue.yaml の merge を書き換えた・オーナーが委譲の merge を変えた
\* （gate は信じる書き手と委譲の範囲を読む。internal/issue/merge.go autoMerge・Authorized）。オーナーが課題を名指しした
\* merge の委譲（andon delegate grant merge … --issues）を足す・取り消す・期限が切れるのも、その課題にとって同じ出来事で、
\* 行動は足さない（ADR-0041 決めたこと 6・8。gate を回し直すきっかけにもしない）。マージしたあとの変更は
\* マージに効かないので入れない
MergeSettingChanges ==
    /\ status # "done" /\ pr # "merged"
    /\ mergeAuto' = ~mergeAuto
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

\* Chief of Staff が止める（andon issue hold。internal/issue/hold.go）。課題の鍵と gate の鍵の中で hold を書き、いまの head の緑の印を黄で
\* 上書きしてから返る。gate が approve か needs_human を決めたあとなら、dispatcher はその判定を捨てる（gate の問いを閉じ、外れたら gate から
\* 回し直す。internal/issue/merge.go holdDecision）。判定を捨てるのは次の巡だが、その間のマージは直前の読み直し（Authorized）が止めを見て断るので、
\* モデルでは同じ歩みにまとめる
ChiefOfStaffHolds ==
    /\ status \in {"doing", "review"} /\ pr \in {"open", "closed"} /\ ~hold
    /\ hold' = TRUE
    /\ mark' = IF mark = "ok" THEN "held" ELSE mark
    /\ gate' = IF status = "review" /\ gate \in {"approve", "human"} THEN "none" ELSE gate
    /\ told' = IF status = "review" /\ gate \in {"approve", "human"} THEN FALSE ELSE told
    /\ moved' = IF status = "review" /\ gate \in {"approve", "human"} THEN FALSE ELSE moved
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, fixSent, q, cancel, mergeAuto, autoMerged, ans, news, sent, twice, required, rel, badMerge>>

\* Chief of Staff が止めを外す（andon issue unhold）。印は書かない。review なら、次の巡で gate が回り直して印を書く
ChiefOfStaffUnholds ==
    /\ hold /\ status # "done"
    /\ hold' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, news, sent, twice, mark, required, rel, badMerge>>

\* dispatcher が 1 分に 1 回 PR を読むとき、印を揃える（andon issue settle の alignMark。internal/issue/settle.go。課題の鍵の中で）:
\*   止めているのに緑が残っている（hold を kanban set で直に書いた）→ 黄。ADR-0056 決めたこと 8
\*   doing に戻った課題の head に緑が残っている → 赤（作った側が直しています）。いまの head を渡したことを取り消す。ADR-0056 決めたこと 3・5
\* 前のバージョンが書いたオーナー待ちの赤を緑に直すのと、Chief Engineer の文書の PR の緑は、課題 1 つの一生の外なので入れない
DispatcherAlignsMark ==
    /\ pr = "open" /\ mark = "ok"
    /\ \/ status \in {"doing", "review"} /\ hold /\ mark' = "held" /\ UNCHANGED rel
       \/ status = "doing" /\ ~hold /\ mark' = "red" /\ rel' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, news, sent, twice, hold, required, badMerge>>

Done ==
    /\ status = "done"
    /\ UNCHANGED vars

Next ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

Spec == Init /\ [][Next]_vars

====
