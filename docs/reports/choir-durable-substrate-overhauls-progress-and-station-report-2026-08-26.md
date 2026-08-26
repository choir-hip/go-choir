# Choir Durable Substrate Overhauls — Progress & Station Report
**Date:** 2026-08-26  
**Context:** Mission Execution Review, Agentic Consensus Synthesis, and Roadmapping  
**Target:** Engineering Leadership, Autonomous Self-Development Agents, and System Operators  
**Authority:** Derived from Apex Doctrine (`docs/choir-doctrine.md`), Durable Substrate Overhauls Definition (`docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md`), Design Specification (`docs/designs/choir-durable-substrate-2026-08-23.md`), and Agentic Consensus Panel (2026-08-26)

---

## 1. Executive Summary & Architectural Station

The **Durable Substrate Overhauls** mission transforms Choir's foundational storage, cryptographic custody, and recovery layers to establish deterministic, bounded-time disaster recovery and zero-unescrowed computer survivability for the **Automatic Computer**.

Over August 23–26, 2026, two foundational tracks achieved major production milestones on staging (`https://choir.news`):

```
+---------------------------------------------------------------------------------------+
|                       DURABLE SUBSTRATE OVERHAULS STATION                             |
|                                                                                       |
|   +-------------------------------------------------------------------------------+   |
|   | TRACK K: KEY ESCROW & WRAP HIERARCHY (COMPLETE & DEPLOYED-PROVEN)             |   |
|   | - 2-of-N Operator Quorum + WebAuthn PRF Wrap Gate                            |   |
|   | - Monotonic Epochs, Append-Only Transparency Log                              |   |
|   | - Lazy Per-Boot Guest Recovery Verified on Staging (4/4 Acceptance Passed)    |   |
|   +-------------------------------------------------------------------------------+   |
|                                          |                                            |
|                                          v                                            |
|   +-------------------------------------------------------------------------------+   |
|   | TRACK F: FILE-CAS & REPLAY WATERMARKS (FOUNDATION & HYDRATION LANDED)         |   |
|   | - Fixed 4 MiB Encrypted Chunks (XChaCha20-Poly1305 + HKDF + Computer AAD)     |   |
|   | - Canonical Merkle Manifests & FileRootCommitted Tape Event Citations         |   |
|   | - POST /api/files/sync Barrier RPC & 15m Periodic Checkpoint Ticker           |   |
|   | - Fail-Open Latest-Root Boot Hydration (HydrateIfNeeded)                      |   |
|   | - Referential Namespace-Contained GC (24h Grace)                              |   |
|   +-------------------------------------------------------------------------------+   |
|                                          |                                            |
|                                          v                                            |
|   +-------------------------------------------------------------------------------+   |
|   | NEXT ACTIVE EXECUTION: TRACK F RESTORE PROOF & TRACK M MAIL ON CAS            |   |
|   | - Atomic Sibling Directory Staging & Tape-Cited Hydration Hardening           |   |
|   | - ProjectionBase Boot Unpack (Reconstruction without Appender Mutation)       |   |
|   | - O(delta) File Restore Deployed Proof on Disposable Staging Computer         |   |
|   | - Track M: Mail MTA Spool & Guest Maildir Architecture                        |   |
|   +-------------------------------------------------------------------------------+   |
+---------------------------------------------------------------------------------------+
```

---

## 2. Track K: Key Escrow & Wrap Hierarchy (Complete)

### 2.1 Implemented & Deployed Architecture
- **Custodian Wrap Module (`internal/keyescrow`)**: X25519-HKDF-XChaCha20-Poly1305 authenticated envelope encryption. Computer Data Encryption Keys (DEKs) are wrapped under operator custodian public keys.
- **Platform Endpoints (`internal/platform/key_escrow_http.go`)**:
  - `POST /internal/computers/keys/escrow` (guest capability-authenticated DEK upload on creation/boot).
  - `POST /internal/computers/keys/unwrap-propose` (operator-initiated unwrap request).
  - `POST /internal/computers/keys/unwrap-approve` (independent operator approval quorum).
  - `POST /internal/computers/keys/unwrap-reveal` (DEK release only after $M$-of-$N$ threshold satisfied).
- **Dolt Schema (`internal/platform/store.go`)**:
  - `computer_key_escrow`: Host-wrapped DEKs with monotonic key generation and creation timestamps.
  - `computer_key_escrow_approvals`: Multi-operator quorum tracking with expiration timestamps.
  - `computer_key_escrow_log`: Append-only cryptographic transparency audit ledger with sequence hashes.
- **Lazy Per-Boot Recovery (`internal/autoputer/key_escrow.go`)**: Guest automatically attempts local DEK unwrap at boot; on a fresh/cold MicroVM, falls back to custodian-wrapped recovery.

### 2.2 Staging Acceptance Proof (Evidence: `docs/evidence/track-k-keys-acceptance-staging-2026-08-26.md`)
Deployed on staging under commit `84e4daee` (subsequently confirmed in `bd97c848`):
1. **Escrow on Boot**: Active test computer booted, established fresh runtime, and uploaded custodian wrap (`key_digest 914ea9b2...`).
2. **Two-Approval Gate**: Self-approval rejected (`403`), single-operator approval held unwrap closed (`409`), two independent operator approvals (`op-a`, `op-b`) released the DEK with matching SHA-256 digest.
3. **Transparency Log**: Chained event log verified with continuous sequence tracking ($0 \to 2$); every reveal and approval committed.
4. **Gate Closure**: Unauthorized requests blocked before operator provisioning (`403`).

---

## 3. Track F: Content-Addressed File-CAS & Replay Watermarks

### 3.1 Implemented & Deployed Foundation (`internal/filecas`, `internal/platform`, `internal/autoputer`)
- **Encrypted Chunk Engine (`internal/filecas/filecas.go`)**: Fixed 4 MiB chunks sealed via `XChaCha20-Poly1305` using the computer DEK with computer-bound Additional Authenticated Data (`AAD: computer_id`). Ciphertext SHA-256 addressing guarantees data confidentiality against host inspection while maintaining immutable CAS semantics.
- **Deterministic Merkle Manifests**: Canonical JSON format mapping file paths, permissions, sizes, and ordered chunk hashes to a deterministic root digest verified via `filecas.VerifyRoot()`.
- **Platform Blob Storage & Endpoints (`internal/platform/file_cas_blobs.go`, `file_cas_http.go`)**:
  - Dedicated namespaces: `file-cas-chunks/<computer>/<digest>` and `file-cas-roots`.
  - Monotonic Dolt tables: `computer_file_roots` and `computer_replay_watermarks`.
  - HTTP Endpoints: `PUT/GET /internal/computers/files/chunks/{digest}`, `PUT/GET /internal/computers/files/root`, `POST/GET /internal/computers/files/watermark`.
- **Referential Garbage Collection (`internal/platform/file_cas_gc.go`)**: Namespace-contained GC collecting unreferenced chunks older than 24 hours while preserving the latest root per computer and all roots active within the grace window.
- **Tape Citation (`internal/computerevent/event.go`)**: `EventFileRootCommitted` (`file_root_committed`) tape kind. Tape owns the root reference via pinned manifest payload; CAS owns the encrypted chunk payloads.
- **Guest Sync Barrier & Ticker (`internal/autoputer/file_sync.go`)**:
  - `fileSync` service walks `/mnt/persistent/files`, hashes chunks, uploads new ciphertext, and publishes manifests.
  - `POST /api/files/sync`: Synchronous barrier endpoint for checkpoint coordination (`sync_computer_files()` RPC).
  - 15-minute default periodic sync ticker (`AUTOPUTER_FILE_SYNC_INTERVAL`).
- **Fail-Open Boot Hydration (`internal/autoputer/file_sync.go`, commit `468e78e4`)**:
  - `HydrateIfNeeded()`: Detects cold boot / empty files tree, fetches latest root manifest, verifies Merkle root, decrypts chunks, and reconstructs files with strict path-traversal defenses.

---

## 4. Synthesis of Agentic Consensus Panel (2026-08-26)

A 12-model agentic consensus panel (`codex`, `cursor`, `opencode`, `omp-gpt56-sol`, `omp-gemini37`, `omp-cursor-grok46`, `omp-muse-spark`, `omp-hy3`, `devin`) was convened to evaluate the overhaul roadmap.

### 4.1 Unanimous Architectural Consensus
1. **Do Not Mutate `ComputerEventAppender` for Watermark Replay**:
   - `ComputerEventAppender` already possesses the correct replay invariants (reconstruct from local projection head to canonical head).
   - Replay bounding must be achieved via a **pre-reconstruction boot materializer** that unpacks a verified `ProjectionBase` into local SQLite/Dolt before invoking `Reconstruct()`. `Reconstruct()` then naturally begins from sequence $H+1$ without modifying tape appender invariants.
2. **Track M Scope Clarification**:
   - `docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md` defines Track M as **Mail MTA Spool & Guest Maildir** (Mail on CAS), not a memory-mapped projection store. The settled Definition must be respected; projection store optimizations belong in a separate, dedicated mission.
3. **Atomic Hydration Staging**:
   - Hydration must unpack into a sibling staging directory, verify file integrity, fsync, and atomically swap into place before the runtime starts. A partial hydration must never leave a half-populated tree that subsequent boots mistakenly treat as complete.
4. **Tape-Cited Root Authority**:
   - Hydration and recovery must target the latest **canonically cited** root on the event tape rather than an unverified or orphaned platform root table entry.

---

## 5. Remaining Overhaul Roadmap & Concrete Execution Plan

### Phase 1: Track F Hardening & Staging Proof (Active Next Step)
1. **Atomic Sibling Hydration**: Update `HydrateIfNeeded` to unpack into `.files.staging.<root>`, fsync directory contents, atomically rename to `files`, and persist a completion marker.
2. **Tape-Cited Root Resolution**: Ensure guest hydration queries the tape for the last valid `EventFileRootCommitted` event.
3. **ProjectionBase Boot Materializer**: Implement verified snapshot unpack prior to appender startup.
4. **Deployed O($\Delta$) File Restore Proof**:
   - Target: Disposable test computer on staging (`choir.news`).
   - Sequence: Write test files $\to$ Trigger `POST /api/files/sync` $\to$ Record `FileRootCommitted` event $\to$ Simulate disk loss (wipe guest files) $\to$ Reboot VM $\to$ Prove byte-identical reconstruction in $O(\Delta)$ time.

### Phase 2: Track M — Mail MTA Spool & Guest Maildir
1. Implement guest Postfix/Maildir storage backed by the File-CAS write-back commit protocol.
2. Wire mail arrivals to atomic CAS checkpoints and event citations.
3. Verify zero-data-loss mail preservation across simulated MicroVM crashes.

### Phase 3: Assurance & Scale
1. Implement self-describing portable recovery capsules.
2. Construct recovery cells with restore budget scheduling and disaster recovery drills.

---

## 6. Station & Integrity Verification

- **Current Repository State**: `main@468e78e4` (all tests passing locally and in CI).
- **Staging Deploy Commit**: `5edb0174` (CI `32973745840` success; Node B deployed).
- **Protected Boundaries**: Staging host hold on historical artifact `computer-03335285269bdba4f94377e56879f9e6` strictly respected; all testing isolated to designated test computers.
