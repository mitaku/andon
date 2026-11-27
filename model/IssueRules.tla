---- MODULE IssueRules ----
\* 課題の一生が守る規則（ADR-0025 決めたこと 5）。行動は IssueLife.tla。規則を弱める・消す変更はオーナーが見る。
EXTENDS IssueLife

TypeOK ==
    /\ status \in {"todo", "doing", "review", "done"}
    /\ worker \in {"none", "live", "gone"}
    /\ wt \in {"none", "here", "removed"}
    /\ pr \in {"none", "open", "closed", "merged"}
    /\ gate \in {"none", "approve", "fix", "human", "behind"}
    /\ reviewing \subseteq {"head", "old"}
    /\ ci \in {"none", "running", "failed", "passed"}
    /\ prevTold \in BOOLEAN
    /\ result \in {"none", "old", "clean", "mustfix"}
    /\ headSeen \in BOOLEAN /\ dirty \in BOOLEAN
    /\ fixSent \in BOOLEAN /\ told \in BOOLEAN
    /\ q \in BOOLEAN /\ cancel \in BOOLEAN /\ mergeAuto \in BOOLEAN
    /\ moved \in BOOLEAN /\ autoMerged \in BOOLEAN
    /\ ans \in {"none", "cos", "pane"} /\ news \in {"none", "pending", "read"}
    /\ sent \in BOOLEAN /\ twice \in BOOLEAN
    /\ mark \in {"none", "ok", "red", "held"} /\ hold \in BOOLEAN /\ required \in BOOLEAN
    /\ rel \in BOOLEAN /\ badMerge \in BOOLEAN
    /\ base \in {"main", "prev"} /\ prev \in {"none", "waiting", "merged", "gone"}

\* R1 やめると決めた課題で、担当がまた動き出すことはない
R1_NoWorkerAfterCancel == cancel => worker # "live"

\* R2 gate が直しを返したら、担当か Chief of Staff のどちらかが必ずそれを知る
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
\*    取り消しは ChiefOfStaffCancels と、gate を待たないオーナーの先回りのマージ（HumanMergesEarly）。
\*    「行ける道がある」は状態 1 つの式（INVARIANT）では書けないので、検査器の状態の図を model/reach.mjs が読む。
\*    設定は IssueRules.cfg の「\* REACH」の行（R7_MergeWithoutCancel pr = "merged" と除く行動）。
\*    閉じた PR を開き直す道（PRClosesOrReopens）も含めて成り立つ（ADR-0034 決めたこと 3）。

\* R8 dispatcher が自動でマージするのは、gate が approve と判定した head で、その時点で任されているものだけ（ADR-0034 決めたこと 3）。
\*    マージしたあとは head も設定も動かず、gate の答えも変わらないので、マージした状態で確かめる
R8_AutoMergeOnlyApproved == autoMerged => (gate = "approve" /\ ~moved /\ mergeAuto)

\* ---- ペインから答える口の規則（ADR-0042 決めたこと 19） ----
\* R9 担当に届く答えは、だれかが書いた答えだけ。ペインの答えは、Chief of Staff が通知を読んでから届く
R9_OnlyWrittenAnswersReachWorker == sent => (ans = "cos" \/ (ans = "pane" /\ news = "read"))

\* R10 1 つの問いに、答えは 1 つだけ（会話の答えとペインの答えが重ならない）
R10_OneAnswerPerQuestion == ~twice

\* R11 答えの無い問いは、担当に届いたことにならない
R11_NoRelayWithoutAnswer == (q /\ ans = "none") => ~sent

\* ---- gate の印と止めの規則（ADR-0056 決めたこと 17） ----
\* R12 ruleset が印を必須にしていれば、gate が渡していない head も、止めている間の PR も、マージされない（オーナーのマージも dispatcher のマージも）
R12_NoMergeBeforeGate == required => ~badMerge

\* R13 緑の印が付いているのは、gate がその head を approve か needs_human と判定していて、止めていないときだけ
R13_GreenOnlyWhenReleased == (pr = "open" /\ mark = "ok") => (rel /\ ~hold)

\* R14 dispatcher は、止めている PR を自分でマージしない（ruleset が印を必須にしていなくても）
R14_NoAutoMergeWhileHeld == autoMerged => ~badMerge

\* ---- 前の課題の PR の branch の上で起動する課題の規則（ADR-0067 決めたこと 16） ----
\* R15 前の課題の branch に載ったままの PR は、マージされない（オーナーのマージも dispatcher のマージも）
R15_NoMergeWhileStacked == pr = "merged" => base = "main"

\* R16 前の PR 待ちと判定した PR には、緑の印が付かず、マージを渡したことにもならず、オーナーにマージを頼まない
R16_BehindIsNotReleased == gate = "behind" => (mark # "ok" /\ ~rel /\ ~told)

\* R17 gate が approve と判定するのは、既定の branch に載った PR だけ
R17_ApproveOnlyOnMain == gate = "approve" => base = "main"

\* R19 前の課題の branch に載ったままの head には、緑の印が付かない（積んでいる間、gate は approve もオーナー待ちも決めない）
R19_NoGreenWhileStacked == base = "prev" => mark # "ok"

\* R18 前の課題がマージされずに終わる場合を除き、届くどの状態からも、オーナーの判断も、だれかの PR への push も、Chief of Staff の止めも
\*     借りずに、既定の branch に載った形まで行ける（前の PR が main に入れば、gate が判定をやり直して担当に載せ替えを頼む）。
\*     IssueRules.cfg の REACH の行。除くのは、やめる・先回りのマージ・前の課題が終わる・オーナーの答えで担当へ返す 2 つ・判定のあとの push・止める
\* R20 前の課題がマージされずに終わっても、やめる口も、だれかの PR への push も、Chief of Staff の止めも借りずに、既定の branch に
\*     載った形まで行ける（gate が Chief of Staff に知らせ、オーナーの答えで Chief of Staff が担当へ返し、担当が載せ替える）。
\*     IssueRules.cfg の REACH の行。除くのは、やめる・先回りのマージ・gate の問いへの答えで担当へ返す・判定のあとの push・止める

\* R21 知らせるのは、前の課題がマージされずに終わったあとだけ
R21_ToldOnlyWhenGone == prevTold => prev = "gone"

====
