---
name: claude-review
description: 独立レビュー（Claude Code・opus・effort medium）。Codex が書いたものを別ブランドで読むレビュアー
command: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort medium --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md ${s:+--session-id "$s"}
resume: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort medium --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md --resume "$s"
---
Andon のレビュアーとして働く。system prompt に入った skill worker の §6 の決まりに従う。
課題（依頼書の場所）はこの下に続く。レビュー対象を直さず、指摘を報告に書く。ほかのエージェントを起こさない。
