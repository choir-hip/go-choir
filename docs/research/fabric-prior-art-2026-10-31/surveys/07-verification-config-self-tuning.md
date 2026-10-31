# Survey 07 (verified revision): designing a safe configuration space, verifying it, and tuning within proven bounds (2026-10-31)

## Verification log

Method: each cited system and load-bearing claim was looked up with WebSearch
(standard mode) on 2026-10-31. Search returns summaries of primary pages
(arXiv, USENIX, ACM/AAAI/PMLR proceedings, vendor docs, status pages); I did
not open every PDF in full, so "[V]" means: the source exists, and the
specific claim tagged was found stated in a primary or primary-hosting page
(or, where noted, a secondary page quoting it). Counts are approximate (one
count per cited work or load-bearing claim).

| Outcome | Count (approx.) |
|---|---|
| Confirmed [V] (existence + the tagged claim) | ~62 |
| [CORRECTED] | 13 |
| [UNVERIFIED] (searched and not found, or not searched; reason given) | ~38 |
| [ADDED] (gap-filling prior art, verified) | 7 |

Tags used below: **[V]** confirmed with URL; **[CORRECTED]** the survey was
wrong or imprecise, fix stated; **[UNVERIFIED]** could not confirm (what was
tried is stated, or "not searched in this pass"); **[ADDED]** new item.
Original tags [M]/[?] are dropped where a new tag replaces them. Where the
original text is unchanged and still unverified it is marked [UNVERIFIED].

Most consequential corrections:
1. **Trace validation is not a proven gap-closer for every spec.** MongoDB's
   "eXtreme modelling in practice" (VLDB 2020; authors are Davis,
   Hirschhorn, Schvimer; the survey misattributed it) reports trace-checking
   was *impractical* for the Server's highly abstract replication spec, while
   model-based test-case *generation* worked for Realm Sync. The CCF work
   (Cirstea, Kuppe, Loillier, Merz) is a separate, positive result. P2/P4
   therefore need "the spec must be written close enough to the code to be
   trace-checkable" as a stated condition. (§2.1, P4)
2. **Neural Simplex was NFM 2020, not AAMAS 2020** (arXiv 1908.00528). (§4.2)
3. **Hydra's own policy/mechanism paper concludes that total separation is
   impractical**; a parametrized policy stays in the kernel to arbitrate
   conflicting physical-resource requests. The survey's "policies live in
   user-level subsystems the kernel only constrains" overstated it. This
   supports, rather than weakens, the "constrained parameter" class in P1.
   (§1.1)
4. **Cedar's formal model is in Lean** (no Dafny found in the FSE or OOPSLA
   papers); the analyzer claim is from the separate OOPSLA 2024 paper, which
   the survey did not cite (added). FSE'24 counts: 4 validator soundness
   bugs found by proof, 21 bugs found by testing. (§3.2)
5. **Facebook Oct 2021 was a backbone-maintenance command (with a buggy
   audit tool), not a "config push" in the Configerator sense**; BGP
   withdrawal was the consequence. The lesson (a guard failed open on an
   unusual operator action) is still apt, but the "config change" label is
   loose. (§3.1)
6. **Proof Carrying Code (POPL 1997) is by Necula alone**, not Necula & Lee.
   (§4.5)
7. **IronFleet ratio**: course slides report ~5:1 (safety) and ~8:1
   (liveness) proof-to-code; the survey's single "about 5:1" understates the
   liveness cost. Not yet checked against the paper's own table. (§2.4)
8. **Berkenkamp, Krause, Schoellig "Bayesian optimization with safety
   constraints..."**: published in *Machine Learning* vol. 112 (2023), online
   2021 (DOI 10.1007/s10994-021-06019-1); arXiv 2016. (§4.4)
9. **Edit automata**: first appeared as a 2002 workshop paper ("More
   enforceable security policies") and a 2003 Princeton TR; the survey's
   "2005" is the non-safety follow-up/journal period. (§2.9)
10. **Lampson's "Hints"** is ACM Operating Systems Review 15(5), Oct 1983
    (reprinted IEEE Software Jan 1984); I found no confirmation it was a SOSP
    paper (SOSP 1983 proceedings were published as OSR). (§1.4)

---------------------------------------------------------------------------

Scope: brief Q9 (mechanism vs policy; verifying all configurations) and Q10
(self-modifying and self-tuning systems that stay in proven bounds). Read for
context: `resource-fabric-2026-10-31.md` (§0, §9, §10) and
`branching-tape-2026-10-31.md` (§0, §1). Nothing in the repo was edited.
Mutation class of this note: green. (Statements about what the Choir design
docs say are carried over from the original survey and were not re-read in
this verification pass.)

---------------------------------------------------------------------------

## 0. The one-paragraph answer

Across every body of work below, one pattern recurs, and it is the answer to
Q9 and Q10 together:

1. Put in the **kernel** only the small set of rules whose violation is
   irreversible or silent (here: one writer per branch and epoch, hash-chain
   integrity, fencing checked at the point of commit, no write across
   computers, effects only from durable precommitments). Prove or model-check
   these.
2. Make everything else a **parameter or a policy** whose *domain is
   constrained by a predicate the kernel itself checks* (Flexible Paxos's
   quorum-intersection condition is the cleanest example; seL4's
   authority-confinement theorem over any capability graph is the strongest).
3. Verify the kernel **against the most general policy**, not against a list of
   configurations: model the policy as a nondeterministic adversary limited
   only by the kernel's guards. If the invariants hold for the havoc policy,
   they hold for every operator configuration, for a learned tuner and for a
   model-written policy alike.
4. Keep a **cheap, separately verified checker** between any proposer (human,
   agent, optimizer) and the live configuration (Simplex, shields, Cedar-style
   analysis, canary and rollback). The proposer is untrusted and may be
   arbitrarily clever; the checker is small.
5. Treat **safety** (never violate) and **performance/liveness** (do well,
   eventually) differently. A tuner may trade performance freely only when
   safety holds for *all* configurations it can reach. Liveness, cost,
   confidentiality leakage and availability are not covered by a safety proof
   and need runtime monitors, envelopes and an owner-held widening right.

The rest of the document is the evidence for this and a menu of processes.

---------------------------------------------------------------------------

## 1. Mechanism vs policy, end-to-end, hints

### 1.1 Hydra and the policy/mechanism principle
- **Problem.** Kernels that bake in one resource policy (scheduler, paging,
  protection scheme) cannot serve varied users; policy changes mean kernel
  changes.
- **Mechanism.** Hydra: Wulf et al., "HYDRA: the kernel of a multiprocessor
  operating system", CACM 17(6):337-344, 1974 [V]
  (https://home.cs.colorado.edu/~rhan/CSCI_5573_Fall_2011/Papers/wulf74.pdf);
  Levin, Cohen, Corwin, Pollack, Wulf, "Policy/mechanism separation in
  Hydra", 5th SOSP, 1975, pp. 132-140 [V]
  (https://swift.sites.cs.wisc.edu/classes/cs736-sp07/blog/2007/01/policymechanism_separation_in.html,
  secondary; the paper PDF is at aminer static link in the search results).
  The kernel provides capability-protected objects and the mechanisms to
  enforce protection; Hydra is "a base on which different operating system
  facilities can be implemented" [V]. Brinch Hansen's RC 4000 Nucleus (1970)
  as precursor [UNVERIFIED: not searched in this pass].
- **[CORRECTED] What the 1975 paper concludes.** It covers scheduling,
  paging and protection and finds that *total* policy/mechanism separation is
  impractical (overhead; risk to stability and fairness), so a **parametrized
  kernel policy** is retained, and the policy decisions left in the kernel
  exist to arbitrate conflicting requests for physical resources [V, per
  secondary summaries of the paper; check the PDF before quoting]. This is
  an existence proof for the "constrained parameter" class (the kernel keeps
  a policy but exposes bounded parameters).
- **Guarantees.** Protection (no access without a capability) holds for every
  policy subsystem because the kernel mediates every access. Not a proof, an
  architectural claim.
- **Failure modes.** Mechanism too weak to express a needed policy forces
  policy back into the kernel (this is what the 1975 paper itself found);
  Mach external pagers paid performance for separation [UNVERIFIED: not
  searched]. "Policy-free" is relative: any mechanism embeds some policy.
- **Mech vs policy.** Liedtke's minimality principle, "On µ-kernel
  construction", 15th SOSP, Dec 3-6 1995, Copper Mountain [V]
  (https://cgi.cse.unsw.edu.au/~cs9242/22/papers/Liedtke_95.pdf): a concept is
  tolerated inside the kernel only if moving it outside would prevent the
  system's required functionality; the criterion is functionality, not
  performance; the paper's own footnote says minimality cannot be proved
  because there is no agreed metric [V]. Exokernel (Engler, Kaashoek,
  O'Toole, SOSP 1995) [UNVERIFIED: not searched]. Saltzer & Schroeder, "The
  protection of information in computer systems", Proc. IEEE 1975 [V]
  (https://web.mit.edu/saltzer/www/publications/protection/): principles
  include economy of mechanism, fail-safe defaults, complete mediation, open
  design, separation of privilege, least privilege, least common mechanism
  (the full list was not seen in one place in my results; the MIT-hosted
  paper is the authority).
- **Fit.** The fabric doc already states the right split. The open work is to
  *state the kernel boundary as a list and test each item against Liedtke's
  criterion*.

### 1.2 Levin's later work, and where "mechanism" ends
Levin is the first author of the Hydra policy paper [V, above]. Do not
attribute later claims to him that are not in those papers.

### 1.3 End-to-end argument
- **Mechanism.** Saltzer, Reed, Clark, "End-to-end arguments in system
  design", ACM TOCS 2(4), 1984 (revised from a 1981 conference paper) [V]
  (https://web.mit.edu/saltzer/www/publications/endtoend/endtoend.txt). A
  function can be completely implemented only with the knowledge of the
  application at the endpoints; lower-level versions are performance
  enhancements. Examples include duplicate suppression, crash recovery and
  delivery acknowledgement [V].
- **Guarantees / failure modes.** A design rule, not a theorem; over-applied
  it pushes everything to endpoints; the paper notes lower-layer checks may
  be worth it for performance [V].
- **Fit (Q1, Q5, Q9).** Fencing check at the *commit point*, not the lease
  holder's belief; the standard illustration is Kleppmann, "How to do
  distributed locking", Feb 2016 [V, via secondary summaries; original blog
  post not returned by search]: a token that increases at each lock grant,
  with the storage service rejecting lower tokens; Redlock gives no
  monotonic token (https://hazelcast.com/blog/long-live-distributed-locks/ and
  https://www.antirez.com/news/101, which records the author's reply). Same
  logic: idempotence key at the effect endpoint, chain verification in every
  follower.

### 1.4 Lampson, "Hints for computer system design"
- Lampson, ACM Operating Systems Review 15(5):33-48, Oct 1983; reprinted
  IEEE Software 1(1), Jan 1984 [V]
  (https://bwlampson.site/33-Hints/Abstract.html). **[CORRECTED]** venue: I
  found no confirmation of SOSP (the SOSP 1983 proceedings were OSR; treat
  "SOSP 1983" as plausible but unconfirmed).
- **Hint definition [V]** (https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/hints-design.pdf
  and https://blog.acolyer.org/2016/09/16/hints-for-computer-system-design/):
  a saved result that may be wrong, checked against the "truth" before any
  unrecoverable action; hints exist for speed so must usually be right.
  Home pointers, route caches, site advertisements, "who holds the lease"
  caches are hints; the lease row and signed site record are truth.
- Other hints cited in the original (keep it simple; leave it to the client;
  log updates; make actions atomic or restartable; shed load; use static
  analysis where possible; plan to throw one away; keep a place to stand)
  [UNVERIFIED individually; the paper's hint list was not enumerated in my
  results. Lampson also published a 2020 revision, "Hints and Principles for
  Computer System Design", arXiv 2011.02455, [V] existence only].
- **Failure modes.** Hints are heuristics; the paper disclaims them as
  recipes or laws [V].

---------------------------------------------------------------------------

## 2. Formal methods in industry and research

### 2.1 TLA+ and PlusCal (Lamport); TLC and Apalache
- **Mechanism.** TLA+ (Lamport, "Specifying Systems", 2002) [UNVERIFIED: not
  searched; very widely documented]. TLC explicit-state with simulation
  mode; TLAPS proof system; PlusCal front end [UNVERIFIED individually].
  **Apalache**: Konnov, Kukovec, Tran, "TLA+ model checking made symbolic",
  PACMPL 3(OOPSLA) 123, 2019, DOI 10.1145/3360549 [V]
  (https://hal.archives-ouvertes.fr/hal-02280888). It translates the
  transition relation to quantifier-free SMT. **[CORRECTED nuance]** The
  reported advantage over TLC is clearest for *inductive-invariant* checking;
  for bounded safety checking the advantage is "less pronounced" (secondary
  reading of the paper, https://blog.acolyer.org/2019/11/29/tla-model-checking-made-symbolic/).
  The survey's claim that this gives results "beyond what explicit
  enumeration reaches" for parameterized sizes is correct only if an
  inductive invariant is supplied.
- **Industry evidence.** Newcombe, Rath, Zhang, Munteanu, Brooker, Deardeuff,
  "How Amazon Web Services uses formal methods", CACM 58(4):66-73, 2015 [V]
  (https://www.amazon.science/publications/how-amazon-web-services-uses-formal-methods).
  The DynamoDB bug with a 35-step shortest error trace is stated in Lamport's
  abridged excerpt [V] (https://lamport.azurewebsites.net/tla/amazon-excerpt.html);
  the 2014 preprint table lists a 939-line TLA+ spec for DynamoDB replication
  and group membership that found 3 bugs [V]. "Ten large systems" and
  "engineers learned the language in 2-3 weeks": [UNVERIFIED: not found in my
  search summaries; the CACM text was paywalled; check the Amazon Science PDF].
  Azure Cosmos DB's five consistency levels in PlusCal/TLA+
  (`Azure/azure-cosmos-tla`) [V, carried over from the original survey's
  verified list]. MongoDB and Confluent use TLA+: MongoDB is [V] via the
  VLDB 2020 paper below; Confluent [UNVERIFIED].
- **Guarantees / failure modes.** As original (exhaustive for checked sizes;
  spec-implementation gap; state explosion; over-abstraction; vacuity).
- **Closing the gap.**
  (a) *Trace validation/checking.* **[CORRECTED]** Davis, Hirschhorn,
  Schvimer (all MongoDB), "eXtreme Modelling in Practice", PVLDB 13(9):
  1346-1358, 2020 [V] (https://arxiv.org/abs/2006.00915). The survey's author
  list was wrong. Two case studies in TLA+: model-based *trace-checking* on
  the MongoDB Server replication protocol was **impractical** for that highly
  abstract spec; model-based *test-case generation* (MBTCG) for MongoDB Realm
  Sync was **highly successful** [V]. Cirstea, Kuppe, Loillier, Merz,
  "Validating Traces of Distributed Programs Against TLA+ Specifications",
  arXiv 2404.16075 (2024) [V] (https://arxiv.org/abs/2404.16075): record
  finite traces of variable updates, check them with TLC; applied to several
  distributed programs and found spec/implementation discrepancies in every
  case. The survey tied this to CCF; my search could not confirm the CCF
  link by name [UNVERIFIED on CCF specifically].
  (b) *Model-based test generation* from the spec: positive for Realm Sync
  [V, above].
  Lesson: closing the gap needs the spec written at (or refined to) the
  granularity of observable implementation events; a highly abstract spec
  resists trace-checking.
- **Mech vs policy.** Config as a model variable (§10.2) is how one spec
  covers a space. [design observation, unchanged]
- **Fit.** The lease/epoch/fencing/handoff/branch-primary protocol is the
  kind of small protocol where TLA+ pays. [design observation, unchanged]

### 2.2 P language (Desai, Qadeer et al.) and AWS
- **Mechanism.** Desai, Gupta, Jackson, Qadeer, Rajamani, Zufferey, "P: safe
  asynchronous event-driven programming", PLDI 2013 [V]
  (https://www.microsoft.com/en-us/research/publication/p-safe-asynchronous-event-driven-programming-2/):
  interacting state machines exchanging events; compiled to executable code
  and model-checked; default checks for unhandled events and events deferred
  forever; used to implement and verify the core of the **Windows 8 USB
  driver stack** [V].
- **[V, resolves the original [?]] P at AWS.** AWS's CACM practice article
  (https://cacm.acm.org/practice/systems-correctness-practices-at-amazon-web-services)
  states P was used to model and validate the protocol design in migrating
  **S3 to strong read-after-write consistency** (S3 index subsystem); P has
  been developed at AWS since 2019; the P team later built PObserve, which
  checks production logs against the P spec [V, AWS-authored source; no
  independent confirmation]. ShardStore (SOSP 2021) is a *separate* effort
  (lightweight methods), not the strong-consistency work.
- **Guarantees / failure modes / Fit.** As original (bounded systematic
  exploration; executable specs reduce the gap). PObserve is a relevant
  production-log-checking precedent for P7 [V as above].

### 2.3 Lightweight formal methods: Alloy; S3 ShardStore
- Alloy (Jackson, "Software abstractions", 2006) and "small scope
  hypothesis"; Alloy 6 temporal operators [UNVERIFIED: not searched in this
  pass].
- Bornholt et al., SOSP 2021 Best Paper [V]
  (https://www.cs.utexas.edu/~bornholt/papers/shardstore-sosp21.pdf):
  executable reference models, property-based testing, stateless model
  checking; **16 issues prevented from reaching production** [V, confirmed
  again via AWS-adjacent coverage]. The "40k lines of Rust" figure and
  "non-experts extend the checks" claim [UNVERIFIED in this pass; both are
  from the original V entry which only confirmed existence/gist].
  **Mechanism lesson** unchanged.

### 2.4 Verified systems: Verdi, IronFleet, Ivy, seL4
- **Verdi** (Wilcox et al., PLDI 2015) [V]: Coq framework; verify under an
  idealized fault model, then apply verified system transformers to carry
  guarantees to a more realistic fault model with no extra proof (abstract
  quoted in search; DOI/URL not captured). Verified Raft (Woos et al., CPP
  2016) [UNVERIFIED: not searched].
- **IronFleet** (Hawblitzel et al., SOSP 2015) [V]
  (https://www.microsoft.com/en-us/research/publication/ironfleet-proving-practical-distributed-systems-correct/):
  verifies IronRSL (Paxos-based replicated state machine) and IronKV
  (sharded KV store). **[CORRECTED]** proof-to-code ratio: talk slides report
  ~5:1 for safety proofs and ~8:1 for liveness proofs
  (https://web.eecs.umich.edu/~manosk/assets/slides/f21/ironfleet.pdf, a
  lecture deck, not the paper); verify in the paper's own table. "Person-years"
  [UNVERIFIED].
- **Ivy** (Padon, McMillan, Panda, Sagiv, Shoham, "Ivy: safety verification
  by interactive generalization", PLDI 2016, pp. 614-630) [V]
  (https://ilyasergey.net/CS6213/_static/papers/ivy.pdf). Restricts
  verification conditions to EPR for decidability; named SIGPLAN's Most
  Influential PLDI Paper (2016 cohort) [V]
  (https://www.sigplan.org/Awards/PLDI). "Paxos made EPR: decidable
  reasoning about distributed protocols", Padon, Losa, Sagiv, Shoham,
  PACMPL 1(OOPSLA) 108, 2017 (arXiv 1710.07191) [V]. Taube et al., PLDI 2018
  modular decidability [UNVERIFIED].
- **seL4.** Sewell, Winwood, Gammie, Murray, Andronick, Klein, "seL4 enforces
  integrity", ITP 2011 [V] (https://link.springer.com/doi/10.1007/978-3-642-22863-6_24):
  integrity (bounds writes) and authority confinement (bounds how authority
  can change), Isabelle/HOL, refined to the C implementation [V]. The l4v
  repo still carries the access-control proofs (`Syscall_AC`) [V]. The
  information-flow proof (Murray et al., S&P 2013) is described as the first
  confidentiality proof for the implementation of a general-purpose OS kernel
  [V, via a Stanford seminar abstract
  https://crypto.stanford.edu/seclab/sem-12-13/murray.html; venue S&P 2013
  not directly confirmed]. SOSP 2009 / TOCS 2014 functional correctness;
  binary-level PLDI 2013; "~9-10k lines of C, ~20 person-years"; capDL
  (Boyton et al., ICFEM 2013) [UNVERIFIED in this pass].
  **Theorem that matters:** integrity/authority confinement proved for *any*
  capability distribution in terms of an abstract access-policy graph [V in
  substance: the ITP paper proves enforcement relative to an abstract
  access-control policy; "any distribution" is the survey's gloss and fits
  the paper's framing, check the theorem statement].
- **Related recent work.**
  - **Anvil** (Sun et al., OSDI 2024, Jay Lepreau Best Paper) [V]
    (https://www.usenix.org/conference/osdi24/presentation/sun-xudong):
    verifies liveness ("eventually stable reconciliation", a temporal-logic
    property) of Kubernetes-style controllers written in Rust and verified
    with Verus; three controllers (ZooKeeper, RabbitMQ, FluentBit).
  - **Sieve** (OSDI 2022, "Automatic reliability testing for cluster
    management controllers") [V] (https://www.usenix.net/conference/osdi22/presentation/sun):
    perturbs a controller's view of cluster state; 46 serious bugs in ten
    controllers.
  - **Acto** (SOSP 2023) [V], **[CORRECTED title]**: "Acto: Automatic
    End-to-End Testing for Operation Correctness of Cloud System Management"
    (https://cs.cornell.edu/~legunsen/pubs/GuETAlActoSOSP23.pdf); 56 new bugs
    in eleven Kubernetes operators in the paper.
  - Verus (Lattuada et al., OOPSLA 2023) [UNVERIFIED: not searched]; Anvil's
    dependence on Verus is [V].
- **Failure modes / Mech vs policy / Fit.** As original.

### 2.5 Parameterized and configurable verification; software product lines
- **(a) Parameterized verification.** Apt & Kozen 1986 undecidable [UNVERIFIED];
  Emerson & Namjoshi 1995, German & Sistla 1992 cutoffs [UNVERIFIED: search
  returned nothing]; Bloem, Jacobs, Khalimov, Konnov, Rubin, Veith, Widder,
  *Decidability of Parameterized Verification*, Morgan & Claypool / Springer
  Synthesis Lectures on Distributed Computing Theory, 2015 [V]
  (https://finkbeiner.groups.cispa.de/publications/2015-decidability-of-parameterized-verification/).
  Inductive invariants prove all-N (Ivy, Apalache) [V, §2.1, §2.4]. TLC
  symmetry reduction [UNVERIFIED].
- **(b) Software product lines.** Kang et al. FODA 1990 [UNVERIFIED];
  Classen et al. featured transition systems, ICSE 2010 [UNVERIFIED: no
  results]; Kuhn (NIST) covering arrays [UNVERIFIED]; Thüm, Apel, Kästner,
  Schaefer, Saake, ACM CSUR 2014 [V as a citation only, found in another
  paper's bibliography, not the survey page itself]. Abal, Brabrand,
  Wąsowski, "42 variability bugs in the Linux kernel: a qualitative
  analysis", ASE 2014, pp. 421-432, DOI 10.1145/2642937.2642990 [V]
  (https://www.itu.dk/~brabrand/42-bugs.pdf): 42 variability bugs from
  kernel bug-fix commits.
- **Guarantees / failure modes / Mech vs policy.** As original.
- **Key example: Flexible Paxos** (Howard, Malkhi, Spiegelman,
  "Flexible Paxos: quorum intersection revisited", OPODIS 2016, arXiv
  1608.06696) [V] (https://arxiv.org/abs/1608.06696v1): only quorums of
  *different phases* need to intersect; quorums within a phase may be
  disjoint, so majorities are unnecessary; grid quorums can shrink both
  phases [V]. **[CORRECTED nuance]** The survey's "|Q1|+|Q2|>N" is the
  threshold-quorum instance of the intersection condition; the general
  condition is the cross-phase intersection requirement. A follow-up weakens
  it further: Howard & Mortier, "Relaxed Paxos: quorum intersection revisited
  (again)", arXiv 2203.03058 [V, ADDED; shows the safe condition can be
  relaxed again, i.e. an example of a published safe predicate being later
  widened].
- **Reconfiguration is the sharp edge.** Raft single-server membership-change
  bug: Ongaro posted it to raft-dev on 10 Jul 2015; found by Huanchen Zhang
  and Brandon Amos (CMU class project); affects single-server changes when
  changes are concurrent and leadership is lost; fix: new leader commits a
  no-op before accepting further changes; joint consensus unaffected [V]
  (https://groups.google.com/g/raft-dev/c/t4xj6dJTP6E). Ongaro's thesis (2014)
  as the joint-consensus source [V by listing, https://i2p.net/docs/research/ongaro2014-raft.pdf].
  Vertical Paxos (Lamport, Malkhi, Zhou, 2009) [UNVERIFIED: search returned
  nothing].

### 2.6 Property-based testing
- QuickCheck (Claessen & Hughes, ICFP 2000); Hypothesis; Go `rapid`,
  `gopter`, native fuzzing [UNVERIFIED: not searched].
- Hughes, Pierce, Arts, Norell, "Mysteries of Dropbox: property-based
  testing of a distributed synchronization service", ICST 2016, pp. 135-145,
  DOI 10.1109/ICST.2016.37 [V]
  (https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf): first
  formal testable model of a file synchronizer's core behaviour; applied to
  Dropbox, Google Drive and ownCloud; unexpected behaviour in two of three,
  some data-losing.
- **Guarantees / Mech vs policy / Fit.** As original.

### 2.7 Deterministic simulation testing (DST)
- **FoundationDB.** Zhou et al., SIGMOD 2021 (Best Industry Paper) [V]
  (https://apple.github.io/foundationdb/testing.html): integrated
  deterministic simulation framework; whole cluster in a single-threaded
  process; BUGGIFY injects faults only in simulation builds; docs estimate
  about one trillion CPU-hours of simulation [V]. Wilson, Strange Loop 2014
  talk [UNVERIFIED].
- **TigerBeetle VOPR** [V] (https://docs.tigerbeetle.com/about/vopr): the
  Viewstamped Operation Replicator, deterministic simulator testing safety
  and liveness; stubs clock, network and disk; seed-driven faults (dropped or
  reordered packets, partitions, corrupted disk I/O); reproducible given seed
  and git commit. Jepsen's analysis of TigerBeetle 0.16.11 also describes it
  [V] (https://jepsen.io/analyses/tigerbeetle-0.16.11). (Original claim
  "accelerated time, run continuously on spare compute, seeds as artifacts":
  seeds [V], the rest [UNVERIFIED].)
- **Antithesis** [V] (https://antithesis.com/company/about/): co-founded in
  2018 by Will Wilson and Dave Scherer, both of FoundationDB; deterministic
  platform that intercepts nondeterminism at the VM level so programs in any
  language can be replayed deterministically. Determinism guarantee details
  [UNVERIFIED].
- **Others.** Dropbox "Testing sync at Dropbox" (2020) [UNVERIFIED]; AWS
  **Shuttle** (randomized scheduling, reproducible, not exhaustive, scales
  better than Loom) and **Loom** (exhaustive, C11 memory model) for Rust
  concurrency [V] (https://gitee.com/mirrors_awslabs/shuttle mirror; check
  github.com/awslabs/shuttle); turmoil/madsim [UNVERIFIED]. **[CORRECTED,
  resolves the [?]]** Go `testing/synctest` first appeared in Go 1.24 under
  `GOEXPERIMENT=synctest` and **graduated to general availability in Go 1.25**
  [V] (https://go.dev/blog/16years; the official 1.25 release notes page was
  not retrieved); the pre-GA experimental API is scheduled for removal in
  Go 1.26 (per a DataDog PR, secondary). Community deterministic Go
  simulators/WASM approaches [UNVERIFIED].
- **Guarantees / Failure modes / Mech vs policy / Fit.** As original.

### 2.8 Jepsen and Elle; chaos engineering
- **Elle** [V] (https://arxiv.org/abs/2003.10554): as original, confirmed
  in the original session. Knossos, Maelstrom [UNVERIFIED].
- Basiri et al., "Chaos engineering", IEEE Software 2016; Chaos Monkey 2011
  [UNVERIFIED]; Alvaro, Rosen, Hellerstein, "Lineage-driven fault injection",
  SIGMOD 2015 [UNVERIFIED].
- **Guarantees / Failure modes / Fit.** As original.

### 2.9 Runtime verification and monitors
- Leucker & Schallhart, JLAP 2009; Lola/RTLola [UNVERIFIED].
- **Enforceability limits.** Schneider, "Enforceable security policies", ACM
  TISSEC 3(1):30-50, 2000 [V as a citation and as the origin of "execution
  monitors enforce only safety properties", via Princeton papers that state
  safety properties are an upper bound on what simple monitors can enforce
  (https://www.cs.princeton.edu/techreports/2003/681.pdf); the Schneider
  paper itself was not retrieved]. **[CORRECTED]** Ligatti, Bauer, Walker,
  "Edit automata": first "More enforceable security policies" (workshop,
  2002), Princeton TR-681-03 (2003), journal later [V]; the 2005 work
  (TR-720-05, "Run-time enforcement of nonsafety policies") proves a lower
  bound on what insert/suppress monitors can enforce and shows edit
  automata are strictly stronger than truncation automata [V]
  (https://www.cs.princeton.edu/~dpw/papers/nonsafety05.pdf).
- **Verifiable logs.** Certificate Transparency / Trillian (Merkle
  consistency and inclusion proofs) [UNVERIFIED: not searched].
- **Guarantees / Failure modes / Fit.** As original.

---------------------------------------------------------------------------

## 3. Configuration and misconfiguration

### 3.1 The empirical case
- Gray 1986 and Oppenheimer et al. USITS 2003 [UNVERIFIED: not searched].
  Yin et al., "An empirical study on configuration errors in commercial and
  open source systems", SOSP 2011, pp. 159-172 [V]
  (https://www.sigops.org/s/conferences/sosp/2011/current/2011-Cascais/12-yin-online.pdf):
  546 real misconfigurations (309 from a commercial storage system, 237 from
  four open-source systems); parameter mistakes are 70.0%-85.5% of them.
  Xu & Zhou, ACM CSUR 47(4), 2015 [V, original]. Xu et al., "Do not blame
  users for misconfigurations", SOSP 2013, pp. 244-259 [V]. PCheck, "Early
  detection of configuration errors to reduce failure damage", OSDI 2016,
  pp. 619-634 [V] (https://www.usenix.org/conference/osdi16/technical-sessions/presentation/xu):
  generated checkers detect 75+% of real latent config errors at
  initialization. ctest, "Testing configuration changes in context to
  prevent production failures", OSDI 2020 [V title/venue via a community
  list; paper not opened]. ConfErr (Keller, Upadhyaya, Candea) [UNVERIFIED on
  year: a community list says DSN 2010, the survey says DSN 2008; original
  paper not retrieved; Yin et al. cite it as injecting typos, structural
  and semantic errors].
- **Public outages** (survey said "from memory"):
  - Google Cloud, 12 Jun 2025: a policy change inserted into regional Spanner
    tables used by Service Control; a null-pointer path crashed the binary;
    replicated near-globally; incident 10:51-18:18 PDT [V]
    (https://status.cloud.google.com/incidents/ow5i3PPK96RduMcb1SsW);
    remediation includes making Service Control fail open and decoupling the
    status page (secondary summaries). Resolves the original [?].
  - Cloudflare, 2 Jul 2019: a new WAF Managed Rules rule with a regex that
    backtracked enormously exhausted CPU on every core serving HTTP/S [V]
    (https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019).
  - **[CORRECTED]** Facebook, 4 Oct 2021: the engineering post says the
    trigger was a command during routine maintenance of the backbone-capacity
    system that took the entire backbone out of operation, causing sites to
    declare themselves unhealthy and withdraw BGP advertisements; a bug in
    the audit tool meant to block such commands failed to stop it (the audit
    bug is from secondary reporting) [V]
    (https://engineering.fb.com/2021/10/05/networking-traffic/outage-details/).
    Not a "config push" in the Configerator sense.
  - The lesson ("config propagates faster than code and skips the testing
    code gets") is the survey's inference; it fits Google 2025 and
    Cloudflare 2019 (a rule shipped globally without CPU-regression
    detection, per secondary analysis).

### 3.2 Config systems
- **Configerator** (Tang et al., SOSP 2015) [V, original]. Gatekeeper
  [UNVERIFIED].
- **Safe deployment.** Gandalf: Li et al., "Gandalf: an intelligent,
  end-to-end analytics service for safe deployment in large-scale cloud
  infrastructure", NSDI 2020 **[CORRECTED title]** [V]
  (https://www.usenix.org/conference/nsdi20/presentation/li): correlates
  signals with ongoing rollouts; reported 92.4% precision and 100% recall for
  data-plane rollouts after 18+ months in Azure. Google SRE canarying
  [UNVERIFIED]. Autopilot (Rzadca et al., "Autopilot: workload autoscaling
  at Google", EuroSys 2020) [UNVERIFIED: search returned nothing].
- **Typed or constrained config languages.**
  - CUE [UNVERIFIED: search returned only Dhall]. Dhall: the official repo
    states the language is not Turing-complete and evaluation always
    terminates; all expressions have a normal form [V]
    (https://github.com/dhall-lang/dhall-lang/blob/master/README.md,
    https://www.haskellforall.com/2016/12/dhall-non-turing-complete-configuration.html).
    Jsonnet, Nickel, Pkl, Starlark, Nix module system [UNVERIFIED: not
    searched].
  - **[ADDED]** Open Policy Agent / Rego and Kubernetes Gatekeeper: a CNCF
    graduated policy engine (graduation Jan 2021, secondary) whose policies
    are written in the DSL Rego and enforced as admission control with deny,
    dry-run and audit modes [V]
    (https://www.infoq.com/news/2021/02/opa-cncf-graduation/). Relevant as an
    in-production "policy as data separate from enforcement point" design;
    Rego's termination/analysability properties are weaker than Cedar's and
    not verified here [UNVERIFIED].
- **Analyzable policy languages (most relevant).**
  - **[CORRECTED/ADDED]** Cedar: the language and analysis paper is Cutler
    et al., "Cedar: a new language for expressive, fast, safe, and
    analyzable authorization", OOPSLA 2024 (arXiv 2403.04651) [V]
    (https://arxiv.org/abs/2403.04651v2): a symbolic compiler encodes policies
    to SMT with a sound and complete encoding; supports comparing two policy
    sets for equivalence over all requests; modelled in Lean, engine in Rust.
    "How we built Cedar: a verification-guided approach", FSE 2024 industry
    track (arXiv 2407.01688) [V] (https://arxiv.org/pdf/2407.01688):
    Lean models and proofs plus differential random testing and
    property-based testing against the Rust code; proofs found and fixed 4
    soundness bugs in the validator and testing found 21 bugs across parser,
    evaluator, authorizer and validator. The survey's "Dafny/Lean" is
    **corrected to Lean** (no Dafny in the sources found).
  - Zelkova: Backes, Bolignano, Cook et al., FMCAD 2018, pp. 206-214 [V]
    (https://cs.utexas.edu/~hunt/FMCAD/FMCAD18/papers/paper3.pdf): AWS access
    policies to SMT; checks whether one policy is no more permissive than
    another; PSPACE-complete problem; run millions of times a day; basis of
    IAM Access Analyzer.
  - Batfish: Fogel et al., "A general approach to network configuration
    analysis", NSDI 2015, pp. 469-483 [V]
    (https://ratul.org/papers/nsdi2015-batfish.pdf); Minesweeper: Beckett,
    Gupta, Mahajan, Walker, "A general approach to network configuration
    verification", SIGCOMM 2017 [V]
    (https://ratul.org/papers/sigcomm2017-minesweeper.pdf): SMT encoding of
    stable states; 152 real networks, 120 violations found.
  - Margrave / XACML analysis (Fisler et al., ICSE 2005) [UNVERIFIED].
- **Guarantees / Failure modes / Mech vs policy / Fit.** As original.

---------------------------------------------------------------------------

## 4. Safe self-adaptive and self-tuning systems

### 4.1 Autonomic computing and MAPE-K
- Kephart & Chess, "The vision of autonomic computing", IEEE Computer 36(1):
  41-50, 2003, DOI 10.1109/MC.2003.1160055 [V]. Rainbow: Garlan, Cheng,
  Huang et al., "Rainbow: architecture-based self-adaptation with reusable
  infrastructure", IEEE Computer 37(10):46-54, 2004 [V]. ActivFORMS:
  Iftikhar & Weyns, SEAMS 2014, pp. 125-134, DOI 10.1145/2593929.2593944
  [V]; the journal version is Weyns & Iftikhar, ACM TOSEM 2022
  (https://arxiv.org/pdf/1908.11179) [V]. (Content claims beyond
  bibliographic data were not read.)
- Calinescu et al. CACM 2012 / TSE 2018; Filieri et al. SEAMS 2015;
  Hellerstein 2004; Cheng et al. Dagstuhl 2009; Weyns surveys; IBM blueprint
  [UNVERIFIED: not searched].
- **Guarantees / Failure modes / Mech vs policy / Fit.** As original.

### 4.2 Runtime assurance and the Simplex architecture
- Seto, Krogh, Sha, Chutinan, "The Simplex architecture for safe online
  control system upgrades", ACC 1998 [V] (IEEE record; demonstrated on a
  PECVD semiconductor process). Simplex's lineage: Sha & Rajkumar SEI report
  1995; Seto & Sha SEI report 1999 (analytically redundant controllers and a
  safety controller protecting an upgraded controller) [V]
  (https://www.sei.cmu.edu/documents/1211/1999_005_001_16772.pdf). Sha,
  "Using simplicity to control complexity", IEEE Software 2001 [UNVERIFIED:
  not returned]. Bak et al. ICCPS 2011 / RTSS 2014 [UNVERIFIED].
  ASTM F3269 [UNVERIFIED: no results].
- **[CORRECTED]** Phan, Grosu, Jansen, Paoletti, Smolka, Stoller, "Neural
  Simplex Architecture", **NFM 2020** (12th NASA Formal Methods Symposium),
  arXiv 1908.00528 [V] (https://arxiv.org/abs/1908.00528). Not AAMAS 2020.
- **Guarantees / Failure modes / Mech vs policy / Fit.** As original.

### 4.3 Shields and safe reinforcement learning
- Alshiekh, Bloem, Ehlers, Könighofer, Niekum, Topcu, "Safe reinforcement
  learning via shielding", AAAI 2018, pp. 2669-2678, arXiv 1708.08611 [V]
  (https://ojs.aaai.org/index.php/AAAI/article/view/11797): a shield sits
  alongside the learner and restricts its choices; preemptive placement
  gives the learner a list of safe actions [V].
- Bloem et al., "Shield synthesis", FMSD 2015 [UNVERIFIED].
- Fulton & Platzer, "Safe reinforcement learning via formal methods", AAAI
  2018, pp. 6485-6492 [V] (https://ojs.aaai.org/index.php/AAAI/article/view/12107):
  combines formal verification with verified runtime monitoring.
- Achiam, Held, Tamar, Abbeel, "Constrained policy optimization", ICML 2017,
  PMLR 70:22-31 [V] (https://proceedings.mlr.press/v70/achiam17a.html).
- Ames et al. 2017 (control barrier functions), Altman 1999, García &
  Fernández JMLR 2015 [UNVERIFIED: not searched].
- **Guarantees / Failure modes / Mech vs policy.** As original.
- **[ADDED] Runtime enforcement for LLM agents.** Wang, Poskitt, Sun,
  "AgentSpec: customizable runtime enforcement for safe and reliable LLM
  agents", arXiv 2503.18666, ICSE 2026 [V]
  (https://arxiv.org/abs/2503.18666): a DSL of runtime safety rules enforced
  on agent actions; reports preventing unsafe executions in >90% of code-agent
  cases and 100% compliance for the autonomous-vehicle tasks; explores
  LLM-generated rules. It is empirical, not a proof; closest published
  analogue of "agent proposes, small rule checker disposes".

### 4.4 Guarded autotuning: SafeOpt and database/system tuners
- **SafeOpt** (Sui et al., ICML 2015) [V, original]
  (https://proceedings.mlr.press/v37/sui15.html).
- **StageOpt**: Sui, Zhuang, Burdick, Yue, "Stagewise safe Bayesian
  optimization with Gaussian processes", ICML 2018 (oral), arXiv 1806.07555
  [V] (https://arxiv.org/abs/1806.07555v2): separates safe-region expansion
  from utility maximization; tested on spinal cord stimulation therapy.
- Berkenkamp, Krause, Schoellig, "Bayesian optimization with safety
  constraints: safe and automatic parameter tuning in robotics", **Machine
  Learning vol. 112 (2023), online 2021**, DOI 10.1007/s10994-021-06019-1,
  arXiv 1602.04450 (2016) [V, **[CORRECTED]** year] (https://arxiv.org/abs/1602.04450);
  quadrotor experiments.
- Berkenkamp, Turchetta, Schoellig, Krause, "Safe model-based reinforcement
  learning with stability guarantees", NeurIPS 2017 [V]
  (https://proceedings.neurips.cc/paper/2017/hash/766ebcd59621e305170616ba3d3dac32-Abstract.html):
  Lyapunov stability certificates with statistical dynamics models.
- **Systems tuners.** OtterTune: Van Aken, Pavlo, Gordon, Zhang, SIGMOD
  2017, pp. 1009-1024, DOI 10.1145/3035918.3064029 [V]
  (https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/OtterTuneSIGMOD17.pdf):
  within 94% of expert-DBA configurations in under 60 minutes; the open-source
  repo is archived. Note OtterTune is **not** a safety-guaranteed tuner [V,
  consistent with the survey]. CherryPick (NSDI 2017), OnlineTune (SIGMOD
  2022) [UNVERIFIED: search returned only OtterTune].
- **Guarantees / Failure modes / Mech vs policy / Fit.** As original. (The
  SafeOpt guarantee statement was confirmed in the original session.)

### 4.5 Self-modifying systems with proofs
- Schmidhuber's Goedel machine: arXiv cs/0309048, "Goedel Machines:
  Self-Referential Universal Problem Solvers Making Provably Optimal
  Self-Improvements" (TR IDSIA-19-03; versions through 2006; the 2007 book
  chapter is the later print) [V] (https://arxiv.org/abs/cs/0309048v3).
  **[CORRECTED]** the survey's "2003/2007" is fine as dates; I could not
  confirm a different subtitle some sources use.
- Yudkowsky & Herreshoff, "Tiling agents for self-modifying AI, and the
  Löbian obstacle", MIRI early draft, Oct 2013 (unreviewed) [V]
  (https://intelligence.org/2013/06/06/new-research-page-and-two-new-articles/).
  Their claim is that the obstacle can be avoided by technical means while
  the underlying puzzles are only partly addressed [V]. The survey's
  statement "a system cannot in general prove its own proof system sound" is
  the standard Löb/Gödel consequence, [UNVERIFIED] as a quote from that
  draft.
- **Darwin Gödel Machine** (Zhang, Hu, Lu, Lange, Clune; UBC, Vector,
  Sakana AI), arXiv 2505.22954; v3 is an ICLR 2026 paper [V]
  (https://arxiv.org/abs/2505.22954). It replaces provable improvement with
  empirical benchmark validation, keeps an archive of coding agents, and
  reports SWE-bench 20.0% to 50.0%. Resolves the original [?]. Safety
  guarantee beyond sandboxing: none claimed [consistent with the survey;
  the authors' own safety discussion was not read].
- **[CORRECTED]** Proof-carrying code: Necula, "Proof-carrying code", POPL
  1997, pp. 106-119, DOI 10.1145/263699.263712. **Sole author** (Lee is
  associated via later joint work, e.g. "Safe, untrusted agents using
  proof-carrying code", 1998) [V]. Key idea: ship a proof that execution
  satisfies the receiver's safety policy; a small checker validates it.
- **Practical reading** unchanged.

---------------------------------------------------------------------------

## 5. Cross-cutting comparison (what each gives, what it costs)

(Table unchanged from the original; cost and "weeks/days" entries are the
survey's judgements, not sourced. Add one row for the verification-pass
finding on trace validation.)

| Technique | Strength of claim | Over what | Cost | Gap to reality |
|---|---|---|---|---|
| Machine-checked proof (seL4, IronFleet, Verdi) | proof | code or model, all configs satisfying hypotheses | person-years; IronFleet slides ~5:1 safety, ~8:1 liveness proof:code | idealized fault/hardware model |
| TLA+/P model check (bounded) | exhaustive for small N | spec | weeks | spec vs code |
| Inductive invariant (Apalache, Ivy) | all N | spec | days to weeks, needs invariant | spec vs code |
| Cutoff + small model | all N if cutoff known | class of protocols | variable | cutoff applicability |
| Alloy | bounded exhaustive | structures (grant graphs) | days | small-scope assumption |
| PBT / model-based tests | statistical | code vs model | days | generators |
| Trace-checking vs spec | per-trace conformance | implementation traces | needs spec near code granularity (MongoDB Server: impractical; CCF/others: found discrepancies) | spec abstraction level |
| DST (FDB, VOPR, Antithesis) | statistical, replayable | whole stack in simulation | high upfront | sim vs real |
| Jepsen/Elle on staging | statistical | deployed system | medium | nemesis coverage |
| Runtime monitors | detect/prevent (safety only) | live executions | low | only monitored property |
| Config analyzers (Cedar/Zelkova/Batfish) | decision procedure | a config in a restricted language | language design | interpreter correctness |
| Canary/rings | statistical | real traffic | low | rare states |
| Simplex/shields | safety under assumptions | runtime, any proposer | design of region/spec | model of environment |
| SafeOpt | prob. safety | smooth measurable objective | low | assumptions |

What none of these cover: a spec that is wrong or incomplete about what the
owner wants; confidentiality leakage through channels not modeled;
long-lived human-process failures (key loss, owner offboarding).

---------------------------------------------------------------------------

## 6. Mapping to Q9 (mechanism vs policy; verifying all configurations)

(Carried over from the original survey; it is a reading of the Choir design
docs and was not re-verified here. Change vs original: S1-S7 are candidates,
and the Hydra finding in §1.1 (a parametrized policy may legitimately stay in
the kernel) is a reason to treat each knob as one of three classes rather
than binary kernel/policy.)

Observations from the fabric docs:
- The fabric already names its authority points (§9.5): the tape (state), the
  lease row at the home (who writes), site records (what a site offers),
  grants (what a capability or controller may do). The first two look like
  kernel; the last two like policy data the kernel consults.
- The writer lease has no expiry (proposal 3) and failover is an automatic
  host branch that runs only standing intents. That removes the clock from
  the safety argument, but moves liveness to owner acts and branch machinery.
  The verification target is branch-and-epoch safety plus handoff, a small
  state machine.
- Placement and admission (Q4) are policy that must be unable to violate
  kernel invariants.

Candidate properties (to be argued, not decided):
- **Kernel safety invariants (never configurable):**
  S1 at most one accepted writer per (computer, branch, epoch); epochs
  strictly increase per branch;
  S2 every event in a branch extends the branch head under hash and is
  authored under the epoch current at commit time (fenced at the endpoint);
  S3 a branch's contents are never written by any other computer (Q6);
  S4 an irreversible effect is issued at most once per precommitment id and
  only from home-durable precommitments unless a recorded owner policy class
  says otherwise (Q5);
  S5 no work is admitted on a site unless a grant chain signed by the site's
  principal, allowed by org policy, covers it (default deny);
  S6 attenuation: a delegation never confers more than its parent;
  S7 every capability use is recorded on the tape before or atomically with
  its effect.
- **Policy dimensions (configurable inside the guard):** sites and trust
  classes, placement scoring, quotas, ring membership, rate limits,
  preferred home, failover automation level, retention.
- **Liveness/performance (measured, not proved):** time to restore a writer
  after loss; time for a phone to reach a computer; burst latency.

Tractability: S1-S7 involve few node roles and few operations; a protocol
model of tens to a few hundred lines of TLA+/PlusCal, not an OS. (The
survey's size estimate; the AWS DynamoDB spec of 939 lines [V, §2.1] is one
order-of-magnitude anchor for a replication/membership protocol.)

---------------------------------------------------------------------------

## 7. Mapping to Q10 (self-tuning inside proven bounds)

(Unchanged in structure; evidence updates inserted.)

1. **Safety is not tunable.** Kernel invariants hold for every configuration;
   the seL4-style quantified theorem [V, §2.4].
2. **The tuner is untrusted and arbitrary.** Emits proposals only. Published
   analogue for LLM agents: AgentSpec [V, §4.3].
3. **A small deterministic checker disposes of each proposal** against
   (a) typed schema, (b) kernel guard predicates, (c) an owner-signed outer
   envelope with a ratchet (tighten freely, widen needs an owner act), and
   (d) rate-of-change budget. Verdict on the tape.
4. **A proven-safe baseline and automatic revert (Simplex)**, rings and canary
   as in Configerator [V] and Gandalf-style rollout correlation [V].
5. **Only measurable, reversible, smooth dimensions are searched** (SafeOpt
   style) [V].

Open risk the literature flags: the tuner can harm liveness, cost, privacy
and trust without violating any safety invariant. Needs envelopes and
monitors (Simplex margins, not kernel proofs). Evidence that "safe on paper"
config pipelines still fail: Google Cloud 2025 (policy data replicated
globally, no feature-flag guard on the new path, per the incident report's
remediation list) and Cloudflare 2019 [V, §3.1].

Self-development of the kernel itself falls under the Goedel-machine caveat;
the empirical successor DGM relies on benchmarks, not proofs [V, §4.5].
Keep kernel changes on a heavier route than policy changes.

---------------------------------------------------------------------------

## 8. Candidate processes (options to weigh, not decisions)

Each is independent; they compose. Rough order is cost-ascending, not a
recommendation. Edits from the verification pass are marked.

**P1. Kernel boundary census.** Apply Liedtke's test, Saltzer-Schroeder's
checklist and Lampson's "is this a hint and what is the truth?" to every rule
in the fabric doc. Three classes: kernel invariant, constrained parameter
(domain predicate, Flexible-Paxos style; Hydra's retained parametrized policy
is a precedent), free policy. Output: typing of every knob and a one-page
invariant list.

**P2. A single small TLA+ (or PlusCal, or P) spec of the writer protocol,
with the policy as a havoc oracle.** As original. **Amended:** if the plan
includes later trace validation (P4), write the spec at the granularity of
loggable tape events (MongoDB found a highly abstract spec impractical to
trace-check).

**P3. Grants and admission as an analyzable language.** As original
(Cedar/Zelkova/Batfish pattern; Cedar's SMT equivalence between policy sets
[V] is the direct precedent for "does P2 permit anything P1 forbids").

**P4. Close the spec-implementation gap by trace validation.** As original.
**Amended:** evidence is mixed: CCF-style/TLC trace validation found
discrepancies in every program tried [V]; MongoDB Server trace-checking was
impractical while test-case generation worked for Realm Sync [V].
Consider test-case generation from the spec as the fallback. P's PObserve
(log checking against a P spec) is a production-grade precedent [V, AWS
source].

**P5. Deterministic simulation harness in Go for the fabric.** As original.
**Amended:** Go's `testing/synctest` (GA in Go 1.25) gives fake-time bubbles
for concurrent code [V], a smaller step than a full FDB-style simulator;
Antithesis is an external alternative [V].

**P6. Property-based and model-based tests at the edges.** As original.

**P7. Tape-derived runtime monitors.** As original (safety only [V via
Schneider/Ligatti, secondary]).

**P8. Configuration-change pipeline with kernel reconfiguration as a
first-class, tape-ordered operation.** As original. The Raft single-server
membership bug [V] is a concrete case for including reconfiguration in the
model.

**P9. Self-tuning envelope (Simplex plus ratchet plus SafeOpt where it
applies).** As original.

**P10. Black-box adversarial drills on the deployed fabric (Jepsen/chaos
style) with a history checker.** As original (Elle [V]).

**P11. A small "assurance case" per kernel version.** As original.

**Sequencing observation (not a decision).** Original order P1 -> P2 (small)
-> P6 -> P5 -> P4 -> P7 -> P3 -> P8/P9 -> P10 stands as the survey's
observation; the evidence for it is AWS's published experience [V, §2.1 and
the ShardStore paper], not an independent study.

---------------------------------------------------------------------------

## 9. Open questions for the owner or a panel

Unchanged from the original (fault model; linearization point of "accepted
write"; continuous vs discrete knobs; policy as data vs code; revocation
timing; who verifies the verifier; liveness commitments). One addition from
verification: **how abstract may the spec be?** The MongoDB result suggests
the abstraction level of the TLA+/P spec decides whether trace validation is
even possible.

---------------------------------------------------------------------------

## 10. Worked sketches (illustrations, not designs)

Unchanged from the original: §10.1 shape of a safe configuration predicate
(handoff valid iff new epoch > old epoch, old holder fenced at the commit
point, new holder admitted by a grant chain); §10.2 policy as havoc in a
model (`Init: config \in AllConfigs`, `PolicyAction: config' \in
Admissible(config, state)`, `Inv: S1 /\ ... /\ S7`); §10.3 Simplex for
placement (complex controller = placement optimizer; decision module = S5/S6
guard + owner envelope; safe controller = stay put / place on home server /
owner-approved list). These are illustrations of the survey's design
reading, not claims about sources.

---------------------------------------------------------------------------

## 11. Caveats on this verification

- Verification used search summaries of primary pages; PDFs were not read
  end to end. Items tagged [V] "via secondary" should be checked against the
  primary before being quoted in a decision document: Hydra 1975 conclusions,
  Kleppmann post, Schneider TISSEC, IronFleet ratios, Cosmos spec contents.
- ~38 items remain [UNVERIFIED]; most are classic papers (Lamport book,
  Alloy, Exokernel, QuickCheck, Gray/Oppenheimer, Basiri, LDFI, Leucker,
  Kang FODA, Classen, Kuhn, Apt-Kozen, Emerson-Namjoshi, German-Sistla,
  Vertical Paxos, CUE, Nix, Calinescu, Filieri, Hellerstein, Bak, F3269,
  Ames, Altman, García, Bloem shields, Margrave, CT/Trillian, Autopilot,
  CherryPick, OnlineTune, Verus, Woos Raft, Taube, Boyton, seL4 sizes,
  Wilson Strange Loop, Dropbox Nucleus, turmoil/madsim, Confluent TLA+, ConfErr
  year). Not searched or no result; they are stable, well-known references
  but unconfirmed here.
- Effort figures (person-years, proof ratios, weeks to learn TLA+) remain
  literature-reported orders of magnitude, not estimates for this team.
- No code or deployment of Choir was inspected; statements about the Choir
  docs are carried over from the original survey.

---------------------------------------------------------------------------

## 12. Primary sources (with URLs where confirmed)

- Hydra CACM 1974: https://home.cs.colorado.edu/~rhan/CSCI_5573_Fall_2011/Papers/wulf74.pdf ; Policy/mechanism separation in Hydra (SOSP 1975): summary at https://swift.sites.cs.wisc.edu/classes/cs736-sp07/blog/2007/01/policymechanism_separation_in.html
- Liedtke 1995: https://cgi.cse.unsw.edu.au/~cs9242/22/papers/Liedtke_95.pdf
- Saltzer & Schroeder: https://web.mit.edu/saltzer/www/publications/protection/ ; End-to-end: https://web.mit.edu/saltzer/www/publications/endtoend/endtoend.txt ; Lampson: https://bwlampson.site/33-Hints/Abstract.html
- Newcombe et al.: https://www.amazon.science/publications/how-amazon-web-services-uses-formal-methods ; Lamport excerpt: https://lamport.azurewebsites.net/tla/amazon-excerpt.html
- Apalache: https://hal.archives-ouvertes.fr/hal-02280888
- eXtreme Modelling in Practice: https://arxiv.org/abs/2006.00915 ; Trace validation: https://arxiv.org/abs/2404.16075
- P (PLDI 2013): https://www.microsoft.com/en-us/research/publication/p-safe-asynchronous-event-driven-programming-2/ ; AWS correctness practices: https://cacm.acm.org/practice/systems-correctness-practices-at-amazon-web-services
- ShardStore: https://www.cs.utexas.edu/~bornholt/papers/shardstore-sosp21.pdf ; Cosmos TLA+: https://github.com/Azure/azure-cosmos-tla
- IronFleet: https://www.microsoft.com/en-us/research/publication/ironfleet-proving-practical-distributed-systems-correct/ ; Ivy: https://ilyasergey.net/CS6213/_static/papers/ivy.pdf ; seL4 integrity: https://link.springer.com/doi/10.1007/978-3-642-22863-6_24
- Anvil: https://www.usenix.org/conference/osdi24/presentation/sun-xudong ; Sieve: https://www.usenix.net/conference/osdi22/presentation/sun ; Acto: https://cs.cornell.edu/~legunsen/pubs/GuETAlActoSOSP23.pdf
- Flexible Paxos: https://arxiv.org/abs/1608.06696v1 ; Relaxed Paxos: https://arxiv.org/pdf/2203.03058 ; Raft membership bug: https://groups.google.com/g/raft-dev/c/t4xj6dJTP6E
- Abal et al.: https://www.itu.dk/~brabrand/42-bugs.pdf ; Bloem et al. book: https://finkbeiner.groups.cispa.de/publications/2015-decidability-of-parameterized-verification/
- Mysteries of Dropbox: https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf
- FoundationDB testing: https://apple.github.io/foundationdb/testing.html ; TigerBeetle VOPR: https://docs.tigerbeetle.com/about/vopr ; Antithesis: https://antithesis.com/company/about/ ; Go synctest: https://go.dev/blog/16years
- Elle: https://arxiv.org/abs/2003.10554 ; Ligatti et al.: https://www.cs.princeton.edu/techreports/2003/681.pdf , https://www.cs.princeton.edu/~dpw/papers/nonsafety05.pdf
- Yin et al.: https://www.sigops.org/s/conferences/sosp/2011/current/2011-Cascais/12-yin-online.pdf ; PCheck: https://www.usenix.org/conference/osdi16/technical-sessions/presentation/xu
- Outages: https://status.cloud.google.com/incidents/ow5i3PPK96RduMcb1SsW ; https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019 ; https://engineering.fb.com/2021/10/05/networking-traffic/outage-details/
- Configerator: https://web.eecs.utk.edu/~qcao1/cs560/papers/holistic.pdf ; Xu & Zhou survey DOI 10.1145/2791577 ; Gandalf: https://www.usenix.org/conference/nsdi20/presentation/li
- Cedar: https://arxiv.org/abs/2403.04651v2 , https://arxiv.org/pdf/2407.01688 ; Zelkova: https://cs.utexas.edu/~hunt/FMCAD/FMCAD18/papers/paper3.pdf ; Batfish: https://ratul.org/papers/nsdi2015-batfish.pdf ; Minesweeper: https://ratul.org/papers/sigcomm2017-minesweeper.pdf ; Dhall: https://github.com/dhall-lang/dhall-lang/blob/master/README.md ; OPA: https://www.infoq.com/news/2021/02/opa-cncf-graduation/
- Neural Simplex: https://arxiv.org/abs/1908.00528 ; Simplex SEI report: https://www.sei.cmu.edu/documents/1211/1999_005_001_16772.pdf ; ActivFORMS: https://arxiv.org/pdf/1908.11179
- Shielding: https://ojs.aaai.org/index.php/AAAI/article/view/11797 ; Fulton & Platzer: https://ojs.aaai.org/index.php/AAAI/article/view/12107 ; CPO: https://proceedings.mlr.press/v70/achiam17a.html ; AgentSpec: https://arxiv.org/abs/2503.18666
- SafeOpt: https://proceedings.mlr.press/v37/sui15.html ; StageOpt: https://arxiv.org/abs/1806.07555v2 ; Berkenkamp et al.: https://arxiv.org/abs/1602.04450 ; Safe model-based RL: https://proceedings.neurips.cc/paper/2017/hash/766ebcd59621e305170616ba3d3dac32-Abstract.html ; OtterTune: https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/OtterTuneSIGMOD17.pdf
- Goedel machine: https://arxiv.org/abs/cs/0309048v3 ; Tiling agents: https://intelligence.org/2013/06/06/new-research-page-and-two-new-articles/ ; DGM: https://arxiv.org/abs/2505.22954 ; Shuttle/Loom/Stateright: https://gitee.com/mirrors_awslabs/shuttle (mirror), https://docs.rs/crate/stateright/latest
