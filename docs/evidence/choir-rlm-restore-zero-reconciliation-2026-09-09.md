# Restore-Zero Read-Only Reconciliation — 2026-09-09

Class: green (docs/evidence only; no runtime behavior change).
Authority: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md` (`now.next_action`: read-only reconcile live identities, then implement preparation; no repair code before reconciliation completes).

## 1. Source / worktree identities (observed 2026-09-09, UTC-4)

- `HEAD`: `24be54a29253229080cd1b1f5a3cc94c647bfdea` (`main`, promotion commit: restore-zero to working spine; cutover to blocked remainder holder; mission-1 stub).
- `git status --short`: empty (clean primary worktree `/Users/wiz/go-choir`).
- Other worktrees exist and are untouched (preflight review, codex definitions, autoputer-v2, s0-ratchet, s3i17, terminal-outcome-closure, chatgpt-search-provider, sandbox-rename). This mission owns only named restore/recovery surfaces; unrelated WIP preserved in place.
- Direnv/Nix shell loaded (`.envrc` allowed, `CGO_CFLAGS` exports ICU include path); `go test` runnable directly.
- Define receipt pinned: the code-free Define + topology promotion is immutable commit `24be54a2` (problem_ref: genesis-by-default rematerialization; silent required-base deferral; missing ancestry/compatibility verification; authorization_ref: owner topology answers 2026-09-09; registry conformance: `docs/ACTIVE.md`, `docs/mission-graph.yaml`, `docs/doc-authority-manifest.yaml` — all updated in that commit).

## 2. Staging deploy identity (anonymous product-path observation)

- `GET https://choir.news/health`: `status ok`, `service proxy`, `vmctl_status ok`, `vmctl_routing enabled`.
- Proxy build: `commit 3ef4405c91c63bf048e34fcdd7df2d3fb4755bbf`, `version 0.1.0`, `built_at 20260908151005`.
- Source `24be54a2` is AHEAD of staging build `3ef4405c` (docs-only delta since; no platform behavior change pending deploy — expected divergence, not a failed deploy).
- NOT re-observed (require scoped owner credentials; must precede any red mutation or drill): staging computer identity (`computer-03335285269bdba4f94377e56879f9e6` liveness, epoch, guest CodeRef, effects mode OFF, pre-A fence `99949fe2`), watermark/base-retention state, nontrivial-tail availability.

## 3. Verified caller map: producer → selection → installer → replay → verifier → route

All paths re-read at `HEAD`; line refs are current-source evidence for implement preparation.

1. **Producer (offline, not recovery proof):** `cmd/choir-rebuild-base` → `internal/projectionbase/rebuilder.go` (`Rebuilder.Run`: isolated full-tape replay from `CASReplaySource` into scratch) → `internal/projectionbase/publisher.go` (`Publisher.PublishDir`: atomic tar + sha256 to `sha256/projection-base/<digest>`, fsync) + `internal/projectionbase/types.go:50-81` (`Descriptor`: `computer_id`, `sequence`[=W], `canonical_head`[@W], `blob_sha256`, `blob_size_bytes`, `reducer_version`, `schema_version`, `vm_local_content_witness`, `created_at`). **Gap confirmed: no `vocabulary_version` field; `Validate()` does not require reducer/schema/witness.**
2. **Selection/advertisement (platform):** `internal/platform/file_cas_http.go` — `fileCASWatermarkRequest{computer_id, watermark_sequence, base_ref}` (watermark endpoint) and payload endpoint `artifact:sha256:<baseRef>`. Handshake does not mandate the full descriptor tuple.
3. **Installer (boot only):** `internal/autoputer/projection_base.go:29-149` (`materializeProjectionBaseIfNeeded`): empty-store gate → capability token → watermark GET → payload GET → sha256 check → `projectionbase.Unpack` to staging → rename into `storeDir` → fsync. **Silent-deferral confirmed:** missing capability/token, non-200 watermark, decode failure, `WatermarkSequence==0`/empty `baseRef` all `return false, nil`. Only blob-digest mismatch is an error — and the sole caller `internal/autoputer/run.go:551-554` (`runReplayPhase`) logs `ProjectionBase materialization deferred` and reconstructs anyway.
4. **Replay (three disjoint entries, no shared base contract):**
   - Boot: `runReplayPhase` → `ComputerEventAppender.Reconstruct` (from existing local head; `RUNTIME_RECOVERY_REPLAY_ONLY` one-shot host drive).
   - Rematerialize/restore: `internal/agentcore/rematerialize.go:92-107` (`RematerializeFromTape`: `choirstore.OpenFresh` staged store → `ReconstructThroughTarget(ctx, staged, targetHead)` from nil/zero local head; never consults a base) → `RebindProjection` flips the realization.
   - Probe: `internal/agentcore/replay_completeness.go:204-245` (`ReplayCompleteness`: disposable-workspace `ReconstructInto`, live store observed-only).
   - Core: `internal/computerevent/appender.go:682-732` (`reconstruct`: `after = localHead.Sequence`, forward-only apply with `Reduce` + receipt verify + `Prepare`/`finalizeProjection`), `959-988` (`ReconstructThroughTarget`: halts at target head).
5. **Verifier:** `internal/vmctl/recovery_authorities.go:62-129` (`HTTPRecoveryVerifier.VerifyRecovery` → guest `GET /api/computers/{id}/self-development/replay-completeness` + equivalence/serving-join) establishes replay-completeness/serving identity but NOT verified-base ancestry or tail-only work (`cold_recover.go:112-115` interface comment). `RematerializeFromTape` checks the materialized witness against the checkpoint witness post-reconstruction.
6. **Route/publication:** `internal/vmctl/cold_recover.go` (journal `coldRecoveryJournal`, quarantine/staging, `ColdRecoveryHeadReader` re-reads corpusd before/after, route publication after `VerifyRecovery`) + `internal/proxy/computer_lifecycle.go:360-369` (cold-recover fencing: non-empty checkpoint body refused with `cold recovery does not accept a checkpoint request`; owner-authorized cold-recover path). `ColdRecoverRequest` (`cold_recover.go:22-28`: exactly `computer_id`, `expected_canonical_head`, `expected_route_generation`, `idempotency_key`; strict decode rejects checkpoint/historical/base-override before host mutation) — `recover_current` remains current-head-only.

## 4. Red-surface candidate classification

Already protected (confirmed demonstrated): `internal/agentcore/rematerialize.go`, restore/replay-completeness handlers, `internal/computerevent/appender.go`, `internal/autoputer/projection_base.go` + `run.go`, `internal/projectionbase/*`, `internal/platform/file_cas.go` + `file_cas_http.go`, `internal/vmctl/cold_recover.go` + `recovery_authorities.go`, `internal/proxy/computer_lifecycle.go`, `cmd/choir/main.go`, `cmd/choir-rebuild-base/main.go`, canonical head/checkpoint/route/journal/quarantine/staging-deploy routing.

Candidates adjudicated (demonstrated → include; speculative → exclude):

- **INCLUDE `internal/agentcore/replay_completeness.go` + `api_self_development.go` replay routes:** mandatory verifier behind `VerifyRecovery`, rematerialize witness gate, and checkpoint bindings. Without it the "one verified ancestry predicate" has no carrier.
- **INCLUDE `internal/proxy/self_development.go` + `handlers.go` replay-completeness path:** deployed verifier transport (dedicated timeout, owned-computer binding, write-deadline extension). Product-path proof traverses it.
- **INCLUDE `TrustedGuestKeyCopier` (`internal/vmctl`, guest key copy seam):** sole path by which cold recovery reads guest data (quarantine read-only); recovery cannot be fenced without it.
- **INCLUDE `internal/agentcore/checkpoint_restore_bindings.go`:** binds checkpoint publication to `ReplayCompleteness`; publication-fence work lands here.
- **INCLUDE file-CAS hydration seam (`fileSyncService.HydrateIfNeeded`, called in `runReplayPhase` before reconstruct):** adjacent durable input to replay/witness; tail-only instrumentation must account for it or it becomes an unmeasured prefix scan.
- **EXCLUDE native goals v4, conductor-agentic, desks/rename, Engineering/Texture/Research/Management, prompt-bar, shadow evaluations, provider/hill-climbing:** no demonstrated restore participation; explicitly excluded in Definition boundaries. If implementation touches any, it joins protected_surfaces then with demonstration — never on enumeration.

## 5. Unknowns carried forward (unchanged, now dated)

- Staging computer/guest/epoch/effects/fence liveness (scoped-auth observation required before red mutation).
- Existence of a retained, compatible ProjectionBase + nontrivial tail for the staging computer.
- Retained-base selection/ancestry index for historical H older than the latest watermark.
- Measured full-path recovery cost; complete V1 role-bearing field inventory (mission-2 scope; only the `vocabulary_version` seam belongs here).

## 6. Next

Implement preparation: red-surface candidate classification is above (pending Definition `protected_surfaces` amendment to add the five INCLUDE items); then slice 1 (descriptor + `vocabulary_version` + verified installation/ancestry predicate) with simplification adjudication per slice. No repair code has been written; no behavior changed in this receipt.
