Working...
Numbered divergent options. No verdict, no ranking. Each is a candidate kernel: small primitives plus guaranteed properties. Policy surface = what stays configurable above it. Newcomer gloss: tape = append-only hash-chained log; writer = sole process allowed to append; branch = divergent tail; home = always-on rendezvous/relay; site = a machine running work; leaf = disposable burst VM; outbox = held external effects.

1. Home-lease kernel (the sketch in the design doc, as kernel)
Core: primitives `Lease(computer,branch)->(epoch,holder,state)`, `Route(computer)->site`, `SiteRecord(site)`. Guarantees: at most one holder per epoch; home serializes all lease transitions; route always follows the lease; read-only follower copy at home never depends on the writer site.
Policy left open: who may check out / hand back / revoke; placement by trust, locality, availability, capacity, cost; failover aggressiveness.
Prior art: Chubby locks, NFSv4 delegations, AFS volumes, Tailscale control + DERP relay.
Unsafe configurations it permits: home compromise is total; home restored from stale backup rolls epochs back unless the lease table is itself monotonic; aggressive auto-failover turns every sleep into a branch; a site admission grant of `org:*` silently voids the employee-A/B separation.
Genuinely different because: liveness and authority both live in one always-on home; everything else is a cache or a checkout.
Sharpest failure: home partition means no new writers anywhere; offline work can only branch, never stay primary. Q-focus: Q1, Q2.

2. Epoch-fence / config-service kernel
Core: primitives `NextEpoch(computer,branch)->fencingToken`, `FencedAppend(token,data)`, storage-side `RejectStale`. Guarantee: storage itself rejects writes with old tokens even if the old writer is alive and believes it is primary.
Policy left open: when to bump epochs; who may request; sync-vs-async replication; promotion triggers.
Prior art: ZooKeeper + BookKeeper fencing, HDFS NameNode fencing, чим Paxos-based config services, FoundationDB coordinators.
Unsafe configurations: any site whose store skips the token check reintroduces silent split-brain; single-replica config service is a single point of loss; epoch-bump on every heartbeat flaps writers.
Genuinely different because: authority is a monotonic counter, not a holder identity; the home issues numbers but never sees data.
Sharpest failure: needs a highly available counter service per org; workstation offline cannot mint authority to start anything new. Q-focus: Q1, Q9.

3. Per-computer consensus kernel
Core: primitives `ProposeAppend`, `Vote`, `Snapshot`. Each computer's replicas (home plus sites that hosted it) form their own Raft/Paxos group; writer = current leader. Guarantee: linearizable appends and leader completeness inside the group.
Policy left open: voter vs witness membership; quorum size; when to add/remove a laptop as voter; snapshot cadence.
Prior art: etcd/Consul per-key consensus, per-shard Raft in RethinkDB/CockroachDB, TigerBeetle, Cloudflare Durable Objects failover.
Unsafe configurations: 2-voter groups (laptop + home) lose majority on any partition; making every workstation a voter makes availability worse, not better; witness-only durability misconfigured as real replication.
Genuinely different because: no external authority at all; the computer's replicas vote the writer into existence.
Sharpest failure: consensus chatter over flaky laptop links; leader flapping; tape growth makes snapshot/rejoin heavy. Q-focus: Q1, Q4.

4. Pure-branch / no-lease git kernel
Core: primitives `Branch(base)`, `Append(branch)`, `SetPrimary(signedPointer)`. No leases, no expiry, no fencing. Guarantees: branches are immutable causal chains; equivocation is detectable by hash; primary is one signed ref.
Policy left open: when to branch; who may move primary; branch retention cap; how to surface divergent effects.
Prior art: Git + signed refs, Secure Scuttlebutt, IPFS/IPLD + IPNS, Matrix event DAGs, Datomic.
Unsafe configurations: holder of an old key extends an abandoned branch forever; replayed old `SetPrimary` reverts the pointer without a monotonic guard; unbounded branch sprawl with no retirement policy.
Genuinely different because: it deletes time and liveness from the kernel; partitions are normal data, resolution is always a user act.
Sharpest failure: every partition waits on the owner; irreversible effects happily happen twice on two branches. Q-focus: Q1, Q5.

5. Bearer-delegation kernel (macaroons / biscuits everywhere)
Core: primitives `Mint(attenuatedToken)`, `Check(token,request,caveats)`, `RevocationList`. Every right (append branch N, place leaf class K, open mic, control child C) is a caveated bearer token verified offline by public key. Guarantee: attenuation is monotonic; a token can only lose power as it is delegated.
Policy left open: caveat vocabulary; delegation depth; token lifetimes; revocation freshness requirements.
Prior art: Google Macaroons, Biscuit, SPKI/SDSI, ZCAP-LD, AWS IAM/Cedar evaluation.
Unsafe configurations: stolen bearer = impersonation until revocation propagates; one broad caveat (`org:*`, `cap:*`) silently over-grants; verifiers with stale revocation lists accept revoked employees.
Genuinely different because: authority is a transferable cryptographic object, not a row in a home database; verification needs no round trip.
Sharpest failure: revocation under partition is the entire security story; audit depends on every verifier logging. Q-focus: Q3, Q6, Q7.

6. Virtual-actor kernel
Core: primitives `Activate(computer)`, `Deactivate`, `Call`, `Reminder`. Runtime migrates a single-threaded actor between sites transparently via a distributed directory; placement directors pick sites. Guarantee: at most one activation per computer; reminders survive migration.
Policy left open: placement director logic; idle-deactivation timeouts; reminder delivery semantics.
Prior art: Orleans virtual actors, Akka Cluster Sharding, Erlang `global`, Ray actors, Durable Objects.
Unsafe configurations: split-brain directory yields two activations (still needs fencing underneath); migrate-on-every-heartbeat policies flap; at-least-once reminders double-fire across migration.
Genuinely different because: location is hidden by the runtime; the owner never checks out or places the writer explicitly.
Sharpest failure: hides exactly what the owner cares about (which machine holds my keys); "where is my computer" becomes undebuggable. Q-focus: Q1, Q4.

7. Market-clearing kernel
Core: primitives `Bid(site,resources,price,trust)`, `Ask(work,constraints,budget)`, `Clear->(allocation,receipt)`. Guarantee: stated mechanism properties (strategy-proofness, envy-freeness, budget balance) within one clearing; receipts are verifiable evidence of contribution.
Policy left open: budgets per computer/person/org; reserve prices; bidding strategies; clearing cadence.
Prior art: Tycoon/Spawn grid markets, Filecoin storage market, AWS spot, Kubernetes scheduler + quota as a weak non-market form.
Unsafe configurations: rich computer starves others by default; colluding sites inflate clearing prices; latency-blind clearing puts an interactive writer on a cheap sleeping laptop.
Genuinely different because: placement is price discovery, not policy evaluation; trust enters as a constraint on who may bid.
Sharpest failure: mechanism complexity dwarfs the kernel; gaming, price spikes during outages, emergent instability. Q-focus: Q4, Q8.

8. Proof-carrying policy kernel
Core: primitives `Assert(signedFact)`, `Prove(policyHash,request)->proof`, `Verify(proof)`. The kernel never decides; it only checks that each request carries a derivation from signed facts plus the pinned policy hash. Guarantee: every grant is auditable to named premises; policy upgrades are atomic hash swaps.
Policy left open: everything, as versioned logic modules: placement rules, delegation rules, capability grants, org roles.
Prior art: OPA/Rego, Cedar, Zanzibar/ReBAC, SecPAL, Nexus Authorization Logic, AWS Zelkova-style verified policies.
Unsafe configurations: a logic bug is a security bug (`allow if true` compiles fine); stale signed facts (revoked employee, old trust class) accepted until a freshness oracle is enforced; proof-check cost used as a DoS vector.
Genuinely different because: mechanism is one verifier; all behavior is proof-shaped data, so audit and replay are free.
Sharpest failure: writing correct policy becomes the hardest kernel job; hot-path proof checking is slow. Q-focus: Q3, Q9, Q10.

9. WAL-shipping database kernel
Core: primitives `AppendWAL`, `ShipWAL(sinceLSN)`, `Promote(follower)`. Home is a (possibly synchronous) standby; workstation streams WAL up; failover promotes a standby. Guarantee: prefix-consistent followers; RPO=0 while synchronous.
Policy left open: sync vs async per computer/branch; promotion triggers; standby topology.
Prior art: Postgres streaming replication, MySQL semi-sync, SQLite litestream, FoundationDB log servers.
Unsafe configurations: async plus auto-promote loses the tail and presents two histories as one; sync-to-sleepy-laptop stalls every append; two promoters promote two primaries.
Genuinely different because: it treats the fabric as storage replication; authority follows the WAL stream, not a lease row.
Sharpest failure: conflates durability with authority; still needs an external promotion arbiter or it split-brains. Q-focus: Q1, Q2.

10. Epidemic / delay-tolerant outbox kernel
Core: primitives `LocalAppend`, `Gossip(bundle)`, `Witness(head)->signature`, `DeliverWhen(quorumK)`. Every site writes locally; convergence is anti-entropy; external effects need K witness signatures before release. Guarantees: availability-first writes; causal delivery; K-witnessed effects are at-most-once per witness set.
Policy left open: witness sets; quorum K; gossip peers and cadence; conflict surfacing.
Prior art: Bayou, Dynamo/Riak gossip, Secure Scuttlebutt, Briar/DTN messengers, Byzantine-witnessed CRDTs.
Unsafe configurations: low K plus Sybil witnesses double-sends; unbounded offline growth; no-primary views show N divergent truths with no canonical answer.
Genuinely different because: it inverts the default; partition is the normal case and agreement is the expensive exception.
Sharpest failure: pushes the irreversible-effect problem onto witnesses; quorum loss under partition freezes all effects. Q-focus: Q5, Q1.

11. Local object-capability microkernel
Core: primitives `Handle`, `Invoke(handle)`, `Attenuate`, `Revoke`. Screen, mic, files, models, network are unforgeable handles passed into the guest; no ambient authority; revocation is immediate. Guarantees: confinement (no handle, no access); least privilege by construction.
Policy left open: which handles to grant when; consent UX; handle granularity; snapshot/persistence of handle tables.
Prior art: seL4, Fuchsia handles, Genode, Cap'n Proto capabilities, WASI capabilities, XDG portals / Flatpak Powerbox.
Unsafe configurations: confused deputy via handle passing (mic handle + network handle in one leaf); handles persisted in snapshots survive revocation; coarse handles (`files:*`) over-grant by design.
Genuinely different because: it mediates the machine boundary, not the network; says nothing about writers or homes and solves native access completely.
Sharpest failure: needs a second distributed-cap layer above it for cross-site delegation; handle UX and auditing are hard. Q-focus: Q7.

12. Escrowed-effects kernel
Core: primitives `Intend(effect)`, `Escrow(intent,bounds)->escrowID`, `Release(escrowID,proofOfCommit)`, `Cancel`. The writer never touches the outside world; a separate executor fleet holds the outbox, dedups by idempotency key, releases once per escrow even across branch switches. Guarantee: at-most-once per escrow ID within one executor set.
Policy left open: which effects require escrow; bound language (amount, recipient class, time window); executor quorum and topology.
Prior art: transactional outbox (Kafka/DB outbox), Stripe idempotency keys, XA/2PC escrow, Agoric/Zoe escrow contracts, precommitment + threshold-signing quorums.
Unsafe configurations: two disjoint executor sets each release once; bounds language too weak lets a repurposed escrow through; escrow-table loss either blocks effects forever or re-releases.
Genuinely different because: it splits append authority from effect authority; branching the tape does not branch the right to act.
Sharpest failure: the executor set becomes a second fenced authority with its own failover problem; every send pays intend-then-release latency. Q-focus: Q5.

13. Exokernel / library-policy kernel
Core: primitives `StoreBlock`, `Attest(execution)`, `Send(authenticatedMsg)`. The kernel only multiplexes, isolates and attests; each computer ships its own lease manager, scheduler and policy engine as versioned library code. Guarantee: isolation plus attestation, nothing more.
Policy left open: nearly all of Q1-Q8, per computer and per org, evolving with the computer itself.
Prior art: Xok/Exokernel, MirageOS unikernels, SGX/Nitro attestation fabrics, Hyperledger Fabric chaincode.
Unsafe configurations: every computer invents incompatible buggy policy; no global invariant holds (org can only refuse resources, not bound behavior); audit must understand N policy implementations.
Genuinely different because: it standardizes the narrowest waist (bytes, attestation, messages) and refuses to standardize authority at all.
Sharpest failure: maximal flexibility with minimal uniformity; self-development can break its own authority logic from inside. Q-focus: Q9, Q10.

14. Enterprise PKI + RBAC kernel
Core: primitives `Cert(subject,roles,validity)`, `Ticket(service,expiry)`, `CheckACL`. Org CA issues roles; sites enforce ACLs; short-lived Kerberos-like tickets gate placement and control. Guarantees: centralized org control; time-bounded credentials; familiar audit trails.
Policy left open: role graph; ticket lifetimes; admission webhooks; cross-org federation.
Prior art: Active Directory/Kerberos, X.509 + LDAP, Kubernetes RBAC + admission webhooks, BeyondCorp/ALTS.
Unsafe configurations: long tickets plus clock skew keep revoked employees valid; temporary roles calcify into permanent admin; cross-org federation either over-shares or never ships.
Genuinely different because: the org is the trust root and the person is just another principal; ownership flows downward from the CA.
Sharpest failure: hates offline (tickets expire on the plane) and hates persons-as-owners; merges mechanism and policy inside CA configuration. Q-focus: Q8, Q3.

15. Bounded self-tuner kernel
Core: primitives `Policy(params)`, `Bound(invariant,proofObligation)`, `Tune(propose,simulate)->accept/reject`. The kernel holds machine-checked invariants (for example, no placement violates the trust relation); the computer's tuner may move parameters only inside the verified envelope, with each change simulated or model-checked before commit. Guarantee: all reachable configurations satisfy the pinned invariants; tuning trace is replayable.
Policy left open: tunable parameters and ranges; the simulator and its fidelity budget; invariant set versioning.
Prior art: barrier/Lyapunov control theory, eBPF verifier as in-kernel proof checker, TLA+ config verification, AWS automated reasoning, bounded autoscalers with admission gates.
Unsafe configurations: incomplete invariant set proves the wrong property safe; simulator-reality gap letsผ่าน verified-but-harmful tuning through; tuner overfits to the simulator.
Genuinely different because: the kernel's product is not a decision but a proof that a whole configuration space is safe to explore.
Sharpest failure: verification on the hot path is slow; bounds too tight freeze adaptation, too loose prove nothing. Q-focus: Q9, Q10.

Dimensions along which these options differ:
place of authority (home row, counter, quorum, pointer, token, directory, price, proof, WAL, witnesses, handles, escrow, library, CA, invariant set); dependence on time and clocks (no timers vs expiring tickets vs fencing epochs); consistency/availability bias (linearizable append vs available-local-then-gossip); trust root (single home, config quorum, key set, org CA, market, simulator); granularity of rights (whole computer, branch, effect, single handle, single escrow); revocation story (fence, epoch bump, revocation list, handle drop, ticket expiry, pointer move); offline story (read-only, checkout, branch, full local write, frozen effects); verification story (runtime checks, fencing, proofs, model checking, receipts, audit logs); placement decider (owner, management desk, director, market clearing, proof derivation); effect execution (writer-direct, witnessed, escrowed-executor); self-development surface (parameters only, policy modules, library code, nothing).
