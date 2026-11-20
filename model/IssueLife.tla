---- MODULE IssueLife ----
\* 課題 1 つの一生（台帳に入ってから片付くまで）と、それに関わる役の動き（行動）。規則は IssueRules.tla に分けてある。
\* 行動ごとに、写したコードの場所を書く（repo からの相対パスと関数）。課題の一生の動きを変えたら、同じ PR でここを直す。
\* 課題は 1 つ・PR は 1 つ・レビューの回は区別しない。commit は「いまの head」と「それより古いもの」の 2 つに分けて見る。
\* 省いたものは README.md の「モデルが省いたもの」。

VARIABLES
    status,     \* 台帳の status: todo・doing・review・done
    worker,     \* 担当の端末: none（まだ起こしていない）・live・gone（落ちた・片付けで消した）
    wt,         \* 担当の worktree: none・here・removed（finish が消した）
    pr,         \* PR: none・open・merged
    reviewing,  \* 動いているレビュアーが見ている commit の集まり: "head"（いまの head）・"old"（head より古い commit）
    ci,         \* いまの head の CI（GitHub の checks）: none（PR がまだ無い）・running・failed・passed
    result,     \* 照合が読むレビュー（結果を出したレビュアーのうち最後に終わったもの。internal/ledger/ledger.go reviewers）:
                \* none・old（head より古い commit を見た。stale）・clean（head を見て Must fix 以上が無い）・mustfix（head を見て Must fix 以上がある）
    headSeen,   \* いまの head を見たレビューの結果がある（どれが最後かに関わらず。review-request はこれがあると断る）
    dirty,      \* 担当に直すことがある（head のレビューの Must fix・ゲートの fix で届いた直し）。CI が落ちたことは ci で表す
    gate,       \* いまの review の行に対するゲートの答え: none・approve・fix・human
    fixSent,    \* ゲートの fix が担当の端末に届いた（hc send が通った）
    told,       \* いまのゲートの答えが Chief of Staff に届いた（<news> か問い）
    q,          \* 担当の問いが受け取られていない（kanban received の前）
    cancel,     \* 人がやめると決め、Chief of Staff の finish が通った
    mergeAuto   \* issue.yaml の merge: auto（委譲の範囲内）。課題ごとに決まっていて変わらない

vars == <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto>>
\* commit とレビューの変数（担当の push・CI・レビュアーが動かす）
reviewVars == <<reviewing, ci, result, headSeen, dirty>>

Init ==
    /\ status = "todo" /\ worker = "none" /\ wt = "none" /\ pr = "none"
    /\ reviewing = {} /\ ci = "none" /\ result = "none" /\ headSeen = FALSE /\ dirty = FALSE
    /\ gate = "none" /\ fixSent = FALSE /\ told = FALSE
    /\ q = FALSE /\ cancel = FALSE /\ mergeAuto \in BOOLEAN

\* andon issue finish（internal/issue/finish.go finish）: 何かを消す前に closable で受け取られていない問いを確かめ、
\* あれば何もしない（片付けは全部か無し）。無ければ端末を消し（removeTerminals）、worktree を消し、最後に kanban done。
\* 呼び手が ~q を確かめてから使う。
Finish ==
    /\ ~q
    /\ worker' = IF worker = "none" THEN "none" ELSE "gone"
    /\ wt' = IF wt = "none" THEN "none" ELSE "removed"
    /\ status' = "done"
    /\ reviewing' = {}
    /\ UNCHANGED <<ci, result, headSeen, dirty>>

\* andon issue start（internal/issue/start.go start。transition.go が todo の課題だけを通す）: worktree と branch を作り、担当の端末を起こす
ChiefOfStaffStarts ==
    /\ status = "todo"
    /\ status' = "doing" /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue ask（internal/issue/issue.go ask。doing の担当が打つ）
WorkerAsks ==
    /\ status = "doing" /\ worker = "live" /\ ~q
    /\ q' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, cancel, mergeAuto>>

\* Chief of Staff が答え、受け取った（cmd/kanban/questions.go cmdAnswer・cmdReceived）
AnswerReceived ==
    /\ q /\ status # "done"
    /\ q' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, cancel, mergeAuto>>

\* gh pr create と andon issue report --pr（internal/issue/issue.go report）。PR を出すと head の CI が走り出す
WorkerOpensPR ==
    /\ status = "doing" /\ worker = "live" /\ pr = "none"
    /\ pr' = "open" /\ ci' = "running"
    /\ UNCHANGED <<status, worker, wt, reviewing, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto>>

\* GitHub の CI が head で終わった（ゲートが PR の head の checks を読む。internal/gate/decide.go GitHubChecks）
CIFinishes ==
    /\ ci = "running"
    /\ ci' \in {"passed", "failed"}
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto>>

\* 担当が直して commit し、push した（worker skill §3・§4）。直すことがあるとき（CI が落ちた・Must fix・ゲートの直し）だけ。
\* head が新しくなるので、動いているレビュアーは古い commit を見ていることになり、照合が読むレビューも古くなる。CI は走り直す
WorkerPushes ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ (dirty \/ ci = "failed")
    /\ ci' = "running" /\ dirty' = FALSE /\ headSeen' = FALSE
    /\ reviewing' = IF reviewing = {} THEN {} ELSE {"old"}
    /\ result' = IF result = "none" THEN "none" ELSE "old"
    /\ UNCHANGED <<status, worker, wt, pr, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue review-request（internal/issue/issue.go reviewRequest）→ dispatcher がレビュアーを起こす（internal/dispatcher/loop.go reviewRequest）。
\* review-request は CI を読まない（PR を出したらすぐ頼める）。断るのは internal/issue/issue.go rounds の 2 つ:
\*   その課題のレビュアーが動いている間（見ている commit に関わらず。ADR-0032 決めたこと 8）・head のレビューの結果がもうある
ReviewerStarts ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ reviewing = {} /\ ~headSeen
    /\ reviewing' = {"head"}
    /\ UNCHANGED <<status, worker, wt, pr, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto>>

\* レビュアーが findings.json を書いて返した。照合は最後に終わったレビュアーを読む（internal/ledger/ledger.go reviewers・review.go CheckReview）
ReviewerReturns ==
    /\ \E c \in reviewing :
        /\ reviewing' = reviewing \ {c}
        /\ IF c = "head"
           THEN /\ result' \in {"clean", "mustfix"}
                /\ headSeen' = TRUE
                /\ dirty' = (dirty \/ result' = "mustfix")
           ELSE /\ result' = "old"
                /\ UNCHANGED <<headSeen, dirty>>
    /\ UNCHANGED <<status, worker, wt, pr, ci, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue ready（internal/issue/issue.go ready）: unreceived で受け取られていない問いがあれば断り、doing のまま残す。
\* 照合が head のレビューを読めなければ（CheckReview が current でない）断る。CI は読まない（ゲートが待つ）
WorkerReady ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open" /\ ~q
    /\ result \in {"clean", "mustfix"}
    /\ status' = "review" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewVars, q, cancel, mergeAuto>>

\* セルが起き直した・端末が落ちた（hc の端末が無くなる）
WorkerTerminalDies ==
    /\ worker = "live" /\ status \in {"doing", "review"}
    /\ worker' = "gone"
    /\ UNCHANGED <<status, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto>>

\* dispatcher の見回り（internal/dispatcher/pickup.go watch。doing の課題だけを見る）: 担当の端末が Gone なら
\* andon issue resume（internal/dispatcher/resume.go resumeGone → internal/issue/resume.go resume。worktree が無ければ branch から戻す）
DispatcherResumes ==
    /\ status = "doing" /\ worker = "gone"
    /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<status, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto>>

\* ゲートが fix（internal/issue/merge.go mergeOne → sendFix）: 理由を担当へ hc send。送れなければ通知 gate に unsent を、
\* 担当（agent）がいなければ no_agent を書き、dispatcher が Chief of Staff に届ける（internal/dispatcher/route.go RouteOf。送れた fix は記録だけ）。
\* 理由（internal/gate/decide.go Decide）は CI の失敗・レビューの stale や Must fix・受け入れ条件など。直しは担当に直すことを渡すが、
\* 照合が古いレビューを読んでいる（stale）ときは、stale だけが理由のこともあり、直すことがあるとは限らない
GateSaysFix ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "fix" /\ fixSent' = (worker = "live") /\ told' = (told \/ worker # "live")
    /\ dirty' = (dirty \/ result # "old")
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, q, cancel, mergeAuto>>

\* ゲートが needs_human（internal/issue/merge.go mergeOne → askGate）: 問い（from: gate）を出す。通知 asked が届く
GateAsksHuman ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "human" /\ told' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, fixSent, q, cancel, mergeAuto>>

\* ゲートが approve（internal/issue/merge.go mergeOne）。merge: person なら通知 gate が届き、人のマージを待つ（route.go RouteOf）。
\* approve は head の checks が通り、照合が head のレビューを読めて Must fix 以上が無いときだけ（internal/gate/decide.go Decide。
\* checks が走っている間は wait で何も変えない）
GateApproves ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ ci = "passed" /\ result = "clean"
    /\ gate' = "approve" /\ told' = ~mergeAuto
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, fixSent, q, cancel, mergeAuto>>

\* 人が needs_human の問いに「担当に直させる」と答え、Chief of Staff が担当へ送った
ChiefOfStaffSendsBack ==
    /\ status = "review" /\ gate = "human" /\ worker = "live"
    /\ gate' = "fix" /\ fixSent' = TRUE /\ dirty' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, told, q, cancel, mergeAuto>>

\* ゲートの fix が担当に届かなかった（端末が無い・送ったあとに落ちた）とき、Chief of Staff が担当を起動し直して直しを送る
\* （chief-of-staff の playbooks/watch.md の gate: fix の行: kanban set status=doing → andon issue resume → hc send）。
\* 担当はそのあと kanban start を打つ（WorkerTakesFix）。kanban set で一時 doing にする段（issue.yaml の移り方の外）はまとめた
ChiefOfStaffRevives ==
    /\ status = "review" /\ gate = "fix" /\ worker # "live"
    /\ worker' = "live" /\ wt' = "here" /\ fixSent' = TRUE
    /\ UNCHANGED <<status, pr, reviewVars, gate, told, q, cancel, mergeAuto>>

\* 担当が直しを受け、kanban start で doing に戻る（cmd/kanban/main.go cmdStart）
WorkerTakesFix ==
    /\ status = "review" /\ gate = "fix" /\ fixSent /\ worker = "live"
    /\ status' = "doing" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewVars, q, cancel, mergeAuto>>

\* approve ＋ merge: auto（internal/issue/merge.go approve → finishMerged）: dispatcher がマージして同じプロセスで finish。
\* finish が断ったら問い（from: gate）を出し、PR はマージしたまま残る
DispatcherMerges ==
    /\ status = "review" /\ gate = "approve" /\ mergeAuto /\ pr = "open"
    /\ pr' = "merged"
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewVars>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, fixSent, q, cancel, mergeAuto>>

\* 人が GitHub で PR をマージした。ゲートが決めたあと（approve で merge: person・needs_human の問いに人が答えた）と、
\* ゲートを待たずに先にマージした（doing のうち・ゲートが決める前・fix のあと）の 2 つに分ける。どちらも同じ動きで、
\* 分けるのは規則 R7（IssueRules.tla）が先回りのマージを「マージまで行ける」の道に数えないため
HumanMergesDecided ==
    /\ pr = "open" /\ status = "review" /\ gate \in {"approve", "human"}
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto>>

HumanMergesEarly ==
    /\ pr = "open" /\ status \in {"doing", "review"} /\ ~(status = "review" /\ gate \in {"approve", "human"})
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue settle（internal/issue/settle.go settleOne → closeAndFinish）: review か doing の課題の PR が MERGED で、
\* ゲートの外の問いが無ければ finish。問いがあれば片付けずに通知 merged（held）。dispatcher がマージした PR は
\* DispatcherMerges が同じ歩みで片付けるので、ここには来ない（settle は mergedByGate で何もしない）
SettleFinishes ==
    /\ status \in {"doing", "review"} /\ pr = "merged" /\ ~q
    /\ Finish
    /\ UNCHANGED <<pr, gate, fixSent, told, q, cancel, mergeAuto>>

\* 人の go で Chief of Staff が andon issue finish（やめる。internal/issue/finish.go finish）。問いがあれば finish が断るので何も変わらない
ChiefOfStaffCancels ==
    /\ status \in {"doing", "review"} /\ pr # "merged" /\ ~cancel /\ ~q
    /\ cancel' = TRUE
    /\ Finish
    /\ UNCHANGED <<pr, gate, fixSent, told, q, mergeAuto>>

Done ==
    /\ status = "done"
    /\ UNCHANGED vars

Next ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels \/ Done

Spec == Init /\ [][Next]_vars

====
