# Plan: Store Consolidation, Host Sandbox Deletion, and corpusd Rename

**Date:** 2026-06-30
**Status:** planning — architecture simplification before launch
**Authority:** user directive — aggressive deletion, no fallbacks, smaller codebase

## Goal

Simplify the data architecture to exactly two stores:

1. **Platform Dolt SQL Server** (`corpusd`, currently `platformd`) — host-level
   Dolt SQL server at `127.0.0.1:13306`, database `platform`. The single
   object store for the platform: published articles, source captures,
   provenance, web captures, edges, publication lifecycle.
2. **Embedded Dolt per VM** — each user/platform VM has its own embedded Dolt
   workspace for private runtime state: run records, agent records, channels,
   events, Texture document drafts (pre-publication), trace/evidence.

Delete everything else. No fallbacks. No SQLite sidecars. No host sandbox
process. No embedded objectgraph.

## What Gets Deleted

### 1. Embedded objectgraph Dolt store (DELETE)

**Current:** `internal/objectgraph/dolt_store.go` + `internal/runtime/objectgraph_runtime.go`
opens a separate embedded Dolt workspace at `<runtime-dir>/objectgraph-dolt/`
inside the VM. Stores web capture objects, source entities, edges.

**Why delete:** The platform Dolt SQL server should be the object store. Web
captures, source entities, and edges belong in the platform corpus, not in a
per-VM embedded sidecar. The runtime should query the platform Dolt server
(via corpusd API or direct MySQL connection) for object graph data.

**Files to delete/modify:**
- `internal/objectgraph/dolt_store.go` — DELETE (DoltStore implementation)
- `internal/objectgraph/sqlite_store.go` — DELETE (SQLiteStore, already dead code on staging)
- `internal/objectgraph/memory_store.go` — DELETE (in-memory store, only used as cache layer)
- `internal/objectgraph/service.go` — DELETE (the Service wrapper)
- `internal/objectgraph/store.go` — DELETE (Store interface)
- `internal/objectgraph/object.go` — DELETE or move to `internal/corpus/`
- `internal/objectgraph/web_capture.go` — DELETE or move to `internal/corpus/`
- `internal/objectgraph/registry.go` — DELETE
- `internal/runtime/objectgraph_runtime.go` — DELETE (runtime's objectgraph init)
- `internal/runtime/sourcecycled_web_captures.go` — REWRITE to write to corpusd instead
- `internal/runtime/universal_wire.go` — REWRITE to read from corpusd instead of objectgraph
- `internal/runtime/wire_synthesis.go` — UPDATE (sources come from corpusd, not objectgraph)

### 2. Sourcecycled SQLite cycle store (DELETE)

**Current:** `internal/cycle/storage.go` uses `modernc.org/sqlite` at
`/var/lib/go-choir/sourcecycled/sourcecycled.db`. Stores: source items, fetch
records, cycle events, processor requests, reconciler requests, poll state.

**Why delete:** Source items are graph objects — they belong in the platform
Dolt server. Cycle events, processor requests, and reconciler requests are
platform-level queue state — they also belong in the platform Dolt server.
The SQLite store is a separate database for the same data domain.

**Migration:** Sourcecycled should write source items and cycle state to the
platform Dolt server (via corpusd API or direct MySQL). The cycle package
becomes a thin client of corpusd, not a standalone SQLite store.

**Files to delete/modify:**
- `internal/cycle/storage.go` — REWRITE to use corpusd/platform Dolt instead of SQLite
- `internal/cycle/web_capture_graph.go` — FOLD into corpusd (or delete, since corpusd handles it)
- `internal/sourcegraph/` — DELETE (fold into cycle or corpusd, per naming rectification)
- `cmd/sourcecycled/main.go` — UPDATE (remove SQLite store, use corpusd for persistence)

### 3. Host sandbox process (DELETE)

**Current:** `go-choir-sandbox.service` runs a sandbox runtime on the host at
`127.0.0.1:8085` (`sandbox-m1`). It's described as a "placeholder" that
"deployed routing fails closed instead of silently landing on."

**Entanglement:**
- Proxy: `PROXY_SANDBOX_URL=http://127.0.0.1:8085` (default, but vmctl routing overrides it)
- Proxy WS: `ws://127.0.0.1:8085` hardcoded for super-console and WS connections
- Maild: `MAILD_RUNTIME_URL=http://127.0.0.1:8085` (hardcoded to host sandbox)
- Gateway: issues credential for `sandbox-m1`

**Why delete:** All runtime work should happen in VMs. The host sandbox is a
bootstrap artifact from before VM infrastructure existed. Maild and proxy WS
should route through vmctl to user VMs, not to a host process.

**Files to delete/modify:**
- `cmd/sandbox/` — KEEP (the binary is used inside VMs), but remove the host systemd service
- `nix/node-b.nix` — DELETE `systemd.services.go-choir-sandbox` block
- `nix/node-b.nix` — UPDATE proxy to not reference `PROXY_SANDBOX_URL` (vmctl only)
- `nix/node-b.nix` — UPDATE maild to route through vmctl or a VM-resolved URL
- `internal/proxy/handlers.go` — UPDATE WS routing to use vmctl-resolved URLs
- `nix/node-b.nix` — DELETE gateway credential issuance for `sandbox-m1`

### 4. platformd → corpusd rename (RENAME)

**Current:** `cmd/platformd/`, `internal/platform/`, env vars `PLATFORMD_*`,
service name `go-choir-platformd`.

**Why rename:** The naming rectification doc (`docs/naming-rectification-2026-06-27.md`)
already plans this. `corpusd` better describes what it does — managing the
published corpus. The vision doc uses `corpus` as the target term.

**Files to rename:**
- `cmd/platformd/` → `cmd/corpusd/`
- `internal/platform/` → `internal/corpus/`
- Code symbols: `Platform*` → `Corpus*`, `platformd` → `corpusd`
- Config/env vars: `PLATFORMD_*` → `CORPUSD_*`, `RUNTIME_PLATFORMD_URL` → `RUNTIME_CORPUSD_URL`
- Nix: `go-choir-platformd` → `go-choir-corpusd`
- Docs: `platformd` → `corpusd`

## What Remains

### 1. Platform Dolt SQL Server (corpusd)

- Host-level `dolt sql-server` at `127.0.0.1:13306`
- Database `platform` (rename to `corpus`?)
- Tables: existing 19+ publication tables + new object graph tables (web captures,
  source entities, edges, source items, cycle events, processor requests)
- Accessed by: corpusd service (HTTP API), sourcecycled (direct MySQL or API),
  runtime inside VMs (via corpusd HTTP API)

### 2. Embedded Dolt per VM

- Each VM has `internal/store.Store` at `/mnt/persistent/runtime/runtime.db`
- Holds: run records, agent records, channels, events, Texture drafts
- NOT the object graph — that's in corpusd
- The runtime queries corpusd for published articles, source captures, and
  object graph data

### 3. Sourcecycled (simplified)

- Fetches sources (RSS/Telegram/GDELT) — unchanged
- Writes source items as graph objects to corpusd (not SQLite)
- Queues processor requests in corpusd (not SQLite)
- Dispatches processor runs to platform VM via vmctl — unchanged
- No local SQLite database

## Migration Path

### Data migration

**Pre-launch, data is not precious.** Aggressive deletion is acceptable.

1. **Platform Dolt server:** Add object graph tables (web captures, source
   entities, edges, source items, cycle events, processor requests) to the
   existing `platform` database. No data migration needed — start fresh.

2. **Platform VM (`vm-universal-wire-platform`):**
   - The VM's embedded Dolt workspace at `/mnt/persistent/runtime/` has
     existing Texture articles, run records, etc.
   - The VM's embedded objectgraph at `objectgraph-dolt/` has web captures.
   - **Action:** Delete the `objectgraph-dolt/` directory. The runtime will
     query corpusd for object graph data instead.
   - **Action:** Keep the main embedded Dolt workspace (run records, Texture
     drafts). These are private to the VM.

3. **User VM (`yusefnathanson@me.com` QA account):**
   - The user's VM has its own embedded Dolt workspace.
   - No objectgraph to migrate (user VMs don't have source captures).
   - **Action:** No changes needed. User VMs continue using embedded Dolt for
     private state. They read published articles from corpusd.

4. **Sourcecycled SQLite (`sourcecycled.db`):**
   - Contains source items, processor requests, cycle events.
   - **Action:** Delete. Start fresh in corpusd. The 61 queued processor
     requests are stale anyway.

5. **Host sandbox (`sandbox-m1`):**
   - Has its own runtime.db with 0 running runs (confirmed via health check).
   - **Action:** Delete the service. Delete the data directory.

### Code migration order

1. **Add object graph tables to corpusd** — extend `internal/platform/store.go`
   with web capture, source entity, edge, source item, cycle event, and
   processor request tables.

2. **Add corpusd API endpoints for object graph** — web capture projection,
   source item listing, object/edge queries.

3. **Rewrite sourcecycled** to use corpusd instead of SQLite — replace
   `cycle.NewStorage()` with a corpusd-backed store.

4. **Rewrite runtime objectgraph queries** to use corpusd API instead of
   embedded objectgraph — update `sourcecycled_web_captures.go`,
   `universal_wire.go`, `wire_synthesis.go`.

5. **Delete objectgraph package** — `internal/objectgraph/` (or move remaining
   types to `internal/corpus/`).

6. **Delete host sandbox service** — remove from nix/node-b.nix, update proxy
   and maild routing.

7. **Rename platformd → corpusd** — code, configs, docs.

8. **Deploy and verify** — fresh platform VM boot, fresh sourcecycled start,
   verify source items flow to corpusd, verify articles appear on wire.

## Risk Assessment

- **Pre-launch, data is not precious.** Aggressive deletion is acceptable.
- **Platform VM disk is 91% full** — deleting the objectgraph-dolt directory
  frees space.
- **Sourcecycled queue (61 items) is stale** — deleting is safe.
- **User VM (yusefnathan@me.com)** — no changes needed, no data loss.
- **Host sandbox** — 0 running runs, not serving production traffic (vmctl
  routing overrides it). Safe to delete.

## References

- `docs/computer-ontology.md` — computer/VM/Dolt architecture
- `docs/naming-rectification-2026-06-27.md` — platformd → corpusd rename plan
- `docs/vision-choir-category-texture-transclusion-v0.md` — corpus as target term
- `docs/choir-architecture-review-next-moves-2026-06-11.md` — architecture review
- `internal/platform/store.go` — corpusd store (Dolt SQL server)
- `internal/objectgraph/` — embedded objectgraph (to be deleted)
- `internal/cycle/storage.go` — sourcecycled SQLite store (to be deleted)
- `nix/node-b.nix` — service configuration
