# The resource fabric: workstations, the server, bursting, Choir in Choir, and the native install (2026-10-31)

Status 2026-10-31: design exploration of the end state. Nothing in it is
built. Mutation class of this document: green. The builds it points at are
orange to red, and each will carry its own ceremony. It builds on
[branching tape](branching-tape-2026-10-31.md) (one writer per branch, a
primary ref, no merge; its doctrine delta §10 still needs owner
ratification) and on [one tape](one-tape-durability-2026-10-31.md) (one
mechanism, the computer bundle, for recovery, backup and sync). It does not
compete with either. Its new proposals are listed in §10, and each needs the
owner.

## The owner's question

Owner, 2026-10-31:

- "So we have the Mac following, but what about the Mac leading? ... if
  there's work happening on the VM on a Mac, how is that then streamed back
  up to the server? ... I need to think about this holistically rather than
  just what we can build now, because I'm going to have to actually use it
  and play around with it and figure out what I actually want running on my
  Mac versus running on the server host."
- "I personally wouldn't want to install someone's app and give it full
  computer use privileges ... I would get a whole separate machine just to
  use with computer use. With the right security posture and audit log, if
  I could really understand what was going on with my computer, I may
  eventually build up enough trust to let it run on my own machine."
- "I'm not concerned with the smallest build. I want to design the system
  with the end in mind ... efficiently utilize the resources at your
  disposal. So if you have Mac, Windows, Linux, desktop computers, you should
  use them. You can run VMs on them if you have the memory and CPU and
  storage. And you're going to want to run computer use on them anyway ...
  But ... you also want to have the server ... to access it from any device.
  And you're going to want the burst capacity to scale beyond the single VM
  at a time. That's both bursting on the host and bursting on workstations
  and combinations. ... I call that Choir in Choir, where one VM controls
  another. ... it's just important to have the big picture design in place
  so we can intelligently back into the next steps."
- "It's not just computer use and file provider. There's other advantages
  of a native app in that we can do voice with on-device models. And
  there's other stuff that we can do with native APIs that I haven't even
  considered yet."

The deployment scenario is a 20-person company. Each employee's workstation
runs a VM that streams into the company's server cluster. The server has
burst compute and runs computers for the organization or a team, such as an
org-wide automatic newspaper, or a corporate comms function. The comms
function takes the Autopaper as a data feed, adds proprietary internal and
external data, and produces email newsletters, internal comms, and social
posts through employees' own accounts, automated by computer use on their
workstations.

## Inputs

What exists, read for this document:

- **Sync slice 1.** The Mac follows its computer's tape read-only:
  `internal/tapefollow`, `internal/bundle`, `cmd/choir-bundle follow`,
  `cmd/desktop/sync.go` and `scripts/e2e-mac-follow-staging.sh`.
- **Bundle authenticity.** Trust sets and one host key per signer
  ([design](bundle-authenticity-2026-10-31.md)).
- **The aarch64 guest image** (`.github/workflows/guest-aarch64.yml`).
- **The `vmbackend` seam** at `origin/recovered/vmbackend-2026-10-14`
  (b6f27ded4). It has Firecracker and Cloud Hypervisor backends. **It has
  no Apple Virtualization backend yet.** The vfkit backend is a
  recommendation in
  [desktop sync and local VMs](../reports/desktop-sync-and-local-vms-2026-10-14.md)
  §3.
- **[Outboard capsules](outboard-capsules-arch-11-12-2026-10-22.md):** a
  leased leaf machine with no identity that returns proposed acts.
- **[Email sign-in](email-sign-in-2026-10-31.md)**, the vsock control plane
  ([NET-03](vsock-control-plane-and-nic-removal-2026-10-22.md)) and the
  restart rules in `CLAUDE.md`.
- **Ontology:** forks, single state authority
  ([standing questions](../standing-questions.md) 5), the doctrine's
  invariant I3 ("parent/child is not a control ontology"), the Autopaper
  object graph and [precommitment records](../precommitment-records.md).
- **The Mike repository** (`~/Dev/mike`, read-only). It already ships
  on-device speech, computer-use helpers for macOS, Windows and Linux,
  workstation VMs, organizations and an audit log. §6 builds on it.

---

## 0. The end state in one page

**Choir runs on a resource fabric.** The fabric's *sites* are every
machine an owner or an organization brings:

- the server hosts;
- every workstation, whether Mac, Windows or Linux;
- the agent's own sandbox machines.

Phones and browsers are *clients* of the fabric, not sites. Every site
advertises what it has:

- memory, CPU and storage;
- accelerators and the local models it can serve;
- what lives only there: logged-in accounts, screens, files, microphones
  and cameras;
- its availability (always on, or a laptop that sleeps);
- its trust class: who operates it, and whose data may run on it.

Work is placed on sites by those attributes.

Five rules hold everywhere:

1. **One writer per computer branch, always a host, never a VM.** A computer
   runs its *writer realization* on exactly one site at a time, which holds
   the writer lease. Any site with a host process can be that site, and the
   Mac becomes one by running a small Choir host. Moving the writer is an
   owner act or an owner policy, fenced first (§2).
2. **Every computer has a home, and the home is always reachable.** The home
   (Node B today, a company's cluster later) holds the lease row, the
   follower copy of the tape and the route. A phone always reaches a computer
   through its home, whichever site runs the writer. The home relays live
   traffic to a workstation writer over the workstation's outbound
   connection, and serves the last home-durable state read-only when the
   workstation is gone (§1.3).
3. **Capacity beyond one VM is leaves and children, never a second
   writer.** Fan-out uses three units, all of which exist in the ontology
   (§3):
   - capsules, inside the guest;
   - outboard capsules: leased leaf VMs with no identity, on any site;
   - child computers: Choir in Choir, each with its own tape (§4).

   Their results enter a computer only through that computer's writer.
4. **The native install is a capability point.** A native app on a
   workstation or phone exposes the OS's capabilities to the computer:
   - computer use and the File Provider;
   - voice with on-device models, and local inference;
   - notifications, intents, share sheets, sensors, keys and background
     work.

   All of these go through one model. The OS grant, the owner's grant in
   Choir and a trust ring all have to allow the capability, and every use
   goes on the tape (§5).
5. **Combination across computers is publication and adoption.** That holds
   between a person's computers, between employees and the organization, and
   between a controller and the computers it controls. Nothing writes
   another computer's tape, and nothing merges.

The sections below design each rule. §8 backs from the end state into the
next steps: the decisions it forces now, the near-term builds it constrains,
and what not to build.

---

## 1. The fabric

### 1.1 Sites, and the record each one publishes

A site is a machine with a Choir host process: `choir-host` on a
workstation, the server stack on a server. It enrolls once (§8.2) and from
then on keeps a **site record** current at its home, signed with its own
host key:

```text
site_record {
  site_id, host_key_id, operator (owner | org | platform),
  os, arch, hypervisor (firecracker | cloud-hypervisor | apple-vz | hyper-v | kvm),
  capacity { cpu_cores, memory_bytes, storage_free, accelerators },
  free_now { cpu, memory, storage }       -- refreshed by heartbeat
  availability (always_on | workstation; awake | asleep | offline; on_battery, metered),
  local_models [ {id, kind (stt | tts | llm | embed | vision), runtime, memory} ],
  capabilities [ native capability ids with their OS grant state ]   -- §5
  trust_class, rings_granted                                         -- §5.3
}
```

The site record is platform control state, owned by the home. It is a
substrate-internal kind with one writer, the home's control plane, and it is
not a computer's data. A computer's *use* of a site is computer data, and
goes on its tape: which leaf ran where, under which lease, with what result.

### 1.2 What is placed, and how

Four kinds of work are placed. Each has its own placement rule:

| Unit | Identity, tape | Placed by | Where it may go |
|---|---|---|---|
| A computer's **writer realization** | the computer's own | the owner (checkout) or the owner's standing policy | any site with a host whose trust class admits the computer's data. Never a site the computer's owner does not trust with its keys |
| A **capsule** | none; the parent's apply | the engineering desk, inside its computer | inside the guest, wherever the writer runs |
| An **outboard capsule (leaf)** | none; a per-lease token | the management desk (spend, admission), enforced by the target site's admission | any site with free capacity whose trust class admits the leaf's taint class and data |
| A **native capability lease** (computer use, voice, a sensor) | none | the desk that needs it, under the owner's capability grant (§5) | only the site where the capability physically is |
| A **child computer** (Choir in Choir) | its own | the controller computer, under a controller grant (§4) | any site, by the same rule as a writer realization |

The deciding attributes, in order:

1. **Trust.** Is this site allowed to see this data and hold these keys? A
   hard filter, never traded against speed.
2. **Locality.** Does the work need something that exists only here: an
   account session, a screen, a file, a microphone? Another hard filter.
3. **Availability.** Will the site be up for the work's expected duration?
   A sleeping laptop is a bad place for a schedule.
4. **Capacity.** Is there memory, CPU, storage and an accelerator for it
   now?
5. **Cost.** Owned hardware that is already paid for is cheaper than
   server burst. Battery and metered networks count as cost.

Placement is a **management-desk decision**, because management owns spend
and admission. Each site enforces its own admission. A workstation refuses a
leaf that would push it past the owner's limits, such as "never more than 4
cores and 8 GB while I am typing; anything when idle on power". A refusal
is an answer, not an error, and the desk places the leaf elsewhere.

**Site access control (owner, 2026-10-31, on proposal 1).** "An organization
with many human team members is in general not going to let employee A's VM
run on employee B's workstation. So we need to have the right level of access
controls in place to make sites work." Trust is therefore not only a property
of the site; it is a relation between the site's principal and the work's
owner, checked by the site's admission before anything else:

- A personal workstation's principal is the person who enrolled it. By default
  it admits only work owned by that person's computers: their writer, their
  leaves, their capability leases. Default deny for everyone else.
- Another person's work, even inside the same organization, is admitted only
  under an explicit grant that names the site, the other person's computer and
  the work class, signed by the site's principal and allowed by the org's
  policy. No org policy alone can place employee A's computer on employee B's
  workstation.
- Org-owned sites (spare machines, the cluster) have the org as principal and
  run org-owned work under the org's RBAC. An org-owned, non-confidential leaf
  may run on an employee's workstation only if that employee's site grants the
  org that work class (§3.2).
- Grants are site-record state, signed, revocable by either signer, and every
  admission and refusal is recorded on the placed computer's tape.

### 1.3 Reachable from any device

A computer is reachable at its permalink whichever site runs it:

```text
phone ──HTTPS──► home (choir.news) ──route──► writer on Node B   (local)
                                     └──────► writer on Alice's Mac: over the Mac host's
                                                 outbound session (the home never dials the Mac)
                                     └──────► writer gone: the home's follower copy, read-only
```

- The workstation host keeps one long-lived **outbound session** to its
  home. It carries several things over the same session:
  - live relayed traffic (the computer's surface, its WebSocket);
  - pushed bundles (§2.3);
  - lease revalidation, heartbeats and site-record updates;
  - capability leases.

  It is the server-side vsock control plane's shape, carried over TLS: the
  peer's identity is the authenticated session, never an address.
- **Latency.** A relayed request crosses the home and the workstation's
  uplink. This is fine for Texture and the surface. It is no worse than a
  model call.
- **Fate-sharing.** The read-only view never depends on the workstation.
  Writes from the phone while the writer is on a sleeping laptop wait in
  the phone's outbox, or open a branch by the owner's choice (§2.5).

### 1.4 Why run on workstations at all: the efficiency argument

A personal computer's guest wants about 6 to 8 GiB of memory (desktop sync
report §3.5). For twenty employees, putting their computers on the server
means:

- 120 to 160 GiB of guest memory;
- the CPU for their capsules;
- all of it bought by the company a second time, when each employee
  already has a 16 to 32 GB machine on the desk that is idle most of the
  day.

On the fabric, the server is sized for two things only: the organization's
own always-on computers, and burst. The cost moves to the workstations'
memory and battery: a 16 GB Mac runs one computer with cloud models, and a
large local model on top is tight. So the owner, or the employee, sets the
workstation's admission limits. The defaults keep the machine pleasant to
use first.

The owner's 2026-10-28 note adds an incentive: compute credits for running
one's own computer on one's own hardware, paid against verifiable
contribution, which means lease-held hours with receipts and not
self-reported uptime. The site record and the lease receipts are that
evidence.

---

## 2. The Mac leading: who writes the tape

### 2.1 The rule

The data system frame's rule 1 is "single writer, fenced". Branching tape
refines it to one writer per *branch*. Neither rule says which machine
holds the lease. A host is a process that does four things:

1. It holds a computer branch's writer lease: the epoch, its host key id
   and the realization id.
2. It runs the append compare-and-swap on the head (the guest's `HeadCAS`).
3. It signs each event head receipt with its own `platform-control` key.
4. It stores events, payloads and pins content-addressed, and serves them
   back for replay and export.

Node B does these with PostgreSQL and its file store. A workstation host
does the same four with a local store, SQLite plus packed objects, because
it serves one owner and a few computers. **A VM never writes the tape on any
site.** The guest proposes appends, and the host admits them. So "a VM runs
on the Mac" and "the Mac leads" are different statements. A leaf VM can run
on the Mac while Node B writes, and the writer has not moved.

### 2.2 Three ways to put a computer's work on a workstation

| Mode | Who writes | On the workstation | For |
|---|---|---|---|
| **E. Execution site** | the home site | leaves and native capability leases (§3, §5) | computer use, voice, local models, idle CPU; the organization; any computer whose writer stays on the server |
| **L. Writer checkout** | the workstation's host | the writer realization: desks, Texture, the guest | a personal computer on its owner's machine: local latency, offline work, keys that never leave the machine |
| **F. Fork** | each computer its own | a new sibling computer | a separate experiment that should stay separate. Rare. A branch is cheaper when the result may become the primary (branching tape §8) |

Mode E and mode L combine. A computer whose writer runs on the Mac (mode L)
still leases leaves on the server for burst (mode E in reverse), and a
computer on the server leases the Mac's microphone. The paper and every
`public_platform` or `critical_protected` computer stay on server sites,
never checked out (branching tape case 8).

### 2.3 Streaming work up: the inverse bundle route

Following today is a pull: the Mac `GET`s `/api/computers/<id>/tape/bundle`.
Leading is a push, over the workstation host's outbound session, because the
home cannot dial a laptop.

```text
workstation host (lease main@e, key K_ws)              home (follower for C)
  guest appends --HeadCAS--> local tape store
  each commit window (group commit), or 2 s, or N events:
    push delta bundle since <home head digest>  ------>  verify: TrustSet{home key, K_ws scoped to C}
      (events, payloads, pins; receipts signed by K_ws)  chain from home head; writer_epoch == lease epoch
                                                         import (chunked, head row advanced last)
                                                 <------ home ack {head_seq, head_digest}, signed
  marks events home-durable up to the ack
```

- **Already built:** the bundle format with delta export (`--since`), the
  verifier with trust sets and per-computer scope (`TrustEntry.computer_id`),
  import that refuses a non-chaining range, and the streaming overlap
  compare.
- **New:**
  - the push route;
  - the workstation's host key in the computer's trust set, enrolled at
    checkout (the set never comes from the bundle);
  - `writer_epoch` and `branch_id` in the signed head receipt (branching
    tape do-now 1);
  - the signed home acknowledgment.

**Two levels of durability.** An event is *local-durable* when the
workstation host commits it, and *home-durable* when the home acknowledges
it. The owner's surface shows the difference, for example "Running on this
Mac; on the server to event N, 2 s ago" or "offline; 340 events only on this
Mac". That difference is the loss window if the workstation's disk dies.

**Irreversible external effects fire only from a home-durable
precommitment.** A precommitment record for an external effect, such as a
mail send, a post, a payment or a spending egress grant, is an ordinary
event. The actuator fires only once that record is home-durable, unless the
owner's policy allows a class offline. It costs one push round trip before a
send. In exchange, the home always holds the record of what the computer did
to the world before the world sees it. Offline, the effect waits in the
outbox as a draft with a send intent. That draft is state, not an obligation
in flight.

**Conflict is a branch, never a merge** (branching tape §5). A push that
does not chain to the home head at its epoch opens a branch or proves an
equivocation. The owner sees both tails, their sizes and their effects
digests, and picks the primary.

### 2.4 Checkout and handback

Checkout is an owner act at the home, or a standing owner policy (§2.5).
The writer must be online. It runs in these steps:

1. **Refusals.** The checkout is refused if the computer is protected or
   always-on, or if an engineering capsule is live or a self-development
   apply is running. These are the restart rules' correctness refusals.
2. **One home transaction.**
   - lease(main).epoch += 1, state `checked_out`, holder = K_ws;
   - K_ws joins the computer's trust set;
   - the old writer appends `writer_handoff` (from, to, target release) as
     its last event.
3. **The old writer stops.** It is refused at its next append (stale epoch)
   and takes a planned stop.
4. **The new writer starts.** The workstation pulls the final delta (the
   follower code it already has), checks that the last event is the
   `writer_handoff` naming it, and boots from its local snapshot plus the
   tail.
5. **The `writer_handoff` event is the planned-restart marker, carried on
   the tape.**
   - The first boot on the new site consumes it once, and open work resumes
     there exactly once.
   - A crash before that is a markerless boot, and it closes the work.
   - This is the restart rule's "one resume per planned restart" across
     sites, with no second marker store.

Handback is the mirror image. The workstation host fences itself, appends
`writer_handoff`, pushes the final range, and keeps the lease until the home
acknowledges. That is the "fence first, then grant" rule. A partition during
a handback costs a short unavailability, not a split. A warm handoff (no
stop) waits for a measured reason to want one. A cold handoff is a snapshot
plus a tail.

### 2.5 Offline, sleep, failover, and the lease timer

- **Offline with the lease.** The workstation keeps working. Desks need
  models; offline that means only local models (§6). The phone sees the
  last home-durable state, read-only. "Work here anyway" opens a host
  branch (branching tape case 1).
- **The lease has no expiry** (branching tape). The 2026-10-28 evidence
  proposed a TTL with self-fencing. A TTL that fences an offline laptop
  kills exactly the work that checkout exists for. Revalidation on every
  wake while online covers the honest case.
- **Automatic failover, reconciled.** The owner, 2026-10-28: with no
  network "it automatically offloads to the cloud for you", working "as a
  fallback, not just when it's intentionally triggered". On the branching
  tape this needs no timer on the writer. When the workstation has been
  unreachable past the owner's policy threshold **and** the computer has
  due standing intents (schedules), the home opens a host branch at the last
  home-durable head and runs those intents there. The lease on main is
  untouched. Nothing the workstation wrote is lost. On reconnect, the owner
  sees two tails and picks the primary. The default primary follows the
  owner's policy, and the switch is offered at once. The failover branch
  carries the effects-digest warning, because its schedules may have sent
  mail.
- **Restarts on a workstation:**

| Event | Under the restart rules | What happens |
|---|---|---|
| App or host crash, kernel panic, power loss | crash | open obligations close as "interrupted by a restart"; standing intents stay |
| Lid closed, sleep | not a restart | the VM pauses; on wake the host revalidates the lease if online |
| App quit, OS update with notice | planned | hibernate with the marker; one resume |
| Battery low | planned | offer a handback while online; otherwise hibernate |

### 2.6 Failure modes of a workstation writer

| # | Failure | Refused or bounded by |
|---|---|---|
| W1 | The home and the workstation both write main | the epoch in each host's append CAS; fence before grant; a push at a stale epoch is refused |
| W2 | The workstation's disk dies with events that never reached the home | the push cadence; shown on the surface; no external effect fired from that window (§2.3) |
| W3 | A forged push | K_ws is trusted for this computer only; a session alone never authorizes a push |
| W4 | K_ws is stolen | removed from the trust set (a compromised key is removed, not cut off); the lease moves to a host branch; the thief's tail stays a non-primary branch |
| W5 | A handoff crashes halfway | before the commit nothing changed; after it, a markerless boot closes the work |
| W6 | The home's lease database is restored from an old backup | epochs are anchored in signed receipts; the home takes the highest epoch it has seen |
| W7 | Release skew between home and workstation | the handoff names the target release; the security floor and the skew refusal stay correctness refusals |
| W8 | Push traffic eats battery or a metered link | deltas of packed objects; backoff on battery and metered networks; bytes on the surface |

---

## 3. Bursting: beyond one VM at a time

### 3.1 The three units of fan-out

| | Capsule | Outboard capsule (leaf) | Child computer |
|---|---|---|---|
| Runs | inside the computer's guest | its own microVM, on any site | its own computer, on any site |
| Identity, tape | none | none, a per-lease token | its own ComputerID, keys and tape |
| Lifetime | one assignment | one lease, one tree | persistent, or until retired |
| Output | an effect bundle the computer applies | proposed acts its desk validates and commits | publications the controller adopts (§4) |
| Use it for | most engineering work | heavy or untrusted leaves, wide research trees, parallel builds, anything over the guest's ceiling | independent long-lived work, a separate trust domain, work that needs its own desks |

The data architecture selection (2026-10-28) already states the rule: "One
computer spanning several VMs would reintroduce multiple writers on one
tape." So a burst never splits a writer. It adds leaves and children, and
**the writer is the fan-in point**.

### 3.2 Burst on the host, on workstations, and on both

- **On the host.** Leaves are cloned from a snapshot (outboard capsules
  §3), with CPU oversubscribed, because agent load spikes and then idles.
  Admission is the host's own, with a per-owner and per-org budget.
- **On workstations.** A computer leases leaves on its own owner's
  workstations first. This is free and trusted: the owner's machine, the
  owner's data. An organization may also place leaves on employees'
  workstations, but only for data the operator of that machine may see. The
  10-28 note is explicit that hosting another person's computer on user
  hardware is a different trust model, because "the host operator sees the
  guest". So org-owned, non-confidential leaves (a newspaper's fetch-and-read
  tree) may run on any org workstation, and personal or confidential leaves
  only on their owner's.
- **Both together.** One research tree of a hundred leaves is placed by §1.2:
  - trust filters the sites;
  - locality pins a few leaves (the one that needs Alice's logged-in
    browser);
  - capacity and cost spread the rest: idle workstations first because they
    are paid for, then host burst.

  The desk sees one pool and one budget. The placement and its receipts go on
  the computer's tape.

### 3.3 Fan-in, and the limit that matters

Every leaf's proposed acts come back to the one writer, which validates them
and commits them. So the writer's append rate is the real ceiling of a
burst. Group commit is built, and acts from many leaves batch into one
append. Two things follow:

- A burst wider than the writer can absorb should be a **child computer**
  that absorbs its own leaves and publishes one result (§4). Fan-in is then
  hierarchical, like a team reporting to a lead.
- Large leaf outputs, such as files, datasets or screenshots, travel as
  content-addressed blobs. The proposed act carries a digest, never the
  bytes.

### 3.4 Failure model of a burst

- **A leaf dies or its site sleeps.** Its lease ends. A leaf lease *has* an
  expiry, unlike a writer lease, because a leaf has no authority and losing
  it loses no state. The desk records "leaf lost on site X" and decides
  whether to place new work. That decision is a fresh act, never an
  automatic re-dispatch of the same obligation.
- **The writer crashes.** Every open leaf lease closes at its boot as
  "interrupted by a restart". Leaves still running are refused when they
  report, because their token names a dead realization. This is the crash
  rule applied to leaves.
- **A leaf lies.** Its acts are proposals. The desk validates them as it
  would any untrusted input, and a leaf on another person's workstation is
  of the taint class of that site.
- **A workstation overcommits.** Its admission refuses new leases, and its
  owner's limits always win over the burst.

---

## 4. Choir in Choir: one computer controlling others

### 4.1 What the docs already say

The phrase has carried three meanings in the repository:

1. **Self-development**: Choir changing its own code through a Choir
   computer ([Texture live supervision](../texture-live-supervision-architecture.md)
   §6). That is capsule-based and stays so. `CLAUDE.md`: "Never use a
   candidate/worker VM as a self-development candidate of its parent;
   self-development candidates are capsules."
2. **Supervising many computers.** "One Texture supervises one task, the
   root Texture supervises all Textures in a computer, and a further level
   supervises many computers" (vision); "'Choir in Choir': one computer
   controlling another" (strategy takeaways, 2026-10-14). It builds on
   forks, fleets, org templates and hierarchical RBAC (station T1b, queued,
   no goal file: residual register ST-04).
3. **The owner's definition, 2026-10-28:** "each VM is its own computer with
   its own desks, the whole shebang". The data architecture selection adds
   that each controlled VM has its own tape and lease, and results return by
   publication and adoption. Heavy work can go to "a cloud capsule or a child
   computer (choir in choir), keeping the lease home".

This section designs meaning 2 on meaning 3, and leaves meaning 1 as it is.

### 4.2 The controller grant

Doctrine I3 says parent and child are not a control ontology. So control
cannot come from the fact that one computer created another. It is an
explicit, durable grant:

```text
controller_grant {
  controller: ComputerID A          controlled: ComputerID B
  granted_by: the owner (or org admin) of B, as an owner act on B's tape
  verbs: [ propose_work, read_published, lifecycle(start | stop | hibernate), place(sites) ]
  scope: kinds and topics B accepts proposals about
  budget: spend and capacity per period, counted on B
  expires / revocable: by B's owner at any time; revocation is an ordinary act
}
```

- **A creates B** by asking the platform to create a computer: a fork from
  A's checkpoint with lineage (a fresh identity), or a new computer from a
  template. The creation itself is the owner's act, or one permitted by a
  standing owner policy. B's genesis cites A as provenance only.
- **A directs B** by *addressed publication*. A publishes a work proposal
  to B, much as the org comms computer does to Alice's computer (§7). The
  proposal enters B through B's Texture (invariant I2a: exogenous input
  enters through Texture). B's management admits it under the grant's scope
  and budget, and B's desks do the work.
- **B reports** by publishing results, which A adopts. A never writes B's
  tape, and B never writes A's. Each records the relationship on its own
  tape: A records the proposal it sent and the results it adopted, and B
  records the grant, the proposals it admitted and what it published.
- **Lifecycle verbs** go to the platform, not to B's guest. A asks B's
  home to start, stop or hibernate B, and the home checks the grant. B's
  restart rules are its own.

So a controller supervises the same way an owner does: through policy,
proposals and published records, not by writing state. That keeps every
computer a single-writer computer, and keeps supervision legible at every
level.

### 4.3 Tape and failure model

- **A crashes.** A's open obligations close as interrupted, including
  "awaiting B's result". B keeps running, because B is a computer that
  stays on. When B later publishes, the result is new input A may adopt.
  Nothing is re-dispatched.
- **B crashes.** B's work closes as interrupted on B's tape. B publishes
  that fate, or A sees no result by the proposal's deadline. A decides
  whether to propose again, as a new act.
- **A runs away.** B's budget and scope bound it, and so does B's owner's
  revocation. A grant cannot be wider than its granter, so a controller
  cannot hand out more than it holds.
- **A cycle or explosion** (A controls B controls A, or exponential
  spawning). Creating a computer is an owner act or policy, with a quota per
  owner and per org. Grants form a directed graph that the home refuses to
  close into a cycle.
- **Kill switch.** The owner stops a whole subtree in one act: revoke A's
  grants, then lifecycle-stop everything A controls. Each stop is a fate
  on that computer's own tape.

### 4.4 What maps, and what is missing

| Need | Today |
|---|---|
| Child creation with lineage | forks (S9 design; branching tape §8 `computer_lineage`) |
| Proposals between computers | publication and adoption exist for the Autopaper object graph; addressed publication does not |
| The controller grant | missing; it is C14's policy shape applied to inbound proposals |
| Hierarchical supervision surface | the root Texture of one computer exists; a Texture over many computers does not |
| Org RBAC over computers | missing (Mike has orgs with admin and member roles, §7.3) |

---

## 5. The native install: one capability model

### 5.1 Why a native app

The browser reaches a computer from anywhere. A native app on a workstation
or a phone adds what only an installed program can do on that machine:

- run the workstation host and VMs (the site, §1);
- operate other apps (computer use);
- appear in the file system (the File Provider);
- listen and speak with on-device models (§6);
- run local models as fabric resources;
- notify;
- take part in system intents and share sheets;
- read sensors;
- hold keys in hardware;
- run in the background.

Further capabilities will come that nobody has named yet. The design must
not need a new mechanism for each one.

### 5.2 The capability model

Every native capability, now or later, is described the same way and plugs
into the same three gates and one audit:

```text
native_capability {
  id: "macos.computer_use" | "macos.file_provider" | "apple.speech.stt" | "apple.fm.llm"
      | "macos.notifications" | "apple.app_intents" | "apple.share" | "macos.camera" | ...
  site: where it physically is
  class: sensor (reads the world) | actuator (changes the world) | resource (computes) | surface (shows or accepts)
  os_grant: the OS permission it needs (TCC service, entitlement, portal), and its current state
  effect_kinds: the tape kinds a use writes (observation, precommit, result)
  irreversible: which of its actions are irreversible (submit, send, purchase, delete)
  data_out: what leaves the device, if anything (audio? text only? nothing?)
}
```

The three gates:

1. **The OS grant.** For example TCC Accessibility, Screen Recording, the
   microphone or the camera; the Windows agent account; a Linux portal
   session. The OS holds it, and the native app reports its state in the
   site record. Choir never works around an OS gate.
2. **The owner's grant in Choir.** This is a scoped, revocable owner act on
   the computer's tape: which computer may lease this capability on this
   site, with which scopes (apps, origins, accounts, folders), action
   classes and budget.
3. **The ring** (§5.3). This sets how much of the capability is trusted
   now.

The audit: every use writes the capability's effect kinds on the
computer's tape. A sensor writes an *observation* record that names what was
captured, with the captured bytes as a blob only when the scope keeps them.
An actuator writes a *precommit, act, resolve* triple. A resource writes a
*resource use* record: model, tokens and site. Texture renders all of it as
one owner-readable timeline. Nothing about a native capability lives only
in a log on the device.

The native app ships with a manifest of the capabilities it carries, the
same way an app package declares its kinds. A new OS API therefore arrives
as one more manifest entry with its gates, kinds and ring, and not as a new
subsystem.

### 5.3 Rings of trust

Trust grows ring by ring, as with progressive delivery. Every capability has
rings, and computer use is the strictest case:

| Ring | Computer use runs | Voice, sensors | Reaches |
|---|---|---|---|
| R0 | a separate machine (a spare Mac), a macOS VM, or the agent's own sandbox desktop | push to talk; nothing kept | only accounts deliberately signed in there |
| R1 | a separate OS user account on the owner's machine | hands-free in the app; transcripts kept, audio not | that user's apps, keychain and sessions |
| R2 | the owner's own session, scoped to named apps | wake word, background | the named apps |

- **Widening is an owner act, offered on evidence.** The evidence is:
  - a stated number of uses with no policy violation;
  - the precommitment records' calibration on that capability (did the
    actuator predict the screen it got?);
  - an audit the owner actually read.
- **Narrowing is immediate**, and needs no evidence.

### 5.4 Computer use: posture and what the platforms do now

**The platforms (researched 2026-10-31).**

- **Full Disk Access (macOS).** On 2026-10-02 Apple announced controls so
  that Full Disk Access can be granted "only with very explicit user
  action". It cited AI agents: "As AI agents become increasingly capable and
  autonomous, the risks associated with this level of access will grow
  substantially." Apple gave no version or date
  ([MacRumors, 2026-10-02](https://www.macrumors.com/2026/10/02/apple-announces-macos-full-disk-access-changes/);
  [gHacks, 2026-10-05](https://www.ghacks.net/2026/10/05/apple-will-require-explicit-user-action-to-grant-full-disk-access-in-macos-citing-ai-agents/)).
  Choir should never ask for Full Disk Access. The computer's files are its
  own.
- **Screen recording (macOS).** Since macOS 15 the system re-prompts for
  screen recording monthly, including for ScreenCaptureKit apps. The
  `persistent-content-capture` entitlement, for remote-desktop apps, is
  granted only on request
  ([9to5Mac, 2024-08-14](https://9to5mac.com/2024/08/14/macos-sequoia-screen-recording-prompt-monthly/);
  [TidBITS, 2024-08-19](https://tidbits.com/2024/08/19/apple-reduces-excessive-sequoia-permission-requests-shifts-to-monthly);
  [Michael Tsai, 2024-08-08](https://mjtsai.com/blog/2024/08/08/sequoia-screen-recording-prompts-and-the-persistent-content-capture-entitlement/)).
  A Jamf note reports that the pane's TCC tooling changed again in macOS
  27.2
  ([Jamf setup-checklist v1.1.1beta](https://github.com/Jamf-Concepts/setup-checklist/releases/tag/v1.1.1beta)).
  The monthly prompt is a consent renewal the owner sees, and we keep it.
- **Accessibility and Apple Events (macOS).** These remain user-granted TCC
  permissions. MDM profiles (PPPC) can pre-approve Accessibility on managed
  Macs. They can only let a standard user approve Screen Recording
  (unverified against Apple's current MDM docs).
  - A grant follows the app's signing identity. Mike's helper keeps the
    same bundle id and signing identity between builds "so macOS keeps its
    permission grants after a rebuild" (`computer-helper/README.md`).
  - So stable grants need a stable identity. Distribution needs Developer
    ID and notarization: the paid Apple program.
- **Sanctioned intents (macOS).** macOS 26.1 betas carried code for MCP in
  App Intents
  ([9to5Mac, 2025-09-22](https://9to5mac.com/2025/09/22/macos-tahoe-26-1-beta-1-mcp-integration/)).
  We found no confirmation that it shipped. Where an app exposes intents,
  they are narrower and safer than pixels.
- **Windows.** Copilot Actions (preview, October 2025) runs each agent
  under its own standard *agent account* in a separate session. It reaches
  only known folders unless granted more, and the user can watch and take
  over
  ([Windows Experience Blog, 2025-10-16](https://blogs.windows.com/windowsexperience/2025/10/16/securing-ai-agents-on-windows/);
  [Microsoft Learn](https://learn.microsoft.com/en-au/windows/security/book/operating-system-agentic-security)).
  We found no general-availability notice.
- **Linux.** Wayland allows input and capture only through the
  xdg-desktop-portal ScreenCast and RemoteDesktop portals and libei, with
  per-session consent. X11 has no such limits.

The direction is the same on all three systems: broad, silent and permanent
grants are closing, and agents get their own account, session or explicit
consent. Rings R0 and R1 are that posture.

**What Mike already built**, which carries over almost as it is:

- `computer-helper/`, a Swift menu-bar helper for macOS;
- `computer-helper-rs/`, Rust crates for Windows (UI Automation, SendInput),
  X11 (AT-SPI, XTest) and Wayland (AT-SPI, portal, libei);
- a sandbox container: the agent's own desktop, watched through view-only
  noVNC, which is an R0;
- one wire protocol with shared fixtures.

The helper's safety rails are what §5.3 needs:

- allowed and blocked apps, with Keychain Access, System Settings,
  Terminal and Passwords blocked by default;
- refusal while a secure input field has focus;
- Stop now and Pause;
- **takeover detection**: every posted event is tagged, so any untagged
  click or key is the person, and the batch stops;
- per-batch limits, and refusal of stale frames.

**What Choir adds, and what is missing in Mike.** Mike's helper "is a
prototype without per-action approval". Its audit log is a database table of
activity events, and its device token is polled.

- **Actions on the tape.** In Choir each action is a precommit, act, resolve
  triple on the computer's tape, with the screenshot and accessibility tree
  as blobs.
- **Submit goes through a policy.** The irreversible *submit* class (send,
  post, buy, delete, accept) runs only under a C14 policy, with the owner as
  a required seat in R0 and R1.
- **Leases replace the polled token.** The helper takes capability leases
  over the site's outbound session.
- **The kill switch reaches every site.** It revokes every lease from any
  device, including the phone.
- **Screens are untrusted input.** Content read from a screen is in the
  taint class of fetched web content. An instruction found on a screen never
  widens a lease.

### 5.5 The File Provider

The computer's files root is the single authority, and the folder is a
replica (desktop sync report §2). The File Provider needs an app extension
with an App Group, and in practice the paid Apple program. FileProvider
Testing Mode is development-only
([Apple, supported capabilities](https://developer.apple.com/help/account/reference/supported-capabilities-macos)).
Windows uses the Cloud Files API, and Linux a plain synced folder or FUSE.
In the capability model the File Provider is a *surface*: its uses are file
acts the computer already records.

### 5.6 Other native APIs worth having

Each row below is a manifest entry under §5.2. The ring is where it would
start.

| Capability | macOS / iOS | Windows / Linux | What it gives a computer | Class, start ring |
|---|---|---|---|---|
| Notifications with actions | UserNotifications | toast / libnotify | approvals and "your computer needs you" without opening the app; the reply is an owner act | surface, R1 |
| Intents and Shortcuts | App Intents, Shortcuts, Spotlight indexing | (Windows: app actions, unverified) | the computer's verbs ("ask my computer", "add to my Texture") callable from the system; its documents findable in Spotlight | surface, R1 |
| Share extension | Share sheet | Share target / portal | exogenous input enters Texture (I2a) from any app | surface, R1 |
| Microphone, speaker | AVAudioEngine with voice processing | WASAPI / PipeWire | voice (§6) | sensor and actuator, R0 |
| Camera, document scan | AVFoundation, VisionKit | Media Foundation / PipeWire | photographed receipts and whiteboards as files and objects | sensor, R1 |
| On-device vision, OCR | Vision | Windows AI APIs / Tesseract | reading what the computer is shown, locally | resource, R1 |
| Location | Core Location | Windows location / GeoClue | context for schedules and research; never stored unless scoped | sensor, R1 |
| Calendar, contacts, reminders | EventKit, Contacts | (per app) | imports into the Calendar and Mail app agents' kinds | sensor, R1 |
| Keys and passkeys | Keychain, Secure Enclave, passkeys | TPM / Windows Hello; libsecret | the site's host key, the computer's keys while it serves, owner approval by passkey | resource, R0 |
| Background work | launchd agent, BGTaskScheduler (iOS) | service / systemd user unit | the workstation host runs without the window | resource, R0 |
| Focus and presence | Focus filters, idle state | presence APIs | admission limits ("not while I am typing"); quieter notifications | sensor, R1 |
| Continuity | Handoff, Universal Clipboard | (none) | move the open Texture between Mac and phone | surface, R2 |
| Health, motion (phone) | HealthKit, Core Motion | (none) | owner-chosen context only; never for an org | sensor, R2 |

---

## 6. Voice and local models

### 6.1 What Mike already runs

Mike (`~/Dev/mike`) has built most of on-device voice. Read 2026-10-31:
`docs/voice-chat.md`, `docs/fluid-speech-mini-iphone.md`,
`docs/phonon-read-aloud.md`, `docs/android-speech.md`, `docs/speech.md`,
`docs/mac-app.md`, and `mobile/plugins/speech/`.

- **Speech in.** Fluid Speech Mini (a distilled Parakeet TDT 0.6B v3)
  runs in Core ML on the Neural Engine through FluidAudio. It takes about
  280 MB per 15 s window and gives live partials through a sliding window.
  Apple's SpeechAnalyzer (iOS 26) and SFSpeechRecognizer are fallbacks. On
  Android the same model runs as ONNX on the CPU, and in the browser as ONNX
  on WebGPU.
- **Speech out.** Phonon 31m (Kyutai Pocket TTS lineage) uses about 95 MB,
  streams 80 ms chunks, and runs in Core ML on iOS, as GGUF through a C
  library on Android, and on WebGPU in the browser. Kokoro and Apple's
  voices remain.
- **Hands-free voice.** This is designed and built natively, and the owner
  uses it on device, with dictation and read-aloud, in daily use (owner,
  2026-10-31: "I am using all of the voice modes on device"; Mike's docs
  still say simulator only and are behind). The pieces:
  - one audio engine with Apple's voice processing, so echo cancellation
    hears the assistant;
  - Parakeet realtime EOU for live text and the end of an utterance;
  - Silero VAD;
  - Smart Turn v3.2 to confirm the end of a turn;
  - a final Fluid Speech Mini pass;
  - clause-by-clause Phonon with a played-sample clock;
  - barge-in within tens of milliseconds;
  - speaker verification (WeSpeaker, CAM++).

  It uses about 700 MB of models in all.
- **The split by time.** Anything that must react inside an audio frame
  is native. That covers turn ends, barge-in and the play clock. The
  conversation (the chat turn, speakable text, what was heard) lives in
  shared web code.
- **The Mac.** The Mac app is the iPhone app built with Mac Catalyst, and
  it runs dictation, read-aloud and the voice-mode models on the Mac.
- **The server.** It offers OpenAI-compatible `/audio/transcriptions` and
  `/audio/speech` endpoints, and stores neither audio nor text. A personal
  vocabulary corrects names in every transcript.
- **Local text models.** These are declared endpoints (Ollama, any
  OpenAI-compatible URL) tagged `location: local`. Mike does not use
  Apple's Foundation Models framework or MLX.

### 6.2 What carries over to Choir

- **Engine and contract.** The native voice engine and its contract with
  the conversation layer, as they are. In Choir the conversation layer talks
  to Texture: a spoken turn becomes an owner revision, and the answer that
  is read aloud is Texture's revision. It does not talk to a chat thread.
- **The on-device models.** The models and their runtimes, with their
  licences and pins (Fluid Speech Mini is CC BY 4.0 and needs credit; Phonon
  31m is Apache-2.0).
- **The two-tier speech API.** The server endpoints become a fallback
  resource for sites without a capable device.
- **The personal vocabulary**, which becomes a kind on the owner's
  computer.
- **The Catalyst decision**, which in Choir means one native codebase for
  iPhone and Mac. A choice to make: the Choir Mac app is Wails today. The
  workstation host must not live inside the app's process in either case
  (§8.3).

### 6.3 What is missing

- **Voice on the tape.** In Choir a spoken turn is an owner act whose text
  goes on the tape. Whether audio is kept is a scope of the grant, and the
  default is no.
- **Measured device numbers.** All voice modes run on device in the owner's
  daily use; what is missing is recorded measurements (latency, battery,
  memory, false turn ends) that placement in the fabric can use, and Mike's
  docs catching up.
- **Voice as an owner seat.** An approval spoken aloud must be bound to the
  owner: speaker verification exists, but a spoken "yes" is not a passkey,
  so irreversible classes still need a stronger seat.
- **Windows and Linux voice.** On Windows the Windows AI APIs and Foundry
  Local give on-device models, and Windows ML runs ONNX models across CPU,
  GPU and NPU
  ([Microsoft Foundry on Windows](https://developer.microsoft.com/windows/ai/)).
  So Fluid Speech Mini's ONNX build and Phonon's GGUF build port as they did
  to Android. Linux has the same ONNX and GGUF runtimes on the CPU or GPU.
- **On-device text models as a resource.** Apple's Foundation Models
  framework gives apps the on-device model behind Apple Intelligence (macOS
  26, [MacRumors, 2025-06-09](https://www.macrumors.com/2025/06/09/foundation-models-framework/)).
  At WWDC 2026 it became one Swift API for on-device and server models,
  with image input, custom skills, an `fm` command-line tool and a Python
  SDK on macOS 27, and an announced open-source release
  ([MacRumors, 2026-06-08](https://macrumors.com/2026/06/08/apple-unveils-xcode-and-models-improvements)).
  The M3 and 12 GB requirement for its advanced features is reported by
  secondary sources only. MLX and llama.cpp serve open models on Apple
  silicon, and Core ML runs converted models. None of these is wired into
  Choir's gateway. That gateway lacks a generic OpenAI-compatible provider
  (desktop sync report §3.4).

### 6.4 Local models in the fabric

A local model is a resource of a site, listed in its site record with its
kind, runtime and memory. It is placed like CPU and memory (§1.2), with two
extra attributes:

- **Data out.** A local model sends nothing off the device. So it is the
  default for:
  - speech;
  - redaction before anything leaves;
  - embeddings of private material;
  - summaries the owner marked private.

  For a desk whose data the owner scoped as "never leaves this Mac", the
  local model is the only admissible one, and the desk must say when it is
  not good enough.
- **Quality class.** Small local models fail desk work long before they fail
  chat (desktop sync report §3.4, a hypothesis). The gateway routes each
  call by a declared quality floor. A desk turn uses frontier models unless
  the owner allows local. Speech, redaction, embeddings and classification
  use local first.

The gateway on each site is the single router. On the server it holds the
provider keys. On a workstation it routes local-model calls to the site's
runtime (Foundation Models, MLX, Core ML, ONNX, llama.cpp), and routes
cloud calls either through the home's gateway (the keys stay on the server)
or, when the owner's policy says so, with keys held in the workstation's
Keychain. Every call writes a resource-use record: model, site, tokens and
whether data left the device.

---

## 7. The organization

### 7.1 The shape

```text
                 the company's cluster: home for every company computer and site
  ┌──────────────────────────────────────────────────────────────────────────┐
  │ org computers (server sites, never checked out): org paper, comms, teams │
  │ the org graph (RBAC scope)      burst leaves (host)      gateway, mail   │
  │ homes + follower copies of every employee computer; site records; leases │
  └──────▲───────────────────────────▲───────────────────────────▲───────────┘
         │ outbound session          │                           │
   Alice's workstation          Bob's workstation           spare Mac (R0 sandbox)
   Alice's computer: writer     Bob's computer: writer on   org-owned leaves, the comms
   here (mode L); actuator R1;  the server (mode E); his    team's computer-use sandbox
   voice; idle cores for leaves  Mac lends voice and leaves
```

What in today's ontology maps onto it:

| Scenario piece | Concept |
|---|---|
| An employee's computer | a persistent user computer owned by the employee; writer on the workstation or the server, by choice |
| Org paper, comms, team computers | always-on server computers, like the paper; never checked out, never branched |
| Shared org data | the doctrine's third scope: "an organization sharing one graph under RBAC is a third scope, not a second model" |
| The Autopaper feed | adoption from the Autopaper object graph |
| Burst | leaves on host and workstations (§3), with the org's trust rules |
| An org computer coordinating team computers | Choir in Choir with controller grants (§4) |
| Who may act as Alice | Alice's computer, under Alice's policy |

### 7.2 Corporate comms, end to end

1. **Data in.** The comms computer adopts relevant objects from the
   Autopaper object graph. It reads internal objects that employees'
   computers and connectors *published* to the org graph: product notes,
   launch dates, quotes cleared for use. Its research desk reads the world,
   read-only. Nothing reaches into an employee's computer.
2. **Drafts.** The comms computer's Texture writes the newsletter, the
   internal note and the post drafts. The comms lead is a seat in its
   review policy.
3. **The newsletter and internal comms** go out through the company's mail
   transport, a host service, from the comms computer and as the company,
   under its own effect policy. Each send is a precommitment record with a
   consequence receipt.
4. **The post as Alice.** The comms computer publishes a post proposal
   *addressed* to Alice's computer: the text, the image refs, the reason and
   a deadline. That is all it can do. It holds no credential of Alice's and
   no lease on her workstation.
5. **Alice's computer adopts the proposal.** Its Texture shows it to Alice.
   Her policy for "posts under my name" decides. At first she is a required
   seat. Later she may grant a standing policy: "posts the comms lead
   approved, on these topics, go out without asking; tell me after".
6. **The act.** Alice's computer leases its computer-use capability on her
   workstation, in R1 (her separate OS user, signed in to the social
   network for this). The scope is "this site, account Alice, one post". The
   submit is precommitted, performed, and resolved with a screenshot and the
   post's URL.
7. **Receipts.** The consequence receipt is on Alice's tape. Alice's
   computer publishes a "posted" receipt object (URL, time, the proposal it
   answers) to the org graph, and the comms computer adopts it.

**Who signs what.**

- Alice's computer signs the post, under Alice's policy.
- The comms computer signs the proposal.
- The company signs the newsletter.

No computer acts in another's name. "Why did this go out from my account"
is answered on Alice's own tape. The org sees only what Alice's computer
publishes.

**Failure modes.**

- A comms computer that tries to post directly has nothing to post with.
- A forged proposal is still only a proposal that Alice's policy reads.
  RBAC on the graph names who may address proposals to whom.
- A screen with an injected instruction cannot widen the lease.
- Alice offline means the proposal waits.
- A crash mid-post closes the job as interrupted. The screenshots show
  whether the submit happened, and a retry is a new act.

### 7.3 What is missing

| Missing | Note | Class |
|---|---|---|
| An organization principal (org-owned computers, admins, roles) | Mike has orgs with admin and member roles, invitation-only membership with the recipient's consent, and per-item overrides; a model to borrow | red (auth) |
| The org graph scope with RBAC | named in doctrine, unbuilt | red |
| Addressed publication | shared with Choir in Choir (§4) | orange |
| Effect policies that admit another computer's proposals | C14 is designed; effect policies are not live | red |
| Site enrollment and org-managed trust sets | today trust is files an operator copies | red |
| A home that is not Node B | branching tape open question 3 | red |
| Org-wide kill switch, offboarding | offboarding needs an owner decision first (§10) | — |
| MDM guidance for the fleet's TCC grants | deployment documentation, no Choir code | green |

---

## 8. Backing into next steps

### 8.1 Decisions the end state forces now

These cost little to decide today. Each gets more expensive with every
build that assumes otherwise.

1. **The writer is a host on a site, and the VM is never the writer.** Every
   design of "running on the Mac" starts from a workstation host, not from a
   VM with special powers.
2. **A writer lease has no expiry. A leaf lease does.** Failover is a
   branch, not a self-fence (§2.5). This resolves the 10-28 TTL proposal
   against branching tape. It needs ratification with branching tape §10.
3. **`writer_epoch` and `branch_id` go into the signed head receipt now**
   (branching tape do-now 1). Every receipt signed before that cannot say
   who wrote it.
4. **Every site has its own host key, and trust sets are per computer.**
   Already the bundle-authenticity shape: keep it, and never let a session
   cookie stand in for a site's identity.
5. **A home per computer, separate from where the writer runs.** Routing
   reads the home and the lease, never a realization's claim.
6. **Every native capability goes through one manifest, three gates and the
   tape** (§5.2). Computer use is the first entry, not a special case.
7. **Combination is publication and adoption, including control.** The
   controller grant (§4.2) replaces any idea of one computer writing
   another.

### 8.2 Near-term builds the end state constrains

**Sync slice 1 (just landed).**

- The follower's local store (`internal/tapefollow`) is the start of the
  workstation host's tape store. It should grow an append path behind the
  lease. Do not add a second local store format beside it.
- `GET .../tape/bundle` is the first half of the have/want protocol. The
  push route is its inverse on the same bundle format. Bundle manifests
  gain `branch_id` and `base` before the first branch.
- The follower authenticates with the owner's session cookies. A writer
  must authenticate as a site, with its host key, so the follower should
  move onto the site credential when the host exists.
- The menu line ("Synced to event N") is the start of the durability
  display (§2.3). Keep its states typed (`tapefollow.Status`) so they can
  grow to local-durable and home-durable.

**The Apple Virtualization backend.**

- Build it on the `vmbackend` seam (Backend, Caps, PlanLaunch) as a peer of
  Firecracker and Cloud Hypervisor, with `Caps{Snapshot:false}` first. Do
  not build a desktop-only launcher. The site record's `hypervisor` field
  and placement depend on one interface.
- Run it as a subprocess per guest (vfkit, or a tiny helper), never inside
  the app's process, so an app crash does not take every computer down.
- Use the same guest image layering per architecture. The aarch64 guest
  today keeps the full base (`toolLayers false`). The fabric wants the same
  base plus tool layers on every site, so a leaf's tool profile means the
  same thing on a Mac and on a server.
- Use the vsock control plane (NET-03) for guest identity on every
  hypervisor. Identity must come from the endpoint, never from a tap
  address, and Apple's NAT has none to give.
- Boot from an imported bundle (snapshot plus tail), which is the same
  recovery path as the server.

**Email sign-in.**

- An email session is a *person's* session, and it is weaker. The design
  already keeps it from minting API keys or unwrapping the data root (T5).
  **Site enrollment and checkout must need a passkey session**, because a
  site key can write the owner's tape. An email session may follow (read)
  and use the surface.
- The Mac app's automated tests sign in by email for the surface and the
  follower. Tests that enroll a site use a pre-minted test credential from
  a passkey session, as the design already does for keys.
- The session carries its method (`auth_method`). Capability grants (§5.2)
  should read it too: an email session can see and revoke, not widen.

**The Mac app itself.** Whether it stays on Wails or follows Mike to Mac
Catalyst (one codebase with iPhone, where the voice engine already runs) is
an open choice. Either way:

- The workstation host is a separate background process (a launchd agent),
  not the window. The fabric needs it running when the window is closed.
- Native capabilities are separate signed helpers with stable identities
  (computer use, the File Provider extension), so each OS grant attaches to
  a small, auditable binary.

### 8.3 What not to build, because it would not deform into the end state

- **A TTL lease with self-fencing.** It kills offline work. The branch
  replaces it.
- **A VM that writes its own tape, or a Mac-only local auth and account
  system.** These are second authorities.
- **VMs inside the app's process** (in-process Virtualization.framework in
  the window app). They fate-share every computer with the UI.
- **A computer spanning several VMs, or a leaf with its own tape.** Either
  is multiple writers. Use leaves or child computers.
- **One computer writing another's tape, or "merge" in any form**,
  including in Choir in Choir.
- **A computer-use helper with its own audit log**, outside the tape, or one
  that runs in the owner's main session by default. Mike's helper, carried
  over, gets the tape and the rings, and is not copied as it is.
- **Full Disk Access, or any blanket grant.** The platforms are closing
  them, and the capability model does not need them.
- **Per-platform sync engines.** Use one engine (`internal/filesync`) with
  thin adapters for the File Provider, Cloud Files and a synced folder.
- **A scheduler outside the management desk**, or a site that places work
  on itself without admission.
- **Placing another person's confidential data on a workstation** they do
  not operate. That trust model is out of scope until confidential VMs make
  it safe.
- **Desktop-local copies of computer data** (a second home, I27). Device
  state is the named exception: drafts, caches and UI choices.

### 8.4 Where the owner can start to learn what they want

These come before most of the builds, and need almost nothing new:

- **Computer use in R0, now.** Set up a spare Mac, or a macOS VM, signed in
  to test accounts only. Run Mike's helper or another agent there for real
  chores for a week. Note:
  - what you wanted it to do;
  - what you wanted to see afterwards;
  - when you reached for Stop.

  The notes set the audit view, the default ring and the submit policy.
- **Voice through Mike's engine.** Use dictation and read-aloud on the
  phone and the Mac, to learn which voice loop you want against a computer
  rather than a chat.
- **Following.** Keep following your own computer on the Mac (slice 1),
  and watch how stale the copy feels. That is the home-durability window in
  reverse.
- **The first leading run.** It needs the workstation host, the lease in the
  receipt, the push route and the Apple Virtualization backend. Run it on a
  test computer first, where drills are allowed, then on your personal
  computer. The proof is a round trip with equal head digests, a crash that
  closes work, and a stale-epoch push refused.

---

## 9. Standing questions, for the fabric

1. **Who settled this?**
   - Owner: single writer, fenced (2026-10-30/31); workstation mode is key;
     Choir in Choir means each VM is its own computer (2026-10-28);
     automatic offload as a fallback (2026-10-28); this end-state framing
     (2026-10-31).
   - Proposed, needing ratification: branches and switching primary
     (branching tape §10).
   - Proposals of this document: everything in §10.
2. **Conformance.** No merge. Forks are separate computers. Capsules and
   self-development are unchanged. The paper never checks out. Restart rules
   are unchanged and extended across sites by the handoff event.
5. **Single authority.**
   - The tape is the authority for state.
   - The lease row at the home decides who writes.
   - Site records decide what a site offers.
   - Grants decide what a capability or a controller may do.
   - Every copy elsewhere is a replica.
6. **Artifact.** Equal head digests across sites, verified under the trust
   set. A capability's use is proved by its triple on the tape.
7. **Fate-sharing.** Reads go through the home's follower copy, never
   through the workstation. A computer never depends on the app's window.
8. **Restart.** Sections 2.5 and 3.4.
9. **No SSH.** Checkout, handback, enrollment, grants, revocation and the
   kill switch are all owner acts on the product API, reachable from the
   phone.

## 10. New proposals in this document (each needs the owner)

1. The fabric model: sites with signed site records, placement by trust,
   locality, availability, capacity and cost, owned by the management desk
   and enforced by each site's admission.
   **Accepted** (owner, 2026-10-31), with site access control as a
   condition: default deny across people, explicit signed grants (§1.2).
2. A home per computer, with routing through the home and the workstation's
   outbound session.
3. The writer lease has no expiry. Failover is an automatic host branch that
   runs only standing intents. Leaf leases expire.
4. `writer_handoff` on the tape is the planned-restart marker across sites.
5. Irreversible external effects fire only from a home-durable
   precommitment, unless the owner's policy allows a class offline.
6. Choir in Choir by controller grant and addressed publication, never a
   parent's write. Creating a computer is an owner act or policy, with
   quotas and no grant cycles.
7. One native capability model: a manifest, three gates (OS, owner, ring),
   and the tape as the only audit. Rings R0 to R2 for every capability.
8. Site enrollment and checkout need a passkey session; an email session may
   follow and revoke, never widen.
9. Offboarding default: the employee's computer goes with the employee; the
   org keeps what was published to the org graph; nothing is seized
   silently.
10. Voice: Mike's native engine and on-device models carry over; a spoken
    turn is an owner revision in Texture; audio is not kept by default.

## 11. Lineage

- [branching tape](branching-tape-2026-10-31.md): one writer per branch,
  primary ref, switch, no merge.
- [one tape](one-tape-durability-2026-10-31.md) §4, §4a, §5, S7, S8: the
  bundle, the Mac as the first off-host place, the cold move.
- [bundle authenticity](bundle-authenticity-2026-10-31.md): trust sets and
  per-host keys.
- [object graph redesign](object-graph-redesign-2026-10-30.md) §11: the tape
  protocol is a replication protocol; keys in the Keychain.
- [data system frame](choir-data-system-frame-2026-10-30.md): single writer,
  fenced.
- [data architecture selection](../definitions/choir-data-architecture-selection-2026-10-28.md):
  the owner notes on Choir in Choir, oversubscription, the desktop as the
  main host, credits and fallback offload.
- [outboard capsules](outboard-capsules-arch-11-12-2026-10-22.md): leased
  leaves.
- [vsock control plane](vsock-control-plane-and-nic-removal-2026-10-22.md):
  identity from the endpoint.
- [email sign-in](email-sign-in-2026-10-31.md): T5, the method on the
  session.
- [desktop sync and local VMs](../reports/desktop-sync-and-local-vms-2026-10-14.md):
  vfkit, the File Provider, the resource budget.
- [CPU oversubscription](../evidence/cpu-oversubscription-2026-10-28.md)
  §5.1-5.3: the TTL proposal this document declines.
- [Texture live supervision](../texture-live-supervision-architecture.md)
  §6 and the vision: the other meanings of Choir in Choir.
- Mike (`~/Dev/mike`, external): `docs/voice-chat.md`,
  `docs/fluid-speech-mini-iphone.md`, `docs/phonon-read-aloud.md`,
  `docs/android-speech.md`, `docs/mac-app.md`, `docs/computer-use.md`,
  `computer-helper/README.md`, `computer-helper-rs/README.md`,
  `infra/workstation/README.md`, `docs/organizations.md`,
  `docs/audit-log.md`.
