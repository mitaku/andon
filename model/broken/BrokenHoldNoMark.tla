---- MODULE BrokenHoldNoMark ----
\* 壊した見本（R13 で落ちるはず）: andon issue hold が hold を書くだけで、いまの head の緑の印を黄に上書きしない（internal/issue/hold.go
\* holdMark を外した動き。ADR-0056 決めたこと 8）。オーナーが GitHub で見る印は緑のままで、止めているのにマージしてよく見える。
EXTENDS IssueRules

BrokenChiefOfStaffHolds ==
    /\ UNCHANGED stackVars
    /\ status \in {"doing", "review"} /\ pr \in {"open", "closed"} /\ ~hold
    /\ hold' = TRUE
    /\ gate' = IF status = "review" /\ gate \in {"approve", "human", "behind"} THEN "none" ELSE gate
    /\ told' = IF status = "review" /\ gate \in {"approve", "human", "behind"} THEN FALSE ELSE told
    /\ moved' = IF status = "review" /\ gate \in {"approve", "human", "behind"} THEN FALSE ELSE moved
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
    \/ BrokenChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark
    \/ GateSaysBehind \/ WorkerRestacks \/ PrevMerges \/ PrevEnds \/ ChiefOfStaffContinuesWithoutPrev \/ GateTellsPrevGone \/ Done

\* 前の課題の無い課題だけを見る（積んだ課題の動きは、この見本が壊したものに関わらない。状態の図を積む前の大きさに保つ。ADR-0067 決めたこと 16）
BrokenInit == Init /\ prev = "none"

BrokenSpec == BrokenInit /\ [][BrokenNext]_vars

====
