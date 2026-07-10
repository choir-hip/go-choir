# Documentation Authority Reduction

## Harness Invocation Semantics

```text
/goal docs/definitions/documentation-authority-reduction-2026-07-09.md
```

Read this document as executable semantic authority for the documentation
reduction. Execute the safe, green/yellow work until the live reading system and
retrieval-reduction evidence is verified. The owner has explicitly directed
mass deletion of historical material that pollutes retrieval. Do not treat
moving a file to `docs/archive/` as a solution: the worktree, not the archive
directory, is the retrieval boundary.

## Source Authority Order

1. This Definition for documentation-authority reduction.
2. Owner direction, 2026-07-09/10: settle a minimal permanent semantic set and
   delete historical material from the retrievable worktree because old docs
   corrupt search/retrieval. Git history is the rollback surface; an archive
   directory inside the worktree is not retained merely for nostalgia.
3. `docs/choir-doctrine.md` and `AGENTS.md`.
4. Pre-purge Git history at `b6fbd598` for the settled docs-truth evidence and
   unpromoted Beads proposal; neither is current authority.
6. Observed repository state from `cmd/doccheck`, the authority manifest, and
   the mission graph.

## Mutation Class

Classification and authority correction are **green**. Checker pressure is
**yellow**. The deletion pass is **black** because it removes tracked history
from the worktree, but the owner has authorized the direction. Each deletion
batch still needs a pre-delete rollback commit, an explicit retain set, and
post-delete link/packet verification.

## Real Artifact / Object Of Work

A new agent can read one small, deterministic packet and learn:

1. Choir's settled semantics and authority boundaries;
2. what code/staging evidence establishes now;
3. the one active top-level Definition; and
4. that historical evidence is available through Git history but absent from
   ordinary workspace retrieval.

The object is not a lower Markdown byte count. It is a mechanically legible
authority boundary between permanent semantics, mutable state, active work,
evidence, and history.

## Mission Purpose And Non-Purpose

**Purpose:** reduce default reading context, remove parallel current-state
authority, delete retrieval-polluting history from the worktree, and make
future drift structurally detectable.

**Non-purpose:**

- Not a rewrite of Choir product semantics.
- Not a premature Beads/YAML authority flip.
- Not a mandate that every current source be manually catalogued forever.
- Not a second global mission-control system; this Definition governs only the
  reduction and handoff to the durable documentation system.

## Definition Graph

### K1. `semantic_kernel`

```yaml
id: semantic-kernel
kind: object
status: proposed
source: owner-stated + observed
definition: >-
  The small, hand-maintained set of terms, invariants, authority rules, and
  semantic migrations that survives implementation changes.
non_definition:
  - Current deployment facts.
  - Service, command, model, or VM-materializer names.
  - Active mission status.
  - An architectural decision without a semantic execution effect.
observables:
  - Each record has stable id, scope, non-definition, settlement source,
    evidence/enforcement seam, successor, and invalidation trigger.
execution_effect:
  - Only this kernel and explicit domain contracts may make global semantic
    claims.
settlement:
  rule: Owner accepts the compact record schema and the initial promoted set.
  settled_by: human
```

### K2. `living_read_path`

```yaml
id: living-read-path
kind: authority_rule
status: proposed
source: observed
definition: >-
  The default agent/human reading path has eleven documents or generated views:
  README, AGENTS, doctrine, semantic registry, NOW, ACTIVE, four on-demand
  domain contracts (computer/promotion, runtime/authority, Texture/artifact,
  source/publication), and exactly one active top-level product Definition.
  Narrowly scoped maintenance Definitions may operate outside the default read
  path and may not override that product Definition.
non_definition:
  - Every useful document in the repository.
  - Archive, evidence, proposal, or ledger contents.
observables:
  - `docs/README.md` exposes only these lanes and makes cold-history access
    explicit.
  - No archive item is described as canonical/current by the reading path.
execution_effect:
  - New orientation docs may not add competing current authority. Maintenance
    Definitions remain discoverable from the generated work view, not the
    product-semantic packet.
settlement:
  rule: The reduced portal and generated views resolve to existing authority
    sources and pass strict-live checks.
  settled_by: evidence
```

### K3. `cold_history`

```yaml
id: cold-history
kind: boundary
status: settled
source: owner direction, 2026-07-10
definition: >-
  Historical mission reports, retired designs, raw ledgers, review transcripts,
  and evidence are not ordinary workspace-retrieval material. They leave the
  worktree after current learning is represented by a retained authority; Git
  history is rollback and forensic access, not an onboarding corpus.
non_definition:
  - Deleting a living authority or a required legal/product contract.
  - Relocating old prose under `docs/archive/` and calling retrieval clean.
execution_effect:
  - Archive, evidence, ledger, proposal, and superseded mission packages become
    deletion candidates unless named in the retain set.
  - The authority manifest lists only surviving current documents.
settlement:
  rule: Owner direction, 2026-07-10.
  settled_by: human
```

### K4. `deletion_qualified`

```yaml
id: deletion-qualified
kind: rollback_rule
status: settled
source: owner direction, 2026-07-10; AGENTS.md worktree hygiene
definition: >-
  A deletion package is qualified when it has a precise path set, a named live
  successor or explicit classification as raw process residue, a pre-delete
  rollback commit, and a post-delete proof that the live packet has no broken
  links. Git history is the retained copy; a second in-worktree archive is not
  required.
non_definition:
  - Moving a file to archive and then deleting the archive later.
  - Keeping a stale file merely because it might be interesting someday.
execution_effect:
  - The owner-authorized packages may be deleted in bounded commits after their
    live links and retain set are repaired.
settlement:
  rule: Owner direction plus per-package verifier evidence.
  settled_by: human
```

### K5. `active_work_authority`

```yaml
id: active-work-authority
kind: boundary
status: settled
source: observed; pre-purge Beads proposal at Git commit b6fbd598
definition: >-
  Until a separately proven cutover, each committed paradoc/Definition remains
  the source of its own current mission state. The YAML mission graph is the
  discoverability and historical-corpus index, and Beads remains shadow work
  state. A future cutover must change the graph header, operating skill, CI,
  index, and round-trip proof together.
non_definition:
  - Beads being canonical because a proposal says it should be.
  - A historical graph node being active work.
execution_effect:
  - This mission curates a small active-work view from directly confirmed
    Definitions, repairs index metadata only where evidence is clear, and does
    not bulk-convert the historical corpus or flip the authority source.
settlement:
  rule: Defined by observed current contracts.
  settled_by: orchestrator
```

### K6. `retrieval_pollution`

```yaml
id: retrieval-pollution
kind: boundary
status: settled
source: owner-stated + observed repository audit, 2026-07-10
definition: >-
  Any old prose, raw agent transcript, ledger, proposal revision, or archived
  mission visible to generic workspace search can steer an agent toward retired
  semantics even when the file is labeled historical.
non_definition:
  - A deliberate Git history lookup for forensic recovery.
  - A surviving living contract cited by the default packet.
observables:
  - 331 Markdown files exist under docs; 310 are outside the proposed 21-file
    retain set and occupy about 9.2 MB.
  - docs/archive has 178 files, ledgers occupy about 2.35 MB, and raw
    agent-consensus evidence has 135 files.
execution_effect:
  - Delete by qualified package, beginning with raw process residue, not by
    creating another search-visible archive or preservation index.
settlement:
  rule: Owner direction, 2026-07-10.
  settled_by: human
```

## Determined State Snapshot

```yaml
determined_state:
  settled:
    - claim: The repository needs a smaller default reading path.
      source: user-stated
      execution_effect: retain only current authority in the worktree.
    - claim: Historical docs must be mass-deleted because generic retrieval finds and follows them.
      source: user-stated, 2026-07-10
      execution_effect: historical material is a deletion target, not an archive target.
    - claim: doccheck scanned 346 Markdown documents, found 511 warnings, and exits report-only.
      source: observed tool result, 2026-07-09
      execution_effect: structural-live validation must be separated from archive reporting.
    - claim: The authority manifest has 159 entries and the graph has duplicate/missing-path state.
      source: observed tool result, 2026-07-09
      execution_effect: do not treat either registry as exhaustive until Phase A repair.
  open:
    - node: semantic-kernel
      missing: owner acceptance of the promoted initial term/invariant set.
    - node: deletion-package-boundaries
      missing: precise package order and the small retain-set exceptions.
```

## Execution Phases

### Phase A — Truthful live control plane (green)

- Create the compact authority schema and this mission's inventory receipt.
- Repair index metadata where the direct paradoc/Definition evidence is clear;
  do not let historical graph warnings define current work and do not add
  history merely to clear warnings.
- Remove or redirect dead manifest entries.
- Reduce `docs/README.md` to four lanes: semantics, NOW, ACTIVE, history.

### Phase B — Generated read path and strict-live checker (yellow)

- Introduce the semantic registry and deterministic bootstrap README/NOW/ACTIVE
  views; generate them only when their evidence inputs have stable ownership.
- Split doccheck into strict live structural checks and full report-only scans.
- Gate only new live authority failures; historical vocabulary remains a
  differential discovery signal.

### Phase C — Retrieval reduction (black, owner-authorized)

- Commit the current authority reduction so the deletion pass has a rollback
  reference.
- Delete raw process residue first: `docs/evidence/**` and all `*.ledger.md`.
- Delete `docs/archive/**`, obsolete Definitions, proposal/review chains, and
  non-retained mission documents; repair surviving live links in the same
  package.
- Keep only the explicit current retain set, the slim authority manifest, and
  the minimal graph metadata needed by the operating contract.
- Verify the default packet, manifest, and source tree after every bounded
  deletion commit.

## Proposed Retain Set And Deletion Packages

This is a **proposed** retain set. It is intentionally expressed as exact paths
so the deletion pass can fail closed if a current dependency is discovered.

```yaml
retain_markdown:
  - docs/README.md
  - docs/choir-doctrine.md
  - docs/semantic-registry.md
  - docs/NOW.md
  - docs/ACTIVE.md
  - docs/computer-ontology.md
  - docs/runtime-invariants.md
  - docs/texture-agentic-invariants-2026-06-13.md
  - docs/source-external-data-publication.md
  - docs/current-architecture.md
  - docs/platform-os-app-state.md
  - docs/conjecture-assertion-ledger-2026-06.md
  - docs/heresy-detectors.md
  - docs/why-texture-2026-06-15.md
  - docs/agent-product-doctrine.md
  - docs/choir-prompting-invariants.md
  - docs/memo-problem-documentation-first.md
  - docs/legal/privacy-policy.md
  - docs/legal/terms-of-service.md
  - docs/definitions/og-dolt-heresy-completion-2026-07-08.md
  - docs/definitions/documentation-authority-reduction-2026-07-09.md
retain_machine_metadata:
  - docs/doc-authority-manifest.yaml
  - docs/mission-graph.yaml
```

| Package | Scope | Observed size | Required repair before deletion |
| --- | --- | ---: | --- |
| `D1-raw-process` | all `docs/evidence/**` and every `*.ledger.md` | 148 evidence files; 37 ledgers / ~2.35 MB | Remove evidence/ledger links from living docs and graph; preserve current conclusions in the retained Definition or doctrine. |
| `D2-archive` | remaining `docs/archive/**` | 178 files / ~5.7 MB before overlap with D1 | Rewrite or remove living links from README, doctrine, computer/current architecture, runtime, platform state, active Definition, desktop README, and specs README. |
| `D3-superseded-source` | old Definitions, proposals, reviews, missions, and stale top-level Markdown | 81 Markdown files after D1/D2 | Replace any remaining live claim with a retained contract; remove their graph/manifest nodes. |
| `D4-design-assets` | six historical `docs/assets/design-language/*.png` files | ~6.8 MB of non-Markdown docs corpus is mostly D1 evidence plus these images | Confirm no frontend/build path consumes them; remove their source-design references. |

At the audited snapshot this leaves 21 Markdown authorities plus two
machine-readable metadata files and deletes 310 Markdown candidates (~9.2 MB)
plus 137 non-Markdown candidates. The package order is deliberate: raw process
residue first, then historical prose, then superseded top-level source forms.

## Variant / Progress Measure

```yaml
variant:
  competing_live_authority_surfaces: 0  # default packet excludes graph/Beads/history
  broken_live_graph_records: 205        # full-corpus index debt; not active-work truth
  dead_manifest_entries: 0
  unclassified_default_read_docs: 0
  unharvested_large_ledger_chains: 0
  retrieval_polluting_markdown_candidates: 0
  retrieval_polluting_nonmarkdown_candidates: 0
```

## Forbidden Collapses

- archive path -> cold/non-authoritative by itself.
- fewer files -> clearer semantics.
- report-only doccheck -> verified authority graph.
- owner decision -> still unresolved because experiments remain.
- Beads proposal -> completed authority cutover.
- retained Git history -> durable learning was extracted.
- evidence receipt -> current product fact.

## Completion Semantics

This mission is complete only when the compact reading path is live, strict-live
validation proves it has no structural authority failures, and the qualified
deletion packages have removed historical retrieval material from the worktree.
The remaining `docs/` corpus must equal the explicit retain set plus required
machine-readable authority metadata.

## Rollback And Resumption Policy

All green/yellow changes are individually revertible. Every move records source
and destination paths. Deletions name a pre-delete commit and retained hash or
successor. No production behavior changes are in scope.

## Run Checkpoint & Resumption State

```yaml
run_checkpoint_and_resumption_state:
  status: complete
  last_checkpoint: D3/D4 final deletion verified after D1/D2.
  current_artifact_state: >-
    The default path is a bounded eleven-document content packet plus router;
    legacy graph/index debt is report-only and cannot redefine active work.
  what_shipped:
    - Four-lane docs router: semantics, NOW, ACTIVE, history.
    - Derived semantic registry and dated NOW/ACTIVE views.
    - doccheck --mode=live structural gate; --mode=full retains corpus reporting.
    - Retention baseline, superseded by retrieval-driven deletion authority.
    - D1 removed 148 raw evidence artifacts and 37 mission ledgers.
    - D2 removed the remaining 175 archive documents and collapsed the manifest
      from 152 records to 23 and the mission graph from 175 nodes to two.
    - D3 removed 81 superseded Markdown sources; D4 removed six unused design
      images. Retained docs and adjacent guides were repaired to stop naming
      deleted files as live dependencies.
  what_was_proven:
    - 310 of 331 docs Markdown files are outside the proposed 21-file retain set.
    - Archive has 178 files; raw ledgers have 37 files / about 2.35 MB; raw
      agent-consensus evidence has 135 files.
    - Strict-live packet validation passes with zero failures after all deletes.
    - The docs corpus fell from 331 to exactly 21 Markdown files plus the two
      required YAML authority indexes.
    - The retained architecture and operating docs no longer link into
      `docs/archive/**`; historical detail requires deliberate Git archaeology.
  unproven_or_partial_claims: []
  highest_impact_remaining_uncertainty: none for this documentation mission
  next_executable_probe: none; open a new Definition before expanding the live set
  suggested_goal_string: /goal docs/definitions/documentation-authority-reduction-2026-07-09.md
  evidence_artifact_refs:
    - b6fbd598 (pre-delete authority/retrieval baseline)
    - 2783a97a (D1 raw evidence and ledger deletion)
    - 8f62fe3b (D2 archive deletion and minimal manifest/graph)
  rollback_refs:
    - b6fbd598
    - 2783a97a
    - 8f62fe3b
```

## Suggested Goal String

```text
/goal docs/definitions/documentation-authority-reduction-2026-07-09.md
```
