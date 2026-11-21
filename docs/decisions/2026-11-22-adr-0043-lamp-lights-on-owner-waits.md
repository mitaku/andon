# ADR-0043: 行燈は、オーナーが呼ばれているときだけ灯す

- 日付: 2026-11-22
- 出典: ADR-0043（[adr/0043-lamp-lights-on-owner-waits.md](../adr/0043-lamp-lights-on-owner-waits.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-21「Andonのランプの状態をもう少し整理（Replyなだけでは灯らないようにしたい）」）
- 覆す: [adr/0015-inbox-needs-you.md](../adr/0015-inbox-needs-you.md) 決めたこと 1・3 の「役が終えたターンを Needs you に上げる」と、やらないことの「inbox が kanban を読むこと」。[adr/0023-sidebar-andon-look.md](../adr/0023-sidebar-andon-look.md) 決めたこと 1・3 の「Reply を琥珀で灯す」。[adr/0042-owner-waits-do-not-stop-the-line.md](../adr/0042-owner-waits-do-not-stop-the-line.md) のやらないことの「mod `inbox` の節と、行燈の灯し方を変えること」（一部。オーナー待ちの定義と、読む command と、右のパネルのペインは変えない）

行燈の琥珀は、端末が止まってオーナーを待っているときと、kanban にマージの依頼か判断のオーナー待ちがあるときだけ灯す。赤は、今までどおり、端末のターンが失敗したときである。Chief of Staff と Chief Engineer がターンを終えただけの返事（Reply）では灯さず、Needs you にも上げない。Projects の役の行に、未読の印を付ける。印は、オーナーが見ていない返事の全部に付け、Chief of Staff の申告のラベルは読まない。手作業のオーナー待ちでは灯さない。オーナー待ちは、左の列の Needs you の上に、プロジェクトごとの件数の 1 行で出す。mod `inbox` は、セルの `andon waiting --json --project <slug>` を呼んでオーナー待ちを読み、数える式を持たない。読めないプロジェクトでは、今までどおり Reply で灯す。灯りの状態は mod `inbox` の 1 か所で決め、`shared("inbox.lantern")` と `shared("inbox.lamp")` に置く。Chief Engineer は、本文の問いで答えを待ってターンを終える前に、その問いを kanban に置く。Stream Deck へ出す口は、あとの題に残す。
