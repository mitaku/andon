# 記録だけにする `review` はゲートを回す課題だけ、`held` の `merged` は届ける（ADR-0010 決めたこと 7）

- 日付: 2026-11-10
- 出典: 課題 A-0010（ADR-0010 の「依頼書に切る案」1 の実装）。推奨で進めたもの（人の答えは PR のマージ）

ADR-0010 決めたこと 1 の表を、dispatcher の今ある見分けに合わせて読む。

- `review` を記録だけにするのは、dispatcher がゲートを回す課題（`internal/dispatcher/` の `gated`）だけ。Chief Engineer の課題・調べる課題（`deliverable: candidates`）・repo の無い課題の `review` は届ける。ADR の表の「`profile` が `chief-engineer` でない」をそのまま使うと、ゲートを回さない調べる課題の `review` を誰も読まない。
- `merged` のうち `held` があるもの（人がマージしたが片付けていない）は届ける。片付けていないので、Chief of Staff がすることがある。
- ADR の表に無い `closed`・`corrections` は届ける（安全側）。

捨てた案: ADR の表の字面どおり `profile` だけで `review` を分ける（調べる課題の候補が Chief of Staff に届かなくなる。`tests/andon-dispatcher.sh` の 5b が落ちる）。
