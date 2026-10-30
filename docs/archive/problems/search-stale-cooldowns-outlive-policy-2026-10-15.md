# Search cooldowns written under an old policy outlive it (2026-10-15)

Mutation class of the fix: **orange** (gateway search-plane health store; no
protected surface). Rollback: revert the fix commit; stored deadlines are then
honored as written again. Problem first; the fix commit follows and references
this doc. Standing rule: the search health store is never edited by hand; the
fix is code.

## Evidence

Read-only read of `/var/lib/go-choir/gateway/search-health.db` on Node B at
about 17:20 UTC 2026-10-15:

- `exa | cooling_down | cooldown_until=2026-10-20T10:32:36Z | strikes 14 |
  class error | "parse response: unexpected end of JSON input" | updated
  2026-10-13T10:32Z`. That error is the 256 KiB truncation bug, fixed in
  e672c7a58 and deployed today
  (`exa-search-always-errors-2026-10-15.md`). The current
  `DefaultBackoffPolicy` (`internal/gateway/searchplane/policy.go`) caps class
  `error` at 1 h via `MaxByClass`. The stored deadline was written under an
  older policy with a 7-day maximum and is never re-validated, so Exa will not
  be retried until 10-20 and every search logs `exa=error`.
- `searxng | cooling_down | until 2026-10-15T20:12Z | strikes 12 |
  success_empty | updated 2026-10-08`: also stale, beyond the current
  `MaxByClass` of 6 h for `success_empty`.
- The other providers' cooldowns are real `quota_limited` states (see below)
  and are correct behavior, not part of this fix.

## Root cause

`CooldownDuration` applies `MaxByClass` and `MaxCooldown` only when a deadline
is computed. A row persisted in `provider_health` keeps its `cooldown_until`
forever; `normalizeExpiredLocked` only compares it to now. A policy change
therefore never reaches rows written under the old policy. Substrate: the
health store's read path trusts a stored deadline without re-validating it
against the policy in force.

## Provider credit state (owner billing decision, not part of this fix)

- brave: 429, plan quota exhausted
- serper: "Not enough credits"
- parallel: 402
- tavily: 432 usage limit
- serpapi: "run out of searches"

So search effectively runs on Keenable alone (1,835 calls today) plus ChatGPT
capped at 50/day. Topping up or replacing these providers is an owner billing
decision.

## Fix shape

Clamp `cooldown_until` to at most `updated_at + MaxByClass[last_failure_class]`
(and `updated_at + MaxCooldown` overall) whenever the store loads a row, and
once at startup with one log line per clamped provider. `auth_error` has no
class cap, so it keeps its 7-day cooldown.

## Belief state / remaining error field

- Hypothesis (confirmed by the stored deadline vs. policy arithmetic, not by a
  gateway trace): after deploy, exa becomes eligible at the next search.
- Remaining: whether Exa then succeeds or fails with a fresh error class is
  unobserved until the post-deploy check.
