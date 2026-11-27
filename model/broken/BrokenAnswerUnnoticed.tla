---- MODULE BrokenAnswerUnnoticed ----
\* 壊した見本（R7 で落ちるはず）: dispatcher がペインの答えに通知 answered を積まない（ADR-0042 決めたこと 19 の捨てた案。
\* internal/dispatcher/pickup.go の answered を外した動き）。オーナーがペインで答えても Chief of Staff が動くきっかけが無く、
\* 答えが担当に届かないので、問いが閉じずマージまで行けない。
EXTENDS IssueRules

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
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
