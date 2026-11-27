---- MODULE BrokenMergeNoReread ----
\* 壊した見本（R8 で落ちるはず）: dispatcher がマージの直前に読み直さない（internal/issue/merge.go approve の stillAuthorized と
\* ChecksStill を外した動き。DispatcherMerges の「~moved /\ mergeAuto」が無い）。gate が approve を決めたあとに PR の head が
\* 動いても、merge の設定や委譲が変わっても、approve を決めたときの答えのままマージする。
EXTENDS IssueRules

BrokenDispatcherMerges ==
    /\ UNCHANGED stackVars
    /\ base = "main"
    /\ status = "review" /\ gate = "approve" /\ ~told /\ pr = "open"
    /\ ~hold /\ (required => mark = "ok")
    /\ badMerge' = (badMerge \/ hold)
    /\ pr' = "merged" /\ autoMerged' = TRUE
    /\ IF q
       THEN /\ told' = TRUE
            /\ UNCHANGED <<status, worker, wt, reviewVars>>
       ELSE /\ Finish
            /\ UNCHANGED told
    /\ UNCHANGED <<gate, moved, fixSent, q, cancel, mergeAuto, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ BrokenDispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

\* 前の課題の無い課題だけを見る（積んだ課題の動きは、この見本が壊したものに関わらない。状態の図を積む前の大きさに保つ。ADR-0067 決めたこと 16）
BrokenInit == Init /\ prev = "none"

BrokenSpec == BrokenInit /\ [][BrokenNext]_vars

====
