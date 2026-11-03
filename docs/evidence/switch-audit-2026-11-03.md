# Switch audit and decision sheet (2026-11-03)

Status: green (evidence). Goal station S6b of
[`choir-selfdev-apps-feature-complete-2026-11-03.md`](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).
Owner, 2026-11-03: "i dont want anything to stay off. i want to switch
features on or delete them"; then "audit the switches before going ham ... i
dont even know what we have. and features may conflict." Nothing below has
been switched. Part A is the decision sheet; Parts B and C are the two
read-only audits (guest settings and Go gates; host switches), each opening
with a plain catalog and closing with its conflicts section. The earlier
inventory is [`switched-off-inventory-2026-11-03.md`](switched-off-inventory-2026-11-03.md).

## Owner rulings (2026-11-03 01:5x), which govern Part A

No switches at all: every row below ends deleted, with its code either on
everywhere or gone, walked with the owner group by group. Named rows: gateway
model scheduler deleted; host fetch and capsule mirrors made operational and
on everywhere; story expansion for every signed-in user, the requester list
deleted; `texture.hold_trajectory` and `texture.root_desk` stay; calibration
feedback on, engineering first. Rollout is ring-ordered deploys with
automatic rollback; the paper last and automatic
(design/progressive-delivery-2026-10-26.md, "Owner rulings (2026-11-03)").

## Part A. Decision sheet

**Forcing date.** On 2026-11-04 `check-switch-registry` starts failing CI
for four host rows still off everywhere (`host_fetch_vms`,
`choir_capsule_mirrors`, `proxy_expand_requesters`,
`gateway_model_scheduler`): each must be on at d@e at least, or deleted.

**1. Self-development needs these (decide first).**

| Switch | Proposal | Why / what must happen first |
|---|---|---|
| `files.object` | on, fleet by ring | Files reinvention builds on it; the writer at `file_object.go:373` ignored the default and must be fixed in the same change |
| `app_packages.objects` | on | carries package kinds (decks, events) |
| `engineering.slots_graph` | on | proved on b@c and d@e |
| `engineering.progress_to_texture` | on | proved |
| `engineering.carry_unfinished` | on | saved patches are never pruned (about 2 MiB a round): add pruning |
| `calibration.feedback` | owner rule needed | registry says stays off; a 10-22 owner line says lift it |

**2. On now, low risk.** `texture.owner_directions`, `texture.owner_act_wakes`
(high confidence, proved on b@c and d@e); `packages.publish` (b@c first).

**3. On after a named fix.**

| Switch | Fix first |
|---|---|
| `evidence.graph` | the first write imports the whole old table inline under the slot lock; bound and background it |
| `tape.group_commit`, `store.telemetry_partitions` | d@e, then the paper; partitions write store floor 5 at boot, which blocks rolling back to an older binary |
| `texture.evidence_embeds` | known data-loss risk RV-04 on old clients; tied to the capture list being deleted |
| `dispatch.wide_slots` | owner asked for it on the paper; conflicts with the gateway scheduler's 6 calls in flight per computer against the paper's 8 to 12 slots |
| `gateway_model_scheduler` | raise or class the per-computer cap first, then proof 9 (`scripts/proofs/fp7-gateway.sh` step_9) on d@e |
| `host_fetch_vms`, `choir_capsule_mirrors` | never proven; host fetch needs a `*` token; mirror cache caps total 32 GiB of host disk |
| `VM_CHECKPOINT_CADENCE_ONLY` | test class now; the owner's computer and the paper wait for the append-hold fix (630 ms to 1.13 s per bind on d@e) |
| tunnel identity enforce | Claude-mode capsules stay in observe until a real session shows a TLS pass (`internal/capsule/executor.go` ~213) |

**4. Owner decides.**
- `texture.hold_trajectory`: turn on, or delete it to finish the 2026-11-01
  "start from system prompt and REPL only" ruling (the prior-run seed itself
  is already deleted on main, dd43c84b9 and a832d94db).
- `texture.root_desk`, `source.projection`, `source.rebase_offer`: one
  chain; on together after the U4 rebase runs once on d@e, or delete all
  three (rebase rounds failed on b@c 10-27; root desk failed its paired test
  twice 10-25).
- The Claude delegate: on for the two arms that use it; the owner turned it
  off on 10-15 for subscription spend.
- `appdesk`, `appnet`, `egressgrant`: unwired; doctrine I26 says app desks
  ship with packages, so wire (not a small slice) rather than delete.

**Correction (2026-11-03 01:4x, after a docs review the owner asked for).**
The chain in group 4 was framed wrongly. (a) The b@c rebase deadlock was
solved on 2026-10-27: patch-identity containment (lane/patchid-contained,
reviews 353 and 355, train 34 1ebe74f85) retires a stack already in the base
as `upstreamed`, and b@c's next citation round was admitted through the cast
gate on train 34 (consolidate-and-scale ledger, 2026-10-27 11:3x to 15:0x).
What has not run is a rebase of a change that is *not* already upstream.
(b) The Library design (`design/library-package-propagation-2026-10-26.md`
section 2 and 10.3) makes package changes avoid rebase: a package re-
materializes onto a moved base, and self-development on other paths stays
open; its package-aware containment (S6) is not built. (c) Owner,
2026-11-03: `texture.root_desk` "is important for other reasons, to have
system wide understanding for each user"; it is not a rebase accessory. So:
root desk ON on its own merit (its F4/F5 excerpt-order failures re-proved
first); projection ON with root desk; `source.rebase_offer` ON once one
non-upstreamed rebase is proved on d@e, and package changes move to the
Library's S6 path. `texture.hold_trajectory` stays on (owner).

**5. Delete.** `intake.event_wakes` and its wait path (failed its false-merge
bar three times); `store.event_table` setting and writer arming (the table
stays); `management.serial_tasks`; `store.compact_force`; retired
`store.debounced_checkpoint` and `store.trace_log`; env twins
`CHOIR_TELEMETRY_BATCH` and `CHOIR_SOURCE_PROJECTION`; DAMON
(`VM_GUEST_RECLAIM_ONLY`, measured negative); `VMCTL_STOP_MANAGED_ON_EXIT`;
`VMCTL_PRIMARY_KEEPALIVE_MODE`; the docs-only names `CHOIR_COMPUTE_CAPSULES`,
`CHOIR_EGRESS_GRANTS`, `AUTH_DEVICE_RELEASE`.

**Cold boots.** Kernel-argument selectors and credential-disk grants take
effect at a computer's next cold boot; batch them into one deploy so each
computer pays one cold boot.


## Part B. Guest settings and Go gates

### Audit of guest computer settings and Go gates (read-only), 2026-11-02

Scope: `KnownComputerSettings` (internal/store/computer_setting.go:250, plus init() additions at world_event.go:41-42, file_object.go:35, app_package_objects.go:32) and Go/env gates. Read at origin/main 5cb70f9c4. Nothing was changed for this audit. FACT = read in code or a dated document in this repo. HYPOTHESIS = inferred, not verified. Per-computer stored values are not visible from the repo; every "on at X" below is a ledger or registry claim (FACT that it is claimed, HYPOTHESIS that it still holds) until `GET /api/computer-settings` is read on that computer.

Rings: R0 = d@e (computer-59c902c5...), b@c = computer-6c50d6e5... (owner's test/delegate computer), paper = R5 (computer-1355b83b...).

### 0. What exists in the worktree (so you know)

- Commits made by me: NONE. `git log origin/main..HEAD` is empty.
- Uncommitted edits in /Users/wiz/Dev/go-choir-switches-settings (branch switches-on-settings), made before the stop message, NOT pushed, NOT committed: (a) flipped 23 defaults to true in computer_setting.go, world_event.go (intake.events), file_object.go, app_package_objects.go; (b) file_object.go writer now honours the map default; (c) graph_home.go: CreateEvidence no longer imports inline, and ImportEvidenceToGraph was half-rewritten (the page helper body was NOT finished, so that file does not compile). Treat all of it as discardable: `git -C /Users/wiz/Dev/go-choir-switches-settings checkout -- .` restores origin/main. I did not discard it because you said to leave the branch alone.
- Fact that changes the task: the prior-run desk seed is ALREADY deleted on main (commits dd43c84b9, a832d94db; run_memory.go:120-146 says "Every other desk starts from its system prompt and REPL only (owner, 2026-11-01)"). What remains is Texture's per-document conversation carry (`carryActorConversation`, run_memory.go:157), which is exactly what `texture.hold_trajectory` turns on (texture_agent_revision.go:682-684). See conflict C1.

### 1. Plain-language catalog (one sentence each)

Texture (the owner's document desk)
- texture.owner_directions: when Texture revises a document, it sees every later instruction the owner gave, newest-ranked above the original request, instead of only the latest revision.
- texture.hold_trajectory: a new Texture run continues the previous run's conversation word for word instead of starting from a summary digest.
- texture.owner_act_wakes: when the owner approves, rejects or applies a self-development candidate, the document's Texture is woken with that act as the reason.
- texture.evidence_embeds: when a candidate is frozen with screenshots, the screenshots are embedded in the Texture document.
- texture.root_desk: keeps one "Master document" summarising all open work, written by its own Texture desk and refreshed (rate-limited) when other documents change.
- calibration.feedback: shows desks in the "digest" arm of the calibration experiment their own past confidence-vs-outcome record as context, so they can calibrate.

Engineering / self-development
- engineering.progress_to_texture: engineering progress reports also land in the owner's Texture inbox, not only in management's.
- engineering.carry_unfinished: when an engineering round is stopped before finishing, its partial patch is saved and offered (labelled unverified) to the next round.
- engineering.slots_graph: engineering's "slot held by run X" claims are stored on the tape instead of the SQL table co_super_slots.
- source.projection: keeps a git copy of the computer's source in step with its effective release so capsules start from the right tree and casts are checked against it.
- source.rebase_offer: when platform deploys move the base under an applied self-dev change, asks the root Texture to have engineering rebase it.
- dispatch.wide_slots: the shared concurrent-activation cap goes from 8 to 12.

Store / tape
- telemetry.batch (already default on): high-volume trace events are written in sealed batches instead of one tape write each.
- tape.group_commit: ordinary tape writes that arrive together share one tape event; telemetry and run-memory messages ride along (crash may lose the newest run messages).
- store.telemetry_partitions: tool-result telemetry goes into per-day partition files so retiring a day is deleting a file; writes store schema floor 5.
- store.telemetry_retire (already default on): a daily job retires aged telemetry rows from the working store.
- store.event_table: puts the ten batched telemetry kinds in their own SQL table (choir_events); needs a host grant too; writes store floor 2.
- evidence.graph: research evidence is stored as tape objects instead of the agent_evidence table.
- store.compact_force: a proof lever that makes the daily compact copy due regardless of file size.
- store.debounced_checkpoint, store.trace_log: retired keys; nothing reads them.

Packages / files
- app_packages.objects: opens the owner's HTTP route to objects of package-declared kinds.
- files.object: Files API and the reconcile pass record files as tape objects (choir.file, choir.file_path).
- packages.publish: the owner can publish this computer's applied package change as a release to the Library at owner scope.
- packages.share: the owner can share, withdraw and reinstate package releases.

Sources / Autopaper
- intake.events: new captures from source polls are grouped onto world events by Jev (shadow; nothing reads them yet).
- intake.event_wakes: production waits for events and management wakes per event (failed its false-merge bar three times).
- intake.labels: Jev labels arriving captures and management's prompt mentions them.
- research.system1: the research desk may call System 1 (a cheaper reading model) with default caps.
- research.related_story: research may record judged story-pair relations and export them to Store B.
- management.serial_tasks: a rollback lever that holds management to one task at a time.

### 2. Reader audit: does each reader honour the map default?

FACT unless noted. Generic reader `computerSettingOn` (agentcore/computer_settings.go:36) returns `KnownComputerSettings[key]` on ErrNotFound. BUT it returns false when `CHOIR_OWNER_ID` or computer id is empty (line 37-38); boot-time callers take the owner from `os.Getenv("CHOIR_OWNER_ID")` (telemetry_partitions.go:25, tape_group_commit.go:23, source_projection.go:38, dispatch_slots.go:27). HYPOTHESIS: the guest has that env set (it works for telemetry_retire today); if empty, every boot-time default would read as off.

| Reader | Honours default? |
|---|---|
| agentcore computerSettingOn (all agentcore readers) | yes |
| store.graphHomeOn (graph_home.go:41) used by evidence.graph, slots_graph, telemetry_retire, compact_force | yes, but returns false for an unbound store (test seam) |
| textureowner textureSettingOn (texture_owner_directions.go:122-125) | yes |
| autoputer file_objects.go:126, app_package_objects.go:101 | yes |
| store/file_object.go:373 ApplyFileChanges (the only writer) | NO: ErrNotFound => off. A flipped default would still refuse every write (the "gate at the only writer"). Needs the fix I drafted. |
| agentcore/event_wakes.go:349 | reads the stored value after the generic check passed; with default on and nothing stored it returns an error and logs "read ...: not found", mints nothing. Moot if event_wakes is deleted. |
| sourceproj.Enabled (sourceproj.go:40) | uses a process atomic set by the runtime at boot/POST from computerSettingOn, so it honours the default only via configureSourceProjectionSetting (runtime.go:1053). yes |
| Boot "apply" hooks (telemetry_partitions, tape_group_commit, dispatch slots, source projection) | call computerSettingOn at boot, so a default flips them at next boot. yes |
| api POST /api/computer-settings same-value branch (computer_settings.go:109) | compares only to a STORED value; a POST of `true` when nothing is stored and the default is true writes it and applies (fine). |

### 3. Per-switch audit

Format: (1) what/where; (2) proof; (3) effect of ON on the paper and the owner computer; (4) failure mode and whether silent/irreversible; (5) cost of deleting; (6) recommendation; (7) constraints. Confidence in brackets.

### texture.owner_directions
1. FACT: textureowner/texture_owner_directions.go:113-125; reads owner prompts along the head's parent chain at each Texture run start.
2. FACT: on at b@c from 2026-10-25 04:4x (ledger selfdev-readiness 04:3x-04:4x), proved b@c and d@e (registry note "Proved on b@c and d@e; byte-identical when off"). Paper: not on (HYPOTHESIS).
3. No migration, no boot work. Texture prompts get longer (bounded by owner revisions; HYPOTHESIS about the bound). No new writes.
4. Wrong = Texture over-weights stale owner directions; visible in documents, reversible, not silent in the sense that the document shows it.
5. Delete = keep the code unconditionally (small). 
6. ON for all (fleet). Confidence high.
7. None.

### texture.hold_trajectory
1. FACT: textureowner/texture_agent_revision.go:540 sets `actor_rewarm_carry_conversation`; agentcore/run_memory.go:140-143 `carryActorConversation` continues the earlier run's checkpoint and messages (reason "actor_rewarm").
2. FACT: on at b@c 2026-10-25 08:16:18 (ledger); review 240 ACCEPT; named residuals SD-27 F1-F3 open (partial carry sticky on an append error; seed-less first revision labelled "owner's request" even when drafted prose; carried request feeds hard-requirement hints). Staging proof "pending" at that entry (FACT); no later proof found (HYPOTHESIS none exists). Not on d@e per registry ring "R0,ids:1" (the claim is R0 yes).
3. Every Texture run carries prior conversation: more input tokens per Texture run (HYPOTHESIS larger spend, bounded by the run's own compaction per the design). On the paper Texture runs are frequent; this is the biggest cost lever in the Texture group.
4. Wrong = Texture answers from stale verbatim context, or a hole in the run log silently carries; partly silent.
5. Delete = remove the carry; the non-carry path (digest rewarm) is already the default. Because the owner said on 2026-11-01 "each desk should start with system prompt and repl only", deleting hold_trajectory (and Texture's carry) is the consistent reading. The fallback `actor_rewarm_source_loop_id` metadata (line 679) and `LatestTextureActorMemorySourceRun` are the "digest" path.
6. DECIDE WITH THE OWNER (conflict C1). If the 11-01 ruling applies to Texture: DELETE. If Texture is the exception ("Texture's per-document carry stays", a832d94db wording): ON. Confidence medium; this is the one the ruling most likely contradicts.
7. Conflict C1.

### texture.owner_act_wakes
1. FACT: agentcore/selfdev_owner_act_wake.go:115; mints an owner-act record into the Texture inbox on approve/reject/apply.
2. FACT: on b@c, proved 10-26 (inventory; ledger 10-27 00:4x mentions owner acts never woke before). d@e: registry says R0,ids:1.
3. New writes: one inbox record per owner act; each wakes Texture (a model run). Owner acts are rare. Paper has no owner revisions/candidates (HYPOTHESIS), so no change.
4. Wrong = spurious Texture wakes (cost), loop with progress_to_texture; visible. Reversible.
5. Delete = keep code unconditionally.
6. ON fleet. Confidence high.
7. Overlaps progress_to_texture (C4).

### texture.evidence_embeds
1. FACT: agentcore/texture_evidence_embeds.go:84; Freeze embeds captured screenshots in the operation's Texture document.
2. FACT: on b@c 2026-10-25 05:1x. Open residual RV-04 (residual-register): version gate at enable time not implemented; old cached frontend renders nothing and its serializer drops the embed on the next owner save; old binary's desk Write drops embeds. 
3. Only matters when a freeze has captures; captures need host grant CHOIR_VISUAL_CAPTURE_COMPUTERS (nix; other builder). So default-on is a no-op on computers without the grant (FACT: inventory B1).
4. Wrong = embeds lost silently on an old client's save (silent data loss of the embed block, RV-04).
5. Delete = lose screenshots in documents; moderate code.
6. ON fleet after RV-04's version gate or accept the known loss. Confidence medium.
7. Needs the host visual-capture grant (host side).

### texture.root_desk
1. FACT: textureowner/texture_root.go:124,206,222,331,432; creates one Master document and its own desk texture:<master>; digest on other documents' commits.
2. FACT: PD3 S1 accepted on b@c 2026-10-25 11:32 (ledger 74-75). S2 paired test FAILED twice (F4 excerpt order 12:3x, F5 13:1x). Later status not found in the ledger (HYPOTHESIS: fix status unknown). Registry: "LEAVE OFF on the paper (hundreds of story documents would churn the master)".
3. On the paper: each story-doc commit arms a digest. Bounded: at most 50 documents and 32 KiB per root run, master capped 48 KiB (texture_root.go:44-58); a rate limit exists (textureRootNextSlot, interval and per-hour) but I did not read the numbers. A new Master document and a new Texture desk appear on first enable (new writes). Model spend per root run.
4. Wrong = master doc is stale or misleading (F4/F5 symptom), churn cost; not irreversible.
5. Delete = ~760 + 160 lines in textureowner, store/texture_root.go, wiring in handler.go, texture_controller.go, actorruntime, prompts; AND source.rebase_offer cannot work (rebase_notice.go:100 requires it).
6. HYPOTHESIS that it is correct. Recommend ON for owner computers only, not the paper, until F4/F5 is re-proved; or DELETE together with rebase_offer if the owner chooses. Confidence low.
7. Hard dependency of source.rebase_offer.

### calibration.feedback
1. FACT: agentcore/calibration_feedback.go:286-288, 342; digest arm runs (hash-chosen via `calibrationArmOfRun`) read their own settled calibration as a user turn and in choir.Pack(). API shows the setting: calibration_api.go:83.
2. FACT: never on anywhere. Registry note "Owner 2026-10-24: stays off; the ICRL design replaces it." Inventory cites an owner in-session line "we lift calibration feedback stays off" (ledger 1588) that supersedes it, and docs/design/calibration-seed-2026-11-03.md says "automatic pack injection stays off until held-out evidence". HYPOTHESIS the two conflict.
3. Digest-arm runs get extra context (small token cost, HYPOTHESIS); the snapshot rebuild is one at a time per runtime (calibration_feedback.go:~300). Control arm unchanged. No migration.
4. Wrong = it contaminates the experiment (feedback to desks makes held-out evidence unclean); silent in effect, reversible by turning off but past runs stay contaminated.
5. Delete = ICRL digest injection only; the arm/observation machinery remains.
6. ON (owner asked) at R0 first; confidence medium because the design doc says evidence first.
7. Independent of other settings.

### engineering.progress_to_texture
1. FACT: agentcore/engineering_progress_relay.go:42.
2. FACT: on b@c; bunch 210 APPROVE; "ON NOW" registry.
3. One extra inbox record per engineering report; may wake Texture (model run).
4. Wrong = Texture noise; reversible.
5. Delete = keep code.
6. ON fleet. Confidence high.
7. C4 with owner_act_wakes.

### engineering.carry_unfinished
1. FACT: agentcore/unfinished_patch.go:84,158; patch snapshot taken from the capsule just before revoke; offered next round.
2. FACT: on b@c 2026-10-25 04:4x; bunch 230/234 approved. Known nit bunch230-F7: keeps never pruned, about 2 MiB per round (FACT, registry note).
3. New writes: a patch file per stopped round on guest disk, unbounded accumulation (no pruning). Resource rule says unbounded disk growth is the wrong direction.
4. Wrong = slow disk growth, silent. Not irreversible.
5. Delete = lose cross-round partial carry; moderate.
6. ON after a pruning fix (one small slice) or ON now with the accepted nit. Confidence medium.
7. Overlaps the earlier "rejected candidate carried into the next round" mechanism (C6).

### engineering.slots_graph
1. FACT: store/graph_home.go:253-330, store.go:772; claims stored as choir.co_super_slot objects.
2. FACT: on b@c since 2026-10-25 00:22; bunch 218 both seats approve. Residual bunch218-F4 (delete with RowsAffected==0 silent) nit.
3. First enable: the first claim imports co_super_slots rows under graphHomeMu (store.go:~790 claim path; small table, HYPOTHESIS tiny). New tape writes per claim.
4. Wrong = double-claimed slot (fixed in 44387d4bd per register) or a held slot after rollback; reads union both homes so a flip never hides a record (FACT comment graph_home.go:20).
5. Delete the switch = keep graph path only, drop the table path: the table remains needed to read old rows until imported.
6. ON fleet. Confidence high.
7. Pairs with evidence.graph (same lock, same file) (C3).

### evidence.graph
1. FACT: store/graph_home.go:62-80, 130-210; computer_settings.go:188 import on POST.
2. FACT: on b@c; d@e ON NOW claim (inventory line 71, "with an empty table"). Paper-size import unmeasured (inventory).
3. FIRST-ENABLE: today the import runs inline in CreateEvidence (graph_home.go:70-75) under graphHomeMu for the whole table, with the request context, unbounded; the POST path bounds it to 25 s (computer_settings.go:174, 25*time.Second). With default on and no POST ever made, the FIRST evidence write on a computer with a big agent_evidence table performs the whole import inside that write. HYPOTHESIS: paper table is large (research evidence); this could stall a research tool call and hold the lock that slot claims also take. This is what I started to fix (move to background + lock per page).
4. Wrong = evidence rows go to the tape (the tape grows); a refused-for-size row stays in the table and the import errors each time. Not irreversible (reads union).
5. Delete the switch = keep graph home only; table code kept for reads and import.
6. ON after making the import bounded/backgrounded. Confidence medium-high.
7. Shares graphHomeMu with slots_graph (C3).

### source.projection
1. FACT: agentcore/source_projection.go:33-38, sourceproj/sourceproj.go:28-45; cast gate at source_projection.go:304-330 refuses ordinary casts while the applied stack sits on a moved base.
2. FACT: on b@c since 2026-10-24 21:26:22 (ledger W1 live). Proved B4/B5 per inventory. d@e: HYPOTHESIS not stored.
3. Boot: `go rt.startSourceProjection(ctx, "boot")` (runtime.go:1054), background. First enable fetches the booted base objects from the host (basefetch) and builds a git repo: disk plus network; needs host base fetch reachable. Residual RV-03: HTTP 403 treated as permanent failure, parking the fetch until next boot (open in the register 10-26; fix status unknown).
4. Wrong = ordinary self-dev casts REFUSED (FACT: selfdev-blocked-behind-unperformed-rebase-2026-10-27.md) on a computer with an applied stack after any deploy, until a rebase succeeds. Loud (refusals name the cause), reversible.
5. Delete = the whole projection (internal/sourceproj, source_projection.go, cast gate) and the capsule-source guarantees; large.
6. ON only together with source.rebase_offer + root_desk working; otherwise do not widen. At R0 first. Confidence medium-low (the third round on Contained was a patch-id fix, review 353; no later proof read).
7. Dependency D1: rebase_offer requires projection AND root_desk (rebase_notice.go:100-101).

### source.rebase_offer
1. FACT: agentcore/rebase_notice.go:100; asks the root Texture to cast a rebase through engineering.
2. FACT: registry "act-gated until the U4 rebase has run" (gate.act: "U4 stack rebase has run once on d@e"). b@c rebase rounds ran and FAILED 2026-10-27 (rebase_content_mismatch, then Freeze refused empty capsule); cause was Contained false negative; patch-id fix "as built" (review 353). No evidence that a rebase has succeeded afterwards (HYPOTHESIS).
3. On a computer with an applied stack: a notice per deploy, then a root Texture run, a management cast and an engineering round (model spend each deploy). Paper: HYPOTHESIS no applied stack, so none.
4. Wrong = deadlocked rebase rounds that burn engineering capsules per deploy (FACT happened on b@c).
5. Delete = rebase_notice.go 412 lines + cast-gate rebase branch + tests; the projection gate then has no escape when a base moves.
6. Not ready: run the designed U4 rebase once on d@e (the registry's own gate), then ON at R0. If the owner will not run it, DELETE with root_desk. Confidence medium.
7. D1.

### dispatch.wide_slots
1. FACT: agentcore/dispatch_slots.go:27,48; shared cap 8 -> 12.
2. FACT: owner on 2026-10-27 04:1x: "I think the paper can go to 12 slots... I have the tokens for it"; landed with train 31 (review 348). Whether it was ever switched on: no evidence (inventory says unknown).
3. More parallel model activations: higher provider spend/rate and CPU on every computer, not only the paper. Applies at boot (SetSharedSlotsActor).
4. Wrong = provider rate limits or guest memory pressure; reversible live.
5. Delete = trivial; but the owner asked for it for the paper.
6. ON for the paper (owner ask). For owner computers default 12 is a cost and memory choice: HYPOTHESIS they do not need it. Recommend ON at R5 only if per-computer; if the rule is "default on", set 12 as the default after checking guest memory. Confidence medium.
7. None.

### tape.group_commit
1. FACT: agentcore/tape_group_commit.go:23, planned_restart.go window stamp, store group commit (design tape-group-commit-2026-10-31).
2. FACT: landed abfab8754 (2026-10-31 20:07Z); on d@e by stored setting (ledger "Next: d@e setting on after deploy"); counters pending; no paper run.
3. Telemetry and run-memory `message` entries become crash-lossy within a 10 s window; a crash closes the run. Boot rule: a planned restart whose telemetry drain failed with the window open does NOT resume work (tape_group_commit.go:28-34).
4. Wrong = silent loss of the newest run messages after a crash (owner accepted, 2026-10-31); a hole in a run log is silent (the code damages-marks it).
5. Delete = keep direct path only; loses ~80% fewer tape events measurement (12,485 events/h on the paper, 10,288 acknowledged, ledger 2026-10-31 ~17:00).
6. ON, ring R0 -> paper after counters. Confidence medium-high.
7. Rides telemetry.batch's sealer (default on); requires projection tape bound.

### store.telemetry_partitions
1. FACT: store/telemetry_partition.go:836-870; agentcore/telemetry_partitions.go:25.
2. FACT: on d@e 2026-10-31 15:48:57, writing=true, first partition file seen (ledger 52). Soak since then not read.
3. Writes store schema FLOOR 5 on first enable (SetTelemetryPartitionWrites -> writeTelemetryPartitionFloorAt). Floor means no older binary may boot that store: rollback of the binary is blocked (FACT comment computer_setting.go:219-223: "below epoch 5 needs the floor lifted by hand once no partition holds a row"). New files per day on disk. Stamps only once the stream_seq lease is ready.
4. Wrong = replay/rollback incompatibility; irreversible-ish (floor), and a wrong partition is silent until retirement.
5. Delete = keep tool.result in og_objects; loses the delete-a-file retirement.
6. ON at R0 (done), widen to the paper after soak; default-on means every computer including the paper takes the floor at next boot. Confidence medium; this is the one I'd ask the owner about, because default-on skips the paper-after-d@e step.
7. Shares the lease gate and the Table-stamp mechanism with store.event_table (C2); needs telemetry.batch (HYPOTHESIS: partitions apply to batched kinds).

### store.event_table
1. FACT: agentcore/event_table.go; store/event_table.go:499-640; host grant CHOIR_EVENT_TABLE_COMPUTERS (nix, empty) read from the credential disk (eventtablegrant, vmmanager/event_table_grant.go).
2. FACT: off everywhere, never run on a live computer in the registry; proof scripts scripts/proofs/fp1-event-table-deploy1.sh/deploy2.sh exist. Archive design 2026-10-22.
3. Would write floor 2. Needs host grant too, so a guest-only flip does nothing (FACT event_table.go:60-70).
4. Wrong = rollback floor; irreversible-ish.
5. Delete = the guest half (setting, agentcore/event_table.go 162 lines, API refusal and observability fields) is small; the store writer + tests are ~700 + ~3000 test lines. The choir_events TABLE must stay: it is live SQLite schema (header/body split, header_body_split.go:5,863, telemetry partition rows use the same table) so readers and projector routing stay. Host half (nix, eventtablegrant, vmmanager) is the other builder's.
6. DELETE the switch and the writer arming; keep the table and readers. HYPOTHESIS redundant given batch + partitions + group_commit. Confidence medium-high.
7. C2.

### telemetry.batch (default already true)
1. FACT: agentcore/telemetry_batching.go:46-56. 2. FACT: d@e from 2026-10-30, three boots no sealer error; paper batching on 2026-10-31 14:48:34. 3. Already on. 4. Crash loses open window. 5. Env twin `CHOIR_TELEMETRY_BATCH` (store.TelemetryBatchConfigFromEnv) is now redundant: delete twin. 6. Keep ON; fix registry (stale ring R0). 7. Prereq for group_commit and partitions.

### store.telemetry_retire (default already true)
FACT: store/telemetry_retire.go:336. Ledger line 220 (data-system ledger, quoted in inventory): "Retirement stays off on the paper... owner act". HYPOTHESIS the paper may hold a stored false, so the "default true" claim and the paper state differ. Keep ON; verify the paper's stored value.

### store.compact_force
FACT: store/telemetry_retire.go:341, compact.go:254. A proof lever, not a feature. Cost of deleting: trivial; the planned-restart compact swap would be provable only by waiting for the 1 GiB layout trigger. Recommend DELETE as a setting (keep an explicit operator call only if the owner wants it). Confidence medium.

### management.serial_tasks
FACT: agentcore/management_tasks.go:49,169; K=1 rollback lever for management delegation S2b. DELETE (the setting only; K comes from runtime config). Confidence high. Cost: lose the live no-restart rollback to one task at a time.

### app_packages.objects
FACT: autoputer/app_package_objects.go:93-105 (route 404 when off). On b@c since 2026-10-25 00:22. Effect of ON: opens `/api/app-packages/{package}/objects/{kind}` for the owner; the reader honours default. Writes only on owner use. Silent failure: none visible. Delete = keep route unconditionally. ON fleet; confidence high. (Note the registry unwired_packages row lists internal/appdesk, appnet, egressgrant as having no importer outside tests; this route is not in that set, HYPOTHESIS.)

### files.object
FACT: store/file_object.go:373 (writer, ignores the default, see section 2), autoputer/file_objects.go:126. On b@c since 2026-10-25; ledger claim d@e ON. Effect of ON: the Files API and reconcile pass write choir.file/choir.file_path objects to the tape: reconcile on first enable walks the workspace (HYPOTHESIS: cost scales with file count; not measured), tape growth. Wrong = tape bloat, silent. Delete switch = keep object recording only. ON after the writer fix; confidence medium (reconcile cost on the paper unmeasured).

### packages.publish / packages.share
FACT: agentcore/api_package_release.go:44, 212. publish: guest drafts a release; share: guest answers the proxy's share gate. NEVER run live (inventory B; "no live publish yet"); S1 proofs on d@e (empty list), reviews 274, 293. Effect of ON: nothing happens until the owner publishes; share is cross-owner visibility (red). Wrong = a release visible to other owners that should not be (irreversible exposure). Registry gate: train 10 deployed, b@c booted, first Reader release published on b@c then shared to d@e: that act has not been recorded as done (HYPOTHESIS). Recommend ON publish at R0/b@c, share at R0 only after the first publish; do not default-on share before a real release is shared once. Confidence medium. Dependency: share needs a published release (C7).

### intake.events / intake.labels / research.system1 / research.related_story
FACT: on for the paper (registry R5, ledger table "ON ... Done" 10-26). Readers: event_layer.go:799, computer_settings.go:31, system1_verb.go:106, story_entities_verb.go:129. The 10-26 audit says "Not to flip: intake.*, research.system1, research.related_story (paper-specific; the route already refuses non-platform tiers)". related_story route refuses non-platform tiers by code (HYPOTHESIS from ledger text: "the route refuses non-platform-tier computers"), so default-on is a no-op on owner computers. Effect of ON on an owner computer: intake.events and labels run Jev (model calls per capture) on source polls; system1 allows System 1 calls with caps (spend). Owner computers with no sources: nothing runs (HYPOTHESIS). Recommendation: these are Autopaper features, not generic: ON means ON for the paper (already) and harmlessly default-on elsewhere; DELETE is wrong (paper uses them). Confidence medium. Read half CORPUSD_RELATED_STORY is host.

### intake.event_wakes
FACT: event_wakes.go (535 lines), tool_profiles.go:225, source_schedule.go:212, computer_settings.go:25. Failed its false-merge bar three times (7.0%, 9.5%, 15-30%). Requires intake.events. If it were turned on production would wait on events. DELETE the switch and the wait path (~535 + 658 test lines + two call sites); keep intake.events shadow resolution. Confidence high that it should not be on; medium on the delete cost (event layer shadow stays).

### 4. Go/env gates (not settings)

- CHOIR_SOURCE_PROJECTION env (sourceproj.go:28): a twin of source.projection that turns it on regardless; DELETE once the setting defaults on (redundant).
- CHOIR_TELEMETRY_BATCH env (telemetry_sealer.go:72, telemetry_batching.go:50,76): twin of telemetry.batch; delete after default on everywhere (it also forces on over the setting, so the owner cannot turn batching off if the env is set; HYPOTHESIS no unit sets it).
- CHOIR_OG_WRITER_GATE (objectgraph/writer.go:137): code default observe; registry exp 2026-11-25. Enforce on d@e is a decision for the objectgraph owner; not audited further (HYPOTHESIS).
- CHOIR_APP_DESKS (appdesk.go:328): unwired per registry (no importer outside tests). Turning it on does nothing or breaks; DELETE with internal/appdesk, appnet, egressgrant unless a slice wires them (HYPOTHESIS).
- CHOIR_COMPUTE_CAPSULES, CHOIR_EGRESS_GRANTS, AUTH_DEVICE_RELEASE: not found in Go code by the inventory's grep (FACT per inventory; I did not re-grep); doc-only. Delete the doc mentions.
- CHOIR_PACKAGE_SHARE_READS, CHOIR_RELEASE_READS: normal-state read scopes, not features.
- CHOIR_EVENT_TABLE_CREDENTIAL_DIR (event_table.go:46-): dies with event_table.

### 5. Conflicts, overlaps, ordering

C1. texture.hold_trajectory vs the 2026-11-01 owner ruling "each desk starts from system prompt and REPL only". FACT: the seed was deleted for every desk except Texture's carry (run_memory.go:120-146), and hold_trajectory is the Texture carry (texture_agent_revision.go:682, run_memory.go:140). Deleting "the prior-run seed" is already done; deleting hold_trajectory too would finish the ruling for Texture. Also the actor-rewarm digest path (`LatestTextureActorMemorySourceRun`, metadata actor_rewarm_source_loop_id, texture_agent_revision.go:677-681) is the superseded seed mechanism still sitting beside its replacement; HYPOTHESIS that it is only used by the Texture carry now (run_memory.go comment: "Nothing replaces the seed").
C2. store.event_table vs store.telemetry_partitions vs telemetry.batch vs tape.group_commit: four mechanisms for where telemetry lands. event_table and partitions both stamp a ProjectionOp.Table on the tape, share the stream_seq lease gate (telemetry_partition.go:856,864 "same gate as choir_events") and the same table schema (choirEventRow comment: "or a telemetry partition's, which has the same table"); event_table writes floor 2, partitions floor 5. If both were on, a kind has two possible homes. FACT code comment says both read every table. event_table is the superseded one; recommend delete.
C3. evidence.graph and engineering.slots_graph: share graphHomeMu (graph_home.go) and importGraphHome (computer_settings.go:170-200). The evidence import holds the lock for the whole table while slot claims wait (FACT: ImportEvidenceToGraph locks at entry). Order: slots_graph first (small), evidence second (large) or fix the lock.
C4. engineering.progress_to_texture and texture.owner_act_wakes both mint platform-authored records into the same Texture inbox and wake it; HYPOTHESIS that on a busy self-dev round (report then approve) they produce two close wakes.
C5. texture.root_desk, source.rebase_offer, source.projection: chain D1. rebase_offer is inert unless projection AND root_desk are on (rebase_notice.go:100-101, error text in source_projection.go:328). projection ON without the other two leaves a computer able to be refused casts forever behind a moved base (the 10-27 incident). Required order: root_desk (with F4/F5 resolved), then projection + rebase_offer together, with the U4 rebase proved once on d@e.
C6. engineering.carry_unfinished vs the older "carry rejected candidate into the next round" (ledger 2026-10-25 01:2x, no switch). Two carry mechanisms for different stop causes; HYPOTHESIS they can both offer in one round.
C7. packages.share requires packages.publish (a release must exist); publish requires a self-dev operation (api_package_release.go:~46). Share before publish is a no-op.
C8. app_packages.objects vs files.object vs packages.*: three "object" stores on the tape for different kinds; no overlap in keys (FACT kinds differ: package-declared kinds, choir.file, release records). Both depend on the owner route/Files API being reachable.
C9. intake.event_wakes requires intake.events (event_wakes.go:425-426, source_schedule.go:212); intake.labels is independent but computer_settings.go:25 makes the labels reader return true when event_wakes is on (labels implied).
C10. calibration.feedback and texture.hold_trajectory both inject prior context into a desk's turn; HYPOTHESIS they interact in token budget (not in correctness).
C11. Host-side overlaps (the other builder's; FACT from the inventory): store.event_table <-> CHOIR_EVENT_TABLE_COMPUTERS grant; texture.evidence_embeds <-> CHOIR_VISUAL_CAPTURE_COMPUTERS; research.related_story <-> CORPUSD_RELATED_STORY; source.projection <-> host base fetch availability; telemetry_partitions/group_commit <-> PLATFORM_RECEIPT_COMPACT_COMPUTERS (host receipt compaction of the same tape; HYPOTHESIS no conflict).
C12. Rollback floors: event_table writes floor 2, partitions floor 5; flipping partitions default-on locks every computer out of older binaries on its next boot. Affects every deploy rollback path (FACT comment computer_setting.go:219-223).
C13. Registry staleness (FACT): telemetry_batch row says R0 but default is true; store_event_table / intake_event_wakes rows are off-everywhere with expiry 2026-11-25; texture_root_desk row says leave off on the paper while the inventory says on for owner computers.

### 6. Summary recommendations (confidence)

ON (all computers): owner_directions (H), progress_to_texture (H), owner_act_wakes (H), app_packages.objects (H), slots_graph (H), files.object after writer fix (M), carry_unfinished after pruning or accepted (M), evidence.graph after bounding the import (M-H), group_commit R0 then paper (M-H), calibration.feedback R0 (M), packages.publish R0/b@c (M).
ON with gates: texture.evidence_embeds (needs RV-04 and host grant), partitions (widen by ring, mind floor 5), packages.share (after first real release), dispatch.wide_slots (paper per owner; fleet is a cost choice).
DECIDE: texture.hold_trajectory (ON vs DELETE with the 11-01 ruling), texture.root_desk + source.rebase_offer + source.projection (ON as a chain only after U4 is proved, else DELETE all three).
Autopaper features: intake.events/labels, research.system1/related_story: stay as they are (ON for the paper).
DELETE: intake.event_wakes (+path), store.event_table (writer arming and setting; keep table), management.serial_tasks, store.compact_force (setting), store.debounced_checkpoint, store.trace_log (registry rows; keep keys only if a stored value must still read, which only matters for the settings list), env twins CHOIR_TELEMETRY_BATCH and CHOIR_SOURCE_PROJECTION after default on, unwired CHOIR_APP_DESKS family.

## Part C. Host switches

### Host switch audit (read-only), origin/main 5cb70f9c4, 2026-11-03

Scope: host-level switches in nix/node-b.nix (and nix/autoputer-vm.nix where the guest half lives), the `*_ONLY` selectors and their Go readers, and the registry rows in switches/registry.yaml. Not covered: guest computer settings (the other builder), the visual-capture grant list (being deleted elsewhere; it appears here only where it conflicts with others).

Method: code read at origin/main in worktree /Users/wiz/Dev/go-choir-switches-host; evidence from docs/definitions/*.ledger.md, docs/problems, docs/evidence, scripts/proofs. No ssh, no staging reads. Every claim is labelled FACT (read in the repo, citation given) or HYPOTHESIS (inferred, not confirmed). Per-computer stored settings and what is running on Node B are NOT visible from the repo, so "on at X" is always the nix value, not a live reading.

Commits made by me before the stop: NONE. The branch switches-on-host is at origin/main (5cb70f9c4), the worktree is clean, nothing is pushed.

---------------------------------------------------------------------------

### 0. Plain-language catalog (what we have)

Eight to ten words each would hide too much; one sentence each. "Selector" means a list of computers (ids, `class:test`, or `*`) that a feature applies to; empty means nobody.

### VM lifecycle and memory (vmctl, internal/vmmanager)

- VM_SNAPSHOT_DIFF_ONLY (+ VM_SNAPSHOT_TYPE=diff): when a computer hibernates, write only the memory pages that are resident (a sparse "Diff" file) instead of the whole 8.6 GB image, so hibernate and wake are about a second and use less disk.
- VM_CHECKPOINT_CADENCE_ONLY: makes the computer's own guest publish a checkpoint (a signed, hash-witnessed restore point of its tape and store) every 2,500 events or 60 minutes on its own, instead of only at deploys and self-development applies.
- VM_BALLOON_STATS_ONLY: turns on Firecracker's balloon statistics polling (every 5 s) so the host can read a guest's own view of its memory; it does not change memory, it makes it visible.
- VM_GUEST_RECLAIM_ONLY: boots the guest with the kernel's DAMON_RECLAIM, which frees page cache that has not been touched for 5 minutes (an early attempt to give memory back).
- VMCTL_GUEST_CACHE_RETURN (+ _ONLY, _BUSY_COLD_COMPUTERS, VMCTL_GUEST_MEMORY_CAP_COMPUTERS): the host asks an idle or busy-but-cold guest to drop clean page cache so the host gets the RAM back; the memory cap row keeps the paper's guest cache near 3 GiB.
- VM_CAPSULE_TUNNEL_ENFORCE_ONLY: makes the egress proxy for engineering capsules refuse an HTTPS tunnel whose TLS server name does not match the host the capsule asked to connect to (otherwise it only logs the mismatch).
- VMCTL_GUEST_DNS_ONLY: points the guest's resolver at the host's Unbound instead of public DNS, so the host controls name lookups.
- VM_GUEST_LAYERED_COMPUTERS (guestLayeredComputers): boots a computer from a layered image (a slow-changing base plus a small per-deploy layer) so deploys copy and store less.
- PLATFORM_RECEIPT_COMPACT_COMPUTERS: stores the append receipts of the named computers in a smaller lossless form (about a third of the bytes) in corpusd.
- VMCTL_PRIMARY_KEEPALIVE_MODE: "off" lets primary computers hibernate when idle; the code default "under-capacity" would keep them warm while the host has room.
- VMCTL_STOP_MANAGED_ON_EXIT: if true, vmctl stops every computer when it exits; false (the intended value) leaves them running so a vmctl restart does not stop anyone.

### Gateway and providers

- gatewayModelScheduler (GATEWAY_MODEL_SCHEDULER): replaces the per-computer "60 model calls a minute" counter with a fair queue (24 calls in flight overall, 6 per computer, 256 queued, 120 s wait, then 429), so one busy computer cannot starve another.
- CHOIR_CLAUDE_DELEGATE_COMPUTERS (+ _EXPIRES, _TOKEN_FILE): lets named computers' engineering capsules call the owner's Claude subscription (Sonnet) as a "delegate" through a token the host copies onto the computer's credential disk.
- PROXY_EXPAND_REQUESTERS: the list of signed-in users who may press "expand this story" on an Autopaper story (the private door). The public door (PROXY_EXPAND_PUBLIC=on) for logged-out readers is already on.

### Network, mirrors, fetch

- hostFetchVMs / CHOIR_HOST_FETCH_VMS: makes a listed computer's web reads (research reads, imports, podcast and YouTube fetches, source polls) go through the gateway's fetch verb instead of the computer's own internet connection, with an address guard and a per-computer budget.
- CHOIR_CAPSULE_MIRROR_COMPUTERS / _ECOSYSTEMS / _MIRRORS_ONLY (+ the pkgmirror unit's -ecosystems): points engineering capsules at a host package mirror (npm, pypi, crates, git) and, with MIRRORS_ONLY, removes those registries from the capsule's allowed direct connections.
- CHOIR_EVENT_TABLE_COMPUTERS: the host half of store.event_table; lets a computer's writer put ten telemetry kinds in a separate choir_events table instead of the main object table.

### Capsule grants that share the credential disk

- CHOIR_VISUAL_CAPTURE_COMPUTERS: lets capsules launch Chromium to screenshot a candidate (not audited further; being deleted elsewhere).

### App desks

- CHOIR_APP_DESKS (internal/appdesk, appnet, egressgrant): the daemon-side half of "an app package carries its own app desk"; none of it is wired into any binary, so no app desk can exist today.

### Docs-only names (no code)

- CHOIR_COMPUTE_CAPSULES, CHOIR_EGRESS_GRANTS, AUTH_DEVICE_RELEASE: named in design docs; grep of *.go, *.nix, *.yaml, *.sh finds nothing (FACT, ran `grep -rn` over those types).

---------------------------------------------------------------------------

### 1. Cross-cutting facts (read these first)

1. FACT: "off until 2026-11-04" is not a precondition from a drill or doc. It is `grandfathered_off_until`, a lint date set when the registry was seeded on 2026-10-27 (commit cb7ff6814). scripts/check-switch-registry lines 307-313: once that date passes and the switch is still off for every computer, the lint appends an ERROR ("turn it on at R0 or delete it"), which fails CI. Affected host rows: choir_capsule_mirrors, host_fetch_vms, proxy_expand_requesters, gateway_model_scheduler (the settings rows packages.publish, packages.share and others carry the same date; not mine). So on 2026-11-04 CI fails unless each is moved to R0 (name d@e) or deleted. "Turn on at R0" (the proof ring, d@e) satisfies the lint; it does not require fleet-wide on.
2. FACT: "off until 2026-11-25": choir_event_table_computers and unwired_packages.
3. FACT: almost every VM selector below puts a kernel argument or a credential-disk file in the launch-config hash. Naming a computer therefore forces ONE cold boot of that computer at its next start (snapshot refused because the hash moved). Examples: computer_select.go docs, guest_dns.go lines 24-27, balloon_stats.go lines 24-27, guest_reclaim.go lines 36-38, checkpoint_cadence.go lines 25-27, capsule_tunnel_identity.go lines 22-24. Grants (host fetch, mirrors, delegate, capture, event table) reach a computer only at a cold boot because a snapshot resume keeps the old credential disk. If several selectors change in one deploy, they share that one cold boot; staging them across deploys costs one cold boot each.
4. FACT: the paper (computer-1355b83b) and the owner's computer (computer-03335285) are protected in some selectors but there is no code-level exclusion in the generic selector: `selectsComputer` (internal/vmmanager/computer_select.go) matches `*` for any computer. Exclusions that exist are ring choices in nix comments, plus proof scripts that refuse the paper, owner computer and delegate as targets (net01-host-fetch.sh header, net02-mirrors.sh header).
5. FACT: the paper already runs: jailer (nix:1557 names it), data discard `*` (nix:1709), receipt compaction active (nix:1139), cache-return busy_cold active (nix:1913) with a 3072 MiB guest ceiling (nix comment ~1920s). The paper does NOT run: Diff snapshots, checkpoint cadence, balloon stats, DAMON, tunnel enforce, layered image, host fetch, mirrors, scheduler (all by nix value). Whether `class:primary` in VMCTL_GUEST_DNS_ONLY reaches the paper is HYPOTHESIS: the code comment (guest_dns.go lines 21-23) says `class:` misses the platform computer's boot and the resume config because WarmnessClass is never set there.
6. FACT: the data system cutover is in flight (branch cutover/data-system). Switch rows or comments that name "Dolt table-hash read", "prolly trees", or "Dolt Fatalf" (checkpoint cadence, event table, apply replay) describe the old store. Whether the checkpoint bind survives the cutover unchanged is HYPOTHESIS; I did not read the new store's checkpoint path.

---------------------------------------------------------------------------

### 2. Per-switch audit

Format per switch: (1) what/where, (2) proof state, (3) effect if on for all, (4) failure mode, (5) cost of deleting, (6) recommendation, (7) dependencies.

### 2.1 VM_SNAPSHOT_DIFF_ONLY (+ VM_SNAPSHOT_TYPE=diff) -- registry vm_snapshot_diff

1. FACT: reader internal/vmmanager/config.go:167; decision snapshot_diff.go:87 and :287; nix node-b.nix:1739-1740 (`VM_SNAPSHOT_TYPE=diff`, `VM_SNAPSHOT_DIFF_ONLY=d@e,class:test`). Needs Firecracker >= 1.15 (snapshot_diff.go diffMinFirecrackerMinor=15, a corruption fix #5705); falls back to Full whenever a condition fails.
2. Proof: FACT d@e: nix comment 1727-1731 records a Diff hibernate writing 503 MB of 8.6 GB in 1.6 s and waking in 287 ms (2026-10-26 20:24). FACT: the first attempt on 10-26 18:58 failed with mincore EPERM under the jailer and fell back to Full (docs/problems/diff-snapshot-mincore-eperm-under-jailer-2026-10-26.md); FACT: a later Diff of d@e at 1371 ms on its layered release (choir-lean-system ledger line 55, 2026-10-31 16:16). class:test: on by nix since 10-28; no wake-to-healthy p95 reading found in the repo (HYPOTHESIS: none taken). The registry's promote bounds (`wake_to_healthy_ms_p95 <= max(2x own baseline, 1500)`, `snapshot_refusals_integrity == 0`) have no recorded reading in the repo.
3. On for all: FACT the paper never idles-hibernates but does hibernate on planned host stop / shutdown (S5 busy hibernate, cmd/vmctl/host_stop.go), so for the paper Diff only affects those. HYPOTHESIS: a Diff of the paper's 8 GiB guest writes only resident pages, and the data-system ledger line 196 shows snapshot chunks for the paper were already filling Node B's disk (54 GB chunk namespace for one paper snapshot, 507 refusal added at <5% free); Diff should shrink that, not grow it, but page-range dedupe interplay is unmeasured. Cold boot: yes, once (SnapshotType is in the launch hash, snapshot_diff.go:287 compares meta.Config).
4. Failure: a Diff that "loads but is wrong" is the worst case and would be SILENT (a computer wakes with a zero-filled page it should have had) and effectively irreversible for that computer's memory state. Mitigations in code: Firecracker >= 1.15 gate, mincore fallback to Full, THP/swap checks (snapshot_diff.go header). Cold-boot fallback exists when a Diff cannot load (nix comment 1736-1737).
5. Delete cost: low. Remove the selector and let Diff be the code default (registry `retire`), or delete the feature and keep Full: the Diff code is about one file (snapshot_diff.go) plus tests.
6. Recommendation: ON, ring widen to `*` after one more reading on class:test, confidence medium. The d@e and class:test evidence is real; what is missing is an integrity soak on a computer with real state. The paper is the highest-stakes target; I would put it last (R5) but the owner's rule says same as the fleet. If the owner wants one deploy: `*`.
7. Dependencies: needs Firecracker >= 1.15 (all computers run the pinned build; FACT nix comment 1727). Interacts with layered ring (Diff worked on layered, 10-31), snapshot_release_retain (release-bound snapshots; both on), and the host disk guard.

### 2.2 VM_CHECKPOINT_CADENCE_ONLY -- registry vm_checkpoint_cadence

1. FACT: internal/vmmanager/checkpoint_cadence.go:39 (reader) and :60 (decision); nix:1689 (`d@e,class:test`); guest consumer internal/agentcore/checkpoint_cadence.go (comment line 30); guest boot script nix/autoputer-vm.nix ~567-580 validates two boot args (`choir.checkpoint_every_events`, `..._every`). The guest compiles the cadence off; this selector is the only thing that turns it on.
2. Proof: FACT d@e and b@c ran cadence (docs/problems/paper-never-checkpointed-2026-10-27.md lines 17-18: checkpoints from d@e's guest cadence and b@c `cadence`; none for the paper). FACT measured cost on d@e: the bind held appends 630 ms at 13:34 and 1.131 s at 16:48 under drill load (data-system ledger line 202, 2026-10-30 to 31). FACT, same line: "Checkpoint cadence not widened to the owner's computer. ... on the owner's 2 GB store that is tens of seconds an hour" -- the cause is a CAS on the witnessed head forcing a pinned attempt that holds appends for the whole scan. The fix (design 11.1, "a checkpoint references a snapshot record") is listed as "red, not started". FACT: ledger line 184 states the paper and owner computer bind checkpoints only at planned updates and applies.
   This is a measured correctness-adjacent reason (append stall), not a staging preference. It is the one host selector where the repo contains a numeric reason not to widen.
3. On for all: HYPOTHESIS, from line 202: the owner's computer would stall appends for tens of seconds per hour; the paper (store much larger: 4.93 GB run telemetry, 660,000 events/day per ledger line 191) could be worse. Cold boot once. Disk: each checkpoint is a published object; unknown size (HYPOTHESIS). Provider spend: none.
4. Failure: loud, not silent: a stalled append is visible latency; a failed tick fails loud (paper-never-checkpointed lines 460-464). But on the paper a long append hold during the paper's 24/7 publishing is a visible outage-class slowdown.
5. Delete cost: HYPOTHESIS medium; deleting the selector makes the cadence unconditional (registry `retire`), which is the same as turning it on for all. Deleting the feature leaves the paper with no routine checkpoint, which is the recorded problem (paper-never-checkpointed).
6. Recommendation: ON at class:test + d@e now (as is), widen to the owner computer and paper ONLY AFTER the design 11.1 fix (or after the data system cutover changes the bind; HYPOTHESIS that it does). Confidence high that `*` today is wrong, medium on timing. Because the owner rule forbids "stays off", the honest framing is "ON, ring-walked, blocked_by a named fix", which is what the registry's `gate` already says (read last_bind_millis).
7. Dependencies: tape.group_commit (guest setting; HYPOTHESIS it batches appends, changing the hold the bind sees), store epoch/checkpoint floor, the cutover.

### 2.3 VM_BALLOON_STATS_ONLY -- registry vm_balloon_stats

1. FACT: config.go:146; balloon_stats.go:44; nix:1643 (`computer-6c50d6e5,class:test`). Rides the existing free-page-reporting balloon (VM_BALLOON_FREE_PAGE_REPORTING, all computers per registry).
2. Proof: FACT RES-10 steps 1 and 2 built; inert deploy L proved (ledger first-pass lines 149, 179, 203: balloon reason `stats_disabled`, 2026-10-21). FACT: on for b@c and class:test since 2026-10-26 (nix comment 1640-1642). Stats reader exists and is host-only read-only with a 2 s timeout.
3. On for all: cold boot once (balloon is in the launch hash, balloon_stats.go header). Memory: stats polling every 5 s is a few bytes; no RAM change. Disk/network/spend: none. Its value is that D6/S6 memory control reads guest memory views (HYPOTHESIS that guest_balloon.go reads these stats rather than guest /health; I did not trace).
4. Failure: low. Worst case a stuck VMM read; bounded at 2 s (balloon_stats.go). Not silent: reasons are returned instead of zeros.
5. Delete cost: very low; make polling interval unconditional (registry retire).
6. Recommendation: ON `*`, confidence medium-high. Smallest-risk widening in the audit; one cold boot per computer, which should be coalesced with other kernel-arg changes.
7. Dependencies: free page reporting balloon (on). Shares a cold boot with DNS, cadence, layered, DAMON.

### 2.4 VM_GUEST_RECLAIM_ONLY (DAMON) -- registry vm_guest_reclaim_damon

1. FACT: guest_reclaim.go:40 (reader), :56-70 (kernel params: min_age 300 s, wmarks 950/900/5, skip_anon); nix:1660 names 9648031f and 71afbf14.
2. Proof: FACT negative. docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md line 23 quotes ledger 10-22 13:06: "guest page-cache reclaim as configured returns no host memory within the idle window" (1,055 to 1,024 MiB cache; host RSS flat). FACT: no control computer existed; the registry's `blocked_by` is "a control host-RSS reading" and has no reading. FACT: line 79 of the same doc and the density design (d6, line 573) say DAMON is off for the paper and the paper gets `busy_cold` instead.
3. On for all: HYPOTHESIS: no host memory benefit (measured negative) plus a CPU quota of 10 ms/s and 128 MiB/s of reclaim per guest, which can cause refault reads (disk) for hot-ish cache. One cold boot. Not worth it.
4. Failure: perf degradation (extra refaults), not silent corruption.
5. Delete cost: low. guestReclaim code in config.go, manager.go (3 uses), guest_reclaim.go and its test; two computers cold boot once to drop the args. D6-M20 already says D6 never names these two.
6. Recommendation: DELETE, confidence high. Superseded by D6 (page-cache return via memory.reclaim) and the S6 elastic ceiling, which measured 6 MiB/min guest reads at a 3 GiB ceiling.
7. Dependencies: none inbound. It is the mechanism D6-M20 keeps apart from cache return so readings do not blur.

### 2.5 VMCTL_GUEST_CACHE_RETURN family -- registry vmctl_guest_cache_return

1. FACT: nix:1888-1889 (`active`, `_ONLY=d@e,computer-734d0344`), :1890 triggers idle, :1913 `_BUSY_COLD_COMPUTERS=d@e=active,paper=active`, plus `VMCTL_GUEST_MEMORY_CAP_COMPUTERS` (ring 2 by class per nix comment). Code: internal/vmctl/guest_cache_return.go, guest_balloon.go.
2. Proof: FACT paper busy_cold first act 2026-11-01 07:34Z: guest reclaimed 1031 MiB, host RSS 7748 to 7574 MiB (17%), refaults 0; outcome B (cache refilled in minutes because the derived-key validation streamed state.db) (memory-optimization ledger lines 12; problem doc paper-busy-cold-freed-cache-refills-2026-11-01). FACT: derived-key marks fix proven 16:11Z same day (host RSS 2.9-3.3 GiB for ten minutes). FACT: S6 ladder by hand on the paper 16:33-17:33Z: ceiling 3072 holds 3.0 GiB RSS at 6 MiB/min guest reads; 2048 bends to 508 MiB/min. Chosen: paper 3072, d@e 2048. FACT: telemetry retirement under the cap 17:51-18:03Z held RSS 2.96-3.04 GiB, no OOM, 9.0 GB disk reads in 12 min. So the paper IS already a live member of this family via BUSY_COLD and the cap. The generic `_ONLY` idle trigger names two computers (d@e and 734d0344) and the registry target is R5 with `confirm: human`.
3. On for all (idle trigger `*`): HYPOTHESIS: idle trigger on computers that are about to hibernate anyway (VMCTL_IDLE_TIMEOUT=10m, nix:~1820) mostly duplicates what hibernate frees; value is on computers that stay up. Cost: some disk re-reads. The paper never idles; the owner computer rarely does (protected from hibernate per nix comment ~1815 "never ... the paper, a public platform computer or a paid account's computer"), so the idle trigger is a no-op for the protected pair (HYPOTHESIS: their activity keeps them non-idle; busy_cold is their trigger).
4. Failure: refault storms or a guest OOM if the ceiling is too low for a burst; the code stands aside for a live capsule or self-development, drops clean cache only, never anonymous memory (nix comment). Loud (refault_stop backs off for three intervals).
5. Delete cost: high. This is the memory-density mechanism the owner has been building toward (host RAM is the density limit); deleting it forfeits the 8.1 to 3.0 GiB paper reduction.
6. Recommendation: ON; `_ONLY` to `*` is harmless/low-yield (idle computers hibernate anyway); the valuable widening is the memory-cap and busy_cold entries for the owner computer. Confidence medium. Owner computer should get a BUSY_COLD entry only after it has its own ladder (its working set is unmeasured).
7. Dependencies: needs free page reporting negotiated, balloon stats (HYPOTHESIS), guest memory.reclaim (kernel 6.18.57 per ledger), hibernate timers. Conflicts with DAMON (see section 3).

### 2.6 VM_CAPSULE_TUNNEL_ENFORCE_ONLY (+ image CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY) -- registry vm_capsule_tunnel_identity

1. FACT: config.go:144; capsule_tunnel_identity.go:26 (kernel param `choir.capsule_egress_tunnel_identity=enforce`); guest boot script nix/autoputer-vm.nix:561-565; image default observe at autoputer-vm.nix:1030; policy applied in internal/capsule/executor.go:218 (`policy.TunnelIdentity = e.tunnelIdentity`) for both L2 and claude-mode capsules; refusal in internal/capsule/egress.go:445-454. nix:1675 names 9648031f only.
2. Proof: FACT 2026-10-22: observe on 71afbf14, 11 CONNECT entries, all toolchain tunnels passed, exactly one would_refuse (github.com, SNI pypi.org, a git/script tunnel); enforce on 9648031f, 9 entries, all toolchains passed, the mismatch refused with 0 bytes (first-pass ledger lines 658-659). The problem doc's global gate (lines 227-238) needs: (a) zero would_refuse for real toolchains over time, (b) the selected computer's toolchains passing (done), (c) "a real claude-mode (Claude delegate) capsule session shows tunnel_identity pass for api.anthropic.com". FACT: (c) is NOT met: the side-by-side ledger line 7 says the delegate token is "unproven" and "no claude-mode run on record since 2026-10-28" -- and the delegate's Bun/Node client may send GREASE ECH, "one GREASE ECH from it would end every delegate session under enforce".
3. On for all (image to enforce): engineering capsules on every computer refuse mismatched tunnels; no restart needed for a flip in the image line (it is unit Environment, read at capsule executor start) but the guest image changes, so the guest security epoch gate applies. For the paper/owner computer: their capsules (self-development) would enforce. HYPOTHESIS: github.com `git` tunnels that sent a non-matching SNI (the one would_refuse) would fail builds that rely on that path.
4. Failure: a wrong refusal breaks a build or the delegate; loud (capsule ledger entry `refused`), not silent. The security failure of staying observe is silent (an unbound CONNECT).
5. Delete cost: medium. Deleting the *selector* is cheap (3 files); deleting the *check* removes SEC-06's binding. Do not.
6. Recommendation: ON everywhere EXCEPT claude-mode capsules until (c) holds. This needs one small code change (claude policies stay observe even when the image says enforce) -- I had found that spot (executor.go:213-218) before the stop. Alternative with zero code: `VM_CAPSULE_TUNNEL_ENFORCE_ONLY=*` minus the two delegate computers is not expressible (no exclusion syntax). Confidence medium-high. Name this as a red guest-surface change (needs Security-Effect trailer).
7. Dependencies: delegate (gate (c)), mirrors (with MIRRORS_ONLY the mirrored registries leave the allowlist so fewer tunnels are checked), the "name computer ids only; class: misses the platform computer's boot" note (capsule_tunnel_identity.go lines 12-16): `class:` is unreliable, `*` is the global flip meant to go through the image line.

### 2.7 VMCTL_GUEST_DNS_ONLY -- registry vmctl_guest_dns

1. FACT: config.go:205; guest_dns.go:34 (`choir.dns=<tap host address>` kernel arg when VMCTL_GUEST_DNS_PORT>0); nix:1506 `class:test,class:primary`.
2. Proof: FACT SEC-10 step 4 PASS 2026-10-23 08:12: UDP DNAT counter 0 to 2 packets after test computer T booted, guest resolved through Unbound (first-pass ledger line 1055). FACT a defect found and fixed: guests fell back to public DNS because resolved had FallbackDNS; fix 506f5c4f5 writes an empty FallbackDNS (ledger line 1261). FACT D32 widened to class:test (line 1480). `class:primary` added later (date not located). The registry gate says "paper and public_platform at a watched planned refresh, cold boot once" and promote bounds guest_dns_probe_exit == 0 and start_refusals <= control + 1; no recorded reading found (HYPOTHESIS: not taken).
3. On for all: cold boot once per remaining computer (kernel arg in the hash; guest_dns.go lines 24-27). Network: lookups go to host Unbound (loopback) instead of public resolvers; the host becomes a single point for guest name resolution. Rollback order matters (guest_dns.go lines 28-31): empty the list FIRST; removing Unbound or the port before the named computers cold boot leaves them with no resolver.
4. Failure: loud and total for the affected computer: no DNS means no model calls (gateway is by IP? HYPOTHESIS: the guest reaches the gateway by tap IP, so inference may still work, but all name-based egress fails). If Unbound is down every DNS-using guest feature fails at once (single point).
5. Delete cost: low-medium; unconditional host resolver (registry retire). Leaving public DNS means the host cannot enforce egress policy by name (HYPOTHESIS tied to host-egress-floor problem).
6. Recommendation: ON `*`, confidence medium. Should come with the Unbound health check in the acceptance steps. Prefer `*` over `class:primary` for reliability (comment says `*` belongs to "step 2 (public 53 out of the floor), after a probe"): if `*` is chosen, the public 53 floor change is a separate decision and must not be bundled.
7. Dependencies: Unbound unit, VMCTL_GUEST_DNS_PORT, host egress floor.

### 2.8 Layered ring (guestLayeredComputers) -- registry has no row

1. FACT: nix:48 `[d@e]`; nix:1588-1589 VM_GUEST_LAYERED_*; tool profile nix:~52-60 `toolchain=*;office=none;browser=grant:visual-capture;delegate=grant:claude-delegate`. Reader internal/vmmanager/layered_ring.go.
2. Proof: FACT d@e hibernate/wake onto layered release 9sn23vjm, 2026-10-31 16:16 (lean-system ledger line 55); install defect (sed missing in wrapper) found and fixed 16:25 (line 56) and verified live 18:01 (line 64). FACT: a store-schema defect hit b@c's apply on 11-02 ("release declares no store schema epoch" ... "every non-layered ...", side-by-side ledger line 7, truncated in my read -- I did not confirm whether it is a layered-specific defect; HYPOTHESIS it is about the single-image path).
3. On for all: cold boot once; disk/deploy-time savings are the point; tool layers: a layered computer without a matching rule gets an empty tool slot ("a clear error, never a hang") so `office=none` removes Office tools for everyone and the browser/delegate tools exist only when the grant exists.
4. Failure: loud (missing layered release launches the single image, per nix comment 44-45).
5. Delete cost: high (it is the new deploy architecture).
6. Recommendation: ON by ring, d@e then class:test then fleet, confidence low-medium on timing (only one computer has run it). It is a rollout item, not a stuck switch. NOT in the registry: registry hygiene gap (the lint's let-binding suffix list does not include "Computers"? it does include "Computers" -- so I cannot explain why it passes; HYPOTHESIS: lint matches only names ending exactly with the suffixes and guestLayeredComputers ends in "Computers", so a registry row may exist under another id I did not find).
7. Dependencies: tool profile grants: delegate and capture grants become layer attachments, so grant lists and layered ring must change together.

### 2.9 PLATFORM_RECEIPT_COMPACT_COMPUTERS

1. FACT: nix:1139: d@e, paper, owner computer all `active`. The nix comment (1128-1135) still says dry-run for the paper and owner; the value says active. FACT inventory B17 says "d@e + 2 ids active". Registry row platform_receipt_compact: ring R0,ids:2, ops.
2. Proof: FACT data-system ledger line 64 (lean-system): receipt compaction d@e=active, paper and owner dry-run at 2026-10-31 18:25 UTC; later widened. Dry-run counts and round-trip proofs were the gate.
3. On for all: it is already on for the three computers that matter; "all" would add the rest of the fleet (small stores). Corpusd-side; no restart.
4. Failure: lossless by design; rollback needs `=expand` and wait for backfill done "before reverting the binary" (nix comment). Irreversible-ish if the binary is reverted mid-backfill.
5. Delete cost: n/a (it is a storage win, ~2/3 of the receipt bytes).
6. Recommendation: ON already; fix the stale comment (nix:1128-1135) and registry ring; widen to `*`. Confidence high.
7. Dependencies: the data system cutover (HYPOTHESIS: receipts live in the old corpusd store; check they survive the cutover).

### 2.10 VMCTL_PRIMARY_KEEPALIVE_MODE -- registry vmctl_primary_keepalive

1. FACT: cmd/vmctl/main.go:905 (default under-capacity), internal/vmctl/warmness_policy.go:23-24,65,72-74, ownership.go:3763; nix:1828 `off`.
2. Proof: FACT D3 comment (nix:1815-1827): off is what lets primary computers idle-hibernate at 10 minutes; "on" is the rollback. Registry `off_ok`: "off is the normal state".
3. On (under-capacity): primary computers stay warm while the host has room: more RAM resident, fewer cold wakes. Opposes the owner's density priority (2026-10-26 ruling).
4. Failure: none; both modes are valid.
5. Delete cost: low-medium: delete the mode and the under-capacity branch (ownership.go:3763 and warmness_policy), making "off" unconditional. The nix env line goes.
6. Recommendation: DELETE the mode (hard-code the D3 behavior), confidence medium. It is a rollback lever kept for a feature that is no longer rolled back. But the code default under-capacity means if someone removes the nix line, behavior silently flips -- another reason to delete.
7. Dependencies: VMCTL_IDLE_TIMEOUT, pressure reclaim, cache return.

### 2.11 VMCTL_STOP_MANAGED_ON_EXIT -- registry vmctl_stop_managed_on_exit

1. FACT: cmd/vmctl/main.go:158 `envBool(..., false)`; host_stop.go:1-30; nix:1741 `false`.
2. Proof: FACT CLAUDE.md "computers stay on", S5 busy hibernate decides hibernate on host shutdown regardless (host_stop.go decideHostStop). `true` kills every guest at every vmctl restart (every deploy).
3. On = stop all computers at every deploy. Never wanted.
4. Failure if on: every deploy hard-stops the paper.
5. Delete cost: trivial.
6. Recommendation: DELETE (remove the env, hard-code false) -- it is not a feature, it is a hazard. Confidence high. Registry: drop row.
7. Dependencies: host_stop.go reads the bool; keep the shutdown path.

### 2.12 gatewayModelScheduler (GATEWAY_MODEL_SCHEDULER) -- registry gateway_model_scheduler

1. FACT: nix:515 `"off"`, :2582 passes it to the gateway unit; internal/gateway/model_scheduler.go:42 (env), :91 (`on|true|1` only), defaults :46-49 (24 global, 6 per computer, 256 queued, 120 s wait, capped 5 min), health at /health `model_scheduler`.
2. Proof state: FACT proof 9 never ran. Proof 9 is defined in docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md lines 719-760 ("RES-11, after flipping GATEWAY_MODEL_SCHEDULER=on: 9. From one test computer, fire 40 concurrent small model calls through the product path while a second test computer makes one call: the second call's latency stays near a single call's; all 41 complete; /health shows queued and wait counters moving and returning to zero") and implemented as scripts/proofs/fp7-gateway.sh step_9 (lines 181-205, run `--label before`, make the calls manually, run `--label after`; passes if granted rose by >= 41, queued and in_flight return to 0). It REQUIRES the scheduler to be on first, so it can only run after the flip. FACT the document's order (line 626-628): body caps deployed in observe, proofs 10 and 11, proof 12 after a day, caps to enforce as their own deploy and proof 12 again, "Only then flip the scheduler on and run proof 9". FACT: caps are `enforce` since 2026-10-24 (nix:546-548). So the precondition is met in nix; whether proofs 10-12 were recorded is HYPOTHESIS (nix comment says "observe since D12 showed would_refuse {} and max_seen.model 163 KB", i.e. a reading exists). FACT problem-doc item 11 (queue wait outside the call deadline): the doc says the repair is a precondition of turning it on; model_scheduler.go header mentions the wait counts against inferenceTimeout (maxModelQueueWait comment), so it appears repaired (HYPOTHESIS: I did not read scheduleModelCall).
3. On for all: FACT the 60/min per-computer window is REPLACED by concurrency bounds (model_scheduler.go header). HYPOTHESIS effects: the paper and the owner computer are limited to 6 model calls in flight each; the paper's dispatcher runs 8 slots, 12 with dispatch.wide_slots (consolidate ledger line 22), plus research child calls, so the paper's calls above 6 queue up to 120 s then get a 429 with Retry-After. If calls take ~20 s, 6 in flight is ~18 calls/min, below the old 60/min -- the scheduler could REDUCE the paper's throughput and so its provider spend, or stall research fan-out; that depends on call duration (unmeasured). Provider spend per se is "tracked, never capped" (DEC-07), so no spend cap is introduced, but the old 60/min backstop on a runaway disappears, replaced by concurrency. No restart of computers; a gateway restart on deploy (cold, brief).
4. Failure: worst case in the problem doc (item 1): a slot leaks (panic, cancelled caller, provider that never returns), the pool drains, and every computer's model calls hang (global outage, LOUD but total). Mitigations in code: per-call context, Acquire returns for gone callers, wait bounded, `/health` counters. Rollback is a one-word nix change plus a gateway deploy.
5. Delete cost: medium-low. Code is one file plus tests (the old limiter remains underneath). Deleting forfeits fairness between computers, which matters once the fleet grows; HYPOTHESIS: today's fleet is small, so the benefit is not yet visible.
6. Recommendation: ON, and the cheapest correct gate is to run proof 9 on staging against two test computers as the first act after deploying "on" (it can only run when on). Confidence medium: the design and bounds are sound, but the paper's 6-concurrent cap vs its 8-12 slots is an unexamined interaction. If the owner wants safety: raise GATEWAY_MODEL_INFLIGHT_PER_COMPUTER (env exists, model_scheduler.go:44) to 12 for the paper's sake -- HYPOTHESIS this is a better first value than 6. Ring: the gateway has no per-computer switch; the scheduler is global (no ring mechanism exists).
7. Dependencies: body caps enforce (met), bearer binding enforce (met; its 429 with Retry-After 2 looks like the scheduler's 429 to the guest retry loop, HYPOTHESIS), guest 1.5 s research spacer and Retry-After loops ("two clocks", problem doc), dispatch.wide_slots.

### 2.13 hostFetchVMs / CHOIR_HOST_FETCH_VMS -- registry host_fetch_vms

1. FACT: nix:28 `hostFetchVMs = ""`, :1786 (vmctl), :2567 (gateway). Readers: internal/hostfetch/hostfetch.go:113-129 (`ParseVMList` drops `class:` tokens, no `*`), internal/vmmanager/host_fetch.go:32-39 (grant for exact vmID), internal/gateway/host_fetch.go:129 (`if !svc.allowed[computerID]` refuse), guest route hostfetch.GuestRoute (hostfetch.go:~170-190).
2. Proof state: FACT no proof ever ran: scripts/proofs/net01-host-fetch.sh exists (steps 1-8 listed in its header) but docs/evidence has no net01 artifact and the ledgers show only "NET-01 host fetch verb (off: no computer listed)" at deploy N (first-pass ledger line 235, 2026-10-22). The script's own header refuses the paper, owner computer and delegate as targets, so its proof computer is a test computer; the order is: baseline step 1 (--mutate), step 2, grant P by nix + deploy + cold boot, steps 3-8, then the rollback drill. Needs an owner API key (`CHOIR_API_KEY`) and SSH to Node B. It cannot run before a host list contains a test VM id.
3. On for all (needs code: no `*` token exists; `class:` is dropped on purpose, problem doc host-fetch-verb-open-to-every-guest-2026-10-22): every granted computer's research reads, content imports, podcast/YouTube fetches and source polls make their network hop through the gateway and "never fall back to direct egress" (nix comment 7-13). For the paper: its ~180 feed polls per 15-minute cycle plus article imports (hostFetchDefaultPerMinute 600 is sized for it, gateway/host_fetch.go:36-42) all go through the gateway on Node B. In-flight 16 per computer. Cold boot once (credential-disk grant). Network: shifts traffic from per-guest NAT to the host gateway process; gateway memory/CPU rises; the gateway also hosts all model traffic, so a fetch storm contends with it (HYPOTHESIS; separate budget but same process, MemoryMax 8G per nix).
4. Failure: LOUD and total for the affected computer's web reads: if the gateway is down or the verb misbehaves, a granted computer cannot fetch at all (by design, "never falls back"). On the paper this halts newspaper polling = production outage. It is not silent. Not irreversible: rollback = empty the list, deploy, cold-boot the computer.
5. Delete cost: medium. Deleting hostfetch (internal/hostfetch, gateway verb, vmmanager grant, guest route) removes the one mechanism that closes guest direct egress for research reads (SDC NET-01, "high security value" per registry).
6. Recommendation: ON at R0 (d@e) first and only then widen, confidence medium-low that "all computers now" is safe because the proof never ran. Do the registry minimum on 11-04: name d@e's VM id... but note the selector is VM ids (vm-...) not computer ids, and d@e's VM id changes at cold boot? (FACT: hostfetch.go: "VM IDs (vm-...)", d@e vm-70070cf7 appears in a 10-26 log). HYPOTHESIS: VM ids are stable per computer; unverified. A `*` token or computer-id matching would be needed to avoid VM-id churn; that is a code change.
7. Dependencies: the gateway fetch verb (gateway unit); budget CHOIR_HOST_FETCH_PER_MINUTE; DNS (the gateway resolves on the host, so guest DNS switch is irrelevant for fetches); the egress floor; `source.projection` needs "host base fetch reachable" (inventory, HYPOTHESIS about which fetch path).

### 2.14 Capsule mirrors -- registry choir_capsule_mirrors

1. FACT: nix:1782-1784 (all empty); readers internal/vmmanager/capsule_mirrors.go (computers: ids and `class:test`, no `*`; ecosystems; MirrorsOnly); pkgmirror unit nix ~2880-2900 `-ecosystems ''` (empty, so every ecosystem off); guest side capsule.MirrorAddr / executor.SetCapsuleMirrors (capsule_executor_linux.go:125-137).
2. Proof: FACT unit deployed inert 2026-10-22 (deploy G, ledger line 124). FACT scripts/proofs/net02-mirrors.sh has steps 0-5 (landing, grant, route mode, mirrors-only mode, sharing, rollback). The capsule probe (scripts/capsule-mirror-probe.sh) must run INSIDE an implementation capsule on the granted computer: "no channel from here reaches it, so the capsule runs are MANUAL" (script header). No net02 artifact in docs/evidence (FACT). So: never proven. Related FACT: a problem doc "capsule-package-egress-is-not-mirrors-only-2026-10-21" records that capsule egress to registries is open today.
3. On for all: each engineering capsule on a granted computer resolves npm/pypi/crates/git through the host mirror. Disk: pkgmirror cache (`-max-bytes 17179869184` = 16 GiB, git 16 GiB, nix:~2893) on Node B (HYPOTHESIS: 32 GiB combined cap competes with the 218G used / 253G volume noted in the memory ledger, i.e. real pressure). Network: mirror fetches from public registries by the host. MirrorsOnly additionally removes registry hosts from capsule direct egress: a package the mirror cannot serve fails the build with no fallback. Cold boot once. The paper/owner computers run self-development capsules, so they are directly affected.
4. Failure: broken builds for engineering, LOUD (capsule ledger/refused CONNECT). Cache poisoning protection exists (hash check before cache entry).
5. Delete cost: medium; pkgmirror is a whole unit (internal/pkgmirror) with a vendored dependency; deleting leaves capsules on open egress (the registered "not mirrors-only" gap).
6. Recommendation: ON at R0 with MIRRORS_ONLY off first, then class:test, then MIRRORS_ONLY, confidence medium. The registry's own walk plan says that; a "route mode" step 2 and "only mode" step 3 are separate proofs. For 11-04 the lint is satisfied by naming d@e.
7. Dependencies: the pkgmirror `-ecosystems` list must be updated in the SAME deploy as the vmctl ecosystem list or capsules are pointed at a prefix that answers 404 (vmmanager/capsule_mirrors.go header). Tunnel identity enforce (fewer checked tunnels). Go module proxy route is separate (already on).

### 2.15 CHOIR_EVENT_TABLE_COMPUTERS -- registry choir_event_table_computers

1. FACT: nix:1771 empty; vmmanager/event_table_grant.go:25-59 (ids only; class/`*` tokens are dropped on purpose); guest consumer internal/agentcore/event_table.go (both host grant AND owner setting store.event_table must hold); setting in internal/store/computer_setting.go:73.
2. Proof: FACT deploy 2 live and proved on 2026-10-22 07:4x ("fp1-event-table-deploy2.sh step 0 (no effect where off) passes 4/4 on the paper and 4/4 ..." first-pass ledger line 1526) and "store.event_table.* ops are live, each hold under a second" (line 1037). The grant list has been empty since; a computer that stamps raises the store floor irreversibly (design: "once a computer stamps, deploy 1 is its rollback floor").
3. On: HYPOTHESIS that it is redundant. Its stated rationale (computer_setting.go:73-80: "4 prolly trees per insert instead of 13") is Dolt-era. The data-system ledger (line 206) records "choir.event split, measured instead of built": the journal cuts already stopped the big appends and the batching/partition/group-commit work covers the rest. FACT the guest SQLite has a live choir_events table (inventory). Irreversible store floor.
4. Failure: irreversible store-schema floor for any computer that stamps (SILENT in effect: older binaries refuse to boot there).
5. Delete cost: low-medium: host half is two files (event_table_grant.go + test, plus manager.go wiring and credential material); guest half (agentcore/event_table.go, setting, writer stamping, dual readers) belongs to the settings builder and must be deleted with it.
6. Recommendation: DELETE (host half; guest half in tandem), confidence medium. Needs the owner's call because the registry marks `hold: owner` and the question "R7 may make choir_events redundant".
7. Dependencies: store.event_table (guest setting), telemetry.batch, telemetry_partitions (store floor 5), tape.group_commit. Must be decided together with the settings builder's treatment of store.event_table.

### 2.16 CHOIR_CLAUDE_DELEGATE_* -- registry choir_claude_delegate

1. FACT: nix:1809-1811; internal/vmmanager/claude_delegate.go (allowlist of ComputerIDs only, `ExpiresAt` required, token copied from the staged file); guest brokers internal/capsule/claude_delegate.go. nix:1810 expiry `2026-11-17T00:00:00Z`.
2. Expiry mismatch: FACT registry expires `2026-11-03` and its note says "Its own nix expiry is 2026-11-03T00:00:00Z"; nix says 2026-11-17 (renewed 2026-11-02, nix comment 1802-1804; side-by-side ledger line 7 "renewal to 2026-11-17 on branch delegate-renew-2026-11"). Registry is stale. Also the registry `life: experiment`: a registry expiry is the experiment window; after it, lint warns, and fails 7 days later only when the change adds a switch (script header item 4).
3. Proof state: FACT delegate computers: ad1b46d4 and 6c50d6e5 (two-arm comparison arm, side-by-side ledger line 7, set up 2026-11-02 20:30Z). FACT: the Sonnet check said "token from 2026-10-15 unproven, no claude-mode run on record since 2026-10-28". FACT: neither comparison arm reached engineering code as of that entry. So it has run (2026-10-20 resume on Sonnet, 10-25 b@c add, claude-delegate-resume-sonnet doc) but its current token/health is unproven.
4. On for all (a list of every computer): spends the OWNER'S Claude subscription for every engineering capsule: HYPOTHESIS unbounded subscription burn; FACT the owner turned it off on 2026-10-15 04:00 because "the owner's subscription is near its limit and must not be spent by Choir" (nix comment 1798-1800). The two-arm comparison needs the delegate on ONE computer and not the other (ledger line 7), so on-for-all ends the comparison. It is a permission-and-spend grant, not a safety feature.
5. Delete cost: medium (token path, broker, credential files, vmmanager reader, capsule claude mode, tests).
6. Recommendation: KEEP as an experiment with the window renewed in the registry (set expires 2026-11-17, owner-approved). This is the one switch where "everything on" conflicts with a recorded owner constraint (spend). Confidence high. If the owner wants no experiment, DELETE after the comparison reads out (Devin review 386 shipped the delegate-first path, so the code is actively used this week).
7. Dependencies: tunnel identity gate (c), the engineering model pin (Synthetic DeepSeek), layered tool profile (`delegate=grant:claude-delegate`), the comparison design.

### 2.17 PROXY_EXPAND_REQUESTERS -- registry proxy_expand_requesters

1. FACT: nix:946 empty; internal/proxy/config.go:85-105 (`ParseExpandRequesters`), :170; internal/proxy/platform_expand.go (doors). `PROXY_EXPAND_PUBLIC=on` at nix:953 already opens the public door.
2. Proof: FACT public door on since 2026-10-20 (owner notes section 32); caller logic at platform_expand.go:314-339. I did not find a recorded private-door use (HYPOTHESIS: none).
3. On: add the owner's user id (not in repo; an id, not an email, but I do not have it) or all signed-in users. Effect: a signed-in owner can ask the paper's research desk to expand a story; the public door already allows the same act for any reader (3/h/address, 10/day, global 3 in flight and 12/day, platform_expand.go:47-65). So the private door adds no capability beyond the public door, only a different throttle (HYPOTHESIS). Provider spend: each accepted request casts research on the paper (same as public).
4. Failure: spend on the paper; bounded by the global 12/day. Loud.
5. Delete cost: medium (registry says "a feature removal and test rewrite; lane/delete-dead refused it"). Deleting the private door leaves the public door as the only path.
6. Recommendation: ON for the owner only by adding the owner's user id (the registry's own suggested fix), confidence medium; DELETE of the private door is the alternative if the owner prefers one door. Cheapest decision that clears the 11-04 lint: add the owner id.
7. Dependencies: PROXY_EXPAND_PUBLIC, the paper's research desk, dispatch.wide_slots, scheduler (research calls).

### 2.18 CHOIR_APP_DESKS and unwired packages -- registry unwired_packages

1. FACT: internal/appdesk/appdesk.go:322-340 (`EnvMode=CHOIR_APP_DESKS`, only "on" is on) and `Activate`; no non-test caller of appdesk anywhere (grep `appdesk"` over *.go: only appdesk's own test, deskworker test and comments). internal/appnet (imports egressgrant) is imported by nobody. The WORKER half is wired: internal/deskworker/deskworker.go:30 calls yaegikernel.RegisterCarriedAppDesk at every desk worker start, which is a no-op for system roles.
2. Proof: FACT unit tests only (appdesk_test.go:253 etc.); design docs (app-package-carries-app-desk-2026-10-21.md section 6) say the order is "FP2's manifest, then S7-0 on a rebased branch, then this slice, then the first app desk (the podcast and reader app agents, launch item L3-03)". FACT side-by-side ledger line 7: slides/files/calendar/podcasts are being rebuilt as `<app>-next` packages "with no opens or desk until a switch".
3. On: wiring Activate needs (a) the daemon to fold package install events from the tape into appdesk.Set at boot, (b) a package manifest `desk` member that exists today in internal/apppackage? (HYPOTHESIS: apppackage exists, I did not verify a `desk` field), (c) a package that actually declares a desk, (d) the write-gate overlay for `choir.app.<package>.<kind>`. That is a multi-component slice, not small. Effect on the paper: none until a package declares a desk.
4. Failure: dead code, no runtime failure. Risk is the doctrinal drift (I26 says app desks ship with app packages; Texture first).
5. Delete cost: FACT 2,618 lines across appdesk/appnet/egressgrant with tests. CLAUDE.md and I26 forbid deleting app desk code ("app desk ships with its app package").
6. Recommendation (the one UNDECIDED item): do not delete appdesk. HYPOTHESIS (confidence medium-low): wiring is not a small slice. appnet and egressgrant are a different question: they are the egress-grant half (CHOIR_EGRESS_GRANTS, fp10-net-05) with no importer and no switch name in code; they could be deleted without touching I26 (HYPOTHESIS: appdesk does not import them; appnet imports egressgrant; confirm with `go build` after removal). Owner decision.
7. Dependencies: apppackage install events, deskmanifest, the `<app>-next` packages.

### 2.19 Docs-only names

CHOIR_COMPUTE_CAPSULES, CHOIR_EGRESS_GRANTS, AUTH_DEVICE_RELEASE: FACT no matches in *.go, *.nix, *.yaml, *.sh. They live in design docs only (app-desk-compute-capsule-2026-10-21, fp10-net-05, mac-device-key-release-2026-11-01 line ~213). No registry row. Recommendation: mark them "design only, not a switch" in those docs (green), confidence high. Cost: docs only.

### 2.20 Rows I did not audit in depth (registry host rows with `all` or ops posture)

vm_snapshot_release_retain (all, on since 2026-10-26), vmctl_jailer (paper included at nix:1557), vm_data_discard (`*`), vm_balloon_free_page_reporting, vmctl_admission_* (measured/active/wake since 2026-11-01), vmctl_pressure_reclaim (active), vm_hibernate_mode, the modes gateway_bearer_binding/body_caps/issuer/internal/owner_act (enforce). These are ON for all by nix value already; they only need `ready_to_retire` cleanup. HYPOTHESIS: registry ring column drift in several rows (e.g. platform_receipt_compact says ids:2, nix has three).

---------------------------------------------------------------------------

### 3. Conflicts, overlaps and ordering

### 3.1 Contend for the same cold boot / credential disk
- FACT: DNS (`choir.dns`), cadence (2 boot args), balloon stats, DAMON, tunnel enforce, layered image, and the grant files (host fetch, mirrors, delegate, visual capture, event table) all take effect only at a cold boot of the computer. Doing them in separate deploys cold-boots each computer once per deploy. The paper's cold boot is a planned-marker boot (CLAUDE.md restart rules) with replay/validation cost; coalesce everything into one deploy per computer.
- FACT: delegate, visual capture, mirrors, event table and host fetch all write into the SAME root-only credential disk directory, which a snapshot resume does not refresh.

### 3.2 Memory-return mechanisms that overlap (five of them)
- FACT: free page reporting balloon (all) + balloon stats (visibility) + DAMON (two ids; measured negative) + D6 idle cache return (`_ONLY` d@e + 734d0344) + D6 busy_cold (d@e, paper) + S6 elastic memory cap (paper 3072, d@e 2048, others by class) + D3 idle hibernate (10 min) + pressure reclaim (active). D6-M20 explicitly keeps DAMON and D6 on disjoint computers so readings do not blur (nix comment 1881-1884). Deleting DAMON removes the blur.
- FACT: VMCTL_PRIMARY_KEEPALIVE_MODE=under-capacity (code default) would keep primary computers warm and fight D3 idle hibernate and cache return; `off` is set only in nix.
- FACT: VMCTL_STOP_MANAGED_ON_EXIT=true contradicts "computers stay on" and S5 busy hibernate.

### 3.3 Snapshot/disk contention
- FACT: Diff snapshots, layered ring, snapshot_release_retain, the host disk guard (507 at <5% free), pkgmirror cache (2 x 16 GiB cap) and the off-host backup all draw on Node B's one volume (218G of 253G per memory ledger 2026-11-01). Turning mirrors on fleet-wide adds up to 32 GiB of cache demand (HYPOTHESIS: far less in practice, but it is the configured cap). Diff reduces snapshot bytes; cadence checkpoints add published objects.

### 3.4 Checkpoint vs snapshot (similar names, different mechanisms)
- FACT: "checkpoint cadence" (tape/store restore point, guest) vs "Diff snapshot" (VM memory image, host). Both are called restore points and both are produced by planned events; they should be read as two layers. HYPOTHESIS: cadence holds appends (bind) while a Diff pause freezes the VM; a Diff hibernate that lands during a bind is a contention the code may or may not serialize (not traced).

### 3.5 Gateway
- FACT: scheduler replaces the 60/min limiter; the guest still has its own 1.5 s research spacer and Retry-After loops; bearer binding enforce also answers 429 with Retry-After 2. Two clocks, three 429 meanings (problem doc item list; bearer doc line 745). dispatch.wide_slots (paper 12 slots) versus the scheduler's 6-per-computer default: HYPOTHESIS they conflict for the paper (see 2.12).
- FACT: body caps must be enforce before the scheduler is on (in observe, 24 x 576 MiB is no bound); caps are enforce, so the order constraint is met.

### 3.6 Network egress controls (ordering)
- Host fetch (guest process fetches), mirrors (capsule package fetches), tunnel identity (capsule CONNECT tunnels), guest DNS (name lookups), host egress floor (netfilter) all cut the same traffic from different angles. Ordering: pkgmirror ecosystems before mirror computers (same deploy); mirrors in route mode before MirrorsOnly; DNS rollback list-first, Unbound last (guest_dns.go); tunnel identity enforce must not name delegate computers before gate (c); host-fetch proof requires a test computer and an API key.
- FACT: MIRRORS_ONLY removes registries from capsule direct CONNECT, so tunnel enforce then only judges the remaining hosts (github, huggingface, claude API).

### 3.7 Delegate vs everything
- FACT: delegate on one computer is the experiment's design (two-arm). On-for-all contradicts it and the owner's spend rule. Delegate computers must not be in tunnel enforce yet. Layered tool profile gives delegate tools only to computers holding the delegate grant.

### 3.8 Event table family (telemetry destinations)
- FACT: store.event_table (host grant + setting) vs telemetry.batch (default on) vs store.telemetry_partitions (floor 5) vs tape.group_commit vs store.telemetry_retire: five mechanisms that each change where/how telemetry bytes land; three raise the store schema floor (stamping, partitions). Floors stack: a rollback past the highest floor boots nowhere. Deleting event_table removes one floor.

### 3.9 Superseded mechanisms still next to their replacements
- DAMON (VM_GUEST_RECLAIM_ONLY) next to D6 cache return and S6 cap.
- CHOIR_EVENT_TABLE_COMPUTERS next to batching/partitions/group commit; its rationale (prolly trees) is the old store.
- VMCTL_PRIMARY_KEEPALIVE_MODE (under-capacity) next to D3 idle hibernate.
- Single guest image next to the layered image (by design during migration).
- Retired setting keys store.debounced_checkpoint, store.trace_log (guest builder's).
- The env twin CHOIR_TELEMETRY_BATCH next to the setting telemetry.batch (default true) (guest builder's).
- PROXY_EXPAND private door next to the public door.
- Cache-return `_ONLY` generic selector next to `_BUSY_COLD_COMPUTERS` and `MEMORY_CAP_COMPUTERS`: one registry row (explicitly folded to stay under the release cap of 64) but three independent selectors with three ring states.
- Docs: stale nix comments (receipt compaction "dry-run" at nix:1128-1135 vs `active` values; cache return comment "never the paper" at nix:1866-1867 vs paper `active`; the visual capture header comment "OFF while the list is empty" vs four entries).

### 3.10 Overlaps with guest computer settings (what I can see from the host side)
- store.event_table (setting) needs CHOIR_EVENT_TABLE_COMPUTERS (host); decide together.
- texture.evidence_embeds needs the visual capture grant (CHOIR_VISUAL_CAPTURE_COMPUTERS), which another branch is deleting (inventory A5 says so). If the grant list goes away, the setting needs a new source.
- source.projection needs a host base fetch reachable (inventory); hostFetchVMs and the host-fetch route are the candidate path (HYPOTHESIS, not traced).
- dispatch.wide_slots (paper 12 slots) interacts with the gateway scheduler cap.
- tape.group_commit and telemetry.batch/partitions interact with the cadence's append hold (HYPOTHESIS).
- store.compact_force and PLATFORM_RECEIPT_COMPACT_COMPUTERS: both compaction levers, different stores (guest daily compact vs corpusd receipts); no direct conflict found.
- packages.publish/share and the host mirror: unrelated (different "mirror").

---------------------------------------------------------------------------

### 4. Summary table (recommendations, not actions)

| Switch | Rec | Ring / when | Confidence |
|---|---|---|---|
| VM_SNAPSHOT_DIFF_ONLY | ON | class:test now, `*` after an integrity soak | medium |
| VM_CHECKPOINT_CADENCE_ONLY | ON by ring, widen only after design 11.1 fix or cutover | d@e + class:test now; owner/paper blocked | high (not `*` yet) |
| VM_BALLOON_STATS_ONLY | ON | `*` (one cold boot, coalesce) | medium-high |
| VM_GUEST_RECLAIM_ONLY (DAMON) | DELETE | now | high |
| VMCTL_GUEST_CACHE_RETURN family | ON | idle `_ONLY=*` trivial; owner computer needs its own ladder before busy_cold | medium |
| VM_CAPSULE_TUNNEL_ENFORCE_ONLY | ON everywhere except claude-mode capsules | image enforce + small code exemption | medium-high |
| VMCTL_GUEST_DNS_ONLY | ON | `*`, with Unbound health check | medium |
| guestLayeredComputers | ON by ring | d@e, class:test, fleet | low-medium |
| PLATFORM_RECEIPT_COMPACT | ON (already) | fix stale comment, widen `*` | high |
| VMCTL_PRIMARY_KEEPALIVE_MODE | DELETE | now | medium |
| VMCTL_STOP_MANAGED_ON_EXIT | DELETE | now | high |
| gatewayModelScheduler | ON | global; run proof 9 as first staging act; consider per-computer 12 for the paper | medium |
| hostFetchVMs | ON at R0, then widen | proof never ran; needs `*`/computer-id code | medium-low for fleet-wide now |
| capsule mirrors | ON at R0 (route mode), then class:test, then MirrorsOnly | proof never ran; ecosystems + computers in one deploy | medium |
| CHOIR_EVENT_TABLE_COMPUTERS | DELETE (with store.event_table) | owner decision | medium |
| CHOIR_CLAUDE_DELEGATE_* | KEEP as experiment, fix registry expiry to 2026-11-17 | owner-stated spend limit | high |
| PROXY_EXPAND_REQUESTERS | ON (owner id) or DELETE private door | needs the owner user id | medium |
| CHOIR_APP_DESKS / appdesk | UNDECIDED; do not delete appdesk | not a small slice | medium-low |
| appnet + egressgrant | DELETE candidate (owner decision) | unwired, no switch in code | medium |
| CHOIR_COMPUTE_CAPSULES, CHOIR_EGRESS_GRANTS, AUTH_DEVICE_RELEASE | mark design-only in docs | docs | high |

CI deadline: on 2026-11-04 `check-switch-registry` errors for host_fetch_vms, choir_capsule_mirrors, proxy_expand_requesters, gateway_model_scheduler unless each is set on at R0 or deleted (FACT, script lines 307-313).

Unexamined by me (stated so it is not mistaken for a pass): the guest-side readers of checkpoint cadence and event table beyond their headers; guest_balloon.go's use of balloon stats; scheduleModelCall's deadline handling; the exact content of the registry rows for layered and for DNS history after D32; whether VM ids are stable across cold boots for the host-fetch list.

## Backlog walk with the owner (2026-11-03)

Each decided switch is deleted; "on" means the code runs everywhere.

**Group 1, Texture.**
- `texture.owner_directions`: on everywhere, the paper included.
- `texture.owner_act_wakes`: on everywhere; Texture wakes are debounced
  (owner: "we should already be debouncing. but shouldnt add more than a few
  seconds latency for users"), so a progress-report wake and an owner-act
  wake within the window run once; the window is a few seconds at most.
  Check whether a debounce already exists before adding one.
- `texture.evidence_embeds`: on with screens-as-files when that branch lands;
  RV-04 (an old client dropping embeds) fixed in the same change.
- `texture.root_desk`: on everywhere after the excerpt ordering (F4/F5,
  2026-10-25) is re-proved on d@e; on the paper, published-story documents
  feed the Master as one batched entry an hour instead of a refresh per
  commit.
- `texture.hold_trajectory`: on everywhere (earlier ruling).

**Group 2, engineering and self-development.**
- `engineering.progress_to_texture`, `engineering.slots_graph`: on
  everywhere; the co_super_slots table and its path deleted once the import
  has run on every computer.
- `engineering.carry_unfinished`: on everywhere, pruned to the last 3 carries
  per document, merged with the rejected-candidate carry into one offer (the
  newest unfinished or rejected tree).
- `source.projection`, `source.rebase_offer`: owner, "i think we need to
  accelerate library then this will resolve itself". The Library's package
  path (design/library-package-propagation-2026-10-26.md S6, package-aware
  containment and re-materialization) is pulled forward; projection goes on
  everywhere with it, and the rebase offer is deleted once package changes no
  longer need it.
- `dispatch.wide_slots`: the fixed activation cap is deleted; concurrency
  follows the computer's memory and CPU admission.

**Group 3, store and tape.**
- `telemetry.batch`: on (already).
- `store.telemetry_partitions`: on everywhere, the paper included, after the
  partition GC starvation bug (paper-compaction-2026-10-31.md: no retired
  partition file is deleted while a staged compact copy exists) is fixed;
  the store floor 5 is accepted like any schema step.
- `tape.group_commit`: deleted with its code (owner: "i dont like the sound
  of losing events just before a crash. those are among the most important
  events!").
- `store.event_table` and `CHOIR_EVENT_TABLE_COMPUTERS`: deleted, setting
  and writer; the existing table read until empty.
- `evidence.graph`: measured first on a copy of the paper's store (rows,
  bytes, write volume, memory, lock hold of the import), then decided
  (owner: "we just spent the last few days working to optimize storage and
  memory usage. cant undo that").
- `store.compact_force`, `management.serial_tasks`, `store.debounced_checkpoint`,
  `store.trace_log`: deleted.

**Group 4, host.**
- Balloon stats, guest DNS (with an Unbound health check), receipt
  compaction, layered images: on everywhere, one batched cold boot. DAMON
  reclaim, `VMCTL_PRIMARY_KEEPALIVE_MODE`, `VMCTL_STOP_MANAGED_ON_EXIT`:
  deleted.
- Diff snapshots: on everywhere. The Firecracker >= 1.15 check is deleted
  (the flake pins 1.17.0, so it is dead weight; owner: "protect against real
  issues. but not fake ones"). The swap, cold-origin and huge-page checks
  stay: they are what makes a mincore Diff correct (a skipped page must read
  as zero), and Node B has a swap device. No new wake hash check.
- Checkpoint cadence: the append hold is fixed first (a no-hold checkpoint,
  as the snapshot producer already does), then on everywhere.
- Capsule tunnel enforcement: one Claude-delegate capsule session shows a
  clean pass on the Claude arm, then enforce for every capsule; the observe
  mode is deleted.

**Group 5, packages, files, app desks.**
- `files.object` (after its reader fix), `app_packages.objects`,
  `packages.publish`, `packages.share`: on everywhere; publish and share are
  the Library's normal path.
- App desks (`appdesk`, `appnet`, `egressgrant`, `CHOIR_APP_DESKS`): wired
  and on, as part of the accelerated Library work.

**Group 6, Autopaper.**
- `intake.event_wakes`: not deleted. Owner: "lets get to the bottom of this
  ... i dont want to remove stuff which may just need fixing". A root-cause
  investigation of the three false-merge failures goes to the Autopaper
  section that follows progressive delivery and self-development.

**Group 7, the paper's features and the delegate.**
- `intake.events`, `intake.labels`, `research.system1`,
  `research.related_story`: the switches go. Owner: "these features should,
  on ordinary computers, still be able to access paper apis. eg research
  related stories of a topic is useful regardless of your endpoint. intake
  may not be relevant outside paper though. rather than switches there can
  be indefinite divergence of computers. thats the point of this
  architecture, indeed." Research's System 1 and related-story reads run on
  every computer against the paper's public APIs; intake runs where source
  schedules exist. Computers differ by what they run and adopt, never by a
  platform flag.
- Claude delegate: the nix computer list and expiry are replaced by an owner
  act per computer (grant and revoke, with a spend cap, recorded on the
  tape). It is a permission, not a feature switch.

**Order (owner):** ring-ordered deploys and automatic rollback first; the
backlog deletions then ride ring deploys in batches.
