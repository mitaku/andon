---- MODULE BrokenOwnerMergesStacked ----
\* 壊した見本（R15 で落ちるはず）: 前の課題の branch に載ったまま（draft で、向き先が前の課題の branch）の PR を、オーナーがマージできる
\* （ADR-0067 決めたこと 15 の draft の守りが無い動き。決めたこと 16）。前の課題の変更ごと、この課題の PR がマージされる。
EXTENDS IssueRules

BrokenHumanMergesEarly ==
    /\ UNCHANGED stackVars
    /\ pr = "open" /\ status \in {"doing", "review"} /\ ~(status = "review" /\ gate \in {"approve", "human"})
    /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ ~rel \/ hold)
    /\ pr' = "merged"
    /\ UNCHANGED <<status, worker, wt, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ BrokenHumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
