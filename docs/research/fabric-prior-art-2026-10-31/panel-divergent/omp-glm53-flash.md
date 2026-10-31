Working...
I read the fabric design (§0, §1–§4, §5, §9, §10), the branching-tape doc (§0–§2), and the section map of both. Divergent output follows. Each option is a candidate *kernel* — a small set of primitives with guaranteed properties — plus its policy surface, ancestry, unsafe configurations, and sharpest tradeoff. Historical notes are included per the lens; recurrence across decades is itself evidence about which primitives keep getting re-invented vs. rejected.

---

## Candidate kernel architectures for the resource fabric

**1. Home-anchored lease writer (the doc's own proposal, stated as a kernel: {append-with-epoch-CAS, primary-ref, branch-open, signed site record, outbound session}).**
- Core idea: authority for a branch lives in a durable table at one home; the lease has no expiry; failover is branch-by-policy.
- Genuinely different from the others: the root of trust for *authority* is a named party (the home), not a protocol or a possession.
- Ancestry: GFS chunk leases (2003) → Chubby epochs (2006) → Raft leader leases → the Kubernetes node-lease model. Sprite's write-sharing leases (late 1980s) are the same primitive at workstation scale, and Sprite's authors eventually abandoned leases for exactly the case Choir re-considers: an offline laptop holding a lease was fine, an offline *server* holding one was fatal.
- Policy surface: trust classes, checkout policy, failover thresholds, branch retention, per-computer caps.
- Safest-until (unsafe configs): home backup restore (epoch rewind, branching-tape case 6); two homes for one computer (nobody fences); org sites that want to admit work without talking to the home (admission state forks).
- Sharpest tradeoff: the home is simultaneously the authority root, the relay, and the archive. Compromise one, lose all three. The design accepts this for simplicity.

**2. Emergent-leader replication group (no lease table; each computer is a small rotating Raft/PacificA group of sites, leader = writer).**
- Core idea: the "writer" is the leader of a group chosen at checkout time from eligible sites; leadership changes are protocol-driven, not owner acts.
- Genuinely different: split-brain is impossible *by the protocol*, not by fencing; there is no authority table to restore from backup wrong.
- Ancestry: Raft (2014), PacificA (Microsoft, 2008), Spanner per-keyspace groups, Kafka partition leadership.
- Policy surface: *the entire policy surface is group membership* — who is eligible to sit in computer C's writer group is where all access control goes.
- Unsafe configs: two corrupt/colluding eligible sites own your tape forever; a witness-that-is-also-a-workstation creates silent dependencies between employees' computers; budget explosion (3× the write amplification of option 1).
- Sharpest tradeoff: protocol-guaranteed authority in exchange for making every workstation a potential peer of every other. The owner's stated requirement ("employee A's VM never on employee B's workstation") fights the replication-group model directly.

**3. Key-possession writer (no lease, no epoch; the tape's writer key IS the authority.**
- Core idea: whoever holds a per-computer writer key may append; moving the writer = moving the key; epochs are optional receipts for audit, not authority.
- Genuinely different: zero coordination in the happy path; the handoff is atomic in a key, not in a database transaction at a third machine.
- Ancestry: Hypercore/Dat (single-writer append-only logs keyed by possession), Secure Scuttlebutt, Signal's key-as-identity; the ancient UNIX "uid as authority" mistake and its modern corrections.
- Policy surface: key custody (who generates, who holds, who may duplicate), whether possession alone confers authority or possession plus a home countersignature does (this fork is the whole design).
- Unsafe configs: key theft (the only defense is revocation, which needs a coordinator — so the "no coordinator" claim quietly fails); a checkout crash mid-key-move recreates branching-tape case 6 with no epoch to arbitrate; org sites joining/leaving the key's trust set need the same statement machinery as option 1 anyway.
- Sharpest tradeoff: elegant, un-auditable. "Possession is authority" has been reimplemented and re-abandoned repeatedly (capability systems, bearer bonds, betting-site wallets); every cycle ends with someone adding a registry back.

**4. Local-first default (the writer's primary is the owner's personal device, permanently; the home is a relay and archive only).**
- Core idea: the fabric doc's mode L *generalized to the default* — the inverse bundle route becomes the primary data path and no handback exists; "leading" is the steady state.
- Genuinely different: it does not try to solve the partition problem; it *ranks* the partition — the owner's device is presumptively right, the org's copy is a follower.
- Ancestry: Coda (1987–1998, hoarding/volume version vectors), Bayou (1995, primary-copy reconciliation), local-first software (Kleppmann et al., 2019).
- Policy surface: what must never be primary-on-device (public_platform computers per branching-tape case 8), what the home keeps for recovery.
- Unsafe configs: 24/7 org computers (the newspaper) on sleeping laptops by accident; an owner device that silently wins a partition it should have lost (the owner closes the laptop un réseau flaky); DR: the org's incident response now goes through an employee's power button.
- Sharpest tradeoff: sovereignty vs. organizational operability. Historically, Coda died on this exact axis, AFS won by refusing the offline-primary case, and the industry came back 30 years later willing to pay the price for consumer sync.

**5. Convergent tape (CRDT kernel; the no-merge rule is replaced by a convergence guarantee).**
- Core idea: multi-writer by construction; the machine's replay must be a confluent function of the event multiset; branches converge automatically when tails are exchanged.
- Genuinely different: eliminates the choice-of-primary problem categorically rather than managing it.
- Ancestry: Bayou's merge procedures, CALM theorem (Hellerstein/Alvaro, 2010), Automerge, Riak, Git's merge semantics — and Git's *recomputation* of merges as the reason the owner's instinct "no merge, publication only" has a lineage.
- Policy surface: which event kinds are commutative (the entire type system of the machine becomes available policy); who is allowed to write at all (back to option 1's problem, unsolved by convergence).
- Unsafe configs: any non-commutative event kind — `writer_handoff`, payment confirmations, capability *revocations* (revocation on a losing tail is the classic Bayou/Automerge wound) — reintroduces an order rule, piecewise.
- Sharpest tradeoff: replay determinism must be re-proven for every event kind ever added, by everyone forever. The tape's core guarantee ("replay the tape, get the same computer") becomes a distributed type obligation. Convergence was almost universally abandoned in systems where the events are *acts, not data*; Choir's tape is acts.

**6. Fork-choice tape (cryptoeconomic/cryptographic weight selects primary).**
- Core idea: anyone may append to any branch; a deterministic, publicly-checkable weight rule (signed endorsements, most-event, most-stake) names the primary with no human act.
- Genuinely different: authority selection is an *arithmetic* property of the log, not a property of any record, table, key, or leader.
- Ancestry: Nakamoto consensus, longest-chain Ethereum, Casper FFG fork-choice, IPFS/IPNS; the "heaviest chain" rule is also (re)independently in CRDT conflict resolution (max-register) — this option is that rule without trusting sites to be honest.
- Policy surface: who may add weight (this becomes a consensus/payment question), what "endorsement" means, branch retention economics.
- Unsafe configs: weight is cheap to forge (spend-2-cents-own-the-computer's-history); external irrevocable effects fired on a tail that later loses — the double-spend problem, unbounded; auto-tuning policy (owner Q10) on a branch whose weight is uncertain.
- Sharpest tradeoff: removes the owner act (primary switch is automatic) at the price of always having a nonzero chance the *world-affecting* branch was the losing one. Bitcoin pays with electricity; a home paying with signature weight is showing the clearest historical echo — this is what happened to DHTs based on similar token schemes.

**7. Plan 9 attach kernel (namespace synthesis; no "home", no "reachability" problem by construction).**
- Core idea: every machine serves the same global persistent namespace; a "computer" is a subtree; clients (phones, other computers) simply *attach*; route and authority are properties of the namespace, not of a site.
- Genuinely different: rejects the premise "every computer has a home" — the home network's *cluster* is the namespace; reachability is DNS-like name resolution.
- Ancestry: Plan 9 / 9P (Pike et al., 1990s), Plan 9 from User Space, the synthesis in Coda/AFS's per-file mount semantics.
- Policy surface: namespace shape per person, per org, per role; ACLs attached to subtrees; which site may serve which subtree.
- Unsafe configs: orgs that are not in one namespace topology (site enrollment across org boundaries produces subtree forking or mirroring, both unbounded); mobile-weak jurisdiction — a namespace subtree that "is" one laptop's state is exactly the crossorigin that no design solves; the "employee A on B's workstation" line requires cross-subtree ACL semantics nobody has a clean model of.
- Sharpest tradeoff: the cleanest single-organization design in the list; the worst multi-organization design in the list. Plan 9's own history: beloved technically, never survived the federation test.

**8. Two-level offer scheduler (the kernel is the offer protocol; placement is desk policy, always).**
- Core idea: sites publish offers; desks (management) accept and bind; the kernel guarantees only that an offer is honest and binding once accepted.
- Genuinely different: separates *who decides placement* (the desk) from *who has the resources* (the site) as a protocol primitive rather than a policy.
- Ancestry: Mesos (2010), Borg, Kubernetes scheduler extender, HTCondor's ClassAds matchmaking, Nomad's constraint evaluation.
- Policy surface: offer pricing, offer filtering, admission DSL on the site side (the doc's §1.2 admission statements become policy the kernel exposes an API for but doesn't interpret).
- Unsafe configs: desks that hoard offers; sites that lie in offers (the kernel's only recourse is “stop believing that site”, which is a revocation problem, not an offer problem); fairness across persons when sites are privately owned (the_mesos model assumes *a* cluster, not *many* owners).
- Sharpest tradeoff: maximum placement flexibility, minimum provable safety — the negotiation is the system, and negotiation has historically resisted proof. Mesos lost to Kubernetes in industry largely because "the negotiation is the system" turned out to be un-ownable.

**9. Capability kernel (every resource is a capability; leases, epochs, grants, rings, controller grants are all specializations of mint/attenuate/revoke).**
- Core idea: the kernel has exactly two primitives, mint-capability and revoke-capability, and everything else (the writer's authority over a branch, the right to run a VM on a site, a microphone lease, a controller grant) is a capability in scope.
- Genuinely different: no authority table *and* no protocol; authority is carried forward in unforgeable references, and revocation is a separate, explicit mechanism you must design.
- Ancestry: EROS/KeyKOS (Shapiro), Amoeba, seL4, Tahoe-LAFS, Macaroons/Biscuit tokens, WebAssembly's capability model.
- Policy surface: attenuation shapes, expiry intervals, ring definitions, capability-scoping per site class — plus a full "who can mint what" policy, which ends up being most of the access-control design.
- Unsafe configs: revocation latency (a stolen/leaked capability stays valid until the next revalidation cycle — reintroduces a heartbeat, which reintroduces the home); audit (answering "who did this" requires a log of capability use, which erodes back to the tape-as-log with write authority still unresolved).
- Sharpest tradeoff: the capability segregation is beautiful for untrusted-site placement and data-containment (the owner's key insight) but is historically weakest exactly at Q5 — an irreversible actuator gated by a *revalidated* capability is "capability" only in name.

**10. Attestation-based site trust (no trust classes; a site's standing is a fresh signed measurement of its running software).**
- Core idea: before placement, the target site presents a measurement (hypervisor + host binary boot chain); the kernel only places work on sites whose measurement matches a policy-signed manifest.
- Genuinely different: answers "what is this machine running" rather than "who operates this machine"; the two questions are orthogonal and must be composed.
- Ancestry: TPM remote attestation, Intel SGX, Android Play Integrity, Windows measured boot, macOS sealed system volume + notarization; Terra/Farsite used attestation specifically for fabric-style work placement (Farsite was P2P CFS shaped like this fabric's workstation ring).
- Policy surface: the measurement manifest (which releases are admissible), what happens on measurement mismatch (this is Q8 offboarding and release skew, morphing into one policy).
- Unsafe configs: sites that can't meaningfully attest (Macs (implicitly), Windows Home, Linux without a TEE, phones); collusive org admins whitelisting a tampered build; mismatch-handling policy that becomes a fork in authority (does a mismatch pause the tape, failover, or branch?).
- Sharpest tradeoff: strong against malware, silent against the owner-principal question. Historically, attestation has always needed to be *paired* with admission statements (option 1's signed grant) — Farsite did exactly this and still lost to reputation-of-operator systems in practice.

**11. Epidemic replication + notary quorum (availability-first; the kernel is gossip and a signed quorum requirement on irrevocable acts).**
- Core idea: sites replicate the tape epidemically to any peer that will accept; "the home" is just the most complete replica; irreversible external effects additionally require a k-of-n signed endorsement set before firing.
- Genuinely different: forgoes "one writer" as an axiom; irrevocability is protected not by authority but by *preconditions of acknowledgment*.
- Ancestry: Bayou/gossip protocols, Epidemic algorithms (Demers et al., 1987), Hypercore's replication rates, Bittorrent DHT, IPFS pinning-serv quorums.
- Policy surface: endorsement-set shape (who are acceptable notaries — the org's cluster, the owner's other computers, a platform notary service), how tails converge, what "current" means for a read.
- Unsafe configs: a colluding notary set; slow convergence making "did the action go out?" ambiguous under partition (endangering Q5, the most safety-critical open question); mode L-like personal devices ending up as notaries for org work (A's Mac endorsing B's send-mail is exactly what the owner wants blocked).
- Sharpest tradeoff: giving up "one writer" buys availability at the price of a *semantic* question — what does the tape mean — that no amount of protocol engineering answers. Historically, epidemic replication has been the right answer for calendar/mail/contact sync and always the wrong answer for workflow systems.

**12. Verified-kernel policy-DSL monolith (the "small kernel with proven properties" taken literally; all of policy is a typed, terminating DSL the kernel interprets).**
- Core idea: the kernel exposes append, place, mint/revoke, observe, propose, each primitive proven safe; everything that varies across organizations — leases, trust classes, rings, placement rules, capability gates, grant shapes — is policy in the DSL; "safe in every configuration" becomes "every well-typed policy is safe".
- Genuinely different: makes the kernel/policy split a *type-theoretic* guarantee rather than a design aspiration; the config space is enumerable in principle.
- Ancestry: seL4's proof discipline, IronFleet, Verdi, Datalog with stratification, OPA/Rego (and OPA's known "safe query" limitation as the warning), Liquid/effect types, fathomed policy languages.
- Policy surface: everything; that is its selling point and its pathology.
- Unsafe configs: anything that escapes the DSL (a native call, a model provider, a manual CLI against the home — every escape hatch is a configuration unsafety the type system cannot see); a well-typed policy whose *semantics* is undesired ("employee B's workstation may host A's computer when A's calendar is enabled" type-checks and is exactly what the owner said no one wants).
- Sharpest tradeoff: the DSL becomes a second kernel with a second authority. In practice, every "policy language organization" has grown escape hatches (OPA, Rego, AWS IAM policy conditions); the escape hatches are where the incidents live.

**13. Refusal-of-sharing kernel (single-tenancy per site as a hard primitive; multi-org coexistence achieved by not sharing machines at all).**
- Core idea: the kernel's only admission rule is "this site serves exactly one principal"; there is no cross-person placement in the mechanism at all; orgs federate exclusively through publication/adoption.
- Genuinely different: instead of engineering cross-trust, it deletes the cross-trust problem; the owner's cited example (A's VM on B's workstation) is not a policy case, it is unrepresentable.
- Ancestry: single-tenant dedicated hosts (EC2 dedicated), per-tenant cells in OpenStack, ESXi farms, Matrix homeserver federation (single-tenant servers federating), the original "workstation = one user" model that distributed OSes of the 1980s (Sun's cluster, Apollo Domain) built on — and whichMultiprocessor shared machines eventually broke.
- Policy surface: almost none in the kernel; nearly all policy social/contractual (who is allowed to enroll which machine into which org); efficiency policy ("may my spare Mac join the org pool?") is a single boolean the kernel already enforces at the enrollment record.
- Unsafe configs: the entire efficiency argument in fabric doc §1.4 (burst on idle employee workstations) becomes impossible by construction; org-shared capabilities (the corporate comms computer using an employee's screen) have no placement story; per-employee-tenant machines with two humans on one Mac (shared family computer, hot desk) exceed the primitive.
- Sharpest tradeoff: deletes the hardest access-control problem and pays for it by deleting the fabric's own economic reason to exist. Historically, "one principal per machine" was the 1980s assumption, replaced in the 1990s by timesharing, replaced in the 2010s by containers, and it has been "replaced" each time by a layer of shared-infrastructure trust — each replacement bought efficiency at the cost of exactly the isolation the owner wants.

**14. Contract-ledger mechanism (org lifecycle is a first-class primitive: dated grants, dated releases, self-expiring tickets).**
- Core idea: instead of an ownership model for who owns what site/computer, the kernel has a primitive "signed commitment with expiry" that is the *only* way work, data, capability or placement persists; org onboarding is minting a family of grants; offboarding is refusing renewal; "who owns what" is derivable from which grants are live.
- Genuinely different: models Q8 (lifecycle) as the *time dimension* of authority rather than a separate ownership table; policy surface is the calendar, not the ACL.
- Ancestry: Kerberos tickets, X.509 chain validity windows, JSON Web Tokens with `exp`, AWS STS temporary credentials; the Bayou/Lease lineage again, at the relationship level; the "warrant canary" is the negativespace cousin.
- Policy surface: default lifetimes per grant class (site enrollment, checkout, controller grant, native capability lease, org membership), renewal policy, what happens at expiry (the only real question).
- Unsafe configs: lazy renewal (a stale grant that nobody notices is still live — the ticket-expiry problem in every token system ever); long-lived "emergency" grants minted during an incident; org-shared objects that outlive the org's right to them (data retention vs. grant expiry is a real policy conflict this mechanism surfaces rather than solves).
- Sharpest tradeoff: turns "forgot to offboard" from an unbounded exposure into a bounded one — but every lease-based design has (Chubby, Kerberos, IAM roles) shipped with an escape hatch for long-lived credentials because the operational pain of expiry is real.

---

## Dimensions along which these options differ

1. **Root of authority (Q1).** A named party (1), a protocol (2), a possession (3), a presumptive owner device (4), an arithmetic rule (6), an unforgeable reference (9), a measurement (10) — or the problem is deleted (5, 11, 13).
2. **Split-brain posture.** Prevented by fencing (1, 12), prevented by protocol (2), made harmless by convergence (5), made ambiguous resolved by market/weight (6, 11), made impossible by ownership (3, 4, 13).
3. **Trust granularity for sites.** Classed labels (1, 4), signed statements per relation (1, 8), measurement (10), capability in-hand (9), quorum of peers (11), refusal to share at all (13).
4. **Where policy lives and how much the kernel "understands" it.** In a dict/table at the home (1, 3, 8), in a typed DSL the kernel interprets (12), in protocol semantics itself (2, 6, 7), in the shape of capabilities (9), in time (14), almost nowhere (13).
5. **Access control mechanism (Q3).** Statement-based per-relation ACL (1, 8), capability attenuation (3, 9), role/tenant walls (2, 13), token expiry (14), no mechanism by construction (6, 11).
6. **Offline posture.** Offline-as-defect-to-fence (Sprite/GFS/1), offline-as-normal-state (4, 11), offline-forbidden-for-irrevocable-acts (most options' Q5 answer), offline-as-unrepresentable (13).
7. **Coordination cost vs. proof cost.** Heavy protocol, light reasoning (2, 6, 10), light protocol, heavy type/proof (5, 12), no leader, medium everything (11), and the classical compromise (1, 8, 14) that has historically survived outside academia.
8. **Historical recurrence.** Writer-lease/epoch reappears every decade (Sprite → GFS → Chubby → Raft) because the alternative keeps being found unprovable in the general case. Deletion of the writer-secondary problem (5, 6) reappears every decade as an optimization and is abandoned when events are *acts, not bytes* — which is exactly Choir's situation and the reason the owner's no-merge instinct has a lineage worth taking seriously.
