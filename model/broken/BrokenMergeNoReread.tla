---- MODULE BrokenMergeNoReread ----
\* 壊した見本（R8 で落ちるはず）: dispatcher がマージの直前に読み直さない（internal/issue/merge.go approve の stillAuthorized と
\* ChecksStill を外した動き。DispatcherMerges の「~moved /\ mergeAuto」が無い）。gate が approve を決めたあとに PR の head が
\* 動いても、merge の設定や委譲が変わっても、approve を決めたときの答えのままマージする。
EXTENDS IssueRules

BrokenDispatcherMerges ==
    /\ status = "review" /\ gate = "approve" /\ ~told /\ pr = "open"
    /\ pr' = "merged" /\ autoMerged' = TRUE
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewVars>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, moved, fixSent, q, cancel, mergeAuto>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ BrokenDispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
