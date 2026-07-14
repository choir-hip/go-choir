# Choir Autoputer Completion

## Invocation And Status

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-13.md
```

**Status:** working. This is the sole executable product mission and supersedes
`docs/definitions/choir-autoputer-completion-suite-2026-07-11.md`.

The predecessor remains append-only historical evidence. Do not resume its
B0-S9 orchestration, locks, delegation transactions, or documentation-per-stage
rhythm. Import only the settled receipts named in this Definition. The same
command above resumes this mission after interruption.

## Mutation Class And Ceremony

This authority/registry cutover is **yellow**: it changes future optimization
pressure and mission semantics, not deployed product behavior. Rollback is a
single revert of the cutover commit. The successor was requested by the owner
after the prior S3 topology stopped converging.

Runtime work governed by this Definition is normally **orange**. A mutation is
**red** when it changes Texture canonical writes, Trace/evidence, promotion or
rollback, candidate computers, auth/session renewal, vmctl, gateway/provider
calls, run acceptance, or deployment routing. Red work records conjecture
change, protected surfaces, admissible evidence, rollback, and heresy delta in
the canonical state capsule before implementation.

## Real Artifact And Purpose

The product object is one persistent user computer that an external agent can
inspect, operate, change through a candidate, verify, promote, roll back, and
continue after restart. Its machine-world state lives in the computer
filesystem and VM snapshot. Its Dolt semantic state uses exactly two
non-conflated stores:

1. the world-wire `ObjectGraphStore`, served by corpusd in sql-server mode;
2. one VM-local embedded Dolt workspace per user computer for app state and
   branch-based promotion/rollback.

Route-slot tables live with corpusd control truth but do not form a third
product-state store. vmctl is their sole CAS writer.

This mission completes the autoputer before Choir-in-Choir or a newly defined
Autopaper mission may begin. It must:

1. dissolve `internal/runtime` into cohesive owners and delete it;
2. prove audited-computer construction and external observation;
3. establish artifact-verified run truth on the extracted core;
4. prove candidate self-development, promotion, and rollback;
5. prove contained co-super authority before Choir-in-Choir;
6. complete the alias-free vocabulary cutover and successor handoff.

## Non-Purpose

- No Autopaper editorial or reconciler activation.
- No third state store, route ledger, or shadow current-state projection.
- No harness-owned semantic workflow engine.
- No raw-export, accessor, facade, callback, interface, wrapper, forwarding,
  alias, duplicate handler, or temporary registry seam used to make a package
  move compile.
- No broad vocabulary sweep while deletion and correctness are active.
- No documentation receipt for every process transition. Durable state changes
  are coalesced with the Define or Implement boundary they govern.
- No local-only claim for shared-platform behavior.

## Source Authority Order

1. Owner direction recorded here: supersede the prior suite, preserve the
   autoputer objective, and use the refined Definition/consensus execution
   model.
2. `AGENTS.md`, `docs/standing-questions.md`, and
   `docs/choir-doctrine.md`.
3. `docs/computer-ontology.md` and `docs/agent-product-doctrine.md`.
4. This Definition's kernel and canonical state capsule.
5. Active subordinate contracts named by the phase graph, within their scope.
6. `docs/runtime-dissolution-inventory.yaml` and observed repository, CI,
   staging, and product artifacts.
7. The predecessor Definition and its evidence references as historical
   receipts only.

A lower source may refine implementation but cannot add a third store, weaken
atomic cutover, revive retired ontology, reorder this graph, or claim success.

## Definition Graph

```yaml
definition_nodes:
  - id: one-mission-authority
    kind: authority
    status: settled
    settled_by: owner
    source: user-stated
    definition: This file is the only executable product mission until complete, blocked_incomplete, or explicitly superseded.
    non_definition: A registry view, subordinate contract, or historical Definition is not a second mission authority.
    observables: [one mission-graph entrypoint, one active authority-manifest root, one ACTIVE goal string]
    execution_effect: Registries expose exactly this /goal entrypoint; subordinate Definitions are contracts, not separate runs.
    invalidation_triggers: [owner-authorized successor lands atomically across all registries]

  - id: autoputer-before-successors
    kind: term
    status: settled
    settled_by: owner
    source: user-stated
    definition: Autoputer completion precedes Choir-in-Choir and any new Autopaper authority.
    non_definition: Partial external operability does not authorize either successor.
    observables: [R1-R7 completion receipts, contained Choir-in-Choir acceptance]
    execution_effect: Later phases and successor work remain closed behind named acceptance evidence.
    invalidation_triggers: [new owner direction]

  - id: two-store-topology
    kind: authority
    status: settled
    settled_by: owner
    source: user-stated
    definition: Dolt semantic state uses exactly two non-conflated stores—the corpusd world-wire store and one VM-local embedded store per user computer—while filesystem/VM snapshots own machine-world state; vmctl alone CAS-writes corpusd route slots.
    non_definition: Machine-world filesystem state is not a Dolt store, the two Dolt stores are not interchangeable, and route-slot control rows are not a third product-state store.
    observables: [world-wire corpusd store, VM-local embedded Dolt workspace, corpusd route-slot tables, vmctl writer call graph, promotion certificate]
    execution_effect: Preserve VM-local app-state and promotion authority, world-wire publication authority, and sole-writer route CAS; no mission may create a third Dolt domain or write tracked source directly on Node B.
    invalidation_triggers: [new owner topology decision]

  - id: atomic-runtime-extinction
    kind: invariant
    status: settled
    settled_by: owner
    source: user-stated
    definition: Runtime extraction moves one complete ownership boundary and all production callers, then deletes the old path without aliases or dual authority.
    non_definition: A package split, facade, accessor, wrapper, or test-only caller move is not extraction.
    observables: [generated ratchet decreases, production caller cutover, old declarations deleted, focused behavior unchanged]
    execution_effect: Every source mutation is bounded by the runtime ratchet and clean-cutover checks.
    invalidation_triggers: [new owner cutover rule]

  - id: dependency-ordered-dissolution
    kind: execution_order
    status: settled
    settled_by: orchestrator
    source: observed
    definition: When transport ownership is compile-coupled to private domain behavior, first extract a cohesive typed domain operation boundary, then move that domain's thin HTTP shell and delete the runtime copy.
    non_definition: This does not authorize raw private exports, a general runtime API, or indiscriminate step-4 extraction.
    observables: [S3 compile-falsification receipt, current dependency graph, finite domain candidate, exact production callers]
    execution_effect: S3's transport-before-domain order is retired. This ordering is binding for execution but does not invent new product authority; owner ratification remains required for any genuinely new protected authority boundary.
    invalidation_triggers: [new compile evidence falsifies domain-first order, owner changes ordering or authority]

  - id: artifact-verified-success
    kind: evidence_class
    status: settled
    settled_by: owner
    source: user-stated
    definition: Completion requires product-path artifacts, verifier contracts, restart durability, and staging truth for protected surfaces.
    non_definition: Compile, unit, local, worker, consensus, health, or deploy identity alone is not product acceptance.
    observables: [E4 deploy identity, E5 product transition, E6 independent recomputation, E7 protected acceptance]
    execution_effect: Every success claim is limited to the strongest evidence class actually observed.
    invalidation_triggers: [stale artifact identity, reproduced product failure, verifier contract failure]

  - id: define-implement-rhythm
    kind: operator
    status: settled
    settled_by: orchestrator
    source: user-stated
    definition: One compact Define boundary authorizes one coherent Implement boundary; implementation, focused tests, capsule update, and local evidence land together.
    non_definition: Dispatch intents, lock renewals, worker returns, consensus starts, and routine checkpoints are not separate durability boundaries.
    observables: [one pre-implementation mutation lock, one implementation commit, coalesced capsule and local evidence]
    execution_effect: Process telemetry stays in external evidence; Git records semantic and implementation boundaries.
    invalidation_triggers: [lost restart durability, conflicting authority, owner changes operating preference]
```

`dependency-ordered-dissolution` records the owner's stated preference for
extracting domain behavior before the remaining handler move and the compile
receipt that falsified the old order. It is not permission to expose 77 private
runtime declarations or to move step-4 behavior indiscriminately. Each accepted
domain boundary must be finite, cohesive, typed, behavior-preserving, and
independently reviewable.

## Invariants

1. Exactly one canonical state capsule exists: the YAML block in this file.
2. State authority is not duplicated in plans, reports, generated dashboards,
   branch-local files, or agent output.
3. Only one source mutation lock may be active. Read-only mapping and review may
   run concurrently after an immutable base identity is recorded.
4. A Define beat names the object, boundary, invariants, exact source scope,
   evidence class, rollback, and close condition before an Implement beat.
5. A changed source or authority base invalidates the candidate and its reviews.
6. Every runtime slice reduces a mechanically generated ratchet; no count may
   increase and no new alias/indirection category may appear.
7. Tests defend observable behavior, transitions, boundaries, precedence, and
   real errors; they do not test source text or plumbing.
8. New reliable product failures are documented in the next Define boundary
   before a behavior-changing fix. The problem record and revised mutation lock
   share one commit; process receipts do not.
9. Source-changing work follows `commit -> push origin main -> CI -> staging
   deploy -> deployed identity -> product-path acceptance` before closure.
10. A worker result, panel majority, local test, health endpoint, or deployed SHA
    cannot certify its own claim.
11. Failed candidates leave exact diagnostics and a clean worktree or an
    attributable recovery ref. No hidden WIP may become mission state.
12. The predecessor suite is evidence, not a second active authority.

## Assurance Profiles

Assurance is selected by mutation class, not by habit.

```yaml
assurance_profiles:
  green:
    examples: [ordinary prose, comments, labels]
    second_opinion: none
    verifier: focused deterministic check
    panel: none
  yellow:
    examples: [tests, ratchets, detector manifests, prompt framing, mission semantics]
    second_opinion: independent verifier required
    panel: compact when authority or optimization pressure changes
  orange:
    examples: [runtime ownership, product APIs, app state, database queries]
    second_opinion: independent implementation review required
    panel: compact at the Define boundary or batch close, not per process transition
  red:
    examples: [Texture writes, Trace, promotion, rollback, candidate computers, auth, vmctl, gateways, run acceptance, deploy routing]
    second_opinion: independent verifier plus multi-family panel
    panel: full before commit and again only if the reviewed candidate changes
```

A compact panel uses two independent model families. A full panel uses three or
more independent families. The default hard deadline is 180 seconds per
reviewer. Review outputs are evidence artifacts keyed by candidate SHA; they do
not become their own Git commits. `consensus` means review evidence exists;
`adjudicated` means the orchestrator resolved every blocking finding against
the immutable candidate. Majority does not override one reproduced blocker.

## Operators

The orchestrator may:

- `define`: close ambiguity and write the next exact mutation lock;
- `implement`: execute only that lock in an isolated worktree or clean canonical
  tree;
- `verify`: run focused behavior checks and inspect the exact candidate diff;
- `request_second_opinion`: run the assurance profile against an immutable
  candidate;
- `adjudicate`: accept, repair, reject, or escalate findings with evidence;
- `settle`: land implementation, capsule change, and local evidence together;
- `probe`: gather read-only repository, CI, staging, or product evidence;
- `rollback`: use the recorded parent/ref when postconditions fail;
- `supersede`: create one successor authority and cut registries atomically.

Use cognitive transforms only when they change the next probe, route, scope,
verifier, evidence plan, or stop rule. Three failed attempts around one boundary
trigger substrate/dependency reassessment, not a fourth incremental patch.

## Phase Graph

Phases are ordered. A later phase may be mapped read-only but cannot mutate its
product surface before its predecessor closes.

### R1 — Runtime Dissolution By Dependency Order

Subordinate contracts:

- `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` for deletion gates;
- `docs/runtime-dissolution-inventory.yaml` for the checked baseline;
- predecessor S3 evidence refs listed in the capsule for already landed work.

Ordered frontier:

1. **Boundary compilation.** Reconcile the generated inventory and production
   caller/dependency map at current `origin/main`. Partition private runtime
   behavior into finite domain-operation groups. Each candidate names existing
   behavior, intended owner, all production callers, tests, protected surfaces,
   and the exact old declarations deleted after cutover.
2. **Domain-first extraction.** Select the smallest cohesive group whose typed
   operations can move without raw accessors or a general runtime facade. Move
   behavior and its direct tests to the natural owner. A temporary dual path is
   forbidden; if transport still calls the moved operation, it must call the
   new owner directly in the same landing.
3. **Thin transport cutover.** Once a domain group has a real owner, move that
   group's HTTP receiver, DTO, route binding, and test ownership to
   `internal/apihandler` in one atomic landing. Delete the runtime receiver and
   imports. Keep the canonical 46-slot route table and one `server.Server`
   product/API tool unchanged.
4. Repeat 1-3 until the remaining handler transport is mechanically movable,
   then complete the one-type/constructor/receiver cutover without aliases.
5. Continue dependency-ordered slices until every runtime production importer,
   export, route, tool, wrapper, file, and literal citer reaches zero; delete
   `internal/runtime` and make the ratchet extinction target pass.

No candidate may weaken a ratchet to fit the slice. If the smallest cohesive
boundary is too broad for independent review or rollback, subdivide by product
domain, not by introducing indirection.

R1 closes only when the directory is absent, all ratchet categories are zero,
focused and broad Go tests pass, CI/deploy are green, staging product behavior
is accepted, and an independent verifier recomputes the extinction result.

### R2 — Audited Computer

Subordinate contract:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phases 1-2.

Prove computer construction/reconstruction, causal build input, Nix/build
identity, service health, persistent state, retained audit history, clean
restart, and no fate-sharing between computer state and control truth.

### R3 — External Operator Truth

Subordinate contract:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 3.

An external agent using the approved CLI must observe identity, build/deploy
state, route, health, work/run state, recent failure, promotion history, and
rollback evidence without SSH, database access, or hidden operator knowledge.

### R4 — Run Truth

Subordinate contract:
`docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`.

Establish one lifecycle authority, durable restart/resumption, artifact-verified
completion, and independent acceptance on the extracted core. Retired
continuation-level semantics must not be normalized.

### R5 — Self-Development And Promotion

Subordinate contracts:

- `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 4;
- `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` promotion gates.

Prove an external agent can inspect a computer, create an isolated candidate,
change source, build, verify, produce a promotion certificate, promote by CAS,
observe the new route, and roll back after restart.

### R6 — Containment And Choir-In-Choir

Subordinate contract:
`docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` Phase 5.

Prove contained-key scope non-escalation, guest revocation, Trace separation,
and clean restart. Only then may Choir-in-Choir authority open.

### R7 — Vocabulary Cutover And Successor Handoff

Subordinate contract:
`docs/definitions/choir-vocabulary-cutover-2026-07-11.md`.

After deletion and correctness stabilize, perform the alias-free ontology
cutover, rerun detectors and product acceptance, and author a new Autopaper
Definition only if still desired. Autoputer completion does not automatically
authorize Autopaper execution.

## Evidence Semantics

Evidence classes are scoped:

- **E0 source identity:** exact base/candidate/commit SHA and clean/attributable
  tree state.
- **E1 structural:** generated inventory, AST/caller graph, detector, ratchet,
  or compile result.
- **E2 focused behavior:** deterministic package or scenario tests.
- **E3 integration:** cross-package/process behavior with real dependencies.
- **E4 deployment:** CI, deploy receipt, and staging build/commit identity.
- **E5 product path:** user/operator-visible transition and durable artifacts.
- **E6 independent verification:** a different agent/model recomputes the claim
  from immutable artifacts.
- **E7 protected acceptance:** verifier contract, owner review where required,
  promotion/rollback receipts, containment, or run-acceptance evidence.

Each promoted claim records its node, scope, exact source/object identity,
command or method, result, evidence class, verifier independence, and artifact
reference. Store full transcripts outside the Definition; keep only durable
references in the capsule. Temporary `/tmp` references are diagnostic and must
not be the sole evidence for a completion claim.

## Completion And Stop Semantics

`status: complete` is legal only when R1-R7 are complete with named E0-E7
evidence appropriate to their surfaces; every autoputer acceptance contract is
observed after restart; no active runtime, dual-authority, third-store, hidden
operator, or retired-ontology heresy remains; and an independent verifier
recomputes completion from product artifacts.

`status: blocked_incomplete` is legal only when no safe in-bound probe remains
after root-cause clustering and observer shifts, and the capsule names the exact
missing authority or external prerequisite. A phase boundary, failed worker,
review finding, terminal exit, context loss, or pending CI is not a blocker.

`status: superseded` is legal only when one successor Definition and all three
registries land atomically. Checkpoints never imply completion.

## Canonical State Capsule

```yaml
state_capsule:
  schema_version: 1
  updated_at: 2026-07-14T00:56:02Z
  kernel_digest: sha256:cc4c4a96427ea132bb73c79e8a579247fec44dc553c8779245c0096936918e73
  expected_parent_or_authority_ref: refs/heads/main@origin@1b28520d6a3d31ecf36b2a645623367b4630faa0
  status: working
  current_subgoal: R1-prompt-store-package-cutover-03
  active_phase: R1-runtime-dissolution
  active_frontier:
    - R1-prompt-store-package-cutover-03
  locks:
    - id: R1-prompt-store-package-cutover-03
      status: defined
      mutation_class: orange
      classification_rationale: Prompt persistence and default-loading ownership are runtime behavior, so their package cutover is orange even when prompt bytes, filesystem state, and API behavior remain unchanged; no red protected surface is touched.
      conjecture: The prompt store, its direct tests, and all seeded default assets can move atomically to a top-level internal/promptstore owner with a clean Store/Descriptor/New API, without changing prompt bytes, override/reset semantics, persistent paths, role policy, routes, tools, state authority, provider routing, or model selection.
      object: prompt store and seeded-default package ownership
      selection_rationale: The store imports only settled top-level owners and the standard library, owns one cohesive persistence/default-loading domain with direct tests, and is consumed through one runtime field and accessor; moving it removes the next dependency-leaf production file plus its test and eight embedded assets.
      exact_source_scope:
        - internal/runtime/prompt_store.go
        - internal/runtime/prompt_store_test.go
        - internal/runtime/prompt_defaults
        - internal/promptstore/store.go
        - internal/promptstore/store_test.go
        - internal/promptstore/defaults
        - internal/runtime/runtime.go
        - internal/runtime/prompts.go
        - internal/runtime/test_helpers_test.go
        - cmd/doccheck/main.go
        - cmd/doccheck/main_test.go
        - docs/choir-prompting-invariants.md
        - docs/runtime-dissolution-inventory.yaml
        - docs/definitions/choir-autoputer-completion-2026-07-13.md
      production_callers:
        - internal/runtime/runtime.go
        - internal/runtime/prompts.go
        - internal/runtime/tool_profiles.go
      test_callers:
        - internal/runtime/prompt_store_test.go
        - internal/runtime/runtime_test.go
        - internal/runtime/test_helpers_test.go
      path_consumers:
        - cmd/doccheck/main.go
        - cmd/doccheck/main_test.go
        - docs/choir-prompting-invariants.md
      exact_deletions:
        - internal/runtime/prompt_store.go
        - internal/runtime/prompt_store_test.go
        - internal/runtime/prompt_defaults
      invariants:
        - Preserve every seeded default YAML byte and every LoadCore, List, Load, Save, Reset, validation, sanitization, override, reset, and filesystem persistence behavior.
        - Cut PromptStore, PromptDescriptor, and NewPromptStore directly to promptstore.Store, promptstore.Descriptor, and promptstore.New; add no alias, forwarding symbol, wrapper, accessor, callback, interface, duplicate implementation, compatibility path, or copied asset.
        - Keep the runtime PromptStore accessor only as the existing product-facing ownership edge, returning the new concrete owner; update construction and test fixtures in the same landing.
        - Move the eight embedded defaults with their owner, change only the embed-relative path, redirect current invariant citers, and retain dated historical path evidence unchanged.
        - Make doccheck classify the new default asset path as runtime-prompt and continue rejecting Markdown beside default YAML through focused behavioral tests; remove recognition of the deleted path.
        - Keep prompt content, persistent root/defaults/users layout, role policy, route registrations, tool registrations, state authorities, provider routing, and model policy unchanged.
        - Regenerate the runtime inventory without weakening debt authority. The implementation may rebaseline documentation citers from the landed 268 to exactly the mechanically observed post-Define value 270; production files, production LOC, exports, export caller edges, initial unused-export debt, routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, legacy state writers, and legacy store reads may not increase, while runtime Go files and runtime LOC must decrease.
      protected_surfaces: []
      admissible_evidence:
        - E0 clean canonical source identity
        - E1 byte identities for all eight moved YAML assets, absent old files and directory, zero stale symbols/path imports in current source, current citer redirects, and runtime ratchet PASS with decreased runtime file and LOC counts plus only the explicitly authorized documentation-citer rebaseline
        - E2 direct promptstore persistence/default tests, doccheck classification/Markdown tests, focused runtime prompt API/override/default-loading tests, and full affected package tests
        - E6 independent immutable-candidate verification
      rollback_ref: 1b28520d6a3d31ecf36b2a645623367b4630faa0
      close_condition: The old store files/default directory and stale symbols are absent, the new concrete owner contains the implementation, direct tests, and eight byte-identical YAML assets, persistent/API behavior and doccheck enforcement pass, the regenerated ratchet passes with only the authorized documentation-citer rebaseline and no source-category growth, independent review finds no forbidden seam or behavior delta, and canonical CI/deploy/authenticated prompt-artifact receipts bind the landed commit.
      assurance:
        independent_verifier: required
        panel: compact
        review_binding: frozen base, exact diff digest, asset byte-identity manifest, commands, ratchet delta, and persistent-path/API assertions
        define_review_result:
          candidate_diff_sha256: 4b0581652ed85606a414c1f3eff998c0706e3a6ef19705dee53a2b2b6ed40053
          reviewers:
            - opencode/hy3-free: PASS
            - google-antigravity/gemini-3.5-flash: PASS
          adjudication: Both reviewers independently confirmed the eight assets, complete concrete symbol and caller graph, path consumers, current-versus-historical citer policy, exact 268-to-270 Define-only citer drift, closure receipts, authority identity, and executable clean-cutover constraints. No blocking finding remained.
          no_rerun_rationale: Appending this review receipt changes only non-authoritative assurance provenance; it does not change the reviewed lock, graph, evidence floor, or stopping condition.
      heresy_delta:
        discovered:
          - This Define authority replaces the prior lock and mechanically raises documentation citers from 268 to 270 before implementation; source counts remain unchanged.
        introduced: []
        repaired:
          - nested runtime ownership of prompt persistence and seeded defaults
  authority_transition:
    transition_id: autoputer-successor-authority-2026-07-13-01
    canonical_ref: refs/heads/main@origin
    rollback_ref: 30ddd8e69c65a3eb9668842e676140a26a84c926
  settled_receipts:
    - id: predecessor-B0-authority
      status: complete
      artifact_ref: 008a7b88cf200119c0f762cc51cfba6be3007445
      evidence_refs: [docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md]
      rollback_refs: [27db14c36c482e321b56a056f6ce5e0accb338a4]
    - id: predecessor-S0-ratchet
      status: complete
      artifact_ref: 2327fcef4716aef070eb4b819296f01b44267364
      evidence_refs: [docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md]
      rollback_refs: [008a7b88cf200119c0f762cc51cfba6be3007445]
    - id: predecessor-S1-deploy
      status: complete
      artifact_ref: 9dff369044c2147140782958de3e91971caed6bc
      evidence_refs: [docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md]
      rollback_refs: [2327fcef4716aef070eb4b819296f01b44267364]
    - id: predecessor-S2-wire
      status: complete
      artifact_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
      evidence_refs: [docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md]
      rollback_refs: [9dff369044c2147140782958de3e91971caed6bc, 481fb8c89a33743021e4fa96568a0936a4f6ba45]
    - id: predecessor-S3-partial
      status: checkpoint_incomplete
      artifact_ref: docs/runtime-dissolution-inventory.yaml
      evidence_refs:
        - docs/evidence/s3-step2-phase-gate-2026-07-13.md
        - docs/evidence/s3-api-route-authority-dispatch-2026-07-13.md
        - docs/evidence/s3-api-handler-ownership-blocker-2026-07-13.md
      rollback_refs: [b7b1262e455a779ca00c8d968ef28b3fa6af9b50]
      imported_effect: Already-landed extraction and ratchet reductions remain canonical; the failed whole-handler candidate does not.
    - id: R1-promptspec-package-cutover-01
      status: complete
      artifact_ref: 642b391a1589196cccf8c35169b1d32b5e791131
      evidence_refs:
        - https://github.com/choir-hip/go-choir/actions/runs/29293772373
        - https://github.com/choir-hip/go-choir/actions/runs/29293772373#job-86964336512
        - staging-submission:c8e6d073-2382-4d01-81a8-3616bcd08de0
        - texture-document:ee5c16e2-be99-49b0-b730-25a06e79d381
        - texture-revision:7b837837-29b8-4a6c-a6ad-491f42a024ae
      rollback_refs: [f4d47c1b5cd412333384de7ef516a7d723c443b3]
      result: CI and all race shards passed; activation receipt bound ordinary guest, sandbox, active computers, and gateway to the implementation commit; an authenticated CLI prompt completed and its fetched Texture artifact contained the exact requested deployed-parser sentence.
      invalidation_triggers: [reproduced prompt parsing or rendering failure, old package path reintroduced, runtime ratchet regression]
    - id: R1-prompt-packages-cutover-02
      status: complete
      artifact_ref: 488664d98b7466f47b7639607ef318b241be44e7
      evidence_refs:
        - https://github.com/choir-hip/go-choir/actions/runs/29295978398
        - https://github.com/choir-hip/go-choir/actions/runs/29295978398#job-86971008673
        - staging-submission:abd95009-6186-40f4-8525-8532959d04fa
        - texture-document:51d1047f-4727-481e-8aa2-3b6019796eab
        - texture-revision:8fb7afe3-2d2b-4286-9cea-52a4dcc34f25
      rollback_refs: [6627cc3294c8e950f5b7c5339b8e0bb056ace3d8]
      result: CI, every selected race shard, differential SBOM, and deploy passed; the activation receipt bound ordinary guest, sandbox, active computers, and gateway to the implementation commit; an authenticated CLI prompt completed and its fetched Texture artifact contained the exact requested relocated-package acceptance sentence.
      invalidation_triggers: [reproduced prompt rendering or embedded asset failure, old package path reintroduced, runtime ratchet regression]
  artifact_identity:
    source: refs/heads/main@origin@1b28520d6a3d31ecf36b2a645623367b4630faa0
    build: https://github.com/choir-hip/go-choir/actions/runs/29295978398
    deploy: https://github.com/choir-hip/go-choir/actions/runs/29295978398#job-86971008673
    staging: submission abd95009-6186-40f4-8525-8532959d04fa completed and Texture revision 8fb7afe3-2d2b-4286-9cea-52a4dcc34f25 fetched after activation
  variant:
    measure: unsatisfied_phase_contracts
    value: 7
    target: 0
  determined_claims:
    - claim: This file is the sole executable product mission.
      source: user-stated
      execution_effect: All registries and resumption point here.
    - claim: Autoputer completion precedes Choir-in-Choir and Autopaper.
      source: user-stated
      execution_effect: Successor work remains closed through R6/R7.
    - claim: Dolt semantic state uses the corpusd world-wire store and one VM-local embedded store per user computer; filesystem/VM snapshots own machine-world state and vmctl alone CAS-writes corpusd route slots.
      source: user-stated
      execution_effect: Preserve both Dolt authorities and forbid a third product-state store.
    - claim: Runtime ownership moves by clean atomic cutover with no aliases or dual authority.
      source: user-stated
      execution_effect: Every R1 implementation is ratchet bounded.
    - claim: The whole-handler transport-first move is compile-falsified; domain-first cohesive extraction is the next execution order.
      source: observed
      execution_effect: R1 begins by selecting one finite typed domain-operation boundary.
    - claim: Define and Implement are the only routine Git durability boundaries.
      source: operational-preference
      execution_effect: Process telemetry remains external to the canonical capsule.
    - claim: Race checks and differential SBOM generation are temporarily paused in canonical CI.
      source: user-stated
      execution_effect: Keep both jobs skipped until the owner directs re-enablement; commit 1b28520d6a3d31ecf36b2a645623367b4630faa0 is the rollback boundary.
  open_findings:
    - S3 whole-handler transport-first cutover is compile-falsified by private domain dependencies.
    - Staging gateway readiness failures caused by local runtime/Dolt/Ollama refusal remain non-attributable to the deployed product until reproduced there.
  belief_changes:
    - The predecessor's transport-before-domain S3 order was not executable at the observed boundary.
    - Repeated orchestration receipts increased context and commit volume without increasing product evidence.
    - Domain-first cohesive extraction followed by thin transport cutover is the current evidence-backed route.
    - The first clean cutover proved that leaf prompt ownership can leave the runtime directory without new indirection, source-category growth, or deployed prompt regression.
    - Two consecutive clean leaf cutovers preserved deployed prompt behavior while reducing runtime from 143 to 137 Go files without increasing any source-debt category.
  highest_impact_remaining_uncertainty: Whether prompt persistence, seeded defaults, runtime construction, and doccheck enforcement can leave the runtime package together without changing bytes, filesystem state, or API behavior.
  next_executable_probe: Freeze and review this code-free R1-prompt-store-package-cutover-03 boundary, commit it, then move the store, direct tests, eight assets, concrete construction edge, path enforcement, and current citer atomically before regenerating the ratchet.
  evidence_index_refs:
    - docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
    - docs/runtime-dissolution-inventory.yaml
    - docs/evidence/s3-api-handler-ownership-blocker-2026-07-13.md
  invalidation_triggers:
    - canonical source diverges from the recorded authority ref
    - any registry exposes another executable product mission
    - new owner direction changes topology or execution order
    - current dependency evidence falsifies the domain-first route
  suggested_goal_string: /goal docs/definitions/choir-autoputer-completion-2026-07-13.md
```

## Resumption

On every invocation:

1. verify this file remains the sole registry entrypoint;
2. resolve `authority_transition.transition_id` from canonical Git history and
   reconcile current source, CI, deployment, and dirty paths;
3. verify settled receipts without rerunning them;
4. invalidate any candidate or review whose base differs from canonical source;
5. resume `active_frontier` and execute the next safe probe;
6. update the capsule only at the next Define or Implement boundary.

Do not reload the predecessor's full 2,000-line transition ledger during normal
resumption. Read a referenced historical range only when a concrete claim needs
forensic support.

## Forbidden Collapses

- Definition exists -> mission topology is implemented.
- predecessor receipt imported -> predecessor orchestration remains active.
- domain-first -> expose private runtime accessors.
- package split -> ownership moved.
- wrapper/facade/alias -> extraction.
- tests use new API -> production uses new API.
- lower ratchet count -> runtime behavior is correct.
- compile passes -> protected behavior is accepted.
- panel majority -> reproduced blocker is cleared.
- worker says done -> candidate is verified.
- local proof -> staging protected-surface proof.
- deployed SHA -> product-path transition worked.
- checkpoint -> completion.
- more documentation commits -> more durability.
- contained key exists -> key cannot escalate.
- external autoputer works -> Choir-in-Choir is safe.
- autoputer complete -> Autopaper is automatically authorized.
