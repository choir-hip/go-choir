> **Superseded / redirect:** This document is archived source material. The
> current executable authority for OG/Dolt/heresy completion is
> `../definitions/og-dolt-heresy-completion-2026-07-08.md`. A `/goal` against
> this file should redirect there. Internal harness strings and unarchived
> `docs/...` paths below are historical source material only, not current
> executable targets or authority paths.

# Heresy Eradication Mission

## Harness Invocation Semantics

```text
/goal docs/definitions/heresy-eradication-2026-07-07.md
```

Read this document as executable semantic authority. Execute autonomously until
its completion semantics are satisfied with named evidence, or until a sharply
evidenced escalation, blocker, or supersession condition is met. Do not treat
this document as a plan, checklist, or summary. Its definitions govern what
"heresy," "eliminated," and "done" are allowed to mean.

## Source Authority Order

1. This document (definition graph + determined state + completion semantics)
2. Owner statements, 2026-07-06/07: object graph becomes canonical by hard
   cutover; Dolt version-control features become load-bearing; all named
   heresies are eliminated and doctrine prose is replaced by executable
   enforcement; candidate computers are no longer VMs — capsules (containers)
   host what candidate VMs used to, over substrate-independent audited
   computers.
3. `docs/definitions/substrate-independent-audited-computer-2026-07-04.md`
   (ComputerVersion, materializer, route-over-computer-version)
4. `docs/choir-doctrine.md` heresy registry (H001–H031) — registry entries are
   the per-heresy authority for bad pattern and blessed replacement
5. `docs/mission-og-dolt-heresy-hard-cutover-v0.md` (program sequencing this
   mission executes the heresy track of)
6. `docs/assessment-overall-state-2026-07-07.md` (evidence baseline)
7. `AGENTS.md` (repo operating contract, mutation ceremony)

Where this document conflicts with older mission docs that treat wiring as
pending, internal/runtime as a zombie, or candidates as VMs, this document
governs.

## Mutation Class

Authoring this definition is **yellow**. Execution is **red** wherever it
touches Texture canonical writes, run acceptance, promotion/rollback, conductor
routing, the store schema, vmctl lifecycle, or public API routes; red passes
use the AGENTS.md ceremony (conjecture delta, protected surfaces, admissible
evidence class, rollback path, heresy delta).

## Real Artifact / Object Of Work

The real object is not a cleaned-up codebase. It is a **system whose invariants
are mechanically enforced**: every named heresy eliminated (code deleted or
replaced, tests inverted, external contracts migrated), a detector that fails
CI if the pattern returns, and a doctrine document reduced to thesis +
invariants + pointers to enforcement. The registry closes; the detectors stay.

Subordinate projections: detector manifest + CI job; per-cluster deletion
diffs; inverted tests; migration shims (410 handlers) with removal dates;
doctrine/README/ontology updates; the heresy-delta ledger.

## Mission Purpose And Non-Purpose

**Purpose:** Take the live heresy set to zero — parent/child residue, dual
execution paths, continuations, texture forcing, acceptance overclaim,
non-durable obligations, vocabulary drift, retired app surfaces, and the newly
registered candidate-computer-as-VM cluster — in dependency order, with
executable detectors as the permanent guard, so doctrine prose can shrink
without the invariants weakening.

**Non-purpose:**

- Not the object-graph cutover or Dolt adoption themselves (sibling tracks in
  mission-og-dolt-heresy-hard-cutover-v0; this mission must not port heresies
  into them, and sequences around them).
- Not a rewrite of internal/runtime; business-logic extraction is its own
  mission — this one deletes dual paths inside what exists.
- Not documentation cleanup for its own sake; every doc change must close a
  registry entry or correct a premise agents optimize against.
- Not detector theater: a detector that cannot fail is not evidence.

## Definition Graph

### 1. Term: `heresy`

```yaml
id: heresy
kind: term
status: settled
source: docs/choir-doctrine.md
definition: A named, registered pattern in code, docs, tests, prompts, or routes that violates a doctrine invariant — typically a dual path, a deprecated control channel, or vocabulary that teaches agents the wrong ontology.
non_definition:
  - Any code smell an agent dislikes.
  - Unregistered suspicions (register first, then eliminate).
observables:
  - A registry entry with bad pattern, blessed replacement, and detector vocabulary.
execution_effect:
  - Only registered heresies are elimination targets; new discoveries are registered (discovered++) before repair.
settlement:
  rule: Settled by the registry format in choir-doctrine.md.
  settled_by: human
```

### 2. Term: `eliminated`

```yaml
id: eliminated
kind: term
status: settled
source: owner-stated + doctrine I5
definition: A heresy is eliminated only when (a) the pattern's code is deleted or replaced by the blessed pattern at every production call site, (b) tests asserting the old behavior are inverted or removed and tests pinning the blessed behavior exist, (c) external contracts (API routes, wire fields, frontend) are migrated or explicitly shimmed with a removal date, (d) a CI detector fails on the pattern's reappearance, and (e) the registry entry is closed citing (a)–(d).
non_definition:
  - Deprecation comments.
  - Doc closure without code deletion.
  - Detector green because the symbol was renamed.
  - Test deletion (tests must be inverted, not silenced).
counterexamples:
  - Continuations marked deprecated while /api/continuations/* still serves requests.
  - H005 "closed" while ensureSpawnedCoagentWorkItem still has call sites.
observables:
  - Zero production grep hits for the detector patterns; detector job in fail mode; registry entry closed with evidence refs.
execution_effect:
  - No cluster may be reported repaired below this bar; heresy-delta `repaired` counts only eliminations.
settlement:
  rule: Per cluster, by detector-zero + inverted tests + closed registry entry.
  settled_by: evidence
```

### 3. Object: `detector-manifest`

```yaml
id: detector-manifest
kind: object
status: proposed
definition: A structured manifest (scripts/check-heresies.sh + per-family pattern list) mapping each heresy family to grep/lint patterns with allow-contexts (tests, doctrine, archive, provenance-only) and reject-contexts (production code, wire contracts), wired into CI.
non_definition:
  - A one-off grep in a mission log.
  - A detector with an empty reject set.
observables:
  - CI job exists; per-family mode is discovery (log) or enforce (fail); baselines recorded per phase.
execution_effect:
  - Each cluster's elimination flips its family from discovery to enforce in the same change.
formalization:
  status: required
  note: Families whose invariants are stateful (authority scoping, promotion) also need spec/property coverage, not only greps.
settlement:
  rule: Settled when every registry family has patterns, a baseline, and a mode.
  settled_by: evidence
```

### 4. Object: `heresy-H031-candidate-computer-as-vm`

```yaml
id: heresy-H031-candidate-computer-as-vm
kind: object
status: proposed
source: owner-stated 2026-07-07
definition: Implementing the candidate-computer concept as VM identity — fork by cloning a VM/image, speculative mutation inside a candidate VM, promotion/rollback as VM-route or image operations, or any route resolving to a VM/desktop ID.
non_definition:
  - The candidate concept itself (a forked ComputerVersion is blessed).
  - Materializers using VMs as one substrate behind the capability boundary.
examples:
  - vmctl candidate-desktop publish/switch lifecycle (internal/vmctl/handlers.go:312, client.go:191).
  - internal/computerversion/candidate_computer_package*.go capturing VM state as candidate identity.
  - Proxy wire route hard-coded to owner="universal-wire-platform", desktop="platform" → vmctl VM resolve (internal/proxy/handlers.go:970) — the 2026-07 wire hang was this heresy expressing itself.
counterexamples_of_bad:
  - Candidate = (CodeRef, forked ArtifactProgramRef); effects execute in capsules (internal/capsule, tools_capsule.go); promotion = atomic route flip between ComputerVersions.
observables:
  - Registry entry exists in choir-doctrine.md (being registered 2026-07-07); detector patterns: candidate desktop, route→desktop_id, DoltHeadSnapshot-as-promotion-identity.
execution_effect:
  - Elimination is gated on route-over-computer-version + promotion-over-ComputerVersion landing (mission-og-dolt Phase 4); until then the cluster is frozen — no new accretion — under a named deletion clock.
settlement:
  rule: Settled when no route resolves to a VM identity and candidate lifecycle code is deleted or re-pointed to ComputerVersion refs.
  settled_by: evidence
```

### 5. Invariant: `kill-order`

```yaml
id: kill-order
kind: invariant
status: settled
source: elimination research 2026-07-07
definition: Clusters are eliminated in dependency order — M3.1 texture forcing (H009–H012, H024a/b, H026) → M3.2 parent/child (H001–H005, H015–H016) → M3.3 acceptance + durable obligations (H013–H014, H017–H018) → M4 continuations (H006–H008) → M5 surfaces/vocabulary (H019–H029, parallel-anytime) → H031 candidate-VM (gated on Phase 4 of the OG/Dolt mission). Store-resident heresies (continuations, parent/child columns) die before their entities migrate to the object graph.
execution_effect:
  - Never port a heresy into the object graph; never delete a cluster whose prerequisite proof gate is open.
settlement:
  rule: Settled by the dependency analysis; reopened only if a proof gate falsifies a prerequisite.
  settled_by: evidence
```

### 6. Term: `proof-gate`

```yaml
id: proof-gate
kind: term
status: settled
definition: The evidence that must exist before a cluster's deletion is safe. M3.1 — Texture produces honest first revisions and unforced delegation decisions (new test suite). M3.2 — all authority flows are trajectory-scoped; zero GetLatestActiveRunByAgent production call sites. M3.3 — blockers/questions/assignments materialize as durable work items; acceptance levels match evidence classes. M4 — zero production continuation callers; work-item passivation + trajectory settlement demonstrably cover continuation semantics. H031 — routes and promotion records name ComputerVersion.
execution_effect:
  - A deletion pass without its gate's evidence is a forbidden construct; write the gate test first.
settlement:
  settled_by: evidence
```

### 7. Target Conjecture: `doctrine-as-enforcement`

```yaml
id: doctrine-as-enforcement
kind: target_conjecture
status: proposed
source: owner-stated ("get rid of all doctrines")
definition: Prose doctrine can shrink to system thesis + a small invariant set + pointers, with the enforcement burden carried by TLC-checked specs and the fail-mode detector manifest, without invariant regression.
test: After M4, rewrite choir-doctrine.md to the reduced form; run one full CI cycle plus one adversarial pass attempting to reintroduce each eliminated pattern; every attempt must be caught by a detector or spec, not by a human reading prose.
execution_effect:
  - Registry entries close into detector references; the grip checkpoint (docs/choir-grip-checkpoint-2026-07-07.md) becomes the narrative layer; framing updates (human-improving; World Wire) land in the reduced doctrine.
settlement:
  rule: Settled by the adversarial reintroduction test.
  settled_by: evidence
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: H030 (actor polling) is repaired; actor mailbox is a Go channel.
      source: observed (internal/actor/actor.go:141; 2026-06-27 memo)
      execution_effect: Registry closure only; not an elimination target.
    - claim: The actor runtime is the only execution substrate (dispatchActor panics if nil); internal/runtime is live business logic, not a zombie.
      source: observed (internal/runtime/runtime.go:76-80; assessment 2026-07-07)
      execution_effect: Elimination work targets dual paths inside a wired system; no re-wiring passes.
    - claim: Live cluster baselines — H001-05 ~10 sites (spawned work items at runtime.go:759,902,1342,1435; GetLatestActiveRunByAgent ×5 prod); H006-08 ~200 sites incl. store/continuations.go (~146 LOC) and /api/continuations/* routes; H009-24 ~50 sites (next_required_tool ×49); H013-18 ~20; H019-29 ~100 (frontend/docs).
      source: observed (elimination research 2026-07-07)
      execution_effect: These are the detector baselines; enforcement flips to fail at zero per family.
    - claim: Capsule execution is wired — internal/capsule (landlock/seccomp/namespaces/capability broker) + tools_capsule.go (spawn_capsule, mint_capability, capsule_exec, commit_transaction).
      source: observed
      execution_effect: H031's blessed replacement exists; its elimination is sequencing, not invention.
    - claim: Candidate computers are not VMs; capsules host speculative effects; the product object is ComputerVersion.
      source: owner-stated 2026-07-07 + substrate-independent definition doc
      execution_effect: H031 registered and frozen pending Phase 4 gate.
  contested:
    - node: proof-gate (M4 slice)
      issue: Whether work-item passivation + trajectory settlement fully cover continuation semantics is asserted but not demonstrated.
      next_resolution_step: Write the coverage test — for each continuation call pattern in tests, show the equivalent settlement path; falsify or settle.
    - node: proof-gate (M3.1 slice)
      issue: Texture agency without forcing has no staging evidence yet.
      next_resolution_step: texture_no_forcing test suite + one staging trajectory with forcing disabled behind a flag.
  open:
    - node: detector-manifest
      missing: The manifest file, CI job, and per-family baselines do not exist yet. First construct of this mission.
    - node: heresy-H031-candidate-computer-as-vm
      missing: Registry entry (in flight via doc-update pass, 2026-07-07); detector patterns unbaselined.
```

## Invariants

```yaml
invariants:
  - id: HE-I1
    rule: Detector lands with or before its cluster's deletion; never delete a pattern the CI cannot yet see return.
  - id: HE-I2
    rule: Tests are inverted, never silenced; every deleted behavior gets a test pinning the blessed replacement.
  - id: HE-I3
    rule: Store-resident heresies die before their entities migrate to the object graph (no heresy fossils in og_objects kinds).
  - id: HE-I4
    rule: No spec weakened and no detector allow-list widened to make a deletion pass; widening requires a registry amendment.
  - id: HE-I5
    rule: External contracts (choir CLI shapes, frontend routes) are migrated or shimmed with 410 + removal date; never silently broken.
  - id: HE-I6
    rule: Heresy-delta accounting on every pass: discovered / introduced / repaired, with `repaired` held to the `eliminated` bar.
  - id: HE-I7
    rule: H031 deletion is gated on route-over-computer-version evidence; until then the cluster is frozen against new accretion (detector in enforce mode for NEW sites, discovery for existing).
```

## Authority Boundaries

```yaml
authority_boundaries:
  orchestrator:
    may:
      - build the detector manifest and CI job
      - write proof-gate tests
      - execute deletion passes in kill-order under red ceremony
      - register newly discovered heresies (discovered++)
      - close registry entries meeting the eliminated bar
    must_escalate:
      - any deletion whose proof gate is contested and cannot be settled by evidence in-mission
      - external contract breaks without a shim path
      - reduction of choir-doctrine.md (doctrine-as-enforcement construct) — owner reviews the reduced text before it replaces the canonical doc
      - anything touching production user state
```

## Homotopy / Realism Parameters

Valid simplifications: detector families in discovery mode before enforce;
per-cluster deletion behind a flag for one staging cycle; 410 shims during
migration windows. Fake islands: renaming a symbol so the detector goes green;
deleting a failing test instead of inverting it; closing a registry entry on a
doc edit; counting deprecation comments as `repaired`; a detector whose reject
set matches nothing.

## Variant / Progress Measure

```text
variant = open registry entries
        + detector families not in enforce mode
        + contested proof gates
        + production call sites across all cluster baselines
```

Current (2026-07-07): ~31 registry entries (H030 pending closure, H031 pending
registration), 0 detector families exist, 2 contested gates, ~380 baseline
sites. Target: 0 open entries, all families enforcing, 0 contested gates, 0
production sites. A pass that reduces none of these is motion theater.

## Execution Operators And Control Loop

Use the Definition skill's standard operators (define, probe, construct,
verify, settle, monitor) over this graph, receding-horizon. Default next
constructs in order: (1) detector-manifest + CI job with discovery baselines;
(2) H030 registry closure; (3) M3.1 proof-gate test suite; then kill-order.

## Dense Feedback Channels

- Detector CI on every push (the mission's own instrument).
- Focused Go tests per cluster (inverted + blessed-pattern pins).
- `go test ./... ` + doccheck for each deletion pass.
- choir CLI against staging/production for external-contract checks
  (trajectories/texture shapes; /api/continuations must 410 after M4).
- TLC for promotion/authority specs where clusters touch stateful invariants.

## Evidence Ledger

```yaml
evidence:
  - claim: Full deletion inventory with file:line, prerequisites, and blast radius exists for every cluster H001–H029.
    definition_node: kill-order
    evidence_class: observed file (research report, 2026-07-07 session)
    source: heresy elimination research; docs/mission-og-dolt-heresy-hard-cutover-v0.md
    result: Kill sequence M3.1→M3.2→M3.3→M4→M5 adopted with per-cluster cards.
    uncertainty: Line numbers drift with ongoing work; re-grep before each pass.
    promotion_relevance: Authorizes deletion passes in order.
  - claim: Capsule effect-chamber replacement for candidate VMs is implemented and tool-wired.
    definition_node: heresy-H031-candidate-computer-as-vm
    evidence_class: observed file
    source: internal/capsule/*, internal/runtime/tools_capsule.go
    result: Blessed replacement exists; H031 elimination is not blocked on new construction.
    uncertainty: bash_in_capsule opt-in flag path and broker deployment status unverified in staging.
    promotion_relevance: Grounds the H031 registry entry.
```

## Completion Semantics

This mission is **COMPLETE** only when all hold with named evidence:

1. Detector manifest exists; every registry family has patterns, a baseline,
   and is in **enforce** mode; CI fails on seeded reintroduction of at least
   one pattern per family (non-ceremonial proof).
2. Clusters M3.1, M3.2, M3.3, M4 each meet the `eliminated` bar: zero
   production sites, inverted tests, migrated/shimmed contracts, closed
   registry entries.
3. M5 surface/vocabulary residue removed (frontend launchers, lease
   vocabulary, Live/Target/Retired doc split).
4. H031: no route resolves to a VM identity; candidate lifecycle code deleted
   or re-pointed to ComputerVersion; registry entry closed. (May complete
   after 1–3 if the Phase 4 gate is still open — in that case mission exit is
   `checkpoint_incomplete` with H031 as the sole remaining error field, never
   `complete`.)
5. `doctrine-as-enforcement` settled: reduced choir-doctrine.md accepted by
   owner; adversarial reintroduction test passed.
6. Heresy-delta ledger shows repaired ≥ discovered-at-start with zero
   uninverted deletions; choir CLI contract checks green before/after.

## Escalation Rules

Escalate only: doctrine reduction review (owner taste); contested M4 coverage
gate if evidence cannot settle it; any pass requiring production-state
mutation; discovery of a heresy whose blessed replacement does not exist yet
(register + freeze, escalate for replacement authority).

## Forbidden Collapses

- detector green -> heresy eliminated (rename evasion)
- registry closed -> code deleted
- deprecated -> eliminated
- route 410 -> contract migrated (shims carry removal dates)
- doc updated -> agents retrained (detectors are the retraining)
- capsules exist -> H031 eliminated (sequencing gate still applies)

## Rollback And Resumption Policy

Each deletion pass is one revertable commit citing its registry entries and
detector flips. Rollback = revert + detector family back to discovery.
Resume from Run Checkpoint below; re-grep baselines before resuming any
in-flight cluster.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: definition authored 2026-07-07; doc-update passes in flight (doctrine H031 registration, README/ontology/AGENTS corrections)
  current_artifact_state: no detector manifest yet; all clusters at research baselines
  what_shipped: []
  what_was_proven: []
  unproven_or_partial_claims:
    - M3.1 texture agency (contested gate)
    - M4 continuation coverage (contested gate)
  highest_impact_remaining_uncertainty: M4 coverage gate
  next_executable_probe: construct detector-manifest + CI job in discovery mode; record baselines
  suggested_goal_string: "/goal docs/definitions/heresy-eradication-2026-07-07.md"
  evidence_artifact_refs:
    - docs/assessment-overall-state-2026-07-07.md
    - docs/mission-og-dolt-heresy-hard-cutover-v0.md
  rollback_refs: []
```

## Suggested Goal String

```text
/goal docs/definitions/heresy-eradication-2026-07-07.md
```
