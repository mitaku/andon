---- MODULE BrokenReady ----
\* 壊した見本（R6 で落ちるはず）: andon issue ready が開いた問いを確かめずに review へ移す（A-0006 で直す前の動き）。
EXTENDS IssueRules

BrokenWorkerReady ==
    /\ UNCHANGED stackVars
    /\ ~(base = "prev" /\ prev # "waiting")
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
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

\* 前の課題の無い課題だけを見る（積んだ課題の動きは、この見本が壊したものに関わらない。状態の図を積む前の大きさに保つ。ADR-0067 決めたこと 16）
BrokenInit == Init /\ prev = "none"

BrokenSpec == BrokenInit /\ [][BrokenNext]_vars

====
