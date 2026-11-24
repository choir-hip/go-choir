---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-22: "hmm, we should be able to rearchitect this to be 2x as efficient? or maybe more because the store is copying the corpus a lot too, no?"; "we will offload the tape soon. for now, lets do the other efficiencies"; "so, the new storage work should be written up as a new throughline mission added to the metamission".'
review:
  reviewer: none (the owner's statement is the authority; each station names its own review tier)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-22T16:31Z
  source:
    canonical_ref: origin/main c8d48a589 (docs); the last code commit is aefdfbbc4
    deploy_identity: staging and the paper on c27a50d98; a deploy of main (CI run 45044023185, carrying 34bc283ee and aefdfbbc4) was dispatched at 16:28Z and had not finished
  what_is_true_now:
  - 'Node B has one 476 GiB disk. On 2026-11-22 it fell to 44 GiB free: every deploy was refused under the 60 GiB floor (scripts/node-b-deploy-disk-preflight) and hibernated computers were refused on wake (docs/problems/compact-copy-staged-on-a-51-gib-store-holds-node-b-under-the-deploy-floor-2026-11-22.md).'
  - 'One computer, the paper, holds most of it: a 51.5 GiB store, a 47.7 GiB compact copy staged beside it, 89 GiB of tape segments, 34 GiB of snapshot chunks, and its share of 41 GiB of capture bodies. Postgres holds 41 GiB, of which the event receipt table is 28 with about 12 live.'
  - Two routine operations each write a second copy of the store. The daily snapshot copies the store to a scratch file (7m17s), computes a logical digest of every row (54m51s) and uploads chunks (4m15s; 79% of chunks new after a day). The weekly layout compaction stages a compact copy until a planned restart, which only a deploy performs (internal/store/compact.go).
  - 'Read from code, not run: a restore recomputes the same whole-store digest, and a rebuild from tape computes a second one after replay, so either on the paper would run until its caller''s context cancels (10 or 30 minutes), with no fallback. restoreRequestBudget is used only by a test.'
  - 'What the store is made of is not measured. The by-table reading stops at about 95 s with 15 of 51.5 GiB attributed: object bodies 6.6 GiB or more, objects 5.4 or more, run memory 2.2 or more; 36 GiB unattributed. The store doubles in about eleven days (docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md).'
  - 'A hibernated computer''s image keeps blocks its guest has freed: the b@c test computer uses 8.4 GiB and its image holds 19.8.'
  - 'Done by hand on 2026-11-22 with the owner''s word: five computers removed and their disks deleted; the paper''s three oldest snapshot roots removed (their chunks leave through the hourly artifact collector). Free space was 70 GiB at 16:28Z. The tape segments and snapshot chunks of the five removed computers are still on the host.'
  - '2026-11-24: why two test computers hold 9 and 13 GiB is recorded in docs/problems/computer-disks-hold-self-development-leftovers-2026-11-24.md: discard works (on since 2026-10-22); the space is self-development leftovers with no remover (candidate source copies, capsule receipts and logs, rematerialize quarantines). A fresh computer holds 0.5 to 0.7 GiB.'
  branches_in_flight:
  - 'lane/store-by-kind: a background store reading by object kind (S0), building.'
  - 'lane/no-layout-compact: the layout trigger deleted (S1), building.'
  - 'lane/snapshot-one-pass: the snapshot reader for a page digest (S2), building after a written check of the design against the code.'
finish:
  deliver: No routine operation writes a second copy of a store; a restore of the largest store finishes inside its budget; what the store holds is measured by kind and its largest avoidable part is removed; the deploy floor is computed from what the computers can actually grow by.
  artifact: 'on main and staging: the layout trigger deleted; snapshot records of version 2 with a page digest, captured in one pass with no scratch file; a restore path proved on d@e for both record versions; the store reading by kind and its report on the paper; the preflight''s compaction constant replaced; a design, from the reading, for what leaves the store.'
  acceptance:
  - action: read the paper's data volume use and Node B's free space from the host through one whole week that contains seven snapshot captures
    proves: the paper's data volume never holds more than its store plus 6 GiB, and no deploy in that week is refused for disk
    evidence_class: deployed count
  - action: the snapshot producer's own log lines for seven consecutive captures of the paper
    proves: each capture completes, in under 30 minutes, with no scratch file and a read transaction under its pin deadline
    evidence_class: deployed count
  - action: on d@e, restore from a version 2 record and from a version 1 record, and compare the restored store's logical digest with the source's at that head
    proves: both record versions restore the same rows; a flipped byte is refused
    evidence_class: deployed drill
  - action: run the store reading by kind on the paper to completion
    proves: every GiB of the store is attributed to a table and an object kind, with version depth and duplication
    evidence_class: deployed count
  rollback: each station is one commit that reverts. After S3 an old binary skips version 2 records and restores an older base with a longer tail, bounded by PlanRecovery; S2 is on every computer before S3 lands.
  landing:
    required: true
    environment: staging
    required_receipts:
    - pushed_commit
    - ci
    - deploy
    - environment_identity
    - deployed_acceptance
value:
  better_means: the disk a computer needs falls from about three times its store toward one, a deploy is never refused because of a copy the system made of itself, and the store's growth is explained by kind so the next cut is chosen from numbers
  goodharting_would_be: free space won by deleting things once by hand; a lowered deploy floor; a copy moved from the guest to the host; a snapshot that is fast because it no longer proves what restore needs; a measurement nobody acts on
homotopy:
  realism_axis: from the copies (deleted outright), to the snapshot (reader first, then the capture), to what the store holds (measured, then designed, then moved); the same store file, record and restore path at each step
boundaries:
  mutation_class: red (the store's file, snapshot capture and restore, checkpoint and recovery); S0 is orange
  authority_sources:
  - the owner's words above; CLAUDE.md; docs/computer-ontology.md
  owner_rulings:
  - '"we will offload the tape soon. for now, lets do the other efficiencies" (2026-11-22). Reading: the tape''s place on the host is not this mission''s to change.'
  - '"Retire it and remove old paper snapshots"; "yes, we should delete the extra computers. we can make more later. only [the owner''s own account] has real data. b@c and d@e can be reset" (2026-11-22). Reading: those two acts were authorised once; neither is a standing permission to delete.'
  - '"we should design for user computers being always accessible. todays storage issues wont remain" (2026-11-22).'
  - '"Yes, let''s do these wins in the background... The storage stuff can be done before and while we do other stuff I think" (2026-11-23), answering the lead''s list: the one-pass snapshot, snapshots that share more, the store reading run on the paper, story versions, the receipt table rewrite, the small leftovers. Reading (the lead''s): Q1 is settled as its default; Q3 is yes in a quiet hour; Q2 (deleting the removed computers'' tape and chunks) cannot be undone, so the lead names it to the owner again before doing it.'
  - decision: the capture stops computing the per-row logical digest; the corruption check runs apart from capture on a slower schedule (weekly)
    said: 2) yes,delete tape of deleted computer. otherwise i agree with recommendations
    'on': '2026-11-24'
  - decision: 'the tape segments and snapshot chunks of the five removed computers are deleted from the host. Lead''s constraint, not the owner''s words: a dry run lists each computer''s id, size and last activity first, the deletion is a reviewed operator command, and nothing of a computer that still has an ownership record is touched'
    said: 2) yes,delete tape of deleted computer. otherwise i agree with recommendations
    'on': '2026-11-24'
  - decision: the event receipt table is not rewritten now; revisit when the host's free disk is under 100 GiB
    said: 2) yes,delete tape of deleted computer. otherwise i agree with recommendations
    'on': '2026-11-24'
  - decision: the paper's snapshot budget stays at 90 minutes
    said: 2) yes,delete tape of deleted computer. otherwise i agree with recommendations
    'on': '2026-11-24'
  - decision: 'self-development leftovers: measure first; the collector does not run at boot, it waits some time after boot; the other options (quarantine retention, patch-not-copy, the ceiling alarm) agreed in the order given'
    said: i like measuring. i dont like running the collector at boot. should wait some time after boot. ottherwise i agree with storage options
    'on': '2026-11-24'
    reading: 'lead''s reading: the collector runs once the computer has been up and idle for a set time, never in the boot path, so a boot is never slowed or failed by it; quarantine deletion still shows the owner real numbers before anything is removed'
  must_preserve:
  - the paper publishes throughout; nothing changes on the paper or another user's computer except by deploy; drills run on d@e only
  - snapshotDue, the capture floor, the backoff and the 540,000-event ceiling are not touched (three review rounds failed there on 2026-11-15)
  - a crash restart never resumes work; a compact copy is swapped only at a planned restart
  - the deploy floor is not lowered by hand; no switches
  - no secrets or the owner's email in the repo or output; the host token is used only through its header file
  excluded:
  - moving the tape off the host (the owner's, soon)
  - resetting the two test computers
  - a larger disk or a second host
stations:
- id: S0
  name: The store measured by object kind
  scope: 'a background reading inside the runtime that finishes on a 51 GiB store: sizes and counts by table, object kind, version depth, duplication and run-memory age, never content; run on the paper and reported'
  class: orange
  review: one seat
  depends_on: []
  lane: lane/store-by-kind (landed 0f4421aae); run on the paper 2026-11-23T21:45Z to 22:02Z, all 30 tables complete, docs/evidence/store-composition-paper-2026-11-23.md
  status: proved
- id: S1
  name: The layout trigger deleted
  scope: a compact copy is due only by free pages or when forced; the staged copy already on the paper is handled as today at a planned restart
  class: red
  review: two seats
  depends_on: []
  lane: lane/no-layout-compact (landed 06c49c1b8)
  status: landed
- id: S2
  name: The snapshot reader for a page digest
  scope: record version 2 carries a digest of the pages in order and no logical digest; restore verifies it as it assembles, then the head; version 1 is verified as today; anything else is refused in restoreOne
  class: red
  review: two seats
  depends_on: []
  lane: lane/snapshot-one-pass (landed 9e3e2060f)
  status: landed
- id: S3
  name: The capture in one pass
  scope: the state file streamed inside the head's read transaction with the digest inline; the scratch copy, its cross-check and the capture's logical digest deleted; a pin deadline; the free-space check becomes the floor plus WAL headroom
  class: red
  review: two seats
  depends_on:
  - S2
  lane: lane/snapshot-capture-s3
  status: working
- id: S4
  name: Restore and rebuild inside their budgets
  scope: 'design first: the whole-store digest a rebuild from tape computes after replay (a checkpoint-binding claim), and a version 1 restore on a large store; what replaces the corruption check the capture''s digest gave'
  class: red
  review: two seats
  depends_on:
  - S2
  status: pending
- id: S5
  name: The deploy floor from real growth
  scope: the preflight's constant 14 GiB compaction term replaced by the sum of what vmctl says each restarted computer's image may still grow; the remaining freelist-triggered copy gated on host room
  class: red
  review: two seats
  depends_on:
  - S1
  - S3
  status: pending
- id: S6
  name: What leaves the store
  scope: 'from S0''s report: a design for the largest avoidable part (captured text the host also holds, superseded versions, old run memory), then its stations. Measured 2026-11-23: every Texture revision is a full copy (content, body document, sources, citations) and nothing prunes them; public permalinks read pinned versions in the platform graph, not these rows, so old private bodies can go or become differences without breaking a permalink; what still reads old revisions is the history list, a revision by id, the base of the next turn, and the oped and calibration paths. On the public side superseded versions hold about 315 of 443 MB.'
  class: green for the design; red after
  review: a panel on the design
  depends_on:
  - S0
  status: pending
- id: S7
  name: The host's leftovers
  scope: 'recorded and, where the owner rules, removed: the tape and chunks of removed computers (about 12 GiB across 23 non-active directories; the host has no record of which five were removed, so the ids come from the owner before any deletion); the dead space in the event receipt table (28 GiB, about 8 returned; a plain rebuild stalls every computer''s appends, so only a copy-and-swap near 03:00Z, held by the lead while the disk has room). The third item is closed: vmctl already has discard on the data drive, the guest trims hourly, and the one hibernated image has no slack.'
  class: black for a deletion; orange for the trim
  review: two seats and the owner
  depends_on: []
  status: pending
- id: S8
  name: Snapshot chunks that are shared between days
  scope: 'chunks are cut at fixed 4 MiB offsets of the state file; about 0.2% of pages change in a day but scattered, so 82% of chunks were new between the roots of 11-21 and 11-22 and a capture writes about 8 GiB. Cut at 256 KiB: an estimated 0.7 GiB per later capture and about 12 GiB fewer stored chunks, for a manifest of about 13.5 MB a root and 16 times the chunk files. Restore refuses a chunk over twice the chunk size, so records cut at 4 MiB keep their own bound. Measure sharing on a copy of a real store before choosing the size.'
  class: red
  review: two seats
  depends_on:
  - S3
  status: pending
- id: S9
  name: The tape's payload bytes
  scope: 'recorded, not built here: the paper''s tape is 91 GiB in 2,157 segments, 69% sealed payloads, 20% receipts, 12% events; 13.81 of 13.9 million events are projection batches; nothing retires tape by time. A shared dictionary for payload compression in the guest might save 15 to 30% of new payload (a guess). Design only, after the tape''s move off the host, which is the owner''s.'
  class: green for the design; red after
  review: a panel on the design
  depends_on: []
  status: pending
decisions_for_the_owner: []
now:
  status: working
  as_of: 2026-11-23T21:33Z
  slice: S3 building in lane snapshot-capture-s3; a read-only measurement of the disk, the store reading, snapshot sharing, story versions and the receipt table running
  source_ref: origin/main c8d48a589
  deploy_identity: staging and the paper on c27a50d98; a deploy of main in flight
  candidate: none frozen
  conjecture:
    claim: with the layout trigger deleted and the capture in one pass, the paper's first two captures after S3 deploys each complete in under 30 minutes and the data volume's use during them stays under the store's size plus 6 GiB
    test: the snapshot producer's log lines and the guest's data-volume reading for those two captures
    edge: resource
    edge_note: the read transaction now lasts as long as the upload, so a slow host or a busy paper stretches the pin; the 8 to 12 minute estimate is from rates measured with a scratch file, not without one
    observer_upgrade: the pin's length and the WAL's peak size printed by the capture itself
    scope_if_supported: S5 may compute the floor without a second-store term
    ledger_id: C195
    status: untested
  believed_state: 'S0, S1 and S2 are landed. The weekly layout copy is gone. The daily snapshot still writes a scratch copy until S3 lands (free space dipped to 106 GiB during it on 2026-11-23; 155 GiB free at 21:30Z). The store reading was started on the paper at 21:45Z on 11-23. Disk by kind at 21:40Z: tape 112 GiB (paper 91), the paper''s store 57, capture bodies 42, Postgres 42, snapshot chunks 32.'
  main_uncertainty: 'which indexes hold the 21 GiB of the file that is not stored values (stored values are 30.2 of 51.6 GiB: object bodies 16.0, run memory 7.5, object headers 5.7)'
  next_observation: S0's report on the paper; S1's and S2's commits
  blocker: none
  next_action: review S3 (two seats) and land it; then S8; read S0's report from the paper when it finishes (about 05:45Z on 11-24 at the latest) and write S6's design from it; the receipt table swap is held while the disk has room
operating_notes:
- One landing and one deploy at a time, in the metamission's order; S1 goes early because it stops the next staged copy.
- A snapshot or restore test never runs against the paper. A restored store's equivalence is proved on d@e.
- Review a code diff, not prose, and name the exact base commit.
receipts:
- 'problem records: docs/problems/compact-copy-staged-on-a-51-gib-store-holds-node-b-under-the-deploy-floor-2026-11-22.md; docs/problems/paper-snapshot-aborts-at-its-time-budget-and-its-scratch-copy-blocks-deploys-2026-11-22.md; docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md'
- 'earlier designs: docs/design/capture-static-copy-2026-11-15.md; docs/design/paper-compaction-2026-10-31.md; docs/problems/incremental-snapshots-design-three-review-rounds-structural-assessment-2026-11-15.md'
---

# Storage efficiency: no store-sized copies, and a store that is measured

On 2026-11-22 Node B ran out of room because the system copies its own
largest file twice as a matter of routine. This mission removes those copies,
makes restore fit its budget, measures what the store holds, and then cuts
its largest avoidable part. Moving the tape off the host is the owner's and
is not here.
