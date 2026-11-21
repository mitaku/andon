---- MODULE BrokenFixUnsent ----
\* 壊した見本（R2 で落ちるはず）: gate の fix を担当へ送れなくても、誰にも届けない（A-0003 で直す前の動き）。
\* 担当の端末が無いと hc send が失敗し、通知 `gate`(fix) は記録だけなので、課題は review のまま止まる。
EXTENDS IssueRules

BrokenGateSaysFix ==
    /\ status = "review"
    /\ ((gate = "none" /\ pr \in {"open", "closed"}) \/ Regate)
    /\ gate' = "fix" /\ fixSent' = (worker = "live")
    /\ dirty' = (dirty \/ result # "old")
    /\ moved' = FALSE
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, told, q, cancel, mergeAuto, autoMerged>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ BrokenGateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
