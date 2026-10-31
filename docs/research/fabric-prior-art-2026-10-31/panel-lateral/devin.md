Warning: --sandbox always uses the autonomous permission mode; ignoring --permission-mode dangerous.
Let me read the rest of the resource fabric doc, especially §10 and the sections on children, capabilities, orgs, and mechanism/policy.Let me finish the tail of the branching-tape doc before writing.# Lateral panel response — resource fabric brief

## The frame I reject

**The whole brief is written in datacenter vocabulary.** "Sites advertise capacity, work is placed, one machine holds the writer lease, the home is always reachable" — this is the mental model of a cloud scheduler (regions, primaries, lock managers, failover). It quietly decides the answers before the research starts: the server is the natural center, workstations are *checkout exceptions*, personal machines are *underutilized capacity*, conflicts are *failure modes*, allocation needs *a scheduler*.

But the product is not a datacenter. It is **a person's persistent estate, operated by delegated agents, sitting partly on land other people own.** The distant domain that already solved every question in the brief is not distributed systems — it is **property and agency law**, with biology and economics as the supporting cast:

| Brief question | Legal/institutional/biological mechanism that already answers it |
|---|---|
| Q1 writer authority, offline work, leases | **Agency law**: power of attorney, acts by an unreachable agent are *ratified* later by the principal; contested claims go to a court of record |
| Q2 reachability, "home" | **Corporate law**: domicile (which law governs) ≠ registered agent (where you can always be served) ≠ presence (where you happen to be). Three functions, deliberately separated |
| Q3 access control across persons/orgs/sites | **Property law**: B's workstation is B's *land*; A's work enters only under an easement (the signed grant). An org is a *jurisdiction*, not a bigger person |
| Q4 placement | **Coase's theory of the firm**: the boundary between what you employ (capsules, children) and what you contract (site leases) is set by transaction costs — and the doc already invented a currency (compute credits) while keeping a planning board |
| Q5 irreversible effects | **Thermodynamics**: external effects are entropy production. The precommitment gate is the boundary between the reversible interior (tape, branches, replay) and the irreversible exterior — the design rule is "maximize the reversible zone" |
| Q6 computer controlling computers | **Fiduciary license**: delegation with duties, liability caps (budget), anti-self-dealing (no cycles), sovereignty retained (revocation) |
| Q8 offboarding | **Accession/confusion doctrine + emigration law**: who owns a thing made of mixed inputs is a 2,000-year-old solved problem; "employee keeps the computer, org keeps published objects" *is* the accession answer |
| Q9/Q10 safe configuration space, self-tuning | **Immune system, not prophylaxis**: you cannot prove all configurations safe; you prove detection, attribution, isolation, cheap rollback. Equivocation proofs, taint classes, branches and fences *already are* immune mechanisms |

## The reframed question

Not: *"which machine writes the tape and how do we route to it"* — but: **"who holds delegated authority over the estate, how are an agent's acts ratified, and what court adjudicates contested claims."**

Once said that way, several of the named assumptions dissolve or invert:

1. **"The lease" is actually a power of attorney, and "does it expire" is the wrong question.** A PoA doesn't need a TTL; it needs revocability and a forum. Offline writing isn't an anomaly the design survives — it's an agent acting while unreachable, whose appends are **ratified** by the home's ack (the doc already built this and called it "home-durable") or *contested* (adjudicated as branches, with signed equivocation as provable breach of loyalty). The right parameter isn't lease expiry — it's **revalidation cadence**.

2. **"Home" fuses three things law deliberately separates.** Custody (durable copies — plural, content-addressed, already exists), jurisdiction (the epoch authority — must stay singular), and rendezvous (how a phone finds the writer — pure replaceable infrastructure, the MX record of the system). Splitting them makes "multiple homes" stop being scary: custody and rendezvous replicate freely; only jurisdiction stays singular — and even that can re-domicile later (branching-tape open question 3). It also exposes a standing-questions violation: the lease row at the home is a **second state authority beside the tape** (SQ 5). Named honestly, the home is a *court of record and registrar* — the tape is still the estate's title, and lease state should ideally be *derivable from the highest signed epoch receipts* rather than a parallel database.

3. **"One writer per branch" isn't a concurrency mechanism — it's a narrative requirement.** The tape exists so the owner has *one legible account*. A ledger's single writer is like a court's single record: correctness is almost incidental; defensibility is the point. Named this way, failover-by-branch stops being an emergency measure and becomes the system's native posture (two narratives, adjudicated), and pessimistic fencing is demoted to best-effort hygiene under provable detection.

4. **"The computer stays on" conflates germline and soma.** Biology's deepest trick is separating the immortal line (genome) from disposable bodies (soma). Tape + name + policy are the germline; every writer realization is already disposable soma. Push it one step: the writer is an *ephemeral officer claiming per-episode authority* — then there is no incumbent to fail, "checkout" becomes the universal mode rather than a special one, sleep is just between-sessions, and Q1's failover problem partially dissolves.

5. **"Sites" imports a utilization ethic onto people's dwellings.** A laptop is not stranded capital; it's a house with spare rooms. The doc's ordering bakes the datacenter default into every section — home = Node B, workstation = the exception called "checkout." Invert the *default*: a personal computer's writer lives where its person lives; the server is the land-registry and archive. Modes E/L already exist — this is a defaults change, but defaults are exactly where frames hide.

6. **"Rings" assume trust is spatial containment.** The alternative legal/economic model is *earned certification*: a driver's license is granted on demonstrated competence. The precommitment calibration records already *are* the transcript — ring-advancement eligibility becomes computable from the tape (N calibrated uses, zero violations → owner prompted to widen). That is precisely the self-tuning-within-proven-bounds mechanism Q10 asks for; rings stay the grant, evidence becomes the input.

7. **"Phones are clients, not sites" is a policy dressed as ontology.** A phone has Neural Engine, 8 GB, keys in hardware. "Client = a device that enrolls no host" is the derived rule; declaring the taxonomy fixed closes the future where a phone hosts a writer for fully-offline work.

## What the reframe changes in practice

- **Q1**: stop designing a lock manager; design *delegated agency + ratification*. Lease rows become attorney grants scoped (computer, branch); offline appends are drafts-until-ratified; expiry questions become revalidation-cadence questions.
- **Q2**: split "home" into custody / jurisdiction / rendezvous. Home failover becomes "the registry has mirrors" — cheap. Phone reachability becomes name-resolution over pluggable relays.
- **Q3/Q8**: grants are easements; the org is a jurisdiction; offboarding is emigration with accession-style property division — including *liens over the tape* for embedded org data, which the clean-separation default can't express today.
- **Q4**: the kernel is the market mechanism (offers, receipts, admission = reserve price); desks are brokers; budgets and bid curves are policy — the mechanism/policy split Q9 wants, with the compute-credit currency already invented.
- **Q9**: restate the provable kernel property from "every configuration is safe" to "every configuration fails *detectably, attributably, recoverably*" — which is what drills already verify and what the doc actually builds.
- **Q10**: trust tuning becomes a tape-computed function inside owner-set bounds rather than a new mechanism.

## Sharpest objection to my own reframe

**Law presumes a court; this system has none in real time.** Agency and property doctrine tolerate ambiguity, delay, and ex-post adjudication because a standing coercive forum exists. Fencing must be decided in milliseconds with no appeal — the epoch check in the CAS path is doing work no metaphor can do. Worse: every reframe that *removes* the singular home (derived lease state, plural jurisdiction) reintroduces a consensus problem, because the tape is the very thing being contended — a title registry can't be derived from a disputed deed. The centralization at the home may be the load-bearing simplification of the whole design, not a hidden assumption at all.

Similarly: markets presume liquidity (at n=20 a planning desk beats a thin market, and compute credits get Goodharted instantly); ephemeral writers presume a watcher (standing intents need something awake — the home again); partial causal order is the merge they already rejected (the linear tape is a *product feature* — the owner reads it).

So the honest version of my own reframe is smaller than it looks: **change the vocabulary and the defaults, not the architecture.** Keep the epoch check, keep one jurisdiction per computer, keep the desk placing work — but name the home a *registrar, not a lock manager*; name the writer an *agent-in-fact, not a primary*; default the writer to the person's own device; make the tape the transcript that computes trust and adjudicates claims. The doc is already building a legal order and calling it a cloud.
