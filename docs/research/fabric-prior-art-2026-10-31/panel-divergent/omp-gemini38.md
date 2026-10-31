Working...
# Choir Resource Fabric: Architectural Option Space (Lens: Organization)

Mode: **Divergent**. Eight candidate kernel architectures derived from first principles. Each establishes a distinct set of minimal kernel primitives and guaranteed invariants, leaves a specific policy surface open for organizational configuration and self-development, descends from distinct prior art, and identifies the sharpest trade-offs and unsafe configurations.

---

### 1. The Cryptographic Capability-Mesh Kernel (Zero-Hierarchy Object-Capabilities)

#### 1. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `Keypair`: Asymmetric cryptographic identity for every entity (person, computer, site, capability helper).
  2. `TapeBranch`: Append-only, content-addressed event log signed by a keypair.
  3. `CapabilityToken`: Cryptographically chained, attenuating delegation ticket (e.g., UCAN/Macaroon) carrying `(Issuer, Audience, ResourceURI, Verb, Caveats[], Signature)`.
  4. `EffectPrecommitment`: Cryptographic proof of intent that requires a matching signed execution receipt to resolve on the tape.
- **Guaranteed Properties**:
  - The kernel contains *no concept* of "organizations", "roles", or "servers".
  - Authority is unforgeable, self-verifying, and strictly delegable via attenuation (rights can only shrink down a delegation chain, never expand).
  - Every interaction (work placement, tape append, screen capture, model call) is admitted if and only if accompanied by a cryptographically valid token chain rooted in the resource owner's key.

#### 2. Why Genuinely Different
Replaces directory services, central homes, and access-control lists (ACLs) with proof-carrying authorization. An "organization" is not an administrative entity in the kernel; it is simply a root cryptographic keypair. Access control, work placement on employee machines, and cross-computer control share an identical mathematical primitive: verifying a signature chain against cryptographic caveats.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Reachability (Q1, Q2)**: Writer lease is a time-attenuated capability token signed by the computer's owner key to a site's host key. Reachability relays are untrusted byte-shufflers verified by mutual TLS keyed to entity keypairs; any site can relay traffic without seeing plaintext.
- **Access Control & Placement (Q3, Q4)**: Employee A's workstation generates a host key. Its local admission kernel refuses any work lacking a capability token signed directly by A's key. The company cluster holds a delegation token from the org key; it cannot place work on A's workstation unless A has signed a token delegating specific leaf-execution rights to the org.
- **Irreversible Effects & Control (Q5, Q6)**: External actions require a precommitment token signed by the computer; outbox workers execute only with a valid delegation. Computer A controls Computer B by presenting an addressed proposal token that B's policy predicate evaluates.
- **Lifecycle (Q8)**: Onboarding is signing a delegation token from the org key to the employee. Offboarding is revoking the token via cryptographic accumulators or allowing short-lived epoch caveats to lapse.

#### 4. Policy Surface Left Open
- Delegation topologies (hierarchical, peer-to-peer, consortium).
- Caveat grammar (time windows, rate limits, spend caps, data-redaction tags).
- Self-development: An agent can autonomously mint attenuated sub-tokens to delegate scoped tasks to child computers or leaf VMs without touching platform infrastructure or requesting admin approval.

#### 5. Prior Art
- **UCAN** (User Controlled Authorization Networks, Fission/Banyan)
- **Google Macaroons** (cookies with contextual caveats and third-party proofs)
- **SPKI/SDSI** (Rivest & Lampson: Simple Public Key Infrastructure)
- **Tahoe-LAFS** (capability-based decentralized storage grid)
- **KeyKOS / CapROS** (pure capability operating system microkernels)

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Revocation without a central authority is expensive. Either tokens must be short-lived (requiring frequent online re-signing, which breaks offline/airplane work), or the fabric must maintain synchronized revocation accumulators (CRLs/merkle trees), re-introducing consensus overhead.
- **Unsafe Configurations**: Ambient trust environments. An organization requiring instant, unconditional emergency seizure of all employee-hosted corporate data cannot enforce it if the employee's machine goes offline while holding valid, non-expired delegation tokens.

---

### 2. Bilateral Federated Realms Kernel (Domain-Sovereign Federation)

#### 2. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `RealmAuthority`: Explicit administrative domain entity holding a root authority key (`@org.com`, `@alice.net`).
  2. `ScopedIdentity`: Compound identifier binding entity to realm (`comp1@org.com`, `host1@alice.net`).
  3. `BorderRelay`: Cryptographic gateway mediating all inter-realm traffic.
  4. `BilateralTrustContract`: Signed treaty between two Realm Authorities defining permitted cross-realm routing, capability scopes, and verification endpoints.
- **Guaranteed Properties**:
  - Administrative sovereignty is absolute: a realm boundary cannot be crossed without traversal of a Border Relay governed by a Bilateral Trust Contract.
  - A realm authority possesses absolute jurisdiction over all identities, leases, and sites enrolled under its domain.

#### 2. Why Genuinely Different
Rejects universal flatness. Rather than treating all nodes as peers in a shared mesh, it establishes a hard two-tier topology: intra-realm operations are trusted/managed by a realm authority, while inter-realm operations are strictly untrusted, protocol-mediated external diplomacy.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Reachability (Q1, Q2)**: The computer's realm home holds the authoritative lease row. Reachability across domains resolves via DNS-like realm discovery; phones reach computers via their home realm gateway.
- **Access Control & Placement (Q3, Q4)**: Employee A's workstation enrolls in `@alice.net` (or a personal sub-realm). The company server is `@org.com`. The rule "A's VM cannot run on B's workstation" is enforced at the realm boundary: `@alice.net` exposes zero execution endpoints to `@bob.net`. Work placement across realms is rejected at the Border Relay unless an explicit cross-realm treaty exists.
- **Native Capabilities & Control (Q6, Q7)**: Comms computer (`comms@org.com`) publishes proposals across the border relay to `alice_computer@org.com` or `alice_computer@alice.net`. The proposal is audited and admitted at the destination realm boundary.
- **Lifecycle (Q8)**: Onboarding is provisioning an identity under `@org.com`. Offboarding is deleting the identity from the realm directory; the realm authority instantly revokes all realm-issued certificates and shuts down home relaying.

#### 4. Policy Surface Left Open
- Federation contracts (which external domains are peered, quota limits between domains).
- Intra-realm administrative policies (centralized Active Directory/LDAP style vs. lightweight personal homeservers).
- Self-development: Agents can modify intra-computer desk logic and propose cross-realm interactions, but the kernel guarantees that no self-development loop can expand its reach beyond the realm boundary without external realm-admin intervention.

#### 5. Prior Art
- **Kerberos Cross-Realm Trusts** & Active Directory Forest Federations
- **Matrix Homeserver Federation Protocol**
- **ActivityPub / W3C Fediverse**
- **Andrew File System (AFS) Cells**
- **DNS / SMTP Architecture**

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: High administrative friction and boundary brittleness. Moving a computer between personal and corporate realms requires complete cryptographic re-homing or complex dual-homed identities.
- **Unsafe Configurations**: Seamless ad-hoc collaboration. Highly dynamic, ephemeral teams (cross-company joint ventures, temporary contractors) require continuous negotiation of bilateral realm treaties, leading operators to configure unsafe wildcard peering rules (`*@* permitted`) that defeat the architecture.

---

### 3. Distributed Microkernel Namespace Fabric (Synthetic Filesystem & IPC)

#### 3. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `Channel`: Authenticated, bidirectional, ordered byte stream between two processes (host-to-host or host-to-guest).
  2. `Namespace`: Per-computer hierarchical tree of synthetic files and directories dynamically constructed via mount tables.
  3. `SyntheticServer`: Process that serves state, hardware, or capabilities as a directory tree via a minimal IPC protocol (e.g., 9P2000 / modern equivalent).
  4. `FileDescriptorCapability`: Handle representing an open synthetic file supporting read, write, and compare-and-swap operations.
- **Guaranteed Properties**:
  - Everything is a file: tapes are `/tape/events`, leases are `/tape/lease`, screens are `/dev/screen`, leaf compute slots are `/cpu/spawn`.
  - Isolation is namespace construction: a process cannot name, address, or interact with any resource that is not bound into its private mount table.

#### 2. Why Genuinely Different
Eliminates bespoke domain-specific protocols for audio, video, tape streaming, and remote execution. Hardware devices, cloud VMs, database heads, and inter-agent communication share a single unified abstraction: reading and writing synthetic filesystem nodes over authenticated IPC channels.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Native Caps (Q1, Q7)**: The tape is an append-only file `/n/tape/events`. Writing is an authenticated write transaction. The Mac microphone is `/n/local/mic`; computer use is `/n/local/screen`. Capability mediation is standard filesystem permission bits and open-flags checked at mount time.
- **Access Control & Placement (Q3, Q4)**: Employee A's workstation exports its compute interface as a 9P server listening only on A's local channel. Employee B cannot access A's workstation because A's host channel is completely absent from B's namespace mount table. Work placement is simply opening `/net/cluster/burst/new_vm` and writing a guest specification.
- **Computer Control (Q6)**: Computer A controls Computer B by mounting B's public inbox directory `/net/compB/inbox` and writing a proposal file.
- **Lifecycle (Q8)**: Corporate onboarding mounts the corporate synthetic tree `/org/...` into the employee's computer namespace. Offboarding unmounts `/org`; corporate files and servers become unreachable instantly.

#### 4. Policy Surface Left Open
- Mount table composition rules (which servers mount where and under what union flags).
- Synthetic server logic (how a fileserver handles writes, e.g., triggering a model call on write to `/dev/ai/prompt`).
- Self-development: An agent can reconfigure its own namespace, mount new tools, bind local caches, and alias interfaces, while the kernel enforces that it cannot mount any remote channel without presenting valid credentials.

#### 5. Prior Art
- **Plan 9 from Bell Labs** (per-process namespaces, 9P protocol, `/net` device)
- **Amoeba Distributed Operating System** (Tanenbaum et al.)
- **Inferno / Styx OS**
- **Mach Microkernel Ports & IPC**
- **FUSE / Linux Virtual File System (VFS)**

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Network partition semantics on filesystem operations are historically awkward. Simulating distributed consensus, atomic lease fencing, and event-log branching over filesystem read/write/stat primitives requires complex synthetic control semantics (`/tape/ctl`), often obscuring network failures behind blocked I/O calls.
- **Unsafe Configurations**: High-throughput distributed data streaming. Piping bulk model weights, VM snapshots, and multi-gigabyte audio streams through a serialized, stateful 9P-like channel abstraction introduces severe serialization and context-switching bottlenecks compared to zero-copy data planes.

---

### 4. Sovereign Local-First Peer Mesh (Gossip & Validation-Predicate Kernel)

#### 4. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `PeerID`: Autonomous cryptographic identity.
  2. `BranchDAG`: Append-only, content-addressed directed acyclic graph of event blocks.
  3. `ValidationPredicate`: Pure, deterministic Wasm bytecode defining the valid state transitions for a given tape or topic.
  4. `AntiEntropySync`: Symmetric 3D range-reconciliation protocol (Path, Time, Peer) between any two reachable peers.
- **Guaranteed Properties**:
  - Complete elimination of privileged "home" servers; servers are simply peers with high availability and static network addresses.
  - Local-first sovereignty: a computer always accepts local writes to its own branch; network partitions never block local execution.
  - State validity is decentralized: every peer validates received blocks using the associated `ValidationPredicate`; invalid blocks are rejected regardless of who signed them.

#### 2. Why Genuinely Different
Radically inverts authority: removes the central lease coordinator and authoritative home registry entirely. Consistency is achieved through asynchronous anti-entropy gossip and deterministic event validation rather than synchronous locking or centralized fencing.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Partition (Q1, Q5)**: Writer authority is branch-based. If a user works on their Mac on a plane and their phone concurrently appends to the server, both branches are valid hash-chains. When connectivity restores, both branches sync via anti-entropy. Irreversible external effects (paying money, sending email) require a deterministic multi-peer escrow predicate embedded in the event DAG.
- **Access Control & Placement (Q3, Q4)**: Work placement is peer-to-peer task advertisement. Employee A's workstation announces compute availability over a restricted gossip topic encrypted with keys shared only among A's devices. Employee B's machine does not have the decryption keys for A's gossip channel and cannot discover or request compute on A's hardware.
- **Lifecycle (Q8)**: An organization is a shared gossip topic with a consortium validation predicate. Offboarding rotates the topic encryption keys; the ex-employee retains their local history (which is physically on their disk) but cannot decrypt future org gossip or sync new events to the org cluster.

#### 4. Policy Surface Left Open
- Validation predicate design (authoring business rules, financial spending bounds, and governance voting rules in Wasm).
- Gossip topology and peer discovery heuristics (MDNS on local LAN, static relay nodes, Bluetooth mesh).
- Self-development: Agents can synthesize new deterministic validation predicates and propose state transitions; the kernel guarantees that no local agent modification can force remote peers to accept invalid blocks.

#### 5. Prior Art
- **Willow Protocol** (Earthstar Project: 3D range sync, prefix pruning)
- **Secure Scuttlebutt (SSB)** (append-only gossip logs)
- **Holochain** (DHT + deterministic application validation rules)
- **Ink & Switch Local-First Research** (Kleppmann et al.)
- **Git** (distributed branch/commit graph without merge)

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Eventual consistency and split-brain proliferation. In the absence of a linearizable lease coordinator, diverging branches require explicit human or policy-level publication/adoption to reconcile. Real-time interactive collaboration between two devices without network connectivity inevitably produces split histories.
- **Unsafe Configurations**: Systems requiring strict, real-time single-copy asset ownership (e.g., immediate double-spend prevention, strict single-instance software licenses). Without a synchronous coordinator, enforcing an absolute single writer across disconnected machines is theoretically impossible under CAP.

---

### 5. Formally Verified Separation Kernel with Decoupled Policy Engine (DIFC / Logic Guard)

#### 5. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `ContextTriple`: Canonical representation of every platform event `(Subject, Verb, Object, EnvironmentAttributes)`.
  2. `LogicEngine`: Deterministic, formally verifiable policy decision point (e.g., Datalog, Cedar, or decidable First-Order Logic).
  3. `PolicyBundle`: Cryptographically signed, immutable declarative ruleset defining permissible state transitions, placement rules, and hardware access.
  4. `EnforcementPoint`: Minimal, hypervisor-level barrier that halts execution until the LogicEngine returns a verifiable safety proof.
- **Guaranteed Properties**:
  - Total separation of mechanism and policy: the kernel contains zero business logic, zero hardcoded org rules, and zero placement heuristics.
  - Formal verifiability: all configurations can be statically verified against high-level safety invariants (e.g., "no data tagged `OrgConfidential` may reach a site tagged `PersonalWorkstation`") via SMT solvers (Z3) prior to policy activation.

#### 2. Why Genuinely Different
Treats distributed authorization, scheduling, and resource allocation as a continuous automated theorem-proving problem. Rather than relying on procedural code, access tokens, or static filesystem permissions, every single state transition must be proved safe by an embedded declarative logic engine.

#### 3. Organizational Model & Handling of Core Challenges
- **Access Control & Placement (Q3, Q4)**: The rule "Employee A's VM cannot run on Employee B's workstation" is expressed as a pure declarative constraint:
  `forbid(PlaceWorkload, Site, VM) :- VM.owner != Site.owner, Site.trust_class == "workstation", not ExplicitGuestGrant(Site, VM.owner).`
  The placement engine queries the LogicEngine; invalid placements fail mathematically at the enforcement barrier.
- **Native Capabilities & Effects (Q5, Q7)**: Invoking the Mac microphone or executing an irreversible email send requires a proof from the LogicEngine that the current environment satisfies all precommitment, ring-grant, and rate-limiting rules.
- **Lifecycle (Q8)**: Organization lifecycle is managed via signed Policy Bundles. Onboarding injects a rule linking the employee's ComputerID to the org's resource pool. Offboarding deploys an updated Policy Bundle that revokes all matching rules; the change takes effect instantaneously and uniformly across the fabric.

#### 4. Policy Surface Left Open
- The entire organizational rulebook: roles, hierarchies, spend limits, temporal restrictions, dual-custody authorization.
- Self-development: A self-tuning agent can optimize its operational parameters, desk weights, and scheduling heuristics within an "autonomous policy sandbox", while the SMT solver guarantees that the agent's proposed policy updates cannot violate the organization's root safety axioms.

#### 5. Prior Art
- **seL4 Microkernel** (formal verification of functional correctness and security)
- **AWS Cedar / Open Policy Agent (Rego)** (declarative policy engines)
- **Flask Architecture / SELinux** (strict decoupling of policy decision from enforcement)
- **Asbestos & HiStar** (Decentralized Information Flow Control operating systems)
- **Nexus Authorization Logic** (Schneider et al., Cornell)

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Policy evaluation latency and complexity explosion. Evaluating complex Datalog queries on every event append and capability invocation introduces computational overhead. Rule interactions in large organizations can cause subtle policy deadlocks (e.g., work cannot be placed anywhere because mutually exclusive constraints negate all candidate sites).
- **Unsafe Configurations**: Highly dynamic, rapid-fire operational environments where policy rules are modified ad-hoc by non-technical users without automated formal verification, resulting in unintended denial-of-service or privilege escalation via policy conflict.

---

### 6. Linearizable Consensus Fabric Kernel (Chubby / Raft Replicated Coordinator)

#### 6. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `ConsensusTerm`: Monotonically increasing epoch number driven by a replicated consensus quorum (Raft/Paxos/BFT).
  2. `LinearizableCell`: Authoritative distributed state cell holding global leases, site records, and network routes.
  3. `FencedLeaseToken`: Lease grant bearing an incremented epoch counter; any write presenting an older epoch is unconditionally rejected by storage hosts.
  4. `TwoPhaseOutbox`: Transactional staging queue for irreversible external effects.
- **Guaranteed Properties**:
  - Strong linearizability: split-brain, stale writes, and uncoordinated branch forks are mathematically precluded.
  - Single state authority: at any given microsecond, there is unambiguous, global agreement on exactly which site holds the primary writer lease for any computer branch.

#### 2. Why Genuinely Different
Prioritizes consistency and safety over partition tolerance (CP in Brewer's theorem). Rejects the concept of autonomous, offline lease checkout without timers; authority is always held relative to a linearizable consensus quorum.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Fencing (Q1)**: The server cluster runs a Raft consensus ring. Moving a writer from the server to Alice's Mac requires a linearizable consensus transaction that increments the epoch and issues a `FencedLeaseToken`. If the Mac disconnects and fails to renew its lease heartbeat within a strict deadline, the consensus ring revokes the lease and fences the Mac host.
- **Reachability & Routing (Q2)**: The phone reaches the computer by querying the consensus cell's live route register; routes update with linearizable consistency.
- **Access Control & Placement (Q4)**: The consensus ring maintains the central admission registry. Placement is scheduled by a central fabric coordinator that evaluates site capacity, trust classes, and org policies in a single consistent pass.
- **Lifecycle & Revocation (Q8)**: Offboarding is a single atomic consensus commit. The employee's computer is fenced, lease tokens are canceled, and org graph permissions are wiped simultaneously across all sites.

#### 4. Policy Surface Left Open
- Consensus membership and quorum configurations (e.g., multi-region, multi-org consortium).
- Scheduling algorithms and placement heuristics in the central coordinator.
- Self-development: Agents can request lease handoffs, spawn child tasks, and adjust internal application state, but cannot alter the fundamental fencing invariants or override consensus timeouts.

#### 5. Prior Art
- **Google Chubby** (distributed lock service for loosely-coupled distributed systems)
- **Apache ZooKeeper / etcd** (linearizable coordination primitives)
- **Spanner** (TrueTime and distributed atomic transactions)
- **Hyperledger Fabric** (permissioned enterprise consensus fabric)
- **Tendermint / CometBFT** (state-machine replication with Byzantine fault tolerance)

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Total intolerance of network partitions. When an employee's workstation loses connectivity to the consensus cluster (airplane mode, mobile tether, ISP outage), the computer must halt all write operations or force an uncoordinated hard branch that requires severe manual reconciliation.
- **Unsafe Configurations**: Offline-heavy or intermittently connected mobile workforces. Configuring long lease TTLs to accommodate offline work creates massive availability freezes (if a machine crashes while holding a long lease, no other machine can assume writer authority until the TTL expires).

---

### 7. Hardware-Attested Information Flow Control (IFC) Enclave Kernel (Confidential Computing / DLM)

#### 7. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `SecurityLabel`: Structured tag `{Secrecy: {Entities}, Integrity: {Entities}}` attached to every memory page, tape event, and network frame.
  2. `AttestedEnclave`: Isolated execution environment backed by hardware root-of-trust (AMD SEV-SNP, Intel TDX, Apple Secure Enclave) capable of generating verifiable cryptographic attestation quotes.
  3. `MemoryEncryptionKey`: Hardware-enforced key tied strictly to an enclave instance, inaccessible to the host OS.
  4. `DeclassificationGate`: Explicit, auditable cryptographic operator required to lower secrecy or raise integrity.
- **Guaranteed Properties**:
  - The physical operator of a site cannot inspect, alter, or tamper with the memory or execution of a guest running inside an attested enclave, even with full root/kernel privileges on the host OS.
  - Mandatory information flow control: data tagged with a secrecy label cannot leave an enclave unless encrypted with a key matching the label's authorized recipient set.

#### 2. Why Genuinely Different
Directly invalidates the fundamental premise that "Employee A's work cannot run on Employee B's machine". By leveraging hardware memory encryption and remote attestation, untrusted physical hardware can be safely used to run highly confidential workloads without trusting the site's human owner or host OS.

#### 3. Organizational Model & Handling of Core Challenges
- **Placement & Multi-Tenancy (Q4)**: The organization cluster can place burst leaves or even full writer realizations of Employee A on Employee B's workstation. B's machine runs the workload inside an AMD SEV-SNP or Apple hypervisor enclave. B cannot inspect A's memory, dump keys, or read network buffers. Workstation compute becomes a completely fungible, zero-trust utility commodity.
- **Access Control & Native Capabilities (Q3, Q7)**: Native capabilities (screen, mic) are exposed through mediated I/O proxies that attach strict integrity and secrecy labels. If an on-device local model processes confidential text, its output memory carries the same secrecy label and is prevented from writing to unencrypted host sockets.
- **Lifecycle & Offboarding (Q8)**: Offboarding is cryptographic zeroization. The organization simply destroys the enclave decryption keys held in the root key management service; all local data on the employee's machine becomes permanent cryptographically unrecoverable noise.

#### 4. Policy Surface Left Open
- Security label lattice definitions (defining corporate secrecy hierarchies, departmental compartments).
- Declassification policies (rules specifying what level of human review or cryptographic proof permits data to be published externally).
- Self-development: Agents can dynamically instantiate enclaves and design internal workflows, provided all data movements satisfy the hardware-enforced label lattice.

#### 5. Prior Art
- **Myers & Liskov Decentralized Label Model (DLM)**
- **Jif (Java Information Flow)** & **HiStar / Flume** (DIFC systems)
- **AMD SEV-SNP / Intel TDX / ARM CCA** (hardware confidential VM architectures)
- **Apple Private Cloud Compute (PCC)** (hardware-attested stateless compute nodes)
- **Enarx / Open Enclave SDK**

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Severe hardware dependency, microarchitectural side-channel vulnerabilities, and extreme debugging difficulty. Enclaves are black boxes: if a VM deadlocks inside an encrypted memory boundary, host diagnostic tools cannot inspect stack traces. Hardware vulnerabilities (Spectre, cache timing, rowhammer) can breach isolation on shared consumer silicon.
- **Unsafe Configurations**: Heterogeneous fleets containing legacy or low-cost hardware lacking hardware-enforced virtualization encryption (e.g., older PCs, budget ARM SBCs). Attempting to enforce this model without ubiquitous hardware silicon support creates massive security holes.

---

### 8. Asynchronous Actor-Vat & Capability-Passing Kernel (E-Language / CapTP Object Network)

#### 8. Core Idea & Guaranteed Properties
- **Primitives**:
  1. `Vat`: Single-threaded, isolated execution context that processes turns deterministically and records state commits to an append-only tape.
  2. `SturdyRef`: Durable, cryptographically secure object URI that persists across network drops and process restarts.
  3. `FarRef`: Live, unforgeable capability reference to an object in another vat, supporting asynchronous promise pipelining.
  4. `CapTPSession`: Authenticated, encrypted transport session between two vats managing mutual export/import capability tables.
  5. `Membrane`: Interposition boundary wrapping an object reference to dynamically attenuate, revoke, or trace method invocations.
- **Guaranteed Properties**:
  - Distributed computing is pure object-capability messaging: there is no distinction between "RPC", "HTTP APIs", and "internal calls".
  - Promise pipelining: operations on unresolved results can be dispatched immediately without waiting for network round-trips (`vatA.getWidget().compute().publish()`).
  - Perfect encapsulation: an actor can only interact with the world through the capabilities (object references) it holds explicitly.

#### 2. Why Genuinely Different
Eliminates the impedance mismatch between distributed messaging, leasing, and capability management. A computer, a native microphone helper, a remote leaf VM, and an organizational registry are all identical primitive constructs: persistent *vats* communicating via promise-pipelined CapTP.

#### 3. Organizational Model & Handling of Core Challenges
- **Writer Authority & Realization (Q1)**: A computer is a persistent vat. Writer realization migration (e.g., moving from server to Mac) is vat hibernation, state transfer via the tape, and resumption on the Mac host. Remote clients holding a `SturdyRef` transparently reconnect to the new physical site via CapTP relay handoffs.
- **Choir in Choir & Control (Q6)**: Computer A controls Computer B by holding a `FarRef` to B's controller facet. A sends asynchronous messages (`bRef <- proposeWork(proposal)`). B's vat evaluates the message inside its turn-loop. Neither writes the other's state.
- **Native Capabilities (Q7)**: The Mac microphone is an object exporting a `listen()` method wrapped in a revocable `Membrane`. The owner's policy revokes the capability by breaking the membrane, immediately severing all downstream references without stopping the VM.
- **Lifecycle (Q8)**: The organization is an administrative vat maintaining member directories and shared resource pools. Onboarding passes an organization `SturdyRef` to the employee. Offboarding revokes the member's outer membrane; all corporate object references across the employee's vats turn into dangling, permanently broken promises.

#### 4. Policy Surface Left Open
- Membrane interceptor design (audit logging, rate limiting, data scrubbing, time-bound attenuation).
- Object interface schemas and contract protocols (smart contract escrows, two-phase commits).
- Self-development: Agents can freely construct new actors, compose complex promise pipelines, and wrap untrusted tools in custom membranes without platform-level privilege elevation.

#### 5. Prior Art
- **E Programming Language** (Mark S. Miller et al., capability-based distributed programming)
- **Agoric Swingset & Zoe Platform** (distributed JavaScript vats and smart contracts)
- **Midori Operating System** (Microsoft's async actor-based distributed microkernel)
- **CapTP** (Capability Transport Protocol)
- **Erlang / Elixir OTP** (distributed actors with supervision trees)

#### 6. Sharpest Trade-off, Failure Mode, & Unsafe Configurations
- **Trade-off / Failure Mode**: Memory overhead and distributed cycle collection. Managing distributed export/import tables across thousands of intermittently connected mobile devices creates significant state bloat. If network partitions persist while deep promise pipelines are in flight, unhandled promise rejections can cascade through actor chains, causing state deadlocks.
- **Unsafe Configurations**: Synchronous, high-throughput batch compute. Passing high-frequency numeric tensors, video streams, or bulk database dumps through fine-grained, turn-based object messaging imposes severe performance penalties compared to dedicated streaming data planes.

---

### Comparative Dimensions of the Candidate Kernels

The eight candidate architectures diverge sharply across six fundamental structural axes:

| # | Architecture | Root Locus of Authority | Failure Domain & Partition Stance | Cross-Site Isolation Guarantee | Mechanism vs. Policy Boundary | Offline Sovereignty | Primary Complexity Sink |
|---|---|---|---|---|---|---|---|
| **1** | **Cryptographic Capability Mesh** | Cryptographic Keys & Delegation Tokens | Partition-tolerant (AP); tokens evaluate locally | Cryptographic signature verification | Mechanism: Token verify; Policy: Caveat grammar | High (until token expiry) | Token revocation & accumulator sync |
| **2** | **Bilateral Federated Realms** | Realm Authorities & Domain Gateways | Realm-isolated; partitions sever cross-realm traffic | Network-level border gateway firewalls | Mechanism: Realm relay; Policy: Bilateral treaties | Medium (realm internal only) | Inter-realm treaty & identity configuration |
| **3** | **Distributed Microkernel Namespace** | Per-Process Channel Mount Tables | Channel-dependent; connection drop halts I/O | Namespace invisibility (unmounted = unreachable) | Mechanism: Channel IPC; Policy: Mount composition | Low-Medium (requires local caching file servers) | Synthetic filesystem protocol impedance |
| **4** | **Sovereign Local-First Peer Mesh** | Local Log & Deterministic Predicates | Partition-native; branches grow locally and sync later | Topic-level end-to-end encryption | Mechanism: 3D range sync; Policy: Wasm predicates | Maximum (offline is normal operation) | Asynchronous split-brain reconciliation |
| **5** | **Formally Verified Separation Kernel** | Immutable Policy Bundle & Proof Engine | Fail-closed; unproven actions are halted | Formal mathematical barrier enforcement | Pure decoupling: Mechanism is dumb, Policy is logic | High (if policy allows offline local proofs) | Policy engine performance & proof construction |
| **6** | **Linearizable Consensus Fabric** | Central Consensus Quorum (Raft/BFT) | Consistent (CP); partitions block authority handoff | Epoch-fenced storage coordinators | Mechanism: Consensus terms; Policy: Coordinator rules | Zero (disconnected nodes cannot lease/fence) | Network latency & quorum availability |
| **7** | **Hardware-Attested IFC Enclave** | Silicon Hardware Root-of-Trust (CPU) | Local enclave survives; remote attestation requires net | Hardware-encrypted memory & CPU registers | Mechanism: Hardware enclaves; Policy: Label lattices | High (once enclave image is provisioned) | Silicon attestation & side-channel defense |
| **8** | **Asynchronous Actor-Vat Network** | Encapsulated Vats & CapTP Membranes | Actor-isolated; partitions produce broken promises | Object encapsulation & revocable membranes | Mechanism: CapTP messaging; Policy: Membrane rules | High (vats queue and resolve on reconnect) | Distributed GC & promise pipeline error recovery |
