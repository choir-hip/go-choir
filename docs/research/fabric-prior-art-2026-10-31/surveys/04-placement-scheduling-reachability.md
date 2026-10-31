# Survey 04 (verified pass): placement, scheduling, admission and reachability (2026-10-31)

## Verification log

Tags used in this file: **[V url]** confirmed against a fetched primary or official page; **[V-prior]** confirmed in the earlier session, not re-fetched; **[CORRECTED]** the original claim was wrong or imprecise, fix given; **[UNVERIFIED]** I could not reach a readable primary source, with what I tried. Untagged background text is unchanged from the original, and its factual claims are [UNVERIFIED] unless stated.

Method limits (be aware when citing):
- The PDFs for Omega, Sparrow, DRF, Firmament, YARN and BOINC (2019) came back as undecodable FlateDecode binary, and no `pdftotext` was available. WebFetch's answers for those PDFs only echoed my prompt, so they are NOT evidence. For those papers I verified authors, venue and abstract-level claims via publisher or abstract pages and search snippets. Internals such as Paxos, token flow and conflict rates stay [UNVERIFIED].
- usenix.org, dl.acm.org and microsoft.com/research returned 403 for several pages. I used search-engine abstract snippets instead and name them as such.
- I did not open the internal design docs (§0 is repeated from the original and is [UNVERIFIED-internal]).

Counts (approximate, by cited system or load-bearing claim): about 38 confirmed, 13 corrected, about 35 left unverified (mostly paper-internal details), 5 prior-art entries added (all verified).

Most consequential corrections:
1. **Nomad:** `max_client_disconnect` is described in the original as the current mechanism. The current docs have a `disconnect` block (`lost_after`, `replace`, `reconcile`, `stop_on_client_after`). Without `lost_after`, a missed heartbeat marks allocations lost and replaces them. `stop_on_client_after` is a client-side self-kill that the survey missed, and it is the closest deployed analogue to a lease that expires on the holder. The `reconcile` options are `keep_original`, `keep_replacement`, `best_score` and `longest_running` (A6).
2. **Mobile IPv4 binding lifetime:** the original contrasts Mobile IP's expiring binding with Choir's no-expiry lease. RFC 5944's Lifetime field allows 0xffff = infinity, so no-expiry bindings are legal in Mobile IPv4, with deregistration by explicit message (B6).
3. **Firecracker snapshots:** the original says clones must be "re-keyed" and that snapshots are non-portable "without templates". The docs say resuming one snapshot more than once is unsafe unless uniqueness is handled, and the VM should be resumed on identical hardware and software. CPU templates are not the portability answer (A13).
4. **Kubernetes default 300 s:** `tolerationSeconds: 300` applies only to the `not-ready` and `unreachable` NoExecute taints, injected by the DefaultTolerationSeconds admission plugin. It is not a generic eviction timer. StatefulSet at-most-one is confirmed, and its release conditions are different from "pod GC" (A3).
5. **Borg citation:** "Verdi et al." should be **Verma** et al. (A1).
6. **Resource Central** (SOSP 2017, Cortez et al.) is a workload-prediction system used for oversubscription. It is not the "harvest VMs" paper, which is a different OSDI 2020 paper I could not verify (A17).
7. **Sparrow:** the headline number is "within 12% of an ideal scheduler" on TPC-H on a 110-machine cluster, with median queueing delay under 9 ms. The "near-optimal at 80–90% load" phrasing is not in the abstract (A7).
8. **Cedar:** the verified formal-methods claim is Lean modeling plus a sound and complete SMT encoding. I could not confirm the "Dafny" claim (A18).
9. **Sprite eviction** moves foreign processes back to their home node. The original's "or kill" is not confirmed (A12).
10. **Cloudflare Tunnel replicas** do not offer steering. Each request goes to the nearest replica, retries others on failure, and there is no guarantee which replica is chosen (B4).

---

Scope: prior art for placing work on heterogeneous, intermittently available, differently owned machines (brief Q4), reaching a computer wherever it runs (Q2), and the mechanism/policy split (Q9). No final choice is recommended; §C maps findings to the questions and §D lists the open design axes.

## 0. What the fabric design asks for [UNVERIFIED-internal: not re-read this pass]

From `resource-fabric-2026-10-31.md` §0, §1.2, §1.3, §3, §7, §10 and `branching-tape` §1:

- Sites publish a signed `site_record` (capacity, free_now, availability, local models, capabilities, trust_class). Home holds it. Placement attributes in order: trust (hard filter), locality (hard filter), availability, capacity, cost.
- Placement is a management-desk decision; each site enforces its own admission, and a refusal is an answer, not an error.
- Site access control: personal workstation admits only its principal's work by default; cross-person placement needs a grant signed by the site's principal and allowed by org policy; grants are signed, revocable by either signer, and admissions/refusals go on the placed computer's tape.
- Writer lease has no expiry (fenced, owner-moved); leaf leases do expire. Restart rule: crash closes obligations, never re-dispatches.
- Reachability: a home per computer; the workstation holds one long-lived outbound session to the home; the home never dials the workstation; read-only follower copy when the writer is gone.
- Units: writer realization, capsule, outboard capsule (leaf), native capability lease, child computer.

Observation: the design has the structure of **filter (hard) then score (soft)** with **both sides having veto**, the shape of HTCondor matchmaking and of Kubernetes filter/score plus admission.

---

## A. Placement, scheduling, admission

### A1. Borg [CORRECTED citation; abstract V; internals UNVERIFIED]

Verified (https://research.google/pubs/large-scale-cluster-management-at-google-with-borg/): Verma, Pedrosa, Korupolu, Oppenheimer, Tune, Wilkes, EuroSys 2015, Bordeaux. Abstract: runs hundreds of thousands of jobs on clusters of up to tens of thousands of machines; high utilization via **admission control, efficient task packing, over-commitment, machine sharing with process-level isolation**; features to shorten fault recovery and policies to reduce correlated failures; declarative job spec; simulation tools. [V]

Remaining details are [UNVERIFIED] (paper PDF not decodable): Paxos-replicated Borgmaster, Borglets, the separate scheduler process, feasibility check then scoring, four priority bands (monitoring, production, batch, best-effort), per-priority quota as admission, reclamation of slack to non-prod, preemption cascades, OOM kills. Treat them as widely reported but unchecked this pass.

- **Mechanism vs policy.** Mechanism: feasibility filter, scoring slot, priority/preemption, quota check. Policy: weights, bands, quota amounts, reclamation aggressiveness. "Quota admits, scheduler places" is the clearest admission/placement split in the corpus. (The abstract's "admission control" and "over-commitment" support this reading.)
- **Relevance.** Template for "owner limits always win" (non-prod = leaf, evictable) and for admission-at-submission distinct from placement. Assumes single ownership and always-on machines.

### A2. Omega [V authors/venue/abstract; internals UNVERIFIED]

Verified (https://research.google/pubs/omega-flexible-scalable-schedulers-for-large-compute-clusters): Schwarzkopf, Konwinski, Abd-El-Malek, Wilkes, EuroSys 2013, Prague, pp. 351–364. The abstract argues monolithic schedulers hinder evolution and proposes **parallelism, shared state, and lock-free optimistic concurrency control**; it measures interference between schedulers; evaluation uses real Google workloads. [V] A conference live blog (https://www.syslog.cl.cam.ac.uk/2013/04/17/live-blog-from-eurosys-2013-day-3) reports the shared-state model matched a complex monolithic scheduler but that conflicts added overhead at scale. [V-secondary]

- **Mechanism.** Shared cell state; each scheduler has a copy, commits transactions; conflicts retried. The "only one conflicting transaction succeeds, the others resync and retry" description is confirmed by lecture slides [V-secondary, https://www.cl.cam.ac.uk/teaching/2122/CC/lectures/mesos-omega.pdf].
- **[UNVERIFIED]:** gang scheduling via atomic multi-resource transactions; "low conflict for service, higher for batch" (the live blog says conflicts matter at scale; the exact numbers are unchecked); the need for a strongly consistent store; the critique of Mesos pessimistic offers as hiding global state (this is the paper's framing as I recall it).
- **Mechanism vs policy / relevance.** Unchanged from the original: safety (no double allocation) in the commit protocol; any scheduler policy is safe by construction; maps onto lease row as compare-and-set authority with many placers, with site admission as final validator.

### A3. Kubernetes scheduler and admission [partly V; one CORRECTED]

- Taints: `NoSchedule` blocks new pods without a toleration and does not evict running pods; `NoExecute` also evicts non-tolerating pods, tolerating pods without `tolerationSeconds` stay indefinitely, with `tolerationSeconds` they are evicted after that period. [V https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/]
- **[CORRECTED]** The default 300 s is the `DefaultTolerationSeconds` admission plugin adding `not-ready`/`unreachable` NoExecute tolerations. I could not read that section of the page (the fetch truncated), so the figure comes from my own knowledge of the plugin. [UNVERIFIED-numeric; mechanism plausible.] It is not a general eviction timer.
- Admission: controllers run in the kube-apiserver after authn/authz and before persistence, on create/delete/modify (not reads); mutating phase then validating phase; any rejection fails the request; extension points are Mutating/ValidatingAdmissionWebhook and the declarative MutatingAdmissionPolicy / ValidatingAdmissionPolicy (no HTTP callouts). [V https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/] (The original omitted MutatingAdmissionPolicy; CEL as the language was not visible in the fetched portion: [UNVERIFIED].) The plugin list (ResourceQuota, LimitRanger, PodSecurity) was truncated: [UNVERIFIED on this page].
- StatefulSet: "at most one Pod with a given identity runs in the cluster at any time"; a pod on an unreachable node is not auto-deleted; it goes Terminating/Unknown and leaves only if (1) the Node object is deleted, (2) the kubelet responds and removes it, or (3) it is force-deleted; force deletion can create a replacement while the original runs (split brain). [V https://kubernetes.io/docs/tasks/run-application/force-delete-stateful-set-pod/] **[CORRECTED]** the original's "pod GC after node confirmed dead" is better stated as "Node object deleted, by an admin or the Node Controller".
- [UNVERIFIED]: scheduling-framework extension-point names beyond what I recall (PreFilter/Filter/PostFilter/Score/Reserve/Permit/Bind), Binding write via etcd, kubelet-side admission and pressure eviction, "isolation depends on admission policy, not the taint".
- **Relevance (unchanged).** Taint = site-side default-deny; toleration is self-asserted by the pod in vanilla Kubernetes, so a signed site grant is something Kubernetes does not provide natively.

### A4. Mesos two-level scheduling [V abstract level]

Verified: Hindman et al., NSDI 2011. Abstract (via https://people.csail.mit.edu/matei/papers/2011/nsdi_mesos.pdf search snippet and the NSDI liveblog https://www.syslog.cl.cam.ac.uk/2011/04/01/nsdi-2011-liveblog-day-3/): a distributed two-level scheduling mechanism called **resource offers**; master decides how many resources to offer each framework, the framework decides which to accept and which tasks to run; **filters** (predicates frameworks send to the master) reduce rejected offers; near-optimal data locality, 50,000 emulated nodes, failure resilience. [V-secondary]
- DRF as the allocation policy implemented in Mesos is stated in the DRF paper's evaluation summary. [V-secondary]
- [UNVERIFIED]: ZooKeeper master HA, optimistic offers, quota/reservations/revocable resources as later features, head-of-line blocking claims.
- **Relevance.** Offers invert control: the site offers and the work accepts. Unchanged.

### A5. DRF [V properties]

Ghodsi, Zaharia, Hindman, Konwinski, Shenker, Stoica, NSDI 2011 (tech report UCB/EECS-2011-18, https://www2.eecs.berkeley.edu/Pubs/TechRpts/2011/EECS-2011-18.html). Generalizes max-min fairness to multiple resource types; satisfies sharing incentive, strategy-proofness, envy-freeness (abstract) and Pareto efficiency (body); implemented in Mesos with better throughput and fairness than slot-based fair sharing. [V-secondary via search snippets]
- [UNVERIFIED]: the Leontief/divisible-task assumptions as stated; effect of intermittent capacity (this is the survey's own inference).

### A6. Nomad [CORRECTED, V https://developer.hashicorp.com/nomad/docs/job-specification/disconnect]

- **[CORRECTED]** The `disconnect` block controls client loss. Default: a node that misses heartbeats is marked `lost` and its allocations replaced.
  - `lost_after` (default 0s): if > 0, the client is "disconnected" and allocations "unknown" (not lost); they may keep running and reconnect without restart.
  - `replace` (default true): place a replacement for an unknown allocation immediately (counts as a reschedule).
  - `reconcile` (default `best_score`): on reconnect keep `keep_original`, `keep_replacement`, `best_score` or `longest_running`.
  - `stop_on_client_after`: the client stops its own allocations after that long without server contact (incompatible with `lost_after` in one block); the server replaces after `heartbeat_grace`, which is a server-agent setting (examples cite a 10 s default).
  - The original's `max_client_disconnect` is not mentioned on the current page; treat it as the older name (relationship to the new block: [UNVERIFIED]).
- [UNVERIFIED]: Raft servers, Omega-like eval/plan/apply with stale-plan rejection, Sentinel, ACL namespaces.
- **Relevance.** `stop_on_client_after` is a deployed model for a site that self-fences when disconnected, the same shape as a leaf lease that expires on the holder. `replace=true` with `lost_after` > 0 explicitly allows original and replacement to both run until `reconcile`. Choir's writer choice (no expiry; fence first) is the other end.

### A7. Sparrow [V abstract; CORRECTED figure]

Ousterhout, Wendell, Zaharia, Stoica, SOSP 2013 (search snippets of https://amplab.cs.berkeley.edu/wp-content/uploads/2013/11/p69-ousterhout.pdf). Stateless decentralized scheduler with **batch sampling** and **late binding**; deployed on a 110-machine cluster, **within 12% of an ideal scheduler on TPC-H, median queueing delay < 9 ms**. [V-secondary] **[CORRECTED]** replace "near-optimal at 80–90% load" with this figure; a blog quoting the paper gives late binding at 0.55× the response time of batch sampling at 80% load, within 5% of omniscient, in simulation [UNVERIFIED, blog]. An earlier draft says 14% on 100 nodes.
- [UNVERIFIED]: probe ratio d=2 as the default, constraint handling, behaviour with sleeping workers (survey inference).

### A8. Quincy and Firmament [V abstracts]

- Quincy: Isard, Prabhakaran, Currey, Wieder, Talwar, Goldberg, SOSP 2009 (22nd SOSP). Scheduling as a graph problem with edge weights and capacities balancing locality, fairness, starvation freedom; standard solver gives an optimal online schedule under a global cost model; a few-hundred-computer cluster; up to 3.9× less cross-cluster data transfer and up to 40% higher throughput. [V https://www.microsoft.com/en-us/research/publication/quincy-fair-scheduling-for-distributed-computing-clusters/]
- Firmament: Gog et al., OSDI 2016. Centralized, min-cost max-flow, continuously reschedules all tasks, sub-second placement at 10,000+ machines using multiple MCMF algorithms, incremental solving and problem-specific optimizations; 12,500-machine Google trace: 20× lower placement latency than Quincy; matches distributed schedulers' latency for short tasks; 6× batch response time vs four widely used schedulers. [V-secondary, search snippet of https://research.google/pubs/firmament-fast-centralized-cluster-scheduling-at-scale/]
- [UNVERIFIED]: "pluggable cost models" and the specific solver names (relaxation, cost scaling) as in the original; flow-network inability to express gang/non-linear constraints; "hard to verify".

### A9. YARN [V metadata; mechanism UNVERIFIED]

Vavilapalli et al., SoCC 2013, Santa Clara, best paper award; separates resource management from the programming model and delegates scheduling functions to per-job components; MapReduce is one app among several (Dryad, Giraph, Hoya, REEF, Spark, Storm, Tez ported). [V-secondary: https://blog.acolyer.org/2017/01/09/apache-hadoop-yarn-yet-another-resource-negotiator/ and https://opencourse.inf.ed.ac.uk/sites/default/files/2023-10/yarn-socc13.pdf search snippets]
- [UNVERIFIED]: container tokens signed by the RM and validated by the NM (the original's key relevance claim; the PDF fetch only echoed my prompt), hierarchical capacity queues, locality-relaxed requests. Check the paper's security section before relying on the token claim.

### A10. HTCondor: ClassAds, matchmaking, desktop owner policy [V-prior for policy; papers V]

- Litzkow, Livny, Mutka, "Condor: a hunter of idle workstations", 8th ICDCS, June 1988, pp. 104–111; when the owner returns the remote job is checkpointed and moved. [V https://minds.wisc.edu/items/c30eb149-ee84-425e-9139-e34759a11f6d and search records]
- Raman, Livny, Solomon, "Matchmaking: Distributed Resource Management for High Throughput Computing", HPDC-7, Chicago, 28–31 July 1998, pp. 140–146; providers and consumers advertise classads, a matchmaker matches them satisfying both sides' constraints. [V https://www.chtc.cs.wisc.edu/doc/hpdc98.pdf via search record]
- The gangmatching paper states matchmaking is limited by its purely bilateral formalism (one customer to one resource), precluding co-allocation. [V-secondary https://www.chtc.cs.wisc.edu/doc/gangmatching.pdf] **Added nuance:** the "bilateral" shape the survey leans on has a known limitation for multi-resource work (e.g., a computer needing a site plus a capability plus storage).
- Owner-policy details (START/SUSPEND/…, machine states, MATCH_TIMEOUT 120 s, Desktop template `IS_OWNER`) are [V-prior] from https://htcondor.readthedocs.io/en/latest/admin-manual/ep-policy-configuration.html and were not re-fetched.
- [UNVERIFIED]: standard-universe checkpoint/remote syscall details, flocking, glideins, claim re-evaluation by startd, ads as soft state, security (IDTOKENS etc.).
- Mechanism vs policy and relevance: unchanged from the original.

### A11. BOINC and volunteer computing [V metadata; mechanism UNVERIFIED]

Anderson, "BOINC: A System for Public-Resource Computing and Storage", Fifth IEEE/ACM Intl. Workshop on Grid Computing, 2004, DOI 10.1109/GRID.2004.14. [V search record, https://homeostasis.scs.carleton.ca/~soma/distos/fall2008/anderson-boinc.pdf] A later overview (arXiv 1903.01699, Anderson 2019) says the system addresses volunteer-device heterogeneity, unreliability and churn. [V-secondary]
- **[UNVERIFIED]** replication/quorum validation, adaptive replication, homogeneous redundancy, credit, code signing, volunteer-controlled preferences, pull model: the PDFs did not decode and the search excerpt of the 2004 paper said only that deliberate falsification would be handled outside the system. Do not attribute quorum validation to the 2004 paper without checking; the mechanism is in later BOINC papers and docs. SETI@home CACM 2002: [UNVERIFIED].

### A12. Cycle stealing and process migration: Sprite, MOSIX, Condor checkpointing

- **Sprite** [V]: Douglis & Ousterhout, SPE 1991 (https://web.stanford.edu/~ouster/cgi-bin/papers/migration-spe.pdf). Transparency is defined relative to the **home machine**; a migrated process keeps its PID and appears on the home machine; the paper trades off transparency, residual dependencies, performance and complexity; high transparency means residual dependencies on home. Eviction when a host is reclaimed moves the foreign process back to its home node, and it may migrate again elsewhere. Used in regular service (pmake) by 20+ users per the 1990 thesis. Ousterhout's retrospective says migration was complicated and hard to maintain. [V-secondary via search snippets] **[CORRECTED]** "migrate them back or kill": only "migrate back to home" is confirmed. [UNVERIFIED]: idle-host selection by keyboard/mouse and central migration server details; kernel-level security.
- **MOSIX** [UNVERIFIED]: deputy at home node, probabilistic load exchange, economic migration model.
- **Condor checkpointing** [UNVERIFIED beyond the 1988 abstract].
- **Relevance (unchanged).** The home-node-as-authority-for-location-dependent-state pattern, its home-failure weakness, and owner-return eviction as a first-class event.

### A13. VM live migration, checkpoint/restore, snapshots

- **Clark et al., NSDI 2005** [V https://static.usenix.org/event/nsdi05/tech/full_papers/clark/clark_html]: iterative pre-copy while the VM runs, then pause, copy remaining pages, resume at destination; built on Xen; **60 ms downtime for a Quake 3 server, 210 ms for SPECweb**; dynamic rate limiting; frequently dirtied pages not pre-copied. The original's "tens to hundreds of ms" is consistent. [UNVERIFIED]: Nelson et al. VMotion (ATC 2005), Hines post-copy (VEE 2009) and its failure property, convergence-failure behavior, arm64-to-x86 impossibility (obvious but not sourced).
- **CRIU** [UNVERIFIED this pass; https://criu.org not fetched].
- **Firecracker snapshots** [V https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/docs/snapshotting/snapshot-support.md]: resuming one snapshot more than once is "insecure" unless uniqueness is handled; the safe pattern is to snapshot, terminate the original, resume one copy; VMGenID (Linux guests 5.18+ ACPI, 6.10+ DeviceTree) lets the guest reseed the kernel PRNG, but application-level IDs, cached randomness and tokens remain duplicated; VMClock `vm_generation_counter` for clock notification (needs newer kernel); network connectivity after resume is not guaranteed; memory loaded lazily either via a `MAP_PRIVATE` file mapping (file must stay unmodified for the VM's lifetime) or via a UFFD user-space handler; **snapshots should be resumed on identical hardware and software**, with only a limited, unsupported exception; arm64 GICv2/GICv3 snapshots not interchangeable; full snapshots are large and need quotas. **[CORRECTED]** the original's "not portable across CPU models without templates" overstates what templates give; the page says identical hardware and software.
- **Relevance (unchanged, with the correction above).** Moving the writer by tape replay avoids hypervisor/arch coupling; the snapshot-uniqueness hazard maps to the fork re-keying rule.

### A14. Placement of stateful objects: Durable Objects, Fly.io, Orleans

- **Cloudflare Durable Objects [V-prior]** (https://developers.cloudflare.com/durable-objects/reference/data-location/): unchanged from the original. The "at most one instance" and output-gates claims remain [UNVERIFIED] (design posts not fetched).
- **Fly.io `fly-replay`** [V https://docs.fly.io/networking/dynamic-request-routing]: targets `region` (ordered list, aliases), `instance` (a Machine ID; `prefer_instance` falls back), and `app` (same org/private network by default; cross-org needs allowlisting). Limits: request body > 1 MB cannot be replayed; replayed requests cannot themselves replay; replay caching is an optimization, not a guarantee. **Added:** the cross-org allowlist is a deployed example of consent-by-target for routing.
- **LiteFS** [V partial https://docs.fly.io/litefs]: Fly states it is "outside our scope of support", pre-1.0, "APIs may change and features could be removed"; **do not use with Fly Machine autostop/autostart because the proxy can stop and restart Machines without knowing which holds the lease or the latest data, so a Machine with stale data could acquire the lease, causing rollback and permanent data loss.** That is a documented partition/lease hazard directly on point. The Consul lease election and TTL claims in the original are [UNVERIFIED] (not on the fetched page).
- **Orleans** [V https://learn.microsoft.com/en-us/dotnet/orleans/grains/grain-placement]: built-in strategies are resource-optimized (default since Orleans 9.2; previously random), random, prefer-local, hash-based, activation-count-based (power of two choices), stateless-worker and silo-role-based; custom strategies via `PlacementStrategy` plus `IPlacementDirector`; **placement filtering** (Orleans 9+) removes ineligible silos before the strategy picks (hard filter, then scoring: the same shape as Choir's order). Activation repartitioning and rebalancing are experimental. **[CORRECTED/added]** default-strategy change and filtering. Single activation: the directory rejects a second registration for a single-activation grain; in failure-free times one activation, under failure only eventually; default directory is eventually consistent and allows occasional duplicates while the cluster is unstable; a pluggable directory (e.g., Redis) can give stronger guarantees. [V-secondary: Microsoft docs via search snippet https://learn.microsoft.com/sr-latn-rs/dotnet/orleans/host/grain-directory; paper MSR-TR-2014-41 via snippets.] [UNVERIFIED]: ETag-based optimistic concurrency in storage; SoCC 2011 "Orleans: Cloud Computing for Everyone" details; author list of the TR.

### A15. Serverless cold start and snapshots [UNVERIFIED except Firecracker snapshot caveats in A13]

Agache et al. NSDI 2020, ~125 ms boot, SnapStart hooks, REAP (ASPLOS 2021), Catalyzer, FaaSnap, vHive, Lambda placement internals, snapshot-sharing side channels: all [UNVERIFIED]. Image-locality scoring (K8s `ImageLocality`) is [UNVERIFIED] by name, plausible.

### A16. Mobile and energy-aware offload

- **MAUI** [V-secondary]: Cuervo, Balasubramanian et al., MobiSys 2010, pp. 49–62; fine-grained, energy-aware offload decided at run time by an optimization engine using measured CPU and network (bandwidth, latency); a **linear programming** formulation; managed-code properties (portability, serialization, reflection, type safety) enable it; results: face recognition used an order of magnitude less energy, an arcade game doubled its refresh rate, a voice-translation app ran components unsupported on the phone. (https://www.microsoft.com/en-us/research/?p=159073 search record.) **[CORRECTED]** the original says "0–1 ILP over the call graph"; "integer" was not confirmed, so say "linear programming formulation" unless the paper is checked. "[R, verify numbers]" is now resolved for the face-recognition claim (order of magnitude).
- CloneCloud (EuroSys 2011), ThinkAir (INFOCOM 2012): [UNVERIFIED]. The "modern consensus that fine-grained offload rarely pays" is the survey's judgment [UNVERIFIED].

### A17. Oversubscription

- Resource Central: Cortez, Bonde, Muzio, Russinovich, Fontoura, Bianchini, SOSP 2017, pp. 153–167: collects VM telemetry, learns behavior offline, serves predictions online; a modified Azure VM scheduler uses predictions to oversubscribe servers safely. [V-secondary https://www.microsoft.com/en-us/research/publication/resource-central-understanding-predicting-workloads-improved-resource-management-large-cloud-platforms/] **[CORRECTED]** the original lists Resource Central as "harvest VMs". It is not. Harvest VMs (OSDI 2020) is a separate paper; I found no source for it: [UNVERIFIED].
- "Take it to the Limit: Peak Prediction-driven Resource Overcommit in Datacenters", EuroSys 2021 (Crossref DOI 10.1145/3447786.3456259 is in the EuroSys 2021 proceedings): defines a "peak oracle" baseline and a max-predictor combining several algorithms that beats static limit-based overcommit on risk and efficiency. [V-secondary, summary at https://medium.com/@weidagang/d892839deca8; authors not retrieved]
- Autopilot: Rzadca, Findeisen, Swiderski, Zych, Broniek, Kusmierek, Nowak, Strack, Witusowski, Hand, Wilkes, EuroSys 2020. [V-secondary AMiner listing] "Borg: the Next Generation" (Tirmazi et al., EuroSys 2020): [UNVERIFIED].
- Kubernetes QoS classes, Mesos revocable resources, ballooning/KSM: [UNVERIFIED]. The repo's CPU-oversubscription note: [UNVERIFIED-internal].

### A18. Other admission and policy mechanisms

- **Cedar** [V]: Cutler et al., "Cedar: A New Language for Expressive, Fast, Safe, and Analyzable Authorization", PACMPL 8(OOPSLA1), 2024 (Crossref 10.1145/3649835; arXiv 2403.04651, extended 2407.01688). Designed to be ergonomic, fast, safe and analyzable; modeled in Lean with proofs about the language; the design admits a sound and complete logical encoding for policy analysis (e.g., checking a refactor preserves permissions); implemented in Rust, open-sourced; compared with OpenFGA and Rego (about equal or better readability, much faster). **[CORRECTED]** "Lean/Dafny" and "SMT" are not both confirmed: Lean and a logical encoding are; Dafny and "AWS 2023 announcement" are [UNVERIFIED]. (https://arxiv.org/abs/2403.04651v2 and https://www.amazon.science/publications/cedar-a-new-language-for-expressive-fast-safe-and-analyzable-authorization)
- Capability/ticket-checked admission (YARN tokens, Condor claim IDs/IDTOKENS, SPIFFE/SPIRE attestation), OPA/Gatekeeper/Kyverno, Globus GSI / VOMS / glidein-WMS / "site autonomy": [UNVERIFIED]. The original's "Nix/Slurm federation" heading mixes unrelated things; Slurm federation and Nix are not discussed in the text and are [UNVERIFIED] as relevant.
- **Added, Kueue [V https://kueue.sigs.k8s.io/docs/overview/]:** Kubernetes-native job queueing that decides when a job waits, is admitted against quota, or is preempted; StrictFIFO and BestEffortFIFO queueing; can admit on an alternate resource flavor when one is exhausted; **MultiKueue dispatches jobs across multiple clusters** to find capacity. A current example of admission-against-quota decoupled from the node scheduler.
- **Added, Virtual Kubelet [V https://virtual-kubelet.io/]:** a CNCF sandbox kubelet implementation that lets Kubernetes nodes be backed by arbitrary providers (e.g., serverless container platforms); a pattern for presenting a foreign execution site as a "node".

---

## B. Reachability

### B1. NAT traversal: STUN, TURN, ICE
- Behavioral claims about STUN, hole punching, keepalive (~30 s), birthday-paradox port spraying numbers, UPnP/NAT-PMP/PCP, >90% direct estimate, relay fallback: [V-prior] from https://tailscale.com/blog/how-nat-traversal-works.
- RFCs [V]: RFC 8489 (STUN, Feb 2020, obsoletes RFC 5389; abstract notes STUN is not a NAT traversal solution by itself); RFC 8656 (TURN, obsoletes RFC 5766 and 6156; a client reaches multiple peers through one relay address; designed to work within ICE); RFC 8445 (ICE, obsoletes RFC 5245); RFC 8838 (Trickle ICE, Jan 2021). (https://www.rfc-editor.org/rfc/rfc8489.html, /rfc8656.html, /rfc8445.html, /rfc8838.html.) RFC 4787: [UNVERIFIED]. TURN over TCP/TLS 443 and credential details: [UNVERIFIED].
- Mechanism vs policy and relevance: unchanged.

### B2. Tailscale: coordination server, WireGuard, DERP
- **DERP** [V https://tailscale.com/kb/1232/derp-servers]: each client gets a DERP map from the coordination server and picks a **home DERP by latency**; the coordination server shares each client's choice with other clients; clients fall back to the next closest region if one becomes unreachable; relayed traffic is WireGuard-encrypted and DERP cannot decrypt it; private keys never leave the device. Relay by destination public key and DERP over HTTPS: [V-prior] (NAT blog); the KB page does not state them.
- **Tailnet Lock** [V https://tailscale.com/kb/1226/tailnet-lock]: every new node needs a signature from a trusted Tailnet Lock key; peers check signatures, so a compromised coordination server cannot get an unsigned node accepted; at least two, up to 20, signing nodes; disabling needs a disablement secret. The original's "[R]" is now confirmed.
- WireGuard NDSS 2017 (Donenfeld), ACL enforcement at nodes, Headscale: [UNVERIFIED].

### B3. WireGuard meshes (Nebula, Netmaker, ZeroTier, Headscale)
- **Nebula relays** [V https://nebula.defined.net/docs/config/relay/]: relay support introduced in v1.6.0; config keys `relays`, `am_relay` (default false), `use_relays` (default true); relays cannot chain; relay nodes need a public IP and inbound UDP; any host can be a relay without being a lighthouse; lighthouses report a host's relays to peers during handshakes. Certificate groups, CA-signed certs, and the **fingerprint blocklist** were not on that page: [UNVERIFIED] (check https://nebula.defined.net/docs/ for the PKI/blocklist docs).
- ZeroTier planets/roots, Netmaker, NetBird: [UNVERIFIED].

### B4. Reverse tunnels: ngrok, Cloudflare Tunnel, frp, SSH -R, inlets
- **Cloudflare Tunnel** [V https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/ and .../configure-tunnels/tunnel-availability/]: `cloudflared` makes **outbound-only** connections; each instance opens **four** connections to four different servers across **at least two data centers**; each replica adds four more; replicas point at the same tunnel and others keep serving if one host dies; **replicas do not support traffic steering; Cloudflare forwards to the geographically closest replica and retries others on failure, with no guarantee which is chosen.** **[CORRECTED]** the original's "load-balance without coordination" implied balancing; the docs say closest-with-failover. QUIC/HTTP2 transport and Access/TLS-termination claims: [UNVERIFIED].
- ngrok, frp, inlets: [UNVERIFIED].
- Relevance (unchanged): the writer must be a singleton, which tunnel replicas explicitly are not.

### B5. libp2p: AutoNAT, circuit relay v2, DCUtR [V-prior]
Unchanged from the original; specs: https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md and .../DCUtR.md. DHT eclipse/Sybil attacks: [UNVERIFIED].

### B6. Mobile IP: home agent routing [V RFCs; CORRECTED binding-lifetime contrast]
- RFC 5944 (Mobile IPv4 revised, Nov 2010, obsoletes RFC 3344) [V https://www.rfc-editor.org/rfc/rfc5944.html]: home agent intercepts datagrams and tunnels to the care-of address; registration Lifetime field in seconds, **0 = deregister, 0xffff = infinity**; Mobility Security Association between nodes; Identification field for replay protection; T bit requests reverse tunneling.
- RFC 6275 (Mobile IPv6, July 2011, obsoletes RFC 3775) [V https://www.rfc-editor.org/rfc/rfc6275.html]: **return routability** (Home/Care-of Test Init and Test, Kbm) authorizes correspondent Binding Updates and enables direct routing; bindings established this way are capped (MAX_RR_BINDING_LIFETIME); mobile node to home agent Binding Updates and Acks are protected by an IPsec SA (typically ESP transport mode with non-NULL authentication), while correspondent updates use return routability instead of IPsec.
- **[CORRECTED]** The original contrasts "binding expires" (Mobile IP) with "no lifetime" (Choir). In Mobile IPv4 a registration can be requested with infinite lifetime (0xffff); the bounded-lifetime case in v6 is the correspondent-node binding, not the home registration. So no-expiry bindings exist in the standards; what is bounded is route-optimization state.
- [UNVERIFIED]: home agent redundancy RFCs (I did not find them in the portion of RFC 5944 I read), VRRP-style HA, deployment history claims.

### B7. Other reachability prior art
- mDNS: RFC 6762 (Cheshire and Krochmal, Apple, Feb 2013) provides local-link DNS-style lookups with minimal configuration [V https://www.rfc-editor.org/rfc/rfc6762.html]. Sleep-proxy behavior is not defined in the portion I read; Apple's Bonjour Sleep Proxy, "Wake for network access", Power Nap, and APNs/WNS/FCM wake behavior: [UNVERIFIED]. **Note:** the original's sentence conflates APNs push-to-wake with Bonjour Sleep Proxy; they are different mechanisms.
- Tor onion services [V roles https://spec.torproject.org/rend-spec/overview.html]: introduction points accept connection requests and pass them to the service; rendezvous points are where client and service meet; client fetches the service's descriptor, builds a rendezvous circuit, then introduces via an intro point; v3 adds offline keys and restricted discovery. Signed-descriptor format: [UNVERIFIED] on the fetched pages (the spec describes it; not read). "Keybase/IPFS" as comparable: [UNVERIFIED].
- Matrix/Signal/WhatsApp offline delivery, MX failover, Consul sessions as used by LiteFS: [UNVERIFIED].

---

## C. Mapping to the brief's questions (no choice recommended)

Unchanged structure from the original; evidence strength noted per bullet.

### C1. Q4 placement and admission
1. **Eligibility (hard filter / admission)**, 2. **Preference (soft score)**, 3. **Binding/commit**, 4. **Revocation.** Supported by: Borg abstract (admission control vs packing) [V], Orleans placement filtering then strategy [V], Kueue admission vs node scheduling [V], Condor bilateral matching [V], K8s admission phases [V].

Observations (evidence notes added):
- Default deny at the node is taint semantics [V]; the signed grant is not in K8s core [UNVERIFIED that no extension does it], and YARN-token and grid-attribute precedents are [UNVERIFIED].
- Both-sides veto = ClassAd bilateral requirements [V]; "claim re-checked by startd" [V-prior/UNVERIFIED]. Gangmatching shows the bilateral form is limited for co-allocation [V-secondary].
- Offer/pull when sites are intermittent: Mesos offers [V], Nomad client-initiated heartbeats [V], BOINC pull [UNVERIFIED], Sparrow late binding [V].
- Intermittent availability stances: kill-fast (Kubernetes taints; Nomad default lost/replace [V]), tolerate-and-reconcile (Nomad `lost_after`+`reconcile` [V]), self-fence on the client (Nomad `stop_on_client_after` [V]), never-reschedule-without-fencing (K8s StatefulSet [V]). BOINC deadlines [UNVERIFIED].
- Authority-free vs authority-bearing work: Borg prod/non-prod [V at abstract level only], BOINC replication [UNVERIFIED].
- Fairness (DRF) [V]; receipts/credit [UNVERIFIED]; image locality [UNVERIFIED]; policy-as-restricted-expression (ClassAd [V], ValidatingAdmissionPolicy [V], Cedar [V], OPA [UNVERIFIED]).

### C2. Q2 reachability
Patterns (a) outbound-connected stable rendezvous/relay (DERP home [V], Cloudflare Tunnel [V], TURN [V], libp2p [V-prior], Mobile IP HA [V]); (b) direct upgrade after relay (ICE [V], DCUtR [V-prior], MIPv6 route optimization [V]); (c) store-and-forward (email, push: [UNVERIFIED]).
- Home failover/multiple homes: Tailscale per-node home DERP chosen by latency with fallback [V]; Cloudflare replicas, nearest-with-retry, no steering [V]; Tor intro points [V]; MX and MIP HA redundancy [UNVERIFIED]. Tailscale and Tor separate the rendezvous hint from the authority of a signed record; for Tailscale the signed-record role is Tailnet Lock for node keys [V].
- Route staleness: MIPv4 allows infinite registration lifetime [V], so the no-expiry choice has a standards precedent (revised from the original).
- Sleeping workstations: [UNVERIFIED] mechanisms; "accept absence" remains an option.
- Enterprise firewalls: TCP/443 fallback [V-prior for DERP]; cloudflared transport [UNVERIFIED].
- Relay trust: DERP relays cannot decrypt [V]; Choir's home as plaintext proxy is a design fact, not prior art.

### C3. Q9 mechanism versus policy
- Omega: safety in the commit protocol regardless of scheduler [V abstract: optimistic concurrency, parallel schedulers].
- Condor: policy as expressions at the owner's daemon [V-prior].
- Kubernetes: mechanism pipeline plus plugin policy; "isolation not kernel-guaranteed" and incident claims [UNVERIFIED].
- Cedar: policies analyzable by a logical encoding, language modeled in Lean [V].
- Firmament: policy confined to a cost model on a flow graph [V at abstract level; "cost model" term UNVERIFIED].
- Orleans: placement directors; single activation not strictly guaranteed under instability [V-secondary]; ETag-CAS storage claim [UNVERIFIED]. (**Correction to framing:** the original said safety lives in storage CAS regardless; the docs say the default directory permits duplicate activations when unstable, and a different directory is needed for stronger guarantees.)
- Verification practices (Kleppmann fencing critique, Borg Fauxmaster, kube-scheduler-simulator, kwok, FoundationDB simulation, TigerBeetle VOPR, Jepsen, TLA+ for leases): all [UNVERIFIED]. A search for Fauxmaster returned nothing relevant.
- Self-tuning within clamps: Autopilot exists [V-secondary]; its min/max clamps, VPA/HPA bounds: [UNVERIFIED].

---

## D. Open design axes surfaced (questions, not answers)

Unchanged from the original, with two additions.

1. Push vs pull vs offer for leaf placement on intermittent sites.
2. Where the site's veto is evaluated: match time and again at claim time; signed grant carried by the work vs site-local lookup.
3. Bilateral expressions vs requester-presented capability; revocation latency when a site is offline (Nebula blocklist; Tailnet Lock).
4. Whether the home's route/lease authority and relay role are one component or separated.
5. Disconnect semantics per work class: fence-and-wait; reschedule after timeout; reconcile on return; **site self-fence after N seconds without home contact (Nomad `stop_on_client_after`)** (added).
6. Contribution evidence for credits.
7. Global optimization vs greedy filter/score; verifiability.
8. Policy language and kernel clamps for self-tuning.
9. Sleeping/battery workstations: wake path vs absence.
10. Image/snapshot locality; arch/hypervisor compatibility as hard filters; moves are tape-based. Firecracker's identical-hardware requirement supports treating host type as a hard filter.
11. **(added)** Whether a multi-resource request (site + capability + storage) needs co-allocation beyond bilateral matching (gangmatching), or can be decomposed.
12. **(added)** Whether duplicate activations are prevented by the lease mechanism (Choir writer) or tolerated and reconciled (Orleans default directory, Nomad `replace`+`reconcile`), per work class.

---

## E. Primary sources

Verified this pass or earlier [V]:
- Borg abstract: https://research.google/pubs/large-scale-cluster-management-at-google-with-borg/
- Omega: https://research.google/pubs/omega-flexible-scalable-schedulers-for-large-compute-clusters
- Quincy: https://www.microsoft.com/en-us/research/publication/quincy-fair-scheduling-for-distributed-computing-clusters/
- Kubernetes taints: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ ; admission: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/ ; StatefulSet: https://kubernetes.io/docs/tasks/run-application/force-delete-stateful-set-pod/
- Nomad disconnect: https://developer.hashicorp.com/nomad/docs/job-specification/disconnect
- Firecracker snapshots: https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/docs/snapshotting/snapshot-support.md
- Orleans placement: https://learn.microsoft.com/en-us/dotnet/orleans/grains/grain-placement
- Fly: https://docs.fly.io/networking/dynamic-request-routing ; https://docs.fly.io/litefs
- Cloudflare Tunnel: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/ ; .../configure-tunnels/tunnel-availability/
- Tailscale: https://tailscale.com/blog/how-nat-traversal-works ; https://tailscale.com/kb/1232/derp-servers ; https://tailscale.com/kb/1226/tailnet-lock
- Nebula relay: https://nebula.defined.net/docs/config/relay/
- libp2p: https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md ; .../DCUtR.md
- HTCondor EP policy: https://htcondor.readthedocs.io/en/latest/admin-manual/ep-policy-configuration.html
- Cloudflare DO: https://developers.cloudflare.com/durable-objects/reference/data-location/
- RFCs: 8489, 8656, 8445, 8838, 5944, 6275, 6762 at https://www.rfc-editor.org/rfc/rfcNNNN.html
- Kueue: https://kueue.sigs.k8s.io/docs/overview/ ; Virtual Kubelet: https://virtual-kubelet.io/
- Cedar: https://arxiv.org/abs/2403.04651v2
- Tor rend spec overview: https://spec.torproject.org/rend-spec/overview.html
- Sprite: https://web.stanford.edu/~ouster/cgi-bin/papers/migration-spe.pdf (located, snippet-level)
- Live migration: https://static.usenix.org/event/nsdi05/tech/full_papers/clark/clark_html

Verified at abstract/snippet level only [V-secondary]: Mesos (https://people.csail.mit.edu/matei/papers/2011/nsdi_mesos.pdf), DRF (https://www2.eecs.berkeley.edu/Pubs/TechRpts/2011/EECS-2011-18.html), Sparrow (https://amplab.cs.berkeley.edu/wp-content/uploads/2013/11/p69-ousterhout.pdf), Firmament (https://research.google/pubs/firmament-fast-centralized-cluster-scheduling-at-scale/), YARN (https://blog.acolyer.org/2017/01/09/apache-hadoop-yarn-yet-another-resource-negotiator/), Condor 1988 and HPDC 1998 (https://www.chtc.cs.wisc.edu/doc/hpdc98.pdf), BOINC 2004 metadata, MAUI, Resource Central, Take it to the Limit, Autopilot.

Still [UNVERIFIED] (cited in the original; no readable primary source reached): MOSIX; CRIU; VMotion; post-copy; Firecracker NSDI 2020 and SnapStart; REAP/Catalyzer/FaaSnap; CloneCloud; ThinkAir; Harvest VMs OSDI 2020; Borg: the Next Generation; Kleppmann 2016; WireGuard NDSS 2017; ZeroTier/Netmaker/NetBird; ngrok/frp/inlets; RFC 4787; MIP home-agent redundancy RFCs; Apple Sleep Proxy/Power Nap/APNs; Fauxmaster; kube-scheduler-simulator/kwok; FoundationDB/TigerBeetle/Jepsen; OPA/Kyverno; GSI/VOMS/glidein-WMS; SPIFFE/SPIRE; BOINC quorum validation and the SETI@home CACM paper; YARN container-token mechanism.

Candidate additions I did not verify (not included as claims): KubeEdge / OpenYurt for disconnected edge nodes, Slurm federation, Ray scheduling, Apache Aurora, Kubernetes Descheduler, Globus/GSI delegation.

Internal: `docs/design/resource-fabric-2026-10-31.md` and `branching-tape-2026-10-31.md` were not re-read in this pass.
