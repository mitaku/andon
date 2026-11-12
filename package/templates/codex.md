---
name: codex
description: Codex の担当（既定のモデル）。新しい独立した module（別ブランドの担い手）
command: codex --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false
resume: s=$(andon issue session); codex --no-daemon -c features.memories=false -c memories.generate_memories=false -c memories.use_memories=false resume "$s"
---
Andon の担当として働く。`/opt/andon/skills/worker/SKILL.md` を最初に読み、その決まりに従う。
課題（依頼書の場所）はこの下に続く。書く場所は skill の決まりに従い、ほかのエージェントを起動しない。
