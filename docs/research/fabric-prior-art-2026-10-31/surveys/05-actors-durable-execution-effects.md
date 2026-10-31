# Survey 05 (verified): computation that controls computation, durable execution, external side effects

Date 2026-10-31. Read-only research for the resource-fabric design. Maps to Q5 (irreversible effects), Q6 (one computer controlling another without writing its state), Q10 (self-tuning inside proven bounds). Does not recommend a choice.

## Verification log

Tag legend (replaces the original [M]/[K]/[R]/[U]/[?] legend):
- [V] = primary source (official docs, spec, paper page) opened and read this session; URL given.
- [V-s] = confirmed from search-result snippets of primary or near-primary pages, page not opened. Weaker than [V].
- [CORRECTED] = original claim was wrong or imprecise; fix and URL given.
- [UNVERIFIED] = could not confirm; what was tried is stated. Classic citations marked "not attempted" were not looked up at all; their bibliographic details are from memory.
- [ADDED] = prior art the original survey missed, verified as indicated.

Counts (by claim/system item, approximate): about 45 confirmed [V], about 20 confirmed [V-s], 11 corrected, about 40 left [UNVERIFIED] (most of them classic papers not attempted for lack of time), 6 added.

Most consequential corrections:
1. IETF Idempotency-Key draft is now -08 (22 Oct 2026, expires 25 Apr 2027), not -07 of Oct 2025; it also specifies 409 for a concurrent retry.
2. Step Functions: Express workflows do not support `.waitForTaskToken` (callback) or `.sync`; Standard history is retrievable 90 days after completion (the one-year figure is the maximum execution duration); the docs say Standard tasks "never run more than once unless Retry is specified", which is stronger than the survey's "at-least-once on retry unless idempotent". Callback tokens work only within one AWS account.
3. Temporal: the docs pages do not use the phrase "at-least-once" and do not state a workflow-id+activity-id key. The guarantee is derived from the default unlimited retry policy; with max attempts 1 an activity is at-most-once. Shard `RangeID` as a fencing generation number is confirmed, but the enforcement is not described in the docs read.
4. Restate: the docs I could open state "exactly-once" for invocations and do not define `ctx.run` delivery semantics. The survey's "at-least-once for ctx.run" is plausible but UNVERIFIED from the primary docs.
5. Akka: the shard coordinator's state is held in Distributed Data (persistence mode is deprecated); SBR lease-majority is currently backed by a Kubernetes CRD; stable-after default is 20 s.
6. Ray: actor `max_restarts` defaults to 0, and actor tasks are at-most-once by default (retry via `max_task_retries` gives at-least-once); GCS HA via external Redis is officially supported only with KubeRay.
7. Azure Durable Functions: recommended backend is now the Durable Task Scheduler; Netherite support ends 31 Mar 2028. The survey's "activities at-least-once" was not stated on the pages read.
8. seL4: the SOSP 2009 paper proves functional correctness. The survey's attribution of integrity/confidentiality proofs to Klein et al. 2009 is not supported by what I read (those are separate later results, not fetched).
9. Cedar is confirmed (OOPSLA 2024, Lean formalization, Rust implementation) and the [?] on its venue resolved.
10. Sha's Simplex paper could not be confirmed by search.

Not re-checked: §0 (what the Choir docs fix) was not compared with the repo this session; it is as in the original.

## 0. What the Choir docs already fix (the constraints any prior art is read against)

[UNVERIFIED against the repo this session; carried over from the original.]

From resource-fabric-2026-10-31.md (§0, §2, §4, §9, §10) and branching-tape-2026-10-31.md (§1, §6):

- One writer per computer branch, always a host. The lease has an epoch, no expiry (so no timer-driven self-fencing). A VM proposes appends; the host admits them with a compare-and-swap on the head and signs receipts.
- Two durability levels: local-durable (workstation committed) and home-durable (home acknowledged). "Irreversible external effects fire only from a home-durable precommitment" (§2.3, proposal 10.5), unless owner policy allows a class offline. Offline, the effect waits in an outbox as a draft with a send intent: "state, not an obligation in flight".
- Restart rules (CLAUDE.md): a crash closes every open obligation as "interrupted by a restart" and never re-dispatches; standing intents (schedules) survive, their occurrences do not; a planned restart with a one-shot marker may resume.
- Conflict is a branch, never a merge. The losing branch's external effects "stay sent"; the new primary gets a branch notice, and an effects digest is shown. "External state that moved on the losing side (rotated refresh token, publisher counter) is the unsafe residue" (branching tape §6, §12).
- Choir in Choir (§4): a controller grant (verbs, scope, budget, expiry/revocable by the controlled computer's owner), addressed publication into the controlled computer's Texture, results returned by publication and adoption; lifecycle verbs go to the home/platform, not to the guest; no cycles; quotas; subtree kill switch. A grant cannot be wider than its granter.
- An actuator writes a precommit, act, resolve triple on the tape (§5.2, §5.4).

Terminology hazard worth flagging: in `docs/precommitment-records.md` a "precommitment record" is a prospective-self-alignment object (a registered prediction/expectation scored later). In the fabric doc it is used in the write-ahead intent-log sense (record intent durably before the act). The prior art below is about the second sense; the two senses may be one mechanism or two, which is a design question, not a research finding.

## 1. The actor model and its descendants

### 1.1 Actor model (Hewitt, Bishop, Steiger 1973; Agha 1986; Clinger 1981)
- Problem: a model of concurrent computation with no shared state, suited to distributed systems.
- Mechanism: an actor has private state, a mailbox, and on each message may (a) send messages to addresses it knows, (b) create new actors, (c) designate its behavior for the next message. Addresses are unforgeable references acquired by creation, receipt in a message, or being the creator. Message delivery is asynchronous, guaranteed-eventually in Agha's formulation, with no ordering guarantee between different senders.
- Guarantees: no data races by construction; locality of state; the only way to affect an actor is to send it a message it may choose to ignore.
- Failure modes: the model itself is silent on failure and on durability; "guaranteed delivery" is an abstraction that real systems implement as at-most-once or at-least-once. Address knowledge is the authority model (relevant to Q6).
- Mechanism vs policy: the mechanism is create / send / become; everything else (supervision, persistence, placement) is layered.
- Sources: [UNVERIFIED, not attempted] Hewitt, Bishop, Steiger, IJCAI 1973; Agha, MIT Press 1986; Clinger, MIT AI-TR 633, 1981. A modern actor system with the same create/send/become shape and explicit vats/turns is verified in §14.4 (Goblins).

### 1.2 Erlang/OTP: links, monitors, supervision trees, "let it crash"
- Problem: telephony switches that must run for years with software faults present (Armstrong). [UNVERIFIED, thesis not attempted]
- Mechanism: processes share nothing and communicate by message. A *link* is bidirectional (only one link between two processes) and propagates exit signals; a *monitor* is unidirectional and delivers `{'DOWN', Ref, process, Pid, Reason}`; monitoring a nonexistent process delivers DOWN at once with reason `noproc`. [V] https://www.erlang.org/doc/system/ref_man_processes.html
- A *supervisor* applies a restart strategy to children: one_for_one, one_for_all, rest_for_one, simple_one_for_one. Restart *intensity* and *period* default to 1 and 5 (the survey's "3 in 5 s" is an example value, not the default). Exceeding the bound makes the supervisor terminate all children and itself with reason `shutdown`, and the next supervisor up restarts it or terminates in turn. Child restart types: permanent (always), transient (only on abnormal exit), temporary (never). [V] https://www.erlang.org/doc/system/sup_princ.html
- [ADDED detail, V] `auto_shutdown` (never / any_significant / all_significant) with `significant` children: a supervisor can be configured to terminate itself when specific children end. Same page.
- A restart re-runs the child's init from a clean state; it does not resume the in-memory state of the dead process. [UNVERIFIED as a quoted statement; consistent with the supervisor page's restart semantics]
- Guarantees: failure is contained to a subtree; the restart policy is declarative data; escalation is bounded.
- Failure modes: a crash-restart loop (bounded by intensity). Supervision does not make external effects safe: a child that crashed after sending but before recording will redo the send [UNVERIFIED, reasoning not a source claim]. For links across nodes, `noconnection` is the exit reason when "the connection between the nodes was lost" and the other process may still be alive. [V] ref_man_processes. So a partition is indistinguishable from a crash: the same split-brain hazard as Q1.
- Mechanism vs policy: link/monitor/exit-signal is the kernel; supervisor strategy, intensity and child specs are policy data.
- Resemblance to Choir's rules: "crash restart never resumes work" = OTP restart-to-clean-state; the schedule rule ("closed if its tick crashes the computer twice") = restart intensity plus escalation; standing intent = permanent child spec, occurrence = transient state. Difference: OTP has no durable tape recording the interrupted fate.
- Sources: [V] the two Erlang doc pages above; [UNVERIFIED, not attempted] Armstrong, KTH thesis 2003.

### 1.3 Akka: cluster sharding, remember-entities, singleton, split-brain resolver
- Problem: location-transparent actors across a cluster that must have at most one live instance of each entity.
- Mechanism: *cluster sharding* maps entity ids to shards and shards to nodes via a shard coordinator. [CORRECTED] The coordinator's state is stored in the "State Store"; default is Distributed Data ("replicated across the cluster but not stored to disk"); Persistence mode is deprecated. The page I read did not call the coordinator a cluster singleton (the survey said so); that is UNVERIFIED. *remember-entities* is optional, off by default, restarts entities after rebalance/crash and disables automatic passivation. [V] https://doc.akka.io/libraries/akka/current/typed/cluster-sharding.html (first 100k of 130k chars read)
- *Cluster singleton*: "Always running on the oldest member with specified role." Warns not to use a downing strategy that may split the cluster into several clusters (each would start its own singleton); "more than one active singletons is prevented by all reasonable means"; some corner cases resolved only by configurable timeouts; a lease is recommended for extra safety. [V] https://doc.akka.io/libraries/akka/current/typed/cluster-singleton.html
- *Split Brain Resolver*: strategies static-quorum, keep-majority (default), keep-oldest, down-all, lease-majority. [CORRECTED/refined] lease-majority uses a distributed lock "currently backed by a Kubernetes CRD". `stable-after` default 20 s. Docs: "When there is uncertainty it selects to down more nodes than necessary, or even downing of all nodes." [V] https://doc.akka.io/libraries/akka/current/split-brain-resolver.html
- Guarantees: at most one entity instance only when the downing decision is correct (the singleton page says this in softer terms; see above).
- Failure modes: unreachable is not dead; a downed node that keeps running is a zombie until it observes its own downing [UNVERIFIED as stated]; SBR depends on timeouts. The survey's claim that persistent actors also need journal-level fencing because the cluster layer alone gives no mutual exclusion is [UNVERIFIED]: not found on the pages read (Akka Persistence docs not opened).
- Mechanism vs policy: downing strategy pluggable over a membership/failure-detector kernel. [V]
- Contrast with Choir's lease: SBR decides automatically by timeout and majority which side keeps writing; Choir's lease decides by owner act (no expiry) then fences, accepting unavailability. Auto failover for standing intents creates a second branch rather than a majority decision. Neither dominates; depends on what a split costs.

### 1.4 Orleans virtual actors (grains)
- Problem: make actor programming scalable without the programmer managing placement and lifecycle.
- Mechanism: a grain has a stable identity and is activated on some silo on first message and deactivated when idle. Persistence is explicit via `IPersistentState<T>` with an `Etag`; a storage provider detecting an Etag violation should fault the write with `InconsistentStateException`. [V] https://learn.microsoft.com/en-us/dotnet/orleans/grains/grain-persistence/ Activations are single-threaded, one request at a time by default (non-reentrant) [V] https://learn.microsoft.com/en-us/dotnet/orleans/grains/request-scheduling
- Reminders vs timers [V]: timers stop if the grain deactivates or the silo crashes; reminders are persistent, associated with a grain not an activation, reactivate the grain, and "each specific occurrence with its specific time isn't stored", so a tick due while the cluster is down is missed and only the next tick occurs. https://learn.microsoft.com/en-us/dotnet/orleans/grains/timers-and-reminders This is the same split as Choir's "standing intents survive, occurrences do not".
- Directory/single activation: [V-s] the directory enforces single activation by rejecting a second registration and returning the existing address; during failures or partition the guarantee is eventual and duplicates are deactivated once the convergence window closes (Orleans FAQ and TR summaries; sources found via search: sergeybykov.github.io/orleans/Documentation/frequently_asked_questions.html, shinesmile.gitbooks.io TR translation). The primary TR PDF was fetched but not machine-readable here.
- [V] Grain docs also warn that grains are not always deactivated cleanly (silo crash) so lifecycle hooks cannot be relied on. https://learn.microsoft.com/en-us/dotnet/orleans/grains/grain-lifecycle ; the Redis provider's `EntryExpiry` option "can cause duplicate activations" (grain-persistence page).
- Guarantees: the original survey's "at-most-once message delivery by default" is [UNVERIFIED]: not found in the pages read, and the TR text was unreadable. Treat as likely but unconfirmed.
- Mechanism vs policy: placement and deactivation policy pluggable. [UNVERIFIED as stated]
- Sources: [V] Bernstein, Bykov, Geller, Kliot, Thelin, "Orleans: Distributed Virtual Actors for Programmability and Scalability", MSR-TR-2014-41, March 2014, https://www.microsoft.com/en-us/research/publication/orleans-distributed-virtual-actors-for-programmability-and-scalability/ ; [UNVERIFIED, not attempted] Bykov et al., SoCC 2011.

### 1.5 Ray
- Problem: dynamic task graphs and stateful actors for ML workloads.
- Mechanism: tasks (stateless) and actors (stateful) return futures. [V] Actors: "The default value of max_restarts is 0, meaning that the actor won't be restarted"; -1 restarts without limit; `max_task_retries` applies to exceptions and actor crashes, and -1 or a positive value gives at-least-once; "By default, actor tasks execute with at-most-once semantics"; a task may complete even if `ray.get` raises `RayActorError`. https://docs.ray.io/en/latest/ray-core/fault_tolerance/actors.html
- [CORRECTED, resolves the survey's [?]] GCS fault tolerance via external HA Redis exists but is "officially supported only if you are using KubeRay"; an embedded RocksDB backend is alpha. https://docs.ray.io/en/latest/ray-core/fault_tolerance/gcs.html
- Lineage re-execution of lost task outputs and owner fate-sharing: [V-s] Ownership paper confirmed (Wang, Liang, Oakes, Hindman, Luan, Cheng, Stoica, NSDI 2021, pp. 671-686; "assigns each object a leader") https://www.usenix.org/conference/nsdi21/presentation/cheng ; the specific lineage-reconstruction and owner-death claims were not read. [UNVERIFIED] Moritz et al., OSDI 2018 (search did not return the paper).
- Relevance: lineage reconstruction is the opposite of tape replay: Ray recomputes pure work, Choir persists results because desks are not pure.

## 2. Durable execution and workflow engines

Common idea: progress is recorded as a log of non-deterministic inputs and effect results; after a crash the program is re-executed with logged results substituted. This is event sourcing for control flow; the Azure docs describe it so ("Orleans... event sourcing" for orchestrators) [V]. It makes "resume after a crash" the default, the opposite of Choir's crash rule and the same as its planned-restart rule.

### 2.1 Temporal / Cadence
- Mechanism: workflow code is deterministic; the service stores an event history; replay compares each generated Command with the history, and a mismatch yields a non-deterministic error. [V] https://docs.temporal.io/workflow-definition : causes are code changes to a running definition and intrinsic non-determinism (local time, random); non-deterministic work (API calls, LLM invocations, DB queries) belongs in Activities; `SideEffect`/`MutableSideEffect` produce Commands and must not be reordered; patching (`Patched`/`GetVersion`) and Worker Versioning exist, with **Worker Versioning "the recommended way"** over patching (the original did not say this). `workflow.Now` by name is not on that page ([UNVERIFIED]; the page says SDK time APIs store results in history).
- Activities: [V] "If an Activity attempt fails, it is automatically retried using its Retry Policy"; "We recommend that it be idempotent". https://docs.temporal.io/activities [CORRECTED] The docs page does not use the term "at-least-once" nor describe the idempotency key (workflow id + activity id). [V-s] Temporal's community thread and blog derive it: default retry policy is unlimited attempts with backoff; a worker that finishes an activity then dies before reporting causes a retry; with maximumAttempts=1 an activity is at-most-once; Temporal's framing is at-least-once delivery plus idempotency gives effectively-once. https://community.temporal.io/t/execution-guarantees-of-activities/3405 ; https://temporal.io/blog/idempotency-and-durable-execution
- Shard ownership: [V] each History Service instance owns a subset of shards via a ShardController using Ringpop membership; each shard has a `RangeID`, "a monotonically increasing generation number used for fencing". The page does not explain the enforcement check. https://github.com/temporalio/temporal/blob/main/docs/architecture/history-service.md Survey's "range ID as a fencing token in the persistence layer" is consistent but the persistence-layer enforcement is [UNVERIFIED].
- Signals, queries, child workflows with parent-close policy, continue-as-new, heartbeat timeouts: [UNVERIFIED], not opened.
- Mechanism vs policy: history + replay + task queues = mechanism; retry, timeouts, versioning = policy.
- Sources: https://docs.temporal.io/workflow-definition [V]; https://docs.temporal.io/activities [V]; Cadence (cadenceworkflow.io) [UNVERIFIED, not attempted].

### 2.2 AWS Step Functions
- [V] Standard: up to one year; "exactly-once" model, "your tasks and states are never run more than once, unless you have specified Retry behavior in ASL"; suited to non-idempotent actions such as payments. Execution history retrievable via API for up to 90 days after completion. Express: up to five minutes; asynchronous Express = at-least-once, synchronous Express = at-most-once; Express does not persist state between transitions, and does **not** support Job-run (`.sync`) or Callback (`.waitForTaskToken`). Starting a Standard execution with the name of a running one returns an idempotent response. https://docs.aws.amazon.com/step-functions/latest/dg/choosing-workflow-type.html
- [CORRECTED] original: "history up to a year" (it is retrievable 90 days post-completion; one year is max duration) and "task states still at-least-once on retry unless idempotent" (docs say never more than once unless Retry configured; the survey's caution about retried tasks stands but the docs claim is stronger).
- Callback: [V] workflow pauses until `SendTaskSuccess`/`SendTaskFailure` returns the task token; waits up to the one-year quota unless `HeartbeatSeconds` is set; tokens must come from principals in the same AWS account; on timeout a new random token is generated. https://docs.aws.amazon.com/step-functions/latest/dg/connect-to-resource.html
- Program as data (Amazon States Language) is analyzable [UNVERIFIED as a claim about analyzability; ASL being a declarative JSON definition is V on the pages].

### 2.3 Azure Durable Functions / Durable Task
- [V] Orchestrators use event sourcing; an append-only store records actions; on wake-up the orchestrator "re-executes the entire function from the start", consulting history for completed activities; must be deterministic (no wall-clock, GUIDs, random, direct I/O, env vars, static state, blocking sleeps; use `CurrentUtcDateTime`, `NewGuid`, activities, durable timers). Unhandled orchestrator exception ends in `Failed` and cannot be retried. https://learn.microsoft.com/en-us/azure/azure-functions/durable/durable-functions-orchestrations and .../durable-functions-code-constraints (canonical now under learn.microsoft.com/azure/durable-task/common/)
- [CORRECTED/updated] Storage backends: Durable Task Scheduler (recommended, Azure-managed), Azure Storage, MSSQL, Netherite (Event Hubs + FASTER; support ends 31 Mar 2028). The Azure Storage provider has no transactional guarantee between table and queue and uses eventual-consistency patterns; MSSQL gives strong consistency. https://learn.microsoft.com/en-us/azure/azure-functions/durable/durable-functions-storage-providers
- Entities [V]: key-addressed, operations on one entity run serially; signal = one-way, "The runtime guarantees delivery, but the sender can't observe result values or errors"; call = two-way. Critical sections via entity locks (.NET/JS). https://learn.microsoft.com/en-us/azure/azure-functions/durable/durable-functions-entities
- Not found on pages read: "activities at-least-once" [UNVERIFIED]; `IsReplaying` [UNVERIFIED] (docs mention replay-safe logging).
- [V-s] Burckhardt, Gillum, Justo, Kallas, McMahon, Meiklejohn, "Durable Functions: Semantics for Stateful Serverless", PACMPL 5(OOPSLA) art. 133, 2021: idealized model on untyped lambda calculus; two execution models proved equivalent to the high-level model. https://www.microsoft.com/en-us/research/publication/durable-functions-semantics-for-stateful-serverless/ The "at-least-once activity model" detail is [UNVERIFIED].
- Netherite evaluation: [V-s] arXiv 2103.00033, "Serverless Workflows with Durable Functions and Netherite".

### 2.4 Restate
- [V] Restate "tracks every step of your code execution in a journal", records operation and result, replays the journal skipping completed steps; invocations "run exactly once regardless of failures" (vendor claim; mechanism not explained on page); virtual objects have per-entity state with only one handler modifying state at a time; an idempotency key header makes Restate deduplicate requests. https://docs.restate.dev/concepts/durable_execution ; https://docs.restate.dev/foundations/key-concepts
- `ctx.run`: [V] stores a result in the execution log; failures inside are retried unless `TerminalError`; the doc page read does not state the delivery guarantee or the effect-before-journal crash window. https://docs.restate.dev/develop/ts/durable-steps [UNVERIFIED] the survey's "at-least-once for ctx.run" and "single-binary log-based runtime" claims. Likely true (the same window exists in every journaling engine, see §14.3, §4.3) but not source-confirmed.
- Mechanism vs policy: journal + per-key serialization = mechanism. [V-consistent]

### 2.5 DBOS
- [V] Workflows and steps checkpointed to Postgres; steps "should be idempotent, meaning it should be safe to retry"; "once a step completes and is checkpointed, it is never re-executed"; recovery restarts interrupted workflows with checkpointed inputs; workflow functions must be deterministic. https://docs.dbos.dev/architecture
- [V-s] Transactional steps: the step's database writes and the checkpoint commit in one Postgres transaction (DBOS blog "Postgres Transactions are a Distributed Systems Superpower", https://www.dbos.dev/blog/co-locating-workflow-state-with-your-data), giving exactly-once for database effects; ordinary steps are at-least-once; external effects need idempotency keys. Workflow-id-as-idempotency-key: dbos-transact-py README suggests using an event id as workflow id (https://github.com/DBOS-project/dbos-transact-py). The architecture page itself does not discuss these ([UNVERIFIED on that page]).
- [UNVERIFIED, not attempted] Skiadopoulos et al., VLDB 2021 "DBOS".
- Relevance (unchanged): exactly-once cheaply only when the effect shares the log's atomic commit; for Choir, maild enqueue is the candidate boundary.

### 2.6 What the engines share and where they diverge (for the fabric)
| Aspect | Temporal/Durable Fns/Restate/DBOS | Choir tape |
|---|---|---|
| Log | per-execution history | per-computer branch, hash-chained, signed receipts |
| Replay | re-execute deterministic code against history | tape replay rebuilds state; desks (LLM-driven) are not deterministic, so effects results must be journaled, never recomputed |
| After crash | resume by default | close as interrupted, do not resume (planned restart: resume once) |
| Effects | at-least-once unless idempotent / same-DB | precommit, act, resolve triple |
| Single writer | shard owner (RangeID) / per-key serialization / DB row lock | host lease epoch + CAS |
| Zombie handling | timeouts and heartbeats, storage fencing | epoch in CAS, no expiry |
| Versioning code under replay | patching, worker versioning (Temporal recommends the latter) | not yet addressed for replayed desks [open] |

Choir's "never resume after a crash" removes the largest class of engine problems at the price that every crashed obligation is lost unless re-initiated. "Planned restart may resume once" reintroduces a narrow form of the replay problem.

## 3. Distributed transactions and sagas

### 3.1 Two-phase commit and its blocking problem
- Mechanism, guarantees, failure modes as in the original (PREPARE, durable prepared record, coordinator decision as commit point, blocking if coordinator fails; presumed-abort/commit variants). [UNVERIFIED, textbook; not attempted]
- Parallel: a no-expiry lease is the same trade as 2PC (safety over liveness, human act as heuristic resolution); a TTL lease is the Chubby/Percolator trade. [reasoning]
- Sources: [UNVERIFIED, not attempted] Gray 1978; Gray and Reuter 1993; Bernstein-Hadzilacos-Goodman 1987.

### 3.2 3PC and Paxos Commit
- [UNVERIFIED, not attempted] Skeen, SIGMOD 1981; Gray and Lamport, ACM TODS 2006; Corbett et al., Spanner, OSDI 2012. Claims unchanged: 3PC non-blocking only under synchronous assumptions; Paxos Commit replicates the coordinator; Spanner = 2PC over Paxos groups.

### 3.3 Percolator
- [V-s] One cell per transaction is the primary lock and the synchronization point; the commit point is the write replacing the primary lock; a crash before it rolls back, after it rolls forward; cleanup and commit both modify the primary under a Bigtable row transaction so only one wins; readers finding a stale lock check whether the owner is dead (Chubby-based liveness per summaries) and inspect the primary. Secondary summaries only (e.g. https://www.cse.iitd.ac.in/~srsarangi/courses/2013/csl860/docs/percolator.pdf appeared in results, a copy of the paper; not opened).
- Relevance: commit point as one durable write on a single authority; in-doubt resolved by inspecting the record.
- Source: Peng and Dabek, OSDI 2010.

### 3.4 Sagas and compensation
- [V-s] Garcia-Molina and Salem, "Sagas", SIGMOD 1987, pp. 249-259 (DBLP). Helland, "Life beyond Distributed Transactions: an Apostate's Opinion", CIDR 2007, pp. 132-141 (DBLP).
- Pivot/retriable/compensatable vocabulary and TCC: [UNVERIFIED], attributed to later literature (Richardson); not opened. Richardson saga page not opened.
- Relevance (unchanged): the fabric's "effects stay sent and recorded on the losing branch plus a notice" is a compensation-free, notice-only saga.

## 4. Exactly-once, idempotency, outbox

### 4.1 Impossibility and the end-to-end argument
- Two Generals/FLP framing and Saltzer-Reed-Clark 1984: [UNVERIFIED, not attempted]. The effectively-once framing (at-least-once delivery plus idempotent processing) is [V-s] stated by Temporal ("at-least-once delivery (Temporal) + idempotency (your Activity and receiver) = effectively-once"), AWS Lambda durable functions docs (§14.3), and microservices.io (below).

### 4.2 Idempotency keys
- [V] Stripe API v1: client-generated key (up to 255 chars); saves status code and body of the first request "after endpoint execution begins" and returns it on retries, including 500s; keys may be pruned after at least 24 hours and a reused key then generates a new request; incoming parameters are compared with the original and error if different; results not saved if validation fails or the request conflicts with a concurrently executing one; all POSTs accept keys, GET/DELETE do not. [ADDED] API v2 differs: "retries don't repeat successful operations. Stripe retries failed or partially failed work to completion when possible." https://docs.stripe.com/api/idempotent_requests
- [CORRECTED] IETF draft is `draft-ietf-httpapi-idempotency-key-header-08`, 22 October 2026, active Internet-Draft in the HTTPAPI WG (Standards Track), expires 25 April 2027, no RFC yet. Servers SHOULD answer 422 for key reuse with a different payload, 400 for a missing required key, 409 for a retry while the original is in flight. https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header (original said -07, October 2025.)
- [V] Brandur: "atomic phases" (local state mutations in transactions between external calls) and "recovery points" (named checkpoints after each phase) let a retry resume; key unique across `(user_id, idempotency_key)`; `locked_at` marks a key as being worked on. https://brandur.org/idempotency-keys (The survey's wording "unique index on account and key" is consistent.) The brandur.org/fragments draft commentary was not re-opened; it is [V-s] only from the original.
- [UNVERIFIED, not attempted] Helland, "Idempotence Is Not a Medical Condition", ACM Queue 10(4), April 2012: bibliographic data [V-s] via Crossref (DOI 10.1145/2181796.2187821); contents not read.

### 4.3 Transactional outbox (and inbox)
- [V] microservices.io: the sender stores each message in the DB in the same transaction as the entity update; a separate relay forwards to the broker; the relay can publish the same message more than once (e.g. crash after publish before recording), so "a message consumer must be idempotent", often by tracking processed message ids. https://microservices.io/patterns/data/transactional-outbox.html
- CDC relays (Debezium), inbox table naming, DDIA ch. 11: [UNVERIFIED, not attempted].
- Relevance (unchanged): the outbox draft with a send intent fired once home-durable is a transactional outbox whose database is the tape and whose relay is the actuator; the relay's sent-but-unrecorded window is the in-doubt state the precommit/act/resolve triple makes visible.

### 4.4 Kafka transactions and zombie fencing
- [V] KIP-98: broker keeps sequence numbers per PID per partition to dedupe; stable `TransactionalId` spans producer sessions; "Exactly one active producer with a given TransactionalId" via epoch bump which fences an older instance; a Transaction Coordinator with a transaction log; COMMIT(PID)/ABORT(PID) control messages written to partition logs; `read_committed` consumers see only committed transactional messages. https://cwiki.apache.org/confluence/display/KAFKA/KIP-98+-+Exactly+Once+Delivery+and+Transactional+Messaging
- "Exactly-once holds inside Kafka only; external effects at-least-once": [UNVERIFIED as a KIP statement]; widely stated in Confluent material (not opened). Consume-transform-produce offset commit in the same transaction: [UNVERIFIED on page read]. Flink `TwoPhaseCommitSinkFunction`: javadoc URL returned 404; [UNVERIFIED]. Narkhede 2017 blog, KIP-129: not attempted.

### 4.5 Fencing tokens and leases
- [V-s] Kleppmann 2016: a lock service issues a monotonically increasing token; the storage layer rejects writes with a lower token than already accepted; Redlock lacks such a token, hence good for efficiency locks, not correctness locks. (Secondary summaries via search; the post itself, https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html, not opened.)
- [UNVERIFIED, not attempted] Burrows, Chubby OSDI 2006 (sequencers); Gray and Cheriton, SOSP 1989 (leases).
- The mapping to Choir's epoch in the append CAS, and the observation that effects outside the tape have no CAS (check-then-act race unless the effect carries the epoch/idempotency key), is analysis, unchanged.

### 4.6 The effect taxonomy these results imply (descriptive, not a recommendation)
Unchanged from the original: (1) idempotency-key API; (2) queryable outcome; (3) neither; (4) transactional with Choir's own store. Note for class 1: Stripe's own docs show the key has a retention window (24 h on v1) and a parameter-comparison check [V], so the precommit's retry horizon must be shorter than the target's retention. Choir's restart rule picks at-most-once for class 3 and for any in-doubt precommit; the open design choice is representation of in-doubt, who resolves it, and by what evidence.

## 5. The precommitment / intent-log pattern in prior art
- WAL/ARIES (Mohan et al. 1992): [UNVERIFIED, not attempted].
- Intent logs: Percolator primary lock [V-s], 2PC prepared record [UNVERIFIED], saga log [UNVERIFIED], TCC reservations [UNVERIFIED], Stripe/Brandur recovery points [V].
- Common structure (analysis): durable intent with unique id; act using the id as the external idempotency key if supported; durable outcome. Intent without outcome = in-doubt; redo/undo/query/escalate is policy.
- Choir's PSA records share only the "register before acting" shape.

## 6. Deterministic replay and event sourcing (adjacent)
- Event sourcing/CQRS: Fowler, Young [UNVERIFIED, not attempted]; Azure docs describe orchestrators as event sourced [V] (§2.3).
- ReVirt (Dunlap et al., OSDI 2002), rr (O'Callahan et al., USENIX ATC 2017): [UNVERIFIED, not attempted].
- [V-s] Calvin (Thomson, Diamond, Weng, Ren, Shao, Abadi, "Calvin: Fast Distributed Transactions for Partitioned Database Systems", SIGMOD 2012): all nodes agree on transaction order and execute deterministically, so distributed commit protocols are avoided; cost: non-conversational transactions only. https://www.cs.princeton.edu/courses/archive/fall19/cos418/papers/calvin.pdf
- [V-s] FoundationDB (Zhou et al., "FoundationDB: A Distributed Unbundled Transactional Key Value Store", SIGMOD 2021): deterministic simulation runs a whole cluster, network, disk and faults in one single-threaded process for testing. Official: https://apple.github.io/foundationdb/testing.html (appeared in results; not opened). Note: determinism there is a testing technique, distinct from Calvin's determinism in production execution. TigerBeetle simulator and Antithesis: [UNVERIFIED, not attempted].

## 7. Process and agent hierarchies in operating systems (control without owning state)

### 7.1 Unix process trees
- fork/wait/kill/setrlimit/ptrace, orphan reparenting, subreapers, SCM_RIGHTS, pidfd: [UNVERIFIED, not attempted; man pages].
- Ambient authority of the parent over the child (same uid) vs revocable grant: analysis.

### 7.2 cgroups (v2)
- [V] Delegation is by granting write access to the subtree's directory and control files to a less privileged user; the "no internal process constraint" (non-root cgroups can distribute resources to children only when they hold no processes themselves); `cgroup.kill` (write 1) SIGKILLs the cgroup and all descendants (fails with EOPNOTSUPP in threaded cgroups); `memory.oom.group` kills tasks "together or not at all"; `pids.max` is a hard process limit, default "max". https://docs.kernel.org/admin-guide/cgroup-v2.html (first 100k of 141k chars). [CORRECTED wording] "changing ownership" -> granting write access (ownership change is the usual mechanism, but the page says write access).
- That a child cannot exceed its parent's limit: [UNVERIFIED as quoted; part of the cgroup resource-distribution model on the page].
- Relevance: the clearest OS analogue of a grant with budget and scope plus subtree kill; enforces resources, not state confidentiality.

### 7.3 Plan 9 namespaces
- [UNVERIFIED, not attempted] Pike et al., "The Use of Name Spaces in Plan 9", 1993; "Plan 9 from Bell Labs", 1990. Relevance statement is analysis.

### 7.4 Kubernetes controllers
- [V] Leases are used so only one instance of a component runs (kube-controller-manager, kube-scheduler in HA); kubelet heartbeats update `spec.renewTime` of a Lease; the controller-manager releases its leader-election lock actively on transition rather than waiting for TTL. The page defines neither `holderIdentity`/`leaseDurationSeconds` semantics for leader election nor any fencing/clock-safety statement. https://kubernetes.io/docs/concepts/architecture/leases/ The survey's "known caveat that a timed lease needs fencing at the resource" is thus [UNVERIFIED on that page] (it is the Kleppmann argument, §4.5).
- spec/status split, level-triggered reconcile, ownerReferences/finalizers, RBAC, ValidatingAdmissionPolicy with CEL: [UNVERIFIED, controller page not opened]. Burns et al., ACM Queue 2016: not attempted.

## 8. Capability-safe supervision (control without access to state)
- Object-capability model (Dennis and Van Horn 1966; Miller 2006; E; "Capability Myths Demolished" 2003): [UNVERIFIED, not attempted]. Spritely Goblins/OCapN, a living descendant of E, is verified in §14.4.
- [V-s] Macaroons: Birgisson, Politz, Erlingsson, Taly, Vrable, Lentczner, NDSS 2014 (DOI 10.14722/ndss.2014.23212): chained-HMAC bearer credentials with caveats; "decentralized delegation with expressiveness comparable to public-key systems like SPKI/SDSI". https://www.ndss-symposium.org/ndss2014/programme/macaroons-cookies-contextual-caveats-decentralized-authorization-cloud/ The sub-claims "holder attenuates offline by appending caveats" and "verification needs the root key" are [UNVERIFIED] by the snippet but are the standard construction. Biscuit and UCAN: [UNVERIFIED, not attempted].
- seL4: [V-s] Klein et al., "seL4: Formal Verification of an OS Kernel", SOSP 2009, pp. 207-220: machine-checked proof of functional correctness from abstract spec to C (about 8,700 lines C, 600 lines assembler). [CORRECTED] The survey said this paper gives "proofs of integrity and confidentiality" and an authority-confinement property; that is not the 2009 result (those are later, separate papers, not fetched; so [UNVERIFIED] who proved what). Capability revoke deleting derived capabilities: [V-s] course slides and seL4 mailing list thread (note: a mailing-list post says the current design permits limited derivation levels; behavior should be checked against the seL4 manual). https://trustworthy.systems/publications/papers/Klein_EHACDEEKNSTW_09.pdf ; Elphinstone and Heiser, "From L3 to seL4", SOSP 2013, pp. 133-150 [V-s].
- Fuchsia: [V] `zx_handle_duplicate` is the only syscall that copies a handle and "Rights can be dropped during this process"; `zx_handle_replace` creates an equivalent handle "(possibly with fewer rights)" and invalidates the original. The page does not state that rights cannot be increased (implied, not stated). https://fuchsia.dev/fuchsia-src/concepts/kernel/handles
- Erlang pids as ambient, non-attenuable send capabilities and Akka ActorRef: analysis; [UNVERIFIED] as a quoted claim.

## 9. Multi-agent LLM orchestration frameworks (structural points only)
- LangGraph: [V] checkpointers "persist a thread's graph state as checkpoints", used for conversation continuity, human-in-the-loop, time travel and fault tolerance; keyed by `thread_id`; stores are for long-term cross-thread memory. https://docs.langchain.com/oss/python/langgraph/persistence [UNVERIFIED] on that page: saving after every superstep, the interrupted node re-running on resume (at-least-once), and fork-from-checkpoint semantics.
- AutoGen core: [V] a runtime "facilitates communication between agents" and manages lifecycles; standalone runtime for single-process, distributed runtime with a host servicer and workers connected via gateways. https://microsoft.github.io/autogen/stable/user-guide/core-user-guide/core-concepts/architecture.html [UNVERIFIED] actor-model framing, gRPC, (type,key) identity, pub/sub, per-agent single-threaded handling (other pages not opened).
- OpenAI Agents SDK: [V] handoffs "allow an agent to delegate tasks to another agent", exposed to the model as tools (`transfer_to_<agent>`), and "Handoffs stay within a single run"; Sessions/RunState track history. https://openai.github.io/openai-agents-python/handoffs/ Swarm lineage: [UNVERIFIED] (not mentioned there).
- Anthropic multi-agent research system: [V] orchestrator-worker; the lead agent plans, "spawns subagents to explore different aspects simultaneously", saves its plan to Memory, systems "resume from where the agent was when the errors occurred", and combine adaptability with "deterministic safeguards like retry logic and regular checkpoints". https://www.anthropic.com/engineering/multi-agent-research-system (resolves [?]; slightly different from the survey's wording "recovered by checkpointing the plan").
- Temporal/Restate/DBOS under agent loops as a 2025-2026 pattern: [V-s] partially; Dapr Workflow, AWS Lambda durable functions and DBOS marketing target AI agents (see §14); vendor claims.
- Structural takeaways (analysis, unchanged): no framework offers effect safety beyond idempotence advice; the control relationship is parent-invokes-child-awaits-result.

## 10. Mapping to the brief's questions (no recommendation)

### Q5: irreversible external effects under partition and offline
(Analysis, unchanged from the original except where the verified facts adjust it.)
1. Where is the commit point? Percolator [V-s], 2PC, and the outbox [V] place it at one durable write on one authority. The fabric doc places it at "home-durable precommit".
2. The window between act and outcome record exists everywhere (relay crash [V microservices.io], activity ack lost [V-s Temporal], step completed before checkpoint [V-s AWS, DBOS]). Pattern: make it detectable, pass a stable id, define resolution per effect class. Exactly-once only when the effect shares a commit with the log (DBOS transactional steps [V-s], Kafka-internal [V], Choir-owned store). AWS offers an explicit at-most-once-per-retry step mode [V-s, §14.3] as a built-in policy knob for this exact choice.
3. Fencing the actuator: epoch checks work for stores that accept a token; mail and payments get idempotency key plus single-actuator rule, which leaves lease-handoff races.
4. Branches: sagas offer forward-only with notice; no merge for effects.
5. Offline policy classes fit the taxonomy of §4.6.
6. Two-writer failover hazard: shared external dedupe key derived from intent id and occurrence id, or restrict failover branch to deduping targets. Note target-side retention (Stripe v1: keys pruned after at least 24 h [V]) bounds how long a derived key stays protective.
7. Choir owns maild: "send" is a local enqueue (class 4), leaving downstream SMTP as at-least-once to the recipient.
Open design questions: in-doubt as first-class tape state; key-derivation rule; who resolves in-doubt and with what evidence; dedupe retention vs retry horizon; whether the WAL-sense and PSA-sense records are one object.

### Q6: one computer controlling another without writing its state
Convergent patterns (analysis, with the verified instances): controller writes intent into the controlled system's input channel and the controlled system writes its own state (actor mailbox, Orleans grain messages, Durable Functions entity signals [V], Restate virtual-object handlers [V], Kubernetes spec/status [UNVERIFIED], Choir addressed publication); lifecycle control separated from state control (Erlang exit signals/monitors [V], `cgroup.kill` [V], OTP supervisor); attenuable/revocable references (ocap [UNVERIFIED], macaroons [V-s], cgroup delegation [V], Fuchsia handle rights [V]); bounds and escalation (OTP intensity [V], cgroup `pids.max` [V]); supervision verdicts as "no report by deadline" because a monitor on a partitioned child cannot distinguish dead from unreachable (`noconnection` [V], Akka unreachable [V]). Warnings from literature: confused deputy, grants outliving purpose, authority amplification [UNVERIFIED, ocap literature not opened].

### Q10: self-modifying or self-tuning inside proven bounds
- Supervision as safety net for change: OTP hot code loading with supervised rollback (appup/relup): [UNVERIFIED, not attempted].
- Policy as data over a verified kernel: Step Functions state machines [V as data], Kubernetes admission CEL [UNVERIFIED], cgroup ceilings [V], and Cedar. [V-s, resolves [?]] Cutler et al., "Cedar: A New Language for Expressive, Fast, Safe, and Analyzable Authorization", PACMPL OOPSLA 2024 (arXiv 2403.04651): designed for a sound and complete logical encoding enabling precise policy analysis; formalized in Lean; Rust implementation; used in Amazon Verified Permissions and Verified Access. https://www.amazon.science/publications/cedar-a-new-language-for-expressive-fast-safe-and-analyzable-authorization ; https://arxiv.org/abs/2403.04651 (the specific "SMT" wording is not in the snippets; "logical encoding" is.)
- Runtime assurance: Sha, "Using Simplicity to Control Complexity", IEEE Software 2001: [UNVERIFIED] (search returned nothing). Shielding: [V-s] Alshiekh, Bloem, Ehlers, Könighofer, Niekum, Topcu, "Safe Reinforcement Learning via Shielding", AAAI 2018, pp. 2669-2678 (arXiv 1708.08611): a shield around the learning agent that restricts it as little as possible and forbids actions only if they could endanger safe behavior. https://ojs.aaai.org/index.php/AAAI/article/view/11797
- Code changes under replay: Temporal's patching and Worker Versioning [V] exist because changing code with histories in flight breaks determinism; Azure docs say the same of orchestrators ("Code changes that affect running orchestrations can break replay behavior", [V]); AWS durable functions raise `NonDeterministicExecutionError` on step name/order mismatch [V-s]. A self-developing system whose desks replay from the tape faces the same unless replay never re-executes desk logic.
- Deterministic simulation for verifying a configuration space: FoundationDB [V-s]; TLA+ model checking of lease/handoff: [UNVERIFIED, not attempted].
- Limitation (analysis): these prove a policy cannot breach kernel invariants, not that the policy is good.

## 11. Recurring kernel primitives (a cross-cutting list, for the owner's Q9 thinking)
1. A monotone epoch / fencing token checked at the point of mutation: Kafka producer epoch [V], Temporal shard RangeID "generation number used for fencing" [V], Orleans Etag [V], Chubby sequencer [UNVERIFIED], Choir append CAS.
2. Journal before act, with a unique intent id that doubles as the external idempotency key (Stripe [V], Brandur recovery points [V], AWS idempotency keys [V-s]).
3. In-doubt as an explicit state, resolved by evidence or an authorized actor.
4. A single-authority commit point (Percolator primary [V-s]).
5. Link/monitor with bounded restart and escalation (OTP [V], cgroup `pids.max` [V], K8s backoff [UNVERIFIED]).
6. Attenuating, revocable grants; authority never widens along a chain (Fuchsia [V], macaroons [V-s]).
7. Intent flows to the controlled; state is written only by its owner.
8. Deterministic core + simulation for verification.
Policy variants layered on those: effect classes and offline allowances, retry and restart intensity, who may resolve in-doubt, budgets, failover thresholds.

## 12. Primary sources (consolidated)

[V] opened this session:
- Erlang: https://www.erlang.org/doc/system/sup_princ.html ; https://www.erlang.org/doc/system/ref_man_processes.html
- Akka: https://doc.akka.io/libraries/akka/current/split-brain-resolver.html ; .../typed/cluster-sharding.html ; .../typed/cluster-singleton.html
- Orleans: https://learn.microsoft.com/en-us/dotnet/orleans/grains/grain-persistence/ ; .../grains/timers-and-reminders ; .../grains/grain-lifecycle ; .../grains/request-scheduling ; MSR-TR-2014-41 page https://www.microsoft.com/en-us/research/publication/orleans-distributed-virtual-actors-for-programmability-and-scalability/
- Ray: https://docs.ray.io/en/latest/ray-core/fault_tolerance/actors.html ; .../gcs.html
- Temporal: https://docs.temporal.io/workflow-definition ; https://docs.temporal.io/activities ; https://github.com/temporalio/temporal/blob/main/docs/architecture/history-service.md
- Restate: https://docs.restate.dev/concepts/durable_execution ; .../foundations/key-concepts ; .../develop/ts/durable-steps
- Step Functions: https://docs.aws.amazon.com/step-functions/latest/dg/choosing-workflow-type.html ; .../connect-to-resource.html
- Azure Durable: https://learn.microsoft.com/en-us/azure/azure-functions/durable/durable-functions-orchestrations ; .../durable-functions-code-constraints ; .../durable-functions-entities ; .../durable-functions-storage-providers
- DBOS: https://docs.dbos.dev/architecture
- Stripe: https://docs.stripe.com/api/idempotent_requests ; IETF: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header ; Brandur: https://brandur.org/idempotency-keys
- Outbox: https://microservices.io/patterns/data/transactional-outbox.html
- Kafka: https://cwiki.apache.org/confluence/display/KAFKA/KIP-98+-+Exactly+Once+Delivery+and+Transactional+Messaging
- cgroup v2: https://docs.kernel.org/admin-guide/cgroup-v2.html ; K8s Leases: https://kubernetes.io/docs/concepts/architecture/leases/ ; Fuchsia handles: https://fuchsia.dev/fuchsia-src/concepts/kernel/handles
- LangGraph: https://docs.langchain.com/oss/python/langgraph/persistence ; AutoGen: https://microsoft.github.io/autogen/stable/user-guide/core-user-guide/core-concepts/architecture.html ; OpenAI Agents SDK handoffs: https://openai.github.io/openai-agents-python/handoffs/ ; Anthropic: https://www.anthropic.com/engineering/multi-agent-research-system
- Cloudflare Durable Objects: https://developers.cloudflare.com/durable-objects/concepts/what-are-durable-objects/ ; Dapr Workflow: https://docs.dapr.io/developing-applications/building-blocks/workflow/workflow-overview/

[V-s] search-snippet confirmed: Temporal community/blog (activity guarantees); DBOS blog and README; Burckhardt et al. OOPSLA 2021 and Netherite arXiv 2103.00033; Ownership NSDI 2021; Percolator summaries; Kleppmann summaries; Sagas/Helland/Idempotence bibliographic; Macaroons NDSS 2014; seL4 SOSP 2009 and Elphinstone-Heiser SOSP 2013; Cedar OOPSLA 2024; Shielding AAAI 2018; Calvin SIGMOD 2012; FoundationDB SIGMOD 2021; AWS Lambda durable functions docs; Spritely Goblins docs; Orleans FAQ.

[UNVERIFIED, not attempted or failed] Hewitt/Agha/Clinger; Armstrong thesis; Bykov SoCC 2011; Moritz Ray OSDI 2018 (search miss); Cadence; Skiadopoulos DBOS VLDB 2021; Gray 1978, Gray-Reuter, BHG, Skeen, Gray-Lamport, Spanner; ARIES; Saltzer-Reed-Clark; Burrows Chubby; Gray-Cheriton; Dunlap ReVirt; O'Callahan rr; Fowler/Young; Pike Plan 9; Dennis-Van Horn; Miller thesis and Capability Myths; Biscuit/UCAN; Sha Simplex (search returned nothing); Burns Borg/Omega/K8s; Kubernetes controller doc; Flink TwoPhaseCommitSinkFunction (404); TLA+; Debezium; Kleppmann post itself; Unix man pages.

## 13. Verification gaps
- Orleans at-most-once default delivery: primary TR PDF not machine-readable here; confirmed only that the directory/convergence behavior exists.
- Restate `ctx.run` delivery semantics and the effect-before-journal window: not stated in the pages opened.
- Temporal range-ID enforcement and signals/child-workflow policies not read.
- Several fetches returned truncated pages (cgroup 100k/141k chars; Akka sharding 100k/130k).
- Classic papers were mostly not attempted; their claims are carried as [UNVERIFIED].
- seL4's integrity/confidentiality result needs its own citation.
- §0 not re-checked against the go-choir design docs.

## 14. Added prior art (missed by the original)

### 14.1 Cloudflare Durable Objects [ADDED, V partial]
Each object has a "globally-unique name" so requests can reach a specific object from anywhere; "single-threaded and cooperatively multi-tasked"; its own "durable, transactional, and strongly consistent storage"; an Alarms API wakes the object at a future time. The page did not state "at most one instance" nor input/output gates (not confirmed). https://developers.cloudflare.com/durable-objects/concepts/what-are-durable-objects/ Shape: virtual actor with per-object storage, alarms as the durable timer (cf. Orleans reminders and Choir standing intents).

### 14.2 Dapr Workflow [ADDED, V partial]
"A durable execution engine" run via the sidecar; workflows resume from the last completed step after a crash/restart/redeploy using persisted results; a "fast path" keeps at-least-once execution unchanged and uses a per-instance janitor reminder; only certain state stores support workflows; timers and reminders work as in Dapr actors. Determinism, replay and actor-state-store claims not on the page. https://docs.dapr.io/developing-applications/building-blocks/workflow/workflow-overview/

### 14.3 AWS Lambda durable functions [ADDED, V-s]
Steps have at-least-once semantics by default ("during replay, the SDK returns checkpointed results without re-executing completed steps"); an `AT_MOST_ONCE_PER_RETRY` mode is recommended for financial transactions and inventory; idempotency keys recommended for payments and DB writes; replay validates step names and order and raises `NonDeterministicExecutionError`; state outside steps (closure/global) is not restored on replay; `CheckpointDurableExecution` is the SDK's checkpoint API; event source mappings do not support idempotency at launch (each invocation starts a new durable execution). Sources (search snippets of AWS docs): https://docs.aws.amazon.com/lambda/latest/dg/durable-execution-idempotency.html ; https://docs.aws.amazon.com/lambda/latest/dg/durable-best-practices.html ; https://docs.aws.amazon.com/lambda/latest/dg/API_CheckpointDurableExecution.html A third-party post notes a step interrupted before its completion checkpoint re-runs under the default mode, and that at-most-once per step alone does not give exactly-once per workflow (commentary, not official).
Relevance: an engine exposing the at-most-once vs at-least-once choice as a per-step policy flag, matching Choir's "owner policy per effect class".

### 14.4 Spritely Goblins / OCapN / CapTP [ADDED, V-s]
Distributed object programming environment: actors live in vats (actormaps), process messages in atomic turns (cheap local transactions), change behavior by returning a replacement; vats communicate asynchronously in or across processes; authority is possession of a reference; OCapN is the cross-network protocol with CapTP as transport; third-party handoff (A gives C a capability to an object on B, C connects directly to B); promise pipelining; inspired by E. OCapN drafts are explicitly not final and change significantly. Sources via search: https://files.spritely.institute/docs/guile-goblins/0.12.0/What-is-Goblins_003f.html ; https://lwn.net/Articles/960912/ (some other results were third-party notes; not relied on). Directly relevant to Q6/Q3: an actor system whose control relation is capability-by-reference with revocation patterns from E.

### 14.5 Temporal's own framing of effectively-once [ADDED, V-s]
"At-least-once delivery + idempotency = effectively-once" (Temporal standalone-activities tutorial and idempotency blog; see §2.1).

### 14.6 Candidates noted but not verified (not claimed)
Akka Persistence (journal fencing), Service Fabric Reliable Actors, Golem, Inngest, Cloudflare Workflows, Camunda/Zeebe, Pony reference capabilities, Waterken/Web-key, Debezium outbox, TigerBeetle/Antithesis. Each is a plausible addition but not checked this session.
