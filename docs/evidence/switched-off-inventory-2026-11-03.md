# Switched-off inventory, origin/main, 2026-11-02

Sources (FACT, read at origin/main): `switches/registry.yaml` (86 entries, seeded 2026-10-28, lint `scripts/check-switch-registry`), `internal/store/computer_setting.go` (KnownComputerSettings, 27 keys, plus 2 added by init() in `internal/store/app_package_objects.go:32` and `internal/store/file_object.go`; intake.events and intake.event_wakes are also registered elsewhere), `nix/node-b.nix`, `docs/evidence/switched-off-inventory-2026-10-26.md` (prior audit, per-computer proofs).
Caveats: (1) the registry's `ring` column is a hand-kept snapshot (FACT: header says "until the controller exists the nix value is the state"); per-computer stored settings are NOT visible from the repo, so "on at d@e/b@c" for settings is registry/ledger claim, not verified here (HYPOTHESIS until `GET /api/computer-settings` per computer). (2) Rings: R0 d@e (computer-59c902c5...), R1 class:test, R5 the paper (computer-1355b83b...), advances only by human act.
(3) The 10-26 audit's finding that a default flip does nothing is now OUT OF DATE: FACT `internal/agentcore/computer_settings.go:44` computerSettingOn returns `KnownComputerSettings[key]` on ErrNotFound, and `autoputer/file_objects.go:126` and `autoputer/app_package_objects.go:101` do the same. So flipping the map default works for those readers. Other direct readers named in the audit (event_wakes.go:349, store/graph_home.go:37, textureowner/texture_owner_directions.go:121) were not re-checked: HYPOTHESIS they still ignore the default.

## A. Computer settings (owner act at /api/computer-settings; default in KnownComputerSettings)

| # | Setting (path) | Does | Default / where on | Introduced / waits-on evidence | Recommendation |
|---|---|---|---|---|---|
| 1 | texture.owner_directions (computer_setting.go, SettingTextureOwnerDirections) | Texture turns see owner's later directions, ranked above seed | off; registry: R0 + 1 id (b@c). Byte-identical when off | texture-spec-follows-owner-2026-10-25; proved b@c and d@e (registry note "ON NOW") | SWITCH ON: default true (fleet), paper included is low risk but paper has no owner revisions |
| 2 | engineering.progress_to_texture | engineering Reports also land in owner's Texture inbox | off; R0 + b@c | problems/engineering-reports-never-reach-texture-2026-10-24; bunch 210 APPROVE | SWITCH ON default |
| 3 | engineering.carry_unfinished | stopped round's patch kept and offered to next round | off; R0 + b@c | selfdev-trace-reviews-2026-10-25; known nit: keeps never pruned (~2 MiB/round, bunch230-F7) | SWITCH ON default, fix pruning as follow-up |
| 4 | texture.owner_act_wakes | approve/reject/apply wakes the candidate's Texture | off; R0 + b@c | problems/owner-acts-on-a-candidate-do-not-wake-texture-2026-10-25; proved b@c 10-26 | SWITCH ON default |
| 5 | texture.evidence_embeds | Freeze's visual captures embed in the Texture doc | off; 1 id | design texture-evidence-embeds-te-2026-10-25; needs CHOIR_VISUAL_CAPTURE grant (item B1) | SWITCH ON where capture grant exists (R0,R1, then fleet with B1) |
| 6 | texture.hold_trajectory | Texture continues earlier conversation verbatim vs rewarm digest | off; R0 + b@c | design texture-holds-the-trajectory-2026-10-25; review 240 ACCEPT; named residuals 1-3 "watch on staging" | SWITCH ON owner computers; paper after owner-computer read |
| 7 | source.projection (also env CHOIR_SOURCE_PROJECTION, internal/sourceproj/sourceproj.go:28) | guest source repo follows effective release; cast gate checks it | off; 1 id (b@c) | computer-update-architecture-rnd-2026-10-24 4.6; B4/B5 proved | SWITCH ON proof ring then fleet; needs host base fetch reachable. Possible Dolt dependence: HYPOTHESIS no, it is a git repo |
| 8 | source.rebase_offer | asks root Texture to rebase applied stack when base moved | off; ring "unknown" in registry | design u4-stack-rebase-build-2026-10-26; "act-gated until the U4 rebase has run" | KEEP OFF until U4 rebase run once on d@e (stated reason in registry); effective only with source.projection + texture.root_desk |
| 9 | engineering.slots_graph | engineering slot claims on tape not co_super_slots table | off; 1 id (b@c since 10-25) | data-has-more-than-one-home-2026-10-24; bunch 218 both APPROVE | SWITCH ON default (also moves toward single state authority) |
| 10 | evidence.graph | research evidence on tape graph kind, imports table on turn-on | off; 1 id | same problem doc; paper-size import (25 s bounded) unmeasured | SWITCH ON owner computers/R0-R1; paper after a dry run on a paper-size copy |
| 11 | app_packages.objects (store/app_package_objects.go:30-32) | owner route to package-declared kinds | off; 1 id | design app-packages-replan-2026-10-24 P2b | SWITCH ON default |
| 12 | files.object (store/file_object.go; design file-object-2026-10-24 line 263) | file object / Files API mutations as objects | off; 1 id | file-object-2026-10-24 | SWITCH ON default |
| 13 | texture.root_desk | master document of open work, own Texture desk | off; 1 id | archive pd3-texture-root-desk; S1 accepted, S2 F4 FAILED, fix status not in ledger | KEEP OFF on paper (registry: hundreds of story docs churn master); owner computers on after F4 fix proved |
| 14 | packages.publish | owner may publish package releases to Library | off everywhere; registry off_until 2026-11-04 | library-package-propagation-2026-10-26 3.2; no live publish yet | SWITCH ON R0/R1, run first Reader release publish; walk-by date is in 2 days |
| 15 | packages.share | share/withdraw/reinstate releases | off everywhere; off_until 2026-11-04 | same design S3; review 293 PASS | SWITCH ON R0 with 14 (red: cross-owner visibility) |
| 16 | intake.events | event-layer shadow resolution | off; paper ON | handoff-2026-10-15 | paper feature; KEEP as is (not for owner computers) |
| 17 | intake.labels | Jev labels for arriving captures | off; paper ON | l2a ledger 10-14 | KEEP (paper feature) |
| 18 | research.system1 | research desk may ask System 1 | off; paper ON | owner notes 29 | KEEP (paper feature) |
| 19 | research.related_story | judged pair relations, exported to Store B | off; paper ON since 10-26 | problems/related-stories-thin... ; read half CORPUSD_RELATED_STORY on in nix | KEEP; platform-tier only |
| 20 | intake.event_wakes | production waits on events | off everywhere; off_until 2026-11-25 | event layer; false-merge bar failed 3 times (7.0%, 9.5%, 15-30%) | KEEP OFF, stated reason: structural assessment owed (CLAUDE.md dead-end rule). Not a flip |
| 21 | store.event_table (+ host CHOIR_EVENT_TABLE_COMPUTERS, nix/node-b.nix:1771, empty) | telemetry in own choir_events table | off everywhere; registry off_until 2026-11-25 | archive choir-event-table-f-c-2026-10-22 | NOT DEAD: FACT choir_events is a live SQLite table (docs/design/store-header-body-split-2026-11-01.md). But description speaks of "prolly trees" (Dolt) so the rationale is stale. Recommend DECIDE: given telemetry.batch + partitions + group_commit now carry the cost, HYPOTHESIS it is redundant; delete or justify |
| 22 | calibration.feedback | ICRL digest to desks | off everywhere | owner 10-24 stays off; BUT ledger line 1588 (owner, in-session): "Yes, we lift calibration feedback stays off" supersedes 10-14. docs/design/calibration-seed-2026-11-03.md: "automatic pack injection stays off until held-out evidence" | CONFLICT: registry note says stays off, owner ruling says lift. Resolve; switch on d@e if the held-out evidence is available. Needs owner to confirm which ruling stands |
| 23 | management.serial_tasks | K=1 management tasks | off | S2b rollback lever | NOT a feature; "on" is the rollback. Exclude |
| 24 | dispatch.wide_slots | dispatcher slots 8 -> 12 | off; registry ring "unknown", no walk data | owner for the paper 2026-10-27 "I have the tokens for it" (related-stories-not-showing-on-the-paper) | SWITCH ON for paper now: owner asked for it; if never turned on, say so. Verify current state via GET |
| 25 | store.telemetry_partitions | tool.result in per-day partition files | off; R0 (d@e stored) | one-tape-durability-2026-10-31 6c; turning on writes store floor 5 | WIDEN: paper next, then default (the doc's own ladder). Floor 5 is irreversible-ish, so soak d@e first |
| 26 | tape.group_commit | acknowledged writes share one tape event; telemetry and run messages ride it | off; R0 | tape-group-commit-2026-10-31; ledger: "Next: d@e setting on after deploy, then counters" | WIDEN: paper, then default, after d@e counters |
| 27 | store.compact_force | makes daily compact copy always due | off, R0 by design | paper-compaction-2026-10-31 | KEEP (proof-ring-only lever by design) |
| 28 | telemetry.batch | batched telemetry | default ON now (FACT computer_setting.go) | | already on; registry still says R0 (stale) |
| 29 | store.telemetry_retire | daily retirement | default ON now | | already on; paper first run removes ~2 GB, owner act pending (data-system ledger line 220: "Retirement stays off on the paper... owner act") — FACT conflict with "default on": the paper may hold a stored false |
| 30 | store.debounced_checkpoint, store.trace_log | retired | known, nothing acts | FACT comments in computer_setting.go | DELETE as dead (registry entries and Known keys); keep keys only so stored values read |

## B. Host selectors and let-bindings in nix (host edit + deploy, effective at cold boot)

| # | Name (nix/node-b.nix) | Does | Current | Doc / evidence needed | Recommendation |
|---|---|---|---|---|---|
| B1 | CHOIR_VISUAL_CAPTURE_COMPUTERS :1762 | visual capture grant to capsules | ON: b@c (7011602c), computer-6c50d6e5, d@e (59c902c5, added 2026-11-02), class:test | selfdev preview | WIDEN to owner computers/class:primary, pairs with A5 |
| B2 | CHOIR_CLAUDE_DELEGATE_COMPUTERS :1809 (+EXPIRES 2026-11-17) | Claude/Sonnet delegate | ON: computer-ad1b46d4, computer-6c50d6e5; registry experiment, expires 2026-11-03 but nix expiry now 11-17 (registry stale) | claude-delegate-slice1-2026-10-14 | KEEP experiment; update registry expiry |
| B3 | CHOIR_EVENT_TABLE_COMPUTERS :1771 | host half of A21 | empty | see A21 | see A21 |
| B4 | CHOIR_CAPSULE_MIRROR_COMPUTERS/ECOSYSTEMS/MIRRORS_ONLY :1782 | capsules use go-choir-pkgmirror | empty everywhere; off_until 2026-11-04 | fp10-net02-mirrors; also needs pkgmirror unit -ecosystems | SWITCH ON R0 (walk plan in registry: one test computer, class:test, MIRRORS_ONLY=1) |
| B5 | hostFetchVMs (let-binding; CHOIR_HOST_FETCH_VMS) | host fetch route, closes direct egress | "" everywhere; off_until 2026-11-04 | fp10-net01-host-fetch; "high security value" | SWITCH ON R0 first (red) |
| B6 | gatewayModelScheduler | model scheduler at gateway | off; off_until 2026-11-04; "proof 9 never run" | ec25d61ff | RUN proof 9, then on; own deploy |
| B7 | PROXY_EXPAND_REQUESTERS | proxy expand requester list | none; off_until 2026-11-04 | audit item 49 | SWITCH ON or delete; no evidence on file |
| B8 | VM_SNAPSHOT_DIFF_ONLY | diff snapshots | R0,R1 | accept_five.py | WIDEN R2..R4 per soak (registry expires 11-11) |
| B9 | VM_CHECKPOINT_CADENCE_ONLY | checkpoint cadence | R0 only | "not the paper until measured" | WIDEN to class:test; measure |
| B10 | VM_BALLOON_STATS_ONLY | balloon stats | R1 + 1 id | RES-10 | WIDEN |
| B11 | VM_GUEST_RECLAIM_ONLY (DAMON) | guest page reclaim | 2 ids (9648031f, 71afbf14) | RES-06; ledger line 376: DAMON returns no host memory in idle window for the paper | KEEP narrow; paper reclaim stays off until free-page-reporting is read |
| B12 | VM_CAPSULE_TUNNEL_ENFORCE_ONLY | capsule tunnel identity | 1 id | | WIDEN |
| B13 | VMCTL_GUEST_DNS_ONLY | guest DNS | R1,R3 | | WIDEN |
| B14 | VMCTL_GUEST_CACHE_RETURN_* | D6 cache return | R0 + 1 id; target R5 | D6 design s12 | keep ladder |
| B15 | VMCTL_PRIMARY_KEEPALIVE_MODE | ring "none" in registry | none | ops | check: HYPOTHESIS it is off everywhere; decide |
| B16 | VMCTL_STOP_MANAGED_ON_EXIT | ring none | none | ops | check |
| B17 | PLATFORM_RECEIPT_COMPACT_COMPUTERS :1139 | lossless receipt compaction | d@e + 2 ids active | paper widens by human act after dry run | KEEP ladder |
| B18 | VMCTL_GUEST_LAYERED / guestLayeredComputers :48 | layered guest image | d@e only | | widen when proven |
| B19 | ci_reuse_green_pr_run | CI reuse | none; off_until 2026-11-04 | audit row 42 | decide: on or delete |
| B20 | unwired_packages: internal/appdesk, appnet, egressgrant | unwired code | none; off_until 2026-11-25 | app-packages-replan: "on main, no importer outside tests" | See C |

## C. Code env gates (not nix-wired)

| Name | Does | Default | Evidence | Recommendation |
|---|---|---|---|---|
| CHOIR_APP_DESKS (internal/appdesk/appdesk.go:328) | package-to-desk join, app desks | off; selfdev-apps mission says stays off "unless a slice needs it" | doctrine I26 (app desks, Texture first) vs this off switch: tension | SWITCH ON d@e: contradicts "app desk ships with its app package" in CLAUDE.md. Needs evidence of Activate being wired; HYPOTHESIS importers are zero (registry unwired_packages) |
| CHOIR_COMPUTE_CAPSULES (design app-desk-compute-capsule-2026-10-21 S2) | compute capsules | docs say off by default; NOT FOUND in internal/cmd/nix grep | | HYPOTHESIS unbuilt or renamed: verify, else DELETE from docs |
| CHOIR_EGRESS_GRANTS (fp10-net-05) | egress grants | docs say default off; NOT FOUND in code grep | registry says internal/egressgrant unwired | HYPOTHESIS dead/unwired: wire (R0) or delete |
| AUTH_DEVICE_RELEASE (mac-device-key-release-2026-11-01 line 213) | device key release | "stays off" in design; NOT FOUND in code | | design-only, no switch yet |
| CHOIR_OG_WRITER_GATE (internal/objectgraph/writer.go:137) | og writer gate, code default observe | observe | registry exp 2026-11-25 | decide enforce on d@e |
| CHOIR_TELEMETRY_BATCH env (telemetry_sealer.go:72) | env twin of telemetry.batch | now redundant (setting default true) | | DELETE env twin |
| CORPUSD_RELATED_STORY | read half of A19 | on in nix | registry: verify unit-env file and nix agree | verify |
| CHOIR_PACKAGE_SHARE_READS, CHOIR_RELEASE_READS | read scope | normal state | | EXCLUDE (not feature switches) |

Frontend: `grep import.meta.env` under frontend/src found no feature flag (only DEV/PROD/MODE excluded; none other returned). FACT.

## D. Counts
- Registry entries: 86 (FACT). Settings in Known map: 27 + 2 init. Of 29 settings: 2 on by default, 2 retired, 3 rollback/proof levers (serial_tasks, compact_force, calibration contested), ~13 off pending widen, 5 paper-only features.
- Off for every computer (registry `ring none`/off_until): packages.publish, packages.share, intake.event_wakes, store.event_table, capsule mirrors, hostFetchVMs, gatewayModelScheduler, PROXY_EXPAND_REQUESTERS, ci_reuse_green_pr_run, unwired_packages, calibration.feedback, management.serial_tasks, retired keys, VMCTL_PRIMARY_KEEPALIVE, VMCTL_STOP_MANAGED_ON_EXIT, CHOIR_EVENT_TABLE_COMPUTERS.

## E. Dead / data-system-stale candidates
- store.debounced_checkpoint, store.trace_log: retired (FACT comments). Delete registry rows; keep keys.
- store.event_table: live on SQLite but its rationale ("4 prolly trees vs 13") is Dolt era; likely redundant after batch/partitions/group_commit (HYPOTHESIS).
- CHOIR_COMPUTE_CAPSULES, CHOIR_EGRESS_GRANTS, AUTH_DEVICE_RELEASE: named in docs, absent from code grep (HYPOTHESIS dead or unbuilt).
- internal/appdesk, appnet, egressgrant: unwired per registry.
- Registry staleness: telemetry_batch ring R0 (default is now true); claude_delegate expiry 11-03 (nix says 11-17); store_telemetry_retire "on for every computer" vs ledger saying paper retirement is an owner act.
