# Docs Pruning Plan and Result, 2026-10-14

Owner direction (2026-10-14, `reports/owner-notes-2026-10-14.md` section 9):
seriously prune the docs, including the documentation-driven-development
framework overhead; archive much of what is labelled current, delete much of
the archive (git keeps history); do not lose the value of the system; focus on
removing, updating, merging and synthesizing docs with outdated vision,
architecture, terminology or concepts.

Mutation class: **green** (docs only; the only non-doc edits are comment-path
fixes in four Go files and one path in `AGENTS.md`). Rollback: every deletion
and move is one `git revert` away, and `archive/PRUNED-2026-10-14.md` lists
each file with a reason.

## Result

| directory | files before | files after | deleted | moved out | size before | size after |
|---|---:|---:|---:|---:|---:|---:|
| `evidence` | 437 | 203 | 234 | 0 | 10.2 MB | 4.2 MB |
| `problems` | 267 | 230 | 35 | 2 | 1.7 MB | 1.3 MB |
| `archive` | 252 | 117 | 198 | 0 | 7.5 MB | 3.3 MB |
| `reports` | 142 | 69 | 59 | 15 | 3.5 MB | 2.3 MB |
| `definitions` | 78 | 51 | 1 | 26 | 1.4 MB | 1.0 MB |
| top level | 59 | 40 | 5 | 15 | 1.3 MB | 0.8 MB |
| `deck` | 25 | 7 | 18 | 0 | 5.4 MB | 3.3 MB |
| `consensus` | 25 | 3 | 22 | 0 | 0.2 MB | 0.1 MB |
| `reviews` | 14 | 2 | 8 | 4 | 0.2 MB | 0.0 MB |
| `research`, `learn`, `runbooks`, `legal`, `design`, `refactors` | 39 | 39 | 0 | 0 | 2.3 MB | 2.3 MB |
| **total** | **1338** | **761** | **580** | **62** | **33.8 MB** | **18.7 MB** |

Top-level `docs/*.md`: 70 before, 38 after. By action (1338 inventoried files):
580 deleted, 62 moved (55 archived, 7 precommitment originals folded into the
new doc and archived), 20 updated in place, 676 kept untouched.

Rewritten rather than only moved:

- `precommitment-records.md` (new, 57 KB): one canonical doc from eight (the
  Engineering Memo, the Sep-22 theory paper and its condensed copy, the PSA
  working paper, the protocol, predictions-and-scoring and signing designs, the
  predictive-ICRL delta). Originals are in `archive/precommitment/`.
- `choir-vision.md` (rewritten, 9.8 KB): the one idea, the order, PSA as the
  core, multiplayer by design and the organizational operating system, the
  foundations-first correction; absorbs the one-pager's inversion argument.
- `choir-doctrine.md` (44.6 KB to 37.7 KB): naming, links, PSA named as the
  core mechanism, historical narrative and duplicate prose removed. All 82
  rule/heresy ids (`C`, `HYP`, `I`, `H`) are present; only the "Live Heresies"
  sub-headings (no inbound citers) were collapsed.
- `agent-product-doctrine.md`: legacy conductor/processor/reconciler wording
  removed, authority aligned to the four desks, the optimal-harness thesis
  distilled into the harness-minimalism section.
- `ACTIVE.md` (65.9 KB to 21.4 KB): the work view now holds the foundations
  push, the ledger and re-engineering metamissions, the roadmap, persistent
  desks and the app-development metamission. Gate 1 receipts, the World Wire
  successor spine, and the "completed definition" blocks were removed.
- `README.md` (docs router): rewritten as the map below.
- Targeted terminology/status fixes: `current-architecture.md` (Autopaper was
  listed as "Tabled"), `computer-ontology.md`, `platform-os-app-state.md`,
  `source-external-data-publication.md`, `runbooks/platform-dolt-authority-split`,
  `desk-rlm-rectification-plan` (banner), `operational-invariants-register`,
  `memo-diagonal-media-strategy` (banner), the two 10-08 decks and the deck
  treatment (camel-case product names), the app-development metamission
  (World Wire to Autopaper), `world-wire-mission-stack` (superseded banner),
  `NOW.md`, `reports/dead-code-census-2026-10-12.md`.

## Method

1. **Inventory (6 Haiku workers, one per partition).** A mechanical index
   (`path, bytes, last commit, title, inbound references by class (root, skills,
   scripts, code, manifest, graph, router, other docs), status line, retired-term
   counts`) was generated first so workers judged from signals and a head read,
   not from filenames. Partitions: top-level and misc; archive; problems;
   evidence and consensus; reports, reviews, research; definitions. Each worker
   wrote a decisions TSV (action, target, confidence, status, superseded-by,
   needed reference updates, reason, outdated claims) and a notes file.
2. **Plan (coordinator).** Combined the six TSVs (1338 rows), applied the hard
   constraints mechanically (below), resolved merge groups, and spot-checked 14
   random non-high-confidence deletions (all were dead receipts or superseded
   letters).
3. **Apply in four commits** (below), with `scripts/doccheck` and a repo-wide
   link/path check after each, and two Sonnet workers for the judgement
   rewrites (precommitment merge; vision, doctrine, agent-product-doctrine).
   Eight sub-agents in total.

Hard constraints enforced by script, not by trust:

- Nothing referenced by `AGENTS.md`/`CLAUDE.md`, `skills/`, `scripts/` or code
  (comments and tests) was deleted. The only referenced doc that moved is the
  protocol design (cited in `internal/types/commitment.go`; comment updated).
- Deleted docs cited by a still-current doc (another top-level doc, an open
  goal file, a kept report) were converted to archive moves so links survive
  (14 files); research bundle members were kept.
- Manifest entries and mission-graph nodes for removed paths were dropped, for
  archived docs the manifest entry is dropped (the archive is not authority),
  graph nodes follow their moved paths.
- `scripts/doccheck --mode=full`: exit 0 before and after, 78 warnings down to
  61 (see "Verification"). `--mode=live`: 2 failures before and the same 2
  after (pre-existing: two `entrypoint: true` graph nodes; not touched).

## Coordinator overrides of the inventory

- **Landed goal files were not deleted** (the P6 worker proposed deleting 22).
  `AGENTS.md` says keep landed receipts, so they moved to
  `archive/definitions/`. The owner can delete that directory later.
- Kept in place because code, doccheck or `AGENTS.md` names them: `NOW.md`,
  `conjecture-assertion-ledger-2026-06.md`, `world-wire-mission-stack-2026-09-22.md`
  (banner: superseded), `reviews/rlm-implementation-consensus-2026-08-29.md`,
  `definitions/choir-research-fanout-2026-10-10.md`.
- The two 10-14 PDFs (state-and-vision, isolation review) and the PDF
  stylesheet were kept: they are the owner's reading copies of today's reports.
- Proposed problem "digest" docs (m11, effects-red, document-cast, platform
  update) were not created: those records are open or still informative, so they
  stay as they are.
- The held-computer outage postmortem stays in `reports/`; two problem records
  it enumerates were deleted and redirected to it.

## The docs map (what to read)

This is the new `docs/README.md`, summarized.

- **A newcomer**: root `README.md`, then `reports/choir-state-and-vision-2026-10-14.md`
  (plus `strategy-takeaways` and `owner-notes`), `choir-vision.md`,
  `precommitment-records.md`, `learn/`.
- **An agent** (default packet, unchanged, ten documents required by doccheck):
  `README.md`, `AGENTS.md`, `choir-doctrine.md`, `semantic-registry.md`,
  `NOW.md`, `ACTIVE.md`, `computer-ontology.md`, `runtime-invariants.md`,
  `texture-agentic-invariants-2026-06-13.md`, `source-external-data-publication.md`.
- **By task**: goal files (`skills/throughline`, `standing-questions.md`,
  `memo-problem-documentation-first.md`, worked example `definitions/choir-pd1-*`);
  agent/desk behavior (`agent-product-doctrine.md`, `choir-prompting-invariants.md`,
  `re-engineering-decisions-2026-10-12.md`); architecture and state; Autopaper;
  security.
- **Lanes**: `definitions/` (in-flight goal files), `problems/`, `evidence/`,
  `reports/`, `research/`, `learn/`, `runbooks/`, `archive/` (opt-in).

## Proposed CLAUDE.md (AGENTS.md) edits (not applied)

The only edit made to `AGENTS.md` is a path: the precommitment mechanism now
lives at `docs/precommitment-records.md` instead of the Engineering Memo. The
rest is for the owner:

1. **Roadmap pointer.** Replace "The mission roadmap from here to World Wire
   live is `docs/world-wire-mission-stack-2026-09-22.md`" with "Mission
   sequencing lives in `docs/ACTIVE.md` and
   `docs/archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md`." The stack
   is bannered superseded and kept only because this line cites it; once the
   line moves, `world-wire-mission-stack-2026-09-22.md` can go.
2. **Product direction.** Add the tagline ("multiplayer intelligence for the
   learning economy") and one sentence that precommitment records / PSA
   (`docs/precommitment-records.md`) are the core mechanism and that the
   throughline discipline is its builder-side precursor.
3. **Heresy delta.** "Heresy delta (discovered, introduced, repaired)" is
   demanded for red/black work and in Final Evidence. Proposal: keep it only
   when a detector family in `docs/heresy-detectors.md` actually moved, and
   say that file is the CI detector manifest (`scripts/check-heresies.sh`),
   not a doctrine surface. A deeper cut (retire the zero-target detector rows,
   keep only `enforce: zero` ones) is a separate CI change.
4. **Goal-Station Handoff.** Add: landed goal files move to
   `docs/archive/definitions/`; superseded drafts are deleted.
5. **Docs hygiene (new short paragraph).** Dated reports, evidence receipts and
   problem records are pruned once their work is closed and nothing current
   cites them; deletions are logged in a `docs/archive/PRUNED-<date>.md`
   manifest; a new doc must be linked from `docs/README.md` or `ACTIVE.md`.
6. **Reading list.** The "Load those on demand" sentence could point to
   `docs/README.md` as the map instead of enumerating docs.
7. Candidates to slim, not delete: "Convergence Before Patching" (the "Review
   the traces" paragraph is long) and "Restarts End Work": the rules are
   sound; the prose can be halved.

## What was kept on purpose (the value core)

Throughline and goal-file discipline (`skills/throughline`, three worked
examples under `definitions/`, lineage skills in `archive/skills/`),
problem-documentation-first, convergence-before-patching and mutation classes
(in `AGENTS.md`), `standing-questions.md`, the restart rules, the commitment
ledger and the new `precommitment-records.md`, `computer-ontology.md`, the
runtime and Texture invariants, `heresy-detectors.md` (CI reads it),
`operational-invariants-register` (draft), the 202 problem records from 10-01
onward, all 10-13/14 reports, research and `learn/`, and the in-flight and
landed station files.

## Risks and things left for the owner

- **Deleted evidence the inventory judged dead by citation.** 234 evidence
  files went; the heuristic was "cited only by archive, old receipts or nothing,
  and dated before 2026-09-20". Mitigation: the manifest and git. Low-confidence
  rows were kept. Possibly worth a look: `evidence/rlm-replay/goldens` (kept; no
  tracked harness reads it, though the dead-code census says one does).
- **Dangling citations in historical docs.** Archive, problem, evidence and
  report docs still cite deleted siblings (451 path mentions in `archive/`, 46 in
  `problems/`, 48 in `reports/` are unresolved; most predate this prune).
  Current docs (top-level, open goal files, README, ACTIVE) were repaired; the
  link/path check lists the rest. Not worth rewriting history.
- **`problems/` second pass.** 202 records dated 2026-10-01 or later were kept
  unread beyond the status line. P3 flagged 18 resolved ones with fix commits
  already in git as deletion candidates after 30 days (autopaper-arrivals-unanswered,
  c1e-b1-engineering-assignments-cancelled-on-arrival, captures-undisposed-*,
  free-model-429-opens-provider-circuit, live-paper-computer-reclaimed,
  object-graph-registration-and-anchor-gaps, the s0-/s0m-/s2- families,
  texture-research-hollow-revisions). Eleven families (s0m 14, s0 13, s2 17,
  texture 20, m11 13, held 10, ci 10, capsule 8, guest 8, research 8, search 6)
  could be merged into one digest per family.
- **Precommitment doc is long (57 KB) and has open items** (its section 8.3):
  whether rule checks count as runtime scoring, what a desk may see of its own
  settled score, whether hypotheses are mandatory, whether `Retract` is gone,
  and owner ratification of D1, D6 and D13. It was not shortened further so no
  decision text is lost.
- **Doctrine slimming stopped at 37.7 KB.** More would mean cutting normative
  text. The apex doc, `heresy-detectors.md` and `semantic-registry.md` still
  overlap in vocabulary; a real consolidation wants the owner's call on which
  of the `H`-ids still earn their CI row.
- **doccheck hard-codes paths**: `docs/NOW.md` (read packet, high-read list),
  `docs/platform-os-app-state.md`, `docs/conjecture-assertion-ledger-2026-06.md`,
  and a nonexistent `docs/why-texture-background-2026-06-15.md`
  (`cmd/doccheck/main.go` line 1665; the file is `why-texture-2026-06-15.md`).
  Changing the checker would let `NOW.md` (a retired stub) and the 2026-06
  ledger go. Not done: code change, separate mission.
- **Live doccheck still fails with the same 2 errors** as before the prune
  (two `entrypoint: true` nodes in `mission-graph.yaml`; stale graph statuses
  for reeng-metamission, texture-ledger-consumer, platform-dolt per the P6
  worker). Out of scope here.
- **Decks**: 18 earlier pitch/seed variants, treatments, bones and two
  screenshots were deleted; the 2026-10-08 pre-seed pair, its treatment and the
  two screenshots they use remain. `deck/shot-worldwire.png` shows a retired
  name on screen and is kept because the 10-08 decks embed it.
- Code comments citing docs that never existed (`docs/glossary.md`,
  `docs/getting-started.md`, and a few `docs/designs/...`) were left, except
  three that had a clear archive target (fixed).

## Verification

Run on the final tree:

| check | before | after |
|---|---|---|
| `scripts/doccheck --mode=full` | exit 0; 1117 docs; 78 warnings, 289 info | exit 0; 644 docs; 61 warnings, 158 info; no warning that was not already present |
| `scripts/doccheck --mode=live` | 2 failures | the same 2 failures (two `entrypoint: true` graph nodes) |
| `go test ./cmd/doccheck`; `go build` of `internal/types`, `actor`, `keyescrow`, `autoputer` | pass | pass |
| path/link scan of current docs, `AGENTS.md`, skills, scripts, code | n/a | no new dangling reference from a current doc; remaining misses are historical docs or predate the prune |

Warnings that disappeared were dangling-link and reachability warnings for
deleted docs plus the retired-vocabulary hits that lived in them.
