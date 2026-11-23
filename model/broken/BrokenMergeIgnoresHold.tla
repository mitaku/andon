---- MODULE BrokenMergeIgnoresHold ----
\* 壊した見本（R14 で落ちるはず）: dispatcher がマージの直前に止めを読み直さない（internal/issue/merge.go Authorized の held を外した動き。
\* ADR-0056 決めたこと 9）。gate が approve を決めたあとに Chief of Staff が止めても、approve を決めたときの答えのままマージする。
\* ruleset が印を必須にしていなければ、印が黄でも GitHub は断らない。
EXTENDS IssueRules

BrokenDispatcherMerges ==
    /\ status = "review" /\ gate = "approve" /\ ~told /\ pr = "open"
    /\ ~moved /\ mergeAuto
    /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ hold)
    /\ pr' = "merged" /\ autoMerged' = TRUE
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewVars, qVars>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, fixSent, q, cancel, mergeAuto, moved, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

\* 判定を捨てるのが次の巡（holdDecision）で、止める歩みとは別だと見る: 止めても gate の判定は残る
HoldsKeepingDecision ==
    /\ status \in {"doing", "review"} /\ pr \in {"open", "closed"} /\ ~hold
    /\ hold' = TRUE
    /\ mark' = IF mark = "ok" THEN "held" ELSE mark
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, ans, news, sent, twice, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ BrokenDispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ HoldsKeepingDecision \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
