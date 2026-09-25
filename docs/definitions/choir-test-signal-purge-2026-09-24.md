---
definition_version: 3
definition_id: choir-test-signal-purge-2026-09-24
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-24T23:45:00Z'
  source:
    canonical_ref: main@42eee113
    deploy_identity: staging https://choir.news (K ontology kernel landing in flight)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: choir-ontology-kernel-2026-09-24 (K)
    disposition: in flight - dispatcher Stop deadlock + kernel-mode test
      delivery fixes pushed (42eee113); CI pending. R1.5 sequences before R2.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
  observed_artifact:
    - claim: 'The repo carries 408 *_test.go files / ~3395 test functions.
        A large fraction are low-signal unit tests that pin implementation
        detail, mock echoes, plumbing, or incidental defaults rather than
        observable behavior. They cost CI time, create false confidence, and
        (as the shard-6 hang showed) can deadlock on test-only scaffolding
        that does not reflect production wiring.'
    - claim: 'The kernel cutover already broke four recovery tests that drove
        delivery via actorRT.Sweep (a kernel-mode no-op) - they asserted a
        mechanism, not an outcome, and rotted silently.'

finish:
  deliver: >-
    The test suite keeps only tests that would catch a real bug the E2E suite
    misses. Every deleted test is one whose failure would not reveal a
    user- or contract-visible defect beyond what E2E already covers. AGENTS.md
    carries the authoring rules that stop the pattern from regrowing.
  artifact: >-
    (a) A reduced *_test.go corpus - low-signal unit tests deleted; (b) three
    testing rules added to AGENTS.md; (c) a deletion ledger naming each removed
    test and the reason it was low-signal.
  acceptance:
    - action: 'go build ./... && go test ./... (or the runtime-shard scripts)'
      proves: the surviving suite compiles and passes with the deletions applied
      evidence_class: local test
    - action: 'grep AGENTS.md for the three testing rules'
      proves: the authoring rules are present in the operating contract
      evidence_class: static analysis
    - action: 'review the deletion ledger'
      proves: each deletion carries a reason tied to the low-signal criterion,
        not a blanket removal
      evidence_class: human inspection
  rollback: >-
    git revert of the deletion commits restores every test. Deletions are
    grouped per package/subsystem so a single over-deletion can be reverted
    without re-running the whole mission.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity]

value:
  better_means: >-
    Minimize the count of tests whose failure cannot reveal a real defect the
    E2E suite misses, while preserving every test that guards an observable
    contract, boundary, invariant, transition, precedence, or real error path.
  goodharting_would_be: >-
    Deleting tests to hit a count, or keeping a green suite by removing
    assertions. The cheap proxy is "fewer tests, still green" - satisfied by
    deleting the hard tests and keeping the trivial ones, the exact opposite
    of the goal. A deletion is only honest when the ledger names why the test
    could not catch a real bug.

homotopy:
  realism_axis: >-
    Test granularity: unit (single function, mocked deps) -> integration
    (real store/bus, no network) -> E2E (deployed product path). The mission
    moves the suite toward the high-resolution end without deleting the
    low-resolution tests that still guard a contract E2E cannot reach.

boundaries:
  mutation_class: yellow
  authority_sources:
    - owner directive (this session): delete low-signal unit tests, prefer E2E
    - AGENTS.md testing conventions
  must_preserve:
    - every test that guards an observable contract, boundary, invariant,
      transition, precedence, or real error path an E2E test cannot reach
    - E2E/acceptance tests and deployed-proof harnesses (the keep set)
    - determinism and isolation of surviving tests
  excluded:
    - rewriting surviving tests to a new framework or style
    - adding new tests (out of scope; this is a deletion mission)
    - deleting tests that fail for a real reason (fix or escalate, don't delete
      to green)
  protected_surfaces:
    - none (yellow class; no canonical-write or provider-routing surface)

now:
  status: complete
  slice: low-signal unit tests purged; suite green
  source_ref: main@2fbecebc
  deploy_identity: staging https://choir.news build.commit=ce28e407 (test-only
    change; deploy correctly skipped — no runtime surface touched)
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r15-signal
    claim: >-
      A large share of the 3395 test functions assert implementation detail,
      mock echoes, plumbing, or incidental defaults - their failure would not
      reveal a real defect the E2E suite misses. Deleting them loses no
      protective coverage.
    test: >-
      For each candidate deletion, ask: "if this test failed, what
      user/contract-visible bug would it reveal that E2E would not?" If the
      answer is none, it is low-signal. Spot-check deletions by re-introducing
      the bug they claim to guard and confirming no surviving test catches it.
    edge: missing_oracle
    delta_o: >-
      The deletion ledger forces a per-test reason; a test that cannot name
      the bug it guards is deleted, a test that can is kept. The oracle is the
      reason, not the suite staying green.
    scope_if_supported: >-
      the *_test.go corpus under internal/, cmd/, and pkg/ - unit tests only;
      E2E and acceptance harnesses are excluded
    status: active
    evidence_refs: []
  decision:
    what: >-
      Fan deletion across parallel subagents by package/subsystem; each
      returns a deletion ledger. AGENTS.md gains three testing rules.
    kind: operational
    status: settled
    evidence_ref: owner directive this session
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The corpus is dominated by unit tests added after the code they cover;
      many pin implementation. The keep set is the E2E/acceptance layer plus
      unit tests guarding contracts E2E cannot reach (fencing, epoch conflict,
      poison routing, due-index).
    main_uncertainty: >-
      Which unit tests guard a contract E2E genuinely cannot reach (concurrency
      fences, store invariants, error paths) vs. which only re-assert
      implementation. This is the per-test judgment the subagents must make.
    next_observation: >-
      The first deletion pass's ledger - the ratio of deleted to kept, and
      whether any kept test names a contract no E2E covers.
  blocker_or_risk: >-
    Over-deletion: a test that looks low-signal may guard a subtle invariant
    (epoch fencing, serial-per-actor). Mitigate by requiring each subagent to
    name the contract a kept test guards, and by grouping deletions per package
    for surgical revert.
  next_action: >-
    Author the per-package deletion rubric and fan out subagents; land the
    AGENTS.md rules in the same mission.
receipts:
  - id: test-signal-purge-landed
    boundary: terminal
    identity: main@2fbecebc
    proof_refs:
      - 'CI run 36177280541 success — all Go Test shards green post-purge'
      - 'git diff: 729 test/benchmark functions removed, 13 test files deleted, ~21.7k test lines'
      - 'deploy-impact classify: deploy_needed=false (test-only, no runtime surface)'
      - 'ledger: docs/evidence/test-signal-purge-ledger-2026-09-25.md'
      - 'AGENTS.md: three testing rules added (Testing section)'
    rollback_ref: 'git revert 2fbecebc restores all deleted tests'
    disposition: landed — 729 test functions removed, suite green
    landing:
      source_commit: 2fbecebc
      ci_ref: run 36177280541 (success)
      deploy_ref: none — test-only change, deploy correctly skipped
      environment_identity: 'choir.news build.commit=ce28e407'
      deployed_acceptance: 'suite green in CI shards; go build + go vet clean'
---

## The three AGENTS.md rules (owner-stated, verbatim)

Add under a testing section:

- Never write unit tests after you write code.
- Highly prefer E2E tests as the sole testing mechanism. Use them to verify
  complex features work. At the end of E2E tests, produce a verifiable and
  repeatable artifact.
- If you must test a system in isolation, first write down all the ways it
  could fail, then write the code.

## Deletion rubric (the per-test judgment)

Delete a test when its failure would NOT reveal a real defect the E2E suite
misses. Signals of low signal:

- asserts implementation: wiring, field copies, defaults, forwarding, mock
  echoes, source text - not what a consumer observes;
- asserts a mechanism E2E already exercises end-to-end;
- pins incidental behavior (exact log wording, internal ordering, struct
  shape) that is not a contract;
- is a tautology, a bare not-throw, or a non-empty/length-grew check;
- duplicates another test's coverage of the same path with different inputs.

Keep a test when it guards an observable contract, boundary, invariant,
transition, precedence, or real error path that no E2E test can reach - e.g.
concurrency fences, epoch-conflict handling, poison/dead-letter routing,
store invariants, due-index scheduling. When in doubt, keep and name the
contract.

## Sequencing

R1.5 lands before R2. It is a deletion + docs mission (yellow), not a
behavior change; the landing loop still applies because it touches tracked
source (test files) and AGENTS.md.
