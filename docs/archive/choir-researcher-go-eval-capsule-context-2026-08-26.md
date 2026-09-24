# Define Boundary: Researcher Go-Evaluation Capsule Context

<decision_id: researcher-go-eval-capsule-context>
<frozen_candidate_digest: f252fc550fd0a1acc22a5aa0198dcf9b6078ce04db96d8b6fc141a42a73a634a>
<deployed_commit_at_decision: ae12f82d>
<recorded_at: 2026-08-26>

## Decision

The Researcher Go-only profile (the Definition's "restricted Researcher" conformance
probe) REQUIRES a new dedicated Researcher capsule-context path. It cannot be
wired by a safe patch to the existing CoSuper path, and it MUST NOT be fabricated
by reusing the CoSuper authoring capsule (that would grant a Researcher an
authoring/mutation capsule, violating the Go-only restricted profile). The design
below is the accepted boundary for the next implementer.

## Why the current code cannot express it (verified)

1. **No Researcher case in activation.** `executeWithToolLoop` (runtime.go ~293-298)
   injects a `CapsuleToolCtx` for `agentprofile.Super` and `agentprofile.CoSuper`
   only. A `Researcher` run never receives a `CapsuleToolCtx`, so `capsule_go_eval`
   (which requires a `CapsuleToolCtx`) is unreachable to a Researcher.

2. **Researcher registry installs no capsule tool.** `buildRegistryForRole` builds
   the Researcher registry from `agentprofile.PolicyFor(agentprofile.Researcher)`
   (read-only files + research tools). `RegisterCapsuleLocalTools` (the
   `capsule_go_eval` installer) is called only from the assigned-CoSuper builder
   (tool_profiles.go:311). So even with a context, a Researcher has no
   `capsule_go_eval` tool to call.

3. **Researcher capabilities are wildcard-only.** `MintCapabilityHandle` forces
   `RoleResearcher` to `TargetCapsule == "*"` ("read-only inspection across
   capsules"), and `resolveOne` rejects any `TargetCapsule == "*"` capability.
   A Researcher therefore cannot resolve a *specific* capsule for Go evaluation.

## The accepted design (authority-consistent)

A Researcher that may evaluate Go must be bound to a **dedicated, read-only,
disposable capsule** (a new lifecycle capability), NOT the wildcard inspection
target and NOT a CoSuper authoring capsule. Concretely:

1. **New capsule class** `RoleResearcher` spawn: on a Researcher assignment, the
   runtime spawns a dedicated read-only capsule (no write path surfaced — the
   Researcher's verb set is read-only files + go_eval; go_eval evaluates in a
   restricted interpreter, never mutates the capsule).

2. **Non-wildcard binding**: `MintCapabilityHandle` for `RoleResearcher` must bind
   to this dedicated capsule ID (remove the `capsuleID == "*"` force for the
   Researcher-eval path; keep the wildcard read-only inspection path separate
   OR drop it in favor of the dedicated capsule). `resolveOne` then resolves the
   dedicated capsule instead of refusing wildcard.

3. **Registry**: install `capsule_go_eval` into the Researcher registry (add a
   `RegisterResearcherCapsuleTools` or extend the Researcher build), gated to
   `RoleResearcher` with a read-only manifestation.

4. **Context injection**: add a `case agentprofile.Researcher:` in
   `executeWithToolLoop` that injects a `CapsuleToolCtx` with
   `Role: capsule.RoleResearcher` and the dedicated capsule handle, mirroring the
   CoSuper path but without any mutation authority.

5. **Conformance test**: an end-to-end staging test proving Go evaluation
   succeeds, while Bash (`exec`), `write_file`, host tools, and cross-capsule
   access all refuse.

## Why not implemented now

This is a red-class protected-surface change (capsule execution + activation +
capability broker) requiring a Define boundary, a frozen candidate, and a
re-review before landing. It is recorded here so the next implementer follows
the design rather than fabricating a binding that would grant a Researcher an
authoring capsule (an unauthorized authority path).

## Evidence

- runtime.go `executeWithToolLoop` switch (Super/CoSuper only)
- tool_profiles.go `buildRegistryForRole` + `RegisterCapsuleLocalTools` caller
- capsule/executor.go `MintCapabilityHandle` (wildcard force) + `resolveOne`
  (wildcard refusal)
