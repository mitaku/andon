---- MODULE BrokenFixUnsent ----
\* 壊した見本（R2 で落ちるはず）: ゲートの fix を担当へ送れなくても、誰にも届けない（A-0003 で直す前の動き）。
\* 担当の端末が無いと hc send が失敗し、通知 gate(fix) は記録だけなので、課題は review のまま止まる。
EXTENDS IssueRules

BrokenGateSaysFix ==
    /\ status = "review" /\ gate = "none" /\ pr = "open"
    /\ gate' = "fix" /\ fixSent' = (worker = "live")
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, told, q, cancel, mergeAuto>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ AnswerReceived \/ WorkerOpensPR
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ BrokenGateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ WorkerTakesFix \/ DispatcherMerges \/ HumanMerges
    \/ SettleFinishes \/ ChiefOfStaffCancels \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
