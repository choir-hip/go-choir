# Survey 01: single-writer authority, consensus, leases, fencing, replication (2026-10-31) — verified revision

Scope: prior art for brief questions Q1 (writer authority), Q2 (reachability/home), Q5 (irreversible effects), Q9 (mechanism vs policy). No recommendation is made; section 9 lists options and trade-offs.

## Verification log

Tags: **[V]** = confirmed against a primary or official source this pass (URL given). **[CORRECTED]** = the original claim was wrong or imprecise; fix and URL given. **[UNVERIFIED]** = could not be confirmed against a primary source; what was tried is stated. Untagged text is unchanged reasoning/analysis (not a citable fact) or is covered by the section's tag. Items still carrying **[M]** in the original were either upgraded or relabelled **[UNVERIFIED]**; no [M] tags remain.

**Method limits (important).** This sandbox could not extract text from PDFs (no poppler/pypdf), and the USENIX Delos page returned HTTP 403, so papers available only as PDFs (Chubby, Raft ATC paper, Spanner, Calvin, FoundationDB, Corfu, Gray-Cheriton, VR Revisited, Zab, SUNDR, Bayou, Coda, PeerReview, MinBFT/CheapBFT, Quorum Leases) were confirmed only for existence/authors/venue via search snippets and secondary summaries, not for the detailed mechanism claims. Those mechanism claims are marked [UNVERIFIED] even where they are well established in the literature, so treat [UNVERIFIED] as "not checked against the primary text here", not "believed false".

**Counts (approximate, by distinct cited system/claim cluster):** confirmed [V] ≈ 22 (3 from the earlier pass: Kleppmann, Flexible Paxos, CASPaxos); corrected 6; unverified ≈ 40; added (new systems, verified) 5, plus 1 added but unverified.

**Most consequential corrections**
1. **GitHub 2018 incident (§2.5)** was mis-described. The 43 s partition was between the **US East Coast data centre and the US West Coast data centre** (not "between US East coasts"); Orchestrator (itself using Raft) promoted West Coast primaries; both sides then held writes the other lacked, so failback was not possible. Degraded service lasted 24 h 11 m. GitHub's post does not use the term "split brain". https://github.blog/news-insights/company-news/oct21-post-incident-analysis/
2. **Cloudflare "etcd/Raft incident 2020"** could not be found; the 2023 incident (Nov 2, 2023) was a **data-centre power failure** (Flexential/PDX-04) that took out the control plane, not a consensus or etcd failure. Original sentence retracted. https://blog.cloudflare.com/post-mortem-on-cloudflare-control-plane-and-analytics-outage/
3. **CT "Aviator" log (§5)** was not an equivocation/misbehaviour-by-lying case: it exceeded its Maximum Merge Delay (signer failed five runs) and Google shut it to new submissions on 2016-12-01. Useful as "log removed by policy", not as a split-view example. https://groups.google.com/a/chromium.org/g/ct-policy/c/lsZqdM_u7DI/m/qAsiRHkgCAAJ
4. **Datomic HA (§4.7)**: official docs describe an active/standby transactor pair where the standby takes over after missed **heartbeats** (default 5 s interval, worst case ~11 s) and a too-slow active transactor self-destructs; this is a time-based mechanism. The survey's claim that failover is by "CAS on the storage root pointer" is not confirmed by the official page (only a third-party ADR says "storage-level lease"). https://docs.datomic.com/on-prem/operation/ha.html
5. **Jepsen Kafka (§4.1)**: the Jepsen post shows data loss when a partitioned leader is replaced via ZooKeeper-detected failure; it does not frame the result as "unclean leader election", and the 2013 date was not confirmed from the page. https://aphyr.com/posts/293-jepsen-kafka
6. **Delos MetaStore (§4.4)**: secondary summaries say the MetaStore is a versioned key-value store implemented with Multi-Paxos, and that a loglet only needs a fault-tolerant **seal**; the survey's "single-decree consensus or any linearizable CAS register" is a fair abstraction but not the paper's wording. Primary page was 403. https://www.usenix.org/conference/osdi20/presentation/balakrishnan

---

## 0. The design under study, in the terms of this literature
The fabric design is, in distributed-systems terms:
- A single-writer replicated log (the tape) with hash chaining.
- A lease table held by one authority (the home; Node B) assigning an epoch per (computer, branch). Grants are fence-first. Leases never expire (owner decision, proposal 3).
- Followers are asynchronous replicas via signed delta bundles. The home is a follower with a durability ack ("home-durable").
- Conflicts (stale epoch pushes, same-seq different-digest) are resolved by branching plus an owner-chosen primary ref, not by merging.

Structural fact: the lease authority is a single database (a single point of failure and of truth), not a consensus group. The literature treats that as a legitimate point (primary-backup with an external configuration master) provided the authority is trusted and durable. Risks concentrate in (a) the authority's durability/rollback (W6, case 6), (b) liveness when the holder is unreachable and the lease never expires, (c) the gap between "fenced at the log" and "fenced at the external world".

## 1. Survey by system

### 1.1 Paxos family

**Single-decree / Multi-Paxos** (Lamport 1998 "The Part-Time Parliament"; 2001 "Paxos Made Simple"; Chandra, Griesemer, Redstone, "Paxos Made Live", PODC 2007, pp. 398–407).
- **[V]** "Paxos Made Live" exists with those authors/venue and reports that a replica with a corrupted disk that loses persistent state may renege on promises, violating a Paxos assumption; they added checksums and GFS "marker files" to distinguish corrupted from newly started replicas. https://research.google.com/archive/paxos_made_live.html (confirmed via search snippet; full text not read).
- **[UNVERIFIED]** The mechanism description below (ballots, two phases, Multi-Paxos stable leader), FLP-based liveness statement, and "membership change under-specified". Textbook content; not re-fetched. Lamport's paper: https://lamport.azurewebsites.net/pubs/paxos-simple.pdf (not opened).
- Problem: agree on a value (log slot) among n replicas, crash failures, asynchronous network.
- Mechanism: ballots; Phase 1 prepare/promise; Phase 2 accept by a majority; Multi-Paxos runs phase 1 once for a stable leader. The ballot is an epoch/fencing token intrinsic to the protocol.
- Guarantees: safety under any asynchrony; liveness only under partial synchrony (FLP 1985).
- Failure modes: dueling proposers; lost acceptor state silently breaks safety (an acceptor that forgets promises = a rolled-back lease DB; [V] above); membership change under-specified.
- Mechanism vs policy: ballot ordering, quorum intersection, durable acceptor state = mechanism; leader choice, timeouts, quorum membership = policy.

**Flexible Paxos** (Howard, Malkhi, Spiegelman 2016) **[V]** https://arxiv.org/abs/1608.06696. Only phase-1 and phase-2 quorums must intersect. Relevance: formalizes "cheap commits, expensive failover"; a home + workstation + witness shape is expressible as asymmetric quorums; a whole configuration space proven safe by one intersection predicate.

**CASPaxos** (Rystsov 2018) **[V]** https://arxiv.org/abs/1802.07000 (re-confirmed: single author Denis Rystsov, leaderless, per-key register updated by a user-supplied change function). Close in shape to "computer_writer_leases" and "computer_primary_ref" as CAS rows.

**EPaxos** (Moraru, Andersen, Kaminsky, SOSP 2013). **[UNVERIFIED]** original-paper details (fast path, dependency sets, SCC execution ordering). **[V]** Sutra, "On the correctness of Egalitarian Paxos" (arXiv 1906.10917, 2019; Information Processing Letters 156, 2020): flaw in both the TLA+ spec and the implementation, concerning how replicas move between ballots while computing a command's dependencies, allowing replicas to disagree and break linearizability. **[CORRECTED]** the survey said "known correctness bugs in the published recovery" — more precisely the flaw is in the ballot-switch/dependency-computation in recovery as specified (TLA+) and implemented (Go); I did not confirm it is in the SOSP paper text itself. https://arxiv.org/abs/1906.10917. Relevance to a single-writer tape is low.

**Vertical Paxos** (Lamport, Malkhi, Zhou, PODC 2009) **[V]**: an auxiliary configuration master handles reconfiguration; a special case yields traditional primary-backup protocols (Vertical Paxos II); the authors argue deployed primary-backup systems can be seen as instances. https://www.microsoft.com/en-us/research/publication/vertical-paxos-and-primary-backup-replication/ This supports the survey's "closest formal analogue" claim.

**Related**: Mencius, Fast Paxos [UNVERIFIED, not checked]; Spanner uses Paxos leader leases (see §2.6).

### 1.2 Raft (Ongaro, Ousterhout, USENIX ATC 2014; Ongaro PhD 2014)
- **[UNVERIFIED]** Venue/authors and mechanism list (terms, randomized timeouts, leader completeness, log matching, joint consensus, ReadIndex/lease reads). The PDF could not be text-extracted; the PDF metadata title is "In Search of an Understandable Consensus Algorithm". https://raft.github.io/raft.pdf
- **[UNVERIFIED]** PreVote/CheckQuorum are described in Ongaro's dissertation (not the ATC paper); not re-checked.
- **[V]** Single-server membership-change bug: Ongaro announced on raft-dev (July 2015) that the dissertation's single-server algorithm was flawed when concurrent changes occur (counterexamples on 4-server clusters, possible data loss and permanent split brain), found by Huanchen during a CMU class project; fix: leader must commit an entry in its own term before accepting membership changes; joint consensus unaffected. https://groups.google.com/g/raft-dev/c/t4xj6dJTP6E
- **[UNVERIFIED]** "Jepsen findings on many Raft implementations (etcd, Consul, RethinkDB, others)": only etcd checked (§1.6).
- Mechanism vs policy: as in the original (analysis, unchanged).

### 1.3 Viewstamped Replication
- **[V]** "Viewstamped Replication Revisited" (Liskov, Cowling) is MIT-CSAIL-TR-2012-021; has normal operation, view-change, recovery and reconfiguration protocols; a recovering replica sends Recovery messages and waits for f+1 RecoveryResponses including one from the primary, rejoining only after it knows a state at least as recent as before the crash (secondary summaries quoting the paper). https://dspace.mit.edu/handle/1721.1/71763
- **[UNVERIFIED]** the specific "nonce" detail; the original 1988 Oki-Liskov PODC details; "primary = view mod n".
- **[UNVERIFIED]** TigerBeetle as VSR-based with protocol-aware recovery and a deterministic simulator (VOPR); not fetched.
- Lesson (analysis): a node that lost state must not vote until it has learned from a quorum — consistent with the W6 hazard.

### 1.4 Zab / ZooKeeper
- **[V]** Zab (Junqueira, Reed, Serafini, DSN 2011) is crash-recovery atomic broadcast for ZooKeeper's primary-backup model; primary propagates incremental state changes; transaction ids are (epoch, counter) pairs; discovery phase picks history with highest epoch then highest id; new leader recovers before broadcasting (secondary summaries incl. Acolyer). https://blog.acolyer.org/2015/03/09/zab-high-performance-broadcast-for-primary-backup-systems/
- **[UNVERIFIED]** "zxid = epoch<<32 | counter" bit layout, session/ephemeral-node/watch semantics, and "ZK recipes recommend fencing by zxid" (ZooKeeper docs not opened). Hunt et al. USENIX ATC 2010 not opened.
- Mechanism vs policy: analysis unchanged.

### 1.5 Chubby (Burrows, OSDI 2006)
- **[UNVERIFIED]** All specifics (advisory locks; KeepAlive session leases; master grace period; sequencers; lock-delay; "sequencer little used"). Tried: WebFetch of the Google PDF twice (compressed binary, not readable). These are widely attributed to the paper but not checked here. https://research.google.com/archive/chubby-osdi06.pdf
- Failure mode / mechanism vs policy: analysis unchanged.

### 1.6 etcd
- **[V]** Jepsen's etcd 3.4.3 analysis (2020) found etcd locks unsafe as mutual exclusion in asynchronous networks without fencing; etcd's own blog says the lock is only safe within etcd's keyspace with revision-checked transactions; recommendation: pass a fencing token such as the lock key revision to any external system. KV operations tested strict-serializable under pauses, crashes, clock skew, partitions. https://jepsen.io/analyses/etcd-3.4.3 ; https://etcd.io/blog/jepsen-343-results/
- **[UNVERIFIED]** v3 lease TTL semantics ("TTL counted by leader in logical ticks"), global revision, mod_revision txns, concurrency.Election; etcd docs not opened. The claim about TTL being counted "in logical ticks" in particular is not confirmed and may be imprecise (etcd leases use wall-time TTLs managed by the leader).
- Operational hazards (fsync latency → leader churn): **[UNVERIFIED]**.

### 1.7 Summary table (consensus)
Unchanged from the original; each row inherits the tags of its section. The etcd row's "none (TTL counted by leader)" and the Chubby row are **[UNVERIFIED]**.

| System | Epoch name | Quorum | Fencing available? | Time assumption for safety | Liveness assumption |
|---|---|---|---|---|---|
| Multi-Paxos | ballot | any intersecting | ballot | none | partial synchrony + majority |
| Flexible Paxos | ballot | P1/P2 intersect | ballot | none | as chosen |
| Raft | term | majority | term | none (lease reads need drift bound) | partial synchrony |
| VR | view | majority | view | none | partial synchrony |
| Zab | epoch | majority | zxid | none | partial synchrony |
| Chubby | sequencer / master epoch | Paxos | sequencer (optional) | lease drift bound | partial synchrony |
| etcd | revision | Raft | revision | none (TTL counted by leader) | partial synchrony |

## 2. Leases, clocks, fencing, epochs

### 2.1 Leases (Gray & Cheriton, SOSP 1989)
- **[V]** Paper exists: SOSP 1989, pp. 202–210, doi 10.1145/74850.74870; a lease is a time-limited grant; a cache needs a valid lease to serve reads/apply writes; analytic model for lease terms using V-system data; short terms (~10 s) found efficient; the design tolerates non-Byzantine failures but does **not** cover clock failure, treated as a Byzantine fault. https://courses.cs.duke.edu/spring15/cps210/readings/leases-sosp89.pdf (via search snippets; full text not read)
- **[UNVERIFIED]** The exact drift-bound formulation in the paper ("holder treats lease valid for T minus drift allowance"; grantor waits T+ε). The survey's "bounded drift, not synchronized time" phrasing is the standard reading but I did not confirm the paper's wording; note the paper's own statement that clock failure is outside its fault model.
- Key corollary, tradeoffs, safety-fails-under-pauses: analysis, unchanged. Infinite terms (no expiry) = grantor never reclaims on its own.
- Variants: Spanner 10 s leader leases **[UNVERIFIED]** (blog quoting the paper; paper not read); Quorum Leases (Moraru et al., SoCC 2014) **[UNVERIFIED]**.

### 2.2 Clock assumptions
Analysis, unchanged. Hypervisor pause/resume making guest clocks jump is **[UNVERIFIED]** as a general statement (platform-dependent). "Use time for liveness, epochs for safety" is consistent with Kleppmann **[V]** (§2.3).

### 2.3 Fencing tokens and the Redlock critique
- **[V]** Kleppmann, "How to do distributed locking" (2016): https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html (verified earlier pass).
- **[UNVERIFIED]** Antirez, "Is Redlock safe?" (2016) rebuttal; not fetched (antirez.com). The characterization "community consensus (Jepsen-style) is that the token argument holds" is an opinion not a verified fact; downgrade to "Kleppmann's argument; Antirez disputed it".
- Corollary for the fabric (analysis): `writer_epoch` plus append CAS is the token; external world (mail, payments) does not check it.
- **[V]** BookKeeper (added) implements writer fencing exactly this way (§4.8).
- **[UNVERIFIED]** HDFS QJM epochs, Ceph OSD map epochs, Hyperdex/Hibari/Cassandra "generation"; Kafka epochs see §4.1.

### 2.4 Epochs / generation numbers
Analysis, unchanged. "Fork-consistency family" pointer: see §5.

### 2.5 Split-brain prevention in practice
- STONITH, SBD, quorum disk, SCSI-3 persistent reservations, Azure cloud witness, MongoDB arbiters, Galera garbd, "arbiter loses durability under w:majority": **[UNVERIFIED]** (not checked against vendor docs). Analysis of the two-node problem and lease-on-arbiter stands as reasoning.
- **[CORRECTED]** GitHub, Oct 21–22, 2018: a 43-second loss of connectivity between the US East Coast primary data centre and the US West Coast data centre; Orchestrator (running on Raft) formed a quorum with West Coast and public-cloud East Coast nodes and failed database clusters over so writes went to the West Coast; each side then held writes the other lacked, preventing safe failback; incident declared resolved after 24 h 11 m. The survey's "divergent primaries between US East coasts" and "orchestrator failover across a 43 s partition" are restated accordingly. The post does not itself say "split brain" or "without fencing"; those are interpretations. https://github.blog/news-insights/company-news/oct21-post-incident-analysis/
- **[UNVERIFIED]** GitLab 2017 (backup/replication confusion): not fetched.
- **[CORRECTED / retracted]** "Cloudflare etcd/Raft incident 2020": no such incident found. The Nov 2, 2023 Cloudflare control-plane/analytics outage was caused by a Flexential data-centre power failure (PDX-04) and a failover to other sites; a "HA expectations vs reality" lesson is plausible but it is not a consensus incident. https://blog.cloudflare.com/post-mortem-on-cloudflare-control-plane-and-analytics-outage/
- Mechanism vs policy: analysis unchanged.

### 2.6 Spanner and TrueTime (Corbett et al., OSDI 2012; Brewer 2017)
- **[UNVERIFIED]** leader leases 10 s default; uncertainty typically < 10 ms; eviction of skewed nodes. Course notes confirm commit-wait (leader picks commit timestamp ≥ TrueTime bound and waits until it is in the past) and that the leader assigns timestamps only within its lease interval, but the paper PDF was not read. Brewer 2017 not fetched.

### 2.7 Calvin (Thomson et al., SIGMOD 2012)
- **[UNVERIFIED]** Not fetched. Content (replicated pre-agreed input log, deterministic execution) is standard; the analogy to the tape is analysis.

## 3. Primary-backup, chain replication, CRAQ
- **[UNVERIFIED]** Alsberg & Day 1976; Budhiraja et al. 1993; "f+1 replicas tolerate f crashes" statement; van Renesse & Schneider OSDI 2004; Terrace & Freedman ATC 2009; Elnozahy et al. CSUR 2002 survey. None re-fetched.
- **[V]** Remus (Cully et al., NSDI 2008, best paper): active-passive pair; VM execution discretized into replicated checkpoints (as often as every 25 ms); outgoing network packets are held until the state that produced them has been replicated/checkpoint committed. https://www.usenix.org/event/nsdi08/tech/full_papers/cully/cully_html
- Direct mapping (home = config service plus ack backup; local-durable vs home-durable = async vs sync) and "output commit" correspondence: analysis, unchanged.

## 4. The log as the database

### 4.1 Kafka
- **[V]** KIP-101: replaced high-watermark-based truncation with leader epochs (OffsetForLeaderEpoch request; follower truncates to the end offset of the largest epoch both know); the HWM approach could lose data or diverge logs under rapid double failovers. https://cwiki.apache.org/confluence/spaces/KAFKA/pages/67634337/KIP-101+-+Alter+Replication+Protocol+to+use+Leader+Epoch+rather+than+High+Watermark+for+Truncation
- **[UNVERIFIED]** KIP-320 contents; producer epochs / idempotent producer / transactions; Kreps et al. NetDB 2011 (not fetched).
- **[V]** `unclean.leader.election.enable` exists and defaults to false since 0.11.0.0 (secondary: Netdata, Datadog; KIP-460 calls it potentially dangerous). **[CORRECTED]** Jepsen Kafka (aphyr, post 293): a fully partitioned leader was replaced after ZooKeeper detected the disconnect, resulting in data loss; the post is not framed as unclean election, and the 2013 date was not confirmed. https://aphyr.com/posts/293-jepsen-kafka
- Mechanism vs policy and the contrast with Choir's branch-keeping: analysis, unchanged.

### 4.2 LogDevice (Facebook)
- **[UNVERIFIED]** Epoch store in ZooKeeper, sequencer epoch bump, seal-then-activate, copysets, bridge/hole records. Original source (code.fb.com blog) not fetched.

### 4.3 Corfu / Tango
- **[V]** CORFU (Balakrishnan, Malkhi, Prabhakaran, Wobber, Wei, Davis; NSDI 2012, pp. 1–14): sequencer hands out tokens for empty log positions; clients hold a projection mapping positions to flash units, changed by installing a new projection with a new epoch; flash units treated as write-once registers; writes go to the replica set in fixed order. https://www.usenix.org/conference/nsdi12/corfu-shared-log-design-flash-clusters-0 (via secondary summaries + USENIX listing)
- **[UNVERIFIED]** "sequencer is a hint, not a safety device" and the "seal old projection before installing new" wording, "Paxos-replicated auxiliary" (paper text not read). Tango SOSP 2013 not fetched.
- Lesson (UNIQUE(computer, branch, seq) as write-once property): analysis.

### 4.4 Delos / Virtual consensus (Balakrishnan et al., OSDI 2020)
- **[V]** Paper exists, OSDI '20 best paper; VirtualLog chains Loglets, each covering a segment of the address space; a MetaStore maps the address space to loglets over time; reconfiguration = seal current loglet, write new mapping to MetaStore, route new writes to the new loglet; a loglet needs only a fault-tolerant seal; sealing can cause brief unavailability and reconfiguration has a latency cost. (Secondary: Acolyer, Murat, Vanlightly; USENIX page returned 403.) https://blog.acolyer.org/2020/11/09/delos/
- **[CORRECTED]** MetaStore: described as a versioned key-value store implemented with classic Multi-Paxos (not "single-decree or any linearizable CAS register").
- **[UNVERIFIED]** The list of loglet implementations (ZooKeeper-based, LogDevice, NativeLoglet) and runtime swap claims; fetch the PDF https://www.usenix.net/system/files/osdi20-balakrishnan.pdf to confirm.
- Mapping to the lease table / kernel+plugin pattern: analysis.

### 4.5 FoundationDB (Zhou et al., SIGMOD 2021, pp. 2653–2666)
- **[V]** (via secondary) All failure handling is reduced to a single recovery operation: the transaction system shuts down on failure; a new transaction-system instance gets a new epoch which blocks the old one. https://muratbuffalo.blogspot.com/2022/03/foundationdb-distributed-unbundled.html
- **[UNVERIFIED]** coordinators/cluster controller specifics, "lock old log servers", "recovery version", Will Wilson Strange Loop 2014 talk, "buggify" (not fetched).

### 4.6 Aurora (Verbitski et al., SIGMOD 2017)
- **[V]** Six copies across three AZs (two per AZ), write quorum 4/6, read quorum 3/6; tolerates loss of an AZ plus one more fault without losing data (AWS blog). https://aws.amazon.com/blogs/database/amazon-aurora-under-the-hood-quorum-and-correlated-failure
- **[UNVERIFIED]** "single writer instance assigns LSNs, volume epoch bumped on writer recovery, storage rejects stale epochs; VCL/VDL" (the SIGMOD 2018 follow-up "On avoiding distributed consensus for I/Os, commits and membership changes" is the likely source for the epoch detail; not read). https://www.cs.purdue.edu/homes/bb/cs542-20Spr/readings/impl/sigmod-18-amazon-aurora-avoiding-consensus.pdf
- Mapping: analysis.

### 4.7 Event sourcing, CQRS, Datomic
- **[UNVERIFIED]** Event-sourcing/CQRS practice, EventStoreDB expected-version, Axon, Marten, outbox pattern (Richardson, microservices.io): not fetched. They are standard patterns; check the sources before quoting.
- **[CORRECTED]** Datomic HA: official docs: the first transactor to communicate with storage becomes active, the second stays standby; the standby takes over if it sees the active miss two consecutive heartbeats; default heartbeat 5 s (worst-case failover ≈ 11 s); a too-slow active transactor self-destructs. So failover is heartbeat/time-based. The survey's "standby wins a conditional put on the storage root pointer" is **[UNVERIFIED]** (a third-party ADR says "storage-level lease"). https://docs.datomic.com/on-prem/operation/ha.html
- Hickey 2012 InfoQ talk: **[UNVERIFIED]**.

### 4.8 (Added) Apache BookKeeper — single-writer ledgers with reader-driven fencing **[V]**
- A ledger has one writer; the writer closes it by CAS-updating ZooKeeper metadata to CLOSED with the last confirmed entry. Any reader may force a close ("recovery"): mark metadata IN_RECOVERY (the old writer's metadata CAS then fails), send fence messages to bookies of the last fragment, find the last add-confirmed entry, close; concurrent recoverers converge via the metadata CAS; closed ledgers cannot be reopened for writing. A fence message may not reach every bookie, so a fenced old writer may still write to some bookies but cannot reach ack quorum. https://bookkeeper.apache.org/docs/development/protocol/ (and /docs/4.10.0/development/protocol/)
- Relevance: a production precedent for "single writer per log, takeover by a third party that fences first, old writer discovers on next write, nothing time-based required for safety". Mechanism: fence + metadata CAS; policy: who triggers recovery.

### 4.9 (Added) Azure Storage stream layer **[UNVERIFIED]**
Secondary slides/patent say streams are append-only, extents are sealed (immutable thereafter) on size, write failure or load balancing, and a new extent is created on other nodes and added to the stream. Paper (Calder et al., SOSP 2011, doi 10.1145/2043556.2043571) not opened; single-writer claim not confirmed.

### 4.10 (Added) CockroachDB range leases **[V]**
Older docs: epoch-based leases tie a leaseholder's validity to node liveness records (a node whose liveness lapses loses all its leases at once), expiration-based leases (typically seconds) for meta/system ranges because those store the liveness data; newer docs: leader leases make Raft leader and leaseholder the same replica and remove the node-liveness-range single point. https://www.cockroachlabs.com/docs/v21.2/architecture/replication-layer ; current: https://docs.cockroachlabs.com/docs/stable/architecture/replication-layer
Relevance: an explicit design progression showing the cost of a single liveness authority and of separating lease from consensus.

### 4.11 (Added) LiteFS (Fly.io) — SQLite primary lease **[V]**
One primary node is selected by a Consul-held lease; only it accepts writes; static-primary mode exists; sync replication and time-bounded async replication are listed as future. Lease is time-based. https://fly.io/docs/litefs/how-it-works/ ; https://fly.io/blog/introducing-litefs
Relevance: closest small-scale analogue of "single-writer file/DB replicated to followers with an external lease"; it picks a TTL lease, i.e. the opposite point from Choir's no-expiry choice.

### 4.12 (Added) Kubernetes Lease API **[V]**
Lease objects (coordination.k8s.io/v1) used as a lightweight distributed lock for control-plane leader election; fields include holder, renewTime, leaseDurationSeconds, leaseTransitions (count of holder changes); candidates wait lease duration plus margin before taking over. The docs do not present leaseTransitions as a fencing token. https://kubernetes.io/docs/reference/kubernetes-api/ (Lease v1) ; https://kubernetes.io/docs/concepts/cluster-administration/coordinated-leader-election/
Relevance: widely deployed TTL leases without data-path fencing, the case Kleppmann's critique addresses.

### 4.13 (Added) Cloudflare Durable Objects **[V]**
Each object has a globally unique name, is single-threaded, and has storage private to its instance; docs say each ID maps to exactly one instance globally, with optional placement hints. https://developers.cloudflare.com/durable-objects/what-are-durable-objects/ . Relevance: single-writer-per-entity as a product, with placement handled by the platform. Internal mechanism for ensuring uniqueness during failover is **[UNVERIFIED]** (not in the sources read).

## 5. Hash-chained logs and fork consistency
- **[UNVERIFIED]** Mazières & Shasha PODC 2002; Li, Krohn, Mazières, Shasha, SUNDR OSDI 2004 — fork consistency / fork-linearizability definition; not fetched. Standard reference; verify before quoting.
- **Git**: ref update by compare-and-swap, `--force-with-lease`: **[UNVERIFIED]** (git docs not opened).
- **Certificate Transparency**: RFC 6962 (2013) and RFC 9162 (2021) — Merkle tree, signed tree heads, consistency/inclusion proofs, gossip: **[UNVERIFIED]** for details; RFC 6962 listing found at https://www.rfc-editor.org/rfc/rfc6962. **[CORRECTED]** Aviator: not a lying-log case; it exceeded the Maximum Merge Delay (five consecutive signer runs failed to incorporate submitted chains in time), breaching RFC 6962 §3 and Chrome policy; Google agreed to stop it accepting submissions effective 2016-12-01. https://groups.google.com/a/chromium.org/g/ct-policy/c/lsZqdM_u7DI/m/qAsiRHkgCAAJ
- **Secure Scuttlebutt**: **[V]** paper "Secure Scuttlebutt: An Identity-Centric Protocol for Subjective and Decentralized Applications", Tarr, Lavoie, Meyer, Tschudin, ACM ICN 2019, doi 10.1145/3357150.3357396. **[CORRECTED]** author list (the survey said "Tarr, Schwerin, et al."). **[UNVERIFIED]** the fork-handling claims ("feed fork = identity burned", peers stop replicating, same key on two devices): the Protocol Guide (https://ssbc.github.io/scuttlebutt-protocol-guide/) does not discuss feed forks; it says each identity has exactly one append-only feed and describes "fusion identity" as the multi-device approach ("Each device still uses its own identity"). The fork behaviour may be in client implementations or the ICN paper; not confirmed.
- **Hypercore / Autobase**: **[V]** Autobase: each writer appends to its own local Hypercore; Autobase linearizes the cores into one causal order via a DAG of references; an `apply` handler rebuilds a derived view; reordering can cause undo/reapply. https://docs.pears.com/p2p/how-to/store-and-replicate/build-a-multiwriter-app-with-autobase.md ; https://www.npmjs.com/package/autobase . **[UNVERIFIED]** "single writer per Hypercore" is implied by the docs ("A plain Hypercore has a single writer") but fork-detection/key-recovery claims not checked.
- **Matrix state resolution**: **[V]** (refined) State Resolution v2 had further flaws: CVE-2025-49090 (state rollback/"state reset" via crafted events, high severity) fixed by State Resolution v2.1 in Room Version 12; CVE-2024-31208 (Synapse DoS via crafted events). **[CORRECTED]** the survey said "security bugs found in v1"; sources found concern v2 (and the v1→v2 move was a previous upgrade). https://matrix.org/blog/2025/08/project-hydra-improving-state-res/
- **[UNVERIFIED]** Keybase sigchains; Trillian, Rekor, CONIKS.
- Mechanism vs policy line (SSB burn vs Choir branch): analysis; premise about SSB behaviour is unverified.

## 6. Byzantine variants
- **[UNVERIFIED]** PBFT (OSDI 1999), HotStuff (PODC 2019), Tendermint, PeerReview (SOSP 2007), A2M/MinBFT/CheapBFT. None fetched; all are standard and likely accurate on authors/venues, but the "2f+1 with a trusted counter" statement should be checked against MinBFT/CheapBFT.
- The "when BFT is overkill" reasoning is analysis about the design, not a cited fact.

## 7. Offline-first, CRDTs, alternatives to single-writer
- **[UNVERIFIED]** CRDTs (Shapiro et al. 2011), Dynamo (DeCandia 2007), Bayou (Terry et al., SOSP 1995: tentative writes, primary commit, application-defined conflict detection/merge), Coda (Kistler & Satyanarayanan, TOCS 1992: hoarding, disconnected operation, reintegration with conflicts flagged). None fetched; characterizations are standard recollection.
- Mapping of the fabric's "checkout + branch on override" to Coda/Bayou: analysis.

## 8. Mechanism versus configuration: summary matrix
Unchanged from the original (analysis, not citable fact). Rows referencing unverified systems (Spanner, Kafka dials) inherit those tags.

| Concern | Fixed by the mechanism | Left to config/policy |
|---|---|---|
| Two writers at once | Epoch checked atomically at the log (fencing) | Who gets the next epoch; whether an epoch can be taken from an unreachable holder (expiry, quorum, human) |
| Stale holder doing harm | Fencing token at the resource, seal-then-activate | Resources that cannot check tokens need gateways/idempotency (extra mechanism per resource) |
| Authority rollback | Quorum-replicated or receipt-anchored epoch | Where the anchors are kept; who the witnesses are |
| Failover | Safe iff fence-before-grant; liveness only under timing | Detection timeout; automatic vs manual; what runs on the fallback |
| Durability vs availability | Replication level semantics (local vs quorum-durable) | Required level before externalizing (min ISR, w:majority, home-durable) |
| Fork/equivocation | Hash-chain + signed receipts make forks provable | Reaction: burn (SSB, unverified), truncate (Kafka), branch (Choir), halt (Raft) |
| Quorum size | Intersection predicate (Flexible Paxos) | Sizes, placement, witnesses |
| Clock use | None needed for epoch safety | Lease terms, drift margins, only for liveness |
| Time-based reads | Bounded uncertainty (Spanner) | Hardware and eviction policy |

## 9. Mapping to the brief's questions (options and trade-offs, no choice)
(Sections reasoning about the design are analysis; the new verified precedents are marked.)

### Q1: Writer authority across sites
Fixed points in the design (for reference): one writer per branch, epoch, fence-before-grant, no expiry, branch-on-override, signed receipts.

Options for **who decides the epoch**:
1. *Single home DB* (current). Pro: simple; Vertical Paxos shape **[V]**. Con: SPOF; rollback hazard (W6); trust concentration. Note CockroachDB's history **[V]** (node-liveness range as single point, later removed by leader leases) as a cautionary precedent for a single liveness/lease authority.
2. *Replicated lease table* (Raft/etcd, CASPaxos register per computer, synchronous replica with fencing). Pro: no rollback. Con: more machinery. Delos MetaStore (Multi-Paxos KV) **[V-secondary]** is a published instance of "small consensus-backed record, everything else pluggable".
3. *Receipt-anchored max-epoch* (current W6 mitigation). Quorum-intersection formalization à la Flexible Paxos **[V]**.
4. *Witness/arbiter per site pair*. Datacenter witness products **[UNVERIFIED]**.
5. *Transparency-log authority* (CT-style): detection not prevention. CT mechanics **[UNVERIFIED]** here.
6. *(Added)* *Reader-triggered fencing recovery (BookKeeper)* **[V]**: any party may seal and take over a single-writer log via a metadata CAS; old writer cannot reach ack quorum afterwards. Pro: no home required to be reachable by the stale holder; Con: needs a metadata CAS store and a quorum of storage nodes that honour the fence.

Options for **lease expiry / liveness**:
- *No expiry* (current): safety independent of time; liveness depends on revoke; fallback by branching.
- *TTL with self-fencing* (Gray-Cheriton; LiteFS **[V]**, Kubernetes Lease **[V]** do TTL leases): automatic failover; clock/pause assumptions; Datomic's heartbeat failover **[V]** is the same family.
- *TTL plus epoch-fencing at the log* (belt and braces): time proposes, epoch decides. Kleppmann **[V]**, etcd/Jepsen **[V]** support this layering.
- *Checkout with revalidation on wake* (current): reduces window; does not eliminate it.

Options for **offline work**: (a) checkout, no expiry (Coda-like **[UNVERIFIED]**); (b) read-only; (c) tentative local writes committed by home (Bayou **[UNVERIFIED]**); (d) local branch with owner-chosen primary (current); (e) CRDT-typed state (restricted).

Options for **conflict resolution**: truncate loser (Kafka KIP-101 **[V]**: follower truncates to the leader-epoch end offset); burn identity (SSB, **[UNVERIFIED]**); branch (Choir); merge by type (CRDT); merge by deterministic function (Matrix state resolution **[V]**: v2 and v2.1 have had state-reset CVEs); semantic merge by publication.

Known pitfalls to check the design against: (1) epoch in the same atomic check as the append; (2) stale holder discovers fencing only on next contact (BookKeeper docs explicitly note a fenced writer may still write to some storage nodes **[V]**); (3) authority rollback (Paxos Made Live disk-corruption **[V]**); (4) key duplication (SSB claim **[UNVERIFIED]**); (5) clock: no timing needed for safety.

### Q2: Reachability and the home
Unchanged analysis. Added precedents: Durable Objects **[V]** (platform-managed single-instance location with placement hints); Kubernetes Lease and LiteFS **[V]** store the current leader's address/identity in the lease record so clients can find it. Routing with reject-with-hint on stale epoch (ZooKeeper/Kafka/etcd clients) **[UNVERIFIED]** in detail. Kafka's client metadata refresh is standard but not fetched.

### Q5: Irreversible external effects under partition/offline
- *Output commit* (Remus **[V]**; Elnozahy survey **[UNVERIFIED]**).
- *Outbox + at-least-once + idempotent receiver* (Richardson; Kafka idempotent producer) **[UNVERIFIED]**; Stripe idempotency keys, SMTP lacking dedupe: **[UNVERIFIED]** (not fetched).
- *Fencing the egress*; *sagas* (Garcia-Molina & Salem 1987, **[UNVERIFIED]**); *exactly-once impossible end-to-end without receiver cooperation* (Two Generals; Kleppmann DDIA): **[UNVERIFIED]** page references.
- *Precommitment as witnessed intent*; branching interaction and effect-ledger options: analysis.

### Q9: Mechanism vs policy; verifying the configuration space
Precedents: Delos/VirtualLog **[V-secondary]**; Flexible Paxos **[V]**; Kafka dials **[V]** (config exists; "unsafe region" characterization is interpretation).
Verification methods used in practice: TLA+ (Newcombe et al., CACM 2015), Raft TLA+ spec, Cosmos DB TLA+, IronFleet (SOSP 2015), Verdi (PLDI 2015), Jepsen, deterministic simulation (FoundationDB; TigerBeetle VOPR; Antithesis), P language / S3 strong consistency: all **[UNVERIFIED]** (not fetched; Jepsen etcd **[V]** as an example of black-box testing finding a documentation/safety gap).
Configuration-space safety techniques and the minimal invariant list: analysis. Open-risk point ("proofs cover protocols, not deployment") is supported by Paxos Made Live **[V]** (implementation/disk issues) and the GitHub 2018 incident **[V]** (automation across sites); the claim "most bugs were in implementation and operations" is **[UNVERIFIED]** as a quotation.

## 10. Gaps and caveats
- No claim is empirical about Choir's code.
- Not covered or still unverified at primary level: Chubby specifics, Raft ATC paper details, Spanner/TrueTime numbers, Calvin, Corfu seal semantics, FoundationDB recovery steps, Zab bit layout, SSB fork behaviour, Bayou/Coda, PBFT/HotStuff/MinBFT, TLA+/IronFleet/Verdi.
- Prior art worth adding in a later pass (not researched here): Hermes/ZAB-style replication variants, Orleans single-activation grains, Akka Cluster singleton and split-brain resolver, HLC/hybrid logical clocks, Raft dissertation PreVote/CheckQuorum text, Apache Pulsar (BookKeeper-based) topic ownership, Ceph monitors/Paxos maps.

## 11. Primary sources
Verified this pass [V]:
- Kleppmann, 2016. https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
- Howard, Malkhi, Spiegelman, Flexible Paxos. https://arxiv.org/abs/1608.06696
- Rystsov, CASPaxos. https://arxiv.org/abs/1802.07000
- Sutra, EPaxos correctness. https://arxiv.org/abs/1906.10917
- Lamport, Malkhi, Zhou, Vertical Paxos. https://www.microsoft.com/en-us/research/publication/vertical-paxos-and-primary-backup-replication/
- Ongaro, raft-dev membership bug. https://groups.google.com/g/raft-dev/c/t4xj6dJTP6E
- Chandra et al., Paxos Made Live (existence + disk corruption). https://research.google.com/archive/paxos_made_live.html
- Jepsen etcd 3.4.3. https://jepsen.io/analyses/etcd-3.4.3 ; https://etcd.io/blog/jepsen-343-results/
- GitHub post-incident analysis. https://github.blog/news-insights/company-news/oct21-post-incident-analysis/
- Cloudflare 2023 post-mortem. https://blog.cloudflare.com/post-mortem-on-cloudflare-control-plane-and-analytics-outage/
- KIP-101. https://cwiki.apache.org/confluence/spaces/KAFKA/pages/67634337/
- Remus. https://www.usenix.org/event/nsdi08/tech/full_papers/cully/cully_html
- Datomic HA docs. https://docs.datomic.com/on-prem/operation/ha.html
- BookKeeper protocol. https://bookkeeper.apache.org/docs/development/protocol/
- CockroachDB replication layer. https://www.cockroachlabs.com/docs/v21.2/architecture/replication-layer
- LiteFS. https://fly.io/docs/litefs/how-it-works/
- Kubernetes Lease / coordinated leader election. https://kubernetes.io/docs/concepts/cluster-administration/coordinated-leader-election/
- Durable Objects. https://developers.cloudflare.com/durable-objects/what-are-durable-objects/
- Aurora quorum (AWS blog). https://aws.amazon.com/blogs/database/amazon-aurora-under-the-hood-quorum-and-correlated-failure
- Matrix Project Hydra / state res v2.1. https://matrix.org/blog/2025/08/project-hydra-improving-state-res/
- Autobase docs. https://docs.pears.com/p2p/how-to/store-and-replicate/build-a-multiwriter-app-with-autobase.md
- SSB ICN 2019 (bibliographic only). doi 10.1145/3357150.3357396
- CT Aviator thread. https://groups.google.com/a/chromium.org/g/ct-policy/c/lsZqdM_u7DI/m/qAsiRHkgCAAJ
- Jepsen Kafka post. https://aphyr.com/posts/293-jepsen-kafka
- Gray & Cheriton (bibliographic + abstract-level). https://courses.cs.duke.edu/spring15/cps210/readings/leases-sosp89.pdf

Confirmed only through secondary summaries (treat as [V-secondary]): VR Revisited recovery, Zab epochs, Delos, CORFU, FoundationDB epoch recovery, Azure Storage sealing.

Not verified (citations kept as leads): Lamport Part-Time Parliament; Moraru EPaxos SOSP 2013; Oki & Liskov 1988; Hunt et al. ZooKeeper; Burrows Chubby; Ongaro/Ousterhout ATC 2014 (PDF unreadable); Corbett Spanner; Brewer 2017; Thomson Calvin; Alsberg-Day; Budhiraja et al.; van Renesse-Schneider; Terrace-Freedman; Elnozahy survey; Kreps Kafka; KIP-320; LogDevice; Balakrishnan Tango; Hickey Datomic; Richardson outbox; Mazières-Shasha; SUNDR; RFC 9162; Hypercore docs; Castro-Liskov PBFT; Yin HotStuff; Haeberlen PeerReview; Terry Bayou; Kistler-Satyanarayanan Coda; Shapiro CRDTs; Newcombe; Hawblitzel; Wilcox; Garcia-Molina Sagas; GitLab 2017; Antirez Redlock rebuttal.
