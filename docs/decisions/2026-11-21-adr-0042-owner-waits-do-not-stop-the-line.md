# ADR-0042: オーナー待ちが、待っていない仕事を止めないようにする

- 日付: 2026-11-21
- 出典: ADR-0042（[adr/0042-owner-waits-do-not-stop-the-line.md](../adr/0042-owner-waits-do-not-stop-the-line.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-21「やはり私への質問待ちで Factory が半停止状態になっているのが気になります」）
- 覆す: [adr/0013-short-role-names-and-plain-japanese.md](../adr/0013-short-role-names-and-plain-japanese.md) 決めたこと 6 が `ask-one-by-one` skill に置いた「選択肢で答えられる問いはダイアログ」（一部。Chief of Staff について。Chief Engineer はダイアログのまま）と、オーナーが 2026-11-20 に決めた「並行の上限は `doing` と `review` の担当の数」の数え方（上限の値は変えない）

Chief of Staff は、どの問いも返事の本文に書き、答えを待たずにターンを終える。起動行で、ダイアログの道具を使えなくする。オーナー待ちを、マージの依頼・判断・手作業の 3 種に定め、読むだけの command `andon waiting` の 1 つで、オーナー待ちと並行の枠を数える。並行の枠は、オーナー待ちの課題を数えない。オーナー待ちが上限の数に達したら、新しい課題を起動しない。マージ待ちの後ろに並べた課題は先に起動せず、待ちが止めている課題の数を見せて、多い順に出す。Herdr App の右のパネルに、オーナー待ちのペイン（mod `waiting`）を足す。判断の問いには、ペインから選択肢で答えられる。端末の外から書かれた答えをオーナーの答えとして扱い、戻せない決断はペインから答えられない。答えは、dispatcher が通知 `answered` で Chief of Staff に届け、Chief of Staff が担当へ渡す。マージは GitHub のままにする。mod `kanban-board` に、ガントチャートの形の表示 Flow を足す。左にプロジェクト・マイルストン・親の課題・課題の木を、右に順番を横軸にした棒を置く。Chief Engineer の文書から切った課題は、もとの Chief Engineer の課題に `parent` で結ぶ。マイルストンの使い方と、課題どうしの関連の欄は、次の題に残す。中身・理由・捨てた案は ADR が正。
