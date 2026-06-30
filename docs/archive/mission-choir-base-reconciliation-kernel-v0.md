# MissionGradient v0: Choir Base Reconciliation Kernel

**Status:** proposed / deferred until source-system landing unless explicitly reactivated  
**Created:** 2026-06-06  
**Primary spec:** [choir-base-product-spec-2026-06-06.md](choir-base-product-spec-2026-06-06.md)  
**Research basis:** [choir-base-research-report-2026-06-06.md](choir-base-research-report-2026-06-06.md)  
**Current platform context:** source-system streamlining is the active CI/CD and
staging lane. Do not execute this mission while that remains true unless the
owner explicitly reactivates Base work. If reactivated, this mission is
local-code-first and must not require Node B, staging deployment, Apple
Developer signing, or File Provider production entitlements.

## Document Role

This is a proposed deferred MissionGradient. It exists so Base work can resume
quickly when the owner reactivates it. It is not the current active mission.

Current planning focus while this mission is deferred:
[news-voice-autoradio-forward-plan-2026-06-06.md](news-voice-autoradio-forward-plan-2026-06-06.md).

Before execution, refresh the belief state against the then-current codebase,
source-system status, CI/CD lane, and staging/Node B availability.

## One-Line Goal String

```text
/goal Run docs/mission-choir-base-reconciliation-kernel-v0.md as a local-only MissionGradient mission: create the Choir Base reconciliation kernel as a production-shaped Go substrate, not a deployed platform feature and not a local sync product. Build the smallest real version of internal/base model, planner, tree/status/testkit, and optional local blob/journal scaffolding needed to prove three-tree reconciliation over remote/local/synced state with deterministic fault tests. Preserve owner-scoped ContentItem compatibility, avoid Node B/staging/File Provider/Wails production dependencies, avoid vendoring external sync engines, and stop only when the kernel has named evidence for convergence-or-explicit-conflict behavior plus a checkpoint explaining what should become the later platform, Wails, File Provider, or host-to-VM mission.
```

## Mission Frame

Choir Base is the owner-scoped artifact substrate for files, source artifacts,
imports, exports, manifests, native file surfaces, and device sync. The hardest
part is not the desktop shell. It is making sync trustworthy enough that it
never silently loses user work.

The current product constraint is that source-system cleanup owns the active
staging/CI/CD lane. Base should therefore begin with local-only code evidence:
pure model, reconciliation, deterministic tests, and local scaffolding that can
later deform into the platform implementation without crossing a trust-boundary
cliff.

This mission is a first executable slice, not a slow roadmap. It deliberately
keeps the next mutation radius small so other agents can quickly follow with
Wails, File Provider, news, voice, iOS, Autoradio, and host-to-VM work once the
reconciliation kernel has enough shape to compose with them.

## Real Artifact

The artifact is a Base reconciliation kernel:

```text
Base value model
-> immutable blob/version/event/status vocabulary
-> remote/local/synced tree representation
-> pure planner
-> deterministic scenario/fault testkit
-> local test evidence that conflicts are explicit and convergence is explainable
```

The artifact is not:

- a deployed `/api/base` product;
- a Node B blob store;
- a Wails desktop app;
- a macOS File Provider extension;
- a Dropbox/Syncthing/rclone wrapper;
- a local-only toy with different authority semantics from the future platform;
- host-to-VM file sync.

## Clarification: "Local-Only" Does Not Mean Local Sync Product

This mission uses "local-only" in the test/proof sense:

```text
one Go process
in-memory or temp local stores
simulated remote/local/synced replicas
deterministic scenarios
no Node B, no deployed API, no real VM boundary
```

That is not a user-facing local sync feature. It is the reconciliation circuit
that a later host-to-VM, File Provider, or platform sync path can call.

Host-to-local-VM sync is a meaningful future proof, but it is a separate
mission because it adds real process/VM/filesystem boundaries:

```text
Mac host folder or File Provider domain
<-> local Choir desktop app / Base client
<-> local candidate computer or sandbox VM
```

The current repo already has `vmctl` and a Firecracker-backed VM manager path,
with host-process fallback for local development. On macOS, Firecracker is not
the normal local path; `vmctl` falls back to host-process sandbox mode unless a
real VM manager is available. Therefore this mission must not claim that Choir
already runs full local VMs on the Mac.

A later Apple Virtualization mission should add a Darwin VM manager behind the
existing `vmctl` lifecycle contract. That mission should prove local computer
isolation and host-to-VM artifact flow. This mission should only provide the
Base reconciliation logic that such a flow will need.

## Invariants

- Owner scope exists in the model from the first code path.
- Stable item IDs, not paths, define file/folder identity.
- Blob content is immutable and hash-addressable in the model.
- Metadata changes are journal-shaped even if the first implementation uses an
  in-memory or local-only event source.
- The planner is pure: no filesystem, network, database, UI, wall clock, random
  source, or staging dependency.
- The planner derives actions from `remote`, `local`, and `synced` trees.
- Opaque conflicts preserve both sides.
- Stuck/non-converged states are represented as product states, not only test
  failures or log lines.
- Existing `ContentItem` semantics remain respected; do not create a parallel
  source-artifact ontology.
- No mission evidence may claim deployed/platform behavior.

## Value Criterion

Move uphill by minimizing:

```text
sync ambiguity
+ invalid tree states
+ path-as-identity assumptions
+ timestamp dependence
+ silent conflict winners
+ side effects inside the planner
+ dependency surface in correctness code
+ distance from current ContentItem substrate
+ future platform rewrite required
```

subject to the invariants above.

The mission gains value when a later Node B/API/File Provider mission can reuse
the reconciliation kernel rather than reinterpret its results.

## Quality Gradient

Target quality: **solid**.

Solid means:

- package boundaries are boring and obvious;
- model names match the spec;
- planner tests read like sync stories, not implementation trivia;
- failure cases produce explicit conflict/status values;
- no external sync engine is vendored;
- no local-only shortcut would need deletion before platform or host-to-VM work;
- docs name exactly what was proven and what remains unproven.

Excellent means:

- the planner can be fuzzed or table-driven with compact scenario fixtures;
- actions are idempotent enough to support retry reasoning;
- conflict/status values are suitable for a future sync inspector UI;
- ContentItem compatibility is represented by an adapter or mapping design, not
  hand-waved;
- the final checkpoint cleanly seeds the next platform MissionGradient.

Substandard work:

- building API/UI first because it is more visible;
- using paths or mtimes as the central truth;
- producing only happy-path add/edit/delete tests;
- hiding conflicts behind "last writer wins";
- importing Syncthing/rclone/Nextcloud concepts wholesale without modeling
  their semantics;
- claiming readiness for File Provider or Node B without reconciliation evidence.

## Belief State

Current beliefs:

- `content_items` already provide an owner-scoped source-artifact seed.
- Base should grow beside and eventually under `content_items`, not replace it
  abruptly.
- The pure planner is the highest-value local artifact because it does not
  require Node B, staging, signing, or UI proof.
- Whole-file blob identity is enough for the first reconciliation kernel; chunking can
  come after the model proves correctness.
- Wails v3 is promising, but should wait until the kernel has useful state to
  observe.

Main uncertainties:

- Exact Go package boundaries that fit this repo's store/runtime organization.
- Whether Base metadata should eventually live in the VText Dolt workspace or a
  separate Base workspace.
- The minimal `ContentItem` compatibility adapter that preserves current
  VText/source behavior.
- How expressive the first tree/action/status model must be to avoid a rewrite
  before File Provider integration.

Highest-impact uncertainty:

```text
Can the local planner model enough real sync failure cases while staying pure,
small, and reusable by the later platform/File Provider paths?
```

Next observation that reduces uncertainty:

Read the current `ContentItem` store/runtime tests, then implement or sketch the
smallest `internal/base` model and scenario table that can express concurrent
local/remote edits, deletes, moves, and conflicts.

## Homotopy Parameters

Increase realism along these axes without changing topology:

- in-memory tree fixtures -> local SQLite/test journal -> platform SQL/Dolt
  journal;
- whole-file blob refs -> fixed chunks -> content-defined chunks;
- local-only planner tests -> local API tests -> host-to-VM proof -> staging API acceptance;
- synthetic local observations -> filesystem scan adapter -> File Provider
  adapter;
- CLI/test evidence -> Wails observer -> Finder/File Provider user proof;
- single owner -> owner plus tenant/org/device/subject;
- opaque file conflicts -> semantic merge for Choir-native artifacts.

## Receding-Horizon Control

Work in short loops:

1. Inspect current model/store/runtime code.
2. Define the next smallest model or planner shape.
3. Add a focused test scenario before or with implementation.
4. Run the narrow test.
5. Update belief state when the code shape disagrees with the spec.
6. Continue only if the next move still strengthens the reconciliation kernel.

If implementation pressure pulls toward API, UI, File Provider, Node B, or
Autoradio, stop and reparameterize. That is a different mission.

## Dense Feedback

Required local feedback:

- focused Go tests for `internal/base/...`;
- planner scenario tests covering convergence and explicit conflicts;
- status/action snapshots in test failures;
- `go test` commands run through the repo dev shell when native dependencies
  are involved;
- final `git status --short` with dirty paths classified.

Useful optional feedback:

- property/fuzz-style planner tests if the first table tests are stable;
- local benchmark only if planner complexity becomes questionable;
- a tiny CLI/debug printer for scenarios if it materially improves diagnosis.

No staging/browser proof is required for this local mission.

## Forbidden Shortcuts

- Do not deploy Base to Node B.
- Do not modify staging acceptance flows for Base.
- Do not start real File Provider sync.
- Do not require Apple Developer certificates or notarization.
- Do not build Autoradio in this mission.
- Do not vendor Syncthing, rclone, Nextcloud, ownCloud, Seafile, or git-annex as
  the core sync engine.
- Do not make paths or mtimes the only identity/version truth.
- Do not use "last writer wins" for opaque conflicts.
- Do not call reconciliation-kernel tests proof of cloud/platform durability.
- Do not weaken current source-system work or CI/CD focus to land Base.

## Rollback Policy

This mission should be easy to roll back because it is local-code-first:

- keep Base code under `internal/base/...`;
- avoid touching existing source/VText behavior except explicit compatibility
  adapters/tests;
- do not alter production routes unless a later mission authorizes platform
  integration;
- if the model shape is wrong, remove or replace the new local package without
  migrating live data.

If any existing VText/source test fails because of Base code, treat it as an
invariant issue and either fix the compatibility boundary or stop with exact
evidence.

## Learning Side-Channel

Record durable learnings in this mission doc or a dated checkpoint section:

- model names that survived tests;
- failure scenarios that changed the design;
- rejected abstractions/dependencies and why;
- what must be true before the Node B platform mission;
- what the Autoradio path will need from Base.

Do not put bulky command logs in the spec. Keep detailed evidence in test names,
commit messages, or a small evidence section if needed.

## Stopping Condition

The mission is complete only when local evidence shows:

- a Base model package exists or a precise blocker explains why it should not;
- a pure planner can represent `remote`, `local`, and `synced` trees;
- tests cover at least:
  - local add vs remote add same path;
  - local edit vs remote edit same file;
  - local delete vs remote edit;
  - local move vs remote edit;
  - duplicate remote event idempotence;
  - corrupt/locked local item becoming stuck status or explicit conflict;
- opaque conflicts preserve both sides;
- existing `ContentItem` compatibility is either implemented or specified with
  exact code refs and next test;
- no Node B/staging/File Provider/Apple signing claims are made;
- the mission doc has a checkpoint that states the next executable mission:
  likely platform Base integration, Wails observer, or File Provider proof,
  depending on source-system readiness.

If only part of this lands, report `checkpoint_incomplete`, not complete.

## Run Checkpoint & Resumption State

```text
status: proposed
last checkpoint: mission authored before local implementation
current artifact state: docs/spec/research exist; Base code not started here
what shipped: nothing
what was proven: research/spec direction only
unproven or partial claims: local planner, Base model, ContentItem adapter
belief-state changes: local lane should precede Node B/platform lane
remaining error field: package boundary, data model minimality, planner coverage
highest-impact remaining uncertainty: smallest pure planner that survives real sync scenarios
next executable probe: inspect ContentItem/store tests and create internal/base model/planner/testkit skeleton with focused tests
suggested resume goal string: Run docs/mission-choir-base-reconciliation-kernel-v0.md as a local-only MissionGradient mission and satisfy its stopping condition without Node B/staging/File Provider dependencies.
evidence artifact refs: docs/choir-base-product-spec-2026-06-06.md; docs/choir-base-research-report-2026-06-06.md
rollback refs: new local files under internal/base/... should be removable until platform integration begins
```
