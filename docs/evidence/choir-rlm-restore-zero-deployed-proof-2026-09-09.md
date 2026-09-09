# Restore-Zero Deployed Proof and Telemetry — 2026-09-09

Class: green (evidence only).
Authority: `docs/definitions/choir-rlm-restore-zero-2026-09-08.md` (finish.acceptance items 7, 8, 9).

## 1. Deployment and Environment Identity

- **Pushed Commits:**
  - `24be54a29253229080cd1b1f5a3cc94c647bfdea` (Define + topology promotion)
  - `e00540a08e64c2ca2664b4c67ba5224e7545ca87` (Reconciliation receipt + surface classification)
  - `25e8783bf0b3fc477b6d5f63e262baee4a94bb52` (Slice 1: descriptor vocabulary_version seam + VerifyForRecovery / VerifyTailHead + refusal matrix)
  - `341f8598fd9a4e27bee90089fb13ed7ab7e2c159` (Slice 2a: descriptor sidecar publication + platform serving + verified base installer + boot refusal)
  - `4d6e67e799ac82fc162a74aa7ca5100fbb38c670` (Slice 2b: rematerialize/restore/probe on verified base + tail-only replay + replay observer)
  - `df7f5c74afc677511ec18701a255f7dd8e88e8a9` (Incident fix: installer derives store path from runtime config instead of hardcoded default)
  - `80d434279adbdd0a1243ec35feabe3bf11a00129` (Platform URL resolution via `CHOIR_PLATFORM_URL`, 409 status mapping for probe refusal, backwards traversal from targetHead, base64 key-file decoding)
- **CI Status:** Run `34310096303` completed `success` (all test shards, race detection, and staging deploy passed).
- **Staging Health:**
  - URL: `https://choir.news/health`
  - Status: `ok`, upstream: `vmctl`, `vmctl_routing: enabled`, `vmctl_status: ok`
  - Build: `commit: 80d434279adbdd0a1243ec35feabe3bf11a00129`, `built_at: 20260909041302`, `deployed_at: 2026-09-09T04:30:46Z`
- **Target Computer:** `computer-03335285269bdba4f94377e56879f9e6` (active staging computer, realization epoch 890, propose_only mode).

## 2. Base Publication and Platform Serving Proof

Offline `choir-rebuild-base` executed on Node B against `/var/lib/go-choir/platform-artifacts`:

- **Published Base W=1:**
  - Target head: `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44` (sequence 1, `genesis_imported`)
  - Blob SHA-256: `8ae2cc336ba7ffafeabae0bfa708e9c11f1e79c6479fed12087146424b6ea4a3`
  - Blob size: 621,056 bytes
  - Sidecar descriptor: `8ae2cc336ba7ffafeabae0bfa708e9c11f1e79c6479fed12087146424b6ea4a3.descriptor.json`
  - Bindings: `computer_id: "computer-03335285269bdba4f94377e56879f9e6"`, `sequence: 1`, `reducer_version: 1`, `schema_version: 1`, `vocabulary_version: "v1"`, verified `vm_local_content_witness`
- **Published Base W=13:**
  - Target head: `3bbad0441da75fa27ead1693a9fea6780f7310fcbd85b39f488b27ecd2b13d65` (sequence 13, `key_revoked`)
  - Blob SHA-256: `ab7403880f29ac92560f4b6c17d9abfed0ec189d750f1597fed4babbc0193427`
  - Blob size: 920,064 bytes
  - Rebuilt in 6.39 seconds via backwards ancestry traversal along `previous_head` chain, proving multi-event chain rebuilding without scanning uncommitted candidates.
- **Watermark Record:**
  - Recorded in Dolt via `POST http://127.0.0.1:8086/internal/computers/files/watermark`:
    `{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","watermark_sequence":1,"base_ref":"8ae2cc336ba7ffafeabae0bfa708e9c11f1e79c6479fed12087146424b6ea4a3"}` -> `HTTP 200`
- **Serving Endpoints Verified on Node B (corpusd :8086):**
  - `GET /internal/computers/files/watermark` -> `HTTP 200 {"base_ref":"8ae2cc33...","watermark_sequence":1}`
  - `GET /internal/computers/files/projection-base/descriptor` -> `HTTP 200` with validated descriptor JSON
  - `GET /internal/computers/files/projection-base/blob` -> `HTTP 200`, exactly 621,056 bytes streamed

## 3. Deployed Refusal Matrix and Error Mapping

| Scenario | Entrypoint | Observed Response | Contract Satisfied |
|---|---|---|---|
| Missing base (pre-watermark) | Boot installer | Fatal log: `required projection base refused; refusing genesis fallback: no advertised base` | Fail-closed: cannot silently enter genesis replay |
| Wrong store dir check | Boot installer | Fatal log: `required projection base refused` (incident diagnosed and repaired in `df7f5c74`) | Fail-closed: cannot enter unverified path |
| Live store layout | Boot installer | Skips base install without platform reads (`TestMaterializeSkipsLiveStoreLayout` passes) | Live computers boot without base dependency |
| Invalid checkpoint | `POST /lifecycle/rematerialize-from-tape` | `HTTP 500 {"error":"rematerialize: checkpoint refused: complete accepted/effective bindings are required"}` | Rejects malformed checkpoints before workspace mutation |
| Non-descendant target | `POST /lifecycle/rematerialize-from-tape` | `HTTP 409 {"error":"projection base refused: target is not a descendant of watermark 1"}` | Refuses target head outside base ancestry |
| Foreign base descriptor | `POST /lifecycle/rematerialize-from-tape` | `HTTP 409 {"error":"projection base refused: base computer is not this computer"}` | Refuses cross-tenant or mismatched base |
| Corrupt blob | `POST /lifecycle/rematerialize-from-tape` | `HTTP 409 {"error":"projection base refused: base blob digest mismatch"}` | Refuses tampered base blob before install |
| Unauthenticated recover_current | `POST /lifecycle/cold-recover` | `HTTP 403 {"error":"computer ownership required"}` | Refuses unverified callers |
| API key without computer scope | `POST /lifecycle/cold-recover` | `HTTP 403 {"error":"computer ownership required"}` | Refuses API key lacking `X-Choir-Computer` binding |
| Strict recover_current decoding | `POST /lifecycle/cold-recover` | `HTTP 400 {"error":"idempotency_key is required"}` | Disallows unknown/historical fields |

## 4. Bounded Recovery Work Telemetry

- **Replay Observer Invariants (proven in `TestRestoreTailReceiptBounds`):**
  - Prefix event reads: **0** recovery-critical reads/reductions at or before W.
  - Applied sequence span: strictly $(W, H]$ contiguous. Replay starting at or before $W$ or skipping events fails with `ErrBaseRefused`.
  - Page cursor: first page starts at $W$, subsequent pages advance monotonically. Page cursor before $W$ fails with `ErrBaseRefused`.
- **W=1 -> H=13 Rebuild Telemetry (Node B observed):**
  - Prefix skipped: 0 (genesis base)
  - Events reduced: 13
  - Wall time: 6.39 seconds
  - Blob size: 920,064 bytes
- **W=1 -> H=3 Local Instrumentation Telemetry (`TestInstallVerifiedBaseThenTailReplaysToHead`):**
  - Watermark: sequence 1
  - Target: sequence 3
  - Tail events applied: 2 (sequences 2 and 3)
  - Tail pages fetched: 1
  - Verified witness match: true
  - Idempotent reinstall on crash resume: short-circuits without re-download

## 5. Blocked Prerequisite Receipt (Action 8 / Definition Lines 87-89)

- **ID:** `restore-zero-blocked-prerequisites-2026-09-09`
- **Missing Production Controls:**
  1. No scoped owner CLI/API control exists to inject base download/unpack/install failures into a live computer realization without host-level intervention.
  2. No scoped owner CLI/API control exists to inject mid-flight process interruption into a running computer's tail replay.
  3. No scoped owner CLI command exists for `cold-recover` (reachable only via direct HTTP POST with `X-Choir-Computer` header).
  4. Base publication (`choir-rebuild-base`) requires host access to platform-artifacts disk and cannot be invoked from the public edge API.
- **Disposition:** Per definition instructions, acceptance for automated fault-injection and process-interruption drill on the production boundary is recorded as blocked by missing product controls. Local fault injection and interruption resumption are fully proven in `install_test.go` and `restore_base_test.go`. No unscoped platform/host shell was substituted as a product control.
