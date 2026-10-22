# An OpenCode key that answers 401 is never parked: a third of the gateway's OpenCode calls fail

Found 2026-10-22 01:30 UTC. FP2 batch 1's deployed proof step 5 (the
Texture acceptance spec) failed on test computer computer-9648031f
(vm-80b173b9): the research revision never landed, and revising after a
cancel was refused.

The trace is the gateway journal on Node B, 20:00 to 01:30 UTC, counted by
key name (values never read):
- On the test computer, 36 of its 39 OpenCode calls answered `401
  Unauthorized`. Its fallback, ChatGPT gpt-6-luna, answered 429 and then
  "circuit open". The research desk had no working model.
- The paper (vm-1669020a) has had a steady stream of 401s since at least
  20:00: about 1,020 of about 3,460 OpenCode calls, 30%.
- Calls by key: `OPENCODE_API_KEY` 1,082, `OPENCODE_API_KEY_ALT_0` 2,537.
  Total 401s across computers: about 1,060. The primary key is almost
  certainly the one that answers 401; ALT_0 serves.
- From 00:00, ChatGPT failed on every call, with 429 and an open circuit.
  That is the same OpenAI quota exhaustion that took the review panel's
  sol out until 03:44 UTC. With that fallback gone, the 401s became
  failed turns, not slower ones.

Cause, read in code: `openCodeKeyPool.park`
(`internal/provider/opencode_keys.go:128`) parks a key only on 402 (6 h)
or 429 (60 s). On any other status the call returns that status, and the
next key is never tried. `pick` hashes the session to one healthy key, so
a session that hashes to a key whose credential is revoked or expired
fails on every call for as long as the key stays configured. The pool
from 2026-10-15 (docs/problems/opencode-key-exhausted-no-failover-2026-10-15.md)
covered funds and rate limits, not a dead credential.

Is it a product defect? Yes. The paper loses about a third of its model
calls, and so does any computer whose sessions hash to the dead key. A
new computer can lose all of them. Whether the turns fail or retry
depends on the caller.

The other step 5 results predate FP2. The earlier acceptance receipts in
docs/evidence show:
- T5b, revise after cancel, refused with 409 "durable lifecycle state is
  unavailable or terminal". Every receipt that records T5b since
  2026-10-09 has the same 409. Cancel ends the document's trajectory, so
  the 409 is the documented behavior. It is a named residual of the
  supervised self-development gate
  (docs/definitions/choir-appdev-gate2-supervised-self-development-2026-10-09.md:445),
  not FP2.
- T8 counted open work items after the crash restart: 3 and 5 on
  2026-10-09, 1 today. That also predates FP2. Whether a crash restart
  leaves a work item open against the "restarts end work" rule is still
  unread in the trace. Residual.
- T6, the research revision, landed in only one earlier receipt. Today
  it failed for the cause above: no model answered.

So step 5 does not judge FP2. Texture's own turns (T2a, T2b) passed.
Research has to be re-run once OpenCode serves the test computer's
sessions.

Repair:
1. Code (red: gateway and provider calls). A 401 or 403 parks the key as
   a dead credential, for 6 h like out of funds, and the call is retried
   on the next key. The error and the log name the key's variable, never
   its value. Tests first: a pool with one key that answers 401 serves
   every session from the other key, and with every key dead no upstream
   call loops.
2. Owner. Renew or remove `OPENCODE_API_KEY` in
   `/var/lib/go-choir/gateway-provider.env` on Node B. Until then the code
   repair routes around it. This is the same expired opencode-go key the
   review panel lost at 00:20.

Rollback: revert the repair commit. 401s then fail sessions again, as
before.
