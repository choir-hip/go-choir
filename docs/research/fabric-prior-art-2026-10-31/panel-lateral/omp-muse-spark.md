Working...
## Lateral panel: the placement frame is the trap

### 1. The frame I reject

Everyone takes for granted: **a computer is a running process that must live somewhere.** From that one sentence the entire apparatus follows as forced moves:

- *sites* = machines that host the process → need enrollment, site records, heartbeats;
- *single writer + leases* = only one home for the process at a time → need epochs, fencing, checkout, handback;
- *home* = the capital city that always knows where the king is → need routing, relays, follower copies;
- *branches* = the shameful exception when the king was in two places → need primary refs, switches, effects digests;
- *management-desk placement* = central planner assigning work to machines;
- *rings R0–R2* = castle walls around the process.

The brief asks Q1–Q10 inside this frame: where does the writer run, how does the phone find it, who schedules what. All ten questions presuppose **computation-placement** is the problem. I reject that. The computer is not a process. It is a **principal**.

### 2. The reframe

**From:** "Where does the computer run, and who writes the next event?"
**To:** "What mandates has this principal issued, and what has settled?"

Import from **agency law + double-entry bookkeeping** (backed by one biology image):

- A **corporation** does not "run" anywhere and has no "single writer per branch." It is a legal person that owns assets and issues **bounded mandates** to officers, employees, subsidiaries acting concurrently worldwide. Authority **attenuates downward** (this card, $500/day, these merchants) and **accounts upward** (every agent owes a journal). Double-spend is stopped not by a lease on the corporation's brain but by the **clearinghouse refusing the second check** and by mandates that cannot cover both spends. Errors are fixed by **contra-entries**, never by rewriting history. Revocation is a **stop-payment**, not a fence.
- The **octopus** keeps 2/3 of its neurons in its arms. Arms act semi-autonomously on local sense + a goal from the brain; the brain does not sequence arm movements. There is no single writer of octopus behavior — only goals out and reports back.

In this frame: the tape is not a process log that one host appends. It is **consolidated books**, settled periodically from journals kept everywhere. Inner thought (drafts, plans, reads, inference) needs no total order and no writer — arms think locally. Only **external commitments** (spend, send, publish, rotate credential) need settlement, and settlement is a small, boring, replaceable clearing function. The precommitment record / outbox — currently a sidecar to the writer — **becomes the kernel**. The writer lease is demoted to what it always was: one implementation of a checkbook.

### 3. What this changes in practice (Q1–Q10 dissolve or invert)

- **Q1 writer/leases/failover → mandates + settlement.** No writer lease, no expiry debate, no checkout. Every site holding the computer's work holds a **mandate**: a macaroon-style attenuated capability (scope, spend/rate bounds, nonce range, expiry *of the mandate*, not of truth). Offline work is not an exception — it is an arm acting inside its mandate. Over-mandate acts queue as proposals. "Two sides active" is normal concurrency; the anomaly is only a **double-commit attempt**, handled like a bounced check: second settlement refused + contra-entry, not a branch + owner switch ceremony. The Mac sleeping is a non-event: its mandate lapses, its journal syncs later, nothing fences.
- **Q2 home/reachability → a person, not a server.** You reach a person via any of their phones, secretaries, published addresses — there is no "capital." Any replica of the public books + any live agent is a valid contact point; rendezvous is introducers/DHT/gossip, not home-relay. The "home" shrinks to **a clearinghouse**: whoever settles commitments this week. Commodity, replaceable, possibly the phone itself for small principals. Home failover stops being a design problem because clearing is idempotent settlement of signed mandates, re-runnable anywhere.
- **Q4 placement → market, not planner.** Management-desk bin-packing (central planning) becomes **offer/mandate matching**: sites post ephemeral offers (cores now, mic here, idle-on-power), mandates flow to capability. The desk sets budgets and goals; it does not schedule. Self-development tunes *bidding and delegation strategy* inside budget bounds — a strictly safer learning surface than tuning a scheduler.
- **Q5 irreversible effects → the kernel, not a constraint on it.** At-most-once via mandate nonces cleared at settlement; "exactly-once" abandoned as fiction, replaced by idempotent settlement + reconciliation (contra-entries). The effects digest (§6 of branching tape) stops being a switch-time apology and becomes the ordinary settlement report.
- **Q3/Q7 access, rings → attenuation, not walls.** Rings R0–R2 (castle model: which room is the process in) become mandate scope (agency model: what may this act do, under whose authority, with what accounting). Site admission ("B's workstation refuses A's VM") is not an ACL checked before placement — it is the absence of a mandate chain from B. Default-deny falls out of attenuation monotonicity: a delegate can never widen its own mandate. One mechanism covers persons, orgs, sites, controllers, and native capabilities.
- **Q6 control without writing → the normal case.** A controller never wanted to write the child's books; it issues mandates and reads published journals. Choir-in-Choir is a parent company and subsidiaries, not VMs puppeteering VMs.
- **Q9 mechanism-vs-policy → the law of agency is the kernel.** Prove once: attenuation-only delegation, nonce-clearing settlement, monotonic books, stop-payment revocation. Then *every* delegation chart an org or computer writes is safe by construction — policy cannot escalate, only narrow. Model-check the clearinghouse (small), not the configuration space (vast).
- **Q8/Q10 lifecycle + self-tuning → charter + accounting.** Onboarding = issuing mandates; offboarding = stop-payments + journal closeout; ownership = who holds the root mandate. Self-development tunes the delegation chart within attenuation bounds — provably unable to grant itself what its principal never had.

Concretely, the "do now" list inverts: the highest-leverage build is not `writer_epoch` in the receipt (branching tape do-now 1) but **mandates + idempotent settlement + journals**, because that is the substrate the next ten years rest on; writer epochs are one clearinghouse's internal sequence numbers.

### 4. Sharpest objection to my own reframe

**Belief is not money, and reconciliation is merge smuggled back in.** Double-entry works because dollars commute and contra-entries restore truth. Agent state does not: the reducer's single pending transition, Texture's canonical revision, queue positions, "is this mail handled" — two arms thinking independently produce **divergent beliefs about one world**, and no clearinghouse settles "what does the computer believe." The single-writer rule exists precisely to keep one deterministic belief stream with replayable semantics. My frame keeps thought unordered and hopes journals "consolidate" — but consolidating divergent agent cognition *is* the merge problem the branching design explicitly rejected (I-c), now renamed "accounting" with no algorithm attached. Worse: an offline arm with a genuine mandate can do genuine harm *inside* its mandate (exfiltrate at $500/day for 30 days; harass within rate limits) — attenuation bounds blast radius but does not substitute for supervision. And someone must still run settlement with liveness and durability under partition — which, under pressure, re-acquires exactly the properties of the home + writer (one sequencer, fenced, always reachable), just wearing a banker's suit. If belief needs a single thread, I have renamed the king "clearinghouse" and doubled the machinery.

*Read-only; no files modified.*

