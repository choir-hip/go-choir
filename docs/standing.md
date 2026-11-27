# Standing triage of 281 documents

Standing: governing | kind: happened | settled-by: the triage of 2026-11-27, station T6 of docs/definitions/choir-docs-transform-2026-11-27.md; a file's own first line wins over this list

Produced 2026-11-27 by a builder lane (station T6 of
[the docs transform](definitions/choir-docs-transform-2026-11-27.md)). **This list is the triage
record, not the source of truth: where a file carries a `Standing:` line, that line wins over
this table.** The list was made from a header-only skim of `docs/design`, `docs/definitions` and
the top level (buckets A governing, B built, C undecided, D superseded, E could not tell), then
each A, D and C file was opened and its basis checked. Buckets B and E are not headed in this
station.

## The first line

One line after the title (after the frontmatter and title for goal files; a YAML comment,
`# Standing: ...`, in the three old-format goal files whose whole body is YAML):

    Standing: <governing|superseded|undecided|built> | kind: <exists|decided|wanted|happened> | [scope: in part |] settled-by: <ruling or outcome and where it is quoted, or nothing> | [replaced-by: <path or none> |] [note: <text>]

Fields are separated by ` | `; the first word after `Standing:` is the standing, so
`grep -rn '^Standing: governing' docs` lists governing documents with their basis. `settled-by`
is an owner ruling (dated, with the line where the words are quoted, or the document that carries
them) or an outcome the author cannot write (a CI gate that reads the file, an entry in
`docs/current-architecture.md`); `nothing` for undecided. `scope: in part` marks a document
superseded only where the named successor conflicts. No `built` was written: that needs an
outcome recorded in `docs/current-architecture.md`, which this station did not check per file.

## Finding a document

Run `scripts/docs <words>` (or `scripts/docs --help`). It reads the first lines of the files, so it is
as current as they are. Groups run governing, built, undecided, unmarked (no Standing line, matched
by path or title only), superseded (with its replacement); the newest date in the name comes first.

    $ scripts/docs vault
    undecided  docs/design/mac-device-key-release-2026-11-01.md  Releasing a computer's key to the owner's Mac (st…  | nothing; accepted by a review seat (Devin ro…
    undecided  docs/design/secrets-vault-design-2026-10-28.md  Secrets vault design (draft, 2026-10-28; revision…  | nothing; this file's status (line 5): draft fo…
    undecided  docs/provider-vault-and-model-policy-design-2026-10-22.md  Provider key vault, host model policy and agentic…  | nothing; this file's status (line 5…

    $ scripts/docs restore bases
    unmarked   docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md  Restore bases design: three review rounds without…  (no S
    superseded docs/design/restore-bases-store-once-2026-11-15.md  Restore bases that cost the change (2026-11-15)  -> replaced-by: none

    $ scripts/docs --show docs/design/one-tape-durability-2026-10-31.md
    path:       docs/design/one-tape-durability-2026-10-31.md
    title:      One tape: telemetry, recovery, backup and sync on one mechanism (2026-10-31)
    standing:   governing
    kind:       decided
    settled-by: owner 2026-10-31 (line 8): 'one append-only tape'
    note:       partly built (tape group commit)

A new file in `docs/design` or `docs/definitions` needs a Standing line (`scripts/check-doc-standing`).

## Counts

Headed 217: governing 52, built 99 (65 of them in part), superseded 38, undecided 28; 126 of them were headed
by station T10. Unverified, no header:
A 3, D 8, C 10. Not assessed (E 47).

## Built (station T10, 2026-11-27)

Headed by a builder lane (first half of station T10 of [the docs transform](definitions/choir-docs-transform-2026-11-27.md)): the 126 documents the triage sorted as built were each read and checked for evidence on main. The triage was often wrong: most mission files still read `working` or `checkpoint_incomplete`. Where a file carries a `Standing:` line, that line wins over this list. Paths are under `docs/`.

### Built (34)

Documents whose work was carried out.

- `definitions/choir-launch-fp0-proof-scripts-2026-10-21.md`: scripts/proofs library and fp1 to fp8 scripts on origin/main (50d5c3773); own status checkpoint_complete at line 86
- `definitions/choir-launch-fp10-res-14-2026-10-22.md`: a0ede4268 (checklist: RES-14 done, snapshot budget counts true compressed bytes, probe 20ea9b6b6 on main); checklist line 491 is ticked; the file's own now.status blocked (line 82) is stale
- `definitions/choir-launch-fp10-res17-reaper-2026-10-21.md`: 24627242f and 57ad29ef0 (reaper, dedicated desk-worker binary) on origin/main, a00232665 ledger deploy F: RES-17 workers 87 to 24 MiB RSS; deskReaperTimer in internal/agentcore/tools_desk.go:150; file line 81 leaves only FP0 proof steps
- `definitions/choir-launch-fp10-sec14-deploy-user-2026-10-21.md`: b549daeae (CI deploys as choir-deploy through the gate), f77799459 (root no longer accepts the CI key), 674460d15 (SEC-14 step 3 proved, rollout complete); the file's own now.status checkpoint_incomplete (line 80) is stale
- `definitions/choir-ordinary-prompt-path-2026-11-09.md`: now.status complete at line 112 and 043db0977 (sandbox pause drill passed on d@e)
- `definitions/choir-ordinary-prompt-path-2026-11-09.ledger.md`: the ledger of docs/definitions/choir-ordinary-prompt-path-2026-11-09.md (now.status complete at line 112; 043db0977)
- `definitions/choir-progressive-delivery-2026-11-03.md`: now.status complete at line 118, owner at line 119: Yes, close pd. It works.; cb1a6cba9 and 869255bb4; leftovers carried to the modular architecture goal
- `definitions/choir-progressive-delivery-2026-11-03.ledger.md`: ledger last line 2026-11-04 settled complete on the owner's word; cb1a6cba9; 869255bb4
- `definitions/choir-store-header-body-split-2026-11-02.ledger.md`: the ledger of docs/definitions/choir-store-header-body-split-2026-11-02.md (now.status complete, line 78; S6 judged complete in the ledger's last entry; landed 1d33cafce)
- `definitions/choir-store-header-body-split-2026-11-02.md`: own now.status complete (line 78, 2026-11-03); landed 1d33cafce, dcbe7b4c4, 17afb82f2 on origin/main; cmd/store-split-migrate exists
- `design/app-motion-2026-11-18.md`: mobile/AutoradioLab/transitions.contract and mobile/scripts/check-no-loading.py exist on origin/main (joined in 33cf20826); NoWaitingWordsTests present; no status line in the doc itself
- `design/app-packages-replan-2026-10-24.md`: own status line 3 (2026-10-30: app packages have landed); internal/apppackage exists; 886bf9510 (MA5.M4 packages load as modules) on origin/main
- `design/autoradio-app-api-2026-11-10.md`: own status line 3 (a map of what exists at origin/main 7f1b2487f, step 0.2); added in 72aed7b5f
- `design/autoradio-l1-failure-list-2026-11-15.md`: station L1 programme route 45bf7ce4c; its F-tests exist in internal/agentcore/api_autoradio_programme_test.go; docs/current-architecture.md:110 (programme route proven)
- `design/autoradio-system1-route-2026-11-10.md`: POST /api/system1/judgments in internal/agentcore/api_system1.go and internal/apihandler/routes.go, added in 79bd88f8b; docs/current-architecture.md:169 row
- `design/bundle-authenticity-2026-10-31.md`: 013c7de93 on origin/main (squash of lane/bundle-authenticity); line 3 'not on main' is stale
- `design/choir-data-system-frame-2026-10-30.md`: 871ab6531 data system cutover (guest SQLite, host PostgreSQL, Dolt deleted) and 1db2d897a; docs/current-architecture.md:28 says Dolt and Store B deleted 2026-10-30
- `design/data-system-build-sheet-2026-10-30.md`: 871ab6531 data system cutover (Dolt deleted, guest SQLite, host PostgreSQL) and 1db2d897a; docs/current-architecture.md:28
- `design/density-d6-page-cache-return-2026-10-26.md`: D6 S1 b068de456, S3 f9af6e7d3, S5 fixes a7f2a7e33, active on d@e cba66873b
- `design/derived-key-marks-2026-11-01.md`: 6b30bcd67 and hardening 3e0f2997a on origin/main; own status line 3 says built
- `design/desk-shots-and-review-2026-11-07.md`: 9ff269597 (choir.ShotStart, ShotWait, ShotVerdict replace the platform shot worker; the worker file is gone) and internal/agentcore/desk_shots.go
- `design/email-sign-in-2026-10-31.md`: email sign-in d439ba098 (security epoch 25, 849966c1e) and deployed e2e 0cff2f27a; ledger 3ae113888 says live
- `design/enforce-flip-2026-11-08.md`: 1c9fcf758 (writer, reader and disclosure gates enforce for every kind; observe leaves product code), 13618eec5, 922a1a58d
- `design/engineer-reuse-2026-11-04.md`: 07a98d046 (engineer reuse wired to per-shot review and verify-owed), e2634dd4f, afc343285; docs/ledger records it proven live
- `design/ma65-yaegi-go-modules-2026-11-06.md`: 5c83a08ff (MA6.5 gomodule) and d@e receipts 4e63947a6 (MA6.4 and MA6.5 accepted)
- `design/ma66-code-mode-import-sets-2026-11-06.md`: 916114811 (MA6.6 code mode, per-desk import sets) and receipt 9ce5a09e7 (MA6.6 accepted on d@e)
- `design/object-verbs-quotas-ma38-ma39-2026-11-05.md`: own status line 3 'built 2026-11-05'; 15ac89707 (MA3.8) and bac28589a (MA3.9)
- `design/research-more-goes-to-research-2026-10-21.md`: line 3 status built; 73b84cb78 feat(research) More research goes straight to the research desk; 79ff71499
- `design/ring-ordered-deploys-2026-11-03.md`: 869255bb4 ring-ordered deploys and automatic guest-release rollback; internal/vmctl/ring_walk.go on origin/main
- `design/run-record-written-less-2026-11-15.md`: 8d81b7fe5 fix(texture) the source-entities list is no longer stored in the run record; the join ids are (first slice T2)
- `design/s12-runtime-install-build-2026-11-04.md`: line 3 record of what landed; 889b73619 packages runtime install (S12); 03037239a; 9f3970680
- `design/source-only-freeze-and-candidate-shots-2026-11-06.md`: e65affa11 source-only freeze, shot plan, candidate_shot interlock; 3706f7cc7 security epoch 93
- `design/store-header-body-split-2026-11-01.md`: f3e5f7d2b security epoch 33 for the store header/body split; ce210a56b store split S6 goal settled complete; 8a85a2ca7 closing report
- `precommitment-records-practice-2026-11-08.md`: line 3 status says it describes the engineering on main; internal/store/commitment.go exists on origin/main

### Built in part (65)

Documents whose work was carried out in part; the note names what was not.

- `definitions/choir-autopaper-overnight-2026-11-09.ledger.md`: the ledger of choir-autopaper-overnight-2026-11-09.md; landings 0a03add78 and b473db3af on origin/main, and its own passes to 2026-11-10 12:20Z (not built: goal file now.status is still working (line 117); no terminal receipt)
- `definitions/choir-autopaper-overnight-2026-11-09.md`: ledger passes show landings 0a03add78 (auth rate classes) and b473db3af (poll arm) on origin/main; the file's own now.status is working (line 117) (not built: no terminal receipt; image stations deferred by the owner)
- `definitions/choir-autoradio-exact-cutover-2026-11-18.md`: Exact cutover train merged to main (83d5fe9e1, c5de9cfe6), shared-code moves 877b8b17d; the phone app is Exact per CLAUDE.md Product (not built: file now.status is working (line 344), X3 to X12 still pending in the file; X0 device tests open)
- `definitions/choir-autoradio-full-experience-overnight-2026-11-15.ledger.md`: the ledger of choir-autoradio-full-experience-overnight-2026-11-15.md: K landed a99e07d3b, S landed 53747dcca, both on origin/main, and C107 and C109 true (not built: goal file now.status is working (line 170); ACTIVE.md:215 heads it Superseded)
- `definitions/choir-autoradio-full-experience-overnight-2026-11-15.md`: K landed a99e07d3b and S landed 53747dcca (podcast clips) on origin/main; ledger 08:40Z final install (not built: now.status is working (line 170); ACTIVE.md:215 heads it Superseded by the 2026-11-17 goal; automatic matching and the durable worker were not built)
- `definitions/choir-autoradio-learning-engine-2026-11-14.ledger.md`: the ledger of choir-autoradio-learning-engine-2026-11-14.md; heard-memory row in docs/current-architecture.md line 121 (proven on d@e 2026-11-18, 4fc286f69, e2225bf67 on origin/main) (not built: goal file now.status is working at slice L0 (line 277); the ledger is review rounds only)
- `definitions/choir-autoradio-pass1-2026-11-10.md`: round 3 landings 6f0c773ed, 26f9d66f3 and 215e9ac9a on origin/main, deployed 2026-11-11 (line 146) (not built: now.status is blocked_incomplete (line 143) on the owner install go and the TestFlight verdict)
- `definitions/choir-data-system-build-2026-10-30.ledger.md`: the ledger of choir-data-system-build-2026-10-30.md: now.status superseded at line 400, phase 1 landed (871ab6531, 8f867aaeb on origin/main); system-map.md line 30 (not built: phase 2 moved to docs/definitions/choir-lean-system-2026-10-30.md)
- `definitions/choir-data-system-build-2026-10-30.md`: goal now.status superseded at line 400 with phase 1 landed (PostgreSQL on the host, SQLite in computers; 871ab6531, 8f867aaeb, 226cab848 on origin/main); system-map.md line 30 (not built: phase 2 moved to docs/definitions/choir-lean-system-2026-10-30.md as L4 and L5)
- `definitions/choir-launch-fp0-density-bench-2026-10-21.md`: scripts/density written (a70fa7cec on origin/main, with accept_five.py 3cb626bb6) and the file's status ready_for_fp0 at line 84 (not built: its own card says nothing had run on Node B; no receipt of the measurements found)
- `definitions/choir-launch-fp0-vm-retention-2026-10-22.md`: scripts/proofs/fp0-vm-retention.sh on origin/main (deb31074f); the retention shadow plan was replaced by the active plan (70563c0d1) (not built: own status waiting_on_fp0 at line 251; enforcement waited on the shadow report and owner questions)
- `definitions/choir-launch-fp10-arch-08-2026-10-21.md`: eb16f144a and d99b5ce08 (research children return token spending; slice 1), 3c46d5196 now card; file now.status checkpoint_incomplete line 89 (not built: later slices of budgets passed down agent trees not found; ARCH-08 unticked in the checklist)
- `definitions/choir-launch-fp10-arch-09-2026-10-21.md`: 93931bbe2 (S0 internal/computecapsule on origin/main), 9f098a5d8 now card; file now.status checkpoint_incomplete line 60 (not built: slices S1 to S4 (wiring the capsule kind to app desks) were left as residuals; nothing imports the package)
- `definitions/choir-launch-fp10-arch-11-12-2026-10-21.md`: design committed 36aa9dae1 and station file 44591f56f; file now.status checkpoint_incomplete line 99 (not built: design only by its own header; no executor VMs or cross-host executors built, ARCH-11 and ARCH-12 unticked in the checklist)
- `definitions/choir-launch-fp10-net-03-04-2026-10-21.md`: design committed 6096e3e72 and station 0ad393bec; file now.status checkpoint_incomplete line 97 (not built: design only; no vsock control plane and no guest NIC removal built, NET-03 and NET-04 unticked in the checklist)
- `definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md`: 0967e296f (host fetch verb behind the gateway) on origin/main, 45f5d6ea9 says on main, off; file now.status checkpoint_incomplete line 85 (not built: landed off by default; closing general guest egress (NET-04) was not this station and is not built)
- `definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md`: f2a5dd85a (npm, PyPI, crates mirror), 252ff704d (git mirror), e1497db1b (per-computer capsule switch), all on origin/main; file now.status checkpoint_incomplete line 104 (not built: every ecosystem off by default; deployed proof not found)
- `definitions/choir-launch-fp10-p2-small-2026-10-21.md`: 117871ae6 (ARCH-15 launcher rename) on origin/main; RES-15 was a comment; ARCH-14 landed as 6e33c3d8f (not built: ARCH-14 internal/appverbs was deleted by 510cd7ee0 as unused, so that item is not standing)
- `definitions/choir-launch-fp10-res-08-2026-10-21.md`: design and DEC-05 threat analysis 134113c61 and the read-only page-cache probe landed (rounds through 6727d7bd2); file now.status is a checkpoint line 102 (not built: the pmem and DAX shared base OS itself was not built; RES-08 unticked in the checklist)
- `definitions/choir-launch-fp10-res-10-2026-10-21.md`: a60a5b731 (guest procmem reports balloon features, DAMON reclaim and page reporting, FP8 P4 / RES-10) on origin/main; file readiness built_steps_1_2 line 7 (not built: emergency inflation (later steps) not found)
- `definitions/choir-launch-fp10-res-13-22-2026-10-21.md`: 591f75306 (L3a Diff snapshot on hibernate) and 933870ff2 (D5 step 2 Diff snapshots for test computers) on origin/main; file now.status blocked line 93 (not built: RES-13 L4 and L20 and RES-22 slab and btrfs metadata not found built; checklist unticked)
- `definitions/choir-launch-fp10-res-20-2026-10-21.md`: design report a7a6d8812 and station b6cf4f86e on origin/main; file now.status checkpoint_incomplete line 91 (not built: design only; no Cloud Hypervisor backend seam built (checklist RES-20 unticked))
- `definitions/choir-launch-fp2-desk-boundary-2026-10-21.md`: fcfc8c8ae (desk manifest, per-frame role check, slot-bound frame) and 139d1b23d (checklist ticks SEC-01, SEC-02, ARCH-02, ARCH-07 with deployed evidence); file now.status checkpoint_incomplete line 185 (not built: ARCH-05, SEC-13, SEC-15 and RES items not all settled; some candidates not landed (lines 359, 374))
- `definitions/choir-launch-fp3-host-floor-2026-10-21.md`: f0b382feb (SEC-03, SEC-09 done by FP3 cut 1, deploy M, SEC-10 host side) and 4bc3aa3ec (cut 1 deployed proof) on origin/main; file now.status working line 110 (not built: groups 2 and 3 (batches B4 and B5) were not all landed under this file)
- `definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md`: a233b7506 (ARCH-10 admission queues, FIFO), 6f0a35a40 (ARCH-04 one writer per object kind), 50e3bc862 (ARCH-03 deskworker app desk) on origin/main; file now.status checkpoint line 208 (not built: ARCH-03 install not wired (checklist unticked))
- `definitions/choir-launch-fp6-network-2026-10-21.md`: 074a86b9a (Obscura removed from the guest image, replay baa388d02), SEC-06 tunnel binding ba643e435 and enforce selector e03fb969a on origin/main; file now.status checkpoint_incomplete line 146 (not built: host-mediated network not complete; enforce rollout and proofs pending)
- `definitions/choir-launch-fp7-gateway-identity-2026-10-21.md`: 55420cab4 (the Claude delegate token relay, FP7 finished) and 3d4037f6b (gateway body caps enforce); file now.status working at line 151 (not built: the file's other batches and its re-freeze were never marked closed)
- `definitions/choir-lean-system-2026-10-30.md`: f0e9b8fa6 and now.status checkpoint_incomplete at choir-lean-system-2026-10-30.md:226 (build work deployed; the multi-day watch never settled) (not built: ea124dd70 paused the lean system for guest isolation; the watch settlement is not on record)
- `definitions/choir-lean-system-2026-10-30.ledger.md`: f0e9b8fa6 and now.status checkpoint_incomplete at choir-lean-system-2026-10-30.md:226 (build work deployed; the multi-day watch never settled) (not built: ea124dd70 paused the lean system for guest isolation; the watch settlement is not on record (the ledger of the mission file))
- `definitions/choir-lean-system-2026-10-30.lanes.md`: f0e9b8fa6 and now.status checkpoint_incomplete at choir-lean-system-2026-10-30.md:226 (build work deployed; the multi-day watch never settled) (not built: ea124dd70 paused the lean system for guest isolation; the watch settlement is not on record (the lane register of the mission file))
- `definitions/choir-lp-s6-commit-gate-2026-10-20.md`: S6-0 drills 77d8a5863 and 16b97d28c and fix 22b287a5d; file now.status working at line 236 (not built: the card stops at S6-0, later stations not shown closed)
- `definitions/choir-modular-architecture-2026-11-04.md`: 546a37cf6 (phases 1 to 3 closed) and 25ecce33c (MA3.7c) on origin/main; file now.status working at line 236 (not built: F5 and later left open at the 2026-11-08 rewrite)
- `definitions/choir-modular-architecture-2026-11-04.ledger.md`: the ledger of docs/definitions/choir-modular-architecture-2026-11-04.md (546a37cf6; now.status working at line 236) (not built: F5 and later left open)
- `definitions/choir-modular-architecture-2026-11-04.precommit.md`: the precommit of docs/definitions/choir-modular-architecture-2026-11-04.md (546a37cf6; now.status working at line 236) (not built: F5 and later left open)
- `definitions/choir-storage-solutions-2-2026-11-12.md`: bfe378e98 (tape write counters) and 68ee06726; file now.status working at line 268 (not built: S4 snapshots, S5 coalescing and S7 sweep were still in flight at its last card)
- `definitions/choir-storage-solutions-2-2026-11-12.ledger.md`: the ledger of docs/definitions/choir-storage-solutions-2-2026-11-12.md (bfe378e98; now.status working at line 268) (not built: S4, S5 and S7 in flight at last card)
- `definitions/choir-storage-solutions-3-store-once-2026-11-14.md`: a512e78d5, c1ea7a418 (T7 prune) and d46fb88b7 (run-memory retirement); file now.status working at line 248 (not built: T1 and T2 were design lanes at the last card)
- `definitions/choir-storage-solutions-3-store-once-2026-11-14.ledger.md`: the ledger of docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md (a512e78d5, 824a01aab; now.status working at line 248) (not built: T1 and T2 were design lanes at the last card)
- `definitions/choir-v-boot-vocab-pass-removal-2026-10-27.md`: landed 8f8b2a26c (receipts line 117; frozen_ref line 14; paper boot vocabulary phase 0 s) and the b@c 8 GiB start observed 2026-10-28 (not built: now.status is checkpoint_incomplete (line 94), never settled complete; residuals at lines 108-113)
- `definitions/ci-test-cooptimization-2026-10-24.md`: C2 cache prune 18cc777d9 and prove-once wiring ba6625542 (inert, CI_REUSE_GREEN_PR_RUN false); CI wall-clock fix 17a7b353f, 1ed31abef (line 41) (not built: now.status waiting_for_FP0 (line 121); C3 to C6 not shown landed)
- `definitions/corpus-first-search-one-verb-2026-11-19.md`: landing L1 (search leads) b1257d04a, docs/current-architecture.md row 94, ledger C172 settled true (not built: header line 5 says not built; L2 (Sufficiency, Leads), L3 (the one verb, guest gate deleted), L4 not shown landed)
- `design/agent-tree-budgets-2026-10-21.md`: slice 1 (observe-only token totals, one research tree line) eb16f144a and d99b5ce08 on origin/main; header line 5 says one observe-only slice built (not built: slices 2 to 5 (egress, child bound, worker bound, management trees) not built)
- `design/app-design-bar-2026-10-26.md`: the bar landed a8423e965 and 84203ceb5 (review fixes); the Reader redesigned to it 5f536fdee (not built: header (line 3) says green doc only; Slides and signed-out /read change lists not verified built)
- `design/append-clustered-store-2026-10-31.md`: own status line 3: part 1 (WAL bound) and S1 (UUIDv7 run-memory ids) built, S1 is 0d99548b6 (not built: S2 to S5 (og_id layout) designed only; the header/body split mission took a different route)
- `design/append-receipt-trim-2026-10-31.md`: phase 1 lossless compact receipt body with paced backfill, d68fb3d87 on origin/main (not built: phase 2 (deleting rows) is marked not built (line 162); it was not part of the store split mission)
- `design/autoradio-app-ui-2026-11-10.md`: Autoradio 1.0 build 473021 WAITING_FOR_REVIEW (docs/current-architecture.md:27); pages run as shared Exact (rows 113, 143); plan added in 3b23b6714 (not built: header line 3 says design not built; the whole-app plan was built as Exact not as written, per-ruling status unchecked)
- `design/autoradio-lab-2026-11-10.md`: mobile/AutoradioLab exists (29e01354f, the Autoradio iOS app on Exact); docs/current-architecture.md:115 names the lab app variants (not built: header line 3 says not built; the lab's runtime-configuration and replay stages not verified)
- `design/autoradio-page-2026-11-17.md`: a RadioPage page kind is wired in mobile/AutoradioLab/pages.contract lines 22 and 35; run sheet and radio row docs/current-architecture.md:113 (not built: header line 3 says not built; the Built on rows and tile removal not individually verified)
- `design/autoradio-queue-desk-2026-11-15.md`: own status line 3 and section 4 line 191 (first slice built); internal/autoradioqueue; c00312841; docs/current-architecture.md:116 row built (not built: section 6 line 211: the cell module and queue ops on the programme route not built)
- `design/autoradio-turns-2026-11-10.md`: the turns route /api/autoradio/turns (docs/current-architecture.md:22) and the programme route 45bf7ce4c; dfe1bbbff added the note (not built: header line 3 says not built; prepared pivots and a desk with cells not built (current-architecture.md:20))
- `design/bounding-the-guest-store-2026-11-14.md`: store readings 0f4421aae (docs/changes.md:596) and run-memory retirement fixes 789db9dc4 (not built: only the reading and R-1 retirement are shown landed; R-2 to R-7 not verified)
- `design/coding-agent-desk-station-2026-11-10.md`: 65dee1879 and 3238c5e35 (internal/deskmcp, internal/deskjudge, cmd/choir-desk-mcp) (not built: line 7 says built additive and unwired; no runtime path calls it and shape (a) is not built (line 103))
- `design/object-graph-redesign-2026-10-30.md`: phase 0 guest write removal landed 2026-10-30 (line 575) and the cutover 871ab6531 (not built: header line 3 says proposal; later phases 1 to 4 not verified)
- `design/object-storage-2026-10-30.md`: cutover 871ab6531 with Dolt removed 1db2d897a (frame says storage design realized) (not built: header line 3 says design revision 13; holder holds and later items not verified)
- `design/package-operation-decision-2026-11-06.md`: P3 fe7b1f70f and epoch 5d49b1504 (package operation publishes by decision, package revert uninstalls) (not built: line 3 says design for build; P5 host build seat is later and not verified)
- `design/paper-compaction-2026-10-31.md`: fe6477cd4, 5a4fa56ad; docs/changes.md line 607 (not built: the layout trigger and layout marker were deleted by 06c49c1b8 (2026-11-22, header lines 3-6); only the free-pages trigger remains)
- `design/podcast-clips-2026-11-15.md`: 24cd67412; docs/current-architecture.md line 88 (row status partial) (not built: only tonight's slice (worker match mode) landed; the row says partial and the document's own status line 8 says rapid prototype)
- `design/sections-as-questions-2026-11-10.md`: 3c1cf9c9c, 6030e6240; docs/current-architecture.md line 101 (not built: not built (line 139): the reader-side list scored on the reader's computer, a reader-editable list, digest_render section list)
- `design/shot-verdict-reuse-2026-11-09.md`: line 3 the tonight cut is built; 6c8c42616 feat(selfdev) a clean rebase with unchanged shot inputs reuses its screenshot verdict (not built: only the tonight cut (section 12); the wider slices S1 to S5 are not shown built)
- `design/tape-group-commit-2026-10-31.md`: abfab8754 tape group commit (not built: the setting tape.group_commit was deleted by owner ruling; 10018c430 replaced it with lossless coalescing and no setting)
- `design/visual-evidence-slice-2026-10-15.md`: d43f5f7bd self-dev visual preview; 0f6928b13 (not built: 0f6928b13 deleted choir-capture, internal/visualcapture, the capture grant list and the visual-evidence route, replaced by the engineer's own screenshot files)
- `autopaper-sw-design-2026-10-09.md`: line 9 Autopaper live since 2026-10-13; docs/current-architecture.md line 76 and 81 (not built: line 3 rough draft v3; its storage (Store B, Dolt) was replaced by the object graph and SQLite state.db)
- `model-pools-design-2026-10-12.md`: b0d69d4d4 M0 model pools; 8e515d24b pools autoroute on failures (not built: 269c1e11a made every desk thread run gpt-6-luna with deepseek only as failover, unlike the two-model mix designed)
- `restart-subsystem-design-2026-10-13.md`: 22fec6022 planned restart one resume; 23c461b58 delete restart-resumption sweeps; CLAUDE.md Restarts rules (not built: the v3 design's numbered parts were not each matched to code)
- `vmctl-360-review-2026-10-10.md`: line 224 Phase 0 as landed; 4eb1ec554, 19ba30346, 2f755bb34, d41c2caf9 (not built: phases 1 to 4 (systemd units, executor and lease, lease-gated idle stop, file splits) not shown built)

### Superseded (17)

Documents the triage called built that are headed superseded.

- `definitions/choir-autopaper-improvement-2026-11-09.ledger.md`: the ledger of choir-autopaper-improvement-2026-11-09.md, now.status superseded at line 286 (only D13 landed, 36534be53; S2 to S5 deferred)
- `definitions/choir-autopaper-improvement-2026-11-09.md`: now.status superseded at line 286 of the goal file (merged into the overnight goal; only D13 landed, S2 to S5 deferred)
- `definitions/choir-autoradio-overnight-2026-11-17.md`: docs/ACTIVE.md line 206 heads it Superseded (2026-11-17 night); its own now.status is working (line 219)
- `definitions/choir-consolidate-and-scale-metamission-2026-10-26.ledger.md`: the ledger of choir-consolidate-and-scale-metamission-2026-10-26.md, now.status superseded at line 246; docs/ACTIVE.md:439 heads it Superseded
- `definitions/choir-data-architecture-selection-2026-10-28.md`: docs/ACTIVE.md line 427 heads it Superseded (2026-10-30); the data system build goal carries its decision (PostgreSQL on the host, SQLite in computers)
- `definitions/choir-data-architecture-selection-2026-10-28.precommit.md`: the ledger of choir-data-architecture-selection-2026-10-28.md, headed Superseded at docs/ACTIVE.md line 427
- `definitions/choir-guest-isolation-2026-11-01.ledger.md`: this ledger line 49 (owner made it a throughline, superseding; I1 to I6 landed, I0 closing report not written) and docs/ACTIVE.md line 406
- `definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md`: the ledger of choir-launch-first-pass-metamission-2026-10-21.md, now.status superseded at line 284 of the mission file (owner order 2026-10-24); docs/ACTIVE.md:472
- `definitions/choir-launch-fp1-a1-boot-alter-2026-10-21.md`: docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md line 3 (Dolt EnsureRecordIndexes is gone; the problem stays open as a boot-migration concern of PostgreSQL and SQLite); own status done (lane) at line 113
- `definitions/choir-launch-fp10-net-05-2026-10-21.md`: a18568ee4 deleted internal/egressgrant (landed unwired as b52a4c026) as never wired; the package no longer exists on origin/main
- `definitions/choir-launch-fp10-net10-2026-10-22.md`: a18568ee4 deleted internal/appnet and internal/egressgrant, never wired (appnet landed unwired as 54ded1675); file now.status checkpoint_incomplete line 97
- `definitions/choir-memory-optimization-2026-11-01.ledger.md`: docs/definitions/choir-memory-optimization-2026-11-01.md:71-72 (now.status superseded by the guest isolation goal, owner: I'll drop the current goal)
- `definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.ledger.md`: ledger line 79 and the mission file now.status superseded at line 117
- `definitions/choir-selfdev-apps-side-by-side-2026-11-02.ledger.md`: mission file now.status superseded at docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md:155
- `definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md`: ledger entry 2026-10-26 12:1x (settled checkpoint_incomplete, ordering superseded) and mission file lines 462-464
- `design/autoradio-voice-modes-2026-11-10.md`: its own line 3 says design, not built; the owner confirmed hands-free only and removed tap to talk (docs/problems/autoradio-owner-phone-review-2026-11-15.md:80,83; fd9448be4)
- `design/egress-grant-object-2026-10-21.md`: its own line 3-4: built unwired in internal/egressgrant, never wired, code removed November 2026

### Undecided (10)

No evidence found either way.

- `definitions/choir-launch-fp10-arch-06-2026-10-21.md`: nothing found; only the station file and problem record landed (1600e21ea), now.status is blocked_incomplete (line 112), design only, no pooled-slot code or checklist tick for ARCH-06 on origin/main
- `definitions/choir-mac-sync-stage2-2026-11-01.md`: nothing found; now.status reviewed at line 150; only design commits 345075752, 17ad01f18, aef02845a found by grep for key release, no code
- `definitions/choir-selfdev-apps-feature-complete-2026-11-03.ledger.md`: nothing found; mission file now.status working at line 234 (resumed under the superseded overnight metamission, selfdev station paused); searched ledger, mission file and git log
- `design/apps-feature-complete-spec-2026-11-03.md`: nothing found; its goal file choir-selfdev-apps-feature-complete-2026-11-03.md has now.status working (line 234), no terminal receipt; only scattered Files and Slides commits (89c780936, 3b2aa7eef)
- `design/autoradio-desk-fast-turns-2026-11-11.md`: nothing found; header line 3 says nothing built; docs/current-architecture.md:20 says there is no Autoradio desk that runs cells and the turns route is one streamed model call
- `design/ci-node-b-build-2026-10-24.md`: nothing found; header line 3 only says the Dolt compile cost no longer applies; searched git log for prebuild, incremental and closure commits and found none that name these proposals
- `design/intake-desk-pace-2026-11-10.md`: nothing found; line 3 says design, not built; db1bdc647 changed intake (oldest first, own lane) the opposite of the sampling asked, and no commit makes arrivals stop minting tasks
- `design/small-object-storage-2026-10-31.md`: nothing found; line 3 says nothing is built by this note; searched commits for small object, store split, compaction and changes.md
- `design/tetramark-motion-2026-11-18.md`: nothing found on main; line 3 says built on lane tetramark-motion, not seen on a device; origin/lane/tetramark-motion (a445ce8cf) is not an ancestor of origin/main
- `semantic-registry.md`: nothing found; line 3 says derived navigation for the doctrine, a standing reference that is not a work with a completion to prove

## Overlap groups

**Vaults.** None governs; all three are undecided. `design/secrets-vault-design-2026-10-28.md`
(draft rev 2, review round 2 pending; a line in its review notes says "design accepted; execution
held by the owner" but no owner ruling is quoted), `provider-vault-and-model-policy-design-2026-10-22.md`
(proposal, not owner-ratified) and `design/mac-device-key-release-2026-11-01.md` (accepted by a
review seat, Devin round 3, not by the owner; it releases a computer's key to the owner's Mac). The
owner said on 2026-11-27 "we need two vaults" (`docs/inbox.md` line 169; `docs/big-picture.md`
item 4): one for the platform's secrets, one for people's credentials. No design is written for the
second; which design, if any, serves the first is the owner's to say.

**Storage.** The mission `definitions/choir-storage-efficiency-2026-11-22.md` governs the work
(owner 2026-11-22, live in ACTIVE). Among designs: `design/one-tape-durability-2026-10-31.md`
governs the tape (owner 2026-10-31); `design/snapshot-one-pass-2026-11-22.md` replaces
`snapshot-without-scratch-copy-2026-11-20.md` (its own line 3; that file is headed superseded);
`design/density-d2-snapshots-survive-deploys-2026-10-26.md` replaces the L20 section of
`fp10-snapshot-levers-and-host-slab-2026-10-22.md` (headed in part); `restore-bases-store-once-2026-11-15.md`
was set aside after three review rounds (headed superseded, replaced by none). The remaining
storage designs are bucket B or E and were not headed, and `store-once-2026-11-14.md` is
unverified. No one storage design governs; the mission names only `paper-compaction` and
`capture-static-copy` among designs.

**The four SwiftUI-era app designs of 2026-11-10** (`autoradio-app-ui`, `autoradio-lab`,
`autoradio-turns`, `autoradio-voice-modes`; all bucket B, not headed). The rule that the phone app
is Exact and only voice is native (CLAUDE.md Product; owner 2026-11-18 and 2026-11-20) governs
where they put logic in Swift. The lab note itself carries an owner ruling of 2026-11-10 ("no
swiftui please. lets do all exact", its line 29). Whether each is superseded, wholly or in its
Swift parts, is not decided by any ruling found.

**Autopaper and engine.** `design/desks-and-engine-2026-11-24.md` governs (owner 2026-11-24; system
map section 10). `ideal-form-draft-2026-11-23.md` and `semantic-layer-2026-11-20.md` are headed
superseded in part by it; `first-design-2026-11-22.md` is governing except where the newer wins;
`first-design-path-2026-11-22.md` is superseded in part. `entity-layer-2026-11-20.md` and
`corpus-first-retrieval-and-research-2026-11-18.md` are headed undecided (their own status: not
ratified).

## For the owner

1. Which vault design, if any, is the platform-secrets vault; whether the people's-credentials
   vault gets a new design.
2. Does `design/store-once-2026-11-14.md` stand? ACTIVE lists it as a live station of 2026-11-14;
   its sibling restore-bases design was set aside; nothing says this one was.
3. Are the four SwiftUI-era app designs superseded by the Exact rule, wholly or only in their Swift
   parts?
4. Is `entity-layer-2026-11-20.md` superseded by the one-kind "subject" ruling (it reads so; not
   verified)? Same question for the entity half of `corpus-first-retrieval-and-research-2026-11-18.md`.
5. Is the apps feature-complete goal (on hold, resumed by wave P8 of the modular-architecture goal)
   still wanted?
6. Is `choir-autoradio-learning-engine-2026-11-14.md` (goal) superseded? ACTIVE lists it live; the
   learning-loop goal of 2026-11-18 was drafted and cleared.
7. The six intent goal files (voice pipeline, records, shipping, operations, desktop, standing
   metamission) are marked governing, kind wanted, on the owner's words in their authority fields.
   Confirm that "an intention recorded from the owner's words" counts as governing.
8. `docs/ACTIVE.md` is marked governing on CLAUDE.md alone; decision D3 of the transform may replace it.

## Where the skim was wrong

D: `choir-selfdev-apps-feature-complete` (on hold, not superseded); `density-d2` (it is the
superseding document); `entity-layer` (a proposal, not superseded by any text found);
`platform-os-app-state` (stale, not superseded); `store-once`, `autoradio-mission-handoff`,
`autoradio-swift-test-map`, `choir-autoradio-learning-engine` goal and `choir-selfdev-readiness`
(no statement of supersession). C: `choir-autoradio-in-the-desktop` carries the owner's words
(governing, wanted); `memo-diagonal-media-strategy` and `signal-is-sparse` are memos, not
proposals; `mac-device-key-release` is undecided only because no owner ruling backs a seat's
acceptance. A: `README`, `choir-prompting-invariants` and the security-density checklist have no
owner ruling or outcome behind them. In bucket A, `autoradio-in-exact` and `first-design` are
governing for the rulings they carry although their status lines say proposed or idea.

## All rows

Skim bucket, path, standing set here, basis, replaced-by, reason when not headed.

| Bucket | Path | Standing | Settled by | Replaced by | Reason |
| --- | --- | --- | --- | --- | --- |
| A | docs/ACTIVE.md | governing | named the live station list in CLAUDE.md Product; the campaign file may replace it (owner decision D3 open) |  |  |
| A | docs/README.md | unverified |  |  | a map of the docs tree; no owner ruling or outcome behind it, so it is not marked governing |
| A | docs/agent-product-doctrine.md | governing | named normative product rules in CLAUDE.md Desks ('Product rules: docs/agent-product-doctrine.md') |  |  |
| A | docs/changes.md | governing | CLAUDE.md Delivery, owner 2026-11-22: 'its hard to catch up on what claude has done'; every landing is entered here |  |  |
| A | docs/choir-doctrine.md | governing | CLAUDE.md: 'Architecture is normative in docs/choir-doctrine.md'; doctrine wins where the two conflict |  |  |
| A | docs/choir-prompting-invariants.md | unverified |  |  | its status line asserts an operating invariant; no owner ruling or CLAUDE.md line names it |
| A | docs/choir-vision.md | governing | named the product vision in CLAUDE.md Product; its status line (line 5) puts it under the doctrine |  |  |
| A | docs/computer-ontology.md | governing | CLAUDE.md Product: read before changing VM, candidate, promotion, package or persistent-state behavior |  |  |
| A | docs/current-architecture.md | governing | CLAUDE.md Product, owner 2026-11-16: read before saying what the system does; a landing that changes a row edits it |  |  |
| A | docs/definitions/choir-autoradio-on-android-2026-11-22.md | governing | owner 2026-11-25, quoted in this file's authority field (line 4); live goal in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md | governing | owner 2026-11-20, quoted in this file's authority field (line 5); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-first-design-2026-11-22.md | governing | owner 2026-11-22 ('great, this is the plan for the next throughline'), authority field line 4 |  |  |
| A | docs/definitions/choir-operations-podcasts-and-providers-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness intent |  |  |
| A | docs/definitions/choir-records-and-measurement-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness executable; live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-shipping-the-app-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-standing-metamission-2026-11-22.md | governing | owner 2026-11-22, quoted in this file's authority field (line 4); named as the run target in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-storage-efficiency-2026-11-22.md | governing | owner 2026-11-22, quoted in this file's authority field (line 4); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-voice-pipeline-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness intent |  |  |
| A | docs/design/account-create-delete-2026-11-23.md | governing | owner rulings 2026-11-23 listed in this file's status note (line 5) |  |  |
| A | docs/design/autoradio-dj-programme-2026-11-11.md | governing | owner 2026-11-11, verbatim (line 24): 'a new desk, an auto radio' |  |  |
| A | docs/design/autoradio-in-exact-2026-11-18.md | governing | owner 2026-11-18, section 'Where data may go' (line 238); the rule is CLAUDE.md Product |  |  |
| A | docs/design/autoradio-learned-document-2026-11-17.md | governing | owner rulings 2026-11-17 (line 122) |  |  |
| A | docs/design/capsule-overlay-computer-state-2026-11-03.md | governing | owner 2026-11-03 (line 11): 'capsules should be an overlay over the exact computer state' |  |  |
| A | docs/design/design-session-2026-11-23.md | governing | the owner's words of 2026-11-23, quoted exactly in this file (line 7); a record, not a design |  |  |
| A | docs/design/desks-and-engine-2026-11-24.md | governing | owner 2026-11-24 (line 11): 'the new design superseded'; docs/system-map.md section 10 names it current |  |  |
| A | docs/design/first-design-2026-11-22.md | governing | owner 2026-11-22, 'this is the plan for the next throughline' (the first-design goal file, authority field) |  |  |
| A | docs/design/forced-modularity-build-2026-11-04.md | governing | owner rulings 2026-11-04, settled and quoted (line 5) |  |  |
| A | docs/design/jev-batched-questions-2026-11-10.md | governing | owner correction 2026-11-10, verbatim (line 12); also CLAUDE.md Delivery (owner 2026-11-15) |  |  |
| A | docs/design/library-app-store-security-intent-2026-11-04.md | governing | owner conversation 2026-11-04 recorded (line 5): not to build yet |  |  |
| A | docs/design/modular-architecture-plan-2026-11-04.md | governing | owner rulings R1 to R38 of 2026-11-04 (section 1, line 31); 'the one plan the owner reads' |  |  |
| A | docs/design/onboarding-voice-tour-2026-11-25.md | governing | owner direction 2026-11-25 (line 5) |  |  |
| A | docs/design/one-tape-durability-2026-10-31.md | governing | owner 2026-10-31 (line 8): 'one append-only tape' |  |  |
| A | docs/design/platform-operator-account-2026-11-06.md | governing | owner ruling 2026-11-06 (line 9) |  |  |
| A | docs/design/programmatic-model-switching-2026-11-09.md | governing | owner rulings 2026-11-09 (lines 13, 119) |  |  |
| A | docs/design/recoverable-changes-sr7-2026-10-24.md | governing | owner invariant 2026-10-24 (line 13): any release boots on any tape written by it or a later release |  |  |
| A | docs/design/resource-fabric-2026-10-31.md | governing | roadmap-2026-10-31.md line 25 names it the stage 2 end state (owner 2026-10-31: 'the canonical sequence') |  |  |
| A | docs/design/second-factor-for-sensitive-acts-2026-11-09.md | governing | owner direction 2026-11-09 (line 5) |  |  |
| A | docs/design/selfdev-apply-and-revert-2026-11-06.md | governing | owner rulings 2026-11-06 (section 0, line 12); docs/system-map.md says read first for self-development |  |  |
| A | docs/heresy-detectors.md | governing | executable: scripts/check-heresies.sh reads it in CI; named by docs/choir-doctrine.md line 39 |  |  |
| A | docs/inbox.md | governing | the owner's words, dated, as said (line 5); named an authority source in docs/definitions/choir-docs-transform-2026-11-27.md |  |  |
| A | docs/launch-checklist-2026-10-21.md | governing | owner direction 2026-10-21 (line 9); linked from docs/ACTIVE.md |  |  |
| A | docs/learning-engine.md | governing | written from the owner's statements of 2026-11-13 and 2026-11-14 (line 6); named in CLAUDE.md Desks |  |  |
| A | docs/memo-per-computer-frontend-2026-08-13.md | governing | listed as a source in docs/choir-doctrine.md lines 34 and 235; cited in docs/current-architecture.md line 190 |  |  |
| A | docs/memo-problem-documentation-first.md | governing | CLAUDE.md 'Problems come first' names it as the rule |  |  |
| A | docs/node-a-shared-host.md | governing | owner decision 2026-10-09 (line 5) |  |  |
| A | docs/operating-contract-rationale.md | governing | keeps the owner quotes, dates and reasons behind each CLAUDE.md rule (line 6) |  |  |
| A | docs/re-engineering-decisions-2026-10-12.md | governing | owner-agreed design direction (status line 5), agreed before any code change |  |  |
| A | docs/roadmap-2026-10-31.md | governing | owner 2026-10-31 (line 10): 'that's my mental roadmap'; named in CLAUDE.md Product |  |  |
| A | docs/security-density-architecture-checklist-2026-10-21.md | unverified |  |  | a living checklist linked from docs/ACTIVE.md; no owner ruling found in it |
| A | docs/standing-questions.md | governing | CLAUDE.md Missions: read before authoring or running a goal file |  |  |
| A | docs/system-map.md | governing | CLAUDE.md Product, owner 2026-11-22: 'each compaction you lose context on the complex system' |  |  |
| D | docs/NOW.md | superseded | this file's status (line 5): retired 2026-09-08; it points to docs/ACTIVE.md | docs/ACTIVE.md |  |
| D | docs/conjecture-assertion-ledger-2026-06.md | superseded | status (line 5): historical ancestor, no longer updated; CLAUDE.md Proof names the live ledger | docs/calibration/claude-ledger.md |  |
| D | docs/definitions/choir-autoradio-learning-engine-2026-11-14.md | unverified |  |  | no statement of supersession found; docs/ACTIVE.md lists it as a live station and choir-autoradio-learning-loop-2026-11-18.md does not say it replaces it (skim was wrong or unproved) |
| D | docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md | superseded | docs/ACTIVE.md 'Superseded (2026-10-28): consolidate and scale', dropped by the owner; terminal receipt in the file | none |  |
| D | docs/definitions/choir-exact-search-entities-2026-11-20.md | superseded | the successor's supersedes field (its line 4) | docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md |  |
| D | docs/definitions/choir-guest-isolation-2026-11-01.md | superseded | this file's now.status (line 145); the successor names it as baseline source | docs/definitions/choir-store-header-body-split-2026-11-02.md |  |
| D | docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md | superseded | successor header line 6 'Supersedes'; docs/ACTIVE.md 'Superseded (2026-10-24)' | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md |  |
| D | docs/definitions/choir-memory-optimization-2026-11-01.md | superseded | owner 2026-11-01, this file's now.superseded_by (line 72); docs/ACTIVE.md says guest isolation supersedes it | docs/definitions/choir-guest-isolation-2026-11-01.md |  |
| D | docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md | superseded | the successor's supersedes field (its line 4) and line 257 | docs/definitions/choir-exact-search-entities-2026-11-20.md |  |
| D | docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md | superseded | owner 2026-11-04 ~20:55Z 'Let's supersede the current metamission' (line 173); successor line 12 | docs/definitions/choir-modular-architecture-2026-11-04.md |  |
| D | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md | unverified |  |  | skim was wrong: it is on hold (owner 2026-11-03) and choir-modular-architecture wave P8 resumes it; not superseded |
| D | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md | superseded | this file's readiness note (line 15) and now.status (line 155); successor line 7 | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md |  |
| D | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md | unverified |  |  | closed checkpoint_incomplete (docs/ACTIVE.md); choir-consolidate-and-scale supersedes only its ordering, and was itself dropped |
| D | docs/definitions/choir-t1a-app-self-development-2026-10-26.md | superseded | this file's readiness note (line 18): 'SUPERSEDED 2026-11-02 ... do not execute'; successor line 7 | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md |  |
| D | docs/design/autoradio-learning-engine-2026-11-14.md | superseded | this file's header (line 5) and the successor's line 6 | docs/design/autoradio-learned-document-2026-11-17.md |  |
| D | docs/design/autoradio-mission-handoff-2026-11-09.md | unverified |  |  | a handoff for the Swift era; no document says it is superseded (skim inferred it) |
| D | docs/design/autoradio-swift-test-map-2026-11-18.md | unverified |  |  | no document says it is superseded; it is a station record of the Exact cutover |
| D | docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md | unverified |  |  | skim was wrong: it is the document that supersedes fp10 lever L20, not superseded itself; standing not established |
| D | docs/design/entity-layer-2026-11-20.md | undecided | nothing; this file's status (line 5): a proposal, not ratified |  |  |
| D | docs/design/first-design-path-2026-11-22.md | superseded | this file's header (lines 5 to 7): 'the owner later ruled one kind, subject' | docs/design/first-design-2026-11-22.md |  |
| D | docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md | superseded | this file's line 5 and the successor's line 5 | docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md |  |
| D | docs/design/ideal-form-draft-2026-11-23.md | superseded | owner 2026-11-24 'the new design superseded' (header line 5; system-map section 10) | docs/design/desks-and-engine-2026-11-24.md |  |
| D | docs/design/library-package-propagation-2026-10-26.md | superseded | owner ruling 2026-11-03 'published releases mean published' (line 11) | docs/problems/published-release-visible-only-to-publisher-2026-11-03.md |  |
| D | docs/design/progressive-delivery-2026-10-26.md | superseded | owner 2026-11-03 'no switches at all' (line 9; rulings at line 785) | docs/definitions/choir-progressive-delivery-2026-11-03.md |  |
| D | docs/design/restore-bases-store-once-2026-11-15.md | superseded | header line 5: set aside 2026-11-15 after three review rounds (docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md) | none |  |
| D | docs/design/semantic-layer-2026-11-20.md | superseded | owner 2026-11-24 'the new design superceded' (header line 5; system-map section 10) | docs/design/desks-and-engine-2026-11-24.md |  |
| D | docs/design/snapshot-without-scratch-copy-2026-11-20.md | superseded | this file's header line 5; the replacing file's line 3 ('It supersedes') | docs/design/snapshot-one-pass-2026-11-22.md |  |
| D | docs/design/store-once-2026-11-14.md | unverified |  |  | its own header says design, nothing built; only the skim says it was set aside (restore-bases-store-once was, this one was not shown to be) |
| D | docs/platform-os-app-state.md | unverified |  |  | its status says canonical and stale as of 2026-09-22; stale is not superseded and nothing replaces it |
| D | docs/world-wire-mission-stack-2026-09-22.md | superseded | this file's status (line 6): SUPERSEDED roadmap, retained as a record (2026-10-14 prune) | docs/ACTIVE.md |  |
| C | docs/big-picture.md | undecided | nothing; this file's status (line 5): draft for the owner to cut down |  |  |
| C | docs/definitions/choir-autoradio-in-the-desktop-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); deferred in docs/ACTIVE.md |  |  |
| C | docs/design/autopaper-learning-materiality-2026-11-04.md | undecided | nothing; this file's status (line 5): design note, not decided |  |  |
| C | docs/design/autoradio-on-device-voice-configurations-2026-11-11.md | undecided | nothing; this file (line 5): a research note with a proposal |  |  |
| C | docs/design/branching-tape-2026-10-31.md | undecided | nothing; this file's status (line 5): design only, section 10 doctrine delta needs owner ratification |  |  |
| C | docs/design/branching-tape-mission-proposal-2026-11-10.md | undecided | nothing; this file's status (line 11): proposal, needs owner ratification of the doctrine delta |  |  |
| C | docs/design/capsule-long-exec-2026-10-15.md | unverified |  |  | no status line saying proposal or ratified; may be built |
| C | docs/design/ci-runner-slots-2026-10-24.md | undecided | nothing; this file (line 8): proposal for FP0, not implemented |  |  |
| C | docs/design/coding-agent-desk-over-mcp-2026-11-09.md | undecided | nothing; this file (line 754): its shape ordering is a proposal, unratified |  |  |
| C | docs/design/computer-history-restore-audit-2026-10-30.md | undecided | nothing; this file's status (line 5): not ratified, nothing built |  |  |
| C | docs/design/corpus-first-retrieval-and-research-2026-11-18.md | undecided | nothing; this file's status (line 5): a proposal, not ratified |  |  |
| C | docs/design/delegate-first-for-source-changes-2026-11-02.md | unverified |  |  | carries an owner quote of 2026-11-02 but is an orange change record; no status line |
| C | docs/design/engineering-network-grants-2026-10-10.md | undecided | nothing; this file's status (line 6): design proposal, not settled |  |  |
| C | docs/design/leafrunner-seam-mission-proposal-2026-11-10.md | undecided | nothing; this file's status (line 5): proposal, readiness drafted |  |  |
| C | docs/design/library-app-store-2026-11-04.md | unverified |  |  | states rules after an owner review quote of 2026-11-04; reads as built, not a proposal |
| C | docs/design/mac-device-key-release-2026-11-01.md | undecided | nothing; accepted by a review seat (Devin round 3, this file line 5), no owner ruling found |  |  |
| C | docs/design/object-bodies-inline-on-the-tape-2026-11-15.md | unverified |  |  | design record; no status line on ratification |
| C | docs/design/outboard-capsules-arch-11-12-2026-10-22.md | unverified |  |  | design only, P2; its name is a proposal (OQ-1) but the design is not marked unratified |
| C | docs/design/paper-control-cli-2026-11-09.md | unverified |  |  | status is design, not built; no statement that it is unratified |
| C | docs/design/sdlc-audit-2026-11-10.md | undecided | nothing; this file's status (line 5): proposal for the owner, nothing adopted |  |  |
| C | docs/design/secrets-vault-design-2026-10-28.md | undecided | nothing; this file's status (line 5): draft for re-review, nothing authorized to execute |  |  |
| C | docs/design/texture-spec-follows-owner-2026-10-25.md | unverified |  |  | a change record (red, tier 1); no status line; probably built |
| C | docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md | unverified |  |  | design, nothing built; no statement on ratification |
| C | docs/memo-diagonal-media-strategy-2026-09-17.md | unverified |  |  | skim was wrong: owner strategy direction recorded, not doctrine, predates Autopaper (header update 2026-10-14) |
| C | docs/operational-invariants-register-2026-10-08.md | undecided | nothing; this file's status (line 5): draft for owner review |  |  |
| C | docs/provider-vault-and-model-policy-design-2026-10-22.md | undecided | nothing; this file's status (line 5): proposal, not owner-ratified |  |  |
| C | docs/signal-is-sparse-not-the-learner-2026-08-01.md | unverified |  |  | skim was wrong: a foundational memo, not a proposal; its status says memo, not doctrine |
| B | docs/autopaper-sw-design-2026-10-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autopaper-improvement-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autopaper-overnight-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autopaper-overnight-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-full-experience-overnight-2026-11-15.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-learning-engine-2026-11-14.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autoradio-overnight-2026-11-17.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-pass1-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-architecture-selection-2026-10-28.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-system-build-2026-10-30.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-system-build-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-guest-isolation-2026-11-01.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-launch-fp0-density-bench-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp0-proof-scripts-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp1-a1-boot-alter-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-06-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-08-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-09-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-11-12-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net-03-04-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net-05-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net10-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-p2-small-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-08-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-10-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-13-22-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-14-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-20-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res17-reaper-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-sec14-deploy-user-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp3-host-floor-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp6-network-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-lean-system-2026-10-30.lanes.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-lean-system-2026-10-30.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-lean-system-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-mac-sync-stage2-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-memory-optimization-2026-11-01.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-modular-architecture-2026-11-04.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-modular-architecture-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-modular-architecture-2026-11-04.precommit.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-ordinary-prompt-path-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-ordinary-prompt-path-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-progressive-delivery-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-progressive-delivery-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-2-2026-11-12.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-2-2026-11-12.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-store-header-body-split-2026-11-02.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-store-header-body-split-2026-11-02.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-v-boot-vocab-pass-removal-2026-10-27.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/ci-test-cooptimization-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/corpus-first-search-one-verb-2026-11-19.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/design/agent-tree-budgets-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/app-design-bar-2026-10-26.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/app-motion-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: built (header); UI rule |
| B | docs/design/app-packages-replan-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/append-clustered-store-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/append-receipt-trim-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/apps-feature-complete-spec-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/autoradio-app-api-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-app-ui-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-desk-fast-turns-2026-11-11.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-l1-failure-list-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-lab-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-page-2026-11-17.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-queue-desk-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-system1-route-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-turns-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-voice-modes-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/bounding-the-guest-store-2026-11-14.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/bundle-authenticity-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/choir-data-system-frame-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/ci-node-b-build-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/coding-agent-desk-station-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/data-system-build-sheet-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/density-d6-page-cache-return-2026-10-26.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/derived-key-marks-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/desk-shots-and-review-2026-11-07.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/egress-grant-object-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/email-sign-in-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner 2014-10-14/10-31 ask; email sign-in code and deployed e2e exist (commit 0cff2f27a) |
| B | docs/design/enforce-flip-2026-11-08.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/engineer-reuse-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/intake-desk-pace-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/ma65-yaegi-go-modules-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/ma66-code-mode-import-sets-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/object-graph-redesign-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/object-storage-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/object-verbs-quotas-ma38-ma39-2026-11-05.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/package-operation-decision-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: built per its header; apply-and-revert release 8a9f79cdf |
| B | docs/design/paper-compaction-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/podcast-clips-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/research-more-goes-to-research-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/ring-ordered-deploys-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner rulings 2026-11-03; internal/vmctl/ring_walk.go exists; CLAUDE.md Ring-Expedite rule is its outcome |
| B | docs/design/run-record-written-less-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/s12-runtime-install-build-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/sections-as-questions-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/shot-verdict-reuse-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/small-object-storage-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/store-header-body-split-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/tape-group-commit-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: tape group commit landed (abfab8754) |
| B | docs/design/tetramark-motion-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: built (header); UI rule |
| B | docs/design/visual-evidence-slice-2026-10-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/model-pools-design-2026-10-12.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/precommitment-records-practice-2026-11-08.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/restart-subsystem-design-2026-10-13.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/semantic-registry.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/vmctl-360-review-2026-10-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| E | docs/definitions/choir-autoradio-learning-loop-2026-11-18.md | not assessed |  |  | bucket E is not headed in this station; skim said: readiness drafted; owner cleared the goal 2026-11-18 |
| E | docs/design/account-boundary-2026-11-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/account-switcher-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/app-desk-compute-capsule-2026-10-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/autopaper-next-mission-handoff-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/autopaper-step0-contracts-2026-11-10.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/autoradio-exact-integration-2026-11-19.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-experiment-e1-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-seed-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-wiring-permutations-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/candidate-on-de-before-main-2026-11-10.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/capsule-heavy-work-scheduling-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/capsule-overlay-s4-source-failure-modes-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/capture-static-copy-2026-11-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ci-prove-once-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/claude-delegate-slice1-2026-10-14.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/cli-agent-onboarding-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/computer-update-architecture-rnd-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/delegate-one-round-build-2026-10-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/desk-install-review-2026-11-08.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/file-object-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/first-design-offline-slice-plan-2026-11-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/gateway-handoff-without-a-gap-2026-11-20.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/gateway-streaming-for-every-model-2026-11-11.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/guest-tool-layers-2026-10-31.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/guest-tools-live-load-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/layered-guest-image-2026-10-31.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ma62-ma63-hardening-2026-11-05.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ma64-gvisor-desk-workers-2026-11-06.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/model-pool-autoroute-2026-11-02.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/modularity-check-build-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/replay-time-capture-with-unknown-cost-2026-11-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/sandbox-pause-resume-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/snapshot-one-pass-2026-11-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/snapshots-cost-the-change-2026-11-13.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/storage-pressure-frame-2026-11-12.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/texture-evidence-embeds-te-2026-10-25.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/texture-holds-the-trajectory-2026-10-25.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/trace-segment-log-2026-10-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/update-now-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/desk-system-harness-architecture-2026-10-01.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/mission-residues.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/runtime-invariants.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/source-external-data-publication.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/texture-agentic-invariants-2026-06-13.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/texture-live-supervision-architecture.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/why-texture-2026-06-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
