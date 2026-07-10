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

### PC-0. Deployment identity follows activation — OPEN, P0

```yaml
id: deployment-identity-follows-activation
kind: boundary
status: testing
source: observed staging acceptance 2026-07-10
definition: >-
  A service health response distinguishes the immutable commit compiled into
  the serving binary from the release target selected by the deploy. The
  release target cannot become accepted deployment identity before the
  affected services are installed, restarted, and healthy.
problem: >-
  The Node B workflow writes CHOIR_DEPLOYED_COMMIT immediately after checkout,
  before selected service builds and activation. buildinfo.Snapshot then
  overwrites the binary Commit field with that mutable file value. During CI
  run 29078163452, proxy /health reported 3f4f4aac while the deploy job was
  still in progress and the auth service still served the pre-repair API-key
  handler: a read-only key minted an admin child with HTTP 201. During the
  rerun of CI run 29079492757 for commit 94416899, the deploy host fetched and
  reset to the then-current origin/main commit b0b6d8af instead of the tested
  workflow commit. The auth fast build then fell back to Nix, whose
  service-specific source filter omitted internal/buildinfo even though
  internal/server now imports it. Deployment failed before activation.
problem_cluster:
  - proxy health exposes mutable repository target identity, not immutable serving-binary identity
  - generic service health, including auth, exposes no compiled build identity
  - the deploy job tests one immutable workflow commit but Node B independently selects a moving branch tip
  - fast and Nix fallback builds use separate source/dependency declarations and inject different build identity fields
  - cmd/choir is not installed on Node B or guest images, yet its source falls through to the conservative full host plus both guest-image deploy class
  - cmd/desktop is a separate Wails distribution module, but its native-only source also falls through to the full platform deploy class; only shared frontend changes belong to Node B
root_cause:
  - deploy.env publishes the target SHA before service activation
  - buildinfo conflates compiled artifact identity with mutable deploy metadata
  - the remote checkout trusts origin/main rather than the immutable workflow SHA that passed CI
  - the auth Nix source filter does not carry internal/server's internal/buildinfo dependency and common Nix ldflags omit Commit and BuiltAt
  - the landing loop treated proxy-global identity as proof for an affected auth service
  - path fallback substitutes repository-wide deployment for an explicit artifact dependency map
protected_surfaces: [deployment routing, run acceptance, service build identity]
settlement_rule: >-
  Node B builds exactly the workflow commit that passed CI; compiled service
  commit remains immutable and independently observable;
  release-target metadata advances only after successful activation; affected
  service identity is probed before product acceptance; an inverted deploy
  contract prevents a target SHA from masquerading as a serving binary SHA;
  undistributed cmd/choir-only and cmd/desktop-native-only changes do not
  activate Node B or guest images.
execution_effect: >-
  Health target identity alone is inadmissible for PC-1 or later settlement.
  Wait for the deploy job and affected service activation before repeating the
  product proof, then repair this boundary before the next identity-only claim.
```

### PC-1. API-key capability delegation — SETTLED, P0

```yaml
id: api-key-capability-delegation
kind: invariant
status: settled
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
deployed_evidence:
  - commit 3f4f4aacb7a77416b528982ec4da47d877858ed9
  - all standard, frontend, deploy, rolling-flake, and differential-SBOM jobs succeeded in CI run 29078163452 before the run container was superseded by the problem-record docs push
  - Node B auth activation job completed successfully at 2026-07-10T08:03:16Z
  - read-only create-admin=403 and revoke-sibling=403
  - denied sibling remained usable with list=200; read-only self-revoke=204
  - manager create-subset=201, create-broader=403, revoke-subset=204
  - cleanup observed zero active codex-capability proof keys
protected_surfaces: [auth/session, API-key authority, headless CLI]
settlement_rule: >-
  Negative tests prove read-only keys cannot create or revoke; delegated key
  managers cannot mint broader scopes; admin and cookie-owner paths remain
  explicit; full auth/proxy tests and staging acceptance are green.
execution_effect: >-
  Settled at 3f4f4aac after affected-service activation and public staging
  denial/delegation proof. This settlement does not settle PC-0: proxy-global
  deploy target identity was proven capable of preceding auth activation.
heresy_delta:
  discovered:
    - Bearer authentication discarded the caller capability envelope
    - read-only keys could mint admin authority and revoke sibling keys
  introduced:
    - explicit manage:keys delegation scope and child-scope subset boundary
  repaired:
    - API-key capability escalation and unauthorized sibling revocation
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

## Authority Convergence and Deletion Map

The repeated failure pattern is not missing functionality. It is one product
purpose represented by multiple peers, none of which is forced to carry the
whole truth. The simplification rule is one authoritative path per purpose;
other paths are evidence, adapters, or deletion candidates.

| Real purpose | Competing paths or meanings | Authoritative path to keep | Delete or demote | Minimum proof |
|---|---|---|---|---|
| Prove what code is serving | compiled service commit; mutable global deploy target; proxy health used for unrelated services | immutable per-service build identity plus completed activation receipt | commit override and repository-global identity inference | affected service reports compiled SHA after deploy job succeeds |
| Activate an approved ComputerVersion | live-app adoption; candidate-intake switch/rollback; optional Dolt tag adapter; vmctl desktop publish; actual proxy/VM route | one route-slot writer with activation/rollback receipt | candidate mutation routes, tag/publish semantics, and active UI/persistence when no executor changed the served route | ordinary owner request resolves through the receipted version |
| Give the desktop a durable authenticated session | direct exchange-redirect flow; bridge flow; JavaScript cookies; native process; cloud and local proxy/auth stacks | one bridge/passkey flow and one native Go cookie jar behind the renderer proxy | direct exchange attempt, raw token responses, JS token/cookie handling, secret logs, production claims for dev-only local orchestration | built app survives reload and renewal; logout clears; JS/log secret scan is empty |
| Turn one source handoff into one processor run | graph-projection synthesis; typed ingestion handoff; non-idempotent retry | typed, durable, idempotent ingestion handoff and runtime admission | projection-triggered `wire_synthesis` and synthesis response fields | one cycle/request ID maps to exactly one processor run and publication lineage |
| Provide private exact-byte computer files | placeholder downloader; path-derived identity; random folder versions; unwired contract builders | stable item/device identity, content-addressed bytes, explicit conflict ancestry | zero-byte success, cursor advance across unresolved conflicts, inert builders as presumed architecture | two-device rename/conflict/restart proof with exact hashes and owner denial |
| Offer a coherent headless client | `run start` creates a prompt-bar submission; 30-second client deadline versus 60-second server bound; undistributed CLI classified as platform code | thin transport over canonical request/submission, trajectory, Texture, and evidence contracts with its own future release lane | misleading run vocabulary, mirrored private schemas, hidden timeout authority, and Node B/guest deploy fallback | CLI/web conformance IDs and shapes match; structured server 504 reaches CLI near 60 seconds; no platform activation for CLI-only diff |

This map is a routing constraint, not a claim that every kept path is already
correct. In particular, typed Autopaper retry is not yet idempotent, Wails has
no native jar yet, Base has no admissible kernel, and promotion has no
load-bearing route executor.

Root-cause clustering applies now. Promotion symptoms share the missing
`route_slot -> ComputerVersion` writer; Base symptoms share the missing stable
identity + exact blob + acknowledged-cursor transaction; Autopaper symptoms
share the missing durable source-cycle transaction. Do not patch another UI,
placeholder, retry, or status symptom in those clusters without repairing or
deleting at the substrate boundary.

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
  false_deploy_identity_paths: 1
  reachable_auth_boundary_failures: 1
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

1. Finish PC-1 API-key capability delegation against the fully activated auth
   service; do not accept proxy-global health identity as sufficient proof.
2. PC-0 deployment identity truth repair.
3. PC-2 Wails token containment.
4. PC-3 CLI timeout and staging acceptance.
5. PC-4 promotion truth gate through the OG Definition.
6. PC-6 Autopaper single activation.
7. PC-5 Base exact-byte/stable-identity kernel.
8. PC-7 Wails package/acceptance lane, then authorized Base/File Provider UI.

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
- claim: API-key capability delegation is enforced after auth activation.
  definition_node: api-key-capability-delegation
  evidence_class: pre-fix negative proof + tests + CI/deploy + public staging matrix
  command_or_observation: >-
    Commit 3f4f4aac; focused, package, race, vet, and frontend checks; CI run
    29078163452; completed Node B auth activation; rate-limit-aware ephemeral
    create/revoke matrix through https://choir.news/auth/api-keys
  result: >-
    Pre-fix read-only create-admin=201 and revoke-sibling=204. After activation,
    read-only create-admin=403 and revoke-sibling=403; denied target remained
    active; self-revoke=204; manager subset create/revoke=201/204; broader
    create=403; zero active proof keys remained after cleanup.
  uncertainty: >-
    Wails still violates the separate native secret-containment boundary. The
    false deployment-identity window remains open as PC-0.
- claim: proxy health target identity can precede affected-service activation.
  definition_node: deployment-identity-follows-activation
  evidence_class: deployed staging timing + source call graph
  command_or_observation: >-
    CI run 29078163452 in progress; https://choir.news/health reported
    deployed_commit 3f4f4aac; immediate ephemeral API-key negative proof
  result: >-
    Read-only Bearer created an admin child with HTTP 201 while the selected
    auth service deploy was still in progress. All three uniquely labelled
    proof keys, including the unexpected admin child, were revoked. Workflow
    writes deploy.env before builds; buildinfo.Snapshot replaces compiled
    Commit with the mutable deployed target.
  uncertainty: >-
    The same negative proof must be repeated after the deploy job completes to
    distinguish the transient activation window from an auth packaging defect.
- claim: cmd/choir-only changes select an unrelated full platform rollout.
  definition_node: deployment-identity-follows-activation
  evidence_class: executable deploy-impact classifier
  command_or_observation: >-
    printf cmd/choir/main.go and cmd/choir/main_test.go into
    .github/scripts/deploy-impact-classify
  result: >-
    deploy_host_service=true, deploy_ordinary_guest=true,
    deploy_playwright_guest=true, vmctl restart and active-VM refresh=true,
    even though cmd/choir has no Node B or guest installation target.
  uncertainty: >-
    A later distribution package will need an explicit release lane; it must
    not silently inherit platform deployment semantics.
- claim: native Wails source selects an unrelated full platform rollout.
  definition_node: deployment-identity-follows-activation
  evidence_class: executable deploy-impact classifier
  command_or_observation: >-
    printf cmd/desktop/main.go, cmd/desktop/desktop_auth.go, and
    frontend/src/lib/auth.js into .github/scripts/deploy-impact-classify
  result: >-
    Native desktop paths select host services, ordinary and Playwright guests,
    vmctl restart, and active-VM refresh. The shared auth.js path separately
    and correctly selects the frontend bundle.
  uncertainty: >-
    PC-7 must create an explicit macOS build/release lane; native desktop paths
    cannot remain a generic ignored class after that lane exists.
- claim: PC-0 landing is blocked by a pre-existing actor lost-wake race failure.
  definition_node: deployment-identity-follows-activation
  evidence_class: CI race result; investigation pending
  command_or_observation: >-
    GitHub Actions run 29079492757, non-runtime race job 86318810189,
    TestNoLostWakeUnderConcurrentSendsAndPassivations
  result: >-
    Standard and all four runtime race lanes passed. The actor test failed
    after 26.24s with "timeout waiting for: every update processed"; the
    aggregate gate then failed and staging deployment did not run.
  uncertainty: >-
    Focused repetition and scheduler/passivation call-graph review must decide
    whether this is a real lost wake or suite-pressure timing failure before
    any actor or timeout change. It is not attributed to PC-0 source.
- claim: the actor race failure was a non-reproducing timing anomaly, and the rerun exposed two deployment authorities.
  definition_node: deployment-identity-follows-activation
  evidence_class: same-SHA CI rerun + deploy log + source/build call graph
  command_or_observation: >-
    GitHub Actions run 29079492757 attempt 2; deploy job 86324338577;
    focused TestNoLostWakeUnderConcurrentSendsAndPassivations race repetition;
    flake.nix auth source filter and commonGoArgs; .github/workflows/ci.yml
    remote checkout
  result: >-
    The actor lane passed on the same SHA without an actor or timeout change.
    The deploy step then reset /opt/go-choir to origin/main at b0b6d8af rather
    than workflow SHA 94416899. Selected auth deployment fell back from the
    host fast build to Nix and failed because the filtered source omitted
    internal/buildinfo, now imported by internal/server. The Nix common
    ldflags also set Version only, unlike the fast build's Version, Commit,
    and BuiltAt. No service was activated by this failed deploy.
  uncertainty: >-
    Repair must pin checkout to the workflow SHA, wire the existing buildinfo
    dependency into the auth package, and make both build paths compile the
    same identity fields before staging acceptance is admissible.
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
  last_checkpoint: deploy rerun selected moving main and failed the auth Nix fallback before activation
  current_artifact_state: >-
    API-key delegation commit 3f4f4aac is active and accepted on staging after
    the auth deploy completed. The transient pre-activation proof remains the
    PC-0 problem record. The CLI timeout repair is green locally but held until
    deployment routing stops treating the undistributed CLI as a full
    host-and-guest rollout. PC-0 commit 94416899 is pushed but not deployed.
    Its same-SHA actor rerun passed, then deploy job 86324338577 reset Node B
    to newer origin/main commit b0b6d8af and failed the auth Nix fallback
    because its source filter omitted internal/buildinfo. No activation ran.
  what_shipped:
    - 3f4f4aac API-key capability-envelope enforcement
    - eb3bdd35 false deployment identity problem record
  what_was_proven:
    - CLI trajectories read works on staging
    - CLI timeout hides the server's bounded 504
    - current source contains reachable API-key and Wails secret-boundary failures
    - Base and Autopaper gaps are substrate/control-path defects, not missing UI alone
    - proxy-global deployed_commit is not affected-service activation proof
    - API-key delegation and sibling revocation are bounded by caller capability on staging
  unproven_or_partial_claims:
    - immutable per-service deployment identity
    - workflow-SHA-pinned deployment and PC-0 clean CI/deploy rerun
    - no Wails built-app acceptance
    - no deployed Autopaper duplicate count
    - no Base exact-byte two-device proof
    - no served ComputerVersion promotion
  highest_impact_remaining_uncertainty: immutable per-service deployment identity
  next_executable_probe: >-
    Pin Node B checkout to the tested workflow SHA, add internal/buildinfo to
    the auth Nix source closure, inject Commit and BuiltAt through the shared
    Nix build path, then rerun CI/deploy and verify affected service identity.
    Only then land and stage-time the prepared CLI timeout slice.
  suggested_goal_string: "/goal docs/definitions/choir-product-completion-2026-07-10.md"
  evidence_artifact_refs:
    - this Definition's evidence ledger
    - docs/definitions/og-dolt-heresy-completion-2026-07-08.md
  rollback_refs:
    - 224243de (pre-program source state)
    - b7f689d4 (pre-API-key behavior repair)
```
