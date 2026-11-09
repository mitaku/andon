# ADR-0009: Chief of Staff を人の既定の入口にする（CoS と Chief Engineer の skill）

- 日付: 2026-11-09
- 出典: ADR-0009（[adr/0009-cos-default-entry.md](../adr/0009-cos-default-entry.md)。Chief Engineer の課題 A-0008。人が対話の中で決めた）

人が話す相手は基本 Chief of Staff。CoS skill §1 の表の「判断」に当たれば Chief of Staff が自分で `andon chief-engineer --brief -` を打ち、経緯を依頼書で渡す（人が同じ話を2回しない）。Chief Engineer は1プロジェクトに同時に複数いてよい（WIP 制限を外す）。誤った振り分けは既存の口で両向きに戻す。依頼書の型に「repo を読めば分かること（作法・構造・直し方）は書かない」の原則1行と「範囲の外」。差し戻しは依頼書の追記で数え、2回で通らなければ依頼書を直した新しい課題に切る。wiki の手順（何を・いつ・形・lint）は Chief of Staff の skill の文で持ち、学びとオーナーの思考・志向を残す（template に `--add-dir /workspace/wiki`）。skill の行数の上限は置かない。Chief Engineer の skill には価値の仮説の席だけ。`andon issue stop-check` は Chief Engineer の課題を止めない。design.md §1.4・§1.8 を合わせて直す。決めたこと 11 件・切る案 S1〜S6 は ADR が正。
