---- MODULE IssueRules ----
\* 課題の一生が守る規則（ADR-0025 決めたこと 5）。行動は IssueLife.tla。規則を弱める・消す変更は人が見る。
EXTENDS IssueLife

TypeOK ==
    /\ status \in {"todo", "doing", "review", "done"}
    /\ worker \in {"none", "live", "gone"}
    /\ wt \in {"none", "here", "removed"}
    /\ pr \in {"none", "open", "closed", "merged"}
    /\ gate \in {"none", "approve", "fix", "human"}
    /\ reviewing \subseteq {"head", "old"}
    /\ ci \in {"none", "running", "failed", "passed"}
    /\ result \in {"none", "old", "clean", "mustfix"}
    /\ headSeen \in BOOLEAN /\ dirty \in BOOLEAN
    /\ fixSent \in BOOLEAN /\ told \in BOOLEAN
    /\ q \in BOOLEAN /\ cancel \in BOOLEAN /\ mergeAuto \in BOOLEAN
    /\ moved \in BOOLEAN /\ autoMerged \in BOOLEAN

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

\* R7 取り消しを使わずに届くどの状態からも、取り消しを使わずにマージまで行ける（ADR-0032 決めたこと 9）。
\*    取り消しは ChiefOfStaffCancels と、ゲートを待たない人の先回りのマージ（HumanMergesEarly）。
\*    「行ける道がある」は状態 1 つの式（INVARIANT）では書けないので、検査器の状態の図を model/reach.mjs が読む。
\*    設定は IssueRules.cfg の「\* REACH」の行（R7_MergeWithoutCancel pr = "merged" と除く行動）。
\*    閉じた PR を開き直す道（PRClosesOrReopens）も含めて成り立つ（ADR-0034 決めたこと 3）。

\* R8 dispatcher が自動でマージするのは、ゲートが approve と判定した head で、その時点で任されているものだけ（ADR-0034 決めたこと 3）。
\*    マージしたあとは head も設定も動かず、ゲートの答えも変わらないので、マージした状態で確かめる
R8_AutoMergeOnlyApproved == autoMerged => (gate = "approve" /\ ~moved /\ mergeAuto)

====
