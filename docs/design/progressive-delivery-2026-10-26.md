# Progressive delivery for Choir (2026-10-26)

Design for one switch registry, a ring order, and a host controller that
promotes and reverts switches on measured evidence, so the owner stops
ruling on each switch.

- **Status:** ratified by the owner 2026-10-28 (section 10, "Owner
  answers"), with a more aggressive policy table than proposed. **Review 340**
  (Devin): code claims verified; findings F1 to F7 folded in (class tokens on
  protected rings, security revert is hold-and-page, 4 releases kept, finding
  number, OG gate not in nix, controller expiry, seed rows by id). Receipt:
  `_runs/bunch340/devin.out`.
- **Mutation class of this document:** green. Nothing here is built. Every
  build slice carries its own class and review tier in section 8.
- **Review tier:** a design that a build will follow is reviewed like the
  change it designs (CLAUDE.md, Mutation Classes). The controller is red
  (host, VM lifecycle, settings plane), so this document wants tier 2 before
  slice 3 starts. Slices 0 to 2 need no model review.
- **Prompted by the owner, 2026-10-26:** "we need to be thinking at a higher
  level. i cant keep making all these ad hoc rulings ... canary deployments
  ... fleet mgmt, a/b testing, auto revert ... lets think about how we can
  get smarter, and apply established winning patterns effectively." Also: "I
  don't want to wait a week on anything"; "We've got into this habit of
  writing code but having it switched off."
- **Inputs read:** the fresh switched-off audit
  (`switched-off-inventory-2026-10-26.md`, 52 rows, finding F-0),
  `internal/vmmanager/computer_select.go`,
  `internal/store/computer_setting.go`, D2 design (§0, §5), `ci.yml`,
  `scripts/node-b-ci-gate`, `.github/scripts/node-b-ci-deploy`,
  `docs/archive/roadmap-2026-10-26.md`, and CLAUDE.md. I read no live system and
  called nothing on staging. Where I did not verify something in the repo it
  is marked **unverified**.
- **Sources.** Fetched this session: Google SRE Workbook, "Canarying
  Releases" (S1); Argo Rollouts analysis docs (S2); Flagger "how it works"
  (S3); Fowler and Hodgson, "Feature Toggles" (S4); Microsoft Learn, Windows
  servicing strategy and deployment rings (S5). Cited from memory, not
  re-fetched (the fetch failed or the page moved): Spinnaker Kayenta (S6),
  Unleash feature-flag lifecycle (S7), Google SRE Workbook error budget
  policy (S8), Chrome release channels and staged rollout (S9). Treat S6 to
  S9 as lower confidence; the design leans on them only where stated.

## 1. The answer in brief

**What the problem is.** Choir has no way to say "this change is safe
enough to be on for everyone", so every switch waits for a person. The
audit's 52 rows show it: 14 can go on now, 25 wait for "one named step",
5 need a ruling, 8 should be deleted. Of the 25 named steps, by my reading
of the table, 7 are a metric read from `/health`, observability or the
journal, 9 are a proof script or drill that already exists, 3 are a
supervised host act, 4 need code first (F-0 among them), and 2 are an
external event. Almost all of these are things a machine can read. The
person is acting as a slow, inconsistent metric reader.

**What to build, smallest first.**

0. **A ring order and a policy table** (section 3), plus **one registry
   file** declaring every switch with kind, owner, surface, promotion
   metric, bound and expiry (section 4). No code. FP0 walks the 14 "ON NOW"
   switches through the rings by hand under the policy on day one, using the
   same readouts the controller will use later. This alone removes the
   per-switch rulings.
1. **A CI lint** that fails a new unregistered switch and enforces expiry
   (section 7). Tooling only.
2. **F-0, settings follow the registry default** (section 4.3). A missing
   row reads the registered default instead of off; a new computer inherits
   what its class's ring has already proved. Red, tier 2; already roadmap
   station W1.
3. **A ring controller on the host** that samples the metrics Choir already
   exposes, advances a switch one ring when its soak and guards pass, and
   reverts it when a guard breaks (sections 5 and 6). Shadow mode first.
4. Later: settings and guest env that follow the ring live, ring-ordered
   code refresh, and automatic code rollback.

**What transfers and what does not.** Rings, soak by exposure, a small set
of guard metrics, automatic revert, and an expiry on every switch transfer
fully. Traffic-weighted canaries, statistical canary scoring, blue/green and
user-cohort A/B do not: Choir has one host, one persistent stateful computer
per user, and cohorts of single digits (section 2).

**The one decision for the owner.** Ratify the **promotion policy table** in
section 3.3: for each mutation class, the furthest ring a switch may reach
without a person, and the rule that the paper never advances without a human
act. After that, a switch that is registered, passes its guards and finishes
its soak advances on its own. The owner is told by digest and is asked only
for the escape cases: a switch that needs a ring beyond its class's
ceiling, a black-class act, or a stalled switch.

Time to all non-paper computers for an ordinary switch: about 7 hours
(section 3.2), against "no earlier than a week" today.

## 2. Patterns that transfer, and the ones that do not

Each pattern is listed with what it needs in order to work. The four
prerequisites asked for are: **metric**, **control group**, **soak**,
**owner**.

### 2.1 Transfers

| Pattern | Source | Needs | Choir version |
|---|---|---|---|
| **Canary: a small slice runs the change while the rest is the control** | S1: "Deploy the change to a small slice of production (the canary) while the rest (the control) runs the live version" | Metric: a few user-visible SLI-like signals, "roughly a dozen at most", not noisy host CPU. Control: the unchanged rest, compared side by side and against absolute targets. Soak: long enough for "a representative sample"; for non-interactive work, "at least one full work unit". Owner: the pipeline escalates "to a human". | The control is the other computers on the same host at the same time, plus the same computer's own prior series (section 5.2). The slice is a ring. |
| **One canary at a time** | S1: "We strongly advise running only one canary deployment at a time" | Attribution: a regression must be traceable to one change. | Single flight per surface, bundles and bisection for the rest (section 5.5). |
| **Canary duration must fit the release cadence** | S1: "If you release daily, you can't let your canary last for a week" | A soak shorter than the interval between changes. | Choir deploys several trains a day (trains 20 to 26 on 10-26). A ring soak of hours is the only soak that keeps up. |
| **Analysis as a template with success, failure and limits** | S2 (Argo): `successCondition`, `failureCondition`, `interval`, `initialDelay`, `count`, `failureLimit`; a run that meets neither condition is **inconclusive** and the rollout pauses; background analysis failure aborts and returns weight to zero | Metric: a query per condition. Soak: interval times count. Owner: a human resumes or aborts an inconclusive run. | The registry's `analysis` block has the same fields. Inconclusive means hold, never advance (section 5.4). |
| **Stepwise promotion with a failed-check threshold** | S3 (Flagger): every `interval` run checks, weight rises by `stepWeight`, and `threshold` failed checks stops the analysis and rolls back | Metric with a threshold range. Soak: interval times steps. | Ring step is the weight step. `failure_limit` is the threshold. |
| **Automated canary judgement against a baseline** | S6 (Kayenta, from memory): compares canary and baseline metrics and scores pass, marginal or fail; works best against a fresh baseline, not the old production fleet | Metric: many samples per group. Control: a baseline launched at the same time. | Only the idea transfers: a concurrent baseline. The statistics do not (2.2). |
| **Deployment rings: small expert group, then broader groups, with each department split across rings** | S5 (Microsoft): pilot users who give feedback, then broad deployment "using deployment rings"; special devices excluded from the phased cycle | A ring list, recruited volunteers, an exclusion list for critical devices. | The ring order in section 3. The paper is the "special-purpose device" and sits last and apart. |
| **Release channels** | S9 (Chrome, from memory): Canary, Dev, Beta, Stable, plus a percentage staged rollout of Stable over days | A user or machine chooses a channel; the build is promoted between them. | A computer's ring is its channel. A later owner act can pin a computer to an earlier or later ring (section 4.3). |
| **Feature toggle categories and lifetimes** | S4 (Fowler, Hodgson): release toggles "should generally not stick around much longer than a week or two"; ops toggles are mostly short-lived, with some long-lived kill switches; permissioning toggles can last years | Know which kind each toggle is, because the kind sets its expiry. | `life:` field in the registry: release, ops, permission, experiment. |
| **Toggle inventory has a carrying cost** | S4: "Add a removal task ... when a Release Toggle is introduced", set expiration dates, make "time bombs" that fail a test after expiry, cap the number of live toggles | An inventory, an owner per toggle, a hard date, a cap. | Sections 4 and 7. |
| **Prefer static configuration in source control** | S4: toggle config should "move through the same delivery pipeline as code" | Declaration in git. | Declaration in git (`switches/registry.yaml`). Runtime state (current ring) is separate and lives on the host (section 4.2). |
| **Stale-flag detection** | S7 (Unleash, from memory): flags carry a type with an expected lifetime and are marked potentially stale when they outlive it | A type, a creation date. | The lint and the controller's "ready to retire" report. |
| **Error budget gates the pace of change** | S8 (SRE Workbook, from memory): when the budget is spent, releases freeze except for reliability and security fixes | An SLO and a measurement. | A **rollout budget** (section 5.6), not an SLO budget. |
| **Automatic rollback on a failed gate** | S2, S3 | A last-known-good target kept ready. | Per kind of switch in section 6. D2 keeps 4 guest releases on Node B (`GO_CHOIR_GUEST_RELEASE_KEEP_MAX`, default 4). |

### 2.2 Does not transfer, and why

| Pattern | Why it does not fit Choir |
|---|---|
| **Traffic-weighted canary (1%, 5%, 25% of requests)** (S1, S3) | Choir's unit is a persistent computer with its own disk and tape, not a request. A user's computer cannot be split between two versions of itself. Routing 5% of one computer's requests to a new build would run two writers on one tape. The unit of exposure is a whole computer. |
| **Statistical canary scoring** (Kayenta's nonparametric tests, S6) | Needs tens of samples per group. A ring of 1 to 8 computers produces too few. Use **guards**: zero-tolerance counters (a crash, a refused start, a corrupt snapshot) and wide relative bounds, not p-values. |
| **Blue/green with instant traffic cutover** | One host, one set of persistent computers, stateful disks. There is no second environment to cut over to. The nearest analogue is the guest release directory (D2): old and new coexist and a computer moves between them at wake. |
| **User-cohort A/B and experiment toggles** (S4 "experiment toggles", LaunchDarkly-style targeting) | An experiment toggle needs a population large enough to detect an effect "long enough to generate statistically significant results". We have single-digit cohorts. Experiments on model, prompt or desk behavior are the ICRL line of work, with paired runs on the same computer; they are out of scope here. The ring walk is a safety mechanism, not an experiment. |
| **Error budget from a user-facing SLO** (S8) | There is no external SLO and not enough traffic to compute a burn rate. A budget of **incidents** (unplanned crashes and auto-reverts) per day is the usable form. |
| **Multi-region waves, percent-of-fleet over days** | One host. The fleet is a handful of computers; the ring is the percent. |
| **Hours-to-days fleet bake times** (S5, S9) | Right for millions of devices where a rare defect needs volume to appear. Choir's defects have shown up in the first wake or first launch (the mincore filter bug on d@e's first Diff hibernate; the 11:49 refused start). A cycle of launch, idle, hibernate, wake is 10 to 30 minutes. Soak by **exposure**, with a short time floor. |
| **Staging then production** | `https://choir.news` staging **is** the acceptance and the user-facing environment (CLAUDE.md, Default Environment). There is no production to promote into. Rings replace the missing environment, inside the one host. |

## 3. Choir's rings

### 3.1 Cohorts we have

The unit is a computer. The selector already speaks in computers, ids and
warmness classes (`selectsComputer`, `internal/vmmanager/computer_select.go`:
VM id, computer id, `class:<warmness>`, `*`). Classes named in the repo and
audit: `test`, `primary`, `premium_always_on`, `public_platform`,
`critical_protected`.

| Ring | Members today | Size (what I know) |
|---|---|---|
| **R0 proof** | d@e (`computer-59c902c5...`), the designated proof computer; plus a disposable test computer when a probe needs a fresh one | 1 to 2 |
| **R1 test** | `class:test`: every test account's computer, including the DAMON pair (9648031f, 71afbf14) and fp0-jobpark-1024b. b@c is **excluded** while X1 leaves it crash-looping at boot (D2 §proof computers) | single digits; **unverified**, the controller's first act is to count them from vmctl |
| **R2 owner** | the owner's own computers, named by **explicit computer id** (not `class:`; see below) | few; **unverified** |
| **R3 primary** | `class:primary`: every other user's ordinary computer, and every new computer | **unverified**; possibly 0 to a few |
| **R4 platform** | `public_platform`, by explicit computer id | 1 |
| **R5 paper** | `critical_protected`, by explicit computer id: the paper, always busy | 1 |

**Ring membership is by id, except R1 (review 340 F1).** `WarmnessClass` is
a boot-only input: the ownership and resume paths (`internal/vmctl/ownership.go`
around :1750 and :1840) never set it, so a `class:` token can miss a resumed
or re-adopted computer. The repo already bans `class:` selection for protected
computers (`capsule_tunnel_identity.go:14`, `guest_dns.go:20`,
`checkpoint_cadence.go:55-71`). So R0, R2, R3, R4 and R5 are lists of explicit
computer ids that the controller derives from vmctl's computer table and
records in `state.json`; only R1 may use `class:test`. R3 (primary) is "every
computer not named in another ring", resolved by the controller at each tick,
so a new computer lands there. `follow:<id>` expands to ids (plus `class:test`
for R1), never to the other class tokens. Making class reliable (set on
resume) is a prerequisite for any later switch to class-based rings.

Two consequences of the sizes. First, **an empty or tiny ring is normal**: a
ring with no eligible computer is skipped, and a ring smaller than the
exposure requirement uses all of its members. Second, the owner's computers
are a canary for everyone after them. That is deliberate: the owner is the
one person who will notice a regression and report it, and the owner has
said "turn things on as soon as we can". It is also a cost the owner should
ratify (open question 3).

### 3.2 Soak per ring

A ring's soak ends when **both** hold: a time floor has passed and an
exposure floor is met. Exposure counts only events **under the switch**: for
a launch-time selector, launches whose recorded decision was "selected"
(vmctl already records the decision at launch; a snapshot restore validates
against the same decision); for a live switch, calls observed after the flip.
A probe that passes against the ring's computers can stand in for exposure.

| Ring | Time floor | Exposure floor | Why this long |
|---|---|---|---|
| R0 | 30 min | one full cycle: launch, idle hibernate (the idle timeout is 10 min), wake to healthy; and the switch's probe passes | The defects found so far appeared in the first cycle. |
| R1 | 2 hours | every R1 computer that is awake has launched or woken once under the switch, and at least 3 distinct computers (or all, if fewer) | Covers several hibernate/wake cycles and the 5-minute settle window. |
| R2 | 4 hours | each owner computer woke at least once under the switch; at least one of them was in use for a session | The owner's real use is the signal here. |
| R3 | 4 hours | at least 2 distinct computers exercised (or all) | Skipped if empty. |
| R4 | 2 hours, and it waits for a watched planned refresh if the switch needs a relaunch | the platform computer launched or refreshed under the switch | Public platform; refresh only at a planned, marker-backed moment. |
| R5 | 1 hour after its own watched planned refresh | the paper completed one tick under the switch | Never time-driven. Section 3.3. |

A security switch with an `observe` then `enforce` ladder (issuer auth, OG
writer gate, tool sandbox, capability floor) walks the **observe** stage
through all rings first, then walks the **enforce** stage, with the soak
doubled for enforce. The observe stage's metric is the `would refuse` count
(the evidence the audit's rows 20, 21, 22 and 23 ask for).

**Best case for an ordinary switch, R0 through R3: about 7 hours** (0.5 + 2
+ 4, with R3 running in parallel with R2's tail when R2 and R3 guards are
independent and both soaks are clean). R4 adds 2 hours, then R5 waits for
its watched refresh, usually the next day. "Turn it on in the morning, on for
every user by evening, the paper next morning." Compare "no earlier than
2026-11-02" for D2.

### 3.3 The policy table (the owner's one decision)

The table says how far a switch may go **without a person**, by the
mutation class of the switch's flip (CLAUDE.md classes). Beyond the ceiling,
the controller still runs the analysis and prints the readout; a person
(FP0 or the owner) types one advance. That is a confirmation, not a ruling:
the question was already answered by the evidence.

| Switch class | Auto ceiling | Soak multiplier | Notes |
|---|---|---|---|
| orange (behavior, app state, queries) | R4 | x1 | Never R5 automatically. |
| red, additive, one-act rollback visible within minutes (resource selectors, observe-mode flips, per-computer features) | R3 | x1 | R4 and R5 by confirmation. This is most of the audit. |
| red, security enforcement (observe to enforce, jailer, DNS, egress) | R1 | x2 | Auto-walk the proof and test rings; owner computers on a confirmation with the readout, because an enforce flip can lock a user out. |
| red, silent or irreversible (deletes, tape and evidence, Texture canonical writes, auth and session, store schema, boot path, new concurrency) | R0 | x2 | The controller measures; a person advances every ring. Two-seat review (tier 2) of the change itself still applies. |
| black | none | none | Never in the registry's auto path. |

Standing rules that hold for every row:

1. **The paper (R5) never advances without a human act.** It is the one
   critical computer, always busy, and the restart rules forbid a restart
   outside a planned, marker-backed refresh and never mid self-development
   apply or while a capsule is live (CLAUDE.md).
2. **A revert is always automatic and needs no ruling**, in any ring, for any
   class. The asymmetry is the point: advance is bounded, retreat is not.
3. A switch at its ceiling is "stalled at ceiling" and shows in the digest. It
   is not an open question. The owner can raise a ceiling by editing the
   registry entry.

## 4. One switch registry

### 4.1 What it is

One file, `switches/registry.yaml` (JSON-compatible YAML so Go and bash both
read it), reviewed like code, in the host-floor path so an engineering
capsule cannot edit the live copy (the floor is "all host-level and
security-relevant code, everything engineering has no access to", owner
2026-10-25). It **declares**. It does not hold current state.

```yaml
- id: vm_snapshot_diff_only            # unique, snake_case
  kind: host_selector                  # host_selector | host_env | let_binding | guest_env | setting | code
  life: release                        # release | ops | permission | experiment   (S4)
  where: nix/node-b.nix:1822           # lint checks that this location still holds the id
  owner: density                       # a station or a named agent desk; never "FP0" by default
  class: red                           # CLAUDE.md class of the flip; selects the policy row
  surface: snapshot                    # attribution domain (section 5.5)
  introduced: 2026-10-24
  expires: 2026-11-07                  # introduced + 14 days unless set; extensions are recorded in-line
  ladder: [on]                         # or [observe, enforce]
  target_ring: R3                      # set from the policy row; a higher value needs a confirmation
  analysis: host-default-v1            # a shared template (section 5.3); overrides below
  promote:                             # per-switch metric and bound
    - metric: wake_to_healthy_ms_p95
      bound: "<= 1000"                 # absolute
    - metric: snapshot_refusals
      bound: "== 0"
  probe: scripts/density/accept_five.py   # optional; stands in for exposure
  revert: {mode: ring_state, effect: next_launch}   # section 6
  retire: "delete the _ONLY selector; Diff becomes the code default"
```

The registry carries a **stage list** (`ladder`) so one entry covers
`observe` then `enforce`. It carries a `gate` kind for the audit's
"ON AFTER" items that are not a metric: `gate: {act: "supervised squash"}`
means a person must record the act once; the controller waits for the record
and still walks the rings.

### 4.2 Declaration versus state: one authority each

Standing question: single state authority. So:

- **Git** (`switches/registry.yaml`) is the only authority for *what a
  switch is, who owns it, how far it may go and when it expires*.
- **The host** (`/var/lib/go-choir/switches/state.json`, root-owned, plus an
  append-only `history.jsonl`) is the only authority for *which ring each
  switch has reached right now*. The controller is the sole writer. vmctl and
  corpusd read it. It survives a vmctl restart (restart durability).
- **nix** stops carrying the value for a registered switch. A registered host
  selector or env in `nix/node-b.nix` takes the value `follow`, meaning "ask
  the state file". The nix line still sets the **code default** floor (off),
  so a missing or unreadable state file means off. The lint refuses a
  registered switch whose nix value is anything but `follow` or the default.
  This is what ends the habit of nix edits as the promotion mechanism.

`follow` is a small addition to `selectsComputer`: a token
`follow:<switch-id>` expands, at each launch, to the ring list in
`state.json`, mapped to the ring table of section 3.1 (ids for R0, classes for
R1 to R5). `selectsComputer` keeps its existing grammar; `follow:` is one
more token. Selectors are already "read at each launch", so promotion takes
effect at each computer's next launch with no deploy.

### 4.3 How F-0 and "follow the ring" work

F-0 (audit section 0 finding 2, item 24): today `computerSettingOn`
(`agentcore/computer_settings.go:36-48`) and six other direct readers read
"no stored row" as off and never consult `KnownComputerSettings`, so all 26
settings are off for every new computer.

The fix has two steps; the first is useful alone.

**Step 1, F-0 as the audit states it.** One helper,
`store.SettingDefault(key)`, and `store.GetComputerSetting` falls back to it
when there is no row. The seven direct readers use the helper. A stored
`false` stays an explicit per-computer opt-out; a missing row follows the
default. The idempotent same-value POST at `computer_settings.go:107` must be
re-checked. Red, tier 2 (the settings plane; every desk reads it).

**Step 2, "follow the ring".** The registered default of a setting is not a
boolean but the setting's *ring*. The effective value for a computer is:

```
stored row (true or false)            -> that value        (explicit override wins)
no row, ring state includes my ring   -> true              (follow the ring)
no row, ring state does not include   -> false
platform-excluded (paper rows)        -> an explicit false row, written once by the owner act that excludes it
```

A computer knows its own ring from its warmness class. The registry compiles
into a small generated Go table at build time (the declaration), and the
current ring comes from the host. How a guest reads the host's ring state is
**unverified**: I did not find, in the files I read, a guest-to-host channel
for it. The assumption is that vmctl serves the effective switch map for a
computer over the existing host link (as it serves guest-release facts), and
the guest caches it with a short TTL and re-reads at wake. If that channel
does not exist, slice 4 builds it, and until then step 1 alone gives new
computers the registered boolean default.

Two details that matter:

- A new computer is class `primary` (R3). It inherits whatever has completed
  R3's soak, and settings still at R2 do not reach it. That is the right
  behaviour for "new computers get nothing proved": they get exactly the
  proved set.
- An explicit row survives promotion. The ring walk does not overwrite an
  owner's or a user's explicit choice, and a revert does not either; revert
  changes only what "no row" resolves to.

### 4.4 Bootstrapping from the audit

The audit table is already one row per switch with kind, where, verdict and
named step. Seed the registry from it mechanically:

1. **DELETE rows (8: 45 to 52):** do not register them. Delete the code and
   the switch in one commit each (green or yellow, tier 0 when it is a
   no-caller deletion; check the citers per the deletion-citers question).
   The registry starts smaller than the audit.
2. **ON NOW rows (14: 1, 2, 10, 25 to 35):** register at `ring: none`, then
   walk the rings by hand on day one under section 3's soaks. Row 1 (D2) is
   the first: its week gate is a measurement, not a safety gate, and the
   policy replaces it.
3. **ON AFTER rows (25):** register with the step encoded as one of four
   gate kinds, by my reading of the table:
   - `metric` (rows 3, 4, 5, 9, 13, 20, 21): the audit's section 12 already
     says which health field or grep to read. Becomes a `promote` bound.
   - `probe` (rows 8, 11, 12, 14, 16, 17, 18, 19, 36): a script or drill that
     exists (`scripts/proofs/net01-host-fetch.sh`, `net02-mirrors.sh`,
     `scripts/proof-u4-rebase`, `scripts/density/accept_five.py`, the
     kill -9 durability drill). Becomes `probe:`.
   - `act` (rows 6, 7, 15): a supervised host act. Becomes `gate: {act: ...}`,
     recorded once by FP0.
   - `needs_code` (rows 22, 23, 24, 39) and `external` (37, 38): register as
     `blocked_by:` naming the build or event; they enter the walk when it
     lands.
4. **OWNER RULING rows (5: 40 to 44):** row 43 ("which proved settings
   become default-true") is exactly the policy table of 3.3 plus F-0; it
   ceases to be a question. Rows 40 and 41 (event layer) and 44 (unwired
   packages) get registry entries with `hold: owner` and a named question, so
   a hold is visible and has an expiry like any other switch. Row 42
   (`CI_REUSE_GREEN_PR_RUN`) is CI, not a runtime switch; register it as
   `kind: ci_env` with the same expiry rule.
5. **Section 9 of the audit (rollback levers, normal states):** register as
   `life: ops` with an owner and a review date, no expiry. They are the
   kill switches S4 says should be few and visible.
6. **Seed script** (tooling, yellow): reads the audit's table and emits the
   YAML skeleton with `where`, `kind`, `class` and `blocked_by`; a person
   fills in `owner` and the bounds. The audit file is then archived and the
   registry is the inventory.

Result: of 52 rows, 8 disappear into deletions, and the other 44 become
registry entries that a controller can walk, with 5 explicit holds.

## 5. Automatic analysis and promotion

### 5.1 Who does it

A **ring controller**: a deterministic host service, no model in the loop,
in the host floor next to vmctl. It reads the registry, samples metrics,
writes `state.json`, and posts every transition to the tape as a commitment
with its evidence. Reasons:

- A model judging a rollout is another thing that can be wrong in a
  correlated way with the change. Rule: the thing that changes behavior does
  not grade itself.
- Doctrine puts computer-wide coherence with the **management** desk and
  capsule-bound mutation with **engineering**; the host's own rollout is
  neither. It is host-level, security-relevant code that engineering cannot
  touch, so it belongs in the floor, run by FP0's host tooling.
- **Management desk** (each computer's): reads the ring status for its own
  computer and records it in the commitment ledger so Texture can render it
  to the owner. It may *propose* a registry change through a cast to
  engineering. It never advances or reverts a host switch.
- **Owner:** ratifies the policy table, receives the digest, can freeze
  everything with one command (`switches freeze`), and types the confirmation
  for advances beyond a switch's ceiling. The owner supervises; the owner is
  not a metric reader.
- **FP0** types the confirmations the owner delegates, and runs the manual
  ring walk until the controller exists.

No-SSH operability: status is a read-only vmctl internal endpoint and a
`choir` CLI read; `freeze`, `unfreeze` and `advance` are typed verbs on a
forced-command gate in the style of `scripts/node-b-ci-gate`, so an operator
does not need a shell.

### 5.2 What we can read today, and what control group means on one host

Nothing here is a new signal; these are what the audit and the D-series
designs already point at.

| Signal | Where it is read today | Used for |
|---|---|---|
| Start decisions, `need_source`, refusals | vmctl `/health` `admission.recent` | D4; any switch that changes memory accounting |
| Page cache return acts, host RSS fall, refaults | `/health` `memory.cache_return.recent` | D6 |
| Hibernate and wake timelines: snapshot type, size, time, wake-to-healthy, snapshot refusal reason, cold-boot fallback | `/health` per-computer timelines | D2, D5, D3; any snapshot or launch switch |
| Whether a computer is "behind" the current guest release | vmctl `guest-releases` endpoint (`in_use`, `image_moved_since_launch`) | D2 and every guest-image change |
| OG writer gate mode and counts | `/api/runtime/observability` | row 20 |
| Issuer-auth `would refuse` count | journal grep (FP0 only) | row 21 |
| Checkpoint cadence state, `last_bind_millis` | `choir computer checkpoints` | X1 |
| Run fate "interrupted by a restart", unplanned crash closures | the tape, per computer | every switch; the crash-loop guard |
| Deploy receipt, commit identity, CI result | `/var/lib/go-choir/deploy-receipt.json`, `/health` | deploy blackout (5.5) |
| Host available memory against the floor | `/health` | every switch |

Gaps, stated plainly:

- **No time series.** `/health` is point-in-time with `recent` ring buffers.
  I found no store that keeps a series (**unverified**: I did not sweep for
  one). The controller samples `/health` every 60 seconds into a bounded
  local JSONL, 7 days (resource use first: a few MB), and computes its own
  windows.
- **Guest-side signals need a path to the host.** Observability is per
  computer behind that computer's key; the controller has no keys. Slice 3
  uses only host-side signals (vmctl). Guest-side signals (OG gate counts,
  run acceptance) come over the host link in slice 4.

**Control group on one host.** There are three, in order of strength:

1. **Concurrent, same host, same class, not selected.** Because the host,
   kernel, disk and neighbors are shared, host-wide noise hits both groups,
   which is exactly what a canary wants (S1: "the two groups can share
   infrastructure", so also check absolute targets). Exists whenever the
   ring is smaller than its class, which is the usual case until a switch
   reaches `*`.
2. **Behind-release computers (code and guest-env changes).** Under D2, a
   computer may wake on an older release and update at its next idle moment.
   During any deploy, computers still behind are a free baseline: same
   host, same time, previous code. A ring-ordered refresh (slice 5) makes
   this deliberate.
3. **The computer's own prior series.** FP0 already does this (the DAMON
   reading on 71afbf14 against its pre-DAMON series). The controller keeps a
   per-computer baseline of the guard metrics over the 24 hours before the
   flip and compares after. It is the only control that exists once a switch
   reaches `*`.

No A/B assignment is needed: the ring is the assignment, chosen by
operational order, not randomly. That means **ring order is not a random
sample** and a ring's good result does not prove the next ring (failure mode
9.1).

### 5.3 Guards and the analysis template

Following S2's template shape, one shared template and per-switch
overrides. The default template, `host-default-v1`, holds about 6 guard
metrics, within S1's "roughly a dozen at most":

| Guard | Failure condition (default) | Kind |
|---|---|---|
| Unplanned crash closures of computers in the ring ("interrupted by a restart" with no planned marker) | any in the soak window; or 2 on one computer | zero tolerance |
| Start refusals or cold-boot fallbacks in the ring | more than the control's rate plus 1 | relative plus slack |
| Wake-to-healthy p95 | above max(2 x the computer's own 24 h baseline, 1.5 s) | relative |
| Host available memory | below the existing floor | absolute |
| Snapshot refusals with a corruption or integrity reason | any | zero tolerance |
| Controller's own sampling freshness | no sample in the last 3 minutes | forces hold |

A switch adds its own promotion metric and bound from the registry (for
D4: `need_source == settled` and no refusal that `peak` would have avoided;
for D6: host RSS falls by at least 70% of the guest-reclaimed MiB within
10 seconds, no process exit, refaults within twice the before figure, from
its design §10). Template fields, from S2: `interval` (default 60 s),
`initial_delay` (the switch's first-effect delay, e.g. the 5-minute settle
window), `failure_limit` (default 0 for zero-tolerance guards, 2 consecutive
samples for relative ones), `success` and `failure` conditions.
**Inconclusive** (neither condition met, or stale samples, or exposure not
reached) **holds the ring; it never advances.**

### 5.4 What advances a ring and what holds it

A ring advances only on **positive evidence**, never on silence:

1. The previous ring's soak is complete (time floor and exposure floor).
2. Every guard is evaluable (fresh samples) and none failed.
3. The switch's own bound is met at least once.
4. No other switch is in flight in the same surface; no deploy blackout.
5. The ring is within the policy-table ceiling for the switch's class. If
   not, the controller waits at "stalled at ceiling" for a confirmation.
6. The rollout budget is not exhausted (5.6).

The controller does one thing: set the switch's ring to the next ring. It
cannot change code, nix or the registry.

### 5.5 Single flight, surfaces, bundles and bisection

S1's rule is one canary at a time, for attribution. Serial walks of 14
switches at 7 hours each would be 4 days, which defeats the point. So:

- A switch declares a **surface** (snapshot, memory, egress, settings-texture,
  settings-selfdev, ci). The rule is one walk in flight **per surface**;
  disjoint surfaces walk concurrently, because their guards and failure modes
  are disjoint. Priority within a surface follows the owner's standing order:
  security, then availability, then customizations (2026-10-25).
- **Settings are mostly independent of host selectors**, so the 10 "ON NOW"
  settings (rows 25 to 34) walk together as one **bundle**: one soak for the
  bundle, one set of guards. On a guard failure the controller **bisects**:
  revert half, wait one short soak (R0 time floor), then the other half,
  until the culprit is found or the failure disappears. Bundles are allowed
  only within a surface and only for switches of class orange or red-additive.
- **Deploy blackout:** no ring advances from 10 minutes before a deploy until
  30 minutes after its receipt shows complete, and no advance while a deploy
  receipt is `incomplete`. A regression in that window belongs to the deploy
  until shown otherwise.

### 5.6 The rollout budget

The error-budget idea in a form Choir can measure: a count of incidents, not
a burn rate. Defaults: **3 auto-reverts or unplanned crash closures in
any 24 hours across R0 and R1; 1 across R2 and above.** When spent, the
controller freezes all promotions (reverts continue), posts one digest line,
and waits for a human `unfreeze`. Rollouts are fast while the budget is
healthy and stop on their own when it is not. Security fixes and rollbacks are
exempt, as S8 exempts reliability and security work.

## 6. Auto-revert

Three rules first.

1. **A revert is a state change, not a restart.** The controller never
   restarts a computer, never writes a planned-restart marker by itself, and
   never creates a boot-time resumer. This keeps the restart doctrine intact:
   a crash restart closes work; a planned update may resume it; the marker is
   consumed before resuming.
2. **A revert goes to the previous ring, not necessarily to off (non-security switches).** If the
   same switch is reverted twice in 24 hours it goes to `off` and is
   **quarantined**: it cannot re-enter the walk until a new commit changes
   its entry's `revision` (a fix, or a changed bound). That is hysteresis
   against flapping.
   **Security-enforcement switches (review 340 F2):** reverting enforce to
   observe or off is a security downgrade. At or above the first
   human-confirmed ring, a breach is **hold-and-page**: the ring stops
   advancing, the owner is alerted immediately, and a person decides the
   revert. Below that ring (R0, R1 under the policy table) the automatic
   revert stays, because the exposure is test computers. Security first.
3. **A revert must land before the next boot.** If the switch is crashing a
   computer, the markerless boot that follows a crash closes work and
   boots under the *reverted* selection, because the file write is
   immediate. That is why `state.json` is read at launch and is the sole
   place the controller writes.

Per kind of switch:

| Kind | Examples | Revert action | Effect | Interaction with the marker rule |
|---|---|---|---|---|
| **Host selector** (`*_ONLY`) with `follow:` | `VM_SNAPSHOT_DIFF_ONLY`, `VM_CHECKPOINT_CADENCE_ONLY`, `VMCTL_JAILER_ONLY`, `VM_BALLOON_STATS_ONLY` | controller writes the previous ring list | **At each computer's next launch.** Computers already launched under the switch keep it until then | None needed for soft reverts. For a revert that cannot wait (the switch corrupts state) the controller may *request* a planned refresh through vmctl's deferred-refresh sweep: idle computers now, busy ones at their next idle moment, marker-backed with reason `switch revert: <id>`. The paper and `public_platform` are never refreshed this way by the controller; only a human requests it. |
| **Host env mode read by a daemon** (`VMCTL_ADMISSION_NEED`, `VMCTL_GUEST_CACHE_RETURN`, `VMCTL_STORE_COMPACT_PRESSURE`, `GATEWAY_*` modes) | D4, D6, row 14 | the daemon re-reads the mode from `state.json` on each decision (mtime-cached); controller writes the previous mode | **Hot**, within one decision | No restart. This is the cheapest kind to revert and the first to convert from a plain nix env to `follow`. |
| **Let-binding** (nix compile-time: `snapshotReleaseRetain`, `ciGateEpochCheck`, `hostFetchVMs`, `gatewayModelScheduler`, `platformDoltSquashTimerEnabled`) | rows 1, 2, 6, 12, 17 | the controller **cannot** change these; they are baked into units by a nix build | Only by a commit and a deploy | `git revert` of the flip commit is the repo rollback path (CLAUDE.md). The controller's role is to *detect* the breach, freeze other promotions and print the exact revert commit. Convert the ones with a runtime reader to `follow` over time: `ciGateEpochCheck` is already read from a file (`/var/lib/go-choir/ci-gate-epoch-mode`, written by a tmpfiles rule), so the state file can be its source at once. `snapshotReleaseRetain` can become `follow` through the existing env it feeds (`VM_SNAPSHOT_RELEASE_RETAIN`). |
| **Guest env** (`CHOIR_*` in `nix/autoputer-vm.nix`) | `CHOIR_OG_WRITER_GATE` (not in `nix/autoputer-vm.nix` today: unset, code default observe, `objectgraph/writer.go:134-143`; registering it adds the line), `CHOIR_DOCUMENT_TOOL_SANDBOX`, `CHOIR_RUNTIME_CAPABILITY_FLOOR`, capsule memory policy | the AV:523 pattern: vmctl passes a per-computer value at boot; the guest env default is the safe mode | At the computer's **next boot** (boot arguments are in the launch-config hash, so changing the value voids that computer's snapshot and it cold boots: a real cost the soak must price in). Hot only if the guest re-reads the mode live, which is **unverified** per variable | A boot argument change is a launch-time change; it takes effect at the next planned or natural launch and never forces one. |
| **Setting** (per-computer, in the object graph) | the 26 `KnownComputerSettings` | with step 2 of F-0, retreat changes what "no row" resolves to; explicit rows are untouched | Within the guest cache TTL (about 60 s), no restart | Settings that start background work (`source.projection`, `texture.root_desk`) apply through their idempotent apply functions; a revert calls the same apply with `false`. Check that each such apply tolerates being turned off mid-run (**unverified**; see failure mode 9.5). |
| **Code deploy** (host services) | any train | no automatic revert exists. The deploy script records `previous_receipt`, `frontend-previous`, `<service>-previous` and the NixOS profile in the incomplete receipt, but I found no post-switch health check that rolls back on its own (**unverified**; I read `node-b-ci-deploy` only for the receipt shape) | Manual `git revert` plus a train, or the recorded previous profile | Slice 6: a post-deploy health probe that, on failure, selects the previous NixOS generation. One host means no canary here; the defence is the pre-deploy gates (`node-b-ci-gate`, the classifier, CI) and a fast post-deploy check. |
| **Code deploy** (guest image, D2 on) | any guest-image change | stop refreshing outer rings (they stay on the previous release, which is the control and the free rollback); for rings already refreshed, a planned refresh back onto the previous release directory | Behind computers are untouched. Refreshed ones return at their next idle moment | The return refresh is a planned update, marker-backed, so it may resume work, same as the forward one. Release directories are retained up to 4 on Node B (`GO_CHOIR_GUEST_RELEASE_KEEP_MAX`), within the one-week support window (`VM_GUEST_RELEASE_SUPPORT_WINDOW=168h`). Never during a self-development apply or a live capsule. |

Self-development rollback (just fixed, never yet run) and the U4 stack
rebase are per-computer rollback paths for engineering changes to a
computer's own code. They are separate from fleet switches. Do not wire the
controller to them. Do record in the registry that their switches
(`source.rebase_offer`, the rollback path itself) are `act`-gated until each
has run once on d@e (the roadmap loop 4 closes on one real rollback).

## 7. Flag-debt rule

From S4 (expiry, time bombs, a cap) with Choir's speed:

1. **No unregistered switch.** A CI check fails any change that adds a key to
   `KnownComputerSettings`, a `*_ONLY` selector, a `let` binding under the
   marked switch block in `nix/node-b.nix`, or a guest env under the marked
   block in `nix/autoputer-vm.nix` without a registry entry having `owner`,
   `class`, `surface`, `promote` and `expires`. Detection convention: a
   `# switch: <id>` comment on the line (the lint verifies the id, the line
   and the kind still agree, so `where` cannot rot). Tooling, yellow, tier 0.
2. **Default expiry is 14 days from `introduced`** for `life: release`
   (S4's "a week or two"; the ring walk is under a day, so 14 days is
   generous). `ops` and `permission` switches have no expiry but a
   `review_by` date at most 90 days out and a named owner.
3. **At expiry, the switch is either at `*` and retired, or deleted.** Retired
   means the switch is removed and the behavior is the code default. If it
   reached all rings, the controller prints "ready to retire" from day 1
   after R5; the retire commit deletes the switch and its registry entry
   together. If it did not reach all rings, the choice is delete it
   (the audit's DELETE verdicts) or extend it.
4. **Extension** is an edit to the entry with a reason, at most twice, at
   most 14 days each. A third extension is a delete.
5. **Enforcement is soft for deploys and hard for new switches** (S4's "time
   bomb" without the outage): an expired entry is a CI **warning** on every
   run from expiry, and from 7 days past expiry it makes the lint **fail any
   change that adds a new switch**. It never fails a bugfix, a security fix or
   a revert, so a debt can never block the deploy of the fix for itself (see
   9.6). This is the lean limit (S4): to add one, retire one.
6. **A cap on live release switches: 12.** Above the cap, the lint fails the
   addition of a new one. The 52 rows are a backlog; the cap applies once
   deletions and the first walks bring the live count under it (the baseline
   is recorded in the lint's config and ratchets down).
7. **The controller is a switch too (review 340 F6).** The shadow-mode
   controller (S3a) is built-but-off, the exact habit this design targets. It
   has a registry entry (`life: release`, expiry 14 days from S3a landing,
   `decision_by` the date shadow mode must be promoted to acting or deleted,
   with the shadow-versus-hand-walk comparison as the evidence). The debt rule
   applies to it.
8. **Retire is part of the definition.** Every mission Definition that adds a
   switch names its retire step and its owner (standing-questions: decision
   provenance, deletion citers, registry hygiene). The registry is the
   registry-hygiene answer.

## 8. Build plan in slices

Ordered by rulings removed per unit of risk. Class and tier per CLAUDE.md
(green and yellow: tier 0; orange: tier 1; red additive with one-act rollback:
tier 1; red where a defect is silent or irreversible: tier 2).

| Slice | What | Class | Review | Removes |
|---|---|---|---|---|
| **S0** | Owner ratifies the policy table. `switches/registry.yaml` seeded from the audit by named switch id (data only; the held ids are `intake.event_wakes`, `store.event_table` with `CHOIR_EVENT_TABLE_COMPUTERS`, `CI_REUSE_GREEN_PR_RUN`, and the unwired `appdesk`/`appnet`/`egressgrant`; counts are the audit's, re-derived by the seed script). A hand ring walk by FP0 under section 3's soaks and the same guards, starting with the 14 "ON NOW" items; the flips themselves keep their own class and review (row 1 is red). | green (the registry and policy); each flip keeps its class | none for the registry; the flips per their tier | The per-switch rulings: 14 ON NOW, and the 25 "named step" rows become gates the walk checks. **No code.** |
| **S1** | The registry lint: unregistered switch fails, expiry warns, cap, `where` check; the audit seed script | yellow | tier 0 (the checks are the review) | New debt. The ruling about whether a switch may be added. |
| **S2** | F-0 step 1: `store.SettingDefault`, fallback in `GetComputerSetting` and the seven readers; a roadmap W2 commit sets the default for the owner-approved settings; explicit `false` rows for the paper where meaningless | red (settings plane) | tier 2 (every desk reads it) | New computers get the proved set without a POST. Row 43. |
| **S3a** | `follow:<id>` token in `selectsComputer`, `state.json` reader in vmctl, `switches status` read endpoint and CLI; the controller in **shadow mode**: samples, evaluates guards, logs "would advance / would revert", writes nothing | orange | tier 1 | None yet; it measures the controller against the hand walk for a week of real flips (a replay of the audit's past incidents is the test: section 9.2). |
| **S3b** | The controller acts: advances and reverts selectors and daemon-read env modes; freeze, unfreeze, advance as typed gate verbs; rollout budget; deploy blackout | red | tier 2 (new concurrency, host state, boot path read) | The hand walk. Rows 3, 4, 5, 9, 10, 13, 14, 15, 16 and the other host-selector rows. |
| **S4** | Settings and guest env follow the ring live: the vmctl effective-switch endpoint on the host link, guest cache, `follow` for settings (F-0 step 2), the AV:523 boot-argument pattern generalised to the sandbox and capability-floor modes (rows 22, 23) | red | tier 2 | The settings rows, rows 20, 22, 23, and the per-computer-selector prerequisite in finding 4. |
| **S5** | Ring-ordered guest refresh: the D2 behind-refresh sweep takes the ring into account so inner rings refresh first and outer rings wait for the soak | red (VM lifecycle) | tier 2 | Code and guest-image changes get the ring walk too. |
| **S6** | Post-deploy health probe and automatic NixOS generation rollback; automatic return-refresh for a bad guest release | red | tier 2 (deploy routing) | Manual rollback of a bad train. Last, because it is the hardest and the one-host constraint limits its value. |

**Cheapest slice that removes the most rulings: S0.** It needs no code, one
decision from the owner and one pass of hand-walking. S1 and S2 follow
because they stop the debt from regrowing and fix the habit at its root.
The controller (S3) is built only after the hand walk has produced real
readings to calibrate it against, which is also the order the Convergence
rule asks for: understand before tuning.

Landing loop for S2 to S6 follows CLAUDE.md (commit, push, CI, staging
deploy, identity, deployed acceptance). The acceptance for S3b is a
**ring walk of a harmless switch on staging with a planted fault**: a test
switch that the controller promotes through R0 and R1 and then reverts when
a synthetic guard breaks, leaving the history file as the verifiable,
repeatable artifact.

## 9. Failure modes

1. **A rollout hides a regression.** Rings are ordered by operations, not by
   a random sample. R0 and R1 are test computers with synthetic use; a defect
   that appears only under owner use, under long uptime or under the paper's
   always-busy pattern will pass them. Defences: exposure floors that require
   real use at R2; per-computer baselines (5.2) that catch drift after the
   switch reaches `*`; a **late-regression window** (the guards keep running
   for 24 hours after R5 and a breach then reverts to the last ring, with the
   switch marked `regressed`); and the owner's `freeze`, which is a manual
   guard that costs one command. A regression that only shows weeks later is
   out of reach of any soak and belongs to the normal problem-doc path.
2. **A bad metric auto-promotes.** The classic is "no alarm" read as "good".
   Defences: advance requires fresh samples and a positive bound met (5.4),
   inconclusive holds (S2), the controller stops itself on stale samples, and
   **every guard has a replayed known-bad case**: recorded `/health` windows
   from real incidents (the mincore-filter bug on d@e's first Diff hibernate;
   the 11:49 refused start; b@c's crash-loop at boot) are fixtures the guard
   must flag. A guard with no known-bad fixture is not eligible to gate a
   promotion. Shadow mode (S3a) exists to find such guards before they act.
3. **The paper.** It is the one critical computer, always busy, with a
   standing intent (24/7 publishing) and the rule that it takes a planned,
   marker-backed refresh while busy, never mid self-development apply or with
   a live capsule. The controller never advances it, never refreshes it and
   never reverts it by restart. A revert reaches it only through hot
   mechanisms (a daemon-read mode, a setting) or waits for the next planned
   refresh. For a switch that has crashed other computers, the revert takes
   effect before the paper's next boot because the paper reads the same state
   file, and a crash boot is markerless, so work is closed rather than
   resumed (the doctrine's loop-breaker). The paper's guard set adds a
   schedule-tick success check: a schedule whose tick crashes the computer
   twice is already closed with that cause (`recoverSourceSchedulesAtBoot`).
4. **A revert storm.** Several switches fail together (a shared cause: a
   deploy, host memory pressure), the controller reverts all, the revert
   itself shifts load, guards flip again. Defences: the rollout budget (5.6)
   freezes promotion after 3 events in R0 and R1, 1 above; revert rate limit
   of 3 per hour, after which the controller stops reverting and posts one
   digest line instead of acting 20 times; hysteresis (rule 2 in section 6);
   deploy blackout, so a deploy-caused breach is not attributed to a switch
   (5.5); a revert never triggers a restart, so a storm cannot become a
   restart storm.
5. **The switch cannot be turned off cleanly.** Some settings start work
   whose "off" is not the inverse of "on" (the `evidence.graph` 25-second
   bounded import, projection jobs, `store.event_table` with its floor write
   and lease). Revert changes the resolved default; it cannot un-import. The
   registry entry for such a switch carries `revert: {mode: one_way}` and the
   class goes up one row in the policy table (the ceiling drops). The audit
   row 41 (`store.event_table`) is the clear case and stays a hold.
6. **The debt rule blocks the fix for itself.** Hard failure on expiry could
   stop a deploy that retires the debt. So expiry fails only the addition of
   new switches (7.5), never a fix or a revert.
7. **The registry and reality diverge.** A `where` that no longer holds, a
   nix line edited by hand away from `follow`. The lint checks `where` and the
   `follow` value on every CI run, and the controller compares its state
   against what vmctl records at launch; a mismatch is a hold with a digest
   line, not an advance.
8. **The controller is itself a heresy.** It is a new host authority over
   computers. It can only set a ring number within registry bounds, it is in
   the floor, it has a freeze, its writes are on the tape with evidence, and
   it runs in shadow first. It adds one state file and one writer; it deletes
   the nix-edit-per-step path and the per-switch ruling. Net, it reduces
   authorities.
9. **Attribution under concurrency.** Two surfaces walking at once may
   interact (snapshot and memory share the host memory guard). Defence:
   surfaces are declared by the author and reviewed; a guard breach during
   two concurrent walks reverts the **newer** one and holds the older; the
   bisection rule applies inside bundles.
10. **Owner computers as canary.** An R2 defect reaches the owner first. The
    mitigation is that R0 and R1 have run for 2.5 hours and exercised the
    same path, and that soft reverts are immediate. The remaining cost is
    open question 3.

## 10. Open questions for the owner

1. **Ratify the policy table (section 3.3).** In particular: may a red,
   additive, one-act-rollback switch reach owner and ordinary-user computers
   (R2, R3) with no per-switch ruling once its guards and soaks pass, while
   security enforcement stops at the test ring and the paper never advances
   without a human act? A yes replaces the per-switch rulings. A no on any row
   moves that row's ceiling; the design does not change.
2. **May a revert ask for a planned refresh?** When a switch is found to be
   corrupting state, the controller could request a planned, marker-backed
   refresh (idle computers now, busy ones at their next idle moment) of the
   computers that launched under it, instead of waiting for their next
   natural launch. Do you accept that for owner and ordinary-user computers,
   never for the paper or `public_platform` without a human act?
3. **Are your computers the first user-facing ring?** Putting
   `premium_always_on` at R2, ahead of every other user, means you see a
   defect before anyone else does. The alternative is to put a disposable
   fleet of simulated-use computers there first, which is slower to build and
   less real. Which do you want, and do you accept that cost?

## Owner answers (2026-10-28 01:2x UTC)

Context, owner: "we should definitely do runtime checks and make sure we
arent breaking things before rolling out features. but we shouldnt keep
shipping code in off state. thats getting messy as we have good code not
on needlessly... basically, we need to design the progressive delivery
system now."

1. **Policy table: more aggressive than section 3.3.** Every class except
   black auto-advances through R4 (every computer but the paper) once its
   guards pass and its soak ends. Security enforcement and silent or
   irreversible switches included; their soak multiplier (x2) and automatic
   revert stay. The paper (R5) advances only by a human act. Black never
   enters the auto path. This supersedes the section 3.3 ceilings.
2. **A revert may request a planned, marker-backed refresh everywhere,**
   the paper and `public_platform` included.
3. **The owner's computers are R2,** the first user-facing ring, ahead of
   every other user.
4. **Build order: the registry plus the hand walk first.** The registry
   file declares every switch, and a CI lint refuses an unregistered
   switch and enforces expiry. FP0 then walks the "ON NOW" switches through
   the rings by hand with the controller's readouts. The controller follows.

Standing rule (AGENTS.md): a mechanism never lands off for every computer;
it ships on at least R0 and widens by ring.

Later (owner, 2026-10-28): "the progressive delivery system will of course
evolve. like, id love to have a fleet of simulated users. we will
definitely get there. but not this week." A simulated-user fleet becomes a
ring ahead of R2 when it exists; not this week.
