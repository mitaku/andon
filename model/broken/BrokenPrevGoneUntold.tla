---- MODULE BrokenPrevGoneUntold ----
\* 壊した見本（R20 で落ちるはず）: 前の課題がマージされずに終わったことを、gate が前の PR 待ちと判定したときにしか Chief of Staff に
\* 知らせない（前の PR 待ちと判定したあとに前の課題が終わっても、dispatcher が gate を回し直して通知 gate を届けない。ADR-0067 決めたこと 14・16）。
\* 判定のあとに前の課題が終わった課題では、Chief of Staff がオーナーに聞けず、オーナーの答えで担当へ返す動きに届かないので、
\* 積んだ課題は前の PR 待ちのまま置かれる。
EXTENDS IssueRules

BrokenGateSaysBehind ==
    /\ ~hold /\ mark' = "held" /\ rel' = FALSE
    /\ status = "review" /\ pr = "open"
    /\ (gate = "none" \/ Regate)
    /\ base = "prev" /\ prev \in {"waiting", "gone"}
    /\ ci \in {"passed", "none"} /\ result \in {"clean", "mustfix"}
    /\ gate' = "behind" /\ told' = FALSE /\ moved' = FALSE
    /\ prevTold' = (prev = "gone")
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, fixSent, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ UNCHANGED <<hold, required, badMerge, base, prev>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ BrokenGateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
