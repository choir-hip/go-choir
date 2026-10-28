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
