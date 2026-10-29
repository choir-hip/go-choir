# Choir data architecture selection (2026-10-28)

```yaml
definition_version: throughline/1
readiness: reviewed
review:
  - round: 1
    reviewer: Devin + DeepSeek v4.1 flash (bunch379)
    frozen_ref: ee275750e
    verdict: send_back
    evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch379
    adjudication: >-
      All findings adopted in round 2 (this text): incumbent control;
      bounded replay; hard gates incl. a substrate-neutral witness and a
      differential semantic test before any ratification; parent
      invariants inherited; projector harness named as a build step;
      census by identity kind and rates; pre-registered decision rule;
      receipt corrections; dedup and multi-host boundaries; the
      stop-the-bleeding track kept in parallel.
  - round: 2
    reviewer: Devin (bunch380)
    frozen_ref: 2c5944649
    verdict: send_back
    evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch380
    adjudication: >-
      Adopted in 33d9771ed: online rollback-safe schema gate; cadence
      commits, not per write; squash phase unmeasured; deploy receipt;
      frozen base plus tail; witness gate detail.
  - round: 3
    reviewer: Devin (bunch381)
    frozen_ref: 33d9771ed (includes the owner's multi-host, off-host and
      scale-horizon additions)
    verdict: accept
    evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch381
    adjudication: >-
      Two nits applied (this ledger; the store list). The owner's
      bounded-peak-memory gate (c9449418a) landed after the frozen ref; it
      tightens a gate and is owner-authored; a one-seat check rides the
      next review.
  - round: 4
    reviewer: Devin (plan review 1)
    frozen_ref: 038adc19b (sha256 67c276a9...; includes the owner answers
      through the revised roadmap)
    verdict: send_back
    evidence_ref: .agentic-consensus/data-arch/plan-review/out/devin.out
    adjudication: >-
      All adopted in e378c3a1d: B1 tape-schema claim corrected (schema v2
      is its own red goal), B2 migration is its own goal, B3 byte
      deletions placed, B4 candidate floor and witness spike, B5 manifest;
      non-blocking items in the amendments section. The owner's
      bounded-peak-memory gate was in scope and drew no finding.
  - round: 5
    reviewer: Devin (plan review 2)
    frozen_ref: e378c3a1d (goal sha256 262c2277..., ledger sha256 20c4b4af...)
    verdict: send_back (round-1 findings all verified resolved)
    evidence_ref: .agentic-consensus/data-arch/plan-review2/out/devin.out
    adjudication: >-
      N3 corrected in the amendments; N1, N2 and the non-blocking ledger
      items answered by round-0b revision records in the ledger (round-0
      records stay unedited). Every finding was doc-level wording in a
      green file, so per the review tiers no further round runs; the next
      review is the one that gates execution.

start:
  source: main 0bf89ed84, 2026-10-28 ~12:30 UTC (staging /health deployed_commit 0bf89ed84, observed by FP0 after CI success)
  worktrees: none in scope
  facts:
    - Three Dolt stores. Per-computer embedded (the paper 26.5 GB after
      the 10-28 compaction, capture report). Corpus server (12.6 GiB,
      btrfs du on Node B by FP0 2026-10-28). Platform Store A server
      (50.9 GiB and 120,335 commits, btrfs du and dolt_log count by FP0
      2026-10-28).
    - Dolt versioning is read by no product path found: Texture history
      is an explicit parent chain of content-addressed rows
      (internal/store/texture.go:1235); restore treats the Dolt head as
      an audit receipt (internal/selfdevprotocol/restore.go:28); no
      production DOLT_BRANCH/CHECKOUT/MERGE or AS OF read.
    - It is not incidental. Commits exist deliberately for AS OF
      addressability (internal/store/store.go:216,
      internal/store/checkpoint_cadence.go:18). The Texture history read
      path runs a dirty-gated Dolt checkpoint (internal/store/texture.go:1259).
      The Store A squash consumes the commit graph (squash.go:92,175,184,211).
      Witness v2 uses DOLT_HASHOF_TABLE
      (internal/computerversion/dolt_table_hashes.go:32-40). Dolt branch
      isolation was once settled for candidate worlds
      (dolt_branch_isolation_pinned_test.go); the current ontology uses
      capsules.
    - Costs paid: snapshot-commit history at a cadence
      (defaultFinalizeCheckpointEvery = 100, internal/store/store.go); the
      Store A squash OOM at 10G (phase unmeasured; parallel track); the online GC on the write connection (81% of
      the paper's non-200 health samples); compaction; a two-hour capture
      with about 72 GiB of clone growth.

finish:
  deliver: >-
    A data architecture for Choir chosen by measurement against owner
    budgets and hard gates, proven semantically equivalent on a shadow
    projection before the owner ratifies it, with a migration plan.
  artifact: >-
    docs/design/data-architecture-decision-<date>.md: the census, the
    budgets (owner-edited), the hard gates, the pre-registered decision
    rule, the candidate set with the incumbent as control, the scorecard,
    the differential result, the panel adjudication, the migration plan.
    Also a projector harness in the repo that replays a frozen tape window
    into any candidate. It doubles as a permanent storage regression test.
  acceptance:
    - the scorecard reproduces from the harness at a frozen tape head and
      receipt key;
    - the winner passes every hard gate, including the differential test;
    - the owner ratifies (provisional ratification is allowed only before
      the differential test, and is binding only after it).
  rollback: docs, read-only measurement and an offline harness; nothing live changes.
  landing: harness code follows the landing loop; no live store changes in this goal.

value:
  better_means: >-
    Minimize per-computer memory, disk, write amplification, checkpoint
    cost and reader blocking against the incumbent measured the same way,
    with no hard gate lost.
  goodharting_would_be: >-
    Choosing by a synthetic benchmark or by preference; ratifying on the
    performance scorecard before the differential test; or a candidate
    that wins by dropping durability, the witness, crash semantics,
    Texture's single writer or per-owner isolation.

homotopy:
  realism_axis: >-
    Fidelity of the workload a candidate is judged on: a frozen d@e tape
    window, then a sampled window of the paper's tape with extrapolation,
    then the paper's base 5a241cb0 plus its tape to a frozen head
    (receipt key named) for at most two survivors, then a shadow
    projection on d@e (this goal ends at the d@e shadow differential;
    the migration is its own goal).

boundaries:
  mutation_class: green/yellow (docs, read-only measurement, offline
    harness). The migration it plans is red and is its own goal.
  must_preserve (inherited from the metamission):
    - a computer stays on while it is working; crash restarts close work,
      planned restarts resume
    - the paper and the owner's computers are never used for adversarial
      or proof steps
    - security first, availability second, customizations third
    - guests do not swap
    - Texture is the sole agent writer of documents; one home and one
      writer per kind of data
    - the tape is never modified; candidates are projectors over it
    - no boot-path whole-store pass; no new boot-time resumer
    - schema changes are online and rollback-safe (no offline migration)
  protected_surfaces: Texture canonical writes; tape and evidence;
    checkpoint and witness; self-development apply.
  excluded:
    - SQL against production stores beyond owner-approved read-only
      census queries (census runs on captured bases where possible)
    - cross-owner dedup of private content (dedup within an owner; across
      owners only for published public objects)
  scope_note: >-
    Evidence is Node B only, but the design is for many hosts (owner,
    2026-10-28: scale hosts vertically and horizontally; the Autopaper is
    to grow toward processing global open-source intelligence). Two
    decisions with separate budgets: (A) the per-computer authority
    engine (small, single-writer, private, sharded by computer, portable
    between hosts by tape head plus a small snapshot); (B) the corpus
    tier (a replicated append log, a content-addressed object store,
    and derived indexes per query kind: full text, vector, entity graph,
    time series). Build single-host implementations behind interfaces
    that a distributed backend can replace (local CAS -> S3-compatible;
    segment files -> a replicated log); do not build a distributed
    system yet.

hard_gates (pass/fail before any scoring):
  - bounded peak memory: no operation's memory grows with data size
    (scans, sorts, witnesses and maintenance stream or spill past a fixed
    buffer; caches are fixed-size; no whole result set materialized; GC,
    compaction and checkpoints run in bounded steps or are designed out);
    each operation's bound is written down. Owner, 2026-10-28: memory,
    not disk or CPU, is the cost limiter, and spikes, not steady state,
    were the killer (this week: witness 14G, Store A squash >10G, online
    GC holds, the boot vocabulary pass, capture compaction growth).
    Bounded spikes give stability, capacity planning (overcommit is
    safe only when peaks are bounded) and security (unbounded memory in
    an input's size is a denial-of-service vector).
  - durability: no acknowledged write lost under a kill -9 drill
  - crash semantics: the restart rules above hold
  - witness: a substrate-neutral state root computed in O(tables), not a
    full scan; live and replay roots equal; judged by checkpointd; a
    restore derives the same root; bounded hold on the serving path
    (successor to witness v2)
  - Texture: a sole-writer story and a history read path with no Dolt
    coupling
  - self-development: a candidate-world isolation story (for example a
    whole-store clone at authority-store size)
  - differential: the shadow projector's answers and witness match the
    incumbent's on d@e over a frozen window
  - security: per-owner encryption and crypto-erase, least-privilege reads;
    zero trust between hosts (signed, hashed, verified data); public
    intelligence objects and private owner data in separate tiers
  - data tier separable: shared stores (the corpus server, platform Store A,
    and the new corpus tier) reachable by address with authentication and TLS, artifacts by
    an object-store API, so they can run on their own machines
  - scale-out: state shardable by computer; bytes content-addressed;
    every index rebuildable from the log; no single-host assumption in
    any interface; a computer moves between hosts without copying its
    history

decision_rule (pre-registered before the bake-off):
  - hard gates first; failing one ends a candidate
  - at most three candidates survive the paper cut; at most two get the
    full-tape replay
  - weights over the budget metrics set by the owner with the budgets,
    frozen with the tape head and receipt key before any run

now:
  status: reviewed
  conjecture:
    claim: >-
      Most of the stores' bytes and load are immutable content
      duplicated from the tape or the corpus, and a non-versioned engine
      plus a content-addressed store meets every hard gate and budget
      with less memory and disk than the incumbent.
    test: >-
      The census by identity kind (og_objects split IdentityContentHash
      vs IdentityExternalKey), bytes, rows and write and read rates over
      a fixed window; then the bake-off against the incumbent control.
    edge: missing_oracle (no census by kind or rate exists)
    delta_o: the census
    scope: Node B workloads as of 2026-10
  slice: 1, the census
  next_action: >-
    The census for the paper (captured base 5a241cb0), Store A and the
    corpus, by table and by object identity kind, with rates from traces;
    then a budget draft for the owner to edit.
  census_procedure: >-
    Host-locality inventory first pass done
    (docs/evidence/host-locality-inventory-2026-10-28.md). The size census
    of the paper runs as a planned job, not ad hoc: unpack base 5a241cb0
    (26.5 GB) into a scratch directory on the root disk (needs about 27 GB
    free; 83G free at 12:45 UTC); run the per-table and per-kind queries
    in a systemd-run unit under MemoryMax 6G (host memory available was
    about 10G with the paper running), Nice and idle IO, streaming
    aggregates only (COUNT, SUM(LENGTH(...)) per table and per og_objects
    identity kind); delete the scratch after. Start with d@e's base to
    validate the queries; then the paper; Store A and the corpus only by
    owner-approved read-only queries.
  parallel_track: >-
    Stop the bleeding with what exists, ahead of the control measurement:
    the kill -9 drill and ring walk for store.debounced_checkpoint; the
    Store A squash per-phase memory measurement on a reflink copy; R7.
    The incumbent control is measured after these land.

plan:
  1: census by table and identity kind, with rates (read-only); and an
     inventory of host-locality assumptions (loopback-only DSNs and the
     loopback-only internal token; artifacts read by path; FICLONE and
     btrfs reflinks in capture and squash; same-filesystem renames),
     because the shared stores move off the compute host (owner,
     2026-10-28) while per-computer state stays local as a projection
     whose durable truth (tape segments, snapshots) is shipped off-host
  2: budgets, hard gates and decision rule (owner edits; frozen),
     written as unit costs (bytes per object, writes per event, CPU per
     ingested document, steady-state and peak memory and disk per computer
     and per service) and growth targets
     on two axes (corpus volume; number of computers). Horizons (owner,
     2026-10-28: about 100,000x, roughly 10x a quarter): the interfaces
     must hold to 100,000x without redesign; the implementations to the
     next 10x, replaced component by component at a quarterly scale
     review (measure, project the next 10x, swap what breaks first,
     proven on the replay harness before it carries load)
  3: candidates by divergent panel, with candidate 0 the incumbent Dolt
     (after the parallel-track mitigations); cut on paper to three
  4: build the projector harness (today the rebuilder is Dolt-bound:
     internal/projectionbase/rebuilder.go:385-460, cmd/choir-rebuild-base);
     run the d@e window, then a sampled paper window, then the full paper
     tape for two; kill -9 drill per candidate; peak memory (max RSS,
     not averages) recorded for every operation including GC,
     compaction, checkpoint, witness and restore, and under adversarial
     inputs (huge objects, huge tables, bursts); per-candidate wall-clock
     budget recorded
  5: differential shadow on d@e for the leader; provisional ratification
     before it, binding after it
  6: convergent panel on the scorecard and differential; owner ratifies;
     the migration goal is authored (shadow, widen by ring, reads last;
     rollback = drop the shadow)
```

## Owner answers (2026-10-28 ~13:00 UTC), binding on steps 2-6

- Objective, not budgets: "see how efficient we can make it". A
  multidimensional optimization against physics, information theory and
  sound microeconomics, on trusted open-source parts (no mission-critical
  services from scratch). The budgets step becomes an objective with
  measured unit costs; the hard gates stay.
- Memory model: steady state below 2 GB per computer with 1 vCPU, spiking
  for capsules (Playwright, many concurrent capsules). High memory caps
  with oversubscription, not low caps: maximize virtual memory, minimize
  real (resident) memory. Bounded peaks per operation stay a hard gate;
  that bound is what makes oversubscription safe.
- Checkpoint and restore: fast, with minimal resource use while they run
  and minimal disk growth of the corpus.
- Downtime: the rearchitecture may take significant downtime while
  prelaunch. Autoradio is not built until Autoputer and Autopaper are a
  stable base. After Autoradio ships, no downtime of user computers or
  the Autopaper is tolerated, so the target architecture must deploy with
  zero downtime.
- Public and private: everything is private except what is public (the
  Autopaper's articles and what users explicitly publish). Published
  Textures are public; drafts and owners' captures are private.
  Future-proof for paywall/auth tiers and internal org-wide deployments
  (not built yet): access is a policy over objects, not a binary flag.
- Hosts: zero-downtime deploys come first, then scaling users and
  ingestion, then separating services onto machines (approach open).
  This connects to the progressive delivery system
  (docs/design/progressive-delivery-2026-10-26.md).
- Self-hosting everything is committed: "a bet already paying dividends
  and will accrue as AI becomes ASI, especially at SRE".
- Autoradio (to be documented separately; the owner's sketch): an AI DJ
  playing clips and full episodes of podcasts and videos, interspersed
  with TTS of Autopaper articles and readings of emails and Textures that
  represent agentic workloads; a full voice interface so the user can
  always interrupt and direct it; learns each user's content preferences;
  users can monologue and record their own takes and publish them (with
  or without AI editing), which become transcludable sources read by
  agents and possibly played to other users. Storage implications for
  the census and bake-off: large media objects (content-addressed, by
  reference or cached), streaming reads, per-user preference state, user
  recordings (private until published), low-latency voice paths.
- Desktop computers and drive sync (owner, 2026-10-28 ~13:50 UTC): Mac
  app now, Windows and Linux apps later. Computers run as VMs on desktop
  machines too, and VM state syncs from the cloud to the desktop and back.
  On top of that, a cloud drive through the platform file-sync APIs
  (macOS/iOS File Provider, the Windows Cloud Files API, a Linux
  counterpart), so a file uploaded or downloaded from a desktop or phone
  goes straight to the user's computer, whether Choir hosts it or the user
  self-hosts the whole system. FP0 reading of the consequences for the
  bake-off (to be confirmed by the census and the design, not decided
  here): the portable unit is the semantic state (tape tail, missing CAS
  objects, a small authority snapshot), never a VM memory snapshot, which
  does not cross hypervisors (Virtualization.framework, Hyper-V, KVM) or
  CPU architectures (arm64 desktops, x86 cloud); one active realization
  per computer at a time, handed over by a lease; files are refs plus CAS
  objects in one home (docs/problems/data-has-more-than-one-home-2026-10-24.md),
  with the tape sequence as the sync anchor and the content hash as the
  item version, which is the shape the File Provider and Cloud Files APIs
  expect; the single-box self-hosted install must run the whole stack with
  no hosted dependency (the object store interface keeps a local
  directory backend). Open: whether a desktop may keep working offline
  while the cloud copy also runs (a fork and merge question under the
  doctrine's publication-only rule).
- Zero downtime, staged (owner, 2026-10-28 ~13:55 UTC): "we could end up
  doing either, but I kind of think we want to migrate to cloud
  hypervisor eventually... a staged rollout... keep the cloud hypervisor
  thing as something to do later on". Stage 1 (Autoradio launch):
  session continuity on Firecracker: no call drop, p99 audio gap at most
  250 ms, no lost utterance, at most one invisible reconnect per deploy;
  the media relay at the host edge, session drain, and a deploy classifier
  (agent re-exec / rootfs / VMM). Stage 2 (later): Cloud Hypervisor with
  live migration, so a computer need not restart for host or VMM moves.
  Consequence for this selection: no tier may depend on a
  Firecracker-specific mechanism for correctness (snapshot format, diff
  snapshots, Firecracker's balloon behaviour); the hypervisor stays behind
  the VM seam, and the semantic checkpoint stays the portable unit.
- Follow-ups (owner, 2026-10-28 ~14:00 UTC): the hypervisor deserves its
  own deeper study ("maybe we need to think about the hypervisor more
  deeply"), including what moving to Cloud Hypervisor costs (diff
  snapshots, footprint, attack surface) against what it buys (live
  migration, virtio-mem). Not a blocker here: no tier may depend on
  hypervisor-specific mechanisms for correctness. Offline desktop work
  while the cloud copy also runs is wanted eventually but not now ("no
  one is saying I would use Choir, but not for that feature"): for now,
  one active realization per computer, handed over by a lease; the
  design must not preclude a later fork-and-reconcile feature.
- Roadmap and bake-off scope (owner, 2026-10-28 ~14:10 UTC, revised
  ~14:30): "do the bake-off with Firecracker and then do another
  hypervisor bake-off... more than just cloud hypervisor... different
  configurations, like with Apple virtualization"; "in the bake-offs, we
  should do some light optimization. And then after progressive delivery,
  we should do another hard core optimization pass. And then we can do
  feature development with self-dev." Not started: the owner holds
  execution ("I don't want to start yet"). Order of work:
  1. Data architecture bake-off on Firecracker (this goal, ending at the
     d@e shadow differential, as its finish says), with light optimization
     of the candidates; then the rearchitecture landing on the single
     staging host (Node B), which is the separate red migration goal this
     goal authors at its plan step 6.
  2. Hypervisor bake-off (its own goal file, to be authored): Firecracker
     and Cloud Hypervisor on the server, in their candidate configurations
     (restore mode, private vs shared memory, virtio-mem vs balloon,
     snapshot type), plus the desktop hypervisors the apps need (Apple
     Virtualization.framework, Linux KVM, Windows HCS/WSL2), with light
     optimization. Measures: per-VM host RSS idle and steady, hibernate
     write and wake cost, virtio-mem plug and unplug on Node B, whether
     guest page cache returns to the host, boot time, confinement, and
     portability of the semantic checkpoint across them.
  3. Progressive delivery systematized on the single host
     (docs/design/progressive-delivery-2026-10-26.md): "the feature work
     really benefits from progressive delivery... we're already doing some
     kind of progressive delivery. It's just not systematized yet."
  4. A hard-core optimization pass across the new architecture and
     hypervisor, under progressive delivery.
  5. Feature development with self-development, then Autoradio
     feature-complete on staging.
  6. The non-functional requirements: multiple hosts, zero-downtime
     deploys, off-host durability (relay receipts) and logging.
  Existing Cloud Hypervisor code, checked 2026-10-28: a backend package
  (internal/vmmanager/vmbackend: Backend interface, Firecracker and Cloud
  Hypervisor backends, conformance suite, no caller; CH cold boot, stop,
  metrics, tap by fd, seccomp, raw images, Landlock opt-in, reboot
  detection; no snapshot) at b6f27ded4 on origin/recovered/vmbackend-2026-10-14,
  not on main; it predates the jailer, virtio-mem and FP8 (problem record
  fe1ebc970). The seam design against current main is on
  origin/lane/fp10-res20-ch-seam (docs/reports/vm-hypervisor-seam-design-2026-10-21.md,
  milestones M0-M6). Step 2 starts by rebasing or porting that backend,
  not writing a new one.
  FP0 consequences for this goal: off-host receipts are deferred to step
  6; on the single host the acknowledged write is the local fsynced tape
  append. Correction 2026-10-28 ~14:40 (Devin plan review 1, B1): the
  first version of this sentence said the tape schema already keeps lease
  epoch, receipt and fork-ready fields. It does not. Today an event has
  event_id (a random UUIDv7), previous_head (one parent: a linear chain)
  and sequence (internal/computerevent/event.go:78-109); there is no
  parents[], branch_id or per-append lease epoch; the stream_seq lease is
  a high-water-mark allocator (internal/store/stream_seq_lease.go), not a
  fencing epoch; segment records carry namespace, digest and body, and the
  only receipts are corpus pin acks (internal/tapestore/format.go:33-37).
  Making the schema fork-ready and fenced (content-derived event identity,
  parents[], branch_id, the lease epoch on every append, a slot for relay
  receipts) is a tape schema v2: its own red goal, a predecessor of the
  per-computer migration, so that step 6 and the offline feature are then
  additions, not a retrofit. No tier chosen in step 1 may depend on a Firecracker-specific
  mechanism, so step 2 can change the hypervisor without reopening step 1.

## Amendments 2026-10-28 ~14:45 UTC (Devin plan review 1, send_back; owner precommitment ruling)

Review: Devin, frozen at 038adc19b (sha256 67c276a9...), verdict send_back,
evidence `.agentic-consensus/data-arch/plan-review/out/devin.out`. B1 and B2
are corrected in place above. The rest:

- Expectations, not budgets (owner, 2026-10-28 ~14:40: "the point is not to
  have performance targets or a budget, but to have and state our
  expectations of what sort of performance we can achieve, and then we
  build it, then we optimize... at each step of the way, we are scoring our
  predictions... we want to keep this ledger of learning as well. It's not
  enough merely to make the system work. We also want to extract all of
  our learnings."). Wherever this file says budgets (finish, plan step 2,
  decision_rule, now.next_action), read: precommitted expectations with
  intervals, falsifiers, named measurement difficulties and mitigations,
  recorded in the companion precommitment ledger
  docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md
  before the evidence that resolves them exists, scored when it arrives,
  and followed by new predictions. The decision rule's weights are set by the owner at plan step 2 and frozen with the tape head and receipt key before any candidate run (correction, Devin plan review 2, N3: an earlier wording froze them "with the first predictions", which round 0 already passed without weights); each phase's predictions cite them. Hard gates stay gates.
- Byte deletions (B3): event_json out of the tape index, and content-hash
  kinds to refs plus CAS, are separate orange/red goal files sequenced
  between the census and the harness, per the consensus adjudication. The
  incumbent-model scorecard cells are measured from a frozen tape window
  and base taken before the first deletion. This goal stays read-only.
- Candidates and the witness spike (B4): plan step 3 refines the
  consensus record (docs/evidence/data-architecture-consensus-2026-10-28.md)
  instead of regenerating it, and imports its kill criteria as
  pre-registered falsifiers. Candidate floor: the 2x2 {Dolt, SQLite} x
  {incumbent model, refs+CAS}, plus DoltLite (beta 2026-08-31; no WAL, one
  writer). Before scoring, a witness spike per non-Dolt candidate: a
  per-table multiset hash (LtHash32 or MuHash3072) over canonical full-row
  images, maintained in the same transaction, tested against REPLACE
  (recursive_triggers), VACUUM rowid renumbering, ALTER TABLE ADD COLUMN
  DEFAULT, and UPSERT (docs/evidence/data-architecture-research-2026-10-28.md).
- Census memory shape: the census also records smaps_rollup anon and file
  for the Firecracker process, guest /proc/meminfo, balloon stats and
  per-file page-cache residency of the store files, so memory conclusions
  rest on shape, not table bytes.
- Durability gate, defined: on the single host an acknowledged write is the
  local fsynced tape append; relay receipts are step 6.
- Hypervisor conditionality: correctness may not depend on a
  Firecracker-specific mechanism (above); efficiency results are recorded
  as Firecracker-conditional, with guest anon and file memory per
  operation, so the hypervisor bake-off can reason about reclaim.
- Named residual for step 6: a self-hosted single box cannot produce an
  off-host receipt (a second machine or NAS, a remote bucket, or a
  documented weaker guarantee).
- Stale inputs: MinIO is archived (verified 2026-10-28), so SeaweedFS is
  the later object-store default and the local CAS directory the first;
  Qdrant removal is owner-decided (a separate small orange goal), so no
  candidate plans around a dedicated vector engine.
- Census procedure is a one-off planned host job (systemd-run under
  MemoryMax on Node B), named break-glass under standing question 9; a
  routine census goes through a scoped host-job path.
- Split (goal files, authored when their turn comes, not now): tape schema
  v2 (fork-ready, fenced); byte deletions; the migration/rearchitecture
  (red, authored at plan step 6); the hypervisor bake-off (starts by
  porting origin/recovered/vmbackend-2026-10-14 against the seam design);
  progressive delivery on the single host; the hard optimization pass;
  multi-host, zero downtime, off-host receipts and logging; Qdrant removal.

## Owner requirement 2026-10-28 ~14:47 UTC: a secrets vault

"One data system we haven't yet built that we deeply need... a secrets
vault. Eventually that becomes its own system on its own hardware, or
cluster, or global replication. But we can start off with that as another
host side microservice. Because users are going to have secrets. Every user
is going to need encrypted secrets... for integrating with other services
and platforms. And there's already some issues of, we have different
encryption keys, and there are these little odds and ends that are bits of
data that we're having trouble capturing and holding on to. So we're going
to be forced to reckon with all this as we work to get portability." Not
built now; designed and reviewed. It joins the split as its own goal file:
a host-side vault service first (per-owner encrypted secrets, the key
hierarchy for per-owner CAS encryption and crypto-erase, consolidation of
the existing internal/keyescrow and the other scattered keys), behind an
interface that survives a later move to its own hardware, a cluster and
global replication. The tape records secret references and acts, never
values. It is on the portability critical path: a computer restored on
another host, a Mac or a self-hosted box must get its secrets back by
re-mint or transfer. Design research delegated to DeepSeek; review by
Devin.

## Owner note 2026-10-28 ~15:05 UTC: agent density, then multi-VM per user

"We are working on data architecture to improve stability and
perf/scalability. Dolt is a major bottleneck and probably not helpful
actually. Our actual go rlm harness is quite lightweight, so we should be
able to get high agent density in a single microvm (with Eng agents using
capsules for isolation), and then we can increase our agent concurrency
caps. Then we will expand to multi-vm per user as well (through 'choir in
choir', where one vm controls others). But review this critically."

FP0 critical review (evidence, not a decision):
- Supported: the harness is light. Desk workers are about 17 MiB private
  each and Choir's own retained heap about 50 MiB; the daemon's 690 MiB
  private is mostly the store (docs/evidence/guest-memory-split-balloon-2026-10-14.md;
  docs/problems/daemon-memory-res18-2026-10-22.md). Dolt's versioning is
  read by no product path.
- Not yet supported: that the engine is the concurrency bottleneck. The
  measured limiter is the store's serialisation and read volume: the engine
  mutex was held 70% of 122 s on the paper, about 89% of it reads
  (GetObject 10,701 calls), and the design note's verdict on more slots is
  "nothing: adds waiters" until the work is reduced
  (docs/design/store-engine-concurrency-2026-10-21.md). SQLite also has one
  writer; the win comes from fewer bytes, WAL readers that do not block,
  and an in-process cache of immutable objects, which the refs+CAS model
  makes safe. Raise caps only on a measured curve.
- The likely next limiters at high density are not memory: 1 vCPU (the
  owner's steady-state target) shared by context assembly, JSON, Yaegi
  interpretation and GC (allocation churn about 225 MiB/s on the paper,
  RES-18); provider rate limits and cost; the tape append as the single
  serialisation point; and capsule spikes (Playwright). Density also means
  shared fate: one OOM or crash closes every desk's work in that computer.
- Choir in choir: if one VM controls others, each controlled VM should be
  its own computer with its own tape and lease (the forked-computer rule:
  results return by publication and adoption, never direct merge). One
  computer spanning several VMs would reintroduce multiple writers on one
  tape, which every gate here assumes away. Multi-VM is warranted for
  isolation (untrusted code, separate trust domains), for work larger than
  one VM's ceiling, and for parallel heavy capsules, not as the first lever
  for agent count.
- Consequence for the bake-off: add an agent-density curve on the leading
  candidate (concurrent desks vs p99 turn latency, store wait, CPU, RSS),
  with predictions in the ledger (round 0d).

## Owner note 2026-10-28 ~16:00 UTC: CPU oversubscription, isolation tiers, desktop as the main host

(Correction: first committed as ~15:35; `date -u` read 16:04 at commit 003d6bdd2.)

Owner, paraphrased with key phrases quoted: Choir in choir means "each VM
is its own computer with its own desks, the whole shebang". CPU is the
density limit, so "we also have to oversubscribe CPUs"; agent load spikes
then idles, so a bigger host should carry a higher ratio because
correlated spikes across users are less likely. Concern about cross-tenant
CPU vulnerabilities; price by isolation: shared (default), dedicated CPU
(an upcharge), dedicated machine (more), self-hosting (most). Also: the
user's own Mac/Windows/Linux desktop as the main computer host, operated
from their phone.

FP0 facts read 2026-10-28 (Node B, read-only): Xeon E-2236, 6 cores / 12
threads, SMT on; kernel reports l1tf, mds and mmio_stale_data "SMT
vulnerable"; 3 Firecracker VMs running; load average 0.08, host CPU
pressure 0. Already recorded: SMT stays on until outside users share the
host (owner ruling DEC-06, checklist NET-07,
docs/problems/host-kernel-216-days-unpatched-2026-10-15.md:138-150).
Delegated: DeepSeek research on oversubscription mechanics, per-hypervisor
differences, side-channel tiers and the desktop-host lease (evidence doc to
follow).

## Owner note 2026-10-28 ~16:10 UTC: compute credits for self-hosting, fallback offload

Owner: give users compute credits for running their own computer
("incentivize users to run the infrastructure for us"); many will run it
24/7 or on a spare machine, others pay for the cloud, "either way is fine".
Intensive work offloads to the cloud; with no network "it automatically
offloads to the cloud for you"; the transition must work "as a fallback,
not just when it's intentionally triggered".

FP0 critical notes (design input, not decisions):
- Unplanned failover is a partition, and a partition looks like a crash.
  The cloud can take over only from the last head it received; the desktop
  cannot be told to stop. So: (a) the desktop self-fences when its lease
  expires unrenewed; (b) the unreplicated tail is the data-loss window,
  bounded only if desktop acks wait for an off-desktop receipt; (c) work in
  flight at takeover is closed as interrupted (crash rule), standing
  intents continue on the cloud; a planned handoff (lid close with notice,
  heavy task) carries the planned-restart marker and resumes.
- "Desktop keeps working offline while the cloud also runs" is the
  deferred branch-and-reconcile feature; fallback failover must not quietly
  become it. Until reconcile exists, an expired desktop lease means the
  desktop stops writing.
- Heavy work need not move the whole computer: offload one job to a cloud
  capsule or a child computer (choir in choir), keeping the lease home.
- Credits are for hosting one's own computer. Hosting other users'
  computers on user hardware is a separate trust model (the host operator
  sees the guest) and is out of scope unless confidential VMs make it safe.
- Credits need a verifiable contribution: lease-held hours with receipts,
  not self-reported uptime.

## Handoff 2026-10-28 16:12 UTC (FP0, token-limited)

State: nothing executing; the owner holds execution ("I don't want to
start yet"). All work this session is docs on main; worktree clean.

Landed today (see the review ledger and amendments above): consensus
verdict, research, revised 6-step roadmap, precommitment ledger (rounds
0-0d, owner records O-01/O-02, vault V0-01..16), portability inventory and
verification design, secrets vault design (accepted, Devin round 2 at
208df8855), owner notes on density, choir-in-choir, CPU oversubscription,
isolation tiers, desktop-as-host and self-hosting credits.

In flight at handoff: DeepSeek research on CPU oversubscription, hypervisor
CPU differences, side-channel tiers and the desktop-host lease. Prompt and
output: `.agentic-consensus/data-arch/cpu-oversub/` (gitignored; answer
lands in `answer.md`). Update 16:2x: it completed and is landed verbatim, unverified, at
docs/evidence/cpu-oversubscription-2026-10-28.md. Next agent: spot-check its
sources, then copy its predictions to the ledger as DeepSeek's records.

Queue (not started; owner holds execution): the census; the kill -9 drill
for store.debounced_checkpoint; split goal files (tape schema v2, byte
deletions, migration, hypervisor bake-off, progressive delivery,
optimization pass, multi-host, Qdrant removal, the vault build sequence
per the accepted design §slices, desktop host and failover); agent-density
curve in the bake-off (round 0d). Owner yes needed to delete the
temporary branch lane/fp0-desktop-req.

## Owner request 2026-10-29: deepen alternatives before selecting

Owner: "lets not rule them out yet. a bit more setup at first is fine if it
improves resource utilization or stability longterm", specifically reopening
MySQL/MariaDB on the host and LMDB/bbolt/RocksDB in guests, plus adjacent
options. This authorizes comparative research and documentation, not runtime
code, production experiments, engine ratification or the migration.

The [expanded comparison](../evidence/storage-engine-alternatives-2026-10-29.md)
records primary sources, positive cases, failure mechanisms and qualitative
falsifiers. The [direction note](../design/data-rearchitecture-2026-10-28.md)
now distinguishes hypotheses from deployed properties and carries the review
perspectives. Neither is a new execution design or consensus ratification.

Research candidates are not restricted to the earlier Dolt/SQLite/DoltLite
floor. Host control and corpus compare PostgreSQL, Oracle MySQL/InnoDB and
MariaDB/InnoDB separately; guest research includes SQLite, LMDB, MDBX, bbolt,
RocksDB and Pebble, with Badger/redb and corpus-oriented hybrids considered
where their mechanisms fit. Additional setup or loss of SQL is not itself a
paper-cut rejection. Existing hard gates and limits on measured survivors
and full-paper replays remain unchanged; the census is still next.

Before any expanded candidate experiment, record its expectations in the
precommitment ledger and a workload/implementation plan under the existing
review discipline. Compare equivalent semantics, maintained indexes,
logical witness and durability, then idiomatic physical layouts with all
mapping/index costs included. No numerical performance ranking or prediction
is settled by the research.

In particular:

- Compare sustained reclamation and debt, not the label "no GC": WAL/FTS
  checkpoint/merge, vacuum, purge, COW page reuse, LSM compaction and value-log
  GC have different costs and blockers.
- Count whole-VM/host residency, post-scan cache retention, cold-open and
  recovery peaks, physical I/O, temporary space and backup retention.
- Preserve transactional rows/indexes/watermark/root and the repairable
  prepare -> host CAS -> finalize protocol. The actor SQLite recovery domain
  and other off-tape state require explicit recovery contracts.
- Owner clarification: independently operated control/corpus services are
  required, even when they initially share a host and use the same engine.
  Schemas/databases/roles do not recreate process/WAL/restart isolation.
  Shared-instance memory savings do not supersede this deployment boundary.
- Equal power-loss durability is the starting comparison. A process-kill
  drill is not proof of VM/OS crash, host-loss or partition behavior.
- The earlier engine-share/keep-Dolt heuristics are research hypotheses to
  revisit in the scorecard discussion, not substitutes for lifetime resource
  and operational-complexity evidence. This note does not silently rewrite
  the frozen decision rule or select its weights.

### Subsequent owner clarification: corpus drives the shared-engine default

Prefer one server technology for independently operated corpus and control
services, not a separate engine optimization for each. Corpus is intended to
grow through data-provider ingestion, then dedicated machines, clusters and
eventually a multi-region system; these are growth requirements, not claims
that today's schema or any engine automatically supplies that architecture.
Control should not share the corpus service's failure/maintenance domain.
Security ranks first, stability second, scalability and simplicity jointly
next. Additional setup is acceptable for long-term resource/stability gains.

The [read-only control/corpus investigation](../evidence/control-corpus-resource-profile-2026-10-29.md)
maps what control actually stores and reconstructs hours of active Autopaper
publishing alongside corpus database lifetimes and a confirmed OOM. It separates
current state, accepted-event receipts and physical Dolt history. It also names
the missing matched historical counters. The owner rejected the initial
30-second sample; it is excluded from workload characterization and selection
evidence. Neither the historical findings nor these preferences authorize a
migration, select an engine or silently revise the frozen acceptance gates.

### Owner preference: PostgreSQL, with MariaDB as the server alternative

The owner prefers not to depend on Oracle, narrowing the active host-server
shortlist to PostgreSQL and MariaDB. Earlier Oracle MySQL research remains
comparative evidence, not an active default. The owner leans PostgreSQL for
its ecosystem/default advantage and the option to use pgvector. This is a
provisional preference, not engine ratification or migration authorization.
The guest-engine comparison is unchanged.

pgvector is a credible initial retrieval option, not evidence that one
PostgreSQL instance will serve every future vector workload. Its useful scale
depends on vector dimensions/count, index choice, memory, update rate,
filtering, recall and latency requirements. There is no selected numeric
cutover to a separate vector service. Keep canonical object identity,
permissions and embedding/model provenance explicit so a derived search
index can be rebuilt without becoming a second canonical content authority.

Ordinary PostgreSQL streaming replication does not itself provide transparent
write sharding or globally distributed multi-writer SQL. Partitioned tables
are not distributed shards. The remaining work includes shard ownership and
routing, skew, cross-shard operations, failover/fencing, recovery, and explicit
regional durability/consistency contracts. MariaDB does not remove the need
to make those choices. Prefer a sound single-region acceptance path and
measured growth boundaries over treating a hypothetical future topology as
already designed. No runtime mechanism or new acceptance gate is selected here.

### Research-session closeout: recorded versus unresolved

This session closes the comparative review, not the architecture-selection
mission. The existing census and acceptance gates remain in force; no `/goal`
is completed or superseded by this documentation landing.

Recorded owner constraints: avoid an Oracle dependency; prefer one host server
technology but independently operated control/corpus services; prioritize
security, stability, then scalability and simplicity together. PostgreSQL is
the working preference, MariaDB the active server alternative. pgvector is an
option, not a mandatory permanent search backend. The guest winner is open.

Before implementation, resolve or precommit measurements for:

- Accepted-event granularity, receipt size and retention, including safe replay,
  idempotency and verification boundaries. User count and token throughput are
  not sufficient control-capacity inputs.
- Guest engine/query mapping, off-tape recovery, portable restore, and the
  independent logical witness replacing Dolt roots.
- Corpus unique input bytes/documents, agent reads/writes, query fan-out,
  permissions/provider-license boundaries, and derived-index rebuild cost.
- Measured growth boundaries, partition ownership and hot-key behavior;
  failover/fencing, regional acknowledgement durability, recovery objectives
  and permission/index freshness. Do not mistake table partitioning or ordinary
  replicas for a finished sharding/multi-region design.
- Matched multi-hour resource counters under active use and maintenance.
  Retrospective publication/lifetime logs establish activity and failure, not
  complete matched telemetry or a replacement-engine capacity benchmark.

Node B's observed 98% filesystem use, failed control squash/audit and corpus
OOM remain operational residuals. No cleanup or runtime repair was authorized
or performed in this review; documentation landing does not resolve them.


## Storage audit 2026-10-29 (FP0): what the disk holds, mapped to the plan

Input: [Node B storage inventory](../evidence/node-b-storage-inventory-2026-10-29.md)
(read-only, Sonnet, 2026-10-29), after the owner asked for "a better
accounting of what all our data is and where its saved and what is
real/valuable/unique and what is junk". It is a host disk audit, not the
census: it measures allocated bytes per path, not logical bytes per table or
kind, so it settles no round-0 record. Root fs 475 GiB, 451 used, 15 free;
growth about 85 GB/day from 10-28 to 10-29. Nothing was deleted by it.

### The host's bytes by the target's three tiers

| Tier in the target | Today, where and how big | Finding |
|---|---|---|
| 1. Tape (canonical) | `tape-seg` 27 GB (26 of it the paper), plus the legacy one-file-per-pin layout 47 GB (`pin-receipts` 19.7, `computer-event-payload` 17.8, `computer-event` 9.9; 7.4 M inodes) | The only steady grower: about 2.5 GB/day, almost all the paper. The tape is never modified, so its rate is the floor every other tier sits on; what goes onto it (R7 trace lines, telemetry) is the lever, not deletion. The legacy layout is the same class in a second shape and its directories were still touched today, while corpusd /health counts 5,252 legacy pins and 0 fallbacks: unexplained, to investigate (repack, then retire). |
| Derived accelerators | `projection-base` 61.6 GB; Store A and B Dolt history | The 10-28 jump is mostly bases: 28.4 GB written that day, one base 26.5 GB (the paper's whole embedded store). GC keeps nearly all of them (7-day grace plus liveness; under 1 GB/day freed). A base is a full copy of the guest store, so it carries `event_json` again: one event lives in tape-seg or a legacy pin, in `computer_event_index`, in every retained base, and transiently in the capture clone. |
| 2. Immutable content | file CAS chunks 28 GB; `capture-bodies` 9.6 GB (static since 10-13); image blobs (guest) | File CAS is about 150 MB per computer across about 190 computers, likely the same seeded `files/Source` (289 MB on the host) encrypted per computer, so it cannot dedup. Seeded source is public content: a concrete case for owner question 2 (cross-owner dedup for public objects only). |
| 3. Small current state | Store A current state: 251 head rows | Store A is 53.7 GiB of which 53.4 is oldgen (the profile doc); its live control state is small. The bytes are history, not authority. |
| Backups | `platform-dolt/.choir-backup` 52 GB exclusive on the same disk; restic adds about 52.7 GB to the remote repo per night (365 GB raw) | Dolt history is paid three times: live, a local copy, and a poorly deduplicated remote copy. The backup of control should be a bounded logical snapshot, not its engine history. |
| Computer disks | paper `data.img` 55 GiB exclusive (137 GB sparse); hibernated computers hold no local disk (16 vm-state entries vs 145 hibernated) | The paper's disk is twice its compacted store (26.5 GB on 10-28): unexplained, to measure in the census. Hibernated computers live entirely in tier 1, file CAS and bases, so base retention is a wake-path decision, not only a disk one. |
| Not data | `/nix/store` 49 GB, swap 17, guest releases 12, builder 6, caches about 12 | Out of scope for the engine choice; retention by rule. |
| Junk | candidate fleets 14 GB plus an 8 GiB memory snapshot, an old VM 3.2, `/data` 9, `computer-inputs` 0.7, retired bases 0.45, `taperepack-work` | Delete with the checks the inventory names (vmctl for VMs); `/data` needs the owner. |

### What this changes in the plan

1. **The bulk is outside the stores the bake-off compares.** Of about 450 GB,
   the Dolt stores' current state is a few GB. History (Store A 53, Store B
   14), its backup copy (52), projection bases (62) and the tape in two
   layouts (74) are the mass. The scorecard's disk metric must therefore be
   per tier and include retained bases, backups and the tape, as the
   alternatives research already asks ("retained snapshot/backup space");
   an engine that wins on table bytes but keeps whole-store bases would not
   move the host.
2. **Step 2 (stop the bleeding) gets a measured order:** projection-base
   retention (latest per computer plus pinned restore points; up to about
   44-55 GB, and the burst driver); the local Dolt backup copy replaced by a
   transient snapshot or logical dump (52 GB local, about 52 GB a night
   remote); the legacy pin repack and why it is still written (47 GB, 7.4 M
   inodes); then junk. Each is its own change with its own class: base
   retention and the backup procedure are red (checkpoint, evidence).
3. **Step 3 (`event_json` out) and step 4 (content to CAS) shrink every base,
   not only the guest store.** Bases are the place their savings multiply.
4. **The tape needs a retention design, not only a compaction.** At about
   2.5 GB/day for one computer, a canonical, never-modified tape is the
   long-run grower. R7 (trace lines leave the tape and the store) is the
   first cut; tape schema v2 should carry a payload-class field so
   telemetry can be retained by rule while semantic events stay forever.
5. **Shared public bytes are being stored per owner.** Seeded source in every
   computer's encrypted file CAS is the measurable example; the vault and
   CAS design should name a public tier for it.
6. **No time series exists** (the profile doc found none). The census should
   add a daily per-tier size ledger (`btrfs filesystem du` per subvolume or a
   `du` per tier into a JSONL file) so growth claims stop resting on mtimes.

### Census additions from the audit

- Logical size of `computer_event_index` versus the paper's `data.img`
  (55 GiB exclusive against a 26.5 GB store): what fills the difference.
- Which computer and pin hold each base over 1 GB (the GC dry-run report
  answers it without a tree scan).
- Why legacy pin directories are still written, and how many pins a day.
- Store B growth after 10-28 (the audit script exits on Store A's failure
  before sampling Store B).
