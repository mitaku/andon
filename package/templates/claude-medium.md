---
name: claude-medium
description: 見込みが小・中の担当（Claude Code・opus・effort medium）。effort のほかは claude と同じ
command: s=$(andon issue session); env $(andon otel env) CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort medium --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md ${s:+--session-id "$s"}
resume: s=$(andon issue session); env $(andon otel env) CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude --model opus --effort medium --plugin-dir /opt/andon --append-system-prompt-file /opt/andon/skills/worker/SKILL.md --resume "$s"
---
Andon の担当として働く。system prompt に入った skill worker の決まりに従う。
課題（依頼書の場所）はこの下に続く。書く場所は skill の決まりに従う。ほかの担当・レビュアー・プロファイルは起動しない（`hc run` でも、`claude`・`codex` を command で打つのでも起動しない）。見るだけで書き換えず、その turn のうちに結果を返して終わるサブエージェントは起動してよい（skill の §4 の反論の段で使う）。裏で走らせ続けない。
