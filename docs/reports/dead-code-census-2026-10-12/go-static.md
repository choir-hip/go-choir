# Go static reachability census (read-only)

Repo `/Users/wiz/go-choir` at HEAD `c30147a6` (main, clean tree at start). Nothing in the repo was edited or committed. All scratch work (a vendor-stubbed copy of the source, raw tool output) lives in the scratchpad directory `(census scratch) `.

## Commands run

```
# 1. darwin/arm64 host build (cgo + ICU from ~/.bashenv)
go install golang.org/x/tools/cmd/deadcode@latest         # -> ~/go/bin/deadcode
~/go/bin/deadcode -test=false ./...     # 758 unreachable funcs (13.5k lines)   raw: dc-notest.txt
~/go/bin/deadcode -test ./...           # 390 unreachable funcs                  raw: dc-test.txt
~/go/bin/staticcheck -checks U1000 ./...                # 293 findings           raw: sc.txt

# 2. linux/amd64 (the production target) -- CANONICAL numbers below
#    Dolt needs cgo (gozstd, ICU), so a cross-build fails. I made a copy of go.mod/internal/cmd/scripts/vendor
#    in the scratchpad, replaced vendor/github.com/dolthub/gozstd with a 12-line stub, and ran:
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 deadcode -test=false -tags gms_pure_go ./...   # 667 funcs   raw: dcL-notest.txt
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 deadcode -test       -tags gms_pure_go ./...   # 373 funcs   raw: dcL-test.txt
GOOS=linux ... deadcode -test -tags gms_pure_go,integration,experiment ./...         # 370 (only 3 fewer)
GOOS=linux ... staticcheck -tags gms_pure_go -checks U1000 ./...                     # 297 findings raw: scL.txt
# 3. packages: go list -deps ./cmd/<each> (non-test) vs go list ./internal/... ; cmd usage: grep of flake.nix,
#    nix/, scripts/, start-services.sh, .github/ for each cmd/ name
```

Why a linux pass was necessary: the darwin run is wrong for this repo. `internal/capsule`, `cmd/capsule-broker`, `cmd/choir-updater`, `yaegikernel/desk_worker_*_linux.go`, `updater/kernel_probe_linux.go` and `autoputer/capsule_executor_linux.go` are `//go:build linux`. On darwin 104 functions (57 in capsule, 39 in updater) were falsely "dead" and 9+4 capsule/broker functions were falsely "live". Use the linux numbers. `cmd/desktop` is a separate Go module (`cmd/desktop/go.mod`, Wails) and is not in `./...`; it imports no `internal/` package.

## Totals (linux, canonical)

| Measure | Funcs | Lines in those funcs |
|---|---|---|
| Unreachable from every `main`, tests not counted (`-test=false`) | 667 | 11,212 |
| of which **dead even with tests as roots** ("DEAD") | 320 | 5,209 |
| of which **used only by tests** ("TESTONLY") | 347 | 6,003 |
| staticcheck U1000 (unused identifiers, tests included) | 297 (235 funcs, 33 types, 17 consts, 8 fields, 4 vars) | 62 are in `_test.go` files |
| of those, not already found by deadcode | 115 (types/consts/fields/vars and test helpers) | |
| internal/ packages with zero non-test importers | 5 (`base/testkit`, `desktop`, `diskinstantiation`, `recovery`, `trustedoutbox`) | 1,920 prod lines |
| internal/ packages reachable only from undeployed `cmd/*` | 3 (`base/api`, `base/planner`, `cmdutil`) | |
| internal/ packages reachable only from `cmd/sourcecycled` (deployed, default-off) | 2 (`cycle`, `wire/processorkey`) + the cmd itself | 3,475 prod lines |
| `cmd/*` with no nix/script/CI/deploy reference | 6 (`basecompare`, `baseharness`, `baseobserve`, `choir-rebuild-base`, `runtime-ratchet`, the empty dir `choir-checkpointd`) | |

Reflective-use check (affects the "dead" verdicts): the yaegi exports in `internal/yaegikernel/choir.go` are hand-written `reflect.ValueOf(s.Method)` tables (static references, so deadcode sees them) plus `choirScopeType.MethodByName` in `surface.go` (only `ChoirScope` methods, none are in the dead list). There are no generated yaegi symbol files (`go:generate`/`yaegi extract` absent), no `go:linkname`, no `plugin`, no JSON-RPC dispatch tables. `text/template` is used only by `internal/promptspec` (prompt bodies); template data that exposes a dead exported method by name is the one theoretical gap (low risk; prompt data is maps). Not detectable by any tool used: dead exported types/consts/vars (only unexported are checked by U1000), and functions "kept alive" only by other dead code in the same cluster (deadcode already handles that transitively).

Caveat on removal: several "dead" methods exist to satisfy an interface (`GatewayClient.Name/IsReal`, `TCPChecker.Name`, `RedactingStore.*`, `BridgeProvider.*`, `Runtime.Drain`, `HandlerFunc.HandleUpdate`). Deleting them requires editing the interface too.

## Item index (largest first; prod lines, tests excluded unless noted)

| # | Item | Dead / test-only funcs | Lines | Scheduled |
|---|---|---|---|---|
| 1 | `cmd/sourcecycled` + `internal/cycle` + `wire/processorkey`: dormant source-ingestion daemon | 41 dead (of ~3.5k) | 3,475 pkg | no |
| 2 | Base file-sync stack: `internal/base/{api,planner,testkit}`, `internal/desktop`, `cmd/base{harness,compare,observe}`, `internal/cmdutil` | ~60 | ~2,425 pkg (+~2.3k tests) | no |
| 3 | `internal/computerversion` proof machinery (isomorphism, TreeToFS, promotion certificate, ...) | 12 dead + 39 test-only | 1,342 funcs | no |
| 4 | `internal/store` SQLite-era row scanners and helpers | 41 dead + 11 test-only | 1,030 + 227 | no |
| 5 | Orphaned packages: `diskinstantiation`, `recovery`, `trustedoutbox` | 0 + 30 | 1,152 pkg (+801 tests) | no |
| 6 | `internal/pii` + trace PII redaction/HTTP/SQLite | 7 dead + 44 test-only | 747 | no |
| 7 | Wire processor/reconciler remnants in `agentcore` (+ `qdrant_dedup`) | 29 dead | 744 | partly Cut 1 |
| 8 | Coagent source-packet helper layer | 34 dead | 665 | partly Cut 1 |
| 9 | `internal/provider` bridge providers + OpenAI-responses leftovers | 7 dead + 15 test-only | 527 | no |
| 10 | `internal/yaegikernel` test-only subsystems | 2 dead + 20 test-only | 426 | no |
| 11 | `internal/maild` host spool queue + drain worker | 1 dead + 15 test-only | 354 | no |
| 12 | `internal/gateway` GatewayClient (HTTP client for the gateway) + constructors | 5 dead + 7 test-only | 288 | no (Cut 2 touches search only) |
| 13 | `agentcore/run_acceptance.go` trace-scanning helpers | 10 | 221 | no (file is Cut 1 protected, not deleted) |
| 14 | `internal/capsule` + `cmd/capsule-broker` leftovers | 18 dead + 6 test-only | 215 + 46 | no |
| 15 | `agentcore/tools_capsule.go` capsule tool family never registered | 6 dead + 1 test-only | 144 | no |
| 16 | `agentcore/value_helpers.go` | 12 | 113 | no |
| 17 | `agentcore` + `textureowner` + others: stray helpers | ~117 | ~1,164 (see appendix) | no |
| 18 | Unwired `cmd/*` binaries (`runtime-ratchet`, `choir-rebuild-base`) | n/a | 1,761 + 383 | no |
| 19 | Test files behind the never-run `comprehensive` build tag | n/a | 6,014 test lines | no |
| 20 | Smaller test-only clusters (vmctl, vocabmigrate, routeledger, decisionpolicy, types, platform, ...) | ~150 test-only | see below | no |
| 21 | Repo-hygiene finds: tracked 32 MB binary `capsule-broker`, tracked `result` symlink, 43 GB ignored `specs/states` | n/a | n/a | no |

---

## 1. Dormant source-ingestion daemon: `cmd/sourcecycled` + `internal/cycle` + `internal/wire/processorkey`

- **Location**: `cmd/sourcecycled/main.go` (760 lines, +525 test), `internal/cycle/` (2,619 prod lines: `storage.go` 1,564, `mem_store.go` 659, `cycle.go`, `ingestion_event.go`, `synthesize.go`, `store.go`, `web_capture_graph.go`; +987 test), `internal/wire/processorkey/` (96 lines).
- **What it does/did**: the "source service" ledger daemon (born 2026-06-04, "feat: add source service ledger nucleus"). It polls the source registry, writes cycles/fetches/items to a Dolt-backed `cycle.Storage`, projects items to the object graph, and used to dispatch typed processor/reconciler requests to the wire (Universal Wire / "World Wire", now retired name for Autopaper) desks. The processor/reconciler pipeline was deleted end to end in `3c963edf` (2026-10-05).
- **Why dead/dormant**: it is built (`flake.nix` goServiceSubPackages) and defined in `nix/node-b.nix` but **deliberately not in the boot set**: "pure fetch-debt burning corpus-dolt (WW corpus burn)", durable default-off since `ee013c9a` 2026-10-05; CI explicitly omits it from health/identity probes. Statically it is reachable (a real `main`), so deadcode does not flag it. Within it deadcode flags 41 funcs as unreachable even with tests: `MemoryStore` (32 funcs, 580 lines, in-memory Store impl; last `cycle.NewMemoryStore` call removed in `3c963edf`), `Synthesizer` (52), `Engine.PollAll/Cluster` (19), `ingestionEventIDsForItems` (21), and in `processorkey` `SourceProcessorKey/SafeKeyPart/OrderedSourceItemIDs/ProcessorHandoffPrompt` (64 lines; frozen-history recognition kept by `3c963edf`, nothing calls them). `cycle`, `processorkey` and `cmd/sourcecycled` are imported only by each other.
- **Citers**: tests `internal/cycle/*_test.go`, `cmd/sourcecycled/main_test.go`; docs `docs/autopaper-sw-design-2026-10-09.md` §9 (code rename TODO: sourcecycled is a retired name), CLAUDE.md itself names "sourcecycled" as retired; flake.nix, nix/node-b.nix:608-625, ci.yml:1313/1522/1536 (comments); `internal/sources` (live) and `sourceapi` (live, imported by agentcore/textureowner) are used by this daemon but also by the research desk, so they stay.
- **Already scheduled?**: no. Cut 1/Cut 2/metamission/decisions never mention `sourcecycled` or `internal/cycle` (grep: 0 hits).
- **Proposal**: owner question first: is Autopaper going to reuse this daemon's polling/ledger, or is source polling now `researchtools/poll_source.go` + `agentcore/source_schedule.go` + corpusd captures (Cut 2 treats those as "the one capture writer")? If the latter, remove the whole stack (3.5k lines + 1.5k tests) and the flake/nix/CI mentions. Minimum safe step: delete the 41 statically dead funcs (`MemoryStore`, `Synthesizer`, `Engine.Cluster`, processorkey helpers).
- **Risk**: nix service unit and flake subPackage list reference it, so removal must touch `flake.nix` (goServiceSubPackages, `sourcecycled = mkGoService`), `nix/node-b.nix`, `deploy-impact-classify`, ci.yml comments. The Dolt "WW corpus queue tables" are read-only history (3c963edf message); `cycle` storage readers are kept "for replay of stored records". Check no deploy path starts it by name (`systemctl start`) before deleting.

## 2. Base file-sync stack

- **Location and size**: `internal/base/api` (616 prod, 446 test), `internal/base/planner` (499, 399), `internal/base/testkit` (357, 200; zero importers at all), `internal/desktop` (411, 130; zero non-test importers), `cmd/baseharness` (245, 304), `cmd/basecompare` (81, 160), `cmd/baseobserve` (108, 182), `internal/cmdutil` (108; only `basecompare` imports it). Roughly 2.4k prod + 2.3k test lines. (`internal/base/{blob,journal,model,tree}` are still imported by `computerversion`'s `base_*` extractors, so they are live today, but see item 3.)
- **What it does/did**: "Choir Base", the Dropbox-like file-sync product of June 2026: a pure three-tree reconciliation kernel (`planner`, M2), append-only journal and tree derivation (M3), content-addressed blob store and REST API for Base items with API-key auth (`base/api`, M4), a Wails desktop sync engine with keychain storage and File Provider IPC (`internal/desktop`, M5/M6, Swift side in `macos/`), plus three harness/compare CLIs added with the substrate-computer landing surfaces (`30f0301f`, 2026-07-04).
- **Why dead**: `7b59d33e` (2026-07-10, "dead-code: ... unregistered SyncService, and desktop SyncEngine subtree") removed the desktop sync engine and its callers; `base/api` lost its last production importer in `7d635330` (2026-07-19, "freeze disabled audited cutover", -48k lines). `cmd/desktop` (separate module) imports no `internal/` package. The three base CLIs have no nix/script/CI reference. Statically: `internal/desktop` has 8 dead + 12 test-only funcs (Keychain/File key stores, `BaseClient`), `base/testkit` 14 test-only (scenario fixtures), `MemJournal` 8, `planner.ApplyEvent` 2.
- **Citers**: tests in each package; docs: `docs/archive/choir-seam-repair-2026-07-10.md`, `docs/archive/choir-tape-recovery-2026-08-13.md`, `docs/reviews/*-consensus-2026-08-29.md`, `docs/evidence/choir-rlm-v1-inventory-corpus-2026-09-10.txt` (history only; 2 non-archive docs mention `internal/base`, 2 mention `internal/desktop`, 16 mention "File Provider"); `macos/ChoirFileProvider*` (7 tracked files).
- **Already scheduled?**: no.
- **Proposal**: owner question: is Base sync a product that returns? If not, remove `base/api`, `planner`, `testkit`, `internal/desktop`, `cmd/base*`, `cmdutil`, and the `macos/` File Provider (another agent's area). Keep or fold `base/{blob,journal,model,tree}` into computerversion only if item 3 survives.
- **Risk**: `macos/` Swift code and the 1.5-commit README promise; frontend may still call `/api/base/*` (not checked: not Go). Nothing reflective.

## 3. `internal/computerversion` proof machinery

- **Location**: `internal/computerversion/` is 3,878 prod lines; within it: `tree_to_fs.go` (7 test-only funcs, 444 lines: `TreeToFS`, `generateIntoDir`, symlink-safety checks), `object_graph_snapshot.go` (128), `user_isomorphism.go` (107), `filesystem_projection.go` (105 test-only + 54 dead), `promotion_certificate.go` (102), `state_generator.go` (61 + 23 dead), `artifact_program_binding.go` (52 + 7 dead), `dolt_head_snapshot.go` (47 + 23 dead), `dolt_state_extractor.go` (24), `observation_bundle.go` (`CombineObservationSets`, 57 dead), `construction_state.go` (53 dead), `contract_header.go` (34 dead), small ones. 12 dead + 39 test-only funcs, 1,342 lines.
- **What it does/did**: the SIAC "substrate-independent audited computer" proof framework: generate a filesystem from a ComputerVersion tree, extract observations from a realization (Dolt heads, object-graph snapshots, file manifests), and check "user isomorphism" and promotion certificates across substrates. Added 2026-07-04..07-16 (`30f0301f`, `24886d24`, `0dc3fea3` "construct audited disposable realizations").
- **Why dead**: the audited-construction route that called `ConstructionState`/`TreeToFS`/filesystem projection was frozen in `7d635330` (2026-07-19, vmctl `construction_launcher.go` etc. deleted). What remains live in `computerversion` is used by `selfdevprotocol`, `routeledger`, `projectionbase/witness.go`, `agentcore/rematerialize.go`, `platform_update.go`, `cmd/vmctl`; the proof/generator half is exercised only by its own tests (and `cmd/basecompare`, `cmd/baseobserve`, item 2).
- **Citers**: tests (`state_generator_test.go`, `tree_to_fs_security_test.go`, `user_isomorphism_test.go`, `promotion_certificate_test.go`, `object_graph_snapshot_test.go`, ...); docs: 33 files mention `computerversion`, 36 live docs mention ComputerVersion as a concept (the type stays); `docs/evidence/audited-construction-phase-*.md`.
- **Already scheduled?**: no.
- **Proposal**: owner question (is the audited-construction / isomorphism proof line still wanted?). The 12 dead funcs are removable now; the 39 test-only ones go with their tests if the answer is no. Keep the ComputerVersion ref types.
- **Risk**: `Verify`-style names are common; double-check callers of `contract_header.go` (`ValidateContractHeader` dead) in `selfdevprotocol`. No reflection.

## 4. `internal/store`: SQLite-era row scanners

- **Location**: `store.go` (11 funcs, 298 lines: `scanRun`, `scanAgent`, `scanEvent`, `scanChannelMessage`, `scanInboxDelivery`, `scanWorkerUpdate`, `workerUpdateSelectSQL`, `listRunsWhere`, ...), `texture_source_graph.go` (10, 216: the whole `writeTextureSourceGraph`/`putTextureSource*GraphRecord` write path), `texture.go` (`scanDocument/Revision/ContentItem`, 135), `run_acceptance.go` (5, 110), `trajectory.go` (`scanTrajectory`, `scanWorkItem`, 88), `browser.go` (61), `podcast.go` (44), `continuations.go` (41), plus `queryDesktopPlacement`, `lifecycleControlActivationFailureEvidence`, `actorWakeRunContentJSON`, `rawJSONOrDefault`. 41 dead funcs / 1,030 lines. Test-only: `OpenTextureWorkspace`, `deleteDocumentPhysicalForTest`, `SeedEngineeringAssignmentAuthority` (74, a test fixture living in a prod file), arrival-ordinal helpers (11 funcs, 227).
- **What it does/did**: `database/sql` row decoders and SQL builders from the original relational store (2026-04-11 foundation). Persistence moved to the object-graph/Dolt store.
- **Why dead**: orphaned by `63dca14d` (2026-07-02 "complete OG-only cutover") and `b307dfab` (2026-07-12 "store: delete relational objectgraph replay"), and the lifecycle consume-at-commit rewrite `dc108d7e` (2026-09-28) for `scanAgent`. U1000 also lists unused `rowScanner` type, `selectTrajectoryByID`, `workItemColumns` consts, `actorWakeRunContent` type.
- **Citers**: tests none for the scanners themselves (that is why they are DEAD, not TESTONLY); `SeedEngineeringAssignmentAuthority` is used by 9 agentcore/store tests; docs: Cut 1 lists `store/run_acceptance.go` and `store/lifecycle*.go` as protected surfaces (so edits need red-class ceremony even for dead code), `scanWorkItem` is not named.
- **Already scheduled?**: no for the scanners; Cut 1 deletes work items and packets (`store/trajectory.go` WorkItem path) but lists the scanner funcs nowhere. Check overlap with Cut 1's work-item deletion before cutting `scanWorkItem` separately.
- **Proposal**: remove the 41 dead funcs (mechanical; compile-checked). Move `SeedEngineeringAssignmentAuthority` to a `_test.go` file (it is exported only for cross-package tests: keep).
- **Risk**: `store/run_acceptance.go` and `store/lifecycle.go` are protected (red) surfaces per Cut 1; no reflection.

## 5. Orphaned packages: `diskinstantiation`, `recovery`, `trustedoutbox`

- **`internal/diskinstantiation`** (509 prod + 363 test): substrate-neutral ext4 disk plan/receipt/geometry verification (`Ext4Backend.Instantiate`, 100 lines, `Plan.Validate`, `VerifyReceipt`, `FinalizeReceipt`). Added 2026-07-16 (`0dc3fea3`) for audited construction; last importer removed in `7d635330` (2026-07-19). 14 test-only funcs, zero importers. Docs: only `docs/evidence/audited-construction-phase-d-2026-07-16.md` and archive/reports. Not scheduled. Proposal: remove (owner question tied to item 3).
- **`internal/recovery`** (336 + 170): self-describing recovery capsule metadata (`BuildCapsule`, `RecoveryCapsule.Verify`), restore drill runner, background integrity scrub. Added 2026-08-26 (`150b0f64`); never had a production importer (`git log -S` finds none). Docs: `docs/problems/s0-storage-lifecycle-gaps-2026-10-05.md` and `fresh-realization-skips-file-hydration-2026-10-09.md` discuss recovery gaps (these may describe this as the intended-but-unwired fix). Not scheduled. Proposal: owner question: wire in (storage lifecycle work) or delete.
- **`internal/trustedoutbox`** (307 + 268): trusted-outbox dispatch for `irreversible-email-v1` decision policy (red, 2026-08-15 `3141b90b`; only importer is `agentcore/effects_rehearsal_test.go`). 9 test-only funcs (`Outbox.Dispatch` 108 lines, `Reconcile`). Docs: `docs/evidence/effects-trusted-outbox-2026-08-16.md`. Not scheduled. Proposal: owner question (effects rehearsal is not live); tied with `decisionpolicy` (item 20).
- **Risk**: none reflective. `trustedoutbox` embeds `decisionpolicy` bytes; check `decisionpolicy` after removal.

## 6. `internal/pii` + trace redaction/HTTP/SQLite

- **Location**: `internal/pii/` (845 prod: regex and SLM redactors, `Pipeline`; 29 test-only funcs, 501 lines + 1 dead), `internal/trace/redact.go` (`NewRedactingStore`, `NewPipelineFromConfig`, 73 lines test-only + 11 dead), `internal/trace/query.go` (`NewHTTPHandler`, `Queries.ParentChain`, 136 test-only), `internal/trace/store.go` `NewSQLiteStore` (20).
- **What it does/did**: M20/M21 (2026-06-28): Dolt-persisted trace store with a PII redaction pipeline applied at ingestion, plus an HTTP read API. The Dolt store is live (agentcore/actorruntime/autoputer import `trace`), but the redacting wrapper, HTTP query handler and SQLite store are not constructed anywhere in production. `internal/pii` is imported only by `trace/redact.go`.
- **Why dead**: wired in `9e976da6` ("wire PII redaction into trace ingestion path") but no production caller of `NewRedactingStore` exists at HEAD; `2fbecebc` (2026-09-25 test purge) removed the last references from tests I could trace. Cut 0 mentions `internal/trace` (3 hits) in the context of the trace export/tracer; confirm Cut 0's new trace export does not intend to reuse `NewHTTPHandler`/redaction before deleting.
- **Citers**: `internal/pii/*_test.go`, `internal/trace/redact_test.go`, `query_test.go`, `internal/qdrant/qdrant_test.go` (uses `pii.NewPipeline`); docs minimal.
- **Already scheduled?**: no (Cut 0 touches trace but does not name these).
- **Proposal**: owner question: is PII redaction of traces a required property before trace export (Cut 0)? If yes, wire it; if not, remove `pii` and the redact/HTTP/SQLite parts.
- **Risk**: `qdrant_test.go` imports pii; `trace.Store` interface methods on `RedactingStore` exist for interface satisfaction.

## 7. Wire processor/reconciler remnants in `agentcore`

- **Location**: `agentcore/wire_publication.go` (10 dead funcs, 373 lines of 659: `beginWireProcessorDecisionWorkItem`, `recordWireProcessorDecision`, `reconcileWireProcessorRequestResolution`, `beginWireStoryResolutionWorkItem`, ...), `wire_source_items.go` (3, 74), `wire_metadata.go` (3, 23), `universal_wire.go` (`wirePlatformRoutePath`, 18), `qdrant_dedup.go` (7, 137, Qdrant semantic dedup of source items) + `qdrant_runtime.go closeQdrantPipeline` (8), `api.go` (internal-run-submission fingerprint funcs 47: `hashInternalRunSubmission`, `internalRunSubmissionFingerprint`, `legacyInternalIngestionSubmissionFingerprint`; `authenticatedUserEmail` is unrelated), `lifecycle_control_issue.go` packet-id helpers (6). 29 dead funcs, 680 lines; plus 64 in `wire/processorkey` (item 1). U1000 adds: `wireProcessorDecisionVerdict/Update` types, 5 `wireProcessorDecision*` consts, `qdrantDedupResult`, `internalRunSubmissionFingerprintMetadataKey`, `internalIngestionSubmissionLegacyFingerprintKeys`, `textureMetadataPromptUnixTS`.
- **What it does/did**: the typed wire-processor decision work items (opened_texture / already_covered / not_newsworthy ...), source-item decision fan-out, and idempotent typed ingestion for the "processor/reconciler" desks of Universal Wire.
- **Why dead**: `3c963edf` (2026-10-05, "delete processor/reconciler pipeline end to end - four desks carry the work") removed every caller but left these bodies (and kept some "for frozen history"). Same commit orphaned `qdrant_dedup`'s last caller.
- **Citers**: no tests (their tests were deleted in the same commit); docs: Cut 1 (`cut1` lines 84, 126-130, 204, 289, 645-646), `docs/re-engineering-decisions-2026-10-12.md` line 398.
- **Already scheduled?**: **Cut 1** for the decision work-item part (cut1 table: "wire-processor decision work items (wire_publication.go:149-260) ... deleted with the Universal Wire path") and for `MaybeAutonomousPublishWireArticle`, `recoverOpenWirePublicationClaims` (live today, in the same file, not in the dead set); **no** for `wire_source_items.go`, `wire_metadata.go`, `universal_wire.go`, `qdrant_dedup.go`, the api.go fingerprints.
- **Proposal**: remove the dead functions now (do not wait for Cut 1; they are already uncalled, so deleting them reduces Cut 1's surface). The live remainder of `wire_publication.go` (publication claim recovery, autonomous publish, `beginWirePublicationWorkItem`) belongs to Cut 1.
- **Risk**: Cut 1 plans `wire_publication.go:21-260` line-number citations; deleting now shifts those lines (docs cite line numbers). `wirepublish` package (354 lines) is still live via `MaybeAutonomousPublishWireArticle`.

## 8. Coagent source-packet helper layer

- **Location**: `agentcore/tools_worker_update.go` (15 dead funcs, 308 of 1,195 lines: `CoagentSourcePacketPayloadSchema`, `coagentSourcePacketPayloadProperties` 125, `rejectLegacyUpdateCoagentFields` 52, `coagentSourceFromTypedEvidenceRef(s)`, packet summary/kind/notes accessors), `textureowner/value_helpers.go` (7, 120: `newCoagentPacket`, `coagentSourceFromURI`, `coagentClaimsFromTexts`, ...), `textureowner/texture_media_sources.go` (6, 138: media source enrichment and `formatTextureMediaSourceRefsForPrompt`), `textureowner/texture_evidence_sources.go` (24), `agentcore/management_controller.go` (`buildCoagentBacklogPrompt`, `buildCoagentUpdatePrompt`, `buildPersistentManagementUpdatePrompt`, 75). 34 funcs / 665 lines. Test-only: `coagentSourceFromURI/coagentClaim/coagentAction` (33).
- **What it does/did**: builders for `types.CoagentSourcePacket` payloads and the `update_coagent` tool schema; "source-centric update_coagent cutover" (`be52b194` 2026-06-21). Texture media-source prompt enrichment (2026-05-30).
- **Why dead**: `update_coagent` registration was deleted in `3c963edf`; "one packet contract" in `19b7ef48` (2026-10-09). The rest of `tools_worker_update.go` (authority resolvers) is still live.
- **Citers**: `types.CoagentSourcePacket` (live type), Cut 1 line 41 and the grep gate on `CoagentSourcePacket` (line 289); `docs/` mentions in many reports.
- **Already scheduled?**: **Cut 1, partly**: Cut 1 deletes packets entirely ("Packets ... deleted with every reader") and adds a grep gate for `CoagentSourcePacket`/`update_coagent`; these funcs are already dead so they are removable before Cut 1 starts. `texture_media_sources.go` is **not** mentioned in either cut.
- **Proposal**: remove now.
- **Risk**: `textureowner` and `agentcore` have duplicate copies of the same helpers (`coagentSourcePacketPayload*`, `sanitizeExportPart`, `looksLikeArtifactPath` exist in both): delete both. No reflection.

## 9. `internal/provider` bridge providers

- **Location**: `provider/bridge.go` (381 test-only lines: `BridgeProvider`, `GatewayBridgeProvider` `Execute`/`CallWithTools`, `convertRawMessages` 82; +53 dead: `RuntimeProviderPolicy` x2, trivial accessors), `provider/provider.go` (`parseOpenAIResponse` 45, `anthropicThinkingForReasoning` 19 dead; `NewFireworksProviderFromEnv`, `ResolveProvider` test-only). U1000: unused `openAIResponse`, `openAIUsage`, `openAIResponseItem`, `openAIResponseContent` types.
- **What it does/did**: the 2026-04-11 "real-provider bridge" (Bedrock, Z.AI) and gateway bridge for the runtime; superseded by `gatewayruntime.Provider` and `provideriface` (live `RuntimeProviderPolicy` is on `gatewayruntime/provider.go:57`).
- **Why dead**: no production constructor; last non-test use removed during the R3d texture-RLM cutover `3ed94220` (2026-09-26) per `git log -S`, and the gateway runtime replaced it. Tests keep it alive (`provider_test.go`, `provider_integration_test.go`, `gateway_bridge_test.go`).
- **Already scheduled?**: no. Cut 2 mentions `internal/modelcatalog` for model comparison, not these. M0 model-pools (`b0d69d4d`, today) is routing-level.
- **Proposal**: remove after confirming `provider.go` stays the home of the live providers (Fireworks, Anthropic, OpenAI) used by the gateway; owner question only if the integration tests (`-tags integration`) are still run manually.
- **Risk**: provider routing is "orange/red" per CLAUDE.md; compile-time only.

## 10. `internal/yaegikernel` test-only subsystems (not the yaegi export table)

- **Location**: `actor_state.go` (`ActorStateManager`, 6 funcs, 119 lines: durable actor state and cross-model rewarm, Phase 2 `6a783351`), `eval.go` (`Evaluator.Eval`, 94: the original in-process evaluator, Phase 0 `94232b6c`), `transclusion.go` (`ReceiptCollector`, 55: salient receipts to Texture, Phase 3 `3d88dcc9`), `sidecar.go` (`SidecarRunner.RunSubprocess` 67, `ExecuteWorkerStdin` 25, dead `RunInProcess` 8), `session_loop.go` (`RunSessionLoop*`, 37 + dead `RunSessionLoopFramed` 3), `transport.go` `SocketPair` (23), `inspect_bundle.go` (3). 2 dead + 20 test-only funcs.
- **What it does/did**: August 2026 yaegi sidecar milestones (process-per-activation sidecar, actor state, CoSuper/Researcher transclusion) that the desk-RLM design replaced with `session.go`/`choir.go`/`broker.go`. They are exercised by `actor_state_test.go`, `transclusion_test.go`, `containment_test.go`, `refusal_test.go`, `session_test.go`, `transport_test.go`, `egress_roundtrip_test.go`, `cmd/capsule-broker/session_worker_test.go` (`SocketPair`).
- **Why test-only**: production wiring went through `desk_session_worker.go`/`broker.go`; the 2026-09-25 test purge (`2fbecebc`) trimmed but did not remove these.
- **Already scheduled?**: no (Cut 1 touches `internal/yaegikernel` as protected "desk act path" surface; Cut 2 touches `session_loop.go` SessionFrame, line 25).
- **Proposal**: remove `ActorStateManager`, `Evaluator`, `ReceiptCollector`, `SidecarRunner`, unused `RunSessionLoop*`; keep `SocketPair` (tests in capsule-broker use it; could move to a test file).
- **Risk**: highest-care area because of yaegi exports: confirmed the export table (`choir.go:ChoirExports`) is static and does not reference any of these; confirm `allowlist.go` and `surface.go` have no string reference (checked: only `ChoirScope` methods).

## 11. `internal/maild` host spool queue and drain worker

- `maild/spool.go` (9 test-only funcs, 242 lines: fsync'd host MTA spool `SpoolQueue`, `Enqueue`, `FetchPending`, `PurgeDeliveredBefore`) and `maild/drain.go` (`DrainWorker`, 79); added together 2026-08-26 (`08c95f00`, "host fsyncd MTA spool queue, async drain worker, and guest Maildir storage"). Not constructed in `cmd/maild` or `Handler`; only `spool_test.go`/`drain_test.go`. U1000: unused `mu` field in spool. 1 dead (`draftAttachmentMaxBytes`), plus `Store.CountDraftApprovalEvents` (17) and 2 small store funcs test-only. Not scheduled. Proposal: owner question: was the spool queue the planned inbound/outbound mail design (mail docs `docs/` mail-*)? If mail goes via Resend/Gandi paths only, remove. Risk: maild is deployed (flake `maild`, `maildctl`); verify no CLI subcommand references the spool by string (`cmd/maildctl` flags).

## 12. `internal/gateway` GatewayClient

- `gateway/client.go`: `NewGatewayClient`, `GatewayClient.Call` (110 lines), `Stream` (49), `parseGatewaySSE` (71), `setAuthorization` (8); `NewHandler`/`NewMultiHandler` (18) in `handlers.go`; plus dead `Name/IsReal` (2), `intFromEnv` (11), `truncateSearchAttemptError` (7), `drainBody` (5). Test-only 263 + dead 25. Added 2026-04-11 (`fc7a29fc`, "host-side gateway provider boundary"); consumers moved to `gatewayruntime`/`hostlink`. Tests: `gateway/gateway_test.go`. Cut 2 touches the gateway search path (search plane / `HandleSearch`), not the client. Not scheduled. Proposal: remove client + constructors after checking `gateway_test.go` is the only reason they exist. U1000 also flags `searchProviderBatch` type.

## 13. `agentcore/run_acceptance.go` helpers

- 10 dead funcs, 221 lines (`collectAcceptanceToolErrors(Any)`, `collectAcceptancePendingToolInvocations(Any)`, `traceToolErrorText`, `acceptanceOutputSlice`, `acceptanceStringSlice/AnyMap`, `acceptanceAppPackageSummaries`, `acceptanceBuilder.addRollbackRef`) + unused types `acceptanceToolError`, `acceptanceToolInvocation`. Added 2026-05-13 (`11a9e4ea` "Add durable run acceptance verifier"); callers removed progressively (`28f6f190` 2026-05-16 added, last touched by `7d635330` 2026-07-19). Cut 1 lists `run_acceptance.go` as protected (kept, with a path-B branch under review), so the dead helpers can go but the file stays. Not scheduled. Risk: protected red surface; trivial compile-only delete.

## 14. `internal/capsule` and `cmd/capsule-broker` (linux)

- Dead: `capsule/diagnostics.go` (7 funcs, 107: `InspectUpperDir`, `SafeOpenFile`, `ReadProcStatus`, overlayfs-era upperdir inspection), `capsule/namespace.go` (`CreateCapsuleNamespaces`, `NamespaceSet.Do/Close`, `SetupUserNamespaceMappings`, 40), `capsule/capability.go` (`UnmarshalFromTransport`, `CapabilitiesEqual`, `signingPayloadBytes`, 17), `cmd/capsule-broker/main.go` (`handleSyncRevokedCaps` 17, `closeNonStdioFDs` 26, `stdoutWriter/stderrWriter` 8). Test-only (46): `UnmountOverlayFS`, `executor.NewExecutor*`, `writeCapsuleIdentityEtc`. All from the v14 capsule runtime (`138626e9`, 2026-07-05). `namespace.go` was last referenced in docs commit `7da99b14` (builder substrate = host-service; "privileged-builder-capsule needs new machinery"), i.e. the namespace-based capsule isolation was replaced by landlock/seccomp. Not scheduled. Proposal: remove dead ones. Risk: darwin deadcode run reports these differently; verify with a linux build (done above).

## 15. `agentcore/tools_capsule.go` capsule tool family never registered

- `RegisterCapsuleTools` (10), `newSpawnCapsuleTool` (52), `newDestroyCapsuleTool` (27), `newListCapsulesTool` (14), `newInspectCapsuleTool` (24), `randomCapsuleID` (7): 134 lines, plus `RegisterCapsuleLocalTools` test-only (10, used by `tool_profiles_authority_test.go`). Added `138626e9` 2026-07-05; its `RegisterCapsuleTools(` call disappeared in `7d635330` (2026-07-19). The rest of `tools_capsule.go` (the Verify-binding / capsule exec path, Cut 1 cites lines 251-270) is live. Cut 1 mentions the file for the live part. Proposal: remove the dead tool family. Risk: tool names (`spawn_capsule`, `destroy_capsule`, ...) may still appear in prompts: grep `internal/promptstore internal/runtimeprompts` found no references; tool-profile allowlists should be re-grepped before cut.

## 16. `agentcore/value_helpers.go`

- 12 dead funcs, 113 lines (`rawJSONOrFallback`, `sha256Hex`, `digestParts`, `computerKindForID`, `candidateSourceRefForComputer`, `normalizeRouteProfile` (22), `stringFromMap`, `executionTargetKind`, `looksLikeArtifactPath`, `isHTTPURL`, `sanitizeExportPart`, `intMapValue`). Created by the `c791a0ae` runtime-package extraction (2026-07-14) from older code; `normalizeRouteProfile` last touched by `f9cc3246` (2026-07-19). Not scheduled. Remove (whole file may be deletable; it appears to hold only these 12).

## 17. Stray unused helpers (lumped per package)

Counts are DEAD funcs (unreachable even with tests); names are in the appendix.

- `internal/agentcore`: 19 funcs, 291 lines in `runtime.go` (`latestActorToolLoopBudgetSpend` 52, `channelHasGroundedHistory` 36, `mergeStoredConductorRoute` 26, `fallbackPromptBarInitialContent` 26, `buildAssignedWorkItemPrompt`), `tools_research.go` (`ensureFindingEvidence` 39 + 3 tiny), `engineering_assignment_runtime.go` (`runtimePersistentManagementControlID` 35), `tool_profiles.go`, `lifecycle_control_issue.go`, `replay_completeness.go`, ... Orphaned over time (`460c1423` 2026-08-05 "Remove rejected Texture supervision architecture", `95196069` 2026-05-31, `c791a0ae`). Test-only agentcore (11 funcs/107): `persistEvent`, `LoadInboxCursor`, `TextureRecoveryOccurrence`, `deskSessionWorkers.count/closeAll`. Cut 1 will delete `rlm_reduce.go` inbox cursor parts (`LoadInboxCursor` test-only: scheduled Cut 1: envelope mailbox and cursor).
- `internal/textureowner`: dead 11 funcs / 205 lines (`toolExecutionContextForRun`, `agentIDForRun`, `channelIDForRun`, `desktopIDForRun` (63; orphaned `3c963edf`), `extractTextureProjectionFromDOCX/PDF` (59; import helpers from `f03bf14a`, orphaned `05162395` 2026-06-16), `recentWorkerMessages`, `userRevisionDiffSummaries` (55; orphaned `460c1423`), `countRawJSONItems`, ...); test-only 9/165 (`textureActorToolLoopBudget` 47, `initialTextureToolChoice`, `emitTextureDecisionRecordedEvent` 41 ...). Cut 1 covers `texture_turn_runtime.go`/`texture_agent_revision.go` as protected surfaces (`textureTurnAuditDigest` dead).
- `internal/platform`: dead 9/92: `control_keys.go` write path (`insertControlKey`, `activateControlKey`, `revokeControlKey` + nullable helpers, 71). The key resolver/reader is live, but nothing can register/activate/revoke a key (added 7d635330 2026-07-19 and never called): owner question: where do control keys come from (nix-seeded SQL?). Also `execAll`, `summarizeSQL`, `ListGCNamespaces`. Test-only: `VerifyRevisionSignature`, `HandlePublicTexture`; U1000 `statement` type.
- `internal/vmctl`: dead 14/57: nine 3-line exported `*Endpoint` wrappers in `handlers.go` (`HibernateEndpoint`, `ResumeEndpoint`, ...), `OwnershipRegistry.RemoveOwnership/SetAutoputerCredential`, `Handler.SetColdRecoveryStateRoot/RecoveryLease`, `RouteAuthority.Transition`. Test-only 19/116: ownership wrappers (`HibernateVM`, `ResumeVM`, `CheckIdleVMs`, `MarkUnhealthyForDesktop` 19 ...). Last commits `2fbecebc` and the vmctl redesign docs `84c71907` (2026-10-10; vmctl is under a Phase 0 redesign, so check before touching).
- Others (all 1-3 funcs): `actor`, `actorruntime` (`WithOnActorFailure`, `Adapter.Drain/ActorRuntime`), `agentprofile.IsTexture`, `autoputer` (`RegisterTerminalRoutes`, `FilesHandler.RootDir`), `base/tree`, `cmdutil` (3, 38), `coagentowner.normalizeSlot`, `computerevent` (`NewDefaultHTTPClient`, `canonicalInt`), `content`, `desktop`, `health`, `maild`, `modelpolicy` (`RuntimeConfigFallbackSelection`, `RegisterVerifyModelCapabilityTool`), `proxy`, `researchtools` (2/27), `sourcecontract` (`NormalizeSelectors`, `IsRelationalEvidenceState`), `sources.MarshalRaw`, `toolregistry/toolloop.go` (`WithRequiredWriteToolResult` ...), `vmmanager` (`goBuildCommit`, `CreateSparseDataImage`), `vocabmigrate.MigrateInboxDelivery`, `updater.VerifyKernelCapabilityProbe` (36). Not scheduled; remove.

## 18. Unwired `cmd/*` binaries

- **`cmd/runtime-ratchet`** (1,761 prod + 935 test, 2026-07-11 `71377a9f`): "runtime dissolution inventory ratchet" comparing `docs/runtime-dissolution-inventory.yaml` (last touched 2026-10-06). No CI/script/nix reference (the CI uses `cmd/lockscope` for the analogous ratchet). Docs: 14 files (evidence/dispatch reports; 1 live). Owner question: delete or wire into CI.
- **`cmd/choir-rebuild-base`** (383, `6185910a` 2026-08-23, last `0f7c58ba` 2026-10-08): offline projection rebuilder CLI for recovery ("parent workers read --result-file"). No reference outside docs (6 docs incl. `docs/problems/vm-restart-no-rebind-8085-2026-09-25.md`, recovery policy evidence). It is an operator tool; recent edits (2026-10-08) show it is maintained. Keep; owner question whether to nix-package it.
- **`cmd/basecompare`, `cmd/baseharness`, `cmd/baseobserve`**: item 2.
- **`cmd/choir-checkpointd`**: empty untracked directory (not in git); the systemd unit is `go-choir-checkpointd` running `cmd/checkpointd`. Harmless; delete the empty dir.
- Used and fine: `auth proxy maild maildctl vmctl gateway corpusd checkpointd projection-compact choir-updater choir-receipt-signer capsule-broker autoputer choir-builder sourcecycled(dormant) zot` (flake), `doccheck lockscope platformrelease` (CI), `choir` (user CLI; README + skill; not nix-built), `desktop` (Wails module).

## 19. Test files behind the never-run `comprehensive` build tag

- 14 `_test.go` files (6,014 lines) carry `//go:build comprehensive`: 9 in `agentcore` (87-831 lines), `apihandler/costs_test.go`, `browsercontrol/handler_test.go` + `browser_live_test.go`, `content/content_test.go`, `desktopstate/handler_test.go`, `mediastate/handler_test.go`, `textureowner/texture_live_llm_workflow_test.go`. No workflow/script/Taskfile passes `-tags comprehensive`. `go vet -tags comprehensive ./internal/agentcore` fails: `content_prompt_test.go:24 handler.HandlePromptBar undefined`, `promptBarSubmitResponse` undefined; `desktopstate` also fails. So at least two packages' comprehensive tests no longer compile: they are dead test code that references deleted symbols. Neither deadcode nor staticcheck sees them. Proposal: delete the non-compiling ones; revive or delete the rest. (Some are also keepers if the owner runs them by hand: owner question.)

## 20. Smaller test-only clusters (production file, only tests call it)

Mostly legitimate test doubles or fixtures that live in non-test files; options are "keep", or move to `_test.go`:
- `routeledger.MemoryLedger` (6 funcs, 68): in-memory ledger used by 6 test packages; prod uses `sql_ledger.go`. Keep or move to `routeledgertest`.
- `decisionpolicy` (`Verify` 20, `Store.Revoke/Known`, embed byte getters, 46) and `trustedoutbox` (item 5): the self-dev decision-policy reducer; consumers only `agentcore` decision binding. Owner question.
- `vocabmigrate` (13 test-only funcs, 116: `MigrateAgentID`, `MigrateRunRecord`, `RevertRowV1`, ...): V1->V2 row migration core from `b2eccab3` (2026-09-10); production uses only the live sets/fences. Frozen-history per `3c963edf`; keep until owner confirms V1 data is gone.
- `types/commitment_views.go` (`BuildSupervisionPack`, `AccrualByAgent/Doc`, 54) and `types/texture_revision_hash.go` (`ComputeRevisionHash`, 32): R4 ledger read surface (`68a2e023` 2026-09-26) and D2 tamper-evident hash chain (`f592052e` 2026-06-18); tested, not called. Cut 1/2 may reuse as "Texture renders the ledger"; owner question.
- `auth/testhelpers.go` (`TestStore`, `TestConfig`, 48): non-test file for tests; `computerevent` helpers (`NewMemoryArtifactReader`, `DetectPrivateSecrets/RefusalSecrets`, `ResolveVocabulary`, 57+32+21), `base/journal` (item 2), `health` (`TCPChecker.Check`, `CheckerFunc`), `platformrelease` (`Load`, lineage methods 36), `sourcefetch` (`SetAllowPrivateNetworkForTests`, `ValidateHost`, 33), `toolregistry` option funcs (`WithInitialToolChoice`, `WithPassivatingToolSuccesses`, ... 49), `selfdev`, `selfdevprotocol`, `projectionbase.FindLatestCompatibleBase`, `storeschema.Read`, `workitem.SourceItemDecisionFingerprint`, `updater.New`/`MandatoryKernelCapabilities`.

## 21. Repo hygiene finds (not Go reachability but found while checking)

- `capsule-broker` at repo root: a **tracked 32 MB executable** (`git ls-files -s capsule-broker` = mode 100755; added `19e2c256` 2026-09-27). `.gitignore` covers `/choir`, `/maild`, `/choir-builder`, `/choir-updater`, `/sourcecycled`, ... but not `/capsule-broker`. Also a tracked `result` symlink -> a nix store path `prefetch-npm-deps` (dangling elsewhere). Remove from tracking and add to `.gitignore`.
- `specs/states/`: 43 GB of TLC model-checker state dumps, ignored by git but live on disk; a naive `cp -R`/rsync of the repo (which I hit) is inadvisable.
- Untracked-but-ignored binaries at root (`choir`, `maild`, `sourcecycled`, `choir-builder`, `choir-updater`) and logs/db (`auth.db`, `*.log`) are ignored; fine.

---

## Appendix: where the raw data is

All in `(census scratch, not retained) `:
- `dcL-notest.txt`, `dcL-test.txt` (linux deadcode, canonical), `dc-notest.txt`, `dc-test.txt` (darwin), `scL.txt`, `sc.txt` (staticcheck U1000), `rowsL.tsv` (linux rows: file:line, function line count, name, DEAD/TESTONLY), `reach*.txt` (package reachability).
- `copy/`: the stubbed source copy used for the linux pass (safe to delete).

### Appendix: stray DEAD helpers by file (name(lines))

```
internal/actor/actor.go: Runtime.Drain(18)
internal/actorruntime/adapter.go: WithOnActorFailure(7), Adapter.Drain(7), Adapter.ActorRuntime(3)
internal/agentcore/engineering_assignment_runtime.go: runtimePersistentManagementControlID(35), persistentManagementRunStateAllowedRuntime(3)
internal/agentcore/lifecycle_control_issue.go: LifecyclePacketIDForRecord(3), lifecyclePacketIDForRecord(3)
internal/agentcore/replay_completeness.go: sameReplayHead(6)
internal/agentcore/runtime.go: buildAssignedWorkItemPrompt(3), mergeStoredConductorRoute(26), fallbackPromptBarInitialContent(26), Runtime.latestActorToolLoopBudgetSpend(52), Runtime.channelHasGroundedHistory(36)
internal/agentcore/self_development_materializer.go: bundleDigestFromRelease(6)
internal/agentcore/tool_profiles.go: textureAgentIDMatchesDoc(8), docIDFromTextureAgentID(9)
internal/agentcore/tools_engineering_assignment.go: requirePersistentManagementExecution(8)
internal/agentcore/tools_research.go: ensureFindingEvidence(39), stringSlicesEqual(11), nonEmpty(7), rawJSONText(6)
internal/agentcore/trace_helpers.go: payloadBool(4)
internal/agentprofile/agentprofile.go: IsTexture(4)
internal/autoputer/files.go: FilesHandler.RootDir(3)
internal/autoputer/terminal.go: RegisterTerminalRoutes(5)
internal/base/tree/tree.go: Tree.LiveItems(9), Tree.Has(4)
internal/cmdutil/obsset.go: ParseEpoch(10), RequireDirIfSet(14), RequireFileIfSet(14)
internal/coagentowner/spawn_tool.go: normalizeSlot(8)
internal/computerevent/canonical.go: canonicalInt(3)
internal/computerevent/http_client.go: NewDefaultHTTPClient(3)
internal/content/extract.go: decodeHTMLEntities(3), safeBaseName(6)
internal/desktop/apikey.go: KeychainKeyStore.Load(10), KeychainKeyStore.Delete(6), DefaultFileKeyStorePath(7), NewDefaultAPIKeyStore(3), NewFallbackAPIKeyStore(7)
internal/desktop/client.go: BaseClient.PutBlob(24), BaseClient.PutItem(12), BaseClient.GetItem(8)
internal/gateway/client.go: GatewayClient.Name(1), GatewayClient.IsReal(1)
internal/gateway/handlers.go: drainBody(5)
internal/gateway/search.go: intFromEnv(11), truncateSearchAttemptError(7)
internal/health/health.go: TCPChecker.Name(1)
internal/maild/attachments.go: Handler.draftAttachmentMaxBytes(6)
internal/modelpolicy/model_policy.go: RuntimeConfigFallbackSelection(12)
internal/modelpolicy/tools_model_verify.go: RegisterVerifyModelCapabilityTool(3)
internal/pii/slm_redactor.go: WithSLMPrompt(3)
internal/platform/artifact_gc.go: ListGCNamespaces(6)
internal/platform/control_keys.go: Store.insertControlKey(25), activateControlKey(14), revokeControlKey(14), nullableUint64(6), nullableTime(6), nullableControlString(6)
internal/platform/service.go: execAll(8), summarizeSQL(7)
internal/provider/bridge.go: BridgeProvider.Inner/ProviderName(1 each), BridgeProvider.RuntimeProviderPolicy(38), GatewayBridgeProvider.ProviderName(1), GatewayBridgeProvider.RuntimeProviderPolicy(12)
internal/provider/provider.go: anthropicThinkingForReasoning(19), parseOpenAIResponse(45)
internal/proxy/handlers.go: Handler.autoputerWSURL(3)
internal/researchtools/researchtools.go: researchRunHasSuccessfulTool(17), boolValue(10)
internal/sourcecontract/evidence.go: IsRelationalEvidenceState(4); selector.go: NormalizeSelectors(10)
internal/sources/types.go: MarshalRaw(7)
internal/textureowner/handler.go: toolExecutionContextForRun(21), agentIDForRun(14), channelIDForRun(17), desktopIDForRun(11)
internal/textureowner/texture.go: countRawJSONItems(14)
internal/textureowner/texture_agent_revision.go: Handler.recentWorkerMessages(31), Handler.userRevisionDiffSummaries(24)
internal/textureowner/texture_import.go: extractTextureProjectionFromDOCX(42), extractTextureProjectionFromPDF(17)
internal/textureowner/texture_turn_runtime.go: textureTurnAuditDigest(5)
internal/textureowner/tools_texture.go: trimNonEmpty(9)
internal/toolregistry/toolloop.go: WithRequiredWriteToolResult(10), isInitialToolChoicePreconditionError(6), successfulTerminalToolNames(3)
internal/trace/query.go: FormatTimestamp(3); redact.go: RedactingStore.Pipeline/ListByRun/ListByOwner/ListByTrajectory/Close(11 total)
internal/updater/kernel_capabilities.go: VerifyKernelCapabilityProbe(36)
internal/vmctl/cold_recover.go: Handler.SetColdRecoveryStateRoot(6), Handler.RecoveryLease(6)
internal/vmctl/handlers.go: HibernateEndpoint, ResumeEndpoint, RecoverEndpoint, LogoutEndpoint, IdleCheckEndpoint, ReclaimEndpoint, RetentionPlanEndpoint, RetentionShadowPlanEndpoint, PruneEndpoint (3 each)
internal/vmctl/ownership.go: OwnershipRegistry.RemoveOwnership(3), SetAutoputerCredential(12); route_authority.go: RouteAuthority.Transition(3)
internal/vmmanager/boot_timeline.go: goBuildCommit(12); manager.go: CreateSparseDataImage(3), createSparseDataImage(3)
internal/vocabmigrate/migrate.go: MigrateInboxDelivery(5)
internal/yaegikernel/session_loop.go: RunSessionLoopFramed(3); sidecar.go: SidecarRunner.RunInProcess(8)
```

### Appendix: U1000 identifiers deadcode cannot see (non-test, non-func)

`cmd/capsule-broker/main.go`: types `stdoutWriter`, `stderrWriter`. `agentcore`: consts `internalRunSubmissionFingerprintMetadataKey`, `runMetadataEngineeringReplacementRequested`, `persistentManagementEngineeringCancelContinuationPrompt`, `canonicalTextureSourcePathMetadataKey`, `textureAvailableSourceEntitiesKey`, `textureMetadataPromptUnixTS`, 5 `wireProcessorDecision*`; vars `internalIngestionSubmissionLegacyFingerprintKeys`, `durableMetadataKeys`; types `qdrantDedupResult`, `acceptanceToolError`, `acceptanceToolInvocation`, `actorToolLoopBudgetSpend`, `registryToolInstaller`, `researchFindingEvidenceInput`, `wireProcessorDecisionVerdict`, `wireProcessorDecisionUpdate`; struct fields `Runtime.internalIngestionSubmissionMu`, `Runtime.textureEditMu`. `auth/handlers.go` const `rotationGraceDuration`. `base/planner` fields `remoteID`, `remoteVer`. `gateway/search.go` type `searchProviderBatch`. `health` field `refreshDone`. `maild/spool.go` field `mu`. `platform/service.go` type `statement`. `provider/provider.go` 4 OpenAI-response types. `server/server.go` field `once`. `store`: `actorWakeRunContent`, `rowScanner`, consts `selectTrajectoryByID`, `workItemColumns`. `textureowner`: `textureAgentRevisionResponse`, `submitCoagentUpdateArgs`. Plus 62 unused identifiers in `_test.go` files (35 in agentcore, 15 in provider, 7 toolregistry, 5 textureowner, 3 store, ...): see `scL.txt`.
