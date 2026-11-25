# ADR-0063: 言語の道具を image に焼かず、案件の repo の mise.toml を正にして volume に置く

- 日付: 2026-11-25
- 出典: ADR-0063（[adr/0063-language-tools-on-volume.md](../adr/0063-language-tools-on-volume.md)。Chief Engineer の課題。元はオーナーの 2026-11-25 の依頼で、セルを作り直すたびに言語の道具が消えて手で入れ直していることを、セルの側のつなぎを作らずに Andon 本体で直す）

mise が入れた言語の道具は 553MB あり、コンテナの中にしか無いので、セルを作り直すたびに消えていた。宣言を満たす手を Andon の誰も持っていなかった。言語の道具のバージョンの正は案件の repo の `mise.toml` にし、image はバージョンを持たない。入れた道具は compose の volume `mise`（`~/.local/share/mise`）に置き、消えてもよいキャッシュとして扱う。宣言を満たす手（`mise install`）は、Andon が担当・レビュアー・Chief Engineer の端末を起動する直前に、その worktree で打つ。満たせなくても起動は続ける。gate の検査と `image/apply-main.sh` も、道具を使う前に同じ手を打ち、gate は満たせなければ判定しない。image の mise の既定の go の設定はやめ、`mise.toml` の無い所では、役の端末と gate の検査だけが、自分の worktree の `mise.toml` をいちばん弱い既定（`MISE_SYSTEM_CONFIG_FILE`）として引く。端末へは、プロファイルの行が通す葉 `andon tools env` が渡す。古いバージョンの掃除は自動でしない。

捨てた案:

- **プロファイルの行の中で宣言を満たす**: hc が役のプログラムの準備を待つ時間に食い込み、最初のメッセージが届かなくなる。レビュアーの cwd からは worktree が引けない。
- **worktree を作るときにだけ宣言を満たす**: 作り直しのあとに続きから起動し直す担当は、worktree がもう在るので通らない。
- **`andon boot` が宣言を満たす**: boot は worktree の `mise.toml` を知らない。起動のたびに network を使う手が増える。
- **mise の自動インストールに任せる**: 空の置き場では shim が無く、効かない。
- **go 本体まで image に焼く**: image にバージョンの写しを持つことになり、go を上げる PR の worktree と main の worktree を同時に満たせない。
- **`andon boot` が main の `mise.toml` から既定を作る**: 作り直す写しで、遅れる。go を上げる PR の担当が、黙って古い go を引く。
- **`andon boot` が古いバージョンを自動で消す**: 得る量に対して、動いている検査のバージョンを消す事故の種が大きい。
