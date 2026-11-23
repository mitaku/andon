---- MODULE BrokenClosedNoRecheck ----
\* 壊した見本（R7 で落ちるはず）: andon issue settle が、閉じた PR を聞き直さない（internal/issue/settle.go settleOne が
\* CLOSED と分かったあと GitHub に聞かなくなる動き）。オーナーが PR を開き直しても、ラインは閉じたままと見るので、
\* やめるか先回りのマージのほかにマージへ進む道が無い。
EXTENDS IssueRules

BrokenPRCloses ==
    /\ status \in {"doing", "review"}
    /\ pr = "open" /\ pr' = "closed"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ BrokenPRCloses \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
