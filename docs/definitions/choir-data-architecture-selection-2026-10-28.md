# Choir data architecture selection (2026-10-28)

```yaml
definition_version: throughline/1
readiness: drafted
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

start:
  source: main 0bf89ed84 (staging deployed), 2026-10-28 ~12:30 UTC
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
    - Costs paid: commit-per-write history; Dolt GC memory (the Store A
      squash OOM at 10G); the online GC on the write connection (81% of
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
    then the full paper tape for at most two survivors, then a shadow
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
  - durability: no acknowledged write lost under a kill -9 drill
  - crash semantics: the restart rules above hold
  - witness: a substrate-neutral state hash that a replay reproduces,
    with a bounded hold on the serving path (successor to witness v2)
  - Texture: a sole-writer story and a history read path with no Dolt
    coupling
  - self-development: a candidate-world isolation story (for example a
    whole-store clone at authority-store size)
  - differential: the shadow projector's answers and witness match the
    incumbent's on d@e over a frozen window
  - security: per-owner encryption and crypto-erase, least-privilege reads;
    zero trust between hosts (signed, hashed, verified data); public
    intelligence objects and private owner data in separate tiers
  - data tier separable: shared stores (corpus, platform, the corpus
    tier) reachable by address with authentication and TLS, artifacts by
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
  status: drafted
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
     ingested document, memory and disk per computer) and growth targets
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
     tape for two; kill -9 drill per candidate; per-candidate wall-clock
     budget recorded
  5: differential shadow on d@e for the leader; provisional ratification
     before it, binding after it
  6: convergent panel on the scorecard and differential; owner ratifies;
     the migration goal is authored (shadow, widen by ring, reads last;
     rollback = drop the shadow)
```
