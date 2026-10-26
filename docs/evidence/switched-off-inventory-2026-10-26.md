# Switch inventory, built but off (2026-10-26)

Read-only. Source: `origin/main` at `99524f638`; `docs/evidence/switched-off-inventory-2026-10-24.md` (updated here), `internal/store/computer_setting.go` (+ `init()` in `world_event.go`, `file_object.go`, `app_package_objects.go`), `nix/node-b.nix`, `nix/autoputer-vm.nix`, the selfdev and first-pass ledgers, the residual register of 10-26. Per-computer values are the orchestrator's (the repo cannot show them). Nothing was run against staging or Node B.

Legend. Class = CLAUDE.md class of the flip itself. "Setting act" = `POST /api/computer-settings {"key","value"}` with that computer's owner key (it acts on the computer the call reaches; not PUT as the 10-24 inventory said). Host selector = nix edit + deploy; reaches a computer at its next cold boot.

## 0. Findings that change the plan

1. **27 settings, not 9.** `KnownComputerSettings` has 23 keys plus 4 added by `init()`: `intake.events`, `intake.event_wakes`, `files.object`, `app_packages.objects`. Your paper list has 26 keys; **`packages.share` is missing from it** (added by Library S3, `22c3b13bd`/`3e4a08a69`, in train 10 `99524f638`, CI in flight at the last ledger line). Treat it as off everywhere.
2. **A default flip in `KnownComputerSettings` does nothing today.** `computerSettingOn` (agentcore/computer_settings.go:40) and six other direct readers (`event_wakes.go:349`, `autoputer/app_package_objects.go:93`, `autoputer/file_objects.go:122`, `store/file_object.go:373`, `store/graph_home.go:37`, `textureowner/texture_owner_directions.go:121`) treat `ErrNotFound` as off and never consult the map; the map's default only shows in `GET /api/computer-settings`. The durable fix is one substrate change first: make `GetComputerSetting` (or one `store.SettingDefault(key)` helper used by all seven) fall back to the map's default when no row is stored, so an explicit stored `false` stays a per-computer opt-out. Then flip the map values. Red (settings plane, every desk reads it); same-value POST idempotence at computer_settings.go:107 must be re-checked.
3. **Owner ruling 2026-10-15 is still standing against two switches** (`docs/reports/autopaper-compute-halving-2026-10-15.md` lines 11-17): "Don't economize on search; maintain the append-only pattern for 90%+ caching." That is `research.search_caps` (search) and `research.run_memory_elision` (rewrites cached history bytes). The 10-24 "be aggressive" ruling is general; neither ledger records it lifting this one. Treated below as: elision ON AFTER a cache-ratio read; search_caps LEAVE OFF. **Update 2026-10-26: the owner removed both switches entirely (docs/problems/research-economy-switches-removed-2026-10-26.md); the ruling below is moot.**
4. **`store.trace_log` has no reader** (registered by R7 S2, "not acted on"; grep shows only the definition and map entry). Turning it on does nothing. R7 S3-S6 not landed.
5. **`CORPUSD_RELATED_STORY` is the read half of `research.related_story`, which went on for the paper today.** It lives in `/var/lib/go-choir/unit-env/corpusd-retrieval.env` (outside the repo, not in nix) plus a corpusd restart. Its value is unknown to me. With it off the paper writes pairs and nobody sees them (the designed "audit-before-show" state).
6. `intake.event_wakes`: the design's bar (false merge <= 5%) failed three times (7.0%, 9.5%, 15-30% on small samples; `event-layer-false-merges-at-marginal-p-2026-10-16.md`), and the design says a third failure means a structural assessment. Not a flip.
7. Several "switches" are rollback levers whose default is the normal state; do not turn them: `management.serial_tasks`, `CHOIR_PACKAGE_SHARE_READS` (unset = `scoped` = normal; `owner` is the rollback), `CHOIR_RELEASE_READS` (unset = `owner`; `open` would widen reads), `CHOIR_OWNER_ACT_MODE` (observe by owner ruling 10-24 17:5x).

## 1. The paper (computer-1355b83b…, public Autopaper)

Rule from the owner's 10-24 delegation: paper last, after test and owner computers. Most self-development settings are meaningless here (no candidates, no capsule Freeze, no Reader packages).

| Switch | Kind | Default | What it does | Paper today | Proof | Class | Precondition | Recommendation |
|---|---|---|---|---|---|---|---|---|
| `intake.events`, `intake.labels`, `research.system1`, `research.related_story` | setting | off | event shadow resolution; Jev labels; System 1 for research; judged story pairs | ON | events: handoff-2026-10-15; labels: l2a ledger 10-14 06:50Z; system1: 614fd445f; related_story: 11:25Z today | orange | none | Done. |
| `CORPUSD_RELATED_STORY` | host env file | off | corpusd walk reads `related_story` edges, so the front shows pairs | unknown (file outside repo) | none; write side only live since 11:25Z | orange | FP0 audits the first pairs from the tape and Store B (design section 7); then edit the env file, restart corpusd | **ON AFTER** that one audit read. |
| `research.run_memory_elision` | removed | rolling elision of old research tool results | | | | | | **REMOVED 2026-10-26 by owner ruling** (docs/problems/research-economy-switches-removed-2026-10-26.md): the setting, env twins and code are deleted; stored rows stay on the tape, unread. |
| `research.search_caps` | removed | 12 results/provider and 300-char snippets | | | | | | **REMOVED 2026-10-26 by owner ruling** (docs/problems/research-economy-switches-removed-2026-10-26.md): the setting, env twins and code are deleted; stored rows stay on the tape, unread. |
| `store.debounced_checkpoint` | setting (env twin) | off | guest Dolt history commit off the write path (60 s / 10000 ops) | off | deploy R (0bf8a097d, rounds 1-7) shipped; kill -9 durability step never run | red (store durability) | kill -9 drill on a throwaway test computer, then another test computer | **ON AFTER** the kill -9 drill. Resource win on the paper's write path. |
| `evidence.graph` | setting | off | research evidence to the tape-backed graph kind; turning on imports the table rows (25 s bounded, retry by same-value POST) | off | b@c on since 10-25 00:22; no paper-sized import measured | red (tape) | measure the import against a copy of the paper-sized evidence table | **ON AFTER** a paper-size import dry run. |
| `texture.hold_trajectory` | setting | off | Texture continues its earlier conversation verbatim instead of the rewarm digest | off | b@c since 10-25 08:0x; d@e | red (Texture runs) | larger contexts on the paper's many Texture runs: read cost on owner computers first | **ON AFTER** it has run on an owner computer; then paper. |
| `texture.root_desk` | setting | off | debounced master document of the computer's open work + its own Texture desk | off | PD3 S1 accepted on b@c (pd3-s1-acceptance-2026-10-25.md); S2 paired test FAILED (F4, oldest-first excerpt), fix status not in the ledger | red | F4 fix proved | **LEAVE OFF** on the paper (hundreds of story documents would churn the master). |
| `texture.owner_directions`, `texture.owner_act_wakes`, `texture.evidence_embeds`, `engineering.progress_to_texture`, `engineering.carry_unfinished`, `engineering.slots_graph`, `source.projection`, `app_packages.objects`, `files.object`, `packages.publish`, `packages.share` | setting | off | self-development / owner-document / Library features | off | proved on b@c/d@e | various | n/a | **LEAVE OFF**: meaningless on the paper (no owner revisions, candidates, packages or files). Turn on only if the paper ever self-develops. |
| `intake.event_wakes` | setting | off | production waits on events; management wakes per event | off | V3a proof on a test computer 10-16 only; bar failed 3 times | red | structural assessment (third failure) | **LEAVE OFF**. |
| `store.event_table` | setting + host grant | off | ten telemetry kinds to `choir_events` | off | F-c deploys 1-2; host grant `CHOIR_EVENT_TABLE_COMPUTERS` empty; the POST refuses until the stream_seq lease is ready (computer_settings.go:98) | red (schema floor) | lease + grant + R7 decision | **LEAVE OFF** (R7 may make it redundant). |
| `store.trace_log` | setting | off | future trace segment log | off | no reader | n/a | R7 S4 | **LEAVE OFF** (no effect). |
| `telemetry.batch` | setting | off | batches ten trace kinds | off | phase-1 gate 10-22; ~10% journal gain | red | n/a | **LEAVE OFF** (owner 10-23: R7 replaces it). Delete with env twin. |
| `calibration.feedback` | setting | off | ICRL calibration digest | off | none | orange | n/a | **LEAVE OFF** (owner 10-24: stays off; the ICRL design replaces it). |
| `management.serial_tasks` | setting | off | K=1 management tasks | off | n/a | orange | n/a | **LEAVE OFF** (rollback for S2b; "on" is the rollback, not a feature). |

Paper ON NOW: **none new.** The only moves are the two ON AFTER reads (elision, related-story read) and the checkpoint drill. (Update 2026-10-26: the elision read is void; run_memory_elision is removed.)

## 2. The owner's own computers (values unknown to me)

I cannot see them; the orchestrator should `GET /api/computer-settings` for each first. These are the computers where the owner submits requests, so the self-development set is the point of them. Class premium_always_on (jailer wave 2a joins them at next launch; `VMCTL_JAILER_ONLY` now `class:test,class:primary,class:premium_always_on`).

| Switch | Default | What it does | Proof | Class | Precondition | Recommendation |
|---|---|---|---|---|---|---|
| `texture.owner_directions` | off | every Texture turn sees the owner's later directions, ranked above the seed | b@c 10-25 04:40, d@e; byte-identical when off | orange | none | **ON NOW** |
| `engineering.progress_to_texture` | off | engineering progress Reports also land in the owner's Texture inbox | b@c 10-24 22:42:50, d@e; bunch 210 APPROVE | orange | none | **ON NOW** |
| `engineering.carry_unfinished` | off | a stopped round's source change is kept and offered, labelled unfinished, to the next round | b@c 10-25 04:40, d@e; bunches 230/234 | orange | known nits: keeps never pruned (bunch230-F7, ~2 MiB per stopped round) | **ON NOW** |
| `texture.owner_act_wakes` | off | approve/reject/apply wakes the candidate's Texture | b@c 10-26 04:47:34, d@e | red-light (mints into the Texture inbox) | none | **ON NOW** |
| `texture.evidence_embeds` | off | a Freeze's visual captures embed in the trajectory's Texture document | b@c 10-25 05:1x; reader/WM rounds judged on them; bunches 231/235 | red (Texture canonical schema/writes) | needs the visual-capture grant on that computer (below) to have captures | **ON NOW** with the grant |
| `source.projection` | off | guest source repo follows the effective release; cast gate checks it | b@c 10-24 21:26; B4/B5 cases proved; three-way containment live 10:52 today | red | host base fetch reachable (fetches booted commit by sha) | **ON NOW** |
| `engineering.slots_graph` | off | engineering slot claims on the tape (double-claim fix included) | b@c since 10-25 00:22; bunch 218 both APPROVE | red (tape) | none | **ON NOW** |
| `evidence.graph` | off | research evidence on the tape; import on turn-on | b@c since 10-25 00:22 | red (tape) | owner computers have small evidence tables | **ON NOW** |
| `app_packages.objects`, `files.object` | off | package-declared kinds / file objects via the owner's route | b@c since 10-25 00:22; `/api/file-objects` 200 | orange | none | **ON NOW** |
| `texture.hold_trajectory` | off | Texture holds its trajectory across wakes | b@c 10-25 08:0x; d@e | red | none | **ON NOW** (first owner-class run is itself the paper's precondition) |
| `packages.publish`, `packages.share` | off | owner may publish package releases to the Library at owner scope; share/withdraw/reinstate | S1 proofs on d@e (list empty, id 404, ledger 05:0x); S2 review 274 approve; S3 re-review 293 PASS; **no live publish yet** | orange / red (cross-owner visibility) | train 10 `99524f638` deployed and the computer booted onto it; publish is the owner's act | **ON AFTER** train 10 deploys and the Reader's first release publishes on b@c (see section 3) |
| `texture.root_desk` | off | master document of open work | PD3 S1 accepted; S2 F4 failed | red | F4 fix proved | **ON AFTER** F4 fix |
| `research.run_memory_elision`, `research.search_caps` | removed | research context economy | | | | **REMOVED 2026-10-26 by owner ruling** (docs/problems/research-economy-switches-removed-2026-10-26.md): the setting, env twins and code are deleted; stored rows stay on the tape, unread. |
| `intake.*`, `research.system1`, `research.related_story`, `calibration.feedback`, `management.serial_tasks`, `telemetry.batch`, `store.event_table`, `store.trace_log`, `intake.event_wakes` | off | paper intake / retired / rollback | n/a | n/a | n/a | **LEAVE OFF** (intake and related_story are paper features; related_story's route also refuses non-platform-tier computers) |
| `store.debounced_checkpoint` | off | as paper | none yet | red | kill -9 drill | **ON AFTER** the drill (resource win) |

## 3. Test computers

b@c (computer-6c50d6e5…, apparently; it is the delegate and visual-capture computer in nix): ON per the register SW-01 plus ledger: `source.projection`, `engineering.progress_to_texture`, `research.run_memory_elision` and `research.search_caps` (stored true on b@c; both removed 2026-10-26, rows unread), `app_packages.objects`, `evidence.graph`, `engineering.slots_graph`, `files.object`, `engineering.carry_unfinished`, `texture.owner_directions`, `texture.evidence_embeds`, `texture.hold_trajectory`, `texture.root_desk`, `texture.owner_act_wakes` (10-26 04:47:34), `intake.labels`, `intake.events`, `research.system1` (from 10-24 20:50). Verify with GET; I did not read it.

| Switch | Computer | Recommendation |
|---|---|---|
| `packages.publish`, `packages.share` | b@c | **ON AFTER** `99524f638` is deployed and b@c has booted it; then publish the Reader as the first real release and share it to d@e. This is the Library plan's own order (overnight report) and the only unproved built-off feature at the front of the queue. |
| `packages.publish`, `packages.share` | d@e | **ON AFTER** b@c's publish works (d@e is the sharing target; `packages.share` is the gate on the sharing computer, so b@c, not d@e, needs it). |
| `evidence.graph`, `files.object` | d@e | **ON NOW** to match b@c (d@e is currently the Themes computer; low risk, orange / red-tape for evidence.graph with an empty table). `research.run_memory_elision` and `research.search_caps` are removed 2026-10-26, nothing to flip. |
| `intake.labels`, `intake.events`, `research.system1` | d@e | **LEAVE OFF** (test computer has no feeds). |
| `research.related_story` | any test | **LEAVE OFF** (route refuses non-platform-tier computers; no effect). |
| `store.debounced_checkpoint` | a throwaway test computer, not b@c/d@e (both carry live rounds) | **ON NOW** *as the proof step*: set it, run the kill -9 drill (deploy R's procedure), then widen. The drill is the missing proof. |
| `store.event_table` | test | **LEAVE OFF** (lease/grant/R7). |
| `intake.event_wakes` | test | **LEAVE OFF** (bar failed). |
| `telemetry.batch`, `calibration.feedback`, `management.serial_tasks`, `store.trace_log` | all | **LEAVE OFF** (superseded / rollback / no reader). |

Other test computers (9648031f…, 71afbf14…, 7011602c…): values unknown; set the self-development block only on the ones that actually run self-development.

## 4. Platform / host env (nix, vmctl, gateway, corpusd, guest image)

Current values are from `origin/main`. "Wave" refers to the 10-24 inventory.

| Switch | Where | Default / now | What it does | Proof | Class | Recommendation |
|---|---|---|---|---|---|---|
| `CHOIR_VISUAL_CAPTURE_COMPUTERS` | node-b.nix:1769 | two computers (7011602c…, 6c50d6e5…) | right to launch Chromium in a capsule capture job | b@c rounds embed captures; the 7011602c drill result never found (register SW-02d) | orange | **ON NOW**: add the Themes/engineering test computer (the one with delegate id ad1b46d4…, apparently d@e) so verify can see; "propose needs visual evidence". Host edit + deploy; computer picks it up at next cold boot. |
| `VM_BALLOON_STATS_ONLY` | node-b.nix:1667 | one computer | balloon statistics device (RES-10) | deploy L; probe `scripts/res10_balloon_stats_probe.sh` | orange (adds a device; one cold boot) | **ON AFTER** the probe on `class:test`; then `*`. |
| `VM_SNAPSHOT_TYPE` / `VM_SNAPSHOT_DIFF_ONLY` | node-b.nix:1749-1750 | `full` / empty | diff snapshots (only resident pages) | deploy Q, four accepts incl. 12 mutants; no live compare | red (snapshot format) | **ON AFTER** hibernate/resume/size compare on one test computer. Disk win. |
| `VM_CAPSULE_TUNNEL_ENFORCE_ONLY` + image `CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY` | node-b.nix:1699; autoputer-vm.nix | one computer enforce; image `observe` | refuse CONNECT whose SNI differs | probe PASS (ledger 16:38 10-23) on 9648031f | red | **ON AFTER** a Claude-mode capsule's `api.anthropic.com` pass is recorded; then widen to `class:test` and flip the image line. |
| `VMCTL_GUEST_DNS_ONLY` | node-b.nix:1563 | `class:test,class:primary` | guest resolves through host Unbound | D12/D19 | red | **ON AFTER** a primary computer is probed; paper and public_platform last (cold boot once). |
| `VMCTL_JAILER_ONLY` | node-b.nix:1609 | `class:test,class:primary,class:premium_always_on` | Firecracker under the jailer | D32, D42, D44 | red | **ON AFTER** D43 readings; paper/public_platform last, alone, at a watched planned refresh. |
| `gatewayModelScheduler` | node-b.nix:553 | `off` | fair queue for model calls | built (ec25d61ff, 72caf899d); proof 9 never run | red | **ON AFTER** proof 9 (40 concurrent calls from one test computer, a second computer's latency stays near one call). |
| `hostFetchVMs` (`CHOIR_HOST_FETCH_VMS`) | node-b.nix:28 | empty | all guest fetches via the host; closes direct egress | deploy N | red | **ON AFTER** `net01-host-fetch.sh` on one test computer. High security value. |
| `CHOIR_CAPSULE_MIRROR_COMPUTERS/_ECOSYSTEMS/_MIRRORS_ONLY` | node-b.nix:1789-1791 | empty | engineering capsules fetch packages from the host mirror | deploy G inert | red | **ON AFTER** one test computer builds through it; then `MIRRORS_ONLY=1`. |
| `issuerAuthMode` | node-b.nix:115 | `observe` | issuer token required for credential issuance | none | red (wrong flip stops boots) | **ON AFTER** a full day of journal with no `hostauth: observe: would refuse ... issuer:` including a paper boot. |
| `platformDoltSquashTimerEnabled` | node-b.nix:47 | `false` | weekly Store A squash | never run | red/black | **ON AFTER** the supervised first run in a window (owner already decided "one squash then weekly"). 24 GB resource win. |
| `VMCTL_STORE_COMPACT_PRESSURE` | node-b.nix:1855 | `observe` | compaction under disk pressure | none | red | **LEAVE OFF** until a rehearsed pressure trigger on a test image. |
| `VM_GUEST_RECLAIM_ONLY` | node-b.nix:1684 | two computers | DAMON reclaim boot args | no control reading | orange | **LEAVE** until a control host-RSS reading exists. |
| `CHOIR_DOCUMENT_TOOL_SANDBOX` / `CHOIR_RUNTIME_CAPABILITY_FLOOR` / `CHOIR_CAPSULE_MEMORY_POLICY` | autoputer-vm.nix:960-984 | `observe` each | doc-tool sandbox; capability floor; capsule memory doctrine | observe counters; CAP_SYS_TIME fix f3cfd4e1f reported; a release build exceeded 2 GiB pre-bounds | red | doctool: **ON AFTER** observe counters read (shadow vs real) on a test computer; cap floor: **ON AFTER** confirming the CAP_SYS_TIME repair and one test computer through a capsule and every desk under enforce; memory policy: **ON AFTER** a release-build re-measure under `doctrine`. |
| `CHOIR_OG_WRITER_GATE` | env unset | observe | ARCH-04 object-graph writer gate | counts served, no SSH reader | red | **ON AFTER** its three preconditions (counts reader, story-seeding declaration, install kinds); Texture first; paper last. |
| `CI_REUSE_GREEN_PR_RUN` | ci.yml:36 | `'false'` | CI prove-once: skip Go lanes already green on a PR run | wired, never used | red-light (CI) | **ON AFTER** one green draft PR run. |
| `PROXY_EXPAND_REQUESTERS` | node-b.nix:993 | empty | private-door story expansion; public door already `on` | n/a | orange | **ON NOW** (add owner id) or delete the key; no user-visible change either way. |
| `CHOIR_CLAUDE_DELEGATE_COMPUTERS` (+EXPIRES) | node-b.nix:1813-1814 | two computers, expires 2026-11-03 | claude-mode capsule on credential disk | scoped by owner | red | **LEAVE** (renew expiry 11-03 by edit; not a flip). |
| `CORPUSD_RELATED_STORY` | host env file | unknown | see section 1 | none | orange | **ON AFTER** audit read. |
| `CHOIR_PACKAGE_SHARE_READS` | corpusd env, unset | `scoped` | each listing's own scope; `owner` is the rollback | S3 as-built | red | **LEAVE** (normal state). |
| `CHOIR_RELEASE_READS` | corpusd env, unset | `owner` | release reads owner-scoped; `open` would widen | S1 proofs on d@e | red | **LEAVE** (normal state). |
| `CHOIR_OWNER_ACT_MODE` | node-b.nix:1230 (`ownerActMode`) | `observe` | corpusd verifies owner-act signature | owner ruling 10-24 17:5x | red | **LEAVE** by owner ruling. |
| `VMCTL_RETENTION_*_MODE`, `VM_DATA_DISCARD`, `VM_BALLOON_FREE_PAGE_REPORTING`, `VM_HIBERNATE_MODE`, `GATEWAY_BODY_CAPS`, `PLATFORM_TAPE_SEGMENTS`, `PLATFORM_*_GC/SWEEP`, `CORPUSD_OPINION_LISTING`, `MAILD_DIGEST`/`SEND_GATE`, `*_AUTH_MODE` enforce, `offHostBackup`, `VMCTL_DATA_IMAGE_TEMPLATE`, `CHOIR_DESK_CELL_CONFINEMENT`, `CHOIR_CAPSULE_TEARDOWN_RECLAIM` (code default on) | nix | on | n/a | see 10-24 inventory D/B rows | n/a | already on (updated: retention test-idle and quarantine are now `active`, body caps `enforce`, `VM_MEMORY_HOTPLUG=off`). |
| `VM_MEMORY_HOTPLUG`, `VM_SNAPSHOT_VAULT_*`, `VM_FIRECRACKER_NEXT_*`, `CHOIR_KEY_ESCROW_SEAL`, `GATEWAY_ANTHROPIC_RELAY_*`, `AUTH_REGISTRATION_CAP`, `MAILD_TRUSTED_AUTHSERV_IDS`, `CHOIR_APP_DESKS`/`_VERBS`/`_NETWORK`, `CHOIR_SEARCH_*` alternatives, `CHOIR_YOUTUBE_TRANSCRIPT_*`, `FRONT_IMPORTANCE_DIRECT`, `VMCTL_VMM_SPECULATION_BARRIERS`, env twins `CHOIR_TELEMETRY_BATCH` / `CHOIR_GUEST_CHECKPOINT` | various | off | owner acts, unwired scaffolding, dead, or alternatives | n/a | n/a | **LEAVE OFF** (reasons unchanged from 10-24 inventory A-E and the register SW-03). |

## 5. Settings whose default should flip to true in code

Do this only after the substrate change in section 0 item 2 (one fallback-to-default read, red, tier 2: it touches every settings reader). Candidates, ordered by readiness. A flip is deploy-wide, so each should first be ON for the owner's computers (section 2) for a few days, with the paper explicitly stored `false` where listed as paper-meaningless-but-harmless-off.

1. `texture.owner_directions` (orange; byte-identical when off; on at b@c and d@e)
2. `engineering.progress_to_texture`
3. `engineering.carry_unfinished` (after the pruning nit F7, bunch230, is scheduled)
4. `texture.owner_act_wakes`
5. `engineering.slots_graph` and `evidence.graph` (the graph homes are the doctrine's single-home direction; table path becomes the rollback)
6. `app_packages.objects`, `files.object` (follow the app-package replan)
7. `texture.evidence_embeds`, `source.projection`, `texture.hold_trajectory` (red; flip after owner computers have run them)
8. `packages.publish`, `packages.share` (after one real release and a cross-owner share has been proved)

Not to flip: `intake.*`, `research.system1`, `research.related_story` (paper-specific; the route already refuses non-platform tiers), `store.*` (red; drill and R7 first), `texture.root_desk` (F4), `management.serial_tasks`, `calibration.feedback`, `telemetry.batch` (superseded or rollback). When all of 1-8 are default-true, retire the per-computer rows rather than keeping a per-feature key forever (the 10-24 inventory's direction for `intake.labels`).

## 6. What changed since the 10-24 inventory

- Settings: 9 -> 27 keys (+18 listed above: the 8 self-development, 3 Texture-doc, 2 graph-home, 2 package, 2 store, 1 research.related_story).
- Done since: body caps `enforce`; retention modes `active`; hotplug `off`; jailer wave 2a (premium_always_on); GUEST_DNS to `class:primary`; b@c's whole set on (SW-01).
- Still stale in the older doc: "PUT /api/computer-settings" (it is POST), "nine settings", and B8 (already off).
- New since: `packages.publish`, `packages.share`, `CHOIR_RELEASE_READS`, `CHOIR_PACKAGE_SHARE_READS`, `CI_REUSE_GREEN_PR_RUN`, `CORPUSD_RELATED_STORY`.
