# Mission Ledger: Universal Wire Stabilization v1

## Pass 0 — 2026-07-03 03:00 EDT

**Conjecture:** The Universal Wire pipeline is structurally complete (v1.1
settled the prompt fix) but blocked by a flaky CI test, a failed staging
deploy, and unverified end-to-end publishing.

**Move:** construct (mission document creation + analysis)

**Expected ΔV:** 0 (no conjectures decided, just framing)

**Actual ΔV:** 0

**Conjectures recorded:**
- C1: Flaky test is a test bug, not production data race — SUPPORTED
  (local tests pass without -race; failure message is "missing event
  kind" not "DATA RACE detected"; pattern is async event persistence
  race in test assertions)
- C2: Fixing event polling makes CI green — UNDECIDED
- C3: Staging VMs recoverable by re-running deploy — UNDECIDED
- C4: Sourcecycled cycling and dispatching — STRUCTURALLY SUPPORTED
- C5: Agent pipeline produces real articles on staging — UNDECIDED

**Receipts:**
- CI run 28642495032 (race shard 2 failure)
- CI run 28642494977 (deploy failure + race shard 2 failure)
- Local test pass: `go test ./internal/runtime -run "TestToolLoopEndToEndWithRuntime" -count=3` → ok
- Staging health: `curl https://choir.news/health/ready` → degraded (runtime port 8085 refused)
- Proxy deploy commit: `660eee6633ec1d64b3321b394b31288dd5b165b8`

**Open edges:**
- VM refresh timeout: is it transient (cold boot slow) or systemic (OG
  migration broke guest startup)?
- Are there other tests with the same event-checks-after-terminal-state
  race?

**Next:** Fix the flaky test, push, monitor CI. In parallel, trigger
workflow_dispatch to re-deploy staging.

## Pass 0b — 2026-07-03 03:15 EDT

**Conjecture:** Two substrate-level hypotheses (H1: race-detector CI
model wrong, H2: TLA+ specs stale) should be explicit in the mission so
they're available if the first-pass fix fails.

**Move:** construct (add substrate hypotheses + escalation rule to
mission doc)

**Expected ΔV:** 0 (new conjectures proposed, not decided)

**Actual ΔV:** +2 (V increased from 5 to 7 — but this is discovery, not
zero progress. The mission now carries the owner's substrate hypotheses
explicitly, which changes the route if C2 fails.)

**Conjectures recorded:**
- H1: Race-detector CI model wrong for current architecture — PROPOSED
  (activate if C2 falsified)
- H2: TLA+ specs don't match current architecture — PROPOSED (activate
  if TLA+ check fails or stale invariants found)

**Receipts:**
- Owner input: "simplify. the race detector model, that is something to
  review. tla+ was written some time ago, it may not be ready and stable
  and well designed and well formed for the current architecture. the
  probability it is seems drawn from a sparse base distribution."

**Open edges:**
- H1 and H2 are proposed but not active. They activate on C2 falsification
  or TLA+ failure respectively.
- Escalation rule: if C2 fails, shift from "fix this test" to "is this
  testing approach correct?" — simplify, not patch.

**Next:** Fix the flaky test, push, monitor CI. In parallel, trigger
workflow_dispatch to re-deploy staging. If C2 falsified, activate H1.

## Pass 0c — 2026-07-03 03:30 EDT

**Conjecture:** H1 and H2 were framed as conjectures (uncertain, to be
tested) but they are actually definitions (established by observation
of architecture history). They should be releveled from conjecture to
definition, informing the approach from the start rather than
conditionally. Additionally, the documentation itself needs the same
constructive critique (D3).

**Move:** shift (relevel H1→D1, H2→D2 from conjecture to definition; add
D3 for documentation critique; update Parallax State, variant, and goal
string to reflect the releveling)

**Expected ΔV:** -2 (H1 and H2 removed from V as they are no longer
conjectures)

**Actual ΔV:** -2 (V decreased from 7 to 5; H1/H2 became D1/D2
definitions, not conjectures; D3 added as a third definition)

**Conjectures recorded:**
- D1: Race-detector CI model is from a prior architecture — DEFINITION
  (releveled from H1 conjecture; established by observation that the
  runtime underwent OG migration, actor runtime migration, and wire
  pipeline rewrite without rewriting tests for the new concurrency model)
- D2: TLA+ specs are from a prior architecture — DEFINITION
  (releveled from H2 conjecture; established by observation that specs
  were written before the migrations)
- D3: Documentation needs constructive critique — DEFINITION
  (new; established by observation that docs accumulated through
  multiple migrations without a composition and clarity pass)

**Receipts:**
- Owner input: "i realized that one needed relevling... not a refactor
  but a releveling... is from conjecture to definition. yes, we are due
  a constructive critique of composition and clarity of communication
  in our documentation and systematic description"

**Open edges:**
- D1: How systemic is the event-polling race pattern? (informs whether
  the first-pass fix is sufficient or the race detector model needs
  releveling)
- D2: What do the TLA+ specs actually model? (needs review of specs/)
- D3: Which docs carry the most stale assumptions? (needs review of
  mission docs, architecture docs, doctrine)

**Next:** Fix the flaky test, push, monitor CI. In parallel, trigger
workflow_dispatch to re-deploy staging. Begin D3 documentation critique.
If C2 falsified, shift to D1: review race-detector CI model.
