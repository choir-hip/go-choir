Warning: --sandbox always uses the autonomous permission mode; ignoring --permission-mode dangerous.
I'll read the design docs to ground the option space, then generate divergent candidate kernels with a security bias.Let me read the rest of the resource-fabric doc, especially §10 and the middle sections.I read both design docs in full. The doc already contains an implicit kernel (home-DB lease rows + CAS + signed receipts + home-durable precommitments). Below are the widest set of candidate kernels I can construct — each is a different minimal primitive set with a different guaranteed property and a different trust root. Security lens throughout: for each I name what the kernel *proves*, what it leaves to policy, and which configurations silently fall outside the guarantee.

---

**1. Lease-authority kernel — the home's control DB is the single arbiter**

Core: kernel = {lease registry with epoch CAS, signed head receipts naming (branch, epoch, holder key), equivocation proofs, home-durable effects gate}. This is the doc's own design stated as a kernel. The guaranteed property: *at most one writer per (computer, branch, epoch), provable after the fact and prevented in the honest case*.

Why distinct: authority is a *live mutable fact in one database*. Everything else — trust classes, rings, failover thresholds, checkout policy — is policy reading that database.

Prior art: Chubby/GFS leases, single-master replication with fencing tokens (Kleppmann), etcd sessions.

Policy surface left open: failover thresholds, checkout admissibility, trust-class assignment, ring widths, effect classes.

Unsafe configs: the lease DB is the fabric's crown jewel — a compromised or stale-restored home can mint a second writer (W6 mitigates rollback, not malice). Any config where the home's durability < the tape's durability breaks epoch monotonicity. Sharpest failure mode: **one operator compromise = fabric-wide write authority**; equivocation proofs tell you it happened, after mail was sent.

**2. Self-certifying delegation-chain kernel — authority is a signed chain, not a row**

Core: a writer's right to append is a verifiable delegation chain rooted in the computer's (owner-derived) key: owner → computer key → site host key → realization. Any site or client verifies a receipt offline, with no home lookup. The home is demoted to relay, cache, and follower — it *witnesses* head receipts but never *issues* authority. Revocation is a signed list, checked as a freshness datum.

Why distinct: the trust root is the owner key, not the home. Removing the home entirely degrades liveness, never correctness — no party can mint what wasn't delegated.

Prior art: Secure Scuttlebutt feeds, KERI, UCAN delegation chains, SPKI/SDSI cert graphs, signed git.

Policy surface: delegation terms, chain depth, re-delegation rules, revocation-list distribution and staleness tolerances.

Unsafe configs: unbounded re-delegation depth; revocation lists served stale (a revoked site keeps writing until the CRL reaches the verifier — the verifier must choose a freshness bound and the kernel can't give it one); a captured owner key is unrecoverable total loss. Failure mode: **with no arbiter, every partition is a branch** — correctness is preserved, ordering is not; "the lease row decides" is gone forever.

**3. Witnessed-checkpoint kernel — detect and prove, never prevent**

Core: kernel = {append-only hash chain, per-head receipts, N independent witnesses who cosign checkpoints after verifying chain consistency}. Writers write freely on any site; equivocation is *guaranteed detectable* as two cosigned checkpoints that cannot both be honest, producing an undeniable proof artifact. Branch-and-switch is the designed response, not an exception.

Why distinct: inverts option 1's posture. Nothing prevents a split-brain; the guarantee is that dishonesty is *provable and attributable*. The security primitive is accountability.

Prior art: Certificate Transparency, Sigsum, Trillian verifiable logs, SUNDR fork-linearizability, CONIKS key transparency.

Policy surface: witness composition and quorum, freshness bounds, which divergences count as compromise vs partition, repair ceremony.

Unsafe configs: witnesses sharing one operator/blast radius (cosigns prove nothing); witnesses that sign without holding history (split-view attacks pass silently); zero-witness configs degrade to option 1 with no arbiter. Failure mode: **detection without enforcement means every partition costs the owner a decision** — and deterrence only works if someone actually reads the proofs.

**4. Replicated-consensus lease kernel — the home is a quorum, not a machine**

Core: the lease row lives in a small Raft/BFT log spanning 3–5 homes/sites; lease moves and primary switches are consensus decisions. Guaranteed property: single-writer safety survives the loss of any minority of homes, and the control plane has no single point of compromise for *honest-but-crashy* faults (full BFT variant covers Byzantine homes).

Why distinct: the arbiter is a protocol, not a process. Blast radius shifts from "the home" to "a quorum".

Prior art: etcd/ZooKeeper/Chubby, Paxos commit, PBFT/BFT-SMaRt, Consul.

Policy surface: quorum membership per computer/org, voter site selection, thresholds per act class.

Unsafe configs: voters on flaky workstations (three laptops asleep = write unavailability); quorum members sharing infrastructure; learner nodes counted in quorums. Failure mode: **solving a non-problem at maximum cost** — one writer per tape needs *safety* (which is cheap) not replicated *liveness*; cross-org quorums convert a governance dispute into a protocol stall. Also: a Byzantine home that waits to vote can wedge the computer forever.

**5. Hardware-rooted kernel — leases only issue to attested stacks**

Core: site identity is a measured-boot identity; writer leases, capability leases, and key custody are bound to attestation quotes (TPM, Secure Enclave, SEV-SNP/TDX); host keys are sealed to the measured stack. Guaranteed property: *an attested stack, not an IP or a password, holds authority* — a compromised OS that alters the host process fails attestation and is refused.

Why distinct: trust root is silicon + manufacturer endorsement. It answers a question no other kernel can: *is the code that holds the lease the code we shipped?*

Prior art: Azure/GCP confidential VMs, TPM-bound keys, Apple Key Attestation, Android ID attestation, Constellation/Edgeless Systems.

Policy surface: accepted measurement sets (the "known-good build" list), attestation freshness, which capability classes need which attestation tier.

Unsafe configs: "temporary" unmeasured stacks (the escape hatch becomes the hole); sealed-key policies not bound to lease epoch (old measurement unseals new lease); trusting the vendor's attestation service blindly — you've moved the trust root to Intel/AMD/Apple's online service. Failure mode: **verifier monoculture** — one stale measurement list silently admits compromised sites fleet-wide; attestation outage = total lease outage.

**6. Universal-capability kernel — everything is an attenuable token**

Core: no lease table, no grant tables as primitive state. Writer authority, leaf admission, controller grants, native-capability leases are *all* unforgeable, attenuable, caveated tokens verified locally at each site. The kernel is just {token format, verifier, attenuation rules, revocation lists}. Guaranteed property: *every act is performed under a token whose caveats cover it* — authorization is self-contained and offline-verifiable.

Why distinct: it erases the doc's distinction between lease rows, grants, site records and controller grants — one object does all of it. Enforcement happens at every site independently; there is no arbiter to be down.

Prior art: KeyKOS/EROS/seL4, Cap'n Proto capabilities, macaroons, Biscuit, UCAN, SPKI.

Policy surface: the caveat vocabulary, minting authorities, TTL curves, revocation propagation.

Unsafe configs: long TTLs — the classic ocap hole: a stolen token is indistinguishable from a delegate until expiry; a caveat vocabulary too weak to express "this computer, this branch, this capability, this action class" (any slack = ambient authority); unbounded delegation depth. Failure mode: **bearer tokens leak by design** — possession is authority, so every confused deputy and every logged token is a live grant. The doc's DB row can be made unreadable; a token already in the wild cannot.

**7. Information-flow kernel — the guarantee is confinement, not authority**

Core: every object, leaf, tape segment and capability carries a taint label in a lattice (owner-only, org-nonconfidential, public, web-derived/untrusted-instruction). The kernel enforces two rules on every site hop and every runtime boundary: data never flows below its label's floor, and web-derived instructions never elevate authority. Guaranteed property: a noninterference statement — no configuration can move confidential data to a site that shouldn't see it, or let screen content widen a lease.

Why distinct: all other kernels police *who may act*; this one polices *where data may flow*, including runtime exfiltration — which leases and tokens cannot see (a duly-leased leaf can still read the guest's memory and ship it out).

Prior art: HiStar, Flume, DIFC (Aeolus), Decentralized Label Model (Myers–Liskov), SELinux/MLS, Android data-safety labels.

Policy surface: the lattice itself, declassification/endorsement rights, taint of each new capability kind, runtime enforcement granularity.

Unsafe configs: declassify rights granted per-principal instead of per-flow (one sloppy site principal leaks everything it touches); labels checked at placement but not enforced in the leaf sandbox (covert network egress); ignored covert channels. Failure mode: **label creep → universal top-secret → users declassify wholesale → the guarantee is law, not practice**.

**8. Effects-gate kernel — the precommitment outbox is the entire kernel**

Core: the kernel guarantees exactly one thing: *no irreversible external effect fires except through a home-durable precommitment record + a single-use actuation token + an idempotent actuator*. Placement, leases, reachability — all policy, all replaceable. The kernel is a two-commit gateway between "state on tape" and "world".

Why distinct: it locates safety at the only place harm is unrecoverable. Partition a tape and you get a branch; fire a payment twice and no branch logic repairs it. This kernel treats every other question (Q1–Q4, Q6–Q8) as negotiable.

Prior art: transactional outbox, sagas, Kafka idempotent/transactional producers, effect systems with handlers, the repo's own precommitment records.

Policy surface: which effect classes require home-durability vs may fire offline, budgets, actuation scopes, retry semantics.

Unsafe configs: **any effect path that bypasses the gate voids the kernel entirely** — the guarantee is all-or-nothing; an "offline-allowed" class that turns out to be irreversible; actuators without real dedup (Gmail send has no idempotency key — you get screenshots as the dedup). Failure mode: it says *nothing* about confidentiality — a malicious site reads everything it hosts while faithfully firing only precommitted effects.

**9. Policy-proof kernel — every act carries a machine-checkable authorization proof**

Core: the kernel is a small *total* policy language (Cedar/Datalog-class: every query terminates) plus one rule — every admission, lease grant, capability use and switch is committed with the evaluation trace that allowed it, so any auditor can replay the decision. "Every configuration is safe" becomes a theorem about the language plus static analysis over policy (e.g., "no policy admits cross-person placement without a site grant").

Why distinct: the kernel doesn't hardcode any security model at all — it guarantees only that decisions are *decidable, recorded, and checkable*, and that forbidden configurations can be *proved absent* rather than tested for.

Prior art: Cedar (AWS), OPA/Rego, Binder/proof-carrying authorization, Datalog authorization (Bingo), XACML as the cautionary tale.

Policy surface: everything — that is the point; the kernel is the evaluator plus the audit obligation.

Unsafe configs: policy builtins with side channels (time, network, entropy) that break totality; policy update without re-running the invariant checks; recorded proofs nobody replays. Failure mode: **expressiveness creep** — the moment someone needs one more builtin and the language inches toward Turing-completeness, "every configuration safe" becomes undecidable and the kernel dies silently, still answering queries.

**10. Verified-machine kernel — the kernel is a proved state machine**

Core: the fabric's core (lease, branch, switch, admission, revocation) is specified as one state machine and machine-proved (TLA+/Ivy/IronFleet-style) to preserve invariants for *all parameters inside a declared bound* — trust sets, thresholds, timing, site counts. Sites run the extracted/verified core; configuration = choosing parameters inside the proven envelope. Guaranteed property: literal theorem — no in-bounds configuration violates the invariants.

Why distinct: "every reasonable configuration is safe" stops being an aspiration and becomes the artifact. Also makes the self-tuning requirement tractable: the computer may only tune parameters *inside the proven polytope*.

Prior art: IronFleet/Ironclad (Dafny), Verdi (verified Raft), seL4, CompCert, TLA+ at AWS/MongoDB, Ivy.

Policy surface: parameters inside the proven bounds — and *only* those.

Unsafe configs: anything outside the adversary model (a spec assuming crash-faults deployed against Byzantine sites is proof of the wrong theorem); new mechanism kinds are by definition outside the envelope; implementation drift (hand-edited TCB) voids the proof while retaining the confidence. Failure mode: **proofs protect the model, not the world** — a spec bug is a guaranteed-safe vulnerability; and verification cost per change collides head-on with the owner's tempo rule.

**11. Consent-ceremony kernel — the owner act is the only mint**

Core: a small fixed vocabulary of authority-minting acts (checkout, switch primary, site enrollment, grant, revoke, ring change, effect precommit) each with *required evidence display and required seats* (passkey session, m-of-n), producing receipts that are the sole source of all delegated authority. Guaranteed property: every authority in the system traces to a replayable consent record — and nothing else can mint authority, including the computer itself.

Why distinct: the kernel's primitive is neither a lock nor a token but a *ceremony*; all other authority is provably derived or provably absent. It is the only kernel that directly answers "why did this happen" with a proof rather than a log.

Prior art: multisig/threshold signing, WebAuthn ceremonies, Android trusted-path confirmation, Safe (Gnosis) spending policies, SSH key confirmation prompts.

Policy surface: seat requirements per act class, evidence templates, standing-policy expressiveness (pre-authorized conditional consent).

Unsafe configs: **standing policies that let the computer mint owner acts** — delegation with no bound is consent laundering, and it is the default failure path because every user eventually grants it; evidence surfaces the computer itself can influence (the computer writes the "is this safe?" text — confused deputy against the owner); session methods that can widen (the doc's email-vs-passkey split is this kernel already half-built). Failure mode: consent fatigue — ceremony becomes theater, and theater is worse than no ceremony because it looks like review.

**12. Fork-first kernel — delete the lease; branches are the normal state**

Core: radical simplification of the doc's own design. No lease, no fencing: any site may always extend its own branch; the *only* controlled objects are the primary ref (owner-act-only) and the effects gate (option 8, mandatory here). Have/want sync and effects digests are the whole protocol. Guaranteed property: *no partition ever stalls availability of writes, and divergence is always convergent-representable* (two tails, one ref, owner picks).

Why distinct: it removes the mechanism everyone else fights to keep safe. Security is relocated entirely to the effects gate and the ref; the tape stops being an authority and becomes evidence.

Prior art: local-first (Ink & Switch), git branching as the default, SSB multi-device same-as problem, branching-tape doc taken to its limit.

Policy surface: which branches may run standing intents, effects-digest display thresholds, branch caps, auto-branch policy.

Unsafe configs: **any path letting a non-primary branch fire effects** — the outside world does not branch; mail sent from a losing tail is sent; branches minting child computers (identity spawning from unmerged state); branch-explosion DoS. Failure mode: continuous divergence — the owner becomes the merge protocol; "simplicity" (the owner's own first criterion) survives the mechanism but dies in operation.

**13. Market/receipts kernel — sites are untrusted contractors paid on proof**

Core: sites hold no trust class at all; they are paid for verified contributions — lease-held hours with cosigned receipts, replicated execution for result-checking, escrowed payment on verified delivery. Guaranteed property: *lying is provable and unprofitable* — the kernel is receipts + replication + settlement, not admission.

Why distinct: it replaces *access control* with *incentive + verification*, which is the only kernel that scales placement to machines no one vouches for (the doc's compute-credits note is this kernel embryonically).

Prior art: BOINC/SETI@home redundant computation, Golem/iExec, Filecoin storage proofs, Truebit/verifiable computation, spot markets.

Policy surface: redundancy factors, pricing, which work classes may ever touch untrusted sites.

Unsafe configs: confidential data — *no economic model repairs a confidentiality leak*, so this kernel partitions the workload: public/verifiable work on the market, everything else on trust classes; redundancy below the collusion threshold; paying for self-reported uptime (the doc already flags this). Failure mode: incentive gaming — spam results, replayed receipts, and a permanent adversarial arms race priced into every burst.

**14. Two-plane kernel — a tiny audited control plane over a free data plane**

Core: hard separation. Control plane: lease rows, primary refs, revocations, grants — a few KB per computer, itself an event-sourced hash-chained log, strongly consistent, minimal TCB. Data plane: bundles, relays, sync, followers — optimistic, untrusted, free to be clever. Guaranteed property: *data-plane components can never mint a control fact*; every authority claim is resolved by replay of the control log.

Why distinct: it's the only kernel whose design criterion is *size of the thing you must trust*, letting everything else be fast, sloppy, and replaceable. It is also the honest version of the doc — the doc already behaves as if this split exists; making it a kernel means no data-plane cache may ever answer an authority question.

Prior art: SDN control/data-plane separation, Kubernetes API server vs kubelets, Spanner's Paxos state machine vs tablet data, GitOps.

Policy surface: the entire data plane.

Unsafe configs: data-plane caches answering authority questions (a stale route = silent split-brain — this is the seductive bug this kernel exists to name); control-plane reads that skip the replay path for speed; a control plane allowed to grow features (TCB bloat). Failure mode: drift — the data plane starts making control decisions ("the follower promoted itself") and the kernel's one guarantee is quietly gone.

**15. Reachability kernel — the only guarantee is an identity-bound channel**

Core: minimal-by-subtraction. The kernel provides *routable identity*: every computer and site reachable by name from anywhere (rendezvous servers, relays, hole-punching), with channels bound to endpoint keys — never to addresses. Guaranteed property: *you are talking to the key you named*. Everything else — leases, placement, grants — is edge behavior.

Why distinct: it treats Q2 (reachability) as the whole kernel and makes the rest a problem for the applications. Deliberately the smallest possible trusted substrate.

Prior art: Tailscale (WireGuard + DERP relays + key-bound node identity), ICE/TURN/STUN, libp2p circuit relay, Tor rendezvous, push-notification transports.

Policy surface: relay operators, direct-connect policy, fallback order, which traffic may transit which relay.

Unsafe configs: relays terminating plaintext (must be end-to-end); accepting relay-assigned addressing (the doc's "identity = authenticated session" is exactly this); single-operator relays as the only path. Failure mode: a *reachable* evil relay is a permanent selective-DoS/MitM if channel binding is weak — and "the system works perfectly while the relay lies" is the worst kind of failure to debug.

**16. Pin-and-migrate kernel — one computer, one site, replicas read-only**

Core: the conservative alternative the doc argues against. A computer's writer is bound to exactly one site; moving is a full export→import→cutover with no overlapping authority window; all other devices are read-only replicas with an outbox of intents. Guaranteed property: single-writer is enforced by *physical singleton*, not by coordination — the simplest possible safety argument.

Why distinct: it is the null hypothesis — the kernel that forbids what the fabric doc wants (live checkout, offline work) in exchange for needing *no lease mechanism at all*. Its value in the option space is showing what you pay for the fabric's flexibility.

Prior art: VM migration (vMotion/Xen live migration), LPAR migration, cold failover, the repo's own existing "cold move".

Policy surface: migration triggers, replica freshness tolerances, outbox semantics.

Unsafe configs: adding "just one exception" for the Mac — the kernel's only guarantee evaporates the moment a second live site exists; it cannot express the owner's offline-work requirement, so a config that tries is unsafe *by construction*. Failure mode: it answers the security question by declining the product question.

**17. Empirical kernel — the invariant checklist + drills IS the mechanism**

Core: meta-option. There is no formal kernel; primitives remain today's (Postgres rows, signed bundles, sessions). "Every configuration is safe" is replaced by "every configuration we run is covered by a drill that can falsify the safety claim" — adversarial drills on d@e as the verifier, the standing questions as the spec. Guaranteed property: none formally; *empirically bounded*.

Why distinct: it refuses the premise that a small kernel with proven properties is achievable for this system, and substitutes continuous falsification — the operating-tempo doctrine applied to the security model itself.

Prior art: chaos engineering, GitOps drift detection, "worse is better", the repo's own drill/keep-set/differential discipline.

Policy surface: the invariant list itself, drill coverage, which invariants are worth hard enforcement later.

Unsafe configs: any configuration not covered by a drill is unsafe and *unknowably* so; drills that can't observe the failure they claim to test. Failure mode: **test coverage mistaken for proof** — the first time a real adversary explores the space, they explore exactly the configs nobody drilled.

---

## Dimensions along which these options differ

- **Root of trust:** home control DB (1) · owner/delegation keys (2) · witness quorum (3) · consensus quorum (4) · silicon/vendor (5) · whoever holds the token (6) · the lattice (7) · the outbox (8) · the policy engine (9) · the proof (10) · the ceremony (11) · the owner as continuous arbiter (12) · the settlement ledger (13) · the control log (14) · endpoint keys (15) · physical singleton (16) · test coverage (17)
- **Safety posture:** prevent (1,4,5,6,7,9,10,11,14,16) · detect-and-prove (3) · deter-economically (13) · absorb-and-adjudicate (2,12) · falsify-continuously (17) · channel-only (15) · effects-only (8)
- **Enforcement locus:** at write time (1,2,4,5,6,10,14,16) · at the effect boundary (8,11) · at data flow (7) · at admission+proof (9) · at the channel (15) · post-hoc (3,13,17) · never enforced, owner-adjudicated (12)
- **Liveness under partition:** writes stall (1,4,9,10,11,14,16) · writes continue as branches (2,3,12) · writes continue locally under token TTL (6) · unaffected (7,8,15,17 in different senses)
- **Revocation model:** row update (1,14) · signed list freshness (2,6) · epoch/consensus (4) · attestation expiry (5) · label/policy change (7,9) · ceremony receipt (11) · ref move (12) · settlement (13) · can't revoke reachability (15) · migrate (16) · drill-driven patch (17)
- **What a compromised home gets you:** everything (1,4-voters,14) · almost nothing cryptographically, but liveness+routing (2,3,15) · leases to honest stacks only (5) · minting for its own scope (6,9) · the precommitment gate (8) · nothing but policy trust (7,10,11,12) · the escrow (13)
- **Equivocation:** prevented by epoch (1,4,10,14) · impossible to prevent, designed-detected (3,12) · provable-by-receipts (2,5,6) · outside the guarantee (7,8,9,11,13,15,16,17)
- **How "every config safe" is established:** invariant design + drills (1,2,6,12) · decidable policy + static analysis (9) · machine proof over parameters (10) · enumerated-by-tiny-mechanism (8,15,16) · witnessed audit (3) · attestation rules (5) · lattice soundness (7) · economic bounds (13) · separation boundary (14) · ceremony completeness (11) · empirical coverage (17)
- **Fit to self-tuning:** the computer may safely tune policy inside bounds without re-deriving the kernel in (5,8,9,10,11,14); tuning *is* the kernel in (7,9,12); unsafe to tune freely in (1,4,6,13); nothing to tune in (15,16,17)
- **Cost of the guarantee:** arbiter SPOF (1,14) · no arbiter → branch-normal (2,12) · witness ops (3) · quorum ops+governance (4) · vendor dependence+TCB breakage (5) · bearer-token leakage (6) · label maintenance+covert channels (7) · all-or-nothing gate (8) · totality ceiling (9) · proof cost per change (10) · consent fatigue (11) · incentive arms race (13) · MITM-by-relay risk (15) · declined product requirements (16) · unfalsified space (17)

Notable tensions worth the panel's attention: options 1/14 vs 2/12/15 disagree on whether a *live arbiter* should exist at all; options 5 vs 6 vs 7 disagree on whether authority, possession, or data-flow is the thing to secure; options 3/13/17 share a "prove it happened" posture that contradicts the prevention posture of 1/9/10/11 — and option 8 argues the whole argument is misplaced because only effects are unrecoverable.
