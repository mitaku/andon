# ADR-0054: gate は、オーナーのマージ待ちの PR に赤い印を付けない

- 日付: 2026-11-22
- 出典: ADR-0054（[adr/0054-gate-status-by-decision.md](../adr/0054-gate-status-by-decision.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元はオーナーの発言 2026-11-22「CIのgateのステータスチェック、ルールでgateが通らない場合にFAILになると、オーナーにはFAILでメールの通知が行ってしまい、テストで落ちているのか判断がつかないということを感じている」）

`andon gate` は、判定がオーナー待ち（`needs_human`）のとき、PR の head の commit に印（commit status `andon/gate`）を書かず、PR のコメントだけを書く。通したとき（`approve`）の緑と、担当への差し戻し（`fix`）の赤は今のまま書く。同じ commit に前の判定の印が残っているときだけ、オーナー待ちを `pending` で上書きする。印の説明文は、見出しに最初の理由を括弧で足す。印には、理由を書いた PR のコメントへのリンクを付ける。印は commit status のままで、名前も変えない。gate のルールと判定、GitHub のメールと Actions の設定、今ある PR の印は変えない。印の決まりの正は `docs/contract.md` に置き、オーナー向けの読み方を `docs/operate.md` に書く。調べると、GitHub の失敗のメールは Actions の実行が落ちたときだけ届いていて、gate の印からは出ていなかった。gate が判定する前のマージを止めることは、この ADR で扱わず、別のテーマにする。中身・理由・捨てた案は ADR が正。
