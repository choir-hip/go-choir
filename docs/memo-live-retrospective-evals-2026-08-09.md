# Memo: Live and Retrospective Evals for the RLM Computer

**Date:** 2026-08-09  
**Status:** architecture proposal; not promoted doctrine  
**Mutation class:** green (documentation only)  
**Scope:** multi-model, multi-context, multi-capsule, and multi-realization
evaluation over Choir actors and trajectories

## Thesis

Choir should make every consequential model call and RLM activation capturable
as a frozen evaluation case. The same case can then be executed under different
model, context, orchestration, and capability configurations without giving the
counterfactual trials authority over the live computer.

```text
frozen activation case
  x evaluation variants
  -> isolated trials
  -> typed outcomes, artifacts, traces, cost, and latency
  -> layered graders
  -> eval finding
  -> optional model/context-policy proposal
```

This follows naturally from the architecture in [Persistent RLM Actors and
Capsule-Isolated Go Orchestration](memo-persistent-rlm-actors-2026-08-09.md):

- the trajectory owns durable problem continuity;
- the actor is the accountable identity and address;
- the RLM activation is disposable computation;
- model and connector calls cross trusted brokers;
- contexts, artifacts, typed obligations, and effect proposals have durable
  identities;
- capsules isolate execution;
- the canonical computer event chain remains semantic authority.

An eval is therefore best understood as **counterfactual execution over a
frozen activation case**. The workstream snapshot is necessary but not
sufficient: a fair case also binds the exact actor wake, memory cursor,
context-construction inputs, observation world, capability surface, and temporal
context presented at the execution boundary.

The object being evaluated is not a model in isolation. It is a cognitive
configuration:

```text
model selection
  x semantic context
  x context-construction policy
  x orchestration program
  x capability profile
  x workstream state
```

## Goals

Choir should support:

- retrospective comparison of models over the same historical contexts;
- live-subject observation with isolated baseline and challenger trials;
- changing one actor's model while holding the rest of a trajectory fixed;
- changing one actor's context selection or compaction policy;
- evaluating whole consensus architectures rather than only individual calls;
- evaluating multisupervision protocols, seat independence, and quorum policy;
- comparing local model fan-out with durable agent delegation;
- measuring context efficiency, evidence quality, authority compliance, cost,
  latency, and downstream work settlement;
- safely learning better per-computer model and context policy;
- scaling trials over multiple capsules and execution hosts without inventing
  candidate computers or alternate promotion paths.

The system owns experiment architecture, model registry, routing constraints,
fallback, cycling, and policy updates. Users communicate goals and corrections
through product prompts; they do not directly manipulate an experimental
control plane.

## Evaluation Vocabulary

| Object | Meaning |
| --- | --- |
| Eval subject | The model call, activation, actor shift, or workstream decision being studied |
| Eval case | Frozen input state and contracts needed to evaluate that subject |
| Variant | One explicit change to model, context, orchestration, capability, or resource policy |
| Trial | One isolated execution of a case under a variant |
| Live subject/champion-of-record | The ordinary live activation or supervision-protocol outcome selected by policy before evaluation; authoritative product behavior, not an eval trial |
| Baseline replay | A non-authoritative trial of the live subject's declared configuration against the frozen case |
| Challenger/shadow | A counterfactual lane that can produce eval evidence but no live effects |
| Grader | A deterministic contract, executable verifier, model judge, human signal, or downstream outcome measure |
| Eval finding | An evidence-bound comparison across trials; never promotion authority by itself |
| Policy proposal | A typed recommendation to change system-owned model or context policy through a separate transaction |

The vocabulary must not overload existing objects:

- an eval trial is not a product run that can settle live work;
- a trial realization is not a new `ComputerID`, candidate computer, or route;
- an eval finding is not `RunAcceptance`, a verifier certificate, or a computer
  acceptance event;
- replaying a case is not replaying the canonical event reducer;
- a model judge is not a verifier of effect safety or promotion authority.

## Evaluation Units

Choir should allow evaluation at several nested levels.

### Model-call eval

Hold the semantic request and observations fixed; vary model selection,
reasoning level, sampling parameters, or provider adapter.

Useful for extraction, classification, synthesis, tool-choice, and judge
calibration.

### RLM-activation eval

Hold the trigger and initial workstream snapshot fixed; vary the root model,
context construction, orchestration libraries, model-call portfolio, or
capability profile.

Useful for evaluating CoSuper and Research context virtualization.

### Actor-shift eval

Evaluate one bounded period of responsibility for an actor. A later trial may
use a different model or even a replacement actor identity while inheriting the
same frozen projection of durable workstream obligations. It never acquires the
live work-item identities, mailbox, memory cursor, or disposition authority.

Useful for hot-swappable supervision and recovery.

### Workstream eval

Evaluate a complete trajectory or a causally closed segment: how well the
organization reduced uncertainty, satisfied obligations, handled disagreement,
produced evidence, and reached settlement.

Useful for comparing consensus architectures, delegation graphs, and Super
protocols.

### Realization eval

When environment fidelity matters, reconstruct an exact accepted computer head
in a disposable eval realization and observe read-only or capsule-contained
behavior. This is an execution environment for a case, never a product
identity, serving route, or self-development candidate.

## Frozen Eval Case

The eval case is the reproducibility spine. A proposed Go shape is:

```go
type EvalCase struct {
    CaseID             EvalCaseID
    SchemaVersion      string
    PrivacyClass       PrivacyClass

    ComputerID         ComputerID
    ComputerHead       EventHead
    WorkstreamHead     WorkstreamHead
    TrajectoryID       TrajectoryID
    WorkItemIDs        []WorkItemID
    ActorID            ActorID
    TriggerRef         ObligationRef
    ActorWakeRef       ArtifactRef
    MemoryCursorRef    ArtifactRef
    TemporalContextRef ArtifactRef

    CanonicalContextRef ArtifactRef
    ChoirRenderedRef    ArtifactRef
    ContextPolicyRef   ArtifactRef
    PromptProgramRef   ArtifactRef
    ModuleManifestRef  ArtifactRef
    CapabilityRef      ArtifactRef
    OrchestrationRef   ArtifactRef

    InputArtifactRefs  []ArtifactRef
    ObservationRefs    []ArtifactRef
    TraceRoot          TraceRef
    ContractRefs       []ContractRef

    ProviderPolicyRef  ArtifactRef
    AdapterRefs        []ArtifactRef
    CreatedAt          time.Time
}
```

The case must bind all inputs that could materially change behavior:

- system, role, and assignment instructions;
- active work items, questions, blockers, evidence, and dispositions;
- owner instruction and triggering message/obligation;
- selected artifact and source views;
- compacted memory and its source refs;
- tool/module schemas and descriptions;
- context selection, ordering, truncation, and compaction policy;
- model registry and effective per-computer policy version;
- provider adapter and request-serialization versions;
- capability manifest and budgets;
- orchestration recipe or RLM program refs when present;
- external observations used by the original subject;
- expected schemas, invariants, tests, and evidence contracts.

Current Choir builds initial provider context from several runtime surfaces:
the run prompt, system prompt, tool registry, skill context, temporal context,
persisted memory, compaction state, injected owner instructions, and coagent
updates. Context-pressure thresholds also depend on the selected model's window.
The first eval substrate change should make that assembly explicit rather than
attempting to reconstruct it later from loosely related Trace events.

The manifest must bind the exact actor wake envelope and resume/memory cursor
that caused the activation. A trajectory head or obligation ref alone does not
reproduce current harness state.

## Layered Context Identity

`SemanticContextManifest` should be a layered immutable object, not a single
ambiguous digest.

### Canonical model-agnostic context

This layer contains the unrendered Choir meaning:

- source instructions and their content digests;
- frozen temporal value and locale;
- actor wake, workstream obligations, and memory entries;
- artifact/evidence views and ordering;
- unrendered module and tool definitions;
- context selection, compaction, and truncation decisions;
- budgets and capability descriptions.

### Choir-rendered context

This layer contains the exact system text, skill text, tool catalog, messages,
injected turns, and forced reminder/continuation text handed to the provider
adapter. It is captured at the shared production tool-loop request boundary.

### Provider-wire request

Each trial records the exact adapter-rendered request digest, adapter version,
declared variant, actual provider/model, fallback path, and provider-reported
model identity where available.

## Same Context Across Models

“Same context” has two meanings and both should be recorded.

### Semantic equality

Every same-context variant receives the same canonical and Choir-rendered
context layers: same artifact views, workstream state, instructions, tool
contracts, temporal value, memory entries, injected turns, and observation
world. Provider-specific rendering begins only after those layers are frozen.

### Provider-wire equality

The exact provider request bytes, message roles, tool schemas, tokenizer, and
media encoding may differ across providers. Choir should record the rendered
request digest for each trial and never claim byte or token equality where the
adapters differ.

A fair cross-model comparison therefore binds:

```text
shared semantic_context_digest
  + shared choir_rendered_context_digest
  + per-trial provider_request_digest
  + provider_adapter_version
```

Provider-specific accommodations should be declared variant dimensions, not
hidden prompt edits.

## Retrospective Evals

Historical cases can be evaluated in three distinct modes.

### Regrade only

Apply new graders to existing outputs, artifacts, and traces without making new
model calls. This is cheapest and safest, and is useful when a new deterministic
contract or rubric becomes available.

### Closed-world replay

Run new model or context variants inside a state-bound virtual world. This is a
strong causal comparison only to the extent that the challenger's observation
path remains inside the case's supported world.

Static response cassettes are insufficient for RLMs because a challenger may
write a different Go loop or choose a different read sequence. Closed-world
modules should therefore distinguish:

- **snapshot-backed pure reads:** files, artifact graphs, frozen sources, and
  other deterministic queries resolve against content-addressed case state;
- **recorded nondeterministic observations:** historical search, provider, or
  API results resolve through an explicitly frozen observation dataset;
- **capsule-local speculative writes:** allowed only inside trial scratch and
  evaluated as inert diffs/artifacts;
- **unmatched external observations:** refused without network access and
  recorded as outside support;
- **external or canonical writes:** always refused.

The closed-world broker must have no network transport. A fresh network call is
a structural trial failure, not a warning.

Every trial receives one comparability label:

- `comparable_closed_world`: all consequential observations resolved inside the
  shared support;
- `partial_closed_world`: an explicit unmatched set exists and only supported
  portions may be compared;
- `noncomparable`: the variant's strategy required a materially different
  observation world.

For the first implementation, the principal causal unit should be a first
provider call or bounded activation with a fully supported observation world,
not arbitrary free-running RLM orchestration.

### Refreshed-world rerun

Allow read-only connectors to retrieve current information under current
privacy and egress policy. This measures robustness in today's world, not
causal performance on the historical context. Results must be labeled with new
observation times and source digests.

No retrospective mode may repeat external writes, email delivery, agent
messages to live recipients, canonical mutation, acceptance, materialization,
checkpointing, or routing.

## Live Evals

For selected live subjects, Choir observes the ordinary live activation or
protocol outcome as the **champion-of-record** and creates isolated trials from
its frozen case. The live subject is not an `EvalTrial` and is not automatically
a same-context comparator for a whole activation: the live world can advance
and live actors can receive new inputs.

```text
live trigger
  -> freeze EvalCase
  -> ordinary live activation (champion-of-record; product authority)
  -> isolated baseline replay of champion config (eval only)
  -> isolated challenger A (eval only)
  -> isolated challenger B (eval only)
  -> eval reducer compares only frozen-case trials as paired evidence
```

The live configuration is selected by normal policy before any eval outputs
exist. It cannot be substituted post hoc by a shadow. Shadows and the baseline
replay receive an evaluation capability profile that mechanically lacks:

- live mailbox or channel delivery;
- work-item disposition;
- actor-memory or continuation mutation;
- external write connectors;
- canonical artifact mutation;
- promotable effect-bundle finalization;
- computer-event append, acceptance, materialization, checkpoint, or route;
- policy mutation.

For a single provider-call eval, the exact frozen request sent by the live
subject can be compared with trial requests. For activation- or workstream-level
eval, the baseline replay—not the evolving live activation—is the paired
counterfactual lane.

Shadow tool calls use one explicitly declared observation mode: frozen-only or
refreshed read-only. They never mix the modes under one comparison label.
Shadow artifacts are marked counterfactual and do not enter live actor context
unless a separate, explicit consensus architecture uses them. Once shadow
output influences the live decision, the operation is multi-model inference,
not an evaluation shadow.

Live eval sampling should be bounded by privacy class, cost budget, latency
budget, provider health, and work risk. High-volume shadows should normally run
asynchronously so eval latency does not become product latency.

### Multisupervision: production consensus with live eval evidence

Super should be a system-owned protocol, not a singleton actor. A live
supervision round freezes a decision case, eligible persistent actor seats,
independence policy, quorum, dissent handling, timeout behavior, and authority
contract before any supervisor output is visible.

```text
frozen live decision case
  -> supervisor seat A proposal --\
  -> supervisor seat B proposal ----> quorum/adjudication reducer
  -> supervisor seat C objection --/              |
                                                  v
                                     supervision decision receipt
                                                  |
                                     trusted commit/refusal/approval path
```

Each seat is an accountable actor activation with its own model/context
receipts. The protocol outcome—not any one seat—is the live subject. Because
the seats inspect a common decision case, their individual proposals and
disagreements also provide valuable live comparative evidence about model,
context, and supervisory policy.

Those production receipts can be regraded retrospectively and tracked as live
longitudinal eval evidence without adding shadow inference. They reveal which
seats caught defects, supplied decisive evidence, dissented correctly, or
failed together. Controlled counterfactual claims still require frozen trials.

That evidence does not make the participating seats eval trials. Once their
outputs can influence the real decision, they are production inference lanes.
Choir may additionally run isolated baseline or challenger trials over the same
frozen case; only those inert lanes are paired eval trials. This preserves the
line between observing a live consensus protocol and granting a shadow a vote.

Multiple live seats should be the normal Super configuration. Single-seat
operation is an explicit degraded or low-risk continuity mode and cannot
satisfy an irreversible-decision policy. High-consequence classes require a
stronger threshold; an irreversible decision requires all designated
independent seats to approve, no unresolved safety dissent, and any separately
required human approval. The protocol records:

- eligible and participating seats, identities, terms, and model-policy slots;
- independence domains such as provider, model family, prompt lineage,
  connector path, and execution host;
- proposals, objections, abstentions, failures, timeouts, and unresolved
  dissent;
- whether the quorum was satisfied without dynamically shrinking the panel;
- the exact obligation/evidence head seen by every seat;
- any human approval or external authority still required.

Multisupervision is defense in depth against a bad or compromised model, not a
proof of correctness. Correlated seats can share the same context defect,
provider failure, unsafe orchestration library, or host bug. The trusted reducer
validates the receipt and invariants; consensus itself never appends canonical
state, promotes a candidate, sends an external effect, or waives a required
human approval.

## One Execution Kernel, Injected Authority Surfaces

The eval system must execute the same context assembly, memory interpretation,
tool-loop, provider adapter, RLM interpreter, and typed module code as the live
path. A separate evaluation implementation would drift and violate Choir's
one-path doctrine.

The shared kernel should receive explicit host interfaces at construction:

```go
type ExecutionSurfaces struct {
    Observations ObservationWorld
    Artifacts    ArtifactSink
    Trace        TraceSink
    ActorBus     ActorBus
    Work         WorkReducer
    Computer     ComputerAuthority
    Connectors   ConnectorBroker
    Policy       ModelPolicyResolver
}
```

Live execution receives live scoped implementations. Eval execution receives:

- a frozen or refreshed read-only `ObservationWorld`;
- a counterfactual artifact sink;
- an eval-only trace sink;
- no live actor bus;
- no live work reducer;
- no computer authority;
- a connector broker whose mode is mechanically frozen-only or refreshed
  read-only;
- a trial-scoped model resolver that cannot write policy or escape admission.

This is stronger than a prompt, metadata flag, or registry denylist. Forbidden
interfaces are absent or fail closed before model-authored code runs. Trial mode
must also bypass live run-memory persistence, actor recovery snapshots, event
bus emission, channel delivery, and `RunAcceptance` synthesis.

The current `llm_policy_overlay_id` mechanism is live trajectory configuration
and may be inherited by spawned coagents. It must not be reused as the shadow
or counterfactual isolation mechanism. Trial variants are eval-scoped and
non-inheritable; current “eval/model arm” overlay semantics should be retired or
re-described when the eval substrate replaces them.

## Variant Matrix

The same case can vary:

- root actor model;
- any named model role inside an orchestration program;
- fallback and cycling order;
- consensus panel membership and independence policy;
- Super protocol membership, decision class, quorum, and adjudication policy;
- reasoning effort, token limit, and stopping rule;
- context selector, ordering, chunking, and compaction;
- artifact retrieval depth and source mix;
- system or role prompt program;
- interpreted RLM program or reviewed orchestration recipe;
- compiled Go combinator versions;
- module and capability profile;
- local model fan-out versus durable actor delegation;
- concurrency, capsule resources, deadline, and budget;
- supervisor escalation and review policy.

Paired one-factor comparisons are the clearest evidence. Factorial and adaptive
search can follow, but their multiple comparisons, selection bias, and stopping
rules must be recorded. An experiment should predeclare its primary metrics and
decision rule rather than selecting whichever metric flatters the challenger.
Where provider/model behavior is stochastic, the experiment must predeclare
repeat count or sequential stopping rules; one sample cannot establish a stable
ranking.

## Trial Identity and Isolation

Each trial should bind:

```go
type EvalTrial struct {
    TrialID            EvalTrialID
    ExperimentID       EvalExperimentID
    CaseRef            EvalCaseRef
    VariantRef         EvalVariantRef
    DeclaredModelRef   ModelRef
    ExecutedModelRefs  []ModelRef
    FallbackPathRef    ArtifactRef
    ExecutionHostRef   ExecutionHostRef
    CapsuleRef         CapsuleRef
    RealizationRef     *RealizationRef
    SemanticContext    Digest
    ProviderRequest    Digest
    ModelReceipts      []TraceRef
    ConnectorReceipts  []TraceRef
    OutcomeRef         ArtifactRef
    TraceRoot          TraceRef
    Usage              Usage
    Status             EvalTrialStatus
}
```

Trial identifiers are separate from live run, actor, message, work-item, and
event identities. A trial may reference the live subject but cannot reuse its
authority-bearing IDs.

Declared and executed variants are different facts. A provider precondition,
fallback, cycle, retry, or upstream alias may change the provider/model that
actually ran. Controlled experiments should normally pin or disable fallback;
otherwise the executed route becomes an explicit variant dimension. A trial
must never be scored as the declared model when receipts show another model ran.

Capsules provide process, filesystem, and resource isolation. The eval broker
provides authority isolation. Running a trial on another VM or host expands
capacity, not semantic authority.

An eval case references the source `ComputerID`; it does not mint a substitute
or `eval` ComputerID. All eval handles use a disjoint, non-authority-bearing
namespace and are rejected by live run, actor, work, message, event,
effect-bundle, acceptance, checkpoint, and route APIs.

### Storage and observation namespace

Cases, variants, trials, grades, and findings should be content-addressed,
owner/computer-scoped artifacts and provenance objects using Choir's existing
blob and object-graph ledgers. They do not create another semantic event ledger.
Their kinds and identity/retention rules must be registered explicitly before
use, for example:

```text
choir.eval_case
choir.eval_variant
choir.eval_trial
choir.eval_grade
choir.eval_finding
```

Trial traces must not appear in live run or trajectory Trace queries. The
implementation may use an eval-only trace store or an explicit experiment/trial
namespace with mandatory query separation, but it cannot overload live
`run_id`, `trajectory_id`, or actor identity. Trace remains observation, never
case or finding authority.

Privacy deletion may tombstone a case and make future replay unavailable while
retaining only the minimum deletion and aggregate-accounting receipt. Frozen
does not mean exempt from deletion policy.

When an exact computer environment is needed, the scheduler may construct a
read-only realization at the case's accepted effective head. It must never
restore arbitrary running state or use a pending desired head as though it were
effective state.

## Multi-Capsule and Multi-Host Scheduling

Choir's growing inference capacity can be exposed through an eval scheduler:

```text
experiment planner
  -> case x variant matrix
  -> privacy/capability/model admission
  -> trial shards
  -> capsules across eligible execution hosts
  -> immutable results and health receipts
  -> eval reducer
```

Scheduling policy should account for:

- provider quotas and correlated rate limits;
- host and capsule CPU/memory pressure;
- model concurrency limits;
- trial priority and live-product load;
- data residency and provider egress constraints;
- deterministic case/variant sharding;
- cancellation and partial-panel semantics;
- retry identity and duplicate suppression;
- cost ceilings at trial, experiment, computer, and platform scopes.

The scheduler must not treat the user computer as disposable worker identity.
Evaluation compute is a realization/capsule service over frozen inputs.

## Scoring and Evidence Classes

No single scalar score should collapse all quality dimensions. Trial evidence
should preserve a vector of measurements and the exact grader version.

### Deterministic contract graders

- schema validity;
- capability and authority refusals;
- citation/source identity;
- required artifact presence;
- idempotency and replay contracts;
- test, build, lint, and executable verifier results;
- capsule-local diff shape and effect classification, without creating a
  promotable bundle;
- work-item and settlement invariants.

### Semantic graders

- factual support and evidence coverage;
- contradiction and uncertainty handling;
- instruction and role adherence;
- decision quality;
- decomposition and synthesis quality;
- context selection relevance;
- calibrated confidence.

Semantic grading can use multiple model families, pairwise comparisons, blinded
ordering, and explicit rubrics. Judge outputs remain evidence and should be
calibrated against deterministic checks and human signals.

### Operational graders

- latency and tail latency;
- tokens, calls, and monetary cost;
- fallback and retry rate;
- context bytes/tokens consumed;
- context virtualization ratio: available artifact bytes versus root-model
  context consumed;
- RLM Go cell count, evaluation/compile latency, recursive depth, and failed
  cells;
- goroutine/channel leak or cancellation failures;
- snapshot-supported versus unmatched operations;
- capsule and host resource use;
- delegation fan-out and message volume;
- provider and connector failure recovery.

### Downstream outcome graders

- owner correction or rejection of the live subject;
- later contradiction by stronger evidence;
- obligation reopening;
- accepted artifact quality;
- successful tests or deployed behavior where applicable;
- time and cost to trajectory settlement;
- rollback or remediation incidence.

Downstream outcomes are valuable but confounded by later actors and world
changes. They should complement, not overwrite, paired case comparisons.
Owner correction is evidence about the output the owner actually saw; it is not
a discriminator for an unexposed challenger. Comparing human response to
challengers requires a separately disclosed and consented randomized exposure
design.

## Judge Safety and Eval Integrity

LLM-as-judge is useful but structurally limited. Choir should:

- use deterministic contracts before semantic judges;
- blind model/provider identity where possible;
- randomize pairwise presentation order;
- use more than one judge family for consequential comparisons;
- record judge model, context, rubric, and adapter versions;
- test judges against human-labeled calibration sets;
- retain dissent rather than reducing every panel to majority vote;
- separate proposal generation from grading where contamination matters;
- detect verbosity, style, and self-preference biases;
- keep sealed holdout and canary cases out of optimization prompts and agent
  memory;
- record missing, failed, and timed-out graders rather than silently shrinking
  the panel.

An eval system can be gamed by the agents it evaluates if exact graders and
cases are repeatedly visible. Case access should follow the minimum disclosure
needed for execution; hidden contracts must still be legitimate product
requirements, not traps unrelated to user value.

Every semantic judge is itself an eval subject and a new data recipient. Judge
provider/model, rubric, adapter, context, retention, and failures are recorded;
judge admission uses the same or stricter privacy gate as the evaluated trial.
No judge panel may silently add a provider that was inadmissible for the case.

## Privacy and Data Governance

Retrospective evaluation creates a new use of historical data. It must not
silently widen disclosure.

- Case creation retains the source computer's ownership and privacy scope.
- A model variant is admissible only if current computer policy permits that
  provider to receive the case's data classes.
- “The original model saw it” does not authorize sending it to a new provider.
- Private prompts, messages, sources, and artifacts remain encrypted or
  appropriately scoped at rest.
- Eval artifacts inherit or strengthen the strictest input retention class.
- Judge providers, storage replicas, telemetry sinks, and execution hosts are
  separately admitted data recipients; subject-model admission does not cover
  them implicitly.
- Cross-computer aggregate learning requires a separately defined privacy-safe
  aggregation protocol; raw cases do not become platform training data by
  implication.
- Deletion and retention policy must cover cases, rendered provider requests,
  outputs, judge artifacts, and aggregate statistics.
- Live shadowing must be visible in system privacy disclosures and operational
  accounting even when it is not a user-facing control.

## From Findings to Model Policy

An eval finding can recommend a model or context-policy change, but cannot
activate it directly.

Counterfactual findings cannot prove that live behavior occurred and may not
serve as acceptance, materialization, checkpoint, route, or promotion evidence.
They may inform which live subject to build or verify; the resulting subject
still requires its normal evidence and acceptance path.

```text
eval evidence
  -> policy proposal
  -> system-owned validation and review
  -> typed per-computer or platform-policy transaction
  -> later activation uses new effective policy
```

Policy decisions should name their evidence domain. A model that wins research
extraction cases should not automatically replace a model used for effect
verification or owner-facing Texture work.

Over time, Choir may learn a contextual router:

```text
work/context features
  -> permitted portfolio prediction
  -> live subject plus sampled isolated trials
  -> outcome evidence
  -> router evaluation
```

The router remains bounded by the host registry, per-computer policy, privacy,
modality, budget, capability, and risk class. Exploration is explicit and
audited. A temporary provider outage should not be mistaken for a durable model
quality result.

## Connection to Choir as It Exists

| Existing surface | Eval use | Gap or caution |
| --- | --- | --- |
| Trajectories and work items | Identify workstream state and downstream settlement | Need frozen workstream-head and causally closed case projection |
| `RunRecord` and model metadata | Seed subject/model identity | Run completion is not work completion; metadata does not capture a complete context |
| Trace event store | Observation and causal receipt source | Trial traces require a separate namespace/store and must never pollute live run/trajectory queries |
| Evidence and source records | Grounding inputs and grader evidence | Need immutable case membership and privacy propagation |
| `internal/modelpolicy` | Baseline selections, fallbacks, and policy refs | Resolves mainly by role today; live trajectory overlays and inherited “eval arms” are not safe counterfactual isolation |
| Tool registries/capability profiles | Freeze executable affordances | Need a read-only eval profile and versioned module manifest |
| Actor wake plus run memory and compaction | Historical harness/context input | Case must bind wake envelope, cursor, entries, frozen time, and model-dependent compaction decisions rather than rely on opaque reconstruction |
| CoSuper assignments and capsules | First RLM activation eval target | Effects remain capsule-local; shadow finalization and live messaging must refuse |
| Provider/tool loop | Shared execution kernel and exact request/usage sequence | Must accept injected inert authority surfaces; a separate eval tool loop would become a drifting dual path |
| RunAcceptance | Possible downstream evidence reference | Historical evidence projection only; must not become eval truth or policy authority |
| Computer event and ComputerVersion reconstruction | Bind accepted world state for faithful trials | Eval realization cannot append, accept, checkpoint, or route |

There is no general eval object or scheduler in current Choir. The first work is
not a dashboard or a judge prompt. It is an exact capture boundary for context,
configuration, observations, and authority.

## Adoption Sequence

### Phase 1: capture without counterfactual execution

- Define layered `SemanticContextManifest`, `EvalCase`, storage/retention kinds,
  and immutable model-call receipt schemas.
- Capture one non-private CoSuper or Research provider-call boundary: actor wake,
  run memory, temporal value, system/skill/tool text, rendered provider request,
  module/capability manifests, observations, output, usage, and trace root.
- Prove privacy inheritance, content identity, and reconstruction of the
  semantic context presented to the provider adapter.
- Add deterministic regrading over existing outputs.

### Phase 2: bounded retrospective closed-world trials

- Share the exact production context/tool-loop kernel with injected inert eval
  surfaces.
- Run a baseline replay and one permitted alternate model against the same
  first-call or fully supported bounded case.
- Resolve pure reads against frozen state and nondeterministic reads against the
  observation dataset.
- Prove trial IDs cannot collide with live authority IDs.
- Compare typed outcomes, evidence graphs, cost, latency, and declared versus
  executed model route using deterministic graders.
- Hard-refuse network access and label unmatched operations with explicit
  comparability status.

### Phase 3: live shadow trials

- Sample low-risk activations with one ordinary live subject, an isolated
  baseline replay, and bounded challengers.
- Give shadows a mechanically empty external/canonical effect set.
- Prove shadow output cannot enter actor memory, work disposition, messages,
  policy, effect bundles, or the canonical event chain.
- Run trials asynchronously where product latency would otherwise change.

### Phase 4: workstream and organization evals

- Evaluate consensus architectures, delegation graphs, multisupervision
  protocols, supervisory shifts, and context selection across causally closed
  trajectory segments.
- Compare seat diversity and independence policies, quorum rules, dissent
  handling, and single-seat versus multi-seat supervision without conflating
  production votes with shadow trials.
- Join trace, obligation, evidence, cost, and downstream outcome measurements.
- Calibrate semantic judges and publish vector results with dissent.

### Phase 5: policy learning

- Generate typed model/context-policy proposals from repeated, scoped evidence.
- Use sampled shadows and holdouts to detect regression and distribution shift.
- Keep activation of policy changes in the normal audited configuration path.
- Add privacy-safe aggregate learning only through a separately reviewed
  protocol.

## First Acceptance Proof

The first meaningful proof should remain a model-call eval, not attempt the
whole architecture. Capture one real, non-private CoSuper or Research first-call
case and execute exactly two frozen trials:

1. baseline replay with the current model configuration;
2. one permitted alternate model with the same canonical and Choir-rendered
   context.

The proof must show:

- one immutable case with actor-wake, memory-cursor, frozen temporal context,
  canonical-context, and Choir-rendered-context digests;
- distinct rendered provider-request digests and adapter refs;
- exact model, policy, capability, prompt, and module identities;
- a first-call or fully supported frozen observation world;
- typed outcomes and complete usage receipts;
- declared and actually executed model/fallback identity;
- deterministic schema, authority, privacy, and no-effect grades; no model judge
  is required for this proof;
- no external writes, live messages, work disposition, actor memory mutation,
  live Trace/event rows, effect finalization, `RunAcceptance`, computer event,
  checkpoint, or route change;
- replay/idempotency under equal trial identity and conflict on changed variant;
- explicit treatment of failed or timed-out trials;
- a vector finding that does not automatically change model policy.

A second proof may add one declared context-policy change under the same model,
producing a deliberately different semantic-context digest. A later RLM proof
should kill an activation and use a frozen bounded case on another eligible
execution host; multi-host scheduling is not part of the first eval proof.

## Decisions Still Required

- What exact shared production function returns the canonical context,
  Choir-rendered context, and provider-adapter input without introducing an eval
  assembly path?
- Which modules can be virtualized against frozen state, which require recorded
  observation datasets, and what unmatched threshold makes a trial
  noncomparable?
- How much RLM Go source and interpreter observation is retained for privacy,
  audit, and debugging?
- Are cases immutable forever, or can privacy deletion produce a durable
  tombstone that makes later replay unavailable?
- What makes a trajectory segment causally closed enough for workstream eval?
- Which metrics are primary for each actor/capability profile?
- How are judge calibration sets created without leaking private user data?
- How are provider-specific modality differences represented in a fair semantic
  case?
- When is synchronous live shadowing justified by the value of the comparison?
- How are experiment sample size, early stopping, and multiple comparisons
  encoded?
- What evidence threshold can propose a per-computer versus platform-default
  policy change?
- How are policy regressions detected after workload or provider behavior
  shifts?
- Which aggregate eval statistics may cross computer or cloud boundaries?

## Invariants

1. Eval trials never have live product, external-effect, or promotion authority.
2. The live subject is selected by normal policy before eval outputs exist and
   is never itself an eval trial; paired comparison uses an isolated baseline
   replay.
3. A shadow that influences a live decision is part of an explicit multi-model
   inference architecture, not an eval shadow.
4. Canonical context, Choir-rendered context, and provider-wire bytes are
   different claims and are recorded separately.
5. Closed-world replay performs no fresh network observation.
6. Refreshed-world results never masquerade as causal historical comparisons.
7. Trial identities cannot be used as run, actor, work-item, message, event,
   capsule-effect, acceptance, checkpoint, or route identities.
8. Eval realizations reconstruct only accepted effective state.
9. Trace, RunAcceptance, judge votes, and eval findings remain evidence rather
   than canonical event or model-policy authority.
10. Eval findings never substitute for live acceptance, verification,
    materialization, checkpoint, route, or promotion evidence.
11. Evaluation never widens disclosure beyond effective privacy and computer
    policy; subject models, judges, storage, telemetry, and execution hosts are
    each admitted.
12. Grader identity, version, inputs, failures, and dissent are durable parts of
    the finding.
13. Declared and executed model routes are both recorded; fallback cannot
    silently change the tested variant.
14. Policy activation is a separate typed transaction.
15. Missing and timed-out trials remain visible; a panel does not silently
    redefine itself.
16. Eval artifacts inherit the strictest relevant input retention class.
17. Trial traces and identities never enter live run, trajectory, actor, or
    recovery namespaces.
18. Scaling execution capacity cannot create a second computer, work, or
    promotion authority.
19. Super is evaluated as a protocol outcome with attributable seat receipts,
    not as a privileged singleton model response.
20. A supervisor lane that can influence the live decision is production
    inference, not an eval shadow.
21. Required supervision quorum is selected before outputs exist and cannot be
    weakened by silently dropping failures, abstentions, or dissent.

## Agentic Consensus Review

The first complete draft was reviewed on 2026-08-09 by the repository's default
agentic-consensus panel in convergent mode. The reviewed candidate SHA-256 was
`2b83fe3a9f2ee086a128d932e3eb46b6774489c707e0ec7051e2121ebca0e48c`.
The exact prompt digest was
`04809231158e618495cc89bd7c6eaa4d805d6c0ce183e8ca62801bd6857c65f9` and
the manifest digest was
`262e76cac9b66fdac10db3bdbd3687ef9b83be4e392eabae58a58ce96d05a7cf`.

### Panel health

| Panelist | Status |
| --- | --- |
| Devin configured default | succeeded |
| Cursor configured default | succeeded |
| OpenCode configured default | succeeded |
| OMP Gemini 3.6 Flash, high thinking | succeeded |
| OMP DeepSeek V4 Flash Free, high thinking | succeeded |
| OMP Cursor Grok 4.5 High | failed: provider API key unavailable |
| Codex configured default | timed out at 300 seconds |
| OMP GPT-5.6 Sol, medium thinking | timed out at 300 seconds |

Raw session diagnostics remain in the gitignored local directory
`.agentic-consensus/agentic-consensus-20260809-010818/`; the durable
adjudication is recorded here.

### Consensus

All five successful reviewers returned **revise**, while accepting the central
claim that evaluation should use frozen counterfactual cases with no live
authority. Their must-fix findings converged on seven boundaries:

1. The live champion-of-record is not an eval trial; fair paired comparison uses
   an isolated replay of its configuration.
2. Context identity must be layered and bind actor wake, memory cursor, frozen
   time, skill/tool text, injected messages, compaction decisions, and exact
   adapter output.
3. Dynamic RLM plans require state-bound virtualization, not only historical
   call-response lookup.
4. Eval and live execution must share one production kernel with injected
   authority/effect surfaces; a separate eval harness would drift.
5. Trial identities and traces must be isolated from live actor, workstream,
   recovery, and Trace namespaces.
6. Privacy admission applies independently to subject variants, judges,
   storage/telemetry, and execution hosts.
7. Declared model configuration and the actually executed route may differ
   through fallback or provider drift and must be recorded separately.

These claims were locally checked against the current runtime: system context
injects temporal and skill text; the tool catalog is constructed in the shared
tool loop; run-memory compaction thresholds depend on model context window;
owner/coagent turns are injected around activation; current model-policy
overlays can inherit across a trajectory; Trace queries use live run and
trajectory identity; and actor wake/recovery is bound to update and memory
cursors.

### Dissent and adjudication

Reviewers differed on how narrow the first proof should be. Suggestions ranged
from capture-and-regrade only to three configurations plus a live shadow. This
memo adopts the smaller common denominator: one non-private first-call case,
one baseline replay, one permitted alternate model, deterministic graders, and
mechanical proof of zero live writes. Context-policy variants, semantic judges,
live shadows, RLM-wide replay, realizations, and multi-host scheduling follow
later.

One review suggested assigning a distinct eval `ComputerID`; that was rejected
because it would violate the persistent-computer ontology. Eval objects instead
reference the source computer and use a disjoint non-authority namespace.

Another suggestion was to reuse current model-policy overlays for trial arms.
That was rejected for counterfactual isolation because overlays are live
trajectory configuration and may be inherited by spawned agents. The existing
overlay mechanism remains useful live policy substrate, but not an eval autoputer.

The panel's strongest lateral contribution was to treat closed-world replay as
program virtualization: arbitrary pure reads resolve against frozen state,
nondeterministic observations resolve against a frozen dataset, speculative
writes remain capsule-local, and unsupported external observations produce an
explicit comparability failure. This is now the memo's target rather than rigid
tool-response cassette replay.

### Material changes from review

- Replaced champion trial semantics with live subject plus isolated baseline
  replay.
- Added layered context identity and exact actor wake/memory bindings.
- Added one shared execution kernel with mechanically inert eval interfaces.
- Added comparability classes for dynamic closed-world execution.
- Added eval artifact/trace storage and namespace rules.
- Extended privacy admission to judges and infrastructure recipients.
- Added declared-versus-executed model/fallback identity.
- Narrowed the first proof and deferred live shadows, judges, and multi-host
  scheduling.

### Post-review extension: Super as a protocol

After the recorded panel run, the architecture was extended to remove the
remaining singleton-Super assumption. The extension treats multisupervision as
a production consensus protocol whose seat-level receipts also create live eval
evidence. It does not claim that the earlier panel reviewed this addition.

The extension preserves the panel's core boundary: any supervisor output that
can influence the live result is part of production inference, while baseline
and challenger trials remain isolated. Stronger predeclared quorum for
irreversible decisions adds defense in depth but does not turn consensus into
canonical commit, promotion, or human-approval authority.

## Current Non-Claims

This memo does not claim that:

- Choir currently has a general eval scheduler, case store, or shadow system;
- current Trace events reconstruct exact model context;
- provider outputs are deterministic under identical inputs;
- historical tool observations can answer arbitrary challenger tool plans;
- LLM judges provide ground truth;
- RunAcceptance is an eval score;
- a shadow result can be substituted for the live subject after outputs exist;
- multi-VM execution implies multiple candidate computers;
- an eval winner may directly change model policy;
- cross-computer private data may be pooled for platform learning.

## Relationship to Current Doctrine

This memo proposes an evaluation layer beneath current authority boundaries. It
inherits [Choir Doctrine](choir-doctrine.md), [Choir Agent Product
Doctrine](agent-product-doctrine.md), [Choir Computer
Ontology](computer-ontology.md), and the companion [Persistent RLM Actors
memo](memo-persistent-rlm-actors-2026-08-09.md).

If adopted, durable eval claims should be promoted through a separately reviewed
architecture change. This memo must not silently turn Trace, model policy,
RunAcceptance, or execution capacity into new semantic authorities.
