# One OpenCode key, no failover: 7,299 calls refused with 402 in 6 hours

Date: 2026-10-15 (~00:30 UTC). Class: orange (provider routing).

## Evidence

- The owner reports that the OpenCode account ran out of credits and
  DeepSeek stopped. A second key, `OPENCODE_API_KEY_ALT_0`, is now in the
  operator `.env`. The first key's credits return in about 4.5 days.
- Gateway journal, last 6 hours on Node B (counts only):
  - 7,299 × `opencode-go: status 402 Payment Required`, with reason
    "Insufficient account funds";
  - 180 × `429 Too Many Requests`, with reason "Rate limit exceeded".
- `internal/provider/provider.go` `openCodeAPIKeyFromEnv` reads one key
  (`OPENCODE_API_KEY`, else `OPENCODE_GO_API_KEY`). Every OpenCode Go and
  Zen provider calls upstream with that key on every request. Nothing
  remembers that the key is out of funds, so each call pays a round trip
  to learn it again.

## Failure modes the repair must handle

1. **A key out of funds (402)** keeps being tried. It must be parked for
   hours, not retried per call, and still recover by itself when the
   credits return.
2. **A rate-limited key (429)** should be parked for seconds and the
   request retried on another key.
3. **A retry after a response has started streaming would duplicate
   output.** Retry only on a status received before any body is read.
4. **Prompt-cache affinity.** OpenCode routes a session to one upstream
   for cache reuse. Spreading one desk thread across keys could cost cache
   hits, so keep a thread on one healthy key while it is healthy.
5. **All keys parked.** Fail fast without an upstream call, with an error
   the model pool's failover can act on. Don't hammer upstream.
6. **Key material in logs or errors.** Name keys by position
   (`OPENCODE_API_KEY_ALT_0`), never by value or prefix.
7. **Gateway restart forgets parking.** That is acceptable: the first
   call to an exhausted key re-parks it.
8. **The ALT key reaches Node B through `nix/deploy-provider-creds.sh`'s
   upsert mode** (`CHOIR_PROVIDER_ONLY_KEYS`). Values travel on stdin,
   never on the command line.

## Repair

A process-wide OpenCode key pool, read from `OPENCODE_API_KEY`,
`OPENCODE_GO_API_KEY` and `OPENCODE_API_KEY_ALT_<n>`. It has per-key
parking (402: 6 h, then the next call is a probe; 429: 60 s), retries once
on another healthy key before any body is read, and gives each session a
sticky choice among the healthy keys. Rollback: revert, and the gateway
reads `OPENCODE_API_KEY` alone again.
