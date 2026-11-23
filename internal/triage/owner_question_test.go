package triage

import kanbanledger "andon/internal/ledger"

// トリアージと振り返りの apply は、andon issue ask --new と同じ関数でオーナーの課題と問いを置く（ADR-0057 決めたこと 7・9・11）。
// その関数を、この package から import が循環せずに呼べることを、build で確かめる（internal/issue に置くと循環する）。
var _, _ = (*kanbanledger.Ledger).NewOwnerQuestion, (*kanbanledger.Ledger).Ask
