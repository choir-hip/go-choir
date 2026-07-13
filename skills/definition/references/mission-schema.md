# Mission Definition Authoring Schema

Read this reference when creating a new mission Definition, migrating an older
Definition, or changing its kernel/schema. Routine execution and resumption use
the compact rules in `SKILL.md` and need not load this file.

## Contents

- Recommended document sections
- Definition node schema
- Determined state schema
- Canonical state capsule schema
- Conjecture schema
- Evidence record schema
- Human escalation schema
- Assurance and second-opinion schemas
- Definition operators

## Recommended Document Sections

Use only load-bearing sections:

```text
# <Mission Name>

## Harness Invocation Semantics
## Source Authority Order
## Real Artifact / Object Of Work
## Mission Purpose And Non-Purpose
## Definition Graph
## Canonical State Capsule
## Invariants
## Authority Boundaries
## Value Criterion
## Homotopy / Realism Parameters
## Conjecture And Belief State
## Variant / Progress Measure
## Execution Operators
## Receding-Horizon Control Loop
## Dense Feedback Channels
## Evidence Index
## Completion Semantics
## Escalation Rules
## Forbidden Collapses
## Rollback And Resumption Policy
## Mission Report Policy
## Suggested Goal String
```

## Definition Node Schema

Common node kinds:

```text
term object mission boundary invariant observable status operator
evidence_class authority_rule forbidden_collapse completion_semantics
escalation_rule formalization_seam rollback_rule conjecture variant
homotopy_parameter
```

Common statuses:

```text
unresolved proposed contested under_deliberation testing settled promoted
weakened falsified invalidated superseded requires_human_authority
```

```yaml
id: <stable-id>
kind: <node-kind>
status: <node-status>
source: user-stated | observed | inferred | reviewer | formal-check | worker-report
term: <name>
definition: <what it means>
non_definition: []
examples: []
counterexamples: []
observables: []
execution_effect: []
forbidden_collapses: []
formalization:
  status: not-applicable | candidate | required | done | blocked
  note: <proof obligation or executable checker>
settlement:
  rule: <what settles the node>
  settled_by: orchestrator | human | formal-check | reviewer | evidence
  invalidation_triggers: []
```

## Determined State Schema

```yaml
determined_state:
  settled:
    - claim: <authoritative statement>
      source: user-stated | observed | settled-definition | operational-preference
      execution_effect: <what this changes>
  contested:
    - node: <definition id>
      issue: <why not settled>
      next_resolution_step: <critical-process step>
  open:
    - node: <definition id>
      missing: <what must be defined>
```

## Canonical State Capsule Schema

```yaml
state_capsule:
  schema_version: 1
  updated_at: <timestamp>
  kernel_digest: <digest excluding mutable capsule and generated views>
  expected_parent_or_authority_ref: <reconciliation identity>
  status: working | complete | checkpoint_incomplete | blocked_incomplete | superseded
  current_subgoal: <id>
  active_frontier: [<node-or-slice-id>]
  settled_receipts:
    - id: <id>
      status: <status>
      artifact_ref: <ref>
      evidence_refs: []
      rollback_refs: []
  artifact_identity: {source: <ref>, build: <ref>, deploy: <ref>}
  locks: []
  open_findings: []
  belief_changes: []
  highest_impact_remaining_uncertainty: <node or claim>
  next_executable_probe: <next safe, valuable, in-bound action>
  evidence_index_refs: []
  invalidation_triggers: []
```

## Conjecture Schema

```yaml
id: <conjecture-id>
kind: conjecture
status: proposed | testing | settled | weakened | falsified | superseded
claim: <what might be true>
test: <how the current observer would know>
edge:
  blind_spot: <what this observer cannot see>
  class: independence | resource | missing_oracle | frame_lock
observer_upgrade: <smallest shift that shrinks the edge>
scope_if_supported: <domain over which the claim may be asserted>
falsifier: <fastest observation that would kill the claim>
execution_effect: <what changes if supported or falsified>
```

## Evidence Record Schema

```yaml
claim: <scoped claim>
definition_node: <node id>
evidence_class: <class>
source: <file/tool/command/trace/reviewer>
command_or_observation: <exact command or observation>
artifact_path: <path or URI>
result: <observed result>
uncertainty: <remaining edge or caveat>
promotion_relevance: <what this authorizes, if anything>
```

## Human Escalation Schema

```yaml
human_escalation:
  node: <definition id>
  issue: <why orchestration cannot settle it>
  options:
    - choice: <option>
      execution_consequence: <what happens if chosen>
  recommendation: <recommended choice>
```

## Assurance And Second-Opinion Schemas

```yaml
assurance_profile:
  risk_class: low | medium | high | protected | irreversible
  novelty: routine | adjacent | novel
  evidence_floor: <minimum executable evidence>
  independent_verifier: required | optional
  second_opinion_tier: none | compact | standard | full
  required_model_families: <count or list>
  per_reviewer_timeout: <duration>
  total_compute_or_token_budget: <bound>
  escalation_triggers: [surprise, dissent, unique_blocker, weak_evidence, protected_surface, ratchet_drift]
```

```yaml
second_opinion_request:
  node: <definition id>
  unresolved_question: <specific question>
  expected_decision_impact: <what could change>
  why_internal_deliberation_is_insufficient: <reason>
  chosen_tool: <tool>
  exact_model_or_family: <identity if knowable>
  compute_tier: internal | normal_external | premium
  timeout: <hard bound>
  token_or_cost_budget: <hard or estimated bound>
  max_output_shape: <verdict, counterexample, execution effect, etc.>
```

## Definition Operators

```text
define split merge narrow widen counterexample operationalize formalize probe
shift construct verify request_second_opinion settle weaken falsify invalidate
supersede promote escalate monitor
```

Every operator must produce an observable result. Commit only at a Git
durability boundary defined by the core skill.
