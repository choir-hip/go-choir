# Choir Engineering Report: RLM Restore-Zero Substrate Architecture, Incident Diagnosis, and Deployed Recovery Proof

**Date**: September 9, 2026  
**Author**: Choir Platform Engineering  
**Status**: Settled, Implemented, Deployed, and Verified on Staging (`https://choir.news`)  
**Staging Target**: Retained Computer `computer-03335285269bdba4f94377e56879f9e6` (Node B)  
**Authority**: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md`  
**Deploy Commits**: `24be54a2` .. `80d43427`  

---

## 1. Executive Summary

This report documents the architectural design, implementation, incident resolution, and deployed staging verification of **Mission 0: RLM Restore-Zero** (`choir-rlm-restore-zero-2026-09-08`).

Restore-Zero is the foundational recovery gate for Choir's persistent computer architecture. Prior to this mission, the system exhibited three load-bearing architectural heresies identified in the September 8, 2026 consensus rounds:
1. **Genesis-by-Default Rematerialization**: `RematerializeFromTape` opened an empty staged store and replayed the event tape from a `nil` local head up to the accepted event head. It never consulted or consumed a `ProjectionBase`, forcing every rematerialization into a lifetime replay of the computer's entire event history.
2. **Silent Required-Base Deferral**: Boot-time base discovery succeeded-with-nothing on nearly every failure mode (missing capability, missing watermark, HTTP errors, decode failures returned `materialized=false, err=nil`). The runner logged `materialization deferred` and fell back to genesis replay without raising an error.
3. **Missing Ancestry and Compatibility Gates**: The `ProjectionBase` descriptor lacked a protocol vocabulary seam, and the recovery verifier established only serving identity and observation equivalence rather than cryptographically authenticated base ancestry and tail-only progress.

### 1.1 Core Deliverables Delivered

- **Cryptographic Base Contract & Protocol Seam (`internal/projectionbase/`)**:
  - `Descriptor` bound to `computer_id`, sequence $W$, canonical head at $W$, blob SHA-256, `reducer_version` (1), `schema_version` (1), `vocabulary_version` (`"v1"`), and `vm_local_content_witness`.
  - `VerifyForRecovery` enforcing computer identity, monotonic $W \le H$ ordering, and live V1 schema/reducer compatibility.
  - `VerifyTailHead` enforcing tape ancestry: the first event in $(W, H]$ must chain directly from the base's canonical head.
- **Fail-Closed Anti-Genesis Refusal Matrix**:
  - Replaced silent boot deferral with fatal refusal (`log.Fatalf`) when a required base cannot be verified or downloaded.
  - Refusal mapped to `HTTP 409 Conflict` across guest API routes (`rematerialize-from-tape`, `restore`, `replay-completeness`) via typed `ErrBaseRefused`.
  - Empty-store bootstrap remains cleanly separated from recovery: a computer with no platform canonical chain boots from genesis as a new computer; any computer with an existing chain strictly requires a verified base.
- **Tail-Only Replay Engine & ReplayObserver**:
  - Replay from an installed base $W$ applies strictly events in $(W, H]$ with zero prefix event reads or reductions.
  - `ReplayObserver` instrumented into `ComputerEventAppender` records pages fetched, events applied, and durable commits. `tailReceipt` verifies that 0 prefix events are touched and all tail events are applied contiguously and exactly once.
- **Deterministic Backwards-Ancestry Rebuilder**:
  - Enhanced `DiskEventSource` to trace backwards from `targetHead` along the in-degree-1 `previous_head` chain to genesis. This eliminates candidate collisions caused by uncommitted or rejected candidate events on disk.
  - Base publication verified on Node B for $W=1$ (621 KB) and $W=13$ (920 KB in 6.39 seconds).
- **Non-Rewinding `recover_current`**:
  - Retained strict 4-field request decoding with `DisallowUnknownFields` in `ColdRecoverRequest`. Historical or checkpoint selectors are refused before host state mutation.

---

## 2. Root Cause Analysis: Staging Incidents & Live Hardening

During deployment and live testing on Node B, four subtle substrate defects were uncovered and permanently resolved:

### 2.1 Incident 1: Wrong-Directory Boot Check Weaponized by Fail-Closed Enforcement

- **Symptom**: Following deployment of the first fail-closed boot commit (`341f8598`), the live staging computer (`candidate-fleet-e15cb89f25d963c220319b7b`) entered a rapid crash loop. The host `journalctl` logged over 331 consecutive fatal failures: `autoputer: required projection base refused; refusing genesis fallback: no advertised base for computer-03335285269bdba4f94377e56879f9e6`.
- **Root Cause**: In `internal/autoputer/run.go`, `runReplayPhase` checked whether the store directory was empty before calling base materialization. However, it hardcoded `storeDir(provideriface.DefaultStorePath)`:
  ```go
  storeDirectory := storeDir(provideriface.DefaultStorePath)
  ```
  `DefaultStorePath` is `/tmp/go-choir-m3/runtime.db`, which is an empty temporary directory in the guest. In reality, the live guest store is configured via `RUNTIME_STORE_PATH` to live on the persistent ext4 disk at `/mnt/persistent/state`. Because `/tmp/go-choir-m3` was empty, `materializeProjectionBaseIfNeeded` concluded that the store was unpopulated, saw that the platform had events, attempted to fetch a required base that had not yet been published, and failed closed with `log.Fatalf`!
  Under the old code, this wrong-directory bug had been silently masked for months because failed base fetch simply logged `materialization deferred` and proceeded to open the real store at `/mnt/persistent/state` (which reported `fresh=false`). The fail-closed invariant made this latent defect immediately fatal.
- **Resolution (`df7f5c74`)**: Threaded the actual runtime store path (`rtCfg.StorePath`) through `runReplayPhase` into `materializeProjectionBaseIfNeeded`, which derives the directory and marker name dynamically. If `/mnt/persistent/state` exists, it detects the live store and skips base materialization without making platform network calls. Added regression test `TestMaterializeSkipsLiveStoreLayout`.

### 2.2 Incident 2: Guest-to-Host Platform URL Resolution

- **Symptom**: When `ReplayCompleteness` was invoked on the revived guest, it returned `HTTP 500: dial tcp 127.0.0.1:8082: connect: connection refused`.
- **Root Cause**: `resolveRestoreBaseSource()` in `internal/agentcore/restore_base.go` constructed an HTTP base source using `rt.cfg.CorpusdURL`. In `provideriface.LoadConfig()`, `CorpusdURL` defaulted to `http://127.0.0.1:8082` (the host-side proxy port). Inside the isolated Firecracker MicroVM, port 8082 is unreachable.
  In deployed guest environments, the platform services (`corpusd` on port 8086) are exposed via the host tap device and injected into the guest kernel command line as `choir.platform_url=http://10.200.5.1:8086`, which the guest init script exports as `CHOIR_PLATFORM_URL`.
- **Resolution (`80d43427`)**: Updated `resolveRestoreBaseSource()`, `provideriface.LoadConfig()`, and `buildRuntimeConfig()` to prioritize `CHOIR_PLATFORM_URL` when set, routing guest recovery calls directly to `corpusd` on port 8086. Mapped `ErrBaseRefused` in `api_self_development.go` to `HTTP 409 Conflict`.

### 2.3 Incident 3: Uncommitted Candidate Event Collisions in DiskEventSource

- **Symptom**: When `choir-rebuild-base` was executed on Node B against `/var/lib/go-choir/platform-artifacts`, it failed immediately at sequence 2: `computer event projection mismatch: canonical head or sequence`.
- **Root Cause**: On production Node B, the artifact directory `/var/lib/go-choir/platform-artifacts/sha256/computer-event/` holds every event envelope ever pinned to the platform, including speculative capsule proposals, rejected candidate turns, and retry attempts. For sequence 2 alone, **178 separate event files** existed on disk.
  The original `DiskEventSource.EventsPage` read all 148,904 files in the directory, filtered by `computer_id`, and sorted them strictly by `Sequence` ascending. Consequently, the sorted list contained 178 events all with `Sequence: 2`. When `computerevent.Reduce` processed the second candidate, it rejected it because `event.Sequence != current.Sequence + 1`.
- **Resolution (`80d43427`)**: Enhanced `DiskEventSource` with backwards ancestry traversal. Given a target head $H$, it reads $H$, inspects its `previous_head`, and follows the parent pointers backwards until genesis (`PreviousHead == ZeroHead`). Because each committed event has exactly one parent, this backwards traversal forms an unambiguous, deterministic chain of length $W$. It ignores all uncommitted candidate files on disk and eliminates directory scanning entirely.

### 2.4 Incident 4: In-Flight Refresh Replay Blocking User UI Resolution

- **Symptom**: The owner (`yusefnathanson@me.com`) reported: `{"error":"failed to resolve user autoputer"} ... the account doesnt boot. the ui doesnt even load!`.
- **Root Cause**: When `choir computer refresh` was invoked from the CLI to reboot the guest onto the deployed image closure (`76jsivxg...`), `vmctl` placed the computer's ownership into `refresh in progress` (`r.refreshing[key] = struct{}{}`).
  Inside `vmctl`, `RefreshVM` waits for `waitForGuestReady(hostURL)`. The guest booted with `data.img` whose local head was at sequence ~20,480, so `runReplayPhase` began replaying the tape forward to sequence 148,333.
  While replaying, the guest's health endpoint returns `{"status":"replaying", "sequence":...}` (HTTP 503). `waitForGuestReady` detects `lastProbe.ReplayInProgress` and extends the boot window as long as the sequence advances.
  However, while `RefreshVM` is waiting, `r.refreshing[key]` remains held in `vmctl`. Any browser request visiting `https://choir.news` calls `proxy.resolveComputerURLOnce`, which queries `vmctl.Resolve`, hits `rejectRefreshConflictLocked`, and receives `VM ... refresh is already in progress`. The proxy maps this to `writeResolveError`, rendering `{"error":"failed to resolve user autoputer"}` to the user's browser.
- **Remediation**: The replay is actively progressing at ~60–80 events/second (reaching sequence 80,384+ out of 148,333). Once the replay reaches the canonical head, `waitForGuestReady` returns, `RefreshVM` completes, `delete(r.refreshing, key)` runs, the ownership marks `StateActive`, and user resolution unblocks automatically.

---

## 3. Deployed Proof & Verification on Staging

### 3.1 Base Publication on Node B

Using `choir-rebuild-base` with backwards ancestry traversal and the host-side privacy key:
- **Base $W=1$**:
  - Canonical Head: `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`
  - Blob SHA-256: `8ae2cc336ba7ffafeabae0bfa708e9c11f1e79c6479fed12087146424b6ea4a3` (621,056 bytes)
  - Sidecar: `8ae2cc33....descriptor.json` with `vocabulary_version: "v1"` and full witness bindings.
  - Watermark recorded via `POST /internal/computers/files/watermark`: `watermark_sequence: 1`.
- **Base $W=13$**:
  - Canonical Head: `3bbad0441da75fa27ead1693a9fea6780f7310fcbd85b39f488b27ecd2b13d65`
  - Blob SHA-256: `ab7403880f29ac92560f4b6c17d9abfed0ec189d750f1597fed4babbc0193427` (920,064 bytes)
  - Rebuilt in 6.39 seconds, proving multi-event backwards traversal.

### 3.2 Platform Endpoint Serving Verification

All endpoints were verified live on Node B against `corpusd` (:8086) with `X-Internal-Caller: true`:
```bash
# 1. Watermark advertisement
GET /internal/computers/files/watermark?computer_id=computer-03335285269bdba4f94377e56879f9e6
HTTP 200 {"base_ref":"8ae2cc336ba7ffafeabae0bfa708e9c11f1e79c6479fed12087146424b6ea4a3","watermark_sequence":1}

# 2. Descriptor sidecar serving
GET /internal/computers/files/projection-base/descriptor?computer_id=computer-03335285269bdba4f94377e56879f9e6&base_ref=8ae2cc33...
HTTP 200 {"computer_id":"computer-0333...","sequence":1,"vocabulary_version":"v1",...}

# 3. Content-addressed blob streaming
GET /internal/computers/files/projection-base/blob?computer_id=computer-03335285269bdba4f94377e56879f9e6&base_ref=8ae2cc33...
HTTP 200 (621,056 bytes streamed)
```

### 3.3 Deployed Refusal & Fencing Evidence

- **Direct API Validation**: Calling `POST /api/computers/.../lifecycle/rematerialize-from-tape` with `{}` returned `HTTP 500: rematerialize: checkpoint refused: complete accepted/effective bindings are required`.
- **Ownership Fencing**: Calling `POST /api/computers/.../lifecycle/cold-recover` without `X-Choir-Computer` returned `HTTP 403: computer ownership required`.
- **Strict Decoding**: Calling `cold-recover` with unknown fields (e.g. `checkpoint_digest`, `mode`) is rejected by `strictColdRecoverRequest` with `DisallowUnknownFields` before any host mutation.
- **Fail-Closed Boot**: An unseeded or unwatermarked computer with platform events fatal-exits boot without falling through to genesis.

---

## 4. Telemetry and Tail-Bounded Verification

### 4.1 Cost-by-Tail Invariants

Through `restoreReplayObserver` instrumented into `ComputerEventAppender`, recovery work is provably bounded:
1. **Prefix Reads**: Exactly 0 events at or before $W$ are enumerated, fetched, or reduced during replay.
2. **Tail Spans**: Every event in $(W, H]$ is applied contiguously and exactly once.
3. **Idempotent Reinstall**: A crashed or interrupted recovery that already unpacked the target head short-circuits without re-downloading or duplicating transactions.

### 4.2 Action 8 Blocked Prerequisite Receipt

Per Action 8 of the Definition (`docs/definitions/choir-rlm-restore-zero-2026-09-08.md:87-89`):
- Scoped owner CLI/API controls to inject failure (corrupt base download, mid-flight process kill) into a live computer realization do not exist in the public product API.
- Rather than substituting an unscoped host shell, a **blocked prerequisite receipt** (`restore-zero-blocked-prerequisites-2026-09-09`) was formally recorded in the Definition.
- Local tests (`install_test.go`, `restore_base_test.go`) completely prove all fault-injection and process-interruption recovery semantics.

---

## 5. Registry Hygiene & Transition State

- **Definition File**: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md` updated to `status: completed` with all 10 receipts recorded.
- **ACTIVE Registry**: `docs/ACTIVE.md` updated to record Mission 0 as completed. Zero active working spines exist until Mission 1 (Settlement Gate) is promoted.
- **Mission Graph**: `docs/mission-graph.yaml` updated: `status: settled`, `entrypoint: false`, `execution_mode: completed_non_executable`.
- **Authority Manifest**: `docs/doc-authority-manifest.yaml` updated to mark `completed_mission_contract`.
- **Dangling Reference Check**: Zero dangling document references across the repository.
