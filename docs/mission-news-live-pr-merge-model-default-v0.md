# Parallax Mission: News Live + PR Merge + Model Default + Doc Cleanup

**Date:** 2026-06-29
**Status:** active paradoc
**Ledger:** `docs/mission-news-live-pr-merge-model-default-v0.ledger.md`
**Source program:** this document
**Mission graph node:** `news-live-pr-merge-model-default-v0`

## Mission Conjecture

If we (1) merge the combined trace+runtime branch to main via PR, (2) fix
PR #28's security issue and merge, (3) make gpt-5.5-low the default model
for all roles except super (gpt-5.5-high), (4) fix the Universal Wire
edition bootstrap gap so real articles appear on staging, (5) deploy to
staging and verify with the API key connection, and (6) delete/archive
the ~186 docs flagged by the audit — then the audited computer vision is
materially advanced because: the platform has a clean merged substrate
with correct model defaults, a working product surface (real news on
choir.news), and a reduced-noise documentation corpus.

The load-bearing bridge: **merging the combined branch + fixing the
edition bootstrap + deploying to staging + verifying with the API
connection produces a platform where real articles appear on choir.news,
served by the correct model defaults, on a clean merged codebase.** The
product-visible outcome (real articles on choir.news) is the evidence gate
for the whole mission.

## Deeper Goal (G)

The audited computer: `computer = choir_code(artifact_program)`, where the
tape is the program, the program is self-authoring, and every state change
is a typed transaction with provenance.

This mission advances G by:
- Clearing the PR backlog so the substrate is unified (no more dangling branches)
- Setting correct model defaults so the system uses its tokens efficiently
- Making the first product surface (news) actually work end-to-end
- Reducing doc noise from 308 to ~120 living docs so agents can navigate

## Current Branch State

**We are on `combined-trace-runtime` branch**, which has:
- 3 commits ahead of main: the combined #22 (trace redaction) + #27 (runtime
  fixture repairs) work that resolves the circular dependency
- 5 commits behind main: the 5 already-merged PRs (#19, #20, #21, #23, #26)

**Staging (choir.news) is running commit `0fde3628`** — behind main, doesn't
have the 5 merged PRs or the combined trace+runtime work.

**API key system**: The auth service has `POST /auth/api-keys` (requires
WebAuthn session), and the proxy validates `Bearer choir_sk_...` tokens.
The API key system is "half-configured" — the endpoints exist but no key
has been created on staging yet. After deploy, we need to create a key via
browser auth session and use it for headless API testing.

## Operating Model

The mission proceeds in phases, not fully parallel tracks, because there
are deployment dependencies:

### Phase 1 — Code Changes (parallel where possible)

- **Track A:** Rebase `combined-trace-runtime` onto main. Change model
  policy defaults (gpt-5.5-low for all roles except super=gpt-5.5-high).
  Fix PR #28's `.envrc` security issue. Create a PR with all changes.
- **Track B:** The edition bootstrap fix is already written in
  `wire_publication.go` (ensureUniversalWireEdition). Verify it builds and
  tests pass. Include it in the same PR or a separate PR.
- **Track C:** Doc cleanup (delete 53, archive 133). Separate PR (docs-only,
  no CI deploy needed).

### Phase 2 — Merge and Deploy

- Merge the code PR(s) to main
- Monitor CI
- Verify staging deploy (check `https://choir.news/health` for new commit SHA)
- Wait for Node B to pick up the new deployment

### Phase 3 — Staging Verification

- Create an API key on staging via browser auth (or ask the user to do this)
- Use the API key to test Universal Wire: `curl -H "Authorization: Bearer choir_sk_..." https://choir.news/api/universal-wire/stories`
- Verify the edition bootstrap works: the feed should show real articles
  after sourcecycled runs an ingestion cycle
- Verify model defaults: check run metadata shows gpt-5.5-low for non-super roles

### Phase 4 — Doc Cleanup Merge

- Merge the docs PR (no CI deploy needed)

## Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never optimize across):**
- No weakening existing auth security
- No production deploy without staging verification (orange+ mutations)
- Problem Documentation First for any new bug discovered
- Each track works in its own worktree/branch — no cross-track file contention
- No silent conflict resolution
- Model policy change must not break existing per-computer overrides
- Doc cleanup must not delete docs still referenced by the mission graph
  without updating the graph reference first
- PRs must pass CI before merging
- The edition bootstrap must be self-healing (no manual operator action)

**Qualities:**
- Each PR merge verifies CI passes before merging
- Model policy change includes both the TOML default text and the Go
  fallback policy
- News diagnosis produces a strong definitive statement (already found:
  edition alias never bootstrapped in production)
- Doc cleanup records what was deleted and what was archived
- Every commit references the mission and conjecture
- Staging verification uses the API key connection, not just local tests

**Domain ramp:**
- Phase 1: local code changes (build + test)
- Phase 2: merge + deploy to staging
- Phase 3: staging verification with API key
- Phase 4: docs cleanup (no deploy needed)

## Variant (Conjecture Descent) V

```
V = driving conjectures still undecided
  + conjectures whose evidence class is below settlement tier
  + conjectures with no strong definitive statement yet recorded
```

**Initial conjectures:**

- C1 (Rebase + PR): "The combined-trace-runtime branch can be rebased onto
  main and merged via PR without breaking CI." — undecided
- C2 (Model default): "Changing the default model to gpt-5.5-low for all
  roles except super (gpt-5.5-high) can be done in the Go fallback policy
  and TOML default text without breaking per-computer overrides or
  existing tests." — undecided
- C3 (Edition bootstrap): "Making autonomousPublishWireArticleToEdition
  bootstrap the edition document + alias on first use (when
  GetDocumentAlias returns ErrNotFound) fixes the zero-stories bug and
  the API returns real articles after an ingestion cycle." — undecided
  (root cause already found, fix written, needs verification)
- C4 (PR #28 fix): "Removing the `dotenv .env` line from .envrc and keeping
  the AGENTS.md update produces a mergeable PR." — undecided
- C5 (Staging deploy): "After merging, staging deploys the new code and
  the health endpoint shows the new commit SHA." — undecided
- C6 (API key): "An API key can be created on staging via browser auth and
  used to authenticate Universal Wire API calls." — undecided
- C7 (News live): "After deploy + edition bootstrap fix, real
  LLM-synthesized articles appear on choir.news within one sourcecycled
  ingestion cycle." — undecided
- C8 (Doc cleanup): "The 53 DELETE-verdict docs can be git-rm'd and 133
  ARCHIVE-verdict docs moved to docs/archive/ without breaking doc
  checker, mission graph, or agent context packet." — undecided

**V = 8** (all undecided)

## Budget

**Granted:** One overnight session (~8 hours wall-clock, abundant tokens)
**Spent:** 0
**Remaining:** Full budget
**Solvency:** 8 conjectures across 4 phases in 8 hours is feasible. Phases
1-2 are code work (2-3h), Phase 3 is staging verification (1-2h), Phase 4
is docs (1h). If any phase blocks, record as open edge and continue.

## Authority / Bounds

- **Phase 1:** May create branches, write code, run tests. May create PRs.
  May NOT merge PRs without CI verification.
- **Phase 2:** May merge PRs to main (after CI passes). May push to
  origin/main. Must verify staging deploy.
- **Phase 3:** May curl staging API with auth. May create API keys via
  browser auth. May read staging logs. May NOT directly modify Node B
  config (must go through git).
- **Phase 4:** May git rm docs, create docs/archive/, move docs. May update
  doc-authority-manifest.yaml and mission-graph.yaml. May NOT touch
  runtime code.

## Mutation Class / Protected Surfaces

- **Phase 1:** ORANGE/RED (runtime code, model policy, trace, auth)
- **Phase 2:** RED (merge to main, staging deploy)
- **Phase 3:** ORANGE (API key creation, staging verification)
- **Phase 4:** GREEN (docs only)

## Evidence Packet

- CI run status for merged PRs
- Staging deploy identity (commit SHA on choir.news/health)
- Model policy test results (go test ./internal/runtime/...)
- Edition bootstrap test results (go test ./internal/runtime/... -run Wire)
- Universal Wire API response with API key auth (before and after fix)
- Screenshot or curl output showing real articles on choir.news
- Doc count before and after cleanup
- Mission graph integrity check after doc moves

## Heresy Delta

- **Discovered:** 1 (edition alias never bootstrapped — found by Track B)
- **Introduced:** TBD
- **Repaired:** TBD

## Suggested Goal String

```text
Use Parallax on docs/mission-news-live-pr-merge-model-default-v0.md.
Treat the mission document as the single source program and handoff: read
it and its required references, compile or update a compact Parallax State
section in place (state, not log; move history appends to
docs/mission-news-live-pr-merge-model-default-v0.ledger.md), declare the
variant (conjecture descent) and budget, then run the circuit. Four
phases: (1) rebase combined-trace-runtime onto main + change model default
to gpt-5.5-low for all roles except super which uses gpt-5.5-high + fix PR
#28 .envrc security issue + verify edition bootstrap fix in
wire_publication.go, create PR; (2) merge PRs to main, monitor CI, verify
staging deploy on choir.news; (3) create API key on staging via browser
auth, use it to verify Universal Wire returns real articles after edition
bootstrap fix; (4) delete 53 docs + archive 133 docs from audit report.
Each pass states position/blind spot, chooses probe / shift / construct /
settle by which conjecture it will decide, bounds mutation, batches
unambiguous construct sequences with a deviation tripwire, records the
conjecture verdict as a strong clear definitive statement and actual ΔV,
and checks budget solvency. Full suite + consolidation at batch
boundaries; widest checker + independent prover before any exit. Exit only
at settled, open_handoff, blocked, or superseded. Platform behavior
settlement requires repo landing proof in the same document. No claim
outruns its evidence class; no self-checked proofs; no fake islands; no
descent-free passes.
```

## Parallax State

```text
status: working
mission conjecture: if we rebase + merge combined branch + fix edition
  bootstrap + change model defaults + deploy + verify with API key +
  clean docs, then the platform has a clean merged substrate with correct
  model defaults, a working product surface, and reduced doc noise
deeper goal (G): the audited computer — clean substrate, working product,
  reduced noise
witness/spec (A/S): merged PRs + model policy change + real articles on
  choir.news + 308→~120 living docs
invariants / qualities / domain ramp (I/Q/D):
  I: no auth weakening, no deploy without staging proof, problem-doc-first,
     no mission graph breakage, no silent conflict resolution, PRs pass CI
  Q: CI passes before merge, model policy tests pass, edition bootstrap is
     self-healing, doc cleanup recorded, staging verified with API key
  D: staging (choir.news) is the acceptance environment
variant (conjecture descent) V: 8 undecided conjectures (C1-C8); V=8;
  last ΔV: 0 (mission start)
budget: granted=8h overnight; spent=0; remaining=full; solvent
authority / bounds: see Authority section above
mutation class / protected surfaces:
  Phase 1: ORANGE/RED (runtime, model policy, trace, auth)
  Phase 2: RED (merge, deploy)
  Phase 3: ORANGE (API key, staging verification)
  Phase 4: GREEN (docs only)
evidence packet: CI status, staging SHA, model policy tests, Wire API
  response with API key, doc counts, mission graph integrity
heresy delta: discovered=1 (edition alias), introduced=0, repaired=0
position / live conjectures / open edges:
  Position: on combined-trace-runtime branch, 3 ahead / 5 behind main.
  Can see: the 5 merged PRs on main, the combined #22+#27 work on this
  branch, the edition bootstrap fix already written in wire_publication.go,
  the problem doc documenting the root cause, staging health endpoint.
  Cannot see: whether sourcecycled is running on Node B, whether the
  edition bootstrap fix passes tests, whether staging will deploy cleanly.
  Live conjectures: C1-C8 all undecided.
  Open edges: PR #28 .envrc fix not yet done. Model policy change not yet
  done. Doc cleanup not yet done. API key not yet created on staging.
next move: Phase 1 — rebase combined-trace-runtime onto main, verify
  edition bootstrap fix builds and tests pass, change model policy
  defaults, fix PR #28 .envrc, create PR with all changes.
ledger file: docs/mission-news-live-pr-merge-model-default-v0.ledger.md
version / lineage: v1 (revised to include deployment workflow and API key)
learning state: edition bootstrap root cause found and documented (see
settlement: not yet — mission just started
```

## Track Details

### Phase 1: Code Changes

**1a. Rebase combined-trace-runtime onto main:**
```
git fetch origin
git rebase origin/main
```
Resolve any conflicts. The 5 merged PRs on main should not conflict with
the trace+runtime work since they touch different files.

**1b. Verify edition bootstrap fix:**
The fix is already in `internal/runtime/wire_publication.go` —
`ensureUniversalWireEdition` bootstraps the edition on first use. Run:
```
go build ./...
go test ./internal/runtime/... -run Wire
```

**1c. Change model policy defaults:**
In `internal/runtime/model_policy.go`:
- `defaultModelPolicyText()` TOML: all roles except super use
  `reasoning = "low"`, super uses `reasoning = "high"`
- `fallbackModelPolicy()` Go struct: same reasoning changes
- Update any tests that assert specific reasoning values

**1d. Fix PR #28 .envrc security issue:**
- Checkout PR #28's branch
- Remove `dotenv .env` line from `.envrc`
- Keep the AGENTS.md update
- Commit the fix

**1e. Create PR:**
Create a PR with the rebased branch + model policy change + edition fix.
Title: "feat: combined trace+runtime merge + model defaults + wire edition bootstrap"

### Phase 2: Merge and Deploy

- Verify CI passes on the PR
- Merge to main: `gh pr merge <num> --squash`
- Monitor CI on main: `gh run list --limit 5`
- Check staging health: `curl https://choir.news/health | python3 -m json.tool`
- Verify the deployed commit SHA matches main

### Phase 3: Staging Verification

- Create an API key on staging:
  - Either via browser auth session (register passkey → POST /auth/api-keys)
  - Or ask the user to create one and provide the secret
- Test Universal Wire with API key:
  ```
  curl -H "Authorization: Bearer choir_sk_..." https://choir.news/api/universal-wire/stories
  ```
- If sourcecycled is running, wait for an ingestion cycle and check again
- If sourcecycled is NOT running, document as open edge (staging-ops concern)
- Verify model defaults in run metadata (check trace or run records)

### Phase 4: Doc Cleanup

- Delete 53 DELETE-verdict docs (from /tmp/choir-doc-audit-report.md)
- Create docs/archive/ and move 133 ARCHIVE-verdict docs there
- Update mission-graph.yaml and doc-authority-manifest.yaml
- Verify doc checker passes
- Commit and push (docs-only, no CI deploy needed)

## References

- `docs/overnight-pr-review-verdicts-2026-06-29.md` — PR review verdicts
- `internal/runtime/model_policy.go` — model policy defaults
- `internal/runtime/universal_wire.go` — Universal Wire API handler
- `internal/runtime/wire_publication.go` — edition bootstrap fix (already written)
- `/tmp/choir-doc-audit-report.md` — full doc audit report with verdicts
