---- MODULE BrokenFinish ----
\* 壊した見本（R3 で落ちるはず）: finish が端末と worktree を消してから問いを確かめる（A-0002 で直す前の動き）。
\* 問いが開いていると kanban done だけが断られ、片付けかけ（worktree は消えたが done でない）が残る。
EXTENDS IssueRules

BrokenFinishStep ==
    /\ worker' = IF worker = "none" THEN "none" ELSE "gone"
    /\ wt' = IF wt = "none" THEN "none" ELSE "removed"
    /\ status' = IF q THEN status ELSE "done"
    /\ reviewing' = FALSE

BrokenCancels ==
    /\ status \in {"doing", "review"} /\ pr # "merged" /\ ~cancel
    /\ cancel' = TRUE
    /\ BrokenFinishStep
    /\ UNCHANGED <<pr, gate, fixSent, told, q, mergeAuto>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ AnswerReceived \/ WorkerOpensPR
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ WorkerTakesFix \/ DispatcherMerges \/ HumanMerges
    \/ SettleFinishes \/ BrokenCancels \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
