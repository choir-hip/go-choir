# Roadmap re-cut: the remaining first-pass checklist against the roadmap (2026-10-23)

Status: **DRAFT; proposal for the owner to drive in the morning.** Nothing here is
decided. Every call below is a default the owner can overrule, and section 7
lists the calls that are his alone. Mutation class: green (docs only). This
is not a `/goal` file, and it does not change the stations of the
[first-pass metamission](choir-launch-first-pass-metamission-2026-10-21.md).
Item facts come from `/Users/wiz/Dev/lanes/_runs/progress/items.json`
(as of 2026-10-23T04:06Z) and the repo at `2ce86f869`.

## 0. What the owner said

Owner, 2026-10-23, about 04:30 to 04:50 UTC:

- "self-dev on my computer is a pre-req for strangers. they need to be able to do self-dev too."
- "my roadmap is autoputer -> autopaper -> autoradio -> launch."
- "autoputer and autopaper arent that cool on their own. its only with audio that they become really cool." The voice pipeline is being built in `~/mike`. The sequence: Autoputer (stable, scalable, secure, self-dev), then Autopaper ingestion scales up and content quality is tuned, then the voice layer for Autoradio, then a soft ship of the web app, then the iOS and Android apps.
- "we can move faster than you think. the current ci bottleneck is annoying".
- "we need to have more focus on precommitment records/psa ... thats the key bridge from autoputer and self-dev to autoradio. and its the essential core scientific idea of choir".
- "the storage and memory leaks and security holes and other platform problems... they are very important, as they are show-stoppers for our real innovation. so, its important that we do this groundwork before going back high level into ideas".
- "i need to take more of a drivers seat on the agenda tomorrow."

Earlier that night the owner asked "is all of this really needed as
foundation?". This document answers by giving each open item one
milestone.

**How this changes the first pass.** The first pass treated the whole
security and density checklist as one list for launch (SDC DEC-04: "every
security and density item that can land before launch does"). This
re-cut keeps every item. Each item goes to the milestone that first needs
it. "Launch" now means the soft ship to strangers.

**The rule this proposal follows.** The platform groundwork comes first
and is not cut for speed. That means storage growth, memory return, leaks
and known security holes. When an item was close to the line, it went into
the Autoputer exit. Speed comes from doing more at once and from a shorter
landing cycle, never from leaving groundwork out.

## 1. The Autoputer exit

The Autoputer exit is done when four properties hold on staging. Section
4 turns them into checks.

### Stable

- The paper and the owner's computer survive a planned deploy refresh, a
  crash boot and a host reboot, and the restart rules in `CLAUDE.md` still
  hold. A crash closes open work with its fate. A planned restart resumes
  once. Source schedules are re-armed at boot (`recoverSourceSchedulesAtBoot`).
- `/health` stays up under the paper's real write load and the online GC.
  Today the GC runs every 10 to 20 minutes on a journal of about 1.6 GiB
  and stalls the store for 20 to 30 seconds each time
  (`docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md`).
  The D4 holder log names a full scan of the actor wake outbox (OPS-01 in
  items.json; `docs/problems/paper-post-refresh-health-windows-2026-10-22.md`).
- A runaway desk worker, capsule or Go build is stopped by its own budget,
  and the daemon stays alive (RES-01 to RES-04).
- Storage does not grow without bound. Node B's free space holds steady
  over a day. btrfs metadata, now 82 to 84% used (RES-22), has headroom.

### Scalable

The target is friends and family, plus Autopaper's ingestion scale-up.
The proposed size is about 25 friends' computers registered, 5 of them
busy at once, beside the paper and the owner, with the paper polling about
twice as many sources as today. The owner sets the real number (section 7).

Node B already serves about 100 more registered users today, with 2 or 3
busy beside the paper and the owner. That figure is an estimate, not a
load test (`docs/launch-checklist-2026-10-21.md` L4-03;
`docs/reports/vm-density-limits-2026-10-14.md`). So friends and family
should fit. The work is to measure it (RES-19), give memory back (RES-06,
RES-17), share the gateway fairly (RES-11), and stop storage growth
(RES-22, RES-23, RES-24).

### Secure

Friends and family are trusted people. The real attackers are the web
pages their agents read, the model-written code that self-development
runs in capsules, and any bug that lets one computer reach another. So the
bar is: no path from one computer to another computer, to the host or to
the shared object graph, and no path from a capsule to the network that
skips its identity check. Strangers add one more attacker, the user, who
drives their own guest on purpose.

| Boundary | Items | Friends and family? | Proposed bucket |
|---|---|---|---|
| Desk confinement, frame role checks, capsule seccomp | SEC-01, SEC-02, SEC-13 | yes | done |
| Host egress floor (no cross-tenant, private or metadata addresses) | SEC-03 | yes | done |
| Gateway bearer bound to its computer | SEC-05 | yes: friends' computers spend the owner's model quota | exit (enforce is live; step 5 proof owed) |
| Capsule egress tunnel identity | SEC-06 | yes: this is where model-written code reaches the network | exit (global enforce) |
| Object-graph route trusts loopback | SEC-11 | yes: cross-tenant writes to the shared graph | exit (enforce) |
| Dolt auth and owner-act enforce | SEC-12 | yes: the owner act is what arms self-development on a friend's computer | exit (cut 3, then cut 4) |
| Host unit containment | REC-02 | yes | exit (nearly done) |
| One writer per object kind | ARCH-04 | yes: a self-developed desk must not write Texture documents | exit (Texture enforce) |
| Firecracker jailer | SEC-04, RES-05 | conservative yes: it is built, it is P0, and a VMM escape is root over every computer, the paper included | exit |
| Guest DNS through the host resolver | SEC-10 | conservative yes: the guest line is built; it rides the jailer's cold boot | exit |
| Non-root guest daemon, guest sysctls | SEC-15 | strangers: desks are already confined inside the guest, and the guest belongs to its user | pre-launch (its two enforce switches can ride an exit deploy) |
| No general guest internet; package mirrors | NET-01, NET-02, SEC-10's last slice | strangers. Honest cost for friends: a prompt-injected agent can still send that friend's own data out over open egress | pre-launch (owner call, section 7) |
| Data at rest | NET-06 | strangers. Friends are told that the host operator can read their data | pre-launch (owner call) |
| SMT and core scheduling | NET-07 | strangers (owner ruling already) | already deferred |
| Claude delegate key off the guest disk | SEC-08 | no: friends never get the owner's Claude token | later (owner call) |

### Self-development

- **On the owner's computer, end to end.** A request in Texture. Then the
  engineering cast, the capsule build, the freeze, the owner's approval as
  an enforced owner act, the apply and the route receipt. The computer
  keeps working across the planned restart, and the owner can restore it
  to the head before the apply. It all shows on the tape. Today this is
  proven only on test computers: the GPT-6 Luna arm went from mint to
  restore in about 6 minutes
  (`docs/reports/selfdev-two-arm-drill-imageapp-2026-10-21.md`). The first
  pass keeps self-development off real computers ("Self-development and
  tracers only on test computers or the allowlisted delegate computer",
  the metamission's `must_preserve`). Lifting that rule is the owner's
  act (section 7).
- **Then on a friend's computer.** The friend's own owner act arms it, not
  the owner's. The guest can no longer arm its own mode
  (`internal/platform/self_development_modes.go`, the owner-only write).
  Owner acts are still in observe mode, so SEC-12 cut 4 is part of this.
- **What it needs from the checklist.** ARCH-10 round 4 (a lost frozen
  candidate, report replay, the wake path) and the cold-start proof.
  RES-04 (a release build goes over the capsule's 2 GiB request). A host
  reboot path that does not wait minutes on a user's self-development run
  (the metamission's `blocker_or_risk`; owner: "We should definitely not
  be waiting minutes just to maintain some user self-development run").
- **Built on precommitment records.** Vision: "Self-development is only
  real when built on precommitment scoring". Section 6 places that work.

## 2. Every unfinished item, assigned

There are 50 unfinished items: any slice below stage 6, leaving out the
slices deferred to the owner (NET-07's SMT, ARCH-08's caps). The count is
24 for the Autoputer exit, 3 for Autopaper, 2 for Autoradio, 6 for
pre-launch hardening, 12 for later and 3 to drop.

| Item | Bucket | Reason |
|---|---|---|
| REC-02 host containment | Autoputer exit | Containment gaps are deployed (D6). What is left is the step 6 re-run and the issuer enforce flip. Both are quick. |
| SEC-04 jailer | Autoputer exit | It is built and off. A VMM escape is host root over every tenant, so it is groundwork even for friends. |
| SEC-05 bearer binding | Autoputer exit | Enforce is live. A copied-token proof and a rotation reading close it. |
| SEC-06 tunnel identity | Autoputer exit | It enforces on one test computer. Global enforce guards the network path that self-development uses. |
| SEC-08 Claude delegate | Later | Luna engineering works without it. The exception expires 2026-11-03 (`nix/node-b.nix:1721-1722`). Needed only if the owner wants Claude-backed engineering on real computers. |
| SEC-10 guest DNS | Autoputer exit | The guest line is built (lane/fp3-sec10-guest-dns). It rides the jailer's all-computer cold boot. "No public resolver" waits on NET-01. |
| SEC-11 object-graph loopback | Autoputer exit | Observe is clean. An enforce flip closes a cross-tenant write path. |
| SEC-12 Dolt auth, owner acts | Autoputer exit | Cut 3 is accepted. Cut 4 (owner-act enforce) is what makes a friend's self-development approval unforgeable. |
| SEC-15 non-root guest | Pre-launch hardening | Desks are confined. The daemon split is large and protects against the guest's own user. Its enforce switches can ride an exit deploy. |
| NET-01 host fetch, no guest internet | Pre-launch hardening | It is the stranger-abuse and exfiltration boundary, and it is large: every capability needs a host route first. The source half may come forward with Autopaper if per-origin politeness needs it. |
| NET-02 package mirrors | Pre-launch hardening | SEC-06 global enforce covers capsule egress for friends. Mirrors are the durable closure and dedup downloads at stranger scale. |
| NET-03 vsock control plane | Later | Design only. P2. It follows NET-01. |
| NET-04 remove the guest NIC | Later | It follows NET-01 to NET-03. |
| NET-05 egress grant object | Later | The package is inert. Wiring is needed when app agents ask for new egress. |
| NET-06 data at rest | Pre-launch hardening | The vault and escrow are built dark. Volume keys are an owner decision. For friends, the disclosure that the operator can read their data stands in for it. |
| NET-09 policy signers, passkey acts | Later | Design only. P2. It matters for public track records (section 6) and strangers. |
| NET-10 app-agent network policy | Pre-launch hardening | It is needed once people install app packages that others published. |
| RES-01 in-guest cgroups | Autoputer exit | The OOM leg has never run on staging. This is a stability proof. |
| RES-02 desk memory budget | Autoputer exit | The same OOM leg (`?leg=oom`, 26186d831). Note the earlier attempt that could not drive the OOM (`docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md`). |
| RES-03 exec-go-session bound | Autoputer exit | It is deployed with no proof. Self-development's Go sessions run in it. |
| RES-04 capsule memory | Autoputer exit | Release builds go over 2 GiB. Self-development on real computers needs the bound proved and the doctrine-mode decision. |
| RES-05 page cache under the jailer | Autoputer exit | It ships with SEC-04, or the jailer costs density. |
| RES-06 memory return | Autoputer exit | Memory return is a stated scalable bar. It is on for test computers and not yet for the paper. |
| RES-07 idle page-cache split | Later | It only informs RES-08. |
| RES-08 pmem and DAX | Later | Net saving about 197 MiB. It needs DEC-05, a cross-tenant side-channel question. |
| RES-09 virtio-mem hotplug | Drop | It cannot work on Node B (39-bit addresses, problem record b430934fa). Turn hotplug off. Revisit only through RES-20. |
| RES-10 balloon stats, emergency inflation | Later | DAMON reclaim (RES-06) is the main lever. Emergency inflation matters at stranger density. |
| RES-11 gateway fair share | Autoputer exit | Several friends self-developing at once share one quota with the paper. Fairness is a stated bar. |
| RES-12 batch endpoints | Autopaper | Calls that can wait are mostly ingestion and synthesis. This is the cost lever for the scale-up. |
| RES-13 snapshot levers | Pre-launch hardening | Every deploy voids every snapshot. That costs little at friends' scale and matters for many idle strangers. |
| RES-16 data images shrink | Drop | Done in substance: discard is on for every computer under the 1.17 pin (the paper lost 29.6 GiB at its first trim). The trial slice is superseded. Tick it. |
| RES-17 idle reaper | Autoputer exit | It is a memory-return item. Shorten pool retention below the idle timeout and read the effect. Quick. |
| RES-18 daemon memory | Drop | Measured. FP0 found no daemon lever (11:44). Its follow-up is RES-06, which is in the exit. No groundwork is lost. |
| RES-19 density benchmark | Autoputer exit | It sets the friends-and-family number. Never run on Node B. |
| RES-20 Cloud Hypervisor | Later | Owner, 2026-10-23: keep Firecracker 1.17. CH stays on the itinerary, not a priority. |
| RES-22 btrfs metadata | Autoputer exit | At 82 to 84% used, running out of metadata space can stop the whole host. This is storage groundwork. |
| RES-23 paper journal writes | Autoputer exit | The write load drives the GC stalls behind the `/health` windows. R1 to R4 are proved. R7 (the trace segment log) is the next cut. R5 is only needed if the GC bar is still unmet. |
| RES-24 artifact store repack, retention | Autoputer exit | Storage growth on Node B. Run the census. The legacy 46 GB delete waits for the owner's yes. It also gates the off-host ledger copy (section 6). |
| ARCH-03 app package carries its desk | Autoradio | The podcast and reader app agents ship as packages with their own desks (launch checklist L3-02, L3-03). |
| ARCH-04 one writer per kind | Autoputer exit | Observe is live. Texture enforce is quick and bounds what a self-developed desk can write. |
| ARCH-06 agents as records, pooled slots | Autopaper | The ingestion scale-up multiplies research agents. Pooled worker slots are the density lever for that. |
| ARCH-08 tree token totals | Autopaper | The deployed reading is the scale-up's cost meter (DEC-07: track spending, no cap). |
| ARCH-09 compute capsule kind | Later | No milestone needs an app-desk compute capsule yet. |
| ARCH-10 N capsules, admission | Autoputer exit | Round 4 and the cold-start proof are self-development correctness. The N-capsules slice can follow the exit if the owner prefers. |
| ARCH-11 executor VMs | Later | Design only. Open owner questions. |
| ARCH-12 executors on other hosts | Later | After ARCH-11 and S9. |
| ARCH-13 store fan-in | Autoputer exit | Round 7 is landable. The kill -9 drill is a restart-durability check, then debounced checkpoints on the paper. |
| ARCH-14 app verb registry | Autoradio | Autoradio's routing ("send an email to Sally") calls app verbs across desks. |
| ARCH-15 launcher rename | Autoputer exit | Deployed. Eyeball the icons and tick it. A two-minute check. |
| OPS-01 paper across restarts and load | Autoputer exit | This is the stable bar itself: crash closure step 5, the reboot script, the `/health` fix under GC. |

Two notes on the table:

- Autopaper's real scale-up work, the launch checklist's phase 5 and
  source expansion, is not in items.json. The three Autopaper items above
  are only the checklist items that serve it.
- Autoradio's main work is the voice layer (`~/mike`, L0-07 and L7-01) and
  the server items L6-01 to L6-07. None of that is in items.json either.

## 3. The Autoputer exit: order, effort, parallel lanes

Effort: **quick** means a switch and a proof on a test computer. **Medium**
means a lane, review and a deploy. **Large** means design first.

### Batch 0: no deploy (start now, FP0 plus one worker)

These are readings and proofs on what is already deployed:

- the RES-01 and RES-02 OOM leg;
- the RES-04 Go-bounds drill reading;
- the ARCH-10 cold-start spawn lines;
- RES-19, the density benchmark on Node B;
- RES-22, the metadata reading and a balance plan;
- RES-24, the copy-only repack census;
- REC-02, the verifier re-run;
- ARCH-15, the eyeball check;
- the observe windows that the flips in E1 need (SEC-06, SEC-11, ARCH-04, REC-02 issuer);
- SEC-05 step 5, if a host-side guest shell route exists (medium if one has to be built);
- OPS-01 step 5, at the paper's next crash boot.

All of these are quick.

### Deploys, in order

1. **E1, the flip deploy** (quick each; nix selectors and enforce
   constants). It carries:
   - SEC-06 global enforce;
   - SEC-11 enforce;
   - ARCH-04 Texture enforce;
   - REC-02 issuer enforce;
   - RES-11's scheduler on one test computer;
   - RES-06 DAMON on the paper (the owner's 2026-10-22 rule puts resource utilization ahead of short-term uptime);
   - RES-17's shorter retention;
   - the SEC-15 enforce switches, if the owner wants them;
   - riders: RES-03's capsule probe leg (inert).

   Each flip has its own proof and can be reverted alone. This needs an
   owner ruling: a flip-only deploy counts as one live red batch (section 7).
2. **E2, the jailer on one test computer** (medium). SEC-04 step 2 with
   RES-05, a vmctl change, live red alone.
3. **E3, Dolt locked** (medium). SEC-12 cut 3, alone (auth).
4. **E4, the store** (medium to large). The OPS-01 `/health`-under-GC fix
   with ARCH-13 round 7. Alone (store engine, boot path), tier 2 review.
5. **E5, self-development and the gateway** (medium). ARCH-10 round 4
   (new concurrency, tier 2), plus RES-11 for every computer and the
   gateway body-caps enforce. These are two disjoint live reds, which the
   owner's 13:35 rule allows.
6. **E6, the jailer everywhere** (medium). SEC-04 steps 3 and 4. Step 4
   cold boots every hibernated computer once, so SEC-10's guest image
   change rides it for free.
7. **E7, the trace segment log** (large; its design is ruled, O1, O5 and
   O6). RES-23 R7. Alone (tape and evidence), tier 2. Then measure the GC
   again. Build R5 only if `/health` still fails the bar.
8. **E8, owner-act enforce** (medium). SEC-12 cut 4, alone (auth), after
   its observe window.

Two items run as host operations with no deploy: the RES-24 legacy delete
(after the owner's yes) and the RES-22 balance. Then come the two
self-development episodes in section 4. They need no deploy, only the
owner's act.

### Parallel lanes

Five workers plus FP0 fits the 5-to-7 local rule, with more on the agents
VM:

- **Lane S, store:** the OPS-01 `/health` fix and ARCH-13 round 7, then RES-23 R7. This is the long pole.
- **Lane D, self-development:** ARCH-10 round 4, RES-03's probe, RES-04's doctrine-mode measurement, and the episode script for the owner's computer.
- **Lane H, host authority:** SEC-12 cuts 3 and 4, REC-02, the SEC-11 config, the jailer rollout steps, and the force-reboot path.
- **Lane G, gateway:** RES-11 and body caps, and the SEC-05 copied-token route.
- **Lane C, CI:** the three cuts below.
- **FP0:** batch 0, landing, proofs.

**Rough time.** The landing itself is about 8 deploys at about 25 minutes
each, plus proofs, so roughly half a day of serialized landing once the
candidates are ready. The long poles are building R7 and the `/health`
fix. If each converges in its first round, the exit is about two to three
days of wall clock. The security and density flips alone are about one
day. This is an estimate, not a measurement.

### The CI and deploy cycle (an explicit exit item)

**Current wall time.** These are the last 20 `ci.yml` runs on main
(`gh run list --workflow ci.yml --limit 20`, 2026-10-22 18:18 to
2026-10-23 03:55 UTC):

- 12 full successful runs, median 11.8 minutes, range 8.9 to 15.7;
- 3 runs with no Go change, about 1 minute each;
- 2 failures and 3 cancelled runs.

The full landing cycle is about 25 minutes from push to proof (ledger,
2026-10-21 18:45, B1).

On top of that sits the Linux `-race` mirror on the agents VM, which runs
one heavy test at a time. For example, D4's mirror waited from 23:07 and
passed at 23:44:43. The tests are no longer the critical path, because
the shard balance has landed. In the three runs read (39892562878,
39873896293, 39897596898), the Node B host NixOS closure prebuild took
414 to 440 seconds. The deploy waits for that build.

The top three cuts:

1. **Replace the serial mirror with CI on the lane branch** (quick;
   yellow or orange CI).
   - `ci.yml` already accepts `workflow_dispatch`, and both Node B jobs
     only run on `refs/heads/main` (`.github/workflows/ci.yml:818, 870`).
     So `gh workflow run ci.yml --ref lane/<x>` runs the 16 test shards
     without deploying.
   - Add a race input so dispatch can choose `-race`.
   - That is about 8 minutes on parallel runners in place of a 37-minute
     queue on one VM.
   - The cost is a share of the account's 20-job limit, so schedule it
     away from a main run.
2. **Stop deploying the host for coding-agent skill edits** (quick;
   orange CI).
   - Both classifiers treat every `skills/*` path as a runtime change
     (`.github/scripts/ci-impact-classify:115, 127`;
     `.github/scripts/deploy-impact-classify:335-338`).
   - The autoputer service ships every `SKILL.md` (`flake.nix:400-428`).
   - The runtime reads only `cognitive-transform-portfolio`
     (`internal/agentcore/skill_context.go:12-14`;
     `internal/provideriface/config.go:205`).
   - All 5 commits on main that touched `skills/` since 2026-10-20 were
     docs or skills only. Each one still paid for Go tests and a host
     closure build. For example, `2ce86f869` ran 9.3 minutes and rebuilt
     the host (run 39897596898).
   - Narrow the path to `skills/cognitive-transform-portfolio/`.
3. **Take the Node B closure build off the deploy path** (medium to
   large; red, because it touches deploy artifacts).
   - Build the closure on a builder with a warm cache, and let Node B
     substitute it. This is the open proposal in
     `docs/problems/ci-deploy-latency-per-service-go-builds-2026-10-09.md`.
   - It saves about 5 to 6 of the 7 minutes on every deploy.
   - A cheap step with it: the Actions cache is at 9.8 GB of its 10 GB
     limit. Eviction costs a cold run about 250 seconds
     (`docs/problems/ci-critical-path-race-shards-2026-10-22.md`, finding 4).

There is a fourth lever, for after the exit. Ten rollout selectors live
in `nix/node-b.nix` (`*_ONLY=`), so every "switch on one test computer"
costs a host closure build and a vmctl restart. A runtime selector file
that vmctl reads would make flips into config writes. It is red (vmctl),
and it pays off most in the pre-launch rollouts.

## 4. The Autoputer exit proof

"Done" is all of these, run against `https://choir.news` and recorded
with their run ids:

1. **Restarts.** The paper and the owner's computer go through a planned
   deploy refresh, a crash boot and a host reboot.
   - Schedules are armed.
   - Polls resume within 5 minutes.
   - After the crash, open work is closed as "interrupted by a restart".
   - Planned work resumes once.
2. **Load and GC.** A day of `/health` at 10-second intervals on both
   computers, across at least 50 online GC passes. At least 99.9% answer
   200, and no 503 window is longer than 10 seconds.
3. **Budgets.** The desk OOM leg, the capsule exec-go-session bound and a
   capsule release build each kill only inside their own cell, and the
   daemon stays alive.
4. **Density.** RES-19 runs at the friends-and-family target with the
   paper and the owner. Host memory stays inside the admission bound.
   Memory comes back after an idle period (RES-06). Two busy computers
   and the paper share the gateway fairly (RES-11).
5. **Storage.** Node B's free space is flat or rising over the same day.
   btrfs metadata is below 75%. The artifact-store census is recorded.
6. **Security probes.** All of these pass:
   - `verify-host-hardening` with 0 FAIL;
   - the isolation self-test (forged frame, seccomp, OOM);
   - a bearer copied between two test computers is refused;
   - a capsule tunnel mismatch is refused on every computer;
   - a loopback object-graph caller is refused;
   - a passwordless Dolt login is refused;
   - a mode write from a guest credential, or one without an owner act, is refused;
   - another desk's write of a Texture document is refused;
   - `vmm_confinement_verify` passes on a jailed computer.
7. **Self-development on the owner's computer.** One full episode, from
   the request to a restore that the owner can reach, readable on the
   tape. Its engineering cast commits a hypothesis that the apply's
   verification settles (section 6).
8. **Self-development on a friend's computer.** The friend's own owner
   act arms it. The same evidence as check 7.
9. **The landing cycle.** One timed landing of a Go change runs from push
   to deployed in under 15 minutes, with the mirror replaced by CI.

## 5. After the exit, in the owner's order

- **Autopaper.** Ingestion scales up (sources, NET-01's source half if
  needed, RES-12, ARCH-06, ARCH-08). Then the owner tunes content
  quality.
- **Autoradio.** Bring in the voice layer from `~/mike` (L0-07, L7-01),
  then the server items L6-01 to L6-07, ARCH-03 and ARCH-14.
- **Pre-launch hardening, before the web soft ship.** SEC-15, NET-01,
  NET-02, NET-06, NET-10, RES-13, plus the launch checklist's B11 and B14
  (accounts, recovery, admission limits, red teaming, the vault).
- **Ship.** The iOS and Android apps.

## 6. Precommitment records: the bridge from the Autoputer to Autopaper and Autoradio

Owner (2026-10-23): "precommitment records/psa ... thats the key bridge
from autoputer and self-dev to autoradio. and its the essential core
scientific idea of choir". This section only uses the docs' own words for
it.

**How the docs use the terms.** `docs/precommitment-records.md` §1.5:
"'Precommitment record' is the primitive (code: *commitment record*, kind
`choir.commitment_record`). PSA is the research program: whether conduct
and consequences stay coupled to the prospective representation under
which a system was allowed to act." §1.2 calls in-context reinforcement
learning "the core claim". It quotes the owner: storing records in the
desk's REPL, with context packs and on-demand retrieval, "serves as
in-context reinforcement learning". The vision says the same:
"Self-development is only real when built on precommitment scoring —
self-supervising and self-learning, not another app builder"
(`docs/choir-vision.md`).

**What exists today.** As of the doc's §8.1 status of 2026-10-14, plus
the L3 station file. This has not been re-checked on staging for this
proposal.

- *Built:*
  - the typed record and its five acts;
  - append-once storage on the object graph;
  - derived views, including score-free and score-carrying packs, and a
    gate that refuses a learning claim citing no scored records;
  - the desk verbs and one Cast and Ask surface;
  - records as the work state (inbox sequences, conversations, rounds,
    hypotheses with open, settled and dropped states). "Engineering runs
    on records (C1e)."
- *L3-1 is deployed* (`0fe4cead0`, 2026-10-15). Hypotheses may name a kind
  (assertion or forecast), a criterion, and a `resolves_by` date within
  7 days. The smoke check saw the paper's Casts carry kinds
  (`docs/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md`).
- *Not built* (§8.1):
  - "settlement and scoring of hypotheses on the live paper";
  - routine independent scoring;
  - automatic pack injection;
  - record query verbs in the REPL;
  - `commit_hash` and signing.

  §8.1 also says: "Pre-outcome ordering is not proven", and "'records
  exist' is not 'records settle'. None of H1 to H5 has been run."

**The proposed first slice after the Autoputer exit: L3-2, the settler's
record, with one self-development use.**

- *On the paper.* L3-2 as its station file already defines it:
  - a settlement carries the settler's identity and evidence refs, and
    the opener never settles;
  - an ordering witness shows that the commitment came before the
    earliest capture the settlement cites. It uses the event-chain
    sequence plus a host-signed first-creation witness at the corpusd
    write. A design panel already decided that witness (3 of 4,
    `docs/evidence/l3-file-review-2026-10-15.md`);
  - a forecast past its `resolves_by` with no settlement counts as
    overdue.

  The acceptance from §8.2: "ten or more hypotheses committed before
  their evidence, at least half settled with evidence refs".
- *In self-development.* The engineering cast in the exit episode commits
  a hypothesis about its change. The apply's verification settles it.
  This is the vision's "self-development built on PSA" in its smallest
  form, and it is exit check 7.
- *Next after that:* L3-3, the scorers (the self-score plus Jev and one
  other model, per-desk views with denominators).
- *The link to Autoradio.* The phone's supervision feed is "the
  materiality-ordered supervision projection of
  `docs/precommitment-records.md`", and the plan pulls it "out of design"
  (`docs/reports/autoradio-mobile-app-plan-2026-10-14.md`, lines
  1446-1451). The prototype says that turning a spoken take into "a
  precommitment record to an app desk" depends on, among other things,
  "the settlement loop (L3)" (`docs/reports/autoradio-prototype-2026-10-14.md`,
  lines 131-135). Settlement comes before the voice layer can route
  anything.

**What it depends on in the Autoputer exit.**

- **RES-24 and the off-host ledger copy.** L3-2 "starts once the off-host
  copy is recoverable" (L3 station file). The node-a backup key is done
  (L0-03). What is left is the tape-segment repack, the census and a
  restore drill (L2-04). Without them, settlements are not recoverable
  evidence.
- **RES-23 R7.** Moving trace lines to a segment log must not move or
  drop commitment records. They stay on the object graph and tape (§2.2).
  E7's review should check this.
- **OPS-01 and ARCH-13.** The settling loop runs on the paper around the
  clock, so the paper has to survive GC and restarts first.
- **SEC-11 and SEC-12.** The first-creation witness is a corpusd write,
  and the shared object graph must refuse forged callers. A settlement is
  only worth something if nobody else can write its evidence.
- **Self-development on the owner's computer (exit check 7).** It is the
  first place a scored engineering hypothesis can exist on a real
  computer.

## 7. Decisions that are the owner's

The owner drives these in the morning. Each has a default so work does
not stall.

1. **Lift the test-computer-only rule for self-development.** First for
   the owner's computer, then for named friends. Also: does a friend's
   apply need only the friend's approval? Default: yes to both, in that
   order, after E8.
2. **Engineering model.** Luna or DeepSeek engineering (proven) versus the
   Claude delegate with Option K (SEC-08). Default: Luna, and let the
   delegate exception expire on 2026-11-03.
3. **The friends-and-family security line** in section 1. Specifically:
   leave general guest internet open, leave data at rest unencrypted, and
   run the guest daemon as root until pre-launch, with friends told so.
   The default above moved the jailer and guest DNS into the exit.
4. **A flip-only deploy is one live red batch** (E1), with each flip
   proved and revertible alone. Default: yes.
5. **The friends-and-family size** that sets the RES-19 bar. Default: 25
   registered, 5 busy at once.
6. **Host operations.** The RES-24 legacy tape delete (about 46 GB, on the
   census evidence) and a filtered btrfs metadata balance in a quiet
   window (RES-22). Default: yes to both after batch 0.
7. **PSA's place.** Is L3-2 the first slice after the exit, and should a
   scored engineering hypothesis be part of the exit proof (check 7)?
   Default: yes to both.
8. **Drops.** RES-09 (hotplug off), and closing RES-16 and RES-18 as done.
   Default: yes.
