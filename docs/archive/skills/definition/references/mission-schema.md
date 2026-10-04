# Goal File Schema

Read this reference when creating or migrating a `/goal` file. Routine work
uses the compact goal file and loads evidence only for its active slice.

The source may be YAML frontmatter followed by short Markdown explanation, or
equivalent clearly labelled YAML blocks. Keep the machine-readable record near
the top. The goal file is semantic authority; generated HTML and evidence
archives are projections.

## Minimal Template

```yaml
---
definition_version: 2

start:
  captured_at: <RFC-3339 timestamp>
  source:
    canonical_ref: <branch and commit or other immutable authority ref>
    deploy_identity: <identity or unknown>
  worktree_inventory:
    status: reconciled | reconciling
    evidence_ref: <inventory receipt or unknown>
    preservation_rule: <required while reconciling>
  worktrees:
    - path: <absolute or repo-relative path>
      status: clean | dirty | unknown
      class: user_wip | goal_candidate | other_agent_wip | generated_temp | unknown
      owner: <person, agent, or unknown>
      touch: forbidden | read_only | goal_owned
      paths_or_digest: <bounded path list or evidence ref>
      recovery: <leave in place, branch, stash, patch, or other handle>
  candidates:
    - id: <stable candidate id>
      ref: <worktree, branch, patch, or none>
      base: <immutable ref>
      scope: [<path>]
      disposition: paused | active | discarded | landed | unknown
      evidence_ref: <optional immutable evidence ref>
  observed_artifact:
    - claim: <what demonstrably exists now>
      evidence_ref: <observation>
  unknowns:
    - <only facts whose absence can change the next action>

finish:
  deliver: <plain-language outcome for a person or external agent>
  artifact: <specific API, UI, durable record, version, or other object>
  acceptance:
    - action: <product path, command, or observation>
      proves: <scoped claim>
      evidence_class: <local test, deployed proof, human inspection, etc.>
  rollback: <reversal, prior ref, or refusal path>
  landing:
    required: true | false | unknown
    environment: <staging, production, local, or not_applicable>
    required_receipts: [<pushed_commit, ci, deploy, environment_identity, deployed_acceptance>]
  not_done_when:
    - <active candidate or unresolved candidate disposition remains>
    - <only weak signals are green>

boundaries:
  mutation_class: unknown | green | yellow | orange | red | black
  authority_sources: [<ordered sources>]
  must_preserve: [<short invariant list>]
  excluded: [<non-goals>]
  protected_surfaces: [<required for red/black work>]
  completion_evidence_floor: [<evidence classes required before complete>]

measures:
  - name: <short name>
    kind: gate | weak_signal | telemetry
    baseline: <observed value/ref or unknown>
    desired: <direction, threshold, or none>
    decision_use: <what this can change>
    cannot_prove: <what it never certifies>

now:
  status: working | complete | checkpoint_incomplete | blocked_incomplete | superseded
  slice: <one coherent active change>
  question: <one unresolved question that changes execution, or none>
  reconciliation:
    observed_at: <RFC-3339 timestamp>
    source_ref: <current immutable commit/digest>
    deploy_identity: <current observed identity or unknown>
    authority_identities: [<immutable decision/doctrine/registry refs in scope>]
    policy_resolution_ref: <current immutable cell policy-resolution ref or not_applicable>
    worktree_inventory_ref: <current compact inventory evidence ref>
    status: reconciled | reconciling
  candidate:
    id: <candidate id or none>
    state: none | paused | rehearsing | frozen | reviewed | ready | discarded | landed
    ref: <current worktree, branch, patch, or none>
    owner: <person, agent, or none>
    base: <immutable ref or none>
    digest: <content digest when frozen, or none>
    scope: [<path>]
  decision:
    selected: <accepted route or none>
    kind: operational | purpose | architecture | authority | safety | none
    status: settled | proposal | none
    source: owner | observed | orchestrator | formal_check | none
    evidence_ref: <immutable decision/registry/evidence ref or none>
    owner_ratification_ref: <required for orchestrator architecture/authority/purpose proposal, else not_applicable>
    recorded_at: <RFC-3339 timestamp or none>
    consequence: <what execution may now do>
  evidence_refs: [<small current set>]
  blocker_or_risk: <none or precise statement>
  next_action: <one safe, executable move or none>

receipts:
  - id: <closed slice>
    boundary: define | implement | terminal
    commit_or_artifact: <immutable identity>
    proof_refs: [<evidence>]
    rollback_ref: <ref>
    disposition: <closed result>
    problem_ref: <required for a problem-documenting Define, else not_applicable>
    authorization_ref: <repair/mutation authority, else not_applicable>
    candidate_or_evidence_refs: [<frozen candidate or discovery evidence refs>]
    landing:
      source_commit: <SHA or not_applicable>
      ci_ref: <run/status or not_applicable>
      deploy_ref: <run/status or not_applicable>
      environment_identity: <build/deploy identity or not_applicable>
      deployed_acceptance: <action/result/accepted IDs or not_applicable>
    registry_conformance_ref: <create/settle/supersede verification or not_applicable>

view:
  path: <generated local HTML path or none>
  generator: <command/version or none>
---
```

Add concise Markdown only where it makes the finish, a decision, or a
constraint clearer. Do not duplicate `now` as prose.

## Field Rules

### Start Is A Receipt

`start` records the observed entry state. It is immutable apart from a dated
`start_correction` which preserves the original fact, names the correcting
evidence, and explains why it matters. It is not a checkpoint ledger.

Every dirty worktree must be classified. `unknown` is permitted only while
reconciliation is the `now.next_action`; it does not authorize a nearby
mutation. Keep large or sensitive diffs out of the goal file: record paths and
a digest or immutable evidence ref instead.

Use `worktree_inventory.status: reconciling` when known dirty work has not yet
been safely enumerated. Its preservation rule protects that WIP while the sole
next action is read-only inventory. `boundaries.mutation_class: unknown` is
valid in this bootstrap state only; classify it before mutation. A candidate
may likewise be `paused` until its base, scope, and disposition are known.

### Finish Is The Contract

`finish.artifact` must be something that can be fetched, observed, or used.
`acceptance` names what the action proves and its evidence class. A test,
review, package, candidate VM, panel agreement, or deployment identity may be
one input to acceptance, but must not be described as completion unless it is
the promised artifact and the stated claim is actually observed.

For red or black work, include the protected surfaces, admissible evidence, and
rollback in `boundaries` in addition to the repository-required ceremony.
For any source or platform-behavior change, set `finish.landing.required: true`
and name the CI/deployment/deployed-acceptance receipt floor. A docs-only goal
may set it false with `environment: not_applicable`; it may not silently claim a
local test is deployed proof.

### Measures Do Not Govern Status

Use `kind: gate` only for a real invariant or acceptance predicate. Use
`weak_signal` for structural movement and `telemetry` for cost/latency/process
learning. A measure that cannot justify `complete` must say so explicitly.

Do not use a metric that the goal's own documentation predictably changes as a
completion proxy. For example, documentation-citer count can be telemetry, but
not evidence that an extraction succeeded.

### Now Is The Sole Mutable Card

Keep `now` small. Its `status`, `slice`, `reconciliation`, `candidate`,
`decision`, blocker, and `next_action` must describe the same reality.
`reconciliation` is the current observed delta from immutable `start`, not a
second start receipt: it holds only current source/deploy identity and a compact
inventory reference, plus immutable authority/policy identities observed at that
time. A mismatch requires a semantic-diff/reconciliation gate before work
continues. A human decision must enter `now.decision` before the harness acts on
it. Store full deliberation and review material in evidence artifacts, then link
the adjudicated result.

An orchestrator may settle an `operational` route inside an existing owner
boundary. A purpose, architecture, or authority decision synthesized by an
orchestrator remains `proposal` until its `owner_ratification_ref` exists. The
decision must point to the immutable authority/decision evidence rather than
copying a registry into the goal.

If no candidate is active, set `candidate.id: none` and `state: none`; do not
leave a stale candidate hash or old next action behind. If a candidate is
discarded, say why in a receipt and choose the next concrete action.

### Receipts Are Compact

One receipt closes a durable boundary. Include only identities and references
needed to resume, audit, or roll back. Put commands, transcripts, full diffs,
CI logs, and panel outputs in linked evidence. Do not add a receipt or commit
for dispatch, agent output, heartbeat, dashboard refresh, or a routine CI poll.

A problem-documenting Define receipt points to the discovery (`problem_ref`),
the authorized repair/mutation boundary (`authorization_ref`), and the relevant
candidate or evidence. That makes ordering auditable after `now` moves on,
without copying the full problem record into current state.

For a behavior-changing terminal receipt, fill in `landing` with the pushed
commit, CI/deploy result, environment identity, and deployed acceptance action
and result. Definition create, settle, and supersession receipts also link their
registry-conformance verification; the registry itself remains separate.

## Candidate And Review Record

Before an independent review, make the candidate addressable:

```yaml
candidate:
  id: <id>
  ref: <isolated worktree, branch, or patch>
  owner: <person or agent>
  base: <commit>
  scope: [<path>]
  digest: <digest>
  review_question: <one decision the review could change>
  review_receipt: <evidence artifact>
  adjudication: accept | repair | reject | escalate
```

A candidate commit or isolated worktree is review substrate, not canonical
mission state. Material candidate changes require a new digest and a new review
only when the change can alter the reviewed decision. Use a panel only when its
answer can change scope, evidence, rollback, authority, or stopping condition.
If reliable evidence identifies a platform problem requiring a repair, the
problem's code-free Define receipt precedes any repair-code candidate commit.

## Persistent Deliberation Cells

Include this optional section only when the goal itself builds or uses durable
agent deliberation:

```yaml
cell:
  computer_id: <persistent computer identity>
  policy_resolution:
    authority_ref: <computer-owned policy source>
    revision_or_digest: <immutable policy identity>
    observed_resolution_ref: <run/API/product-path receipt>
  members:
    - id: <durable agent identity>
      obligation: <builder, falsifier, verifier, etc.>
      run_or_trajectory_ref: <durable observed execution receipt>
      independence: <different context, tool, source, or model lineage>
  acceptance: <product-path proof of resolved policy, durable memory/restart behavior, and useful output>
```

The goal references computer-owned policy and observed resolution; it does not
copy member model, tool/search, memory, or budget values into a second
configuration authority. “Reasoning budget” must be disambiguated in the
product policy when it matters: reasoning effort, output-token limit, total
activation budget, and spend cap are distinct controls.

## Generated HTML View

For a broad goal, generate a local HTML view from the goal file. It should show
the finish line first, then start/protected WIP, current candidate, proof
obtained and missing, clearly amber weak measures, dissent, and next action.
It must identify source digest, generator version, and generation time.

The view is never an editable second authority. Commit a changed view with the
Define or Implement boundary that changed its source; do not create a separate
refresh commit. The rendering command and localhost serving arrangement belong
to the project implementation, not this schema.

## Migrate A v1 Definition

1. Preserve the active mission's owner authority and registry topology.
2. Extract its real purpose into `finish`, and capture repository/deploy/WIP
   facts into `start` without rewriting history.
3. Turn the live boundary into `now`; collapse earlier locks, reports, and
   panel narratives into receipts with evidence refs.
4. Reclassify metrics as gates, weak signals, or telemetry.
5. Delete or generate duplicate current-state summaries. Do not create an
   unregistered competing `/goal` file.

Use `unknown` honestly when a historical fact cannot be reconciled. A fresh
candidate rehearsal is usually cheaper and safer than reconstructing an old
prediction from prose.
