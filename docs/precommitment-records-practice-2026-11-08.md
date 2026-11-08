# Precommitment records: the practice (2026-11-08)

Status: written at MA8.0 of the modular architecture mission. It describes the
engineering on main at the date above: the files, the invariants, the failure
modes and how to extend each part. The theory is in
[psa-theory-2026-11-08.md](psa-theory-2026-11-08.md). The longer, earlier
statement is [precommitment-records.md](precommitment-records.md); this document
replaces its engineering sections (4 to 6) as the description of what runs, and
that doc keeps its role for the motivation and the research record. The plan
([design/modular-architecture-plan-2026-11-04.md](design/modular-architecture-plan-2026-11-04.md))
and the ledger
([definitions/choir-modular-architecture-2026-11-04.ledger.md](definitions/choir-modular-architecture-2026-11-04.ledger.md))
hold the history. Line numbers drift; find code by name.

This document is revised at every phase that changes the protocol. A protocol
bump is not complete until its section here is.

## 1. The desk ontology on main

There are no special desks or apps (doctrine I26, as amended; CLAUDE.md Desk
Ontology). Management, engineering, research, Texture, Mail and the packages are
desks registered the same way in `internal/deskmanifest`. A desk's powers are
**grants** from a closed catalogue (`deskmanifest/grants.go`): its verbs,
capsule verbs, modules and cell deadline are computed from its grants at
registration. Two grants have no verb and exactly one holder, and registration
refuses a second holder: `admit.assignment` (management; the admission ledger
for engineering work) and `platform.update`. "System" or "app" says only who
publishes the desk.

Each data kind has one custodian desk with sole read and write. Desks talk only
through precommitment records. The commitment ledger on the tape is the durable
substrate, and Texture renders it to the owner.

To add a desk: ship a package whose manifest (v2) declares the desk row, its
grants and the kinds it owns (`choir.app.<pkg>.*`). The registry validates it,
`appdesk` records a `desk_package_events` row whose sequence is the desk epoch,
and a run pins the epoch at its first cell. Never add a desk by name to Go code;
the names gate in CI refuses desk-name literals outside a short allowlist.

## 2. The record

The type is `types.Commitment` (`internal/types/commitment.go`). The fields a
request adds are `p_success`, `weight`, `impact_class`, `settlement` and
`materiality_procedure` (a `MaterialityFreeze`: the catalogue entry, its hash
and the frozen weights). Optional on read, required on write by the reducer, so
older tape records stay decodable.

- **Act set**: cast, ask, report, reply, escalate, precommit, resolve,
  disagreement, plus the typed request kinds feed and decline. There is no
  note and no emit channel; both were deleted.
- **Requests** (cast, ask, escalate, feed, decline) must state `p_success` in
  [0,1]; a missing or out-of-range value is refused. `weight` is required at
  impact I1 and above, and a Cast that states no weight says why in
  `weight_zero`. `settlement` is a rule kind from the closed vocabulary
  (`commitment_rules.go`) or "recipient".
- **Provenance** is stamped by the reducer from the run record (agent, model,
  desk, package and version, run, origin class). A cell cannot set it; a
  negative test pins that.
- **Labels** (section 6) join the record: durable on the run record, a
  session-scoped union, derived records taking the union of their sources.
- **Inertness**: the reducer's detector sets `inert_flags` at commit. A cell can
  neither set nor clear them, and no cast is refused for being inert.
- **Sub-calls**: every child-agent call is a Cast whose typed questions carry
  the parent's precommitment; the child's Report, a settlement, a self-score
  and the judge's work are records (`subcall_record.go`, `subcall_judge.go`).

Failure modes seen: a Cast landing mid-run and then a final Report stranded the
caster (fixed: a final Report is refused while a Cast in its round is unread,
and a run end tells the caster); a judge not durable before its coalesce sleep
(fixed: the store writes the judge work with the first scorable Cast).

To extend: add the field as optional on read and required on write, add a golden
fixture, re-capture the protocol lock (section 3), and add a negative test that
a cell cannot supply it.

## 3. The inter-desk protocol and its lock

One address resolver (`internal/agentcore/inbox_records.go`, with
`deskmanifest` for the registry) maps a recipient name to its registered inbox.
The inbox shape comes from the recipient's declared `instance`: per owner
(`<desk>:<owner>`), per subject, or per run. The non-desk inbox classes (the
capture-source inbox, the open-new-subject address `texture:new`, `job:<id>`)
are registered classes owned by the service that reads them. A request to an
unregistered name is refused. Any desk may send to any desk; the recipient
admits on its own grants. A cast to engineering from a desk without
`admit.assignment` lands in management's inbox.

The protocol surface is locked. `internal/deskprotocol` extracts the surface
(wire actions, envelope kinds, enums, the desk row, transition tables) and
`cmd/deskprotocol-lock` checks it against `internal/deskprotocol/lock/<Version>.json`.
The current version is 14.0.0 (`deskprotocol/version.go`). The CI job
`desk-protocol` fails a change that moves the surface without the matching bump:
a removed field or an added enum value on a closed type is a major, an added
optional field a minor. Locks are append-only, so a reverted slice still leaves
its lock file. The deployed base's checker tolerates a lock absent at the base.
A commit that moves the surface carries the gate trailers `ci.yml` checks.

To extend: change the code, run `cmd/deskprotocol-lock` to see the
classification, add the lock file, bump `Version`. Add a transition to the table
only if the store performs it (the transition test runs against the store, not
only the table).

## 4. Leases

`internal/lease` is a kernel service. A lease is a `choir.lease` record. A
request for one is a precommit-kind record, scored like any other, and the
service decides it inside the desk's manifest ceiling (`Desk.LeaseKinds`,
`LeaseCaptures`), re-checked at every use. The cell verb is
`choir.RequestLease(scope, purpose, req)`, for package desks only; the host
binds desk and run.

A lease ends with the run's end (a store observer ends it on every terminal run
write), at boot (`leases_end_at_boot`; a crash never resumes a lease), on owner
revoke (`GET /api/leases`, `POST /api/leases/{id}/revoke`), or when the holder
is retired. The reader gate admits a non-custodian against a live lease
(`objectgraph.WithLeases`). The capsule store copy keeps only the kinds the
assignment's lease names. Platform-issued leases exist for the capsule and for
Texture-to-research (`texture.brief`) and Texture search (`texture.search`).

`lease/policy.go` sets duration from the holder's posterior lower bound: a
strong record gets a durable lease that renews while the record holds; a thin
one stays per-task. `RevokeOnFidelity` revokes at once when the breaker fires.

Known limits: the use log is a counter plus a once-per-cause log line, not a
frame record; the `texture.brief` lease admits every Texture-to-research message
of the run, not only briefs; engineering has no manifest ceiling, so its capsule
lease is a platform bound.

To extend: add the kind to the desk's `LeaseKinds`, add the grant to the
catalogue and an impact row for it in `internal/trust/impact_table.json` (a test
requires a row for every catalogue grant).

## 5. Custody gates

`internal/objectgraph` enforces custody. Every write declares its writer: the
reducer puts the committing desk on the context (`objectgraph.DeskWriter`, from
the run record), host loops declare the kind's custodian, owner routes declare
the owner. Reads declare a reader with `WithReader`. `CheckReadAs` runs in every
SQL-store read that names or returns a kind. A multi-writer batch commits
atomically with each part checked under its own writer (`PutBatchSegments`).

**Package kinds enforce. Built-in kinds are counted, not enforced.** For a
built-in kind a violation increments a counter keyed
`kind|declared writer|outcome` (`wrong_writer`, `undeclared_writer`, and the
reader and route equivalents) at `/api/runtime/observability`, logs the first
of each, and allows. The counters reached zero on a representative day and on
d@e, but the enforce flip for built-in kinds has not been taken. This is open.

Route custody (`internal/routekinds`) maps routes to kinds and checks the
`X-Choir-App` identity; a package-objects route refuses another package, and
built-in rows count `wrong_app` and `undeclared_app`.

Package desks hold four object verbs (`Put`, `Retract` and the reads), bounded
per desk and owner at 10,000 live objects, 64 MiB and 600 writes an hour, and
at 120 wakes an hour (`deskmanifest.MaxWakesPerHour`).

## 6. The disclosure path

Each run carries provenance labels: `world_untrusted`, `platform`,
`publisher:<id>`, `owner_private:<kind>`. At each sink the strictest label
decides (`internal/disclosure/rules.go`; the table is pinned cell by cell by a
test). Sinks: publication (`AutoPublishRevision`), egress (the cell's
`choir.Read`, `choir.SourceAPI`, `choir.SearchStart`), desk message (the
reducer's addressed acts), mail send, Library release, and the act gate.
`world_untrusted`, `platform` and `publisher:<id>` allow every sink, so the
paper keeps publishing. `owner_private:<kind>` allows egress for every desk
(owner ruling, 2026-11-05: no asking before each search), asks at publication
and Library release, allows a desk message to the kind's custodian or a lease
holder, and asks otherwise.

An ask goes to a System 1 closed yes/no/unsure question over host-composed
evidence (`agentcore/disclosure_gate.go`). The full distribution is a
precommit-kind record carrying the run's labels. The rule allows only when
P(yes) is at least 0.8 and P(no) at most 0.1; otherwise the owner is asked. The
tap is a `disclosure.decision` event answered through `GET /api/disclosures` and
`POST /api/disclosures/{id}/approve|deny` (owner routes; the ProductAPI refuses
them).

To extend: add the sink with its table row and its host call site; add the cell
to the pinned test.

## 7. Judgment: System 1 and desks

The owner's ruling of 2026-11-08: System 1 models outside desks are fine; LLMs
are not. The host judge service, the prompt router, the sub-call settler, the
desk cast judge, the inertness judge, the disclosure gate judge, the support
judge, intake labels, story analysis and the relate checks all call the System 1
judgment client (`rt.candidateJudge`, the gateway judgment client) and stand. An
LLM judgment of a candidate goes through a desk.

Desk shots are the worked example (design:
[desk-shots-and-review-2026-11-07.md](design/desk-shots-and-review-2026-11-07.md)).
A platform candidate-shot worker was deleted. Management has three verbs:
`choir.ShotStart(operationID)` starts the before and after shots in the existing
sandbox; `choir.ShotWait(jobID, maxSeconds)` waits (at most 240 s inside the 300
s cell deadline) and queues the job's images for management's own model;
`choir.ShotVerdict(operationID, verdict, findings, reason)` records management's
verdict on the tape as a dedicated event bound to the operation, host build job,
shot job and plan digest. A verdict is refused without findings for every screen
or without the injected images in the run's memory. Tool-loop failover is
image-aware. d@e proved the path on a one-line platform change (apply-proof-5d,
2026-11-08: management's pass verdict, applied).

Open: two sites still call an LLM from the computer runtime outside any desk.
The install review seats (`internal/installreview`; Devin unwired, DeepSeek
through the tool-loop provider) are one. The anchor sampler was the other and was
reverted whole (0047d0892) on 2026-11-08. The install review design that moves
the seats onto a desk
(`docs/design/desk-install-review-2026-11-08.md`) is not on main; main still has
the platform seats.

## 8. The install review gate

An install that widens its ceiling installs only with a sealed install-review
verdict for its exact digests (`internal/installreview`,
`agentcore/install_review.go`). Mechanical checks run first; seats are sized by
risk, and each answer is a precommitment. Known limits: the seal proves only
that the socket was reachable, not who the reviewer was; the platform has no
Devin provider and the DeepSeek seat had no funds when last exercised, so a
widened install is refused today. Publisher reputation (`trust/reputation.go`)
is written as sentences with n from signed settled outcomes, never a score.

## 9. The trust view and the act gate

`internal/trust` derives the view and never stores it. `Derive(Inputs, Params)`
returns a row per subject and action class: posterior, lower bound, tau,
breaker. Evidence classes E1 to E8 (`evidence.go`), the impact table as locked
data (`impact_table.json`, version impact-table/3, a row for every catalogue
grant), a hierarchical prior and change discount (`prior.go`), tempered and
capped weighting both computed (`posterior.go`), and `Params` logged with every
view so it reproduces. `GET /api/trust` serves the view; `?replay=1` adds the
replay AUC and `gate_replay` (owner taps per day). `agentcore/api_trust.go`
builds the inputs from sub-call records, desk Cast settlements and tape facts.

The act gate (`internal/actgate`) is consulted in `commitActIntent` for every
staged act whose record class is I2 or higher. Autonomous only when the desk is
not first-party, its lower bound clears tau, and its grant has no open breaker;
I4 never. Otherwise the act goes to the owner tap on the disclosure tape (sink
`act_gate`) and management is told. First-party desks are counted, not held. The
breaker trips on a forged frame for every class of the desk, writes a
`trust.breaker` tape fact (read back as E3), revokes leases and notifies
management.

Known limits: the view reads sub-call records and desk Cast settlements, so a
package desk is thin and gated until its records build up (the conservative
default); E4 and E7 are not view inputs; the only real-path fidelity event is
the forged frame, since the reducer stamps no declared capabilities.

## 10. Anchors and the MA7.9 harness

`internal/trust` also holds the library half of the anchor machinery:
`anchors.go`, `scorers.go` (scorer profiles), `ppi.go` (prediction-powered
intervals), `randomize.go` (propensities). It has no live caller. The live
sampler, its provider judge, the trust-anchor store kinds and the harness anchor
reader were reverted on 2026-11-08, so P6 and P7 of the harness have no served
anchors. Converting the anchor question into a desk act is open.

`cmd/trustharness` and `internal/trustharness` produce the pre-registered tests
and write dated artifacts under `docs/evidence/ma7-9-harness/`; `-check`
reproduces a stored result byte for byte. Results append to the pre-registration
file whatever they show. Drills plant known-bad packages on d@e only.

## 11. Isolation of the desk worker

Every desk worker runs under gVisor (runsc, pinned, sha512 both arches), one
sandbox per trust class: A first-party custodians, B first-party desks reading
untrusted content, C one per other publisher. "Required" mode means
gVisor-confined, and a non-first-party desk is refused where gVisor is
unavailable. Code mode uses yaegi with a per-desk import set derived from the
desk's grants (`yaegikernel.DeskImportSet`). Go modules a package ships are
checked against a curated list at publish and install. The isolation self-test
is `POST /api/runtime/isolation-selftest`.

## 12. Restarts and obligations

A crash restart closes every open obligation with the fate "interrupted by a
restart" and never resumes it, including leases. A planned restart may resume,
only with a durable marker consumed once at the next boot. A desk's boot runs
`desk_reconcile` and then `close_open_obligations_after_crash`. An open shot job
closes at every boot.

## 13. Open items

- Built-in kinds counted rather than enforced (section 5).
- MA4.11: the desk-name switches. Management's lifecycle code still keys on the
  profile name, and the names gate keeps a Texture residue until Texture's desk
  moves with its app.
- The anchor question as a desk act, and the install review seats as a desk act
  (sections 7, 10).
- Parameters: impact numbers, tau, decay and the weighting mode are starting
  policy until replay decides.
- Phase 8, the return to self-development under the floors, has not started.
