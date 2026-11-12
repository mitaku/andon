# ADR-0019: 人の書きかけに `<news>` を混ぜない（仕様・設計。2026-11-11。Chief Engineer の課題 A-0128）

- 状態: **提案（2026-11-11）**。人が 2026-11-11 に「決めた」と言った（Chief Engineer の対話 A-0128）。人が PR をマージした日が決定の日（[decisions/2026-11-11-adr-0019-cos-input-box.md](../decisions/2026-11-11-adr-0019-cos-input-box.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: dispatcher が Chief of Staff の入力欄へ打つ口（経路③の `<news>` と、提供元に切られたときの「続けて」）を、人の書きかけに混ざらない形にすること。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。
- 入力: 依頼書 A-0128（人の発言 2026-11-11 と Chief of Staff の経緯・追記）、A-0127 の報告と候補（c1〜c4）、[dispatcher.md](../dispatcher.md) §2.4・§3〜§4.4、`internal/dispatcher/deliver.go`・`pickup.go`・`drain.go`、`package/hooks/`、A-0078 の実測の行、[ADR-0010](0010-news-ack.md)。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-11）: 「打ってる途中に入って勝手に送信された気がします」。

言い直し（人が同意 2026-11-11）。目的は2つ。人が Chief of Staff の入力欄に打った文は、人が Enter を押したときに、人の文だけで送られること。そして、人が離れていて Chief of Staff も待っている間に起きた通知で、Chief of Staff を起こす口は残すこと（誰も起こさないとラインが止まる）。困りごとは、いま離席中に起こす口（経路③）が入力欄に `<news>` を貼って Enter を押す形であること。`hc send` は人の手が止まるのを待ってから書きかけに続けて送り、打った側からは取り消せないので、dispatcher が直前に何度確かめても、確かめたあとに人が打ち始めれば書きかけごと送られる。A-0127 の直し（窓を縮める）ではゼロにならない。

同じ題の材料（依頼書の追記）:

- 同じ通知が2回届く（`hc send` で届いた分が同じターンの終わりの Stop でもう一度〈3件〉、Stop で渡した分が次の人の送信でもう一度〈2件〉）。
- Stop の block で渡すと、人の画面に「Stop hook error」と出て失敗に見える（人 2026-11-11「StopHook Errorがでてるかも」）。

事実（人に聞かず読んで・測って確かめたもの）:

- `hc send`（0.1.17）の選択肢は `--if idle|working`・`--force`・`--wait` だけ。待たない・入力欄が空のときだけ、の条件は無い。人が打っている間は手が止まるまで待ち、書きかけに続けて貼って Enter を押す。CLI を殺しても送りは取り消されない（A-0127 の再現）。
- Chief of Staff の入力欄へ dispatcher が打つ口は2つ: `<news>`（`deliver.go`）と「続けて」（`pickup.go` の `retryIfEmpty`）。担当・レビュアーへ打つ4つ（最初のメッセージ・その送り直し・「続けて」・`fix`）は人が打つ端末ではない。
- 同じ「書きかけに貼り付いて送られた」事故は 2026-11-03 にも起き、そのときの直しが「配達役を1つにして、入力欄が空のときだけ送る」（decisions.md §6 の D22 の写し）。今回はその守りに残った窓。
- **`asyncRewake` のフックは、入力欄を通らずに idle の Claude Code を起こす**（実測 2026-11-11・Claude Code 2.1.322・捨てる試しの端末）。Stop フックに `asyncRewake: true`・40 秒待って stderr に文を書き exit 2 する command を置いた。ターンが終わって入力待ち（hc の状態 `done`）の間に入力欄へ `kakikake` と打ちかけたところ、40 秒後に新しいターンが始まり（hc の `turn` 1 → 2）、モデルが stderr の文に答えた。書きかけ `kakikake` は入力欄にそのまま残った。画面の表示は「Stop hook feedback」。公式文書（hooks の Limitations）にも「`asyncRewake` のフックが exit 2 で終わると、idle でもすぐ Claude を起こす」とある。`timeout` は `asyncRewake` のフックにも効く（command の既定は 600 秒）。
- **Stop フックは `hookSpecificOutput.additionalContext` で返すと「Stop hook feedback」と出て、エラーの通知は出ない**（公式文書の Stop decision control。`decision: block` と同じく続きのターンになり、`stop_hook_active` と連続 8 回の上限も同じ）。
- **ほかの製品**（2026-11-11 に公開の文書・コード・issue を読んだ。実行はしていない）:
  - 端末の入力欄へ貼り付けて届ける製品は、この問題を解けていない。shepherdr 3.1（`mods/tasks/bin/herdr_task/news.py`）は今の Andon と同じ形（画面を読んで dim を除き、書きかけがあれば送らず次の回へ。送りは `hc send`）で、確かめと送りの間の隙間は `hc send` の待ちに頼る。0.2 の実測の記録に「書きかけに続けて送られた」があり、世代ごとに守りを変えてきた（60 秒 idle かつ入力欄が 10 秒空 → idle のときだけ → 書いている間は貯める）。提供元に切られたときの「続けて」も同じ口。Gas Town は同じ事故を issue に記録し（#1216・#4610）、消して復元する案を断念して「入力欄と同じ口を使う限り解けない。要るのは入力欄でない横の口」と結論している。Tmux-Orchestrator は守りが無い。
  - 解けている製品は、入力欄を通らない口を持つ。opencode（HTTP の `prompt_async`）、OpenAI Codex の複数エージェント（プロセスの中で親へ注入）、vibe-kanban（自前の画面で下書きと待ち行列が別）、Claude Code 自身（agent teams の mailbox・クロスセッションのソケット・channels・`asyncRewake`）。shepherdr 3.x は人が chat の画面から書くとき、未送信の字がある間だけ端末にラベルを付けて機械の送りを止める（人が端末へ直に打つ分は画面読みに戻る）。
  - 機械は打たず、一覧と通知で人に知らせるだけの製品もある（claude-squad・CCManager）。離席中にラインを動かすことは目的にしていない。
  - クロスセッションのソケットは公式文書に「script や hook から投稿できる」とあるが、載っているのは認証の1行の形だけで、メッセージの行の形は無い。
- **`timeout` は既定を超えて延ばせる**（実測 2026-11-11）。`timeout: 7200` を置き、660 秒待ってから exit 2 する待ち役は、11 分後に同じようにセッションを起こした。置ける上限は公式文書に無い。
- 同じイベントのフックは並んで走る（公式文書）。Stop の `drain` と、同じ Stop で起きる待ち役は同時に動く。Stop フックが返した続きのターンには UserPromptSubmit は来ない。API エラーで終わったターンは Stop ではなく StopFailure が走り、その出力と終了コードは無視される。
- 未実測: 働いている最中（ターンの途中）に待ち役が exit 2 したときの届き方。`timeout` に置ける上限。`timeout` で切られた・0 と 2 以外で終わった背景のフックが画面にどう出るか。セッションを起こし直した（`--resume`）あとに古い待ち役が残るか。`error`（提供元に切られた）で止まったセッションを起こせるか。`additionalContext` で続けたターンが hc の `turn` を進めるか（`decision: block` は進めない。contract.md §9）。

## 前提（同意済み。番号）

1. **2026-11-11** この事故は、回数を減らすのではなく、構造として起きない形にする（窓をゼロにする）。人の文が人の Enter なしに送られると、まだ決めていない言葉が Chief of Staff への指示になる。窓を縮める直し（A-0127）は足しにしかならない。
2. **2026-11-11** Chief of Staff が提供元に切られて止まったときの「続けて」も、同じ窓があるので、この題の範囲に入れて同じ基準で塞ぐ。
3. **2026-11-11** hc への相談はしない（人の言葉）。hc は外部の部品で Andon からは直さない。塞ぎ方は Andon の中だけで閉じる。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-11 経路③を、入力欄へ打つ `hc send` から、`asyncRewake` のフックの「待ち役」に替える。dispatcher は Chief of Staff の入力欄へ `<news>` を打たない。** 経路①（Stop）・②（UserPromptSubmit）は今のまま残す。
   - 待ち役は Chief of Staff の Stop フックが `asyncRewake: true` で起こす command（仮の名 `andon dispatcher wait`）。届ける通知（dispatcher.md §3.2 の「届ける」）が待ちに出るまで待ち、出たら exit 2 で終わる。Claude Code が入力欄を通らずに新しいターンを始める。（最初は「待ち役が待ちを取り出して `<news>` を stderr に書く」としたが、同じ日に 8 で「合図だけ」に改めた。）
   - Chief of Staff の端末（ラベル `role=chief-of-staff`）でなければ、待たずに exit 0（`drain` と同じ見分け。managed settings のフックは担当の端末でも走る）。
   - 落ちたとき（台帳・控えが読めない）は exit 0 で通す。待ちは控えに残り、次の口（①②）で届く。
   理由: 入力欄を通らないので、人の書きかけに混ざる窓が構造として無い（前提 1。実測で書きかけが残った）。離席中も Chief of Staff を起こせる（言い直しの目的の後半）。実測で合格している①②に手を入れない。貼り付けで届ける製品はどれも窓が残り、解けている製品は入力欄を通らない口を使っている（事実の「ほかの製品」）。
   捨てた案:
   - A「経路③を消すだけ」（窓はゼロだが、人が離れている間は通知が届かずラインが止まる）。
   - C「①もやめ、通知の口を待ち役の1本に寄せる」（2回届く件が構造で消えるが、未実測の点に通知の全部が乗り、実測で合格した①を捨てる。待ち役を回した実績を見てから、要るなら別の題）。
   - hc に条件を足してもらう（前提 3）。
   - dispatcher がクロスセッションのソケットへ直接書く（メッセージの行の形が公式文書に無く、非公開の形式に乗る）。
   - 人と話す Chief of Staff と通知を受ける Chief of Staff を分ける（ADR-0010 の捨てた案 1 と同じ。新しい役を足さない）。
   - 人が端末でなく別の画面（chat の mod）から書くようにし、書いている間は機械を止める（shepherdr 3.x の形。mod を新しく作る大きさで、人が端末へ直に打てば窓が戻る）。

2. **2026-11-11 待ち役の持ち方（Chief Engineer の推奨を人が受けた）。→ 同じ日に 8 で改めた（反論 R2〜R6）。**
   - 待ち役はセッションに1つだけ。重ねて起こさない（もう居れば、後から来たほうが exit 0）。
   - 人が送信したら（UserPromptSubmit のフック）待ち役を止め、そのターンの終わりの Stop で起こし直す。待ち役が生きているのは、Chief of Staff がターンを終えて入力を待っている間だけ（働いている最中に待ち役が起こす場面を作らない。未実測の届き方に乗らない）。
   - フックの `timeout` は置ける範囲でいちばん長くする（値は実装の課題が実測で決める）。セッションの始まり（SessionStart フック）でも待ち役を起こす。
   - 待ち役が渡した通知は、①②と同じ控え（渡した turn・端末・通知）に書いてから渡す。`andon news ack` と `news.replied` の照らし方（ADR-0010 決めたこと 4・5）、渡した turn の終わりを見ないまま別の turn が来たときの1回だけの届け直しも①②と同じ。

3. **2026-11-11 待ち役がいない間（`timeout` で切れた・人が Esc でターンを止めた・セッションを起こし直した直後）に起きた通知は、届かないことを受け入れる。→ `timeout` の分は同じ日に 9 で改めた（反論 R1）。入力欄へ打つ口も人を呼ぶ口も足さない、は変わらない。** 人が次に打ったとき（②）か次のターンの終わり（①）に届く。dispatcher は「届ける待ちがあるのに待ち役がいない」をログに残すだけで、入力欄へ打つ口も、人を呼ぶ口も足さない。
   理由: 隙間はどれも人が戻れば埋まる。通知は控えに残り失われない。埋めるために `hc send` を残せば窓が戻る（前提 1）。
   捨てた案: 待ち役がいないときだけ `hc send` で送る（窓が残る）／待ちが続いたら Herdr の通知で人を呼ぶ（dispatcher.md §6「人を呼ばない」を変える）。

4. **2026-11-11 Chief of Staff への「続けて」（提供元に切られたとき。`retryIfEmpty`）の `hc send` をやめる。** Chief of Staff が提供元に切られて `error` で止まったら、dispatcher は打たずにログに1行残す（今の「5 回目に切られたとき」と同じ扱い。人は Herdr App で Chief of Staff の `error` に気づく）。担当・レビュアーへの「続けて」は今のまま（人が打つ端末ではない）。
   - 並行して、待ち役（か StopFailure のフック）から `error` で止まったセッションを入力欄を通らずに起こせるかを、調べる課題で測る。通れば、その口から「続けて」を足す（この文書の「決めたこと」に下から足す）。
   理由: 窓がすぐゼロになり、未実測のものに決定を乗せない。失うのは、離席中に Chief of Staff だけが切られたときの自動の立ち直り（Claude Code 自身の約 3 分のやり直しは残る）。
   捨てた案: やめるだけで測らない／測るまで今の `hc send` を残す（その間は窓が残る）。

5. **2026-11-11 1 と 4 が入ったら、Chief of Staff の入力欄を読む仕組み（`inputEmpty`・`observe`・`screen.go` の離席の判定。dispatcher.md §4）は読み手がいなくなるので消す。** dispatcher.md §4 は「経路③の守り」から「待ち役」の節に書き直す。A-0127 の直し（窓を縮める）は、1 と 4 が入るまでのつなぎ。

6. **2026-11-11 経路①（Stop）の渡し方を、`{"decision": "block", "reason": …}` から `{"hookSpecificOutput": {"hookEventName": "Stop", "additionalContext": …}}` に替える。** 続きのターンになるのは同じで、画面の表示が「Stop hook feedback」になり、エラーの通知が出ない（人には失敗に見えていた）。担当の Stop フック（`andon issue stop-check`。報告も問いも無いまま止まろうとした）は block のまま（そちらは止めることが役）。

7. **2026-11-11 同じ通知が2回届く件は、直しとして扱う。** `hc send` で届いた分が同じターンの終わりの Stop でもう一度届く形は、1 で `hc send` が消えるので無くなる。Stop で渡した分が次の人の送信でもう一度届く形は、原因を調べて直す課題にする。待ち役は通知を運ばない（8）ので、2度渡す筋を足さない。
   - 調べる手がかり（反論 R8。コードから）: 続きのターンの終わりの Stop の `drain` が、見回りの持つ控えの lock を 10 秒待って諦める（`drain.go` の `lockState`。見回りは lock を巡の終わりまで持ち、その中でゲート・finish・`hc send` を打つ）と、渡した turn の印（控えの `Hook`）が消えずに残り、次の UserPromptSubmit が「終わりを見ないまま別の turn が来た」として届け直す。

8. **2026-11-11 待ち役は合図だけを打つ。通知を取り出して渡すのは `drain` の1人のまま（2 を改める）。**
   - 待ち役は控えの待ちを取り出さない・控えに書かない。届ける通知が待ちにあり、Chief of Staff の端末の状態が `idle` か `done` のときに、固定の1行（「通知が待っている。これは合図で、人の依頼ではない。何も書かずにターンを終える」の旨。文は実装の課題）を stderr に書いて exit 2 する。起こされたターンの終わりの Stop で、今の経路①（`drain`）が `<news>` を渡す。`andon news ack`・`news.replied`・届け直しの照らし方は①のまま変えない。
   - 待ち役は人の送信で止めない。ターンをまたいで生き、Chief of Staff が働いている間は打たずに待つ（そのターンの終わりに①が渡す）。合図に中身が無いので、働いている最中に届いても・人の送信と重なっても、通知は2度届かず、失われもしない。
   - 新しい Stop が来たら、新しい待ち役が古い待ち役と替わる（古いほうは exit 0 で静かに終わる。端末に1つ。`timeout` の時計はターンごとに巻き直る）。SessionStart では起こさない（実測の外で、最初の応答を塞ぐ疑いがある）。起こしたばかりのセッションは、最初のターンが終わるまで待ち役がいない。
   - 払うもの: 離席中に起こすたび、中身の無い短いターンが1回増える。
   理由（反論で証明されたこと）: 待ち役が自分で `<news>` を運ぶと、待ちを取り出す者が `drain` と待ち役の2人になる。同じ Stop のフックは並んで走るので、①が続きのターンを返しても待ち役は生きていて、同じ待ちを取り合う（2度届く・確かめの印が早く消える）。待ち役が起こしたターンの終わりを今の照らし方（`sameTurn` の `stop_hook_active`・`prompt_id`）では見分けられない。人の送信と exit 2 が重なると、2度届くか取りこぼすかの二択になる。
   捨てた案: 待ち役が `<news>` を運ぶまま、取り合い・照らし方・重なりを実装で解く（余分なターンは無いが、未実測の点に乗る）。

9. **2026-11-11 待ち役は `timeout` が切れる前に張り直す（3 の `timeout` の分を改める）。** `timeout` は長く置く（例: 12 時間。値は実装の課題が実測で決める）。切れる少し前まで打つ機会が無ければ、待ち役は「張り直し」の合図で exit 2 し、Chief of Staff は何も書かずに終わり、その Stop で次の待ち役が起きる。人が何日離れても待ち役が居続ける。払うものは、静かな間 `timeout` ごとに短いターンが1回。
   - 残る隙間（3 のとおり受け入れる）: 起こしたばかりのセッションの最初のターンが終わるまで。Claude Code ごと落ちた・起こし直したあと、次のターンが終わるまで。どちらも人が打てば埋まる。
   理由: 張り直さないと、「離席中も起こせる」（言い直しの目的の後半）が `timeout` の長さまでしか成り立たない。担当が `timeout` より長く働くのは離席のふつうの形（反論 R1）。
   捨てた案: 張り直さない（余分なターンはゼロだが、`timeout` を過ぎた通知は人が戻るまで届かない）。

10. **2026-11-12 提供元に切られて `error` で止まった Chief of Staff は、StopFailure の `asyncRewake` のフックが合図で起こす（4 の続き。W6 の実測 A-0135 の形 B）。dispatcher の「続けて」（`hc send`）は戻さない。**
   - フックは `andon dispatcher rewake`（`package/hooks/stop-failure`）。入力の `error` が `server_error`・`rate_limit` で、端末が Chief of Staff（ラベル `role=chief-of-staff`）なら、続けて切られた 1〜4 回目はそれぞれ 1・2・4・8 分あとに合図の1行で exit 2。5 回目は起こさず exit 0 とログ1行。ほかの `error`・Chief of Staff でない端末は待たずに exit 0（担当・レビュアーへの「続けて」は dispatcher のまま）。
   - 回数は控えの横に持ち、Chief of Staff の Stop（ターンが通った）で数え直す。待ったあと、端末が `error` のままで回数も変わっていない（待つ間に人が打ったターンが通っていない・また切られていない）ときだけ合図する。
   - 合図は中身を運ばない（8 と同じ）。起こしたターンには切られたユーザー文がそのまま入るので、合図だけで「続けて」の代わりになる（A-0135）。待ち役と同時に exit 2 しても、`<news>` を渡すのは `drain` の1人なので2度届かない。Stop の待ち役は変えない。
   理由: 入力欄を通らずに起こせることを実測した（A-0135。書きかけも残る）。StopFailure は落ちるたびに起き、`error` の種類で見分けが付き、画面を読まずに済む。
   捨てた案: 形 A（生きている待ち役の exit 2。1 つの Stop で1回しか起こせず、最初のターンが落ちたときは待ち役がいない）／dispatcher の「続けて」を Chief of Staff に戻す（入力欄の窓が戻る）。

## やらないこと（範囲の外。理由）

- **hc に条件を足してもらうこと・hc の持ち主への相談**（A-0127 の候補 c2）。前提 3。
- **通知の口を待ち役の1本に寄せること**（代案 C）。決めたこと 1 の捨てた案。
- **待ち役がいない間を埋める口**（`hc send` の控え・人を呼ぶ通知）。決めたこと 3・9。
- **SessionStart で待ち役を起こすこと**。決めたこと 8。
- **担当・レビュアーへ打つ口**（最初のメッセージ・その送り直し・「続けて」・`fix`）。人が打つ端末ではないので今のまま。Chief Engineer の端末は dispatcher が打たない（今も同じ）。
- **Codex の Chief of Staff**。Chief of Staff は最初の版では Claude Code だけ（contract.md §9）。`asyncRewake` は Claude Code のフックの機能。
- **`<news>` の文・届ける／記録だけの線・`andon news ack`**（ADR-0010）。変えない。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は3つ。新しい継ぎ目は作らない。

1. **`tests/andon-dispatcher.sh`（試しの台帳と偽の hc。既にある）と `internal/dispatcher` の Go のテスト**:
   - 届ける通知が待ちにあり、Chief of Staff の端末が `idle`・`done` のとき、待ち役が合図の1行を stderr に書いて終了コード 2 で終わる。待ちと控えは変わらない（待ち役は取り出さない・書かない）。続く Stop の `drain` がその通知を1回だけ渡す。
   - 待ちが無い間・Chief of Staff が `working` の間は終わらない。`role=chief-of-staff` でない端末では、待たずに終了コード 0。
   - 待ち役が居る端末で次の待ち役が起きたら、古いほうが終了コード 0 で終わり、新しいほうが残る（止められた待ち役の終了コードは 0。0 と 2 以外で終わらない）。
   - `timeout` の少し前まで打つ機会が無ければ、待ち役が張り直しの合図で終了コード 2 で終わる（テストでは短い値を渡す）。
   - Stop で渡して確かめ済みになった通知は、次の UserPromptSubmit でもう一度出ない。終わりを見ないまま別の turn が来たときの1回だけの届け直し（今のテスト 12b）は残る。見回りが控えの lock を持っている間に Stop の `drain` が来ても、確かめの印が残って届け直しにならない（決めたこと 7 の手がかりの再現）。
   - dispatcher の見回りが、Chief of Staff の端末に対して `hc send`・`hc attach --observe` を一度も呼ばない（偽の hc が呼ばれたら落とす）。Chief of Staff が `error`（提供元に切られた）のときも `hc send` を呼ばず、ログに1行。担当・レビュアーへの「続けて」は今までどおり呼ぶ。
   - Chief of Staff の Stop の `drain` の出力が `hookSpecificOutput.additionalContext` で、`decision` のキーが無い。担当の `stop-check` は `decision: block` のまま。
2. **文書と約束（`tests/public.sh`・`tests/package.sh`・schema のテスト）**: `package/settings.json` の Stop に `asyncRewake` のフック（`timeout` つき）がある。contract.md §9・dispatcher.md §2.4・§3・§4・§7 の表・chief-of-staff skill が新しい形を書いている。`tests/run --changed` green。
3. **セルで実測（本物の Claude Code。報告に「見た日・結果」を1行ずつ）**:
   - Chief of Staff の入力欄に書きかけを残したまま、担当の報告で通知を起こす → Chief of Staff が新しいターンで受け、書きかけはそのまま残る。
   - 画面に「Stop hook error」が出ない（①も待ち役も「Stop hook feedback」）。
   - 待ち役が起こしたターンの終わりに `<news>` が1回だけ届き、次の待ち役が起きている（続けて2件目も届く）。
   - Chief of Staff が働いている最中に通知が起きても、ターンの途中に合図が入らず、ターンの終わりに①で届く。
   - 置けた `timeout` の値と、張り直しの合図で待ち役が替わること。人が Esc で止めたあと・送信を Esc で取り消したあとも待ち役が居ること。`timeout` で切られた・止められた待ち役が画面に「hook error」を出さないこと。
   - `additionalContext` で続けたターンが hc の `turn` を進めるか（進むなら `andon news ack` の照らし方を合わせる）。
   - セッションを起こし直したあとに古い待ち役が残らないこと。
   - 1日回して、同じ通知が2回届いた件数（0 を期待）。

## 価値の仮説（P6 で形を決める）

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| W1 | この文書の PR（ADR-0019・`decisions/` の1ファイル） | `tests/run --changed` green。人がマージ | — |
| W2 | 経路①の渡し方を `additionalContext` に替える（決めたこと 6）。`drain`・contract.md §9・dispatcher.md §3 の表 | 受け入れ条件 1 の最後の項目。セルで「Stop hook error」が出ない | W1 |
| W3 | Stop で渡した通知が次の UserPromptSubmit でもう一度届く件を調べて直す（決めたこと 7） | 再現のテストが先に落ち、直して通る | W1 |
| W4 | 待ち役を入れ、経路③の `hc send` を消す（決めたこと 1・3・8・9）。`andon dispatcher wait`・`package/settings.json` と `package/hooks/`・`deliver.go`・contract.md §9・dispatcher.md §3・§4・§7 | 受け入れ条件 1（待ち役の項目）・2 | W2・W3（待ち役の配達は①に乗るので、①の表示と2度届く件を先に直す） |
| W5 | Chief of Staff への「続けて」をやめ、入力欄を読む仕組みを消す（決めたこと 4・5）。`pickup.go`・`screen.go`・dispatcher.md §2.4・§4 | 受け入れ条件 1 の `hc send`・`observe` を呼ばない項目 | W4 |
| W6 | 調べる課題: `error`（提供元に切られた）で止まった Claude Code を、生きている待ち役の exit 2 か、StopFailure のフックに付けた `asyncRewake` から、入力欄を通らずに起こせるか（偽の API。dispatcher.md §2.4 の実測と同じ道具） | 起こせる／起こせないと、その形の報告。起こせるなら Chief Engineer の課題へ戻さず、この文書の決めたこと 4 の続きとして直す課題に切ってよい | W4（待ち役が要る） |
| W7 | セルで実測（受け入れ条件 3） | 項目ごとに「見た日・結果」。落ちた項目は直す課題 | W4・W5 のマージ |

- A-0127（窓を縮める）は W4・W5 が入るまでのつなぎ。W5 がその直しの対象（`inputEmpty`）ごと消す。

## 開いている問い（残った問い。決断の種類つき）

- 無し（2026-11-11 の対話で閉じた）。未実測の点は受け入れ条件 3 と W6 が測る。測った結果が決めたことと食い違えば、Chief of Staff が Chief Engineer の題に戻す。

## 出典（読んだもの・捨てた案の出所）

- 依頼書 A-0128（経緯・追記）。人の答え 2026-11-11（言い直し・前提 1〜3・決めたこと 1〜9。前提 3 は人の言葉。塞ぎ方は「Shepherdr など同じようなことをしている製品がどうしているか調べてから」と言われ、調べを足してから選んだ）。
- A-0127 の `report.md`・`library/candidates.json`・範囲の問い 1 とその答え。A-0078 の依頼書の実測の行。
- [dispatcher.md](../dispatcher.md) §2.4・§3〜§4.4、`internal/dispatcher/deliver.go`・`pickup.go`（`retryIfEmpty`）・`drain.go`、`package/hooks/stop`・`user-prompt-submit`、[decisions.md](../decisions.md) §6（2026-11-03 の配達役の行）。
- Claude Code の公式文書（2026-11-11 に読んだ）: hooks（`asyncRewake`・Run hooks in the background の Limitations・Stop decision control）、cross-session messaging、channels、headless、agent view。
- 実測（2026-11-11。Claude Code 2.1.322・hc 0.1.17。捨てる試しの端末）: 上の「事実」の `asyncRewake` の行と `timeout` の行。
- ほかの製品（2026-11-11 に読んだ。public）: eliasstravik/shepherdr（main 3.1.31 と branch `legacy`・`legacy-herdr-projects`・`v2.0`・`v2.1`）、gastownhall/gastown（`internal/cmd/nudge.go`・issue #1216・#4610・PR #2497）、openai/codex（`codex-rs/core/src/agent/control.rs`）、opencode の server の文書、Jedward23/Tmux-Orchestrator、claude-squad、kbwo/ccmanager、BloopAI/vibe-kanban。
- 反論（2026-11-11。別の文脈の読むだけの読み手に「この設計を壊せ。証明できたものだけ返せ」）: R1〜R8。R1 → 決めたこと 9、R2〜R6 → 8、R7 → 受け入れ条件 1 の文、R8 → 決めたこと 7 の手がかり。
