# The Self-Development Retry Path: Why It Kept Breaking, and What to Do About It

**Report date:** 2026-08-17
**Scope:** `computer-03335285269bdba4f94377e56879f9e6`, operation `selfdev-b090bcd72d300fed17cb3f5a142f8595`
**Authority:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Method:** Independent agentic-consensus panel (4 of 8 agents completed), synthesized against live staging evidence and source.

---

## Executive summary

A self-development operation that should take one clean pass was stuck for days. The
retry path — Super wakes Texture, Texture opens work for Super, Super delegates to a
capsule CoSuper — broke three different ways in a row. Each time one break was fixed,
the next one appeared.

Two are now fixed and verified. The third is diagnosed and has an agreed remedy but is
**not yet patched**, because it sits on a protected surface (Texture's canonical writes).

In one sentence: **the retry path treats ephemeral, restart-losing state — run IDs,
credential tokens, derived indexes — as durable authority, and each gate fails closed
on a different piece of that state.**

---

## The three failures, plainly

### 1. The CoSuper that never woke up *(fixed)*

The implementation agent (a "CoSuper" working in an isolated capsule) started, then
froze on its first model call. No tokens, no result, no error — just stuck.

On every restart, the system is supposed to notice a stuck assignment and cancel it.
It didn't, for two reasons:

- The stuck run had been created **before** a later fix that added an index of active
  runs. The index never listed it, so recovery never saw it.
- Even after a computer-wide sweep was added, the sweep walked the trajectory's
  assignments in order and **tripped on the first one** — an old, already-cancelled
  assignment that had never gotten a capsule — and gave up before reaching the one that
  mattered.

**Fix landed:** `cec68e23` makes the sweep skip assignments that never had a capsule.
Verified live: the stuck CoSuper is now cancelled, its capsule revoked.

### 2. The expired keycard *(remedied, no code change)*

Once the CoSuper was cleaned up, retrying the operation failed with
`guest credential: renewal refused`.

The guest's platform credential is a short-lived keycard (about 4–5 minutes, with a
60-second grace). The computer had been idle longer than that between the refresh and
the retry, so the keycard had gone stale and the platform rightly refused to renew it.

**Remedy:** a fresh `choir computer refresh` **immediately followed** by the retry. No
code change — but this is fragile, and the report recommends fixing it properly (see
Remedies).

### 3. The impostor supervisor *(diagnosed, not yet patched)*

After the keycard was fresh, the retry got one step further and failed with:

```
target work does not bind the open target obligation and caller provenance
```

What happened: Texture (the supervising agent) originally opened the work for Super
under one internal identity (a "run" ID). Over days of restarts, that identity was
retired, and a **new** Texture identity was minted. The work item still remembers the
**old** identity as its author, but the retry arrives as the **new** identity. The
security check (correctly) refuses to let a different identity continue work it didn't
author.

The fix is to stop inventing new identities and instead **restore the original author
identity** before continuing. Details below.

---

## Common root cause

The panel agreed the three failures share one substrate problem, not three coincidences:

> **Durable progress is gated on ephemeral identity that restarts are allowed to change
> or forget.**

Concretely, four "who is current / what is discoverable" projections drifted apart, and
each fail-closed gate caught a different one:

| Piece of state used as authority | How it failed |
| --- | --- |
| Run-state index | Older runs were never indexed, so recovery couldn't see them |
| Assignment sweep ordering | One irrelevant terminal item aborted the whole trajectory |
| Guest credential token | Lazy renewal assumed another call would arrive inside the TTL |
| Texture caller run ID | Work provenance required the *current* caller to equal the *original* author |

Two of these (the run-state index gap and the Texture caller recreation) are **substrate**
defects. The credential expiry is best understood as a **symptom** made load-bearing only
because the other two forced manual retries — the real fix is to renew the credential
inside the retry, not to lengthen the timeout.

---

## What the panel recommends

### For failure 3 (the immediate blocker)

**Agreed remedy: restore the original author identity — do not relax the security check
and do not rewrite history.**

The four completed panelists converge on this:

- **Reject "rewrite the work's provenance to the new identity."** That would falsify the
  audit record — effectively forging who authored the work on a protected surface.
- **Reject "match on the Texture agent alone and ignore the run ID."** That weakens a
  fail-closed check globally to work around one bug.
- **Do this instead:** `ensureSelfDevelopmentTextureCaller` should reactivate the
  **deterministic original caller run** (`3b18a6d7…`) rather than adopt the successor
  (`aa4fc186…`), and only then continue. The invariant to preserve:

  > The historical run proves *who opened the obligation*; the current activation proves
  > *who may continue it*. Both must be verified, but they are not the same field.

Two important refinements the panel surfaced:

1. **A second poison pill.** The caller run's command ID is derived deterministically,
   but its activation digest hashes the current time — so the run is creatable **exactly
   once**. The next retry would then fail with a command conflict even after the caller
   is fixed. The caller run's ID and command ID must be made incarnation-scoped off the
   predecessor run.
2. **The "empty `CreatedByRunID`" may be a field-name artifact.** The provenance field
   serializes as `created_by_loop_id`, not `created_by_run_id`. If it is genuinely empty,
   the admissible fallback is: empty `CreatedByRunID` may match `requested_by_run_id`
   (same run, missing field) — **never** a different run.

### Durable remediation (the whole retry path)

1. **Stable identities are the authority** — operation ID, trajectory, work item,
   control, reducer sequence. Runs, credentials, processes, and indexes are reconstructible
   projections, never the source of truth.
2. **Reconcile per obligation, not fail-fast per trajectory.** One irrelevant settled item
   must not abort recovery of the rest (the shape of failure 1 is only half-fixed).
3. **Never use a derived index as the only recovery path.** Backfill old objects and keep
   a bounded canonical scan as a repair fallback.
4. **Separate "origin" from "current activation" in Texture provenance** (as above).
5. **Renew credentials inside the retry request**, not by widening the TTL.
6. **Add a restart-contract test:** open work under Texture run A, passivate A, activate
   B, retry, and prove exactly one obligation continues with A as immutable origin and B
   as current caller.

---

## Dissent and risks

- One panelist (claude) argued for option (ii) in a *narrowed* form — keep the run-ID
  check but verify the creator run resolves to a real activation of the same agent,
  rather than requiring the *current* run to equal it. This is compatible with the
  "immutable origin vs current activation" framing the others reached; the disagreement
  is over where the line is drawn, not over rejecting the blunt relaxation.
- **Unverified claims:** the panel could not independently read the live `created_by_loop_id`
  value, so whether empty `CreatedByRunID` is a second blocker remains unconfirmed. The
  report marks this medium confidence.
- **Protected surface:** the fix touches Texture's canonical writes (red mutation class).
  It should land with full ceremony, not as a quick patch.

---

## How to avoid re-entering this loop

Until the durable fixes land, an operator should, in order:

1. Confirm staging deploy identity and the retained computer's epoch.
2. Read the product observability surface — operation phase, active/passivated runs,
   CoSuper assignment dispositions, open Super/Texture work, last reconcile outcome.
3. Confirm no live CoSuper remains before retrying.
4. Run `choir computer refresh`, then **immediately** retry the **same** operations POST
   (same prompt, same idempotency key) inside the credential TTL.
5. **Do not** directly restart Super, forge Texture control, hand-edit database rows,
   SSH-patch state, or issue a new operation identity.
6. Re-read the projections after every attempt before trying again.

---

## Panel and evidence

- **Panel (4/8 completed):** omp-gpt56-sol, claude, omp-gemini36, cursor. The other four
  (codex, opencode, omp-gpt56-terra, omp-deepseek-v4-flash-free) failed or timed out.
- **Source anchors:** `internal/agentcore/selfdev_texture_join.go`,
  `internal/store/lifecycle_texture_target.go`, `internal/store/texture_turn.go`,
  `internal/store/lifecycle.go`, `internal/platform/event_capability.go`.
- **Live evidence:** `docs/evidence/effects-red-cosuper-run-state-index-2026-08-17.md`,
  `docs/evidence/effects-red-cosuper-terminal-2026-08-17.md`,
  `docs/evidence/effects-red-guest-credential-renewal-2026-08-17.md`,
  `docs/evidence/effects-red-retry-cluster-2026-08-17.md`.

**Confidence:** high on the root cause and on rejecting provenance-rewrite and
agent-only relaxation; medium on the exact repair shape (backfill vs narrow fallback)
because it depends on the live `created_by_loop_id` value.
