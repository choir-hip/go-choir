Working...
## 1. The frame I reject

**The document treats the fabric as infrastructure *beneath* the computers, and therefore as entitled to ordinary mutable control state.** Everything named in the brief is a consequence of that one assumption:

| Assumed | What it secretly is | Inversion |
|---|---|---|
| a **site** is a machine with one principal (§1.1 `operator`) | a *tenancy*: a machine hosts several principals' admissions | record per `(machine, principal)`, not per machine; "A's VM on B's workstation" is the general case, not a grant exception (law: extraterritoriality; physics: frequency-division multiplexing) |
| a **home** is a place with authority (§0 rule 2) | a *witness* plus a derived view | identity is self-certifying; "home" is a coordinate chart, not a location (Certificate Transparency; frame independence) |
| a **lease** is a promise about the future, held in a row (§2.5) | a *monotone epoch proof* anchored in signed receipts | the doc half-says this itself (W6); make it explicit or delete W6 |
| **one writer per branch** is ontology (branching tape §10) | *deterministic totalization*; single-writer is the cheapest totalizer | the invariant is "one reduction"; one replisome per fork, thousands of forks per genome (DNA replication) |
| **placement is a management-desk decision** (§1.2) | a *bid* plus a *local admission predicate* | sites are the kernel; the desk is tunable policy (immune self/non-self is local, not centrally scheduled) |
| **rings** are levels of trust widened by evidence (§5.3) | a *bounded exposure schedule* — a bond | gate on exposure, display the ring (law: status → contract; surety, escrow, limited liability) |

The tell is internal, not philosophical. §2.5 says the lease has no expiry and the home's row decides who writes; W6 and branching-tape §12 case 6 say epochs are anchored in receipts and on restore "the home takes the highest epoch it has seen." Both cannot be the authority. W6 is the *old data system's* failure — a mutable row restored from backup silently changes who may write — reappearing one layer up. The document spends its length ensuring a computer cannot equivocate about its own history, then lets the layer that decides *who writes that history* equivocate. §1.1 confirms the split: site records are "platform control state… not a computer's data," a second authority beside "the tape is the authority for state" (§9 Q5).

**Imported frame:** the doc builds a *sovereign* (the home's control DB) with discretionary acts — lease grant, placement, ring widening — that are on no tape and can roll back, while the computers live under a *constitution* (the tape). The reframe is rule of law: the sovereign's acts get written, signed, and bounded by the same constitution. Not because it is elegant — because the failure class is already documented.

## 2. Reframed question

Not "how is work placed on sites and writers moved between homes," but:

> **Which fabric facts are constitutional — durable, signed, and restorable without changing anyone's rights — and which are telemetry? Give the constitutional ones the same append-only, epoch-monotone treatment as a computer's tape, sharded by authority domain (owner registry, org registry, site), never one global log; leave telemetry off-tape.**

Criterion the doc lacks: *if restoring a fact from an old backup could change who may write, what may fire, or who may see, it is constitutional. Otherwise it is telemetry.* Then the kernel is five objects — owner key, signed append-only log, monotone epoch, local admission predicate, exit — and sites, homes, desks and rings are views and policies over that algebra. Two rules fall out:

- **Local power may only subtract.** A site/tenancy/desk may refuse or narrow without consulting anyone (refusal is an answer, §1.2). Any power that *grants* — write exclusivity, irreversible effect, capability widening — must be a proof or the one linearizer.
- **One linearizer survives.** Under partition, ordering needs a decider (FLP/CAP); the kernel keeps exactly one per branch for *epoch grants only*. All other authority becomes checkable.

## 3. What changes in practice

- **Q1:** remove "lease" from kernel vocabulary. Kernel text: *a branch head is the unique reduction of signed appends with monotone epochs.* Home = epoch sequencer; its state is `max(epoch)` over receipts. Acceptance test: destroy the home, rebuild from tapes alone — no consensus, no rights changed. Branching-tape §13 Q3 ("can the home authority move") becomes yes by construction.
- **Q2:** split identity / ordering / discovery. Reads are signed heads served by any valid holder; "home always reachable" must become a stated replication property (k copies), not an adjective. Multiple homes = a witness-set change, a policy event, not a new mechanism. Phone resolution is name resolution, not routing.
- **Q3/Q8:** grants are events on the *granter's* registry tape (they are already owner acts, §1.2), not home rows. **Exit becomes a kernel invariant:** the bundle and keys always leave; org policy and site admission may only subtract. Offboarding (§10.9) becomes a theorem rather than a default.
- **Q4:** each site/tenancy enforces a monotone admission predicate; placement is tunable policy (hence self-developable); no placement database. The five-attribute filter (§1.2) should be expressible as a refusal-composable predicate.
- **Q5:** at-most-once = "the effect fires only on a prefix of the total order after the precommitment." The outbox is a projection of the tape, not a second truth; the effects digest is the cross-branch check.
- **Q6:** no change — the controller grant is already two tapes, i.e. already correct under this frame.
- **Q7:** name the real invariant: **the signing key is never in the capability process.** "Host, never VM" (§0 rule 1, §2.1) is a deployment claim; a native app driving the screen must not be able to sign a head.
- **Q9:** the verified object becomes one log schema plus one predicate algebra; every org topology is a model of it — protocol safety, not topology safety. That is the small kernel.
- **Q10:** self-development tunes placement, exposure and discovery; it never touches log, epoch, or exit.

## 4. Sharpest objection to my own reframe

**I am rebuilding the fabric as a second data system to avoid a second data system.** The doc's virtue is that it is mostly existing mechanisms plus policy; "registry tapes and a predicate algebra" is more machinery, and the owner asked for the simplest kernel, not the most principled one. Half of what I want taped must not be: `free_now`, heartbeats, battery state change every few seconds — taping that is write amplification and a resource regression that the repo's own "resource use ahead of uptime" rule would reject. My restore-unsafe criterion is the answer, but it is a *new* rule the doc does not yet carry, and if it is applied loosely the fabric's log becomes the telemetry dump.

Second objection: "single writer is a mechanism, not ontology" loosens exactly the degree of freedom self-development must not explore — wrong-branch restore and ancestry-blind idempotency are *silent* (branching-tape §12). Concede the write path: keep it ontological. Relax only placement, exposure and discovery, where failure is visible. So the reframe is narrower than "authority is a proof, not a post": proofs govern grants, reads and exit; the one linearizer stays for write admission. And a witness-set home costs follower copies at every witness — disk the one-tape work bounded — so the honest version is "home is a logical role over the org's replicated store, and the doc must state the replication and the read proof," not a new protocol.
