# Survey 02: partitions, disconnected operation, branching and reconciliation (2026-10-31)

Scope: prior art for a single-writer-per-branch, hash-chained tape whose writer
may go offline. Read first: the brief, `docs/design/resource-fabric-2026-10-31.md`
(§0, §1.3, §2, §7, §9, §10) and `docs/design/branching-tape-2026-10-31.md`.
No repo edits. No recommendation of a final choice (per the brief).

## Verification log (pass 2, 2026-10-31)

Method: WebSearch/WebFetch against primary sources, abstracts and official docs.
Many paper PDFs (Bayou, SUNDR, Speculator, Coda UW copy, PACELC) came back
unreadable or blocked to the fetcher; for those I confirmed citation and
abstract via search results only and say so. Tags now used: **[V]** confirmed
by a source retrieved in this pass (URL given); **[V-cite]** bibliographic
data confirmed, mechanism details not re-read; **[CORRECTED]** claim changed;
**[UNVERIFIED]** tried and not confirmed, or not attempted (stated);
**[ADDED]** new entry. Tags **[K]** / **[U]** from pass 1 remain only where
nothing below says otherwise, and should be read as [UNVERIFIED]. Items
tagged [V] in pass 1 that I did not re-open are marked **[V-prior]**
(carried over, not re-checked by me).

Counts (approximate, per claim cluster): confirmed ~28 (incl. partial), corrected 9,
still unverified ~35 (listed in §13), added 6.

Most consequential corrections:
1. **Realm / Atlas Device Sync is end-of-life, not "possibly deprecated"**:
   MongoDB deprecated Atlas Device Sync and the Device SDKs, EOL 2025-09-30
   (already past); the on-device DB continues as open source (§2.5).
2. **Mahajan-Alvisi-Dahlin's bound is *real-time* causal consistency**, not
   plain causal; and plain causal is only *sticky* available; under total
   availability causal and read-your-writes are unachievable (Jepsen). Attiya
   et al. is PODC 2015 (also TPDS 2017), with an OCC upper bound for
   multi-value-register stores (§1.3).
3. **HAT is VLDB 2014** (arXiv 2013 extended version), not 2013 (§1.3).
4. **Fossil prevents forks under autosync** ("would fork" abort) and warns
   when they occur anyway; it is not merely a post-sync warning. Autosync
   default and private-branch semantics are not stated on the branching page
   (§4.2).
5. **Local-first paper**: Upwelling is not in the essay; Trellis is built on
   Automerge; the essay's stated weak point for file sync is lack of real-time
   collaboration; "conflicts rare" is not stated there (§9).
6. **SSB forks** arise mainly from one identity used on two devices / a stale
   copy reusing a sequence number + previous-hash; the backup-restore variant
   is plausible but unconfirmed (§6.2).
7. **Coda** cite: SOSP 1991 + TOCS 10(1) 1992; the "Coda TC 1990" cite is a
   different paper. Coda's disconnection reports are "one to two days" (V).
8. **Irmin vs MRDT**: the OOPSLA 2019 MRDT paper (Kaki et al.) synthesizes
   merge functions from invertible relational specs (Quark implementation); a
   link to Irmin was not confirmed. Irmin's own merge paper is Farinier et al.
   (JFLA 2015) (§4.4).
9. **Notes/CouchDB/CloudKit/Dropbox** details now split into confirmed vs not
   (see inline tags); CouchDB winner rule and Dropbox three-tree/4 MiB claims
   were *not* found on the pages tried.

Evidence marking in the original text below: tags written in pass 1 are
superseded by any bracketed "**Pass 2:**" note attached to the item.

Vocabulary used to map to the design: *tape* = hash chain; *branch* = linear
chain from a common ancestor; *home* = the control-plane authority holding the
lease row; *local-durable / home-durable* = the design's two durability levels;
*checkout* = lease held by a workstation with no expiry.

---

## 0. The design under study, in the terms of the literature

The two design docs already compose, in effect:

- an **exclusive checkout** (lease, no expiry, fence-then-grant) as the
  normal path: this is *pessimistic* concurrency control, a CP choice per
  branch;
- a **tentative/committed** split (local-durable vs home-durable): Bayou's
  vocabulary;
- **branch instead of merge** on divergence, with the owner picking a
  primary: Fossil/Git "fork" semantics plus a human resolution;
- **signed equivocation proofs**: SUNDR's fork-detection, with signatures
  per epoch;
- **external effects gated on home-durable precommitment**: Speculator's
  "hold external output until the speculation is confirmed", and the
  transactional outbox.

So the literature mostly answers "which of these is a known pattern, what
broke in it before, and what are the alternatives on each axis".

---

## 1. Theory: impossibility results and consistency models

### 1.1 CAP (Brewer 2000; Gilbert & Lynch 2002) [V-cite, K content]
**Pass 2:** Brewer, "CAP Twelve Years Later: How the 'Rules' Have Changed",
IEEE Computer 45(2):23-29, 2012, DOI 10.1109/MC.2012.37 [V]; confirmed points:
only perfect C+A during a partition is ruled out, partitions are rare, the C
in CAP is single-copy consistency (narrower than ACID C), latency is ignored
by classic CAP (https://www.infoq.com/articles/cap-twelve-years-later-how-the-rules-have-changed).
"Choice is per-operation" matches the paper's thrust; the Gilbert-Lynch
theorem statement and Brewer 2000 keynote were not re-opened [UNVERIFIED].
- **Problem:** what a replicated service can promise when the network drops
  messages between replicas.
- **Mechanism (a theorem, not a mechanism):** in an asynchronous network, no
  implementation of an atomic (linearizable) read/write register can be both
  available (every request to a live node gets a response) and tolerate
  arbitrary partitions. Brewer's talk framed it "pick 2 of 3"; Gilbert-Lynch
  proved the narrow form, and Brewer himself later (2012) said the framing
  hides that the choice is per-operation and that partitions are rare and
  recoverable.
- **Guarantees/failure model:** crash and message loss; "availability" means
  *non-failing nodes answer*, not "the user's workstation is up".
- **Failure modes of the idea:** misuse as a 3-way menu; "CA" systems do not
  exist if partitions can occur; treating C as anything weaker than
  linearizability.
- **Mechanism vs policy:** CAP fixes which pairs are impossible; where a
  system sits is **policy per operation/datum**. A single-writer lease is
  the CP point for that branch: during a partition the minority side cannot
  write (in Choir: the phone is read-only while the Mac holds the lease).
  Branching ("work here anyway") is the AP escape hatch, paid for by
  divergence that must later be reconciled by a human.

### 1.2 PACELC (Abadi 2012) [V-cite via search; content UNVERIFIED]
**Pass 2:** fetch of https://www.cs.umd.edu/~abadi/papers/abadi-pacelc.pdf
failed (certificate rejected). Brewer 2012 independently notes that classic
CAP ignores latency (the gap PACELC fills) [V, link in §1.1]. The system
classifications (Dynamo/Cassandra PA/EL, Spanner/VoltDB PC/EC) are
[UNVERIFIED]; note that VoltDB and Spanner classification should be checked
against the paper before quoting.
- **Problem:** CAP says nothing about the no-partition case, where most
  systems actually choose between latency and consistency.
- **Mechanism:** "if Partition: Availability vs Consistency; Else: Latency vs
  Consistency". Classifies Dynamo/Cassandra PA/EL, Spanner/VoltDB PC/EC, etc.
- **Relevance:** a writer checked out to a workstation gives *local latency*
  (EL: local-durable commit, async push to home) at the price that the home
  copy lags. The design's "2 s push cadence" is literally the E-L side of
  PACELC. A synchronous push-before-ack would be E-C and would cost a WAN
  round trip per event.

### 1.3 Consistency models and Jepsen's map [partly V in pass 2]
**Pass 2 corrections/confirmations:**
- **[CORRECTED]** HAT: Bailis et al., "Highly Available Transactions: Virtues
  and Limitations", **VLDB 2014** (arXiv 1302.0309, first submitted Feb 2013,
  revised Oct 2013) [V-cite, https://arxiv.org/abs/1302.0309]. The abstract
  says it introduces "a taxonomy of highly available systems"; the
  total/sticky/unavailable split was not visible on the abstract page
  [UNVERIFIED there], but Jepsen's causal page confirms the *classes*:
  causal is **sticky available**, and under **total** availability causal and
  read-your-writes are unachievable while writes-follow-reads, monotonic
  reads and monotonic writes remain [V, https://jepsen.io/consistency/models/causal].
- **[CORRECTED]** The "strongest achievable" result is **real-time causal
  (RTC)**, not plain causal: Mahajan, Alvisi, Dahlin, "Consistency,
  Availability, and Convergence", UT Austin TR-11-22 (2011); Jepsen: RTC "is
  proven to be the strongest consistency model in an always-available,
  one-way convergent system" [V, Jepsen link above; paper
  https://cs.utexas.edu/~dahlin/papers/cac-tr.pdf located, not read in
  full]. Attiya, Ellen, Morrison, "Limitations of Highly-Available
  Eventually-Consistent Data Stores", **PODC 2015** (385-394; IEEE TPDS 2017)
  [V-cite]: for multi-value-register stores nothing strictly stronger than
  *observable causal consistency* is achievable (secondary summaries,
  https://arxiv.org/pdf/1512.00168).
- Terry et al., "Session Guarantees for Weakly Consistent Replicated Data",
  PDIS 1994, pp. 140-149, DOI 10.1109/PDIS.1994.331722: RYW, MR, WFR, MW,
  per-session, developed for Bayou [V, https://www.cs.cornell.edu/courses/cs734/2000FA/cached%20papers/SessionGuaranteesPDIS_1.html].
- The Jepsen map's full hierarchy still did not render (the index page
  lists only Models/Phenomena/Dependencies links); hierarchy below remains
  background knowledge [UNVERIFIED]; fetch https://jepsen.io/consistency/models
  next time.

- Hierarchy (strongest to weakest, roughly): strict serializable /
  linearizable > sequential > causal (+ "causal+" with convergence) >
  PRAM/FIFO and session guarantees (read-your-writes, monotonic reads,
  monotonic writes, writes-follows-reads) > eventual / strong eventual.
- **Availability classes (Bailis et al., "HAT", 2013) [V]:** *totally
  available* (any non-failing replica serves, e.g. read committed, MAV,
  causal in some forms), *sticky available* (available if a client keeps
  talking to the same replica or a causally-up-to-date one: read-your-writes,
  PRAM, causal), and *unavailable* (linearizability, snapshot isolation,
  serializability, "consistent reads" under partitions). Causal consistency
  is the strongest model achievable with sticky availability (Mahajan, Alvisi,
  Dahlin 2011 "Consistency, availability, convergence" [K]; Attiya et al.
  2015 [U]).
- **Session guarantees (Terry et al., PDIS 1994) [K]:** RYW, MR, MW, WFR,
  enforced by a client carrying version metadata to whichever replica it
  reaches. They are the cheapest user-visible rescue of eventual consistency.
- **Failure modes:** vendor claims of "strong" that are actually weaker
  (Jepsen analyses). Linearizability does not compose with offline
  anything.
- **Mechanism vs policy:** the *model* is a property of a data type/operation
  pair, so it is configuration of the datum, not of the platform. The
  Choir tape is **linearizable per branch by construction (one writer)**;
  across branches the only relation is the shared prefix (a fork point),
  i.e. *no* consistency is offered between branches, deliberately.
- **Jepsen map note:** I could not extract the full hierarchy from the
  fetcher, so the model list above is from knowledge [K].

### 1.4 Causal+ and COPS (Lloyd, Freedman, Kaminsky, Andersen, SOSP 2011) [V-prior; not re-opened in pass 2]
- **Problem:** geo-replicated stores want low latency and availability yet
  stronger than eventual.
- **Mechanism:** *causal+ consistency* = causal + convergent conflict
  handling (concurrent writes to a key resolved by a deterministic, commutative
  handler, e.g. last-writer-wins or a merge function). Clients track
  dependencies (nearest-dependency set) and a datacenter exposes a write only
  after its dependencies are visible locally. COPS-GT adds read-only
  transactions.
- **Failure model:** datacenter partitions; assumes data per key fits a
  conflict handler.
- **Known failure modes:** dependency metadata growth (garbage-collected
  after global commit); write-heavy cost for GT; LWW silently drops writes.
- **Mechanism vs policy:** dependency tracking is mechanism; the conflict
  handler is policy per type.
- **Relevance:** it is the *rejected* design in branching-tape §8
  ("needs parent lists, hybrid logical clocks, version vectors and per-kind
  CRDT rules"). COPS shows the machinery is real and what it costs.

---

## 2. Disconnected-operation systems (primary-copy and optimistic)

### 2.1 Coda (Satyanarayanan et al. 1990; Kistler & Satyanarayanan, SOSP 1991 / TOCS 1992) [V partial]
**Pass 2:** [V] SOSP 1991 pp. 213-225, TOCS 10(1) Feb 1992; Venus states
hoarding / emulation / reintegration; optimistic replica control; disconnected
modifications logged and replayed at reintegration; the authors report
operating disconnected for **one to two days**; the central idea is that
caching can also enhance availability (secondary summaries + paper copy at
https://classpages.cselabs.umn.edu/Spring-2018/csci5105/papers/coda.pdf;
the UW copy hit a login page and the UMN PDF was not text-read by me).
**[UNVERIFIED]**: CVV/store-id granularity (volume vs object), "whole replay
rejected on one conflict", replay-log compaction, trickle reintegration, and
AVSG/VSG details are from the 1990-92 papers by memory. The IEEE TC 1990 Coda
paper is a different paper from the TOCS one; its title is unconfirmed.
- **Problem:** a client must keep working when the file servers are
  unreachable (laptop, mobile).
- **Mechanism:** the client cache manager (Venus) has three states:
  *hoarding* (prefetch what the user will need, using a hoard database and
  priority), *emulating* (disconnected: serve from cache, log every
  mutation in a replay log, compacting cancelling operations), and
  *reintegrating* (replay the log to servers on reconnect; weakly
  connected mode uses trickle reintegration). Coda has **no leases or
  locks for disconnected work**: it is optimistic. Conflicts are detected by
  comparing *store identifiers / version vectors per volume and object*
  (CVVs) at reintegration; a conflicting replay is rejected as a whole and
  the user repairs via a repair tool. Coda also replicated server volumes
  (AVSG, VSG) with optimistic server replication.
- **Guarantees:** no consistency during disconnection; eventual
  convergence at reintegration; the TOCS paper reports one to two day
  disconnections [V]. Update/update conflicts are *detected*, never silently
  lost.
- **Failure modes:** hoard misses (cache lacks a needed file while
  disconnected, an availability failure that policy cannot fully fix);
  log growth; rare but real write-write conflicts that need manual repair;
  the replay log was per-volume, so one conflict rejects a batch.
- **Mechanism vs policy:** the three-state machine, replay log and
  version-vector conflict detection are mechanism; hoard profile,
  priority, and when to reintegrate are policy/config.
- **Mapping:** Choir's "Mac offline with lease" is Coda's emulation state
  *with the optimistic part removed* (the lease makes conflicts impossible
  rather than detected), and Choir's push of events is reintegration but
  of an append-only log (no per-object compaction, since the tape is
  evidence). Coda's lesson: **hoarding is the real offline problem**.
  Choir's equivalent is "what does the Mac need locally to run desks
  offline": local models (§6 of the fabric doc), snapshot, secrets.

### 2.2 Bayou (Terry, Theimer, Petersen, Demers, Spreitzer, Hauser, SOSP 1995) [V partial]
**Pass 2:** [V via abstract/summaries] per-write **dependency checks** and
client-supplied **merge procedures**; two states, *committed* and *tentative*;
clients may observe tentative writes; a **primary** server determines commit;
conflict detection/merge must be deterministic so replicas converge
(https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/bayou.pdf
did not text-extract; abstract via https://web.eecs.umich.edu/~manosk/assets/papers/bayou.pdf
search result). **[UNVERIFIED]**: tentative order is (timestamp, replica id),
undo-log rollback and re-execution, and CSN details; these are from memory of
the paper. Note: Bayou defines committed/tentative by whether the write's
conflicts are *ultimately resolved*, and the primary fixes the commit order;
the mapping to Choir's home-durable below is an analogy, not identity.
- **Problem:** collaborative apps (calendar, mail) on weakly connected
  replicas where file-level conflict detection is too coarse.
- **Mechanism:** every write carries an application-supplied **dependency
  check** (a precondition query on the replica state) and a **merge
  procedure** (what to do if the check fails). Replicas gossip writes
  pairwise (anti-entropy). Writes are first *tentative*, ordered by
  (timestamp, replica id); one designated **primary** replica assigns
  a global commit order (CSN); a write becomes *committed* once the
  primary orders it. Replicas roll back and re-run tentative writes when
  an earlier one arrives (undo log). Session guarantees (Terry 1994) let
  clients choose RYW/MR/MW/WFR for each session.
- **Guarantees:** eventual consistency; committed prefix is the same on all
  replicas; tentative suffix may be reordered; application-level conflict
  semantics.
- **Failure modes:** writing good merge procedures is hard (application
  burden); the primary is a liveness dependency for *commit* (not for
  tentative operation); rollback of externally visible effects is
  impossible (Bayou does not solve it, and the paper restricts to
  database state).
- **Mechanism vs policy:** tentative/committed, primary commit, and
  anti-entropy are mechanism; dependency checks and merge procs are per-app
  policy.
- **Mapping:** the design's *home-durable = committed*, *local-durable =
  tentative* is exactly this. Differences: in Bayou two replicas can both
  produce tentative writes to the same data and the primary merges them;
  in Choir only one writer exists per branch, so "primary" assigns nothing
  and rollback is never needed, but the cost is that the second writer's
  work lands in another branch, not in the order. **Bayou's central point
  for Q5:** tentative writes must not cause irreversible effects, or the
  application must model them as *drafts with intent*; the design's
  "outbox as draft with send intent" is this.

### 2.3 Ficus / Rumor (Guy, Popek, Page, Heidemann, Reiher et al. 1990-1998) [V-prior citations; mechanism UNVERIFIED, not re-opened in pass 2]
- **Problem:** peer-to-peer replicated filing with one-copy availability,
  no central server, partitions normal.
- **Mechanism:** any replica accepts updates (update/any, reconcile
  periodically). Per-file version vectors detect concurrent updates;
  directory conflicts are auto-resolved (directories treated as a set-like
  structure with name-conflict and remove/update semantics); file conflicts
  are detected and reported, with type-specific resolvers (mailbox merge,
  etc.) for common cases (Reiher et al. 1994).
- **Failure modes:** remove/update and name/name conflicts; "insidious"
  conflicts where a rename and update interleave; resolvers must be trusted
  code. Peer-to-peer reconciliation graph needs spanning-tree coverage.
- **Mechanism vs policy:** version vectors and reconciliation protocol =
  mechanism; resolvers and conflict policy = per-type.
- **Mapping:** the *pure optimistic* end of the spectrum, for the "automatic
  merge" alternative the design rejected.

### 2.4 Lotus Notes replication (Iris Associates, 1989+) [V partial]
**Pass 2:** [V via HCL docs and community write-ups: https://help.hcl-software.com/notes/10.0.1/rep_save_conflicts_t.html ,
https://help.hcl-software.com/dom_designer/11.0.1/basic/H_WHY_ARE_DELETED_DOCUMENTS_REAPPEARING.html ,
https://stwissel.github.io/blog/2013/01/what-to-do-with-save-replication-conflicts.html]
Winner = copy edited/saved more times, tie broken by latest save; the loser
becomes a response document carrying `$Conflict` (shown as "[Replication or
Save Conflict]"); deletion stubs are purged after the purge interval and if
stubs are purged before replicating, **deleted documents reappear**. The
exact edit-count scope (per replica vs total) is [UNVERIFIED]; "Iris
Associates 1989" and "field-level merge optional per form" are
[UNVERIFIED] (the "merge conflicts" form option exists in Notes but I did
not retrieve it).
- **Problem:** field sales/laptop users working offline against shared
  databases, ~1990.
- **Mechanism:** each document has a unique id and a *sequence number* /
  modification time; replicas replicate document-by-document. On conflict
  (both changed since last replication) the winner (more edits, then later
  time) becomes the main document and the loser becomes a **"Replication or
  Save Conflict"** *response document* retained for a human. Field-level
  merge was optional (per form).
- **Failure modes:** conflict documents pile up unnoticed; deletion stubs
  and purge intervals cause "zombie" resurrection of deleted documents if
  a replica is offline longer than the purge interval (a documented
  operational failure).
- **Mechanism vs policy:** replication scheduling, conflict field merge and
  purge interval are config.
- **Mapping:** the **"keep both, let the owner choose"** policy precisely
  as the design does it with branches; the purge-interval lesson applies to
  branch retention (a branch retired while an offline replica still
  holds it).

### 2.5 Mobile/offline sync products (Couchbase Lite / PouchDB-CouchDB; Realm/MongoDB Device Sync; Firebase) [mixed; see Pass 2]
**Pass 2:**
- **CouchDB [V]** (https://docs.couchdb.org/en/stable/replication/conflicts.html):
  a deterministic winner is chosen so all peers agree; plain GET returns only
  the winner; losers stay as conflicting leaves, visible via
  `?conflicts=true` / `open_revs=all`; the app resolves via one `_bulk_docs`
  that updates one revision and sets `_deleted:true` on the others; compaction
  drops non-leaf bodies but keeps the revision list; "revision pruning"
  limits history growth. **[UNVERIFIED]**: the page does **not** state the
  winner rule ("longest history, then lexicographic rev") nor name
  `_revs_limit`; both are from memory. Couchbase Lite / Sync Gateway
  conflict-resolver callback [UNVERIFIED, not fetched].
- **[CORRECTED] Realm/Atlas Device Sync:** MongoDB deprecated Atlas Device
  Sync and the Atlas Device SDKs with **end of life 2025-09-30**; the
  on-device database continues as an open-source project; App Services auth
  triggers, wire protocol and Data Access Permissions also deprecated
  [V, https://www.mongodb.com/docs/atlas/device-sdks/deprecation.md]. What
  happened to hosted services after that date was not checked. The
  server-side operation-transformation / LWW policy and "client reset"
  details are [UNVERIFIED].
- **Firebase** offline queue, LWW, and rule-change rejection: [UNVERIFIED],
  not fetched. The local-first essay does score Firebase as proprietary and
  weak on longevity/privacy/control [V, §9].
- **CouchDB / PouchDB / Couchbase Lite:** documents have a revision tree
  (`_rev` = generation + hash); replication is incremental via a changes
  feed with checkpoints; concurrent edits create **branches in the
  revision tree** (conflicts); CouchDB deterministically picks a winner
  (longest history, then lexicographic rev), keeps losers retrievable as
  conflicting revisions, and the app is expected to resolve by writing a new
  revision on the winning branch and deleting the loser. Couchbase Lite 2+
  Sync Gateway adds a conflict resolver callback. **This is the closest
  product analogue to "branches, owner chooses primary"**, with the
  difference that the winner is picked deterministically by default, and
  the DB goes on after conflict. Known failure: unresolved conflict
  buildup, revision tree growth requires compaction/`_revs_limit`.
- **Realm / Atlas Device Sync (MongoDB):** operations transformed
  server-side with a defined conflict policy (LWW at field level; list
  inserts merged; deletes win); the client keeps a local DB and a
  *queue of operations* uploaded when connected. Failure modes: *client
  reset* (when the server cannot reconcile, e.g. after compaction or schema
  change, the client's unsynced changes may be discarded or recovered
  manually); the product was deprecated/ended-of-life for some SDKs in 2024
  [U, verify].
- **Firebase (Realtime DB, Firestore):** offline cache with a local write
  queue; writes are applied optimistically to the cache and replayed;
  conflict = last write wins, transactions need connectivity.
  Failure modes: silent LWW overwrites; queued writes outliving auth or
  security-rule changes (rejected on reconnect); no tamper evidence.
- **Mechanism vs policy:** local store + outbound op queue + change feed =
  mechanism; resolver = config.
- **Mapping:** these all use *server-assigned* order + client op queue (Bayou
  shape). The Choir design differs by making the **client the writer** and
  the server the follower, which is more like Git than like Firebase.

---

## 3. Quorum/leaderless replication and its metadata

### 3.1 Dynamo (DeCandia et al., SOSP 2007) and version vectors [V-prior; not re-opened]
**Pass 2:** Dotted version vectors, Preguiça, Baquero, Almeida, Fonte,
Gonçalves, arXiv:1011.5808 (submitted 2010-11-26, 12 pp) [V-cite]. The
abstract confirms: existing causality tracking either loses causality or has
metadata growing with clients/updates, and DVV capture causality with metadata
bounded by the number of *servers* registering updates for an item
(https://arxiv.org/abs/1011.5808). The specific "concurrent writes through the
same server by different clients" defect is the paper's body argument
[UNVERIFIED: not on the abstract page]. Parker et al. 1983 [UNVERIFIED].
Dynamo's vector-clock truncation and sloppy quorum are [V-prior].
- **Problem:** an "always writeable" shopping-cart store across datacenters.
- **Mechanism:** consistent hashing, sloppy quorum (N, R, W) with hinted
  handoff, vector clocks on each value, read repair, Merkle-tree
  anti-entropy. Reads may return multiple concurrent versions
  (*siblings*), which the application reconciles (the cart is unioned).
- **Guarantees:** eventual consistency; availability for writes under
  partitions (at the cost of siblings).
- **Failure modes:** vector clock size growth (the paper truncates them,
  losing causality information: spurious conflicts); lost updates with LWW
  resolution (Cassandra, Riak LWW mode); sloppy quorum is not a quorum
  (stale reads); deleted-item resurrection.
- **Mechanism vs policy:** N/R/W per bucket is configuration; version
  vectors are mechanism.
- **Version vectors (Parker et al. 1983, Locus)** [K]: per-replica counters
  detect concurrency in *state* (is it a descendent, ancestor, or
  concurrent). **Dotted version vectors** (Preguiça, Baquero, Almeida, Fonte,
  Gonçalves, 2010) [V] fix a correctness defect: plain per-server version
  vectors cannot represent concurrent writes made *through the same server*
  by different clients (the server is forced to either overwrite or
  over-merge, causing lost updates or sibling explosion), and per-client
  vector clocks grow unboundedly. A "dot" tags each write with a single
  (replica, counter) event. Size stays O(servers).
- **Mapping:** the branching-tape doc rejects version vectors because the
  *unit of write authority is a lease epoch*, and the chain + epoch
  already acts as a "dot". The cost model is useful: the design avoids
  all metadata by never allowing concurrent writes in one branch.

### 3.2 CRDTs (Shapiro, Preguiça, Baquero, Zawirski, 2011) [UNVERIFIED in pass 2]
**Pass 2:** HAL record (inria-00555588) blocked by an anti-bot page; not read.
Confirmed in this pass: Kleppmann, "Making CRDTs Byzantine fault tolerant",
PaPoC 2022 (Rennes, April 2022), DOI 10.1145/3517209.3524042: a scheme
tolerating any number of Byzantine nodes (Sybil-immune) with SEC and modest
changes to existing CRDTs [V, https://martin.kleppmann.com/2022/04/05/bft-crdt-papoc.html].
**[ADDED]** Kleppmann & Howard, "Byzantine Eventual Consistency and the
Fundamental Limits of Peer-to-Peer Databases", arXiv:2012.00472 (Dec 2020): a
formal model for applications that tolerate arbitrarily many Byzantine nodes
[V, https://arxiv.org/abs/2012.00472]. The 2022 paper was not shown to say that
"one Byzantine replica corrupts everyone" in unauthenticated CRDTs; read as
motivation (its abstract says non-Byzantine assumptions are increasingly
dangerous). The CALM/I-confluence attribution stays [UNVERIFIED] (§12).
Automerge JSON CRDT (Kleppmann & Beresford, arXiv 1608.03960) [V-prior]; Yjs
below.
- **Problem:** replicas must converge without coordination.
- **Mechanism:** *strong eventual consistency*: any two replicas that have
  received the same set of updates are in the same state. Two styles:
  *state-based* (join-semilattice, merge = least upper bound, idempotent,
  commutative, associative) and *operation-based* (commutative ops,
  requires exactly-once, causal-order delivery). Catalog: G-/PN-counters,
  G-/2P-/OR-sets, LWW-register, MV-register, sequence CRDTs (Treedoc, RGA,
  Logoot, LSEQ).
- **Failure modes:** metadata/tombstone growth (garbage collection needs
  causal stability, i.e. coordination); "convergence" does not mean
  "intention preserved" (interleaving anomalies in sequences); invariants
  such as "balance >= 0" or "this mail was sent at most once" cannot be
  maintained coordination-free (CALM/I-confluence, Bailis et al. 2014 [K]);
  unauthenticated CRDT replicas let one Byzantine replica corrupt
  everyone (Kleppmann, "Making CRDTs Byzantine fault tolerant", PaPoC 2022
  [V]).
- **Mechanism vs policy:** the lattice is mechanism; the choice of which
  data type to use per app object is design-time policy; **safe CRDT use
  requires the app to restrict itself to monotone invariants**.
- **Automerge (Kleppmann & Beresford, JSON CRDT, 2017 [V]; Automerge
  2.x with Rust core [K]):** JSON document CRDT, history retained as a
  DAG of changes with hashes (like Git). Concurrent edits to the same
  map key retain all values (*conflicts*) and expose a deterministic
  winner; counter/text/list types. Failure: history growth; compaction
  loses history; large docs slow before Automerge 2.
- **Yjs (Nicolaescu, Jahns, Derntl, Klamma, GROUP 2016, pp. 39-49, DOI
  10.1145/2957276.2957310, "Near Real-Time Peer-to-Peer Shared Editing on
  Extensible Data Types") [V-cite; a dblp entry lists different authors, the
  DOI record and repository support the four above]:** YATA, open-source as Yjs,
  claims convergence, intention preservation, offline editing. The rest
  ("very fast", GC of tombstones, state vectors, no built-in history)
  [UNVERIFIED]. Sequence CRDT, very fast,
  GC drops tombstones when allowed, state vectors for sync; no built-in
  history.
- **Mapping:** per-document CRDTs *inside* a Texture revision are a known
  option for collaborative-editing; the design chose "Texture is the sole
  writer of canonical revisions", which is the same decision as "one
  linearizing writer per object kind" (the dispatch of app desks). The
  design explicitly says it falls back to per-kind leases for what matters.

### 3.3 Operational transformation (Ellis & Gibbs 1989; Jupiter, Nichols et al. 1995; Google Wave/Docs) [partly V]
**Pass 2:** [V] the claim that published OT transformation functions failed
TP1/TP2: Oster et al. (Collaborate Com 2006, "Tombstone Transformation
Functions") say none of the published propositions satisfy TP1 and TP2
(https://eudl.eu/doi/10.1109/colcom.2006.361867); Randolph, Boucheneb, Imine,
Quintero give counterexamples, including Imine's IT function failing TP2
(https://arxiv.org/pdf/1302.3292). Oster, Urso, Molli, Imine, "Data
Consistency for P2P Collaborative Editing" (file oster06cscw, CSCW 2006)
exists (https://wooki.sourceforge.net/papers/oster06cscw.pdf), but I did not
open it, so the exact CSCW venue/counterexamples are [UNVERIFIED]. The
Jupiter/Wave/Docs and Ellis-Gibbs details are [UNVERIFIED].
**[ADDED]** Gentle & Kleppmann, "Collaborative Text Editing with Eg-walker:
Better, Faster, Smaller", EuroSys 2025: aims to combine OT's small memory and
CRDT's offline merge; claims order-of-magnitude lower steady-state memory
than CRDTs and orders-of-magnitude faster long-branch merges than OT
[V, https://arxiv.org/abs/2409.14252]. Relevant because it treats long-running
offline branches as the central workload.
- **Problem:** real-time collaborative text editing.
- **Mechanism:** transform concurrent ops against each other so they
  converge; with a central server (Jupiter/Wave/Docs) the server's total
  order simplifies the transformation function to a pairwise one.
- **Failure modes:** TP2 violations in decentralized OT (many published
  algorithms were wrong; Oster et al. 2006 [K,U]); needs a central server
  for correctness in practice; high implementation complexity.
- **Mapping:** OT = "a server-ordered tentative/committed model for
  text". Same shape as Bayou, specialised.

### 3.4 Session guarantees and sticky routing [K]
For phone clients that hit a home during a writer's absence, the phone is
a *session* reading a follower copy: RYW across writer moves needs a
(branch, sequence, digest) token carried by the client and checked by the
home. That is an easy mechanism and not stated in the docs.

---

## 4. Version-control systems: branch-not-merge, patches, and forks

### 4.1 Git (Torvalds 2005) [UNVERIFIED in pass 2: not fetched; textbook material, https://git-scm.com/book/en/v2]
- **Problem:** distributed development with cheap local work.
- **Mechanism:** content-addressed Merkle DAG of commits; every clone is
  a full replica; refs (branches) are mutable pointers; `push` is
  compare-and-swap on a ref (fast-forward only unless forced); merge
  creates a commit with two parents; rebase rewrites history.
- **Guarantees:** integrity (hash chain); no availability dependency; no
  automatic semantic conflict detection (only textual three-way merge).
- **Failure modes:** force-push destroys others' work (reflog saves
  locally); silent semantic conflicts that merge cleanly textually;
  unlimited divergence; binary files unmergeable; no identity of
  writers beyond optional signatures; signed-tag/commit trust is
  user-managed.
- **Mechanism vs policy:** ref update CAS and the DAG = mechanism; branch
  protection, who may push, merge vs rebase, signing = policy (hosting
  platform).
- **Mapping:** the primary ref ("computer → branch") is a Git ref;
  "switch primary" is `git update-ref` with a compare; "no merge" is
  a restriction Git itself does not impose. **Git workflows show that the
  hard part is not data structure but the *policy over the ref***.

### 4.2 Fossil (Hipp, 2006) [partly V / CORRECTED]
**Pass 2:** [V, https://fossil-scm.org/home/doc/trunk/www/branching.wiki] A
**fork** is "a check-in [with] two or more direct (non-merge) children in the
same branch", i.e. accidental divergence, versus *branching* by intent. With
autosync, committing against an outdated leaf is **aborted with a "would
fork" message**; forks can still arise offline, with autosync off, or with
`--allow-fork`, and Fossil cannot detect them until the repos synchronize;
it then warns. **[CORRECTED]**: the original "warns the user after a sync
that creates one" omits the commit-time prevention. **[UNVERIFIED]**: the
page does not state that autosync is the default or that private branches are
never synced (both are from memory; the private-branch claim is from Fossil's
docs on `--private`, not fetched). "Hipp, 2006" date unverified.
- **Problem:** distributed VCS with built-in tickets/wiki; a small team.
- **Mechanism:** SQLite repository, Merkle-ish artifacts, *autosync* by
  default (commit pushes to the remote if reachable). Divergent heads
  without a branch name are called **forks**, and Fossil warns the user
  after a sync that creates one and the UI marks it; resolution is by
  merge or by closing a leaf. A *"private" branch* is never synced.
- **Mapping:** Fossil's *autosync + warn on fork* is the policy the
  design proposes for "tails sizes and effects digest, owner picks".
  Terminology collision: Fossil "fork" = Choir "branch"; Choir "fork" =
  separate repository/clone.

### 4.3 Darcs / Pijul patch theory (Roundy 2005; Mimram & Di Giusto 2013; Pijul, Meunier et al.) [UNVERIFIED in pass 2: not fetched]
**[ADDED] Jujutsu (jj):** records conflicts *as data in the commit tree*:
conflicts are stored as lists of states to add and to remove (so N-sided
conflicts exist), materialized into markers only in the working copy; rebasing
a conflicted commit does not nest markers [V, https://docs.rs/crate/jj-cli/0.41.0/source/docs/conflicts.md
and technical/conflicts.md]. This is the closest deployed analogue of
"record the conflict as data rather than block" on a Git-compatible store,
cited in the Darcs/Pijul "Mapping" below as the key idea.
- **Problem:** treat changes as first-class objects that commute, so
  merges are order-independent.
- **Mechanism:** Darcs: patches with commutation rules; merge = commute
  the incoming patches past local ones; exponential-time merge cases are
  documented ("conflict-of-conflicts"). Pijul: a category-theoretic model
  where files are graphs of lines and conflicts are a first-class state
  (a "zombie/cyclic" graph) that can be committed and resolved later;
  merge is associative and commutative by design.
- **Failure modes:** Darcs's exponential merges; Pijul's complexity and
  small ecosystem; commuting patches does not imply semantic
  correctness.
- **Mapping:** an extreme form of the "merge" option; the key idea for
  the design is **recording the conflict as data rather than blocking**,
  which is what the branch-notice and the retained non-primary branch do
  at coarse granularity.

### 4.4 Irmin (Madhavapeddy, Gazagnaire et al.; Kaki, Priya, Sivaramakrishnan, Jagannathan, OOPSLA 2019 "Mergeable Replicated Data Types") [V-cite / CORRECTED attribution]
**Pass 2 [CORRECTED]:** The MRDT paper is Gowtham Kaki, **Swarn** Priya, KC
Sivaramakrishnan, Suresh Jagannathan, PACMPL 3 (OOPSLA), art. 154, 2019, DOI
10.1145/3360580 [V, https://www.cs.purdue.edu/homes/suresh/papers/oopsla19-mrdt.pdf].
It derives merge functions for composable types from invertible relational
specifications, implemented as Quark in OCaml. My search found **no
statement tying MRDT to Irmin**; the Irmin-specific merge reference is
Farinier, Gazagnaire, Madhavapeddy, "Mergeable persistent data structures",
JFLA 2015 [V-prior, hal-01099136]. The per-type three-way merge / branch /
clone / pull / push description of Irmin itself and the LCA/criss-cross
failure mode are [UNVERIFIED] (irmin.org not fetched).
- **Problem:** git-like branching/merging for *typed, user-defined
  application data* in a store.
- **Mechanism:** a content-addressed store with branches and commits; the
  application supplies a **three-way merge function** for each value type
  (derived from invertible relational specs in the MRDT work); branch /
  clone / merge / pull / push as in Git; backends in memory, disk, git,
  HTTP.
- **Failure modes:** correctness depends on user-written merge; lowest
  common ancestor ambiguity with criss-cross merges (recursive LCA);
  garbage collection across branches is hard.
- **Mapping:** Irmin is the **"keep the DAG, add typed merge" extension**
  of the design; it shows what the "publication and adoption" step would
  look like if automated per kind.

### 4.5 Exclusive checkout in earlier VCS: RCS, SCCS, CVS reserved checkout, Perforce exclusive-open, SVN lock, Git LFS locks [Git LFS partly V; rest UNVERIFIED]
**Pass 2:** Git LFS: `git lfs unlock --force` skips the clean-status checks
and tells the server to remove a lock **even if owned by another user**; who
may do that depends on the LFS server (some require admin) [V, man pages via
https://man.archlinux.org/man/git-lfs-unlock.1.en.txt and
https://github.com/git-lfs/git-lfs/wiki/File-Locking]. The wire API is
`POST /locks/{id}/unlock` with a `force` field (third-party client summary;
authoritative spec docs/api/locking.md in the git-lfs repo, not opened).
"The server cannot make the old holder stop" is a reasonable inference,
[UNVERIFIED]. RCS/SCCS/CVS/Perforce/SVN lock behavior [UNVERIFIED].
Pessimistic locking of binary or unmergeable files. Known failure mode:
**stale locks** held by someone on vacation or whose machine died;
mitigated by an administrator *break lock* operation, which is the
analogue of "owner revokes the lease and opens a host branch". Git LFS
locking mirrors this ("steal" lock requires force; the server cannot
make the old holder stop).

---

## 5. Cloud-drive sync and its conflict behavior

### 5.1 Dropbox [partly V / UNVERIFIED]
**Pass 2:** From https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine
[V]: the engine (Nucleus) is in Rust; the control thread is designed to be
entirely deterministic given fixed inputs and scheduling, enabling
pseudorandom simulation testing; strong data model with tight invariants.
**[UNVERIFIED]**: that page did not mention the three-tree
(Remote/Local/Synced) model or 4 MiB blocks, nor "convergent" wording; the
"conflicted copy" naming and who-wins rule are from memory (Dropbox help
center not fetched). The three-tree model may be in a different Dropbox post.
- **Mechanism:** a sync engine tracks three trees (remote, local, synced
  base); operations are derived by diffing, and designed to be convergent
  and testable (Nucleus, Rust); content-addressed 4 MiB blocks; server
  file journal assigns a monotone cursor.
- **Conflict handling:** if a file changes on two devices from the same
  base, the later-synced version wins as the path and the other is saved as
  "filename (X's conflicted copy YYYY-MM-DD)".
- **Failure modes:** conflicted-copy proliferation; surprises with
  renames and deletes, offline moves of directories; the user is
  the merge engine; no transactional multi-file semantics.
- **Mapping:** the baseline "keep both under a different name" policy.

### 5.2 iCloud Drive / CloudKit / NSFileVersion (Apple) [UNVERIFIED]
**Pass 2:** The Apple doc page for `CKError.Code.serverRecordChanged` loaded
with no body text, so the fetcher answered from its own background knowledge
(recordChangeTag optimistic locking; server, client and ancestor records in
userInfo). I therefore do **not** count that as verification. CKSyncEngine
(2023), NSFileVersion UI and Core Data+CloudKit LWW/data-loss edge cases
likewise [UNVERIFIED]; check developer.apple.com directly.
- **CloudKit:** records with a `recordChangeTag` (opaque version); a save
  with a stale tag fails with `serverRecordChanged`, returning the server
  record, the client's record and the ancestor record; the app resolves
  and retries (optimistic CAS). Change tokens for incremental fetch;
  zone-level atomicity; `CKSyncEngine` (2023) wraps the pattern.
- **iCloud Drive/documents:** conflicts surface as `NSFileVersion`
  unresolved versions; the user is shown a picker ("Keep Both / Keep
  this") or the app resolves programmatically.
- **Core Data + CloudKit (NSPersistentCloudKitContainer):** field-level
  last-writer-wins by default; documented data-loss edge cases when
  schema or tokens reset.
- **Mapping:** per-record CAS with a returned three-way triple is a very
  clean API, and the tape's "append CAS on head" is the same primitive.

### 5.3 Lessons common to 5.1-5.2 and 2.5
(1) The default is nearly always **keep-both or LWW**, never merge, for
non-structured content. (2) Users do not notice conflicts until data
is lost, so **conflicts must be a first-class visible object**. (3)
The failure that hurts is *reset* (client reset, token reset, purge).

---

## 6. Authenticated divergence: fork consistency and equivocation

### 6.1 SUNDR and fork consistency (Mazières & Shasha PODC 2002; Li, Krohn, Mazières, Shasha, OSDI 2004) [V partial]
**Pass 2:** [V] OSDI 2004 pp. 121-136: SUNDR is a network FS for untrusted
servers; clients detect unauthorized changes once they have seen each other's
modifications; the permitted attack is a *fork attack* (server hides
operations), after which forked clients can never again see the same history
(USENIX abstract https://usenix.org/conference/osdi-04/secure-untrusted-data-repository-sundr
plus MIT 6.824 notes https://pdos.csail.mit.edu/archive/6.824-2010/notes/l18.txt,
secondary). The PDFs did not text-extract, so per-user signed version
structures, crash-blocking and the PODC 2002 paper are [UNVERIFIED]. The
Cachin/Shelat/Shraer fork-linearizability and Chubby/fencing attributions are
likewise [UNVERIFIED] here. Kleppmann's fencing article [V,
https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html]:
a fencing token is an increasing number issued by the lock service on each
acquire; a GC-paused holder with an expired lease can still write; the
article cites Burrows' Chubby paper for the efficiency-vs-correctness lock
distinction but does not say Chubby originated fencing tokens (Chubby's
"sequencers" are the related mechanism; [UNVERIFIED] here).
**[ADDED] A2M and TrInc:** Chun, Maniatis, Shenker, Kubiatowicz, "Attested
Append-Only Memory: Making Adversaries Stick to their Word", SOSP 2007
pp. 189-204: a trusted append-only log that eliminates *equivocation*
(telling different nodes different stories); A2M-enabled PBFT variants stay
correct and live with half the replicas faulty [V via abstract, search].
Levin, Douceur, Lorch, Moscibroda, "TrInc: Small Trusted Hardware for Large
Distributed Systems", NSDI 2009 (best paper): a non-decreasing counter plus a
key giving unique once-in-a-lifetime attestations; case studies include A2M
and PeerReview [V via abstract, https://www.microsoft.com/en-us/research/publication/trinc-small-trusted-hardware-for-large-distributed-systems/].
These are hardware-rooted answers to "prevent a writer from signing two
histories", the alternative to Choir's detect-after-the-fact equivocation
proof; they need trusted hardware at the writer, which a user-owned
workstation may or may not have.
- **Problem:** a file system on an untrusted server; the server may lie,
  reorder or hide writes.
- **Mechanism:** clients sign per-user version structures (version vectors
  over all users) with each operation; each client's view includes a signed
  history of what it saw. **Fork consistency:** if the server shows two
  clients different histories, it must keep them forked forever; if they
  ever see each other's later operations, the fork is exposed. Detection
  needs out-of-band communication (clients comparing views), not the
  server.
- **Guarantees:** server can at worst fork, not forge or silently
  reorder; clients detect the fork once they exchange any later signed
  state.
- **Failure modes:** blocking when a client crashes mid-operation
  (the paper notes this); detection is not prevention; needs
  client-to-client gossip; later work (SUNDR's successors: Venus, Depot,
  Cachin/Shelat/Shraer "Efficient fork-linearizable access to untrusted
  shared memory" 2007, CONIKS/transparency logs, Certificate Transparency
  [K]) improves liveness.
- **Mechanism vs policy:** hash/signature chain and gossip = mechanism;
  who gossips and how often = policy.
- **Mapping to the design:** the "signed equivocation proof" in
  branching-tape §3.3 is fork-consistency detection with a trusted home
  as the gossip point. Key difference: in Choir the *writer* is the
  entity that can fork (not the server), the home is the witness, and
  detection is immediate when the writer next syncs. Related prior art
  for the lease-epoch itself: Chubby (Burrows 2006), fencing tokens
  (Kleppmann, "How to do distributed locking", 2016) [K]: a lease without
  a monotonically increasing token the *resource* checks is unsafe; the
  design does check the epoch in the append CAS, so it is the right shape.

### 6.2 Append-only authenticated logs [partly V]
**Pass 2:** [V] RFC 9162 Certificate Transparency v2.0 (Dec 2021, Laurie,
Messeri, Stradling) is **Experimental**, obsoletes RFC 6962, Merkle-tree
append-only logs that make misissuance detectable rather than preventable
(https://www.rfc-editor.org/rfc/rfc9162). [V partial] SSB: ssb-db chains each
message to the hash of the previous one and feeds replicate in order
(https://github.com/ssbc/ssb-db, https://github.com/dominictarr/secure-scuttlebutt);
the documented way feeds fork is **one identity on two devices / a stale copy**
posting with a reused sequence number and previous-hash (Patchfox issue
via search). **[CORRECTED]** the "restored from backup" fork is plausible
under the same mechanism but I found no source; "clients refuse the feed
afterwards" and the "SSB fork bug" lore remain [UNVERIFIED]. Trillian and
Bluesky AT Protocol (repo per DID, signed commit chain, PDS migration)
[UNVERIFIED, not fetched]. Tarr et al. ICN 2019 [UNVERIFIED].
Certificate Transparency (RFC 6962/9162), Trillian, Secure Scuttlebutt
(feeds per identity: one writer per feed, fork = **corrupted/forked feed**,
detected by two messages with the same sequence number and clients refuse
the feed afterwards: *this is exactly Choir's branch-with-single-writer,
with the policy "fork = identity compromise, stop following"*), Bluesky AT
Protocol (repo per DID with signed commit chain, PDS migration). SSB is
the closest real deployment of "one writer per log, offline-first, forks
mean trouble"; its failure mode is that users who restored from backup
produced forks and lost their identity feed (the "SSB fork bug" lore
[U]).

---

## 7. Speculation, tentative work, and external effects

### 7.1 Speculator / speculative execution in distributed FS (Nightingale, Chen, Flinn, SOSP 2005) [V]
**Pass 2:** [V via abstract, https://www.microsoft.com/en-us/research/?p=154829]
SOSP 2005 pp. 191-205, DOI 10.1145/1095810.1095829; Linux kernel support;
processes share speculative state by tracking causal dependencies through IPC;
speculative processes cannot externalize output (network, screen) until the
speculation is confirmed; ~2x gain for NFS on LAN and an order of magnitude on
WAN (PostMark, Andrew-style). Extended in ACM TOCS 24(4):361-392, 2006
**[ADDED cite]**. Checkpoint/rollback specifics are [UNVERIFIED] (PDF unreadable).
- **Problem:** blocking on remote results costs round trips.
- **Mechanism:** checkpoint the process (fork-style), continue with the
  predicted result, track causal dependencies when speculative
  processes interact (IPC, files), **hold external output (screen, network)
  until the speculation is validated**, and roll back to the checkpoint
  on mis-speculation.
- **Failure modes:** side effects that cannot be buffered; correlated
  mis-speculation; speculative state sharing complexity.
- **Mapping:** this is the rigorous form of "do not let a tentative
  (local-durable) fact cause an irreversible effect until the home
  confirms (home-durable)". Choir's *precommitment record* plays the role
  of the confirmed speculation; the "outbox as draft with intent" plays the
  buffered output. The difference: Choir cannot roll back a branch's
  tentative history (the tape is append-only evidence); it can only
  *abandon* it as a non-primary branch.

### 7.2 Optimistic replication survey (Saito & Shapiro, ACM Computing Surveys 37(1), 2005) [V-cite]
**Pass 2:** vol. 37 no. 1, pp. 42-81, DOI 10.1145/1057977.1057980; abstract
names ordering, conflict detection/resolution, propagation and divergence
bounding [V]. The single-/multi-master, state-/operation-transfer taxonomy
and the "single-master is simplest" observation below were not re-read
[UNVERIFIED, but match the paper's well-known structure].
Taxonomy that organises sections 2-4: *single-master vs multi-master*;
*state transfer vs operation transfer*; *scheduling* (epidemic, primary
commit); *conflict handling* (detection: syntactic via version vectors or
semantic via preconditions; resolution: manual/automatic/ commutative ops,
precondition-merge); *commitment* (agreement, primary commit, voting).
Observation: **single-master optimistic replication is the simplest
point, with multi-master requiring conflict handling**. The Choir design
sits in "single-master per branch, operation transfer (event log),
primary-commit (home acknowledges), no automatic resolution".

---

## 8. Offline-first queues and outboxes; exactly-once under retry

### 8.1 Mobile outboxes (iOS background tasks, Android WorkManager, Firebase queue, Apple CKSyncEngine, Matrix/Signal send queues) [UNVERIFIED in pass 2; not fetched]
**[ADDED] Replicache (and successor Zero) sync model:** server datastore is
canonical; client mutator calls are recorded as pending mutations, pushed in
batches and re-run by server-side mutators; locally unconfirmed mutations are
replayed on the new server state (rebase-like); mutators must take IDs as
parameters because they may be replayed [V partial: official docs
https://doc.replicache.dev/concepts/how-it-works; the "rebase" wording and
Zero as successor are from third-party blogs]. It is the clearest current
example of the Bayou shape (server order, client tentative queue) that the
survey contrasts with Choir's client-as-writer model. Linear's sync engine
was not found.
- **Mechanism:** the app writes an intent to a durable local queue
  immediately; a background worker drains it on connectivity with
  retry/backoff; each item carries an idempotency key; server dedups.
  Android's WorkManager adds constraints (network, charging), exponential
  backoff and unique-work policies; iOS BGTaskScheduler is opportunistic
  (not guaranteed), so a pending item can wait for hours.
- **Failure modes:** duplicates from ack loss (retry after the server
  already processed), reordering across items, queue items outliving the
  credentials/permissions that authorised them (re-validate at send time),
  stuck head-of-line items, and user-visible "sent" state that is only
  "queued".
- **Mapping:** the phone's outbox ("writes from the phone while the writer
  is on a sleeping laptop wait in the phone's outbox", fabric §1.3) is this
  pattern. It needs idempotency keys that the writer honors across
  *epochs and branches* (branching-tape §3.2's ancestry-aware idempotency
  is exactly this).

### 8.2 Transactional outbox / idempotent receivers / exactly-once [UNVERIFIED in pass 2; Stripe Idempotency-Key, Kafka transactions, Garcia-Molina & Salem Sagas 1987 not fetched]
"Exactly-once" for effects on the world reduces to: *at-least-once
delivery + idempotent effect keyed by a stable id*, or at-most-once with a
possibility of loss. Where the external system cannot dedup (SMTP send),
the best available is: record intent durably (home-durable), perform the
effect with a unique message-id, and on uncertain outcome *reconcile by
reading* (e.g. search Sent for the Message-ID) rather than retry blindly.
Stripe-style `Idempotency-Key` headers, Kafka transactional producers,
Saga/compensation patterns (Garcia-Molina & Salem 1987) [K] are the
usual toolkit. The unavoidable gap: crash between "effect happened" and
"effect recorded" → at most one *ambiguous* effect per crash, which must be
shown to the owner.

---

## 9. Local-first software (Kleppmann, Wiggins, van Hardenberg, McGranaghan; Ink & Switch, Onward! 2019) [V, with CORRECTIONS]
**Pass 2 [CORRECTED]** from https://www.inkandswitch.com/essay/local-first/ :
seven ideals are Fast, Multi-device, Offline, Collaboration, Longevity,
Privacy, User control (the survey's list below is a paraphrase, not the
essay's names). Scores: file sync (Dropbox) strong on speed/longevity/control,
"the weakest point ... is the lack of real-time collaboration"; Git/GitHub
good for async work, lacks real-time fine-grained collaboration, weak on
mobile; Firebase (and CloudKit/Realm) is proprietary, low on
privacy/longevity/user control. CRDT prototypes: "user experience with offline
work is splendid", but libraries do not fully realize the ideals; network
communication and history growth remain open. **Upwelling is not mentioned**
in the essay; Trellis is built on Automerge. The "conflicts rare" finding and
"Git poor for non-text" are **not stated** there [UNVERIFIED].
- **Problem:** cloud apps lose ownership, offline, longevity.
- **Mechanism (a position paper):** seven ideals: no spinners (data on the
  device is primary), not trapped on one device, network optional, seamless
  collaboration, the Long Now, security and privacy by default, ultimate
  ownership. Technical suggestion: CRDTs for merge, a sync server as an
  optional encrypted relay.
- **Findings from the paper [V]:** Dropbox: conflicted copy; Git: poor for
  non-text, no real-time; Firebase: proprietary, not ownable; CRDT
  prototypes (Trellis, Upwelling, Automerge) found conflicts rare; the
  unresolved cases are same-property concurrent edits, history growth,
  and network layer.
- **Mapping:** Choir's "primary copy lives on the Mac, server is
  follower" is "local-first with a relay"; the paper's honest weakness
  (concurrent edits to the same property, history growth) is the
  rejection reason for automatic merge in branching-tape §8. Choir
  differs by being **agent-driven**, where an offline writer is not a
  human but a process that can emit irreversible effects at machine
  speed, a case the local-first literature largely does not cover.

---

## 10. Leases, fencing, failover (adjacent to the above; needed for Q1)

- **Leases (Gray & Cheriton 1989) [V partial]:** time-bound grants; safety
  requires bounded clock drift; recovery by waiting out the term. Pass 2:
  confirmed that correctness assumes clocks with bounded drift (a fast server
  or slow client breaks consistency; clock failure is treated as Byzantine),
  non-Byzantine failures hurt performance not correctness, and short terms
  (~10 s) were favored [secondary summaries,
  https://blog.acolyer.org/2014/10/31/leases-an-efficient-fault-tolerant-mechanism-for-distributed-file-cache-consistency/ ,
  paper https://courses.cs.duke.edu/spring15/cps210/readings/leases-sosp89.pdf].
  Note the original use is cache consistency, not writer exclusivity.
- **Fencing tokens / epochs (Chubby 2006; ZooKeeper zxid; Raft term;
  Kleppmann 2016) [K]:** the resource rejects operations with a lower
  token. Safety does not depend on clocks, only on the resource's check.
- **Failover and STONITH (Pacemaker) [K]:** when a primary is
  unreachable, a standby cannot know whether it is dead or partitioned;
  safe failover needs either a lease timeout (clock assumption), a
  quorum, or *forcibly powering the old primary off*. Without any of the
  three, automatic failover risks split-brain.
- **Primary-backup with a view service (Viewstamped Replication, Raft
  leader election, Paxos) [K]:** a majority quorum decides who is the
  leader, so a minority-side leader that does not know it was deposed
  *cannot commit* (its writes need a majority). In Choir, a single home
  database plays the quorum, and the minority-side Mac **can** keep writing
  locally (because its store is local), the difference that makes
  the equivocation/branch problem exist at all.
- **Failure modes:** GC pauses/clock jumps expiring leases while a
  holder still acts (Kleppmann); lease database restored from backup
  rolling epochs back (the design names it, §12); manual lease break
  leaving two live writers.

---

## 11. Synthesis: mechanism vs policy across the field

| Layer | Mechanism (small, provable) | Policy (config, org/owner-specific) |
|---|---|---|
| Authority | epoch-fenced append CAS; monotone epochs anchored in signed receipts | who may check out; whether leases expire; who may break a lease; auto-failover thresholds |
| Divergence | hash-chain fork detection; signed equivocation proof; branch record with base | which branch is primary; default after reconnect; retention of losers; branch cap |
| Reconciliation | none (no merge), or per-kind typed merge function (Irmin), or CRDT lattice | which kinds may be merged/adopted automatically; owner vs admin approval |
| Durability levels | local-durable vs home-durable acks; push protocol | push cadence; battery/metered backoff; which effect classes may fire on local-durable only |
| Effects | precommit record + idempotency key + outcome reconciliation | classes allowed offline; spend limits; quarantine of effects on a non-primary branch |
| Client sessions | (branch, seq, digest) token for RYW | whether phones may open branches ("work here anyway") |

**Observed regularities across the prior art:**
1. Every system that allows offline writers makes one of three choices:
   *prevent* (pessimistic lock/lease: RCS, Coda server volumes with
   callbacks, Perforce), *detect and keep both* (Coda, Ficus, Notes,
   CouchDB, Dropbox, Git, Fossil), or *converge automatically* (CRDT, OT,
   Dynamo with merge). The Choir design combines prevent (normal path) and
   detect-and-keep-both (escape path).
2. **Stale-lock handling is the unavoidable policy question** in the
   prevent approach: the designs differ by (a) timeouts, (b) administrator
   break, (c) STONITH, (d) quorum. All of them trade availability for
   safety or the reverse.
3. **Irreversible effects are the part nobody solved by replication
   alone**; every working system gates them behind a confirmed record
   (Speculator, outbox, idempotency keys).
4. **Reset / purge / restore-from-backup** is the dominant real-world
   failure (Notes zombies, Realm client reset, SSB forks, epoch
   rollback), not the clean partition.
5. Conflicts nobody sees are conflicts that lose data; every product
   that worked made the conflict a visible object.

---

## 12. Design options per question (no recommendation)

### Q1 Writer authority across sites, offline work, failover

| Option | Mechanism | For | Against / failure mode | Prior art |
|---|---|---|---|---|
| A1 Checkout lease, no expiry, fence-then-grant (current draft) | home epoch row; append CAS; handoff event | no timing assumptions; offline work is never killed; conflict-free in the normal path | availability: a dead/lost laptop blocks the writer until an owner act; relies on the owner or policy to revoke; offline Mac cannot learn of a revoke so it keeps writing the old epoch locally (needs branch/equivocation handling) | RCS/Perforce locks, Chubby sans TTL, Coda server volumes |
| A2 Lease with TTL + self-fence | writer stops when it cannot renew | automatic, bounded split window (clock-dependent) | kills offline work; clock skew/GC pauses; a sleeping laptop self-fences on wake; was the 2026-10-28 proposal and rejected | Gray-Cheriton leases |
| A3 Lease with TTL, but expiry only *permits* a standby branch (not a fence of the old writer) | expiry = permission to open a host branch; old writer never forcibly stopped | offline work survives; failover without owner act | produces branches by design (every long offline period); needs the effect policy for the standby | the draft's "failover branch for standing intents" is a restricted form |
| A4 Quorum/consensus-owned lease (e.g. home replicated across 3 sites, Raft) | majority decides epoch | no single-point home; survives home loss | complexity; still does not stop an offline minority-side local writer | Raft, Spanner Paxos groups |
| A5 STONITH-like forced stop (remote kill of the old writer's VM when reachable) | home command to workstation host | removes split-brain while reachable | not available during the partition, the very case | Pacemaker |
| A6 Multi-writer per computer with automatic merge | version vectors/CRDT/OT | no unavailability | explicitly rejected; breaks hash-chain linearity; invariants (single pending transition, queues, Texture revisions) need coordination anyway | Bayou, COPS, Automerge |
| A7 No lease; every writer proposes, a sequencer orders (Bayou primary) | home assigns order; sites send tentative ops | converges with one history | the order is not the writer's own chain; writer needs rollback; contradicts "VM never writes the tape" | Bayou, Corfu-like logs |

Sub-options on the *offline-tail disposition* (what happens when the old
writer reconnects at a stale epoch):
- B1 reject and discard (lossy; Realm client reset): rejected by the
  "nothing is lost" intent;
- B2 import as a branch, owner picks primary (draft);
- B3 import as a branch, automatic default primary by policy (largest
  tail / lease holder / most recent / org rule) with owner override;
- B4 import as a branch and run a typed reconcile for chosen kinds (Irmin
  style), human for the rest;
- B5 quarantine the tail: store but hold effects and adoption until
  reviewed.

Sub-options on *failover*: none (wait), owner-triggered branch, policy-triggered
branch for standing intents only (draft), full promotion of the host copy
(with B2 on return).

### Q2 Reachability, home, multiple homes
(Mostly outside this survey; relevant prior art for the *authority* side.)
- One home per computer holding the lease row (draft): simple; home is a
  CP point; home loss blocks lease changes (restore-from-backup rolls
  epochs back; receipts anchor).
- Home as a replicated group (Raft/Paxos): availability, complexity.
- Multiple homes with a per-computer *home-of-record* and secondary
  followers (read-only): reads survive; writes need the home.
- Home movable by an owner act (a handoff event on the tape plus a
  signed new-home key; analogous to Bluesky PDS migration or Matrix
  server migration [K,U]): needs a rule for who signs the move if the old
  home is dead (the owner key).
- Session tokens (branch, seq, digest) so a phone moving between homes
  keeps read-your-writes (§3.4).
- Fork-consistency view: any follower that has seen head H at epoch E
  is a witness against a later conflicting head at (E, same seq); more
  followers = more witnesses (SUNDR/CT gossip), which is a use of
  multiple homes beyond availability.

### Q5 Irreversible effects under partition and offline
| Option | Mechanism | Trade-off |
|---|---|---|
| E1 Fire only from home-durable precommit (draft) | one push RTT before effect; outbox as draft | no effects offline; cannot send mail on a plane; strong audit |
| E2 Per-class offline policy | allow classes (e.g. reversible, low-value, idempotent keyed) on local-durable | the class lattice and limits become the policy surface; needs a bound (count/value) that is enforceable without the home (pre-issued tokens/budgets, e-cash-style) |
| E3 Pre-issued capability budget | home signs a budget (N mails, $X) valid for the epoch; the writer spends offline; budget spent is on the tape | bounded loss under partition and bounded duplicate if branched (the budget is per-epoch, so branches share it: must be checked after the fact) |
| E4 Effect proxy with idempotency | effects go through a service (gateway/mailer) that dedups on (computer, effect-id) and refuses unknown epochs | exactly-once-ish for effects that pass the proxy; the proxy is an always-on dependency; offline needs queueing at the proxy |
| E5 Compensating effect (Saga) | design the effect to be undoable (recall, void) | not available for mail/post/payment settlements |
| E6 Quarantine on non-primary branch | effects emitted by branch not-primary are held (draft) until the owner confirms | prevents duplicate sends after a split; delays standing intents on the failover branch |
| E7 Ambiguity record | on crash between effect and record, mark "outcome unknown", reconcile by reading the external system | the only honest answer for non-idempotent endpoints; needs an owner surface |

Prior art converging on the first row: Speculator (hold output until
confirmed), Bayou (tentative writes are not durable facts), transactional
outbox, SSB/CT (append, then act). Note the draft's own hole: the
*home-durable precommit does not prevent the same precommit being
fired from two branches* after an equivocation; effect ids must be
globally unique per (computer, intent) and the actuator (not the branch)
must dedupe, which is E4.

### Q8 Organization lifecycle (partition/branch angle only)
- Offboarding while the employee's laptop is offline with a checked-out
  lease: the org cannot fence an offline holder. Options: (i) the computer
  goes with the employee (draft; nothing to fence); (ii) org revokes the
  site key in the trust set, effective at the next sync, accepting that the
  offline tail is a retained non-primary branch (draft's W4); (iii)
  time-bounded checkout for org-owned computers or sites (policy: TTL
  only for org-owned, never personal), reintroducing A2/A3 for that class;
  (iv) device attestation (MDM/TPM) that wipes on unenroll.
- Org data on a departing branch: publication/adoption already limits it;
  the org's retained copy is the last home-durable state: *home-durable
  cadence becomes a retention-of-record policy*.
- Many orgs want a *break-glass* for lost laptops: the A1 "owner revoke"
  with an org-admin delegate; same mechanism, different principal.
- Mapping to prior art: Lotus Notes ACLs + replica purge intervals;
  Dropbox Business remote wipe and account transfer; Apple Business
  Manager / Android Enterprise managed-device wipe [K,U].

### Q9 Mechanism vs policy, verifying the configuration space
Prior art on making a *parameterised* protocol safe for all parameters:
- **Safety invariants independent of parameters:** single writer per
  (branch, epoch); epoch monotone; append CAS compares (epoch, head);
  effects need a durable intent. Parameters (TTL, thresholds, push
  cadence, offline classes, retention, primary default) only affect
  *liveness* or *loss bounds*, not these invariants. This is the
  structure of Paxos/Raft (safety independent of timeouts, liveness
  dependent) [K]. A design where **every policy knob is a liveness or
  loss-bound knob and none can touch a safety invariant** is the form
  verification wants; the knobs that *can* touch safety (lease expiry
  that fences a still-live writer; auto-primary choice that un-revokes
  a key; offline effect classes) are the ones needing proof/bounds.
- **Model checking:** TLA+ (Lamport; used at AWS, Newcombe et al. CACM 58(4)
  66-73, April 2015 [V: AWS engineers used TLA+ on 10 large systems; the paper
  itself notes verified designs do not prove the code matches the design,
  https://cacm.acm.org/magazines/2015/4/184701-how-amazon-web-services-uses-formal-methods/fulltext];
  the rest of this bullet (P, Alloy, FoundationDB/TigerBeetle/Antithesis
  simulation, Verdi PLDI 2015, IronFleet SOSP 2015) is [UNVERIFIED]: searches
  returned nothing for Verdi/IronFleet) for the lease/branch/switch state machine across epochs,
  crashes, partitions, restore-from-backup; Alloy for the trust/ACL
  configuration space; P language (AWS/Microsoft) for async
  message-passing; Jepsen-style fault injection (partitions, clock skew,
  process pauses, kill -9) against the real implementation; deterministic
  simulation testing (FoundationDB, TigerBeetle VOPR, Antithesis) [K] for
  exploring configurations with a seeded simulator; Verdi/IronFleet
  (verified distributed systems in Coq/Dafny) [K] for the strongest
  assurance of the small kernel. The kernel being "append CAS plus epoch
  plus branch record plus switch transaction" is small enough for TLA+
  exhaustive checking with a bounded number of branches, epochs and
  hosts.
- **Parameter space coverage:** check the invariants over the cross
  product of knob values with a symbolic or bounded model (e.g. TTL in
  {none, short, long} x offline-effect-class x auto-failover on/off).
  Each configuration then needs no individual review ("every reasonable
  configuration is safe").
- **CALM / I-confluence (Hellerstein; Bailis et al. 2014) [K]:** a
  decision procedure for which operations/invariants need coordination at
  all; it can classify the Choir app kinds (an append-only evidence
  event is I-confluent; "exactly one pending transition" and "balance>=0"
  are not), telling the kernel which kinds need single-writer leases and
  which may be merged.

---

## 13. Open uncertainties in this survey (after pass 2)
Resolved in pass 2: Realm EOL (confirmed, 2025-09-30); Yjs and OT TP2
citations (confirmed); Lotus Notes winner rule and zombie deletions
(confirmed); Fossil fork definition (confirmed, with correction).

Still [UNVERIFIED] (tried and failed, or not attempted):
- Jepsen hierarchy; PACELC classifications (fetch errors).
- Bayou internals ((timestamp, replica) order, undo); SUNDR version structures
  and crash blocking; Speculator rollback; Coda CVV granularity, replay
  rejection granularity (all: paper PDFs not text-extractable by the fetcher).
- CouchDB winner rule and `_revs_limit`; Couchbase Lite; Firebase; CloudKit;
  Dropbox three-tree model, 4 MiB blocks, conflicted-copy rule.
- Darcs/Pijul; Irmin mechanism and its relation to MRDT; Git/RCS/Perforce lock
  details (not attempted).
- CRDT tech report (blocked), COPS, Dynamo, Ficus, Automerge (V-prior, not
  re-opened), Parker 1983, I-confluence/CALM, Cachin et al., Chubby sequencers.
- Outbox/idempotency (Stripe, Kafka, WorkManager, BGTaskScheduler), Sagas.
- Verdi, IronFleet, P, Alloy, DST tools; Pacemaker/STONITH; Bluesky/Matrix
  migration; MDM wipe features.
- SSB backup-restore forks; Trillian.

---

## 14. Primary sources

Added in pass 2 (all retrieved at least at abstract level):
- Brewer, "CAP Twelve Years Later", IEEE Computer 45(2), 2012, DOI 10.1109/MC.2012.37. https://www.infoq.com/articles/cap-twelve-years-later-how-the-rules-have-changed
- Jepsen, Causal consistency. https://jepsen.io/consistency/models/causal
- Attiya, Ellen, Morrison, PODC 2015. https://arxiv.org/pdf/1512.00168 (survey of results)
- Terry et al., Session Guarantees, PDIS 1994. https://www.cs.cornell.edu/courses/cs734/2000FA/cached%20papers/SessionGuaranteesPDIS_1.html
- Kleppmann & Howard, Byzantine Eventual Consistency, arXiv:2012.00472.
- Gentle & Kleppmann, Eg-walker, EuroSys 2025. https://arxiv.org/abs/2409.14252
- Chun et al., A2M, SOSP 2007; Levin et al., TrInc, NSDI 2009 (links in §6.1).
- Jujutsu conflicts docs. https://docs.rs/crate/jj-cli/0.41.0/source/docs/conflicts.md
- RFC 9162. https://www.rfc-editor.org/rfc/rfc9162
- MongoDB Atlas Device SDKs deprecation. https://www.mongodb.com/docs/atlas/device-sdks/deprecation.md
- HCL Notes replication conflicts. https://help.hcl-software.com/notes/10.0.1/rep_save_conflicts_t.html
- Replicache docs. https://doc.replicache.dev/concepts/how-it-works
- Git LFS file locking. https://github.com/git-lfs/git-lfs/wiki/File-Locking
- Newcombe et al., CACM 58(4), 2015 (link in §12).
- Gray & Cheriton, Leases, SOSP 1989 (link in §10).

Original list follows. Entries were "verified" in pass 1 only at the
level of "located via search"; pass 2 re-opened only those with links above.

Verified this session [V] (located via search; full text not always read):
- Kistler & Satyanarayanan, "Disconnected Operation in the Coda File System", ACM TOCS 10(1), 1992. https://courses.cs.washington.edu/courses/cse550/10au/papers/coda.pdf ; https://classpages.cselabs.umn.edu/Spring-2018/csci5105/papers/coda.pdf
- Satyanarayanan et al., "Coda: A Highly Available File System for a Distributed Workstation Environment", IEEE TC 1990 (title from knowledge [K]).
- Terry, Theimer, Petersen, Demers, Spreitzer, Hauser, "Managing Update Conflicts in Bayou, a Weakly Connected Replicated Storage System", SOSP 1995, doi:10.1145/224056.224070. https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/bayou.pdf
- Saito & Shapiro, "Optimistic Replication", ACM Computing Surveys 37(1), 2005, doi:10.1145/1057977.1057980. https://www.lip6.fr/Marc.Shapiro/papers/Optimistic_Replication_Computing_Surveys_2005-03_cameraready.pdf ; https://hal.archives-ouvertes.fr/hal-01248208
- Mazières & Shasha, "Building secure file systems out of Byzantine storage", PODC 2002; Li, Krohn, Mazières, Shasha, "Secure Untrusted Data Repository (SUNDR)", OSDI 2004. https://cs.nyu.edu/~shasha/papers/sundr.pdf ; https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sundr.pdf
- Nightingale, Chen, Flinn, "Speculative Execution in a Distributed File System", SOSP 2005, doi:10.1145/1095810.1095829. https://web.eecs.umich.edu/~pmchen/papers/nightingale05.pdf
- Preguiça, Baquero, Almeida, Fonte, Gonçalves, "Dotted Version Vectors: Logical Clocks for Optimistic Replication", arXiv:1011.5808, 2010. https://arxiv.org/abs/1011.5808
- Lloyd, Freedman, Kaminsky, Andersen, "Don't Settle for Eventual: Scalable Causal Consistency for Wide-Area Storage with COPS", SOSP 2011. https://web.eecs.umich.edu/~manosk/assets/papers/cops.pdf
- DeCandia et al., "Dynamo: Amazon's Highly Available Key-value Store", SOSP 2007, doi:10.1145/1294261.1294281. https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/dynamo.pdf
- Bailis, Davidson, Fekete, Ghodsi, Hellerstein, Stoica, "Highly Available Transactions: Virtues and Limitations", VLDB 2014. https://arxiv.org/abs/1302.0309
- Kleppmann & Beresford, "A Conflict-Free Replicated JSON Datatype", IEEE TPDS 2017. https://arxiv.org/abs/1608.03960
- Kleppmann, "Making CRDTs Byzantine Fault Tolerant", PaPoC 2022. https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf
- Kleppmann, Wiggins, van Hardenberg, McGranaghan, "Local-first software: you own your data, in spite of the cloud", Onward! 2019. https://www.inkandswitch.com/essay/local-first/
- Reiher, Heidemann, Ratner, Skinner, Popek, "Resolving File Conflicts in the Ficus File System", USENIX Summer 1994. https://www.usenix.org/legacy/publications/library/proceedings/bos94/heidemann.html ; Popek, Guy, Page, Heidemann, "Replication in Ficus Distributed File Systems", 1990. https://ant.isi.edu/~johnh/PAPERS/Popek90a.html ; Page et al., "Perspectives on Optimistically Replicated, Peer-to-Peer Filing", SP&E 1998. https://ant.isi.edu/~johnh/PAPERS/Page98a.html
- Farinier, Gazagnaire, Madhavapeddy, "Mergeable persistent data structures", JFLA 2015. https://hal.archives-ouvertes.fr/hal-01099136v2 ; Kaki, Priya, Sivaramakrishnan, Jagannathan, "Mergeable Replicated Data Types", PACMPL 3(OOPSLA) 2019.
- Gilbert & Lynch, "Brewer's conjecture and the feasibility of consistent, available, partition-tolerant web services", SIGACT News 33(2), 2002 (MIT copy reported at groups.csail.mit.edu/tds/papers/Gilbert/Brewer2.pdf, not opened). Abadi, "Consistency Tradeoffs in Modern Distributed Database System Design", IEEE Computer 2012 (reported at https://www.cs.umd.edu/~abadi/papers/abadi-pacelc.pdf, not opened).

Not verified this session [K]:
- Brewer, "Towards Robust Distributed Systems" (PODC keynote 2000); Brewer, "CAP twelve years later", IEEE Computer 2012.
- Shapiro, Preguiça, Baquero, Zawirski, "Conflict-free Replicated Data Types", SSS 2011 (INRIA RR-7687); "A comprehensive study of CRDTs", INRIA RR-7506 2011.
- Terry et al., "Session Guarantees for Weakly Consistent Replicated Data", PDIS 1994.
- Parker et al., "Detection of mutual inconsistency in distributed systems", IEEE TSE 1983 (version vectors).
- Mahajan, Alvisi, Dahlin, "Consistency, Availability, and Convergence", UT Austin TR 2011.
- Bailis et al., "Coordination Avoidance in Database Systems" (I-confluence), VLDB 2015; Alvaro et al., "Consistency Analysis in Bloom: a CALM and Collected Approach" CIDR 2011.
- Ellis & Gibbs, "Concurrency control in groupware systems", SIGMOD 1989; Nichols et al., "High-latency, low-bandwidth windowing in the Jupiter collaboration system", UIST 1995.
- Nicolaescu, Jahns, Derntl, Klamma, "Near Real-Time Peer-to-Peer Shared Editing on Extensible Data Types" (Yjs/YATA), GROUP 2016.
- Roundy, Darcs (2005); Mimram & Di Giusto, "A categorical theory of patches", 2013 (arXiv:1311.3903); Pijul https://pijul.org.
- Fossil: https://fossil-scm.org/home/doc/trunk/www/concepts.wiki and https://fossil-scm.org/home/doc/trunk/www/branching.wiki
- Git: https://git-scm.com/book/en/v2 ; Irmin: https://irmin.org
- Lotus Notes replication: IBM/HCL Notes documentation "Replication conflicts" (URL not checked).
- Dropbox: "Rewriting the heart of our sync engine" (2020) https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine
- Apple: CloudKit `CKError.serverRecordChanged`, `CKSyncEngine` (WWDC23); NSFileVersion docs.
- CouchDB replication and conflicts: https://docs.couchdb.org/en/stable/replication/conflicts.html
- Couchbase Lite, Realm/Atlas Device Sync, Firebase offline docs (URLs not checked).
- Gray & Cheriton, "Leases: an efficient fault-tolerant mechanism for distributed file cache consistency", SOSP 1989; Burrows, "The Chubby lock service", OSDI 2006; Kleppmann, "How to do distributed locking" (2016) https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
- Cachin, Shelat, Shraer, "Efficient fork-linearizable access to untrusted shared memory", PODC 2007.
- Tarr et al., "Secure Scuttlebutt: an identity-centric protocol for subjective and decentralized applications", ICN 2019.
- Garcia-Molina & Salem, "Sagas", SIGMOD 1987.
- Newcombe et al., "How Amazon Web Services Uses Formal Methods", CACM 2015; Lamport, Specifying Systems (TLA+); Wilcox et al., Verdi, PLDI 2015; Hawblitzel et al., IronFleet, SOSP 2015.
- Jepsen consistency map: https://jepsen.io/consistency (page fetched, models list did not render).
