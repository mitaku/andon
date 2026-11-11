---
name: codex-review
description: 独立レビュー（Codex・gpt-6-luna・effort max）。Claude が書いたものを別ブランドで読むレビュアー
command: codex --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false -m gpt-6-luna -c model_reasoning_effort="max"
resume: s=$(andon issue session); codex --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false -m gpt-6-luna -c model_reasoning_effort="max" resume "$s"
---
Andon のレビュアーとして働く。`/opt/andon/skills/worker/SKILL.md` を最初に読み、その §6 に従う。
課題（依頼書の場所）はこの下に続く。レビュー対象を直さず、指摘を報告に書く。ほかのエージェントを起こさない。
