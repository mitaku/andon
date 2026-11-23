---- MODULE BrokenPaneAnswersTwice ----
\* 壊した見本（R10 で落ちるはず）: andon waiting answer が答え済みの問いにも書く（開いている問いだけを通す守りを外した動き。
\* internal/waiting/answer.go の status open の確かめと kanban answer の moveQuestion が無い）。担当が問いを出し、Chief of Staff が
\* 会話の答えを書いたあとにオーナーがペインで答えると、1 つの問いに答えが 2 つになる（ADR-0042 決めたこと 19 のモデルが見つけた破れ）。
EXTENDS IssueRules

BrokenOwnerAnswersInPane ==
    /\ q /\ status # "done"
    /\ ans' = "pane" /\ twice' = (twice \/ ans # "none")
    /\ UNCHANGED <<status, worker, wt, pr, reviewVars, gate, fixSent, told, q, cancel, mergeAuto, moved, autoMerged, news, sent>>
    /\ UNCHANGED mark

BrokenNext ==
    \/ ChiefOfStaffStarts \/ WorkerAsks \/ ChiefOfStaffAnswers \/ BrokenOwnerAnswersInPane \/ DispatcherNotifiesAnswer
    \/ ChiefOfStaffRelays \/ AnswerReceived \/ WorkerOpensPR
    \/ CIFinishes \/ WorkerPushes \/ WorkerRebasesSamePatch
    \/ ReviewerStarts \/ ReviewerReturns \/ WorkerReady \/ WorkerTerminalDies
    \/ DispatcherResumes \/ GateSaysFix \/ GateAsksHuman \/ GateApproves
    \/ ChiefOfStaffSendsBack \/ ChiefOfStaffRevives \/ WorkerTakesFix \/ DispatcherMerges
    \/ HumanMergesDecided \/ HumanMergesEarly
    \/ SettleFinishes \/ ChiefOfStaffCancels
    \/ HeadMovesAfterGate \/ PRClosesOrReopens \/ MergeSettingChanges
    \/ DispatcherAlignsMark \/ Done

BrokenSpec == Init /\ [][BrokenNext]_vars

====
