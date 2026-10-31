Working...
Read the fabric doc (§0, §1–§5, §8–§10) and branching tape. Below: the space of candidate kernels. I deliberately hold the doc's current point (branch + non-expiring lease + home-durable precommitment) as **one** point in it, not the frame.

**Two decompositions that make the space legible before the list.**

*"Single writer" is three properties, not one.* **Exclusion** (no two writers), **ordering** (which act came first), **durability** (nothing lost). Every kernel below is a choice of which of these is a theorem, which is a runtime check, and which is left to detection or economics. The doc's current design makes durability a theorem, ordering a theorem (hash chain), and exclusion a *detection* property (equivocation proof + owner-chosen primary). That inversion is defensible but unusual and it moves the real security boundary to the effect gate.

*The sink theorem.* Exclusion is only as strong as the least-fenced sink. In a personal-computer fabric the highest-value sinks are the owner's logged-in accounts driven by computer use — mail, payments, social — and **those cannot be fenced**; no mail server checks an epoch. Therefore any kernel whose safety story is "only one writer, so effects fire once" is unsound for exactly the case the product exists for. Either the kernel makes effects the primitive, or it makes the writer the primitive and accepts a hole at the world boundary.

---

## A. Exclusion / authority kernels

**1. Lease + fencing token** (exclusion kernel).
*Core:* a lease authority grants `(holder, epoch)` with a TTL; every sink rejects writes below its highest-seen epoch. *Guarantee:* at most one *effective* writer at any instant, given bounded clock drift and universal sink checking. *Policy open:* TTL, renewal, who may take the lease, what counts as a sink. *Prior art:* Gray–Cheriton leases (1989), Chubby, ZooKeeper, etcd, SCSI persistent reservations, STONITH/Pacemaker. *Unsafe:* every sink that doesn't check — a local filesystem, an outbound socket, a browser session. This is the textbook kernel and it fails precisely on non-cooperative sinks; the doc rejects its TTL for offline reasons, but the fencing half is orthogonal to the TTL half and could be kept alone.

**2. Branch ref, no expiry** (durability kernel; the doc's current point).
*Core:* any site appends to its own branch; one CAS-advanced `primary` ref at the home names the authoritative chain; conflicts are visible, never merged. *Guarantee:* no lost work, no silent divergence, total ordering *within* a branch. *Policy open:* branch cap, who may switch primary, what may run on a non-primary branch. *Prior art:* Git refs, IPNS, Dynamo conditional writes, "fork-and-choose" in P4/version-control systems. *Unsafe:* authority is *deferred* — during a split both branches are live writers, so non-idempotent effects can duplicate. Exclusion becomes a post-hoc owner judgment; the kernel is safe only if paired with 21 or 22. This is the sharpest inversion in the space and the doc should name it as such.

**3. Deterministic sequencer / log owner.**
*Core:* sites emit intents; one sequencer assigns a total order; all sites replay deterministically. *Guarantee:* a single serial history; equivocation is structurally impossible; effects are a pure function of log position. *Policy open:* sequencer placement, batch window, which acts may be speculative. *Prior art:* Calvin, Spanner/TrueTime, Zab, state-machine replication, Kafka's log-as-truth. *Unsafe:* no offline commit (an intent needs the sequencer); sequencer is a SPOF unless it is a Raft group (→ 5); speculative execution of effects is unsound, so offline work must be effect-free by construction.

**4. Quorum / escrow.**
*Core:* N rights-holders; a writer needs ≥ q tokens; offline writers receive escrowed write-rights in advance. *Guarantee:* two writers require intersecting quorums — impossible for q > N/2. *Policy open:* N, q, how many tokens a laptop may pre-escrow, token expiry. *Prior art:* Gifford weighted voting, Oki–Liskov escrow transactions, Dynamo R/W quorums. *Unsafe:* pre-escrow is a promise of future writes; a laptop that escrows and vanishes strands the computer for the escrow window, and one that escrows too little stalls offline work unpredictably. The escrow authority is itself a consensus problem, so this kernel relocates the hard part rather than removing it.

**5. Per-computer Raft group.**
*Core:* 3–5 replicas per computer; leader elected by timer; all appends through the leader; log replicated. *Guarantee:* linearizable tape, automatic failover in one election timeout, no manual epoch bookkeeping. *Policy open:* replica placement, election timeout, read quorum. *Prior art:* Raft, Viewstamped Replication, Multi-Paxos, etcd. *Unsafe:* 3–5× cost, majority must be reachable to write, and a sleeping laptop is by definition a minority partition. This is the correct kernel if computers are servers and the wrong one if they are laptops — the fabric wants both, so it can only be a per-class choice.

**6. BFT committee.**
*Core:* a known committee of sites; PBFT-style three-phase commit; 3f+1 tolerates f Byzantine. *Guarantee:* no equivocation and no forged history even with f malicious sites. *Policy open:* committee membership, rotation, sanction. *Prior art:* PBFT, Tendermint, HotStuff, Stellar. *Unsafe:* 2/3 of sites must be online, so offline progress is impossible; 2–3 RTT latency; and personal-computer fabrics have no natural committee — the org case does, the individual case does not. Good fit for the *organization's* always-on computers, bad fit for a person's Mac.

**7. Cold-move-only (unavailability-first).**
*Core:* exactly one writer by construction; moving = snapshot, new epoch, confirmed power-off of the old site. *Guarantee:* two writers are physically impossible if you accept downtime. *Policy open:* who may move, the confirmation protocol. *Prior art:* STONITH, "power off then migrate" in hypervisors, airline-style single-operator models. *Unsafe:* a laptop that dies without a confirmed power-off leaves the computer down until the owner acts; no offline mode at all; and a *confirmed* power-off over a network is precisely what a partition prevents, so the guarantee is weakest exactly when it is needed.

**8. Linear/affine ownership token.**
*Core:* the writer token is a linear value — movable, not copyable — enforced by a type system or runtime. *Guarantee:* at most one holder by construction; no epoch arithmetic, no timers. *Policy open:* transfer protocol, recovery from token loss. *Prior art:* Wadler's linear types, Honda's session types, Austral, Rust ownership, unique capabilities in E. *Unsafe:* networks duplicate and lose messages, so a *distributed* linear type needs a runtime that can prove non-duplication — which is consensus wearing a type theory hat. Token loss loses the computer, and the recovery path is itself a copy, which is the contradiction the model cannot escape.

**9. Key-possession chain (crypto authority).**
*Core:* authority is the ability to extend a signed hash chain; key rotation and revocation are the entire mechanism. *Guarantee:* verifiable provenance, offline signing, zero coordination. *Policy open:* key custody, rotation cadence, revocation channel. *Prior art:* git commit signing, certificate transparency, Sigstore, KERI. *Unsafe:* a stolen key silently forks the tape and only *transparency* detects it; "who may sign now" becomes a social fact rather than a kernel property; and two holders of the same key are indistinguishable, so exclusion has no meaning.

**10. Threshold / MPC device quorum.**
*Core:* the owner's authority is a threshold key split across their devices; a writer must present a threshold signature. *Guarantee:* no single device compromise yields authority; revocation is re-sharing. *Policy open:* threshold, share custody, recovery contacts, org-admin shares. *Prior art:* Shamir, FROST, MPC wallets, Apple recovery contacts. *Unsafe:* availability collapses when devices sleep or are left at home — the same failure the lease TTL had, moved into cryptography; org admins need shares, which reintroduces a principal who can act alone.

## B. Authority-as-data kernels (capabilities, labels, policy)

**11. Object-capability local kernel.**
*Core:* objects, unforgeable capabilities, invocation only through a cap, caps attenuable and transferable. *Guarantee:* confinement by construction — least authority holds for every component, including compromised ones. *Policy open:* root-cap minting, the initial cap graph. *Prior art:* seL4, E, KeyKOS, Genode, Fuchsia components, WASI capabilities. *Unsafe:* it is a *local* kernel. Cross-site capability transfer needs a distributed protocol, revocation is the known hard problem, and the kernel says nothing about *whose* work may run on *whose* site — that is a trust relation, not a cap.

**12. Bearer capability + attenuation.**
*Core:* a token is a signed caveat list; any holder may append caveats offline; any verifier checks locally. *Guarantee:* monotone attenuation — privilege can only shrink; delegation works without the issuer. *Policy open:* caveat language, lifetime, revocation feed. *Prior art:* Macaroons, Biscuit, SPKI/SDSI, UMA, OAuth scopes (the weak cousin). *Unsafe:* bearer theft is total until expiry; revocation is the one non-monotone operation the model cannot express natively, so it must be bolted on via TTLs or a revocation list; and caveat languages reliably grow into policy engines with their own bug class.

**13. Information-flow lattice.**
*Core:* every datum and every sink carries a label; a flow rule (no read-up, no write-down); explicit declassification points. *Guarantee:* confidentiality and integrity for *every* configuration that respects the lattice — literally the "all reasonable configurations safe" property. *Policy open:* the lattice shape, per-site labels, declassifiers. *Prior art:* Bell–LaPadula, Biba, Denning's lattice, Jif, Asbestos/Flume, seL4 information-flow. *Unsafe:* labels are coarse; a 20-person org with per-person, per-project, per-client rules needs a product lattice that explodes combinatorially; covert channels remain; and over-tainting collapses into "everything confidential", which starves placement and makes every workstation ineligible.

**14. Decidable policy-as-code.**
*Core:* kernel facts `(principal, action, resource, context)`; every decision is a pure function in a language with decidable authorization. *Guarantee:* authorization is analyzable, diffable, and auditable; no implicit authority. *Policy open:* the schema of facts, the policy itself, freshness bounds. *Prior art:* Zanzibar (ReBAC), Cedar, OPA/Rego, XACML. *Unsafe:* facts must be timely and trustworthy — a stale grant is an authorization bug, not a caching bug; the policy becomes the entire attack surface; and it solves *access*, not *exclusion*: two writers can both be authorized.

**15. Cross-principal role + resource-owned policy.**
*Core:* a principal assumes a scoped, time-bounded role; the policy lives on the *resource*, not the caller; an external ID defeats the confused deputy. *Guarantee:* no principal acts on another's resource without that resource's own policy; revocation = delete the trust document. *Policy open:* role definitions, permission boundaries, session length. *Prior art:* AWS IAM AssumeRole + ExternalId, GCP service accounts, Azure RBAC, Hardy's confused deputy. *Unsafe:* policy sprawl; session credentials outlive policy changes; and the case that matters — "the org says yes but the employee's workstation says no" — requires the *site* to be a principal, which most IAM models do not represent.

**16. Treaty / federation protocol (detection, not prevention).**
*Core:* each site is sovereign; the kernel is a protocol among principals — signed statements, a shared naming root, bilateral grants. *Guarantee:* no global authority means no global compromise; the org is a trust root, not a controller. *Policy open:* the treaty text (grants), dispute procedure. *Prior art:* BGP, SMTP, DNS, X.509 PKI, Matrix, ActivityPub, SPKI. *Unsafe:* no global invariant is enforceable — you can prove "no two writers" only *after* the fact by equivocation evidence; revocation propagates at the speed of gossip; and a member can simply ignore the treaty, which is a political failure mode, not a technical one.

## C. Placement and admission

**17. Two-level quota-offer scheduler.**
*Core:* sites offer resources; each owner/org holds a quota; per-scope schedulers place work; the site's admission is final. *Guarantee:* no scheduler can exceed quota, no site can be oversubscribed, regardless of scheduler bugs. *Policy open:* quota allocation, offer shape, scheduler choice. *Prior art:* Mesos two-level scheduling, Borg/Omega, Kubernetes ResourceQuota + taints/tolerations, SLURM. *Unsafe:* no global optimum, priority inversion, and trust is only enforced if the *offer* is trust-scoped — a scheduler can place confidential work onto a site whose offer says "any" and the kernel will not notice.

**18. Market / auction admission.**
*Core:* a credit price per site-second; placement is bidding; the owner's budget is the only global constraint. *Guarantee:* the cost bound is never exceeded; already-paid-for idle hardware wins naturally. *Policy open:* price schedule, credit issuance, caps. *Prior art:* spot markets, DRF, Borg's cost model, the owner's 2026-10-28 compute-credit note. *Unsafe:* money cannot encode trust — a bid can be gamed and a cheap untrusted site wins confidential work; and a market without a hard trust filter is an incentive to cheat. It is a layer *inside* 13/17, never a substitute.

**19. Attestation-gated placement (TEE as the trust primitive).**
*Core:* every site proves what code it runs and that the guest is isolated; keys release only into an attested enclave. *Guarantee:* the host operator cannot read the guest — which dissolves the org's "A's VM on B's workstation" problem cryptographically instead of by policy. *Policy open:* which TEEs are trusted, attestation freshness and policy. *Prior art:* SEV-SNP, TDX, Apple SEP/Secure Enclave, Nitro Enclaves, Confidential Containers, Keystone. *Unsafe:* side channels, a thin attestation ecosystem, vendor concentration — and attestation proves *code*, not *behavior*: an attested program can still lie about what it did.

**20. Taint/toleration trust relation.**
*Core:* a first-class relation `site principal × work owner × work class`; default deny across people; only a signed grant opens a cell. *Guarantee:* no org policy can override a site owner's refusal. *Policy open:* the relation algebra, grant duration, delegation depth. *Prior art:* Biba integrity, Kubernetes taints/tolerations, SELinux/MCS, the doc's own §1.2 site access control. *Unsafe:* the relation is only as good as the site's ability to *see* the work's labels; a hypervisor that cannot inspect the guest cannot enforce them, so the kernel silently degrades to trust-on-enrollment.

## D. Effect kernels

**21. Effect ledger / precommitment barrier.**
*Core:* every irreversible effect is a precommitment record made durable before the actuator fires; the receipt is appended after; a global effect name dedupes. *Guarantee:* at-most-once per effect name, and the owner can always see what the computer did to the world. *Policy open:* which classes may fire offline, required durability level, namespace ownership. *Prior art:* Choir's own precommitment records, transactional outbox, Stripe idempotency keys, Kafka idempotent producer, 2PC's prepare phase. *Unsafe:* it does not exclude two writers by itself (both branches can precommit); it inserts a round trip before every send, which is a fate-sharing dependency on the home for every irreversible act; and exactly-once requires a *single* effect namespace, which is a global naming problem, not a local one.

**22. Durable execution / idempotent activities.**
*Core:* a run is a deterministic replay of recorded steps; each effectful step's result is recorded once, keyed by `(run, step)`. *Guarantee:* exactly-once effects for any step that has an idempotency key; crash-resume without re-firing. *Policy open:* retry classes, timeouts, which steps are effectful. *Prior art:* Temporal, DBOS, Cadence, AWS Step Functions, sagas. *Unsafe:* needs deterministic workflow code (versioning pain is real); an effect without a key cannot be made exactly-once; and it solves *within one writer* — it says nothing about the two-writer problem, which is the fabric's actual question.

**23. Compensating effects.**
*Core:* allow both branches to act; require every effect to be reversible or compensable within a window; reconcile by compensation. *Guarantee:* convergence without coordination. *Policy open:* the compensable class, the window, who bears the cost. *Prior art:* sagas, financial holds, "eventual consistency with repair". *Unsafe:* not all effects are compensable — a sent mail is sent; compensation is itself an effect and can fail; and the world is not obliged to cooperate with the compensation protocol.

**24. CRDT / commutative operations.**
*Core:* all state is a join-semilattice; merge is the join; no coordination. *Guarantee:* strong eventual consistency across any partition. *Policy open:* which types, which operations. *Prior art:* CRDTs (Shapiro et al.), Bayou, Automerge, Riak. *Unsafe:* effects and authorization are not commutative — "send" cannot join, "revoke" versus "grant" needs causal order, precommitment needs a total order. A pure CRDT kernel makes the one thing that must be safe — the world — unsafe. Right for views, wrong for authority.

## E. Control, org, platform

**25. Supervisor tree / actor links.**
*Core:* processes, links, monitors, supervision strategies, messages only. *Guarantee:* no orphaned obligation — failure propagates by signal, deterministically. *Policy open:* restart strategies, escalation. *Prior art:* Erlang/OTP, Akka, Hewitt actors, Kubernetes ownerReferences (weak cousin). *Unsafe:* it is exactly the "parent/child is a control ontology" that doctrine I3 forbids; links are symmetric authority; and a partition turns "down" into an ambiguity supervision trees handle badly because they assume a local node.

**26. Controller grant + addressed publication** (the doc's own §4.2, generalized).
*Core:* control is never a write; it is a durable grant naming verbs, scope, budget, expiry; the controller publishes proposals, the controlled computer admits them, results return by publication. *Guarantee:* every computer stays single-writer; no grant exceeds its granter; no cycles. *Policy open:* verb set, scope language, budget accounting, cycle prevention. *Prior art:* AWS cross-account roles, OAuth delegated authorization, capability attenuation, publication/adoption in the Autopaper graph. *Unsafe:* latency — every directive takes two publications; a runaway controller is bounded only by budget, which must be counted where the work happens; and the grant graph is a second authority that must itself be consistent.

**27. OS-native kernel (rent the platform's security).**
*Core:* the site's OS is the reference monitor — TCC, entitlements, App Sandbox, launchd, the Keychain; Choir is an app plus signed helpers. *Guarantee:* inherit a battle-tested isolation model; no new TCB. *Policy open:* per-OS capability manifests. *Prior art:* macOS TCC/App Sandbox, Windows AppContainer, Flatpak/XDG portals, Android permissions, MDM. *Unsafe:* no uniform cross-platform guarantee, so orgs cannot audit one model; per-OS divergence becomes the policy; and the OS cannot express "whose data may run here", which is the fabric's central question.

## F. Verification and self-tuning

**28. Proof-carrying configuration.**
*Core:* a formal spec of the fabric state machine; every deployment is an instance; a model checker or refinement proof discharges safety for that instance before admission. *Guarantee:* "every admitted configuration is safe" is a theorem, not a test. *Policy open:* a deliberately bounded configuration language. *Prior art:* TLA+, Alloy, seL4 refinement, IronFleet, Chaparral, Nix's configuration-as-value. *Unsafe:* state explosion; the spec's adequacy is itself unproven (you can prove the wrong theorem); and restricting the config language pushes real policy into unverified code, which is where the bugs move.

**29. Runtime shield / invariant monitor.**
*Core:* a monitor in every action path; an inductive invariant; allow only if the post-state satisfies it. *Guarantee:* unsafe states are unreachable at runtime, even with buggy policy. *Policy open:* the invariant set, fail-open versus fail-closed. *Prior art:* runtime verification, shielding for RL (Alshiekh et al.), admission webhooks, safety envelopes in autonomous control. *Unsafe:* the monitor must see every action (it is a membrane, → 30); a monitor bug is a total failure; and it can only enforce local invariants — "no two writers across a partition" is not a local predicate.

**30. Membrane / cell chokepoint.**
*Core:* a cell = VM + host agent + lease; every crossing is a membrane invocation that can inspect, transform, or refuse. *Guarantee:* one chokepoint; egress cannot bypass policy. *Policy open:* how much semantics the membrane understands. *Prior art:* object-capability membranes, Qubes, gVisor, service-mesh sidecars, network middleboxes. *Unsafe:* a byte-proxying membrane cannot enforce "at most one mail send" without parsing the effect; it costs latency; and it must sit *in* the effect path to be a real barrier rather than a logger.

**31. Adaptive policy inside a proven envelope (self-tuning as a kernel property).**
*Core:* the kernel fixes an envelope (a set of admissible states or a cost bound); a tuner moves policy within it; every tuning act is itself recorded and reversible. *Guarantee:* self-development cannot leave the proven set, by construction rather than by review. *Policy open:* the envelope's dimensions, the tuner's objective, the rollback horizon. *Prior art:* safe RL with shields, Lyapunov/control-barrier methods, adaptive controllers, "budgeted autonomy". *Unsafe:* if the envelope is expressed as a runtime check it inherits 29's blindness; if expressed as a type it must be narrow, and narrow envelopes make the tuner useless; and the objective function is where the design's real values hide.

---

## Dimensions along which these differ

1. **Which property is a theorem:** exclusion (1, 5, 6, 7, 10), ordering (3, 5), durability (2, 9), confinement (11), flow (13), effect-once (21, 22).
2. **Where the decision is made:** central authority (1, 3, 4, 5), data sink (1, 21, 30), resource owner's policy (15), type system (8, 13, 28), runtime monitor (29), economics (18), after the fact (2, 16).
3. **Coordination cost:** zero (9, 16, 24), one RTT (2, 21), quorum (4, 5, 6), unavailability (7).
4. **Partition posture:** refuse writes (3, 5, 6, 7), branch and reconcile (2, 23), act on one side and detect (1, 9, 16), heal silently (24).
5. **Offline posture:** first-class (2, 12, 21-with-policy), speculative-only (3), impossible (5, 6, 7), delegated (10, 12).
6. **Effect semantics:** at-most-once (21, 22), exactly-once (22 with keys), at-least-once + dedupe (21/22 hybrid), compensable (23), excluded by assumption (1, 7).
7. **Unit of trust:** site (19, 20), principal (15), data label (13), code measurement (19), capability (11, 12), contractual party (16).
8. **Where policy lives:** kernel constant (1, 8), config value (17, 18), per-object document (15, 14), proof obligation (28), learned parameter (31).
9. **Revocation latency:** immediate (15, 26), at expiry (1, 12), at revalidation (2), by transparency log (9, 16), by compensation (23).
10. **What counts as the TCB:** the lease service (1, 4, 5), the hypervisor + TEE (19), the type checker (8, 13, 28), the monitor (29), the OS (27), nothing shared (16).
11. **Expressiveness vs decidability:** full policy language (14, 20) versus bounded and checkable (28, 31) versus none (1, 7, 8).
12. **Who bears the failure:** the owner (2, 7), the site operator (20, 27), the vendor (19), the economics (18), the spec author (28).

## Coverage map (which items speak to which question)

- Q1 writer authority: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 — and 21/22 for the part exclusion cannot cover.
- Q2 reachability: 16 (federation/naming), 30 (chokepoint), 27 (native), 25 (links); none of the authority kernels solve routing, which is a separable kernel the doc already treats correctly.
- Q3 access control: 11, 12, 13, 14, 15, 16, 20.
- Q4 placement: 17, 18, 19, 20, 27.
- Q5 irreversible effects: 21, 22, 23 — plus 2 and 9 name why they are *not* covered by authority.
- Q6 one computer controlling another: 26, 25, 11, 12.
- Q7 native capabilities: 27, 30, 12, 29.
- Q8 org lifecycle: 15, 16, 20, 28 (offboarding as a policy instance, not a kernel).
- Q9 mechanism vs policy: 28, 29, 31, 14, 17.
- Q10 self-tuning: 31, 28, 29.

## Structural incompatibilities (not rankings — pairs that cannot both be theorems)

- 1 (TTL lease) and 2 (no-expiry branch) cannot both be the authority for the same branch; the doc has already picked 2 for writers and 1 for leaves, which is coherent only if leaves truly hold no state.
- 3/5/6 (ordering or quorum) and offline-first (2) are mutually exclusive for the same acts; a fabric must choose per work class, not globally.
- 24 (CRDT everywhere) and 21 (effect barrier) are incompatible: a join-semilattice cannot host a total-order gate.
- 8 (linear token) and 2 (branch) conflict on the meaning of "the same authority existing twice" — the token says impossible, the branch says expected.
- 16 (treaty) and 28 (proof-carrying config) pull opposite ways: the first accepts unenforceable global invariants, the second insists on them.
- 19 (TEE) and 27 (OS-native) overlap as TCB choices; picking both doubles the attestation surface without adding a guarantee.
- 7 (cold-move) and 2.5's automatic failover are directly opposed; the doc currently says failover is a host branch, which is 2, not 7.
- 29 (monitor) and 11 (capability) can be layered, but only if the monitor is inside the invocation path — otherwise the monitor is advisory and the capability is the kernel.

**Skeptic's note on the doc's own point.** Options 2 + 21 + 20 is what the design currently is. Its unexamined assumptions are: (a) that durability is the property worth making a theorem while exclusion is worth detecting — plausible for personal computers, wrong for the org's always-on committers; (b) that the home can be a dependency for every irreversible effect without violating the stated fate-sharing rule, which it does; (c) that "trust class" is a hard filter when it is in fact an undefined lattice — 13 and 20 are the fork the doc has not taken; and (d) that self-tuning inside bounds (Q10) is reachable from a monitor, when the only kernels that actually bound self-modification are 8, 28 and 31.
