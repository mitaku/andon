---- MODULE BrokenFixUnsent ----
\* 壊した見本（R2 で落ちるはず）: gate の fix を担当へ送れなくても、誰にも届けない（A-0003 で直す前の動き）。
\* 担当の端末が無いと hc send が失敗し、通知 `gate`(fix) は記録だけなので、課題は review のまま止まる。
EXTENDS IssueRules

BrokenGateSaysFix ==
    /\ UNCHANGED stackVars
    /\ status = "review"
    /\ ((gate = "none" /\ pr \in {"open", "closed"}) \/ Regate)
    /\ gate' = "fix" /\ fixSent' = (worker = "live")
    /\ dirty' = (dirty \/ result # "old")
    /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, told, q, cancel, mergeAuto, autoMerged, qVars>>
    /\ ~hold /\ mark' = "red" /\ rel' = FALSE
    /\ UNCHANGED <<hold, required, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ BrokenGateSaysFix \/ GateAsksHuman \/ GateApproves
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
