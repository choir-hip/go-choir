# M11 probe route: divergent consensus synthesis

**Date:** 2026-09-29. Panel: divergent mode, 13 lenses; 5 panelists completed
(codex/operator, devin/architect, omp-gpt6-sol/maintainer, omp-gemini38/researcher,
omp-gpt6-luna/systems); the rest were killed by a 300s watch timeout on the
foreground shell. Raw outputs: `.agentic-consensus/agentic-consensus-20260929-160814/`.

## The reframe the panel converged on

**Separate defect discovery from acceptance witnessing.** Four of five panelists
independently proposed the same two-tier structure:

- devin: "op-farm drill + episode witness" — pre-position computers holding ops
  at `awaiting_approval`; drill approve/reject/restore in seconds per variant;
  run the full episode once as the M11 acceptance witness.
- gemini: "dual-clock verification" — fast protocol drills on parked computers;
  the monolithic 35-minute episode runs once as an acceptance ceremony.
- codex: "renewable decision-drill station" — resume diagnosis from nearest
  valid state; drills are diagnostic evidence, not M11 acceptance.
- sol: "park real staging computers with real operations; run approve/reject/
  restore drills; fresh-owner episode only for the acceptance bar."

Rationale is identical everywhere: 100% of discovered defects (runs 5-8) were
deterministic protocol bugs in the decision/evidence path — predicate
assignment, pagination math, CAS privacy class, handler-drain race. Zero defects
landed in the bootstrap/desk-production prefix, yet every discovery pays its
~20-minute latency.

## Sharpest unconsidered reframe (gemini)

**The probe is a split-brain instrument.** It drives the public API through
Playwright/WebAuthn *and* injects modes + reads event heads via SSH loopback as
unauthenticated cluster root (`curl -H "X-Internal-Caller: true"` to
`127.0.0.1:8086`). It generates the timing races it discovers — run 8's
drain-vs-handler collision was triggered by out-of-band `setMode` polling no
owner ever performs. The probe is not a mirror; it perturbs the consistency
boundary it measures.

Objection carried: the run-8 race is real code regardless of who triggered it —
the drain genuinely races the handler's own transition, and any sufficiently
fast client would hit it. Split-brain timing explains *discovery*, not
*existence*.

## Second reframe (devin, standing-questions-adjacent)

**The clustering doctrine already fired.** Runs 7+8 are the same substrate —
the commit-then-finalize window around the decision append. The repo's own
convergence rule triggers at 3+ same-subsystem defects: audit
`self_development_decision_binding.go` + materializer + drain as one contract
instead of sampling the path once per 35 minutes. The event-authority surface
is enumerable; sampling it is the slow way to cover it.

## Cheapest structural change (devin, sol, luna agree)

**Overlap the two desk windows.** candidate-B's op can start under the same
`propose_only` arm alongside primary — both cook through desk latency
concurrently, both reach `awaiting_approval` in the same ~20min window.
Sequence: arm qualified_consensus → approve primary → re-arm propose_only →
reject candidate-B. Halves wall-time, removes the probe-imposed
`legs.applied` gate, and tests the reject fix on the same run that tests the
approve path. No new instrument, no substrate change.

## Adopted actions

1. Restructure the probe: candidate-B op starts concurrently with primary;
   legs run non-halting where causally possible; every API response recorded.
2. Keep `commitment_materiality_visible` informational (not in predicate) —
   its falsifier is `falsified_commitment_visible`, which is leg evidence.
3. Document the drain/handler 409-rescue wart as a named residual: correct
   end-state, wrong HTTP response for a committed decision. Not M11-blocking.
4. The SSH split-brain stays — it is how the probe exercises modes a normal
   owner cannot arm, which the episode definition requires.
