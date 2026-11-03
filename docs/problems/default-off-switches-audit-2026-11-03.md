# Default-off switches: audit and flip (2026-11-03)

Status: fixed in part (this change). Mutation class: red (the defaults of
guest computer settings change fleet behaviour at each computer's next read;
Texture canonical writes are among them, through evidence embeds).

## Problem

Owner, 2026-11-03 ~21:00Z: "nothing is off by default. thats the old wrong
pattern of all these hidden switches." The trigger was
`texture.evidence_embeds`, which defaulted to false, so Texture never
embedded a Freeze's screenshots in the owner's documents. The Library had
the same problem: `packages.publish` defaulted off (`packages_publish_off`).
`packages.share` had the same problem too; 8e2814b6f retired it, because a
published release is now public.

Evidence: before this change, `KnownComputerSettings` held 28 keys at
origin/main before 8e2814b6f (24 in `computer_setting.go`, 4 added by
`init()`), and 26 of them were false. Of those 26:
- 21 were features, product behaviour that only a stored owner act could
  turn on;
- 3 were operator levers;
- 2 were retired keys. The registry
(`switches/registry.yaml`) recorded most of them as "ON NOW" for one or two
computers. Per-computer stored values are not visible from the repo, so
"on at X" below is the registry's claim.

## Classes

- (a) feature switch: product behaviour behind a flag. Owner rule: on by
  default.
- (b) correctness refusal: crash and restart rules, live capsule, release
  skew, auth. Stays.
- (c) operator safety bound: a budget, a rate, a capacity, a proof or
  rollback lever. Stays.

## Guest computer settings (`KnownComputerSettings`)

### Flipped to on by default (this change)

| Setting | What it does | Notes |
|---|---|---|
| texture.evidence_embeds | A fresh Freeze's screens are embedded in the operation's Texture document | Fleet effect below |
| texture.owner_directions | Texture turns see the owner's later directions | Owner: on everywhere, the paper included |
| texture.hold_trajectory | Texture continues its earlier conversation | Owner: on everywhere. It costs more input tokens per Texture run, bounded by the run's own compaction; the paper runs Texture most |
| engineering.progress_to_texture | Engineering Reports also reach the Texture inbox | |
| engineering.carry_unfinished | A stopped round's patch is kept and offered to the next round | Residual: kept patches are never pruned (at most `capsule.CarriedPatchMaxBytes` per stopped round). The owner wants the last 3 per document |
| engineering.slots_graph | Slot claims on the tape, not co_super_slots | The first claim imports co_super_slots, which holds at most one row per live trajectory and desk slot. evidence.graph stays held because its table is unbounded research evidence |
| app_packages.objects | The owner's route to package kinds | |
| files.object | The Files API and the 15-minute reconcile record choir.file objects | The only writer (`ApplyFileChanges`) read "nothing stored" as off and ignored the default. Fixed so it reads the default. The first reconcile on each computer writes one object per workspace file; later passes hash only files whose stat changed against the recorded objects. Not measured on the paper |
| packages.publish | The owner may draft a package release of an applied operation | No correctness refusal is lost. The draft still refuses an unapplied operation (`operation_not_applied`), and publishing is still an authenticated owner act with its `--confirm-paths` preview of what becomes public. The registry's b@c first-publish gate is superseded by the owner's rulings |
| intake.events | Jev resolves captures onto events (shadow) | Runs only where source polls capture |
| intake.labels | Jev's labels on captures, plus management's intake text and verbs | A stored value decides. With nothing stored, the default applies only on a computer that has a source schedule (owner: "intake runs where source schedules exist"; `intakeLabelsEnabled`). A computer without sources keeps management's prompt as before. The paper has the setting stored on |
| research.system1 | The research desk may call System 1 with the default caps | Owner (switch audit, group 7): System 1 runs on every computer. Spend is bounded by the caps (40 per run, 3000 per day). Runtime-config caps still win over the setting, as before |

Already on: `telemetry.batch`, `store.telemetry_retire`.

The paper already has `intake.events`, `intake.labels` and
`research.system1` stored on (registry ring R5), so those three flips add
no spend there.

Rollback: a stored off on one computer (POST /api/computer-settings),
read live, with no deploy or restart. A revert of this commit restores the
old defaults fleet-wide at the next deploy.

Removed, because nothing read them: `store.debounced_checkpoint` and
`store.trace_log`. Stored rows stay inert, as for the earlier removed keys
(TestStoredRowForARemovedSettingIsInvisibleAndHarmless), and their
registry rows are deleted.

### texture.evidence_embeds across the fleet

Which computers embed: only computers whose engineering Freezes save
screens. That needs the host visual-capture grant on the capsule
(`CHOIR_VISUAL_CAPTURE_COMPUTERS`), so the flip changes nothing on a
computer without the grant. The path is in
`settleFreezePrecommits` (`screens > 0`).

Cost: one Texture revision per fresh Freeze with screens. The revision
carries embed nodes that reference asset digests, not image bytes. There
are at most `capsule.MaxScreens` (12) screens of at most 8 MiB each, and
those bytes are already in the asset store. The embed pins them, so they
are held as long as the revision. A second embed of the same operation
writes nothing (the head check).

The paper (computer-1355b83b): it runs Texture for its story documents,
but it does not run self-development Freezes with screens, so no embed is
written there. If one ever were, it is the same bounded revision: at most
12 digest-referencing embed nodes, checked by `texturedoc.Project`. No size
or limit risk was found.

RV-04 (an old client drops embeds) is moot today. The frontend learned
embeds on 2026-10-24 (6db4736bd). Every guest, the paper included, has had
a planned refresh since then: the paper migrated at boot on 2026-11-01
(store split ledger). The frontend is served from main. No guard was
added.

### Not flipped: features held, with the reason and the fix

| Setting | Why on-by-default would break or contradict today | Fix that makes on safe | Owner decision |
|---|---|---|---|
| texture.root_desk | The owner set two conditions (switch audit 2026-11-03, group 1). The F4/F5 excerpt order must be re-proved on d@e. On the paper, published-story documents must feed the Master as one batched entry an hour; that is not built, and today every story commit arms a digest (up to 20 root runs an hour) | Build the paper's hourly batch, re-prove F4/F5 on d@e, then default on | "Turn root desk on now on owner computers by a stored setting while the batch is built?" |
| research.related_story | Its finish rule (`requireStoryEntitiesRecorded`, `recordsInboxPrompt`, `storyAnalysisFor`) treats every `research:<doc>` desk as a story desk. Default on would hold owner research rounds to record entities for a document that was never published | Key the story rules on "the document is a published story", not on the setting. Then default on, so related-story reads work on every computer (owner: "useful regardless of your endpoint") | "Approve keying the story rules on publication, then delete the setting?" |
| evidence.graph | The owner asked for this to be measured first: "we just spent the last few days working to optimize storage and memory usage. cant undo that". The first write imports the whole agent_evidence table inline, under graphHomeMu, which slot claims also take | Measure on a paper copy (rows, bytes, lock hold), then bound and background the import | "Decide after the paper-copy measurement" |
| store.telemetry_partitions | GCTelemetryPartitions returns 0 while a staged compact copy exists, so a computer that never restarts never deletes retired partition files. Turning it on writes store floor 5 on every computer, the paper included | Fix the GC starvation (compare the copy's head with the retire op's sequence); then default on (owner: floor 5 accepted) | "Default on once the GC fix lands?" |
| tape.group_commit | Owner ruled delete: "i dont like the sound of losing events just before a crash" | Delete it and its code | Decided: delete |
| store.event_table | Owner ruled delete. It also needs the host grant `CHOIR_EVENT_TABLE_COMPUTERS` (empty), so a guest flip does nothing | Delete the setting and the writer arming; keep the table until it is empty | Decided: delete |
| intake.event_wakes | Failed its false-merge bar three times (7.0%, 9.5%, 15-30%) | Root-cause the false merges (owner: investigate, do not delete) | Investigate |
| calibration.feedback | Today it gives management, Texture and research a digest. The owner ruled engineering first, and other desks wait for engineering's results (docs/design/calibration-seed-2026-11-03.md). Engineering is not yet a digest desk (`calibrationDeskOfThread`), and the per-computer hash salt is not in place | Add engineering to the digest, move the hash to a per-computer salt, then make the digest on with no setting (the A/B split is the experiment) | "Start the engineering arm?" |

### Kept: (b) and (c)

| Setting | Class | Why it stays |
|---|---|---|
| management.serial_tasks | (c) rollback lever | On means K = 1, the S2b rollback. Owner ruled delete (not done here) |
| store.compact_force | (c) proof lever | Off is the normal state on every computer but d@e. Owner ruled delete (not done here) |
| dispatch.wide_slots | (c) capacity bound | 8 to 12 shared activation slots. Owner: replace with memory and CPU admission |

## Host and guest environment (nix/node-b.nix, nix/autoputer-vm.nix)

Not changed in this change. Most are `*_ONLY` or `*_COMPUTERS` selectors
that are on for at least one ring (R0 d@e or class:test), which is a ring
walk, not off everywhere.

Widening a selector to `*` is a host deploy with a cold boot per computer,
because the launch-config hash moves. The owner wants those batched into
one ring deploy (switch audit 2026-11-03, group 4).

(a), on for at least one ring, widen in the batched host deploy:
- VM_SNAPSHOT_DIFF_ONLY;
- VM_CHECKPOINT_CADENCE_ONLY (waits on the append-hold fix);
- VM_BALLOON_STATS_ONLY;
- VMCTL_GUEST_DNS_ONLY (waits on an Unbound health check);
- VMCTL_GUEST_CACHE_RETURN_ONLY and its BUSY_COLD list;
- VM_GUEST_LAYERED_COMPUTERS;
- PLATFORM_RECEIPT_COMPACT_COMPUTERS;
- CHOIR_CAPSULE_MIRROR_COMPUTERS (d@e);
- CHOIR_HOST_FETCH_VMS (d@e);
- VMCTL_JAILER_ONLY.

(a), off everywhere, not flipped:
- `CHOIR_EVENT_TABLE_COMPUTERS=` is the host half of store.event_table.
  Owner ruled delete.
- `CHOIR_APP_DESKS` and the unwired appdesk, appnet and egressgrant
  packages are wired as part of the Library work. Turning them on today
  does nothing, because nothing imports them.

(b), security enforcement or safety (stays):
- guest `CHOIR_CAPSULE_MEMORY_POLICY`, `CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY`,
  `CHOIR_DOCUMENT_TOOL_SANDBOX` and `CHOIR_RUNTIME_CAPABILITY_FLOOR`, all
  `observe` today. Enforcement is proved on d@e before it ships;
- `VM_CAPSULE_TUNNEL_ENFORCE_ONLY`;
- `CHOIR_OG_WRITER_GATE` (code default observe);
- `CHOIR_CAPSULE_MIRRORS_ONLY=` (forces mirror-only egress);
- `VMCTL_STOP_MANAGED_ON_EXIT=false` (false keeps computers running across
  a vmctl restart);
- `RUNTIME_ENABLE_TEST_APIS` (test APIs stay off in production);
- `MAILD_SEND_GATE_MODE`, `PROXY_OBJECT_GRAPH_INTERNAL_AUTH` and the auth
  modes (all on or enforce).

(c), operator bounds and modes (stays):
- `VMCTL_PRIMARY_KEEPALIVE_MODE=off` lets primaries hibernate (the
  resource-first rule);
- `VM_GUEST_RECLAIM_ONLY` (DAMON; measured negative, owner ruled delete);
- the admission, pressure, retention, GC, rate-limit and search caps;
- `RUNTIME_MAINTENANCE_HOLD` and `RUNTIME_RECOVERY_REPLAY_ONLY` (operator
  modes);
- `CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH` (a kill, default on);
- `CHOIR_CLAUDE_DELEGATE_COMPUTERS`, which spends the owner's money. It is
  a permission, and the owner wants it as an owner act, not a flag.

## Go `if !enabled` gates

Every guest feature gate reads a computer setting through
`computerSettingOn`, `graphHomeOn`, `textureSettingOn` or the autoputer
readers. When nothing is stored, all of them return the declared default.
The exceptions:
- `ApplyFileChanges` read "not stored" as off (fixed here);
- `intakeLabelsEnabled` also requires a source schedule (this change);
- `graphHomeOn` reads off on a store with no projection tape, which is
  deliberate: a graph is only a home once it is on a tape;
- a store read error reads off everywhere (fail-closed, unchanged).

`event_wakes.go:349` reads the stored row after the generic check. It is
moot while `intake.event_wakes` stays off.

## Tests

- `internal/store/computer_setting_defaults_test.go` (written before the
  flip):
  - TestFeatureSettingsDefaultOn;
  - TestHeldSettingsKeepTheirOffDefault, which also fails when a new
    setting is not classified;
  - TestRetiredStoreSettingsAreRemoved;
  - TestFileObjectWriterHonoursTheDefault.
- Updated tests that pinned the old defaults: the file-object, app-package
  route and Texture hold/directions fixtures now store an explicit off
  where they test off.
