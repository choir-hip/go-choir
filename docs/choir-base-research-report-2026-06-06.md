# Choir Base Research Report

**Date:** 2026-06-06  
**Status:** research checkpoint  
**Source docs:** [vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md), [source-external-data-publication.md](source-external-data-publication.md), [news-econ-publishing-synthesis-2026-06-04.md](news-econ-publishing-synthesis-2026-06-04.md), [project-goals.md](project-goals.md)  

## How To Use This Report

This document is a research ledger, not the current executable plan. It
preserves the reasoning, sources, rejected paths, and earlier build slices that
led to the current specs.

Current controlling docs:

- [choir-base-product-spec-2026-06-06.md](choir-base-product-spec-2026-06-06.md)
  for the current Choir Base product architecture and paused execution posture.
- [mission-choir-base-reconciliation-kernel-v0.md](mission-choir-base-reconciliation-kernel-v0.md)
  for the proposed but deferred Base reconciliation mission.
- [news-voice-autoradio-forward-plan-2026-06-06.md](news-voice-autoradio-forward-plan-2026-06-06.md)
  for the current planning focus while source-system work is active.

Sections below that say "first build," "recommended first build slice," "spike,"
or "mission" should be read as historical research outputs unless the current
spec or a newly authored MissionGradient reactivates them.

## Initial Research Conclusion

Choir Base should be the owner-scoped artifact substrate for files, source
artifacts, imports, exports, manifests, and device sync. It should not be a
separate Dropbox clone, a separate document editor, or a local-only sync script.
It is the durable base layer that VText, Source Service, publications, radio,
desktop apps, and future native clients all consume.

The best product path is:

```text
Base service nucleus
-> macOS File Provider app shell
-> full Choir macOS desktop with local candidate computers where feasible
-> automatic radio service
-> watch-first screenless radio proof
-> iOS/iPadOS File Provider + radio app
-> Windows and Android parity
```

The Mac "file provider utility" should not be a disposable side product. It
should be the smallest signed macOS app bundle that can later become the full
Choir app: File Provider extension first, web desktop shell second, local
computer runtime third. This preserves topology and avoids building a utility
that must be replaced.

## Worktree Safety Note

This report was authored from `/Users/wiz/.codex/worktrees/5b24/go-choir`, a
separate worktree from `/Users/wiz/go-choir`. At the start of this planning
turn, this worktree was clean and detached at `93d9f819`, the same commit as
`origin/main`. Work here does not edit the other running agent's working tree.
The only coordination risk is later Git-level conflict if two agents push or
merge changes to the same branch or same docs.

## Research Findings

Apple File Provider is the correct native integration point for Finder and
Files-grade behavior. Apple's File Provider documentation describes it as an
extension other apps use to access files and folders managed by an app and
synced with remote storage. The replicated model is available on macOS 11+ and
iOS 16+, with the system managing local copies while the extension syncs local
and remote state. Apple's replicated extension docs require adopting
`NSFileProviderReplicatedExtension` and `NSFileProviderEnumerating`. Apple's
synchronization docs distinguish dataless and materialized items, working-set
enumeration, and signaling updates through `NSFileProviderManager`.

Apple's File Provider updates matter for product direction. The June 2024
updates added support for syncing known Desktop and Documents folders and
storing domains on eligible external volumes. That suggests Choir Base should
initially register its own domain, then later consider optional Desktop and
Documents capture only after trust, conflict handling, and retention are solid.

iOS can also use File Provider, but a lighter "local documents visible in Files"
mode is different from cloud provider behavior. Apple documents local sharing
through `UISupportsDocumentBrowser`, `UIFileSharingEnabled`, and
`LSSupportsOpeningDocumentsInPlace`; that is useful for an early container app,
but not a real sync provider.

Windows parity should use the Cloud Files API / Cloud Filter API. Microsoft
documents the Cloud Files API as the Windows 10 version 1709+ support layer for
cloud sync engines, placeholder files, and File Explorer integration.
`CfRegisterSyncRoot` registers a sync root and its policies; hydration policy
controls how placeholder files become local content.

Android parity should use Storage Access Framework provider semantics rather
than broad filesystem access. Android's docs state that cloud or local storage
services participate by implementing a `DocumentsProvider`, and that SAF lets
users browse documents from multiple providers through a standard picker.

watchOS is useful as a radio forcing function. Apple's background audio docs
require the Audio Background Mode, an activated audio session, and a Bluetooth
audio route for long-form audio. This makes Apple Watch a good proof target for
screenless automatic radio, but not a substitute for iOS app infrastructure.

Reference links:

- Apple File Provider: https://developer.apple.com/documentation/fileprovider
- Apple Replicated File Provider: https://developer.apple.com/documentation/fileprovider/replicated-file-provider-extension
- Apple File Provider sync: https://developer.apple.com/documentation/FileProvider/synchronizing-the-file-provider-extension
- Apple File Provider updates: https://developer.apple.com/documentation/Updates/FileProvider
- Apple watchOS background audio: https://developer.apple.com/documentation/watchkit/playing-background-audio
- Microsoft Cloud Sync Engines: https://learn.microsoft.com/en-us/windows/win32/cfapi/cloud-files-api-portal
- Microsoft Cloud Filter API: https://learn.microsoft.com/en-us/windows/win32/api/_cloudapi/
- Microsoft `CfRegisterSyncRoot`: https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfregistersyncroot
- Android Storage Access Framework: https://developer.android.com/guide/topics/providers/document-provider
- Android `DocumentsProvider`: https://developer.android.com/reference/android/provider/DocumentsProvider.html

## Product Thesis

Choir Base is the user's durable source/content ground:

```text
files, URLs, feeds, imports, exports, generated artifacts, source snapshots
  -> immutable blobs and owner-scoped ContentItems
  -> versions, manifests, selectors, indexes, policies, and lineage
  -> VText, publication, radio, agents, desktop apps, and native file clients
```

The name "Base" should mean:

- owner-scoped content lives somewhere durable;
- every artifact has identity, hash, provenance, and policy;
- imports preserve originals and create semantic projections;
- exports produce new artifacts with embedded provenance;
- source/news/radio uses exact artifacts, not prompt-only summaries;
- native clients expose the same artifact graph through OS file surfaces.

## Non-Goals

Do not start with:

- a separate local-only sync system that cannot become the cloud-backed service;
- a web-only file manager and call it Finder/Files integration;
- a full local VM product before the Base sync and identity contracts exist;
- an iOS desktop clone that will not survive App Store review or mobile UX;
- radio as a playlist UI without source manifests and listen-state lineage;
- Windows/Android before the Apple path has proven the service contract.

## Required System Objects

### ContentItem v2

The current `ContentItem` is the right nucleus, but Choir Base needs to extend
it from "source artifact row" into "file/source artifact identity":

- `content_id`
- `owner_id`
- `source_type`
- `media_type`
- `app_hint`
- `display_name`
- `canonical_path`
- `source_url`
- `canonical_url`
- `current_version_id`
- `content_hash`
- `size_bytes`
- `blob_ref`
- `text_content_ref` or extracted text ref
- `metadata`
- `provenance`
- `created_at`
- `updated_at`
- `deleted_at`
- `retention_policy`
- `sharing_policy`

The current fields can continue to serve existing VText/source paths while the
new fields are added behind compatible APIs.

### Blob

Immutable bytes keyed by hash:

- hash and algorithm;
- media type and sniffed type;
- size;
- encryption/materialization state;
- malware/macro scan state;
- storage backend ref;
- created/imported timestamps.

### Version

Each file-like item has version lineage:

- `version_id`
- `content_id`
- parent version IDs;
- blob hash/ref;
- metadata hash;
- actor/device/run identity;
- created timestamp;
- conflict group when needed;
- import/export relation when produced from another artifact.

### Manifest

Manifests are first-class, not incidental JSON:

- import manifest;
- export manifest;
- source manifest;
- asset manifest;
- radio issue/listen manifest;
- sync manifest;
- conflict manifest;
- source selector manifest.

### Device

Native clients need explicit device identity:

- device ID;
- platform;
- app build;
- registered domains/sync roots;
- auth/session state;
- cursor state;
- capabilities;
- last health report.

### Sync Cursor

Sync should be cursor-based and replayable:

- per-owner delta cursor;
- per-device acknowledgement;
- tombstones;
- move/rename semantics;
- conflict records;
- hydration/materialization state;
- backoff and retry state.

## Base Service API Shape

Minimum service boundary:

```text
POST   /api/base/devices
GET    /api/base/items?cursor=...
POST   /api/base/items
GET    /api/base/items/{content_id}
PATCH  /api/base/items/{content_id}
DELETE /api/base/items/{content_id}

POST   /api/base/uploads
PUT    /api/base/uploads/{upload_id}/parts/{part}
POST   /api/base/uploads/{upload_id}/complete
GET    /api/base/blobs/{hash}

GET    /api/base/deltas?cursor=...
POST   /api/base/deltas/ack
POST   /api/base/conflicts/{conflict_id}/resolve

POST   /api/base/imports
POST   /api/base/exports
GET    /api/base/manifests/{manifest_id}
GET    /api/base/search
```

The existing `/api/content/items` and `/api/content/import-url` APIs should
remain as compatibility/product-path endpoints. Choir Base can initially wrap
or expand them, then promote `/api/base/*` once the sync contract is real.

## macOS Client Spec

The first native client should be a signed macOS app bundle with:

- login/session pairing to Choir cloud or self-hosted single-tenant endpoint;
- an `NSFileProviderReplicatedExtension`;
- one default Choir Base domain in Finder;
- item enumeration from `/api/base/deltas`;
- fetch contents by blob/version;
- upload local edits as new versions;
- tombstone, rename, move, and conflict mapping;
- menu bar or minimal settings window for account, sync status, pause/resume,
  logs, and endpoint selection;
- diagnostic export that never includes secrets by default.

The v0 UI can be extremely small. The product proof is Finder behavior:

- file appears as dataless placeholder;
- opening hydrates bytes;
- editing uploads a new version;
- remote update appears in Finder;
- conflict creates an explicit conflict artifact, not silent overwrite;
- VText can import/open the same ContentItem lineage.

This app should be structured so the full Choir desktop can be added into the
same bundle later, likely as a native shell around the web desktop plus local
runtime services.

## Full macOS Choir Desktop

The full Mac app should include:

- the File Provider extension from v0;
- the Choir web desktop as a native desktop surface;
- local auth/session management;
- local cache and offline Base browsing;
- optional local runtime for personal candidate computers;
- local VM/container provider where hardware and OS policy allow it;
- cloud fallback for heavier workers, shared model/search calls, and platform
  promotion;
- AppChangePackage export/import for local app/runtime changes;
- explicit owner control over whether local artifacts sync to cloud.

Local computers should be introduced as a capability-gated runtime, not as a
precondition for the Mac app. The first full Mac desktop can still use Choir
cloud computers while proving that Base, desktop state, VText, and radio feel
native.

## News And Automatic Radio Dependency

Automatic radio depends on the source system being trustworthy. The news system
should land as a source ledger and issue manifest substrate, not as a standalone
newspaper daemon.

Required before serious AI DJ work:

- source registry with policy and standing;
- fetch records with raw snapshot hashes;
- cleaned source items with stable IDs;
- issue/event manifests with exact source refs;
- VText/source entity handoff;
- listen queue manifest;
- user feedback and skip/save state;
- voice prompt route into conductor;
- radio run evidence that can be reviewed as VText.

The "AI DJ" is then a rendering/orchestration layer over source manifests:

```text
source ledger
-> issue manifest
-> radio program plan
-> narration script with source refs
-> TTS/audio segments
-> listen state and user voice interrupts
-> VText/source follow-up artifacts
```

## Watch-First Radio Proof

Apple Watch should be used to force the product to work screenlessly. The watch
version should not try to expose the desktop. It should prove:

- start/resume automatic radio;
- pause/skip/save;
- "why am I hearing this?" source explanation;
- voice input routed to conductor;
- short spoken answer or program adjustment;
- handoff to iPhone/Mac/VText for deeper reading;
- background audio survives wrist-down behavior within watchOS constraints.

Because watchOS long-form audio has route and background constraints, the first
watch proof likely needs a minimal paired iOS app before the full iOS File
Provider app is ready.

## iOS And iPadOS App

The iOS app should combine:

- File Provider extension for Files integration;
- automatic radio and watch pairing;
- share extension / open-in-place import;
- VText reading and lightweight review;
- source capture from URLs/files;
- cloud computer control, not local desktop virtualization.

For App Store and UX reasons, the iOS app should not present itself as a full
desktop VM product. It should be the mobile Base/radio/control surface for the
user's Choir cloud or self-hosted computer.

## Windows And Android

Windows should shadow the Mac architecture:

- signed desktop app;
- Cloud Files API sync root;
- Explorer placeholders;
- same Base sync protocol;
- web desktop shell;
- later local runtime provider where feasible.

Android should shadow the iOS architecture:

- `DocumentsProvider` for Storage Access Framework;
- automatic radio;
- share/open flows;
- cloud computer control;
- no broad filesystem or desktop clone assumptions.

Agents can build these toward spec and parity after the Mac/iOS service contract
has proven enough behavior.

## Pre-Mission Cognitive Transform Review

Current uncertainty:

The Wails v3 decision is directionally right and reversible. The mission can
still fail if it treats the shell as the hard part. The dominant product risk is
Base sync correctness: if sync works 99% of the time, users will still lose
trust. Choir does not have platform monopoly power to hide iCloud-tier
frustration behind defaults.

The real risk is whether the Base model, version history, local/remote
reconciliation, File Provider integration, native shell, local services, auth,
and platform authority model compose into one trustworthy product path.

Selected transforms:

1. **Object transform**: the real object is not a wrapper; it is a reliable
   artifact/sync substrate exposed through native surfaces and a personal
   computer control pane.
2. **Prototype honesty**: the spike must preserve the hard topology instead of
   proving a decorative WebView.
3. **Single-writer / authority**: local desktop services, File Provider, cloud
   computer, and node admin must not create competing mutation authorities.
4. **Value of information**: the next probe should maximize learning about the
   riskiest joins: Base data model + deterministic reconciliation +
   File Provider materialization + Wails/native bridge + auth.
5. **Failure-mode / red-team**: admin UI plus local runtime can become a local
   privilege-escalation surface unless capability boundaries are designed from
   the first spike.

Route-changing insights:

- The first mission should not be "wrap Choir web in Wails." That is too easy
  and would overstate progress.
- The first mission should also not be "build a Dropbox clone." Choir Base is a
  source/artifact substrate for Choir computers, VText, publication, radio, and
  native file surfaces.
- The first mission should prove the Base sync model before broad UI work:
  immutable blobs, stable item IDs, versioned metadata, local/remote/synced
  state, cursor deltas, explicit conflicts, and reproducible fault tests.
- A Wails v3 app can still be part of the first mission, but as the native
  control surface for observing sync state and calling one typed Go service,
  not as the primary proof.
- Node Admin should be present early, even as a read-only/control-pane skeleton,
  because it reveals whether the desktop is merely a client or also a platform
  workstation controller.
- The local service boundary should be designed before local VMs, local LLMs,
  or candidate computers. Otherwise convenience APIs can become accidental
  root authority.
- The first artifact should produce upstream Wails repros when framework gaps
  appear. This is part of the plan, not a detour.

Changed plan:

- **Implementation**: start with a Base sync-model proof: server metadata
  journal, immutable blob refs, local/remote/synced trees, deterministic planner
  tests, cursor delta API, and a tiny Wails v3/node-admin observer if it helps
  inspect state.
- **Verifier/evidence**: require model/fault tests that create, edit, delete,
  move, rename, hydrate, dehydrate, go offline, reorder remote events, crash
  between writes, and produce explicit convergence or conflict evidence. Add
  Wails/File Provider proof only after the model can explain the behavior.
- **Scope**: do not start local VMs, local LLMs, broad desktop UI, or full File
  Provider sync until Base's model and failure ledger are real.
- **Stopping condition**: stop when Choir has a written Base data model plus
  executable proof that the reconciler never silently drops local or remote
  changes under the first fault matrix, and every non-mergeable case
  materializes an explicit conflict.

Next high-information action:

Define a MissionGradient for the Base sync-model proof, with Wails v3 as an
optional observer/control surface. The mission should make progress only when it
reduces the probability of iCloud-tier opaque sync failure.

## Sync Lore And Design Consequences

### Dropbox Lessons

Dropbox is the best public reference because it has repeatedly written about
why sync is hard, not just how storage scales.

Important lessons:

- split mutable file metadata from immutable content storage;
- use stable file/folder IDs, not paths, as the identity of filesystem objects;
- make moves atomic metadata updates, not recursive delete/add storms;
- keep an append-only server-side journal of file versions/changes;
- do not persist "work to do" as the core sync model; persist observations of
  consistent states and derive work from them;
- maintain at least three trees locally: latest remote state, last observed
  local disk state, and last fully synced state;
- treat the synced tree as a merge base, so the client can distinguish "local
  delete" from "remote add";
- design away invalid intermediate states, such as a child before its parent;
- make the core scheduler deterministic enough to reproduce failures from a
  seed;
- fuzz the sync engine with reordered network calls, filesystem failures,
  crashes, and random local/remote mutations.

Dropbox Nucleus is especially relevant to Choir. Sync Engine Classic stored
outstanding work per file, which allowed too many transient states. Nucleus
stores three individually consistent filesystem trees. The goal is convergence:
when local and remote match, sync is complete. This maps directly to Choir Base:

```text
Remote Tree: latest server metadata journal view.
Local Tree: last observed File Provider/local cache view.
Synced Tree: last known common base between local and remote.
Planner: derives upload/download/delete/conflict actions from tree diffs.
```

### Box Lessons

Box's public Box Drive writeup highlights the shift from old-style full sync to
a virtual filesystem/streaming model. The hard part is not showing files in
Finder. The hard part is reliable local event capture plus applying remote
changes locally without unwanted side effects.

For Choir, this means File Provider is not merely UI integration. It becomes a
fourth participant in the sync protocol:

```text
server journal <-> Base client DB <-> File Provider extension <-> Finder/apps
```

The Base client must track materialized items, pending local changes, hydration
state, and OS-triggered evictions. It must explain each stuck item.

### Google Drive Lessons

Google Drive for desktop exposes the core product tradeoff: streaming saves
disk, mirroring gives users simpler mental models. On macOS 12.1 and later,
streaming mode uses Apple's File Provider technology. Google's own advanced
guide also documents a real recovery path for unsynced local files lost by a
specific Drive for desktop issue in version 84.x.

For Choir, the consequence is blunt: even large teams ship sync bugs. The
product must have a recovery ledger from day one:

- local unsynced-change journal;
- per-item sync status and last error;
- repair/rescan action;
- conflict artifacts instead of silent overwrite;
- exportable diagnostics for a support or agent run.

### iCloud Drive Lessons

iCloud Drive's repeated user pain is not just that sync fails. It is that sync
fails opaquely: placeholder files, "waiting to upload", fileproviderd stalls,
storage confusion, slow uploads, and weak per-file explanation. Apple can get
away with more frustration because it owns the platform default. Choir cannot.

Choir must therefore make sync state legible:

- every Base item has local/remote/synced version identifiers;
- every pending operation has an owner, reason, timestamp, and retry state;
- placeholders must never masquerade as durable local bytes;
- a user or agent can ask "what is not synced and why?";
- there is always a conservative recovery path before destructive repair.

### Academic And Open-Source Lessons

The USENIX `*-Box` paper shows that loosely coupled cloud-sync clients can
silently propagate local corruption and inconsistent crash recovery. A sync
client must not blindly trust that every local filesystem observation is a
valid user intent.

The FAST 2020 lock-free collaboration study shows common service differences:
many services keep conflicting versions, iCloud has historically used
latest-version behavior in some cases, and Box supports manual locking. For
Choir, the default should be:

```text
Never silently choose a winner for opaque binary/file conflicts.
Materialize conflicts.
Keep version history.
Offer typed merge only for formats Choir understands.
```

Syncthing is useful lore because it exposes conflict files instead of pretending
multi-device writes are always mergeable. It also reinforces that database
files, constantly rewritten app state, and background mobile sync limitations
are dangerous workloads unless ignored, locked, or handled at the application
layer.

### Infrastructure Consequences For Node A / Node B

Node B's current shape, roughly 1 Gbps unmetered with RAID storage, is enough
for low-volume Base if Choir keeps the architecture simple and honest:

- server-authoritative metadata journal in Postgres or the current durable DB;
- immutable blob storage on local RAID-backed disk;
- content-addressed blob refs plus size/hash validation;
- periodic scrub of blob references against metadata;
- Node A as offsite/second-node backup and disaster-recovery rehearsal target;
- no claim that RAID is backup;
- clean storage abstraction so OVH bare metal can later expand into multiple
  boxes, clusters, or customer single-tenant deployments.

Forward compatibility with enterprise does not require full enterprise
hardening now. It does require the right nouns now:

- owner/org/tenant IDs in every Base object;
- auth subject and device IDs in every mutation;
- append-only audit/change log;
- capability/policy hooks around every admin or sync action;
- storage-provider abstraction;
- no direct DB/filesystem mutation path from node admin UI;
- passkeys now, SSO/WorkOS later without changing Base object identity.

### Base Sync Design Rule

Choir Base should be stricter than iCloud and smaller than Dropbox:

```text
Small scope.
Strong model.
Explicit conflicts.
Legible stuck states.
Recoverable local journal.
Deterministic fault tests before broad UI.
```

Sources:

- Dropbox Magic Pocket: https://dropbox.tech/tech/2016/05/inside-the-magic-pocket/
- Dropbox streaming file synchronization: https://dropbox.tech/tech/2014/07/streaming-file-synchronization
- Dropbox Nucleus rewrite: https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine
- Dropbox sync testing: https://dropbox.tech/infrastructure/-testing-our-new-sync-engine
- Box Drive architecture: https://blog.box.com/blog/box-drive-how-we-made-all-box-available-desktop
- Google Drive for desktop macOS: https://support.google.com/drive/answer/12178485
- Google Drive advanced guide: https://support.google.com/drive/answer/16631477
- Apple File Provider sync: https://developer.apple.com/documentation/FileProvider/synchronizing-the-file-provider-extension
- USENIX `*-Box`: https://www.usenix.org/conference/hotstorage13/workshop-program/presentation/zhang
- FAST 2020 lock-free collaboration proceedings: https://www.usenix.org/sites/default/files/fast20_full-proceedings-interior.pdf
- Syncthing sync model: https://docs.syncthing.net/users/syncing
- Ramsey/Csirmaz file sync formalism: https://www.cs.tufts.edu/~nr/pubs/sync-abstract.html
- Data Synchronization filesystem theory: https://arxiv.org/abs/2210.04565

## Broader Sync Perspectives And Working Hypotheses

This field has a public surface and a hidden craft layer. The vendors publish
the clean architecture diagrams; long-tail developers and frustrated users
expose the boundary failures: backup tools hydrating placeholders, file watchers
going quiet, conflict files multiplying, app-specific database files tearing,
OS upgrades changing storage semantics, and status UIs saying "up to date" when
the user's actual work is not safe.

### Perspective A: Dropbox / Nucleus Correctness

The strongest systems-engineering view is that smooth sync is a consequence of
a small number of hard invariants:

- stable item IDs rather than path identity;
- immutable content blocks separate from mutable metadata;
- an append-only metadata journal;
- a synced-base tree so local and remote changes can be interpreted relative to
  a known common ancestor;
- a planner that derives work from observations, not from loosely persisted
  "things to do";
- deterministic tests that can reorder events, inject crashes, and reproduce
  the same failure seed.

Hypothesis:

```text
Fast, stable cloud filesystems are mostly a data-model and testability problem.
Performance work compounds only after invalid states are designed away.
```

Choir consequence:

Build the Base sync planner as a testable core library before tying correctness
to File Provider callbacks, Wails windows, or staging storage. The planner
should accept remote/local/synced trees and emit explicit actions/conflicts.

### Perspective B: Local-First / CRDT Thought Leaders

Ink & Switch, Martin Kleppmann, Automerge, and adjacent local-first work argue
that cloud sync should preserve user ownership, offline work, multi-device
collaboration, and long-term data access. They also expose a crucial split:
generic file sync can preserve opaque binary versions, but it cannot understand
user intent inside application data unless the app exposes semantic operations.

Kleppmann's work on replicated tree move operations is especially relevant
because filesystems are trees, and concurrent moves can create invalid states
such as cycles or lost subtrees if treated casually. This is not an edge case
for a sync product; it is the core shape of shared folders.

Hypothesis:

```text
The best user experience comes from semantic sync where possible and
conservative file sync where not possible.
```

Choir consequence:

Use CRDTs or semantic merge for Choir-native artifacts such as VText structure,
radio state, annotations, manifests, and agent-authored drafts. Use conservative
versioned file sync for opaque files: preserve both versions, materialize
conflicts, and never pretend a binary merge happened.

### Perspective C: OS Placeholder / File Provider Reality

Dropbox, Google Drive, Box, OneDrive, and iCloud have all been pushed toward OS
cloud-file APIs: Apple's File Provider on macOS/iOS and Cloud Files on Windows.
These APIs improve native integration, but they introduce a new category of
truth ambiguity:

- file names may be present without bytes;
- backup tools may either hydrate the world or skip cloud folders;
- OS indexing/search may enumerate materialized working sets in the background;
- "online only" flags can disagree with user expectations;
- the sync root location and extension domain become OS-controlled;
- a general POSIX-looking path no longer behaves like a simple local file.

Hypothesis:

```text
Native cloud-file integration is a UX win only if placeholder state is explicit
and every third-party/local access path can tell whether bytes are really local.
```

Choir consequence:

Base must expose durable states for `dataless`, `materialized`, `dirty-local`,
`dirty-remote`, `hydrating`, `evictable`, `pinned-offline`, `conflicted`,
`blocked`, and `unknown-needs-rescan`. The UI should never collapse these into a
single spinner.

### Perspective D: User Mental Models And Trust

Microsoft Research's "That syncing feeling" and long-running Dropbox/iCloud
complaints show that users need a working model of what the cloud is doing:
personal repository, shared repository, replicated local store, shared
replicated store, and synchronization mechanism are different concepts. Products
fail when they slide between these models without saying so.

Andy Polaine's Dropbox critique makes the same point from a product-design
angle: users often think a shared sync folder is a server share, but it is
actually a replicated folder with delayed reconciliation. That mismatch creates
privacy mistakes, stale edits, and collaboration confusion.

Hypothesis:

```text
Trust depends on process transparency as much as raw convergence.
A user must know which replica is authoritative for the action they are taking.
```

Choir consequence:

Base needs an owner-readable sync inspector:

- "what is local-only?";
- "what is remote-only?";
- "what has not uploaded?";
- "what has not downloaded?";
- "what changed on another device?";
- "what is waiting on OS/File Provider/app lock/network/auth?";
- "what would be lost if I disconnect this device?";
- "make this folder truly local and backup-safe."

### Perspective E: Self-Hosted / Open-Source Pragmatism

Syncthing, Nextcloud, Seafile, and Unison show the trade space outside big
vendor sync:

- Syncthing is honest about conflicts by creating conflict files and exposing
  block/index state, but peer-to-peer always-on sync can create repeated
  conflicts with background-updated app files.
- Nextcloud exposes the operational fragility of self-hosted sync: local sync
  databases, file watchers, locks, unknown errors, large-file-count scans, and
  VFS modes can become the actual product.
- Seafile's Git-like/block-level orientation suggests that content-addressed
  chunks and libraries scale better than naive full-file copying.
- Unison's long-lived formalism is valuable because it separates update
  detection from reconciliation and treats conflict semantics as a specification
  problem, not just an implementation accident.

Hypothesis:

```text
Small teams succeed by shrinking sync scope, making conflicts explicit, and
building repair tooling; they fail when they copy enterprise surface area before
they have enterprise operational machinery.
```

Choir consequence:

Do not aim for "sync any arbitrary home directory" in the early product. Start
with owner-scoped Choir Base libraries and app-created/source artifacts. Treat
database files, package manager directories, Git repos, build outputs, and
rapidly-mutating app state as ignored/unsupported until Choir has a declared
policy for them.

### Perspective F: Backup Is Not Sync

Backblaze/Dropbox/OneDrive placeholder disputes are a crucial warning. Users
often believe a local cloud folder is backed up because it appears in Finder or
Explorer. In reality it may contain placeholders, not bytes; backup software may
skip it, hydrate it, or silently change policy. Sync also propagates deletes and
corruption, so it is not a backup.

Hypothesis:

```text
A cloud filesystem earns trust only when it distinguishes sync, backup,
archive, cache, and publication as separate product states.
```

Choir consequence:

Base should have separate labels and mechanics for:

- sync replica;
- local cache;
- pinned local copy;
- backed-up copy;
- archived immutable version;
- published/exported artifact;
- source manifest.

RAID on Node B improves availability after disk failure; it is not backup. Node
A should become an explicit off-node backup/restore rehearsal target before Base
claims durability beyond low-volume alpha use.

### Perspective G: Security / Privacy / Enterprise Forward Compatibility

Dropbox's historical security controversies, cloud provider deduplication
debates, and privacy-first alternatives expose a tradeoff: global dedup and
server-side indexing improve cost and features, while end-to-end encryption and
tenant isolation reduce operator access and cross-user leakage. Enterprise SSO
does not fix storage trust; it only improves identity governance.

Hypothesis:

```text
Security posture is set by object identity, key boundaries, auditability, and
operator access paths long before enterprise SSO arrives.
```

Choir consequence:

For the early product:

- content-address within an owner/tenant boundary, not globally across tenants;
- include owner/org/tenant/device/auth-subject on every mutation;
- keep audit/change logs append-only;
- do not let node-admin mutate blobs or metadata outside the product journal;
- plan key hierarchy now even if full E2EE comes later;
- make storage-provider boundaries explicit for OVH bare metal and
  customer-cloud single-tenant deployments.

### Synthesis: Choir's Best Theory For Base

The strongest combined hypothesis is:

```text
Choir Base should be a narrow, model-first, owner-scoped sync substrate:
Dropbox-like in invariant discipline,
local-first where Choir owns the data type,
File-Provider-native but placeholder-honest,
self-hostable like the pragmatic open-source tools,
and legible enough that users and agents can repair stuck states.
```

Early low-resource design:

1. Server-authoritative metadata journal.
2. Immutable blob store on Node B RAID-backed disk.
3. Node A offsite backup/restore rehearsal target.
4. Owner-scoped stable item IDs.
5. Remote/local/synced tree planner.
6. Conservative conflict materialization for opaque files.
7. Semantic merge only for Choir-native artifacts.
8. Per-item sync inspector and repair handles.
9. Deterministic fault test harness.
10. Wails v3 desktop as observer/control pane after the model exists.

Forward-compatible design:

1. Tenant/org fields now; WorkOS/SSO later.
2. Storage-provider abstraction now; clusters/customer-cloud later.
3. Audit journal now; enterprise compliance export later.
4. Key hierarchy now; stronger E2EE/customer-managed keys later.
5. File Provider on Mac first; Cloud Files/SAF parity later.
6. Run acceptance and verifier evidence now; formal operational SLOs later.

Additional sources:

- Local-first paper: https://martin.kleppmann.com/2019/10/23/local-first-at-onward.html
- Ink & Switch local-first essay: https://www.inkandswitch.com/essay/local-first/
- Replicated tree move operation: https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html
- Tonsky local-first filesync reflection: https://tonsky.me/blog/crdt-filesync/
- Microsoft Research "That syncing feeling": https://www.microsoft.com/en-us/research/publication/that-syncing-feeling-early-user-experiences-with-the-cloud/
- Andy Polaine on Dropbox mental models: https://www.polaine.com/2012/10/dropboxs-mental-models-are-broken/
- objc.io data synchronization: https://www.objc.io/issues/10-syncing-data/data-synchronization/
- Nextcloud desktop architecture overview: https://deepwiki.com/nextcloud/desktop/2-architecture
- Nextcloud sync algorithm note: https://git.chylex.com/chylex/Nextcloud-Desktop/src/tag/v3.1.0-rc2/doc/dev/sync-algorithm.md
- Nextcloud slow sync issue: https://github.com/nextcloud/desktop/issues/691
- Seafile overview: https://en.wikipedia.org/wiki/Seafile
- Unison specification: https://repository.upenn.edu/entities/publication/df9bb5bd-2f08-4333-89ee-198156a9cf1f
- Backblaze placeholder/cloud-folder controversy: https://www.tomshardware.com/software/cloud-storage/backblaze-redefines-unlimited-while-users-discover-its-not-backing-up-dropbox-and-onedrive-service-changes-could-signal-shift-away-from-home-backups
- Backblaze/Dropbox Smart Sync interaction: https://community.dropbox.com/discussions/101001014/backblaze-is-backing-up-smart-sync%E2%80%99d-files/272188
- Dropbox File Provider feedback thread: https://community.dropbox.com/en/discussion/845182/file-provider-on-dropbox-questions-and-feedback
- PanicVault conflict summary for password DBs: https://www.panicvault.org/cloud-sync/sync-conflicts/
- `*-Box` cloud sync reliability paper: https://www.usenix.org/conference/hotstorage13/workshop-program/presentation/zhang

## Hacker News And Chinese Cloud Drive Lore

### Hacker News Practitioner Signal

HN discussions are valuable here because former sync engineers, local-first
builders, open-source users, and frustrated power users argue from operational
scar tissue instead of vendor positioning.

Recurring signals:

- **The long tail is the product.** In a Dropbox/Syncthing thread, a Dropbox
  sync engineer noted that Dropbox's advantage came partly from engineering
  resources, telemetry, and exposure to hundreds of millions of users running
  every application imaginable. This matters because the hard cases are not the
  happy path; they are weird apps, locks, permissions, filesystems, and platform
  behaviors.
- **File watchers are not enough.** Former/current Dropbox discussion around
  Nucleus mentions startup scans, Windows `ReadDirectoryChangesW`, and possible
  future use of the NTFS USN journal. The lesson is that event streams are
  lossy or incomplete under load; a sync engine needs reconciliation scans and
  platform-specific journals where available.
- **"Keeping server and client both complete" is the dragon.** Local-first HN
  discussion compresses the core problem well: having state on both server and
  client sounds easy until one asks which replica wins and how conflicts are
  represented.
- **CRDTs help where the data type is semantic, but can make server-side
  inspection/query harder.** HN local-first discussion praises Yjs/Ditto while
  also warning that CRDT documents can be awkward to inspect or query without
  loading application-specific state.
- **Sync should be quiet, but not opaque.** An older "Why Sync Is So Difficult"
  thread includes the useful tension: over-reporting minor conflicts can make a
  sync product unusable, but under-reporting serious stuck state destroys
  trust. Choir needs progressive disclosure, not maximal warning banners.
- **LAN/local peer sync is a performance feature, not an authority model.**
  Dropbox LAN sync discussion distinguishes discovering bytes locally from
  accepting unsanctioned metadata authority. For Choir, local peer transfer can
  come later, but server journal authority should remain clear.
- **App-specific real-time sync beats pile-of-files sync for some domains.**
  Recent Obsidian sync discussions favor CRDT keystroke/operation sync for text
  and attachments separately, with stable IDs for rename structure.

Hypothesis from HN:

```text
The public architecture matters less than how the engine handles unreliable
event sources, weird application behavior, status transparency, and the
semantic/opaque-file boundary.
```

Choir consequences:

- Design the Base planner to tolerate missed local events and remote
  notification gaps.
- Add periodic reconciliation and platform journals later; do not depend only
  on File Provider callbacks.
- Use semantic sync for Choir-native text/manifest/radio state; opaque conflict
  preservation for arbitrary files.
- Build a status model with levels: quiet success, visible pending, actionable
  stuck, explicit conflict, and dangerous/destructive repair.

### Chinese Cloud Drive Signal

Chinese cloud drives expose a different set of product optimizations from US
enterprise sync products. Baidu Netdisk, Alibaba Cloud Drive, Quark, 115,
Xunlei, Tencent Weiyun, and similar products are often optimized around mobile
backup, large media libraries, resource sharing, instant transfer, and consumer
download/upload economics. Nutstore/Jianguoyun is the closer Chinese analogue
to Dropbox-style file sync.

Recurring signals:

- **"秒传" instant upload is culturally and technically central.** Many Chinese
  discussions and docs describe instant upload via client-side hash/fingerprint
  checks, deduplication, and sometimes pre-hash shortcuts for large files. This
  can make uploads feel magical for common media/resources while saving
  bandwidth.
- **Dedup has side effects.** Alibaba PDS documentation explicitly notes that
  instant upload requires caller-side scenario judgment because computing MD5,
  SHA-1, or pre-hash values has cost and side effects. Cross-account/global
  dedup also raises privacy, abuse, copyright, and content-moderation issues.
- **Nutstore emphasizes incremental sync and conservative conflicts.** Public
  Nutstore docs describe intelligent incremental sync: detect the changed part
  of a file and upload only the differences. Its conflict docs say conflicts
  are preserved as conflict files to avoid data loss, with user resolution.
- **Database/app-state files remain dangerous.** Chinese Obsidian, Xiaoshujiang,
  Siyuan, and note-app sync discussions repeat the same lesson as Western
  sources: app databases, locked files, and background-mutating state often
  produce conflicts or corruption unless the application owns the sync semantics.
- **Consumer netdisks are not necessarily better shared filesystems.** Baidu,
  Aliyunpan, Quark, and 115 may be excellent at bulk storage, media transfer,
  instant save/transfer, and mobile backup, but that does not imply stronger
  multi-device filesystem reconciliation. Some user reports still show duplicate
  files, sync failures, version-history surprises, and manual-log debugging.
- **Moderation and ecosystem control are first-class architecture forces.**
  Chinese cloud drives often integrate content review, sharing controls, speed
  tiers, unofficial-client blocking, and hash-based transfer controls. These
  forces shape the storage design as much as sync correctness does.

Hypothesis from Chinese cloud drives:

```text
Chinese consumer clouds may be ahead on perceived speed for common large files
because they optimize dedup, instant transfer, mobile backup, and media flows.
They are not automatically ahead on trustworthy shared filesystem semantics.
```

Choir consequences:

- Use owner-scoped content-addressed blobs and chunking early, but avoid global
  cross-tenant dedup until privacy/security policy is explicit.
- Add "instant import" only within safe scopes: same owner, same tenant, or
  explicitly shared library lineage.
- Consider pre-hash/partial-hash probing for large files later, but never trust
  weak hashes alone for integrity.
- Separate "fast save/import" from "sync complete." Instant metadata attachment
  does not mean the user has a local durable copy.
- Learn from Nutstore's incremental sync and conflict-file conservatism more
  than from media-netdisk sharing economics.
- Build for consumer delight on large media/source imports, but keep Base's
  authority model closer to Dropbox/Nucleus than to global netdisk dedup.

### Updated Synthesis After HN And Chinese Sources

The best version of Choir Base should combine:

```text
Dropbox/Nucleus correctness discipline.
Nutstore-style incremental/conservative file conflict handling.
Chinese netdisk speed tricks only inside privacy-safe ownership boundaries.
Local-first semantic sync for Choir-native data.
HN-style skepticism about file watchers, CRDT tradeoffs, and opaque status.
```

This changes the early implementation priorities slightly:

1. Stable item IDs and remote/local/synced planner remain first.
2. Blob hashes should be designed now for future chunking and owner-scoped
   dedup.
3. Conflict files/statuses should be first-class Base objects, not ad hoc
   filename suffixes only.
4. Per-item diagnostics should include the event source: API, File Provider,
   filesystem scan, platform journal, remote journal, or repair action.
5. The first speed feature should be "no duplicate upload inside the same
   owner/tenant," not cross-user global dedup.
6. The first UX feature should be "what is not safe yet?" not a pretty spinner.

Additional HN sources:

- Nucleus HN thread: https://news.ycombinator.com/item?id=22595782
- Testing sync at Dropbox HN thread: https://news.ycombinator.com/item?id=22928726
- Syncthing/Dropbox HN thread: https://news.ycombinator.com/item?id=13857009
- Syncthing replacement HN thread: https://news.ycombinator.com/item?id=7734114
- Local-first HN thread: https://news.ycombinator.com/item?id=44473135
- Resilient sync local-first HN thread: https://news.ycombinator.com/item?id=40772955
- YAOS Obsidian sync HN thread: https://news.ycombinator.com/item?id=47361635
- Why Sync Is So Difficult HN thread: https://news.ycombinator.com/item?id=602275

Additional Chinese-language sources:

- Nutstore intelligent incremental sync: https://help.jianguoyun.com/?p=775
- Nutstore conflict handling: https://blog.jianguoyun.com/?p=2699
- Nutstore conflict discussion mirror: https://cloud.tencent.com/developer/news/267177
- Xiaoshujiang/Nutstore conflict case: https://www.cnblogs.com/Howfars/p/13637246.html
- Nutstore Sync for Obsidian practice: https://www.cnblogs.com/nut-king/p/20001806
- Alibaba PDS technical whitepaper: https://apsara-doc.oss-cn-hangzhou.aliyuncs.com/apsara-pdf/enterprise/v_3_16_0_20220117/pds/zh/technical-whitepaper.pdf
- Baidu enterprise netdisk instant upload: https://eyun.baidu.com/content/856610/
- Baidu instant upload discussion: https://jingyan.baidu.com/article/4b52d7029c9fb9fc5c774b85.html
- Aliyunpan instant upload discussion: https://www.zhihu.com/tardis/bd/art/319962538
- Aliyunpan sync conflict article: https://www.php.cn/faq/1724747.html
- Baidu sync-failure complaint: https://tousu.sina.com.cn/complaint/view/17371454197
- Baidu duplicate-sync report: https://club.fnnas.com/forum.php?mod=viewthread&tid=29887
- Quark sync/backup practice: https://www-kuakewangpan.cn/blogs/1558406704
- Siyuan third-party sync warning: https://b3logfile.com/pdf/article/1626537583158.pdf

## Second Chinese And Open-Source Implementation Pass

### Chinese Practitioner Additions

The second Chinese-language pass added more note-app and WebDAV evidence. The
most useful pattern is that many Chinese Obsidian users converge toward a
three-way distinction:

- plain cloud folder sync is convenient but brittle for fast-changing vaults;
- WebDAV plugins are flexible but easy to overload or misconfigure;
- app-specific sync plugins can beat generic file sync because they know the
  shape of the data, can track records, and can expose meaningful logs.

Useful examples:

- A lightweight WebDAV Obsidian plugin advertises `ETag + SHA-256` three-way
  comparison specifically to avoid timestamp-based mis-sync when servers rewrite
  mtimes.
- Nutstore Sync for Obsidian claims smarter conflict handling and warns users
  to back up before sync. This is still a signal: even a purpose-built plugin
  treats backup as part of the sync ritual.
- Chinese Obsidian posts repeatedly warn not to mix Git, WebDAV, cloud folder
  sync, and plugins over the same vault unless the user understands the conflict
  surface.
- Nutstore-related posts mention request-rate and concurrency problems with
  generic WebDAV sync plugins. This maps to Choir as backpressure: a sync system
  must shape client concurrency instead of letting many devices stampede the
  server.
- Alibaba PDS instant-upload documentation gives an explicit engineering
  warning: pre-hash/MD5/SHA-1 shortcuts reduce upload cost but have scenario
  costs, especially for many non-duplicate files.

Additional Chinese hypothesis:

```text
Generic sync gets worse when several sync systems observe the same directory.
The winning product should own one synchronization authority per subtree and
make all other paths read-only, ignored, or explicitly imported/exported.
```

Choir consequences:

- Base should prevent "double sync" by policy: do not let a Base folder be
  simultaneously managed by Git auto-sync, File Provider sync, WebDAV sync, and
  app-specific sync unless one is declared authoritative.
- Use ETag/content-hash/version IDs over mtimes wherever possible.
- Apply client backpressure and rate shaping in the Base protocol.
- In the UI, distinguish "backup before first import", "sync steady state", and
  "repair/resync" as separate workflows.

### GitHub / Hobbyist / Open-Source Patterns

Current GitHub activity checked on 2026-06-06:

```text
syncthing/syncthing      Go          ~85.1k stars   pushed 2026-06-05
rclone/rclone           Go          ~57.8k stars   pushed 2026-06-06
nextcloud/desktop       C++         ~3.7k stars    pushed 2026-06-06
owncloud/client         C++         ~1.5k stars    pushed 2026-06-05
bcpierce00/unison       OCaml       ~5.3k stars    pushed 2026-05-07
electric-sql/electric   TypeScript  ~10.2k stars   pushed 2026-06-05
rocicorp/mono / Zero    TypeScript  ~3.2k stars    pushed 2026-06-06
pubkey/rxdb             TypeScript  ~23.2k stars   pushed 2026-05-30
evoluhq/evolu           TypeScript  ~1.8k stars    pushed 2026-05-07
sqliteai/sqlite-sync    C           ~472 stars     pushed 2026-06-04
```

#### Syncthing

Syncthing is the open-source reference for continuous peer-to-peer file sync.
Patterns worth borrowing:

- block-level file descriptions;
- per-device identity and TLS;
- local index database;
- explicit conflict files rather than silent winners;
- ignore patterns;
- detailed web UI/API for diagnostics;
- no central server authority by default.

What not to copy directly:

- peer-to-peer authority for Choir's canonical Base state;
- assuming always-on devices;
- making the user understand device/folder graph topology before the product is
  usable.

Choir should borrow Syncthing's transparency, block/index vocabulary, and
conflict conservatism, but keep the server metadata journal authoritative for
Base.

#### rclone bisync

rclone bisync is valuable because it is explicit about prior-run state and
conflict policy. A conflict is a file changed on both sides relative to the last
run and not currently identical. Default behavior is conservative: keep renamed
copies rather than choose a winner. It also documents special cases where hashes
are not stable, such as generated Google Docs exports.

Choir should borrow:

- "prior run" / synced-base semantics;
- default conflict preservation;
- conflict policy flags as product concepts;
- hash instability awareness for generated/projection files.

Choir should not borrow rclone's batch/manual nature for the main app, but it is
an excellent model for repair/resync commands.

#### Nextcloud / ownCloud

Nextcloud and ownCloud show how sync journals, ETags, checksums, file IDs, and
local discovery interact. They also show failure modes common to self-hosted
systems:

- "unknown error" states;
- slow scans with many small files;
- local sync database drift;
- virtual file interactions;
- false conflicts from time deviation or server/client metadata mismatch;
- locked files causing repeated traffic and user uncertainty.

The ownCloud manual's historical notes are especially relevant: sync without a
special server component has limits; file ID support, sync journal correctness,
checksum negotiation, and time skew all matter.

Choir should borrow:

- sync journal tables with ETag/checksum/file-id fields;
- debug archives/exportable logs;
- explicit unsupported-version warnings;
- server capability negotiation.

Choir should avoid:

- opaque "unknown error";
- scanning loops that do not name the item and blocker;
- virtual-file mode without a per-item status model.

#### Seafile

Seafile is interesting because it uses libraries, block-level dedup/sync, and
conflict files. Its user docs describe keeping the first cloud-synced version
unchanged and renaming the later conflicting version with user/time metadata.
It also offers read-only sync and ignore rules.

Choir should borrow:

- library/subtree scoping;
- block-level dedup and incremental transfer;
- conflict filename metadata as a human-readable projection of a real conflict
  object;
- read-only and one-way sync modes.

#### Pydio Cells Sync

Pydio Cells Sync is a Go desktop sync client with multiple endpoint types:
Cells server, local folder, local Cells server, S3-compatible storage, BoltDB
tree snapshots, and gRPC indexation services. The important lesson is not its
popularity; it is its endpoint abstraction and snapshot capture command.

Choir should borrow:

- endpoint abstraction from day one;
- local tree snapshots as first-class debug artifacts;
- CLI verbs for capture/start/service/systray/status;
- server-side sync configuration where policy matters.

#### Unison

Unison is the formalist's reference. It is user-triggered, monitored
bidirectional sync rather than always-on cloud magic. Its contribution is
specification discipline: update detection and reconciliation are distinct, and
conflicts are something the user can inspect and decide.

Choir should borrow:

- a precise reconciliation spec;
- a manual "dry run / preview / apply" repair mode;
- user-readable conflict alternatives;
- avoiding timestamp dependence where content IDs are available.

#### git-annex

git-annex separates file identity/metadata from large content by storing
content-addressed objects and committing pointers/availability metadata through
Git. It is powerful for archiving and data management, but too conceptually
heavy for default consumer sync.

Choir should borrow:

- content-addressable large blob storage;
- "where is this content available?" metadata;
- minimum-copy / backup policy checks;
- manual expert repair tooling.

Choir should avoid:

- exposing pointer-file mechanics as the main UX;
- making Git the primary Base substrate for arbitrary user files.

#### Local-First Database Engines

Electric, Zero, RxDB, Evolu, and SQLite Sync show a different pattern: sync
small structured application state into a local database for instant UI. This is
excellent for Choir-native artifacts but does not replace filesystem sync.

Useful patterns:

- shape/subset sync rather than "download the universe";
- local database reads in the next frame;
- persisted stream offsets;
- transactional consistency across multiple tables;
- explicit unsupported cases for local writes/conflicts in early alpha engines.

Choir should apply these to:

- VText structure;
- source manifests;
- radio queues and listening state;
- desktop/node-admin status;
- model policy and device state.

Choir should not try to represent every arbitrary binary file as a CRDT or
local-first DB row. Files remain blobs plus metadata/version/conflict objects.

### Open-Source Design Rules For Choir

```text
One authoritative sync engine per subtree.
Content hashes and ETags over mtimes.
Synced-base/prior-run state is mandatory.
Default conflicts preserve both sides.
Repair workflows need dry-run previews.
Diagnostics are product surface, not support afterthought.
Endpoint abstraction now prevents bare-metal lock-in later.
Semantic local-first DB sync is for Choir-native objects, not opaque files.
```

Additional open-source sources:

- Syncthing GitHub: https://github.com/syncthing/syncthing
- Syncthing docs: https://docs.syncthing.net/users/syncing
- rclone bisync: https://rclone.org/bisync/
- Nextcloud desktop GitHub: https://github.com/nextcloud/desktop
- Nextcloud architecture overview: https://deepwiki.com/nextcloud/desktop/2-architecture
- ownCloud client GitHub: https://github.com/owncloud/client
- ownCloud desktop manual: https://doc.owncloud.com/pdf/desktop/2.8_ownCloud_Desktop_Client_Manual.pdf
- Seafile conflicts: https://help.seafile.com/syncing_client/file_conflicts/
- Seafile ignore rules: https://help.seafile.com/syncing_client/excluding_files/
- Pydio cells-sync GitHub: https://github.com/pydio/cells-sync
- Pydio Cells Sync docs: https://pydio.com/en/docs/developer-guide/cells-sync
- Unison specification: https://repository.upenn.edu/entities/publication/df9bb5bd-2f08-4333-89ee-198156a9cf1f
- git-annex design overview: https://en.wikipedia.org/wiki/Git-annex
- Electric/PGlite sync: https://pglite.dev/docs/sync
- Zero sync product: https://zero.rocicorp.dev/
- RxDB sync engine: https://rxdb.info/
- Evolu: https://www.evolu.dev/
- SQLite Sync: https://github.com/sqliteai/sqlite-sync

## MissionGradient For Execution

## Engineering Shape: What The Code Wants To Become

The elegant solution is not "a sync app." It is a small circuit of pure
primitives with clear current flow:

```text
Content identity -> immutable bytes -> append-only metadata events
-> derived trees -> planner -> operations -> status/evidence
```

Every part should have one job. The system should spend energy only when a
boundary changes: bytes arrive, metadata changes, a local observation changes,
or a client asks for a projection. This is the circuit-design view: keep the hot
path short, keep mutation single-writer, make derived state cheap to recompute,
and put isolation at the boundaries rather than everywhere.

### Existing Seed In Choir

Choir already has the beginning of Base:

- `content_items` are owner-scoped source artifacts.
- VText import/export and source workflows already resolve by durable
  `content_id`.
- Content records already carry metadata, provenance, media type, app hint,
  source URL, file path, text content, and content hash.

The code does not want a parallel Dropbox clone. It wants `content_items` to
become the visible tip of a stronger Base substrate.

### Proposed Core Packages

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

Responsibilities:

- `model`: stable IDs and value types only. `ItemID`, `BlobID`, `VersionID`,
  `EventID`, `DeviceID`, `TenantID`, `TreeNode`, `Conflict`, `SyncStatus`.
- `blob`: immutable byte storage and hash verification. Local filesystem first;
  storage-provider interface second.
- `journal`: append-only metadata events. Create, update, move, delete,
  tombstone, restore, attach blob, mark projection, materialize conflict.
- `tree`: derive a consistent owner/item tree from journal events.
- `planner`: pure reconciliation. Input is `remote`, `local`, and `synced`
  trees; output is actions and conflicts. No network, database, filesystem, or
  clock.
- `status`: per-item visible state and repair handles.
- `api`: HTTP handlers over journal/blob/delta/status.
- `local`: client-side observation adapters: File Provider later, filesystem
  scan, Wails bridge, local cache DB.
- `testkit`: deterministic fault runner, in-memory blob store, in-memory
  journal, random tree generator, crash/reorder/drop/lock simulations.

The planner is the center of the board. Everything else plugs into it.

### Data Model Layout

Keep the current `content_items` table as compatibility/read model, then add
Base tables beside it:

```text
base_blobs
  blob_id            sha256:<hex>
  owner_id
  tenant_id
  size_bytes
  sha256
  storage_provider  local-fs | s3 | ...
  storage_key
  created_at
  verified_at

base_items
  item_id
  owner_id
  tenant_id
  parent_item_id
  name
  kind               file | folder | package | projection | conflict
  current_version_id
  deleted_at
  created_at
  updated_at

base_versions
  version_id
  item_id
  owner_id
  tenant_id
  blob_id
  media_type
  content_hash
  manifest_json
  provenance_json
  created_by_device_id
  created_by_subject
  created_at

base_events
  event_id
  owner_id
  tenant_id
  item_id
  device_id
  subject_id
  event_type
  parent_event_id
  cursor_seq
  payload_json
  created_at

base_device_cursors
  owner_id
  device_id
  last_seen_cursor_seq
  last_ack_cursor_seq
  updated_at

base_sync_status
  owner_id
  device_id
  item_id
  local_version_id
  remote_version_id
  synced_version_id
  state
  last_error
  repair_handle
  updated_at
```

This is deliberately boring. The novelty belongs in the invariants and tests,
not in clever schema.

### Circuit Design

The lowest-energy layout:

```text
          immutable bytes
              |
              v
        blob CAS store
              |
              v
user/app -> append-only event journal -> derived remote tree
              ^                         |
              |                         v
        local observations -> local tree + synced tree
                                      |
                                      v
                                  planner
                                      |
                                      v
                           actions / conflicts / status
```

Energy-efficient choices:

- **Single writer for metadata**: server journal owns canonical Base mutation.
- **Immutable blobs**: no in-place mutation, no overwrite ambiguity.
- **Derived trees**: rebuildable from journal and snapshots.
- **Pure planner**: cheap tests, deterministic failures, no side effects.
- **Owner-scoped CAS**: dedup within owner/tenant first; no global privacy trap.
- **Snapshots as cache**: tree snapshots speed reads but are disposable.
- **Status as product state**: stuck states are first-class, not log spelunking.

### Resource Strategy For Node A / Node B

Early low-resource deployment:

```text
Node B:
  canonical app/API
  Base metadata DB
  local RAID-backed blob store
  journal snapshots
  scrub job

Node A:
  off-node blob replica
  metadata/journal backup
  restore rehearsal target
  experimental/candidate node
```

Use the 1 Gbps unmetered bandwidth for:

- streaming uploads/downloads directly to Node B;
- background Node A replication;
- periodic blob scrub/verify;
- export/import package movement.

Do not spend early complexity on:

- multi-master cluster metadata;
- cross-tenant dedup;
- global content search over all bytes;
- local P2P LAN sync;
- arbitrary home-directory sync;
- enterprise E2EE before object/key boundaries are designed.

The forward-compatible path is a storage-provider interface plus event journal
replication, not a distributed filesystem.

### Dependencies And Vendor Policy

Default: **use almost no new deps for Base v0**.

Use now:

- Go standard library for `crypto/sha256`, `io`, `fs`, `filepath`, `net/http`,
  `encoding/json`, `database/sql`, and deterministic tests.
- Existing `modernc.org/sqlite` for local/client test stores where needed.
- Existing Dolt/SQL store path for canonical metadata if that remains the
  platform direction.
- Existing `github.com/google/uuid` only where random IDs are already the local
  pattern; prefer deterministic/content IDs where possible.

Add later only if the first model proves need:

- `github.com/wailsapp/wails/v3` in the desktop app module, not in Base core.
- A small content-defined chunking implementation after whole-file/fixed-chunk
  proof. Prefer a tiny internal FastCDC/Buzhash-style implementation or one
  well-audited direct dependency; do not import Syncthing just for chunking.
- Platform-native File Provider/Cloud Files bindings in native app packages,
  isolated from the core planner.
- OpenTelemetry only around API/service boundaries if existing platform
  telemetry is already active; not inside the planner.

Do not vendor:

- Syncthing as the sync engine. Borrow patterns, not authority.
- rclone as the core. Use it as inspiration for repair/resync semantics.
- Nextcloud/ownCloud clients. Too much surface area and old assumptions.
- CRDT frameworks for opaque files. Use semantic CRDTs only for Choir-native
  artifacts if and when VText/radio need them.
- S3/cloud SDKs in the core package. Keep them behind storage-provider adapters.

Vendoring rule:

```text
If a dependency affects correctness of conflict resolution, either keep it out
of the core or make its behavior small enough to model in deterministic tests.
```

### What To Build First In Code

The first implementation should be a small executable proof, not a platform
rewrite:

1. `internal/base/model`: item/version/blob/event/status types.
2. `internal/base/planner`: pure three-tree reconciliation.
3. `internal/base/testkit`: deterministic scenario runner.
4. Table DDL and store methods for blobs/items/versions/events.
5. Compatibility adapter that projects Base item heads into existing
   `content_items`.
6. Minimal HTTP:
   - `POST /api/base/blobs`
   - `POST /api/base/items`
   - `GET /api/base/delta?cursor=...`
   - `GET /api/base/items/{id}/status`
   - `POST /api/base/repair/preview`
7. Only then: Wails v3 observer and File Provider feasibility.

### Non-Negotiable Tests

Before UI breadth:

- local add vs remote add same path;
- local edit vs remote edit same file;
- local delete vs remote edit;
- local move vs remote edit;
- concurrent folder moves;
- rename case-only path on case-insensitive FS;
- missed local event recovered by scan;
- duplicate remote event idempotent;
- crash after blob write before event append;
- crash after event append before status update;
- corrupt local blob detected by hash;
- locked file becomes actionable stuck status;
- projection export does not overwrite original.

This is where correctness should live. If these tests are easy to write, the
architecture is right. If they are hard, the code is lying about its shape.

### Real Artifact

Choir Base: the durable owner-scoped content/source/file substrate plus native
client contracts that expose it through OS file surfaces and Choir apps.

### Invariants

- `ContentItem` remains the owner-scoped source artifact substrate.
- Originals are preserved; projections are marked as projections.
- Sync is versioned, cursor-based, conflict-aware, and replayable.
- Native clients do not bypass product auth, provenance, or policy.
- VText, publication, source, and radio resolve artifacts through durable IDs.
- Local clients can cache or materialize bytes but do not become the source of
  truth without version promotion.
- Platform behavior changes require staging proof per repo contract.

### Value Criterion

Move uphill when more user artifacts become durable, addressable, source-aware,
syncable, searchable, and available to VText/radio/native apps without losing
provenance or creating parallel storage paths.

### Homotopy Parameters

- One owner -> many owners/orgs.
- Cloud-only blobs -> cloud plus local cache -> optional local-first mode.
- Manual upload/import -> Finder File Provider sync -> Files/Explorer/SAF.
- Text artifacts -> binary documents/media -> extracted selectors and indexes.
- Cloud computers only -> Mac app shell -> local candidate computers.
- Visual VText reading -> audio/radio traversal -> watch voice control.

### Forbidden Shortcuts

- Claiming Finder/Files support from browser upload/download.
- Treating imported DOCX/PDF projections as the original file.
- Building radio from LLM summaries without source manifests.
- Making a Mac utility that cannot become the full Mac app.
- Silent conflict resolution.
- Local VM proof used as evidence for cloud worker/platform behavior.
- Test-only routes or raw event mutation for product acceptance.
- Treating "99% sync success" as acceptable.
- Hiding stuck sync state behind generic "up to date" or "waiting" labels.
- Using timestamps or path names as the only conflict/version truth.
- Allowing node-admin or local desktop bridges to mutate Base storage outside
  the server journal.

### Dense Feedback

- Unit tests for Base item/version/manifest models.
- API tests for upload, delta, conflict, import, export, and search.
- Deterministic reconciler tests over remote/local/synced trees.
- Fault-injection tests for reordered events, dropped notifications, crash
  between writes, locked files, corrupt local bytes, rename/move storms, and
  offline divergent edits.
- Sync status tests proving every stuck item has a visible reason and recovery
  path.
- macOS manual/Finder proof for File Provider hydration/edit/upload/conflict.
- Staging API proof for ContentItem/VText/source/radio interactions.
- Watch/iOS audio proof for background/resume/voice control.
- Run acceptance records for long platform changes.

### Stopping Condition For First Build Mission

Stop the first implementation mission only when a deployed or explicitly scoped
local-native proof shows:

- Base service stores versioned ContentItems with immutable blob refs and
  manifests;
- server metadata changes are append-only, cursor-addressable, and replayable;
- the sync planner models remote/local/synced trees and derives work from
  observations, not from ad hoc pending-work flags;
- deterministic fault tests prove create/edit/delete/move/rename/offline/crash
  scenarios converge or materialize explicit conflicts;
- every non-converged item has a legible status, last error, and recovery
  handle;
- existing VText/source paths still work;
- File Provider/Wails feasibility is recorded as evidence, but does not replace
  the Base sync proof;
- the remaining gaps are documented with exact evidence.

## Recommended First Build Slice

Build the Base sync nucleus first, then attach the native shell/File Provider
surface:

1. Document the Base model and add compatibility fields around current
   `ContentItem`.
2. Add immutable blob/version/manifest records without breaking
   `/api/content/items`.
3. Add an append-only metadata journal and cursor delta API over
   ContentItem create/update/delete/move.
4. Build the local sync planner around remote/local/synced trees.
5. Add deterministic fault tests before broad UI.
6. Add upload/download paths for binary bytes.
7. Add import/export manifest creation for VText DOCX/PDF/MD/TXT/HTML.
8. Add Wails v3 observer/control surface for sync status and node admin.
9. Scaffold the macOS File Provider extension against the delta API.
10. Prove one real Finder roundtrip only after the model-level proof passes.

If choosing "just do it," this is the path that preserves agentic momentum
without building a fake island.

## Desktop Shell Alternatives Research

The app shell choice should be evaluated as a local-compute substrate, not just
as a way to show the Svelte app.

### Strong Candidates

**Tauri v2** is no longer the default candidate, but it remains the strongest
fallback if Wails v3 fails a critical path. It supports existing Vite and
Svelte frontends, bundles static web assets, targets desktop and mobile, and
has a capability permission model. Its sidecar support can bundle external
binaries such as a Go runtime, Base sync daemon, local gateway, or model helper.
The downside is WebView variability: macOS/iOS use WebKit, Windows uses
WebView2, Android uses the system WebView, and Linux uses WebKitGTK/Wry. Choir
must specifically test passkeys, WebSockets, EventSource, PDF.js, media, and
desktop-window interactions under each target WebView.

**Wails** is worth serious consideration because Choir's backend is Go. Wails
is explicitly a Go + web frontend framework, has Svelte templates, generates
TypeScript models from Go structs, supports native menus/dialogs, uses Vite in
development, and uses the platform WebView rather than bundling Chromium. This
fits a "local Choir runtime as Go process" architecture better than Tauri's
Rust core. The tradeoff is platform reach: Wails is a desktop app framework,
not a path to iOS/Android, and it is less aligned with mobile shell reuse than
Tauri v2.

**Electron** is still the compatibility fallback. It bundles Chromium, has a
mature process model, mature packaging/signing tooling, and the largest native
module ecosystem. It is the safer choice if WebKit breaks critical Choir web
behavior, if passkey/auth behavior is more reliable in Chromium, or if browser
automation/devtools embedding becomes core to the desktop product. Its costs
are larger bundles, higher memory baseline, Node/native-module security risk,
and no mobile path.

### Secondary Candidates

**Neutralinojs** is attractive for a tiny utility but too thin for the main
Choir desktop. Its architecture is a lightweight C++ core, static web server,
WebSocket native API, and optional extensions in any language. That can work
for a menu-bar helper or ultra-light Base utility, but the main Choir app needs
native extension packaging, durable sidecars, local runtime supervision,
auth/session hardening, and eventually local candidate computers.

**NW.js** is an older Chromium + Node option. It allows Node modules directly
from the DOM and uses Chromium/Node. That is not the trust boundary Choir wants
for a user computer surface. Treat it as not recommended unless a very specific
legacy browser/runtime need appears.

**Flutter** is not a good reuse path for the current Svelte desktop. It is a
real cross-platform UI framework and may be useful for a future native-feeling
mobile radio/watch companion, but porting the Choir desktop to Flutter would
throw away too much web UI and app-surface work.

**SwiftUI/AppKit native shell** is attractive for File Provider, menu bar,
watch/iOS pairing, and Apple-platform polish. It is not the right first shell
for the full desktop unless the web desktop is hosted in `WKWebView` inside a
native app. The practical Apple-native role is: own the app bundle, entitlements,
File Provider extension, app group, keychain/session bridge, menus, and status
surfaces; embed the Svelte desktop as the main view.

**PWA/installable web app** is useful as the thinnest client, but it does not
provide Finder/File Provider, local runtimes, privileged filesystem sync, local
VMs, or App Store-native surfaces.

Reference links:

- Tauri create project: https://v2.tauri.app/start/create-project/
- Tauri Vite integration: https://v2.tauri.app/start/frontend/vite/
- Tauri sidecars: https://v2.tauri.app/develop/sidecar/
- Tauri capabilities: https://v2.tauri.app/security/capabilities/
- Tauri WebView versions: https://v2.tauri.app/reference/webview-versions/
- Wails introduction: https://wails.io/docs/v2.9.0/introduction/
- Wails architecture: https://v3.wails.io/concepts/architecture/
- Electron process model: https://www.electronjs.org/docs/latest/tutorial/process-model
- Electron security: https://www.electronjs.org/docs/latest/tutorial/security
- Electron code signing: https://www.electronjs.org/docs/latest/tutorial/code-signing
- Neutralino architecture: https://neutralino.js.org/docs/contributing/architecture/
- Neutralino introduction: https://neutralino.js.org/docs/
- NW.js docs: https://docs.nwjs.io/

## Thin-To-Local Continuum

The desktop app should run along a continuum. Do not split "cloud Choir" and
"local Choir" into different products. Make one computer policy choose where
each capability runs.

### Mode 0: Browser Thin Client

The user visits `https://choir.news`. Everything mutable runs in Choir cloud.
The local machine provides only browser UI, upload/download, audio/video
capture where allowed, and WebAuthn/passkeys.

Use for:

- weak machines;
- public or guest exploration;
- locked-down enterprise devices;
- mobile browsers;
- users who do not want local background processes.

### Mode 1: Packaged Thin Client

The native app wraps the deployed web desktop and adds OS affordances:

- app icon/window/menus;
- login/session/keychain bridge;
- deep links;
- notifications;
- file open/share handlers;
- optional menu-bar status;
- optional local cache for static assets.

Backend remains cloud. This is the fastest path to a useful Mac app and a good
baseline for weak machines.

### Mode 2: Base Sync Client

The app adds native File Provider and a small local sync process:

- device registration;
- Base delta cursor;
- blob upload/download;
- Finder hydration/materialization;
- conflict/tombstone handling;
- local encrypted cache;
- diagnostics and pause/resume.

The cloud remains source of truth for shared state. Local state is a cache plus
pending deltas.

### Mode 3: Local Runtime Mirror

The app runs selected Choir services locally as sidecars:

- Go runtime API for the user's active computer;
- embedded Dolt workspace for user app/VText state;
- local Base cache/blob store;
- local source extraction/import workers;
- local Super Console repair loop;
- local gateway proxy that can route to cloud or local models.

This mode allows offline-ish reading/editing and local private work while still
syncing through Base and using cloud for heavyweight workers or platform-owned
provider credentials.

### Mode 4: Local Candidate Computers

The app can create local candidate worlds for risky work:

- source/build worktrees;
- Dolt branches;
- app/runtime package builds;
- local verifier runs;
- preview route inside the desktop app;
- promotion certificate back into the user's active local/cloud computer.

On macOS, local Linux workers can use Apple's Virtualization framework directly
or a managed layer such as Lima/containerd. Apple's Virtualization framework
provides APIs for Linux/macOS VMs on Apple silicon and Intel Macs. Lima provides
Linux machines with automatic filesystem sharing and port forwarding and was
designed around containers on macOS. Docker Desktop is useful as an optional
integration, but Choir should not require Docker Desktop for ordinary users.

### Mode 5: Mostly Local Computer

The user's active computer runs locally by default:

- local Go runtime;
- local Dolt;
- local Base blobs and indexes;
- local source service;
- local app build/package loop;
- local candidates;
- cloud sync/backups/publication as explicit policy;
- cloud workers only for tasks exceeding local capability.

The product still records provenance and promotion certificates. "Local" cannot
mean "opaque machine accidents are canonical."

### Mode 6: Extreme Local / Local LLM

The user can run local models and route compatible turns locally:

- Ollama for easy model management and localhost API;
- llama.cpp or llama-cpp-python server for OpenAI-compatible local inference;
- MLX/MLX-LM on Apple silicon for high-performance Apple-native local models;
- Core ML / ExecuTorch paths later for packaged on-device models.

Provider routing should treat local LLMs as another provider family in the
platform catalog and per-computer model policy:

```text
platform catalog
-> per-computer model policy
-> task/modality/cost/privacy requirement
-> choose local model, cloud model, or hybrid
```

Local LLMs are appropriate for private summarization, low-risk drafting,
offline search over local Base, embeddings, source triage, and simple verifier
passes. They are not automatically suitable for high-stakes publication,
multimodal verification, complex coding, or source-grounded reasoning unless
the selected local model and evidence path prove that capability.

Reference links:

- Apple Virtualization framework: https://developer.apple.com/documentation/virtualization
- Lima: https://lima-vm.io/docs/
- Lima containers: https://lima-vm.io/docs/examples/containers/
- Docker Desktop: https://docs.docker.com/desktop/
- Docker Desktop Mac VMM: https://docs.docker.com/desktop/features/vmm/
- Ollama API: https://docs.ollama.com/api
- Ollama macOS: https://docs.ollama.com/macos
- llama-cpp-python OpenAI-compatible server: https://llama-cpp-python.readthedocs.io/en/stable/server/
- Apple Core ML overview: https://developer.apple.com/machine-learning/core-ml/
- Apple on-device Llama with Core ML: https://machinelearning.apple.com/research/core-ml-on-device-llama

## What Backend Comes Local First

Do not bring the whole platform backend local at once. Bring local pieces in
this order:

1. **Base sync daemon:** safest and most product-visible. It exercises auth,
   device identity, deltas, blobs, and conflict records.
2. **Static desktop bundle/cache:** lets the app open fast and survive transient
   network failure, while still calling cloud APIs.
3. **Local content extraction workers:** URL/file/PDF/DOCX/media extraction can
   run locally for privacy and speed, then write ContentItems/manifests.
4. **Local Dolt/runtime for VText and app state:** enables local private
   documents and offline-ish work. This must preserve sync/merge semantics.
5. **Local Super Console and app build loop:** useful for self-repair and
   personal app changes.
6. **Local candidate worlds:** only after typed deltas, verifier records, and
   promotion certificates are solid.
7. **Local model providers:** useful early as an optional provider, but not as
   the foundation for local correctness.
8. **Full local active computer:** last, because it combines all ledgers and
   promotion hazards.

The cloud-hosted pieces that should remain platform-owned even for local-first
users:

- account/auth authority;
- public routes/publication;
- shared platform package registry;
- provider secret custody unless the user deliberately configures local keys;
- platform model catalog defaults;
- billing/abuse/rate policy;
- cross-device backup/sync coordination.

## Updated Decision

Proceed with a **Wails v3-first desktop spike**.

The earlier Tauri recommendation weighted mobile shell reuse heavily. That
weight is lower now: the full Choir desktop/computer surface should not ship as
an iOS/iPadOS app because App Store policy and mobile UX both push against
embedded app stores, virtualization, and on-demand software environments. The
mobile product should instead be automatic radio, watch/iPhone control, file
provider, share/open flows, and cloud computer control. The deep desktop,
candidate, package, and node-admin surfaces can remain server-side or
desktop-native.

That makes Wails a better first desktop candidate than Tauri because Choir is
already Go + Svelte, and the v3 project shows the signs Choir wants in a
framework partner: active development, fast product pace, and explicit
AI-agent-aware contribution workflows.

```text
Svelte/Vite desktop
-> Wails bridge
-> Go local runtime / Base sync / node admin APIs
```

Default posture:

```text
Build on Wails v3.
Pin the exact alpha tag used by the spike.
When Choir hits framework-level breakage, reduce it to a minimal repro.
Contribute fixes or docs upstream where practical.
Keep local services framework-neutral so the shell can be swapped if necessary.
```

Wails v2 remains a contingency/reference baseline, not the primary path. Tauri
and Electron remain fallback shells if v3 fails a critical path such as
packaging, auth, media, windowing, file integration, or platform policy.

instead of:

```text
Svelte/Vite desktop
-> Tauri bridge
-> Rust shell
-> Go sidecars
```

Run one focused Wails spike:

1. Wrap the current Svelte/Vite desktop.
2. Test passkeys, cookies, WebSocket, EventSource, PDF.js, audio/video, and
   large VText editing in the system WebView.
3. Expose one local Go method to JavaScript and generate TypeScript bindings.
4. Add a tiny Base sync mock or local status endpoint.
5. Add a "Node Admin" prototype surface that can point at a Choir platform
   server/workstation node and read health/build/runtime status through
   authenticated product/admin APIs.
6. Test macOS app bundle, signing/notarization shape, and whether a native
   File Provider extension can live cleanly beside the Wails app bundle.
7. Record upstream Wails gaps as minimal reproducible issues if they block
   Choir. Contribute fixes upstream when the fix is clearly framework-level.

Keep Tauri as a fallback if Wails packaging/native-extension integration blocks
the File Provider or if Wails' WebView bridge cannot support the desktop app
cleanly. Keep Electron as the final compatibility escape hatch if system
WebViews cannot support critical Choir behavior.

## Desktop As Platform Node Control Pane

The Choir desktop app should also be a control pane for a Choir platform server,
workstation node, or self-hosted single-tenant node.

This is not the same as exposing host internals to ordinary web clients. It is
a signed desktop/admin surface that can be installed by an owner/operator and
paired with a node using explicit credentials and capability scopes.

Admin/control-pane capabilities should include:

- pair with a Choir platform node or workstation node;
- show node identity, deployed commit, build identity, and health;
- show runtime services and restart-safe status;
- show computer inventory at the owner/operator scope;
- inspect Base sync status, blob/cache pressure, and source-service health;
- inspect model provider availability and local/cloud routing policy;
- manage local worker capacity and optional local model providers;
- tail bounded logs and download diagnostic bundles;
- start safe maintenance actions through product/admin APIs;
- open the ordinary Choir desktop for a selected user/computer only through
  authorized product routes;
- never read provider secrets, raw host credentials, or private user content
  unless the capability explicitly grants it.

The control pane should use the same trust model as the rest of Choir:

```text
desktop admin app
-> authenticated admin/control API
-> node capability policy
-> durable action/evidence record
```

Avoid SSH-terminal-as-product and avoid direct database mutation. Super Console
can remain the repair surface inside a user computer; Node Admin is the
operator surface for the platform/workstation node.

## Risk Research Before Wails Spike

This section records risk-first research before building the desktop spike.

### 1. Wails Maturity And Version Risk

Wails v2 is the stable line. Wails v3 has a cleaner architecture and stronger
future direction, but its official roadmap still marks it as alpha. Community
issue traffic shows real v3 churn around window lifecycle, hidden/tray windows,
Windows WebView2 options, and startup/build regressions.

Observed risks:

- v3 APIs and behavior may change before beta;
- Windows WebView2 controller creation can crash for some launch options;
- window hide/show and tray-only behavior has had platform-specific crashes;
- focus and tab cycling issues affect both v2 and v3 on Windows;
- dev and production asset serving can differ enough to produce blank screens;
- WebView2 runtime version detection and embedding remain operational hazards.

How others handle it:

- pin Wails, Go, WebView2 loader, and frontend toolchain versions;
- build minimal repros before assuming app code is at fault;
- keep a CI matrix for local build and GitHub Actions build artifacts;
- test actual packaged apps on clean machines, not only `wails dev`;
- upstream small framework fixes when the repro is clear.

Sources:

- Wails v3 roadmap: https://v3alpha.wails.io/status/
- Wails architecture: https://v3.wails.io/concepts/architecture/
- Wails troubleshooting: https://wails.io/docs/v2.9.0/guides/troubleshooting
- Wails macOS packaging: https://v3.wails.io/guides/build/macos/
- Wails code signing: https://wails.io/docs/v2.10/guides/signing
- Windows focus issue: https://github.com/wailsapp/wails/issues/3783
- WebView2 launch crash issue: https://github.com/wailsapp/wails/issues/4559
- Hidden window issue: https://github.com/wailsapp/wails/issues/4498
- Go native WebView2 loader thread: https://github.com/wailsapp/wails/issues/2004

### 1a. Wails v2 vs v3 For Choir

Wails v3 is a rewrite, not a small version bump. The official migration guide
names the important differences:

- v2 uses one `wails.Run()` setup path; v3 separates application creation,
  window creation, and application execution.
- v2 binding commonly stores Wails context inside an app struct; v3 uses
  explicit services and dependency injection.
- v2 runtime operations are context-based global calls; v3 calls methods on app
  and window objects.
- v2 generated frontend bindings are organized by Go package/struct under
  `wailsjs/go`; v3 organizes bindings by app/service.
- v2 events are looser and variadic; v3 has typed event objects and stronger
  TypeScript generation.
- v2 is effectively single-window; v3 has first-class multi-window support.

Why this matters for Choir:

- **Node Admin wants services.** A Wails v3 service layout maps cleanly to
  `BaseSyncService`, `NodeAdminService`, `LocalRuntimeService`,
  `FileProviderBridgeService`, and `ModelProviderService`.
- **Desktop/admin wants multiple windows eventually.** The app may need a main
  desktop window, node-admin window, settings/pairing window, logs window, and
  background status/tray behavior. v3 is designed for that; v2 is not.
- **Testability matters.** Choir should keep Go business logic testable outside
  the Wails runtime. v3's service model is better for this.
- **Typed contracts matter.** Choir already suffers when runtime/frontend
  contracts drift. v3's improved bindings and typed events are directionally
  better.
- **Stability matters more than elegance for the first package.** v3 being alpha
  means the first production desktop should not depend on v3 unless the spike
  proves the required path and we are willing to absorb upstream churn.

Earlier conditional stance, now superseded by the Wails v3-first decision:

```text
Use Wails v3 as the primary shell.
Use Wails v2 only as a stability/control comparison if v3 fails suspiciously.
Contribute upstream fixes for framework-level bugs where feasible.
Keep the local Go services framework-neutral so they can move between v2/v3.
```

Sources:

- Wails v2 to v3 migration: https://v3.wails.io/migration/v2-to-v3/
- Wails v3 what's new: https://v3.wails.io/whats-new/
- Wails v3 events guide: https://v3.wails.io/guides/events-reference/
- Wails v2 event docs: https://wails.io/docs/reference/runtime/events/
- Wails v2 how it works: https://wails.io/docs/v2.10/howdoesitwork
- Wails v3 roadmap: https://v3alpha.wails.io/status/

### 1b. Wails v3 Alpha Duration, Velocity, And AI Posture

As of 2026-06-06, Wails v3 has been in alpha for roughly three years and four
months:

- `v3.0.0-alpha.0` was published on 2023-01-19.
- `v3.0.0-alpha.98` was published on 2026-06-03.
- There have been 99 tagged v3 alpha releases from `alpha.0` through
  `alpha.98`.

The raw duration makes v3 sound stalled, but the release shape is more nuanced.
Early v3 alpha releases were sparse: `alpha.1` did not arrive until 2023-10-28,
and `alpha.5` was not published until 2024-07-30. The 2026 cadence is much
faster: `alpha.74` was published on 2026-03-01 and `alpha.98` on 2026-06-03,
which is 25 alpha releases in about three months. In May 2026 alone, releases
advanced from `alpha.82` to `alpha.97`.

Development appears active, not dormant:

- the repository was pushed on 2026-06-06;
- recent June 2026 commits include Linux WebKit freeze fixes, macOS packaging
  fixes, bindings/build metadata fixes, and an experimental Go-native build
  runner;
- Wails v2 is still maintained, with `v2.12.0` published on 2026-03-26;
- the repository had about 34.6k stars, 1.7k forks, and 293 open issues when
  checked on 2026-06-06;
- open PRs include v3 documentation, mobile-support proposals, Windows
  refinements, and build/CLI improvements.

Interpretation for Choir:

```text
Wails v3 is high-momentum alpha software.
That is better than abandoned alpha software, but still not stable platform
infrastructure until Choir proves the exact path and pins a known-good version.
```

The right posture is to treat v3 as a serious spike target, not as an assumed
production dependency. A successful Choir spike should pin the alpha tag,
exercise package/auth/WebView/Base/node-admin behavior, and record every
upstream gap as a minimal repro. If the alpha churn is painful, Choir should
fall back to Wails v2 or Tauri for the shell while keeping local Go services
framework-neutral.

The Wails project does not appear anti-AI. The repository contains an
`AGENTS.md` titled "AI Agent Instructions for Wails v3", references GitHub
Copilot integration, includes a Claude Code GitHub Actions workflow, and uses
CodeRabbit configuration for automated review. That does not prove that the
maintainers want large unreviewed AI-generated patches, but the project is
explicitly AI-agent-aware and appears friendly to agent-assisted contribution
when patches are small, reproducible, tested, and fit the maintainer workflow.

Sources:

- Wails releases: https://github.com/wailsapp/wails/releases
- Wails commits: https://github.com/wailsapp/wails/commits/master
- Wails repository metadata: https://github.com/wailsapp/wails
- Wails AI agent instructions: https://github.com/wailsapp/wails/blob/master/AGENTS.md
- Wails Claude workflow: https://github.com/wailsapp/wails/blob/master/.github/workflows/claude.yml
- Wails CodeRabbit config: https://github.com/wailsapp/wails/blob/master/.coderabbit.yaml
- v3 beta-scope docs PR: https://github.com/wailsapp/wails/pull/5490

### 2. System WebView Compatibility Risk

Wails inherits system WebView behavior instead of bundling a known Chromium.
This keeps the app lighter but means Choir must test WebKit, WebView2, and
WebKitGTK behavior explicitly.

Choir-critical surfaces:

- WebAuthn/passkeys;
- cookies and secure session renewal;
- WebSocket live desktop sync;
- EventSource VText/document streams;
- PDF.js canvas rendering;
- audio/video/media permissions;
- drag/drop and file open;
- focus handling for prompt bar, VText, and desktop shortcuts.

Passkeys are the highest-risk item. General WebView passkey support has been
uneven, and many teams route authentication through a system browser or native
passkey API when embedded WebViews are unreliable. For desktop Wails, macOS
`WKWebView` and Windows WebView2 must be tested directly against Choir auth.

How others handle it:

- keep an explicit "open in browser" or native auth fallback;
- avoid passkey flows that depend only on conditional UI;
- instrument WebAuthn errors separately from user cancellation;
- verify HTTPS/origin/RP-ID assumptions under packaged-app origins;
- test clean-machine registration and login, not just existing cookies.

Sources:

- Can I WebView WebAuthn matrix: https://caniwebview.com/features/web-feature-webauthn/
- WebView passkey challenges: https://www.corbado.com/faq/webviews-challenge-passkeys-mobile-apps
- WebAuthn in iframes security context: https://web.dev/articles/webauthn-within-iframe
- WebView2 process failure handling: https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/process-related-events

### 3. macOS Packaging, Signing, And File Provider Risk

The Mac app is not just a Wails binary. The real product bundle needs:

- Wails app bundle;
- signed/notarized distribution;
- hardened runtime;
- native File Provider extension;
- entitlements;
- app-group/keychain/session sharing;
- possible helper/daemon launch items;
- diagnostics that do not leak secrets.

File Provider is a system contract with its own complexity. Apple requires the
replicated extension to adopt `NSFileProviderReplicatedExtension` and
`NSFileProviderEnumerating`. The sync model depends on working-set enumeration,
materialized/dataless state, signaling enumerators, and correct version/conflict
semantics.

How others handle it:

- start with one domain, not Desktop/Documents takeover;
- treat Finder proof as its own acceptance path;
- keep conflict copies explicit and user-visible;
- include pause/resume, status, and "why is this not syncing?" diagnostics;
- test eviction/materialization and offline behavior separately;
- publish known issues instead of pretending sync is magic.

Dropbox and Box support docs are useful product signals: real sync products
have user-facing sync dashboards, selective sync/online-only controls, conflict
copy explanations, and known-issue pages. Box specifically documents File
Provider Extension known issues, including spurious conflict copies for 0-byte
files. Nextcloud community threads show that eviction/free-space behavior can
confuse users when the provider and Finder do not agree about materialization.

Sources:

- Apple replicated File Provider: https://developer.apple.com/documentation/fileprovider/replicated-file-provider-extension
- Apple File Provider sync: https://developer.apple.com/documentation/FileProvider/synchronizing-the-file-provider-extension
- Dropbox desktop/sync help: https://learn.dropbox.com/self-guided-learning/dropbox-desktop-app
- Box File Provider known issues: https://support.box.com/hc/en-us/articles/4409609282579-Box-Drive-on-macOS-File-Provider-Extension-FPE-Known-Issues
- Nextcloud macOS VFS issue example: https://help.nextcloud.com/t/macos-vfs-cannot-free-up-local-space-files-never-become-virtual-context-menu-options-do-nothing/236508/4
- File Provider enumeration confusion example: https://stackoverflow.com/questions/78651951/file-provider-extension-not-enumerating-subfolders

### 4. Sync Semantics Risk

Sync systems fail when they pretend the filesystem is simple. Choir Base needs
rename/move/tombstone/conflict semantics from the start.

Risks:

- conflict copies from concurrent edits;
- stale local edits overwriting remote versions;
- rename-vs-delete ambiguity;
- case-insensitive macOS paths vs case-sensitive Linux/VM paths;
- symlinks and package directories;
- large files and partial uploads;
- "online-only" files opened by tools that expect POSIX semantics;
- local cache eviction while VText/source imports still reference artifacts.

How others handle it:

- selective sync/online-only as explicit policy;
- conflict files instead of silent overwrite;
- content hashes and version vectors;
- per-device cursors and acknowledgements;
- status surfaces that explain stuck sync;
- file-type exceptions for symlinks/packages/unsupported metadata.

Source signal:

- Dropbox sync docs emphasize selective sync, online-only, pause/resume, status,
  and conflict help.
- USENIX cloud-storage comparisons show major providers use different update
  and conflict strategies, and many keep conflicting versions rather than
  trying to choose silently.

Sources:

- Dropbox sync help: https://help.dropbox.com/sync
- Dropbox desktop app learning: https://learn.dropbox.com/self-guided-learning/dropbox-desktop-app
- USENIX cloud storage sync comparison proceedings: https://www.usenix.org/sites/default/files/fast20_full-proceedings-interior.pdf

### 5. Local VM / Candidate Computer Risk

Mac local workers are feasible, but not trivial. Apple Virtualization framework
provides high-level APIs for Linux/macOS VMs. Lima provides Linux machines with
automatic file sharing and port forwarding. Docker Desktop runs Linux
containers through a VM on macOS and documents file-sharing/networking
complexity.

Risks:

- host-to-VM file sharing performance;
- path permissions and symlink behavior;
- case-sensitivity mismatch;
- port forwarding and VPN interference;
- no Docker-style Linux containers without a Linux VM;
- GPU/nested virtualization limits;
- resource pressure on weak machines;
- unclear cleanup/rollback if a local candidate fails.

How others handle it:

- explicit resource limits;
- VM state directories separate from source/content ledgers;
- synced/cached source trees rather than heavy host bind mounts for hot paths;
- port-forward inventory and conflict detection;
- one local worker at a time by default;
- clear "cloud fallback" when local capacity is insufficient.

Sources:

- Apple Virtualization framework: https://developer.apple.com/documentation/virtualization
- Lima docs: https://lima-vm.io/docs/
- Lima container docs: https://lima-vm.io/docs/examples/containers/
- Docker Desktop: https://docs.docker.com/desktop/
- Docker synchronized file shares: https://docs.docker.com/desktop/synchronized-file-sharing/
- Docker Desktop Mac VMM: https://docs.docker.com/desktop/features/vmm/

### 5a. Local Isolation Options

There are four realistic local execution tiers. They should be policy choices,
not separate product identities.

#### Tier A: Host Process / macOS Sandbox

Run trusted local Choir services directly as signed app helpers or launch agents:
Base sync, content indexing, extraction, local admin API, and model-provider
brokers.

Pros:

- lightest and fastest;
- easiest access to Keychain, app group, File Provider state, local cache, and
  Apple APIs;
- easiest to package with Wails/Swift helper code;
- good fit for trusted Choir-owned services.

Cons:

- weak isolation for untrusted code or agent-written code;
- a bug in a helper can touch user files allowed by entitlements;
- not equivalent to server-side candidate computer isolation.

Use for:

- Base sync;
- File Provider bridge;
- node-admin control client;
- local content extraction for trusted parsers;
- local LLM provider broker.

#### Tier B: Containers In A Linux VM

Docker, containerd, nerdctl, or Apple container-style flows run Linux containers
inside a Linux VM on macOS. Docker Desktop is the polished commercial/dev UX.
Lima is a lower-level open-source way to run Linux VMs with file sharing and
port forwarding; it can run containerd/nerdctl and is used under tools such as
Colima/Rancher Desktop.

Pros:

- lighter than full per-worker VMs once a Linux VM is warm;
- OCI image ecosystem;
- good developer familiarity;
- useful for build/test jobs and reproducible toolchains;
- Lima gives us more control and less Docker Desktop dependency.

Cons:

- on macOS, Linux containers still require a Linux VM;
- containers share the VM kernel, so isolation is weaker than one VM per
  candidate;
- file sharing between macOS and Linux VM is a real performance and semantics
  problem;
- Docker Desktop is a third-party dependency with licensing/product assumptions;
- network/VPN/port-forward behavior needs product-grade diagnostics.

Use for:

- trusted-ish build/test jobs;
- package builds;
- source extraction that benefits from Linux tools;
- disposable local workers when full kernel isolation is not required.

#### Tier C: Apple Virtualization Framework VM Per Candidate

Use Apple's native Virtualization framework directly, or through a small helper,
to run Linux/macOS VMs. Apple's API validates VM configuration and requires the
`com.apple.security.virtualization` entitlement.

Pros:

- native Apple path and likely best long-term macOS integration;
- full guest kernel boundary per candidate if we choose one VM per candidate;
- clean mapping to Choir's server-side computer/candidate ontology;
- can model snapshot/rollback/resource policy more honestly than containers;
- avoids requiring Docker Desktop.

Cons:

- heavier than containers;
- more engineering to build image, networking, file sharing, logs, lifecycle,
  snapshots, and cleanup;
- GPU/nested-virtualization/device support is limited compared with a real
  Linux server;
- per-candidate VM startup and disk use may be too expensive for weak machines;
- Apple entitlement/signing behavior becomes part of the product surface.

Use for:

- risky agent-written code;
- local candidate computers;
- local verifier worlds where isolation matters;
- high-trust personal promotion proofs.

#### Tier D: Cloud/Server Worker

Keep execution on Choir cloud or a self-hosted workstation/server node.

Pros:

- same environment as current server-side candidate model;
- stronger capacity, observability, and existing gateway/provider boundaries;
- no local resource pressure;
- easier to enforce platform acceptance for shared behavior.

Cons:

- less private;
- requires network;
- costs cloud/server resources;
- weaker "my computer runs locally" feel.

Use for:

- weak machines;
- untrusted/risky work when local isolation is unavailable;
- heavy model/search/browser tasks;
- platform behavior-changing acceptance.

#### Native Path vs Docker/Lima Decision

The "Apple native" inclination is right for the durable product path, but it
does not mean skipping Lima/containerd entirely.

Recommended stance:

```text
Host helpers for trusted local services.
Lima/containerd for fast local build/test worker experiments.
Apple Virtualization framework for true local candidate computer isolation.
Cloud/server fallback for weak machines and heavyweight work.
```

This keeps homotopy with the server architecture: untrusted candidate mutation
still gets a kernel boundary when it matters, while lighter trusted work does
not pay VM overhead unnecessarily.

Sources:

- Apple Virtualization overview: https://developer.apple.com/documentation/virtualization
- `VZVirtualMachineConfiguration`: https://developer.apple.com/documentation/virtualization/vzvirtualmachineconfiguration
- Lima docs: https://lima-vm.io/docs/
- Lima VM types: https://lima-vm.io/docs/config/vmtype/
- Lima FAQ: https://lima-vm.io/docs/faq/
- Lima containers: https://lima-vm.io/docs/examples/containers/
- Docker Desktop: https://docs.docker.com/desktop/
- Docker Desktop networking: https://docs.docker.com/desktop/features/networking/
- Docker synchronized file sharing: https://docs.docker.com/desktop/synchronized-file-sharing/

### 6. Node Admin Security Risk

A desktop node-admin UI is powerful enough to become a security footgun if it
turns into a pretty SSH client or a bearer-token dashboard.

Risks:

- static admin tokens leaked from the app or diagnostics;
- overbroad "admin" role instead of capability-scoped actions;
- DNS rebinding or localhost service exposure;
- missing audit trail for maintenance actions;
- UI role mismatch where visibility implies authority;
- admin action bypassing product promotion/evidence records;
- private user content exposed to node operators by accident.

How others handle it:

- Tailscale emphasizes device approval, key expiry/rotation, auth key hygiene,
  role separation, and DNS rebinding defenses for private HTTP services.
- Kubernetes Dashboard history is a cautionary tale: admin dashboards exposed
  with weak auth or cluster-admin tokens become production liabilities; modern
  guidance stresses least-privilege RBAC, network restriction, and audit.

Choir implication:

```text
desktop admin app
-> paired device identity
-> short-lived/session-bound capability
-> product/admin API
-> durable action/evidence record
```

No raw DB writes. No long-lived root token in the app bundle. No admin UI
action without server-side authorization and audit.

Sources:

- Tailscale security best practices: https://tailscale.com/docs/reference/best-practices/security
- Tailscale node keys: https://tailscale.com/kb/1010/node-keys/
- Kubernetes dashboard docs: https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/
- Kubernetes dashboard security risk summary: https://www.portainer.io/blog/kubernetes-dashboard
- Kubernetes dashboard hardening guide: https://k8s-security.guru/kubernetes-security/best-practices/cluster-setup-and-hardening/network-security/exposed-dashboard-mitigation/

### 7. Local LLM Risk

Local models are attractive for privacy and cost, but they should not be treated
as a universal replacement for cloud providers.

Risks:

- weak local model quality for complex coding/verification;
- context and tool-use limits;
- model downloads consuming tens or hundreds of GB;
- unbounded CPU/GPU/RAM pressure;
- privacy confusion when a task silently falls back to cloud;
- local model server exposed on localhost/network without policy;
- provenance gaps if local inference is not recorded like other providers.

How others handle it:

- expose local LLMs through explicit localhost APIs such as Ollama or
  OpenAI-compatible llama.cpp servers;
- list installed models and capabilities;
- make cloud/local routing visible in policy;
- record model/provider choice and usage in run evidence;
- keep local model use opt-in or policy-driven.

Sources:

- Ollama API: https://docs.ollama.com/api
- Ollama macOS: https://docs.ollama.com/macos
- llama-cpp-python OpenAI-compatible server: https://llama-cpp-python.readthedocs.io/en/stable/server/
- Apple Core ML: https://developer.apple.com/machine-learning/core-ml/
- Apple on-device Llama/Core ML research: https://machinelearning.apple.com/research/core-ml-on-device-llama

## Risk-First Spike Gate

The first Wails spike should not be judged by "the app opens." It should pass
or precisely fail this gate:

```text
Wails packaged app
  + clean-machine auth/passkey proof
  + WebSocket/EventSource proof
  + PDF/media/VText proof
  + one Go bridge call
  + one local status/Base mock
  + one node-admin read-only pairing proof
  + macOS signing/notarization dry run
  + File Provider extension feasibility note
```

If any item fails, record whether the blocker is Wails-specific, system-WebView
specific, Apple packaging/File Provider specific, or Choir app-specific before
choosing Tauri/Electron/native fallback.
