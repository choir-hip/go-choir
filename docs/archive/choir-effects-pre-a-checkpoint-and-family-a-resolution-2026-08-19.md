# Family A Remediation and Pre-A Checkpoint Baseline: Technical Report

**Date:** August 19, 2026  
**Author:** Choir Engineering  
**Target Audience:** System Architects, Infrastructure Engineers, and Autonomous Systems Operators  
**Mission Governing Document:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`  
**Staging Environment:** `https://choir.news`  
**Retained Computer:** `computer-03335285269bdba4f94377e56879f9e6` (Realization Epoch 324, Active)

---

## Executive Summary

The Choir supervised self-development effects mission requires an addressable, verified pre-state checkpoint (the "pre-A checkpoint") before authorizing autonomous runtime self-modification (the Solitaire headless API and storage excursion). Checkpoints fail closed unless the target computer satisfies single semantic state authority—meaning the computer's entire VM-local relational state (Dolt) can be reconstructed deterministically from its canonical event tape with zero unaccounted direct-write rows.

This report documents the resolution of three sequential substrate failures that blocked pre-A checkpoint publication:
1. **Family A Alias Scoping & Schema Migration Boot Crash:** Unscoped tables (`texture_document_aliases`, `desktop_workspaces`) and schema migration ordering bugs caused MySQL/Dolt Error 1072 during boot on existing workspaces.
2. **Replay Completeness Drift & Index Name Collisions:** `CREATE INDEX IF NOT EXISTS` name-reuse and leftover un-scoped row twins created schema and content non-equivalence during replay verification.
3. **Proxy and Guest HTTP Write-Deadline Timeouts:** Heavy 40-table Dolt state extractions (~120 seconds) were severed by standard 30-second and 120-second HTTP server write deadlines.

Following the deployment of fixes in commits `0aa1ffb3`, `997f25cb`, `087cf290`, `06ec8f8d`, and `0d8b8920`, replay completeness achieved full equivalence at sequence 3403 with `eligible: true`. Pre-A checkpoint `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` was successfully generated, signed by platform-control, and published on staging. An independent multi-agent consensus panel evaluated the repair slice and returned unanimous **ACCEPT** verdicts.

---

## 1. Problem Statements and Root Cause Analysis

### 1.1 Problem 1: Unscoped Texture Aliases & The EnsureTextureSchema Boot Crash

* **Symptoms:** Following the introduction of computer scoping in commit `0aa1ffb3`, existing guest workspaces failed `EnsureTextureSchema` on startup with `Error 1072: key column 'computer_id' doesn't exist in table`. The guest autoputer service died, causing host proxy route resolutions to time out (HTTP 504 / 60s) for authenticated computer accounts.
* **Root Cause:** `0aa1ffb3` added `computer_id` to `texture_document_aliases` in `textureSchemaDDL` and executed `CREATE INDEX IF NOT EXISTS idx_texture_aliases_doc ON texture_document_aliases(owner_id, computer_id, doc_id)`. Because `CREATE TABLE IF NOT EXISTS` is a no-op on existing workspaces, the live table lacked the `computer_id` column when the index creation executed. The column-addition migration (`ensureTextureColumn`) was positioned after `textureSchemaDDL`, violating migration dependency order.

### 1.2 Problem 2: Index Name Reuse and Desktop Workspace Leftover Twins

* **Symptoms:** After fixing the boot crash in `997f25cb`, replay completeness at sequence 3369 reported `not_equivalent`. Live `SHOW COLUMNS` for `texture_document_aliases` marked `doc_id` with Key `MUL`, whereas a clean replay projection did not. Concurrently, `desktop_workspaces` table hashes diverged between live and replay.
* **Root Causes:**
  1. *Index Name Collision:* The legacy schema had created index `idx_texture_aliases_doc(doc_id)`. When the schema was updated to create `idx_texture_aliases_doc(owner_id, computer_id, doc_id)`, `CREATE INDEX IF NOT EXISTS` detected that an index named `idx_texture_aliases_doc` already existed and silently skipped creation. Fresh replay workspaces received the composite index, while live workspaces retained the single-column index.
  2. *Leftover Row Twins:* In `desktop_workspaces`, scoped writes inserted rows with `computer_id = '<id>'` without removing legacy un-scoped rows where `computer_id = ''`. Replay projections built from events only contained scoped rows, causing hash mismatches.

### 1.3 Problem 3: Checkpoint Route Proxy and Guest Server Timeouts

* **Symptoms:** After achieving `eligible: true` at sequence 3375, executing `choir computer checkpoint` returned HTTP 502 Bad Gateway (`workspace replace authority unavailable`) after 31.8 seconds. When client budgets were widened to 110s, the request failed with an empty 502 after 122.8 seconds, and guest logs recorded `runtime api: json encode error: write tcp ...: i/o timeout`.
* **Root Causes:**
  1. *Proxy Client Routing:* `HandleComputerWorkspaceReplace` forwarded `/lifecycle/checkpoint` through the general-purpose `autoputerHTTP` client (30s timeout) instead of the long-running route client.
  2. *HTTP Server Write Deadlines:* Checkpointing extracts 40 Dolt tables and computes complete schema/table cryptographic hashes, requiring ~120 seconds on staging. Go's standard `http.Server.WriteTimeout` is 120 seconds in `internal/server/server.go`. Both the guest handler and the proxy handler failed to invoke `http.NewResponseController(w).SetWriteDeadline(...)`, causing the TCP connection to terminate when the 120-second server write timer expired.

---

## 2. Architectural Remediations

The table below summarizes the commits that resolved the substrate failures:

| Commit | Scope | Summary of Changes |
| :--- | :--- | :--- |
| `ebce5455` | Reducers & Projection | Added `ProjectOpTextureAlias` to `ProjectionBatch V2`, implemented alias event reducers in `internal/store/project.go`, and added residue import handlers. |
| `0aa1ffb3` | Schema & PKs | Migrated `desktop_workspaces`, `desktop_sessions`, and `texture_document_aliases` to include `computer_id` in composite primary keys. |
| `997f25cb` | Store Bootstrap | Removed `idx_texture_aliases_doc` from static DDL; moved index creation to `bootstrapTexture` after column and PK migrations complete. |
| `087cf290` | Index & Row Cleanup | Implemented `ensureTextureDocumentAliasDocIndex` to drop and recreate mismatched index shapes; deleted `computer_id=''` row twins on scoped desktop writes. |
| `06ec8f8d` | Proxy Client Routing | Routed `/lifecycle/checkpoint`, `/lifecycle/restore`, and `/lifecycle/rematerialize-from-tape` through `replayAutoputerHTTP` (10-minute timeout). |
| `0d8b8920` | Server Write Deadlines | Added dynamic write deadline extensions on long routes in both guest `agentcore` (`api_self_development.go`) and host `proxy` (`computer_lifecycle.go`). |

### 2.1 Idempotent Schema Bootstrap Pattern

To prevent schema regressions, `internal/store/texture.go` enforces a strict three-phase sequence during `bootstrapTexture()`:
1. **Column Verification (`ensureTextureColumn`):** Queries `information_schema.columns`. Issues `ALTER TABLE ... ADD COLUMN` only if the column is absent.
2. **Primary Key Alignment (`ensureTextureDocumentAliasesPrimaryKey`):** Inspects `information_schema.table_constraints`. If the primary key does not match `["owner_id", "computer_id", "source_path"]`, it drops and recreates the key.
3. **Index Inspection & Reshaping (`ensureTextureDocumentAliasDocIndex`):** Queries `information_schema.statistics` for `idx_texture_aliases_doc`. If the column sequence is not `["owner_id", "computer_id", "doc_id"]`, it executes `DROP INDEX` followed by `CREATE INDEX IF NOT EXISTS`.

### 2.2 Response Controller Deadline Extension Pattern

For routes with non-deterministic or heavy execution times (e.g. Dolt state extraction), handlers extend their write deadlines dynamically using Go 1.20+ `http.ResponseController`:

```go
// internal/agentcore/api_self_development.go
func extendReplayCompletenessGuestWriteDeadline(w http.ResponseWriter) {
    _ = http.NewResponseController(w).SetWriteDeadline(
        time.Now().Add(replayCompletenessGuestTimeout + replayCompletenessGuestWriteGrace),
    )
}
```

```go
// internal/proxy/computer_lifecycle.go
if client != nil && client.Timeout > 0 {
    _ = http.NewResponseController(w).SetWriteDeadline(
        time.Now().Add(client.Timeout + 30*time.Second),
    )
}
```

This pattern preserves fail-fast behavior (30s) on interactive endpoints while allowing long-running maintenance routes to run for up to 10 minutes without dropping connections.

---

## 3. Verification and Evidence

### 3.1 Replay Completeness Verification

Replay completeness was executed on staging against `computer-03335285269bdba4f94377e56879f9e6` using the production CLI:

```bash
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 15m
```

**Observed Results:**
* **Schema Version:** 3
* **Sequence:** 3403
* **Live Event Head:** `515b1f76aa4f820c239f94508906de6f7aa9e1fd1acb7867154f8fdfe41fc154`
* **Replay Event Head:** `515b1f76aa4f820c239f94508906de6f7aa9e1fd1acb7867154f8fdfe41fc154`
* **Status:** `equivalent` (0 table differences across 40 Dolt tables)
* **Replay Eligibility:** `eligible: true` (`manifest_version: 2`)
* **Run Memory Matching:** 1,083 / 1,083 entries matched (0 live-only, 0 replay-only, 0 differing)

### 3.2 Pre-A Checkpoint Publication

With eligibility established, the pre-A baseline checkpoint was taken via the product API:

```bash
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer checkpoint \
  --computer computer-03335285269bdba4f94377e56879f9e6
```

**Published Checkpoint Receipt:**
* **Checkpoint Digest:** `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`
* **Release Digest:** `7482d7f0a2c5b55d34ce6396f4e49eed48f08042913ca158a77a4be0f6cb20b2`
* **Content Root Hash:** `c302f6d9e570f8755936be9c178d9a8e16ccf417551a5181b5d8be5a0637c903`
* **Dolt Head Commit:** `ho5aeocad9fplm8q5eskfaqkj98cf8s5`
* **Frontend Identity Digest:** `00ec37261b45592ec84d1339f99d1b6c7bebf1d4fbafa131b3b0089c8e8bf643`
* **Signer Domain:** `platform-control` (Key ID: `868f96cca8726f99`)
* **Issued At:** `2026-08-19T12:23:14.107068167Z`
* **Signature:** `yMfipG9dn+T0KyjdlhP3Lgfisy2hM49ZgqEJC0YEIX+Z19McjmslNsJtPh3kQ0XCI8qYwRFlpd4U/ZtyW8k4DA==`

---

## 4. Multi-Agent Consensus Adjudication

An independent agentic consensus panel was convened to evaluate the repair slice (`0aa1ffb3` through `0d8b8920`).

### 4.1 Panel Summary

| Panelist | Model / Backend | Verdict | Confidence | Key Finding |
| :--- | :--- | :---: | :---: | :--- |
| **Devin** | `swe-1-6-slow` | **ACCEPT** | HIGH | Migration ordering and index inspection prevent Error 1072; route segregation maintains fast fail-closed budgets. |
| **Gemini 3.7** | `gemini-3.7-flash` (thinking high) | **ACCEPT** | HIGH | Full 40-table Dolt equivalence at sequence 3403 confirms complete event-derivability; pre-A fence is satisfied. |

### 4.2 Consensus Conclusions

1. **Schema Resilience:** The defensive inspection pattern (`information_schema` check before `ALTER`/`CREATE`) safely handles fresh and legacy workspaces.
2. **Timeout Boundaries:** Scaling `SetWriteDeadline` only when client timeout is non-zero preserves fail-fast boundaries on standard user traffic.
3. **Clear Authority Boundary:** The pre-A checkpoint satisfies the baseline restoration requirement. Super start, Solitaire capsule execution, multi-agent consensus policy evaluation, and outbox email remain unpaid and must not be marked complete without live trajectory proof.

---

## 5. Residual Risks and Next Actions

### 5.1 Residual Risks

1. **Guest Capability Token Expiry:** Background capability tokens have a 5-minute TTL. While initial request authorization is sufficient for single-step extractions, complex multi-phase maintenance tasks must ensure token freshness before initiating long operations.
2. **Concurrent Dolt Access:** Dolt workspace extraction requires table locks. Checkpointing must remain serialized and single-tenant with respect to active agent writes.

### 5.2 Next Actions

1. Durably record the pre-A checkpoint receipt in `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`.
2. Keep self-development effects **OFF** (`propose_only generation 1`).
3. Prepare the Solitaire candidate specification and decision-policy manifests for the subsequent supervised execution slice.
