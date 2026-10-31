# Lateral seat: the resource fabric

I read `docs/design/resource-fabric-2026-10-31.md` (§0–§2, §4, §5.2–5.3, §7, §10) and `docs/design/branching-tape-2026-10-31.md` (§0–§6). I changed no files.

## The main reframe

### (1) The rejected frame: this is about placing compute

"Resource fabric", "sites advertise capacity", "placement by trust, locality, availability, capacity, cost": this is how a datacenter scheduler such as Borg sees the world. Work has a location, and the problem is choosing it.

The physics says otherwise. A Choir computer spends most of its time waiting on remote model calls. The one real cost the doc names is 6–8 GiB of resident guest memory, and that is a cost of keeping a VM resident, not of computing. The things that really are tied to one place are:
- **seal**: the key that signs head receipts (who may append);
- **custody**: whoever owns the hardware can read every byte it hosts;
- **contact with the world**: screen, microphone, logged-in sessions, and the point where an irreversible effect leaves.

Compute itself can go anywhere.

### (2) The reframe: an authority fabric

The kernel is three relations, plus a proof that each one holds whatever policy says:
- **Seal.** Who may extend which branch. This is a key and an epoch, not a machine.
- **Custody.** Whose bytes may exist on which hardware. The model is the law of bailment: a site is a *bailee* holding the owner's goods for a stated purpose. Economics adds Grossman–Hart–Moore residual control: the hardware owner can always pull the plug and read memory, whatever the policy says.
- **Touch.** Who may cause which effect in the world, and under which precommitment identity.

Compute placement is then a **liveness optimization** on top. It can be tuned freely, by the management desk or later by self-development, because it can never break safety.

### (3) What that changes in practice

**a. One kernel rule answers Q9 and Q10: policy may only change liveness, never safety.** If every safety property holds whatever the policy value, then every configuration is safe by construction. Model checking becomes one model of the kernel with policy as a free, nondeterministic input, not a search over every org's configuration. Self-tuning is safe as long as it only touches liveness knobs. Auditing the current draft against this rule finds the leaks right away:
- "unless the owner's policy allows a class offline" (§2.3) lets policy decide whether an irreversible effect can fire twice;
- the failover threshold (§2.5) opens a host branch whose schedules "may have sent mail". A timing knob there buys a duplicate effect.

Both need a mechanism, not a knob (see c).

**b. Site trust is directional, and custody is the direction that matters.** The owner framed the rule as protecting B: "employee A's VM on B's workstation". Residual control means the sharper risk runs the other way: B can read all of A's data. A bailment needs both parties to consent, the bailor (A, whose data it is) and the bailee (B, whose site it is). Org policy is a ceiling on that consent, never a substitute for it. The proved property becomes: *no byte of computer C exists on a site whose principal C's owner has not authorized to read it.* Every site is assumed able to read everything it hosts, unless a TEE or attestation provides real evidence otherwise. This turns trust class (§1.1) from a label into a reading-rights relation.

**c. Effect identity comes from the shared prefix, and the counterparty dedupes (Q5).** Think of cheque numbers: the bank refuses a second cheque with the same number, whoever presents it.
- Derive each effect's idempotency id from the precommitment event's digest. Because of `previous_head`, that digest pins one position on the tape.
- Then two branches that inherit the same outbox item from their common prefix present the same id to Stripe, an API with idempotency keys, or a mail transport that dedupes on Message-ID.
- A duplicate across a split becomes refused by the world, rather than shown afterwards in an "effects digest".
- The home-durable rule then matters only for effects whose counterparty cannot dedupe. That narrows the cost and makes the offline-class policy a liveness choice again.

**d. "Home" is three jobs bundled into one place.** Address, bank and notary are different institutions.
- **Rendezvous (Q2).** Any relay can do it, as with DERP-style relays.
- **Notary/witness.** This does the lease-epoch ordering and head cosigning. It can be a witness quorum in the style of Certificate Transparency or Sigsum, and the phone holding the owner's passkey can be one of the witnesses.
- **Custody.** The follower copy, or backup.

Splitting them makes "multiple homes" and "home failover" ordinary. It also exposes a hidden single point of failure: today the home's control database *is* the epoch authority. That is why branching-tape case 6, "restored from an old backup, epoch rolls back", exists at all.

**e. Placement by pull, not push (Q4).** Hayek's local-knowledge argument applies: a sleeping laptop knows its own state, and a central management desk does not. Sites pull leaf work they can admit, as with a taxi rank or BOINC. The desk's job shrinks to budgets and ceilings.
- Intermittent availability needs no prediction: a laptop that wakes pulls work.
- "Refusal is an answer" turns into "you never offered".

**f. The phone is not just a client.** It holds the owner's passkey, so it is the root of every owner act. It is the natural seal-holder or witness for writer moves and primary switches. Calling it "not a site" hides the one device that roots authority.

### (4) The sharpest objection to this reframe

Splitting the home into a witness quorum, and pushing idempotency out to counterparties, adds parties and protocols. That is exactly what the owner ranked last ("simplicity is probably the most important"). Most counterparties do not dedupe: SMTP does not deduplicate by Message-ID, and computer-use posting on a social site has no idempotency key. So (c) covers only a minority of effects, and the home-durable precommit stays the workhorse.

For a 20-person org with one server, "one home, one control database" really is simpler, and good enough. A quorum of one is still a quorum, so the reframe may change the names and add nothing.

My answer: keep the single home as the default configuration, but write the kernel so that the epoch authority and the rendezvous are *roles*. Then the rule in (a) can be proved against an abstract notary, instead of being bound to Node B's PostgreSQL.

## Smaller frames, each with its own objection

**Lease as a possession of a place.**
- *Rejected frame:* a writer lease belongs to a site.
- *Reframe:* Orleans virtual actors and Cloudflare Durable Objects. The computer is the tape plus its seal, and a realization is an *activation*, a cache that can be materialized anywhere from a snapshot. The doctrine already says the working store is derived. "Checkout" then means "where the next activation happens", and the efficiency argument in §1.4 weakens: deactivating idle computers saves server memory without moving anything to workstations.
- *Objection:* cold activation of a 6–8 GiB guest takes too long to be per-request, and offline work and local keys still need a resident writer on the laptop.

**Conflict happens at branch granularity.**
- *Rejected frame:* when two sides diverge, the unit of conflict is the whole branch.
- *Reframe:* double-entry bookkeeping with a suspense account. Diverged items are booked individually, not chosen as whole ledgers.
- *Objection:* this is merge by another name, and the owner rejected merge on simplicity grounds. I would hold it only for effect-bearing items, where (c) already gives each one an identity.

**Rings measured by time and evidence.**
- *Rejected frame:* trust widens on accumulated evidence over time (R0 to R2).
- *Reframe:* insurance underwriting, where trust is bounded by *exposure limits* (maximum harm per period, irreversible actions per day) rather than by a ring label.
- *Objection:* exposure is hard to quantify for "posts as Alice". Rings are legible to owners in a way that limits are not.

**Mechanism versus policy as code layers.**
- *Rejected frame:* the kernel and the policy are layers of code.
- *Reframe:* a constitution. The kernel is the entrenched clauses (seal, custody, touch), policy is statute, and self-development is regulation by agencies, subject to review by runtime monitors. Generalize the doc's "narrowing is immediate, widening is an owner act" into a type rule: every knob is monotone, and the computer can move it on its own only in the safe direction.
- *Objection:* it only works if (a) holds. If any knob affects safety, the monotone direction is not well defined.

## One check worth running

List every policy knob in §1–§7 of the fabric doc and tag each one *liveness-only* or *safety-affecting*. Every knob tagged safety-affecting marks a place the kernel is unfinished. Today the two clear cases are the offline-effects class and the failover threshold.
