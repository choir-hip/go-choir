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

## Investigation 2026-11-09 evening

(a) Truth: all three items have fixes on main, and tonight's gateway agrees.
- Item 1 (carry not append-only): 1f8dfe4a5 ("a carried wake keeps every
  earlier message byte for byte", orange).
- Item 2 (5 minute cache, 10 minute wake gap): d2adcb6ca (Bedrock cache
  breakpoints use the one-hour TTL), c4d72a61a (mixed lifetimes priced into
  spend and the $20 budget).
- Item 3 (Bedrock 400): 18a654482 (foreign tool ids mapped, unsigned thinking
  and empty text blocks dropped, input_schema always sent), 4b35bf051,
  b430fad68 (a result for every tool call), d141afcb0 (thinking never off for
  Haiku 5.5), 3ab5f1eef (4xx type and message now logged). The ledger entry
  bb8c25b6f carries the adjudicated root cause; this record's hypothesis was
  in the right family only.
Tonight [M]: 0 Bedrock lines with a 4xx or 5xx status in 3 hours against 2,584
request and success lines; success lines show `cached=25158
cache_write_1h=653`, so the one-hour cache is read and written.

(b) Still open: a Texture Bedrock request at 01:30Z (vm-70070cf7) says
`reasoning=high`; the owner ruled low for everything except Sonnet.
d141afcb0 floors "off" at the lowest thinking-on effort, so "high" is either
policy resolving high or a path that ignores the ruling. Hypothesis; confirm
by reading the policy resolve output for Texture on that computer.

(c) Items 1-3 were provider-translation symptoms. The reasoning level is a
policy-resolution question.

(d) Residual fix: read what policy resolves for Texture; if low, find the
path sending high. Orange, under 2 hours.

(e) Items 1-3 are stale: fixed by the commits above.
