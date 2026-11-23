---- MODULE BrokenReviewTwice ----
\* 壊した見本（R7 で落ちるはず）: review-request が断るのは head を見るレビュアーが動いているときだけで、古い commit を見る
\* レビュアーが動いていても頼み直せる（ADR-0032 決めたこと 8 で直す前の動き）。CI が落ちて直した commit で頼み直すと
\* レビュアーが 2 人走り、古い commit のレビュアーが後に終わると、ready は stale で断り、review-request は「head のレビューは
\* もうある」で断る。直すことが無ければ push もしないので、取り消しのほかにマージへ進む道が無い。
EXTENDS IssueRules

BrokenReviewerStarts ==
    /\ status = "doing" /\ worker = "live" /\ pr = "open"
    /\ "head" \notin reviewing /\ ~headSeen
    /\ reviewing' = reviewing \cup {"head"}
    /\ UNCHANGED <<status, worker, wt, pr, ci, result, headSeen, dirty, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, qVars>>
    /\ UNCHANGED <<mark, hold, required, rel, badMerge>>

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ OwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes
    \/ BrokenReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ ChiefOfStaffHolds \/ ChiefOfStaffUnholds \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
