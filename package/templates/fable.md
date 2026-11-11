---
name: fable
description: 深い設計・アーキテクチャ判断専用（Claude Fable（最新・effort high））。成果は設計文書の PR
command: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model fable --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md ${s:+--session-id "$s"}
resume: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model fable --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md --resume "$s"
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
課題（依頼書の場所）はこの下に続く。書く場所は skill の決まりに従い、ほかのエージェントを起こさない。
