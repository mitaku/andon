---
name: fable
description: 深い設計・アーキテクチャ判断専用（Claude Fable 5.1・effort high）。成果は設計文書の PR
command: env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model claude-fable-5-1 --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
タスク（依頼書の場所）はこの下に続く。自分の worktree の外にファイルを書かず、ほかのエージェントを起こさない。
