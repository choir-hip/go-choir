---
definition_version: 4

# Station FP10 / NET-05 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): the grant object for
# open egress. Authority: FP0's assignment of 2026-10-21 (favor a design plus
# a small off-by-default slice: the EgressGrant record, validation and
# expiry, owner-act-only decide; no wiring); SDC NET-05; the grants design
# §4.0a and §4.2; NET-10's design.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commit to the bunch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T22:00:00Z
  source:
    canonical_ref: main d1c60ad87
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp10-net05-grants
      status: clean
      class: goal_candidate
      owner: FP10 NET-05
      touch: goal_owned
      recovery: branch lane/fp10-net05-grants from d1c60ad87

finish:
  deliver: >-
    One grant model for open egress, shared by engineering assignments and
    app-desk manifest grants: a validated, time-limited EgressGrant record
    whose only decider is a verified owner act, in a new package that nothing
    calls yet (off); and a design for how it reaches the host proxy, the
    host fetch service (NET-01) and the mirrors (NET-02).
  artifact: >-
    Commits on lane/fp10-net05-grants: the problem record and this file
    (green); docs/design/egress-grant-object-2026-10-21.md (green);
    internal/egressgrant with its tests (red, unwired).
  acceptance:
    - action: go test ./internal/egressgrant; go vet ./internal/egressgrant
      proves: the failure list below is refused, fail closed
      evidence_class: local test
    - action: grep for egressgrant imports outside the package at the frozen ref
      proves: the slice is off (no caller, no route, no boot step)
      evidence_class: local test
  rollback: git revert of the code commit; nothing reads the package, nothing is stored
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci]
    note: >-
      Nothing to prove on staging until wiring: the package has no caller.
      Landing rides any deploy (it is inert); FP0 confirms CI only.

value:
  better_means: >-
    Open egress (L1, L3, L4) becomes grantable at all, by the owner only,
    for a bounded time, under one record type the proxy and the app-desk
    installer will both read.
  goodharting_would_be: >-
    A record type that validates but whose decide path can be reached
    without a verified owner act; an expiry the checker does not read; or a
    second grant shape for app desks.

homotopy:
  realism_axis: >-
    Unit tests of the pure package, then (later stations) the proxy reading
    a grant in observe mode on a test computer, then enforcement.

boundaries:
  mutation_class: green (docs); red (the package: egress authority), shipped unwired
  authority_sources:
    - FP0's NET-05 brief, 2026-10-21
    - SDC NET-05 and NET-10; docs/design/engineering-network-grants-2026-10-10.md §4.0a, §4.2, §5
    - docs/design/app-agent-network-policy-2026-10-21.md
    - docs/choir-doctrine.md and CLAUDE.md
  must_preserve:
    - No edits to vmctl, vmmanager, the VM image (FP8's lane), the capsule egress proxy (FP6) or the host proxy.
    - L2 stays the default for implementation capsules and is untouched.
    - The slice is off: no caller, no route, no stored kind, no boot step.
    - Every decide is a verified owner act bound to the exact grant id; no other decider in this slice.
    - Problem documentation first; the test before the code.
  excluded:
    - Wiring into the proxy, capsule capability, management or engineering verbs, Texture, the store.
    - Tier computation, signals, research vetting, category grants (grants design §4.3 to §4.4).
    - The widening diff for manifests (NET-10 §7 step 2) and the manifest schema (ARCH-02).
  protected_surfaces: [egress authority (new, unwired)]

now:
  status: working
  slice: problem record and this station file
  source_ref: d1c60ad87
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-net05-grants
    state: drafting
    ref: lane/fp10-net05-grants
    base: d1c60ad87
    digest: none
    scope: [docs/problems/no-grant-object-for-open-egress-2026-10-21.md, docs/definitions/choir-launch-fp10-net-05-2026-10-21.md]
  conjecture:
    id: C-one-grant-model
    claim: >-
      One EgressGrant record, holder-typed (engineering assignment or app
      desk), carries both engineering open-egress grants and app-desk
      manifest grants, with the owner act as the only decider.
    test: >-
      The package's tests build both holder kinds from the declaration forms
      in the grants design §4.2 and NET-10 §2.1, and the same validate,
      decide and check path accepts and refuses them.
    edge: missing_oracle
    delta_o: the proxy wiring (design §5) is the first real reader
    scope_if_supported: the grant model, not enforcement
    status: open
    evidence_refs: [docs/problems/no-grant-object-for-open-egress-2026-10-21.md]
  belief:
    believed_state: no grant object exists; owner-act receipts exist and can bind a decision
    main_uncertainty: how NET-01's host fetch service and NET-02's mirrors name a destination
    next_observation: their lane designs when FP0 lands them
  blocker_or_risk: none
  next_action: write the design, then the tests from the failure list, then the package
---

# FP10 NET-05: the grant object for open egress

Problem: [no grant object for open egress](../problems/no-grant-object-for-open-egress-2026-10-21.md).
Design: [egress grant object](../design/egress-grant-object-2026-10-21.md).

## Failure list (written before the tests)

The package must refuse, with an error and no state change:

1. a grant with no holder, or a holder kind other than engineering
   assignment or app desk;
2. a level other than L1, L3, L4 (L2 is not a grant; it is the default);
3. a destination that is a private, loopback, link-local, CGNAT, multicast,
   unspecified or metadata address, any IP literal, `localhost`, or a
   `.internal`/`.local`/`.localhost` name;
4. a wildcard destination below L3, or any wildcard or L4 on an app desk;
5. a non-HTTPS scheme at L1 or L3;
6. an L4 grant without the `no_private_data_mounted` condition;
7. an app-desk grant with no verb or effect class, or a `visible-write`
   with no operation contract;
8. a missing or non-positive budget, or one above the hard caps;
9. no expiry, `expires_at` not after `not_before`, or a lifetime over the
   cap (24 hours for an assignment, the renewal cap for an app desk);
10. a decide with no owner act, an owner act that fails verification, a
    different act kind, another computer, another grant id, a decision word
    other than `grant` or `deny`, or a mismatch between the act's decision
    and the one asked;
11. a decide on a grant that is not `requested` (no replay, no regrant);
12. a check that allows a grant that is not `granted`, before `not_before`,
    at or after `expires_at`, or for a destination or method it does not
    cover;
13. a grant id that does not equal the digest of its scope and binding (a
    mutated record).
