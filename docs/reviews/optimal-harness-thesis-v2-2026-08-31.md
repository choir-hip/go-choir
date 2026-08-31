# The Optimal Harness Thesis, v2

*Self-contained edition. 2026-08-31. Supersedes v1
(`optimal-harness-thesis-2026-08-31.md`) after an ontology-creep audit.
Written for a reader who has never heard of Choir: every term is defined in
§3, every decision carries its reason, and every concept that appeared in v1
without earning its keep is recorded as deleted in §6.*

## 1. The goal: continuous autonomy

Choir is a runtime (a "harness") for AI models. Its purpose is
**continuous autonomy**: a persistent computer, run by models, that works
without stopping to ask permission.

The destination application is the **World Wire automatic newspaper**: a
system that starts by pulling RSS and social-media feeds, then takes
increasing human and agent input, and projects it into more formats for more
readers. A newspaper never stops. It accepts input continuously — a human
typing in the Texture UI, an agent calling the CLI, a feed delivering items —
all multiplexed into one stream. The system processes that stream
continuously. It does not pause for approval, does not halt for supervision
checkpoints, does not generate reasons for a human to intervene.

Two consequences govern every design decision below:

1. **A human is an input, not a gate.** Owner input enters through the same
   event stream as every other source and is handled concurrently. The only
   human-gated acts are the ones physics makes irreversible from inside the
   machine (see §4).
2. **Redundancy exists for availability, not control.** Capsules, multiple
   VMs, and heterogeneous models are defense-in-depth so the system keeps
   running when a component fails — not a supervision apparatus. If a
   mechanism's only product is a reason to stop, it is deleted.

## 2. The minimal machine

A single model in a **ReAct loop** (§3) with no guardrails can already do
self-development if the model is smart enough. Add a good **restore** system
— the ability to return the machine to a prior state — and that is
basically enough. Restore is what makes boldness safe: an edit that breaks
the computer is reverted, not mourned. Everything else in this harness must
justify itself against that baseline. The thesis in one line:

> **Harness = ReAct loop + durable restore + the few things models cannot do at all.**

The last clause is the entire design discipline. §4 names those things.
§6 records the machinery that failed this test and was cut.

## 3. Glossary — every term, defined

Standard terms are preferred. Where Choir uses a word non-standardly, the
standard equivalent is given and the word is queued for renormalization
(§7, D4).

- **ReAct loop** — the model's elementary work cycle: observe context, think,
  act (run code or call a tool), observe the result, repeat until the task
  is done. Standard term.
- **Model** — a large language model accessed through a provider API. A
  model call is stateless: it receives text, returns text. All memory,
  identity, and continuity live in the harness.
- **Harness** — the runtime around models: it stores state, executes model-
  written code, enforces effect boundaries, and keeps the machine alive
  across failures and restarts.
- **Computer** — Choir's product object: one persistent, addressable machine
  with an owner. Not a metaphor: it has durable state, an event history, and
  a restore point. One owner, one computer.
- **Event tape** — the computer's append-only, ordered record of everything
  that happened. *Non-standard usage* ("tape"); the standard term is an
  **event log** or **journal**. Renormalization queued (§7, D4). Everything
  durable derives from it: current state is a projection of the log, and
  **restore** means rebuilding the machine at an earlier log position
  (a **checkpoint**).
- **Checkpoint** — a named, hash-addressed position in the event log the
  machine can be restored to. Standard term.
- **Restore** — returning the computer to a prior checkpoint. The safety net
  that makes autonomous mutation survivable. Standard term.
- **Receipt** — a durable record that an effect (a real-world action) was
  authorized and what it did, or that a request was refused. Append-only.
  Near-standard (audit log entry).
- **Effect** — any action that changes the world outside the model's own
  head: writing state, starting a process, calling a provider, publishing a
  page. Standard term.
- **Capability gate** — a checked boundary an effect passes through, which
  records who authorized it. Minimal by design: gates exist so effects are
  attributable and revocable, not to ration work (§4.2).
- **VM / guest** — a virtual machine; "guest" is the OS running inside it.
  Standard terms.
- **Capsule** — a guest-local execution sandbox: a namespace (filesystem,
  network, process view) plus a resource budget, in which model-written code
  runs. Failure in a capsule cannot damage the host guest. Choir-specific
  word for a standard concept (sandbox / container-lite).
- **Activation** — one model-call episode with its scratch state, running
  inside a capsule. Terminates when the call ends or the capsule is killed.
- **Goroutine** — Go's lightweight thread. A leaked goroutine cannot be
  force-killed by the language; only the OS killing its process guarantees
  death. This fact motivates the capsule layer. Standard Go term.
- **Super** — the computer's resident coordinator agent (a model in a
  managed loop) that schedules work and responds to events. Choir-specific
  name; the role is a standard "main agent / orchestrator."
- **CoSuper** — a subordinate agent Super assigns a bounded task to, running
  in its own capsule. Choir-specific name; standard concept: subagent /
  worker agent. **Its bash tool is removed** (§7, D2).
- **Texture** — the user-facing control plane: the document/UI surface where
  the owner sees and steers the computer, and where a *Texture agent* (the
  sole non-owner writer of canonical state) commits durable changes.
  Choir-specific name.
- **RLM** — "Recursive Language Model": the target architecture in which
  model-written Go code is the interface to everything, replacing the JSON
  tool-call surface models were trained on (§7, D3).
- **Interpreted Go** — Go source executed directly (via an interpreter)
  rather than compiled ahead of time, so the machine can run code the models
  just wrote, inside a capsule, without a build/deploy cycle.
- **Self-development** — the computer improving its own harness: models
  propose changes to the code running them, the changes are tested,
  checkpointed, and promoted; restore can always take them back.
- **World Wire** — the automatic newspaper (§1): the destination application
  that makes continuous autonomy non-negotiable.
- **Heterogeneous models** — running different providers/model families for
  the same roles so that no single model's failure mode (correlated
  hallucination, provider outage) takes the whole system down. Standard term
  (model diversity).
- **OOD (out-of-distribution)** — the deployment environment differs from
  the models' training environment. Here, deliberate (§7, D3).

## 4. What the harness supplies

Four things — exactly what models cannot supply for themselves, nothing more.
Each item states why models cannot do it, because that is the only valid
reason for harness machinery to exist.

**4.1 Durable state.** Models are stateless per call. The harness keeps the
event log, receipts, artifacts, and continuity across restarts, plus
identity (who acted — attributable receipts) and time (timers, wake-ups,
deadlines — a model cannot wake itself up).

**4.2 Effect boundaries.** Models hold no authority of their own; the
harness makes their actions attributable and revocable through capability
gates. **The gate's job is recording and revocation, not rationing.** The
test for a justified gate: does it prevent an *irreversible* error (money
sent, mail delivered, production data destroyed), or make a reversible one
*recoverable* (restore)? A gate that merely pauses work "for review" is a
stop, and stops are what this harness is built not to do. Gates that fire
should let the system continue on another path (fail-over), not idle.

**4.3 Physical backstops.** Goroutines cannot be force-killed in-language;
the machine cannot be oversubscribed without OS limits. The harness enforces
capsule death and resource ceilings at the OS level. These are physics
analogues, not supervision: they act on resource exhaustion, never on
semantic judgments.

**4.4 Multiplexed grounding.** The machine accepts concurrent input from any
source — owner (Texture UI), external agents (CLI/API), feeds (RSS, social)
— as one ordered event stream. Human input is first-class but never
privileged as a blocking gate: it is handled in stream order like everything
else. The exceptions are the irreversibles from 4.2, which are gates on
*effects*, not on *input*.

Nothing else. In particular the harness does **not** supply: supervision
hierarchies with authority to halt, constitutional review bodies, consensus
requirers, or "human-in-the-loop" checkpoints. Where v1 had these, §6
records the cuts.

## 5. Redundancy is for autonomy

Why does one model in one loop need capsules, multiple VMs, and heterogeneous
models? For the newspaper, not for control:

- **Capsules** isolate the blast radius of a bad autonomous edit — the
  machine keeps running while one sandbox fails. Continuation.
- **Multiple VMs** survive guest crashes, bad deploys (one VM keeps the old
  build), and provider-side failures. Continuation.
- **Heterogeneous models** survive correlated failure: when one model family
  hallucinates or a provider goes down, a different family takes the shift.
  Continuation.
- **Checkpoint/restore** makes any failure a rewind instead of a halt.
  Continuation.

Every redundancy mechanism must answer: *"when this fires, what keeps
running?"* If the answer is "nothing until a human decides," the mechanism
is mis-designed. Fail-over, not escalation, is the default response to
failure.

## 6. Ontology audit — what v1 had, and what happened to it

v1 (`optimal-harness-thesis-2026-08-31.md`) was produced by a multi-agent
consensus process and accreted panel vocabulary. Audited against §2's
baseline, the dispositions:

| v1 concept | Disposition | Why |
|---|---|---|
| Three constitutional regimes (physics kernel / charter / ordinances) | **Cut.** | A constitution is machinery for a polity; this is one computer with one owner. Owner-ratified rules live in the repo docs and the Definition, reviewed like any other change. |
| Charter, charter-class claims, charter ratification steps | **Cut.** | Same reason; also removed a plan step (bootstrap charter before M5) — sequencing now has no ratification ceremony. |
| Ordinances (learned protocol rules) | **Cut as a term.** The underlying idea — models may tune loop parameters — stays as ordinary self-development: parameters are code, code is proposed, tested, restore-able. |
| Offices, incomparabilities (authority lattice) | **Cut.** | Replaced by the three-role structure that actually exists (§3: Super, CoSuper, Texture agent) with its real, already-implemented authority facts; no new vocabulary. |
| Overturning-condition receipts (graded trust classes) | **Cut as machinery.** The useful kernel — record what evidence would falsify a decision — is a doc/Definition practice, not a subsystem. |
| External witnesses as admission gate | **Kept, re-derived simply.** A claim about the world is settled by running the thing (a test, a build, a deployed proof), not by panel review. That is ordinary verification; no "witness" ontology. |
| Agentic supervision with contestable decisions | **Cut as a subsystem.** Supervision bugs are found by the same process as code bugs: evidence, revert, repair. Restore is the supervisor. |
| Equal footing / trust-accrual / blinding rules | **Cut.** Panel-procedure vocabulary with no load-bearing role in one machine's operation. |
| Floor guards / static caps as "gauges" | **Kept** as plain resource limits (4.3) — no new epistemology. |

**Audit principle going forward:** a concept may be introduced only if
(a) no standard term covers it, (b) it does load-bearing work in §4, and
(c) it can be exercised, restored, or deleted like everything else. Concepts
failing this enter the renormalization list (§7, D4) instead of the design.

## 7. Decisions recorded in this version

- **D1 — Continuous autonomy is the acceptance criterion.** Any mechanism
  whose exercise requires the system to stop and wait for a human is
  presumed defective; it must fail over or restore instead. Human input is
  multiplexed stream input, never a blocking gate (exceptions: §4.2
  irreversibles).
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
  continuing a training simulation. We do not chase familiarity.
- **D4 — Terminology renormalization (future task).** Sweep all Choir docs
  and code for non-standard terms and either adopt the standard term or
  record why the bespoke one stays. Known instance: "tape" → event log /
  journal (§3). Others to be enumerated in the sweep, not invented now.
- **D5 — ReAct + restore is the baseline every mechanism is judged against**
  (§2). New machinery must beat the baseline or be deleted.

## 8. Path forward (simplified from v1)

1. **Land candidate A** — the supervised self-development gate on the
   existing path; effects OFF; fence checkpoint untouched.
2. **Substrate repairs on the A-path** — occurrence identity, predicate
   family, dead-letter handling, remaining scan-cutover waves.
3. **Wave-1 deletions** (~5,000 LOC verified) in parallel — dead weight off
   the execution path.
4. **RLM M1–M4** — session interpreter, prebound context, gated model calls,
   role manifests — buildable without touching the A-path.
5. **M5 parity, then M6.5 nested activations** — proven on the
   consensus-panel workload inside one sealed assignment. (No charter
   prerequisite: §4.2's rules are already the doctrine.)
6. **Full-RLM cutover** — delete the ambient tool surface only after A's
   gate and parity both pass; forced-death and different-model recovery
   acceptance before staging.
7. **Then the newspaper**: feeds in, formats out, continuously — the
   application that keeps the machine honest about autonomy.

Human involvement in 1–6 is the ordinary owner role: ratifying the mission
Definition, reading evidence, and exercising restore — through the stream,
not as a gate inside it.

## 9. What remains genuinely open

Held questions, not blocked ones — the standard explanation for each is that
model intelligence, given §4's substrate, is expected to solve them, and the
harness is judged by whether it forecloses those solutions:

- **The supervision fixed point** — a supervisor that is itself a model can
  fail like any model. Current answer: restore + heterogeneous models +
  fail-over; no supervisory hierarchy. If that proves insufficient, the
  repair must still pass D1/D5.
- **Correlated model error** — heterogeneous models reduce, not eliminate,
  shared blind spots. Current answer: settlement by execution (run the
  thing) rather than by review.
- **Efficiency vs legibility** — per-call capability refinement vs one
  legible audit trail. Current answer: prefer legibility; refine only where
  cost forces it, and record the refinement in the receipt.

Each is revisited on evidence from the operating machine, not by adding
ontology.
