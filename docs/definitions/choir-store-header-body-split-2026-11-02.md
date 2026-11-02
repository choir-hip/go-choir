---
definition_version: 4
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-01T19:00:00-05:00
  source:
    canonical_ref: main 3529f400d
    deploy_identity: 61772f7417593eb40e3427b3a60f5fdd54711e07
  worktrees:
    - path: .claude/worktrees/agent-a68a1d96eb5bf63ea
      status: clean
      class: goal_candidate
      owner: the guest isolation session
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
  status: checkpoint_incomplete
  slice: drafted; waits on the guest isolation closing report and owner direction
  source_ref: 3529f400d
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
    status: proposed
    evidence_refs: [docs/definitions/choir-guest-isolation-2026-11-01.ledger.md (2026-11-01 reread entries)]
  decision:
    what: none
    kind: architecture
    status: proposal
    evidence_ref: this file
    owner_ratification_ref: required
  belief:
    believed_state: >-
      Five reread fixes on 2026-11-01 were the same defect: a filter field
      after the body. Each was patched by an index; the layout remains.
    main_uncertainty: >-
      The migration cost on the paper's store (2.5M events, multi-GB) and
      whether a cheaper column reorder (body last) gets most of the win.
    next_observation: the guest isolation closing numbers, then a page-read count per statement on a store copy
  blocker_or_risk: owner ratification (architecture); the guest isolation mission closes first
  next_action: after the closing report, measure body-last versus split on a synthetic store, then send this file to review

receipts: []
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
