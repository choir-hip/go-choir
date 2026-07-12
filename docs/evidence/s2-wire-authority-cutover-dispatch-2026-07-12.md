# S2 Wire Authority Cutover Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Subgoal: `S2`
- Dispatch nonce: `s2-wire-authority-cutover-01-nonce-01`
- Canonical parent: `d4bcf26f55d9c8f5acb43ed00ab3ec74df48e591`
- Mutation class: red

## Problem Evidence

The S2 authority audit found a live split-brain world-wire implementation:

1. `internal/runtime/universal_wire.go` serves `/api/universal-wire/stories` from VM-local Texture documents and an edition alias, while corpusd already owns published public objects.
2. `internal/runtime/wire_publication.go` publishes the article to corpusd and then creates and advances a second VM-local `Wire.texture` edition as feed authority.
3. `cmd/sourcecycled/main.go` projects durable web captures through `/internal/runtime/objectgraph/web-captures`, forcing the shared world-wire write through a user-computer runtime even though corpusd already exposes canonical object-graph writes.
4. `internal/store/migration.go` and sandbox startup still replay retired relational runtime state into the VM-local object graph on every computer boot.
5. The proxy already forwards canonical publication writes and corpusd already owns publication, route, revision, blob, provenance, retrieval, and object-graph APIs. Connecting those existing capabilities is cheaper and safer than extending the superseded runtime paths.

This document records the reliable authority failure before any S2 behavior-changing fix, satisfying Doctrine I11.

## Classification

- Substrate: authority routing and persistence topology.
- Protected surfaces: corpusd canonical world-wire writes, public publication reads, VM lifecycle, runtime-to-host proxy routes, ingestion publication, staging deployment identity.
- Conjecture delta: no architecture pivot. This repairs the implementation to the already-settled two-store topology: corpusd owns durable shared world-wire state; each user computer owns only private working state.
- Heresy delta at dispatch: `discovered=[runtime_world_wire_read_authority, runtime_local_edition_write_authority, runtime_mediated_shared_capture_write, boot_time_retired_sql_replay]`, `introduced=[]`, `repaired=[]`.
- Admissible acceptance evidence: focused behavior tests, ratchet proof, green CI/deploy at the landed SHA, staging build identity, and deployed product-path proof showing wire publication/read and source ingestion without VM boot or VM-local wire persistence.
- Rollback: revert the landed S2 commit before accepting any new publications. Do not restore dual-read or dual-write fallback.

## Fresh Authority And Caller Inventory

### Existing canonical host capabilities

- `cmd/corpusd/main.go` registers `internal/platform` publication and object-graph services on the platform Dolt store.
- `internal/platform/handlers.go` exposes canonical publication, resolution, retrieval, Texture revision, and sync routes.
- `internal/platform/objectgraph_handlers.go` exposes canonical object/edge writes and reads.
- `internal/proxy/wire_platform_publish.go` validates autonomous publication and forwards it to corpusd.
- `internal/proxy/platform_objectgraph.go` already forwards internal object-graph operations to corpusd.

### Superseded runtime-local authority

- `internal/runtime/universal_wire.go`: runtime-local story read model and `/api/universal-wire/stories` registration.
- `internal/runtime/wire_publication.go`: VM-local edition bootstrap/advance and edition settlement reference.
- `internal/runtime/objectgraph_runtime.go`: shared capture publication handler inside a user computer.
- `internal/store/migration.go`, migration tests, `OpenOptions.DeferObjectGraphBackfill`, `BackfillObjectGraph*`, and `cmd/sandbox` startup loop: boot-time retired relational replay.

## Atomic Mutation Slices

### S2-A — Delete boot-time retired SQL replay

Allowed paths: `internal/store/migration.go`, `internal/store/migration_test.go`, `internal/store/store.go`, directly dependent `internal/store/*_test.go`, `cmd/sandbox/main.go`, `cmd/sandbox/main_test.go`, and the runtime ratchet inventory.

Change: remove the migration implementation, completion/cursor tables and APIs, deferred-open option, sandbox background replay loop, and tests that normalize replay. Preserve normal schema initialization and VM-local private store opening. No migration shim, feature flag, or compatibility alias.

Acceptance: store and sandbox focused tests pass; a fresh sandbox opens without invoking any replay API; ratchet records fewer production files/LOC and no new compatibility marker.

### S2-B — Make corpusd the only public wire read and edition authority

Allowed paths: `internal/platform/**`, `internal/proxy/**`, `internal/runtime/universal_wire.go`, `internal/runtime/wire_publication.go`, their focused tests, route/config registries, and runtime ratchet inventory.

Change: expose a corpusd-backed story list from canonical published objects; proxy the existing `/api/universal-wire/stories` product route directly to corpusd; remove runtime registration and local read helpers; remove VM-local `Wire.texture` bootstrap/advance; settle publication from the corpusd publication/route receipt. Keep private article drafting in the user computer only until canonical publication. No dual read, dual write, backfill, or fallback.

Acceptance: proxy/corpusd tests prove response shape and ordering from canonical corpusd objects; runtime tests prove publication no longer creates/reads a local edition; old runtime story route is absent; ratchet passes.

### S2-C — Publish source captures directly to corpusd

Allowed paths: `cmd/sourcecycled/**`, `internal/cycle/web_capture_graph.go`, `internal/proxy/platform_objectgraph.go`, `internal/platform/objectgraph_handlers.go`, deployment configuration directly required to provide the host endpoint, focused tests, and runtime ratchet inventory.

Change: replace the sourcecycled runtime projection target with the existing corpusd object-graph HTTP service through the host service/proxy path; remove `/internal/runtime/objectgraph/web-captures` and its runtime implementation/tests after the caller cutover. Preserve ingestion processor activation as a separate runtime task; only durable shared capture publication moves. No VM boot, local fallback, direct third store, or dual path.

Acceptance: sourcecycled tests prove canonical object/edge publication to the host path and no runtime projection request; runtime route is absent; deployment provides the host endpoint; ratchet passes.

## Dependencies And Landing Rule

S2-A is independent. S2-B and S2-C share the canonical object/publication authority edge and must be integrated in one landed S2 commit so no deployed state has mixed authority. Implementation agents may work in isolated branches, but the orchestrator performs conflict resolution, full focused verification, one final authority audit, then lands atomically.

The S2 phase does not pass until independent verification and a post-implementation consensus panel find no remaining runtime-local shared wire read/write/migration authority, and deployed staging proves publication, feed read, source capture persistence, VM stop/start fate-sharing, and feed visibility across restart.

## S2-VER-001 — Retained VM-Local Edition Read Gate

At canonical `97dc05f7`, independent verifier `S2IndependentVerifier` found a blocking retained authority in `internal/runtime/universal_wire.go:40-89`. Although S2-B deleted local edition advancement, `resolveUniversalWireTextureReadOwner` still authorized cross-owner Texture document and revision reads only when the platform document appeared in the VM-local `universal-wire/Wire.texture` alias and current revision. New corpusd publications can never satisfy that stale gate, so runtime Texture reads return not-found and the VM-local store remains shared-wire read authority.

Classification: substrate authority-routing regression. The existing canonical public publication route is proxy/corpusd; runtime Texture endpoints are private working-state surfaces. The repair is therefore deletion-first: remove the cross-owner runtime read exception and edition parser/gate, make runtime Texture reads owner-scoped only, and leave public article/feed reads exclusively on proxy/corpusd. Do not add a corpusd fallback inside runtime Texture handlers.

### S2-VER-001 Repair Receipt

Commit `08803bb2` deletes `resolveUniversalWireTextureReadOwner`, `universalWireEditionIncludesDoc`, the local edition parser and source-path constant, and all cross-owner fallbacks from runtime Texture document, revision, revision-list, history, and stream handlers. Runtime Texture reads now use only the authenticated owner's VM-local private store. `TestRuntimeTextureReadsRemainOwnerScopedAfterWireCutover` seeds a platform-owned local document/revision and proves every retained runtime Texture read surface returns not-found to another owner. Canonical public article and feed reads remain exclusively on proxy/corpusd.

Focused proof passed:

- `go test ./internal/runtime -run 'Test(RuntimeTextureReadsRemainOwnerScopedAfterWireCutover|RuntimeDoesNotRegisterUniversalWireStories|WirePublicationSettlesFromCorpusdReceiptWithoutLocalEdition|WirePublicationDoesNotBootstrapLocalEdition)$' -count=1`
- `go run ./cmd/runtime-ratchet`
- `go test ./cmd/runtime-ratchet`

## S2-D — Product VM Stop/Observe/Start Control

The S2 completion contract requires the feed to be read while the platform VM is stopped through a product/vmctl surface. The deployed product already exposes authenticated `/api/compute/status` and wake/recovery, and vmctl already exposes owner-scoped `StopDesktop`; the missing connection is a stop action and CLI commands. This is a bounded red extension authorized by the S2 Definition itself, not a new lifecycle state machine.

Allowed paths: `internal/proxy/compute_status.go`, `internal/proxy/handlers_test.go` or focused compute tests, `cmd/choir/main.go`, `cmd/choir/main_test.go`, and the runtime ratchet baseline if mechanically required.

Change: add `stop_current_computer` to the existing authenticated compute recovery action switch, call the existing owner-scoped vmctl `StopDesktop`, and return the stopped computer receipt. Add `choir computer status|stop|start` over the existing compute status/recovery routes. `start` uses the existing `wake_current_computer` action. No runtime API, new state machine, raw internal route exposure, SSH, or process control.

Acceptance: focused proxy and CLI tests prove API-key owner-scoped status/stop/start request shapes; deployed CLI stops the current platform computer, status observes it stopped, `choir wire stories` returns the same story while stopped, and start restores the computer.

### S2-D Deployed Acceptance Receipt

Commit `7fa4e62f0182fde1931821dc17735760adbc3aac` implements the bounded lifecycle surface. `TestComputeRecoveryStopUsesOwnerScopedVMCTL` proves that the authenticated owner reaches vmctl `StopDesktop` and that both the product receipt and ownership lookup report `stopped`. `TestComputerLifecycleCommandsUseProductComputeAPI` proves the CLI request shapes:

- `choir computer status` -> `GET /api/compute/status`;
- `choir computer stop` -> `POST /api/compute/recovery` with `stop_current_computer`;
- `choir computer start` -> the same product route with `wake_current_computer`.

Focused package proof passed with `go test ./internal/proxy ./cmd/choir -count=1`. The final runtime ratchet and its tests passed after recording this evidence document's new runtime citer.

GitHub Actions run `29185043037` passed and deployed the same full SHA. At `2026-07-12T08:18:49Z`, `https://choir.news/health` reported `7fa4e62f0182fde1931821dc17735760adbc3aac`. The deployed product CLI then proved:

- initial current-computer state `active`;
- stop receipt state `stopped`;
- independent `choir computer status` observation state `stopped`;
- Wire story count `100` before and while stopped;
- first story ID `source-network-texture-202ba0b3-48b9-40fb-8ab5-40dd190c8155` unchanged while stopped;
- start receipt action `wake_current_computer`;
- final independently observed current-computer state `active`.

Independent reviewer `S2LifecycleVerifier` returned PASS at confidence `0.94`: owner identity comes only from authenticated `AuthResult.UserID`; status requires `read:runtime`; stop/start require `write:runtime`; no Wire, VM-local runtime, SSH, raw internal route, fallback, or migration coupling was introduced. The new proxy test uses cookie authentication rather than an API key, but the pre-existing API-key scope test and CLI Bearer transport independently cover those paths; the reviewer classified this as non-blocking test coverage, not a defect.

The owner imported one `choir.news` session cookie into the isolated QA browser through the explicit cookie picker. The reloaded desktop was authenticated: it opened with zero windows rather than the signed-out Choir Preview. Opening the Universal Wire desktop icon rendered a real application window with `100 articles`; the first article was `Hackers Used Meta’s AI Support Bot to Seize Instagram Accounts`, matching the stopped-VM CLI story, and its visible provenance stated `This Texture article is published in the canonical corpusd world-wire store.` and `Source provenance is carried by the canonical publication object graph.` The authenticated human-browser article-render criterion is therefore satisfied without exposing cookie values.