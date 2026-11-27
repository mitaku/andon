---- MODULE BrokenNoRegateAfterPrev ----
\* 壊した見本（R18 で落ちるはず）: 前の PR 待ち（behind）と判定したあと、前の課題が main に入っても gate が判定をやり直さない
\* （前の課題が main に入ったあとに、前の PR 待ちの課題の gate を回し直して載せ替えを頼む口が無い動き。ADR-0067 決めたこと 12・16）。積んだ課題は
\* 前の PR 待ちのまま置かれ、載せ替えを頼む直しが担当に届かず、オーナーの判断も push も止めも借りずには既定の branch に載れない。
EXTENDS IssueRules

BrokenGateSaysFix ==
    /\ GateSaysFix
    /\ ~(gate = "behind" /\ ~moved)

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ BrokenGateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
