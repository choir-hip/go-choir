# Beads Mission-State Doctrine Delta v0

Status: proposed (promote into AGENTS.md / agent-parallax-rules.md when the
`beads/parallax-v2` branch is quiescent). This is the harvested learning of epic
`choir-pfg` (Beads-native mission state / Parallax v2). It is the residue artifact
for that epic per the residue contract below.

## What changed

Mission coordination state moves out of the doc tree into beads (`bd`), a
dependency-graph issue tracker on embedded Dolt. Docs go back to being docs.

- **epic = mission**, **sub-issue (task, `--parent`) = conjecture**.
- Beads holds the churny middle: mission definition, state, ledger, and the DAG.
- Docs hold the durable ends: the ideas that seed missions and the learning that
  exits them. `docs/mission-graph.yaml` becomes a generated projection (after the
  flip; mechanism proven in C6, cutover gated on branch quiescence).
- The truth boundary is **"is this still in motion?"** In motion -> beads.
  Settled into idea or learning -> docs.

See `skills/beads-missions/SKILL.md` for the operating command surface.

## The truth boundary vs. the doc-truth system

The doccheck registries are affected as follows:

- `mission-graph.yaml` -> superseded by the beads store; demoted to a generated
  projection. doccheck gains rule **R8** (report-only) that natively reads
  `.beads/issues.jsonl` (hermetic, no `bd` shell-out) and validates dependency
  integrity, acyclicity, YAML<->beads binding drift, and V-consistency.
- `doc-authority-manifest.yaml` and `conjecture-assertion-ledger-2026-06.md`
  STAY docs — they govern/harvest durable knowledge.
- The "no double-ledger" rule is HONORED, not violated: state is MOVED into
  beads, not duplicated. The ledger lives in beads as the issue event log /
  append-notes, not transcribed into both a doc and the graph.

## The residue contract (the safe-mowing gate)

Operationalizes "Partial work is allowed; lost learning is not." Two gates before
an epic may close:

- **G1 descent:** V = 0 (all child conjectures closed) OR remaining children
  explicitly superseded/handed-off (edges named). V = count of open child
  conjectures; it is a query, never hand-typed.
- **G2 residue:** the close must leave durable learning that RESOLVES — an
  assertion-register entry (A*/I*/E*), an architecture/doctrine doc edit, or
  (for a SUPPORTED settle) a merged commit AUTO-LINKED to an assertion (ratified
  Q1). A ledger entry alone, or "see the bead," does NOT count.
  - falsified/weakened MUST leave an evidence doc (Problem-Documentation-First).
  - superseded carries its residue obligation FORWARD to the successor
    (ratified Q2: defer-but-never-escape); a genuine reframe discharges the
    inherited debt with a single assertion line.

**Mowability predicate:** a `docs/mission-*.md` projection is safe-to-delete IFF
its epic is closed AND its residue links resolve. Proven computable in C9: C6's
comment-capture put each superseded mission's lineage rationale into the bead
`design` field, so superseded missions' residue resolves in beads and their
paradocs are mowable now. Settled missions predate this contract and need
case-by-case harvest first. Bulk mow is owned by `dead-vtext-workspace-cleanup-v0`.

## Status / verdict vocabulary (frozen, C4)

planned->open; working->in_progress; open_handoff->in_progress + label `handoff`;
blocked->open + label `blocked`; settled->close reason "settled: ...";
superseded->close reason "superseded: ..." + label `superseded`.
Conjecture verdict = close-reason prefix: supported|weakened|falsified|superseded.
`discovered` is not a close — it is a new sub-issue via a `discovered-from` edge
(raises V; that is progress). `enables` is DERIVED as the exact inverse of
`depends_on` (ratified; depends_on is the canonical load-bearing edge).

## Lazy descent (C7)

Imported missions are flat epics. A mission is descended into conjecture
sub-issues WHEN IT BECOMES ACTIVE WORK, not in bulk. Settled/superseded epics are
never descended. This matches Parallax's just-in-time compilation of state.

## Operating-contract rules as queries (C10)

Rules that were human vigilance become standing beads queries over the event log:

- **Dead-End Escalation** ("3+ iterations / 2+ days without convergence"):
  `bd list --status in_progress --json` filtered on `started_at` age threshold.
- **Root-Cause Clustering** ("3+ bugs in one subsystem in a week"):
  group open issues by subsystem label over a time window.
- **Heresy delta** (discovered/introduced/repaired): counts over `heresy:*` labels.

## Open edges (not yet settled)

- Actual YAML demotion (flip cutover) — mechanism proven (C6), gated on branch
  quiescence + team coordination.
- Cross-worktree dolt-remote merge — unverified (C8 open edge); shared-store
  parallel writes are proven safe.
- Bulk harvest-and-mow of the settled paradocs — owned by
  `dead-vtext-workspace-cleanup-v0`.
- R8 graduation from report-only to enforcing — per the docs-truth philosophy of
  typed allow-contexts before enforcement.

## Provenance

This epic was executed natively in beads as its own first mission (dogfood): epic
`choir-pfg` with conjecture sub-issues `choir-pfg.1`..`.10`, V descending by typed
verdicts, ledger as append-notes. C6 returned a real `weakened` verdict (flip not
lossless) before a second pass earned `supported` — the conjecture-circuit working
as designed, not a rubber stamp.
