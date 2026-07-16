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

## G1 frozen-candidate review receipt

A frozen candidate at base `a1d2f88c6a7135c8a1db916b6fb4f00acf43fb36`, patch SHA-256 `4028940b4fcd759441159fb8c38e69e2008fd1c79f36ca4a6e4973cd56dc7348`, received two accepts and one high-confidence repair. Under the mission's minority rule, G1 remains **repair**, not accepted. Durable local panel outputs: `/tmp/choir-g1-consensus-repair/{manifest.tsv,codex.out,claude.out,omp-gemini35.out}`.

The reproducible minority blockers are:

1. `CodeClosure` and `ArtifactProgram` validate declaration hashes but accept floating source commits, arbitrary claimed content digests, and mutable/nonexistent artifact URIs. The catalog therefore does not yet prove the referenced construction content immutable.
2. Production input resolution occurs outside the SQL route transaction. A catalog row can change between resolution and D-ROUTE CAS, leaving a committed route whose inputs no longer resolve.
3. Approval, promotion-certificate, rollback-receipt, and idempotency fields are separately named strings rather than non-interchangeable validated domain types.
4. `receiptMatchesCommand` omits the idempotency key from its exact command/receipt join.
5. `HandleHibernateWorker` inspects ownership before D-ROUTE resolution. Operator observation routes require an explicit non-activation exemption; every mutating lifecycle path must gate before ownership/VM access.

The first background-activation review also found startup reattach and warmness-policy bypasses. Those are repaired in the current working candidate: VM-manager binding no longer reattaches implicitly, reattach and warmers are route-guarded, authority initialization precedes activation, and focused refusal tests pass. No G1 acceptance or construction-phase transition is claimed until the five remaining blockers above are repaired and a newly frozen candidate is independently adjudicated.


### G1 second frozen-candidate adjudication

The repaired frozen candidate at the same base, patch SHA-256 `26b5336ab30e3e48c70de33bb7fb88ce625b596f157974b7a53a3fffd84fb9e5`, received an accept from Claude and a high-confidence repair from Codex; additional Devin/Cursor review was launched after the owner reported Claude rate-limit risk. The minority repair remains controlling. Local packet: `/tmp/choir-g1-consensus-final-repair/`.

Two blockers remain:

1. Background and operator-triggered bulk lifecycle mutation (`StartIdleSweeper`, reclaim, stale-state reclaim, retention prune, idle stop, and the corresponding HTTP handlers) enumerates and mutates computers without authorizing each affected owner/computer D-ROUTE first. Observation-only health/list/pulse/plan endpoints are exempt; mutation is not.
2. Go named string types prevent accidental assignment but current validation still permits evidence-domain substitution and does not resolve approval/certificate evidence. An idempotency key such as `approval:x` passes, and invented `approval:forged` / `certificate:forged` values can authorize a transition. G1 requires disjoint validated domains and durable evidence resolution before CAS.

Non-blocking findings retained for later axes: source-commit syntax alone does not prove Git object existence; catalog byte verification occurs at pin and must be repeated by the constructor before use; direct SQL permissions still need deployed least-privilege proof; late exact idempotent replay returns a historical receipt beside the current slot and needs explicit caller semantics.


### G1 third frozen-candidate adjudication — accepted

Frozen identity:

- base: `a1d2f88c6a7135c8a1db916b6fb4f00acf43fb36`
- patch SHA-256: `22a2ba443f14642b561249eff473a5e7696df6b460a562262c01fa6f014798eb`
- scope: 38 paths in `/tmp/choir-g1-immutable-authority.patch`

The third candidate repairs every reproducible minority blocker from the first two rounds:

- all bulk/background lifecycle mutation receives a per-computer D-ROUTE guard before mutation; identity-less orphan deletion is refused;
- approval and promotion-certificate evidence are separate durable records bound to complete hash-verified payload bytes, route slot, and ComputerVersion, preflighted by vmctl and re-resolved under the SQL CAS transaction;
- invented, cross-domain, wrong-slot, and wrong-ComputerVersion evidence is refused;
- production artifact pinning still verifies content bytes before catalog insertion; SQL route transitions re-resolve immutable inputs under the same serializable transaction and foreign keys protect routed catalog entries.

Deterministic evidence:

- `git diff --check` passed.
- `go test ./internal/routeledger ./internal/computerversion ./internal/vmctl ./internal/proxy ./internal/maild ./cmd/vmctl ./cmd/proxy -count=1` passed.
- `go test ./internal/routeledger -run 'TestSQLLedger(ConcurrentCASHasOneWinner|RefusesUnresolvableInputsAndProtectsRoutedCatalogRows)' -count=5` passed.
- `scripts/doccheck -mode live` passed.
- Node B Nix evaluation places `VMCTL_ROUTE_DSN` in the existing `platform` Dolt database; no third store was introduced.

Independent frozen review:

- Gemini 3.5 Flash: **accept**, no blockers, high confidence (`/tmp/choir-g1-consensus-final-22a2/omp-gemini35.out`).
- OMP GPT-5.5: **accept**, no blockers, high confidence; independently applied the patch to an isolated worktree and reran the focused suite (`/tmp/choir-g1-consensus-final-alternates/omp-gpt55.out`).
- Codex reached model capacity after inspecting the candidate and emitted no verdict. OpenCode was denied `/tmp` patch access and emitted no verdict. GLM timed out. These are reviewer failures, not votes.

Adjudication: **G1 accepted**. The immutable-input/state-authority boundary is admitted. Phase C mutation remains gated on landing this red change through origin/main, successful CI/deploy, matching Node B identity, and a deployed route-authority smoke receipt.

Heresy delta for G1: `repaired` — competing static/lineage served-route authority and unguarded VM lifecycle activation are removed from the frozen candidate; `introduced` — none observed; `discovered` — production SQL least-privilege grants and mutable runtime-package construction remain realism axes for later phases, not G1 authority blockers.


### G1 landing receipt

- Pushed commit: `7d310551c01dd5c63be3dcbb641dd752a201d8d6`.
- CI: GitHub Actions run `29480269240`, attempt 2, success. Attempt 1 exposed the existing unrelated `TestCancelRunTrajectoryDrainsMoreThanOneActivePage` Dolt deadline flake; the failed shard passed unchanged on rerun.
- Deploy: Node B job `87564091427`, success; activation receipt published at `2026-07-16T07:50:08Z` for commit `7d310551c01dd5c63be3dcbb641dd752a201d8d6`.
- Staging identity: uncached `https://choir.news/health?g1=7d310551` reports proxy/vmctl healthy and build/deployed commit `7d310551c01dd5c63be3dcbb641dd752a201d8d6`.
- Deployed route refusal: Node B loopback GET of `/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer:g1-missing:primary` with the internal caller header returned `HTTP 404` and `route ledger: slot not found`. Route authority is initialized and absence is refusal, not fallback.
- Deployed inventory at activation: `0` active VMs, `149` ownerships (`147` hibernated, `1` stopped, `1` failed). No D-ROUTE CAS was executed.
- Rollback ref: prior origin/main and deployment `a1d2f88c6a7135c8a1db916b6fb4f00acf43fb36`; deployment activation receipt preserves the previous artifact set.

G1 is landed. Phase C may now mutate constructor and disk-instantiation code. G2 remains mandatory before verifier/promotion work.


### Phase C constructor preflight problem checkpoint

Captured: 2026-07-16T08:39:29Z

Mutation class: **red**. Protected surfaces: production Firecracker construction and vmctl immutable-input delivery. Rollback remains commit-level restoration to landed G1 (`7d310551c01dd5c63be3dcbb641dd752a201d8d6`); no route CAS or existing realization mutation is authorized in Phase C.

Before committing constructor code, focused implementation and adversarial review identified the following failures in the proposed Phase C path:

1. The immutable runtime-package endpoint digest-verified bytes but streamed an unvalidated tar archive into a guest `tar -xf` boot path. A content-addressed but structurally unsafe archive could traverse or link outside the intended runtime directory.
2. Product-path construction readback followed filesystem symlinks. A substituted manifest path or parent component could make equivalence inspect bytes outside the constructed `/files` projection rather than refuse.
3. The legacy VM launch contract named only an opaque store-disk path. It lacked a constructor-owned, attach-only device path and immutable CodeRef binding, so a backend-created device could not be joined to guest boot without reintroducing raw-image semantics into the materializer.

These are substrate failures at the construction boundary, not isolated guest symptoms. The existing `StateGenerator`, typed `ComputerVersion`, VM manager, and immutable artifact catalog remain the replacement substrate; the repair must connect them through one materializer rather than patch the old reboot path.

Conjecture delta: C2 and C3 remain unproven; the preflight adds concrete falsifiers for unsafe immutable archive structure and symlink-following readback. Heresy delta: `discovered` — two unsafe boundary behaviors and one missing typed launch seam; `introduced` — none in the landed system because this candidate is not committed or deployed; `repaired` — none until the candidate is independently reviewed and exercised on staging. Admissible evidence remains focused deterministic checks, frozen G2 independent review, then deployed Firecracker construction/readback receipts.

### Phase C terminal G2 repair checkpoint

- Captured: `2026-07-16T09:59:28Z`
- Frozen candidate before this repair: base `0856b898`; patch `/tmp/choir-g2-construction-repaired.patch`; SHA-256 `0eb94f757631289b1271cab6374ea22600a6a4d616ff317f85c54ec42e55ba51`; 25 staged paths.
- Independent packet: `/tmp/choir-g2-consensus-terminal/` (5 successful reviewers, 2 unavailable). Verdicts included one accept and four reproducible `repair` findings; minority blockers govern.
- Newly documented blockers before repair commit:
  1. Ext4 reclaim removed `data.img` but retained its realization directory, preventing deterministic retry with the same realization identity.
  2. A failed constructed ownership could enter the ordinary failed-to-fresh legacy `BootVM` branch without immutable device or CodeRef bindings.
  3. vmctl restart reattachment did not project the durable construction device and CodeRef into manager metadata.
  4. Registration/stop/destruction failures did not fate-share with disk reclaim, permitting a live or uncertain VM to lose its backing path.
  5. Persisted constructed ownership accepted absent/corrupt or pre-commit bindings, and D-ROUTE exact comparison skipped a missing version.
  6. The production launcher imposed an untyped `<StateDir>/<VMID>/data.img` convention that defeated conforming backend substitution.
- Classification: lifecycle/persistence/backend substrate defects, not semantic projection symptoms.
- Route/promotion state: no D-ROUTE CAS, promotion, or production mutation executed.
- Rollback: prior `origin/main` plus the frozen patch identities above; subsequent repair remains an unpublished candidate until a new G2 frozen review accepts it.

### Phase C lifecycle root-cause cluster

- Captured: `2026-07-16T10:10:23Z`
- Trigger: the terminal repair review `/tmp/choir-g2-consensus-terminal-repair/` accepted the prior six repairs in four successful verdicts but produced one reproducible minority `repair` verdict. This is the third lifecycle-cleanup review iteration; incremental cleanup patches stop here.
- Shared substrate cause: construction spans four separately authoritative moments—disk instantiation, VM-manager launch, vmctl ownership persistence, and post-boot evidence commit—but the durable lifecycle record begins only after VM launch. `BootVM` can also return an error after creating manager/process state while the launcher reports no boot identity. The gap makes both error returns and process crashes capable of producing a VM with no durable construction intent.
- Dependency graph: `ProductionMaterializer` owns the typed disk and cleanup decision -> `VMConstructionLauncher` sequences lifecycle -> `vmmanager.BootVM` may create process/manager state -> `OwnershipRegistry` currently persists only after boot -> final `Commit` seals observed disk evidence. D-ROUTE and restart recovery consume only `OwnershipRegistry`.
- Substrate classification: lifecycle transaction boundary, not a disk-reclaim or route symptom. No replacement implementation exists to wire in.
- Structural repair selected under the Phase C authority: persist an uncommitted construction intent before `BootVM`; update that same record with host/epoch only after boot; retain it as failed on any uncertain cleanup; remove it only after confirmed stop/state destruction; always return the realization identity when `BootVM` may have created state. Post-boot `Commit` remains the sole finalized transition.
- Admissible proof: focused crash/error-order tests must show pre-boot durability, no identity-free BootVM error, no disk reclaim while VM cleanup is uncertain, and fail-closed restart/legacy replacement behavior; then a newly frozen independent G2 review.
- Rollback: source remains unpublished; prior checkpoint `28a07e59` and frozen patch `3b5e51728becc0b3aabb1f8975285b78dd31ba3a20b847230863be6ddf1eb949` identify the pre-structural candidate.
- Route/promotion state: no D-ROUTE CAS, promotion, or production mutation executed.

### G2 accepted constructor round-trip

- Frozen base: `449f84f0`.
- Frozen candidate: `/tmp/choir-g2-construction-churn-proven.patch`; SHA-256 `439da6ce2c7e20f451c185e9d377868693b28d201ec0cac45b74fbcb95de4278`; 26 staged paths.
- Deterministic evidence: all affected focused Go suites and `go vet` passed; Node B Nix vmctl environment evaluation passed; real e2fsprogs execution passed `TestExt4BackendChurnReclaimReconstructionBound`, including debugfs cache write/delete, physical allocation accounting, receipt-bound root reclaim, same-ID reconstruction, and the 2 GiB policy bound.
- Independent gate packet: `/tmp/choir-g2-consensus-churn-proven/`. Codex, Cursor, GPT-5.5, and OpenCode adjudicated `accept` with no blocking findings. Gemini and GLM were unavailable and did not contribute votes. The reproducible-minority rule found no remaining blocker.
- Accepted boundary: immutable exact-one CodeRef/runtime and ArtifactProgram replay; substrate-independent generated semantic state; fresh sparse ext4 realization with independent pre/post host geometry and guest statfs joins; explicit allocation/headroom/reclaim policy; durable pre-Boot construction intent; finalized-only route/reattach lifecycle; identity-preserving cleanup uncertainty; no prior image read/clone/grow, third store, route CAS, or promotion.
- Residual G2 risks carried forward: ownership write/rename lacks host-power-loss fsync proof; uncommitted crash residue requires explicit cleanup; deployed Node B geometry/reconstruction remains mandatory; constructed disk lifetime outside failed-construction cleanup must be joined in later destruction/fleet lifecycle acceptance.
- Decision: G2 accepted. Phase D verifier/promotion/route work may begin. No D-ROUTE CAS is authorized until G3 accepts its frozen candidate.
