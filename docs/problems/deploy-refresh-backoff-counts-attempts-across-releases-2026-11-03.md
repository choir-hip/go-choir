# Deploy-refresh retry backoff counts attempts across releases (problem 27)

Mutation class: green (this doc). Date: 2026-11-03.

## Evidence

Node B `/var/lib/go-choir/vm-state/deploy-refresh-pending.json` at 19:10Z:

- claude test arm: target 70160304a, first_deferred_at 16:21:53Z, attempts 37,
  last_attempt_at 18:16:18Z.
- deepseek arm: first_deferred_at 14:29:55Z, attempts 41.

Both arms are busy for honest reasons (a live capsule) and idle between builds.
The claude arm was idle from 18:51Z and still not refreshed at 19:10Z, while
arms with fresh entries refreshed within a minute.

## Cause

`deployRefreshRetryDue` backs off once `Attempts >= deployRefreshRetryBackoffAfter`
(30), doubling the gap up to `deployRefreshRetryMaxGap` (1 h). Its comment says
the backoff is for a refusal "that does not clear by itself". When a new deploy
re-targets an existing entry (`settle`, the woke_behind `setTarget`, and the
ring walk's `ensure` for a woke_behind entry), `Attempts` and `FirstDeferredAt`
carry over, so a fresh release inherits the previous release's backoff. A new
release is new work, not a repeated refusal.

## Fix

Re-targeting an entry resets `Attempts` to 0 and `FirstDeferredAt` to now and
logs the old count. Repeated refusals of the same target keep the backoff.
Whether "busy" refusals should skip the backoff is left unchanged: see the fix
commit.
