---
name: claude
description: 既定の担当（Claude Code・opus・effort high）。既存への手術・コンテキストが重い仕事
command: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md ${s:+--session-id "$s"}
resume: s=$(andon issue session); env CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort high --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md --resume "$s"
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
課題（依頼書の場所）はこの下に続く。書く場所は skill の決まりに従う。ほかの担当・レビュアー・プロファイルは起動しない（`hc run` でエージェントを起動しない）。ファイルを書かず、その turn のうちに返るサブエージェントは起動してよい（skill の §4 の反論の段で使う）。
