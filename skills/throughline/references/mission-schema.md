# Goal File Schema

Read this reference when creating or migrating a `/goal` file. Routine work
uses the compact goal file and loads evidence only for its active slice.

The source may be YAML frontmatter followed by short Markdown explanation, or
equivalent clearly labelled YAML blocks. Keep the machine-readable record near
the top. The goal file is semantic authority; generated HTML and evidence
archives are projections. See [`../SKILL.md`](../SKILL.md) for the circuit and
the rules these fields serve.

## Minimal Template

```yaml
---
definition_version: 3

start:
  captured_at: <RFC-3339 timestamp>
  source:
    canonical_ref: <branch and commit or other immutable authority ref>
    deploy_identity: <identity or unknown>
  worktrees:
    - path: <absolute or repo-relative path>
      status: clean | dirty | unknown
      class: user_wip | goal_candidate | other_agent_wip | generated_temp | unknown
      owner: <person, agent, or unknown>
      touch: forbidden | read_only | goal_owned
      recovery: <leave in place, branch, stash, patch, or other handle>

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

value:
  better_means: <the divergence being reduced — the loss function>
  goodharting_would_be: <what the metric looks like if faked>

homotopy:
  realism_axis: <the continuous low→high resolution parameter>

boundaries:
  mutation_class: unknown | green | yellow | orange | red | black
  authority_sources: [<ordered sources>]
  must_preserve: [<short invariant list>]
  excluded: [<non-goals>]
  protected_surfaces: [<required for red/black work>]

now:
  status: working | complete | checkpoint_incomplete | blocked_incomplete | superseded
  slice: <one coherent active change>
  source_ref: <current immutable commit/digest>
  deploy_identity: <current observed identity or unknown>
  candidate:
    id: <candidate id or none>
    state: none | paused | rehearsing | frozen | reviewed | ready | discarded | landed
    ref: <current worktree, branch, patch, or none>
    base: <immutable ref or none>
    digest: <content digest when frozen, or none>
    scope: [<path>]
  conjecture:
    id: <conjecture id>
    claim: <what might be true>
    test: <how the current observer would know>
    edge: independence | resource | missing_oracle | frame_lock
    delta_o: <smallest observer upgrade that shrinks the edge>
    scope_if_supported: <domain the claim may assert if the test passes>
    status: proposed | active | testing | supported | weakened | falsified | superseded | promoted_to_assertion
    evidence_refs: [<receipts>]
  decision:
    what: <accepted route or none>
    kind: operational | purpose | architecture | authority | safety | none
    status: settled | proposal | none
    evidence_ref: <immutable decision/evidence ref or none>
    owner_ratification_ref: <required for orchestrator architecture/authority/purpose proposal, else not_applicable>
  belief:
    believed_state: <what the mission currently takes to be true>
    main_uncertainty: <the load-bearing unknown>
    next_observation: <what would most change the picture>
  blocker_or_risk: <none or precise statement>
  next_action: <one safe, executable move or none>

receipts:
  - id: <closed slice>
    boundary: define | implement | terminal
    identity: <immutable commit/artifact identity>
    proof_refs: [<evidence>]
    rollback_ref: <ref>
    disposition: <closed result>
    landing:                       # present only when finish.landing.required
      source_commit: <SHA or not_applicable>
      ci_ref: <run/status or not_applicable>
      deploy_ref: <run/status or not_applicable>
      environment_identity: <build/deploy identity or not_applicable>
      deployed_acceptance: <action/result/accepted IDs or not_applicable>

# optional
weak_measures:
  - name: <short name>
    kind: gate | weak_signal | telemetry
    baseline: <observed value/ref or unknown>
    desired: <direction, threshold, or none>
    decision_use: <what this can change>
    cannot_prove: <what it never certifies>

cell:                            # only for goals building/using durable agent deliberation
  policy_resolution_ref: <immutable policy-resolution/run receipt>
  members: [<mission obligations only>]
  acceptance: <what the cell must produce>

view:
  path: <generated local HTML path or none>
  generator: <command/version or none>
---
```

Add concise Markdown only where it makes the finish, a decision, or a
constraint clearer. Do not duplicate `now` as prose.

## Field Rules

### `start` — a receipt, not a ledger

`start` records the observed entry state. It is immutable apart from a dated
`start_correction` which preserves the original fact, names the correcting
evidence, and explains why it matters. It is not a checkpoint ledger.

Every dirty worktree must be classified. `unknown` is permitted only while
reconciliation is the `now.next_action`; it does not authorize a nearby
mutation. Keep large or sensitive diffs out of the goal file: record paths and
a digest or immutable evidence ref instead.

### `finish` — the contract

`finish.artifact` must be something that can be fetched, observed, or used.
`acceptance` names what the action proves and its evidence class. A test,
review, package, candidate VM, panel agreement, or deployment identity may be
one input to acceptance, but must not be described as completion unless it is
the promised artifact and the stated claim is actually observed.

`landing.required: true` for any source or platform-behavior change; name the
CI/deployment/deployed-acceptance receipt floor. A docs-only goal may set it
false with `environment: not_applicable`; it may not silently claim a local
test is deployed proof. No silent substitution.

### `value` — two one-liners

`better_means` is the divergence being reduced — the loss function, not "build
X." `goodharting_would_be` is what the metric looks like if faked — the cheap
proxy that satisfies the letter and misses the goal. If you cannot state both,
the mission is not ready.

### `homotopy` — one line

`realism_axis` is the continuous low→high resolution parameter. A
simplification is valid only if it deforms continuously into the real system;
a fake island is not progress.

### `boundaries` — authority and invariants

`mutation_class`, `authority_sources`, `must_preserve` (invariants), `excluded`
(non-goals), and `protected_surfaces` (required for red/black). For red or
black work, include admissible evidence and rollback per the repository
ceremony.

### `now` — the sole mutable card

One `status`, one `slice`, one `source_ref`, one `deploy_identity`, one
`candidate`, one `conjecture`, one `decision`, three `belief` lines, one
`blocker_or_risk`, one `next_action`. All fields must describe the same
reality. Do not retain a second `next probe`, dashboard summary, checkpoint
ledger, or hand-written current status elsewhere.

`candidate` subfields: `id`, `state`, `ref`, `base`, `digest`, `scope`. A
candidate is disposable; it is not proof or a new authority. If none is active,
set `id: none`, `state: none`.

`conjecture` subfields: `id`, `claim`, `test`, `edge`, `delta_o`,
`scope_if_supported`, `status`, `evidence_refs`. Carries the bridge conjecture
and its current verdict.

`decision` subfields: `what`, `kind`, `status`, `evidence_ref`,
`owner_ratification_ref`. An orchestrator may settle an `operational` route
inside an owner boundary; a purpose/architecture/authority decision stays
`proposal` until `owner_ratification_ref` exists.

`belief` subfields: `believed_state`, `main_uncertainty`, `next_observation`.
The compact resumable picture — three lines, not a second ledger.

### `receipts` — compact

One receipt per closed boundary. `landing` is present only when
`finish.landing.required: true`. Do not add receipts for dispatch, heartbeat,
dashboard refresh, or routine CI poll.

### `weak_measures` (optional)

Only when a measure changes a real decision. Each: `name`, `kind`, `baseline`,
`desired`, `decision_use`, `cannot_prove`. A weak measure never governs
`now.status` or advances `complete`.

### `cell` (optional)

Only for goals that build or use durable agent deliberation. Policy resolution,
members (mission obligations only), acceptance. Do not copy model/tool/memory
values into the goal.

### `view` (optional)

Generated HTML path and generator version, if used. Never editable authority.

## Migration From v2

- Extract the live purpose into `finish`; capture repo/deploy/WIP facts into
  `start`.
- Collapse `start.candidates` into `now.candidate`.
- Flatten `now.reconciliation.*` into top-level `source_ref`/`deploy_identity`.
- Reduce `now.decision` from nine fields to five; move owner ratification to a
  receipt reference.
- Reclassify `measures` entries as `weak_measures` (with `decision_use` /
  `cannot_prove`) or delete.
- Add `value`, `homotopy`, `now.conjecture`, `now.belief`. Fill honestly;
  `unknown` is valid during reconciliation.
- Delete `finish.not_done_when`; it is derivable from `receipts`.
- Preserve the active mission's owner authority and registry topology; do not
  create a parallel `/goal` file.

## Cut From v2

- `start.candidates` and `start.observed_artifact`.
- `now.reconciliation` nesting.
- Nine-field `now.decision`.
- `measures` as a first-class block (now optional `weak_measures`).
- `receipts.landing` for non-landing receipts.
- `finish.not_done_when`.
- The `problem_ref` / `authorization_ref` / `registry_conformance_ref` receipts
  subfields — project conventions, not schema.
