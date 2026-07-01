# Mission: Universal Wire Service-as-Appagent v1

## Status

Paradoc v1, created 2026-06-27 12:30 EDT, Boston, MA.
Supersedes v0. Updated after full-stack review and horizontal scaling
discussion.

## Objective

Refactor Universal Wire from centralized runtime methods to platform
appagents running in always-on platform VMs. Scale horizontally across
multiple VMs with work sharding by processor key. Delete the quarantined
centralized service code.

The goal: platform services like Universal Wire run as appagents in
always-on platform VMs, not as code that runs on the host. Multiple VMs
divide the work by source processor key. The reconciler coordinates
cross-shard corpus review.

## Current Architecture

### Single Platform Computer

`internal/vmctl/platform_computer.go`:
- One platform computer: `UniversalWirePlatformOwnerID` =
  `"universal-wire-platform"`, `UniversalWirePlatformDesktopID` =
  `"platform"`, `UniversalWirePlatformVMID` = `"vm-universal-wire-platform"`
- `EnsureUniversalWirePlatformComputer` boots/resumes this single VM
- `WarmUniversalWirePlatformComputer` resumes from stopped/hibernated
- Proxy routes `/api/universal-wire/stories` and
  `/api/universal-wire/live-arrival` to this single computer

### Centralized Runtime Methods

The Wire service is ~5,600 lines of direct methods on the `Runtime` struct:
- `wire_publication.go` (761 lines) — work item lifecycle, trajectory
- `wire_platform_publish.go` (260 lines) — corpusd sync
- `wire_reconciler_debounce.go` (222 lines) — debounced reconciler
- `sourcecycled_web_captures.go` (~300 lines basic) — capture ingestion
- `tools_wire_processor.go` (95 lines) — processor tool
- `wire_synthesis.go` (being deleted in Mission 1)
- `universal_wire.go` (~800 lines after session 2 revert) — stories API,
  edition management

These run on the host Runtime, not in a platform VM. The platform computer
exists but is only used for proxy routing and corpusd publication — the
actual Wire logic runs in the host process.

## Target Architecture

### Multiple Platform VMs with Work Sharding

Instead of one platform computer, run N platform VMs, each owning a shard
of the Universal Wire work. The natural sharding key already exists:

`internal/cycle/ingestion_handoff.go:sourceProcessorKey`:
```
processor:vertical:region:sourceType
```

Examples:
- `processor:global_firehose:global:gdelt` — GDELT global feed
- `processor:tech:us:rss` — US tech RSS
- `processor:politics:eu:telegram` — EU politics Telegram

Each processor key maps to a shard. Shards are assigned to VMs. A VM can
own multiple shards (for low-volume keys) or one shard (for high-volume
keys like GDELT global).

### Shard Assignment

```
shardKey = sourceProcessorKey(item)
vmID = shardRouter.Route(shardKey)
```

The shard router is a consistent-hash ring mapping processor keys to VM
IDs. When a VM is added or removed, only the keys on that VM's portion of
the ring move. This minimizes key redistribution on scaling events.

The router lives in the `OwnershipRegistry` (or a new `ShardRouter` type
that wraps it). It persists shard assignments so that a VM restart doesn't
lose its shard ownership.

### VM Lifecycle

Each platform VM:
1. Boots with a profile: `AgentProfileProcessor` + assigned shards
2. Receives processor dispatches for its shards via channel messages
3. Runs processor agent loops, routes to Texture agents, publishes
4. Reports health to the trajectory supervisor
5. Can be hibernated when idle, resumed when work arrives

The existing `WarmUniversalWirePlatformComputer` pattern generalizes to
multiple VMs. The idle sweeper can hibernate VMs with no pending work;
the dispatch path can resume them when new items arrive for their shards.

### Channel-Native Coordination

Instead of direct runtime method calls:

```text
sourcecycled → shardRouter.Route(key) → channel message to VM
  → VM's processor agent receives message
  → processor routes to Texture agent (same VM or channel to another VM)
  → Texture agent writes article, publishes to corpusd
  → publication event → channel message to reconciler VM
  → reconciler reviews corpus across shards
```

The channel system (`internal/runtime/channels.go`) already supports
inter-agent communication. The `ChannelManager` routes messages between
appagents. For cross-VM communication, the channel manager needs a
network transport (currently in-process only).

### Reconciler as Cross-Shard Coordinator

The reconciler is special: it reviews the corpus across all shards. It
should run on a dedicated VM (or the conductor's VM) and receive
publication events from all processor VMs.

The existing `wirePublishDebouncer` pattern works: batch publications
from multiple VMs, then dispatch one reconciler run. But the debouncer
needs to aggregate across VMs, not within a single Runtime.

### Edition Aggregation

The Wire edition document (`universal-wire/Wire.texture`) is the product
surface — it lists all published articles. Currently, a single Runtime
appends to it. With multiple VMs, edition updates need coordination:

Option A: Each VM publishes its articles to corpusd. A dedicated
edition aggregator (could be the reconciler VM) reads all published
articles and builds the edition document.

Option B: Each VM writes to its own section of the edition document. The
edition is a merge of all VM sections. This requires a merge strategy
(conflict-free replicated data types or last-writer-wins per section).

Option A is simpler and matches the existing pattern. The edition
aggregator runs as a periodic job on the reconciler VM.

### Supervision Protocol

The conductor supervision design
(`docs/design-conductor-supervision-protocol-2026-06-23.md`) defines
read-only observation (Phase 1) and protocol nudge (Phase 2).

For the appagent architecture:
- Each platform VM reports health to the trajectory supervisor
- The supervisor observes: pending work items, publication rate, error
  rate, VM state (active/hibernated/stopped)
- Phase 1: read-only observation, no nudges
- Phase 2: supervisor can restart a stuck VM, rebalance shards, or
  alert the conductor

### Horizontal Scaling Flow

```
1. Sourcecycled cycle completes with N items
2. BuildIngestionHandoff creates M processor requests
3. For each request:
   a. shardRouter.Route(processorKey) → vmID
   b. If VM is hibernated, resume it
   c. Send channel message to VM with ProcessorRequest
4. VM's processor agent runs:
   a. Reviews source items
   b. Records typed decisions (opened_texture, already_covered, etc.)
   c. Routes newsworthy items to Texture agent
   d. Texture agent synthesizes article using gpt-5.5
   e. Publication pipeline publishes to corpusd
   f. Publication event → channel message to reconciler VM
5. Reconciler VM:
   a. Batches publication events (debouncer)
   b. Dispatches reconciler agent run
   c. Reconciler reviews corpus, may update existing articles
   d. Edition aggregator builds/updates Wire edition document
```

### Scaling Events

**Scale up**: add a new VM, assign it shards from the consistent-hash ring.
Keys that move to the new VM are migrated on the next dispatch cycle. No
active migration needed — the old VM finishes in-flight work, the new VM
picks up the next dispatch.

**Scale down**: hibernate a VM. Its shards revert to the remaining VMs on
the ring. In-flight work completes; no new dispatches.

**VM failure**: the supervisor detects the failure (health check timeout).
The VM's shards are reassigned to other VMs. In-flight work items remain
open in the store; the reassigned VM picks them up on its next dispatch.

## What Needs to Be Built

### 1. ShardRouter

A consistent-hash ring mapping processor keys to VM IDs. Persists
assignments. Supports add/remove VM, route key, list shards for VM.

### 2. Multi-VM Platform Computer Management

Generalize `EnsureUniversalWirePlatformComputer` to manage N platform VMs.
Each VM has:
- VM ID (stable, derived from shard assignment)
- Owner ID (same for all: `universal-wire-platform`)
- Desktop ID (shard-specific or shared)
- State (active/hibernated/stopped)

### 3. Cross-VM Channel Transport

The channel system needs a network transport for cross-VM messages.
Options:
- HTTP POST between VM sandboxes (simple, works with existing proxy)
- Shared queue in the store (durable, survives VM restart)
- Hybrid: channel messages are written to the store, VMs poll or get
  notified

The hybrid option is most robust: channel messages are durable work items
in the store, and VMs are notified via the existing wake mechanism
(`textureWakeAfter`). This survives VM restart and doesn't require
network connectivity between VMs.

### 4. Edition Aggregator

A periodic job that reads all published Wire articles and builds/updates
the Wire edition document. Runs on the reconciler VM or a dedicated VM.

### 5. Supervision Protocol Phase 1

Read-only observation of platform VM health. The supervisor reads:
- VM state (active/hibernated/stopped)
- Pending work items per VM
- Publication rate per VM
- Error rate per VM

Reports health verdicts to the conductor. No nudges in Phase 1.

## Checklist

- [ ] Verify Mission 2 (agent pipeline) is settled
- [ ] Design `ShardRouter` type (consistent-hash ring, processor key → VM ID)
- [ ] Implement `ShardRouter` with persistence
- [ ] Generalize platform computer management to N VMs
- [ ] Implement cross-VM channel transport (store-backed hybrid)
- [ ] Move processor dispatch from host Runtime to platform VM
- [ ] Move publication pipeline from host Runtime to platform VM
- [ ] Implement edition aggregator
- [ ] Implement supervision protocol Phase 1 (read-only observation)
- [ ] Delete quarantined centralized service code from host Runtime
- [ ] Update `HandleUniversalWireStories` to read from platform VM
- [ ] Verify repo compiles and tests pass
- [ ] Deploy to staging with 2 VMs
- [ ] Run authenticated staging acceptance: articles still render, sources
      cited, edition shows articles from both VMs

Acceptance: Universal Wire runs as appagents in platform VMs. Sourcecycled
triggers processor dispatch via channel messages to sharded VMs. The
quarantined centralized service code is deleted. Staging product proof
shows articles from multiple VMs in the edition.

## Parallax State

status: proposed

mission conjecture: if Universal Wire is refactored from centralized
runtime methods to platform appagents running in sharded always-on VMs
with channel-native coordination and supervision, then the centralized
service heresy is repaired and the Wire can scale horizontally.

deeper goal (G): Choir as a self-improving mainframe where every platform
service runs as an appagent in a platform VM, scales horizontally by
sharding, and is supervised — no centralized host-level services.

witness/spec (A/S): ShardRouter implementation, multi-VM platform
management, cross-VM channel transport, edition aggregator, supervision
observation records, deleted centralized code, staging product proof
with multiple VMs.

invariants / qualities / domain ramp (I/Q/D): Do not break the agent
pipeline from Mission 2. Do not touch Texture core or O1-O3. Channel
communication uses existing ChannelManager with store-backed transport.
VM lifecycle uses existing vmctl patterns. Domain ramp: local test with
mock VMs → local test with 2 real VMs → staging deploy with 2 VMs →
authenticated staging acceptance.

variant (conjecture descent) V: count conjectures about the appagent
architecture. Current: 6.
- C1: ShardRouter maps processor keys to VM IDs consistently
- C2: Cross-VM channel transport delivers messages durably
- C3: Platform VMs run processor agents that produce articles
- C4: Edition aggregator builds a coherent edition from multiple VMs
- C5: Supervision protocol observes VM health without breaking anything
- C6: Centralized service code can be deleted without regression
Target: 0.

budget: 8-15 passes. This is a significant architectural refactor.

authority / bounds: may modify `vmctl/platform_computer.go`, `channels.go`,
`runtime.go` (appagent registration), `wire_publication.go` (move to VM),
`wire_platform_publish.go` (move to VM), `wire_reconciler_debounce.go`
(move to VM), `sourcecycled_web_captures.go` (dispatch to VM). May
implement ShardRouter, cross-VM transport, edition aggregator, supervision
Phase 1. May deploy to staging. May not touch Texture core, O1-O3, or
the agent pipeline's provider calls.

mutation class / protected surfaces: Orange/Red — refactoring runtime
architecture, changing service coordination, deleting code, adding VM
management. Protected: Texture revision creation, corpusd sync contract,
source entity graph, public API contract (`/api/universal-wire/stories`).

evidence packet: ShardRouter code, multi-VM management code, channel
transport code, edition aggregator code, supervision observation records,
deleted code list, staging commit SHA, CI/deploy status, authenticated
product replay with multiple VMs.

heresy delta: `repaired` for the centralized service pattern heresy.
`discovered` for any new architectural issues.

next move: design the ShardRouter — consistent-hash ring mapping processor
keys to VM IDs, with persistence and add/remove VM support.

ledger file: `docs/mission-universal-wire-service-as-appagent-v1.ledger.md`

version / lineage: v1. Depends on
`docs/mission-universal-wire-agent-pipeline-v1.md`.

settlement: settled when Universal Wire runs as appagents in multiple
platform VMs, centralized service code is deleted, and staging shows
articles from multiple VMs. Open handoff if cross-VM channel transport
cannot be made reliable (document the blocker).

## Open Architecture Questions

1. **Desktop ID per shard or shared?** If each shard has its own desktop
   ID, the proxy can route directly to the owning VM. If shared, the proxy
   routes to any platform VM and the VM forwards internally. Per-shard is
   cleaner but requires more ownership entries.

2. **Texture agent runs on which VM?** The processor VM that routed the
   story, or a dedicated Texture VM? Running on the processor VM is
   simpler (no cross-VM routing for the article). A dedicated Texture VM
   could use a different model (gpt-5.5) and scale independently.

3. **Store access from VMs?** Each VM needs store access for documents,
   revisions, work items. The existing sandbox store proxy pattern should
   work, but needs verification for multi-VM concurrent access.

4. **Reconciler VM or conductor VM?** The reconciler could run on a
   dedicated VM or share the conductor's VM. Dedicated is cleaner for
   scaling; sharing reduces VM count for small deployments.

These questions can be resolved during implementation. The paradoc
doesn't need to answer them now — the ShardRouter design will surface
the right answers.

## Suggested Goal String

```text
Use Parallax on docs/mission-universal-wire-service-as-appagent-v1.md. Mission: refactor Universal Wire from centralized runtime methods to platform appagents in sharded always-on VMs. Build ShardRouter (consistent-hash ring, processor key → VM ID). Generalize platform computer management from 1 VM to N VMs. Implement cross-VM channel transport (store-backed hybrid: channel messages as durable work items, VMs woken via textureWakeAfter). Move processor dispatch, publication, and reconciler from host Runtime to platform VMs. Build edition aggregator. Implement supervision Phase 1 (read-only VM health observation). Delete quarantined centralized service code (wire_publication.go, wire_platform_publish.go, wire_reconciler_debounce.go, sourcecycled synthesis trigger, tools_wire_processor.go). Keep HandleUniversalWireStories as public API, read from platform VM. Do not break agent pipeline from Mission 2. Do not touch Texture core or O1-O3. Budget: 8-15 passes. Exit: settled when Universal Wire runs in multiple platform VMs, centralized code is deleted, and staging shows articles from multiple VMs.
```
