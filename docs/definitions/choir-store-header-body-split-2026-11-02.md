---
definition_version: 4
readiness: executable  # owner 2026-11-01: "Make it a throughline now, superseding"

review:
  reviewer: none  # executable on the owner's word; the build runs under the data system Operating Tempo (runtime checks verify); the landed candidate gets tier 2 (Devin + DeepSeek) before complete
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-01T19:36:31-05:00  # promoted to executable; drafted 19:00 at 3529f400d
  source:
    canonical_ref: main 2ec066cdb
    deploy_identity: 61772f7417593eb40e3427b3a60f5fdd54711e07
  worktrees:
    - path: .claude/worktrees/agent-a68a1d96eb5bf63ea
      status: clean
      class: goal_candidate
      owner: this session (the guest isolation session, now this mission)
      touch: goal_owned
      recovery: leave in place

finish:
  deliver: >-
    A guest store that reads a row's small fields without reading its body:
    a scan, filter or sort over objects or events touches only headers, and a
    body is read only when it is used.
  artifact: >-
    og_objects and choir_events (and the telemetry partitions) split into a
    header table (canonical id, kind, owner, computer, version, hash,
    created_at, updated_at, tombstone, superseded_by, the hot metadata fields
    and event kind and phase as typed columns) and a body table (canonical
    id, body); every reader of headers reads the header table; the
    workaround indexes and "refs without body" helpers that the split makes
    unnecessary are deleted.
  acceptance:
    - action: the store suites, with plan tests on every header read (no SCAN of a body table, no body column read)
      proves: no header-only read touches a body
      evidence_class: local test
    - action: migration differential on a copy of the paper's store, then d@e, then the fleet
      proves: migrated state answers like the old state (rows, bodies, derived keys, replay head)
      evidence_class: deployed proof
    - action: the memory meter on the paper for a day after the cutover, against the guest isolation closing numbers
      proves: per-minute reads, the retirement pass and the boot read burst fall (or the claim fails visibly)
      evidence_class: deployed proof
  rollback: >-
    The migration keeps the old tables until the differential is green on
    each ring; a failed ring restores from its pre-migration snapshot.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the bytes read per row returned by header reads (scans, filters,
    sorts, retirement, boot) while preserving every body read, the tape, and
    replay equivalence.
  goodharting_would_be: >-
    Adding more covering indexes, or caches, until the plans look clean:
    fewer reads now, more copies of the same fields, and the next new filter
    reads bodies again.

homotopy:
  realism_axis: >-
    one table on a synthetic store with large bodies → both tables on a copy
    of the paper's store → d@e → the fleet under the meter

boundaries:
  mutation_class: red
  authority_sources: [owner 2026-11-01 (memory is the bottleneck; read from disk, never extra reads), Choir Doctrine, docs/definitions/choir-data-system-build-2026-10-30.md]
  must_preserve: [the tape and replay equivalence, every body read, the keep set, derived keys, crash-close/planned-resume restart rules]
  excluded: [the tape format, Postgres, run_memory_entries (its own split later if the meter says so)]
  protected_surfaces: [guest store schema, boot path, Texture canonical writes (bodies), replay]

now:
  status: complete  # 2026-11-03T05:30Z: S6 judged on the meter day. The claim holds on read volume (mean -64%, p90 -67%, per model call -68%, retirement pass ~8.5 GB to 0.6-1.3 GB, boot burst ~2.6x lower) and fails on the median minute (4.1 to 46.8 MiB/min), named as a successor problem; .presplit finalized on the paper and the owner's computer
  slice: >-
    S0 measure: on a synthetic SQLite store with the og_objects and
    choir_events shapes and large bodies, count pages read per statement for
    the hot header reads (retirement slices, lifecycle reads, kind/owner
    newest, boot scans) under three layouts: today's (body mid-row), body
    last, and header/body split. Then S1 design the split tables and the
    migration; S2 build (readers, writers, migration, plan tests, index
    deletions); S3 differential on a copy of the paper's store; S4 d@e;
    S5 fleet; S6 a meter day against the baseline.
  source_ref: 2ec066cdb
  deploy_identity: 61772f7417593eb40e3427b3a60f5fdd54711e07
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: H1
    claim: >-
      Most guest disk reads beyond the working set come from header reads
      that read bodies because the body precedes the header fields in the
      row; splitting headers from bodies removes the class, and most of the
      five indexes added 2026-11-01 become unnecessary.
    test: count header reads that touch body pages before and after, on a copy of the paper's store; then the meter
    edge: missing_oracle
    delta_o: a per-query bytes-read figure on the guest (SQLite page reads per statement)
    scope_if_supported: the guest SQLite store
    status: supported  # synthetic rung (S0): split reads 3-40x less than today and than body-last on every header read; body reads unchanged
    evidence_refs: [docs/evidence/store-layout-reads-2026-11-01.md, docs/definitions/choir-guest-isolation-2026-11-01.ledger.md (2026-11-01 reread entries)]
  decision:
    what: split og_objects and choir_events into header and body tables (direction ratified; body-last is measured as the control in S0)
    kind: architecture
    status: ratified
    evidence_ref: this file
    owner_ratification_ref: owner 2026-11-01 "Make it a throughline now, superseding" (after the drafted file and the data-oriented design inspiration)
  belief:
    believed_state: >-
      Five reread fixes on 2026-11-01 were the same defect: a filter field
      after the body. Each was patched by an index; the layout remains.
    main_uncertainty: >-
      The migration cost on the paper's store (2.5M events, multi-GB) and
      whether a cheaper column reorder (body last) gets most of the win.
    next_observation: S0 page reads per statement, three layouts; the baseline is the paper's memory meter from 2026-11-01T19:32Z (guest isolation checkpoint numbers)
  blocker_or_risk: none for this mission; residuals in docs/problems/paper-steady-read-floor-after-split-2026-11-03.md (the median minute rose to a ~40 MiB/min floor; host disk did not return the 22.4 GB the finalize freed inside the guest, since the image is sparse and the guest has not discarded)
  next_action: none; the successor probe is in the problem doc

receipts:
  - S6 2026-11-03 (meter day 2026-11-02T05:00Z to 2026-11-03T05:00Z against 2026-11-01T19:32Z to 2026-11-02T04:00Z, /var/lib/go-choir/growth-meter/mem-2026-11.jsonl, paper vm-1669020a): read_bytes per minute, all minutes: baseline median 4.1, mean 193.0, p90 399.7 MiB/min; day median 46.8, mean 69.5, p90 131.6. Excluding 30 min after each boot: baseline 3.1 / 75.8 / 132.7, day 38.7 / 49.4 / 104.5. Paper inference requests: baseline 14,375 (28.3/min), day 46,859 (32.5/min); reads per request 6.7 to 2.1 MiB. Retirement pass: about 8.5 GB before (2026-11-01 ledger); 591 MiB (22:51Z, 7 ops in 1m35s) and 1,268 MiB (00:08Z, 9 ops in 1m18s). Boot burst (first 15 min): baseline 0.7 to 21.6 GiB over 10 boots, median about 5.5; day 1.6 to 5.2 GiB over 15 boots, median about 2.2 (the 207 GiB migration boot at 04:05Z excluded). Guest anon: median 1.41 to 2.32 GiB, p95 6.47 to 3.94, max 7.91 to 4.69. No snapshot producer run appears in the guest logs available (from 2026-11-02 21:40Z); none excluded. Finalize: the paper freed 22,382,874,624 bytes, the owner's computer 473,993,216 (split-finalize, 05:2xZ); host free space unchanged at 143G. Partition fix b80fe36bd re-review: Devin approve (scratchpad review-part). Verdict: holds on volume, fails on the median (successor problem).
  - S5 2026-11-02 (running fleet): the paper (9m39s, 3.41M objects), the owner's computer (8.3s), d@e (2.7s) migrated on 1d33cafce..81f09e677; keep set grew only
  - S4 2026-11-02: d@e copy rung 13/13 green, then d@e migrated and finalized (95.8 MB freed)
  - S3 2026-11-01: paper store copy migrated in 8m33s (22.30 -> 21.31 GB, 3.4M objects); differential green except the intended store_meta schema_version 1 -> 2 row (diff tool now exempts it); ledger 22:45 entry
  - S2 2026-11-01: split/s2-integrate f3e5f7d2b (slices 0, A-E, migrate command, epoch 33); CI mirror light set green
  - S1 2026-11-01: design docs/design/store-header-body-split-2026-11-01.md (header tables keep their names; bodies in og_bodies/choir_event_bodies keyed by an explicit INTEGER PRIMARY KEY; witness via logical tables so every table digest is unchanged; boot migration to a new file behind an early replaying gate; .presplit kept until the differential is green)
  - S0 2026-11-01: split chosen over body-last; docs/evidence/store-layout-reads-2026-11-01.md
---

# Store Header/Body Split

Inspiration (owner, 2026-11-01): data-oriented design. Keep hot fields
together, put cold blobs elsewhere, and redesign rather than add caches.
An index on a field that the row layout hides behind a blob is a cache.

The defect it removes, seen five times on 2026-11-01 (ledger): the event
kind, phase, `updated_at` and metadata sit after `body` in `og_objects` and
`choir_events`, so any statement that tests them reads the body's overflow
pages. The retirement pass read about 8.5 GB for a few hundred rows; the
lifecycle reads decoded every object on the computer; the "refs without
body" helpers (objectgraph `ListObjectRefsByKindOwner`,
`ListAllObjectRefsByKindOwner`) read every body they claim to skip.

Candidates to delete once headers stand alone: the expression indexes
`idx_og_objects_choir_event_kind_updated` and
`idx_og_objects_choir_event_phase_updated`, the json generated columns that
exist only to avoid body reads, and the two-phase snapshot reads whose first
phase exists to avoid bodies.

## Supersedes

This mission supersedes the guest isolation goal
([`choir-guest-isolation-2026-11-01.md`](choir-guest-isolation-2026-11-01.md)),
owner 2026-11-01. Its stations I1 to I6 landed; its I0 closing report is
folded in here as the baseline: the memory meter on the paper from
2026-11-01T19:32Z, summarized in
[`../reports/guest-isolation-checkpoint-2026-11-01.md`](../reports/guest-isolation-checkpoint-2026-11-01.md),
is the before for S6. Its residuals (the write-storm neighbour cost, the
undrilled network limits, every guest deploy rebooting every guest) stay
named there and do not block this mission.
