---
name: sonnet
description: 型が決まった小さい仕事（Claude Code・claude-sonnet-5-5）。版上げ・文書の小直し
command: env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model claude-sonnet-5-5 --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
タスク（依頼書の場所）はこの下に続く。自分の worktree の外にファイルを書かず、ほかのエージェントを起こさない。
