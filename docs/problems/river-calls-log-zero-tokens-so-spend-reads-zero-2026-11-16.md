# River calls log zero tokens, so River's spend reads zero and its budget cannot trip

Date: 2026-11-16. Status: open.

## Problem

Every River call in the gateway's journal logs `tokens_in=0 tokens_out=0`.
The hourly status script therefore reports River calls as "unpriced" and
River's spend as $0.00 of its $10 daily budget. An owner-set spending cap
that cannot see spend does not hold.

## Evidence

- Node B journal, 2026-11-16: River calls from the `autoradio` and
  `autoradio_programme` roles, all `stream=true`, all with zero tokens. One
  programme call ran 17:16:18 to 17:17:18 and returned 14,071 characters.
- `scripts/status-hourly`, 2026-11-16T16:35Z: "river: 0 calls last hour, 0
  today, $0.00 today (15 calls unpriced)".
- `include_usage` appears only in `internal/gateway/openai_compat.go`; nothing
  under `internal/provider` sends `stream_options` on a streamed call.
  `docs/reference/RIVER.md` predicted this.

## Belief

A streamed call to an OpenAI-compatible provider returns usage only when the
request asks for it, and ours does not ask (likely; read from code, not from a
captured response).

## Remaining error

- Not confirmed against River's own documentation that it honours the usage
  option on a stream.
- Real spend since 2026-11-15T19:07Z is unknown. At the table's prices a
  programme call of this size is a fraction of a cent (estimate).
- Wrong statements made on 2026-11-16 from stale docs: "River is not
  registered on the gateway" (it has been since 2026-11-15T19:07Z) and "the
  48 s stretch went to a provider that does not stream" (the call that was
  timed predates the registration; a streamed programme call today still took
  about 60 s end to end, because the route returns one long text).
