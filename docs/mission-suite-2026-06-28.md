# Mission Suite — 2026-06-28

**Date:** 2026-06-28
**Status:** active — open loops compiled for parallax mission authoring
**Purpose:** single view of all open work, sequenced into delegatable missions

This document compiles every open loop from the road-ahead, production
readiness checklist, doctrine docs, design docs, and worktree prototypes
into a suite of missions ready for parallax authoring and background agent
delegation.

## Mission Index

| # | Mission | Class | Status | Blocks / Unblocks | Est |
|---|---------|-------|--------|-------------------|-----|
| M1 | API Auth (headless) | orange | ready to delegate | unblocks M2, M5, M7, M9 | 3 pts |
| M2 | Choir Base reconciliation kernel | yellow | ready to delegate | unblocks M3, M4, M5 | 5 pts |
| M3 | Base journal + tree derivation | yellow | depends on M2 | unblocks M4 | 3 pts |
| M4 | Base API + blob store | orange | depends on M3 | unblocks M5, M6 | 3 pts |
| M5 | Desktop sync (Wails + Base) | orange | depends on M1, M4 | unblocks M6 | 5 pts |
| M6 | macOS File Provider | orange | depends on M5 | — | 5 pts |
| M7 | Auth: account recovery + multi-device | orange | depends on M1 | — | 5 pts |
| M8 | Runtime refactor continuation | red | active (3c_2 done) | unblocks M9-M12 | 8 pts |
| M9 | Mutation transaction hardening | red | depends on M8 | unblocks M10 | 3 pts |
| M10 | Choir-in-choir (candidate → promote) | red | depends on M9 | unblocks force multiplier | 5 pts |
| M11 | Race detector in CI | yellow | ready to delegate | — | 2 pts |
| M12 | Flaky Dolt test quarantine | yellow | ready to delegate | — | 1 pt |
| M13 | Privacy policy + ToS | green | ready to delegate | — | 1 pt |
| M14 | LLM cost tracking | orange | ready to delegate | — | 2 pts |
| M15 | Docs checker cleanup merge | green | ready to delegate | — | 1 pt |
| M16 | ObjectGraph prototype merge | yellow | ready to evaluate | depends on M8 | 2 pts |
| M17 | Qdrant pipeline merge | yellow | ready to evaluate | depends on M8 | 2 pts |
| M18 | Worktree triage (4 worktrees) | green | ready to delegate | — | 1 pt |
| M19 | Mission graph triage (27 open_handoff) | green | ready to delegate | — | 1 pt |
| M20 | Trace as primary observability | orange | ready to delegate | — | 5 pts |
| M21 | PII retraction pipeline | orange | ready to delegate | — | 5 pts |
| M22 | Health checks + circuit breakers | orange | ready to delegate | — | 3 pts |
| M23 | Bounded inbox + backpressure | orange | depends on M8 | — | 5 pts |
| M24 | Frontend auth fix staging verification | red | ready (code landed) | — | 1 pt |

## Mission Details

### M1: API Auth (Headless Access)

**Class:** orange — new DB tables, new endpoints, proxy behavior change
**Status:** ready to delegate — design doc complete
**Spec:** `docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md` §1

Add API key system to existing auth service:
- `api_keys` table in `internal/auth/store.go`
- `POST /auth/api-keys`, `GET /auth/api-keys`, `DELETE /auth/api-keys/{id}`
  handlers in `internal/auth/handlers.go`
- Bearer token validation in `internal/proxy/handlers.go` (fallback when
  cookie auth fails)
- Scope model: `read:base`, `write:base`, `read:texture`, `write:texture`,
  `read:runtime`, `write:runtime`, `admin`
- API keys created via WebAuthn session, used headlessly via Bearer header
- `last_used_at` tracking, soft-delete revocation

**Unblocks:** M2 (Base needs auth for transactions), M5 (desktop sync),
M7 (auth recovery), M9 (mutation transactions need author identity)

**Verification:** `go test ./internal/auth/...`, `go test ./internal/proxy/...`

### M2: Choir Base Reconciliation Kernel

**Class:** yellow — new packages, tests only, no production behavior change
**Status:** ready to delegate — spec and mission doc complete
**Spec:** `docs/archive/mission-choir-base-reconciliation-kernel-v0.md`,
`docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md` §2

Build `internal/base/model`, `internal/base/planner`, `internal/base/testkit`:
- Model types: Item, Version, Blob, Event, SyncStatus (tape entry types)
- Pure planner: `Plan(remote, local, synced Tree) ([]Action, []Conflict)`
- Testkit: deterministic scenario fixtures (6 required scenarios)
- No I/O, no wall clock, no random — pure functions
- Conflicts preserve both sides, never silent resolution
- Stable item IDs, not paths, define identity

**Unblocks:** M3 (journal), M4 (API), M5 (desktop sync)

**Verification:** `go test ./internal/base/...`, `go build ./...`

### M3: Base Journal + Tree Derivation

**Class:** yellow — new packages, tests only
**Status:** depends on M2 (needs model types)
**Spec:** `docs/archive/choir-base-product-spec-2026-06-06.md` data model section

Build `internal/base/journal`, `internal/base/tree`:
- Journal: append-only event store (in-memory + SQLite for tests)
- Tree derivation: rebuild consistent trees from journal events
- Event chain: ParentEventID forms Merkle chain (tamper-evident tape)
- CursorSeq: monotonic sequence for ordering
- Device cursors: track per-device sync position

**Unblocks:** M4 (API needs journal/tree)

**Verification:** `go test ./internal/base/...`

### M4: Base API + Blob Store

**Class:** orange — new HTTP endpoints, new storage
**Status:** depends on M3 (needs journal/tree)
**Spec:** `docs/archive/choir-base-product-spec-2026-06-06.md` API v0 section

Build `internal/base/blob`, `internal/base/api`:
- Blob store: immutable, content-addressed (SHA-256), hash-verified
- API v0: `POST /api/base/blobs`, `POST /api/base/items`,
  `GET /api/base/items/{id}`, `GET /api/base/delta?cursor=...`,
  `GET /api/base/items/{id}/status`, `POST /api/base/repair/preview`
- Auth integration: uses API key Bearer token (from M1)
- Each mutation creates a journal Event with SubjectID = authenticated identity

**Unblocks:** M5 (desktop sync needs API), M6 (File Provider needs API)

**Verification:** `go test ./internal/base/...`, staging deploy + API tests

### M5: Desktop Sync (Wails + Base)

**Class:** orange — desktop app behavior change
**Status:** depends on M1 (auth) + M4 (Base API)
**Spec:** `docs/archive/spec-choir-desktop-wails-v3-2026-06-22.md` Phase 4

Wire Base sync into Wails desktop app:
- Desktop app authenticates with API key (created via WebAuthn, one time)
- Background sync loop: scan local folder → compare with remote tree →
  plan actions → execute uploads/downloads/deletes
- Conflict surfacing: show conflicts in desktop UI, don't silently resolve
- Sync status: per-item state visible in desktop UI

**Unblocks:** M6 (File Provider needs desktop sync working)

**Verification:** local desktop proof, no staging deploy needed

### M6: macOS File Provider

**Class:** orange — native macOS extension
**Status:** depends on M5 (desktop sync)
**Spec:** `docs/archive/choir-base-product-spec-2026-06-06.md`,
`docs/choir-base-research-report-2026-06-06.md`

Implement NSFileProviderReplicatedExtension:
- File Provider domain backed by Base sync
- Finder integration: files appear in Finder under Choir domain
- Read/write: edits in Finder sync through Base to remote
- Conflict files: Base conflicts projected as `.conflict` files
- Development entitlements first, Developer ID signing later

**Verification:** local macOS proof with dev entitlements

### M7: Auth — Account Recovery + Multi-Device

**Class:** orange — auth behavior change
**Status:** depends on M1 (API key system provides the auth foundation)
**Spec:** `docs/road-ahead-2026-06-27.md` §3

- Email magic link recovery flow (fallback when passkey lost)
- Multi-device passkey management (add/remove/list credentials)
- Session management (view active sessions, revoke)
- Rate limiting on auth endpoints

**Verification:** `go test ./internal/auth/...`, staging deploy

### M8: Runtime Refactor Continuation

**Class:** red — protected surface (runtime, execution substrate)
**Status:** active — 3c_2 done, extraction plan documented
**Spec:** `docs/runtime-deletion-and-extraction-plan-2026-06-27.md`,
`docs/road-ahead-2026-06-27.md` §1

Continue the critical path:
1. Runtime cleanup — delete ~3-4K lines of dead code
2. agentcore extraction — tool loop as library
3. App registration API — `appagent.App`, init() registration
4. Texture extraction — first app, hardest, most entangled
5. Wire extraction — first verifier of the refactor
6. Remaining app extraction — browser, apppromotion, vmctl
7. Runtime dissolution — delete `internal/runtime/`

**Unblocks:** M9 (mutation hardening), M10 (choir-in-choir), M16/M17
(object graph + qdrant merge), M23 (bounded inbox)

**Verification:** full CI + staging deploy at each step

### M9: Mutation Transaction Hardening

**Class:** red — protected surface (promotion, rollback)
**Status:** depends on M8 (runtime refactor)
**Spec:** `docs/road-ahead-2026-06-27.md` Phase 2

Harden the appchange/promotion system:
- Complete capture: every promotion captures full delta
- Rollback refs: every promotion has valid rollback ref
- Verifier evidence: every promotion carries test/acceptance evidence
- Transaction semantics: atomic promotion (all or nothing)
- Freshness checks: candidate must be fresh relative to active computer
- Author identity: each transaction has SubjectID (from M1 API auth)

**Unblocks:** M10 (choir-in-choir needs trustworthy promotion)

**Verification:** full CI + staging deploy

### M10: Choir-in-Choir (Candidate → Promote)

**Class:** red — protected surface (candidate computers, promotion)
**Status:** depends on M9 (hardened transactions)
**Spec:** `docs/road-ahead-2026-06-27.md` Phase 3

Enable agents to create and edit apps through candidate → verify → promote:
1. Candidate VM app building — agent writes files, candidate builds
2. App verification in candidate — verification agents test
3. App promotion — candidate → active via hardened transaction
4. Frontend auto-discovery — new components picked up automatically

**Unblocks:** the force multiplier — everything else parallelizes

**Verification:** full CI + staging deploy + candidate promotion proof

### M11: Race Detector in CI

**Class:** yellow — CI config change
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P0

Add `-race` flag to runtime test shards in CI:
- `go test -race ./internal/runtime/...` in sharded CI
- May need to split shards further (race detector is slower)
- Primary defense against the bug class that borked the port

**Verification:** CI passes with race detector enabled

### M12: Flaky Dolt Test Quarantine

**Class:** yellow — test infrastructure
**Status:** ready to delegate
**Spec:** prior session noted `TestVSuperCoSuperSlotReusedByTrajectorySlot` as flaky

Quarantine the flaky Dolt test:
- Add `t.Skip()` with a comment referencing the flakiness
- Or move to a `//go:build comprehensive` tag
- Document the flakiness pattern for later investigation

**Verification:** CI passes without the flaky test failing

### M13: Privacy Policy + ToS

**Class:** green — docs only
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P0

Write privacy policy and terms of service:
- What data is collected (source captures, articles, LLM logs, trace events)
- How it's used (news synthesis, agent pipeline, observability)
- Third-party processors (LLM providers — OpenAI, Anthropic)
- Retention policy (how long each data type is kept)
- User rights (access, erasure, portability — GDPR)
- Acceptable use, liability, API terms

**Verification:** legal review (not code)

### M14: LLM Cost Tracking

**Class:** orange — store/provider change
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P0

Track per-cycle, per-article LLM API cost:
- Record token counts and costs in trace events
- Aggregate per-cycle, per-article, per-provider
- Alert on cost anomalies (spike detection)
- Surface in observability documents

**Verification:** `go test`, staging deploy, verify cost data in trace

### M15: Docs Checker Cleanup Merge

**Class:** green/yellow — docs + test fixes
**Status:** ready to delegate — worktree `go-choir-6b7967c1`
**Spec:** `docs/road-ahead-2026-06-27.md` worktree section

Merge the docs checker cleanup worktree:
- 15 files, 1565 insertions, 115 deletions
- `doc-authority-manifest.yaml` expansion
- Multiple docs updated for retired vocabulary
- Two test files fixed
- Check for conflicts with recent doctrine changes

**Verification:** `nix develop -c go run ./cmd/doccheck`, CI

### M16: ObjectGraph Prototype Merge

**Class:** yellow — new package with tests
**Status:** ready to evaluate — worktree `go-choir-29131320`
**Note:** may already be merged — check if `internal/objectgraph/` exists in main

Merge or re-evaluate the object graph prototype:
- `internal/objectgraph/` — service with dolt/sqlite/memory stores
- 633 lines of tests
- Not wired into runtime — standalone package
- May need integration with extracted app packages (after M8)

**Verification:** `go test ./internal/objectgraph/...`

### M17: Qdrant Pipeline Merge

**Class:** yellow — new package with tests
**Status:** ready to evaluate — worktree `go-choir-87c664e7`

Merge or re-evaluate the Qdrant pipeline prototype:
- `internal/qdrant/` — client, pipeline, schema, embed, samples
- `cmd/qdrantctl/main.go` — CLI tool
- `docker-compose.qdrant.yml` — local Qdrant
- May overlap with existing `internal/runtime/qdrant_dedup.go`

**Verification:** `go test ./internal/qdrant/...`

### M18: Worktree Triage

**Class:** green — docs/evaluation only
**Status:** ready to delegate
**Spec:** `docs/road-ahead-2026-06-27.md` worktree section

Triage four worktrees from ~2026-06-23:
1. Docs Checker Cleanup (go-choir-6b7967c1) → M15
2. ObjectGraph Prototype (go-choir-29131320) → M16
3. Qdrant Indexing Pipeline (go-choir-87c664e7) → M17
4. PPTX Renderer Prototype (go-choir-f4fdeb09) → evaluate for Slides app

For each: check if still exists, assess conflicts with current main,
recommend merge/hold/discard.

### M19: Mission Graph Triage

**Class:** green — docs work
**Status:** ready to delegate
**Spec:** `docs/road-ahead-2026-06-27.md` §6

Triage 27 open_handoff missions in the mission graph:
- Mark superseded missions
- Consolidate overlapping missions
- Identify missions absorbed into M8-M10 (runtime refactor)
- Identify missions that become trivial after choir-in-choir
- Document remaining active missions

### M20: Trace as Primary Observability

**Class:** orange — observability infrastructure
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P0

Promote trace from debugging tool to primary observability store:
- Persist trace events to Dolt (self-owned, versioned, queryable)
- No SaaS log export
- Supervision hierarchy reads trace events as structured observations
- Foundation for all supervision and self-learning layers

**Verification:** `go test`, staging deploy, verify trace events in Dolt

### M21: PII Retraction Pipeline

**Class:** orange — privacy infrastructure
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P1

SLM actor that redacts PII from trace events before persistence:
- 7B or smaller local model
- Redact at ingestion, never store raw PII
- Runs as another actor in the runtime
- Fine-tunable on Choir's specific patterns over time
- Privacy-by-design as pipeline stage, not deletion-after-the-fact

**Verification:** `go test`, verify redacted trace events

### M22: Health Checks + Circuit Breakers

**Class:** orange — reliability infrastructure
**Status:** ready to delegate
**Spec:** `docs/production-readiness-checklist.md` P1

Add health endpoints and circuit breakers:
- Health endpoints for sourcecycled, runtime, Qdrant, Dolt, Ollama
- CI deploys verify health before routing traffic
- LLM provider failures circuit-break (not retry endlessly)
- Qdrant failures degrade gracefully (skip dedup, don't block)
- Ollama failures: skip semantic dedup or queue for later

**Verification:** `go test`, staging deploy, verify health endpoints

### M23: Bounded Inbox + Backpressure

**Class:** orange — runtime behavior change
**Status:** depends on M8 (actor runtime)
**Spec:** `docs/production-readiness-checklist.md` P1

Bound actor mailboxes and add backpressure:
- Bounded inbox: prevent unbounded memory growth under burst
- Backpressure on Send: prevent unbounded durable log growth
- Actor failure observability: silent actor deaths are undebuggable
- Graceful shutdown drain: in-flight handler cancellation = partial side effects

**Verification:** `go test -race ./internal/runtime/...`

### M24: Frontend Auth Fix Staging Verification

**Class:** red — staging verification of landed code
**Status:** code landed (commit 6286f89f), needs staging verification
**Spec:** `docs/memo-frontend-auth-transient-logout-2026-06-28.md`

Verify the frontend auth retry fix on staging during a deploy:
- Trigger a staging deploy
- During the deploy (auth service restart), verify the frontend shows
  "reconnecting" state instead of logging out
- After deploy completes, verify the user is still signed in
- Test with a real WebAuthn session

**Verification:** staging deploy + manual browser test during deploy

## Dependency Graph

```
M1 (API auth) ──────────────┬──→ M5 (desktop sync) ──→ M6 (File Provider)
                             ├──→ M7 (auth recovery)
                             └──→ M9 (mutation hardening)

M2 (Base kernel) ──→ M3 (journal) ──→ M4 (Base API) ──┬──→ M5 (desktop sync)
                                                       └──→ M6 (File Provider)

M8 (runtime refactor) ──→ M9 (mutation hardening) ──→ M10 (choir-in-choir)
                       ├──→ M23 (bounded inbox)
                       ├──→ M16 (object graph merge)
                       └──→ M17 (qdrant merge)

Independent (ready now):
  M11 (race detector)    M13 (privacy policy)   M14 (LLM cost)
  M15 (docs cleanup)     M18 (worktree triage)  M19 (mission graph triage)
  M20 (trace observability)  M21 (PII retraction)  M22 (health checks)
  M12 (flaky test quarantine)
```

## Tonight's Delegation Plan

**Background agents (parallel, no dependencies between them):**

1. **M1: API Auth** — orange, well-specified, unblocks the most downstream work
2. **M2: Choir Base kernel** — yellow, pure Go, no deployment needed
3. **M11: Race detector in CI** — yellow, small, CI config only
4. **M12: Flaky test quarantine** — yellow, small, test infrastructure

**After tonight (next session):**

5. M13 (privacy policy) — green, can delegate anytime
6. M14 (LLM cost tracking) — orange, can delegate anytime
7. M15 (docs cleanup merge) — green, can delegate anytime
8. M18 (worktree triage) — green, can delegate anytime
9. M20 (trace observability) — orange, can delegate anytime

**Serial (critical path, not delegatable to parallel agents):**

10. M8 (runtime refactor) — red, requires careful staging
11. M9 (mutation hardening) — red, depends on M8
12. M10 (choir-in-choir) — red, depends on M9

## Lineage

- Compiled from `docs/road-ahead-2026-06-27.md` (open loops),
  `docs/production-readiness-checklist.md` (P0-P3 items),
  `docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md`
  (API auth + Base design),
  `docs/memo-artifact-program-doctrine-2026-06-28.md` (the tape),
  `docs/archive/mission-choir-base-reconciliation-kernel-v0.md` (Base mission),
  and the four worktree assessments.
- The mission suite is sequenced to maximize parallelism: independent
  missions start tonight, critical-path missions are serial, and the
  force multiplier (M10) unblocks everything else.
