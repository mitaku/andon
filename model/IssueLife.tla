---- MODULE IssueLife ----
\* 課題 1 つの一生（台帳に入ってから片付くまで）と、それに関わる役の動き（行動）。規則は IssueRules.tla に分けてある。
\* 行動ごとに、写したコードの場所を書く（repo からの相対パスと関数）。課題の一生の動きを変えたら、同じ PR でここを直す。
\* 課題は 1 つ・PR は 1 つ・レビューの回は区別しない。省いたものは README.md の「モデルが省いたもの」。

VARIABLES
    status,     \* 台帳の status: todo・doing・review・done
    worker,     \* 担当の端末: none（まだ起こしていない）・live・gone（落ちた・片付けで消した）
    wt,         \* 担当の worktree: none・here・removed（finish が消した）
    pr,         \* PR: none・open・merged
    reviewing,  \* レビュアーが動いている（dispatcher が review_request を拾って起動した）
    gate,       \* いまの review の行に対するゲートの答え: none・approve・fix・human
    fixSent,    \* ゲートの fix が担当の端末に届いた（hc send が通った）
    told,       \* いまのゲートの答えが Chief of Staff に届いた（<news> か問い）
    q,          \* 担当の問いが受け取られていない（kanban received の前）
    cancel,     \* 人がやめると決め、Chief of Staff の finish が通った
    mergeAuto   \* issue.yaml の merge: auto（委譲の範囲内）。課題ごとに決まっていて変わらない

vars == <<status, worker, wt, pr, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

Init ==
    /\ status = "todo" /\ worker = "none" /\ wt = "none" /\ pr = "none"
    /\ reviewing = FALSE /\ gate = "none" /\ fixSent = FALSE /\ told = FALSE
    /\ q = FALSE /\ cancel = FALSE /\ mergeAuto \in BOOLEAN

\* andon issue finish（internal/issue/finish.go finish）: 何かを消す前に closable で受け取られていない問いを確かめ、
\* あれば何もしない（片付けは全部か無し）。無ければ端末を消し（removeTerminals）、worktree を消し、最後に kanban done。
\* 呼び手が ~q を確かめてから使う。
Finish ==
    /\ ~q
    /\ worker' = IF worker = "none" THEN "none" ELSE "gone"
    /\ wt' = IF wt = "none" THEN "none" ELSE "removed"
    /\ status' = "done"
    /\ reviewing' = FALSE

\* andon issue start（internal/issue/start.go start。transition.go が todo の課題だけを通す）: worktree と branch を作り、担当の端末を起こす
ChiefOfStaffStarts ==
    /\ status = "todo"
    /\ status' = "doing" /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<pr, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue ask（internal/issue/issue.go ask。doing の担当が打つ）
WorkerAsks ==
    /\ status = "doing" /\ worker = "live" /\ ~q
    /\ q' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, gate, fixSent, told, cancel, mergeAuto>>

\* Chief of Staff が答え、受け取った（cmd/kanban/questions.go cmdAnswer・cmdReceived）
AnswerReceived ==
    /\ q /\ status # "done"
    /\ q' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, gate, fixSent, told, cancel, mergeAuto>>

\* gh pr create と andon issue report --pr（internal/issue/issue.go report）
WorkerOpensPR ==
    /\ status = "doing" /\ worker = "live" /\ pr = "none"
    /\ pr' = "open"
    /\ UNCHANGED <<status, worker, wt, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue review-request（internal/issue/issue.go reviewRequest）→ dispatcher がレビュアーを起こす（internal/dispatcher/loop.go reviewRequest）
\* ~reviewing: その commit のレビュアーが動いている間は、review-request が断る（internal/issue/issue.go rounds）
ReviewerStarts ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open" /\ ~reviewing
    /\ reviewing' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, gate, fixSent, told, q, cancel, mergeAuto>>

ReviewerReturns ==
    /\ reviewing
    /\ reviewing' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, gate, fixSent, told, q, cancel, mergeAuto>>

\* andon issue ready（internal/issue/issue.go ready）: unreceived で受け取られていない問いがあれば断り、doing のまま残す
WorkerReady ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open" /\ ~reviewing /\ ~q
    /\ status' = "review" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewing, q, cancel, mergeAuto>>

\* セルが起き直した・端末が落ちた（hc の端末が無くなる）
WorkerTerminalDies ==
    /\ worker = "live" /\ status \in {"doing", "review"}
    /\ worker' = "gone"
    /\ UNCHANGED <<status, wt, pr, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

\* dispatcher の見回り（internal/dispatcher/pickup.go watch。doing の課題だけを見る）: 担当の端末が Gone なら
\* andon issue resume（internal/dispatcher/resume.go resumeGone → internal/issue/resume.go resume。worktree が無ければ branch から戻す）
DispatcherResumes ==
    /\ status = "doing" /\ worker = "gone"
    /\ worker' = "live" /\ wt' = "here"
    /\ UNCHANGED <<status, pr, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

\* ゲートが fix（internal/issue/merge.go mergeOne → sendFix）: 理由を担当へ hc send。送れなければ通知 gate に unsent を、
\* 担当（agent）がいなければ no_agent を書き、dispatcher が Chief of Staff に届ける（internal/dispatcher/route.go RouteOf。送れた fix は記録だけ）
GateSaysFix ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "fix" /\ fixSent' = (worker = "live") /\ told' = (told \/ worker # "live")
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, q, cancel, mergeAuto>>

\* ゲートが needs_human（internal/issue/merge.go mergeOne → askGate）: 問い（from: gate）を出す。通知 asked が届く
GateAsksHuman ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "human" /\ told' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, fixSent, q, cancel, mergeAuto>>

\* ゲートが approve（internal/issue/merge.go mergeOne）。merge: person なら通知 gate が届き、人のマージを待つ（route.go RouteOf）
GateApproves ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "approve" /\ told' = ~mergeAuto
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, fixSent, q, cancel, mergeAuto>>

\* 人が needs_human の問いに「担当に直させる」と答え、Chief of Staff が担当へ送った
ChiefOfStaffSendsBack ==
    /\ status = "review" /\ gate = "human" /\ worker = "live"
    /\ gate' = "fix" /\ fixSent' = TRUE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, told, q, cancel, mergeAuto>>

\* 担当が直しを受け、kanban start で doing に戻る（cmd/kanban/main.go cmdStart）
WorkerTakesFix ==
    /\ status = "review" /\ gate = "fix" /\ fixSent /\ worker = "live"
    /\ status' = "doing" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewing, q, cancel, mergeAuto>>

\* approve ＋ merge: auto（internal/issue/merge.go approve → finishMerged）: dispatcher がマージして同じプロセスで finish。
\* finish が断ったら問い（from: gate）を出し、PR はマージしたまま残る
DispatcherMerges ==
    /\ status = "review" /\ gate = "approve" /\ mergeAuto /\ pr = "open"
    /\ pr' = "merged"
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewing>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, fixSent, q, cancel, mergeAuto>>

\* 人が GitHub で PR をマージした
HumanMerges ==
    /\ pr = "open" /\ status \in {"doing", "review"}
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewing, gate, fixSent, told, q, cancel, mergeAuto>>

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
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ WorkerTakesFix \/ DispatcherMerges \/ HumanMerges
    \/ SettleFinishes \/ ChiefOfStaffCancels \/ Done

Spec == Init /\ [][Next]_vars

====
