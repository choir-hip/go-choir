# Overnight Mission Suite Adversarial Review - Commits After 4857205

Date: 2026-06-28  
Baseline: `4857205c` (`docs: vision - the portfolio of perspectives`)  
Reviewed range: `4857205..580d3bf4` on `main`  
Head at review start: `580d3bf4` (`docs: fix checkpoint report and ledger counting errors from review`)  
Mutation class of this document: green  
Review mode: adversarial, read-only, per-commit Codex thread team plus leader synthesis

## Executive Verdict

The overnight mission suite produced real substrate progress, but it is not safe to treat the suite as fully settled. The work added useful primitives across actor runtime, Base, trace, PII, health, auth, desktop sync, File Provider, CI, and mission accounting. However, several mission claims overstate product-path proof, and multiple protected surfaces remain incomplete or unsafe in current composition.

The highest-risk unresolved issues are:

1. Auth recovery is unsafe: the recovery request endpoint returns the raw recovery token to unauthenticated callers, enabling account takeover for any known email.
2. API keys are unsafe/incomplete in the reviewed M1 commit: management routes were initially unwired, bearer secrets can be forwarded upstream, scopes are not enforced, and deployed proxy auth DB configuration was not proven.
3. Trace persistence and PII redaction are not composed: M20/M20b mounts a raw `trace.NewDoltStore`, while M21/M21b adds a `RedactingStore` wrapper that production code does not instantiate.
4. Actor mailbox durability/liveness remains suspect: channel-delivered updates can mutate memory while durable acknowledgement/snapshot failures are ignored, and later backpressure proof does not reproduce under `-race`.
5. Base ownership/provenance and conflict semantics are insufficient: Base APIs accept caller-supplied owner IDs, planner path-collision conflicts are one-sided, and desktop sync can advance cursors despite unresolved conflicts.
6. Public health routes are not externally observable at `choir.news`, and their error payload path can leak raw checker errors despite no-secret claims.
7. Mission/checkpoint accounting repeatedly drifted: V counts, mainlined counts, PR mappings, "all 24" scope, and live conjecture lists frequently diverged from the actual mission graph.
8. CI/Nix/SBOM work eventually converged for Go services at `0fde3628`, but the path was a brittle manual hash-chasing loop with weak SBOM artifact completeness diagnostics.

The suite should be considered a high-volume discovery/implementation pass, not a cleanly accepted platform settlement. Several commits are solid or mostly solid, but the aggregate state needs substrate-level follow-up before further mission claims rely on it.

## Methodology

The leader created a durable Codex team:

- Team id: `019f0fdb-9051-7801-bcb1-34fd44215a4a`
- Team state: `.omo/teams/019f0fdb-9051-7801-bcb1-34fd44215a4a/`
- Commit manifest: `.omo/teams/019f0fdb-9051-7801-bcb1-34fd44215a4a/artifacts/commit-manifest.tsv`
- Members: `C01` through `C69`, one member per commit after `4857205`

Each member thread was instructed to work read-only, inspect its assigned commit in context, compare adjacent duplicate/merge-lineage commits where relevant, and report:

- commit intent;
- changed files;
- what works;
- what does not;
- bugs/risks with file:line evidence;
- missing proof/tests;
- opportunities and EXPAND leads;
- final severity summary.

The leader additionally reviewed cross-commit patterns and incorporated late thread updates. Some late-running threads were still completing long verification commands during synthesis; their available findings are included and marked as preliminary where no final report had landed yet.

## Severity Key

- P0: exploit/security/data-loss/blocker.
- P1: high product correctness, protected-surface, privacy, auth, or deployment risk.
- P2: material proof, orchestration, test, or process defect.
- P3: hygiene, docs precision, or lower-risk maintainability issue.

## Critical Cross-Commit Findings

### 1. Auth Recovery Account Takeover Risk

Commits: `1734ec06`, `901039b4` (C43/C44)

M7 added email recovery and session/passkey management, but `HandleRecoveryRequest` returns the raw recovery token in the unauthenticated JSON response. The implementation comment says production should send the token by email and omit it from the response, but the route is wired into `cmd/auth/main.go`, and tests assert the token is returned.

Impact: anyone who knows a user email can request recovery, receive the token, verify it, register their own authenticator, and obtain cookies.

Additional auth gaps:

- current-session revocation can be bypassed when only an access cookie is present;
- general "10 per IP per minute" auth endpoint rate limiting from the M7 spec was not found;
- recovery IP limiting trusts caller-supplied `X-Forwarded-For`;
- legal docs still claim no password/recovery flow after M7 added self-serve recovery.

Required action: remove token from public recovery responses, enforce email delivery only, add route-level black-box tests, add global auth rate limiting, fix session revocation guard, and update legal docs.

### 2. API Key System Is Not Securely Accepted

Commits: `d0f82ad0` (C28), related to C06/M1 design

The API-key primitives are useful, but the reviewed M1 implementation is not product-safe:

- `/auth/api-keys` handlers existed but were not registered in `cmd/auth/main.go` at the assigned commit;
- proxy forwards raw `Authorization: Bearer choir_sk_*` headers upstream on generic protected routes;
- `PROXY_AUTH_DB_PATH` was required but not proven wired in deployment config;
- scopes are validated and propagated, but not enforced before protected proxy routes;
- any valid key can reach protected endpoints once bearer auth is active.

Required action: register key routes through black-box tests, strip `Authorization` and cookies before upstream forwarding, wire proxy auth DB in deploy config, and enforce route/method scope checks centrally.

### 3. Trace Persistence Bypasses PII Redaction

Commits: `b399bd2a`, `fc337c2a`, `9e976da6`, `6266a812`, `e5840c43`, `f89a8db8` (C30/C31/C35/C38/C39/C40)

The suite split trace persistence and PII redaction into separate missions, then failed to compose them in production:

- M20 added a raw trace SQL/Dolt store that persists arbitrary event payload JSON.
- M21 added PII pipeline libraries.
- M21b added `trace.NewRedactingStore` and unit tests.
- M20b mounted `trace.NewDoltStore(db.DB())` directly in `cmd/sandbox/main.go` and passed it to runtime via `actorruntime.WithTraceStore`.
- `NewRedactingStore`/`NewPipelineFromConfig` are used only in tests/docs, not production wiring.

Additional trace concerns:

- trace HTTP handler owner checks are not storage-bound; list-by-run verifies only the first event owner;
- parent-chain fetch can include cross-owner parents;
- direct runtime `AppendEvent` style paths may bypass the trace store projection;
- redaction failure fallback stores original payload, explicitly allowing raw PII persistence on redactor error.

Required action: wrap the production trace store with `NewRedactingStore`, make redaction fail closed or quarantine raw payloads outside the primary store, add runtime-level tests that persisted `trace_events` payloads are redacted, and owner-scope trace queries at the storage boundary.

### 4. Actor Mailbox Durability And Backpressure Are Not Settled

Commits: `676b2032`, `2748dea9`, `2161b2e3` (C01/C54/C55)

The H030 mailbox direction is correct, but durability/liveness proof is incomplete:

- `processOne` applies handler effects, ignores `MarkProcessed` errors, and still adds the update ID to the skip set. A successful handler can be snapshotted while the durable log still says unprocessed, enabling replay/double-execution on a later cold start.
- passivation ignores `SaveSnapshot` errors after updates may already be marked processed.
- M23 bounded inbox/backpressure appends to the durable log before backpressure checks; `ErrInboxFull` can occur after the update is already logged.
- backpressure is opt-in; no product-path `WithBackpressure` caller was found outside tests/docs.
- `nix develop -c go test -race ./internal/actor -run TestNoLostWakeUnderConcurrentSendsAndPassivations -count=1 -v` failed in C55 review with timeout, while non-race targeted test passed.

Required action: define the atomic contract between handler effects, `MarkProcessed`, and snapshots; add fault-injection tests for `MarkProcessed` and `SaveSnapshot`; wire and prove product-path backpressure or narrow the mission claim to library support.

### 5. Base APIs And Desktop Sync Break Ownership/Conflict Contracts

Commits: `c26a0f4c`, `7b600d13`, `b6a37a61`, `cbfa1489`, `1616e37c` (C27/C48/C49/C52/C53)

Base made substantial progress, but the API/sync composition is not safe:

- Base API accepts request `owner_id` and defaults only if empty; handlers do not bind all reads/writes to authenticated `ar.UserID`.
- `handleGetItem`, `handleGetStatus`, and `handleDelta` derive from all journal entries and ignore authenticated user for scoping.
- API can create versions referencing missing blobs; no `h.blobs.Has(req.BlobRef)` check exists.
- blob REST surface is upload-only; remote downloads write empty placeholders because no blob GET endpoint exists.
- oversized blob upload uses `io.LimitReader` and can silently truncate at 64 MiB.
- planner treats identical content with different non-empty VersionIDs as conflicts because content-addressed equality only applies when one VersionID is empty.
- path-collision conflict records only one structured `ItemID`, so downstream conflict resolution can address the wrong participant.
- desktop sync says unresolved conflicts should not advance, but persisted state writes `delta.Cursor` unconditionally.
- Wails service was registered, but no frontend caller/UI was found at the commit.

Required action: enforce owner scoping at Base API boundaries, add blob existence and max-size rejection, add blob GET or narrow sync claims, redesign conflict participant identity, and prevent cursor advancement until unresolved conflicts are resolved.

### 6. Health Endpoints Are Not Externally Proven And Can Leak Error Details

Commits: `1d1779a0`, `12508a18`, `5eb2ffdc` (C32/C41/C42)

Health/circuit-breaker primitives are useful but not accepted through the product surface:

- M22 initially configured gateway readiness with no checkers, so `/health/ready` could report ok with zero dependencies.
- M22b later wired checkers, but public staging probes to `https://choir.news/health/ready`, `/health/qdrant`, and `/health/ollama` returned the frontend HTML shell, not gateway JSON.
- node-b public edge proxies exact `/health`, not the service-specific gateway routes.
- `HandleServiceHealth` returns `err.Error()` truncated to 200 chars; the no-secrets test injects a secret but never asserts absence.
- `/health/ready` can return 200 for degraded status.

Required action: sanitize service-health errors, route/proxy the intended public health endpoints or explicitly classify them internal-only, and add deployed acceptance that curls the actual edge paths and validates JSON.

### 7. vmctl Race Repair Is Incomplete

Commits: `fccf3db7`, `8f83d4a5` (C45/C46)

The targeted idle-sweeper getter race is fixed by returning snapshots, and focused `-race` tests passed. However:

- `LiveSandboxURL` still captures `own := r.ownerships[...]`, releases the lock, then reads `own.VMID`/`own.SandboxURL`.
- protected lifecycle paths mutate those fields under `r.mu`.
- `HandleSandboxProxy` reaches `LiveSandboxURL`.
- other handler-facing APIs still return live pointers after locks are released.

Required action: convert all external registry read surfaces to snapshots/DTOs under lock and add focused race tests overlapping sandbox-proxy URL resolution with refresh/resume.

### 8. File Provider Work Is Component-Level, Not Product-Ready

Commits: `96558859`, `f8638483` (C59/C60)

The Go bridge tests pass, but the macOS File Provider integration has product-blocking issues:

- Swift Codable structs expect camelCase while Go bridge JSON uses snake_case; reads/writes/moves/status decode or encode wrong fields.
- Swift `BridgeClient` returns response bodies before checking HTTP status; mutating calls can look successful on bridge errors.
- `URLProtocol` transport registration discards the configured socket path; URLSession instantiates protocol classes without the custom initializer.
- direct Swift type-check reported compile errors in `ChoirFileProviderBridge.swift` around `protocolClasses` and the untyped nil guard.
- host app does not register/package the File Provider domain; registration is only a README snippet.
- entitlement/app-group story does not match the socket path; code uses ordinary `~/Library/Application Support/Choir/fileprovider.sock`.
- no Finder/manual macOS proof exists; local Xcode was unavailable/broken for compile proof.

Required action: align JSON key strategies, fix HTTP status handling, move socket into app-group container, register/package the extension from the host app, and require signed Finder proof.

### 9. SBOM/Nix VendorHash Cluster Eventually Converged, But The Substrate Is Brittle

Commits: `23703040`, `b6c64605`, `0a8eb553`, `6b3cc03d`, `05a42f25`, `39348558`, `ae2c4f16`, `20236d85`, `db5f52c2`, `e6777ba3`, `2c47751d`, `0fde3628` (C05/C20/C22/C23/C24/C58/C61-C67)

This was a root-cause cluster, not isolated fixes:

- initial SBOM integration used invalid sbomnix flags;
- flake SBOM derivations attempted nested `nix build` from inside a builder sandbox;
- CI allowed zero SBOM artifacts while reporting success in one repair;
- stderr was captured into `store_path`, causing "file name too long";
- later log truncation hid package failure causes;
- vendor hashes were chased service-by-service;
- `internalDirs` source filters repeatedly missed transitive internal package dependencies;
- C67/`0fde3628` converged Go-service hashes and passed CI/deploy; that later green run is the acceptance proof for intermediate auth/proxy/internalDirs fixes. `obscura` still skipped SBOM under warning semantics, and diagnostics regressed from full logs to `head -20`.

Required action: replace manual per-service hash chasing with a CI gate that builds every service derivation before deploy, validates all required SBOM artifacts, emits a skipped-package manifest, and fails when required packages are missing. Decide whether SBOM is a hard supply-chain gate or a best-effort audit artifact.

### 10. Mission Control Plane Drifted Repeatedly

Commits: `7b62a615`, `cd5e2242`, `573ad45d`, `aefcad6e`, `7adca1e6`, `2aa3938b`, `9e4ea17b`, `8c796f6f`, `66ef1077`, `80674c3e`, `580d3bf4` (C07-C12/C19/C47/C57/C68)

The mission docs were useful but not reliable as executable control state:

- source mission suite claimed "all open loops" but omitted P0 "Snapshot save race window";
- M20 trace persistence and M21 PII redaction were listed as independent despite "redact before persistence";
- early paradocs allowed orange work to settle with commit/push/CI but no staging acceptance proof;
- "all 24" claims did not enumerate all 24 as live conjectures;
- V values and live conjecture lists diverged;
- Pass 2 swapped M21/M22 PR mappings;
- Pass 3/4/5 mainlined counts did not match their own mission lists;
- Pass 4 V trajectory mixed total open variants with new-only launch variants;
- settlement docs cited test counts and CI claims without durable run URLs/log artifacts.

`580d3bf4` repairs some Pass 5 counting errors and the iCloud PDF appears regenerated, but earlier ledger rows still preserve incorrect mappings/counts.

Required action: generate checkpoint/ledger state from structured mission data, add doccheck rules for V/list/count/PR mapping consistency, and require durable evidence refs per mission verdict.

## Positive Outcomes Worth Preserving

- Actor runtime direction shifted away from database polling toward resident mailbox delivery.
- Base model/planner/journal/API/desktop sync primitives now exist, even if ownership/conflict contracts need repair.
- Trace store, redaction pipeline, and runtime wiring pieces exist; the missing part is composition.
- Health/circuit-breaker primitives exist and tests cover many local paths.
- Race fixes for proxy/server and gateway mock provider are supported by focused race tests.
- Node A wrong offsite-replica shape was repaired by restoring real Node A mirror config; live `choir-ip.com/health` reported the repair SHA during review.
- Final Go service vendorHash cluster converged at `0fde3628` and deployed to staging.
- The review process itself found and partially repaired checkpoint/ledger counting errors in `580d3bf4`.

## Per-Commit Appendix

| ID | Commit | Review Status | Finding Summary |
| --- | --- | --- | --- |
| C01 | `676b2032` actor H030 mailbox | Final | Correct direction, but P1 durability bug: `MarkProcessed` errors ignored after memory mutation; snapshot save errors ignored. |
| C02 | `caac2f2c` actorruntime reactivation | Final | Code likely fixes active-state drop, but tests synthesize actor updates and do not prove durable worker-update delivery/marking. |
| C03 | `7c2b0259` frontend auth problem memo | Final | Accurate problem diagnosis; docs-only commit overclaims "repaired" heresy and lacks full red-class memo fields. |
| C04 | `6286f89f` frontend transient auth retry | Final | Fixes session-check ingress but `fetchWithRenewal` can still return original 401 after transient renewal failure, causing logout through callers. |
| C05 | `23703040` SBOM integration | Final | Initial SBOM job not deployable: invalid sbomnix flags and nested Nix sandbox path. |
| C06 | `86d2c8dd` artifact program/headless auth docs | Final | API-key authorship design cannot produce promised key identity; doctrine/source-of-truth and EventID ambiguity. |
| C07 | `7b62a615` mission suite | Final | Dropped P0 snapshot race; mis-modeled trace persistence and PII redaction as independent. |
| C08 | `cd5e2242` initial orchestrator paradoc | Final | Orange missions could settle without staging acceptance; dependency inconsistency between M1/M2. |
| C09 | `573ad45d` V=10 expansion | Final | Stale live state (`V=4`), omitted protected-surface ceremony, omitted P0 snapshot race, scheduled M20 without M21. |
| C10 | `aefcad6e` all-24 ambition | Final | "All 24" not represented; stale V/goal text; M20/M21 privacy ordering propagated. |
| C11 | `7adca1e6` v3 worktree model | Final | V=12/all-24 scope mismatch; evidence schema weakened for orange/red settlements. |
| C12 | `2aa3938b` Pass 1 launched | Final | Launch claims lack durable subagent receipts; next-action checklist understates CI/staging gates. |
| C13 | `9715eff5` flaky test quarantine | Final | Narrow quarantine but executable coverage lost for StartCoagentRun slot-reuse contract. |
| C14 | `b3308e19` legal docs | Final | Not user-reachable, includes internal mission text, overclaims retention/erasure, later auth recovery drift. |
| C15 | `1dfab8b0` mission graph triage | Final | Successor links are comments/prose only; live graph edges still depend on superseded nodes. |
| C16 | `a14c39db` M12 merge | Final | Merge mechanics clean; inherits C13 coverage/proof debt. |
| C17 | `7b49b43c` M13 legal merge | Final | Clean merge; legal docs still not production-ready or exposed. |
| C18 | `651fd854` M19 graph merge | Final | Clean merge; graph semantics remain machine-unsafe around superseded nodes. |
| C19 | `9e4ea17b` Pass 2 settlement | Final | M21/M22 PR numbers swapped; durable evidence refs weak. |
| C20 | `b6c64605` CI/cache/SBOM/Node A config | Final | Introduced flake break by referencing missing Node A files; SBOM repair incomplete. |
| C21 | `fc6f2e77` wrong Node A offsite replica | Final | Critical wrong ontology: assumed Node A did not exist and added SSH-only offsite host. |
| C22 | `0a8eb553` direct sbomnix CI | Final | Removed nested-sandbox failure but exact CI still failed on `obscura`; unpinned sbomnix source. |
| C23 | `6b3cc03d` SBOM resilience | Final | Captured stderr into store path; CI could succeed with zero SBOM artifacts. |
| C24 | `05a42f25` SBOM stderr separation | Final | Store-path bug fixed; diagnostics truncated and CI green with missing `obscura` SBOM. |
| C25 | `fb2b54aa` Node A real config repair | Final | Restored real Node A; live health proved SHA, but all-service/toplevel proof incomplete and CI canceled. |
| C26 | `048abfdf` doccheck cleanup | Final | Exact commit still had 117 warnings; broadened detector allowances and historicalized active mission. |
| C27 | `c26a0f4c` Base planner | Final | Content-address equality bypassed; path-collision conflict lacks both participant IDs; ApplyEvent contract false. |
| C28 | `d0f82ad0` API keys | Final | Critical: routes unwired, bearer secret forwarded upstream, proxy DB not proven configured, scopes not enforced. |
| C29 | `cdd61309` LLM cost | Final | Time windows filter after limit; pricing table stale/incomplete/mis-keyed by model only. |
| C30 | `b399bd2a` trace store | Final | Owner-scope leaks in run/parent-chain queries; raw payload persistence; no product wiring in that commit. |
| C31 | `fc337c2a` PII library | Final | Library only; current product still bypasses redaction; redaction errors can store raw payload. |
| C32 | `1d1779a0` health/circuit breakers | Final | Readiness had no dependency checkers until M22b; route-level breaker behavior proof incomplete; gateway OpenTimeout literals remain. |
| C33 | `f98afaa3` proxy/server races | Final | Synchronization looks correct; proof relies on `-race`, later CI race gate not aggregate-required. |
| C34 | `03ba57bd` race merge | Final | Merge-only of C33; focused race/vet tests pass; super-console route direct test gap remains. |
| C35 | `9e976da6` PII redaction wiring | Final | RedactingStore added but not mounted in production; raw trace store remains. |
| C36 | `620c0f51` Base journal/tree | Final | SQLite append not SQL-transaction serialized across handles/processes; first event can carry non-empty parent and later fail verification. |
| C37 | `90f06fbb` Base journal merge | Final | Merge duplicate of C36; inherits journal durability/replay risks. |
| C38 | `6266a812` PII redaction merge | Final | Same as C35: wrapper exists, production mounts raw store. |
| C39 | `e5840c43` trace runtime wiring | Final | Mounted raw store despite RedactingStore already existing; several direct `AppendEvent` production paths still bypass the trace store. |
| C40 | `f89a8db8` trace wiring merge | Final | Runtime trace tests pass; production still raw/unredacted; no deployed trace persistence proof. |
| C41 | `12508a18` gateway health wiring | Final | Public service-health can leak raw checker errors; deployed public paths not proven. |
| C42 | `5eb2ffdc` gateway health merge | Final | Merge-only of C41; public staging `/health/ready` and `/health/{service}` returned frontend HTML. |
| C43 | `1734ec06` auth recovery feature | Final | P0 raw recovery token in response; session/rate-limit/legal drift gaps. |
| C44 | `901039b4` auth recovery merge | Final | Same tree as C43; confirms P0 token exposure, active-session expiry gap, missing general auth limiter. |
| C45 | `fccf3db7` vmctl ownership snapshots | Final | Target race fixed, but `LiveSandboxURL` and other pointer-return surfaces retain old pattern. |
| C46 | `8f83d4a5` vmctl merge | Final | Merge-only of C45; focused `-race` passes; residual `LiveSandboxURL` risk. |
| C47 | `8c796f6f` Pass 3 docs | Final | Mainlined counts inconsistent; Pass 2 PR mapping error persists; proof refs weak. |
| C48 | `7b600d13` Base blob/API | Final | Owner isolation broken; missing blob existence check; no blob GET; oversized uploads truncate. |
| C49 | `b6a37a61` Base blob/API merge | Final | Merge duplicate of C48; confirms no product route mount plus Base owner/provenance isolation gaps. |
| C50 | `148135f9` gateway mock race | Final | Supported test-only fix; focused `-race` passes; C51 is merge duplicate. |
| C51 | `922f0dfe` gateway mock race merge | Final | Merge-only of C50; focused and full gateway `-race` tests passed, no blocking issue found. |
| C52 | `cbfa1489` desktop Base sync | Final | Cursor advances despite unresolved conflicts; downloads empty placeholders; no frontend caller/UI; keychain-only API key path. |
| C53 | `1616e37c` desktop sync merge | Final | Merge wrapper around C52; confirms cursor advances with unresolved conflicts, duplicate sync loops can start, downloads write placeholders, and desktop module proof fails without `frontend/dist`. |
| C54 | `2748dea9` bounded inbox/backpressure | Final | Ship blocker for M23 acceptance: inherited C01 durability bug, blocking send can target stale mailbox after passivation, durable log still grows before backpressure, product backpressure opt-in only, required runtime race proof incomplete. |
| C55 | `2161b2e3` bounded inbox merge | Final | Merge-only of C54; `-race` lost-wake test fails; no product `WithBackpressure` caller. |
| C56 | `30d33c54` race CI jobs | Final | Race jobs added and green in CI, but aggregate `check`, branch protection, and deploy gates do not require them. |
| C57 | `66ef1077` Pass 4 docs | Final | Mainlined count and V math inconsistent; proof refs still weak. |
| C58 | `39348558` go-keyring/vendorHash | Final | Discovery commit, not repair: set service hashes to `fakeHash`, exposed hash values in SBOM logs, and main CI/deploy did not complete. |
| C59 | `96558859` File Provider | Final | Swift/Go JSON mismatch, HTTP status bug, socket transport misconfiguration, no host registration or Finder proof. |
| C60 | `f8638483` File Provider merge | Final | Merge-only of C59; direct Swift type-check fails, app-group/socket and registration gaps remain. |
| C61 | `ae2c4f16` vendorHash/logs | Final | Diagnostic commit, not final repair; CI failed but exposed auth hash and missing internalDirs/hash issues. |
| C62 | `20236d85` auth vendorHash | Final | Applies auth hash from parent CI log; own CI canceled before SBOM/deploy proof; acceptance belongs to later `0fde3628`. |
| C63 | `db5f52c2` internalDirs | Final | Not package-build complete: proxy/gateway/sourcecycled/sandbox still used `fakeHash`, CI failed, and gateway/sourcecycled/sandbox missed transitive `internal/pii` until later repair. |
| C64 | `fa166de1` runtime deletion | Final | Not deletion-safe: removed trace trajectory API still has callers, comprehensive runtime build fails on stale tests, CI/deploy proof absent, and legacy `work_id` fallback claim is false. |
| C65 | `e6777ba3` add internal/pii dirs | Interim | Correctly adds transitive `internal/pii` for gateway/sourcecycled/sandbox, but CI was canceled and service hashes still remained fake. |
| C66 | `2c47751d` proxy vendorHash | Final | Proxy hash correct and SBOMed, but CI still failed on sourcecycled fakeHash; cluster not landed until C67. |
| C67 | `0fde3628` final hashes | Final | Go-service hash cluster converged and CI/deploy succeeded; SBOM diagnostics regressed for skipped `obscura`, and proof should not be phrased as all-package coverage. |
| C68 | `80674c3e` Pass 5 docs | Final | Introduced Pass 5 counting errors later repaired by `580d3bf4`; current report still has 26-delegated vs 27-row accounting drift and narrative-only proof refs. |
| C69 | `df5a2db7` qdrant OpenTimeout | Final | Correct qdrant `time.Hour` fix; broader bare `OpenTimeout: 3600` literal class remains in gateway circuit-breaker tests. |

## Recommended Next Work

1. Treat auth recovery token exposure as the first emergency fix. Add a regression test that proves the token is never returned in HTTP response.
2. Wire trace redaction into the production trace store constructor and add a runtime persistence regression.
3. Fix API-key proxy handling: strip bearer secrets, enforce scopes, prove route registration and deploy config.
4. Repair Base owner isolation and desktop conflict/cursor semantics before relying on Base sync.
5. Snapshot vmctl ownership data for every external read path, especially `LiveSandboxURL`.
6. Sanitize health errors and prove edge routing for `/health/ready` and service health routes.
7. Convert the mission/checkpoint ledger to structured/generated accounting with PR/commit/run evidence IDs.
8. Replace the SBOM/vendorHash loop with a single all-service Linux build and SBOM completeness gate before deploy.
9. Re-run targeted `-race` and runtime tests after actor mailbox/backpressure fixes; do not accept M23 on local non-race tests.
10. For File Provider, require Swift/Go JSON contract tests and signed Finder manual proof before calling M6 supported.

## Evidence Hygiene

This review intentionally does not claim:

- full deployed acceptance for every behavior-changing mission;
- that every late-running thread finished after this report cutoff;
- that local current-HEAD tests prove historical commit correctness;
- that docs-only ledger statements are reliable without external CI/run artifacts.

It does claim:

- every commit after `4857205` was assigned to a distinct Codex review thread;
- the principal risks above were independently reported by per-commit reviewers and cross-checked by the leader where possible;
- the report captures both final and interim thread evidence available at synthesis time.
