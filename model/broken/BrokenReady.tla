---- MODULE BrokenReady ----
\* 壊した見本（R6 で落ちるはず）: andon issue ready が開いた問いを確かめずに review へ移す（A-0006 で直す前の動き）。
EXTENDS IssueRules

BrokenWorkerReady ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ result \in {"clean", "mustfix"}
    /\ status' = "review" /\ gate' = "none" /\ fixSent' = FALSE /\ told' = FALSE
    /\ UNCHANGED <<worker, wt, pr, reviewVars, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ BrokenWorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
