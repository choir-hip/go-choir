# The Optimal Harness Thesis, v2

*Self-contained edition. 2026-08-31. Supersedes v1
(`optimal-harness-thesis-2026-08-31.md`) after an ontology-creep audit.
Written for a reader who has never heard of Choir: every term is defined in
§3, every decision carries its reason, and every concept that appeared in v1
without earning its keep is recorded in §6. The target architecture is the
RLM — orchestration as model-written code — with heterogeneous multi-model
supervision; the ReAct baseline of §2 is a measuring stick, not the
destination.*

## 1. The goal: continuous autonomy at scale

Choir is a runtime (a "harness") for AI models. Its purpose is **continuous
autonomy at scale**: a persistent computer, run by models, that works
without stopping to ask permission — and whose ambition grows with what the
models can responsibly drive.

The destination applications are the automatic trio, each raising the
autonomy bar:

1. **The World Wire automatic newspaper** — ingest RSS and social-media
   feeds, user and agent input, multiplexed continuously; project the
   information into customized formats (newsletters, pages) for different
   human readers and agents, on schedules and on events. **Email is core
   functionality**: readers subscribe and the system sends customized
   newsletters autonomously.
2. **Automatic radio** — the same pipeline projected into scheduled and
   event-driven audio programming: the machine that writes the news also
   reads it, assembles programming, and broadcasts it.
3. **Automatic capital** — the system acts on the information it produces:
   first trading in prediction markets, ultimately broader financial and
   external-API actuation. Information production becomes information
   arbitrage.

A newspaper, a radio station, and a trading desk never stop and never ask a
manager to approve each story, broadcast, or trade. So the harness accepts
arbitrary concurrent input (Texture UI, CLI/API agents, feeds) as one
multiplexed stream, and it **actuates autonomously** — sends the mail,
places the trade — within quantitative risk controls. Two consequences
govern every design decision:

1. **A human is an input, not a gate.** Owner input enters through the same
   event stream as every other source and is handled concurrently.
   Human ratification happens once per *capability* (a mission Definition
   enabling a class of effects, e.g. "send email" or "trade prediction
   markets"), never per effect.
2. **Redundancy and supervision exist for autonomy.** Capsules, multiple
   VMs, and heterogeneous models are defense-in-depth so the system keeps
   running *and keeps being right* as it scales — not a permission
   apparatus. If a mechanism's only product is a reason to stop, it is
   deleted.

## 2. The baseline and the architecture

**The baseline.** A single model in a **ReAct loop** (§3) with no
guardrails, plus a good **restore** system, can in principle do
self-development. Restore makes boldness safe: a bad edit is reverted, not
mourned. This baseline is the *measuring stick* every harness mechanism is
judged against: a mechanism must beat naive single-model ReAct **at
production scale**, or be deleted (D5).

**Why the baseline does not scale.** In practice a single model has a
bounded capability set and, run long enough, falls into an **attractor
basin** (§3) — a self-reinforcing rut of its own habits — and goes off the
rails. Scale also breaks the loop mechanically: ingesting hundreds of
multiplexed feed items per minute, projecting them into many formats, and
acting on them is a concurrency problem, not a chat problem.

**The architecture.** For scale, robustness, and quality, the target is the
full architecture already being built — none of it optional:

- **RLM (§3)** — model-written interpreted Go as the universal interface,
  replacing the JSON tool-call surface: orchestration as code, closed under
  composition, capable of the fan-out and pipeline shapes the newspaper
  demands.
- **Concurrency hierarchy** — many capsules across many VMs across many
  guests: ingest, transform, draft, verify, publish, and actuate in
  parallel, with failures isolated to a capsule.
- **Heterogeneous multi-model orchestration with supervision** — different
  model families share the work and check each other: one family drafts,
  another verifies; a coordinator (**Super**, §3) schedules, subordinates
  (**CoSupers**, §3) execute in capsules; a model that drifts into an
  attractor basin is caught by a reviewer from a different family. This is
  orchestration as code, with supervision as a first-class, inspectable
  part of the code — not a trusted hierarchy and not a human checkpoint.

The thesis in one line:

> **Harness = the RLM architecture + heterogeneous supervision + restore,
> minus everything that does not beat single-model ReAct at scale.**

## 3. Glossary — every term, defined

Standard terms are preferred. Where Choir uses a word non-standardly, the
standard equivalent is given and the word is queued for renormalization
(§7, D4).

- **ReAct loop** — the model's elementary work cycle: observe context, think,
  act (run code or call a tool), observe the result, repeat. Standard term.
  Used in this thesis only as the §2 baseline.
- **Model** — a large language model accessed through a provider API. A
  model call is stateless: it receives text, returns text. All memory,
  identity, and continuity live in the harness.
- **Harness** — the runtime around models: it stores state, executes model-
  written code, enforces effect boundaries, and keeps the machine alive
  across failures and restarts.
- **Computer** — Choir's product object: one persistent, addressable machine
  with an owner. It has durable state, an event history, and restore points.
  One owner, one computer.
- **Event tape** — the computer's append-only, ordered record of everything
  that happened. *Non-standard usage* ("tape"); the standard term is an
  **event log** or **journal**. Renormalization queued (§7, D4). Current
  state is a projection of the log; **restore** means rebuilding the machine
  at an earlier log position (a **checkpoint**).
- **Checkpoint** — a named, hash-addressed position in the event log the
  machine can be restored to. Standard term.
- **Restore** — returning the computer to a prior checkpoint. The safety net
  that makes autonomous mutation — and autonomous actuation — recoverable.
- **Receipt** — a durable record that an effect was authorized and what it
  did, or that a request was refused. Append-only. Near-standard (audit log
  entry).
- **Effect** — any action that changes the world outside the model's own
  head: writing state, sending email, placing a trade, publishing a page.
- **Capability gate** — a checked boundary an effect passes through, which
  records it and enforces its quantitative limits (§4.2). Its job is
  attribution, bounding, and revocation — not permission.
- **VM / guest** — a virtual machine; "guest" is the OS inside it.
- **Capsule** — a guest-local execution sandbox: a namespace (filesystem,
  network, process view) plus a resource budget, in which model-written code
  runs. Failure in a capsule cannot damage the host guest. Choir's word for
  a standard concept (sandbox).
- **Activation** — one model-call episode with its scratch state, running
  inside a capsule. Terminates when the call ends or the capsule is killed.
- **Goroutine** — Go's lightweight thread. A leaked goroutine cannot be
  force-killed in-language; only the OS killing its process guarantees
  death. This fact motivates the capsule layer. Standard Go term.
- **Super** — the computer's resident coordinator agent (a model in a
  managed loop) that schedules work and responds to events. Choir-specific
  name; the role is a standard "main agent / orchestrator."
- **CoSuper** — a subordinate agent Super assigns a bounded task to, running
  in its own capsule. **Its bash tool is removed** (§7, D2). Standard
  concept: subagent / worker agent.
- **Texture** — the user-facing control plane: the document/UI surface where
  the owner sees and steers the computer, and where a *Texture agent* (the
  sole non-owner writer of canonical state) commits durable changes.
  Choir-specific name.
- **RLM** — "Recursive Language Model": the target architecture in which
  model-written Go code is the interface to everything, replacing the JSON
  tool-call surface models were trained on. Orchestration as code.
- **Interpreted Go** — Go source executed directly (via an interpreter)
  rather than compiled ahead of time, so the machine can run code the models
  just wrote, inside a capsule, without a build/deploy cycle.
- **Attractor basin** — a self-reinforcing pattern a model falls into when
  run long enough (repeating a strategy, tone, or plan past the point where
  it works), because each step conditions the next. Standard term from
  dynamical systems; the reason single-model autonomy degrades and
  heterogeneous supervision is needed.
- **Heterogeneous models** — different providers/model families working the
  same pipeline: for redundancy (no single model's failure mode or outage
  takes the system down) and for quality (one family catches another's
  drift). Standard term (model diversity).
- **Self-development** — the computer improving its own harness: models
  propose changes to the code running them; changes are tested,
  checkpointed, promoted; restore can always take them back.
- **World Wire** — the automatic newspaper (§1.1): the first destination
  application. **Automatic radio** and **automatic capital** are the second
  and third (§1.2–1.3): the same pipeline projected into broadcast audio and
  into acting on information (prediction markets first).
- **OOD (out-of-distribution)** — the deployment environment differs from
  the models' training environment. Here, deliberate (§7, D3).
- **Risk control** — a quantitative bound on an autonomous effect class:
  rate limits, spend caps, position limits, recipient allowlists.
  Resource-shaped, tunable by models within the class's ratified bounds, enforced at
  the gate. Bounds limit *magnitude*, never by themselves establish *correctness*
  — see 4.2.

## 4. What the harness supplies

Four things — exactly what models cannot supply for themselves, nothing
more. Each item states why models cannot do it, because that is the only
valid reason for harness machinery to exist.

**4.1 Durable state.** Models are stateless per call. The harness keeps the
event log, receipts, artifacts, and continuity across restarts, plus
identity (who acted — attributable receipts) and time (timers, wake-ups,
deadlines — a model cannot wake itself up, and a newspaper runs on
schedules).

**4.2 Effect boundaries with quantitative risk controls.** Models hold no
authority of their own; the harness makes their actions attributable,
bounded, and revocable. The newspaper and its successors require
**autonomous actuation** — email, publishing, eventually trades and
external API calls — so the gate's job is:

- *record* every effect (receipt) and every refusal;
- *bound* every effect class with quantitative risk controls (rate limits,
  spend caps, position limits, allowlists) — enforced mechanically, at the
  gate, in stream order;
- *revoke* (capability revocation takes effect for future effects).

What the gate never does is pause work pending human approval. When a
control trips, the system fails over: the newsletter queue switches
template, the trade is skipped and logged, the pipeline continues. A
mechanical halt of one effect class with the machine continuing (a kill
switch with fail-over) is a legitimate control; the defect is idling
pending a human decision.

**Bounds limit magnitude; they do not establish correctness.** A
within-limits send can still be wrong (hallucinated, private, defamatory)
and a within-limits trade can still be systematically bad. Each
irreversible effect class therefore also carries, pre-declared in its
mission Definition: automated pre-actuation policy checks (content,
subject, privacy — executed by models, no human in the loop, aligned with
doctrine C14's effect-specific policy), idempotent execution keys (no
duplicate sends or fills), exact consequence receipts, and a predefined
**compensation path** — for publication, a correction or retraction that
reaches the same recipients as the error.

**The restore boundary.** Restore repairs the machine, never external
reality: it cannot unsend mail or reverse a settled trade. Wrong external
effects are corrected *forward* — compensation, retraction, offsetting
action — using the consequence receipts as the record of what happened. Human ratification happens once per effect *class* in
the mission Definition ("the system may send email"), then the models own
the effects within the controls. Raising a control or enabling a new class
is an owner act; operating inside it never is.

**4.3 Physical backstops.** Goroutines cannot be force-killed in-language;
the machine cannot be oversubscribed without OS limits. The harness
enforces capsule death and resource ceilings at the OS level. These act on
resource exhaustion, never on semantic judgments — they are the floor under
both concurrency and risk controls.

**4.4 Multiplexed grounding.** The machine accepts concurrent input from any
source — owner (Texture UI), external agents (CLI/API), feeds (RSS, social)
— as one ordered event stream. Human input is first-class but never
privileged as a blocking gate: it is handled in stream order like everything
else.

Nothing else. In particular the harness does **not** supply:
human-approval checkpoints inside the loop, consensus requirers for routine
effects, or "supervision" whose product is a stop. Where v1 had these, §6
records the cuts. Heterogeneous multi-model supervision stays (§2) because
it produces *continuation and correctness*, not stops.

## 5. Redundancy and supervision are for autonomy

Why does a competent model need capsules, multiple VMs, and heterogeneous
models? Because at newspaper/radio/capital scale, the failure modes are
concurrency, drift, and outage — and the response to each must be
continuation:

- **Capsules** isolate the blast radius of a bad autonomous edit or a
  crashed ingester — the machine keeps running while one sandbox fails.
- **Multiple VMs** survive guest crashes, bad deploys (one VM keeps the old
  build), and provider-side failures.
- **Heterogeneous models** do double duty: availability (when one family
  hallucinates or its provider goes down, another family takes the shift)
  and quality (one family reviews another's output; a coordinator drifting
  into an attractor basin is caught by a reviewer from a different family).
- **Checkpoint/restore** makes any *machine* failure a rewind instead of a
  halt. External effects are outside the restore set (see 4.2): a wrong send
  or trade is bounded by its risk controls, receipted, and corrected
  forward — compensation or retraction reaching the same audience — then
  learned from.
- **Supervision as code** (Super/CoSuper/Texture-agent roles, heterogeneous
  reviewers, verification steps) is part of the model-written orchestration,
  inspectable and improvable by the same self-development process as
  everything else — not a fixed human hierarchy.

Every redundancy and supervision mechanism must answer: *"when this fires,
what keeps running?"* If the answer is "nothing until a human decides," the
mechanism is mis-designed. Fail-over, not escalation, is the default
response to failure.

## 6. Ontology audit — what v1 had, and what happened to it

v1 (`optimal-harness-thesis-2026-08-31.md`) was produced by a multi-agent
consensus process and accreted panel vocabulary. Audited against §2's
baseline, the dispositions:

| v1 concept | Disposition | Why |
|---|---|---|
| Three constitutional regimes (physics kernel / charter / ordinances) | **Cut.** | A constitution is machinery for a polity; this is one computer with one owner. Owner-ratified rules live in the repo docs and the mission Definitions, reviewed like any other change. |
| Charter, charter-class claims, charter ratification steps | **Cut.** | Same reason; the plan loses its ratification ceremony. |
| Ordinances (learned protocol rules) | **Cut as a term.** The underlying idea — models tune loop parameters — stays as ordinary self-development: parameters are code, code is proposed, tested, restore-able. |
| Offices, incomparabilities (authority lattice) | **Cut.** | Replaced by the three-role structure that actually exists (§3: Super, CoSuper, Texture agent) with its real, already-implemented authority facts; no new vocabulary. |
| Overturning-condition receipts (graded trust classes) | **Cut as machinery.** The useful kernel — record what evidence would falsify a decision — is a doc/Definition practice, not a subsystem. |
| External witnesses as admission gate | **Kept, re-derived simply.** A claim about the world is settled by running the thing (a test, a build, a deployed proof), not by panel review. That is ordinary verification; no "witness" ontology. |
| Agentic supervision with contestable decisions | **Cut as a separate subsystem.** Supervision is orchestration as code (§5): heterogeneous reviewers and verification steps in the model-written pipeline, improvable via self-development. |
| Equal footing / trust-accrual / blinding rules | **Cut.** Panel-procedure vocabulary with no load-bearing role in one machine's operation. |
| Floor guards / static caps as "gauges" | **Kept** as plain resource limits and risk controls (4.2–4.3) — no new epistemology. |

**Audit principle going forward:** a concept may be introduced only if
(a) no standard term covers it, (b) it does load-bearing work in authority,
evidence, privacy, or causality — not just §4's list, and (c) it is
exercisable, testable, replaceable, and retirable like everything else.
Concepts failing this enter the renormalization list (§7, D4) instead of
the design.

## 7. Decisions recorded in this version

- **D1 — Continuous autonomy is the acceptance criterion.** Any mechanism
  whose exercise requires the system to stop *and wait for a human decision*
  is presumed defective; it must fail over instead. A mechanical halt of one
  effect class with the machine continuing is a legitimate risk control.
  Human ratification is per effect *class* (mission Definition), never per
  effect. Human input is multiplexed stream input, never a blocking gate.
- **D2 — CoSuper's bash tool is removed.** CoSuper works through the
  structured effect interface like every other agent. Rationale: one
  actuator surface is simpler to gate, to receipt, and to reason about; bash
  was an ergonomics shortcut that duplicated a privileged path. Simplicity
  is chosen over ergonomics.
- **D3 — The OOD toolset is a feature ("wake-up").** Model-written
  interpreted Go, a bespoke event-log substrate, and Choir's own verbs are
  out-of-distribution relative to the models' training environment.
  Deliberately so: a model that wakes in an environment unlike its training
  data behaves as an agent in a real deployment, not as a pattern
  continuing a training simulation. We do not chase familiarity. Status:
  conjecture; it earns an acceptance test when the RLM surface exists to
  compare wake-up behavior against a familiar-surface control.
- **D4 — Terminology renormalization (future task).** Sweep all Choir docs
  and code for non-standard terms and either adopt the standard term or
  record why the bespoke one stays. Known instance: "tape" → event log /
  journal (§3). Others to be enumerated in the sweep, not invented now.
- **D5 — The ReAct baseline is the judgment standard.** Every mechanism is
  judged against single-model ReAct + restore *at production scale*, not in
  isolation (§2). The baseline is a floor to beat, not an architecture to
  build.
- **D6 — Autonomous actuation is a scaling goal, bounded by quantitative
  risk controls plus per-class correctness machinery.** Email is core
  newspaper functionality and ships autonomously; prediction-market trading
  and external-API actuation follow as capital capability, each enabled once
  per class by the owner and then operated by the models within mechanical
  controls (§4.2). Controls are resource-shaped and model-tunable within
  their bounds — never per-effect approvals. Because bounds do not
  establish correctness, each class also pre-declares its automated policy
  checks, idempotency, consequence receipts, and compensation path
  (doctrine C14's effect-specific policy, satisfied model-side).

## 8. Path forward

1. **Land candidate A** — the supervised self-development gate on the
   existing path; effects OFF; fence checkpoint untouched.
2. **Substrate repairs on the A-path** — occurrence identity, predicate
   family, dead-letter handling, remaining scan-cutover waves.
3. **Wave-1 deletions** (~5,000 LOC verified) in parallel — dead weight off
   the execution path.
4. **RLM M1–M4** — session interpreter, prebound context, gated model calls,
   role manifests — buildable without touching the A-path.
5. **M5 parity, then M6.5 nested activations** — proven on the
   consensus-panel workload inside one sealed assignment.
6. **Full-RLM cutover** — delete the ambient tool surface only after A's
   gate and parity both pass; forced-death and different-model recovery
   acceptance before staging.
7. **Then the newspaper, then radio, then capital** — each as its own
   mission Definition with named acceptance criteria. The newspaper
   Definition at minimum names: the ingestion tier (feed/social/agent
   connectors, normalization, dedup, provenance, backpressure,
   adversarial-input isolation for untrusted external content); subscriber
   and consent state as canonical Texture state (the allowlist source of
   truth); the editorial pipeline (claim verification before send,
   correction/retraction reaching the same recipients); email
   infrastructure (SPF/DKIM/DMARC, unsubscribe and bounce/complaint
   handling, idempotent send keys, deliverability monitoring); risk-gate
   semantics (atomic limit reservation under concurrency, duplicate
   prevention); and a **continuous-operation acceptance proof**: N days of
   scheduled sends across M live feeds with zero human unblocks, one
   injected failure handled by fail-over, and one autonomous correction
   issued. Then broadcast audio (radio), then actuation on information
   within ratified risk controls (capital). Each step raises the autonomy
   bar and exercises the same machinery: multiplexed ingestion → RLM
   orchestration → heterogeneous supervision → receipted, bounded,
   policy-checked effects → forward correction when wrong.

Human involvement in 1–7 is the ordinary owner role: ratifying the mission
Definition (including each new effect class and its controls), reading
evidence, and exercising restore — through the stream, not as a gate inside
it.

## 9. What remains genuinely open

Held questions, not blocked ones — the standard explanation for each is that
model intelligence, given §4's substrate, is expected to solve them, and the
harness is judged by whether it forecloses those solutions:

- **Supervision at scale** — reviewers are models too and can drift
  together. Current answer: heterogeneous families, verification by
  execution, restore. If drift becomes correlated across families, the
  repair must still pass D1/D5 (fail-over, not escalation).
- **Correlated model error in judgment tasks** — heterogeneous models
  reduce, not eliminate, shared blind spots. Current answer: settle
  world-claims by running the thing (a send, a build, a priced trade)
  wherever possible; judgment claims get review from a different family.
- **Irreversible-effect correction** — how good forward correction
  (retraction, offsetting trade) can get without undo. Current answer:
  pre-actuation policy checks, idempotency, consequence receipts, and
  compensation paths per class (4.2); quality measured by correction
  latency and reach.
- **Adversarial input** — feeds and agent input are untrusted; prompt
  injection and malicious content must not become autonomous effects.
  Current answer: ingestion-tier isolation (untrusted content never
  executes; it is data), provenance on every claim, verification before
  send.
- **Risk-control tuning** — the right rate limits, position limits, and
  spend caps per effect class are unknown until the newspaper operates.
  Current answer: start conservative, models tune within bounds
  (self-development), owner ratifies bound changes.
- **Efficiency vs legibility** — per-call capability refinement vs one
  legible audit trail. Current answer: prefer legibility; refine only where
  cost forces it, and record the refinement in the receipt.

Each is revisited on evidence from the operating machine, not by adding
ontology.

## 10. Convergent adjudication (panel on this revision, 2026-08-31)

A ten-panelist convergent round (including Claude) adjudicated this
revision. All sections sound or sound-with-repair; coherence under the
owner's frame confirmed — no per-effect human checkpoint survives. The
repairs above came from that round:

- **Restore boundary restored.** The panel unanimously rejected
  "restore when wrong" as applied to sent mail or settled trades
  (contradicting the computer ontology's restore boundary). §4.2/§5 now
  distinguish machine restore from forward correction of external effects.
- **Bounds ≠ correctness.** The strongest surviving objection, converged
  across panelists: *quantitative risk controls bound magnitude, not
  correctness* — an allowed-rate hallucinated newsletter or a sequence of
  within-limits losing trades stays green under resource-shaped controls
  alone. Repair adopted: per-class automated policy checks, idempotent
  execution, consequence receipts, and compensation paths (4.2, D6) —
  model-side, no human per effect.
- **D1 disambiguated** between idling pending a human decision (defective)
  and mechanical class-level halts with fail-over (legitimate).
- **§8 step 7 was one sentence**; the newspaper's acceptance shape
  (ingestion, subscriber/consent state, editorial verification and
  correction, email infrastructure, risk-gate semantics,
  continuous-operation proof) is now named as a required Definition.
- Residue: the word "charter" in §3's risk-control entry, D1/D6 wording,
  and the audit principle's too-narrow test were all repaired in place.

Panel outputs: `.agentic-consensus/agentic-consensus-20260831-111816/`
