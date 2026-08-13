# Memo: Persistent RLM Actors and Capsule-Isolated Go Orchestration

**Status:** architecture synthesis; kernel and authority claims promoted to
Choir Doctrine and Agent Product Doctrine on 2026-08-12; implementation detail
remains proposal material
**Mutation class:** green (documentation only)
**Scope:** agent execution, multi-model orchestration, durable communication,
connector access, and integration with the audited computer

## Thesis

Choir should keep agents as persistent actors inside one audited user computer,
while moving model-authored computation into disposable, capability-bound
execution capsules containing a Yaegi Go REPL.

```text
Durable actor loop
  wait -> wake -> run RLM activation -> park/complete -> wait

RLM activation loop
  model -> Go cell -> observations -> model -> Go cell -> typed outcome
```

The actor persists. The activation does not. Capsules contain executions, not
agents.

This framing coheres several existing Choir ideas:

- the durable actor mailbox is the wake, park, restart, and recovery substrate;
- the lifecycle trajectory and work-item graph holds durable responsibility;
- the capsule is the effect and resource isolation boundary;
- the model is a programmable reasoner that authors orchestration code;
- typed Go modules are the capability surface;
- the host model broker owns multi-model policy and provider access;
- typed messages make supervision observable, while durable trajectory
  obligations and evidence remain the authoritative supervision state;
- the canonical computer event chain remains the only semantic state authority;
- frozen effect bundles, external acceptance, materialization, checkpointing,
  routing, and rollback retain their existing authority separation.

The proposal is therefore not a second multiagent system. It is a new execution
substrate for actors in the existing multiagent system.

The promoted doctrine sharpens the model-facing contract. A restricted actor
activation has one actuator: evaluate a Go cell. Search, fetch, transforms,
artifacts, assignments, messaging, evidence, and outcomes are imported Go
modules rather than parallel ambient tools. An effects-capable implementation
assignment may additionally receive direct Bash, backed by the same capsule
execution broker and receipts as Go execution calls. This is assignment
authority, never privilege inherited from the `CoSuper` name.

The complete architectural unit is not the actor or activation alone:

```text
persistent supervised trajectory/workstream
  + addressable accountable actors
  + disposable capability-scoped RLM activations
```

The actor owns identity, relationship, and accountability. The trajectory owns
settlement-critical continuity: work items, questions, evidence, budgets,
authority decisions, and artifact state. The activation owns temporary
computation. No one of these three may silently absorb the authority of the
others.

## Why Go and Why an RLM

Python REPLs naturally suggest calculation. An interpreted Go environment
naturally suggests orchestration: typed functions, explicit errors, contexts,
goroutines, channels, structured dataflow, and service APIs.

A Choir actor frequently needs to do more than fit one task into one model
context. It may need to:

- partition and inspect a long corpus without inserting all of it into one
  prompt;
- call different models for extraction, criticism, planning, synthesis, or
  verification;
- run independent inference calls concurrently;
- cycle or fall back among models according to system policy;
- call search, source, email, code-execution, or other connector modules;
- delegate durable responsibilities to other actors;
- wait for those actors and resume after process restart;
- assemble evidence, commands, tests, diffs, and artifacts into one typed
  outcome.

An RLM makes orchestration itself model-authored code. Go variables and artifact
references become working memory; model calls become bounded reasoning
operations; durable actors carry responsibility across activations.

## One System, Two Loops

### Durable actor loop

The harness owns identity and time:

```text
mailbox update
  -> wake persistent actor
  -> reconstruct durable context
  -> provision activation capsule and scoped capabilities
  -> run RLM activation
  -> validate typed outcome
  -> commit, park, or complete
  -> passivate actor
```

The durable actor retains:

- actor identity and organizational relationship;
- optional compact memory and a pointer into durable workstream state;
- mailbox cursor and work-item bindings;
- lifecycle, cancellation, and settlement state;
- supervisory and channel memberships;
- restart continuity.

Actor-local memory must not be the sole location of anything needed for
settlement, reassignment, supervision, or rewarm. Those facts belong to the
trajectory/work-item and artifact state. The actor is a durable address and
accountable organizational principal; the workstream is the durable unit of
problem continuity.

### RLM activation loop

The capsule owns temporary computation:

```text
model reasons about current observations
  -> emits a Go cell
  -> Yaegi evaluates it
  -> typed module calls produce observations and artifacts
  -> model reasons again
  -> activation returns a typed outcome
```

The activation may retain ordinary Go state while it is running, including
goroutines and channels. It does not durably retain:

- the Yaegi heap or interpreter stack;
- goroutines or channels;
- half-completed model or connector calls;
- scratch filesystem state that has not been frozen into an artifact or effect
  bundle;
- an activation capability issued against an earlier computer or work head.

Long work parks by emitting a typed continuation containing artifact refs,
completed step receipts, pending assignments, open questions, and remaining
budget. A new activation reconstructs from that durable continuation rather
than restoring an interpreter process.

## Agent, Role, Profile, and Module

These concepts should be kept separate:

| Concept | Meaning |
| --- | --- |
| Actor | Durable identity, address, organizational relationship, accountability, and lifecycle |
| Trajectory/workstream | Durable problem continuity, obligations, evidence, budgets, questions, and settlement |
| Organizational role | Relationship to other actors and authoritative objects |
| Capability profile | The typed modules and operation classes available for an assignment |
| Model policy | System-owned rules for selecting, cycling, and falling back among models |
| RLM activation | Disposable model-authored Go computation for one actor wake |

The common harness does not fork by persona. A Researcher, CoSuper, Texture, or
actor occupying a Super protocol seat uses the same RLM machinery. Differences
are expressed through organizational bindings, prompts, imported modules,
host-validated activation capabilities, and typed outcomes.

Importing a module is not itself authority. A call succeeds only when the
activation also holds matching unforgeable capabilities and the broker validates
current actor, computer, trajectory, work, operation, budget, and state heads.
This makes the same Go program harmless when evaluated under a weaker
assignment. Persisted source never preserves authority.

Some names remain semantically meaningful even on a uniform substrate:

- **Super** is a system-owned supervision protocol for a computer or mission,
  fulfilled by one or more independently accountable actor seats.
- **CoSuper** is a delegated supervisor responsible for a bounded workstream.
- **Texture** owns the evolving canonical document and artifact-control-plane
  relationship.
- **Email** owns a persistent external conversation and approval lifecycle.
- **Research** can increasingly be a capability profile and playbook rather
  than a distinct execution architecture.

## Super Is a Protocol, Not a Singleton

No actor should acquire ambient authority merely by being named `Super`.
Supervision is a durable protocol instance over an explicit scope, membership,
decision policy, and evidence state. Persistent actors occupy versioned seats
in that protocol; they do not become the protocol or its authority.

```text
supervision scope + decision class
  -> freeze eligible seats and independence policy
  -> concurrent or staged supervisor activations
  -> typed proposals, objections, abstentions, and evidence
  -> quorum/adjudication reducer
  -> supervision decision receipt
  -> trusted authority reducer may commit, refuse, or request human approval
```

Multiple active seats are the normal mode. A one-seat round is an explicit
degraded or low-risk continuity mode, not the definition of Super, and cannot
satisfy an irreversible-decision policy. High-consequence decisions require a
stronger threshold. An irreversible decision requires all designated
independent seats to approve, no unresolved safety dissent, and any separately
required human approval. The policy is selected before outputs are visible.
Failures and abstentions remain visible and cannot silently shrink the quorum.
A coordinator may schedule calls and synthesize evidence, but its synthesis is
another typed proposal, not a privileged final answer.

The protocol should bind:

- supervision scope and decision class;
- eligible seats, actor identities, and terms;
- model-policy slots and independence domains;
- quorum, dissent, timeout, escalation, and replacement rules;
- the frozen obligation/evidence head considered by the round;
- each proposal's context, model, trace, and evidence refs;
- the adjudicated decision and any unresolved dissent;
- the authority or human-approval contract required after consensus.

A possible host-owned shape is:

```go
type SupervisionRound struct {
    RoundID          SupervisionRoundID
    ScopeRef         ScopeRef
    DecisionClass    DecisionClass
    CaseRef          ArtifactRef
    EligibleSeats    []SupervisionSeat
    IndependenceRef  PolicyRef
    QuorumRef        PolicyRef
    ProposalRefs     []ArtifactRef
    DecisionRef      *ArtifactRef
    Status           SupervisionRoundStatus
}
```

The interpreted actors author proposals and objections. Trusted Go code freezes
membership, checks receipts and independence, reduces quorum, and decides which
authority API—if any—may receive the resulting decision receipt.

Multisupervision reduces single-model capture, hallucination, and silent
compromise, but only if independence is real. Two seats using the same provider,
prompt lineage, context defect, or compromised connector are correlated, not
two independent security principals. The host validates the independence claim
and retains commitment authority.

## Why CoSuper Now Has the Right Name

CoSuper is not primarily a smaller coding worker beneath Super. It is a
co-supervisor of a bounded problem. It owns the result while delegating much of
the computation.

A CoSuper is encouraged to orchestrate rather than personally consume every
source or perform every operation in its foreground context. It may:

- map extraction over thousands of artifact chunks;
- ask different models to plan, criticize, implement, and verify;
- find disagreements and launch focused follow-up calls;
- use APIs and tools through typed connectors;
- open durable assignments for other CoSupers or research-capable actors;
- supervise a shared channel;
- synthesize the resulting evidence graph into a compact recommendation or
  effect proposal.

This is context virtualization. Bulk context remains in immutable artifacts and
source records. The CoSuper model receives bounded views, unresolved questions,
and evidence summaries. Go code carries the orchestration structure.

Delegation does not transfer accountability. The CoSuper must disposition
conflicts, identify gaps, and return a defensible typed outcome rather than a
stack of unexamined summaries.

## Multi-Model Execution

The platform owns a host-level model registry. Each user computer owns durable
model policy within the choices the platform makes available. Model selection,
fallback, cycling, consensus architecture, and role following remain
system-owned processes rather than user-edited configuration.

Yaegi receives typed model clients such as:

```go
models.Call(ctx, request)
models.Parallel(ctx, requests...)
models.Map(ctx, inputs, task)
models.Pipeline(ctx, stages...)
models.Reduce(ctx, results, reducer)
consensus.Run(ctx, architecture, question)
```

The RLM program may request a role, modality, cost/latency class, or named
system policy slot. It does not receive provider credentials and cannot select
a model outside the computer's effective policy.

Every call crosses the trusted model broker, which resolves the request and
records:

- activation, actor, trajectory, and work-item identity;
- requested role and constraints;
- resolved provider, model, reasoning level, and policy version;
- fallback, cycling, retry, or consensus position;
- prompt/input artifact digest and output artifact ref;
- causal parent and dataflow edges;
- tokens, cost, timing, cancellation, and error state;
- the current computer head and capability-policy digest.

These receipts form an RLM trace graph. They do not each become canonical
computer events. A typed outcome references the trace root when the computation
becomes relevant to durable state or an effect proposal.

## Concurrency

Choir needs two different concurrency models.

### Activation-local concurrency

Within one RLM activation, real Go goroutines and channels may coordinate model
and connector calls:

```go
architect := make(chan models.Result, 1)
critic := make(chan models.Result, 1)

go func() { architect <- models.MustCall(ctx, architectRequest) }()
go func() { critic <- models.MustCall(ctx, criticRequest) }()

combined := consensus.Compose(<-architect, <-critic)
answer := models.MustCall(ctx, synthesisRequest(combined))
```

Models do not need their own mailbox privilege. Go code passes one model's
output into another model's input. This is ephemeral dataflow, not actor
communication.

Common combinators such as `models.Parallel` should execute their internal
concurrency in compiled host libraries where budgets, deterministic result
ordering, cancellation, and audit are easiest to enforce. Model-authored code
may still use interpreted goroutines and channels for novel workflows.

All calls remain bounded by activation context, fan-out limits, semaphores,
budgets, and capsule resource controls. Destroying the capsule terminates leaked
or deadlocked activation-local computation.

An effects-capable implementation activation may also receive direct Bash for a
one-off command before deciding its next move. Direct Bash and
`execution.Run`/`execution.Shell` from Go are ergonomic views of the same
capsule execution broker, transaction tape, process-tree cancellation, and
receipt schema. Subprocesses receive no Choir capability, broker/provider
credential, canonical-state access, or control socket. Restricted profiles
such as Researcher have no Bash or equivalent general execution module.

### Durable multiagent concurrency

When independent identities must own work, remember it, communicate progress,
or survive restart, the harness uses actors, work items, and mailboxes. It does
not keep an RLM goroutine alive while waiting.

```text
model.Result over a Go channel = local computation
coagents.Message               = durable organizational communication
```

## Typed Obligations and Messages as the Supervision System

Agent-to-agent communication is Choir's richest continuous supervision and
observability vector, but the transcript is not authoritative supervision
state. A useful three-plane split is:

1. **Transport plane:** authenticated messages, mailboxes, wakeups, and channel
   projections.
2. **Supervision plane:** trajectory work items, assignments, questions,
   blockers, evidence, budgets, dispositions, and artifact heads.
3. **Authority plane:** canonical computer events and externally accepted
   effect transactions.

A message is an authenticated input to a reducer. If its assignment, blocker,
question, disagreement, evidence result, capability request, or disposition
matters for settlement or rewarm, the reducer must materialize corresponding
durable obligation state. Reading a transcript helps a supervisory seat
understand a workstream, but settlement queries the workstream and evidence
graph.

All durable agent communication should pass through a host-owned typed
obligation/message operation. Arbitrary network sockets, shared mutable
directories, and direct capsule references must not become alternate agent
communication systems.

A typed envelope should carry at least:

```go
type Message[T any] struct {
    ID            MessageID
    Type          MessageType
    SchemaVersion string
    Sender        ActorID
    Recipient     ActorID
    WorkItem      WorkItemID
    Correlation   CorrelationID
    ReplyTo       *MessageID
    Intent        Intent
    Payload       T
    ArtifactRefs  []ArtifactRef
    TraceRoot     TraceRef
    AuthorityRef  AuthorityRef
    Deadline      time.Time
}
```

Free text communicates information, not authority. Receiving a message never
grants a capability. An instruction in message content remains a request until
trusted host policy validates and durably materializes an assignment,
capability, obligation, or transaction.

The current `coagent_source_packet.v1` and `update_coagent` path already points
in this direction: the model supplies a typed payload, while runtime-loaded
owner, computer, target, trajectory, work, and call identity determine delivery
authority; wake occurs after the durable write. The target should generalize
that one-write rule, not invent a second semantic message bus.

### Shared supervised channels

A Super protocol round may propose that the host reducer create a durable
workgroup projection for Researchers and CoSupers. The reducer validates the
protocol receipt and scoped authority over the work tree, writes membership and
obligation state, and creates participant and supervisor subscriptions. Channel
log, delivery, wake, and supervision views are causal projections of the same
typed write.

```text
Researcher A --\
Researcher B ----> immutable channel log ---> participant mailboxes
CoSuper A -----/             |
                              +--> supervisory seat A
                              +--> supervisory seat B
```

A supervisory seat can observe a channel without being sender or recipient.
When a seat actor is also a participant, its participant and supervisor
relationships still observe the same canonical envelope and transcript.

Useful explicit supervision modes include:

- **audit:** transcript inspection without wakeups;
- **observe:** selected message types wake assigned supervisory seats;
- **review-effects:** information flows, but capability and effect requests are
  held for disposition;
- **mediated:** every message requires approval, reserved for exceptional work;
- **escalation-only:** disagreements, stalls, budget exceptions, or authority
  requests open a supervision round.

The normal policy should be observe plus review-effects. Requiring synchronous
multi-seat approval for all information exchange would turn supervision into a
bottleneck and a liveness dependency. These modes ultimately govern which
obligation and effect classes may advance without review, not merely which
messages are interesting enough to open a supervision round.

## Connectors and Network Access

Network access is a capability substrate, not a general-purpose network agent.
Yaegi should not receive unrestricted `net/http`. It receives narrow typed
modules:

```go
sources.Search(ctx, query)
sources.Fetch(ctx, sourceID)
github.ReadIssue(ctx, repo, number)
github.ProposeComment(ctx, issue, body)
email.Search(ctx, query)
email.Read(ctx, messageID)
email.ProposeSend(ctx, draft)
```

A trusted connector broker owns credentials, destination and operation
allowlists, request schemas, rate limits, budgets, retries, idempotency,
response limits, and audit receipts. Network responses enter the capsule as
tainted provenance-bearing artifacts rather than trusted instructions.

A separate durable actor is justified when an integration has an independent
lifecycle: inbound events, long waits, retry across restart, conversation
ownership, memory, or approval state. A bounded API call is not itself a reason
to create another agent.

Unknown APIs may eventually be described by owner-approved connector manifests,
possibly derived from OpenAPI. The manifest must bind endpoints, operations,
schemas, credential scopes, and effect classification before a typed module is
mounted. There should be no generic `http.Do` escape hatch.

## Email

Email illustrates the distinction between actor and connector well.

The Email actor may be durable because it owns conversations, inbound wakeups,
draft lineage, approval state, delivery receipts, and retries. The provider
adapter and delivery executor are trusted connector services, not model agents.

```text
provider adapter receives email
  -> durable EmailReceived envelope
  -> email actor wakes
  -> email RLM reads, reasons, drafts, or delegates
  -> EmailSendProposal
  -> policy and owner approval
  -> trusted outbox executor
  -> Delivered or Failed receipt wakes the actor
```

The Email RLM never sends raw SMTP or provider requests. Outbound communication
is an idempotent external-effect transaction. Attachments and message bodies are
tainted artifacts with source and privacy metadata.

This generalizes the current Texture-to-Email draft handoff and maild approval
path rather than replacing its safety boundary.

## Integration with the Audited Computer

The audited computer sees an RLM activation as a proposal and evidence producer,
not a semantic state authority.

Conceptually, Choir has three related ledgers:

1. **Canonical computer event tape:** semantic transitions, accepted and
   rejected effects, desired/effective heads, application, rollback, and
   reconstruction identity.
2. **Actor and RLM trace tape:** mailbox delivery, activation, Go cells, model
   and connector calls, continuations, results, failures, and recovery evidence.
3. **Capsule transaction tape:** speculative filesystem activity and commands
   that may be frozen into a content-addressed effect bundle.

They may share causal references without sharing authority. Recording a model
call does not make it a computer event. A trace or verifier result cannot append
or accept an event. A frozen filesystem diff remains inert until externally
accepted.

### Constructing a realization from tape

Given a `ComputerID` and target event head, reconstruction should:

1. load the base image or `ComputerVersion` checkpoint;
2. verify and replay the canonical event chain;
3. retrieve accepted artifacts and effect bundles by digest;
4. materialize the effective applied state;
5. restore durable actor records, mailbox cursors, memory, and work bindings;
6. launch fresh RLM capsules only when actors are subsequently woken.

Reconstruction never reruns model, tool, or network observations and never
restores a Yaegi heap. Pending proposals can be reconstructed as inert state,
but they do not silently become filesystem state.

### Promotion

An effects-capable CoSuper may construct and freeze a proposal:

```text
RLM activation
  -> capsule-local filesystem and command effects
  -> frozen CapsuleEffectBundle plus trace root
  -> independent verification
  -> external scoped acceptance event
  -> root guest materialization and health
  -> applied event and effective head
  -> ComputerVersion checkpoint
  -> vmctl route projection
```

No Yaegi module exposes `Accept`, `Promote`, `Materialize`, `Checkpoint`, or
`Route`. Those remain trusted host/runtime responsibilities. Acceptance binds
the exact bundle, base and current heads, capability policy, verifier evidence,
and immutable execution refs.

### Rollback

Rollback is a new forward transaction that selects a prior applied event and
rematerializes it. It does not erase or rewind the event tape.

At rollback, the runtime revokes activation capabilities bound to the displaced
head, cancels or stale-marks active capsules, rejects late outcomes through head
checks, and wakes affected actors in fresh activations against the new effective
state.

## Connection to Choir as It Exists

This proposal connects to live or code-present Choir substrate as follows:

| Existing surface | RLM interpretation | Required change |
| --- | --- | --- |
| `internal/actor` and `internal/actorruntime` | Durable actor loop, wake/passivate/restart | Replace the current model/tool activation body incrementally, not the mailbox substrate |
| `coagent_source_packet.v1`, `update_coagent`, store-backed channel log, and inbox delivery | Typed update seed plus separate audit and wake projections | Derive log, delivery, wake, supervision, and settlement consequences from one typed authoritative write; delete rather than preserve a dual path |
| Lifecycle trajectories and work items | Durable responsibility graph | Make RLM continuation, channel, and assignment relationships first-class in the existing graph |
| Role-specific tool registries | Prototype capability profiles | Replace persona-shaped registration with typed module manifests plus per-activation capabilities |
| `internal/modelpolicy` | Per-computer model policy and fallback seed | Generalize from one model per activation role to programmatic multi-call selection and consensus receipts |
| CoSuper capsule assignments | Initial effects-capable RLM target | Generalize implementation/verification scripts into capability-bound Go orchestration without weakening frozen-subject verification |
| Researcher typed updates | Research capability profile and typed outcome | Move research orchestration into the same RLM substrate while retaining its narrow canonical write boundary |
| Texture owner/reducer | Future RLM actor with unique canonical ownership | Preserve atomic Texture turn and reducer authority; do not expose direct document mutation outside its typed transaction |
| Existing Super lifecycle | Seed for persistent supervision actors | Replace singleton semantics with a scoped Super protocol, versioned seats, quorum receipts, and no direct effect authority |
| Email appagent and maild | Durable integration actor plus trusted connector/outbox | Move drafting/orchestration into RLM while retaining approval and delivery outside model-authored code |
| Capsule transaction builder | Typed effect proposal | Bind RLM trace root and exact activation/capability/model receipts into the frozen proposal |
| Computer event, updater, ComputerVersion, vmctl | Audited commit and projection path | No authority collapse; RLM outcomes enter only through existing typed reducers and transactions |

The largest conceptual simplification is that `Researcher`, `CoSuper`, and
eventually other agents no longer require separate model loops. They become
configurations of one durable-actor-plus-RLM kernel.

The largest required migration is coordination accounting. Current channel
messages are primarily string-bearing audit records while addressed delivery
and obligation consequences follow other paths. The target needs one typed
obligation mutation whose log, delivery, wake, supervision, and settlement
projections cannot disagree. The envelope is its observable transport shape,
not a second authority.

## Opportunities Opened by the RLM Kernel

### Context-scale independence

Agents can operate over collections much larger than any model window by
keeping context in artifact graphs and bringing only selected views into each
call.

### Concurrent and hot-swappable supervision

Because settlement-critical context belongs to the workstream, a trajectory can
survive process failure, model change, deliberate supervisor rotation, or actor
replacement. A stronger model can take a later shift over unresolved
obligations without pretending to restore the prior model's mind. This is
supervisor independence, not only long-context independence.

The same property supports multiple simultaneous supervisory seats. Seats can
inspect the same frozen decision state, exchange typed objections, or operate
independently until adjudication. Replacing one seat does not replace the
protocol, erase dissent, or strand the workstream.

### Explicit orchestration graphs

Yaegi provides adaptive imperative authorship, but its meaningful intermediate
plan should be inspectable as a dataflow/evidence graph: inputs, partitions,
calls, joins, disagreements, outcomes, and causal receipts. Choir should test a
hybrid in which the model writes Go while the broker and RLM runtime expose a
normalized execution graph. The graph is durable when it matters for
continuation or settlement; the interpreter heap remains disposable.

### System-owned consensus architectures

Consensus becomes a reusable Go library rather than prompt choreography. The
system can vary panel composition, independence constraints, dissent passes,
adjudication, cost, and stopping conditions without changing agent ontology.
For Super, this library is part of the production supervision protocol rather
than an optional technique chosen by a singleton supervisor. Its result is a
durable decision receipt and remains input to trusted authority enforcement.

### Capability composition instead of role proliferation

New functions can often ship as typed modules and policy rather than new
harness roles. A legal-research or incident-response actor can compose research,
source, communication, and evidence modules on the same kernel.

### Supervision as a programmable organizational graph

Shared channel policies allow a scoped set of supervisory seats to observe an
entire delegated workstream, not merely direct replies. Typed disagreements,
blockers, capability requests, and effect proposals can drive selective
wakeups, new rounds, and stronger quorum requirements.

### Reproducible orchestration recipes

Successful model-authored Go programs can be normalized into reviewed,
versioned orchestration libraries. The system can learn reusable patterns from
traces without granting those traces canonical authority.

A reviewed, pinned recipe may eventually become part of the computer's typed
artifact program. This must remain an earned promotion from adaptive execution,
not a requirement that freezes every novel RLM program before it can run.

### Evaluation below the final answer

Choir can evaluate model selection, context partitioning, delegation shape,
evidence coverage, retry behavior, and disagreement resolution as distinct
parts of an execution graph rather than scoring only a terminal response.
Trajectory settlement rules can then require exact evidence-graph properties
instead of equating a final answer or completed run with completed work.

### Connector ecosystem without ambient network authority

Typed connector manifests can make APIs composable by RLMs while preserving
credential, destination, privacy, and external-effect controls.

### Actor-level cost and resource economics

Because every inference and connector call is causally bound to work, Choir can
budget, attribute, compare, and optimize whole orchestration strategies rather
than isolated prompts.

## Promoted Invariants

The following claims were promoted into Choir Doctrine on 2026-08-12:

1. The complete execution unit is durable trajectory/workstream plus
   accountable actor plus disposable private activation.
2. Yaegi is an orchestration runtime, not the security boundary; the capsule,
   import policy, activation capabilities, and host capability checks are the
   layered boundary.
3. Persistent identity and accountability belong to the actor; settlement-
   critical continuity belongs to the trajectory/workstream, never the
   interpreter process.
4. Restricted profiles use the Go activation only. General process execution
   is absent by default and may be mounted for an effects-capable assignment.
5. Direct Bash and Go execution calls share one capsule broker and receipt path.
   Spawned processes never inherit Choir authority.
6. Activation-local goroutines and channels never substitute for durable actor
   communication, waiting, or recovery.
7. Every model and connector call crosses a trusted broker; credentials and
   provider tokens never enter model-authored code.
8. Module import does not grant authority without matching current activation
   capabilities and per-operation host validation.
9. Persisted model-authored source is inert and carries no activation
   capability, live handle, heap, credential, or prior authority into a later
   activation.
10. Free text cannot grant authority, and receiving a message cannot expand the
    recipient's capabilities.
11. Every consequential agent-to-agent update produces one authenticated typed
    mutation whose log, delivery, wake, supervision, and settlement
    consequences are causally joined projections.
12. Supervisory visibility is scoped by authoritative work relationships, not
    by a global omniscient role name.
13. Network reads produce tainted provenance-bearing artifacts; external writes
    use typed, idempotent effect transactions.
14. Every Go cell and consequential capability crossing produces immutable,
    citable causal evidence. The host derives the complete salient excerpt set;
    actors cannot omit inconvenient actions or dissent from supervision.
15. RLM traces, reports, Texture transclusions, and model consensus are
    evidence, not canonical computer-event, Texture, acceptance, or promotion
    authority.
16. A frozen capsule effect bundle is the only self-development candidate.
17. Reconstruction restores actors and accepted state, never executions.
18. Restore is an append-only forward transaction and revokes stale activation
    authority.
19. Super is a scoped, versioned protocol, never a singleton actor or ambient
    authority-bearing role name.
20. High-consequence and irreversible decisions require their predeclared
    supervision quorum; missing seats, failures, abstentions, and dissent cannot
    be erased by dynamically redefining the panel.

Exact APIs, schemas, module lists, safe-package allowlists, resource limits, and
migration sequence remain Definition-owned implementation questions rather than
doctrine.

## Adoption Sequence

### Phase 1: common kernel, first CoSuper profile

- Embed one private Yaegi interpreter per bounded activation in disposable
  CoSuper capsules.
- Make Go-cell evaluation the only model-facing path for artifacts, assignments,
  messaging, evidence, work, and outcomes; do not preserve duplicate ambient
  tools.
- Expose direct Bash only for an effects-capable implementation assignment and
  route it through the same broker as Go execution calls.
- Expose artifact, execution, evidence, assignment, message, trace, and outcome
  modules with activation-scoped capabilities.
- Return typed implementation, verification, evidence, and continuation
  outcomes.
- Preserve current capsule-only effects and the reversible envelope proven by
  the supervised-self-development-effects Definition; expose no
  Accept/Materialize operation inside the interpreter.
- Prove context virtualization and dynamic multiagent orchestration inside one
  sealed assignment before depending on a new shared-channel design.
- Kill an activation mid-work, resume with a different permitted model, and
  complete from durable artifacts and obligations without restoring Yaegi or
  relying on hidden conversation state.
- Produce supervision excerpts and a Texture version transcluding exact command,
  message, and recovery or verification receipts.

### Phase 2: Research capability profile

- Mount source/search/fetch/evidence modules on the same RLM kernel.
- Preserve the Researcher's narrow typed canonical update path.
- Compare ephemeral multi-model research with durable delegated research.
- Remove role-specific loop behavior that capability policy can express.

### Phase 3: typed supervised obligation projections

- Generalize the existing typed coagent update into one versioned obligation
  mutation rather than adding another envelope authority.
- Derive channel log, addressed delivery, actor wake, supervision, work
  settlement, and Trace from that write.
- Add participant membership and scoped supervisor subscriptions.
- Make capability, disagreement, blocker, and effect messages mechanically
  recognizable.
- Prove duplicate and reordered delivery reduces idempotently to the same
  supervisory state.

### Phase 4: connector and Email convergence

- Put source and API access behind typed connector modules.
- Preserve maild as trusted ingress, approval, and delivery infrastructure.
- Move Email drafting and conversation orchestration onto the RLM kernel.
- Prove tainted input handling and idempotent external effects.

### Phase 5: Texture and the Super protocol

- Give persistent actors versioned seats in a system-owned Super protocol with
  supervised-channel affordances and no direct effect authority.
- Add decision-class quorum, independence, dissent, timeout, and replacement
  receipts; require stronger policy for irreversible decisions.
- Evaluate Texture on the RLM substrate while preserving its unique owner and
  atomic reducer contract.
- Delete superseded persona-specific activation loops only after product-path,
  restart, supervision, and refusal evidence proves the replacement.

## Implementation Questions For The Definition

- What is the smallest safe computational standard-library allowlist, and how
  are source imports and rich returned host types checked for indirect authority
  leaks?
- What exact module API prevents artifact refs, actor addresses, and generic
  clients from becoming confused-deputy or cross-workstream capability paths?
- Which compiled combinators should handle common concurrency while arbitrary
  interpreted goroutines remain bounded by capsule limits?
- What is the exact typed continuation schema and compaction policy?
- Does the RLM trace retain exact source plus a normalized orchestration graph,
  and what minimum graph edges are required for supervision?
- What constitutes deterministic-enough replay evidence when model calls and
  connector observations are intentionally not rerun?
- How do message schema evolution and old-actor replay interact?
- Which supervision events wake one seat, all seats, or open a stronger quorum
  round?
- How are connector manifests reviewed, versioned, revoked, and reconstructed?
- When does repeatedly successful source become a reviewed recipe, and what
  evidence prevents premature promotion?
- What resource and output limits preserve useful arbitrary capsule-local
  execution while reliably killing fork, leak, deadlock, disk, and output
  exhaustion?
- Which command, message, refusal, and recovery receipts are host-selected as
  salient, and how are privacy/redaction and exact Texture transclusion ranges
  represented?
- What is the minimum first acceptance proof that demonstrates the common
  kernel rather than replacing JSON tools with Go syntax? The bar is a sealed
  CoSuper assignment with dynamic delegation, citable consequential receipts,
  forced activation death, cross-model recovery from durable state, and an exact
  capsule effect bundle whose acceptance remains external.

## Lateral Agentic Consensus

The first complete draft was reviewed on 2026-08-09 by the repository's default
agentic-consensus panel in lateral mode. The reviewed candidate SHA-256 was
`51e0cfa2a9638f31e5bf119343f96ceed93b3fa9b90c4fa6f6cedf01136a25b3`.
The exact prompt digest was
`16b803028060210ea258c4c72a717bb44a677e21ee7363dff0fb79c00d81241d` and
the manifest digest was
`20a235270090a266655f74a4eba666bb3a7461b3bc184736e759b9c96d0a533f`.

### Panel health

| Panelist | Status |
| --- | --- |
| Devin configured default | succeeded |
| Cursor configured default | succeeded |
| OMP Gemini 3.6 Flash, high thinking | succeeded |
| OMP DeepSeek V4 Flash Free, high thinking | succeeded |
| OMP GPT-5.6 Sol, medium thinking | succeeded |
| OMP Cursor Grok 4.5 High | failed: provider API key unavailable |
| OpenCode configured default | timed out at 240 seconds |
| Codex configured default | timed out at 240 seconds |

Raw session diagnostics remain in the gitignored local directory
`.agentic-consensus/agentic-consensus-20260809-003834/`; the adjudicated
findings are recorded here because the raw directory is not durable evidence.

### Convergent reframes

Four of the five successful panelists independently rejected the draft's
actor/message-centric unit. Their analogies differed—air-traffic control,
clearinghouse accounting, Kubernetes reconciliation, and materialized
views—but their architectural claim converged:

- persistent actors should remain addresses, accountable principals, and
  organizational relationships;
- the trajectory/work-item, obligation, artifact, and evidence graph should
  own settlement-critical problem continuity;
- typed messages should be authenticated reducer inputs and observable
  projections, not a parallel semantic authority;
- CoSuper is better understood as a context query planner, obligation compiler,
  or sector controller than as a coding worker;
- the first acceptance proof should demonstrate recovery and reassignment from
  durable workstream state, not merely successful Yaegi evaluation.

This reframe was locally verified against current doctrine: C5 assigns roles to
authority envelopes, C6 targets durable actors, C7 assigns causality and
settlement to trajectories/work items, I5 rejects dual paths, and I7 requires
settlement-relevant coordination to become durable obligation state. Current
`update_coagent` already separates model-authored typed payload from
runtime-loaded delivery authority and wakes only after the durable write.

### Productive dissent

One panelist argued that Yaegi itself may be the wrong center: Choir could make
a declarative, durable execution/evidence graph the primary RLM intermediate
representation and treat compiled Go transformers as only one executor. This is
not accepted as a replacement because adaptive model-authored orchestration is
the proposal's central capability, and forcing every local loop through durable
graph writes would add latency and risk converting agency into a workflow
engine.

The objection is retained as a hybrid design requirement: Go execution should
not be opaque. The RLM runtime and brokers should expose a normalized
orchestration graph, and any state needed for continuation, supervision, or
settlement must become durable graph state rather than remaining only in the
Yaegi heap.

A second minority reframe proposed that recurring RLM programs, not actors or
activations, are the real durable unit and should become reviewed artifact
programs. This is retained as an opportunity rather than an initial invariant.
Prematurely freezing every novel program would sacrifice the adaptivity that
motivates an RLM; successful recurring recipes can earn promotion later.

### Adjudication

The panel materially changed this memo in five ways:

1. The kernel is now explicitly trajectory + accountable actor + disposable
   activation, not actor + activation alone.
2. The supervision design now distinguishes transport, supervision, and
   authority planes.
3. The communication migration builds on the existing typed coagent obligation
   write and treats log/wake/channel views as projections rather than inventing
   another message authority.
4. Hot-swappable, cross-model supervisory shifts and normalized orchestration
   graphs are now first-class opportunities.
5. Phase 1 acceptance now requires forced activation death and cross-model
   recovery from durable artifact and obligation state.

The panel did not change the capsule security boundary, model-broker ownership,
Email outbox boundary, or event-derived promotion and rollback design; no
successful reviewer identified a reason to collapse those authorities.

## Current Non-Claims

This memo does not claim that:

- Yaegi or the RLM kernel is implemented in Choir;
- doctrine promotion is deployed product evidence;
- self-development effects are enabled or accepted on staging;
- current role registries are already safe general-purpose module systems;
- the current string channel record is the proposed typed supervision plane;
- model consensus can authorize promotion;
- an interpreter or import allowlist can replace capsule isolation and
  per-operation capability validation;
- arbitrary network access is safe inside a capsule;
- a ComputerVersion, route, checkpoint, trace, report, transclusion, or verifier
  statement is semantic event authority.

## Relationship to Current Doctrine

The kernel, capability, execution-profile, learned-source, and citable-evidence
claims in this memo were promoted into [Choir Doctrine](choir-doctrine.md) and
[Choir Agent Product Doctrine](agent-product-doctrine.md) on 2026-08-12. Those
documents now govern. This memo remains the architecture synthesis and design
rationale; its implementation questions and adoption sequence are proposal
material for the successor executable Definition and cannot overrule doctrine.
