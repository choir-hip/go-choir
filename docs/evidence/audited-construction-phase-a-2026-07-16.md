# Audited Construction Phase A — Containment and Extraction Receipt

Captured: 2026-07-16T04:39Z
Mission: `docs/definitions/choir-audited-autoputer-construction-2026-07-15.md`
Source: `d87bdc446ecc28585c3bc08d4d469b9f94d3c246`
Staging runtime: `9d9945e65f5b54069e1a86a530cb0960d96b3474` (the cumulative delta is classified `deploy_needed=false`)

## Protected owner realization

The read-only Node B storage proof classified owner `yusefnathanson@me.com`, user ID `5bd6de97-3b58-408c-bf89-c42c81b083de`, as a protected refusal. Its VM `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` was stopped. The state directory allocated 31.74 GiB: 15.76 GiB in `data.img` and 15.98 GiB in two existing manual snapshots.

Both snapshots now have valid `choir.manual-data-img-snapshot.v1` sidecars. They remain deletion-refused until recovery settlement, rollback proof, and owner approval:

- `data.img.pre-e2fsck-20260703T182100Z`: 16 GiB logical, 7.99 GiB allocated; preserved as the read-only extraction source and never a constructor input.
- `data.img.rollback-20260704T0426Z`: 32 GiB logical, 7.99 GiB allocated; preserved as rollback evidence and never a constructor input.

Post-mutation `scripts/node-b-storage-proof` passed. Its report recorded `metadata_missing_count=0`, `metadata_present_count=2`, both sidecars `typed_sidecar_valid`, the VM still stopped, and the protected identity still `refuse_delete`. Local captured report: `/tmp/choir-audited-construction-contained/node-b-storage-report.{md,json}`.

## Bounded read-only extraction

The attempt read only `data.img.pre-e2fsck-20260703T182100Z`. `debugfs` extracted into `/tmp/choir-owner-extract-20260716` on Node B; it did not mount or modify the snapshot. Source identity before and after matched exactly: inode `62506737`, logical size `17179869184`, mtime epoch `1783102912`, allocated blocks `16749080`.

Recovered classes:

| Class | Result |
| --- | --- |
| Legacy Texture Dolt working set | SQL export, 61 MiB, SHA-256 `f0e9f62f4571408d997cd795c1f266ff9178a927757bf11fa36f8591a4875eba` |
| Legacy VText Dolt working set | SQL export, 434 MiB, SHA-256 `5a99adf4e1aff3f3600da3ce0f1e6794879e01d5de12a5cfbf515dea2e05cf25` |
| Actor recovery database | 24 KiB, SHA-256 `82cdd14b36786e010cbce82e71cee17950b0ec483eabb5ad91a7fe2333733e6e`; 2 snapshots and 8 updates |
| User-visible `/files` tree | 92 MiB extracted, including VText/Texture aliases, source documents, PDF/PPTX, and user-created files |
| Prompt defaults | 60 KiB extracted |

The two SQL exports imported successfully into fresh Dolt databases. Independent readback found 39 Texture tables and 63 VText tables. The legacy repositories' committed `main` heads were initialization commits (`9kk091379ud9o9dhihvci5sapdft4adh` and `2fikvsn8f8j8d9hs558eokpr6tbfc7ok`); all application tables existed in their working sets rather than a pinned commit. Therefore the SQL export hashes identify the bounded recovery payload, while those initialization heads do **not** bind the recovered semantic state and cannot serve as `ArtifactProgramRef` values.

## Near-new control

The read-only vmctl lookup reconciled `a@b.com`, user ID `0e5c45ab-44de-49cd-b07d-e58973b21ad5`, to active primary VM `vm-d067e51c904a6fc6b7810ec7dee75ad1`, epoch 25, staging runtime `9d9945e65f5b54069e1a86a530cb0960d96b3474`. Guest health was `ready` with no running work.

The control also exposes the storage-geometry defect: vmctl reports a 32 GiB data-image cap and 503,889,920 file bytes, while guest statfs reports only 8,350,298,112 total bytes. It is useful as an early healthy-boot control, but it is not constructor acceptance.

## Scope and omissions

This is legacy recovery evidence, not product acceptance. It used SSH, host files, and a test-only extraction shape; it cannot prove no-SSH operability, a production materializer, authenticated readback, ComputerVersion binding, or route authority. The failed current `data.img` was not read or repaired. The bounded attempt did not claim every cache, runtime binary, Go build artifact, or opaque local accident as durable state. Recovered payloads remain unaccepted until an immutable typed authority resolves them, pins their hashes, and a fresh realization reproduces the selected observations through the product path.

## Construction authority map

| Required class | Settled authority | Current production path | Phase-A disposition |
| --- | --- | --- | --- |
| Immutable executable closure (`CodeRef`) | Immutable source/build identity, with `origin/main` only as a mutable integration ref until pinned | `computerversion.CodeRef`; deployed build identity; source lineage metadata | **Blocker:** no `CodeRef` resolver or closure store exists in production. Search found no `ResolveCodeRef`/`CodeRefResolver`; the type alone is not a resolver. |
| Immutable ordered semantic program (`ArtifactProgramRef`) | A tamper-evident typed program that captures acknowledged durable state | `computerversion.ArtifactProgramRef`; Base journal/tree/blob adapters exist only as code-present substrate | **Blocker:** no production resolver exists. `StateGenerator.Generate` explicitly trusts the caller's journal binding; LSP found only test callers. The legacy SQL hashes are recovery payload identities, not accepted refs. |
| VM-local app/Texture/ObjectGraph state | The one VM-local embedded Dolt workspace per post-cutover computer | `internal/sandbox/run.go` opens `internal/store.Store`; `internal/store/store.go` shares that Dolt connection with `internal/objectgraph.DoltStore` | **Blocker:** current acknowledged writes are not pinned into an immutable artifact program. The legacy owner extraction found two historical workspaces (Texture and VText); they must be reconciled into the selected typed program, not perpetuated as post-cutover stores. |
| Content blobs and user files | Content-addressed blobs plus typed metadata in the semantic program | Base blob/tree adapters under `internal/base` and `internal/computerversion`; current legacy `/files` tree | **Blocker:** Base observation/generation code has no production caller, and not every acknowledged file/blob currently resolves from a pinned artifact program. |
| Actor recovery | Narrow actor-update log and compacted activation snapshots only | `state-actor.db`; `internal/actorruntime` | Recovered as a separately hashed 24 KiB SQLite artifact. It remains recovery-only and cannot become app, route, or promotion truth. |
| Served route identity | D-ROUTE tables on the corpusd world-wire sql-server; vmctl is the sole CAS writer | Settled contract in `og-dolt-heresy-completion-2026-07-08.md`; current proxy lineage/hard-coded owner/desktop resolver | **Blocker:** route-slot and immutable transition-receipt tables/CAS port are absent. Current lineage and fallback routing violate the authority. |
| World Wire public/object graph | Corpusd world-wire sql-server (the first of exactly two Dolt stores) | `internal/platform/objectgraph_store.go`, `cmd/corpusd` | Existing authority; it may host D-ROUTE tables but must not absorb VM-local semantic app state. |
| Construction/verification evidence | Immutable receipts keyed by ComputerVersion and realization ID; evidence only | `internal/computerversion` realization, observation, promotion-certificate types | **Blocker:** contracts are code-present but not durably stored or joined to production lifecycle. |
| Realization-local disk policy/device | Typed policy resolver and subordinate backend receipt; never ComputerVersion identity | No production contract/backend exists | **Blocker:** current vmmanager directly owns `data.img` lifecycle and geometry. Backend, policy, allocation, reclaim, and geometry receipts must be added behind the materializer boundary. |
| Auth/session and provider credentials | Existing platform auth/gateway authorities, referenced as external capabilities | auth, gateway, per-computer model policy | External named dependencies only; never copied into `ArtifactProgramRef`, a disk image, or a new state store. |
| Caches, logs, build outputs, runtime binaries | Disposable realization-local acceleration state | `data.img`, runtime workspace, Go/package caches | Explicit omission from durable state. Recompute from immutable code/program inputs or discard. |

### Existing replacement connection

`internal/computerversion` already supplies the sole identity type, typed observations/equivalence, Base journal/tree/blob extraction, `StateGenerator`, VM-manager state classification, and promotion-certificate vocabulary. These are not wired into production: LSP found `StateGenerator.Generate` only in tests and `VMManagerScopedMaterializer.Materialize` only in `cmd/vmrealize` plus tests; that materializer explicitly does not launch a VM. Phase B/C must connect and complete this substrate rather than patching the opaque reboot loop or creating another constructor.

The production route remains the superseded path: `internal/proxy/route_resolver.go` falls back to hard-coded platform owner/desktop constants, `LineageBasedRouteResolver` reads `RouteProfile`, `cmd/proxy/main.go` can open `PROXY_RUNTIME_DB_PATH`, promotion and candidate-package services rewrite `ActiveSourceRef`/`RouteProfile`, and vmctl recovery reboots the same VM ID/data disk. D-ROUTE's settled deletion list governs the clean cutover.
