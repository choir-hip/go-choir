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
  updated_at: 2026-07-14T05:24:41Z
  kernel_digest: sha256:cc4c4a96427ea132bb73c79e8a579247fec44dc553c8779245c0096936918e73
  expected_parent_or_authority_ref: refs/heads/main@origin@c70f083e
  status: working
  current_subgoal: R1-toolregistry-facade-extinction-07
  active_phase: R1-runtime-dissolution
  active_frontier:
    - R1-toolregistry-facade-extinction-07
  locks:
    - id: R1-prompt-store-package-cutover-03
      status: complete
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
      rollback_ref: caa714e1f1070a1b12d076210588d547c0bc9315
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
        implementation_review_result:
          candidate_diff_sha256: a6fa98cd5cfc19cfbe6c9702ab46a25033fa3476f0dc988011977f5871e2e572
          reviewers:
            - opencode/hy3-free: PASS
            - google-antigravity/gemini-3.5-flash: PASS
          adjudication: Both bounded reviewers independently confirmed the clean Store/Descriptor/New cutover, eight exact asset renames, complete caller updates, persistent layout and method behavior, dual-root doccheck enforcement, current/historical citer treatment, exact ratchet reductions, and absent forbidden seams. The first panel attempt timed out only after reproducing the same evidence through unnecessary broad commands and raised no finding.
          no_rerun_rationale: Appending this review receipt changes only non-authoritative assurance provenance; it does not change the reviewed source, inventory, lock, graph, evidence floor, or stopping condition.
      local_evidence:
        - class: E0
          observation: canonical implementation parent caa714e1f1070a1b12d076210588d547c0bc9315; every candidate change is within the reviewed lock
        - class: E1
          observation: all eight seeded default YAML assets are detected as 100% byte-identical renames; old store files/default directory and stale source symbol/path searches are empty; runtime ratchet PASS at go_files 135, production_files 71, test_files 64, production_loc 43460, test_loc 49836, exports 958, export_caller_edges 311, initial_unused_export_debt 16, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, citers 269
        - class: E2
          observation: direct promptstore and full doccheck package tests passed
          artifact_ref: artifact://202
        - class: E2
          observation: focused runtime prompt-store, prompt API, system-prompt, override, provider-prompt, and prompt-bar behavior tests passed
          artifact_ref: artifact://198
        - class: E2
          observation: runtime ratchet unit tests passed
          artifact_ref: artifact://204
        - class: E2
          observation: gopls reported no diagnostics in promptstore, runtime construction/API edges, or cmd/doccheck after workspace refresh
      landed_evidence:
        artifact_ref: fb97e4b36ec32df9b6edb6b3eaf69e812e722b4e
        ci:
          run_id: 29298475787
          run_url: https://github.com/choir-hip/go-choir/actions/runs/29298475787
          status: success
          artifact_ref: artifact://226
        deploy:
          job_id: 86977543375
          status: success
          activated_at: 2026-07-14T01:31:55Z
          target_commit: fb97e4b36ec32df9b6edb6b3eaf69e812e722b4e
          active_artifacts: [ordinary_guest, sandbox, active_computers, gateway]
          artifact_ref: artifact://229
        acceptance:
          level: E5
          computer_status: active primary computer, epoch 1871, observed 2026-07-14T01:33:25Z
          submission_id: 98ae4573-13cd-4a42-b14a-ccdecca65d9a
          document_id: 0824ee56-9bfe-4f24-95ec-c119d3cbb989
          revision_id: 8fec021e-4f7f-4464-ba65-6602200740aa
          assertion: authenticated deployed prompt parsing created a Texture artifact whose sole body sentence was "promptstore package cutover accepted after deployed prompt parsing."
          receipt_ref: artifact://240
          artifact_ref: artifact://242
      heresy_delta:
        discovered:
          - This Define authority replaces the prior lock and mechanically raises documentation citers from 268 to 270 before implementation; source counts remain unchanged.
        introduced: []
        repaired:
          - nested runtime ownership of prompt persistence and seeded defaults
    - id: R1-agent-profile-policy-cutover-04
      status: complete
      mutation_class: red
      classification_rationale: Agent-profile normalization, per-profile tool capability grants, and delegation allowlists are authorization authority. This is a source-ownership-only cutover with no intended policy delta, but moving that authority is red and requires protected-surface acceptance.
      conjecture: The two duplicate canonical profile alias tables plus the complete role capability/delegation policy can move atomically from internal/runtime/tool_profiles.go and internal/toolregistry/batch_executor.go into the existing dependency-leaf internal/agentprofile owner, with all 68 normalization callers and every policy caller cut directly to one concrete API and no change to profile aliases, default/unknown handling, batch spawn classification, tool grants, delegation targets, tool registration, runtime identity, provider/model selection, routes, or persisted state.
      conjecture_delta: Source authority coalesces from duplicate runtime and toolregistry normalization tables plus runtime policy into the existing agentprofile dependency leaf; product semantics, batch classification, capability grants, and delegation authority remain unchanged.
      object: canonical agent-profile vocabulary and role capability/delegation policy
      selection_rationale: internal/agentprofile already owns every canonical profile identifier and is imported by both runtime and toolregistry policy consumers. Moving both normalization copies and policy resolution there deletes a pre-existing duplicate table and removes a high-fanout semantic authority from runtime without adding a dependency, interface, callback, wrapper, or new package.
      exact_source_scope:
        - internal/agentprofile/agentprofile.go
        - internal/agentprofile/agentprofile_test.go
        - internal/runtime/tool_profiles.go
        - internal/runtime/prompts.go
        - internal/runtime/api.go
        - internal/runtime/runtime.go
        - internal/runtime/skill_context.go
        - internal/runtime/super_controller.go
        - internal/runtime/texture_handoff.go
        - internal/runtime/tools_coagent.go
        - internal/runtime/tools_email.go
        - internal/runtime/tools_vmctl.go
        - internal/runtime/tools_wire_processor.go
        - internal/toolregistry/batch_executor.go
        - internal/toolregistry/batch_executor_test.go
        - internal/runtime/tools_worker_update.go
        - internal/runtime/trajectory.go
        - internal/runtime/wire_metadata.go
        - internal/runtime/delegate_worker_update_fallback.go
        - internal/runtime/tools_researcher.go
        - internal/runtime/tools_texture.go
        - docs/runtime-dissolution-inventory.yaml
        - docs/definitions/choir-autoputer-completion-2026-07-13.md
      exact_symbols:
        - internal/runtime/tool_profiles.go:type:AgentRoleSpec
        - internal/runtime/tool_profiles.go:func:roleSpec
        - internal/runtime/tool_profiles.go:func:canonicalAgentProfile
        - internal/runtime/tool_profiles.go:func:isTextureProfileValue
        - internal/runtime/tool_profiles.go:func:canDelegateTo
        - internal/toolregistry/batch_executor.go:func:canonicalAgentProfile
        - internal/agentprofile/agentprofile.go:type:Policy
        - internal/agentprofile/agentprofile.go:func:PolicyFor
        - internal/agentprofile/agentprofile.go:func:Canonical
        - internal/agentprofile/agentprofile.go:func:IsTexture
        - internal/agentprofile/agentprofile.go:func:CanDelegate
      caller_graph:
        canonical_profile: "68 production call sites across 14 files: the 62 runtime call sites in api.go, runtime.go, skill_context.go, super_controller.go, texture_handoff.go, tool_profiles.go, tools_coagent.go, tools_email.go, tools_vmctl.go, tools_wire_processor.go, tools_worker_update.go, trajectory.go, and wire_metadata.go, plus six batch_executor.go call sites"
        role_policy: "11 current production resolution call sites across prompts.go and tool_profiles.go; CanDelegate moves one resolution inside the owner, leaving ten direct runtime PolicyFor call sites; four concrete Policy type edges span prompts.go, tool_profiles.go, and tools_coagent.go"
        delegation_policy: one production caller in tools_coagent.go
        texture_profile_predicate: three production callers across delegate_worker_update_fallback.go, tools_researcher.go, and tools_texture.go
      invariants:
        - Preserve the complete profile alias table, trimming, underscore-to-hyphen normalization, lowercase behavior, unknown-profile behavior, and empty-profile behavior byte-for-byte at every caller.
        - Preserve every Policy field and exact grant/deny value for conductor, researcher, Texture, processor, reconciler, Email, co-super, v-super, super, and unknown profiles, including the exact ordered AllowedDelegateTargets slices.
        - Preserve every delegation decision and every tool registry assembled for each profile; do not add, remove, rename, or reorder a registered tool.
        - Cut all callers directly to agentprofile.Policy, PolicyFor, Canonical, IsTexture, and CanDelegate. Delete AgentRoleSpec, roleSpec, both canonicalAgentProfile definitions, isTextureProfileValue, and canDelegateTo; add no alias, forwarding symbol, wrapper, accessor, callback, interface, duplicate table, or compatibility path.
        - Keep runtime run-metadata extraction, Texture actor-ID helpers, prompt assembly, provider/model policy, routes, state authority, and persistent data unchanged.
        - Add direct table-driven owner tests for canonical aliases, unknown/empty normalization, every profile policy, exact delegate targets, allowed and denied delegation, and the Texture predicate; retain focused runtime tool-policy, prompt-policy, coagent delegation, worker-update, Email, vmctl, Texture behavior, and toolregistry batch-spawn classification tests.
        - Regenerate the runtime inventory without weakening debt authority. This Define mechanically raises documentation citers from the landed 269 to exactly 292 before implementation; runtime Go/test/production file counts, test LOC, initial unused-export debt, routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, legacy state writers, and legacy store reads may not increase, while runtime production LOC, exports, and export caller edges must decrease.
      protected_surfaces:
        - per-profile tool capability grants
        - coagent delegation allowlists
        - contained co-super and v-super authority
        - toolregistry batch spawn classification
      admissible_evidence:
        - E0 clean canonical source identity and exact pre-mutation policy manifest
        - E1 complete LSP caller migration, absent old runtime symbols, direct concrete owner API, unchanged tool/route/store ratchets, and decreased runtime production LOC/exports/caller edges
        - E2 direct agentprofile policy tests plus focused runtime prompt policy, registry assembly, coagent authorization, worker update, Email handoff, vmctl delegate, and Texture routing tests, and toolregistry batch-executor tests
        - E5 canonical CI/deploy identity and authenticated staging observations of role policy/tool exposure plus a product-path prompt artifact
        - E6 independent immutable-candidate verification bound to exact diff and pre/post policy manifests
      rollback_ref: fb97e4b36ec32df9b6edb6b3eaf69e812e722b4e
      close_condition: Runtime and toolregistry contain none of the six superseded policy symbols; agentprofile is the sole concrete normalization/policy authority with direct exhaustive tests; every caller and registry uses it directly; policy manifests, batch spawn classification, tool exposure, delegation decisions, routes, persistence, and provider/model behavior are unchanged; the ratchet passes with only the authorized Define citer rebaseline and required runtime reductions; independent review finds no authority delta or seam; and canonical CI/deploy plus authenticated staging policy and prompt-artifact receipts bind the landed commit.
      assurance:
        independent_verifier: required
        panel: compact
        review_binding: frozen base, exact diff digest, complete LSP caller graph, pre/post policy manifests, ratchet delta, focused authorization tests, and staging role-policy/tool observations
        define_review_result:
          candidate_diff_sha256: f44c5f01180cc9f1118226e1f3f8f4645bd668ac0243174a9c9c00abda676d86
          reviewers:
            - opencode/hy3-free: PASS after repair
            - google-antigravity/gemini-3.5-flash: PASS
          adjudication: The first panel exposed the pre-existing duplicate canonicalization table and six omitted toolregistry callers. The repaired lock now deletes both copies, binds all 68 callers across 14 files, includes batch classification tests, states the exact 11-to-10 role-policy transition and four type edges, reconciles the prior 270 Define-only peak with the landed 269 baseline, and leaves discovered heresy unrepaired until implementation. Both reviewers independently recomputed the repaired graph and found no remaining blocker.
          no_rerun_rationale: Appending this review receipt and the mechanically verified 269-to-292 documentation-citer baseline changes only non-authoritative assurance provenance and ratchet data; it does not change the reviewed lock, graph, evidence floor, or stopping condition.
        implementation_review_result:
          candidate_diff_sha256: 4b2bf313db1d0a25b287e833e80e5cccb8e7e3ad8a1ad1cf6c6b454a541b5800
          reviewers:
            - opencode/hy3-free: PASS
            - google-antigravity/gemini-3.5-flash: PASS
          adjudication: Both independent reviewers matched the frozen candidate digest, recomputed sole ownership and behavioral equivalence of aliases, unknown/default handling, all profile grants and delegate ordering, all 68 normalization calls and 11 policy resolutions, the batch classification edge, direct exhaustive tests, authorized ratchet reductions, and absence of wrappers, alternate authority, dependencies, or protected-surface deltas. Neither found a blocker.
          no_rerun_rationale: Appending this immutable-candidate review receipt changes only non-authoritative assurance provenance; it does not change implementation, tests, inventory, reviewed authority, evidence floor, or stopping condition.
      local_evidence:
        - class: E0
          observation: canonical implementation parent da22c4e4; every candidate source and test change is within the reviewed lock
        - class: E1
          observation: both prior canonicalization tables and all five prior runtime policy symbols are absent; 68 normalization calls, 11 policy resolutions, four concrete Policy edges, one delegation decision edge, and three Texture predicate edges bind directly to the single agentprofile owner
        - class: E1
          observation: runtime ratchet passed after authoritative rebaseline at go_files 135, production_files 71, test_files 64, production_loc 43308, test_loc 49836, exports 957, export_caller_edges 308, initial_unused_export_debt 16, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, citers 291; only production LOC, exports, caller edges, and the removed stale package-comment citer decreased
        - class: E2
          observation: exhaustive direct Canonical, PolicyFor, CanDelegate, and IsTexture tests plus all toolregistry batch-executor tests passed
          artifact_ref: artifact://279
        - class: E2
          observation: focused runtime registry exposure, processor/reconciler delegation, Email authority, worker update, internal run profile constraint, prompt policy, and Texture policy behavior tests passed
          artifact_ref: artifact://277
        - class: E2
          observation: runtime compiled with no tests selected, and go vet passed for agentprofile, toolregistry, and runtime
          artifact_ref: artifact://270
        - class: E2
          observation: gopls reported no diagnostics in the new owner, runtime policy edge, or toolregistry batch edge after workspace refresh
        - class: E6
          observation: opencode/hy3-free and google-antigravity/gemini-3.5-flash independently passed immutable candidate 4b2bf313db1d0a25b287e833e80e5cccb8e7e3ad8a1ad1cf6c6b454a541b5800 with no required repair
      validation_notes:
        - The comprehensive-tag runtime test target remains independently non-compilable across pre-existing stale prompt, API, and Texture tests; the attempted prompt-list test never ran, is excluded from evidence, and makes deployed E5 role-policy observation mandatory.
      landed_receipts:
        commit: 0490b4de1f784d5753baa215979ec7a1a076becd
        ci:
          run_id: 29300688070
          status: success
          url: https://github.com/choir-hip/go-choir/actions/runs/29300688070
        deployment:
          job_id: 86984154290
          status: success
          activated_at: 2026-07-14T02:24:11Z
          ordinary_guest: active
          sandbox: active
          active_computers: active
          gateway: active
        staging_computer:
          desktop_id: primary
          epoch: 1872
          runtime_status: ready
          lookup_status: ok
        staging_acceptance:
          submission_id: 91dd8fa2-45d0-4d6f-977d-aa9af5223373
          trajectory_id: 91dd8fa2-45d0-4d6f-977d-aa9af5223373
          doc_id: f5526dd7-c522-4453-88cf-c4e1e71d9582
          revision_id: 6545e764-afe1-46c1-8f57-2f72f043a991
          revision_hash: rev2:22bbfa9047dbbc1547c27df9a9900f20d0ec7ba778ad34845a4f6d579be22eae
          observed_profile: texture
          observed_tool: patch_texture
          observation: The authenticated product path created the exact requested Texture content through patch_texture in 141 ms with no researcher or super request and no pending or consumed worker update.
          evidence_note: The production route intentionally excludes the settings/test prompt-list endpoint, which returned 404; deployed policy evidence is therefore the authenticated Texture role and tool transition plus the exact artifact, bound to the activation receipt and exact local policy matrices.
        acceptance_level: E6
        verifier_contracts:
          - sole agentprofile source authority
          - byte-identical profile normalization and policy matrices
          - unchanged protected role and tool behavior
          - authenticated product-path Texture role and patch_texture transition
        residual_risks:
          - The comprehensive-tag runtime test target remains stale independently of this cutover.
          - The production route has no direct settings endpoint for enumerating all role-policy matrices; exact deny-side coverage remains local and CI-bound while staging proves the positive Texture transition and absence of forbidden delegation on the accepted trajectory.
        next_realism_axis: Move deterministic durable work-item fingerprint authority out of runtime, then stage a real coagent work-item transition.
      completion_adjudication: The reviewed cutover landed at the canonical SHA; CI, deployment, activation identity, computer health, exact profile-policy tests, ratchet reductions, independent verification, authenticated Texture role/tool behavior, and the exact durable artifact all passed. The production settings route is intentionally unavailable, so staging policy evidence binds the positive Texture tool transition and no-delegation result rather than a test-only matrix endpoint; local and CI matrices cover all grant and deny cases.
      heresy_delta:
        discovered:
          - This Define authority mechanically raised documentation citers from 269 to 292 before implementation; all source-category counts remained unchanged. The prior lock's Define-only 268-to-270 rise closed at 269 because implementation redirected one current old-path citer.
          - toolregistry/batch_executor.go contained a second pre-existing canonical profile alias table; a source-authority clean cutover had to delete both copies and move all 68 callers, not only the runtime copy.
          - The comprehensive-tag runtime test target is independently stale across unrelated prompt, API, and Texture tests and cannot currently provide focused prompt-policy evidence.
        introduced: []
        repaired:
          - duplicate canonical profile normalization tables in runtime and toolregistry
          - nested runtime ownership of capability and delegation policy
    - id: R1-work-item-fingerprint-owner-cutover-05
      status: complete
      mutation_class: red
      classification_rationale: Deterministic work-item identities control durable deduplication, obligation reuse, and run-lifecycle behavior. The cutover intends no byte or policy delta, but moving this authority touches lifecycle identity and therefore requires red ceremony.
      conjecture: All five deterministic work-item fingerprint constructors can move atomically from runtime into one dependency-leaf internal/workitem owner, with every production and test caller cut directly to its concrete API and no change to normalized objective bytes, hash bytes, prefix formats, empty-input behavior, persisted fingerprints, work-item deduplication, trajectory obligations, completion, or restart behavior.
      conjecture_delta: Durable work-item identity becomes one explicit domain authority instead of two private runtime helper groups; runtime lifecycle and wire behavior remain unchanged.
      object: deterministic durable work-item fingerprint construction
      selection_rationale: The five pure constructors plus their one objective normalizer are the complete six-symbol WorkItemRecord fingerprint domain in runtime. They share one persisted identity purpose, import only the standard library, have a finite caller graph, and can move without interfaces, callbacks, wrappers, accessors, or store changes. No replacement workitem owner exists to connect. internal/vmctl/ownership.go retains a separate five-field worker-VM ownership fingerprint with desktop-ID normalization; it is not a WorkItemRecord identity, this move replaces rather than adds the runtime normalizer, and coalescing the vmctl function would cross a distinct protected VM lifecycle boundary.
      exact_source_scope:
        - internal/runtime/objective_fingerprint.go
        - internal/runtime/runtime.go
        - internal/runtime/api.go
        - internal/runtime/wire_publication.go
        - internal/runtime/update_coagent_cutover_test.go
        - internal/runtime/trajectory_test.go
        - internal/runtime/wire_processor_decision_test.go
        - internal/runtime/wire_publication_test.go
        - internal/runtime/agent_tools_test.go
        - internal/runtime/api_test.go
        - internal/workitem/fingerprint.go
        - internal/workitem/fingerprint_test.go
        - docs/runtime-dissolution-inventory.yaml
        - docs/definitions/choir-autoputer-completion-2026-07-13.md
      exact_symbols:
        - internal/runtime/objective_fingerprint.go:func:objectiveFingerprint
        - internal/runtime/objective_fingerprint.go:func:normalizeObjectiveText
        - internal/runtime/wire_publication.go:func:wirePublicationWorkItemFingerprint
        - internal/runtime/wire_publication.go:func:wireStoryResolutionWorkItemFingerprint
        - internal/runtime/wire_publication.go:func:wireProcessorDecisionWorkItemFingerprint
        - internal/runtime/wire_publication.go:func:wireProcessorSourceItemDecisionWorkItemFingerprint
        - internal/workitem/fingerprint.go:func:ObjectiveFingerprint
        - internal/workitem/fingerprint.go:func:PublicationFingerprint
        - internal/workitem/fingerprint.go:func:StoryResolutionFingerprint
        - internal/workitem/fingerprint.go:func:ProcessorDecisionFingerprint
        - internal/workitem/fingerprint.go:func:SourceItemDecisionFingerprint
      caller_graph:
        objective: one production caller in runtime.go; add one direct runtime wiring assertion
        publication: one production caller in wire_publication.go
        story_resolution: two production callers in wire_publication.go and three existing test callers across wire_processor_decision_test.go, wire_publication_test.go, and agent_tools_test.go
        processor_decision: six production callers across api.go, runtime.go, and wire_publication.go plus eight existing test callers across trajectory_test.go, wire_processor_decision_test.go, agent_tools_test.go, and api_test.go
        source_item_decision: three production callers in wire_publication.go plus five existing test callers
      invariants:
        - Preserve ObjectiveFingerprint byte-for-byte: trim owner, trajectory, and parent-run IDs; lowercase and tokenize the objective on every non-Unicode-letter or non-Unicode-digit boundary; join the four fields with NUL bytes; return lowercase SHA-256 hex.
        - Preserve the exact wire prefixes and separators for publication, story-resolution, processor-decision, and source-item-decision fingerprints, including the current empty-string return whenever any required component is blank after trimming.
        - Cut every caller directly to workitem.ObjectiveFingerprint, PublicationFingerprint, StoryResolutionFingerprint, ProcessorDecisionFingerprint, or SourceItemDecisionFingerprint. Delete the runtime helper file and all four wire helper functions; add no alias, forwarding symbol, wrapper, facade, accessor, interface, callback, duplicate implementation, or compatibility path.
        - Keep the separate worker-VM ownership fingerprint and normalizer in internal/vmctl/ownership.go unchanged. It has a different five-field schema and desktop-ID authority; this lock moves one existing WorkItemRecord normalizer without creating a third copy or claiming generic objective-text authority.
        - Preserve every already-persisted and newly-created fingerprint byte, store lookup, deduplication decision, work-item detail, assignment, status transition, trajectory obligation, run metadata field, route, provider/model choice, and restart behavior.
        - Add direct table-driven owner tests with hard-coded golden objective hashes, Unicode/punctuation normalization, whitespace and empty cases, every wire prefix, and every missing-component case. Strengthen the spawned-coagent runtime test to assert the exact owner-produced fingerprint.
        - Retain focused spawned-coagent completion/restart, trajectory, processor decision, wire publication, and store fingerprint-deduplication tests.
        - Regenerate the runtime inventory without weakening debt authority. This Define mechanically raises documentation citers from the landed 291 to exactly 307 before implementation while all source-category counts remain unchanged. Runtime production files and LOC must decrease; runtime exports, export caller edges, initial unused-export debt, routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, legacy state writers, and legacy store reads may not increase. Test LOC may rise only for the direct-owner imports and exact wiring assertion required by the locked caller migration; this clarifies the original internally contradictory wording, which required direct migration of six runtime test files while naming only the assertion as authorized test growth, without changing the graph, behavior, or evidence floor.
      protected_surfaces:
        - durable work-item identity and fingerprint deduplication
        - trajectory obligation reuse and settlement inputs
        - spawned-coagent work-item creation and restart recovery
        - wire publication and processor decision obligations
      admissible_evidence:
        - E0 clean canonical source identity and exact pre-mutation fingerprint vectors
        - E1 complete LSP caller migration, absent runtime helpers, one concrete dependency-leaf owner, byte-identical golden vectors, passing ratchet, and reduced runtime production files and LOC
        - E2 direct workitem tests plus focused spawned-coagent completion/restart, trajectory, processor-decision, wire-publication, and store fingerprint-deduplication tests
        - E5 canonical CI/deploy identity and an authenticated staging coagent work-item transition producing an exact Texture artifact
        - E6 independent immutable-candidate verification bound to exact diff, caller graph, golden vectors, ratchet delta, and focused lifecycle tests
      rollback_ref: 0490b4de1f784d5753baa215979ec7a1a076becd
      close_condition: Runtime contains none of the six superseded fingerprint helpers; workitem is the sole concrete constructor owner with direct golden tests; every production and test caller uses it directly; all persisted bytes, deduplication decisions, obligations, statuses, routes, stores, providers, and restart behavior are unchanged; the ratchet passes with only authorized documentation/test growth and required runtime reductions; independent review finds no authority delta or seam; and canonical CI/deploy plus an authenticated staging coagent work-item and exact artifact bind the landed commit.
      assurance:
        independent_verifier: required
        panel: compact
        review_binding: frozen base, exact diff digest, complete LSP caller graph, pre/post golden vectors, ratchet delta, focused lifecycle tests, and staging work-item transition
        define_review_result:
          candidate_diff_sha256: a4e28cd37a20631167c00945225dd786451dbca03a2b54f05cca981a5b0c389d
          reviewers:
            - opencode/hy3-free: PASS after repair
            - google-antigravity/gemini-3.5-flash: PASS after repair
          adjudication: The first panel found two omitted comprehensive test files, three omitted callers, an implicit five-constructor versus six-symbol distinction, and an unaddressed separate worker-VM normalizer. The repaired lock includes both files, binds three story-resolution and eight processor-decision test callers, names five constructors plus one normalizer, keeps the distinct five-field worker-VM identity outside this WorkItemRecord boundary, and records the measured 291-to-307 documentation-only citer rise. Both reviewers independently recomputed the repair and found no remaining blocker.
          no_rerun_rationale: Appending this review receipt changes only non-authoritative assurance provenance; it does not change the reviewed lock, caller graph, evidence floor, or stopping condition.
        implementation_review_result:
          candidate_diff_sha256: d359a04ded5621087a57d36a0bbb7be93180204d29b6f01e2a61a7b173e59cea
          reviewers:
            - google-antigravity/gemini-3.5-flash: PASS
            - opencode/hy3-free: PASS after bounded rereview
          adjudication: Both independent reviewers matched the frozen 14-path candidate digest, recomputed deletion of all six old symbols, sole dependency-leaf ownership, objective and wire byte equivalence, direct migration of production and comprehensive-tag callers, unchanged separate worker-VM identity, complete golden and spawned wiring tests, authorized ratchet reductions, honest test growth, and no wrapper, duplicate authority, persisted-lifecycle delta, or evidence inflation. The bounded rereview reported a cosmetic eight-versus-nine processor-test-caller concern; direct enumeration proves the reviewed count of eight across one trajectory caller, five processor-decision callers, one agent-tools caller, and one API caller. The initial opencode attempt timed out and produced no verdict, so it is excluded rather than counted.
          no_rerun_rationale: Appending this immutable-candidate review receipt changes only non-authoritative assurance provenance; it does not change implementation, tests, inventory, reviewed authority, evidence floor, or stopping condition.
      local_evidence:
        - class: E0
          observation: canonical implementation parent 2e3286e2; every candidate source and test change is within the reviewed 14-path lock
        - class: E1
          observation: all six superseded runtime fingerprint symbols are absent; five exported constructors plus one private normalizer live in the dependency-leaf workitem owner, and every normal-build production and test caller resolves directly to it
        - class: E1
          observation: runtime ratchet passed after authoritative rebaseline at go_files 134, production_files 70, test_files 64, production_loc 43230, test_loc 49847, exports 957, export_caller_edges 308, initial_unused_export_debt 16, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, legacy_state_writers 0, legacy_store_reads 0, citers 307; runtime production lost one file and 78 LOC, test LOC rose only for six required owner imports, import formatting, and the four-line exact spawned fingerprint assertion, and all other ratchet categories remained flat
        - class: E2
          observation: direct workitem golden-vector, Unicode/punctuation normalization, whitespace, empty-input, every-prefix, and every-missing-component tests passed
          artifact_ref: artifact://351
        - class: E2
          observation: focused spawned-coagent completion and process-restart recovery, trajectory, processor-decision, wire-publication, and store fingerprint-deduplication tests passed
          artifact_refs: [artifact://354, artifact://356]
        - class: E2
          observation: runtime compiled with no tests selected, and go vet passed for workitem and runtime
          artifact_refs: [artifact://358]
        - class: E2
          observation: gopls reported no diagnostics in the new workitem owner or normal-build runtime caller files after workspace refresh; LSP resolved the normal-build caller graph directly to the owner, while the two comprehensive-tag files remain outside gopls package metadata and were migrated explicitly within the reviewed scope
        - class: E6
          observation: google-antigravity/gemini-3.5-flash and opencode/hy3-free independently passed immutable candidate d359a04ded5621087a57d36a0bbb7be93180204d29b6f01e2a61a7b173e59cea with no required repair; direct caller enumeration adjudicated one non-blocking reviewer miscount
      validation_notes:
        - The comprehensive-tag runtime target remains independently stale across unrelated prompt, API, and Texture tests. Those two locked comprehensive caller files were formatted and migrated directly, but are excluded from local compilation evidence; canonical CI and E5 remain mandatory.
      landed_receipts:
        commit_sha: 4a1bbdd1a43b0d0cbda6b5ef03950aa48785a97
        ci:
          run_id: 29303459550
          status: success
          url: https://github.com/choir-hip/go-choir/actions/runs/29303459550
        deployment:
          job_id: 86992455034
          status: success
          staging_build_sha: 4a1bbdd1a43b0d0cbda6b5ef03950aa48785a97
          computer_status: ready
        staging_acceptance:
          trajectory_id: 5053c721-50ad-4238-9608-7ba694f881c5
          conductor_loop_id: f42008db-e1d5-4118-a013-36e76a5e98d6
          researcher_loop_id: b5c2f6c2-9aea-435b-b2fd-d6c12973c114
          work_item_id: 09907fb1-715d-4b40-8c4a-57b4fddf1789
          objective_fingerprint: spawned_coagent:e11286c0a0ef435fc777bd4f368896fcaf9ed3bceb04614fdd756c485fe86e93
          texture_doc_id: 846abd91-a34d-4586-80f7-542d8916dfdd
          initial_revision_id: e98c1652-a751-4798-bf06-fcbcb921b810
          accepted_revision_id: d6beaad9-bdbd-4a01-8001-63858a7aaaf5
          accepted_revision_hash: rev2:81aa3c2d359afd22fb891415e45841e05bf2e5e5f4903a213dc67c8400d92f10
          observation: The authenticated staging conductor created exactly one researcher work item with the independently recomputed owner fingerprint, waited for the exact twenty-token result, consumed the worker update, reached zero open work items and zero pending updates, and replaced the Texture document with the exact acceptance sentence.
      completion_adjudication: Complete at E6. Canonical CI and deploy succeeded for the landed commit; staging health and the current computer reported that exact build; the authenticated product path exercised conductor-to-researcher creation, durable work-item identity, completion, update consumption, settlement, and exact Texture revision publication; the independent immutable-candidate panel had already verified the exact caller graph, golden vectors, lifecycle tests, and ratchet delta. No protected-surface failure, authority seam, or residual work-item obligation remained.
      heresy_delta:
        discovered:
          - deterministic durable work-item identity is split between a standalone runtime objective helper file and four private wire-publication helpers
        introduced: []
        repaired:
          - deterministic durable work-item identity split between a standalone runtime objective helper file and four private wire-publication helpers
    - id: R1-search-gateway-owner-cutover-06
      status: complete
      mutation_class: red
      classification_rationale: Web-search provider routing is protected. The cutover changes only Go source ownership, but it moves the gateway transport contract and deletes an unused direct-provider implementation, so exact routing, authentication, outage semantics, tool exposure, and deployed provider behavior require red ceremony.
      conjecture: The gateway-backed web-search client, response contract, and structured outage semantics can move atomically from internal/runtime into the existing dependency-leaf internal/search owner while its unused direct-provider implementation is deleted, with no request, response, provider-routing, tool-policy, evidence, or agent-visible behavior delta.
      conjecture_delta: Gateway search transport becomes one explicit dependency-leaf authority instead of a private runtime client beside an unwired direct-provider package. The gateway remains the sole provider selector and health authority; no provider, model, cadence, projection, or routing policy changes.
      object: gateway-backed research search client and response-contract ownership
      selection_rationale: This is the smallest cohesive production-used research boundary. internal/runtime/search_gateway.go is already classified for deletion, and its only production constructor caller and response consumers are the research tool registry and projection. The existing internal/search package has no repository importer; connecting its direct Tavily/Brave/Exa/Serper implementation would bypass the canonical gateway and violate sole provider-routing authority. Deleting that superseded implementation and reusing its package path is deletion-first, avoids a third search package, and removes one runtime production file and one runtime test file.
      exact_source_scope:
        - internal/search/search.go
        - internal/search/search_test.go
        - internal/runtime/search_gateway.go
        - internal/runtime/search_gateway_test.go
        - internal/runtime/tool_profiles.go
        - internal/runtime/tools_research.go
        - internal/runtime/tools_test.go
        - docs/runtime-dissolution-inventory.yaml
        - docs/definitions/choir-autoputer-completion-2026-07-13.md
      owner_contract:
        package: internal/search
        exports:
          - Client
          - Response
          - NewGatewayClientFromEnv
        private_implementation:
          - gatewaySearchClient
          - gatewaySearchAttempt
          - gatewaySearchOutageBody
          - gatewayProviderHealth
          - parseGatewaySearchOutage
          - firstNonEmptyString
        behavior: Client.Search POSTs the same query and optional positive max_results JSON to /provider/v1/search, sends the same bearer token and content type, preserves the same 30-second default timeout, returns the same successful response fields, projects search_outage into the same structured non-error response, and preserves the same malformed, transport, status, and gateway-error failures.
        dependency_note: firstNonEmptyString is redefined privately in internal/search/search.go with the same trim-first behavior because the runtime helper lives in tools_vmctl.go outside this boundary; internal/search must not import runtime.
      runtime_deletions:
        - internal/runtime/search_gateway.go
        - internal/runtime/search_gateway_test.go
        - internal/runtime.webSearchClient
        - internal/runtime.webSearchResponse
        - internal/runtime.gatewaySearchClient
        - internal/runtime.gatewaySearchAttempt
        - internal/runtime.gatewaySearchOutageBody
        - internal/runtime.gatewayProviderHealth
        - internal/runtime.newGatewaySearchClientFromEnv
        - internal/runtime.parseGatewaySearchOutage
      superseded_search_deletions:
        - SearchResult
        - SearchResponse
        - SearchRequest
        - SearchProvider
        - SearchClient
        - NewSearchClient
        - SearchClient.Search
        - SearchClient.AvailableProviders
        - TavilyProvider and parseTavilyResults
        - BraveProvider and parseBraveResults
        - ExaProvider and parseExaResults
        - SerperProvider and parseSerperResults
        - truncateError
      exact_caller_graph:
        constructor:
          production:
            - internal/runtime/tool_profiles.go:Runtime.InstallDefaultAgentTools
          tests:
            - internal/search/search_test.go
        client_contract:
          production:
            - internal/runtime/tool_profiles.go:Runtime.buildRegistryForRole
            - internal/runtime/tools_research.go:RegisterResearchTools
            - internal/runtime/tools_research.go:newWebSearchTool
        response_contract:
          production:
            - internal/runtime/tools_research.go:newWebSearchTool
            - internal/runtime/tools_research.go:compactWebSearchProjection
          tests:
            - internal/runtime/tools_test.go:TestCompactWebSearchProjectionGuidesResearchUpdateCheckpoint
            - internal/runtime/tools_test.go:TestCompactWebSearchProjectionSurfacesGatewayOutage
        legacy_direct_provider_importers: []
      invariants:
        - RUNTIME_GATEWAY_URL remains first-choice base URL and PROXY_VMCTL_URL remains the fallback; missing base URL or RUNTIME_GATEWAY_TOKEN yields a nil client and unavailable web_search rather than a typed-nil interface.
        - The request method, /provider/v1/search suffix, query bytes, positive max_results omission rule, Authorization bearer value, Content-Type, timeout, unbounded response-body read behavior, and error prefixes remain byte-for-byte unchanged.
        - HTTP 200 success preserves query, provider, providers, attempts, results, merged_count, waves, degraded, provider_health, outage, code, and error JSON semantics.
        - A body whose code or error is search_outage remains a successful structured Response with empty non-nil results, fallback query behavior, attempts, provider health, outage, degraded, code, and error fields preserved exactly.
        - Non-outage non-200 responses, malformed JSON, request creation, transport, and body-read failures retain their existing observable error classes and text.
        - The gateway remains the sole provider selection, cooldown, health, merge, and credential authority. No runtime or internal/search code may call Tavily, Brave, Exa, or Serper directly after the cutover.
        - web_search registration, researcher authorization, minimum forty-result request floor, fifty-result schema ceiling, compact projection, Trace full-output evidence, checkpoint cadence, source_search, and all other research tools remain unchanged.
        - No runtime compatibility alias, facade, wrapper, duplicate response type, legacy constructor, direct-provider fallback, or import cycle is retained.
        - All production and test callers import the concrete internal/search owner directly; internal/search imports no internal/runtime package.
        - Persisted state, routes, stores, lifecycle, work items, Texture, Trace, promotion, vmctl, provider/model policy, and agent-profile policy are unchanged.
      exact_tests:
        - internal/search constructor environment precedence, missing configuration, successful request method/path/headers/body and complete response decode
        - internal/search structured outage projection with fallback query, attempts, provider health, empty results, degraded flag, code, and error
        - internal/search non-outage gateway error, generic status, malformed success response, and transport failure behavior
        - existing runtime compact web-search success/checkpoint and structured-outage projections
        - focused runtime tool-profile registry and web_search execution behavior
      forbidden_targets:
        - gateway route, authentication, provider selection, cooldown, health, merge, credential, or response schema changes
        - direct provider calls or provider API-key reads outside the gateway
        - web_search name, description, parameters, result floor, result ceiling, projection, Trace evidence, checkpoint cadence, or role exposure changes
        - source_search, source-service, content import/fetch, route, store, lifecycle, work-item, Texture, promotion, vmctl, or model-policy behavior
        - edits outside exact_source_scope
        - compatibility aliases, re-exports, facades, duplicate clients, or dual routing
      protected_surfaces:
        - gateway provider routing and authentication
        - web_search agent tool behavior and Trace evidence
        - role-based tool exposure
      admissible_evidence:
        - E0 frozen base, exact diff, complete LSP caller graph, and proof that internal/search has no importer before cutover
        - E1 runtime extinction of the private gateway client/response types and all direct provider symbols, one dependency-leaf owner, no internal/search-to-runtime import, and ratchet reduction
        - E2 direct owner tests plus focused runtime projection, registry, and web_search behavior tests
        - E5 canonical CI/deploy identity plus an authenticated staging coagent web_search transition whose Trace evidence proves gateway-backed results or an honest structured outage
        - E6 independent immutable-candidate verification bound to exact diff, caller graph, request/response golden behavior, ratchet delta, and focused tests
      rollback_ref: 4a1bbdd1a43b0d0cbda6b5ef03950aa48785a97
      close_condition: Runtime contains none of its private gateway-search client, response, or outage types and no search_gateway files; internal/search contains only the gateway-backed client contract and direct tests, with every legacy direct-provider symbol absent; all callers use the owner directly; request, response, outage, error, tool-policy, provider-routing, role exposure, and evidence behavior are unchanged; the ratchet passes with runtime file and LOC reductions, flat unrelated interface candidates, and only authorized documentation changes; independent review finds no authority delta or seam; and canonical CI/deploy plus an authenticated staging coagent web_search transition bind the landed commit.
      assurance:
        independent_verifier: required
        panel: compact
        review_binding: frozen base, exact diff digest, complete LSP caller graph, pre/post request-response vectors, ratchet delta, focused tests, and staging coagent web_search transition
        define_review_result:
          initial_candidate_diff_sha256: 84335ccf2520bea251cc44d911d8877a1cce5c58549b4f0437dfb444312833f6
          repaired_candidate_diff_sha256: a87f3c896da1a08f2ab45a35dd329c3f57fd7ead8f639dd9d3dd6d32c67b96ba
          reviewers:
            - google-antigravity/gemini-3.5-flash: PASS
            - opencode/hy3-free: PASS after bounded repair
          adjudication: Both reviewers verified the zero-importer legacy search package, complete gateway constructor/client/response caller graph, request/response/outage semantics, red evidence floor, and Define-only citer rise. One reviewer required the lock to preserve the actual gatewaySearchClient and firstNonEmptyString names, state that the dependency-local helper replaces an out-of-scope runtime helper without importing runtime, and distinguish the method's delete disposition from the containing file's research domain. The repaired candidate passed bounded rereview with no remaining semantic blocker; the repaired wording mechanically changed the exact authorized documentation-only endpoint from 334 to 333, which was recomputed after appending this receipt.
          count_review:
            candidate_diff_sha256: 3c0c6fa7a88fbc96eec597f286b095cd1370da2b7919896a1514c40a7315777c
            reviewer: opencode/hy3-free
            result: PASS
            observation: Independent rerun of the ratchet confirmed citers 307 to 333 with every source-category count unchanged and the semantic repairs intact.
          ratchet_correction_review:
            candidate_diff_sha256: 593969b367c91068d349a7d89530dbb84b4009e7194ddf5485b97376ac3c9f2b
            reviewer: opencode/hy3-free
            result: PASS
            observation: Exact parent 029e3dd859ac9fd7122ca0f0f2c52df03fdda5d0 and correction digest were independently verified; the four candidates are unrelated runtime-to-store boundaries, flat-at-four preserves debt authority, and required runtime file and LOC reductions remain binding.
            yaml_repair_review:
              candidate_diff_sha256: 1e3c9a0ff8ba762c2ca27126d51aeac43babead71e37e2a843c2aae786f1c6d8
              reviewer: opencode/hy3-free
              result: PASS
              observation: Independent parsing confirmed the quoted problem note is a scalar, the exact parent and correction semantics are unchanged, and the prior correction receipt remains present.
          no_rerun_rationale: Appending this review receipt changes only non-authoritative assurance provenance; it does not change the reviewed lock, caller graph, behavior invariants, evidence floor, or stopping condition.
        implementation_review_result:
          candidate_diff_sha256: ca179bcdae8ff4498dc2732a260e0dce45fe1ef9e3d5c54884a932a9adc77b74
          reviewers:
            - google-antigravity/gemini-3.5-flash: PASS
            - opencode/hy3-free: PASS
          adjudication: Both independent reviewers matched the frozen nine-path candidate digest and exact authority parent; verified deletion of the unused direct-provider implementation, both runtime gateway files, private runtime client/response contracts, and direct provider credentials; recomputed the complete constructor, Client, and Response caller graph; matched request, response, outage, trimming, error, tool-policy, Trace, and role behavior; validated focused tests and the exact ratchet reductions with four unrelated interface candidates flat; parsed the unique-key capsule; and found no wrapper, alias, fallback, dual route, authority seam, or unclaimed E5 evidence.
          no_rerun_rationale: Appending this immutable-candidate review receipt changes only non-authoritative assurance provenance; it does not change implementation, tests, inventory, reviewed authority, evidence floor, or stopping condition.
        staging_review_result:
          reviewers:
            - opencode/hy3-free: PASS
            - google-antigravity/gemini-3.5-flash: PASS
          adjudication: Both independent verifiers read the lock literally and accepted the durable authenticated researcher result as the product-path Trace artifact. Diagnostic trajectory 4ba004d6-ac56-4a2a-9c49-284c15376b82 and researcher run 6eeedde6-7e44-40c0-91d5-55c7c2f491c4 returned the exact gateway-aggregated seven-provider health map and empty attempts after one web_search; that content proves the deployed gateway structured outage reached the researcher projection. They found that demanding the raw internal event endpoint would contradict the approved public-edge isolation and no-SSH product path, and required no repair.
          no_rerun_rationale: Appending this product-path review receipt changes only non-authoritative assurance provenance; it does not change the landed implementation, provider state, lock semantics, or exact acceptance identities.
      local_evidence:
        - class: E0
          observation: repository import search finds no caller of internal/search, while LSP resolves the deployed gateway constructor only from runtime tool-profile installation and the runtime response contract only through research registration, execution, projection, and two projection tests
        - class: E0
          observation: the inventory classifies gatewaySearchClient.Search as delete and the containing search_gateway.go file under the research domain; R1 requires the file itself to leave runtime. The existing internal/search direct-provider implementation is unwired and would violate the canonical gateway provider-routing boundary if connected.
        - class: E1
          observation: both runtime search_gateway files, the private runtime client and response contracts, and every legacy direct-provider symbol are absent; the existing search package is now the sole gateway-backed owner with three exports, no runtime import, and direct production wiring
        - class: E1
          observation: runtime ratchet PASS at go_files 132, production_files 69, test_files 63, production_loc 43047, test_loc 49769, exports 955, export_caller_edges 308, initial_unused_export_debt 15, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, legacy_state_writers 0, legacy_store_reads 0, citers 333; relative to the reviewed implementation authority, runtime lost two files, one production file, one test file, 183 production LOC, 78 test LOC, two exports, and one unit of unused-export debt while all other source categories remained flat
        - class: E2
          observation: direct gateway owner tests cover missing configuration, URL precedence, nil-interface behavior, timeout, request method/path/headers/body, positive result limits and non-positive omission, every response field, structured outage fallback and optional fields, gateway/status/decode/request/transport/read errors, and pass under the race detector
          artifact_refs: [artifact://483, artifact://485]
        - class: E2
          observation: focused runtime profile installation, gateway route, proxy fallback, unavailable-client, compact success/checkpoint, and structured-outage projection tests passed
          artifact_ref: artifact://483
        - class: E2
          observation: search owner and runtime compiled with no tests selected; go vet passed both packages; runtime-ratchet unit tests and the authoritative ratchet passed
          artifact_refs: [artifact://487, artifact://490]
        - class: E2
          observation: gopls reported no diagnostics in the owner, research tool, or profile registry wiring; LSP resolved the constructor to one production caller plus direct owner tests, Client to three runtime boundaries, and Response to runtime projection plus two projection tests
        - class: E5
          observation: >-
            Canonical CI run 29306556937 and deploy job 87001461766 succeeded for landed commit 59f514efae75bd00a07743c4944a7018d23a49d8; the activation receipt bound both sandbox and gateway to that commit. Authenticated diagnostic trajectory 4ba004d6-ac56-4a2a-9c49-284c15376b82 created researcher run 6eeedde6-7e44-40c0-91d5-55c7c2f491c4, whose single web_search durably returned exact JSON with attempts empty and gateway provider_health for all seven configured providers: Brave cooling down after HTTP 422 because count exceeded twenty; Exa, Parallel, Serper, and Tavily quota-limited; SerpAPI rate-limited with no searches remaining; and SearXNG cooling down after repeated empty success. The expected public-edge HTTP 403 on the internal events route preserved product isolation. Two independent verifiers accepted this durable authenticated result as the lock's product-path Trace artifact proving an honest structured gateway outage.
      validation_notes:
        - Runtime baseline before this Define is go_files 134, production_files 70, test_files 64, production_loc 43230, test_loc 49847, exports 957, export_caller_edges 308, initial_unused_export_debt 16, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, legacy_state_writers 0, legacy_store_reads 0, citers 307.
        - This Define mechanically raises documentation citers from 307 to exactly 333 before implementation; all source-category counts remain unchanged. The implementation must rebaseline that authorized documentation-only rise, then reduce runtime production files, test files, production LOC, and test LOC without increasing any other source category; interface candidates must remain flat at four.
        - 'Problem documented before ratchet correction: the first implementation measurement showed interface_candidates remained four, not lower. That category enumerates four pre-existing runtime-to-store interface call boundaries and does not count the private webSearchClient declaration being moved. Requiring a decrease would force unrelated scope or a false reclassification. Correcting the lock to require the category remain flat preserves debt authority and changes no source behavior.'
        - Local and ratchet proof covers exact transport, response, outage, projection, role wiring, and source authority. Provider routing and agent-visible search remain protected and require canonical CI/deploy plus staging product-path proof before completion.
        - 'Problem documented before any further probe or fix: the first authenticated deployed coagent web_search after commit 59f514efae75bd00a07743c4944a7018d23a49d8 returned structured search_outage and no results. This is a search-provider/gateway substrate failure observation, not evidence that the ownership cutover caused it. An honest structured outage is admissible E5 only when Trace proves it; the durable researcher result alone is indirect, and the public edge correctly forbids the internal events route. Do not label the lock complete until an admissible product-path artifact independently proves the structured gateway response or a later deployed coagent search returns gateway-backed results.'
      completion_adjudication: Complete at E6. Canonical CI and deploy bound sandbox and gateway to the landed implementation; the authenticated staging coagent transition exercised the relocated client and returned the exact structured outage projection with gateway-only aggregate health for all seven providers; two independent verifiers accepted the durable run result as admissible product-path Trace and required no raw internal-route bypass. The observed provider cooldown and quota exhaustion predate and remain outside this ownership-only cutover; no provider policy, credential, route, model, cadence, or fallback was changed. The exact caller graph, golden transport/response vectors, ratchet delta, immutable-candidate review, and staging evidence leave no authority seam or residual obligation in this lock.
      heresy_delta:
        discovered:
          - gateway-backed search transport and response authority are nested in runtime beside an unwired internal/search package that directly selects providers and reads provider credentials
          - the configured forty-result web_search floor exceeds Brave's maximum count of twenty, causing HTTP 422 and provider cooldown while the other configured providers are quota-limited, rate-limited, or repeatedly empty
        introduced: []
        repaired:
          - gateway-backed search transport and response authority nested in runtime beside an unwired direct-provider search package
    - id: R1-toolregistry-facade-extinction-07
      status: defined
      mutation_class: orange
      classification_rationale: Tool construction, schema rendering, system-prompt catalogs, projection envelopes, and result JSON are direct runtime behavior. The cutover removes only runtime aliases and forwarding helpers while preserving exact bytes and tool contracts; it does not change provider routing, tool authorization, tool implementations, Trace persistence, state authority, or any red protected mutation.
      conjecture: The runtime Tool alias and five toolregistry forwarding/encoding helpers can be deleted atomically by extending the existing internal/toolregistry owner with the two missing pure encoders and prompt-catalog composition, then migrating every caller directly, without changing any tool definition, schema, catalog bytes, result bytes, projection envelope, execution behavior, role exposure, or Trace evidence.
      conjecture_delta: Tool types, schemas, prompt catalogs, and result envelopes become direct toolregistry authority rather than runtime aliases and facades. Runtime retains domain-specific tool implementations only; no registry, schema, encoding, or prompt-composition policy remains duplicated there.
      object: runtime toolregistry alias and facade extinction
      selection_rationale: This is the smallest cohesive cross-cutting boundary already owned elsewhere. internal/runtime/tools.go contains one type alias and four forwarding helpers, while toolResultJSON is a fifth pure encoder in tool_profiles.go; all delegate to or semantically belong to internal/toolregistry. Direct caller migration deletes the wrapper file, removes an unrelated encoder from profile installation, shrinks duplicate registry tests, and reduces wrapper debt without introducing an interface, accessor, callback, compatibility name, or new package.
      exact_source_scope:
        - internal/toolregistry/toolregistry.go
        - internal/toolregistry/toolregistry_test.go
        - internal/toolregistry/toolloop.go
        - internal/runtime/tools.go
        - internal/runtime/tool_profiles.go
        - internal/runtime/runtime.go
        - internal/runtime/prompts.go
        - internal/runtime/tools_capsule.go
        - internal/runtime/tools_coagent.go
        - internal/runtime/tools_coding.go
        - internal/runtime/tools_email.go
        - internal/runtime/tools_evidence.go
        - internal/runtime/tools_model_verify.go
        - internal/runtime/tools_research.go
        - internal/runtime/tools_shipper.go
        - internal/runtime/tools_texture.go
        - internal/runtime/tools_vmctl.go
        - internal/runtime/tools_wire_processor.go
        - internal/runtime/tools_worker_update.go
        - internal/runtime/tools_test.go
        - internal/runtime/run_memory_integration_test.go
        - docs/runtime-dissolution-inventory.yaml
        - docs/definitions/choir-autoputer-completion-2026-07-13.md
      owner_contract:
        existing:
          - toolregistry.Tool
          - toolregistry.JSONSchemaObject
          - toolregistry.CloneSchemaMap
          - toolregistry.ToolRegistry.Catalog
        add:
          - toolregistry.BuildSystemPrompt
          - toolregistry.ResultJSON
          - toolregistry.ProjectionResultJSON
        behavior:
          - BuildSystemPrompt returns the base prompt byte-for-byte when the registry is nil or empty; otherwise it returns base plus two newlines plus the existing deterministic registry catalog.
          - ResultJSON applies encoding/json Marshal once to the supplied value and returns the identical compact JSON string or marshal error previously returned by runtime toolResultJSON.
          - ProjectionResultJSON normalizes nil metadata to an empty object and calls ResultJSON on the identical envelope keys and values: __choir_tool_projection true, model_output, durable_output, and projection.
      owner_duplicate_deletions:
        - internal/toolregistry/toolloop.go:buildSystemPromptWithTools
      runtime_deletions:
        - internal/runtime/tools.go
        - internal/runtime.Tool alias
        - internal/runtime.jsonSchemaObject
        - internal/runtime.cloneSchemaMap
        - internal/runtime.buildSystemPromptWithTools
        - internal/runtime.toolProjectionResultJSON
        - internal/runtime.toolResultJSON
        - duplicate runtime registry/schema/prompt/projection tests whose authority moves to internal/toolregistry
      exact_caller_graph:
        type_and_schema:
          production:
            - internal/runtime/tools_capsule.go
            - internal/runtime/tools_coagent.go
            - internal/runtime/tools_coding.go
            - internal/runtime/tools_email.go
            - internal/runtime/tools_evidence.go
            - internal/runtime/tools_model_verify.go
            - internal/runtime/tools_research.go
            - internal/runtime/tools_shipper.go
            - internal/runtime/tools_texture.go
            - internal/runtime/tools_vmctl.go
            - internal/runtime/tools_wire_processor.go
            - internal/runtime/tools_worker_update.go
          tests:
            - internal/runtime/tools_test.go
            - internal/runtime/run_memory_integration_test.go
        prompt_catalog:
          production:
            - internal/runtime/runtime.go
            - internal/runtime/prompts.go
            - internal/toolregistry/toolloop.go
          tests:
            - internal/runtime/tools_test.go
        result_json:
          production:
            - internal/runtime/tools_capsule.go
            - internal/runtime/tools_coagent.go
            - internal/runtime/tools_coding.go
            - internal/runtime/tools_email.go
            - internal/runtime/tools_evidence.go
            - internal/runtime/tools_model_verify.go
            - internal/runtime/tools_research.go
            - internal/runtime/tools_shipper.go
            - internal/runtime/tools_texture.go
            - internal/runtime/tools_vmctl.go
            - internal/runtime/tools_wire_processor.go
            - internal/runtime/tools_worker_update.go
          tests:
            - internal/runtime/tools_test.go
        projection_json:
          production:
            - internal/runtime/tools_research.go
          tests:
            - internal/runtime/tools_test.go
      invariants:
        - Every runtime tool constructor and collection uses toolregistry.Tool directly; the runtime package declares no Tool alias.
        - Every schema caller uses toolregistry.JSONSchemaObject directly and prompt descriptor cloning uses toolregistry.CloneSchemaMap directly.
        - Tool names, descriptions, JSON Schemas, required arrays, additionalProperties flags, functions, registration order, sorted catalog order, eighty-byte description truncation, default empty schemas, and role/profile exposure are byte-for-byte unchanged.
        - BuildSystemPrompt preserves nil, empty, and populated registry output exactly, including the two-newline separator and catalog trailing newline.
        - ResultJSON preserves encoding/json compact output, HTML escaping, map handling, nil behavior, and errors exactly; no custom encoder, indentation, newline, buffering, or alternate serialization is introduced.
        - ProjectionResultJSON preserves the exact four-key envelope, nil metadata normalization, model-versus-durable split, and projection metadata used by Trace and research checkpoints.
        - Tool execution, parallelism, event kinds and payloads, durable full output, model projection, errors, checkpoints, authorization, provider calls, state mutation, and Trace persistence are unchanged.
        - Runtime retains no alias, wrapper, forwarding function, deprecated name, fallback, or duplicate implementation for the moved authority.
      exact_tests:
        - owner tests for Tool validation, registry duplicate/default/sorted behavior, schema creation and deep cloning
        - owner golden tests for nil/empty/populated BuildSystemPrompt bytes
        - owner golden tests for ResultJSON compact bytes, HTML escaping, nil values, and unsupported-value errors
        - owner golden tests for ProjectionResultJSON normal and nil-metadata envelope bytes plus unsupported-value errors
        - focused runtime tool profile, prompt catalog, representative schema, result, projection, research outage, work-item update, and run-memory tool-loop behavior
      forbidden_targets:
        - any tool name, description, parameter schema, handler, registration order, result field, projection field, prompt text, catalog format, role exposure, or execution semantics
        - provider/model/search routing, gateway behavior, Trace storage, event contracts, work-item identity, Texture, stores, routes, promotion, vmctl behavior, or persisted state
        - changes outside exact_source_scope
        - compatibility aliases, deprecated wrappers, facades, callback seams, interfaces, re-exports, or dual encoders
      protected_surfaces:
        - none mutated; Trace and tool authorization are observed invariants
      admissible_evidence:
        - E0 frozen parent, exact diff digest, complete LSP caller graph for all six runtime symbols, and pre-change golden vectors
        - E1 runtime alias/wrapper extinction, direct owner imports, no duplicate encoder or prompt composer, runtime file/LOC and wrapper reductions, and no unrelated ratchet increase
        - E2 owner golden tests plus focused runtime registry, schema, prompt, projection, research, update, and tool-loop tests
        - E5 canonical CI/deploy identity plus an authenticated staging coagent transition whose durable tool result and downstream update/Texture artifact prove unchanged schema, execution, JSON result, projection, and prompt-catalog behavior
        - E6 independent immutable-candidate verification bound to exact diff, caller graph, golden bytes, ratchet delta, focused tests, and staging transition
      rollback_ref: d6014fa7
      close_condition: Runtime tools.go is absent; runtime declares none of Tool, jsonSchemaObject, cloneSchemaMap, buildSystemPromptWithTools, toolProjectionResultJSON, or toolResultJSON; internal/toolregistry is the sole owner; all callers use it directly; exact schemas, prompt catalogs, result and projection bytes, execution, role exposure, and Trace behavior are unchanged; duplicate runtime owner tests are removed; the ratchet passes with production file/LOC, test LOC, and wrapper reductions and no unrelated category increase; independent review finds no alias, facade, behavior delta, or evidence gap; and canonical CI/deploy plus authenticated staging product-path proof bind the landed commit.
      assurance:
        independent_verifier: required
        panel: compact
        review_binding: frozen parent, exact diff digest, complete six-symbol caller graph, golden byte vectors, ratchet delta, focused tests, and staged coagent result/update/Texture transition
        define_review_result:
          initial_candidate_diff_sha256: bb2b3ecb0113e298d3a43b32a9b20291c2583cbf86f06944d76eb514ae7c1d4b
          repaired_candidate_diff_sha256: 05cd8832cc4f96644b3f8e789f79477e6e5b05243719a24b737928881c8a02e0
          reviewers:
            - google-antigravity/gemini-3.5-flash: PASS
            - opencode/hy3-free: PASS after bounded repair
          adjudication: Both reviewers recomputed the six-symbol runtime caller graph, exact owner contract, 333-to-391 documentation-only citer rise, ratchet and evidence floor, and prior search-lock closure. Gemini's first pass exposed the pre-existing private prompt composer in internal/toolregistry/toolloop.go; the lock was repaired to include that file in scope and caller graph. Opencode then required the duplicate's explicit deletion target; the final candidate names internal/toolregistry/toolloop.go:buildSystemPromptWithTools in owner_duplicate_deletions. A final immutable Gemini check independently verified the exact repaired hash, valid YAML, all three scope/caller/deletion repairs, one exported BuildSystemPrompt owner, unchanged citer endpoint, and no prior-lock regression with no remaining blocker.
          no_rerun_rationale: Appending this review receipt changes only non-authoritative assurance provenance; it does not change the reviewed lock, caller graph, behavior invariants, evidence floor, or stopping condition.
      validation_notes:
        - Runtime baseline before this Define is go_files 132, production_files 69, test_files 63, production_loc 43047, test_loc 49769, exports 955, export_caller_edges 308, initial_unused_export_debt 15, routes 2, tools 48, production_importers 4, wrappers 4, compatibility_markers 8, store_calls 443, interface_candidates 4, legacy_state_writers 0, legacy_store_reads 0, citers 333.
        - This Define mechanically raises documentation citers from 333 to exactly 391 while every source-category count remains unchanged. Implementation must rebaseline that reviewed documentation-only rise, then reduce runtime production files, production LOC, test LOC, and wrappers without increasing any other source category.
        - Problem documented before scope correction: implementation preflight proved internal/toolregistry/toolregistry_test.go does not exist; the package's existing behavior test owner is internal/toolregistry/toolloop_test.go. The reviewed lock therefore names an impossible owner-test path and cannot execute until a separate authority commit replaces only that path and independently verifies the repaired scope.
      heresy_delta:
        discovered:
          - runtime declares a Tool alias plus schema, prompt-catalog, projection, and result-encoding facades over the existing toolregistry owner
        introduced: []
        repaired: []
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
