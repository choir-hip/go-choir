# A Texture wake misses the warm cache, and Bedrock refuses the paper's requests (2026-11-09)

Owner, 2026-11-09: "when another desk wakes texture, we're properly using the
warm cache"; history is kept, never trimmed. Evidence and method:
[the append-only and cache audit](../evidence/token-efficiency-2026-11-09/followup/append-only-cache-audit.md).

## 1. A carried Texture wake is not append-only

88% of the paper's Texture wakes (276 of 315) start a new run that carries
the earlier conversation. The carry drops the earlier run-context turns,
message 0 included (`internal/agentcore/run_memory.go:267`), so the new
request shares only the system prompt and tools with the last one.

Measured on Synthetic (d@e and b@c; the paper's crouton route reports no
cache): the first call of a carried wake reads 22% from cache where a pure
append would read 87%; median fresh tokens 25.4k against 8.2k (44 wakes). A
call-by-call trace of one d@e document shows the cached amount equals
everything before the previous run's run-context turn.

## 2. The cache lives 5 minutes; Texture wakes come 10 minutes apart

`cache_control` is the plain 5-minute `ephemeral`
(`internal/provider/provider.go:524`, `:2008`). The paper's wake gap is
p50 628 s and 82% of gaps exceed 300 s, so on Bedrock most wakes would find
the cache expired even with the carry fixed.

## 3. Bedrock answered 400 to every paper request

Gateway journal, last 8 hours: 9 `provider=bedrock` requests for Haiku 5.5,
all at 19:48:12–16 UTC from one computer, all `status 400 Bad Request`; no
success. The requests were mid-run (5, 16 and 26 messages, Texture's system
prompt, `reasoning=high`). The reason is not logged (the rejection text is
sanitized away). Hypothesis, untested: histories written by another model
carry no thinking blocks, which the Anthropic API refuses when thinking is
on. Until one mid-run request succeeds, the paper has no working paid
fallback, and crouton's free week ends about 2026-11-10.

Also seen: the request says `reasoning=high` for Texture, where policy says
low.

## Belief state

1 and 2 are measured on Synthetic and read from code; their effect on Bedrock
is inferred. 3 is measured as a failure; its cause is a hypothesis. Other
ranked items (threshold compaction, the 8,000-rune cell output cut, repeated
wake prompt text, whole-document frames) are in the audit, section 6.
