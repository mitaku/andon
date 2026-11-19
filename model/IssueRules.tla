---- MODULE IssueRules ----
\* 課題の一生が守る規則（ADR-0025 決めたこと 5）。行動は IssueLife.tla。規則を弱める・消す変更は人が見る。
EXTENDS IssueLife

TypeOK ==
    /\ status \in {"todo", "doing", "review", "done"}
    /\ worker \in {"none", "live", "gone"}
    /\ wt \in {"none", "here", "removed"}
    /\ pr \in {"none", "open", "merged"}
    /\ gate \in {"none", "approve", "fix", "human"}
    /\ reviewing \in BOOLEAN /\ fixSent \in BOOLEAN /\ told \in BOOLEAN
    /\ q \in BOOLEAN /\ cancel \in BOOLEAN /\ mergeAuto \in BOOLEAN

\* R1 やめると決めた課題で、担当がまた動き出すことはない
R1_NoWorkerAfterCancel == cancel => worker # "live"

\* R2 ゲートが直しを返したら、担当か Chief of Staff のどちらかが必ずそれを知る
R2_FixReachesSomeone == (status = "review" /\ gate = "fix") => (fixSent \/ told)

\* R3 worktree を片付けたら、課題は done になっている（片付けは全部か無し）
R3_CleanupIsWhole == wt = "removed" => status = "done"

\* R4 done の課題は、マージしたか、やめると決めたものだけ
R4_DoneMeansMergedOrCancelled == status = "done" => (pr = "merged" \/ cancel)

\* R5 done の課題に担当は残らない
R5_NoWorkerWhenDone == status = "done" => worker # "live"

\* R6 担当の問いが開いたまま review にならない
R6_NoReviewWithOpenQuestion == status = "review" => ~q

====
