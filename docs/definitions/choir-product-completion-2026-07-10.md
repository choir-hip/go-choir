# Choir Product Completion: CLI, Desktop, Base, and Autopaper

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-product-completion-2026-07-10.md
```

Read this document as executable semantic authority. Reconcile its determined
state with current source and staging, execute safe in-bound probes, update the
definition graph and evidence ledger, and continue until the completion
semantics are satisfied or an explicit blocker/supersession condition is met.
A checkpoint is not completion.

## Source Authority Order

1. `docs/choir-doctrine.md`
2. `AGENTS.md`
3. `docs/computer-ontology.md`
4. `docs/agent-product-doctrine.md`
5. this Definition for CLI, Wails desktop, Choir Base, and Autopaper work
6. `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` for
   storage/heresy/promotion authority
7. `docs/runtime-invariants.md`, `docs/source-external-data-publication.md`,
   and `docs/texture-agentic-invariants-2026-06-13.md`
8. observed source, tests, CI, and staging evidence

This document is a disjoint companion to the OG/Dolt/heresy Definition. It may
record promotion dependencies and false-success evidence, but it cannot rewrite
promotion semantics, phase gates, or rollback authority owned there.

## Real Artifact / Object of Work

The real object is the set of owner-facing product paths that connect Choir's
computer substrate to a usable headless client, native desktop, private file
substrate, and automatic publication loop:

- `cmd/choir` and its authenticated deployed API behavior;
- `cmd/desktop`, the shipped Wails bundle, and native authentication/session
  containment;
- `internal/base`, `internal/desktop`, and any eventual deployed Base owner;
- the current source-cycle → runtime → Texture → edition publication path that
  may become Autopaper;
- their tests, packaging, deployment, staging acceptance, and rollback refs.

The object is not the historical Autopaper mission corpus, the presence of
classes or handlers, a passing unit suite, an unbuilt Wails wrapper, or a set of
API records that no served product path consumes.

## Mission Purpose and Non-Purpose

Purpose:

1. remove reachable authority and secret-containment failures first;
2. make existing CLI behavior coherent and prove it against staging;
3. define and prove an exact-byte, stable-identity Base kernel before product
   wiring or File Provider packaging;
4. revive Autopaper from current Texture/source/publication contracts, starting
   with one authoritative activation per ingestion handoff;
5. make Wails a tested, packaged, daily-driver candidate rather than a wrapper
   inferred to work from shared frontend tests.

Non-purpose:

- This mission does not resurrect deleted plans as authority.
- Choir Base does not become a competing canonical app-state store; embedded
  Dolt remains authoritative for private product/app state.
- Autopaper is not authorized as a separate service or free-form scheduler.
- CLI verbs must not turn adoption/lineage records into false promotion claims.
- Wails must not trade away HttpOnly secret containment for native convenience.
- This mission does not enable the interim tag-only Dolt promotion adapter.

## Mutation Classes and Rollback

- Documentation and status correction: `green`.
- CLI timeout/configuration and tests: `orange`; rollback to the pre-change CLI
  commit.
- API-key delegation, Wails auth/session, promotion/rollback claims, private
  Base state, and publication activation: `red`; each slice requires a problem
  record, conjecture delta, protected surfaces, admissible evidence, rollback,
  and heresy delta.
- Destructive Base resets, table drops, or production state repair: `black` and
  require explicit owner approval.

Initial rollback ref: `224243de`.

## Definition Graph

### PC-1. API-key capability delegation — OPEN, P0

```yaml
id: api-key-capability-delegation
kind: invariant
status: testing
source: observed 2026-07-10
definition: >-
  A Bearer API key may never mint, broaden, or revoke authority beyond its own
  delegated capability. Cookie-authenticated owner sessions retain owner key
  management authority.
problem: >-
  Bearer authentication validates a key but discards its scopes before the
  /auth/api-keys handlers run. Any unrevoked read-only key can request a new key
  with any valid scope, including admin, and can revoke sibling owner keys.
observables:
  - internal/auth/handlers.go validateAPIKey and API-key management handlers
  - internal/auth/handlers_test.go bearer-to-bearer creation contract
  - cmd/choir api-key create/revoke surface
existing_replacement: >-
  Proxy route authorization already carries and enforces API-key scopes. The
  auth handler must preserve the same capability envelope and enforce explicit
  key-management delegation plus child-scope subset rules.
construction:
  - add explicit manage:keys scope
  - preserve the validated Bearer key through API-key management authentication
  - require manage:keys or admin for sibling create/revoke operations
  - constrain delegated and revoked sibling scopes to the caller's envelope
  - retain cookie-owner authority and safe Bearer self-revocation
local_evidence:
  - pre-fix negative tests observed read-only create=201 and sibling revoke=204
  - post-fix negative, subset, admin, cookie-owner, list, and self-revoke tests pass
  - full internal/auth and internal/proxy suites pass
  - focused auth/proxy race suites and go vet pass
protected_surfaces: [auth/session, API-key authority, headless CLI]
settlement_rule: >-
  Negative tests prove read-only keys cannot create or revoke; delegated key
  managers cannot mint broader scopes; admin and cookie-owner paths remain
  explicit; full auth/proxy tests and staging acceptance are green.
execution_effect: >-
  Local repair is testing. No deployed repair claim until CI, Node B identity,
  and non-destructive staging denial/delegation evidence are green. Do not
  broaden CLI default scopes or add mutation verbs before settlement.
```

### PC-2. Wails token containment — OPEN, P0

```yaml
id: wails-token-containment
kind: invariant
status: testing
source: observed 2026-07-10
definition: >-
  Native passkey exchange codes, access tokens, and refresh tokens never enter
  JavaScript-readable state or logs. The native boundary establishes an
  HttpOnly same-origin session and returns only non-secret auth state.
problem: >-
  The reachable desktop bridge logs the callback URL containing the exchange
  code, returns raw access and refresh tokens to JavaScript, and auth.js writes
  both as JavaScript-readable cookies. The compiled frontend contains this path.
observables:
  - cmd/desktop/main.go desktop auth routing, callback logging, and token response
  - frontend/src/lib/auth.js desktop-auth bridge and document.cookie writes
  - built Wails app document.cookie, hard reload, renewal, logout, and unified log
protected_surfaces: [auth/session renewal, native desktop, passkey exchange]
settlement_rule: >-
  Unit/source invariants and a built-app staging proof show HttpOnly containment,
  successful reload/renewal/logout, and no code/token in response bodies,
  JavaScript, or logs.
execution_effect: >-
  Fix before treating the Wails wrapper as distributable or daily-driver ready.
```

### PC-3. CLI request-budget and capability coherence — OPEN, P1

```yaml
id: cli-request-budget
kind: conjecture
status: testing
source: observed local + staging 2026-07-10
claim: >-
  The CLI can wait through the server's bounded 60-second computer-resolution
  window and surface its structured response, while retaining an explicit user
  timeout override.
problem: >-
  cmd/choir hard-codes a 30-second http.Client timeout. Authenticated staging
  `wire diagnostics` therefore exits at 30 seconds with context deadline
  exceeded, while direct curl receives the server's structured 504 at 60.12
  seconds.
evidence:
  - go test and go vet for ./cmd/choir pass
  - authenticated `choir trajectories` succeeds with four records
  - authenticated `choir wire diagnostics` times out at 30 seconds
  - direct endpoint returns HTTP 504 at 60.12 seconds
existing_replacement: >-
  No CLI timeout configuration exists. internal/desktop already recognizes a
  longer integration budget, but the CLI needs its own flag/environment contract.
settlement_rule: >-
  Default exceeds the server resolution bound; CHOIR_TIMEOUT and --timeout are
  tested; delayed-server cancellation remains bounded; staging surfaces the
  structured 504 after roughly 60 seconds instead of cancelling at 30.
execution_effect: >-
  Implement after PC-1 is recorded. Do not add write scopes to default CLI keys
  until capability delegation is repaired.
```

### PC-4. Promotion truth gate — DEPENDENCY ON OG PHASE D

```yaml
id: promotion-truth-gate
kind: boundary
status: proposed
source: observed 2026-07-10; authority remains OG/Dolt/heresy Definition
definition: >-
  Adoption verification and owner approval are not served activation. No API or
  UI may persist or display active/rollback-success unless a load-bearing
  ComputerVersion route executor changed the served version and emitted a
  receipt.
problem: >-
  Features can persist adopted and show Activated with rollback available when
  the optional tag adapter is absent or fails. Rollback can advance lineage
  after best-effort DOLT_RESET fails. Ordinary proxy routing consumes neither.
execution_effect: >-
  The active OG Definition must document and execute the red truth-gate slice.
  This mission may verify user-facing claims but may not redefine the protocol.
```

### PC-5. Choir Base exact-byte and stable-identity kernel — OPEN

```yaml
id: base-exact-byte-kernel
kind: object
status: proposed
source: observed 2026-07-10 + computer ontology
definition: >-
  A per-computer private file/source substrate with append-only observations,
  content-addressed exact bytes, stable owner/device/item identity, explicit
  conflict ancestry, and derived materialization. It does not own canonical
  Texture, runtime, trajectory, or promotion truth.
problem_cluster:
  - no deployed service owns /api/base routes and owner scoping is incomplete
  - remote downloads write zero-byte placeholders and mark them synced
  - unresolved-conflict persistence advances the full remote cursor
  - path-derived item IDs break identity on rename and remote materialization
  - folder versions randomize on each scan
  - 38 unused base_* contract builders risk becoming architecture by inertia
classification: substrate
settlement_rule: >-
  A fresh two-device proof demonstrates exact bytes/hash, cross-owner denial,
  rename-stable identity, conflict cursor retention and all resolution choices,
  plus restart durability before any Wails/File Provider/deployed wiring.
execution_effect: >-
  Do not connect the existing handlers or SyncService yet. First define service
  ownership, identity namespace, cursor ancestry, Dolt boundary, and
  ArtifactProgramRef resolution.
```

### PC-6. Autopaper single authoritative activation — OPEN

```yaml
id: autopaper-single-activation
kind: conjecture
status: proposed
source: user-restated 2026-07-10 + observed current source/publication path
provisional_definition: >-
  Autopaper is an automatic publication program inside a Choir computer:
  scheduled Source configurations produce typed observations; one authoritative
  ingestion handoff activates processing; canonical Texture artifacts become an
  edition only through explicit publication contracts. It is not a separate
  service and does not bypass Texture or provenance authority.
problem: >-
  Every non-empty source cycle currently has two processor activation paths.
  The web-capture projection path starts a run implicitly, then sourcecycled
  independently dispatches the typed ingestion handoff. Overload can turn this
  into delayed sequential duplicates rather than obvious parallel duplicates.
existing_replacement: >-
  BuildIngestionHandoff plus /internal/runtime/runs is already the typed,
  sourcecycled-tracked activation path. Projection should persist observations,
  not synthesize an untracked run.
open_definition_edges:
  - personal versus platform publication ownership
  - schedule ownership and per-computer configuration
  - edition/publication acceptance and retry identity
settlement_rule: >-
  Delete projection-triggered activation, prove exactly one run per handoff
  across retry/overload, preserve capture projection, then prove one deployed
  source cycle to canonical Texture/edition evidence before widening scope.
execution_effect: >-
  The single-activation repair may proceed without settling the wider product
  edges because duplicate processing is invalid under every admissible
  Autopaper definition.
```

### PC-7. Wails build and packaging lane — OPEN

```yaml
id: wails-build-package-lane
kind: observable
status: proposed
source: observed 2026-07-10
definition: >-
  The nested cmd/desktop module, generated frontend embed, native package,
  File Provider extension when authorized, and staging login/sync smoke have an
  executable CI/acceptance lane.
problem: >-
  cmd/desktop is outside root CI, has zero Go tests, fails without copied
  frontend assets, and does not package/register the File Provider extension.
  SyncService is registered but unused by the frontend.
execution_effect: >-
  Establish after PC-2. Do not package Base/File Provider before PC-5 settles.
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: The user reopened CLI, Wails, Choir Base, and Autopaper as real product work.
      source: user-stated 2026-07-10
      execution_effect: These loops require current Definitions rather than deleted plans.
    - claim: Promotion protocol authority remains in the OG/Dolt/heresy Definition.
      source: retained authority graph
      execution_effect: PC-4 is a dependency/truth gate, not competing protocol authority.
    - claim: Choir Base cannot replace embedded Dolt as canonical private app-state authority.
      source: computer ontology and semantic registry
      execution_effect: Base work begins with a bounded exact-byte kernel.
  observed:
    - claim: Two reachable auth-boundary failures precede product expansion.
      source: source call-graph audit
    - claim: CLI timeout, promotion false-success, Base data integrity, and Autopaper duplicate activation are reproducible code-path defects.
      source: source and staging audit 2026-07-10
  open:
    - node: autopaper-single-activation
      missing: wider product ownership, schedule, and edition acceptance semantics
    - node: base-exact-byte-kernel
      missing: deployed owner, identity namespace, conflict ancestry, and ArtifactProgramRef binding
```

## Invariants

1. Secrets and delegated authority never cross into a weaker observer.
2. Canonical state does not change before verified, approved, receipted
   promotion.
3. One ingestion identity causes at most one authoritative processor
   activation; retry is idempotent.
4. Base materialization never invents bytes, identity, ancestry, or success.
5. Texture owns canonical Autopaper documents; source captures and processor
   packets are evidence inputs.
6. No local/build/test artifact authorizes a deployed or daily-driver claim.
7. Existing unsafe wiring is deleted or fail-closed before new product surface
   is added.

## Value Criterion and Variant

Priority is: reachable authority/secret containment, false canonical-success
claims, durable data integrity, duplicate side effects, then distribution and
surface expansion.

```yaml
variant:
  reachable_auth_boundary_failures: 2
  false_promotion_success_paths: 1
  cli_product_contract_failures: 2
  base_substrate_invariants_open: 5
  autopaper_authoritative_activation_paths: 2
  wails_unowned_build_acceptance_lanes: 1
  external_product_nodes_without_deployed_acceptance: 4
target:
  all_counts: 0
```

## Execution Order

1. PC-1 API-key capability delegation.
2. PC-2 Wails token containment.
3. PC-3 CLI timeout and staging acceptance.
4. PC-4 promotion truth gate through the OG Definition.
5. PC-6 Autopaper single activation.
6. PC-5 Base exact-byte/stable-identity kernel.
7. PC-7 Wails package/acceptance lane, then authorized Base/File Provider UI.

The order may change only when new evidence changes blast radius or unlocks a
strictly safer dependency. Base and File Provider wiring may not jump PC-5.

## Evidence Ledger

```yaml
- claim: CLI transport timeout is shorter than the deployed server contract.
  definition_node: cli-request-budget
  evidence_class: observed source + deployed staging timing
  command_or_observation: >-
    go build ./cmd/choir; choir wire diagnostics; authenticated curl with
    --max-time 70 to /api/universal-wire/stories
  result: CLI deadline at 30s; server HTTP 504 at 60.12s
  uncertainty: underlying platform-computer availability remains a separate failure
- claim: CLI read path itself is live.
  definition_node: cli-request-budget
  evidence_class: deployed staging proof
  command_or_observation: authenticated choir trajectories
  result: HTTP success and four decoded document trajectories
  uncertainty: no supported distribution artifact yet
- claim: API-key scope escalation is reachable in source.
  definition_node: api-key-capability-delegation
  evidence_class: code-level call graph + existing positive test
  result: read-only Bearer may create arbitrary valid-scope child key and revoke sibling keys
  uncertainty: staging exploit was not executed because source proof is sufficient and mutation is unsafe
- claim: Autopaper has two activation paths per non-empty source cycle.
  definition_node: autopaper-single-activation
  evidence_class: code-level call graph
  result: web-capture projection starts a run before sourcecycled dispatches the typed handoff
  uncertainty: deployed duplicate IDs/counts not yet captured
```

## Completion Semantics

This mission is complete only when:

1. PC-1 and PC-2 are repaired with negative tests and deployed/native evidence;
2. CLI commands have coherent capabilities, explicit time budgets, a supported
   build/distribution path, and recorded staging acceptance;
3. promotion surfaces cannot claim activation without a served-route receipt;
4. Autopaper has one authoritative activation, settled product ownership and
   schedule semantics, and a deployed source → Texture → edition proof;
5. Base passes the exact-byte/two-device/restart kernel and has an explicit
   deployed owner without competing with Dolt;
6. Wails has a CI build/package lane plus real passkey/reload/logout acceptance;
7. residual risks, rollback refs, protected surfaces, and heresy deltas are
   recorded for every behavior slice.

Passing unit tests, an API record labeled adopted, a local `.app`, a registered
SyncService, or one published Texture is not completion.

## Run Checkpoint and Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: source/staging audit at 224243de
  current_artifact_state: >-
    The API-key delegation repair is green locally but unlanded. Current
    evidence also records Wails token exposure, a CLI timeout mismatch, false
    promotion success, a broken Base synchronization kernel, and duplicate
    Autopaper activation.
  what_shipped: []
  what_was_proven:
    - CLI trajectories read works on staging
    - CLI timeout hides the server's bounded 504
    - current source contains reachable API-key and Wails secret-boundary failures
    - Base and Autopaper gaps are substrate/control-path defects, not missing UI alone
  unproven_or_partial_claims:
    - API-key delegation CI/deploy/staging proof
    - no Wails built-app acceptance
    - no deployed Autopaper duplicate count
    - no Base exact-byte two-device proof
    - no served ComputerVersion promotion
  highest_impact_remaining_uncertainty: API-key delegation deployed enforcement
  next_executable_probe: >-
    Commit and push the API-key capability-envelope repair, require auth/proxy
    standard and race CI green, verify Node B identity, then prove on staging
    that a read-only Bearer cannot mint or revoke sibling authority while an
    explicitly delegated manager can mint only a subset key.
  suggested_goal_string: "/goal docs/definitions/choir-product-completion-2026-07-10.md"
  evidence_artifact_refs:
    - this Definition's evidence ledger
    - docs/definitions/og-dolt-heresy-completion-2026-07-08.md
  rollback_refs:
    - 224243de (pre-program source state)
```
