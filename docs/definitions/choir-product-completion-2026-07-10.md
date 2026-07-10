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

### PC-0. Deployment identity follows activation — TESTING, P0

```yaml
id: deployment-identity-follows-activation
kind: boundary
status: testing
source: observed staging acceptance 2026-07-10; service-scoped receipt check added by seam-repair 2026-07-10
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
  - vmctl stamps its own process build identity into the separately built sandbox runtime package
  - deployed-origin acceptance requires frontend and proxy compiled commits to be equal even though differential deployments intentionally advance them independently
  - cmd/choir is not installed on Node B or guest images, yet its source falls through to the conservative full host plus both guest-image deploy class
  - cmd/desktop is a separate Wails distribution module, but its native-only source also falls through to the full platform deploy class; only shared frontend changes belong to Node B
root_cause:
  - deploy.env publishes the target SHA before service activation
  - buildinfo conflates compiled artifact identity with mutable deploy metadata
  - the remote checkout trusts origin/main rather than the immutable workflow SHA that passed CI
  - the auth Nix source filter does not carry internal/server's internal/buildinfo dependency and common Nix ldflags omit Commit and BuiltAt
  - a global cross-component equality assertion substitutes for selected-component activation receipts
  - runtime-package generation inherits vmctl identity instead of consuming a sandbox artifact manifest
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
  Product acceptance names the affected artifact's compiled identity and a
  completed activation receipt. Mutable target metadata, another component's
  commit, or an in-progress deployment remains inadmissible.
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
  HttpOnly same-origin session and returns only non-secret auth state. The auth
  service is the sole cookie-name, attribute, token-minting, rotation, and
  revocation authority; Wails owns only a native cookie jar and proxy boundary.
problem: >-
  The first containment slice removed callback/token logging, JavaScript token
  responses, and document.cookie writes, but it retained a deeper split
  authority. Auth exposes both JSON and redirect exchange-code issuers; each
  stores raw access and refresh bearer tokens in plaintext
  desktop_exchange_codes. Redeem returns both tokens as JSON, and Wails then
  redefines the auth service's cookie names, paths, and attributes in
  seedSession. Safari and Wails consequently share one refresh credential, so
  the first client to rotate it invalidates the other. desktop-bridge.html also
  retains a dead JSON exchange caller.
observables:
  - internal/auth/handlers.go HandleDesktopExchange and HandleDesktopExchangeRedirect mint the same authority through two routes
  - internal/auth/store.go desktop_exchange_codes persists access_token and refresh_token values and ConsumeDesktopExchangeCode returns them
  - internal/auth/handlers.go HandleDesktopRedeem serializes raw bearer credentials instead of using issueSession
  - cmd/desktop/desktop_auth.go desktopTokenResponse and seedSession duplicate canonical server cookie policy
  - frontend/public/desktop-bridge.html exchangeTokens is an unused JSON-exchange path while every live branch uses exchange-redirect
existing_replacement: >-
  Handler.issueSession already mints the access JWT, stores only a refresh-token
  hash, and sets the canonical HttpOnly cookies. http.Client plus its native
  cookie jar already absorbs Set-Cookie on redemption and can proxy those
  cookies without exposing values to the renderer.
authority_cut:
  - retain only the redirect exchange issuer used by ASWebAuthenticationSession
  - persist only an opaque one-time code, authenticated user reference, and expiry; compatibility token columns must contain empty values until removed by a later schema migration
  - redeem consumes the code, resolves the user, and invokes issueSession; the response body contains authenticated/user state and no token-shaped fields
  - delete Wails token decoding and seedSession; the native jar accepts only auth-service Set-Cookie policy
  - delete the dead bridge JSON exchange function and the auth route registration/handler it targeted
protected_surfaces: [auth/session renewal, native desktop, passkey exchange]
mutation_class: red
admissible_evidence:
  - source and handler tests prove one exchange issuer and zero raw-token response fields/callers
  - store tests inspect desktop_exchange_codes and prove no non-empty bearer value is persisted
  - redemption tests prove canonical Secure, HttpOnly, SameSite, path, and TTL attributes plus a safe body
  - coexistence tests prove Safari and native receive independent refresh sessions and can rotate in either order without signing the other out
  - native proxy tests prove the jar absorbs redemption/rotation/logout cookies while renderer bodies and headers remain secret-free
  - a built-app staging proof covers passkey, hard reload, renewal, logout, and unified-log/source scans
rollback_path: >-
  Before distribution, revert to the process-local native jar with desktop auth
  disabled. Never restore raw-token JSON redemption, plaintext exchange storage,
  JavaScript cookies, or the duplicate JSON exchange issuer.
heresy_delta:
  discovered:
    - plaintext bearer credentials stored behind an exchange code
    - auth-service cookie policy reimplemented by Wails
    - duplicate JSON and redirect exchange issuers
  introduced: []
  repaired:
    - JavaScript-readable desktop session tokens and secret callback logging
settlement_rule: >-
  Unit/source invariants and a built-app staging proof show HttpOnly containment,
  one redirect-only issuer, empty/absent exchange-token storage, server-owned
  cookie policy, successful reload/renewal/logout, and no code/token in response
  bodies, JavaScript, or logs.
execution_effect: >-
  The current native-jar patch is containment, not settlement. Delete the
  duplicate issuers and make server Set-Cookie the only session mint before
  treating the Wails wrapper as distributable or daily-driver ready.
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

### PC-4. Promotion truth gate — TESTING: ROUTE-SLOT WRITER FORMAT FIXED

```yaml
id: promotion-truth-gate
kind: boundary
status: testing
source: observed 2026-07-10; RouteProfile format fixed to owner_id/computer_id by seam-repair 2026-07-10; authority remains OG/Dolt/heresy Definition
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
source: observed source/caller audit 2026-07-10 + computer ontology
definition: >-
  A per-computer private file/source substrate with append-only observations,
  content-addressed exact bytes, stable owner/device/item identity, explicit
  conflict ancestry, and derived materialization. It does not own canonical
  Texture, runtime, trajectory, or promotion truth.
non_definition:
  - a path-keyed desktop folder scanner
  - a REST handler collection or registered Wails service
  - a zero-byte presence projection awaiting later byte repair
  - the internal/computerversion Base contract-builder tower
  - a canonical private app-state store competing with embedded Dolt
classification: substrate
authority_cut:
  kernel: >-
    One computer-scoped observation and acknowledgement protocol is the only
    authority for Base identity, bytes, ancestry, conflicts, and cursor
    progress. internal/desktop is a future adapter; REST, Wails, and File
    Provider are future transports or projections; computerversion is evidence
    and materialization support.
  immediate_effect: >-
    SyncEngine, the registered SyncService, local JSON synced state, and the
    self-validating Base contract-builder tower were deleted by seam-repair
    2026-07-10 (commit 0434ff13 / 7b59d33e). Do not resurrect them as kernel
    authority. Do not patch the placeholder downloader as an incremental route
    to PC-5.
missing_kernel_transaction:
  name: computer-scoped-observation-and-acknowledgement
  transaction_identity:
    - computer_ref
    - authenticated_owner_id
    - registered_device_id
    - stable_item_id
    - expected_parent_event_id
    - idempotency_key
  commit_observation_phase: >-
    Resolve computer and owner authority; allocate or validate a stable ItemID;
    read the file bytes once; derive the sole BlobRef from those bytes; durably
    store and re-hash the blob; compare the caller's explicit expected parent;
    then append one immutable observation and return a receipt containing the
    item, event, parent, blob, and cursor identities. Empty file bytes are
    valid. Oversize, partial-read, missing, corrupt, hash-mismatched, stale,
    cross-owner, and cross-computer inputs fail closed before an accepted event
    can reference them.
  acknowledge_materialization_phase: >-
    After a device has durably materialized exact bytes or durably recorded and
    resolved a conflict, append/update its contiguous acknowledged position
    under (computer, owner, device). An error or unresolved event at cursor k
    prevents acknowledgement of k or any later cursor as one accepted prefix.
    Conflict records, choices, pending observations, identity mappings, and the
    last acknowledged cursor survive process restart.
  conflict_rule: >-
    A stale expected parent is never silently rewritten to the current head.
    The rejected or branched observation preserves its proposed parent and both
    byte refs so the common ancestor and every resolution remain inspectable.
  derived_materialization_rule: >-
    Filesystem state is a projection of accepted observations and verified
    blobs. Materialization cannot invent bytes, allocate replacement identity,
    or advance acknowledgement merely because a path now exists.
identity_laws:
  - owner identity comes from authenticated authority, never a request body
  - device identity is registered and durably unique, never a constant path basename
  - ItemID is allocated/adopted once and survives rename, move, restart, and materialization
  - path and parent/name are mutable location observations, not object identity
  - computer scope is explicit or structurally isolated and resolves ArtifactProgramRef
content_laws:
  - BlobRef is the single SHA-256 authority for file bytes
  - ContentHash, when exposed for compatibility, is derived from BlobRef and never an independent write input
  - VersionID identifies an immutable observation and cannot override unequal BlobRefs
  - a folder scan with no semantic change cannot mint a new version
problem_cluster:
  - no deployed service owns /api/base routes and owner scoping is incomplete
  - remote downloads write zero-byte placeholders and mark them synced
  - unresolved-conflict persistence advances the full remote cursor
  - path-derived item IDs break identity on rename and remote materialization
  - folder versions randomize on each scan
  - desktop device identity is desktop:.choir on every machine
  - blob upload rejects valid empty files and silently accepts a truncated 64 MiB prefix
  - caller-controlled BlobRef, ContentHash, VersionID, OwnerID, and DeviceID are not bound into one authority check
  - the journal auto-selects the latest parent by ItemID and cannot express a stale expected parent
  - server device cursors are unused while a second client JSON cursor claims full progress
  - keep_both uploads and downloads the same ItemID instead of preserving two objects
  - event folding, tree types, REST schemas, status, and materialization each have competing implementations
  - 38 unused base_* contract files and their tests were deleted by seam-repair 2026-07-10; do not resurrect them as architecture
observed_contradictions:
  - >-
    internal/base/model and planner require path-independent ItemID. The former
    path-hashing desktop localtree scanner was deleted by seam-repair 2026-07-10;
    any future desktop adapter must not reintroduce path-as-ItemID.
  - >-
    Historical desktop device identity derived every device ID as desktop plus
    the basename of ~/.choir (desktop:.choir across devices). That SyncEngine
    path is deleted; the identity defect remains a kernel requirement for any
    future adapter.
  - >-
    internal/base/blob/store.go verifies exact content-addressed bytes and
    internal/computerversion/TreeToFS materializes them safely, while
    internal/desktop/sync.go writes an empty placeholder and returns success.
  - >-
    internal/base/api/handlers.go uses a 64 MiB LimitReader without checking an
    additional byte, rejects a zero-length request, trusts a supplied OwnerID,
    and authenticates reads before returning an unfiltered global tree/delta.
  - >-
    internal/base/planner versionsEqual trusts equal client-controlled
    VersionIDs before comparing bytes; model Version.Valid and POST /items do
    not require ContentHash to equal BlobRef or prove that the blob exists.
  - >-
    internal/base/journal device cursors are test-only and keyed by device alone;
    desktop persistState ignores its executed set and stores the full delta
    cursor after errors and unresolved conflicts.
  - >-
    tree.Derive, desktop applyDelta, and planner.ApplyEvent implement three event
    folds; the latter two are incomplete or wrong for updates against an
    existing snapshot.
existing_replacement:
  keep:
    - internal/base/blob Store Put/Get/Stat exact-byte substrate
    - internal/base/tree Derive canonical replay
    - internal/base/planner Plan pure three-tree reconciliation
    - internal/computerversion StateGenerator, GenerateFromEvents, and TreeToFS
    - internal/computerversion Base current-state observation and blob integrity readers
  limits: >-
    These are useful primitives, not a kernel. No existing code supplies stable
    local identity, computer/owner-scoped expected-parent commit, durable
    conflict choice, contiguous acknowledgement, or ArtifactProgramRef
    resolution. The current materializer also requires an already-correctly
    scoped journal/blob binding.
wiring_prohibition:
  status: binding
  until: every pre-wiring row in the PC-5 acceptance matrix passes
  prohibited:
    - mount /api/base handlers in any deployed service
    - reintroduce or enable StartSync from a Wails SyncService (deleted by seam-repair 2026-07-10)
    - package or register the Base-backed File Provider extension
    - add a blob GET endpoint as a substitute for the kernel transaction
    - cite fake HTTP, MemJournal, path-rescan, contract-builder, or placeholder tests as kernel proof
  permitted: >-
    cmd/baseharness and cmd/evidenceroot may remain explicitly local fixture and
    evidence tools. The SyncService registration and SyncEngine subtree were
    deleted by seam-repair 2026-07-10; do not reintroduce them before the PC-5
    acceptance matrix passes.
mutation_class: red
protected_surfaces:
  - private persistent state
  - owner and computer authorization
  - exact blob bytes
  - stable identity and conflict ancestry
  - device acknowledgement and restart durability
admissible_kernel_evidence: >-
  A fresh integration proof using real SQLite persistence, real filesystem blob
  stores, two independently persisted device roots, a second owner, actual
  process close/reopen boundaries, and byte-for-byte/hash assertions. Pure
  planner tests remain useful component evidence but cannot settle the object.
conjecture_delta:
  falsified: >-
    Adding blob download to SyncEngine or mounting the existing handlers would
    complete Base. Those moves retain path identity, unscoped authority,
    auto-parented ancestry, split hash inputs, and false cursor success.
  supported: >-
    blob.Store, tree.Derive, planner.Plan, and the concrete computerversion
    generator/observer code are reusable beneath one new authority transaction.
rollback_policy:
  pre_wiring: >-
    Build and test only against fresh fixture roots. Rollback is commit revert
    plus fixture deletion; no live Base data migration, route, Wails service, or
    File Provider state is authorized.
  future_product_cutover: >-
    Requires a separately documented schema/data migration, previous accepted
    kernel version, persistence backup/restore probe, and route-level disable
    path. The current placeholder SyncEngine is not an admissible rollback
    target because it violates byte and cursor invariants.
heresy_delta:
  discovered:
    - path-as-item identity and constant desktop device identity
    - placeholder existence and fetched cursor treated as accepted materialization
    - client-supplied owner/hash/version fields treated as authority
    - linear auto-parent history treated as explicit conflict ancestry
    - inert contract builders treated as architecture candidates
  introduced: []
  repaired: []
settlement_rule: >-
  Every pre-wiring acceptance row passes in one fresh two-device trajectory;
  the deletion/authority cut is complete; exact bytes/hash, cross-owner denial,
  rename-stable identity, conflict cursor retention, all resolution choices,
  replay idempotence, ArtifactProgramRef binding, and restart durability are
  evidenced before any Wails, File Provider, REST, or deployed wiring.
execution_effect: >-
  First document the red problem, then build the transaction behind a fresh
  kernel-only acceptance caller. Do not connect the existing handlers or
  deleted SyncService. After the kernel gate passes, separately define and prove a
  deployed service owner without widening Base into embedded-Dolt app authority.
```

#### PC-5 Acceptance Matrix

The matrix is an executable gate. Rows marked `pre-wiring` must pass together
before any Base-backed API, Wails, or File Provider product path is enabled.

| Gate | Phase | Executable setup | Passing observation |
|---|---|---|---|
| Computer and owner scope | pre-wiring | Create one computer for owner A with devices A1/A2, plus owner B and a second computer; deliberately reuse guessed item/blob identifiers. | B and the second computer cannot read, delta, mutate, acknowledge, or materialize A's state; physical blob deduplication never grants logical access. |
| Exact bytes | pre-wiring | A1 commits an empty file, binary bytes containing NULs, ordinary text, and supported size-boundary files; A2 materializes them. | Every length and byte sequence matches exactly; SHA-256 equals BlobRef; oversize, partial-read, missing, and corrupt inputs fail without an accepted observation or cursor advance. |
| Stable identity | pre-wiring | Create a file and folder, close/reopen A1, rename/move both, sync A2, and rescan unchanged state. | Each object keeps one ItemID; rename is a location observation rather than delete/create; unchanged folders mint no new version. |
| Explicit ancestry | pre-wiring | Disconnect A1/A2 after a shared parent, edit both, then submit both expected-parent observations. | The stale/concurrent branch is not auto-parented to the latest head; conflict evidence names the common parent and preserves both byte refs. |
| Cursor retention | pre-wiring | Place an unresolved conflict at cursor k followed by an independent event; fail and restart materialization between pulls. | The contiguous acknowledgement remains below k until durable resolution/materialization; it advances exactly once afterward and never jumps to the full fetched cursor. |
| All resolutions | pre-wiring | Replay the same conflict independently with keep-local, keep-remote, and keep-both. | Local and remote preserve the chosen exact bytes; keep-both produces two stable ItemIDs and two non-colliding paths; no side is silently overwritten. |
| Restart durability | pre-wiring | Close and reopen journal, blob store, identity map, pending observation/conflict store, and device cursor between every major transition. | Identity, pending work, choices, ancestry, acknowledged position, and materialized bytes are unchanged after restart. |
| Idempotent delivery | pre-wiring | Repeat the same idempotency key/event receipt and redeliver an accepted delta around restart. | One observation/version exists, tree and bytes are unchanged, and acknowledgement does not double-advance. |
| Canonical replay and materialization | pre-wiring | Derive and materialize the same accepted tape through the retained tree/blob/generator path. | One replay implementation determines state; no desktop merge or placeholder path participates; traversal and blob mismatch fail closed. |
| Artifact and Dolt boundary | pre-wiring | Resolve the tape/blob set from its ArtifactProgramRef and compare the emitted file/blob observations. | The ref selects exactly that state; Base writes no canonical Texture/runtime/trajectory/promotion or embedded-Dolt app truth. |
| Service ownership | post-gate only | After all prior rows pass, name one deployed owner/router and repeat owner denial plus exact-byte acceptance on staging. | The serving service and build identity are explicit, scoped requests traverse the proved kernel, and Wails/File Provider remain adapters rather than alternate authorities. |

Current passing tests do not satisfy this matrix. The former
`TestSyncEngineDownloadCycle` / `TestLocalTreeBuilderDeterministicIDs` and Base
contract-builder suites were deleted with SyncEngine and the contract tower by
seam-repair 2026-07-10. Remaining Base/component tests still do not prove the
kernel transaction; only the PC-5 acceptance matrix can settle it.

#### PC-5 Deletion and Authority Map

| Surface | Measured state | Disposition | Mutation class |
|---|---|---|---|
| `internal/computerversion/base_*_contract.go` | Deleted by seam-repair 2026-07-10 (7b59d33e); previously 38 production files with no non-test callers outside the closed cluster | Deleted. Do not resurrect; PC-5 integration matrix replaces self-validation. | black (done) |
| Paired `base_*_contract_test.go` files | Deleted by seam-repair 2026-07-10 with the unused builders | Deleted. | black (done) |
| `base_event.go`, `base_journal.go`, `base_tree.go`, `base_blob.go`, `base_current_state*.go`, `state_generator.go`, `tree_to_fs.go` | Concrete replay, integrity observation, and exact materialization | Retain as evidence/materialization substrate; bind only through the kernel's scoped tape/blob view. | no behavior change until kernel wiring |
| `planner.ApplyEvent` and its item-count-only test | No non-test caller; does not parse move/update payloads or track EventID | Delete after canonical replay is exposed to the kernel. | yellow with test-pressure change |
| Desktop SyncEngine (`applyDelta`, path-derived scanner, placeholder downloader, JSON cursor) and Wails SyncService | Deleted by seam-repair 2026-07-10; `internal/desktop` retains only client/apikey | Deleted from authority. Future desktop adapter must target the proved PC-5 kernel, not resurrect SyncEngine. | black (done) |
| Mirrored desktop REST request/response structs and API status semantics | A second schema/status authority before service ownership exists | Do not widen; delete or generate from the settled kernel contract during post-gate adapter work. | red when product API changes |

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
  Projection-triggered activation was deleted; the remaining typed ingestion
  handoff is the sole authoritative activation path
  (autopaper_authoritative_activation_paths: 1). Retry/overload idempotency for
  that handoff is still unproven, so duplicate processing under retry remains an
  open settlement gate.
validation_contradiction: >-
  The opt-in universal-wire staging spec rejects both source values it later
  branches on, then both requires and forbids story_texture_doc_id. The shared
  UI also contains duplicate `story_texture_doc_id || story_texture_doc_id`
  expressions left by a blind field rename. That spec is inadmissible as
  activation or publication evidence until repaired.
existing_replacement: >-
  BuildIngestionHandoff plus /internal/runtime/runs is already the typed,
  sourcecycled-tracked activation path. Projection should persist observations,
  not synthesize an untracked run.
open_definition_edges:
  - personal versus platform publication ownership
  - schedule ownership and per-computer configuration
  - edition/publication acceptance and retry identity
settlement_rule: >-
  Projection-triggered activation is already deleted. Prove exactly one run per
  handoff across retry/overload, preserve capture projection, then prove one
  deployed source cycle to canonical Texture/edition evidence before widening
  scope.
execution_effect: >-
  The single-activation repair may proceed without settling the wider product
  edges because duplicate processing is invalid under every admissible
  Autopaper definition. Repair the contradictory reader acceptance separately;
  it cannot substitute for cycle/request/run identity proof.
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
  The SyncService registration was deleted by seam-repair 2026-07-10; do not
  reintroduce it before PC-5 settles.
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
| Activate an approved ComputerVersion | live-app adoption; candidate-intake switch/rollback; optional Dolt tag adapter; vmctl desktop publish; actual proxy/VM route | one route-slot writer with activation/rollback receipt (RouteProfile format fixed to owner_id/computer_id by seam-repair 2026-07-10) | candidate mutation routes, tag/publish semantics, and active UI/persistence when no executor changed the served route | ordinary owner request resolves through the receipted version |
| Give the desktop a durable authenticated session | direct exchange-redirect flow; bridge flow; JavaScript cookies; native process; cloud and local proxy/auth stacks | one bridge/passkey flow and one native Go cookie jar behind the renderer proxy | direct exchange attempt, raw token responses, JS token/cookie handling, secret logs, production claims for dev-only local orchestration | built app survives reload and renewal; logout clears; JS/log secret scan is empty |
| Turn one source handoff into one processor run | typed ingestion handoff; non-idempotent retry (projection-triggered wire_synthesis deleted by prior guardrail run) | typed, durable, idempotent ingestion handoff and runtime admission | resurrected projection-triggered `wire_synthesis` and synthesis response fields | one cycle/request ID maps to exactly one processor run and publication lineage |
| Provide private exact-byte computer files | placeholder downloader; path-derived identity; random folder versions; deleted SyncEngine/contract-builder tower (seam-repair 2026-07-10) | stable item/device identity, content-addressed bytes, explicit conflict ancestry | zero-byte success, cursor advance across unresolved conflicts, resurrected SyncEngine/contract builders as presumed architecture | two-device rename/conflict/restart proof with exact hashes and owner denial |
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
    - claim: CLI timeout, promotion false-success, and Base data integrity remain reproducible code-path defects; Autopaper projection-triggered duplicate activation was deleted and only typed-handoff idempotency remains open.
      source: source and staging audit 2026-07-10; seam-repair 2026-07-10
    - claim: Base has exact blob, canonical replay, and exact materialization primitives, but no stable-identity and acknowledged-cursor transaction.
      source: Base source and caller audit 2026-07-10
    - claim: The current desktop Base path contradicts the kernel definition and its passing tests encode placeholder, path-identity, and cursor false success.
      source: Base source, test, and caller audit 2026-07-10
  open:
    - node: autopaper-single-activation
      missing: wider product ownership, schedule, and edition acceptance semantics
    - node: base-exact-byte-kernel
      missing: >-
        computer/owner-scoped stable identity, expected-parent observation
        commit, exact-byte acknowledgement, durable conflict/cursor state,
        ArtifactProgramRef binding, and a later deployed owner
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
  false_deploy_identity_paths: 0
  reachable_auth_boundary_failures: 1
  false_promotion_success_paths: 1
  cli_product_contract_failures: 2
  base_substrate_invariants_open: 5
  autopaper_authoritative_activation_paths: 1
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
- claim: Base's useful exact-byte substrate exists, but the product-facing path bypasses it and no stable-identity/acknowledgement kernel exists.
  definition_node: base-exact-byte-kernel
  evidence_class: read-only source/caller audit + focused unit-suite baseline
  command_or_observation: >-
    Inspected internal/base, internal/desktop, cmd/desktop,
    internal/computerversion, cmd/baseharness, cmd/baseobserve, cmd/basecompare,
    and cmd/evidenceroot; searched all production callers of Base builders,
    cursor methods, replay helpers, persistent route helpers, and exact
    materializers; ran go test ./internal/base/... ./internal/desktop/...
    ./internal/computerversion.
  result: >-
    All focused suites passed while TestSyncEngineDownloadCycle explicitly
    accepted a zero-byte placeholder and cursor advance. The audit found
    path-derived ItemIDs, random folder versions, desktop:.choir shared device
    identity, unscoped owner reads/writes, silent 64 MiB upload truncation,
    auto-parented linear history, full-cursor persistence across conflicts,
    non-durable/incorrect keep-both, and three event-fold paths. Exact
    blob.Store, tree.Derive, planner.Plan, StateGenerator, and TreeToFS are
    reusable primitives. The unused Base contract tower contains 38 production
    files, 39 builders, and 26,380 source-plus-test lines with no product caller.
  uncertainty: >-
    No stable local identity map, expected-parent transaction, owner/computer
    scoping decision, durable conflict/ack store, or ArtifactProgramRef resolver
    has been implemented or proven. Local fixture routes are not deployed
    service ownership.
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
- claim: deployed-origin acceptance pressures independent component identities back into one false global identity.
  definition_node: deployment-identity-follows-activation
  evidence_class: executable Playwright contract + differential deploy classifier
  command_or_observation: >-
    frontend/tests/deployed-origin-auth-shell.spec.js requires
    window.__CHOIR_BUILD__.commit to equal proxy /health build.commit, while
    .github/scripts/deploy-impact-classify permits frontend-only and
    service-only activation.
  result: >-
    The equality can only remain universally true if mutable deployment
    metadata overwrites compiled identity or unrelated components are
    needlessly redeployed. Both outcomes violate PC-0. Frontend and proxy must
    expose their own immutable identities; a deploy receipt must compare the
    workflow SHA only with components selected and activated by that run.
  uncertainty: >-
    The repaired deployed-origin test can prove identity presence and internal
    proxy header/body consistency, but selected-component equality belongs to
    the deploy workflow rather than a timeless cross-component browser test.
- claim: the sandbox runtime package inherits vmctl identity instead of carrying its own artifact identity.
  definition_node: deployment-identity-follows-activation
  evidence_class: source call graph
  command_or_observation: >-
    internal/vmctl/handlers.go passes buildinfo.Snapshot("vmctl") to
    writeRuntimePackageTar and projects that commit into choir-runtime.env as
    RUNTIME_WORKER_REPO_BASE_SHA.
  result: >-
    One artifact claims another process's identity. A vmctl rebuild can change
    the runtime-package commit even when the installed sandbox artifact did
    not change, and a sandbox-only package change has no independent manifest
    authority.
  uncertainty: >-
    This PC-0 repair records but does not yet replace the package contract.
    The follow-up must create a sandbox manifest at build/install time and have
    vmctl transport that manifest without re-authoring its identity.
- claim: one shared vendor checksum still materializes one vendor output per service derivation.
  definition_node: deployment-identity-follows-activation
  evidence_class: Nix derivation graph + independently generated vendor trees
  command_or_observation: >-
    Evaluate all nine Go service derivations and generate the filtered vendor
    tree for auth, sourcecycled, and sandbox.
  result: >-
    All filtered trees converge on
    sha256-JxOGfaZ3J71NVicFEhn1Vsgy5nOa1Sk74gQ0oroAhLA=, but buildGoModule
    creates a pname-specific go-modules derivation for each service. The
    identical vendor tree is about 354 MiB, or roughly 3.2 GiB across a cold
    nine-service build before Nix store optimization.
  uncertainty: >-
    This is a deployment cost, not an identity correctness blocker. A later
    build-graph slice should expose one shared vendored source or one
    multi-binary derivation without collapsing per-service activation and
    rollback pointers.
- claim: PC-0 is settled by exact-source build, selected-artifact verification, and an activation receipt.
  definition_node: deployment-identity-follows-activation
  evidence_class: full CI + exact-SHA deploy log + inverted in-progress probe + public staging acceptance
  command_or_observation: >-
    GitHub Actions run 29083767049 and deploy job 86334471531 for
    f2d1d330c532e164bd21cdc1c013fcbe370c5404; GET https://choir.news/health;
    GET https://choir.news/auth/session; inspect the served frontend asset.
  result: >-
    All standard and five race lanes passed. Node B fetched and reset to the
    workflow SHA, and the one Nix builder completed the selected closure in
    344 seconds. During activation, proxy already served compiled/header
    commit f2d1d330 while deployed_commit remained empty; it advanced only
    after verification. The durable receipt names ordinary_guest active,
    playwright_guest installed, sandbox and one active computer active, all
    seven host services active, and frontend asset index-CDCcbfLL.js active,
    every one at f2d1d330. Public proxy health then reported compiled commit,
    header commit, and deployed_commit all equal to f2d1d330; auth independently
    reported its service/header commit, and the served frontend asset embedded
    the same selected commit.
  verifier_contracts:
    - .github/scripts/deploy-workflow-contract-test
    - internal/buildinfo activation-receipt validation
    - internal/server immutable identity middleware
    - sandbox runtime build manifest and active-computer commit poll
  acceptance_level: deployed selected-artifact activation receipt plus public product identity proof
  accepted_deploy_id: "29083767049/1"
  rollback_refs:
    - /var/lib/go-choir/deploy-receipt-previous.json
    - /var/lib/go-choir/services/<service>-previous
    - /var/www/go-choir/frontend-previous
    - /nix/var/nix/profiles/system
  mutation_class: red
  protected_surfaces: [deployment routing, service identity, guest activation, run acceptance]
  heresy_delta:
    discovered: [moving-main checkout, dual builders, handwritten dependency closures, cross-component identity equality, inherited sandbox identity, nonfatal refresh]
    introduced: []
    repaired: [moving-main checkout, dual builders, handwritten dependency closures, cross-component identity equality, inherited sandbox identity, nonfatal refresh]
  conjecture_delta: >-
    Deployment truth is not one repository-wide SHA assertion. It is one
    immutable tested source commit projected into independently observable
    selected artifacts, followed by a receipt only after those artifacts meet
    their activation class.
  residual_risk: >-
    Identical 354 MiB vendor trees remain separately materialized per service;
    a later build-graph optimization must preserve the settled identity and
    per-service rollback boundaries.
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

Passing unit tests, an API record labeled adopted, a local `.app`, a resurrected
SyncService, or one published Texture is not completion.

## Run Checkpoint and Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: working
  last_checkpoint: seam-repair 944d4d94 activated on staging (CI 29091595925)
  current_artifact_state: >-
    Staging still carries f2d1d330 selected-artifact activation receipts. Local
    seam-repair commits af042d1e (service-scoped deployMetadata), 0b21ecdb
    (RouteProfile owner_id/computer_id), a1073731 (compiled-only source lineage),
    and 7b59d33e (SyncEngine/contract-builder/wire-synthesis dead-code excision)
    are not yet proven on staging. PC-0 remains testing until service-scoped
    identity is staging-proven. PC-4 is testing with the route-slot writer format
    fixed. PC-5/PC-6/PC-7 remain open; SyncService and the Base contract tower
    are deleted, not registered.
  what_shipped:
    - 3f4f4aac API-key capability-envelope enforcement
    - eb3bdd35 false deployment identity problem record
    - f2d1d330 exact-SHA single-builder selected-artifact activation receipts
    - af042d1e service-scoped deployMetadata (local; staging proof pending)
    - 0b21ecdb RouteProfile owner_id/computer_id writers + resolver legacy normalize (local)
    - a1073731 compiled-commit-only source-workspace identity (local)
    - 7b59d33e dead-code excision of SyncService/SyncEngine/contract tower/wire helper (local)
  what_was_proven:
    - CLI trajectories read works on staging
    - CLI timeout hides the server's bounded 504
    - current source contains reachable Wails secret-boundary failures (PC-2 still open)
    - Base exact-byte primitives exist, but stable identity plus expected-parent commit plus acknowledged cursor is the missing kernel transaction
    - SyncEngine, SyncService registration, and the Base contract-builder tower are deleted and no longer product authority
    - projection-triggered Autopaper activation is deleted; typed handoff is the sole activation path
    - proxy-global deployed_commit is not affected-service activation proof; deployMetadata is now service-scoped in code
    - API-key delegation and sibling revocation are bounded by caller capability on staging
    - compiled identity is visible before and independent from activation receipt metadata
    - every selected artifact in deploy 29083767049 has an explicit verified receipt entry
  unproven_or_partial_claims:
    - no staging PromoteAppAdoption/RollbackAppAdoption mutation (no packages)
    - no Wails built-app acceptance
    - no deployed Autopaper typed-handoff idempotency proof
    - no Base exact-byte two-device proof
    - no served ComputerVersion promotion end-to-end
  highest_impact_remaining_uncertainty: staging proof of service-scoped identity and route-slot promotion after seam-repair
  next_executable_probe: >-
    Land seam-repair through CI/deploy, then prove per-service /health identity
    and a RouteProfile promotion/rollback on choir.news. Resume PC-2/PC-3/PC-5
    product work only after those receipts exist.
  suggested_goal_string: "/goal docs/definitions/choir-seam-repair-2026-07-10.md"
  evidence_artifact_refs:
    - this Definition's evidence ledger
    - docs/definitions/choir-seam-repair-2026-07-10.md
    - docs/definitions/og-dolt-heresy-completion-2026-07-08.md
  rollback_refs:
    - 224243de (pre-program source state)
    - b7f689d4 (pre-API-key behavior repair)
    - f2d1d330 deployment receipt rollback paths named in the PC-0 evidence entry
    - a1073731 (pre-delete rollback for Phase D dead-code excision)
```
