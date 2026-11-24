---- MODULE BrokenHoldNoMark ----
\* 壊した見本（R13 で落ちるはず）: andon issue hold が hold を書くだけで、いまの head の緑の印を黄に上書きしない（internal/issue/hold.go
\* holdMark を外した動き。ADR-0056 決めたこと 8）。オーナーが GitHub で見る印は緑のままで、止めているのにマージしてよく見える。
EXTENDS IssueRules

BrokenChiefOfStaffHolds ==
    /\ status \in {"doing", "review"} /\ pr \in {"open", "closed"} /\ ~hold
    /\ hold' = TRUE
    /\ gate' = IF status = "review" /\ gate \in {"approve", "human"} THEN "none" ELSE gate
    /\ told' = IF status = "review" /\ gate \in {"approve", "human"} THEN FALSE ELSE told
    /\ moved' = IF status = "review" /\ gate \in {"approve", "human"} THEN FALSE ELSE moved
    /\ UNCHANGED <<status, worker, wt, pr, reviewing, ci, result, headSeen, dirty, fixSent, q, cancel, mergeAuto, autoMerged, ans, news, sent, twice, mark, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ BrokenChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
