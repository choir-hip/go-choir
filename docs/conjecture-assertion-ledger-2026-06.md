# Conjecture / Assertion Ledger — June 2026

**Status:** canonical epistemic state. Assertions with receipts, invariant
candidates awaiting promotion, and open hyperthesis edges. Updating this
ledger is part of mission stopping conditions (MissionGradient v2.0.0).
An assertion whose premise dies reverts to a conjecture — edit it here,
visibly, rather than letting it rot into heresy.

Format per `conjecture-learning-proof-theory-2026-06-11.md`: a claim's scope
is the domain its evidence covers; receipts are named artifacts.

---

## Assertions (supported, with receipts)

### A1 — VText delegation tool-scope is enforced in code

- **Claim:** VText agents cannot spawn co-super or vsuper; spawn targets are
  exactly `[researcher]`, enforced at call time; super is reachable only via
  `request_super_execution` (routes to the persistent super, never spawns).
- **Receipts:** `internal/runtime/tool_profiles.go:223-229` (AllowedDelegateTargets),
  `tool_profiles.go:314` (canDelegateTo), `tools_vtext.go:194-237`.
- **Scope:** tool-level enforcement only. Does NOT cover continuation-path
  authority handoffs (that is the M2–M4 cutover work).
- **Invalidation triggers:** any change to RegisterCoAgentTools or roleSpec.

### A2 — The actor messaging protocol has no lost-wake states (model level)

- **Claim:** under send/activate/deliver/steer/passivate/evict/crash, every
  durably appended update is eventually processed; passivation cannot strand
  work; eviction is crash-equivalent.
- **Receipts:** `specs/actor_protocol.tla` — exhaustive at 2 agents/3 updates
  (3,016 states) and 3 agents/4 updates (218,055 states); sabotage variants
  (mailbox-only passivation guard, no sweep) produce counterexamples.
  Package level: `internal/actor` tests green under `-race`, including the
  concurrent send-vs-passivation stress test.
- **Scope:** the model, and the `internal/actor` package as tested. Transfers
  to the integrated runtime only after M3 + conformance; NOT yet a claim
  about production.
- **Invalidation triggers:** spec edits; actor core changes to the locking
  protocol; the M3 integration replacing the loop shape.

### A3 — Cross-VM outbox semantics are loss-free (model level)

- **Claim:** with ack-after-durable-receipt, every committed cross-VM update
  survives drops, duplicate deliveries, and either VM crashing.
- **Receipts:** `specs/actor_protocol_xvm.tla` (8,834 states, exhaustive);
  premature-ack sabotage caught in 4 states by NetworkCovered.
- **Scope:** the model. No implementation exists yet.

### A4 — The redesigned Wire pipeline rules hold under full concurrency (model level)

- **Claim:** suppressed-implies-published, edition honesty, settlement
  soundness, and every-item-settles hold with processors fully parallel.
- **Receipts:** `specs/wire_pipeline.tla` (412 states); both June production
  incidents (coverage-vs-drafts f44065ed; list/open split-brain) reproduce as
  counterexamples from sabotaged guards.
- **Scope:** the model. The formal case for retiring `maxProc=1` — retired in
  reality only after M5's production falsifier cycle.

### A5 — Promotion approval gate and freshness CAS are enforced

- **Claim:** PromoteAppAdoption requires `owner_approved` (produced only by
  the approve transition; the Features Activate click records it), and a
  freshness CAS blocks promotion when the foreground lineage moved since
  verification.
- **Receipts:** commit 77f65651; `internal/runtime/app_promotion.go`
  (ApproveAppAdoption, promoteFreshnessCAS); comprehensive test asserts
  premature promote → 400; unit tests cover fresh/moved/legacy.
- **Scope:** the two guards. Promotion still does not deploy anything real
  (route flip unconsumed) — that claim awaits M6.

### A6 — The full handoff sources were recovered

- **Claim:** the truncated companion handoffs were replaced with complete
  sources (1014 and 830 lines; truncation banners gone).
- **Receipts:** commit 295b4b14; `wc -l` in the review v2 §0.

---

## Invariant candidates (proposed; promote with evidence)

### I1 — Import verification

Documents imported from tool output must be verified by line count or tail
inspection before being treated as canonical. (Origin: the truncated-handoff
incident — scope exceeding reach in the docs layer.)

### I2 — Generative imagery labeling

All generative images are labeled as generated. A photograph asserts an
observation; a generated image asserts nothing — it is illustration. The rule
is not "no generative images" but "no unscoped assertions": an unlabeled
generative image is heresy in the artifact layer. (Origin: grand synthesis
§1.4.)

### I3 — Activation caps are load-bearing for liveness

Per-owner activation caps are correctness machinery, not just cost policy:
bounded evictions are what make EventuallyProcessed hold. Removing the cap
breaks the liveness proof. (Origin: actor_protocol.tla design dividend.)

### I4 — No claim outruns its evidence class

"Verified" means the named contracts passed, never "safe" or "correct"; tests
are existential evidence; model checks are universal over the model only.
Surfaces (UI copy, docs, reports) are audited for this as part of heresy
sweeps. (Origin: proof-theory doc §3; promotion conjecture §11.)

### I5 — Actors get obligations, not identities

Actor prompts state trajectory, conjecture, obligation, authority envelope,
and settlement criteria — never personas. "You are X" is a banned pattern.
(Origin: role-free actor protocol; promote after M2 rewrites prompt_defaults
and the proof mission measures the effect.)

---

## Open hyperthesis edges (named, not resolved)

### E1 — Gates vs. extremes (the open dialectic; status: open)

- **Blind spot:** a fully gated system may be protected from slop and from
  breakthrough by the same mechanism. Significance-*detection* may require
  calibration against extremes that verification-*gates* filter out.
- **Boundary type:** frame_lock (the gate vocabulary cannot express what it
  excludes).
- **Bound:** record, do not resolve prematurely; revisit with evidence from
  missions where a gated process demonstrably missed a live insight.
- (Origin: The Portfolio Mind → grand synthesis §2. This is the hyperthesis
  edge of the whole project.)

### E2 — Conjecture machinery may be decorative (C0's edge; status: testing)

- **Blind spot:** the format may feel profound while failing to change
  action, verifier choice, or stopping conditions.
- **Bound:** the M1 proof mission adjudicates (§15 criteria); MissionGradient
  v2.0.0 carries the anti-decoration gate in-skill. Do not promote the
  discipline to invariant before the measurement.

### E3 — Settlement rules may be wrong in inexpressible ways (status: open)

- **Blind spot:** a trajectory that settles by rule while real work still
  mutates its artifact — the rule vocabulary may lack the predicate that
  would catch it.
- **Boundary type:** frame_lock. **Bound:** rules are data, reviewed after
  M5's first real cycle; the falsifier is explicit in the portfolio.

### E4 — Verified harness, unverified cognition (status: accepted residual)

- **Blind spot:** formal verification covers the harness — what wakes whom,
  what enters canonical state, what authority a message carries — never the
  model's reasoning. Scope creep toward "verify the model" is the failure
  mode.
- **Bound:** every 2027-target claim names this boundary explicitly.

---

## Conventions

**Bimodal naming for hyperthesis:** STT cannot distinguish "hyperthesis" from
"hypothesis" — the failure is silent (the transcript reads plausibly). Say
**"blind edge"** aloud; write **"hyperthesis"** in text. The concept (scope
minus reach) is the invariant; the names are channel-specific bases. Choir's
own transcript pipeline should add a post-processing rule (portfolio:
deferred implementation item).
