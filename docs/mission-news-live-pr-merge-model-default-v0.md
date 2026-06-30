# Parallax Mission: News Live + PR Merge + Model Default + Doc Cleanup

**Date:** 2026-06-29
**Status:** active paradoc
**Ledger:** `docs/mission-news-live-pr-merge-model-default-v0.ledger.md`
**Source program:** this document
**Mission graph node:** `news-live-pr-merge-model-default-v0`

## Mission Conjecture

If we (1) merge the 5 accepted overnight PRs, (2) fix and merge the circular
PR pair (#22 + #27), (3) make gpt-5.5-low the default model for all roles
except super (which uses gpt-5.5-high), (4) diagnose and fix Universal Wire
so real articles appear on staging, and (5) delete/archive the ~186 docs
flagged by the audit — then the audited computer vision is materially
advanced because: the platform has a clean merged substrate with correct
model defaults, a working product surface (real news on choir.news), and a
reduced-noise documentation corpus.

The load-bearing bridge: **merging accepted work + fixing the model policy
+ diagnosing the news pipeline end-to-end produces a platform where real
articles appear on staging, served by the correct model defaults, on a
clean merged codebase.** The product-visible outcome (real articles on
choir.news) is the evidence gate for the whole mission.

## Deeper Goal (G)

The audited computer: `computer = choir_code(artifact_program)`, where the
tape is the program, the program is self-authoring, and every state change
is a typed transaction with provenance.

This mission advances G by:
- Clearing the PR backlog so the substrate is unified (no more 10 dangling branches)
- Setting correct model defaults so the system uses its tokens efficiently
- Making the first product surface (news) actually work end-to-end
- Reducing doc noise from 308 to ~120 living docs so agents can navigate

## Operating Model

Three parallel tracks, each delegated to background subagents:

- **Track A (PR Merge + Model Default):** Merge accepted PRs, fix circular
  pair, change model policy defaults. This is orange/red class work.
- **Track B (News Live):** Diagnose why Universal Wire returns zero stories
  on staging, fix the pipeline gap, prove real articles on choir.news.
- **Track C (Doc Cleanup):** Delete ~53 docs, archive ~133 docs, update
  ~6 docs. Green class, no runtime behavior change.

The orchestrator (Devin) launches all three tracks in parallel, verifies
each return, and mainlines confident work.

## Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never optimize across):**
- No weakening existing auth security (PR #19, #20 are red class — verify
  before merge)
- No production deploy without staging verification (orange+ mutations)
- Problem Documentation First for any new bug discovered
- Each track works in its own worktree — no cross-track file contention
- No silent conflict resolution
- Model policy change must not break existing per-computer overrides
- Doc cleanup must not delete docs still referenced by the mission graph
  without updating the graph reference first

**Qualities:**
- Each PR merge verifies CI passes before merging
- Model policy change includes both the TOML default text and the Go
  fallback policy
- News diagnosis produces a strong definitive statement about why the
  feed is empty (not "it should work")
- Doc cleanup records what was deleted and what was archived
- Every commit references the mission and conjecture

**Domain ramp:**
- Wave 1: Independent tracks (A, B, C) launch in parallel
- Wave 2: After Track A merges, verify staging deploy
- Wave 3: After Track B diagnosis, fix and verify on staging
- Critical path: Track B (news) is the product-visible outcome

## Variant (Conjecture Descent) V

```
V = driving conjectures still undecided
  + conjectures whose evidence class is below settlement tier
  + conjectures with no strong definitive statement yet recorded
```

**Initial conjectures:**

- C1 (PR merge): "The 5 accepted PRs (#19, #20, #21, #23, #26) can be
  merged to main without breaking CI or staging." — undecided
- C2 (Circular PRs): "PR #22 and #27 can be merged together (or
  restructured) to eliminate the circular dependency without losing the
  trace redaction or runtime fixture repairs." — undecided
- C3 (Model default): "Changing the default model to gpt-5.5-low for all
  roles except super (gpt-5.5-high) can be done in the Go fallback policy
  and TOML default text without breaking per-computer overrides or
  existing tests." — undecided
- C4 (News diagnosis): "Universal Wire returns zero stories on staging
  because of a specific, fixable pipeline gap — not a fundamental
  architecture problem." — undecided
- C5 (News fix): "After fixing the diagnosed gap, real LLM-synthesized
  articles appear on choir.news within one sourcecycled ingestion cycle."
  — undecided
- C6 (Doc cleanup): "The 53 DELETE-verdict docs can be git-rm'd without
  breaking any doc checker, mission graph reference, or agent context
  packet." — undecided
- C7 (Doc archive): "The 133 ARCHIVE-verdict docs can be moved to
  docs/archive/ without breaking doc checker path filters or mission
  graph references." — undecided

**V = 7** (all undecided)

## Budget

**Granted:** One overnight session (~8 hours wall-clock, abundant tokens)
**Spent:** 0
**Remaining:** Full budget
**Solvency:** 7 conjectures across 3 parallel tracks in 8 hours is
feasible. Each track has 2-3 conjectures. If any track blocks, the other
tracks continue.

## Authority / Bounds

- **Track A:** May merge PRs to main (after CI verification). May change
  model policy Go code and TOML defaults. May push to origin/main. Must
  verify staging deploy after merges.
- **Track B:** May read staging logs, curl staging API, read sourcecycled
  state. May fix pipeline code in internal/runtime/wire_*.go and
  internal/runtime/universal_wire.go. May push fixes to main after
  verification. Must run deployed acceptance proof on choir.news.
- **Track C:** May git rm docs, may create docs/archive/ directory, may
  move docs. May update doc-authority-manifest.yaml. May update
  mission-graph.yaml. Must not touch runtime code.

## Mutation Class / Protected Surfaces

- **Track A:** RED (auth PRs #19, #20), ORANGE (model policy, PRs #21,
  #23, #26). Protected: auth/session, gateway/provider, trace/evidence.
- **Track B:** ORANGE (wire pipeline code). Protected: Texture canonical
  writes, sourcecycled ingestion.
- **Track C:** GREEN (docs only).

## Evidence Packet

- CI run status for each merged PR
- Staging deploy identity (commit SHA on choir.news)
- Model policy test results (go test ./internal/runtime/...)
- Universal Wire API response before and after fix
- Screenshot or curl output showing real articles on choir.news
- Doc count before and after cleanup
- Mission graph integrity check after doc moves

## Heresy Delta

- **Discovered:** TBD (diagnosis may reveal new heresies in the news pipeline)
- **Introduced:** TBD (model policy change may introduce new heresies if
  it contradicts doctrine)
- **Repaired:** TBD (PR merges may repair heresies identified in the
  overnight review)

## Suggested Goal String

```text
Use Parallax on docs/mission-news-live-pr-merge-model-default-v0.md.
Treat the mission document as the single source program and handoff: read
it and its required references, compile or update a compact Parallax State
section in place (state, not log; move history appends to
docs/mission-news-live-pr-merge-model-default-v0.ledger.md), declare the
variant (conjecture descent) and budget, then run the circuit. Three
parallel tracks: (A) merge 5 accepted PRs + fix circular PR pair #22/#27
+ change model default to gpt-5.5-low for all roles except super which
uses gpt-5.5-high, (B) diagnose and fix Universal Wire so real articles
appear on staging, (C) delete 53 docs + archive 133 docs + update 6 docs
from the audit report. Each pass states position/blind spot, chooses
probe / shift / construct / settle by which conjecture it will decide,
bounds mutation, batches unambiguous construct sequences with a deviation
tripwire, records the conjecture verdict as a strong clear definitive
statement and actual ΔV, and checks budget solvency. Full suite +
consolidation at batch boundaries; widest checker + independent prover
before any exit. Exit only at settled, open_handoff, blocked, or
superseded. Platform behavior settlement requires repo landing proof in
the same document. No claim outruns its evidence class; no self-checked
proofs; no fake islands; no descent-free passes.
```

## Parallax State

```text
status: working
mission conjecture: if we merge accepted PRs + fix circular pair + change
  model defaults + diagnose/fix news + clean docs, then the platform has
  a clean merged substrate with correct model defaults, a working product
  surface, and reduced doc noise
deeper goal (G): the audited computer — clean substrate, working product,
  reduced noise
witness/spec (A/S): merged PRs + model policy change + real articles on
  choir.news + 308→~120 living docs
invariants / qualities / domain ramp (I/Q/D):
  I: no auth weakening, no deploy without staging proof, problem-doc-first,
     no mission graph breakage, no silent conflict resolution
  Q: CI passes before merge, model policy tests pass, news diagnosis is
     definitive, doc cleanup recorded
  D: staging (choir.news) is the acceptance environment
variant (conjecture descent) V: 7 undecided conjectures (C1-C7); V=7;
  last ΔV: 0 (mission start)
budget: granted=8h overnight; spent=0; remaining=full; solvent
authority / bounds: see Authority section above
mutation class / protected surfaces:
  Track A: RED/ORANGE (auth, gateway, trace, model policy)
  Track B: ORANGE (wire pipeline, Texture writes)
  Track C: GREEN (docs only)
evidence packet: CI status, staging SHA, model policy tests, Wire API
  response, doc counts, mission graph integrity
heresy delta: discovered=0, introduced=0, repaired=0 (mission start)
position / live conjectures / open edges:
  Position: mission start. Can see: PR list, review verdicts, model policy
  code, Universal Wire code, doc audit results. Cannot see: staging logs,
  sourcecycled state, why Wire returns zero stories.
  Live conjectures: C1-C7 all undecided.
  Open edges: none yet.
next move: Launch 3 parallel subagents — Track A (PR merge + model
  default), Track B (news diagnosis), Track C (doc cleanup).
ledger file: docs/mission-news-live-pr-merge-model-default-v0.ledger.md
version / lineage: v0, initial paradoc
learning state: none yet
settlement: not yet — mission just started
```

## Track Details

### Track A: PR Merge + Model Default

**Subagent prompt summary:**

1. Merge PRs #19, #20, #21, #23, #26 to main (verify CI passes first for each)
2. Fix the circular dependency between PR #22 (trace) and PR #27 (runtime):
   - Checkout #27's branch, cherry-pick #22's changes, or merge both into a
     combined branch, resolve conflicts, verify build + tests, merge
3. Drop PR #28's `.envrc` security issue (remove `dotenv .env` line), keep
   the AGENTS.md update, merge the cleaned version
4. Change model policy defaults:
   - In `internal/runtime/model_policy.go`:
     - `defaultChatGPTForegroundModel` stays `gpt-5.5`
     - All roles except `super` get `reasoning = "low"`
     - `super` gets `reasoning = "high"` (was `"medium"`)
     - Update `defaultModelPolicyText()` TOML: all roles except super use
       `reasoning = "low"`, super uses `reasoning = "high"`
     - Update `fallbackModelPolicy()` Go struct: same reasoning changes
   - Run `go test ./internal/runtime/...` to verify
5. Push to main, monitor CI, verify staging deploy

**Conjectures decided by this track:** C1, C2, C3

### Track B: News Live

**Subagent prompt summary:**

1. Diagnose why Universal Wire returns zero stories on staging:
   - Check if sourcecycled is running and fetching sources on Node B
   - Check if source entities are being created in the object graph
   - Check if the wire synthesis pipeline is triggering (processor →
     reconciler → Texture agent)
   - Check if Texture articles are being published to the wire feed
   - Check the `/api/universal-wire/stories` response with auth
2. Identify the specific broken link in the chain
3. Fix it (the code fix may be in sourcecycled, wire_synthesis.go,
   universal_wire.go, or the agent pipeline)
4. Verify end-to-end: source fetch → ingestion → synthesis → Texture
   article → Universal Wire feed shows real articles on choir.news
5. Record a strong definitive statement about what was broken and why

**Conjectures decided by this track:** C4, C5

### Track C: Doc Cleanup

**Subagent prompt summary:**

1. Delete the 53 DELETE-verdict docs (from the audit report in
   /tmp/choir-doc-audit-report.md — the subagent should read the report
   to get the full list)
2. Create `docs/archive/` directory
3. Move the 133 ARCHIVE-verdict docs to `docs/archive/`
4. Update `docs/mission-graph.yaml` to reflect any moved/deleted docs
5. Update `docs/doc-authority-manifest.yaml` to reflect the new structure
6. Update the 6 UPDATE-verdict docs (or mark them as explicitly dated if
   they can't be fully updated)
7. Verify doc checker still passes: run the doccheck workflow or command
8. Commit and push (docs-only, no CI deploy needed)

**Conjectures decided by this track:** C6, C7

## References

- `docs/overnight-pr-review-verdicts-2026-06-29.md` — PR review verdicts
- `docs/mission-universal-wire-agent-pipeline-v1.md` — news pipeline mission
- `internal/runtime/model_policy.go` — model policy defaults
- `internal/runtime/universal_wire.go` — Universal Wire API handler
- `internal/runtime/wire_synthesis.go` — wire synthesis pipeline
- `/tmp/choir-doc-audit-report.md` — full doc audit report with verdicts
