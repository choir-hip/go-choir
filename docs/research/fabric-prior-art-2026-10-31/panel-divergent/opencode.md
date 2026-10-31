
> build · particle-ai/particle-1
→ Read docs/design/resource-fabric-2026-10-31.md [limit=200]
$ ls docs/design/ | head -50
agent-tree-budgets-2026-10-21.md
app-agent-network-policy-2026-10-21.md
app-design-bar-2026-10-26.md
app-desk-compute-capsule-2026-10-21.md
app-package-carries-app-desk-2026-10-21.md
app-packages-replan-2026-10-24.md
append-receipt-trim-2026-10-31.md
branching-tape-2026-10-31.md
bundle-authenticity-2026-10-31.md
capsule-long-exec-2026-10-15.md
choir-data-system-frame-2026-10-30.md
choir-intro-showreel.html
ci-node-b-build-2026-10-24.md
ci-prove-once-2026-10-24.md
ci-runner-slots-2026-10-24.md
claude-delegate-slice1-2026-10-14.md
computer-history-restore-audit-2026-10-30.md
computer-update-architecture-rnd-2026-10-24.md
data-system-build-sheet-2026-10-30.md
delegate-one-round-build-2026-10-21.md
density-d2-snapshots-survive-deploys-2026-10-26.md
density-d6-page-cache-return-2026-10-26.md
egress-grant-object-2026-10-21.md
email-sign-in-2026-10-31.md
engineering-network-grants-2026-10-10.md
file-object-2026-10-24.md
fp10-snapshot-levers-and-host-slab-2026-10-22.md
guest-tool-layers-2026-10-31.md
layered-guest-image-2026-10-31.md
library-package-propagation-2026-10-26.md
object-graph-redesign-2026-10-30.md
object-storage-2026-10-30.md
offhost-weekly-prune-node-a.patch
one-tape-durability-2026-10-31.md
outboard-capsules-arch-11-12-2026-10-22.md
progressive-delivery-2026-10-26.md
recoverable-changes-sr7-2026-10-24.md
research-more-goes-to-research-2026-10-21.md
resource-fabric-2026-10-31.md
secrets-vault-design-2026-10-28.md
small-object-storage-2026-10-31.md
tape-group-commit-2026-10-31.md
tasks-precommit-icrl-2026-10-24.md
texture-evidence-embeds-te-2026-10-25.md
texture-holds-the-trajectory-2026-10-25.md
texture-spec-follows-owner-2026-10-25.md
trace-segment-log-2026-10-22.md
u4-stack-rebase-build-2026-10-26.md
visual-evidence-slice-2026-10-15.md
vsock-control-plane-and-nic-removal-2026-10-22.md
> build · particle-ai/particle-1
→ Read docs/design/resource-fabric-2026-10-31.md [offset=200, limit=280]
$ wc -l docs/design/resource-fabric-2026-10-31.md docs/design/branching-tape-2026-10-31.md
1274 docs/design/resource-fabric-2026-10-31.md
  427 docs/design/branching-tape-2026-10-31.md
 1701 total
> build · particle-ai/particle-1
→ Read docs/design/branching-tape-2026-10-31.md [limit=180]
→ Read docs/design/resource-fabric-2026-10-31.md [offset=480, limit=400]
> build · particle-ai/particle-1
→ Read docs/design/resource-fabric-2026-10-31.md [offset=880]
> build · particle-ai/particle-1
Read-only pass complete: I read `resource-fabric-2026-10-31.md` (§0, §1–§11) and `branching-tape-2026-10-31.md` (§0–§3). Nothing modified. Below, "the doc" means the resource-fabric design; its current commitments are treated as *one* option (option 1), not as the frame.

The generative move: the document fixes four primitives (site, home, writer lease with no expiry, one tape per branch with no merge). Each option below keeps, replaces, splits, or dissolves a different subset of those four, and derives a different guaranteed property from what remains.

---

## Candidate kernels

**1. Home-as-notary: fenced lease + precommitment (the doc's own kernel, stated as one option).**
*Core:* primitives are a site (host key), a `(computer, branch) → epoch` lease held by the home, an append CAS, and a home-durable precommitment record that gates irreversible effects. Guarantees: at most one writer per branch per epoch; no external effect without a durable prior record; equivocation becomes a signed proof.
*Different because:* authority is a *single* mutable row at the home, and safety is bought with a round trip before any send.
*Policy left open:* who may check out, trust classes and cross-person site grants, failover thresholds, branch caps, which effect classes may fire offline.
*Prior art:* Chubby/ZooKeeper sessions, Kafka controller epochs, fencing tokens (Kleppmann), Corda notary, Spanner's lease-plus-clock.
*Sharpest failure:* it guarantees nothing about the *local-durable* window — a Mac on a plane keeps writing to a tail the home has never seen (§2.3, W2), so "safe" configurations still silently hold divergent state; and a host key the home cannot reach is unfenceable until reconnect (W4). Unsafe configuration: any policy that allows an offline effect class.

**2. Mobility kernel: there is never a second instance, only a move.**
*Core:* primitives are snapshot, restore, a location-independent computer address, and a quarantine flag on the source before the target boots. No lease, no epoch, no branch: single-writer holds *by construction* because exactly one live realization exists at a time.
*Different because:* it replaces exclusion-by-token with exclusion-by-lifecycle; "checkout" becomes "migrate", and the hard problem moves from fencing to proving the source is dead.
*Policy:* move windows, who may initiate, whether moves are warm or cold, quarantine timeout.
*Prior art:* vMotion/live migration, CRIU, Sprite process migration, Plan 9's location transparency, textbook stop-copy-start.
*Sharpest failure:* a restored old snapshot is a resurrection, not a move — the moment a backup is booted (W6, branching-tape case 6) you have two instances and no primitive that can tell you. Also: no offline+online simultaneity at all, which deletes the doc's main use case (case 1).

**3. Witness kernel: detection instead of prevention.**
*Core:* primitives are an append-only Merkle log per computer, signed checkpoints, and gossip to a witness set. Writer identity is a key, not a lease. Two heads at one sequence are an *equivocation*, provable and publishable.
*Different because:* it removes the home's authority entirely; the home becomes one witness among several, and the guarantee is "no undetected fork" rather than "no fork".
*Policy:* witness quorum, which equivocations auto-freeze a branch, disclosure rules.
*Prior art:* Certificate Transparency, Sigstore/Rekor, Scuttlebutt, gossip logs, Bitcoin's fork-choice-as-policy.
*Sharpest failure:* the effect fires before the detection arrives. For mail, payments and org posts this is the wrong side of the guarantee, and offline double-send becomes a *detectable* rather than *impossible* event. Unsafe configuration: any at-most-once effect requirement.

**4. Consensus kernel: each computer is a small Raft group.**
*Core:* primitives are replicated log, leader election, quorum commit, membership change. The writer is the leader; the home is a voting member; branches are logs.
*Different because:* it makes durability and uniqueness the *same* mechanism (quorum), so there is no separate "home-durable" concept — commit *is* durable.
*Policy:* replica placement and geography, quorum size, learner membership.
*Prior art:* Raft/Paxos, etcd, CockroachDB ranges, Consul.
*Sharpest failure:* a laptop alone has no quorum, so the offline-first use case is structurally impossible; a 20-person org pays 3 replicas × 200 computers in storage and cross-site latency; and "home failover" becomes "reconfigure consensus", the most dangerous operation in the system.

**5. CRDT kernel: convergent state, with a separate linear gate only for effects.**
*Core:* primitives are CRDT objects, vector clocks, causal delivery — plus one narrow linear service (the effect gate) for irreversible acts. State converges without any lease; only effects need exclusion.
*Different because:* it dissolves the single-writer rule for *state* and keeps it only where physics demands it.
*Policy:* which object kinds are CRDT vs linear, per-kind merge semantics, effect-gate quorum.
*Prior art:* Automerge/Yjs, Dynamo, Bayou, Riak, collaborative editors.
*Sharpest failure:* the tape is a hash chain, and a CRDT is not a chain — replay, audit, Texture canonical writes and the keep set all assume order. Semantic conflicts (two Textures, two mail drafts) have no convergent answer, so "converged" can mean "silently lost intent". Unsafe configuration: any canonical-write or evidence surface.

**6. Object-capability kernel: authority is a reference, not a name.**
*Core:* primitives are unforgeable caps, attenuation, revocation by membrane. Sites, computers, capability leases, and relays are all caps; delegation is passing a cap; there are no ACLs and no ambient authority.
*Different because:* it makes "who may" unrepresentable — the only question is "do you hold the cap" — which is the smallest possible authority primitive.
*Policy:* who mints roots, membrane placement (i.e., where revocation is enforced), cap storage in hardware.
*Prior art:* E, Cap'n Proto, Fuchsia, seL4, Tahoe-LAFS, object-capability OSes.
*Sharpest failure:* org requirements are inherently identity-and-relation shaped ("no employee A work on employee B's workstation", "org RBAC over computers", §1.2, §7.3). Caps can't answer "who in the org", revocation of a copied cap needs a membrane that is functionally a home, and cap sprawl is unauditable without a central registry.

**7. Identity kernel: Zanzibar-style relation tuples with a consistency token.**
*Core:* primitives are `(subject, relation, object)` tuples, rewrite rules, and a zookie returned by every write. Admission is a `Check`; revocation is a tuple delete.
*Different because:* it makes the *relation graph* the single authority and treats the tape as data, not as the security substrate.
*Policy:* the entire person/org/site/computer space, expressed as tuples — this is the option with the richest policy surface.
*Prior art:* Google Zanzibar, OpenFGA, SpiceDB, plus Cedar/OPA above it.
*Sharpest failure:* it's a second state authority and a network dependency on the admission path — an offline Mac cannot check, and a stale zookie is a stale authorization. Recursive group membership also creates transitive authority nobody drew (§7.3 "who may act as Alice").

**8. Edge-policy kernel: signed policy bundles evaluated at the site, home reconciles only.**
*Core:* primitives are a versioned signed policy bundle, a local decision point on every site, and a decision receipt pushed home. No online check.
*Different because:* it optimizes for revocation *latency being visible* rather than revocation being instant, and it makes offline admission possible.
*Policy:* everything — the kernel guarantees only "a decision names a bundle version".
*Prior art:* OPA/Cedar at the edge, MDM profiles, Kubernetes admission webhooks, mobile device policy.
*Sharpest failure:* bundle staleness is the security model. A revoked employee's workstation keeps admitting from yesterday's bundle, and the propagation delay *is* the exposure window. Also: authorization is not mutual exclusion — both sides can be "allowed" to write.

**9. Linearity kernel: static types prove single ownership.**
*Core:* primitives are a linear (move-only) writer token, session-typed protocols between site and home, and effect rows. The verifier proves at most one writer token per branch, statically.
*Different because:* it moves the guarantee from runtime leases to compile time, so the common path needs no coordination at all.
*Policy:* which protocols exist, which principals may hold which token types, re-issuance rules.
*Prior art:* linear logic, Rust ownership, session types, Nomos, effect systems, seL4's cap derivation.
*Sharpest failure:* crashes and restarts require runtime token re-issuance, which is precisely where the static guarantee evaporates — and a self-modifying system that generates code cannot stay well-typed without a proof checker in the loop.

**10. Microhypervisor kernel: the TCB is isolation and endpoint identity, nothing more.**
*Core:* primitives are address space, vCPU, vsock endpoint identity, measured launch. All fabric semantics (lease, home, grants) are ordinary distributed software above the hypervisor.
*Different because:* it refuses to put fabric semantics in the kernel at all; the kernel's only job is that two tenants cannot touch each other and identity comes from the endpoint, never an address.
*Policy:* images, device passthrough, network grants.
*Prior art:* seL4, Xen, Firecracker, Hyper-V VBS, Apple Virtualization, Kata.
*Sharpest failure:* writer uniqueness becomes a userspace convention with no kernel backing, and the operator-visibility problem (org work on Bob's Mac) is untouched because the *host* still sees the guest. No story for phones/browsers.

**11. Global-log kernel: one total order for the whole fabric; every tape is a projection.**
*Core:* primitives are a sequencer, a global append, and per-computer filters. Leases, grants and revocations are just events in the one log.
*Different because:* it makes equivocation impossible by construction and auditing trivial, at the cost of locality.
*Policy:* filters, projections, retention, cross-org visibility.
*Prior art:* Kafka + compaction, FoundationDB's txn log, Datomic, event sourcing, Ripple.
*Sharpest failure:* the sequencer is a global SPOF and a global latency floor; an offline laptop cannot append; and one total order co-mingles orgs' metadata, which is itself a leak. "One tape per computer" degrades to a view.

**12. Tuple-space kernel: work is claimed, not assigned.**
*Core:* primitives are `write`, `read(pattern)`, `take(pattern)` in scoped spaces. Placement is a `take`; exactly-once is atomic take; results are written back.
*Different because:* it decouples in space and time and makes admission a data operation rather than an authority decision.
*Policy:* space topology, pattern visibility, lease/visibility timeouts.
*Prior art:* Linda, JavaSpaces, Redis streams, NATS JetStream, SQS, Erlang ETS.
*Sharpest failure:* no per-branch chain, so the tape's history must be reconstructed elsewhere; visibility timeout is the TTL problem reborn (a slow worker's take expires and a second worker takes it); trust classes are not first-class, so "this tuple may not go to that site" needs an overlay.

**13. Market kernel: placement and honesty are priced.**
*Core:* primitives are offer, bid, bond/escrow, receipt, credit balance. The owner's limits are budgets; a site's honesty is its bond.
*Different because:* it is the only option where overcommit and untrusted sites are handled by incentive rather than by exclusion.
*Policy:* prices, bond sizes, credit issuance, dispute adjudication (and that adjudicator is a hidden authority).
*Prior art:* BOINC credits, Filecoin/Storj proofs, spot markets, internal chargeback (§1.4's credits proposal).
*Sharpest failure:* a bond is not a fence. Against a Byzantine site, a buggy scheduler, or a self-dealing owner, the guarantee is statistical, and drills prove nothing about a rational adversary who is willing to lose the bond.

**14. Federated-homes kernel: many homes, state resolution.**
*Core:* primitives are home servers, per-computer event DAGs, state-resolution rules, server ACLs. Any home can serve any computer.
*Different because:* it makes availability a federation property and accepts a *rule-based* reconciliation as the conflict primitive.
*Policy:* which homes federate, per-computer ACLs, resolution rule version.
*Prior art:* Matrix, XMPP, SMTP, Usenet, ActivityPub.
*Sharpest failure:* state resolution *is* merge. Two branches reconcile by algorithm rather than by owner act, which directly contradicts the doc's "no merge" and "switching primary is an owner act". Cross-org metadata leakage, and home compromise is total.

**15. Personal-root kernel: the owner's device is the only authority.**
*Core:* primitives are an owner-held root key, a device certificate graph, relay mailboxes and store-and-forward queues. No home.
*Different because:* it removes the third-party authority entirely; reachability becomes a mailbox, and revocation is local.
*Policy:* device list, relay choice, recovery quorum.
*Prior art:* Signal device graph, Tailscale coordination, libp2p, Urbit Ames, Solid, Bitcoin wallets.
*Sharpest failure:* the phone is a SPOF and the highest-value target; a lost root is total compromise; and always-on computers (the paper, org computers) cannot depend on a phone being awake, so a shared org root reappears as an unreviewed home.

**16. Hardware-rooted attestation kernel: trust class is a measurement.**
*Core:* primitives are measured boot, sealed keys, remote attestation, SPIFFE-like site identity that cannot leave its TPM/SEP.
*Different because:* it makes "this site is what it claims" a cryptographic fact rather than an enrollment ceremony, and it prevents host-key exfiltration (W4).
*Policy:* acceptable measurements, enrollment thresholds, update policy.
*Prior art:* TPM 2.0, SGX/TDX, SEV-SNP, Apple SEP, Android StrongBox, Teleport device identity.
*Sharpest failure:* a heterogeneous fleet has no uniform root (old Windows desktops, Linux, Macs), and a legitimate OS update looks exactly like a compromise, so the fleet either freezes updates or auto-trusts anything — and it says nothing about placement or the operator reading the guest.

**17. Supervisor-tree kernel: failure and restart are structural.**
*Core:* primitives are actors, mailboxes, links/monitors, supervision strategies, location transparency. Computers, desks and leaves are actors; a failure is a supervision event.
*Different because:* it makes *failure policy* the kernel, not authority — restart intensity is the safety parameter.
*Policy:* supervision trees per org, restart intensities, escalation paths.
*Prior art:* Erlang/OTP, Akka, Orleans virtual actors, Kubernetes controllers.
*Sharpest failure:* supervision is not a security boundary, and a supervisor with authority over children recreates "one computer controls another" (doctrine I3). Virtual-actor turn-based concurrency gives at-most-one activation within a cluster, not across a partition.

**18. Human-adjudicated kernel: the kernel only guarantees presentability.**
*Core:* primitives are proposal, evidence bundle, owner decision, durable record. No automatic resolution exists anywhere; every contested state is shown with both tails and their effects digests.
*Different because:* it is the only option that treats the owner as the conflict-resolution primitive rather than as a policy author.
*Policy:* delegation, deadlines, default-if-silent, who may adjudicate which class.
*Prior art:* Git's conflict model, Corda notary-plus-human flows, approval workflows, safety-critical "human in the loop".
*Sharpest failure:* adjudication load. 200 computers × org schedules makes the owner the availability bottleneck and the weakest link (rubber-stamping); 24/7 computers have no human to wait for.

**19. Model-checked minimal kernel: the architecture *is* the spec.**
*Core:* primitives are chosen for checkability (lease, epoch, append, grant, effect) and the whole configuration space — sleep, partition, revoke, checkout, restore-from-backup — is model-checked before implementation; policy is constrained to a checked parameter space.
*Different because:* its defining property is verification coverage, not any particular primitive; it is the only option that claims safety for *all* configurations rather than tested ones (Q9).
*Policy:* only parameter ranges admitted by the model.
*Prior art:* AWS TLA+ specs (S3, DynamoDB), IronFleet, Chaparral, seL4 refinement, FoundationDB simulation.
*Sharpest failure:* model fidelity. Abstraction hides exactly the bugs that bite (clock, disk, OS, human), and state explosion forces the lease/epoch model to be simplified until W6 (restore-from-backup) is out of scope. Also collides with the "agentic speed" cadence.

**20. Dissolve-into-deployment kernel: the fabric is a content-addressed closure graph.**
*Core:* primitives are content-addressed store, derivation, signed closure, and a `current` pointer per computer. Placement is evaluating a derivation on a site; rollback is a pointer move.
*Different because:* it reframes "which site runs this computer" as "which closure is active", making reproducibility and atomic switch the primitives and writer authority a consequence.
*Policy:* which derivations may run where, pinning, GC roots.
*Prior art:* Nix/NixOS, Bazel, OCI, OSTree, systemd-sysext, Unison.
*Sharpest failure:* runtime state, effects and identity are not build artifacts. Two sites can both hold "current", and it explains deployment without explaining authority at all.

**21. Dissolve-into-contracts kernel: bilateral signed agreements, no global kernel.**
*Core:* primitives are a signed pairwise agreement (site↔computer, org↔person) and a local enforcement point. Safety is compositional from contracts.
*Different because:* it refuses global invariants and pushes everything to the smallest possible pair.
*Policy:* negotiation, breach handling, term expiry.
*Prior art:* Corda bilateral agreements, B2B EDI, OAuth client registration, mTLS meshes, legal contracts.
*Sharpest failure:* n-party invariants (no two writers, no control cycles, one identity) are not pairwise expressible; you need an n-party observer, and that observer is a home under a new name.

**22. Content-addressed pub/sub kernel: immutable objects plus mutable names.**
*Core:* primitives are immutable object, signed mutable pointer, subscription. Work is an object; the tape is a chain; reachability is a DHT.
*Different because:* it makes integrity and dedup the primitives and treats "who may write" as pointer custody.
*Policy:* pointer custody, GC/keep-set, subscription ACLs.
*Prior art:* IPLD/IPFS, Hypercore/Dat, Secure Scuttlebutt, Git, Tahoe.
*Sharpest failure:* pointer custody has no exclusion — two pointers to two heads is the default state — garbage collection collides with the keep set, and content addressing leaks equality (same blob ⇒ same digest).

**23. Cell kernel: shard the fabric into independent cells.**
*Core:* primitives are a cell boundary (per org, per ~100 computers), a cell-local home/sequencer, and a one-way cell-move protocol.
*Different because:* it caps blast radius and makes scale a partitioning question; there is deliberately no cross-cell kernel.
*Policy:* cell sizing, cross-cell work rules, migration approvals.
*Prior art:* AWS cells/shuffle sharding, Azure scale units, Slack channel sharding, Bulkhead pattern.
*Sharpest failure:* a person in two orgs, or a computer that changes employer, needs a migration protocol that is itself a distributed transaction; global invariants (identity uniqueness, no grant cycles) need a meta-cell.

**24. Two-plane kernel: data plane and control plane are separate kernels.**
*Core:* claim there is no single kernel. Data plane primitives: append/read content-addressed objects, integrity only. Control plane primitives: lease/grant/revoke, authority only. Composition is explicit and the two never share a protocol.
*Different because:* it names the doc's implicit split (bundles vs. leases) as the architecture, instead of hiding it inside one "fabric".
*Policy:* all of it, in the control plane.
*Prior art:* SDN control/data split, etcd-vs-kubelet, TPM endorsement vs. operational hierarchies, cellular architecture.
*Sharpest failure:* drift — any data-plane path that bypasses the control plane is an unguarded door (the bundle push route is exactly such a path, §2.3); and it doubles the number of protocols requiring verification.

---

## Dimensions these options differ along

1. **Locus of authority** — single home / witness quorum / no third party / pairwise / none (state converges).
2. **Exclusion mechanism** — lease+epoch, mobility (one instance), quorum leader, static types, atomic take, bond, human, none.
3. **What "conflict" means** — an owner act (1), a proof (3), a resolution rule (14), a convergence (5), a human decision (18), or an impossibility (2).
4. **Trust root** — host key, hardware measurement, capability possession, identity tuple, policy bundle version, money.
5. **Offline model** — full offline with a loss window (1), impossible (4), bounded by TTL, mailbox store-and-forward (15), local policy evaluation (8).
6. **Effect semantics** — precommit-then-fire (1, 7), detect-after (3), exactly-once via atomic claim (12), quorum commit (4), priced (13).
7. **Verification method** — runtime check, model check, type check, attestation, incentive, human review.
8. **Policy expressiveness** — near-zero (6: caps only) to maximal (7/8: arbitrary relations and policy code).
9. **TCB size** — microhypervisor (10) and typed kernel (9) at one end; global sequencer (11) and federation (14) at the other.
10. **Scaling axis** — per computer (1, 3), per cell (23), global (11), per person (15).
11. **Metadata privacy** — total order leaks (11), federation leaks (14), pairwise hides (21), content addressing leaks equality (22).
12. **Audit locus** — the tape (1, 2), the log (3, 11), the tuple graph (7), the policy decision (8), the market ledger (13).
13. **Self-development fit** — a typed kernel (9) and a model-checked kernel (19) bound self-tuning; a policy kernel (8) lets the computer tune itself into unsafe states; a market (13) lets it tune itself into insolvency.
14. **What it makes unsafe** — every option above names a configuration the doc currently assumes: offline effects (1, 3), laptop-only writing (4), canonical writes (5), org RBAC (6), offline admission (7), revocation latency (8), dynamic joins (9), operator visibility (10), offline append (11), history (12), Byzantine sites (13), no-merge (14), shared org root (15), heterogeneous fleets (16), security boundaries (17), unattended computers (18), model fidelity (19), runtime state (20), n-party invariants (21), pointer custody (22), cross-cell movement (23), data-plane bypass (24).

**Coverage of the open questions, by option:** Q1 (writer authority) — 1, 2, 3, 4, 9, 21; Q2 (reachability/home) — 1, 14, 15, 23; Q3 (access control) — 6, 7, 8, 16, 21; Q4 (placement) — 12, 13, 20, 23; Q5 (irreversible effects) — 1, 3, 5, 12, 18; Q6 (control without writing) — 17, 18, 21; Q7 (native capabilities) — 6, 10, 16; Q8 (org lifecycle) — 7, 8, 14, 23; Q9 (mechanism vs. policy, all-config safety) — 8, 9, 19, 24; Q10 (self-tuning in bounds) — 9, 19, plus the explicit warning in 8 and 13.
