# Choir Base Product Spec

**Date:** 2026-06-06  
**Status:** implementation spec v0 / execution paused behind source-system landing  
**Research basis:** [choir-base-research-report-2026-06-06.md](choir-base-research-report-2026-06-06.md)  

## Document Role

This is the current product spec for Choir Base. It defines the target
architecture, invariants, sync planes, and product landmarks.

It is not the active implementation mission while source-system streamlining is
in progress. During that pause, use
[news-voice-autoradio-forward-plan-2026-06-06.md](news-voice-autoradio-forward-plan-2026-06-06.md)
for the current forward-looking news/voice/Autoradio work. When Base is
reactivated, author or resume a MissionGradient from the then-current code and
staging state rather than treating this spec as a checklist.

## Decision

Choir Base is the owner-scoped substrate for files, source artifacts, imports,
exports, manifests, native file surfaces, and device sync. It is not a Dropbox
clone and not a standalone editor. It is the durable artifact layer consumed by
VText, Source Service, publications, automatic radio, desktop apps, and future
native clients.

The first Base engineering direction is **Base reconciliation-kernel proof**. This
is not a user-facing local sync product and not the same as deploying Base on
Node B. It means implementing and testing the pure algorithmic core that can
compare `remote`, `local`, and `synced` trees and produce operations,
conflicts, and stuck states.

Execution note: while source-system work owns CI/CD and staging attention, do
not start Base implementation unless explicitly reactivated. Use the interim
window for current source/news/voice/Autoradio docs, captured in
[news-voice-autoradio-forward-plan-2026-06-06.md](news-voice-autoradio-forward-plan-2026-06-06.md).
Wails v3 remains the desktop shell decision, but the shell is secondary until
the reconciliation kernel is credible.

## Product Path

```text
Base reconciliation kernel
-> Wails v3 observer/control pane
-> Base platform integration on Node B
-> Node A/B backup and restore proof
-> macOS File Provider proof
-> full Choir macOS desktop
-> automatic radio and watch-first screenless proof
-> iOS/iPadOS File Provider + radio/control app
-> Windows Cloud Files and Android DocumentsProvider parity
```

The macOS File Provider utility must be the smallest signed app bundle that can
become the full Choir app. It should not be disposable.

## Execution Lanes

Base should progress in two lanes:

```text
Local lane:
  pure model/planner/testkit for reconciliation
  local immutable blob store
  local SQLite or in-memory journal
  Wails v3 observer/control prototype
  no staging dependency

Platform lane:
  canonical API/store integration
  Node B blob storage
  Node A backup/restore
  staging deployment proof
  product-path acceptance
```

During active source-system streamlining, prefer local-lane work. Promote to the
platform lane only when CI/CD and Node B access can be the active main task.

## Sync Planes

"Sync" is not one system in Choir. There are at least four planes:

```text
1. Base artifact sync
   owner-scoped files/source artifacts/blobs/manifests/versions

2. Computer state movement
   VM/OS/runtime/Dolt/source/build state for active and candidate computers

3. Native client sync
   macOS File Provider, Wails desktop, iOS Files, Windows Cloud Files,
   Android DocumentsProvider

4. Host-to-local-computer sync
   Mac host folder or File Provider domain <-> local Choir VM/candidate computer
```

The multi-tenant deployed version is the target topology. Local and
single-tenant versions should be scaled-down projections of it, not separate
architectures.

For `choir.news`, Base artifact sync must be multi-tenant and server
authoritative:

- every object is owner/tenant scoped;
- every mutation carries subject and device identity;
- canonical metadata changes go through the Base journal;
- blobs are immutable and owner/tenant scoped;
- user VMs/candidate computers are clients of Base, not owners of global truth;
- node-admin and desktop bridges cannot mutate storage outside product APIs.

For a local Mac deployment, the same topology collapses onto one machine:

```text
Mac host
  -> local Choir app / Base service
  -> local blob/journal store
  -> local VM/candidate computer
```

The local version may use loopback and local disks, but it should preserve the
same authority shape: Base owns artifact truth; the local VM is a computer that
syncs or imports through Base contracts.

## Apple Virtualization And vmctl

Apple Virtualization is a good local-computer substrate, but it should augment
`vmctl` as a real Darwin VM manager rather than being treated as "sync."

Current shape:

- Node B uses Firecracker/KVM when available.
- macOS currently falls back to host-process sandbox mode for local development.
- `vmctl` already owns lifecycle vocabulary: boot, resolve, fork, publish,
  request worker, stop, hibernate/resume/recover, ownership, warmness, and
  pressure/reclaim policy.

Desired shape:

```text
vmctl lifecycle contract
  -> Firecracker manager on Linux/Node B
  -> Apple Virtualization manager on macOS
  -> host-process fallback only for development diagnostics
```

That unlocks a real local proof:

```text
Mac host File Provider/Base folder
<-> local Base service
<-> Apple Virtualization-backed Choir computer
```

But the VM manager and Base reconciler should remain separate. The VM manager
owns computer lifecycle and isolation. Base owns artifact identity, versioning,
conflict detection, and sync status.

## Base/Desktop Reactivation Posture

Target posture once Base/Desktop is reactivated:

```text
Base reconciliation kernel: build immediately.
News/source service: finish and validate in parallel where already largely built.
Wails v3 desktop shell: scaffold as soon as useful local Base status exists.
Apple Virtualization vmctl path: consider as a parallel local-computer mission,
behind the existing vmctl lifecycle contract.
macOS File Provider: prototype locally with development/testing entitlements,
then prepare Developer ID signing/notarization once the subscription is active.
Base platform integration: promote when source-system CI/CD/staging lane is free.
Voice / AI DJ / iOS v0: pursue as soon as Base/news artifacts can feed Autoradio.
```

This is not a slow serial roadmap. The product landmarks can become parallel
agent missions when their dependencies are real enough. The spec's job is to
preserve topology and authority boundaries while enabling agentic-speed
execution.

## Core Principle

```text
Small scope.
Strong model.
Explicit conflicts.
Legible stuck states.
Recoverable local journal.
Deterministic fault tests before broad UI.
```

The code shape:

```text
Content identity -> immutable bytes -> append-only metadata events
-> derived trees -> pure planner -> operations/conflicts/status
```

The planner is the center. File Provider, Wails, filesystem scans, cloud
storage, auth, repair UI, and future clusters are adapters around it.

## Non-Goals

- Syncing arbitrary home directories in v0.
- Cross-tenant/global deduplication.
- Silent conflict resolution for opaque files.
- Treating RAID as backup.
- Treating browser upload/download as Finder/Files support.
- Treating Wails proof as Base proof.
- Local VMs, local LLMs, or candidate computers in the first Base mission.
- Enterprise SSO/E2EE hardening before the object/key boundaries exist.

## Invariants

- Every artifact is owner-scoped from birth.
- Stable item IDs, not paths, define file/folder identity.
- Blob bytes are immutable and content-verified.
- Metadata changes are append-only journal events.
- Derived trees are rebuildable from journal plus snapshots.
- The sync planner derives actions from `remote`, `local`, and `synced` trees.
- Opaque conflicts preserve both sides.
- Choir-native artifacts may get semantic merge only when the data type supports
  it.
- Every stuck item has a visible state, last error, and repair handle.
- One authoritative sync engine owns a subtree.
- Native clients do not bypass product auth, provenance, journal, or policy.

## Architecture

Proposed packages:

```text
internal/base/model
internal/base/blob
internal/base/journal
internal/base/tree
internal/base/planner
internal/base/status
internal/base/api
internal/base/local
internal/base/testkit
```

Package responsibilities:

- `model`: value types and stable IDs.
- `blob`: immutable byte storage and hash verification.
- `journal`: append-only metadata events.
- `tree`: derive consistent trees from journal events.
- `planner`: pure reconciliation over `remote`, `local`, and `synced` trees.
- `status`: per-item state and repair handles.
- `api`: HTTP endpoints over journal/blob/delta/status.
- `local`: adapters for filesystem scan, Wails bridge, File Provider later.
- `testkit`: deterministic scenarios, fault injection, in-memory stores.

## Data Model

Keep existing `content_items` as a compatibility/read model. Add Base tables
beside it.

```text
base_blobs
  blob_id, owner_id, tenant_id, size_bytes, sha256,
  storage_provider, storage_key, created_at, verified_at

base_items
  item_id, owner_id, tenant_id, parent_item_id, name, kind,
  current_version_id, deleted_at, created_at, updated_at

base_versions
  version_id, item_id, owner_id, tenant_id, blob_id, media_type,
  content_hash, manifest_json, provenance_json,
  created_by_device_id, created_by_subject, created_at

base_events
  event_id, owner_id, tenant_id, item_id, device_id, subject_id,
  event_type, parent_event_id, cursor_seq, payload_json, created_at

base_device_cursors
  owner_id, device_id, last_seen_cursor_seq, last_ack_cursor_seq, updated_at

base_sync_status
  owner_id, device_id, item_id,
  local_version_id, remote_version_id, synced_version_id,
  state, last_error, repair_handle, updated_at
```

Conflict files should be projections of real `base_conflict`/status objects, not
only filename suffixes.

## API v0

Minimum API:

```text
POST /api/base/blobs
POST /api/base/items
GET  /api/base/items/{id}
GET  /api/base/delta?cursor=...
GET  /api/base/items/{id}/status
POST /api/base/repair/preview
```

Later API:

```text
POST /api/base/repair/apply
POST /api/base/local-observations
GET  /api/base/tree
GET  /api/base/conflicts
POST /api/base/conflicts/{id}/resolve
```

## Node A / Node B

Early deployment:

```text
Node B:
  canonical app/API
  Base metadata DB
  RAID-backed local blob store
  journal snapshots
  scrub job

Node A:
  off-node blob replica
  metadata/journal backup
  restore rehearsal target
  experiment/candidate node
```

RAID improves availability after disk failure. It is not backup. Before Base
claims durability beyond alpha, Node A must prove restore from Node B backup
artifacts.

## Dependency Policy

Use almost no new dependencies for Base v0.

Use:

- Go standard library for hashing, IO, JSON, HTTP, filesystem, and tests.
- Existing SQL/Dolt/SQLite paths.
- Existing UUID dependency where random IDs match local patterns.

Defer:

- Wails v3 dependency to the desktop app module.
- File Provider/Cloud Files bindings to native adapter packages.
- Content-defined chunking until whole-file or fixed-chunk proof exists.
- CRDT libraries until Choir-native VText/radio structures require semantic
  merge.

Do not vendor Syncthing, rclone, Nextcloud, ownCloud, Seafile, or git-annex as
the core engine. Borrow patterns, not machinery.

Rule:

```text
If a dependency affects conflict correctness, either keep it out of the core or
make its behavior small enough to model in deterministic tests.
```

## Product Progress Landmarks

These are product landmarks, not mission definitions. Each landmark can become
one or more MissionGradient runs when current code, staging state, and prior
evidence make the next executable shape clear.

### Landmark 1: Base Model Nucleus

Base has stable item/version/blob/event/status objects and can project current
item heads into existing `content_items` paths without breaking VText/source
workflows.

Proof shape:

- owner-scoped item/version/blob records;
- append-only metadata event records;
- compatibility projection to `content_items`;
- focused store/API tests.

### Landmark 2: Reconciliation Kernel

Base has a pure planner that accepts `remote`, `local`, and `synced` trees and
emits operations, explicit conflicts, and stuck states without filesystem,
network, database, UI, or clock dependencies.

Proof shape:

- deterministic planner tests;
- fault scenarios for reorder/drop/crash/lock/corruption;
- no silent winner for opaque conflicts.

### Landmark 3: Delta And Repair API

Base exposes a cursor-addressable delta API, per-item status, and dry-run repair
preview. A user or agent can ask what is not safe yet and why.

Proof shape:

- monotonic cursor replay;
- idempotent duplicate event handling;
- status/repair API tests;
- owner-scoping proof.

### Landmark 4: Blob Durability And Node A/B Recovery

Base stores immutable bytes on Node B and proves Node A can restore metadata and
blob state from backup artifacts. RAID is treated as availability, not backup.

Proof shape:

- hash/size verification;
- blob scrub job;
- off-node backup;
- restore rehearsal evidence.

### Landmark 5: Wails v3 Observer / Node Control Pane

The desktop shell can observe Base sync status and node health through typed Go
services without becoming a privileged mutation bypass.

Proof shape:

- pinned Wails v3 alpha;
- typed bridge call;
- node/base status view;
- packaged macOS app smoke;
- no direct DB/filesystem mutation path.

### Landmark 6: macOS File Provider Proof

A signed macOS app bundle can register a File Provider domain and roundtrip a
small Base-backed subtree through Finder without violating Base authority.

Proof shape:

- list/hydrate/edit/upload one file;
- explicit conflict/tombstone behavior;
- visible placeholder/materialization state;
- exact blocker docs for any Apple API friction.

### Landmark 7: Native Desktop Product

The Mac app becomes the full Choir desktop surface: web desktop, Base status,
node admin, local-service boundary, and later local runtime/candidate controls.

Proof shape:

- auth/session proof;
- WebSocket/EventSource/media proof;
- Base status integration;
- node-control capability boundaries;
- packaged/notarized app path.

### Landmark 8: Radio And Mobile Surfaces

Automatic radio, watch-first screenless use, and iOS/iPadOS File Provider/control
surfaces consume Base artifacts and source manifests without becoming mobile
desktop clones.

Proof shape:

- source-grounded radio queue;
- audio I/O/background/resume proof;
- watch voice control proof;
- iOS file/share/open flow proof.

### Landmark 9: Windows / Android Parity

Windows Cloud Files and Android DocumentsProvider mirror the proven Mac/iOS Base
contract rather than inventing new sync semantics.

Proof shape:

- Explorer placeholder/hydration proof;
- Android SAF provider proof;
- same delta/status/conflict contract;
- parity gap list.

## Standing Test Patterns

Model/planner tests:

- local add vs remote add same path;
- local edit vs remote edit same file;
- local delete vs remote edit;
- local move vs remote edit;
- concurrent folder moves;
- case-only rename on case-insensitive filesystem model;
- duplicate remote event is idempotent;
- missed local event recovered by scan observation;
- crash after blob write before event append;
- crash after event append before status update;
- corrupt local blob detected by hash;
- locked file becomes actionable stuck status;
- projection export does not overwrite original.

API/store tests:

- blob write verifies size/hash;
- event cursor is monotonic and replayable;
- item tree can be rebuilt from events;
- owner scoping prevents cross-owner reads;
- `content_items` compatibility projection remains readable by existing
  VText/source paths.

## Mission Definition Rule

Do not encode executable mission scope in this spec. Before each implementation
run, define a MissionGradient from:

- current repo/staging state;
- current dirty worktree status;
- most recent evidence from prior landmarks;
- active platform constraints;
- the next highest-information product risk.

Each mission should name its real artifact, invariants, value criterion,
homotopy axes, forbidden shortcuts, feedback loops, rollback, and stopping
condition.

First proposed local-lane mission:
[mission-choir-base-reconciliation-kernel-v0.md](mission-choir-base-reconciliation-kernel-v0.md).

## Open Questions

- Should canonical Base metadata live in the existing VText Dolt workspace or a
  separate Base workspace?
- Should v0 use whole-file blobs only, or fixed chunks immediately?
- What exact object maps one Base item to one existing `content_item`?
- Which files/types are explicitly unsupported or ignored in v0?
- What is the first UI for the sync inspector: web desktop, Wails, or CLI?
