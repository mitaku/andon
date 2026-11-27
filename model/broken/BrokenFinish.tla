---- MODULE BrokenFinish ----
\* 壊した見本（R3 で落ちるはず）: finish が端末と worktree を消してから問いを確かめる（A-0002 で直す前の動き）。
\* 問いが開いていると kanban done だけが断られ、片付けかけ（worktree は消えたが done でない）が残る。
EXTENDS IssueRules

BrokenFinishStep ==
    /\ UNCHANGED stackVars
    /\ worker' = IF worker = "none" THEN "none" ELSE "gone"
    /\ wt' = IF wt = "none" THEN "none" ELSE "removed"
    /\ status' = IF q THEN status ELSE "done"
    /\ reviewing' = {}
    /\ UNCHANGED <<ci, result, headSeen, dirty>>

BrokenCancels ==
    /\ UNCHANGED stackVars
    /\ status \in {"doing", "review"} /\ pr # "merged" /\ ~cancel
    /\ cancel' = TRUE
    /\ BrokenFinishStep
    /\ UNCHANGED <<pr, gate, fixSent, told, q, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ BrokenCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

\* 前の課題の無い課題だけを見る（積んだ課題の動きは、この見本が壊したものに関わらない。状態の図を積む前の大きさに保つ。ADR-0067 決めたこと 16）
BrokenInit == Init /\ prev = "none"

BrokenSpec == BrokenInit /\ [][BrokenNext]_vars

====
