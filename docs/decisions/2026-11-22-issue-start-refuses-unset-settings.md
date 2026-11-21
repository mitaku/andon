# andon issue start は、予約キーが揃っていない課題を起動せずに断る

- 日付: 2026-11-22
- 出典: Chief of Staff の依頼（2026-11-22）。11月の3日間で少なくとも3件、予約キーが無いか前のアドレスの Chief of Staff が書いた課題で、担当が実装を終えたあと `andon issue review-request` か `ready` を断られ、Chief of Staff が気づくまで止まった

- `andon issue start` は、起動する前に予約キー（[contract.md](../contract.md) §2）を確かめる。要るキーが無い・知らない値か、最後に書いたのが今の Chief of Staff の端末でない（gate・dispatcher と同じ履歴の照合）なら、何も作らずに終了コード 1 で断る。`--dry-run` も同じ理由で断る。
- 要るキーは課題の型で決める。`review` と `estimate` はどの担当の課題にも要る。`must_fix` と `merge` は、PR を出す課題（repo があって `deliverable` が `candidates` でない。dispatcher が gate を回す課題と同じ）だけに要る。`reviewer` と `review_rounds` は、そのうえ `review` が `off` でない課題だけに要る。`off` の課題でも、値があれば形と照合を確かめる（差分がレベル B・C なら dispatcher はレビュアーを起動し、値のある `reviewer`・`review_rounds` に照合を求めるので、通すと担当がレビューの段で止まる）。`review` が `off` でも gate は `must_fix` を読むので、PR を出す課題には求める。
- Chief Engineer の課題（profile `chief-engineer`）は見ない（予約キーを置かない）。レビュアーの課題は、今までどおりこの確かめの前に断る（起動するのは dispatcher だけ）。
- 断るときは、キーの名前と、Chief of Staff の端末で打てば直る `kanban set` の行を最後に出す。照合が外れたキーは今の値で書き直す行にする。無い・知らない値のキーは、値を選ぶ場所（`'review=<on|off>'` の形）を残す。コマンドは値を決めず、何も書かない（値を決めるのは Chief of Staff）。
- 変えないもの: gate・dispatcher・`review-request`・`ready` の読み方。Chief of Staff が起動し直したあと予約キーを書き直す段（`package/skills/chief-of-staff/playbooks/wake.md`）は残す。起動したあとで Chief of Staff のアドレスが変わった課題は、この確かめでは捕まらない。
