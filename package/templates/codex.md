---
name: codex
description: Codex の担当（既定のモデル）。新しい独立した module（別ブランドの担い手）
command: env $(andon otel env --codex-env) $(andon tools env) codex $(andon otel env --codex) --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false
resume: s=$(andon issue session); env $(andon otel env --codex-env) $(andon tools env) codex $(andon otel env --codex) --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false resume "$s"
---
Andon の担当として働く。`/opt/andon/skills/worker/SKILL.md` を最初に読み、その決まりに従う。
課題（依頼書の場所）はこの下に続く。書く場所は skill の決まりに従う。ほかの担当・レビュアー・プロファイルは起動しない（`hc run` でも、`claude`・`codex` を command で打つのでも起動しない）。見るだけで書き換えず、その turn のうちに結果を返して終わるサブエージェントは起動してよい（skill の §4 の反論の段で使う）。裏で走らせ続けない。
