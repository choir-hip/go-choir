# The Optimal Harness Thesis

*Choir platform theory, 2026-08-31. Three consensus rounds (divergent, lateral,
convergent) across 11-agent panels including Claude, grounded in the
persistent-RLM memo, the computer ontology, and three code audits. Epistemic
status: a logical architecture argument, deliberately not a benchmark result —
model churn makes eval-optimization self-defeating; the invariants below
reference no specific model.*

---

## 1. The Principle

**The harness is not built to solve anything. It is built to let the models
solve everything, being as dependent on models as possible but no more, while
facilitating multiplayer human and external-agent interaction.**

The harness supplies exactly four things models cannot supply for themselves:

1. **Durable substrate** — an event-sourced tape, receipts, artifacts, and
   continuity across restarts, including identity/attribution
   (cryptographic agent identity — attributable challenges depend on it) and
   durable time (timers, wake-ups, deadlines — models cannot wake
   themselves) (models are stateless per call).
2. **Effect authority** — typed capability gates that make actions ownable,
   revocable, and attributable, with trusted mediation and information
   boundaries (credential confinement, declassification, admission
   control) (models hold no authority; they propose).
3. **Physical backstops** — OS-level forced death and machine resource
   ceilings (goroutines cannot be killed; the machine cannot be oversubscribed).
4. **Multiplayer grounding** — owner authority, external agents, and the
   external world as verification sources (the escape from pure self-reference).

Where a question mark exists (the supervision fixed point, herding dynamics,
efficiency-legibility tradeoffs), the standard explanation applies: **model
intelligence is expected to solve it, and the harness is judged by whether it
forecloses that solution.** A harness constraint is justified only if model
intelligence cannot, in principle, supply the guarantee it provides.

## 2. Layered concurrency: properties attached by relationship, not a nesting stack

One Go binary per VM hosts many capability-scoped capsules, each hosting many
per-activation interpreters, each running many goroutines. But the layers are
**deployment topology, not authority topology**, and four isolation axes are
attached independently, per relationship:

- **Capability** lives on the effect/gate-call (unforgeable tickets,
  revocable leases with epochs and rollback contracts), evaluated where
  messages cross actor boundaries — not in container manifests. Authority
  is held as **incomparable offices** (Super and Texture hold different,
  non-nested powers per the ontology), not a parent/child lattice. Capsules
  are effect chambers, blast radius, and accounting domains — never principals.
- **Preemption** (park/kill) attaches at the cheapest boundary containing the
  blast radius — interpreter step-fuel, cgroup freeze, process group, VM stop.
  "Process = death" is a Go accident, not a law.
- **Durability** lives on the event chain and artifact store, never on a
  process or VM lifetime.
- **Ownership** lives on ComputerID + owner identity + the tape, never on the
  hypervisor.

Efficiency principle: push each concurrency down to the cheapest layer
satisfying the isolation the *relationship* requires. The expensive things are
boundary crossings — context re-stuffing across agent boundaries, serialization
across process/VM boundaries, cold starts. Inside one guest, inter-agent
messages are content-addressed refs; networking is memory-speed. Cross-VM
traffic is the expensive, receipted, durable kind, reserved for real
computer-to-computer semantics (publish, install, consensus handoff).

**Held tension (legibility vs efficiency):** fine-grained, per-call capability
algebra destroyed Java's SecurityManager by making composed authority
unauditable; coarse bundles waste isolation. Default: coarse bundle at the
capsule, per-call refinement only where the charter grants it, and charter-
mandated legibility defaults (every capability decision appears in exactly one
legible place). Model intelligence is expected to manage the composition;
the harness must keep it *auditable*, not keep it simple.

## 3. Full computational expressivity, gated effects

Model-authored Go is the sole model-facing orchestration surface inside an
activation — trusted reducers, brokers, and typed reducers remain the actual
actuators behind the gates, and doctrine-permitted direct Bash is another
front end to the same capsule broker: orchestration, control flow, data
transformation, and inter-agent communication are written as code, not
enumerated as JSON tool calls — code is closed under composition;
tool DSLs are not. Effects pass exclusively through typed capability gates;
every admitted effect **and every refusal** is receipted, with
ordinance-level aggregation bounds on refusal volume (a cost sink and a
gameable side channel if unbounded). The dual constraint
is deliberate: full expressivity for computation, narrow authority for action.

Model churn note: the *effective* action space is what models reliably
generate, which changes across releases. The harness therefore fixes the
*authority surface* (small, stable) and leaves the *expression surface* (large,
evolving) to the models — the authority/expressivity split is what makes the
design model-agnostic.

## 4. Three constitutional regimes

The anti-herding and supervision rules split by amendment rate — the
constitutional insight from the lateral round ("the legislature must not own
the amendment rule"; germline vs somatic adaptation):

1. **Physics kernel** (never learned, never negotiated): machine resources,
   provider hard-fails, owner authority, OS forced-death. Protects the
   machine and grounds the system in the external world.
2. **Charter** (amendable only by owner-ratified conjecture delta — harder
   than ordinary deliberation, deliberately): the offices and their
   incomparabilities, independence-domain requirements, the rule that model
   consensus is never canonical authority, supervision-decision contestability,
   and the amendment rule itself. The owner is the only seat that cannot be
   herded from inside; the charter is how that seat scales.
3. **Ordinances** (learned by dynamic orchestration, revised through the same
   receipted/fenced self-development process as everything else): rounds,
   visibility patterns, pairing and roles, transform selection, reveal timing,
   synthesize-vs-preserve thresholds.

Static budgets (steps, tokens, wallclock, dollars) are inductive biases that
**inform** — gauges in prebound context — and serve only as transitional floor
guards. They are not the primary preventer of pathology. Supervision is
agentic: lifecycle operations (park, kill, restructure, reparent, reallocate)
are typed module calls with authority rules; pathology (loops, deadlocks,
non-progress, herding) is detected semantically in receipt traces. Supervision
decisions are themselves evidence-gated and contestable — exercised by fiat,
they are the regress. **Held tension (supervision fixed point):** supervisors
are model-driven and inherit the failure modes they police; the design does
not solve this — it grounds it (physics kernel + owner charter + contestable
decisions + distributed detection with standing for any agent to flag any
trace) and expects model intelligence + owner ratification to handle the rest.
Static caps remain as the innate-immunity complement: never retired, never the
primary control.

## 5. Deliberation protocol: invariants, learned parameters, and earned trust

**Invariants** (derivable, model-agnostic — theorems about why herding happens):

- Typed deliberation moves (agree / object-with-evidence / transform), never
  free chat; equal move budgets; equal footing regardless of model identity.
- Evidence-gated convergence: a dissent resolves only via an artifact the
  dissenter can independently verify, or a transform that genuinely changes
  their representation — never by vote counts. Agreement is not evidence.
- Calibrated pluralism: deliberation terminates in round-bounded synthesis
  that preserves dissent; a minority warning is a legitimate output, not a
  failure to converge.
- Provenance is evidence. Blinding (content-only exposure) is a reversible
  experimental intervention against anchoring, applied where order effects
  dominate — not a universal. Removing status is useful; removing provenance
  can make correlated error look like independent confirmation.
- Model-policy diversity is the primary decorrelator — purchased in manifests
  (different providers/tiers), not by duplicating harnesses.

**Learned parameters** (ordinances): rounds, visibility patterns, pairing,
transform selection, reveal timing. Herding signatures — early consensus
without new evidence, minority views dying unengaged — are detectable in the
receipted trace, and countering them is an orchestration action. Which
patterns work cannot be known a priori; the population of models changes
monthly; the protocol learns.

**Trust is appellate, not procedural.** Procedurally immaculate deliberation
among correlated priors still produces confidently wrong consensus. Every
consensus output carries a mandatory **overturning-condition receipt** — a
machine-checkable statement of what evidence would reverse it — with standing
for any future agent to reopen it by producing that evidence. Trust accrues by
surviving named, attributable challenges and consequential use. A consensus
that names no overturning condition is unfalsifiable and gets the lowest
trust class. And the binding constraint is **external witnesses**: consensus
outputs are instruments, not warrants — they earn belief only where their
claims remain vulnerable to a world that can prove the entire panel wrong
(test execution, measurement, cryptographic verification, the owner).

## 6. Recursion semantics

Three primitives, never conflated: **bare call** (models.Call — stateless
inference, no execution, no authority), **nested activation** (sub-model + own
interpreter + own scoped manifest, running to a typed outcome — budget-carved,
receipt-linked into the parent graph, monotone-downgraded), **actor spawn**
(durable, supervised, policy-gated). Cheap recursion = bare calls bounded by
gates; mid-cost orchestration = nested activations with host-enforced manifest
downgrade; expensive recursion = durable spawns bounded by role policy. Depth
caps and forced-death apply at the containing worker; prompt-escalation is
structurally impossible (authority comes from host-signed manifests, never
prompt content). The agentic-consensus pattern becomes: independent nested
reviews (sealed, committed) → cross-review restricted to disagreements →
typed-move deliberation with transform injection → synthesis preserving
dissent → overturning conditions attached — orchestrated as code by a parent
activation, replacing the external CLI panel for routine volume.

## 7. What the harness deliberately does not do

It does not guarantee convergence, truth, or termination beyond the physics
kernel. It does not solve the supervision fixed point, prevent all herding, or
select the best model. It makes all of these *observable, attributable,
contestable, revisable, and survivable* — and it makes model intelligence
load-bearing everywhere else. The failure mode it categorically refuses is
the one where the harness's static structure forecloses a solution models
could have found.

## 8. Path forward

1. **Land candidate A** (the supervised self-development gate) on the existing
   path; effects OFF; fence 99949fe2 untouched.
2. **Substrate repairs on the A-path**: generation-stamped occurrence
   identity, named predicate family (Terminal/Active/Replaced), dead-letter
   for unknown actor kinds, remaining scan-cutover waves.
3. **Wave-1 deletions** (~5,000 LOC verified) in parallel — disjoint from the
   execution path.
4. **RLM M1–M4**: session interpreter, prebound context, gated models.Call,
   role manifests — buildable without touching the A-path.
5. **Bootstrap charter ratified by the owner BEFORE M5/M6.5**: minimal
   charter — all consensus outputs are recommend-only and the owner is the
   sole effect authorizer until the full charter lands; unclassified rules
   default to ordinances and cannot bind supervision; floor-guard values get
   a named owner-ratified change path (or they become an accidental fourth
   regime). Steps downstream of M4 must not run on an unratified implicit
   charter.
6. **M5 parity**, then **M6.5 nested activations** proven on the consensus-
   panel workload inside one sealed assignment, with the overturning-
   condition receipt mechanism landing with or before M6.5 (the consensus
   workload is its natural first consumer).
7. **Full-RLM cutover** (delete ambient tools) only after A's gate and parity
   both pass; **M6** forced-death/different-model recovery acceptance before
   staging; effects-ON requires the ratified charter's effect-authorization
   rule, not just parity.
8. **Full charter codification**: the owner ratifies the charter regime
   (offices, independence domains, consensus-is-not-authority, amendment
   rule) as a durable Definition — the first exercise of the constitutional
   mechanism.
9. **Ordinance learning begins** only after the substrate is self-developing:
   the harness tunes its own protocol through the same proof pattern it runs
   on code.

## 9. Convergent adjudication (final panel, 2026-08-31)

Ten-panelist convergent round (including Claude) on this document: all
pillars **sound or sound-with-repair**; the Principle is coherent *because it
is falsifiable as a design rule* ("a harness constraint is justified only if
model intelligence cannot, in principle, supply the guarantee" converts every
future architecture argument into a checkable question). Repairs folded
above: identity/attribution and durable time named as harness-supplied;
trusted mediation/information boundaries added to effect authority;
"sole actuator" narrowed to "sole model-facing orchestration surface";
refusal-receipt aggregation bounded; unclassified rules default to
ordinances; floor-guard values given an owner-ratified change path;
equal footing scoped to *within* deliberation while earned trust affects
*selection into* panels, never weight inside them; overturning receipts
graded (machine-checkable / named-observable / owner-judgment) instead of
all-or-nothing; bootstrap charter inserted before M5/M6.5.

**Strongest surviving objection** (what a future failure report would cite,
converged from two panelists): *the foreclosure test is one-way.*
Over-harnessing is detectable — a model solution was blocked. **Under-
harnessing of charter-class, world-uncheckable claims is not**: physics
catches exhaustion, effect gates catch unauthorized action, but nothing
catches a receipted, overturning-condition-bearing consensus that is simply
wrong, produced by a correlated model population, with ordinances learned by
the same population they police, and overturning conditions written by the
panel they constrain. Owner attention is the scarce resource the system
exists to amplify. The repair that dissolves the objection without violating
the Principle: for **canonical writes and charter-class claims, external
witness is an admission gate** (owner ratification, test execution,
measurement, cryptographic verification) — not merely a trust-class comment.
That is multiplayer grounding, not the harness solving cognition. It is now
a charter-level requirement.

Panel outputs: `.agentic-consensus/agentic-consensus-20260831-102236/`
(prior rounds: divergent `...-080155`, lateral `...-085319`, both 2026-08-31
directory timestamps notwithstanding the 08-29 working session).

## 10. Open unknowns brought back to the owner

- **Charter content**: which offices, which incomparabilities, and what the
  owner-ratified amendment rule literally says — owner decisions, not
  derivations.
- **Supervision authority allocation**: who may park/kill whom (peer mutual
  authority vs supervisor hierarchy) — charter-level.
- **Consensus-for-effects rule**: exactly when nested-activation consensus is
  allowed to authorize an effect vs only recommend — charter-level.
- **The efficiency-legibility default**: where per-call capability refinement
  is permitted — charter-level default, tunable later.


