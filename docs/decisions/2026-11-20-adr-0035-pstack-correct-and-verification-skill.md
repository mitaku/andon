# ADR-0035: pstack の correct と create-verification-skill は、足りない所だけを Andon の仕組みに移す

- 日付: 2026-11-20
- 出典: ADR-0035（[adr/0035-pstack-correct-and-verification-skill.md](../adr/0035-pstack-correct-and-verification-skill.md)。Chief Engineer の課題。オーナーが対話の中で決めた。元は人の発言 2026-11-20「PStackのcorrect, create-verification-skillsを活用したらもっとAndonの開発サイクルが改善しないかな？」）

pstack の skill をそのまま工程には入れず、Andon に無い所だけを移す。同じ誤りを見つけて直すループは振り返りの1本のままにし、`correct` からは3つを借りる。`AGENTS.md` を「決まり｜守らせるもの」の対にして検査すること、守らせるものの無い今の行を一度だけ棚卸しすること、繰り返しを止めるために検査を足す直しは誤りを戻すと落ちることを記録で示すこと。確かめる側は、走らせた記録を台帳に残す command を足し、`andon issue ready` とゲートは報告の行が記録を指すときだけ中身を見る。依頼書が名指す受け入れ条件の行が報告に無ければ command が断る。「セルで実測」は repo の中の検証の skill にし、マージした変更をセルへ当てたあと、台帳の課題として起動する独立の担当が、差分に当たる機能だけを本物で動かす。地図はその担当が直し、定期の保守は足さない。`create-verification-skill` は検証の skill を作る課題が手順として読むだけにし、ADR-0021 の「やらないこと」をその分だけ改める。検査の例外の書き方はこの題で決めない。見る日は 2026-11-27。
