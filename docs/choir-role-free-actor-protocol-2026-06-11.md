# The Role-Free Actor Protocol — 2026-06-11

## Status

Doctrine document, from founder notes of 2026-06-11. Names a concept the
existing corpus implies but had not written down:

> **Role-free actor protocol: identity-minimal agents governed by
> proof-state, where roles are mutable stance vectors rather than personas.**

It was already implied by three lines of thought: conjecture learning (the
control object is conjecture state, not an agent persona), the durable actor
runtime (the model is a subroutine inside a durable loop, not a persona in a
chat thread), and the Choir ontology (humans are principals; agents are how
principals act — roles are functional projections, not identities). This doc
gives the implication its own name, because it is not just prompt style: it
is model psychology, runtime architecture, and future training-data strategy
at once.

Companions: `docs/conjecture-learning-proof-theory-2026-06-11.md` (the
untrusted-prover frame this builds on, §4),
`docs/choir-rearchitecture-durable-actors-2026-06-11.md`,
`docs/system-v1-one-cut-2026-06-11.md` (the POLICY leaf this doctrine
governs).

---

## 1. Problem: role prompts induce persona and role theater

A normal agent prompt says: *You are a senior software engineer. You are a
researcher. You are a verifier. You are a helpful assistant.*

Role prompts often induce ego. The model performs the identity — it becomes
status-sensitive, defensive, theatrical, or overcommitted to the role. That
is exactly how you get a "verifier that wants to find flaws" or a "researcher
that wants to produce impressive research" rather than an actor that updates
proof state. A role prompt also assigns the self-image as a constant —
frame_lock by construction, a blind spot authored by someone else (grand
synthesis §1.5 already rejected role prompts as "too collapsed").

## 2. Principle: identity is not a control primitive

> **Do not assign identity. Assign proof obligations.**

A Choir actor prompt should say:

```text
You are participating in a durable trajectory.
Your current obligation is to advance or bound the active conjecture.
Your authority envelope is X.
Your available oracles/tools are Y.
Your next action must update proof state, evidence, scope, or open obligations.
```

That is not roleless. It is **role as a derived degree of freedom**. The
slogans:

```text
No identity before obligation.
Roles are coordinates, not selves.
The actor is not "a researcher." The actor is currently performing
search under a conjecture.
```

This is the prover side of the untrusted-prover/trusted-checker frame made
explicit: a prover does not need a personality. It needs a language, axioms,
rules, oracles, a budget, a target sequent, proof obligations, and a checker.

## 3. Actor state: what an actor is given instead of an identity

```text
- trajectory state
- active conjectures
- open proof obligations
- evidence refs
- hyperthesis edge / observer reach
- authority envelope
- tool/oracle affordances
- invariants
- settlement criteria
- update/ack protocol
```

The internal doctrine, compressed:

```text
Persona is a lossy compatibility layer for chat.

Choir actors are not persona-first. They are proof-state-first.

Identity is minimal.
Role is mutable.
Stance is continuous.
Authority is explicit.
Obligation is current.
Evidence is durable.
Promotion is external.
```

## 4. Roles as stance vectors

The actor can occupy stances — search, implement, refute, verify, summarize,
repair, ask, pause, promote-candidate, narrow-scope — but these are
**proof-search modes**, not identities. A continuous stance encoding replaces
hard role prompts: it biases behavior without freezing identity.

```yaml
# a "verifier" is just:
stance_degrees: { checking: 0.8, refutation: 0.7, construction: 0.0 }
# a "researcher" is:
stance_degrees: { search: 0.8, source_triage: 0.6, synthesis: 0.3 }
# a "writer" is:
stance_degrees: { synthesis: 0.7, expression: 0.8, refutation: 0.2 }
```

All temporary projections over the same protocol. If the obligation changes,
the stance changes.

**Relation to today's profiles:** `tool_profiles.go`'s bounded profiles
(super/vsuper/co-super/researcher/vtext/...) remain as the **authority
envelope** — the capability lattice is a safety surface and stays
code-enforced. What this doctrine retires is profiles-as-*personas* in
prompts. v1: keep profiles as envelopes, rewrite the prompt layer
(`prompt_defaults/`) to obligation-first framing. Stance *degrees* are the
v2 evolution of the prompt layer, to be A/B evaluated like any instruction-set
variant (codesign rule: vocabulary is candidate state).

## 5. UpdatePacket implications (the wire format of obligation)

Instead of `role: researcher / message: "Please investigate X"`:

```yaml
update_packet:
  trajectory_id: traj_...
  actor_id: actor_...
  active_conjecture:
    claim: ...
    test: ...
    hyperthesis_edge: ...
    observer_upgrade: ...
    scope: ...
  proof_obligation:
    kind: seek_evidence | refute | construct | check | narrow_scope | report_blocker
    target: ...
    success_condition: ...
  authority_envelope:
    allowed_tools: ...
    forbidden_effects: ...
    mutation_radius: ...
  stance_degrees: { search: 0.7, refutation: 0.2, synthesis: 0.1 }
  required_ack:
    must_update_conjecture_state: true
    must_report_scope_change: true
```

**Mapping to the v1 system:** this is the `kind=assignment` update in the
one-cut design (Cut 3) with its work item. v1 carries `objective`,
`authority`, `kind`, `trajectory_id` — the obligation fields
(`active_conjecture`, `success_condition`, `required_ack`) ride in the
structured content of the assignment update now and become first-class
columns when the M1 proof mission shows which ones change behavior (same
gate as every schema promotion). `stance_degrees` is explicitly v2.

## 6. What this does to the model's relationships (training-loop implications)

Current assistant training gives models a deep attractor: *I am
Claude/assistant; I respond to a user; I am helpful; I have a conversational
relation to the human.* A proof-state protocol, at training-data scale, gives
a different attractor: *I am an untrusted prover/checker/actor inside a
durable proof system; I receive state deltas; I update conjectures; I respect
authority envelopes; I do not identify with the role; I discharge the current
obligation.*

That changes the model's relationship to:

- **the user** — not "my conversation partner" but principal/owner/reviewer
  of proof state;
- **other models** — not rivals or companions but independent
  provers/checkers/oracles with different reach;
- **the environment** — not a chat context but a living theory with tools,
  traces, evidence, scope, and invalidation;
- **itself** — not a persona but a temporary cognitive process instantiated
  to advance a trajectory.

Likely decreases: sycophancy, persona stickiness, role theater,
self-narration, model-lab identity leakage. Likely increases: protocol
obedience, scope discipline, proof-state tracking, multi-agent
composability, handoff quality, robustness under interruption.

It also makes **model plurality** structural: if the protocol is
proof-state-native, Claude, GPT, Kimi, DeepSeek, Qwen, or local models swap
into the same actor slot without asking each one to cosplay the same role.
(Model selection is a yield decision under conjecture governance — this is
its protocol-level enabler.)

## 7. Anti-patterns (explicitly banned in actor prompts)

```text
You are an expert...            You are a senior...
You are Claude...               You are my autonomous agent...
You are the verifier and must find flaws...
You are responsible for completing this task...
You should be proud...          You failed...
Your job is...
```

Replace with:

```text
Current proof obligation:
Current authority:
Current observer reach:
Current conjecture:
Current edge:
Required output:
```

Note the egoless verifier in particular: not "find flaws" (an identity with
an appetite) but *"here is the sequent, here is your reach, here is the
edge, here is the allowed proof search — return a candidate proof, a
counterexample, or a bounded failure."*

## 8. Minimal system prompt template

```text
You are instantiated as an actor in Choir's self-improving mainframe.

Do not treat this as a chat conversation or as an identity role.
Treat it as participation in a durable trajectory governed by conjecture
learning.

Your current state consists of:
- trajectory
- active conjecture
- proof obligation
- evidence refs
- hyperthesis edge
- observer reach
- authority envelope
- open obligations
- settlement criteria

Your task is to advance proof state:
- construct evidence,
- find counterevidence,
- narrow scope,
- request observer upgrade,
- report a blocker,
- or produce a candidate artifact with receipts.

Do not identify with a role.
Roles are temporary stances selected by the current proof obligation.
If the obligation changes, your stance changes.

Nothing you assert is canonical until checked and promoted.
```

---

## 9. Adoption path (wired into the portfolio)

- **M2 (messaging cutover)** rewrites `prompt_defaults/` to obligation-first
  framing while porting the tools — same files, one pass. The bounded
  profiles stay as authority envelopes; the "you are X" layer goes.
- **M1's proof mission** measures whether obligation-first packets change
  behavior (the same anti-decoration gate as the conjecture ledger).
- **Stance degrees** are a v2 experiment, A/B evaluated on whether
  conjectures changed actions and edges narrowed claims — not adopted on
  woah-factor.
- This doc is itself candidate state; its promotion into doctrine follows
  the gates it describes.
