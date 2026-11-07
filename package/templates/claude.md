---
name: claude
description: 既定の担当（Claude Code・claude-opus-5-5）。既存への手術・文脈が重い仕事
command: env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model claude-opus-5-5 --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
課題（依頼書の場所）はこの下に続く。自分の worktree の外にファイルを書かず、ほかのエージェントを起こさない。
