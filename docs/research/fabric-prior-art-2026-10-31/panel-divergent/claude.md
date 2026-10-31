These are 20 candidate kernels for the fabric, as asked through the simplicity lens. For each one: the primitives, the property it guarantees, the policy it leaves open, the prior art it comes from, and the configurations it would make unsafe or impossible. I don't rank them, and some contradict each other or the current draft (resource-fabric §0, §2.5, §10; branching-tape §0, §2). I read those sections only; I changed no files.

---

### 1. Signed log plus capability: two primitives in total
- **Kernel:** (a) a signed append-only log (the tape) and (b) an attenuable bearer capability, a macaroon/Biscuit-style token chain. Everything else is an event on some tape: site records, grants, leases, placements, revocations.
- **Guarantee:** every authority traces to a root signature. Attenuation is monotone, so a holder can narrow a token but never widen it. The audit trail is the tape itself.
- **Policy left open:** who mints which capabilities, their caveats, how long they last.
- **Prior art:** SPKI/SDSI, macaroons, Biscuit, Keybase sigchains, Certificate Transparency.
- **Unsafe:** revocation that must take effect instantly while the holder is offline. A bearer capability is valid until it expires or a revocation list reaches the holder, so revocation lag becomes a policy dial.
- **How it differs:** there is no separate lease or ACL machinery; one primitive covers Q1, Q3, Q6 and Q7.

### 2. Classic fenced leases with a TTL, one lock authority per computer
- **Kernel:** the home is a Chubby-like lock service. Leases expire; every append carries a fencing token (the epoch); stores reject stale epochs.
- **Guarantee:** at most one writer at a time, given bounded clock drift. Failover is automatic once the TTL passes.
- **Policy left open:** TTL length, renewal cadence, who may acquire.
- **Prior art:** Gray and Cheriton's leases, Chubby, ZooKeeper/etcd fencing tokens (Kleppmann).
- **Unsafe:** long offline work on a laptop that holds the lease. This is exactly what §2.5 rejected.
- **How it differs:** it is the textbook baseline the branching design argues against. It belongs in the space because it is the easiest to model-check.

### 3. Branch always, no leases
- **Kernel:** each branch is minted by one host and written only by that host. Single writer then holds by construction, not by lease. The only coordinated object is the **primary ref**, a pointer the owner signs.
- **Guarantee:** writers can never conflict. Fencing is not needed anywhere. Going offline is just another branch.
- **Policy left open:** when a branch becomes primary, and when old branches are pruned.
- **Prior art:** Secure Scuttlebutt (one feed per device), git and Fossil, Hypercore.
- **Unsafe:** irreversible effects, because two branches may both send the mail, so this kernel needs option 11 or 12 beside it. Also branch sprawl that the owner has to triage.
- **How it differs:** it deletes the writer lease from the kernel instead of refining it. That is simpler than the current draft, which keeps a lease per branch.

### 4. Device feeds with a deterministic fold (merging allowed)
- **Kernel:** a computer is a set of per-device feeds. Its state is a deterministic causal fold over them, using CRDTs or Matrix-style state resolution.
- **Guarantee:** state converges with no owner act at all.
- **Policy left open:** the conflict-resolution functions for each object kind.
- **Prior art:** Hypercore Autobase, Matrix state resolution v2, Automerge, local-first software.
- **Unsafe:** anything that is not commutative, such as a desk's LLM outputs that depend on prior state, or Texture's canonical revisions. It contradicts invariant I-c ("no merge").
- **How it differs:** it is the anti-branching pole, and it is useful for showing where merging *would* be safe, which is the monotone facts.

### 5. A split by monotonicity (the CALM theorem as a kernel)
- **Kernel:** two lanes.
  - **Monotone facts** (grants added, revocations, tombstones, key rotations, site records) are grow-only sets that sync with no coordination.
  - **Non-monotone acts** (lease moves, primary switches, irreversible effects) need one coordination primitive: a CAS at a named authority.
- **Guarantee:** whatever the kernel lets through without coordination is provably confluent (CALM).
- **Policy left open:** which authority serves as the CAS point for each non-monotone kind.
- **Prior art:** CALM (Hellerstein and Alvaro), Bloom, CRDT revocation sets. Branching-tape I-d already does this halfway.
- **Unsafe:** an "un-revoke" act, which is impossible by construction. That is a feature, but it means mistaken revocations can only be fixed by minting a new grant.
- **How it differs:** the kernel is defined by a mathematical property rather than by entities like home, site or lease.

### 6. Linear resources: a writer lease and an effect intent are both "spend once"
- **Kernel:** one primitive, a linear token: an owned object whose transfer consumes it. The writer lease, an effect intent ("send this mail once"), a leaf's budget and a capability lease are all linear objects.
- **Guarantee:** the token can't be duplicated. Exactly-one-writer and at-most-once effects become the same theorem.
- **Policy left open:** who issues tokens, and their denominations (a budget of N offline sends).
- **Prior art:** linear logic, Rust ownership, Move/Sui owned objects, Bitcoin UTXOs.
- **Unsafe:** a token held on a machine that dies. Recovering it means re-issuing, and that requires proving the token was never spent, which comes back to branching or witnesses.
- **How it differs:** it unifies Q1 and Q5 under one rule instead of two mechanisms.

### 7. Offline e-cash for effects: a double spend is detectable and attributable
- **Kernel:** before going offline, a site gets a pre-issued wallet of one-time effect tokens, one per effect class, signed by the home. Spending one offline is allowed. Spending one twice reveals the spender's identity after the fact, so no prevention is needed.
- **Guarantee:** offline effects are bounded by the wallet size, and misuse is provable after the fact.
- **Policy left open:** wallet size per class (owner policy, which the computer can tune within a cap).
- **Prior art:** Chaum, Fiat and Naor's offline e-cash; Brands; mobile payment float limits.
- **Unsafe:** effects whose *harm* is not bounded by their count, such as one catastrophic payment. Those classes have to get a wallet of size zero.
- **How it differs:** it gives a quantitative answer to "fire offline or not" (§10 proposal 5) instead of a yes/no class list.

### 8. One verified policy decision point, plus a decision log
- **Kernel:** a single policy language with formal semantics (Cedar/Zanzibar-like). Every admission, grant, placement and capability use is a query to it, and every decision is logged on the tape.
- **Guarantee:** policies are analyzable. An SMT check can prove "policy P′ is no more permissive than P", which is a ready-made bound for self-tuning (Q10).
- **Policy left open:** everything org-specific, written as policy text.
- **Prior art:** Zanzibar, Cedar (semantics verified in Lean), AWS Zelkova, OPA.
- **Unsafe:** relational facts going stale offline ("is A still an employee?"). It also leaves open whether a cached decision is valid.
- **How it differs:** the kernel is a *language*, not a protocol. Organizations differ only in policy text.

### 9. A capability microkernel with a derivation tree (seL4 in spirit)
- **Kernel:** a few calls (send, receive, grant, revoke), all mediated by handles. Revocation walks the capability derivation tree, so it kills every descendant.
- **Guarantee:** isolation and noninterference proofs carry over in shape. Revocation is complete.
- **Policy left open:** user-level servers implement placement, org rules and capability brokers.
- **Prior art:** seL4, KeyKOS/EROS, Fuchsia Zircon handles.
- **Unsafe:** a derivation tree spread across sites that partition, where revocation can't reach a branch of the tree. That forces either a central tree (at the home) or revocation with an expiry.
- **How it differs:** like 1, but revocation is *strong*. Option 1 trades that away for offline operation.

### 10. Sturdyrefs over CapTP: reachability *is* authority
- **Kernel:** every computer, site and capability is an object reached through an unguessable sturdyref. The home is just a durable resolver for sturdyrefs plus a relay, and reachability (Q2) and access (Q3) are the same act.
- **Guarantee:** no ambient authority. You can only reach what you hold a reference to.
- **Policy left open:** reference distribution and caretaker proxies for revocation.
- **Prior art:** E, OCapN, Spritely Goblins, Cap'n Proto.
- **Unsafe:** org audit questions like "who can reach B's workstation?". Those can't be answered without tracking every reference, so they are poor for compliance-heavy orgs.
- **How it differs:** it deletes the separate routing layer. Most other options keep routing and access apart.

### 11. Key event logs for every principal
- **Kernel:** persons, orgs, sites, computers and writer realizations each get a key event log with pre-rotation. Delegation is a delegated identifier, and revocation is rotation. The writer's authority is "the currently delegated writer key of the computer's key log".
- **Guarantee:** key compromise is recoverable through pre-rotation. Witnesses detect duplicity, such as two forks of a key log.
- **Policy left open:** witness sets, delegation depth, rotation thresholds.
- **Prior art:** KERI, did:peer and did:webs, Keybase.
- **Unsafe:** a principal with too few witnesses, such as one person with only a laptop.
- **How it differs:** the lease reduces to *key delegation*. There is no lease table, only key state.

### 12. A witness quorum instead of a single home
- **Kernel:** a branch head is valid once k of n witnesses have countersigned it. Witnesses can be any of the person's sites plus the home. Moving the writer means the quorum signs a new epoch.
- **Guarantee:** no single home is a point of failure, and failover is possible without branching when a quorum is reachable.
- **Policy left open:** n, k, and which sites count as witnesses.
- **Prior art:** Paxos/Raft (flexible quorums), CT witnesses, Sigstore Rekor, KERI witnesses.
- **Unsafe:** n = 2 (laptop plus server) has no majority, so it falls back to a single home anyway. Also a quorum of devices one person controls, which is a security illusion for org audit.
- **How it differs:** it challenges rule 2 ("every computer has a home") directly.

### 13. Only effects are coordinated
- **Kernel:** the tape is freely branchable and nothing about it is coordinated. The *only* coordinated primitive is an effect gate per external account: an idempotency-keyed outbox at the edge of that mailbox or payment rail, run by the home or the provider.
- **Guarantee:** exactly-once at the edge, and everything inside the tape stays reversible.
- **Policy left open:** which effects pass through which gate, and the gate's approval rules.
- **Prior art:** Stripe idempotency keys, the transactional outbox, sagas, Temporal.
- **Unsafe:** providers with no idempotency (raw SMTP to a third party), and offline effects, since the gate isn't local.
- **How it differs:** it moves the irreversibility boundary to the edge of the world. Every internal mechanism then gets simpler.

### 14. Precommitment with countersignatures (an n-party escrow)
- **Kernel:** every consequential act is a signed proposal that runs only after the required countersignatures arrive. Examples are placing a leaf on B's workstation, sending mail, offboarding, or moving a writer. The roles are owner, site principal, org and home.
- **Guarantee:** no single party can act unilaterally where policy demands consent. That is exactly the owner's rule that no org policy alone can place A's work on B's workstation.
- **Policy left open:** which acts need which signer set.
- **Prior art:** multisig, TUF threshold roles, two-person integrity, 2PC, Interledger escrow.
- **Unsafe:** liveness when a signer is absent (a sleeping laptop, an away employee). A long timeout weakens consent.
- **How it differs:** consent is the primitive, rather than authority or ownership. It matches Choir's precommitment records.

### 15. Information-flow labels (DIFC)
- **Kernel:** every datum, computer, site and leaf carries a label. Placement, capability use and publication are allowed only when the label may flow there. Declassification is a privilege that names its owner.
- **Guarantee:** noninterference. A's data provably can't reach B's workstation unless someone with declassify rights says so.
- **Policy left open:** the label lattice, meaning the org's confidentiality structure, and who holds declassify rights.
- **Prior art:** HiStar, Flume, Asbestos, Jif, Bell–LaPadula.
- **Unsafe:** label creep, where everything ends up tainted top-secret, and declassification that is too liberal. All the real risk concentrates there.
- **How it differs:** it governs where *data* may flow, not who may *act*. It also covers the native capabilities: screen contents carry the label of what they show.

### 16. Thin sites, an authoritative home
- **Kernel:** only homes write tapes. Sites are stateless executors (leaves, capability endpoints, CPU servers) and keep nothing durable beyond caches.
- **Guarantee:** trivially one writer, and losing a site is never a data-loss event.
- **Policy left open:** which sites execute what.
- **Prior art:** Plan 9 CPU servers and terminals, the Borg/Kubernetes control plane, thin clients.
- **Unsafe:** offline work is impossible. That makes it safe by construction, but it rejects "the Mac leads".
- **How it differs:** it is the simplest pole. It shows how much complexity offline work alone costs.

### 17. A migrating sealed computer (the baton)
- **Kernel:** the computer is one portable sealed image (snapshot plus tape tail) with a baton. Whoever holds the baton is the writer, and the baton moves only by an atomic two-party handoff.
- **Guarantee:** a single writer, with no lease table.
- **Policy left open:** when the baton moves, and who may receive it.
- **Prior art:** Sprite and Emerald process migration, VM live migration, token ring, Telescript.
- **Unsafe:** a baton lost on a dead machine. Recovering it needs either a quorum (12) or a branch (3), so the baton is never truly the whole story.
- **How it differs:** location is the identity of the writer, which suits the "checkout and handback" mental model.

### 18. Bilateral matchmaking for placement
- **Kernel:** sites and work each publish an advert with a `requirements` expression and a `rank` expression. A match needs *both* requirements to hold. The kernel is just the matching and admission check, and refusal is a normal answer.
- **Guarantee:** a site's principal always has a veto, because their requirements are part of the match. Trust is a hard filter by construction.
- **Policy left open:** everything in the adverts, including the rank terms for cost and battery.
- **Prior art:** Condor ClassAds, two-level scheduling with Mesos resource offers, Tycoon/Spawn markets.
- **Unsafe:** rank functions that trade trust for price, unless the kernel forbids trust terms in `rank`.
- **How it differs:** it treats Q4 as symmetric negotiation instead of a central management-desk decision.

### 19. An identity overlay network as the kernel
- **Kernel:** a WireGuard-style mesh where a coordination server distributes only keys and an ACL. Relays handle NAT traversal (Tailscale's DERP), and every connection is authenticated by node identity.
- **Guarantee:** reachability from any client to any site, with the ACL enforced at both ends.
- **Policy left open:** the ACL file, meaning org and person rules.
- **Prior art:** Tailscale, Nebula, ZeroTier, BeyondCorp.
- **Unsafe:** a compromised coordination server (it can add keys), and ACLs that don't distinguish work within a site.
- **How it differs:** it solves Q2 and coarse Q3 at the network layer and leaves the tape layer unaware of the fabric.

### 20. A verified safety monitor over an untrusted tuner, plus deterministic simulation
- **Kernel:** a small verified monitor checks invariants such as I-a to I-d, default deny across people, and effect budgets. An untrusted policy tuner (the computer's own self-development) proposes configurations freely, and the monitor reverts to a safe baseline when one breaches the envelope. Configurations are admitted only after a TLA+/P model check and deterministic simulation pass over that parameter range.
- **Guarantee:** whatever the tuner does, the system stays inside the proven envelope (Q9, Q10).
- **Policy left open:** everything inside the envelope.
- **Prior art:** the Simplex architecture (Sha), shields in safe reinforcement learning, runtime verification, FoundationDB and Antithesis simulation, Jepsen.
- **Unsafe:** invariants nobody wrote down (the monitor protects only what it states), and non-deterministic LLM steps that the simulation can't replay unless their outputs are recorded.
- **How it differs:** this is a meta-kernel. It can wrap any of 1–19 and is the only option aimed squarely at self-development.

---

### How these options differ
1. **What gets coordinated:** everything (2, 16), only non-monotone acts (5), only effects (13), or nothing (3, 4).
2. **How a single writer is enforced:** a lease with a TTL (2), a lease with no expiry (the current draft), by construction (3), a linear token or baton (6, 17), key delegation (11), or a quorum (12).
3. **Merge stance:** never (3, 16), deterministic folds (4), or monotone facts only (5).
4. **Authority model:** bearer capabilities (1, 10), a capability derivation tree (9), a policy language (8), information-flow labels (15), multi-party consent (14), or a network ACL (19).
5. **Revocation strength:** expiry or lag (1), complete (9), monotone and irreversible (5), or rotation (11).
6. **Where the home sits:** sole authority (16), resolver and relay (10), one witness among several (12), or absent (3, 4).
7. **Offline effects:** forbidden (13, 16), budgeted (7), or allowed per class (the current draft).
8. **Who places work:** a central desk (the current draft), bilateral matching (18), or label flow (15).
9. **What is verified:** protocol invariants (2, 12), a policy language (8), noninterference (9, 15), or envelopes enforced at runtime (20).
10. **Primitives counted:** one (6), two (1), and up to about six for the current draft's home, lease, ref, branch, grant and capability manifest.
11. **Which personas break:** a person with two devices (12), long offline use (2, 16), compliance audit (10), and providers without idempotency (13).
