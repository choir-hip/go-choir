# Reorientation: Docs Overhaul + Precommitment Records Cutover

Date: 2026-09-22
Mutation class: **green** (docs-only survey and plan; the prune it proposes is
also green — no runtime behavior).
Status: **executed 2026-09-22** — Phases A–C landed (zombie authority killed,
PICL→precommitment rename, ~14 superseded definitions retired to Git history,
manifest/graph/ACTIVE repaired). Skill generations mission-gradient, parallax,
and definition are **deprecated, kept for reference** (not deleted — owner
correction); throughline is the current `/goal` format. Phase D (reorient
standing docs) landed. The corrected roadmap is
[`world-wire-mission-stack-2026-09-22.md`](world-wire-mission-stack-2026-09-22.md)
(under owner review); the owner's situation brief is
[`current-situation-2026-09-22.md`](current-situation-2026-09-22.md).

## Why now

Three inputs converged on 2026-09-22:

1. **Precommitment records** (the renamed PICL — renamed for a 2023 paper
   collision) is the maturation of the conjecture-ledger lineage:
   `CLAIM/TEST/EDGE/ΔO/SCOPE` in mission-gradient → parallax's conjecture
   circuit → Definition v2's goal-file format → **throughline** (current).
   docs (`Precommitment Records Theory and Implications.md`,
   `Precommitment Records — Engineering Memo.md`) are the current canonical
   statement: one mechanism serving context packs (learning), procedural
   fidelity (alignment), and audit (enterprise).
2. **The pitch** (`docs/deck/pitch-bones-vision.md`, rendered
   `choir-seed-deck-2026-09-22-light.pdf`) already sells this: §6 the learning
   loop, §10 the box score, §12 "everyone sells the agent, we sell the
   record." The supervision workbench's substance is the commitment ledger.
3. **The docs have sprawled**: 604 `.md` files, 45 definitions, 3 generations
   of ledger skills live in-tree, and the PICL→precommitment rename was
   half-applied (now completed — see Phase B). Retrieval pollution is a
   settled failure mode here (documentation-authority-reduction K6).

## Inventory findings (six-scout survey, 2026-09-22)

### Authority stack (intended vs actual)

Intended: `docs/choir-doctrine.md` (apex) → `AGENTS.md` (operating contract) →
promoted Definition (sole executable authority) → `ACTIVE.md` (curated view) →
`mission-graph.yaml` (discovery metadata) → evidence/reports (non-authority).

Actual drift — **live-authority defects** (pre-execution findings; all eight
repaired by Phase A — see status at top):

| Defect | Location | Problem |
|---|---|---|
| Stale active-slice claim | `docs/ACTIVE.md:178-185, 217-218` | Names superseded `choir-rlm-session-interpreter-cutover-2026-09-02` as the active executable slice; contradicts the same file's correct sole-entrypoint claim (lines 8-15) |
| Stale residue | `docs/mission-residues.md` R7 | Says carrier "not yet chartered"; it is chartered and working |
| Zombie executable | `choir-autoputer-completion-2026-07-13` (retired to Git history) | Header said `working`; graph and its own later state said superseded/blocked |
| Zombie executable | `docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md:130-157` | `resumed_owner_directed` + live `next_action`; graph/ACTIVE say superseded |
| Zombie executable | `docs/definitions/choir-host-orchestrated-recovery-2026-08-22.md:101-132` | Says `working`, asks for push/deploy; graph says settled/superseded-incomplete |
| Missing status | `docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md:106-133` | No canonical `now.status`; graph says settled |
| Stale A/B next step | `docs/mission-mail-app-v2-2026-09-15.md:92-116` | "Owner picks keeper" — owner already picked Mail; EmailApp deleted |
| Broken link | `docs/mission-mail-attachments-and-app-cutover-2026-09-17.md:201-203` | Promises `mission-report-mail-attachments-2026-09-17.md`; file does not exist |
| Stale architecture claims | `docs/current-architecture.md`, `docs/platform-os-app-state.md` | Say no product Definition is executable; carrier is |
| Duplicate authority | `CLAUDE.md` | Verbatim copy of AGENTS.md; drift risk |
| Graph hygiene | `docs/mission-graph.yaml` | Invalid `operational_recovery` kind; 4 definitions ungraphed; `mission-residues.md` ungraphed |
| Stale snapshot | `doccheck-report.md` | Sep-11 full-corpus report (588 docs then, 604 now) |

### Skill lineage

`mission-gradient` (typed conjecture belief state) → `parallax` (budgeted
conjecture circuit, variants, ΔV, observer shifts) → `throughline` (compiles
both into executable `/goal` files) → `definition` v2 (the promoted authority
per AGENTS.md).

- `mission-gradient`, `parallax`, `definition`: superseded at the skill layer —
  **deprecated, kept for reference** (owner correction: not deleted).
- `throughline`: the current `/goal` authoring format. Existing
  Definition-format goal files remain valid/executable; throughline is the
  authoring discipline layered on the same `/goal` artifact.
- `cognitive-transform-portfolio`, `agentic-consensus`, `choir-cli`: keep.

### PICL / precommitment corpus

| File | Date | Term | Disposition |
|---|---|---|---|
| `Precommitment Records Theory and Implications.md` | 09-22 | precommitment | **Canonical concept statement** |
| `Precommitment Records — Engineering Memo.md` | 09-22 | precommitment | **Canonical implementation spec** (schema, scoring, context packs, harness integration, open decisions) |
| `reviews/picl-consensus-synthesis-2026-09-17.md` | 09-17 | PICL | Dated evidence: 8 beta schema decisions, replay-control critique, epistemic-boundary invariant — unique content, keep |
| `reviews/picl-paper-review-2026-09-17.md` | 09-17 | PICL | Dated evidence: adopt/defer analysis |
| `reports/choir-status-rlm-picl-worldwire-2026-09-17.md` | 09-17 | PICL | Point-in-time program status; unique carrier-blocker + World Wire sequencing record |
| `designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` | 09-15 | PICL | Durable design input: event-driven RLM ontology incl. learning-record subsection |
| `memo-diagonal-media-strategy-2026-09-17.md` | 09-17 | PICL | Owner strategy direction (record/box-score/diagonal GTM) |
| `memo-predictive-icrl-delta-2026-09-10.md` | 09-10 | (pre-PICR) | Adjacent predecessor: objective resolution, provenance, episode schema |
| `rlms-ontology-brief-2026-09-10.md` | 09-10 | — | Forecast/Observation/Adjudication/Assessment distinctions |
| `choir-one-pager-2026-09-18.md` | 09-18 | precommitment | **Renamed** (was PICL) |
| `deck/pitch-bones-vision.md` | 09-18 | precommitment | **Source renamed**; the two 09-18 HTML renders still say PICL → re-render or retire in favor of the 09-22 seed deck |

Both Sep-22 docs are now registered in `docs/doc-authority-manifest.yaml`.

### Code-side reality (grounds the mechanism)

- **No runtime conjecture/precommitment ledger exists.** The ledger is
  Markdown (`docs/conjecture-assertion-ledger-2026-06.md`) validated by regex
  in `cmd/doccheck`. No prediction/score/resolution types in `internal/`.
- `internal/capsule/executor.go:524` "precommit" = capability-handle binding,
  unrelated. `internal/qdrant/routing_test.go` has no PICL token.
- Integration points for a real precommitment module (from code survey):
  - Agent-facing API: `internal/yaegikernel/choir.go` `ChoirExports` (the
    `choir` package model code imports) + allowlist wiring in
    `yaegikernel/eval.go`, `sidecar.go`, `cmd/capsule-broker/session_worker.go`.
  - Persistence: additive OG object kind + provenance edges via
    `Store.AppendEvent` → `AppendEventOG`/`ogPut` → `og_objects` (VM Dolt).
    Canonical computer-event chain (`ComputerEventAppender` → platform CAS)
    only if commitments are computer-authority/replay semantics.
  - Authority: reducer-side, not Yaegi worker-side; agents commit/resolve but
    cannot edit committed records (matches Engineering Memo §Harness).

### Bulk tree

- `docs/archive/` 177 files — cold storage, keep as searchable history.
- `docs/evidence/` 271 files — proof artifacts, keep-all by policy.
- `docs/reports/` 37, `docs/reviews/` 13, `docs/designs/` 8, `docs/problems/`
  26, `docs/definitions/` 45, docs-root 51.
- Repo prune doctrine (documentation-authority-reduction-2026-07-09, K3/K4/K6):
  **Git history is the retained copy; a second in-worktree archive is not
  required; searchable stale prose is retrieval pollution → delete, don't
  archive.** Blanket-archiving is explicitly not the solution.

## The reorientation

The mission stack's substrate program (restore-zero → settlement gate →
versioned rename → carrier) built the trustworthy event log. Precommitment
records are what that log is *for*: the commitment ledger is the learning
mechanism, the alignment surface, and the product moat — the deck's "owned
work history compounds" made literal.

Reordered stack:

```text
substrate (event log, recovery, carrier)   — built; carrier blocked on owner decision
    ↓
precommitment records (mechanism)          — the product capability:
    commit → observe → score → revise → retrieve (context packs)
    ↓
supervision workbench (product)            — Texture + Mail + Newspaper;
    the commitment ledger is what humans supervise
```

Consequences:

- The carrier's pending owner decision is re-scoped: it matters insofar as
  the carrier is the desk the precommitment module ships through. The
  wake-authority repair is substrate correctness, not product-gating.
- Missions 9 (shadow evals) + 10 (native goals) in the 11-mission overview
  are where scoring/selection live — the precommitment mechanism absorbs
  them; the overview's ordering rationale needs a rewrite.
- `docs/conjecture-assertion-ledger-2026-06.md` is the Markdown ancestor of
  the record schema; the Engineering Memo's schema is its successor.

## Proposed phases

**Phase A — kill zombie authority (rewrite, no deletion).**
Fix the eight live-authority defects in the table above: ACTIVE.md stale
slice claims, R7 residue, four zombie definitions → superseded tombstones,
mail-app-v2 next step, broken report link, stale architecture claims,
mission-graph hygiene. Small, surgical, unblocks honest retrieval.

**Phase B — rename pass (PICL → precommitment records).** *Completed.*
Customer-facing only: `choir-one-pager-2026-09-18.md`,
`deck/pitch-bones-vision.md` renamed. The two 09-18 HTML renders still say
PICL — re-render from the renamed source or retire in favor of the 09-22 seed
deck. Historical PICL-named files stay as dated records. Both Sep-22 docs
registered in `doc-authority-manifest.yaml`.

**Phase C — prune per K3/K4/K6 (delete, git retains).**
- Definitions: keep-as-authority = carrier only. Keep-as-evidence (rewrite to
  compact receipts): audited construction, coherent convergence, tape
  recovery, substrate overhauls/recovery, private Go kernel, restore-zero,
  settlement gate, versioned rename, mail attachments, CI reports,
  platform-Dolt goal. Keep-as-remainder: target-architecture cutover (R6).
  Keep-as-draft-or-delete: the two never-activated drafts. Delete the rest
  (~20 files) after citation repair.
- Skills: remove `mission-gradient`, `parallax` from live skills (git
  retains); settle throughline↔definition relationship in AGENTS.md.
- Reports/reviews/designs: demote superseded August/early-Sep architecture
  and status reports to dated evidence headers or delete; keep the PICL
  corpus table above as the retain set.
- Re-run doccheck; regenerate `doccheck-report.md`.

**Phase D — reorient the standing docs.**
Update `choir-vision.md`, `README.md`, `docs/README.md`, `ACTIVE.md`, and the
11-mission overview to carry the substrate → mechanism → product stack and
the precommitment-records direction. Rewrite `mission-residues.md` R7–R11
against the new ordering. This is where the owner decision lands.

## Owner decisions needed

1. **Direction confirm**: precommitment records as the product capability
   (substrate → mechanism → product), per the stack above — or a research
   track beside the product?
2. **Prune aggressiveness**: full K3/K4/K6 deletion of superseded
   definitions (~20 files, git retains), or tombstone-everything (safer,
   keeps searchable bodies)?
3. **Carrier decision timing**: does the pending carrier owner decision fold
   into this reorientation (re-scoped against the precommitment path), or
   stay a separate call?
4. **Throughline vs Definition**: fold throughline into definition skill, or
   keep both with an explicit relationship clause?

## Evidence

Scout inventories (this session): DoctrineLayer, SkillLineage, PICLCorpus,
MissionState (full 45-definition ledger), DocsBulk, CodeLedger — transcripts
under `history://<name>` in the session that authored this doc.
